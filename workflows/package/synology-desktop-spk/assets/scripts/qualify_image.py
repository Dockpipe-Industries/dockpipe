"""Qualify the imported desktop under a non-root identity with no network access."""

import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
import uuid


PROBE = r'''
import base64
from pathlib import Path
import ssl
import sys
import urllib.error
import urllib.request

url = 'https://127.0.0.1:3001/webman/3rdparty/dockpipe-desktop/desktop/'
context = ssl._create_unverified_context()
try:
    urllib.request.urlopen(url, context=context, timeout=3)
except urllib.error.HTTPError as error:
    if error.code != 401:
        sys.exit(1)
else:
    sys.exit(1)
settings = Path('/run/dockpipe-package/desktop-settings')
credential = (settings / 'username').read_bytes() + b':' + (settings / 'password').read_bytes()
request = urllib.request.Request(url, headers={'Authorization': 'Basic ' + base64.b64encode(credential).decode()})
with urllib.request.urlopen(request, context=context, timeout=3) as response:
    if response.status != 200 or "frame-ancestors 'self'" not in response.headers.get('Content-Security-Policy', ''):
        sys.exit(1)
for process in Path('/proc').iterdir():
    if not process.name.isdigit():
        continue
    try:
        executable = Path((process / "cmdline").read_bytes().split(bytes([0]))[0].decode()).name
        state = (process / 'stat').read_text().split(') ', 1)[1].split()[0]
        if executable == 'dockpipe-launcher' and state != 'Z':
            print('ready')
            sys.exit(0)
    except (OSError, UnicodeError, IndexError):
        continue
sys.exit(1)
'''


def run(arguments, timeout=60, **options):
    return subprocess.run(arguments, check=True, timeout=timeout, **options)


def qualify(archive, receipt, temporary_parent):
    uid, gid = os.getuid(), os.getgid()
    if uid == 0 or gid == 0:
        raise ValueError("Desktop qualification requires a real unprivileged runner identity")
    run(["docker", "load", "--input", str(archive)], timeout=1200)
    name = "dockpipe-desktop-qualification-" + uuid.uuid4().hex
    with tempfile.TemporaryDirectory(prefix=".desktop-probe-", dir=temporary_parent) as temporary:
        temporary = Path(temporary)
        home = temporary / "home"
        settings = home / "desktop-settings"
        settings.mkdir(parents=True, mode=0o700)
        data = temporary / "data"
        data.mkdir(mode=0o700)
        password = secrets.token_urlsafe(32)
        for key, value in (("username", "qualification"), ("password", password), ("uid", str(uid)), ("gid", str(gid))):
            path = settings / key
            path.write_text(value)
            path.chmod(0o600)
        try:
            run([
                "docker", "run", "--detach", "--name", name, "--pull", "never", "--network", "none",
                "--user", f"{uid}:{gid}", "--cap-drop", "ALL", "--security-opt", "no-new-privileges=true",
                "--shm-size", "1g", "--volume", f"{home}:/run/dockpipe-package:ro",
                "--volume", f"{data}:/config", receipt["offline_image"],
            ])
            deadline = time.monotonic() + 120
            ready = False
            while time.monotonic() < deadline:
                result = subprocess.run(
                    ["docker", "exec", name, "python3", "-c", PROBE],
                    capture_output=True, text=True, timeout=10,
                )
                if result.returncode == 0 and result.stdout.strip() == "ready":
                    ready = True
                    break
                time.sleep(1)
            if not ready:
                raise ValueError("Imported desktop did not pass authenticated startup qualification")
            environment = run(["docker", "inspect", "--format", "{{json .Config.Env}}", name], capture_output=True, text=True).stdout
            logs = run(["docker", "logs", "--tail", "100", name], capture_output=True, text=True)
            if password in environment or password in logs.stdout or password in logs.stderr:
                raise ValueError("Desktop qualification detected a credential in persisted environment or logs")
            run(["docker", "stop", "--time", "15", name], timeout=30)
            state = json.loads(run(["docker", "inspect", "--format", "{{json .State}}", name], capture_output=True, text=True).stdout)
            if state["ExitCode"] != 0 or state["OOMKilled"]:
                raise ValueError("Imported desktop did not shut down cleanly")
        finally:
            cleanup = subprocess.run(
                ["docker", "rm", "--force", name], capture_output=True, text=True, timeout=60,
            )
            if cleanup.returncode and "no such container" not in cleanup.stderr.lower():
                raise ValueError("Could not remove the owned desktop qualification container")
    print("Qualified imported desktop: non-root, no network/pulls, authenticated UI, live Qt process, clean stop")

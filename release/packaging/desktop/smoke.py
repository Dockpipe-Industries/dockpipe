"""Exercise deployed Qt and the launcher's CLI lookup outside a source checkout."""
import argparse
import os
from pathlib import Path
import subprocess
import tempfile


def check(launcher, expected_cli):
    launcher = Path(launcher).resolve()
    with tempfile.TemporaryDirectory(prefix="dockpipe-desktop-smoke-") as temporary:
        home = Path(temporary)
        environment = dict(os.environ, HOME=temporary, XDG_CONFIG_HOME=str(home / "config"),
                           XDG_DATA_HOME=str(home / "data"), XDG_CACHE_HOME=str(home / "cache"),
                           XDG_STATE_HOME=str(home / "state"), DOCKPIPE_GLOBAL_ROOT=str(home / "global"),
                           QT_QPA_PLATFORM="offscreen")
        if os.name == "nt":
            environment.update(APPDATA=str(home / "roaming"), LOCALAPPDATA=str(home / "local"))
            windows = Path(os.environ["SystemRoot"])
            environment["PATH"] = os.pathsep.join([str(windows / "System32"), str(windows)])
        else:
            environment["PATH"] = "/usr/bin:/bin:/usr/sbin:/sbin"
        for name in ("DOCKPIPE_BIN", "DOCKPIPE_SYSTEM_ROOT", "DOCKPIPE_REPO_ROOT", "QT_PLUGIN_PATH",
                     "QML2_IMPORT_PATH", "QT_QPA_PLATFORM_PLUGIN_PATH", "DYLD_FRAMEWORK_PATH",
                     "DYLD_LIBRARY_PATH", "LD_LIBRARY_PATH"):
            environment.pop(name, None)
        result = subprocess.run([str(launcher), "--check-installation"], cwd=home, env=environment,
                                capture_output=True, text=True, timeout=45)
        if result.returncode or f"CLI: {expected_cli}" not in result.stdout:
            raise RuntimeError(result.stdout + result.stderr)
        print(result.stdout)
        with (home / "launcher.log").open("w+") as log:
            process = subprocess.Popen([str(launcher), "--allow-second-instance"], cwd=home,
                                       env=environment, stdout=log, stderr=log)
            try:
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    print("PASS: deployed Qt window remains running")
                else:
                    log.seek(0)
                    raise RuntimeError(f"Launcher exited early ({process.returncode}): {log.read()}")
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("launcher")
    parser.add_argument("expected_cli")
    args = parser.parse_args()
    check(args.launcher, args.expected_cli)

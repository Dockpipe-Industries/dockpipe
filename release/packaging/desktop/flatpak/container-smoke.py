#!/usr/bin/env python3
"""Exercise a real engine, Compose and host-visible bind paths; leave no stack."""
import argparse
from pathlib import Path
import subprocess
import tempfile
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("app", type=Path)
    parser.add_argument("--socket", type=Path, default=Path("/run/docker.sock"))
    parser.add_argument("--image", default="debian:12", help="already available test image")
    args = parser.parse_args()
    if not args.socket.is_socket():
        raise SystemExit(f"Engine socket is not available: {args.socket}")
    project = "dockpipe-flatpak-check-" + uuid.uuid4().hex[:10]
    with tempfile.TemporaryDirectory(prefix="dockpipe-flatpak-container-") as temporary:
        root = Path(temporary)
        (root / "input").write_text("flatpak-bind-ok\n")
        compose = root / "compose.yml"
        compose.write_text("services:\n  check:\n    image: " + args.image + "\n"
                           "    network_mode: none\n    command: [cat, /qualification/input]\n"
                           "    volumes:\n      - type: bind\n        source: " + str(root) + "\n"
                           "        target: /qualification\n        read_only: true\n")
        command = ["flatpak", "build", "--runtime", "--nofilesystem=home", f"--filesystem={root}",
                   f"--filesystem={args.socket}", f"--env=DOCKER_HOST=unix://{args.socket}",
                   f"--env=XDG_CONFIG_HOME={root / 'config'}", f"--env=XDG_CACHE_HOME={root / 'cache'}",
                   str(args.app.resolve()), "/app/bin/docker", "compose", "--project-name", project,
                   "--file", str(compose)]
        try:
            result = subprocess.run([*command, "run", "--rm", "--no-deps", "--pull", "never", "check"],
                                    check=True, text=True, stdout=subprocess.PIPE, timeout=120)
            if result.stdout.strip() != "flatpak-bind-ok":
                raise RuntimeError(f"Container did not read the bind mount: {result.stdout}")
        finally:
            subprocess.run([*command, "down", "--remove-orphans"], check=True, timeout=60)
    print("PASS: Flatpak Compose and real engine bind mount; disposable stack removed")


if __name__ == "__main__":
    main()

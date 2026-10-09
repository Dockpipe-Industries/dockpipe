#!/usr/bin/env python3
"""Check the shipped Flatpak runtime, core-only inventory, and real workflow execution."""
import json
import subprocess
import sys
import tempfile
from pathlib import Path


def smoke(app):
    with tempfile.TemporaryDirectory(prefix="dockpipe-flatpak-smoke-") as temporary:
        root = Path(temporary)
        options = ["flatpak", "build", "--runtime", "--unshare=network",
                   "--nofilesystem=host", "--nofilesystem=home", f"--filesystem={root}",
                   f"--build-dir={root}", "--env=QT_QPA_PLATFORM=offscreen"]
        for key, subdir in (("HOME", "home"), ("XDG_DATA_HOME", "data"),
                            ("XDG_STATE_HOME", "state"), ("XDG_CACHE_HOME", "cache"),
                            ("XDG_CONFIG_HOME", "config")):
            (root / subdir).mkdir()
            options.append(f"--env={key}={root / subdir}")

        def run(*args):
            return subprocess.run([*options, str(app), *args], check=True, text=True,
                                  stdout=subprocess.PIPE, timeout=90).stdout

        cli = "/app/bin/dockpipe"
        version = run(cli, "--version").strip()
        assert "git version" in run("/app/bin/git", "--version")
        assert "Docker version" in run("/app/bin/docker", "--version")
        assert run("/app/bin/docker", "compose", "version", "--short").strip() == "5.6.0"
        assert "v0.38.0" in run("/app/bin/docker", "buildx", "version")
        assert run("/app/bin/dockpipe-launcher", "--version").strip() == version
        assert "CLI: /app/bin/dockpipe" in run("/app/bin/dockpipe-launcher", "--check-installation")
        inventory = json.loads(run(cli, "package", "list", "--format", "json", "--workdir", str(root)))
        assert len(inventory["packages"]) == 1, inventory
        assert inventory["packages"][0]["kind"] == "core", inventory

        workflow = root / "workflow.yml"
        workflow.write_text("name: flatpak-smoke\nplatforms: [flatpak]\ndocker_preflight: false\n"
                            "dependencies:\n  host:\n    - command: git\nsteps:\n"
                            "  - id: local\n    kind: host\n    cwd: repo\n    run: smoke.sh\n")
        script = root / "smoke.sh"
        script.write_text("#!/usr/bin/env bash\nset -euo pipefail\n"
                          "test -f /.flatpak-info\n"
                          "case \"$TMPDIR\" in \"$XDG_CACHE_HOME\"/*) ;; *) exit 1 ;; esac\n"
                          "printf flatpak-ok > result.txt\n")
        script.chmod(0o755)
        run(cli, "--workflow-file", str(workflow), "--workdir", str(root))
        assert (root / "result.txt").read_text() == "flatpak-ok"

        # Export the already bundled core from inside the actual Platform. This
        # exercises the same detector used for Marketplace listing and install.
        options.append("--env=DOCKPIPE_PACKAGES_ROOT=/app/share/dockpipe/packages")
        run(cli, "package", "build", "store", "--workdir", str(root),
            "--only", "core", "--out", str(root / "store"))
        store = json.loads((root / "store/packages-store-manifest.json").read_text())
        assert "-flatpak-org.kde.Platform-6.10" in store["platform"], store
        assert not store["packages"].get("workflows"), store
        assert not store["packages"].get("resolvers"), store
    print("PASS: Flatpak launcher, CLI, core-only inventory, workflow, and package platform")


if __name__ == "__main__":
    smoke(Path(sys.argv[1]).resolve())

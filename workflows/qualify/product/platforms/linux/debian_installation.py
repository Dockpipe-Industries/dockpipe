"""Bind an installed Pop!_OS desktop to exact owner-selected Debian packages."""

from hashlib import sha256
from pathlib import Path
import platform
import subprocess
import sys
import tarfile

from desktop import digest

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "assets/scripts"))
from contracts import require, write_json


LAUNCHER = Path("/usr/bin/dockpipe-launcher")
CLI = Path("/usr/bin/dockpipe")
DESKTOP_FILES = (
    "/usr/lib/dockpipe/bin/dockpipe-launcher",
    "/usr/lib/dockpipe/lib/libdockpipe-launcher-ui.so.1",
    "/usr/lib/dockpipe/lib/libSpatialWebBrowser.so.0",
)


def package_identity(artifact, names):
    pending = set(names)
    hashes = {}
    with subprocess.Popen(["/usr/bin/dpkg-deb", "--fsys-tarfile", str(artifact)],
                          stdout=subprocess.PIPE) as process:
        try:
            with tarfile.open(fileobj=process.stdout, mode="r|") as archive:
                for member in archive:
                    name = "/" + member.name.removeprefix("./")
                    if name in pending:
                        require(member.isfile() and member.size <= 64 << 20, "Unexpected selected DEB member")
                        checksum = sha256()
                        with archive.extractfile(member) as stream:
                            for block in iter(lambda: stream.read(256 << 10), b""):
                                checksum.update(block)
                        hashes[name] = checksum.hexdigest()
                        pending.remove(name)
            for _ in iter(lambda: process.stdout.read(64 << 10), b""):
                pass
            require(process.wait(timeout=5) == 0 and not pending, "Selected DEB members are incomplete")
        finally:
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=5)

    require(all(digest(Path(name)) == expected for name, expected in hashes.items()),
            "Installed product bytes differ from the selected package")
    return hashes


def admit(request, output):
    target = request["target"]
    release = platform.freedesktop_os_release()
    require(target["platform"] == "linux-amd64-deb-pop22.04" and target["kind"] == "local"
            and platform.machine() == "x86_64" and release["ID"] == "pop" and release["VERSION_ID"] == "22.04",
            "Installed desktop driver requires the selected Pop!_OS 22.04 amd64 target")
    config = request["configuration"]
    require(set(config) == {"artifact", "cli_artifact", "cli_artifact_sha256", "xvfb", "xvfb_sha256"},
            "Select the two DEBs and the private X11 executable")
    artifacts = {
        "artifact": target["artifact_sha256"], "cli_artifact": config["cli_artifact_sha256"],
        "xvfb": config["xvfb_sha256"],
    }
    for key, expected in artifacts.items():
        path = Path(config[key])
        require(path.is_absolute() and digest(path) == expected, "Selected input differs from its pin: " + key)
    require(LAUNCHER.resolve() == Path(DESKTOP_FILES[0]), "Installed launcher alias selects another executable")

    output.mkdir(mode=0o700, parents=True)
    binaries = package_identity(Path(config["artifact"]), DESKTOP_FILES)
    binaries.update(package_identity(Path(config["cli_artifact"]), [str(CLI)]))
    identity_path = output / "package-identity.json"
    write_json(identity_path, {"package_pins": artifacts, "installed_files": binaries})
    return Path(config["xvfb"]), identity_path, binaries

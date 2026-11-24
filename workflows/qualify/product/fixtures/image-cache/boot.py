"""Verify one immutable Linux VM cache image with a fresh offline boot."""
from pathlib import Path
import argparse
import json
import os
import secrets
import shutil
import stat
import subprocess
import sys


sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "assets/scripts"))
from contracts import DIGEST, IDENTIFIER, digest_file, matches, read_json, require, write_json
from process import run_driver


MAX_IMAGE_BYTES = 64 * 1024**3


def make_seed(output, manifest, nonce):
    seed = output / "nocloud"
    seed.mkdir(mode=0o700)
    write_json(seed / "meta-data", {"instance-id": nonce, "local-hostname": "dockpipe-image-test"})
    release = manifest["release"]
    operating_system = manifest["os_id"]
    command = [
        "sh", "-ec",
        ' . /etc/os-release\n'
        'test "$ID" = "$1"\n'
        'test "$VERSION_ID" = "$2"\n'
        'test "$(uname -m)" = x86_64\n'
        'marker=/var/lib/dockpipe-image-cache-proof\n'
        'test ! -e "$marker"\n'
        'printf "%s\\n" "$3" > "$marker"\n'
        'test "$(cat "$marker")" = "$3"\n'
        'sync\n'
        'printf "DOCKPIPE_IMAGE_BOOT:%s:%s:%s\\n" "$1" "$2" "$3" > /dev/ttyS0',
        "dockpipe-boot-proof", operating_system, release, nonce,
    ]
    config = {"runcmd": [command],
              "power_state": {"mode": "poweroff", "timeout": 30,
                              "condition": ["test", "-f", "/var/lib/dockpipe-image-cache-proof"]}}
    (seed / "user-data").write_text("#cloud-config\n" + json.dumps(config) + "\n")
    (seed / "network-config").write_text('{"version":2,"ethernets":{}}\n')
    media = output / "nocloud.iso"
    subprocess.run(["genisoimage", "-quiet", "-V", "cidata", "-J", "-R", "-o", str(media),
                    str(seed)], stdin=subprocess.DEVNULL, check=True, timeout=20)
    return media


def probe_image(qemu_img, path, backing_chain=False):
    command = [qemu_img, "info", "--output=json"]
    if backing_chain:
        command.append("--backing-chain")
    result = subprocess.run([*command, str(path)], stdin=subprocess.DEVNULL,
                            stdout=subprocess.PIPE, check=True, timeout=15)
    return json.loads(result.stdout)


def boot(manifest_path, base, output):
    manifest = read_json(manifest_path)
    require(manifest["schema"] == "dockpipe.qualification-image.v2"
            and manifest["architecture"] == "amd64" and manifest["image"]["format"] == "qcow2",
            "Expected a pinned amd64 QCOW2 qualification profile")
    require(matches(IDENTIFIER, manifest["os_id"]) and matches(IDENTIFIER, manifest["release"]),
            "Select an explicit Linux OS identity and version")
    image = manifest["image"]
    digest = image["sha256"]
    require(matches(DIGEST, digest), "Image must have a pinned SHA-256")
    base = base.absolute()
    require(base.resolve(strict=True) == base and stat.S_ISREG(base.lstat().st_mode)
            and base.stat().st_mode & 0o222 == 0,
            "Select a canonical, read-only regular cache file")
    require(base.stat().st_size == image["bytes"] and digest_file(base, MAX_IMAGE_BYTES) == digest,
            "Cached image does not match the selected size and SHA-256")

    tools = {name: shutil.which(name) for name in ("qemu-system-x86_64", "qemu-img", "genisoimage")}
    require(all(tools.values()) and os.access("/dev/kvm", os.R_OK | os.W_OK),
            "KVM, qemu-img, QEMU and genisoimage are required")
    info = probe_image(tools["qemu-img"], base)
    require(info["format"] == "qcow2" and not info.get("backing-filename")
            and not info.get("dirty-flag") and not info["format-specific"]["data"].get("corrupt"),
            "Base must be a standalone clean QCOW2 image")

    settings = manifest["boot"]
    require(settings["firmware"] in {"bios", "uefi"}
            and settings["machine"] == "q35" and settings["network"] == "off"
            and settings["datasource"] == "nocloud" and settings["disk_bus"] == "virtio"
            and settings["cpus"] == 2
            and type(settings["memory_mib"]) is int and 768 <= settings["memory_mib"] <= 4096,
            "Unsupported pilot boot settings")
    output = output.absolute()
    output.mkdir(mode=0o700, parents=True)
    nonce = secrets.token_hex(12)
    seed = make_seed(output, manifest, nonce)
    overlay = output / "run.qcow2"
    subprocess.run([tools["qemu-img"], "create", "-q", "-f", "qcow2", "-F", "qcow2",
                    "-b", str(base), str(overlay)], stdin=subprocess.DEVNULL, check=True, timeout=15)
    chain = probe_image(tools["qemu-img"], overlay, backing_chain=True)
    require(len(chain) == 2 and Path(chain[1]["filename"]).resolve(strict=True) == base,
            "Private overlay does not point to the pinned cache file")
    before = overlay.stat().st_size
    command = [tools["qemu-system-x86_64"], "-machine", "q35,accel=kvm", "-cpu", "host",
               "-smp", "2", "-m", str(settings["memory_mib"]), "-display", "none", "-monitor", "none",
               "-serial", "stdio", "-nic", "none", "-no-reboot", "-boot", "order=c",
               "-drive", f"file={overlay},format=qcow2,if=virtio", "-cdrom", str(seed)]
    require("," not in str(overlay), "The QEMU overlay path must not contain a comma")
    if settings["firmware"] == "uefi":
        firmware = Path("/usr/share/OVMF")
        variables = output / "ovmf-vars.fd"
        shutil.copyfile(firmware / "OVMF_VARS.fd", variables)
        command.extend(["-drive", f"file={firmware / 'OVMF_CODE.fd'},if=pflash,format=raw,readonly=on",
                        "-drive", f"file={variables},if=pflash,format=raw"])

    environment = {name: os.environ[name] for name in ("PATH", "LANG", "LC_ALL") if name in os.environ}
    execution = run_driver(command, output, 240, environment)
    after = digest_file(base, MAX_IMAGE_BYTES)
    serial = (output / "driver.stdout").read_text(errors="replace")
    expected = f"DOCKPIPE_IMAGE_BOOT:{manifest['os_id']}:{manifest['release']}:{nonce}"
    wrote_in_guest = expected in serial.splitlines()
    overlay_grew = overlay.stat().st_size > before
    passed = (execution["exit_code"] == 0 and not execution["failure"]
              and wrote_in_guest and after == digest and overlay_grew)
    write_json(output / "result.json", {
        "schema": "dockpipe.qualification-image.boot/v1", "passed": passed,
        "release": manifest["release"], "architecture": manifest["architecture"],
        "image_sha256_before": digest, "image_sha256_after": after,
        "overlay_grew": overlay_grew, "guest_write_observed": wrote_in_guest,
        "boot_settings": settings, "execution": execution,
    })
    require(passed, "Offline boot, guest write, shutdown or cached-image verification failed")
    print(f"Verified {manifest['os_id']} {manifest['release']} booted from a fresh overlay and preserved the cache")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--base", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    inputs = parser.parse_args()
    os.umask(0o077)
    boot(inputs.manifest, inputs.base, inputs.output)

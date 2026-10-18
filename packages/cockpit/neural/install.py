"""Install the optional local inference runtime. Never called by page content.

The native parent owns a ten-minute deadline and the installer's process group.
A completion marker is published only after dependency/model checks pass. Partial
files are reusable only after checksum verification. No model inference downloads.
"""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import urllib.request
import venv


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            checksum.update(chunk)
    return checksum.hexdigest()


def download(entry, destination):
    target = destination / entry["name"]
    if target.is_file() and target.stat().st_size == entry["bytes"] and digest(target) == entry["sha256"]:
        return
    temporary = target.with_suffix(".partial")
    started = time.monotonic()
    received = 0
    request = urllib.request.Request(entry["url"], headers={"User-Agent": "Dockpipe-Cockpit/0.1"})
    try:
        with urllib.request.urlopen(request, timeout=30) as response, temporary.open("wb") as output:
            if not response.url.startswith("https://"):
                raise ValueError("Model redirect must use HTTPS")
            while chunk := response.read(1024 * 1024):
                received += len(chunk)
                if received > entry["bytes"] or time.monotonic() - started > 180:
                    raise ValueError("Model download exceeded its byte or time budget")
                output.write(chunk)
        if received != entry["bytes"] or digest(temporary) != entry["sha256"]:
            raise ValueError("Model checksum mismatch")
        temporary.replace(target)
    finally:
        temporary.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--destination", type=Path, required=True)
    parser.add_argument("--gpu", action="store_true", help="Install the larger CUDA-capable ONNX runtime")
    args = parser.parse_args()
    root = args.destination.resolve()
    root.mkdir(parents=True, exist_ok=True)
    lock = (root / "install.lock").open("a")
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError as error:
        raise ValueError("Another Cockpit window is installing this runtime") from error
    # The installer is package-owned, as is this manifest. Page content supplies
    # neither download URLs nor Python package names.
    manifest = json.loads(Path(__file__).with_name("models.json").read_text())
    print("Preparing isolated local inference runtime…", flush=True)
    runtime = root / "runtime"
    if not (runtime / "bin/python").is_file():
        venv.EnvBuilder(with_pip=True).create(runtime)
    python = str(runtime / "bin/python")
    engine = "onnxruntime-gpu[cuda,cudnn]==1.23.2" if args.gpu else "onnxruntime==1.23.2"
    subprocess.run([python, "-m", "pip", "install", "--disable-pip-version-check", "--no-input",
                    "--only-binary=:all:", "--timeout", "30", "--retries", "1",
                    engine, "tokenizers==0.22.1", "numpy==2.2.6", "Pillow==12.0.0"],
                   check=True, timeout=300)
    models = root / "models"
    models.mkdir(exist_ok=True)
    for entry in manifest["files"]:
        print("Downloading and verifying " + entry["name"] + "…", flush=True)
        download(entry, models)
    subprocess.run([python, "-I", "-c", "import onnxruntime, tokenizers, numpy, PIL"],
                   check=True, timeout=30)
    marker = root / "ready.partial"
    marker.write_text(json.dumps({"version": 1, "gpu": args.gpu}))
    os.replace(marker, root / "ready.json")
    print("Local world intelligence is ready.", flush=True)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print("Installation failed: " + str(error)[:300], file=sys.stderr)
        sys.exit(1)

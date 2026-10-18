#!/usr/bin/env python3
"""Explicit native SDK audit/split and detached publisher-signature verification."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from elf_artifacts import audit, split


def file_digest(path, maximum_bytes=None):
    checksum = hashlib.sha256()
    total = 0
    with Path(path).open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            total += len(block)
            if maximum_bytes is not None and total > maximum_bytes:
                raise ValueError("Artifact content exceeds its authenticated size")
            checksum.update(block)
    return checksum.hexdigest()


def verify_signature(manifest, signature, public_key):
    # The caller supplies the trusted publisher key independently of the download.
    # No private signing key is ever read by this consumer tool.
    result = subprocess.run(
        ["openssl", "pkeyutl", "-verify", "-pubin", "-inkey", str(public_key),
         "-rawin", "-in", str(manifest), "-sigfile", str(signature)],
        capture_output=True, timeout=30,
    )
    if result.returncode:
        raise ValueError("Publisher signature verification failed")


def verify_files(manifest, root, maximum_bytes):
    if maximum_bytes <= 0:
        raise ValueError("A positive artifact budget is required")
    if manifest.stat().st_size > 1024 * 1024:
        raise ValueError("Release metadata exceeds the 1 MiB protocol limit")
    value = json.loads(manifest.read_text())
    if (not isinstance(value, dict) or value.get("schema") != 1
            or value.get("artifact_class") not in {"runtime", "sdk"}):
        raise ValueError("Unsupported public artifact metadata")
    files = value.get("files")
    if not isinstance(files, list) or not files:
        raise ValueError("Release must list its complete artifact files")
    expected = {}
    directories = set()
    total = 0
    for entry in files:
        if not isinstance(entry, dict):
            raise ValueError("Invalid artifact entry")
        name = entry.get("path")
        size = entry.get("bytes")
        checksum = entry.get("sha256")
        if not isinstance(name, str) or not name or "\\" in name:
            raise ValueError("Invalid artifact path")
        relative = Path(name)
        if (relative.is_absolute() or ".." in relative.parts or name == "."
                or relative.as_posix() != name or name in expected
                or not isinstance(size, int) or isinstance(size, bool) or size < 0
                or not isinstance(checksum, str) or len(checksum) != 64
                or any(character not in "0123456789abcdef" for character in checksum)):
            raise ValueError("Invalid or duplicate artifact path/size/digest")
        total += size
        if total > maximum_bytes:
            raise ValueError("Release exceeds the caller's artifact byte budget")
        expected[name] = entry
        directories.update(parent.as_posix() for parent in relative.parents if parent != Path("."))
    if root.is_symlink() or not root.is_dir():
        raise ValueError("Artifact root must be a real directory")
    pending = [root]
    seen = set()
    # Only visit manifest-derived directories; reject unexpected entries immediately.
    while pending:
        directory = pending.pop()
        with os.scandir(directory) as entries:
            for item in entries:
                path = Path(item.path)
                name = path.relative_to(root).as_posix()
                if item.is_symlink():
                    raise ValueError("Artifact links are not allowed")
                if item.is_dir(follow_symlinks=False) and name in directories:
                    pending.append(path)
                    continue
                if name not in expected or not item.is_file(follow_symlinks=False):
                    raise ValueError("Unlisted or unsupported artifact entry")
                entry = expected[name]
                if item.stat().st_size != entry["bytes"] or file_digest(path, entry["bytes"]) != entry["sha256"]:
                    raise ValueError("Artifact content does not match authenticated metadata")
                seen.add(name)
    if seen != set(expected):
        raise ValueError("Missing artifact files")
    return value


def verify_release(manifest, signature, public_key, root, maximum_bytes):
    # Snapshot bounded signed inputs so verification and parsing use identical bytes.
    with tempfile.TemporaryDirectory(prefix="native-sdk-verification-") as temporary:
        inputs = []
        for source, name, limit in ((manifest, "manifest.json", 1024 * 1024),
                                    (signature, "signature", 16 * 1024),
                                    (public_key, "publisher.pem", 64 * 1024)):
            with source.open("rb") as stream:
                content = stream.read(limit + 1)
            if len(content) > limit:
                raise ValueError("Signature verification input exceeds its protocol limit")
            target = Path(temporary) / name
            target.write_bytes(content)
            inputs.append(target)
        verify_signature(*inputs)
        return verify_files(inputs[0], root, maximum_bytes)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("audit", "split"):
        command = commands.add_parser(name)
        command.add_argument("--binary", type=Path, required=True)
        command.add_argument("--exports", type=Path, required=True)
        command.add_argument("--receipt", type=Path, required=True)
        if name == "split":
            command.add_argument("--runtime", type=Path, required=True)
            command.add_argument("--debug", type=Path, required=True)
    verify = commands.add_parser("verify")
    for name in ("manifest", "signature", "public-key", "root"):
        verify.add_argument("--" + name, type=Path, required=True)
    verify.add_argument("--max-bytes", type=int, required=True)
    args = parser.parse_args()
    try:
        if args.command == "verify":
            if args.max_bytes <= 0:
                raise ValueError("A positive artifact budget is required")
            verify_release(args.manifest, args.signature, args.public_key, args.root, args.max_bytes)
            print("Publisher signature and complete artifact contents verified.")
            return
        expected = args.exports.read_text().splitlines()
        if args.receipt.exists():
            raise ValueError("Receipt already exists; use a fresh output")
        if args.command == "audit":
            result = audit(args.binary, expected)
        else:
            result = split(args.binary, args.runtime, args.debug, expected)
            result["runtime_sha256"] = file_digest(args.runtime)
            result["debug_sha256"] = file_digest(args.debug)
        result["source_binary_sha256"] = file_digest(args.binary)
        args.receipt.parent.mkdir(parents=True, exist_ok=True)
        args.receipt.write_text(json.dumps(result, indent=2) + "\n")
        print("Native artifact verification passed.")
    except (ValueError, OSError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        print(str(error), file=sys.stderr)
        raise SystemExit(1) from None


if __name__ == "__main__":
    main()

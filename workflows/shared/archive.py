"""Deterministic regular-file archives shared by independent artifact producers."""
import gzip
import hashlib
import tarfile


def digest(path):
    checksum = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            checksum.update(block)
    return checksum.hexdigest()


def write_archive(root, archive, entries, archive_root):
    with archive.open("xb") as destination:
        with gzip.GzipFile(fileobj=destination, mode="wb", filename="", mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.PAX_FORMAT) as bundle:
                for entry in entries:
                    source = root / entry["path"]
                    member = tarfile.TarInfo(archive_root + "/" + entry["path"])
                    member.size = entry["bytes"]
                    member.mode = 0o755 if source.stat().st_mode & 0o111 else 0o644
                    with source.open("rb") as stream:
                        bundle.addfile(member, stream)

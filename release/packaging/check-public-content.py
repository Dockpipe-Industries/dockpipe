#!/usr/bin/env python3
"""Reject private identifiers and organization-specific deployment defaults."""
import argparse
from pathlib import Path, PurePosixPath
import re
import subprocess
import tarfile

HOME_PATH = re.compile(r"/(?:home|Users)/([A-Za-z0-9_.-]+)/")
EXAMPLE_USERS = {
    "ExampleUser", "YOUR_USER", "TestUser", "developer", "you", "user",
    "test", "me", "x", "text", "node", "coder", "runner",
}
ACCOUNT_ENDPOINT = re.compile(r"\b([a-f0-9]{32})\.r2\.cloudflarestorage\.com", re.I)
ENVIRONMENT_ID = re.compile(r'\b(?=[a-z2-7]{0,25}[2-7])[a-z2-7]{26}\b')
ACCOUNT_SETTING = re.compile(r'(?:account_id|zone_id|CloudflareAccountId|TfVarZoneId).*\b[a-f0-9]{32}\b', re.I)
DEPLOYMENT_DEFAULT = re.compile(
    r'(?:R2_BUCKET|R2Bucket|STATE_BUCKET|StateBucket|RELEASE_BUCKET|REPOSITORY)'
    r'[^\n]*?(?:[=:]\s*["\x27]?|:-)(?:dockpipe(?:-staging|-tfstate)?|Dockpipe-Industries/dockpipe)'
    r'(?=["\x27}\s;]|$)'
)
# These exact payload roots are populated from the pinned upstream inputs in
# desktop/flatpak/prepare-package-tools.py. Our wrappers, source metadata, npm
# lockfiles, and all tracked repository files remain subject to the check.
BUNDLED_TOOL_PAYLOAD = re.compile(
    r'^(?:resolvers|workflows)/[^/]+/assets/tooling/bin/\.deps/'
    r'(?:aws/aws/|node/runtime/|go/go/|wrangler/node_modules/|azure/)'
)


def is_bundled_tool_payload(name):
    path = PurePosixPath(name)
    if path.is_absolute() or ".." in path.parts or "\\" in name:
        return False
    canonical = path.as_posix()
    if canonical.endswith('/.deps/azure/source.json'):
        return False
    return BUNDLED_TOOL_PAYLOAD.match(canonical) is not None


def findings(path, text):
    problems = []
    fixture = any(part in {"tests", "testdata"} for part in Path(path).parts) or Path(path).name.endswith("_test.go")
    for number, line in enumerate(text.splitlines(), 1):
        if any(match[1] not in EXAMPLE_USERS and not match[1].startswith(".") for match in HOME_PATH.finditer(line)):
            problems.append((number, "personal home path; use a documented example path"))
        if not fixture and (ACCOUNT_ENDPOINT.search(line) or ACCOUNT_SETTING.search(line)):
            problems.append((number, "account-specific R2 endpoint; require operator configuration"))
        if not fixture and ENVIRONMENT_ID.search(line):
            problems.append((number, "opaque environment identifier; use an explicit placeholder"))
        if path.startswith(("packages/", "workflows/")) and not fixture and DEPLOYMENT_DEFAULT.search(line):
            problems.append((number, "organization-specific deployment default"))
    return problems


def check_entries(entries):
    failures = 0
    for path, data in entries:
        try:
            text = data.decode("utf-8")
        except UnicodeError:
            continue
        for line, reason in findings(path, text):
            print(f"{path}:{line}: {reason}")
            failures += 1
    return failures


def tracked_entries(root):
    names = subprocess.check_output(["git", "ls-files", "-z"], cwd=root).decode().split("\0")
    for name in names:
        path = root / name
        if name and path.is_file() and not path.is_symlink():
            yield name, path.read_bytes()


def archive_entries(path):
    with tarfile.open(path, "r:*") as archive:
        for member in archive:
            if member.isfile() and not is_bundled_tool_payload(member.name):
                stream = archive.extractfile(member)
                if stream is not None:
                    yield "packages/" + member.name.lstrip("./"), stream.read()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--archive", type=Path, action="append", default=[])
    parser.add_argument("--store-root", type=Path)
    arguments = parser.parse_args()
    if arguments.store_root is not None:
        arguments.archive.extend(sorted(arguments.store_root.rglob("*.tar.gz")))
        if not arguments.archive:
            parser.error("No package archives found in the requested store")
    if arguments.archive:
        failures = sum(check_entries(archive_entries(path)) for path in arguments.archive)
    else:
        failures = check_entries(tracked_entries(arguments.root))
    if failures:
        print(f"Public-content check failed: {failures} finding(s); matched values withheld.")
        return 1
    print("Public-content check passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

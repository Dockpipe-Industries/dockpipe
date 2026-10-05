#!/usr/bin/env python3
"""Copy only package publication credentials to GitHub; secrets travel on stdin."""
import argparse
import base64
import binascii
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap

REPOSITORY = "Dockpipe-Industries/dockpipe"
SECRETS = ("AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "APT_SIGNING_KEY")
VARIABLES = ("R2_ENDPOINT_URL", "DOCKPIPE_RELEASE_BUCKET", "R2_PREFIX")


def run(arguments, *, value=None, env=None):
    result = subprocess.run(arguments, input=value, env=env, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(f"{arguments[0]} operation failed (output withheld)")
    return result.stdout


def signing_fingerprint(key):
    with tempfile.TemporaryDirectory(prefix="dockpipe-signing-check-") as directory:
        env = dict(os.environ, GNUPGHOME=directory)
        os.chmod(directory, 0o700)
        try:
            run(["gpg", "--batch", "--import"], value=key, env=env)
            listing = run(["gpg", "--batch", "--with-colons", "--list-secret-keys"], env=env)
            primary = [line.split(":") for line in listing.splitlines() if line.startswith("sec:")]
            if len(primary) != 1 or "s" not in primary[0][11].lower():
                raise ValueError("APT_SIGNING_KEY must contain one signing identity")
            fingerprint = next(line.split(":")[9] for line in listing.splitlines() if line.startswith("fpr:"))
            payload = Path(directory) / "check.txt"
            payload.write_text("DockPipe signing configuration check\n")
            run(["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "",
                 "--local-user", fingerprint, "--detach-sign", str(payload)], env=env)
            return fingerprint
        finally:
            subprocess.run(["gpgconf", "--kill", "gpg-agent"], env=env, capture_output=True)


def normalize_signing_key(value):
    """Restore ASCII armor when a secret environment flattens its line breaks."""
    value = value.strip()
    if len(value.splitlines()) > 1:
        return value
    header = "-----BEGIN PGP PRIVATE KEY BLOCK-----"
    footer = "-----END PGP PRIVATE KEY BLOCK-----"
    if not value.startswith(header) or not value.endswith(footer):
        raise ValueError("APT_SIGNING_KEY must contain a complete ASCII-armored private key")
    body = "".join(value[len(header):-len(footer)].split())
    checksum = ""
    if len(body) > 5 and body[-5] == "=":
        checksum, body = body[-5:], body[:-5]
    try:
        if not base64.b64decode(body, validate=True):
            raise ValueError("Empty signing key")
        if checksum and len(base64.b64decode(checksum[1:], validate=True)) != 3:
            raise ValueError("Invalid armor checksum")
    except (ValueError, binascii.Error):
        raise ValueError("APT_SIGNING_KEY armor is invalid (value withheld)") from None
    lines = [header, "", *textwrap.wrap(body, 64)]
    if checksum:
        lines.append(checksum)
    lines.append(footer)
    return "\n".join(lines) + "\n"


def configure(apply):
    values = {name: os.environ.get(name, "") for name in (*SECRETS, *VARIABLES)}
    missing = [name for name, value in values.items() if not value]
    if missing:
        raise ValueError("Missing release settings: " + ", ".join(missing))
    if not values["R2_ENDPOINT_URL"].startswith("https://"):
        raise ValueError("R2_ENDPOINT_URL must use HTTPS")
    values["APT_SIGNING_KEY"] = normalize_signing_key(values["APT_SIGNING_KEY"])
    fingerprint = signing_fingerprint(values["APT_SIGNING_KEY"])
    print(f"Validated APT signing fingerprint: {fingerprint}")
    print(f"Target: {REPOSITORY}, GitHub environment: release")
    if not apply:
        print("Checks passed. Set RELEASE_SETUP_APPLY=1 to copy the three scoped secrets and public settings.")
        return
    for name in SECRETS:
        run(["gh", "secret", "set", name, "--repo", REPOSITORY, "--env", "release"], value=values[name])
    for name, value in {**{name: values[name] for name in VARIABLES}, "APT_SIGNING_FINGERPRINT": fingerprint}.items():
        run(["gh", "variable", "set", name, "--repo", REPOSITORY, "--env", "release", "--body", value])
    print("Configured the GitHub release environment. No release was started.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true")
    arguments = parser.parse_args()
    try:
        configure(arguments.apply)
    except (ValueError, RuntimeError, OSError) as error:
        parser.exit(1, f"{error}\n")

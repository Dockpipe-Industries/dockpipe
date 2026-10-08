"""Render a pinned macOS formula from a completed DockPipe staging release."""
import re
from pathlib import Path


CANDIDATE_PATTERN = re.compile(r"(\d+\.\d+\.\d+)-staging\.([1-9]\d*)\.([1-9]\d*)\.([0-9a-f]{12})")
ORIGIN = "https://packages.staging.dockpipe.com"


def candidate_parts(candidate):
    match = CANDIDATE_PATTERN.fullmatch(candidate)
    if not match:
        raise ValueError("Invalid staging candidate")
    return match.groups()


def candidate_order(candidate):
    version, run, attempt, _ = candidate_parts(candidate)
    return (*map(int, version.split(".")), int(run), int(attempt))


def parse_checksums(text):
    checksums = {}
    for line in text.splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9_.-]+)", line)
        if not match or match[2] in checksums:
            raise ValueError("Invalid or duplicate release checksum")
        checksums[match[2]] = match[1]
    return checksums


def render(candidate, checksums):
    version, _, _, _ = candidate_parts(candidate)
    base = f"{ORIGIN}/packages/candidates/{candidate}"
    blocks = []
    for architecture, condition in (("arm64", "on_arm"), ("amd64", "on_intel")):
        binary = f"dockpipe_{version}_darwin_{architecture}.tar.gz"
        core = f"dockpipe-core-{version}.tar.gz"
        for filename in (binary, core):
            if not re.fullmatch(r"[0-9a-f]{64}", checksums.get(filename, "")):
                raise ValueError(f"Missing or invalid checksum: {filename}")
        blocks.append(f'''  {condition} do
    url "{base}/{binary}"
    sha256 "{checksums[binary]}"

    resource "core" do
      url "{base}/{core}"
      sha256 "{checksums[core]}"
    end
  end''')
    template = Path(__file__).with_name("staging_formula.rb.template").read_text()
    return template.replace("@CANDIDATE@", candidate).replace("@RELEASE_VERSION@", version).replace("@PLATFORMS@", "\n\n".join(blocks))


def render_cask(candidate, checksums):
    version, _, _, _ = candidate_parts(candidate)
    base = f"{ORIGIN}/packages/candidates/{candidate}"
    blocks = []
    for architecture, condition in (("arm64", "on_arm"), ("amd64", "on_intel")):
        filename = f"dockpipe-desktop_{version}_darwin_{architecture}.zip"
        checksum = checksums.get(filename, "")
        if not re.fullmatch(r"[0-9a-f]{64}", checksum):
            raise ValueError(f"Missing or invalid checksum: {filename}")
        blocks.append(f'''  {condition} do
    url "{base}/{filename}"
    sha256 "{checksum}"
  end''')
    template = Path(__file__).with_name("staging_cask.rb.template").read_text()
    return template.replace("@CANDIDATE@", candidate).replace("@PLATFORMS@", "\n\n".join(blocks))

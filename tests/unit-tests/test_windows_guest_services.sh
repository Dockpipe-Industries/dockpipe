#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKPIPE_TEST_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:?DOCKPIPE_TEST_DOCKPIPE_BIN is required}"
FIXTURE="$(mktemp -d)"
trap 'rm -rf "$FIXTURE"' EXIT

pwsh -NoProfile -NonInteractive -File "$ROOT/tests/unit-tests/test_windows_guest_services.ps1"
pwsh -NoProfile -NonInteractive -File "$ROOT/tests/unit-tests/test_windows_guest_agent_repair.ps1"

export XDG_STATE_HOME="$FIXTURE/durable"
export XDG_CACHE_HOME="$FIXTURE/cache"
export DOCKPIPE_BIN="$DOCKPIPE_TEST_BIN"
export DOCKPIPE_WORKDIR="$FIXTURE"
export DOCKPIPE_VMIMAGE_SOURCE_ONLY=1
unset DOCKPIPE_PACKAGE_ROOT DOCKPIPE_PACKAGE_ID DOCKPIPE_PACKAGE_STATE_DIR DOCKPIPE_SDK_ROOT
# Load only the VM script's functions; no guest or host service is invoked.
# shellcheck source=/dev/null
source "$ROOT/src/core/assets/scripts/vmimage-run.sh"

for mode in '' manual; do
  export DOCKPIPE_VM_WINDOWS_INSTALL_MODE="$mode"
  [[ "$(vmimage_windows_install_mode)" == manual ]]
  if vmimage_windows_should_unattend installer-iso; then
    echo 'Manual mode unexpectedly enabled unattended installation.' >&2
    exit 1
  fi
done
for mode in unattended automated auto; do
  export DOCKPIPE_VM_WINDOWS_INSTALL_MODE="$mode"
  [[ "$(vmimage_windows_install_mode)" == unattended ]]
  vmimage_windows_should_unattend installer-iso
  if vmimage_windows_should_unattend image; then
    echo 'Existing disk images must not use installer first-logon provisioning.' >&2
    exit 1
  fi
done

export DOCKPIPE_VM_WINDOWS_ADMIN_USER="fixture'user"
export DOCKPIPE_VM_WINDOWS_SSH_PUBKEY="ssh-ed25519 fixture'public-key"
vmimage_windows_bootstrap_encoded > "$FIXTURE/bootstrap.b64"
python3 - "$FIXTURE" <<'PY'
import base64
from pathlib import Path
import sys

root = Path(sys.argv[1])
script = base64.b64decode((root / 'bootstrap.b64').read_bytes()).decode('utf-16le')
assert "'provision-windows-ssh.ps1'" in script
assert "-UserName 'fixture''user'" in script
assert "-AuthorizedKey 'ssh-ed25519 fixture''public-key'" in script
assert '-CreateUser $false' in script
assert '-InstallDockPipeAgent $true' in script
assert '-EnablePasswordAuth $false' in script
assert '$attempt -lt 60' in script
assert 'Start-Sleep -Seconds 2' in script
assert 'throw ' in script
assert 'Add-WindowsCapability' not in script
assert 'Set-Content' not in script
(root / 'bootstrap.ps1').write_text(script)
PY
DOCKPIPE_TEST_BOOTSTRAP_PATH="$FIXTURE/bootstrap.ps1" pwsh -NoProfile -NonInteractive -Command '
  $Path = $env:DOCKPIPE_TEST_BOOTSTRAP_PATH
  $tokens = $null
  $parseErrors = $null
  [Management.Automation.Language.Parser]::ParseFile($Path, [ref]$tokens, [ref]$parseErrors) > $null
  if ($parseErrors.Count -ne 0) { throw "First-logon bootstrap parse errors: $parseErrors" }
'

# Bootstrap assets have disk capacity and stay separate from the XML floppy.
bootstrap_media="$FIXTURE/bootstrap-media"
unattend_media="$FIXTURE/unattend-media"
mkdir -p "$bootstrap_media" "$unattend_media"
python3 - "$bootstrap_media" "$unattend_media" <<'PY_MEDIA'
from pathlib import Path
import sys

bootstrap, unattended = map(Path, sys.argv[1:])
(bootstrap / 'guest-agent-fixture.exe').write_bytes(b'x' * (4 * 1024 * 1024))
(unattended / 'Autounattend.xml').write_text('<unattend />')
PY_MEDIA
export DOCKPIPE_VM_WINDOWS_UNATTEND_DIR="$unattend_media"
attached_media="$(vmimage_attach_bootstrap_media_dir "$bootstrap_media")"
[[ "$attached_media" == "$bootstrap_media" ]]
for boot_source in image installer-iso; do
  [[ "$(vmimage_bootstrap_attachment_mode "$boot_source" "$attached_media")" == disk ]]
done
[[ -z "$(vmimage_attach_bootstrap_media_dir '')" ]]
[[ -z "$(vmimage_bootstrap_attachment_mode installer-iso '')" ]]
if (vmimage_bootstrap_attachment_mode unsupported "$bootstrap_media" >/dev/null 2>&1); then
  echo 'Unsupported bootstrap sources must fail instead of selecting a media type.' >&2
  exit 1
fi
python3 - "$bootstrap_media" "$unattend_media" <<'PY_MEDIA'
from pathlib import Path
import sys

bootstrap, unattended = map(Path, sys.argv[1:])
assert (bootstrap / 'guest-agent-fixture.exe').stat().st_size == 4 * 1024 * 1024
assert sorted(path.name for path in unattended.iterdir()) == ['Autounattend.xml']
assert (unattended / 'Autounattend.xml').read_text() == '<unattend />'
PY_MEDIA

# Render the actual first-logon XML and prove it carries the shared helper call.
vmimage_windows_unattend_dir() {
  printf '%s\n' "$unattend_media"
}
export DOCKPIPE_VM_WINDOWS_LOCALE=en-US
export DOCKPIPE_VM_WINDOWS_KEYBOARD=en-US
export DOCKPIPE_VM_WINDOWS_TIMEZONE=UTC
export DOCKPIPE_VM_WINDOWS_COMPUTER_NAME=fixture-vm
export DOCKPIPE_VM_WINDOWS_ADMIN_PASSWORD=nonsecret-test-fixture
export DOCKPIPE_VM_WINDOWS_FULL_NAME=Fixture
export DOCKPIPE_VM_WINDOWS_ORG=Fixture
vmimage_windows_write_unattend
python3 - "$unattend_media/Autounattend.xml" "$bootstrap_media" <<'PY_XML'
import base64
from pathlib import Path
import sys
import xml.etree.ElementTree as ET

xml_path, bootstrap = map(Path, sys.argv[1:])
root = ET.parse(xml_path).getroot()
namespace = {'w': 'urn:schemas-microsoft-com:unattend'}
commands = root.findall('.//w:FirstLogonCommands/w:SynchronousCommand/w:CommandLine', namespace)
assert len(commands) == 1
prefix = 'powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand '
assert commands[0].text.startswith(prefix)
script = base64.b64decode(commands[0].text[len(prefix):]).decode('utf-16le')
assert "'provision-windows-ssh.ps1'" in script
assert '-InstallDockPipeAgent $true' in script
assert sorted(path.name for path in xml_path.parent.iterdir()) == ['Autounattend.xml']
assert (bootstrap / 'guest-agent-fixture.exe').stat().st_size == 4 * 1024 * 1024
PY_XML

# Staging replaces a custom public-key collision with the host-validated key.
custom_payload="$FIXTURE/custom-payload"
prepared_payload="$FIXTURE/prepared-payload"
mkdir -p "$custom_payload"
printf '%s\n' 'untrusted-key-fixture' > "$custom_payload/dockpipe-guest-ssh.pub"
vmimage_windows_bootstrap_dir() {
  printf '%s\n' "$prepared_payload"
}
vmimage_bootstrap_source_path() {
  printf '%s\n' "$custom_payload"
}
export DOCKPIPE_VM_BACKEND=qemu-kvm
export DOCKPIPE_VM_BOOT_SOURCE=image
export DOCKPIPE_VM_WINDOWS_SSH_PUBKEY='ssh-ed25519 validated-public-key-fixture'
prepared="$(vmimage_prepare_bootstrap_media)"
[[ "$prepared" == "$prepared_payload" ]]
[[ "$(cat "$prepared_payload/dockpipe-guest-ssh.pub")" == "$DOCKPIPE_VM_WINDOWS_SSH_PUBKEY" ]]
cmp "$ROOT/src/core/assets/scripts/provision-windows-ssh.ps1" "$prepared_payload/provision-windows-ssh.ps1"
[[ -f "$prepared_payload/README.txt" ]]
[[ ! -f "$prepared_payload/windows-ssh" ]]
[[ -f "$prepared_payload/Install-Dockpipe-Guest-Tools.cmd" ]]
[[ -f "$prepared_payload/autorun.inf" ]]

if command -v genisoimage >/dev/null 2>&1 && command -v isoinfo >/dev/null 2>&1; then
  guest_tools_iso="$(vmimage_prepare_bootstrap_iso "$prepared_payload")"
  [[ -s "$guest_tools_iso" ]]
  isoinfo -J -i "$guest_tools_iso" -x /autorun.inf > "$FIXTURE/cd-autorun.inf"
  cmp "$prepared_payload/autorun.inf" "$FIXTURE/cd-autorun.inf"
  isoinfo -J -i "$guest_tools_iso" -x /provision-windows-ssh.ps1 > "$FIXTURE/cd-helper.ps1"
  cmp "$prepared_payload/provision-windows-ssh.ps1" "$FIXTURE/cd-helper.ps1"
fi

echo 'Windows installation mode, guest-service bootstrap, and media separation tests passed'

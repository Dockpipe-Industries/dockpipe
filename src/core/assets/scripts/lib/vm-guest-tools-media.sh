# shellcheck shell=bash
# An optional resolver-owned, prebuilt interactive Guest Tools application.
# The canonical runtime provisioning helper remains at the media root.
vmimage_stage_resolver_guest_tools() {
  local destination="$1"
  local relative="${DOCKPIPE_RESOLVER_VM_GUEST_TOOLS_BUNDLE:-}"
  [[ -n "$relative" ]] || return 0
  case "$relative" in
    /*|*\\*|*:*|../*|*/../*|*/..|.|..)
      vmimage_die "Guest Tools bundle must be a contained relative directory"
      ;;
  esac
  local root="${DOCKPIPE_RESOLVER_PACKAGE_ROOT:-}"
  [[ -n "$root" ]] || vmimage_die "Guest Tools bundle requires its resolver package root"
  root="$(vmimage_shell_path "$root")"
  local source="${root}/${relative}" ancestor
  [[ -d "$source" ]] || vmimage_die "Installed Guest Tools bundle is missing"
  ancestor="$source"
  while [[ "$ancestor" != / && "$ancestor" != . ]]; do
    [[ ! -L "$ancestor" ]] || vmimage_die "Guest Tools bundle path must not contain links"
    ancestor="$(dirname "$ancestor")"
  done
  [[ -z "$(find "$source" ! -type f ! -type d -print -quit)" ]] ||
    vmimage_die "Guest Tools bundle contains a linked or special file"
  local file bytes count=0 total=0
  while IFS= read -r -d '' file; do
    count=$((count + 1))
    [[ "$count" -le 512 ]] || vmimage_die "Guest Tools bundle exceeds 512 files"
    bytes="$(wc -c < "$file")"
    total=$((total + bytes))
    [[ "$total" -le 536870912 ]] || vmimage_die "Guest Tools bundle exceeds 512 MiB"
  done < <(find "$source" -type f -print0)
  [[ -f "${source}/dockpipe-guest-tools.exe" && -f "${source}/install-guest-tools.ps1" ]] ||
    vmimage_die "Guest Tools bundle is incomplete"
  rm -rf "${destination}/GuestTools"
  mkdir -p "${destination}/GuestTools"
  vmimage_copy_tree_contents "$source" "${destination}/GuestTools"
  cat > "${destination}/Install-Dockpipe-Guest-Tools.cmd" <<'CMD'
@echo off
title Dockpipe Guest Tools
start "Dockpipe Guest Tools" "%~dp0GuestTools\dockpipe-guest-tools.exe"
CMD
  cat > "${destination}/autorun.inf" <<'AUTORUN'
[autorun]
label=Dockpipe Guest Tools
open=GuestTools\dockpipe-guest-tools.exe
action=Open Dockpipe Guest Tools
shell\install=Install or repair Dockpipe Guest Tools
shell\install\command=GuestTools\dockpipe-guest-tools.exe
AUTORUN
}

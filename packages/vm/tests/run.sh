#!/usr/bin/env bash
set -euo pipefail

PACKAGE_ROOT="${DOCKPIPE_PACKAGE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
VM_TEST_TMP="$(mktemp -d)"
trap 'find "$VM_TEST_TMP" -mindepth 1 -delete; rmdir "$VM_TEST_TMP"' EXIT
mkdir -p "$VM_TEST_TMP/cache" "$VM_TEST_TMP/tmp"

cd "$PACKAGE_ROOT/tools"
GOWORK=off GOCACHE="$VM_TEST_TMP/cache" GOTMPDIR="$VM_TEST_TMP/tmp" CGO_ENABLED=0 go test -mod=readonly ./...

cd "$PACKAGE_ROOT"
grep -Fq 'version: 0.7.0' package.yml
grep -Fq 'models/QemuVmResolverConfig' resolvers/qemu/types.yml
grep -Fq 'models/LinuxQemuVmResolverConfig' resolvers/qemu/types.yml
grep -Fq 'runtime: vm' workflows/windows-vm/config.yml
grep -Fq 'resolver: qemu' workflows/windows-vm/config.yml
grep -Fq 'runtime: vm' workflows/linux-vm/config.yml
grep -Fq 'resolver: qemu' workflows/linux-vm/config.yml
grep -Fq 'Qualification.Enabled: false' workflows/linux-vm/config.yml
grep -Fq '0533b0655c32e68b31d792ecd6ccfca95abdbc536c4446874fe0513bd4140ffe' profiles/ubuntu-24.04-amd64.yml

if grep -Eiq '(qemu-system|virsh|systemctl|poweroff|reboot|shutdown|SIGKILL|pidfd).*([[:space:]]run|[[:space:]]start|[[:space:]]send|[[:space:]]kill)' examples/linux-qualification-offline.yml; then
  echo 'offline qualification example contains a live or destructive command' >&2
  exit 1
fi

if grep -R -Fq 'bin/.dockpipe' workflows/linux-vm profiles manifests examples; then
  echo 'VM foundation must not fall back to bin/.dockpipe' >&2
  exit 1
fi

echo '[vm] package-owned Linux/QEMU foundation tests passed'

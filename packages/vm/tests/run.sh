#!/usr/bin/env bash
set -euo pipefail

PACKAGE_ROOT="${DOCKPIPE_PACKAGE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
VM_TEST_TMP="$(mktemp -d)"
trap 'find "$VM_TEST_TMP" -mindepth 1 -delete; rmdir "$VM_TEST_TMP"' EXIT
mkdir -p "$VM_TEST_TMP/cache" "$VM_TEST_TMP/tmp"

cd "$PACKAGE_ROOT/tools"
GOWORK=off GOCACHE="$VM_TEST_TMP/cache" GOTMPDIR="$VM_TEST_TMP/tmp" CGO_ENABLED=0 go test -mod=readonly ./...

cd "$PACKAGE_ROOT"
grep -Fq 'version: 1.3.2' package.yml
grep -Fq 'models/QemuVmResolverConfig' resolvers/qemu/types.yml
grep -Fq 'models/LinuxQemuVmResolverConfig' resolvers/qemu/types.yml
grep -Fq 'runtime: vm' workflows/windows-vm/config.yml
grep -Fq 'resolver: qemu' workflows/windows-vm/config.yml
grep -Fq 'runtime: vm' workflows/linux-vm/config.yml
grep -Fq 'resolver: qemu' workflows/linux-vm/config.yml
grep -Fq 'Qualification.Enabled: false' workflows/linux-vm/config.yml
grep -Fq '0533b0655c32e68b31d792ecd6ccfca95abdbc536c4446874fe0513bd4140ffe' profiles/ubuntu-24.04-amd64.yml
grep -Fq 'dockpipe.vm.qualification.v2' manifests/linux-qualification.json
grep -Fq '"boot_id_source": "/proc/sys/kernel/random/boot_id"' manifests/linux-qualification.json
grep -Fq 'dockpipe.vm.v2' tools/internal/protocol/frame.go
grep -Fq 'FirstRequestSequence' tools/internal/protocol/frame.go
grep -Fq 'dockpipe.vm.provisioning.v3' manifests/linux-provisioning.template.json
grep -Fq '"guest_verification_timeout_seconds": 300' manifests/linux-provisioning.template.json
grep -Fq 'dockpipe.vm.live-authorization.v3' manifests/linux-live-authorization.template.json
grep -Fq 'dockpipe.vm.cleanup-authorization.v1' manifests/linux-cleanup-authorization.template.json
grep -Fq 'dockpipe.vm.identity-material.v1' tools/internal/identitymaterial/material.go
grep -Fq 'dockpipe.vm.executor.v11' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.executor.v10' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.guest-verification-failure.v1' tools/internal/executor/runner_linux.go
grep -Fq 'dockpipe.vm.executor.v9' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.executor.v8' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.executor.v7' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.executor.v6' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.executor.v5' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.executor.v4' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.executor.v3' tools/internal/executor/contract.go
grep -Fq 'dockpipe.vm.first-boot-observation.v1' tools/internal/provisioning/first_boot_observation.go
grep -Fq -- '--execute-qualification' tools/cmd/dockpipe-qemu-controller/main.go
grep -Fq 'configuration-sha256' tools/cmd/dockpipe-qemu-controller/main.go
grep -Fq -- '--cleanup-executor' tools/cmd/dockpipe-qemu-controller/main.go
grep -Fq 'execute-gate3' tools/cmd/dockpipe-qemu-controller/main.go
grep -Fq 'gate3-reconstitute-executor' tools/cmd/dockpipe-qemu-controller/main.go
grep -Fq 'dockpipe.vm.gate3-reconstitution.v1' tools/internal/executor/reconstitution.go
grep -Fq 'dockpipe.vm.gate3-plan.v2' tools/internal/executor/gate3.go
grep -Fq 'reconstituted Gate 3 plans are inert and cannot execute' tools/internal/executor/gate3.go
grep -Fq 'gate3-inputs' tools/internal/executor/retained_inputs.go
grep -Fq 'provisioning-contract.json' tools/internal/executor/retained_inputs.go
grep -Fq 'qualification-manifest.json' tools/internal/executor/retained_inputs.go
grep -Fq 'dockpipe.vm.gate3-authorization.v1' manifests/linux-gate3-authorization.template.json
grep -Fq '"approved": false' manifests/linux-gate3-authorization.template.json
grep -Fq 'after-validation-before-ack' tools/internal/executor/gate3.go
grep -Fq 'PidfdSendSignal' tools/internal/executor/gate3_runner_linux.go
grep -Fq 'dockpipe.vm.toolchain.v1' toolchains/qemu-11.0.3-linux-amd64/toolchain.evidence.json
grep -Fq 'dockpipe.vm.toolchain-build-evidence.v1' toolchains/qemu-11.0.3-linux-amd64/build-contract.evidence.json
grep -Fq '"execute": false' toolchains/qemu-11.0.3-linux-amd64/build-contract.evidence.json
grep -Fq '11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c' manifests/linux-provisioning.template.json
grep -Fq '3544680aaeaf8087bbf3ef693ff185c2691831560c767672defccd784ec37140' manifests/linux-qualification.json
grep -Fq '"approved": false' manifests/linux-live-authorization.template.json
grep -Fq -- '--serve-virtio-serial=' workflows/linux-vm/assets/systemd/dockpipe-agent.service
grep -Fq 'ATTR{name}=="org.dockpipe.agent.1", GROUP="dockpipe-agent", MODE="0660"' workflows/linux-vm/assets/udev/99-dockpipe-agent.rules
grep -Fq '/etc/udev/rules.d/99-dockpipe-agent.rules' tools/internal/provisioning/render.go
grep -Fq 'package_update: false' workflows/linux-vm/assets/nocloud/user-data
grep -Fq 'ssh_pwauth: false' workflows/linux-vm/assets/nocloud/user-data
if grep -Fq 'ssh_redirect_user:' workflows/linux-vm/assets/nocloud/user-data; then
  echo 'system users must not declare cloud-init ssh_redirect_user' >&2
  exit 1
fi
grep -Fq 'defer: true' tools/internal/provisioning/render.go
grep -Fq '[/usr/bin/chgrp, --dereference, dockpipe-agent, /dev/virtio-ports/org.dockpipe.agent.1]' workflows/linux-vm/assets/nocloud/user-data
grep -Fq '[/usr/bin/chmod, "0660", /dev/virtio-ports/org.dockpipe.agent.1]' workflows/linux-vm/assets/nocloud/user-data
grep -Fq 'dockpipe-data-000001' manifests/linux-qualification.json

if grep -Fq '"boot_id":' manifests/linux-qualification.json; then
  echo 'qualification manifest must learn boot ID from the signed guest bootstrap' >&2
  exit 1
fi

if grep -R -Eq 'REPLACE_(WITH|BUILD)' toolchains/qemu-11.0.3-linux-amd64/*.evidence.json; then
  echo 'finalized QEMU evidence must not contain unresolved placeholders' >&2
  exit 1
fi

guest_exec_files="$(grep -R -El 'os/exec|exec\.Command|syscall\.Exec' tools/internal/guest --include='*.go' || true)"
if [[ "$guest_exec_files" != "tools/internal/guest/harness_linux.go" ]]; then
  echo "guest subprocess execution must exist only in the hash-pinned harness adapter: $guest_exec_files" >&2
  exit 1
fi

executor_exec_files="$(grep -R -El 'os/exec|exec\.Command|syscall\.Exec' tools/internal/executor --include='*.go' || true)"
expected_executor_exec_files=$'tools/internal/executor/runner_linux.go\ntools/internal/executor/gate3_runner_linux.go'
if [[ "$executor_exec_files" != "$expected_executor_exec_files" ]]; then
  echo "subprocess execution must exist only in the typed Linux runners: $executor_exec_files" >&2
  exit 1
fi

grep -Fq 'cmd.Env = []string{}' tools/internal/executor/runner_linux.go

if grep -Eiq '(qemu-system|virsh|systemctl|poweroff|reboot|shutdown|SIGKILL|pidfd).*([[:space:]]run|[[:space:]]start|[[:space:]]send|[[:space:]]kill)' examples/linux-qualification-offline.yml; then
  echo 'offline qualification example contains a live or destructive command' >&2
  exit 1
fi

if grep -R -Fq 'bin/.dockpipe' workflows/linux-vm profiles manifests examples; then
  echo 'VM foundation must not fall back to bin/.dockpipe' >&2
  exit 1
fi

echo '[vm] package-owned Linux/QEMU foundation tests passed'

# DockPipe VM package

The `vm` package owns guest-specific workflows, QEMU resolver models, and the
VMM-neutral control protocol. DockPipe core remains generic. Version 1.1.0
adds the sealed first-boot console observation path to the package-owned Linux
qualification runner alongside the unchanged `windows-vm` surface.

The Linux foundation has two deliberately different paths:

- `linux-vm` composes the generic `runtime: vm` and `resolver: qemu` for
  ordinary user-owned development images.
- qualification execution remains separately authorized and disabled by every
  checked-in template. The controller owns the sealed production path, but no
  VM operation occurs without a fresh exact live authorization.

The immutable Ubuntu profile is `profiles/ubuntu-24.04-amd64.yml`. Downloads are
never implicit: the official release-stamped URL and SHA-256 must both match,
and an override path still requires its own explicit SHA-256.

## XDG layout

| Material | Location |
| --- | --- |
| Images | `${XDG_CACHE_HOME:-$HOME/.cache}/dockpipe/vm/images` |
| Instances | `${XDG_STATE_HOME:-$HOME/.local/state}/dockpipe/vm/instances` |
| Evidence | `${XDG_STATE_HOME:-$HOME/.local/state}/dockpipe/evidence` |
| Configuration and public pins | `${XDG_CONFIG_HOME:-$HOME/.config}/dockpipe/vm` |
| Sockets and process records | `$XDG_RUNTIME_DIR/dockpipe/vm` |

`XDG_RUNTIME_DIR` is mandatory. There is no checkout, `.dorkpipe`, or generated
store fallback. Private keys and recovery tickets use owner-only permissions.

## Package tools

`tools/cmd/dockpipe-guest-agent` and
`tools/cmd/dockpipe-qemu-controller` are package-owned tools. The controller can
validate a manifest, verify the exact cached source image, and print a
deterministic inert provisioning or QEMU argv plan. Planning also verifies the
six NoCloud/systemd inputs against reviewed hashes compiled into the controller.
A separate short-lived authorization must bind to both the exact contract and
complete typed-plan digests. `--execute-qualification` consumes only that
authorized inert plan through the sealed typed runner; it is not a generic
command surface. The guest binary implements the systemd-referenced virtio-serial
service mode and the protocol-v2 boot-identity bootstrap. Before reading any
controller request, it reads the actual kernel boot ID from
`/proc/sys/kernel/random/boot_id` and writes one canonical, guest-signed
`bootstrap`/`identity/v1` frame. That frame is sequence 1 and is bound to the
fresh launch bootstrap nonce, all static run/machine/disk/scenario/boundary
identities, and both key and binary pins. The controller can therefore learn
the boot ID only after authenticating the pinned guest key; no unsigned frame
or pre-launch boot-ID claim is authoritative.

After that bootstrap, controller-signed requests start at sequence 2 and carry
the authenticated boot ID. The guest verifies controller signature, identity,
contiguous sequence, fresh non-reused nonce, lifetime, capability, and
binary/key pins before returning a guest-signed response. Identity, health, and
hash-pinned-launch are operational; only signed `request` frames are accepted
after the bootstrap, and checkpoint and recovery recognize only the
reviewed signed payload shape and fail closed because the Gate 2 foundation
does not own a harness adapter. No other capability or arbitrary execution
surface exists.

`manifests/linux-provisioning.template.json` is deliberately non-runnable. A
live gate must replace every marker with fresh identities, a 32-byte launch
bootstrap nonce, the current XDG
runtime root, task-owned binary paths and hashes, and mutually pinned fresh
keys. Fresh keypairs are generated in memory first, their public hashes are
bound into the contract and plan, and only those same keys may then be reserved
exclusively. The controller rejects relative, checkout, `.dockpipe`, `.dorkpipe`,
pre-existing, mismatched, expired, or substituted inputs.

`--prepare-identity-material` implements the reviewed cross-authorization key
lifecycle. It generates the nonce and both keypairs in memory, then durably
creates exactly five owner-only files under a new mode-`0700` task staging root
outside the checkout and live XDG roots. Gate 2 binds the emitted public hashes
and nonce into provisioning v3. The live invocation reloads and validates the
exact keypairs, reserves them durably under the final configuration root, and
only then consumes the staging copy. A failure before that point preserves the
staging bundle; a later failure preserves the final configuration root.
The staging descriptor expires exactly 24 hours after creation. Expiry never
deletes material automatically; it requires explicit removal and fresh preparation.

The executor-v3 verification request requires the controller to read and verify
the bootstrap before it writes anything to the stream. Acceptance requires the
exact pinned guest signature, fresh time window, bootstrap nonce, sequence 1,
phase `bootstrap`, static identity tuple, kernel boot-ID source, and key/binary
pin payload. Before request sequence 2, the controller must exclusively create
owner-only `bootstrap.json`, write the verified frame and learned boot ID, and
fsync both file and parent directory. Existing evidence, any mismatch, or any
write/fsync failure stops once with complete preservation and no retry.

The provisioning contract also requires a separate task-owned immutable QEMU
`11.0.3` Linux/amd64 bundle. Its exact owner-only root, manifest digest,
`qemu-img`, `qemu-system-x86_64`, and complete runtime-library/ROM/data inventory
are hash-pinned. Validation rejects `PATH`, checkout/generated-root overlap,
symlinks, extra files, widened modes, version/hash substitution, and fallback
tools. Gate 1 materialized the exact bundle at
`/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1`, pinned by
manifest SHA-256
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`.
The exact manifest and build evidence live under
`toolchains/qemu-11.0.3-linux-amd64/`; they do not wire package installation,
release, registry, signing, or version-resolution backlog.

`tools/internal/executor` derives a closed execution contract only from the
authorized provisioning contract and plan digest. Its injected runner exposes
only typed OS-clone, sparse-raw, NoCloud-seed, QEMU-launch, signed-verification,
controlled-shutdown, preservation, and exact-cleanup methods. It has no generic
command, shell, environment, network, SSH, passthrough, share, physical-disk,
or fallback method. The Linux production adapter uses an empty environment and
only the contract-pinned absolute `qemu-img` and `qemu-system-x86_64` argument
vectors. It creates the raw disk and deterministic ISO in Go, performs the
guest-first signed exchange, sends only QMP `system_powerdown`, preserves every
failure root, and never retries or cleans automatically. The first authorized
Gate 2 attempt on 2026-08-08 failed closed at `launch-qemu` before its sockets
became ready: the reviewed QMP and agent pathname sockets were 174 and 176
bytes, exceeding Linux's 107-byte `sockaddr_un` pathname limit. The four live
roots were preserved, the exact owned QEMU process was no longer active, and a
separately authorized cleanup removed only the executor-bound 11-resource
list. Planning now rejects either overlength socket path before authorization.
After that correction, one fresh Gate 2 invocation reached `verify-guest` and
failed closed when no complete four-byte-length-prefixed signed bootstrap
arrived within the single 60-second deadline. QEMU created both exact sockets
and the controller connected to the agent chardev, but bootstrap verification,
evidence creation, and controller requests were never reached. A separately
authorized read-only disk inspection found no `/var/lib/cloud` state and no
agent-service journal entries, so the earliest missing observable milestone is
cloud-init/first-boot state; the guest-side cause remains unproven.

`dockpipe.vm.first-boot-observation.v1` is now bound into every fresh
provisioning plan, its digest, executor-v3, and the exact QEMU argv. QEMU exposes
only the existing `isa-serial/ttyS0` stream as a one-shot Unix client; before
launch, the controller creates the exact listener and an exclusive mode-`0600`
`first-boot-console.log`. The controller retains at most the first 4 MiB,
fails closed on the next byte, fsyncs the retained prefix and evidence
directory, and closes and joins capture before verification returns, shutdown,
or failure preservation. Listener-setup failure also propagates listener-close,
sink-sync, sink-close, and parent-directory-sync errors. Path, transport, role,
reconnect, mode, cap, overflow, and lifecycle substitutions are rejected by
planning and sealed executor validation. The path adds no NoCloud or guest
mutation, private-payload read, network, shell, retry, reconnect, signal,
fallback, or automatic cleanup.

Fresh qualification execution now requires executor-v3 with that exact policy.
Preserved executor-v2 files remain loadable only for their separately
authorized exact cleanup resource lists; they cannot regain qualification
execution, and an independent historical-v2 serialization test pins their
original digest shape. The checked-in live authorization remains disabled and
the plan remains `execute=false`. No new live identity, root, disk, seed,
process, socket, or evidence was created by this offline slice. Gate 2 remains
unqualified. The subsequent offline source-build review produced two
independent byte-identical Linux/amd64 builds from the package-owned source with
`GOWORK=off`, `CGO_ENABLED=0`, `-trimpath`, and `-buildvcs=false`. The controller
SHA-256 is `b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc`,
the Linux guest-agent SHA-256 is
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`,
and the Windows/amd64 guest-agent compatibility build SHA-256 is
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`.
The Windows output is cross-compilation evidence only. None of these outputs
was promoted or copied into a live or preserved root. A fresh separately
authorized Gate 2 invocation is the explicit next live gate, and Gate 3 remains
blocked.

`manifests/linux-live-authorization.template.json` is separately inert with
`approved=false`. A later reviewed gate must bind a fresh, short-lived copy to
the emitted contract and plan SHA-256 values; the offline controller still
leaves the reviewed plan itself `execute=false`. A live invocation selects the
typed executor explicitly. `manifests/linux-cleanup-authorization.template.json`
is a second inert authorization bound to the exact executor digest and ordered
resource list; cleanup is never implied by execution success or failure.

The production CLI modes are mutually exclusive:

```text
--prepare-identity-material <absolute-root> --run-id <id> --cohort-id <id>
--validate-manifest <file> --configuration-sha256
--validate-manifest <file> --plan-provisioning <file> [--live-authorization <file>]
--validate-manifest <file> --plan-provisioning <file> --live-authorization <file> --identity-material <absolute-root> --execute-qualification
--cleanup-executor <executor.json> --cleanup-authorization <file>
```

Run package tests with:

```bash
./src/bin/dockpipe package test --workdir . --only vm
```

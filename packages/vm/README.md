# DockPipe VM package

The `vm` package owns guest-specific workflows, QEMU resolver models, and the
VMM-neutral control protocol. DockPipe core remains generic. Version 0.9.0 adds
the offline task-owned QEMU toolchain and typed executor contract alongside the
unchanged `windows-vm` surface.

The Linux foundation has two deliberately different paths:

- `linux-vm` composes the generic `runtime: vm` and `resolver: qemu` for
  ordinary user-owned development images.
- qualification manifests, protocol code, provisioning and QEMU plan
  generation, recovery tickets, executor requests, and cleanup plans are
  offline-only. No package binary in this slice starts, stops, reboots, kills,
  provisions, or removes a VM or disk.

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
`tools/cmd/dockpipe-qemu-controller` are source foundations. The controller can
validate a manifest, verify the exact cached source image, and print a
deterministic inert provisioning or QEMU argv plan. Planning also verifies the
six NoCloud/systemd inputs against reviewed hashes compiled into the controller.
A separate short-lived
authorization must bind to both the exact contract and complete typed-plan
digests, but the plan remains
`execute=false`: this slice contains no subprocess runner or live CLI execution
flag. The guest binary now implements the systemd-referenced virtio-serial
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

The executor-v2 verification request requires the controller to read and verify
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
or fallback method. Tests use a fake runner; production subprocess execution is
still deliberately absent. Gate 2 and Gate 3 have not started.

`manifests/linux-live-authorization.template.json` is separately inert with
`approved=false`. A later reviewed gate must bind a fresh, short-lived copy to
the emitted contract and plan SHA-256 values; the offline controller still
leaves `execute=false` after validating it.

Run package tests with:

```bash
./src/bin/dockpipe package test --workdir . --only vm
```

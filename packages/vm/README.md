# DockPipe VM package

The `vm` package owns guest-specific workflows, QEMU resolver models, and the
VMM-neutral control protocol. DockPipe core remains generic. Version 0.8.0 adds
the offline Gate 2 provisioning foundation alongside the unchanged
`windows-vm` surface.

The Linux foundation has two deliberately different paths:

- `linux-vm` composes the generic `runtime: vm` and `resolver: qemu` for
  ordinary user-owned development images.
- qualification manifests, protocol code, provisioning and QEMU plan
  generation, recovery tickets, and cleanup plans are offline-only. No package
  binary in this slice starts, stops, reboots, kills, provisions, or removes a
  VM or disk.

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
`execute=false`: this slice contains no subprocess executor. The guest binary
now implements the systemd-referenced virtio-serial service mode. It verifies
the controller signature, identity, sequence, nonce, lifetime, capability, and
binary/key pins before returning a guest-signed response. Identity, health, and
hash-pinned-launch are operational; only signed `request` frames are accepted,
and checkpoint and recovery recognize only the
reviewed signed payload shape and fail closed because the Gate 2 foundation
does not own a harness adapter. No other capability or arbitrary execution
surface exists.

`manifests/linux-provisioning.template.json` is deliberately non-runnable. A
live gate must replace every marker with fresh identities, the current XDG
runtime root, task-owned binary paths and hashes, and mutually pinned fresh
keys. Fresh keypairs are generated in memory first, their public hashes are
bound into the contract and plan, and only those same keys may then be reserved
exclusively. The controller rejects relative, checkout, `.dockpipe`, `.dorkpipe`,
pre-existing, mismatched, expired, or substituted inputs.

`manifests/linux-live-authorization.template.json` is separately inert with
`approved=false`. A later reviewed gate must bind a fresh, short-lived copy to
the emitted contract and plan SHA-256 values; the offline controller still
leaves `execute=false` after validating it.

Run package tests with:

```bash
./src/bin/dockpipe package test --workdir . --only vm
```

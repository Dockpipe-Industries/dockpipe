# Linux VM workflow and qualification foundation

`linux-vm` is the first-party Linux workflow over the existing generic VM
runtime and the `qemu` resolver. Its runnable path is development-only. The
qualification fields are a typed contract for offline validation and are fixed
to `Qualification.Enabled: false` until a separate integration and live gate.

## Immutable Ubuntu profile

The reviewed profile is Ubuntu 24.04 LTS amd64 release stamp `20260801`:

- image: `ubuntu-24.04-server-cloudimg-amd64.img`
- SHA-256: `0533b0655c32e68b31d792ecd6ccfca95abdbc536c4446874fe0513bd4140ffe`
- provisioning source: local NoCloud only
- network and SSH: disabled for qualification
- `apt` update/upgrade: disabled; no additional debs are requested

No image is downloaded by a workflow or package test. A future download gate
must fetch the exact immutable URL into the XDG image cache, hash before use,
and reject a user override without its own checksum.

## Qualification contract

`manifests/linux-qualification.json` is an illustrative, deliberately
non-runnable manifest. Validation requires purpose `qualification`,
`disposable=true`, KVM (TCG is rejected), host CPU, two vCPUs, 4096 MiB, no
swap, exactly two private disks, and no network, SSH, passthrough, physical
disk, share, extra disk, or arbitrary command capability. Host and guest
machine identities must differ.

The manifest does not prescribe a boot UUID. Schema
`dockpipe.vm.qualification.v2` fixes only
`boot_id_source=/proc/sys/kernel/random/boot_id`; the actual per-boot value is
learned through the authenticated guest-first protocol below.

The data disk is a 4 GiB sparse raw whole-device ext4 filesystem. Its reviewed
creation tuple is:

```text
mkfs.ext4 -F -L dockpipe-qual -U <manifest-uuid> -E lazy_itable_init=0,lazy_journal_init=0 /dev/disk/by-id/virtio-<manifest-serial>
```

It mounts by UUID at `/var/lib/dockpipe-qualification` with exactly
`rw,noatime,nodev,nosuid,noexec,data=ordered`. Discard, DAX, `nobarrier`, bind,
remount, nested filesystems, overlays, encryption, lazy initialization, and
unexpected ext4 features are rejected. Only qualification tickets, SQLite
roots, and results belong there.

QEMU planning records the binary version and SHA-256, configuration SHA-256,
both host backing identities, node names, serials, filesystem identity, mkfs
version, host and guest kernel releases, ext4 features, and mount ID.
Both disks use persistent block nodes, `cache=none`, flush preservation,
guest write-cache exposure, disabled discard/zero detection, and stop-on-error.
Threaded AIO is the default; native AIO requires an explicitly reviewed tuple.
There is no silent fallback, and QEMU 6.2 is not claimed as qualified.

## Signed protocol and recovery

Each instance has new Ed25519 controller and guest keys with mutually pinned
public-key hashes. Frames use a four-byte big-endian length prefix, a 64 KiB
maximum frame, a 32 KiB maximum canonical-JSON payload, a five-minute maximum
lifetime, and 30 seconds of clock skew. Unknown or duplicate keys, extra data,
noncanonical JSON, oversized data, invalid signatures, stale windows, replayed
nonces, out-of-order sequences, and identity substitution are rejected.

Every signature covers protocol version, capability, machine UUID, data-disk
serial, boot ID, sequence, run ID, nonce, scenario, durability boundary, phase,
time window, and payload. Qualification capabilities are exactly:
`identity/v1`, `health/v1`, `checkpoint/v1`, `recovery/v1`, and
`launch-hash-pinned/v1`.

Protocol `dockpipe.vm.v2` has one exact bootstrap. The fresh provisioning-v3
`bootstrap_nonce` is generated with the per-instance Ed25519 keys, bound into
the provisioning contract, inert plan, short-lived live authorization,
NoCloud guest configuration, and sealed executor-v2 digest. Once the guest has
read the reviewed kernel boot-ID path, it writes the first length-prefixed frame
before reading controller bytes. That canonical frame has kind `bootstrap`,
capability `identity/v1`, sequence 1, phase `bootstrap`, the launch bootstrap
nonce, actual boot ID, all static identity fields, and a payload containing the
boot-ID source plus both public-key and binary SHA-256 pins. It is signed only
by the pinned guest key; there is no unsigned identity or controller bootstrap
exception.

The controller must read first and verify framing, canonical JSON, freshness,
the pinned guest signature, exact bootstrap nonce, sequence/phase, static
machine/disk/run/scenario/boundary context, boot-ID UUID, boot-ID source, and all
four pins. It must then exclusively create mode-`0600` `bootstrap.json`, record
the verified frame and learned boot ID, and fsync the file and evidence
directory before writing any request. Existing evidence or any verification or
durability failure stops with preservation and no retry. The first
controller-signed `identity/v1` request uses that boot ID at sequence 2 with a
new nonce; later requests are contiguous and may not reuse the bootstrap or any
request nonce. Guest-signed results echo each request context. This second step
proves the guest's controller-key pin while retaining mutual Ed25519 pinning.

A pending signed ticket blocks all work other than the exact matching recovery.
Machine, disk, boot, run, scenario, boundary, nonce, and harness hash must all
match. The guest durably records `consumed` and the result hash before returning
one result. It never resends, retries, repairs, reopens, or resumes a consumed
ticket; a lost result fails the cohort. Stale/consumed cleanup is explicit and
outside qualification.

## Trial isolation and future gates

One VM is reserved per cohort, with a private OS clone and data disk. Run,
cohort, boundary, and attempt roots are unique and immutable; database,
journal, nonce, and ticket material are never reused. Recovery is sequential.
Any failure preserves the complete instance. Cleanup is only an inert plan
until it matches the exact run ID and ordered resource enumeration, and it
refuses completed roots.

The six systemd and NoCloud files under `assets/` are now exact renderer inputs
whose reviewed SHA-256 values are compiled into the controller.
Rendering requires binary hashes, mutually pinned Ed25519 keys, fresh
run/cohort/machine/disk/filesystem/bootstrap-nonce identities, and package XDG roots. The
keypairs are generated before authorization, their public hashes are included
in the contract and plan, and reservation refuses different key material. The
rendered seed disables network and SSH, requests no packages or apt changes,
formats only the exact virtio data-disk serial with the reviewed UUID/ext4
tuple, mounts only by that UUID, installs only the hash-pinned guest binary plus
reviewed systemd/config/key files, and starts the fixed agent service. There is
no user-provided command field.

The service is unprivileged, nologin, capability-free, private-networked,
ordered before network, and uses virtio-serial. Its implemented service mode
emits the one canonical signed bootstrap and then recognizes only canonical,
signed, length-prefixed `request` frames for the five reviewed capabilities.
Identity, health, and binary-pin verification respond; checkpoint
and recovery remain fail-closed until a separate reviewed harness adapter owns
their state transition. Signature, public-key pin, binary pin, freshness, replay,
sequence, identity, nonce, and payload substitution failures close the stream.
System state stays on the OS disk; tickets and results use the qualification
mount.

The controller's provisioning plan is a deterministic closed set of typed
operations: exclusive identity reservation; source verification; private OS
clone and 4 GiB sparse raw data-disk creation; exact NoCloud rendering and seed
creation; hash-pinned asset installation; stable format/mount; QEMU launch;
guest verification; controlled shutdown; failure preservation; and later exact
cleanup. Planning invokes no subprocess and the emitted plan is always
`execute=false`. A distinct, short-lived authorization file must authenticate
both the exact contract digest and the complete typed-plan digest but cannot
make this offline slice execute it. The package authorization template defaults
to `approved=false` and is not itself a live authorization.

The next offline contract layer binds that plan to one task-owned QEMU `11.0.3`
Linux/amd64 bundle. The bundle contains only hash-pinned `qemu-img`,
`qemu-system-x86_64`, and their exact runtime closure. `qemu-img` has one fixed
120-second private backing-clone argv; the controller owns exclusive 4 GiB
sparse-raw creation and deterministic `dockpipe-go-iso9660-v1` seed creation.
QEMU launch is bounded to 120 seconds, signed guest verification to 60 seconds,
and QMP `system_powerdown` shutdown to 120 seconds with no fallback signal.
Any failure stops once, preserves all four instance roots, and never retries or
cleans. Cleanup requires a separate fresh authorization bound to the exact
contract, plan, executor digest, and ordered resource list.

Gate 1 materialized this bundle at
`/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1`, with
manifest SHA-256
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`.
Two independent builds matched the same complete output inventory. Only an
injected typed runner exists in this slice, with fake-runner tests; no `os/exec`
adapter or live controller flag exists. Gate 2 and Gate 3 have not started.

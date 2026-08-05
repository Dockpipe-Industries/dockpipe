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

The systemd and NoCloud files under `assets/` are design assets only. The
service is unprivileged, nologin, capability-free, private-networked, ordered
before network, and uses virtio-serial. System state stays on the OS disk;
tickets and results use the qualification mount. Binary hashes must be pinned
before a later provisioning gate installs anything.

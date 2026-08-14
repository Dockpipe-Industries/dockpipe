# QEMU 11.0.3 Linux/amd64 toolchain contract

This directory owns the recipe and reviewed Gate 1 evidence for the task-owned
Linux/amd64 KVM bundle used by qualification. It does not connect the bundle to
DockPipe package installation, release, registry, or version-resolution work.
Those package-layer concerns remain separate backlog work.

The bundle contains exactly `qemu-img`, `qemu-system-x86_64`, and the complete
runtime library/ROM/data closure enumerated by `toolchain.json`. The controller
implements sparse raw-disk creation and deterministic NoCloud ISO9660
construction itself; `cloud-localds`, `xorriso`, `genisoimage`, shell lookup,
and fallback tools are not part of the contract.

Gate 1 completed from the digest-pinned QEMU Alpine builder on 2026-08-07. Two
independent builds produced the same 125-entry output inventory SHA-256,
`22f24ba020b98b0802d67956bd5d7699bcd9d12a99773e185165087b8b1aedec`.
The immutable result is
`/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1`; its
`toolchain.json` SHA-256 is
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`.
`toolchain.evidence.json` is the exact checked-in copy of that manifest, and
`build-contract.evidence.json` records the source/signature, builder closure,
controlled environment, configure argv, recipe hashes, and reproducibility
evidence without unresolved markers.

The bundle uses its own musl loader at the absolute immutable path, literal
`$ORIGIN/../lib` RPATH, and `NODEFLIB`; it requires neither `LD_LIBRARY_PATH`
nor host-library fallback. Every directory is mode `0500`, the two tools and
loader are `0500`, and libraries, ROM/data, and manifest are `0400`. All entries
are owned by `jamie:jamie`; there are no symlinks, writable entries, or
group/world permissions.

Gate 2 and Gate 3 have not started. No VM, VM disk, NoCloud seed, live root,
QEMU process, or cleanup action was created by Gate 1.

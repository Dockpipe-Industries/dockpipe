# QEMU 11.0.3 Linux/amd64 toolchain recipe

This recipe builds a task-owned Linux/amd64 KVM bundle containing `qemu-img`,
`qemu-system-x86_64`, and their complete runtime library/ROM/data closure. It does
not run a VM or connect the bundle to package installation or release.

Set these three explicit absolute paths before invoking the materializer:

- `DOCKPIPE_QEMU_SOURCE_DIR`: directory containing `qemu-11.0.3.tar.xz`, its
  `.sig`, `qemu-release-key-ubuntu.asc`, and `qemu-release-key.asc`. Input hashes,
  signing identity, and the builder image remain pinned in `build-spec.json`.
- `DOCKPIPE_QEMU_BUILD_ROOT`: a new directory for the two independent builds and
  their complete records.
- `DOCKPIPE_QEMU_FINAL_ROOT`: a new immutable toolchain directory. Its sibling
  `<final-root>.partial` must also be absent.

Paths must be separate, absolute, free of symlinks, and use letters, digits,
dots, underscores, hyphens, and slashes. Whitespace and Docker/linker separators
are rejected before any build or filesystem changes. No path defaults to the
maintainer's home, cache, checkout, or temporary source files.

```bash
# Set the three variables to your chosen directories first.
bash materialize.sh --check-config
# After verifying the paths and pinned inputs, materialize the bundle:
bash materialize.sh
```

`--check-config` validates and prints paths only; it does not create directories,
read source archives, call Docker, or start a build. Normal execution retains
signature verification, the pinned builder, two-build comparison, owner-only
publication, and refusal to overwrite existing build or output directories.

The host passes the selected final root explicitly into the isolated builder.
The ELF interpreter is pinned to that absolute root, while RPATH remains the
literal `$ORIGIN/../lib` and `NODEFLIB` prevents host-library fallback. The bundle
is buildable for another machine's chosen root; it is not relocatable afterward.
A different root or recipe requires fresh build evidence and hashes. Populate
provisioning from the resulting `toolchain.json`, never from a prior run.

`build-spec.json` describes the parameterized recipe; `${DOCKPIPE_QEMU_*}` tokens
are documentation of the required inputs, not implicit shell evaluation.
Actual configure arguments and environment are recorded by each build and
compared before publication. The source repository preserves historical Gate 1
JSON evidence in `docs/research/vm-toolchain-2026-08-07/`, outside shipped package
inputs. It does not establish reproducibility of this updated recipe.

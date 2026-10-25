# Release documentation and outputs

This directory keeps reviewed announcement content, maintainer documentation and
ignored artifact outputs. Build and release implementation lives beside the owning
[Dockpipe workflows](../workflows/release/README.md).

| Path | Contents |
| --- | --- |
| `docs/` | Branching, staging, publication and announcement guidance |
| [CLI candidates](cli-candidate.md) | Immutable standalone runtime bootstrap contract |
| [Go SDK](go-sdk.md) | Runtime contracts consumed by external Go packages |
| [Display SDK](display-sdk.md) | Native QWidget interface artifact contract |
| `articles/announcement.md` and `articles/assets/` | Reviewed dev.to announcement and its public images |
| `artifacts/` | Gitignored CLI, installer, manifest, checksum and release-notes outputs |

`VERSION` selects the release line and numeric floor. The release workflow generates
the next available version and a notes header with the exact tag, source and install
guide. GitHub appends the merged-PR changelog during publication. Dry runs retain the
header without calling the write-permission notes API. There is no per-version notes
file to maintain. The optional dev.to announcement remains separately reviewed content.

Workflow-specific scripts, tests, assets and dependency locks belong under
`workflows/`; shared workflow helpers belong under `workflows/shared`.
Make and GitHub Actions launch those workflows. There is no core archive producer
or required core package. Optional package catalogs release independently from
runtime binaries. See [runtime resource ownership](../docs/concepts/runtime-resource-ownership.md).

The publication workflows upload qualified immutable artifacts to the explicitly
selected GitHub and R2 destinations. Keep operator configuration and secret values
outside the public checkout; use workflow inputs, repository/environment variables
and secret references. Existing releases, downloads and version catalogs are
retained. See [releasing](docs/releasing.md), [staging](docs/staging.md) and the
[user installation guide](../docs/install.md).

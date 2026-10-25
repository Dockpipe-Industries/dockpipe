# Release documentation and outputs

This directory keeps release notes, articles, maintainer documentation and ignored
artifact outputs. Build and release implementation lives beside the owning
[Dockpipe workflows](../workflows/release/README.md).

| Path | Contents |
| --- | --- |
| `releasenotes/` | Per-version release bodies consumed by the release workflow |
| `docs/` | Branching, staging, publication and announcement guidance |
| [CLI candidates](cli-candidate.md) | Immutable standalone runtime bootstrap contract |
| [Go SDK](go-sdk.md) | Runtime contracts consumed by external Go packages |
| [Display SDK](display-sdk.md) | Native QWidget interface artifact contract |
| `articles/` and `demo/` | Public release media and recording guidance |
| `artifacts/` | Gitignored CLI, installer, manifest and checksum outputs |

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

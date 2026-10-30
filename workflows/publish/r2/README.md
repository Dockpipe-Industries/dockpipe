# Runtime R2 publication

The authored `config.yml`, `release.yml` and `staging.yml` workflows prepare a
manifest, call the installed `dockpipe-cloudflare` package's
`dockpipe.cloudflare.r2bulk` workflow, and publish ordered metadata only after
all immutable payloads pass its HEAD verification. Use the released CLI selected
by `workflows/shared/bootstrap.lock.json` and the released package and pinned
Python SDK selected by `release/r2bulk.lock.json`. That schema-1 lock declares
`package`, `version`, immutable `manifest_url`, `manifest_sha256`,
`archive_sha256` and `requirements_lock_sha256`.

Bootstrap explicitly downloads the locked public GitHub manifest and archive
with Python's standard library. Its public transport sends no authentication,
uses HTTPS only, permits at most three redirects among `github.com`,
`release-assets.githubusercontent.com` and `objects.githubusercontent.com`,
and bounds request time and download sizes. Both downloaded digests must match
the lock. The catalog must contain exactly the named version and archive.

The runtime-owned `r2bulk-runtime` artifact scope caches the verified archive.
The project must declare this supported package source in `dockpipe.config.json`
before starting the workflow, preserving its other configuration:

```json
{"packages": {"sources": [{"kind": "store", "path": ".toolchains/r2bulk/packages"}]}}
```

Keep `/.toolchains/` ignored. Bootstrap validates that exact declaration and
materializes the verified compiled workflow under
`.toolchains/r2bulk/packages/workflows/dockpipe.cloudflare.r2bulk`. It accepts only
bounded regular files and directories within the expected archive prefix,
rejecting links, traversal and duplicate members. A temporary directory becomes
visible through a final directory rename. A receipt binds the archive SHA and
each file digest; existing cache contents must match before reuse and conflicts
fail without replacement. The resolver sees the declared store at nested-workflow
resolution time. Bootstrap exports `R2_BULK_PYTHON`; it does not depend on a step
output overriding the process-level package root. No sibling checkout or implicit
source fallback is selected. Verified requirements bytes also populate the SDK
runtime scope.
The Python interpreter running bootstrap must provide working `venv` and
`ensurepip` support. Select that interpreter explicitly; bootstrap does not
install operating-system packages or substitute another Python interpreter.

Bulk objects contain an absolute source path, destination key, byte size,
SHA-256, content type and cache control. They include versioned release files
except `release-manifest.json`, plus APT pool files and by-hash indexes. Staging
includes both candidate-specific and permanent APT immutable payload keys.
The package may skip matching remote objects; conflicting existing objects fail.
It uses bounded defaults of four objects and one multipart request each.

The original metadata order remains: candidate or stable APT ordinary indexes,
`Release`, `Release.gpg`, `InRelease`; release catalog; staging permanent APT
indexes and signed metadata; runtime `latest.json`. A bulk failure prevents every
metadata upload. The metadata phase additionally requires the package's
`verified-head` state for the exact manifest identity, selected bucket and endpoint. This verifies HEAD
and locally hashed source bytes; it does not claim a full remote GET.

`--dry-run` remains available through `config.yml` arguments. It validates local
inputs, creates the plan, invokes the provider-free package dry run and prints
ordered metadata operations without provider calls. Existing release/channel,
version advancement, symlink, signed APT and unpublished catalog guards remain.
Plans and durable resume journals use the runtime-owned `r2-publication`
artifact scope; SDK virtual environments use the `r2bulk-runtime` artifact scope.

The caller supplies the bucket, HTTPS R2 endpoint and credentials. No deployment
location or account identity is embedded in this public workflow.

The consumer runner is pinned to released CLI `0.6.33`, including archive hashes
for every existing target. Older `0.6.25` and `0.6.29` runners do not resolve this
declared nested package store. Package compilation remains separately qualified
with `0.6.25`; installed-consumer execution uses the newer pinned runner.

# Go compatibility SDK release

Dockpipe owns the Go SDK consumed by independently released catalog tools.
`go-sdk-source.lock.json` identifies an exact committed source snapshot, source
inventory, archive and manifest. The SDK includes the generic infrastructure,
file publication and remote execution contracts. Package and compiler
implementations are excluded.

The **Go compatibility SDK** workflow checks out that exact source commit and
runs the current archive producer against it. It verifies the resulting archive
and manifest against the committed content lock before uploading build artifacts.
Dependency installation precedes the offline source discovery and archive export.
A changed producer or dependency closure must reproduce the same bytes, or the
lock must be deliberately updated with new consumer evidence.

Run `python3 workflows/build/go-sdk/assets/scripts/build-locked.py --source-root /path/to/committed-snapshot --output /fresh/output`
to reproduce locally after installing the snapshot's Go module dependencies.
The source directory must contain the locked snapshot; it is an explicit producer
input and is never a consumer fallback.

A normal manual workflow run only creates CI artifacts. Setting the `publish`
input requests a release job behind the `release` environment. That job creates
an immutable `dockpipe-go-sdk-<version>` GitHub Release at the locked source
commit; an existing release causes creation to fail rather than replacing assets.
Repository environment approval settings remain responsible for approval policy.

Catalog consumers pin this core repository, exact tag, archive and manifest
checksums. They authenticate the downloaded assets before importing the SDK.
Prepared local artifacts do not establish that a GitHub Release exists. Initial
publication still requires separate authorization under the repository's objective
and session rules.

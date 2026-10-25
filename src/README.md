# `src/` — source tree

- **`cmd/`** — Dockpipe CLI entrypoint (`main.go`).
- **`lib/`** — generic runtime, application, domain, infrastructure and host SDK code.
- **`app/`** — native desktop host and its tooling.
- **`bin/`** — generated developer CLI binaries; ignored by Git.

`go.mod` and `go.sum` stay at the repository root, the module boundary for
`dockpipe/src/...`. `go.work` and `go.work.sum` describe the root workspace.
There are no root Go source files or authored workflow embeds.
Generic runtime resources live beside their owning library packages, including
`src/lib/runtimeassets/containers/entrypoint.sh`.

Builds and tests run through the owning Dockpipe workflows. From the repository
root, the optional Make adapter is `make -f tooling/Makefile build`.
See [tooling](../tooling/README.md) for the adapter and direct workflow commands.

Local development builds report `dev+<12-character-commit>` with `.dirty` when
built from uncommitted changes. Without embedded VCS metadata they report
`dev+unknown`; the running binary never queries the checkout. Release builds
retain the explicit `-X main.Version=<version>` identity. The Make adapter accepts
`BUILD_VERSION=<version>`; `DEB_VERSION` controls installer packaging separately.

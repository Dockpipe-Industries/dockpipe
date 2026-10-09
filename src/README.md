# `src/` — source tree

- **`cmd/`** — Dockpipe CLI entrypoint (`main.go`)
- **`cmd/dorkpipe`** — DorkPipe DAG orchestrator CLI
- **`lib/`** (`application`, `domain`, `infrastructure`) — Dockpipe library
- **`lib/dorkpipe`** — DorkPipe library
- **`bin/`** — Launcher scripts (`dockpipe`, `pipeon`, …) and **`make` outputs** (`dockpipe.bin`, `dorkpipe`, `dockpipe.exe`)
- **`Makefile`** — `build`, `build-windows`, `test` (included from the **repository root** `Makefile`)

**`go.mod`** stays at the repository root (same module: **`dockpipe`**). **`embed.go`** also stays at the root so `//go:embed` can include `src/core/`, `assets/entrypoint.sh`, and `VERSION` without `..` paths.

Run **`make`** from the **repository root** — it includes `src/Makefile` for Go targets.

Local `go build` and `make build` report `dev+<12-character-commit>` with `.dirty`
when the checkout had uncommitted changes at build time. Without embedded VCS metadata,
they report `dev+unknown`. The running binary never queries the current checkout.
Release builds keep their explicitly supplied `-X main.Version=<version>` identity;
`make build-core BUILD_VERSION=<version>` provides the equivalent override. `DEB_VERSION`
controls installer packaging, not the identity of ordinary development builds.

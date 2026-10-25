# Contributing

Primitive first: run in container, optionally act after. Keep the core minimal.

Follow the [coding style and reliability standard](docs/coding-style.md) for readable
responsibilities, resource bounds, cancellation, recovery and failure-path tests.

Dockpipe owns the generic engine, CLI, desktop host and public SDK contracts.
Remaining catalog package source lives in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages).
The compiler, language SDK, language tests and research live in
[PipeLang](https://github.com/Dockpipe-Industries/pipelang). Dockpipe depends on the
versioned PipeLang Go SDK for typed models and preserves its existing language CLI
and automatic materialization behavior. Do not copy compiler or package source
back into this repository or add sibling-checkout dependencies.

Run package builds, tests and publication workflows from their owning repository
using a pinned Dockpipe binary and SDK artifacts. Run core tests here. Release
composition imports immutable package archives and validates their inventory and
digests. See [ecosystem ownership](docs/packages/core-tools.md).

**Security:** do not file public issues for undisclosed vulnerabilities — see **[SECURITY.md](SECURITY.md)**.

**Issues** for bugs and ideas. **PRs** for code/docs; see [AGENTS.md](AGENTS.md). Prefer **feature branch** or **fork** → **PR → `staging`** (integration). **Do not push directly** to **`staging`** / **`master`** if those branches are protected. The maintainer ships by **PR `staging` → `master`** with updated **`release/releasenotes/<VERSION>.md`** for the selected release line (the workflow generates patch numbers) — that merge runs **Release**. **CI** (**`govulncheck`**, **`gosec`**, tests, **`make`**, **`.deb`**, **`tests/run_tests.sh`**, **`tests/integration-tests/run.sh`**) runs on PRs to **`staging`** and **`master`**; the **VERSION / release-notes gate** runs only on PRs **into `master`**. **CodeQL** runs in **`.github/workflows/codeql.yml`**. Details: [release/docs/branching.md](release/docs/branching.md). **Tests:** `bash tests/run_tests.sh` (unit); integration: `bash tests/integration-tests/run.sh`.

The [PR template](.github/pull_request_template.md) asks for the resulting behavior, verification and user impact. Remove its release section for ordinary integration PRs; release PRs must update the matching end-user notes.

**Go:** layout is `src/lib/{domain,application,infrastructure}` — see [src/lib/README.md](src/lib/README.md). Run `go test ./...` and `gofmt` before PRs. Application tests set **`DOCKPIPE_SKIP_DOCKER_PREFLIGHT=1`** so the suite does not require a running Docker daemon (use **`dockpipe doctor`** manually to verify Docker).

**Full local CI** (same as **`make ci`** / **`python3 workflows/shared/launch.py workflows/ci/ci-emulate/config.yml`**): install **`govulncheck`**, **`gosec`**, and **`jq`** (`make dev-deps` or **`python3 workflows/shared/launch.py workflows/setup/developer-dependencies/config.yml`**). Use the toolchain from **`go.mod`** with `GOTOOLCHAIN=local` and put Go’s **`bin`** on **`PATH`** (`export PATH="$(go env GOPATH)/bin:$PATH"`) so **`govulncheck`** matches the repo’s Go version. Then **`make ci`** (needs Docker). If **`govulncheck`** errors on stdlib / **`iter`**, reinstall it: **`go install golang.org/x/vuln/cmd/govulncheck@latest`**.

**Source layout:** required bundled definitions and shared runtime assets live under
`src/core/`. Generic engine code stays in `src/lib/` and `src/cmd/`.
This repository's own automation lives under `workflows/`; package automation lives
with the external package. Workflow-specific scripts belong beside the workflow's
`config.yml`; shared scripts belong at the lowest common owning boundary. Do not
create root script shadows for package workflows. Run `make check-paths` after
changing authored paths.

**Workflow YAML:** keep [the YAML reference](docs/workflows/workflow-yaml.md),
schema and the owning editor extension synchronized when changing public syntax.
Use existing resolver/runtime/strategy contracts rather than adding product names
to the engine. See [architecture](docs/concepts/architecture-model.md) and
[package authoring](docs/agents/packages/package-authoring.md).

**Releases:** merge **`staging` → `master`** (after **`VERSION`** + **`release/releasenotes/…`**) triggers **`.github/workflows/release.yml`**. See [release/docs/releasing.md](release/docs/releasing.md). Optional **dev.to**: [release/docs/devto.md](release/docs/devto.md).

---

## Platform testing (we need you)

The maintainer **cannot exercise every OS, CPU, Docker setup, and shell** before a release. CI runs **`go test ./...`** and builds artifacts on **Linux amd64**; everything else depends on **real machines**. If you use dockpipe on an under-tested combo, your reports and small PRs are valuable.

**Manual QA:** **[docs/manual-qa.md](docs/manual-qa.md)** (Linux, macOS, Windows).

**High-impact areas to try before or after a release:**

| Area | Why it matters |
|------|----------------|
| **Windows + WSL2** | [docs/manual-qa.md](docs/manual-qa.md) · [docs/install.md](docs/install.md) · [docs/runtime/wsl-windows.md](docs/runtime/wsl-windows.md) |
| **Linux arm64** | `*_arm64.deb` / `linux_arm64` tarball — [docs/manual-qa.md](docs/manual-qa.md) |
| **macOS** | [docs/manual-qa.md](docs/manual-qa.md) |
| **Fresh install** | Release **`.deb` / tarball / exe** per **[manual-qa.md](docs/manual-qa.md)** (not only `go run` / `make` from a dev tree). |

**When you open an issue**, please include:

- **Platform:** OS + version, CPU arch (`uname -m` on Unix), Docker version if relevant.
- **How you installed:** e.g. `.deb` arch, tarball, `go install`, from source.
- **Exact command** and **what you expected vs what happened** (paste stderr if small).

**When you open a PR** that fixes a platform-specific bug, add or extend a **test** if you can (Go unit tests run everywhere; shell integration tests may be gated — see `tests/`). Doc-only PRs that clarify install or edge cases for your platform are welcome too.

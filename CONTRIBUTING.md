# Contributing

The engine runs commands on the selected host or container runtime and applies
the configured follow-up actions. Keep package-specific behavior outside it.

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

**Issues** for bugs and ideas. **PRs** for code/docs; see [AGENTS.md](AGENTS.md).
Prefer **feature branch** or **fork** → **PR → `staging`** (integration). **Do not push
directly** to protected **`staging`** / **`master`** branches. The maintainer ships by
**PR `staging` → `master`**; that merge runs **Release**. `VERSION` selects the release
line and numeric floor. The workflow generates the patch number and notes header;
GitHub appends the merged-PR changelog at publication. No per-version notes file or
notes-change gate is required.

**CI** (**`govulncheck`**, **`gosec`**, tests, **`make`**, **`.deb`**,
**`tests/run_tests.sh`**, **`tests/integration-tests/run.sh`**) runs on PRs to
**`staging`** and **`master`**. **CodeQL** runs in **`.github/workflows/codeql.yml`**.
Details: [release/docs/branching.md](release/docs/branching.md).
Run checks through their owning workflows from the repository root:

```sh
python3 workflows/shared/launch.py workflows/ci/core/config.yml
python3 workflows/shared/launch.py workflows/ci/shell/config.yml
python3 workflows/shared/launch.py workflows/ci/integration/config.yml
```

The integration suite requires Docker. The workflow bootstrap uses the separately
released CLI pinned in `workflows/shared/bootstrap.lock.json`; it does not use a
newly built candidate to orchestrate its own checks.

The [PR template](.github/pull_request_template.md) asks for the resulting behavior,
verification and user impact. Keep titles and descriptions useful to readers of the
generated changelog, and link user documentation for upgrade steps and known limits.
Remove its release section for ordinary integration PRs.

**Go:** responsibilities live under `src/lib/{domain,application,infrastructure}`
with focused child packages; see [src/lib/README.md](src/lib/README.md). Run `gofmt`
on changed Go files and use the core test workflow above. It sanitizes inherited
Dockpipe workflow state and isolates package roots before running `go test ./...`.
Use `dockpipe doctor` separately when checking a live container engine.

**Full local CI** (same as **`make -f tooling/Makefile ci`** / **`python3 workflows/shared/launch.py workflows/ci/ci-emulate/config.yml`**): install **`govulncheck`**, **`gosec`**, and **`jq`** (`make -f tooling/Makefile dev-deps` or **`python3 workflows/shared/launch.py workflows/setup/developer-dependencies/config.yml`**). Use the toolchain from **`go.mod`** with `GOTOOLCHAIN=local` and put Go’s **`bin`** on **`PATH`** (`export PATH="$(go env GOPATH)/bin:$PATH"`) so **`govulncheck`** matches the repo’s Go version. Then **`make -f tooling/Makefile ci`** (needs Docker). If **`govulncheck`** errors on stdlib / **`iter`**, reinstall it: **`go install golang.org/x/vuln/cmd/govulncheck@latest`**.

**Source layout:** generic engine code and library-owned runtime resources live in
`src/lib/`; the CLI entrypoint is `src/cmd/`. The Qt GUI shell and native host SDKs
live in `src/dockpipe-app/`; see its [architecture guide](src/dockpipe-app/ARCHITECTURE.md).
This repository's own automation lives under `workflows/`; package automation lives
with the external package. Workflow-specific scripts belong beside the workflow's
`config.yml`; shared scripts belong at the lowest common owning boundary. Do not
create root script shadows for package workflows. Run `make -f tooling/Makefile check-paths` after
changing authored paths.

**Workflow YAML:** keep [the YAML reference](docs/workflows/workflow-yaml.md),
schema and the owning editor extension synchronized when changing public syntax.
Use existing resolver/runtime/strategy contracts rather than adding product names
to the engine. See [architecture](docs/concepts/architecture-model.md) and
[package authoring](docs/agents/packages/package-authoring.md).

**Releases:** merging **`staging` → `master`** triggers **`.github/workflows/release.yml`**.
Review the generated notes header and comparison link in a release dry run; the
GitHub-generated changelog is added only during publication. See
[release/docs/releasing.md](release/docs/releasing.md). Optional **dev.to** uses a
separately reviewed announcement: [release/docs/devto.md](release/docs/devto.md).

---

## Platform testing (we need you)

Automated qualification covers selected Linux, macOS and Windows runners; it does
not cover every OS version, CPU, container engine or physical desktop. Cross-builds
alone do not prove native installation or execution. Reports from real machines
remain valuable, especially for combinations outside those workflows.

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

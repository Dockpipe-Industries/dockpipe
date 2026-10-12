# Tests

- **unit-tests/** — CLI, runner, repo-root resolution, layout guard, clone-worktree include. Run: `bash tests/run_tests.sh` (from repo root).
- **Maintainer packages** — Shell and Go tests: **`packages/pipeon/tests/`**, **`packages/dorkpipe/tests/`**, **`packages/dorkpipe-mcp/tests/`**. `run_tests.sh` calls each package’s **`tests/run.sh`**.
- **integration-tests/** — Full flow with Docker: templates, actions, mounts, env, detach, agent-dev image. Run: `bash tests/integration-tests/run.sh` (from repo root). See [integration-tests/README.md](integration-tests/README.md) for details.

Smoke and **test_deb_install** (Docker + `.deb`) run from **`run_tests.sh`** when available.

For interactive remote setup, run `python3 tests/unit-tests/test_remote_interactive.py /absolute/path/to/dockpipe`
on Linux/macOS. It uses a real PTY and an isolated fake installer to check approval, password prompt
visibility, hidden input, and failure propagation. It performs no host installation or provider setup.

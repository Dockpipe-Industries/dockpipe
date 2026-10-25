# Tests

- `unit-tests/` covers the generic CLI, runner, repository paths, layout and clone/worktree behavior. Run `python3 workflows/shared/launch.py workflows/ci/shell/config.yml` from the repository root.
- `integration-tests/` covers Docker workflows, actions, mounts, environments and detached execution. See [the integration guide](integration-tests/README.md).
- Workflow-specific Python and PowerShell tests live beside their owners under `workflows/`. Run the Python contracts with `python3 workflows/shared/launch.py workflows/ci/workflow-contracts/config.yml`; native PowerShell and desktop checks have platform-specific workflow entrypoints.
- Catalog-package and VM tests belong to their external package owners in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages). Compiler/language SDK tests belong to [PipeLang](https://github.com/Dockpipe-Industries/pipelang). They consume pinned released runtime/SDK artifacts; this repository does not import their sibling source trees.

The shell sweep runs Docker smoke checks and the Debian installation check when their prerequisites are available. Build the Debian fixture through `workflows/package/linux-cli/config.yml`, supplying a numeric version and `--output release/artifacts`.

For interactive remote setup, run `python3 tests/unit-tests/test_remote_interactive.py /absolute/path/to/dockpipe` on Linux/macOS. It uses a real PTY and isolated installer fixtures to check approval, password prompt visibility, hidden input and failure propagation. It performs no host installation or provider setup.

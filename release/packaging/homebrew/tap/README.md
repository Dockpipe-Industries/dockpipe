# DockPipe Homebrew tap

Staging packages for macOS Apple Silicon and Intel. Each formula uses the tested
native CLI and complete package store from one immutable staging candidate.

Once the first **Update staging formula** workflow succeeds:

```sh
brew install Dockpipe-Industries/dockpipe/dockpipe-staging
dockpipe --version
```

To update:

```sh
brew update
brew upgrade dockpipe-staging
```

The formula version includes the staging run and attempt, so Homebrew recognizes
new candidates even when `dockpipe --version` still reports the planned core
version. The command remains `dockpipe` and conflicts with the stable formula.
Use a test machine or set `DOCKPIPE_GLOBAL_ROOT` to a separate test directory;
staging otherwise uses normal DockPipe user data. Docker is required only by
workflows that use containers. Linux users can use the staging APT repository.

The workflow checks the public staging pointer every 15 minutes (GitHub schedules
can be delayed) and can also be run manually. It verifies the source CI run has
completed successfully, checks release provenance and checksums, tests installation
on both macOS architectures, then updates the formula. Unchanged candidates do
not trigger installs or commits. The tap uses only its own `GITHUB_TOKEN`; it has
no R2 credentials or permission to publish DockPipe releases.

Maintainer source: `release/packaging/homebrew/tap/` in the
[DockPipe repository](https://github.com/Dockpipe-Industries/dockpipe).
Copy changes to this tap through review; the workflow only updates the generated
formula. Public-repository schedules may be disabled after 60 days without activity;
check the Actions page and re-enable the schedule if needed.

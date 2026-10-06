# DockPipe Homebrew tap

Staging packages for macOS Apple Silicon and Intel. Each formula uses the tested
native CLI and complete package store from one immutable staging candidate.

For CLI / Remote Worker installation:

```sh
brew install Dockpipe-Industries/dockpipe/dockpipe-staging
dockpipe --version
```

For Desktop (launcher plus CLI), after a desktop candidate and the **Update staging CLI and desktop** workflow have passed on both Mac architectures:

```sh
brew install --cask dockpipe-industries/dockpipe/dockpipe-desktop-staging
```

The cask reuses the CLI formula and installs DockPipe in Applications. Its removal retains the CLI and user data. Use either the cask or the direct DMG installer. Staging desktop builds require a separate signing/notarization qualification before normal Gatekeeper-approved distribution.

To update the CLI:

```sh
brew update
brew upgrade dockpipe-staging
```

The formula version includes the staging run and attempt, so Homebrew recognizes
new candidates even when `dockpipe --version` still reports the planned core
version. The command remains `dockpipe`. A stable formula is not published yet;
conflict declarations must be added when it becomes available.
Use a test machine or set `DOCKPIPE_GLOBAL_ROOT` to a separate test directory;
staging otherwise uses normal DockPipe user data. Docker is required only by
workflows that use containers. Linux users can use the staging APT repository.

The workflow checks the public staging pointer every 15 minutes (GitHub schedules
can be delayed) and can also be run manually. It verifies the source CI run has
completed successfully, checks release provenance and checksums, tests installation
on both macOS architectures, then updates the formula and cask. Unchanged candidates do
not trigger installs or commits. The tap uses only its own `GITHUB_TOKEN`; it has
no R2 credentials or permission to publish DockPipe releases.

Maintainer source: `release/packaging/homebrew/tap/` in the
[DockPipe repository](https://github.com/Dockpipe-Industries/dockpipe).
Copy changes to this tap through review; the workflow only updates the generated
formula and cask. Public-repository schedules may be disabled after 60 days without activity;
check the Actions page and re-enable the schedule if needed.

Desktop users update both with `brew upgrade dockpipe-staging dockpipe-desktop-staging`.

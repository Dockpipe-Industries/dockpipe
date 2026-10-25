# Manual QA (before release)

Use the exact release artifacts being qualified and verify their checksums before
installation. Automated Go, workflow and native-platform checks cover different
contracts; validate installation and interaction on real machines when those paths
change. Record the release version and platform for each result.

**Install / first run:** [installation](install.md) · [first workflow](onboarding.md)

## Common checks

- [ ] `dockpipe --version` reports the intended version; `dockpipe --help` works.
- [ ] Run the host-only `hello` workflow from [onboarding](onboarding.md) in a new
  project outside the Dockpipe source checkout. It must work without Docker.
- [ ] For container workflows, start the selected engine, run `dockpipe doctor`,
  then `dockpipe --runtime dockerimage --isolate alpine:3.22 -- echo ok`.
- [ ] For a desktop installation, open Dockpipe, select the test project, run its
  host workflow and check the output. Close and reopen the app to check saved state.
- [ ] Upgrade an older installation using the same installation method. Repeat the
  applicable checks and confirm existing user contexts and projects remain available.

The container example requires a reachable Docker-compatible engine and may pull
the image. Host commands use Bash on Linux/macOS and Windows PowerShell on Windows;
workflows using `.sh` scripts also need Bash.

## Linux

- [ ] **DEB CLI:** install the matching `dockpipe_VERSION_ARCH.deb` with
  `sudo apt install ./dockpipe_VERSION_ARCH.deb`; check that `command -v dockpipe`
  resolves to the packaged executable.
- [ ] **DEB desktop:** install the matching CLI and `dockpipe-desktop` packages
  together as shown in [installation](install.md#desktop-installation). Respect the
  separate amd64 and arm64 system baselines.
- [ ] **Portable archive:** extract the matching Linux tarball and run
  `./dockpipe --version` before adding it to `PATH`.
- [ ] **Flatpak:** follow the [bundle installation](install.md#flatpak-desktop-linux-amd64)
  steps, open the app, and run the bundled CLI with explicit project access. Check
  host-engine connectivity separately from the sandboxed host-only workflow.

## macOS

- [ ] **DMG:** install the matching Apple Silicon or Intel artifact. Open
  `DockPipe.app` from Applications and verify the installed CLI from a new terminal.
  Record Gatekeeper/signing failures; do not bypass system policy as part of QA.
- [ ] **Portable CLI:** extract the matching `darwin_arm64` or `darwin_amd64`
  tarball and run `./dockpipe --version`.
- [ ] **Container engine:** test the selected Docker Desktop or Colima profile
  separately, including project mounts. Desktop installation success does not prove
  container networking, Compose or provider integration.

## Windows

- [ ] **MSI:** test the default desktop installation and the CLI-only feature
  selection. Open a new terminal and run the common CLI checks.
- [ ] **ZIP:** extract the x64 CLI archive and run `dockpipe.exe --version` before
  adding its directory to the user `PATH`.
- [ ] **Native container run:** with Docker Desktop available, run
  `dockpipe.exe --runtime dockerimage --isolate alpine:3.22 -- echo ok`.
- [ ] **Optional WSL bridge:** after configuring Dockpipe inside WSL, set
  `$env:DOCKPIPE_USE_WSL_BRIDGE = '1'` in a temporary PowerShell session. Run the
  same container command from a Windows project and verify its working-directory
  mapping. Close that session before returning to native Windows checks.

**Details:** [WSL bridge](runtime/wsl-windows.md) · [CLI reference](cli-reference.md)

## Source-build checks

For contributor builds, use `make -f tooling/Makefile build` from the repository root,
then `./src/bin/dockpipe --version`. This validates a source build, not a packaged
installation. GUI build inputs and focused checks are documented in
[the app architecture guide](../src/dockpipe-app/ARCHITECTURE.md).

## If something fails

Record the OS version, CPU architecture, exact artifact and checksum, install method,
Dockpipe version, command and stderr. For container failures, include the engine
version and selected context. Keep local, native-platform and published-release
results distinct.

**Packaging maintainers:** [Homebrew](../workflows/publish/homebrew/README.md) ·
[Linux CLI packages](../workflows/package/linux-cli/all.yml)

**Contributors:** [platform testing](../CONTRIBUTING.md#platform-testing-we-need-you)

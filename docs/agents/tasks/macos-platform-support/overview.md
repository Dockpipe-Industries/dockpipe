# TASK-033 macOS Dockpipe Platform Support And Qualification

## Goal

Make Dockpipe work and remain qualified on selected macOS releases and Apple Silicon, with Intel
support retained only if explicitly selected and proven.

## Scope

- Select exact macOS release/architecture baselines and lifecycle policy.
- Implement and validate installation/upgrade, codesigning/quarantine expectations, shell and PATH,
  APFS permissions and durability, process trees, CLI, packages/resolvers/workflows, runtimes,
  applications, Docker/VM integrations, and diagnostics.
- Keep TASK-013 ownership of App Server storage/session evidence and TASK-023 ownership of any future
  host-sandbox decision; consume their evidence without inferring platform-wide support.
- Distinguish native arm64, Rosetta/x86_64, virtualized macOS, and cross-compiled artifacts.

## Acceptance Criteria

- Every advertised macOS release/architecture combination passes reproducible native evidence for
  all claimed surfaces, including APFS, permissions, process teardown, and application integration.
- Codesigning, notarization, quarantine, developer-tool, and container/VM prerequisites are explicit.
- Intel support is independently retained, deprecated, or excluded; TASK-025 records the decision.

## First Bounded Slice

TASK-036 adds Darwin-compilable remote-node code and launchd user-service manifests. Linux loopback
proof and cross-builds do not qualify Mac execution; native launchd, sleep/wake, and teardown evidence
remain required. See [remote onboarding](../remote-node-onboarding/overview.md).

Inventory current Darwin builds, installation paths, Apple Silicon/Intel assumptions, APFS and
process gaps, available CI/native hardware, and TASK-013 evidence. Do not run paid macOS CI or VMs,
install software, modify source, publish/notarize artifacts, or claim support.

## Release implementation progress

The user subsequently authorized the 0.6 cross-platform release implementation. The release workflow
now has native macOS Intel and Apple Silicon jobs, native helper stores, and a host-workflow smoke
check. The direct installer uses the macOS data directory. The
[desktop release dry run](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37409460637)
passed both native Mac jobs at `962c1390`, including DMG installation of the CLI and launcher.
See the [qualification receipt](../recurring-codebase-hygiene/staging-release.md#native-desktop-qualification-passed).
Launchd, sleep/wake, notarization/quarantine, M6 hardware, and full application/runtime acceptance
remain open; installer qualification does not prove Colima integration.

## Workflow portability audit (2026-10-13)

The user authorized enabling macOS for workflows whose implementation can support it after a
remote `test` job reached the Mac but failed its host-platform allowlist.

- Enabled 31 workflow definitions across core starters, container examples, agent resolver
  workflows, editors, secrets, Terraform/publication, and bounded DorkPipe helpers. Updated 13
  matching workflow/package manifests and macOS dependency installation hints.
- Changed `test` to bridge networking so Docker Desktop does not require optional host networking.
  Linux operators can still explicitly select host networking where needed.
- Documented Homebrew Bash 4.2+ and service PATH requirements for SDK-based host scripts.
- Retained VM/CI-emulation, GNU-dependent doctor/Pipeon, and complex orchestration/self-analysis
  restrictions pending their own portability work. Package availability does not override those
  workflow restrictions.
- All 31 workflow definitions validate. An isolated package compile completed 44 workflow builds
  from the affected roots, including unchanged siblings; Dev Container lifecycle fixtures pass.
  Tarball inspection confirms macOS declarations, bridge networking, and deferred restrictions.
  Separately compiled and newly scaffolded starters also preserve macOS support.
  Compilation hooks produced local helper artifacts under the existing ignored build tree, with
  the validation store and private test state under `/tmp`.
- Validation exposed an existing compiler cleanup collision: compiling `secretstore` with force
  removes previously emitted `secretstore-*` tarballs through the name-prefix glob in
  `src/lib/application/internal/packagecompile/workflow.go`. The affected secrets workflows were
  compiled and inspected in independent temporary stores. Fix that cleanup boundary before
  claiming a complete combined package store; it was not changed in this YAML portability slice.
- This is source/local validation, not native macOS execution, provider acceptance, or publication.
  Existing compiled packages and queued job snapshots still need updated workflow content.

Canonical prerequisites and exceptions: [Workflows on macOS](../../../workflows/macos.md).

## Colima container workflows backlog

Requested and authorized for implementation 2026-10-06. Enable Dockpipe CLI and launcher container
workflows on macOS using Colima's Docker runtime as an alternative to Docker Desktop. The first
compatibility fix is implemented locally; native Colima qualification remains open.
[Upstream Colima](https://github.com/abiosoft/colima#docker) documents using the Docker client with
its Docker runtime; containerd, Kubernetes, and other backends are outside this first slice.

- Document optional Colima, Docker CLI, and required Compose/build tooling prerequisites for both
  Homebrew and DMG users. Host-only Dockpipe workflows must remain usable without a container VM.
- Audit Docker endpoint resolution across CLI, launcher, and package workflows. Honor explicit
  context/host settings, support named Colima profiles, and preserve the user's selected context.
- Diagnose missing tooling, stopped profiles, and unreachable daemons with actionable setup/start
  guidance. Installation and VM lifecycle actions must follow user intent; merely launching
  Dockpipe must not install or start Colima.
- Verify image build/run, Compose, bind mounts and permissions, port access, cancellation, and
  cleanup on Apple Silicon and Intel hosts. Include coexistence with Docker Desktop and regression
  checks for the existing Docker path.
- Keep Colima-specific setup and lifecycle in package/resolver assets; change the generic engine
  only if endpoint compatibility needs a general primitive. Coordinate Dev Container acceptance
  with [TASK-014](../native-devcontainer-support/overview.md).

Acceptance: reproducible native evidence with pinned host/tool versions, working CLI and launcher
flows, clear setup/repair documentation, and no implicit changes to existing VM profiles or contexts.

### First compatibility fix

- The audit confirmed Docker commands inherit the client's context/host/config settings. No
  Colima runtime type or engine-specific provider branch is needed.
- Finder startup now appends standard Homebrew executable directories after inherited PATH entries.
  Both direct Docker observations and CLI/workflow subprocesses inherit the resulting environment.
- Docker preflight diagnostics now refer to the selected daemon/VM provider and context overrides.
  [Installation guidance](../../../install.md#containers-with-colima) covers Colima, Buildx/Compose
  prerequisites, named contexts, and the difference between Finder and shell environments.
- Local Linux validation passed the Qt build, CTest child-environment regression, focused Go Docker
  tests, installer ownership/wrapper tests, a staged CLI/launcher startup smoke, and shellcheck.
  The initial smoke used a build directory without its sibling CLI and failed the expected-path
  assertion; staging the intended installed layout passed. The Unix release build now runs CTest
  on both Mac architectures as well as Linux.

Remaining: native Colima build/run, mount/port, Compose, cancellation/cleanup, named-profile, and
Docker Desktop coexistence checks on macOS. These local tests and the earlier installer pipeline
do not prove those VM behaviors. No Colima software or VM was installed/started by this change.

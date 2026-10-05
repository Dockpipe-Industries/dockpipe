# TASK-036 Remote Node Onboarding

## Objective and authority

Connect Cloudflare through browser login, pair a Mac without SSH, execute a locally approved native
benchmark workflow, and return results. Preserve generic core execution and resolver-owned edges.
The user authorized implementation in this checkout on 2026-10-03. Source/local verification does
not authorize live account mutation, service installation on this machine, or publication.

## Current state

- Added single-target delivery, private pairing/state, session-bound jobs, durable worker journals,
  cancellation, local workflow profiles, results, and bounded artifact collection.
- Added remote CLI commands and Linux/macOS user-service manifests.
- Added resolver-owned Cloudflare browser login, tunnel/DNS setup, credential separation, and
  preserved evidence for ambiguous provider mutations.
- Core has no provider names/imports. Existing workflow YAML semantics and TASK-015 fixtures stay
  unchanged. DorkPipe retains orchestration and placement ownership.
- Real Linux CLI loopback smoke passed: pairing, native workflow, artifact download, and submission
  idempotence. Race tests cover foreign worker sessions and saved-result recovery without execution.
- Existing `TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap` fails with
  `resolve project checkout: lstat /path`; reproduced with pre-change `run.go` via a Go overlay.

## Local verification

- Core domain/infrastructure/application/CLI regressions pass with the one independently reproduced
  pre-existing checkout-path test excluded. The unfiltered run remains a preserved failure.
- Focused race tests and `go vet` pass; package tests pass through the standard package test hook.
- The package source hook builds and compiles the resolver. Tarball readback confirms its helper
  has executable permissions and runs after extraction.
- Linux CLI/helper builds and Darwin arm64 CLI/helper cross-builds pass. No native Mac claim.
- Shell syntax and diff whitespace checks pass. The Qt launcher `make build` was not run; this
  change builds the CLI directly and does not modify the launcher.
- Generated outputs: remote helper, resolver staging and tarball under existing `bin/.dockpipe`
  runtime/store helpers; isolated test state and cross-builds under `/tmp`. No generated output
  belongs in the source commit. No account, installed service, commit, or push was performed.

## Remaining qualification

- Real Cloudflare browser/account/DNS/readiness with authorization.
- Native Apple Silicon execution, launchd, sleep/wake, and process teardown.
- The actual Nucleon benchmark workflow and source/compiler/measurement evidence on the Mac.

## Boundaries

Initial scope is a single-operator CLI. GUI onboarding, additional Linux dependency installers, Windows
workers, retention cleanup, corpus/source synchronization, multi-tenant hosting, and DorkPipe graph
integration remain follow-ups. TASK-015 stays closed on fixture proof. TASK-033 owns wider Mac
qualification. Canonical behavior: [Remote nodes](../../../runtime/remote-nodes.md).

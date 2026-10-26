# TASK-036 Remote Node Onboarding

## Completed-job capacity and replay safety (2026-10-24)

- A consenting native validation worker reached the documented lifetime limit of 128
  retained jobs; all jobs were terminal, yet new delivery refused.
- The broker now keeps at most 128 recent jobs and archives oldest terminal metadata
  before admitting new work. Archived IDs preserve exact retry, result, cancellation,
  and worker-result identity; unresolved jobs never retire. Payload storage is unchanged.
- Private bounded immutable records, hot/archive overlap checks, and a persisted
  downgrade guard preserve fail-closed recovery. Backups include terminal archives;
  restoring stale broker metadata is not a supported rollback.
- Focused offline remote/domain tests pass, including restart, conflicting replay,
  incomplete state publication, malformed records, private path bounds, and canceled
  or unknown work. Native APFS and NTFS checks pass. The supported live broker
  service update preserved the original jobs, paired nodes and configuration;
  subsequent Mac package and visible UI validation resumed successfully.

## User-service upgrade recovery (2026-10-13)

- A live pairing failure was traced to a user service still pinned to an older test CLI,
  despite newer desktop installations. Application/package updates did not migrate that service.
- Remote info now reports registration drift, including executable-content changes at the same
  path. Machines offers explicit service replacement/restart with an interruption/custom-settings
  confirmation; hosted brokers have no local service action.
- `remote service --role broker|worker --replace` preserves remote identity/provider state,
  publishes the service definition atomically, and restarts through systemd or launchd. Failed or
  interrupted activation remains update-required until an explicit retry completes.
- Focused remote-command/broker race tests, service failure/retry/cancellation fixtures, launcher
  confirmation/refusal/retry checks, native Linux builds, Go vet, and Darwin arm64 test cross-build
  passed. Rendered UI was inspected; the repaired CLI detects the stale local registration.
- After separate user approval, the affected live Linux broker service was repointed from the old
  test binary to the installed 0.6.13 CLI and restarted. Its prior definition was backed up; no jobs
  were running. The process executable, active service, local broker response, and public tunnel
  response were verified. Revoked enrollments remained revoked until the user's subsequent pairing.
- The Mac subsequently paired and received a newly submitted `test` workflow. Its returned result
  failed at the workflow platform allowlist (`macos` excluded), establishing delivery and worker
  pickup but not successful workflow execution. Native service lifecycle qualification remains
  open; the new upgrade UI/CLI source is not published.
- The authorized portability follow-up enables 31 workflow definitions and matching manifests for
  macOS, including `test`, whose default Docker network is now bridge. See
  [TASK-033](../macos-platform-support/overview.md) and the
  [workflow support guide](../../../workflows/macos.md) for prerequisites and deferred paths.

## Worker setup completion and Activity roles (2026-10-13)

- The user confirmed that the Mac is successfully paired to receive workflows.
  The reported `remote.jobs` missing-operator-file error is an operator-only
  Activity request, not evidence of a submitted workflow failing.
- The launcher now starts the worker user service after successful pairing,
  discloses that behavior before the request, and reports startup failure while
  preserving pairing. The existing service button retries without re-pairing.
- Activity reads the public remote-info projection first. Receive-only workers
  get role guidance, no broker-wide jobs request, and disabled job-management
  actions. CLI operator commands give actionable setup guidance when credentials
  are missing; worker credentials never become operator credentials.
- Focused Go remote tests and the Qt launcher fixture pass. The UI fixture covers
  cancelled pairing, automatic service startup, startup failure and service-only
  retry, plus operator-to-worker Activity transitions. Linux builds pass.
- The broader `go test ./src/lib/... ./src/cmd` sweep was stopped after
  application-IR tests refused to compile generated Go outside the required
  containment runner. This is not full-suite proof; the guard was not bypassed.
- Test service commands are fixtures; no live service was installed or started
  during local verification. Staging promotion is tracked separately. Native Mac
  service readiness and an actual delivered workflow still need verification.

## Objective and authority

Connect Cloudflare through browser login, pair a Mac without SSH, deliver selected workflows with their assets and package dependencies, execute them on an
explicitly consenting worker, and return results. Preserve generic core execution and resolver-owned edges.
The user authorized initial implementation on 2026-10-03 and actual source delivery plus simpler
onboarding on 2026-10-05. This is objective `remote-workflow-delivery-onboarding`. Source/local verification does
not authorize live account mutation, service installation on the current host, or publication.

## File-free pairing and launcher workspace (2026-10-09)

- User authorized secure request/approve pairing and the workflow-focused launcher
  refresh after rejecting invitation-file transfer as onboarding UX.
- Generic remote endpoints now support operator-opened pairing windows, worker
  verification codes, explicit approval/denial, bounded anonymous requests and
  persisted credential identity. Existing file invitations remain supported.
- The launcher exposes Apps, Workflows, Machines and Activity. Machines owns the
  setup/pairing/service controls; workflow delivery previews sources and binds
  submission to their digest. CLI and private state remain the authority.
- Live broker DNS was isolated to the router relay: direct Pi-hole A/AAAA answers
  were valid while router responses were intermittently malformed. After disabling
  relay and renewing the Mac lease, the user confirmed public endpoint HTTP 401.
  This proves reachability, not completed Mac pairing or workflow execution.
- Local validation passes: application/internal command suites, remote domain and
  infrastructure tests, CLI tests, remote race-detector tests, and all seven Qt
  CTest checks. The Qt test exercises machine discovery, a displayed verification
  code, real process cancellation, and preview invalidation/digest submission with
  isolated fixtures. Screenshots were visually inspected.
- Linux CLI and launcher builds and a Darwin arm64 CLI cross-build succeeded.
  Native macOS/Flatpak UI and the live Mac workflow remain untested. The complete
  repository/compiler/release matrix was not run for this local UX iteration.
- Generated test builds: `/tmp/dockpipe-pairing-build/dockpipe`,
  `/tmp/dockpipe-pairing-build/dockpipe-darwin-arm64`, and
  `/tmp/dockpipe-launcher-remote-build/dockpipe-launcher`. Fixture screenshots:
  `/tmp/dockpipe-remote-ui-{machines,pairing,workflows}.png`.
- Existing live services and installed apps are not replaced, and this continuation
  has not been committed or published. Both broker and worker need the updated CLI
  before using online pairing; an older running broker cannot serve the new routes.

## Provider-aware remote setup (2026-10-09)

- User clarified that future Dockpipe-managed access means hosted service with Dockpipe
  Cloud account sign-in, rather than another locally hosted tunnel.
- The public catalog exposes resolved provider metadata. Machines shows the configured
  provider, version, hosting mode and endpoint, with explicit unknown metadata for legacy
  setups. Setup selects installed providers instead of free-text names.
- Generic hosted setup supports a package-owned authentication hook and validated private
  result, authenticated readiness before adoption, and hosted operator routing. It does
  not start a local broker. Existing local administration remains loopback-only.
- Provider changes preserve existing state and require separate state directories. No
  actual Dockpipe Cloud provider, account service, multi-tenant backend or automatic token
  renewal is implemented; these remain separate work.
- Verification: focused catalog/resolver tests, remote command and broker race suites,
  remote-command vet, Linux CLI/Qt builds and all seven Qt CTests passed. Dark/light
  fixture captures were inspected. No live hosted authentication or native Mac/Flatpak
  qualification was performed. Build/capture outputs remain under `/tmp`.
- Source changes and isolated validation only; no installed binaries, active broker,
  accounts or provider resources changed.

## Cloudflare dependency setup (2026-10-09)

- Live user testing reached successful browser authorization and tunnel credential creation, then
  exposed a package validator that incorrectly required `0600` for Cloudflare's `0400` tunnel file.
  The provider adapter now accepts either owner-readable mode without chmod, retains owner/link
  checks, and leaves generic mutable Dockpipe-state permissions unchanged. Package tests cover
  both accepted modes, rejected shared/unreadable/executable modes and links, and recovery without
  repeated tunnel creation. The local test package helper was rebuilt; the user retry passed credential validation and started the broker.

- Follow-up interactive UX repair: remote setup no longer runs an enclosing spinner, and dependency
  installation emits start/final results without timed heartbeat output while a child owns the
  terminal. A handoff message explains installer prompts and hidden password input. This is generic
  CLI behavior, with provider-specific installation still owned by the package.
- The real-PTY regression passes with the local CLI and reproduces prompt interference with the
  installed CLI. It holds a fake password prompt open beyond the old heartbeat interval, checks
  hidden input, and confirms installer failures propagate. No real sudo or Cloudflare calls occur.
- Local retry CLI: `bin/dockpipe-interactive-test`; package override remains
  `/tmp/dockpipe-remote-local-rfgbyfd5/packages`. The running installed CLI is not hot-patched.

- Fixed the native Debian/Ubuntu/Pop!_OS package dependency declaration after installed setup
  reported `installer_present=false`. Adapter and parent package versions advance to 0.3.1.
- The existing approval flow offers Cloudflare's signed APT repository installer, with architecture
  scoping, isolated repository refresh, download cleanup, and failure propagation. Provider logic
  remains package-owned; no engine or workflow-schema changes were required.
- Package tests execute the declared installer with isolated command fixtures, covering success,
  download failure, signing-key installation failure, and repository-refresh failure. The package
  test hook and focused core approval/preflight/bundled-tool regressions pass.
- Flatpak explicitly requires bundled `cloudflared`. Current package metadata, a fresh provider
  helper, and the lock-verified client passed preflight and client execution in the existing Flatpak
  Platform with networking and host integration disabled. This is dependency proof, not live remote
  setup or a full new release build. Native APT mutations were mocked, not applied to the host.
- Generated qualification files remain under `/tmp/dockpipe-cloudflare-dependency-nzpyzrmr`.
  No installed package, Cloudflare account, service, or release was changed by this repair.

## Delivery continuation (2026-10-05)

- State: completed for implementation and local verification. No worktree, commit, push,
  publication, live tunnel/DNS change, credential action, or persistent service installation.
- Implemented explicit worker delivery permission, bounded content-addressed source snapshots,
  safe private staging, package closure and workflow validation, and existing journal/result flow.
- Added CLI preview, source/extra input/package selection, profile-free opt-in pairing, next-step
  guidance, and resolver-owned desktop-session propagation with login progress.
- Real Linux delivery smoke passes with the original sender paths removed before worker start:
  new workflow, assets, extra input, package child workflow, logs, results and identity conflicts.
- Terminal proof passed: focused remote domain/infrastructure/CLI/resolver race suite; application,
  domain, infrastructure, model and CLI regressions; `go vet`; the standard remote package test hook;
  real CLI profile and delivery smokes; native Linux CLI build and Darwin arm64 CLI/helper builds.
  Final offline snapshot tests also cover oversized dependency manifests before YAML parsing.
- The full `go test ./src/lib/... ./src/cmd ./packages/remote/tools/...` attempt is not a pass:
  applicationir generated-Go checks refused the missing 1 GiB cgroup guard. The unrelated remaining
  pipelang compiler test was explicitly stopped after that failure. Its contained full campaign and
  Qt `make build` were not run for this remote-only change. Current application regressions pass,
  including the test recorded as failing in the earlier baseline below.
- Canonical docs and CLI help are updated; remote resolver/package versions are 0.2.0. Core remains
  provider-neutral, provider/browser behavior remains package-owned, and workflow YAML is unchanged.
- Generated binaries, cross-builds and test logs are under `/tmp`; smokes clean their temporary
  broker/worker state. No generated deliverables were added to source. HEAD, stashes and `.vscode`
  settings match the admitted handoff anchors. No additional task or subagent was created.
- Native Mac and live Cloudflare remain separate qualification; the new code is not published.

## Earlier completed state

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

## Earlier local verification

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

### Launcher provider setup (2026-10-12)

Source changes add a capability-filtered provider Marketplace action directly to
remote setup, return to setup with the installed provider selected, and retain an
existing connection's address while offering provider review from pairing. The
launcher delegates installation to the pinned package CLI and setup to the existing
provider contract; Cloudflare behavior stays package-owned. An explicit connection
refresh follows interactive setup. Local Qt fixtures cover missing providers,
legacy connection display, hosted/local selection and checksum-pinned installation.
Cloudflare fake-provider checks retain browser login, saved-state reuse and the
unknown-create refusal. Live account changes, native Mac UI acceptance and release
publication have not been performed for this change.

- Real Cloudflare browser/account/DNS/readiness with authorization.
- Native Apple Silicon execution, launchd, sleep/wake, and process teardown.
- The actual Nucleon benchmark workflow and source/compiler/measurement evidence on the Mac.

## Boundaries

Initial scope is a single-operator CLI. GUI onboarding, non-Debian native Linux dependency installers, Windows
workers, retention cleanup, large-corpus/whole-checkout synchronization, multi-tenant hosting, and DorkPipe graph
integration remain follow-ups. TASK-015 stays closed on fixture proof. TASK-033 owns wider Mac
qualification. Canonical behavior: [Remote nodes](../../../runtime/remote-nodes.md).

## Staging promotion qualification (2026-10-09)

User approved checkpoint, publication and promotion through js/dev and dev to staging.
The final pass refreshed the exact authored embed manifest for four new Cloudflare helper
files and repaired fresh pairing after access removal: explicit pairing rotates a revoked
credential and still requires a new operator-approved verification code. Tests retain the
old-credential rejection check. Local development CLI/launcher identities now include the
build commit and dirty state; release version injection is preserved.

## P1 authentication and lifecycle hardening refinement (2026-10-26)

Part of the [security and reliability initiative](../dependency-lifecycle/overview.md),
with TASK-039 owning shared authorization contracts, TASK-042 workload identities,
TASK-041 optional approval delivery and TASK-046 enrollment/passkey assessment.
This refines existing pairing ownership; no second pairing or identity task is created.

Inspection of `src/lib/infrastructure/remote/{broker,pairing}.go` found hashed
administrator/worker bearer credentials, operator-opened pairing windows, explicit
code approval and worker revocation. Invitation expiry is not continuing worker
session expiry; the inspected authenticated dispatch does not itself establish
hardware-backed approval or operation-scoped elevated authority. These are design
gaps to address, not evidence of exploitation or implemented enhancements.

Required P1 scope:

- Define mandatory hardware-backed FIDO2/WebAuthn security-key authentication for
  remote pairing authorization and privileged lifecycle/restart approvals. Reuse
  established verification libraries and shared TASK-039 identity/grant contracts.
  Specify supported authenticators, RP/origin binding, user presence/verification,
  hardware provenance/attestation policy and enrollment/revocation/recovery.
  A generic or synced passkey must not automatically satisfy a hardware-key policy.
- Define bounded worker/admin session expiry, rotation and revocation, plus scoped
  authorization separate from pairing. Fail closed for missing/expired hardware
  assertions where required; recovery cannot silently downgrade the mandatory gate.
- Pairing only enrolls the machine. Installing/updating software, elevation and
  critical service restarts require exact target/action/plan authority checked by
  the executor at use. Unattended work requires a separately provisioned bounded
  policy; it cannot synthesize a hardware assertion or renew a human grant.
- Coordinate TASK-049/050 existing broker jobs and target recovery. Token renewal,
  reconnect and result retries cannot renew approvals or replay updates.

Acceptance/tests: reject software-only credentials under hardware-required policy,
wrong-origin/RP assertions, replay, expired sessions, revoked keys/workers and
merely paired privileged requests. Verify authorized local/headless human approval,
scoped unattended policy, loss/replacement of a hardware key, audit attribution,
and tunnel heartbeat-loss recovery with TASK-052. Preserve existing pairing
rate limits, private credentials and anonymous-approval rejection tests.

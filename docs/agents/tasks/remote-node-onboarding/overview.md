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
  remote pairing authorization. Remote feature unlock uses OS authentication;
  per-operation hardware prompts are deferred configurable policy. Reuse
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

## Remote unlock and global auto-lock policy (2026-10-26 refinement)

User refinement: mandatory physical hardware-key verification applies to approval
of a new pairing. Routine remote operations use an OS-authenticated remote unlock
session; a hardware-key touch for every operation is not the initial requirement.
Per-operation approval rules and remotely delivered approvals remain future work.
This narrows the earlier hardware-approval wording; it does not relax broker-side
scope checks or separate maintenance/restart authority.

Provide one global, machine-level Dockpipe **Security > Remote access** policy,
consumed by the desktop, CLI and shared authorization service. This is proposed
behavior, not an implemented settings page or supported config schema.

- **Auto-lock after inactivity:** configurable duration; proposed default 5 minutes,
  with presets such as 1, 5, 15, 30 and 60 minutes plus a validated custom duration.
  Define supported bounds with the session contract. Changing the timeout must not
  silently unlock access. Do not add an unlimited session default.
- **Lock when the computer locks** and **lock on sleep:** separately configurable,
  enabled by default. Start locked after application/service restart; wake does
  not restore the old unlock session. Expose an immediate Lock remote access action.
- Measure inactivity from trusted local human interaction, never broker polling,
  telemetry, background jobs, API calls or automation. Document OS idle detection
  and supported fallback; unavailable lock/sleep integration must be visible.
- Protect the signing credential using the platform authentication/key-storage
  integration. OS reauthentication unlocks proof for a broker-verified scoped
  session; a UI flag or static bearer token alone cannot authorize remote actions.
  Mandatory hardware pairing cannot fall back to an ordinary OS password.
- Local auto-lock stops signing/issuing requests and invalidates local unlock
  state. Broker-side expiry and revocation remain authoritative even if the app
  is bypassed or disconnected. Negotiate a bounded remote session lifetime and
  refresh rules; background activity cannot extend it. A client preference cannot
  exceed a stricter broker policy or renew a separate operation approval.
- Locking prevents new remote actions; already admitted jobs can continue under
  their existing authority. It does not unpair machines, cancel work implicitly
  or change the lifetime of worker machine credentials.
- Keep policy out of project/workflow/package configuration. A repository, resolver,
  MCP client or background workflow must not weaken the user's global lock settings.
  Settings changes that weaken security require an authenticated operator context;
  tightening a policy takes effect immediately. Use one settings owner, not separate
  CLI/UI/MCP policy stores.

Acceptance/tests: cover inactivity boundaries, trusted versus background activity,
OS lock/sleep/wake, process restart, immediate lock, timeout changes while unlocked,
multiple clients, broker expiry/revocation, missing OS integration and a disconnected
client. Prove a direct API call cannot bypass locked/expired authority, mandatory
hardware pairing stays enforced, and existing admitted jobs continue correctly.
Coordinate shared contracts with TASK-039 and qualification with TASK-052.

## Mandatory hardware-key pairing implementation (2026-10-26)

Objective: implement local-owner registration and broker-enforced hardware-key
pairing for review, including persistent identities on both sides. Per-action
prompts, OS-authenticated remote unlock/auto-lock, remotely delivered approvals,
publication and live machine migration remain excluded from this implementation.
The later global lock policy is machine-level; unlocking authorizes the current
OS user, not every user on that machine.

Source now adds a pinned Go-compatible WebAuthn verifier, manufacturer CA
attestation admission, a private localhost ceremony, and broker-side one-use
approval bound to the pending request. Local registration requires the broker
stopped; adding a spare requires an existing key and cannot replace trust roots.
Legacy new/unused file invitations cannot bypass the gate. Existing paired
workers retain their current access pending deliberate revocation/re-pairing.

New workers persist Ed25519 installation identities and pin the broker identity;
requests and responses use one-use signed nonce exchanges. These identities belong
to each private broker/worker state directory, not hostnames or login sessions.
They are installation identities, not hardware-attested machine identities;
OS-keystore/system-service storage is still future work. [Remote nodes](../../../runtime/remote-nodes.md#register-pairing-hardware-keys)
is canonical for commands, trust roots, compatibility and limitations.

Local validation completed:

- Repository Core workflow, `./... -count=1`: passed.
- Focused race tests for remote, remotecmd, hardwarekey and remoteidentity: passed.
- Hardware attestation tests reject missing/self/untrusted attestation, backup
  eligibility, missing presence/verification, wrong origin/RP/challenge and replay.
- Pairing/machine tests cover both identities, stale status, nonce replay/expiry,
  scope changes, denial/close, uncertain persistence and per-node capacity.
- Native CLI build: passed; disposable binary/evidence under a task-owned temporary
  directory. macOS arm64 and Windows amd64 compile-only checks passed for hardwarekey,
  remoteidentity and remote packages. These are not native execution qualification.
- Desktop source-layout checks, local doc links and diff whitespace checks passed.
  Desktop button text and ceremony timeout were updated; no current native Qt build
  was available, so its native interaction test remains pending.
- Independent bounded security review found three issues (historical certificate
  expiry, global challenge starvation and stale pairing status); all were repaired
  and covered by regression tests, with no remaining blocker in that review.

Physical-key/browser interaction and M6 native qualification remain unproven.
The user authorized a local implementation commit after review; no push or CI
pipeline is authorized or started. Key removal/trust rotation, machine-wide OS-keystore storage, remote
unlock policy and per-operation/Cloud approvals remain separate work.
No running broker, physical key, Cloudflare tunnel or remote machine was changed.

## Pre-commit security review (2026-10-26)

A further local review reproduced an active-intermediary replay: a previously
signed machine challenge plus response could authenticate a later exchange. The
broker now signs the fresh client nonce into challenge issuance, and the client
checks it before signing or accepting the response. The regression failed before
the fix and passes after it.

The same review found that the browser launcher exposed its short-lived capability
in process arguments on multi-user systems. Browser launch now uses an owner-only
temporary redirect file; arguments reveal only the file URI, and lifecycle cleanup
removes the file on completion, failure or cancellation.

Pairing consent metadata/options were also unsigned on the operator channel.
Consent is now broker-signed and bound to the requested code and a fresh operator
nonce. The native client verifies it against protected local broker state before
showing it in the browser. Hosted approvals are explicitly refused in this slice;
remote approval remains deferred. Small challenge/consent routes have request
body bounds matched to their wire contracts.

Tests cover transcript replay, substituted consent/keys/options/code, stale
operator nonces, hosted-approval refusal, private launcher permissions and cleanup,
and absence of capabilities from opener arguments. Physical-key/browser and M6
qualification remain pending. The user subsequently authorized a local commit
and physical-key testing. Push, remote CI, service restart and live-state migration
remain outside that authorization; physical-key testing begins with isolated state.


## Native hardware-key interaction (2026-10-26 refinement)

The user requested a normal Dockpipe dialog and terminal CLI interaction instead
of either an embedded or external browser. This supersedes the browser ceremony
presentation described in the initial implementation above; the broker hardware
attestation, signed consent and persistent machine identities are retained.

- Core owns one native FIDO2 adapter over the existing system libfido2 library.
  It leaves CTAP/PIN cryptography to libfido2 and produces responses checked by the
  existing verifier. Native library loading uses supported absolute system paths;
  missing APIs/platform support fail closed. No dependency is installed implicitly.
- The GUI owns a normal key-selection/masked-PIN dialog. CLI uses terminal prompts
  with echo disabled and works without the GUI running. A bounded private child
  process protocol binds each GUI response to one fresh prompt and retained device.
  PINs stay out of command arguments, settings and logs. Closing the GUI cancels;
  terminal cancellation restores input state. Native calls have bounded timeouts.
- No automatic PIN retry, PIN creation/change, key reset or browser fallback.
  First/spare registration actions retain explicit manufacturer trust and the
  stopped-broker requirement. OS-authenticated remote unlock remains separate.
- The first physical-key attempt reached native signing but failed manufacturer
  chain validation. Supplemental published intermediates can now complete a path
  to the explicit self-signed roots; they cannot become independent trust anchors.
  Regression tests cover missing intermediates, complete chains, wrong roots and
  intermediate-only rejection. PIN wording explicitly identifies the existing key
  PIN; it never configures or changes the key.
- Linux/macOS amd64/arm64 native adapters are implemented. Windows native key
  interaction currently fails closed; its platform adapter and native qualification
  remain open under TASK-052. Installation packaging for libfido2 is also pending.
- Full local Go suite, focused security/race suites, native Qt transport/dialog
  checks and CLI build pass. Native adapter macOS arm64 and Windows amd64 compile
  checks pass; the Windows runtime adapter still fails closed as described above.
- The user's Yubico Security Key registered through the production native dialog
  and libfido2 adapter in disposable Linux broker state. A second physical-key
  assertion approved a dummy loopback worker; its signed approval and both saved
  installation identities matched. An administrator token plus code alone was
  rejected before that approval. Test broker/worker processes stopped afterward;
  no services or jobs were installed or run. The PIN was not changed or reset.
- GUI native-key commands now select an operator-controlled CLI rather than
  automatically discovering a project executable, preventing a repository from
  supplying the native PIN endpoint. Explicit absolute host overrides remain
  supported; missing/invalid choices fail clearly. Regression coverage includes
  project/CWD/relative-PATH executable refusal.
- Physical CLI terminal interaction, macOS native key interaction,
  Windows adapter implementation and packaged-install qualification remain open.
  A generated response fixture or successful build is not that acceptance.

The user authorized a local commit of this validated native refinement. The earlier
pairing implementation was committed locally as `794104e9f`. No push, pipeline, production
service change or live machine migration is authorized by this refinement.

### Local macOS application identity refinement

Bundled remote services now associate with their enclosing app's actual bundle
identifier, read using the system plist tool with bounded output and cancellation.
Standalone CLIs do not impersonate an installed application. Packaging gives the
CLI and workspace reader explicit signing identifiers, preserves nested identifiers
when signing the app, and includes a local-network permission purpose string.
Existing service registrations are not rewritten or restarted automatically.

Focused native macOS tests passed for XML/binary plist handling, malformed metadata,
cancellation, output bounds, enclosing-app association and service update behavior.
A registered, explicitly ad-hoc-signed temporary app reached a disposable broker
over verified TLS and submitted signed machine pairing requests. This proves only
that exact development build's connectivity. Consistent Apple-issued signing and
installed-app update/privacy continuity still require release qualification;
ad-hoc identifiers alone do not provide that guarantee. A subsequent fresh
cross-machine test completed with a physical Yubico assertion in the Linux native
dialog. The macOS worker received signed approval, both saved
public machine identities matched the broker state, and authenticated nonce/idle
polls succeeded. The temporary agents were removed, the disposable enrollment was
revoked, and the test broker/listener stopped. Installed services and existing
machine pairings were preserved. This does not qualify a packaged app upgrade or
macOS-hosted physical-key interaction.

A repeated fresh pairing was personally confirmed by the operator after completing
the native PIN/touch interaction. The macOS worker again received approval and
completed authenticated polls, both public machine identities matched, and the
temporary agents, broker and listener were stopped with the test enrollment
revoked. This repeat closes the user-observed cross-machine pairing check; it
does not expand the platform or packaged-upgrade qualification above.

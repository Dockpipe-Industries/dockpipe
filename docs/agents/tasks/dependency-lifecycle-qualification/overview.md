# TASK-052 Dependency upgrade recovery and multi-platform qualification

## Status, priority and dependencies

**P1; proposed, decision ready, unclaimed.** Added 2026-10-26 under
[TASK-049's security and reliability initiative](../dependency-lifecycle/overview.md).
Tests may be designed alongside TASK-049; acceptance depends on TASK-049/050/051
implementation and TASK-036/039 authorization contracts. This qualification item
blocks unattended enablement, not immediate operator-controlled P0 remediation.
Reuse TASK-025 platform qualification and TASK-032/033 Windows/macOS evidence;
do not create a second platform certification program.

[TASK-054](../dockpipe-self-update/index.yaml) adds Dockpipe self-update qualification
to this matrix: native Windows MSI/portable behavior, coherent CLI/GUI/broker and
helper versions, stale running processes, file locks/UAC/reboot, active-work drain,
state migration and compatible component-set recovery. Installation exit zero
cannot substitute for proving the actual running components and authenticated
post-update broker behavior. Unimplemented Windows broker/OS-auth rows remain
explicit prerequisites rather than inferred platform support.

## Current architecture findings and problem statement

Core has preflight/installer tests in `src/lib/application/internal/hostdependencies/`,
package integrity/dependency tests in `src/lib/infrastructure/packagecatalog/`,
operationrecord tests and broker/worker/bundle/retry tests under
`src/lib/infrastructure/remote/`. The worker journals results and marks uncertain
interrupted execution unknown; heartbeat loss cancels execution. Those contracts
make a tunnel restart over the same management connection a critical failure case.

External `packages/remote/tools/cloudflare/dependency_test.go` mocks signed APT
installation failures; setup/credential tests cover resumable creation and private
files. These do not prove safe updates of active services across platforms, a
working rollback, or a surviving broker connection. Qualification must establish
the operational outcome, not just installer exit zero or a running process.

## Proposed design and components

Use Core fixture providers and existing runner/broker integration tests for generic
contracts. Put Cloudflare installation/service/container tests in the external
remote package. Add receipts and native VM/container gates through existing CI
and TASK-025 qualification conventions. Candidate areas are the existing test
paths above, Qt package/remote CLI-client tests, TASK-039 capability tests and
package-owned fixture/qualification workflows; no production host is a fixture.

| Matrix axis | Required evidence |
| --- | --- |
| OS/architecture | Linux distribution/manager rows, Windows and macOS supported architectures; explicit unsupported rows; align TASK-025/032/033 |
| Installation owner | Native verified binary; apt/dnf/rpm where supported; Homebrew/pkg; approved Windows manager/service; Docker exact image digest; signed bundled/Flatpak replacement path |
| Runtime state | Missing, stopped, running, multiple clients, stale installed/active mismatch and shared services |
| Execution path | Local CLI, paired worker approved profile, admitted delivered bundle, CLI/UI/MCP parity and denied access |
| Connectivity | Separate management path, redundant connectors, single critical connector with approved local recovery; broker restart/reconnect/revocation |
| Failure phase | Discovery, fetch, verify, prepare, install, restart, health, audit, recovery and result delivery |

Installation methods are not assumed supported merely because they appear in the
matrix. Each enabled row needs native evidence or an explicit manual-only limit;
compile-only and mocked results are labelled and do not satisfy runtime gates.

## Security considerations and tests required

- Prove read-only/dry-run leaves dependency, locks, services, configuration and
  credentials unchanged. Verify secret byte/permission preservation privately;
  publish only redacted identities/digests and no test credentials.
- Inject checksum/signature/key failures, source substitution, malicious metadata,
  pin conflicts, stale plans, changed active executable and insufficient privileges.
  Assert no mutation before required verification and durable intent succeed.
- Reject unauthenticated, merely paired, expired, revoked, replayed, wrong-machine,
  wrong-digest and wrong-operation grants. Reconnect, new identity tokens and
  scheduler retries cannot renew a human approval. Include hardware-required
  policy enforcement and unavailable authenticator behavior from TASK-036/039.
- Crash/cancel/reboot at every mutation boundary. Verify per-resource exclusion,
  package-manager lock contention, partial native replacement, failed service
  restart, failed health check, rollback failure and unavailable old artifact.
  A known-vulnerable recovery is not reported as successful security remediation.
- Cut the exact tunnel carrying broker heartbeat during update; prove bounded
  authorized recovery or refusal before disruption. Reconnect/result retry must
  reconcile the journal, never duplicate apply or recreate the tunnel.
- Validate service/connector readiness, authenticated broker reachability,
  active binary/image version and an authorized end-to-end application probe.
  For the initial advisory, test the client's actual TLS verification path using
  controlled fixtures; a browser or unrelated curl result is insufficient.
- Exercise offline/stale/corrected advisory evidence, notification retries and
  audit write/flush failures. Assert before/after identity, actor, authority,
  verification, health and recovery history correlate without secrets.

## Implementation phases and acceptance criteria

1. Define failure injection, deterministic fake provider and evidence format with
   TASK-049; extend existing suites rather than invent parallel harnesses.
2. Qualify manual local update/recovery and TASK-051 assessment/notification using
   isolated fixtures. Fix contract gaps before exposing unattended mutation.
3. Run disposable native OS/service/container gates and broker connectivity-loss
   scenarios. Record tested package/Core/provider versions and supported rows.
4. Admit CLI/Qt/MCP parity and scheduling/policy gates; publish a reviewed matrix
   before opt-in unattended enablement, using existing owner release gates.

- [ ] Every enabled installation/platform row has before/after running-version,
  configuration preservation, end-to-end health and failure-recovery receipts.
- [ ] Unsupported/untested rows remain visible and cannot auto-update.
- [ ] Interruptions and duplicate broker/scheduler deliveries cannot replay a
  mutation; unresolved outcomes and failed recovery remain explicit.
- [ ] Critical tunnel restart is denied without authority and a proven management
  recovery path; authorized restart demonstrates restored broker connectivity.
- [ ] All five policies, lock conflicts, advisory coverage/freshness and
  authorization failures are exercised; Core remains provider-neutral.
- [ ] Existing preflight, package, broker and UI contract regressions pass. Reports
  distinguish mocked tests, native runtime proof, publication and actual remediation.

### Standalone change/recovery fixture evidence (2026-10-27)

Local tests cover exact operation/input approvals, versioned provider negotiation,
strict result transport, stale installed bytes, checksum/pin/owner rejection,
linked targets, concurrent locks, cancellation before mutation, failed validation
with safe restoration, corrupt backup/unrelated-target preservation, interrupted
replacement reconciliation, idempotent completion and separately approved emergency
vulnerable rollback. A full candidate CLI → resolver workflow → SDK-built helper
fixture performed dry-run/apply/reconcile/recovery against private fake binaries.
Qt offscreen checks compile the actual dialog and exercise separate approvals,
disabled premature actions and immutable private request snapshots. They do not
qualify a native live app, actual Cloudflare connectivity, macOS/Windows replacement,
a package manager/service/container, broker restart, remote heartbeat recovery or
application TLS. Those acceptance gates remain open; no live tunnel was updated.

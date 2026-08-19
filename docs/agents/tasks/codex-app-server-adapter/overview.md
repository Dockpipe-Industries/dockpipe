# TASK-013 Codex App Server adapter for top-level orchestrators

## Epic

### Problem statement

Pipeon and future top-level DockPipe orchestrators use host codex exec, buffered output and transcript-file discovery. That preserves native workspace sandboxing but cannot represent live turn state, native approvals, interruption completion or connection loss reliably. This is separate from successful disposable codex exec workflow workers.

### Expected value

- replace transcript/timestamp discovery with typed thread ownership;
- give Pipeon live progress, follow-up, user-input, approval, cancellation and recovery states without raw provider protocol;
- improve audit evidence with correlated provider events and decisions;
- preserve Codex sandbox/escalation and the user's selected native approval authority;
- retain a provider-neutral contract and CLI fallback.

### Decision status

**Prototype successful; founder migration direction accepted; implementation in progress.** CAS-01
through CAS-13 proved the constrained, package-owned adapter boundary through controlled
integration. CAS-14 makes App Server the primary/default adapter for one Pipeon consumer while
retaining governed `codex_exec` fallback and bounded workers. Canonical research:
`docs/research/codex-app-server-top-level-orchestrators-2026-07.md`.

Repository has no ADR convention; this task is the accepted product-decision record, not an ADR.

### Current state

- The protocol spike, provider-neutral contracts, supervision, lifecycle, approvals, cancellation,
  recovery, persistence, audit, security fixtures, and controlled Codex integration are complete.
- The boundary remains package-local. The first Pipeon migration seam carries and session-pins an
  explicit Codex adapter choice. New normal Pipeon Codex sessions now capture
  `codex_app_server` by default, while the explicit configured `codex_exec` escape hatch applies to
  later new sessions. Existing and migrated Pipeon sessions retain one immutable closed choice, and
  legacy sessions without explicit resource, workspace, or user evidence conservatively retain the
  historical `codex_exec` default. An explicit `codex_app_server` choice supports a first
  host-resident turn and later turns only after fresh-child model/policy revalidation plus exact
  verified-idle recovery. Its transient approval, user-input, and user-requested cancellation
  controls now render in Pipeon with exact one-time delivery. Eligible failures proven before any
  `turn/start` attempt now roll back only the exact newly claimed turn and fall back once through the
  governed buffered `codex_exec` route. Provider-pool bindings remain authoritative and unchanged.
  Retained App Server sessions now persist and render one strictly normalized post-turn neutral
  status snapshot from the existing closed provider-pool response. Sessions without a snapshot
  remain valid and gain no inferred history. A retained valid `recovery_required` snapshot with an
  exact unknown outcome now blocks another normal direct App Server prompt before messages,
  interactive monitors, MCP, provider-pool state, claims, fallback, replay, retry, or children. The
  host retains the adapter, snapshot, messages, and persistent warning unchanged; the webview only
  disables its existing Send action for ordinary Codex text from the allowlisted display projection.
  A package-private read-only recovery-candidate classifier now recognizes only the exact canonical
  App Server binding, prior strict-loader-validated verified-idle session state, and canonical
  unresolved next-turn claim for the same session. Repeated classification is byte-for-byte inert,
  and candidate status authorizes no child, provider call, reconciliation, claim removal, snapshot
  transition, new turn, or safe-continuation claim. First-turn uncertainty and every incomplete or
  mismatched evidence set remain non-candidates. A package-private recovery-only host operation now
  consumes only that exact candidate classification, strictly reloads the retained state, constructs
  one fresh supervisor with the retained provider-session ID, recovery evidence, model, and reasoning
  values, and invokes the existing `RecoverBaseline` exactly once. It has no prompt input or dispatch,
  always shuts the fresh supervisor down after the attempt, and returns only a non-authorizing
  observation or error. The binding, completed state, unresolved claim, Pipeon snapshot, messages,
  warning, guard, and Send state remain unchanged. The follow-up atomic-transition investigation
  proved that the provider-pool package's three independent binding, completed-state, and claim files
  cannot be safely coordinated with Pipeon's separately owned recovery-required workspace snapshot.
  The accepted product/storage direction now defines the truthful non-terminal post-state as
  `reconciled_outcome_unknown`: the exact retained provider thread satisfied the verified-idle
  recovery baseline, but the prior prompt outcome remains unknown and continuation, replay, retry,
  and normal Send remain unauthorized. It selects the provider-pool coordinator as the sole durable
  transaction owner of one canonical per-session aggregate, consumes the unresolved claim into that
  aggregate as permanent no-replay evidence, and makes Pipeon's retained snapshot a projection only.
  A future exact user decision bound to the aggregate would be required before a fresh later turn
  could be authorized. Maintainer acceptance on 2026-08-04 established the product/storage
  direction, and authorized Slice 1 now implements only the package-private canonical aggregate
  contract, strict bounded loader, and inert package-state path described below. They remain unused
  and unreachable from production, and no aggregate has been written. The dated pre-Slice-2
  primitive decision packet below now records the accepted all-platform research policy and the
  accepted current negative documentation result for Windows/NTFS/amd64, Linux/ext4/amd64, and
  macOS/APFS/arm64. Acceptance covers only the recorded `D`/`R`/`U` documentation findings. A
  focused Windows re-audit now documents an unprivileged NTFS rename-metadata
  sync candidate, but atomic first-publication/replacement and resulting power-loss visibility remain
  unresolved. Linux has a documentation-supported candidate sequence for later native evidence,
  while Windows and macOS retain required undocumented guarantees. The all-or-nothing
  documentation gate is therefore unmet: no storage primitive, API sequence, dependency version,
  platform allowlist, implementation, or evidence harness is accepted, prototype evidence remains
  unauthorized on every platform, and no support reduction or implementation choice is made.
  The authorized bounded transactional-store selection below now accepts the logical-one-aggregate,
  physical-per-session-SQLite direction and fixes an exact design/evidence baseline. The accepted
  dependency baseline is `modernc.org/sqlite v1.56.0`, its required
  `modernc.org/libc v1.74.4`, embedded SQLite 3.53.3, native `win32`/`unix` VFSes, rollback journal,
  exclusive connection locking, and `synchronous=EXTRA`. The closed version-skew qualification below
  proves every SQLite 3.53.4 fix irrelevant to the selected fixed schema, single-database/no-`ATTACH`
  contract, and bounded SQL surface. A separately authorized dependency-pin and test-only Windows
  smoke slice now pins those exact modules and proves the selected contract once on the current
  fixed NTFS Windows/`amd64` host with CGo disabled. A separate opt-in 10,000-cycle Windows native
  reader-publication cohort proves exact old reads, live-owner fail-closed contention, protected
  journals, and exact new reads across independent persistent child processes. The additional opt-in
  1,000-cycle Windows contention/forced-termination cohort below proves exact old-row hot-journal
  recovery and one later clean commit while an independent different-session database continues
  committing. None of these tests changes production storage source, the inert Slice 1 path/loader,
  aggregate, migration, or lifecycle behavior. The deterministic matrix now reaches the exact
  post-COMMIT-invocation/pre-result loss boundary through the pinned driver's native commit hook;
  its required genuine-commit-error row remains proven unreachable under the selected exclusive
  shape, so the complete-matrix claim remains open. The native Linux/`amd64` smoke below now
  qualifies the selected contract once on the current Pop!_OS/local-ext4 host with the exact
  platform compile-option set; broader Linux cohorts, macOS runtime evidence, reboot/power-loss
  evidence, and the production-use gate remain open. A shared cross-session
  database remains rejected because it would couple otherwise independent session writers.
  Migration, operations, storage implementation, observation,
  compare-and-commit, cutover, dispatch guards, decision controls, later-turn claiming, Pipeon
  projection, platform evidence acceptance, compatibility, and rollback remain separately gated.
  Slice 2 has not started; TASK-013 and CAS-14 remain open.
  Operations rollout, live event transport, and durable event-history rendering/retention have not
  started.
- The CAS-14 product direction is accepted. Its provider-neutral contract is complete, and the
  supervisor-only projections now cover stable model/reasoning, native approval/reviewer and
  sandbox policy, safe capability selection, provider-backed bounded user-input prompt
  normalization/lookup, exact one-time response delivery, and strict approval-request decisions.
  The normal Pipeon chat route now has the validated adapter-selection foundation plus bounded
  dispatch, idle-session continuation, the package-local approval-decision controller below, and
  one bounded concurrent MCP server chat slot across stdio and HTTP with transient exact approval
  request/decision transport, exact user-input request/response transport and Pipeon controls, the
  package-private cancellation-controller seam, the transient cancellation MCP transport, and the
  Pipeon cancellation control and immutable Pipeon session adapter cache below.
  The normal CLI and zero-value dispatcher still supply no approval, user-input, or cancellation
  source. The persisted adapter binding remains `codex_app_server` across the one-shot exec fallback;
  the first Pipeon-owned post-turn snapshot is durable, while live event streaming/cursors, full
  durable event history, operations evidence, and cross-platform acceptance remain incomplete.

### Pause / resume checkpoint — 2026-08-13

Feature implementation is intentionally paused at `js/dev` commit
`6752dce7c0540d68cb95e1f718ba0998ea0eae35` after the MCP package-ownership cleanup. CAS-14 and
TASK-013 remain open. Slice 1's package-private canonical aggregate, strict loader, and state-path
helper are implemented but inert and unused; no aggregate has been written. Slice 2 has not started.

The private-per-session SQLite transactional-store, dependency, and evidence direction recorded
below is accepted, but production storage, migration, authority cutover, recovery authority,
dispatch/projection integration, remaining platform evidence, later user-decision/claim lifecycle,
compatibility, rollback, and operations acceptance remain separately gated. Resumption requires a
fresh read of this checkpoint and current repository evidence plus explicit authorization for one
bounded remaining gate. This checkpoint authorizes no implementation, prototype, migration,
cleanup, live evidence run, or successor slice by itself.

### Scope

- Codex App Server adapter for host-resident long-running top-level sessions;
- generic session/state/event/approval contract usable by other providers;
- Pipeon as the first primary/default App Server consumer, retaining an explicit exec escape hatch;
- process supervision, persistence, audit, cancellation and recovery;
- controlled tests using installed Codex.

### Non-goals

- replacing bounded codex exec workflow workers;
- Codex-specific engine changes under src/lib or src/cmd;
- WebSocket/remote-control transport;
- DockPipe-fabricated automatic approval, silent authority expansion, inherited full access,
  thread shell-command, or raw protocol in Pipeon;
- broad production abstraction beyond the named first Pipeon consumer.

### Architectural constraints

- App Server runs on host; Codex native sandbox remains active.
- Codex decides whether escalation is needed and may review it under the user-selected native policy;
  DockPipe validates the policy, projects neutral records, and never fabricates approval.
- Host authority is never expanded silently or inferred from approval automation.
- Adapter owns provider JSON-RPC; generic contracts expose provider-neutral events only.
- Crash/disconnect is never reported as safe continued execution.
- codex_exec remains available and no active prompt is replayed into it.
- Pipeon uses the generic contract and receives first migration advantage.

### Security constraints

- stdio only initially; no external listener;
- workspace-write/declared roots by default; broader access requires conspicuous per-session choice
  and must not be inherited accidentally; thread shell-command remains rejected;
- expose only validated native reviewer modes; automatic review never means DockPipe blindly
  approves, and approval automation never implies broader sandbox access;
- approval uses process/thread/turn/item/request correlation and one-time persistence;
- no credential copying; redact sensitive raw payloads;
- unknown authority-expanding capabilities fail closed; experimental capabilities are gated
  individually rather than by a global enable-all switch;
- default deny on timeout, disconnect, stale event, schema mismatch or malformed message;
- append-only event/approval audit with gap detection and reconciliation.

### Migration and rollback

1. Complete constrained protocol spike for selected Codex version.
2. Add provider-neutral contracts and selectable codex_app_server adapter.
3. Make App Server primary/default for one Pipeon direct top-level session with an explicit
   `codex_exec` escape hatch and session-pinned policy.
4. Run contract/integration/security evidence review.
5. Migrate remaining consumers only after maintainer decision; retain codex_exec and bounded-worker behavior.

Rollback selects `codex_exec` for new sessions or administratively disables App Server. Existing App
Server sessions become Disconnected until explicitly reconciled; never replay an active turn. Retain
audit records and offer user-guided resume/fork only after recovery checks.

### Remaining implementation gates

- Implement the accepted single-consumer adapter and provider-neutral session-control surface.
- Widen the proven fixed model/human-review/workspace-write hardening into validated available
  model/reasoning plus independently selected native approval and sandbox policies.
- Prove fallback, rollback, local event retention, and user-visible reconnect/recovery behavior.
- Cross-platform controlled evidence beyond the completed Windows integration harness.
- Maintainer acceptance of implementation evidence before CAS-14 completion.

### Epic acceptance criteria

- Spike proves launch, initialization, start/resume, stream, approval+denial, interrupt, clean exit, child death, native sandbox and fail-closed recovery.
- Codex types do not leak into generic orchestration/Pipeon APIs.
- Approval cannot replay or cross-apply.
- Pipeon App Server default and governed `codex_exec` escape/fallback work together.
- Bounded worker codex exec remains unchanged.
- Schema gates, audit/security tests and operations/migration docs pass.


# TASK-049 Core dependency lifecycle architecture and interfaces

## Status, priority and initiative

**P1; proposed, decision ready, unclaimed.** Added 2026-10-26 after source review at
Core `1f2a687ffdcc35a9d204e39231e51cad30351a69`. This record coordinates the
**Security and Reliability — Dependency Lifecycle and Authentication Hardening**
initiative; it does not authorize implementation or host changes.

| Item | Priority | Ownership and relationship |
| --- | --- | --- |
| TASK-049 (this item) | P1 | Core lifecycle policy, contracts, orchestration and shared status |
| [TASK-050](../cloudflare-dependency-lifecycle/index.yaml) | P1 framework adoption; P0 exposure lane | Cross-repository delivery; Cloudflare implementation remains in dockpipe-packages |
| [TASK-051](../dependency-advisory-monitoring/index.yaml) | P1 | Generic advisory assessment and notifications; provider metadata stays package-owned |
| [TASK-052](../dependency-lifecycle-qualification/index.yaml) | P1 | Integration, recovery, platform and remote connectivity qualification |
| [TASK-010](../declarative-dependency-install-ux/index.yaml) | P1 lifecycle prerequisite scope | Existing installer trust and exact package selection/lock owner; do not duplicate |
| [TASK-036](../remote-node-onboarding/index.yaml) | P1 security enhancement scope | Pairing authorization, hardware-backed enrollment, worker credential expiry/revocation |
| [TASK-039](../unified-mcp-gateway/index.yaml) | P1 security enhancement scope | Shared authentication, mandatory hardware-backed privileged approval, scoped expiring grants |
| [TASK-042](../oidc-workload-federation/index.yaml) | P1 | Workload identity verification; separate from human authority |
| [TASK-041](../cloud-mcp-approvals/index.yaml) | P1 security integration scope | Optional Cloud approval delivery; local enforcement cannot depend on Cloud |
| [TASK-046](../native-remote-desktop/index.yaml) | P1 shared security refinement scope | Reuse enrollment/passkey assessment; desktop/media delivery retains its own scope |

P0 means an immediately exploitable affected installation needing remediation,
not the entire lifecycle or identity program. No machines were inventoried or
exploitation established by this planning pass. TASK-050 owns the initial
advisory exposure triage; unknown version/coverage is unknown, not healthy.
The priorities above apply to the stated initiative slices, not unrelated work
inside the wider existing tasks.

## Current architecture findings

| Inspected component | Current behavior and design consequence |
| --- | --- |
| `src/lib/domain/resolver.go`, `src/lib/infrastructure/resolver_metadata.go`, `resolver_for_capability.go` | Resolver assignment/profile and capability selection exist; no lifecycle provider contract was found in these paths. Extend optional resolver capabilities rather than build another registry/resolver. |
| `src/lib/application/internal/hostdependencies/{service,install}.go` | Workflow/package `dependencies.host` checks executable presence and runs platform install hints through approval. Presence does not establish a version, vulnerability state or running-service provenance. Preserve this preflight path. |
| `src/lib/infrastructure/package_inventory.go`, `packagecatalog/{install,install_dependencies}.go` | Installed package versions, transitive catalog dependencies, bounded archives, checksum verification and atomic archive publication exist. Host tool inventory and running versions are separate observations; reuse verified catalog/store operations for package artifacts. |
| [Exact package selection](../../../packages/version-pinning.md), TASK-010 | `dockpipe.lock.json` pins the producer CLI today. Project/package selection locks and active exact selection remain proposed. Do not treat the CLI lock as a host dependency lock or create a competing lock format here. |
| `src/lib/application/{run,run_steps,resolver_workflow}.go` | Resolver workflows enter the existing workflow runner. Lifecycle actions should invoke that runner and preserve cancellation, operation results and host/runtime choice. |
| `src/lib/infrastructure/remote/{broker,worker,executor,bundle_build}.go` | Enrolled workers execute approved profiles/delivered bundles; journals preserve interrupted outcomes as unknown and retries deliver results without replaying execution. Use the same broker/jobs, not a new update agent or transport. |
| `src/lib/infrastructure/operationrecord/{operation_result,operation_event}.go` | Structured results/events and event indexes exist. They are reuse points, not evidence of a durable, tamper-resistant update ledger or enforced audit success. Define retention and failure behavior explicitly. |
| `src/dockpipe-app/ARCHITECTURE.md`, `src/dockpipe-app/src/packages/presentation/PackageManagerDialog.cpp` | Qt consumes Core/CLI contracts. Extend shared status records; keep policy and update execution out of widgets. MCP shared capability ownership remains TASK-039. |
| External remote package README | DorkPipe retains scheduling. No dependency-specific scheduler was identified in inspected Core paths; integrate scheduled workflow invocations through that owner or operator-managed host scheduling. Inventory actual supported trigger contracts before implementation. |

## Problem statement

Executable presence and package installation do not tell an operator whether the
active dependency is safe, whether an update violates a pin, or whether a failed
upgrade can recover. Ad hoc self-updaters bypass execution, authorization and
history boundaries. Core needs one generic lifecycle use case with provider-owned
mechanics and the existing workflow/resolver/package infrastructure.

## Proposed design

1. Add optional, version-negotiated lifecycle capability contracts alongside the
   existing resolver model. Proposed operations: discover, assess candidates,
   plan, prepare/verify, apply, validate and recover. Names are design vocabulary,
   not new supported CLI/YAML/API syntax. Packages may implement them with
   workflows/assets/helper adapters; no mandatory in-process Go plugin ABI.
2. Observations identify dependency/provider, machine, install method, architecture,
   package owner/source, installed and active versions/digests, required-by scope,
   health, last observation and confidence. Multiple installations stay distinct.
   Missing permissions, unsupported methods and unparseable versions stay unknown.
3. Core orchestrates inventory -> assessment -> immutable plan -> approval/policy
   decision -> verified preparation -> mutation -> health validation -> history.
   Resolver packages own detection, candidate/version comparison semantics,
   installers, service actions and method-specific validation/recovery. Package
   installation still uses the existing catalog and stores.
4. An approved plan binds machine and resource identity, observed preimage,
   exact target/digest, resolver version, pin/lock scope, verification evidence,
   privileges, allowed restarts, health deadline and recovery actions. Revalidate
   at execution; drift requires a new plan. Dry-run performs no install, service,
   lock, configuration or credential mutation and cannot implicitly approve.
5. Serialize mutation per machine/dependency resource, including shared service
   use across projects. Persist a private recovery journal through supported state
   helpers before irreversible steps. Treat crash/timeout/disconnection as unknown
   until reconciled. Cancellation and failure use shared operation-result contracts.
6. Keep backup/recovery artifacts private; use package/state helpers for durable
   state and artifact scopes for run receipts. Recover verified prior bytes or
   package-manager state only when the method supports it. Never claim universal
   transactional rollback. A vulnerable rollback requires explicit emergency
   authority and a recorded unresolved exposure; safe forward recovery is preferred.

### Policy and pin semantics

| Policy | Required behavior |
| --- | --- |
| manual | Operator starts checks/updates; no unattended mutation |
| notify | Scheduled assessment and notification only |
| security-only | Only verified advisory fixes within constraints and explicit unattended authorization |
| compatible | Provider-defined compatible candidates within declared ranges and explicit unattended authorization; unknown compatibility blocks |
| latest | Latest supported stable candidate permitted by target/platform constraints and explicit unattended authorization; never implicit prereleases |

Default existing installations to manual; notify is an explicit nonmutating
schedule choice. Pins and locks constrain every policy, including latest and
security-only. A vulnerable pin creates an urgent conflict requiring an explicit,
reviewable pin change; never silently rewrite producer, project or package locks.
TASK-010 owns package selection semantics; implementation must define conflict
precedence with shared host dependencies and existing ecosystem lockfiles.

Scheduling submits the same check/update workflows through existing DorkPipe or
host scheduler integration. Record trigger identity, policy version, maintenance
window/timezone, missed-run behavior, overlap lock and bounded retry/backoff.
The scheduler supplies timing, not privileged authority; expiry/revocation and
restart permission are checked again at use. No scheduler daemon is added to Core.

### Security and remote execution

- Delegate vendor download/signature semantics to providers while Core enforces
  verification policy and records evidence. Reuse catalog digest/size/origin
  validation for catalog artifacts; require authenticated metadata, trusted key
  identity and available signatures/checksums for external tools. Missing evidence
  is explicit; a checksum from an untrusted source is not authenticity proof.
- Apply least privilege, bounded downloads/advisory payloads, safe extraction,
  cancellation and secret redaction. Reject untrusted installer commands, source
  substitution, stale plans and verification bypasses.
- Pairing is machine enrollment, not permission to install, elevate or restart.
  TASK-036/039 own scoped authority and hardware-backed human approval; TASK-042
  identity tokens cannot renew grants. TASK-041 is optional delivery only.
- Run remote discovery and update on the target via existing approved profiles,
  broker jobs and bundle admission. Check target capabilities/protocol versions;
  unsupported workers fail explicitly. Never convert broker connectivity into sudo.
- Updating the tunnel carrying broker traffic can cancel the running job when its
  heartbeat fails. TASK-050/052 must qualify a preauthorized local recovery path,
  independent management channel or redundant connector before allowing restart.

## Components and phases

Candidate areas, subject to bounded implementation design: `src/lib/domain/dependency/`,
resolver metadata, `src/lib/application/internal/hostdependencies/`, a focused Core
lifecycle application service, existing package inventory/catalog and state helpers,
`src/lib/infrastructure/remote/`, operationrecord, CLI command routing, Qt package/
remote views and TASK-039 native capability adapters. Authored fields require
workflow/package schema, canonical docs and pinned external PipeLang/editor owner
coordination; no copied SDK or Cloudflare logic enters Core.

1. Agree capability/wire contracts, inventory schema, policy semantics and TASK-010
   lock boundary. Review TASK-036/039 threat model and denied paths first.
2. Ship manual read-only inventory/assessment, dry-run plans and structured CLI
   status/history. Prototype a fake provider; TASK-051 ingestion can progress here.
3. Add approved local apply/verify/recovery and target-side broker execution;
   integrate TASK-050 against the stable contracts.
4. Expose the same records/actions in Qt and MCP. Add scheduled notify first,
   then opt-in automatic updates after TASK-052 qualification and authority gates.

## Acceptance criteria

- [ ] One resolver discovery path, one workflow runner and existing package stores
  serve inventory, plan and update. A non-Cloudflare fixture proves generic behavior.
- [ ] All five policies, pin conflicts, installed versus active state, unsupported
  methods and stale/unknown observations have explicit machine-readable outcomes.
- [ ] CLI supports headless inventory, check, plan/dry-run, approved update,
  history and recovery; UI/MCP consume the same contracts and enforce the same gates.
- [ ] Each attempt records actor/trigger, target, before/after versions/digests,
  advisory/source freshness, policy/approval IDs, verification, restart, health and
  recovery outcome. Secrets never enter audit. Define audit retention/access and
  fail-closed behavior before mutation if required durable intent cannot be written.
- [ ] Lock/pin compatibility, cancellation, per-resource serialization, approval
  expiry/revocation and verification remain enforced remotely and during recovery.
- [ ] Scheduled checks/updates use existing trigger infrastructure; no second
  resolver, scheduler, package framework or provider-specific Core branch appears.

## Tests required

Unit/contract tests for policy selection, version uncertainty, lock precedence,
compatibility, preimage drift and metadata negotiation; negative tests for denied,
expired/revoked/replayed authority, missing signatures, checksum/key mismatch and
malicious metadata; integration tests for audit write failure, cancellation,
concurrent updates and recovery journals. TASK-052 owns end-to-end/platform proof,
including CLI/UI/MCP parity and a generic fixture provider. Existing preflight,
package resolution and broker regression suites must remain passing.

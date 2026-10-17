# DorkPipe Integration Direction — Audit and Staging Checkpoint

Recorded 2026-10-17. Direction/reuse audit completed at the document and inventory level;
implementation design and deployment guarantees remain separate qualification gates. This draft does not approve new feature implementation, retirement or extension renames.
The user separately authorized the DorkPipe repository split and related staging checkpoint;
report those operations independently from this feature audit.

## Agreed Direction

- DockPipe owns generic runtime, versioned package lifecycle, extension contracts and
  unified MCP. DorkPipe owns AI, knowledge, graph/context, models, chat and AI authoring.
- One DockPipe desktop application; DorkPipe contributes features to it and to CLI/MCP.
  Ordinary local use requires no separate AI container stack or managed service.
- Work core-up: shared networking and package/version foundations, then unified MCP,
  then DorkPipe graphs, knowledge and context. Skills management, chat, Models UI and richer authoring
  remain deferred. Supported local/remote/hosted execution remains user-selected and governed.
- A DockPipe editor integration covers DockPipe and PipeLang, with optional DorkPipe AI
  chat. Basic language support remains available without AI installation or activation.
- Pipeon retirement is tentative. Preserve existing packages/extensions and user data.
- AGENTS.md becomes a minimal local bootstrap; unified MCP routes knowledge/context to
  DorkPipe, with provenance, source access boundaries and safe offline behavior.
- Keep public repositories focused on technical contracts. Private planning context must
  not enter public docs, retrieval outputs or shared caches.

## Audit Findings and Boundaries

| Finding | Disposition |
| --- | --- |
| TASK-008 described a separate DockPipe AI package/app | Corrected to DorkPipe contributions in the existing app |
| TASK-012 still directed Pipeon startup investment | Marked paused historical scope pending retirement review |
| ForgePipe task still described a separate AI app package | Corrected ownership pointer; engine scope unchanged |
| Two extension identities and exact frozen VSIX pins already exist | Preserve current delivery; require explicit migration and rollback plan |
| Model execution risked gating knowledge delivery | Basic indexing/search and explicit graphs require no inference |
| MCP, knowledge and chat could duplicate configuration/state | Shared domain operations and authoritative CLI/API; surfaces consume them |
| Version selection and contribution lifecycle are incomplete | Qualify exact-version command/view/tool activation before dynamic integration |
| Existing release/source-retirement review predates these decisions | Reconcile its guarded docs and scope before resuming; do not apply stale postimages |

## Direction Audit — Result and Limits

The ownership and delivery sequence are coherent as a target. Implementation readiness
is not established. This is a backlog/document audit using existing recorded reuse
inventories; it does not qualify source behavior, protocol interoperability, security,
platform UI, release readiness or provider guarantees.

| Area | Reuse first | Gap or decision before building |
| --- | --- | --- |
| Package delivery | Existing catalog, manifests, verified artifacts and independent pipelines | Exact-version selection must bind contributions and dependencies; no global-version override |
| MCP | Existing bridge transports, policy checks and client code | Generic core extraction, supported protocol matrix, registration/collisions and compatibility |
| App integration | Existing launcher commands, navigation, workflow canvas and event consumers | Small reviewed contribution seam; no new UI framework or unrestricted package loader |
| Knowledge/context | Existing DorkPipe evidence, retrieval and graph inventory | Prove local ingestion/search, decision supersession and durable authoritative writes; an index is not the source of truth |
| Agent bootstrap | Current AGENTS rules and source routing | Define bootstrap fields, host setup, unavailable-context behavior and migration/export before removing files |
| Context authority | Existing runtime policy and permissions | Retrieved text cannot override repository rules or approve actions; conflicts must be visible |
| Privacy | Existing project/source scope and credential references | Filter before retrieval/snippets/traversal; isolate caches, exports and public/private targets |
| Chat/models | Existing guarded provider/session/event adapters | Shared sessions, context visibility, cancel/resume and user-selected targets; no default new service stack |
| Skills manager (deferred) | Existing curated skill sources, target-neutral IDs and Codex/Claude/generic renderer | DORK-002 owns catalog/provenance, version/scope selection, target rendering and safe update/removal; skills never grant permissions |
| Editor/Pipeon | Two published extension identities and frozen consumer pins | Retirement remains undecided; inventory settings/sessions/commands and define migration/rollback |
| Identity/approvals | Existing authentication and approval reuse candidates | TASK-042 identity, TASK-039 capability policy and TASK-038 domain grants stay separate |
| Lifecycle/data | Existing package state and operation results | Disable/update/uninstall semantics for watchers, workers, contributions and retained data |
| Server networking | Existing container policy, Docker modes, policy records and MCP TLS/auth | TASK-043 must define ingress/egress/config precedence and verify strict restrictions per backend; no implicit public exposure |
| Delivery scope | Existing standalone staging receipts and prepared integration | Refresh guarded integration after doc changes; no stale application or new feature release claims |

### Proposed First End-to-End Acceptance

Using an explicitly selected DorkPipe version, register a local Markdown fixture source,
retrieve a scoped decision/task through the one DockPipe MCP endpoint with provenance,
and inspect the same source/context through CLI/API and the existing app contribution.
Prove that superseded/conflicting decisions are surfaced, denied/private sources never
appear, bounded queries cancel safely, and MCP unavailability follows the local bootstrap.
No inference, chat, separate container stack, Cloud service or Terraform apply is needed
for this first knowledge integration proof. Start with existing source formats; do not
build a new universal knowledge store before proving this workflow.

### Migration and Defer List

Preserve versioned specs, essential local rules and current supported clients. Markdown
can remain an input/export format; reducing its routing burden is the goal, not deleting
all docs. Inventory provenance, authoritative records, write authorization, backup/export,
rebuild and rollback before moving task/decision ownership into durable knowledge data.
Audit host MCP setup rather than assuming AGENTS text can configure every client.

The DorkPipe repository split is separately authorized. Defer Pipeon removal, extension ID changes,
skills management, full model management, broad autonomous authoring, remote/mobile UI and optional Cloud
approvals. Terraform stays a separate package workstream after generic contracts; it
must not dictate or block the first local knowledge slice.

## Server Networking Addition

[TASK-043](../configurable-networking/overview.md) owns generic configurable ingress,
egress, proxies and effective networking diagnostics. This is core runtime work,
not DorkPipe AI domain logic. It coordinates gateway listeners, imported processes,
OIDC fetches, native sandbox and remote execution without owning VPC/NAT provisioning.
Audit administrator ceilings, trusted reverse proxies, TLS termination, secret redaction
and advisory versus enforced restrictions. Basic local stdio MCP and offline knowledge
remain independently deliverable; publicly reachable or privileged deployments must
meet their actual enforcement/authentication requirements before release.

## Deferred Skills Manager

DorkPipe's DORK-002 owns skills management, extending the existing skill renderer and
provider adapters. Target a Skills view inside DockPipe over shared CLI/API operations,
with authorized skill discovery/context access routed through the unified MCP gateway.
Keep canonical skill content, source/version/digest, project/global scope, target render
status and conflicts inspectable. Reuse knowledge/context for relevant selection without
conflating skill instructions with factual knowledge or execution policy.

Audit package/version provenance, local/imported sources, compatibility, explicit activation,
reviewable install/update/export/removal and rollback. Do not silently overwrite user-edited
host skills or make skill metadata an approval grant. Treat imported instructions/assets as
untrusted and require explicit permission for executing bundled scripts. Exact contribution
and protocol shape are design work, not existing commands or automatic MCP behavior.

This is deferred vision only and does not block networking, MCP or graph/knowledge delivery.
The authoritative domain backlog is docs/agents/tasks/skills-manager/index.yaml in the
current DorkPipe repository; DockPipe owns only generic package/contribution mechanisms.

## Repository Split and Staging Checkpoint

The user authorizes private `dorkpipe` → `dorkpipe-platform`, a new public `dorkpipe`
for reusable package/MCP/editor/AI sources, and private platform ownership for website
and integration-specific work. Preserve private history and releases; migrate only the
reviewed snapshot with attribution. This supersedes earlier repository deferral records.

The bounded source audit found 627 tracked files plus seven backlog files in the reviewed
snapshot, with no credential/private deployment matches in the performed pattern scans.
This is not an exhaustive secret or licensing qualification. Existing publisher checks
require a private source repository, and the pinned Go SDK lives in a private release
with `distribution_approved: false`. The owner explicitly approved a new generic Go SDK
distribution; version 0.1.1 now has new approval/provenance metadata, unchanged audited
source archive bytes and verified anonymous download. Public source publisher visibility
checks were repaired and tested. Private native SDK approvals remain separate.
Preserve immutable old SDK/package assets and use new provenance where source identity changes.

Before new feature work, finish this audit and promote only related reviewed changes
across the affected repositories through js/dev → dev → staging. Re-admit each checkout,
protect unrelated work, qualify hosted checks and verify artifact/source/catalog pins.
Do not publish production, retire Pipeon or sweep unrelated changes into these commits.

## Decisions for the Review

1. Audit the core-first sequence: TASK-043 networking and package/version lifecycle,
   then TASK-039 unified MCP, then DORK-001 graph/knowledge/context. Define bounded
   acceptance for each slice; chat is not a release prerequisite.
2. Confirm chat scope: DockPipe/project assistance with explicit context and governed tools;
   rich general IDE editing and autonomous agents remain outside the first chat slice.
3. Choose the extension migration after inventory: extend the existing language extension
   with optional chat, or retain a companion temporarily with an explicit transition.
   Branding alone cannot change published identities, settings or consumer locks safely.
4. Decide whether to retire Pipeon, and define preserved sessions/settings, reinstall,
   deprecation notice, artifact retention and rollback before any removal.
5. Review generic contribution seams and supported execution/isolation constraints. Avoid
   requiring a complete plugin UI framework before the first useful knowledge feature.

6. Confirm the minimal bootstrap and authority/fallback rules, including what must stay
   versioned locally and which clients need explicit MCP setup.
7. Review the first end-to-end acceptance above and the private/public source boundaries.
   Resolve data ownership and migration before replacing the Markdown routing tree.

## Unified Capability Refinement

TASK-039 now owns the [shared capability and interface contract](../unified-mcp-gateway/unified-capability-contract.md),
including explicit REST exposure, CLI operation with gateways disabled, existing package storage,
separate Terraform/Cloudflare domain authority and opt-in durable execution. Its design gate requires
a migration inventory and measured evidence that the registry does not burden ordinary local
commands. Privileged policy stays enforced through every interface. This refines the accepted
direction without starting feature implementation or changing the staging-first sequence.

## Audit Disposition

- Accepted direction: core networking/package-version foundations → unified MCP →
  DorkPipe graphs/knowledge/context. AGENTS bootstrap and MCP context routing are target
  contracts. Shared CLI/API operations serve app and gateway consumers.
- Deferred: skills manager, chat, model UI, rich authoring, editor identity migration and
  Pipeon retirement. None is required to finish this staging checkpoint.
- Repository split: GitHub private rename and distinct public repo creation are complete;
  public source snapshot and approved generic Go SDK are published. Private history,
  immutable old releases and original dirty checkout are preserved. Staging promotion
  and resulting package feeds require independent readback before marking delivery complete.
- Release repair: dev qualification found an existing policy-artifact publication/read race;
  repair and regression qualification are required before DorkPipe staging. Do not retry
  blindly or weaken the artifact-integrity checks.
- Implementation gates retained in owning tasks: network capability/enforcement and admin
  ceilings; version-scoped contribution lifecycle; MCP protocol/client compatibility;
  authoritative knowledge writes, privacy, provenance, conflicts, export and offline fallback.

This completes the direction audit without claiming a security review or shipping new
networking, gateway, knowledge, graph, chat or skills-manager functionality.

## Review Exit

Record accepted boundaries and unresolved decisions with the user, then select one bounded
implementation slice. Keep DORK-001 authoritative in the DorkPipe repository and TASK-039
responsible for generic integration. This draft does not mark either implementation complete.

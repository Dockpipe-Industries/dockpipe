# TASK-039 — Unified DockPipe MCP Gateway and Native Capabilities

Repository ownership note: package paths in this record describe external source. Current catalog packages live in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages); previously extracted [DorkPipe/MCP](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages) and [Pipeon](https://github.com/Dockpipe-Industries/pipeon/tree/master/packages/pipeon) remain in their own repositories. Historical paths and receipts do not recreate a local package tree.

## CLI-First Implementation Requirement

The [shared CLI-first management contract](cli-first-management.md) is mandatory for this task.
Core runtime and CLI/API own behavior, configuration and lifecycle; the existing desktop UI and MCP
exposure consume those capabilities. All essential operations must work headlessly. Reuse existing
package commands, backend services and UI patterns; no separate MCP configuration store or UI
business logic. Include the shared CLI/UI parity and process-lifecycle acceptance gates in delivery.

## Status And Objective

Backlog; architecture direction recorded on 2026-10-17. Foundation for **P1 TASK-038**. No runtime
implementation or deployment is claimed. The [full revised design request](design-request.md) is
preserved verbatim and defines the requested architecture; this overview routes implementation.

Provide one built-in logical DockPipe MCP endpoint with independently installable native capabilities
and imported third-party servers. MCP is an interface to execution, not a second execution engine.
Prefer compatible extensions of existing infrastructure and document any necessary replacement.

## Unified Capability Refinement

The [unified capability contract](unified-capability-contract.md) is the current architecture
refinement: protocol-independent package capabilities serve CLI, MCP, explicit REST and desktop.
It requires measured lightweight local execution with heavier authorization and durable services
only where needed. Read it before the earlier MCP-focused proposal; its conflicting assumptions
are superseded. No separate registry initiative or feature implementation is introduced.

## Federation Refinement

[P1 TASK-042: provider-agnostic workload federation](../oidc-workload-federation/overview.md)
extends this initiative with external OIDC verification and normalized workload identities.
TASK-039 retains the gateway authentication interfaces and profiles; TASK-042 owns trust registry,
validation and provider adapters. Keep workload authentication, capability authorization and
privileged operation approval separate. Existing API-key clients remain supported.

Direct external tokens are preferred where supported; MCP authorization compatibility must be
assessed before implementation. Optional exchange/adapters must not become a mandatory identity
platform or Cloud dependency. A fresh identity token cannot renew a human permission envelope.

## Current Architecture And Gaps

Bounded source inspection on 2026-10-17 found these reuse points. This is an initial gap analysis,
not protocol interoperability, provider-security, or production qualification.

| Area | Existing evidence | Required extension or investigation |
| --- | --- | --- |
| Bridge and transport | `packages/dorkpipe-mcp/README.md`, `mcpbridge/server.go`, `mcpbridge/http_server.go`: package-owned bridge, framed stdio, HTTP JSON-RPC, TLS configuration and per-key access tiers | Define compatible migration to a built-in generic gateway; validate supported MCP protocol versions/transports before claiming client compatibility |
| Registry and policy | `mcpbridge/catalog.go` has a static tool catalog; `server.go` filters listing and checks invocation with `ToolAllowed` | Dynamic package registrations, reserved namespaces, collision rejection, identity-bound profiles, removal/refresh and compatibility aliases |
| Package lifecycle | `docs/packages/package-model.md`, `src/lib/domain/package_manifest.go`: manifests, capabilities, compatibility validation, catalog/install/uninstall and versioned stores | Protocol-independent contribution contract in existing manifests/catalog/bucket; categories for discovery, existing storage conventions and explicit lifecycle semantics |
| Approvals | `mcpbridge/approval_transport.go`: transient correlated provider-pool approvals | Evaluate reusable contracts; these are not proof of durable human identity, expiring infrastructure grants, or restart-safe approvals |
| Client role | `packages/dorkpipe/lib/cmd/dorkpipe/mcp_client.go`: HTTP discovery and bounded tool calls | TASK-040 needs general downstream discovery/forwarding and stdio process lifecycle; do not treat this client as a complete import gateway |
| Terraform | `packages/terraform/resolvers/terraform-core/README.md`: existing provider-agnostic host workflow and backend inputs | TASK-038 must evaluate reuse and supply isolation, saved-plan authorization, durable execution and reconciliation |
| Secrets and persistence | `packages/secrets/tools/environment/` has 1Password/Infisical adapters; `packages/dorkpipe/lib/appserversupervisor/sqliteevidence/` contains SQLite evidence work | Inspect contracts before reuse; neither location establishes a generic credential broker or durable MCP job service |

The open task index has no other dedicated unified gateway/import owner. Coordinate with TASK-008
and TASK-013 where provider-pool clients and approval contracts overlap; do not absorb their UI or
App Server scope. Do not replace the existing bridge, package registry or storage bucket by default.

## Optional DorkPipe Knowledge Consumer

DorkPipe's **P1 DORK-001** owns the new knowledge/graph/context feature in its extracted repository.
The [ownership and extension record](dorkpipe-knowledge-boundary.md) identifies only the generic
CLI/UI/package/MCP dependencies retained here. Preserve optional installation, one application,
one MCP endpoint, shared domain implementations and headless use. This adds a native package
consumer, not a duplicate DockPipe knowledge initiative or any change to TASK-038 Terraform scope.
AI authoring and inspection in TASK-008 also belong to DorkPipe and extend the existing
DockPipe application; there is no separate DockPipe AI package/application. Ordinary
local AI features require no separate container stack, persistent daemon or managed
database. Generic contribution lifecycle must bind CLI, views and tools to the selected
package version. The private `dorkpipe-platform` / distinct public `dorkpipe` repository split is complete at
repository creation and source publication; resulting staging feeds still require qualification.

## Proposed Ownership

| Layer | Responsibility |
| --- | --- |
| Generic DockPipe gateway | Stdio and authenticated HTTPS, identity, discovery, registry, dispatch, audit hooks and package contribution lifecycle |
| Workload federation | TASK-042 verifies configured external issuers and maps claims; the gateway consumes verified identities without minting upstream pipeline identity |
| Shared runtime services | Reusable policy/grant/approval, vault-provider, durable job, polling, webhook and isolation contracts, introduced only when justified |
| Native packages | Domain schemas, interpretation, validation and executors; Terraform plans stay in TASK-038, direct provider API rules in their own capabilities |
| Import adapters | TASK-040 handles downstream MCP client transports and isolated server lifecycle without inheriting native execution authority |
| Cloud integration | TASK-041 optionally supplies remote decisions and notifications; local execution remains self-hostable |

Only generic primitives belong in `src/lib/` and `src/cmd/`. Keep DorkPipe, Terraform and provider
behavior package-owned, without checkout-relative package paths in the engine. The precise core
extraction and compatibility plan is a design deliverable, not permission for a blanket rewrite.

## Initial Requirements

- One logical endpoint, stdio and authenticated HTTPS, tool discovery/invocation, registration and
  removal, audit integration, and dynamic capability discovery where supported.
- Use the existing package registry, bucket and storage conventions. Categories support discovery;
  no separate MCP registry, bucket or mandatory MCP-specific prefix is required.
- Contributions declare stable ID/version/compatibility, tool schemas/descriptions, required
  capabilities, external dependencies, auth and authorization needs, isolation, optional config and
  necessary lifecycle hooks. Install/update/disable/remove capabilities independently. Installation
  never grants privileged authority. Keep schemas, docs and editor support synchronized when built.
- Stable package-qualified tool identities such as `dockpipe.terraform.plan`; reserve core namespaces,
  reject duplicate full names without overwriting, and define atomic registration/upgrade behavior.
  Preserve existing integrations with explicit compatibility mappings where practical.
- Identity-bound infrastructure/development/pipeline profiles filter discovery efficiently. Enforce
  permissions again on invocation, including aliases, stale catalogs, and guessed tool names.
- Where declared execution requirements need durability, reusable contracts cover IDs, states, leases, attempts, grant expiry/revocation,
  idempotency, audit, polling and optional signed webhook outbox/retries. Use SQLite initially where
  appropriate, short WAL transactions, and operations outside transactions. Do not add distributed
  infrastructure without a demonstrated need. Domain backends retain their own state.
- Shared authorization uses package/domain-bound scope, identity, policy version, expiration and
  integrity bindings. No universal operation parser or automatic cross-package grant inheritance.
- Local human authorization works without Cloud. External vaults retain secret ownership.

## Initial Threat Model And Required Proof

| Threat or boundary | Required control and acceptance evidence |
| --- | --- |
| Compromised caller guesses hidden tools or uses aliases | Invocation-time policy checks, identity-bound profiles and rejection tests independent of listing |
| Package upgrade or import shadows a privileged tool | Reserved namespaces, duplicate rejection, stable provenance and no silent schema/authority widening |
| Caller impersonates an approver or replays a grant | Distinct authenticated approval identity, bound scope/expiry/policy, revocation and replay/renewal checks |
| Imported server uses unrestricted credentials | Isolate credentials and processes; admit only authority the downstream boundary can enforce; fail closed for unsupported scoped guarantees |
| Compromised executor/provider leaks secrets or exceeds a plan | Controlled credential access and egress, isolation, sanitized artifacts, verified provider restrictions; a plan is not a full API-call inventory |
| Gateway or credential authority is compromised | Document the trusted computing base and residual authority; provider-native least privilege and temporary credentials bound impact but cannot make a compromised root of trust safe |
| Restart, duplicate submission or expired approval races | Durable IDs/leases/idempotency, authorization at execution, explicit in-flight expiry/revocation policy and partial-operation reconciliation |

Complete the threat model and enforcement matrix before privileged execution. Specify behavior for
unsupported MCP features, downstream outages, process cancellation, resource limits, audit retention,
package disablement during work, and exposed credentials. Security claims must match verified adapter
and executor guarantees. Aggregating a tool does not make its downstream execution safe.

## Dependencies And Implementation Sequence

1. Finish the architecture inventory and gap analysis, protocol compatibility review, security model,
   package schema decision, and generic-core migration plan before building.
2. Prove two independently installed native capabilities can register and execute through one
   endpoint with stable names, collision rejection, distinct caller profiles and no privilege gain.
3. [TASK-040](../third-party-mcp-import/overview.md) validates an existing third-party MCP through
   the same endpoint, including credential isolation and the unsupported-feature matrix.
4. [TASK-038](https://github.com/Dockpipe-Industries/dockpipe-packages/blob/master/docs/agents/tasks/standalone-terraform-mcp-executor/overview.md) is the priority native capability
   after the foundation. Implement shared services only as required by its governed vertical slice.
5. [TASK-042](../oidc-workload-federation/overview.md) builds on the gateway identity/profile
   interfaces for federated pipeline authentication; design its contracts alongside the foundation.
   It gates federated Terraform acceptance, not the basic gateway or API-key execution path.
6. [TASK-041](../cloud-mcp-approvals/overview.md) adds optional Cloud approvals later.

TASK-038 depends on this task's registry, loading, authentication and shared contracts. TASK-040
also depends on this foundation. Imports are required for the overall architecture but need not
block independent Terraform core work once those contracts are established.

## Acceptance

The shared-interface, migration and measured lightweight-execution gates in the
[unified capability contract](unified-capability-contract.md) are required alongside these gates.

- Two separately installed native capabilities share one endpoint; package updates/removal do not
  overwrite others, and duplicate names fail clearly.
- Clients with different profiles see different tools; denied invocations remain denied even when
  callers know the exact name. Installation, aliases and imports never expand grants implicitly.
- Existing supported clients retain compatibility or receive documented migration instructions.
- A separately imported server participates through the same endpoint after TASK-040 verification.
- Native Terraform runs through the gateway with the retained TASK-038 approval/durability scenario.
- No parallel registry, mandatory Cloud subscription, secrets vault, hosting/billing platform, or
  Biztraak replacement is introduced. Direct provider packages are justified separately; prefer
  maintained imports when they meet functionality and security needs.

- Federated clients pass TASK-042 issuer/audience/claim validation and capability checks without
  inheriting production authority. Token renewal does not renew human approval. Remote access has
  rate limits, request/execution quotas, audit and applicable tenant/environment isolation.

## Networking Coordination

[TASK-043](../configurable-networking/overview.md) owns generic ingress, egress, proxy
and effective enforcement contracts. Reuse its server/listener and workload policy
boundaries; reachability never grants capability or operation authority. Local stdio
functionality does not require the full server-networking initiative. Coordinate
strict isolation and remote exposure acceptance before claiming those deployments.

## P1 dependency lifecycle and hardware approval linkage (2026-10-26)

The [security and reliability initiative](../dependency-lifecycle/overview.md)
links TASK-049 lifecycle capabilities, TASK-050 provider implementation, TASK-051
advisories and TASK-052 qualification to this existing authentication/privileged
approval owner. No separate dependency MCP server, policy engine or identity
platform is proposed. Essential operations remain Core/CLI-owned and headless.

Refine shared contracts with TASK-036: mandatory hardware-backed FIDO2/WebAuthn
security-key authentication for pairing authorization, including authenticator
provenance policy, trusted
registration, RP/origin and user-verification checks, key revocation and explicit
recovery. Ordinary passkey support alone does not prove a hardware-bound key.
TASK-046's optional future FIDO2 assessment must consume this shared policy rather
than create an independent enrollment system. These are P1 proposed enhancements.

An approval binds authenticated human/trigger identity, machine, dependency,
installed preimage, exact target/digest, provider/policy version, privileges,
restart/maintenance scope and expiry. Recheck scope, expiry, revocation and replay
at execution and each privileged phase; a newly authenticated session or OIDC
identity never extends it. Automated updates require an explicit bounded policy
established through the applicable human gate. TASK-041 delivers optional Cloud
decisions; it does not replace executor verification or local approval.

Acceptance/tests: shared CLI/UI/MCP status and action parity; readonly discovery
cannot apply; merely paired machines and valid workload identities cannot elevate;
missing hardware verification, source/target drift, wrong operation, expired,
revoked and replayed grants fail closed. Include key-loss recovery, concurrent
requests, scheduled policy expiry and audit attribution in TASK-052. Actual
advisory exposure may require P0 remediation; this integration remains P1 absent
an independently established exploitable authorization finding.

Remote session refinement: use the [global remote auto-lock policy](../remote-node-onboarding/overview.md#remote-unlock-and-global-auto-lock-policy-2026-10-26-refinement)
as the single owner for inactivity and OS lock/sleep behavior. Routine remote
access unlocks through OS authentication and broker-verifiable session authority;
per-operation hardware prompts and remote approval delivery are deferred policy
extensions. Enforce expiry/scope on the server regardless of UI lock state; the
shared adapter must not accept an unlocked client flag as authorization.

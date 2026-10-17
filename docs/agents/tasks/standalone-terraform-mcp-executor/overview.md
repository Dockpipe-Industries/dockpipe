# TASK-038 — Native Terraform MCP Executor with Scoped Authorization

## CLI-First Implementation Requirement

The [shared CLI-first management contract](../unified-mcp-gateway/cli-first-management.md) is mandatory for this task.
Core runtime and CLI/API own behavior, configuration and lifecycle; the existing desktop UI and MCP
exposure consume those capabilities. All essential operations must work headlessly. Reuse existing
package commands, backend services and UI patterns; no separate MCP configuration store or UI
business logic. Include the shared CLI/UI parity and process-lifecycle acceptance gates in delivery.

## Backlog Status

- Priority: **P1**. Backlog only; implementation has not started.
- Added and revised: 2026-10-17.
- Depends on [TASK-039: unified MCP gateway](../unified-mcp-gateway/overview.md) for the
  registry, authenticated caller profiles, native package loading, and shared execution contracts.
- [TASK-040: imports](../third-party-mcp-import/overview.md) is a sibling integration and
  cross-package authorization acceptance dependency, not a Terraform execution-core dependency.
- [TASK-041: Cloud approvals](../cloud-mcp-approvals/overview.md) is optional future work.
- [TASK-042: workload federation](../oidc-workload-federation/overview.md) supplies shared OIDC
  authentication for the pipeline acceptance path; the Terraform execution core remains provider-neutral.
- Start with the capability and threat-model review; provider enforcement remains unverified.

## Architecture Decision And History

The revised [design request](../unified-mcp-gateway/design-request.md) supersedes the
[original standalone-server proposal](original-proposal.md). Keep TASK-038 and its existing
folder path for link continuity. Terraform now contributes tools to one built-in DockPipe MCP
endpoint; it does not host a separate public MCP server inside DockPipe.

TASK-039 owns generic transport, registry, package contribution, authentication, capability profiles,
and reusable authorization/execution service contracts. TASK-038 owns Terraform plan interpretation,
provider permission mappings, saved-plan execution, and reconciliation. Shared services must not
become a universal operation parser. Existing package and engine boundaries remain in force.

The execution core stays independently callable for CLI or possible future Biztraak reuse, without
requiring or replacing Biztraak. Native Terraform grants do not authorize direct provider tools or
imported servers. Installing the package grants no infrastructure authority.

## Objective

Design and implement an independently installable native Terraform MCP capability, distributed through the existing DockPipe package ecosystem and exposed through the unified DockPipe gateway.

The goal is to simplify local and CI/CD infrastructure operations while providing durable Terraform execution, external vault integration, dynamically scoped authorization, and optional human-in-the-loop approvals.

**Core principle: The caller owns infrastructure intent. DockPipe owns the governed execution and authorization boundary. The Terraform package interprets and executes Terraform operations. An external vault owns secrets.**

This must remain a focused execution tool, not an infrastructure management platform or replacement for Biztraak.

## 1. Deployment model

The package must support:

- Local MCP execution through the DockPipe gateway over stdio.
- Self-hosted remote MCP through the same logical gateway over authenticated HTTPS.
- Execution from GitLab CI, GitHub Actions, DockPipe, and other MCP-compatible clients.
- Deployment through DockPipe-managed environments, including local machines, NAS devices, containers, and cloud VPCs.
- No mandatory dependency on DockPipe Cloud or Biztraak.

Supported pipelines should authenticate with short-lived external OIDC tokens through TASK-042.
Existing API keys remain a fallback. Neither mechanism independently grants infrastructure-changing
privileges. Terraform consumes verified workload identity, not GitHub/GitLab-specific token logic.

TASK-039 owns MCP transport. Keep the Terraform execution core independently callable so additional client interfaces can use it without rewriting execution logic.

## 2. Core Terraform workflow

Expose a focused MCP tool interface:

- `dockpipe.terraform.submit`
- `dockpipe.terraform.plan`
- `dockpipe.terraform.status`
- `dockpipe.terraform.logs`
- `dockpipe.terraform.request_approval`
- `dockpipe.terraform.apply`
- `dockpipe.terraform.cancel`
- `dockpipe.terraform.revoke`

The consumer supplies Terraform configuration or a reference to a trusted workspace/artifact.

The Terraform package, using shared DockPipe services, must:

1. Accept and validate the execution request.
2. Establish an isolated Terraform workspace.
3. Initialize Terraform with controlled provider/plugin installation.
4. Generate and inspect a saved Terraform execution plan.
5. Determine the infrastructure operations and permissions requested.
6. Evaluate the requested changes against an authorization policy.
7. Obtain human approval where required.
8. Execute the authorized Terraform plan.
9. Persist execution state and return an execution identifier.
10. Support status polling, logs, cancellation, and completion webhooks.

The caller determines when to invoke Terraform and what to do with the results. The MCP server does not manage application deployment lifecycles.

## 3. Dynamic authorization envelopes

This is the primary differentiating feature.

Instead of approving an agent's entire session or an unrestricted cloud credential, support temporary authorization envelopes based on the infrastructure operations requested.

An authorization envelope should describe:

- Provider and target account/project.
- Allowed Terraform resource types.
- Permitted actions: create, read, update, delete.
- Permitted resource identifiers and constraints.
- Resource quantity and configuration restrictions where enforceable.
- Authorized execution identity/workspace.
- Approval identity and timestamp.
- Absolute expiration time.
- Policy version and integrity binding.

Default privileged authorization lifetime: **15 minutes, configurable**.

The same authorization envelope may cover multiple Terraform executions during its lifetime.

### Critical behaviour

The caller is allowed to modify Terraform configuration and resubmit jobs.

Every new plan must be evaluated against the active authorization envelope.

- Changes entirely within the approved scope may proceed.
- Changes outside that scope must be denied or require additional approval.
- Expired authorization cannot be renewed by the agent or pipeline itself.
- A fresh authorization must be issued by an authorized human where human approval is required.
- New sessions cannot bypass renewal restrictions.
- Revocation invalidates future privileged execution.
- Unknown, unsupported, or unverified permissions must fail closed.

Authorization must be enforced by a trusted execution boundary, not merely by trusting the agent's description of the work.

Where supported, use provider-native permissions and temporary credentials. Where exact provider-level scoping is unavailable, clearly distinguish gateway-enforced restrictions from provider-enforced restrictions. Never claim a stronger security guarantee than the implementation provides.

Approval must be checked at execution time, not only at job submission.

## 4. External vault integration

Do not implement a new secrets vault.

Integrate with existing credential providers through a pluggable interface, initially evaluating:

- 1Password.
- Infisical.
- Environment-specific workload identities.
- Other vault providers in future versions.

Requirements:

- Do not expose provider credentials in MCP tool results.
- Do not inject unrestricted production credentials into AI-controlled environments.
- Trusted executors should retrieve or broker credentials only after authorization.
- Prefer temporary provider-issued credentials and narrowly scoped credentials.
- Minimize credential lifetime and memory exposure.
- Prevent credential leakage through logs, plans, Terraform outputs, artifacts, process inspection, and error messages.
- Clearly define what revocation can enforce when an external provider issues long-lived tokens.

## 5. Durable execution using SQLite

Use SQLite as the default embedded execution-state database.

Support:

- Execution records and state transitions.
- Execution attempts and worker leases.
- Authorization grants and expiry.
- Idempotency keys.
- Workspace/state-lock coordination.
- Cancellation requests.
- Audit events.
- Webhook outbox, delivery attempts, and retries.

Prefer SQLite WAL mode with short transactions.

Terraform operations must execute outside database transactions.

Worker restarts must recover execution metadata. An interrupted Terraform apply must be treated as potentially partially completed and reconciled against infrastructure state before retrying.

Terraform state itself remains in the backend configured by the consumer.

Use a pluggable persistence abstraction so a distributed database may be introduced later without redesigning the core execution model.

## 6. Asynchronous execution and callbacks

Submission must return a durable execution ID without requiring the caller to maintain an open connection.

Support both:

**Polling**
- Query execution status.
- Retrieve progress and sanitized logs.
- Inspect approval state.
- Retrieve terminal execution results.

**Webhooks**
- Signed completion and failure notifications.
- Approval-required notifications.
- Retry with backoff.
- Stable event identifiers.
- Idempotent delivery semantics for consumers.

Webhook delivery failures must never affect execution correctness.

Clearly distinguish execution completion from webhook delivery completion.

## 7. Local human authorization

The package must support local human approvals through shared DockPipe authorization services without DockPipe Cloud.

Requirements:

- Approval of an explicit permission envelope.
- Configurable authorization duration.
- Explicit denial and revocation.
- Approval bound to an authenticated human identity.
- Default-deny behavior for privileged production changes.
- No automatic renewal of human-gated permissions.
- Auditability of approval decisions and the operations performed under them.

Routine development workflows may be configured for automatic authorization, while sensitive environments require human intervention.

## 8. Optional DockPipe Cloud integration

[TASK-041](../cloud-mcp-approvals/overview.md) owns optional remote approvals, mobile notifications,
team policies, history, and expiration/revocation notifications. It is not a prerequisite for local
or self-hosted Terraform execution. Cloud must not require custody of infrastructure credentials,
Terraform state, or full plan contents. Decisions must bind identity, requested scope, policy and
expiration cryptographically; the executor verifies them independently. Send minimal metadata.

## 9. Initial provider support

Prioritize the actual DockPipe infrastructure use cases:

1. DigitalOcean.
2. Cloudflare.

Build provider-specific permission mappings where necessary.

Explicitly determine which permissions can be enforced by native provider credentials and which require additional restrictions in the trusted executor.

Do not attempt universal provider support in the initial release.

## 10. Security and correctness requirements

Treat submitted Terraform configuration, providers, modules, provisioners, and execution artifacts as untrusted inputs.

Require:

- Isolated execution environments.
- No direct credential access by AI-controlled processes.
- Controlled outbound network access.
- Protection against MCP authorization bypass.
- Authenticated and authorized job submission.
- Strict separation of caller identities and approval identities.
- Approved-scope validation for every new plan.
- Saved-plan integrity checks.
- No silent permission escalation.
- Protection against concurrent state modifications.
- Secure workspace cleanup and artifact handling.
- Configurable behavior when authorization expires during an active apply.
- Recovery procedures for interrupted or partially completed applies.

Do not assume that a Terraform plan completely describes every API call performed by a provider.

Document enforcement limitations and ensure unsupported guarantees are not silently implied.

## 11. Scope boundaries

This package must not become Biztraak.

Explicitly exclude from the initial scope:

- Application lifecycle management.
- Managed hosting.
- Persistent environment orchestration.
- Billing or organizational account management.
- General-purpose secrets storage.
- A universal cloud permissions platform.
- A full infrastructure management dashboard.
- Replacing Biztraak's existing Terraform executor.

Shared libraries or authorization techniques may eventually be reused by Biztraak v3, but do not force architectural coupling.

## 12. Implementation approach

Before implementation:

1. Inspect the existing DockPipe package system, MCP conventions, runtime isolation facilities, approval mechanisms, and credential handling.
2. Review existing Terraform-related capabilities and reusable components.
3. Identify the smallest practical execution-core abstraction.
4. Research Terraform provider permission enforcement and temporary credential capabilities.
5. Produce a threat model addressing compromised agents, callers, MCP services, executors, and credentials.
6. Define the durable job state machine, authorization-envelope schema, and executor contracts.
7. Separate confirmed implementation capabilities from future enhancements.
8. Break work into independently deliverable milestones with acceptance tests.

Suggested milestones after TASK-039 establishes the gateway contracts:

**M1 — Native Terraform foundation:** Register the package's namespaced tools, keep an independently
callable execution core, and provide isolated planning, saved-plan inspection, durable SQLite job
state, polling, and sanitized results. Use the shared gateway transports and authentication.

**M2 — Authorization envelopes:** Package-owned plan analysis and provider mappings, shared policy
and approval contracts, configurable 15-minute grants, revision checks, execution-time enforcement,
revocation, and audit. Validate authority for every new plan and each apply.

**M3 — External credentials:** Reuse evaluated vault adapters, integrate DigitalOcean and Cloudflare,
and prove the distinction between provider-enforced and executor-enforced restrictions.

**M4 — Production durability:** Worker recovery, idempotency, cancellation, partial-apply
reconciliation, state locking, and hardening. Signed webhooks are optional to enable but remain a
planned delivery feature; polling is required. Webhook failure never changes execution outcome.

**Future integration:** TASK-041 adds optional Cloud approvals after standalone local authorization
is proven. Direct provider MCP packages and universal provider support are outside this task.

Milestones may be reorganized after inspection. Do not release privileged execution without the
security controls required for its deployment mode. Durable execution uses shared interfaces only
where they are reusable; Terraform state and Terraform-specific interpretation remain separate.

## Acceptance scenario

An AI agent requests Terraform execution to provision a DigitalOcean droplet and configure Cloudflare DNS.

The Terraform MCP generates and evaluates the plan.

A human approves a restricted 15-minute authorization envelope.

The agent applies the configuration.

The agent discovers a mistake, modifies Terraform, and submits another plan.

The new plan remains inside the approved envelope, so the correction proceeds without another human approval.

The agent subsequently attempts to create an additional unauthorized resource.

The MCP denies the operation.

After the 15-minute authorization expires, further privileged execution requires a new human authorization.

The entire sequence survives client disconnects, supports durable execution tracking, and records an auditable history.

**Success criterion:** A developer or CI pipeline can execute Terraform reliably through one reusable MCP interface, while infrastructure authority remains externally controlled, narrowly scoped, temporary, and auditable.

The result must be a small, useful, independently deployable DockPipe package—not another infrastructure platform.

## Unified Gateway Acceptance

- Install Terraform independently and discover its stable namespaced tools through the same
  authenticated endpoint as another native capability and a TASK-040 imported server.
- Different caller profiles expose only their permitted tools; direct calls to hidden or denied
  tools still fail server-side authorization.
- Execute the acceptance scenario above with no separate Terraform MCP listener or Cloud subscription.
- An approved Terraform grant cannot be reused by a direct Cloudflare tool or an imported server.
  Downstream credentials must not provide an alternate route around the approved authority.
- Package disablement/removal prevents new invocations while applying the explicit in-flight job
  lifecycle policy; it must not silently abandon or retry a potentially partial apply.


## Federated Pipeline Acceptance And Recovery

Use [TASK-042](../oidc-workload-federation/overview.md) for shared trust and identity validation:

1. A GitHub Actions, GitLab CI/CD or other supported workload obtains an external token intended
   for DockPipe and authenticates to the gateway without a permanent DockPipe API key.
2. Verified issuer-scoped identity and capability policy permit submission; Terraform produces and
   evaluates the plan. Sensitive changes require a separate human-approved permission envelope.
3. Execute approved changes within the configurable 15-minute window; poll or receive a webhook.
4. Reauthenticate after approval expiry. A valid new token permits only otherwise-authorized
   capabilities and does not permit another privileged apply until new human approval is issued.
5. Reject workflows/forks/branches/environments disallowed by verified-claim policies. Imported tools
   cannot reuse the identity or Terraform grant to bypass domain authorization.

Identity-token lifetime and infrastructure authorization lifetime remain independent. Cloud is
optional. Document a controlled local Terraform recovery path with human-held external-vault
credentials and backend locking/reconciliation when the remote executor is unavailable; recovery
must not require that same remote endpoint. TASK-042 owns the identity-bootstrap/recovery design,
while this package owns safe Terraform execution and partial-apply reconciliation.

## Shared Capability Architecture Refinement

Follow TASK-039's [unified capability contract](../unified-mcp-gateway/unified-capability-contract.md):
shared domain implementations serve CLI, MCP, explicit REST and desktop, with existing package
storage and version lifecycle. Ordinary local execution stays lightweight; required privileged
controls cannot be bypassed by selecting a lighter interface or execution mode. Domain ownership
and this task's existing security/acceptance requirements remain intact. Backlog only.

> Historical proposal, superseded on 2026-10-17 by the unified gateway design.
> The independent Terraform MCP server and original milestone ownership are superseded;
> use overview.md for current requirements and dependencies.

# TASK-038 — Standalone Terraform MCP Executor with Scoped Authorization

## Backlog Status

- Priority: **P1**.
- Status: backlog; implementation has not started.
- Added: 2026-10-17.
- Next step: repository capability review, provider enforcement research, and threat modeling as specified in section 12.
- The requirements below preserve the full submitted prompt; capabilities and milestones are proposed, not implemented guarantees.

## Objective

Design and implement a standalone, self-hostable Terraform MCP server distributed through the DockPipe package ecosystem.

The goal is to simplify local and CI/CD infrastructure operations while providing durable Terraform execution, external vault integration, dynamically scoped authorization, and optional human-in-the-loop approvals.

**Core principle: The caller owns infrastructure intent. The Terraform MCP owns governed execution. An external vault owns secrets.**

This must remain a focused execution tool, not an infrastructure management platform or replacement for Biztraak.

## 1. Deployment model

The package must support:

- Local MCP execution via stdio.
- Self-hosted remote MCP over authenticated HTTPS.
- Execution from GitLab CI, GitHub Actions, DockPipe, and other MCP-compatible clients.
- Deployment through DockPipe-managed environments, including local machines, NAS devices, containers, and cloud VPCs.
- No mandatory dependency on DockPipe Cloud or Biztraak.

Remote API keys authenticate callers to the MCP server. They must not independently grant infrastructure-changing privileges.

Keep the MCP transport separate from the core execution engine so additional client interfaces can be supported without rewriting Terraform execution logic.

## 2. Core Terraform workflow

Expose a focused MCP tool interface:

- `terraform.submit`
- `terraform.plan`
- `terraform.status`
- `terraform.logs`
- `terraform.request_approval`
- `terraform.apply`
- `terraform.cancel`
- `terraform.revoke`

The consumer supplies Terraform configuration or a reference to a trusted workspace/artifact.

The MCP service must:

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

The standalone package must support human approvals without DockPipe Cloud.

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

Provide an optional integration with the DockPipe Cloud API.

This must not be required for local or self-hosted operation.

Potential paid capabilities:

- Mobile push notifications for Terraform approval requests.
- Remote approval and denial from authenticated user devices.
- Centralized approval history.
- Shared team approval policies.
- Multi-environment authorization visibility.
- Approval expiration and revocation notifications.

DockPipe Cloud must not require custody of infrastructure credentials, Terraform state, or full Terraform plan contents.

Prefer transmitting minimal approval metadata, hashed/bound scope information, and signed authorization decisions.

The executor must independently verify the approval decision before granting access.

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

Suggested milestones:

**M1 — Terraform MCP foundation:** Local and remote MCP interfaces, isolated execution, durable SQLite job state, polling and basic results.

**M2 — Authorization envelopes:** Plan analysis, policy evaluation, 15-minute grants, revision detection, approval enforcement and audit trail.

**M3 — External credentials:** Vault adapters, DigitalOcean and Cloudflare integrations, and verified enforcement boundaries.

**M4 — Production durability:** Worker recovery, idempotency, webhooks, cancellation, reconciliation and security hardening.

**M5 — DockPipe Cloud:** Optional authenticated mobile approvals and remote notifications.

Milestones may be reorganized based on repository findings and security dependencies. Do not release privileged execution without the security controls required for its intended deployment mode.

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
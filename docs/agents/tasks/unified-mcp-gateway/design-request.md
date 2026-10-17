# DockPipe Unified MCP Architecture — Refactor Task 38

## Objective

Review and revise **Task 38** to align with a broader architectural direction for DockPipe: a unified, extensible MCP gateway integrated directly into the DockPipe runtime and package ecosystem.

The goal is to make DockPipe the single MCP endpoint through which AI agents, local tools, and CI/CD consumers discover and execute capabilities.

Individual packages contribute tools to this unified endpoint. Existing third-party MCP servers can also be imported and exposed through the same gateway.

**Core principle: One DockPipe MCP server. Multiple independently installable capabilities. One governed execution and authorization boundary.**

Terraform remains a high-priority use case, but it must no longer dictate the architecture of the entire MCP system.

## 1. Inspect existing architecture first

Before changing anything:

1. Locate and read Task 38 in full, including its current scope, requirements, dependencies, and acceptance criteria.
2. Inspect the existing DockPipe MCP implementation and its exposed capabilities.
3. Inspect the current package manager, package manifests, registry layout, installation mechanisms, and runtime isolation facilities.
4. Identify reusable authorization, human approval, execution, and persistence infrastructure.
5. Determine which parts of Task 38 are Terraform-specific and which should belong to a shared MCP execution framework.
6. Identify existing functionality that already satisfies these requirements.

Do not replace working infrastructure without a documented reason.

Prefer extending existing architecture over building a parallel MCP implementation.

Preserve existing MCP compatibility wherever practical.

## 2. Make MCP a first-class DockPipe feature

DockPipe should provide a built-in MCP gateway.

The gateway must support:

- One logical MCP server exposed to clients.
- Local stdio transport.
- Remote HTTPS transport.
- Client authentication and authorization.
- Tool discovery and invocation.
- Tool registration and removal.
- Package-based capability contributions.
- Per-client tool visibility.
- Execution routing.
- Audit integration.
- Dynamic capability discovery where supported.

The same DockPipe installation may expose multiple installed packages through one MCP endpoint.

Do not require each native package to launch or expose its own MCP server.

Keep the MCP protocol layer independent from underlying execution implementations.

MCP is an interface into DockPipe capabilities, not the execution engine itself.

## 3. Introduce an MCP package category

Extend the existing DockPipe package ecosystem to support MCP packages as a distinct category.

Use the existing package registry and storage bucket, organized under an appropriate MCP-specific prefix or folder.

Example conceptual structure:

```text
packages/
  ...

mcp/
  terraform/
  cloudflare/
  digitalocean/
  artifacts/
```

Adapt this structure to existing repository and registry conventions rather than introducing unnecessary parallel infrastructure.

Each MCP package should declare:

- Stable package identifier.
- Version and compatibility requirements.
- Exposed tools.
- Tool schemas and descriptions.
- Required capabilities.
- External service dependencies.
- Authentication requirements.
- Authorization requirements.
- Execution isolation requirements.
- Optional configuration.
- Lifecycle hooks where necessary.

Packages must be independently installable, updatable, disableable, and removable.

Installing a package must not automatically grant privileged access to its tools.

## 4. Unified tool registry and collision handling

Implement or extend a central tool registry.

Each tool must have a stable, uniquely addressable identity.

Use package namespaces to prevent naming collisions.

Examples:

```text
dockpipe.terraform.plan
dockpipe.terraform.apply
dockpipe.cloudflare.dns.update
dockpipe.artifacts.upload
```

Requirements:

- Reserve namespaces for DockPipe core functionality.
- Reject duplicate fully qualified tool registrations.
- Never silently overwrite registered tools.
- Clearly report namespace collisions.
- Support independent package upgrades.
- Avoid breaking existing client integrations unnecessarily.
- Maintain stable public tool identities where possible.

Consider aliasing and compatibility mappings for existing tools.

The registry must support authorization-aware discovery.

A client should discover only tools permitted by its configured capability profile, with permissions rechecked during invocation.

## 5. Support importing third-party MCP servers

This is a major requirement.

DockPipe should act as both an MCP server and an MCP client.

Users must be able to register existing third-party MCP servers and expose their capabilities through DockPipe's unified endpoint.

Initially support:

- Local stdio MCP servers.
- Remote HTTP-based MCP servers.
- Tool discovery through standard MCP mechanisms.
- Namespaced imported tools.
- Configurable authentication for remote servers.
- Lifecycle management for local imported processes.

Example:

```yaml
mcp:
  imports:
    - id: github
      transport: stdio
      command: github-mcp-server

    - id: external-terraform
      transport: http
      url: https://example.com/mcp
```

Imported tools should appear alongside native DockPipe tools without requiring agents to configure additional MCP endpoints.

Requirements:

- Preserve tool schemas and descriptions.
- Translate tool calls and results correctly.
- Support discovery refresh where downstream servers provide change notifications.
- Isolate imported processes.
- Prevent credential leakage across integrations.
- Authenticate and authorize calls at the DockPipe boundary.
- Prevent downstream credentials from being exposed to callers.
- Explicitly account for MCP features beyond ordinary tool invocation, including any unsupported capabilities.

Do not assume imported MCP servers are trustworthy.

Imported tools must not automatically inherit privileged DockPipe execution permissions.

Most importantly, **aggregation is not equivalent to secure execution**. An imported third-party MCP must not bypass authorization by performing privileged operations through its own unrestricted credentials.

## 6. Capability profiles and selective tool exposure

One unified MCP endpoint should not mean exposing hundreds of tools to every AI model.

Introduce capability profiles that determine which tools a client can discover and invoke.

Examples:

**Infrastructure profile**
- Terraform planning and execution.
- Cloud infrastructure inspection.
- Relevant approval and execution-status tools.

**Development profile**
- Repository tooling.
- Package operations.
- Development environment controls.

**Pipeline profile**
- Terraform execution.
- Artifact publishing.
- Status polling.

Profiles should be configurable by users and tied to authenticated caller identities.

Keep discovery efficient for LLM clients.

Do not load or advertise irrelevant capabilities when a narrower profile is available.

Authorization enforcement must remain server-side, independent of tool visibility.

## 7. Shared authorization and execution services

Introduce shared infrastructure only where it is genuinely reusable.

Potential services include:

- Authorization policy evaluation.
- Time-limited permission grants.
- Human approval workflows.
- Approval expiry and revocation.
- Vault-provider interfaces.
- Durable execution tracking.
- Auditing.
- Webhook notifications.
- Status polling.
- Worker isolation.

These services should be available to native MCP packages without requiring every package to implement them independently.

However, individual packages must retain responsibility for interpreting their domain-specific operations.

Terraform understands Terraform plans.

Cloudflare integrations understand Cloudflare API operations.

Artifact packages understand storage operations.

Do not build a universal operation parser.

A generic authorization service must not claim to enforce restrictions that the underlying provider adapter or executor cannot actually guarantee.

## 8. Preserve Terraform as the first major native MCP package

Refactor Task 38 so Terraform is an independently installable native MCP capability built on top of the shared DockPipe infrastructure.

Preserve its key requirements:

- Local and remote execution.
- Terraform planning and saved-plan inspection.
- Dynamic authorization envelopes.
- Human approval for sensitive infrastructure changes.
- Configurable 15-minute authorization windows.
- Reuse of approved authority across multiple compliant plans.
- Denial of operations outside the approved envelope.
- Integration with existing external vaults.
- DigitalOcean and Cloudflare as initial providers.
- Durable execution using SQLite.
- Polling and optional webhooks.
- Crash recovery and idempotency.
- Isolated Terraform execution.
- Independent Terraform state backends.

The Terraform package should not implement a separate public MCP server when hosted inside DockPipe.

Its execution core should remain independently callable so it can potentially be reused through CLI interfaces or by Biztraak in the future.

Do not force Biztraak onto this implementation.

### Terraform security contract

The caller may modify its Terraform configuration freely.

Every new Terraform plan must be evaluated against the currently approved authorization envelope.

Changes inside that envelope may execute during the approved window.

Changes outside it must be denied or require additional human approval.

Expired authorization must not be automatically renewed by an AI agent or pipeline when human approval is required.

The trusted executor must control credential access and Terraform apply execution.

Do not trust agent-supplied claims about the operations it intends to perform.

Provider-specific permission enforcement must be verified rather than assumed.

## 9. Distinguish Terraform MCP from direct provider MCP packages

Terraform MCP and Cloudflare MCP serve different purposes.

**Terraform MCP**
- Declarative infrastructure management.
- Terraform plan interpretation.
- Multi-provider execution.
- Scoped authorization across planned changes.

**Cloudflare MCP**
- Direct Cloudflare API capabilities.
- Individual DNS, zone, or other supported operations.
- Provider-specific authorization rules.

Both can coexist in the unified tool registry.

They must not automatically share privileged authorization grants.

An authorization issued for a Terraform operation must not implicitly permit an equivalent unrestricted direct API call through another MCP package.

## 10. Durable execution infrastructure

Use SQLite as the initial persistence implementation for durable local execution metadata where appropriate.

Support:

- Execution IDs.
- Job states.
- Worker leases.
- Execution attempts.
- Authorization grants.
- Idempotency.
- Audit events.
- Webhook outbox and retries.
- Recovery after service restart.

Keep Terraform state in its configured Terraform backend.

Avoid introducing PostgreSQL, Redis, or a distributed orchestration platform unless actually required by the deployment model.

Keep persistence interfaces extensible.

Do not turn DockPipe into Biztraak.

## 11. Optional DockPipe Cloud integration

Design an optional integration with DockPipe Cloud for:

- Remote human approvals.
- Mobile push notifications.
- Approval history.
- Centralized policy management.
- Team approvals.
- Execution notifications.

This must remain optional.

Self-hosted DockPipe must support local authorization without a DockPipe Cloud subscription.

DockPipe Cloud must not require custody of user infrastructure credentials or Terraform state.

Approval decisions must be cryptographically bound to the relevant identity, requested authority, and expiration.

## 12. Keep the product boundaries clear

DockPipe is the runtime and execution control plane.

The unified MCP gateway is a built-in interface to DockPipe capabilities.

MCP packages provide installable functionality.

Existing third-party MCP servers can be imported rather than reimplemented.

Biztraak remains a separate infrastructure management and orchestration platform.

Do not introduce unnecessary:

- Hosting platforms.
- Secret vaults.
- Billing systems.
- General-purpose infrastructure orchestration.
- Duplicate package registries.
- Parallel MCP servers.
- Provider integrations already adequately served by existing third-party MCP implementations.

Build native packages when DockPipe needs differentiated functionality, stronger security guarantees, or deeper runtime integration.

Otherwise, prefer importing an existing maintained MCP implementation.

## 13. Task 38 backlog cleanup

Update Task 38 to reflect this architecture.

Specifically:

1. Remove assumptions that Terraform requires its own independent MCP server.
2. Identify the general MCP functionality that belongs in DockPipe core.
3. Keep Terraform-specific implementation details inside the Terraform package.
4. Preserve the existing Terraform authorization and durability requirements.
5. Separate reusable infrastructure from Terraform-specific logic.
6. Identify dependencies on the unified tool registry, package loading, and authentication.
7. Add third-party MCP import support to the broader architecture.
8. Clarify which capabilities are initial requirements and which are future enhancements.
9. Split Task 38 into smaller backlog items where appropriate, with clear dependencies.
10. Preserve existing task history, relevant decisions, and useful acceptance criteria.

Suggested backlog structure:

**Foundation — Unified MCP Gateway**
- Built-in MCP server.
- Tool registry and namespacing.
- Client authentication.
- Capability profiles.
- Native package contribution contract.

**Extension — Third-Party MCP Import**
- Stdio and remote HTTP adapters.
- Tool discovery and forwarding.
- Imported-server lifecycle.
- Security and isolation.

**Task 38 — Terraform MCP Executor**
- Terraform execution core.
- Dynamic authorization envelopes.
- Vault integration.
- Durable execution.
- Initial DigitalOcean and Cloudflare support.

**Future — DockPipe Cloud Approvals**
- Remote approval service.
- Mobile notifications.
- Centralized authorization policies.

Reuse existing task identifiers and relationships where appropriate. Do not create duplicate backlog items when suitable ones already exist.

## 14. Implementation discipline

Start with a repository inspection and an architecture gap analysis.

Do not immediately implement an entirely new MCP subsystem.

Determine what already exists and what must change.

Prefer the smallest compatible extension to the current DockPipe runtime.

Before building, produce:

1. Current architecture summary.
2. Proposed unified MCP architecture.
3. Reusable components and existing implementations.
4. Required changes to Task 38.
5. New backlog items and dependencies.
6. Security boundaries and threat model.
7. Proposed implementation sequence.
8. Initial acceptance criteria.

Prioritize a vertical slice proving that two independently installed capabilities can register tools through one DockPipe MCP endpoint, without collisions or privilege escalation.

Then validate importing an existing third-party MCP server through that same endpoint.

Terraform remains the priority implementation after the gateway foundation is established.

## Success criteria

A user installs DockPipe.

They enable its unified MCP interface.

They install the native Terraform MCP package.

They import an existing third-party MCP server, such as the official Terraform documentation MCP or GitHub MCP.

All capabilities appear through one authenticated DockPipe MCP endpoint with stable namespaces.

Different client identities discover different permitted capabilities.

The Terraform package can accept infrastructure work, produce plans, request scoped authorization, execute approved changes, and maintain durable execution state.

A caller cannot bypass authorization by invoking an imported MCP tool or a different native provider tool.

DockPipe Cloud may optionally provide remote mobile approvals.

No separate Terraform MCP server is required, and no secrets vault is implemented inside DockPipe.

**Final architectural goal: DockPipe is one extensible, governed MCP runtime. Packages supply capabilities. Existing MCP servers can be imported. The human retains control over privileged execution.**
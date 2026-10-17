# P1 — Finalize DockPipe's Unified Capability, Package & Interface Architecture

## Objective

Review, consolidate, and correct the existing DockPipe architecture and backlog to establish a consistent, protocol-independent capability system.

This is the intended long-term foundation for DockPipe's package ecosystem.

The architecture must support independently installable packages that contribute functional capabilities to DockPipe. Those capabilities can then be accessed through MCP, REST APIs, the CLI, and the desktop UI without each package implementing separate versions of its functionality.

**Fundamental architectural principle:**

- Packages own what they do.
- DockPipe core owns how capabilities are registered, secured, executed, and observed.
- MCP, REST, CLI, and desktop UI are interfaces into those capabilities.
- Domain-specific authorization belongs to the package, using shared authorization primitives from DockPipe core.

The architecture must be local-first, lightweight, modular, headless-compatible, and independently deployable.

Do not create separate execution systems for each interface.

Do not turn DockPipe into a separate infrastructure orchestration platform.

This request is primarily for architectural consolidation and backlog correction. Inspect existing implementations and update the backlog before beginning substantial implementation.

---

## 1. Inspect existing architecture and backlog

Before proposing changes, inspect:

1. DockPipe's existing runtime and execution architecture.
2. The package manager, package manifests, installation lifecycle, and registry.
3. Existing CLI command infrastructure.
4. Existing desktop UI integration mechanisms.
5. The existing MCP server and unified MCP gateway work.
6. Existing REST/API services.
7. Authentication, OIDC federation, and API-key support.
8. Authorization, approvals, and security boundaries.
9. Execution persistence, auditing, logging, and observability.
10. Task 38 and its existing Terraform requirements.
11. Any Cloudflare-specific package or integration work.
12. Existing packages that may already contain reusable capability implementations.

Identify which requirements are already implemented, partially implemented, or missing.

Do not assume that new services, databases, or abstraction layers are necessary.

**Prefer adapting existing architecture over replacing it.**

Preserve existing working code, tests, data formats, and supported interfaces unless there is a documented reason to change them.

## 2. Core architectural model

The intended structure is:

```text
             Consumers
                 |
    +------------+------------+
    |            |            |
   MCP          REST         CLI
    |            |            |
    +------------+------------+
                 |
          DockPipe Core
                 |
    +------------+------------+
    |            |            |
 Capability   Auth/Policy   Execution
 Registry     + Approvals   + Durability
    |            |            |
    +------------+------------+
                 |
         Installed Packages
                 |
    +------------+------------+
    |            |            |
 Terraform   Cloudflare   Other Packages
    |            |            |
    +------------+------------+
                 |
         DockPipe Runtime
```

The desktop UI is another consumer of the same core services and capability contracts.

Packages must not be required to understand or implement the individual invocation protocols.

For example, the Terraform package must not need to implement an MCP server, a REST API, and a separate CLI execution engine.

It should implement Terraform functionality once and register the capabilities it supports.

DockPipe adapts those capabilities to the appropriate interfaces.

## 3. Unified capability registry

Introduce or formalize a central capability registry within DockPipe core.

Each registered capability should describe:

- Unique capability identifier.
- Owning package.
- Namespace.
- Version and compatibility information.
- Input schema.
- Output schema.
- Execution behavior.
- Supported interfaces.
- Authentication requirements.
- Authorization policy.
- Optional approval requirements.
- Execution isolation requirements.
- Durability requirements.
- Resource limits.
- Observability metadata.

Example conceptual capabilities:

```text
terraform.plan
terraform.apply
terraform.status

cloudflare.dns.list
cloudflare.dns.create
cloudflare.dns.update
cloudflare.dns.delete

artifacts.upload
artifacts.publish

knowledge.search
knowledge.index

models.start
models.stop
```

Each capability has one authoritative implementation.

The registry must:

- Prevent naming collisions.
- Enforce namespace ownership.
- Support package registration and removal.
- Validate capability contracts.
- Enforce version compatibility.
- Support runtime discovery.
- Support capability enablement and disablement.
- Preserve stable public identifiers.
- Integrate with existing package lifecycle management.

A package cannot register a capability into another package's reserved namespace without explicit authorization.

Capability registration must never implicitly grant execution permissions.

## 4. Native package architecture

Native DockPipe packages are the primary unit of extensibility.

A package may implement one or many related capabilities.

Examples:

### Terraform package

Owns:

- Terraform initialization.
- Configuration validation.
- Provider initialization.
- Plan generation.
- Saved-plan parsing and inspection.
- Terraform-specific permission analysis.
- Authorization-envelope evaluation.
- Apply execution.
- Terraform-specific recovery.
- Terraform state backend interactions.
- Terraform-specific execution diagnostics.

It consumes shared DockPipe services for:

- Caller authentication.
- Authorization grants.
- Human approvals.
- Credential-provider access.
- Durable execution tracking.
- Worker isolation.
- Logs and observability.
- Webhook delivery.

### Cloudflare package

Owns:

- Cloudflare API integration.
- API request construction and validation.
- Cloudflare-specific resource operations.
- DNS management.
- Zone management.
- Provider-specific permission mapping.
- Cloudflare-specific authorization constraints.
- Provider error interpretation.
- Relevant Cloudflare API schemas.

It consumes the same DockPipe core services for:

- Authentication.
- Authorization.
- Human approval.
- Time-limited access.
- External vault integration.
- Execution.
- Auditing.

### Other packages

Future packages may implement:

- Artifact storage and publishing.
- Model management.
- Container registry operations.
- Kubernetes operations.
- Knowledge management.
- Other developer infrastructure capabilities.

Do not add these implementations as part of this architecture task unless already required by existing backlog work.

## 5. Shared authentication and authorization

Authentication and generic authorization infrastructure belong to DockPipe core.

Packages must not independently implement their own authentication systems.

Shared infrastructure should provide:

- User identity.
- Workload identity.
- External OIDC verification.
- API-key authentication where appropriate.
- Package capability permissions.
- Time-limited authorization grants.
- Human approval workflows.
- Revocation and expiration.
- Audit records.
- Authorization context available to capability implementations.

### Domain-specific authorization

Individual packages remain responsible for understanding their own domain.

For example:

**Terraform** determines whether a plan is within an approved infrastructure permission envelope.

**Cloudflare** determines which API operation is being requested and whether its parameters satisfy an approved Cloudflare-specific policy.

DockPipe core provides the mechanism for policy decisions and approval enforcement.

Packages provide the domain-specific information required to make those decisions.

Do not force DockPipe core to understand every cloud provider, Terraform resource, or API operation.

### Security enforcement

Authorization must be enforced by trusted runtime components.

Never trust a caller's claim that an operation has been approved.

An MCP tool call, REST request, or CLI invocation must not bypass authorization.

For sensitive operations, the trusted executor must control credential access and execution.

If a provider cannot enforce fine-grained restrictions natively, distinguish provider-enforced restrictions from restrictions implemented within DockPipe's trusted boundary.

Do not claim that a permission has been enforced unless the implementation can actually guarantee it.

## 6. MCP as an interface adapter

DockPipe owns one unified MCP server.

MCP must not be the underlying execution engine.

Native packages register capabilities with DockPipe core.

The MCP interface makes permitted capabilities discoverable and invocable as MCP tools.

Responsibilities include:

- MCP protocol handling.
- Tool discovery.
- Schema translation.
- Invocation routing.
- Tool namespacing.
- Caller authentication.
- Capability-profile filtering.
- Authorization enforcement.
- Tool result translation.
- Dynamic registry updates.

Native packages should not have to implement MCP-specific code for ordinary capability exposure.

### Third-party MCP imports

Preserve support for importing external MCP servers.

Supported imports should include:

- Local stdio MCP processes.
- Remote HTTP MCP servers.
- Existing tools such as Glider and glider-trace.
- Other compatible third-party MCP implementations.

DockPipe acts as an MCP client toward imported servers and exposes their authorized tools through the unified gateway.

Local imported servers may run as DockPipe-managed child processes.

Imported MCP servers must remain distinct from native DockPipe capability packages.

Do not automatically treat imported third-party MCP tools as trusted native capabilities.

Do not grant imported tools unrestricted runtime, filesystem, or credential access.

## 7. REST API as an interface adapter

DockPipe must expose appropriate capabilities through authenticated REST APIs.

REST is primarily intended for:

- CI/CD pipelines.
- External automation.
- Scripts.
- Third-party applications.
- Remote DockPipe consumers.

Conceptual routes:

```http
POST /api/v1/terraform/plan
POST /api/v1/terraform/apply

POST /api/v1/cloudflare/dns

POST /api/v1/executions
GET  /api/v1/executions/{id}
GET  /api/v1/executions/{id}/logs
```

These are illustrative routes.

Follow existing DockPipe API conventions.

Do not automatically expose every installed capability as a public HTTP endpoint.

REST exposure must be explicitly supported and authorized.

Requirements:

- Versioned API contracts.
- Request validation.
- Consistent error handling.
- Identity-aware authorization.
- Idempotency where required.
- Appropriate synchronous and asynchronous execution.
- Durable job IDs where appropriate.
- Polling.
- Optional webhooks.
- API documentation.

REST must invoke the same underlying implementation as MCP and CLI.

No duplicate business logic.

## 8. CLI as a first-class interface

Preserve DockPipe's CLI-first design.

Packages should contribute commands through the existing CLI extension mechanisms.

Illustrative commands:

```bash
dockpipe terraform plan
dockpipe terraform apply
dockpipe terraform status

dockpipe cloudflare dns list
dockpipe cloudflare dns update

dockpipe artifacts upload
```

Use actual DockPipe command conventions when implementing.

The CLI must:

- Operate headlessly.
- Support scripting.
- Return appropriate exit codes.
- Support structured output.
- Use shared runtime capabilities.
- Enforce the same applicable authorization policies.
- Respect package installation and enablement.
- Avoid duplicate execution implementations.

A package may provide CLI functionality without requiring the MCP or REST interface to be enabled.

## 9. Desktop UI integration

The desktop UI must consume the same DockPipe core services.

Installed packages may contribute appropriate management interfaces and application features.

For example:

**Terraform**
- Execution history.
- Plans.
- Approval requests.
- Logs.
- Execution status.

**Cloudflare**
- Configured accounts.
- DNS management.
- Operation history.
- Approval status.

**MCP**
- Installed tools.
- Imported servers.
- Client connections.
- Capability profiles.
- Server status.

The UI must not contain an independent implementation of capability execution or authorization.

Use existing DockPipe UI patterns and navigation.

Do not automatically require every package to provide a custom UI.

## 10. Durable execution

Shared durable execution belongs to DockPipe core where appropriate.

Long-running operations should use common infrastructure for:

- Execution IDs.
- Job status.
- Worker leases.
- Execution attempts.
- Idempotency.
- Cancellation.
- Progress.
- Logs.
- Audit records.
- Human approval state.
- Webhook delivery.
- Crash recovery.

Use existing implementations wherever possible.

SQLite remains an appropriate default embedded persistence option if consistent with existing DockPipe architecture.

Keep execution metadata separate from package-specific data and external Terraform state.

Do not force simple synchronous capabilities into durable job processing unnecessarily.

A capability should declare whether durability is required.

## 11. Terraform — Task 38 correction

Review Task 38 in full.

Terraform must be a native DockPipe package exposing capabilities through the unified capability registry.

Do not implement Terraform as a standalone MCP-only server.

Preserve all existing Terraform-specific requirements:

- Plan generation.
- Plan interpretation.
- Dynamic permission envelopes.
- Human authorization.
- Configurable 15-minute grants.
- Multiple executions within an approved permission envelope.
- Rejection of unauthorized changes.
- External vault integrations.
- DigitalOcean and Cloudflare as initial providers.
- Isolated execution.
- Durable jobs.
- Polling.
- Webhooks.
- Crash recovery.
- State backend independence.

### Example

An AI agent invokes `terraform.plan` through MCP.

DockPipe routes the request to the Terraform package.

The Terraform package generates and interprets the plan.

DockPipe's shared authorization service evaluates the requested permission envelope using Terraform-specific policy information.

A human approves the required authority.

The Terraform package executes through the trusted runtime.

The result is returned through MCP.

If the same capability is called through REST or CLI, it must follow the same execution and authorization path.

### Iterative execution

An authorized caller may modify Terraform freely.

Every new plan must be re-evaluated.

Changes within an active approved envelope can proceed.

Changes outside it require additional authorization.

Expired grants must not be automatically renewed by agents or pipelines when human approval is required.

The caller's authentication token lifetime is independent of infrastructure authorization lifetime.

## 12. Cloudflare package architecture

Cloudflare should be implemented as a native DockPipe capability package.

It should not require a dedicated MCP server.

It registers Cloudflare-specific capabilities with DockPipe.

Those capabilities may then be exposed through:

- MCP for agents.
- REST for pipelines.
- CLI for developers.
- Desktop UI for human management.

Cloudflare-specific permissions and API validation remain inside the package.

Shared authentication and approvals remain in DockPipe core.

### Important authorization separation

An authorization grant for Terraform operations must not automatically authorize direct Cloudflare API operations.

Even when Terraform and Cloudflare manage the same infrastructure, they are distinct capability scopes.

The authorization service must account for:

- Package identity.
- Capability identity.
- Provider account.
- Target resource.
- Requested operation.
- Approved parameters.
- Authorization expiration.

A caller must not gain broader authority by switching from Terraform MCP to Cloudflare MCP or REST.

## 13. Package registry and distribution

Preserve DockPipe's existing package manager and package distribution infrastructure.

Do not introduce separate package registries for MCP, REST, or CLI.

Categories may be used for discovery, but must not determine runtime architecture.

For example:

```text
Packages
├── Infrastructure
│   ├── Terraform
│   └── Cloudflare
├── Storage
│   └── Artifacts
├── Knowledge
│   └── DorkPipe
└── Integrations
    └── Third-party MCP imports
```

The same package may expose multiple interfaces.

Package manifests should declare supported capabilities and interface contributions using existing conventions.

Avoid unnecessary changes to bucket layout if the current package registry already supports categories.

## 14. DorkPipe ownership

DorkPipe remains a separate optional package and repository.

Its ownership includes:

- Knowledge bases.
- Markdown ingestion.
- RAG.
- Semantic retrieval.
- Knowledge graphs and DAGs.
- Context management.
- Knowledge-specific capabilities.

DorkPipe registers these capabilities through DockPipe's shared registry.

DockPipe core owns the interfaces and generic execution services.

DorkPipe owns knowledge-specific implementations.

Do not move DorkPipe's code back into DockPipe core.

Keep DorkPipe backlog ownership within its own repository.

## 15. Provider-agnostic OIDC authentication

Preserve the existing external OIDC federation architecture.

DockPipe verifies externally issued workload identities from configured trusted providers.

Examples:

- GitHub Actions.
- GitLab CI/CD.
- Microsoft Entra.
- Kubernetes workloads.

Do not require DockPipe to become an identity provider.

Do not require a proprietary token-issuing service unless necessary for a specific supported protocol flow.

Workload identity verification must remain separate from privileged operation authorization.

The same authenticated identity should receive consistent capability permissions across MCP and REST.

Publicly exposed APIs and MCP endpoints must enforce authentication and authorization.

## 16. Observability

Every capability invocation should be observable through existing DockPipe tracing and diagnostic infrastructure.

Capture where applicable:

- Caller identity.
- Invocation interface.
- Package identifier.
- Capability identifier.
- Execution ID.
- Authorization decision.
- Approval reference.
- Execution status.
- Duration.
- Worker identity.
- Resource consumption.
- Errors and recovery events.

Avoid multiple unrelated tracing systems.

Sensitive credentials and protected payloads must never be exposed through ordinary telemetry.

## 17. Security and isolation

The shared capability model must not weaken existing execution security.

Requirements:

- Validate capability inputs.
- Enforce package boundaries.
- Prevent namespace collisions.
- Restrict credential access.
- Enforce authorization independently from transport.
- Prevent privilege escalation through alternate interfaces.
- Isolate untrusted package execution.
- Prevent imported MCP servers from bypassing trusted capability boundaries.
- Respect network restrictions.
- Audit privileged operations.

In-process execution may be appropriate for trusted first-party packages where supported.

Untrusted packages may require isolated workers or processes.

Do not force unnecessary containers for ordinary local capabilities, but do not sacrifice isolation to eliminate containers.

## 18. Backlog consolidation

Review the current backlog before creating new tasks.

Specifically inspect:

- Unified MCP gateway initiative.
- Native package capability work.
- Package manifest and registry changes.
- REST API work.
- CLI contribution mechanisms.
- UI extension mechanisms.
- OIDC federation.
- Authorization infrastructure.
- Durable execution.
- Task 38.
- Cloudflare integration work.

Reconcile overlap.

Update existing issues rather than creating duplicates.

Preserve comments, architectural decisions, acceptance criteria, and implementation history.

### Expected ownership

**DockPipe Core**
- Capability registry.
- Package integration interfaces.
- Authentication.
- Generic authorization.
- Approvals.
- Execution durability.
- Observability.

**MCP Interface**
- Unified gateway.
- Native capability tool exposure.
- Third-party MCP imports.
- Client configuration.

**REST Interface**
- Remote capability invocation.
- Authenticated pipeline access.
- Execution status and webhooks.

**CLI/Desktop**
- Capability command registration.
- UI contributions.
- Shared configuration.

**Terraform Package / Task 38**
- Terraform-specific execution logic.
- Plans.
- Permission envelopes.
- Provider mappings.

**Cloudflare Package**
- Cloudflare-specific API functionality and permission mapping.

**DorkPipe Repository**
- Knowledge, RAG, graph/DAG, and context capabilities.

Do not move domain-specific implementation logic into DockPipe core.

Do not create separate protocol-specific implementations inside individual packages.

## 19. Implementation sequence

Before implementation, produce:

1. Current architecture inventory.
2. Existing capability and command mechanisms.
3. Proposed unified capability contract.
4. Package lifecycle integration plan.
5. MCP adapter integration plan.
6. REST adapter integration plan.
7. CLI and desktop integration plan.
8. Shared authentication and authorization model.
9. Terraform Task 38 corrections.
10. Cloudflare package requirements.
11. Backlog dependency updates.
12. Migration and compatibility strategy.
13. Security risks and acceptance criteria.

The first technical validation should demonstrate:

- Two independently installed native packages.
- Both register capabilities through one shared registry.
- Both can expose supported capabilities through the same MCP gateway.
- At least one capability is also callable through REST and CLI.
- All interfaces invoke the same underlying implementation.
- Authorization cannot be bypassed by changing invocation interfaces.
- Disabling a package cleanly removes its capabilities.

Use the smallest vertical slice that proves the architecture.

Do not begin a broad package rewrite before validating this foundation.

## 20. Acceptance criteria

The architecture is successful when:

1. DockPipe core operates without requiring optional capability packages.
2. Terraform installs as a native package.
3. Cloudflare installs as a separate native package.
4. Each package implements its own domain logic.
5. Both register through the same DockPipe capability registry.
6. DockPipe can expose their supported capabilities through one MCP endpoint.
7. Pipelines can invoke supported capabilities through REST.
8. Developers can invoke supported capabilities through the CLI.
9. The desktop UI reflects installed packages and their relevant capabilities.
10. Authentication, authorization, approvals, and auditing remain consistent.
11. Terraform and direct Cloudflare permissions remain independently scoped.
12. Durable jobs and webhook support are shared where required.
13. Third-party MCP servers can still be imported through the gateway.
14. DorkPipe remains independently developed and optional.
15. No redundant execution engines, servers, or package registries are introduced.

## Final architectural contract

**DockPipe core provides the runtime, capability registry, security, execution services, and observability.**

**Native packages implement domain-specific functionality and register capabilities.**

**MCP, REST, CLI, and the desktop UI expose those capabilities through shared adapters.**

**Domain-specific policy interpretation stays with the package; approval and generic authorization enforcement stay with DockPipe core.**

**One implementation per capability, regardless of how it is invoked.**

Update the existing DockPipe backlog, including Task 38 and the relevant Cloudflare work, to reflect this architecture.

Preserve existing functionality, avoid duplicate tasks, and identify necessary migrations before implementation.

This is an architecture and backlog reconciliation task. Do not begin major feature implementation unless separately instructed.
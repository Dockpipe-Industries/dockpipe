# TASK-040 — Third-Party MCP Import Through the Unified Gateway

## CLI-First Implementation Requirement

The [shared CLI-first management contract](../unified-mcp-gateway/cli-first-management.md) is mandatory for this task.
Core runtime and CLI/API own behavior, configuration and lifecycle; the existing desktop UI and MCP
exposure consume those capabilities. All essential operations must work headlessly. Reuse existing
package commands, backend services and UI patterns; no separate MCP configuration store or UI
business logic. Include the shared CLI/UI parity and process-lifecycle acceptance gates in delivery.

## Status And Dependencies

Backlog; initial architecture requirement, added 2026-10-17. Depends on
[TASK-039](../unified-mcp-gateway/overview.md) registry, authentication, profiles and lifecycle
contracts. Implements sections 5 and 14 of the [design request](../unified-mcp-gateway/design-request.md).
TASK-038 is a sibling, not an implementation dependency. No external server has been connected
through a configured DockPipe runtime/gateway. The internal `mcpsession` owner now
uses the official Go SDK and existing process-tree supervision; neutral child
fixtures verify protocol/cancellation/recovery behavior. The internal `mcpimport` seam now requires injected trusted operation authorization
and an owned runtime lease, binding the existing compiled dockerimage policy.
Production authorizer/Docker executor, receipt reverification and CLI/gateway
exposure remain pending; neutral host fixtures do not establish Docker containment.
See [the precise component boundary](../../../runtime/mcp-registry.md#internal-session-owner-and-pending-application-integration).

## Scope

Make DockPipe both an MCP server and client. Register local stdio servers and remote HTTP-based
servers, discover their tools and expose them beside native packages through one endpoint.

- Give each import a stable identity and namespace; reject collisions with native/core tools.
- Preserve schemas, descriptions and result semantics; define call/error/cancellation translation.
- Support configurable downstream authentication without returning credentials to callers or sharing
  credentials between imports. Bind discovery and invocation to the caller's capability profile.
- Own startup, shutdown, failure recovery and bounded resource use for isolated local processes.
- Refresh discovery when downstream change notifications support it; re-evaluate permissions when
  schemas or tool sets change. Never silently widen authority after refresh or reconnect.
- Document the protocol/transport compatibility matrix and explicitly support or reject features
  beyond tools, including resources, prompts, subscriptions, sampling, elicitation and roots.
  Verify actual upstream contracts during implementation; examples in the request are conceptual.
- Extend the existing HTTP discovery/client and bridge where appropriate, after checking their
  assumptions; do not assume they already provide general MCP import support.

## Security Boundary

Imported servers and their metadata are untrusted. Authentication to DockPipe does not confer
privileged downstream credentials. Imported tools receive no native grants automatically, and a
Terraform grant cannot authorize a direct API/import call by equivalence of intended operation.

Aggregation is not secure execution: an imported process with broad cloud credentials can exceed a
claimed scope. Require an enforceable downstream credential/executor policy for privileged imports;
otherwise deny that use and document the limitation. Isolate process/filesystem/network/credential
access and test direct-call bypasses as well as discovery filtering. No shared secret injection.

## Federated Caller Boundary

[TASK-042](../oidc-workload-federation/overview.md) supplies verified upstream workload identities.
Keep namespace and identity mappings stable, authorize before forwarding, and configure security
explicitly per import. Do not forward external tokens intended for DockPipe to unrelated downstream
servers. Respect downstream audiences and keep upstream/downstream credentials isolated.
Federated callers do not inherit downstream permissions; imported servers are not assumed capable
of enforcing Terraform envelopes. This gates federated-import acceptance, not basic import work.

## Acceptance

1. Import an existing maintained server over stdio and another supported HTTP endpoint through the
   same DockPipe endpoint as native capabilities. Choose and verify upstream fixtures during work.
2. Namespaces, schemas and results survive forwarding; duplicates fail without registry mutation.
3. Distinct profiles filter listing and reject unauthorized direct calls. Import refresh, restart,
   failure, cancellation and removal do not expand permissions or leak credentials.
4. Demonstrate that neither an imported server nor a direct provider tool can reuse a Terraform
   grant to bypass scope, expiry or human approval. Unsupported enforcement fails closed.
5. Publish a compatibility/limitations matrix for tool and non-tool MCP behavior.
6. A federated caller receives only its configured capabilities. Tests reject cross-audience token
   forwarding, downstream credential exposure and permission inheritance through imported tools.

Direct Cloudflare/DigitalOcean packages are not required deliverables here. Prefer importing
maintained implementations when their functionality and enforcement boundaries meet the use case.

## Networking Coordination

[TASK-043](../configurable-networking/overview.md) owns generic ingress, egress, proxy
and effective enforcement contracts. Reuse its server/listener and workload policy
boundaries; reachability never grants capability or operation authority. Local stdio
functionality does not require the full server-networking initiative. Coordinate
strict isolation and remote exposure acceptance before claiming those deployments.

## Shared Capability Architecture Refinement

Follow TASK-039's [unified capability contract](../unified-mcp-gateway/unified-capability-contract.md):
shared domain implementations serve CLI, MCP, explicit REST and desktop, with existing package
storage and version lifecycle. Ordinary local execution stays lightweight; required privileged
controls cannot be bypassed by selecting a lighter interface or execution mode. Domain ownership
and this task's existing security/acceptance requirements remain intact. Backlog only.

## Read-Only Registry Foundation

The scoped registry foundation adds CLI search/version metadata and
validation of `mcp.servers` in the existing project configuration. See
[the public contract](../../../runtime/mcp-registry.md) for exact behavior, bounds,
verification and remaining gaps. Registry discovery does not install, connect, launch, authorize or
expose imported servers. TASK-040's gateway, process lifecycle, policy, desktop and
federated-import acceptance gates remain open; no external server is connected.


## Local artifact milestone

The generic `mcp bundle inspect/install` CLI now verifies and materializes a limited
MCPB 0.3 native binary subset under the existing project artifact cache. Tests cover
identity/architecture, traversal/alias rejection, unsupported launch contracts,
checksum mismatch, cancellation, staging cleanup and no overwrite. This does not
complete TASK-040: Registry URL selection, configured registration, execution grants,
shared upstream sessions, lifecycle controls and gateway forwarding remain pending.

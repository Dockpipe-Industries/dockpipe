# TASK-043 — Configurable Networking, Ingress and Egress

Repository ownership note: package paths in this record describe external source. Current catalog packages live in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages); previously extracted [DorkPipe/MCP](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages) and [Pipeon](https://github.com/Dockpipe-Industries/pipeon/tree/master/packages/pipeon) remain in their own repositories. Historical paths and receipts do not recreate a local package tree.

## Status and Objective

Backlog architecture/planning, added 2026-10-17. No implementation, network mutation,
service exposure, firewall change or provider qualification is authorized by this record.
The [full proposal](design-request.md) preserves user intent; sample commands and YAML
are conceptual. Include this task in the [direction audit](../agentic-app-ui/direction-review.md)
before building. Do not introduce a parallel networking stack.

Extend existing generic DockPipe configuration/runtime services for local, NAS, container,
VPS and private-VPC deployments. Core and CLI/API own policy, effective configuration,
lifecycle and diagnostics; the existing desktop consumes them. Package/workflow declarations
express requirements subject to administrator policy, not independent network authority.
Cloud is optional; networking does not become an AI-specific feature in DorkPipe.

## Bounded Reuse Inventory

Initial working-tree inspection, not enforcement or deployment qualification:

| Existing source | Reuse and limit |
| --- | --- |
| `src/lib/domain/runtime_policy_values.go` | Existing policy modes offline/allowlist/restricted/internet; do not confuse them with backend attachment modes auto/bridge/host/none |
| `src/lib/application/internal/runtimepolicy/security.go` | Compiled policy/enforcement summaries and override composition; restricted/allowlist can be advisory, with a separate proxy profile. Labels alone do not prove enforcement |
| `src/lib/infrastructure/docker.go` | `DockerNetworkModeFromEnv` and container `--network` construction; backend-specific mechanism, not a unified host policy |
| `src/lib/application/run_steps_container.go`, `run.go` | Existing container policy and network-mode application; preserve execution lifecycle |
| `src/lib/infrastructure/run_policy_registry.go` | Existing effective policy records; extend diagnostics rather than maintain a second status store |
| `packages/dorkpipe-mcp/mcpbridge/http_server.go` | Listen address, TLS, API-key/tier authentication, bounded HTTP and loopback-only insecure mode; coordinate compatible generic gateway extraction with TASK-039 |
| `docs/workflows/workflow-yaml.md`, `docs/security/security-policy.md` | Existing container-only authored security contract; host steps are outside Docker enforcement |

Complete proxy, remote worker, service binding, firewall and UI inventories before selecting
new fields or commands. Existing workflow/step override behavior does not establish an
administrator-controlled maximum policy; audit this boundary explicitly.

## Contract and Security Requirements

- Preserve ordinary host-route inheritance and current compatible defaults. Secure service
  bindings stay local unless explicitly configured; never expose an endpoint because a host
  is in a VPC/container. Public URL configuration does not open a port or grant access.
- Separate backend attachment, workload egress policy and service ingress. Define effective
  precedence across administrator, project, package/workflow and step settings; lower-trust
  declarations cannot widen the administrator ceiling. Existing overrides need migration review.
- Proxy URL/bypass support is configuration, not proof of confinement. Credentials remain
  references and are redacted. Strict destination policy requires a verified backend boundary;
  account for DNS/rebinding, redirects, IPv6, direct IPs and alternate routes. Unsupported
  strict policies fail before execution without unrestricted fallback.
- Configure bind/port, external URL, TLS termination and exact trusted-proxy boundaries through
  shared core operations. Reject conflicts and spoofed forwarding headers. Remote MCP still
  requires gateway authentication and invocation-time authorization.
- Native processes, imported stdio servers, containers, Terraform workers, model workers and
  remote targets need explicit capability/enforcement reports. Do not apply Docker semantics
  to every backend or claim host-wide firewall control from environment variables.
- Apply authorized updates safely, report restart requirements and active-workload impact,
  and retain rollback. No implicit host firewall, routing, NAT, DNS or VPC provisioning.
- Diagnostics are bounded, cancellable and sanitized. Connectivity probes must be explicit
  and use the intended workload path; observed public egress is not a static-IP guarantee.
  Protect test targets and probe interfaces against unauthorized/internal-network access.

## Core-First Priority

The user's latest 2026-10-17 direction prioritizes shared networking and package/version
foundations, then unified MCP, then DorkPipe graph/knowledge/context. Chat and model UI
are deferred. Audit first and deliver bounded backend-qualified slices; this does not
require implementing every future network backend before useful local MCP work.

## Related Owners and Delivery Order

- TASK-039 owns unified MCP identity/dispatch; this task owns generic listener/network contracts.
- TASK-040 owns imported-server lifecycle; reuse this task's per-process network guarantees.
- TASK-042 owns OIDC trust; network reachability, authenticated identity and permission grants
  remain separate. Apply networking constraints to issuer/JWKS fetching as appropriate.
- TASK-038 owns Terraform domain execution; consume network contracts without provider logic in core.
- TASK-023 owns native sandbox mechanisms and fail-closed enforcement; avoid a second namespace driver.
- TASK-036 owns remote onboarding; coordinate remote listener/worker configuration and diagnostics.
- DorkPipe consumes generic network contracts for optional providers and knowledge sources.

Start with inventory and normalized configuration/effective status, then explicit server MCP
binding/TLS/proxy integration, then workload restriction enforcement and existing-app settings.
Reorder around actual reusable code. Local stdio MCP and offline knowledge must not wait for a
complete server/firewall platform. Strict privileged remote execution does require verified
network controls appropriate to its deployment; do not defer those controls after release.

## Acceptance

1. Default local use works without new configuration; no public listener is created implicitly.
2. An isolated test server behind an authenticated HTTPS reverse proxy exposes only intended MCP
   routes; configured trusted proxies work and spoofed headers/port conflicts fail.
3. A container and supported native/imported process use their specified route/proxy path, with
   effective policy visible consistently through CLI/API and desktop consumers.
4. Strict restricted/offline policies resist tested DNS/redirect/direct-IP/IPv6/alternate-route
   bypasses on supported backends; unsupported guarantees reject before workload start.
5. Package/step overrides cannot exceed administrator policy. Proxy credentials never appear
   in status, logs, process diagnostics or artifacts; cancellation cleans up owned resources.
6. Test private-network NAT/proxy/no-internet scenarios with isolated fixtures and documented
   platform limits. Distinguish schema/unit validation from actual network enforcement proof.
7. Auth, source permissions and execution grants remain enforced independently of reachability;
   no Cloud dependency, infrastructure provisioning or unnecessary service is introduced.

Authored schema changes must update parser, docs and DorkPipe-owned editor support together;
no sample `network:` surface in the proposal is accepted or shipped merely by adding this task.

## Shared Capability Architecture Refinement

Follow TASK-039's [unified capability contract](../unified-mcp-gateway/unified-capability-contract.md):
shared domain implementations serve CLI, MCP, explicit REST and desktop, with existing package
storage and version lifecycle. Ordinary local execution stays lightweight; required privileged
controls cannot be bypassed by selecting a lighter interface or execution mode. Domain ownership
and this task's existing security/acceptance requirements remain intact. Backlog only.

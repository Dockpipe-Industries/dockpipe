# DockPipe Core — Configurable Networking, Ingress & Egress

## Objective

Extend DockPipe's existing networking configuration to support local, containerized, NAS, VPS, and cloud VPC deployments consistently.

DockPipe should work with the host's existing networking automatically, while allowing users to explicitly configure ingress, outbound egress, proxies, network restrictions, and per-workload overrides.

This must follow DockPipe's existing architecture:

**Core runtime → CLI/API → Desktop UI → Package and workflow configuration.**

The goal is to make DockPipe network-aware and configurable, not turn DockPipe into a VPC networking or infrastructure provisioning platform.

## 1. Inspect existing networking first

Before implementing anything, inspect:

- Current DockPipe networking configuration and runtime behavior.
- Docker and other supported execution backends.
- Container network creation, bridging, and NAT behavior.
- Existing proxy support.
- Host networking and port binding.
- Remote execution and worker configuration.
- Existing firewall or egress controls.
- Package manifests and workflow network settings.
- DockPipe CLI configuration.
- Desktop UI settings.
- Existing MCP HTTP endpoint configuration.

Identify reusable functionality and missing configuration surfaces.

Do not replace working network management code or introduce a parallel networking stack.

Preserve existing defaults and backwards compatibility wherever practical.

## 2. Unified networking configuration

Introduce or extend a consistent networking configuration model.

Illustrative configuration:

```yaml
network:
  mode: auto

  egress:
    mode: inherit
    proxy_url: null
    allowed_hosts: []
    denied_hosts: []

  ingress:
    bind: 127.0.0.1
    port: 8080
    public_url: null
    trusted_proxies: []

  isolation:
    default_policy: inherit
```

This is conceptual. Adapt it to existing DockPipe configuration conventions.

Supported modes should be determined by actual runtime capabilities.

### Network mode

Support:

- `auto` — Detect and use appropriate existing host networking.
- `bridge` — Use supported container bridge networking.
- `host` — Use host networking where explicitly permitted.
- `none` — Disable external networking where the backend supports enforcement.
- Additional backend-specific modes where appropriate.

Do not assume Docker networking modes apply identically to native execution, QEMU, Kubernetes, or remote hosts.

## 3. Outbound egress configuration

Support configurable outbound connectivity for DockPipe-managed workloads.

Initial capabilities:

- Inherit the host's existing outbound route.
- Use a configured HTTP/HTTPS proxy.
- Configure proxy bypass rules where appropriate.
- Define allowed or denied destinations where enforcement is supported.
- Support workload-specific egress settings.
- Expose effective networking configuration and diagnostic information.

Possible egress modes:

```yaml
egress:
  mode: inherit # inherit | proxy | restricted | none
```

### Critical distinction

DockPipe must not assume that it controls the public egress IP.

For example:

```text
DockPipe workload
      |
      v
Container / Host Network
      |
      v
VPC Private Address
      |
      v
Cloud NAT Gateway
      |
      v
Public Egress IP
```

The public egress address is normally determined by the host and upstream networking infrastructure.

DockPipe may expose observed public egress information for diagnostics, but must not mistake that observation for a guaranteed static address or assume all workloads share the same route.

DockPipe must not attempt to provision NAT gateways, cloud routes, or public IP assignments as part of this feature.

Those remain infrastructure responsibilities.

## 4. Enforceable network restrictions

Where DockPipe supports restricted egress, restrictions must be enforced at an appropriate network boundary.

Do not rely solely on application configuration or environment variables.

Evaluate mechanisms appropriate to each backend, such as:

- Container network isolation.
- Firewall or packet-filter policies.
- Network namespaces.
- Dedicated egress proxies.
- Backend-native network policies.

Host allowlists must account for DNS changes, redirects, alternate IP addresses, IPv6, and direct-IP connections.

An HTTP proxy alone does not guarantee network isolation if workloads can bypass it.

Requirements:

- Clearly distinguish enforced restrictions from advisory configuration.
- Reject unsupported strict isolation requirements rather than silently falling back to unrestricted networking.
- Prevent untrusted workloads from modifying their own network enforcement rules.
- Document enforcement limitations by backend.

## 5. Ingress and service exposure

Support explicit ingress configuration for DockPipe-managed services.

This includes the unified MCP gateway and other services that legitimately expose HTTP endpoints.

Configuration should support:

- Bind address.
- Listening port.
- External/public URL.
- Reverse-proxy awareness.
- Trusted proxy configuration.
- TLS termination expectations.
- Optional service-specific ingress overrides.

Example:

```yaml
network:
  ingress:
    bind: 127.0.0.1
    port: 8080
    public_url: https://dockpipe.example.com
    trusted_proxies:
      - 10.0.0.10
```

Do not automatically expose privileged services publicly merely because DockPipe is running inside a VPC or container.

Remote MCP access must require appropriate authentication and authorization.

Only trust forwarded client identity, scheme, and host headers from explicitly configured trusted proxies.

Avoid conflicts between package ports and DockPipe-managed endpoint registrations.

## 6. Per-package and per-workload networking

Packages and workflows should be able to declare their network requirements using the existing configuration system.

Examples:

```yaml
network:
  egress:
    mode: restricted
    allowed_hosts:
      - api.cloudflare.com
      - api.digitalocean.com
```

These declarations must be evaluated against an administrator-controlled maximum policy.

A package must not be able to weaken a stricter host or organization policy.

Support overrides where appropriate for:

- Workflow executions.
- Docker containers.
- Native processes.
- Imported MCP child processes.
- Terraform execution workers.
- Local model runtimes.
- Remote workers.

Do not expose host-level network administration capabilities to untrusted packages.

## 7. VPC and cloud compatibility

Ensure DockPipe behaves predictably when deployed on cloud instances with private networking.

Cover scenarios such as:

- Public instances with outbound internet connectivity.
- Private subnets using NAT gateways or NAT instances.
- Private subnets using outbound proxies.
- Restricted subnets without public internet access.
- Private API connectivity through supported network endpoints.
- Multi-interface hosts.
- IPv4 and IPv6 environments.

DockPipe must inherit upstream routes by default.

It should not require users to manually specify their public IP for ordinary operation.

Support configuring proxies and external service URLs when a VPC deployment requires them.

## 8. CLI-first management

All configuration must be manageable through existing DockPipe CLI conventions.

Illustrative commands:

```bash
dockpipe network status
dockpipe network inspect
dockpipe network test
dockpipe config set network.egress.mode proxy
dockpipe config set network.ingress.bind 0.0.0.0
```

Adapt these examples to existing command conventions.

Provide machine-readable output for automation.

Useful diagnostics should include:

- Active network mode.
- Effective egress policy.
- Configured proxy.
- DNS resolution.
- Route/interface information where available.
- Observed public egress IP, if explicitly requested.
- Connectivity tests to specified destinations.
- Effective ingress bindings.
- Unsupported or unenforced network restrictions.

Connectivity diagnostics must use the intended workload networking path when claiming to represent a workload's effective connectivity.

## 9. Desktop UI integration

Expose the same networking configuration through DockPipe's existing desktop application.

Potential views:

**Networking settings**
- Default network mode.
- Proxy configuration.
- Ingress bindings.
- Trusted proxies.
- Default egress policies.

**Runtime diagnostics**
- Effective network configuration.
- Connectivity checks.
- Workload-specific overrides.
- Network isolation status.
- Policy enforcement warnings.

The UI must use existing DockPipe APIs and configuration services.

Do not implement separate networking behavior in the frontend.

Keep the CLI and UI synchronized through the same source of truth.

## 10. Security and compatibility

Requirements:

- Preserve secure localhost binding defaults.
- Never enable public ingress implicitly.
- Do not expose proxy credentials in logs or diagnostics.
- Avoid accepting arbitrary trusted proxy headers.
- Validate hostnames, addresses, ports, and URLs.
- Support safe configuration updates.
- Avoid disrupting active workloads unexpectedly.
- Respect host firewall and networking rules.
- Handle unsupported backend capabilities explicitly.
- Document host-network mode risks.
- Require appropriate authorization for modifying privileged network settings.

The configuration should work without requiring DockPipe Cloud.

## 11. Testing

Add coverage for:

- Default local execution.
- Docker bridge networking.
- Host networking where supported.
- No-network execution.
- Proxy-based outbound connectivity.
- Restricted egress enforcement.
- Public versus private subnet routing behavior.
- DNS resolution.
- IPv6 where supported.
- Ingress binding and port conflicts.
- Reverse-proxy configurations.
- Imported MCP subprocess networking.
- Per-workload overrides.
- Policy inheritance and denial of privilege escalation.
- CLI/UI configuration consistency.

Tests must distinguish between configuration validation and actual network-policy enforcement.

Use isolated integration tests where appropriate.

## 12. Backlog and implementation approach

This is a DockPipe core initiative.

Before implementation:

1. Inspect the current network implementation.
2. Identify existing features and gaps.
3. Produce an architecture proposal.
4. Define the configuration contract.
5. Identify backend-specific capabilities and limitations.
6. Break implementation into focused backlog tasks.
7. Add security and integration acceptance criteria.
8. Avoid duplicating existing work.

Suggested phases:

**Phase 1 — Configuration and diagnostics**
- Unified network settings.
- CLI management.
- Effective configuration inspection.
- Host/VPC networking awareness.
- Proxy support where missing.

**Phase 2 — Ingress integration**
- Service binding configuration.
- MCP gateway integration.
- Reverse-proxy awareness.
- Port conflict handling.

**Phase 3 — Workload policies**
- Per-workload overrides.
- Egress restrictions.
- Backend-specific enforcement.
- Imported MCP process isolation.

**Phase 4 — Desktop UI**
- Settings integration.
- Networking diagnostics.
- Policy visibility.

Reorder phases where existing implementation makes another sequence more practical.

## Acceptance criteria

A local DockPipe installation works without explicit network configuration.

A DockPipe instance deployed into a private VPC subnet successfully uses the host's existing NAT or outbound proxy configuration.

A DockPipe-managed container can reach authorized external services through the intended egress path.

A workload configured with strict egress restrictions cannot bypass those restrictions through an alternate route.

Unsupported isolation configurations fail explicitly.

The unified MCP gateway can be exposed through an authenticated HTTPS reverse proxy without exposing unrelated services.

Users can inspect and configure network settings through the DockPipe CLI.

The desktop UI reflects the same configuration.

Packages and workflows inherit secure defaults and cannot silently expand their network privileges.

No additional NAT provisioning, VPC management platform, or unnecessary background service is introduced.

**Final goal: DockPipe works naturally on local machines, NAS devices, servers, and cloud VPCs while providing predictable, configurable, and enforceable networking wherever its execution backends support it.**

This is a backlog architecture and implementation-planning task. Inspect existing functionality and update the backlog before making large implementation changes.
# TASK-042 — Provider-Agnostic OIDC Workload Federation

Repository ownership note: package paths in this record describe external source. Current catalog packages live in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages); previously extracted [DorkPipe/MCP](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages) and [Pipeon](https://github.com/Dockpipe-Industries/pipeon/tree/master/packages/pipeon) remain in their own repositories. Historical paths and receipts do not recreate a local package tree.

## CLI-First Implementation Requirement

The [shared CLI-first management contract](../unified-mcp-gateway/cli-first-management.md) is mandatory for this task.
Core runtime and CLI/API own behavior, configuration and lifecycle; the existing desktop UI and MCP
exposure consume those capabilities. All essential operations must work headlessly. Reuse existing
package commands, backend services and UI patterns; no separate MCP configuration store or UI
business logic. Include the shared CLI/UI parity and process-lifecycle acceptance gates in delivery.

## Status And Ownership

**P1**, backlog and architecture refinement only; added 2026-10-17. No federation implementation,
provider qualification or protocol compatibility is claimed. The [full request](design-request.md)
is preserved verbatim. This is a shared authentication workstream within
[TASK-039: unified MCP gateway](../unified-mcp-gateway/overview.md), not a separate identity platform.

DockPipe verifies external workload identities. External providers issue identity tokens; DockPipe
maps verified claims and enforces capability policy; humans retain control of privileged operations.
Do not build an OIDC issuer or general-purpose OAuth authorization server.

## Initial Authentication Assessment

Bounded source inspection found these reuse points and gaps:

| Evidence | Current behavior | Required work |
| --- | --- | --- |
| `packages/dorkpipe-mcp/mcpbridge/auth.go` | Extracts Bearer/ApiKey/X-API-Key credentials and compares API keys | Add explicit authentication-method selection and verified identity output without treating arbitrary bearer strings as validated OIDC or silently falling back after failed validation |
| `mcpbridge/http_key_tiers.go`, `tier_context.go` | Per-key readonly/validate/exec tiers and request-context tier | Integrate issuer-scoped identities and workload policy while retaining existing key-based clients |
| `mcpbridge/http_server.go` | HTTPS configuration, authenticated HTTP entrypoint and bounded requests | Integrate federation, abuse protection, quotas and tenant/environment boundaries |
| `mcpbridge/server.go`, `catalog.go` | Tool filtering and invocation checks | Consume TASK-039 profiles using verified identities; do not use discovery visibility as the security boundary |

The inspected MCP bridge, engine Go sources and root module dependencies did not identify an
existing OIDC/JWKS validator. This is not a repository-wide absence proof. Complete the reuse and
library review before implementation; use established validation libraries, not custom cryptography.
Existing provider-pool approvals are not workload authentication or infrastructure permission grants.

## Three Independent Security Layers

| Layer | Decision | Owner |
| --- | --- | --- |
| A: workload authentication | Is this token from an explicitly trusted issuer and intended for DockPipe? | TASK-042 verifier and identity adapters |
| B: capability authorization | Which packages/tools may this identity discover and invoke? | TASK-039 profiles and server-side invocation policy |
| C: privileged operation authorization | Is this specific plan/operation within a valid approved envelope? | Shared authorization contracts plus TASK-038 domain evaluation/executor |

Authentication lifetime and infrastructure grant lifetime are independent. A new external token,
new session or successful capability check never renews a configurable 15-minute human approval.
Do not treat authentication tokens as approval grants or infer production authority from valid OIDC.

## Trust Registry And Identity Model

- Administrators explicitly configure multiple issuers, exact issuer URLs, expected audiences,
  discovery/JWKS endpoints, allowed algorithms, required claims, matching/mapping rules and optional
  provider-specific checks. Valid discovery metadata never creates trust automatically.
- Verify signature, issuer, audience, expiry and not-before. Require an audience intended for DockPipe;
  reject tokens intended for AWS, Azure or other unrelated resources. Define bounded clock skew,
  key caching/refresh and failure behavior rather than relying on implicit defaults.
- Support key rotation and enabling/disabling/revoking trust configurations. Persist portable,
  recoverable trust policy across restarts; authorize and audit administrative policy changes.
- Protect discovery and key retrieval from SSRF, redirect/endpoint substitution and unbounded fetches.
  Define explicit network policy for private/self-hosted issuers without accepting arbitrary targets.
- Normalize verified issuer and subject into issuer-scoped workload identity keys, with configurable
  attributes and audit provenance. Prefer stable repository/project IDs over mutable display names.
  Do not assume similarly named claims have equivalent trust semantics across providers.
- Put provider-specific claim semantics at the identity-adapter boundary. Never authorize using
  caller-supplied unverified metadata or put GitHub-specific authentication in the Terraform executor.

## Gateway And Protocol Compatibility

Prefer direct validation of short-lived external workload tokens for supported clients. Do not
mandate a second DockPipe-issued token or a Cloud dependency. Keep existing API keys available for
clients that cannot use federation; prefer OIDC for pipelines with suitable workload identities.

Before implementation, assess the current MCP HTTP authorization specification and candidate
OIDC/OAuth libraries using primary documentation. Record supported client flows, token types,
audiences, discovery/challenge behavior and limitations. Direct external identity-token validation
must be distinguished from standard OAuth MCP resource-server flows; neither is claimed compatible
by this backlog entry. If specific clients require access tokens or exchange, evaluate an optional
adapter or trusted external authorization service and document trade-offs. Protocol correctness
must not be weakened to avoid another token. A general-purpose authorization server is out of scope.

Remote access requires HTTPS, per-workload policy, rate limiting, abuse protection, request bounds,
execution quotas, sanitized audit, short-lived credentials and applicable tenant/environment
isolation. Recheck invocation policy regardless of listing. Do not log tokens or return credentials.

## Provider Validation And Delivery Breakdown

1. **Assessment:** complete authentication/library inventory, trust/identity design, MCP direct-token
   compatibility matrix, threat model, gateway/Terraform interface changes and acceptance plan.
2. **Trust foundation:** explicit issuer registry, safe discovery/JWKS rotation, verified claims,
   revocation and persistence. Exercise multiple issuers and cross-issuer collision rejection.
3. **Identity and gateway:** normalize identities, connect profiles and invocation policy, and prove
   one secure end-to-end workload flow without replacing existing authentication.
4. **Initial integrations:** validate GitHub Actions and GitLab CI/CD. Evaluate Microsoft Entra ID /
   Azure workload identities and Kubernetes projected service-account tokens; admit additional
   standards-compliant issuers through configuration/adapters after verifying their claim semantics.
5. **Hardening and recovery:** run the negative/security matrix below and document local recovery.
   Optional protocol exchange adapters are conditional on compatibility findings, not mandatory.

These are deliverable areas within this task, not duplicate gateway, identity-platform or Terraform
backlogs. No provider is declared supported solely because its discovery endpoint is reachable.

## Dependencies And Integration

- TASK-039 owns gateway authentication hooks, normalized request context, profiles and audit contracts;
  this task supplies the shared federation implementation. Design may proceed together; federation
  depends on those interfaces, while the basic gateway can retain API-key authentication.
- [TASK-038](https://github.com/Dockpipe-Industries/dockpipe-packages/blob/master/docs/agents/tasks/standalone-terraform-mcp-executor/overview.md) consumes verified identities through
  the shared boundary. Its federated pipeline acceptance depends on TASK-042; its execution core and
  existing-authentication path do not. Preserve plan checks, human approvals, vault ownership,
  DigitalOcean/Cloudflare scope, polling/webhooks and durable job state.
- [TASK-040](../third-party-mcp-import/overview.md) must authorize before forwarding, isolate upstream
  and downstream credentials and respect token audiences. Never forward DockPipe workload tokens to
  unrelated imports. Per-import security policy must prevent downstream authority inheritance.
- [TASK-041](../cloud-mcp-approvals/overview.md) stays optional. Federation neither requires Cloud nor
  grants a workload the authenticated human approval role.

## Security Matrix And Acceptance Tests To Implement

| Case | Required result or assessment |
| --- | --- |
| Invalid signature, unknown/substituted issuer, wrong audience, unsupported algorithm, expired/not-yet-valid token, spoofed claims | Reject authentication before invocation; no permissive fallback |
| JWKS rotation, unavailable discovery, stale caches, trust disablement/revocation and restart | Explicit bounded cache/refresh policy, fail-closed invalid trust, recoverable authorized configuration |
| Duplicate subjects across issuers; cross-tenant or cross-environment requests | Distinct issuer-scoped identities and policy isolation |
| Mutable repository names, forked pull requests, untrusted branches/environments | Enforce configured verified-claim policy; evaluate immutable IDs, approved workflow references and protected environments where supported |
| Unauthorized trust edits, SSRF, discovery/JWKS substitution | Reject unapproved mutation and retrieval targets; preserve audit evidence |
| Token replay and compromised CI runners | Evaluate replay defenses and residual bearer-token risk; valid identity alone cannot establish a safe workload or infrastructure authority |
| Hidden/guessed tools, imported credentials, native/import privilege escalation | Recheck invocation policy, isolate credentials and deny unsupported downstream guarantees |
| Fresh identity token after human approval expiry; identity token supplied as approval | Authenticate the permitted workload but deny privileged execution until separately authorized |

A self-hosted instance must accept multiple configured issuers. A GitHub Actions or GitLab workflow
must authenticate with a short-lived external token without a permanent DockPipe API key, submit
permitted Terraform work, and poll or receive a webhook. Production operations still require the
independent human envelope. Test in-scope correction, out-of-scope denial and approval expiry with
reauthentication. No Cloud subscription or per-provider Terraform authentication code is required.

## Bootstrap And Recovery

External providers issue workload identity independently of DockPipe's infrastructure lifecycle.
Portable trust configuration survives service restoration. This solves identity bootstrap, not
executor availability: a failed remote executor cannot be its own mandatory recovery dependency.

Document and validate a controlled local Terraform recovery path using human-controlled external
vault credentials, the configured Terraform state backend and state-lock coordination. Require
explicit recovery authority, audit the operation and reconcile partial remote execution before any
retry. This is a future recovery procedure to design and test, not authorization to run Terraform
or retrieve credentials during this backlog update.

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

# P1 — Provider-Agnostic OIDC Federation for DockPipe Unified MCP

## Objective

Extend the existing DockPipe unified MCP architecture to support **provider-agnostic OIDC workload identity federation**.

The primary goal is to allow CI/CD pipelines, automated workloads, and trusted external applications to authenticate securely to DockPipe's unified MCP gateway without requiring long-lived API keys.

Authentication must remain independent of DockPipe's own infrastructure lifecycle.

**Core principle: External identity providers establish identity. DockPipe verifies identity and enforces authorization. Humans control privileged infrastructure access.**

This work should integrate into the existing unified MCP initiative and Task 38 (Terraform MCP Executor), not become a separate identity platform.

## 1. Architectural requirements

DockPipe must operate as a verifier of external workload identities, not as a general-purpose OIDC identity provider.

The architecture must:

- Accept workload identity tokens issued by explicitly trusted external providers.
- Validate those tokens against configured trust relationships.
- Map verified claims to DockPipe workload identities.
- Authorize MCP capabilities based on those identities.
- Preserve independent authorization for privileged operations.
- Support direct token validation without mandatory token exchange.
- Avoid introducing dependencies on DockPipe Cloud.
- Work in local and self-hosted deployments.
- Preserve existing authentication mechanisms for clients that cannot use OIDC.

Do not implement an OAuth/OIDC authorization server unless a concrete protocol requirement makes it necessary.

If standard MCP authorization compatibility requires an access-token exchange or broker, evaluate that separately and document the trade-offs rather than assuming DockPipe must always issue its own tokens.

## 2. Inspect existing architecture first

Before making changes:

1. Review the current DockPipe MCP implementation.
2. Inspect existing authentication, API-key, identity and authorization components.
3. Review the unified MCP package architecture and Task 38.
4. Identify reusable token-validation and trust-policy functionality.
5. Determine how OIDC authentication can integrate without replacing existing mechanisms.
6. Identify existing third-party authentication libraries or standards-compliant implementations that can be reused.

Prefer existing infrastructure and proven libraries over custom cryptographic implementations.

Do not create a parallel identity subsystem.

## 3. Provider-agnostic OIDC trust configuration

Introduce a configurable trust registry supporting multiple external identity providers.

Initial providers to evaluate:

- GitHub Actions.
- GitLab CI/CD.
- Microsoft Entra ID / Azure workload identities.
- Kubernetes projected service-account tokens.
- Other standards-compliant OIDC issuers.

Each trust configuration should support:

- Trusted issuer URL.
- Expected audience values.
- Discovery metadata and JWKS endpoints.
- Allowed signing algorithms.
- Signature verification.
- Token expiry and not-before validation.
- Required identity claims.
- Configurable claim matching and mapping.
- Optional provider-specific validation rules.
- Issuer key rotation.
- Enable, disable and revoke trust configurations.

Trust relationships must be explicitly configured by an administrator.

Never automatically trust an arbitrary issuer simply because its discovery endpoint is valid.

Issuer discovery and key retrieval must protect against SSRF and untrusted endpoint substitution.

## 4. Direct workload identity authentication

The preferred workflow is:

```text
External OIDC Provider
        |
        | Issues short-lived identity token
        v
   CI/CD Workload
        |
        | Authenticates to DockPipe
        v
  DockPipe MCP Gateway
        |
        | Validates issuer, audience,
        | signature, expiry and claims
        v
  Workload Identity Mapping
        |
        v
  Capability Authorization
        |
        v
  MCP Tool Execution
```

For supported workloads, DockPipe should be able to authenticate the externally issued token directly.

Do not require the workload to first obtain a separate DockPipe identity token.

Ensure the token is explicitly intended for DockPipe through correct audience validation.

An identity token issued for AWS, Azure, or another unrelated resource must not be accepted as a DockPipe credential.

Investigate compatibility with the current MCP HTTP authorization specification.

Where direct external identity-token authentication is implemented, clearly distinguish it from standard OAuth MCP resource-server flows. If an OAuth access token or standards-based exchange is necessary for particular clients, support that through an optional adapter or trusted external authorization service.

Do not compromise protocol correctness solely to avoid an additional token.

## 5. Normalized workload identities

Create a generic internal representation of authenticated workload identities.

Example:

```yaml
identity:
  type: workload
  issuer: https://issuer.example.com
  subject: external-workload-identity

  attributes:
    repository_id: "12345"
    environment: production
    ref: refs/heads/main
    pipeline_id: "98765"
```

This structure is illustrative.

Not all providers expose the same claims.

Requirements:

- Preserve the verified issuer and subject.
- Use issuer-scoped identity keys.
- Support configurable provider-specific claim mappings.
- Prefer stable identifiers over mutable display names.
- Avoid assuming equivalent claim names mean equivalent trust semantics.
- Prevent identity collisions across issuers.
- Record sufficient identity information for auditing.
- Never authorize based on unverified caller-supplied metadata.

Keep provider-specific behavior at the identity-adapter boundary rather than introducing GitHub-specific logic throughout the authorization system.

## 6. Separate authentication and authorization

Maintain three distinct security layers.

### Layer A — Workload authentication

Determine whether the caller is a trusted workload.

Validate its external OIDC identity.

### Layer B — MCP capability authorization

Determine which packages and tools the workload may discover and invoke.

Examples:

- Terraform planning.
- Terraform execution requests.
- Artifact publishing.
- Cloud provider inspection.
- Execution status retrieval.

Tool visibility must never be the sole security control.

Invocation authorization must be enforced server-side.

### Layer C — Privileged operation authorization

Determine whether a specific infrastructure operation is permitted.

This layer includes:

- Terraform plan evaluation.
- Permission envelopes.
- Human approval.
- Expiration and revocation.
- Environment restrictions.
- Provider-specific policy enforcement.

**A correctly authenticated workload must not automatically receive production infrastructure privileges.**

OIDC authentication must not bypass the human approval model established in Task 38.

## 7. Terraform MCP integration — Task 38

Integrate OIDC federation with the existing Terraform MCP backlog requirements.

Expected workflow:

1. A GitHub Actions, GitLab CI/CD, or other supported pipeline obtains an OIDC token from its provider.
2. The pipeline authenticates to DockPipe's unified MCP endpoint.
3. DockPipe validates the external identity and applies the configured workload policy.
4. The pipeline submits Terraform work.
5. The Terraform package produces and evaluates a plan.
6. Existing authorization policy determines whether the requested changes require human approval.
7. Approved changes execute within the authorized permission envelope.
8. The pipeline polls or receives a webhook for the execution result.

Preserve the existing concept of configurable 15-minute privileged authorization windows.

Workload authentication lifetime and infrastructure authorization lifetime are independent.

A pipeline may obtain another valid external identity token without receiving additional infrastructure authority.

A new identity token must never extend an expired human approval.

## 8. Prevent circular infrastructure dependencies

DockPipe infrastructure may itself be deployed by pipelines using DockPipe's Terraform MCP.

The identity architecture must not create unnecessary circular dependencies.

Specifically:

- GitHub, GitLab or another external identity provider issues the workload identity independently of DockPipe.
- DockPipe must not be responsible for minting the upstream pipeline identity.
- DockPipe Cloud must not be required for local federation.
- Authentication configuration must be portable and recoverable.
- Trusted issuer configurations must survive service restarts and infrastructure recovery.

External identity federation solves the identity bootstrap problem, but not the separate executor-availability problem.

Document a controlled recovery mechanism for situations where the remote DockPipe Terraform executor is unavailable.

A possible recovery path uses locally executed Terraform and existing human-controlled credentials from an external vault.

Do not create a mandatory dependency on a remote MCP endpoint to recover that same endpoint.

## 9. Public MCP authentication

Support authenticated access to remotely exposed DockPipe MCP endpoints.

Requirements:

- HTTPS for remote access.
- Explicit issuer and audience validation.
- Per-workload authorization policies.
- Rate limiting and abuse protection.
- Request size and execution quotas.
- Audit logging.
- Short-lived credentials.
- No credentials exposed through logs or tool results.
- Secure handling of third-party imported MCP server authentication.
- Tenant and environment isolation where applicable.

Existing static API keys may remain available as a fallback.

OIDC should be preferred for CI/CD platforms that support suitable workload identity federation.

A public MCP endpoint must never imply public access to privileged tools.

## 10. Third-party MCP import security

The unified MCP gateway may import third-party MCP servers.

Federated identities must not automatically inherit the permissions of those downstream services.

DockPipe must:

- Authorize calls before forwarding them.
- Keep upstream and downstream credentials isolated.
- Avoid forwarding external OIDC tokens to unrelated MCP servers.
- Respect downstream token audiences and identity boundaries.
- Prevent imported tools from bypassing native authorization policies.
- Keep tool namespaces and caller identity mappings stable.
- Support explicit per-import security configuration.

Do not assume that an imported MCP server can enforce DockPipe's Terraform permission-envelope model.

## 11. Security requirements

Evaluate and test:

- Invalid signatures.
- Unknown issuers.
- Incorrect audiences.
- Expired tokens.
- Tokens not yet valid.
- Unsupported signing algorithms.
- JWKS rotation.
- Token replay.
- Issuer substitution.
- Claim spoofing.
- Mutable repository or project names.
- Forked pull-request workflows.
- Untrusted branches and environments.
- Cross-tenant authorization.
- Unauthorized trust-policy changes.
- Compromised CI runners.
- MCP tool privilege escalation.
- Credentials exposed to imported MCP tools.
- SSRF through OIDC discovery.
- Reuse of authentication tokens as approval grants.

For high-risk policies, evaluate restrictions based on immutable repository identifiers, approved workflow references, protected environments and other verified claims where supported.

Do not assume that a valid OIDC token means the originating workflow is safe or trustworthy.

Use established OIDC and OAuth validation libraries.

## 12. Backlog integration

Review the existing unified MCP initiative and Task 38.

Integrate federation as a shared authentication capability.

Suggested work areas:

**Foundation — OIDC trust registry**
- Trusted issuer configuration.
- Token validation.
- Signing key discovery and rotation.
- Claim validation.

**Identity — Workload normalization**
- Provider-agnostic identity representation.
- Configurable claim mappings.
- Stable workload identifiers.
- Audit integration.

**Gateway — MCP authentication**
- Direct external token validation where supported.
- Authorization-aware tool discovery.
- Invocation policy enforcement.
- Optional standards-compliant access-token exchange when needed.

**Integrations — Initial provider validation**
- GitHub Actions.
- GitLab CI/CD.
- Additional providers through configuration.

**Security — Federation hardening**
- Negative authentication tests.
- Cross-issuer isolation.
- Audience restrictions.
- Privilege-escalation testing.
- Bootstrap and recovery documentation.

Update Task 38 to consume the shared federation capability.

Do not introduce GitHub-specific authentication into the Terraform executor.

Avoid duplicating existing backlog items.

## 13. Implementation sequence

Before implementation, produce:

1. Existing authentication architecture assessment.
2. Proposed OIDC trust and identity model.
3. Direct-token compatibility assessment against the MCP authorization specification.
4. Security and threat model.
5. Required unified MCP gateway changes.
6. Required Task 38 changes.
7. Backlog breakdown with dependencies.
8. Acceptance tests.

Prioritize the smallest implementation that demonstrates a complete secure authentication flow.

Do not build a general-purpose identity provider or authorization server as part of this initiative.

## Acceptance criteria

A self-hosted DockPipe instance can configure multiple trusted OIDC issuers.

A GitHub Actions or GitLab CI/CD workflow can authenticate using a short-lived externally issued workload identity without storing a permanent DockPipe API key.

DockPipe independently validates the identity and applies capability authorization.

A valid pipeline identity can submit permitted Terraform work.

Production changes still require an independent human-approved permission envelope.

Reauthenticating with a new OIDC identity token does not renew expired production authorization.

Unauthorized workflows, forks, branches and environments are denied according to configured policies.

Imported third-party MCP tools cannot bypass authorization.

The architecture supports additional OIDC issuers without rewriting its authentication core.

DockPipe Cloud is not required.

A documented local recovery path remains available if the remote DockPipe MCP service is unavailable.

**Final goal: Provider-agnostic workload authentication, one unified DockPipe MCP gateway, no mandatory static pipeline credentials, and human-controlled infrastructure authorization—without turning DockPipe into an identity provider.**

This is a backlog and architecture refinement task. Do not implement the full feature unless separately instructed.
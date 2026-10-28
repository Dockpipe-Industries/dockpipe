# TASK-041 — Optional DockPipe Cloud MCP Approvals

## CLI-First Implementation Requirement

The [shared CLI-first management contract](../unified-mcp-gateway/cli-first-management.md) is mandatory for this task.
Core runtime and CLI/API own behavior, configuration and lifecycle; the existing desktop UI and MCP
exposure consume those capabilities. All essential operations must work headlessly. Reuse existing
package commands, backend services and UI patterns; no separate MCP configuration store or UI
business logic. Include the shared CLI/UI parity and process-lifecycle acceptance gates in delivery.

## Status And Dependencies

Future enhancement; backlog, added 2026-10-17. Depends on
[TASK-039](../unified-mcp-gateway/overview.md) approval, identity and audit contracts, with
[TASK-038](https://github.com/Dockpipe-Industries/dockpipe-packages/blob/master/docs/agents/tasks/standalone-terraform-mcp-executor/overview.md) as the first infrastructure scenario.
Implements the optional Cloud scope in the [design request](../unified-mcp-gateway/design-request.md).
Local/self-hosted authorization and Terraform execution do not depend on this task.

## Workload Identity Boundary

[TASK-042](../oidc-workload-federation/overview.md) works locally without Cloud. Its workload
identity tokens are not human approval grants; reauthentication never extends a permission envelope.
Retain distinct approval identities and independently verified decisions. Cloud is not the issuer
of upstream CI identities or a mandatory dependency for federation or executor recovery.

## Scope

Provide optional remote human approval/denial, mobile push notifications, approval history,
team approvals, centralized policy management, execution notifications, and grant expiration and
revocation notifications. Coordinate Cloud-side ownership separately; this backlog entry does not
claim implementation in the Cloud repository or authorize deployment.

- Bind signed decisions to authenticated approval/caller identities, exact requested authority,
  package/domain, policy version, expiration and replay-resistant request identity.
- The executor independently verifies each decision and checks current expiry/revocation at use.
  Agents/pipelines cannot renew human-gated grants or bypass renewal by creating new sessions.
- Transmit minimal approval metadata and bound scope digests; Cloud must not require custody of
  infrastructure credentials, Terraform state or full Terraform plan contents.
- Preserve local approvals without a subscription and distinguish notification delivery from
  execution completion. Define fail-closed behavior when remote approval cannot be verified.
- No new vault, billing system, hosting platform or Biztraak coupling is part of this task.

## Acceptance

- Authenticated humans can approve, deny and revoke an explicit envelope remotely; the executor
  rejects tampered, mismatched, replayed, expired or revoked decisions.
- Cloud failure cannot silently grant or renew permissions. Local policy remains usable without
  the integration, and configured human gates remain enforced.
- Audit links approval identity and decision to execution; notification retries do not repeat applies.
- Payload review proves infrastructure credentials, Terraform state and full plans are not required.

## P1 security and reliability integration scope (2026-10-26)

Associate this existing approval/notification work with the
[dependency lifecycle initiative](../dependency-lifecycle/overview.md). TASK-049/050
consume TASK-039's exact update/restart envelopes; TASK-036/039 own mandatory
hardware-backed approval policy. Cloud delivery cannot weaken those gates or
become required for local/manual remediation. TASK-051 owns advisory findings and
notification deduplication, reusing delivery adapters where available. TASK-052
must verify expiry, revocation, notification failure and executor-side rejection;
retrying delivery must not replay an update. P1 applies to this integration scope;
Cloud product rollout remains optional and separately owned.

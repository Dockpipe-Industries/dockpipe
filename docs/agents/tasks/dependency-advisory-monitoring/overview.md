# TASK-051 Dependency security advisory monitoring and notifications

## Status, priority and dependencies

**P1; proposed, decision ready, unclaimed.** Added 2026-10-26 under
[TASK-049's security and reliability initiative](../dependency-lifecycle/overview.md).
Bounded backlog/source inspection found no existing generic dependency advisory
monitor; TASK-041 owns approval notifications, not dependency vulnerability feeds.
Extend existing event/notification contracts; do not duplicate approval delivery.

Depends on TASK-049 observation/assessment contracts; coordinate TASK-050's first
provider mapping, TASK-039 read/execute authorization and optional TASK-041 Cloud
notifications. TASK-052 validates delivery/failure behavior. Monitoring can ship
before update mutation, hardware approval integration or exact-lock activation.

## Current architecture findings and problem statement

Host preflight in `src/lib/application/internal/hostdependencies/service.go`
records presence, not advisory coverage. `package_inventory.go` records installed
Dockpipe package versions, not active host-service provenance.
`src/lib/infrastructure/operationrecord/operation_event.go` already emits
structured events and an index. The external remote package assigns scheduling
to DorkPipe. TASK-039 is the shared CLI/MCP capability owner; TASK-041 proposes
optional notification delivery and explicitly separates delivery from execution.

Operators need timely evidence of unsafe dependencies, including advisories
without CVEs, without treating unreachable feeds or unknown versions as safe.
A provider-specific feed poller inside Core would violate the resolver boundary.

## Proposed design and security considerations

- Define a generic normalized advisory observation: source URL/identity, advisory
  ID/aliases, publication/update/fetch times, metadata digest/signature evidence,
  affected/fixed ranges, artifact/platform/install-method constraints, severity,
  confidence, reason and remediation reference. CVE presence is optional.
- Resolver packages supply authoritative sources, dependency identities and
  provider-specific applicability/version semantics. Core joins those records to
  target observations and records affected, unaffected, unknown or stale outcomes.
  Keep outdated, vulnerable and installed-but-not-active separate dimensions.
- Preserve bounded cached evidence and freshness/expiry. Handle corrections,
  withdrawals, backports and conflicting sources without silent last-writer-wins.
  Feed outage, parse failure, unsupported version and absent coverage remain visible;
  stale data cannot authorize unattended updates or close a P0 exposure.
- Schedule existing check workflows through DorkPipe/operator trigger integration,
  with overlap control, bounded retries, jitter, offline behavior and rate limits.
  Do not add a Core feed daemon or new scheduler.
- Deduplicate by advisory/target/artifact identity; notify on new exposure,
  severity/applicability change, pinned vulnerable dependency and remediation or
  failure. Support acknowledgements, expiry-bound suppressions with owner/reason,
  and escalation policy. Acknowledgement does not waive update authorization.
- Store findings in existing status/audit paths; expose authenticated CLI/Qt/MCP
  views through TASK-049/039. Local notifications must work without Cloud;
  external channels are explicitly configured, redact machine secrets and use
  existing delivery adapters. Retry notification delivery without replaying updates.
- Restrict source origins/redirects and network scope; reject SSRF targets,
  oversized/compressed payload abuse, forged/stale metadata and remote executable
  content. Vendor messages and URLs are untrusted data, never shell instructions.
  Honor signing-key rotation/revocation and trust policy from TASK-049.

## Components and implementation phases

Candidate Core areas: domain dependency findings, TASK-049 assessment application
service, operationrecord/history readers, CLI and Qt status clients, TASK-039
native capability adapter. Provider feed mappings and fixtures belong in their
own packages; canonical docs/schema follow any new authored fields.

1. Stabilize normalized evidence/freshness schema with TASK-049 and threat model.
2. Add deterministic fixture ingestion and manual assessment; TASK-050 supplies
   the October 20 Cloudflare advisory without hardcoded Core vendor logic.
3. Add scheduled check integration, deduplicated local notifications and optional
   existing remote delivery. Qualify offline/recovery behavior in TASK-052.

## Acceptance criteria

- [ ] cloudflared fixtures identify affected active artifacts, fixed candidates,
  stale evidence and unknown/macOS-channel applicability without needing a CVE.
- [ ] Non-Cloudflare fixtures prove generic ingestion and provider ownership.
- [ ] CLI/UI/MCP show evidence/source age, coverage gaps, pinned conflicts and
  notification state; they never infer safety from an empty or unavailable feed.
- [ ] Findings link to exact update plans and audit history; notification,
  acknowledgement and suppression grant no execution or restart authority.
- [ ] Existing scheduling, notification and capability infrastructure is reused;
  no duplicate approval workflow or Cloud requirement is introduced.

## Tests required

Range/version and backport fixtures, alias deduplication, source corrections and
withdrawals, missing CVE, forged signature, oversized payload, redirect/SSRF,
clock/freshness boundaries, offline cache and rate limiting. Verify repeated
checks notify once, changed exposure reopens, suppression expires, delivery
failure retries only notifications, and secret-bearing provider data is redacted.

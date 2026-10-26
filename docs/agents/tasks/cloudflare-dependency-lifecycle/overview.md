# TASK-050 Cloudflare Tunnel resolver lifecycle implementation

## Status, priority and dependencies

**P1 implementation; P0 immediate affected-installation remediation lane.** Proposed
2026-10-26, decision ready, unclaimed. Cross-repository coordination record within
[TASK-049's security and reliability initiative](../dependency-lifecycle/overview.md).
Implementation belongs to the external
[dockpipe-packages remote package](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/remote),
not Core. This local record tracks integration without claiming external changes.
No dependencies were inventoried or upgraded during planning.

Depends on TASK-049 capability/plan contracts, TASK-010 trusted installer/pin rules,
TASK-036/039 target authorization and TASK-051 advisory records. TASK-052 blocks
unattended rollout. Exposure triage and approved manual remediation must not wait
for the general framework or optional Cloud approval integration.

## Advisory evidence and immediate exposure lane

[Cloudflare's October 20 advisory](https://blog.cloudflare.com/emudhra-certificate-compromise/)
requires cloudflared 2026.10.2 and fixed container redeployment after compromised
intermediate CAs. It reports no identified active exploitation against Cloudflare
services. [2026.10.3 release](https://github.com/cloudflare/cloudflared/releases/tag/2026.10.3)
and [release notes](https://github.com/cloudflare/cloudflared/blob/master/RELEASE_NOTES)
confirm a newer release. Sources reviewed 2026-10-26.

The request additionally requires 2026.10.3 for certain macOS installations. The
inspected primary advisory does not specify that exception. Preserve 2026.10.3 as
the conservative macOS planning target; obtain authoritative artifact/channel
applicability evidence before declaring a macOS 2026.10.2 installation safe. Do
not fabricate a CVE or infer protection from the displayed version alone.

P0 lane: identify affected installed **and running** clients, exact image/binary
provenance and TLS verification path; record exposure, owner and remediation
window; prepare a verified fixed release and explicit approval/maintenance/recovery
plan; activate it and prove the running connector and application TLS path are
fixed. Unknown installations require urgent triage, not a false clean result.
Neither a patched browser nor a downloaded binary proves the active tunnel safe.
No restart or trust-store edits are authorized by this backlog record.

## Current architecture findings and problem

External source inspected at `de960322070e477a058516dbe156ed9bf828392a`:

| External path (under `packages/remote/`) | Finding |
| --- | --- |
| `package.yml`, `resolvers/dockpipe.cloudflare.remote-edge/package.yml` | Version 0.3.1; `remote.edge` resolver declares cloudflared presence and Homebrew/signed APT install hints. No Windows installer or version/advisory contract is declared. |
| Resolver `profile`, `assets/scripts/setup.sh` | Existing provider-owned setup hook; extend this package rather than add a Core Cloudflare special case. |
| `tools/cloudflare/setup.go` | Private setup journal/lock, reusable login/tunnel credentials and DNS routing; emits an edge config running `tunnel --no-autoupdate run`. Automatic lifecycle is therefore not supplied by this managed command. |
| `tools/cloudflare/credential_{unix,windows}.go`, `credential_test.go` | Private credential validation; preserve supported 0400/0600 Unix modes, ownership/link checks and Windows ownership handling. |
| `tools/cloudflare/dependency_test.go`, `setup_test.go` | Isolated APT failure fixtures and resumable setup tests exist; these are not upgrade/service/Docker qualification. |
| Resolver `assets/flatpak/package.json` | Bundled cloudflared is required; sandbox upgrades must use the owning signed package/application distribution path, never mutate read-only bundled tools or run host APT. |

Core broker/worker reuse points and heartbeat cancellation limits are recorded in
TASK-049. Existing setup does not inventory active processes, detect vulnerable
versions, update service/container owners or establish post-upgrade recovery.

## Proposed design and security considerations

- Implement the optional lifecycle operations in the resolver package. Enumerate
  executable paths, all relevant services/processes, package-manager ownership,
  container image digest/entrypoint and architecture. Correlate installed and
  active versions; never assume PATH's client owns the running tunnel.
- Maintain an explicit supported-method matrix: Linux native binaries and
  apt/dnf/rpm-managed installations; macOS native/pkg/Homebrew and launchd;
  Windows native/approved package manager and Service Control Manager; Docker
  on supported hosts. Discover other managers without taking over their state;
  unsupported combinations return a manual remediation plan and qualification gap.
- Select updates through the existing installation owner: manager-native upgrade,
  verified native binary replacement, signed distribution update or exact image
  digest redeploy. Do not call self-update over a manager-owned binary, bypass
  package-manager locks, or treat a mutable Docker tag as exact provenance.
- Provider metadata declares affected/fixed versions and artifact/method-specific
  applicability; TASK-051 normalizes evidence. Enforce Core pin/security conflicts
  and signature/checksum policy; preserve OS package trust and signing/notarization
  verification where applicable. No disabling TLS verification as remediation.
- Snapshot nonsecret configuration identity/permissions and privately protect any
  required backup. Preserve tunnel IDs, DNS/ingress configuration, credential
  paths/bytes/ACLs, service identity/arguments and existing connections where the
  method permits. No login, credential rotation or tunnel recreation by default.
- Show restart/disruption explicitly, including package-manager post-install
  restarts. Critical tunnels require a separately bound approval or explicit
  maintenance policy; update policy alone grants no restart authority. Use redundant
  connectors/rolling replacement when available; disclose unavoidable interruption
  for a single connector rather than promise zero downtime.
- Updating a connector carrying this broker connection can stop heartbeat and
  cancel the remote job. Require verified redundant or out-of-band management,
  or a bounded preauthorized target-local recovery/supervision path using existing
  workflow/service capabilities. The target must retain exact recovery authority
  through the window; no unrestricted detached shell or second remote agent.
- After activation verify active binary/image version, service/process readiness,
  connector registration, authenticated broker reachability and an authorized
  end-to-end tunnel health probe. Validate the application's actual TLS path;
  distinguish staged, installed, active and healthy states.
- Journal method-specific phases before mutation. Reconcile after reboot,
  heartbeat loss or ambiguous manager exit; broker result retries must never
  rerun an upgrade. Recovery restores approved safe bytes/configuration or applies
  a verified forward fix; vulnerable downgrade is a recorded emergency exception.

## Components and implementation phases

External files above plus package-owned lifecycle helpers/workflows, tests,
platform metadata, distribution tool pins and docs. Core changes are only generic
TASK-049 capabilities and existing `src/lib/infrastructure/remote/` integration.

1. P0 lane: artifact-specific advisory mapping and inventory/remediation runbook;
   verify macOS exception and operator-controlled recovery prerequisites.
2. Implement read-only discovery/assessment/plan and Linux native/manager fixtures
   against TASK-049; include all platforms in inventory contracts from the outset.
3. Implement approved apply/health/recovery by method, then macOS, Windows,
   Docker and bundled-artifact paths. Qualify each separately with TASK-052.
4. Enable scheduled notify, then opt-in updates only for qualified methods and
   approved maintenance/recovery policies. Publish through owner release gates.

## Acceptance criteria

- [ ] Installed and running cloudflared versions and provenance are visible for
  local and paired remote machines on Linux, Windows and macOS; multiple installs,
  stopped services, inaccessible processes and unsupported methods are explicit.
- [ ] Advisory fixtures detect pre-2026.10.2 exposure and apply the macOS 2026.10.3
  requirement conservatively until channel evidence resolves its exact scope.
- [ ] Every supported matrix row has verified update, preservation, health and
  recovery proof; unsupported rows cannot auto-update or report successful remediation.
- [ ] Critical tunnel restarts and indirect manager restarts are blocked without
  appropriate exact approval/maintenance policy and a validated recovery path.
- [ ] Configuration, credentials and tunnel identity survive upgrade/failure;
  active remote reachability and version are proven after reconnect.
- [ ] No Cloudflare behavior, source paths or vendor thresholds enter Core.

## Tests required

Extend existing package fixture tests for version/process detection, manager
ownership, unavailable releases, signatures/checksums, multiple clients, private
credential preservation and denied restarts. TASK-052 adds real native/service/
container matrices, no-connection recovery, broker retry nonreplay, health failure,
rollback failure and private audit checks. Package publication and live tunnel
qualification remain separate from mocked success.

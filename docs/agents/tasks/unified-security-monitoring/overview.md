# TASK-057 Unified Security Monitoring & Vulnerability Detection

- **Project:** Dockpipe
- **Component:** Security Monitoring
- **Priority:** Medium — after Environments consolidation
- **Status:** Proposed — deferred, unclaimed; documentation only
- **Implementation:** Native, deterministic, cross-platform
- **AI dependency:** None

## Planning status and related owners

This proposal is backlog only. Select it after Environments consolidation; it does
not authorize implementation, scanning, privileged inspection, or remediation.
The consolidation prerequisite must be explicitly accepted before activation.

- [TASK-051](../dependency-advisory-monitoring/overview.md) owns normalized dependency
  advisory evidence, applicability, freshness and notification contracts. Reuse
  those contracts rather than creating a second vulnerability feed pipeline.
- [TASK-049](../dependency-lifecycle/overview.md) owns generic dependency assessment
  and lifecycle contracts; provider mappings remain package-owned.
- [TASK-053](../adaptive-remote-threat-containment/overview.md) owns adaptive remote
  threat signals and session isolation. This epic provides deterministic posture
  findings, not a replacement containment or EDR system.
- [TASK-043](../configurable-networking/overview.md) owns configurable networking;
  coordinate listener policy and exposure evidence at that boundary.

## Objective

Introduce a top-level **Security** tab in DockPipe to monitor connected physical machines, virtual machines, containers, and other managed environments.

Detect exposed network services, dangerous firewall configurations, known software vulnerabilities, missing security updates, and infrastructure misconfigurations.

Provide actionable findings, severity classifications, remediation instructions, optional automated fixes, and ongoing monitoring.

The feature must operate without LLMs or external AI services.

## 1. Network Exposure & Port Monitoring

- Enumerate listening TCP and UDP ports.
- Identify associated processes, services, and executables.
- Inspect host firewall configurations.
- Detect unrestricted inbound firewall rules.
- Detect unnecessary `0.0.0.0` and IPv6 wildcard bindings.
- Identify publicly exposed administrative services.
- Detect unexpected changes to open ports.
- Inspect UPnP/NAT-PMP router mappings where supported.
- Support explicitly configured external or self-hosted probes to verify public TCP exposure.
- Support configurable approved ports, IP addresses, and subnets.
- Identify IPv6 exposure independently of IPv4.
- Detect firewall rules that no longer correspond to installed services.
- Distinguish confirmed exposure, suspected exposure, and unknown status.

### DockPipe LAN Security Enforcement

When direct LAN access is enabled:

1. Inspect local listener and firewall configuration.
2. Verify public reachability using an external probe.
3. Refuse to activate the LAN listener only when public exposure of that exact
   Dockpipe endpoint is confirmed through authenticated endpoint identity or an
   equivalent scoped proof. A reachable public IP and port alone are insufficient.
4. Provide remediation instructions.
5. Recheck after relevant network changes.

Never automatically configure router port forwarding.

A failed or unavailable external check must not be represented as proof of safety.
LAN-only installations must remain operable without cloud connectivity. Keep
unverified reachability visible as unknown; do not silently convert it to safe.
External probes must be explicitly configured and authorized for operator-owned
targets; permit self-hosted probes instead of depending on a free public service.

## 2. Software Vulnerability Detection

- Inventory installed software, OS packages, and versions.
- Identify known CVEs affecting installed versions.
- Integrate with OSV, NVD, CISA KEV, and vendor security advisories.
- Detect vulnerabilities with known active exploitation.
- Identify end-of-life operating systems and runtimes.
- Detect missing critical security patches.
- Scan container images and application dependencies.
- Identify known compromised or malicious package releases.
- Detect vulnerable development tools, servers, and libraries.
- Provide fixed versions and remediation instructions.
- Track vulnerability status after updates.

Prioritize findings using exploitation evidence, network exposure, asset criticality, and patch availability rather than CVSS alone.

## 3. Firewall & Network Configuration Auditing

- Detect disabled host firewalls.
- Detect overly broad inbound permissions.
- Identify unexpected public network profiles.
- Detect insecure SMB/NFS sharing configurations.
- Identify exposed database administration interfaces.
- Detect unrestricted Docker daemon endpoints.
- Identify insecure DNS configurations where observable.
- Detect missing TLS on sensitive network services.
- Identify expired, self-signed, or incorrectly configured certificates where relevant.
- Detect unexpected network configuration changes.

Self-signed certificates must not automatically be considered vulnerabilities when explicitly trusted and properly scoped.

## 4. Operating System Security Checks

Implement OS-specific checks for Linux, Windows, and macOS.

- Security update status.
- Disk encryption status.
- Secure Boot status where supported.
- Host firewall status.
- Remote access configuration.
- Unnecessary privileged services.
- Weak file and directory permissions.
- Unexpected privileged accounts.
- Insecure startup services.
- Security auditing and logging configuration.
- Unsupported OS versions.
- Available endpoint protection status where supported.

Checks requiring elevated privileges must request explicit authorization.

## 5. Container & VM Security

- Detect privileged containers.
- Identify dangerous host mounts.
- Detect containers running as root.
- Identify exposed Docker or Kubernetes management APIs.
- Detect unrestricted container network configurations.
- Scan container images for known vulnerabilities.
- Detect outdated base images.
- Identify missing resource limits where relevant.
- Detect unsafe VM networking configurations.
- Identify exposed hypervisor management interfaces.
- Check VM guest-tool versions and security update status where available.
- Detect unexpected VM or container configuration changes.

## 6. Credential & Secrets Security

- Detect secrets accidentally committed to managed repositories.
- Identify credentials exposed in workflow logs.
- Detect insecure credential file permissions.
- Identify unencrypted private keys where policy requires protection.
- Detect exposed environment files containing credentials.
- Monitor failed authentication attempts where supported.
- Identify unexpected changes to authorized access configurations.
- Detect expired certificates and credentials where metadata is available.

Do not collect or transmit secret values for centralized analysis. Prefer local scanning and redacted findings.

## 7. Security Monitoring Dashboard

Create a top-level **Security** tab with:

- Overall security posture by environment.
- Critical, high, medium, and low severity findings.
- Newly detected vulnerabilities.
- Publicly exposed services.
- Missing security updates.
- Firewall configuration warnings.
- Security trends and historical findings.
- Last successful scan and coverage status.
- Recommended remediation actions.
- Findings grouped by machine, VM, container, or service.

Support filtering, sorting, acknowledgment, suppression with justification, and finding history.

Record each check as passed, finding detected, unsupported, permission denied,
failed, or stale. Record observation time, last successful scan, evidence source
and scope, and freshness independently of severity. Suppression or acknowledgment
must not turn a finding into a passed check. Do not present an environment as
secure, or show a reassuring aggregate posture score, when required coverage is
incomplete, unavailable, or stale.

## 8. Notifications & Remediation

- Notify users when critical vulnerabilities are discovered.
- Alert on newly exposed administrative ports.
- Notify users about known exploited vulnerabilities.
- Remind users about outstanding critical updates.
- Detect newly introduced insecure configurations.
- Support desktop notifications.
- Support optional mobile notifications through DockPipe Cloud.
- Provide OS-specific remediation instructions.
- Offer automated firewall corrections with approval.
- Offer supported package updates with approval.
- Verify that remediation succeeded.
- Maintain an audit history of security changes.

Automated remediation must be optional, narrowly scoped, and require authorization for privileged changes.

## 9. Security Architecture

Implement security monitoring as independent, deterministic modules contributed
by a Security package. Package checks, platform resolvers, advisory providers and
remediation assets belong in the owning package repository. The Security tab is a
package-contributed application surface; the GUI shell and runtime remain generic.
Do not hardcode security products, feed vendors, platform commands or package names
in Core. Add a Core primitive only for an independently useful generic contract.

Use existing environment identity, capability authorization, findings/status,
notification and audit contracts where suitable. Scheduling must work through
operator triggers or existing scheduling interfaces without requiring DorkPipe,
AI services or a new Core security scheduler.

**Core components:**

- Security monitoring service.
- Host inspection adapters.
- Network exposure checker.
- Vulnerability intelligence provider.
- Software inventory collector.
- Configuration policy evaluator.
- Findings database.
- Notification dispatcher.
- Remediation interface.

**Architectural requirements:**

- Reuse DockPipe's existing machine identity and environment model.
- Support local and remote environments.
- Keep sensitive inspection data local by default.
- Avoid permanent administrator/root privileges where possible.
- Cache vulnerability intelligence for offline operation.
- Support fully self-hosted deployments.
- Allow security checks to run independently.
- Avoid duplicate findings across repeated scans.
- Record evidence and timestamps for findings.
- Rate-limit external scanning requests.
- Never perform unauthorized network scanning.
- No dependency on DorkPipe, LLMs, or AI inference.

## 10. Suggested Delivery Phases

### Phase 1 — Local Security Visibility (MVP)

- Package-contributed Security tab and stable, deduplicated findings model.
- Explicit check outcomes, coverage, freshness, evidence and scan failures.
- Local listening-port inventory with process attribution where permitted.
- Host firewall inspection through platform resolvers.
- Manual OS-specific remediation guidance.
- Optional, authorized public TCP exposure verification; unavailable probes
  remain unknown and do not prevent offline local inspection.

### Phase 2 — Vulnerability Inventory and Matching

- Basic software inventory and authoritative advisory matching using TASK-051
  contracts, including vendor version semantics and security backports.
- Fixed-version guidance and critical security update reminders.
- Bounded local intelligence cache, offline assessments, freshness and expiry
  reporting from the first vulnerability-detection release.
- Prioritization by exploitation evidence, exposure, asset criticality and fixes.
- Unknown inventory, missing feed coverage and stale evidence stay explicit.

### Phase 3 — Environment-Wide Monitoring

- Remote machine monitoring through existing authorized environment access.
- Container and VM security checks.
- Scheduled scans and configuration change detection.
- Deduplicated notifications and vulnerability history.
- Cross-environment dashboard with per-environment coverage.

### Phase 4 — Optional Remediation and Enforcement

- Optional, narrowly scoped automated remediation with authorization and
  post-action verification.
- Network exposure enforcement only with exact endpoint attribution.
- Credential and secret exposure checks with local redaction.
- Security policy profiles, optional mobile alerts and exportable reports.

## Acceptance Criteria

- [ ] Security monitoring works without AI services or DorkPipe.
- [ ] Checks, platform resolvers, advisory mappings and remediation stay package-owned;
  the runtime and GUI shell consume generic contracts and application surfaces.
- [ ] Checks distinguish passed, finding detected, unsupported, permission denied,
  failed and stale outcomes; incomplete coverage cannot imply a secure environment.
- [ ] Exposure enforcement requires proof identifying the exact Dockpipe endpoint;
  an open public IP/port or an unavailable probe cannot establish that proof.
- [ ] Vulnerability detection ships with bounded offline intelligence caching and
  explicit freshness, expiry and missing-coverage reporting.
- [ ] Users can inspect security findings for individual environments.
- [ ] Listening ports and associated services are discoverable where permitted.
- [ ] Host firewall configuration issues are detected.
- [ ] Public TCP exposure can be verified through an external service.
- [ ] IPv4 and IPv6 are evaluated separately.
- [ ] Known vulnerable software versions are identified using authoritative advisories.
- [ ] Findings include severity, evidence, affected resources, and remediation guidance.
- [ ] Scan failures are distinguishable from successful checks.
- [ ] Security checks do not require permanent elevated privileges.
- [ ] Privileged remediation requires explicit user approval.
- [ ] Self-hosted installations remain functional without DockPipe Cloud.
- [ ] Security monitoring supports Linux, Windows, and macOS through platform-specific adapters.
- [ ] Existing remote desktop, workflow, and environment functionality remains unaffected.

## Non-Goals

- Antivirus or malware signature scanning.
- Full endpoint detection and response (EDR).
- Intrusion prevention systems.
- Automated penetration testing.
- Unauthorized scanning of third-party infrastructure.
- AI-driven security analysis.
- Replacing dedicated enterprise SIEM platforms.

**Outcome:** DockPipe provides native, cross-platform infrastructure security visibility and actionable vulnerability monitoring directly alongside its existing environment management capabilities.

# TASK-048 — SignPath Foundation signing for Windows releases

Priority: **Medium–High**. Initial status: **Pending SignPath Foundation application
and approval**. Added 2026-10-24; ownership unclaimed.

This is a backlog record, not an application submission, signing integration or
website deployment. Do not interrupt the current VM, extraction or staging work.
No signing implementation starts until Foundation approval is confirmed and a
bounded implementation request is authorized.

## Ownership

| Owner | Scope |
| --- | --- |
| Dockpipe repository | Primary owner: application prerequisites, Windows release signing, artifact verification and publication gates |
| Website owner for `dockpipe.com` | Small attribution change on the Windows MSI download card and links to the public signing policy |

Keep the existing installer/update experience, release channels, manifest formats,
checksum distribution and Cloudflare infrastructure. Do not introduce Azure
Artifact Signing or any paid signing subscription.

## Phase 1 — Website and application preparation

1. Locate the website's existing Windows MSI card. Below its installer description,
   add a small, muted line without changing layout, styling or download behavior:

   > Windows code signing through SignPath Foundation is pending approval.

   Link “SignPath Foundation” to <https://signpath.org/>. This wording describes a
   pending plan; do not imply an application was submitted unless recorded.
2. Verify the actual public download-page URL, product description and working MSI
   link for the application. Record that URL and the application status/date.
3. Complete the prerequisite review below and resolve blockers before applying.
   Draft a truthful public code-signing policy with team roles and privacy details;
   do not present the service as operational before approval and verification.

### Current requirements and blockers

Reviewed 2026-10-24; the Foundation currently labels its
[terms](https://signpath.org/terms) draft. Recheck before applying.

- Eligibility: OSI-approved licensing, no commercial dual licensing or proprietary
  components, active maintenance, existing releases and public product documentation.
  Dockpipe's root license is Apache-2.0; the complete Windows payload still needs
  a component/license audit. Acceptance and project reputation remain unconfirmed.
- Policy: publish “Code signing policy” links on home/download pages, identify
  authors/reviewers/approvers, require MFA, and document privacy, installation
  changes and uninstall behavior. Validate actual behavior before making claims.
- Attribution: the required policy text is “Free code signing provided by
  [SignPath.io](https://about.signpath.io), certificate by
  [SignPath Foundation](https://signpath.org/)”. The requested card line alone
  does not satisfy that policy requirement.
- Signing scope: sign project-owned binaries; inventory upstream DLLs separately
  rather than signing them as Dockpipe. Enforce project name/version metadata.
- Approval: every release requires manual signing approval. Automate submission
  and verification around that decision, not approval itself.

The official [GitHub integration](https://docs.signpath.io/trusted-build-systems/github)
requires trusted build/origin verification, a workflow artifact uploaded before
submission, and GitHub-hosted runners for OSS jobs leading to signing. Audit the
actual release graph, external artifact inputs and permissions before asserting
compliance. Confirm the Foundation's accepted artifact configuration and access
requirements during [application](https://signpath.org/apply).

Open prerequisites: Foundation acceptance; full payload licensing/ownership review;
public policy and role assignments; website/download URL verification; and build
origin/runner/artifact eligibility. None is marked complete by this record.

## Phase 2 — After confirmed approval

1. Record approval and the accepted signing policy/configuration. Integrate with
   the existing Windows release pipeline; inspect `.github/workflows/release.yml`,
   `release/packaging/msi/` and their tests at implementation time.
2. Protect authorization using the existing secret-reference and release approval
   boundaries. Keep signing keys out of the repository, artifacts and logs; limit
   submitter/approver access and eligible release sources.
3. Sign supported Dockpipe-owned executables and the containing MSI in the order
   required by the approved artifact configuration. Verify final signatures,
   certificate trust, expected publisher and signed contents before publication.
4. Calculate release hashes and manifests from the final signed bytes. Keep their
   existing schemas, channel routing and Cloudflare distribution. Never sign or
   mutate bytes after their published hashes are finalized.
5. Block production publication when required signing, approval or verification
   fails; do not silently substitute unsigned artifacts. Define staging/test
   behavior explicitly without changing current releases before approval.
6. Only after verified signing is operational, change the muted card line to:

   > Windows installers are digitally signed using certificates provided by the SignPath Foundation.

   Link the attribution to <https://signpath.org/> and retain the required public
   policy attribution. Do not claim signing coverage for unverified channels or
   older unsigned downloads.

## Acceptance

- Website accurately shows pending or operational status, preserves downloads and
  provides the required policy links/attribution.
- Eligibility, application prerequisites, blockers and approval evidence are recorded.
- Approved Windows releases are signed and automatically signature/trust verified,
  with the Foundation's manual approval step preserved.
- Missing, denied, failed or tampered signatures prevent required production
  publication; successful signed bytes match manifests and downloaded checksums.
- Install, upgrade and uninstall checks pass for the signed MSI. Existing CI/CD,
  release channels and Cloudflare delivery continue working.
- No paid signing infrastructure, premature signing claims or pre-approval signing
  implementation is introduced.

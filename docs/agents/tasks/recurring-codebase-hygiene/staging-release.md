# Independent packages and staging releases

Objective contract:
- objective_id: staging-release-channel
- state: waiting_for_user
- execution_skill: dorkpipe-objective-execution
- execution_authority: approved_objective_creation
- authorized_objective: independently version package output; automatically build and publish an installable staging candidate after successful staging CI; prepare isolated Terraform infrastructure through the existing package.
- done_when: focused version/cache, release admission/publication and infrastructure isolation checks pass; staging infrastructure and publication credentials are ready; the full release path is prepared for activation and end-to-end qualification after the required Git checkpoint/promotion approval.
- inherited_invariants: production remains master-only; package/engine boundaries and unrelated checkout state are preserved.
- explicit_exclusions: production release and branch promotion remain excluded. The user approved staging infrastructure apply, full staging preparation, the completed GitHub credential setup, and commit/push of these prepared changes on js/pipelang. The user will handle MRs. Production credential changes and unrelated editor files remain excluded.
- checkpoint_policy: automatic_within_objective
- verification_policy: focused executable tests, workflow validation, then affected regression suites and a Terraform plan when credentials permit.
- handoff_policy: user_requested_only
- context_pressure_policy: warn_and_continue
- checkpoint_output_policy: quiet_success_bounded_failure
- terminal_conditions: completed | blocked | failed_verification | cancelled

## Decisions

- Publish automatically after Linux and Windows CI succeed on a staging push.
- Use `packages.staging.dockpipe.com`, bucket `dockpipe-staging`, and separate Terraform state.
- Packages retain independent manifest versions; generated children inherit their nearest owning package.
- Candidates have immutable identities incorporating the CI run and attempt. Native installer versions retain the planned numeric CLI version; repeated candidates require explicit installation, not automatic package-manager upgrades.
- Staging uses its own GitHub environment and bucket-scoped upload credentials.

## Implementation and verification — 2026-10-05

Implemented locally on `js/pipelang`, based on `1f215b219be5007d115e3959fca5584a129e034f`.
No commit, push, branch promotion, credential mutation or external publication occurred.
The protected `.vscode/settings.json`, both stashes and HEAD remain unchanged.

- Compiler versions now follow the nearest owner within the workdir. Explicit child
  versions win; external sources use their own manifest. Version changes invalidate
  cached child output. Inherited versions are written into compiled manifests and
  image provenance. Core still uses the CLI version.
- CI calls the release workflow after both test jobs succeed on staging pushes.
  Five-platform assembly precedes `release-staging` publication. Prerelease tags,
  immutable candidate stores/APT paths, source provenance and staging-only bucket
  guards separate candidates from production. Separate CodeQL is not a dependency
  of this CI job chain.
- The staging infra workflow composes the existing Cloudflare package, pins bucket
  and domain after vault injection, isolates module working files and remote state,
  and disables zone-wide cache/WAF ownership. It accepts planning commands only.
- Credential setup can target `release-staging`; actual staging provider credentials
  and the named secret-environment binding still need to be provisioned.

Passed:
- Go package version/compiler/store tests, including inherited-version cache invalidation.
- `bash release/packaging/test-runtime.sh` on the host (sandbox sockets are unavailable).
- All 23 Python release-tooling tests, including real stable/staging APT signing and
  isolated APT readers, candidate provenance and publication guards, credential target
  isolation, and production-setting rejection by the actual Terraform wrapper/pipeline.
- Focused Staticcheck, CLI build, both modified DockPipe workflow validations,
  ShellCheck, embedded-input membership check, path guard and whitespace checks.
- Actionlint 1.7.7 passed with only its obsolete `macos-15-intel` label diagnostic
  excluded; that unchanged runner already passed previous hosted qualification.

Generated outputs: the reviewed `embed_assets.go` membership update; a local compiled
staging workflow under ignored `bin/.dockpipe/`; temporary test logs, validation tools
and CLI under `/tmp/dockpipe-staging-*`. No saved Terraform plan exists yet.

Pending boundary: the live plan stops at `op inject` because 1Password desktop is
unavailable. The user was asked to open/unlock it. Once ready, rerun:

```bash
/tmp/dockpipe-staging-cli --workflow package-store-staging-infra --workdir /home/jamie/source/dockpipe --tf plan --
```

Use a reviewed host execution for existing vault/state/provider access. Inspect the
saved plan for only the staging bucket/custom domain; its apply is now approved.
GitHub credential writes still need separate authority. Full hosted matrix execution, public HTTPS/download
verification and actual candidate installation remain pending activation. Native
versions retain the planned numeric release version; repeated candidates require
explicit installation. Engine changes remain generic; provider/channel behavior
stays in repository workflows and release/package tooling.

## Live infrastructure authorization — 2026-10-05

The user approved the live Terraform operation. Generate and inspect the plan, then
apply only the staging bucket/custom-domain additions and verify the resulting
remote state. Use the shared `dockpipe-tfstate` bucket with the separate staging key.
This approval does not include Git publication or release credential configuration.

The approved retry still stopped at `op inject`, before Terraform. Started the
installed 1Password Desktop app and confirmed its host processes are running;
CLI 2.39.1-beta.01 still cannot connect. Asked the user to unlock the app and
approve any CLI prompt. No Terraform apply or infrastructure change occurred.
Apply authority persists while waiting for credential access.

## Applied staging infrastructure — 2026-10-05

Credential access recovered after the user's retry request. Inspected saved plan
`staging-plan.ODpdzwBc/staging.tfplan` under the staging workflow's private Terraform
artifact scope (SHA256 `5d41d2f4861d9531e5f069a82cc008054ac88f60abe621c069f36c7d64da06e8`).
It contained exactly two creates: `cloudflare_r2_bucket.publish` (`dockpipe-staging`)
and `cloudflare_r2_custom_domain.publish[0]` (`packages.staging.dockpipe.com`).
Verified backend bucket `dockpipe-tfstate`, key
`state/package-store-staging/terraform.tfstate` before apply.

Applied that saved plan through `dockpipe.cloudflare.r2infra` with init skipped and
its isolated module/data directory retained. Apply completed at 18:33:53 UTC:
2 added, 0 changed, 0 destroyed; remote state contains exactly two managed objects.
An initial optional package selector failed source lookup before Terraform; retrying
through the configured workflow lookup succeeded with no prior external effect.

Fresh post-apply plan `staging-plan.raeVLNbn/staging.tfplan` read remote state and
Cloudflare successfully: both resources are `no-op`. Only refreshed output metadata
would change. Domain ownership and SSL were pending at that read-back; the first
HTTPS probe failed during the handshake. Package publishing credentials, Git
publication, hosted qualification and candidate installation remain pending.

Final provider read-back: custom domain is enabled, ownership is `active`, and SSL
is `pending`. The first two public HTTPS probes failed the handshake while the
certificate was pending. Bucket/domain apply and remote-state verification are
complete; HTTPS readiness is not yet proven. The bounded implementation and approved
infrastructure work is complete. Staging publication credential setup, Git
checkpoint/push and hosted activation remain separately pending. No Git mutation
or production publication occurred. Protected HEAD, stashes and editor bytes were
rechecked unchanged. Live plan/apply/verification logs are in `/tmp/dockpipe-staging-terraform-*.log`;
saved plans stay in the private ignored workflow artifact scope.

## Full staging release activation — 2026-10-05

The user requested the complete production-equivalent artifact set on staging for
end-to-end testing. The existing reusable release path includes five native CLI
and package-store builds, Linux DEB/RPM/APK/Arch packages, Windows MSI, install
scripts, catalogs/checksums, a GitHub prerelease, and candidate-specific signed APT.
Added an artifact inventory and test-VM APT instructions to `release/docs/staging.md`.

The user supplied staging 1Password Environment ID `kuuppko2xxsb2wcr7am46bs6xa`.
`dockpipe.config.json` now defines `packages-staging` (upload pair) and
`packages-staging-release-setup` (upload pair plus signing key). No secret values
are stored in source. The saved staging signing key matches
`7295FA3FC25A3998E146D0779EAF522778B9C909`, expires 2028-10-04, and passed import,
unattended signing and verification. DockPipe's checks-only release setup passed
with the actual binding and target `release-staging`.

Created GitHub environment `release-staging` with no manual reviewer or wait timer
and with custom branch policies enabled. Automatic approval review then rejected
the combined staging-branch rule and credential-copy command because exact transfer
of sensitive staging credentials/signing key to this GitHub destination lacked
explicit approval. It did not execute. Read-back confirms zero secrets and zero
allowed branch rules; no deployment can use this environment yet. Required approval:
copy `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `APT_SIGNING_KEY` from the
staging 1Password environment into `Dockpipe-Industries/dockpipe` environment
`release-staging`, set the four public publication variables, and allow only the
`staging` branch. Do not retry until that approval and the credential correction below.

Read-only R2 readiness found that the saved staging credentials can successfully
list objects under an unused prefix in all three buckets: `dockpipe-staging`,
`dockpipe`, and `dockpipe-tfstate`. No object contents were read or changed; provider
responses and secrets were withheld. This fails the intended staging-only scope.
The user needs to restrict or replace the staging token with Object Read & Write
for `dockpipe-staging` only, update the saved pair, and have that scope rechecked.
Write permission has not been tested.

Cloudflare read-back still reports ownership active / SSL pending. Public HTTPS
fails its handshake. Public DNS resolves, and CAA queries at the hostname, its
staging parent, and zone apex return no restricting records. Certificate readiness
remains unresolved; Cloudflare documents Retry connection for stuck custom domains.

Current local checks: JSON/binding validation, release-setup workflow validation,
actual secret-backed checks-only setup, and `git diff --check` passed. Previously
passed release/runtime suites remain applicable; no release algorithm changed in
this activation step. Protected editor hash and both stashes are unchanged.
HEAD remains `1f215b219be5007d115e3959fca5584a129e034f` on `js/pipelang`;
remote staging was `55d7d10987e6fdd495ba1fd16f5b636a03119d63` at read-back.
No commit, push, promotion, release dispatch or artifact publication occurred.
Generated output: the release-setup workflow recompiled in its ignored package
cache; read-only readiness helpers are under `/tmp/dockpipe-staging-*`.

Next: correct and recheck token scope; obtain the exact GitHub-transfer approval;
finish staging environment configuration/read-back; resolve HTTPS; obtain the
normal-session checkpoint/push/promotion approval and activate the reviewed source
on staging. Then qualify the hosted matrix and public candidate installation before
any production release. Engine/package boundaries remain preserved.

### Credential scope corrected and verified

The user removed an unintended default token permission and requested a recheck.
The earlier audit was dismissed at the 1Password authorization prompt and had no
external effect. The fresh read-only audit passed: the saved secret access key
matches the saved API token; invalid-signature and unsigned controls are rejected;
the actual saved pair can list an unused staging prefix, while production and
Terraform-state requests return `AccessDenied`.

Cloudflare account-token metadata identifies active `dockpipe-packages-staging`
with exactly one allow policy, restricted to the `dockpipe-staging` bucket, granting
`Workers R2 Storage Bucket Item Write` and `Workers R2 Storage Bucket Item Read`.
No writes were performed. This supersedes the token-scope blocker above; no further
credential correction is needed. Explicit approval for the previously rejected
GitHub secret transfer, HTTPS readiness and Git activation remain pending.

### GitHub staging credentials applied and verified

The user explicitly approved the staging secret transfer. Read-back first confirmed
that the rejected operation had created no secrets or branch rules. Added exactly
one deployment rule (`staging`, type `branch`), then ran `package-release-setup`
with `packages-staging-release-setup`, environment `release-staging`, bucket
`dockpipe-staging`, and `RELEASE_SETUP_APPLY=1`. It validated the known staging
signing fingerprint and completed successfully at 2026-10-05 20:55:51 UTC.

GitHub read-back confirms the three approved secrets (`AWS_ACCESS_KEY_ID`,
`AWS_SECRET_ACCESS_KEY`, `APT_SIGNING_KEY`) and four expected public variables
(`APT_SIGNING_FINGERPRINT`, `DOCKPIPE_RELEASE_BUCKET`, `R2_ENDPOINT_URL`, `R2_PREFIX`).
The deployment rule permits only the staging branch; there are no reviewer/wait
gates. No secret values were printed. This supersedes the credential-transfer
blocker above. No release or Git publication was started by this setup.

The public HTTPS probe still fails its TLS handshake. HEAD and the unstaged source
boundary remain unchanged; whitespace checks pass. Remaining activation steps are
normal-session commit/push/promotion approval, SSL readiness, and hosted plus
public-installation qualification. Do not repeat the completed secret transfer
unless a later credential change or concrete failure requires it.

### Approved source checkpoint and push

The user approved committing and pushing the prepared changes on `js/pipelang`
and explicitly retained ownership of MRs. The checkpoint includes independent
package versioning, full staging release wiring, isolated infrastructure workflow,
staging secret-environment references, tests and documentation. It excludes
`.vscode/`, generated output and secrets. The strict request adapter rejected the
new untracked authored files before mutation (it currently accepts tracked
modifications only). Use the existing runtime checkpoint/publication operations
with a frozen exact-path/content/mode manifest and protected-state checks, as used
for prior checkpoints in this session. Verify the resulting commit and remote SHA
at `origin/refs/heads/js/pipelang`.
Do not merge or advance staging/master. Hosted staging publication awaits the
user's normal MR promotion, and public HTTPS readiness remains unresolved.

# Independent packages and staging releases

Objective contract:
- objective_id: staging-release-channel
- state: waiting_for_user
- execution_skill: dorkpipe-objective-execution
- execution_authority: approved_objective_creation
- authorized_objective: independently version package output; automatically build and publish an installable staging candidate after successful staging CI; prepare isolated Terraform infrastructure through the existing package.
- done_when: focused version/cache, release admission/publication and infrastructure isolation checks pass; staging infrastructure and publication credentials are ready; the full release path is prepared for activation and end-to-end qualification after the required Git checkpoint/promotion approval.
- inherited_invariants: production remains master-only; package/engine boundaries and unrelated checkout state are preserved.
- explicit_exclusions: production release and promotion to master remain excluded. The user approved staging infrastructure, GitHub staging setup, and source commit/push. For the VM harness repair, the user additionally authorized MRs and merges through js/pipelang -> js/dev -> dev -> staging. Production credential changes and unrelated editor files remain excluded.
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

## Hosted staging failure inspection — 2026-10-05

Checkpoint `62dda3a898a6a3c21a4ecab0d28810dc35552573` was pushed to
`origin/js/pipelang` and the remote tip verified. The user handled MR promotion.
Inspected actual failed steps and logs before changing source:

- Push run `37375091956` at `de9a23a9076c20156fcc6ef2d326159d62e726c6`
  passed Linux/Windows CI, native release builds and assembly. Publication failed
  at `Require the current staging head`: staging had advanced to
  `c99474992c97f9391aca34f68b2a726b0f527014`. This is the intended stale-candidate
  guard; GitHub release creation and R2 publication were skipped. Do not rerun
  that stale candidate or weaken the guard.
- PR run `37375911684` at the newer SHA failed the master-only release-notes gate.
  Its log reports `echo: write error: Broken pipe`, then incorrectly claims that
  `release/releasenotes/0.6.0.md` is absent, although the printed 2,268-path diff
  contains it. Under `pipefail`, the early-exiting `grep -q` can make the producer
  fail and invert the successful match.
- The newer staging push `37375906954` is still in progress with no failed jobs
  at the last read-back. Windows tests and CodeQL passed. No rerun was dispatched.

Replaced the `echo | grep` condition with fixed-string exact matching over a Bash
here-string, preserving both the version and changed-notes requirements. Added
executable regression cases against the authored gate for a 10,001-path diff and
absent/lookalike note paths. The original condition fails the large fixture while
the repaired condition passes. All eight release-workflow tests, Actionlint for
`ci.yml`, and whitespace checks pass. The failure is timing-sensitive with the
exact hosted path list; that smaller local replay happened to pass, so it is not
claimed as a deterministic reproduction.

The user approved committing and pushing this follow-up repair on `js/pipelang`,
with MR promotion still owned by the user. It changes
only workflow shell logic, its focused tests and task evidence; no engine behavior
or release admission policy changed. Logs are in
`/tmp/dockpipe-staging-{pr-failure-37375911684,publish-failure-37375091956,release-gate-tests}.log`.

## VM harness race in hosted staging regression tests

The approved release-notes fix was committed and pushed as
`15e09e093602fa7e632dd3c4a1b593f691b73bfa`; its remote tip was verified and the user
handled subsequent MRs. The next failing staging push run is `37379712446`, at
`0556843a56cdd745ba0f65c26d88fa46e3c883fd`.

Both CI test jobs and all five native builds passed, including Windows MSI. Linux
amd64 failed afterward in `Runtime and package regression tests`, specifically
`TestRunHarnessProvidesOnlyReviewedRoleAndLookupPath` in the VM guest package:
`decode pinned harness evidence: read |0: file already closed`. Assembly and both
publication jobs were skipped. This is separate from the prior release-notes gate.

`runHarness` started `process.Wait()` concurrently with JSON decoding from
`StdoutPipe`. Go's subprocess contract closes that pipe during Wait, so a fast
child exit could interrupt the read. The VM-owned fix starts Wait only after
evidence decoding/validation finishes. Failure paths still kill and reap the child
before returning; successful holding checkpoints still receive the wait channel.
No engine, workflow admission, version, or package isolation policy changed.

Verification:
- The ordinary existing harness test passed 1,000 times, confirming the failure is
  timing-sensitive rather than reliably reproducible without stress.
- Before the fix, the same existing test with `-race -count=1000 -cpu=1,2,4`
  reproduced the hosted error 19 times across 3,000 invocations.
- After the fix, all 3,000 race-enabled invocations pass.
- `dockpipe package test --only vm` passes the complete offline VM package suite,
  including all Go packages, state-split checks, four Python checks and package
  architecture guards. No live VM or provider operation was started.
- `git diff --check` passes. The first direct Go test command lacked `GOWORK=off`
  and stopped at module selection; subsequent commands correctly isolate the VM
  module and use cached Go 1.25.13 with network module downloads disabled.

Logs: `/tmp/dockpipe-staging-failure-37379712446.log`,
`/tmp/dockpipe-harness-{before,race-before,race-after}.log`, and
`/tmp/dockpipe-vm-harness-package-tests.log`. The repair is local on `js/pipelang`
and has user approval for checkpoint/push plus MR promotion through `js/dev` and
`dev` to `staging`. Reuse matching open MRs, verify their diff/checks before merging,
and stop before master. No Action retry was dispatched; the staging merge will
start the normal CI and candidate publication path.


## Reusable release environment secrets — 2026-10-05

The VM repair `d6293516` reached staging through merged MRs #35, #36 and #37;
staging head was `eccec1db1d11fb157f362afa7fe6fe85a462e86d`. Run `37384217934`
passed CI, native builds, MSI and assembly. The publisher stopped at configuration
validation: all three secret expressions were empty, while staging environment
variables resolved correctly. Read-only GitHub metadata confirmed the three secret
names still existed with their original setup timestamps. No publication ran.

The behavior matches https://github.com/actions/runner/issues/4453. The user
approved fixing it after diagnosis. The caller now uses `secrets: inherit`, the
reported workaround; the existing workflow contract test requires this setting.
The release guide documents the additional repository/organization secret
visibility. Publication remains in `release-staging`, with its staging-only branch
rule and bucket check. No secret values, credentials or production gates changed.
Continue the approved checkpoint/push and MR route through `js/dev`, `dev` and
`staging`; stop before master. Local checks and hosted confirmation follow.

Local validation passed: all eight release-workflow tests and `git diff --check`.
Actionlint 1.7.7 passed with only its outdated `macos-15-intel` runner-label check
excluded; the preceding hosted native build already passed on that runner.
These checks validate workflow structure and guards, not hosted secret delivery.

## Staging delivery verified; Homebrew prepared — 2026-10-05

Run `37389279395` completed successfully at staging SHA
`9725dee90d4d72522cb602d9690fea21e2b15ff7` and published candidate
`0.6.0-staging.37389279395.1.9725dee90d4d`. The R2 hostname now has active SSL
after the staging-only `/.well-known/` redirect exception and connection retry.
HTTPS reads of the pointer, catalog, all five store manifests, installer and APT
metadata passed; `gpgv` verified the expected staging signing fingerprint.
This supersedes the hosted-publication and TLS blockers above. Native installation
qualification remains separate.

The user requested Homebrew support and selected a tap that polls every 15 minutes
using its own GitHub token, without a new secret. The source under
`release/packaging/homebrew/tap/` prepares a candidate formula from the existing
public artifacts, requires successful matching staging CI, validates catalog and
artifact checksum inputs, gates publication on Intel/Apple Silicon Homebrew tests,
and rechecks freshness before an idempotent formula update. A launcher uses the
existing `DOCKPIPE_SYSTEM_ROOT` override for the keg's complete package store.
No engine, main release workflow, production credential or bucket changes were
needed.

Local checks: all 32 release-tool tests passed (the existing APT test required a
reviewed host run for its temporary GPG agent); focused Homebrew tests and tap
workflow Actionlint passed. Actionlint's known outdated `macos-15-intel` label
check was excluded. Read-only generation against the live candidate succeeded;
preview: `/tmp/dockpipe-homebrew-prepared/dockpipe-staging.rb`.

At this checkpoint the tap repository is absent, source changes are uncommitted,
and no native Homebrew installation or formula publication has run. Activation
requires the public tap seed on `main` and its first successful workflow. The
stable formula remains an unpublished stub. Preserve `.vscode/` and existing
stashes during any approved checkpoint; do not promote to master.

### Approved Homebrew activation completed

The user explicitly approved source commit/push, creation and seeding of the public
tap, and its first native validation. Source checkpoint
`9b7aac9a48ece11597e184f8a93312fd26ea1b97` was pushed and verified on `js/pipelang`.
The public `Dockpipe-Industries/homebrew-dockpipe` repository was seeded on `main`
at `5e219cada65a2de05da3782a12436744731e4dec` with the reviewed tap source and license.

Run `37397290496` succeeded: preparation, Apple Silicon installation/formula test,
Intel installation/formula test, version ordering, and gated publication all passed.
The published formula exactly matches the locally reviewed candidate preview;
its blob is `0a93ffe63cdf56f90db5c000557b3f7c7bc473f6`. The update workflow is active
with its 15-minute schedule. Native logs and publication receipts are saved under
`/tmp/dockpipe-homebrew-publication-20261005/`.

This supersedes the tap-activation blocker above. Homebrew staging installation is
qualified on the hosted Macs; user hardware, APT/MSI installation, and broader
end-to-end product behavior remain distinct checks. No master promotion occurred.

### Desktop installation cleanup — 2026-10-05

User approved a consistent Desktop versus CLI / Remote Worker install, including
an ordinary macOS DMG that installs both CLI and launcher. Implemented locally:

- Native Qt desktop build steps for both Mac architectures, Windows x64 and Linux
  amd64/arm64. Windows release MSI now requires the deployed launcher, Qt plugins
  and app-local VC runtime; no silent CLI-only fallback. The CLI feature remains
  mandatory, while the default-enabled launcher can be removed/restored.
- macOS app bundle, complete native store and CLI wrapper; direct DMG contains an
  Apple Installer package for Applications plus `/usr/local/bin/dockpipe`. Its
  preinstall guard preserves foreign commands/apps. Developer ID and notarization
  hooks accept existing keychain references; CI currently produces development
  signatures only, not a Gatekeeper-qualified public desktop release.
- Homebrew desktop cask source with formula dependency, pinned native app ZIPs,
  provenance checks and both-architecture validation. Paired publishing checks
  both versions first and recovers an interrupted formula/cask update. Removed
  the invalid conflict with the unpublished stable formula.
- Linux desktop DEB, icons and application menu entry, exact CLI dependency and
  distribution-derived Qt ABI dependencies. APT now indexes both packages. Docker
  moved from required dependency to Suggests in the CLI DEB, matching host-only
  workflows and the existing other Linux package formats.
- Download catalog and install docs distinguish Desktop from CLI. Packaged launcher
  searches its sibling CLI before PATH, retaining explicit/development overrides.
  Changes remain in release tooling and the standalone launcher; no engine change
  is required for this packaging objective.

Local proof: native Linux Qt build, real desktop DEB extraction and icon/menu
payload, deployed Qt/CLI diagnostic with SDK paths removed, five-second window
startup, and real native host workflow passed. All 36 packaging Python tests passed,
including actual temporary-key signed APT/index consumption via a reviewed host
run. Windows PowerShell parameter/parse checks, wrapper/ownership tests, shellcheck
and workflow actionlint passed (the older local actionlint runner catalog required
ignoring only its existing `macos-15-intel` label diagnostic; existing unrelated
workflow ShellCheck warnings were not changed).

Receipts/artifacts: `/tmp/dockpipe-desktop-packaging-final.log`,
`/tmp/dockpipe-desktop-apt-tests.log`, `/tmp/dockpipe-desktop-smoke.log`,
`/tmp/dockpipe-desktop-build-final.log`, `/tmp/dockpipe-desktop-artifacts/` and
`/tmp/dockpipe-desktop-extracted/`. Native Mac DMG/Homebrew and Windows MSI checks
are authored but have not run for this change. No new release, tap update, commit,
push, native installer execution on user machines, or signing provisioning occurred.
The activation order is native dry-run, Apple signing qualification for normal
public desktop delivery, desktop candidate publication, then reviewed tap update.

### Approved checkpoint and native workflow verification

The user approved committing the reviewed work and running the workflows after
the proposed branch checkpoint/push and native dry-run. Scope is the completed
remote workflow delivery plus desktop packaging on `js/pipelang`, CI and release
verification with `dry_run=true` and `build_msi=true`. It does not authorize master
promotion, public release/tap publication or Apple credential provisioning.
Pre-commit admission found the new remote smoke asset missing from the authored
embed manifest; regenerated `embed_assets.go` adds only that reviewed file.

First approved checkpoint `a4b7f2b9c9d55bb2fd4ebc7f9c9ca99a9aeb1268` was pushed
to `origin/js/pipelang`. CI run `37407548104` passed Windows/runtime/security but
stopped Linux at Staticcheck U1000 for the unused remote `pair` wrapper. Release
dry-run `37407555314` passed both Linux architectures and their desktop checks.
Both Macs built/installed the DMG and ran the CLI workflow, then failed launcher
smoke because `DockPipe` and wrapper `dockpipe` collide on case-insensitive APFS.
Windows stopped at quoting of the Visual Studio environment bootstrap path.
No publication job ran.

Focused repairs remove only the unused helper, use `dockpipe-cli` for the internal
Mac wrapper while retaining the public `dockpipe` command, verify the GUI remains
Mach-O before deployment, and initialize Windows through `Launch-VsDevShell.ps1`.
Remote-command tests/Staticcheck, wrapper ownership tests, PowerShell parsing and
shellcheck pass locally. The user's workflow-verification approval covers this
bounded repair checkpoint and fresh CI/release dry-run; no release/tap publication
or promotion is included. Logs are `/tmp/dockpipe-desktop-ci-37407548104-failure.log`
and `/tmp/dockpipe-desktop-release-37407555314-failure.log`.

Repair checkpoint `7cbfb77feb47b75fed458959de35f40f5cdf6a33` was pushed.
CI `37408380164` passed all checks, including security, Staticcheck, runtime,
shell and Docker integration. Release retry `37408385904` passed both Linux
jobs; Apple Silicon confirmed the corrected app/CLI layout but its smoke forced
an undeployed `offscreen` Qt plugin (the deployed Mac plugin is `cocoa`). Windows
compiled/deployed the launcher and built/installed its MSI, but the same smoke
configuration timed out before reporting the CLI. Logs were read before repair.

The next narrow repair tests the deployed native Cocoa/Windows plugins, retaining
headless offscreen only on Linux, compares Qt CLI output as filesystem paths so
Windows separator spelling is irrelevant, and includes plugin diagnostics on a
future timeout. The Homebrew native check uses Cocoa too. Native verification is
still required; a Windows timeout is not itself proof of a plugin cause. Local
Linux diagnostic/window smoke and Homebrew tests pass with the corrected test.

### Native desktop qualification passed

Verified code commit `962c1390dc27668fff56714c9b3443c6df199f3e` is pushed on
`js/pipelang`. Both final hosted runs completed successfully:

- [CI 37409454216](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37409454216):
  Linux and Windows runtime checks, security, Staticcheck, shell tests and Docker
  integration passed.
- [Release dry-run 37409460637](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37409460637):
  Linux amd64/arm64 desktop packages and startup passed; Apple Silicon and Intel
  DMGs installed the CLI and app, ran the CLI workflow and native Cocoa launcher,
  and passed managed reinstall checks. Windows MSI passed native launcher startup,
  CLI-only feature modification, restoration and uninstall. Artifact assembly,
  checksums, complete stores, temporary-key APT verification and dry-run upload
  passed. Production, staging publication and dev.to were all skipped.

The native-plugin smoke repair resolved both desktop test failures. Source and
runtime receipts are in `/tmp/dockpipe-desktop-publication-20261005/` and the two
`/tmp/dockpipe-desktop-repair{1,2}-20261005/` directories. Final job receipts are
`/tmp/dockpipe-desktop-ci-success.json` and
`/tmp/dockpipe-desktop-release-success.json`; native Windows and Apple Silicon logs
are `/tmp/dockpipe-desktop-windows-success.log` and
`/tmp/dockpipe-desktop-mac-arm-success.log`.

This final documentation-only receipt does not change the verified implementation.
Apple Developer ID signing/notarization, a new staging/public desktop release,
and deploying the reviewed cask source to the public Homebrew tap remain separate
unperformed steps. The existing public tap and release were not changed. Protected
`.vscode/settings.json` and both pre-existing stashes retain their original hashes.

### Staging desktop and Homebrew cask published — 2026-10-06

The user approved promotion through `js/dev`, `dev`, and `staging`. PRs #44–46
merged after staging PR CI and CodeQL passed. Staging commit `48a5701e3652` includes
the Colima compatibility follow-up `bebf0edd`; master remained unchanged.
[Staging CI 37414177001](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37414177001)
passed all native builds and published candidate
`0.6.0-staging.37414177001.1.48a5701e3652`.

To complete the requested Homebrew installation path, the six reviewed tap source
files were deployed through
[tap PR #1](https://github.com/Dockpipe-Industries/homebrew-dockpipe/pull/1).
[Tap run 37419531103](https://github.com/Dockpipe-Industries/homebrew-dockpipe/actions/runs/37419531103)
passed CLI/cask installation, launcher diagnostics, uninstall, and version ordering
on Apple Silicon and Intel, then published both definitions. Read-back matched
the tested candidate and confirmed the invalid stable-formula conflict was absent.
Receipts are under `/tmp/dockpipe-homebrew-desktop-20261006/`.

`brew install --cask dockpipe-industries/dockpipe/dockpipe-desktop-staging` now
installs the app and CLI dependency. Apple signing/notarization and live Colima VM
qualification remain open. The user reports having an Apple account; paid program
activation and signing credential provisioning have not yet been verified.

## Launcher package remotes — local implementation, 2026-10-06

The saved-only Package Remotes field and empty Marketplace have been replaced with an
asynchronous CLI-backed catalog/install flow. Settings expose production/staging presets;
new settings default to production, while an explicit empty list disables remote browsing.
The CLI resolves latest → release → native-platform store, pins displayed checksums, and
verifies bounded package archives before publishing to the existing user store. JSON package
inventory includes project/configured/user/system roots. No package execution model changed.

Local evidence: affected application/infrastructure/CLI Go suites passed; Qt build and subprocess
and dialog tests passed. The new CLI read 59 live staging packages for Linux amd64 and installed
one core, workflow, and resolver archive into an isolated `/tmp` root. No package was executed.
Native macOS/Windows runtime tests and release publication of this implementation remain pending.
Canonical behavior is in `docs/packages/package-model.md` and the launcher README.

The first promotion CI run (`37424683434`) passed Windows and security checks but exposed
an omitted validation input in the backlog fixture. Added `package_inventory.go` to the
explicit fixture input list and advanced its asserted count to 230. The exact failing
`test_backlog_remote_workflow.sh` passed with host loopback access; sandbox-only TLS
listener failures were not counted as a pass.


## Core-only installers and package actions — source correction, 2026-10-08

The macOS app and staging Homebrew formula incorrectly shipped the complete release
store, displaying every optional package as installed. The corrected source stages
only the verified core archive in the app and downloads only core for the formula.
The full platform catalog remains published for explicit Marketplace installations.
Linux native packages and Windows MSI already select core only. Installer smoke tests
now reject optional installed packages; the core staging test also rejects stale output.

The user explicitly excluded installer cleanup/migration logic. None is included.
A separate manual Brew uninstall/reset script is provided outside the repository;
it leaves the user package store, settings, and data intact. No host cleanup was run.

The package manager now offers Uninstall for optional user archives, protects core and
externally managed packages, disables duplicate version installs, and hides Cancel
when idle. Active cancellation stops both inventory and remote requests. Generic
`package uninstall --path` enforces the user archive boundary and retains package state.

Local validation: 37 release packaging tests and all five Qt tests passed, including
install/uninstall button states and bounded cancellation. Application and infrastructure
Go packages, CLI tests/build, staticcheck, configured gosec, and the backlog source fixture
passed. A real CLI inventory/uninstall round trip retained user settings. The broad
`go test ./src/lib/... ./src/cmd` attempt was stopped after application-IR tests refused
missing compiler-containment limits; it is not a full-suite pass. An exploratory gosec
run without repository configuration reported existing path/permission findings; the
repository-configured check on both affected packages passed. Native macOS installation,
Homebrew runner validation, and release/tap deployment remain pending. No commit or
publication has been performed for this correction.

Promotion PR #54 exposed Windows short/long ancestor spelling in the uninstall
boundary (`TestUninstallUserPackagePreservesOtherPackagesAndData`). Normalize both
store and selected archive after rejecting direct archive/category symlinks; retain
root-confined removal. The focused removal/inventory tests, including an ancestor-alias
regression, pass locally. The follow-up must pass native Windows CI before staging merge.

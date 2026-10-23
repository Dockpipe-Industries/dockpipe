# Standalone PipeLang and external packages

Objective contract:

```yaml
objective_id: TASK-044
state: executing
execution_skill: dorkpipe-objective-execution
execution_authority: approved_objective_creation
authorized_objective: Extract PipeLang and every remaining package from Dockpipe into independent public source repositories.
done_when:
  - PipeLang compiler, SDK, tests, documentation and workflows build independently.
  - Remaining packages live in dockpipe-packages with independent workflows and tests.
  - Dockpipe contains no packages directory or .staging/packages tree.
  - Dockpipe builds without the PipeLang compiler or sibling source checkouts.
  - Package consumption uses external artifacts and generic public contracts.
  - Documentation links point to the owning repositories and pass an audit.
inherited_invariants:
  - Preserve all existing authored work, including uncommitted Cockpit and launcher SDK changes.
  - Keep workflow-specific scripts with their workflows and reusable helpers at their owning shared layer.
  - Preserve behavior, tests, licensing and provenance; extracted Git history is unnecessary.
explicit_exclusions:
  - Commit, push, publication, infrastructure mutation and registry account setup require separate authority.
checkpoint_policy: automatic_within_objective
verification_policy: Focused tests followed by independent builds, package integration and documentation audit.
handoff_policy: user_requested_only
context_pressure_policy: warn_and_continue
checkpoint_output_policy: quiet_success_bounded_failure
terminal_conditions: completed | blocked | failed_verification | cancelled
```

## Ownership

- `Dockpipe-Industries/pipelang`: language compiler, language SDKs, native compiler commands, language-specific test harnesses and language design/reference documentation.
- `Dockpipe-Industries/dockpipe-packages`: every remaining authored package, package publishing workflows, and package-specific examples and documentation.
- `Dockpipe-Industries/dockpipe`: generic execution engine, launcher SDK, package installation/compilation machinery, generic workflow contracts and engine contributor workflows.
- Existing dedicated product repositories remain canonical for products already extracted.

## Admission

The starting Dockpipe revision is `e491c7938fd721a27f842ec8dc7bba9889d7f96e` on `js/dev`.
Existing changes include ForgePipe documentation, Cockpit graph behavior, launcher SDK production and tests, and an untracked native C++ PipeLang command. Preserve their current bytes at their owning destination.

## Progress

- Confirmed the user wants all remaining packages extracted, including Cockpit, IDE and native SDK.
- Confirmed the new catalog repository is `dockpipe-packages`; history preservation is unnecessary.
- Inventoried direct compiler imports, automatic package materialization, embedded package sources and release producer coupling.
- Implementation and validation are in progress. No extraction completion or release qualification is claimed.

## Continued validation checkpoint

The exact CLI pin checkpoint passed on 2026-10-22. Both staged external repositories
pin Dockpipe 0.6.25 and lock the five published platform archives by size and SHA-256.
Ten bootstrap integrity tests pass, five GitHub workflow YAML files parse, and the
published Linux CLI runs the standalone PipeLang build and bounded test workflows.
Catalog package workflow compilation also passes with that CLI. Windows ZIP selection
is covered with a platform simulation; native Windows execution remains pending CI.
Evidence: `/tmp/dockpipe-extraction/version-pin-checkpoint.json` and its linked logs.

Remaining preparation: replace the unavailable SDK release pin with a reproducible
core-owned artifact, remove copied catalog CLI-builder helpers, repair agent image
asset ownership, finish the optimized native camera check, reseal the external
PipeLang module, synchronize destination sources and rerun affected final checks.
Sources and release artifacts remain unpublished; initial branches and protections
cannot be installed until the separately authorized source publication gate.

## Prepared extraction and final checks (2026-10-22)

Implementation preparation is complete; source and SDK publication still require
separate user approval. The core-owned SDK now includes generic remote contracts
needed by the catalog. Its 272-file archive reproduces twice from committed core
source, and catalog secrets/remote tests, Linux builds and Windows cross-builds pass.
Copied catalog CLI-builder code is removed; SDK downloads use portable deadlines.
Agent resolver images contain package-owned entrypoints; Claude host Docker checks
and six compiled archive context checks pass without model API calls.
The optimized extracted Cockpit dirt-bike test now passes unchanged.

PipeLang was resealed from 680 authored inputs into a fresh canonical offline Go
module proxy and cache. Core go.sum matches that unpublished candidate. All 43
core library/command packages and make build pass against it without a go.mod
replacement. The catalog builds through its locked published CLI and SDK archive.
Both saved external checkouts were synchronized with postimage guards; three
obsolete unchanged files were removed. Core package source roots remain absent.

The final read-only documentation audit checks 234 core, 123 language and 54
catalog Markdown files with no missing local or checked cross-repository targets.
Public target availability awaits publication. The authored embed manifest is
regenerated and checked. git diff --check passes. Existing dirty native SDK work
and unrelated ForgePipe notes remain preserved.

Evidence lives in /tmp/dockpipe-extraction: version-pin-checkpoint.json,
go-sdk-checkpoint.json, catalog-helper-checkpoint.json,
dirt-bike-optimized-checkpoint.json, agent-image-checkpoint.json,
final-sealed-pipelang-module.json, final-sealed-core-tests.log,
final-sealed-core-build.log, final-pinned-catalog-build.log and
resumed-final-doc-audit.json. The publication review binds the prepared source
postimages and artifacts; it is evidence, not approval.

Limits: native Windows CI and physical GPU/gameplay are not qualified here.
Codex/Ollama image archives are structurally checked; only Claude was built/run.
The broad repository Go sweep retains the admitted generated Qt fixture limitation;
the full costly compiler campaign was not run. Initial master/staging/dev branches
and protections require published initial source commits. The previously read-back
public/free security settings are preserved; revalidate before any later mutation.
No commit, push, tag, release publication or paid security add-on was performed.

## Live Windows VM validation resumed

The user identified the existing Windows QEMU image and authorized opening it
through Dockpipe. Host preflight confirms the image is qcow2, 64 GiB virtual,
with usable KVM/QEMU/TPM tooling. The launch exposed two additional runtime defects:
public package-state subdirectories were rejected as migration tokens, and deep
runtime paths exceeded Unix socket limits. Generic fixes now resolve validated
package token suffixes and allocate short private temporary VM socket directories.
Focused migration tests cover selected legacy identity preservation, missing
suffixes, linked paths and cross-project rejection. Host socket bind checks pass.
The candidate core artifact was verified against the changed VM runner before use.

A Dockpipe-managed interactive VM is now running from an ephemeral overlay in a
fresh test project. Windows login/desktop state awaits user observation; this is
not yet native Windows build/test proof. Historical guest state is preserved:
the original context contains transient socket files and multiple legacy TPM
cohorts, so no migration authority was guessed. The guest-agent input is disabled
for this manual desktop session because its default host port is occupied.
No base disk rewrite, credential reset, host policy change or publication occurred.

The earlier prepared publication manifest is stale for core and catalog sources
and must be regenerated/reviewed after these runtime corrections and Windows
validation. No pending approval from that older payload authorizes changed bytes.
Evidence: /tmp/dockpipe-extraction/windows-validation/ and its live desktop log.

## Existing Windows guest services repair

The user confirmed the Windows desktop and account, then explicitly authorized
repairing and reusing the existing guest-services installer before native Windows
validation. Explicit unattended/auto installation now reaches the existing
first-logon path; that path invokes provision-windows-ssh.ps1 rather than
maintaining duplicate SSH setup. Manual installation remains the default, and
existing images still need one elevated helper invocation before trusted SSH.

The helper now preserves existing authorized keys, selects the standard shared
administrator key file when appropriate, keeps SSH settings outside Match blocks,
validates configuration before service mutation, and reloads a running sshd.
The SYSTEM guest agent remains loopback-only. Bootstrap executables use a separate
read-only disk; unattended XML remains on a small read-only floppy. Focused shell
and mocked PowerShell tests pass, and the candidate core builds.

The current ephemeral guest needs its elevated PowerShell/UAC state confirmed
before installer input. Native Windows build/test results are still pending.
The installer transport reuses the exact core helper and existing staged agent;
no alternate web bootstrap, password reset, account recreation or new public agent
listener is introduced. The prior publication review remains stale.

## Windows Guest Tools durability and ownership audit

The user requires provisioning recovery on subsequent boots, including interrupted
setup, and asked whether extraction left the guest-tools path coupled incorrectly.
A read-only trace confirms the catalog workflow selects the QEMU resolver and the
installed core VM runtime stages its adjacent core-owned helper. No sibling source
lookup was found in that active path. A stale, unused workflow-owned helper
predated extraction and lacked guest-agent support; it has been removed from the
catalog, the saved checkout synchronized with postimage guards, and the Windows
workflow archive rebuilt with the pinned CLI. Resolver documentation now describes
installed core assets rather than a nonexistent catalog-local src/core path.

Core setup now persists protected helper/agent/configuration files and installs a
SYSTEM startup task that reconciles provisioning at every Windows boot. Repair
never recreates accounts or resets passwords. Scheduled attempts are bounded;
invalid configuration or failed file protection prevents task registration. The
same helper is reachable through a newly authored Guest Tools media entrypoint;
CD AutoRun is conditional on Windows policy. Hosts with genisoimage/mkisofs/xorriso
receive a read-only CD; hosts without a CD builder retain the explicit USB entrypoint.
No historical named Dockpipe Guest Tools installer was located in the inspected
source, Git history/reflogs or archived checkout; this is not evidence that an
untracked installer never existed.

Focused shell/PowerShell regression tests and the Windows agent cross-build pass.
The core candidate was rebuilt and byte-checked against the live helper/runner,
and the workflow archive was checked for absence of the obsolete helper. A
source-verified read-only CD was attached to the existing ephemeral test guest via
its QEMU monitor through Dockpipe. Windows did not launch it automatically in this
session; after verified keyboard delivery the elevated helper reached OpenSSH
capability installation. SSH, native builds, and the user's restart test remain
pending. Rebooting Windows preserves this overlay; starting again from the original
base image with a fresh ephemeral overlay does not preserve guest setup.
Evidence: /tmp/dockpipe-extraction/windows-validation/guest-tools-decoupling-audit.json,
guest-tools-cd.json and guest-services-durable-tests.log. No commits or publication.


## Authorized staging continuation (2026-10-23)

The user authorized completing the extraction/dependency staging work and a
Dockpipe staging release after the native installed-launcher checks. Production
publication remains excluded. The public owning repositories exist but have no
published default branch yet; source publication and release input pins remain
part of the staging work.

The exact locked Go compatibility SDK was reproduced again and published at
`dockpipe-go-sdk-0.1.0-source.90db6efa41c56d31`, bound to source commit
`c604b291d463906acf0a05ef8cfe2efc071bdbaa`. Public archive and manifest downloads
match the authoritative content lock (archive SHA-256
`91b8a7490d75ec7323d792d02e5e44026dc502afbfa491ae762d01f40528cd43`).
The production latest release remained `v0.6.11`. This removes the catalog's
previous missing immutable SDK dependency; it does not qualify the remaining
native packages or a Dockpipe staging runtime release.
Evidence: `/tmp/dockpipe-extraction/full-launcher-validation/go-sdk-publication.json`.

### Publication source seal correction (2026-10-23)

The guarded extraction synchronizer now excludes generated `bin`/`build` only at
repository roots. Authored `workflows/build` trees are retained, including the
standalone compiler build and VM Guest Tools build. Saved checkouts now contain
682 PipeLang and 893 catalog authored files; all prior destination postimages
were admitted before synchronization and no user-owned drift was overwritten.

The fresh unpublished PipeLang v0.1.0 seal includes these workflows and the three
current transcript-test fixes. Its canonical Go sum is
`h1:LXd+PKL3jpgwnUSd8G4Uug6zBppqS3lGlKN1tJzi2V4=`; core `go.sum` now binds it.
Core PipeLang application consumer tests, transcript tests from the sealed module,
and the standalone bounded test workflow through pinned Dockpipe 0.6.25 pass.
The prior 680-file module/publication manifest is superseded for release use.
Evidence: `/tmp/dockpipe-extraction/publication-pipelang-module.json` and
`full-launcher-validation/publication-{module-core-tests,module-transcript-tests,pipelang-baseline}.log`.
Public source/tag/module fetching remains pending; this is a local exact-source
seal, not public Go proxy proof. The already published Go SDK has no PipeLang
module dependency and remains unchanged.

### Native bootstrap and archive contracts (2026-10-23)

The native Python interpreter policy is now shared by host `run: .py` and package
source/test hooks. Explicit interpreter overrides fail closed; Unix and authored
container environment casing remain intact. A measured Windows `Path`/`PATH`
alias defect was repaired at the inherited native environment boundary: the same
inside-workflow probe changed from three truncated paths in eight runs to eight
complete paths. Native Windows Python scripts and package hooks pass with Bash
absent; native macOS Python scripts, Unicode/space paths and explicit interpreter
selection also pass. The macOS system Python launcher is compared against the
same direct operator command, rather than incorrectly requiring its launcher and
actual framework interpreter to be the same file.

Installed archive workflows now materialize explicit typed models through the
standard package cache before catalog input discovery. The resolution workflow
therefore exposes its width, height and selected-run inputs without depending on
source checkout paths. Its compiled config and script were byte-checked against
authored source before task-store installation. The generated authored embed
inventory was regenerated and its independent root compatibility test passes;
the new boot-display helper is included. All 44 core library/CLI packages pass.

Core-owned Display SDK and store-free CLI candidate producer/publisher workflows
are prepared with immutable source/tag/asset binding. The SDK's seven-file native
consumer passes on Linux amd64, Windows amd64 and macOS arm64. The CLI bootstrap
breaks the old-CLI/new-catalog-store dependency cycle; it requires a fresh public
PipeLang fetch and an accepted source push before publication. These new CI lanes,
public artifact fetches and store lock updates are still pending; local native
proof is not a published dependency. The compatibility Go SDK remains the only
newly published dependency at this checkpoint.

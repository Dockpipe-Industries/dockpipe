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
publication remains excluded. The user explicitly approved the exact source
payloads and clarified that an explicit bounded staging request authorizes its
necessary commits, pushes, dependency publications and staging promotion without
repeated approval. Implementation stays uncommitted until that authority applies.

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
682 PipeLang and 894 catalog authored files; all prior destination postimages
were admitted before synchronization and no user-owned drift was overwritten.

The PipeLang v0.1.0 seal includes these workflows and the three
current transcript-test fixes. Its canonical Go sum is
`h1:LXd+PKL3jpgwnUSd8G4Uug6zBppqS3lGlKN1tJzi2V4=`; core `go.sum` now binds it.
Core PipeLang application consumer tests, transcript tests from the sealed module,
and the standalone bounded test workflow through pinned Dockpipe 0.6.25 pass.
The prior 680-file module/publication manifest is superseded for release use.
Evidence: `/tmp/dockpipe-extraction/publication-pipelang-module.json` and
`full-launcher-validation/publication-{module-core-tests,module-transcript-tests,pipelang-baseline}.log`.
PipeLang commit `50c17bbf019e4736a9943513931c1c10eb8d1f7d` is now public on
master/dev/staging with source tag `v0.1.0`. Independent fresh public Go proxy and
checksum-database fetching verifies this exact sum and source commit. Core tests
also pass with fresh public-only module caches and no sibling replacements.
The already published Go SDK has no PipeLang module dependency and remains unchanged.

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
provides the new execution contracts for catalog producers; the catalog also
needs an explicitly installed compiled runtime core for resolver discovery.
SDK/CLI hosted candidate publication and store lock updates remain pending;
local native proof is not a published dependency.

### Public source and native installed qualification (2026-10-23)

Catalog initial commit `99a9acf93a546a5939e112a3527f41ab536abf7f` is public on
master/dev/staging. Core source commit `564ad3d5f78f3d6ef6ac582a691eebbc78cdd56e`
is public on js/dev in PR #207 targeting dev. Public source refs were independently
read back. The PR needs integration with newer dev SDK and native qualification
changes before CI can run. That integration now uses a Dockpipe-managed worktree
on `codex/extraction-staging-merge`; the saved checkout and its unrelated ForgePipe
overview remain intact. The actual rename-aware merge identified 34 conflicts,
including 12 files retained for current host SDK and filesystem-reader contracts.
Existing worktree Git lifecycle tests (20) and path/state/Docker tests (12) pass;
the workflow cwd, workdir and source root all resolve to the isolated worktree.
The user permits managed worktrees and authorizes validated Git operations and
publications within an explicitly requested bounded staging task.
The merge completed as `f32e5ae90c168862c3221c7a831715836b97361b` through
Dockpipe. The actual merged Go suite passes with Go 1.26.10; four packages needed
reviewed host loopback access. The release suite runs 232 tests successfully with
four existing environment/platform skips. The actual merged Display SDK producer
and relocated Linux consumer also pass with GNU 11.4 and Qt 6.10.3. These local
checks do not establish merged Windows/macOS or public candidate qualification.

Complete task-local packaged Windows and macOS applications passed actual VM tab,
Canvas picker, tiling picker, native input acknowledgement, close/reopen, retained
managed QEMU ownership and explicit child cleanup. These are native installed
payload proofs, not MSI upgrade or global application replacement proofs. The
live original Windows guest also visibly shows Qt Guest Tools ready for SSH,
agent and durable startup repair through the Linux host's native Qt VM tab.
No activation, credentials, clipboard contents or reboot were needed for that
final visible check. The strict pointer corner proof retains a one-pixel upstream
edge limitation. The Windows driver currently exposes only 1280x800, so an
alternate live resolution switch remains unproven.

The initial hosted catalog Linux test found a missing compiled runtime-core
dependency; Windows hit the old 0.6.25 CLI's durable ownership check. The Linux
dependency fix, commit `5578ae4054c411c46b480c95f769cad19e659a24`, declares and
verifies the existing immutable public core archive, installs it into a fresh
task-private global root and wires both CI jobs to that setup. All 34 tooling
tests pass, as does the actual previously failing resolver lookup with the
published CLI and isolated catalog source. Hosted follow-up CI and the corrected
CLI candidate remain required. PipeLang's initial native Windows CI has the same
old-CLI ownership failure; its Linux/release-source checks and public module
identity are separate evidence. No tag was moved.

The catalog ownership cleanup is committed as
`28d38451453b94bfb275964e643b1a1198d53379`: 359 older copied Cockpit files were
retired from active discovery after exact hash/mode and historical archive
verification. Their original public history remains intact; core dev's independently
qualified SpatialPipe 0.1.3 lock is preserved. Six product tests now live beside
the owning release-setup workflow. All 34 tooling tests, six owning tests and the
actual installed-CLI IDE test hook pass. An isolated authored-source copy compiles
39 resolver/workflow archives with QEMU retained and no competing Cockpit archive.
Two measured hosted fixture defects were corrected without dropping assertions:
native Windows path comparison and inherited parent-workflow state in an IDE fixture.
Hosted follow-up CI passed the first five Linux package suites and the Windows
tooling suite. Linux VM tests now stop at a newer short socket-root SDK contract
absent from CLI 0.6.25; Windows still stops at that old CLI ownership check. The
corrected immutable CLI pin is required for both; the checks remain intact.

The owning catalog still needs actual immutable native artifact publication and
core import pins. A strict core importer extension passes 21 focused tests;
catalog native store and Guest Tools publication work is being prepared with
explicit dependency/provenance contracts. No new native store or full Dockpipe
staging release is published at this checkpoint. Production remains outside scope.

### Independent host and optional package releases (2026-10-23)

The user confirmed that launcher and host SDK ownership stays in the main Dockpipe
repository. CLI-only users do not require the desktop. The release boundary is now
being qualified with required CLI/core and desktop Qt/Browser inputs separated from
optional VM/Cockpit/catalog integration. Required host stores reject optional
payloads; Marketplace discovery keeps its independent pointer while runtime
publication uses `packages/runtime/latest.json`. Strict optional Cockpit and
Flatpak catalog checks remain in independent workflows.

The existing minimum-runtime contract is used for the VM family's new installable
slices; exact producer pins and native ABI checks remain separate. CLI candidate
versions now derive from published release history rather than the older release-line
baseline, with the original allocation retained for immutable retries. A real local
core-only store compiles and verifies. The release-tooling suite passes 261 tests
with four existing environment/platform skips. Hosted qualification, independent
package publication and final staging promotion remain outstanding. The original
saved checkout and protected ForgePipe changes remain preserved.

The source-only catalog compatibility overlay is now published from catalog commit
`30859228a77ba20a23a4b69ab81aae133d6ccfe9` after hosted build, installed default/failure
workflow checks, consumer composition, CI attestations and anonymous readback. Its
three owned workflow repairs are pinned only for independent optional-catalog
qualification. They are not native VM or Guest Tools release proof and no longer
block host artifact assembly. The Marketplace pointer was not changed.

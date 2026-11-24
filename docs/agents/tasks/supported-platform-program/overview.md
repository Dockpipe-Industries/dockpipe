# TASK-025 Dockpipe Supported-Platform Program And Qualification Matrix

## Goal

Maintain one truthful, versioned definition of where Dockpipe is supported while separate platform
tasks implement and qualify their materially different operating-system and distribution contracts.
Package publication, installation, compilation, normal operation, workload-specific qualification,
and production support are distinct claims.

## Owned Platform Tasks

| Task | Platform boundary |
| --- | --- |
| TASK-024 | Fedora and Debian |
| TASK-026 | RHEL, Rocky Linux, and AlmaLinux enterprise family |
| TASK-027 | Alpine Linux with musl, BusyBox, and apk |
| TASK-028 | Arch Linux rolling release and pacman |
| TASK-029 | Amazon Linux cloud hosts |
| TASK-030 | openSUSE Leap and Tumbleweed |
| TASK-031 | NixOS declarative hosts |
| TASK-032 | Windows |
| TASK-033 | macOS |

Ubuntu/Pop!_OS evidence remains represented in the existing implementation and relevant workload
tasks. A future dedicated task is needed only if the shared matrix identifies unowned compatibility
work beyond maintaining those existing baselines.

## Scope

The current [remote feature qualification matrix](../../../runtime/remote-platforms.md)
separates desktop operator/provider roles, native backend availability, contained
hosting restrictions, browser presentation and VM package surfaces. It records
implementation boundaries rather than promoting broad platform support. Live
Windows, Linux VM and contained-viewer parity remains pending independent proof.

- Define canonical support terms, maturity levels, release/architecture lifecycle, revocation, and
  evidence freshness.
- Maintain a matrix for artifact availability, install/upgrade, CLI, packages/resolvers/workflows,
  runtimes, diagnostics, security boundaries, and workload-specific consumers.
- Require exact releases or an explicit rolling-release policy; never infer support from package
  format, kernel family, `ID_LIKE`, cross-compilation, or container-only tests.
- Route compatibility fixes to the owning engine, workflow, package, resolver, installer, or docs
  surface and preserve Dockpipe's architecture boundaries.
- Keep public support documentation synchronized only with accepted evidence.

## Acceptance Criteria

- Every advertised platform maps to an open or completed owning task, exact baselines, claimed
  surfaces, evidence, freshness policy, and known exclusions.
- Each platform can advance, defer, or revoke independently without silently changing another.
- The matrix distinguishes native host, VM, container, cross-compile, and workload-specific proof.
- Unsupported or stale combinations fail closed and are not described as supported.
- Windows, macOS, Linux-family, cloud-vendor, rolling, musl, and declarative-host differences remain
  explicit rather than collapsed into lowest-common-denominator behavior.

## First Bounded Slice

Create the canonical matrix schema and inventory current claims/evidence only. Link each row to its
owning task and mark unknowns honestly. Do not implement compatibility fixes, add CI, run live hosts
or VMs, publish artifacts, or promote support claims in this slice.

## Experimental Synology artifacts (2026-11-02)

The user authorized DSM 7+ x64/arm64 CLI SPKs and the full GUI through a browser
desktop container, with NAS testing before any SynoCommunity submission. The
release workflow now stages both SPKs and matching GUI project bundles, requires
them during assembly, and includes them in the download catalog and checksums.
The GUI bundle reuses released Linux DEBs with a pinned multiarchitecture Ubuntu
Webtop base; it does not port Qt to DSM or change engine/package ownership.

Local proof includes cross-built CLI SPKs, extraction and a real x64 host workflow,
packaging/release contract tests, and an x64 container build from checksum-verified
0.6.35 staging DEBs. The container GUI's installation check and a real host workflow
pass. HTTPS rejects missing/incorrect credentials; after user sign-in, the streamed
Qt window, native menu interaction and embedded browser rendering were observed.
GUI-launched workflow jobs, native arm64 execution, DSM install/upgrade/removal,
NAS workload compatibility and hosted release execution remain separate evidence.
The initial CLI/portable-GUI artifacts were subsequently published as staging
`0.6.36-staging.41416259425.1.067d543e3597` from source `067d543e35974f4ce7da46a8be788533dc0b9857`.
All hosted build/assembly/publication jobs passed and four GitHub download
assets matched the published catalog and SHA-256 checksums. No community
submission was made.

The next user-directed slice adds a combined GUI + CLI SPK with a DSM launcher,
credential wizard and Package Center lifecycle through the official Container
Manager Docker Project worker. It retains the CLI-only edition and portable GUI
bundle. This combined installer remains subject to DSM 7.2.1+ hardware acceptance;
container execution and simulated package scripts do not qualify that boundary.
Local combined-package proof passed for archive admission/reproducibility, wizard
and retained-state lifecycle scripts, fail-closed missing/root identities and
authenticated HTTPS. On x64, the packaged session opened Dockpipe directly,
ran a workflow through the GUI and retained working authentication after a
container restart. ARM64 archive production passed; native ARM64 execution and
actual DSM lifecycle acceptance remain pending.
Hardware feedback confirmed CLI installation, but combined 0.6.37-1 failed to
start: DSM's Docker Project worker forced the unprivileged package UID, while
Webtop init required root to configure nginx. The packaging repair uses the
pinned streamer's native unprivileged session/HTTPS/authentication path and
process-local UID mapping. A revision-2 upgrade candidate is being qualified
against that worker constraint before further NAS acceptance. Local repair proof
passed 26 producer/lifecycle/runtime checks, authenticated page/asset/API/WebSocket
checks and a real GUI-launched workflow under an arbitrary unprivileged UID with
capabilities dropped except NET_BIND_SERVICE and no-new-privileges enabled.
See the [combined producer](../../../../workflows/package/synology-desktop-spk/README.md)
and the current
[Synology installation and acceptance guide](../../../install-synology.md),
[SPK producer](../../../../workflows/package/synology/README.md) and
[browser desktop producer](../../../../workflows/package/synology-desktop/README.md).

## 0.6 release implementation progress

The user authorized a cross-platform release pipeline. It now builds native Linux amd64/arm64, macOS Intel/Apple Silicon, and Windows amd64 artifacts plus separate native package stores. Linux formats are DEB/RPM/APK/Arch and tarball; Windows ZIP/MSI; macOS tarball. Signed APT and versioned R2 publication are implemented, with a credential-free dry-run lane. Host-workflow smoke checks run per target; local Linux package generation and APT tests are distinct from the still-pending hosted matrix and downstream distro/hardware qualification. The initial pipeline excluded Flatpak; the 2026-10-08 scope below supersedes that decision.

Local validation on 2026-10-03: Linux amd64 release build produced a checksum-verified 59-package store (42 workflows, 16 resolvers, core), CLI archive, DEB/RPM/APK/Arch packages. Native CLI and all four packaged secret-environment adapters passed outside-checkout smoke tests using fake provider processes. Seven release-tool tests passed, including real GnuPG signing and isolated APT index consumption. The runtime/package Go gate, focused race tests, Go vet, workflow validation, shell lint, and Actions syntax checks passed. Mac/Windows secret helpers cross-built only; hosted native runs remain pending.

A broad Go run failed existing PipeLang generated-compilation containment checks and timed out after ten minutes. The explicitly scoped 0.6 runtime gate excludes the compiler campaign packages; this does not establish PipeLang conformance. Language qualification remains with `tests/containedexec/` for the deferred release. Build outputs are ignored under `bin/.dockpipe`, `src/bin`, and `release/artifacts`; isolated tests/cross-builds are under `/tmp`.

Production setup on 2026-10-03: the signing key saved in **Dockpipe Packages - Production** passed an unattended GnuPG signing check with fingerprint `71B23431AB2FD718629B2696A27C6DAE150CD947`. The release setup workflow copied the two scoped R2 credentials and signing key into GitHub's `release` environment, plus the four public settings. GitHub secret names/update timestamps and public values were read back; required-reviewer protection remains enabled. The Cloudflare dashboard shows `packages.dockpipe.com` active with access enabled. A public object download, hosted native matrix, and live release publication remain unverified; no release was started by setup.

Portability cleanup on 2026-10-03 removed machine-specific defaults from VM provisioning, QEMU build scripts/specification, and package examples. QEMU now requires validated explicit source/build/final directories and passes its selected loader root into the isolated builder. The old machine-specific JSON evidence is preserved byte-for-byte under `docs/research/vm-toolchain-2026-08-07/`, outside embedded package inputs. Store exports omit the builder's private directory, and generated core/workflow/resolver descriptions no longer include absolute source paths. Full VM package tests, focused core/export/embed tests, path-admission tests, shell lint, and fresh VM workflow/resolver compilation passed. The updated QEMU recipe has not been rebuilt or live-qualified; prior hashes do not qualify new paths.


## Flatpak qualification accepted 2026-10-08

The user accepted a launcher-plus-CLI Flatpak for atomic Linux desktops, initially
motivated by Bazzite and Pipeon. Selection follows the running environment: native
Dockpipe uses native stores; Flatpak Dockpipe uses its architecture/runtime store.
No OS heuristic, toggle, automatic native fallback, or bundled Marketplace catalog.

Implemented source: shared process-platform detection, platform-labelled store
exports, catalog/pinned-install checks, a KDE 6.10 desktop builder, offline
package-owned dependency build/runtime-test recipes, and a reusable release CI
workflow. See the canonical [Flatpak build contract](../../../../workflows/build/flatpak/README.md).

Local amd64 proof: six Qt tests, actual Platform launcher/CLI/core-only inventory,
real sandbox workflow, a package with a compiled shared-library dependency, and
native/Flatpak catalog rejection tests passed. Runtime regression and 47 release
packaging tests passed. A local unsigned staging bundle was exported under `/tmp`;
no hosted dispatch, public release, installation, or user data migration occurred.

Host integration was explicitly approved on 2026-10-08. The source now includes
shared Docker/Compose/Buildx socket selection, explicit host bridges, host-compatible
remote service commands, and Pipeon release payloads that carry their container
helpers/context. First-party package profiles and pinned dependency bundles cover
all 61 optional authored artifacts, including the core-owned dotenv resolver. The
offline Platform dependency/preflight pass completed. Pipeon started containers,
MCP endpoints and its browser UI twice from the same isolated project and saved
state. The test used an existing code-server image and no provider credentials.
Fixes include user-owned nested mount targets, a private code-server umask, packaged
provider catalogs, and retry failure propagation. A compiler regression now retains
declared workflow identities when folder names differ, including during pruning.
The final local export contains 62 entries: required core plus 15 resolvers and
46 workflows. Every optional exported archive matches its qualification checksum.

The reusable Flatpak CI job is wired into release assembly, preserving the existing
publication gates and a distinct runtime-specific Marketplace store. Local Docker
and Compose bind-mount checks passed. Hosted qualification and staging publication
provide additional evidence beyond this local proof.

Publication rechecked 2026-10-10: the staging pointer selects
`0.6.3-staging.37959504785.1.4da6948415b9`; its manifest includes the 62-entry Flatpak
store and the combined launcher/CLI bundle, whose package-host URL returns HTTP 200.
The matching hosted Flatpak run succeeded. See the canonical
[publication evidence](../../../../workflows/build/flatpak/README.md#release-integration-and-proof-limits)
and [installation guide](../../../install.md#flatpak-desktop-linux-amd64).

Still open: native Bazzite, Podman/SELinux, arm64, signed/updating Flatpak remote,
production Flatpak publication and authenticated/provider or VM end-to-end parity. Package
availability checks do not prove every workflow on every host. See the canonical
build contract for current test boundaries and host prerequisites.

Synology repair follow-up: hardware upgrade feedback exposed preinst falling back
to wizard inputs after DSM removed the old package links. Revision 3 resolves
persistent home/data paths from DSM's install volume and tests the missing-link,
no-wizard upgrade sequence. NAS upgrade/startup and port cleanup remain pending.

Synology uninstall repair adds an explicit keep/delete saved-data wizard, with
keep as default. Only real uninstall plus explicit deletion purges owned data;
upgrade/default callbacks retain it. Simulated destructive-boundary checks cover
symlink isolation, idempotence and retaining data when the service port is live.
DSM-managed orphan project recovery and on-NAS uninstall cleanup remain unproven.

## Local product regression program (2026-11-12)

The user explicitly expanded acceptance to real Linux/Windows VM installation,
embedded operation, installed Flatpak GUI behavior and a deterministic full-product
regression workflow. This supersedes the original inventory-only slice for this
authorized work; it does not authorize publication or broader support claims.
The heavy program remains opt-in and local, with no CI registration.

The [product qualification workflow](../../../../workflows/qualify/product/README.md)
now inventories feature/platform/scenario assertions, pins target artifacts and
driver sources, verifies hashed observations and reports missing/manual/hardware
checks as unpassed warnings. Its target is 90% of applicable registered cases and
all critical cases; this is not a claim to detect 90% of possible defects.
Bazzite is a separate Flatpak profile. Source/component, installed product,
guided UI and human evidence remain distinct, and incomplete effectful invocations
are retained rather than automatically replayed.

The [pinned platform expansion](../../../../workflows/qualify/product/PLATFORM-MATRIX.md)
adds separate Linux distribution, Windows 11 ARM64 and WSL 2 profiles. Its
critical package-upgrade case requires a real workflow before and after an
in-place upgrade. Alpine, Rocky Linux and Amazon Linux passed the scoped
container install and upgrade cases. Desktop lanes require installed GUI
evidence. The operator-provided Windows Server 2025 lane is last.

The [installed remote feature evidence](../../../../workflows/qualify/product/REMOTE-MATRIX.md)
records the Mint 22.2 Remote Git GUI failure and the passing retest on the
upgraded private native DEB. The pinned public Browser SDK still blocks the
public desktop build. The recovered, digest-verified private SDK built the
matching Mint 0.6.44+qualification4 desktop. Its installed CLI and GUI passed
Remote Monitoring in an offline Mint VM. The fixture cleanup and backing
image digest passed, and the VM remained running for follow-on checks. The
installed Mint CLI also passed Network routing, trust, isolation and real lock
cancellation with the Cinnamon native locker held active in the guest.

The installed Fedora Workstation 44 amd64 guest passed the offline RPM
installation and in-place upgrade from the repacked 0.6.38 baseline to the
local 0.6.39 candidate. Its project workflow ran for both versions and the
project inputs survived the upgrade. The pinned 0.6.38 Flatpak also passed
CLI workflow execution, GNOME menu launch, project selection, GUI workflow
execution, and workspace and history retention after normal Quit and relaunch.
An offline test overlay exercised replacement with the private portal-fix
Flatpak. Its initial repack reused a launcher entry that Flatpak had already
wrapped, which prevented normal GNOME launch. Re-exporting the same payload
from the original desktop entry fixed launch and preserved the previous
workspace and successful runs. A new project outside the home directory ran
through the GNOME document portal and wrote its marker to the original path.
The History view stayed empty on the first portal test. The guest confirmed
that the Flatpak runtime path was a symlink and the portal mount rejected
`flock`. A private candidate resolved the grant and stored history in the
app's local state. The installed CLI and GUI each ran the portal workflow,
both CLI path spellings returned the same records, and the GUI displayed both
successes after Quit and relaunch. The private candidate's editor reported a
working-directory error while the saved workflow picker and History continued to
work. A later trace showed a detached directory entry with the same inode as the
portal grant. The CLI uses the matching canonical workspace for project-version
admission when its working directory cannot be resolved. A cold GNOME launch on
the next private candidate restored the workspace and passed editor open,
validation, save, workflow execution, guest file read-back, and History. The
released Flatpak version upgrade and remaining Fedora cases are open.

Initial local CLI proof covers private portable installation, init in a path with
spaces and a pre-existing empty workflows folder, repeated init, a real host
workflow, typed compiled-package discovery, exact archive-locator execution and
revision-checked contained file operations. Runner contract tests cover forged or
stale evidence, nonzero drivers, missing cases, process deadlines and resume.
An actual cached-image container run exposed a source/compiled-policy handoff
gap. Named execution now retains the compiler's exact archive, while declared
security without a compiled manifest is refused at container admission. The
regression independently reads back offline networking, read-only root, non-root
execution, capabilities, CPU/memory/PID limits and exact container removal. The
corrected core passed the repository-owned full Go suite (68 tested packages).

Local Linux guest observations also cover a fresh Ubuntu 24.04.4 installation
through the normal GUI, installation of the selected published CLI/desktop DEBs,
the guest GUI and typed input inside the host's embedded display, disk-only
restart retaining guest identity and installed product, and final normal guest
shutdown with independent process absence. These observations predate the final
reusable driver request and are retained as such; they are not relabeled as a
fresh deterministic framework invocation or general production support.

The registry remains broader than the implemented drivers; full-product and
cross-platform acceptance are pending. The current native VM package's declared
platforms also do not establish a qualified Flatpak VM provider artifact.

The installed private Flatpak candidate now has normal GUI evidence for authored
and packaged workflow discovery, YAML edit/save/run with independent output,
settings persistence and normal restart. The independently copied task
installation upgraded from candidate A to B with full commit/executable checks;
that run used a fresh generated profile, so old-profile migration remains a
separate check. Candidate B's producer passed 109 tests, with one explicitly
unavailable live extension check. Actual GUI evidence used recorded visual and
keyboard actions; accessibility automation remains unqualified. These local
unsigned artifacts do not establish publication, Bazzite or VM-provider support.

Windows qualification has reached a fresh Windows desktop in the normal
embedded VM tab, actual keyboard input/file readback, and successful installation
of the selected published MSI. Native CLI version, isolated init and a real
PowerShell workflow passed. Guest GUI settings save/reopen and a same-disk
media-free restart were observed for their captured cohort. The authorized
package-owned guest-services repair now passes its fresh provisioning receipt
and independent authenticated SSH and guest-agent checks. Account, configuration,
host keys, authorized keys and SSH configuration were preserved. A normal full
OS shutdown completed, with independent host confirmation that the original
QEMU and TPM processes exited. A media-free restart of that repaired guest now
passes strict SSH readback, persistent identity and service checks, plus a fresh
value entered through the ordinary embedded GUI and read back independently.
Current OS boot time, clock agreement and monotonic uptime establish a boot
after the prior full shutdown; no direct prior OS boot-time baseline was saved.
The guest shut down normally again, with independent final process and endpoint
absence. This supplementary proof is not a passing reusable Windows workflow;
that package-owned executor remains incomplete. Earlier failed display-supervisor
attempts remain preserved.
The unchanged 32 GiB disk reserve remains enforced; relocating two public
installer ISOs to another mounted drive restored headroom. Windows Fast Startup
did not establish a fresh kernel boot identity in the earlier cohort.
Package-owned repairs address
QEMU's missing vvfat support, host/guest agent port separation, and an oversized
Windows first-logon command by staging a readable bootstrap script on existing
read-only setup media. Original guest disks and uncertain failed attempts remain
preserved; these observations do not promote general Windows support.

Further local package regression now compiles real resolver/workflow archives,
runs a package-owned test, exports a store, calls a typed packaged child twice,
and checks allowed/refused cloning. This exposed and repaired generic explicit
package/nested namespace lookup of compiled archives, retaining selected archive
identity, project namespace filters, relative assets and caller project scope.
The older CLI fails the same fixture; the corrected candidate passes. Compiled
package and GUI candidates retain their exact independent source/artifact
identities rather than inheriting later fixes retroactively.

Installed Flatpak browser proof now includes generated local pages and DOM
acknowledgements, retained tabs during app navigation, an exact private download,
normal restart, retained preferences and deliberately cleared temporary browser
storage. A separate same-profile A-to-B migration retained settings byte-for-byte
and both original workflow catalogs with the original generated project mounted
read-only. Original profiles/workspaces and exact process cleanup were verified;
no Bazzite or portal qualification is inferred.

The reusable provider-owned remote workflow also passed a fresh contained
Linux-to-Darwin component case with one generated physical input chord and
independently verified Core/native/fixture/local cleanup. It records no pixels
or clipboard contents and remains separate from normal installed-GUI acceptance.
Core qualification now includes actual package lifecycle, pinned PipeLang SDK
materialization/error paths, workflow cancellation and scoped observability with
independent metric units and bounded refreshes. Missing platforms/features and
genuine hardware prerequisites remain unpassed warnings; the program has not
achieved its overall coverage target.

The local portable cohort now includes a public-CLI selected-path Git commit and
stale-review refusal, with independent verification of unrelated staged and
unstaged content and exact temporary-repository cleanup. Initialization also
has a real read-only-project failure fixture: normal actionable refusal,
unchanged project/canary bytes and no private-home fallback. The Git fixture
producer remains manual and outside CI.

Windows testing found typed workflow defaults replacing GUI selections passed
as environment entries, causing an unexpected ephemeral disk overlay. Declared
nonsecret typed GUI inputs now use authoritative CLI variable overrides;
explicit empty values survive serialization. Imported editor inputs retain
their exact-value environment transport. A fresh normal GUI boot independently
confirmed the intended persistent disk without an overlay or installation
media. Guest-services repair has since passed the independent checks described
above; complete installed-GUI remote qualification remains pending.

A generic sidebar regression reproduced a selected package row becoming clipped
after window resize, even when that same page was selected again. The shell now
keeps the current row visible after layout and reselection. Three positive Qt
behavior tests, source-layout checks and a fresh full launcher build pass. This
source change has not been deployed to the captured native or Flatpak candidates.

Input overrides and passive-notification policy now use private per-invocation
launch options. The exported context header matches its existing layout, and
legacy session-launch signatures remain available. Focused GUI tests include
effective values through the pinned CLI and context-ID reuse without options
leaking into later launches. Private native app, UI library and helpers remain
qualified as matched builds; broader SDK/release qualification is separate.

A fresh Flatpak candidate with the current input transport passes installed CLI
initialization and catalog checks, including an already-empty workflows folder
and its configured workflow root. A fresh guided run also passes ordinary GUI
edit/save/run, retained workflow catalogs and settings, and both normal quits.
An earlier attempt exceeded the normal-quit bound; that failure remains recorded
without a claimed cause or production fix. The static output-panel Stop button
does not establish whether a workflow is still running.
The package-owned experimental Flatpak VM producer also passes its contained
manager, display, factory and broker component checks. The original profile
uses software emulation without networking. Its installed package now appears
in the ordinary Flatpak GUI, refreshes inventory and opens the disk/ISO
configuration routes. Those observations were cancelled before guest launch;
embedding, input, persistence and networking remain separate checks. Neither
result promotes Flatpak VM support or replaces earlier artifact-specific evidence.

A separate experimental profile now passes component checks with contained
QEMU user networking. The actual QEMU binary opens an independently observed
loopback-only forward, rejects a duplicate forward and exits normally; the
released port can then be rebound. An Ubuntu-capable distribution also passes
normal compile, export, private TLS installation and catalog readback through
the installed Flatpak CLI. It exposes bounded CPU and memory selections,
explicit persistence, a 64 GiB sparse installation default and a pinned legacy
BIOS. No guest has booted under this profile yet. Embedded viewing, input,
independent guest SSH readback and a media-free persistent restart remain
unqualified. The disk-space guard remains enforced before guest launch.

Private macOS sender qualification also exposed an idle native-worker exit that
left its desktop catalog empty. Starting the existing registered service
restored availability without configuration changes; idle recovery remains
unqualified. A subsequent view-only lease test refused an occluded generated
fixture and verified exact cleanup. It supplies no control or lease-expiry
acceptance. Read-only diagnostics identify small Apple WindowServer surfaces at
the public cursor level, without reading titles or pixels; their individual
purpose remains unclassified. A bounded fixture policy for those system
overlays passes native identity, renderer and policy contract checks. Later
live attempts still failed before connection, first on startup readiness and
then on the existing transport freshness bound. They supply no viewing,
control or expiry credit. The owning qualification transport now passes three
actual Darwin metadata queries using a finite dispatcher and a complete seeded
identity binding, within the existing timing limits. That component proof does
not establish desktop viewing or expiry behavior. The idle worker availability
failure recurred and remains unresolved.

The reusable desktop fixture now accepts optional bounded screen dimensions;
its original default and older artifacts remain unchanged. The owning VM
qualifier consumes the versioned fixture and passes geometry as profile data.
This permits a complete owned-window capture when the app's minimum height
exceeds the former private display height. A real launching-parent lifetime
study also exposed a mapped-window startup race and a strict process-pin caller
mismatch. A corrected run observed the actual public workflow exit successfully
with its original GUI resources surviving. Its guided Quit image expired before
the action, so that run remains failed. The retained GUI subsequently quit
normally after fresh visual review, with independent process and endpoint
absence. A subsequent complete study passes public workflow exit, original GUI
survival, image-bound normal Quit and independent process/endpoint cleanup.
Read-only refresh correctly refused changed pixels and required a new review.
Unknown runtime-directory entries remain preserved with an explicit warning.
This qualifies the GUI lifetime prerequisite, not a Linux guest lifecycle or
product case; failed attempts remain recorded separately.


A subsequent installed Flatpak View-only expiry run passes its original capture
and stop deadlines, full generated nonce validation while the original GUI is
paused, unassisted Core/native expiry, normal GUI exit and exact independent
cleanup. This is supplementary evidence for its captured cohort; registration
and fresh execution through the full-product workflow remain separate work.
Native-worker process and generation changes remain an unresolved availability
observation and receive no availability credit.

The first actual Flatpak Linux lifecycle attempt stopped during window capture
after opening the VM Actions menu, before configuration or guest launch. The
frozen executor retained only the exception type, so the exact rejection cannot
be established from that run. Independent host checks found all recorded
processes and endpoints absent, and the original Ubuntu disk hash was unchanged.
Unknown private runtime entries and the failed overlay remain preserved.
Owning-package diagnostics must identify the rejected capture branch before
another lifecycle attempt; strict process and window guards remain enforced.


A real isolated document-portal prerequisite also passes: a private D-Bus,
permission store and document service produced an independently identified FUSE
mount under a fresh runtime. The document service removed that mount on normal
termination, and independent host checks confirmed exact process, socket and
mount absence. No document grant, chooser or application was started. Actual
Flatpak chooser selection, cancellation, discovery and retained access still
require separate qualification; this gives no Bazzite or SELinux credit.


Fresh execution through the manual product workflow now passes three installed
Flatpak CLI cases: installation, initialization scopes and workflow discovery.
Both fresh and initially empty workflow folders initialize idempotently, and the
generated starters run with normal defaults without implicit vault setup. The
contained target now receives a fresh private environment, preventing outer
workflow scope paths from becoming an inaccessible working directory. The prior
failed cohort remains retained; these cases do not establish full-product or
Bazzite qualification.

Later Linux GUI prelaunch runs exposed a startup wait that was not connected to
the executors, followed by ambiguous host-wide process-namespace observation.
Startup-only mapping readiness is now integrated under the original shared
budget. A retained private-proc namespace observer is being qualified separately;
unknown members still require resource retention. The last failed run created no
guest. Its GUI quit normally after explicit cleanup approval, followed by only
the four approved helper terminations and independent host process, endpoint and
mount absence. Evidence and runtime files remain preserved. Registered remote
expiry execution also remains failed despite complete independent cleanup; a
fully framed metadata-program failure must retain safe diagnostics and remain
distinct from an uncertain transport before another run.


A fresh public-workflow execution now passes the registered installed Flatpak
viewer-expiry case with all five assertions. Independent native/Core/fixture
readback and a separate host check confirm closure of the exact remote session,
absence of all eighteen recorded local process paths and four private endpoints,
and an empty recovery journal. Two helpers had original PID-only records; no
start times were reconstructed. This is one passing case in its exact cohort,
not full-product coverage or control/clipboard acceptance. Build-matched sender source and a targeted registered-job observation show
that the latest worker replacement is consistent with intentional clean session
retirement and supervisor recycling: latest exit zero, running replacement and
an unchanged failure log. A stable worker PID is not an availability contract.
The earlier idle exit-one/catalog-empty incident remains distinct and unresolved;
earlier failed runs remain unchanged.


The latest installed Flatpak VM profile visibly exposes the package's navigation
entry and resource page, and refreshes its state through the normal GUI. Its
Actions-menu run remains failed before configuration: the capture helper rejected
an XWD layout. A separate generated-window experiment reproduces a valid mixed
visual format with a nonminimal declared stride; a bounded decoder extension and
independent pixel checks are being qualified before another application run.
No guest was launched in that attempt. Exact retained helpers are reconciled,
with independent process and endpoint absence; private evidence remains intact.
The owning qualifier also passes a real syscall test for a known member exiting
between enumeration and read, requiring positive original pidfd exit, host absence
and the same complete retained process view before cleanup may proceed.

The next installed Flatpak run passes the former capture failure point and opens
the ordinary VM configuration form. Cancel and normal Quit also pass, followed
by independent host checks for all fourteen recorded process paths, four private
endpoints and owned runtime mounts. This is configuration coverage only; no
guest was launched. The decoder accepts the reproduced mixed visual only after
checking the complete identity palette and bounded declared stride. Existing
TrueColor output remains byte-identical in the generated-window experiment.

The next Linux two-boot trial uses new private storage on a drive with adequate
headroom; the original disk remains read-only and no resource floor was lowered.
Read-only sector inspection finds valid GPT checksums, a BIOS boot partition and
a GRUB marker in the MBR, but actual boot remains unproven. The owning VM
qualifier now admits the current package's exact input-monitor arguments and
schema-two display descriptor. Windows delivery, actual document-portal chooser
flows and broad product coverage remain pending. None of these heavy workflows
are registered in CI.

The registered package restart-recovery component case now passes through the
normal Dockpipe product workflow. A private loopback catalog truncates one real
same-pin reinstall response; the CLI refuses it with an actionable EOF error,
preserves the original package and bystander, and succeeds on one explicitly
reconciled retry. Exact marker counts and final store bytes exclude duplicate
fixture effects. The listener closes and recorded process paths are absent;
process observation samples descendants rather than tracing their full lifetime.
Overall coverage remains incomplete. This qualifies interrupted reinstall, not
package version updates, rollback or promotion.

The subsequent Linux attempt created the declared overlay and started the GUI,
then failed before guest launch because its namespace CLI could not identify the
selected provider. The source disk remained unchanged; the unused overlay and
failure evidence remain preserved. Exact helper reconciliation and independent
host checks confirm all recorded processes, endpoints and runtime mounts absent.
A separate real kernel test qualifies the bounded process exit/reap repair.
Fresh real-GUI context observations find the correct provider; no environment
relay change is justified. The lifecycle now publishes its initial window before
provider admission and validates the provider when normal VM navigation begins.
No normal GUI Quit or VM-management pass is claimed for the earlier failed run.

The file-workspace failure component case also passes through the normal product
workflow: stale revisions, parent escapes and a generated symlink all produce
normal actionable refusals with unchanged fixture and adjacent canary bytes.
The current product unit/contract suite passes 180 tests with no skips. These
component results do not establish broad platform or GUI coverage.

The next installed Flatpak Launch exposed a package producer error: its host
launcher reference was workflow-relative although the script belongs to the
resolver archive. QEMU never started. The producer now uses the existing logical
resolver script reference; no engine special case or duplicated workflow asset
is needed. Fresh installed Flatpak checks reproduce the old missing-script error
and reach the fixed launcher's deliberate unsupported-command guard with the new
artifacts. This proves package script resolution; it does not qualify guest boot.
The failed run remains failed, with its disk and evidence intact;
normal postfailure Quit and exact helper cleanup have independent host closure.

A subsequent real installed Flatpak run boots the declared Linux overlay and
displays the GNOME login screen inside the normal VM page. Independent native
checks confirm that the protected source disk is open read-only and the new
overlay is open for writing. The strict SSH inventory deadline then expires; a
normal shutdown attempt receives invalid structured output and the live guest
and its transports are retained. Embedding is observed, while input, restart
persistence and the overall lifecycle remain unqualified pending diagnosis.

A bounded diagnostic identifies a qualification-helper SSH quoting error: the
known-hosts path on a drive with a space is passed through OpenSSH configuration
syntax without quotes. With only that option corrected, the same live guest,
alias, pinned key and strict checking return valid inventory JSON. Original
failure evidence is retained; this diagnostic does not pass the lifecycle case.
The correction belongs in the package helper shared by Linux and Windows.

The retained Linux guest subsequently shuts down through the embedded GNOME
power menu and confirmation. Independent host observation confirms its QEMU
process is gone, providing supplementary embedding and mouse-control evidence.
The fixture's noninteractive sudo shutdown is declined; the workflow needs the
normal GUI shutdown route before a fresh two-boot qualification. The earlier
failed run is not relabelled as a pass.

A real worker-thread launch exposed another qualifier defect: its process
traversal inspected only main-thread children. The package helper now reads
bounded child lists across the owned process's threads and verifies the
original process identity. Real worker-child and disappearing-thread controls
cover the fix. The subsequent private installed Flatpak candidate passes the complete
existing-image management case: two real boots with changed kernel boot IDs,
retained hardware and overlay identity, GUI-only nonce entry with independent
SSH readbacks on both boots, retained first-boot data, normal OS shutdowns,
and normal Dockpipe Quit. All three registered assertions pass; independent
review verifies 186 evidence hashes and absence of the 25 recorded process
paths, four private endpoints and runtime mounts. The explicit test runtime
mapping remains a qualification limit. Fresh installers, Windows and broad
all-platform feature coverage remain separate pending work.

Fresh Linux installation now has package-owned admission contracts that separate ISO boot from existing-image management and declare seed, strict host-key and offline dependency requirements. The owning suite passes 284 tests without skips; execution stays blocked until the seed service and authenticated offline SSH bundle are qualified. Windows native admission also handles the existing reserved input channel alongside TPM without weakening the legacy profile. Its owning suite passes 272 tests without skips; the new normal-GUI delivery component and full Windows lifecycle still need live qualification. These are source checkpoints, not installer or platform passes.

The manual package-owned Ubuntu offline SSH producer now passes its public workflow planning and acquisition paths. Its bundle contains two exact packages selected through verified Ubuntu signed metadata, with independent member-hash review against the minimal ISO dependency baseline. This prepares an installer prerequisite; target package retention, first-boot key delivery, the private namespace seed service and actual fresh installation remain unqualified. No host package installation or heavy CI registration is involved.

The package-owned Linux seed and bounded service sources are integrated, with 22 focused controls and 306 owning tests passing without skips. A synthetic seed passes the pinned official installer and cloud-init schemas, including negative controls. Native key derivation, the namespace service and a fresh installer remain separate live prerequisites; the public installer execution gate stays blocked.

A native component subsequently derives both newly generated fixture key pairs through the package's retained read-only descriptor path. All four tools exit normally, and independent host checks confirm the controller and tool process paths absent. This qualifies positive key derivation only. The wrong-pair fixture is rejected by reference validation before derivation, so it supplies no additional native cryptographic mismatch coverage. No seed, listener, account or guest is created by this component.

The synthetic namespace service component also passes inside the selected installed Flatpak candidate. Actual permission metadata admits only its private fixture filesystem, with no shared network, sockets or devices. Two fixed routes return the expected harmless bytes, unknown-path and unsupported-method requests are refused, and the original helper closes its listener through normal termination. The fixture exits naturally; two observations of the retained private process namespace are empty, and independent host checks confirm all seven recorded process paths absent. This qualifies the helper prerequisite, without fresh-install or guest authentication credit.

The package-owned Linux installer coordinator and its manual action documentation are integrated with the execution guard retained. Review fixes preserve unresolved resource custody across repeated cancellation and wait for atomic bootstrap-result publication under the original readiness deadline. All 366 isolated source controls pass without skips; the 84-file applied cohort matches the tested snapshot. Actual installer UI, private seed delivery, media ejection, target installation and cold boot remain unqualified.

The Windows stderr TPM repair has native component proof. The latest GUI attempt passes SDK selection but fails on a controller capture command before Launch; its resources are closed. A private successor validates all 15 finite command operations, with 24 controls passing, but has no fresh GUI or guest proof. The Flatpak chooser reaches correlated frontend/backend selection, then selects the wrong generated child and returns a URI outside the independently observed app mount, with an unexpected Chromium permission. Strict guards stop before marker/init/restart. That fixture is closed; source diagnosis and a normally closed neutral-unit identity probe support fixture repairs, without portal or Bazzite acceptance credit.


## PipeLang testing and first local-host family (2026-11-16)

The [local-host product workflow](../../../../workflows/test/product/README.md)
uses an explicitly selected installed PipeLang SDK and immutable testing library.
PipeLang owns stable cases, typed assertions and setup/body/cleanup outcomes;
Dockpipe workflows and bounded native ports own execution and observation.
Exact final-SDK normal and full producer/consumer ASan/UBSan/LSan Linux runs passed
failure/recovery and graceful cancellation after readiness, with accepted SIGINT,
product cleanup and complete owned trees. No forced disposal, unreaped process or
infrastructure failure was accepted. The private general Linux CLI signal repair
has focused CLI/PTY/failure, infrastructure race and Darwin/Windows compile proof.

The same SDK passed its complete declared Linux regression with all subset
selectors unset, an explicit GNU compiler resource profile and opt-in STATIC linked
runtime. Header-only defaults and SDK pins remain unchanged; the original
header-only full-debug project gate remains unqualified. This qualifies one local
host family and one executable runtime link domain. Broader platform, VM, GUI,
property/replay/debugger and full-product work remains open. The earlier guest
campaign remains paused. No publication, deployment or release promotion occurred.

### Windows harness preparation (2026-11-16)

The same source case family now accepts platform-selected outside-proof probes;
Linux retains its original paths, assertions and budgets. Both normal and full
producer/consumer ASan/UBSan/LSan Linux regression runs pass after this change.
Windows PowerShell fixture assets preserve the original output, exit-code and LF
receipt contracts; their component checks passed under local PowerShell on Linux.
That component evidence is not native Windows acceptance.

The existing Windows VM is authorized for this harness qualification, while the
older VM campaign remains paused. Native execution awaits an identified VM
profile/connection and qualified Windows process/file adapters, console
interruption and containment. Wine and cross-compilation cannot close that gate.

The native installed-CLI lane now checks both the Windows host and PE
architectures against the requested profile. MSI runs require an installation
receipt and matching log bound to the requested installer and executable.
Portable runs verify the executable against the selected ZIP member. The
admission regressions exercise these checks without claiming a native pass.

## Typed PipeLang SDK and harness transport (2026-11-16)

The library-owned `Dockpipe.SDK` bundle is materialized by the selected CLI and
consumed through a pinned source-library descriptor. It exposes workflow/package
workflow requests, scope objects and paths, owned operation IDs, cancellation,
observation and explicit disposal. The native bridge currently supports a
single-client, dedicated Linux headless process with exclusive child reaping.
It adds no package layout or workflow policy to the engine.

The [SDK consumer qualification](../../../../workflows/test/sdk-pipelang/README.md)
and migrated local-host product family pass normal and full producer/consumer
ASan/UBSan/LSan runs against the same final private bundle and installed PipeLang
SDK. The two real journeys retain all 29 assertion statements, case IDs and
suite budgets, with five fully reaped, unforced CLI invocations. An injected
failed-launch control additionally proves that source cleanup recovers a newer
retained handle while preserving the primary failure. Package scope object/path
access is proven; installed package workflow execution remains separate.

This is local Linux qualification. Windows/macOS native transports, cancellation
during an already-running scope service, and native Windows product acceptance
remain open. The existing Windows VM is authorized; its usable launch/connection
route still needs identification. No guest, media, publication or default/pin
operation forms part of this proof.

### Attached Docker cancellation

The [product qualification runner](../../../../workflows/qualify/product/README.md)
observed cancellation of an attached workflow on a disposable, offline Linux
guest. One signal to Dockpipe produced exit 143, removed the exact container and
its observed workers, and did not replay the fixture. The consolidated plan
passed 25 of 101 cases with zero failures. Resume revalidated all 14 driver
receipts without another driver launch.

### Attached Docker restart recovery

The offline Linux guest passed the registered restart-recovery case. After the
attached CLI owner was killed, a fresh `dockpipe runs observe` process reported
the original live execution without replay. Docker independently reported one
start and exit zero. The final observer retained that execution as unverified,
and the driver removed the exact stopped container. The consolidated plan
passed 26 of 101 cases with zero failures. Resume revalidated all 15 driver
receipts without another launch.

### Synchronous host workflow failure

The default Linux portable plan now tests that a failing host script reports its
exit code and diagnostic and prevents the following step from running. The
consolidated offline Linux guest passed 27 of 101 cases with zero failures.
Resume revalidated all 16 driver receipts without another launch.

The default Linux portable plan also passed the separate synchronous host
cancellation case. A signal to the CLI reached the exact worker and its child
was reaped. Fresh observations retained one execution and the later step never
ran. The default plan passed 15 of 101 cases with zero failures.

Synchronous host restart recovery also passed. When the original CLI died, the
worker acknowledged the parent-death signal and reaped its child. A fresh
observer retained the same execution as unverified after the processes exited.
The fixture recorded one start and the next step stayed absent. The default
plan passed 16 of 101 cases with zero failures.

### Installed Browser on Pop!_OS

The registered installed Browser case passed with the distributed 0.6.38 DEBs
on Pop!_OS 22.04 amd64. The driver switched between two visible tabs, verified the
exact download, and observed temporary storage clear after normal Quit and
restart. The private X11 session and loopback server closed.

The registered installed Terminal case also passed against the same DEBs.
It selected Bash, verified the exact rendered output through the private X11
clipboard, and confirmed that normal Terminal Close reaped the original
shell and fixture child. The combined plan passed both installed cases and
revalidated both receipts without relaunch.

The installed desktop and host workflow cases passed against the same DEBs.
The workflow picker launched the installed CLI and the public observer recorded
exit zero for the original step in the selected project. A normal restart
restored that project. The combined Pop!_OS plan passed four installed cases
and revalidated all three driver receipts without relaunch.

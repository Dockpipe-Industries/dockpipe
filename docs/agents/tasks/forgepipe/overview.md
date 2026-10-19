# TASK-037 ForgePipe engine and spatial ecosystem extraction

The user authorized incremental extraction of the existing Cockpit spatial ecosystem.
Canonical scope, responsibility map, dependency analysis, provenance and acceptance are in
[the migration record](../../../architecture/forgepipe/README.md). Its file inventory records
original source hashes and include dependencies. Do not reinterpret this as inventory-only work.

Objective contract:

- objective_id: forgepipe-spatial-ecosystem-extraction-2026-10-16
- state: executing
- execution_skill: dorkpipe-objective-execution
- execution_authority: user-approved implementation and continuation; GitHub publication authorized 2026-10-16
- authorized_objective: inventory-first extraction into ForgePipe, MesaMoto, SpatialPipe,
  Spatial-Web and the existing optional Cockpit package
- done_when: independent SDK and standalone applications, shared library consumers, packaged
  integration/data-upgrade proof, public development path and CI/release instructions
- inherited_invariants: preserve behavior, saved data, user work, attribution, package/engine
  boundaries, current preview and offline operation; one authoritative implementation
- explicit_exclusions: credentials/profile mutation, paid services, Steam/multiplayer implementation,
  license-rights invention, repository visibility changes, unrelated refactoring or publication of unrelated dirty work
- checkpoint_policy: automatic_within_objective
- verification_policy: focused regressions, installed-artifact consumers, final integration proof
- handoff_policy: user_requested_only
- context_pressure_policy: warn_and_continue
- checkpoint_output_policy: quiet_success_bounded_failure
- terminal_conditions: completed, blocked, failed_verification, cancelled

Initial inventory is authored. First extraction checkpoint: independently installed native
rendering/terrain/flight SDK and Cockpit consumption. Keep status and evidence in the canonical
migration record as each seam advances. No original code retirement before equivalent proof.

ForgePipe is the engine product name; the former "Game Studios" suffix is retired.
[TASK-008](../agentic-app-ui/overview.md) records DorkPipe-owned AI features inside DockPipe;
it does not define a separate AI application or package.
It no longer uses the ForgePipe name.
[TASK-016](../guided-game-development-package/overview.md) remains the Unreal-first guided path.

Repository setup has advanced: focused AI guides and root DockPipe initialization/workflows
now exist for all seven product roots. Private DorkPipe/MCP consume a content-addressed
local Go compatibility SDK, with independent helper builds and explicit Pipeon integration
checks. Native and editor CI entrypoints use DockPipe workflows and a pinned CLI bootstrap.
See [repository execution](../../../architecture/forgepipe/repository-workflows.md) for
proof and limits. Three new products remain unconnected local scaffolds.
ForgePipe's private staging SDK is published and verified. DorkPipe 0.6.4 passed
SDK build, all 21 shell tests, both editor builds and all 26 packages on the pinned
DockPipe 0.6.17 compiler. Its six frozen assets and 26-package staging feed passed
all 59 download, size, checksum and source/provenance checks. The package now owns
the two documentation orchestration workflows. Pipeon's exact DorkPipe 0.6.4 seed
is verified; Pipeon 0.6.5 passed hosted qualification, private publication and all
eighteen package-feed object checks, with all six identities hidden.
Main source retirement and composed-store staging
qualification remain open; complete native/spatial extraction is separate work.

Release authority added by the user on 2026-10-16: publish standalone package versions
through GitHub, then qualify DockPipe staging against them; production promotion follows
successful staging/package gates. Work on `js/dev`, with promotion through `dev`, `staging`
and `master`. This authorizes the necessary runtime-owned commits, pushes, GitHub promotion
and release workflow execution within this objective. It does not approve unrelated dirty
work, new license rights or unqualified public SDK redistribution. Standalone repositories
own package candidates; one central publisher must retain unrelated shared-catalog entries.

Controlled-consumer pinning is implemented for all seven hosted CLI bootstraps,
private DorkPipe's committed Go SDK snapshot and Pipeon's two editor dependencies.
Upgrades require lock changes plus consumer qualification. The released CLI exposed
workflow compatibility problems; authored root fields and explicit CI entrypoints
were corrected and verified. DorkPipe source hooks now consume verified prepared
artifacts. The published compiler supports typed asset-only resolvers, bounded
transitive dependencies and hidden metadata. Controlled standalone consumers pin
actual qualified release assets and upgrade their version and digest together.
Mutable latest selection and silent sibling source dependencies are forbidden.

## Product composition and built-in browser (2026-10-17)

Follow the [updated product/package contract](../../../architecture/forgepipe/product-package-boundaries.md).
Complete independent SpatialPipe, MesaMoto and Spatial-Web extraction and package consumption.
SpatialPipe exposes custom planets and spatial web without DockPipe/filesystem host integrations;
Cockpit supplies those adapters. MesaMoto remains standalone with an optional universe adapter.
Normal browsing becomes a built-in top-level DockPipe desktop feature, independent of Cockpit
and spatial rendering, using the shared ordinary browser component. Headless core remains unaffected.
The existing package-loading workaround is not catalog delivery; staging install/readback gates
remain required. The local Browser implementation below does not establish package publication.

User refinement: SpatialPipe owns a Games World populated by compatible installed game packages.
MesaMoto is both a standalone game and an optional Games World contribution. Remove its integration
from Spatial-Web entirely; no game dependency, track button, launch callback or bundled content in
spatial or ordinary browsing. Future browser/game integration is deferred. See the product contract
for migration and absence-test requirements; source extraction remains unfinished.


Local implementation: Browser is now a lazy top-level desktop page with its own session and ordinary
navigation/search/download controls, tabs, annotated captures and page tools. Unpacked Manifest V3
extensions use Qt 6.10+ builds with extension support; passwords/autofill use compatible extensions,
with no built-in vault. Shared `BrowserRuntime` owns URL policy and renderer/page setup;
Spatial Web embeds the same `BrowserWidget` in Live page, preserving the current world across tab
changes until Build space. The spatial browser's Moto-track button and game-launch callback
are removed; bike runtime/content remain for extraction. Headless engine/CLI code is unchanged.
Desktop packaging inputs now include WebEngine. Standalone extraction, Games World package discovery
and hosted package availability are still pending.

Validation for this slice: Linux native and offscreen Browser fixtures pass page loading,
JavaScript, navigation/history, cancellation, saved/cancelled downloads, missing-runtime handling
and page/profile cleanup. The full-window navigation regression passes with Cockpit disabled;
spatial-web model, native spatial-web and native dirt-bike regressions pass. Desktop packaging
scripts/workflows pass syntax checks; macOS/Windows bundles and hosted catalog delivery remain
unverified. Test screenshots/logs are temporary local artifacts, not committed product assets.

Browser polish verification: five focused browser/annotation/extension/launcher/model checks pass.
Native browser checks cover PNG/PDF export and session reset; native spatial fixtures cover shared
tabs, media-controller disposal, preserved worlds/cameras, image extraction, corridors and theatre
playback. Password-manager compatibility and cross-platform extension builds remain unverified.

## Protected SDK and Cockpit staging delivery (2026-10-19)

The user explicitly approved including a protected ForgePipe SDK release and
Cockpit integration in the current Dockpipe staging build. The canonical migration
record describes the fixed public SDK inventory, private diagnostic retention,
publisher attestation and native Qt ABI gates. Current implementation advances
this delivery; hosted qualification and final staging catalog readback remain
open. This does not complete the wider standalone product extraction objective.

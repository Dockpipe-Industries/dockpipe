# Spatial Products, Packages and Built-in Browser

## User Direction — 2026-10-17

Complete the existing SpatialPipe, MesaMoto and Spatial-Web extractions into their own repositories
and consume qualified packages instead of keeping their implementation inside Cockpit. Publish the
compatible package set so normal installation works; a development package override is not delivery.
This refines TASK-037, not a separate duplicate extraction initiative.

The standalone SpatialPipe experience contains custom planets/worlds and spatial web. It must not
ship DockPipe workspaces, filesystem exploration, Git, containers, workflow execution, machine
controls or desktop-app capture as built-in capabilities. Those host integrations belong to Cockpit.
MesaMoto may be installed as an optional experience in that universe and must remain standalone.

The ordinary browser is a **built-in, top-level DockPipe desktop feature**. It must work without
Cockpit, SpatialPipe, MesaMoto or spatial rendering. This refers to the desktop application feature
set, not adding Qt/WebEngine dependencies to the headless CLI or generic execution engine.

## Product and Artifact Map

Names below describe proposed artifact roles, not new manifest fields or already-shipped packages.
Use existing package, SDK, platform and release conventions when selecting exact public identities.

The 2026-10-20 continuation prepares dependency candidates before the Dockpipe
consumer cutover. Dockpipe pins exact releases and checksums and then qualifies
staging. Production and Freeflight are outside this continuation. PipeLang remains
in Dockpipe; its extraction is deferred for a separate decision.

| Owner/product | Reusable artifact | Application composition |
| --- | --- | --- |
| ForgePipe | Versioned engine SDK/runtime: rendering, geometry, terrain and generic physics | Shared foundation; does not depend on consumer products |
| SpatialPipe | Offline world/planet/scene runtime and persistence | Standalone universe with custom planets and spatial web; no DockPipe or filesystem-provider dependency |
| Spatial-Web | Ordinary browser component plus a separate spatial-web extension/library | Browser component is used by DockPipe's built-in Browser; spatial extension is consumed by SpatialPipe and Cockpit; retain separate browser shell where required by existing scope |
| MesaMoto | Game runtime/content module, with standalone game shell | Standalone motorcycle game; optional installable universe experience through a narrow scene/session adapter |
| SpatialPipe repository: separate Cockpit package | Host adapters and DockPipe-specific workspace UI | Shared SpatialPipe/Spatial-Web runtime plus filesystem, Git, containers, workflows and explicitly enabled host capabilities |
| DockPipe desktop | Built-in Browser navigation/view and host application shell | Ships ordinary browsing by default; optional spatial packages enhance it without owning or gating the base browser |

Keep a single implementation of browser behavior, world runtime and game behavior. Source remains
with the owning repository. Distribute matching versioned binary SDK/runtime artifacts with exact
ABI/platform/Qt compatibility and integrity checks; do not use sibling source imports in releases.
The ordinary browser component must have no ForgePipe/SpatialPipe/Cockpit dependency. The spatial
extension may depend on the browser component and shared world/engine runtime; never invert that
relationship merely to reuse the current combined widget.

## SpatialPipe Versus Cockpit

User refinement (2026-10-19): the SpatialPipe repository owns Cockpit as a separate
`packages/cockpit` package alongside the standalone application. Repository ownership does
not make Cockpit part of the standalone application or Steam composition. Those builds
omit privileged Dockpipe adapters and their dependencies entirely. Dockpipe discovers and
installs compatible Cockpit artifacts through the existing package system; release builds
must not import the SpatialPipe sibling checkout. Preserve the existing package identity
and saved-world compatibility during the ownership transfer.

The Files graph keeps its lightweight 2D/orthographic 3D visualization. Remove flight,
Cockpit and app-billboard entry points from that graph toolbar and options menu. The graph
must eventually be available without loading the optional world/game runtime; its current
implementation still resides in the Cockpit library, so removing navigation entries alone
does not satisfy this dependency gate.

SpatialPipe is a world application, not a DockPipe workspace with forbidden buttons hidden. Its
composition must omit privileged host adapters. A world can persist opaque provider references for
compatibility, but loading it cannot implicitly register or grant those capabilities. Plain local
world persistence is expected; this restriction concerns filesystem exploration/host operations,
not the application's ability to save its own data.

Cockpit adds trusted host adapters over the shared world runtime. Shared libraries define neutral
scene/screen/action contracts; the host explicitly registers supported providers and permission
boundaries. Spatial websites, world packages and game modules cannot acquire host authority by
naming a provider. Preserve saved user worlds and unknown/unavailable provider IDs through fixture-
verified upgrades; do not migrate or rewrite real user data during extraction.

## Games World and Package Discovery

SpatialPipe owns a Games World that discovers installed, enabled and compatible game contributions.
MesaMoto contributes a destination through that package contract; it is not hardcoded into the
browser or required by the universe. Other SpatialPipe contributions such as scenery or tools do
not automatically become games. Define entry/exit, input/audio lifetime, save ownership and cleanup
through a narrow shared contract, reusing the standalone game's implementation.

Disabled, removed or incompatible games withdraw their destinations without deleting saves or
breaking custom planets and spatial browsing. Package discovery must not load arbitrary native
code or grant host authority merely because package metadata describes a game.

## MesaMoto and Spatial-Web Separation

User decision: remove MesaMoto integration from Spatial-Web entirely for this extraction. Spatial-Web
must have no MesaMoto dependency, bundled game content, bike-track button, game-launch callback or
MesaMoto-specific portal/action. A generic browser API is not justification to retain hidden game
wiring. The same restriction applies to DockPipe's ordinary browser.

MesaMoto has two supported entry points: its standalone game and an optional SpatialPipe Games World
package. Games World is owned by SpatialPipe, not a spatial website or browser feature. Both games
and spatial browsing may exist in the same universe without depending on each other. Possible
future Spatial-Web/MesaMoto integration is explicitly deferred and requires a separate design.

The local implementation removes `SpatialWebBrowser::showBikeTrack`, its toolbar wiring and
game-specific include. The bike runtime, content and gameplay tests remain intact. The standalone
MesaMoto extraction and Games World adapter remain pending; removing the browser entry point does
not complete either product integration.

## MesaMoto Inside the Universe

Separate game runtime/content from its standalone shell. A thin optional SpatialPipe adapter can
register a destination/portal and manage game-session entry, input/audio ownership, save state and
return-to-universe behavior. It consumes the same game runtime as standalone MesaMoto. Preserve
physics defaults, rider/race/rival behavior and assets; no copied implementation in SpatialPipe.

Do not make MesaMoto a required dependency of the universe. Missing, disabled or incompatible game
packages leave custom planets and spatial web usable. The standalone game must run without the
SpatialPipe application or DockPipe. Package integration is not a decision to distribute the paid
game through the public DockPipe catalog: existing separate installer/Steam distribution policy
remains, with the exact authorized delivery channel/entitlement chosen before paid publication.

## Built-in Ordinary Browser

Promote normal browsing into DockPipe's primary navigation, using the shared browser component and
current UI conventions. Reuse existing navigation, page rendering, profile/session behavior and URL
policy; no separate browser engine or copied browser implementation. Headless DockPipe stays free
of desktop browser dependencies. Ordinary browser startup must not load a spatial scene or require
optional spatial packages.

Extract ordinary browsing from `packages/cockpit/src/SpatialWebBrowser.cpp` rather than moving the
whole widget. It currently combines QWebEngine page/profile/navigation, `WorkspaceFlightView`,
spatial projection, headquarters and bike-track actions. Host/home/game actions belong in their
consumers; ordinary URL policy must no longer depend on a spatial document model. Retain working
browser behavior and verify resource cleanup/profile isolation. Normal web pages must not receive
DockPipe filesystem, command, package-install or approval privileges through browser integration.

## Implementation Order and Gates

1. Re-admit current checkout/source inventory and package-release state. Preserve the running test
   instance and unrelated work. Identify exact reusable boundaries before retiring any source.
2. Split shared world/scene/persistence from host providers. Build/install the SpatialPipe runtime
   independently, then consume it in Cockpit with saved-format and host-provider parity tests.
3. Extract the ordinary browser component and spatial-web extension separately. Integrate normal
   browsing as a built-in desktop feature and verify it with Cockpit/spatial packages absent.
4. Extract MesaMoto runtime/content and standalone shell; verify gameplay parity, then implement
   the optional Games World adapter without creating a reverse application dependency. Remove
   MesaMoto-specific browser dependencies, controls and callbacks; verify ordinary and spatial
   browsing build and run with MesaMoto absent.
5. Build the standalone SpatialPipe composition with custom planets and spatial web only. Prove
   that filesystem/Git/container/workflow/native-capture capabilities are absent, not merely hidden.
6. Freeze compatible artifacts, compose the package dependency set, and publish through the existing
   staging release path. Read back the catalog, bytes and checksums and exercise actual installation,
   update, disable, removal and rollback. Retire originals only after equivalent consumers pass.

Extraction, desktop/browser integration, standalone native interaction and hosted package delivery
are separate completion gates. The current local Cockpit override proves package loading/rendering,
not catalog availability or completion of these new product boundaries. At inspection the three
consumer repositories were scaffolds; do not report this plan as completed extraction.

## Local Browser Implementation (2026-10-17)

DockPipe now has a lazy top-level Browser page in its sidebar and View menu, available with Cockpit
disabled or absent. `BrowserWidget` provides tabs, address/search entry, compact navigation,
annotated captures, find, print, zoom, viewport-width previews, screenshots, session history,
clear browsing data and settings. `BrowserDownloads` owns save prompts and a session panel with
progress, pause/resume, cancel, saved-folder reveal and clear-finished controls.

Unpacked Manifest V3 extensions can be loaded and enabled for the session on Qt WebEngine 6.10+
builds with extension support enabled. There is no Chrome Web Store installer or built-in password
vault. Password storage and autofill require a compatible extension; password-manager compatibility
has not been verified. The named profile uses isolated temporary storage removed on normal shutdown;
browsing data, extensions and history do not persist between sessions. Explicitly saved files remain.

Spatial Web embeds the same `BrowserWidget` for its Live page. Tab changes detach the old media
controller before page disposal and preserve the existing world until Build space is requested.
The package owns projection, capture and media lifecycle; extension pages cannot become worlds.
`BrowserRuntime` shares URL policy, renderer lookup and page settings with the spatial browser.
The shared browser has no spatial or game dependency. The desktop links Qt WebEngine directly; the headless CLI
remains unchanged. Device/location/notification permissions are currently denied.

Desktop build and packaging inputs include WebEngine. This local source work is separate from
publishing Cockpit or extracting SpatialPipe, Spatial-Web, MesaMoto and their package dependencies.

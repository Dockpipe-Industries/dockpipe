# Cockpit

Cockpit is the optional Dockpipe desktop package for rooms, filesystem flight,
app control, spatial web browsing and procedural vehicles. It is off by default.

In **Settings → Cockpit mode**, enable the feature to find or download the native
package from your configured HTTPS catalog. The launcher delegates catalog lookup,
checksum verification and installation to the Dockpipe CLI. It enables the feature
only after the package loads successfully. Progress, cancellation and retry remain
inside Settings. No admin installation or separate browser window is requested.

Disabling Cockpit destroys its active views and stops their input, capture and
rendering. Downloaded files remain installed for reuse. Qt/Chromium code stays
mapped until the process exits, because it owns process-wide callbacks.

## Package boundary

All flight, rooms, native app streaming, browser extraction, construction rules,
vehicle code, voice assets and their tests live here. The launcher retains a small
native UI interface, settings, package acquisition and shared file-editor services.
It does not link Cockpit in its normal build. The desktop's ordinary top-level
Browser uses the shared browser runtime and Qt WebEngine independently of this package.
This is an ordinary resolver asset package; it adds no engine plugin registry.
On Linux with Qt older than 6.4, the shared launcher UI loads the small Qt OpenGL
and OpenGLWidgets support libraries at startup. Those Qt versions cannot add their compositor to an
existing window after loading a package. This compatibility dependency does not
load the Cockpit package, spatial browser view, or neural models.

The installed module is found with `dockpipe scope resolver cockpit assets-dir`.
`assets/ui` contains the native module and bounded filesystem-reader executable.
The Linux bundle includes the matching Qt libraries, renderer process,
resources and locales needed by its reader and browser child processes. Those
processes use a package-local library path. The native interface is versioned and Qt checks module ABI
compatibility. Build and distribute the package against the desktop's Qt/toolchain
baseline. Currently only Linux packaging is implemented; other platforms must not
advertise this package until their native runtime bundles are qualified.

## Development and validation

The native SDK contract is ForgePipe **0.1.0**, Qt **6.10.3**, Linux amd64.
`assets/forgepipe-sdk.lock.json` pins the public archive and release metadata by
exact size, hash and source commit. Preparation verifies both GitHub attestations
against Dockpipe's release workflow before extracting the SDK. Private or
unattested SDK candidates cannot satisfy this build contract.

From the checkout root, with GitHub CLI authentication available for attestation
verification:

```sh
bash release/packaging/desktop/install-linux-qt.sh /tmp/dockpipe-Qt
export DOCKPIPE_QT_ROOT=/tmp/dockpipe-Qt/6.10.3/gcc_64
DOCKPIPE_COCKPIT_NATIVE_TEST=1 bash packages/cockpit/tests/run.sh
```

`DOCKPIPE_WORKSPACE_PACKAGE` is an explicit development override, not a checkout
search or production fallback. For a portable package, run the package's
`assets/scripts/build.sh` with the repo-local CLI. The script obtains disposable
build/staging paths through the package-state helper, bundles the runtime and
compiles a resolver archive through the ordinary package compiler. A matching
`QtWebEngineProcess` must be installed on the builder; an explicit
`QTWEBENGINEPROCESS_PATH` can select a separately provisioned matching helper.
General source builds skip this optional native payload unless
`DOCKPIPE_BUILD_COCKPIT=1` is set. The authored profile is a template, so a normal
resolver compile cannot accidentally publish an empty Cockpit archive. Calling
`assets/scripts/build.sh` directly builds it explicitly.
`DOCKPIPE_QT_ROOT` must select Qt **6.10.3** exactly. Launcher, Cockpit and
ForgePipe expose Qt C++ types and must use the same SDK ABI. Other architectures
and Flatpak runtime SDKs require their own qualified ForgePipe artifact.
Qt 6.2's Chromium 90 cannot parse current website bundles (including class static
initializers), leaving pages with placeholder images even in Live page. Changing
the user agent does not fix that. Keep the SDK's security updates current.
The build preparer selects qmake from that SDK and rejects a mismatched version.
It supplies the verified ForgePipe prefix and Qt prefix directly to CMake.

Build dependencies include Qt Widgets, Network, OpenGLWidgets, WebEngineWidgets,
CMake, a C++17 toolchain and Python 3. X11 capture is optional.
Modern Qt's Linux XCB plugin also needs `libxcb-cursor0` at runtime. A separate
local SDK can carry that library without replacing the operating system's Qt.

On a native desktop, `DOCKPIPE_WEB_IMAGE_FIXTURE=1 cockpit-test --spatial-web`
checks modern JavaScript hydration, delayed/lazy/responsive images, placeholder
rejection and cancellation. `DOCKPIPE_WEB_IMAGE_PROBE=1` with
`DOCKPIPE_WEB_LIVE_URL` diagnoses an explicitly selected public page; it prints
bounded public image metadata and checks that source photographs reach the world.

`dockpipe package test --workdir . --only cockpit` runs the portable Python
contracts. The required `cockpit-native` CI job separately provisions the matching
toolchain, verifies the SDK and runs the native package suite. Its X11 GPU and
late-package-loading rendering checks must pass under Xvfb; exit 77 fails that
qualification. Staging publication depends on this job.
`DOCKPIPE_COCKPIT_TEST_BUILD` selects a configured integration build directory.
The package-owned preparer obtains a fresh verified SDK for each native run.
The core-only settings tests use a tiny fixture package without the heavy runtime.
The native `cockpit-package-rendering-test` links only the launcher UI and opens
its host window before loading Cockpit. Run it with `DOCKPIPE_WORKSPACE_PACKAGE`
pointing to the built assets on a real desktop to check late loading, composition,
and flight frame timings. It skips on the offscreen platform.

The new resolver must be published in the platform catalog before ordinary users
can download it. Authoring and local validation do not publish that catalog entry.
The Linux amd64 release builds Cockpit in the same modern-Qt desktop CMake tree,
runs native graphics and package-loading checks, retains private diagnostics, and
packages the stripped runtime as Cockpit 0.1.1. The final store composer replaces
exactly `resolver/cockpit` after validating its SDK-bound qualification receipt;
the current core and every unrelated pinned archive remain unchanged. Other
platform stores do not acquire this unqualified native payload.

See [construction architecture](docs/spatial-world-construction.md) for the typed
world graph, resource budgets, procedural rules and current limits.

## Site-wide Spatial Internet

**Cockpit → Web** can discover a public website progressively and turn its
semantic sections into a persistent world. **World overview** and **Map /
discovery** provide section navigation, source search, filtering reasons and
controls. Existing single-page browsing remains available by turning **Site-wide**
off. See [site worlds, controls, budgets and limits](docs/site-worlds.md).

## Furnishing

**Start / F1** opens one command menu across orbit, rooms, Git and Spatial Web.
D-pad / left stick or arrow keys choose; A / Enter selects; B / Esc returns.
Descriptions and HUD hints adapt to the input in use. The menu stops craft input
until controls return to neutral after closing. **View / P** pauses/resumes preview
scrolling outside furnishing; fullscreen is in the menu or on F11.

Inside a room, choose **Furnish**, press **F2**, or open **Start → Build / spawn an item**.
Place and edit 55 furniture, decor and screen items with surface targeting,
rotation, valid/invalid previews and automatic layout saving. Each screen can select
a running/installed app, share scene content, or show a local text/file preview.
Posters, banners and wall art accept static images through **Choose image or file**.
See [furnishing controls, inputs and limits](docs/furnishing.md).

## Arcade flight

Flight has two modes, available from the on-screen mode button or **F**:

- **Explorer** (default): soft acceleration and rapid damping into a precise
  hover when controls are released. Gentle banking and assisted leveling apply
  on planets; space flight preserves your chosen pitch and roll, with no horizon
  or pitch limit.
- **Pilot**: higher speeds, longer coasting, coordinated banking, and unrestricted
  pitch and roll. Use **Z/C** to roll and **X** to brake.

Both use WASD thrust, Q/E vertical thrust, arrows or right-drag steering, and
Shift boost. Middle-drag looks around the physical canopy without changing the
craft's heading. On a controller, Y switches modes and X brakes. In **Pilot**,
left/right bumpers roll left/right, left-stick click boosts, and right-stick click
is push-to-talk when voice commands are enabled. **Z/C** also roll left/right.
Explorer retains left-bumper boost, right-bumper talk, and D-pad left/right roll
in space. Sticks and vertical triggers remain available. Walking, driving, and
furnishing retain their own controls.
Mode selection belongs to each flight view and starts in Explorer on launch.

The sculpted canopy, recessed consoles and amber local radar follow the hull.
Instruments display actual craft velocity, flight mode, heading, altitude and
local contacts; speed uses world units per second. Flight physics is independent
of head look and scene painting. Integration is bounded to six 120 Hz substeps
per update, with swept movement samples at most eight world units apart to
prevent boost movement skipping thin colliders. Explorer stops repainting once
settled; hiding the view, editing, losing focus or disconnecting the controller
halts motion. Orbit animation and media capture retain their own lifecycles.

## Optional local world intelligence

Inside **Cockpit → Web**, **Enhance** installs a separate local inference runtime
and about 95 MB of checksum-pinned model files on first use. The default CPU lane
uses two threads; the menu offers an optional CUDA lane with approximately 2 GB of
additional downloads (several GB installed). Both stay outside the base launcher
and Cockpit native module. Native Linux currently needs Python 3 with `venv`/pip
support. Installation shows progress, is cancellable, and can be retried.

A compact semantic encoder interprets the page and each destination independently.
Its scores combine woodland, stonework, orbital structures, shelving, retail,
workshop, vehicle and carnival construction parts. These are composable features,
not generated executable code or a model-selected complete room template. Page
structure still owns navigation, original destinations and semantic collections.
This first version uses English semantic reference descriptions; multilingual and
unfamiliar subject matter can produce weak matches. It is not a general world LLM.

Products with suitable images get local foreground extraction. Images without a
useful foreground retain their original artwork. This is a cutout, not a complete
3D object or a depth reconstruction. The renderer adds cached contact shadows,
directional face lighting and distance haze without per-frame inference.

The initial world appears first. Designs arriving before movement apply while
preserving the look direction. If you have already moved or entered a vehicle,
**Enter new design** appears instead; it takes you to the new entrance when chosen.
Cutout images arrive in place. Turning Enhance off cancels work and retains the
current scene. Closing Cockpit destroys inference state and the private caches.

No page text or pixels are sent to a model service or saved in an inference cache
on disk. Only model/runtime installation uses the network. Models and runtime stay
in the application's Cockpit data directory for reuse. One child process handles
one request at a time, with at most twelve pending image requests, a 30-second
request deadline, and 30-second idle release. A bounded session cache avoids repeat
inference. Missing GPU support falls back to CPU; the world status shows the actual
backend. Download completion is published only after model SHA-256 verification.

Models: [MiniLM](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2)
(Apache-2.0) and [U²-Net small](https://github.com/xuebinqin/U-2-Net)
(Apache-2.0), using the ONNX export distributed by
[rembg](https://github.com/danielgatis/rembg). Exact artifacts and hashes are in
`neural/models.json`. The optional Python/ONNX dependencies retain their upstream
licenses; see their installed distribution metadata. No PyTorch or generative
mesh model is required.

For an isolated development runtime, run `python3 packages/cockpit/neural/install.py
--destination /tmp/cockpit-neural` (add `--gpu` for CUDA), then set
`DOCKPIPE_COCKPIT_NEURAL_ROOT=/tmp/cockpit-neural` for the launcher or tests.
`cockpit-test --spatial-intelligence` always exercises construction and malformed
helper responses; that override additionally enables real semantic/cutout tests.
`DOCKPIPE_TEST_NEURAL_GPU=1` requires the real CUDA backend in that test.
`DOCKPIPE_WEB_TEST_INTELLIGENCE=1 cockpit-test --spatial-web` exercises the native
browser, deferred design application, continuous movement and cancellation.

## Spatial operations

**Containers** opens an industrial district from the existing monitoring snapshot.
Containers are factories, networks are switchyards, and named volumes are storage
depots. Cyan cables represent actual network-to-container attachments; amber
cables represent container-to-volume mounts. A container may connect to several
networks. Moving pulses indicate a running container, not observed network traffic
or storage throughput. Compose dependencies and bind mounts remain available in
the original controls; they are not drawn as network cables.

**Executions** opens a mission campus showing the existing managed workflow,
task, process and runtime relationships. Select a resource's sign to open its
existing inspector and actions. **Machines** opens a system of machine planets.
Approach a planet, then walk into or select its laboratory entrance. Inside, a
console customizes the machine's android name, finish and chassis; the other
console opens machine controls. Pairing/access labels are not health probes.
Android appearances are local preferences, bounded to 256 profiles; they never
change remote machine configuration. Settings is no longer a Cockpit planet;
preferences remain under the application's Settings menu.

Each operations sector contains twelve primary resources and at most eight
connected neighbors, with at most 64 visible cables. Sector controls cover the
remaining inventory; the status reports visible resource/link counts. The source
snapshot itself retains the host canvas limits of 1,024 nodes and 4,096 links and
its discovery/truncation notices. No new discovery subprocesses are created by
the renderer. It samples existing snapshots once a second, defers reconstruction
while moving or using controls, preserves position on refresh when unblocked,
and stops polling when hidden or outside operations. Pulses pause with hidden
views and open controls. The versioned native workspace interface is now 2.0;
rebuild the launcher and package together. Old modules are rejected rather than
loaded with an incompatible interface.

## Switching between the website and its world

**Live page** opens the real website and changes to **Return to space**. Returning
reuses the existing room, images, camera and vehicle state immediately. It does
not extract the page, regenerate geometry or reload the website. Navigation and
browser history inside Live page leave that retained world intact. **Build space**
is the separate action for replacing it with a projection of the current page.

Live page shares the ordinary Browser's persistent settings, downloads, extensions
and page tools. Camera, microphone, location and notifications support Ask, Allow
or Block for HTTP(S), with exact-origin overrides. Ask prompts appear only on the
selected visible live page, and remembering the decision is optional. Unsupported
permissions are denied. Cookies and history remain session-only; saved settings
and remembered site policies are separate. These controls do not enable AI access.

In a theatre, Live page hands playback to the website without pausing or seeking.
Return to space streams the current page's video back onto the existing centre
screen, respecting playback and pause changes made through the website. Returning
does not force autoplay. A page without a usable video reaches the existing
bounded media timeout and leaves the room available. Leaving Web for Cockpit
still stops capture and freezes the browser through the existing lifecycle.

## Git Planet

Review the active project’s saved working changes in a local spatial gauntlet,
with separate staged/unstaged comparisons, selectable code, revision-bound
progress, file staging and explicit index commits. See [Git review](docs/git-review.md)
for controls, bounds, validation and the current provider limitations.

## Dirt-bike proving ground

Open **Cockpit → Web → Moto track** to ride a RedBud 2024 map study with rolling
terrain, switchbacks, jumps, persistent tyre ruts and toggleable rain. The layout
follows the official map; elevations are authored approximations. Shared soil and
surface-water systems affect grip, suspension contacts and puddle drag. Ride with a
visible animated rider in chase view or switch to helmet view. The bike includes
engine/tyre/landing sounds, suspension and arcade traction. RT accelerates, LT
brakes, X slides the rear wheel; keyboard controls and recovery remain available.
Pull back with throttle for a wheelie, or start a three-lap race against two rival
riders. Press T for rain. See [controls, physics and Spatial Web reuse](docs/dirt-bike.md)
and [reusable terrain contracts](docs/terrain-system.md).

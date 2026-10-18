# Moto overnight development — 2026-10-16

## Objective contract

- objective_id: cockpit-moto-overnight-2026-10-16
- state: completed
- execution_skill: dorkpipe-objective-execution; dorkpipe-package-authoring
- execution_authority: user's explicit overnight implementation request
- authorized_objective: research MX Bikes and improve DockPipe Moto physics,
  rider/bike/track fidelity and rendering efficiency toward the supplied reference.
- done_when: retain coherent, tested improvements; finish qualification and report
  by 09:00 America/Toronto on 2026-10-16 (13:00 UTC).
- checkpoint_policy: automatic within this objective; no additional permission
  needed for reversible in-scope code, public research/assets and local validation.
- verification_policy: deterministic handling/landing fixtures, controller and
  scene regression coverage, native-resolution hidden GPU measurements and images.
- inherited_invariants: preserve current grip/backflip fixes, native resolution,
  keyboard/controller unity, current Qt scene/navigation, package isolation,
  bounded caches/work, software fallback, user-owned dirty checkout and preview.
- explicit_exclusions: commits/push/publication, driver/system configuration,
  paid assets/services, unrelated core changes, new tasks/worktrees/subagents,
  automatic preview restart or extracting proprietary MX Bikes assets/code.
- handoff_policy: user-requested only. Keep durable evidence here across compaction.
- terminal_conditions: cutoff, completed validation, explicit stop, or a blocker
  requiring user input. Begin final qualification by 08:30; stop at 09:00.

## Admitted baseline

Saved checkout /home/jamie/source/dockpipe, branch js/dev,
HEAD c604b291d463906acf0a05ef8cfe2efc071bdbaa. No staged changes. Large protected
worktree includes the entire untracked packages/cockpit migration and unrelated
launcher/engine changes. Initial inventory: /tmp/cockpit-overnight-start-status.txt.
Current preview PID 438567, dockpipe-preview-newest.service, must remain running.
Build directory: src/app/tooling/dockpipe-launcher/build/modern-web, Qt 6.10.3.
Build with -j2; do not edit sources while compiling.

Last qualified build: eight CTests pass; full-throttle turn has 5.27 cm/s peak
lateral velocity; authored tabletop backflip lands; bad rotations still crash.
4K RTX 3080 hidden spin render median/p95/max 10.26/13.76/15.01 ms, bike update
p95 0.49 ms. Broad track overview still roughly 25 ms CPU draw submission.
64 MiB material cache avoids observed 447 repeated creations for a 46.15 MiB
working set. Existing renderer uses QPainter projected polygons/BSP on a Qt
OpenGL viewport, plus native-resolution default and explicit reduced fallback.

## Working sequence

1. Research primary MX Bikes features/physics/setup and NVIDIA requirements.
2. Audit and extend the existing rendering seam for real GPU depth/lighting,
   textured terrain and richer geometry without a competing scene engine.
3. Improve contact/tyre/suspension/rider behaviour with deterministic fixtures.
4. Refine track, rider/bike and environment using owned/licensed assets.
5. Qualify controller, crash/flip, scene compatibility, native GPU performance and
   visual fidelity; keep accepted/rejected measurements, document limitations.

## Research

- [MX Bikes official overview](https://www.mx-bikes.com/): motorcycle dynamics,
  setup, tyre model, chassis flex, deforming/wet terrain, modding and telemetry.
  These are reference capabilities, not claims about DockPipe's implementation.
- [NVIDIA Streamline integration guide](https://github.com/NVIDIA-RTX/Streamline/blob/main/docs/ProgrammingGuide.md):
  Streamline's guide specifies Windows and D3D/Vulkan integration. Separate
  [NVIDIA Vulkan support](https://developer.nvidia.com/vulkan) and the
  [DLSS guide](https://raw.githubusercontent.com/NVIDIA/DLSS/main/doc/DLSS_Programming_Guide_Release.pdf)
  confirm DLSS also has a Linux Vulkan path. Our current OpenGL renderer lacks
  the required API integration and temporal motion/depth/jitter inputs. Do not
  label a resize filter DLSS or claim DLSS is universally Windows-only.

## Continuation and evidence

Heartbeat: moto-overnight-development-until-9-am, every 15 minutes, bounded through
13:00 UTC. It resumes only this work; disable at cutoff/completion. Check clock at
checkpoint boundaries. No goal tool or successor task was created.

### Checkpoint 1 — native geometry, 01:40 Toronto

Extended the existing Qt viewport with package-owned SpatialGpuRenderer and
WorkspaceFlightGpu. Existing scene/interaction ownership and painter fallback
remain. Native pass uses depth-tested triangles, shared mipmapped textures,
anisotropic filtering, bounded batches and native-resolution render targets.
The ridden bike's octagonal contact shadow initially caused fallback; fixed
convex-polygon admission and made the hardware fixture assert native rendering
on every capture, including riding/crash frames.

- Build: /tmp/cockpit-native-riding-build.log.
- Eight focused CTests pass: /tmp/cockpit-native-first-tests.log (38.36 seconds).
- Native 4K RTX 3080 spin median/p95/max 2.73/13.06/14.40 ms; update p95 0.46 ms.
  Broad view draw submission 1.41 ms. /tmp/cockpit-native-riding.log and PNGs.
  This is hidden-FBO evidence, not mapped-window gameplay FPS.
- Source compatibility and native path both retained; no preview restart.

### Checkpoint 2 — lighting in progress, 01:51 Toronto

Added smooth per-vertex normals/unlit albedo to bike model data, transformed
with the articulated pose. Added a bounded 2048-square local directional shadow
map and hemisphere/specular lighting; one geometry upload is reused by shadow
and colour passes. Shader syntax error was caught and corrected. Native alpha
test found incorrect destination-alpha accumulation; separate alpha blending
now preserves the Qt compositor's opaque destination correctly.

- New spatial-gpu-test passes on the actual GPU: depth order, cutouts, painter
  restoration, octagonal transparency, invalid/over-budget input, resize and
  explicit cleanup. /tmp/cockpit-gpu-unit.log.
- Native visual/performance test running: /tmp/cockpit-shadow-good.log,
  exec session 8638. Inspect resulting images before accepting lighting.
- Build: /tmp/cockpit-gpu-alpha-build.log.
- Next: finish lifecycle/context-switch tests, inspect shadows and performance;
  improve rider/bike/track presentation and tyre/steering response with fixtures.
  Physics remained the admitted baseline at this checkpoint.

### Checkpoint 3 — smooth model and contact response, 02:20 Toronto

Native depth/cutout/compositor/resize tests pass, including releasing resources
after doneCurrent and recreating them. /tmp/cockpit-gpu-lifecycle.log. Richer
procedural rider/bike now uses 1885 faces (under 2048), smooth normals, curved
helmet peak, denser tyre/spoke/rotor geometry, rounded joints and charcoal kit.
Local directional shadows and derivative material relief are active on the GPU.

Added transient front/rear contact-patch slip state. Rejected early brush-force
clipping because it deprived the powered rear tyre of cornering force; also
rejected a changed friction allocation that weakened the braking contract. The
accepted version feeds unsaturated elastic demands into the existing combined
friction envelope. Ordinary powered turns remain upright and intentional rear
brake slides still fall. No ground/yaw test thresholds were weakened.

- Full physics test passes: /tmp/cockpit-tyre-relaxed-test.log. Heading .227 rad
  and lean -.532 after the steering fixture; full turn survives; authored jumps,
  clean complete backflip and bad-landing rejection pass.
- Native rich-model spin median/p95/max 8.04/9.73/10.38 ms at 4K on RTX 3080;
  bike update p95 .60 ms, broad submission 1.67 ms. Hidden FBO, not gameplay FPS.
  /tmp/cockpit-rich-contact.log and PNGs. Earlier simpler native pass was faster
  (3.21/5.70/12.44 ms) but lacked the current detail/relief.
- Grass colour in that capture was too yellow and is not accepted as final.
  Added CC0 leafy grass and cloudy-sky source assets with provenance. Sky/grass
  integration is building in /tmp/cockpit-sky-build.log (exec 78758).
- Next: inspect actual sky/grass captures and qualify the new sky path; implement
  bounded wheel angular-momentum pitch response in air, then track refinement.
  No preview restart, commit, push or system changes.

Additional primary research: [Project Chrono tyre models](https://api.chrono.projectchrono.org/wheeled_tire.html)
describes contact-patch slip state equations and combined slip, with explicit
limitations for off-road/deformable terrain. Use these concepts as references,
not a claim of calibrated Pacejka/Fiala or a reason to add a large dependency.

### Checkpoint 4 — airborne control, assets and shadow quality, 03:04 Toronto

Added bounded wheel angular momentum: airborne throttle raises the nose and a
rear-brake tap lowers it, without adding forward thrust. Front/rear wheel angles
now come from the 120 Hz solver; fork sliders and the rear swingarm articulate.
The fixture forks one takeoff state into coast/throttle/brake cases and verifies
pitch separation, rear-wheel lock and continued front-wheel rotation. Existing
powered turns, clean backflips and bad-landing rejection remain passing.

Imported nine selected pieces from Innerscene's CC0 unbranded dirt motorcycle,
reduced offline from a 108,138-triangle source to 1,260 triangles. Adapted the
fender, tank and panels to the existing motocross wheelbase and riding stance.
The complete ridden model is 3,167 faces, below the explicit 4,096 cap. There is
no GLB loader or simplification dependency at runtime. Provenance/reproduction
are in assets/models/moto/SOURCE.md and tools/import_moto.py. Added photographed
CC0 grass and sky; sky decode is scaled before allocation to 2048 by 1024.

Ordinary furnishing objects and placement ghosts now remain on the native pass.
Rooms with live screen streams still use the existing compatible painter path.
Slope-scaled shadow caster bias removes the observed diagonal self-shadow acne
on the tank/tail. Native renderer tests cover sky orientation, invalid camera
and normals, plus the previous depth/alpha/resize/context cleanup cases.

- Eight focused CTests: /tmp/cockpit-bodywork-regressions.log, 8/8, 39 seconds.
- Latest full native bike fixture: /tmp/cockpit-shadow-slope.log; 4K RTX 3080
  hidden spin median/p95/max 3.93/5.72/7.60 ms, bike update p95 .96 ms.
  Broad view submission 1.82 ms. Hidden FBO evidence, not gameplay FPS.
- Native unit test: /tmp/cockpit-projection-unit.log, passes.
- Inspected /tmp/cockpit-shadow-slope-furnishing.png: native preview and corrected
  shadows present. Terrain remains too orange/yellow; not final visual quality.
- Current next slice: two-bone rider arm pose keeps hands on bars under steering,
  lean and weight shifts. Building /tmp/cockpit-rider-ik-build.log, session 77795.
  New pose coverage checks shoulder/elbow/hand continuity and fixed bone lengths
  across the full input envelope. Not yet qualified at this checkpoint.
- Preview preserved. No commit, push, restart, driver change or new task.

### Checkpoint 5 — rider joints, banked turns and antialiasing, 03:20 Toronto

DirtBikeRiderPose solves each two-bone arm once per vehicle update. One shared
torso adjustment prevents overextension; hands stay on the steered bars. Tests
check both joint endpoints, fixed bone lengths, torso continuity, bounded poses
and invalid input across steering/lean/weight/compression extremes. CPU and GPU
riding fixtures pass. /tmp/cockpit-rider-ik-test.log and cockpit-rider-ik.log.

SpatialRideSurface now supports bounded signed curved profiles and banking.
Visible triangles supply exact wheel/walking height, including mirrored/rotated
and coarse four-strip arcs. Foliage admission uses the actual curved footprint;
world construction rejects excessively steep central banking. Added two physical
berms to the course and aligned its ribbon. Ground-sideways slope influences
lateral gravity, supported grip, stopped-bike balance and landing rejection.
The controlled turn fixture reaches 2.905 radians with 17.12 cm maximum radius
error, without crashing. Existing flat handling/braking/jump/backflip tests keep
their prior results. /tmp/cockpit-berm-drive-test.log. Neutralized the excessively
orange/yellow track palette. Native banked capture: cockpit-banked-track-berm.png.

Native rendering now supports real 2x/4x MSAA with a 256 MiB colour/depth budget,
source/target sample compatibility and no resolution reduction. Actual GPU edge
coverage and painter tests pass: /tmp/cockpit-msaa-unit.log. Latest 4K spin:
3.96/4.77/8.73 ms median/p95/max, update p95 .89 ms; broad submission 2.61 ms.
/tmp/cockpit-native-msaa.log. These are hidden-FBO timings, not gameplay FPS.

The berm visual review exposed a finite ground edge against the sky. Extending
natural ground into the distance (same physics plot), matching haze colour, and
adding two-context lifecycle/multisampled-source tests are currently building:
/tmp/cockpit-horizon-build.log, session 37957. No source edits until it completes.
Next: run native unit and eight focused regressions, inspect horizon/berm images;
then continue visual/physics improvements with bounded resources. A fuller
material/lighting pass and actual 3D foliage are candidates, not implemented.
No preview restart, commit, push, publication or driver changes.

### Checkpoint 6 — qualified woodland build, 03:30 Toronto

Accepted the horizon fix: natural terrain continues beyond the playable plot,
while the painter fallback keeps its original inner texture-patch density.
The Moto haze colour now matches the photographed sky more closely. Shader
compile/link failures are remembered per context instead of retrying every frame;
explicit release or a new context allows initialization again. Camera basis
validation rejects non-orthogonal axes.

The native unit now also exercises a multisampled source framebuffer and two
independent contexts, including reacquiring the old context for cleanup and
restoring the new one. /tmp/cockpit-horizon-unit.log passes with no GL errors.
Eight focused regressions pass after the latest changes:
/tmp/cockpit-woodland-regressions.log, 8/8, 39 seconds.

The playable track plot is 320 by 380 metres, leaving space around the existing
riding loop for woodland. All 72 authored tree candidates are admitted with the
same collision/placement rules; the shared scene cap is now 96. One shared image
and native texture batch supplies every card. No image is copied per tree.
These remain camera-facing cards, not full 3D trees. Inspected the new berm view:
/tmp/cockpit-woodland-berm.png. It is visibly fuller; repeated silhouettes and
procedural rider/body detail remain limitations against the requested reference.

Latest full GPU fixture: /tmp/cockpit-woodland.log. 4K RTX 3080 hidden spin
median/p95/max 4.10/5.87/9.31 ms, bike-update p95 .94 ms; broad-view submission
2.73 ms. This is hidden-FBO evidence, not mapped-window gameplay FPS. Earlier
runs vary with host/GPU load; do not present one run as a guaranteed frame rate.
The complete authored handling, braking, airborne wheel reaction, banked turn,
backflip/crash, furnishing-preview and native-renderer assertions pass.

No running builds or tests remain. Latest build includes all changes. Preview
readback at 03:28: dockpipe-preview-newest.service active/running, original
PID 438567. It has not been restarted. Status receipt:
/tmp/cockpit-overnight-checkpoint-status.txt. Authored source whitespace/conflict
scan passed; Git diff cannot cover the untracked package by itself.

Continuation remains ACTIVE until 09:00 Toronto, verified from its automation
configuration. Continue this objective; begin final qualification by 08:30.
Good next seams: a bounded richer material/lighting pass (real normal/roughness
maps with existing GPU texture ownership), or better rider/foliage geometry if
it can be licensed, reduced and integrated without large runtime dependencies.
Keep current riding feel and test contracts. Do not add unrelated features just
to fill the remaining time. Requalify changed areas and inspect actual pixels.

Asset research not adopted: Poly Haven pine_tree_01 advertises 17 million
triangles; no model was downloaded. Quaternius Universal Base Characters has
CC0 game-ready humanoids averaging 13k triangles, verified on the author's page,
https://quaternius.com/packs/universalbasecharacters.html. Its standard download
uses the public Itch free-download flow. No character files were downloaded and
no purchase/account access occurred. The public HTML receipts are in
/tmp/cockpit-character-source.html and cockpit-character-download.html. Do not
claim these models are integrated. Existing bodywork provenance is complete.

### Checkpoint 7 — terrain normal/roughness materials, 03:46 Toronto

Fetched matching 1K OpenGL normal and roughness maps for the existing Poly Haven
brown_mud_dry, rock_01 and leafy_grass materials. All six unchanged CC0 JPEGs
were checked against the public asset API sizes and MD5 values before writing;
SHA-256, authors and original URLs are in resources/materials/README.md. Added
5,089,427 bundled bytes, no runtime download or new dependency.

SpatialSurfaceTextures packs each normal RGB/roughness alpha pair once into a
shared linear 1K tile. Three slots use 12 MiB CPU memory. The native renderer
uploads them through its existing 64 MiB texture cache; no budget increase.
Surface normals use a derivative tangent frame, including top-first QImage
orientation. Roughness controls dielectric sun highlights; filtered normal
variance softens distant specular detail. Shadow bias stays tied to geometric
normals. Rubber/fabric retain procedural relief. This is richer local surface
lighting, not HDR/PBR-engine parity, displacement or a collision change.

Native tests now verify shared material data, varying roughness, actual lighting
variation beyond albedo, opaque terrain despite roughness alpha, and GL cleanup.
/tmp/cockpit-material-detail-unit.log passes. Build:
/tmp/cockpit-material-detail-build.log. Full GPU fixture passes:
/tmp/cockpit-material-detail.log. 4K RTX 3080 hidden spin median/p95/max
4.15/5.49/8.39 ms; bike update p95 .93 ms; broad draw submission 2.25 ms.
These remain hidden-FBO timings, not live gameplay FPS. All handling, braking,
air-control, banked-turn and landing results match the prior qualified build.

Inspected /tmp/cockpit-material-detail-rider.png and
/tmp/cockpit-material-detail-berm.png: dirt relief is more apparent, grass remains
opaque, and native geometry/shadows are intact. Repeated tree cards and the
stylized rider remain visible fidelity limits. Eight focused CTests pass in
39.01 seconds: /tmp/cockpit-material-detail-regressions.log. Authored source
whitespace/conflict scan passes. No builds/tests are still running.

Preview readback at 03:45 remains active/running with original PID 438567. No
restart, commit, push, publication, system change or new task occurred. Continue
the active overnight objective until the existing cutoff. Next useful seam is
better rider/foliage form, with licensed bounded geometry and existing animation
ownership, or a focused physics improvement backed by a meaningful fixture.
Avoid adding unrelated features or calling the current visuals AAA/MX Bikes
parity. Start final qualification by 08:30 Toronto and stop at 09:00.

### Checkpoint 8 — articulated legs and shaped rider, 03:58 Toronto

The old legs used the torso's height-weighted displacement, stretching the limb
geometry during weight shifts. Added thigh/shin/boot parts and extended the
bounded rider pose to four two-bone limbs. Boots stay fixed at the footpegs,
hips follow the shared torso, and knees bend forward with rigid bone lengths.
The same bounded reach projection prevents arms or legs from overextending.
The pose is evaluated only for an actively ridden, uncrashed bike; parked
vehicles and crashed-body animation do not pay for unused IK.

Replaced the oval torso/flat number-sign geometry with a shaped jersey profile
and surface-following colour panels. Sleeves and legs taper, knee and boot
surfaces are smoother, and boot soles now align with the pegs. Torso normals
use the inverse-transpose of its actual weight-shift deformation. Ridden model
count is 3,308 faces, below the existing 4,096 cap. These are original procedural
model refinements, not a newly licensed human model or a claim of AAA fidelity.

Expanded pose tests across steering +/-0.6 radians, lean, weight and suspension
travel: all four endpoints/joints remain attached, bone lengths remain constant,
knees bend forward, normals stay perpendicular to transformed torso tangents,
and nonfinite controls are bounded. CPU fixture passes:
/tmp/cockpit-rider-legs-cpu.log. Native fixture and inspected images:
/tmp/cockpit-rider-legs.log, cockpit-rider-legs-rider.png and
cockpit-rider-legs-corner.png. Handling/landing outputs are unchanged. Hidden
4K RTX 3080 spin measured median/p95/max 4.38/5.41/8.87 ms, update p95 1.05 ms;
broad submission 2.44 ms. This measurement preceded the final unused-IK skip;
it is hidden-FBO evidence, not live gameplay FPS.

Eight focused regressions pass after the final functional change, 39.05 seconds:
/tmp/cockpit-rider-legs-regressions.log. Final build including a formatting-only
cleanup passes: /tmp/cockpit-rider-final-build.log. Authored whitespace/conflict
scan passes. No builds/tests remain running. The active preview was not touched,
and no commit, push, publication, system change or task creation occurred.

Continue meaningful fidelity/physics work within the existing deadline. Richer
foliage form and less uniform track surroundings remain larger visual gaps than
adding more polygons to the rider. Existing tree cards still rotate toward the
camera; do not label them actual 3D trees. Keep assets licensed and bounded,
preserve the collision/placement rules, and qualify actual rendered pixels.

### Checkpoint 9 — shared 3D foliage and GPU instancing, 04:24 Toronto

Extended the existing native renderer with bounded instanced batches and explicit
per-face UVs. One base mesh is flattened/uploaded per frame, then a 36-byte
transform/shade record per tree. This is actual glDrawArraysInstanced use in
colour and shadow passes, not CPU geometry duplication; it does not yet cache a
persistent static vertex buffer. Bounds: 96 aggregate instances, eight groups,
8,192 vertices per base mesh, existing 196,608 aggregate staging vertices and
64 MiB texture cache. Invalid transforms/UVs and general transparent instances
are rejected before rendering. Opaque and cutout instances retain depth tests.
Instance buffers follow existing context cleanup/recreation ownership.

Added package-owned SpatialGpuFoliage: one shared 1,256-quad / 2,512-triangle pine
with tapered 3D trunk/branches and fixed intersecting needle sheets. Deterministic
spiral growth avoids horizontal shelves. Moto reuses the same 72 admitted tree
placements and canopy envelope, with stable heading/shade variation. The lower
trunk stays straight and slim; existing movement colliders remain unchanged.
Other worlds and the painter fallback retain their previous tree cards.

Only three small CC0 textures from Poly Haven Pine Tree 01 were downloaded,
1,588,125 bytes total, by Rico Cilliers and Rob Tuytel. The 17-million-triangle
source model / approximately 949 MB mesh was rejected as inappropriate and was
not downloaded. API sizes/MD5 and local SHA-256 verified; provenance and URLs are
in resources/materials/README.md. Runtime decodes the 1K bark once and extracts a
220x440 needle tile once. No network or neural work occurs while riding.

Visual inspection caught opaque neighboring UV islands producing triangular
fringes. A bounded border-component removal now isolates the needle silhouette
without discarding its antialiasing. Tests verify transparent borders and useful
interior coverage. First shelving/fringe captures are retained in /tmp/cockpit-pine*
and /tmp/cockpit-pine-refined*; accepted images are /tmp/cockpit-pine-final-*.png.
Inspected final corner view: clean needle edges, depth and ground shadows, more
volumetric surroundings. Trees/rider still look stylized; no AAA parity claim.

Native test adds shared-mesh identity, canopy bounds, image sharing, translation,
scale, shade, yaw/mask orientation, explicit UVs, occlusion, aggregate/per-mesh
limits, invalid transforms and context recreation with instances. Passes:
/tmp/cockpit-pine-mask-unit.log. Build: /tmp/cockpit-pine-mask-build.log.
Only the initial broad rebuild emitted an existing third-party Qt deprecation
warning; no authored warnings/errors. Eight focused CTests pass in 39.15 seconds:
/tmp/cockpit-pine-regressions.log. Whitespace/conflict scan passes.

Full native fixture passes with unchanged handling/jump/landing outputs:
/tmp/cockpit-pine-final.log. Hidden 4K RTX 3080 spin median/p95/max
4.70/5.85/8.35 ms, update p95 1.02 ms; broad submission 3.15 ms. Prior rider
checkpoint was 4.38/5.41/8.87 ms. This bounded scene cost is accepted for richer
geometry, but hidden FBO timings do not establish mapped-window gameplay FPS.
The spin benchmark deliberately reaches a crash; deterministic normal-turn and
clean-landing assertions still pass. Source docs updated in docs/dirt-bike.md.

Original preview remains active/running, PID 438567, verified at 04:24. No
restart, commit, push, publication, system change or task creation. No tests or
builds remain running. Continue the existing overnight objective; next useful
checkpoint is a focused suspension/terrain contact or steering refinement with
meaningful physical fixtures, rather than more decorative geometry. Keep the
12:30 UTC final-qualification start and 13:00 UTC hard cutoff.

### Checkpoint 10 — suspension-coupled tyre support, 04:34 Toronto

Found that tyre grip only used spring force as a contact boolean, then allocated
fixed static weight using an independent quasi-static transfer estimate. A
nearly unsupported wheel therefore retained nearly full grip. DirtBikeDynamics
now uses each actual suspension force for its friction capacity. The displayed
front load fraction follows the measured support split, while tyre forces use
instantaneous support. Bottom-out tyre capacity is bounded to twice whole-bike
static weight per axle before the existing bank correction. Added frontSupport
and rearSupport diagnostics (force divided by total static weight). No extra
terrain queries, allocations, iterations or physics substeps were introduced.

Reviewed [Chrono's tire model overview](https://api.chrono.projectchrono.org/wheeled_tire.html)
and [vehicle dynamics paper](https://projectchrono.org/assets/white_papers/chronoVehicle_IJVP.pdf)
for load/contact and combined-slip context. Those sources distinguish handling
models from detailed deformable off-road tyre/terrain simulation. This change
reuses DockPipe's assisted model; it does not import Chrono, copy a calibrated
tyre dataset or claim equivalent motorcycle/soft-terrain fidelity.

New regression: at 8 cm suspension extension, supported rear load is .051 of
whole static weight and initial powered speed .568 cm/s versus .808 cm/s at
normal support. Previously the same static grip was available in both states.
Airborne support is zero. Upright 50/200/600 cm drop fixtures produce transient
compression/load then settle to height/velocity within .1 cm / .1 cm/s and .5
static weight per axle, without perpetual bounce. Existing severe-impact,
off-balance landing and full-flip fixtures retain their expected results.

CPU fixture passes: /tmp/cockpit-supported-grip-final-cpu.log. Normal powered
half-turn peak lateral speed 4.50 cm/s, no crash; full turn no crash, 961.1 cm/s
exit speed. Bank radius error 17.13 cm. Both authored straights and clean full
flip pass; forward jump peak clearance 1049.4 cm. Full/rear/gentle braking
speeds after the same interval: 727.7/1099.1/1137.1 cm/s. Changes in these values
are expected from actual support capacity, not weakened acceptance assertions.

Native fixture passes: /tmp/cockpit-supported-grip-native.log, with images at
/tmp/cockpit-supported-grip-*.png; inspected berm view. Hidden 4K RTX 3080 spin
median/p95/max 4.82/5.97/9.71 ms, bike update p95 1.01 ms, broad submission
3.51 ms. Prior checkpoint update p95 was 1.02 ms. These are hidden framebuffer
measurements, not mapped-window gameplay FPS. Final build passes:
/tmp/cockpit-supported-grip-final-build.log. Initial header rebuild emitted only
pre-existing Qt deprecations in browser/tests. Authored whitespace/conflict scan
passes; user-facing physics notes updated in docs/dirt-bike.md.

Eight focused CTests pass in 39.15 seconds:
/tmp/cockpit-supported-grip-regressions.log. No builds/tests remain running.
Only DirtBikeDynamics, its fixture and package docs changed in this checkpoint.
Original preview remains untouched. No commit/push/publication, system change,
worktree or task creation occurred. Continue the same overnight objective; a
useful next seam is pitch-correct axle geometry/damping or a bounded tyre contact
envelope, with strict clean-flip and ordinary cornering regression protection.
Do not introduce a broad solver rewrite merely to add a realism label. Final
qualification begins by 12:30 UTC; edits/experiments stop at 13:00 UTC.

### Checkpoint 11 — consistent pitched axle contacts, 04:53 Toronto

The suspension used sin(pitch) for axle height but assumed level-bike axle
spacing, vertical velocity and torque arms. Made the complete two-contact
geometry consistent: cos(pitch) projects horizontal axle spacing; its derivative
contributes to ground-relative axle motion along with yaw; the same projected
lever drives vertical damping, pitch torque, lateral contact velocity and yaw
torque. Bottom-out correction samples those same projected contacts. Terrain
pitch uses the signed axle separation without treating inverted wheels as an
upright landing surface. This remains the existing two-point, assisted model,
not a full tyre envelope, multibody chassis or exact match to every visual
wheel-center/steering deformation.

Extracted the small bounded spring/damper and pitch lever into package-owned
DirtBikeSuspension.h. Spring/stroke/force limits and damping coefficients are
unchanged; invalid inputs return no support. No extra terrain queries or fixed
steps were added. New tests compare the lever to a numerical height derivative,
verify dissipative damping/conjugate torque through upright/vertical/inverted
angles, and compare actual solver contact compression against independently
quaternion-rotated axles through takeoff. Existing ordinary turns, bank, braking,
drop settling, clean full flip and bad-landing assertions remain intact.

Rejected intermediate: correcting only damping/torque while keeping level-bike
terrain sample positions caused a return-straight BadLanding at z=-2110.6 cm,
pitch 1.364 radians. Kept /tmp/cockpit-pitch-contact-cpu.log,
/tmp/cockpit-pitch-contact-diagnostic.log and /tmp/cockpit-pitch-contact-trace.log.
The trace exposed the inconsistent moving contacts; correcting their positions
and ground-relative velocity resolved it without changing crash thresholds or
weakening acceptance checks. Optional DOCKPIPE_BIKE_CONTACT_TRACE=1 now emits
bounded return-straight contact diagnostics in the fixture. The fixture stops at
the first crash so a later settled wreck cannot hide the original failure.

Final CPU fixture passes: /tmp/cockpit-projected-contact-final-cpu.log.
Forward peak jump clearance 1072.7 cm; return straight reaches z=-6525.8 cm,
no crash. Half/full powered turns remain stable (4.50 cm/s peak lateral speed
in the half-turn). Bank radius error 17.31 cm. Clean full flip and deliberately
under-rotated landing checks pass. Build:
/tmp/cockpit-projected-contact-final-build.log. Authored whitespace/conflict
scan and git diff --check pass; physics notes updated in docs/dirt-bike.md.

Native fixture passes: /tmp/cockpit-projected-contact-native.log, captures at
/tmp/cockpit-projected-contact-*.png. Hidden 4K RTX 3080 spin median/p95/max
4.81/6.11/9.53 ms, physics update p95 1.02 ms, broad submission 2.74 ms. This is
hidden-FBO evidence, not mapped-window gameplay FPS. Original preview remains
untouched; no commit, push, publication, system change or task creation.

Eight focused CTests pass in 39.06 seconds:
/tmp/cockpit-projected-contact-regressions.log. No builds/tests remain running.
Next candidate is a bounded rolling tyre footprint across profile seams, only
if contact quality can improve without compromising runtime cost or the clean
flip/ordinary-turn fixtures. Broader solver or render-engine replacement remains
out of scope. Continue this objective with final qualification by 12:30 UTC and
the unchanged 13:00 UTC hard stop.

### Checkpoint 12 — bounded rolling tyre footprint, 05:06 Toronto

Replaced each axle's point support with a 35 cm rigid rolling-circle envelope.
The new package-owned DirtBikeTyreContact.h samples five heights across the tyre
and solves circle support against their four linear segments. Flat ground is
unchanged; constant slopes have analytic circle clearance. Ramp feet can lift a
tyre before its centre arrives, and crests retain a finite rolling footprint.
Suspension damping samples the moving/rotating footprint; bottom-out correction
uses the same support shape. Grip, crash thresholds, control bindings and fixed
step limits are unchanged. This is a longitudinal approximation along the bike's
heading, not a deformable tyre, steering/lean-aware contact patch or full mesh
collision solution. Features narrower than 17.5 cm can be missed. Interpolated
profile seams are approximate, so documentation distinguishes terrain triangle
queries from the envelope built over them.

Cost is bounded: six envelopes plus bank/chassis samples make at most 33 height
queries per complete riding step, each against at most 16 ride surfaces. Fixed
stack storage; no mesh creation, heap allocation or additional solver steps.
Tests independently check exact slope clearance, a dense scan at a represented
crest, five-sample budget, invalid data, and continuous early support at a ramp
foot. The 0.7-slope seam fixture bounds interpolation error to 3.1 cm and support
change to 0.176 cm per 0.25 cm of travel. Existing independent quaternion axle
geometry checks now include the footprint; ordinary turns, banking, braking,
both straights, upright drop settling and clean full flips still pass unchanged.

Build passes: /tmp/cockpit-tyre-envelope-final-build.log. An initial test-only
compile error omitted the required zero pitch argument to attitude(); corrected
without changing its API. Retained /tmp/cockpit-tyre-envelope-build.log and
/tmp/cockpit-tyre-envelope-build2.log. Baseline source/test copies remain at
/tmp/cockpit-before-tyre-envelope.cpp and /tmp/cockpit-before-tyre-envelope-test.cpp.
Initial CPU proof: /tmp/cockpit-tyre-envelope-cpu.log. Final eight focused CTests
pass in 39.29 seconds: /tmp/cockpit-tyre-envelope-regressions.log. Authored
whitespace/conflict checks and git diff --check pass.

Final native fixture: /tmp/cockpit-tyre-envelope-final-native.log; captures at
/tmp/cockpit-tyre-envelope-final-*.png. Hidden native 4K RTX 3080 spin
median/p95/max 4.78/6.18/8.93 ms, bike update p95 1.04 ms, broad submission
2.58 ms. Previous point-contact update p95 was 1.02 ms; these are hidden-FBO
measurements, not user gameplay FPS. Forward jump peak clearance 1069.8 cm;
return straight reaches z=-6515.0 cm without crashing. Half/full powered turns
remain stable, peak half-turn lateral speed 4.50 cm/s. The spin benchmark still
intentionally induces a crash; this is separate from ordinary-turn acceptance.

Only DirtBikeDynamics.cpp, the new contact helper, DirtBikeTest.cpp and package
docs changed. No build/test remains running. Original preview verified active,
running, PID 438567; no restart. No commit/push/publication, system change,
worktree or successor task. Continue the same overnight objective. A useful next
checkpoint is remaining visual/contact consistency or measured terrain-query
cost on curved profiles; do not add a broad solver or cache layer without a
concrete benefit. Final qualification begins by 12:30 UTC and all edits and
experiments stop at 13:00 UTC, unchanged.

### Checkpoint 13 — visible chassis pivot alignment, 05:17 Toronto

Inspection found the displayed chassis tilted around y=70 cm, while the solver's
projected axle pair corresponds to the midpoint of the wheel centres at y=35 cm.
That extra 35 cm offset translated both visible wheels during pitch and lean,
independently of their physics contacts. Added a small inline chassisPoint helper
in DirtBikeModel and used it in WorkspaceFlightVehicle; the pivot now shares
DirtBikeTyreContact's radius. No solver or camera changes, additional geometry,
allocations or resources. Suspension compression, raked steering and crash/rider
articulation remain the existing approximations. This fixes the shared rest
geometry, not every possible tyre/terrain or suspension-linkage discrepancy.

New tests compare both uncompressed visual axles against the solver's projected
wheelbase across upright, pitched, vertical, inverted and banked attitudes. They
also verify the axle midpoint stays fixed and the body transform preserves
length. Documentation states the remaining articulation limits.
Build: /tmp/cockpit-axle-pivot-build.log. Eight focused CTests pass in 39.20 s:
/tmp/cockpit-axle-pivot-regressions.log. Authored whitespace/conflict checks and
git diff --check pass. Changes are limited to DirtBikeModel.h,
WorkspaceFlightVehicle.cpp, DirtBikeTest.cpp and package docs.

Native fixture passes: /tmp/cockpit-axle-pivot-native.log and
/tmp/cockpit-axle-pivot-*.png. Inspected the before/after corner captures; the
rider, wheels, frame and fork remain coherent. Hidden 4K RTX 3080 spin
median/p95/max 4.96/6.22/9.18 ms, bike update p95 1.04 ms, broad submission
3.57 ms. Physics fixture values are unchanged from checkpoint 12, including
both straights, clean flip and ordinary powered turns. Hidden-FBO timing is not
mapped-window gameplay FPS.

One automatic approval review initially rejected the native fixture by treating
09:14 UTC as 09:14 Toronto. Verified with the clock tool and shell: 09:15:56 UTC
was 05:15:56 EDT (-0400), before the explicit 13:00 UTC cutoff. The identical
fixture was approved on retry with that evidence; no bypass or workaround was
used. Future approvals should include both local and UTC time when relevant.

No build/test remains running. User preview was not restarted or controlled;
no commit/push/publication, system changes or new tasks. Continue this same
objective; remaining useful seams include visual detail or contact-aware visual
articulation, but avoid pretending the assisted solver is a constrained
multibody simulation. Final qualification by 12:30 UTC, hard stop 13:00 UTC.

### Checkpoint 14 — pit detail and upright native signs, 05:39 Toronto

Added a peaked fabric canopy over the pit, slimmer metal supports, a textured
pad, service bench/toolbox and fabric covers on the track-facing side/top of the
18 existing safety blocks. Reused current meshes/materials; no downloads,
textures, runtime dependencies or scene-limit increases. Covers sit 0.5 cm off
the existing blocks to avoid coplanar depth flicker. The canopy is decorative;
eave ceiling/posts retain collision, but the sloping roof is not a new walkable
surface or exact collision mesh. Bench collision and the open pit approach are
verified. This is richer stylized track detail, not a photorealistic asset claim.

Track geometry totals 344/512 vertices, 223/256 faces, six meshes. All 72 trees
remain admitted without placement conflicts. Added finite/nondegenerate mesh,
material-count, outward roof winding and pit access/collision checks. Eight
focused CTests pass in 39.32 seconds: /tmp/cockpit-pit-detail-regressions.log.
Build receipts: /tmp/cockpit-pit-detail-capture-build.log and earlier
/tmp/cockpit-pit-detail-*-build.log. Native shelter inspection is now an explicit
fixture capture, independent of riding/camera direction.

The first visual inspection exposed an existing native-label bug: box faces
start with bottom vertices, while generic image UVs start at the top. Corrected
only the native signage UVs; billboard artwork is unchanged. Also fit the 4:1
label image within each sign so tall panels do not stretch the lettering.
Added a bottom-first sign pixel test to SpatialGpuRendererTest; native GPU unit
suite passes: /tmp/cockpit-pit-label-unit.log. Before/fixed/final aspect captures:
/tmp/cockpit-pit-detail-shelter.png, /tmp/cockpit-pit-final-shelter.png,
/tmp/cockpit-pit-fit-shelter.png. Inspected all three; final text is upright and
normally proportioned. Final build: /tmp/cockpit-pit-label-fit-build.log.

Native physics/scene fixture passes after the final label adjustment:
/tmp/cockpit-pit-fit-native.log, captures /tmp/cockpit-pit-fit-*.png. No physics
values changed from checkpoint 13. Final broad submission 2.88 ms. The full
spin run immediately before the aspect-only adjustment measured hidden native
4K RTX 3080 median/p95/max 4.92/6.09/9.04 ms and bike update p95 1.02 ms:
/tmp/cockpit-pit-final-native.log. Final capture run omitted repeated spin timing;
these remain hidden-FBO measurements, not mapped-window gameplay FPS.

Changed only SpatialBikeTrack.cpp, DirtBikeTrackArt.cpp, WorkspaceFlightGpu.cpp,
the two relevant fixtures and package docs. Authored whitespace/conflict checks
and git diff --check pass. No build/test remains running; active user preview
untouched. No commit/push/publication, driver/system changes or successor tasks.
Continue the same overnight objective; final qualification by 12:30 UTC and
all edits/experiments stop at 13:00 UTC (09:00 Toronto), unchanged.

### Checkpoint 15 — contact-aware dust and gravel, 05:52 Toronto

Moved the existing bounded dust lifecycle into package-owned DirtBikeDust so
contact transitions can be tested independently of the viewport. A moving bike
emits only with rear suspension support; a grounded wreck uses chassis contact.
The emitter follows the projected rear axle and samples terrain height, replacing
the fixed chassis-height offset. Existing particles age through stalls without
replaying missed emissions. Limits remain 24 particles, 1.1 seconds lifetime and
55 ms minimum emission spacing; no new assets or renderer resource budgets.
Airborne audio suppresses stale gravel/skid noise. advanceCrash now recomputes
grounded state after movement instead of retaining ground contact when a sliding
wreck leaves a surface. Normal riding dynamics are unchanged.

Added tests for front-only contact, projected rear position on elevated terrain,
airborne/stationary suppression, grounded wreck emissions, invalid delta time,
stalls, expiry, clear/reset and capacity. An elevated crash fixture verifies loss
and reacquisition of contact. Isolated synth samples verify gravel silence in
air and audible ground contact with the engine off. Build and standalone CPU
fixture pass: /tmp/cockpit-contact-effects-build.log and
/tmp/cockpit-contact-effects-cpu.log. Eight focused CTests pass in 39.25 seconds:
/tmp/cockpit-contact-effects-regressions.log. Authored source checks and git diff
--check pass. Changes are confined to Cockpit source/tests/CMake and this record.

Native fixture passes: /tmp/cockpit-contact-effects-native.log and PNG captures.
Hidden native 4K RTX 3080 spin median/p95/max 4.81/5.93/8.38 ms; bike update p95
1.01 ms; broad draw submission 2.62 ms. This is hidden-FBO timing, not gameplay
FPS. Turn/jump/backflip fixture values remain unchanged. Inspected corner and
spin captures; geometry and native rendering remain coherent. The spin capture
also exposes an existing presentation weakness: the detached rider remains in a
partly seated floating pose after the bike falls. A useful next bounded seam is
terrain-aware rider crash settling, without claiming a full ragdoll simulation.
Dust is still simple translucent geometry, not volumetric particulate rendering.

No build/test remains running. No preview restart, commits/push/publication,
driver/system changes or new tasks. Final qualification by 12:30 UTC; hard stop
13:00 UTC (09:00 Toronto) remains unchanged.

### Checkpoint 16 — terrain-aware rider crash settling, 06:04 Toronto

Added package-owned DirtBikeCrashPose, consumed only while rendering the active
crashed bike. The rider falls to the bike's fall side and settles against terrain
instead of using a fixed vertical offset. The existing bounded model is scanned
once for each occupied body part's transformed bounds, followed by five terrain
samples per occupied part. No new dynamic allocation, model faces, texture budget,
physics steps or dependencies. Positive terrain clearance is applied immediately;
downward settling blends over 0.9 seconds only while the wreck is grounded.
Airborne crashes retain their height rather than snapping to the floor. Vertex
and smooth-normal transforms share the same rotation.

This is a rigid procedural fall, not a ragdoll. It uses conservative terrain
samples and body-part bounds; it does not add articulated limb collisions,
independent rider momentum or arbitrary obstacle contacts. Rest limbs still keep
a stylized bent pose. Very narrow terrain features between samples are not an
exact collision guarantee. Normal riding, suspension, crash triggers and clean
backflip acceptance are unchanged.

Tests check actual rider vertices for ground clearance on flat and +/-0.35
slopes, both fall directions and three headings; settled flat-ground contact is
2 cm. Also check rigid distances, rotation normalization, stable final pose,
all 91 fall stages against flat terrain, and airborne non-teleportation. Build,
CPU fixture and eight focused CTests pass (39.24 seconds):
/tmp/cockpit-crash-pose-build.log, /tmp/cockpit-crash-pose-cpu.log,
/tmp/cockpit-crash-pose-regressions.log. Authored whitespace/conflict checks and
git diff --check pass. Files changed: DirtBikeCrashPose.h/.cpp,
WorkspaceFlightVehicle.cpp, DirtBikeTest.cpp, package CMake and this record.

Native fixture now captures the settled wreck after its animation completes,
separately from the mid-fall spin capture. Inspected
/tmp/cockpit-crash-pose-settled.png: rider is down beside the bike with terrain
contact; the pose remains stylized. This also corrects the previous checkpoint's
interpretation: the earlier spin image was captured mid-fall, so it alone did not
prove a permanently floating rider. The fixed vertical offset lacked any terrain
contact rule regardless, now covered by direct tests and a settled capture.

Native fixture passes: /tmp/cockpit-crash-pose-native.log. Hidden 4K RTX 3080
spin median/p95/max 4.71/6.26/8.53 ms, bike update p95 1.04 ms, broad submission
3.41 ms. These are hidden-FBO measurements, not mapped gameplay FPS. No build or
test remains running; active preview was untouched. No commits/push/publication,
driver/system changes, new tasks or package-boundary changes. Continue the same
objective, begin final qualification by 12:30 UTC and stop at 13:00 UTC.

### Checkpoint 17 — gravity outlives the crash animation, 06:11 Toronto

Separated wreck settling from the 2.4-second crash pose duration. A wreck that
leaves support near the end of that duration keeps falling and keeps the scene
ticking until it reaches terrain. Previously the timer zeroed velocity and
stopped all updates even in mid-air. Grounded wrecks still stop at the existing
duration; no additional simulation step budget or animation duration. Extended
the elevated-surface fixture to leave support shortly before pose completion,
assert ongoing gravity/ticking after the deadline, then verify final ground
contact, zero velocity and idle state.

Build, standalone CPU fixture and eight focused CTests pass (39.19 seconds):
/tmp/cockpit-crash-gravity-build.log, /tmp/cockpit-crash-gravity-cpu.log,
/tmp/cockpit-crash-gravity-regressions.log. No renderer changes in this checkpoint;
native validation remains checkpoint 16. Only DirtBikeDynamics.cpp, its fixture
and this record changed. Preview untouched, no running tests/builds, no commit
or publication. User asked whether work had stopped; clarified that prior runs
paused between 15-minute heartbeat checks. Continue actively between checkpoints
now, keeping the same 12:30 UTC final-qualification and 13:00 UTC stop deadlines.

### Checkpoint 18 — distinct bike material finishes, 06:19 Toronto

Added a small in-memory SpatialSurfaceFinish enum (Standard, Painted, Metal)
through vehicle faces, view meshes and the existing GPU renderer. Standard
preserves existing scene appearance; opaque batching includes finish identity.
Paint uses a tighter highlight, metal uses coloured Fresnel reflection with no
metal diffuse lobe, and fabric/rubber retain their rough textured response.
The metal environment is a bounded analytic sky/ground hemisphere, not live
reflections or a new environment capture. No new assets, geometry, dependencies,
texture budgets or render targets. Painter fallback retains its existing baked
colour/shading. This is improved material separation, not full glTF/PBR pipeline
conformance: existing colour-space handling and simplified environment lighting
remain. Reference for the metallic/roughness and Fresnel split:
https://registry.khronos.org/glTF/specs/2.0/glTF-2.0.html#appendix-b-brdf-implementation

Fixed bike material classification using authored albedo instead of baked face
brightness. Metal rims no longer become rubber on shaded facets; rider cloth
stripes stay cloth. Imported engine/cylinder/exhaust pieces are metal, panels
are painted, and the seat is rubber. Tests cover material assignment, distinct
GPU finishes sharing one texture, invalid finish rejection and painter recovery.
Native unit test passes: /tmp/cockpit-material-finishes-unit.log. Build and CPU
fixture pass: /tmp/cockpit-material-finishes-build.log and -cpu.log. Eight focused
CTests pass in 39.30 seconds: /tmp/cockpit-material-finishes-regressions.log.
Source whitespace/conflict checks and git diff --check pass.

Native scene/physics fixture passes: /tmp/cockpit-material-finishes-native.log
and captures. Inspected /tmp/cockpit-material-finishes-corner.png against the
previous corner capture; the gold fork/frame show distinct highlights, with
preserved silhouettes, clothing and terrain. Improvements are deliberately
modest; this does not make the scene photorealistic. No preview restart or user
app control. No commits/push/publication, system changes or new tasks. Continuing
active work toward 12:30 UTC qualification and 13:00 UTC cutoff.
Checkpoint 18 native measurement: hidden 4K RTX 3080 render median/p95/max
4.95/5.93/8.73 ms; bike update p95 1.09 ms; broad submission 2.88 ms. These remain
hidden-FBO measurements, not mapped-window gameplay FPS.

### Checkpoint 19 — airborne yaw continuity, 06:23 Toronto

The air branch decayed yawRate but never applied it to heading, so takeoff
abruptly stopped visible yaw despite nonzero angular state. It now integrates
the existing damped yaw rate while airborne. Steering still supplies no tyre yaw
torque without contact; existing rider roll control is unchanged. Added a
controlled elevated-straight takeoff fixture with nonzero yaw: compare heading
and rate against an independently accumulated 30-step decay, compare opposite
steering inputs, and assert airborne motion continues. This preserves momentum
within the assisted Euler solver; it is not full 3D rigid-body/gyroscopic coupling.

Build and standalone CPU tests pass: /tmp/cockpit-air-yaw-build.log and
/tmp/cockpit-air-yaw-cpu.log. Existing jump/backflip, return straight and powered
turn fixture values are unchanged. Eight focused CTests pass:
/tmp/cockpit-air-yaw-regressions.log. git diff --check passes. Only dynamics,
its fixture and this record changed. No native renderer change, preview restart,
commits/publication or new tasks. Continuing work within the same cutoff.

### Checkpoint 20 — model-attached fabric and tyre textures, 06:31 Toronto

Authored bounded UV coordinates once on textured bike/rider faces in rest-model
space, carrying them through the existing view mesh to the GPU face. The native
renderer already accepts explicit UVs; this removes its world-coordinate fallback
for moving fabric/rubber so texture detail follows wheel spin and rider motion.
Terrain remains world-aligned and the painter's existing local face mapping is
unchanged. No new images, triangles, runtime caches or renderer limits.

CPU fixture verifies each textured face has finite, nondegenerate coordinates;
plain faces need none. GPU pixel test translates both model and camera and
verifies unchanged fabric texels. Build, CPU and native unit suites pass:
/tmp/cockpit-model-uv-build.log, -cpu.log, -unit.log. Eight focused CTests pass in
39.28 seconds: /tmp/cockpit-model-uv-regressions.log. Whitespace/conflict and git
diff --check pass. Native scene/physics fixture passes:
/tmp/cockpit-model-uv-native.log and captures. Inspected corner capture; preserved
material separation and geometry. Hidden 4K RTX 3080 median/p95/max
4.90/6.70/11.96 ms; update p95 1.09 ms; broad submission 2.76 ms. These are bounded
hidden-FBO measurements, not gameplay FPS; this run's tail is higher than the
previous run and is retained rather than discarded. Shared UV vectors do not
change the model's 3308-face count. Canonical dirt-bike docs also updated for
checkpoints 15–19. No preview restart or unrelated changes; no build/test active.

### Checkpoint 21 — modest dry-surface grip, 06:47 Toronto

Each supported axle now reads its admitted ride-surface/ground material. Dirt
retains prior handling; grass scales available tyre force to .88 and rock .96.
These are assisted dry-surface defaults, not measured soil coefficients. Two
flat dirt lane footprints preserve the track straights; jumps/berms retain dirt.
HUD labels match grass/rock/dirt/air in both camera modes. Invalid/out-of-budget
surfaces cannot influence classification. Geometry remains bounded (10 of 16
ride surfaces); 72 admitted trees and the original track jump/berm results remain.

CPU tests cover footprint limits, invalid/budgeted surfaces, reduced grass
acceleration, braking from identical states and sustained ordinary grass turns.
Initial native spin test failed because its short run-up produced a slide that
slowed safely before the sustained high-speed crash threshold. Kept that failed
receipt (/tmp/cockpit-surface-grip-native.log), plus diagnostic CPU receipts.
Added explicit short-run-up safe-stop and faster LostGrip cases; the native
fixture now takes a longer run-up and has a bounded 90–180 frame slide window.
No crash thresholds or existing normal-turn/backflip assertions were weakened.

Final build: /tmp/cockpit-surface-grip-final-build.log. Eight focused CTests pass
in 39.27 seconds: /tmp/cockpit-surface-grip-final-regressions.log. Native fixture
passes: /tmp/cockpit-surface-grip-final-native.log, 90 measured frames, crash and
settled-wreck assertions pass. Hidden 4K RTX 3080 median/p95/max 5.12/6.53/9.99 ms,
update p95 1.12 ms, broad submission 3.24 ms. Longer run-up changes the trajectory,
so these are not an exact paired comparison with earlier spin captures. Preview
untouched; no commits/publication. Next: reduce obvious terrain texture repetition
using existing material images and verify native GPU cost/visual continuity.

### Checkpoint 22 — terrain repetition, 06:50 Toronto

Native dirt/grass/rock now blend the same shared colour and normal/roughness maps
at two differently oriented/scaled footprints, with deterministic broad world
variation. Tangent normals are rotated into the original basis before blending;
roughness uses squared blending and filtered normal variance. No additional
images, caches, targets, geometry or scene data. Painter fallback is unchanged.
This reduces visible grids/stripes, not a claim of unique terrain photogrammetry.

Build passes: /tmp/cockpit-terrain-variation-build.log. Native GPU unit tests pass
(/tmp/cockpit-terrain-variation-unit.log), including stable repeated pixels,
continuity across mesh splits, and broad placement variation at fixed UVs.
Native full Moto fixture passes: /tmp/cockpit-terrain-variation-native.log and
captures. Inspected overview against checkpoint 20; repeating strips are reduced
but remain perceptible on the simple cliff geometry. Hidden 4K spin median/p95/max
5.17/6.44/10.89 ms, update p95 1.11 ms, broad submission 2.81 ms. Similar median/tail
to checkpoint 21, not a claim of unchanged mapped gameplay timing. The native run
also executes the deterministic physics checks. Fourteen documented source-image
SHA-256 values verified against local bundled files. No active test/build; preview
untouched. Next: bounded near-track ground vegetation and final package test wiring.

### Checkpoint 23 — bounded near-track grass, 06:55 Toronto

Added one reusable opaque tuft mesh (960 triangles / 2880 staging vertices),
instanced in up to 24 already admitted tree footprints. No textures, alpha
blending, new collision or extra instance budget. Trees plus grass remain at
most 96; blade height shrinks from 55–80 metres before omission. This is modest
near-ground detail, not full-field vegetation. Painter fallback unchanged.

Builds pass: /tmp/cockpit-ground-cover-build.log and -pixels-build.log. Native
unit fixture verifies shared identity, normalized bounds, normals, triangle cap
and actual pixel coverage in one instanced draw call:
/tmp/cockpit-ground-cover-unit.log. Full native Moto fixture passes:
/tmp/cockpit-ground-cover-native.log and captures. Inspected corner image: grass
stays under the reserved tree areas and off the riding line. Hidden 4K spin
median/p95/max 5.47/6.80/9.95 ms, update p95 1.10 ms; broad submission 3.63 ms.
A small rendering cost is retained explicitly; no extra per-bike physics work.
No active test/build, preview untouched. Next: camera obstruction smoothing
review and complete package validation wiring before final qualification.

### Checkpoint 24 — chase-camera obstruction after smoothing, 07:02 Toronto

Extracted the bounded chase boom clearance into DirtBikeCameraBoom. The previous
code clipped only the desired position, allowing exponential smoothing to leave
the actual camera inside/behind scenery. Both target and smoothed position now
use radius-aware obstruction/terrain queries: at most two 32-step sweeps at
25 cm spacing and 800 cm reach. Obstructions pull inward immediately; release
outward remains damped. Pending state compares with the clipped target, so a
parked bike beside a wall does not tick forever. This remains sampled clearance,
not an exact swept-sphere solver; an already blocked anchor has no guaranteed
clear position and falls back to the anchor.

Tests cover an obstructed old position, turning around a wall, damped release,
snap, ground clearance, invalid old coordinates, bounded work/reach and eventual
idle beside a wall. Build/CPU/eight focused CTests pass (39.28 seconds):
/tmp/cockpit-camera-clearance-idle-build.log, -cpu.log, -regressions.log.
Full native fixture passes: /tmp/cockpit-camera-clearance-native.log, hidden 4K
median/p95/max 5.45/6.63/9.81 ms, update p95 1.19 ms, broad submission 3.87 ms.
Preview remains untouched. The package test runner now builds/registers the new
spatial-gpu test; bash syntax passes, full runner validation still pending.
Next: front-wheel steering direction in terrain contact (source prepared,
not built yet), then broader package qualification.

### Checkpoint 25 — steered tyre support, 07:12 Toronto

The front tyre's circle support envelope now follows its steering direction,
including the forecast contact used for ground-relative suspension damping.
The rear continues to follow chassis heading. This changes the physical contact
on side slopes without adding terrain queries. A calculated planar-slope test
covers both travel directions and both steering signs, and distinguishes the
old straight-ahead footprint. Tyre lean shape remains an approximation.

Software frames now reject native scene preparation before packing GPU faces
or loading native foliage/material resources. The renderer retains its own
late admission check for direct callers. Added an initial-scene timing receipt
to the native fixture; this includes image capture/setup, not cold driver startup.

Builds, CPU tests, eight focused CTests (38.61 seconds) and six offline neural
tests pass: /tmp/cockpit-steered-contact-build.log, -cpu.log, -regressions.log,
/tmp/cockpit-native-admission-build.log, and /tmp/cockpit-neural-offline-regressions.log.
Native fixture passes: /tmp/cockpit-steered-contact-native.log. Hidden 4K spin
median/p95/max 5.49/7.25/11.54 ms; update p95 1.23 ms; broad submission 3.14 ms.
Initial scene/capture 3410 ms, first measured render 6.42 ms. The initial delay
needs decomposition before attributing it to rendering. Active preview untouched.
Broader package runner started; next: inspect entry timing and package results.

Broader package runner completed: six offline neural tests pass, all configured
targets build, and CTest reports 16 passed / 4 skipped / 0 failed in 116.73 seconds.
Skipped offscreen: package rendering, embedded browser, desktop app windows,
native GPU. Native GPU has separate receipts above; this does not qualify live
browser/app handoff. Log: /tmp/cockpit-overnight-package-validation.log. Added
entry-stage timestamps to distinguish construction/loading/event/capture costs;
that diagnostic-only test change is building separately.

### Checkpoint 26 — force in the steered wheel frame, 07:25 Toronto

Front contact velocity, braking direction and the combined friction budget now
use the steered wheel frame. Rotate the resulting force back for chassis
acceleration and yaw. The old front lateral force had no longitudinal scrub
component and front braking always acted along the chassis. Tests verify contact
power invariance, zero slip for aligned rolling, braking/cornering force signs,
and mirrored braking turns. Normal turns and clean jump/flip landings still pass.

The original short locked-rear slide stopped safely after this correction, so
the old always-crash assertion failed. Retained /tmp/cockpit-wheel-frame-cpu.log
and -probe-cpu.log; a bounded run-up probe confirmed faster slides still crash.
Regression now requires safe recovery for the slow slide and LostGrip for the
faster slide. Existing crash thresholds were not changed. Dry grass retains
both cases too. This is still assisted two-contact handling, not tyre calibration.

Build/CPU/eight focused CTests pass in 38.57 seconds:
/tmp/cockpit-wheel-frame-final-build.log, -final-cpu.log, -regressions.log.
Full native fixture passes (/tmp/cockpit-wheel-frame-native.log), hidden 4K spin
median/p95/max 5.50/6.60/10.32 ms; update p95 1.19 ms, broad submission 5.46 ms.
The slide trajectory changes with the corrected forces; timings are not paired
identical-camera comparisons. Active preview remains PID 438567, running.

Entry-stage probe: constructor 3 ms, show 1 ms, prepare 71 ms, event processing
1594 ms, explicit capture/save 1715 ms. A temporary renderer probe measured first
native packing/material decode 179 ms, initialization 2 ms, texture upload 62 ms,
submission 2 ms. The larger event interval includes Qt/hidden-fixture work; do not
present it as measured mapped-gameplay entry latency. Removed temporary renderer
instrumentation after recording it. Persistent test-stage timestamps remain.
Next: instrumented memory/undefined-behaviour validation in a separate temporary
build, then final qualification by 08:30 Toronto. No preview restart or publication.

### Checkpoint 27 — cutout edge coverage and sanitizer qualification, 07:43 Toronto

Native foliage and lettering now use alpha-to-coverage when the existing MSAA
target is available. Alpha-to-one keeps the resolved scene opaque; sample flags
are restored for Qt. Non-MSAA and shadow cutouts retain the old threshold.
No additional render target, texture, history buffer or resolution reduction.
Reference: [Khronos multisampling](https://wikis.khronos.org/opengl/Multisample).

Native unit tests cover hard-cutout fallback, partial sample coverage with MSAA,
opaque resolved pixels, surrounding painter output and sample-state restoration
with caller flags both disabled and enabled. Build and native unit pass:
/tmp/cockpit-cutout-coverage-build.log, -unit.log, -final-build.log.
Full native fixture/captures pass: /tmp/cockpit-cutout-coverage-native.log.
Hidden 4K spin median/p95/max 5.41/6.97/10.90 ms; update p95 1.18 ms; broad
submission 2.98 ms. Inspected pit image: foliage remains readable and the scene
stays opaque; simple slopes and stylized model fidelity remain limitations.

Separate GCC AddressSanitizer/UndefinedBehaviorSanitizer build in
/tmp/dockpipe-moto-sanitize-20261016 (-O1, debug locations, frame pointers, -j2).
No compiler/runtime/driver installation. First full offscreen UI run failed its
four-second camera-return assertion under instrumented software rendering;
LeakSanitizer also cannot operate inside the tracing sandbox. Kept the failure:
/tmp/cockpit-moto-sanitize-cpu.log. Normal wall-clock integration tests retain
their deadlines. Added DOCKPIPE_BIKE_PHYSICS_ONLY for the deterministic solver,
model, terrain, schema and math lane, without interactive wall-clock assertions.

That lane passes ASan/UBSan/leak detection outside the tracing sandbox:
/tmp/cockpit-moto-sanitize-physics.log. Native GPU assertions also pass, but raw
LSan reports 16,669 bytes in 588 Fontconfig allocations:
/tmp/cockpit-moto-sanitize-gpu.log. A minimal Qt-only text program, loading no
DockPipe code, reproduces 16,681 bytes / 588 allocations with the same Fontconfig
stacks (/tmp/cockpit-fontconfig-baseline.log and source directory). GPU sanitizer
run passes with only libfontconfig.so.1 suppressed; all 588 / 16,669 bytes are
reported in /tmp/cockpit-moto-sanitize-gpu-platform-filtered.log. Suppression is
temporary /tmp/cockpit-fontconfig.lsan, not a product/CI suppression. Do not claim
the raw native process has zero shutdown allocations. No source/runtime fault
was reported in these qualified lanes; full timed UI under sanitizers is not
qualified. Next: exercise GPU texture-cache exhaustion/eviction/recovery; source
prepared and normal build running, no preview changes.

### Checkpoint 28 — texture residency pressure and recovery, 07:50 Toronto

Native tests now submit fourteen distinct 1024-square images together, exceeding
the existing 64 MiB mipmapped GPU cache. The frame fails admission with a clear
residency error and zero geometry draws, without breaking the surrounding Qt
painter. Individual subsequent frames exercise eviction and reload; the oldest
image renders correctly again, errors clear and explicit release returns resident
bytes to zero. Added only a read-only residentTextureBytes diagnostic; no cache
policy or resource budget increase.

Build/native unit pass: /tmp/cockpit-texture-pressure-build.log and -unit.log.
Peak observed resident bytes 61,516,631, below 67,108,864. ASan/UBSan build and
native pressure test also pass: /tmp/cockpit-texture-pressure-sanitize-build.log
and -sanitize.log, with only the previously reproduced Fontconfig shutdown
allocation suppression (588 allocations / 16,669 bytes). Raw platform failures
remain preserved. Focused 33-file whitespace/conflict scan and git diff --check
pass; branch/HEAD unchanged and unrelated dirty state preserved.

Rider pose review found no demonstrated defect in the existing shared torso,
grip and peg constraints; left the animation unchanged. Reviewing low-speed
braking next: a steered contact may apply full opposing force even at zero wheel
velocity. Added a controller-overlap regression to establish whether it creates
unwanted stationary motion before changing the solver. Active preview untouched.

### Checkpoint 29 — stopped brake contact, 07:58 Toronto

The new regression reproduced 1.3881 cm/s residual jitter when holding both
controller triggers with the handlebars turned. The front brake spent a full
force even when there was no contact motion to oppose. Braking demand now caps
the contact impulse using wheel velocity, time and yaw response. Rear braking
includes the current drive request so a held brake does not release/reapply on
alternating steps at rest. Added both steering directions and overlapping
throttle/service-brake tests, plus impulse sign/no-overshoot/invalid-input checks.

Preserved failures: /tmp/cockpit-brake-rest-repro.log and -repro-build.log;
/tmp/cockpit-brake-contact-cpu.log caught the first rear-brake version creeping
at 5.5 cm/s, before accounting for drive. Final deterministic physics pass:
/tmp/cockpit-brake-drive-build.log and -cpu.log. Rest now remains motionless;
normal turns/braking distances/clean jumps and flips retain their prior results.
Slow locked-rear slides recover, faster slides still report LostGrip. Eight
focused CTests pass: /tmp/cockpit-brake-contact-regressions.log. No threshold
relaxations, additional simulation steps or dependencies. Preview untouched.

Next visual seam: scene meshes expose optional smooth normals, but the room
geometry admission currently discards them. Investigating a bounded path that
preserves these normals and their transforms, with a safe flat-shaded fallback.

### Checkpoint 30 — scene normals and track signage, 08:05 Toronto

Scene admission now retains valid per-vertex normals, supplies separate unlit
albedo to the GPU and keeps face lighting for the painter fallback. Missing,
zero, non-finite or mismatched normal arrays fall back to face shading. Streamed
passages rotate normals with geometry. Hill triangles now wind outward and share
area-weighted vertex normals; removed their baked per-face lighting. Same 344
scene vertices / 223 faces, same textures and collision budgets.

CPU and native renderer unit pass: /tmp/cockpit-smooth-scene-build.log, -cpu.log,
-unit.log. Full hidden 4K fixture passes: /tmp/cockpit-smooth-scene-native.log.
Smooth versus flat scene admission changes 17,665 sampled hill pixels, proving
the data reaches the render path. Inspected pit/rider captures: hill lighting
is smoother, source textures remain detailed, but silhouettes and rider are
still stylized. Spin median/p95/max 5.51/6.92/11.50 ms, update p95 1.27 ms,
broad submission 3.54 ms. This remains hidden-FBO evidence, not gameplay FPS.

Capture review also exposed unreadable track signs: 80 cm-wide edges on tall
slabs. Replaced the three slabs with 360 by 160 cm boards on two slim posts,
outside the riding lanes and hill footprints. The return-straight sign faces
the rider's approach. Added proportion, height, lane-clearance and direction
contracts. Final launcher/plugin/test build running:
/tmp/cockpit-final-launcher-build.log. Freeze feature work here; next is final
package, sanitizer and native visual/performance qualification. No preview
restart, commit, publication or system configuration changes.

### Checkpoint 31 — final qualification in progress, 08:10 Toronto

Final launcher/plugin/test build passed. Full package runner passed all six
offline neural tests and 16 CTests, with four display-dependent skips, in
116.86 seconds: /tmp/cockpit-final-package-validation.log. Skips remain package
rendering, embedded browser, desktop app windows and native GPU; native GPU has
separate passing evidence and final hardware capture is still pending. Do not
describe these skips as live browser/app handoff qualification.

Fifteen bundled material hashes match provenance, including the original
juniper image. Forty-five focused authored files pass whitespace/conflict
checks; git diff --check also passes. Branch/HEAD unchanged. Final status list
exactly matches admission (/tmp/cockpit-final-status-difference.txt is empty);
this compares tracked/untracked state, not every unrelated file's content.
Frozen source receipt: /tmp/cockpit-final-source-sha256.txt, 186 package source,
test and resource-manifest files. Documentation remains editable for receipts.

Final sanitizer rebuild running: /tmp/cockpit-final-sanitize-build.log. After
it completes, run deterministic physics, native GPU and hidden native scene
instrumented lanes, then the non-instrumented final 4K captures/performance
fixture and the additional spatial-civic regression. Preserve active preview.

Final deterministic ASan/UBSan/leak lane passes without suppressions:
/tmp/cockpit-final-sanitize-physics.log. Native GPU sanitizer lane passes with
the already reproduced Fontconfig-only suppression (588 / 16,669 bytes):
/tmp/cockpit-final-sanitize-gpu.log. Additional spatial-civic CTest passes in
1.21 seconds (/tmp/cockpit-final-civic.log). Preview still active/running at
PID 438567; all 186 frozen source hashes unchanged at 08:15.

Full hidden native scene assertions also pass under sanitizers, but shutdown
LSan reports another 2,493 bytes / 3 allocations through libdbus-1.so.3:
/tmp/cockpit-final-sanitize-scene.log. Kept that failing receipt. A separate
Qt-only QApplication/QGraphicsView/QOpenGLWidget program loads no DockPipe code
and reproduces 2,297 bytes / 3 allocations with the same D-Bus allocator offsets
(/tmp/cockpit-qt-widget-baseline.log and source directory). The direct object is
184 bytes in both; two indirect reallocations differ in message size. Baseline
also has the previously identified Fontconfig allocations.

Final native scene rerun uses /tmp/cockpit-qt-platform.lsan with only the two
independently reproduced libraries suppressed. No product/CI suppression or
system-library mutation. Do not claim the raw process has zero shutdown leaks;
platform allocation limitations and both independent baseline receipts remain.

### Final result — qualified overnight pass, 08:23 Toronto

Final native scene sanitizer run passes with only the reproduced platform
allocations filtered: /tmp/cockpit-final-sanitize-scene-filtered.log. It reports
588 Fontconfig allocations / 16,653 bytes and three D-Bus allocations / 2,493
bytes suppressed. Raw failures and Qt-only baselines are retained above. The
normal timed UI lane passes without instrumentation; its sanitizer variant was
not qualified because of the earlier timing assertion failure.

Final non-instrumented hidden 3840 by 2160 RTX 3080 fixture passes:
/tmp/cockpit-moto-final-native.log. Camera-turn render median/p95 5.81/11.06 ms
over 36 frames. Deliberate spin-out median/p95/max 5.39/6.75/11.13 ms over 90
frames, bike update p95 1.20 ms. Actual crash, settled idle wreck, native renderer
through furnishing/riding, scene-normal pixel difference and captures all pass.
Broad view draw submission 2.97 ms. These are hidden framebuffer measurements,
not mapped gameplay FPS or a controlled like-for-like comparison to old scenes.
Inspected final pit, cornering and settled-wreck images under
/tmp/cockpit-moto-final-*.png. Signboards are readable; shadows, geometry and
native-resolution textures survive the tested views. The model/landscape remains
stylized and the crash body is a rigid procedural pose, not a full ragdoll.

The launcher and optional cockpit library are built in
src/app/tooling/dockpipe-launcher/build/modern-web. Final package validation is
six offline neural tests plus 16 passing / four skipped CTests; spatial-civic
adds one passing CTest, and native GPU/scene validation is recorded separately.
All 186 source hashes remain frozen after qualification. Fifteen material hashes
match provenance. Generated test logs, captures, sanitizer build, minimal Qt
baselines and suppressions are temporary /tmp artifacts; no one-off root scripts.
Package/engine boundary preserved. No new runtime framework or dependency.

Known limits: assisted two-contact dynamics rather than a calibrated multibody
motorcycle/tyre simulation; no soil deformation/ruts, full rider ragdoll, temporal
AA or DLSS integration; native geometry currently covers eligible outdoor scenes,
while live-display rooms retain the compatible painter path. Physical controller
feel, audible speaker output and live browser/app handoff need user testing.

Feature editing and experiments are complete before the 09:00 deadline. Preserve
the active preview (last verified active/running PID 438567), protected checkout
and branch/HEAD. Nothing committed, pushed, published or restarted. Disable the
heartbeat on completion; do not automatically reopen this completed pass.
Automation update confirmed PAUSED at 08:23 Toronto; no successor was created.

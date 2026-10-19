# Mesh, material and articulation tooling

Cockpit's procedural assets use `SpatialVehicleFace`: rest-space vertices, optional smooth
normals and UVs, unlit albedo, material, surface finish, and an integer articulation group.
The native renderer consumes those same surfaces as the existing spatial scene. There is no
separate bike renderer or external engine dependency.

## Profiled meshes

`SpatialMeshLoft::build` joins elliptical cross-sections with smooth side normals and optional
flat end caps. Each `SpatialLoftSection` gives a centre and positive half-width/half-depth.
The across and depth axes define a common perpendicular cross-section frame; their cross
product is the direction in which centres must advance. Centres may shift sideways between
sections, so the tool supports tapered clothing, swept shells and sculpted covers.

Callers provide the remaining aggregate face budget and a `SpatialVehicleFace` style. The
result preserves part, material, finish and colour. Positions/normals replace the style's
geometry and inherited UVs are cleared; author UVs against the resulting rest-space surfaces
before animation. A successful result has `(section_count - 1 + cap_count) * sides` polygons,
where `cap_count` is two for capped meshes and zero otherwise. Check `succeeded()` and the
error before appending. Invalid frames, nonpositive radii, non-advancing sections, float
overflow, degenerate surfaces and budget exhaustion return an error with no partial mesh.
Both the product and individual dimensions are bounded before output allocation.

Use distinct lofts when a path doubles back through its section plane. This helper is not a
freeform modeller, skinning system or general curved-path extrusion. Existing packaged mesh
loading through `SpatialAssetLibrary` remains available for authored geometry.

## Materials

Use unlit `albedo` for material classification. `color` is the painter fallback and may include
baked lighting. Rubber tyres/soles, fabric clothing/seats, painted plastics and metal covers
use existing `SpatialSurfaceMaterial` and `SpatialSurfaceFinish` contracts. Authored vertex
normals give smooth surfaces; end caps keep hard normals. `DirtBikeModel` assigns UVs once
in rest space so wheel rotation and joint motion do not slide the texture through the mesh.

The new bike pass uses lofts for fenders/seat, jersey/collar, sleeves/trousers, boots, helmet
shell and engine covers. Solid staggered tyre knobs, crossed spokes, brake hardware, chain
runs and a coil spring provide additional shape without changing the 4,096-face admission
limit. Decorative panels are approximate original race graphics, not manufacturer assets.

## Pivots and rider contacts

Keep geometry in the existing model coordinates (centimetres, ground at Y=0, forward +Z).
`DirtBikeModel::Part` separates frame, steering assembly, lower fork, swingarm, wheels and
rider bones. `WorkspaceFlightVehicle` owns transforms; new geometry should use those groups
rather than baking a posed bike. Wheel centres remain at `(0,35,±74)`, steering uses the
shared `DirtBikeSteeringRig` stem and axis, and chassis rotation uses the axle midpoint. The shock is a chassis detail;
coil compression and physical chain deformation are not simulated.

`DirtBikeRiderPose` exposes rest shoulder/elbow/grip and hip/knee/ankle points, analytic
two-bone poses and normal rotation. Fit clothing sections in each rest bone's frame. Hands
must remain at the grips and soles over the pegs throughout steering, lean, weight and
suspension changes. Preserve `DirtBikeCrashPose` for detached fall/settling geometry.
The rider uses rigid pieces with shaped overlap at joints; there is no weighted skeletal
skinning or cloth simulation.

## Verification

`cockpit-test --dirt-bike` checks loft failure boundaries, outward normals, style retention,
model finiteness/nondegeneracy, UVs, material classification, face admission, joint continuity,
hand/foot contacts, crash clearance and solver behaviour. `DOCKPIPE_BIKE_PHYSICS_ONLY=1`
runs these deterministic contracts without the interactive wall-clock lane.

The hidden native GPU lane adds `-bike-detail.png` and `-rider-detail.png` captures alongside
chase, corner, crash and overview views. See [the Moto guide](dirt-bike.md) for capture and
benchmark switches. Inspect silhouettes and contact alignment in those views. Hidden 4K
framebuffer timings are render/update evidence, not mapped-window gameplay FPS.

The [2026-10-16 validation checkpoint](modern-moto-validation.md) records the bounded
implementation, native evidence and remaining fidelity limits.


## Generated front assembly

`DirtBikeSteeringRig` is the attachment contract for the generated front assembly:
head stem, 26-degree raked axis, axle, fork centrelines and left/right grip anchors.
`Builder::frontAssembly` in `DirtBikeModel` constructs fork tubes, triple clamps,
steering shaft/head tube, handlebars, grips and levers from those anchors. These are
procedural parts, not an imported whole-bike asset. Reuse the rig anchors rather
than copying coordinates into a second model or animation implementation.

The handlebars have their own model group, rotating with upper/lower forks, front
wheel, plate and fender about the shared head joint. Wheel spin and the solver's
existing vertical suspension travel are applied before steering. Frame/head tube
stay fixed to the chassis. Fork tubes share a rest centreline. Rider IK takes the
same grip anchors and steering rotation; its hands do not use a separate guessed
bar motion. Tyre forces use the ground-plane rolling yaw projected from that same
raked rotation, rather than treating head-joint rotation as a vertical yaw angle.

Both native and painter paths show these actual surfaces in rider view. The former
fixed 2D handlebar graphic is removed. Head/torso/legs are hidden from rider view;
arms and hands remain attached to the real bars. The rider lens is about 84.5 degrees
vertically, shared by native rendering, painter projection and frustum culling. This
keeps controls in view without a remote camera behind the shoulders. The eye anchor
follows chassis pitch/lean, not bar rotation. It remains a gameplay camera, not an
anatomically exact eye position.

The suspension solver still uses vertical axle travel and assisted steering/lean;
this pass does not claim a calibrated motorcycle simulation, full countersteering,
or exact telescopic wheel/contact trajectories. `DOCKPIPE_BIKE_STEERING_INSPECTION=1`
in the hidden hardware lane captures left/neutral/right rider and chase views and
requires visible image changes while the parked chassis stays in place.

The [steering validation checkpoint](steering-validation.md) records rendered
left/neutral/right proof, affected regressions, measured cost and remaining limits.

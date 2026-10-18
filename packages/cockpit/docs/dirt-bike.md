# Dirt bike and RedBud map study

Open **Cockpit → Web → Moto track**, or use the **RedBud** entrance outside HQ.
The orange bike is directly ahead at spawn. Aim at it and press **E / controller A**.
The default level follows the official 2024 RedBud track-map layout, with authored
rolling hills, switchbacks, jump profiles and grounded woodland scenery. It is a
map-based approximation; elevations and dimensions are not surveyed replica data.
The original Mesa oval remains a deterministic validation fixture.

Tyres cut persistent session ruts into the same terrain sampled by suspension and
walking. Repeated passes deepen them; wet soil cuts faster. **T** or
**Start → Start rain / Stop rain** changes weather. Rain darkens and softens soil,
low spots collect runoff, and water adds drag and lowers grip. Switching rain off
leaves wet ground and puddles that drain gradually. Bike recovery and restarting
a race retain wear; re-entering the world resets it. Storm accumulation is sped
up for short play sessions. See [terrain contracts](terrain-system.md) and
[RedBud qualification](redbud-validation.md).

| Action | Keyboard | Controller |
| --- | --- | --- |
| Accelerate | W | RT, proportional |
| Brake / back up | S; hold at rest for 0.75 s to reverse | LT, proportional; hold at rest for 0.75 s to reverse |
| Steer | A / D | Left stick horizontal |
| Pull back / wheelie / nose up | Down + W while rolling | Left stick back + RT |
| Lean forward / nose down | Up | Left stick forward |
| Look around | Right-drag | Right stick |
| Handbrake / rear-wheel skid | Space or X | X |
| Chase / helmet camera | V | Y |
| Sound on / off | M | Start → Bike sound |
| Rain on / off | T | Start → Start rain / Stop rain |
| Dismount while stopped on the ground | E / Esc | A / B |
| Recover (last checkpoint during a race) | R | Start → Recover bike |
| Start / restart three-lap race | G | Start → Start / restart three-lap race |

## Wheelies

Build a little forward speed, then add throttle and pull back: **W + Down**, or
**RT + left stick back**. Rearward weight, the pull action and rear-tyre acceleration
unload the front suspension. The front wheel lifts through the same solver that
handles normal riding; its angle/height is not animated to a preset wheelie pose.
Feather throttle and rearward input to balance. **Space/X rear brake** brings the
front down; lean forward with Up / left stick forward. Excessive rotation can still
crash, and R / Recover bike returns you safely. Airborne pitch uses the same
pull-back direction. Neutral input retains the previous riding torque tuning.

## Racing Riley and Alex

While riding the orange bike, press **G**, or choose **Start / restart three-lap
race** in F1 / the controller Start menu. A three-second countdown places Riley
(blue) and Alex (green) on the grid. Throttle is ignored and the bike stays still during the countdown. Ride the
normal dirt loop for three laps; the HUD shows lap, place and time. Eight ordered
forward-crossing gates prevent reverse riding, line oscillation and infield
shortcuts from counting as full laps.

The rivals use the same suspension, tyres, steering and crash solver as the player,
with independent look-ahead steering, throttle and braking. Each rider chooses
corner speed, braking confidence and line from its own decision stream. They push
full throttle on straights, brake ahead of berms and correct pitch in flight.
Nearby bikes influence local passing lines; there is no player-gap speed boost or
shared pacing controller. Overcommitting can miss a turn, hit something or cause a
physics-driven fall. A recovery makes only that rider more cautious. They take the jumps and berms;
they are not animated along a rail. They can collide or fall, then recover at their
last checkpoint after four seconds if stuck, waiting if their recovery position is occupied. R rejoins you at your last cleared
checkpoint without resetting your laps or the opponents. Opponents keep racing
while you are down. Your result time freezes on finishing; rivals continue to finish
and brake to a stop. G starts a fresh event without adding duplicate riders.

Menus pause the whole event. Resume riding input to continue after closing a menu;
dismounting cancels the event. This first event is local, three riders, three laps,
and the authored course. Passing is local obstacle avoidance rather than
strategic defending or multiplayer racing. Race riders are runtime actors,
not extra authored track assets. Only the player's engine has synthesized audio.

Holding the service brake at rest engages a walking-pace push assist (about
4 km/h on level ground), shown as **R · PUSH** in the HUD. Release to cancel it;
RT/W returns to forward drive. Rear brake alone, overlapping throttle/brake,
airtime and crashes never engage reverse. This is an accessibility/unsticking
assist, not a reverse gearbox fitted to a real motocross bike.

The native sun shadow uses a fixed light basis snapped in light space, filtered
depth comparisons and a faded local boundary. Looking around no longer shifts
the shadow focus. Flat lanes and underlying grass have separate render depths
to avoid coplanar flicker; physics contact heights are unchanged.

The default chase view follows a visible rider with helmet, goggles, jersey,
boots and bent riding posture. The bike has rounded tyre profiles and raised tread, spoked rims, angular radiator shrouds, a long gripper seat, flat race number plates,
alloy frame spars, fork guards and a high-mounted silencer. Modern race bodywork
is authored in the package; the older exposed tank and round trail headlamp are removed. The rider's shaped jersey, tapered
limbs and helmet move with rider input and suspension travel. Bounded two-bone
poses keep the hands on the steered grips and the boots on the pegs; elbows and
knees bend without stretching the limbs. Torso normals follow its weight shift.
The visible chassis tilts around the axle midpoint, matching the solver's
projected wheelbase without lifting both wheel centres during pitch or lean.
Suspension and raked steering remain visual articulations over that shared rest
geometry, not a full constrained-linkage simulation.
Replacement bike assets must preserve independently addressable chassis, steering
assembly, lower fork, front wheel, swingarm and rear wheel groups. These may share
one asset file and material atlas; fixed details need not become separate draws.
The current package model uses part identifiers and joint transforms, with wheel
centres at local `(0, 35, ±74)` cm, +Y up and +Z forward. A detailed external model
must be aligned and split at those moving boundaries before replacing the rest
geometry. Keep authored normals and UVs with each part; do not flatten steering,
suspension or wheel pivots into a static mesh. The rider currently uses bounded
joint-based poses rather than an imported skinned-character asset.
Front and rear wheels spin independently; the lower fork and swingarm follow suspension travel,
and a bounded dust trail follows the supported rear tyre at terrain height.
It stops emitting when the rear wheel is unloaded or airborne; existing dust
fades out. A grounded wreck uses its sliding body contact instead. Helmet view remains available.
The chase camera smooths movement independently of physics and checks scenery
between the bike and both the desired and smoothed camera positions. Obstructions
pull the camera inward immediately; clearing them restores the distance smoothly.
Two bounded sweeps also keep it above the sampled riding surface. Right-stick or mouse look holds
for two seconds after input stops, then smoothly returns behind the bike (or
forward in helmet view). Looking does not steer or shift rider weight. Menus pause
the return timer; boarding, recovery and camera switching reset the view.

This remains assisted motocross riding, not a calibrated motorcycle engineering
simulator or a claim of MX Bikes parity. A six-speed automatic gearbox now has
shift hysteresis, a 120 ms torque interruption, throttle response and a broad
torque curve. Engine braking slows the bike when the throttle is released. The
HUD shows the actual simulated gear/RPM, and exhaust synthesis uses that same RPM.
Suspension stiffens toward the end of its travel and uses separate compression
and rebound damping. Terrain samples follow the axles' projected spacing as the
bike pitches. Ground-relative damping includes axle movement from pitch and yaw;
the same pitch lever produces suspension torque. This keeps steep takeoff and
landing response consistent with the two-axle geometry. Each axle now uses a
35 cm rigid rolling tyre envelope: five terrain samples across its diameter,
with circle support solved against their four line segments. This begins lifting
the tyre before its centre reaches a ramp foot and carries support over crests.
The front envelope follows its steering angle, while the rear follows chassis
heading. The next-step damping sample includes the front steering response.
Lean-dependent contact shape, tyre deformation and a full multibody chassis
remain outside this approximation. Features narrower than the 17.5 cm sample spacing can be missed;
interpolated seams are approximate, while a constant slope has exact circle
clearance. Ground-relative damping includes rotation of the sampled footprint.
Rider weight affects load distribution; lean has damped
angular momentum, including limited rider roll correction during airtime. Steering leans the bike automatically; tyres regain lateral grip
over rolling distance through separate transient contact-patch slip states.
Tyre grip follows each suspension's actual support force, including unloading
over crests and transient compression on landing. A nearly unsupported wheel
cannot retain its static traction. A bounded impact capacity prevents a single
bottom-out impulse from producing unlimited cornering or braking force.
Dry-surface grip follows each wheel's contact material. Prepared lanes, jumps
and berms retain the established dirt tuning; grass uses 88% and rock 96% of
that available tyre force. These are modest assisted defaults, not measured
soil coefficients or a wet/deforming-terrain model. Flat authored dirt lanes
also mark the two straights, and the chase/helmet HUD distinguishes dirt, grass,
rock and airtime. Decorative meshes do not override the ground contact material.
Front/rear load transfer affects grip, acceleration lifts the nose
and braking compresses the front suspension. The front wheel, forks and bars rotate around the raked steering stem. Separate
front/rear contact velocities generate lateral forces and yaw torque, so the
front guides the turn and the rear follows with finite angular response. The
front contact's slip, braking and friction budget are resolved in its steered
wheel frame, then rotated back into chassis force and yaw. Cornering therefore
includes tyre scrub, and front braking follows the angled wheel rather than
acting straight through the chassis. This coordinate change preserves contact
power; it does not add steering energy. Brake impulses are limited by contact
velocity and yaw response, so holding both triggers at rest cannot generate
braking jitter. Rear braking also accounts for the current drive request to
avoid alternating between creeping and stopping. The
rider shifts into the lean and their arms follow the bars. Steering builds progressively,
with full lock at low speed and wider requested turns as speed rises. Tyres
share a finite friction budget between accelerating, braking and cornering.
Combined force requests scale together, so full throttle cannot consume all
rear cornering grip before the bike even starts turning. The
rear brake reduces lateral grip for slides. Airborne steering
has no tyre traction, but existing damped yaw continues rotating the bike after
takeoff. Rider pitch changes the landing attitude. Airborne wheel angular momentum also matters: throttle raises the nose and a rear-brake tap lowers it. The current gear limits free-wheel speed, and a stopped wheel cannot keep producing brake torque. Automatic clutch remains an assist. Finite
suspension travel resolves bottomed-out landings before they penetrate the
terrain. Fast obstacle impacts, excessive landing impact speed, landing off balance
relative to the local pitch and sideways slope, and sustained loss of cornering grip cause a crash.
Low-speed bumps and clean landings remain forgiving. A completed backflip is
judged by the bike's orientation relative to the landing slope and its impact
speed, not accumulated rotation. Inverted and badly under-rotated landings still
crash. Grip-loss detection requires sustained actual sideslip rather than an
unmet steering-force request; ordinary powered turns retain rear traction.

A crash cuts the engine, plays an impact, slides/tips the bike and throws the rider
beside it. The view temporarily switches to chase so the fall stays readable. The
banner names the cause. Press **R / Start → Recover bike** to return to the boarding
point; your chosen helmet/chase view returns. The fall is a bounded procedural
animation with a collision-checked bike slide, not a general ragdoll simulation.
The detached rider settles against sampled terrain bounds on the fall side.
This is conservative ground clearance, not articulated limb or obstacle collision.
Finishing the pose does not stop gravity: an airborne wreck continues falling
until contact before the simulation becomes idle.
Riding input cannot move the settled wreck. Menu/focus suspension pauses the fall;
returning control resumes only the unfinished animation, not held throttle.
Menus, focus loss and hidden views pause simulation and sound without accumulating
catch-up time. Controller input must return to neutral after menus, including
both triggers independently. Car and spacecraft controls retain their bindings.

## Sound

Original procedural synthesis supplies powertrain-driven engine revs,
wind, tyre skid/gravel and landing impacts. Airborne bikes suppress gravel noise,
even if a stale slip value remains during a contact transition. One optional SDL2 playback device
produces mono 48 kHz float audio with at most 64 ms queued and six blocks filled
per timer tick. Cockpit uses the same optional SDL2 runtime as its controller
support. No audio samples, model downloads, microphone or runtime network access are involved.
An unavailable device leaves riding operational with a status message. M and the
controller menu mute the bike. The stream pauses/clears on suspension or exit,
and its device closes when the view is destroyed.


## Materials and rendering

Moto uses a native geometry pass inside the existing Qt OpenGL viewport, with
hardware depth testing, smooth vertex normals, a local directional shadow map,
hemisphere lighting, surface-normal maps, roughness-controlled sun highlights,
mipmaps and bounded anisotropic filtering. Bike metal and painted panels have
distinct native finishes; clothing and rubber retain rougher textured surfaces.
Metal uses coloured reflection and a simple sky/ground hemisphere approximation,
without live scene reflections or extra reflection targets. The painter fallback
retains baked colour shading. This is not a full calibrated PBR pipeline.
Native 2×/4× multisample antialiasing smooths geometry edges. It preserves the
physical resolution, falls back safely for incompatible Qt framebuffer formats,
and respects a 256 MiB colour/depth-target budget; large viewports can use no MSAA.
When samples are available, foliage and lettering use alpha-to-coverage in that
same buffer, smoothing their filtered texture edges while retaining depth tests.
The resolved scene stays opaque. Without MSAA, cutouts keep their hard threshold;
the local shadow map also retains its existing threshold. No extra target,
supersampling, temporal history or image blur is introduced.
It reuses the existing scene, vehicle, navigation and interaction systems.
There is no separate game engine or new runtime graphics dependency. Native
world signs keep their text upright and preserve its aspect ratio within the
sign face; lettering is not stretched to fill tall panels.

The native pass currently handles outdoor textured scenes, including ordinary
furniture and placement previews. Scenes with live app streams, display surfaces
or network connections retain the existing painter renderer. Devices without
OpenGL 3.3 also retain the painter path. This is an incremental rendering upgrade,
not a claim that every Cockpit environment has moved to native geometry.

Dirt, rock, grass and a cloudy-sky panorama are bundled CC0 Poly Haven assets;
[asset sources, authors and checksums](../resources/materials/README.md) record
provenance. Native surface tiles are shared at 1024 pixels. The sky JPEG is
decoded at 2048 × 1024 before allocation and projected from the camera direction;
it is a tonemapped panorama, not HDR image-based lighting. Rubber and fabric use
original procedural patterns. Native bike/rider texture coordinates are anchored
to the rest model, so weave and rubber detail follow articulation and wheel spin.
Terrain coordinates stay aligned across the ground. Dirt, grass and rock blend
two differently oriented scales of their shared photographs, with gentle
world-space colour variation to reduce obvious repeating grids. Matching normal
and roughness maps follow the same blend and orientation. This adds material
sampling, not textures, geometry or camera-dependent noise. Three shared 1K packed data tiles use 12 MiB of CPU
memory and the existing 64 MiB GPU cache; roughness alpha never makes the ground
transparent. Filtered normal variance softens distant highlights. Surface detail
changes lighting rather than collision geometry. There are no runtime asset downloads or neural rendering
calls while riding.

The bike combines original procedural wheels, frame and rider with selected
CC0 Innerscene bodywork, simplified offline to 1,260 triangles and adapted to the
motocross frame. [Model provenance and the reproducible conversion recipe](../assets/models/moto/SOURCE.md)
are included. The existing bounded package asset loader reads these surfaces;
Python and the offline simplifier are not runtime dependencies. Missing bodywork
assets retain procedural fallbacks.

Mesa Moto places up to 72 trees, including a perimeter woodland belt. Native
rendering instances one shared procedural pine with 2,512 triangles: tapered
trunks/branches and fixed intersecting needle sheets. Two shared CC0 photographic
textures supply bark and needle detail; they are bundled offline with
[provenance](../resources/materials/README.md). This is a lightweight decorative
approximation, not a full photogrammetry model. Native rendering depth-tests the
branches and casts cutout shadows onto scene geometry. The painter fallback
retains camera-facing juniper cards and a shared flat-ground shadow silhouette.
Placement reserves the full rotating canopy footprint plus 50 cm
clearance, checks visible solids, projected terrain, track/ramp footprints,
displays, vehicles, interactions and spawn/exit routes, and reserves five metres
around labelled landmarks. Each tree gets at most 65 candidates within 32 metres;
if none fits, it is omitted. The same admission runs on scene entry. Walking and
riding use small derived trunk colliders without drawing brown boxes. Furnishing
placement also rejects intrusion into the canopy envelope. The scene-wide
foliage contract is 96 trees. The native base mesh is uploaded once per frame,
with small position/scale/heading/shade records for its instances; each texture
batch uses one instanced draw call. Geometry and images are not copied per tree.
The renderer bounds all instances together at 96, at most eight shared groups,
and 8,192 staging vertices per base mesh. General transparent instances are
rejected; opaque and alpha-cutout instances participate in both depth passes.
Near the track, spare instance slots add up to 24 small grass patches inside
already admitted tree footprints. One shared 960-triangle tuft mesh uses opaque
geometry without additional textures or hidden collision. Its blades shrink
smoothly into the ground between 55 and 80 metres; trees and grass together
remain within the existing 96-instance limit. The painter fallback is unchanged.

The native pass shares a 64 MiB GPU texture cache across its material/image
batches, with at most 512 batches and 196,608 staging vertices per frame. One
2048-square depth shadow map follows the observer, and one colour/depth target
matches the physical viewport, bounded at 7680 × 4320. Textures use mipmaps and
up to 8× anisotropy. Context teardown and explicit release discard the targets,
buffers and textures; resize recreates only the viewport target. Smooth normals
follow articulated wheels, steering, suspension and the rider pose. Scene meshes
also retain authored smooth normals, including through rotated streamed passages.
Missing or malformed normals fall back to face shading. The track hills use
area-weighted normals on the same bounded mesh, without extra triangles or images.

The painter fallback retains perspective-correct world-aligned tiles, BSP and
near-plane clipping, offscreen rejection, vector clip paths and a shared 64 MiB
tinted tile cache. Its 512-pixel tiles have five mip levels. The measured native
resolution Moto working set is approximately 46 MiB, so turns do not repeatedly
recreate and upload evicted tiles. Closed model faces can reject back faces;
open sheets retain two-sided drawing.

Native display resolution remains the default, including HiDPI. **Start / F1 →
Resolution: Native → Reduced** explicitly selects a reusable raster layer capped
at 1280 × 800, which looks softer. Returning to Native or replacing the world
releases that reduced layer. Unsupported native frames fall back to the painter
without changing navigation or physics.

This is still an assisted, bounded motocross prototype. Terrain deformation,
calibrated tyre data, full rider ragdolls, temporal antialiasing and DLSS are not
implemented. DLSS has a Linux Vulkan path, but this OpenGL renderer does not yet
supply the required API integration and temporal inputs. An ordinary resize
filter is not presented as DLSS.

## Reuse and budgets

`DirtBikeDynamics` owns a deterministic 120 Hz solver: two spring/damper wheel
contacts, gravity, traction, separate service/rear braking, drag, slope forces
and pitch momentum, including bounded airborne wheel reaction. Front/rear contact-patch slip builds over a 25 cm relaxation length with an 8–40 ms time constant; these are assisted defaults, not measured tyre data. Approximate load transfer uses a 65 cm centre of mass above
the contact plane and a 148 cm wheelbase. Front/rear friction circles constrain
braking/drive and lateral force; these coefficients are tuned for assisted dirt
riding, not a calibrated tyre model. Collision crashes begin above 650 cm/s;
hard landings exceed 1,800 cm/s relative to the ground. Off-angle impacts above
450 cm/s and 0.78 radians can crash. Pitch/roll errors are measured modulo a full
turn; errors over 1.25 radians also reject gentle upside-down landings. Grip loss
requires actual sideslip over 0.65 radians above 650 cm/s for 0.65 seconds.
The fall pose settles over 2.4 simulated seconds; an airborne wreck continues
falling until ground contact before its update loop stops.
A frame advances at most six fixed steps (50 ms). Suspension
damping uses motion relative to the terrain beneath each wheel. Each active step
uses six five-sample tyre envelopes, two bank samples and one chassis floor
sample: at most 33 height queries, each bounded to 16 admitted ride surfaces.
The envelope uses stack storage and adds no geometry, heap allocation or steps.

`SpatialRideSurface` is a shared contact/render profile: a width, heading and
2–8 increasing distance/height knots, beginning and ending at ground level.
Scenes admit at most 16 surfaces, 800 cm elevation and 2,000 cm width. The same
profile creates the visible ramp and supplies walking/wheel ground height.
An optional signed radius (1,000–8,000 cm, at least the width) bends a sculpted
surface through at most a semicircle. Banking raises its outside lane; terrain
height queries use the visible triangles, including mirrored and rotated areas.
The bike fits its bounded rolling envelope over these terrain samples; walking
continues to use the original point height.
The track uses this for its two berms. Banking contributes downhill lateral
acceleration and changes supported grip; the stopped-bike assist holds ordinary
banks, and landing balance is measured against the local sideways slope.
The default linear mode is unchanged. The authored Moto track selects sculpted
surfaces: four monotone-Hermite samples per knot span, then linear triangle contact
and tapered shoulders. The visible shoulder triangles and contact heights agree,
including between vertices. At most 2,688 cached ride-surface faces are admitted
for sixteen maximum-size sculpted profiles, independently of ordinary scene meshes.

The course ribbon and textured slopes fit the existing aggregate
512-vertex / 256-face ordinary mesh limit (344 vertices / 223 scenery faces on
Mesa Moto). Track solids remain below the existing 96 limit.
Bike/rider geometry is cached and capped at 4,096 faces per vehicle; existing car
models remain below their previous 224-face size. A scene still admits at most
12 vehicles. Only the ridden bike animates, reusing face storage, plus one ground
shadow and at most 24 fading dust quads. No external physics or rendering engine
or continuous neural inference is required.

Spatial Web recognizes explicit motocross/dirt-bike/enduro evidence in admitted
vehicle listings and uses the same enterable bike. Generated construction plans
can supply validated ride surfaces; the default web planner does not turn every
motorcycle page into a race track. Product geometry is approximate and is not a
manufacturer model.

`cockpit-test --dirt-bike` covers suspension, jump landings, frame-rate independence,
collision, recovery, separate proportional brakes, combined-brake stopping, audio
synthesis bounds, missing audio devices, playback lifecycle, geometry budgets,
camera switching, trigger-driven riding and web planning. Crash regressions cover
fast versus slow collisions, hard and off-angle landings, sustained grip loss,
clean track jumps, front load/compression under braking, engine cut, wreck settling,
ignored riding input, pause/resume, original camera restoration and recovery. Additional coverage verifies bundled textures/mips, finite non-degenerate model
faces, exact straight/curved triangle contact, mirrored berms, a controlled banked turn,
stationary bank balance, rider joint continuity, gear/RPM bounds and shift stability,
engine braking, and rider weight response. Turning coverage checks stationary steering without
chassis rotation, mirrored corner trajectories, correct inward lean and a fixed
steering-stem pivot. The input-controls
fixture checks the widget's controller routing, camera action, menu trigger-neutral
guard and suspension cleanup. Audio tests use SDL's silent dummy driver.
`DOCKPIPE_BIKE_CAPTURE=/tmp/moto cockpit-test --dirt-bike` captures the pit,
rider, parked bike and course overview. Native software rendering verifies
presentation; it does not establish GPU frame rate, speaker quality or physical
controller ergonomics.

For an explicit host-GPU visual check, set `DOCKPIPE_BIKE_HARDWARE=1` and a
`DOCKPIPE_BIKE_CAPTURE` path prefix. This runs the deterministic physics cases and
renders pit/rider/overview views into real GPU framebuffers using unmapped,
non-activating windows. It checks nonblank pixels and the active hardware context;
it does not claim mapped-window gameplay FPS. Standard and nested-display runs
continue to exercise timed interaction, crash recovery, audio lifecycle and
browser transitions. GPU tests use silent SDL audio just like other fixtures.

`spatial-gpu-test` separately checks native depth ordering, alpha cutouts, sky
orientation, multisample edge coverage, painter composition, invalid/over-budget
geometry, shared instance transforms, UV/mask orientation, foliage bounds,
texture-residency exhaustion, eviction and recovery, resize, two independent
contexts, and release/recreation after the
context is no longer current. It reports a skipped
test when no suitable GL context is available. Hardware Moto captures assert that
the native pass remains active during riding, crashes and furniture previews.
`DOCKPIPE_BIKE_SPIN_BENCHMARK=1` adds a bounded spin/recovery timing run; its hidden
FBO timings are renderer measurements, not mapped-window gameplay FPS.
`DOCKPIPE_BIKE_PHYSICS_ONLY=1` runs the deterministic solver/model contracts
without the interactive wall-clock lane, allowing instrumented builds to check
those contracts without changing normal UI test deadlines.


The modern procedural race-bike pass uses the package's [mesh/material/articulation
tooling](mesh-tooling.md). Native captures also include a parked bodywork view
(`-bike-detail.png`) and a three-quarter articulated rider view (`-rider-detail.png`).
This remains an approximate original asset with rigid joint overlaps, rather than a
scanned or production-skinned motorcycle/rider model.


Rider view now shows the same generated 3D handlebars, grips, levers, forks and
front wheel as chase view. Steering rotates the assembly around its raked head
joint; rider hands follow its grip anchors. The old fixed handlebar illustration
has been removed. See [the generated assembly contract](mesh-tooling.md#generated-front-assembly).

The [wheelie and rival-race validation checkpoint](wheelie-race-validation.md) records
the physics, full-lap AI proof, native captures, performance sample and limits.

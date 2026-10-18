# Reusable terrain, soil and surface water

`SpatialTerrain` is the shared package-owned terrain primitive. It has no bike,
RedBud, race, UI or renderer dependency. Content supplies a configuration and a
height/soil-mask seed callback. The scene's vehicles and walking queries use one
sampler; wheel contacts mutate that same scene-owned soil field. RedBud's map
tracing, hill functions, jump placement and weather rates live in `RedBudCourse`
and `RedBudTerrain`, outside the terrain implementation.

## Contract

Units are centimetres and seconds. `create(configuration, seed)` rejects invalid
or over-budget lattices before allocation and rejects non-finite seed data.
The base lattice holds elevation, soil coverage and water depth. The finer soil
lattice holds compaction depth. `sample(point)` returns height, rut depth, water,
wetness, soil coverage and a relative grip factor. `wheelContact` accepts a contact
position/direction, rolled distance, normalized load and slip; it does not know
which vehicle generated them. Unsupported or stationary contacts do not cut ruts.
`advance` updates wetness and bounded shallow runoff independently of rendering.

The current capacity is 36,225 base nodes and 5,165,569 soil samples: about
20 MiB of soil and 0.6 MiB of base/flow storage per admitted terrain. Resolution
and plot shape are configurable within those aggregate limits. RedBud uses
240 cm base cells and 20 cm soil samples over 537.6 × 384 m. Height seeds are
bounded to ±100 m. Each simulation advance admits at most 50 ms; hydrology updates
at about 5 Hz. Each contact visits at most 25 soil samples. Rut depth saturates
at 16 cm and further compaction slows as the soil packs down.

Water transport moves a bounded fraction toward lower neighboring free surfaces
using a separate accumulation buffer, so traversal order cannot create water.
Rain, infiltration, open boundary drainage and overflow above 40 cm are sources
and sinks. Fine tyre channels retain water when saturated; that sub-cell retention
is an approximation rather than a second fine fluid solver. Wetness reduces grip,
water increases rolling resistance, and wetter/slipping tyres cut soil faster.

## Scene and rendering integration

`SpatialInterior` owns a terrain pointer. Scene admission clones the runtime field;
actors within that scene share it, while separate views do not mutate each other.
Qt vectors share backing storage until the first mutation. Bike and race recovery
retain the field. Re-entering the world starts fresh terrain; disk persistence is
not implemented.

Terrain scenes project admitted ground vehicles and foliage onto the surface.
Local vehicle models and the existing flat-room authored contracts remain intact.
Walking, wheel support, suspension, local slope forces and camera ground clearance
query the same terrain. All racers still use ordinary player-physics inputs.
[Return portal placement and entry](return-portals.md) also follow terrain height
and keep the opening clear of actual scene arrivals.

The existing native and painter backends consume generated triangles/quads.
Rendering uses coarse landscape cells, base-resolution road cells and 20 cm
contact detail within 8 m of the camera. Soil-mask contour clipping creates track
shoulders. Puddles are separate muddy water polygons over retained ground;
water-depth contours clip their boundaries. The physics sampler follows the same
triangle diagonal as the fine mesh. Distant geometry uses level-of-detail
approximations; visible wear can lag the solver by up to roughly 200 ms.

Terrain buffers are reused during refresh. The native aggregate geometry,
texture and render-target limits are unchanged. A content builder must budget
terrain, water, riders and scenery together; a larger accepted height field does
not waive renderer admission. `SpatialTerrainTest` checks the RedBud scene's
headroom against the existing three-rider capacity.

## Scope

These are reusable engine components inside Cockpit. Another game can supply a
different terrain seed and wheel-contact producer. The bike is currently the only
integrated vehicle deformation producer; other vehicle families still need that
integration. Surface textures currently use the shared grass/dirt palette with
configurable colors. There is no general editor, DEM importer, scene persistence,
sediment transport, soil mass redistribution, calibrated tyre/soil mechanics or
full fluid dynamics. Dockpipe's command/workflow engine is unchanged.

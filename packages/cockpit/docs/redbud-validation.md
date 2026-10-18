# RedBud terrain checkpoint — 2026-10-16

The default Moto destination now opens a RedBud 2024 map study. The original Mesa
oval remains a deterministic fixture. Source reference: the official
[2024 Version 2 track map](https://www.americanmotocrossresults.com/xml/mx/events/M2430/trackmap.pdf).
The [venue](https://redbudmx.com/pro-motocross-national) describes RedBud's rolling
Michigan setting. The map is schematic: horizontal scale, elevations, banking,
jump profiles and scenery here are authored approximations, not survey data or a
certified replica. No official artwork, logos or map PDF is shipped as an asset.

The traced loop is approximately 2,141 m long, with about 14.15 m of authored
course elevation variation. Switchbacks, straights, rollers and a rollable western
triple replace the flat oval in the public Moto destination. Terrain-grounded
vehicles, trees and flags use the existing scene, generated riders and renderer.

## Reuse boundary

`SpatialTerrain` owns height sampling, bounded deformable soil and shallow runoff.
It accepts content-supplied seeds and generic wheel contacts. It does not import
RedBud, bike, race, UI or rendering classes. `RedBudCourse` and `RedBudTerrain` own
the map trace, landscape/jump authoring and accelerated weather configuration.
See [reusable terrain contracts](terrain-system.md).

This is a bounded engine-component implementation inside Cockpit. It does not
rename Dockpipe's command engine or introduce track knowledge into `src/lib` or
`src/cmd`. No worktree, commit, push, publication or preview restart was performed.

## Verification

- Modern-web `cockpit-test` and `dockpipe-launcher` builds passed.
- Final affected CTest run: six passed, one skipped, zero failures in 31.63 s.
  Passed: spatial-terrain, dirt-bike, spatial-vehicles, input-controls,
  flight-dynamics and spatial-civic. Spatial-web-browser explicitly skips under
  the offscreen Qt platform; that browser-host lane remains unqualified here.
- A separate synthetic basin exercises the same terrain API without RedBud.
  Unsupported/stationary contacts cause no wear; repeated loaded passes create
  a bounded rut; wet passes cut more deeply; adjacent ground stays unchanged.
  Rain produces substantially more water in the low basin than uphill. Stopping
  rain drains water gradually and preserves rut depth.
- An actual bike over wet terrain reaches 399.92 cm/s after the same one-second
  throttle input that reaches 645.12 cm/s on dry terrain, without either crashing.
  The grip/drag effect therefore reaches the physical solver, not just the HUD.
- Fine mesh vertices and interior triangle contact heights match the shared
  sampler. Invalid seed data, over-budget resolution, non-finite timestep and
  invalid camera position have bounded failure behavior.
- RedBud centreline projection round-trips 128 course samples. Elevated scene
  admission, bike entry and race start pass; scene admission clones terrain state.
- Both physical NPCs complete three ordered laps: 529.94 s / two recoveries and
  568.78 s / four recoveries in the qualified dry seeded event. They share the
  changing ground while retaining separate controllers, control inputs and risk
  streams. Full wet-event race completion is not yet separately qualified.
- The dry terrain sample packs a conservative 72,888 vertices including shoulder
  clipping and distant ground apron. The test reserves three full rider models
  plus scenery headroom under the existing 196,608-vertex native limit.
- Hidden native 3840×2160 inspection passes dry/wet terrain, three-rider racing,
  real contact-generated wear, rain toggle, pause and native admission. Prepared
  wet/worn capture states advance the same terrain API for 90 simulated seconds
  and apply repeated wheel contacts; these images are renderer proof, not a claim
  of a 90-second mapped interactive test.

The final 36-frame hidden native 4K sample measured render median/p95
15.30/26.98 ms and combined terrain/bike update p95 12.02 ms. Native timing
and final image inspection are recorded in
`/tmp/forgepipe-redbud-native.log`. Captures include
`/tmp/forgepipe-redbud-redbud-dry.png`, `-race.png`, `-wet.png` and `-ruts.png`.
Build, CPU and regression receipts are `/tmp/forgepipe-redbud-build.log`,
`/tmp/forgepipe-redbud-terrain.log` and `/tmp/forgepipe-redbud-regressions.log`.
All generated PDFs, PNGs and logs remain temporary; they are not shipped assets.

## Limits

Ruts persist for the current world visit, including race restart and recovery;
world re-entry resets them. No disk save/load is implemented. Rain accumulation
and wetting are accelerated for short playable sessions. Water is coarse shallow
runoff with infiltration, boundary drainage and overflow, plus approximate fine
rut retention. There is no full fluid solver, calibrated hydrology or soil mass
redistribution. Rain streaks are presentation particles; only the water/soil
field affects contacts. Surface water uses opaque muddy polygons rather than
refraction or a dedicated water shader. Far terrain uses level-of-detail
approximations; generic content still needs aggregate scene admission.

The larger landscape costs more to render than Mesa. Native framebuffer samples
qualify that path, not mapped gameplay FPS. Controller ergonomics, full wet-race
completion, measured replica elevation, platform release qualification and editor/
import/persistence workflows remain future work.

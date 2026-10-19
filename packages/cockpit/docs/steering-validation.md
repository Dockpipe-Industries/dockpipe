# Generated bike steering checkpoint — 2026-10-16

Objective `forgepipe-bike-steering-2026-10-16`: completed for the user-requested
steering assembly and model/physics alignment pass. The saved checkout and running
preview were preserved; no commit, publication, worktree or system change was made.

## Cause and implementation

Rider view previously hid the active 3D bike and drew a fixed 2D handlebar graphic.
The chase model could steer while those controls appeared fixed. Both renderer paths
now show the generated front assembly and attached rider arms/hands in rider view.
The old graphic is removed. A shared native-resolution lens and chassis-following
eye anchor keep the controls in frame without detached sleeve ends.

[DirtBikeSteeringRig](../src/DirtBikeSteeringRig.h) owns the raked head joint, fork
centrelines, axle and grip anchors. Generated upper/lower forks, triple clamps,
head tube/shaft, bars, grips and levers use those anchors. Frame/head tube remain
fixed, while bars/forks/wheel/plate/fender rotate together. Rider IK uses the same
grips. The tyre model uses the ground-plane rolling yaw projected from the raked
joint, with independent analytic contact coverage for that projection.

## Validation

- Modern-web `cockpit-test` target build passed.
- Final CTest lanes passed 4/4: dirt-bike, spatial-vehicles, input-controls,
  flight-dynamics. These include rig pivot/length/contact invariants, finite input
  handling, tyre support, rider continuity, stationary steering and crash recovery.
- Hidden native 3840×2160 NVIDIA RTX 3080 captures show neutral/left/right steering
  in rider, chase and front three-quarter views. Images were visually inspected;
  parked position remains within 0.5 cm and steered images materially differ.
- Bounded 36-frame stationary samples: rider median/p95 3.82/4.84 ms, chase
  4.79/6.07 ms. Final moving sample: median/p95 4.99/5.95 ms; 90-frame spin/crash
  sample: render median/p95 5.14/6.05 ms, geometry-update p95 1.66 ms.
- Final model: 4,086 faces under the unchanged 4,096 cap. Source whitespace and
  documentation links passed. Engine/package boundaries remain intact.

Local receipts: `/tmp/forgepipe-steering-build.log`,
`/tmp/forgepipe-steering-lens-native.log`, `/tmp/forgepipe-steering-moving-native.log`,
`/tmp/forgepipe-steering-regressions.log`. Final steering images use
`/tmp/forgepipe-steering-lens-steering-`; moving captures use
`/tmp/forgepipe-steering-moving-`. These are temporary proof, not shipped assets.

## Limits

Timings qualify hidden native framebuffers, not mapped gameplay FPS. Steering and
lean remain assisted. Suspension retains vertical axle travel rather than a full
raked telescopic/contact solution; countersteering, calibrated tyre/steering torque
and physical controller ergonomics are not qualified by this pass. The bike remains
generated from procedural parts; no imported whole-bike asset or separate renderer
was introduced. See [the reusable assembly contract](mesh-tooling.md#generated-front-assembly).

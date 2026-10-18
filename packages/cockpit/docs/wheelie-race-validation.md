# Wheelies and rival racing checkpoint — 2026-10-16

Objective `forgepipe-wheelies-rivals-2026-10-16`: completed for the requested
pull-back wheelies and two rideable-course NPC opponents. Implementation remains
inside Cockpit; the existing preview, unrelated checkout work and engine were
preserved. No commit, publication, worktree or system change was made.

## Delivered

Pulling back with throttle shifts rearward weight and contributes a bounded rider
pull torque. Rear-wheel acceleration unloads the front suspension through the
existing fixed 120 Hz solver. Rear braking lowers the front; neutral input retains
the previous torque tuning. The model, wheel contacts and rider pose use the
resulting physical state, without a prescribed wheelie angle or animation.

G / the input menu starts a three-lap Mesa race against Riley (blue) and Alex
(green), with a three-second held grid, ordered forward checkpoints, live placing,
elapsed time and a finish result. Opponents use the player's solver, generated
articulated model and steering rig. Their bounded look-ahead controller follows
the authored course and brakes for berms. R rejoins the player's last checkpoint;
G restarts without duplicate actors. Menus pause simulation. Rivals can fall and
recover at their last checkpoint after four seconds if stuck. They continue to
finish and park after the player's result time freezes.

## Proof

- Modern-web `cockpit-test` build passed. Final CTest lanes passed 4/4:
  dirt-bike, spatial-vehicles, input-controls, flight-dynamics.
- Focused wheelie proof reached 0.356 rad pitch with front support 0 and rear
  support 0.983 static weights. Rear brake returned pitch to -0.039 rad and front
  support to 0.610 without a crash. Neutral throttle and stationary pull-back
  cases remain supported and stable.
- Both physical NPCs completed all three laps over the authored jumps and berms
  with zero recovery resets: Alex 165.60 s and Riley 170.40 s in deterministic
  simulation. Finish braking stops them without engaging push/reverse assist.
- Ordered-gate tests reject backwards laps, start-line oscillation and teleport
  shortcuts. Event tests cover countdown, placing, player victory, frozen result
  time, opponent completion after player finish, cancellation and restart.
- Hidden native 3840×2160 RTX 3080 captures were inspected for the wheelie, brake
  recovery, grid and moving three-bike race. The countdown keeps the player
  stationary; pause freezes event time; restarting retains exactly three bikes.
- A bounded 36-frame native race sample measured render median/p95 10.92/14.51 ms
  and combined simulation/geometry-update p95 4.27 ms. These are hidden framebuffer
  measurements, not mapped gameplay FPS.

Resources remain bounded: two rivals, three laps, eight ordered gates, 128 fixed
centreline search segments, capped simulation steps and the existing 4,096-face
per-model admission. Three rider models use 12,258 authored polygons in total.
The native geometry/texture/target admission limits remain unchanged. Generated
outputs are local temporary logs and PNG proof, not shipped assets.

Local receipts: `/tmp/forgepipe-wheelie-race-build.log`,
`/tmp/forgepipe-wheelie-race-cpu.log`, `/tmp/forgepipe-wheelie-race-native.log`,
`/tmp/forgepipe-wheelie-race-regressions.log`. Native captures use the
`/tmp/forgepipe-wheelie-race-` prefix.

## Limits

This is assisted riding with authored pull/weight coefficients, not a calibrated
motorcycle engineering model. The initial fixed-line/pace controller has been
superseded by the [independent rival racing checkpoint](rival-racing-validation.md).
Recovery still uses the last cleared checkpoint rather than strategic route finding. Racing is local to
the authored Mesa course; only the player has synthesized engine audio. Physical
controller ergonomics, mapped gameplay frame rate and platform-wide release
qualification remain unverified. See [controls and race rules](dirt-bike.md).

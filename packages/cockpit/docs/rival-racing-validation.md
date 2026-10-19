# Independent rival racing checkpoint — 2026-10-16

The low fixed pace in the first Mesa race has been replaced by a per-rider driver.
Implementation stays inside Cockpit; no engine changes, model budget changes,
commits or publication. The running preview was preserved.

Each rival owns its controller, seeded random stream, corner commitment, braking
estimate, lane choice and recovery caution. Live restarts choose a new event seed;
CPU qualification supplies fixed seeds. A rider's decisions never consume another
rider's random stream. Nearby bikes are sensed locally for passing clearance.
There is no shared pace, player-gap rubber banding, extra power, grip override or
scripted fall. Only ordinary throttle, steering, service brake and pitch controls
reach the same fixed-step dynamics used by the player.

A 3,000 cm horizon with twelve 250 cm intervals plans braking from straight-line
pace into the berms. Corner targets account for the raked front wheel and the
speed-dependent steering envelope. Per-corner confidence can overestimate the
available turn; position, slip, collisions and landing outcomes remain solver
results. In-flight pitch corrections also use normal rider inputs. A recovery
lowers only that rider's confidence. Rivals rejoin at separate lateral positions
at their own last cleared checkpoint, and wait if that position is obstructed.
This fixes the previous possibility of both recovering into each other.

## Verification

- Modern-web `cockpit-test` and `dockpipe-launcher` builds passed.
- Final affected CTest lanes passed 4/4 in 29.02 s: input-controls,
  flight-dynamics, dirt-bike and spatial-vehicles.
- Controller isolation: seven extra ticks per call and repeated recoveries for a
  second controller leave the first controller's steering/throttle/brake/pitch
  identical to an undisturbed reference. Different actors make different choices.
- Default deterministic three-lap event: Riley 120.39 s with one collision/recovery;
  Alex 95.19 s with no recovery. Previous fixed-pace results were 170.40/165.60 s.
- Four additional fixed seeds exercised eight physical riders: all finished in
  94.64–111.83 s, six without recovery. Two needed one recovery, including one
  solver-reported collision. Peak speeds were 1,942–1,964 cm/s (about 70 km/h).
  Overcommitment may also miss the line without falling. Tests require completed
  ordered laps, material pace improvement, bounded recovery and independent decisions.
- Existing countdown, finish parking without reverse, frozen player results,
  cancellation/restart, wheelie and ordered-gate contracts still pass.
- Hidden native 3840×2160 GPU inspection passed wheelie/brake, grid, three-bike
  racing, pause/restart and live corner capture. The captured green rival entered
  the berm at 1,337 cm/s with -0.468 rad lean and 0.069 rad steering. Both visible
  opponents lean and turn on separate lines.
- A bounded 36-frame native sample measured render median/p95 11.21/13.77 ms and
  simulation/geometry-update p95 4.64 ms. This is hidden framebuffer evidence,
  not mapped gameplay FPS.

Receipts: `/tmp/forgepipe-rival-pace-build.log`,
`/tmp/forgepipe-rival-pace-cpu.log`, `/tmp/forgepipe-rival-pace-native.log`,
`/tmp/forgepipe-rival-pace-regressions.log`. Temporary captures share the
`/tmp/forgepipe-rival-pace-` prefix, including `rival-corner.png`.

## Limits

This is local assisted racing on the authored Mesa course, with finite look-ahead
and local obstruction avoidance. It does not implement strategic defending,
learning, multiplayer or calibrated motorcycle engineering. Wipeouts are possible,
not guaranteed on a schedule; the qualified seed set produced a collision rather
than a tyre-grip or landing fall. Random live races can take different lines and
have different results. Opponent engine audio and mapped gameplay performance
remain separate work.

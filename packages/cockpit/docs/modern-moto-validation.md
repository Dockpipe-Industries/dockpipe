# Modern Moto validation checkpoint — 2026-10-16

Objective `forgepipe-modern-moto-2026-10-16`: completed for the bounded procedural
mesh/tooling pass. Authority was the user-approved implementation continued through
one requested handoff. Scope: improve the modern bike/rider, preserve articulation,
prove bounded native rendering, document reusable tooling and record the distinct
ForgePipe direction. Commit, publication, preview restart, system
changes and a whole engine implementation were excluded.

## Delivered

- [Shared loft construction](mesh-tooling.md), with atomic failure, admission before
  allocation, smooth side normals, hard end caps and preserved material/part metadata.
- Shaped clothing, boot/helmet shells, wrapped goggles, solid staggered tread,
  crossed spokes, engine covers/bolts, brake caliper, chain runs and exposed shock coil.
- Rest-space UVs and the existing steering, fork, swingarm, wheel and rider groups.
- [TASK-037](../../../docs/agents/tasks/forgepipe-game-studios/overview.md), linked to
  the Unreal guide and ForgePipe workflow UI without renaming or modifying core.

## Proof

The modern-web CMake build passed. Deterministic bike/model checks passed, including
new loft boundary/failure cases and existing rider contacts/crash clearance. The final
unmodified CTest lanes passed 4/4: dirt-bike, spatial-vehicles, input-controls and
flight-dynamics. Documentation links, YAML task routing and whitespace checks passed.

Native NVIDIA RTX 3080 rendering used hidden 3840×2160 framebuffers. Side bodywork,
articulated rider, cornering and settled crash captures were visually inspected.
The native pass stayed active during riding, crashes and furnishing previews.
The final model has 4,060 polygons under the unchanged 4,096 limit. In the bounded
36-frame turn sample, render median/p95 were 4.68/5.78 ms; in the 90-frame spin sample,
render median/p95 were 4.84/6.07 ms and geometry-update p95 was 1.61 ms.
These are local measurements, not mapped gameplay FPS or cross-machine guarantees.

Local receipts: `/tmp/forgepipe-loft-build.log`, `/tmp/forgepipe-final-native.log`,
`/tmp/forgepipe-final-regressions.log`; native images use `/tmp/forgepipe-final-`.
These temporary files are not shipped assets. The running preview was preserved.

## Limits

Original procedural geometry remains visibly simpler than MX Bikes. There is no
manufacturer asset, weighted skeletal skinning, cloth simulation, deforming chain
or animated shock coil. Rigid joint overlaps and approximate crash grounding remain.
Native scene admission still has an aggregate vertex budget in addition to the
per-model cap; this checkpoint measures one active bike, not twelve simultaneous
maximum-detail bikes. No mapped-window gameplay, physical controller, platform-wide
or production release qualification is claimed. Engine/package boundaries stayed
intact; no engine source was edited by this pass.

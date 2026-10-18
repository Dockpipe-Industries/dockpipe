# Safe return portals

Return portals use a bounded three-dimensional entry volume. Walking anywhere
behind an exit plane no longer returns to orbit. A movement segment must enter
the opening from outside; standing or spawning inside it, moving away, passing
far to one side or passing on another floor does not activate it. A fast movement
segment crossing the opening still counts.

Scene admission preserves a clear authored exit when it is at least four metres
from the actual arrival point. Otherwise a finite search prefers clear space
behind or beside arrival, inside room bounds and outside static obstacles. This
runs for both ordinary entry and streamed arrivals. If a cramped/crowded room
cannot provide that clearance, walk entry is disabled and explicit navigation
remains available. Portal height follows the local walking/terrain surface.

RedBud's authored return position is off the track and about 16.6 m from spawn.
The previous position equaled spawn. Combined with the former unbounded
behind-plane check, this caused the first forward step to return to orbit.

## Verification — 2026-10-16

- Before-fix regression: overlapping arrival followed by forward input fired
  the real scene orbit callback once (`/tmp/forgepipe-portal-before.log`).
- Fixed integration: forward input from an overlapping authored arrival causes
  zero returns. Moving far behind an exit plane stays in the scene. Deliberately
  walking into the actual opening returns exactly once.
- Actual RedBud scene: forward walking moved 50.1 cm and fired zero orbit returns.
  Software painting is disabled for this movement test; it qualifies navigation,
  not a new native render capture.
- Geometry/policy checks cover high-speed entry, height/lateral misses,
  stationary/inside arrivals, invalid radius, obstructed preferred placement,
  stable safe exits, streamed arrivals, cramped-room explicit navigation and
  terrain-grounded portal height.
- Modern-web `cockpit-test` and `dockpipe-launcher` builds passed.
- Final CTest lanes passed 8/8 in 39.54 s: return-portals, spatial-terrain,
  dirt-bike, spatial-vehicles, input-controls, flight-dynamics, spatial-civic
  and spatial-operations. Receipt: `/tmp/forgepipe-portal-regressions.log`.

All changes remain inside Cockpit. The running preview was preserved; restart it
to load the rebuilt fix. No commit, publication or new worktree was created.

# TASK-003 Managed Session Volume Cleanup And Retention

## Current State

- Core Dockpipe now auto-cleans managed session base volumes after successful workflow completion
  and after `dockpipe session publish` when the session uses `workspace.storage: volume`.
- Cleanup is runtime-owned and gated by preflight checks: managed worktree exists, local session
  branch exists and matches the workspace branch, local session branch metadata still exists, and
  no active worker lease still depends on the volume.
- Cleanup writes `session.volume.cleanup.preflight` and `session.volume.cleanup` operation-result
  lines/session events and records cleaned volume metadata in the session record.
- `DOCKPIPE_SESSION_VOLUME_AUTOCLEANUP=false` disables the default cleanup behavior when a user
  intentionally wants to keep the runtime-owned volume around.

## Still Open

- Add explicit inspect/list/prune CLI behavior for retained or stale managed session volumes and
  related workspace artifacts.
- Broaden cleanup evaluation to more failure paths so runtime-owned cleanup is not limited to the
  current successful workflow completion and publish paths.

## Planned expansion: artifact expiry and Settings cache management

The [canonical proposal](../../../architecture/artifact-retention-and-cache-management.md)
adds remote delivery/artifact expiry and product-wide cache management to this
backlog. This is future work; no cleanup or implementation is requested here.

- Configurable retention by job outcome, expiry visibility and pin/keep controls;
  protect active jobs and retain small execution records after outputs expire.
- Settings inventory and clear/prune actions for all Dockpipe-managed cache owners,
  with category, machine, location, size, reclaimable space and policy visibility.
- Equivalent CLI/configuration controls, scoped previews, bounded cleanup and
  owner-provided package cleanup capabilities; distinguish installed dependencies
  and user data from disposable caches.
- Native qualification for concurrent jobs, shared references, cancellation,
  disconnects, pins, partial failures and successful execution after cache clearing.

Coordinate shared dataset cache ownership with
[TASK-047](../paired-storage-mounts/index.yaml). Preserve existing session-volume
cleanup behavior while designing these additional policies.

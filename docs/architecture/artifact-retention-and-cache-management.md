# Artifact retention and cache management — proposed

Status: planned backlog scope, not implemented behavior. Tracked by
[TASK-003](../agents/tasks/session-volume-cleanup/index.yaml).

## Retention

Add configurable expiry for remote delivery workspaces and job artifacts, with
defaults that can distinguish successful, failed and cancelled jobs. Allow users
to pin retained jobs or individual artifacts and see when eligible files expire.
Never remove files belonging to active jobs or held by a live lease. Preserve
small execution records after bulky files expire, clearly marking expired outputs
and allowing reruns to acquire their inputs again.

Track inputs, temporary build files, outputs and shared caches separately so one
job's expiry cannot remove data still needed by another job. Define when each
retention clock starts and how retries, pins and clock changes affect eligibility.
Support explicit expiry, retention overrides and pinning through GUI, CLI and
workflow configuration; final field names and defaults remain design work.

## Settings and cache controls

Add a Storage and caches area in Settings covering Dockpipe-managed caches and
retained artifacts. Inventory existing owners before implementation, including
remote deliveries, package/download/resolver caches, build/tool caches, session
volumes and package-owned dataset caches where present. Distinguish disposable
caches from retained artifacts, installed dependencies and user-owned files.

Show category, owning package, machine, location, size, reclaimable space and
retention policy. Provide clear-by-category, prune-expired and clear-all-eligible
actions with a preview of scope and exclusions. Expose equivalent CLI operations
and configuration for expiry schedules and size budgets. Settings must explain
that clearing caches may require downloads or rebuilding on the next run.

Host Settings aggregates owner-provided inspection and cleanup capabilities;
package-specific paths and eviction logic remain with the owning package. Include
remote machines only through their authorized connection, with explicit target
selection and honest unavailable/offline states. Do not imply local cleanup also
cleans remote storage.

## Cleanup safety and qualification

Cleanup must respect active jobs, leases, pinned artifacts and shared references.
Use bounded, cancellable, idempotent operations; report partial failures and actual
space reclaimed. Validate ownership and paths without following symlinks outside
owned roots. Exclude credentials, pairing state, configuration, VM disks, source
checkouts and arbitrary user files from generic cache deletion.

Verify expiry and manual cleanup across supported hosts, restart recovery,
disconnect/reconnect, concurrent execution, pins, disk pressure and failed deletes.
Prove that clearing eligible caches permits a subsequent job to reacquire its
dependencies and run successfully. Preserve execution history and distinguish
expired artifacts from failed execution.

Coordinate worker cache policies with the
[paired storage proposal](paired-storage-mounts.md); mounted shared data has its
own ownership and retention policy and is not disposable merely because it is
mounted by Dockpipe. This proposal authorizes no live cleanup or implementation.

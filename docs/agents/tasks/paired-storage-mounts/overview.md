# TASK-047 — Paired connectivity, storage mounts and worker data caching

Related: [TASK-003](../session-volume-cleanup/index.yaml) tracks artifact expiry
and Settings cache management, including owner-provided cleanup for worker caches.

Status: **planned / future work**, added 2026-10-23. Ownership: unclaimed.
This task captures backlog ideas only. It does not authorize implementation,
network provisioning, mounts, dependency installation, credentials changes or
publication, and must not interrupt current refactor/VM/staging work.

The [canonical proposal](../../../architecture/paired-storage-mounts.md) records
the user flow and boundaries. Goal: add an optional package that provisions
storage access through existing paired workflow delivery, with shared CLI/GUI
setup, a local mount runtime and optional worker-local caching for data jobs.

## Dependencies and ownership

| Owner | Reuse and limit |
| --- | --- |
| [TASK-036](../remote-node-onboarding/index.yaml) | Existing consenting pairing, machine identity, delivery and revocation are prerequisites; no second connection stack |
| [TASK-043](../configurable-networking/index.yaml) | Shared network setup, provider connectivity, policy and diagnostics; connectivity does not grant share or desktop access |
| [TASK-046](../native-remote-desktop/index.yaml) | RDP/remote desktop consumes the connectivity foundation, has its own tab/client and session grants; mounts are independent |
| [TASK-044](../closed/ecosystem-extraction/index.yaml) | Optional provider/storage implementations stay in the external package owner with pinned dependencies; core remains generic |

## Planned stories

1. Inventory existing onboarding, Cloudflare authentication/provisioning, package
   setup/readiness and mount seams. Define the minimal shared provider contract.
2. Reuse paired delivery to run setup on the storage side and connecting worker.
   Offer first-use GUI setup and the same explicitly invocable CLI workflow.
   Guide authentication/enrollment and provision only authorized resources.
3. Add qualified native mount adapters, source/mount/access/lifetime configuration,
   secure credential references, idempotent readiness and owned cleanup.
4. Support dataset/model/checkpoint/media/artifact workflows using mount paths and
   environment configuration. Keep bounded SSD caching, prefetch/eviction and
   checkpoint transfer in the consuming data/training package.
5. Let storage and RDP reuse the connectivity setup without coupling their data
   paths or merging RDP into the VM tab. Coordinate navigation/tab refinement with
   TASK-046; keep shared mutable cross-OS checkouts out of the initial scope.

## Acceptance

- CLI and GUI use the same setup/runtime operations on each claimed OS.
- Reuse existing pairing without broadening grants or replacing identities.
- Prove actual provider routing, authenticated mounts, requested access mode and
  repeated setup without duplicate resources; report unsupported configurations.
- Cover revoked/expired access, permission denial, network loss, reconnect,
  cancellation and persistent versus job-owned lifecycle without unmounting
  operator-owned storage or disrupting unrelated work.
- Demonstrate a representative cached data job with bounded cache use and verified
  completed checkpoint transfer. Measure remote-storage limits honestly.
- Keep credentials out of logs/artifacts; no additional Cloud dependency in core,
  implicit public service exposure or automatic repository-sharing feature.

Implementation and external provisioning require a later bounded request. This
record changes no runtime or live machine state.

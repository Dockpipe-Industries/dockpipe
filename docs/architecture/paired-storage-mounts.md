# Paired connectivity and storage mounts — future architecture

Status: planned, 2026-10-23. Tracking:
[TASK-047](../agents/tasks/paired-storage-mounts/index.yaml).
This proposal records future work; it adds no shipped workflow/schema, dependency,
network route, mount, service or implementation authority. Keep the current
extraction, Windows VM validation and staging work independent.

## User flow and prerequisite

Start with an existing consenting, paired Dockpipe worker and working workflow
delivery. Reuse [remote onboarding](../runtime/remote-nodes.md), machine identities,
revocation and delivery rather than creating a second pairing or agent stack.
Pairing provides the control path for setup on both machines; it does not itself
create a storage route or authorize access to shares or desktops.

Installing an optional storage package adds setup workflows and its runtime.
On first use, check readiness. The GUI can offer and launch the shared setup
workflow; CLI users can invoke that same workflow directly. The setup guides
required secure authentication, provisions approved provider resources, configures
the worker's local client and mount, and reports readiness. Later runs reconcile
the existing configuration and mount before starting the workload. Reauthentication
or missing OS permissions produce an actionable setup state rather than silent
fallback or repeated infrastructure creation.

## Ownership and connectivity

- Generic core supplies only reusable setup/readiness, execution ownership,
  cancellation, lifecycle and diagnostics primitives. The desktop consumes those
  contracts; it does not own a second provisioner.
- Optional packages in the owning external catalog supply storage protocols,
  provider setup workflows, OS adapters and declared pinned dependencies.
- Reuse a common provider connectivity layer for storage and remote desktop.
  [TASK-043](../agents/tasks/configurable-networking/index.yaml) owns shared network
  contracts; [TASK-036](../agents/tasks/remote-node-onboarding/index.yaml) owns
  pairing and delivery. Audit and extend their existing seams before adding new ones.
- Cloudflare is an optional adapter, with direct LAN and other qualified private
  connectivity left possible. Its proposed storage path uses Tunnel private routes
  and worker enrollment/client setup. An existing public HTTPS broker route is not
  automatically an SMB route. Qualify authentication, routes and client behavior
  before claiming persistent mount support.
- Reuse existing authentication where its scopes permit. Provision additional
  routes/grants explicitly; pairing is not account-administrator authority.
  Cloudflare identity and SMB/share credentials remain separate. Credentials are
  secure store or vault references, never plaintext workflow fields or diagnostics.

Cloudflare currently documents [SMB through Tunnel and its client](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/use-cases/smb/)
and recommends [Client-to-Tunnel for long-lived connections](https://developers.cloudflare.com/tunnel/concepts/routing/).
Reverify provider support, terms and platform prerequisites during implementation.

## Storage runtime and local cache

Configure the source/share, worker mount location, read-only or writable access,
credential references, and persistent versus job-scoped lifetime. Validate the
intended endpoint, access mode and readiness. Distinguish reused/operator-owned
mounts from job-created mounts; cancellation or job cleanup releases only resources
owned by that job. Persistent mounts need explicit configuration. Do not expose
services or alter routing/firewalls merely because a package was installed.

Initial use cases are datasets, model weights, completed checkpoints, media/assets,
and build artifacts. Keep source checkouts, build trees and tool environments local
to each worker by default. Sharing a mutable repository across operating systems
is deferred because filesystem semantics and build outputs differ; no instant-run
or local-disk-performance promise follows from a mount.

Training workflows can pass environment configuration for the dataset root, local
SSD cache directory/budget, shard prefetch count and checkpoint destination. Names
and YAML syntax remain design choices, not accepted schema. The training/data
package owns the cache and shard policy: bounded storage, prefetch while training,
integrity checks, eviction only after readers finish, appropriate sample/shard
shuffling, and transfer of completed checkpoints. Do not place model-specific
batching in the networking runtime. Report connectivity and transfer failures
without claiming a checkpoint is durable before its destination copy completes.

## Remote desktop and navigation

[TASK-046](../agents/tasks/native-remote-desktop/index.yaml) remains the remote
desktop owner. RDP consumes the shared connectivity setup and adds its own
protocol, session authorization and native client; it does not require a storage
mount. Storage adds the share/mount lifecycle independently. Reuse the paired
control path to configure either endpoint without rebuilding Cloudflare setup for
each feature.

The user's direction is a dedicated RDP/remote-desktop tab, separate from the VM
tab. Revisit top-level navigation and tab overload before choosing its final
placement. Reuse generic native surface/Canvas/Tiling seams only where useful;
do not force RDP into the local QEMU VM view or make storage a desktop prerequisite.

## Qualification before delivery

Prove setup and runtime through the same CLI/workflow contracts used by the GUI,
on each claimed host OS. Include existing pairing reuse, first-use setup,
already-configured idempotence, expired credentials, denied permissions, route or
mount failure, disconnect/reconnect, revocation, cancellation and owned cleanup.
Qualify actual mounts and representative cached data jobs, with measured capacity,
throughput and latency; unit/schema checks alone are insufficient. Network access,
share access and desktop control grants must remain independently enforced.

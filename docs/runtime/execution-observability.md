# Execution Explorer and telemetry

The launcher's **Executions** page shows recent Dockpipe-managed processes and a
separate **System** view. Select a process or runtime resource for lifecycle
records, available metrics, duration and its existing operation-log location.
The floating inspector uses the same movable/resizable canvas panels as Docker
and Workflows. **Executions → Runs & logs** contains session logs and stop
controls, plus remote jobs with cancellation and result downloads. Successful
remote submission selects Remote jobs. Apps remains a card library.

## What is observed

Host scripts and commands, attached and detached Docker launches, and remote
worker workflow subprocesses register at their existing launch/wait boundaries.
Each record reuses the host-run identifier source and carries the workflow
invocation, step ID (or positional step index), parent execution where supplied,
local machine, runtime, PID, process start identity, timestamps and exit outcome.
Standalone operations do not need a workflow. Cancellation and signal termination
are distinct from success; a missing terminal record is never inferred as success.

The engine still owns execution. Unix process groups, Windows Job Objects, host
signal forwarding and runtime cleanup retain their existing behavior. Telemetry
cannot launch, stop or gain control over an external process.

The shared Go collector interface, snapshot model and JSON CLI are reusable by
other clients:

```sh
dockpipe runs observe --workdir /path/to/project --json
dockpipe runs observe --workdir /path/to/project --resources --json
dockpipe runs observe --system --resources --json
dockpipe runs observe --global --json
```

Use the **Standalone** toggle or `--global` to inspect operations without a project.
The first command reads lifecycle history and checks live process identities.
`--resources` includes metrics. System discovery reads process names, parent PIDs,
states and kernel counters; it never reads command lines or environments. Missing
metrics serialize as `null` or absent optional objects, and render as **Unavailable**.
Linux CPU percentages require two samples with the same PID and boot/start token;
one fully busy CPU core is 100%. Sampling gaps are not zero-valued measurements.

## Configuration and resource bounds

Lightweight lifecycle recording is enabled by default. Use the existing workflow
`vars` configuration to disable it; there is no second YAML configuration model:

```yaml
vars:
  DOCKPIPE_OBSERVABILITY: "off"
```

The same environment variable applies to standalone host/container operations.
Existing YAML validation, schema, editor support and document round trips already
support these string variables. Tests cover preservation and schema validation.
The Explorer's **Resource metrics** control and CLI `--resources` opt into sampling;
**Pause updates** stops polling and cancels the current read. Leaving the page also
stops monitoring. Known launcher session transitions request a refresh.

Each execution has a two-record asynchronous queue and an explicit close/drain
boundary. Recording does not wait for storage at process start. Storage lock
contention has a 250 ms retry budget; completion drains for up to one second.
Storage failure or overload reports incomplete history without changing execution
results. Local inspection does not start a collector daemon; opted-in remote
monitoring runs within the existing outbound worker lifecycle.

History is private local JSON under the existing project run store's `executions`
subdirectory, bounded to the latest 256 executions and 1 MiB. Project observations
also recover active runtime associations from the runtime-owned host-run registry,
checking the exact kernel identity of their owner before including them. These
verified associations take priority over recent completed executions within the
256-entry result. Registry discovery examines at most 4096 entries per observation.
Operations with no
project use the existing global data directory's `runs/executions` subdirectory.
History updates use a cross-process lock and atomic replacement. It is a recent
activity cache, not a durable audit log. Heavy concurrency can exceed retention;
the snapshot reports its capacity limit. Crashes can leave `running` records:
readback checks PID plus kernel start token and shows **unverified** when identity
has changed, exited, or cannot be checked.

System discovery retains at most 4,096 processes, scans at most 8,192 numeric
entries and has a three-second CLI deadline. The table initially displays 250
matching rows. The managed graph initially displays 32 matching executions, with
an explicit expansion to 64. Filtering searches the retained snapshot. The selected
CPU chart holds 60 samples. Refreshes run every four seconds, never overlap, and
ignore results from an earlier project/view/pause state. Each CLI read has a
five-second launcher timeout and bounded output.

## Runtime associations

Docker launch records retain the container name or returned immutable container
ID. Container cards are associations; their status is not inferred from the Docker
client's exit. Selecting an associated resource with metrics enabled requests
`docker stats --no-stream` through the existing configured Docker CLI/transport.
Only that resource is sampled, with a three-second deadline and 8 KiB output cap.
Historical names are not sampled after their owner exits; detached launches can be
sampled by their returned immutable ID. Docker client process metrics and container
metrics are kept separate. Detailed container lifecycle/log controls remain on the
existing Docker page. No live Docker lifecycle mutation is added by the Explorer.

The existing VM runtime registers its launched QEMU host process using the generic
`runs associate <runtime> <resource-id> <pid>` sidecar API. Registration requires an
existing live host-run start identity; it records the resource's own start token.
The API cannot launch or terminate resources. The host runtime's ordinary cleanup
removes the sidecar, while execution history retains the historical association.
Only QEMU host metrics are exposed. Guest processes, guest CPU and guest memory are
not claimed. The separate VM qualification controller is not retrofitted with a
new execution or authorization path.

## Platform and transport support

| Capability | Linux | macOS | Windows |
| --- | --- | --- | --- |
| Host/Docker/worker lifecycle metadata | Implemented, local tests | Implemented, cross-build only | Implemented, cross-build only |
| Process start identity and System discovery | `/proc`, including boot identity | BSD creation seconds/microseconds; native focused tests | Creation FILETIME; native limited-user tests |
| Host CPU and resident memory | Implemented | Unavailable | Cumulative process CPU; resident memory unavailable |
| Selected Docker resource metrics | Docker CLI; fixture-qualified | Same CLI contract; native unqualified | Same CLI contract; native unqualified |
| QEMU host association/start verification | Existing VM runtime adapter | Package adapter implemented; native identity tests pass; complete association qualification pending | Package adapter implemented; native identity tests pass; complete association qualification pending |
| Whole-system resources | CPU, memory, swap, root disk, network, uptime/load | Unavailable | CPU, physical memory, system-volume capacity, network and uptime; native qualification pending |
| Opted-in remote monitoring | Outbound snapshots | Docker inventory; native resources unavailable | Worker runtime unsupported |

Remote worker execution uses the same local recorder on the worker. Existing
broker job status and raw logs are available in **Executions → Runs & logs**. Remote monitoring adds an
explicitly authorized snapshot channel, never an arbitrary remote shell.

Process inspection on Windows holds a process handle while reading its creation
FILETIME and parent PID, then checks that the same kernel object remains live.
macOS samples fixed-size BSD process metadata twice and admits only an unchanged
creation timestamp. Both collectors check cancellation between bounded native
calls and never read process arguments or environment. Discovery retains at most
4,096 records from at most 8,192 candidates; inaccessible entries produce an
explicit partial-inventory error. Unsupported counters remain null.

## Overall resources and remote machines

Choose **Executions → System** for CPU, memory, swap, root filesystem capacity,
network activity, uptime and one-minute load above the process table. Overall CPU
is normalized across all logical CPUs: 100% means the entire machine is busy.
Memory used is total minus the kernel's available estimate, rather than treating
reclaimable cache as exhausted memory. Disk capacity describes `/` in the
collector's mount namespace. Network rates sum non-loopback interfaces, including
virtual interfaces; bridge/VPN traffic may be counted at multiple interfaces.
The CPU chart retains 60 samples. CPU and network rates need two measurements;
reboots, counter resets and machine changes reset their baselines.

On Windows, whole-system measurements use native Windows APIs through the same
CLI snapshot contract. CPU and network rates appear after two samples. Disk
capacity describes the Windows system volume. Unix load average and swap are
unavailable; Windows commit capacity is not reported as physical swap. This does
not describe per-process counters. Windows process discovery separately uses native
creation identities and cumulative CPU time; resident memory remains unavailable.
Whole-system CPU counters require a single processor group; on machines with
multiple groups, CPU stays unavailable rather than showing a partial total.

Click any process-table column header to sort ascending or descending. PID,
parent PID, memory and CPU sort by numeric values; unavailable metrics stay last.
Sorting covers the full collected snapshot before the visible row limit and
persists across refreshes.

Both **System** and **Containers** offer **This computer** and the paired machines
from the existing **Machines** connection. Switching targets clears the previous
machine's data and discards outstanding results from it. Remote System currently
shows machine resources and discovered processes, not project execution history.

The worker owner enables the desired sharing locally, using the same state
folder as their paired worker:

```sh
dockpipe remote monitoring --system --docker
# Or enable only one category; each command replaces the selected permissions.
dockpipe remote monitoring --system
# Revoke both permissions.
dockpipe remote monitoring --disable
# Operator-side inspection through the existing broker connection.
dockpipe remote observe --node worker-name
```

These commands also accept `--state <directory>`. The separate private
`monitoring.json` file does not change pairing or workflow-delivery authority.
An updated running worker rereads permission every monitoring cycle; a stopped
worker must be started normally. Both worker and broker need this CLI version.
No existing worker is opted in automatically, and the operator cannot turn on
sharing remotely. Hosted brokers must implement this monitoring protocol too.
An older broker, offline worker, disabled category or stale sample is reported as
unavailable; the app never falls back to this computer.

The worker's independent monitoring loop has a five-second collection deadline,
four-second upload deadline and four-second pause. It continues during workflow
execution, is cancelled and joined when the worker stops, and never changes job
results after a monitoring failure. The broker authenticates worker uploads,
binds each sample to its enrolled node, and permits reads only to its operator.
It retains one in-memory snapshot of at most 1 MiB per enrolled node (64 nodes
maximum); snapshots expire for reads after 30 seconds and disappear on restart
or revocation. Process lists that exceed the byte budget carry a truncation flag.
There is no durable telemetry archive.

Remote Docker is a read-only inventory/topology view: containers, networks,
volumes and their associations. Existing local start/stop, terminals, inspect
and log controls remain local. Remote snapshots omit command arguments,
environment values, application logs, arbitrary labels and mount source paths.
Only the Compose project, service and dependency labels are retained. Collection
uses the worker's Docker CLI/context with a three-second total deadline, a
512 KiB command-output bound and 256 objects per kind. Missing or truncated
inventory is labelled. Docker process errors do not cancel worker jobs.

## Privacy and remaining limits

The snapshot contains selected identity metadata and existing log references,
not argument vectors, environment dumps, captured stdout/stderr, or raw errors.
The inspector's log combines lifecycle records with the selected run's existing
workflow operation log. Its tail is bounded to 64 KiB and 200 summaries; only
protocol unit/status, timestamp and duration are returned. Free-form messages,
errors and identifier maps are omitted. Shared workflow logs are labelled as shared
history rather than falsely attributed to one execution. Existing raw run logs
are available in **Executions → Runs & logs** and are not ingested into telemetry.
No root/admin permission is needed. Permission failures leave metrics unavailable.

Managed root processes and explicitly registered runtime children have producer
associations. Other descendants are not promoted to managed based on process names
or a one-time parent-PID match. System parent IDs are best-effort kernel observations.
Pre-scripts used only to resolve environment, arbitrary internal helper subprocesses
and already-running processes are not retrospectively registered as managed runs.

Recommended follow-ups are native macOS/Windows collectors, explicitly authorized
remote container controls/log access, redacted application stdout/stderr integration, and historical audit retention separate from this bounded live view.

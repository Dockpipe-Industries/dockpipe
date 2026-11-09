# Native invocation validation

Dockpipe's shared invocation contract is implemented in
`src/lib/domain/invocation`. It admits typed native operations for existing CLI,
desktop, and authenticated transport adapters. It is not a second workflow engine,
an OS sandbox, an authentication service, or a universal shell parser.

This work implements the shared-boundary portion of
[TASK-039](../agents/tasks/unified-mcp-gateway/index.yaml). The gateway, package
registry contribution contracts, remote feature rollout, and platform qualification
remain separate acceptance work. Do not interpret this page as proof that every
existing adapter has been migrated.

## Boundary

Trusted native composition selects the exact capability, action, version and target.
It supplies an identity asserted by its local or authenticated transport adapter;
identity strings in a caller's request are not authentication. The generic gate:

1. Checks the selected contract and target, including remote machine fingerprint
   and worker generation. An absent or changed remote target never becomes local.
2. Calls the domain's memory-only preparation to validate and capture owned input.
   Preparation cannot read files, start processes, contact a network, or perform
   another external effect before authorization.
3. Requires applicable authorization and a SHA256 binding of the owned input for
   privileged or remote calls. A caller cannot disable a required control.
4. Rechecks cancellation and immediately invokes the domain executor. It returns
   no reusable permit and never retries a side effect automatically.

The domain executor retains filesystem preconditions, Git reviewed snapshots,
compiled runtime policy, process supervision, and provider-specific enforcement.
Authorization and execution remain distinct: an input digest alone does not prove
that a mutable script or workflow file's contents have been reviewed or frozen.

Ordinary local calls run in process without a registry lookup, gateway, database,
remote identity exchange, new service, or durable-job write. Native process
attribution is not a human approval or a privileged grant. Required authorization
fails closed when unavailable; installation and discovery do not grant authority.

## Current source coverage

| Owner | Admission and execution boundary |
| --- | --- |
| Project Git | `RunWorkspaceGit` and `RunWorkspaceGitWithAuthority` admit known typed actions before repository access. Mutable path/commit selections are copied before authorization. Existing snapshot, confirmation, credential and publication checks remain in the Git executor. |
| Workspace files | The core `workspacefiles.Service` admits typed file requests before opening its root. The local CLI and main Files editor use this service asynchronously; remote adapters select roots from worker-local grants. Compact dashboard browsing, workflow-source open/save, and active graph discovery use the same asynchronous service. Workflow authoring remains local regardless of the shared execution machine. Source writes retain exact-byte revisions, preserve BOM/CRLF, and refuse concurrent changes. The exported graph page helper retains its bounded legacy filesystem adapter for compatibility; the UI no longer uses it. This helper is not proof of universal file-adapter admission. |
| Workflows and commands | Workflow selection and concrete host/script/container adapters use the same gate. Existing workflow/schema validation and compiled runtime policy remain authoritative. Standalone Docker image build/pull/existence checks, Compose lifecycle, Docker preflight, and host cleanup have owning admission boundaries. Internal provider/housekeeping subprocesses run within those admitted lifecycles, not as individually sandboxed capabilities. Script internals and mutable file contents are not made immutable by admission; active Docker cancellation retains its existing lifecycle. |
| Containers | Typed local snapshot/control CLI and authenticated remote snapshot/control requests enter the shared gate. Desktop and remote object inspection and fixed 200-line container logs use strict typed read actions through core. Inspection accepts one full container/network ID or exact volume name, with a 512-KiB aggregate output limit and 15-second supervised process deadline. DockerRead includes raw inspect values and diagnostic logs; it is broader than filtered topology inventory. |
| Interactive terminals | Session launch and managed command submission require admission. A terminal's arbitrary input stream is not proof of individually validated shell operations. Local desktop sessions call `terminal --stdio`, which admits the frozen profile and routes actual operations through a private QtCore native provider. Authenticated remote sessions use target-bound leases and native input acknowledgments. Linux real-PTY tests pass; native Windows, macOS and installed Flatpak qualification remain outstanding. |

## Workspace file CLI

`dockpipe files --request <file.json|->` reads one UTF-8 JSON object from a regular
request file or stdin, up to 2 MiB. Example:

```json
{
  "root": "/path/to/project",
  "request": {
    "schema": "dockpipe.files/v1",
    "action": "read",
    "path": "README.md"
  }
}
```

The response contains `schema`, relative `path`, an exact-byte SHA256 `revision`,
and base64 `content`. BOM and line endings are preserved. File content is limited
to the existing editor's 1 MiB bound. The CLI has a 25-second request deadline;
blocking filesystem calls still need the owning helper-process deadline.

Actions are `list`, `read`, `create`, `write`, `rename`, `delete`, and `mkdir`.
Writes, renames and deletion require the reviewed file `revision`. Rename and
delete currently operate on regular files, not directories. Create and rename
refuse an existing destination. New files use private permission bits. Write
publishes a private temporary file by atomic replacement and preserves the
existing permission bits. External writers
can still race the final revision check and replacement; this is not a filesystem
compare-and-swap guarantee against malicious concurrent writers.

Paths use slash-separated relative names; traversal, links and Git metadata paths
are rejected. Root-relative operations use the selected toolchain's
[`os.Root`](https://go.dev/blog/osroot) APIs, with pinned directories and regular-file
checks. This confines ordinary path and symlink traversal; it does not prohibit
mount boundaries, make an untrusted filesystem safe, or constrain an independently
permitted shell/container process.

For a directory page, use `action: "list"`, `path: "."`, `limit: 1..150`, and
optional `hidden`. Follow `after` and `revision` from the previous result. Changes
between pages reject stale cursors. Listing streams entries and retains one page;
its 20-second scan deadline and response budget match the existing graph reader.
Unknown fields, operations, schemas and trailing JSON objects are rejected.

The Files explorer keeps a bounded in-memory cache of first-page directory
metadata: at most 32 directories and 2 MiB of serialized listings per selected
context. Cached folders render immediately while an asynchronous read checks
freshness. While Files is visible, it refreshes on entry and every five seconds;
failed reads preserve the last successful listing and retry on the next active
refresh. Hidden pages stop background reads. Up to four immediate child folders
are prefetched through a separate background read slot, so navigation does not
wait behind prefetch. This is not a recursive machine-wide index. Context,
worker-generation and hidden-file-filter changes clear the cache. File contents,
editing revisions and mutation authority are not served from this metadata cache.

The Git page also refreshes on entry and every five seconds while visible, using
one passive status reader that does not disable the main controls. Draft commit
messages, conflict edits, active operations and context changes prevent stale
background results from replacing a review. An unchanged status preserves the
selected diff and scroll position; hidden branch/stash panels do not issue
catalog reads, and their catalog can reuse the fresh status response. Mutations
still require the current core snapshot and keep their normal admitted lifecycle.

The local envelope's `root` is not a remote authority field. A remote worker must
map an opaque root ID through its own explicit grants before creating the service.
File and Git grants do not constrain an independently authorized unrestricted
terminal or workflow capable of running arbitrary code as that user.

## Interactive execution

A shell can expand aliases and substitutions, source scripts, launch another
interpreter, or send input to a running program. Treating each Enter key as a
complete, trustworthy command would leave bypasses. Admit the actual managed
launch/submission and enforce its permitted host or sandbox authority at execution.
Do not claim that inspecting text constrains all subsequent program effects.
Passwords and raw terminal input must not be added to logs to implement validation.
Remote terminals additionally need target-bound leases, input/output sequencing,
revocation, disconnect handling and process cleanup before being exposed.

## Remote feature CLI

Paired workers explicitly opt in with `dockpipe remote pair --endpoint <https-origin>
--node <name> --allow-features`. This grants the enrolled machine identity, not
workflow delivery. `--allow-delivery` is an independent, broader grant.

On the worker, `remote features-config --request <private-file|->` validates and
saves local root IDs, paths, file/Git read/write/publication grants and Docker
read/control grants. Omitting `--request` reads that configuration. A running
worker must restart after its first feature opt-in; subsequent grant edits reload
live. Requests never choose the worker's host directory or executable.

`remote features --auth never` returns fresh feature catalogs with machine
fingerprints and worker generations. `remote feature-call --request <file|->`
accepts a prepared `dockpipe.remote.features/v1` request: target, optional root ID,
action, payload, caller-fixed nonce and deadline. An omitted ID is computed from
the canonical request; a supplied ID must match. A retry retains the exact intent,
including nonce and deadline. Input is bounded to 2 MiB; the CLI has a 25-second
operation deadline and a bounded non-interactive cancellation attempt.

`remote feature-prepare --request <file|->` validates the exact intent locally and
returns its canonical ID and routing metadata before dispatch. It does not resolve
remote state, authenticate, or contact the broker. Desktop Files, Git and container
clients use preparation and private stdin; preparation and dispatch share one
operation timeout. Uncertain mutation references are retained in a bounded desktop
settings list (128 entries), without payloads, nonce, file bytes or credentials.
The shared selector's **Uncertain operations…** dialog addresses the original
receipt even after the selected machine changes.

`remote feature-status` and `remote feature-cancel` accept the request metadata
from an uncertain receipt. They address the original ID and target; they never
reconstruct or resubmit content. A cancellation acknowledgment is not proof that
a write had no effect. A validated terminal cancellation response preserves the
broker's known success, failure or cancelled status immediately; running, queued,
unknown or mismatched responses remain uncertain. Unknown mutations remain blocked
pending reconciliation.

`remote feature-reconcile --request <metadata-file|->` addresses the original mutation.
Queued work is cancelled before dispatch. Running/unknown work permanently loses
its original invocation admission, then waits for the exclusively locked serial
worker to confirm that native file execution has stopped. A replacement generation
must retain the matching durable receipt; a new worker without that history cannot
acknowledge cleanup. The status remains **unknown** with `reconciled: true`, never
success. Refresh Files and read back the state before independent new work. No
mutation is replayed, and polling cannot recreate revoked authority.

Git/native-provider operations persist process ownership before spawning. Linux
`process_recovery: cgroup-v2` requires a writable delegated cgroup and attaches each
child atomically before execution. Recovery kills and verifies that exact scope
empty, including cooperative descendants that detach their process session. This
is lifecycle supervision of trusted tools, not confinement against hostile same-UID
processes that can migrate out of the delegated tree. Missing delegation fails
process-backed mutations before effects. Local Git behavior is unchanged.

The worker owner can instead select `process_recovery: host-reboot` (including
macOS). Normal confirmed results remain usable; uncertain process operations require
a changed kernel boot identity captured before their effects. Reconciliation never
reboots a host. Legacy receipts without a captured boot/scope cannot retroactively
gain cleanup proof. Git reconciliation does not replay commands, remove repository
locks, or abort repository operations; refresh and inspect the repository afterward.

Native Docker client exit does not prove asynchronous daemon work stopped. Unknown
container control can use the separate owner grant `docker_recovery` and explicit
`--recovery retire-container --expected-container <full-id>` after stopping its
captured native scope. Retirement removes exactly the recorded container through
the captured provider, without volumes or images, and verifies its absence. It is
destructive and requires a fresh recovery admission; it never restores the old
write admission or claims that the original operation succeeded. Reconciled unknown broker receipts are retained
within the existing 128-operation/storage bounds; this is bounded recovery, not an
unlimited operation archive or a general process-recovery mechanism.

The broker and worker retain mutation-only metadata receipts. File contents,
read outputs, terminal input and credential values must not enter these journals.
Reads do not create durable operation journals. Crash recovery never replays a
mutation whose completion is uncertain. These are implementation boundaries,
not evidence of complete desktop rollout or qualification on remote hosts.

## Local container CLI

`dockpipe containers --request <file|->` accepts one strict JSON object up to
4 KiB: `{"action":"snapshot"}` or `{"action":"start","container":"<full-id>"}`.
Read actions also include `inspect` (`kind`: container/network/volume and `object`)
and `logs` (container only, fixed 200-line tail). They accept no arbitrary Docker
arguments or caller-selected executables. Control actions are start, stop and restart; IDs must be the full 64 lowercase
hexadecimal characters. Unknown/duplicate fields, caller-selected executables and
extra arguments are rejected. Local desktop controls use private stdin to the
trusted CLI and retain the selected container identity through completion.


`dockpipe containers exec --container <full-id> --shell /bin/sh` admits the actual
local container-shell session (also `/bin/bash`). The desktop opens this trusted
CLI command in the system terminal; Flatpak re-enters its managed CLI rather than
launching host Docker directly. Provider/environment and inherited stream identities
are captured before authorization. Interactive bytes are not command journals.
The session has its owning signal/context lifetime, supervised local process
cleanup and foreground/terminal-mode restoration. Linux real-PTY fixtures use a
fake Docker provider and cover success, cancellation and redirected output. They
do not qualify actual Docker daemon/container descendant cleanup.

## Remote container terminal

The independent worker-local `docker_exec` grant requires Docker read authority;
host terminal and container-control grants do not imply it. The desktop opens a
captured full container ID and `/bin/sh` or `/bin/bash` through the existing shared
terminal hub and its eight-session bound. A container-only prepared-reference/ACK
exchange retains the original canonical reference before remote opening effects.

The trusted Docker provider's `system dial-stdio` carries fixed Engine API requests,
preserving its selected context and transport. The worker binds the daemon identity
and persists the exact ExecID before starting it. Normal closure requires that
ExecID to have been observed running and then authoritatively stopped on the same
daemon, plus local relay cleanup. This proves the tracked entrypoint stopped; an
interactive shell can leave background processes and persistent effects. Container
deletion is never a normal close action. Normal close sends bounded terminal EOF
before disconnecting and inspecting the captured ExecID; a shell that ignores EOF
remains uncertain. Input bytes are not persisted. Receipt publication and catalog
reads share a cross-process store lock. Terminal heartbeats preserve recovery
entries only for the exact same worker generation; feature heartbeats remain the
authority for replacing that list.

Unknown sessions retain their quota reservation and appear in the recovery dialog.
Only a validated receipt matching the original reference releases it. Verification
requires `docker_exec`; the separate `docker_recovery` grant permits explicitly
confirmed retirement of the captured container when closure cannot otherwise be proven. Changed daemon
identity, missing receipt history and mismatched references fail closed.

## Remote host terminal recovery

New host-terminal launches retain a private metadata-only receipt with their exact
reference, opaque root, launch phase and kernel boot UUID before starting a PTY.
Publication failure prevents launch. Host and container orphans share the existing
eight-session bound; metadata stores remain bounded and use the same cross-process
publication lock. Terminal input and output are never recovery journal contents.

The worker catalog exposes `host_terminals` entries containing only `reference`,
`root` and `phase`. A fresh typed `terminal.host-recover` feature request selects
the current authenticated worker, retains the original root, and supplies
`{"reference": <original terminal reference>}`. That root's independent Terminal
grant is required. Recovery revokes the original opening/input admission and
requires the captured kernel boot UUID to have changed. Restarting a worker or
broker does not satisfy that proof. Only a validated exact-reference recovery
completion releases the corresponding transport reservation; it makes no claim
about the outcome of commands previously typed into that terminal.

The typed `host_recovery` completion metadata survives a lost response or service
restart without retaining original payloads. The worker retains its terminal
receipt until the broker acknowledges that completion. Missing or legacy receipts,
changed machine identity/root, malformed storage and mismatched proof fail closed.
Older host sessions without a pre-effect boot receipt cannot gain one retroactively.
Their diagnostic references must remain explicitly unverified.

`remote terminal --prepare` opts host clients into the existing canonical
prepared-reference/acknowledgment exchange before opening effects. New desktop
sessions use it; the default host stream remains compatible with older clients.
Container streams continue to require that exchange. Desktop host and container
reservations both wait for verified cleanup; malformed final frames cannot publish
cleanup verification. On Darwin, a successful empty fixed-size kernel PID query
means the process disappeared. Native kernel errors and malformed nonempty records
remain cleanup failures rather than being treated as disappearance.

## Remote interactive terminal transport

Authenticated remote input can wait up to one second for its native acknowledgment,
with at most one waiter per exact session. Waiting releases the broker lock and
rechecks authority, session identity and transport epoch before returning. The
worker wakes for actual output and acknowledgments; real input permits a bounded
20 ms polling burst for 250 ms, then returns to its 200 ms idle cadence. The CLI
fetches output immediately after acknowledgment. Duplicate input remains
idempotent; lost acknowledgments do not authorize replay or optimistic echo.

## Local desktop terminal transport

`dockpipe terminal --stdio` accepts private `dockpipe.local-terminal/v1` frames.
The native UI resolves the executable, arguments, directory and environment once;
core admission owns the captured profile and invokes the trusted native provider.
The provider is an operator-selected absolute `DOCKPIPE_TERMINAL_PROVIDER` or the
installed `dockpipe-terminal-host` sibling, never a request-selected executable.
Installation qualification must exercise that selected CLI and provider together;
a successful CLI version check cannot establish local terminal availability. The
desktop smoke checks real shell output, native input/resize acknowledgments and
verified close, with bounded output and an overall deadline.
Native environments are complete snapshots; the explicit Flatpak host route
captures overrides and retains the host-inheritance contract.

The initial profile line is bounded to 2 MiB, later lines to 96 KiB, input to
64 KiB, output frames to 32 KiB and retained core output to 256 KiB per session.
Acknowledgments follow completed native writes. Lost acknowledgments are not
replayed. A vanished process without a verified cleanup receipt retains an
unknown desktop quota reservation. Eight retained or closing sessions are allowed.
Profiles, credentials and raw terminal input are not written to operation journals.

The first implementation uses one core process and one QtCore helper per local
session. An isolated Linux measurement over eight starts and 160 inputs measured
median startup 14.15 ms for the helper alone and 22.63 ms through core; median
input acknowledgment was about 4.16 ms for both. Eight simultaneous sessions used
24 processes including shells and approximately 67.9 MiB total proportional set
size. These are headless fixture measurements, not a GUI startup or cross-platform
benchmark. The existing native Windows/Flatpak backend is preserved; platform
runtime qualification remains separate.

## Captured workflow delivery targets

The main execution surfaces share one machine choice. Workflow source editing and
catalog discovery stay local; selecting a worker sends the reviewed source bundle
through the existing workflow-delivery approval and permission contract. Compact
and full dashboard workflow widgets can follow the shared selection or pin a local
or remote target. A local widget does not inherit a later remote selection. Shared and widget machine
changes refuse active workflow preparation, including the continuation between
supporting-file save and workflow validation. Relaunch captures the original target
while waiting for an existing session to stop.

Desktop delivery supplies `--expected-fingerprint`, `--expected-generation` and,
when selected, `--expected-root`. The submission retains those identities, checks
fresh broker catalogs at submission/assignment, and rechecks worker-local generation
and root grants before materialization/execution. Heartbeats revoke stale-target
execution. A stale queued target fails before assignment. The root ID preserves the
selected grant identity; delivery still uses its existing isolated delivered-workspace
contract and requires the independent delivery permission. Feature grants never
implicitly grant workflow execution. Legacy node-only CLI submissions retain their
existing explicitly selected delivery contract; captured desktop targets never use
that lane.

## Local qualification checkpoint (2026-11-08)

The continuation's 17 affected native suites pass through the repository desktop
workflow: workflow studio/workspace/navigation, language restart, full shell,
graph discovery, dashboard widgets, remote feature/Git/Files/container transports,
remote process/terminal lifecycle, machine selection, hardware-key transport and translations. These fixtures
cover asynchronous source I/O, stale revisions, BOM/CRLF, cancellation, supporting
file discard, captured workflow targets, mismatched receipts, recovery storage
failure/capacity, and synchronous/asynchronous window close. Exported client/widget
layouts remain unchanged; older inline Git busy/cancel callers retain their transport
behavior through child-owned operation state.

Remote container fixtures also cover the prepared-reference acknowledgment, lost
opening responses, independent grants, captured contexts, mixed eight-session
quotas and exact-reference recovery release. A native delegated Linux cgroup test
verified cleanup of a cooperative `setsid` descendant after its parent exited.

Full affected Go package suites pass for remote contracts, infrastructure, remote
infrastructure and the remote/container CLI owners; focused authority/recovery and
container-exec race checks pass. Native Linux PTY proof uses a fake Docker provider.
The desktop/helper and Linux CLI build successfully. Windows amd64 and macOS arm64
CLI builds provide compile/link evidence only. All 17 catalogs contain 2,420 finished
messages, with extraction, placeholder and lrelease checks passing.

A consenting Darwin arm64 worker has now passed keyed pairing, native host PTY,
exact-revision file mutations and Git stage/commit readback through the broker.
Actual Docker Engine validation passed `/bin/sh` and `/bin/bash` sequenced input,
PTY resize, authoritative same-daemon ExecID closure, bounded inspect/log reads,
and exact-container stop/start/restart. An earlier uncertain terminal remained in
the catalog across worker and broker replacements; same-boot recovery was rejected
and preserved that obligation. After a separately approved reboot, exact-reference
reconciliation passed using the changed kernel boot identity and authorized
retirement of only the captured test container. Readback confirmed its absence
and release of the recovery-catalog entry. A real commit interrupted inside a
bounded test hook also retained its original unknown receipt. Same-boot Git
reconciliation refused; after another separately approved reboot, the exact old
reference became `unknown` with `reconciled: true`, without payload, nonce or
replay. Repository readback preserved the prior head and found no leftover Git
locks; the test hook, staged entry and payload files were removed. Worker startup
returned automatically after the restart.

The host-terminal hardening follow-up passed 15 selected native desktop suites,
full affected core/CLI suites and focused races, plus the complete terminalsession
suite natively on Darwin arm64. That native run exposed and verified the repair
for successful empty PID queries during foreground/background teardown. A separate
interrupt fixture now waits for actual ready output instead of echoed input.

On the paired worker, a review worker was interrupted while one idle host terminal
had a durable boot receipt. Its replacement advertised the exact orphan and
rejected same-boot recovery. After a separately approved reboot, exact-reference
host reconciliation passed, the orphan entry was removed, and a replacement
terminal closed with verified cleanup. The normal Homebrew Colima service started
the existing VM automatically at login; its temporary test owner did not return,
and pre-existing container IDs were preserved. Test transfer files were removed
with exact revisions and the fixture Git head stayed unchanged.

The original host session from before durable host receipts existed remains a
historically unverified diagnostic. Its ephemeral transport epoch expired during
the required broker upgrade; it was not converted into a cleanup certificate.
New durable host and container receipts survive those transport epoch changes.
These checks do not qualify every macOS desktop flow, native Windows runtime behavior
or installed Flatpak behavior.

A subsequent visible-app check exposed a missing native companion beside the
selected preview CLI and slow remote typing. The companion was restored and the
installed CLI/provider passed real start, input/output, resize and verified-close
checks. A 12-character paired-worker timing run measured median echo latency
548.46 ms before the transport repair and 75.17 ms afterward; the latter maximum
was 172.30 ms. Idle input still includes the worker polling interval and network
latency. This is bounded fixture evidence, not a general performance guarantee.
Native Darwin system-counter tests, live broker monitoring and the System widget
fed with two actual worker samples also qualify the repaired monitoring path.
The standalone legacy graph helper, mutable executable/script/Compose contents,
provider descendant cleanup and recovery retain the specific trust and platform
limitations described above. No universal OS isolation or all-adapter
validation claim follows from this checkpoint. RDP/Cloudflare private connectivity
has not begun.

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
| Workspace files | The core `workspacefiles.Service` admits typed file requests before opening its root. The local CLI and main Files editor use this service asynchronously; remote adapters select roots from worker-local grants. Compact dashboard browsing uses the same asynchronous service. Graph discovery and workflow-source I/O still require migration. |
| Workflows and commands | Workflow selection and concrete host/script/container adapters use the same gate. Existing workflow/schema validation and compiled runtime policy remain authoritative. Script internals, standalone build/compose actions, and housekeeping subprocesses are not all separate admitted capabilities; active Docker cancellation retains its existing lifecycle. |
| Containers | Typed local snapshot/control CLI and authenticated remote snapshot/control requests enter the shared gate. Existing detailed desktop Docker metadata reads remain separate adapters. |
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

`remote feature-status` and `remote feature-cancel` accept the request metadata
from an uncertain receipt. They address the original ID and target; they never
reconstruct or resubmit content. A cancellation acknowledgment is not proof that
a write had no effect. Unknown mutations remain blocked pending reconciliation;
a general reconciliation UI/command is not yet implemented.

The broker and worker retain mutation-only metadata receipts. File contents,
read outputs, terminal input and credential values must not enter these journals.
Reads do not create durable operation journals. Crash recovery never replays a
mutation whose completion is uncertain. These are implementation boundaries,
not evidence of complete desktop rollout or qualification on remote hosts.

## Local container CLI

`dockpipe containers --request <file|->` accepts one strict JSON object up to
4 KiB: `{"action":"snapshot"}` or `{"action":"start","container":"<full-id>"}`.
Control actions are start, stop and restart; IDs must be the full 64 lowercase
hexadecimal characters. Unknown/duplicate fields, caller-selected executables and
extra arguments are rejected. Local desktop controls use private stdin to the
trusted CLI and retain the selected container identity through completion.


## Local desktop terminal transport

`dockpipe terminal --stdio` accepts private `dockpipe.local-terminal/v1` frames.
The native UI resolves the executable, arguments, directory and environment once;
core admission owns the captured profile and invokes the trusted native provider.
The provider is an operator-selected absolute `DOCKPIPE_TERMINAL_PROVIDER` or the
installed `dockpipe-terminal-host` sibling, never a request-selected executable.
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

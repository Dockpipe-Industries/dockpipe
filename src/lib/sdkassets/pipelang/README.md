# Dockpipe PipeLang source SDK

`dockpipe sdk path --language pipelang --workdir <project>` materializes this
complete, immutable bundle and prints its `library.json` path. The descriptor
names `Dockpipe.SDK` version `0.1.0` and pins every `.pipe` source. Native and
CMake support assets are included in the bundle's content-addressed identity.
Materialization verifies a reused cache against the selected executable's bytes.

Provision the returned descriptor as an explicit installed-SDK
`pipelang.source-library-references.v1` reference. Use the descriptor's parent as
`root`, `library.json` as `descriptor`, and the exact descriptor SHA256 as
`sha256`. Pass that references file through `pipelang_add_model`'s
`SOURCE_LIBRARIES` argument. A consumer imports `Dockpipe.SDK`; it does not depend
on a source checkout of Dockpipe or PipeLang. The source library compiles with the
consumer's selected PipeLang SDK and makes no cross-SDK binary ABI promise.

`Client` is a source facade over an injected `Transport` host interface. It
forwards typed calls and preserves `Result` success, cancellation and failure.
The host owns executable selection, input validation, process custody and I/O;
the facade contains no workflow policy or package/storage layout.

## Workflow lifecycle

`WorkflowRequest` requires an explicit `workdir`, a positive `outputLimitBytes`,
and exactly one workflow selection: `workflowFile`, or both `packageName` and
`workflowName`. Variables are `List<WorkflowVariable>` name/value records;
arguments are `List<string>`. Hosts pass them as distinct command arguments.

`Start` returns an `OperationId` containing its issuing client's identity and
operation value. Every subsequent operation must reject foreign or unknown IDs.
`Observe` waits within its supplied millisecond budget and cancellation token,
returning a bounded snapshot. A workflow's nonzero exit code remains a successful
observation. `exited` describes the root process; `treeComplete` separately proves
that owned children and output pipes have finished. Output truncation is explicit
in `stdoutTruncated` and `stderrTruncated`.

`Interrupt` requests graceful termination. `ForceDispose` explicitly requests
forced cleanup within its supplied wait budget and cancellation token. Cancelling
an observation does not implicitly terminate the workflow or release its custody.
An incomplete cleanup observation retains ownership for a later bounded attempt.
`RetainedOperation` returns the ID still in custody, including a scope query whose
wait was cancelled or expired. It fails when no retained operation remains. Use
that ID with `Observe`, `Interrupt` or `ForceDispose` to finish explicit cleanup.

The bundled native transport's initial supported profile is a dedicated headless
host with explicit exclusive child-reaping ownership. That profile does not claim
support for a GUI or a process sharing child ownership with other subsystems.
The source facade also accepts other transports implementing the same contract.

## Scope service

`StartScope` returns an owned operation ID before waiting. Its path result is
available as `scopePath` in a complete normal observation; object output remains
complete bounded JSON in `stdoutText`. Use this when source code owns observation
slices and cancellation. `Scope` is the typed convenience operation.

`Scope` calls the public `dockpipe scope` service with an explicit positive time
budget, output byte limit and cancellation token. The complete structured response
must fit the byte limit; truncated JSON is a failure. The transport converts the
service's JSON into `ScopeObject`, preserving its workflow/package fields and
using empty strings for absent optional fields. It does not derive storage paths.

`ScopeRequest` with empty `scope` and `packageName` requests the current workflow
object. A `packageName` requests its package object, or a service-resolved path
when `path` contains segments. A `scope` requests the corresponding scope path;
its `path` segments are passed through to the service. A request may select at
most one of `scope` and `packageName`.

`ScopeResult.isPath` distinguishes path results from objects. Path results set
`path` and leave all `value` fields empty; object results clear `path` and fill
`value`. Scope objects expose the public service fields, including artifact,
state, event and source paths, without teaching the caller their storage layout.

## Native bridge

Native composition can select `ExecutableMode::NativeTool` for a separately
admitted executable and call `StartNativeTool` with an argument vector,
confined working directory and per-stream byte limit. The native client owns
the process tree and preserves raw stdout and stderr in its bounded receipts.
Use the same observation, interruption and disposal operations for that
client. The default `ExecutableMode::Dockpipe` continues to expose workflow
and public scope dispatch.

Include `cmake/DockpipePipeLang.cmake` from the materialized bundle and call
`dockpipe_add_pipelang_bridge` with an existing `MODEL_TARGET`, its C++
`MODEL_NAMESPACE`, a `BRIDGE_NAMESPACE`, and an absolute `OUTPUT_DIRECTORY`.
Link the returned target and include its configured `DockpipeSDKBridge.hpp`.
Construct the bridge `Transport` with a shared native `dockpipe::sdk::Client`.
The bridge inherits that client's argument bound and the model's runtime and
instrumentation policy.

The Linux native implementation uses one live client and one active operation in
a dedicated child-reaping process. The Windows backend uses the same one-client,
one-operation profile with owned nested Job Objects. Its source implementation
requires native Windows qualification before platform acceptance. Configure all
resource bounds explicitly. It sanitizes inherited environment bindings and disables vault
access by default; `disableVault = false` selects normal engine vault policy.
Receipts include supplied arguments, so pass secret references rather than secret
values. Scope operations expose objects and absolute paths; resolver fields that
return scalar configuration values are outside this path API.

### Windows native artifacts and custody

The Windows bridge builds a separate console launch/signal executable. Retrieve its
CMake target with `get_target_property(signal_target <bridge-target>
DOCKPIPE_CONSOLE_SIGNAL_TARGET)` and stage `$<TARGET_FILE:${signal_target}>` with
the consumer's runtime artifacts. Pass that staged executable's absolute UTF-8
path as `ClientConfiguration.consoleSignalExecutable`. This field is required on
Windows and ignored on Linux. The SDK pins the opened helper and CLI native PE
images, including their ancestor directory handles; it never invokes a compiler
at runtime or embeds an absolute build-directory path.

Windows requires Windows 10 or later, a filesystem supporting stable volume/file
IDs and nested jobs, and a dedicated headless host. Paths must be canonical local
drive-absolute paths or exact OS volume-GUID paths of the form
`\\?\Volume{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}\...`, with hexadecimal GUID
components. Other device/UNC forms, reparse points, ambiguous components and
paths outside the selected workdir root are refused. `WindowsFiles` provides
handle-relative, reparse-rejecting file observations and exclusive new-file
writes; the existing
`ConfinedFiles` descriptor API remains Linux-specific. Ancestor handles deny
write/delete sharing, so existing incompatible directory handles cause admission
failure rather than a weaker path check. Executables are selected through their
retained volume paths; their process handles and job membership remain owned.

After root admission, confined operations accept either its admitted original
spelling or its retained volume-GUID spelling. The original spelling serves only
as a matching label: relative components are opened through the retained root
handle, and the operation exposes the actual opened volume-GUID path. A drive
mapping is never reopened for a confined request after root admission. Other
aliases of the same directory are refused unless they match one of those two
admitted spellings.

The native client opens each requested workdir once and passes that retained
directory into process start. Both the actual CLI `--workdir` operand and the OS
current directory use its opened volume-GUID path. Expanded UTF-8 operands are
charged against the configured argument limit. Receipts preserve the supplied
executable spelling as argv0 and record the actual canonical workdir operand.
Scope paths and JSON fields remain the public CLI's actual values; the SDK does
not rewrite them to match a request's original drive spelling. Callers requiring
exact path comparisons should use the actual opened `WindowsPath.nativePath`.

The explicitly selected matching helper supports private versioned launch and
signal modes. Startup gives it a private console, then it creates the real CLI
suspended with `CREATE_NEW_PROCESS_GROUP` in that console. The parent pulls and
retains the CLI process/thread capabilities, verifies their identity, mapped
image and exact job membership, acknowledges adoption, and requires the broker
to retire successfully before the parent alone resumes the CLI. The SDK caller
never attaches or detaches its console. `leaderPid`, `exited` and `exitCode`
describe the actual CLI, never the broker.

One absolute 5000ms startup handoff deadline covers preparation, admission,
reply, adoption and broker retirement. `Scope` creates its overall 1..5000ms
deadline before preparation and caps handoff and observation by its remaining
budget. It checks cancellation between phases and while polling; public tokenless
`Start` and `StartScope` remain tokenless. Synchronous Windows calls cannot be
preempted; a detected overrun fails with acquired custody retained. This private
ceiling matches Scope's existing maximum overall budget; it does not add another
5000ms to Scope. The initially proposed 2000ms ceiling failed first-use native
CLI launch, while a same-artifact repeat advanced through six SDK operations.
A fresh native consumer at the selected ceiling advances through all six
pre-interruption operations; this is bounded evidence on the admitted host,
not a universal startup-time guarantee.

The broker receives only a read-only snapshot mapping capability, one fixed
48-byte reply channel, a synchronization-only acknowledgement event and its
three standard handles. All writable snapshot views and capabilities close
before broker creation. The snapshot carries exact prepared command line,
application, directory and environment with checked aggregate lengths. Existing
32767-character Windows string ceilings and the configured environment byte
budget apply. The CLI inherits only standard handles. An older helper refuses
the explicit versioned launch mode before creating a CLI; no runtime compiler,
helper discovery or second public helper path is used.

Every broker and signal helper starts suspended and enters a fresh non-breakaway
nested job before resume; the CLI inherits that job and is independently verified.
Failed assignment, adoption, retirement or resume retains every known handle and
job for explicit disposal. A lost reply may leave an unknown suspended root in
the job: `leaderPid = 0`, `exited = false`, `exitCode = -1` remain unknown rather
than inventing a CLI exit. `RetainedOperation` exposes incomplete custody even
when startup fails or is cancelled. `Interrupt` requires admitted, resumed CLI
custody and refuses after the real root exits.

The job active-process limit derives from `maximumInventoryBytes` without a
silent increase. Allow for the transient broker, CLI and OS console members,
and later descendants plus a signal helper. Physical memory, disk and elapsed
invocation containment remain the caller's separately admitted runtime contract.

Capture overflow records failure while continuing bounded draining to genuine
EOF. Each output pipe has a requested 64 KiB capacity and each update reads at
most 64 KiB per stream; this bounds OS buffering and observation work without
throttling sustained output to one 4 KiB block per Windows polling quantum.
Retained capture still uses the caller's configured per-stream byte limit. `treeComplete` requires every retained process and thread to terminate,
zero active job members and actual broken-pipe EOF on both streams, independently
of whether the root result is known. Windows `reapedChildren` counts terminated
job members, including broker, signal helper and OS console members; it describes
kernel accounting rather than POSIX `waitpid` calls. Cancelling or expiring a wait
retains custody. `ForceDispose` attempts job and all known processes even after a
cleanup failure, preserving `forced = true`. Access-denied termination is accepted
only when the exact retained process subsequently signals, within one shared
250ms allowance capped by the caller's remaining disposal budget. A zero wait
adds no reconciliation wait. A forced custody observation never proves graceful
cleanup. Destructor fallback is also never acceptance evidence.

The production broker passes eight source-owned native cases: exact argument
bytes, actual BREAK receipt with custody, root exit with live writers, 2 MiB
overflow drainage, separate-group exclusion, lost offer, partial adoption and an
actual late offer. Their body and independent cleanup deadlines remain 8000ms
and 5000ms, with the default source budget. Fifty caller-console observations
across those cases and final transport closure retain identical raw facts.

Two additive input cases also pass with those eight cases, retaining the same
default source budget and cleanup deadlines. For the selected actual paths,
source reaches 32,766 UTF-16 command characters excluding NUL, rejects one more
before custody while retaining the independent 64 KiB UTF-8 argument budget,
and checks the actual child cwd and prepared environment. An explicit benign
ambient sentinel is absent from the child. All ten cases pass with eleven raw
receipts and 62 identical caller observations. This is the selected Windows
profile, not a universal path-size or platform promise.

The unchanged shared real-CLI consumer's nine operations and codec controls pass
with the original Windows PowerShell 5 held fixture after the native host-context
repair. An earlier explicitly selected portable PowerShell 7 profile also passed.
The original debugger/closure failures remain retained negative evidence. The
host owner remains cancellable through root wait and output EOF, then releases
ordinary redirected children to preserve early-return GUI/helper behavior.
Native regressions verify retained-child survival after launcher exits 0/17 and
cancellation after root exit. Remaining protocol/nested-job edges, broad product,
actual GUI, physical Windows and other native platforms remain separate work.
Component proofs do not establish full product acceptance.

Graceful interruption runs the pinned helper without changing the caller's
console. It verifies inherited stable process/job handles, bounded console
membership and a 1000ms deadline, attaches to the CLI's private console, and
requests group-targeted `CTRL_BREAK_EVENT`. It never broadcasts group zero.
A helper timeout or refusal is failure with custody retained. Actual event
delivery, application interruption, descendant closure and fixture cleanup need
native Windows proof; successful helper exit alone is no application assertion.
On helper failure, a separate explicitly inherited diagnostic channel reports
one ASCII packet of at most 128 bytes with the failed stage and the full Win32
error code. The client reads only an available bounded prefix after helper exit
and escapes it in the failure detail. Statuses 21, 22 and 23 distinguish an
existing helper console, failed `AttachConsole`, and failed control-handler
installation; status 24 identifies numeric argument conversion. A zero Win32
field denotes a validation failure without an associated failing Win32 call.
These diagnostics do not change launch flags, retry console attachment, signal a
different group, or change success/custody decisions.
The fresh child environment forwards only selected platform/tool/locale/home
bindings and `NO_COLOR`; it excludes inherited Dockpipe/provider/debugger and
application-state bindings. It explicitly sets `DOCKPIPE_BIN` to this client's
opened CLI volume-GUID path for public object scope queries, preserving the
service's existing precedence for a repo-local CLI. When there is no local CLI,
the query can use this selected artifact even when it is absent from PATH. The
inherited environment never selects that binding. A nonempty `stateDirectory` selects an existing
confined Windows per-user data base: the client supplies its opened volume-GUID
path as `LOCALAPPDATA`, which the Windows CLI uses for its own durable state and
data layout. `XDG_STATE_HOME` receives the same canonical path for compatibility;
it does not select the Windows CLI's authoritative state root. A nonempty
`cacheDirectory` is likewise opened, retained and supplied canonically as
`XDG_CACHE_HOME`. That compatibility field does not override Windows consumers
that select cache through other platform-specific APIs. These canonical
environment values are charged against the configured environment byte limit.

macOS native transport remains unimplemented. Source consumers may inject a
different transport without changing their authored workflow policy.

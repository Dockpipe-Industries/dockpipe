# 0.6 error handling and architecture audit — 2026-10-03

## Scope and disposition

Audited the working tree based on `67e787eb`, including the uncommitted 0.6
release, remote-node, secrets, and portability work. This is a repository-wide
triage plus deeper review of selected risk-bearing paths, not a claim that every
line or platform was verified. Production implementation was not changed during
the initial audit; subsequent remediation is recorded in the checkpoint below. The previously completed path cleanup remains separate.

The pass inventoried authored source sizes, checked Domain/Model imports for
dependencies on Application/Infrastructure, traced process cancellation and
result handling, examined package artifact persistence and cloud budget readers,
and inspected release publication and installation boundaries. PipeLang was
inventoried but its deferred compiler campaign was not requalified. Existing
TASK-035 historical rankings are not assumed to describe this working tree.

Four failures below were reproduced with temporary Go overlay tests, without
editing production files. The tests assert the observed defect, so their passing
status confirms the reproduction; it does not establish that the code is fixed.
No provider credentials, network services, release publication, or VM were used.

## Remediation checkpoint — 2026-10-03

The six primary behavior defects below now have focused working-tree fixes and
permanent regression tests. The original audit evidence is retained below as the
before-state; its temporary overlay tests intentionally assert the old defects
and are not the remediation acceptance suite.

| Finding | Implemented behavior | Verification |
| --- | --- | --- |
| A1 | Required budget counters reject missing, unreadable, malformed, null, fractional, negative, or overflowing values. Shell callers check reads before arithmetic and stop before rewriting a damaged ledger. Explicit initial creation remains separate. | Counter tests and the shell regression pass, including invocation through the lock's OR-list where `errexit` alone is ineffective. |
| A2 | Required request, plan, and edit JSON writes return errors; all preparation paths propagate them. Optional diagnostics emit a warning on failure. | Blocked `artifact.json`, blocked `patch.diff`, encoding failure, and valid bundle round-trip tests pass. |
| A3 | Resolver execution owns the process tree. Unix uses process groups; nested provider helpers preserve that group. Windows uses a job object and attaches the suspended process before resuming it. CLI interrupt/termination signals reach resolution contexts. | Direct, nested, cancellation, and normal-exit descendant tests pass on Linux with the race detector. Resolver and provider tests pass. Windows amd64 and macOS arm64 compile checks pass; native execution remains pending. |
| A4 | Immutable result files and their digests are committed before small broker metadata. Existing inline results migrate on open. Valid large results no longer consume the metadata capacity; all job IDs remain retained. Wire size accounts for base64 artifacts and escaped logs. | Three maximum artifact/log deliveries, restart retrieval, inline migration, digest mismatch, duplicate delivery, and healthy admission rejection at 128 jobs pass. |
| A5 | Typed safe HTTP errors distinguish permanent rejection from transient failures. Polling/delivery have five attempts with backoff. Revocation/conflicts/protocol errors return promptly; journals survive delivery failures. Explicit cancellation is distinct from deadline and heartbeat failure. | Retry budget, cancellation, recovery, revocation, heartbeat-vs-cancel, deadline, and existing journal-recovery tests pass. Remote tests also pass with the race detector. |
| A6 | ZIP core packages download to a unique file on the destination volume, verify their checksum, then publish by rename/replacement. Failed downloads preserve an existing package. | PowerShell 7 on Linux passes interrupted, mismatched, replacement, and first-install cases; Windows PowerShell execution is wired into the release job and remains pending. |

Broader regression testing also exposed an existing Windows-path candidate bug in
`provider_pool.go`: host-dependent `filepath.ToSlash` did not normalize persisted
Windows paths on Unix. The focused fix preserves existing hash identities and
adds the slash-normalized candidates expected by the existing regression test.
Both affected DorkPipe Go suites now pass in full. The orchestration shell lane
suite also passes, including cloud usage accumulation, with isolated test state
and the rebuilt helper. The application suite passes
with local HTTP fixture access; sandbox-only runs cannot open those listeners.

Refinement of A5's broader observation: `OperationResult` still uses `fail` as its
terminal event category, but its wrapper returns the original Go error unchanged
and records its text. Go callers retain `errors.Is` cancellation/deadline identity.
Changing the repository-wide serialized event taxonomy is a separate compatibility
decision, not a prerequisite for the remote cause fixes or a reason to add IResult.

The 128-job admission limit remains intentional. There is no implicit pruning or
archive CLI: download results and initialize a fresh private broker state when
full, retaining the old state and its `results/` directory for recovery. Never
reuse an old ID in a new state to bypass unresolved work. The DDD/DRY extractions
below were subsequently addressed in the [2026-10-04 cleanup](release-0.6-cleanup.md).
The publication-recovery rehearsal remains a follow-up.

No provider credentials, live cloud changes, release, commit, or push were used.
Generated changes are the authored embed manifest, regenerated with its owner
script. Test binaries, logs, caches, and isolated fixtures are under `/tmp`.
Native Windows/macOS execution, the full hosted release matrix, and the deferred
PipeLang compiler campaign are not established by this checkpoint.

## Ranked findings

### A1 — P1: corrupt cloud budget state becomes successful zero usage

Location: `packages/dorkpipe/lib/orchestrationhelper/orchestrationhelper.go:157`
and `:6342`; consumer
`packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-common.sh:1579`.

`readJSONMap` returns an empty map for every read or JSON error. Both usage-number
commands then return zero and success. The shell accounting code adds the new
task's usage to that value and evaluates the total budget against it. A truncated
or unreadable existing ledger therefore loses prior usage instead of stopping
accounting. Missing initial state and corrupted existing state are indistinguishable.

Reproduction: a truncated ledger containing a prior token count made
`Run(["usage-number", path, "total_estimated_tokens"], ...)` return `0\n`, nil.
The helper reproduction is direct; the downstream budget consequence is traced
through the shell consumer, not a paid-provider execution.

Recommended slice: separate explicit first-use initialization from required typed
ledger reads; return parse/I/O errors for existing state and stop budget accounting
before scheduling or rewriting it. Audit other uses of the permissive map readers
individually instead of changing their semantics globally.

Validation: absent initial state, truncated JSON, denied reads, wrong numeric
types, and valid nonzero usage; prove corruption cannot become a zero balance.

### A2 — P1: edit preparation reports success after losing its artifact

Location: `packages/dorkpipe/lib/cmd/dorkpipe/edit.go:2318` and
`packages/dorkpipe/lib/cmd/dorkpipe/structured_trace.go:188`.

`writeJSON` returns no error and discards both serialization and write failures.
`writePreparedArtifactBundle` uses it for the required `artifact.json`, then
checks only the patch write before returning success. This helper also serves
other edit/request paths, so its silent contract is wider than one caller.

Reproduction: make `artifact.json` a directory while leaving the artifact bundle
directory writable. Preparing a valid create-file edit returned nil error and
wrote `patch.diff`; `loadPreparedArtifact` then failed. This models a selective
artifact write failure without requiring disk exhaustion or changing permissions.

Recommended slice: return and propagate errors for required artifact writes;
publish a complete bundle before emitting ready/success. Classify optional trace
writes separately so diagnostics cannot hide loss of required recovery inputs.

Validation: artifact serialization/write failure, partial bundle failure, and
successful prepare/load round trip; no ready-to-apply result for incomplete data.

### A3 — P1: cancelling a secret resolver does not contain its child processes

Location: `src/lib/infrastructure/secretenv/resolve.go:62` and
`packages/secrets/tools/environment/runner.go:29`.

The resolver and vendor CLI runners use `exec.CommandContext` and `WaitDelay`
without process-tree containment. Cancellation kills the direct process but
does not establish that its descendants stopped. The existing resolver test uses
`exec sleep`, which deliberately removes the child-process case. Remote execution
already has separate containment code, illustrating inconsistent process ownership.

Reproduction: a resolver launched a short background child and waited. Resolve
returned `context.DeadlineExceeded`; afterward the child wrote its completion
marker. The child ended naturally during the test; no live provider ran.

Recommended slice: provide platform-appropriate process ownership for resolver
and vendor CLI execution and preserve cancellation causes. Also examine the
application boundary at `workflow_secret_environment.go:52`, which currently
starts resolution with `context.Background()` rather than accepting a caller context.

Validation: timeout/cancellation with children and grandchildren, no surviving
work, bounded return time, and continued suppression of secret output. Linux
proof alone is insufficient for macOS/Windows process semantics.

### A4 — P2: valid retained results can make the entire remote broker unavailable

Location: `src/lib/infrastructure/remote/broker.go:73`, `:203`, and `:314`.

Each job may return 8 MiB of artifacts, while all results are stored inline in one
32 MiB JSON state document. Submission does not reserve result capacity. Three
maximum-size results exceed the state cap after base64 encoding. `save` treats
this known pre-write capacity rejection like an uncertain durable write and
poisons the broker, disabling even health requests. The CLI has no archive/prune
operation despite the error telling the operator to archive completed jobs.
The separately documented 128-retained-job cap likewise has no supported retirement path.

Reproduction: three admitted jobs returned individually valid, request-size-compliant
8 MiB results. The third result failed persistence and the following health request
returned HTTP 503. No filesystem fault was required. The worker's local result
journal is retained; this finding does not claim that its artifacts are lost.

Recommended slice: define capacity admission and retention/reconciliation, keep
large results outside the mutable control-state document, and distinguish known
validation failures from uncertain commits. Do not simply remove the cap or
automatically requeue unknown jobs.

Validation: capacity exhaustion, result retry/restart, operator recovery, and
continued read/health access while full; preserve at-most-once execution.

### A5 — P2: permanent remote errors are retried silently

Location: `src/lib/infrastructure/remote/client.go:69` and `worker.go:43`.

Non-200 responses become untyped strings. The worker discards every next-job
error and retries every two seconds, so revoked credentials, conflicts, protocol
errors, and transient outages share the same silent behavior. A revoked worker
can remain alive indefinitely without an actionable diagnostic. Source-traced;
no live revocation or network test was performed in this audit.

The result taxonomy also conflates causes: heartbeat errors cancel execution;
Execute maps both cancellation and deadline expiry to `cancelled`; OperationResult
maps every returned error, including cancellation, to `fail`. These are distinct
boundaries, but their consumers cannot reliably recover the original reason.

Recommended slice: safe typed transport errors with status/retry classification,
bounded/backed-off transient recovery, actionable permanent failure reporting,
and preserved cancellation/deadline/connection causes. Keep provider response bodies
out of diagnostics. Define outcome mapping before adding a generic result wrapper.

### A6 — P2: Windows ZIP installation downloads unverified core into its installed location

Location: `release/packaging/windows/install.ps1:135`.

The core download goes directly to the global package directory before its
checksum is checked. A failed transfer or mismatch leaves the bad file there;
reinstalling the same version can overwrite a valid installed archive before
validation. Throwing afterward does not restore the prior file. Source-traced;
No PowerShell/Windows execution was performed during the initial audit.

Recommended slice: download to a unique temporary location, verify it, then
publish to the installed location. Preserve the previous installation on failure.

Validation: interrupted download, checksum mismatch, same-version reinstall, and
successful update under native Windows with a path containing spaces.

## Architecture and duplication opportunities

These are the original audit observations. See the
[cleanup checkpoint](release-0.6-cleanup.md) for the implemented splits, the
persistence contracts deliberately kept separate, and validation limits.

1. **Remote broker policy is coupled to HTTP and persistence.** The infrastructure
   broker owns enrollment rules, queue transitions, cancellation, retention,
   result validation, authentication, serialization, and durable writes. This is
   a responsibility/DDD concern with concrete consequences in A4/A5, although
   the import direction itself is valid. Extract pure transition/result policy
   into the remote domain and retain authentication/storage/HTTP in adapters.
   Preserve the uncertain-commit and idempotency contracts during any split.
2. **DorkPipe orchestration has several separable responsibilities in one owner.**
   `orchestrationhelper.go` is 8,185 lines and contains CLI dispatch, proposal
   parsing/validation, contract normalization, promotion decisions, budget reads,
   and filesystem publication. Size is only a triage signal; those distinct
   responsibilities and A1 provide the reason to split. Start with the typed
   budget ledger, not an indiscriminate file shuffle.
3. **Persistence helpers duplicate behavior with materially different guarantees.**
   Compare remote `WritePrivate`, infrastructure `writePrivateJSONAtomic`,
   orchestration `writeJSONFileAtomic`, promotion writers, and edit `writeJSON`.
   They differ in error propagation, link handling, privacy, replacement, and
   directory syncing. Define the required contract per caller, then share narrow
   mechanics where contracts match. A universal write helper could weaken the
   strongest boundaries and is not recommended.

No Domain/Model imports of Application/Infrastructure were found in the scanned
production Go files. Cloudflare and secret-provider specifics remain package-owned
in the new implementation. These are positive checks, not proof of complete DDD
conformance. Similar bounded buffers alone do not justify a new abstraction.

## Release sequencing and result conventions

Recommended before 0.6: resolve A1–A3, A6, and the remote capacity/error behavior
in A4/A5 before presenting remote benchmarking as ready. Structural extractions
can follow in bounded slices where they support those fixes. Do not block the
release on a repository-wide rewrite or mechanical DRY conversion.

Release recovery also needs a reviewed rehearsal: GitHub publication precedes R2
upload, while the metadata job rejects an existing release tag. A failure after
the GitHub release therefore requires a recovery procedure. The release docs
already acknowledge non-atomic publication; this is an operational follow-up,
not a newly asserted duplicate code defect. Do not overwrite a published version
or loosen the tag guard as a shortcut.

Use ordinary `(T, error)` internally, with checked writes, typed errors, and
deliberate context propagation. Use explicit domain outcomes where they drive
workflow state or cross a serialized boundary. None of the reproduced failures
requires a repository-wide `IResult` abstraction to fix.

## Initial reproduction evidence and limits

Temporary proof directory: `/tmp/dockpipe-hygiene-audit-20261003/`.
It contains four Go overlay tests and `overlay.json`; no production file was
replaced. Go 1.26.7 ran with local/offline module settings and a temporary cache.

Root module:

```bash
go test -overlay /tmp/dockpipe-hygiene-audit-20261003/overlay.json \
  ./src/lib/infrastructure/remote ./src/lib/infrastructure/secretenv \
  -run TestAudit -v -count=1
```

From `packages/dorkpipe/lib`:

```bash
go test -overlay /tmp/dockpipe-hygiene-audit-20261003/overlay.json \
  ./orchestrationhelper ./cmd/dorkpipe -run TestAudit -v -count=1
```

Observed reproductions: A1 zero usage on corrupt JSON; A2 successful preparation
with unreadable artifact; A3 child activity after cancellation; A4 third valid
maximum-size result poisons health. A5/A6 and structural observations are source
traces, not platform executions. This audit did not run the whole test suite,
hosted matrix, paid providers, live publication, Windows installation, or QEMU.
The proof files are temporary and should become regression tests in the owning
fixes. No commit, push, or remediation implementation was performed during that
initial evidence-gathering pass; see the subsequent remediation checkpoint above.

# Combined inner-selector and outer result arms in terminal tests

## Approved objective

- Objective: `TASK-021-terminal-combined-selector-arms`; state: `completed`.
- Founder selected A and separately said exact `approved` for v0.109.0.
- Baseline: clean saved `js/pipelang` at `f599a20e2f652f2e33645cba222bbf5d8d256475`.
  All 28 v0.108 source hashes, both protected stashes and ignored inventory match.
  Completed v0.108 proof is admitted.
- Admit `if ((a ? (b ? c : d) : (e ? f : g)) ? (h ? i : j) : (k ? l : m))`,
  with flat ternaries in either/both inner-selector arms and either/both outer result
  arms: nine new combinations. Allow any subset of terminal-test positions through
  statement depth three in public pure methods. Omitted visibility remains public.
- Named operands remain inherited nonconditional bool expressions, including supported
  pure calls and earlier lexical bindings. Reached operands/branches stay lazy and
  once-only; reached locals stay eager, once-only and source-ordered. Preserve lexical rules.
- Every v0.108 form remains available elsewhere in the same tree.
- Exclude further nesting, deeper statements, new return/initializer/arrow/argument/
  matching/propagation placements, hidden locals, private methods, inference, mutation,
  loops, effects, new backends and performance research.
- Preserve source -> typed HIR -> target-neutral Core -> evaluator/Core-only Go,
  executable Application IR, inherited nodes and public identities, frozen 45-source
  compatibility, package/engine boundaries and unchanged resource ceilings.
- Done when independent source/Core admission and refusals, all 25 statement shapes
  and applicable condition-position subsets across nine combinations, value/lazy-trace
  agreement with generated Go, computed/lexical cases, all 108 earlier-contract refusals,
  fresh discovered compiler tests, scaling and affected integration/editor/docs checks pass.
- Report precisely which input assignments are independent. The placement matrix has
  nine independent boolean inputs and three independent depth flips, reusing four
  operand inputs and local/return inputs across nodes. A separate root matrix exhausts
  all thirteen operand inputs independently for each of the nine arm combinations.
- Retain 128 MiB / 5-second warm direct compiler ceilings, normal inlining and execution GC.
  Cached offline Go 1.25.13; canonical containment uses 30-second units, 25-second
  children, 700 MiB reclaim, 1 GiB hard, 800 MiB proactive stop, zero swap, 128 pids
  and at most two units. Preparation alone may use GOMEMLIMIT=600MiB. Python uses `-B`.
- Execution skill: `dorkpipe-objective-execution`; automatic in-scope checkpoints;
  handoff only on user request. Implementation, version inheritance, tests and docs
  are authorized. No commit, push, cleanup, worktree, stash mutation, generated-store
  refresh, installation, credential change or live operation is authorized.
- Preserve receipts, caches and all existing protected/ignored/generated state.

## Evidence

Pre-crash receipts under `/tmp/pipelang-v109-proof` were lost during reboot.
Implementation and bounded verification are complete under aggregate containment.
Current receipts: `/home/jamie/.cache/pipelang-v109-resume-20260909`.


## Host memory incident — 2026-09-09

The founder reported that the run exhausted system memory and crashed the machine,
and requested system-log investigation before resuming. Verification was paused
for that investigation; the pre-crash observations alone did not establish completion.

Observed before interruption in this task's tool output:

- The freshly discovered compiler suite reported all 810 functions passing in 6,187
  contained units, with unchanged source. The 15 new functions occupied 1,929 units.
- The new checks reported 6,498 layouts, 26,615,808 placement value/trace vectors,
  73,728 independent root-operand vectors, and 1,944 in-suite scaling cases.
- All nine integration checks passed after correcting an inherited consumer
  diagnostic-text expectation. Source/Core production audit and editor/docs checks passed.
- The last successfully observed isolated result was 637 accepted measurements out
  of 1,944. Later isolated completion and host safety are not established.

Post-reboot investigation:

- The previous boot ended at 20:13:19 EDT; the new boot began at 20:14:14 EDT.
- Readable previous-boot journal, kernel journal, `/var/log/syslog` and
  `/var/log/kern.log` contain no matching OOM-killer event identifying a culprit.
  This absence does not disprove memory exhaustion.
- Syslog records application crash handling and six GNOME SettingsDaemon services
  exiting with SIGKILL at 20:09:57 EDT. These records do not establish who killed
  them or which process consumed memory.
- The founder authorized a privileged read of selected diagnostic fields from the
  root-owned apport report dated 19:11:42, using the system authentication dialog.
  Apport failed with `FileNotFoundError` opening the vanished process's `cwd` while
  handling a .NET SIGABRT (`/home/jamie/.dotnet/dotnet`, host PID 1228425).
  This report does not identify an OOM cause. Its recorded memory figures describe
  the crash reporter, not whole-host memory or the original .NET process.
  The ChatGPT crash report in `/var/crash` is dated September 4 and is not evidence
  of this incident's cause.
- A read-only host process check found no Python test/measurement runner, PipeLang
  test binary, compiler or systemd-run process still running. No tests were resumed.
- `/tmp/pipelang-v109-proof` no longer exists after reboot. Pre-crash numeric
  results above are retained conversation observations, not currently readable receipts.
  Source changes remain present and uncommitted.

A containment gap is confirmed by source inspection: `run.py` limits the inner
systemd test unit, while the outer suite/matrix coordinators and report processing
run outside that unit's memory cap. `pipelang_suite.py` also retains accumulated
reports and joins all batch output into one string. This is a plausible exposure,
not a proven cause of this crash; the suite had reported completion before the
interruption during isolated measurement.

The investigation identified aggregate containment, bounded report processing and
receipts outside volatile `/tmp` as prerequisites for resumption. No containment
repair, retry, installation, credential change, commit or cleanup was performed
during that investigation. The founder subsequently authorized the resumption below.

## Authorized resumption

The founder said “your clear now to continue” after the investigation. HEAD and both
protected stashes still match the admitted baseline; the 28 v0.109 changed paths
survived. The temporary receipts and caches did not. The necessary containment
repair adds a shared temporary slice for coordinators and all child units, streams
log inventory, and retains resumed receipts in the private durable directory
`/home/jamie/.cache/pipelang-v109-resume-20260909`. Three small live aggregate probes
passed (nested containment and cleanup, independent deadline cancellation, and
uncontained-coordinator refusal). Existing per-unit limits and compiler ceilings
are unchanged. Completion is recorded below; this does not assert a crash cause.

Resumption preparation: all five existing per-unit probes also passed. The offline
harness rebuild completed in its unchanged 30-second unit; the shared job peaked
at 742,240,256 bytes with zero swap and no max/OOM events, and removed its cgroup.
The complete resumed suite rediscovered 810 functions in 6,187 units (15 v0.109
functions in 1,929 units). Static syntax, formatting, editor checks and all 234
local documentation links passed. Both protected stashes and all pre-existing
ignored paths remain unchanged. The probes created one task-owned ignored file,
`tests/containedexec/__pycache__/job.cpython-310.pyc`; existing bytecode was preserved.

## Resumed compiler and consumer evidence

The complete discovered inventory finished in 6,187 units across 810 functions,
with unchanged source. Two inherited v0.94 subset units (`/97` and `/99`) reached
the unchanged 25-second deadline during cold native-bundle preparation. Their
original receipts remain in `suite/batch-881.json` and `suite/batch-883.json`; exact
command retries with the populated artifact cache passed in 3.341 and 3.340 seconds.
No execution GC setting, deadline or memory ceiling changed. The suite and retry
cgroups were removed; neither job recorded max/OOM or swap events. The full-suite
aggregate peak was 1,613,160,448 bytes under the new 2 GiB shared cap.

All 1,929 v0.109 units passed on the first resumed run; the maximum unit duration
was 21.498 seconds. Log reconciliation confirms 6,498 unique layouts, including
225 no-new-test controls, across all 25 statement shapes, all nine arm combinations
and every subset. The placement matrix checked 26,615,808 value/trace vectors.
Its nine independent operand bits and three depth flips are reused across nodes;
four named operand positions and local/return inputs share those bits. A separate
root matrix exhausts all thirteen selector operands independently for each arm
combination: nine layouts and 73,728 vectors. Evaluator and pristine generated-Go
values agree with the handwritten oracle; instrumented generated-Go traces also
agree with independently specified reachability and order.

All 108 scaling families passed: 1,944 cases, including 216 zero-local controls,
and 11,664 exported fixture files. In-suite direct-compiler measurements peaked
at 71.801 MiB and 1.877 seconds. Closure depth remained bounded by the one-local
baseline for every larger count, through 256 locals. Separate fresh isolated compiler
measurements subsequently passed, as recorded below.

All nine integration checks passed: Core, HIR, evaluator, Go backend, executable
Application IR, frozen 45-source compatibility, application-focused checks, full
CLI tests and vet. Cold application preparation initially hit its 30-second unit
limit without OOM; the same preparation passed after its cache had populated,
and all subsequent checks passed with unchanged limits. Both original and
continuation receipts remain under the durable proof root. No source edit was
needed for either preparation timeout or the two inherited test timeouts.

## Completion — 2026-09-10

The accepted receipt is
`/home/jamie/.cache/pipelang-v109-resume-20260909/accepted-verification.json`.
It reconciles all 810 discovered functions and all 6,187 scheduled units across
6,189 attempts, retaining the two initial deadline failures and their passing
focused retries. It also verifies unchanged compiler/harness source, complete
layout and vector coverage, all nine integration checks, all 1,944 isolated
fixtures, and removal of every validation cgroup. `accepted-artifacts.json` records
12,032 executed native artifacts; it does not authorize pruning.

The fresh isolated compiler matrix passed every case with normal inlining and
normal execution GC. Maximum waited compiler RSS was **71.977 MiB** and maximum
elapsed time was **0.144 seconds**, below the unchanged **128 MiB / 5-second**
ceilings. Maximum generated closure depth was seven and did not grow beyond each
family’s one-local baseline through 256 locals. The isolated job peaked at
771,428,352 bytes and removed its cgroup without max/OOM or swap events.

The final receipt and streamed artifact-inventory audit ran inside the shared
job cap and the 512 MiB coordinator cap. It passed in 3.929 seconds, peaking at
254,676,992 bytes for the entire audit job, with zero swap and no max/OOM events;
its cgroup was removed. Three aggregate containment probes, five existing
per-unit probes, four planner/inventory tests, editor assertions, syntax/format
checks and documentation-link checks passed.

Production changes remain confined to the eight PipeLang admission/version-
inheritance files; evaluator and Go-emitter implementations, Core node shapes,
public identities and package/engine boundaries remain unchanged. The test harness
adds aggregate containment and streamed log processing. Source tests, executable
consumer fixtures, editor snippets and canonical/task documentation cover v0.109.
No new language seam, backend or performance experiment was introduced.

HEAD remains `f599a20e2f652f2e33645cba222bbf5d8d256475`; both protected stashes and
the pre-existing ignored-path inventory are preserved. The 33 owned source/docs/test
paths remain uncommitted. Private receipts, caches, binaries and fixture exports
are retained under the durable proof root; the single new ignored Python bytecode
file described above is also retained. No commit, push, publication, cache/store cleanup,
worktree, generated-store refresh or successor selection was performed. The host
crash’s original cause remains unconfirmed; the verified containment repair closes
the identified verification-coordinator exposure.

# General lexical blocks — P04.a

Objective: `TASK-021-general-blocks`. State: executing.
Cache reset approved by the user on 2026-09-15: all PipeLang verification caches,
retaining source, installed toolchains/SDKs, fixtures, receipts and failed-run logs.
Current boundary: regenerate caches and complete fresh verification after dependency drift.
Execution skill: `dorkpipe-objective-execution`.
Authority: founder selected P04.a, reviewed the concrete scope, then said `approved`.
Checkpoint policy: automatic within objective. Handoff: user requested only.
Terminal conditions: completed, blocked, failed_verification, cancelled.

Implement nested lexical blocks, initialized immutable locals, sequential Boolean
if/else branches with joins, and early callable returns in supported pure methods.
Preserve left-to-right, once-only evaluation and lazy branches. Locals cannot escape
their block or shadow an existing binding. Reject missing returns, unreachable
statements, invalid references and malformed Core independently of source lowering.
The successor contract is v0.114.0; it is not accepted until terminal proof passes.

Done when source/typechecking, typed HIR, independently validated target-neutral Core,
evaluator and Core-only Go agree with independent value/order/refusal oracles; affected
semantic/editor/reference projections agree; focused checks, a fresh complete terminal
campaign and independent acceptance pass with inherited compatibility and containment.

P04.b owns mutable slots and delayed initialization. P05 owns loops, recursion and
logical fuel. Unrelated D1–D6 proposals remain pending. No new native backend, Qt/app
work, profiling, delegation, worktree, install, commit, push, publication or
external resource mutation. Cleanup is limited to the separately approved cache plan. Preserve package/engine boundaries and inert analysis.

Original admission: saved checkout `/home/jamie/source/dockpipe`, branch `js/pipelang`, HEAD
`1c839e3ce92591c845fc647970d453bcad6dffea`; staging/tracked/untracked inventories empty.
Both protected stashes and all 28 prior authored postimages matched the clean-handoff
receipt. Completed P01 and Qt objectives remain admitted evidence, not active work.
Own compiler/control-flow implementation, focused tests, affected language tooling and
canonical/task documentation. Preserve failed evidence and all caches outside the approved pruning candidates.

Verification retains offline Go 1.25.13, 128 MiB/5-second direct source probes,
2 GiB aggregate/1536 MiB high, 512 MiB coordinator, 1 GiB unit/700 MiB high, zero swap,
96 GiB inclusive estate and 8 GiB reserve. Preparation-only GOMEMLIMIT remains 600 MiB.
Evidence root: `/home/jamie/.codex/visualizations/2026/09/13/01a09d03-2870-7022-be17-0e22f9f2291b/general-blocks`.

Current checkpoint: focused regeneration passed; run fresh complete verification
from regenerated caches, then independent acceptance. See the cache reset below.

Focused checkpoint: `focused-2-job.json` passed block values/native execution,
lexical/refusal and forged-Core cases, call-order instrumentation, inherited scalar/
record/carrier composition, and nine initial compiler scaling fixtures. The initial
HIR helper typo and call-admission routing failure are fixed; failed runs remain.
Further terminal preparation adds larger local/branch scaling, HIR/enum regressions,
Application IR/editor integration and the isolated-matrix fixture schema.

Final focused checkpoint: `focused-4-job.json` and its acceptance record passed
all selected v0.114 checks, 12 direct compiler fixtures (maximum 28.598 MiB /
0.039 seconds), Application IR and editor. The prior task-owned runner indentation
error is retained in focused-3; it started no compiler/tests. Current Go/editor
dependency scope excludes unrelated Qt system-library inputs while retaining all
consumed Go modules, toolchain, source, caches and task proof. No limit changed.
Next: one fresh full campaign with a private executable/native-build cache, followed
by independent acceptance and inherited ordered audit reconciliation.

The first full terminal was stopped after `TestCheckedChainInheritanceAdmission`
reached its inherited 25-second deadline with the cold private cache. It reported
no semantic mismatch. `inherited-probe-job.json` reran that exact test with retained
verified artifacts, the same deadline and fresh native execution; it passed. All
process trees were removed. The suite now exports the new block memory fixtures
for its isolated matrix. A fresh terminal-2 is required after that harness correction;
no semantic receipts from the interrupted terminal are counted as final proof.

Terminal-2 was interrupted after an inherited local-sequence cold-cache deadline
and a skipped C++ export helper. The canonical suite now supplies a task-owned
export directory and binds its outputs as required artifacts; it runs no C++/Qt
compiler or application campaign. `inherited-probe-2-job.json` passed both exact
cases (local sequence 5.444 seconds under the unchanged 25-second deadline).
Fresh terminal-3 follows the harness change. Completed semantic receipts will
remain reusable only within an unchanged campaign and input identity.

Terminal-3 passed more than 880 cases without failures before source review found
the unsupported-contract diagnostic still naming v0.113.0 as the maximum. The
message now correctly names v0.114.0; no admission behavior changed. Its aggregate
was interrupted and removed cleanly. Terminal-4 is the fresh campaign for the
corrected source identity. Earlier runs remain evidence, not final acceptance.

Historical reconciliation distinguishes unchanged generated source/test order and
fresh native counts from fixture representation bytes. Thirteen completed prior
Go fixture conversions predate this objective but postdate the full v0.113 baseline.
All 175 inherited test source files match the admitted HEAD exactly. Final proof
requires unchanged generated source, test order and native counts; raw fixture
deltas are allowed only in those named completed conversion families, whose
independent value/trace oracles execute freshly in the complete campaign.

Terminal-4 exposed a cold native-build deadline in inherited v0.94 subset 96
and a pre-existing transport-test setup failure: the now-contained durable TMPDIR
makes its Unix socket path exceed Linux's pathname limit. The transport test now
uses `/proc/self/fd/<directory>/socket` to address the same open private temporary
directory. Its protocol, seals, digest, valid/refusal and deadline assertions are
unchanged. `focused-5-job.json` rebuilt the compiler and passed that test plus block
refusals under unchanged containment; its complete tree was removed. This necessary
verification repair is the sole inherited test-file edit; the other 174 test files
remain byte-identical to admitted HEAD. The inverse patch restores the exact old
transport-test bytes, bound in `admitted-oracle-inputs.json`.
Fresh terminal-5 owns final proof after this input change. Cold-cache deadlines
alone will be retried by resuming unchanged terminal-5 after its sweep completes.

Verification checkpoint: terminal-5 produced 4,373 passing suite receipts, including
all nine selected v0.114 logical cases. Two temporary-directory inventory races
were handled by unchanged-input resumption. The final resumed phase stopped at
the fixed disk-budget/headroom boundary; every aggregate tree was removed, with
no OOM or swap. Complete suite/matrix/integration acceptance remains pending, so
v0.114.0 is not accepted and v0.113.0 remains the accepted language baseline.

Recovery proposal: `E/cache-recovery-proposal.json` and the digest-bound
`E/cache-prune-candidates.jsonl` identify 23,686 unreferenced, single-link native
build-cache data files (9,888,690,176 allocated bytes, about 9.2 GiB). The plan
retains 119 cache paths referenced by existing proof, all metadata/index files,
verified executables, fixtures, receipts, historical evidence and other cache
roots. No receipt artifact or declared immutable input directly names the candidate
cache data. This proposal originally awaited separate cleanup authority; the user
subsequently approved it, as recorded below. No cap was raised.

All 37 authored postimages matched before this status-only documentation update.
The three updated task-state documents are outside the declared verification input
graph; executable source, harness and editor inputs remain frozen.

### Approved cache maintenance

The user approved the proposed cache-only pruning, including the same bounded
maintenance between stopped runs if needed. `E/cache-prune-1.json` records removal
of exactly 23,686 listed files: 9,839,091,605 logical bytes and 9,888,690,176
allocated bytes. Filesystem available space increased by 9,880,379,392 bytes during
the operation; that observation may include concurrent filesystem activity. All
119 referenced cache files retained identical hashes, and every other cache file
was preserved. Executables, receipts, fixtures and other cache roots were untouched.
Verification resumes under the unchanged limits; v0.114.0 remains unaccepted.

The first post-prune resume stopped during receipt validation at the coordinator
headroom guard, with no OOM/swap or new cases. An unchanged retry completed that
validation and advanced to 5,088 recorded passing cases before the suite refused
a stale storage-controller heartbeat. Both aggregate trees were removed. The
second approved maintenance pass scanned 78,480 proof text files and removed
7,754 eligible cache data files (3,115,099,401 logical / 3,131,203,584 allocated
bytes); all 119 protected hashes and all other cache files were preserved.
`E/cache-prune-2.json` records this operation. Terminal-5 resume-5 continues with
identical compiler/harness/editor inputs and unchanged limits.

### Cache recovery terminal checkpoint

Four approved maintenance passes removed exactly 40,778 native build-cache data
files: 17,181,340,502 logical bytes and 17,266,806,784 allocated bytes (16.08 GiB).
All 119 protected cache-file hashes and all other cache files were preserved at
each maintenance boundary. Verified executables, receipts, fixtures and other
cache roots were retained. `E/pruning-recovery-result.json` records the counts,
checks and individual reports; filesystem available-space deltas are observations,
not isolated physical-I/O or exact temporary-peak measurements.

The current terminal-5 records 6,129 passing suite cases out of 8,874, leaving
2,745 cases, the isolated matrix, integration/editor stage and independent final
acceptance pending. All nine v0.114 selected logical cases remain recorded as
passed. Resumes advanced proof without changing compiler/harness/editor inputs.
The most recent resume stopped during retained-proof validation at the
coordinator memory-headroom guard. Every terminal-5 aggregate tree was removed;
no OOM or swap occurred. Earlier stops and their receipts remain preserved.

After the fourth pruning pass, the next full inventory measured 102,628 MB
allocated (decimal), leaving about 0.45 GB below 96 GiB. All 11,420 executable
cache entries are referenced by retained proof. Superseded failed campaign
directories account for only 483,921,920 allocated bytes and release no otherwise
unreferenced executable entries. No broader deletion is proposed as an adequate
recovery from those small directories, and none was performed. Completion needs
a revised verification retention/resource plan; the fixed caps and protected
proof cannot be silently changed. The objective is blocked and v0.114.0 remains
unaccepted, with v0.113.0 still the accepted baseline. No commit or push occurred.

### 2026-09-15 verification recovery

Live admission was clean on `js/pipelang` at
`177f7e6d8e47700cc75e2165c51c5798364ddd27`, equal to the local upstream ref.
Both protected stashes and all six handoff routing hashes matched. This commit
contains the implemented compiler and subsequent artifact/native-stream work;
its title does not establish terminal acceptance.

Canonical controller and suite fingerprint comparisons each found four changed
files and sixteen additions against terminal-5, including the parser and suite
harness. A fresh campaign is required; the historical 6,129 passing cases remain
retained evidence, not current acceptance. Completed P01, Qt and SDK/composition
proof remains admitted supporting work.

A bounded read-only inventory measured 102,628,446,208 allocated file bytes in the
declared estate plus new task evidence, leaving 450,768,896 bytes beneath 96 GiB.
The audit completed with zero OOM/swap and verified tree removal. This sampled
inventory is not a live campaign capacity gate. Optional artifact packing keeps
raw executables and releases no space by itself.

The recovery report proposes a one-time, reference-checked pruning of data in the
separate shared Go cache. Its exact candidate/protection manifests require new
cleanup approval; the previous native-build-cache approval does not cover this
root. The read-only audit scanned 905,065 proof files, protected 250 present cache
references and identified 125,099 eligible files totaling 39,822,053,376 allocated
bytes (37.09 GiB). Its digest-bound manifest and full limitations are in the report.
The bounded audit completed without OOM/swap and removed its tree; coordinator
hard-cap reclaim events mean future reconciliation headroom is still unproven.
No cache data was removed. Before a fresh full run, also account for the
compiler/header/library inputs used by the newly discovered native-stream C++
tests; the old Go-only discovery does not bind that closure. Keep all existing
proof requirements and limits. P04.a remains blocked and v0.114.0 unaccepted.

Evidence and reviewable recovery plan:
`/home/jamie/.codex/visualizations/2026/09/15/01a0a70d-2a69-7bf1-a168-538d71be58b9/p04a-recovery/README.md`.
This continuation changes only task documentation and audit evidence; it performs
no commit, push, worktree, delegation or native-backend promotion.

### Approved complete cache reset

The user superseded the narrow proposal with “wipe all of our caches” and confirmed
all PipeLang verification Go/build/executable/packed-artifact caches while retaining
source, installed toolchains/SDKs, fixtures, receipts and failed-run logs.
`cache-reset-result.json` records removal of 911,714 files across 96 identified
cache roots: 177,219,245,753 logical bytes and 180,409,221,120 allocated file bytes
(no hardlinked cache inodes). Filesystem available space increased by
180,404,092,928 bytes, an observed delta that can include concurrent activity.
The source/toolchain/SDK fingerprint and Git/stash state matched afterward; no
eligible cache payload remained. Empty cache directories and lock files remain.
The contained reset job completed with zero OOM/swap and verified tree removal.

Receipts remain historical records; deleted cache artifacts must regenerate before
reuse. No old passing receipt is promoted into fresh proof. A necessary verification
repair now supplies explicit installed C++ support inputs and a pinned compiler to
the two newly discovered native-stream execution tests. Their flags and independent
oracles are unchanged. Focused verification and the new complete campaign are next;
v0.114.0 remains unaccepted and v0.113.0 remains the accepted language baseline.

### Fresh focused regeneration

`rebuild-focused-3-job.json` passed in 87.23 seconds: 31 harness tests, installed
C++ header closure, compiler rebuild, v0.114 block checks, both native-stream
execution checks, Application IR and editor. Aggregate peak was 1,159,667,712
bytes, with zero OOM/swap and verified tree removal. The final sampled estate was
7,044,517,888 allocated bytes. No resource limit or semantic oracle changed.

The installed input closure exposed quadratic root normalization; canonical roots
and coverage now use ancestor-set membership while preserving validation and
accounting. A focused independent root-union/refusal test passed. An inherited
symlink allocation assertion now uses actual filesystem blocks for long durable
temporary paths. The interrupted preflight and failed assertion attempts remain
recorded. Fresh terminal proof is running at the recovery evidence root under
`verification-data/campaigns/terminal`; `terminal-job.json` records live state.
Discovery includes 892 functions / 8,882 logical cases, with no inherited removal.
Independent acceptance is prepared in `accept.py` and must run only after the
complete stages and aggregate cleanup succeed. v0.114.0 remains unaccepted.

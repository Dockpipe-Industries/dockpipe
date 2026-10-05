# Mutable locals and definite assignment — P04.b

Objective: `TASK-021-mutable-locals`. State: failed_verification; proposed v0.115.0 is not accepted.
Authority: founder selected P04.b, reviewed the concrete scope, then said “I approve”.
Execution skill: `dorkpipe-objective-execution`. Checkpoints advance automatically within scope.
Handoff: user requested only. Terminal conditions: completed, blocked, failed_verification, cancelled.

## Approved scope

Add explicit `mutable int count = 0;` and simple `count = 1;` assignment in supported
public pure method blocks. Parameters stay immutable. Existing local declarations remain
immutable. Both mutable and immutable locals may omit their initializer; immutable locals
can be assigned only once on any execution path. No implicit default initialization.

Reads require definite initialization on every continuing path. Branch joins intersect
definitely initialized bindings and union possibly initialized bindings; returned branches
do not join. Nested blocks may assign enclosing locals, but cannot leak declarations or
shadow existing bindings. Assignment evaluates its right-hand side once, left-to-right,
before updating the binding. Preserve lazy branches and current value-type admission.

Implement source/typechecking, typed HIR, independently validated target-neutral Core,
evaluator, Core-only Go generation, affected semantic/editor projections and documentation.
Done when independent value/order/refusal checks, a fresh complete canonical campaign,
independent ordered inherited audit, frozen-input readback, cleanup and final storage proof pass.

Excluded: P03 nested records, loops/recursion/fuel, field mutation, closures, compound
assignment, new backends, production adoption, wholesale D1–D6 approval, unrelated efficiency
work, cleanup, SDK installation, commits, push, publication, external mutation and delegation.
Generic engine/package boundaries and independent oracles remain intact.

## Admission and verification

Saved checkout `/home/jamie/source/dockpipe`, branch `js/pipelang`, clean at
`e5f5e2ebbb9e17c201d941d533d73de0d31a238d`. Protected stashes remain
`26ea507907550d2449dc6f9c81b9942bd52d8629` and `e3afeea1dad94ca0c63dac434f0873548875bfc5`.
P04.a / v0.114.0 remains the accepted baseline. Its acceptance manifest, ten recorded
source postimages and all 13 private Nucleon postimages matched during receiver admission.
Nucleon and its installed SDK are outside this objective.

Retain 2 GiB aggregate, 512 MiB coordinator, 1 GiB units, zero swap, isolated compiler
128 MiB / 5 s, inclusive estate 96 GiB and 8 GiB reserve. Offline Go 1.25.13 remains pinned.
No passed baseline campaign is replayed during admission. Fresh final proof follows the last
material source change. Preserve failed receipts and use sequential full-estate audits.

Current checkpoint: implementation and focused checks pass; terminal acceptance is blocked by coordinator headroom. No verification job remains running after closeout.

Evidence root: `/home/jamie/.codex/visualizations/2026/09/16/01a0ac95-5a43-7013-a865-a6c1941137a4/p04b`.
Source and Core independently track definite/possible assignment. HIR retains declaration
mutability and bound assignment targets; Go/evaluator updates preserve copied values and
per-invocation state. Declaration-only types participate in backend support and enum/schema
checks. The propagation parser restores the incoming contract; inherited HIR admission includes
v0.115.0. Existing record construction and list append operand restrictions remain unchanged.

Three containment probes and 13 harness checks passed. Focused attempts are preserved:
initial coordinator inventory headroom refusal; one stopped cold compiler build; fixture syntax/
operand corrections; declaration-only Optional support repair; inherited HIR/version restoration
repair; native propagation test assertion repair. The current focused runner uses the existing
contained-unit inventory arrangement and preparation-only `GOMEMLIMIT=600MiB`, without changing
any limit. No full terminal acceptance is claimed. `inherited-test-postimages.json` binds 191
unchanged inherited test files; the planned independent audit compares directly against accepted C9.


Focused completion: `focused-8-job.json` completed with zero OOM/swap and complete
process-tree removal. All eight new compiler test functions (including twelve scaling
fixtures), the P04.b Application IR consumer and editor checks passed. Prior focused runs
also exercised inherited P04.a checks. `terminal-source-postimages.json` freezes 17,042
source files. Fresh `verification-data/campaigns/terminal` uses accepted C9 as baseline,
existing verified native stores, singleton scheduling and unchanged acceptance ceilings.


Terminal first attempt admitted 900 functions / 8,892 logical cases, then stopped after
40 accepted cases at the existing coordinator memory-headroom guard. No OOM/swap or
semantic failure was observed; the process tree was removed. All 17,042 source postimages
still matched. `suite-one-worker-job.json` uses the established `--workers 1` recovery on
that same campaign. Canonical downstream completion and independent acceptance remain required.

The first single-worker segment reached 2,600 accepted cases in 3,923 seconds before the
same coordinator headroom guard stopped it. There was no OOM, swap use or reported semantic
failure; its process tree was removed. All 17,042 frozen postimages still matched. The next
single-worker segment resumes the same receipt-backed campaign with unchanged limits;
both stopped job receipts remain preserved.

The second single-worker segment advanced to 5,500 accepted cases, then reached the same
coordinator headroom guard after 4,947 seconds. It also had no OOM/swap, removed its process
tree and retained matching frozen inputs. `suite-one-worker-3-job.json` continues from the
verified receipts without changing source, limits or the required final acceptance sequence.

The third single-worker segment reached 7,800 accepted cases before the coordinator headroom
guard stopped it after 2,254 seconds. Frozen inputs and cleanup again verified, with no
OOM/swap. `suite-one-worker-4-job.json` resumes the remaining cases under the same limits;
the failed owner receipt and dependent storage-owner refusal remain preserved.

The fourth segment completed in 2,265 seconds. Suite reconciliation accepts all 8,892 cases
from 900 functions, with no missing/failed cases, unchanged source/toolchain inputs, zero
OOM/swap and complete process-tree removal. All 17,042 frozen postimages still match.
`terminal-resume-1-job.json` runs canonical downstream verification. Full language acceptance
still requires its completion, the independent inherited audit and final storage proof.

The first canonical resume stopped during completed-suite receipt validation at the
coordinator headroom guard (362 seconds), without OOM/swap. Its process tree was removed,
all frozen postimages matched, and the completed suite evidence remains intact.
`terminal-resume-2-job.json` retries the unchanged canonical command, following the accepted
P04.a resume path, with coordinator memory diagnostics retained for any repeated stop.

The second canonical resume stopped after 108 seconds at the same guard. Diagnostics showed
retained process memory exhausting coordinator headroom; no OOM/swap occurred, and cleanup
completed. Dependency admission constructed two overlapping live input guards. The narrow
verification repair extends the existing guard with discovered module roots, retains all old
watches, installs new watches before hashing and rehashes the complete closure. A failed
extension permanently refuses that guard. All limits and resource checks remain unchanged.

Fifty harness tests pass, including mutation/restoration during extension, new-module changes
and failed watch installation. The initial contained test launch is preserved: two inherited
crash-recovery tests require their ordinary standalone context, and an incorrectly named test
module was requested. The corrected standalone test selection passed.

Because the harness changed, the earlier 8,892-case suite is retained as historical proof,
not terminal acceptance for the revised inputs. A fresh campaign now runs under evidence root
`/home/jamie/.codex/visualizations/2026/09/16/01a0ac95-5a43-7013-a865-a6c1941137a4/p04b/guard-1`,
with 17,043 frozen postimages and no semantic receipts copied from the earlier campaign.

That guard-1 admission still exhausted headroom after 117 seconds; no OOM/swap occurred and
cleanup completed. The follow-up removes the workflow parent's redundant full-manifest parse
and releases its unused file-hash rows after persistence. The guard retains its digest and
all live watches. Extension releases superseded file rows before fully rehashing the expanded
closure. Fifty-one harness tests pass, including continued drift detection after release and
full closure reconstruction; the controller test double now supplies the same guard API.
Fresh evidence is under the sibling `p04b/guard-2` directory, with 17,043 frozen files.

Guard-2 passed admission and accepted 400 fresh cases before execution reached coordinator
headroom after 426 seconds. Frozen inputs, zero OOM/swap and cleanup verified. Its
`suite-one-worker-job.json` continues the same revised campaign through supported singleton
recovery; this retains fresh proof requirements rather than reusing the pre-repair suite.

Guard-2's first singleton segment reached 3,220 accepted fresh cases before the headroom
guard after 4,307 seconds. All 17,043 inputs still match; no OOM/swap occurred and the tree
was removed. Its second singleton segment resumes the same campaign with unchanged limits.

The second guard-2 singleton segment reached 5,120 accepted fresh cases before headroom
stopped it after 3,749 seconds. Frozen inputs, cleanup and zero OOM/swap verified again.
Its third singleton segment resumes the remaining cases with all prior receipts preserved.

The third guard-2 singleton segment reached 8,480 accepted fresh cases before headroom
stopped it after 3,705 seconds. Frozen inputs and process-tree removal verified, with no
OOM/swap. The fourth segment resumes the final cases and reconciliation under the same caps.

The fourth guard-2 singleton segment reached 8,680 accepted cases before the same
headroom guard after 865 seconds. All 17,043 frozen inputs, cleanup and zero OOM/swap
verified. The fifth segment resumes the remaining 212 cases with unchanged limits.

The fifth guard-2 segment reached 8,720 cases before headroom stopped it after
637 seconds; frozen inputs, zero OOM/swap and cleanup verified. The sixth segment
continues the remaining 172 cases under the same ceilings.

The sixth guard-2 segment reached 8,820 cases before the same headroom stop after
704 seconds. Frozen inputs, cleanup and zero OOM/swap verified. The seventh
segment resumes the final 72 cases and reconciliation with unchanged limits.

The seventh guard-2 segment stopped during receipt admission without adding cases
(588 seconds); cleanup and zero OOM/swap verified. The eighth records a
coordinator-only allocator experiment: PYTHONMALLOC=malloc, MALLOC_ARENA_MAX=1,
MALLOC_TRIM_THRESHOLD_=131072. Fifty-one harness tests pass with this configuration.
Source/harness hashes, semantic receipt identity, toolchains, compiler environments and
all numerical limits stay unchanged. Native/test units receive only the existing explicit
systemd environment. This is allocator recovery evidence, not a performance claim.

The coordinator allocator experiment also stopped during receipt validation without
adding cases. Live samples reached 353 MiB anonymous memory; it is not adopted as
a demonstrated solution. No OOM/swap occurred and cleanup completed. The suite
retained both its build-input map and full guard hash rows after computing the build
key. It now releases those redundant copies after bootstrap; the saved complete
dependency manifest, digest and continuous drift watches remain. Sixty-four harness
tests pass. This source change requires another fresh campaign: guard-3 freezes
17,043 files, uses the ordinary allocator and copies no semantic receipts. Guard-2
8,820-case proof and every failed attempt remain preserved as historical evidence.

## Verification boundary — 2026-09-17

Guard-3 stopped after 127 seconds at `inventory coordinator memory headroom exhausted`,
before accepting any fresh cases. Its process tree was removed, with zero OOM and swap.
The small coordinator-memory repairs and allocator trial have not established a complete
canonical run under the fixed limits. Further unchanged retries are not supported by the
latest no-progress results. The objective is `failed_verification`, not accepted or complete.

The latest source is frozen in `guard-3/terminal-source-postimages.json`. All 64 harness
checks pass. Earlier language-focused checks, the original complete 8,892-case suite, and
guard-2's 8,820-case partial suite remain historical evidence for their exact source/harness
identities; none replaces fresh final acceptance. Isolated matrix, integration/editor campaign
completion, independent ordered audit and final accepted storage proof remain outstanding.

The next necessary work is a bounded diagnosis/repair of simultaneous workflow, inventory
and suite coordinator memory, preserving the 512 MiB coordinator/384 MiB headroom guard
and all proof requirements. Cleanup, cap increases and proof reuse across changed source
remain unauthorized. P04.a/v0.114.0 stays the accepted language contract.

Closeout readback: all 17,043 frozen files and 13 Nucleon postimages match; protected
HEAD/branch/stashes remain unchanged. A separate contained diagnostic inventory
reports 95,094,255,616 allocated bytes (88.56 GiB),
90,203,726,437 logical bytes and 248,614,707,200 available bytes.
This is not accepted campaign storage proof. No cleanup occurred. The retained
27-job ledger totals 8.55 contained hours, excluding diagnostic closeout
and unmeasured investigation time. RESULTS.md in the evidence root summarizes the boundary.

# PipeLang Go verification artifact reduction

Bounded v0.111 subset-check construction, 2026-09-13, on saved `js/pipelang`.
The compiler, evaluator, Go backend and accepted language remain unchanged.
This is focused verification-harness evidence, not a full-language campaign.

## Result

Current length-prefixed Value/Trace fixtures replace expanded Go assertions in
the v0.111 subset matrix's native-bundle path. Shared checking support preserves
exact string bytes, ordered traces, nil/empty distinctions and fresh fixture reads.
The ordinary path keeps its literal assertions. Every original generated package
still executes in a fresh contained child; emitted program bytes, debug information,
sealed executable validation, owning subtests and queue limits are preserved.

| Twelve selected owning subtests | Literal control | Compact fixtures |
| --- | ---: | ---: |
| Distinct executable identities | 12 | 12 |
| Executable bytes, including normal debug/support | 59,281,897 | 54,102,262 |
| Subsets / independent oracle vectors per pass | 42 / 10,752 | 42 / 10,752 |
| Generated package executions per pass | 24 | 24 |
| Warm misses | 0 | 0 |

The matched sample saves **5,179,635 executable bytes (8.7373%)**. Every selected
case shrinks: 119,323–996,506 bytes each. Source multisets, subset/vector inventories
and fresh native execution counts match. This measures smaller artifacts, not fewer
identities. The selected subtests already each require one binary under the current
lifetime boundary. The v0.109 layout construction is similarly near that floor and
was left intact; its compiler-resource probes were not pooled or changed.

Selections are 31/159/479/799 plus each 800-partition arm offset, covering all three
arm forms and one/four/eight-subset programs across four tree sizes. These are a
bounded matched sample, not a full-family size estimate or whole-cache savings.

One warm pass summed 7.059 seconds for control units and 6.768 seconds for candidate
units. Fresh child execution itself totaled 0.1257 versus 0.1297 seconds. These
single-sample values do not establish a computation or full-campaign speedup.
Cold unit totals were 33.404 versus 18.927 seconds, including generated linking;
candidate dependency bootstrap and coordinator/storage setup are outside those
unit sums, so they are not an inclusive investigation or cold-campaign comparison.

## Proof and boundaries

The final contained job passed the candidate rebuild, twelve cold and twelve warm
candidate executions, and twelve focused checks. Completed exact control receipts
were independently revalidated without rerunning their unchanged programs. Checks
cover exact fixture encoding (Unicode/NUL/newline, nil/empty traces), malformed
counts/lengths/truncation/trailing bytes, wrong current values and traces on cache
hits, source/support invalidation, fresh process state, sealing, special harness
refusal, corrupt preparation, ordinary fallback, scope layouts and a v0.109 layout.
All 24 selected control executions also passed their per-unit resource checks.

The final aggregate job completed with a 921,059,328-byte peak, no OOM/swap/max
events and its cgroup tree removed. Existing 96-GiB storage, 2-GiB aggregate,
512-MiB coordinator, per-unit/child and compiler-resource ceilings are unchanged.
Three generated-test scratch locations now honor the runner's watched `TMPDIR`.
Production engine/package boundaries and language/compiler sources are unchanged.

Final declared storage: 35,152,264,419 logical file bytes, 37,025,763,328 allocated
file bytes, sampled logical/allocated peaks of 35,180,624,447 / 37,054,779,392 bytes.
This includes the relevant source/toolchain/module inputs, shared Go build cache,
task outputs, controls, rejected candidate, attempts and watched scratch. It is
not a new accounting of unrelated historical executable stores. Roughly 501 MB
of task evidence/build outputs remain retained; **zero existing bytes were freed**.
Peaks are sampled metadata, not exact allocator or physical-device measurements.
The proposed 16/32-GiB targets remain unadopted.

## Evidence

`E=/home/jamie/.codex/visualizations/2026/09/13/01a09949-c588-72e2-a577-1f1b7daf5647/go-artifact-reduction/`

`compact-candidate-receipt.json` owns the matched identities, source inventory and
counts; `compact-checks-receipt.json` owns focused checks/storage;
`compact-final-job.json` owns aggregate containment. `validation.json` records
final source postimages, documentation checks and protected checkout anchors.
`validate.py` retains exact invocation and comparison logic.

The JSON-fixture candidate passed the same semantic sample but saved only 1,911
bytes; it was rejected as ineffective. Earlier wrapper failures (missing standard
bootstrap memory setting, unanchored case selector and nonprivate native cache)
are retained in their original receipts and documented in the
[objective record](../agents/tasks/pipelang-reactive-application-language/go-artifact-reduction.md).
They are excluded from successful measurement claims. No cleanup, full language
campaign, installation, C++ pilot, numerical-budget adoption or Git mutation ran.

Reducing the remaining identity count, broader fixture-family conversion and
historical retention are separate unselected work. No whole-family saving is
extrapolated from this sample.

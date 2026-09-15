# PipeLang foundation contract proposals

Status: design proposal for founder review, based on accepted v0.112.0 on 2026-09-11.
The [inventory](pipelang-foundation.md) distinguishes implemented evidence from requirements;
the [delivery plan](pipelang-foundation-delivery.md) assigns owners, dependencies and proof.
Nothing here changes a language version, parser, runtime, editor or acceptance policy.
All pseudocode below is **illustrative, unaccepted syntax**. An example's predicted result is a
proposed acceptance oracle, not a report that today's compiler executes it.

## Compatibility and common execution contract

Retain the frozen legacy entrypoint and all explicitly selected accepted versions. Add capabilities
only behind an explicit successor contract; do not reinterpret historical Struct/Class spelling,
Record equality, Optional/Result carriers, evaluation order or semantic IDs. Migration diagnostics
must identify the required version and source location. A successor may generalize composition,
but its normalized Core verifier must validate types, ownership, effects and control-flow legality
independently of source lowering. Backends consume Core only. Source, HIR, Core, evaluator, generated
Go, Application IR consumers, diagnostics and editor projections must agree at each accepted slice.

Existing fixed decisions remain: non-null values/references, explicit Optional absence, closed
Result failures, checked arithmetic, explicit lossy conversions, ordinal text identity, deterministic
collection semantics, managed memory, inert compiler/catalog analysis and governed effect bridges.
The proposals below fill gaps; they do not reopen those commitments. A target panic is an
infrastructure failure, not an invented domain error. Compilation cannot obtain hidden host authority.

## C1 — Types, values, objects and dispatch

Use nominal declaration identities across locked modules. Enums are closed named values with
unique stable member tags; never expose declaration position as serialization identity. Reject
unknown serialized tags through a typed decode failure. Tagged unions carry typed payloads and
require exhaustive matching; Optional/Result remain their built-in contracts, without implicit
unwrapping, error conversion or null. Adding a public case requires compatibility review.

Records are immutable structural values. Structs are distinct value types with mutable fields only
through an owner-local mutable slot; assignment copies the value, including nested value fields.
A reference field copies its handle, never deep-clones an object. Equality is derived only when
all fields expose compatible equality; stable hashing excludes mutable keys and hidden identity.
A value containing a mutable reference is not transitively immutable or automatically shareable.
Recursive value layouts without reference indirection are rejected. Preserve existing Record
float/equality behavior; generic collection ordering still needs an explicit total-order capability.

Classes are non-null managed references with identity equality. Constructors must definitely
initialize every required field before publishing `this`; initialization failure returns a typed
failure without exposing a partial object. Private members are visible only to their declaring type;
module/internal visibility and public exports are explicit. No virtual call on an incompletely
initialized object. No user-visible addresses or GC/finalizer behavior.

Recommend one class base, multiple interfaces, explicit abstract/virtual/override, sealed by default
unless inheritance is declared. Overrides retain parameter/return types and cannot widen effects,
strengthen preconditions or weaken postconditions. An abstract class cannot be constructed. Interface
calls dispatch by stable slot identity to the runtime class implementation; field requirements become
validated access contracts, not permission to expose private storage. Struct interface adaptation
must not create implicit mutable aliasing; require a value receiver or explicit copy adapter.
Multiple class inheritance, implicit virtual methods and implicit boxing are proposed exclusions,
subject to founder decision D1.

User generics are invariant initially, with explicit type parameters and capability constraints
(equality, stable hash, order, callable/effect and sharing requirements). Reject ambiguous overloads,
unbound parameters, invalid constraints and unbounded specialization expansion with diagnostics.
Use target-neutral instantiated identities, not Go names. Concrete specialization versus backend
runtime dictionaries is an implementation choice only if dispatch, identity and resource semantics
remain identical. No general reflection, dynamic types or variance is implied.

```text
struct Point { mutable Int X; mutable Int Y; }
a = Point(1, 2); b = a; b.X = 9;   // a.X == 1, b.X == 9
class Counter { mutable Int Count; }
c = new Counter(0); d = c; d.Count = 3; // c.Count == 3; same identity
record Label(String Text);
Label("x") == Label("x")          // true, distinct constructions
interface Area { Int area(); }
abstract class Shape implements Area { abstract Int area(); }
class Square extends Shape { override Int area() => 4; }
Area s = new Square(); s.area()    // 4 through interface dispatch
Map<Counter, Int> m;               // reject: mutable reference has no stable key capability
new Shape();                      // reject: abstract class
class Bad extends Left, Right {}   // reject under proposed single-base rule
match Optional<Int>.None { Some(x) => x } // reject: missing None
identity<T>(T value) => value;
identity<Int>("x")                 // reject: argument type mismatch
```

D1 must settle Struct mutability, single-base inheritance and boxing policy before corresponding
implementation. Nominal enums/immutable values can begin without approving the class extensions.

### Numeric and library surface to review

For F02, propose signed/unsigned 8/16/32/64-bit integers and binary32/64, with add/subtract/
multiply/divide/remainder/negate, comparisons, bitwise Boolean operations and shifts. Shift counts
outside the type width fail; signed right shift is arithmetic, unsigned right shift logical.
Checked arithmetic remains the default; wrapping and saturating functions are explicit. Conversion
coverage is the complete pairwise matrix of these families, with explicit rounding for float-to-int.
Keep NaN unordered for ordinary comparison and require a defined total-order comparer for sorting.
Floating transcendental APIs are not silently promised by basic numeric completion; inventory them
as a separately ratified library extension if needed by the founder's definition of full operations.

Propose the initial exact decimal library as sign plus 96-bit coefficient and scale 0–28, canonical
trailing-zero normalization for equality/hash/serialization, checked add/subtract/multiply/divide/
remainder/compare, explicit quantize rounding, and strict locale-free parse/format. Division requiring
rounding must name its precision/rounding mode; divide-by-zero and precision overflow are typed
failures. This representation and rounding catalog are **D1 numeric-library choices**, not a new
accepted numeric contract. Money remains a separate currency-bearing domain value.

For F03/P11, completion requires immutable Bytes indexing/length/slice/concat; scalar and grapheme
index/slice/iteration; strict UTF-8 encode/decode; explicit normalization/casefold with pinned Unicode
data; ordinal search/split/join/trim; and locale-free numeric parse/format. Invalid boundaries and
encodings return typed failures. Additional codecs must be named and versioned, never delegated to
host defaults. Agree this concrete surface before declaring the text library complete.

## C2 — Functions, modules, callable values and control flow

Locked modules own symbol identity and explicitly import exported symbols; no ambient search,
network resolution during analysis, accidental transitive export or initialization side effects.
Source import syntax must preserve the existing structured ModuleSet identity and cycle rules.
Cross-module calls, overloads and visibility resolve before HIR; callable identity includes complete
parameter/result and effect contracts. Reject overload selection that needs unspecified conversions.

General statement blocks contain lexical immutable or mutable locals, assignments, branches,
returns, loops and loop-local break/continue. Definite assignment is an intersection across joining
paths; mutation cannot leak from one branch into another compilation environment. Conditions are
Bool. Evaluation is left-to-right, eager once for arguments and assignments, lazy for unreached
branches/short circuit. Return exits the whole callable after scoped cleanup. Break/continue target
the nearest loop; labels are not required for foundation completion. Reject unreachable statements,
out-of-scope locals, missing returns and reads before definite assignment.

Loops iterate finite collections/ranges or a condition; they are not restricted to literal-size
programs. Recursion is explicit in the call graph and allowed under declared logical call-depth and
step limits. A step model is versioned at Core operation/back-edge/call boundaries, not backend
machine instructions. Exhaustion is a declared execution outcome, distinct from a domain Result and
from host timeout/OOM. There is no claim of proving general termination. Physical host containment
remains an independent guard and cannot be passed off as deterministic logical exhaustion.

Callable values carry parameter/result/effect and capture metadata. Immutable values capture by
value. Owner-local mutable captures share an explicit closure cell confined to the same owner;
returning a closure is safe when that cell has managed lifetime. A closure cannot capture a scoped
resource/borrow or cross a task boundary with an unprotected mutable cell. Recursive closures obey
the same call-depth limits. No implicit asynchronous launch follows from constructing a lambda.

```text
mutable Int sum = 0;
for Int x in [1, 2, 3] { if x == 2 { continue; } sum = checkedAdd(sum, x)?; }
return sum;                       // 4; '?' handles the declared overflow Result
Int x; if flag { x = 1; } return x; // reject: x not assigned on false path
break;                            // reject: outside loop
while true {}                     // logical fuel exhausted; never accepted as terminating
f = () => localCounter += 1;       // owner-local capture allowed
parallel(() => f());               // reject: mutable capture is not shareable
return () => scopedFile.read();    // reject: scoped resource escapes
```

D2 must settle public spelling of mutable slots, fuel/depth declarations and exhaustion outcomes.
The budget proposal adds semantic work; it never replaces existing compiler/test resource caps.

## C3 — Managed memory, ownership and sharing

Recommend task confinement by default. References may alias within one task ownership region;
a mutable object graph cannot cross task boundaries as an ordinary reference. Immutable values
may be copied; immutable reference graphs may be shared only with transitive immutability proof.
Builders are exclusive scoped mutable values that produce immutable snapshots; a live builder or
mutable iterator cannot escape. Managed retention/reclamation is unobservable, but profile heap
limits and deterministic resource close are explicit. Values crossing public boundaries retain the
same semantics regardless of backend allocation strategy.

Shared mutation requires an explicit `Shared<T>` region associated with a lock, or supported Atomic
cells. Construct a Shared region from a fresh graph with no external aliases; reject promotion of
an aliased object. Access to its ordinary fields is only through a scoped guard borrowing that
region. Guard-derived references cannot escape or survive release. Sharing markers and protection
identities flow through fields, generics, interfaces, calls and closures; a backend mutex around an
untracked pointer is insufficient. Multi-region access needs all matching guards. No claim that
race freedom also eliminates deadlock or schedule dependence.

Typed resource handles use deterministic scoped use/close. Cleanup runs on success, early return,
domain failure, cancellation and logical exhaustion. If both body and cleanup fail, preserve the
body outcome with cleanup failures attached in declared resource-close order; never erase failure.
Cleanup is non-cancellable by the scope's cancellation signal but bounded by its own declared
budget; a stuck/failed host close is reported as infrastructure/cleanup failure. No user finalizers.

```text
owned = new Counter(0);
parallel(() => owned.Count += 1);  // reject: task-confined reference captured
shared = Shared.fresh(Counter(0), lockA);
with lockA.guard(shared) as g { g.Count += 1; } // protected access
shared.Count += 1;                // reject: no matching guard
with lockA.guard(shared) as g { saved = g; } // reject: borrowed reference escapes
snapshot = builder.freeze();     // builder consumed; immutable result may cross tasks
builder.append(2);               // reject: consumed builder
```

D3 is the main architectural choice: recommend statically enforced confinement and guarded regions.
A runtime-checked ownership alternative needs explicit founder selection and revised cost/proof;
ordinary unguarded shared references are not a safe default. General ownership transfer between
tasks is not required by this proposal; queue immutable values or explicit Shared handles instead.

## C4 — Effects, tasks, cancellation and structured parallelism

Entrypoints declare typed inputs/results, required profile and effect set. Pure functions cannot
read shared mutable state or invoke clocks, random sources, files, network, processes or models.
Effects remain resolver/host owned and approval metadata does not itself grant execution authority.
The compiler and catalog emit metadata only. Test/replay bridges supply typed effect outcomes.

A task is owned by its nearest scope, starts only through an explicit spawn operation, and has
exactly one terminal outcome: success, typed failure, cancellation or execution-limit failure.
It cannot outlive its parent scope. Await observes completion without blocking the entire executor.
Captured inputs are validated before spawn; each spawn has a stable logical child ID. Parent scope
exit joins all children and completes cleanup before returning. No detached tasks or raw OS threads.

Pure parallel map is bounded by an explicit fan-out limit, takes transitively immutable inputs,
and returns outputs in input order. It cannot observe scheduling. Effectful/shared tasks may observe
order and require a trace; fan-out limits do not grant extra effect authority. Failure triggers
sibling cancellation, then a join. Aggregate all observed child failures in logical child order,
not completion order; independent cleanup failures remain attached. Do not silently retry effects.

Cancellation is cooperative at declared suspension, loop back-edge, call-budget and effect adapter
checkpoints. Completion versus cancellation uses one recorded terminal transition: completion
already committed wins; cancellation committed first prevents later result publication. Deadline
observation is an injected clock event that requests cancellation; it is not a hard real-time promise.
An irreversible effect may succeed after cancellation was requested: retain its host outcome and
reconciliation identity even when the task outcome is cancelled. Progress is typed observational
data and cannot retroactively change an outcome. Retry requires the operation's declared idempotency
and host authorization; language cancellation never promises rollback of external effects.

```text
scope(maxChildren: 2) {
  a = spawn pure square(3); b = spawn pure square(4);
  return [await a, await b];       // [9,16], after both children and cleanup finish
}
spawn detached sendPayment();     // reject: detached lifecycle; no implicit authority
pure f() => clock.now();          // reject: undeclared clock effect
// child 2 fails before child 1; both failures observed before join ends:
join([child1, child2])             // failures ordered [1,2], trace preserves observations
// cancellation commits before a permit/result delivery:
await task                       // Cancelled; no second terminal success
```

D4 must settle cancellation precedence, aggregate failure representation and cleanup budgets before
an async slice is accepted. The proposed logical terminal point requires explicit Core/runtime tests.

## C5 — Atomics, locks, semaphores and communication

Recommend sequential consistency for all initial atomics: a single total order consistent with
each task's program order, including atomic loads/stores/exchange/CAS/read-modify-write. Support Bool
and fixed-width integers first, with explicit checked overflow outcomes. Compare-and-swap compares
values; no object/address CAS or relaxed memory-order knobs. Publish/read synchronization creates
happens-before edges, but an atomic flag never legalizes unguarded ordinary shared-field access.

Locks are managed, non-reentrant, task-owned objects. Acquisition may suspend while unavailable
only when the caller holds no guard; release on every guard exit publishes preceding protected
writes to the next acquisition. A lock
has a stable ordering key (declared rank plus logical allocation identity for equal ranks). Acquire
nested locks in increasing order through non-suspending try-acquire, which returns WouldBlock
when unavailable. To wait for multiple locks, acquire a declared ordered set before entering the
guard body; the runtime releases provisional acquisitions before suspension. Reject statically
provable inversions and return a typed acquisition error before a dynamic inversion. This prevents lock-order cycles, not arbitrary task/
semaphore dependency deadlock. Suspend/await/spawn/effect calls while holding a guard are rejected,
including through a helper's effect summary. Guard bodies obey finite execution budgets.

Semaphores hold a bounded permit count. An acquisition returns an affine permit token, consumed once
by release or deterministic scope cleanup. Waiting suspends and is cancellable. FIFO is by registered
wait order, excluding cancelled waiters; concurrent registration order is trace-observable. Grant and
cancellation arbitrate at one linearization point. A granted token cancelled before delivery is returned
exactly once. Await while holding a permit is allowed, since async concurrency limiting needs this;
explicit budgets/deadlines and deadlock diagnostics remain necessary. Fairness does not promise a
wall-clock completion bound. Permit transfer outside its owning task scope is rejected initially.

F26 requires a founder disposition. Recommend one bounded typed FIFO queue with immutable values
or explicit safe shared handles, backpressure, close/drain and cancellation. Multiple producers are
ordered by enqueue linearization; close rejects later sends, receivers drain prior items before End.
Do not silently add an unbounded channel or an entire concurrent collection library.

```text
atomic = Atomic<Int>(0);
parallel([() => atomic.fetchAdd(1), () => atomic.fetchAdd(1)]);
atomic.load()                     // 2 after join; each old value is 0 or 1
with lockA { await child; }        // reject: suspension under lock
with lockB(rank:2) { with lockA(rank:1) {} } // reject/order error before waiting
with semaphore.acquire() as permit { await fetch(); } // release also on cancellation
permit.release(); permit.release(); // reject: consumed permit
queue(capacity:1).send(second)     // suspends until capacity; never silently drops first
```

D5 must approve SC-only atomics, lock ordering, permit accounting/FIFO and the bounded queue scope.
More memory orders, reentrant locks or transferable permits are additional design/verification work.

## C6 — Reactive state, actions, contracts and state machines

A state owner exposes immutable snapshots. Computed values form a typed dependency graph; reject
static cycles and report dynamic cycles without publishing partial values. An action runs a pure,
non-suspending validated transition against one snapshot and commits all fields together. Observers
see before or after, never intermediate writes. Recompute affected values in deterministic dependency
order and notify in stable subscription order after commit; nested updates enqueue a later action.
Atomic actions are distinct from Atomic variables and do not create cross-owner transactions.

Preconditions, postconditions, invariants, refinements, `old` and result references are pure typed
expressions. Static discharge records proof; otherwise mandatory runtime guards run at the declared
boundary. Failure rejects construction/transition before publication. Overrides cannot strengthen
requirements or weaken guarantees. Public contract changes need compatibility diagnostics.
State machines use enums/unions, guarded actions and explicit transition metadata; no second DSL.
Bounded path exploration reports depth/state/schedule limits and truncation, never universal proof.

```text
state Balance = 5;
action withdraw(3) requires Balance >= 3 { Balance -= 3; } // commits Balance=2 once
withdraw(4);                       // failure, Balance remains 2; no notification
computed A = B + 1; computed B = A + 1; // reject dependency cycle
transition Closed -> Open;         // reject if declared machine makes Closed terminal
Percentage(-1);                    // refinement failure; no invalid value escapes
```

## C7 — Determinism, testing, replay and debugging

Pure value computation remains deterministic for pinned inputs, language/library/profile versions
and logical budgets. Race-safe shared programs can still produce different legal results. Separate
pure determinism, ordered effect replay, and schedule replay in metadata and documentation.
Record task IDs, spawn/join/suspend/resume, synchronization linearizations, terminal races, effect
request/result identities and state commits. Ordinary local operations between synchronization
points need not be logged if ownership enforcement makes them unobservable to other tasks.
Replay consumes recorded effects without issuing live work and enforces recorded choices; missing,
extra, reordered or identity-mismatched events fail. Redacted sensitive payloads stay references;
missing authorized replay material yields an explicit incomplete replay, not guessed results.

Tests are normal typed declarations with stable IDs: unit/table/golden/negative/property/contract/
state-path and schedule tests. Generators and shrinkers preserve constraints and record seeds;
concurrency shrinking retains required ordering edges. Explore finite schedules against an independent
small-step model, then compare evaluator and backend. Record explored schedules, limits and uncovered
space; finite testing is not a proof for arbitrary programs. Repeatability alone does not prove race safety.

Semantic graphs and source maps cover each new node through HIR/Core and target output. LSP and
DAP consume compiler-owned identities and typed values for navigation, stepping, stacks, watches and
sanitized failure bundles. Task suspension stacks and guard/permit ownership must be inspectable.
Target adapter machinery stays separate from semantic metadata; source debugging for the Go seed
is required at the app-start milestone, not postponed to a hypothetical second backend.

```text
parallel([() => atomic.store(1), () => atomic.store(2)]); atomic.load();
// legal result 1 or 2; an exact replay must reproduce the recorded ordering and result
replay(traceWithMissingEffect);    // reject as incomplete; never call the live provider
pure hash("é");                   // same versioned hash on every conforming backend
```

## C8 — Profiles and resource contracts

A program declares requirements; the selected profile/backend manifest admits or rejects the entire
closure before code generation. `full` supports the reviewed foundation. `constrained` uses the same
semantics with declared heap, task, recursion, code-size and library limits. `mcu` additionally requires
an explicit bounded memory plan; allocation after initialization may be forbidden. Unsupported
capabilities reject compilation, never degrade SC atomics, Unicode, numeric behavior or cleanup.
Logical limits are distinct from compiler acceptance limits and host test containment.

Foundation completion requires executable profile admission and refusal, full Go conformance,
constrained/MCU capability and memory-plan validation plus a contained reference execution for
admitted profile programs. This does not certify an MCU board, hard real-time scheduler, native
allocator, Qt runtime or a non-Go backend. Those physical target implementations remain explicitly
separate resolver work. D6 must ratify this milestone interpretation; requiring physical MCU proof
before apps expands the target program and needs its own estimate.

```text
profile mcu(noAllocationAfterInit, maxTasks:0); spawn work(); // reject before generation
profile constrained(maxDepth:8); recurse(depth:9);          // declared limit outcome
backendWithoutSC + Atomic<Int>;                             // reject capability mismatch
```

## Founder decisions and proposed exclusions

D1–D6 are review decisions, not accepted semantics. Approve only decisions needed by the selected
implementation slice; later decisions remain pending while independent work proceeds. All required
F01–F34 rows retain owners in the delivery ledger. Proposed exclusions are multiple class inheritance,
implicit boxing/variance, detached tasks, unguarded sharing, relaxed atomics, raw memory/thread APIs,
general ownership/permit transfer and unbounded concurrent queues. Only raw unsafe memory/hidden
authority exclusions are inherited hard limits; the other scope choices require founder ratification.
No requirement may be declared complete solely because this proposal recommends a smaller surface.

## Approved Qt boundary continuation

The [Qt integration boundary](pipelang-qt-boundary.md) applies the completed pilot
to owned conversion, typed outcome presentation and target-side lifetime/delivery.
This is bounded adapter proof, not completion of P15/P16/P17/P20 or ratification of
D1-D6. The approved continuation also repairs the recorded zero-argument record
transport panic within existing contracts, advancing F01/F05/F14 correctness
without selecting another P-number. Go v0.113.0 remains the accepted backend.


## Approved P04.a control-flow contract

The founder selected and explicitly approved lexical blocks, branch joins and early
returns. [The concrete v0.114.0 contract](pipelang.md#pipelang-v01140-general-lexical-blocks)
owns the bounded C2 rules and pending acceptance. Mutable slots and logical fuel/depth
remain later decisions; this approval does not ratify D1–D6 wholesale.

# Completed bounded function slice

## Step 8a — named pure record predicate filtering (`v0.31.0`)

The founder accepted one first bounded Step-8 function seam. Production source admits a public
same-class `bool PredicateName(R item, P1, ...)` and exact direct
`filter(List<R>, PredicateName, P1, ...) -> List<R>`. `R` is one existing public primitive record;
trailing parameters are primitive and match the direct filter operands exactly.

Predicate bodies are pure and closed over only literals, primitive predicate parameters, one-hop
public primitive fields of `item`, logical not/and/or, equality and ordering comparisons, and the
accepted `contains_casefolded` and `trim` expressions. Lambdas, closures, function values,
overloads, arbitrary calls, class state, construction, effects, async, Optional/Result predicates,
matching, propagation, Application IR, and runtime behavior remain excluded.

Typed HIR and target-neutral Core use `list_filter_predicate`, carrying the predicate method's
existing semantic identity and ordered operands. Core program validation resolves and checks that
identity. The target-neutral evaluator and Core-only Go backend validate the full input and
primitive arguments before stable one-call-per-row iteration, require bool, fail atomically, and
return canonical non-nil fresh copied output. `pipelang.compiler.v1` and
`pipelang.semantic.v1` remain unchanged. `v0.1.0` through `v0.30.0` reject the spelling without
implicit migration.

## Step 8b — same-class pure calls (`v0.36.0`)

Production source admits `Method(expression, ...)` only inside public expression-bodied methods.
The target is one uniquely named public method on the same class, resolved during semantic
analysis; ordered argument types and the return type must match the target signature exactly.
Call participants may reference parameters and match-arm bindings, not class-owned state. Calls may
nest. Direct recursion and indirect cycles fail deterministically as `PL3033`.

Typed HIR and target-neutral Core carry an explicit `call` node containing the target's existing
callable semantic identity, source name, and ordered typed operands. Lowering includes the complete
transitive dependency closure once. Core validation independently proves target presence,
same-owner identity, exact signature, and an acyclic graph. The target-neutral evaluator validates
and copies arguments into an isolated call frame. The Core-only Go backend emits ordinary calls
only after that proof; it performs no semantic lookup or inference.

The Docker observability consumer proves
`OrderContainers(FilterContainers(rows, query))` as a source-to-HIR-to-Core composition while
`dockpipe.application.v1` continues to bind its existing filter and order identities separately.
`pipelang.compiler.v1`, `pipelang.semantic.v1`, and `dockpipe.application.v1` do not change shape;
only the language-contract value advances. The exact 45-source legacy inventory remains frozen.
Cross-class/module calls, private callers or targets, overloads, generics, function values, lambdas,
recursion, blocks, locals, branches, effects, entrypoints, and target-specific behavior remain
excluded.

## Step 8c — general pure-call composition (`v0.37.0`)

Production source may use the already resolved same-class pure call node throughout expressions
that the language already admits as eager and pure, including match-arm bodies, record and Optional
construction, text operations, and arithmetic or comparison operands. Match carriers and
propagation operands remain direct references. The v0.36.0 same-class ownership, public visibility,
exact signature, participant closure, and acyclic call-graph rules remain exact.

Typed HIR and target-neutral Core reuse the existing call node and dependency closure. Core
recursively validates nested placement, target identity, signature, ownership, and cycles; the
evaluator keeps isolated copied frames, and the Go backend consumes Core only. The Docker
observability consumer proves an `Optional<ContainerRow>` match whose `some` arm calls a text
normalization helper. `pipelang.compiler.v1`, `pipelang.semantic.v1`, and
`dockpipe.application.v1` keep their schemas and advance only language-contract metadata. The
45-source legacy inventory remains exact. Cross-class/module calls, blocks, locals, branches,
loops, effects, recursion, and target-specific behavior remain excluded.

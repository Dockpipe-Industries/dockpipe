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

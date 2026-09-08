PipeLang `v0.88.0` adds `pipe-nested-return`: a complete straight-line block return may
nest ternaries in either or both arms through depth two, after zero or more typed locals.
Initializers remain nonnested. Nested choices in statement trees, expression-bodied methods,
conditions and arguments remain excluded. Only selected conditions and arms execute.

PipeLang `v0.87.0` adds `pipe-terminal-leaf-return`: complete nonnested ternary returns
in leaves of terminal if/else trees through depth three, with optional typed conditional
locals in each scope. Selected branches and return arms stay lazy; reached initializers
execute eagerly once in source order. Nested ternaries and new argument/condition placements
remain excluded.

PipeLang `v0.86.0` adds `pipe-conditional-return`: straight-line typed conditional locals
followed by a complete ternary return. Locals execute eagerly once in source order; return arms
are lazy and exactly match the method return type. Nested ternaries, new argument/condition
placements and terminal-tree return choices remain excluded. Existing snippets retain their contracts.

# DockPipe Language Support (VS Code)

PipeLang `v0.85.0` adds `pipe-straight-conditional-locals`: finite complete ternary
initializers in typed immutable-local sequences followed by an ordinary return.
Later choices may depend on earlier locals; initializers run once in order, including
unused bindings, and arms remain lazy. Nested ternaries and new return/condition/argument
placements or match/propagation combinations remain excluded. Earlier versions retain their limits.

PipeLang `v0.84.0` adds `pipe-finite-conditional-locals`: any finite number of lazy ternaries,
each the complete initializer of a typed immutable local in a terminal tree through depth three.
Later choices can depend on earlier in-scope bindings. Exact types, complete carriers, and
ordered/unused initializers are preserved. Nested ternaries, new placements, straight-line
multiple choices, deeper trees, and new matching/propagation combinations remain excluded.
Earlier versions retain their limits.

PipeLang `v0.83.0` adds `pipe-two-conditional-locals`: at most two lazy ternaries per method,
each a complete typed immutable-local initializer within terminal trees through depth three.
A later choice can depend on the first in lexical scope; exclusive branches still count toward
the method-wide limit. A third/nested ternary, new placements, deeper trees, and new matching or
propagation combinations remain excluded. Earlier versioned forms retain their existing rules.

PipeLang `v0.82.0` adds `pipe-conditional-local-tree`: one lazy ternary as the complete
initializer of one typed immutable local anywhere in a terminal tree through depth three.
The occurrence limit is per method, including mutually exclusive branches. Both arms have the
exact local type; only the selected arm runs. No new return/condition/argument placements,
nested or multiple ternaries, matching, propagation, or deeper terminal trees are admitted.
Earlier versioned forms remain unchanged.

PipeLang `v0.81.0` adds `pipe-terminal-tree`: symmetric or asymmetric terminal `if/else`
trees through depth three, with optional ordered typed immutable locals in each scope.
At most seven decisions yield eight return paths; only the selected path executes.
Depth four, fallthrough, assignment, loops, effects, inference, and new combinations with
conditional expressions, matching, or propagation remain excluded. Earlier forms are unchanged.

PipeLang `v0.80.0` adds `pipe-two-expanded-terminal-if`: exactly two of the four leaves
on the symmetric depth-two base expand into terminal `if/else` decisions. All six leaf pairs
are supported, with optional root locals and finite typed local sequences in each lexical scope.
Five conditionals give six return paths, with lazy selected-path execution. A third expansion,
depth four, asymmetric bases, conditional expressions within the topology, propagation, matching,
assignment, fallthrough, other early returns, loops, effects, and inference remain excluded.
Earlier versioned forms are unchanged.

PipeLang `v0.79.0` adds bounded depth-three terminal branching through
`pipe-bounded-depth-three-terminal-if`. An inherited rootful or rootless symmetric depth-two
topology may expand exactly one of its four terminal leaves into one additional terminal
`if/else`; the expanded path and both new leaves retain typed immutable-local sequences and
lexical scope. The other three depth-two leaves remain terminal, giving exactly five return paths.
Two expanded leaves, depth four, non-symmetric bases, conditional expressions within the topology,
propagation, matching, assignment, fallthrough, effects, and inference remain excluded.

PipeLang `v0.78.0` adds rootless symmetric depth-two terminal branching through
`pipe-rootless-symmetric-nested-terminal-if`. Both outer branches end in one inner terminal
`if/else`; typed branch and leaf local sequences retain lexical scope and selected-path execution.
No root local is required. All v0.77 forms remain supported. Additional nesting, conditional
expressions within the topology, propagation, matching, assignment, fallthrough, effects, and
inference remain excluded.

PipeLang `v0.77.0` permits one or more source-ordered explicitly typed immutable root locals before
an outer terminal `if/else` whose two branches each end in exactly one inner terminal `if/else`
through `pipe-symmetric-nested-terminal-if`. Root locals evaluate eagerly once and remain visible
throughout; outer-branch and inner-leaf bindings retain their independent lexical scopes. Only the
selected outer branch, its inner condition, and its selected leaf evaluate. The symmetric topology
without a root local, third-level nesting, conditional expressions within the topology,
propagation, matching, assignment, fallthrough, effects, inference, and deployment remain excluded.

PipeLang `v0.76.0` permits one or more source-ordered explicitly typed immutable locals before the
exact v0.75 bounded nested terminal topology through `pipe-root-local-nested-terminal-if`. Root
locals enter scope after their initializer, evaluate eagerly once before the outer condition, and
remain visible to both outer branches and every descendant branch. The inherited rootless v0.75
form remains valid. Nesting in both outer branches, third-level nesting, conditional expressions
within the topology, propagation, matching, assignment, fallthrough, effects, inference, and
deployment remain excluded.

PipeLang `v0.75.0` adds finite source-ordered sequences of explicitly typed immutable locals to
either inner terminal leaf of the exact v0.74 nested topology through
`pipe-nested-terminal-if-inner-locals`. Each inner-leaf local enters scope after its initializer;
later locals in the same leaf may reference it, but bindings cannot cross into the sibling leaf or
escape the inner decision. The outer-branch local sequence remains visible to the inner condition
and both leaves, and only the selected outer branch, inner branch, and local sequence evaluate.
Top-level locals for the v0.75 topology, nesting in both outer branches, third-level nesting,
conditional expressions within the topology, propagation, matching, assignment, fallthrough,
effects, inference, and deployment remain excluded.

PipeLang `v0.74.0` adds one bounded nested terminal decision through `pipe-nested-terminal-if`.
Exactly one outer branch may end in one inner terminal `if/else` after any finite sequence of
explicitly typed immutable locals; the sibling outer branch retains the v0.73 branch form and the
inner leaves return directly. Both conditions are `bool`, all leaves have the declared return type,
outer-branch locals remain visible to the inner condition and leaves, and only selected branches
evaluate. Inner locals, nesting in both outer branches, third-level nesting, propagation, matching,
conditional expressions within this topology, assignment, fallthrough, effects, inference, and
deployment remain excluded.

PipeLang `v0.73.0` removes the top-level-local prerequisite from terminal `if/else` methods via
`pipe-direct-terminal-if`. The terminal conditional may now be the complete method body while each
branch retains the existing finite ordered immutable-local sequence. Nested branches, propagation,
matching, assignment, fallthrough, other early returns, effects, inference, and deployment remain
excluded.

PipeLang `v0.72.0` generalizes terminal-branch locals through
`pipe-terminal-if-branch-local-sequence`. Either branch may contain any finite source-ordered
sequence of explicitly typed immutable locals before its return. Nested branches, zero top-level
locals, propagation, matching, assignment, fallthrough, effects, inference, and deployment remain
excluded.

PipeLang `v0.71.0` adds a second ordered lexical immutable local per terminal branch through
`pipe-terminal-if-branch-locals`. The second local may reference the first in its branch; a third
branch local, nested branches, zero top-level locals, propagation, matching, assignment,
fallthrough, effects, inference, and deployment remain excluded.

Language support for DockPipe authoring:

- `.pipe` PipeLang syntax highlighting
- PipeLang snippets and keyword completion
- PipeLang diagnostics from the compiler's strict UTF-8, file-aware structured diagnostic contract
- PipeLang model awareness for primitive, object/interface, and `List<T>` field types
- DockPipe `config.yml` IntelliSense for common workflow keys, including `cwd` and `scopes` value suggestions (`repo`, `source`, `artifacts`)
- DorkPipe agent path snippets for `scope:artifacts:...`, `scope:workflow:<name>:...`, and `scope:package:<name>:...` references
- DockPipe `config.yml` support for optional authored `view:` metadata (entry routing, pages, sections, and field-path driven launcher layouts)
- Up-to-date workflow help for packaged workflow steps (`workflow:` + `package:`), Compose host built-ins, and authored security/runtime policy blocks
- DockPipe `package.yml` hover/docs and top-level key completion
- DockPipe `package.yml` `icon` / `artwork` metadata hints for package-owned launcher/tooling assets
- DockPipe `package.yml` image metadata hints for package-owned OCI image refs
- DockPipe `package.yml` support for `script_contract.inject` with valid generic injectable suggestions
- DockPipe `dockpipe.config.json` hover/docs and section-key completion
- First-party package script IntelliSense for workflow cwd, `dockpipe scope`, and focused DockPipe SDK helpers in shell, PowerShell, Python, and Go
- Runtime path env suggestions for scripts: `DOCKPIPE_SOURCE_ROOT`, `DOCKPIPE_ARTIFACT_ROOT`, `DOCKPIPE_OUTPUT_ROOT`, and `DOCKPIPE_STEP_CWD`
- Structure-aware YAML semantic coloring for workflow keys, step keys, `vars:` fields, and `types:` entries
- YAML parse diagnostics for DockPipe workflow files (`config.yml` / `config.yaml`)
- Hover/docs for top-level workflow keys, step keys, `types:` entries, and `vars:` fields from PipeLang XML summaries (`types:` entrypoint)
- `vars:` value suggestions from implementing class defaults and nearby `Struct` known-values
- Completion/hover for SDK-object patterns:
  - shell:
    - cwd/source: prefer `pwd` under explicit workflow `cwd`; use `dockpipe scope source` when a script must resolve the source checkout from another cwd
    - getters: `dockpipe get workflow_name`, `dockpipe get script_dir`, `dockpipe get package_root`, `dockpipe get assets_dir`, `dockpipe get dockpipe_bin`
    - scopes: `dockpipe scope`, `dockpipe scope artifacts <path>`, `dockpipe scope source <path>`, `dockpipe scope workflow <name> <path>`, `dockpipe scope --package <name>`, `dockpipe scope resolver <name> auth-dir`
    - shell-only actions: `eval "$(dockpipe sdk)"` then `dockpipe_sdk init-script`, `dockpipe_sdk require dockpipe-bin`, `dockpipe_sdk require workflow-name`, `dockpipe_sdk source terraform-pipeline`, `dockpipe_sdk die`
  - PowerShell: `$dockpipe.Workdir`, `$dockpipe.DockpipeBin`, `$dockpipe.WorkflowName`, `$dockpipe.ScriptDir`, `$dockpipe.PackageRoot`, `$dockpipe.AssetsDir`, `Invoke-DockpipeScope`
  - Python: `dockpipe.workdir`, `dockpipe.dockpipe_bin`, `dockpipe.workflow_name`, `dockpipe.script_dir`, `dockpipe.package_root`, `dockpipe.assets_dir`, `dockpipe.scope(...)`
  - Go: `dockpipe.Workdir`, `dockpipe.DockpipeBin`, `dockpipe.WorkflowName`, `dockpipe.ScriptDir`, `dockpipe.PackageRoot`, `dockpipe.AssetsDir`, `dockpipe.WorkflowScope()`, `dockpipe.PackageScope(...)`

## Install (dev)

```bash
make package-dockpipe-language-support
```

This writes a VSIX to:
`bin/.dockpipe/extensions/dockpipe-language-support-<version>.vsix`

Install the generated `.vsix` from Cursor/VS Code:
`Extensions` -> `...` -> `Install from VSIX...`

Or install via CLI:

```bash
make install-dockpipe-language-support
```

## Notes

- YAML IntelliSense is context-aware and uses lightweight nesting analysis from the workflow document.
- Workflow authoring help tracks the current public model: steps + runtime + resolver first, with top-level runtime/resolver as defaults, step-level runtime/resolver as overrides, step-level `security` supported for container-only policy tightening, `isolate` treated as the advanced low-level override, top-level `run` / `act` treated as single-flow shorthand only, and async authoring expressed through explicit `group: { mode: async, tasks: [...] }`.
- When present, workflow `view:` stays a declarative launcher presentation layer over the typed model rather than replacing `vars:` / env mappings.
- `types:` suggestions support the interface entrypoint pattern, for example:
  `models/IR2InfraConfig`
- PipeLang editor support understands interface/object field types and generic list shapes such as `List<string>` and `List<IImageResource>`. It also highlights and completes the explicitly versioned arithmetic Result spellings `Result<int, ArithmeticError>` and `Result<float, ArithmeticError>`; `v0.2.0` through `v0.7.0` add the frozen direct checked arithmetic and identical Result parameter/return contracts, and `v0.8.0` adds the exact two-parameter ordinal `string` ordering method shape. `v0.9.0` preserves those contracts and adds public, nonempty primitive immutable records plus exact one-parameter identity transport through a class method. `v0.10.0` adds only exact one-hop read-only `parameter.Field` projection through the `pipe-record-field` snippet. `v0.11.0` adds exact declaration-ordered `new Row { Id = id, ... }` primitive-record construction through `pipe-record-new`. `v0.12.0` adds only direct structural `left == right` or `left != right` comparison of two identical primitive-record parameters through `pipe-record-equality`. `v0.13.0` adds only primitive `Optional<T>` for `string`, `int`, `float`, and `bool`, with exact direct `some(value)`, `none<T>()`, identity transport, and `has_value(value)` methods through `pipe-optional`. `v0.14.0` adds only exact two-parameter primitive defaulting as `value_or(Optional<T>, T) -> T` through `pipe-optional-value-or`. `v0.15.0` adds only immutable record-list values for one existing public primitive record `R`, using exact `empty_list<R>()`, `list(value)`, and direct `List<R>` identity transport through `pipe-record-list`. `v0.16.0` adds only exact direct `count(List<R>) -> int` cardinality through `pipe-record-list-count`. `v0.17.0` adds only exact immutable `append(List<R>, R) -> List<R>` through `pipe-record-list-append`, with complete input validation and copied result storage. `v0.18.0` adds only exact `Optional<R>` construction, identity transport, presence inspection, and bounded `value_or` defaulting for one existing public primitive record `R` through `pipe-record-optional`, with canonical validation and copied record storage. `v0.19.0` adds only exact `Result<List<R>, string>` success/failure construction, identity transport, `is_ok` inspection, and bounded `success_or`/`failure_or` defaulting through `pipe-snapshot-result`, with canonical validation and copied list/record storage. `v0.20.0` adds only exact zero-based `at(List<R>, int) -> Optional<R>` through `pipe-record-list-at`, validating the complete list before returning copied `some` storage or canonical `none`. `v0.21.0` adds only exact stable-key `find_by(List<R>, R.Field, string) -> Optional<R>` through `pipe-record-list-find-by-text`, where `R.Field` names one public string field, the complete list and key are validated, and the first ordinal-equal match returns copied `some` storage or canonical `none`. `v0.22.0` adds only exact `filter_by(List<R>, R.Field, string) -> List<R>` through `pipe-record-list-filter-by-text`, preserving every ordinal-equal match in input order after complete list and key validation and returning fresh copied storage. Primitive/nested/optional/result list elements beyond the exact snapshot envelope, list fields, literals, slicing, iteration, predicate or multi-field filtering, sorting, equality, hashing, maps, sets, builders, mutation, composite keys, case-folding, normalization, Optional extraction beyond `value_or`, general Result composition, propagation, matching, and implicit migration are not suggested.
- `v0.23.0` adds only exact `contains_casefolded(string, string) -> bool` through `pipe-text-contains-casefolded`, using pinned Unicode 17.0.0 full-default folding with strict UTF-8 validation and no normalization or locale tailoring. The preceding case-folding exclusion continues to apply to list filtering and every other unaccepted operation.
- `v0.24.0` adds only exact `filter_contains_casefolded(List<R>, R.Field, string) -> List<R>` through `pipe-record-list-filter-contains-casefolded`, reusing the pinned Unicode 17.0.0 full-default containment rule for one selected public string field while preserving stable input order, canonical empty output, complete validation, and copied result storage. Trimming, joined or multi-field search, normalization, locale tailoring, predicates, sorting, and general iteration remain unaccepted.
- `v0.25.0` adds only exact `Result<string, string>` construction, identity transport, `is_ok` inspection, and bounded `success_or`/`failure_or` defaulting through `pipe-text-result`, with strict UTF-8 validation of tagged payloads and both selected and unselected fallbacks. Other Result argument types, unwrap, propagation, mapping, matching, effects, and composition remain unaccepted.
- `v0.26.0` adds only exact direct `trim(string) -> string` through `pipe-text-trim`, removing maximal leading and trailing Unicode 17.0.0 `White_Space` after strict UTF-8 validation while preserving interior scalars exactly. Normalization, case folding, locale tailoring, grapheme segmentation, collapse, replacement, composition, field/list trimming, and implicit migration remain unaccepted.
- `v0.27.0` adds only exact direct `filter_joined_contains_casefolded(List<R>, R.Name, R.State, R.Image, R.Ports, R.Created, string) -> List<R>` through `pipe-record-list-filter-joined-contains-casefolded`. It requires exactly five distinct public string selectors, joins their values in source order with one U+0020 SPACE, trims the query with the pinned Unicode 17.0.0 `White_Space` rule, and applies the pinned full-default case-folded containment rule with complete validation, stable order, and copied results. Arbitrary selector counts, field-selector values, predicates, regex, normalization, locale tailoring, sorting, composition, and implicit migration remain unaccepted.
- `v0.28.0` adds only exact direct `sort_by_ordinal(List<R>, R.Field) -> List<R>` through `pipe-record-list-sort-by-ordinal`. The selector must identify one public string field of the same primitive record. Sorting is stable and ascending by the existing ordinal Unicode scalar-sequence order after complete list/record/field validation, with fresh copied non-nil results. Descending or multi-key sorting, comparers, normalization, case folding, locale tailoring, mutation, composition, and implicit migration remain unaccepted.
- `v0.29.0` widens only exact direct `filter_joined_contains_casefolded(List<R>, R.Field1, R.Field2, ..., string) -> List<R>` through `pipe-record-list-filter-joined-contains-casefolded`. It requires two or more distinct public string selectors of the same primitive record, bounded by that record's fields, while preserving source-order U+0020 joining, trimmed-query Unicode 17.0.0 case-folded containment, complete validation, stable order, canonical empty output, and copied results. Dynamic selectors, zero/one selector, predicates, sorting, composition, and implicit migration remain unaccepted.
- `v0.30.0` widens only exact direct `sort_by_ordinal(List<R>, R.Field1, R.Field2, ...) -> List<R>` through `pipe-record-list-sort-by-ordinals`. It requires two or more distinct public string selectors of the same primitive record and applies stable ascending lexicographic ordinal Unicode scalar-sequence comparison in source selector order after complete validation, with canonical non-nil empty and copied results. One-selector source retains the exact v0.28.0 behavior and projection. Descending or per-key direction, dynamic selectors, comparers, normalization, case folding, locale tailoring, mutation, composition, and implicit migration remain unaccepted.
- `v0.31.0` adds exact `filter(List<R>, PredicateName, P1, ...) -> List<R>` through `pipe-record-list-filter-predicate`. The same public class must declare a public `bool PredicateName(R row, P1, ...)`; trailing parameters are primitive, filter operands are direct parameters, and the predicate body is a bounded pure composition of literals, primitive parameters, one-hop public primitive record fields, logical/comparison operators, `contains_casefolded`, and `trim`. Evaluation validates all inputs before stable source-order iteration and returns a fresh copied non-nil list. Lambdas, closures, function values, overloads, effects, Optional/Result predicates, arbitrary calls, and implicit migration remain unaccepted.
- PipeLang diagnostics call `dockpipe pipelang check --stdin --format json` without a shell, so unsaved buffers are checked without source or generated-state writes. The extension prefers `DOCKPIPE_BIN`, then a workspace-local `src/bin/dockpipe`, then `dockpipe` from `PATH`.
- Shared script support points authors at the canonical DockPipe SDK under `src/core/assets/scripts/lib/` and `dockpipe sdk`.
- Workflow scripts can use `dockpipe scope` / SDK scope helpers for checkout, workflow artifacts, and durable owner-only package state. Package caches, build output, scratch, and run evidence use `PackageRuntimeDir` or shell SDK `path package-runtime`; runtime env such as `DOCKPIPE_SOURCE_ROOT`, `DOCKPIPE_STEP_CWD`, `DOCKPIPE_OUTPUT_ROOT`, and `DOCKPIPE_ARTIFACT_ROOT` remains available for low-level integrations.
- DorkPipe agent workflow path lists can use `scope:...` references; the orchestration planner resolves them through `dockpipe scope` before writing prompts and task JSON.
- `package.yml` may declare package-owned artwork via `icon:` and `artwork:` paths relative to the manifest.
- `package.yml` may also declare a package-owned OCI image reference via `image:`; DockPipe compiles that into the effective runtime/image artifact manifests.
- `package.yml` `script_contract.inject` declares the generic injected fields. In shell, the public
  way to read those values is `dockpipe get ...`; the backing runtime env vars are
  `DOCKPIPE_WORKDIR`, `DOCKPIPE_WORKFLOW_NAME`, `DOCKPIPE_SCRIPT_DIR`,
  `DOCKPIPE_PACKAGE_ROOT`, and `DOCKPIPE_ASSETS_DIR`. Workflow step cwd/scope support also injects
  `DOCKPIPE_SOURCE_ROOT`, `DOCKPIPE_ARTIFACT_ROOT`, `DOCKPIPE_OUTPUT_ROOT`, and `DOCKPIPE_STEP_CWD`.

- `v0.32.0` adds explicit selector/direction pairs: `sort_by_ordinal(values, Row.State, descending, Row.Name, ascending)`. Directions are contextual, sorting remains stable ordinal and fully validated, and earlier ascending forms remain exact.

- `v0.33.0` adds only exact safe postfix `values[index] -> Optional<R>` for a primitive-record list and signed `int` in the direct two-parameter method shape, through `pipe-record-list-index`. Negative and out-of-bounds indices return `none`; `at(values, index)` remains compatible.

- `v0.34.0` adds contextual `propagate(carrier)` only inside the exact bounded `some(propagate(carrier))` or bounded Result `ok(...propagate(carrier))` method shapes, through `pipe-propagate`.

- `v0.35.0` adds exhaustive bounded `match(value){ some(item) => item, none => fallback }` and `ok`/`err` arms through `pipe-match-optional`.

- `v0.36.0` adds public same-class pure `Method(expression, ...)` calls with exact ordered signatures, parameter/arm-local closure, and an acyclic resolved call graph through `pipe-pure-call`. Class-owned state, cross-class/module calls, private targets, overloads, generics, lambdas, function values, recursion, blocks, locals, branches, effects, and entrypoints remain excluded.

- `v0.37.0` permits those resolved same-class pure calls throughout already admitted eager pure expressions and match-arm bodies through `pipe-pure-call-compose`. Match and propagation carriers stay direct, and all v0.36.0 identity, signature, closure, and acyclicity rules remain exact.

- `v0.38.0` adds one exactly typed lazy `condition ? whenTrue : whenFalse` expression per method through `pipe-conditional`. Both branches are checked, only the selected branch executes, and nested conditional, match, and propagation operands remain excluded.

- `v0.39.0` adds one explicitly typed immutable local plus one terminal return through `pipe-immutable-local`. Initialization is eager and occurs once; an explicit checked-arithmetic `Result` local supplies arithmetic result context. Inference, shadowing, reassignment, multiple locals, contextual propagation, early returns, statement branches, and loops remain excluded.

- `v0.40.0` adds source-ordered explicitly typed immutable locals through `pipe-immutable-locals`. Initializers run eagerly once in order; each local enters scope after its initializer, and later initializers may use earlier locals. Inference, reassignment, shadowing, contextual propagation, early returns, statement branches, and loops remain excluded.

- `v0.41.0` adds one block-scoped bounded propagation through `pipe-block-propagate`. The first explicitly typed immutable local unwraps the method's sole direct Optional or bounded Result carrier parameter; absence or failure returns that identical carrier before later locals and the terminal return. Additional propagation, computed carriers, arbitrary Results, inference, reassignment, statement branches, loops, effects, and runtime behavior remain excluded.

- `v0.42.0` adds one prior-local helper propagation through `pipe-prior-local-propagate`. The first local calls one public same-class pure helper over the method's sole direct parameter; the second local propagates that immediately preceding bounded carrier. Helper evaluation occurs once, and absence/failure returns its identical canonical carrier. Direct call-inside-propagate, extra or computed arguments, other prior locals, multiple propagation, arbitrary Results, inference, statements, effects, and runtime behavior remain excluded.

- `v0.43.0` adds one top-level helper-result match through `pipe-helper-result-match`. A one-parameter public pure method matches one same-class public pure `Result<string, string>` helper called with that direct parameter. The helper evaluates once; exact source-ordered `ok(binding)` then `err(binding)` arms copy the selected payload and evaluate only the selected expression. Other carriers, extra or computed arguments, extra caller parameters, nested matches, reversed or wildcard arms, guards, new blocks/locals, propagation changes, effects, and runtime behavior remain excluded.

- `v0.44.0` adds one general bounded helper-carrier match through `pipe-helper-carrier-match`. A public pure caller passes every parameter directly once in declaration order to one exact-signature public pure same-class helper returning admitted `Optional<T>`, `Result<List<R>, string>`, or `Result<string, string>`. Optional arms are source-ordered `some(binding)` then binding-free `none`; Result arms are `ok(binding)` then `err(binding)`. Arithmetic Results, computed/reordered/omitted/extra arguments, cross-owner calls, overloads, generics, nested or multiple matches, wildcard or reversed arms, guards, propagation changes, new statements/blocks/locals, effects, and runtime behavior remain excluded.

- `v0.45.0` adds one first-local helper-carrier match through `pipe-helper-carrier-match-local`. The first explicitly typed immutable local is initialized by one exact v0.44-compatible `match(Helper(...))`; the copied selected arm result initializes that local once, then existing ordered locals and the terminal return may consume it. Match in later locals or the terminal return, additional or nested matches, computed/reordered/omitted/extra helper arguments, new carrier forms, propagation changes, statements, effects, and runtime behavior remain excluded.

- `v0.46.0` adds checked-arithmetic first-local helper matching through `pipe-checked-arithmetic-helper-match-local`. The exact v0.45 placement and direct-argument rules additionally accept existing `Result<int, ArithmeticError>` and `Result<float, ArithmeticError>` helpers with ordered `ok(binding)` then `err(binding)` arms. Top-level/later/nested matches, computed arguments, arithmetic propagation or new Result construction, statements, effects, and runtime behavior remain excluded.

- `v0.47.0` adds later-local helper-carrier matching through `pipe-later-local-helper-match`. Exactly one existing helper match may initialize any explicitly typed immutable local after zero or more ordinary locals. Earlier locals, the helper, and the matched local each evaluate once in source order; later locals and the terminal return continue. The v0.46 carrier matrix and direct-argument rules remain closed. Terminal-return, argument, nested, multiple, and top-level checked-arithmetic matches; computed arguments; Result widening; statements; effects; and runtime behavior remain excluded.

- `v0.48.0` adds prior-local carrier matching through `pipe-prior-local-carrier-match`. After zero or more ordinary locals, one explicitly typed carrier local calls the exact public pure same-class helper with every caller parameter directly once in declaration order; the immediately following typed local is initialized by one canonical `match(carrier)`. The helper and carrier local evaluate once, only the selected arm evaluates, and later locals plus the terminal return continue. The v0.47 carrier matrix remains closed. Non-adjacent, terminal-return, argument, nested, multiple, and computed forms; propagation changes; Result widening; statements; effects; and runtime behavior remain excluded.

- `v0.49.0` adds bounded two-carrier matching through `pipe-two-carrier-matches`. One public pure method may contain exactly two non-overlapping v0.48-compatible adjacent helper-call carrier and canonical match-local pairs. Ordinary locals may surround or separate the pairs but cannot split either pair. Each helper receives every caller parameter directly once in declaration order; both full carriers are validated, helpers and selected locals evaluate once in source order, and only each selected arm evaluates. Existing zero-match and one-match forms remain exact. A third match, non-adjacent/overlapping pairs, terminal-return, argument, nested, computed, or widened helper/carrier forms; propagation changes; statements; effects; and runtime behavior remain excluded.

- `v0.50.0` adds dependent second-carrier matching through `pipe-dependent-second-carrier-match`. The new form is exactly four contiguous locals: a v0.49-compatible first helper carrier and canonical match, followed by a second helper carrier and canonical match. The second public pure same-class helper receives the first selected local, then every caller parameter directly once in declaration order. The closed carrier matrix, once-only source ordering, full-carrier validation, selected-arm-only evaluation, and all inherited zero/one/independent-two-match forms remain exact. Other local argument arrangements, computed/reordered/repeated arguments, gaps inside the four-local stage, third/nested/terminal matches, propagation changes, arbitrary Result widening, statements, effects, and runtime behavior remain excluded.

- `v0.51.0` adds general dependent carrier chains through `pipe-dependent-carrier-chain`. One public pure method may contain a contiguous chain of two or more carrier/match pairs. The first helper receives the caller parameters exactly; every later public pure same-class helper receives only the immediately preceding selected local followed by every caller parameter directly once in declaration order. The existing closed carrier matrix, canonical arms, once-only source order, full-carrier validation, and selected-arm-only evaluation remain exact. Existing zero/one, v0.49 independent two-pair, and v0.50 dependent two-stage forms remain exact. Gaps, mixed independent/dependent chains, non-immediate dependencies, fan-in, computed/reordered/repeated/omitted/extra arguments, matches outside the exact chain, propagation changes, arbitrary Result widening, statements, effects, and runtime behavior remain excluded.

- `v0.52.0` adds cumulative fan-in carrier chains through `pipe-cumulative-fan-in-chain`. A public pure method may choose one contiguous chain of at least three carrier/match pairs where every later public pure same-class helper receives every prior selected local directly once in chain order, followed by every caller parameter directly once in declaration order. The inherited v0.51 immediate-only mode remains exact, and one method cannot mix modes. The closed carrier matrix, canonical arms, once-only source order, full-carrier validation, and selected-arm-only evaluation remain unchanged. Partial, reordered, repeated, omitted, or extra prior selections; gaps; computed arguments; matches outside the chain; propagation changes; arbitrary Result widening; statements; effects; and runtime behavior remain excluded.

- `v0.53.0` adds multi-parameter helper propagation through `pipe-multi-parameter-helper-propagation`. A public pure method with at least two parameters may store one public pure same-class helper call as its first typed local, passing every caller parameter directly once in declaration order, then immediately propagate that carrier into the payload local. The helper is called once, the complete carrier is validated, success is copied, and absence or failure returns the canonical carrier. The inherited one-parameter v0.42 form and closed carrier matrix remain exact. Computed, reordered, repeated, omitted, extra, cross-owner, private, overloaded, or generic helpers; extra propagation; arbitrary Result widening; statements; effects; and runtime behavior remain excluded.

- `v0.54.0` adds checked-arithmetic helper propagation through `pipe-checked-arithmetic-helper-propagation`. The exact v0.53 first-two-local form additionally admits an existing `Result<int, ArithmeticError>` or `Result<float, ArithmeticError>` public pure same-class helper. Every caller parameter is passed directly once in declaration order; the helper evaluates once; the complete carrier is validated; success is copied; and canonical overflow or division-by-zero returns before the terminal checked arithmetic expression. Direct-parameter or call-inside-propagate forms, additional propagation, other Result carriers, statements, effects, actions, runtimes, targets, adapters, UI, and deployment behavior remain excluded.

- `v0.55.0` adds direct-parameter checked propagation through `pipe-direct-checked-arithmetic-propagation`. A public pure method may take one sole `Result<int, ArithmeticError>` or `Result<float, ArithmeticError>` parameter equal to its return type, propagate it as the first typed local, and continue with one admitted checked arithmetic expression. The complete carrier is validated; success is copied; and canonical overflow or division-by-zero returns before the continuation. Additional parameters, helper or computed operands, later or extra propagation, arbitrary Results, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; the v0.54 helper form stays exact.

- `v0.56.0` adds multi-parameter direct checked propagation through `pipe-multi-parameter-direct-checked-propagation`. The exact new form takes a checked arithmetic Result first and one identical-payload scalar second, propagates the carrier as the first typed local, then uses that local as the left operand and the scalar as the right operand of checked integer add/subtract/multiply or binary64 divide. Incoming failure returns before the continuation. A third or reordered parameter, mismatched scalar, reversed/repeated/literal/computed operands, helper/computed propagation operands, later or extra propagation, arbitrary Results, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; the v0.55 and v0.54 forms stay exact.
- `v0.57.0` adds two-stage checked propagation through `pipe-two-stage-checked-propagation`. The exact form takes one checked arithmetic Result followed by two matching payload scalars, propagates the incoming carrier, stores one checked local Result from the first local/scalar operation, propagates that explicit carrier, and returns the second local/scalar checked operation. Integer stages independently admit add/subtract/multiply; float stages admit binary64 divide. Incoming, first-stage, and terminal failures remain canonical. Computed propagation operands, missing or additional stages, reordered/repeated/literal operands, helpers, arbitrary Results, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; v0.54-v0.56 stay exact.
- `v0.58.0` adds generalized checked-propagation chains through `pipe-checked-propagation-chain`. The exact form takes one checked arithmetic Result followed by `K >= 2` matching payload scalars, propagates the incoming carrier, then spells every non-terminal checked stage as an adjacent explicit Result local and propagation local before the terminal local/scalar operation. Integer stages independently admit add/subtract/multiply; float stages admit binary64 divide. Every complete carrier is validated once and incoming or intermediate failure returns before later evaluation. Fewer than two stages, missing/additional chain locals, ordinary-local gaps, reordered/repeated/literal/computed operands, helpers, arbitrary Results, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; v0.54-v0.57 stay exact.
- `v0.59.0` adds bounded cross-payload Result propagation through `pipe-cross-payload-result`. One public pure method takes exactly one direct `Result<T, string>`, propagates it into the first and only typed `T` local, and terminally calls one public pure same-class `T -> Result<U, string>` helper with that local. `T` and `U` must be distinct and each is exactly text or a list of an existing public primitive record. Incoming failure is validated and copied into a canonical target-shaped failure without invoking the helper. Arbitrary errors, Optional/arithmetic carriers, extra parameters/locals/propagations, computed carriers, helper propagation, computed/private/cross-class/mismatched helpers, inference, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; v0.54-v0.58 stay exact.
- `v0.60.0` adds exactly two-stage bounded cross-payload Result propagation through `pipe-two-stage-cross-payload-result`. One public pure method takes a sole direct `Result<T, string>`, propagates it, stores the exact public same-class `T -> Result<U, string>` helper result, propagates that explicit carrier, and terminally calls an exact public same-class `U -> Result<V, string>` helper. `T`, `U`, and `V` are text or lists of existing public primitive records; adjacent payloads differ, while `T` may equal `V`. Incoming or intermediate failure skips all later helpers and becomes a canonical target-shaped failure. Longer chains, same-payload adjacent stages, extra parameters/locals, computed or helper propagation, arbitrary errors, private/cross-class/mismatched helpers, inference, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; v0.54-v0.59 stay exact.
- `v0.61.0` generalizes bounded cross-payload Result propagation through `pipe-cross-payload-result-chain`. One public pure method takes a sole direct `Result<T0, string>` and spells `K >= 2` adjacent helper stages as an initial direct propagation, an explicit helper-Result/propagation pair for every non-terminal stage, and one terminal helper call. Every payload is text or a list of an existing public primitive record; adjacent payloads differ, while non-adjacent payloads may match. Every complete carrier is validated and copied once; any failure skips all later helpers and becomes a canonical final-target-shaped failure. Fewer than two stages in the generalized form, same-payload adjacent stages, gaps or extra locals, computed or helper propagation, arbitrary errors, private/cross-class/mismatched helpers, inference, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; v0.54-v0.60 stay exact.
- `v0.62.0` adds contextual bounded cross-payload Result propagation through `pipe-contextual-cross-payload-result-chain`. The exact v0.61 `K >= 2` chain gains one second direct `string` context parameter. Every helper receives the immediately preceding payload local first and that unchanged context parameter second. Explicit carrier/propagation pairs, bounded text or primitive-record-list payloads, adjacent payload inequality, shared string failure, same-class public pure helpers, and canonical final-target-shaped failure remain exact. Missing, reordered, repeated, computed, non-string, stage-specific, or additional context arguments; a third caller parameter; same-payload stages; gapped or additional locals; computed or helper propagation; arbitrary errors; statements; effects; actions; runtimes; targets; adapters; UI; and deployment remain excluded; v0.54-v0.61 stay exact.
- `v0.63.0` adds the one-stage contextual bounded cross-payload Result form through `pipe-contextual-cross-payload-result`. One public pure method takes exactly a direct `Result<T, string>` carrier and direct `string` context, propagates the carrier into its first and only typed local, then terminally calls one exact public pure same-class `(T, string) -> Result<U, string>` helper with the local followed by the unchanged context. `T` and `U` remain distinct bounded text or primitive-record-list payloads; incoming failure becomes a canonical target-shaped failure without invoking the helper. A missing, reordered, repeated, computed, non-string, or additional context; a third caller parameter; same-payload flow; extra locals or propagation; computed or helper propagation; arbitrary errors; statements; effects; actions; runtimes; targets; adapters; UI; and deployment remain excluded; v0.54-v0.62 stay exact.
- `v0.64.0` adds one-stage contextual bounded same-payload Result propagation through `pipe-contextual-same-payload-result`. The exact v0.63 caller/helper shape additionally permits identical source and target payloads: `Result<T, string>` to `Result<T, string>`, where `T` is text or a list of an existing public primitive record. Incoming failure remains canonical and skips the helper; success passes the unchanged direct string context once. Same-payload contextual chains with two or more stages, additional or computed context, a third caller parameter, extra locals or propagation, arbitrary errors, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; v0.54-v0.63 stay exact.
- `v0.65.0` adds exact two-stage contextual bounded Result propagation through `pipe-two-stage-contextual-bounded-result`. The exact v0.62 two-helper shape additionally permits equality at either or both adjacent payload transitions; each payload remains text or a list of an existing public primitive record, every helper receives the same unchanged direct string context, and failures short-circuit into the canonical final Result shape. Same-payload transitions in contextual chains with three or more stages, additional or computed context, a third caller parameter, extra or gapped locals, arbitrary errors, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; v0.54-v0.64 stay exact.
- `v0.66.0` generalizes contextual bounded Result propagation through `pipe-generalized-contextual-bounded-result`. One public pure method admits `K >= 2` contiguous helper stages, and any adjacent bounded text or primitive-record-list payloads may be equal or different. Every non-terminal helper Result is stored and directly propagated, the final stage is a terminal helper call, and every exact public pure same-class helper receives the immediately preceding payload plus the same unchanged direct string context. Fewer than two stages, additional or computed context, a third caller parameter, missing, additional, or gapped locals, arbitrary errors, statements, effects, actions, runtimes, targets, adapters, UI, and deployment remain excluded; v0.54-v0.65 stay exact.
- `v0.67.0` generalizes shared context arity through `pipe-generalized-shared-context-result`. One public pure method admits a `K >= 2` contextual bounded Result chain with one direct carrier followed by `N >= 1` direct string contexts. Every helper receives the preceding payload followed by every unchanged context exactly once in caller declaration order. Missing, reordered, repeated, computed, non-string, or stage-specific contexts; one-stage multi-context chains; missing, additional, or gapped locals; arbitrary errors; statements; effects; actions; runtimes; targets; adapters; UI; and deployment remain excluded; v0.54-v0.66 stay exact.
- `v0.68.0` generalizes one-stage shared context arity through `pipe-generalized-one-stage-shared-context-result`. One public pure method takes a direct bounded Result carrier followed by `N >= 1` direct string contexts, directly propagates the carrier into its first and only typed local, and terminally calls one exact public pure same-class helper with the payload followed by every unchanged context exactly once in caller declaration order. Missing, reordered, repeated, computed, non-string, or stage-specific contexts; additional or gapped locals; arbitrary errors; statements; effects; actions; runtimes; targets; adapters; UI; and deployment remain excluded; v0.54-v0.67 stay exact.
- `v0.69.0` adds the exact terminal statement-level `if/else` form through `pipe-terminal-if`. One public pure method has one or more ordered immutable locals followed by one terminal `if (bool) { return T; } else { return T; }`; both branches use already-admitted eager pure expressions and return the declared method type, and at most one preceding local initializer may use the inherited bounded conditional expression. Zero-local forms, branch locals, nesting, missing `else`, fallthrough, returns elsewhere, propagation or matching in the method, assignment, loops, effects, inference, and deployment remain excluded; v0.54-v0.68 stay exact.
- `v0.70.0` adds one lexical immutable local per terminal branch through `pipe-terminal-if-branch-local`. After one or more top-level ordered immutable locals, either or both terminal `if/else` branches may declare exactly one explicitly typed immutable local and immediately return from that branch. Branch bindings are independently scoped, evaluate only on the selected branch, and cannot escape. Nested branches, multiple locals in one branch, propagation, matching, assignment, fallthrough, effects, inference, and deployment remain excluded; the v0.69 direct-return form remains valid.

PipeLang `v0.89.0` adds complete depth-two ternary returns in any subset of terminal
`if/else` leaves through statement depth three. `pipe-nested-leaf-return` supplies an
example. Each scope retains finite typed locals and nonnested conditional initializers;
reached locals are eager once in order and only selected conditions/arms execute.
Exact types, lexical scope and complete carriers remain required. Nested initializers,
deeper choices/trees, new condition/argument placements and effects remain excluded.

PipeLang `v0.90.0` adds complete depth-two ternary initializers in finite typed local
sequences in straight-line block methods. Use `pipe-nested-initializers`; later locals
and existing returns may reuse selected values. Initializers execute eagerly once in
order, including unused locals; nested conditions and arms are lazy. Nested initializers
in statement trees, nested arrow methods, deeper choices and new condition/argument
placements remain excluded. Exact types, lexical scope and complete carriers are preserved.

PipeLang `v0.91.0` adds complete depth-two ternary initializers throughout terminal
if/else trees through statement depth three. Root, intermediate and leaf scopes retain
finite typed immutable-local sequences and ordinary/depth-two returns. The
`pipe-nested-tree-initializers` snippet shows root and branch selections. Initializers
execute eagerly once in source order, including unused locals; only selected conditions
and arms execute. Nested arrow methods, deeper choices and new condition/argument or
match/propagate placements remain excluded. This is syntax/snippet guidance, not a
claim of interactive editor or semantic language-server verification.

PipeLang `v0.92.0` adds complete depth-two ternary arrow-method bodies via
`pipe-nested-arrow`. Either or both arms may contain one further choice. Conditions
remain bool, result types exact, and only selected conditions/arms execute. Deeper
choices and new condition/argument placements remain excluded. This is a syntax
snippet; semantic diagnostics still require the compiler language-service path.

PipeLang `v0.93.0` adds `pipe-depth-three-return` for complete ternary returns
through three decisions per path in straight-line public pure block methods.
Preceding typed immutable locals retain ordered eager execution. Initializers,
arrow bodies and terminal-tree leaf returns retain depth two; conditions and
arguments gain no new ternary placement. See the
[canonical contract](../../../../../docs/concepts/pipelang.md#pipelang-v0930-depth-three-straight-line-returns).

PipeLang `v0.94.0` adds `pipe-depth-three-leaf-return` for complete ternary returns
through depth three in terminal-tree leaves. Statement trees retain depth three;
initializers and arrow bodies retain depth two. Conditions remain bool, types exact,
locals eagerly ordered and selected branches/arms lazy.

PipeLang `v0.95.0` adds `pipe-depth-three-arrow` for complete ternary arrow bodies
through depth three. Either or both arms may nest. Initializers retain depth two;
statement trees and block returns retain depth three. Conditions are bool, arm types
exact, and only reached conditions and selected arms execute. See the
[canonical contract](../../../../../docs/concepts/pipelang.md#pipelang-v0950-depth-three-expression-bodied-methods).

PipeLang `v0.96.0` adds `pipe-depth-three-initializer` for complete ternary initializers
through depth three in straight-line typed-local sequences. Initializers before/inside
statement trees retain depth two. Reached locals execute eagerly once in order, including
unused locals; only selected arms execute. See the
[canonical contract](../../../../../docs/concepts/pipelang.md#pipelang-v0960-depth-three-straight-line-initializers).

PipeLang `v0.97.0` adds `pipe-depth-three-terminal-initializer` for complete ternary
initializers through depth three before and inside inherited terminal if/else trees
through statement depth three. Typed immutable locals retain lexical scope, eager
once-only source order, and lazy selected branches/arms. See the
[canonical contract](../../../../../docs/concepts/pipelang.md#pipelang-v0970-depth-three-terminal-initializers).

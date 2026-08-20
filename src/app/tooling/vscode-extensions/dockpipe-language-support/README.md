# DockPipe Language Support (VS Code)

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

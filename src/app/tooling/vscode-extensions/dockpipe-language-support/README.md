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

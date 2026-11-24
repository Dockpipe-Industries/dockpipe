# TASK-055 architecture findings

Source review on 2026-10-31. These are checkout observations, not release or native
platform qualification. PipeLang source references name its owning repository;
no sibling source is linked into Dockpipe. Recheck these seams before implementation,
particularly while the independent native migration is active.

## Verified integration seams

| Area | Owning source | Finding and consequence |
| --- | --- | --- |
| YAML model | [workflow.go](../../../../src/lib/domain/workflow/workflow.go), `Workflow`, `Step`, `InputBinding` | `Step.Outputs` is a string path; inputs ultimately contain string `Value`/`From`; IDs are optional. There is no inline computation namespace, typed step-result store or general step `when` field here. These require additive contracts. |
| YAML loading/imports | [workflow_imports.go](../../../../src/lib/domain/workflow/workflow_imports.go), `ParseWorkflowFromDisk` | Resolves imports and flattens groups; import depth is bounded at 32. This is not a computation dependency graph. Retain authored-file origin before flattening. |
| Validation | [workflow_validate.go](../../../../src/lib/infrastructure/workflow_validate.go), `ValidateResolvedWorkflowYAML`; [schema](../../../../src/lib/infrastructure/schema/workflow.schema.json) | Loads/domain-validates workflows, checks resolver/script dependencies, then validates decoded YAML against embedded JSON Schema. Unknown fields produce unsupported-version guidance. The proposed syntax must currently be rejected. |
| Source-aware editing | [document.go](../../../../src/lib/domain/workflowdocument/document.go) | Uses `yaml.Node`, preserves unedited comments, and bounds visual edits at 1 MiB/8,192 nodes/depth 64. Its AST machinery is a possible source-map seam, not an embedded-code source mapper or execution budget. |
| Inputs | [bindings.go](../../../../src/lib/application/internal/workflowinputs/bindings.go), `ResolveWorkflow`, `ResolveStep` | Typed field discovery maps explicit literals/environment references into string environment bindings. Missing/blank sources may use fallbacks. Reuse field discovery/adapters, but do not silently inherit these fallback rules for typed computation. |
| Execution | [run_steps.go](../../../../src/lib/application/run_steps.go), `runSteps`; [run_steps_parallel.go](../../../../src/lib/application/run_steps_parallel.go), `runParallelBatch` | Ordered blocking steps and adjacent async batches; siblings use a pre-batch environment snapshot, wait as a batch, then merge outputs in YAML order. No general typed-result readiness DAG is implemented in these owners. |
| Outputs | [workflow_env.go](../../../../src/lib/application/workflow_env.go), `applyOutputsFile` | Parses dotenv, ignores missing/empty/unreadable results, protects locked bindings, merges strings and removes the file. A typed adapter must capture and strictly validate declared fields before deletion; it cannot reinterpret this as a typed JSON result API. |
| Existing expressions | Workflow/input owners above; [workflow reference](../../../workflows/workflow-yaml.md) | Commands are shell strings and inputs use value/from bindings. The inspected workflow model/runner has no `${{ ... }}` PipeLang evaluator. Preserve shell and existing authoring behavior rather than replacing it with a new expression engine. |
| Build closure | [closure.go](../../../../src/lib/application/internal/packagecompile/closure.go), `compileClosureForWorkflow` | Orders workflow/resolver/package compilation from declared dependencies. Reuse closure traversal and cycle-handling patterns; build dependency nodes are not runtime step-value nodes. |
| Build/package staging | [workflow.go](../../../../src/lib/application/internal/packagecompile/workflow.go), `compileWorkflowOne`; [packagebuild](../../../../src/lib/infrastructure/packagebuild/tarball.go) | Validated staged copies, compile hooks, PipeLang materialization, deterministic archives/checksums and guarded publication already exist. Add computation artifacts to this closure without generic user shell compile hooks becoming the language adapter. |
| Automatic compilation | [run.go](../../../../src/lib/application/run.go), `compileDepsWanted` | Run can request transitive compilation today. TASK-055 must explicitly avoid triggering embedded-code compilation from this path; stale feature artifacts require an explicit build. |
| Current Dockpipe/PipeLang calls | [pipelang_cmd.go](../../../../src/lib/application/pipelang_cmd.go); [packagecompile/pipelang.go](../../../../src/lib/application/internal/packagecompile/pipelang.go); [input discovery](../../../../src/lib/application/internal/workflowinputs/discovery.go); [go.mod](../../../../go.mod) | Dockpipe still imports the pinned external Go module v0.1.0 and its CLI/compiler/materializer APIs. A native installed C++ workflow-evaluation adapter is not present. Preserve these compatibility callers until their owning migration qualifies replacements. |
| Installed selection | [package model](../../../packages/package-model.md), `packages.sources`; [version pinning](../../../packages/version-pinning.md) | Existing declared sources/stores and trusted archive admission can locate dependency payloads. Installed inventory is not exact active selection. Project/package selection locks remain a proposal; do not invent an official lock schema for TASK-055. |
| Artifact fingerprints | [imageartifact/manifest.go](../../../../src/lib/application/internal/imageartifact/manifest.go); [runtimeartifact model](../../../../src/lib/model/runtimeartifact/model.go) | Stable JSON, source/provenance fingerprints and policy fingerprints are useful conventions. Image manifests do not already describe language artifacts or prove compiler authenticity. Add a focused computation manifest rather than pretending image metadata covers it. |
| Process lifetime | [run_unix.go](../../../../src/lib/infrastructure/process/run_unix.go); [run_windows.go](../../../../src/lib/infrastructure/process/run_windows.go) | Context-bound Unix process groups and suspended Windows processes assigned to kill-on-close Job Objects support evaluator supervision. They do not remove filesystem/network/environment access or establish portable hard memory/CPU limits. |
| Policy | [runtimepolicy/security.go](../../../../src/lib/application/internal/runtimepolicy/security.go); [security policy](../../../security/security-policy.md) | Effective security manifests exist. Host steps are outside Docker policy; some network controls are advisory. Neither is sufficient isolation for compiled untrusted native code. |
| Audit | [execution_observer.go](../../../../src/lib/infrastructure/execution_observer.go); [operation_event.go](../../../../src/lib/infrastructure/operationrecord/operation_event.go) | Metadata-only process observation and canonical operation-event infrastructure exist. Observation is best-effort and explicitly does not supervise execution. Required decision receipts need stronger admission/publication semantics, while reusing event/run IDs and storage helpers. |

## PipeLang owner contracts inspected

- `src/native/model-compiler/main.cpp` accepts bounded JSON protocols
  `pipelang.native-model.v1/v2/v3`; v2/v3 cover module/object variants. It emits
  generated headers/manifests. Model diagnostics expose `PLM1001`, message and
  byte offset, plus module when applicable. This is not a callable portable
  workflow evaluator or a complete YAML-aware diagnostic API.
- `src/native/model-compiler/checker.cpp` and `generator.cpp` own pure-method
  checks and native emission. Those checks are reusable compiler work, not a
  host capability sandbox or proof that arbitrary emitted native code is safe.
- `sdk/pipeview/model/cmake/PipeLangModelConfig.cmake.in` exposes installed compiler,
  generator and manifest paths. `PipeLangModelBindings.cmake` invokes the SDK tool
  with exact manifest SHA256 and explicit compiler paths; `PipeLang::ModelRuntime`
  belongs to the exact `PipeLangModel` 0.1.0 CMake package. It is a C++ header/runtime contract, not a stable cross-language value ABI.
- `src/native/sdk-tools/manifest.cpp` validates exact version/protocol, roots,
  nonsymbolic paths and per-file hashes. Reuse this admission rather than PATH,
  newest-installed selection or sibling builds. The legacy CLI's development
  version output alone is not an exact installed SDK identity.
- `src/native/legacy-compiler/evaluate.cpp` provides a bounded configuration-expression
  evaluator, while `diagnostics.cpp` exposes schema-1 structured diagnostics with
  primary/related ranges and UTF-16 columns. These are useful owner precedents;
  neither supplies the proposed modern exported-function/artifact evaluator.
- `src/native/scalar-compiler/main.cpp` uses an explicitly selected, hash-checked
  emitter and records source/generated/compiler identities. It emits C++ scalar
  artifacts, not a general sandboxed workflow computation module.
- No WASM backend or portable artifact evaluator was found among the inspected
  native compiler/SDK owners. This does not prove none exists elsewhere; the
  proposed first profile must have a published, qualified owner contract before
  Dockpipe implementation can depend on it.

## Execution-format assessment

| Option | Existing fit | Decision |
| --- | --- | --- |
| Load generated C++ through a C ABI in Dockpipe | Current SDK emits native headers; a C ABI would still need designing | Reject for v1: compiler bugs/native memory corruption could compromise the orchestrator. Never expose C++ object layouts across the boundary. |
| Native evaluator subprocess with OS sandbox | Existing process supervision and native compiler artifacts are reusable | Lifetime control alone is insufficient. A complete sandbox on every supported OS is an unfulfilled prerequisite; no quiet host/container fallback. |
| Restricted PipeLang interpreter | Legacy evaluator is too narrow; no qualified modern artifact interpreter found | Possible upstream alternative, but must be PipeLang-owned with independently enforced metering. Dockpipe must not implement a second language. |
| WASM computation artifact in a supervised evaluator, no WASI/host imports | Fits a small pure subset and avoids native artifact portability | Recommended direction, gated on a qualified compiler profile, metered engine and SDK value adapter. Prefer interpretation for v1; any AOT compilation happens during build. No dependency/engine is selected or installed by this proposal. |

WASM limits access through exposed imports, but engine defects remain part of the
trust boundary; this is not a claim of perfect isolation. See the primary
[WebAssembly security model](https://webassembly.org/docs/security/).
Runtime selection must separately verify bounded memory, metering and interruption.
Wasmtime documents [fuel versus wall-clock interruption](https://docs.wasmtime.dev/examples-interrupting-wasm.html)
and [determinism constraints](https://docs.wasmtime.dev/examples-deterministic-wasm-execution.html);
these are assessment references, not a commitment to that dependency. Fuel does
not replace host supervision, bounded decoding, or memory/stack limits.

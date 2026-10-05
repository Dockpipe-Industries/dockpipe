## Application/Domain/Infrastructure/PipeLang Structure Refresh Audit — 2026-08-15

Completed only the user-authorized offline structure refresh audit covering every Go file under
`src/lib/application`, `src/lib/domain`, `src/lib/infrastructure`, and `src/lib/pipelang`. The audit
made no Go/source, test, schema, configuration, manifest, package, workflow, generated, runtime,
durable, ignored, or external change. It used no network, Docker, VM, live action, staging, commit,
push, worktree, or handoff.

Before the documentation-only audit update, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 56 default-collapsed untracked paths, and status SHA-256
`ddcfa1cf8cf03c141c1b01edbf06aa48b5bc45ae45100855859184eefa0cf349` matched the state left by
the completed resolver split. This task record was
`a17a2f27635033d6a25660ffd56d094ff4a584932c34f8541e87f8da84cafe2f`; the aggregate digest of
the sorted per-file SHA-256 inventory for every file below `src/` was
`71e15103614b79a1efedf4832cd716402f44065be5596e7374df2b483c192d7e`. The protected ignored
Python cache remained a regular file at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

### Refreshed package inventory and directions

Counts use physical Go lines and count `Test`, `Benchmark`, and `Fuzz` declarations as test
functions. Direct means the named package directory only; recursive includes child packages.

| Package | Scope | Production Go files/lines | Test Go files/lines | Test functions |
| --- | --- | ---: | ---: | ---: |
| `application` | Direct | 73 / 19,213 | 59 / 12,512 | 339 |
| `application` | Recursive | 75 / 19,668 | 62 / 13,035 | 375 |
| `domain` | Direct and recursive | 16 / 2,986 | 13 / 2,428 | 118 |
| `infrastructure` | Direct | 58 / 13,432 | 46 / 8,213 | 258 |
| `infrastructure` | Recursive | 65 / 15,052 | 52 / 8,955 | 275 |
| `pipelang` | Direct and recursive | 6 / 1,806 | 3 / 306 | 13 |

The inventory includes tracked, dirty, and untracked files. Application currently has 28 tracked
dirty and 17 untracked paths in scope; domain has seven and one; infrastructure has 11 and 17.
PipeLang has no dirty or untracked path, but all nine of its Go files are included in the inventory.
Tests remain colocated; there is no generic tests directory.

Application remains the fan-in layer. Its production files import `domain` from 45 files, root
`infrastructure` from 55, `packagebuild` from 13, `fetchinstall` from one, and `pipelang` from
three. The two application internal leaves do not import their parent. Root infrastructure imports
`domain` from 10 production files and `packagebuild` from six; `packagebuild` imports only `domain`,
and root infrastructure does not import `fetchinstall`. Domain and PipeLang import no DockPipe
package. The previous description of Domain as literally I/O-free is corrected: it is
dependency-inward, but existing compile-root, project-config, package-manifest, import-path, and
validation contracts intentionally use `os` and path primitives.

PipeLang is structurally cohesive rather than dumped into a generic file: `ast.go`, `lexer.go`,
`parser.go`, `typecheck.go`, `eval.go`, and `compile.go` separately own syntax/value contracts,
tokenization, parsing, semantic validation, evaluation, and compile/invoke plus emitted bindings.
The 460-line parser is one recursive-descent responsibility, and compile/emission remains a
381-line facade exercised by golden and invocation tests. No PipeLang package or file split is
warranted by current size, dependency direction, or test locality.

### Current largest responsibilities

| Production file | Lines | Functions/types | Refreshed assessment |
| --- | ---: | ---: | --- |
| `application/run_steps.go` | 1,268 | 49 / 2 | Blocking and parallel execution, resolver dispatch, env/scopes, host builtins, and pre-scripts remain; the exact parallel block is now the clearest bounded seam. |
| `infrastructure/git_runtime_session.go` | 1,187 | 32 / 10 | Git/Docker session lifecycle, leases, cleanup, and namespace enforcement remain security- and state-coupled. |
| `application/run.go` | 1,143 | 6 / 0 | The approximately 1,019-line `Run` coordinator remains high-coupling control flow, not a byte-move candidate. |
| `infrastructure/docker.go` | 898 | 27 / 1 | Build/run, volume synchronization, Git bootstrap, TTY, user/env behavior, and failures remain OS/runtime coupled. |
| `infrastructure/durable_import.go` | 877 | 24 / 6 | Validation, publication, recovery, manifest comparison, and sync remain one security transaction. |
| `application/package_compile.go` | 869 | 19 / 0 | Core and resolver blocks are gone; workflow, batch-workflow, and all-target orchestration remain identifiable but share compile helpers. |
| `infrastructure/git_checkpoint_request.go` | 853 | 35 / 7 | Request/receipt validation, Git transaction, persistence, JSON, and path safety remain one controlled-checkpoint boundary. |
| `domain/workflow.go` | 796 | 26 / 47 | Validation is gone; model declarations, YAML normalization, step helpers, and parsing remain tightly shared. |
| `application/sdk_cmd.go` | 721 | 24 / 2 | Scope projection is visible, but SDK/get/workdir/binary helpers make the current move boundary non-contiguous. |
| `infrastructure/durable_state.go` | 690 | 23 / 5 | Public durable identity/location/runtime APIs and OS path validation remain cohesive. |
| `application/catalog_inputs.go` | 680 | 26 / 2 | Typed-input/default/view projection remains one PipeLang-backed responsibility with direct tests. |
| `pipelang/parser.go` | 460 | 18 / 1 | One recursive-descent parser responsibility; no split selected. |

### Reconciled and newly ranked findings

The resolver-compilation candidate from the preceding audit is closed by
`package_compile_resolvers.go`. The earlier Domain validation and catalog/PipeLang projection splits
remain valid. File-count growth comes from responsibility siblings and test-colocated internal
leaves, not a generic dumping directory.

| Rank | Bounded finding | Disposition |
| ---: | --- | --- |
| 1 | **Application parallel run-step orchestration.** The contiguous block from `runParallelBatch` through `runParallelStepWorker` owns async-group validation, Docker build prefetch, bounded worker execution, error capture, and ordered output merge. | **Selected.** It is a 277-line/six-function in-package seam with one external caller and five direct tests. A byte-preserving sibling move changes no concurrency or runtime behavior. |
| 2 | **Application SDK scope projection.** `cmdScope`, scope records, path projections, and resolver-auth values remain visible in `sdk_cmd.go`. | **Deferred.** SDK shell-env and shared workdir/path helpers interrupt the candidate; require a fresh exact caller/import boundary before selection. |
| 3 | **Infrastructure controlled-checkpoint persistence/path helpers.** Receipt loading, fingerprints, JSON, and path checks remain visible in `git_checkpoint_request.go`. | **Deferred.** They participate in the same security transaction and durable receipt contract. |
| 4 | **High-coupling coordinators.** `Run`, Git-session lifecycle, Docker execution, durable import publication/recovery, and durable-state public APIs remain large. | **Deferred as cohesive or audit-required.** Size alone does not justify a split. |
| 5 | **Domain and PipeLang.** Workflow models/parsing and the six-file PipeLang compiler pipeline were explicitly reviewed. | **No current split.** Their existing file/package boundaries are cohesive, dependency-inward, and test-local. Re-rank only after material feature growth. |

The generic-boundary scan also found pre-existing product-specific wording in Domain comments and
Claude-focused Docker comments plus the `IS_SANDBOX` injection in `infrastructure/docker.go`.
Those are not structure moves. The Docker behavior in particular requires a separately approved
engine/compatibility audit before any change; this audit does not classify its removal as safe.
Compiled-package `workflows/` archive prefixes and centralized embedded-workflow paths remain
package/artifact mechanics, not newly introduced checkout-path knowledge.

### Exact separately gated successor

Exactly one successor is selected: **TASK-035 parallel run-step orchestration responsibility
split**.

- **Result:** create `src/lib/application/run_steps_parallel.go`; move byte-for-byte only the exact
  contiguous 277-line block beginning at `func runParallelBatch` and ending after
  `runParallelStepWorker`, including `validateParallelOutputPaths`,
  `validateParallelNoResolverDelegate`, `validateParallelNoHostCommit`,
  `prefetchDockerBuildsForBatch`, comments, and the trailing declaration separator. Remove only the
  original block and the `maps`/`sync` imports proven exclusive to it. Keep the caller in
  `runSteps`, all helpers, tests, signatures, errors, log/operation-result bytes, scheduling,
  cancellation/error selection, build prefetch, output ordering, and behavior unchanged. The new
  file must remain an in-package sibling, not a package or abstraction.
- **Grounding:** `run_steps.go` is SHA-256
  `41df3029464a582551f1e1340f1adb235be12f3a4e299a61cc68d2e7f8b4cef6`; the destination is absent;
  the exact 277-line block is SHA-256
  `89fa30cb223c17b1012510c4c19092b34da3100b6ee396d58aaa69fb569cdcab`. `runSteps` is the only
  external production caller. The five direct tests are
  `TestRunSteps_ParallelBatchAggregatesOutputsInOrder`, `TestValidateParallelOutputPaths`,
  `TestValidateParallelNoHostCommit`, `TestRunParallelStepWorkerNonZeroExit`, and
  `TestRunParallelStepWorkerFirstStepExtraPreScript`.
- **Exclusions:** no blocking-step, resolver/host/package-workflow, env/scope, host-builtin,
  pre-script, container-preparation, runtime-policy, image-artifact helper, or test/caller move; no
  behavior/API/CLI/help/schema/config/manifest/operation-result/error/log/concurrency/order change;
  no new package/abstraction, compatibility work, cleanup/migration/generated refresh, live,
  Docker/VM/network action, staging, commit, push, worktree, or additional successor.
- **Required proof:** exact moved-block and original-source reconstruction including the removed
  imports and separator; unique declarations and unchanged caller; the five focused tests; Linux
  and Windows/amd64 application test-package compile-only checks; `gofmt`, import ownership,
  `git diff --check`, task/status/full-`src` reconstruction, and protected-cache preservation.

Audit validation used an existing Go 1.25.0 binary and complete module cache with
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`, and isolated `/tmp` cache/temp
roots. `go list -mod=readonly` resolved application and its internal leaves, Domain, Infrastructure
and its child packages, and PipeLang without network access or an import cycle. Structural counts,
declaration maps, import-file counts, caller/test references, destination absence, exact hashes,
generic-boundary references, dirty state, protected cache, and `git diff --check` were inspected.
No test suite was run because the authorized audit changed no source behavior; the selected future
slice owns its exact focused and cross-platform compile proof.

Terminal disposition: `completed`. The selected successor was not implemented, handed off, or
created. It requires the exact separate user response
`Approve next slice: TASK-035 parallel run-step orchestration responsibility split` before any
fresh-task execution.

## Parallel Run-Step Orchestration Responsibility Split — 2026-08-15

Completed only the approved application parallel run-step orchestration responsibility split in
the saved dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 56 default-collapsed untracked paths, and status SHA-256
`ddcfa1cf8cf03c141c1b01edbf06aa48b5bc45ae45100855859184eefa0cf349` matched the delegated
anchors exactly. This task record, `run_steps.go`, the complete sorted per-file `src` inventory,
the absent destination, the exact 277-line source block, and the protected ignored Python cache
also matched every handed-off digest, type, mode, and size.

Created `src/lib/application/run_steps_parallel.go` and moved byte-for-byte only the contiguous
block from `runParallelBatch` through `runParallelStepWorker`, including
`validateParallelOutputPaths`, `validateParallelNoResolverDelegate`,
`validateParallelNoHostCommit`, `prefetchDockerBuildsForBatch`, comments, and the original trailing
declaration separator. The original file lost only that block and its now-unused `maps` and `sync`
imports. The `runSteps` caller, all other helpers and tests, names, signatures, APIs, CLI/help,
error/log and operation-result bytes, scheduling, cancellation/error selection, Docker prefetch,
image-artifact behavior, output ordering, and cross-platform behavior remain unchanged. The
destination is an in-package sibling and introduces no new package, abstraction, or dependency
direction.

The six moved functions reconstructed with their original trailing separator retain SHA-256
`89fa30cb223c17b1012510c4c19092b34da3100b6ee396d58aaa69fb569cdcab`. Reconstructing
`run_steps.go` from the two post-split sources, including the removed imports and separator,
reproduces SHA-256 `41df3029464a582551f1e1340f1adb235be12f3a4e299a61cc68d2e7f8b4cef6`.
Every moved function has exactly one definition, and `runSteps` remains the sole external
production caller of `runParallelBatch`. Reconstructing the sorted per-file `src` inventory while
excluding the new sibling and substituting the reconstructed original reproduces SHA-256
`71e15103614b79a1efedf4832cd716402f44065be5596e7374df2b483c192d7e`, proving every unrelated
`src` byte unchanged.

Focused offline validation used an existing cached Go 1.25.0 binary and complete matching module
cache with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`, and isolated `/tmp`
build/temp roots:

- the five exact tests `TestRunSteps_ParallelBatchAggregatesOutputsInOrder`,
  `TestValidateParallelOutputPaths`, `TestValidateParallelNoHostCommit`,
  `TestRunParallelStepWorkerNonZeroExit`, and
  `TestRunParallelStepWorkerFirstStepExtraPreScript` passed together;
- Linux/amd64 and Windows/amd64 application test packages compiled through non-executing
  `go test -c` commands; the Windows binary was not run;
- `gofmt`, import/declaration ownership, exact block and source reconstruction, unchanged caller,
  task-record and unrelated-`src` reconstruction, dirty ownership, `git diff --check`, and the
  protected-cache type/mode/size/hash passed.

The recorded extracted Go binary path was absent. A cached Go 1.25.0 binary was found under `/tmp`;
the first module-cache probe then stopped before compilation because pinned `golang.org/x/sys`
bytes were absent from that cache. A second existing complete matching offline module cache passed
`go list -mod=readonly`; the required tests and compile checks then passed without a network
fallback. These setup-only stops changed no repository byte. Generated validation artifacts exist
only under `/tmp/dockpipe-task035-parallel-01a0078d-c5ce-7403-8ead-94f0df9035cd`.

Final dirty ownership is 133 tracked paths and 57 default-collapsed untracked paths with status
SHA-256 `948183f18ab980683467508feebecf1ab8204a38806a6c28e328ea895cf955f2`; no files are staged.
Removing only the new destination status entry reconstructs the exact pre-split status SHA-256.
The ignored cache remains a regular file at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No blocking-step, resolver/host/package-workflow, env/scope, host-builtin, pre-script,
container-preparation, runtime-policy, image-artifact helper, caller, test, package boundary,
behavior, API, CLI/help, schema, config, manifest, compatibility/state/cleanup surface, generated
checkout state, live service, Docker/VM/network resource, staging, commit, push, worktree, or
successor was touched. The engine/package boundary remains intact: generic application orchestration
moved only to an in-package sibling with no package or product-specific special case.

Terminal disposition: `completed`. Ranks 2-5 remain non-blocking deferred findings. No successor
was selected, created, or implemented; any later application-structure slice requires a fresh exact
approval.


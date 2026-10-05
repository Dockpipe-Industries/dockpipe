## Application/Domain/Infrastructure Structure Refresh Audit — 2026-08-15

Completed only the approved offline structure refresh audit in the saved dirty checkout. Before
this documentation edit, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 55 default-collapsed untracked paths, and status SHA-256
`1dcf99a02fdefed5cb51981c507c8e168cb77d813b20460a2ce17823b3c06d5c` matched the
delegated anchors. This task record was
`83aeaaa2bd5baaf9b1b7dd93391c75a4b727884cdf8b3493188337638a6b694a`, and the
aggregate digest of the sorted per-file SHA-256 inventory for every file below `src/` was
`deff9fe2ae85fab9f188cfc91f2e0218fbe3eab3c700f20c203e32ff70cd049a`.
The protected ignored Python cache remained a regular file at mode `0664`, size 8,408, and
SHA-256 `25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

### Refreshed size, tests, and package boundaries

Counts use physical Go lines and count `Test`, `Benchmark`, and `Fuzz` declarations as test
functions. Direct means only the named package directory; recursive includes child packages.

| Package | Scope | Production Go files/lines | Test Go files/lines | Total Go files/lines | Test functions |
| --- | --- | ---: | ---: | ---: | ---: |
| `application` | Direct | 72 / 19,197 | 59 / 12,512 | 131 / 31,709 | 339 |
| `application` | Recursive | 74 / 19,652 | 62 / 13,035 | 136 / 32,687 | 375 |
| `domain` | Direct and recursive | 16 / 2,986 | 13 / 2,428 | 29 / 5,414 | 118 |
| `infrastructure` | Direct | 58 / 13,432 | 46 / 8,213 | 104 / 21,645 | 258 |
| `infrastructure` | Recursive | 65 / 15,052 | 52 / 8,955 | 117 / 24,007 | 275 |

The recursive deltas remain intentional and test-colocated. Application has
`internal/imageartifact` at one 141-line production file and one 103-line focused test with one
test function, plus `internal/wslbridge` at one 314-line production file and two 420-line focused
test files with 35 tests. Infrastructure has `fetchinstall` at one 552-line production file and one
247-line test file with 10 tests, plus `packagebuild` at six production files/1,068 lines and five
test files/495 lines with seven tests. Neither application internal leaf imports its parent;
`imageartifact` imports only `domain` inside DockPipe and `wslbridge` imports no DockPipe package.
`packagebuild` imports only `domain`. `fetchinstall` imports root infrastructure, while root
infrastructure does not import `fetchinstall`. No generic tests directory or cyclic child-to-parent
dependency has appeared.

Source import declarations show application production files importing `domain` from 45 files,
root `infrastructure` from 54, `infrastructure/packagebuild` from 12, and `fetchinstall` from one.
Root infrastructure imports `domain` from 10 production files and `packagebuild` from six. Domain
still imports no DockPipe package and depends only on the standard library and YAML. The increases
in application import-file counts are the expected result of completed in-package splits; they do
not introduce a new dependency direction. Application remains the fan-in orchestration layer,
domain remains dependency-inward and I/O-free, and infrastructure remains the adapter layer.

### Current largest and coupled responsibilities

| Production file | Physical lines | Functions/types | Current responsibility assessment |
| --- | ---: | ---: | --- |
| `application/package_compile.go` | 1,298 | 27 / 0 | Still the largest file after the core split; workflow, resolver, batch, and all-target compilation remain independently identifiable. |
| `application/run_steps.go` | 1,268 | 49 / 2 | Container preparation is gone, but resolver/host dispatch, blocking execution, parallel scheduling, env/scopes, builtins, and pre-scripts remain coupled. |
| `infrastructure/git_runtime_session.go` | 1,187 | 32 / 10 | Storage is gone; lifecycle, Git, Docker volumes, leases, cleanup, and namespace enforcement remain security- and state-coupled. |
| `application/run.go` | 1,143 | 6 / 0 | The approximately 1,019-line `Run` coordinator remains a broad control-flow fan-in and is not a byte-move candidate. |
| `infrastructure/docker.go` | 898 | 27 / 1 | Presentation is gone; build/run, volume synchronization, Git bootstrap, TTY, and failure handling remain OS/runtime coupled. |
| `infrastructure/durable_import.go` | 877 | 24 / 6 | Source inventory is gone; validation, publication, recovery, manifest validation, and sync remain one security boundary. |
| `infrastructure/git_checkpoint_request.go` | 853 | 35 / 7 | Request/receipt validation, commit transaction, postimages, JSON, and path safety remain a controlled-Git boundary. |
| `domain/workflow.go` | 796 | 26 / 47 | Validation is gone; workflow/step models, YAML normalization, step helpers, and parsing remain cohesive in the I/O-free package. |
| `application/sdk_cmd.go` | 721 | 24 / 2 | `sdk`, `get`, and `scope` commands still share workdir, path, and child-binary resolution, but a scope seam is visible. |
| `infrastructure/durable_state.go` | 690 | 23 / 5 | Storage primitives are gone; public durable identity/location/runtime APIs and OS path validation remain cohesive. |

### Prior-finding reconciliation

| Classification | Refreshed finding |
| --- | --- |
| **Closed stale** | All nine targets from the prior structure audit are implemented and have their own completed evidence below: core compilation, catalog typed-input/view projection, domain workflow validation, Git-session storage, step-container preparation, workflow Git-session helpers, launch presentation, durable-state storage primitives, and durable-import source inventory. The old active wording for ranks 2 and 3 and the old rank-1 successor are historical, not pending work. |
| **Current** | The internal leaf boundaries, test colocation, import directions, high-coupling `Run` coordinator, and cohesive publication/recovery portion of durable import remain as previously assessed. No further package boundary is warranted by this audit. |
| **Newly ranked debt** | Resolver compilation remains a cohesive 429-line block inside the still-largest production file. Parallel run-step orchestration, SDK scope resolution, and controlled-checkpoint persistence/path helpers are now explicit deferred candidates rather than unclassified large-file debt. |
| **Not debt from these splits** | Production-file and import-file counts increased because declarations moved into responsibility siblings; tests, callers, public signatures, APIs, and package directions did not expand. Line count alone does not justify recombining them. |

### Re-ranked unresolved candidates

Only rank 1 is selected. Ranks 2-5 are non-blocking deferred findings, not additional successors.

| Rank | Bounded candidate | Callers and focused proof | Disposition |
| ---: | --- | --- | --- |
| 1 | **Application resolver compilation.** Move the exact contiguous 429-line block from `cmdPackageCompileResolvers` through `mergeChildPackagesWalk`, including `resolverMetaFilename`, comments, and its trailing declaration separator, from `package_compile.go` to `package_compile_resolvers.go`. | `cmdPackageCompile` and `cmdPackageCompileAll` call the resolver command; `compileClosureForWorkflow` calls `compileSingleResolverDir`. Five direct tests cover vendor-root discovery/tar output, operation results, authored APT/runtime artifacts, invalid stored-tarball rebuild, and staging-only compile hooks. | **Selected.** Eight functions and one constant form one in-package responsibility, reduce the largest file materially, require no API, caller, test, or package-boundary change, and have direct artifact-focused proof. |
| 2 | **Application parallel step orchestration.** A later audit could bound `runParallelBatch` through `runParallelStepWorker` in the 1,268-line `run_steps.go`. | Five named parallel/output/host-commit/worker tests cover important seams, but the block coordinates Docker prefetch, concurrency, pre-scripts, result order, cancellation, outputs, and env copies. | **Deferred.** Higher runtime, concurrency, security, and cross-platform risk than rank 1; re-audit exact boundaries before any move. |
| 3 | **Application SDK scope resolution.** `cmdScope`, scope records, parsing, and workflow/package/resolver path projection in the 721-line `sdk_cmd.go` are a visible sibling responsibility. | `cmdSDK`/subcommand dispatch and seven colocated tests cover scope, get, resolver auth, workdir normalization, and child-binary selection, but shared helpers cross the proposed seam. | **Deferred.** Lower reduction and a non-contiguous shared-helper boundary need a fresh caller split before selection. |
| 4 | **Infrastructure controlled-checkpoint persistence/path helpers.** JSON, fingerprint, receipt-loading, and path-validation helpers occupy the tail of the 853-line request file. | Eight controlled-checkpoint tests cover exact commits, stale/unrelated state, linked paths, index recovery, metadata/receipt failures, and tamper rejection. | **Deferred.** The helpers are part of one security transaction and durable receipt contract; storage separation is not yet proven safer. |
| 5 | **High-coupling coordinators.** `Run`, the remaining Git-session lifecycle, Docker execution, durable import publication/recovery, and durable-state public location APIs remain large. | Broad application/infrastructure suites cover them indirectly, but no narrow byte-preserving seam is established. | **Deferred as cohesive or audit-required.** Do not use size alone to create another package or abstraction. |

### Exact separately gated successor

Exactly one successor is selected: **TASK-035 package-compile resolver responsibility split**.

- **Result:** create `src/lib/application/package_compile_resolvers.go`; move byte-for-byte only the
  contiguous block beginning at `func cmdPackageCompileResolvers` and ending after
  `mergeChildPackagesWalk`, including `resolverMetaFilename`, comments, and the trailing separator.
  Keep `packageCompileResolversUsageText` in the shared help-text section of `package_compile.go`.
  Remove only the original block and any imports proven unused; preserve every name, signature,
  body, caller, test, API, operation-result field, CLI/help byte, and behavior. Update only this task
  record beside the two application source paths.
- **Dependencies and callers:** the moved block uses standard filesystem/path/string primitives,
  YAML, `domain` workflow/package/namespace validation, infrastructure package-store, operation,
  tarball-cache, workflow-validation, and time helpers, and `packagebuild` tar naming/writing. Keep
  the dispatcher and compile-all calls in `package_compile.go` and the closure caller in
  `package_compile_closure.go` unchanged; the child file must remain an in-package sibling, not a
  new package or abstraction.
- **OS, security, and artifact boundary:** preserve `filepath`/`os` traversal and cleanup semantics,
  project package-root resolution through `PackagesResolversDir`, staging-only compile hooks,
  source-copy behavior, workflow and namespace validation, invalid-tarball rebuild and extract-cache
  invalidation, PipeLang materialization, generated package manifests, per-step runtime/image
  artifacts, tarball names and `resolvers/<name>` prefix, operation-result IDs, and cleanup of the
  temporary staging tree. Do not move or rewrite the helpers that implement those contracts.
- **Exclusions:** no workflow, batch-workflow, all-target, core, closure, runtime-artifact,
  image-selection, derived-image, security-policy, or validation split; no helper/test/caller move,
  new package/abstraction, behavior/API/CLI/help/schema/config/manifest/operation-result change,
  compatibility retirement, core-demo work, cleanup/state migration, generated refresh, live
  action, Docker/VM/network use, staging, commit, push, worktree, or further successor.
- **Validation:** hash the exact pre-move block including its separator, reconstruct the original
  `package_compile.go` byte-for-byte, prove each moved declaration has one definition and the three
  external caller sites are unchanged, and reconstruct the full pre-split sorted per-file `src`
  inventory while excluding the new sibling and substituting the reconstructed original. Run the
  five exact focused tests `TestCmdPackageCompileResolversVendorResolversSubdir`,
  `TestCmdPackageCompileResolversEmitsOperationResults`,
  `TestCmdPackageCompileResolverMaterializesAuthoredAptPackages`,
  `TestCompileSingleResolverRebuildsInvalidStoreTarball`, and
  `TestCompileResolverHooksRunInStagingCopy` offline; compile the Linux and Windows/amd64 application
  test packages without executing the Windows binary; verify Go formatting, import ownership,
  `git diff --check`, dirty-state reconstruction, every unrelated `src` byte, and the protected
  ignored-cache type/mode/size/hash.

This audit changes no Go source, test, caller, helper, schema, configuration, manifest, API, CLI,
package, workflow, generated, ignored, runtime, durable, or external byte. The generic engine/package
boundary remains intact: the selected future move is only an in-package organization of generic
resolver compilation and adds no checkout-specific package/workflow/staging knowledge.

Terminal disposition: `completed`. The selected successor was not implemented or created and
requires the exact user response
`Approve next slice: TASK-035 package-compile resolver responsibility split` before one fresh-task
handoff.

## Package-Compile Resolver Responsibility Split — 2026-08-15

Completed only the approved application resolver-compilation responsibility split in the saved
dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 55 default-collapsed untracked paths, and status SHA-256
`1dcf99a02fdefed5cb51981c507c8e168cb77d813b20460a2ce17823b3c06d5c` matched the delegated
anchors exactly. This task record, `package_compile.go`, the complete sorted per-file `src`
inventory, the absent destination, the exact 429-line source block, and the protected ignored
Python cache also matched every handed-off digest, type, mode, and size.

Created `src/lib/application/package_compile_resolvers.go` and moved byte-for-byte only the
contiguous block from `cmdPackageCompileResolvers` through `mergeChildPackagesWalk`, including
`resolverMetaFilename`, comments, and its original trailing declaration separator. The original
file lost only that block; every existing import remains used outside it. The compile dispatcher,
compile-all path, closure compiler, all helpers and tests, names, signatures, APIs, CLI/help and
operation-result bytes, package manifests, cross-platform behavior, and generic engine/package
boundary remain unchanged. The destination is an in-package sibling and introduces no new package,
abstraction, or dependency direction.

The moved declarations reconstructed with their original trailing separator retain SHA-256
`572ecf7a0a42f078566524caf9d852ed1c20c0c19450d5c33099f7b5e8993599`. Reconstructing
`package_compile.go` from the two post-split sources reproduces SHA-256
`e0a987d666f638c659fc29703518d59a5561209220609639795384dae1d7747c`. Each of the eight moved
functions and `resolverMetaFilename` has exactly one definition. The dispatcher and compile-all
caller excerpts retain SHA-256 `7d11929db2c89da088afe80848fddeeace76f46ef52bff1110f0546b60ad6fdd`,
and the closure caller excerpt retains SHA-256
`054b72d3fa0237bd832f63819c05dd80f749cc248ec348fe30f872f0af4567cd`. Reconstructing the sorted
per-file `src` inventory while excluding the new sibling and substituting the reconstructed
original reproduces SHA-256
`deff9fe2ae85fab9f188cfc91f2e0218fbe3eab3c700f20c203e32ff70cd049a`, proving every unrelated
`src` byte unchanged.

Focused offline validation used an existing cached Go 1.25.0 binary and complete module cache with
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`, and isolated `/tmp` build/temp
roots:

- the five exact tests `TestCmdPackageCompileResolversVendorResolversSubdir`,
  `TestCmdPackageCompileResolversEmitsOperationResults`,
  `TestCmdPackageCompileResolverMaterializesAuthoredAptPackages`,
  `TestCompileSingleResolverRebuildsInvalidStoreTarball`, and
  `TestCompileResolverHooksRunInStagingCopy` passed together;
- Linux/amd64 and Windows/amd64 application test packages compiled through non-executing
  `go test -c` commands; the Windows binary was not run;
- `gofmt`, import/declaration ownership, exact block and source reconstruction, caller ownership,
  task-record and unrelated-`src` reconstruction, dirty ownership, `git diff --check`, and the
  protected-cache type/mode/size/hash passed.

The first offline setup selected a valid cached Go 1.25.0 binary but an incomplete module cache and
stopped before compilation because pinned `golang.org/x/sys` bytes were absent. A second existing
complete offline module cache passed `go list -mod=readonly`; the required tests and compile checks
then passed without a network fallback. The setup-only stop changed no repository byte. Generated
validation artifacts exist only under `/tmp/dockpipe-task035-resolver-split.YDV0iE`.

Terminal disposition: `completed`. No successor was selected, created, or implemented. Ranks 2-5
remain non-blocking deferred findings, and any later application-structure successor requires a
fresh bounded audit and exact separate approval after the next material feature cycle.


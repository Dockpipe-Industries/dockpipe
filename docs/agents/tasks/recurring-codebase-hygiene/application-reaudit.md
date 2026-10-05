## Application Structure Re-audit and Next-Slice Selection — 2026-08-15

Completed only the approved source-read-only re-audit of the current application package after the
SDK scope-projection split. Before this documentation edit, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 58 default-collapsed untracked paths, and status SHA-256
`5e8f318e6f1b291e022bf78f2fe2ed0eb616b71fb19940a36d27f26a88752176` matched the
delegated anchors exactly. This task record was
`e9525c3dda865aa50dd6a923629fe56347b2aa4dd9dab4ccefc9afab90533377`; the aggregate digest
of the complete sorted per-file SHA-256 inventory below `src/` was
`801fd0f559acef3647f733d753fc0b6d03997ce2ddcf95a8f43e627b3596a2d3`. The completed
`sdk_cmd.go` and `sdk_scope.go` split sources matched SHA-256
`ede61864e99780631d1e3f870498d0a791209044cc5a5f4c578e3fbae505b649` and
`c527462fe61e17d25f87b9abb3afc963346f6c5a3efc133782d75c9e40a976b7`. The protected
ignored Python cache remained a regular file at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

### Current production, declaration, test, and boundary inventory

Counts use physical Go lines and include tracked, dirty, and untracked Go files. The direct
application package now has 75 production files/19,234 lines and 59 test files/12,512 lines with
339 test functions. Its two internal leaves add two production files/455 lines and three test
files/523 lines with 36 tests, for a recursive total of 77 production files/19,689 lines and 62
test files/13,035 lines with 375 tests. Recursive production declarations comprise 622 functions,
35 types, and 75 top-level `const`/`var` declarations. Tests remain colocated; there is no generic
test directory. The pre-existing generated `src/lib/application/bin/` tree is not Go source or
structure evidence and was left untouched.

This is the complete current production-file size inventory:

| Physical lines | Production files, including internal leaves |
| ---: | --- |
| 400+ | `run.go` 1,143; `run_steps.go` 989; `package_compile.go` 869; `catalog_inputs.go` 680; `session_cmd.go` 640; `runtime_policy.go` 483; `project_build_cmd.go` 470; `package_compile_closure.go` 467; `dependencies.go` 451; `package_compile_validate.go` 448; `package_compile_resolvers.go` 445; `catalog_cmd.go` 405 |
| 300–399 | `subcmds.go` 397; `windows.go` 384; `pipelang_cmd.go` 379; `package_build.go` 373; `usage.go` 370; `sdk_cmd.go` 367; `package_compile_image_selection.go` 365; `sdk_scope.go` 364; `run_image_artifact.go` 357; `pipelang_materialize.go` 349; `package_compile_core.go` 321; `package_compile_security_policy.go` 317; `internal/wslbridge/translate.go` 314; `package_images_cmd.go` 312 |
| 200–299 | `run_steps_parallel.go` 290; `package_script_hooks.go` 289; `package_cmd.go` 284; `flags.go` 283; `init_workflow.go` 280; `workflow_inputs.go` 274; `terraform_cmd.go` 260; `workflow_env.go` 253; `release.go` 247; `workflow_op_inject.go` 244; `package_compile_runtime_artifacts.go` 234; `install.go` 233; `internal_state_cmd.go` 221; `workflow_test_cmd.go` 218; `run_steps_container.go` 205; `state_env.go` 204; `doctor.go` 200 |
| 100–199 | `cmd_runs.go` 191; `project_build_images.go` 189; `strategy.go` 159; `clone_cmd.go` 158; `package_compile_derived_image.go` 153; `run_git_session.go` 152; `internal/imageartifact/manifest.go` 141; `result_cmd.go` 139; `package_test_cmd.go` 126; `resolver_workflow.go` 115; `workflow_cmd.go` 114; `windows_bridge.go` 104; `runtime.go` 103; `host_bash.go` 101 |
| Under 100 | `resolver_docker_env.go` 98; `image_selection_runtime.go` 97; `test_cmd.go` 97; `package_build_source.go` 79; `container_config.go` 75; `init_gitignore.go` 75; `script_context.go` 70; `compile_config.go` 62; `core_cmd.go` 59; `capability_resolve.go` 55; `run_policy_record.go` 51; `host_commit.go` 43; `image_artifact_fingerprint.go` 34; `worktree_docker_env.go` 33; `prompt_safety.go` 29; `workflow_log.go` 27; `event_log_env.go` 24; `host_spinner_msg.go` 23; `policy_docker_env.go` 20; `package_version.go` 15 |

Application remains the fan-in orchestration layer. Across recursive production files, 47 import
Domain (46 direct plus `internal/imageartifact`), 57 import root Infrastructure, 13 import
`infrastructure/packagebuild`, one imports `fetchinstall`, and three import PipeLang. Neither
internal leaf imports its parent: `imageartifact` imports only Domain within DockPipe, while
`wslbridge` imports no DockPipe package. Root application imports each leaf from exactly one owner,
`image_artifact_fingerprint.go` and `windows_bridge.go`. Offline Go 1.25.0
`go list -mod=readonly` resolved application and both leaves, Domain, Infrastructure and both of
its child packages, and PipeLang without a network fallback or import cycle. The completed SDK
split added only an in-package sibling and its expected Infrastructure import; it changed no
dependency direction or package boundary.

### Closed findings and source-backed remaining ranking

The prior resolver-compilation, parallel run-step, and SDK scope-projection findings are closed by
the present 445-line `package_compile_resolvers.go`, 290-line `run_steps_parallel.go`, and 364-line
`sdk_scope.go`. Their parents are now 869, 989, and 367 lines respectively. The earlier catalog,
Domain validation, container preparation, Git-session helper, and internal-leaf findings also
remain closed; none is a current successor merely because file counts grew.

Only rank 1 is selected. Ranks 2–5 are non-blocking deferred findings, not additional successors.

| Rank | Bounded current finding | Callers and direct proof | Disposition |
| ---: | --- | --- | --- |
| 1 | **Single-workflow package compilation.** The contiguous block from `cmdPackageCompileWorkflow` through `runWorkflowCompileHooks` in `package_compile.go` owns CLI parsing plus one workflow's validation, freshness/rebuild decision, staging, hooks, PipeLang/runtime artifacts, manifest, and tarball output. | The three declarations occupy lines 82–328 (247 lines), SHA-256 `ff02ffb88b02b3c693260e5cd7a25525241c37fa21ae6ff4441a1dbd82e58ef3`. Dispatch calls the command; batch, dependency-closure, and PipeLang materialization call `compileWorkflowOne`. Thirteen direct workflow-compile tests cover the command, result events, staging hooks, invalid-store rebuild, image/runtime/security artifacts, package image metadata, APT materialization, allowlists, and proxy enforcement. | **Selected.** It is the largest low-coupling contiguous application seam left, has strong direct proof, and moves no shared helper, caller, test, package boundary, or behavior. |
| 2 | **Batch workflow discovery and stale-tarball pruning.** `cmdPackageCompileWorkflowsBatch` through `pruneStaleWorkflowTarballs` is a 166-line/two-function block, SHA-256 `57b18c6444ff80af47cdac41af5c44ce98e5c9025fa4d9639d0158d1da2c0386`. | The `bundles` alias, `workflows` dispatch, and compile-all path call it; two direct tests cover YAML/PipeLang discovery and stale pruning. | **Deferred.** It is smaller, has less direct proof, and couples multi-root discovery, duplicate handling, two source formats, result accounting, and explicit prune behavior. |
| 3 | **Session worker lease commands.** `cmdSessionWorkerAcquire` and `cmdSessionWorkerRelease` form a visible 170-line block in the 640-line `session_cmd.go`. | Only `cmdSession` dispatch calls them; current session tests directly cover list/inspect/switch, checkpoint, and publish, but not worker acquire/release. | **Deferred.** Runtime-owned Git authority, lease, heartbeat, and release semantics need a fresh focused-test boundary before any move. |
| 4 | **Remaining run-step orchestration.** The 989-line `run_steps.go` still combines blocking execution, resolver/host/package-workflow dispatch, env/scopes, host builtins, and pre-scripts. | Numerous direct tests exercise these seams, but `runBlockingStep` and shared option/env helpers cross them. | **Deferred as coupled.** The clean parallel block is already gone; no new byte-preserving responsibility boundary is presently established. |
| 5 | **Large coordinators and cohesive siblings.** The 1,143-line `Run` coordinator remains high-coupling, while catalog inputs, package closure/validation/resolver files, dependency checks, and runtime policy each retain one evidenced responsibility. | Coverage is broad or responsibility-local, without a narrow new caller/import seam. | **No size-only split.** Re-audit after material feature growth instead of creating a cosmetic folder or abstraction. |

Infrastructure controlled-checkpoint persistence remains outside this application-only re-audit. It
is still part of one security transaction and is not silently promoted into the selected
application successor.

### Exact separately gated successor

Exactly one successor is selected: **TASK-035 single-workflow package compilation responsibility
split**.

- **Result:** create `src/lib/application/package_compile_workflow.go`; move byte-for-byte, in
  original order, only current `package_compile.go` lines 82–328, from
  `cmdPackageCompileWorkflow` through `runWorkflowCompileHooks`, including the two-line
  `compileWorkflowOne` comment and the trailing declaration separator. Remove only that block and
  the now-exclusive `gopkg.in/yaml.v3` import from `package_compile.go`. Keep dispatch, batch,
  closure and PipeLang callers, all tests, usage text, and every shared staging, hook, manifest,
  tar-validation, image-entrypoint, path, operation, runtime-artifact, image-selection, and
  security-policy helper in place. The destination is an in-package sibling, not a new package or
  abstraction.
- **Grounding:** `package_compile.go` is 869 lines at SHA-256
  `03373e9034f34e6f0772270edf16e38d6bc9268c6983b9e08a1bacd16022d085`; the destination is
  absent; the exact 247-line block is SHA-256
  `ff02ffb88b02b3c693260e5cd7a25525241c37fa21ae6ff4441a1dbd82e58ef3`; and the exclusive
  YAML import line is SHA-256
  `9a31bac62ac062ba0fa459b0e216501cece27853cb1db62ebaebf40761ddf1d8`. Production callers
  remain at `package_compile.go` dispatch and batch orchestration,
  `package_compile_closure.go`, and `pipelang_materialize.go`.
- **Behavior and boundary:** preserve every name, signature, declaration body, CLI/help/error/log/
  operation-result byte, workdir/source/name/force parsing, validation order, freshness and legacy
  store behavior, cache invalidation, staging cleanup, compile hooks and environment, PipeLang and
  runtime/security/image artifact materialization, manifest defaulting and namespace selection,
  tarball name/prefix/mode, package-store/path helpers, cross-platform behavior, and generic
  engine/package boundary.
- **Exclusions:** no shared-helper, caller, test, batch/prune, closure, core, resolver,
  all-target, runtime-artifact, image-selection, derived-image, security-policy, validation,
  package-script, schema, configuration, manifest-contract, compatibility, cleanup/migration,
  generated/runtime/durable, live/network/Docker/VM edit; no new package or abstraction; no
  behavior/API/CLI/help/error/log/environment/path byte change; no staging, commit, push, worktree,
  handoff, or additional successor.
- **Required proof:** retain the exact 247-line block and YAML-import hashes; reconstruct the
  original `package_compile.go` byte-for-byte at SHA-256
  `03373e9034f34e6f0772270edf16e38d6bc9268c6983b9e08a1bacd16022d085`; prove the three
  declarations remain unique and the four external production call sites unchanged; run the 13
  direct tests `TestCmdPackageCompileWorkflow`,
  `TestCmdPackageCompileWorkflowEmitsOperationResults`,
  `TestCmdPackageCompileWorkflowRunsCompileHooksInStaging`,
  `TestCmdPackageCompileWorkflowRebuildsInvalidStoreTarball`,
  `TestCmdPackageCompileWorkflowWritesImageArtifactForTemplateBuild`,
  `TestCmdPackageCompileWorkflowMaterializesAuthoredAptPackages`,
  `TestCmdPackageCompileWorkflowWritesPerStepRuntimeArtifacts`,
  `TestCmdPackageCompileWorkflowUsesPackageImageRegistryMetadata`,
  `TestCmdPackageCompileWorkflowStepRuntimeOverridesPackageImageRegistryMetadata`,
  `TestCmdPackageCompileWorkflowUsesWorkflowSecurityNetworkMode`,
  `TestCmdPackageCompileWorkflowPreservesAllowlistRules`,
  `TestCmdPackageCompileWorkflowSupportsProxyNetworkEnforcement`, and
  `TestCompileWorkflowHooksRunInStagingCopy` together; compile the Linux and Windows/amd64
  application test packages without executing the Windows binary; and verify Go formatting,
  import ownership, offline `go list -mod=readonly`, `git diff --check`, task/status/full-`src`
  reconstruction, every unrelated source byte, and protected-cache preservation.

Audit validation used cached Go 1.25.0 with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`,
`GOWORK=off`, a complete existing module cache, and isolated `/tmp` build/temp roots. Structural
file/declaration/test counts, imports, internal-leaf directions, candidate declarations, callers,
tests, destination absence, exact hashes, dirty state, full-`src` inventory, and protected cache
were inspected. No test suite was run because this audit changed no source behavior; the exact
successor owns focused and cross-platform compile proof. Audit cache files exist only under
`/tmp/dockpipe-task035-reaudit-01a007f1`.

Terminal proof removes only this inserted audit section and reconstructs the task preimage exactly
at SHA-256 `e9525c3dda865aa50dd6a923629fe56347b2aa4dd9dab4ccefc9afab90533377`. The TASK-only
no-index comparison is insertion-only and its whitespace check emits no error; repository
`git diff --check` also passes. Final status remains 133 tracked/58 default-collapsed untracked,
none staged, at SHA-256
`5e8f318e6f1b291e022bf78f2fe2ed0eb616b71fb19940a36d27f26a88752176`; the complete sorted
per-file `src` inventory remains
`801fd0f559acef3647f733d753fc0b6d03997ce2ddcf95a8f43e627b3596a2d3`; and both SDK sources
and the protected cache retain every anchored hash, type, mode, and size.

This audit changes no Go source, test, caller, helper, schema, configuration, manifest, API, CLI,
package, workflow, generated, ignored, runtime, durable, or external byte. The generic
engine/package boundary remains intact.

Terminal disposition: `completed`. The selected successor was not implemented, handed off, or
created. It requires the exact separate user response
`Approve next slice: TASK-035 single-workflow package compilation responsibility split` before
any fresh-task execution.


## Application Package Structure Revisit — 2026-08-15

The post-rank-10 feature-cycle revisit confirmed that application-package organization remains an
open TASK-035 hygiene lane. Before the first extraction, `src/lib/application` held 127 Go files and
375 tests in one flat `application` package. Go directories are package boundaries, so ordinary unit
tests must move with cohesive implementation leaves rather than into a generic `tests/` directory.
Large compile and execution surfaces require responsibility splits before any package extraction.

The first bounded slice is complete. Windows/WSL argv, path, mount, environment/variable, URL,
init/create, release-upload, fallback mapping, and bash-forward quoting now live with 35 focused
tests under `src/lib/application/internal/wslbridge`. The root bridge retains environment,
configuration, working-directory, process execution, and exit-code ownership and calls the leaf
through its narrow translator API. The leaf imports only `path/filepath` and `strings` and does not
import its parent package. Focused tests, Linux and Windows/amd64 compile-only checks, formatting,
`git diff --check`, dirty ownership, and the protected ignored-cache check passed. The broader
application run still has unrelated sandbox-sensitive loopback and inherited-path failures; they
were not repaired as part of this extraction.

The second bounded slice is complete. Deterministic build-tree fingerprinting, provenance
normalization, relative build-spec construction, and build-manifest assembly now live with their
focused test under `src/lib/application/internal/imageartifact`. The root retains narrow wrappers
for existing callers and generic JSON serialization; package compilation, run, image-index/cache,
materialization, prebuild, and process ownership remain in `application`. The leaf imports only the
generic domain contract plus Go's standard library and does not import its parent package. Its test
locks schema, kind, state, trimmed provenance and identities, relative build paths, exact source and
artifact fingerprints, security-policy separation, and JSON-visible bytes. Focused leaf and parent
image-artifact tests, Linux and Windows/amd64 compile-only checks, formatting, `git diff --check`,
destination/ownership checks, dirty ownership, and the protected ignored-cache check passed.

The third bounded slice is complete. The contiguous 12-helper compiled security-policy
responsibility, from `normalizeWorkflowPolicyProfile` through `compiledRuleIDs`, moved byte-for-byte
from `package_compile_runtime_artifacts.go` into the in-package sibling
`package_compile_security_policy.go`. Names, signatures, bodies, callers, and existing tests are
unchanged. Runtime-manifest assembly, image selection, APT/derived-image generation, build-manifest,
run/image-index/cache/materialization/prebuild/process ownership, CLI behavior, schema, and package
boundaries remain unchanged.

The three existing focused workflow-security compile tests passed for offline/native,
allowlist/advisory, and restricted/proxy behavior. Linux and Windows/amd64 non-executing application
test-package compiles, Go formatting, exact helper-body comparison, function ownership,
`git diff --check`, dirty-state reconstruction, and protected-byte validation also passed. The first
setup-only test command selected the Go 1.22 launcher under `GOTOOLCHAIN=local` and stopped at the Go
1.25 workspace requirement before compilation; validation then used the cached Go 1.25.0 toolchain
with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and isolated `/tmp` cache/temp roots. No
generated artifact entered the checkout.

The fourth bounded slice is complete. The contiguous derived-image/Dockerfile helper responsibility,
consisting of `aptPackageNamePattern` and the 10 helpers from `normalizeAptPackages` through
`imageRefSlug`, moved byte-for-byte from `package_compile_runtime_artifacts.go` into the in-package
sibling `package_compile_derived_image.go`. Names, signatures, bodies, callers, and existing tests
are unchanged. Image selection, runtime-manifest assembly, registry/provenance/artifact and security
policy logic, build-manifest, run/image-index/cache/materialization/prebuild/process ownership, CLI
behavior, schema, and package boundaries remain unchanged.

The existing `TestCmdPackageCompileWorkflowMaterializesAuthoredAptPackages` test passed, covering APT
normalization and ordering, derived image references, and Dockerfile generation and insertion. Linux
and Windows/amd64 non-executing application test-package compiles, Go formatting, exact block-hash
comparison, declaration/function ownership, `git diff --check`, dirty-state reconstruction, and
protected-byte validation also passed. As in the preceding split, the setup-only commands using the
host Go 1.22 launcher with `GOTOOLCHAIN=local` stopped before compilation at the Go 1.25 workspace
requirement; validation then used the cached Go 1.25.0 toolchain offline with isolated `/tmp`
cache/temp roots. No generated artifact entered the checkout.

The fifth bounded slice is complete. The contiguous 9-helper package-compile image-selection
responsibility, from `stepHasImageSelectionOverride` through `registryExpectedDigest`, moved
byte-for-byte from `package_compile_runtime_artifacts.go` into the in-package sibling
`package_compile_image_selection.go`. Names, signatures, bodies, callers, and existing tests are
unchanged. Image-selection precedence, local-image discovery, provenance, package-image registry
metadata, pull policy, expected digest, fingerprints, artifact fields, runtime manifests,
derived-image/Dockerfile and security-policy logic, CLI behavior, schema, and package boundaries
remain unchanged.

The four existing focused package-compile tests passed for template/local image selection, per-step
runtime artifacts and provenance, package registry metadata and digest, and step override precedence.
Linux and Windows/amd64 non-executing application test-package compiles, Go formatting, exact
helper-block comparison, function ownership, `git diff --check`, dirty-state reconstruction, and
protected-byte validation also passed. The setup-only command using the host Go 1.22 launcher with
`GOTOOLCHAIN=local` stopped before compilation at the Go 1.25 workspace requirement; validation then
used the cached Go 1.25.0 toolchain offline with isolated `/tmp` cache/temp roots. No generated
artifact entered the checkout.

The sixth bounded slice is complete. The two contiguous package-compile image-selection entry
points, `selectCompiledImageArtifact` and `selectCompiledImageArtifactForStep`, moved byte-for-byte
from `package_compile_runtime_artifacts.go` into the existing in-package image-selection sibling.
Names, signatures, bodies, callers, and existing tests are unchanged; runtime-artifact assembly no
longer owns either selection body. Image-selection precedence, local-image discovery, provenance,
registry metadata, pull policy, digest, fingerprints, artifact fields, runtime manifests,
derived-image/Dockerfile and security-policy logic, CLI behavior, schema, and package boundaries
remain unchanged.

The five focused package-compile tests passed for template/local image selection, authored APT
derivation, per-step runtime artifacts, package registry metadata and digest, and step override
precedence. Linux and Windows/amd64 non-executing application test-package compiles, Go formatting,
exact block comparison, reconstruction and function-ownership checks, `git diff --check`, dirty-state
reconstruction, and protected-byte validation also passed. The setup-only command using the host Go
1.22 launcher with `GOTOOLCHAIN=local` stopped before compilation at the Go 1.25 workspace
requirement; validation then used the cached Go 1.25.0 toolchain offline with isolated `/tmp`
cache/temp roots. No generated artifact entered the checkout.

After the two extractions and four in-place splits, the root remains substantial at 127 Go files: 68
production and 59 test files. The application tree has 132 Go files and 375 tests overall; the
increase reflects idiomatic colocated package tests, narrow parent wrappers, and the three new
in-package responsibility siblings, not new CLI behavior. TASK-035 therefore remains open for
repeated, separately authorized structure passes. Current ordering is:

1. **Completed bounded leaf — image-artifact fingerprinting.** The cohesive fingerprint/manifest
   unit and focused test are colocated under `internal/imageartifact`; application-owned wrappers
   and serialization preserve the existing caller and artifact contracts.
2. **Completed in-place responsibility split — compiled security policy.** The 12 security-policy
   helpers are colocated in `package_compile_security_policy.go`; package compilation and all callers
   remain in `application` with unchanged behavior.
3. **Completed in-place responsibility split — derived-image/Dockerfile helpers.** The APT pattern
   and 10 helpers are colocated in `package_compile_derived_image.go`; package compilation and all
   callers remain in `application` with unchanged behavior.
4. **Completed in-place responsibility split — package-compile image selection.** The two selection
   entry points and 9 local-image, provenance, registry-metadata, pull-policy, and digest helpers are
   colocated in `package_compile_image_selection.go`; package compilation and all callers remain in
   `application` with unchanged behavior.
5. **Later coupled surfaces.** Split oversized package-compilation and run/image-artifact files by
   responsibility in place before considering further package boundaries. Each move needs a fresh
   reference/coupling audit and its own behavior-preserving validation slice.

This record does not authorize broad application reorganization, a generic tests directory,
package-compile/run/image-index extraction, CLI or artifact-contract changes, compatibility work,
cleanup, staging, commit, push, worktrees, or unrelated edits.

Terminal disposition for the package-compile image-selection entry-point consolidation: `completed`.
No successor was created; any later responsibility split requires fresh approval.

## Application/Domain/Infrastructure Structure Audit — 2026-08-15

Completed the approved offline structure audit against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`. Before this documentation update the checkout matched
the delegated anchors exactly: 128 tracked dirty paths, 46 default-collapsed untracked paths, no
staged files, status SHA-256
`6dcbcc093d5f1191926d84c6579c27581b9e89326e4b204ecaa85aafaef22720`, and this record at
`29973a38b0e5b245fb950a60df4e8999be8dbbb2ae2e68aa3dce0d7ef76ff335`. The protected ignored
Python cache remained mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`. The aggregate digest of
the sorted per-file SHA-256 inventory for every file below `src/` was
`d975ec0e6d3ce2da048e177236a0fed9c9077d6827e6c779702db5d69d0d2d31`.

### Current size and test locality

Counts use physical Go lines and count `Test`, `Benchmark`, and `Fuzz` declarations as test
functions. Direct means the named package directory only; recursive includes its child packages.

| Package | Scope | Production Go files/lines | Test Go files/lines | Total Go files/lines | Test functions |
| --- | --- | ---: | ---: | ---: | ---: |
| `application` | Direct | 68 / 19,151 | 59 / 12,512 | 127 / 31,663 | 339 |
| `application` | Recursive | 70 / 19,606 | 62 / 13,035 | 132 / 32,641 | 375 |
| `domain` | Direct and recursive | 15 / 2,979 | 13 / 2,428 | 28 / 5,407 | 118 |
| `infrastructure` | Direct | 54 / 13,389 | 46 / 8,213 | 100 / 21,602 | 258 |
| `infrastructure` | Recursive | 61 / 15,009 | 52 / 8,955 | 113 / 23,964 | 275 |

The recursive deltas are intentional package boundaries. Application has
`internal/imageartifact` at 141 production/103 test lines with one focused test and
`internal/wslbridge` at 314/420 lines with 35 focused tests. Infrastructure has `fetchinstall` at
552/247 lines with 10 tests and `packagebuild` at 1,068/495 lines with seven tests. There is no
generic tests directory; tests remain beside their package and can exercise unexported seams.

### Responsibility and coupling map

- **Application** owns CLI/application orchestration for compile, run, catalog, build, install, and
  workflow/session commands. Its production files import `domain` from 41 files, root
  `infrastructure` from 51, `infrastructure/packagebuild` from 11, and `fetchinstall` from one. The
  two internal leaves remain one-way dependencies and do not import their parent. This is the main
  fan-in layer, so package extraction is inappropriate until file-local responsibilities have
  narrow APIs and tests.
- **Domain** owns I/O-free configuration and value contracts. It imports no DockPipe package;
  package-level dependencies are the standard library and YAML. `workflow.go` is the exception in
  size: 1,242 lines, 47 types, and 52 functions combine model, YAML normalization, step helpers,
  parsing, and validation. `domain.Workflow` is referenced from 31 Go files and `domain.Step` from
  14, while the three workflow-focused test files hold 54 tests in 1,136 lines. File splits must
  preserve the single `domain` package and every exported type/function.
- **Infrastructure** owns filesystem, process, Docker, Git, package-store, durable-state, schema,
  and workflow-loading adapters. Root infrastructure imports `domain` from 11 production files and
  `packagebuild` from six. `packagebuild` imports only `domain`; `fetchinstall` imports root
  infrastructure, while root infrastructure does not import `fetchinstall`, preserving an acyclic
  direction. Application imports root infrastructure broadly, so infrastructure splits must retain
  public signatures and OS/security boundaries rather than introduce application knowledge.

The import graph above was resolved offline with the cached Go 1.25.0 toolchain and isolated
`/tmp` cache/temp roots. An initial setup-only `GOTOOLCHAIN=local` invocation selected the host Go
1.22.12 launcher and stopped at the workspace's Go 1.25 requirement before listing packages; it
changed no repository state.

### Ranked bounded split plan

Each row is a separate approval gate. Ranking weighs responsibility reduction, existing test proof,
caller breadth, and invariant risk rather than line count alone.

| Rank | Bounded target and target ownership | Dependencies/callers and existing test proof | Risk and ordering |
| ---: | --- | --- | --- |
| 1 | **Completed — application core compilation.** `package_compile.go` was 1,605 lines/33 functions. Move only `cmdPackageCompileCore`, `runCoreSourceBuildTarget`, `defaultCoreSource`, `seedCompiledCoreFromInstalledTarball`, `latestGlob`, `copyFileWithMode`, and `packageCompileCoreUsageText` to `package_compile_core.go`; leave shared `fileExists` in the original file. | The block depends on existing application compile/config/script helpers, `domain` manifest parsing, infrastructure package paths/operations, `packagebuild` tar writing, and YAML. Callers remain the compile dispatcher, closure compiler, and compile-all path. Three core tests in `package_cmd_test.go` plus `TestCmdPackageCompileAllUsesCanonicalWorkflowRoots` cover source, source-build, skip, and orchestration paths. | **Completed first.** The contiguous responsibility moved byte-for-byte with stable callers and exact reconstruction proof, materially reducing the current largest file without changing behavior. |
| 2 | **Application catalog typed-input/view projection.** Move the PipeLang regexes, type-shape records, typed-input/default/view builders, module parsing, and field-doc/env helpers from the 1,076-line `catalog_cmd.go` to `catalog_inputs.go`; keep command parsing, workflow/resolver/core discovery, icons, and rendering in `catalog_cmd.go`. | Depends only on the existing domain/PipeLang contracts and standard-library parsing/path helpers. Callers are catalog workflow assembly and workflow input resolution. All six tests in `catalog_cmd_test.go` cover typed inputs, external/inferred resolver types, annotations, nesting, and views. | **Low-medium; second.** The approximately 667-line block is cohesive and test-local, with no public CLI or package-boundary change. |
| 3 | **Domain workflow validation.** Move `ValidateWorkflowTypeField` through the host/Compose validation helpers to `workflow_validate.go`; retain model declarations, YAML decode/flattening, step methods, and `ParseWorkflowYAML` in `workflow.go`. | No DockPipe imports. `ValidateLoadedWorkflow` is referenced from application compile/run/steps and infrastructure load/validate paths. The 54 workflow-focused tests plus domain package tests lock parsing and validation. | **Medium; third.** High leverage, but exported validation and schema-adjacent semantics require exact declaration ownership and full domain proof even for a byte-preserving move. |
| 4 | **Infrastructure Git-session storage.** Move list/load/root/sort and JSON/event/receipt persistence helpers from the 1,537-line `git_runtime_session.go` to `git_runtime_session_store.go`; retain session lifecycle, Git/Docker operations, leases, and cleanup in the original file. | Depends on session contracts plus JSON/filesystem/time helpers. Application run/session commands and infrastructure checkpoint/publication paths consume the contracts. Fourteen session tests in 718 lines, especially lifecycle and list/load, plus checkpoint/publication tests exercise persistence. | **Medium-high; fourth.** Metadata and event bytes are durable contracts; require exact JSON and path reconstruction before separating later lifecycle concerns. |
| 5 | **Completed — application step-container preparation.** Move `buildStepContainer`, container path/env mapping, and run-policy/image-provenance helpers from the 1,461-line `run_steps.go` to `run_steps_container.go`; retain scheduling, blocking/parallel execution, resolver delegation, outputs, host builtins, and pre-scripts. | Depends on domain step/runtime-artifact contracts and infrastructure container options. Callers stay in blocking/parallel workers. Twelve dedicated build-container tests and 48 tests across the four run-step files cover the seam. | **Completed fifth.** The seam is identifiable, but it participates in security policy, mounts, compiled manifests, and cross-platform path behavior. |
| 6 | **Completed — application workflow Git-session helpers.** Move the existing create/resolve/finalize/checkpoint/cleanup/session-slug helpers after the 1,019-line `Run` body to `run_git_session.go`; do not decompose `Run` yet. | Depends on domain workflow policy and infrastructure's runtime-owned Git-session API. Callers remain `Run`; run/session tests provide indirect proof. | **Completed sixth.** This reduces mixed ownership without changing coordinator control flow and creates a safer seam for a later, separately audited `Run` decomposition. |
| 7 | **Completed — infrastructure launch presentation.** Banner constants, width selection, print/render, and spinner-width helpers are colocated in `docker_banner.go`; Docker build/run/volume behavior remains in `docker.go`. | Depends on existing terminal/file helpers. Application run and usage remain the two production callers; all three tests in `docker_test.go` pass. | **Completed seventh.** The contiguous EOF block moved byte-for-byte without a CLI text or behavior change. |
| 8 | **Completed — infrastructure durable-state storage primitives.** Private JSON/file/atomic-write/lock and boundary-validation helpers are colocated in `durable_state_storage.go`; public identity/path APIs remain in `durable_state.go`. | Internal callers remain durable project/package resolution, durable import, and public package-state publication. All 16 focused generic/Unix/Windows tests remain compile-covered, and all 15 Linux-applicable tests pass. | **Completed eighth.** A fresh OS-boundary audit preceded the exact EOF move; helper bodies and security behavior are byte-identical. |
| 9 | **Completed — infrastructure durable-import source inventory.** The contiguous read-only legacy source-inventory block is colocated in `durable_import_inventory.go`; validation, locking, publication, manifest validation, and recovery remain in `durable_import.go`. | `PrepareDurableCohortImport` retains five external source/test caller files and relies on unchanged durable-state private primitives. Six focused generic/Unix tests cover selection, durable-wins, unsafe authority, interruption recovery, concurrency, and special files. | **Completed ninth.** The exact block still ends before temporary creation and atomic publication. Publication and recovery remain together because they share pending/authoritative manifests, inventory comparison, rename, and sync boundaries. |

`run.go` itself remains a high-coupling coordinator, and `durable_state.go`/`durable_import.go` remain
large but cohesive. They are not immediate extraction candidates. No new package boundary is
recommended by this audit: application remains the fan-in orchestrator, domain remains dependency
inward and I/O-free, and infrastructure remains the adapter layer.

### Exact first implementation successor

Exactly one successor is selected: **TASK-035 package-compile core responsibility split**.

- **Result:** create `src/lib/application/package_compile_core.go`; move byte-for-byte only the six
  rank-1 functions and `packageCompileCoreUsageText`; remove their original copies and unused
  imports; keep names, signatures, bodies, callers, help text, shared `fileExists`, tests, APIs,
  schema, manifests, dynamic resolution, and behavior unchanged. Update only this task record beside
  those two application source paths.
- **Exclusions:** no resolver/workflow/all compile split, run/catalog/domain/infrastructure split,
  test relocation, new abstraction or package, behavior/API/CLI/help-text/schema/config/manifest
  change, compatibility retirement, cleanup/state migration, generated-state refresh, live action,
  Docker/VM/network use, staging, commit, push, worktree, or further successor.
- **Acceptance:** the moved block and help text reconstruct their exact pre-split bytes and each
  symbol has exactly one definition; caller/reference ownership is unchanged; the three exact
  `TestCmdPackageCompileCore*` tests and
  `TestCmdPackageCompileAllUsesCanonicalWorkflowRoots` pass offline; Linux and Windows/amd64
  application test packages compile without executing the Windows binary; Go formatting,
  function/import ownership, `git diff --check`, dirty-state reconstruction, all unrelated `src`
  bytes, and the protected ignored-cache mode/size/hash pass.

This audit changed no source, test, schema, configuration, package, workflow, generated, or ignored
file. Terminal disposition: `completed`. The selected successor was not implemented or created and
requires the exact user response
`Approve next slice: TASK-035 package-compile core responsibility split` before handoff.


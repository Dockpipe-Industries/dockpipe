## Next Revisit

After the next material feature cycle, refresh sizes and references, close findings that are no
longer current, add newly introduced debt, and re-rank the unresolved findings before proposing any
cleanup. Ranks 1-10 are resolved; the application-package structure lane remains open and must
advance one bounded leaf or responsibility split at a time. This record grants no implementation
authority for any compatibility retirement or core-demo untethering.

## Application Structure Cleanup Objective — 2026-08-15

Objective terminal state: `completed`. The approved objective was to re-audit the current
application package, implement every presently justified low-risk behavior-preserving
responsibility split without returning to approval after each reversible checkpoint, and stop
when the remaining large areas were evidenced deferrals. No one-shot gate was required. No
handoff, compatibility retirement, destructive cleanup or migration, generated-state refresh,
live/network/Docker/VM action, staging, commit, push, worktree, task creation, or successor was
performed.

Before objective mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, origin `js/dev`
`115fb785f0c4dc789ffce8c93c79fe147ea58229`, 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, zero staged paths, 141 tracked dirty paths, 61
default-collapsed untracked paths, and status SHA-256
`ce9061522b9382b5e07fe8c6c131521a7045a145319c6687da0f7c9d408c86cf` matched the admitted
objective anchors. This TASK was SHA-256
`60ad054a2a9f9138ff1a86f0d9b0335e91fd03408d8a3be724d22f7a835bedbf`;
`package_compile.go` was 619 lines at SHA-256
`58a6c99b4534982877a3221085dccba041915df20c6c891c1ca88d5f55c680ff`; and the complete sorted
per-file `src/` inventory was
`417af2d7434c4240f5bae0b25442fbd7eefe34fb0d191b9478178fc49296da40`. The completed
`package_compile_workflow.go` sibling remained 263 lines at SHA-256
`54f8debc2ae81af52556f05b8899c1fba361db099fd65518bc89504612f49ab9`. The protected ignored
Python cache remained a regular mode-`0664`, 8,408-byte file at SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

The objective completed three in-package responsibility checkpoints:

1. Created `package_compile_workflows_batch.go` for batch workflow discovery, duplicate handling,
   compilation accounting, and stale-tarball pruning. The exact moved declaration block remains
   SHA-256 `57b18c6444ff80af47cdac41af5c44ce98e5c9025fa4d9639d0158d1da2c0386`;
   the formatted 179-line sibling is
   `7ac9debcf1c4b624cc5ecdb403ca9351a4c7482497e937378a797a06721eddf1`.
2. Created `package_compile_all.go` for the compile-all coordinator and its argument helper. The
   exact moved declaration block remains SHA-256
   `18ba81edae47b5edd206ae163fd478d3117b0756e1a7c75b04ae461493576d70`;
   the formatted 103-line sibling is
   `5d0971298c2b0ba09f823add99acec08887d60868077e8407bc650f148cc65f9`.
3. Created `package_compile_usage.go` for the seven package-compilation usage constants. The exact
   94-line moved block remains SHA-256
   `15d090b94fa77c1dcf05e11ef603d6658e3e271762536c46c6eb7ac39ce333e9`;
   the formatted 96-line sibling is
   `a4befae7fde34eeebf4c001ef7944376ffdbd3bd1f8a1411564390f131f14e2a`.

The resulting `package_compile.go` is 265 lines at SHA-256
`7ffb830f6a8032b22b47fdbe83d856647a25e8cef6baa50ed3b22cf3d7b8c49d`. Reinserting the three
exact blocks in original order reconstructs its 619-line pre-objective form byte-for-byte at
`58a6c99b4534982877a3221085dccba041915df20c6c891c1ca88d5f55c680ff`. Every moved function and
constant has exactly one production declaration, and the dispatcher, batch, closure, PipeLang,
and build callers remain unchanged. Substituting that reconstructed parent and omitting only the
three new siblings reconstructs the complete pre-objective `src/` inventory exactly at
`417af2d7434c4240f5bae0b25442fbd7eefe34fb0d191b9478178fc49296da40`; the terminal current
inventory is `9dcbd5a020ecaf350c11d8e5e781ece777e5e9833e49aca47307c6851cf5b13d`.

Cached Go 1.25.0 validation used `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`,
the existing complete module cache, and `/tmp` build/temp roots. The four cumulative focused tests
for config.pipe discovery, stale pruning, canonical compile-all roots, and build delegation passed
together (`ok dockpipe/src/lib/application`, 22.469s). The 13 direct single-workflow compilation
tests listed in the preceding responsibility-split record passed together again
(`ok dockpipe/src/lib/application`, 98.422s). Offline `go list -mod=readonly` resolved application,
both application internal leaves, Domain, Infrastructure and both child packages, and PipeLang;
`go vet ./src/lib/application` passed. Linux/amd64 and Windows/amd64 application test packages
compiled with `go test -c`; the Windows binary was not executed. The terminal binaries exist only
under `/tmp/dockpipe-task035-objective-terminal/`, at SHA-256
`f5e588d0562d03bdcae8934c9de876bbd6408f0387a8096fa181183adfd081f6` for Linux and
`d09e272b7fbe0eb5d11a28891513bc7dc0a3ff8929e2063a7aa73d64c70f2c6e` for Windows/amd64.

The broad `go test ./src/lib/application -count=1` ran but remains non-green for two independent,
unowned reasons: `TestCmdInstallCoreEmitsOperationResults` cannot open its loopback listener in the
Codex sandbox, and `TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap` reproduces alone
because inherited durable-state code now resolves that test's intentionally nonexistent
`/path/to/your/project`. Neither failure reaches or depends on the declarations moved by this
objective. The test run appended ignored event records and created five ignored policy files;
terminal cleanup removed only those proven run products. The three affected event logs exactly
regained their pre-run hashes, the five policy paths were absent from the admitted inventory, and
the full pre-objective `src/` reconstruction above passes.

Go 1.25 `gofmt -d` emits no diff for all five owned package-compilation sources, import ownership
is exact, and `git diff --check` passes. The protected cache retains its admitted type, mode, size,
and hash. This remains a behavior-preserving organization change: no API, CLI/help/error/log,
operation-result, environment, path, parsing, freshness/rebuild, legacy-store, cache, staging,
hook, PipeLang, runtime/security/image artifact, manifest, namespace, tarball, package/store,
cross-platform, schema, configuration, or package-boundary contract changed.

The refreshed inventory leaves `run.go` and `run_steps.go` as high-coupling coordinators;
`catalog_inputs.go`, runtime policy, package closure/validation/resolver siblings, and dependency
checks as cohesive responsibilities; session worker leasing without a direct focused-test seam;
and project build/clean/rebuild coupled to destructive path and durable-state safety. No additional
size-only split is justified. The application-structure lane is therefore converged for the
current feature state and should be re-opened only after material feature growth or new focused
proof establishes a real responsibility seam. No successor is selected or preauthorized.

## Application Internal-Package Extraction Objective — 2026-08-15

State: `completed`. User authority created a new multi-checkpoint objective to replace cohesive
flat application implementation with parent-independent `application/internal/<domain>` packages,
starting with the runtime/security-policy dependency needed by package compilation, then extracting
package compilation itself and continuing through further evidenced seams without micro-approval
loops. Ordinary reversible source/test/documentation checkpoints advance automatically.

Done means each selected child has a narrow internal API, imports no parent `application` package,
keeps its tests beside implementation where practical, preserves public CLI and authored behavior,
passes focused and cross-platform compile proof, and leaves remaining flat areas evidenced as
coupled or cohesive rather than merely large. Exclusions remain behavior/API/schema/compatibility
changes, destructive cleanup or migration, generated/durable/live/network/Docker/VM actions,
staging, commits, pushes, worktrees, and external publication.

Admission anchors: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, origin `js/dev`
`115fb785f0c4dc789ffce8c93c79fe147ea58229`, 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, zero staged paths, 141 tracked dirty paths, 64
default-collapsed untracked paths, status SHA-256
`1cfb7613d43d52dace5ea00432231b1c485c9e45f759fe3b88b773802531cb73`, TASK SHA-256
`dc0fc39bf8d79b42210fea8367403a264ff1334eafbaf42ba95081da59cfdec4`, and full sorted per-file
`src/` inventory SHA-256 `9dcbd5a020ecaf350c11d8e5e781ece777e5e9833e49aca47307c6851cf5b13d`.
The protected ignored Python cache is a regular mode-`0664`, 8,408-byte file at SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

### Checkpoint 1 — runtime policy and compile configuration leaves

Created `application/internal/runtimepolicy` as a parent-independent child owning compiled runtime
manifest loading/application, security-policy compilation, enforcement summaries, proxy policy,
and default policy fingerprints. The former flat `runtime_policy.go` and
`package_compile_security_policy.go` implementations moved there; the two compiled-runtime manifest
loaders moved from `run_image_artifact.go`. Five direct proxy/policy tests moved from `run_test.go`
beside the implementation. The child imports Domain, Infrastructure, and packagebuild only and has
no parent Application import. Terminal checkpoint hashes are
`afcf72f94d062c6874b6f6cd5ccbbf40313eb8137bf1e5206b298a06e580caf1` (`runtime.go`),
`95e342fe74eb804132a511e7c1e9844c80764143cf78bc9ca2c2c03c88aab30c` (`security.go`), and
`0391b9e18ad1a101e22a29cc322cdb9b4624dbf4401bdaa010ed3d3de692e4e7` (`runtime_test.go`).

Created `application/internal/compileconfig` as a parent-independent child owning config loading,
core-source selection, and canonical workflow/resolver compile-root resolution. Three direct root
selection and missing-path tests moved beside it; the compile-all integration test remains in the
parent. The child imports Domain and Infrastructure only and has no parent Application import.
Terminal hashes are `b32e8f9dd38d91958f71a428ddbee5a84805e65ce8eb0a8f79ce6cddab844f27`
(`config.go`) and `f1406058e19ec95095e8f7271545c53523f200e350e7d1bc4074e2c1416dcb19`
(`config_test.go`). Parent callers now depend downward on the two internal packages; behavior and
wire/CLI text are unchanged.

Focused validation passed: runtimepolicy's five direct tests; compileconfig's three direct tests;
the parent runtime-policy integration and workflow security compilation tests; compile-all,
doctor, PipeLang, package-test, and workflow-test focused coverage. Offline `go list -mod=readonly`
resolved Application plus all four internal leaves, Domain, Infrastructure children, and PipeLang.
Compile-only package checks passed for the parent and both new children. Linux and Windows/amd64
application test binaries compiled without executing Windows; their `/tmp`-only hashes are
`000bb26c8fe8be7e9739ae808cd91555d63891048c55ca877cf797a529087e34` and
`cc23583f476634b153d21719c9908967a726a0e4342a6f10aefa77092a71eb07`.
`git diff --check` passes, no generated checkout file changed, and the protected cache remains exact.

The active objective now transports once under `context_handoff_policy:
automatic_at_safe_boundary`. Soft signals are accumulated anchor/tool output dominating the next
checkpoint, the pending package-compilation extraction requiring a materially different file/API
context, and the completed state being clearer as this compact durable checkpoint. Live pre-handoff
state is HEAD/stash/upstream unchanged, zero staged paths, 151 tracked dirty paths, 63
default-collapsed untracked paths, status SHA-256
`caf0e2c5e86df7f411017904fea606fdad5ea812fc2467706cd12149e311cb63`, TASK pre-handoff SHA-256
`eb5df3aded60bb6099102ac2866e6809dbe16f40ec6012ce76f2d5297d4b7beb`, and full sorted per-file
`src/` inventory SHA-256 `01b8bd9c32ee03b6aa35c0b612763ec2f32063542cbba154ccb29056e4350289`.
Pending checkpoint: extract `application/internal/packagecompile` on top of compileconfig and
runtimepolicy, retain thin parent adapters, move direct tests beside the child, validate, then
continue the same objective into further evidenced domains. This is transport only, not a successor
or new authority.

### Checkpoint 2 — package compilation and supporting leaves (terminal)

Created `application/internal/packagecompile` as the parent-independent owner of package compile
dispatch, full and dependency-scoped compilation, core/resolver/workflow materialization, PipeLang
workflow staging, runtime/image artifact emission, output validation, and the unchanged compile help
surface. The child is 16 files with tree SHA-256
`9cfa6d19ad737074b7ac22364bc5f653c70d4e0316d7c3e933d065b2df78c9b0`; ten direct closure,
validation, and staging-hook tests are colocated. The 88-line parent adapter retains
`cmdPackageCompile`, `CompileClosureForWorkflow`, `CompileWorkflow`,
`ValidateOutputsForMode`, integration-test compatibility, and the explicit parent-owned callback
that runs a core source-build script with application artifact/state bindings. Its SHA-256 is
`72f7a5e4f1e661c7954580f253b5f5d923b5bf9e2763c5efe668a69a6416bd67`. No child imports the
parent application package.

Cross-domain compile dependencies became narrow downward leaves rather than upward imports:
`operationids` owns stable operation-result identifier construction, `packageversion` owns authored
VERSION fallback, `packagescript` owns portable script command/env/path construction,
`pipelangmaterialize` owns PipeLang discovery and materialization, `textvalue` owns first-non-empty
normalization, and `treecopy` owns ordinary/core source copying and Python-cache exclusion. Their
tree SHA-256 values are respectively
`e0f3a9c52f8a5955fa957bb698e9b5678006ac669a0b07530ef12fd87e850244`,
`59c35f254527eeda6d7d1e28929964f67ed669d7c53dde8defbd13702d016ea5`,
`b56a9237212838032bc6c1a8f77fe434202c26f0846238df6d0b85776929ccd0`,
`9300b6cef2ebf84aabed5444be2c92a28f973f07a197ecc7fb457a3c65b5a04c`,
`be6b262060280160da0ca1e84257902253b8155254bec14ea451b28630d98651`, and
`6263b343b04451e1afe716b5efccfd7fe9b7232d3e19f6f0c9325419e06181b9`. Ten additional direct
tests are colocated across these leaves. Parent CLI/orchestration compatibility adapters preserve
the prior function and byte surfaces.

Terminal focused proof passed with cached Go 1.25 and `GOTOOLCHAIN=local`, `GOPROXY=off`,
`GOSUMDB=off`, and `GOWORK=off`: every application internal leaf test; the parent package compile,
resolver, closure, PipeLang materialization, compile-all, and build-delegation tests; offline
`go list -mod=readonly`; and `go vet` for application, all internal leaves, Domain, Infrastructure
children, and PipeLang. A broad parent run reached only the two inherited pre-objective failures:
the sandbox-prohibited loopback listener in `TestCmdInstallCoreEmitsOperationResults` and the
intentionally nonexistent inherited workdir in
`TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap`. Linux and Windows/amd64 compile-only
test binaries passed without executing Windows; SHA-256 values are
`49ed7e56fa51540af28294f546d7052fa103ff77344ad58cf9b7b073ec65539b` and
`9cfc088e914b9ab7b5a225937e6dc998fcd2157593530a9ba9ffc1d7578b0402` for Application, and
`d5ecdd58b9a5cce8b409998435adf1da483f652709ce8f5f7e2707bef2914890` and
`86e11486aed4d06ab98418e3d8786a86a1fc28172e24d2453a7e7d841ac31f7a` for packagecompile.
`gofmt` and `git diff --check` pass.

The remaining large flat files are evidenced as cohesive or parent-coupled rather than merely
large: `run.go` is one sequential public orchestration function and `run_steps*.go` shares the
private `runStepsOpts` state machine; `catalog_inputs.go` constructs the parent-owned catalog wire
records declared by `catalog_cmd.go`; `session_cmd.go`, `dependencies.go`, and
`project_build_cmd.go` each bind guarded CLI lifecycle, approval, or removal semantics directly to
parent command state; and `pipelang_cmd.go`, `package_build.go`, and `sdk_cmd.go` are now thin command
orchestration over the extracted lower leaves and Infrastructure. Further movement would either
export parent wire/state types or split one cohesive lifecycle, so no additional child is selected.

Terminal live anchors remain branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, origin `js/dev`
`115fb785f0c4dc789ffce8c93c79fe147ea58229`, 0 behind/1 ahead, and `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, with zero staged paths. Before this terminal record,
status was 155 tracked dirty and 55 default-collapsed untracked paths at SHA-256
`9710379b509fd945f9693e1d276224672a016c083d06774b361f0402dd00e60b`; the full sorted per-file
`src/` inventory SHA-256 is `10856d20aabb55538e79d5879ef85ed99eaf9c9afde11c654cd6ad2858c01817`.
No checkout generated path changed. The protected ignored Python cache remains regular mode-`0664`,
8,408 bytes, SHA-256 `25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.
The objective is complete; no gate, second transport, staging, commit, push, publication, cleanup,
migration, or compatibility authority was used.

## Domain Vocabulary Cleanup — 2026-08-15

State: `completed`. After the application-internal extraction and its reviewed commit series were
pushed by the user, the user requested another cleanup pass focused on DDD, models, constants,
enums, and class-like ownership. The admitted checkpoint was limited to a source-compatible domain
vocabulary cleanup: preserve all exported Go struct field types and authored JSON/YAML/CLI bytes,
introduce value types only for closed domain vocabularies, replace production decision literals
with named constants, and keep behavior beside its owning package. No generic `Models` directory
was created because `src/lib/domain` already owns DockPipe's model and validation layer; Go value
types and package functions are the idiomatic equivalent of enum/model classes here.

Created `src/lib/domain/runtime_policy_values.go` as the owner of ten closed vocabularies:
`PolicyProfile`, `NetworkMode`, `NetworkEnforcement`, `FilesystemRootPolicy`,
`FilesystemWritePolicy`, `ProcessUserPolicy`, `ImageSource`, `ImageAutoBuildMode`,
`ImagePullPolicy`, and `ImageArtifactState`. Validation now converts exported string wire fields at
the Domain boundary and checks typed allowed-value sets. Runtime policy compilation, image
selection, image artifact materialization, and package image reporting branch on the domain values
and use their named constants. `RuntimeKind` is now a value type with normalization and validity
behavior while its existing constants, `ValidRuntimeKinds` string slice, unknown-value preservation,
and `ResolverAssignments.RuntimeKind` string field remain source-compatible.

Direct Domain tests prove the constant-to-wire mapping, known and unknown runtime-kind behavior,
trimmed-value validation, non-mutation of authored values, and compile-time preservation of every
exported string field touched by the cleanup. Focused tests passed for Domain, runtimepolicy,
packagecompile, imageartifact, package image collection, planned/materialized image handling,
registry pulls, runtime policy application, and step security overrides. Offline Go 1.25
`go list -mod=readonly` and `go vet` passed for Domain, Application, and the three affected internal
packages. Linux and Windows/amd64 compile-only proofs passed for Domain and Application; Windows
binaries were not executed. The four `/tmp` verification binaries, including two unexpectedly
large Application test binaries, were removed immediately after successful compile proof.

`gofmt`, `git diff --check`, downward dependency checks, generated-checkout inventory, and protected
cache verification pass. No schema, authored workflow, public help/error/log bytes, generated
checkout artifacts, durable state, network, Docker, VM, staging, commit, push, publication, or
compatibility behavior changed. Remaining obvious closed vocabularies—workspace lifecycle/storage
and container mount modes—are a separate authored-runtime concern and were not folded into this
checkpoint without their own focused dependency and behavior review.

## Authored Runtime Vocabulary Cleanup — 2026-08-15

State: `completed`. The approved continuation first inventoried workspace lifecycle/storage and
container mount modes without changing authored or runtime behavior. `workspace.mode`,
`workspace.storage`, `workspace.lifecycle.checkpoint`, and `workspace.lifecycle.publish` are closed
authored/runtime vocabularies shared by Domain validation, Application orchestration, and generic
Git-session Infrastructure. Repository and branch values remain open-ended identifiers. Authored
`container.mounts[].mode` is the closed `ro`/`rw` Domain vocabulary. Resolver
`auth-mount-mode` remains string-based: it is an unvalidated resolver/profile protocol value and
the downstream Docker/WSL mount path intentionally recognizes additional option suffixes, so the
shared spelling alone is not evidence that it has the authored mount vocabulary.

### Checkpoint 1 — authored container mount mode

Created `src/lib/domain/workflow_runtime_values.go` as the owner of `ContainerMountMode` and the
named `ContainerMountReadOnly` / `ContainerMountReadWrite` constants. Domain validation and
Application mount-spec construction now make typed internal decisions against that owner while
`WorkflowContainerMount.Mode` remains `string`; YAML values, trimming behavior, validation errors,
and emitted Docker `host:guest:mode` bytes are unchanged. Direct Domain tests prove constant/wire
mapping, exported-field source compatibility, trimmed-value acceptance, and non-mutation of the
authored field. Focused Domain parse/validation and Application container merge/resolution tests
pass offline with cached Go 1.25.0.

### Checkpoint 2 — workspace lifecycle and storage values

The same Domain owner now defines `WorkspaceMode`, `WorkspaceStorage`,
`WorkspaceStorageBackend`, `WorkspaceCheckpointMode`, and `WorkspacePublishMode` with named
constants for the existing wire values. Domain validation, Application checkpoint/volume handling,
and Infrastructure session creation, defaulting, reuse, and cleanup now branch on typed values.
The authored storage set intentionally excludes explicit `bind` exactly as before, while the
runtime storage type retains `bind` for the mode-derived backend. Case-insensitive recognition of
stored `docker_volume` metadata is preserved through Domain normalization.

All exported workflow, Git-session request, JSON metadata, and policy fields remain `string`.
Direct tests prove constant/wire mappings, string-field source compatibility, trimmed authored
validation without mutation, and stored-backend normalization. Full Domain tests and focused
Application and Infrastructure workspace/container/session tests pass. Offline Go 1.25.0
`go list -mod=readonly` and `go vet` pass for Domain, Application, and Infrastructure. Linux and
Windows/amd64 compile-only test binaries for all three packages pass; Windows binaries were not
executed.

No further vocabulary is selected in this bounded concern. Repository IDs, paths, branch/session
names, environment cleanup-policy tokens, operation/session/volume statuses, worker lease modes,
resolver auth mount options, Docker CLI words, and Git command arguments are respectively
open-ended, separately owned, presentation/protocol-required, or coupled values rather than the
authored workspace/container vocabularies. No schema, authored surface, public help/error/log/
operation-result/environment/path bytes, generated checkout state, durable state, behavior,
network, Docker, VM, staging, commit, push, publication, cleanup, migration, compatibility, or
external action changed.

## Remaining Domain Vocabulary Overhaul — 2026-08-16

State: `completed`. The bounded audit excludes the completed runtime-policy, image,
runtime-kind, workspace, and container-mount owners. It ranks the remaining source-compatible
closed vocabularies as follows:

1. `packages.sources[].kind` is selected first because the same trim, case-fold, empty-to-`store`
   default, and `store`/`tarball_dir` decision was independently repeated in Domain validation,
   Infrastructure package-source resolution, and Application compile-output validation.
2. Workflow step execution values (`kind`, cwd/scopes, and `host_builtin`) are directly evidenced
   across Domain validation/defaulting and Application execution, but form a materially different
   authored-workflow checkpoint.
3. `package_state.compatibility_import` is closed and cross-layer, but its consumer is the
   separately coupled durable compatibility-import lifecycle.
4. Host platform values are closed at validation, but effective platform and installer selection
   are coupled to GOOS and Linux distribution detection.
5. Vault tokens include compatibility aliases and feed the separately owned secret-injection
   protocol. Async group mode is a single parser protocol token. Package kinds/distribution are
   package/build protocol values without one currently validated closed set. Operation statuses
   already have an Infrastructure owner; Git session/lease and durable-import statuses are persisted
   lifecycle protocols. Repository, branch, path, environment, operation ID, CLI argument, image
   reference, resolver/profile, and user-supplied status/detail strings remain open-ended. Package
   image source/pull decisions remain with the completed image vocabulary and are not reopened
   without a direct failure or dependency.

### Checkpoint 1 — explicit package source kind

Created `src/lib/domain/project_config_values.go` as the sole owner of the
`PackageSourceKind` value type, the existing source-compatible untyped `store` and `tarball_dir`
constants, normalization/defaulting, and validity behavior. `DockpipePackageSourceConfig.Kind`
remains `string`, validation does not mutate authored JSON, and unknown normalized values remain
available for the unchanged validation error. Infrastructure now retains the typed kind through
deduplication and store/tarball routing; Application compile validation switches on the same Domain
value. Direct Domain tests prove constant/wire mapping, exported string-field compatibility,
case/trim/default behavior, unknown-value preservation, and non-mutation.

Focused cached-offline Go 1.25.0 proof passes: full Domain tests, the two configured external-store
Infrastructure tests, and Application's configured-store dependency-closure test. Domain
`go list -mod=readonly` and `go vet` pass against the pinned dependency graph. The saved offline
module cache no longer contains the pinned `golang.org/x/sys v0.28.0` source required to recompile
Infrastructure; affected Infrastructure/Application list, vet, focused tests, and Linux plus
Windows/amd64 compile-only checks therefore used a `/tmp`-only modfile selecting the available
cached `v0.47.0`, with network disabled and no checkout dependency-file mutation. Linux/Windows
binary hashes are respectively `11f1108955653a8c11addc87b7676ef076528bd50079adbf598ea057bdfc47f4` /
`0663d2b30a0afc172a8cc1c856e0753b6d5d519154911442a8cdb2e84ea0b42c` for Domain,
`b4dcaa7e13ae39aaba534114edddeefbeae29905e601b6566d1e900bebfb4f29` /
`68e42ac257af9e8b246e3fac00c2d41e55364360c0c742abdda5dce9c1384188` for Infrastructure, and
`daf88653eae60e04b3cb6e108d0cf2bce45dd68c9b5b3289d281ad3bcc8dfbd6` /
`c2e54924663ed1811c7896a35d13263a4aeccb4f2e7d63f6868a4d1c83c6a5d4` for packagecompile. The
six `/tmp` binaries and temporary modfiles were removed after hashing. `gofmt` and
`git diff --check` pass. Integrity read-back remains on branch `js/dev`, HEAD
`0972303d4745474f50b4adcab3b2f5d058f92f61`, origin `js/dev`
`3fe03013e695b7b13b62221bd7ccbc6ad4334e00`, 0 behind/1 ahead, both inherited stash objects
unchanged, and zero staged paths. Offline module resolution created or refreshed two ignored,
empty mode-`0664` Go cache lock files under `bin/.dockpipe/tmp`; each remains 0 bytes at SHA-256
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`. No generated payload
bytes changed. Future proof must copy any needed module cache into `/tmp` before use so the checkout
cache is not touched again. The protected Python cache remains exact.

At this safe boundary, accumulated audit/verification evidence dominates the task and the ranked
workflow-step checkpoint needs materially different parsing/execution context. Those two soft
signals trigger the objective's authorized transport-only continuation. Pending checkpoint: admit
this proof, revalidate only affected anchors, then implement the ranked workflow-step vocabulary
without widening authored behavior.

### Checkpoint 2 — workflow step execution values (terminal)

Created `src/lib/domain/workflow_step_values.go` as the owner of `StepKind`, `StepPathScope`, and
`StepHostBuiltin`. It owns the existing container default; case/trim normalization and unknown
preservation for step kinds; source/artifact defaults and the `repo`, `workdir`, and `artifact`
aliases; the four supported host builtins; compose/Docker requirements; and unchanged compose
action suffixes. `Step.Kind`, `Step.CWD`, `Step.Scopes.Source`, `Step.Scopes.Artifacts`, and
`Step.HostBuiltin` remain exported `string` fields. Existing `KindName`, `CWDMode`,
`SourceScopeMode`, and `ArtifactsScopeMode` methods retain their `string` results and exact wire
bytes while new typed effective-value methods serve Domain and Application decisions.

Domain validation now checks the owner values without mutating authored YAML and retains the exact
kind, cwd, scope, host-builtin, and missing-compose error strings. Workflow Docker preflight uses
the typed builtin requirement. Application step cwd/scope selection, artifact-root creation,
host-builtin dispatch, compose dispatch, and autodown branching use the same typed values; string
conversion remains only at the existing environment, logging, error, and compose-action output
boundaries. Direct owner tests prove constant/wire mapping, exported-field and helper signature
compatibility, defaults, aliases, case behavior, unknown preservation, compose action bytes,
non-mutation, and exact unknown-value errors. The new owner and direct test SHA-256 values are
`1d3b34b34e79f1cef9eb975f8bb4e47ee5b63ef33bd88ca6fe841d8f892de49b` and
`c38677e5f10f48b73caf8525cb34b675ac12d71f588fb8da22864e60815c88d4`.

Cached-offline Go 1.25.0 proof passes for the full Domain package and six focused Application step
kind/cwd/scope/compose tests. Domain `go list -mod=readonly` and `go vet` pass against the checkout
module graph. The offline caches now also lack `github.com/mattn/go-shellwords v1.0.12` source and
zip, in addition to pinned `golang.org/x/sys v0.28.0`. Application tests, list, vet, and compile
therefore used a `/tmp`-only modfile selecting cached `x/sys v0.47.0` and a signature-compatible
`go-shellwords.Parse` compile stub; the focused tests do not call that parser. This proves the
affected Application type and execution paths but is not a production shell-parsing dependency
proof. No checkout dependency file or checkout module cache was changed. Linux and Windows/amd64
compile-only test binaries passed without executing Windows; SHA-256 values were
`97903741e07eeaf2bd02e71c34c47f2b57e5cb601fe2b279a60eb0aa307b6171` /
`ba47bb4ffff1e74b2ab6c30bab01778b9ffc1bd5598072f36fb7e46c033f7251` for Domain and
`24ac2bc1269e57839ce8bf80efb2045a3b360b8eaf0db111fae37f89460a2dc5` /
`a8f9b87faf4922211e1ff2b7529f430fc9838f61f2a27b5abf743ece49989986` for Application.

`gofmt`, `git diff --check`, literal-decision ownership, dependency direction, generated-state,
and protected-cache checks pass. The two inherited empty mode-`0664` checkout cache lock files
remain byte-identical and no generated payload changed. The temporary verification tree was
kept outside the checkout at `/tmp/dockpipe-task035-step-vocabulary/` because cleanup is excluded.
Public Go fields and authored JSON/YAML/CLI/help/error/log/operation-result/environment/path bytes
remain compatible; unknown values retain their prior validation behavior.

The bounded selected set is complete. `package_state.compatibility_import`, host platform values,
vault aliases, async group mode, package kinds/distribution, operation and session statuses, and
the remaining open-ended identifiers stay classified exactly as ranked above: they are coupled
lifecycle/protocol concerns, already owned values, or open-ended data rather than another directly
evidenced source-compatible seam. No completed vocabulary was reopened. No schema, authored
surface, behavior, durable/generated/live state, network, Docker, VM, cleanup, migration,
compatibility retirement, staging, commit, push, publication, worktree, external action, gate, or
transport authority was used by this receiver.


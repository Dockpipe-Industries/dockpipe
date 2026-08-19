## Model/Domain Boundary — 2026-08-16

State: `completed`. Objective `TASK-035-model-domain-boundary` establishes a sibling
`src/lib/model` layer for authored and generated wire structures while preserving the root Domain
API. The completed Domain-vocabulary proof above is admitted unchanged and is not reopened.

### Ownership audit and bounded set

The live Domain production package has 20 Go files. It imports no Application or Infrastructure
package, while Application and Infrastructure consume its public types broadly. Its contents fall
into five ownership classes:

| Class | Current files | Decision |
| --- | --- | --- |
| Authored/shared dependency shape | `dependencies.go` | **Selected:** `model/dependency` owns the three YAML structures; Domain retains cross-shape validation and public aliases. |
| Package manifest wire shape | `package_manifest.go` | **Selected:** `model/package` will own manifest and nested YAML structures and depend only on `model/dependency`; Domain retains normalization/validation and the legacy parsing facade. |
| Project config wire shape | `project_config.go` | **Selected:** `model/project` will own JSON structures and the custom `compile` decoder required by Go method ownership; Domain retains validation, defaults, path policy, and compatibility facades. |
| Compiled runtime/image payloads | `runtime_artifact.go` | **Selected:** `model/runtimeartifact` will own JSON/YAML payload structures and wire-name constants; Domain retains cross-model policy validation and compatibility wrappers. |
| Workflow graph | `workflow.go`, `workflow_inject.go` | **Unselected:** the large mutually referential shape has custom YAML methods and public methods coupled to Domain value types. Moving it now would either invert the chosen dependency or create a cycle. |
| Domain policy/value semantics | `runtime_policy_values.go`, `runtime_kind.go`, `workflow_runtime_values.go`, `workflow_step_values.go`, `project_config_values.go`, `workflow_validate.go`, `vault_mode.go`, `namespace.go`, `env.go`, `branchslug.go` | **Unselected:** these are named values, normalization, validation, or domain rules rather than data-shape ownership. |
| Parsing/merge behavior | `workflow_imports.go` | **Unselected:** import expansion and merge policy operate across workflow documents and remain Domain behavior. |
| Runtime/profile projections | `resolver.go`, `strategy.go` | **Unselected:** these are normalized environment-assignment projections without authored serialization contracts. |
| Filesystem/path result and compatibility I/O | `compile_roots.go`, the loader/path functions in `project_config.go`, and path parsing in `package_manifest.go` | **Unselected:** moving filesystem behavior belongs to a separately bounded Infrastructure/API design; it is not smuggled into Model or changed under this objective. |

Three sibling layouts were compared. One flat `model` package was rejected as the prohibited
generic bucket. Serialization-format packages such as `model/yaml` and `model/json` were rejected
because they mix unrelated responsibilities. Cohesive responsibility packages were selected:
`model/dependency`, `model/package`, `model/project`, and `model/runtimeartifact`.

The dependency direction is `domain -> model`, with the only planned model-to-model edge
`model/package -> model/dependency`. Model packages must not import Domain, Application, or
Infrastructure. The reverse `model -> domain` option was rejected because Domain validators and
root compatibility aliases would then create an import cycle. A new shared internal abstraction
was rejected because these concrete wire owners need no third layer. Existing callers continue to
use root Domain type aliases, constants, and wrappers; direct model imports are optional.

### Checkpoint 1 — host dependency model

Created `src/lib/model/dependency/model.go` as the sole owner of `DependencySpec`,
`HostDependency`, and `HostDependencyInstallHint`. The complete declaration block, including every
exported field type and YAML tag, reconstructs byte-for-byte to the pre-move Domain block at
SHA-256 `89926b18352bcdf5eeccec320f64aa0203fd191417f8cf13fb5f9c1cee334e4f`.
`src/lib/domain/dependencies.go` now exposes source-compatible aliases under the existing names and
retains `ValidateDependencySpec`, platform validation, and install-hint policy.

Colocated model tests prove exact emitted YAML field names/shape and authored YAML round-trip.
Cached-offline Go 1.25.0 tests pass for `model/dependency`, the full Domain package, and all focused
Application host-dependency checks. `go list -mod=readonly` equivalent resolution through the
existing `/tmp`-only compatibility modfile reports no imports for production `model/dependency`,
Domain importing Model, and Application importing Domain. `go vet` passes for all three packages.
Linux and Windows/amd64 compile-only test binaries pass for Model, Domain, and Application; Windows
binaries were not executed. The checkout module cache was not used or changed: verification copied
the admitted `/tmp` cache and modfile into `/tmp/dockpipe-task035-model-boundary/`. The initial
isolated-cache attempt failed before compilation because it lacked pinned `go-shellwords` metadata;
the copied offline compatibility modfile uses the admitted signature-compatible shellwords stub and
cached `x/sys v0.47.0`, so Application proof does not re-prove production shell parsing.

`gofmt`, `git diff --check`, declaration uniqueness, reconstruction, import direction, and focused
source compatibility pass. Model implementation/test SHA-256 values are
`46bac847c9d92c1c30f12212a4f680673f54faa11dfcf4e9ddba27dc0f2e847e` and
`aaeb24d05b2456108092a429ce2d365c46cdd55a1ed2726e25781921467dd23e`.
No authored bytes, validation behavior, public field types, generated/durable/live state, staging,
commit, push, cleanup, publication, worktree, external action, gate, or transport authority changed.

Pending checkpoint: extract the package-manifest wire structures into `model/package`, using
`model/dependency` for the nested dependency shape while retaining all Domain validation and public
facades.

### Checkpoint 2 — package manifest model

Created `src/lib/model/package/model.go` as the sole owner of `PackageManifest` and its six nested
YAML structures: `PackageImageSpec`, `PackageScriptContract`, `PackageStateSpec`,
`PackageBuildSpec`, `PackageSourceBuildSpec`, and `PackageTestSpec`. The package imports only
`model/dependency`, and `PackageManifest.Dependencies` uses that package's `DependencySpec`.
Domain retains source-compatible aliases for all seven public names plus filesystem parsing, YAML
compatibility normalization, and every validation/policy function; no Application or
Infrastructure caller changed. Replacing only the explicit qualified dependency type reconstructs
the complete pre-move declaration block byte-for-byte at SHA-256
`8d0d5f732e4b298dc58783aea7053af483e8b8ff12aecb821a402173f36a8cb1`.

Colocated model tests prove the exact emitted YAML names and nesting for every field, including the
model/dependency edge, plus authored YAML round-trip behavior. Cached-offline Go 1.25.0 tests pass
for `model/package`, the full Domain package, focused Application package-manifest consumers,
focused package-compiler closure validation, and focused Infrastructure capability/package-state
consumers. Offline `go list -mod=readonly` and `go vet` pass for `model/package`, Domain,
Application, packagecompile, and Infrastructure through the admitted `/tmp`-only compatibility
modfile/cache. Linux and Windows/amd64 compile-only test binaries pass for Model, Domain, and
Application; Windows binaries were not executed. Their hashes are respectively
`235b7c7778f56a9fd0fe3230c5e398c9f6bf13793e0b18040af9b760892cf43e` /
`ca1cc12e54e9217a38e134f93de3ab956d517c01e331f9c6e42ed861961a79ab` for Model,
`c2679c58d3eb98e4d159728f8c988c7c8f35ba445cd3f0825d70b58d8345d7bd` /
`05682d736bd15076045245b083f8a6bf097fc48fadaa72dc05716e625d8a79c7` for Domain, and
`6f748a920bb464a4d566a6c7840daf96877cbf878828c19aa897890c1adef999` /
`3f3e530c12fbe681ac4858b13ab4dc73287b52b7892712829e2e1eb2517c9503` for Application.
The six binaries remain under `/tmp/dockpipe-task035-model-boundary/` because cleanup is excluded.

The first attempted owner test selected the host Go 1.22.12 toolchain and stopped before
compilation; the successful proof used the explicit cached Go 1.25.0 binary. `gofmt`,
`git diff --check`, declaration uniqueness, reconstruction, source-compatible aliases, and import
direction pass. Model implementation/test SHA-256 values are
`96b83f35b75e84243c74ca7652a4b49f3b5f9c3d5189ea6ff4112758c80ad114` and
`cf189278c82f308ce9b25bb0194e83e4374c6c336d0083aa95d1e740a18008a1`.
The protected Python cache remains mode `0664`, 8408 bytes, and exact at SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.
No authored, public API, error, path, generated checkout, durable, live, or external bytes changed.

Pending checkpoint: extract the selected project-config wire structures and custom `compile`
decoder into `model/project`, retaining Domain validation, defaults, path policy, filesystem
facades, and public aliases.

### Checkpoint 3 — project config model

Created `src/lib/model/project/model.go` as the sole owner of `DockpipeProjectConfig`,
`DockpipeSecretsConfig`, `DockpipeCompileConfig`, `DockpipePackagesConfig`, and
`DockpipePackageSourceConfig`, including `DockpipeCompileConfig.UnmarshalJSON`. Domain retains the
project-config filename, filesystem loader and root discovery, default scaffold, secret-template
path policy, package-source normalization, validation, and source-compatible aliases for all five
public types. No Application or Infrastructure caller changed. The complete pre-move declaration
and decoder block reconstructs byte-for-byte from the new owner at SHA-256
`237c2ed892427b03f088b0c5a279d1707a7c0fc6c5529358fbda1943a4ad2471`.

Colocated owner tests prove exact emitted JSON field names and nesting, explicit empty-list pointer
semantics, forward-compatible unknown-key handling, malformed-JSON behavior, and exact rejection
bytes for the retired `compile.resolvers` and `compile.bundles` keys. Cached-offline Go 1.25.0 tests
pass for `model/project`, the full Domain package, Application compile-config and selected project
config consumers, packagecompile configured-source/compile-root consumers, and focused
Infrastructure compile-root, configured-store, and workflow-tarball consumers. Offline
`go list -mod=readonly` and `go vet` pass for the owner and all affected parent packages through
the admitted `/tmp`-only compatibility modfile and cache. The first test setup stopped before
compilation because workspace mode rejects `-modfile`; the second selected the host Go 1.22.12
launcher under `GOTOOLCHAIN=local` and also stopped before compilation. Successful proof used the
explicit cached Go 1.25.0 binary with `GOWORK=off`; no checkout module or build cache was used.

Linux and Windows/amd64 compile-only test binaries pass for Model, Domain, and Application;
Windows binaries were not executed. Their SHA-256 values are
`a75f511afc44c60254208cfd8affb6a3a43dde81b7130d553d4b9efd72acd9aa` /
`272a986c1e9230be1be134d660a36b7d6116e283f9462acba220656ee134b321` for Model,
`8002e3d99c69b69983a3bbb4c93c254c97491db4d3c39bfe1f949af02b4f4d1a` /
`2c26f619db8cf05f5b7a35b473b33e22dc122b816ea94d3a83226f0f39eeb549` for Domain, and
`33b6c4c8d574c7ae8190adabc4f344eb0a2ade21df8d665d74a0f92b12ae1278` /
`6fe7b4c9228a32b9d9725ee5687a94218b67705125250a9624c3c021da919a15` for Application. The six
new binaries remain under `/tmp/dockpipe-task035-model-boundary/` because cleanup is excluded.

`gofmt`, `git diff --check`, declaration uniqueness, source-compatible aliases, reconstruction,
and dependency direction pass. Production `model/project` imports only the standard library;
Model still imports no Domain, Application, or Infrastructure package. Model implementation/test
SHA-256 values are `066045cce16b086ba4513878ae6c7571441cfd47cb78d39660291d466b2eb381`
and `7cc193b501651755496b8f038396c83f9c482e12ad4c8a005843a4e13ab57da4`.
The protected Python cache and both inherited empty mode-`0664` checkout Go-cache locks remain
byte-identical. No authored/public/error/path bytes, validation/default/unknown-value behavior,
generated or durable checkout state, live/external state, schema, cleanup, migration,
compatibility retirement, staging, commit, push, publication, worktree, gate, or transport
authority changed.

### Checkpoint 4 — compiled runtime and image artifact model

Created `src/lib/model/runtimeartifact/model.go` as the sole owner of the six runtime/image artifact
wire-name and path constants plus eleven compiled JSON/YAML payload structures:
`CompiledRuntimeManifest`, `PolicySources`, `CompiledSecurityPolicy`,
`CompiledNetworkPolicy`, `CompiledFilesystemPolicy`, `CompiledProcessPolicy`,
`CompiledResourceLimits`, `CompiledImageSelection`, `CompiledImageBuildSpec`,
`ImageArtifactManifest`, and `ImageArtifactProvenance`. Domain retains runtime/security/image
cross-model validation, fingerprinting, step-artifact path sanitization, and source-compatible
aliases and constants. No Application or Infrastructure caller changed. The complete pre-move
constant/declaration block reconstructs byte-for-byte from the new owner at SHA-256
`1e76955853e539807493a3848f0aaa00f97b50b8e757dbf5bd7a73caaf491a86`.

Colocated owner tests prove every constant byte, exact JSON and YAML field tag including
`omitempty`, and full JSON/YAML round-trip behavior across both top-level payload graphs. Cached-
offline Go 1.25.0 tests pass for `model/runtimeartifact`, the full Domain package, the imageartifact,
runtimepolicy, and packagecompile leaves, and the selected Application runtime-policy, package-
compile, planned/indexed image, prebuild, and run-time artifact consumers. Offline `go list
-mod=readonly` and `go vet` pass for the owner and all affected parents through the admitted
`/tmp`-only compatibility modfile and cache. Production `model/runtimeartifact` imports nothing;
Model still imports no Domain, Application, or Infrastructure package.

Linux and Windows/amd64 compile-only test binaries pass for Model, Domain, and Application;
Windows binaries were not executed. Their SHA-256 values are
`1eb7a284d1772d60f66bf2c56cbff2a7b4b4a7ac21943942949cb639ab3e05cd` /
`db90ca10cbd17ffa5b4a3ed479264e752e1e404125222dca79d4fd0211a94945` for Model,
`ceaeb5103c5546b773959fde3363cc872a9a0738251eb6133c6a65b1f2b4a9f0` /
`987ae4b3c2d2b19a4ff99a91287df9b543757e624d998f86e90eb14c1d7272e1` for Domain, and
`318b0a3844c9fc826e44aff08f288ae6d5185cd22bab0fb8ed56b45859a68d59` /
`de89cda6c22ee736f73390169a14b73ad3c14b1b8865a164ee089b2ad5197877` for Application. The six new
binaries remain under `/tmp/dockpipe-task035-model-boundary/` because cleanup is excluded.

`gofmt`, `git diff --check`, declaration uniqueness, source-compatible aliases, reconstruction,
and dependency direction pass. Model implementation/test SHA-256 values are
`5e8e12df23c4e949e1d7dd4b89a83fc1dc66873351ae3d1e0f037002991a25e7` and
`340e78fd3556184c4de996c47ca041180503bb05992906fd401b7ab474cecdda`.
The protected Python cache and both inherited empty mode-`0664` checkout Go-cache locks remain
byte-identical. No authored/public/help/error/log/operation-result/environment/path bytes,
validation/default/unknown-value behavior, generated or durable checkout state, live/external
state, schema, cleanup, migration, compatibility retirement, staging, commit, push, publication,
worktree, gate, or transport authority changed.

### Terminal integrated proof

The bounded selected set is complete. Cached-offline Go 1.25.0 tests pass together for all four
Model owners and the full Domain package. Offline `go list -mod=readonly` and `go vet` pass for all
Model packages, Domain, all Application packages, and all Infrastructure packages through the
admitted `/tmp`-only compatibility module/cache. Direct imports prove the intended acyclic graph:
`model/dependency` and `model/runtimeartifact` import nothing; `model/project` imports only the
standard library; `model/package` imports only `model/dependency`; and Domain imports all four
owners. No Model package imports Domain, Application, or Infrastructure.

The eight owner implementation/test hashes exactly match their checkpoint records, declaration
ownership is unique, every root Domain facade remains source-compatible, all four reconstruction
hashes pass, and the latest Linux/Windows compile-only proof covers Model, Domain, and Application.
`gofmt` and `git diff --check` pass. Terminal live anchors remain branch `js/dev`, HEAD
`0972303d4745474f50b4adcab3b2f5d058f92f61`, origin `js/dev`
`3fe03013e695b7b13b62221bd7ccbc6ad4334e00`, 0 behind/1 ahead, both inherited stash objects
unchanged, and zero staged paths. Status contains the admitted state plus exactly the three
runtimeartifact-owned paths: 32 paths at SHA-256
`c2fc8fd6989e5efd4fc0135a3e90e07378be4785196caee5f40e0be825f9b634`. The protected Python cache
and both inherited empty checkout Go-cache locks remain byte- and mode-identical. The two admitted
`/tmp` verification trees remain, including the six new runtimeartifact proof binaries; cleanup is
excluded.

Workflow graph/inject shapes, Domain values and validation, workflow import/merge policy,
resolver/strategy projections, and compile-root/compatibility filesystem I/O remain unselected
exactly as classified. No behavior, schema, authored surface, public help/error/log/operation-
result/environment/path bytes, unknown-value behavior, package/store or durable-state behavior,
generated checkout state, network, Docker, VM, cleanup, migration, compatibility retirement,
staging, commit, push, publication, worktree, external action, gate, or transport authority changed.
The objective is complete.


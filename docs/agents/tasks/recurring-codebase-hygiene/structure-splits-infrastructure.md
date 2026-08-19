## Infrastructure Launch Presentation Responsibility Split — 2026-08-15

Completed only the approved rank-7 infrastructure launch-presentation responsibility split in the
saved dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 132 tracked
dirty paths, 52 default-collapsed untracked paths, and status SHA-256
`d16306f7771e75f572a9745f0a20d2dada659c79be1e0252031640ed598a8eca` matched the
delegated anchors exactly. This task record, `docker.go`, the complete `src` inventory, the absent
destination, the exact contiguous EOF block, and the protected ignored Python cache also matched
every handed-off digest, mode, and size.

Created `src/lib/infrastructure/docker_banner.go` and moved byte-for-byte only the contiguous EOF
block from `const banner` through `shouldShowSpinner`. No original import became unused. The
original file retains `fdInt`, `useDockerInteractiveTTY`, `isTerminalDockerFn`, and all Docker
build/run/network/volume/Git-helper operations and seams. Banner and compact-text bytes, width
thresholds, terminal selection, spinner policy, callers, signatures, APIs, CLI behavior, and
cross-platform contracts remain unchanged.

The moved block retains SHA-256
`4ffa6cdf24a7003d48eebc53fa612c7460787ea97dc923eb6c02b3a981f4aaa5`. Reconstructing
`docker.go` from the two post-split sources reproduces SHA-256
`af3026fd2e81e2b716a8885e3d12a682c9aa2a6b5ee991e63629ce2a5a0c3e9b`. Each of the eight
moved declarations has exactly one definition, and application callers remain only `run.go` and
`usage.go`. Reconstructing the sorted per-file `src` inventory while excluding the new destination
and substituting the reconstructed original reproduces SHA-256
`422cd25b4116aabae843ccb9b38ebd50ed48e5079296fd10752de09499111b43`, proving every
unrelated `src` byte unchanged.

Focused offline validation used a copied existing Go 1.25.0 toolchain and module cache with
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and isolated `/tmp` home, module, build, and
temporary roots:

- `TestRenderBannerForWidth`, `TestShouldShowSpinner`, and
  `TestUseDockerInteractiveTTYNonTTYFiles`, the complete three tests in `docker_test.go`, passed
  together;
- Linux/amd64 and Windows/amd64 infrastructure and application test packages compiled through
  non-executing `go test -c` commands; neither Windows binary was run;
- `gofmt`, import/declaration ownership, exact block and source reconstruction,
  `git diff --check`, task-record reconstruction, unrelated-`src` reconstruction, dirty ownership,
  and protected-cache checks passed.

The initial setup-only host `go version` invocation selected the Go 1.22 launcher, attempted to
select Go 1.25, and stopped before compilation because its home module-cache lock path was read-only.
Selecting and copying the already cached Go 1.25.0 toolchain and modules allowed every required
offline test and compile check to pass. The setup-only stop changed no repository state, and all
validation binaries and cache copies exist only under `/tmp`.

Final owned source SHA-256 values are
`ee1fa757c344b178527abb920197221ca82db02de7d8c293c8658e32cfe51fd5` for `docker.go` and
`072b5a3f6f7d071faa856af1d23684259ecaaa681a3ed82ee5eb810b5b6364b3` for
`docker_banner.go`. Final dirty ownership is 133 tracked paths and 53 default-collapsed untracked
paths with status SHA-256
`84f0d808662e0db81646b689b12f8fbeca4171fac903c0f9b4bff16d464d2a1a`; no files are staged.
Removing only the owned original-source status entry and new destination entry reconstructs the
exact pre-split status SHA-256. The ignored cache remains present and byte-identical at mode `0664`,
size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No Docker/runtime operation or seam, test, package boundary, behavior, API, CLI, schema, config,
manifest, compatibility/state/cleanup surface, generated checkout state, live service,
Docker/network resource, staging, commit, push, worktree, or successor was touched. The
engine/package boundary remains intact: generic launch presentation moved only to an in-package
infrastructure sibling with no new abstraction, import direction, or product-specific special
case.

Terminal disposition: `completed`. Ranks 8-9 remain separately gated. No successor was created.

## Infrastructure Durable-State Storage-Primitives Responsibility Split — 2026-08-15

Completed only the approved rank-8 infrastructure durable-state storage-primitives split in the
saved dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 53 default-collapsed untracked paths, and status SHA-256
`84f0d808662e0db81646b689b12f8fbeca4171fac903c0f9b4bff16d464d2a1a` matched the
delegated anchors exactly. This task record, `durable_state.go`, the complete `src` inventory, the
absent destination, the exact contiguous EOF block, and the protected ignored Python cache also
matched every handed-off digest, mode, and size.

The fresh OS-boundary audit mapped the complete move through the unchanged Unix and Windows
identity, device, link/reparse, owner-permission/DACL, lock, atomic-replacement, and directory-sync
seams. It also confirmed all callers in durable project/package resolution, durable import, and
public package-state publication before mutation. Created
`src/lib/infrastructure/durable_state_storage.go` and moved byte-for-byte only the contiguous EOF
block from `readDurableProjectIndex` through `durablePathExists`. The original file lost that block
and its now-unused `bytes` and `encoding/json` imports only. Public identity/path APIs, all helper
names/signatures/bodies, metadata and index JSON bytes, error text, owner identities, aliases,
permissions, locks, atomic replacement/sync, boundary checks, callers, tests, CLI behavior, and
cross-platform contracts remain unchanged.

The moved block retains SHA-256
`696610180469298c73499ec73573abe62d0038abc071b14be1cb9f56776bf2a4`. Reconstructing
`durable_state.go` from the two post-split sources, including only the two removed imports,
reproduces SHA-256 `24ea988f4b832935515ca7762ee1020d189d57d83326445a70cd9a40c9a14b0a`.
Each of the 15 moved declarations has exactly one definition, and their reference inventory retains
the same callers. Reconstructing the sorted per-file `src` inventory while excluding the new
destination and substituting the reconstructed original reproduces SHA-256
`fcbf83d624e9912aca2d757e6a80f0afc4b0f6bf99148bf5f6b7531c11e1a95f`, proving every
unrelated `src` byte unchanged.

Focused offline validation used the already copied Go 1.25.0 toolchain and module cache with
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and isolated `/tmp` home, build, and temporary
roots:

- all 12 generic and three Unix tests in `durable_state_test.go` and
  `durable_state_unix_test.go` passed together;
- Linux/amd64 and Windows/amd64 infrastructure and application test packages compiled through
  non-executing `go test -c` commands; no Windows binary was run, and the combined platform
  compilation covers all 16 generic/Unix/Windows durable-state tests;
- Go formatting, import/declaration ownership, exact block and source reconstruction,
  `git diff --check`, task-record reconstruction, unrelated-`src` reconstruction, dirty ownership,
  and protected-cache checks passed.

Final owned source SHA-256 values are
`3ff7bb463d3403e54d27a984c92f5bf81dab3ba0c264bf2548f7b133e6c31f48` for
`durable_state.go` and `291ded58ec96a886ead2bc46a58483e43f16046d87f41d47bb9162a01cd34fb7`
for `durable_state_storage.go`. Final dirty ownership is 133 tracked paths and 54
default-collapsed untracked paths with status SHA-256
`2c59942c46f453e53d4f7578cc1406c4c398d85ac00a59e806f46651b7c2c949`; no files are staged.
Removing only the new destination status entry reconstructs the exact pre-split status SHA-256.
The ignored cache remains present and byte-identical at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No durable-import phase, test, package boundary, helper behavior, API, CLI, JSON/schema/error text,
identity, permission, lock, atomicity, boundary, config, manifest, compatibility/state migration or
cleanup surface, generated checkout state, live service, Docker/network resource, staging, commit,
push, worktree, or successor was touched. The engine/package boundary remains intact: generic
durable-state storage internals moved only to an in-package infrastructure sibling with no new
abstraction, import direction, or product-specific special case.

Terminal disposition: `completed`. Rank 9 remains separately gated. No successor was created.

## Infrastructure Durable-Import Phase Audit — 2026-08-15

Completed only the approved offline rank-9 audit in the saved dirty checkout; no durable import was
executed and no Go, test, helper, generated, ignored, runtime, or durable-state byte changed. Before
this documentation update, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133
tracked dirty paths, 54 default-collapsed untracked paths, and status SHA-256
`2c59942c46f453e53d4f7578cc1406c4c398d85ac00a59e806f46651b7c2c949` matched the
delegated anchors exactly. This task record was
`ca7f36b1d8459245c9648430d2b1dcdf4a82544f787882c9a74255d210dcf255`,
`durable_import.go` was
`f26da453d7dec4e6355aaaf6216ec43542ef47419b0be0487fac4fcf40365414`, and the
sorted per-file SHA-256 inventory for every file below `src/` was
`64da05c93c31873e8f2785d5b83e44c40700df24e0e6a5ce9b7d756bc4cd81c3`.
The protected ignored Python cache remained a regular file at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

### Complete declaration and dependency audit

All 38 top-level declarations in the 1,051-line source were classified; the counts below treat the
single constant block as one declaration, matching the source declaration inventory.

| Responsibility | Declarations | Dependency and boundary finding |
| --- | --- | --- |
| Contracts and coordinator (8) | constant block (`durableImportSchema`, manifest/pending names, size limit); `DurableImportMapping`; `DurableCohortImportSpec`; `DurableCohortImportStatus`; `durableImportEntry`; `durableImportManifest`; `durableCohortImportTestHook`; `PrepareDurableCohortImport` | Public contracts, manifest bytes, the test interruption seam, package/project/cohort locks, durable-wins dispatch, and phase ordering must remain in the original file. |
| Input, identity, and runtime preparation (7) | `validatedDurableImportSpec`; `validateDurableImportSpec`; `resolveLegacyPackageStateToken`; `durableImportStorageIdentity`; `prepareDurableImportRuntimeDir`; `ensurePrivateImportSubdirectory`; `durableImportPathContains` | Validation is not a pure extraction: it resolves the maintained public-state compatibility token, validates owner/path mappings, and prepares the separate disposable runtime directory through durable-state boundary helpers. |
| Legacy source inventory (6) | `inspectDurableImportLegacy`; `collectDurableImportEntries`; `collectDurableImportTree`; `readDurableImportFile`; `requireDurableImportDevice`; `newDurableImportManifest` | This is the selected read-only seam. It walks only package-selected mappings, rejects link/reparse/special/cross-filesystem substitution, captures identity/size/digest bytes, sorts destination paths, and returns an immutable source manifest before any publication temporary exists. |
| Publication (2) | `publishDurableImport`; `copyDurableImportEntry` | These create and permission the sibling temporary, write/sync the copying manifest, byte-prove each copy, re-inventory the source and temporary, write the authoritative manifest, remove the pending marker, sync, rename, and sync the cohort root. They must stay together. |
| Interruption recovery (1) | `recoverDurableImportTemporary` | Recovery interprets the same pending/authoritative manifests and observed destination inventory as publication. It promotes only a byte-proven ready temporary, removes only a byte-proven incomplete subset, and fails closed on ambiguity or multiple temporaries. |
| Published authority and durable-wins validation (3) | `validatePublishedDurableImport`; `collectPublishedDurableImportTree`; `durableImportLegacyDiverged` | Existing durable authority is boundary-, permission-, type-, device-, manifest-, and inventory-validated before reporting durable-wins divergence. This phase is coupled to both manifest validation and source inventory. |
| Shared manifest and inventory proof (9) | `writeDurableImportManifest`; `readDurableImportManifest`; `validateDurableImportManifest`; `encodeDurableImportManifest`; `durableImportInventoryDigest`; `sameDurableImportSource`; `sameDurableImportInventory`; `durableImportInventorySubset`; `durableImportDestinationInventory` | These canonical JSON and comparison primitives are consumed across cohort publication, cohort recovery, published-authority validation, and whole public package-state publication/recovery. Moving them as a nominal validation phase would cross atomic/recovery ownership and is not selected. |
| Durability and test sequencing (2) | `syncDurableImportDirectories`; `runDurableImportHook` | Recursive directory sync completes publication durability; hooks model interruption after inventory/copy and before/after rename. Both remain beside publication/recovery. |

The five external source/test caller files were read in full or at every durable-import reference:

| Caller | Current SHA-256 | Contract exercised |
| --- | --- | --- |
| `src/lib/application/internal_state_cmd.go` | `7d7da5fa60bfbdda9ff548bd52707d04690cd2cc581ea7b78d2c470a4464d0cc` | The hidden host-script bridge constructs the public spec/mappings and prints unchanged durable/runtime/status fields. |
| `src/lib/infrastructure/public_package_state.go` | `e352c791d65a6a4cdb5c2f135d1f19540d47786da086c750210c080c958c5f7b` | Whole public package-state import shares the entry/manifest shapes and source inventory, device, file-read, copy, canonical-manifest, inventory, sync, publication, and recovery proof helpers. |
| `src/lib/infrastructure/durable_import_test.go` | `88c779c8679d9a34ea81a6e37391f10d0d8b8a559d3a74b542261d3e5cf62063` | Five generic tests exercise the public coordinator and its test hook. |
| `src/lib/infrastructure/durable_import_unix_test.go` | `3d0816066ea5be894903d5302f66e8c8958d03aa5793ec99c8550d5e9144d21f` | The Unix test proves special-file rejection during source inventory. |
| `src/lib/infrastructure/public_package_state_test.go` | `cc4d541764c9bb918e67c5d8d3ba13201d15f86de04747ef160e5e8c4ef912b7` | The mixed-package test calls the cohort importer using the durable public package-state token and proves selected-tree import without legacy mutation. |

The six focused test declarations map to every high-risk phase:

- `TestDurableCohortImportCopiesSelectedStateAndKeepsRuntimeDisposable` covers package-selected
  inventory, byte-preserving copy, owner-private durable publication, durable/runtime separation,
  restart identity, and collision safety.
- `TestDurableCohortImportDurableWinsAndRejectsUnsafeAuthority` covers existing-authority manifest
  validation, immutable durable-wins behavior, and legacy divergence reporting.
- `TestDurableCohortImportRejectsLinksSubstitutionAndPermissionDrift` covers source identity
  substitution, legacy and destination links, and fail-closed Unix owner-mode validation without
  repair.
- `TestDurableCohortImportRecoversOnlyByteProvenInterruptions` covers incomplete copying, ready
  before rename, and lost-acknowledgement restart paths through the pending/authoritative manifests.
- `TestDurableCohortImportConcurrentPreparationIsStable` covers package/cohort lock serialization
  and stable durable identity under eight concurrent callers.
- `TestDurableCohortImportRejectsSpecialLegacyFiles` covers the Unix FIFO/special-file rejection
  path during tree inventory.

The public package-state mixed-owner test is additional caller proof, not a seventh declaration in
the six-test durable-import set.

### OS, atomicity, and recovery finding

The selected inventory block depends on, but does not own or alter, the rank-8 durable-state
storage/OS seam. Unix uses device/inode identity, device IDs, symlink rejection, current-UID
ownership, `0600`/`0700` validation, `flock`, and directory `fsync`. Windows uses volume/file IDs,
open-reparse-point inspection, reparse rejection, current-user ownership, a protected DACL granting
only that user and Local System, `LockFileEx`, and its existing directory-sync behavior. Generic
boundary walking rejects links/reparse points and device/volume changes for every existing path
component. Inventory also re-stats opened files and compares identity, size, and SHA-256 before
publication can consume the manifest.

Publication and recovery are one atomic state machine and are not safe candidates for this split.
The coordinator holds the per-instance lock while publication uses an owner-only sibling temporary
on the verified filesystem, a canonical `copying` manifest, synced copied files, a second source
inventory, an observed destination inventory, a canonical `authoritative` manifest, pending-marker
removal, deepest-first directory sync, destination-absence proof, directory rename, and cohort-root
sync. Recovery reads the same two manifests and inventory relations: it renames only a complete
ready temporary, removes only an incomplete byte-proven subset, and otherwise stops for manual
review. Separating either manifest validation or shared inventory comparison from this pair would
reduce locality across both cohort and public package-state recovery without creating an independent
responsibility.

### Exact separately approved implementation slice

One future split is warranted because the read-only legacy inventory phase is cohesive, shared by
the cohort and public package-state importers, and terminates before temporary creation or any
atomic/recovery transition.

- **Result:** create `src/lib/infrastructure/durable_import_inventory.go`; move byte-for-byte only
  the contiguous block from `inspectDurableImportLegacy` through `newDurableImportManifest`
  (current lines 385-558, six declarations, block SHA-256
  `53d4b4153325c7a43f86e6f2612713a8ae30f59f3f0d8b097ff1c76af2004d3b`). The destination is
  currently absent. Its imports are only `crypto/sha256`, `encoding/hex`, `errors`, `fmt`, `io`,
  `os`, `path/filepath`, and `sort`; all current imports remain required by the original file, so
  its import block does not change. Preserve every name, signature, body, declaration order, and
  same-package caller.
- **Callers:** `PrepareDurableCohortImport`, `publishDurableImport`, and
  `durableImportLegacyDiverged` continue to call `inspectDurableImportLegacy`; whole public
  package-state inventory continues to call `newDurableImportManifest`,
  `requireDurableImportDevice`, and `readDurableImportFile`. Public APIs and the application bridge
  remain unchanged.
- **Required invariants:** exact legacy and durable inventory bytes/digests; source identities;
  package owner, cohort, instance, and run IDs; sorted paths; mappings/ignores; durable-wins;
  owner-only modes/DACLs; link/reparse/special-file/filesystem-boundary rejection; locks; sibling
  temporaries; canonical manifests; sync/rename/recovery and interruption semantics; legacy-byte
  preservation; runtime/durable separation; public signatures, CLI output, generic engine
  boundaries, and cross-platform behavior.
- **Validation matrix:** on Linux, run `go test ./src/lib/infrastructure -run '^(TestDurableCohortImport|TestPackageStateDir)'` to cover all six named durable-import generic/Unix tests and all six public package-state preparation tests, plus `go test ./src/lib/application -run '^TestInternalStatePrepareDurableCohort$'` for the application caller;
  compile without executing the Linux and Windows/amd64 infrastructure and application test
  packages; verify Go formatting, exactly one definition for each moved declaration, unchanged
  references/import ownership, the exact block hash, reconstruction of `durable_import.go` to
  `f26da453d7dec4e6355aaaf6216ec43542ef47419b0be0487fac4fcf40365414`, unrelated-`src`
  reconstruction to
  `64da05c93c31873e8f2785d5b83e44c40700df24e0e6a5ce9b7d756bc4cd81c3`, task/dirty-state
  reconstruction, `git diff --check`, and the protected-cache mode/size/hash.
- **Exclusions:** no publication, recovery, manifest-validation, inventory-validation, runtime,
  durable-state, public package-state, OS helper, test, caller, API, CLI, JSON, schema, error,
  config, manifest, compatibility, migration, cleanup, or generated-state behavior change; no new
  package or abstraction; no live/Docker/network action, staging, commit, push, worktree, adjacent
  cleanup, further implementation, or successor creation.

Terminal disposition: `completed`. The audit selected but did not implement or create the exact
successor **TASK-035 infrastructure durable-import source-inventory responsibility split**. It
requires the exact user response
`Approve next slice: TASK-035 infrastructure durable-import source-inventory responsibility split`
before one fresh-task handoff.


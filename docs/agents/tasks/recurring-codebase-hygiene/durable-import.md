## Infrastructure Durable-Import Source-Inventory Responsibility Split — 2026-08-15

Completed only the approved rank-9 infrastructure durable-import source-inventory responsibility
split in the saved dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 54 default-collapsed untracked paths, and status SHA-256
`2c59942c46f453e53d4f7578cc1406c4c398d85ac00a59e806f46651b7c2c949` matched the
delegated anchors exactly. This task record was
`9b6828ae657661df5cb7e5e103a5f3ae3a4000728e60ebb819d6fbd83039e7c0`,
`durable_import.go` was
`f26da453d7dec4e6355aaaf6216ec43542ef47419b0be0487fac4fcf40365414`, the
destination was absent, the selected contiguous block was
`53d4b4153325c7a43f86e6f2612713a8ae30f59f3f0d8b097ff1c76af2004d3b`, and the
sorted per-file `src` inventory was
`64da05c93c31873e8f2785d5b83e44c40700df24e0e6a5ce9b7d756bc4cd81c3`.
The protected ignored Python cache also matched its delegated type, mode, size, and digest.

Created `src/lib/infrastructure/durable_import_inventory.go` with only the required standard-library
imports and moved byte-for-byte only the contiguous six-declaration block from
`inspectDurableImportLegacy` through `newDurableImportManifest`. The original import block remains
unchanged. Publication, recovery, manifest and inventory validation, durable-wins coordination,
contracts, locking, OS helpers, and every caller remain in their prior files and declaration order.

The moved block, including its original trailing declaration separator, retains SHA-256
`53d4b4153325c7a43f86e6f2612713a8ae30f59f3f0d8b097ff1c76af2004d3b`. Reconstructing
`durable_import.go` from the two post-split sources reproduces SHA-256
`f26da453d7dec4e6355aaaf6216ec43542ef47419b0be0487fac4fcf40365414`. Each moved
declaration has exactly one definition, caller references are unchanged, and reconstructing the
sorted per-file `src` inventory while excluding the new sibling and substituting the reconstructed
original reproduces SHA-256
`64da05c93c31873e8f2785d5b83e44c40700df24e0e6a5ce9b7d756bc4cd81c3`, proving every
unrelated `src` byte unchanged.

Focused offline validation used the previously cached Go 1.25.0 toolchain and complete module
cache with `GOTOOLCHAIN=local`, `GOWORK=off`, `GOPROXY=off`, and isolated `/tmp` build and
temporary roots:

- `go test ./src/lib/infrastructure -run '^(TestDurableCohortImport|TestPackageStateDir)'`
  passed all selected infrastructure tests;
- `go test ./src/lib/application -run '^TestInternalStatePrepareDurableCohort$'` passed the
  application caller test;
- Linux/amd64 and Windows/amd64 infrastructure and application test packages compiled through
  non-executing `go test -c` commands; neither Windows binary was run;
- Go formatting, import/declaration ownership, exact block and source reconstruction,
  task-record and unrelated-`src` reconstruction, dirty ownership, `git diff --check`, and the
  protected-cache checks passed.

The default host Go launcher first stopped before compilation because its home module-cache lock
path was read-only. An older partial offline cache also lacked the pinned `x/sys` module and stopped
before compilation. Selecting the existing complete offline Go 1.25.0 cache allowed every required
test and compile check to pass. These setup-only stops changed no repository state, and all
validation caches and binaries exist only under `/tmp`.

Final owned source SHA-256 values are
`4b3316a789f51cf914b791043efdc440a83c9605f960b738d056dcbb0375ea95` for
`durable_import.go` and
`577d6db1334b9081815bfe2ec2de69dcc3fa9111487b9f188b5c63aa2b445031` for
`durable_import_inventory.go`. Final dirty ownership is 133 tracked paths and 55
default-collapsed untracked paths with status SHA-256
`1dcf99a02fdefed5cb51981c507c8e168cb77d813b20460a2ce17823b3c06d5c`; no files are
staged. Removing only the new destination status entry reconstructs the exact pre-split status
SHA-256. The ignored cache remains present and byte-identical at mode `0664`, size 8,408, and
SHA-256 `25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No function body, signature, declaration order, test, caller, helper, publication, recovery,
manifest or inventory validation, runtime, durable state, public package state, OS behavior, API,
CLI, JSON, schema, error, config, manifest, migration, cleanup, generated checkout state, live
service, Docker/network resource, staging, commit, push, worktree, or successor was touched. The
engine/package boundary remains intact: generic read-only durable-import inventory internals moved
only to an in-package infrastructure sibling with no new abstraction, import direction, or
product-specific special case.

Terminal disposition: `completed`. No successor was created.


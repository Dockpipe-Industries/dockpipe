### Rank 2 Durable Package-State Cohort 1 — 2026-08-14

Implemented only the generic durable-state primitives authorized by cohort 1. The saved checkout
matched the delegated anchors before mutation: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, tracked dirty
diff SHA-256 `2913afee3fc24b5e39e9c2dcff23356eaeda98468478aca60e844d9d708da12c`,
status-name SHA-256 `ba78e8e5428484e9ddd9831fce0d6c9458e0462c3a2ca3e568211410643c231f`,
and this 658-line task record SHA-256
`fdc6e807a6cb182175c4ff3ea8c24a95acd4fb97ed4365feb1d1537f8050d6ad`. Removing
the 179-line durable-contract checkpoint reconstructed the earlier 479-line record byte-for-byte
at SHA-256 `91884f49bea67d20eff73ef326a7c8061350268c5c1614d01405260ea0061f10`.

The new engine infrastructure provides:

- `DurableStateRoot`, with Linux `$XDG_STATE_HOME/dockpipe` or
  `~/.local/state/dockpipe`, macOS `~/Library/Application Support/dockpipe/state`, and Windows
  `%LOCALAPPDATA%\dockpipe\state` plus the existing user-home fallback. It is deliberately
  independent of `DOCKPIPE_GLOBAL_ROOT` and `DOCKPIPE_PACKAGES_ROOT`.
- `ProjectStateRoot`, backed by a random 128-bit project ID, owner-only versioned project index and
  metadata, canonical real checkout path, and OS filesystem identity. Symlink aliases and
  same-filesystem renames retain the ID and update the canonical path; a copy or changed filesystem
  identity receives a new ID. Identity resolution is serialized by an owner-only advisory lock and
  malformed or substituted metadata fails closed.
- `ProjectPackageStateDir`, using the exact trimmed, case-preserving durable owner ID and an
  informational slug plus its full SHA-256. The accepted manifest-compatible printable ASCII
  identity alphabet is intrinsically NFC, while empty, non-ASCII, control, NUL, and invalid UTF-8
  identities fail closed without adding an external Unicode-normalization dependency. Case and
  legacy-sanitizer collisions therefore cannot share durable bytes.
- POSIX owner validation with exact `0700` directories and `0600` files regardless of umask.
  Windows creation validates the current-user owner, replaces inheritance with a protected DACL
  granting full control only to that user and Local System, and rejects reparse points.
- `JoinStatePath`, which rejects absolute, volume-qualified, empty, current/parent, control,
  alternate-data-stream, reserved-device, trailing-dot/space, noncanonical, and escaping suffixes.
  Every existing selected-root component rejects symlinks, Windows junctions/reparse points, and
  filesystem/volume substitutions.
- `PackageRuntimeDir`, a separately named collision-safe helper under disposable
  `bin/.dockpipe/packages-runtime`, and `DiscoverLegacyPackageState`, a non-mutating lookup of the
  existing sanitized `bin/.dockpipe/packages/<scope>` root that rejects linked or substituted
  boundaries. Neither helper creates or imports legacy state.

Focused validation passed with dependency lookup disabled and isolated cache/temp roots under
`/tmp`:

- the durable-state test cohort passed under `go test -race`, covering root selection, concurrent
  stable identity, alias/rename/copy behavior, collision-safe owner storage, invalid identities,
  durable/runtime separation, non-mutating legacy discovery, traversal/device-name rejection,
  symlink boundaries, fail-closed metadata, and POSIX modes under umask zero;
- the complete `src/lib/infrastructure` package passed, and `go vet ./src/lib/infrastructure`
  passed;
- broader `go test ./src/lib/...` passed domain, infrastructure, fetch/install, packagebuild, and
  PipeLang, then ended nonzero only at the previously recorded unrelated
  `TestCmdInstallCoreEmitsOperationResults`, whose loopback listener was denied by the workspace
  sandbox (`listen tcp4 127.0.0.1:0: socket: operation not permitted`);
- Windows/amd64 and macOS/amd64 compile-only package checks passed with outputs under `/tmp`; no
  produced test binary was executed;
- formatting, generic-boundary/reference scans, `git diff --check`, owned hashes/modes, protected
  inherited hashes, no-staged-file proof, and final dirty-state inventory passed.

Only the six new `src/lib/infrastructure/durable_state*` files and this task record belong to this
cohort. Existing `PackageStateDir`, public `dockpipe get`/`scope`, environment and SDK surfaces,
package/workflow consumers, canonical docs, generated-state/prune behavior, clean/rebuild,
`DOCKPIPE_PACKAGES_ROOT`, and every inherited dirty file remain unchanged. Tests created only
isolated temporary state and compile artifacts; no real package state was copied, moved, imported,
deleted, or cleaned. The engine/package boundary is preserved because the implementation supplies
only generic identity, path, permission, and discovery primitives; package cohort selection and
migration remain package-owned and separately gated.

Terminal disposition: `completed`. Cohort 2, migration, public cutover, cleanup, staging, commit,
push, and successor creation remain unexecuted and unauthorized. No successor was created.

### Rank 2 Durable Package-State Cohort 2 — 2026-08-14

Implemented only the approved DorkPipe provider recovery-authority migration in the saved dirty
checkout. Before mutation the checkout matched the delegated anchors exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, tracked dirty
diff SHA-256 `2913afee3fc24b5e39e9c2dcff23356eaeda98468478aca60e844d9d708da12c`,
full status SHA-256 `846788a7bbcdfe38c450a0a32076b7dc03233e07c67288f04f582e68f96d9922`,
and this 729-line task record SHA-256
`3527418cd18bc2707ebeeba3becbedb8580b1dee2543c14232ba31537fd6be51`.

The package-owned `statepaths` layer now selects the cohort-1 collision-safe durable DorkPipe owner
for only these recovery-authority consumers:

- provider resume bindings in `provider-pools/sessions.json`;
- immutable session-adapter pins in `provider-pools/session-adapters`;
- App Server sessions and unresolved `.json.lock` claims, snapshots, audit, and aggregates under
  `provider-pools/app-server`.

Provider leases and scratch remain on the existing checkout-local package-state path. Insights,
history, metrics, training, VM state, disposable callers, unresolved IDE/code-server state, and all
public package-state/SDK/environment surfaces remain unchanged.

`PrepareProviderRecoveryAuthority` owns the compatibility import. It resolves and locks the stable
project/package identity, validates the legacy DorkPipe package root without following links or
filesystem substitutions, rejects unclassified recovery-root entries and every special file, and
builds a sorted relative-path/type/size/SHA-256 inventory plus per-object filesystem identities.
Only the approved cohort is copied; legacy leases, scratch, and all non-provider cohorts are left
byte-for-byte and mode-for-mode unchanged. The copy is streamed into an owner-only sibling
temporary, every file and directory is synchronized, the source is re-inventoried before publish,
and canonical versioned provenance binds the exact source identity and inventory.

The complete temporary is atomically renamed into the durable package directory only while the
destination remains absent. A pre-existing, linked, malformed, or substituted destination fails
closed. After publication the durable directory always wins; later regular legacy divergence is
reported by `ProviderRecoveryMigrationStatus` and is never merged. Legacy source substitution,
including identical-byte inode replacement across restart, fails closed without publishing a
destination. A byte-proven incomplete temporary may be removed and recopied; a byte-proven ready
temporary is resumed without replaying the copy. An interruption after rename is treated as lost
acknowledgement: restart validates and uses the durable authority without re-importing legacy bytes.

Provider resume-map reads now return migration and malformed-state errors instead of silently
starting from an empty map. Writes use owner-only directory/file modes. Existing immutable adapter
and App Server state validation remains authoritative, and the focused restart fixture proves that
resume bindings, adapter pins, unresolved-turn no-replay claims, snapshots, audit, and aggregates
survive import and a restart after the legacy provider tree is detached.

Focused offline validation used the cached Go 1.25 toolchain and isolated roots only:

- `go test -race ./statepaths` passed, including selected-cohort copy/no-mutation, owner-only modes,
  provenance/inventory proof, durable-wins divergence, malformed/link/source/destination
  substitution rejection, incomplete-temporary recovery, ready-temporary resume, identical-byte
  restart substitution rejection, and post-rename lost-acknowledgement no-replay;
- focused `./cmd/dorkpipe` provider recovery, adapter, App Server, aggregate, restart, and no-replay
  tests passed;
- `go vet ./statepaths ./cmd/dorkpipe` passed;
- Windows/amd64 and macOS/amd64 compile-only checks passed for both `statepaths` and
  `cmd/dorkpipe`; no cross-compiled test binary was executed;
- the complete affected Go run passed `statepaths` and reached only the inherited, untouched
  `TestProviderPoolWorkdirHashCandidatesIncludeWindowsStyleNormalizations` assertion in
  `cmd/dorkpipe`;
- the full DorkPipe package harness ran offline with the cached toolchain/module store. Its relevant
  Go-backed CAS, insight, orchestration, repository, build-result, auth, and lifecycle tests passed;
  it retained two unrelated inherited failures: `test_software_dev_workflow.sh` could not find
  `workflows/software-dev/task-pack.yml`, and `test_backlog_remote_workflow.sh` reported that
  canonical `--next` unexpectedly selected a task. Neither failing surface has an owned cohort-2
  diff, so both are non-blocking deferred evidence rather than repair authority.

Tests touched only `t.TempDir`, `/tmp` Go build/compile outputs, and the package harness's existing
ignored `bin/.dockpipe/tmp/package-tests` area. No real user durable root or checkout package state
was imported, moved, rewritten, deleted, or cleaned. Generated-state history was not pruned. The six
cohort-1 `src/lib/infrastructure/durable_state*` files and every unrelated inherited byte remain
preserved. No public CLI, config, environment, SDK, editor, workflow, canonical documentation,
clean/rebuild, `DOCKPIPE_PACKAGES_ROOT`, generated-state/prune, staging, commit, push, worktree, or
successor action was performed.

The engine/package boundary remains preserved: cohort 2 adds no engine special case and consumes
only the generic cohort-1 identity/path/discovery primitives from package-owned DorkPipe code.

Terminal disposition: `completed`. Cohort 3 and every later public cutover/cleanup slice remain
separately gated. No successor was created.

### Rank 2 Durable Package-State Cohort 3 — 2026-08-14

Implemented only the approved DorkPipe insights/history and cumulative metrics/training migration
in the saved dirty checkout. Before mutation the checkout matched the delegated anchors exactly:
branch `js/dev`, HEAD `6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0
behind/1 ahead, `stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no
staged files, 34 tracked dirty paths, 19 untracked paths, full status SHA-256
`5d0a4f41196c6d6f73bb92f9c4019039643a95199eafb280e0398d990e73e0b0`, and this
811-line task record at SHA-256
`8e9f27e6ef8686ed12ae9203cd7fb38ae3a8ff8cb43e95d2bfe58ffda7192536`.

The cohort-2 package importer is now parameterized by a package-private durable-cohort
specification while its provider wrapper, manifest shape, recovery behavior, and complete focused
test suite remain intact. Cohort 3 uses that same copy/provenance/recovery machinery to atomically
publish one independent `learning` authority under the cohort-1 collision-safe durable DorkPipe
package owner. Its selected legacy inventory is exactly:

- `analysis/queue.json`, `analysis/insights.json`, and `analysis/history.jsonl`;
- root `metrics.jsonl`;
- `training/metrics.jsonl`.

`analysis/by-category` remains a checkout-local deterministic export. Provider state, run-local
training products, self-analysis, and every other package/runtime family remain unmoved. Selected
legacy bytes are validated without following links or crossing filesystem boundaries, inventoried
as sorted regular-file/directory records with sizes, SHA-256 hashes, and filesystem identities, and
copied without legacy mutation into an owner-only sibling temporary. Canonical provenance, a second
source inventory, byte equality, file/directory synchronization, destination absence, and atomic
rename establish durable authority. A published durable tree wins over later legacy divergence.
Incomplete byte-proven temporaries are removed and recopied; ready temporaries are resumed without
copy replay; identical-byte source-object replacement, malformed provenance, links, special or
unclassified selected entries, source/destination substitution, and filesystem substitution fail
closed. Package-root device validation now protects both cohort-2 and cohort-3 destinations and
recovered temporaries.

All maintained cohort-3 consumers now resolve through the package-owned durable facade:

- user-insight enqueue/process/review/stale/supersede and read-only handoff/request consumers use
  durable analysis authority, while category export stays disposable;
- engine evaluation/promotion/handoff metrics use durable cumulative `metrics.jsonl` with private
  file creation;
- orchestration resolves durable cumulative training through the package-owned helper, appends with
  owner-only creation, and retains its explicit test override;
- self-analysis reads the durable metrics path through the same helper instead of public
  `dockpipe scope --package`; no public package-state, CLI, SDK, environment, editor, or workflow
  surface changed.

Focused offline validation used only isolated state/cache/temp roots under `/tmp`:

- `go test -race ./statepaths ./userinsight ./engine` passed, including all inherited cohort-2
  migration tests and the new selected-copy/no-mutation, provenance/hash, durable-wins, malformed,
  link/source/destination substitution, incomplete/ready interruption, identical-byte restart
  substitution, lost-acknowledgement no-replay, owner-mode, cumulative metrics, and insight restart
  cases;
- focused `handoff`, `orchestrationhelper`, and `cmd/dorkpipe` consumer tests passed, and
  `go vet` passed across `statepaths`, `userinsight`, `engine`, `promotion`, `handoff`,
  `orchestrationhelper`, and `cmd/dorkpipe`;
- Windows/amd64 and macOS/amd64 compile-only checks passed for `statepaths`, `userinsight`,
  `engine`, `handoff`, `orchestrationhelper`, and `cmd/dorkpipe`; no produced binary was executed;
- the new durable-training and self-analysis-metrics shell contracts passed, as did shell syntax,
  and the updated user-insight package smoke proved its reported insights path is outside the
  checkout while category exports remain checkout-local;
- the authoritative offline DorkPipe package harness passed the cohort-3 insight/training tests,
  orchestration lanes and cumulative metrics, repository/build/auth/GPU/CAS/lifecycle checks, and
  retained only the two inherited rank-1 failures: missing
  `workflows/software-dev/task-pack.yml` and canonical backlog `--next` unexpectedly selecting a
  task. A first harness attempt without the preserved module/toolchain cache was rejected as setup
  evidence; the corrected `GOPATH=/home/jamie/go`, `GOPROXY=off` run is authoritative;
- the full affected Go run passed `statepaths` and reached only the inherited, untouched
  `TestProviderPoolWorkdirHashCandidatesIncludeWindowsStyleNormalizations` assertion in
  `cmd/dorkpipe`.

Tests created only isolated durable roots, package-test products, caches, and cross-compile outputs
under `/tmp`; no real user or checkout package state was migrated, rewritten, deleted, or cleaned.
No VM, disposable-caller, IDE/code-server, public state surface, canonical documentation,
clean/rebuild, `DOCKPIPE_PACKAGES_ROOT`, generated-state/prune, external service, dependency
installation, staging, commit, push, worktree, or successor action occurred. Cohorts 1 and 2,
generated-state history, and every unrelated dirty byte remain authoritative. The engine/package
boundary remains preserved: cohort 3 adds no `src/lib` or `src/cmd` product special case and keeps
selection, migration, paths, scripts, and tests in the DorkPipe package.

Terminal disposition: `completed`. VM state, disposable caller conversion, public cutover, cleanup,
and every later cohort remain separately gated. No successor was created.


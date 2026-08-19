## Rank 1 Implementation — 2026-08-14

The generic CI-artifact contract now accepts the existing explicit bindings `workflow`,
`workflow:<name>`, `package`, and `package:<name>` without selecting a product package when no owner
is present. Workflow context remains an automatic workflow binding, explicit raw/analysis paths
still win, and an unbound generic SDK leaves both paths unset. Package script hooks provide the
current manifest name as their explicit package owner. No authored workflow field or schema changed.

DorkPipe retains its previous owned locations through package-owned declarations and caller inputs:

- package tests declare `DOCKPIPE_CI_ARTIFACT_SCOPE=package:dorkpipe`;
- the DorkPipe SDK test proves that explicit binding through public package-state helpers;
- repository CI exports the workflow-scoped `DOCKPIPE_CI_RAW_DIR` and
  `DOCKPIPE_CI_ANALYSIS_DIR` it already resolves before calling the DorkPipe normalizer;
- direct DorkPipe normalization keeps its package-owned Go fallback in the DorkPipe package.

Validation evidence:

- focused `src/lib/application` state-environment tests passed with the cached Go 1.25.11 toolchain
  and offline module cache;
- shell syntax, Pipeon SDK coverage, DorkPipe SDK coverage, and
  `src/scripts/check-templates-core-paths.sh` passed;
- `dockpipe package compile workflows --workdir . --from packages/dorkpipe --force` compiled all
  16 discovered workflow packages, including `dorkpipe`;
- the final offline `dockpipe package test --workdir . --only dorkpipe` run passed the CI
  normalizer, SDK, user-insight, orchestration, auth, GPU-policy, CAS-01, Codex-update, and lifecycle
  checks, then ended nonzero only because `test_software_dev_workflow.sh` could not find its
  generated `workflows/software-dev/task-pack.yml` fixture and `test_backlog_remote_workflow.sh`
  unexpectedly selected a canonical next task; those unrelated failures were not repaired in this
  slice;
- `go test ./src/lib/...` passed every package except `src/lib/application`, whose unrelated
  `TestCmdInstallCoreEmitsOperationResults` could not bind `127.0.0.1:0` in the workspace sandbox;
  the focused application tests above passed.

The compile refreshed ignored package tarballs and package-owned tooling under `bin/.dockpipe/`.
No generated-state cleanup, compatibility retirement, staging, commit, push, network access, live
service, credential, VM, or cloud action was performed. The engine/package boundary is preserved.

## Rank 2 Implementation — 2026-08-14

The project configuration now declares 14 non-overlapping generated roots with a logical owner and
one of three retention classes: `protected`, `retained`, or `rehydratable`. Protected runtime
evidence and retained workflow/package evidence can never become prune-review candidates.
Rehydratable build/cache roots become `review_candidate` only after their configured age threshold.
Every configured path, including an absent path, must resolve to a concrete Git ignore rule or the
command fails closed. Project-relative child paths, unique/non-overlapping roots, owners, retention
values, and age use are validated when `dockpipe.config.json` loads.

Both public modes are read-only:

- `dockpipe generated-state inventory` reports path, logical owner, retention, presence,
  newest-content age, logical bytes, files, owning ignore rule, and disposition;
- `dockpipe generated-state prune-plan` emits the same inventory and dry-run review classification;
- `--json` emits schema `dockpipe.generated-state-report/v1` in configured-root order;
- no flag, configuration field, or internal call removes or mutates a root.

Refreshed read-only evidence from the saved checkout:

- allocated size under `bin/.dockpipe/`: 43,754,168,320 bytes;
- configured-root logical total: 45,679,784,021 bytes and 87,468 files across 14 present roots;
- classification: 2 protected, 3 retained, and 9 rehydratable roots; 5 old rehydratable roots were
  review candidates at the observation time;
- current ignore ownership resolved through the root `.gitignore`, the Pipeon Desktop `.gitignore`,
  and the launcher `.gitignore`; no configured root was unowned.

Validation evidence:

- focused generated-state and project-config tests passed with Go's module proxy disabled and an
  isolated writable build cache;
- a real CLI build under `/tmp` successfully emitted both text inventory and JSON prune-plan output
  against the current checkout;
- `go test ./src/cmd`, `src/scripts/check-templates-core-paths.sh`,
  `tests/unit-tests/test_repo_layout.sh`, and `git diff --check` passed;
- a fresh CMake launcher root under `/tmp/dockpipe-task035-launcher.ovXZL7` configured and built
  offline (109 files; 1,609,552-byte executable) without touching the retained or configured build
  roots; the optional Vulkan-header probe was absent but the build completed;
- the first focused Go invocation could not use the host's read-only default Go build cache; the
  exact tests passed after selecting `/tmp/dockpipe-task035-gocache` and keeping `GOPROXY=off`;
- broad `go test ./src/lib/...` passed domain, infrastructure, fetch/install, packagebuild, and
  PipeLang packages, then ended nonzero only in `src/lib/application` because the unrelated existing
  `TestCmdInstallCoreEmitsOperationResults` could not bind `127.0.0.1:0` in the workspace sandbox.

The command and config model are generic; this checkout's root taxonomy remains authored data in
`dockpipe.config.json`. CLI and package-model documentation are synchronized. No workflow YAML or
schema/editor surface changed. No configured generated root was deleted, pruned, or mutated, and no
staging, commit, push, network, live service, credential, VM, cloud, or unrelated cleanup occurred.
The engine/package boundary is preserved.

Terminal disposition: `completed`. Rank 3 remains a separate, unapproved slice; no successor was
created.

## Rank 2 Maintainer Correction — 2026-08-14

The implementation above remains accurate historical evidence, but it did not close the product and
authoring problem. The required `generated_state.roots` declaration moved internal cleanup taxonomy
onto every project author. A user should not need to enumerate standard generated paths or understand
DockPipe's internal owner, retention, and age classifications before cleaning a disposable tree.

The follow-up package-boundary review also found a prerequisite defect. The generic
`infrastructure.PackageStateDir` helper currently resolves package state under
`bin/.dockpipe/packages/<scope>`. DorkPipe's package-owned `statepaths` layer uses that helper for
provider-pool session records, adapter bindings, App Server aggregates, run metadata, metrics, and
other state with mixed durability requirements. Retention classifications inside the disposable tree
would conceal that ownership error rather than fix it.

Maintainer direction for the corrective slice:

- `bin/.dockpipe/**` is disposable generated state by convention and requires no project declaration;
- build products, caches, temporary state, run artifacts, and reproducible evidence may use that
  tree and must tolerate its complete removal;
- package state that must survive ordinary clean belongs in an OS-appropriate durable state root,
  scoped by stable project and package identity, outside `bin/.dockpipe/**`;
- package-authoring guidance and helpers must make the disposable/durable choice explicit; a package
  must not obtain persistence by placing a retention label inside the disposable tree;
- the uncommitted `generated_state` project taxonomy and read-only `prune-plan` surface should be
  removed rather than made mandatory or preserved as an incomplete cleanup workflow;
- `dockpipe clean --dry-run` should preview the complete `bin/.dockpipe/**` removal and estimated
  reclaimed size, while ordinary `dockpipe clean` should perform that removal;
- nonstandard generated roots outside `bin/.dockpipe/**`, if supported later, require a separate
  simple opt-in contract and must not complicate the default clean path.

The correction is ordered because cleanup must not outrun persistence repair. The first bounded slice
must inventory every `PackageStateDir` and package-scope consumer, classify each as disposable or
durable, select the generic durable project/package state helper, and define compatibility/migration
for existing durable bytes without moving caches or artifacts. A later separately authorized slice
may migrate one exact consumer cohort. Only after no required durable state depends on
`bin/.dockpipe/**` may the final cleanup slice remove the 14-entry checkout declaration, config/model
ceremony, `generated-state`/`prune-plan` surface, and synchronized docs/tests, then widen ordinary
`dockpipe clean` with its dry-run preview.

Acceptance requires package-state survival across clean, complete removal of the disposable tree,
zero project configuration, deterministic dry-run output, no deletion outside the resolved
`bin/.dockpipe/**` root, and focused compatibility, package, path, layout, and CLI tests. This task
update changes no CLI, config behavior, source, generated state, migration, or cleanup authority.

## Rank 2 Durable Package-State Contract Checkpoint — 2026-08-14

Completed as a documentation-only checkpoint against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`. Before mutation the checkout was 0 behind/1 ahead,
`stash@{0}` was `26ea507907550d2449dc6f9c81b9942bd52d8629`, no file was staged, and this
479-line untracked record had SHA-256
`91884f49bea67d20eff73ef326a7c8061350268c5c1614d01405260ea0061f10`. The inherited dirty
checkout, including the seven other untracked files, is authoritative and remains outside this
checkpoint's ownership.

### Maintained-Surface Inventory

The inventory covered maintained source, scripts, package assets, tests, public documentation, and
editor support while excluding ignored/generated `bin/.dockpipe`, `.dorkpipe`, `.staging`, and
`node_modules` trees. Classification applies to the data selected by each surface, not merely to the
current directory name.

| Surface | Evidence anchors | Current role and required disposition |
| --- | --- | --- |
| Generic path primitive | `src/lib/infrastructure/packagelayout.go` (`PackageStateDir`, `SanitizePackageStateScope`, `StateRoot`); `packagelayout_test.go` | Currently maps every package to `bin/.dockpipe/packages/<sanitized-scope>`, so it is a mixed disposable/durable primitive. It must split into the durable `ProjectPackageStateDir` contract below and a checkout-local disposable package-runtime helper. Sanitized display names may not be storage identity. |
| Workflow and package environment injection | `src/lib/application/state_env.go` (`applyDockpipeStateEnv`, package-bound CI paths); `resolver_workflow.go`; `package_script_hooks.go`; their focused tests | Injects `DOCKPIPE_PACKAGE_ID` and `DOCKPIPE_PACKAGE_STATE_DIR` for workflow, resolver, and package hooks and temporarily restores resolver context. The existing variable remains a compatibility surface; after durable cohorts move it must identify durable state, while disposable callers use the new runtime helper/environment. Package-bound CI paths are explicitly disposable and must stop deriving from durable state. |
| Public CLI | `src/lib/application/sdk_cmd.go` (`cmdGet`, `cmdScope`, `packageScopeObject`); `usage.go`; `sdk_cmd_test.go` | `dockpipe get package_state_dir` and `dockpipe scope --package <name>` currently expose the mixed root, including JSON `root` and `state_root`. Their eventual cutover is to the durable package root with legacy discovery; a distinct package-runtime path is required for caches, build products, scratch, and run evidence. The public suffix join must remain within its selected root. |
| Shell SDK | `src/core/assets/scripts/lib/dockpipe-sdk.sh` (`dockpipe_sdk_refresh`, `dockpipe_sdk_package_state_dir`, package CI binding, `path package`, `scope`); `src/core/assets/scripts/README.md`; package SDK tests | Mirrors the CLI fallback under `DOCKPIPE_STATE_DIR/packages`. It must consume injected durable state rather than independently recreate its location. Explicit package CI bindings move to disposable runtime/artifact paths. |
| Go SDK and editor support | `src/core/assets/scripts/lib/repotools/repotools.go` (`PackageScope`, `PackageScopePath`); `src/app/tooling/vscode-extensions/dockpipe-language-support/{README.md,extension.js}` | These are pass-through consumers of `dockpipe scope --package`; no separate storage policy is allowed. Completion/docs must describe durable package state and the separate disposable runtime surface together. |
| Isolation and reset boundaries | `src/scripts/ci-emulate.sh`; `packages/pipeon/resolvers/pipeon/bin/pipeon` | Both deliberately unset inherited package-state context when changing the effective workdir/container boundary. Preserve that behavior so state from one project cannot be rebound to another project identity. |
| Package-authoring and artifact guidance | `docs/agents/core/{core-package-model.md,path-scopes.md,repo-map.md}`; `docs/agents/runtime/artifacts-and-mcp.md`; `docs/workflows/workflow-yaml.md`; `docs/runtime/artifacts.md`; `docs/packages/package-model.md` | Current guidance groups service state, credentials, caches, metrics, and run artifacts under one package scope. Implementation must update canonical and compressed guidance to require an explicit durable-versus-disposable choice; workflow artifacts remain disposable. |
| Clean/build compatibility | `src/lib/application/project_build_cmd.go` (`cmdClean`, `cmdRebuild`); `project_build_cmd_test.go`; `src/lib/infrastructure/packagelayout.go` (`PackagesRoot`) | `clean` currently deletes only the resolved compiled package store and honors `DOCKPIPE_PACKAGES_ROOT`; `rebuild` calls `clean`. The decision below makes ordinary clean checkout-only and decouples rebuild's compiled-store reset. Compiled package stores never become durable project/package state. |
| Core VM package-state consumer | `src/core/assets/scripts/vmimage-run.sh` (`vmimage_state_dir` and callers) | Mixed. Machine UUID/MAC/disk serial, writable firmware variables, TPM state, generated administrator password/SSH identity, and other state needed to preserve a persistent guest identity are **durable** and sensitive. Per-run overlays, sync archives, PID/socket/log files, bootstrap temporaries, and transient run state are **disposable**. The current common root must split before clean. |
| DorkPipe Go path owner | `packages/dorkpipe/lib/statepaths/statepaths.go`; all non-test `statepaths.*` call sites under `packages/dorkpipe/lib` | Mixed facade over the generic helper. Exact owned cohorts are classified below; it must expose separate durable/runtime path functions rather than move its whole tree. |
| DorkPipe package scripts | `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/{aggregate-reasoning-context.sh,dev-stack-lib.sh,merge-paste-prompt.sh,orchestrate-common.sh,orchestrator-prompt.sh,run-self-analysis.sh,self-analysis-prep.sh,self-analysis-signals.sh}` | Direct `dockpipe scope --package dorkpipe` callers span durable cumulative training metrics and disposable build/dev-stack/self-analysis/handoff/node products. Each call must select the corresponding new surface explicitly. |
| IDE resolvers | `packages/ide/resolvers/{cursor-dev,vscode}/assets/scripts/*-common.sh`, `*-session.sh`, `session-idle.sh`; Cursor prep; `packages/ide/tests/test_ide_state_ownership.sh` | **Resolved 2026-08-14.** `remote_active`, `session_container`, active PID/container markers, tool/XDG caches, logs, remote-server downloads, and rehydratable products are **disposable** resolver runtime. General home bytes, XDG config/data, remote `User`/`Machine` settings, and user-installed extensions use separately imported owner-only durable cohorts and nested mounts. Canonical guidance remains for the public-cutover slice. |
| Pipeon build/context | `packages/pipeon/assets/scripts/build.sh`; `packages/pipeon/resolvers/pipeon/assets/scripts/{bundle-context.sh,chat.sh,pipeon.sh}`; Pipeon extension/docs/tests | Built VSIX files under `pipeon/extensions` and regenerated `pipeon-context.md` are **disposable**. The launcher workdir reset is a compatibility boundary, not a state owner. |
| Pipeon dev stack | `packages/pipeon/resolvers/pipeon-dev-stack/assets/scripts/{common.sh,desktop.sh}`; code-server Dockerfile and package tests | **Resolved 2026-08-14.** Ports, PIDs, stack context, runtime environment, GPU/status/image state, generated local keys/TLS, caches, logs, and code-server products are **disposable** runtime. General home/configuration, `User`/`Machine`, and user-installed extensions use separate owner-only durable cohorts; package-built extensions remain rehydratable image built-ins. |

Within DorkPipe's current `bin/.dockpipe/packages/dorkpipe` tree, the exhaustive maintained cohorts
are:

| Cohort | Classification | Evidence and reason |
| --- | --- | --- |
| `provider-pools/sessions.json` | **Durable** | Provider resume mapping read/written by `loadProviderPoolSessions`/`saveProviderPoolSessions` in `provider_pool.go`; deletion loses session affinity. |
| `provider-pools/session-adapters/*.json` | **Durable** | Immutable per-session adapter pins from `ProviderPoolSessionAdaptersDir`; deletion can silently change adapter identity. |
| `provider-pools/app-server/sessions/*.json` and authoritative `.json.lock` unresolved-claim records | **Durable** | Completed-turn/recovery state and no-replay claims in `provider_pool.go`; the `.lock` suffix does not make an unresolved claim disposable. |
| `provider-pools/app-server/{snapshots,audit,aggregates}` | **Durable** | Supervisor restart snapshots, append-only audit evidence, and lifecycle aggregates selected by `ProviderPoolAppServerDir` and `ProviderPoolAppServerAggregatePath`; these are restart/recovery authority. |
| `analysis/{queue.json,insights.json,history.jsonl}` | **Durable** | `packages/dorkpipe/lib/userinsight/userinsight.go` retains user input, review decisions, stale/supersede state, and history. |
| `metrics.jsonl` and `training/metrics.jsonl` | **Durable** | Cumulative metrics drive promotion/training decisions (`engine/provenance.go`, `promotion/promotion.go`, `orchestrate-common.sh`); they are cross-run audit/training records. |
| `analysis/by-category` | **Disposable** | Deterministic export regenerated from `insights.json` by `ExportByCategory`. |
| `edit/<request>`, `reasoning/<request>`, `ci/{raw,analysis}` | **Disposable** | Request/analysis/CI artifacts are reproducible bounded-run evidence, not continuity authority. |
| `self-analysis`, `handoff`, `nodes`, `run.json` | **Disposable** | Derived facts/prompts, worker outputs, and last-run provenance can be regenerated or replaced by the next run. |
| `provider-pools/{leases,scratch}` | **Disposable** | Live coordination and temporary tool/home material; stale bytes must not revive a lease or session. |
| Direct `build`, `dev-stack`, and other package-script runtime subtrees | **Disposable** | Go caches/temp, local stack state, and similar execution products are selected directly by the scripts above and must tolerate full clean. |

No maintained consumer remains unclassified: the two IDE home families and Pipeon
`code-server-home` are split into explicitly durable and disposable subtrees, and every other
observed family is durable or disposable. Unrecognized third-party package-scope bytes are
compatibility data, not evidence that they are disposable; their conservative import rule is
defined below.

### Durable Project/Package State Contract

The selected generic API is `ProjectStateRoot(workdir)` plus
`ProjectPackageStateDir(workdir, packageID)`. The implementation may add an internal metadata/index
helper, but package code receives only the package directory. `PackageStateDir` and public
package-state surfaces eventually delegate to this durable API; a separately named
`PackageRuntimeDir` remains under the checkout's disposable `bin/.dockpipe` tree.

- **OS root:** Linux uses `$XDG_STATE_HOME/dockpipe` or `~/.local/state/dockpipe`; macOS uses
  `~/Library/Application Support/dockpipe/state`; Windows uses
  `%LOCALAPPDATA%\dockpipe\state` with the existing user-home fallback. A future explicit override
  must be absolute, owner-controlled, and dedicated to DockPipe state. `GlobalDockpipeDataDir` is
  reusable path-resolution precedent, but its install/data semantics and `DOCKPIPE_GLOBAL_ROOT`
  override do not define or relocate durable project state.
- **Project identity:** storage uses a random 128-bit project ID recorded in owner-only state-root
  metadata, never a basename, remote URL, sanitized path, or repository content hash. The index
  binds that ID to the canonical real checkout path and an OS filesystem identity (device/inode on
  Unix; volume/file ID on Windows). Symlink aliases resolve to the same project. A same-filesystem
  rename preserves the filesystem identity and updates the path alias; a copy/clone receives a new
  ID even with the same remote or bytes. A cross-filesystem move is a new identity unless an
  explicit future adopt operation proves and records the old ID; no heuristic auto-merge is allowed.
- **Package identity:** storage uses a canonical, validated durable-owner ID. A package hook uses its
  manifest ID; a declared resolver/app/component uses
  `<manifest-id>/<component-kind>/<declared-id>`, preventing unrelated packages from colliding on
  names such as `cursor-dev`. A direct public scope name without package metadata remains an exact
  compatibility owner ID until its maintained caller migrates; it is never guessed into another
  package. Storage uses an informational safe slug plus the full SHA-256 of the NFC-normalized,
  trimmed, case-preserving owner ID, so current lowercase/punctuation sanitizer collisions cannot
  alias. Empty/default scope is invalid for durable state. A package rename or component ownership
  move creates a new identity and requires an explicit package-owned migration/alias decision;
  automatic name folding is forbidden.
- **Layout:** `<os-state-root>/projects/<project-id>/packages/<slug>-<full-id-digest>/`. Project and
  package metadata record schema version and exact identities. Metadata is not author configuration
  and adds no repository file or `dockpipe.config.json` field.
- **Permissions:** on POSIX, DockPipe creates state/project/package directories as `0700` and files
  as `0600` regardless of umask; a package may explicitly publish a less restrictive derived file,
  never by weakening the root. On Windows, inheritance is disabled and the current user SID receives
  full control; broad `Everyone`/`Users` grants are rejected, with only OS-required system access
  retained. Existing roots are validated before use and fail closed if ownership or permissions
  cannot be made safe without following links. Atomic replacements preserve these protections.
- **Path boundary:** package IDs and suffix arguments are data, not path fragments. Reject absolute
  suffixes, `..`, empty identity, NUL/invalid Unicode, alternate data streams, reserved device names,
  and any cleaned result outside its selected root. Walk every existing component without following
  symlinks, junctions, mount substitutions, or Windows reparse points; the state root, project,
  package, lock, temporary, and destination paths must stay on the validated boundary. Migration
  may read a separately validated legacy root but never link the two roots.

### Compatibility, Migration, and Interruption Recovery

Compatibility is discovery-and-copy, not a symlink from durable state back into the disposable
checkout:

1. Resolve and lock the stable project/package identity. Check the durable destination first.
2. If absent, validate the legacy
   `<workdir>/bin/.dockpipe/packages/<SanitizePackageStateScope(packageID)>` without following links.
   Maintained mixed packages copy only the durable cohorts listed above; disposable and unresolved
   cohorts stay in place. An unknown third-party package is conservatively imported whole so clean
   cannot silently destroy unclassified public-surface data.
3. Copy into an owner-only sibling temporary directory under the durable project root. Reject
   special files and links, preserve only regular-file bytes and required directories, write
   versioned provenance containing source identity and cohort, synchronize files/directories, then
   compare a sorted relative-path/type/size/SHA-256 inventory with the selected legacy source.
4. Atomically rename the complete temporary directory into place and synchronize its parent. The
   durable directory then becomes authoritative. Do not delete, rewrite, or mark the legacy tree;
   later clean authority handles it only after all maintained migrations and unresolved splits pass.
5. Before first durable write, migration must complete. Read-only callers may use validated legacy
   fallback while the destination is absent. If both exist, durable state wins and divergent legacy
   bytes are reported but never merged automatically.

An interruption before rename leaves legacy authoritative and only a removable owner-only
temporary. An interruption after rename leaves durable state authoritative even if acknowledgement
was lost. Restart reacquires the identity lock, validates any destination and migration provenance,
removes or resumes only a byte-proven incomplete temporary, and never retries an uncertain merge.
Malformed metadata, ownership drift, ambiguous project identity, destination/source substitution,
or inventory mismatch fails closed without changing either authoritative tree.

`DOCKPIPE_PACKAGE_STATE_DIR`, `dockpipe get package_state_dir`, `dockpipe scope --package`, the shell
SDK, Go SDK, and editor completions retain their names and converge on the durable root only after
all maintained disposable callers have selected `PackageRuntimeDir`. An explicitly injected
`DOCKPIPE_PACKAGE_STATE_DIR` remains a compatibility input only when it equals the resolved durable
root or a separately validated owner-controlled override; changing workdir continues to clear it.
There is no durable-to-legacy symlink and no automatic copy from one checkout/project ID to another.

### `DOCKPIPE_PACKAGES_ROOT`, Clean, and Ordered Implementation

`DOCKPIPE_PACKAGES_ROOT` remains the compiled package-store override and has no effect on durable
project identity or state. Future ordinary `dockpipe clean` removes only the resolved checkout
`<workdir>/bin/.dockpipe/**`; it never deletes an override outside that boundary. `dockpipe rebuild`
must stop calling clean as its package-store reset: it performs a separate, explicitly reported
reset of the resolved `PackagesRoot` (including the existing override behavior) after rejecting the
filesystem root, user home, workdir or an ancestor, durable state roots, links/reparse points, and
other unsafe targets, then performs the build. This preserves external compiled-store compatibility
without widening ordinary clean beyond the checkout.

Implementation remains separately authorized and ordered:

1. Add and test stable project identity, OS durable roots, owner-only creation, boundary validation,
   durable/runtime helpers, and non-mutating legacy discovery. Do not change public scope yet.
2. Migrate DorkPipe provider resume bindings, adapter pins, App Server sessions/claims,
   snapshots/audit/aggregates first because they are recovery authority; prove restart and
   interruption behavior.
3. Migrate DorkPipe insights/history and cumulative metrics/training, then split VM durable identity,
   firmware/TPM/credential state from run temporaries with secret-safe permissions.
4. Convert every maintained disposable DorkPipe, CI, build, Pipeon, and IDE marker/cache caller to
   `PackageRuntimeDir`. Resolve and split IDE/package code-server homes before proceeding; unresolved
   state is a hard stop for clean.
5. Cut the CLI/environment/shell/Go/editor package-state surfaces over to durable state with the
   compatibility importer, update canonical/package-authoring guidance, and decouple
   rebuild/`DOCKPIPE_PACKAGES_ROOT` as specified.
6. Only after focused migration, permission, collision, rename, traversal/symlink, interruption,
   package, SDK, CLI, clean/rebuild, and cross-platform tests pass may a later cleanup slice remove
   the generated-state taxonomy and make clean preview/remove all `bin/.dockpipe/**`.

This checkpoint changed only this task record. It did not change Go/source, configuration, schema,
CLI, SDK, editor, package, workflow, canonical documentation, generated state, or package-state
bytes; it moved/deleted nothing and did not run clean or prune. Existing generated-state history
remains historical evidence, and persistence repair still precedes cleanup. The engine/package
boundary is preserved: generic identity/path/permission primitives belong in the engine, while each
package owns its durable-versus-disposable cohort choice and migrations.

Terminal disposition: `completed`. Rank 2 implementation remains open and separately gated. No
successor was created.


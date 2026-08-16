# TASK-035 Recurring Codebase Hygiene

## Practice, Trigger, and Cadence

This is a recurring feature-cycle practice, not a one-time refactor. Revisit this task after each
material feature cycle lands and before the next large feature tranche or release checkpoint. Each
pass should:

1. re-read the current package/engine and generated-artifact rules;
2. inspect current repository evidence before carrying a finding forward;
3. update ranks, evidence, and disposition in this file;
4. select at most one bounded cleanup slice whose value exceeds its regression and validation cost;
5. obtain separate implementation authority, then validate that slice against its affected boundary.

Feature work does not implicitly authorize cleanup. Generated state, compatibility behavior, public
CLI behavior, package ownership, and long-running evidence lanes require their own explicit scope.
Keep this task open while features continue to land; close individual findings when proven resolved
or intentionally retained.

## Ranking Model

- **Leverage:** expected reduction in future defects, review effort, test time, disk churn, or stale
  guidance.
- **Risk:** regression or evidence-integrity risk if the cleanup is implemented incorrectly.
- **Bounded validation:** the smallest credible proof for the candidate slice. It is a future
  requirement, not evidence that the cleanup has already run.

## Baseline Audit — 2026-08-13

Baseline: `js/dev` at `6752dce7c0540d68cb95e1f718ba0998ea0eae35`. This pass was read-only
outside this task documentation and did not remove generated state, run product tests, or implement
any finding.

| Rank | Finding and current evidence | Leverage | Risk | Bounded candidate slice and validation |
| --- | --- | --- | --- | --- |
| 1 | **Resolved 2026-08-14 — generic state setup no longer owns a DorkPipe package default.** Engine state setup and the core shell SDK now require workflow context or an explicit CI artifact binding; package hooks bind to the current package manifest name. DorkPipe's package tests declare `package:dorkpipe`, and repository CI exports its workflow-owned raw/analysis paths before invoking the DorkPipe normalizer. | Completed | Existing DorkPipe package and workflow artifact locations are retained through explicit owner bindings. | Completed as the only implementation slice in this pass. Focused state-env/SDK tests, the path guard, and all 16 DorkPipe workflow compiles passed. The full DorkPipe package command reached two unrelated fixture failures, recorded below. |
| 2 | **Reopened 2026-08-14 — generated-state review added ceremony instead of enforcing the disposable-state contract.** Project authors should not enumerate generated paths, owners, retention classes, or ages. `bin/.dockpipe/**` is disposable by convention, but the current `PackageStateDir` helper also places package state there and DorkPipe uses it for provider-pool sessions, adapter bindings, App Server aggregates, metrics, and other records. Durable consumers must move before clean can truthfully remove the entire tree. | High: restore one obvious cleanup command and make package persistence ownership correct. | High: widening `dockpipe clean` before classifying and migrating durable package consumers could destroy active session or recovery state. | First classify every package-state consumer and move genuinely durable project/package state to an OS-appropriate durable root outside `bin/.dockpipe`; caches, build output, run artifacts, and reproducible evidence remain disposable. Then remove the uncommitted `generated_state` taxonomy and `prune-plan`, make `dockpipe clean --dry-run` preview the whole disposable tree, and make ordinary `dockpipe clean` remove it. Synchronize helpers, package-authoring rules, compatibility/migration behavior, docs, and focused tests through separately authorized bounded slices. |
| 3 | **Resolved 2026-08-14 — the dead extensionless installer duplicate is removed.** Maintained-reference scans proved that direct callers use `src/scripts/install-record-deps.sh`; basename-only references are the public Make target or its delegation. The retained script includes the newer release-binary installation path. | Completed | The supported Make entrypoint and delegated target remain unchanged; the retained `.sh` file is byte-identical. | Completed as the only implementation slice in this pass. Removed only `src/scripts/install-record-deps`; retained `src/scripts/install-record-deps.sh`. Shell syntax, Make dry-run delegation, path/layout guards, maintained-reference scans, hashes, and owned diff checks passed without executing an installer. |
| 4 | **Resolved 2026-08-14 — native SQLite publication evidence now shares only byte-identical cohort orchestration.** The baseline 16-file `sqliteevidence` package and its 25-file immediate parent contained 41 Go files and no non-Go executable source; the checkpoint adds one Go test helper. The Linux and Windows publication harnesses had one byte-identical 53-line state/cycle block; it now lives in a `linux || windows` test-only helper. | Completed | Platform gates, host qualification, VFS/filesystem and mount/DACL checks, child-process setup, fault boundaries, counters, final assertions, and logs remain in their existing platform files. Contention and failure-matrix orchestration remains separate because its root identities, metadata, code counters, validation faults, and child contracts differ. | Completed as the only implementation slice in this checkpoint. Ordinary focused tests passed offline with every `DORKPIPE_SQLITE_*` gate unset. Linux/amd64 and Windows/amd64 compile-only checks passed into `/tmp`; no produced binary or native evidence lane was executed. |
| 5 | **Resolved 2026-08-14 — scheduler-sensitive test coordination now uses explicit barriers.** The transport read-timeout case retains its semantically necessary 25 ms deadline and durable-state assertion. Cloudflare no-ack evidence now ends with an explicit connection-close barrier and rejects a timeout at that barrier. The helper exits only after the parent releases it through a unique test-owned file rather than after a 50 ms sleep. | Completed | Production transport, readiness, timeout, process, and cleanup behavior is unchanged. Synchronous downstream rejection still precedes both the durable-state comparison and no-ack barrier; any buffered acknowledgement still fails the test. | Completed as the only implementation slice in this checkpoint. The two exact tests passed once, 12 repeated runs, three race-enabled runs, and a four-process loaded cohort of four runs each. Windows compile-only, formatting, references, layout, hashes, and owned/unrelated diff checks passed. |
| 6 | **Resolved 2026-08-14 — removed launcher paths are no longer presented as current launchers.** Pipeon shortcut guidance names only its package-owned resolver entrypoint. Remaining `src/bin/pipeon` and `src/bin/mcpd` references are explicit negative layout guards or this completed historical record. | Completed | Package commands and launcher behavior are unchanged. | Completed as the only implementation slice in this checkpoint. Maintained-reference scans, local path claims, the repository-layout guard, formatting, hashes, and owned/unrelated diff checks passed. |
| 7 | **Resolved 2026-08-14 — source-build result plumbing is package-neutral.** One core shell helper owns version lookup, Go cache/temp setup, timing, operation-result fallback, `go build`, and exit propagation. DorkPipe still declares its three tools and DorkPipe MCP still declares only `mcpd`; neither package imports the other package's source tree. | Completed | Source-build flags, paths, fields, fallback text, durations, failures, and exit codes are retained behind a neutral contract discovered beside the injected core SDK. | Completed as the only implementation slice in this checkpoint. Both focused contracts, real offline source hooks, the MCP package suite, layout/reference/syntax/ShellCheck/whitespace checks, hashes, and owned-state checks passed. The DorkPipe package suite retained two unrelated failures already recorded under rank 1. |
| 8 | **Resolved 2026-08-14 — TASK-013 now separates active authority from immutable closed history.** `docs/agents/tasks/codex-app-server-adapter.md` retains current state, the pause/resume checkpoint, open gates, accepted decisions, implementation test matrix, and impact map in 1,723 lines. One reciprocal-linked archive retains the closed implementation and evidence history. | Completed | Both moved blocks are byte-identical to the authoritative 4,945-line source record and retain their original order; the archive is evidence-only and grants no authority. | Completed as the only implementation slice in this checkpoint. Exact whole-record reconstruction, reciprocal links/anchors, headings, references, layout, whitespace, protected hashes, `git diff --check`, and owned/unrelated state passed. |
| 9 | **Resolved 2026-08-14 — maintained compatibility now has one retirement ledger.** `docs/compatibility-retirement.md` inventories 43 engine/config/editor, CLI, layout/state/Git, core, and first-party package surfaces. Each entry distinguishes active source, current promises and callers/fixtures, source introduction or unproven release floor, missing removal evidence, disposition, and one exact separately gated proof profile. | Completed | Inventory is not deprecation or removal authority; active-supported and recovery-only surfaces remain maintained. | Completed as the only implementation slice in this checkpoint. Canonical/compressed docs, router/reference links, 191 exact source anchors, ledger structure, JSON/YAML, focused engine/package/VM tests, protected hashes, whitespace, and owned-state checks passed. No compatibility behavior changed. |
| 10 | **Resolved 2026-08-15 — generated Python caches no longer enter compiled or source-checkout-scaffolded core output.** The two core source-copy surfaces skip only `__pycache__` directories and `.pyc`/`.pyo` files; ordinary, hidden, ignored non-cache, and loose root source files remain eligible. The unused `line` local is gone, and the canonical layout plus guard now name the five category directories and the intentional `package.yml`/`__init__.py` root files exactly. | Completed | The generic copier, product/demo assets, dynamic resolution, and compatibility behavior are unchanged. | Completed as the only implementation slice in this checkpoint. Fabricated tarball/scaffold fixtures, full infrastructure tests, shell/layout/path checks, Windows compile-only, formatting, protected hashes, and owned-state validation passed. |

## Negative and Boundary Evidence

- The tracked-artifact scan found no committed compiled `.exe`, shared library, archive, package,
  coverage, `node_modules`, or Rust `target` output. The tracked `packages/dorkpipe-mcp/bin/mcpd`
  and `packages/pipeon/resolvers/pipeon/bin/pipeon` files are small shell launchers; they are not
  generated binaries. `.staging/packages/README.md` is tracked documentation, not staged package
  output.
- The old nested MCP package/source paths are guarded by `tests/unit-tests/test_repo_layout.sh`; no
  current production `src/lib/` or `src/cmd/` reference to the removed MCP ownership path was found.
- The compatibility items above are debt candidates, not a conclusion that supported behavior is
  dead. Removal requires evidence that the public contract and retained fixtures no longer need it.

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

### Rank 2 Durable Package-State VM Identity Split — 2026-08-14

Implemented only the approved TASK-035 VM durable guest-identity split in the saved dirty checkout.
Before mutation the checkout matched the delegated anchors exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 47
tracked dirty paths, 25 untracked paths, full status SHA-256
`abedb18004e87e4349d49170057d1c10716340ad573b439b9b7d1002d43241e0`, and this
894-line task record at SHA-256
`5d7bb603cd0d566bf05f9ce0308823ec9edfdc93b49da04b9c4bafcf130175ba`.

The VM package now requests two independent roots through one hidden, generic application bridge:
a collision-safe durable project/package cohort keyed by the resolved guest disk identity and a
collision-safe disposable package-runtime run directory keyed by the run identity. Machine UUID,
network MAC, disk serial, writable firmware variables, TPM state, generated Windows administrator
password, and Windows SSH identity use the durable root. QEMU overlays, sync archives, PID files,
TPM sockets/logs, QEMU logs/argument files, installer/bootstrap media, unattended-install products,
and other run temporaries continue through the disposable runtime root. Generated credentials and
identity files are owner-only, reject missing/empty/linked or internally multiline authority, and
survive a new run without regeneration; an interrupted missing SSH public key is reconstructed from
the durable private key and then checked for an exact key identity match.

The minimum general helper builds on cohort 1's durable project/package and package-runtime
primitives. The VM package supplies its owner, cohort, instance, run, exact legacy file/tree
mappings, and TPM socket/log exclusions; no VM or DorkPipe product knowledge entered the generic
importer. Selected legacy regular files and directories are inventoried in sorted order with sizes,
SHA-256 hashes, and source filesystem identities, copied without source mutation into an owner-only
sibling temporary, re-inventoried, synchronized, and atomically published with canonical
provenance. Durable authority wins over later legacy divergence. Ready temporaries resume without
copy replay; incomplete temporaries are removed only when their bytes are a proven source subset.
Malformed provenance, overlapping/traversing mappings, links/reparse points, special files,
filesystem crossings, same-byte object replacement, source/destination substitution, permission
drift, ambiguous legacy TPM authorities, and malformed helper output fail closed. Legacy discovery
remains non-mutating and real state was not exercised.

Bounded offline validation used isolated durable, legacy, runtime, cache, home, and cross-compile
roots only:

- focused importer and hidden-command tests passed under `-race`, including selected-copy and
  no-mutation, durable-wins/divergence, link/special-file/object/destination substitution,
  permission drift, concurrent preparation, incomplete/ready/lost-ack interruption, malformed
  provenance, and disposable-run separation cases;
- full `src/lib/infrastructure`, focused `src/lib/application`, `src/cmd`, and `go vet` checks
  passed with `GOPROXY=off`; Windows/amd64 and macOS/amd64 compile-only checks passed and none of
  their binaries was executed;
- the authoritative `dockpipe package test --workdir . --only vm` harness passed all VM Go tests
  and the package-owned state-split contract, proving restart stability without identity/password/
  SSH regeneration, per-run runtime separation, collision separation between disks, exact legacy
  mapping, ambiguous TPM rejection, interrupted public-key reconstruction, empty-authority and
  malformed-helper rejection, owner modes, and static durable/disposable caller classification;
- `gofmt`, shell syntax, focused ShellCheck, generic-boundary/reference scans, protected-prefix
  hashes, and `git diff --check` passed. Full-script ShellCheck reports only the inherited warning
  set and removes three inherited `SC2155` findings; it introduces no new finding. A broad
  application package run did not produce a terminal result and was stopped after more than two
  minutes; the focused affected application tests and race run are authoritative for this slice.

The package harness refreshed only ignored package-test products and `/tmp` caches/artifacts. No VM
or native/live qualification, real state migration/deletion/cleanup, disposable caller conversion,
IDE/code-server state, public package-state/get/scope/environment/SDK/editor/workflow/documentation
surface, clean/rebuild, package-root cutover, generated-state/prune behavior, external dependency,
staging, commit, push, worktree, or successor action occurred. Cohorts 1-3, their tests, generated
state history, and unrelated dirty bytes remain authoritative. The package/engine boundary is
preserved by a generic hidden durable-cohort primitive and package-owned VM mappings and behavior.

Terminal disposition: `completed`. Disposable caller conversion, unresolved IDE/code-server state,
public cutover, cleanup, and every later cohort remain separately gated. No successor was created.

### Rank 2 Disposable Package-Runtime Callers — 2026-08-14

Implemented only the approved maintained non-IDE disposable caller conversion in the saved dirty
checkout. Before mutation the delegated anchors matched exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 49
tracked dirty paths, 31 untracked paths, full status SHA-256
`bc33008d857b3a0be3c4b355a36bc0c6555b2b8cc1e80429ff22e77ff252a4e0`, and this
961-line task record at SHA-256
`852911d441f52f11a1e14fb217f31393842ea0a043e35b2cee90047d434e2cab`.

One hidden generic application bridge now exposes cohort 1's collision-safe `PackageRuntimeDir`
with validated optional suffixes. Its optional private-root preparation rejects linked/reparsed or
filesystem-substituted runtime components, applies owner-only protection to only the
`packages-runtime` parent and selected package root, and leaves the checkout state-root mode
unchanged. Package-bound CI paths now use the runtime helper, including the exact manifest owner in
package hooks so legacy-sanitizer collisions cannot share CI bytes. `DOCKPIPE_PACKAGE_STATE_DIR`,
`PackageStateDir`, public `get`/`scope`, the Go/editor public SDKs, and workflow/schema surfaces did
not change.

The DorkPipe package-owned facade now sends only its disposable families to package runtime:

- edit/reasoning request artifacts, `run.json`, nodes, CI raw/analysis, self-analysis, handoff and
  paste prompts, and deterministic `analysis/by-category` exports;
- provider leases and scratch, while provider resume bindings, adapter pins, App Server recovery
  state, insights/history, and metrics/training remain on their separately prepared durable
  authorities;
- package-script build caches, local dev-stack products, self-analysis products, handoff products,
  and node aggregation through the same hidden runtime bridge, with no fallback to public package
  state.

Pipeon built VSIX output, regenerated context, chat/status context lookup, and dev-stack ports,
PIDs, stack context, runtime environment, GPU/status/image stamps, API keys, and TLS material now
select collision-safe `pipeon` or `pipeon-dev-stack` runtime ownership. Existing private keys are
revalidated as regular non-links and repaired to owner-only mode; linked key/certificate/runtime
paths fail closed, newly written runtime environment files are owner-only atomic replacements, and
the runtime root is owner-only. The Pipeon extension and DorkPipe edit context reader resolve the
new context path and do not fall back to the legacy package-state location.

The exact unresolved hard stop remains `pipeon_stack_code_server_home`: it still resolves only the
legacy public `pipeon-dev-stack/code-server-home` path because that tree may contain user settings
or extensions. The similarly unresolved Cursor and VS Code package homes, resolver markers/caches,
and all IDE state were not converted. The two legacy-looking VSIX paths in the temporary
code-server Docker build context are image-context filenames rather than checkout state owners;
they were classified but not treated as package-state authority.

Focused offline validation used isolated durable, legacy, runtime, cache, home, and compile roots:

- affected infrastructure/application and DorkPipe statepaths, CI analysis, engine, handoff,
  insight, worker, and exact CLI context-reader tests passed under `-race`; full non-race
  infrastructure and focused application tests passed, and `go vet` passed across all affected Go
  packages;
- Windows/amd64 and macOS/amd64 compile-only checks passed for affected core and DorkPipe packages,
  with outputs under `/tmp` and no produced binary executed;
- shell syntax, focused DorkPipe/Pipeon runtime classification, DorkPipe SDK/insight/self-analysis,
  Pipeon package, secret-mode/link-rejection, TypeScript typecheck, and Pipeon package-runtime tests
  passed;
- the authoritative Pipeon package harness passed. The DorkPipe package harness passed all affected
  CI/runtime/insight/training/self-analysis/repository/build/orchestration/auth/GPU/CAS/lifecycle
  checks and retained only its two inherited rank-1 failures: the missing
  `workflows/software-dev/task-pack.yml` fixture and canonical backlog `--next` unexpectedly
  selecting a task;
- the complete infrastructure race suite exposed an unrelated inherited race in
  `operation_result_test.go`'s heartbeat counter, while the exact affected race cohort passed. The
  broad DorkPipe CLI package likewise retained only the inherited Windows-style workdir-candidate
  assertion already recorded by cohorts 2-3.

Tests changed only package-test products and isolated `/tmp` roots. An ignored Pipeon extension
build product used for typechecking was restored from the pre-slice tracked source before final
proof. Final protected-byte aggregate SHA-256 values were
`72a2dc05e04d8a18a9b01a8d92a0ac289773d44cc7238804def8eed525d07d93` for the
provider/learning migration implementation and tests,
`58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc` for the VM
state-split implementation and test, and
`a4d0e0ef6afa7b4ab4a8a303b7b231e71316394aee602a09f6ea1f1079e49d4e` for the
unresolved Cursor/VS Code resolver trees. The untouched public Go/CLI package-state compatibility
files hashed to `537a3b9e1eee2f3a758e1e712abab5fd8d7bdd0db67d696afc7bc9481524f466`.
The ignored Pipeon extension build product was restored at SHA-256
`4683442e439a7c9ec921e42c2b3ed276c0327181987bcd8ec2b6b3ddd96425bf`.

No real durable or legacy package state was imported, migrated, rewritten, deleted, or cleaned.
Cohorts 1-3 and VM durable identity behavior remain separated; no VM, IDE/code-server home, public
state cutover, canonical-doc/workflow/schema change, clean/rebuild, package-root cutover,
generated-state/prune action, external dependency/resource, staging, commit, push, worktree, or
successor action occurred. The engine/package boundary remains preserved: only generic runtime
path/private-root plumbing entered core, while DorkPipe and Pipeon own their caller classification.

Terminal disposition: `completed`. IDE/code-server state resolution, public package-state cutover,
cleanup, and every later cohort remain separately gated. No successor was created.

### Rank 2 IDE and Code-Server Ownership Split — 2026-08-14

Implemented only the approved maintained Cursor, VS Code, and Pipeon code-server ownership split
in the saved dirty checkout. Before mutation the delegated anchors matched exactly: branch
`js/dev`, HEAD `6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1
ahead, `stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 67
tracked dirty paths, 33 untracked paths, full status SHA-256
`a5e40fd818f809aa768d0a82abcfd208c1db3e3902e8e957a79ea9cbe331ee47`, and this
1,051-line task record at SHA-256
`fc0eef844007225841c61c99ce4a81563ae0346855c1b48a33ed055b64055822`.
The pre-slice Cursor/VS Code resolver-tree aggregate matched
`a4d0e0ef6afa7b4ab4a8a303b7b231e71316394aee602a09f6ea1f1079e49d4e`.

Each maintained IDE tree now has explicit package-owned ownership rather than one relabeled mixed
home:

- Cursor and VS Code general container-home bytes use separate collision-safe owner-only durable
  `ide-user-home-v1` cohorts. Their downloaded server products, binaries, logs, sockets, session
  activity, active-container markers, XDG cache, Dotnet/NuGet, Go module/build/workspace caches, and
  npm cache use the resolver's collision-safe `PackageRuntimeDir` owner.
- Cursor `.cursor-server` and VS Code `.vscode-server` are runtime trees with nested durable mounts
  only for installed remote extensions and `data/User` and `data/Machine`. XDG configuration and
  data use the independent `ide-user-data-v1` durable cohort; XDG cache remains runtime. Resolver
  guidance and session markers also remain runtime. The former VS Code repo-local home/cache move
  path was removed, so source `.vscode-server`, `.cache`, `.copilot`, `.dotnet`, `.gocache`, and
  `.gitconfig` bytes are never migration authority or mutation targets.
- Pipeon's general code-server home uses `code-server-user-home-v1`, excluding tool caches and all
  `.local/share/code-server` products. The latter is runtime except for nested durable `User` and
  `Machine` data, while user-installed extensions use the independent
  `code-server-user-data-v1` extension directory. Package-built Pipeon and DockPipe language VSIXes
  are copied into the image-owned built-in extension tree, so rehydratable package products cannot
  become durable user state. Code-server settings are seeded into the durable User directory by an
  owner-only atomic replacement that rejects linked or non-regular targets.

Every compatibility import is package-declared and uses the existing generic durable-cohort
importer. Selected legacy trees are copied without source mutation, atomically published, and
retain the importer’s durable-wins, restart, interruption, lost-acknowledgement, object-identity,
link/reparse, special-file, permission, and filesystem-boundary protections. Mixed homes are split
with explicit ignores rather than copied into one authority. One minimum hidden generic helper
creates a validated private subdirectory beneath an already private durable/runtime root; it
rejects traversal, duplicate command arguments, links/reparse points, and filesystem substitution.
No IDE, code-server, or Pipeon product knowledge entered `src/lib` or `src/cmd`.

Focused offline validation used only fabricated durable, legacy, runtime, cache, home, workdir,
global, and temporary roots:

- focused infrastructure/application tests and their race-enabled equivalents passed for the
  private-directory bridge and durable importer; `src/cmd`, affected `go vet`, `gofmt`, and
  `git diff --check` passed;
- Windows/amd64 and macOS/amd64 compile-only checks passed for infrastructure, application, and the
  CLI, with outputs under `/tmp` and no cross-compiled binary executed;
- the authoritative IDE package harness passed its existing devcontainer lifecycle fixture and the
  new ownership fixture. The fixture proves exact durable/runtime classification, collision-safe
  owners, owner-only modes, durable-wins restart behavior, linked-legacy failure, legacy byte
  preservation, workdir isolation, and no source mutation. Its local Node child required the
  narrowly reviewed host execution because the workspace sandbox rejected that child process;
- the authoritative Pipeon package harness passed its repository, SDK, package-runtime, host-MCP,
  and extended code-server split contracts. Shell syntax, focused ShellCheck excluding only the
  inherited warning classes, maintained-reference/classification scans, generic engine-boundary
  scans, and protected generated-extension hash proof passed.

Tests created only isolated `/tmp` roots and the existing ignored package-test products. No real
durable or legacy package state was inspected, imported, migrated, rewritten, deleted, or cleaned;
no Cursor, VS Code, code-server, Docker, VM, or external dependency/resource ran. Exact inherited
protected aggregates remained
`72a2dc05e04d8a18a9b01a8d92a0ac289773d44cc7238804def8eed525d07d93` for provider/learning,
`58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc` for VM identity, and
`537a3b9e1eee2f3a758e1e712abab5fd8d7bdd0db67d696afc7bc9481524f466` for public Go/CLI
package-state compatibility. The ignored Pipeon extension product remained at SHA-256
`4683442e439a7c9ec921e42c2b3ed276c0327181987bcd8ec2b6b3ddd96425bf`.

Public `PackageStateDir`, `dockpipe get`/`scope`, `DOCKPIPE_PACKAGE_STATE_DIR`, public Go/shell/editor
SDKs, canonical docs, workflows, schemas, clean/rebuild, `DOCKPIPE_PACKAGES_ROOT`, generated-state
and prune behavior, staging, commit, push, worktree, and successor creation remain unchanged and
separately gated. The engine/package boundary is preserved: core owns only generic validated
private-directory plumbing; the IDE and Pipeon packages own all state classification, mappings,
mounts, and fixtures.

Terminal disposition: `completed`. Public package-state cutover, canonical guidance, cleanup, and
every later step remain separately gated. No successor was created.

### Rank 2 Public Durable Package-State Cutover — 2026-08-14

Implemented only ordered step 5 in the saved dirty checkout. Before mutation the delegated anchors
matched exactly: branch `js/dev`, HEAD `6752dce7c0540d68cb95e1f718ba0998ea0eae35`,
upstream relation 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 77 tracked dirty paths,
34 untracked paths, full status SHA-256
`e36e869d108ef3e2b163736ee5c8cc2cbf5466a251d24dcc43c90238ad6f2d64`, and this
1,133-line task record at SHA-256
`6ba2a7e311a708589ef7705f984fcb7b839a8cefb0e84c04e6142dd546497756`.
The protected Cursor/VS Code aggregate matched
`788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`.

Public package state now resolves to cohort 1's collision-safe, owner-only durable project/package
directory:

- `PackageStateDir`, `dockpipe get package_state_dir`, `dockpipe scope --package`,
  `DOCKPIPE_PACKAGE_STATE_DIR`, package/workflow injection, and shell/Go/editor SDK guidance
  converge on the exact trimmed, case-preserving durable owner. Public suffixes are boundary
  validated; empty/default identity, traversal, links/reparse points, and unsafe injected roots fail
  closed. Workdir refresh discards inherited package-state authority.
- A validated `package.yml` may declare
  `package_state.compatibility_import: package-owned` plus exact `owner_ids`. DockPipe propagates
  the selected manifest through generic package/workflow script context. The engine contains no
  maintained package names or cohort mappings: declared mixed owners leave the legacy public tree
  untouched for their package-owned exact cohort importers, while undeclared third-party owners
  conservatively import the complete validated legacy scope.
- Whole-tree compatibility import uses owner-only sibling temporaries, sorted
  path/type/size/SHA-256 and source-object evidence, a second source inventory, synchronized files
  and directories, atomic publication, and durable-wins divergence reporting. Restart recovers only
  byte-proven incomplete/ready temporaries. Legacy bytes are never rewritten, deleted, or linked to
  durable state, and no rename/copy/project heuristic merges identities.
- DorkPipe, IDE, and Pipeon manifests own their exact maintained public compatibility IDs. The
  unchanged Cursor/VS Code resolver scripts continue passing the public durable token into the
  generic cohort importer; their fabricated ownership fixture proves that only declared durable
  settings/configuration/extensions import while server products, caches, logs, and markers remain
  disposable.

Ordinary `dockpipe clean` remains limited to the checkout compiled store and no longer follows an
external `DOCKPIPE_PACKAGES_ROOT`. `dockpipe rebuild` no longer calls clean: its separately
reported compiled-store reset preserves the override compatibility, validates the exact target,
rejects filesystem roots, user home, workdir/ancestors, durable roots, links/reparse points, special
files, and filesystem substitutions, then builds. No real clean, rebuild, or package-store reset was
run.

Canonical package-model, path-scope, package-authoring, CLI, workflow, artifact, shell SDK, Go SDK,
editor completion, and affected first-party package guidance now agree on durable package state
versus disposable package runtime.

Focused offline validation used only fabricated durable, legacy, runtime, home, cache, workdir,
package-store, and temporary roots:

- current focused infrastructure and application tests passed under the race detector, covering
  whole/selected import, durable-wins, interruption recovery, collision/rename isolation,
  permissions, override validation, traversal, manifest propagation, CLI/get/scope/environment,
  and isolated clean/rebuild reset behavior;
- full domain and Go SDK packages, focused package-hook/application tests, DorkPipe statepaths race,
  Go vet, Go formatting, shell syntax, focused ShellCheck, editor JavaScript syntax, and maintained
  reference/classification checks passed;
- the fabricated shell SDK, IDE ownership, DorkPipe runtime, and Pipeon runtime package contracts
  passed. Windows/amd64 and macOS/amd64 compile-only checks passed for infrastructure and
  application; produced binaries stayed under `/tmp` and were not executed;
- the protected Cursor/VS Code resolver aggregate remained
  `788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`; the VM identity
  implementation/test aggregate remained
  `58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.
  Provider/learning migration paths were outside this slice's owned diff and remained untouched.

No real durable or legacy package state was inspected, imported, migrated, rewritten, deleted, or
cleaned. No clean/rebuild, Cursor, VS Code, code-server, Docker, VM, external dependency/resource,
generated-state/prune removal, clean widening, staging, commit, push, worktree, or successor action
occurred. Generated-state history and all inherited unrelated bytes remain authoritative. The
engine/package boundary is preserved: core owns only generic identity, declaration, import, boundary,
and compiled-store reset primitives; packages own maintained identities and exact cohort mappings.

Terminal disposition: `completed`. Ordered step 6 generated-state/prune removal and ordinary clean
expansion remain separately gated. No successor was created.


## Rank 3 Implementation — 2026-08-14

The saved checkout matched the delegated anchors before mutation: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, local upstream relation 0 behind/1 ahead, and
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`. There were no staged files.
The task record and both installer hashes matched the handoff, as did the protected task-file
hashes.

Maintained-reference proof established that the extensionless file was unsupported:

- repository-wide tracked and non-ignored source scans found direct script-path callers only at
  `Makefile:142` and `Makefile:146`, both naming `src/scripts/install-record-deps.sh`;
- other maintained occurrences name the public `install-record-deps` Make target, and
  `src/scripts/Makefile` delegates that target to the repository-root Makefile;
- no maintained shell invocation names the extensionless `src/scripts/install-record-deps` path;
- the extensionless file was the only tracked installer under `src/scripts/` without a `.sh`
  suffix, and a source diff confirmed that it lacked the retained script's release-binary path.

Only `src/scripts/install-record-deps` was removed. The retained
`src/scripts/install-record-deps.sh` stayed executable and byte-identical at SHA-256
`3c558f0e04f4aacb23ab200e25da91627d5422540b5d7dc674f3b8a431faac26`. The root `Makefile` and
`src/scripts/Makefile` also remained byte-identical.

Validation evidence:

- `bash -n src/scripts/install-record-deps.sh` passed;
- dry runs from both the repository root and `src/scripts/` resolved only to
  `bash src/scripts/install-record-deps.sh`; no installer was executed;
- `src/scripts/check-templates-core-paths.sh` and `tests/unit-tests/test_repo_layout.sh` passed;
- final tracked/non-ignored reference scans contained only the retained `.sh` path and public Make
  target references, with no extensionless shell caller;
- `git diff --check`, owned deletion/record diffs, retained hashes/modes, and protected-file hashes
  passed.

Two initial negative-lookahead scans were not available because this `rg` build reported
`PCRE2 is not available in this build of ripgrep`. Equivalent fixed/default-regex scans passed;
this bounded tooling limitation caused no mutation or validation gap.

No installer, dependency installation, network action, generated-state cleanup, live resource,
rank 4-9 work, TASK-013 work, compatibility retirement, staging, commit, push, credential action,
or worktree operation occurred. No `src/lib` or `src/cmd` file changed, so the engine/package
boundary remains preserved.

Terminal disposition: `completed`. No successor was created.

## Rank 4 Source-Only SQLite Boundary Checkpoint

Completed 2026-08-14 against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35` without a worktree, staging, commit,
network access, dependency installation, native qualification, or opt-in evidence execution.

Executable-language ownership is entirely Go:

- before mutation, `packages/dorkpipe/lib/appserversupervisor/sqliteevidence/` contained 16 files,
  all `.go` test source, totaling 8,539 lines;
- the immediate `appserversupervisor` parent contains another 25 files, all `.go`, so the baseline
  inspected boundary contained 41 Go files and no non-Go executable source;
- the new test-only helper leaves the final inventory at 17 Go files in `sqliteevidence` and 42 Go
  files across the same combined boundary, still with no non-Go executable source;
- non-Go `sqliteevidence` matches are historical/task documentation and command examples, not
  package execution paths.

The Linux/Windows comparison separated identical orchestration from platform semantics:

- the publication harness state initialization and 10,000-cycle operation sequence was exactly
  identical at Windows lines 119-171 and Linux lines 168-220 before mutation; both byte ranges had
  SHA-256 `5308bcaf23c024d6f16e1ebaf30af1f199d8ded4e445e7918841fc2cce8e3cfb`;
- `publication_cohort_orchestration_test.go` now owns that exact block behind
  `//go:build linux || windows`; both platform tests call it after their existing qualification,
  database preparation, and child startup and before their existing stop, counter, digest, tree,
  duration, and evidence assertions;
- Windows retains NTFS/DACL qualification and Windows child setup. Linux retains its Go/module,
  absolute external `TMPDIR`, ext4/mount/root-identity, compile-option, digest, and stable metadata
  predicates. Platform-specific journal and tree implementations remain local;
- contention was not extracted because Linux additionally binds mount/root identity, exact digests,
  and primary SQLite code counters. The failure matrix was not extracted because its child command
  identity, root validation, metadata rollup, compile options, and validation-fault taxonomy differ
  between Windows DACL and Linux ownership/mode/mount evidence.

Bounded offline validation passed:

- `gofmt` and `git diff --check` passed for the three SQLite files in the owned change;
- with `GOWORK=off`, `GOPROXY=off`, writable `GOCACHE`, `GOTMPDIR`, and `TMPDIR` under `/tmp`, and
  every `DORKPIPE_SQLITE_*` opt-in/child variable explicitly unset,
  `go test ./appserversupervisor/sqliteevidence -count=1` passed;
- compile-only `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c` and
  `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -c` passed with outputs only under `/tmp`;
  neither produced binary was executed;
- source/reference scans found the publication cycle only in the shared helper with exactly two
  platform call sites. Protected hashes and the final owned/unrelated diff checks passed.

All `DORKPIPE_SQLITE_*` publication, contention, smoke/evidence, VM harness, and failure-matrix
lanes remain explicitly unexecuted and separately gated. No `src/lib` or `src/cmd` file changed, so
`sqliteevidence` remains package-owned and the engine/package boundary is preserved.

Terminal disposition: `completed`. No successor was created.

## Rank 5 Deterministic Test-Coordination Checkpoint

Completed 2026-08-14 against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35` without a worktree, production
source change, external network, cloud, Docker, VM, credentials, dependency installation, native
SQLite gate, staging, commit, or push.

All three recorded timing sites were classified before mutation:

- `nodeconnectortransport_test.go` retains its 25 ms read deadline. That subtest exercises a real
  transport timeout and proves the failure does not advance durable duplex state; replacing it with
  a readiness signal would stop testing the named behavior.
- The Cloudflare downstream-rejection test did not need a 25 ms absence window. Transport rejection
  and acknowledgement emission are synchronous, so the test now closes the broker side after the
  rejection and unchanged durable-state assertion. A buffered acknowledgement still decodes and
  fails the test; a timeout is rejected as failure to reach the barrier. The normal bounded test I/O
  deadline remains the outer failure bound.
- The Cloudflare helper's 50 ms post-PID sleep was a genuine scheduling gap. The helper now waits
  for a unique regular-file release signal. The parent writes that signal only after origin startup
  has observed PID readiness. The existing one-second process-exit assertion remains, and the helper
  has a five-second fail-safe deadline.

Only `nodeconnectorcloudflaretunnel_test.go` and this task record changed. The transport test stayed
byte-identical at SHA-256
`457ab1504fed06b8de2d147804583865b112d3d13c84e13ee06cfd3a8fd1d695`. No production timeout,
polling interval, runtime behavior, acknowledgement path, durable-state transition, process wrapper,
or cleanup behavior changed.

Bounded offline validation used `GOWORK=off`, `GOPROXY=off`, and writable `GOCACHE`, `GOTMPDIR`,
and `TMPDIR` roots under `/tmp`:

- the two exact top-level tests passed once after mutation and then passed 12 repeated runs;
- the same exact tests passed three runs under `-race`;
- four concurrent local processes each passed four runs using separate temporary roots; the cohort
  used only local loopback sockets and test helper subprocesses;
- Windows/amd64 compile-only validation passed into `/tmp`; the binary was not executed;
- `gofmt`, timing/reference scans, `tests/unit-tests/test_repo_layout.sh`, `git diff --check`,
  protected hashes, and owned/unrelated status and diff checks passed.

The workspace sandbox denied local loopback sockets, so the exact runtime tests were rerun with
narrow host permission; module downloads remained disabled. The package/engine boundary is
preserved because the implementation is package-owned test code only.

Terminal disposition: `completed`. No successor was created.

## Rank 6 Package-Launcher Guidance Checkpoint

Completed 2026-08-14 against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35` without a worktree, production or launcher change,
external resource, dependency installation, generated-state cleanup, staging, commit, or push.

Maintained launcher references were classified before mutation:

- `packages/pipeon/resolvers/pipeon/assets/docs/pipeon-shortcuts.md` contained the only stale current
  launcher statement; it now identifies `packages/pipeon/resolvers/pipeon/bin/pipeon` directly;
- `tests/unit-tests/test_repo_layout.sh` retains explicit negative assertions that
  `src/bin/pipeon` and `src/bin/mcpd` must not exist, plus a negative legacy-MCP reference scan;
- this task record retains removed-path names only as historical evidence and explicitly says they
  are not current launchers.

The command examples in the Pipeon shortcut guide are byte-identical. The package-owned Pipeon and
MCP launchers remain present and executable, and the tracked VS Code task example contains the three
task labels advertised by the shortcut guide. The adjacent scripts README and keybinding example
also remain present.

Bounded offline validation passed:

- maintained source/documentation scans found no removed path presented as a current launcher;
- `tests/unit-tests/test_repo_layout.sh` passed and confirmed both removed paths remain absent;
- local path-claim checks, whitespace/format scans, `git diff --check`, protected hashes, and final
  owned/unrelated status and diff checks passed.

Only `packages/pipeon/resolvers/pipeon/assets/docs/pipeon-shortcuts.md` and this task record changed
in this checkpoint. No `src/lib`, `src/cmd`, package launcher, package behavior, or generated
artifact changed, so the package/engine boundary is preserved.

Terminal disposition: `completed`. No successor was created.

## Rank 7 Package-Neutral Source-Build Result Checkpoint

Completed 2026-08-14 against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35` without a worktree, public CLI/schema/workflow or Go
engine change, package-tree import, external network, dependency installation, generated-state
cleanup, staging, commit, or push.

The duplicated and package-specific portions were classified before mutation:

- version lookup and linker flags, `bin/.dockpipe` output/cache/temp setup, timing, operation-result
  invocation and stderr fallback, the `go build` lifecycle, failures, and exit propagation were
  byte-separate implementations of one package-neutral contract;
- DorkPipe's `dorkpipe`, `skills-render`, and `orchestrate-helper` tool/module/output declarations
  remain in `packages/dorkpipe/assets/scripts/build-source.sh`;
- DorkPipe MCP's `mcpd` declaration and `dorkpipe.mcp` identity remain in
  `packages/dorkpipe-mcp/assets/scripts/build-source.sh`;
- the DorkPipe-only `GOEXE` lookup remains package-owned and retains its original position before
  version/cache initialization.

`src/core/assets/scripts/lib/package-source-build.sh` now owns the neutral mechanics. Source-build
hooks find it beside the already-injected `DOCKPIPE_SDK_SH`, accept an explicit test override, and
fall back to the source-checkout core path for direct maintainer invocation. Neither package imports
the other package tree. The existing dirty `dockpipe-sdk.sh` and scripts README were not edited.

`tests/unit-tests/package-source-build-test-lib.sh` is the neutral fake DockPipe/Go fixture used by
both focused package tests. For every package-owned specification it proves the exact versioned Go
command, module and output identities, default cache/temp paths, successful start/done results,
missing-result-command stderr fallback, failing start/fail results, error field, and exit code `23`
propagation. Its fake `GOEXE=.testexe` also proves that only the two existing DorkPipe helper outputs
receive the platform suffix.

Bounded offline validation evidence:

- both focused `test_build_source_operation_results.sh` contracts passed repeatedly;
- the real DorkPipe source hook built all three tools and the real DorkPipe MCP source hook built
  `mcpd` with the cached Go 1.25.11 toolchain, repository workspace, `GOTOOLCHAIN=local`,
  `GOPROXY=off`, `GOSUMDB=off`, and the existing offline module cache;
- `dockpipe package test --workdir . --only dorkpipe.mcp` passed completely after a narrow host run
  admitted its local loopback HTTP tests; dependency lookup remained disabled;
- `dockpipe package test --workdir . --only dorkpipe` passed the rank-7 contract, both real source
  hook execution paths exercised by its fixtures, and all other runnable checks except the two
  existing rank-1 failures: `test_software_dev_workflow.sh` could not find its generated
  `workflows/software-dev/task-pack.yml` fixture and `test_backlog_remote_workflow.sh` unexpectedly
  selected a canonical next task. Those failures are `non_blocking_deferred` for rank 7;
- an initial package-test attempt with `GOWORK=off` and then an implicit HOME-derived module cache
  was rejected as validation setup evidence: the first hid MCP's intentional workspace dependency,
  and the second hid the existing offline module cache. The corrected runs above are authoritative;
- shell syntax, ShellCheck, targeted ownership/reference scans,
  `tests/unit-tests/test_repo_layout.sh`, `src/scripts/check-templates-core-paths.sh`, whitespace,
  protected hashes, `git diff --check`, and final owned/unrelated status and diff checks passed.

The required package validation refreshed ignored binaries under `bin/.dockpipe/tooling/bin`,
package-test temporary state under `bin/.dockpipe/tmp/package-tests`, and isolated cache/temp roots
under `/tmp`; none was added to source control or cleaned. No package tool list was merged and no
`src/lib` or `src/cmd` file changed, so the package/engine boundary is preserved.

Terminal disposition: `completed`. No successor was created.

## Rank 8 TASK-013 Closed-History Split Checkpoint

Completed 2026-08-14 against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35` without a worktree, feature or product decision,
semantic rewrite, public CLI/schema/workflow or Go/source change, external resource, generated-state
cleanup, staging, commit, or push.

Active and closed content was classified before mutation:

- TASK-013 retains its decision status, current state, pause/resume checkpoint, scope and constraints,
  remaining implementation gates, acceptance criteria, CAS-01 through CAS-13 decisions/evidence, and
  the CAS-14 recommendation, policy, fallback, rendering, retention, and implementation-boundary
  decisions;
- it also retains the accepted post-reconciliation product/storage direction and implementation
  plan, the accepted pre-Slice-2 research policy and selected SQLite design/evidence baseline, the
  implementation test matrix, remaining cross-platform acceptance gates, and impact map;
- the completed CAS-14 implementation-slice history following the retained boundary table was
  immutable closed history; its exact moved bytes have SHA-256
  `f44990470decafe4c19b350acd8a9c389a43543541ea1c4ba1d2bde15e9efb20`;
- the completed dependency-pin, native transactional-store, and Linux VM qualification history
  following the retained SQLite selection was immutable closed evidence; its exact moved bytes have
  SHA-256 `60687f12f7f4d2ddefdf5f4c75e940b3d3deb1644c578068e2ee7adf7ac051f7`.

`docs/agents/tasks/codex-app-server-adapter-closed-history.md` is the single new archive/evidence
record. TASK-013 links to each moved block, and the archive links back to the active task and states
that it grants no implementation, evidence-run, retry, cleanup, migration, or successor authority.
The blocks remain in their original order and are delimited only by archive-owned validation markers.

Bounded offline validation evidence:

- extracting both archive blocks reproduced the exact hashes above; substituting them for the two
  navigation paragraphs reconstructed the authoritative pre-split TASK-013 byte-for-byte at SHA-256
  `a3fffb244993226648e549ad11f5382093147a9133f990ba14456d118fed241f`;
- TASK-013 is now 1,723 lines and the archive is 3,251 lines; each has one top-level heading, the
  retained heading hierarchy is intact, and the two active-to-archive anchors plus reciprocal
  archive-to-active link resolve exactly once;
- the open task index still points only to the active TASK-013 record, and maintained TASK-013
  references remain valid without duplicating or reordering the moved blocks;
- a separately owned concurrent `Rank 2 Maintainer Correction` appeared after the rank-8 baseline
  capture; it was preserved verbatim at SHA-256
  `aabc58ad328b4e7581708e72330a59e48de23f4ba13f7d44d8893fed57e6b1d4`, and isolating only that
  addition proves all inherited rank 1-7 bytes remain unchanged;
- layout, local-link/anchor, trailing-whitespace, file-mode, protected-hash, and dirty-inventory
  checks passed, as did `git diff --check` and final owned/unrelated diff checks;
- no generated artifact was created or refreshed, and no engine/package boundary changed.

Terminal disposition: `completed`. No successor was created.

## Rank 2 Ordered Step 6 Cleanup Contract — 2026-08-14

Implemented only ordered step 6 in the saved dirty checkout. Before mutation the delegated anchors
matched exactly: branch `js/dev`, HEAD `6752dce7c0540d68cb95e1f718ba0998ea0eae35`,
upstream relation 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 105 tracked dirty
paths, 40 untracked paths, full status SHA-256
`993558c6870efa8adfd04020f7990ea86a95727104c7ec631ca56d069b27a031`, and this
1,212-line task record at SHA-256
`fc73f0b53ae1e3509c63392bfc5f8812a7d5338f66f9da57c2e4112dbcd03f39`.

The corrective cleanup contract is now the ordinary product surface:

- the uncommitted `generated_state` project model, validation, 14-root checkout declaration,
  inventory/prune-plan command, report schema, CLI usage, canonical documentation, and focused
  tests are removed; the earlier sections above remain unchanged historical evidence;
- `dockpipe clean --dry-run` requires no project declaration and deterministically reports the
  exact resolved checkout `bin/.dockpipe` target, logical regular-file bytes, file count, and
  `remove` or `noop` action without mutation;
- ordinary `dockpipe clean` removes that complete checkout-local disposable tree after a second
  immediate validation and reports the exact root and inspected totals; a missing tree is a
  reported no-op. Clean reports directly and ignores inherited operation-event paths so reporting
  cannot touch an external file or recreate the disposable tree after removal;
- clean resolves through the generic `StateRoot` helper and rejects parent traversal,
  filesystem-root workdirs, nonstandard/workdir/ancestor/durable targets, linked or reparsed paths,
  special files, and cross-filesystem substitutions. Existing ancestors and the complete removal
  tree must contain only real same-filesystem directories and regular files;
- ordinary clean never consults or follows `DOCKPIPE_PACKAGES_ROOT`. Rebuild retains ordered step
  5's separate guarded reset of the resolved compiled store, including isolated external override
  compatibility.

Focused offline validation used only fabricated workdir, durable, legacy, home, external package
store, filesystem-substitution, link, special-file, and temporary roots:

- focused clean/removal/config tests passed, including exact repeatable dry-run output, whole-tree
  removal, missing-tree no-op, public durable package-state survival, external compiled-store
  and inherited event-log survival, traversal/root/ancestor/durable rejection, link rejection,
  special-file rejection, and simulated filesystem substitution;
- focused infrastructure/application tests passed under the race detector across clean/removal,
  durable identity/import, public package state, package runtime, state environment, internal CLI,
  scope/get, SDK, and manifest-context boundaries; full domain and `src/cmd` tests and affected Go
  vet passed;
- the shell SDK plus DorkPipe, IDE, Pipeon, and VM fabricated ownership/runtime fixtures passed.
  The first DorkPipe/Pipeon fixture invocation lacked isolated durable environment variables and
  was denied by the sandbox before its assertions; both passed when rerun with explicit fabricated
  `HOME` and `XDG_STATE_HOME` roots, and no real state changed;
- an isolated `/tmp` CLI build emitted the exact zero-config missing-tree dry-run preview while an
  external `DOCKPIPE_PACKAGES_ROOT` was set. Windows/amd64 and macOS/amd64 compile-only checks
  passed for infrastructure, application, and `src/cmd`; produced binaries stayed under `/tmp`
  and were not executed except for the native isolated fixture CLI;
- JSON parsing, Go formatting, shell syntax, repository-layout and core-template path guards,
  `git diff --check`, generic engine-boundary scans, active generated-state/prune-plan removal,
  stale clean-contract scans, and branch/HEAD/upstream/stash/index proofs passed;
- the protected Cursor/VS Code resolver-tree aggregate remained
  `788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`, and the VM identity
  implementation/test aggregate remained
  `58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.

No clean, rebuild, prune, migration, compatibility import, or package-state inspection ran against
this checkout or any real/external store. No real durable or legacy package state was inspected,
migrated, rewritten, or deleted. No IDE/code-server, Docker, VM, network, external resource,
staging, commit, push, worktree, or successor action occurred. Generated artifacts are limited to
isolated `/tmp` caches, the native fixture CLI, and compile-only test binaries. The engine/package
boundary is preserved: core owns only the generic checkout cleanup and filesystem-safety primitive;
packages continue to own durable-versus-disposable classification and compatibility migration.

Terminal disposition: `completed`. Any later hygiene or compatibility slice requires separate user
approval. No successor was created.

## Rank 9 Compatibility Retirement Ledger — 2026-08-14

Implemented only the compatibility retirement inventory in the saved dirty checkout. Before
mutation the delegated anchors matched exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 101
tracked dirty paths, 40 untracked paths, full status SHA-256
`492b7aa103bf2d1fda7bfcf05149b3a3852bfea0b999b3bc54137363c1a62b7e`, and this
1,281-line task record at SHA-256
`183f9036bcaa59af899a3cf876c4725d0b22bc72b2a1c3dd6994949d693aa615`.

The maintained contract now has one bounded retirement source of truth:

- `docs/compatibility-retirement.md` records 43 separately addressable surfaces across engine
  configuration and authored YAML, schema/editor mirrors, CLI commands and flags, legacy layouts,
  durable/recovery readers, Git session modes, core environment contracts, and first-party package
  aliases and schemas;
- each row records the exact surface, owner/boundary and active source anchors, current public
  promise, callers/fixtures, first source commit or unproven age, missing removal evidence,
  disposition, and the exact separately gated proof needed before that one surface can retire;
- six proof profiles (`CONFIG`, `CLI`, `LAYOUT`, `STATE`, `PACKAGE`, and `RECOVERY`) keep public
  behavior, state inventory, migration, cleanup, recovery, and package ownership separate;
- active-supported synonyms and architecture fields, forward-compatibility behavior, rejected old
  keys, third-party generated schemas, interoperability inputs, current layouts, and research-only
  future designs are explicitly classified so broad searches cannot silently turn them into debt or
  removal authority;
- `docs/agents/core/compatibility-retirement.md`, `docs/agents/index.yaml`, and `docs/README.md`
  provide the compressed task route and canonical maintainer entrypoint without duplicating the
  ledger.

Focused offline validation used only read operations and isolated Go caches under `/tmp`:

- all 43 IDs are unique and complete, every row has all four fields plus a separately gated proof,
  and 191 parsed exact file/line anchors resolve within 113 current files;
- canonical/compressed local links, repository JSON, and the agent router YAML passed parsing;
- focused domain, application, and infrastructure tests passed for compile-root aliases, vault
  precedence, step aliases/scopes, CLI flags/aliases, workflow layout/runtime normalization,
  legacy run-policy reads, and output-root propagation;
- focused DorkPipe planner/worker, DorkPipe MCP tier, and VM executor compatibility tests passed.
  The first Go commands used `GOSUMDB=off`, which prevented verification of the already selected
  cached Go toolchain; the exact tests passed offline after retaining local checksum verification
  with `GOPROXY=off`. The VM package required `GOWORK=off` from its nested module root before its
  focused tests compiled and passed;
- `git diff --check`, Markdown whitespace/structure, maintained reference scans, protected hashes,
  and final dirty ownership passed. The final status has only the two clean tracked router/reference
  edits plus the two new ledger documents as rank-9 additions; the task record remains its inherited
  untracked path. Final status is 103 tracked dirty and 42 untracked paths with SHA-256
  `ed7063b6ba62292bbc8109623e8d425a8a6d64fd599c132904322a0879cee772`;
- the protected Cursor/VS Code resolver-tree aggregate remained
  `788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`, and the VM identity
  implementation/test aggregate remained
  `58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.

No alias, layout, flag, schema, state, package, workflow, or runtime behavior was deprecated,
warned on, migrated, removed, or changed. No generated state, clean/rebuild, real state inspection,
live service, Docker, VM, network, external resource, staging, commit, push, worktree, or successor
action occurred. Generated artifacts are limited to isolated `/tmp` Go caches. Package/engine
boundaries remain unchanged: the ledger names existing owners and requires package-specific
retirement to remain package-owned.

Terminal disposition: `completed`. Every compatibility retirement remains separately gated. No
successor was created.

## CR-001 `compile.resolvers` Retirement — 2026-08-14

Retired only compatibility ledger entry `CR-001` in the saved dirty checkout. The product owner
established `v0.6.0` as DockPipe's first supported release and confirmed that unreleased development
use of `dockpipe.config.json` `compile.resolvers` does not require compatibility. Local release
policy ships tagged versions from `master`; all available tags stop at `v0.5.8`, the old key first
appeared later in commit `297af0cd`, and no maintained or generated project config uses it. This
completed the previously blocked downstream support/version-floor predicate without inferring a
general downstream absence policy.

Before mutation the checkout still matched the delegated anchors: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 103 tracked
dirty paths, 42 untracked paths, full status SHA-256
`ed7063b6ba62292bbc8109623e8d425a8a6d64fd599c132904322a0879cee772`, this
1,344-line task record at SHA-256
`764e7bdf7a644f7fe22c25f79a3cbbd75587c3da013d2a9cf3ed35727c09de7a`, and the
compatibility ledger at SHA-256
`0b0adf4b9b1aea0506974631b8fa17b2ccbaf7ce1c9b468cede9d9b0fb930b45`.

The bounded retirement removes the `DockpipeCompileConfig.Resolvers` field and resolver-root merge,
rejects that exact JSON key with an error naming `compile.workflows`, and leaves all other unknown
project keys forward-compatible. Canonical `compile.workflows` roots retain configured ordering,
deduplication, flat core-resolver discovery, and the separate `compile.bundles` merge. Package-model
documentation, resolver CLI help, and the VS Code project-config key mirror now expose only the
canonical key. No other compatibility ledger entry changed behavior.

Focused offline verification used isolated `/tmp` Go caches and fabricated project roots. Domain,
command, infrastructure, and focused resolver-compile application tests passed; a native `/tmp` CLI
accepted canonical `compile.workflows`, produced the expected resolver tarball, and explicitly
rejected `compile.resolvers`. The complete application package did not reach a terminal result after
two minutes and was stopped; its CR-001-focused tests had already passed. Editor JavaScript syntax,
Go formatting, 5,920-file JSON caller scanning, 43-row ledger structure, local Markdown links,
focused reference scans, and `git diff --check` passed. Final dirty ownership is 108 tracked paths
and 42 untracked paths with status SHA-256
`d099a470bad79a2f850a766676782540f3ea061b2e7141660568e07dc234a5f3`. The protected
Cursor/VS Code resolver-tree aggregate remains
`788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`; the VM identity
implementation/test aggregate remains
`58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.

No generated checkout state, clean/rebuild, package-state, layout migration, live service, Docker,
VM, network, external resource, staging, commit, push, worktree, or successor action occurred. The
engine/package boundary remains intact: generic project-config parsing owns the exact rejection and
packages continue to consume the canonical generic compile-root contract.

Terminal disposition: `completed`. CR-002 through CR-043 remain separately gated. No successor was
created.

## CR-002 `compile.bundles` Retirement — 2026-08-15

Retired only compatibility ledger entry `CR-002` in the saved dirty checkout. The product owner
established `v0.6.0` as DockPipe's first supported release and confirmed that unreleased development
use of `dockpipe.config.json` `compile.bundles` does not require compatibility. The old key first
appeared after tag `v0.5.8` in commit `297af0cd`, and no authored or generated JSON project config
uses it. The separately governed CLI command `dockpipe package compile bundles` and the
`--with-bundles` / `--skip-bundles` compatibility no-ops remain supported and unchanged.

Before mutation the checkout matched the delegated anchors exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 108 tracked
dirty paths, 42 untracked paths, full status SHA-256
`d099a470bad79a2f850a766676782540f3ea061b2e7141660568e07dc234a5f3`, this task
record at SHA-256 `5edcd2efe0906ec43d8f394c1aa7832496761241620cb48b28c7c6a538271907`,
and the compatibility ledger at SHA-256
`833e17491f353ad64ef87f3e9bf0108000b80712190ed38ca017058dd20638bf`.

The bounded retirement removes the `DockpipeCompileConfig.Bundles` field, workflow-root merge,
dedicated bundle-root cache/API, and dedicated script/Dockerfile consumption. Generic project-config
parsing now rejects that exact key with an error naming `compile.workflows`; all other unknown
project keys remain forward-compatible. `compile.workflows` is the sole configured root authority
for workflow compilation, resolver discovery, PipeLang materialization, logical script lookup, and
Dockerfile lookup. The former old-key path fixture now uses the canonical key.

Maintained coverage proves canonical config loading, exact old-key rejection, rejection when both
canonical and old keys are present rather than precedence or merging, unrelated unknown-key
acceptance, ordered existing/missing root resolution, workflow/resolver/script/Dockerfile consumers,
and a complete fabricated `compile all` that emits core, resolver, and workflow tarballs. Canonical
package-model, package/compile/PipeLang help, and VS Code project-config completion/hover now expose
only `compile.workflows` and `compile.core_from`. The separate CLI alias and no-op help remain
present. Ledger entry `CR-002` is `retired_before_v0.6.0`; `CR-023` and `CR-024` remain unchanged.

Focused offline verification used the reviewed cached Go 1.25.0 toolchain, `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, the existing read-only module cache, and isolated `/tmp` build/temp
caches:

- full `src/lib/domain`, focused compile-root/script/Dockerfile infrastructure, focused config and
  fabricated compile-all application tests, and `src/cmd` passed; focused domain/infrastructure
  `go vet` also passed;
- an isolated `/tmp` native CLI accepted canonical `compile.workflows` plus unrelated future keys,
  emitted the expected workflow tarball, displayed the retained `package compile bundles` alias and
  both retained no-op flags, and rejected a both-key config with the exact
  `compile.bundles is not supported; use compile.workflows` error before creating `bin/`;
- editor JavaScript syntax, Go formatting, 5,992-file local JSON caller scanning, 43-row ledger
  structure, focused source/docs/help/editor reference scans, and `git diff --check` passed;
- two setup-only test attempts stopped before compilation: the first selected the host Go 1.22
  binary under `GOTOOLCHAIN=local`, and the second used an empty isolated module cache. Neither
  changed repository state; validation then passed with the reviewed cached Go 1.25.0 binary and
  existing offline module cache.

Final dirty ownership is 116 tracked paths and 42 untracked paths with status SHA-256
`c53dfeb9f49650fcb26bf8c2e82374851e570629ee49d8d65c5222837c488751`. The
compatibility ledger is SHA-256
`14517144ee400971d6e35899e91129cdd2e339d76adf1d726ed88f9a4bffcdf0`. The
protected Cursor/VS Code resolver-tree aggregate remains
`788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`; the VM identity
implementation/test aggregate remains
`58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.

No generated checkout state, clean/rebuild, package-state, layout migration, live service, Docker,
VM, network, external resource, staging, commit, push, worktree, successor, or other compatibility
retirement occurred. Generated artifacts are limited to isolated `/tmp` caches, fixture trees,
tarballs, and the native validation CLI. The engine/package boundary remains intact: generic config
parsing and compile-root resolution own the retirement, while packages continue to consume the
canonical generic contract.

Terminal disposition: `completed`. CR-003 through CR-043 remain separately gated, including the
independent CLI alias/no-op entries. No successor was created.

## Rank 10 Core Source-Hygiene Audit — 2026-08-15

Completed a read-only `src/core` reachability and architecture-drift audit against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`. Before this documentation checkpoint the checkout had
116 tracked dirty paths, 42 untracked paths, no staged files, and status SHA-256
`c53dfeb9f49650fcb26bf8c2e82374851e570629ee49d8d65c5222837c488751`. Existing dirty source,
TASK-035 ranks 1-9, both completed compatibility retirements, and unrelated package/editor/VM bytes
remain authoritative.

The audit classified evidence without deleting or changing product behavior:

- `src/core` is not broadly dead. The engine resolves runtimes, resolvers, strategies, scripts, and
  bundled workflows by name, and `embed.go` carries the authored tree. `base-dev`, `dev`, VM,
  `dotenv`, the shared SDKs, and bundled workflows have maintained source or documentation callers;
- one ignored 8,408-byte Python cache exists at
  `src/core/assets/scripts/lib/__pycache__/repo_tools.cpython-310.pyc`. Core compilation calls
  `copyDirExcludingTopLevel`, and both that path and source-checkout template merging reach the
  generic unfiltered `copyDir` filesystem walk. The ignored file can therefore become an artifact
  passenger even though it is not maintained source;
- ShellCheck identified the unused `line` declaration in `vmimage_sync_host_to_guest`. Other
  reported dynamic export, SSH, trap, nameref, and pattern warnings were not classified as dead;
- `docs/concepts/architecture-model.md` says the core root contains only category directories, but
  `src/core/package.yml` intentionally drives the guest-agent source build and the Python package
  markers support the documented SDK import path. The layout guard checks category directories but
  does not express those intentional loose-file exceptions;
- `src/core/assets/images/{example,minimal}`, the agnostic Compose demos, example scripts,
  `helloworld.ps1`, and the example resolver are plausible untethering candidates, not proven dead
  code. They remain documented, test-retained, manually buildable, or dynamically copyable. The
  existing `docs/packages/core-vs-packages-audit.md` roadmap already requires a separate product
  decision before moving demo images, Compose, bundled workflows, or lean resolvers into packages.

Read-only/fabricated validation found no unused Go code under `src/cmd` or `src/lib` with
`staticcheck -checks=U1000`. The full infrastructure package tests and focused core compile/template
merge application tests passed offline with isolated `/tmp` caches. A broader application package
run stopped producing output after the infrastructure result and was interrupted without claiming
success; the focused affected tests passed afterward. No repository file changed during the audit,
and its start/final dirty-status hash matched exactly.

This checkpoint authorizes no cleanup. Rank 10 implementation is one separately gated generic
source-hygiene slice only: exact generated-Python-cache exclusion with compiled/scaffolded fixtures,
the unused local removal, and documentation/layout-guard reconciliation. It explicitly excludes
demo/image/Compose/workflow/resolver removal or relocation, compatibility behavior, CLI aliases,
package/state/clean behavior, generated-state cleanup, live resources, staging, commit, push, and
worktrees.

## Rank 10 Core Source-Hygiene Implementation — 2026-08-15

Implemented only the approved rank-10 source-hygiene slice in the saved dirty checkout. Before
mutation the checkout matched the delegated anchors exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 116 tracked
dirty paths, 42 untracked paths, and full status SHA-256
`c53dfeb9f49650fcb26bf8c2e82374851e570629ee49d8d65c5222837c488751`. This
record, the compatibility ledger, and the existing ignored cache matched every handed-off digest,
mode, and size.

The generic `copyDir` contract remains unchanged for workflows, resolvers, clones, and other source
trees. A core-only wrapper now filters exactly directories named `__pycache__` and files ending in
`.pyc` or `.pyo`, without consulting Git or ignore files. Core compilation applies that filter below
its existing top-level resolver/bundle/workflow exclusions, and bundled-core merging applies it only
while copying core source categories or a materialized core source tree. The compiler still carries
loose root source files, and both paths still carry ordinary Python, hidden/ignored non-cache, and
names that merely contain a cache-like suffix.

Focused fixtures fabricate nested `__pycache__` passengers and loose `.pyc`/`.pyo` bytecode. The
compiled-core test proves their omission from the tarball while retaining a root marker, a hidden
core asset, `.py`, and `.pyc.txt` files. The source-checkout merge test proves the cache directory and
loose bytecode do not enter the scaffold while the same ordinary assets and runtime category do.
`vmimage_sync_host_to_guest` lost only its unused `line` declaration. The canonical architecture
document now names `package.yml` as the core source-build manifest and `__init__.py` as the Python
package marker alongside the five category directories; the layout guard enforces those exact file
and directory sets and rejects special root entries.

Focused offline verification used cached Go 1.25.11 with `GOTOOLCHAIN=local`, `GOPROXY=off`,
`GOSUMDB=off`, the existing read-only module cache, and isolated `/tmp` build/temp roots:

- the exact compiled-core, source-build, installed-core merge, and source-checkout merge application
  tests passed, including every fabricated retained/excluded path assertion;
- the complete `src/lib/infrastructure` package passed, including the reconciled core-layout guard;
- Linux shell syntax and warning-level ShellCheck passed after excluding only the pre-recorded
  unrelated dynamic-export, pattern, trap, and array warnings; the unused-local warning class stayed
  enabled;
- repository-layout and canonical template-path guards, Go formatting, and `git diff --check`
  passed;
- Windows/amd64 compile-only builds for the application and infrastructure test packages passed
  with `CGO_ENABLED=0`, emitting only isolated `/tmp` test executables. An earlier `go test -run
  '^$'` attempt compiled those Windows binaries and then predictably failed to execute them on Linux;
  it was replaced by the correct non-executing `go test -c` proof. Two initial setup-only commands
  selected host Go 1.22 under `GOTOOLCHAIN=local` and stopped at the Go 1.25 workspace requirement;
  neither setup attempt changed repository state.

Final dirty ownership is 121 tracked paths and 42 untracked paths with status SHA-256
`ce5ce274a35c3a04f1b0bda4a15e051952816fccf87aceb183c630acae470494`; no files
are staged. The compatibility ledger remains SHA-256
`14517144ee400971d6e35899e91129cdd2e339d76adf1d726ed88f9a4bffcdf0`, the
protected Cursor/VS Code resolver aggregate remains
`788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`, and
`src/lib/application/package_compile.go` remains SHA-256
`8552aab1424d1a065c740826abb5a66c257bba5c1293d29e808b5b287606cbc6`. The real
ignored cache remains present and byte-identical at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`. Reversing
only the owned vmimage local-line edit reconstructs its exact pre-slice SHA-256
`44aff5d4a23fcdebff00e442b07b2baa0b2cd8571756154d44097f089442e1a8`.

No real cache, runtime, resolver, strategy, workflow, image, Compose file, example, PowerShell file,
SDK, CLI alias, compatibility entry, package/state/clean/rebuild behavior, generated checkout state,
live service, Docker/VM/network resource, staging, commit, push, worktree, or successor was touched.
Generated validation artifacts exist only under `/tmp`. The engine/package boundary remains intact:
generic application code owns a narrowly scoped core-source hygiene rule without any package or
product-name special case.

Terminal disposition: `completed`. Demo/core untethering remains a separate product decision, not a
pending rank-10 action. No successor was created.

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

## Package-Compile Core Responsibility Split — 2026-08-15

Completed only the approved rank-1 application responsibility split in the saved dirty checkout.
Before mutation, branch `js/dev`, HEAD `6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream
relation 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 128 tracked dirty paths, 46
default-collapsed untracked paths, and status SHA-256
`6dcbcc093d5f1191926d84c6579c27581b9e89326e4b204ecaa85aafaef22720` matched the delegated
anchors exactly. This task record, `package_compile.go`, the complete `src` inventory, the absent
destination, and the protected ignored Python cache also matched every handed-off digest, mode, and
size.

Created `src/lib/application/package_compile_core.go` and moved byte-for-byte only
`cmdPackageCompileCore`, `runCoreSourceBuildTarget`, `defaultCoreSource`,
`seedCompiledCoreFromInstalledTarball`, `latestGlob`, `copyFileWithMode`, and
`packageCompileCoreUsageText`. The original file lost those declarations and its now-unused `io`
import only. Shared `fileExists` remains defined in `package_compile.go`; the compile dispatcher,
closure compiler, compile-all path, signatures, function bodies, help bytes, tests, APIs, schema,
manifests, and dynamic resolution remain unchanged.

The moved six-function block plus core-help bytes retain SHA-256
`1f40940e1f1b9c672df171916b29f959a7330b63f5593ad6e49c5752f7a68576`. Reconstructing the
original file from the two post-split sources, including the removed import, reproduces
`package_compile.go` SHA-256 `8552aab1424d1a065c740826abb5a66c257bba5c1293d29e808b5b287606cbc6`.
Every moved symbol has exactly one definition, caller/reference ownership is unchanged, and
reconstructing the sorted per-file `src` inventory while excluding the new destination reproduces
`d975ec0e6d3ce2da048e177236a0fed9c9077d6827e6c779702db5d69d0d2d31`.

Focused offline validation used cached Go 1.25.0 with `GOTOOLCHAIN=local`, `GOPROXY=off`,
`GOSUMDB=off`, and isolated `/tmp` build/temp roots:

- `TestCmdPackageCompileCore`, `TestCmdPackageCompileCoreRunsSourceBuildScript`,
  `TestCmdPackageCompileCoreSkipEmitsOperationResults`, and
  `TestCmdPackageCompileAllUsesCanonicalWorkflowRoots` passed together;
- Linux and Windows/amd64 application test packages compiled through non-executing `go test -c`
  commands; the Windows binary was not run;
- Go formatting, declaration/import ownership, `git diff --check`, exact source reconstruction,
  unrelated-`src` reconstruction, dirty ownership, and protected-cache checks passed.

The initial focused-test setup selected the host Go 1.22.12 launcher under `GOTOOLCHAIN=local` and
stopped at the workspace's Go 1.25 requirement before compilation. The first Windows compile setup
also stopped before compilation because its isolated `/tmp` directories were absent. After selecting
the cached Go 1.25.0 binary and creating only those temporary directories, each required check ran
once and passed. Neither setup-only stop changed repository state. Generated validation artifacts
exist only under `/tmp`.

Final dirty ownership is 128 tracked paths and 47 default-collapsed untracked paths with status
SHA-256 `b260e68217d8b7b07b448e5febe7fd5f6f973762c22d2ad85bbd25ad61d746d7`; no files are staged.
The ignored cache remains present and byte-identical at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No resolver/workflow/all compile responsibility, test, package boundary, behavior, API, CLI/help
text, schema, config, manifest, compatibility entry, state/cleanup surface, generated checkout state,
live service, Docker/VM/network resource, staging, commit, push, worktree, or successor was touched.
The engine/package boundary remains intact: generic application compile orchestration was split only
into an in-package sibling with no package or product-specific special case.

Terminal disposition: `completed`. Ranks 2-9 remain separately gated. No successor was created.

## Application Catalog Typed-Input/View Projection Split — 2026-08-15

Completed only the approved rank-2 application responsibility split in the saved dirty checkout.
Before mutation, branch `js/dev`, HEAD `6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream
relation 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 128 tracked dirty paths, 47
default-collapsed untracked paths, and status SHA-256
`b260e68217d8b7b07b448e5febe7fd5f6f973762c22d2ad85bbd25ad61d746d7` matched the delegated
anchors exactly. This task record, `catalog_cmd.go`, the complete `src` inventory, the absent
destination, the exact contiguous source block, and the protected ignored Python cache also matched
every handed-off digest, mode, and size.

Created `src/lib/application/catalog_inputs.go` and moved byte-for-byte only the contiguous block
from the PipeLang regex var group through `catalogFieldNameToEnv`, including its trailing separator.
The original file lost that block and its now-unused `bufio`, `regexp`, and `pipelang` imports only.
Catalog record structs, command parsing, workflow/resolver/core discovery, icons, rendering, text
output, `workflow_inputs.go`, callers, signatures, tests, APIs, JSON/CLI behavior, schema,
configuration, and manifests remain unchanged.

The moved declarations plus their trailing separator retain SHA-256
`abfe48f8a9ef8f68ff737c51137be438c867e7057dd02e682525343118f42727`. Reconstructing the original
file from the two post-split sources, including the three removed imports and the separator,
reproduces `catalog_cmd.go` SHA-256
`689b1119214082073c36f2477d5bf7bf20f37f00adfae1abf40f3e3c08db14a9`. Every moved top-level
declaration is present exactly once: the exact block is complete in `catalog_inputs.go`, absent from
`catalog_cmd.go`, and both focused testing and Linux/Windows package compilation reject duplicate or
missing package declarations. Reconstructing the sorted per-file `src` inventory while excluding
the new destination and substituting the reconstructed original reproduces SHA-256
`7b6e3f08d2bb022f703f38bdfbbd7bf73b541ce75a2e0d7cab4b958fc23199c4`, proving every unrelated
`src` byte unchanged.

Focused offline validation used the cached Go 1.25.0 binary with `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, and isolated `/tmp` build/temp roots:

- all six exact `TestListCatalogWorkflows*` tests passed together;
- Linux/amd64 and Windows/amd64 application test packages compiled through non-executing
  `go test -c` commands; the Windows binary was not run;
- `gofmt`, import/declaration ownership, exact block and source reconstruction, `git diff --check`,
  unrelated-`src` reconstruction, dirty ownership, and protected-cache checks passed.

The initial focused-test setup selected the host Go 1.22.12 launcher under `GOTOOLCHAIN=local` and
stopped before compilation at the workspace's Go 1.25 requirement. Selecting the already cached Go
1.25.0 binary allowed every required offline test and compile check to pass. The setup-only stop
changed no repository state, and generated validation artifacts exist only under `/tmp`.

Final dirty ownership is 129 tracked paths and 48 default-collapsed untracked paths with status
SHA-256 `cef8715de9c5c17ffed221eb375a572f137b87992cfced7677ff53de8808218b`; no files are staged. The
ignored cache remains present and byte-identical at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No record struct, command/list/icon/resolver/core/text ownership, workflow-input caller, test,
package boundary, behavior, API, JSON/CLI, schema, config, manifest, compatibility/state/cleanup
surface, generated checkout state, live service, Docker/VM/network resource, staging, commit, push,
worktree, or successor was touched. The engine/package boundary remains intact: generic catalog
typed-input and view projection logic moved only to an in-package application sibling with no
package or product-specific special case.

Terminal disposition: `completed`. Ranks 3-9 remain separately gated. No successor was created.

## Domain Workflow Validation Responsibility Split — 2026-08-15

Completed only the approved rank-3 domain workflow-validation responsibility split in the saved
dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 129 tracked
dirty paths, 48 default-collapsed untracked paths, and status SHA-256
`cef8715de9c5c17ffed221eb375a572f137b87992cfced7677ff53de8808218b` matched the delegated
anchors exactly. This task record, `workflow.go`, the complete `src` inventory, the absent
destination, the exact contiguous source block, and the protected ignored Python cache also matched
every handed-off digest, mode, and size.

Created `src/lib/domain/workflow_validate.go` and moved byte-for-byte only the contiguous block
beginning with the comment for `ValidateWorkflowTypeField` and ending after
`hostBuiltinNeedsCompose`, including its trailing separator. The original file lost that block and
its now-unused `path/filepath` import only. Workflow model declarations,
`workflowFile`/`stepOrGroupYAML`/`asyncGroupYAML`, YAML decoding and `flattenSteps`, step methods,
`ParseWorkflowYAML`, callers, exported signatures, tests, APIs, and YAML/CLI behavior remain
unchanged.

Reconstructing the moved block with its source separator retains SHA-256
`884a64ca0cfeec3183cc5074f66e37abac21785539ba1411353186bd4359e19c`. Reconstructing
`workflow.go` from the two post-split sources, including the removed import and separator,
reproduces SHA-256 `f4772ab6f597e4258a1945aa21f10c55549057fdfee8cdc06f5c953c030583d7`.
All 26 moved declarations have exactly one definition, the block is absent from `workflow.go`, and
package compilation rejects any missing or duplicate declaration. Reconstructing the sorted
per-file `src` inventory while excluding the new destination and substituting the reconstructed
original reproduces SHA-256
`9c80abee08014b095d11ae36af42a9e7ce8b74588a5ce172cdadd355861a516f`, proving every unrelated
`src` byte unchanged.

Focused offline validation used the cached Go 1.25.0 binary with `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, and isolated `/tmp` build/temp roots:

- all 54 exact tests across `workflow_test.go`, `workflow_helpers_test.go`, and
  `workflow_inject_test.go` passed together;
- the complete `src/lib/domain` package tests passed;
- Linux/amd64 and Windows/amd64 domain test packages compiled through non-executing `go test -c`
  commands; the Windows binary was not run;
- `gofmt`, import/declaration ownership, exact block and source reconstruction,
  `git diff --check`, unrelated-`src` reconstruction, dirty ownership, and protected-cache checks
  passed.

The initial focused-test setup selected the host Go 1.22.12 launcher under `GOTOOLCHAIN=local` and
stopped before compilation at the workspace's Go 1.25 requirement. Selecting the already cached Go
1.25.0 binary allowed every required offline test and compile check to pass. The setup-only stop
changed no repository state, and generated validation artifacts exist only under `/tmp`.

Final dirty ownership is 130 tracked paths and 49 default-collapsed untracked paths with status
SHA-256 `1c00be78e57c5b9cde22b4f1bbd4e94ed425db3686723936bcfb703322795f90`; no files are staged.
Removing only the owned `workflow.go` status entry and new destination entry reconstructs the exact
pre-split status SHA-256. The ignored cache remains present and byte-identical at mode `0664`, size
8,408, and SHA-256 `25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No workflow model, decode/flattening path, step method, parser, caller, test, package boundary,
behavior, API, YAML/CLI, schema, config, manifest, compatibility/state/cleanup surface, generated
checkout state, live service, Docker/VM/network resource, staging, commit, push, worktree, or
successor was touched. The engine/package boundary remains intact: generic I/O-free workflow
validation moved only to an in-package domain sibling with no new package, abstraction, import
direction, or product-specific special case.

Terminal disposition: `completed`. Ranks 4-9 remain separately gated. No successor was created.

## Infrastructure Git-Session Storage Responsibility Split — 2026-08-15

Completed only the approved rank-4 infrastructure Git-session storage responsibility split in the
saved dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 130 tracked
dirty paths, 49 default-collapsed untracked paths, and status SHA-256
`1c00be78e57c5b9cde22b4f1bbd4e94ed425db3686723936bcfb703322795f90` matched the delegated
anchors exactly. This task record, `git_runtime_session.go`, the complete `src` inventory, the
absent destination, all three exact source blocks, and the protected ignored Python cache also
matched every handed-off digest, mode, and size.

Created `src/lib/infrastructure/git_runtime_session_store.go` and moved byte-for-byte only the
three authorized storage blocks: list/load/root/sort helpers; JSON event, session, sync, publish,
worker-lease, and checkpoint persistence helpers; and `listWorkerLeases`. The original file lost
those blocks and its now-unused `errors` and `sort` imports only. All Git-session structs,
CreateSessionBranch, checkpoint/sync/publish/archive/cleanup and lease-policy lifecycle operations,
Git/Docker helpers, callers, exported signatures, JSON/event/receipt/path bytes, tests, APIs, and
CLI behavior remain unchanged.

The three moved blocks reconstruct SHA-256
`ffc6115c8311b133e7d00eb1066d2ae481817a4b361f24cafb3c00d9247ad928`,
`f1be13ce0b75305050e84ecbd27e829c74df1c207f455ffa29eb99f3ce104b03`, and
`c64ad11e49b16c32bac6fc0e70d23ee053e783ca876fdf0c8b41281639358c76`, including each original
trailing separator. Reconstructing `git_runtime_session.go` from the two post-split sources and the
two removed imports reproduces SHA-256
`3847597c6910cdbb5bb4505f97f0fc1ca58474ecf2fb6a9b04dd9a05ba16bb3f`. All 17 moved declarations
have exactly one definition. Reconstructing the sorted per-file `src` inventory while excluding the
new destination and substituting the reconstructed original reproduces SHA-256
`55712c7a16b37d75e7ca97a34ec0faad2fb8d325a7ece08b83e0d503bb8f6584`, proving every unrelated
`src` byte unchanged.

Focused offline validation used the cached Go 1.25.0 toolchain with `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, and isolated `/tmp` build/temp roots:

- all 14 tests in `git_runtime_session_test.go` passed together;
- all eight controlled-checkpoint tests and all five controlled-publication tests passed in the
  same focused run;
- Linux/amd64 and Windows/amd64 infrastructure test packages compiled through non-executing
  `go test -c` commands; the Windows binary was not run;
- `gofmt`, import/declaration ownership, exact block and source reconstruction,
  `git diff --check`, unrelated-`src` reconstruction, dirty ownership, and protected-cache checks
  passed.

The initial focused-test setup selected the Go 1.22.12 launcher under `GOTOOLCHAIN=local` and
stopped before compilation at the workspace's Go 1.25 requirement. Selecting the already cached Go
1.25.0 binary allowed every required offline test and compile check to pass. The setup-only stop
changed no repository state, and generated validation artifacts exist only under `/tmp`.

Final dirty ownership is 131 tracked paths and 50 default-collapsed untracked paths with status
SHA-256 `4e52d2698476a9c4d58ffcb230bd2875a08b4131ec1cfcd7d0255433f2e9e9d9`; no files are staged.
Removing only the owned original-source status entry and new destination entry reconstructs the
exact pre-split status SHA-256. The ignored cache remains present and byte-identical at mode `0664`,
size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No type, lifecycle/Git/Docker/cleanup/lease-policy operation, caller, test, package boundary,
behavior, API, JSON/CLI, schema, config, manifest, compatibility/state/cleanup surface, generated
checkout state, live service, Docker/VM/network resource, staging, commit, push, worktree, or
successor was touched. The engine/package boundary remains intact: generic Git-session storage
logic moved only to an in-package infrastructure sibling with no new abstraction, import direction,
or product-specific special case.

Terminal disposition: `completed`. Ranks 5-9 remain separately gated. No successor was created.

## Application Step-Container Preparation Responsibility Split — 2026-08-15

Completed only the approved rank-5 application responsibility split in the saved dirty checkout.
Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 131 tracked
dirty paths, 50 default-collapsed untracked paths, and status SHA-256
`4e52d2698476a9c4d58ffcb230bd2875a08b4131ec1cfcd7d0255433f2e9e9d9` matched the
delegated anchors exactly. This task record, `run_steps.go`, the complete `src` inventory, the
absent destination, all three exact source blocks, and the protected ignored Python cache also
matched every handed-off digest, mode, and size.

Created `src/lib/application/run_steps_container.go` and moved byte-for-byte only the three
authorized container-preparation blocks: the `buildStepContainer` comment and function;
`applyContainerPathEnv` plus `containerWorktreePath`; and
`runStepImageArtifactProvenance` plus `stepRunPolicyID`. No original import became unused. The
original file retains `runStepsOpts`, every type, `runSteps`, `applyWorkflowContainerMountEnv`,
scheduling, blocking/parallel workers, resolver delegation, outputs, host builtins, pre-scripts,
`mustGetwd`, and `parseStepArgv`. All callers, exported signatures, security/network/mount/image,
runtime-manifest, cross-platform path, API, CLI, schema, configuration, and behavior contracts remain
unchanged.

The three moved blocks reconstruct SHA-256
`1c56ec962c412b4edf3643072ecd5add0006159f259d6fd6776ea82d24dfa858`,
`5dd886b78a6a7920f49e15d357fb76bd0b469ee27cdcfc5ed648369411b5c434`, and
`144cefb5f42a2ea9d6f93c9336675831686e25d2af4a9fa34d8ad89a2b099c2b`, including the original
separators for the first two blocks. Reconstructing `run_steps.go` from the two post-split sources
and the separator before the original final block reproduces SHA-256
`df751ae44b7db4013c815e4126935c25ec2687b48aea9b089765100018e5150b`. Each of the five moved
declarations has exactly one definition. Reconstructing the sorted per-file `src` inventory while
excluding the new destination and substituting the reconstructed original reproduces SHA-256
`274fda3906ece39d46502515054d3a470a272e9d4e2ae65955b23ba276878e4a`, proving every unrelated
`src` byte unchanged.

Focused offline validation used an existing cached Go 1.25.0 toolchain read-only, with its module
cache copied under `/tmp`, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and isolated `/tmp`
module, build, and temporary roots:

- all 48 exact tests declared across `build_step_container_test.go`, `run_steps_test.go`,
  `run_steps_more_test.go`, and `run_steps_seams_test.go` passed together, including all 12 dedicated
  `buildStepContainer` tests;
- Linux/amd64 and Windows/amd64 application test packages compiled through non-executing
  `go test -c` commands; the Windows binary was not run;
- `gofmt`, import/declaration ownership, exact block and source reconstruction,
  `git diff --check`, task-record reconstruction, unrelated-`src` reconstruction, dirty ownership,
  and protected-cache checks passed.

The previously recorded user module-cache toolchain path was absent. The first two focused-test
setup attempts stopped before compilation because the isolated cache lacked
`github.com/mattn/go-shellwords`; the complete matching module was then populated from an existing
local `/tmp` file proxy, after which the exact test command ran with `GOPROXY=off` and passed. No
network, Docker, live service, or checkout-generated write occurred. Validation binaries and cache
copies exist only under `/tmp`.

Final owned source SHA-256 values are
`41df3029464a582551f1e1340f1adb235be12f3a4e299a61cc68d2e7f8b4cef6` for `run_steps.go` and
`f8402186985f01df64b5b29adc19a59f0e5636cad10b2d2705ab0f6d81576ba9` for
`run_steps_container.go`. Final dirty ownership is 132 tracked paths and 51 default-collapsed
untracked paths with status SHA-256
`2c822b9c251845830b9c61f9b039fb95ca9a53194b13b834745ef58c09bcbb77`; no files are staged.
Removing only the owned original-source status entry and new destination entry reconstructs the
exact pre-split status SHA-256. The ignored cache remains present and byte-identical at mode `0664`,
size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No scheduling/resolver/output/host/pre-script logic, type, caller, test, package boundary,
behavior, API, CLI, schema, config, manifest, compatibility/state/cleanup surface, generated
checkout state, live service, Docker/VM/network resource, staging, commit, push, worktree, or
successor was touched. The engine/package boundary remains intact: generic step-container
preparation moved only to an in-package application sibling with no new abstraction, import
direction, or product-specific special case.

Terminal disposition: `completed`. Ranks 6-9 remain separately gated. No successor was created.

## Application Workflow Git-Session Helper Responsibility Split — 2026-08-15

Completed only the approved rank-6 application responsibility split in the saved dirty checkout.
Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 132 tracked
dirty paths, 51 default-collapsed untracked paths, and status SHA-256
`2c822b9c251845830b9c61f9b039fb95ca9a53194b13b834745ef58c09bcbb77` matched the delegated
anchors exactly. This task record, `run.go`, the complete `src` inventory, the absent destination,
the exact source block, and the protected ignored Python cache also matched every handed-off digest,
mode, and size.

Created `src/lib/application/run_git_session.go` and moved byte-for-byte only the contiguous block
from `createWorkflowGitSession` through `timeNowSessionSlug`. The original file lost that block and
its now-unused `time` import only. `Run`, `effectiveWorkdirForWorkflowOpts`, all later helpers, every
caller and type, session/workspace lifecycle behavior, checkpoint and volume-cleanup policy,
operation-result bytes, paths, APIs, CLI behavior, and cross-platform contracts remain unchanged.

The moved block plus its original trailing separator reconstructs SHA-256
`52b769b2a8560b58b325fbe7b6c8888abc8344a1ae5e27c7cdce0f3fc2981170`. Reconstructing `run.go`
from the two post-split sources and the removed import reproduces SHA-256
`96249aef9aab4fba97b90ae115929b484e57ed2971894719318b44758190105f`. Each of the six moved
declarations has exactly one definition. Reconstructing the sorted per-file `src` inventory while
excluding the new destination and substituting the reconstructed original reproduces SHA-256
`17c523cde79ecd7cee93bff15f991e669c9d74e67f406eea4b3ae3765099491b`, proving every unrelated
`src` byte unchanged.

Focused offline validation reused the existing cached Go 1.25.0 toolchain under `/tmp`, with
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and module, build, state, and temporary writes
kept below `/tmp`:

- all 26 tests declared in `run_test.go` and all three tests declared in `session_cmd_test.go`
  passed together from the compiled Linux test binary;
- Linux/amd64 and Windows/amd64 application test packages compiled through non-executing
  `go test -c` commands; the Windows binary was not run;
- `gofmt`, imports, declaration ownership, exact block and source reconstruction,
  `git diff --check`, task-record reconstruction, unrelated-`src` reconstruction, dirty ownership,
  and protected-cache checks passed.

The first direct focused-test attempt exposed an inherited fixture mismatch: one test intentionally
uses the nonexistent placeholder `/path/to/your/project`, while the current durable-state setup now
requires that CLI workdir to exist. It also appended twelve records to three ignored event logs and
created five ignored policy files under the package-relative generated tree. The five new files were
moved intact to the task's `/tmp` recovery area, only those twelve appended lines were removed, and
the exact pre-test `src` inventory digest was restored before validation continued. The unchanged
test binary then ran from a writable `/tmp` cwd inside a local `bwrap` namespace that supplied only
the placeholder directory; all 29 tests passed. No network, Docker, live service, test-source
change, or persistent checkout-generated validation artifact remains.

Final owned source SHA-256 values are
`21908186cad40d32d36786f171998fb2947171079ca53d34d0a475e57f79221e` for `run.go` and
`ddd82a824ccd25ab2769a9c8fd7ab0bbbd2664f18ad2b16b4e634eee965e8461` for
`run_git_session.go`. Final dirty ownership is 132 tracked paths and 52 default-collapsed untracked
paths with status SHA-256
`d16306f7771e75f572a9745f0a20d2dada659c79be1e0252031640ed598a8eca`; no files are staged.
Removing only the new destination status entry reconstructs the exact pre-split status SHA-256.
The ignored cache remains present and byte-identical at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

No `Run` decomposition, later helper, type, test, package boundary, behavior, API, CLI, schema,
config, manifest, compatibility/state/cleanup surface, live service, Docker/VM/network resource,
staging, commit, push, worktree, or successor was touched. The engine/package boundary remains
intact: generic workflow Git-session orchestration moved only to an in-package application sibling
with no new abstraction, import direction, or product-specific special case.

Terminal disposition: `completed`. Ranks 7-9 remain separately gated. No successor was created.

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

## Application SDK Scope-Projection Boundary Audit — 2026-08-15

Completed only the approved read-only rank-2 boundary audit in the saved dirty checkout. Before
this documentation edit, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 57 default-collapsed untracked paths, and status SHA-256
`948183f18ab980683467508feebecf1ab8204a38806a6c28e328ea895cf955f2` matched the delegated
anchors exactly. This task record was
`f17cee2fae4aa46d9d2d9a397e2dfe8deb0dbb479f0f97004a4adaf08702688c`; the aggregate digest
of the sorted per-file SHA-256 inventory for every file below `src/` was
`ce7de671ec813e61702c5139c09b076c9b206db82143bbb2763efbb585e75388`. The protected
ignored Python cache remained a regular file at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

### Exact declaration and seam inventory

`src/lib/application/sdk_cmd.go` is 721 physical lines with SHA-256
`9a2fd29919c62b894f29bf315b0451bf6b7b38be6504d41c4c69f4317807f6ff`. The destination
`src/lib/application/sdk_scope.go` is absent. Fourteen scope-owned declarations occupy five exact
source-order blocks totaling 353 lines; concatenating the five blocks in that order has SHA-256
`69c82b20579d5dbb6a78d500eb13bd8bcb38ea25e316dc63bdda35e83ee867f4`.

| Exact block, including each trailing declaration separator | Lines | SHA-256 |
| --- | ---: | --- |
| `scopeUsageText` | 57-93 (37) | `357f80a2ec0a7e18e0ef13a6746e6e6dea9d290753a7c4f06ca0446f13366425` |
| `cmdScope`, both scope types, and `printScopeObject` | 254-344 (91) | `31067b344b06ba6f3b1b1006c656f1527da2fa666afe349a88baec1ff04016a5` |
| `parseScopeArgs` | 360-396 (37) | `ac87e0b0ea9327f27df6f8dc61dd9453f50592e5b700704e8c31d604000c5601` |
| workflow/package/resolver projections through `resolveOutputRoot` | 437-601 (165) | `289348c6082e7d25088d4a89ad0dc6da1cc7e2813bc100481996571e29fff7da` |
| `sanitizeNamedScope` | 632-654 (23) | `48f7349d8b2ab862d73ce7fc0dc98772eb7e354838ac078b2255c912824be3ce` |

The declaration-level inventory is exact and includes the blank separator following each
declaration:

| Declaration | Lines | SHA-256 |
| --- | ---: | --- |
| `scopeUsageText` | 57-93 | `357f80a2ec0a7e18e0ef13a6746e6e6dea9d290753a7c4f06ca0446f13366425` |
| `cmdScope` | 254-313 | `baa3534481e232a76f9bd841c4fbc62fee95a1f9ad0ce1684199da61895734e3` |
| `scopeArgs` | 314-320 | `7be34000c4ec628121fb83672a0e0515bb28b96c84ce7e5b101954ced0c2fa69` |
| `scopeObject` | 321-335 | `407bc853f0bc9e2d7ce58c403a0c89b8fafcd998f03a11987d8116b633381adf` |
| `printScopeObject` | 336-344 | `d2c2149493ef54c351fac2aae2ffc63f9afda685d3c2bdab7f3a4e95d035c9c4` |
| `parseScopeArgs` | 360-396 | `ac87e0b0ea9327f27df6f8dc61dd9453f50592e5b700704e8c31d604000c5601` |
| `workflowScopeObject` | 437-482 | `1b834a99d678479e58ed9fe14d53844e8d6933a9eae1d2a9aa016ff85a1e131a` |
| `packageScopeObject` | 483-506 | `c674d47b9b6b5a034c4e0fb83832c3b875599056b986f2c86f9092a296478e24` |
| `resolveScopeRoot` | 507-522 | `d70fbe366d1ea724c99e852c5070f2b5e6095df9f56f606fd241353c56da69f4` |
| `resolverScopeValue` | 523-552 | `d5994d7ea7ac7d236c505ea1bf4b5405140211a302fef9e0c59250fd1f222bf1` |
| `workflowScopePath` | 553-565 | `9cfbeceb25844c67c680e215fa616c48d7ce15047c9b90c43d946ddd9d129c6f` |
| `resolveResolverHostPath` | 566-590 | `4a2e6ebee0f4f22092936be9f1fbaccbbed9cbebf07a17f3525734454299276d` |
| `resolveOutputRoot` | 591-601 | `134e78f168c9eec6034ec37de40989f3b00acd87b16499bc1d6d5fb0b9700e5e` |
| `sanitizeNamedScope` | 632-654 | `48f7349d8b2ab862d73ce7fc0dc98772eb7e354838ac078b2255c912824be3ce` |

The seam is non-contiguous for source-backed reasons, not because scope ownership is ambiguous:

- lines 94-253 are the 160-line `cmdSDK`/`cmdGet` block, SHA-256
  `fdc08ae774d7959367f3ec8e411645d8ea63bc2fdd2e64cecd78f85a6230bfea`;
- lines 345-359 are the 15-line shell-environment command, SHA-256
  `0ec0cd016162acf3332f8867d07ee2026eed781187c28382f06b7871269edda8`;
- lines 397-436 are the 40-line shared SDK workdir parser/normalizer block, SHA-256
  `e539104880c1c3cab31268423e20134d00bd266f84a18c97130b32fdef6394f7`;
- lines 602-631 are the 30-line event-log/event-index path block shared by `cmdGet` and the
  workflow scope object, SHA-256
  `456b474c381f50577219d2106693e7074117412488176c36089b1385bf298fb9`;
- lines 655-721 are the 67-line binary selection, child-process fallback, field normalization,
  shell-SDK path, and quoting tail, SHA-256
  `10248002e8f3248f079ffc4ba7ce9774ade155b24246198e835cea90f083ae7a`.

The exact shared dependencies that must stay out of the move are `resolveSDKWorkdir`, used by both
scope parsing and `parseSDKWorkdirFlag`; `resolveEventLogPath` and `resolveEventIndexPath`, used by
both `cmdGet` and `workflowScopeObject`; `resolveDockpipeBinForSDK`, used by `cmdGet`, both scope
objects, and the child-process fallback; and `normalizeGetField`, used by `cmdGet` and scope
dispatch/projection. `workflowArtifactRoot` and `sanitizeWorkflowStateScope` remain in
`state_env.go`, where workflow env and CI-artifact callers also use them. The broadly shared
`firstNonEmpty` remains in `workflow_env.go`. Shell-only `cmdSDKShellEnv`, `resolveShellSDKPath`,
and `shellQuote`, plus `parseSDKWorkdirFlag`, child-binary selection, and repo-local binary lookup,
remain in `sdk_cmd.go`.

The future child needs `encoding/json`, `fmt`, `os`, `path/filepath`, `strings`, and
`infrastructure`. Only the existing `encoding/json` import is exclusive to the 353-line move; its
line has SHA-256 `926ebc76318f711b7e6fce77071eb1a689adb7f244f8cbde070b2852db74b02e`.
The parent retains every other import, including `os/exec`, so no dependency direction or package
boundary changes.

### Production callers and direct proof

`Run` in `run.go` is the sole external production caller of `cmdScope`; the complete SDK/get/scope
dispatch excerpt at lines 157-165 has SHA-256
`9f7596971b622778119ab5d404c7fbfaf27b5ace38f8ef23b481acb08b5221ee`. Every other moved
declaration is called only inside the candidate graph. The graph in turn calls the shared helpers
listed above plus generic infrastructure state, package-state, resolver-profile, and safe state-path
helpers. Repository package/workflow scripts consume the public `scope` CLI forms for artifacts,
workflow artifacts, package state, and resolver auth, but none is a Go declaration caller and none
needs to move or change.

All seven colocated SDK tests are in the unchanged 371-line `sdk_cmd_test.go`, SHA-256
`0315da7df102b84ed12f38925ffadd4e0b0e01acddf2bd5e692b302e71be3887`. The three direct
scope tests are `TestCmdScopeWorkflowAndPackageObjects`, `TestCmdScopeResolverAuthFields`, and
`TestResolveResolverHostPathUsesUserHomeWhenHomeEnvMissing`. The four shared-seam tests are
`TestCmdGetStateFields`, `TestCmdGetStateDirNormalizesGitBashWindowsWorkdir`,
`TestResolveDockpipeBinForSDKPrefersRepoLocalExe`, and
`TestResolveDockpipeBinForChildProcessPrefersCurrentExecutable`. No test or caller move is needed.

### Exact separately gated successor

Exactly one successor is selected: **TASK-035 application SDK scope-projection responsibility
split**.

- **Result:** create `src/lib/application/sdk_scope.go`; move byte-for-byte, in original order, only
  the five exact blocks and fourteen declarations inventoried above. Remove only those source
  blocks and the now-unused `encoding/json` import from `sdk_cmd.go`. Keep all shared helpers,
  dispatch, SDK/get/shell behavior, callers, and tests in place. The destination is an in-package
  responsibility sibling, not a new package or abstraction.
- **Behavior and boundary:** preserve every name, signature, declaration body, JSON tag and byte,
  CLI/help/error/log/environment/path byte, workdir and Git-Bash normalization, source/artifact/
  workflow/package/resolver projection, resolver-auth lookup/default, durable package-state import
  and divergence behavior, safe suffix join, package/runtime/state separation, child-binary
  selection, cross-platform behavior, and generic engine/package boundary.
- **Exclusions:** no shared-helper, caller, test, SDK/get/shell, state-env, workflow-env, child-binary,
  package-state, resolver-profile, infrastructure, Domain, schema, configuration, manifest,
  compatibility, cleanup/migration, generated, runtime, or durable-state edit; no new package or
  abstraction; no behavior/API/CLI/help/error/log/environment/path change; no live, network,
  Docker, VM, staging, commit, push, worktree, or additional successor.
- **Required proof:** retain every per-block and per-declaration hash plus the 353-line aggregate;
  reconstruct the original `sdk_cmd.go` byte-for-byte at SHA-256
  `9a2fd29919c62b894f29bf315b0451bf6b7b38be6504d41c4c69f4317807f6ff` by restoring the
  exclusive import and reinserting the five blocks at their exact anchors; prove each moved
  declaration has one definition and `Run` remains the sole external caller; run all seven named
  SDK tests together; compile the Linux and Windows/amd64 application test packages without
  executing the Windows binary; and verify Go formatting, import ownership, offline
  `go list -mod=readonly`, `git diff --check`, task/status/full-`src` reconstruction, every
  unrelated source byte, and protected-cache preservation.

Audit validation used cached Go 1.25.0 with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`,
`GOWORK=off`, and isolated `/tmp` build/temp roots. The first candidate module cache was correctly
rejected because pinned `golang.org/x/sys` v0.28.0 was absent. The existing complete matching cache
at `/tmp/dockpipe-vm-source-lane-a.fL2D8H1F/tmp/validation-mod` then passed
`go list -mod=readonly` for application and both internal leaves, Domain, Infrastructure and both
child packages without network fallback or an import cycle. Structural declaration, import,
caller, direct-test, CLI-consumer, destination-absence, exact-hash, dirty-state, and protected-cache
checks passed. No test suite was run because this audit changed no source behavior; the exact
successor owns focused and cross-platform compile proof. Generated audit files exist only under
`/tmp/dockpipe-task035-sdk-audit-01a007b3`.

A no-index comparison against the preserved task preimage shows one insertion-only hunk with no
removed or replaced line. Final status and full-`src` digests remain the exact anchored values
above; repository `git diff --check` and the TASK-only no-index whitespace check both pass.

This audit changes no Go source, test, caller, helper, schema, configuration, manifest, API, CLI,
package, workflow, generated, ignored, runtime, durable, or external byte. The generic
engine/package boundary remains intact.

Terminal disposition: `completed`. The selected successor was not implemented, handed off, or
created. It requires the exact separate user response
`Approve next slice: TASK-035 application SDK scope-projection responsibility split` before any
fresh-task execution.

### SDK scope-projection responsibility split completion

Completed only the approved TASK-035 application SDK scope-projection responsibility split in the
saved dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 57 default-collapsed untracked paths, and status SHA-256
`948183f18ab980683467508feebecf1ab8204a38806a6c28e328ea895cf955f2` matched the delegated
anchors exactly. This task record was
`59c73057892fe0099dfc5d236c491d03e7125f31a8f020f6780d8ff327d307bf`; the complete sorted
per-file `src` inventory, `sdk_cmd.go`, absent destination, exact candidate blocks/declarations,
and protected ignored Python cache also matched every handed-off digest, type, mode, and size.

Created `src/lib/application/sdk_scope.go` and moved byte-for-byte, in original order, only the
five audited scope blocks and fourteen declarations. `sdk_cmd.go` lost only those blocks and its
exclusive `encoding/json` import. Every shared workdir, event-path, binary, normalization,
workflow-artifact, workflow-state-scope, and shell helper remains in its audited owner; dispatch,
callers, tests, SDK/get/shell behavior, infrastructure and Domain code, package-state and resolver
semantics, APIs, JSON/CLI/help/error/log/environment/path bytes, and cross-platform behavior remain
unchanged. The destination is an in-package responsibility sibling and introduces no package,
abstraction, or dependency-direction change.

The five blocks reconstructed with each original declaration separator retain their exact
37/91/37/165/23-line SHA-256 values
`357f80a2ec0a7e18e0ef13a6746e6e6dea9d290753a7c4f06ca0446f13366425`,
`31067b344b06ba6f3b1b1006c656f1527da2fa666afe349a88baec1ff04016a5`,
`ac87e0b0ea9327f27df6f8dc61dd9453f50592e5b700704e8c31d604000c5601`,
`289348c6082e7d25088d4a89ad0dc6da1cc7e2813bc100481996571e29fff7da`, and
`48f7349d8b2ab862d73ce7fc0dc98772eb7e354838ac078b2255c912824be3ce`.
Their 353-line source-order aggregate remains
`69c82b20579d5dbb6a78d500eb13bd8bcb38ea25e316dc63bdda35e83ee867f4`, and all fourteen
declarations retain the exact individual hashes in the audit table above. Reconstructing the
original parent by restoring `encoding/json` and reinserting those blocks at the five audited
anchors reproduces the original 721-line `sdk_cmd.go` SHA-256
`9a2fd29919c62b894f29bf315b0451bf6b7b38be6504d41c4c69f4317807f6ff`.
Each moved declaration has exactly one definition, and `Run` remains the sole external production
caller of `cmdScope`.

Focused offline validation used a cached Go 1.25.0 binary and the existing complete matching module
cache with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`, and isolated `/tmp`
build/temp roots:

- `go list -mod=readonly` passed for application and both internal leaves, Domain, Infrastructure,
  and both infrastructure child packages without network fallback;
- all seven exact `sdk_cmd_test.go` tests were selected together: five passed, while the
  Windows-only Git-Bash normalization case and the environment-gated `os.UserHomeDir` fallback
  reported their existing skips;
- Linux/amd64 and Windows/amd64 application test packages compiled through non-executing
  `go test -c` commands; the Windows binary was not run;
- `gofmt`, import/declaration/helper/caller ownership, all block/declaration hashes, parent and
  full-`src` reconstruction, dirty ownership, `git diff --check`, and protected-cache checks passed.

After the move, excluding only the new destination reconstructs the original 133-tracked/57-
untracked status at SHA-256
`948183f18ab980683467508feebecf1ab8204a38806a6c28e328ea895cf955f2`. Excluding that destination
and substituting the reconstructed parent reproduces the original sorted per-file `src` inventory
SHA-256 `ce7de671ec813e61702c5139c09b076c9b206db82143bbb2763efbb585e75388`, proving every
unrelated source byte unchanged. The protected cache remains a regular mode-`0664`, 8,408-byte file
at SHA-256 `25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.
Generated validation artifacts exist only under `/tmp/dockpipe-task035-sdk-split-01a007c7`.

No shared helper, caller, test, SDK/get/shell, state/workflow environment, infrastructure, Domain,
package-state/resolver-profile, behavior, schema, configuration, manifest, compatibility,
cleanup/migration, generated/runtime/durable, live service, network, Docker/VM resource, staging,
commit, push, worktree, handoff, or successor was touched. The generic engine/package boundary
remains intact.

Terminal disposition: `completed`. No successor was selected, created, or implemented; any later
TASK-035 slice remains separately gated.

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

## Single-Workflow Package Compilation Responsibility Split — 2026-08-15

Lifecycle contract terminal state: `completed`. Fresh-task creation supplied
`execution_authority: approved_task_creation` for only the selected single-workflow compilation
split. One-shot readiness remained `required: false`, `status: not_applicable`. No excluded live,
generated, durable, cleanup, compatibility, migration, staging, Git, network, Docker, VM, handoff,
or successor action was taken.

Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, origin `js/dev`
`115fb785f0c4dc789ffce8c93c79fe147ea58229`, 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, zero staged files, 133 tracked dirty paths, 58
default-collapsed untracked paths, and status SHA-256
`5e8f318e6f1b291e022bf78f2fe2ed0eb616b71fb19940a36d27f26a88752176` matched the delegated
anchors. This TASK was SHA-256
`48d579b28c488423615cadc75690800a0ee48c3e78d2307f48891078bf8ef1fc`,
`package_compile.go` was 869 lines at SHA-256
`03373e9034f34e6f0772270edf16e38d6bc9268c6983b9e08a1bacd16022d085`, the destination was
absent, the exact lines 82–328 block was
`ff02ffb88b02b3c693260e5cd7a25525241c37fa21ae6ff4441a1dbd82e58ef3`, and the exclusive YAML
import line was `9a31bac62ac062ba0fa459b0e216501cece27853cb1db62ebaebf40761ddf1d8`.
The complete sorted per-file `src` inventory remained
`801fd0f559acef3647f733d753fc0b6d03997ce2ddcf95a8f43e627b3596a2d3`; the protected ignored
cache remained a regular mode-`0664`, 8,408-byte file at SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

Created `src/lib/application/package_compile_workflow.go` as an in-package sibling. It contains
only the package/import header plus the three declarations `cmdPackageCompileWorkflow`,
`compileWorkflowOne`, and `runWorkflowCompileHooks` in original order. Its lines 17–263 reproduce
the exact 247-line block SHA-256
`ff02ffb88b02b3c693260e5cd7a25525241c37fa21ae6ff4441a1dbd82e58ef3`; its YAML import line
retains SHA-256 `9a31bac62ac062ba0fa459b0e216501cece27853cb1db62ebaebf40761ddf1d8`.
The formatted destination is 263 lines at SHA-256
`54f8debc2ae81af52556f05b8899c1fba361db099fd65518bc89504612f49ab9`.
The parent is 619 lines at SHA-256
`58a6c99b4534982877a3221085dccba041915df20c6c891c1ca88d5f55c680ff`; formatting removed only
the redundant separators made adjacent by the exact block/import removal. Reinserting the exact
block, separator, and exclusive import reconstructs the original parent byte-for-byte at
`03373e9034f34e6f0772270edf16e38d6bc9268c6983b9e08a1bacd16022d085`.

Each moved declaration has exactly one production definition. The dispatcher and batch callers
remain in `package_compile.go`; dependency-closure and PipeLang materialization callers remain in
`package_compile_closure.go` and `pipelang_materialize.go`. Those four caller lines retain combined
SHA-256 `9abccf7ed9c84f2b64c0a9758ecfca3c1a82cc7e0b509a8fb3f446ada8fddad8`.
The sorted inventory of every unrelated `src` file remains
`639c25d374dda1e8c71dcf68de6f8f38805ecdf1684097c968dd1e164a476790`; substituting the
original parent hash and omitting the new sibling reconstructs the complete pre-split `src`
inventory at `801fd0f559acef3647f733d753fc0b6d03997ce2ddcf95a8f43e627b3596a2d3`.

Cached Go 1.25.0 validation used `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`,
and slice-owned `/tmp` module/build/temp roots. The 13 named direct tests in the required-proof
list passed together (`ok dockpipe/src/lib/application`, 96.387s). Two preceding setup probes
stopped before compilation because the inherited module cache was read-only/incomplete; the
passing invocation used the already cached dependency archives copied into the isolated module
cache and made no network request. Linux/amd64 and Windows/amd64 application test packages both
compiled with `go test -c`; the Windows binary was not executed. Offline `go list -mod=readonly`
resolved application and both internal leaves, Domain, Infrastructure and both child packages,
and PipeLang. Go 1.25 `gofmt -d` emitted no diff, import ownership is exact, and repository
`git diff --check` passes.

The compile-only binaries exist only under
`/tmp/dockpipe-task035-workflow-split-01a007fa/`: Linux SHA-256
`6b3f04992b75c1ad7ce2d34963920519961921fa8e902b6121a8f21b9db15bf4` and Windows/amd64
SHA-256 `80d27be8c83eb4939e130bfb430adc780b71911c8b714756d5030b08032910e2`.
No generated checkout artifact was created. Final checkout status is zero staged, 133 tracked
dirty paths and 59 default-collapsed untracked paths at SHA-256
`3c8fe5a18ed7ea866a615fa32524a687ec0f20a2caa25570942059f637d12a05`; removing only the new
sibling status entry reconstructs the inherited status digest
`5e8f318e6f1b291e022bf78f2fe2ed0eb616b71fb19940a36d27f26a88752176`.
The protected cache retains its anchored type, mode, size, and hash. Removing only this terminal
section reconstructs the pre-implementation TASK at SHA-256
`48d579b28c488423615cadc75690800a0ee48c3e78d2307f48891078bf8ef1fc`.

This was a byte-preserving in-package responsibility move: no behavior, API, CLI, help, error,
log, operation-result, environment, path, parsing, freshness/rebuild, legacy-store, cache,
staging, hook, PipeLang, runtime/security/image artifact, manifest, namespace, tarball,
cross-platform, helper, caller, test, package, or authored-surface contract changed. The generic
engine/package boundary remains intact. No successor was selected, created, or implemented.

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

## Application/Domain/Infrastructure Package Topology — 2026-08-16

State: `completed`. Objective `TASK-035-package-topology` replaces flat shared namespaces with
cohesive Go packages while leaving root Application, Domain, and Infrastructure as public
facades/composition surfaces. The completed Domain vocabulary and Model/Domain boundary proof is
admitted unchanged. Automatic reversible checkpoints are authorized; behavior, schema, authored
surface, compatibility retirement, feature work, generated/durable/live state, migration, network,
Docker, VM, credentials, staging, commit, push, publication, worktree, external actions, and
unrelated cleanup remain excluded.

Admission matched the clean saved-checkout anchors: branch `js/dev`, HEAD
`4e71cdeeb2c43aeb43296e23afcb8dc94fe7af6e`, origin `js/dev`
`3fe03013e695b7b13b62221bd7ccbc6ad4334e00`, 0 behind/2 ahead, both inherited stash objects,
zero staged/unstaged/untracked paths, empty status SHA-256
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, and record SHA-256
`5434d637ef58900b245761cef4c09de2821d73c863cc6671e8b88375cf821349`. The root production
inventory was and remains 65 Application, 20 Domain, and 58 Infrastructure Go files. The protected
Python cache and the two inherited empty mode-`0664` checkout cache locks retain their admitted
type, mode, size, and hashes.

### Checkpoint 1 — session command owner

Selected and completed `application/internal/sessioncmd` as the sole owner of session list,
inspect, switch, checkpoint, publication, worker-lease command parsing, presentation, and the exact
usage surface. Its only public API is `Run(args []string) error`; the six-line root
`session_cmd.go` compatibility dispatcher preserves the parent caller. The child imports the
narrow `shellquote` and `textvalue` leaves plus Domain and Infrastructure, and does not import
parent Application. Runtime-owned Git checkpoint/publication/lease behavior remains in
Infrastructure and is only invoked by the command owner.

The parent `firstNonEmpty` compatibility helper now delegates to
`textvalue.FirstNonBlank`, whose direct test proves that selection preserves the original untrimmed
bytes. The shared POSIX quoting algorithm moved from the SDK command into the narrowly named
`application/internal/shellquote` owner; both SDK and session commands depend downward on its
tested `POSIX(string) string` API. No generic helper or command bucket was created.

Reversing only package/API qualification changes reconstructs the original 641-line
`session_cmd.go` exactly at SHA-256
`779685f4c086a9b933c64a2eb0e4bd610b74b5150ec76f1d7acc5f1ca1806487`. Removing the one added
dispatch/worker-validation test and child-local stdout capture, then reversing the same package/API
qualification, reconstructs the original 258-line test exactly at SHA-256
`1bfe6f1ef41676c405a33b1324d706626ba3a66c9c20a4c3fdbcf736c1810860`.
Current owner hashes are `acd4ced308ed36c1c0bb2dde34ea109e15c03109d1d5d4d4553681d4ebfb205f`
for `session.go` and `4efd3e92ac50ca9576d8f36cfa2cb187fbf91f8f263241d0571e79a460f608da`
for its colocated tests.

Cached-offline Go 1.25.0 proof passed for sessioncmd, shellquote, and textvalue; all three moved
session integration tests; the new exact usage/unknown/worker-validation test; and affected SDK,
scope, and first-non-blank parent callers. Offline list and vet passed for Application, the three
affected leaves, Domain, Infrastructure, and Model owners through the admitted `/tmp`-only
compatibility module/cache. Linux and Windows/amd64 compile-only binaries passed for both
sessioncmd and Application; Windows binaries were not executed. Their SHA-256 values are
`a94b7617cef75d512ea0cc9b49b956988aa1c7acc78a16b0ff12ea549ae94c5e` /
`3ba4c81c0d913ca3291cc5abfe7b2b90cf67d849c3b81eb9f5944dca29d5df45` for sessioncmd and
`f86b0ca93a46ef33d61b9aaef4dba2c7a3cdcd20db5bd00a901858e2d9b070c8` /
`c3b19459775feb752d1ec88144713966943d2f6daeeecea17c657610a18c39d4` for Application.
`gofmt`, `git diff --check`, declaration uniqueness, dependency direction, and protected-state
checks pass. Verification artifacts exist only under `/tmp/dockpipe-task035-package-topology/`.

Topology selection after this checkpoint is pending a fresh caller/dependency/test audit. The
session owner is selected/completed; the previously completed Model wire owners remain selected and
complete; the existing Application internal leaves remain admitted downward owners. No additional
Application, Domain, or Infrastructure root cluster is selected merely from size.

### Checkpoint 2 — operation-result command owner

The live caller/test audit selected and completed `application/internal/resultcmd`. It solely owns
canonical operation-result parsing, validation, stderr rendering, event-log mirroring, environment
override/restore, and the exact help text behind `Run(args []string) error`. The root
`result_cmd.go` is a six-line dispatcher retained for the main Application command switch and its
existing dispatch test. The child imports only Infrastructure and does not import parent
Application. Existing parent integration tests plus a colocated validation test pass.

Reversing package/API qualification reconstructs the original implementation exactly at SHA-256
`d0e3d71b1ec014625f23d61ece2c7a3b448f2f2301c9f12f37ddf42f3e9589db`. Current implementation
and colocated-test hashes are `db53af6db65129546a12a17d7c9d265caaad5445e53c25ce23946e96bde967ac`
and `e405879ff9dc8460bf771ac48d0dda451251c4eb2b575598c8df3bdb49f81491`.
Cached-offline Go 1.25.0 focused tests, list, vet, and Linux/Windows compile-only proof pass for the
owner and parent Application. Binary hashes are
`56e80d279a2960ad0c70db02ad5c0eda6e77408dbe729e0ae163eade6be917e3` /
`a6f1018a92ce2f3ea54511f6424583f39f92bef6bf593cf6cefa91d99868851f` for resultcmd and
`27372a7e7374d12ae08c25d67f1b3cbaa28895dae4fcd905f1074dde80594453` /
`73e92ed25602bfd80bf8d1b49122496367fc8934e31263dc84b8290c1a062bdb` for Application. The first
vet setup stopped before analysis because its new `/tmp` `GOTMPDIR` did not yet exist; creating
that directory and rerunning the identical offline command passed. `gofmt`, diff, reconstruction,
declaration, dependency-direction, and protected-state checks pass.

The next selected low-risk vertical slice is the hidden internal-state command: its only production
caller is the parent dispatcher, its five direct tests cover durable-cohort, disposable runtime,
private-directory, collision, traversal, and link behavior, and its implementation depends only on
Infrastructure plus the standard library. Clone, runs, doctor, Windows, Terraform, package-image,
and test/build command surfaces remain under live API/caller audit and are not selected yet.

### Checkpoint 3 — hidden internal-state command owner

Selected and completed `application/internal/internalstatecmd` behind the sole public child API
`Run(args []string) error`; root Application retains only its six-line hidden-dispatch adapter. The
owner imports Infrastructure and the standard library, never parent Application. All five direct
tests moved beside the implementation with a test-local stdout capture and pass, covering exact
durable-cohort result fields, collision-safe disposable runtime paths, non-mutation, private modes,
malformed mappings, traversal, duplicate flags, and symlink rejection.

Reversing package/API qualification reconstructs the original implementation exactly at SHA-256
`7d7da5fa60bfbdda9ff548bd52707d04690cd2cc581ea7b78d2c470a4464d0cc`; removing only the
test-local capture and reversing the same qualification reconstructs the original tests exactly at
`c8a2aeebce5c1dc6ee73bb1218ea8d07bc9c86f64f981c6ac449842f5dfa9380`. Current implementation
and colocated-test hashes are `b0313afd8d672972e9ce290c9cccd1dab83a6f815b28291f095e093287f7d4c5`
and `f61b7b4b66d00aa9d12b7199f5a9782d3d227c34249e979a8a104c300fcf9401`.
Focused tests, offline list/vet, and Linux/Windows compile-only proof pass for the owner and parent;
binary hashes are `76355fdb7fcb04e55bd9a5eb3a50b27468fd4f6971d63b1c30c0a74cfe9157aa` /
`92594ffff471e9983e983e54d497686d10aa42c112a3cf22b86bb952931caf5f` and
`eafbd8565cf88771f3d220957358ba3c17183e643bbf24f3f1d50c771ebe4f33` /
`0eb1d01266b98bb78d0aa77620879c6aa923c3cfbacf4973b54f4a407a4ac277`.
No durable or disposable checkout state was changed by implementation or proof.

The live Application command audit now selects three remaining independent vertical owners in
order: doctor, clone, then runs. Each has one parent dispatch caller, no production dependency on a
parent Application type/helper, direct behavior tests, and a narrow `Run` boundary. Result and
internal-state are complete. Terraform remains unselected pending the explicit ownership decision
for shared `CliOpts` translation; package-image remains unselected pending ownership of the shared
Docker image-existence seam; package/workflow test and package-build remain coupled through parent
target/config/build adapters; catalog input projection remains coupled to parent catalog wire
records; project build/clean/rebuild remains one guarded destructive-safety transaction; run/steps,
dependencies, and strategy remain parent orchestration state. Windows command/bridge remains under
cross-platform public-facade audit rather than being selected by size.

The Domain map remains as admitted by the completed Model boundary: root Domain is the public
facade plus cohesive validation/value/workflow graph behavior; moving workflow graph/inject would
create a Domain-value cycle, and filesystem loaders require a separately designed Infrastructure
API. The Infrastructure audit currently finds its large durable-state, Git-session, Docker,
workflow-store, and path/layout families internally cohesive but mutually dependent on root helper
surfaces; no child is selected until an exact facade/API plan proves it will not import parent
Infrastructure. Existing `fetchinstall` and `packagebuild` remain valid downward children.

Next selected checkpoint: `application/internal/doctorcmd`.

### Checkpoint 4 — doctor command owner

Selected and completed `application/internal/doctorcmd` with `Run(args []string) error`; root
Application retains only the dispatcher facade. The child owns all doctor prerequisite checks,
operation-result records, exact help/output/error bytes, and its test seams. The former shared
`opLookPathFn` test seam was not a production ownership requirement: doctor now has the narrowly
owned `doctorOpLookPathFn`, while workflow secret injection retains its independent seam. The child
imports compileconfig, Domain, and Infrastructure downward and never parent Application.

Both direct doctor tests moved beside the owner and pass. Reverse transformation exactly rebuilds
the original implementation at SHA-256
`e7008a06bd40969e1142fe1fe96b1e63b9685af5e92f99bcb3ff1531ae925f1d` and its tests at
`2fde29cbe6a64fa5496107514be82df4b31c01f62f0528fc0b5dee85f9144b02`. Current implementation
and test hashes are `18c06256479a443e3d96819d73d10ad7631997e881dcab5acd5b2ea6622a9185`
and `5107d3d5b94bea071caaafd2c0d7b3e4bbedd25cf26cd953792d2b1e6f37c126`.
Offline list/vet and Linux/Windows compile-only proof pass for doctorcmd and Application. Binary
hashes are `b9f1a781f95ca04ed6fd508294134197e8cfe8cee88640c46a88916ac1a3eec6` /
`4bf8b1a57ee48052da27f6e9c7fe43cc99cde9a0b3075e92c477869fed9e38b6` and
`89a1ef50fc40f0c83240066513f75e80c002ea0c408bde45d630bb1bf25e0ab3` /
`cc4df12288437bb4d4a4eb1bf2ac4cf4286144183d6936e907b6c5f77cb47ddc`.
`gofmt`, diff, declaration, reconstruction, and dependency-direction checks pass.

Next selected checkpoint: `application/internal/clonecmd`.

### Checkpoint 5 — clone command owner

Selected and completed `application/internal/clonecmd` with the narrow `Run(args []string) error`
API and a root dispatch facade. The child owns compiled-workflow selection, manifest clone policy,
destination safety, extraction, copying, operation-result logging, and exact help/error bytes. Its
only former parent helper dependency was `copyDir`; the owner now imports the existing downward
`treecopy.Copy` primitive that already backs that exact parent facade. It otherwise imports Domain,
Infrastructure, and packagebuild, never parent Application.

The existing two parent integration tests still prove compile-to-clone success and facade denial.
A colocated owner test independently constructs the denied compiled tarball and proves direct API
policy enforcement. Reversing package/API qualification and the exact treecopy facade substitution
reconstructs the original implementation at SHA-256
`9ce00aea58029ac81bbdc8abd30a182f8c4240435631bfbf034ab1bbd6874710`. Current implementation
and test hashes are `1306fbcfd1225ecccab9b28d20198c2ba35fa7a634b6efe986b7908806233057`
and `592fc775666e52d39377e41b9463f7aff97183c58b23607f1758499b32919c21`.
Focused owner/caller tests, offline list/vet, and Linux/Windows compile-only proof pass; binary
hashes are `d90780c4b8ec418c224ec72063a3f0c806d034a71b048ec1a09d4668542e5394` /
`37009e9fb5474adb096008325e0cd764e14911d9d8a1e02e9ba86fedef50649e` and
`41e02e1f86eebe92ea8f4eef6f915f695bedc5217644482fd2ade5df67ae90b1` /
`a0bbddbb2002dfd4450c97d27dd45523f48fce058c06fd4a5d3cc887a5dd4bc9`.

Next selected checkpoint: `application/internal/runscmd`.

### Checkpoint 6 — runs inspection command owner

Selected and completed `application/internal/runscmd` with `Run(args []string) error`; the parent
file is only a dispatcher. The child owns host-run/policy/event selection, filtering, indexing,
text/JSON rendering, exact errors, and the exact runs usage text moved from the root usage
composition file. Its former `firstNonEmpty` dependency now uses the byte-preserving
`textvalue.FirstNonBlank` leaf. The child otherwise imports only Infrastructure and never parent
Application.

All existing `TestCmdRuns*` parent integration tests and two colocated owner tests pass. Removing
the moved usage block and reversing package/API/helper qualification reconstructs the original
command exactly at SHA-256
`e90376f0ccd14ea8cd52219f06e75495a42295d9441ee078c4ea6c2533bd0f7d`; the moved usage block
itself remains exact at SHA-256
`33374a7df7ebc8a0ea67300732fe0b5f72a48e5bfc2ae567a79ca995ac907123`. Current implementation
and test hashes are `541707dbcfcba8d2306e86f6d32c2fdf56f40e18ce27d8e16aec4e3889ba7a98`
and `546fc37419b58ab8db2d7dedb4382bcd1adafa9df1bb98bfb419c375371140ad`.
Offline list/vet and Linux/Windows compile-only proof pass for runscmd and Application. Binary
hashes are `0132b9e3daea5c4ccbab7ae25de2f259d41d6a8553da28305b0ebbe6a25664fe` /
`55599424e3022bf40c2225b3acd0ecda12740108c2d0c74c6717492a0aad5f17` and
`e8a04506caedb50a1a60b0325a1a9248fa0cfed8d96ffc99a71a87c9d8614147` /
`19abddb9b78ed83d662ed7593b411198ab880e8f80ae2cbcc28976f86ea3dbc7`.

The Infrastructure audit selects one low-risk independent owner:
`infrastructure/envfile` for dotenv file/byte parsing. It depends only on the standard library,
has a direct parsing test, and can retain root `ParseEnvFile`/`ParseEnvBytes` compatibility
wrappers without any reverse import. Operation result/event is not selected: it depends on root
terminal/spinner/time seams (`StartLineSpinner`, `fdInt`, `isTerminalDockerFn`, and
`timeNowDockerFn`), so extraction requires a named lifecycle-dependency API rather than a move.
Removal safety similarly depends on the durable-state device identity seam. Next selected
checkpoint: `infrastructure/envfile`.

### Checkpoint 7 — environment-file parser owner

Selected and completed `infrastructure/envfile`. The standard-library-only child owns file, byte,
and reader parsing; root Infrastructure retains source-compatible `ParseEnvFile` and
`ParseEnvBytes` wrappers. The original file implementation, byte implementation, and test each
reconstruct exactly at SHA-256
`0ee59a9e3082837ce8ceec40c72673e7287b6f6d10e4513ae13906fca27095bf`,
`7f1cee82831cbf410dbca4b64249ccfea2541d7d9494824de04119b013b99748`, and
`e69466fb327eed7a483be3043ae62450aa4513ff5c2a9f5f1c0337df3c87ec55`. A new colocated byte
parser test covers the previously untested public wrapper path. Current child hashes are
`10dc22d2f1c32d5dbe7b544fa75351f4192936c2904c988dfa672bace23c43e3`,
`1ec2238a4d22e9f1c703ef61f26f7d7e35032aa1cd710d86bc2307d6ba2abe89`, and
`17bba6c88de74d6eee07bad7c03a1d50998b1a52c05ef460d5c274f82a80e06f`.

Focused child and Application caller tests, offline list/vet, and Linux/Windows compile-only proof
pass for envfile, root Infrastructure, and Application. Binary hashes are
`c76dd2315c18bc9b1c473f3a842f2e9d688930c3fcf7c891e7b4dab831cdb13d` /
`bb891f51cff446dc5dc6b98982a45dab42653e68b64f8f8ede1a4fa52f24a464`,
`2a8ac7ab95f554c5aa3a4bf91801ebd316e6f91d59493e469ad8bc76bb421aa8` /
`53e095cceb94d358f5552eeb0e0138b4218843ba0d3acec0a90913a1c71d06d6`, and
`363388de02cffca28250d253e1f7a3a41ed873308f9e844fa16c95d1767dd510` /
`f44a712f05d418e4725276ef35b5de316461aa890ee7c59c581b2478a003029e`.
The Infrastructure root production inventory is now 57 files.

The final simple-owner pass selects `infrastructure/sourcemtime`: its three source freshness/path
selection functions are standard-library-only, directly tested, and consumed through a stable
three-function root API by packagecompile. Resolver/strategy file loading remains a deliberately
small cohesive root surface because splitting its shared assignment grammar would either make the
strategy owner depend on resolver semantics or introduce the prohibited generic parser bucket.
Next selected checkpoint: `infrastructure/sourcemtime`.

### Checkpoint 8 — source freshness owner

Selected and completed `infrastructure/sourcemtime`. The standard-library-only child owns maximum
source-tree modification time, source-versus-reference freshness, and newest-path selection; root
Infrastructure preserves the three exported wrappers consumed by packagecompile. Implementation
and tests reconstruct exactly at SHA-256
`bd19bdf53cff3dc53a6d317f0fa81a054bfc6fdbc8531ddd868b565ec04804cf` and
`e5dfacfd7522cd74e42204bd8428f569f194b3d347bfeb53d3072841deecb28b`.
Current child hashes are `bb8704d726b0fb9175f94833d32c360c5dc047388eab421bd601c7c44689db40`
and `deabbe6c4e23be79f2af61d46395b38f2c7d07dd29601b9c65ddf18e71b07c9d`.
Focused owner proof, offline list/vet, and Linux/Windows compile-only proof pass for sourcemtime,
root Infrastructure, and packagecompile. Binary hashes are
`d984d39aeaea253cc9b6fa842bb87e061c7129e090227e946bf719862cb1fc89` /
`c1028c8acfb6b4046826b37f3452a5f683701dc8d70b814ed63f30ed7524d2f8`,
`b0416eba78cd2eeab2733cf9899febcd36d71553eb77ec78f71ff1d27e1ae446` /
`9166f19370e35a7b53bba375a109c9be4a595318dcc04edaf34d05458d03dfae`, and
`7813451d1d76d217820fa9f73162fe6cf568aa903c2161f29119c4618bfc51b2` /
`368d7f7c0ce2703c8d0473a05a76a8b0a5cb8d5a60c9ee215b081a17689d7b5d`.

### Checkpoint 9 — operation-record owner and fetchinstall direction repair

Selected and completed `infrastructure/operationrecord` as the sole owner of operation result
types/options, stable stderr records, progress heartbeats, JSONL events and indexes, configured event
mirroring, and line-spinner lifecycle. Root Infrastructure retains source-compatible aliases,
constants, and wrappers for every exported operation API. The child owns independent
`time.Now`/`term.IsTerminal` seams plus its own safe file-descriptor conversion with the same
production defaults; it imports only the standard library and `golang.org/x/term`, never parent
Infrastructure or Application.

The two direct operation test files moved beside the owner. Reverse package/seam qualification
reconstructs the original operation-result implementation and test exactly at SHA-256
`c6c24c67bfdb2c0d9733c3be68dc6833d4bbec23bc968e206c296c2960a34270` and
`c2f9de22858446696a383a0ed7e8eccc7598655b24277890ae1a0448dcbcc0fc`; the event implementation
and test reconstruct at `cd077033cf8d9b8e44bd8a862f983a363e66a2532ab4e518540245a7d89ca693`
and `1e57ead7b7cf1280de7256b21175598ff9226742356a5ff67b81cee91fcf510d`; removing only the local
terminal seam and reversing qualification reconstructs the spinner exactly at
`0bff86ace66ae27e8ab10e17389e12500769d499c8c21e019dbf0919f8b3ad87`. Current owner hashes are
`7c30c9920a037bb322be4e683c1517b22f91ef0412d550f5b70f02255f1061ea`,
`c609b78d9c66ee5ee38f0a8a89ea8e3fdecf3db0a700271917b3196de70ede95`, and
`af3dd95715c440294307ddbb626ed7f128c40dc3e483a7088ec0b7cd71357573` for result, event, and
spinner, with colocated-test hashes
`6e569569bdf687f3ec6003d463d5d613d2586970b6ca22c5a999c0d6f2662130` and
`ec2273725630c5636305e706265227c09357117377dd52852ee6b6b99edc86eb`. Root facade hashes are
`c9d4faaf79319a24e6ff8a7ed9e24ffafab976e0c47bfeb6040afe6a2338bacd`,
`08a4ef90e960aa43cc3019f73edbce57553264d7e5420f95451567ce60f80fbc`, and
`3ce914ae8ec03d8d23d7107bae61920a0c088fef8ee8c52beb2482d2acd28cec`.

`fetchinstall` now imports the sibling operationrecord owner directly for the unchanged
resolve/download/extract calls. Reversing only that import and qualification reconstructs its
original source exactly at SHA-256
`eed6773b138ae693f2fb8550d1c941c0a8389acc6504b003753fa996f6f61179`; the current source is
`aa8512dd5ad807b07ef8205aa5c40f156d6082ab187f7e101297376b626a56cb`. The install CLI proof now
also asserts the exact nested JSONL unit/status order and retained checksum/result IDs while its
existing stderr assertions retain the resolve/download/extract, status, mode, checksum, result,
and destination bytes; the updated test is
`e6bee745889efd3573f3605553ddfa0555ea4125bfd50a719b0abe3519cfeae2`.

Cached-offline Go 1.25.0 proof passed for all operationrecord tests, all fetchinstall tests including
the end-to-end listener path, the install CLI dry-run and nested operation-result/event test, and
seven representative Docker operation-facade callers. The loopback tests first encountered or
skipped the expected Codex listener restriction, then passed under the narrow localhost-only host
test allowance. Offline list and vet passed for operationrecord, fetchinstall, root Infrastructure,
and Application using the admitted `/tmp` compatibility modfile and a copied `/tmp` module cache.
The first two setup attempts stopped before compilation because the host Go 1.22 launcher was
selected and the checkout-external module cache was read-only; neither was a source failure.

Linux and Windows/amd64 compile-only proof passed for operationrecord, fetchinstall, root
Infrastructure, and Application; Windows binaries were not executed. Binary hashes are
`d1b71646f60e1c1684e75ca23ae00846bf1b7d8c8e44942104c66122aa3bbdf3` /
`1bab4dab56df151dab63cdaf43ae7c8491d836055e0dc24383926cc0b1baa830`,
`96e1546dcfe45b283433f18a55cd2a003b1d7f4696ce92d7614e2569b1e2c0a4` /
`cd4765bfc820c82382bd91e479fbcb4e572244c9d87e4c020ce23ee3546a8ac3`,
`db4a39fe94f24d01438836d0bd7a32d2f933a561b263c888cdd1815cba51c648` /
`f164fa86232158ee0480869cf88beb1cbde04067b05da126f9dc4d0e3e499adc`, and
`b66a64d775ac38ff7e7656d03a71554c077e4acb4cb4b57dc213f8e9c0a3e19f` /
`ccf8e75d9c9dc4349fbf9b3acaa93808b4fee747cb71b69712173e2672fa4611`.
`gofmt`, `git diff --check`, implementation uniqueness, and dependency-direction checks pass. Root
production counts remain 65 Application, 20 Domain, and 57 Infrastructure files. All new proof
artifacts are under `/tmp/dockpipe-task035-package-topology/`; no generated or durable checkout
state changed, and the protected cache and both inherited empty locks remain exact.

### Current topology map and terminal proof boundary

Selected/completed Application children are the admitted compileconfig, imageartifact,
operationids, packagecompile, packagescript, packageversion, pipelangmaterialize, runtimepolicy,
textvalue, treecopy, and wslbridge owners plus this objective's sessioncmd, shellquote, resultcmd,
internalstatecmd, doctorcmd, clonecmd, and runscmd owners. Their root command files are thin
dispatch/facade surfaces. Run/run-steps/strategy/workflow-environment files remain cohesive parent
orchestration state; catalog input projection is blocked on parent catalog record ownership;
Terraform is blocked on `CliOpts` translation ownership; Windows command/bridge is blocked on its
public facades plus the host-bash `windowsGoosFn` seam; package images is blocked on the shared
Docker image-existence seam; package/workflow test and build commands remain coupled to parent
target/config/build adapters; project build/clean/rebuild is one guarded destructive-safety
transaction. SDK, flags, usage, and subcommand files are root composition/public facades.

Domain remains the admitted public facade plus cohesive workflow graph, validation, normalization,
value, import/merge, resolver, and strategy behavior. Model/dependency, model/package,
model/project, and model/runtimeartifact remain the selected wire owners. Workflow graph/inject is
blocked on a Domain-value cycle; filesystem loaders require a separately owned Infrastructure API.

Selected/completed Infrastructure children are packagebuild, envfile, sourcemtime, and
operationrecord. `fetchinstall` is a cohesive downward child and now imports only its sibling
operationrecord owner; no Infrastructure child imports parent Infrastructure or Application.
Durable-state, Git-session, Docker, workflow-store, and path/layout families remain cohesive public
transactions/composition; removal safety is blocked on durable device identity; resolver/strategy
assignment loading stays a small cohesive root surface rather than introducing a generic parser
bucket. No additional Infrastructure owner is selected.

The selected/unselected map is converged and every selected owner has a narrow API, colocated proof,
and acyclic direction. No further implementation seam is selected or preauthorized by this map.

Terminal verification is complete. Cached-offline Go 1.25.0 `go list -mod=readonly` and `go vet`
passed across all 31 `src/lib/...` packages. The same full package set compiled as test packages on
Linux/amd64 and Windows/amd64; the broad runs selected no test bodies and did not execute Windows
binaries, so unrelated stateful tests could not mutate excluded generated/durable checkout state.
Focused behavior proof remains the operationrecord owner suite, the full fetchinstall suite with its
end-to-end localhost path, the install CLI nested stderr/JSONL test, and representative root-facade
Docker callers recorded above. `gofmt -d` is empty for every checkpoint-9 source/test file,
`git diff --check` passes, and terminal dependency-direction searches are empty.

Terminal anchors remain branch `js/dev`, HEAD
`4e71cdeeb2c43aeb43296e23afcb8dc94fe7af6e`, origin `js/dev`
`3fe03013e695b7b13b62221bd7ccbc6ad4334e00`, 0 behind/2 ahead, both inherited stash objects, and
zero staged paths. Before this terminal record, the 37-path default-collapsed status was SHA-256
`b8bcf471bec25939e93dd5a29913aa3572f939091d4761a966cad0dc0255b8a4` and the task record was
`3e7559794e98ed0c7528ab7d14df5a6877d65f16e546b86d6f0095e65553a2b3`. The protected ignored
Python cache remains regular mode-`0664`, 8,408 bytes at SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`; the two inherited empty
mode-`0664` checkout locks remain SHA-256 `e3b0c442...`. Verification artifacts exist only under
the admitted `/tmp/dockpipe-task035-package-topology/` tree. No gate or receiver transport,
worktree, cleanup, generated/durable-state mutation, compatibility change, commit, push,
publication, or external action was used. Terminal disposition: `completed`.

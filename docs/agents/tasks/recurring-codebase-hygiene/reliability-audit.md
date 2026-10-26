# Library audit and repairs — 2026-10-25

Scope: `src/lib` in the saved checkout after package extraction and folder
organization. Source review focused on async execution, archive production and
consumption, SDK discovery/prompts, domain boundaries, and obsolete helpers.
Static analysis and the Go suite cover the full library and CLI. This is not a
claim that every execution, provider or concurrency path has been exhaustively
verified. All implementation remains uncommitted and unstaged.

## Confirmed bugs repaired

| Severity | Trigger and previous failure | Owning repair and regression |
| --- | --- | --- |
| P1 | Existing destination symlink redirected extracted contents outside the destination. | `packagebuild/extract.go` uses `os.OpenRoot` for directory creation and file writes. Escape regression preserves the outside sentinel. |
| P1 | Failed archive rebuild truncated/deleted the previous archive while leaving its checksum. | `packagebuild/tarball.go` stages complete archive/checksum outputs before publication. Build and destination-preflight failures preserve old bytes. |
| P1 | An archive output under its source could read its own growing output. | Producer rejects outputs inside the canonical source before creating files, including source/output-parent aliases. No unbounded reproduction was run. |
| P2 | Unsafe paths and unsupported archive members were silently skipped, producing incomplete success. | Extraction explicitly rejects noncanonical paths, links and special entries. Valid dotted basenames remain accepted. |
| P2 | A requested member returned before gzip trailer validation, accepting corrupt archives. | Selected-member reads drain the bounded decoder before success; corrupt CRC regression fails. |
| P2 | Async workers wrote under their own artifact scopes but the barrier read the parent scope. Raw-name collision checks also confused distinct scopes and missed aliases. | Worker-effective output paths are retained for merge; resolved declared paths are admitted before launch, with Windows case folding. Declaration-order overwrite and consumed-output tests remain. |
| P2 | Async host builtins could report success without running. | Builtins run with worker-local options/environment before commands; errors propagate, sibling/parent environments remain isolated. Race regressions pass. |
| P2 | Custom strategy after-hooks received a pre-step environment and missed merged step outputs. | The after-hook boundary rebuilds its environment from current workflow state. A real-step regression preserves declaration-order merge and verifies the hook receives its result. |
| P2 | File/resource prompts accepted directories as files. | Go and Python SDKs share a regular-file/directory predicate within each implementation; terminal retry tests cover both prompt contracts. |
| P2 | Windows SDK lookup missed a repo-local `dockpipe.exe`. | Go/Python lookup prefers the Windows executable, retains extensionless fallback and preserves explicit override precedence. Portable candidate tests cover the host rule. |

## Duplication and unused code

Removed the unreachable core-source copy/filter APIs, their private copy/cache
helpers and their obsolete test. The live generic `treecopy.Copy` keeps all source
entries, including Python caches; it does not inherit the removed core-specific
exclusion policy. Its direct test now exercises the active contract.

Runtime policy value selection now calls the existing `textvalue.FirstNonEmpty`
owner instead of maintaining another implementation. Public aliases/forwarders
remain compatibility entry points, not independently maintained policy. No other
Domain private declaration was proven dead by the source-reference screen.

## Open correctness and DDD findings

1. **P2 — Aggregate workflow import growth remains unbounded by contract.**
   `domain/workflow/workflow_imports.go` caps depth at 32 but repeatedly reads and
   expands duplicate imports. The previous small DAG amplification finding still
   applies to the current implementation. Define read/byte/expanded-step budgets
   and preserve repeated-import semantics before changing admission; do not infer
   engine capacity from the visual editor's separate 1 MiB contract.
2. **P2 — Legacy Git and host-script cancellation remains incomplete.**
   `infrastructure/git_runtime_session.go` still uses unbounded `exec.Command`
   output helpers without caller contexts. `prescript_wait_unix.go` forwards
   signals only to the immediate child and waits without escalation. The newer
   workspace Git and process-tree owners do not automatically repair these older
   callers. Carry cancellation and command-specific limits through the actual
   lifecycle owners; prove ignored termination and inherited-pipe behavior.
3. **DDD — Domain still owns filesystem loading.**
   `domain/project_config.go` reads files and searches directories;
   `domain/packagecontract/package_manifest.go` reads manifest files. Add pure
   decode/validate entry points and Infrastructure loaders, then migrate internal
   consumers. Existing exported path-based APIs require a compatibility plan;
   introducing a Domain-to-Infrastructure cycle would only hide the problem.
4. **DDD — Provider branch naming remains in the engine Domain.**
   `domain/env.go:BranchPrefixForTemplate` hardcodes provider mappings and has
   live Application callers. Move the policy to resolver-owned metadata while
   retaining explicit branch-prefix overrides and handling historical defaults.
5. **Cross-platform contract — Remote path admission differs by validator OS.**
   `domain/remote/result.go:SafeRelative` uses host `filepath.IsLocal`; Windows
   device names therefore differ from Linux admission. Specify one portable wire
   policy before changing accepted names. No platform claim is inferred from
   Linux-only validation.

## Application cleanup — 2026-10-25

The independent architecture review led to a bounded implementation across its
five priorities. Changes remain unstaged and uncommitted in the saved checkout.
Application continues to own execution orchestration; no Domain process-service
layer, wholesale runner rewrite, external package code or pin change was added.

| Priority | Implemented boundary | Behavior retained |
| --- | --- | --- |
| Workflow admission | `preparedWorkflow` resolves and admits packaged children and resolver delegates, then executes the same loaded configuration. `internal/hostdependencies` accepts a narrow workdir/approval request and captured lookup/installer adapters. | Optional tools, package-tool precedence, installer approval and existing errors. Ordinary profiles created/updated by setup still load after setup. |
| Run/step state | `runSession` owns host environment and success-only completion. Shared `preparedStepState` represents projected host/container maps and effective output paths. Cached step environment slices are removed; snapshots are derived at invocation. | Sequential propagation, isolated async snapshots, declaration-order output merge, post-setup output paths, finally ordering and CLI exit codes. Initial root-command environment overrides remain an explicit overlay rather than leaking into steps or container auth. |
| Typed workflow inputs | `internal/workflowinputs` owns discovery, field/default/documentation metadata and binding. Catalog owns JSON/view projection and inferred controls. | Catalog JSON, default/binding precedence and tolerant discovery. Diagnostics are available separately; execution acceptance was not tightened. The now-unused root type-map file was removed. |
| Artifact layout | Infrastructure owns workflow-state/artifact/CI layout and sanitization; Application and SDK scope projection share those helpers. | Exact paths, environment overrides, errors and scope spelling; no data migration or directory cleanup. |
| CLI and execution composition | `Run` dispatches commands and translates process-exit outcomes. A narrow workflow-selection request owns selection/compile/path resolution; execution captures root/step adapters and nested steps inherit them. | CLI API/dispatch, relative-file and archive selection, editor workdir behavior, errors and compilation order. Existing compatibility test seams remain defaults rather than active worker dependencies. |

### Confirmed admission defect repaired

Previously `runStepPackageWorkflow` and resolver-delegate entry bypassed the
host admission performed by top-level `Run`. A host-only CLI fixture showed that
an absent required executable or `platforms: [windows]` on Linux caused direct
invocation to exit 1 before execution, but a packaged child still ran and exited 0.
The regression also reproduced the bypass in step resolver delegates.

Declared children/delegates are now admitted before call-site setup and before
child environment/secret preparation. The selected configuration is retained
through execution, preventing a second load after setup. A delegate first created
or selected by setup is admitted after that setup and before child execution;
unknown dynamic targets cannot be pre-admitted. The canonical
[host-dependency contract](../../../workflows/workflow-yaml.md#host-dependencies)
describes this ordering. No installers were executed during verification.

Root success completion now derives after-hook environments from current state
for resolver delegation as well as steps and host execution. The same stale-slice
failure repaired earlier for ordinary step workflows is covered by a real child
output regression. After-hooks, workspace finalization and explicit host commit
retain their success ordering; failure stops subsequent effects. Finalization is
not an unconditional cleanup defer because it can checkpoint work.

### Deliberate limits

- Parent/child metadata inheritance is unchanged. A packaged child receives the
  parent's workflow identity and artifact/package scope; no new defect claim or
  implicit identity policy is introduced.
- Host and container environments remain separate. Container auth/policy
  projection is explicit; host-only values are not copied wholesale into it.
- Compatibility globals, process workflow-directory overrides, event-log process
  environment and legacy process cancellation remain. Captured adapters do not
  establish concurrent public `Run` support or cancellation of older adapters.
- The runner still coordinates profiles, workspace, images and execution. Its
  remaining `CliOpts` carrier and host/container/delegate policy can be narrowed
  further when a concrete change requires it. Further folder moves or adapter
  fanout are not part of this bounded cleanup.
- The earlier import-capacity, Domain filesystem/provider-policy and portable
  remote-path findings remain open. Promotion remains on hold.

Focused checks cover required/optional dependency admission, platform rejection,
setup-before-execution ordering, selected-child single loading, profiles created
or rewritten by setup, service capture, sourced output paths, separate container
environments, completion error ordering, typed input/catalog parity and exact
artifact paths. All 59 Go packages pass (56 ordinary suites and three existing
localhost-listener suites); focused runner/service race checks and staticcheck
pass. CLI builds pass for Linux/amd64, Windows/amd64 and Darwin/arm64. The final
Linux CLI fixture confirms both direct and nested rejection for missing required
tools and unsupported platforms, while successful metadata inheritance remains
unchanged. Cross-builds do not establish native Windows/macOS runtime behavior.
Terminal logs and owned path/preimage inventories are retained in the task's
temporary proof directory. Two test-created image receipts were retained there
and removed from the checkout; pre-existing generated state was preserved.

### Follow-up correctness and readability review

The follow-up found one admission ordering defect: a host step inheriting a
workflow-delegating resolver could invoke an approved dependency installer before
rejecting the incompatible step kind. Step/profile compatibility now precedes
child loading and dependency admission, including delegates introduced by setup.
Regression fixtures pass structural workflow validation and reproduce both old
paths with a stub installer; no real installer runs. Valid delegates and ordinary
profiles retain their setup ordering.

Two new host-dependency tests now assert Flatpak's missing-dependency failure and
zero installer calls, preserving native installer assertions without skips.
Discovery and resolver comments were corrected, and completion inputs were
expanded for readability. Full Application and extracted-owner suites,
focused admission race checks, staticcheck and simulated Flatpak checks pass.
This pass does not repeat cross-platform builds or establish native Flatpak
runtime qualification. All follow-up changes remain unstaged and uncommitted.

## Dockpipe GUI shell cleanup — 2026-10-25

The GUI shell now lives in `src/dockpipe-app`. Its 228 existing flat source files
were inventoried and moved into feature contexts with domain, application,
infrastructure, presentation and integration responsibilities. Source provenance,
platform conditionals and third-party assets are retained. The canonical
[app architecture](../../../../src/dockpipe-app/ARCHITECTURE.md) documents actual
ownership and remaining mixed responsibilities; the move is not a claim that
every controller is now a pure DDD service.

Catalog metadata and display decisions now have a domain owner separate from CLI
discovery. DTO-only views consume that model directly. Context saves use checked,
atomic replacement instead of truncating the previous file and ignoring write
failure. Failed process starts report once after attempt state is cleared, so
failure handlers can retry safely. Both regressions fail against preserved
original implementations and pass with the fixes.

Visible branding is Dockpipe. Build, packaging, SDK producer and current source
references follow the new path. Installed executable/desktop/IPC identities,
settings schemas/paths, native plugin IIDs and published SDK names remain stable.
The installed flat SDK header closure remains complete; external browser
implementation stays in its pinned SDK.

At the cleanup checkpoint, focused evidence included full native CMake
configuration, 73 workflow contract tests, source-layout checks and three native
CTests. Selected catalog/model and MainWindow translation units compiled. That
checkpoint did not establish full GUI link/startup or cross-platform
qualification. The user deferred the cross-platform local build
and test matrix until after this cleanup. No installed state, real contexts,
credentials, package pins, Git history or staging state were changed. Proof and
preimages are retained under `/tmp/dockpipe-app-cleanup`; work remains unstaged
and uncommitted.

## Local cross-platform verification — 2026-10-25

The subsequent local matrix found and repaired a missing direct GUI test include,
the VS Code import's `darwin` versus authored `macos` platform mismatch, and a
repository layout guard that still scanned removed Go source roots. Regression
tests preserve Linux/Windows imports and verify layout findings are not hidden by
missing directories. Source review also confirms scan errors remain failures.

Test portability corrections canonicalize temporary paths, retain the resource
fixture's 128 levels within native path-length limits, explicitly set permission
fixture modes under restrictive umasks, and select a temporary download directory
before opening the browser cancellation dialog. Runtime path/ownership guards,
download assertions and the browser's 60-second deadline remain unchanged.

Linux passes all 55 tested Go packages, 24 Application race packages, staticcheck,
the full GUI build and 71 CTests with one optional skip. ARM macOS passes the
native CLI and GUI builds and all 55 tested Go packages. Its 68 registered CTests
are covered by 65 passes and three intentional skips, including a focused Git
test rerun after restoring its explicit CLI input. The Mac browser and dashboard
performance checks pass without relaxing thresholds. Native Mach-O symbol and
case-alias archive fixtures pass without skipping. Five CLI targets build;
Windows and both Mac architectures also compile all 55 tested Go packages.
Compilation alone does not establish native execution.

Windows additionally passes the native CLI build, all 55 tested Go packages and
three native archive handle/sharing/junction fixtures. Missing task-local Bash
PATH entries caused four initial package failures; those packages pass after the
existing Git Bash location is selected. The VM adapter dependency was built and
tested in its owning package repository against released producer CLI/SDK inputs,
then consumed through explicit local development archives. Four stale package
test version assertions were corrected; published catalog pins are unchanged.
The guest runs on a task-owned overlay with copied firmware/TPM state and original
backing images opened read-only.

The full Windows GUI build passes. Its 60 registered CTests are covered by 56
passes and four explicit skips, with no remaining failure. The initial full run
passed 55 and failed embedding on the SSH service desktop: both native enumeration
APIs found a disposable window, but `IsWindowVisible` reported false despite its
visible style bit. After direct user login to the disposable guest console, the
unchanged embedding test passes in the same user's interactive session, and the
required non-skipped result verifier passes. The standard-user launcher self-test,
native terminal transport, browser and dashboard performance checks also pass.
No product visibility guard, assertion or timeout was relaxed.

Windows retains four explicit skips: dashboard-store and browser-link-router
configuration-isolation guards, the POSIX-only workspace-package fixture, and the
opt-in live extension test. Internal QtTest skips are listed separately in the
native receipt; passing the embedding gate does not qualify those other opt-ins.

Mac dashboard-store and browser-link-router retain their configuration-isolation
skips; the live extension test remains opt-in. Other installed-plugin, Flatpak,
foreign-window and native print-panel skips are listed in the platform receipts.
The Mac run uses an ARM host and offscreen rendering, not Intel or interactive
Cocoa proof. Its SDK link warning reports a macOS 15 minimum, so this newer-host
run does not qualify macOS 13 support.

Earlier Mac runs relied on XDG variables that do not redirect Darwin's durable
state and Qt preference paths. Metadata records time-correlated runtime/test
state under the worker's user Library; without a pre-run baseline, ownership of
every entry is unproven. Shared state was not deleted. Final GUI verification uses
a fresh task home with a Qt path probe proving configuration isolation. Temporary
logs, hashes and exact skip accounting are retained under
`/tmp/dockpipe-cross-platform-local`.

## Verification and practical limits

Focused archive regressions reproduce the old unsafe extraction, replacement and
CRC failures before the repairs. The complete packagebuild suite, full Application
suite, SDK suites and staticcheck pass. Async regressions also pass with the race
detector. The full Go package set is validated in the sandbox plus a narrow host
run for the three existing localhost-listener suites; no tests were skipped to
avoid sandbox failures. Build/platform results are recorded in the final receipt.

Archive and checksum publication is still two renames: a later checksum rename
failure is explicitly reported as partial publication. Extraction callers own
cleanup after later member failure; the cache wrapper removes failed extraction.
Member lookup now validates the full decoder stream and can do more I/O for
large archives; no performance benchmark was claimed. A conditional case-alias
regression is not applicable on the current case-sensitive Linux filesystem;
the subsequent native macOS run above executes it successfully without skipping.
Content writes are contained, but the existing named-path executable-mode bridge
and concurrent filesystem mutation are not a complete adversarial filesystem
sandbox. Async output collision admission covers declared paths before scripts;
dynamic script reassignment remains outside that guarantee.

Backups, preimages, regression logs, candidate binaries and the final receipt stay
under the task's temporary proof directory. No installed package/runtime data,
external package implementation, credentials, Git history or release state was
changed by the earlier cleanup phases. The subsequent native validation's
runtime/test state footprint is documented above. Promotion remains on hold.

The [superseded October 13 audit](../closed/recurring-codebase-hygiene/reliability-audit-2026-10-13.md)
is preserved separately. Its old repair order and retired-core paths are historical.

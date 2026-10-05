# 0.6 hosted release qualification — 2026-10-04

## Objective and authority

- Objective: `dockpipe-0.6-release-qualification`.
- State: staging CI repair is local and validated at focused scope; broader scan
  findings remain unresolved. Hosted dry-run qualification passed at pushed checkpoint `d6207ae`.
  All five native jobs, MSI lifecycle, and unprotected combined-artifact assembly
  succeeded in ninth run `37251332865`. Production publication and dev.to were
  skipped with no pending deployment approvals. Downloaded package checksums and
  actual APT consumption passed; the redundant Release self-entry is noted below.
- Execution skill: `dorkpipe-objective-execution`.
- Authority: on 2026-10-04 the user requested fixing the failing tests because
  "we need everything green", and explicitly invoked `dorkpipe-task-handoff`.
  This authorizes implementation and one fresh continuation chat, preserving the
  separate commit/push and publication boundaries below.
- Done when: the failing tests are repaired with their safety contracts intact,
  affected checks pass, and the intended SHA and dry-run inputs have verified
  hosted success across all five stores, native smoke, MSI checks, signed APT
  tests, catalog completeness, and checksums. Recorded failures are evidence,
  not completion of this renewed objective.
- Scope: dry-run dispatch, retrieval, diagnosis, and directly related reversible
  readiness/implementation repairs. New commits/pushes require user authorization.
- Required promotion path, confirmed by the user: `js/pipelang → js/dev → dev →
  staging → master`. Production release publication must originate from `master`.
  The `js/pipelang` run is qualification only; `assemble` prepares and verifies
  dry-run artifacts while `publish` is skipped. This clarification does
  not authorize branch merges or a production dispatch. Automatic release runs
  already trigger on pushes to `master`; the pushed repair also rejects
  manual non-dry-run dispatch on every other ref.
- Exclusions: production release/tag, R2/APT publication, dev.to, merge, billing,
  protection rules, credential changes, infrastructure mutation, or automatic handoff.

## Checkpoint and run

The saved `js/pipelang` checkout and the live GitHub branch both matched
`afc0a2cf22330134940fd3857bc33e9d864c9c44` immediately before dispatch. The tree was
clean, `VERSION` was `0.6.0`, and both existing stashes were preserved. The
[audit](release-0.6-audit.md) and [cleanup](release-0.6-cleanup.md) local proof is
admitted; this hosted run tests the later committed and pushed checkpoint.

[Release run 37220714083](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37220714083)
was created at `2026-10-04T17:29:39Z` using `workflow_dispatch` on `js/pipelang`.
The run API confirms the exact head SHA above. Metadata job `111490315248`
succeeded and its log confirms:

```text
INPUT_VERSION: 0.6.0
INPUT_DRY_RUN: true
INPUT_BUILD_MSI: true
```

No previous Release run existed on this branch before dispatch. The workflow's
production configuration, tag check, GitHub release, R2 upload, and dev.to paths
are guarded against dry runs. Signed APT generation uses a temporary key.

## Hosted evidence

The run concluded **failure** at `2026-10-04T17:36:39Z`. Metadata passed; publication and dev.to jobs were
skipped. No production publishing step ran.

| Target / gate | Actual evidence |
| --- | --- |
| Linux arm64 | Job `111490340606` passed. Native CLI/workflow and secrets environment smoke passed. Downloaded artifact `11310357310` contains 58 packages: one core, 42 workflows, 15 resolvers. Independent `verify-store` rehashed all 58 successfully. CLI/store tarballs, DEB, RPM, APK, and Arch package are present. |
| Linux amd64 | Job `111490340609` built and verified 58 packages; native CLI/workflow and secrets smoke passed, as did root runtime Go tests. The shell/package suite failed in `test_backlog_remote_workflow.sh` at its live-index ambiguity assertion. Artifact upload and release tooling/signed-APT tests were skipped. The layout guard also falsely passed when `rg` was missing. |
| macOS Intel | Job `111490340578` failed in the remote source hook: SDK `declare -gA` is unsupported by system Bash. Store verification, native smoke, and upload were not reached. |
| macOS Apple Silicon | Job `111490340611` failed at the same remote source-hook SDK declaration. Store verification, native smoke, and upload were not reached. |
| Windows amd64 / MSI | Job `111490340594` compiled the CLI, then its startup panicked on the pinned Unicode data checksum. Native smoke, runtime tests, installer recovery, MSI build/install/uninstall, and upload were not reached. |
| Combined catalog / APT / checksums | Publish job skipped due to failed dependencies. No combined manifest, signed APT repository, or top-level `SHA256SUMS.txt` was produced. Only the Linux arm64 uploaded store was independently rehashed. |

The Windows panic digest `34f97be27ca68fdd2d3fdbb4a648c545f473c8089eafc50df792afbd8ea5650f`
exactly matches CRLF conversion of `CaseFolding-17.0.0.txt`. Its expected LF digest
is `ff8d8fefbf123574205085d6714c36149eb946d717a0c585c27f0f4ef58c4183`.

## Local repairs and validation

- `.gitattributes` pins that exact Unicode file to LF. `git -c core.autocrlf=true
  cat-file --filters HEAD:<path>` now yields the expected checksum and no CRLF.
  The runtime checksum guard remains unchanged.
- Remote and secrets source hooks call the existing `__state package-runtime`
  CLI helper directly instead of loading the associative-array shell SDK. An
  isolated Linux fixture with stub tool compilation and the real CLI path helper
  and resolver compiler produced all five resolver archives. This is not native
  Mac acceptance or a proof of broader SDK compatibility with Bash 3.2.
- The backlog shell regression now uses a controlled two-task ambiguity fixture.
  Reproduction against the real index showed `malformed_index` from strict schema
  rejection of newer task metadata; the former expected ambiguity was stale.
  Canonical index compatibility is a separate finding; parser strictness and task
  metadata were not changed. The fixture's explicit input list includes 13 newly
  required cleanup/secret-environment files, plus a minimal fixture embed declaration generated with the owning renderer,
  increasing its bound from 215 to 229. Temporary fixture copies bind fingerprints
  to the current synthetic chain before rejection/tamper cases run. Recorded
  provider observations and production validation rules remain unchanged.
- Linux release regression setup explicitly installs ripgrep. The layout guard
  now fails if `rg` is unavailable instead of silently skipping its reference scan.
- Shell syntax, layout guard, workflow YAML parsing, preserved production guards,
  authored embed-manifest check, root embed Go tests, and whitespace checks passed.
  ShellCheck reported existing dynamic-source/trap/
  export warnings; it is not claimed clean.

The full `test_backlog_remote_workflow.sh` regression passed with its synthetic
validation execution, rejection, tamper, replay, and application checks. Local
tests used Go 1.26.7 and isolated state/cache paths; the hosted run used Go 1.25.11.
An initial local retry selected host Go 1.22.12 and stopped before compilation;
later sandbox attempts needed isolated state and local loopback listeners. The
same focused test then passed under reviewed host execution with `GOPROXY=off`.
The missing-ripgrep negative check also passed: the guard fails explicitly when
`rg` is absent. Native Mac/Windows repair acceptance remains pending a new run.

## Approved repair checkpoint and second run

The user approved committing/pushing the 13 reviewed files and rerunning hosted
qualification. Runtime checkpoint `cp-20261004-175629` created
`5491c3f3bb9c411160149b8a9ea04c7d6eb5dd25`; the exact 13 paths, parent, and live
`origin/js/pipelang` tip were verified. The new untracked `.vscode/settings.json`
was excluded and preserved byte-for-byte, as were both protected stashes.

[Release run 37222448250](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37222448250)
was dispatched at `2026-10-04T17:56:49Z`. The API confirms the repaired SHA, and
metadata job `111495368549` confirms `version=0.6.0`, `dry_run=true`, and
`build_msi=true`. The run concluded **failure** at `2026-10-04T18:01:33Z`.

| Target | Second-run result |
| --- | --- |
| Linux arm64 | Job `111495394952` passed, including native CLI/workflow and secrets smoke. Artifact `11311325087`. |
| macOS Intel | Job `111495394922` passed, including native CLI/workflow and secrets smoke. Artifact `11310891176`. The original Bash source-hook failure is resolved on this runner. |
| macOS Apple Silicon | Job `111495394999` passed, including native CLI/workflow and secrets smoke. Artifact `11311076269`. The original Bash source-hook failure is resolved on this runner. |
| Linux amd64 | Job `111495394973` built its store and passed native smoke, then `go list ./...` discovered the new fixture's `embed_assets.go` as a package and failed because its relative `VERSION` input was absent. This was introduced by the first repair. |
| Windows amd64 | Job `111495395032` passed the previous Unicode startup failure, then failed while preparing `dorkpipe.mcp` state: `durable package-state inventory does not match its legacy source`. No native smoke, runtime suite, installer recovery, or MSI steps ran. |

Publication and dev.to were skipped again. Signed APT tests, combined manifest,
top-level checksums, and MSI acceptance remain pending.

All three uploaded stores were downloaded and independently rehashed: 58 packages
per platform (one core, 42 workflows, 15 resolvers), 174 total. Their bundled store
tarballs also contain identical manifests and matching package payload checksums.
The downloaded evidence is under `/tmp/dockpipe-0.6-run-37222448250-artifacts`;
run metadata and job logs use the `/tmp/dockpipe-0.6-run-37222448250` prefix.

### Second local repair

- Rename the fixture embed declaration to `embed_assets.go.txt`, copy it to the
  expected Go filename only inside the temporary consumer, and regenerate the
  authored embed manifest. Root `go list ./...` and the full focused backlog
  shell regression now pass, including rejection and tamper cases.
- Preserve the underlying error when migrated-state inventory inspection fails;
  previously every inspection error was reported as an inventory mismatch.
- Windows DACL validation now accepts generic full control and its equivalent
  file-specific mask; new grants use the file-specific mask. Current-user/System
  trustees, protected DACL, exactly two allow entries, and full-control requirements
  remain unchanged. Existing native creation tests now validate the resulting
  ACL, and a new native test rejects a partial-control grant.

The Windows change is a source-supported repair hypothesis, **not a confirmed
native fix**: the existing validator compared only the literal `GENERIC_ALL` mask,
whereas [Windows maps generic file rights](https://learn.microsoft.com/en-us/windows/win32/fileio/file-security-and-access-rights)
to file-specific rights. The original log suppressed the underlying inspection
error. Native validation or the improved diagnostic is still required.

Focused Linux durable/package-state tests passed. The Windows infrastructure test
binary cross-compiled with the added ACL regressions; it has not run on Windows.
The first cross-compile caught an unavailable x/sys constant; the final build uses
the Windows SDK's documented mask composition. No permissions on user state were
changed by these tests. The user has approved committing and pushing this second repair.

## Approved second repair and third run

The user approved the eight-path follow-up repair (counting the fixture rename's
old and new paths). Runtime checkpoint `cp-20261004-181659` created
`a717168f657f299895f941bd9d7ebaa414843151`; the exact paths, parent, and live
`origin/js/pipelang` tip were verified. Editor settings and both stashes remain
preserved.

[Release run 37223851524](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37223851524)
was dispatched at `2026-10-04T18:17:20Z`. The API confirms the new SHA and metadata
job `111499369620` confirms `version=0.6.0`, `dry_run=true`, and `build_msi=true`.
The run concluded **failure** at `2026-10-04T18:24:39Z`.

| Target | Third-run result |
| --- | --- |
| Linux arm64 | Job `111499400346` passed, including native CLI/workflow and secrets smoke. |
| macOS Intel | Job `111499400333` passed, including native CLI/workflow and secrets smoke. |
| macOS Apple Silicon | Job `111499400320` passed, including native CLI/workflow and secrets smoke. |
| Linux amd64 | Job `111499400306` built the store and passed native smoke and the root runtime Go suite. The package regression failed in the backlog test's nested orchestration-helper suite and subsequent validation-record comparison. Docker smoke and DEB install smoke passed. Release tooling tests and artifact upload were skipped. |
| Windows amd64 | Job `111499400322` completed package/store building and native CLI/workflow smoke. This clears the prior migrated-state build failure for this runner. Runtime tests then failed across Windows path/permission assumptions and fixtures; installer recovery, MSI build/install/uninstall, and upload were not reached. |

All three uploaded stores were downloaded and independently rehashed: 58 packages
per platform (one core, 42 workflows, 15 resolvers), 174 total. Each bundled store
tarball has the same manifest as its loose store and all 58 package payload hashes
match. Evidence is under `/tmp/dockpipe-0.6-run-37223851524-artifacts`; terminal run
JSON, metadata log, and failed-job logs use the `/tmp/dockpipe-0.6-run-37223851524`
prefix. Linux amd64 and Windows artifacts were not uploaded, so their store proof
is hosted build evidence only. The Mac and Linux arm64 jobs do not run the full
runtime regression suite.

### Exact remaining failures

- Linux: `TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExactRoutes/failed_terminal_result`
  fails at `nodeconnectorplacementexecutiongraphnexttaskresultcontinuationoutputdeliverypolicy_test.go:40`
  with `post-transition graph output executor requires the complete immutable predecessor chain`.
  The enclosing `test_backlog_remote_workflow.sh` then fails at line 874 because
  the two `validation-execution.json` records differ at byte 544, line 19. Earlier
  local passes do not override this hosted failure; nondeterminism is a hypothesis
  requiring reproduction, not a confirmed diagnosis.
- Windows policy/removal/remote tests reject runner temporary paths containing
  `RUNNER~1` as linked or reparsed. Source inspection found lexical comparison
  after `EvalSymlinks`; short-name expansion is a plausible shared cause requiring
  native confirmation. Actual reparse-point rejection must remain intact.
- Windows fixture/platform failures also include POSIX-only executable/home paths
  in Cloudflare setup tests; private-directory fixtures without protected DACLs;
  a `0700` mode assertion; home/durable-root protection fixtures; canonical rename
  comparison; and the accepted `a\\b` suffix in `TestJoinStatePathRejectsTraversalAbsoluteAndReservedNames`.
  Remote executor cancellation tests fail before process execution, and remote
  persistence/recovery tests fail before their intended assertions. These are not
  evidence that the intended cancellation or recovery contract passed.
- Windows compilation fails in `tests/containedexec/transcript/extract.go:33,40`
  because `span` is declared in `main_linux.go`. The runtime qualification selector
  includes this Linux-only compiler campaign despite its stated separate scope.

Publication and dev.to were skipped. MSI acceptance, signed APT tests, combined
manifest completeness, and top-level release checksums therefore remain unproven.
No fourth hosted run has been dispatched.

### Third repair: Linux fixture isolation

The exact hosted Linux failure reproduced in the full backlog shell regression.
A broader graph-suite race check identified JSON decoding in the launch-executor
negative tests writing through a shallow fixture copy into shared predecessor
outcome slices while parallel continuation tests read them. The later restore
also wrote shared storage. Decode each case into a zero-value expected structure
instead, and assert that the original serialized expectation remains unchanged.
The scheduling-executor test had the same shallow-copy pattern and now uses fresh
decoded storage too. Production chain validation and rejection cases are unchanged.

The affected launch/continuation tests passed three race-enabled repetitions with
`-parallel=2` on Go 1.26.7. The full backlog regression passed after this repair
and again after the Windows shared-state changes below, using isolated state and
reviewed loopback access. Temporary chain diagnostics were removed. Evidence uses
`/tmp/dockpipe-0.6-repair3-*`; no commit or push is authorized for these repairs yet.

### Third repair: Windows portability and safety

- Replace lexical `EvalSymlinks` equality in policy manifests, removal boundaries,
  overrides, remote profiles, and artifacts with component-by-component link/reparse
  checks. Windows short names may expand without representing a link. Canonical
  checkout exclusion still rejects override aliases inside the checkout. Correct
  the Windows file-info attribute assertion to use `syscall.Win32FileAttributeData`,
  the type returned by `os.Lstat`. Native regressions cover short names, junctions,
  checkout aliases, and target preservation; they are cross-compiled, not yet run.
- Reuse existing private-state ACL and atomic-publication helpers for remote state.
  New files receive owner-only protection before their first content write. Existing
  directories are validated without silently repairing permissions. Protected DACL,
  exact current-user/System trustees, and full-control checks remain intact. Unix
  shared-directory and Windows partial-DACL rejection regressions preserve existing
  permissions. Credential reads use the host permission validator; live Cloudflare
  browser/provider operations were not invoked or qualified.
- Replace shell-only worker fixtures with the native test executable, retaining
  argv, cwd, artifact, deadline, heartbeat, and descendant-process coverage. Remote
  and Cloudflare commands now use the existing process-tree runner, including its
  Windows suspended-start/job-object containment.
- Use native home/state environment variables and actual durable roots in protection
  fixtures, protected private directories in ACL fixtures, canonical paths in rename
  assertions, and ACL validation instead of Unix mode assertions on Windows. Preserve
  the existing Windows suffix contract: backslashes normalize to separators there;
  traversal, absolute paths, device names, and foreign Unix separators stay rejected.
- Exclude `tests/containedexec` and its subpackages from the 0.6 runtime selector,
  matching its already-declared separate PipeLang compiler qualification scope.
  No supported 0.6 runtime test was skipped to obtain a pass.

Local proof: the full 0.6 runtime script passed, as did remote/Cloudflare suites,
focused state and removal checks, targeted race checks, final artifact/cancellation
checks, and the Linux CLI build. All six affected Windows test packages cross-compiled;
the final infrastructure/remote/Cloudflare builds also passed after review fixes.
Shell syntax, authored embed-manifest verification, and whitespace checks passed.
Local Go is 1.26.7; the hosted workflow pins 1.25.11. Native Windows ACL/junction/MSI
execution, signed APT, and combined catalog/checksum success require the next run.

This repair owns 24 tracked paths, including these two evidence documents. Existing
editor settings and both protected stashes are preserved; no staged changes exist.
Only temporary test state, logs, and build outputs under `/tmp` were intentionally
created. No authored embed manifest needed regeneration. Shared engine changes are
generic filesystem/process boundaries; Cloudflare behavior stays package-owned.

## Approved third checkpoint and fourth run

The user approved the 24 reviewed repair/evidence paths, push, and dry-run dispatch.
Runtime checkpoint `cp-20261004-193629` created
`b6da48c8dd27e565a45d85a339ed325d206fab50`, with parent `a717168f` and all 24 exact
postimages verified. The runtime pushed `origin/js/pipelang`; remote read-back
matched the new SHA. The index is empty, editor settings retain their original
digest, and both protected stashes remain unchanged.

[Release run 37228956974](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37228956974)
was dispatched at `2026-10-04T19:37:01Z`. The run API confirms that exact SHA.
Metadata job `111514370724` succeeded and its log confirms `version=0.6.0`,
`dry_run=true`, and `build_msi=true`. The run concluded **failure** at
`2026-10-04T19:43:14Z`. Run metadata and logs use
`/tmp/dockpipe-0.6-run-37228956974`.

| Target | Fourth-run result |
| --- | --- |
| Linux arm64 | Job `111514398814` passed store build, native CLI/workflow and secrets smoke. Artifact `11312503025`. |
| macOS Intel | Job `111514398820` passed store build and native smoke. Artifact `11313232837`. |
| macOS Apple Silicon | Job `111514398809` passed store build and native smoke. Artifact `11312757523`. |
| Linux amd64 | Job `111514398848` passed store/native smoke, the full runtime Go suite, and DorkPipe's backlog regression. The package suite advanced to Pipeon, where its stale durable-import fixture failed. Docker and DEB smoke passed. Release-tooling/signed-APT steps and upload were not reached. |
| Windows amd64 / MSI | Job `111514398757` passed store build and native smoke. Earlier filesystem, ACL, path, and process failures cleared. Remaining failures are six Linux/macOS remote setup/initialization/worker tests incorrectly expecting success on Windows, where the existing lock rejects remote-node operation. Installer recovery/MSI steps and upload were not reached. |

All three uploaded stores were independently rehashed: 58 packages each (one core,
42 workflows, 15 resolvers), 174 total. Their bundled store archives also match the
manifests and payload hashes. Downloaded evidence is under
`/tmp/dockpipe-0.6-run-37228956974-artifacts`. Publication and dev.to were skipped;
no combined catalog, top-level checksums, or signed APT output was produced by this run.

### Fourth local repair

- Pipeon's migration regression now seeds the actual pre-durable layout and binds
  Pipeon's package-owned migration manifest, as the existing IDE regression does.
  The old fixture seeded a new durable public scope, which the importer correctly
  did not treat as the legacy source. A control run confirmed the existing workdir
  environment is honored; the initially suspected production lookup change was
  unnecessary and removed. All original import, cache-exclusion, source-integrity,
  private-key, and linked-key rejection assertions remain.
- TASK-036 explicitly leaves Windows workers for follow-up. Its existing remote
  lock permits Linux/macOS only. Six success-path tests now have those same build
  constraints: three worker tests, broker initialization, and two provider-setup
  tests. Their bodies are unchanged. Windows instead compiles three negative
  regressions proving platform rejection before polling/execution, credential
  publication, or provider commands/recovery publication. Portable protocol,
  persistence, ACL, artifact, cancellation, and process tests remain enabled.
  No runtime OS guard was removed and no Windows worker implementation was added.
- Canonical remote documentation now states the existing platform boundary.
  The owning generator refreshed `embed_assets.go` for the added package test.

Local proof: full Pipeon, secrets, and VM package suites passed; these cover the
package hooks blocked by Pipeon in the hosted run (remote provider tests also
passed). The affected remote/application/provider Go suites passed with `-race`.
All three affected Windows test packages cross-compiled, including the negative
platform tests; native execution remains pending. All nine release-tooling tests
passed, including real APT signature/index validation with a temporary test-only
key. Root embed tests, generator check, shell syntax, preserved test-body comparison,
and whitespace checks passed. Initial sandbox attempts stopped on loopback/GPG
agent restrictions; narrow reviewed host runs passed. An initial package invocation
also hit read-only user state; rerunning with isolated state passed. Local Go is
1.26.7; hosted Go remains 1.25.11.

This follow-up owns 13 paths including the two evidence documents, with no runtime
production-code changes. Package/engine boundaries and public schemas remain intact.
The generated embed manifest is the only generated tracked change. New validation
logs, isolated state, downloaded evidence, and cross-builds are under `/tmp`.

## Approved fourth checkpoint and fifth run

The user approved the reviewed 13-path follow-up, push, and fifth dry run.
Runtime checkpoint `cp-20261004-205622` created
`27224d00bc4d8f5c85ef384aaf46aec2d5c8c6ba`, with parent `b6da48c8` and all exact
approved postimages verified. The runtime pushed `origin/js/pipelang`; remote
read-back matched. The index is empty. Unrelated editor settings retain their
original digest and both protected stashes are preserved.

[Release run 37234061091](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37234061091)
was dispatched at `2026-10-04T20:56:51Z` with `version=0.6.0`, `dry_run=true`, and
`build_msi=true`. The API confirms that exact SHA; metadata job `111529567982`
passed and its log confirms all three inputs. The run concluded **failure** at
`2026-10-04T21:04:28Z`. Run metadata and logs use `/tmp/dockpipe-0.6-run-37234061091`.

| Target | Fifth-run result |
| --- | --- |
| Linux arm64 | Job `111529600486` passed store build and native smoke. Artifact `11314812650`. |
| macOS Intel | Job `111529600460` passed store build and native smoke. Artifact `11315216700`. |
| macOS Apple Silicon | Job `111529600468` passed store build and native smoke. Artifact `11314418987`. |
| Linux amd64 | Job `111529600437` passed store/native smoke, runtime Go tests, DorkPipe/backlog, Pipeon, remote, and secrets package tests. The VM Go tests passed; its shell fixture failed because it supplied a checkout-local `DOCKPIPE_PACKAGE_STATE_DIR`, correctly rejected by the injected current CLI. Docker and DEB smoke passed. Release tooling and upload were not reached. |
| Windows amd64 / MSI | Job `111529600399` passed store/native smoke, the entire runtime test selector including the new platform-rejection tests, and Windows installer failure recovery. It then failed binding the MSI build script's PowerShell parameters. MSI build/install/uninstall and upload remain unqualified. |

Publication and dev.to were skipped. No combined catalog/checksum or signed APT
artifact was produced. The three successful platform artifacts were downloaded to
`/tmp/dockpipe-0.6-run-37234061091-artifacts`. All 174 packages were independently
rehashed (58 per platform: one core, 42 workflows, 15 resolvers); bundled store
archive manifests and package hashes also match.

### Fifth local repair

- MSI build and smoke calls now use named hashtable splatting. The former string
  arrays supplied parameter names as positional values. A native PowerShell
  regression executes the actual workflow call bodies against the real scripts'
  parameter declarations in isolated temporary checkouts. It checks exact values,
  paths containing spaces, CLI-only builds, and launcher-enabled builds. Separate
  negative controls reproduce both original binding failures. The Windows job runs
  this check before attempting MSI packaging. No WiX download or installer is
  executed by this regression.
- The VM shell fixture no longer sends its synthetic legacy checkout path through
  the public package-state override. It controls legacy discovery directly alongside
  its existing fake state helper, uses isolated user state, and selects the injected
  current CLI. SDK/public path validation stays active and unchanged. Identity,
  credential, TPM ambiguity, helper-output, restart, collision, and permission
  assertions are unchanged. This remains a unit fixture, not live VM startup or
  real VM migration acceptance.

Local verification: the original VM failure reproduces through `dockpipe package
test --only vm`, and the repaired package command passes. MSI binding passes with
PowerShell 7.6.6 on Linux; actual Windows MSI execution still requires hosting.
All eight package hooks passed with isolated state and the offline Go cache. The
full sequence passed agent, DorkPipe/backlog, MCP, and IDE, then hit a local Pipeon
PATH mismatch: the temporary executable was not named `dockpipe`, while Pipeon
invokes that basename. The same binary copied under its canonical name passed
Pipeon, remote, secrets, and VM through their package commands; the earlier prefix
was not rerun. The initial local failure is retained in the full-sequence log.
YAML parsing, unchanged MSI/publication conditions,
CLI build, shell syntax, authored embed-manifest check, and whitespace checks pass.
No generated tracked change is needed; validation produces `/tmp` evidence and
ordinary ignored package-test scratch under `bin/.dockpipe/tmp/package-tests`.

## Approved fifth checkpoint and sixth run

The user approved the five-file follow-up, push, and sixth dry run. Runtime checkpoint
`cp-20261004-211507` created `16b10cf5574d77951909b24b06103c0a424e36f1`, with parent
`27224d00` and all five approved postimages verified. Runtime publication and remote
read-back confirm `origin/js/pipelang` at that SHA. Editor settings and both protected
stashes are unchanged; the index is empty. Engine/package boundaries and public state
safety contracts remain unchanged.

[Release run 37235364232](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37235364232)
was dispatched at `2026-10-04T21:15:34Z` with `version=0.6.0`, `dry_run=true`, and
`build_msi=true`. The API confirms the intended SHA and metadata job `111533269221`
confirms all inputs. The run concluded **failure** at `2026-10-04T21:23:28Z`.
Evidence uses the `/tmp/dockpipe-0.6-run-37235364232` prefix.

| Target | Sixth-run result |
| --- | --- |
| Linux amd64 | Job `111533409838` passed store/native smoke, the complete runtime and eight-package sequence, Docker/DEB smoke, and all nine release-tooling tests including signed APT. Artifact `11315327216`. |
| Linux arm64 | Job `111533409897` passed store build and native smoke. Artifact `11314758983`. |
| macOS Intel | Job `111533409829` passed store build and native smoke. Artifact `11314859674`. |
| macOS Apple Silicon | Job `111533409864` passed store build and native smoke. Artifact `11315283235`. |
| Windows amd64 / MSI | Job `111533409854` passed store/native smoke, runtime tests, installer failure recovery, MSI argument binding, and actual WiX MSI build. The smoke installed the MSI, ran the installed CLI, and uninstalled it; its snapshot comparison then falsely rejected the baseline. MSI smoke completion and upload remain pending. |

The Windows failure does not demonstrate leftover files: the same unparenthesized
PowerShell expression rejects two empty snapshots and identical nonempty snapshots.
Checks after that assertion, including the core directory/PATH checks, still require
native execution. Publication and dev.to were skipped, so no combined catalog or
signed-APT release artifact was produced. All four uploaded stores and their bundled
archives were independently verified: 232 packages total, 58 per platform (one core,
42 workflows, 15 resolvers). Downloads are under
`/tmp/dockpipe-0.6-run-37235364232-artifacts`.

### Sixth local repair

- Parenthesize each joined snapshot before comparing them in the MSI smoke test.
  The old expression's operator grouping produced a truthy value for equal snapshots.
  Installer behavior, file removal, and directory/PATH checks are unchanged.
- Extend the existing PowerShell regression to execute the actual smoke predicate
  extracted from its syntax tree. Empty, identical, and identical multi-path snapshots
  must pass; added, removed, and replaced paths must reject. The permanent test failed
  against the original expression and passes with the fix. Workflow argument tests
  still pass with both launcher settings and paths containing spaces.

Local PowerShell 7.6.6 validation, shell-independent embed-manifest verification, and
whitespace checks pass. No engine/package boundary or public contract changes, no
tracked generated files, and no installed-file deletion were introduced. Native MSI
smoke acceptance and combined release output remain pending.

## Approved sixth checkpoint and seventh run

The user approved the four-file follow-up, push, and seventh dry run. Runtime
checkpoint `cp-20261004-213548` created `863ba38bfa2d2735e133cc6392e658c4aa915065`,
with parent `16b10cf5` and all four approved postimages verified. The runtime push
and remote read-back confirm `origin/js/pipelang` at that SHA. The index is empty;
protected editor settings and both stashes are preserved.

[Release run 37236771195](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37236771195)
was dispatched at `2026-10-04T21:36:22Z` with `version=0.6.0`, `dry_run=true`, and
`build_msi=true`. The API confirms the intended SHA. Metadata job `111537335238`
passed and confirms all three inputs. Evidence uses `/tmp/dockpipe-0.6-run-37236771195`.

| Target | Seventh-run result |
| --- | --- |
| Linux amd64 | Job `111537358681` passed store/native smoke, full runtime and eight-package tests, Docker/DEB smoke, and all nine release-tooling tests including signed APT. Artifact `11315717577`. |
| Linux arm64 | Job `111537358722` passed store build and native smoke. Artifact `11316106428`. |
| macOS Intel | Job `111537358724` passed store build and native smoke. Artifact `11315812354`. |
| macOS Apple Silicon | Job `111537358768` passed store build and native smoke. Artifact `11315961672`. |
| Windows amd64 / MSI | Job `111537358719` passed store/native smoke, runtime tests, installer failure recovery, snapshot/binding regressions, WiX MSI build, actual install, installed CLI execution, uninstall, all post-uninstall cleanup/PATH checks, and upload. Artifact `11316396260`. |
| Combined release artifact | Job `111538919622` is waiting on the protected `release` environment before catalog/checksum and dry-run signed-APT assembly. |

The environment API reports required reviewer `jamie-steele`, `prevent_self_review:
true`, and `current_user_can_approve: false`. Administrator bypass is enabled in the
existing environment configuration. The user was asked to clear the gate in GitHub;
no protection rule was changed and no approval/bypass was attempted by the agent.
All five native platform artifacts were downloaded and independently verified:
58 package entries per platform (290 total), including each bundled store archive's
manifest and payload hashes. The MSI payload is present with SHA256
`e1b9b0ed2fa98c57dc42e94dd9b4a795d2cda7aa503135bd9c5e2151a27041d7`.
Evidence is recorded in `/tmp/dockpipe-0.6-run-37236771195-store-verification.log`.
The combined artifact was not produced. The user rejected deployment approval
for dry-run verification and requested a pipeline repair; do not bypass this gate.

## Dry-run deployment isolation repair

The user explicitly requested fixing the pipeline after the dry run incorrectly
requested a deployment approval. The working-tree repair:

- Moves shared catalog/checksum assembly into an unprotected, read-only `assemble`
  job with no production secret references. Dry-run APT uses only a temporary key.
- Keeps production signing and GitHub/R2 publication in the protected `publish`
  job, admitted only for `refs/heads/master` and explicit `dry_run=false`.
  The same admission condition protects dev.to.
- Rejects non-master production dispatch in metadata before platform builds,
  defaults manual dispatch to dry-run, and separates dry-run/production concurrency.
- Transfers prepared production artifacts by a specific artifact name; dry-run
  bundles preserve the existing artifact/catalog/APT layout.
- Documents the required promotion path and adds workflow regression coverage.

All 13 release-tooling tests passed, including real test-key APT signing. The
initial sandbox run could not start the GPG agent; the reviewed host execution
passed with temporary keys and no production credentials/publication. All four new
workflow regression tests reject the original workflow. The metadata tests execute
the actual authored Bash across six refs and four input/event combinations; job
conditions reject dry runs, non-master refs, and missing production-mode outputs.
YAML parsing, 15 Bash step syntax checks, and `git diff --check` passed. Evidence:
`/tmp/dockpipe-release-pipeline-fix-tests-host.log`.

Obsolete run `37236771195` is confirmed `completed/cancelled`. Normal cancellation
left its always-conditioned job waiting, so force cancellation completed the stop.
No environment approval/bypass or protection-rule change was performed.

## Approved pipeline repair checkpoint and eighth run

The user approved committing and pushing the six-file pipeline repair and running
one new dry run. Runtime checkpoint `cp-20261004-235423` created
`66d970e17b9c90b1358ec7a796805c6bc6ef2ce2`, parent `863ba38b`. The exact six
postimages and remote branch tip were verified; editor settings and both stashes
were preserved.

[Run 37245474387](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37245474387)
started at `2026-10-04T23:55:08Z` on `js/pipelang`. Its API head SHA matches the
checkpoint. Metadata job `111562399874` passed and its log confirms `version=0.6.0`,
`dry_run=true`, and `build_msi=true`. Evidence uses
`/tmp/dockpipe-0.6-run-37245474387`.

The run completed with failure. All four Unix jobs passed; Linux amd64's hosted
release-tooling suite includes all 13 tests and the new deployment-isolation
regressions. Windows job `111562426050` failed only
`TestRunContainerAttachedCallsCommitOnHost`: the expected successful log hardcoded
`duration_ms=0`, but the actual successful host-commit log reported `duration_ms=4`.
The runner had already built the Windows CLI/store and passed native smoke.
MSI installation was not reached. Assembly and both production jobs were skipped;
the pending-deployments API returned an empty list. This does not prove successful
unprotected hosted assembly yet.

The local follow-up changes only `src/lib/infrastructure/docker_run_test.go` and
these two evidence documents. The successful host-commit assertion now requires a
numeric nonnegative duration while preserving the complete success/result/path
fields and existing argument/call assertions. A 5 ms delay in the fake commit
operation exercises nonzero duration: the old assertion failed deterministically
with `duration_ms=5`; the corrected Docker-run tests passed 20 repetitions.
No engine behavior changed. Evidence:
`/tmp/dockpipe-0.6-host-commit-duration-negative.log` and
`/tmp/dockpipe-0.6-host-commit-duration-tests.log`.

## Approved timing-test checkpoint and ninth run

The user approved the three-file timing-test follow-up, push, and ninth dry run.
Runtime checkpoint `cp-20261005-012436` created
`d6207aea48f6b4afa57a9c308d388bf083b8217b`, parent `66d970e`. The exact postimages,
protected editor settings, both stashes, and remote `origin/js/pipelang` SHA were
verified. Only the test and two evidence documents were committed; engine behavior
was unchanged.

[Run 37251332865](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/37251332865)
started at `2026-10-05T01:24:57Z` and completed **success** at the intended SHA.
Metadata job `111579411798` confirms `version=0.6.0`, `dry_run=true`, and
`build_msi=true`. Evidence uses `/tmp/dockpipe-0.6-run-37251332865`.

| Target | Ninth-run result |
| --- | --- |
| Linux amd64 | Job `111579436626` passed native/store smoke, runtime and package regressions, and all 13 release-tooling tests. Artifact `11320754864`. |
| Linux arm64 | Job `111579436583` passed native/store smoke and upload. Artifact `11321043170`. |
| macOS Intel | Job `111579436635` passed native/store smoke and upload. Artifact `11321465498`. |
| macOS Apple Silicon | Job `111579436476` passed native/store smoke and upload. Artifact `11320348848`. |
| Windows amd64 / MSI | Job `111579436409` passed runtime tests, installer recovery, MSI regressions, build, install, installed CLI, uninstall, and cleanup. Artifact `11321345243`. |
| Combined artifact assembly | Job `111580908903` started automatically without deployment approval and passed catalog/checksum preparation, temporary-key APT generation, and dry-run artifact upload. Artifact `11320414312`, `dockpipe-release-dry-run-0.6.0`. |
| Production publication | Job `111581218439` skipped. Prepared production artifact upload also skipped. |
| dev.to | Job `111581218441` skipped. |

The pending-deployments API returned an empty list during assembly and after
completion. No environment approval/bypass was performed and no production
publication ran. The obsolete seventh run remains cancelled.

The combined bundle was downloaded to
`/tmp/dockpipe-0.6-run-37251332865-combined`. Independent verification confirmed:

- All five stores, 58 package entries each (290 total), with 42 workflows and
  15 resolvers per store, plus each bundled archive's manifest and payload hashes.
- All 27 top-level checksums, complete platform catalog, Linux installer formats,
  native CLI archives, and the MSI payload.
- APT signature under the temporary key, all four package-index SHA256 entries,
  by-hash paths, and both DEB payloads matching their release counterparts.
- An isolated real APT reader consumed amd64 and arm64 indexes successfully without
  installing packages or changing system sources. Host post-update hooks were
  disabled for the clean isolated check; the first check exited zero but its host
  hook emitted a sandbox connection error.

Evidence: `-verification.log`, `-assemble.log`, `-windows.log`, `-linux-amd64.log`,
and `-apt-reader/reader-no-host-hooks.log` under the evidence prefix above.

### Nonblocking APT metadata cleanup

The initial independent all-entry checksum scan rejected a redundant root
`Release` self-entry: apt-ftparchive scanned the output while writing it, so the
entry describes the initial 167-byte header rather than the final 2385-byte file.
The exact header hash was confirmed. Package indexes and payloads all match their
signed hashes, and the real APT reader accepts the repository. The corrected
verification reports this entry separately; it does not claim the self-checksum
matches the complete file. Preserve `-verification-initial.log` as evidence.
A follow-up can write Release outside the scanned tree and move it into place
before signing, with a regression covering every advertised hash. This is a
metadata cleanup, not a failure of the verified dry-run deployment isolation.

## Remaining release gates

The approved pipeline fix and timing-test follow-up are pushed at
`d6207aea48f6b4afa57a9c308d388bf083b8217b`. Hosted dry-run qualification is complete.
Only the final evidence updates in these two documents remain uncommitted.

Production promotion follows `js/pipelang → js/dev → dev → staging → master` and
remains separately authorized; no merge or production dispatch was performed.
Native hosted success does not establish M6 Mac onboarding, launchd, sleep/wake,
or Nucleon remote execution. Publication-recovery rehearsal and production
configuration/public-origin verification remain separate gates. PipeLang public
rollout and compiler qualification remain deferred to 0.7.


## Staging CI/security repair — 2026-10-05

The user authorized repairing staging CI for PR #18 before release approval and
requested this continuation in the saved checkout. Revalidated `js/pipelang` and
live origin at `d6207aea48f6b4afa57a9c308d388bf083b8217b`; PR #18 remains open,
`staging → master`, at `b29412e0a05061b1c0d93f9213408dcd9fba6a23`, with merge state
`DIRTY`. Master README-only changes must be preserved; merging is not authorized.
The successful release run above is admitted historical proof, not proof of these
new local edits. Commit, push, promotion, and hosted dispatch remain separate gates.

### Implemented local repair

- CI host jobs and `workflows/ci/test` use the existing runtime release selector.
  PipeLang, Application IR, compatibility, and containment harness tests retain
  their separate campaign; no containment check was disabled or compiler
  qualification claimed. Static/security scans still cover compiler source.
- Both scanner install paths pin staticcheck `v0.7.0`, govulncheck `v1.7.0`, and
  gosec `v2.29.0`, with `GOTOOLCHAIN=local`. All three installed successfully using
  the exact Go `1.25.11` toolchain after checksum verification against official Go
  release JSON. The subsequently authorized security update aligns the root/MCP
  modules, CI images, and release build jobs on Go `1.25.13`.
- CodeQL keeps the existing `CodeQL (Go)` check and adds Actions analysis under
  `/language:actions`, matching the coverage required from master. No query,
  severity, protection rule, or alert dismissal was weakened.
- Allocation alerts #1/#17/#18: removed unnecessary capacity addition in the
  environment slice and identifier map helpers.
- MCP path alert #12: repository reads use `os.Root`, including search reads;
  traversal and escaping symlinks cannot redirect reads outside the selected
  repository. Search reads enforce their byte limit during I/O.
- Broker path alerts #13–#16 share a result-path source: domain ID validation
  already rejects separators. The filesystem boundary now also enforces a single
  local component before constructing either read or write paths. Generic durable
  validators remain intact. Remote private reads additionally bind the opened
  file identity to validation and enforce the 64 MiB limit during I/O.
- The existing JSON-only govulncheck command returned zero for vulnerable code.
  Converting the saved scan to text now supplies a failing vulnerability exit
  status while retaining JSON for DorkPipe. The existing scan fails with exit 3.
- gosec excludes `testdata` compilation fixtures, consistent with Go package
  discovery. This removes duplicate-declaration load errors; all 41 reported
  findings remained present before repair. No source package or rule is excluded
  by this change.

### Validation and evidence

Evidence prefix: `/tmp/dockpipe-ci-`. The tools and exact Go version are under
`/tmp/dockpipe-ci-repair-tools`; no generated binaries or caches were added to Git.

| Check | Result |
| --- | --- |
| Exact Go 1.25.11 tool installation | Passed; `tool-install.log` records binary module/build versions. |
| Runtime selector on host | Passed 43 package suites; three packages have no tests (`runtime-host.log`). |
| Operation identifiers, remote infrastructure, MCP suites | Passed on host (`focused-host.log`). Initial sandbox failure is preserved in `focused.log`: loopback sockets were denied. |
| New path regressions | Passed (`regressions.log`); old MCP read calls fail the symlink regression using a temporary Go overlay (`mcp-negative.log`). |
| Durable-state validation tests | Passed (`durable.log`), including owner-only and symlink checks. |
| Windows remote/MCP tests | Cross-compiled successfully to `/tmp`; native execution is still required. |
| CLI | Built successfully to `/tmp/dockpipe-ci-repaired-cli`. |
| CI workflow regressions | Four tests passed: selector scope, host/nested parity and pins, vulnerability failure status, and Go/Actions coverage. |
| Workflow/schema and shell | Nested workflow validates; selector Bash syntax and ShellCheck error gate pass; `git diff --check` passes. |
| Hosted CodeQL/CI | Not rerun: requires a separately authorized checkpoint/push/promotion. Alert closure is unverified locally. |

### Broader scanner repair authorized and implemented

Restoring scanner installation exposed **121 staticcheck diagnostics across 60
files** (127 output lines including six secondary explanations): 68 deprecated
`runtime.GOROOT` calls, 26 unused declarations, 18 error-string style findings,
six possible nil dereferences in tests, and three other diagnostics. Most are in
PipeLang/compiler tests. Raw evidence: `staticcheck.log`; summary:
`scan-summary.json`.

Gosec initially reported **41 findings** after fixture exclusion with zero package-load
errors: 13 conversion bounds, one deterministic PRNG use, seven slice bounds,
and 20 unchecked errors. These span the compiler, its transcript harness, and
runtime code. Raw evidence: `gosec-no-fixtures.json` and `.log`; the initial
fixture-load failure remains in `gosec.json` and `.log`.

Govulncheck identified **seven reachable standard-library vulnerabilities** in
Go 1.25.11. All have fixes by **Go 1.25.13**: GO-2026-4970, GO-2026-5026,
GO-2026-5856, GO-2026-5972, GO-2026-6089, GO-2026-6090, and GO-2026-6218.
Evidence: `govulncheck.json` and `govulncheck-gate.log`. A same-minor security
patch is justified; upgrading to Go 1.26 merely to install latest tools is not.
Toolchain changes must align release build inputs and require fresh qualification
before claiming the release artifacts are secured by the patch.

The user explicitly approved the broader compiler/harness fixes and Go 1.25.13
update ("of course"). The local implementation now:

- Resolves the selected installed Go toolchain in generated-code tests. Retained
  campaigns pass `PIPELANG_TEST_GO`; ordinary tests use their exported `GOROOT`
  or installed Go. The helper rejects version mismatch, and retained artifact
  fingerprints hash that same selected root. Process containment is unchanged.
- Removes 26 unused private declarations; corrects error-string style and test
  nil guards; preserves active compiler entry points and emitted output.
- Checks narrowing conversions and compressed-section bounds in transcript
  tooling, retains deterministic self-test noise, checks symbol-ID capacity,
  and uses direct element references for diagnostic annotation.
- Handles artifact-close failure, explicitly preserves primary failures during
  cleanup, and makes infallible in-memory C++ emitter writes clear to gosec.

### Final local validation of the broader repair

| Check | Result |
| --- | --- |
| Staticcheck v0.7.0 | Passed with zero diagnostics (`staticcheck-accepted.log`). |
| Gosec v2.29.0 | Passed, 282 files / 75,586 lines, zero findings and zero package-load errors (`gosec-accepted.json`); existing two suppressions unchanged. |
| Govulncheck v1.7.0, Go 1.25.13 | No vulnerabilities found; text-conversion gate exits zero (`govulncheck-patched.json`, `govulncheck-patched-gate.log`). Original vulnerable exit-3 evidence retained. |
| Runtime selector, Go 1.25.13 | All 43 package suites passed (`runtime-patched.log`); three packages have no tests. |
| MCP package | Passed (`mcp-patched.log`), including symlink and bounded-read regressions. |
| Core evaluator / Core IR / Go backend | Full component unit suites passed (`core-components.log`). |
| Toolchain resolution / transcript widths | New unit tests passed; transcript self-test passed 21 round trips (`transcript-selftest.log`). |
| Focused compiler campaign | 38/38 selected logical cases passed, no reused semantic receipts; `contained-suite/suite/summary.json` explicitly records partial proof and unchanged source/toolchain during execution. Aggregate job exits zero with process-tree cleanup (`contained-suite/job-focused.json`). A later edit only groups two test-file imports. |
| Application IR generated code | Numeric-comparison and v115 assignment consumers passed under containment (`appir.output`, `appir-job.json`), including process-tree cleanup. |
| Updated-toolchain cross-builds | Linux and Windows CLI builds and Windows remote/MCP test binaries compiled with Go 1.25.13; native Windows execution remains hosted proof. |
| Containment safety | Three job safety probes passed (`containment-preflight.log`). |
| CI/release tooling | All 18 tests passed on the host (`release-tooling-host.log`), including five new CI contract regressions. Initial sandbox GPG-socket denial retained separately. |
| Go vet | Passed across `./...` on Go 1.25.13 (`vet-final.log`). |
| Shell / workflow | ShellCheck error gate and templates/core path guard passed; nested workflow validates; YAML/Python parsing passed. |

The focused compiler evidence preserves unsuccessful preparation attempts:
`contained/focused.json` stopped at the existing 800 MiB proactive memory limit;
`contained-suite/suite/campaign/attempts/f726ddf6babc407b94cecefb01c7527e`
hit the unchanged 30-second cold-build deadline. Resume completed preparation
without increasing limits. A selection-file error named generated test functions
embedded in fixture strings; correcting only the temporary selection to the 38
actual parent tests allowed the focused campaign to execute. The parent tests
still execute their generated tests. No failed receipt was removed or relabeled.

Full compiler/Application IR qualification remains a separate campaign; this is
focused regression proof, not 0.7 language qualification. Hosted CI, CodeQL alert
closure, Docker workflow execution, and fresh release artifacts on Go 1.25.13
remain unverified. The earlier release dry run is historical proof only.
The user subsequently approved committing and pushing this reviewed repair to
`js/pipelang`. The checkpoint/push is being prepared against parent `d6207ae`;
promotion, hosted dispatch, merge, and release publication remain separate gates. Protected editor settings,
both stashes, and the empty index retain their admitted state.


## Hosted shell dependency follow-up — 2026-10-05

The approved repair was committed through runtime checkpoint `cp-20261005-040503`
and pushed to `origin/js/pipelang` as `319543968dc43b481dee4c1677d1ca719655ee7c`.
Exact 92-path postimages, remote tip, protected settings, and both stashes were
verified. Receipt: `/tmp/dockpipe-0.6-ci-security-checkpoint/receipt.json`.

The user reported shell-test failure after subsequent promotion. Read-only
comparison confirms staging `364b3a498ba842a844e99d7f4613a30037055181` has no source
differences from that repair. CI run `37263035986` passes Linux security scans,
runtime tests, staticcheck, ShellCheck, templates/core guard, the Docker test
workflow, and DEB build; Windows passes. CodeQL run `37263035945` passes as well;
individual alert closure has not been independently read back.

The Linux **Shell unit tests** step fails because `rg` is absent:

- `test_repo_layout.sh` explicitly rejects missing ripgrep.
- `test_backlog_remote_workflow.sh:206,211` cannot run its source guards.
- `test_ide_state_ownership.sh:133,139` cannot run its source guards and fails.

Log: `/tmp/dockpipe-ci-37263035986-failed.log`. Docker smoke and DEB installation
within the shell step pass; the later Docker integration step is skipped after
the shell failure. The release workflow already explicitly installs ripgrep;
the ordinary CI workflow omitted it. The local fix adds `ripgrep` to the existing
Linux dependency installation and names that step for build and shell-test
requirements. No runtime, package code, test selection, or failure gate changes.

Local repository-layout and IDE state-ownership checks pass with installed
ripgrep. All five CI contract tests and YAML parsing pass. The standalone backlog
fixture initially encountered the sandbox's read-only user-state directory;
that failure is preserved in `/tmp/dockpipe-ci-ripgrep-backlog.log`. An isolated
sandbox retry also hit denied loopback sockets (`backlog-isolated.log` under the
same prefix). The host retry with temporary XDG state/cache paths passed the
complete fixture (`/tmp/dockpipe-ci-ripgrep-backlog-host.log`). Dependency-step
Bash syntax and `git diff --check` also pass. Hosted revalidation remains
pending the follow-up checkpoint/push and promotion. The user approved committing
and pushing the three-file dependency fix and evidence update to `js/pipelang`.
The runtime checkpoint is being prepared against `3195439`; promotion and hosted
dispatch remain separate gates.

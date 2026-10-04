# 0.6 hosted release qualification — 2026-10-04

## Objective and authority

- Objective: `dockpipe-0.6-release-qualification`.
- State: the approved fifth repair is committed and pushed as `16b10cf5`.
  All four Unix jobs passed in the sixth run. Windows built and installed the
  MSI, then hit a false-positive snapshot assertion after uninstall. The local
  assertion repair is verified and awaits commit/push approval.
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

## Remaining release gates

Current committed/pushed HEAD remains `16b10cf5574d77951909b24b06103c0a424e36f1`.
The four-file follow-up (MSI smoke predicate, PowerShell regression, and two evidence
documents) is local and needs new commit/push approval before a seventh dry run.
The index is empty; protected editor settings and both stashes are preserved.

The next exact SHA still needs the corrected Windows uninstall assertion and
remaining cleanup checks to pass, followed by the complete five-platform catalog,
checksums, and dry-run signed APT artifact. Native hosted success does not establish
M6 Mac onboarding, launchd, sleep/wake, or Nucleon remote execution.
Publication-recovery rehearsal and production configuration/public-origin verification
remain separate gates. PipeLang public rollout and compiler qualification remain
deferred to 0.7.

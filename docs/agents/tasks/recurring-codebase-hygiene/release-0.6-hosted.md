# 0.6 hosted release qualification — 2026-10-04

## Objective and authority

- Objective: `dockpipe-0.6-release-qualification`.
- State: failed_verification; the approved repair checkpoint is committed and
  pushed. The second run is terminal with three successful native platforms and
  two remaining failures. The user approved the eight-path follow-up repair
  checkpoint and another dry run on 2026-10-04.
- Execution skill: `dorkpipe-objective-execution`.
- Authority: user-requested continuation of the approved hosted dry run.
- Done when: the intended SHA and dry-run inputs are verified, the run is terminal,
  and all five stores, native smoke results, MSI checks, signed APT tests,
  catalog completeness, and checksums have passing evidence or exact recorded failures.
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

## Remaining release gates

Native hosted success does not establish M6 Mac onboarding, launchd, sleep/wake,
Docker, or Nucleon remote execution. Publication-recovery rehearsal and production
configuration/public-origin verification remain separate gates. PipeLang public
rollout and compiler qualification remain deferred to 0.7.

The user approved the 13-file repair checkpoint on 2026-10-04. At preparation,
HEAD remained `afc0a2cf`; commit/push and the second dry run were pending. An
unrelated untracked `.vscode/settings.json` is excluded and preserved. Raw logs, run JSON, downloaded artifacts, and local validation output
are under `/tmp/dockpipe-0.6-run-37220714083` (job logs also use that prefix).
The authored `embed_assets.go` manifest was regenerated with
`release/packaging/embedded-inputs.py` for the two added fixture files. No package
store or ignored runtime output was refreshed. Package behavior remains in package
hooks/tests in the first repair; the second repair changes only generic Windows
private-state validation and inventory error propagation in the engine. No public
schema changed. Both protected stashes, unrelated editor settings, and the current
`js/pipelang` checkpoint remain intact. At preparation of the approved second repair checkpoint, HEAD remained
`5491c3f3`; its commit/push and third dry run were pending.

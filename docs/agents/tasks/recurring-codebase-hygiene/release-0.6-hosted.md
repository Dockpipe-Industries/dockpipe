# 0.6 hosted release qualification — 2026-10-04

## Objective and authority

- Objective: `dockpipe-0.6-release-qualification`.
- State: failed_verification; hosted run is terminal, local repairs passed focused
  checks. The user approved committing/pushing the 13 repair/evidence files and
  another dry run; that checkpoint is now being executed.
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
hooks/tests; no engine code or public schema changed. Both protected stashes and
the original branch/HEAD remain intact.

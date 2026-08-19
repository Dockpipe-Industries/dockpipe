**Windows native deterministic SQLite failure-boundary matrix — 2026-08-04.** The new Windows-only
`TestWindowsNativeSQLiteFailureBoundaryMatrix`, gated by
`DORKPIPE_SQLITE_FAILURE_MATRIX=1`, passed with `CGO_ENABLED=0`, `-mod=readonly`, `-count=1`, verbose
output, and the fixed 30-minute timeout. It ran on Windows build `10.0.26200`, `amd64`, Go
`go1.26.4`, fixed NTFS volume `\\?\Volume{2eb284d8-09e6-483c-b096-6deed2208642}\` with serial
`88c9a133` and label `OS`; the unprivileged NTFS-version query remained unavailable. The canonical
temporary root and every scenario directory, main database, and observed journal were owned by
current-user SID `S-1-5-21-2729925100-2499202611-1015899381-1002` and granted full control only to
that SID and `SYSTEM` (`S-1-5-18`).

Every fresh per-attempt database revalidated SQLite `3.53.3`, source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`,
native `win32`, the selected absolute `mode=rw` / `cache=private` / `_txlock=exclusive` / `_dqs=0` /
`_error_rc=1` URI, every selected pragma and exact readback, the singleton STRICT schema and
`user_version=1`, and the exact 57-entry compile-option set listed above. The LF-terminated exact-set
SHA-256 was `e08918d66caa484a6929317e24d92db1d6a078fc115dbf15adb10994f869babf`.
Each database held exactly one bounded canonical JSON BLOB with adapter, session, revision,
unknown-outcome, permanent-no-replay, envelope, and SHA-256 equality. Child processes used a strict
bounded JSON-line protocol on `stderr`, isolated from Go test output on `stdout`; every command and
response carried exact scenario, cycle, attempt, operation, checkpoint, database, and session
identity. Missing, duplicate, substituted, cross-scenario, malformed, or out-of-order traffic failed
closed. Children performed only ordinary SQLite operations and never cleaned or altered physical
store files.

The complete 22-attempt result table follows. `Harness` means the named application checkpoint or
response loss was injected and is not a SQLite/OS result. `Native` means the recorded outcome was
genuinely returned by SQLite or Windows process termination/recovery. Each row began at exact
revision `1`; the table binds its session-specific initial and final digest exactly:

| Scenario | Attempt and boundary | Evidence kind | Authoritative classification | Initial revision / SHA-256 | Final physical revision / SHA-256 |
| --- | --- | --- | --- | --- | --- |
| `01_before_open` | 1; reject before database open | Harness | rejected | 1 / `d11618d4938743fbe7582c37b3ec38f5e480f1a0e7b4ca97a19f46b214abb689` | 1 / `d11618d4938743fbe7582c37b3ec38f5e480f1a0e7b4ca97a19f46b214abb689` |
| `02_contract_reject` | 1; substituted contract evidence before observation | Harness | rejected | 1 / `32e409176da2a0c3bd010747fbd9cce2ac41c6c8b3be4c002989692b68190a07` | 1 / `32e409176da2a0c3bd010747fbd9cce2ac41c6c8b3be4c002989692b68190a07` |
| `03_contention` | 1; same-session lock before observation | Native primary `SQLITE_BUSY` (`5`) | rejected | 1 / `1fca77d81f99a9dca417372de397a9fa801da364e3e140526cee574884577a4a` | 1 / `1fca77d81f99a9dca417372de397a9fa801da364e3e140526cee574884577a4a` |
| `04_cancel_after_observation` | 1; cancellation after exact old observation | Harness plus genuine rollback/reload | rejected | 1 / `a6db7755613e0a36301f372c966dbceef77da2e058634ba5fbbaf26cec93ea94` | 1 / `a6db7755613e0a36301f372c966dbceef77da2e058634ba5fbbaf26cec93ea94` |
| `05_stale_cas` | 1; stale session | Native zero rows plus rollback/reload | known unchanged | 1 / `6cf53d46e2f230531e71a9b9e038dfa69a836adcc2086a801d496ab6202508fb` | 1 / `6cf53d46e2f230531e71a9b9e038dfa69a836adcc2086a801d496ab6202508fb` |
| `05_stale_cas` | 2; stale revision | Native zero rows plus rollback/reload | known unchanged | 1 / `d4fa724c2c7feb3812383febed9c94b5dcb300f6c4f5d9f0864c969bc87ebdbf` | 1 / `d4fa724c2c7feb3812383febed9c94b5dcb300f6c4f5d9f0864c969bc87ebdbf` |
| `05_stale_cas` | 3; stale digest | Native zero rows plus rollback/reload | known unchanged | 1 / `e324953707b27c5af0598cab81f0764c727cd7b445b1d137ecc35aae1a7c0ea3` | 1 / `e324953707b27c5af0598cab81f0764c727cd7b445b1d137ecc35aae1a7c0ea3` |
| `06_after_begin` | 1; injected loss after begin before CAS | Harness plus genuine rollback/reload | known unchanged | 1 / `faaa05f23a2c00cf7301f3fee428254ec19f2eb72419c2a012882f8960af16bf` | 1 / `faaa05f23a2c00cf7301f3fee428254ec19f2eb72419c2a012882f8960af16bf` |
| `07_after_stage` | 1; injected loss after exact CAS staging before commit | Harness plus genuine rollback/reload | known unchanged | 1 / `91f371614445768866b3e0fb9a32890ded0f4d6e15d2296054ef295e0de0c31f` | 1 / `91f371614445768866b3e0fb9a32890ded0f4d6e15d2296054ef295e0de0c31f` |
| `08_terminate_precommit` | 1; forced termination after staging before commit | Native termination/hot-journal recovery | known unchanged | 1 / `b721a5187f4f556565cfd7644037321dba1360f7611e9182d1767aa03578a05a` | 1 / `b721a5187f4f556565cfd7644037321dba1360f7611e9182d1767aa03578a05a` |
| `09_rollback_proof_loss` | 1; forced loss prevents rollback/old-row proof | Harness loss plus later native physical recovery | `unknown_commit_result` | 1 / `0b0c1ce8d1e351f1f95ab480f8112e9c89361e30550308f4e04f598e8c0fdd46` | 1 / `0b0c1ce8d1e351f1f95ab480f8112e9c89361e30550308f4e04f598e8c0fdd46` |
| `10_commit_call_loss` | 1; forced termination from inside SQLite's commit hook after the write-transaction and exclusive-lock checks, before commit phase one or result availability | Native SQLite commit-hook observation plus harness termination | `unknown_commit_result` | 1 / `43bef391b42f6b51b4c67517efe00e7c97dda0eabca6ed52975980468ed0923f` | 1 / `43bef391b42f6b51b4c67517efe00e7c97dda0eabca6ed52975980468ed0923f` |
| `11_genuine_commit_error` | 1; genuine commit error attempt | Proven unreachable under selected exclusive shape; control commit genuinely succeeded | committed | 1 / `538c917b0fc4360a9f1337b5f04a3f0baf9e7436adcc21e1f6374e679587216e` | 2 / `7c0dd65cc2a2850d7fb6dfde8e3bd9142cf26503b74ddcc575fc9c170588e4d8` |
| `12_response_loss` | 1; genuine commit success, caller response lost before reload | Harness | `unknown_commit_result` | 1 / `32097cd580bc4bb23bbdb3a84dc6ea953d9233840965514258386a3bf66e5410` | 2 / `fa586651472644b528aff5c042e98cc78284b0508048e4d63d124de173ff895d` |
| `13_validation_loss` | 1; schema validation result lost after exact reload | Harness | `unknown_commit_result` | 1 / `604abe29c761b3c1f6fd4d303e2d59e2b97414dac714d3d361a76d28124af792` | 2 / `53f1d118dc76593ec110233467c23e292fe769bf480317957cfc908be043a214` |
| `13_validation_loss` | 2; identity validation result lost | Harness | `unknown_commit_result` | 1 / `95f5a71c86ed8215865cf6880460dc1b954aebfde94d882b4378db91f7379eed` | 2 / `37ba916c7c427488bb0482f5e9490a176a1ec91157a74028748df1c5240fc7e5` |
| `13_validation_loss` | 3; digest/envelope validation result lost | Harness | `unknown_commit_result` | 1 / `e2351ed2ad87340ac3671ad728083354787307b52e0b021f441ea31f506f2972` | 2 / `0790b0985f1a1d5782a9dc46cca025a204a952c6a5352c0e5147dadccaf57da3` |
| `13_validation_loss` | 4; sibling validation result lost | Harness | `unknown_commit_result` | 1 / `b9fbd37af1ec09c17bade131abbadd653640c9ce8c4e41ef1f1d457cd89cf9a9` | 2 / `73f98f28c9ba21837cfe1045e7c3cc165a6100030f44c808e6ac04dc777df357` |
| `13_validation_loss` | 5; DACL validation result lost | Harness | `unknown_commit_result` | 1 / `0ede316defffda98a5b2751ade8a26607433a6721809e5f0e2223694ebb9ec9e` | 2 / `39651e0ab7acd41405c527582dc669fae31372148a959e6b1149a379b4669781` |
| `14_close_result_loss` | 1; successful close result lost | Harness | `unknown_commit_result` | 1 / `308fa6348158fc37f3d4f8e639f63666065850bda7d2c7dd306637e70b71a936` | 2 / `d4ad3b44fc10c58caeed27efbee783e3a7bef2188e88b1e793f17d56eb43f0ae` |
| `15_ack_loss` | 1; complete path followed by acknowledgement loss | Harness | `unknown_commit_result` | 1 / `09d2cabd2f6b285735e8b9206d463c89036333c547a08134d7417f5f31e42877` | 2 / `8711029d8629c4ac052c7a963f7f0d15e99f39a7184eab6a772e67e1febf9c9d` |
| `16_success` | 1; full validated path and one acknowledgement | Native | committed | 1 / `2a10f300002f05f312429cfc3c9ee12629fb6c127927be866a23096b15640717` | 2 / `792629f5e19b1b26eb3ca65bb91f19b0f122f2f0b9485236b90ae0615b1f5927` |

The row-9 fresh recovery described physical old state only and did not retroactively invent an
earlier acknowledgement. Row 10 is now deterministically reached through the pinned driver's public
`sqlite.HookRegisterer.RegisterCommitHook` surface obtained from the dedicated
`database/sql.Conn.Raw` connection. The generated SQLite engine invokes that callback from inside
`_vdbeCommit` only after it has found the live write transaction and acquired the pager's exclusive
lock, and before `_sqlite3BtreeCommitPhaseOne`. The callback writes the one strict child-protocol
checkpoint from that native call stack and then blocks without returning. Only after the parent has
validated the exact scenario, cycle, attempt, operation, checkpoint, database, session, and
`commit_invoked=true` / `commit_returned=false` evidence does it terminate the child. Fresh
recovery returned the exact old row. The application outcome remains `unknown_commit_result`; the
injected process loss is not reported as a SQLite error or Windows storage result. Row 11 is
genuinely unreachable under this exact shape without changing SQLite, the
driver, filesystem, or protected code: an independent same-session owner is rejected with
`BUSY/LOCKED` at acquisition before observation and therefore cannot retain a conflicting lock at
commit; the control transaction returned genuine success. No error code was fabricated.

**Exact local commit call-chain qualification — 2026-08-04.** The reviewed standard-library source
was Go `go1.26.4` at `C:\Program Files\Go\src\database\sql\sql.go:2287-2319` and
`C:\Program Files\Go\src\database\sql\driver\driver.go:518-522`. The reviewed pinned module source
was `modernc.org/sqlite v1.56.0` at
`C:\Users\Jamie\go\pkg\mod\modernc.org\sqlite@v1.56.0`, with its required
`modernc.org/libc v1.74.4`. The exact path is:

```text
(*database/sql.Tx).Commit
  -> driver.Tx.Commit through tx.txi under database/sql's driverConn lock
  -> (*modernc.org/sqlite.tx).Commit
  -> (*sqlite.tx).exec(context.Background(), "commit")
  -> sqlite3.Xsqlite3_exec
  -> sqlite3.Xsqlite3_prepare_v2
  -> sqlite3.Xsqlite3_step
  -> _sqlite3Step
  -> _sqlite3VdbeExec
  -> _sqlite3VdbeHalt
  -> _vdbeCommit
  -> detect the live write transaction and acquire the pager exclusive lock
  -> FxCommitCallback / modernc commitHookTrampoline / test callback
  -> _sqlite3BtreeCommitPhaseOne
  -> _sqlite3BtreeCommitPhaseTwo
  -> return through sqlite3_exec, modernc tx.Commit, and database/sql Tx.Commit
```

The driver locations were `tx.go:34-78` for its commit/exec path,
`sqlite.go:618-622` for `HookRegisterer`, and `pre_update_hook.go:53-67,205-215` for commit-hook
registration and dispatch. The generated Windows/amd64 engine locations were
`lib/sqlite_windows.go:5803-5924` for `Xsqlite3_exec`,
`lib/sqlite.go:11963-12022` for `Xsqlite3_step`, and
`lib/sqlite_windows.go:93901-93973,104342-104538,116055-116163` for `_sqlite3Step`, VDBE halt,
the commit-hook boundary, and the two commit phases.

Every candidate observation or interception point was classified explicitly:

- a marker immediately before `Tx.Commit`, entry to a wrapper `driver.Tx.Commit`, goroutine start,
  timer, sleep, or parent-side kill race is too early because the underlying commit may not have
  begun;
- the Go `database/sql` test hooks cover connection return, transaction connection grabbing, and
  rollback, but expose no post-driver-commit-entry/pre-result hook;
- driver connection hooks run at connection setup; pre-update/update hooks run while staging the
  row; rollback hooks run on rollback; authorizer and statement-trace callbacks can run at prepare or
  statement-start boundaries; and progress callbacks are opcode-cadence dependent. None proves the
  selected exact commit boundary as strongly as the native commit hook;
- suppressing a result after `Tx.Commit`, dropping the child response, or relabeling the existing
  response-loss row is too late because the commit result was already observed in the child;
- debugger breakpoints, runtime/symbol patching, a replacement driver, a custom VFS, and direct
  SQLite/pager instrumentation would require a different lower-level harness or dependency surface;
  filesystem/journal observation is both insufficient and prohibited by this evidence contract; and
- the accepted `RegisterCommitHook` callback is the exact seam: SQLite itself calls it from
  `_vdbeCommit` after the write-transaction and exclusive-lock checks and before phase one. The test
  callback never returns, never supplies a nonzero abort code, and never observes or suppresses a
  commit result.

The exact aggregate counters were: rows attempted `22`; rows proven natively `7`; harness-injected
application-boundary rows `14`; rows proven unreachable `1`; rows still unproven `0`; known unchanged
`6`; committed `2`; rejected `4`; `unknown_commit_result` `10`; recovery-only opens `22`; exact old
recoveries `12`; exact new recoveries `10`; genuine `BUSY/LOCKED` before observation `1`, after
observation `0`, and at commit `0`; successful different-session commits `1`; rollback attempts and
exact-old proofs `6` / `6`; forced terminations `4`; commit invocations and genuine return
observations `12` / `11`; success acknowledgements `2` (the independent different-session control
and the full-success row). Duplicate commits, retries, replays, repairs, fallbacks, partial rows,
ambiguous pre-commit recoveries, staged-row leaks, revision gaps/duplicates, digest/envelope
mismatches, unexpected siblings/protection widening, and protocol loss/duplication/reordering were
all `0`.

The clean matrix elapsed time was `3.893s`. The canonical-root pre/post metadata-tree SHA-256 values
were `01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b` and
`527048ffa7ddd5c489824413b8b38a23d70609ace1e728e58dcc8966e38e765e`. The rollups over every
scenario's exact pre/post metadata hash were
`7ba2eb2fd9acaf76c15a9139f494fd31c2515498757db1d162002dcc3e05b7a5` and
`596eb9e3504a83a46846de9e79a4c9de04de66c37b224513f9a42e50590dccc7`.
Each metadata-tree hash is SHA-256 over LF-terminated, ordinally sorted rows containing relative path,
entry type, byte size, and exact owner/DACL evidence. The rollup binds scenario ID, attempt, and its
tree hash. No journal was opened or hashed for contents. Every scenario admitted only exact
`aggregate.sqlite` / `aggregate.sqlite-journal` regular siblings; journals retained the exact private
DACL. Only the parent test framework removed the canonical temporary root after all children and
connections closed.

Required validation then produced these exact results, in order:

```text
DORKPIPE_SQLITE_FAILURE_MATRIX=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteFailureBoundaryMatrix$' -count=1 -v -timeout=30m
PASS; matrix elapsed 3.893s; rows proven natively 7; rows still unproven 0; commit invocations/returns 12/11
DORKPIPE_SQLITE_CONTENTION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteContentionCohort$' -count=1 -v -timeout=30m
PASS; cohort elapsed 2m26.596s; all existing counters and 9e4b...11b8 pre/post hash unchanged
DORKPIPE_SQLITE_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
PASS; cohort elapsed 3m56.453s; all existing counters and dd67...0aaa pre/post hash unchanged
DORKPIPE_SQLITE_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteSmoke$' -count=1 -v
PASS; smoke elapsed 1.46s; primary SQLITE_BUSY 5 and exact revision-2 recovery unchanged
CGO_ENABLED=0 GOOS=<windows|linux|darwin> GOARCH=<amd64|amd64|arm64> go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
PASS; embedded CGO_ENABLED/GOOS/GOARCH matched; binary sizes 12205568 / 11044675 / 10943842 bytes; verified temporary directory removed
go test -mod=readonly ./appserversupervisor ./providersession -count=1
EXPECTED PROTECTED FAILURE; providersession passed; selector assertion remained 17 instead of 1
go test -mod=readonly ./... -count=1 -timeout=90s
EXPECTED PROTECTED FAILURES; selector assertion unchanged; sqliteevidence passed; orchestrationhelper timed out in TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsMalformedDecisionFixtures/malformed
go mod verify
PASS; all modules verified
gofmt -d appserversupervisor/sqliteevidence/windows_failure_boundary_matrix_test.go
PASS; empty output
git diff --check
PASS
```

The full-suite timeout remained in the same protected placement-execution fixture chain and the same
pre-existing `os.ReadFile` path, but its active subtest moved from the preceding
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRejectsTargetSetConflicts/stale_version`
case to
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsMalformedDecisionFixtures/malformed`.
No protected App Server, Pipeon, or orchestration-helper path was edited. Clean cross-target evidence
binaries were inspected outside the repository and the verified directory
`dockpipe-sqlite-failure-cross-769ef3630a344e2e9359f6df6603a836` was removed. The updated
failure-matrix file SHA-256 is
`e15d602c8945a0852a6c388702c8242dfce0a9c9e17959caf4f7a18d9b933077`.

Row 10 is now genuinely reachable and proven. The complete deterministic matrix is still not
claimed closed because the required, genuinely unreachable row 11 remains recorded rather than
simulated. Windows reboot/power-loss trials, broader Linux publication/contention/failure cohorts,
macOS/arm64 GitHub Actions evidence intentionally scheduled last, macOS VM
disruption evidence if still required, complete production host/sidecar acceptance, production
storage, migration, cutover, recovery authority, dispatch/projection/decision integration, and Slice
2 all remain open. TASK-013 and CAS-14 remain open.

The completed dependency-pin/smoke, publication, and contention/forced-termination slices do not
claim the deterministic failure-boundary matrix, Windows VM reboot or hard-power-loss durability,
complete sidecar qualification, a production host allowlist, broader Linux native cohorts,
macOS/arm64 GitHub Actions evidence (intentionally last), or macOS VM disruption evidence if still
required. They add no production store, migration, cutover, recovery authority, lifecycle dispatch,
Pipeon projection, or Slice 2 work. Those gates remain open; TASK-013 and CAS-14 remain open.


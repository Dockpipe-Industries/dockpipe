**Linux/amd64 native deterministic SQLite failure-boundary matrix — 2026-08-05.** The new
Linux-only `TestLinuxNativeSQLiteFailureBoundaryMatrix`, gated by
`DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX=1`, passed natively with `CGO_ENABLED=0`,
`-mod=readonly`, `-count=1`, verbose output, and the fixed 30-minute timeout. The exact command,
run from `packages/dorkpipe/lib`, was:

```text
TMPDIR=/tmp/dockpipe-sqlite-linux-failure-0a13OJYL DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteFailureBoundaryMatrix$' -count=1 -v -timeout=30m
PASS; matrix elapsed 739ms; package ok in 0.748s
```

The caller created the new private parent `/tmp/dockpipe-sqlite-linux-failure-0a13OJYL` outside
the repository with mode `0700`, owner UID/GID `1000:1000`, stat device `66311`, and inode
`57176043`. The test-owned canonical root was
`/tmp/dockpipe-sqlite-linux-failure-0a13OJYL/TestLinuxNativeSQLiteFailureBoundaryMatrix1728090748/001`,
also owned `1000:1000` with mode `0700`. Its retained identity was mount ID `33`, device
`259:7`, inode `57176201`, and kind `directory`. Metadata-only `statx`,
`O_PATH|O_NOFOLLOW` plus `fstatfs`, and `/proc/self/mountinfo` agreed on ext4 magic
`0xef53`, source `/dev/nvme0n1p3`, mount root/point `/` / `/`, mount options
`rw,noatime`, and super-options `rw,errors=remount-ro,stripe=64`. The exact mountinfo row was:

```text
33 2 259:7 / / rw,noatime shared:1 - ext4 /dev/nvme0n1p3 rw,errors=remount-ro,stripe=64
```

The run used Pop!_OS 22.04 LTS, Linux `7.0.11-76070011-generic`, kernel build
`#202606011647~1780583630~22.04~70ad774 SMP PREEMPT_DYNAMIC Thu J`, bare metal according to
`systemd-detect-virt`, `amd64`, and Go `go1.25.0`. The source block device and temporary
parent were non-removable. The lane rejected symlink, substitution, bind, nested, overlay, FUSE,
network, removable, shared-host, `drvfs`, `9p`, `tmpfs`, and cross-mount storage. Every
scenario root and `main` / `other` database directory remained an owned `0700` directory.
Every database and observed rollback journal remained an owned regular `0600` file on the exact
retained mount/device identity. Only `aggregate.sqlite` and
`aggregate.sqlite-journal` were admitted as database siblings.

Every fresh attempt revalidated SQLite `3.53.3`, source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`,
native `unix` VFS, the selected absolute `mode=rw` / `cache=private` /
`_txlock=exclusive` / `_dqs=0` / `_error_rc=1` URI, exact pragma readbacks, the singleton
STRICT schema, and `user_version=1`. The exact sorted 56-option Linux contract was:

```text
ATOMIC_INTRINSICS=1,COMPILER=gcc-12.2.0,DEFAULT_AUTOVACUUM,DEFAULT_CACHE_SIZE=-2000,
DEFAULT_FILE_FORMAT=4,DEFAULT_JOURNAL_SIZE_LIMIT=-1,DEFAULT_MEMSTATUS=0,DEFAULT_MMAP_SIZE=0,
DEFAULT_PAGE_SIZE=4096,DEFAULT_PCACHE_INITSZ=20,DEFAULT_RECURSIVE_TRIGGERS,
DEFAULT_SECTOR_SIZE=4096,DEFAULT_SYNCHRONOUS=2,DEFAULT_WAL_AUTOCHECKPOINT=1000,
DEFAULT_WAL_SYNCHRONOUS=2,DEFAULT_WORKER_THREADS=0,DIRECT_OVERFLOW_READ,DISABLE_INTRINSIC,
ENABLE_COLUMN_METADATA,ENABLE_DBPAGE_VTAB,ENABLE_DBSTAT_VTAB,ENABLE_FTS5,ENABLE_GEOPOLY,
ENABLE_MATH_FUNCTIONS,ENABLE_MEMORY_MANAGEMENT,ENABLE_OFFSET_SQL_FUNC,ENABLE_PREUPDATE_HOOK,
ENABLE_RBU,ENABLE_RTREE,ENABLE_SESSION,ENABLE_SNAPSHOT,ENABLE_STAT4,ENABLE_UNLOCK_NOTIFY,
LIKE_DOESNT_MATCH_BLOBS,MALLOC_SOFT_LIMIT=1024,MAX_ATTACHED=10,MAX_COLUMN=2000,
MAX_COMPOUND_SELECT=500,MAX_DEFAULT_PAGE_SIZE=8192,MAX_EXPR_DEPTH=1000,MAX_FUNCTION_ARG=1000,
MAX_LENGTH=1000000000,MAX_LIKE_PATTERN_LENGTH=50000,MAX_MMAP_SIZE=0x7fff0000,
MAX_PAGE_COUNT=0xfffffffe,MAX_PAGE_SIZE=65536,MAX_SQL_LENGTH=1000000000,
MAX_TRIGGER_DEPTH=1000,MAX_VARIABLE_NUMBER=32766,MAX_VDBE_OP=250000000,
MAX_WORKER_THREADS=8,MUTEX_PTHREADS,SOUNDEX,SYSTEM_MALLOC,TEMP_STORE=1,THREADSAFE=1
```

Its LF-terminated exact-set SHA-256 was
`8b9138f0970b0a9548b57112d02cecf88d573574977d4d0dbc106c4d8cdb7ac0`. It included
`MUTEX_PTHREADS` and excluded `MUTEX_NOOP` and `OMIT_SEH`.

All primary-row URIs used the exact canonical root above, the database suffix in the table below,
and the exact query
`?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix`. Therefore, for
example, row 10 used:

```text
file:///tmp/dockpipe-sqlite-linux-failure-0a13OJYL/TestLinuxNativeSQLiteFailureBoundaryMatrix1728090748/001/10_commit_call_loss-01/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

The independent different-session control in row 3 used:

```text
file:///tmp/dockpipe-sqlite-linux-failure-0a13OJYL/TestLinuxNativeSQLiteFailureBoundaryMatrix1728090748/001/03_contention-01/other/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

The complete 22-attempt result table follows. Each row began with the same platform-independent
session, payload, envelope, revision, and SHA-256 as the protected Windows matrix. The metadata
column records the exact per-scenario pre/post SHA-256 values; unequal values in the three
forced-termination rows reflect journal recovery metadata transitions rather than content access.

| Scenario | Attempt and boundary | Evidence / classification | Initial revision / SHA-256 | Final revision / SHA-256 | Database suffix | Pre / post metadata SHA-256 |
| --- | --- | --- | --- | --- | --- | --- |
| `01_before_open` | 1; before open | Harness / rejected | 1 / `d11618d4938743fbe7582c37b3ec38f5e480f1a0e7b4ca97a19f46b214abb689` | 1 / `d11618d4938743fbe7582c37b3ec38f5e480f1a0e7b4ca97a19f46b214abb689` | `01_before_open-01/main/aggregate.sqlite` | `f5a51e99811dc9807dbe0c708c63a82c92300f7d07c930c401e922a12ebe072b` / `f5a51e99811dc9807dbe0c708c63a82c92300f7d07c930c401e922a12ebe072b` |
| `02_contract_reject` | 1; substituted contract evidence | Harness / rejected | 1 / `32e409176da2a0c3bd010747fbd9cce2ac41c6c8b3be4c002989692b68190a07` | 1 / `32e409176da2a0c3bd010747fbd9cce2ac41c6c8b3be4c002989692b68190a07` | `02_contract_reject-01/main/aggregate.sqlite` | `0de287cb4be9d98f1c24218becce0515c720c681d71b2dfde02edc516731cd91` / `0de287cb4be9d98f1c24218becce0515c720c681d71b2dfde02edc516731cd91` |
| `03_contention` | 1; same-session lock before observation | Native `SQLITE_BUSY`/`LOCKED` / rejected | 1 / `1fca77d81f99a9dca417372de397a9fa801da364e3e140526cee574884577a4a` | 1 / `1fca77d81f99a9dca417372de397a9fa801da364e3e140526cee574884577a4a` | `03_contention-01/main/aggregate.sqlite` | `aee2494bdb08f713391250219f4ac30a125cabca5d48af00513d88fdd620ae92` / `aee2494bdb08f713391250219f4ac30a125cabca5d48af00513d88fdd620ae92` |
| `04_cancel_after_observation` | 1; cancellation after observation | Harness plus rollback/reload / rejected | 1 / `a6db7755613e0a36301f372c966dbceef77da2e058634ba5fbbaf26cec93ea94` | 1 / `a6db7755613e0a36301f372c966dbceef77da2e058634ba5fbbaf26cec93ea94` | `04_cancel_after_observation-01/main/aggregate.sqlite` | `c5d8d893a9768f9316af0757589fdb6f66d56cad258585db02fa91de6284508e` / `c5d8d893a9768f9316af0757589fdb6f66d56cad258585db02fa91de6284508e` |
| `05_stale_cas` | 1; stale session | Native zero rows plus rollback/reload / known unchanged | 1 / `6cf53d46e2f230531e71a9b9e038dfa69a836adcc2086a801d496ab6202508fb` | 1 / `6cf53d46e2f230531e71a9b9e038dfa69a836adcc2086a801d496ab6202508fb` | `05_stale_cas-01/main/aggregate.sqlite` | `5b528d57cdad0efd4f7e195a29bdb996bc865929bbdc0aecf1141e32d9519ea7` / `5b528d57cdad0efd4f7e195a29bdb996bc865929bbdc0aecf1141e32d9519ea7` |
| `05_stale_cas` | 2; stale revision | Native zero rows plus rollback/reload / known unchanged | 1 / `d4fa724c2c7feb3812383febed9c94b5dcb300f6c4f5d9f0864c969bc87ebdbf` | 1 / `d4fa724c2c7feb3812383febed9c94b5dcb300f6c4f5d9f0864c969bc87ebdbf` | `05_stale_cas-02/main/aggregate.sqlite` | `5c1c9a053d80e6535b7ad5817cf9d65fc97b06533bf295213753ec81fb92e4a3` / `5c1c9a053d80e6535b7ad5817cf9d65fc97b06533bf295213753ec81fb92e4a3` |
| `05_stale_cas` | 3; stale digest | Native zero rows plus rollback/reload / known unchanged | 1 / `e324953707b27c5af0598cab81f0764c727cd7b445b1d137ecc35aae1a7c0ea3` | 1 / `e324953707b27c5af0598cab81f0764c727cd7b445b1d137ecc35aae1a7c0ea3` | `05_stale_cas-03/main/aggregate.sqlite` | `6b5f051cf6d136c5f1f1c1a225884cebbdbf10f81f7027a1bc723ebf291ce082` / `6b5f051cf6d136c5f1f1c1a225884cebbdbf10f81f7027a1bc723ebf291ce082` |
| `06_after_begin` | 1; after begin before CAS | Harness plus rollback/reload / known unchanged | 1 / `faaa05f23a2c00cf7301f3fee428254ec19f2eb72419c2a012882f8960af16bf` | 1 / `faaa05f23a2c00cf7301f3fee428254ec19f2eb72419c2a012882f8960af16bf` | `06_after_begin-01/main/aggregate.sqlite` | `2211a93e5a8aca0e237f71c59bcac7295aa772df38c22ae3418f7a0fddf621bc` / `2211a93e5a8aca0e237f71c59bcac7295aa772df38c22ae3418f7a0fddf621bc` |
| `07_after_stage` | 1; after exact staging before commit | Harness plus rollback/reload / known unchanged | 1 / `91f371614445768866b3e0fb9a32890ded0f4d6e15d2296054ef295e0de0c31f` | 1 / `91f371614445768866b3e0fb9a32890ded0f4d6e15d2296054ef295e0de0c31f` | `07_after_stage-01/main/aggregate.sqlite` | `0d1aa4d5fe96f1291e78801fad759e941b29bd743790194713d25c9e79220c45` / `0d1aa4d5fe96f1291e78801fad759e941b29bd743790194713d25c9e79220c45` |
| `08_terminate_precommit` | 1; termination after staging before commit | Native termination/recovery / known unchanged | 1 / `b721a5187f4f556565cfd7644037321dba1360f7611e9182d1767aa03578a05a` | 1 / `b721a5187f4f556565cfd7644037321dba1360f7611e9182d1767aa03578a05a` | `08_terminate_precommit-01/main/aggregate.sqlite` | `50b89159e039bd1e2d3c2312120a870db2fcdcef49a2cffc353cbf912ec3bd5a` / `0a533a3dd66cffb5e8c37a937372f95c8d18cc111464f71dc197b4abd38d1e49` |
| `09_rollback_proof_loss` | 1; rollback/result proof lost | Harness loss plus native recovery / `unknown_commit_result` | 1 / `0b0c1ce8d1e351f1f95ab480f8112e9c89361e30550308f4e04f598e8c0fdd46` | 1 / `0b0c1ce8d1e351f1f95ab480f8112e9c89361e30550308f4e04f598e8c0fdd46` | `09_rollback_proof_loss-01/main/aggregate.sqlite` | `d37d200ce3ec270343211fb9050d2798a8f22f957040fc42202beadfbd4864f4` / `2a72d28e0fe942ae9ece65b644034d6fef953f01391b50a7bf90defe87ccbdac` |
| `10_commit_call_loss` | 1; termination inside commit hook before phase one/result | Native commit-hook checkpoint plus termination / `unknown_commit_result` | 1 / `43bef391b42f6b51b4c67517efe00e7c97dda0eabca6ed52975980468ed0923f` | 1 / `43bef391b42f6b51b4c67517efe00e7c97dda0eabca6ed52975980468ed0923f` | `10_commit_call_loss-01/main/aggregate.sqlite` | `b8bf8e7810404eaae220b0718542ee3d4fb8c31e72feff7c8bd2d916fc2d367c` / `0cb377a6dab30e6815508806f9c091030eec613b26f86c09992ba3d4f2fae5bd` |
| `11_genuine_commit_error` | 1; genuine error attempt | Proven unreachable; control commit / committed | 1 / `538c917b0fc4360a9f1337b5f04a3f0baf9e7436adcc21e1f6374e679587216e` | 2 / `7c0dd65cc2a2850d7fb6dfde8e3bd9142cf26503b74ddcc575fc9c170588e4d8` | `11_genuine_commit_error-01/main/aggregate.sqlite` | `b055c0ed4ddb1432ae08d2200740b7f8bf5ca17134fa03bb09c235df5c0843ff` / `b055c0ed4ddb1432ae08d2200740b7f8bf5ca17134fa03bb09c235df5c0843ff` |
| `12_response_loss` | 1; success then response loss before reload | Harness / `unknown_commit_result` | 1 / `32097cd580bc4bb23bbdb3a84dc6ea953d9233840965514258386a3bf66e5410` | 2 / `fa586651472644b528aff5c042e98cc78284b0508048e4d63d124de173ff895d` | `12_response_loss-01/main/aggregate.sqlite` | `2b994844ee2c4802b8171db997748ff19b33328a1b0163322a0f241793a1c044` / `2b994844ee2c4802b8171db997748ff19b33328a1b0163322a0f241793a1c044` |
| `13_validation_loss` | 1; schema validation result lost | Harness / `unknown_commit_result` | 1 / `604abe29c761b3c1f6fd4d303e2d59e2b97414dac714d3d361a76d28124af792` | 2 / `53f1d118dc76593ec110233467c23e292fe769bf480317957cfc908be043a214` | `13_validation_loss-01/main/aggregate.sqlite` | `ee81208c81f73046d21c7f2337c10a59bb0794dcb867e549a95de4b7b5461bb6` / `ee81208c81f73046d21c7f2337c10a59bb0794dcb867e549a95de4b7b5461bb6` |
| `13_validation_loss` | 2; identity validation result lost | Harness / `unknown_commit_result` | 1 / `95f5a71c86ed8215865cf6880460dc1b954aebfde94d882b4378db91f7379eed` | 2 / `37ba916c7c427488bb0482f5e9490a176a1ec91157a74028748df1c5240fc7e5` | `13_validation_loss-02/main/aggregate.sqlite` | `7ddef3904e5d6dc069cf67261bf9ad995cb12b8c558765f88529481772f1ad2c` / `7ddef3904e5d6dc069cf67261bf9ad995cb12b8c558765f88529481772f1ad2c` |
| `13_validation_loss` | 3; digest/envelope result lost | Harness / `unknown_commit_result` | 1 / `e2351ed2ad87340ac3671ad728083354787307b52e0b021f441ea31f506f2972` | 2 / `0790b0985f1a1d5782a9dc46cca025a204a952c6a5352c0e5147dadccaf57da3` | `13_validation_loss-03/main/aggregate.sqlite` | `e7eb52752ced92f09658dbdc45fdde061186005e6696d13e19ef8719b3c01bb2` / `e7eb52752ced92f09658dbdc45fdde061186005e6696d13e19ef8719b3c01bb2` |
| `13_validation_loss` | 4; sibling validation result lost | Harness / `unknown_commit_result` | 1 / `b9fbd37af1ec09c17bade131abbadd653640c9ce8c4e41ef1f1d457cd89cf9a9` | 2 / `73f98f28c9ba21837cfe1045e7c3cc165a6100030f44c808e6ac04dc777df357` | `13_validation_loss-04/main/aggregate.sqlite` | `b103d13123e1f41e104fc4e4ef23d7e63e861ac49be4bc26bcba23959d9adf19` / `b103d13123e1f41e104fc4e4ef23d7e63e861ac49be4bc26bcba23959d9adf19` |
| `13_validation_loss` | 5; Linux ownership/mode/mount/path validation result lost | Harness / `unknown_commit_result` | 1 / `0ede316defffda98a5b2751ade8a26607433a6721809e5f0e2223694ebb9ec9e` | 2 / `39651e0ab7acd41405c527582dc669fae31372148a959e6b1149a379b4669781` | `13_validation_loss-05/main/aggregate.sqlite` | `ca2c341ea0f3ce0263e1eb5da6ac1a2df4a9d2617d88ec402678133c3bbf6bbd` / `ca2c341ea0f3ce0263e1eb5da6ac1a2df4a9d2617d88ec402678133c3bbf6bbd` |
| `14_close_result_loss` | 1; close result lost | Harness / `unknown_commit_result` | 1 / `308fa6348158fc37f3d4f8e639f63666065850bda7d2c7dd306637e70b71a936` | 2 / `d4ad3b44fc10c58caeed27efbee783e3a7bef2188e88b1e793f17d56eb43f0ae` | `14_close_result_loss-01/main/aggregate.sqlite` | `24e51599db81bcb21a25b3a1d1bf831c34804ee1abe7da982aec96a8192f0f4c` / `24e51599db81bcb21a25b3a1d1bf831c34804ee1abe7da982aec96a8192f0f4c` |
| `15_ack_loss` | 1; acknowledgement lost | Harness / `unknown_commit_result` | 1 / `09d2cabd2f6b285735e8b9206d463c89036333c547a08134d7417f5f31e42877` | 2 / `8711029d8629c4ac052c7a963f7f0d15e99f39a7184eab6a772e67e1febf9c9d` | `15_ack_loss-01/main/aggregate.sqlite` | `7e0e223ecd5bc5d4575db55cfc50ff0d5d566e1e877156d841a10844dd9826ae` / `7e0e223ecd5bc5d4575db55cfc50ff0d5d566e1e877156d841a10844dd9826ae` |
| `16_success` | 1; full validated path | Native / committed | 1 / `2a10f300002f05f312429cfc3c9ee12629fb6c127927be866a23096b15640717` | 2 / `792629f5e19b1b26eb3ca65bb91f19b0f122f2f0b9485236b90ae0615b1f5927` | `16_success-01/main/aggregate.sqlite` | `3e0f13b21495a35f684eea2a1ba4d8bd31009eeff376140dfa2036f1f2927b1b` / `3e0f13b21495a35f684eea2a1ba4d8bd31009eeff376140dfa2036f1f2927b1b` |

The exact aggregate counters were: rows attempted `22`; rows proven natively `7`;
harness-injected application-boundary rows `14`; rows proven unreachable `1`; rows still
unproven `0`; known unchanged `6`; committed `2`; rejected `4`;
`unknown_commit_result` `10`; recovery-only opens `22`; exact old/new recoveries `12` /
`10`; genuine `BUSY/LOCKED` before observation `1`, after observation `0`, and at commit
`0`; different-session commits `1`; rollback attempts/exact-old proofs `6` / `6`; forced
terminations `4`; commit invocations/return observations `12` / `11`; success
acknowledgements `2`. Duplicate commits, retries, replays, repairs, fallbacks, partial rows,
ambiguous pre-commit recoveries, staged-row leaks, revision gaps/duplicates, digest/envelope
mismatches, unexpected siblings/protection widening, and protocol loss/duplication/substitution/
reordering were all `0`.

Every command and response retained exact scenario, cycle, attempt, operation, checkpoint,
database, session, root, and retained-root identity. Strict bounded JSON-line decoding rejected
missing, unknown, duplicate, malformed, multiple-value, substituted, cross-scenario, and
out-of-order data. Children performed ordinary SQLite operations only. No child cleaned, deleted,
renamed, truncated, copied, moved, parsed, or hashed a physical database or journal file. Journal
validation was metadata-only; only the parent test framework removed fixtures after all children and
connections closed.

The canonical-root metadata-tree SHA-256 changed from
`97db40d6fe59408eb182465033f6eed119c76228eb677d0735173f4e058ec9d9` before scenario
creation to `44edd13e72f4edb1be4307e362e74a4e69f776c9af990f77d14df69e92a3798d`
after the matrix. The exact scenario pre/post rollups were
`f6482ec043d37551f246bc6938f350a45ddc0871ef5f2bc6b1ec62e38bd8f96e` and
`2cb51ffafd93743a833d33daad8247b56c859b09660a967acba5843b3cf65b47`.
Each hash covered LF-terminated ordinally sorted rows containing relative path, entry type, byte
size, mode, UID/GID, device major/minor, inode, mount ID, filesystem type/magic, source, and mount
point. Rollups bound scenario ID, attempt, and hash.

**Exact Linux commit call-chain qualification — 2026-08-05.** The reviewed standard-library source
was Go `go1.25.0` under
`/home/jamie/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64`, with
`database/sql/sql.go:2287-2319` and `database/sql/driver/driver.go:519-522`. The reviewed
pinned module source was `modernc.org/sqlite v1.56.0` with `modernc.org/libc v1.74.4` under
`/home/jamie/go/pkg/mod`. The exact path was:

```text
(*database/sql.Tx).Commit
  -> driver.Tx.Commit under database/sql's driverConn lock
  -> (*modernc.org/sqlite.tx).Commit
  -> (*sqlite.tx).exec(context.Background(), "commit")
  -> sqlite3.Xsqlite3_exec
  -> sqlite3.Xsqlite3_prepare_v2
  -> sqlite3.Xsqlite3_step
  -> _sqlite3Step
  -> _sqlite3VdbeExec
  -> _sqlite3VdbeHalt
  -> _vdbeCommit
  -> detect the live write transaction
  -> _sqlite3PagerExclusiveLock
  -> FxCommitCallback / modernc commitHookTrampoline / test callback
  -> _sqlite3BtreeCommitPhaseOne
  -> _sqlite3BtreeCommitPhaseTwo
  -> return through sqlite3_exec, modernc tx.Commit, and database/sql Tx.Commit
```

The driver locations were `tx.go:35-78`, `sqlite.go:618-622`, and
`pre_update_hook.go:53-67,205-215`. The generated Linux/amd64 locations were
`lib/sqlite_linux_amd64.go:82-210` for `Xsqlite3_exec`,
`lib/sqlite.go:11963-12029` for `Xsqlite3_step`,
`lib/sqlite_g_000000000001deab.go:3898-4017,4075-4293` for `_sqlite3Step` and VDBE halt,
`lib/sqlite_g_0000000000003a80.go:18923-19204` for `_vdbeCommit`, its live-write test,
exclusive-lock acquisition, commit callback, and phase calls, and
`lib/sqlite_g_0000000000060000.go:104277-104369` for the two B-tree commit phases.

For row 10 the public `sqlite.HookRegisterer.RegisterCommitHook` callback emitted the strict
`sqlite_commit_hook_entered` checkpoint from inside SQLite after the live write-transaction and
exclusive-lock checks and before phase one or result availability, then blocked without returning.
Only after the parent validated the full checkpoint did it terminate the child. Fresh recovery
returned the exact old row. No timer, sleep, goroutine-start marker, parent kill race, wrapper-entry
marker, debugger, patched dependency, replacement driver, custom VFS, or filesystem/journal
observation substituted for this boundary.

Row 11 remains genuinely unreachable under the selected exclusive shape without changing the
driver, SQLite, filesystem, or protected code: another same-session owner is rejected at lock
acquisition before observation and cannot retain a conflicting lock at commit. Its successful
control commit was genuine; no error was fabricated. Therefore rows still unproven is zero, but the
matrix is not described as closed by pretending the unreachable row executed.

The required unchanged Linux regression cohorts then passed in order:

```text
CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -count=1
PASS; ok in 0.002s
TMPDIR=/tmp/dockpipe-sqlite-linux-failure-0a13OJYL DORKPIPE_SQLITE_LINUX_CONTENTION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteContentionCohort$' -count=1 -v -timeout=30m
PASS; evidence 39.607s; package 39.633s; all seven counters 1000; primary code 5 = 1000; code 6 = 0
TMPDIR=/tmp/dockpipe-sqlite-linux-failure-0a13OJYL DORKPIPE_SQLITE_LINUX_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
PASS; evidence 1m22.493s; package 82.516s; old/BUSY/new/journal counts 10000 each
TMPDIR=/tmp/dockpipe-sqlite-linux-failure-0a13OJYL DORKPIPE_SQLITE_LINUX_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteSmoke$' -count=1 -v -timeout=10m
PASS; evidence 46ms; package 0.047s; genuine primary SQLITE_BUSY (5); recovery quick_check=ok
```

Contention retained exact same-session final revision/digest `1001` /
`e024c4e5dafc3841e26abbc2df7618f2fd78fcabd3b41bd364485b2ad56ff693` and
different-session final revision/digest `1001` /
`5c78a969b9d47e07ef749d58c6b0fa3311512435141191d79376dd50e2f62f26`.
Its fresh-fixture pre/post metadata hash was equal:
`e19110e6836098f82ffd74fedb6b0ee30b9b7cc771f1aa3305ac4e2423a76c76`.
Publication retained final revision/digest `10001` /
`3304b9ccdfd01f7c211e8e4530be8b533c6b2c506975b83ebceb33f6288eb838` and
equal fresh-fixture pre/post metadata hash
`ccaaef3dc1a4eab9ab808bd5ec040fcdedbde14ab4202ad540aee9fb9f362e90`.

The complete test package then cross-compiled with `CGO_ENABLED=0` into the separate newly
verified private directory `/tmp/dockpipe-sqlite-linux-failure-cross-ZeqVuuAu`, outside the
repository with mode `0700` and owner `1000:1000`. `go version -m` confirmed exact embedded
settings for Windows/`amd64`, Linux/`amd64`, and macOS (`darwin`)/`arm64`. The binaries
were respectively `12,087,808`, `11,861,347`, and `10,953,058` bytes. The caller removed
those three exact binaries, then their empty parent. It also removed the now-empty native-fixture
parent `/tmp/dockpipe-sqlite-linux-failure-0a13OJYL`. Neither directory, nor any fixture, test
child, binary, or evidence artifact remained. Cross-compilation is compatibility evidence only; it
is not native Windows or macOS evidence.

The protected Windows matrix remains unchanged and independently qualified with 57 options,
`COMPILER=gcc-12-win32`, `MUTEX_NOOP`, and `OMIT_SEH`. Linux remains independently
qualified with 56 options, `COMPILER=gcc-12.2.0`, and `MUTEX_PTHREADS`. Linux smoke,
publication, and contention remain separate qualifications. This matrix does not qualify Linux
reboot/power-loss durability, production storage, migration, cutover, recovery authority,
dispatch/projection integration, Slice 2, or macOS. Linux reboot/power-loss remains open and macOS
remains intentionally last. TASK-013 and CAS-14 remain open.


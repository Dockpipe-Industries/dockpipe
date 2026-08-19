**Linux/amd64 native SQLite smoke qualification — 2026-08-04.** The Linux-only opt-in
`TestLinuxNativeSQLiteSmoke` passed natively with `CGO_ENABLED=0` on Pop!_OS 22.04 LTS,
Linux `7.0.11-76070011-generic`, kernel build
`#202606011647~1780583630~22.04~70ad774 SMP PREEMPT_DYNAMIC Thu J`, bare metal according to
`systemd-detect-virt`, `amd64`, and Go `go1.25.0`. The pinned module graph remained unchanged:
`golang.org/x/sys v0.47.0`, `modernc.org/libc v1.74.4`, and `modernc.org/sqlite v1.56.0`; the
`go.mod` / `go.sum` SHA-256 values were respectively
`f59ee93b1feb390705c790649a6ac36de360053aa5260818885c78df19881d19` and
`b426dc8754abc50973fbae78d32642746de09cc6c6b6485a24727572cbf610a9`.

The parent created one new private temporary parent
`/tmp/dockpipe-sqlite-linux-fYSJ5FKK` outside the repository, set and revalidated it as a current-user
owned `0700` directory, and used it as `TMPDIR`. The successful test-framework fixture root was
`/tmp/dockpipe-sqlite-linux-fYSJ5FKK/TestLinuxNativeSQLiteSmoke3890145937/001`, also current-user
owned and `0700`. `statx(STATX_MNT_ID)`, a metadata-only `O_PATH|O_NOFOLLOW` handle plus `fstatfs`,
and `/proc/self/mountinfo` agreed on mount ID `33`, device `259:7`, ext4 magic `0xef53`, source
`/dev/nvme0n1p3`, mount root/point `/` / `/`, options `rw,noatime`, and super-options
`rw,errors=remount-ro,stripe=64`. The exact mountinfo row was:

```text
33 2 259:7 / / rw,noatime shared:1 - ext4 /dev/nvme0n1p3 rw,errors=remount-ro,stripe=64
```

The source block device and its parent were non-removable. The lane rejected bind, nested, overlay,
FUSE, network, removable, shared-host, `drvfs`, `9p`, `tmpfs`, symlinked/substituted, and cross-mount
storage. Every fixture/session directory was an owned regular `0700` directory; every database and
observed journal was an owned regular `0600` file on the same exact mount/device. Only the selected
`aggregate.sqlite` and `aggregate.sqlite-journal` siblings were admitted. The metadata checks never
opened a journal for content and never parsed, copied, moved, truncated, deleted, or hashed one.

The queried engine was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`
and native `unix` VFS. The exact main-database URI was:

```text
file:///tmp/dockpipe-sqlite-linux-fYSJ5FKK/TestLinuxNativeSQLiteSmoke3890145937/001/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

The lane applied and read back `journal_mode=delete`, `synchronous=3` (`EXTRA`), `fullfsync=1`,
`temp_store=2` (`MEMORY`), `mmap_size=0`, `busy_timeout=0`, `foreign_keys=1`,
`trusted_schema=0`, `cell_size_check=1`, `locking_mode=exclusive`, and pre-schema
`page_size=4096`. It also rejected unresolved double-quoted SQL, required only `main` in
`PRAGMA database_list`, created exactly the selected singleton STRICT
`app_server_aggregate` table plus `user_version=1`, and used only the selected insert and exact
session/revision/digest conditional-update shape. Every staged and committed row reloaded with exact
canonical payload, envelope, and SHA-256 equality; revisions were strictly monotonic.

Linux exposed this exact sorted 56-entry compile-option set:

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

This is an exact native platform allowlist, not a weakened count-only or subset check: count,
ordering, and every entry fail closed. Windows independently retains its existing exact 57-entry
set and evidence contract. Linux uses `MUTEX_PTHREADS`; Windows uses `MUTEX_NOOP` and additionally
has the Windows-only `OMIT_SEH` entry. Their separately pinned compiler identity entries also remain
platform-specific (`gcc-12.2.0` on Linux and `gcc-12-win32` on Windows). No Windows cohort or
protected Windows evidence contract changed.

The revision-1 insert digest was
`5bacd33f5355f1a64a096841fe3fceeca28a40f211723e2ce4bb9b56988e6fe8`; the exact revision-2 CAS
digest was `37572e06825751539b2e65c19034a23950925abbbe795d296a52ecf1e6e2aca4`.
An independent owner process staged revision 3 and retained the live rollback journal. A fresh
same-session contender returned genuine primary `SQLITE_BUSY` (`5`), while a different-session
database remained independently writable and passed its own `quick_check`. Forced owner
termination occurred before any commit. A fresh recovery child returned `quick_check=ok` and the
exact old revision-2 row. One fresh parent-held dedicated connection then committed revision 3
exactly once, reloaded and integrity-checked it, and produced final digest
`557edb00816e95dbc84b0bba0f347cdaf6087fc49a6661534de903646cd3ec66`.
There were zero retries, replays, repairs, fallbacks, inferred acknowledgements, staged-row leaks,
revision gaps/duplicates, or ambiguous recoveries. Journal metadata was observed at 4,616 bytes for
the initial commits and 8,720 bytes while the clean revision-3 connection remained open.

The pre-contention and post-clean-commit metadata-tree SHA-256 values were respectively
`0a388a03d9be383266d97f96a101f39b28e94b87f00c52f52cf6700f3ae13dc2` and
`6fcf39acb8a48a782087cccba2f97958578d6077633c3fdf4618f96cfe627bc2`.
Each hash covered LF-terminated ordinally sorted rows containing relative path, entry type, size,
mode, owner, device, inode, mount ID, filesystem type/magic, source, and mount point. Evidence elapsed
time was `50ms`; the package result was `ok` in `0.052s`. Only the parent test framework cleaned the
fixture. After the pass, the caller removed the exact now-empty temporary parent and verified it no
longer existed. No child process, fixture, binary, or evidence artifact remained.

The exact successful native command, run from `packages/dorkpipe/lib`, was:

```text
TMPDIR=/tmp/dockpipe-sqlite-linux-fYSJ5FKK DORKPIPE_SQLITE_LINUX_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteSmoke$' -count=1 -v -timeout=10m
```

A preceding sandboxed setup attempt never started the test because the existing Go build cache was
read-only in that sandbox; it supplied no native evidence. The successful command above ran with the
required host access and revalidated a newly created fixture path rather than reusing the removed
attempt fixture. This Linux smoke qualification changes no dependency, production storage, Slice 2
surface, publication/contention/failure cohort, migration, cutover, lifecycle, or support decision.
The shared smoke wrapper has now passed both the native Linux qualification above and the final
native Windows rerun below.

**Final Windows/amd64 shared-wrapper rerun — 2026-08-04.** The Windows-only opt-in
`TestWindowsNativeSQLiteSmoke` passed natively with `CGO_ENABLED=0` on Windows build `10.0.26200`,
`amd64`, and Go `go1.26.4`. The fixture used qualifying fixed local NTFS storage on volume
`\\?\Volume{2eb284d8-09e6-483c-b096-6deed2208642}\` with serial `88c9a133` and label `OS`; the
unprivileged NTFS-version query remained unavailable. The fixture root, both database files, and
observed journals were owned by current-user SID
`S-1-5-21-2729925100-2499202611-1015899381-1002`, admitted only that SID and `SYSTEM`
(`S-1-5-18`) as trustees, and granted them full access.

The queried engine was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`
and native `win32` VFS. The exact selected main-database URI was:

```text
file:///C:/Users/Jamie/AppData/Local/Temp/TestWindowsNativeSQLiteSmoke827599481/001/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=win32
```

The lane read back the selected `journal_mode=delete`, `synchronous=3` (`EXTRA`), `fullfsync=1`,
`temp_store=2` (`MEMORY`), `mmap_size=0`, `busy_timeout=0`, `foreign_keys=1`,
`trusted_schema=0`, `cell_size_check=1`, `locking_mode=exclusive`, and pre-schema
`page_size=4096` pragmas. It required only `main`, the exact singleton STRICT
`app_server_aggregate` schema with `user_version=1`, and the selected absolute URI. Windows retained
its exact sorted 57-option allowlist, including `COMPILER=gcc-12-win32`, `MUTEX_NOOP`, and
`OMIT_SEH`, with no `MUTEX_PTHREADS`. Linux remains exactly 56 options with
`COMPILER=gcc-12.2.0` and `MUTEX_PTHREADS`, without the two Windows-only mutex/SEH entries.

The exact revision-1 insert payload and digest reloaded equal at
`5bacd33f5355f1a64a096841fe3fceeca28a40f211723e2ce4bb9b56988e6fe8`; the exact revision-2 CAS
payload and digest reloaded equal at
`37572e06825751539b2e65c19034a23950925abbbe795d296a52ecf1e6e2aca4`. An independent owner staged
revision 3 and held the same database and its protected rollback journal. A fresh contender returned
genuine primary `SQLITE_BUSY` (`5`), while the different-session database remained independently
writable. The parent forcibly terminated the owner before commit; a fresh recovery child returned
`quick_check=ok` and the exact old revision-2 row. There were zero retries, replays, repairs,
fallbacks, or inferred acknowledgements. The observed journals remained protected siblings, were
4,616 bytes after commit, and were never opened or hashed for content. Cleanup remained
parent-test-only, and the test left no fixture, evidence artifact, binary, or child process.

The exact successful PowerShell command, run from `packages/dorkpipe/lib`, was:

```powershell
$env:DORKPIPE_SQLITE_EVIDENCE = "1"
$env:CGO_ENABLED = "0"
go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteSmoke$' -count=1 -v -timeout=10m
```

The evidence lane elapsed time was `1.429s`; the package result was `ok` in `3.929s`. This completes
the shared-wrapper native Linux and Windows qualification only. It does not qualify the deferred
publication, contention, or failure cohorts on Linux or other platforms, power-loss evidence,
production storage, migration, Slice 2, or macOS.

The remaining required validation produced these exact results from `packages/dorkpipe/lib`:

```text
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
PASS; embedded CGO_ENABLED=0, GOOS=windows, GOARCH=amd64; size 12,087,808 bytes
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
PASS; embedded CGO_ENABLED=0, GOOS=linux, GOARCH=amd64; size 11,137,781 bytes
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
PASS; embedded CGO_ENABLED=0, GOOS=darwin, GOARCH=arm64; size 10,953,058 bytes
go test -mod=readonly ./appserversupervisor ./providersession -count=1
PASS; appserversupervisor 0.712s; providersession 0.002s
go test -mod=readonly ./... -count=1 -timeout=90s
EXPECTED PROTECTED FAILURES; sqliteevidence passed; cmd/dorkpipe failed its existing Windows-style path-normalization candidate assertion; orchestrationhelper timed out after 90s
go mod verify
PASS; all modules verified
gofmt -d appserversupervisor/sqliteevidence/host_other_test.go appserversupervisor/sqliteevidence/sqlite_smoke_test.go appserversupervisor/sqliteevidence/host_linux_test.go appserversupervisor/sqliteevidence/linux_smoke_test.go
PASS; empty output
git diff --check
PASS
```

All three cross-target binaries were written under the one revalidated private ext4 directory
`/tmp/dockpipe-sqlite-cross-0fVZ2Rme`, inspected with `go version -m`, then removed with that exact
directory. The full suite's protected `cmd/dorkpipe` failure was
`TestProviderPoolWorkdirHashCandidatesIncludeWindowsStyleNormalizations`: the candidate list retained
the original and lowercase Windows paths plus Linux-working-directory-prefixed forms, but lacked the
expected normalized variants. The exact protected timeout was
`TestSoftwareDevPromotionPatchGenerationAndApprovedApply`; at the timeout it was inside the
pre-existing bundled-cache fingerprint/extraction path reached through
`ValidateResolvedWorkflowYAML`, promotion patch compilation, and approved apply. This moved from the
Windows baseline timeout in
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsMalformedDecisionFixtures/malformed`,
which was blocked in its pre-existing `os.ReadFile` fixture path. Neither protected failure is in an
authorized path for this Linux qualification, and `appserversupervisor/sqliteevidence` passed in the
same full-suite run.

**Linux/amd64 10,000-cycle native reader-publication cohort — 2026-08-04.** The Linux-only opt-in
`TestLinuxNativeSQLitePublicationCohort`, gated by
`DORKPIPE_SQLITE_LINUX_PUBLICATION_COHORT=1`, passed natively with `CGO_ENABLED=0`,
`-mod=readonly`, `-count=1`, verbose output, and the fixed 30-minute timeout. It ran on Pop!_OS
22.04 LTS, Linux `7.0.11-76070011-generic`, kernel build
`#202606011647~1780583630~22.04~70ad774 SMP PREEMPT_DYNAMIC Thu J`, bare metal according to
`systemd-detect-virt`, `amd64`, and Go `go1.25.0`.

The caller created the new private parent
`/tmp/dockpipe-sqlite-linux-publication-vqoEPdfl` outside the repository with mode `0700`, owner
UID/GID `1000:1000`, device `259:7`, and inode `57176041`, and used it as `TMPDIR`. The test-owned
fixture root was
`/tmp/dockpipe-sqlite-linux-publication-vqoEPdfl/TestLinuxNativeSQLitePublicationCohort2406397794/001`,
also owned `1000:1000` with mode `0700`. Its retained root identity was mount ID `33`, device
`259:7`, inode `57176196`, and kind `directory`. Metadata-only `statx`, `O_PATH|O_NOFOLLOW` plus
`fstatfs`, and `/proc/self/mountinfo` agreed on ext4 magic `0xef53`, source `/dev/nvme0n1p3`, mount
root/point `/` / `/`, mount options `rw,noatime`, and super-options
`rw,errors=remount-ro,stripe=64`. The exact mountinfo row was:

```text
33 2 259:7 / / rw,noatime shared:1 - ext4 /dev/nvme0n1p3 rw,errors=remount-ro,stripe=64
```

The source block device and parent were non-removable. The lane rejected bind, nested, overlay,
FUSE, network, removable, shared-host, `drvfs`, `9p`, `tmpfs`, symlinked/substituted, and cross-mount
storage. Every fixture/session directory remained an owned `0700` directory; the main database and
every observed rollback journal remained owned regular `0600` files on the exact qualified
mount/device. Only `aggregate.sqlite` and `aggregate.sqlite-journal` were admitted.

The queried engine was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`
and native `unix` VFS. The exact selected URI was:

```text
file:///tmp/dockpipe-sqlite-linux-publication-vqoEPdfl/TestLinuxNativeSQLitePublicationCohort2406397794/001/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

Every connection applied and read back `journal_mode=delete`, `synchronous=3` (`EXTRA`),
`fullfsync=1`, `temp_store=2` (`MEMORY`), `mmap_size=0`, `busy_timeout=0`, `foreign_keys=1`,
`trusted_schema=0`, `cell_size_check=1`, `locking_mode=exclusive`, and pre-schema `page_size=4096`.
The lane rejected unresolved double-quoted SQL, required only `main`, and retained exactly the
singleton STRICT `app_server_aggregate` schema with `user_version=1`. The initial connection
fail-closed validated the exact sorted 56-option Linux allowlist recorded in the preceding Linux
smoke qualification: it includes `COMPILER=gcc-12.2.0` and `MUTEX_PTHREADS`, with no `MUTEX_NOOP`
or `OMIT_SEH`. Windows independently remains exactly 57 options with `COMPILER=gcc-12-win32`,
`MUTEX_NOOP`, and `OMIT_SEH`, and without `MUTEX_PTHREADS`.

One persistent writer child and one persistent reader child used a bounded strict JSON-line
protocol. Every command and response retained its exact cycle number and operation. Missing,
duplicate, malformed, substituted, unknown-field, multiple-value, or out-of-order protocol data
failed closed. For every cycle, the reader returned the exact old row, the writer staged exactly the
next revision and held the protected live rollback journal, a fresh same-database reader connection
returned only genuine primary `SQLITE_BUSY` (`5`) or `SQLITE_LOCKED` (`6`), the writer committed
exactly once, and the reader then returned the exact new row and `quick_check=ok`. Complete canonical
payload, envelope, session ID, revision, and SHA-256 equality was required throughout.

The exact aggregate result was:

- cycles: `10000`;
- successful pre-publication exact old reads: `10000`;
- live-owner primary `SQLITE_BUSY`/`SQLITE_LOCKED` results: `10000`;
- successful post-release exact new reads: `10000`;
- protected live-journal observations: `10000`;
- ambiguous or partial reads, revision gaps/duplicates, digest mismatches, and child-protocol loss,
  duplication, substitution, or reordering: `0`;
- retries, replays, repairs, fallbacks, and inferred acknowledgements: `0`;
- initial revision/digest: `1` /
  `aa5cf90832cf7e71136cfa92208ef923e141d7d8103cab900f642ed02e50b3fb`;
- final revision/digest: `10001` /
  `3304b9ccdfd01f7c211e8e4530be8b533c6b2c506975b83ebceb33f6288eb838`;
- cohort elapsed time: `1m33.87s`; package result: `ok` in `93.891s`.

The quiescent pre/post Linux metadata-tree SHA-256 was stable and equal:
`ccaaef3dc1a4eab9ab808bd5ec040fcdedbde14ab4202ad540aee9fb9f362e90`. The hash covered
LF-terminated ordinally sorted rows containing relative path, entry type, byte size, mode, owner,
device, inode, mount ID, filesystem type/magic, source, and mount point. Journal checks were
metadata-only: no journal was opened or hashed for content. Both children exited through the exact
shutdown protocol; only the parent test framework cleaned the fixture. The caller found the private
temporary parent empty, removed that exact directory, verified it no longer existed, and found no
remaining fixture, child process, binary, or evidence artifact.

The exact successful native command, run from `packages/dorkpipe/lib`, was:

```text
TMPDIR=/tmp/dockpipe-sqlite-linux-publication-vqoEPdfl DORKPIPE_SQLITE_LINUX_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
```

The required focused validation, run from `packages/dorkpipe/lib`, produced:

```text
CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -count=1
PASS; ok in 0.006s
TMPDIR=/tmp/dockpipe-sqlite-linux-publication-smoke-BJ0KRAtZ CGO_ENABLED=0 DORKPIPE_SQLITE_LINUX_EVIDENCE=1 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteSmoke$' -count=1 -v -timeout=10m
PASS; evidence 47ms; package 0.049s; primary contention code SQLITE_BUSY (5); recovery quick_check=ok
go mod verify
PASS; all modules verified
gofmt -d appserversupervisor/sqliteevidence/linux_publication_cohort_test.go appserversupervisor/sqliteevidence/host_linux_test.go appserversupervisor/sqliteevidence/linux_smoke_test.go appserversupervisor/sqliteevidence/sqlite_smoke_test.go
PASS; empty output
git diff --check
PASS; empty output
```

The smoke fixture used a separately created private ext4 parent, which was empty after parent-test
cleanup and then removed exactly. The complete test package also cross-compiled with `CGO_ENABLED=0`
to one new verified private directory outside the repository. `go version -m` confirmed embedded
settings for Windows/`amd64`, Linux/`amd64`, and macOS/`arm64`; the binaries were respectively
12,087,808, 11,588,685, and 10,953,058 bytes. The three exact binaries and their now-empty parent
`/tmp/dockpipe-sqlite-linux-publication-cross-OGrCa3sR` were removed. Cross-compilation remains
compatibility evidence only.

The protected Windows publication cohort remains unchanged and independently qualified. This pass
qualifies only the Linux reader-publication cohort. It does not qualify Linux contention or failure
cohorts, power-loss evidence, production storage, migration, Slice 2, or macOS. Linux contention is
the next native cohort; macOS evidence remains intentionally last. TASK-013 and CAS-14 remain open.


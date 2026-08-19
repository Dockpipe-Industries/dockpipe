**Linux/amd64 1,000-cycle native contention/forced-termination cohort — 2026-08-04.** The new
Linux-only `TestLinuxNativeSQLiteContentionCohort`, gated by
`DORKPIPE_SQLITE_LINUX_CONTENTION_COHORT=1`, passed natively with `CGO_ENABLED=0`,
`-mod=readonly`, `-count=1`, verbose output, and the fixed 30-minute timeout. It ran on Pop!_OS
22.04 LTS, Linux `7.0.11-76070011-generic`, kernel build
`#202606011647~1780583630~22.04~70ad774 SMP PREEMPT_DYNAMIC Thu J`, bare metal according to
`systemd-detect-virt`, `amd64`, and Go `go1.25.0`.

The caller created the new private parent
`/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq` outside the repository with mode `0700`, owner
UID/GID `1000:1000`, stat device `66311`, and inode `57176054`, and used it as `TMPDIR`.
The test-owned fixture root was
`/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq/TestLinuxNativeSQLiteContentionCohort1544489198/001`,
also owned `1000:1000` with mode `0700`. Its retained root identity was mount ID `33`, device
`259:7`, inode `57176199`, and kind `directory`. Metadata-only `statx`,
`O_PATH|O_NOFOLLOW` plus `fstatfs`, and `/proc/self/mountinfo` agreed on ext4 magic `0xef53`,
source `/dev/nvme0n1p3`, mount root/point `/` / `/`, mount options `rw,noatime`, and
super-options `rw,errors=remount-ro,stripe=64`. The exact mountinfo row was:

```text
33 2 259:7 / / rw,noatime shared:1 - ext4 /dev/nvme0n1p3 rw,errors=remount-ro,stripe=64
```

The source block device and temporary parent were non-removable. The lane rejected bind, nested,
overlay, FUSE, network, removable, shared-host, `drvfs`, `9p`, `tmpfs`,
symlinked/substituted, and cross-mount storage. The fixture root plus the Linux-qualified `main`
and `other` session directories remained owned `0700` directories; both main databases and every
observed rollback journal remained owned regular `0600` files on the exact retained
mount/device/inode identities. Only `aggregate.sqlite` and `aggregate.sqlite-journal` were
admitted as database siblings.

The queried engine was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`
and native `unix` VFS. The exact selected absolute URIs were:

```text
file:///tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq/TestLinuxNativeSQLiteContentionCohort1544489198/001/main/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
file:///tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq/TestLinuxNativeSQLiteContentionCohort1544489198/001/other/aggregate.sqlite?_dqs=0&_error_rc=1&_txlock=exclusive&cache=private&mode=rw&vfs=unix
```

Every connection applied and read back `journal_mode=delete`, `synchronous=3` (`EXTRA`),
`fullfsync=1`, `temp_store=2` (`MEMORY`), `mmap_size=0`, `busy_timeout=0`,
`foreign_keys=1`, `trusted_schema=0`, `cell_size_check=1`, `locking_mode=exclusive`, and
pre-schema `page_size=4096`. The lane rejected unresolved double-quoted SQL, required only
`main`, and retained exactly the singleton STRICT `app_server_aggregate` schema with
`user_version=1`. Both initial database connections fail-closed validated this exact sorted
56-option Linux contract:

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

Each cycle used a fresh owner child that loaded the exact committed same-session row, began the
selected exclusive transaction, applied exactly one next-revision CAS, validated the complete staged
row, reported `staged_live` with the exact cycle/revision/digest, and remained live with the
protected rollback journal. The parent validated journal metadata and allowed siblings without
opening journal content. A fresh same-session contender returned only genuine primary
`SQLITE_BUSY` (`5`) or `SQLITE_LOCKED` (`6`). A fresh different-session writer validated,
committed, reloaded, integrity-checked, and closed its independent database while the owner remained
live. The parent then killed the owner before any commit command or acknowledgement existed. A fresh
recovery child performed hot-journal recovery, required exactly one `quick_check=ok`, and returned
the exact old same-session row; the killed owner's staged row did not leak. A separate fresh clean
writer committed the same-session next revision exactly once. Complete canonical payload, envelope,
session ID, revision, and SHA-256 equality was required at every boundary.

Every child accepted exactly one bounded strict JSON-line command, returned exactly one bounded
response carrying the exact cycle and operation, and failed closed on missing, duplicate, malformed,
substituted, unknown-field, multiple-value, or out-of-order data. There were no retries, replays,
repairs, fallbacks, inferred acknowledgements, ambiguous recoveries, revision gaps/duplicates, or
child cleanup.

The exact aggregate result was:

- owner transactions staged: `1000`;
- protected live journals: `1000`;
- same-session primary `SQLITE_BUSY`/`SQLITE_LOCKED` results: `1000`;
- primary code `5`: `1000`; primary code `6`: `0`; sum: `1000`;
- different-session commits while the owner remained live: `1000`;
- forced owner terminations before commit invocation: `1000`;
- exact old-row recoveries: `1000`;
- successful post-recovery same-session commits: `1000`;
- same-session initial revision/digest: `1` /
  `bb0b0fa448e6532a65b420e128470a70fe5e32e15e94634b8c4fcf64a0b1e5ed`;
- same-session final revision/digest: `1001` /
  `e024c4e5dafc3841e26abbc2df7618f2fd78fcabd3b41bd364485b2ad56ff693`;
- different-session initial revision/digest: `1` /
  `8b351be57c3b6f86535ca6c2c3f6ef159175513013f7ac6608413e4e411dedfe`;
- different-session final revision/digest: `1001` /
  `5c78a969b9d47e07ef749d58c6b0fa3311512435141191d79376dd50e2f62f26`;
- cohort elapsed time: `46.672s`; package result: `ok` in `46.703s`.

The stable quiescent pre/post metadata-tree SHA-256 was equal:
`8bc08f6ab798ecde1fb8393281e1b4cef975fc11514634c1deb8b2d641ad37b9`. The hash covered
LF-terminated ordinally sorted metadata rows containing relative path, entry type, byte size, mode,
owner, device, inode, mount ID, filesystem type/magic, source, and mount point. Journal checks were
metadata-only: no journal was opened, parsed, copied, moved, truncated, deleted, or hashed for
content. Only the parent test framework cleaned the fixture. After the pass the caller found the
private temporary parent empty and no contention child process or fixture remained.

The exact successful native command, run from `packages/dorkpipe/lib`, was:

```text
TMPDIR=/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq DORKPIPE_SQLITE_LINUX_CONTENTION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteContentionCohort$' -count=1 -v -timeout=30m
```

The required focused validation, run from `packages/dorkpipe/lib`, produced:

```text
CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -count=1
PASS; ok in 0.002s
TMPDIR=/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq DORKPIPE_SQLITE_LINUX_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
PASS; evidence 1m26.551s; package 86.568s; old reads, BUSY/LOCKED results, new reads, and protected journals 10000 each
TMPDIR=/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq DORKPIPE_SQLITE_LINUX_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestLinuxNativeSQLiteSmoke$' -count=1 -v -timeout=10m
PASS; evidence 50ms; package 0.052s; primary contention code SQLITE_BUSY (5); recovery quick_check=ok
go mod verify
PASS; all modules verified
gofmt -d appserversupervisor/sqliteevidence/linux_contention_cohort_test.go appserversupervisor/sqliteevidence/linux_publication_cohort_test.go appserversupervisor/sqliteevidence/host_linux_test.go appserversupervisor/sqliteevidence/linux_smoke_test.go appserversupervisor/sqliteevidence/sqlite_smoke_test.go
PASS; empty output
git diff --check
PASS; empty output
```

The unchanged publication rerun retained its exact initial revision/digest `1` /
`aa5cf90832cf7e71136cfa92208ef923e141d7d8103cab900f642ed02e50b3fb` and final
revision/digest `10001` /
`3304b9ccdfd01f7c211e8e4530be8b533c6b2c506975b83ebceb33f6288eb838`. Its fresh-fixture
pre/post metadata-tree SHA-256 remained stable and equal:
`203464c60a2224e636e380d42821d0b9fc15a0f1de67efa462ad4992eaf688f8`.

The complete test package cross-compiled with `CGO_ENABLED=0` to one separate newly verified
private ext4 directory outside the repository. `go version -m` confirmed exact embedded settings
for Windows/`amd64`, Linux/`amd64`, and macOS/`arm64`; the binaries were respectively
`12,087,808`, `11,694,037`, and `10,953,058` bytes. The caller removed the three exact
binaries, then their now-empty parent
`/tmp/dockpipe-sqlite-linux-contention-cross-KCL300mg`. Cross-compilation is compatibility evidence
only, not native Windows or macOS evidence. After all validation the caller also removed the empty
verified fixture parent `/tmp/dockpipe-sqlite-linux-contention-Gxjuxbnq`. Neither directory, nor
any binary, fixture, child process, or evidence artifact remained.

The protected Windows contention cohort remains unchanged and independently qualified. Windows
retains exactly 57 options with `COMPILER=gcc-12-win32`, `MUTEX_NOOP`, and `OMIT_SEH`, while
Linux retains exactly 56 options with `COMPILER=gcc-12.2.0` and `MUTEX_PTHREADS`, and no
`MUTEX_NOOP` or `OMIT_SEH`. The Linux publication cohort also remains unchanged and independently
qualified. This pass qualifies only the Linux contention/forced-termination cohort. It does not
qualify Linux failure-boundary or power-loss evidence, production storage, migration, Slice 2, or
macOS. Linux failure-boundary qualification is next; macOS evidence remains intentionally last.
TASK-013 and CAS-14 remain open.


### Closed Transactional-Store, Platform, and VM Evidence

<!-- BEGIN TASK-013 VERBATIM CLOSED HISTORY BLOCK B -->
**Dependency pin and test-only Windows native smoke evidence — 2026-08-04.** This bounded slice pins
`modernc.org/sqlite v1.56.0` and `modernc.org/libc v1.74.4` in
`packages/dorkpipe/lib/go.mod` / `go.sum` and adds only `_test.go` files under
`appserversupervisor/sqliteevidence`. The package remains opt-in through
`DORKPIPE_SQLITE_EVIDENCE=1`; ordinary package regression runs skip the native host probe. The module
directive remains Go `1.25`. The Windows-only DACL/volume evidence directly uses the selected graph's
`golang.org/x/sys v0.47.0`, replacing the prior indirect `v0.28.0`; no other pre-existing selected
module version changed. No `go mod tidy` rewrite was accepted because its proposed checksum cleanup
removed unrelated historical entries.

With `GOWORK=off`, the exact resolved non-main module graph contains 32 entries:

```text
dockpipe v0.0.0 => ../../..
github.com/dustin/go-humanize v1.0.1
github.com/google/pprof v0.0.0-20260802141513-ef3492d7dac3
github.com/google/uuid v1.6.0
github.com/hashicorp/golang-lru/v2 v2.0.7
github.com/lib/pq v1.10.9
github.com/mattn/go-isatty v0.0.24
github.com/mattn/go-shellwords v1.0.12
github.com/ncruces/go-strftime v1.0.0
github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec
github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
golang.org/x/mod v0.37.0
golang.org/x/sync v0.21.0
golang.org/x/sys v0.47.0
golang.org/x/term v0.27.0
golang.org/x/tools v0.47.0
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405
gopkg.in/yaml.v3 v3.0.1
modernc.org/cc/v4 v4.29.1
modernc.org/ccgo/v4 v4.34.6
modernc.org/fileutil v1.4.0
modernc.org/gc/v2 v2.6.5
modernc.org/gc/v3 v3.1.4
modernc.org/goabi0 v0.2.0
modernc.org/libc v1.74.4
modernc.org/mathutil v1.7.1
modernc.org/memory v1.11.0
modernc.org/opt v0.2.0
modernc.org/sortutil v1.2.1
modernc.org/sqlite v1.56.0
modernc.org/strutil v1.2.1
modernc.org/token v1.1.0
```

Every one of those 32 module directories exposed a root license/notice file. The complete scan found
only the repository's existing Apache-2.0 dependency plus permissive Apache-2.0, BSD-style, MIT, and
dual MIT/Apache terms; no module lacked license material. The two selected modernc modules each carry
their BSD-style three-clause license.

The native smoke passed on this exact host and toolchain:

- Windows build `10.0.26200`, `amd64`, Go `go1.26.4`, module language baseline Go `1.25`, and
  `CGO_ENABLED=0`;
- fixed local drive, filesystem `NTFS`, volume
  `\\?\Volume{2eb284d8-09e6-483c-b096-6deed2208642}\`, serial `88c9a133`, label `OS`; the optional
  unprivileged NTFS-version query was unavailable, so no NTFS version is claimed;
- one canonical absolute test-framework fixture root with a protected DACL; the root, pre-created
  empty main/other databases, and every observed journal were owned by current-user SID
  `S-1-5-21-2729925100-2499202611-1015899381-1002` and granted full access only to that SID and
  `SYSTEM` (`S-1-5-18`).

The exact queried runtime identity was SQLite `3.53.3` with source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`.
The test opened one absolute `file:` URI per database with `mode=rw`, `cache=private`, `vfs=win32`,
`_txlock=exclusive`, `_dqs=0`, and `_error_rc=1`; an unresolved double-quoted string was rejected,
proving DQS remained disabled. The bounded compile-option record contained exactly 57 entries:

```text
ATOMIC_INTRINSICS=1,COMPILER=gcc-12-win32,DEFAULT_AUTOVACUUM,DEFAULT_CACHE_SIZE=-2000,
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
MAX_WORKER_THREADS=8,MUTEX_NOOP,OMIT_SEH,SOUNDEX,SYSTEM_MALLOC,TEMP_STORE=1,THREADSAFE=1
```

Every selected pragma was applied and read back on the same dedicated connection:

| Setting | Exact readback |
| --- | --- |
| `journal_mode=DELETE` | `delete` |
| `synchronous=EXTRA` | `3` |
| `fullfsync=ON` | `1` |
| `temp_store=MEMORY` | `2` |
| `mmap_size=0` | `0` |
| `busy_timeout=0` | `0` |
| `foreign_keys=ON` | `1` |
| `trusted_schema=OFF` | `0` |
| `cell_size_check=ON` | `1` |
| `locking_mode=EXCLUSIVE` | `exclusive` |
| pre-schema `page_size=4096` | `4096` |

The test created exactly the selected singleton STRICT table and `user_version=1`, rejected every
unexpected schema object/database/sibling, and kept canonical JSON opaque in the BLOB column. The
revision-1 insert used SHA-256
`5bacd33f5355f1a64a096841fe3fceeca28a40f211723e2ce4bb9b56988e6fe8`; the exact revision-2 CAS
used SHA-256 `37572e06825751539b2e65c19034a23950925abbbe795d296a52ecf1e6e2aca4`. Each commit reloaded
the exact singleton, session ID, revision, payload bytes, and digest through the same connection.
`PRAGMA database_list` contained only `main`.

The SQLite-created rollback journal was a regular file while each write transaction was live and
remained present after both commits at 4,616 bytes. It retained the exact current-user/`SYSTEM`
full-control boundary before and after commit, after forced termination, and after recovery. The test
never opened it for content, parsed it, or altered, truncated, moved, or deleted it; the database
directory contained only `aggregate.sqlite` and `aggregate.sqlite-journal`.

An independent owner child staged revision 3 and held the exclusive transaction. A second process
received primary `SQLITE_BUSY` (`5`) for the same database while a different-database process opened,
ran `quick_check`, and committed successfully. Forced owner termination released the first database
lock. A fresh recovery process opened the database, allowed SQLite recovery, returned exactly one
`quick_check=ok`, revalidated the exact schema, and reloaded the allowlisted old revision 2 (not the
uncommitted revision 3). Child processes performed no cleanup; only the parent test framework removed
the exact temporary root.

With `CGO_ENABLED=0`, the test-only package cross-compiled successfully to temporary binaries outside
the repository for Windows/`amd64`, Linux/`amd64`, and macOS/`arm64`; embedded build settings confirmed
each `GOOS`, `GOARCH`, and `CGO_ENABLED=0`. The inspected binary sizes were 11,464,704 bytes,
11,044,675 bytes, and 10,943,842 bytes respectively. All binaries and the final unique temporary
directory `dockpipe-sqlite-cross-5028fe64ad384dbe8eb341a24d032556` were removed after inspection.
The Linux and macOS results are compile compatibility only; neither is runtime evidence.

The exact native smoke command passed. The required focused regression command separately failed in
protected predecessor work: `TestProtocolBoundaryContainsNoGenericOrPipeonProtocolLeak` observed 17
Pipeon adapter-selector occurrences in protected `extension.ts` while the protected
`protocol_test.go` expected 1. The full `go test ./... -count=1` command then exceeded its five-minute
execution bound without package output. A diagnostic rerun with a 90-second per-package timeout
confirmed the same App Server failure and a separate `orchestrationhelper` timeout in
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRevalidatesImmutableBindings`
while it blocked in the pre-existing reconciliation fixture's `os.ReadFile`; all other reported
packages passed, including `appserversupervisor/sqliteevidence`. Neither protected Pipeon/App Server
file is authorized for this slice, and both still match the accepted protected manifest; no
orchestration-helper file was changed. These validation results do not weaken or expand the
successful SQLite smoke evidence.

**Windows 10,000-cycle native reader-publication cohort — 2026-08-04.** The separate Windows-only
`TestWindowsNativeSQLitePublicationCohort`, gated by
`DORKPIPE_SQLITE_PUBLICATION_COHORT=1`, passed with `CGO_ENABLED=0`, `-mod=readonly`, and the fixed
30-minute timeout. It used the pinned SQLite 3.53.3/source-ID baseline, native `win32` VFS, selected
URI parameters, queried pragmas, singleton STRICT schema, `user_version=1`, fixed local NTFS volume,
and current-user/`SYSTEM` DACL contract already proved by the smoke lane.

One persistent writer child and one persistent reader child used a bounded strict JSON-line protocol.
Every command and response carried the exact cycle number; the reader opened a fresh connection for
each observation, and the writer opened one connection for each staged transaction and closed it
after commit and exact reload. Duplicate, missing, malformed, substituted, or out-of-order commands
and responses fail closed. Children never deleted fixture paths; only the parent test framework
cleaned the exact temporary root.

The exact aggregate result was:

- cycles: `10000`;
- successful pre-publication exact old reads: `10000`;
- live-owner primary `SQLITE_BUSY`/`SQLITE_LOCKED` results: `10000`;
- successful post-release exact new reads: `10000`;
- protected live-journal observations: `10000`;
- ambiguous or partial reads, revision gaps/duplicates, digest mismatches, and child-protocol loss,
  duplication, or reordering: `0`;
- initial revision/digest: `1` /
  `aa5cf90832cf7e71136cfa92208ef923e141d7d8103cab900f642ed02e50b3fb`;
- final revision/digest: `10001` /
  `3304b9ccdfd01f7c211e8e4530be8b533c6b2c506975b83ebceb33f6288eb838`;
- cohort elapsed time: `4m33.39s`.

Every live journal was a regular exact-basename sibling with the selected current-user/`SYSTEM`
full-control DACL, and no unexpected sibling appeared. The quiescent pre/post metadata-tree hash—over
ordinal relative path, entry type, size, and exact DACL evidence without opening or parsing journal
content—was stable and equal before and after the cohort:
`dd678add8ff983d5b8794ab62907ed89b3c162c32fa6d988a29a57e0462b0aaa`.

The existing native smoke rerun passed unchanged. With CGo disabled, the complete test-only package
cross-compiled with embedded target settings confirmed for Windows/`amd64`, Linux/`amd64`, and
macOS/`arm64`; binary sizes were 11,949,568, 11,044,675, and 10,943,842 bytes respectively, and the
temporary directory `dockpipe-sqlite-publication-cross-7d65b0ee4d0d4add952e5130031b3f78` was
removed. These cross-target builds are compile compatibility only.

The focused App Server/provider-session regression result matched the protected baseline exactly:
`providersession` passed and `TestProtocolBoundaryContainsNoGenericOrPipeonProtocolLeak` still found
17 protected Pipeon selector occurrences instead of 1. The bounded full `go test -mod=readonly ./...
-count=1 -timeout=90s` run reported that same App Server failure, and the protected
`orchestrationhelper` suite again timed out in the existing placement-execution graph fixture chain.
The exact timed-out subtest moved from the prior immutable-binding run to
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRejectsMalformedAndUnsafeArtifacts/receipt_noncanonical`
while decoding its pre-existing fixture; all other reported packages passed, including
`appserversupervisor/sqliteevidence`. No protected App Server, Pipeon, or orchestration-helper file
changed, and the timeout difference is not attributable to this isolated Windows-only test file.

**Windows 1,000-cycle native contention/forced-termination cohort — 2026-08-04.** The Windows-only
`TestWindowsNativeSQLiteContentionCohort`, gated by
`DORKPIPE_SQLITE_CONTENTION_COHORT=1`, passed with `CGO_ENABLED=0`, `-mod=readonly`, `-count=1`,
verbose output, and the fixed 30-minute timeout. It ran on Windows build `10.0.26200`, `amd64`, Go
`go1.26.4`, fixed NTFS volume `\\?\Volume{2eb284d8-09e6-483c-b096-6deed2208642}\` with serial
`88c9a133` and label `OS`; the unprivileged NTFS-version query remained unavailable. It revalidated
SQLite `3.53.3`, source ID
`2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`,
the native `win32` VFS, all 57 compile options, the selected absolute URI and queried pragmas, the
singleton STRICT schema and `user_version=1`, and one database per synthetic session. The exact
current-user SID was `S-1-5-21-2729925100-2499202611-1015899381-1002`; the canonical temporary root,
both session directories and main files, and every observed journal granted full control only to
that SID and `SYSTEM` (`S-1-5-18`).

Each cycle used a fresh owner process that loaded the exact committed same-session row, began the
selected exclusive transaction, applied the exact next-revision CAS, validated the complete staged
row, and remained live after reporting `staged_live`. The parent observed the exact regular protected
journal, a fresh contender returned only primary `SQLITE_BUSY`/`SQLITE_LOCKED`, and a fresh independent
different-session writer validated, committed, reloaded, integrity-checked, and closed its different
database while the first owner remained live. The parent then killed the owner before any commit
command existed. A fresh recovery-only process allowed hot-journal recovery, required exactly one
`quick_check=ok`, revalidated schema, database identity, protections, and siblings, and returned the
exact old row rather than the killed owner's staged row. A separate fresh clean writer then committed
that same-session next revision exactly once. Deterministic opaque canonical BLOBs carried exact
adapter, session, revision, unknown-outcome, and permanent no-replay values; complete row and SHA-256
equality was required at every boundary. Children never deleted or altered journals or fixture paths;
only the parent test framework owned temporary-root cleanup.

The exact aggregate result was:

- owner transactions staged: `1000`;
- protected live journals: `1000`;
- same-session primary `SQLITE_BUSY`/`SQLITE_LOCKED` results: `1000`;
- different-session commits while the owner remained live: `1000`;
- forced owner terminations before commit invocation: `1000`;
- exact old-row recoveries: `1000`;
- successful post-recovery same-session commits: `1000`;
- ambiguous recoveries, staged-row leaks, revision gaps/duplicates, digest/envelope mismatches,
  unexpected siblings/protection widening, and child-protocol loss/duplication/reordering: `0`;
- same-session initial revision/digest: `1` /
  `bb0b0fa448e6532a65b420e128470a70fe5e32e15e94634b8c4fcf64a0b1e5ed`;
- same-session final revision/digest: `1001` /
  `e024c4e5dafc3841e26abbc2df7618f2fd78fcabd3b41bd364485b2ad56ff693`;
- different-session initial revision/digest: `1` /
  `8b351be57c3b6f86535ca6c2c3f6ef159175513013f7ac6608413e4e411dedfe`;
- different-session final revision/digest: `1001` /
  `5c78a969b9d47e07ef749d58c6b0fa3311512435141191d79376dd50e2f62f26`;
- elapsed time: `3m19.25s`.

The stable quiescent pre/post metadata-tree hash was equal:
`9e4b6e98a9ce839c24ee20cb21f56ecc379eff03133782b593fb10b936e511b8`. The hash is SHA-256 over
LF-terminated rows sorted by ordinal relative path; each row contains relative path, entry type,
byte size, and exact owner/DACL evidence. It opens and hashes no journal content.

The unchanged 10,000-cycle publication cohort rerun passed in `4m19.979s` with its existing counters,
initial/final revisions and digests, and equal metadata-tree hash
`dd678add8ff983d5b8794ab62907ed89b3c162c32fa6d988a29a57e0462b0aaa`. The unchanged native Windows
smoke passed. The complete test package cross-compiled with embedded target settings confirming
`CGO_ENABLED=0` for Windows/`amd64`, Linux/`amd64`, and macOS/`arm64`; binary sizes were 12,056,064,
11,044,675, and 10,943,842 bytes. The verified temporary directory
`dockpipe-sqlite-contention-cross-da2f4b7dfff54e789acf71baa98b4890` was removed.

The focused regression again passed `providersession` and failed only the protected
`TestProtocolBoundaryContainsNoGenericOrPipeonProtocolLeak` assertion at 17 occurrences instead of 1.
The bounded full suite reported that same failure, passed `appserversupervisor/sqliteevidence`, and
again timed out in the protected orchestration-helper reconciliation fixture chain. This run's exact
active subtest moved from the preceding `receipt_noncanonical` case to
`TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRequiresExactAuthorization/inferred_decision`,
blocked in the same pre-existing `os.ReadFile` path. `go mod verify` returned `all modules verified`;
`gofmt -d` for the new file was empty, and `git diff --check` passed. No protected predecessor file
was edited.

The exact validation commands were run from `packages/dorkpipe/lib` (environment assignments shown
in portable prefix form):

```text
DORKPIPE_SQLITE_CONTENTION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteContentionCohort$' -count=1 -v -timeout=30m
DORKPIPE_SQLITE_PUBLICATION_COHORT=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLitePublicationCohort$' -count=1 -v -timeout=30m
DORKPIPE_SQLITE_EVIDENCE=1 CGO_ENABLED=0 go test -mod=readonly ./appserversupervisor/sqliteevidence -run '^TestWindowsNativeSQLiteSmoke$' -count=1 -v
CGO_ENABLED=0 GOOS=<windows|linux|darwin> GOARCH=<amd64|amd64|arm64> go test -mod=readonly -c -o <verified-temporary-binary> ./appserversupervisor/sqliteevidence
go version -m <verified-temporary-binary>
go test -mod=readonly ./appserversupervisor ./providersession -count=1
go test -mod=readonly ./... -count=1 -timeout=90s
go mod verify
gofmt -d appserversupervisor/sqliteevidence/windows_contention_cohort_test.go
git diff --check
```


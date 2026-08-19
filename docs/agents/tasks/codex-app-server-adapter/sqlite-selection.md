**Authorized SQLite selection — exact design/evidence baseline (2026-08-04).** This selection accepts
the logical-one-aggregate/physical-database distinction and replaces only the earlier raw-file
storage-primitive dependency direction if a later implementation is authorized. It does not modify
the inert Slice 1 code or path, add dependencies, create a database, authorize an evidence prototype,
start Slice 2, change cutover or lifecycle semantics, reduce DockPipe platform support, or permit a
Linux-only lane.

| Surface | Selected baseline | Required fail-closed check |
| --- | --- | --- |
| Go binding | [`modernc.org/sqlite v1.56.0`](https://pkg.go.dev/modernc.org/sqlite@v1.56.0), a CGo-free `database/sql` driver, with the binding-required [`modernc.org/libc v1.74.4`](https://gitlab.com/cznic/sqlite/-/raw/v1.56.0/go.mod). `github.com/mattn/go-sqlite3` is rejected because it requires CGo and a platform compiler toolchain. | Future module edits must pin both exact versions, retain `CGO_ENABLED=0` builds, review the complete transitive module graph and licenses, and fail if the resolved versions differ. No module edit is authorized here. |
| SQLite engine | The selected binding embeds SQLite 3.53.3 with source ID `2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62` ([3.53.3 release](https://sqlite.org/releaselog/3_53_3.html)). The closed 3.53.4 delta qualification below accepts this exact engine as the production dependency pin for the selected bounded store. | Query and require exact `sqlite_version()` and `sqlite_source_id()` before any store access. Any binding, engine, schema, SQL-operation, extension, `ATTACH`, VFS, journal, or synchronization change requires a fresh delta review. No module edit is authorized here. |
| Physical scope and name | One private directory per validated Pipeon session at the existing package-owned aggregate root, using the existing SHA-256 session-name derivation; future shape `<aggregate-root>/<session-digest>/aggregate.sqlite`. `aggregate.sqlite-journal` is the only selected SQLite sidecar. | Reject raw session IDs in paths, links/reparse points, nested/cross mounts, non-local storage, aliases, substituted identities, unexpected siblings, and any `-wal`, `-shm`, super-journal, attached database, or alternate database file. The existing inert `.json` path is not changed by this slice. |
| Open/VFS contract | One absolute file URI opened `mode=rw&cache=private`, with explicit `vfs=win32` on Windows and `vfs=unix` on Linux/macOS. The main file is first created empty by the future platform-specific private-file operation and parent entry is synchronized before SQLite opens it; empty physical existence never creates lifecycle authority. SQLite documents `win32` and `unix` as the native defaults ([VFS](https://sqlite.org/vfs.html)). | Prohibit `mode=rwc`, shared cache, `immutable`, `nolock`, `psow`, custom/no-lock VFSes, URI authorities, relative paths, `ATTACH`, loadable extensions, and backup/rename/copy while open. Require the opened database and parent identities to match the prevalidated session path. |
| Connection ownership | Exactly one dedicated `database/sql` connection for one provider-pool owner operation; pool limits are one open and one idle connection, and the handle is closed at the operation boundary. Driver parameters are `_txlock=exclusive`, `_dqs=0`, and `_error_rc=1`. | No second connection, helper process, reader pool, global database, long-lived idle connection, or non-provider-pool writer. Close releases the SQLite-held OS lock; close failure cannot upgrade an uncertain result to success. |
| Journal/durability | `PRAGMA journal_mode=DELETE`, `synchronous=EXTRA`, `fullfsync=ON`, `temp_store=MEMORY`, `mmap_size=0`, `busy_timeout=0`, `foreign_keys=ON`, `trusted_schema=OFF`, and `cell_size_check=ON`; database page size is fixed to 4096 before schema creation. SQLite documents rollback `EXTRA` as ACID and its directory sync after a DELETE-mode journal unlink ([synchronous](https://sqlite.org/pragma.html#pragma_synchronous)). | Apply settings on the dedicated connection, query every selected value back, require the exact returned value, record `PRAGMA compile_options`, and abort before observation on an ignored, substituted, unsupported, or mismatched setting. No default value supplies authority. |
| Lock window | Set and verify `PRAGMA locking_mode=EXCLUSIVE`, then acquire `BEGIN EXCLUSIVE` before reading any legacy source or recovery evidence. SQLite documents that exclusive locking mode retains file locks across transaction completion until the connection closes ([locking mode](https://sqlite.org/pragma.html#pragma_locking_mode)); the same connection performs post-commit strict reload before closing. | `busy_timeout=0` keeps each SQLite attempt nonblocking. Provider-pool may poll only lock acquisition, with caller cancellation and the already accepted absolute 30-second cap. `BUSY`/`LOCKED` after observation begins is not retried. Different session databases must remain independently acquirable. |
| Sidecar/privacy | In exclusive locking mode the rollback journal can remain after commit and is part of the physical store, not stale cleanup material. SQLite requires a hot journal to stay paired with its database ([temporary files](https://sqlite.org/tempfiles.html), [corruption hazards](https://sqlite.org/howtocorrupt.html)). Parent directories remain Unix `0700` or Windows current-user/`SYSTEM`; main and journal files must be Unix `0600` or carry the same restricted Windows DACL. | Never parse, move, copy, replace, truncate, quarantine, or delete the journal outside SQLite. Verify type, identity, ownership/DACL/mode, and sibling set before and after the operation. Any unexplained file or protection widening blocks; future native evidence must prove the SQLite-created journal meets the exact protection contract on all three tuples. |

The selected schema preserves the existing Slice 1 canonical JSON as the only lifecycle payload. SQL
columns provide a singleton/CAS envelope and must equal the values decoded from those exact canonical
bytes; neither an envelope field nor the database file's existence is independently authoritative:

```sql
CREATE TABLE app_server_aggregate (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    pipeon_session_id TEXT NOT NULL CHECK (length(pipeon_session_id) BETWEEN 1 AND 256),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    canonical_json BLOB NOT NULL CHECK (length(canonical_json) BETWEEN 1 AND 16384),
    canonical_sha256 BLOB NOT NULL CHECK (length(canonical_sha256) = 32)
) STRICT;
PRAGMA user_version = 1;
```

Initial migration requires no row, then inserts singleton `1` only after the accepted legacy-source
reread and byte comparison. Later mutation uses one conditional update matching singleton, exact
session ID, previous revision, and previous canonical SHA-256; zero or multiple affected rows reject.
Before commit, provider-pool decodes and byte-compares the candidate row. After commit, the same
exclusive-locking connection reloads the row, requires one exact canonical aggregate, revalidates all
existing schema/identity/revision/fingerprint/outcome invariants, and only then classifies committed.

**Selected error classification.** Lock-acquisition `SQLITE_BUSY`/`SQLITE_LOCKED` may be polled only
before observation. A pre-write validation or zero-row CAS failure rolls back and rejects. After a
write starts but before `COMMIT`, success requires an explicit rollback and exact old-row reload;
rollback/reload uncertainty becomes `unknown_commit_result`. Once `COMMIT` is invoked, it is never
retried. A documented `SQLITE_BUSY` commit leaves the transaction active, so one rollback plus exact
old-row reload may prove unchanged; every other commit error, connection loss, process/OS loss, or
failure to prove the exact old row is unknown. Commit success followed by reload, sidecar, permission,
close, or acknowledgement failure is also unknown until a fresh recovery-only restart open permits
SQLite to apply any required hot-journal recovery and then strictly reloads without an application
write. No branch authorizes resend, re-observation, repair, replay, fallback, inferred terminal
outcome, or a second commit attempt.

**Selected native-evidence plan.** The initial evidence cohorts remain Windows/local fixed-disk
NTFS/`amd64`, Linux 5.8+/local ext4/`amd64`, and macOS/local APFS/`arm64`; this selects test cohorts,
not a production allowlist or a reduction of general DockPipe support. Each run records exact OS/build,
kernel, architecture, filesystem/volume version and properties, storage device/virtualization facts,
Go version, module graph, SQLite version/source ID, compile options, VFS, queried pragmas, DACL/modes,
and pre/post file-tree hashes. The existing independent-process protocol and counts remain: 10,000
old/new reader-publication cycles, 1,000 same-session contention/forced-termination cycles while a
different-session writer succeeds, every deterministic failure boundary, and three controlled VM
hard-reboot or power-loss trials at every durability boundary. Readers may fail closed with
`BUSY` while the exclusive owner is live, but every successful post-release read must be exactly the
old or new canonical row; `quick_check`, strict decode, envelope equality, revision monotonicity,
permanent no-replay state, sidecar pairing, and protection checks must all pass.

**Closed SQLite 3.53.4 version-skew qualification (2026-08-04).** The newest exact CGo-free binding
remains `modernc.org/sqlite v1.56.0`; its tagged `go.mod` requires Go 1.25.0 and
`modernc.org/libc v1.74.4`, while its supported matrix retains Windows/`amd64`, Linux/`amd64`, and
macOS/`arm64` on SQLite 3.53.3. The complete official [3.53.3-to-3.53.4 check-in
timeline](https://sqlite.org/src/timeline?from=version-3.53.0&to=version-3.53.4&to2=branch-3.53&y=ci)
was reviewed against the selected schema and operation surface:

- Check-in `bf70dadc2d` changes hot-journal recovery only for a crash-corrupted super-journal record.
  The originating [official report](https://sqlite.org/forum/info/2026-07-20T18:27:00Z) states that
  the defect requires a multi-database `ATTACH` transaction. This store prohibits `ATTACH`, alternate
  databases, and super-journals before open and rejects any unexpected sibling, so that state is
  unreachable without an already-fail-closed contract violation.
- Check-in `a210f6f939` replaces an unchecked double-to-`int64` cast in VFS current-time conversion and
  one window-frame numeric check. The fixed store issues no date/time or window SQL, uses
  `busy_timeout=0`, and does not derive locking, synchronization, commit, recovery, or error authority
  from VFS wall time.
- Check-in `5d7c6fe1e9` affects expression indexes, subtypes, and unary `+`; the selected singleton table
  has no expression index, subtype operation, or unary-`+` SQL.
- Every other intervening check-in is patch metadata or is confined to the CLI/shell, `sqlite3_rsync`,
  tests, FTS3/4/5, RBU, session/rebaser, JSON/JSONB, `fileio`, incremental-integrity-check,
  `normalize`, Fossil-delta, `series`, `amatch`, or `fuzzer` surfaces. None is loaded, invoked, or
  represented by the fixed private schema and bounded SQL; canonical lifecycle JSON remains an opaque
  `BLOB`, loadable extensions are prohibited, unexpected schema is rejected, and `quick_check` does
  not authorize any of those features.

Therefore every 3.53.4 fix is demonstrably outside the selected rollback-journal, exclusive-locking,
native-VFS, synchronization, sidecar, fixed-schema, and error-classification surface. The version-skew
gate is closed: `modernc.org/sqlite v1.56.0` + `modernc.org/libc v1.74.4` + the exact SQLite 3.53.3
source ID above is the accepted production dependency baseline. This acceptance does not authorize a
module edit, evidence harness, implementation, migration, cutover, lifecycle activation, or Slice 2.

**Selection result and remaining gates.** The storage shape, binding family, exact evidence versions,
schema, VFS mapping, transaction settings, lock window, sidecar policy, error policy, and evidence plan
are selected. The prior standard-library-plus-`x/sys` and persistent empty lock-file direction is
superseded only for this SQLite candidate; no engine code is implicated. Production dependency
selection is closed on the exact versions and source ID above. The separately authorized dependency
pin and test-only Windows smoke slice below completes only the module edit and bounded evidence
harness authorization. Production use remains gated by the complete three native cohorts proving
exact lock release, journal protection, power-loss durability, and host eligibility, plus later
maintainer authorization for any Slice 1 path/loader revision, Slice 2 implementation, migration,
cutover, or lifecycle activation. Slice 2 remains blocked.

Closed dependency-pin, native transactional-store, and Linux VM qualification history is retained
verbatim in the
[TASK-013 closed history and evidence archive](history-store-windows-smoke.md#closed-transactional-store-platform-and-vm-evidence).

The implementation test matrix is:

1. **Adapter selection:** a new normal Pipeon Codex session defaults to App Server; the explicit exec
   escape hatch works; `/codex`, other providers, workflows, existing bindings, and callers without a
   Pipeon adapter choice retain current behavior; unknown values fail closed; a session never drifts
   or rebinds adapters.
2. **Model and capability selection:** validated available stable model/reasoning combinations render
   and execute exactly as selected; removal, mismatch, reroute, and unsupported combinations fail
   visibly without substitution. Unknown authority-expanding capabilities remain disabled and each
   experimental capability requires its own advanced opt-in.
3. **Lifecycle/rendering:** starting, ready, running, both waiting states, completed, interrupted,
   failed, disconnected, and recovery-required render from neutral records with contiguous cursors;
   the effective adapter/model/reasoning/approval/sandbox policy stays visible; no raw protocol or
   private payload reaches extension source or persisted state.
4. **Policy and decisions:** native configured approval plus workspace-write are the new-session
   defaults. Manual and native automatic-review modes remain distinct from sandbox authority;
   broader access requires conspicuous per-session confirmation and is not inherited. Approval and
   user-input responses require the complete current one-time
   correlation; duplicate, stale, cross-session, cross-process, and post-disconnect responses fail
   closed. Denial remains denial; DockPipe never blindly approves on Codex's behalf.
5. **Fallback/rollback:** initialization failures may fall back before `turn/start`; every failure
   after dispatch blocks replay; the exec escape hatch and administrative App Server disablement
   handle new, idle, active, waiting, and disconnected sessions exactly as above; automatic
   reconciliation returns ready only after verified idle.
6. **Regression:** the existing provider-pool Codex exec binding/resume tests, host bridge tests,
   bounded-worker tests, package contract tests, and Pipeon webview smoke tests remain green.
7. **Controlled integration:** through the primary/default App Server route, one Pipeon session
   selects an advertised model/reasoning combination, completes a no-tool turn,
   denies a requested file change, answers bounded user input, interrupts a turn, and observes
   transport/child loss without replay. Separate cases prove native automatic review within
   workspace-write, explicit broader-access confirmation, non-inheritance, and the `codex_exec`
   escape hatch.

**Bounded transactional-store reconsideration — SQLite candidate (2026-08-04).** This subsection
compares SQLite against the accepted safety invariants only. It does not select a dependency or
configuration, authorize implementation or a prototype, reduce Windows/Linux/macOS support, change
lifecycle authority, or supersede the accepted product/storage direction above.

The only candidate shape that survives the comparison is one private database per App Server
session, stored in that session's package-state directory, with one strict authoritative aggregate
record. A single shared database is rejected as a candidate shape: SQLite permits only one writer
per database, so it would serialize unrelated session writers and violate the accepted requirement
that different sessions not share lock authority or needlessly block one another. The database and
any SQLite-required journal, WAL, shared-memory, or temporary support files are collectively the
physical store; no file's existence, including the main database file, independently establishes
lifecycle authority. Authority would still require one successfully committed transaction followed
by strict aggregate schema, identity, revision, fingerprint, and outcome validation.

| Accepted safety invariant | SQLite documentation result | Reconsideration result |
| --- | --- | --- |
| One provider-pool transaction owner; unrelated sessions remain independent | SQLite serializes writers per database, including across processes, while separate databases have separate locking domains ([transaction isolation](https://www.sqlite.org/isolation.html), [transactions](https://www.sqlite.org/lang_transaction.html)). | **Conditional match only with one database per session.** A shared cross-session database is rejected. Provider-pool ownership and the existing single-session mutation boundary remain unchanged. |
| One authoritative aggregate and one indivisible old-or-new commit | SQLite documents serializable ACID transactions and atomic commit; in rollback mode the rollback journal protects the pre-transaction state until the commit point ([transactional guarantee](https://www.sqlite.org/transactional.html), [atomic commit](https://www.sqlite.org/atomiccommit.html)). | **Strong candidate match.** One transaction can replace the aggregate record without composing rename, directory-sync, and replacement primitives in package code. Exact SQL schema and representation remain unselected. |
| Crash/restart and power interruption expose one valid old or new revision, never a partial revision | SQLite documents hot-journal recovery after process or OS failure and durability through power loss, subject to truthful locking, flush, deletion, and storage-device behavior. Rollback mode with `synchronous=EXTRA` additionally syncs the containing directory after journal unlink ([atomic commit](https://www.sqlite.org/atomiccommit.html), [`synchronous`](https://www.sqlite.org/pragma.html#pragma_synchronous)). | **Documentation-supported but not yet accepted.** The guarantee still depends on an exact version, VFS, journal mode, synchronization mode, filesystem, OS, and hardware contract plus native interruption evidence on every supported platform. |
| Cross-process exclusion spans source observation, recovery observation, comparison, commit, and authoritative reload | SQLite's pager and VFS use operating-system locks to coordinate processes; the documented Windows VFS uses `LockFile`/`LockFileEx`, and Unix VFSes use advisory locks ([locking](https://www.sqlite.org/lockingv3.html), [VFS](https://www.sqlite.org/vfs.html)). | **Partial match.** A later design must prove that one write transaction holds the required per-session exclusion for the entire accepted observation/compare/commit window and fails closed on contention, expiry, cancellation, or lock failure. No prototype is authorized by this packet. |
| An ambiguous commit response never authorizes replay, retry, fallback, or inferred terminal outcome | SQLite exposes transaction state on a live connection, while commit and I/O failures can have result-dependent transaction effects ([transactions](https://www.sqlite.org/lang_transaction.html), [`sqlite3_get_autocommit`](https://www.sqlite.org/c3ref/get_autocommit.html)). | **Policy remains application-owned.** After connection, process, OS, or host loss, provider-pool must reopen and strictly reload the exact expected revision. If the result cannot be proven, it must preserve `reconciled_outcome_unknown`; it must never resend, replay, repair, or fall back automatically. Exact binding-specific error mapping remains unresolved. |
| Support files, stale files, and cleanup cannot become independent authority | SQLite rollback journals, WAL files, and shared-memory files can be required for recovery or database integrity; SQLite explicitly treats temporary-file details as implementation-dependent ([temporary files](https://www.sqlite.org/tempfiles.html), [WAL](https://www.sqlite.org/wal.html)). | **Conditional match.** Required sidecars must live in the same private session directory, inherit the accepted protection boundary, survive recovery, and never be parsed as lifecycle authority. Exact allowed files, cleanup rules, and backup/move semantics must be pinned to the selected SQLite version and mode. |
| The store is private to the current user and supported on Windows, Linux, and macOS without weakening the allowlist | SQLite provides built-in Windows and Unix VFSes and supports Windows, Linux, and macOS, but warns that correctness depends on reliable filesystem locks and sync behavior; dangerous no-lock VFS variants exist ([VFS](https://www.sqlite.org/vfs.html), [features](https://www.sqlite.org/features.html), [URI parameters](https://www.sqlite.org/uri.html)). | **Platform breadth match; environment proof gap remains.** The accepted Windows DACL and Unix `0700`/`0600` requirements still apply to the per-session directory and every physical store file. Network/removable filesystems, aliases/links, no-lock VFSes, and unproven host/storage combinations remain excluded pending an exact all-platform allowlist. |
| Storage substitution cannot alter recovery, cutover, dispatch, projection, or explicit-user-decision semantics | SQLite supplies storage transactions and recovery, not App Server lifecycle authority. | **No lifecycle change.** The accepted legacy-source reread and byte comparison, exact compare-and-commit rules, cutover boundary, projection-only Pipeon state, later-turn claiming, explicit user decision, and permanent no-replay guarantees remain mandatory and separately gated. |

**Decision result.** SQLite materially improves the documented transactional-store fit over a
package-owned composition of raw rename and directory-sync primitives, and a private per-session
database is therefore retained as a credible alternative candidate. It is not accepted for
implementation. The comparison does not prove the exact cross-process observation window, select
the physical transaction configuration, close the host/filesystem/hardware evidence gate, or resolve
sidecar protection and ambiguous-error mapping.

Before this candidate could replace the accepted canonical-file direction, maintainers must
separately accept the logical-one-aggregate/physical-database distinction and then authorize a later
selection slice for the exact SQLite version and Go binding, VFS, journal and synchronization modes,
database schema, sidecar/permission/cleanup contract, lock acquisition and timeout behavior,
binding-specific error taxonomy, and Windows/NTFS/amd64, Linux/ext4/amd64, and macOS/APFS/arm64
allowlist and native evidence plan. Until then, the all-platform gate remains unmet, Slice 2 remains
blocked, and implementation and prototype work remain unauthorized.


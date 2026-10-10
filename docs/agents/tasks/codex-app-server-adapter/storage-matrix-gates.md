**Compact 13-property matrix.** `D` means the published docs support the candidate and it is eligible
for later native evidence; `R` means unresolved after the primary surfaces listed above were
exhausted; `U` means a published limitation makes that alternative unsupported. `D` is not executed
proof or primitive acceptance. Locks remain advisory on Unix and mandatory only for Windows byte
I/O (not mapped access); lock bytes/existence never grant authority. Restart means a fresh process,
while durability also requires the separately controlled hard-reboot/power-loss lane.

| # / required property | Windows / NTFS / `amd64` | Linux / ext4 / `amd64` | macOS / APFS / `arm64` | Documentation status |
| --- | --- | --- | --- | --- |
| 1. Session-scoped cross-process exclusive locking | `CreateFileW` + `LockFileEx(EXCLUSIVE|FAIL_IMMEDIATELY)`, byte `[0,1)` | retained inode + advisory `flock(EX|NB)` | retained inode + advisory `flock(EX|NB)` | W:D; L:D; M:D |
| 2. Release after normal exit, crash, termination | exact `UnlockFileEx`/handle close; termination closes kernel handles | `LOCK_UN` or last close; `O_CLOEXEC` prevents exec leak | `LOCK_UN` or last close; `O_CLOEXEC` prevents exec leak | W:D; L:D; M:D |
| 3. Same-session contention; different-session independence | one validated digest-named file per session | one validated digest-named inode per session | one validated digest-named inode per session | W:D; L:D; M:D; mapping still needs process evidence |
| 4. Same-directory exclusive temp creation | sibling `CreateFileW(CREATE_NEW)` | dirfd-relative `openat2(O_CREAT|O_EXCL|O_NOFOLLOW)` | dirfd-relative `openat(O_CREAT|O_EXCL|O_NOFOLLOW)` | W:D; L:D; M:D |
| 5. Atomic first publication without overwrite | `FileRenameInfoEx`, flags `0`; target-exists failure is documented, concurrent complete-file visibility is not | `renameat2(RENAME_NOREPLACE)`; ext4 support since Linux 3.15 | path-based `RENAME_EXCL` capability documents a pre-existing-target warning, but `renameatx_np` publishes only a prototype and no descriptor-relative flag/error/racing-reader atomicity contract | W:R; L:D; M:R |
| 6. Atomic replacement of one complete revision | `FileRenameInfoEx(REPLACE_IF_EXISTS|POSIX_SEMANTICS)` preserves old handles and routes later opens to new, but lacks a racing-reader atomic old/new contract; `ReplaceFileW` is U | same-dirfd `renameat2(..., 0)` documents atomic replacement | general `rename` keeps an instance of the new name through a crash, but flags-`0` `renameatx_np` replacement and exact racing-reader old/new visibility are undocumented | W:R; L:D; M:R |
| 7. File-content sync before publication | exact temp write + `FlushFileBuffers` | exact temp write + `fsync(tempfd)` | `F_FULLFSYNC`; archived support list does not include APFS and current iOS guidance is best-effort, not an APFS/macOS contract | W:D; L:D; M:R |
| 8. Visible aggregate sync after publication | reopen/identity/canonical check + `FlushFileBuffers` | dirfd reopen/identity/canonical check + `fsync` | reopen/identity/canonical check + `F_FULLFSYNC`; no published APFS/macOS persistence guarantee | W:D; L:D; M:R |
| 9. Parent-directory entry sync | source handle opened `FILE_FLAG_WRITE_THROUGH`; NTFS documents rename-metadata flush to disk; no directory or volume flush | `fsync(parent-dirfd)` is explicitly required | no documented directory-entry durability primitive | W:D; L:D; M:R |
| 10. Restart and power-loss visibility | cannot qualify without 5 and 6; write-through hardware support is not universal | full sequence is documented to survive crash/reboot; hard-power evidence remains mandatory | general rename/APFS crash-protection statements do not qualify the exact sequence; cannot qualify without 5-9 | W:R; L:D; M:R |
| 11. Known failure vs unknown result | only pre-publication/proven-unchanged failures are known; all other post-invocation outcomes unknown | same conservative rule; exact old/new restart reload classifies | same conservative rule; unresolved API errors remain unknown | W:D; L:D; M:D as package policy; no automatic retry |
| 12. Reject link/reparse/mount/path/cross-volume substitution | retained non-delete directory handles, `OPEN_REPARSE_POINT`, reparse rejection, file/volume IDs | `openat2` containment + `NO_XDEV`, `statx` mount ID, inode/device checks | component `O_NOFOLLOW`, `fstatfs`, volume ID, and mount-trigger detection lack a documented complete race-free nested/same-filesystem mount-substitution proof | W:D; L:D; M:R |
| 13. Exact local filesystem and minimum-version support | fixed NTFS + NTFS major/minor is detectable; candidate Windows 10 1709+; no version accepted | fixed local ext4; Linux 5.8+ for mount ID; exact kernel/ext4/mount allowlist awaits evidence | APFS v2+ and macOS/`arm64` baseline facts are documented, but no unprivileged exact APFS-feature/OS-build/hardware contract maps them to properties 5-12 | W:D detection only; L:D candidate; M:R |

**Tuple results.** Windows/NTFS/`amd64` is not documentation-qualified because properties 5, 6, and
10 remain unresolved; property 9 now has a documentation-supported write-through candidate, not
implementation acceptance. Linux/ext4/`amd64` is documentation-qualified for a future native evidence
prototype at Linux 5.8+ on an exact later-reviewed allowlist, but that prototype is not authorized.
macOS/APFS/`arm64` is not documentation-qualified because properties 5-10, 12, and the exact
APFS/macOS/host-eligibility contract in 13 remain unresolved. APFS version-2 and `arm64` baseline
facts do not supply that contract. The all-or-nothing documentation gate is unmet; no platform may
begin prototype evidence and Slice 2 remains blocked.

**Documentation gap audit.** The focused Windows re-audit exhausted these Microsoft surfaces:
`CreateFileW` and File Caching; `FlushFileBuffers`, `ZwFlushBuffersFile`, and
`IRP_MJ_FLUSH_BUFFERS`; `SetFileInformationByHandle`, `FILE_INFO_BY_HANDLE_CLASS`,
`FILE_INFORMATION_CLASS`, `FILE_RENAME_INFO`, `FILE_RENAME_INFORMATION`, the MS-FSCC
`FileRenameInformation`/`FileRenameInformationEx` structures, and the MS-FSA
`FileRenameInformation` algorithm; `LockFileEx`/`UnlockFileEx` and process termination;
`FILE_ID_INFO`, `GetVolumeInformationByHandleW`, `GetDriveTypeW`, `FSCTL_GET_NTFS_VOLUME_DATA`,
`NTFS_VOLUME_DATA_BUFFER`, and the NTFS overview; `WRITE_THROUGH` capability reporting;
`IOCTL_VOLSNAP_FLUSH_AND_HOLD_WRITES`; and `ReplaceFileW`. They establish no-overwrite failure,
same-volume/same-directory addressing, old-handle retention, later-open routing, file-content flush,
NTFS rename-metadata write-through, identity/detection surfaces, and the Windows 10 1709 information-
class floor. They do not establish racing-reader atomic first publication or replacement, a complete
post-failure state map, universal hardware power-loss persistence, or an exact NTFS format-version
floor. Property 9 is documentation-supported only through the source handle's
`FILE_FLAG_WRITE_THROUGH`, not a retained directory handle or administrative volume flush;
properties 5, 6, and 10 remain unresolved. For Linux, the reviewed Linux man-pages and
kernel docs explicitly cover `openat2`, `flock`, `open`/`O_EXCL`, `renameat2`, file and directory
`fsync`, `statx`, mountinfo, and ext4 journaling. They qualify the exact candidate for later evidence,
but do not prove the composed application state machine or an accepted environment allowlist. For
macOS, the audit exhausted Apple's published `open(2)`, `flock(2)`, `rename(2)`, `fsync(2)`,
`fcntl(2)`, and `statfs(2)` pages; the APFS Tools and APIs guide; About Apple File System; the APFS
Guide introduction, FAQ, filesystem-details, and volume-comparison pages; current disk-write
guidance; Foundation exclusive-rename, mount-trigger, volume-identifier, and related volume-capability
properties; the 2020-06-22 Apple File System Reference; the current Disk Utility APFS-format guide;
and Apple-silicon porting guidance. The exact results are narrower than the exposed names:
`renameatx_np` has a published prototype but no flag/error/atomicity contract; path-based
`RENAME_EXCL` capability and general `rename` do not establish the descriptor-relative first-create
or replacement reader contract; `F_FULLFSYNC` has no published APFS/macOS guarantee and current iOS
guidance calls it best-effort; no unprivileged parent-directory entry sync is documented; the
identity and mount-trigger values do not provide complete race-free nested-mount containment; and the
APFS format reference's version/feature/software fields are not an unprivileged runtime compatibility
contract binding an exact macOS build and host to the required primitives. Properties 5-10, 12, and
13 therefore remain unresolved. Apple open source can corroborate symbols only and was not used to
fill a documentation gap. Native execution remains required for every `D` cell; cross-compilation
proves only that build tags compile and cannot change a platform result.

**Future independent-process evidence protocol.** The evidence harness remains design only.

1. A parent test controller creates one canonical absolute fixture root under the test framework's
   temporary root, records its resolved identity and a random run token, and passes both explicitly
   to children. Children reject any path outside that exact root, any changed root identity, and any
   relative, parent-traversing, linked, mounted, or reparse-substituted component. Only the parent may
   clean up, after revalidating the exact absolute root and token; no child runs recursive deletion.
2. One test executable exposes private child roles through arguments: `lock-holder`,
   `same-session-contender`, `different-session-contender`, `publisher`, `reader`, `fresh-verifier`,
   and `fault-controller`. Roles communicate readiness and acknowledgement only through inherited
   pipes/handles. They share no mutex, heap, in-process callback, or authoritative marker file.
3. The lock holder acquires the OS lock and reports entry. The same-session contender must not enter
   until release; the different-session contender must enter while the first remains held. Repeat
   after normal exit, forced termination, and crash. Each entrant proves the lock path still names
   the locked inode/file ID. Lock bytes and mere existence are deliberately varied and ignored.
4. Seed exact canonical old and new aggregate byte strings with distinct revisions and hashes.
   Multiple publisher processes repeatedly alternate complete revisions while multiple independent
   readers race the visible target. Every read must equal exactly one allowlisted complete canonical
   byte string and validate its session/revision; missing, empty, truncated, mixed, duplicate-key,
   extra-record, wrong-session, alternate-path, or substituted-inode data fails the run.
5. Fault hooks surround lock open/acquire, source open/read, observation boundary, source reread,
   temp create, every partial/full write, temp sync, publish syscall entry/return, visible reopen,
   identity check, visible sync, parent-directory sync, strict reload, result construction, and
   acknowledgement. The controller may return an injected error, close a handle, substitute a path,
   terminate the child, or crash it at each hook. It never treats a test marker as commit authority.
6. For a known failure before publication, a fresh verifier must see exact legacy/old authority. For
   any loss after publication is invoked and before durable acknowledgement, the result is
   `unknown_commit_result`; the controller must prove no automatic retry or second observation was
   started. A newly launched verifier then accepts exactly one full old or new revision, applies the
   corresponding guard, and rejects every other tree. A selected VM reboot/power-loss lane repeats
   the durability boundaries; process restart alone is not claimed as power-loss proof.
7. First-create races prove exactly one no-replace winner. Replacement races prove one serial winner
   per expected revision; the loser reloads a changed revision and rejects. Temp files, lock files,
   lock contents, pipe acknowledgements, projection files, caller claims, and alternate aggregate
   names are mutated independently to prove they carry no authority.
8. Each case snapshots the exact fixture tree before and after, verifies permissions/type,
   filesystem and mount identity, exact canonical bytes, monotonic revision, consumed observation,
   permanent unknown-turn high-water/no-replay state, and absence of out-of-root writes. Cleanup may
   remove only paths enumerated beneath the revalidated fixture root; failure preserves the fixture
   for review rather than broadening deletion.

The acceptance rule is strict: readers must observe exactly one full old or new canonical revision,
never missing, truncated, mixed, duplicated, substituted, or partially acknowledged authority. An
unknown result prohibits automatic retry even when a later reload shows the old revision; a new
reconciliation attempt requires separately authorized lifecycle semantics outside Slice 2.

**Rejected alternatives.** `os.Rename` is rejected as the cross-platform contract because Go
documents non-Unix non-atomic behavior. Windows `ReplaceFileW` is rejected because its documented
error cases can leave the replaced name absent or move both inputs, and its
`REPLACEFILE_WRITE_THROUGH` flag is unsupported. Directly opening the authoritative target with
`CREATE_NEW`/`O_EXCL` is rejected because partial first-revision bytes become visible before commit.
Create/delete marker locks are rejected because crash leaves stale existence and unlink/recreate can
split lock authority. Process-local mutexes do not cover other hosts/processes. File sync without
parent-directory sync proves content, not namespace durability. `fsync` without `F_FULLFSYNC` is
insufficient for the selected macOS durability claim. Network locks, generic “atomic file” packages,
SQLite, journaling/tombstone files, ordered multi-file writes, and volume-wide privileged flushes are
rejected for this slice because they either change the authority model, do not expose the exact
guarantees, or expand deployment/security scope. Cross-compilation and same-process tests are not
platform evidence.

**Exact unresolved maintainer choices.** Before Slice 2 can begin, a maintainer must explicitly:

- supply or identify normative primary documentation that closes the Windows/NTFS concurrent-reader
  atomicity gaps for first publication and replacement (properties 5 and 6) and the resulting
  restart/power-loss qualification gap (property 10); then accept an exact NTFS/version/host
  allowlist. Property 9's write-through rename-metadata candidate and property 12's containment and
  identity checks are documentation-supported only, not primitive, allowlist, or implementation
  acceptance;
- supply or identify normative primary documentation that closes the macOS/APFS descriptor-relative
  exclusive-rename and replacement atomicity, file and parent-directory durability, complete
  mount-containment, and exact APFS-feature/macOS-build/host-eligibility gaps; documented APFS
  version-2 fields, `arm64` availability, or `F_FULLFSYNC` success alone do not close those gaps;
- accept Linux 5.8 or later, ext4 on one local non-removable mount, and the exact runtime
  filesystem/mount-identification deny policy as the sole documentation-qualified candidate; every
  network, FUSE, overlay, tmpfs, removable, cross-mount, bind-mounted, or unknown filesystem remains
  unsupported by default;
- after all three platform documentation gates pass, select and separately review the exact
  `golang.org/x/sys` version needed to expose the accepted primitives. The currently resolved
  indirect `v0.28.0` is evidence of API availability only and is not approved for promotion; no CGO
  or new third-party library is implied;
- accept exact lock-artifact location, lifetime, permissions/ACLs, inheritance rules, timeout and
  cancellation behavior, and the requirement that cleanup never deletes a live/stale lock inode;
- accept the exact known-failure versus unknown-result error mapping for each syscall and the
  restart/operator response, with no automatic retry, repair, replay, or legacy fallback;
- select the controlled native hosts/filesystems and VM reboot/power-loss method for executable
  durability evidence, including retained evidence artifacts and review criteria; and
- separately authorize Slice 2 implementation and its harness after this packet is accepted.

Until those choices are explicit, every unsupported or ambiguous platform guarantee fails closed,
the legacy guard remains authoritative, and no observation, aggregate creation/replacement,
projection, decision, claim, prompt, fallback, retry, or replay is authorized. Classifier,
recovery-only operation, claims, bindings, responses, fallback, adapter pinning, guards, controls,
rollback, retry, migration, and permanent no-replay behavior remain unchanged.

**Open implementation gates and maintainer choices.** These facts are intentionally unresolved and
must not be silently selected to simplify implementation:

- Slice 1 already selected and implemented the unused package-private aggregate directory/name,
  schema/version syntax, canonical encoding, bounds, revision origin, digest syntax and SHA-256 path
  derivation, and inert package-state path. No production reader or writer uses them and no aggregate
  has been written. Schema evolution, permissions hardening, corrupt-record/operator recovery,
  platform storage primitives, migration, cutover, projection, and later lifecycle activation remain
  unresolved;
- the Windows normative documentation still needed for concurrent-reader atomic first publication
  and replacement (properties 5 and 6) and resulting restart/power-loss qualification (property 10),
  plus maintainer acceptance of an exact NTFS/version/host allowlist. Property 9's write-through
  rename-metadata candidate and property 12's containment and identity checks are documentation-
  supported only. The macOS normative documentation needed for properties 5-10, 12, and the exact
  APFS-feature/macOS-build/host eligibility in 13 also remains open, followed by maintainer acceptance
  of one all-platform primitive/library and supported host/filesystem/version matrix. Linux's
  documentation-qualified candidate does not permit Linux-only implementation or prototype work;
  network/removable/virtual filesystems remain unsupported
  until separately proven;
- the trusted way provider-pool obtains and byte-identifies Pipeon's canonical VS Code workspace
  adapter/guard slice without accepting display state or a caller digest as authority;
- the exact private requester surface and authentication/authorization boundary; CLI, MCP, other
  consumers, and administrative repair are not implicitly approved;
- mixed-version rollout and downgrade behavior that guarantees old readers/writers cannot mutate or
  trust frozen legacy records after aggregate cutover;
- retention duration, audit access, permissions, and eventual cleanup for frozen binding/state/claim
  files, the legacy Pipeon slice, temporary files, and transaction-lock artifacts;
- restart/operator UX for malformed aggregates and unknown commit results, including how a blocked
  session is inspected without providing retry, replay, repair, or legacy-fallback authority;
- exact explicit-user-decision wording, UI, local-authenticity proof, expiry/cancellation semantics,
  durable decision identity, and ambiguous-delivery handling;
- projection versioning, stale-revision display, and Pipeon persistence/query failure UX; projection
  repair can improve display only and cannot change lifecycle authority;
- rollback after authority cutover. Reverting to legacy authority is prohibited by the accepted
  direction, so any supported software rollback/minimum-version mechanism requires an explicit
  compatibility and maintainer decision; and
- final platform evidence acceptance, implementation evidence acceptance, and any later cleanup,
  operations, CAS-15 consumer, or public surface remain separate maintainer gates.

Unresolved gates keep the session blocked. They do not weaken, redesign, or defer the accepted
single-owner aggregate, exact compare-and-commit, projection-only Pipeon, explicit user decision,
strictly later fresh-turn, or permanent no-replay requirements.


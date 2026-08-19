##### Pre-Slice-2 storage-primitive decision packet — 2026-08-04

**Decision status and boundary.** The maintainer research policy below and the matrix's current
negative documentation result are accepted on 2026-08-04. Acceptance covers only the recorded
`D`/`R`/`U` documentation findings. No storage primitive, API sequence, dependency version, platform
allowlist, implementation, or evidence harness is accepted or authorized. This packet adds
documentation only: no prototype, storage code, lock, temporary file, aggregate, process helper,
generated artifact, or production call site was created. Slice 2 has not started; TASK-013 and
CAS-14 remain open. Provider-pool remains the sole future transaction owner, and
`reconciled_outcome_unknown` remains non-terminal and non-authorizing. The unknown pending turn is
permanently replay-forbidden. Package/engine and provider-neutral boundaries are unchanged.

**Accepted storage-research policy.** These decisions govern qualification and are not primitive
acceptance or implementation authority:

1. No production Slice 2 implementation may begin until Windows, Linux, and macOS all qualify.
2. The complete documentation matrix for all three platforms must be reviewed before any prototype.
3. Published vendor/system documentation is normative; official source may only corroborate it.
4. Any undocumented required guarantee blocks the feature; the contract is neither weakened nor emulated.
5. Initial tuples are Windows/local fixed-disk NTFS/`amd64`, Linux/local fixed-disk ext4/`amd64`,
   and macOS/local fixed-disk APFS/`arm64`.
6. Minimum versions derive from documented primitives and later native evidence. Older hosts may run
   DockPipe, but aggregate cutover must fail closed there.
7. Later implementation may use only the Go standard library plus the newest compatible, reviewed,
   exactly pinned `golang.org/x/sys`; no CGO or portability wrapper is authorized.
8. Each session has one deterministic persistent empty lock file. It is immutable,
   non-authoritative, and never substitutes for the validated live OS-held lock.
9. A lock is never deleted, broken, or replaced as stale.
10. Acquisition uses native nonblocking attempts for at most 30 seconds; caller cancellation or a
    shorter context may stop it, but no caller may extend the cap.
11. Every symlink, junction/reparse point, bind mount, nested mount, and cross-volume component in
    the complete transaction path must be rejected.
12. Authority storage is private: Unix directories `0700` and files `0600`; Windows DACL access is
    limited to the current user and `SYSTEM`; broader write access fails closed.
13. Runtime support uses a versioned package-owned evidence allowlist for OS, architecture,
    filesystem version, and relevant mount/volume properties. Local configuration cannot override it.
14. Commit success requires complete temp write, temp sync, atomic no-replace create or replacement,
    visible aggregate reopen and identity verification, visible-file sync, parent-directory entry
    sync, and exact canonical reload.
15. Publication errors use a conservative documentation-plus-evidence allowlist. Once publication is
    invoked, any outcome not positively proven unchanged is `unknown_commit_result`.
16. Restart classifies an unknown result read-only: exact new revision is committed, exact old is not
    committed, and missing/malformed/substituted/unexpected state is blocked. It never retries,
    re-observes, repairs, or mutates.
17. Missing or malformed authoritative aggregates block with read-only diagnostics only; there is no
    reconstruction, deletion, rollback, legacy fallback, or automatic repair.
18. Stale temporary files remain non-authoritative and untouched; cleanup is a later decision.
19. Aggregate cutover creates a one-way minimum-version boundary. Old binaries cannot operate on
    migrated sessions, and downgrade never restores legacy authority.
20. Future prototype/evidence code must be package-owned, platform-specific, build-tagged,
    test-only, and unreachable from production.
21. Evidence uses deterministic synthetic aggregates only.
22. Git retains only bounded summaries, environment tuples, counts, results, and hashes; raw logs,
    VM images, crash dumps, and generated artifacts remain outside Git.
23. Every accepted tuple requires every deterministic failure hook, 10,000 publication/reader race
    cycles, 1,000 lock/forced-termination cycles, and three controlled VM hard-reboot or power-loss
    trials at every durability boundary.
24. Final acceptance is all-or-nothing: every selected tuple must pass every property before Slice 2
    production implementation can be authorized.

**Go surface and version policy.** The standard library is insufficient: [`os.Rename`](https://pkg.go.dev/os#Rename)
explicitly disclaims atomic rename on non-Unix platforms, while [`File.Sync`](https://pkg.go.dev/os#File.Sync)
does not provide locking, no-replace publication, parent-entry sync, containment, identity, or
filesystem qualification. The checkout currently pins `golang.org/x/sys v0.28.0` indirectly. Its
[`windows`](https://pkg.go.dev/golang.org/x/sys@v0.28.0/windows) and
[`unix`](https://pkg.go.dev/golang.org/x/sys@v0.28.0/unix) packages expose the syscall entry points
listed below, but exposure supplies no semantic guarantee. No exact future version is selected here;
after documentation and evidence gates pass, the newest compatible release must be reviewed and
made an exact direct requirement. Platform-specific package code is still required for handles,
flags, retries, identity, ACL/mode checks, publication classification, and runtime allowlisting.

**Candidate platform profiles and normative documentation.** These are research results, not
implementation approval. Status words below mean documented and evidence-eligible, unsupported by
an explicit published limitation, or unresolved after the listed primary surfaces were exhausted.

- **Windows / local fixed-disk NTFS / `amd64`: not documentation-qualified.** Retain every directory
  handle opened by [`CreateFileW`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew)
  with `FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT`; reject every
  `FILE_ATTRIBUTE_REPARSE_POINT`; deny `FILE_SHARE_DELETE`; and compare `FILE_ID_INFO` volume/file
  identity. Microsoft documents that file ID plus volume serial identifies an open file on one
  computer in [`FILE_ID_INFO`](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_id_info),
  while [`GetVolumeInformationByHandleW`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumeinformationbyhandlew),
  [`GetDriveTypeW`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdrivetypew),
  and [`FSCTL_GET_NTFS_VOLUME_DATA`](https://learn.microsoft.com/en-us/windows/win32/api/winioctl/ni-winioctl-fsctl_get_ntfs_volume_data)
  expose NTFS, fixed-media, and NTFS major/minor-version facts. The private DACL must contain only the
  current-user and `SYSTEM` allows; Microsoft documents DACL access decisions and implicit denial in
  [DACLs and ACEs](https://learn.microsoft.com/en-us/windows/win32/secauthz/dacls-and-aces).
  Open the empty lock with `OPEN_ALWAYS`, non-inheritable handle, no delete sharing; use
  `LockFileEx(LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY)` over `[0,1)` and bounded polling.
  [`LockFileEx`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
  documents exclusive byte-range exclusion, including beyond EOF, but mapped access ignores it;
  [`UnlockFileEx`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-unlockfileex)
  and [process termination](https://learn.microsoft.com/en-us/windows/win32/procthread/terminating-a-process)
  document handle/lock release.

  Create the sibling temp with `CREATE_NEW | FILE_FLAG_WRITE_THROUGH`, write all bytes, and call
  [`FlushFileBuffers`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers).
  Use that same write-through source handle for
  [`SetFileInformationByHandle(FileRenameInfoEx)`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle):
  flags `0` for first publication and `FILE_RENAME_FLAG_REPLACE_IF_EXISTS |
  FILE_RENAME_FLAG_POSIX_SEMANTICS` for replacement. [`CreateFileW`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew#caching-behavior)
  explicitly says a write-through request causes NTFS to flush metadata changes such as a rename,
  and [File Caching](https://learn.microsoft.com/en-us/windows/win32/fileio/file-caching) says a file
  flush or `FILE_FLAG_WRITE_THROUGH` stores file-system metadata changes to disk. Together with the
  documented ordinary file/directory access requirements for rename, this qualifies property 9
  without a directory or administrative volume flush. A retained directory handle remains an
  identity/containment anchor only; neither `FlushFileBuffers` nor the driver flush documentation
  promises parent-entry durability for a directory handle. The documented volume-wide flush requires
  administrative privileges and remains rejected.

  [`FileRenameInformationEx`](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/4217551b-d2c0-42cb-9dc1-69a716cf6d0c)
  requires flags `0` to fail if the target exists and says POSIX replacement leaves old handles valid
  while subsequent opens of the target name open the renamed file. The WDK
  [`FILE_RENAME_INFORMATION`](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information)
  contract also requires same-volume rename and describes same-directory naming. The normative
  [`FileRenameInformation`](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fsa/87f86c9b-6c2a-4803-84b7-131a74a434fa)
  algorithm removes and adds directory links, but none of these sources states that first publication
  or replacement is atomic to concurrent readers, excludes a transient missing target, or guarantees
  one complete old-or-new revision for an open racing the operation. They also do not define a local
  NTFS post-failure state for every error. Therefore properties 5 and 6 remain unresolved, and every
  non-allowlisted outcome after publication is invoked remains `unknown_commit_result`.

  The WDK [`FILE_INFORMATION_CLASS`](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/ne-wdm-_file_information_class)
  page documents `FileRenameInformationEx` from Windows 10 version 1709. The public Win32 enum lists
  `FileRenameInfoEx` without a per-value minimum and `SetFileInformationByHandle` warns that information
  classes can vary by OS release. Candidate minimum therefore remains Windows 10 version 1709, not an
  earlier inferred floor. Microsoft exposes NTFS major/minor values but does not bind these rename and
  durability guarantees to an exact NTFS format version; no Windows/NTFS version tuple is accepted.
  `ReplaceFileW` remains rejected because `REPLACEFILE_WRITE_THROUGH` is unsupported and documented
  failures can remove or move names.
- **Linux / local fixed-disk ext4 / `amd64`: documentation-qualified for later native evidence only.**
  Retain one aggregate-directory fd and use
  [`openat2`](https://man7.org/linux/man-pages/man2/openat2.2.html) with
  `RESOLVE_BENEATH | RESOLVE_NO_SYMLINKS | RESOLVE_NO_MAGICLINKS | RESOLVE_NO_XDEV`; the last flag
  explicitly rejects every mount crossing, including bind mounts. Open the persistent lock with
  `O_RDWR | O_CREAT | O_CLOEXEC | O_NOFOLLOW`, `0600`, and poll
  [`flock(LOCK_EX | LOCK_NB)`](https://man7.org/linux/man-pages/man2/flock.2.html); it is advisory,
  scoped to the open file description, and released only by `LOCK_UN` or last close. Create a sibling
  temp with `O_WRONLY | O_CREAT | O_EXCL | O_CLOEXEC | O_NOFOLLOW`, `0600`; the
  [`open(2)`](https://man7.org/linux/man-pages/man2/open.2.html) contract documents exclusive create
  and final-component no-follow behavior. Write exact bytes, `fsync` the temp, publish with
  [`renameat2`](https://man7.org/linux/man-pages/man2/renameat2.2.html) using `RENAME_NOREPLACE` for
  revision one and flags `0` thereafter, reopen/verify/sync the visible inode, then `fsync` the parent
  dirfd. Linux documents atomic replacement, no-replace (ext4 since Linux 3.15), same-mount limits,
  target preservation on replacement failure, and explicitly states in
  [`fsync(2)`](https://man7.org/linux/man-pages/man2/fsync.2.html) that file sync excludes the directory
  entry and a directory `fsync` is also required; it also states successful sync survives crash or
  reboot and flushes a present disk cache. `statx(STATX_MNT_ID)` (since Linux 5.8), `fstatfs`, and
  `/proc/self/mountinfo` bind exact file, mount, ext4 type, and allowlisted read-write/journal/barrier
  properties; see [`statx(2)`](https://man7.org/linux/man-pages/man2/statx.2.html), the kernel
  [`mountinfo`](https://www.kernel.org/doc/html/latest/filesystems/proc.html#proc-pid-mountinfo-information-about-mounts)
  format, and ext4 [journal](https://www.kernel.org/doc/html/latest/filesystems/ext4/journal.html)
  documentation. Candidate minimum is Linux 5.8 because mount identity is required; the exact kernel
  build, ext4 feature set, and explicit mount-property allowlist still require native evidence and
  maintainer acceptance.
- **macOS / local fixed-disk APFS / `arm64`: not documentation-qualified.** Retain a descriptor walk
  using `openat` with `O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC`; use `fstat`/`fstatfs` to require APFS,
  `MNT_LOCAL`, one retained filesystem ID, and accepted mount flags. Apple documents final-component
  no-follow and exclusive create in
  [`open(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/open.2.html),
  advisory whole-file exclusion in
  [`flock(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/flock.2.html),
  and filesystem ID, type, mount-point, mounted-from, and flag fields in
  [`statfs(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/statfs.2.html).
  Those surfaces support properties 1-4 only; `statfs` names `f_fsid` as a filesystem ID but does not
  make the complete descriptor walk a race-free containment or same-volume proof.

  Open the persistent lock with `O_RDWR | O_CREAT | O_CLOEXEC | O_NOFOLLOW`, `0600`, and poll
  `flock(LOCK_EX | LOCK_NB)`. Create the sibling temp with `O_EXCL` and write exact bytes. The APFS
  [Tools and APIs guide](https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/APFS_Guide/ToolsandAPIs/ToolsandAPIs.html)
  labels `renamex_np` and `renameatx_np` as safe-save APIs but publishes only their prototypes.
  Foundation's
  [`volumeSupportsExclusiveRenaming`](https://developer.apple.com/documentation/foundation/urlresourcevalues/volumesupportsexclusiverenaming)
  says that a true value means support for `RENAME_EXCL` on path-based `renamex_np` and describes a
  pre-existing-destination warning. General
  [`rename(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/rename.2.html)
  requires one filesystem and says an instance of the new name always exists even across a crash;
  [About Apple File System](https://developer.apple.com/documentation/foundation/about-apple-file-system)
  also advertises atomic safe-save as an APFS feature. None of those contracts defines
  `renameatx_np` flags or errors, binds `RENAME_EXCL` to the descriptor-relative call, establishes an
  atomic no-overwrite first publication, or establishes flags-`0` replacement in which racing readers
  observe exactly one complete old or new revision without a transient missing target. Properties 5
  and 6 therefore remain unresolved.

  Request `fcntl(F_FULLFSYNC)` before publication and after reopening the visible file only as an
  unresolved candidate. Apple's
  [`fsync(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fsync.2.html)
  says ordinary `fsync` permits data loss and write reordering after power loss or an OS crash. Its
  archived
  [`fcntl(2)`](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fcntl.2.html)
  says `F_FULLFSYNC` asks the drive to flush buffered data but lists only HFS, FAT, and UDF as
  implemented filesystems. Current Apple
  [disk-write guidance](https://developer.apple.com/documentation/xcode/reducing-disk-writes) calls
  `F_FULLFSYNC` a strong expectation and an iOS best-effort guarantee that can still lose data on
  sudden power loss; it does not publish an APFS/macOS persistence contract. No unprivileged
  parent-directory entry-sync primitive is documented. APFS copy-on-write crash-protection statements
  in Apple's
  [FAQ](https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/APFS_Guide/FAQ/FAQ.html)
  do not bind this exact file/rename/directory sequence. Properties 7-10 remain unresolved.

  Foundation publishes
  [`isMountTrigger`](https://developer.apple.com/documentation/foundation/urlresourcevalues/ismounttrigger)
  and
  [`volumeIdentifier`](https://developer.apple.com/documentation/foundation/urlresourcevalues/volumeidentifier)
  detection values, but neither those values nor `O_NOFOLLOW`/`fstatfs` rejects every symlink, mount
  substitution, same-filesystem nested mount, and cross-volume movement throughout the retained walk;
  property 12 remains unresolved. The
  [Apple File System Reference](https://developer.apple.com/support/downloads/Apple-File-System-Reference.pdf)
  documents APFS version 2 as implemented in macOS 10.13, backward-incompatible feature flags, and a
  reserved `nx_newest_mounted_version` field recording the newest Apple software to mount a container.
  Apple also documents APFS support from macOS 10.13 in the
  [Disk Utility guide](https://support.apple.com/en-ca/guide/disk-utility/dsku19ed921c/mac), while
  [Apple-silicon porting guidance](https://developer.apple.com/documentation/apple-silicon/porting-your-macos-apps-to-apple-silicon)
  documents native macOS `arm64` in the macOS 11 porting context. These are format and host
  facts, not an unprivileged mounted-volume API or compatibility contract mapping an exact APFS
  feature set, macOS build, and hardware to the required rename, synchronization, containment, and
  power-loss guarantees. Property 13 and an exact allowlist floor remain unresolved.


#### Linux VM reviewed source-build offline-promotion contract (2026-08-08)

The missing boundary between deterministic source-build evidence and Gate 2 inputs is now a
separately authorized offline promotion. This decision records the already reviewed source root
`/tmp/dockpipe-vm-source-review.ZStX82CE` without re-reading it. The reviewed repository checkpoint
is `15f0ea3f027877221b78f637a00ab010d0a8be1d`; both Linux/amd64 build pairs were byte-identical;
the controller is `5447054` bytes with SHA-256
`b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc`; the guest agent is
`3870038` bytes with SHA-256
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`; and the Windows/amd64
guest-agent compatibility output has SHA-256
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`. The builds used Go
1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOARCH=amd64`, `-trimpath`, and
`-buildvcs=false`, and all non-standard build inputs resolve below `packages/vm/tools/**`. The
Windows artifact is compatibility evidence only and is never promoted for Linux Gate 2.

The fixed non-live namespace is `/home/jamie/.local/share/dockpipe-vm-gates`. It is task-owned VM
qualification input, not the checkout, DockPipe global package/install root, `.dockpipe`,
`.dorkpipe`, an image/toolchain cache, a live instance/evidence/configuration/runtime XDG root, or
any preserved Gate 2 root. Promotion IDs match `vmp-[0-9a-f]{16}`. Each future gate must provide
the exact ID and all source and destination paths before execution and may not discover, substitute,
increment, or fall back to another destination. The proposed first promotion is
`vmp-2026080815f0ea3f`, with exact documentation-only paths:

- promotion root:
  `/home/jamie/.local/share/dockpipe-vm-gates/promotions/vmp-2026080815f0ea3f`;
- evidence directory:
  `/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f`; and
- evidence file:
  `/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f/promotion.evidence.json`.

The promotion root is exclusively created mode `0700`, owned by the effective promotion user and
that user's primary group. Its closed inventory is exactly two regular, non-symlink, single-link,
mode-`0500` files with that same owner and group: `dockpipe-qemu-controller`, copied from the
reviewed `linux-a` controller output with the size/hash above, and `dockpipe-guest-agent`, copied
from the reviewed `linux-a` guest output with the size/hash above. Both are static Linux/amd64 ELF
executables, created exclusively without following links, immutable by lifecycle after success,
and read-only inputs to later Gate 2 work. No directory, manifest, evidence, Windows binary,
temporary file, cache, log, key, authorization, or other artifact may remain inside the promotion
root. The corresponding `linux-b` outputs must be compared byte-for-byte against `linux-a`
immediately before promotion; they are comparison inputs only and are not copied.

Before publishing, the gate verifies every fixed namespace component is an owned non-symlink
directory and rejects checkout, generated-store, global-install, live-XDG, preserved-root,
source-review-root, and mutual overlap. Missing namespace directories are created one component at
a time as mode `0700`. The exact promotion root and evidence directory are exclusively created;
either task-owned destination already existing fails closed. No fallback, alternate lane,
overwrite, rename-over-existing, retry, repair, replacement, or automatic cleanup exists. Every
created directory and file is owned by the effective promotion user and that user's primary group.

The exact durability sequence is:

1. revalidate both Linux pairs, hashes, sizes, types, modes, ownership, link counts, and embedded Go
   build metadata;
2. exclusively create the mode-`0700` promotion root and synchronize its parent directory after
   publishing the new directory entry;
3. exclusively create each destination without following links as mode `0500`, copy exact
   `linux-a` bytes, synchronize the file, close it, reopen without following links, and verify its
   identity, type, size, ownership, mode, link count, SHA-256, and embedded Go metadata;
4. verify the promotion root's exact closed inventory and synchronize that directory;
5. exclusively create the mode-`0700` evidence directory and synchronize its parent;
6. exclusively create mode-`0600` `promotion.evidence.json`, synchronize it, reopen and verify it,
   then synchronize the evidence directory; and
7. immediately read back both roots and report the evidence-file SHA-256.

Any failure stops once and preserves the exact partial promotion and evidence roots. It performs no
retry, replacement, repair, fallback, or cleanup. A failed promotion ID is prohibited from reuse;
inspection or cleanup is another separately authorized exact offline gate.

Canonical evidence uses schema `dockpipe.vm.offline-promotion.v1`, stable snake-case field names,
and ordered artifact entries. It records `schema`, `promotion_id`, `repository_checkpoint`,
`source_review_root`, exact `linux_a_sources` and `linux_b_comparison_sources`, successful
`byte_comparisons`, `build_provenance` (`go_version`, `gowork`, `cgo_enabled`, `goos`, `goarch`,
`goamd64`, `trimpath`, and `buildvcs`), `promotion_root`, `evidence_path`, `effective_uid`,
`effective_gid`, `promotion_root_mode`, `evidence_directory_mode`, and ordered two-entry
`promoted_inventory`. Each inventory entry contains `relative_name`, `absolute_path`,
`source_path`, `sha256`, `byte_size`, `file_type`, `mode`, `uid`, `gid`, `link_count`,
`go_package_path`, and `go_build_settings`. The remaining stable fields are
`windows_amd64_compatibility` with `promoted: false`, `file_syncs`, `directory_syncs`,
`closed_inventory`, `package_engine_boundary`, and `actions_performed`.

`actions_performed` explicitly records `false` for identity preparation, live input, plan,
authorization, disk, seed, socket, QEMU process, Gate 2, cleanup, and Gate 3. Evidence never contains
secrets, private keys, live authorization material, or preserved-root contents. The promotion gate
records the exact embedded `GOAMD64`, Go package paths, and build settings from immediate
revalidation rather than assuming or substituting them.

A successful promotion is immutable task-owned input for a later separately authorized Gate 2
preparation/execution chain. Gate 2 references it read-only; it does not consume, mutate, silently
replace, automatically expire, delete, or include it in qualification cleanup. Removal requires a
separately authorized exact offline-promotion cleanup gate. A later preparation gate, not this
decision, binds the promoted paths into a task-owned provisioning input outside the checkout. The
machine-specific path is intentionally absent from the checked-in provisioning template.

This docs-only gate created no external root, promotion output, or promotion evidence and granted no
Gate 2 authority. It did not inspect the source-review root or any preserved Gate 2 root. It did not
prepare identity or live input, emit a plan or authorization, create a disk, seed, socket, or QEMU
process, execute Gate 2, perform cleanup, or begin Gate 3. Deterministic source-build review, offline
promotion, Gate 2 preparation, Gate 2 live authorization/execution, cleanup, and Gate 3 remain
separate gates. The package/engine boundary remains preserved with no `src/**` change.

#### Linux VM promotion, fresh Gate 2 evidence, and deadline correction (2026-08-08)

The separately authorized promotion `vmp-2026080815f0ea3f` completed once under
`/home/jamie/.local/share/dockpipe-vm-gates`. Its immutable closed inventory contains only the
mode-`0500` Linux/amd64 controller and guest agent with the reviewed hashes above. Canonical evidence
is stored at
`/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f/promotion.evidence.json`
with SHA-256 `c411a6cfa326d61c6bfd9663a7f063d21dcb364520c2274fb3fe34d1f951889b`.

Fresh offline preparation then produced run `g2r-e58b5061e0e69e7e`, cohort
`g2c-b725086f6d664d7d`, provisioning contract SHA-256
`a47cd2f0f9cac67770add46fdf687b67bfc75301f75c6e143d6e45e630d95ce3`, and plan SHA-256
`8f2bbc6418315248fd15220cfd7998dabb2e453dd087cbe5460e9aaba7ac53c5`. One exact live
authorization ran once and is permanently spent. Identity staging was consumed only after durable
reservation, and all four fresh owner-only XDG roots were created. The controller invocation lost
supervision while its exact QEMU child remained active, so no retry, signal, reconnect, fallback,
cleanup, or second live authorization occurred. A sandbox PID-namespace read initially and
incorrectly appeared to show that PID absent; the later escalated host read proved the original QEMU
still active with exact recorded PID `1884350`, start ticks `7105843`, executable SHA-256
`3544680aaeaf8087bbf3ef693ff185c2691831560c767672defccd784ec37140`, and sealed command SHA-256
`86ef04336070f9645355193318a64f368ba7752fa68790b5e5db7a974f6af6d8`. Cleanup correctly refused
the active process without deleting any resource.

The owner-only `first-boot-console.log` captured 58,824 bytes. It proves the exact pinned Ubuntu
guest reached `cloud-init-local.service`, `network-pre.target`, `systemd-networkd.service`, and
`network.target`, then remained in `systemd-networkd-wait-online.service` through the end of capture.
An offline read-only extraction of the exact pinned source image proved that
`systemd-networkd-wait-online.service` is enabled under `network-online.target`, invokes
`systemd-networkd-wait-online` with its documented 120-second default, and is an explicit
predecessor of `cloud-init.service`. NoCloud installs the signed guest agent only after this boundary.
The prior 60-second guest-verification deadline therefore could not observe a bootstrap on the
reviewed networkless `-nic none` boot path.

VM package 1.1.1 corrects only that sealed deadline: guest verification is now 180 seconds. Clone,
QEMU launch, and QMP shutdown remain 120 seconds; networking and SSH remain disabled; complete
failure preservation remains required; and retry, cleanup, and fallback signals remain prohibited.
The timeout remains part of provisioning-v3 and the deterministic plan digest; executor-v4 owns the
new 180-second contract. Preserved executor-v3 and executor-v2 contracts remain loadable only for
their separately authorized exact cleanup lists, and all old inputs fail closed for fresh execution.
Offline tests must pass before new deterministic builds.
The current promotion is immutable historical input; a fresh source-build review, new promotion ID,
fresh run/cohort and identities, new preparation, and a separately authorized live Gate 2 invocation
are required. A separately authorized recovery then negotiated only QMP capabilities and sent
`system_powerdown`; the exact recorded QEMU exited within the 120-second bound and no fallback signal
was sent. A final fresh cleanup authorization bound to executor SHA-256
`adcd4b1e4ea2bcd48078d0545a67699702b00dc344ca79eeeb9e49adefab0926` completed once. Immediate
host read-back confirmed all ordered 12 resources absent. The cleanup did not touch immutable
promotion `vmp-2026080815f0ea3f`, the post-correction source-review root, the checkout, or concurrent
task docs. Gate 2 is not yet qualified and Gate 3 remains blocked.

The post-correction offline source-build review used private root
`/tmp/dockpipe-vm-source-review.2ikAeuDJ` and exact repository checkpoint
`f6d5c19c24613945f5cbcf190aca50725ab51fdf`. Two independent Linux/amd64 lanes
used separate caches and temporary directories with Go 1.25.0, `GOWORK=off`, `CGO_ENABLED=0`,
`GOAMD64=v1`, `-trimpath`, and `-buildvcs=false`. Their controller outputs are byte-identical at
`5447246` bytes and SHA-256
`564d57937bef2856777dc3a3d05a57649e8918a0572f9f7f4d758308e9a7089c`; their guest-agent outputs
are byte-identical at `3870038` bytes and SHA-256
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`. The Windows/amd64
compatibility output is
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e` and is not a Linux
promotion input. Go dependency closure inspection found only the standard library and packages under
`packages/vm/tools/**`. This gate created no live identity, input, plan, authorization, XDG root,
disk, seed, socket, process, cleanup, Gate 2, or Gate 3 action. The builds remain offline review
artifacts until a separately authorized fresh promotion.

The separately authorized immutable promotion `vmp-20260808f6d5c19c` then completed once. Its exact
mode-`0700` root contains only the mode-`0500` controller and guest-agent files with the hashes and
sizes above. Canonical evidence is
`/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-20260808f6d5c19c/promotion.evidence.json`
with SHA-256 `71827ec3cb32d35b92773b74fc0e0e2a68f0ba223341811c5e9da6b2de0f271d`.
Immediate read-back revalidated the independent build comparisons, embedded Go metadata, closed
inventories, owner-only modes, file and directory synchronization boundaries, and package/engine
separation. The earlier promotion remains untouched. This gate created no identity material, live
input, plan, authorization, disk, seed, socket, QEMU process, cleanup, Gate 2, or Gate 3 action.
Fresh Gate 2 preparation remains separately authorized.

The separately authorized offline preparation then created fresh task root
`/tmp/dockpipe-vm-gate2-prep-20260808-d8fb7b9e`, run `g2r-1f9bdb5dd11545a4`, and cohort
`g2c-17706e2c6519c7b0`. It generated a new 24-hour identity bundle, qualification input SHA-256
`c1583eff7db0049a6fb7692d36b153bbe285801232d26ed4b91c5e4df6965ab3`, provisioning input
SHA-256 `460b759b050a68a500aa6ea4e2c2e503ba2317d955e4e8b2d47d6eb6b93b39ec`, contract SHA-256
`656c5bca0ae6d0f994ecdad799b4a4d58354b955396e547d29981c0980521f1c`, and inert plan SHA-256
`bb1670208553885674e698b86bd0fee103ccda4e4cadee0497420b7913c09edc`. The plan retains
`live_authorized=false`, `execute=false`, `authorization_required=true`, and the reviewed
180-second guest-verification deadline. QMP, agent, and console socket paths are respectively 93,
95, and 92 bytes, below Linux's 107-byte bound. All four exact live roots remain absent. No live
authorization, identity reservation, disk, seed, socket, process, Gate 2, cleanup, or Gate 3 action
occurred. Live Gate 2 remains a separate exact authorization.

#### Linux VM Gate 2 NoCloud and virtio correction (2026-08-08)

The exact live authorization for run `g2r-1f9bdb5dd11545a4` and cohort
`g2c-17706e2c6519c7b0` executed once and is permanently spent. Guest
verification failed closed after the complete 180-second executor-v4 deadline;
no signed bootstrap, verification evidence, controller request, retry, signal,
fallback, or cleanup occurred. All task roots remain preserved. An escalated
host read proved the recorded QEMU process absent after failure. The owner-only
`first-boot-console.log` is `86335` bytes.

The captured console and an unprivileged read-only sparse copy of the preserved
OS overlay prove three independent causes. The pinned image spent about 120
seconds in `systemd-networkd-wait-online.service`, then started the agent only
at about 176.2 seconds. Cloud-init `write_files` failed with `Unknown user or
group: dockpipe-agent` because three agent-owned files were rendered before the
users module. Disk setup failed because `/dev/disk/by-id/virtio-g2data-63a5654952ec9a88`
did not exist: the requested serial is 23 characters while Linux exposes only
20 bytes for a virtio-blk serial. Cloud-init's pinned schema additionally
rejected the empty `packages`, `ssh_genkeytypes`, and `ssh_authorized_keys`
arrays, the user `create_home` field, and the misplaced user-data `network`
field. The forensic copy touched neither the preserved overlay nor any live
root.

VM package 1.1.2 corrects the package-owned contract. The three agent-owned
key/config files use cloud-init `defer: true`; the invalid fields are removed or
replaced while the separate NoCloud network-config still declares no Ethernet
interfaces. Both OS and data serial validation now fail closed above the
20-byte virtio-blk limit. Guest verification is 240 seconds: the observed
networkless 120-second wait plus the original 60-second post-agent verification
allowance. Executor-v5 owns the new policy. Preserved executor-v4, executor-v3,
and executor-v2 files remain cleanup-only with their respective 180/60/60-second
shapes, and compatibility tests pin each exact cleanup path. No `src/**` file
changed; the package/engine boundary remains preserved. The spent run still
requires separately authorized exact cleanup, and fresh builds, promotion,
preparation, and one new live authorization are required before Gate 2 can be
qualified. Gate 3 remains blocked.

The corrected source was then rebuilt twice from exact checkpoint
`9f6d406e9725acb73476bcde1617fc4fce87b700` under private root
`/tmp/dockpipe-vm-source-review.HIC5Ps`. Independent Linux/amd64 lanes produced
byte-identical controller outputs of `5447222` bytes and SHA-256
`c6c2ce8abebf9027af01fdde6a4ac8c487eb53124fbea5c2edeee7c538f5ad7b`,
and byte-identical guest-agent outputs of `3870222` bytes and SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`.
The Windows/amd64 guest-agent compatibility output is `3966976` bytes and
SHA-256 `86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`;
it is not a Linux promotion input. Builds used Go 1.25.0, `GOWORK=off`,
`CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and `-buildvcs=false`. Embedded
metadata matches those settings and every non-standard dependency resolves
inside `packages/vm/tools/**`. This was offline evidence only and created no
promotion, identity, authorization, live root, disk, socket, process, cleanup,
Gate 2, or Gate 3 action.

The spent executor-v4 run was then removed through its distinct exact cleanup
boundary. Cleanup wrapper SHA-256
`49d1d779a1a245c4974e23273a2cea8377fe81afec41b7207647805fb4087744`
used cleanup authorization SHA-256
`eee700a82b075564f4a9406101501292980fb62150314319e040e4f3158cdaf1`
once for execution SHA-256
`b59f443e2c5aea26dc8e798500aa7ec58d2c2415ffb017ebc77ce4077d4a0266`.
The controller returned `completed=["cleanup"]`, `cleanup_run=true`, and
`preserved=false`; cleanup-result SHA-256 is
`5e45ed67fc6866bc4715791e190371b76c334330c8495f262c6fa21ed0d5e0f0`.
Independent read-back confirmed all 12 executor-ordered resources absent and
the recorded QEMU PID remained absent. The spent Gate 2 run was not retried,
no fresh live action occurred, and executor-v5 source-review outputs remain
unpromoted. The next live chain still begins with a separately authorized
promotion and fresh preparation; Gate 2 is unqualified and Gate 3 remains
blocked.

The next separately authorized offline gate published executor-v5 promotion
`vmp-202608089f6d406e` from exact checkpoint
`9f6d406e9725acb73476bcde1617fc4fce87b700`. Its closed inventory contains
only the Linux controller SHA-256
`c6c2ce8abebf9027af01fdde6a4ac8c487eb53124fbea5c2edeee7c538f5ad7b`
at `5447222` bytes and Linux guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes. Both are mode `0500`, owner-only, single-link files.
Canonical promotion evidence SHA-256 is
`6ee9dfac5ed41d60cd4335d70284588c198ebdfbfd475a3f3be95c2cf08b8987`.
The Windows artifact remains compatibility-only and was not promoted.
Independent read-back confirmed the exact inventory, ownership, modes, sizes,
hashes, and package/engine boundary. The promotion performed no identity
preparation, live input, plan, authorization, disk, seed, socket, QEMU, cleanup,
Gate 2, or Gate 3 action. Fresh Gate 2 preparation remains a separate gate.

The first executor-v5 preparation attempt for run `g2r-4153130ffe9e0189`
and cohort `g2c-788fe545f4721de8` was executed once and is spent. It created the
owner-only identity bundle and qualification/provisioning inputs, then failed
in the wrapper's final offline assertion because an inert provisioning plan has
no top-level `execution` object. The timeout is bound in the provisioning input
and typed `verify-guest` operation; executor-v5 is sealed only at the later
authorized live boundary. The wrapper had not yet persisted the inert plan.
All four live roots remained absent, so no identity reservation, authorization,
disk, seed, socket, process, Gate 2, or Gate 3 action occurred. The invocation
was not retried.

A separately authorized closed-inventory cleanup then deleted the eight files
and exact partial task root. Cleanup result SHA-256 is
`6df37d51443afeb37b2c3272ee6563e0d7d596a7d2755a525be715686a668aa3`;
independent read-back confirmed the task root and all four live roots absent.
A corrected preparation must use a fresh run, cohort, and identity bundle.

Corrected preparation then completed once for fresh run
`g2r-ff1cc0a230d1f0c2`, cohort `g2c-1e7fe20cf8ac84ad`, and immutable promotion
`vmp-202608089f6d406e`. Qualification input SHA-256 is
`cc1eee4b2fb9cd258307205535f4218ddedd3b0b47ab9e4c2be6c801ecb9e805`,
provisioning input SHA-256 is
`99c715a35c339730c790f62ac23ec11f3e8327a11e791f57db2360b224fee297`,
contract SHA-256 is
`1b09bfb642a968237dd66bbe3693213ade7b1229bdec48569aee331f5b9aed34`,
and plan SHA-256 is
`39177e8612cb0b83e0f6960027268d3d34dc61c30568a13bffeda1d023ab7988`.
The identity expires at `2026-08-09T16:47:02Z`. The persisted plan remains
`live_authorized=false`, `execute=false`, and `authorization_required=true`;
its typed `verify-guest` operation binds 240 seconds. Both disk serials are 20
bytes, and QMP, agent, and first-boot-console socket paths are 93, 95, and 92
bytes. All four live roots remain absent. No authorization, identity
reservation, disk, seed, socket, process, cleanup, Gate 2, or Gate 3 action
occurred. Live Gate 2 remains a separate exact authorization.


# Linux VM workflow and qualification foundation

`linux-vm` is the first-party Linux workflow over the existing generic VM
runtime and the `qemu` resolver. Its runnable path is development-only. The
qualification fields are a typed contract for offline validation and are fixed
to `Qualification.Enabled: false` until a separate integration and live gate.

## Immutable Ubuntu profile

The reviewed profile is Ubuntu 24.04 LTS amd64 release stamp `20260801`:

- image: `ubuntu-24.04-server-cloudimg-amd64.img`
- SHA-256: `0533b0655c32e68b31d792ecd6ccfca95abdbc536c4446874fe0513bd4140ffe`
- provisioning source: local NoCloud only
- network and SSH: disabled for qualification
- `apt` update/upgrade: disabled; no additional debs are requested

No image is downloaded by a workflow or package test. A future download gate
must fetch the exact immutable URL into the XDG image cache, hash before use,
and reject a user override without its own checksum.

## Qualification contract

`manifests/linux-qualification.json` is an illustrative, deliberately
non-runnable manifest. Validation requires purpose `qualification`,
`disposable=true`, KVM (TCG is rejected), host CPU, two vCPUs, 4096 MiB, no
swap, exactly two private disks, and no network, SSH, passthrough, physical
disk, share, extra disk, or arbitrary command capability. Host and guest
machine identities must differ.

The manifest does not prescribe a boot UUID. Schema
`dockpipe.vm.qualification.v2` fixes only
`boot_id_source=/proc/sys/kernel/random/boot_id`; the actual per-boot value is
learned through the authenticated guest-first protocol below.

The data disk is a 4 GiB sparse raw whole-device ext4 filesystem. Both QEMU
virtio-blk serials are limited to 20 ASCII bytes so the Linux-visible stable ID
cannot be truncated. Its reviewed
creation tuple is:

```text
mkfs.ext4 -F -L dockpipe-qual -U <manifest-uuid> -E lazy_itable_init=0,lazy_journal_init=0 /dev/disk/by-id/virtio-<manifest-serial>
```

It mounts by UUID at `/var/lib/dockpipe-qualification` with exactly
`rw,noatime,nodev,nosuid,noexec,data=ordered`. Discard, DAX, `nobarrier`, bind,
remount, nested filesystems, overlays, encryption, lazy initialization, and
unexpected ext4 features are rejected. Only qualification tickets, SQLite
roots, and results belong there.

QEMU planning records the binary version and SHA-256, configuration SHA-256,
both host backing identities, node names, serials, filesystem identity, mkfs
version, host and guest kernel releases, ext4 features, and mount ID.
Both disks use persistent block nodes, `cache=none`, flush preservation,
guest write-cache exposure, disabled discard/zero detection, and stop-on-error.
Threaded AIO is the default; native AIO requires an explicitly reviewed tuple.
There is no silent fallback, and QEMU 6.2 is not claimed as qualified.

## Signed protocol and recovery

Each instance has new Ed25519 controller and guest keys with mutually pinned
public-key hashes. Frames use a four-byte big-endian length prefix, a 64 KiB
maximum frame, a 32 KiB maximum canonical-JSON payload, a five-minute maximum
lifetime, and 30 seconds of clock skew. Unknown or duplicate keys, extra data,
noncanonical JSON, oversized data, invalid signatures, stale windows, replayed
nonces, out-of-order sequences, and identity substitution are rejected.

Every signature covers protocol version, capability, machine UUID, data-disk
serial, boot ID, sequence, run ID, nonce, scenario, durability boundary, phase,
time window, and payload. Qualification capabilities are exactly:
`identity/v1`, `health/v1`, `checkpoint/v1`, `recovery/v1`, and
`launch-hash-pinned/v1`.

Protocol `dockpipe.vm.v2` has one exact bootstrap. The fresh provisioning-v3
`bootstrap_nonce` is generated with the per-instance Ed25519 keys, bound into
the provisioning contract, inert plan, short-lived live authorization,
NoCloud guest configuration, and sealed current executor digest. Once the guest has
read the reviewed kernel boot-ID path, it writes the first length-prefixed frame
before reading controller bytes. That canonical frame has kind `bootstrap`,
capability `identity/v1`, sequence 1, phase `bootstrap`, the launch bootstrap
nonce, actual boot ID, all static identity fields, and a payload containing the
boot-ID source plus both public-key and binary SHA-256 pins. It is signed only
by the pinned guest key; there is no unsigned identity or controller bootstrap
exception.

The controller must read first and verify framing, canonical JSON, freshness,
the pinned guest signature, exact bootstrap nonce, sequence/phase, static
machine/disk/run/scenario/boundary context, boot-ID UUID, boot-ID source, and all
four pins. It must then exclusively create mode-`0600` `bootstrap.json`, record
the verified frame and learned boot ID, and fsync the file and evidence
directory before writing any request. Existing evidence or any verification or
durability failure stops with preservation and no retry. The first
controller-signed `identity/v1` request uses that boot ID at sequence 2 with a
new nonce; later requests are contiguous and may not reuse the bootstrap or any
request nonce. Guest-signed results echo each request context. This second step
proves the guest's controller-key pin while retaining mutual Ed25519 pinning.

A pending signed ticket blocks all work other than the exact matching recovery.
Machine, disk, boot, run, scenario, boundary, nonce, and harness hash must all
match. The guest durably records `consumed` and the result hash before returning
one result. It never resends, retries, repairs, reopens, or resumes a consumed
ticket; a lost result fails the cohort. Stale/consumed cleanup is explicit and
outside qualification.

## Trial isolation and future gates

One VM is reserved per cohort, with a private OS clone and data disk. Run,
cohort, boundary, and attempt roots are unique and immutable; database,
journal, nonce, and ticket material are never reused. Recovery is sequential.
Any failure preserves the complete instance. Cleanup is a separate production
operation and remains unauthorized until a fresh file matches the exact run,
contract, plan, executor digest, and ordered resource enumeration.

The six systemd and NoCloud files under `assets/` are now exact renderer inputs
whose reviewed SHA-256 values are compiled into the controller.
Rendering requires binary hashes, mutually pinned Ed25519 keys, fresh
run/cohort/machine/disk/filesystem/bootstrap-nonce identities, and package XDG roots. The
keypairs and bootstrap nonce are generated before authorization in a new
owner-only `dockpipe.vm.identity-material.v1` staging bundle. Their public hashes
and nonce are included in the contract and plan, reservation refuses different
material, and the staging copy is consumed only after durable final reservation. The
rendered seed disables network and SSH, requests no packages or apt changes,
formats only the exact virtio data-disk serial with the reviewed UUID/ext4
tuple, mounts only by that UUID, installs only the hash-pinned guest binary plus
reviewed systemd/config/key files, and starts the fixed agent service. There is
no user-provided command field.

The service is unprivileged, nologin, capability-free, private-networked,
ordered before network, and uses virtio-serial. Its implemented service mode
emits the one canonical signed bootstrap and then recognizes only canonical,
signed, length-prefixed `request` frames for the five reviewed capabilities.
Identity, health, and binary-pin verification respond; checkpoint
and recovery remain fail-closed until a separate reviewed harness adapter owns
their state transition. Signature, public-key pin, binary pin, freshness, replay,
sequence, identity, nonce, and payload substitution failures close the stream.
System state stays on the OS disk; tickets and results use the qualification
mount.

The controller's provisioning plan is a deterministic closed set of typed
operations: exclusive identity reservation; source verification; private OS
clone and 4 GiB sparse raw data-disk creation; exact NoCloud rendering and seed
creation; hash-pinned asset installation; stable format/mount; QEMU launch;
guest verification; controlled shutdown; failure preservation; and later exact
cleanup. Planning invokes no subprocess and the emitted plan remains
`execute=false`. A distinct, short-lived authorization file authenticates both
the exact contract digest and complete typed-plan digest; the operator must
additionally select the closed `--execute-qualification` CLI mode. The checked-in
package authorization template defaults to `approved=false` and grants no authority.

The next offline contract layer binds that plan to one task-owned QEMU `11.0.3`
Linux/amd64 bundle. The bundle contains only hash-pinned `qemu-img`,
`qemu-system-x86_64`, and their exact runtime closure. `qemu-img` has one fixed
120-second private backing-clone argv; the controller owns exclusive 4 GiB
sparse-raw creation and deterministic `dockpipe-go-iso9660-v1` seed creation.
QEMU launch is bounded to 120 seconds, signed guest verification to 240 seconds,
and QMP `system_powerdown` shutdown to 120 seconds with no fallback signal.
Any failure stops once, preserves all four instance roots, and never retries or
cleans. Cleanup requires a separate fresh authorization bound to the exact
contract, plan, executor digest, and ordered resource list.

Gate 1 materialized this bundle at
`/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1`, with
manifest SHA-256
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`.
Two independent builds matched the same complete output inventory. The
production Linux runner now implements the exact typed operations with no shell,
environment passthrough, fallback tool, retry, or automatic cleanup. Offline
tests use inert subprocess fixtures and in-memory connections. The first Gate 2
attempt on 2026-08-08 failed closed at `launch-qemu` because its 174-byte QMP
and 176-byte agent pathname sockets exceeded Linux's 107-byte Unix-socket path
limit. Exact separately authorized cleanup completed. Planning now rejects
overlength socket paths before authorization. Gate 2 remains unqualified and
Gate 3 has not run.

One subsequent fresh Gate 2 invocation reached `verify-guest`. QEMU created
both exact Unix sockets and the controller connected to the agent chardev, but
the single 60-second verification deadline expired before a complete framed,
signed guest bootstrap arrived. Bootstrap verification, evidence creation, and
all controller requests were therefore never reached. A separately authorized
offline read-only inspection found no `/var/lib/cloud` state and no matching
agent-service journal entries. It did not establish whether the guest failed in
boot, cloud-init, service startup, or unprivileged virtio-port access.

A later fresh Gate 2 attempt made that boundary observable. The first-boot
console reached cloud-init local discovery and networkd, then remained in
`systemd-networkd-wait-online.service` beyond the old 60-second verification
deadline. Offline read-only extraction of the exact pinned Ubuntu image proved
that its wait-online service defaults to 120 seconds and that
`cloud-init.service` is explicitly ordered after it. Because NoCloud installs
the guest agent later, a networkless qualification boot cannot emit the signed
bootstrap inside the old deadline. The reviewed policy now allows 180 seconds
for signed guest verification while retaining `-nic none`, disabled SSH, the
120-second clone/launch/shutdown bounds, complete failure preservation, and no
retry, cleanup, or fallback signal. This policy change invalidates every old
contract, plan, authorization, and executor for fresh execution. Executor-v4
owns the new deadline; preserved executor-v3 and executor-v2 contracts remain
cleanup-only under their exact separately authorized lists.

For this failed run, cleanup initially refused the still-active host QEMU. A
separately authorized QMP-only recovery sent `system_powerdown`, observed the
exact process exit within 120 seconds, and sent no fallback signal. A final
fresh executor-bound cleanup then completed once; immediate host read-back
confirmed all ordered 12 resources absent while the immutable prior promotion
and post-correction build root remained untouched.

The `dockpipe.vm.first-boot-observation.v1` policy is now production-wired but
still non-authorizing. The fresh provisioning plan and sealed executor-v6 bind
the exact evidence and runtime paths, existing `isa-serial/ttyS0` source,
one-shot Unix transport with QEMU as client and the controller as listener,
exclusive mode-`0600` evidence, 4 MiB prefix cap, fail-closed overflow, fsync,
and stop/join lifecycle. The controller creates the bounded sink before QEMU,
captures during guest verification, and deterministically closes and joins it
before verification returns, shutdown, or preservation. Listener-setup failure
propagates listener-close, sink-sync, sink-close, and parent-directory-sync
errors. It adds no seed or guest mutation, private-payload read, network, shell,
reconnect, retry, fallback, signal, or automatic cleanup.

Preserved executor-v5, executor-v4, executor-v3, and executor-v2 contracts
remain accepted only by the exact cleanup path; fresh execution requires
executor-v6 and the observation policy, and compatibility tests pin all
historical cleanup paths.
The checked-in authorization template remains `approved=false`, every emitted
plan remains `execute=false`, and this slice created no live inputs or artifacts.
Absence of a console milestone will remain non-diagnostic. A subsequent offline
source-build review produced byte-identical independent Linux/amd64 controller
and guest-agent builds. Their SHA-256 values are respectively
`b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc` and
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`.
The Windows/amd64 guest-agent cross-build SHA-256 is
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`;
it is compatibility evidence only. No output was promoted or copied into a live
or preserved root.

## Offline promotion boundary

The reviewed deterministic builds are not yet Gate 2 inputs. A distinct
offline promotion must first publish only the Linux/amd64 controller and guest
agent into `/home/jamie/.local/share/dockpipe-vm-gates`, a fixed non-live
task-owned namespace separate from the checkout, package/install and generated
stores, caches, live XDG roots, and preserved Gate 2 roots. Promotion IDs match
`vmp-[0-9a-f]{16}`. The first completed ID is `vmp-2026080815f0ea3f`:

- root:
  `/home/jamie/.local/share/dockpipe-vm-gates/promotions/vmp-2026080815f0ea3f`
- evidence:
  `/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f/promotion.evidence.json`

The root is exclusively created mode `0700`. Its immutable closed inventory is
exactly `dockpipe-qemu-controller` (`5447054` bytes,
`b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc`)
and `dockpipe-guest-agent` (`3870038` bytes,
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`).
Both are regular non-symlink single-link static Linux/amd64 ELF executables,
exclusively created no-follow as mode `0500`, owned by the effective user and
primary group. No other entry may remain. The `linux-b` outputs are rechecked
byte-for-byte immediately before promotion but never copied. The Windows hash
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`
is compatibility-only evidence with `promoted: false`.

The gate verifies owned non-symlink namespace components and rejects checkout,
store, cache, live, preserved, source-review, and mutual overlap. It exclusively
creates exact mode-`0700` promotion and evidence directories, failing if either
task-owned destination exists. All created directories and files are owned by
the effective user and primary group. It synchronizes the promotion-root
parent, copies and synchronizes each exclusive mode-`0500` file, closes and
reopens it no-follow for complete metadata/hash/Go-build read-back, verifies the
exact inventory, and synchronizes the promotion root. It then synchronizes the
new evidence-directory parent, exclusively writes mode-`0600`
`promotion.evidence.json`, synchronizes and reopens it, synchronizes the
evidence directory, and immediately reads back both roots. Any failure stops
once and preserves partial roots without retry, replacement, repair, fallback,
or cleanup; the promotion ID cannot be reused.

Canonical evidence uses schema `dockpipe.vm.offline-promotion.v1`, stable field
names, and an ordered two-entry `promoted_inventory`. It records the promotion
ID, checkpoint, source-review root, exact `linux-a` and `linux-b` paths and
comparison results, full Go build provenance, destination/evidence paths,
effective UID/GID and modes, per-file paths/source/hash/size/type/mode/UID/GID/
link-count/Go metadata, the non-promoted Windows artifact, every file and
directory sync boundary, exact closed inventory, package/engine boundary, and
explicitly false identity, live-input, plan, authorization, disk, seed, socket,
QEMU, Gate 2, cleanup, and Gate 3 actions. It contains no secret, private key,
live authorization, or preserved-root content.

The exact contract and stable evidence field names are defined in the VM
package README. A successful promotion is immutable read-only input: Gate 2
does not consume, replace, mutate, expire, or clean it. A failed promotion
preserves its partial roots. Either removal path requires a separate exact
offline cleanup authorization. Only a later Gate 2 preparation gate may bind
the promoted paths into a task-owned provisioning input outside the checkout;
the checked-in template remains machine-path neutral.

The original docs-only decision produced no promotion evidence and granted no Gate 2
authority. Source-build evidence, offline promotion, Gate 2 preparation, live
authorization/execution, cleanup, and Gate 3 are separate gates. Gate 2 remains
unqualified and Gate 3 remains blocked.

The separately authorized promotion later completed once with that exact
two-file inventory. The evidence-file SHA-256 is
`c411a6cfa326d61c6bfd9663a7f063d21dcb364520c2274fb3fe34d1f951889b`.
It remains immutable historical input. The 1.1.1 verification-deadline fix
required a new build and still requires promotion under a fresh identity; it
cannot replace or reuse this promotion.

The correction was subsequently rebuilt twice from checkpoint
`f6d5c19c24613945f5cbcf190aca50725ab51fdf` under the private offline root
`/tmp/dockpipe-vm-source-review.2ikAeuDJ`. Independent Linux/amd64 lanes were
byte-identical: controller SHA-256
`564d57937bef2856777dc3a3d05a57649e8918a0572f9f7f4d758308e9a7089c`
at `5447246` bytes and guest-agent SHA-256
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`
at `3870038` bytes. The Windows compatibility hash remains
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`.
Builds used Go 1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOAMD64=v1`,
`-trimpath`, and `-buildvcs=false`, and every non-standard dependency remains
package-owned. No build was promoted or used live.

The separately authorized promotion `vmp-20260808f6d5c19c` then published only
those exact Linux controller and guest-agent bytes. Its canonical evidence
SHA-256 is
`71827ec3cb32d35b92773b74fc0e0e2a68f0ba223341811c5e9da6b2de0f271d`.
The promotion is immutable, the earlier promotion remains untouched, and no
identity, plan, authorization, VM, cleanup, Gate 2, or Gate 3 action was
included. Fresh preparation remains separately authorized.

Fresh offline preparation later created run `g2r-1f9bdb5dd11545a4`, cohort
`g2c-17706e2c6519c7b0`, contract SHA-256
`656c5bca0ae6d0f994ecdad799b4a4d58354b955396e547d29981c0980521f1c`,
and inert plan SHA-256
`bb1670208553885674e698b86bd0fee103ccda4e4cadee0497420b7913c09edc`.
The plan remains non-executing and non-authorized, uses the 180-second guest
deadline, and keeps all Unix socket paths within the reviewed limit. No live
root or VM was created; Gate 2 execution remains separate.

That exact authorization was later executed once and is spent. The run failed
closed after 180 seconds, preserved the complete instance, and captured 86,335
bytes of first-boot console evidence. Cloud-init reached the agent start only
at about 176.2 seconds. Its logs also prove two independent provisioning
defects: early `write_files` ownership lookup failed because the agent account
did not yet exist, and the 23-character data-disk serial did not produce the
requested Linux virtio by-id path. The QEMU process is no longer active, but
the roots remain preserved for separately authorized exact cleanup. The run
must not be retried.

VM package 1.1.2 defers the three agent-owned key/config files until cloud-init's
final stage, removes schema-invalid empty/unsupported NoCloud fields, restricts
both disk serials to the 20-byte virtio-blk limit, and gives guest verification
240 seconds. The user-data shape validates against the schema extracted from
the exact pinned Ubuntu image. Executor-v5 seals this policy; executor-v4 keeps
its 180-second shape only for exact cleanup, and executor-v3/v2 keep their
60-second cleanup shapes. Networking remains `-nic none`, the separate NoCloud
network-config has no Ethernet interfaces, SSH is disabled, and there is still
no retry, fallback signal, or automatic cleanup. Fresh builds, promotion,
preparation, and live authorization remain separate. Gate 2 is unqualified and
Gate 3 remains blocked.

The 1.1.2 source review then built exact checkpoint
`9f6d406e9725acb73476bcde1617fc4fce87b700` in
`/tmp/dockpipe-vm-source-review.HIC5Ps`. Independent Linux/amd64 lanes are
byte-identical: controller SHA-256
`c6c2ce8abebf9027af01fdde6a4ac8c487eb53124fbea5c2edeee7c538f5ad7b`
at `5447222` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes. The Windows/amd64 compatibility build is `3966976` bytes
with SHA-256
`86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`.
All builds use Go 1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOAMD64=v1`,
`-trimpath`, and `-buildvcs=false`; all non-standard dependencies remain under
`packages/vm/tools/**`. No output was promoted or used by a live gate.

The failed executor-v4 instance was later cleaned once through its separate
exact cleanup gate. Wrapper SHA-256
`49d1d779a1a245c4974e23273a2cea8377fe81afec41b7207647805fb4087744`
used authorization SHA-256
`eee700a82b075564f4a9406101501292980fb62150314319e040e4f3158cdaf1`
and bound execution SHA-256
`b59f443e2c5aea26dc8e798500aa7ec58d2c2415ffb017ebc77ce4077d4a0266`.
The result reported `completed=["cleanup"]`, `cleanup_run=true`, and
`preserved=false`; result SHA-256 is
`5e45ed67fc6866bc4715791e190371b76c334330c8495f262c6fa21ed0d5e0f0`.
Independent read-back confirmed all 12 ordered resources absent. No retry or
fresh live VM action occurred. The executor-v5 source-review outputs still
require separate promotion, preparation, and live authorization.

The following separately authorized offline promotion published
`vmp-202608089f6d406e` with exactly two mode-`0500`, single-link Linux files:
controller SHA-256
`c6c2ce8abebf9027af01fdde6a4ac8c487eb53124fbea5c2edeee7c538f5ad7b`
at `5447222` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes. Canonical evidence SHA-256 is
`6ee9dfac5ed41d60cd4335d70284588c198ebdfbfd475a3f3be95c2cf08b8987`.
Windows remained compatibility-only. Independent read-back confirmed the
closed inventory and package/engine boundary, with no identity, live input,
plan, authorization, VM, cleanup, Gate 2, or Gate 3 action. Fresh preparation
remains separate.

The first executor-v5 preparation attempt for run `g2r-4153130ffe9e0189` and
cohort `g2c-788fe545f4721de8` was spent without retry after its wrapper expected
a top-level `execution` object that an inert plan does not contain. The timeout
is bound in the provisioning input and typed `verify-guest` operation;
executor-v5 is sealed only at the later live boundary. The wrapper created the
identity bundle and two inputs but did not persist the inert plan; all live
roots remained absent. A separate exact cleanup removed the eight-file partial
root. Cleanup result
SHA-256 is
`6df37d51443afeb37b2c3272ee6563e0d7d596a7d2755a525be715686a668aa3`,
and independent read-back confirmed the task and live roots absent. Corrected
preparation requires a fresh run, cohort, and identity bundle.

Corrected preparation then completed for fresh run `g2r-ff1cc0a230d1f0c2`
and cohort `g2c-1e7fe20cf8ac84ad`. Qualification input SHA-256 is
`cc1eee4b2fb9cd258307205535f4218ddedd3b0b47ab9e4c2be6c801ecb9e805`,
provisioning input SHA-256 is
`99c715a35c339730c790f62ac23ec11f3e8327a11e791f57db2360b224fee297`,
contract SHA-256 is
`1b09bfb642a968237dd66bbe3693213ade7b1229bdec48569aee331f5b9aed34`,
and inert plan SHA-256 is
`39177e8612cb0b83e0f6960027268d3d34dc61c30568a13bffeda1d023ab7988`.
The identity expires at `2026-08-09T16:47:02Z`. The plan is non-authorized and
non-executing, binds 240-second guest verification, uses 20-byte disk serials,
and keeps QMP, agent, and console sockets at 93, 95, and 92 bytes. All live
roots remain absent; live Gate 2 remains separately authorized.

That executor-v5 Gate 2 authorization was subsequently consumed exactly once.
The controller timed out at signed guest verification after 240 seconds,
preserved every root, and did not retry, signal, clean, or enter Gate 3. The
recorded QEMU process is absent. The console plus a read-only forensic copy of
the overlay show cloud-init rejected the agent user because `system: true` and
`ssh_redirect_user: true` cannot be combined. The failed users module then left
the three deferred agent-owned files without an account, so the agent could not
start correctly.

VM package 1.1.3 removes `ssh_redirect_user` while retaining the locked system
account, `/usr/sbin/nologin`, disabled SSH, `-nic none`, and the existing
systemd sandbox. The package pins the corrected NoCloud asset and rejects any
reintroduction in tests. Executor-v6 is the only fresh-execution schema;
executor-v5 remains cleanup-only for this preserved instance. Cleanup, source
review, promotion, fresh preparation, live Gate 2, and Gate 3 remain separate
authorization boundaries.

The complete package Go suite, `go vet`, focused race tests, VM package test,
Linux and Windows workflow validation, and isolated workflow/resolver
compilation all pass. Cloud-init 26.1 extracted from the exact preserved image
also accepts the corrected rendered user-data shape. Those checks were offline
only and created no checkout-generated state, promotion, or live VM action.

Exact checkpoint `97480d78d3e7a69f22f4d17c6551f6b4d9d877d0` was then
rebuilt under `/tmp/dockpipe-vm-source-review.97480d78.5adWdyrB`. Independent
Linux/amd64 lanes are byte-identical: controller SHA-256
`d43af4d07ce6c338494f0a36acfe9530029fce1545201411387067dd6b1ced43`
at `5447246` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes. The Windows/amd64 compatibility build remains `3966976`
bytes with SHA-256
`86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`.
All builds used Go 1.25.0 with the reviewed reproducibility flags, and all
non-standard dependencies remain package-owned. No output was promoted or used
for cleanup or a live gate.

The first approved cleanup packet expired before controller invocation and was
not retried. It removed nothing. A separately approved fresh wrapper then
created authorization SHA-256
`d903cc895e189eccb5facf81ce1f5fdac32adb41bd10fb1340e6740a95ba6dc1`
and completed the exact executor-v5 cleanup once. Result SHA-256 is
`ff971ac3fc994e72e40886c8f2eb6140e1b4ffe4919023e489d12fed6489ace4`;
the typed result reports `completed=["cleanup"]`, `cleanup_run=true`, and
`preserved=false`. Independent read-back confirmed all 12 ordered resources
and the recorded QEMU process absent while both the prior promotion and new
source-review outputs remain unchanged. No retry or other gate action occurred.

The following separately approved offline promotion published
`vmp-2026080897480d78` with exactly two owner-only mode-`0500` files:
controller SHA-256
`d43af4d07ce6c338494f0a36acfe9530029fce1545201411387067dd6b1ced43`
at `5447246` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes. Promotion evidence SHA-256 is
`c82161da827842b926bbed834fddbd14957577df5a69ae5d4d640949deb7eac1`.
Independent read-back confirmed the exact checkpoint, closed inventory, modes,
hashes, and package/engine separation. No identity, plan, authorization, live
VM, cleanup, Gate 2, or Gate 3 action occurred; fresh preparation is separate.

Fresh preparation subsequently completed for run `g2r-40a86fe85ed7b2f8`
and cohort `g2c-c0805ed9b9d9aff1`. Qualification input SHA-256 is
`410adcc1056cb9fe1ac8d190e30ff4d414ceeddcf56d54725ff3cd2fd3ddba37`,
provisioning input SHA-256 is
`74b5d529a49da372f9baec334fd4ddc72183ebdb4cb976b986932150ded621ad`,
contract SHA-256 is
`010406e2cbdf7903229c7cc344fcf1cdfb99fdc2f43cd52b37a36afc1ace6a6a`,
and inert plan SHA-256 is
`afa3ac656db78672aea4b03a4fd55561ffb7b050224714598a09dcd6e3ab9881`.
The owner-only identity expires at `2026-08-09T18:46:09Z`. The plan is
non-authorized and non-executing, binds executor-v6's 240-second verification,
and keeps all socket paths below Linux's limit. Every live root remains absent;
live Gate 2 is separately authorized.

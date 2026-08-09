# DockPipe VM package

The `vm` package owns guest-specific workflows, QEMU resolver models, and the
VMM-neutral control protocol. DockPipe core remains generic. Version 1.2.1
makes the exact virtio-port group and mode persistent across guest boots for
the Gate 3 durability cohort. Executor-v10 is the only fresh-execution schema;
executor-v9 remains exact-cleanup-only after its failed first Gate 3 boot. The
unchanged `windows-vm` surface remains available alongside Linux qualification.

The Linux foundation has two deliberately different paths:

- `linux-vm` composes the generic `runtime: vm` and `resolver: qemu` for
  ordinary user-owned development images.
- qualification execution remains separately authorized and disabled by every
  checked-in template. The controller owns the sealed production path, but no
  VM operation occurs without a fresh exact live authorization.

The immutable Ubuntu profile is `profiles/ubuntu-24.04-amd64.yml`. Downloads are
never implicit: the official release-stamped URL and SHA-256 must both match,
and an override path still requires its own explicit SHA-256.

## XDG layout

| Material | Location |
| --- | --- |
| Images | `${XDG_CACHE_HOME:-$HOME/.cache}/dockpipe/vm/images` |
| Instances | `${XDG_STATE_HOME:-$HOME/.local/state}/dockpipe/vm/instances` |
| Evidence | `${XDG_STATE_HOME:-$HOME/.local/state}/dockpipe/evidence` |
| Configuration and public pins | `${XDG_CONFIG_HOME:-$HOME/.config}/dockpipe/vm` |
| Sockets and process records | `$XDG_RUNTIME_DIR/dockpipe/vm` |

`XDG_RUNTIME_DIR` is mandatory. There is no checkout, `.dorkpipe`, or generated
store fallback. Private keys and recovery tickets use owner-only permissions.

## Package tools

`tools/cmd/dockpipe-guest-agent` and
`tools/cmd/dockpipe-qemu-controller` are package-owned tools. The controller can
validate a manifest, verify the exact cached source image, and print a
deterministic inert provisioning or QEMU argv plan. Planning also verifies the
six NoCloud/systemd inputs against reviewed hashes compiled into the controller.
A separate short-lived authorization must bind to both the exact contract and
complete typed-plan digests. `--execute-qualification` consumes only that
authorized inert plan through the sealed typed runner; it is not a generic
command surface. The guest binary implements the systemd-referenced virtio-serial
service mode and the protocol-v2 boot-identity bootstrap. Before reading any
controller request, it reads the actual kernel boot ID from
`/proc/sys/kernel/random/boot_id` and writes one canonical, guest-signed
`bootstrap`/`identity/v1` frame. That frame is sequence 1 and is bound to the
fresh launch bootstrap nonce, all static run/machine/disk/scenario/boundary
identities, and both key and binary pins. The controller can therefore learn
the boot ID only after authenticating the pinned guest key; no unsigned frame
or pre-launch boot-ID claim is authoritative.

After that bootstrap, controller-signed requests start at sequence 2 and carry
the authenticated boot ID. The guest verifies controller signature, identity,
contiguous sequence, fresh non-reused nonce, lifetime, capability, and
binary/key pins before returning a guest-signed response. Identity, health, and
hash-pinned-launch are operational. Executor-v9 also binds checkpoint and
recovery to one exact test-only SQLite harness binary, durable pending/consumed
tickets, and four reviewed Gate 3 boundaries. The agent launches only that
root-owned hash-pinned binary with one fixed private role variable and a strict
typed JSON command; no generic argument, environment, shell, network, SSH, or
arbitrary execution surface exists.

`manifests/linux-provisioning.template.json` is deliberately non-runnable. A
live gate must replace every marker with fresh identities, a 32-byte launch
bootstrap nonce, the current XDG
runtime root, task-owned binary paths and hashes, and mutually pinned fresh
keys. Fresh keypairs are generated in memory first, their public hashes are
bound into the contract and plan, and only those same keys may then be reserved
exclusively. The controller rejects relative, checkout, `.dockpipe`, `.dorkpipe`,
pre-existing, mismatched, expired, or substituted inputs.

`--prepare-identity-material` implements the reviewed cross-authorization key
lifecycle. It generates the nonce and both keypairs in memory, then durably
creates exactly five owner-only files under a new mode-`0700` task staging root
outside the checkout and live XDG roots. Gate 2 binds the emitted public hashes
and nonce into provisioning v3. The live invocation reloads and validates the
exact keypairs, reserves them durably under the final configuration root, and
only then consumes the staging copy. A failure before that point preserves the
staging bundle; a later failure preserves the final configuration root.
The staging descriptor expires exactly 24 hours after creation. Expiry never
deletes material automatically; it requires explicit removal and fresh preparation.

The executor-v8 verification request requires the controller to read and verify
the bootstrap before it writes anything to the stream. Acceptance requires the
exact pinned guest signature, fresh time window, bootstrap nonce, sequence 1,
phase `bootstrap`, static identity tuple, kernel boot-ID source, and key/binary
pin payload. Before request sequence 2, the controller must exclusively create
owner-only `bootstrap.json`, write the verified frame and learned boot ID, and
fsync both file and parent directory. Existing evidence, any mismatch, or any
write/fsync failure stops once with complete preservation and no retry.

The provisioning contract also requires a separate task-owned immutable QEMU
`11.0.3` Linux/amd64 bundle. Its exact owner-only root, manifest digest,
`qemu-img`, `qemu-system-x86_64`, and complete runtime-library/ROM/data inventory
are hash-pinned. Validation rejects `PATH`, checkout/generated-root overlap,
symlinks, extra files, widened modes, version/hash substitution, and fallback
tools. Gate 1 materialized the exact bundle at
`/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1`, pinned by
manifest SHA-256
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`.
The exact manifest and build evidence live under
`toolchains/qemu-11.0.3-linux-amd64/`; they do not wire package installation,
release, registry, signing, or version-resolution backlog.

`tools/internal/executor` derives a closed execution contract only from the
authorized provisioning contract and plan digest. Its injected runner exposes
only typed OS-clone, sparse-raw, NoCloud-seed, QEMU-launch, signed-verification,
controlled-shutdown, preservation, and exact-cleanup methods. It has no generic
command, shell, environment, network, SSH, passthrough, share, physical-disk,
or fallback method. The Linux production adapter uses an empty environment and
only the contract-pinned absolute `qemu-img` and `qemu-system-x86_64` argument
vectors. It creates the raw disk and deterministic ISO in Go, performs the
guest-first signed exchange, sends only QMP `system_powerdown`, preserves every
failure root, and never retries or cleans automatically. The first authorized
Gate 2 attempt on 2026-08-08 failed closed at `launch-qemu` before its sockets
became ready: the reviewed QMP and agent pathname sockets were 174 and 176
bytes, exceeding Linux's 107-byte `sockaddr_un` pathname limit. The four live
roots were preserved, the exact owned QEMU process was no longer active, and a
separately authorized cleanup removed only the executor-bound 11-resource
list. Planning now rejects either overlength socket path before authorization.
After that correction, one fresh Gate 2 invocation reached `verify-guest` and
failed closed when no complete four-byte-length-prefixed signed bootstrap
arrived within the single 60-second deadline. QEMU created both exact sockets
and the controller connected to the agent chardev, but bootstrap verification,
evidence creation, and controller requests were never reached. A separately
authorized read-only disk inspection found no `/var/lib/cloud` state and no
agent-service journal entries, so the earliest missing observable milestone is
cloud-init/first-boot state; the guest-side cause remains unproven.

A later fresh Gate 2 attempt captured the missing first-boot milestone. The
pinned Ubuntu image reached `cloud-init-local.service`, started
`systemd-networkd.service`, and then remained in
`systemd-networkd-wait-online.service` beyond the controller's 60-second guest
verification deadline. Offline read-only extraction of the exact pinned source
image proved that wait-online is enabled in `network-online.target`, defaults to
120 seconds, and is an explicit predecessor of `cloud-init.service`. NoCloud
installs the guest agent only after that dependency completes, so the old bound
could not observe a signed bootstrap on this deliberately networkless boot.

The reviewed execution policy now allows 180 seconds for guest verification.
Clone, launch, and shutdown remain bounded to 120 seconds; networking, SSH,
automatic retry, automatic cleanup, and fallback signals remain disabled. The
new timeout is part of the provisioning contract, deterministic plan digest,
and sealed executor-v4, so old plans and authorizations cannot acquire the wider
deadline. The failed live roots were preserved pending separately authorized
exact cleanup. That cleanup later completed after an exact QMP-only
power-down; all 12 executor-bound resources are absent and the immutable
promotion remains untouched. Fresh deterministic builds are recorded below;
promotion, preparation, authorization, and one new Gate 2 invocation remain
required before Gate 2 can be qualified.

`dockpipe.vm.first-boot-observation.v1` is now bound into every fresh
provisioning plan, its digest, executor-v8, and the exact QEMU argv. QEMU exposes
only the existing `isa-serial/ttyS0` stream as a one-shot Unix client; before
launch, the controller creates the exact listener and an exclusive mode-`0600`
`first-boot-console.log`. The controller retains at most the first 4 MiB,
fails closed on the next byte, fsyncs the retained prefix and evidence
directory, and closes and joins capture before verification returns, shutdown,
or failure preservation. Listener-setup failure also propagates listener-close,
sink-sync, sink-close, and parent-directory-sync errors. Path, transport, role,
reconnect, mode, cap, overflow, and lifecycle substitutions are rejected by
planning and sealed executor validation. The path adds no NoCloud or guest
mutation, private-payload read, network, shell, retry, reconnect, signal,
fallback, or automatic cleanup.

Fresh qualification execution now requires executor-v8 with that exact policy.
Preserved executor-v7, executor-v6, executor-v5, executor-v4, executor-v3, and executor-v2 files remain
loadable only for their separately authorized exact cleanup resource lists;
they cannot regain qualification execution, and independent compatibility
tests pin all six cleanup paths. The checked-in live authorization remains disabled and
the plan remains `execute=false`. No new live identity, root, disk, seed,
process, socket, or evidence was created by this offline slice. Gate 2 remains
unqualified. The subsequent offline source-build review produced two
independent byte-identical Linux/amd64 builds from the package-owned source with
`GOWORK=off`, `CGO_ENABLED=0`, `-trimpath`, and `-buildvcs=false`. The controller
SHA-256 is `b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc`,
the Linux guest-agent SHA-256 is
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`,
and the Windows/amd64 guest-agent compatibility build SHA-256 is
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`.
The Windows output is cross-compilation evidence only. None of these outputs
was promoted or copied into a live or preserved root.

## Offline source-build promotion

Deterministic source-build evidence is not promotion evidence and is not live
authority. Before any fresh Gate 2 preparation, the reviewed Linux outputs must
pass a separately authorized offline promotion gate into the fixed non-live
namespace `/home/jamie/.local/share/dockpipe-vm-gates`. That namespace is
distinct from the checkout, DockPipe's global package/install root,
`.dockpipe` and `.dorkpipe`, VM image and toolchain caches, every live instance,
evidence, configuration, and runtime XDG root, and every preserved Gate 2 root.
It is task-owned VM qualification input, not a DockPipe package installation or
generated store.

Every promotion ID must match `vmp-[0-9a-f]{16}`. A separately authorized gate
must supply the exact ID and every source and destination path before execution;
it may not discover, substitute, increment, or fall back to another destination.
The first completed identity is `vmp-2026080815f0ea3f`, with exact paths:

- promotion root:
  `/home/jamie/.local/share/dockpipe-vm-gates/promotions/vmp-2026080815f0ea3f`
- evidence directory:
  `/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f`
- evidence file:
  `/home/jamie/.local/share/dockpipe-vm-gates/evidence/vmp-2026080815f0ea3f/promotion.evidence.json`

The promotion root is exclusively created as mode `0700`, owned by the
effective promotion user and that user's primary group. Its closed inventory is
exactly two regular, non-symlink, single-link files, each exclusively created
without following links as mode `0500` and with the same owner and group:

| Relative name | Reviewed source | Bytes | SHA-256 | Type |
| --- | --- | ---: | --- | --- |
| `dockpipe-qemu-controller` | `linux-a` controller | `5447054` | `b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc` | static Linux/amd64 ELF executable |
| `dockpipe-guest-agent` | `linux-a` guest agent | `3870038` | `7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583` | static Linux/amd64 ELF executable |

No directory, manifest, evidence file, Windows binary, temporary file, cache,
log, key, authorization, or other artifact may remain in the promotion root.
The matching `linux-b` files are comparison inputs only: they must be checked
byte-for-byte against `linux-a` immediately before promotion and are never
copied. The Windows/amd64 guest-agent hash
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`
is compatibility evidence only and must be recorded as `promoted: false`.

Before creation, the promotion gate verifies every fixed namespace component is
an owned non-symlink directory and rejects overlap with the checkout, generated
stores, global installs, live XDG roots, preserved roots, the source-review
root, or the other task-owned destination. It creates missing fixed namespace
directories one component at a time as mode `0700`, then exclusively creates
the exact promotion root and evidence directory. Existing task-owned
destinations fail closed. Every created directory and the evidence file are
owned by the effective promotion user and that user's primary group. There is
no overwrite, rename-over-existing, alternate lane, fallback path, retry,
replacement, repair, or cleanup.

The durability sequence is fixed:

1. Revalidate both reviewed Linux pairs, hashes, sizes, types, modes, ownership,
   link counts, and embedded Go build metadata.
2. Exclusively create the mode-`0700` promotion root and synchronize its parent
   after publishing the directory entry.
3. Exclusively create each mode-`0500` destination with no-follow semantics,
   copy the exact `linux-a` bytes, synchronize it, close it, reopen without
   following links, and read back its identity, type, size, owner, group, mode,
   link count, SHA-256, Go package path, and build settings.
4. Verify the exact two-file inventory and synchronize the promotion root.
5. Exclusively create the mode-`0700` evidence directory and synchronize its
   parent.
6. Exclusively create `promotion.evidence.json` as mode `0600`, synchronize it,
   reopen and verify it, then synchronize the evidence directory.
7. Immediately read back both roots and report the evidence-file SHA-256.

Any failure stops once and preserves the exact partial promotion and evidence
roots. A failed promotion ID is never reused; inspection or cleanup requires a
separate exact offline authorization.

The canonical evidence schema is `dockpipe.vm.offline-promotion.v1`. Canonical
JSON uses stable snake-case field names and an ordered two-entry
`promoted_inventory`. It records `schema`, `promotion_id`,
`repository_checkpoint`, `source_review_root`, exact `linux_a_sources` and
`linux_b_comparison_sources`, successful `byte_comparisons`,
`build_provenance` (`go_version`, `gowork`, `cgo_enabled`, `goos`, `goarch`,
`goamd64`, `trimpath`, and `buildvcs`), `promotion_root`, `evidence_path`,
`effective_uid`, `effective_gid`, `promotion_root_mode`, and
`evidence_directory_mode`. Each ordered inventory entry records
`relative_name`, `absolute_path`, `source_path`, `sha256`, `byte_size`,
`file_type`, `mode`, `uid`, `gid`, `link_count`, `go_package_path`, and
`go_build_settings`. The remaining stable fields are
`windows_amd64_compatibility` with `promoted: false`, `file_syncs`,
`directory_syncs`, `closed_inventory`, `package_engine_boundary`, and
`actions_performed`.

`actions_performed` must explicitly record `false` for identity preparation,
live input, plan, authorization, disk, seed, socket, QEMU process, Gate 2,
cleanup, and Gate 3. Evidence contains no secret, private key, live
authorization material, or preserved-root content. The current reviewed source
root is `/tmp/dockpipe-vm-source-review.ZStX82CE`, the repository checkpoint is
`15f0ea3f027877221b78f637a00ab010d0a8be1d`, and the reviewed builds used Go
1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`,
`-trimpath`, and `-buildvcs=false`; the promotion gate must revalidate and
record the exact embedded `GOAMD64`, Go package paths, and build settings.

A successful promotion is immutable task-owned, read-only input for a later,
separately authorized Gate 2 preparation and execution chain. Gate 2 does not
consume it, mutate it in place, silently replace it, expire it, or target it
from qualification cleanup. Removal requires a separately authorized exact
offline-promotion cleanup gate. The later preparation gate binds the two exact
promoted paths into a task-owned provisioning input outside the checkout; the
machine-specific paths do not belong in the checked-in provisioning template.

That promotion completed once with the exact inventory above. Its canonical
evidence file has SHA-256
`c411a6cfa326d61c6bfd9663a7f063d21dcb364520c2274fb3fe34d1f951889b`.
The promotion remains immutable historical input; the 1.1.1 deadline correction
required new deterministic builds and still requires a new promotion identity
rather than changing or reusing it.

The 1.1.1 correction was rebuilt twice from repository checkpoint
`f6d5c19c24613945f5cbcf190aca50725ab51fdf` under
`/tmp/dockpipe-vm-source-review.2ikAeuDJ`. Separate Go caches and temporary
directories produced byte-identical Linux/amd64 outputs with Go 1.25.0,
`GOWORK=off`, `CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and
`-buildvcs=false`. The controller is `5447246` bytes with SHA-256
`564d57937bef2856777dc3a3d05a57649e8918a0572f9f7f4d758308e9a7089c`;
the guest agent is `3870038` bytes with SHA-256
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`.
The Windows/amd64 compatibility build remains
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`
and is not a promotion input. All non-standard dependencies resolve below
`packages/vm/tools/**`. These are offline source-build artifacts only; a fresh
promotion ID and authorization are still required.

That separate gate subsequently published immutable promotion
`vmp-20260808f6d5c19c`. Its exact controller and guest-agent inventory matches
the 1.1.1 hashes and sizes above. Canonical evidence SHA-256 is
`71827ec3cb32d35b92773b74fc0e0e2a68f0ba223341811c5e9da6b2de0f271d`.
The earlier promotion remains untouched. No identity, live input, plan,
authorization, VM, cleanup, Gate 2, or Gate 3 action was included; fresh
preparation is still a separate gate.

Fresh offline preparation for run `g2r-1f9bdb5dd11545a4` and cohort
`g2c-17706e2c6519c7b0` subsequently produced contract SHA-256
`656c5bca0ae6d0f994ecdad799b4a4d58354b955396e547d29981c0980521f1c`
and inert plan SHA-256
`bb1670208553885674e698b86bd0fee103ccda4e4cadee0497420b7913c09edc`.
The plan is not live-authorized, does not execute, binds the 180-second guest
deadline, and keeps all socket paths below 107 bytes. No live root or VM was
created; execution remains separately authorized.

That separately authorized run executed once and is permanently spent. It
failed closed after the full 180-second verification window, preserved every
instance root, and produced an 86,335-byte owner-only first-boot console log.
The log proves cloud-init did not start the agent until about 176.2 seconds.
It also records `write_files` failing because `dockpipe-agent` did not yet
exist and `disk_setup` failing because the requested 23-character data-disk
serial had no matching `/dev/disk/by-id/virtio-*` path. The exact QEMU process
later exited without bootstrap or verification evidence; the preserved roots
remain pending a separately authorized cleanup and the run will not be retried.

Version 1.1.2 corrects those observed failures as one new sealed contract.
Agent-owned key/config files use cloud-init's reviewed `defer: true` path, after
user creation; the rendered config validates against the schema extracted from
the exact pinned image. Qualification OS and data serials are now restricted
to Linux's 20-byte virtio-blk identifier limit. Guest verification is bounded
to 240 seconds, retaining the observed 120-second networkless wait plus the
original 60-second signed-verification allowance after the agent becomes
available. Executor-v5 owns the new policy. Executor-v4 retains its 180-second
shape only for exact separately authorized cleanup; executor-v3 and executor-v2
retain their historical 60-second cleanup shapes. Fresh deterministic builds,
promotion, preparation, and one separately authorized Gate 2 run are still
required. Gate 2 remains unqualified and Gate 3 remains blocked.

The 1.1.2 correction was then rebuilt from exact checkpoint
`9f6d406e9725acb73476bcde1617fc4fce87b700` under private offline root
`/tmp/dockpipe-vm-source-review.HIC5Ps`. Two Linux/amd64 lanes used distinct
Go caches and temporary directories with Go 1.25.0, `GOWORK=off`,
`CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and `-buildvcs=false`. Their
controller outputs are byte-identical at `5447222` bytes with SHA-256
`c6c2ce8abebf9027af01fdde6a4ac8c487eb53124fbea5c2edeee7c538f5ad7b`;
their guest-agent outputs are byte-identical at `3870222` bytes with SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`.
The Windows/amd64 guest-agent compatibility output is `3966976` bytes with
SHA-256 `86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`
and is not a Linux promotion input. Embedded metadata matches the requested
platforms and flags, and all non-standard dependencies resolve under
`packages/vm/tools/**`. These are offline review artifacts only; no promotion,
identity, authorization, live root, VM, cleanup, Gate 2, or Gate 3 action was
included.

The preserved executor-v4 instance was subsequently cleaned through a
separate exact authorization. Wrapper SHA-256
`49d1d779a1a245c4974e23273a2cea8377fe81afec41b7207647805fb4087744`
used authorization SHA-256
`eee700a82b075564f4a9406101501292980fb62150314319e040e4f3158cdaf1`
once. The controller bound execution SHA-256
`b59f443e2c5aea26dc8e798500aa7ec58d2c2415ffb017ebc77ce4077d4a0266`
and returned `completed=["cleanup"]`, `cleanup_run=true`, and
`preserved=false`. Cleanup-result SHA-256 is
`5e45ed67fc6866bc4715791e190371b76c334330c8495f262c6fa21ed0d5e0f0`;
independent read-back confirmed all 12 ordered resources absent. The failed
run was not retried and no fresh VM action occurred. Executor-v5 review outputs
remain unpromoted, so fresh promotion, preparation, and live authorization are
still separate gates.

The next separately authorized offline gate published immutable promotion
`vmp-202608089f6d406e`. Its exact two-file inventory is the Linux controller
SHA-256
`c6c2ce8abebf9027af01fdde6a4ac8c487eb53124fbea5c2edeee7c538f5ad7b`
at `5447222` bytes and Linux guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes. Canonical evidence SHA-256 is
`6ee9dfac5ed41d60cd4335d70284588c198ebdfbfd475a3f3be95c2cf08b8987`.
Independent read-back confirmed both artifacts are owner-only mode `0500`,
single-link files and that Windows remained compatibility-only. No identity,
plan, authorization, VM, cleanup, Gate 2, or Gate 3 action occurred. Fresh
preparation remains separately authorized.

The first executor-v5 preparation attempt used fresh run
`g2r-4153130ffe9e0189` and cohort `g2c-788fe545f4721de8`, but its one approved
invocation failed in the wrapper's final postcondition because an inert plan
has no top-level `execution` object. The timeout is bound by the provisioning
input and typed `verify-guest` operation; executor-v5 is sealed only at the
later authorized live boundary. The attempt created only the owner-only
identity bundle and two inputs; the inert plan was not persisted and every live
root remained absent. It was not retried. A separately authorized exact cleanup
deleted all eight partial files and the task root. Cleanup result SHA-256 is
`6df37d51443afeb37b2c3272ee6563e0d7d596a7d2755a525be715686a668aa3`.
Independent read-back confirmed the task and live roots absent. Any corrected
preparation requires fresh run/cohort identity.

Corrected preparation subsequently completed for fresh run
`g2r-ff1cc0a230d1f0c2` and cohort `g2c-1e7fe20cf8ac84ad`. Qualification input
SHA-256 is
`cc1eee4b2fb9cd258307205535f4218ddedd3b0b47ab9e4c2be6c801ecb9e805`,
provisioning input SHA-256 is
`99c715a35c339730c790f62ac23ec11f3e8327a11e791f57db2360b224fee297`,
contract SHA-256 is
`1b09bfb642a968237dd66bbe3693213ade7b1229bdec48569aee331f5b9aed34`,
and inert plan SHA-256 is
`39177e8612cb0b83e0f6960027268d3d34dc61c30568a13bffeda1d023ab7988`.
The identity expires at `2026-08-09T16:47:02Z`. The plan remains non-authorized
and non-executing, binds the typed 240-second verification operation, keeps both
disk serials at 20 bytes, and keeps socket paths below Linux's bound. Every live
root remains absent; live Gate 2 is still a separate exact authorization.

That exact executor-v5 authorization later ran once and is permanently spent.
Guest verification timed out after 240 seconds with no signed bootstrap, retry,
signal, fallback, automatic cleanup, or Gate 3 action; the complete instance was
preserved and the recorded QEMU process is absent. The first-boot console and a
read-only forensic copy of the preserved overlay prove cloud-init rejected the
agent account with `ValueError: Not creating user dockpipe-agent. Key(s)
ssh_redirect_user cannot be provided with system`. Because `users_groups`
failed, the three deferred agent-owned files also failed and the service could
not bootstrap.

Version 1.1.3 removes only `ssh_redirect_user` from that locked system account.
The account remains `system: true`, uses `/usr/sbin/nologin`, has a locked
password, and is isolated by disabled SSH and networking. The reviewed NoCloud
asset hash and regression tests pin that shape. Executor-v6 owns the corrected
policy; executor-v5 is accepted only for the preserved run's separately
authorized exact cleanup. The failed run is not retried. Offline validation,
fresh deterministic builds, promotion, preparation, and a new live Gate 2
authorization remain separate boundaries.

Offline validation passed the complete Go test suite, `go vet`, focused race
tests, the VM package test, both workflow validations, and isolated workflow
and resolver compilation. Cloud-init 26.1 extracted from the exact preserved
Ubuntu image reported the corrected rendered user-data shape as valid. The
schema check and compilation outputs remain temporary review artifacts outside
the checkout; no promotion or live action occurred.

The correction was then rebuilt from exact checkpoint
`97480d78d3e7a69f22f4d17c6551f6b4d9d877d0` under private review root
`/tmp/dockpipe-vm-source-review.97480d78.5adWdyrB`. Two independent
Linux/amd64 lanes used separate caches and temporary directories with Go
1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and
`-buildvcs=false`. Their controller outputs are byte-identical at `5447246`
bytes and SHA-256
`d43af4d07ce6c338494f0a36acfe9530029fce1545201411387067dd6b1ced43`;
their guest-agent outputs are byte-identical at `3870222` bytes and SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`.
The Windows/amd64 compatibility output remains `3966976` bytes with SHA-256
`86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`.
Every non-standard dependency resolves under `packages/vm/tools/**`. These are
offline review artifacts only; no promotion, cleanup, or live gate occurred.

The first separately approved cleanup wrapper for the preserved executor-v5
run was consumed once after its fixed authorization had expired. It failed
before controller invocation, removed nothing, and wrote no result. A fresh
one-shot wrapper then created a 600-second authorization and invoked only the
exact cleanup path. Authorization SHA-256 is
`d903cc895e189eccb5facf81ce1f5fdac32adb41bd10fb1340e6740a95ba6dc1`;
the controller returned `completed=["cleanup"]`, `cleanup_run=true`, and
`preserved=false`. Cleanup-result SHA-256 is
`ff971ac3fc994e72e40886c8f2eb6140e1b4ffe4919023e489d12fed6489ace4`.
Independent read-back confirmed all 12 executor-ordered resources and the
recorded QEMU process absent. The prior immutable promotion and executor-v6
source-review output remain unchanged. No retry, promotion, preparation, live
Gate 2, or Gate 3 action occurred.

The next separately authorized offline gate published immutable executor-v6
promotion `vmp-2026080897480d78`. Its closed inventory contains only the Linux
controller SHA-256
`d43af4d07ce6c338494f0a36acfe9530029fce1545201411387067dd6b1ced43`
at `5447246` bytes and Linux guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes. Both are mode `0500`, owner-only, single-link files.
Canonical promotion evidence SHA-256 is
`c82161da827842b926bbed834fddbd14957577df5a69ae5d4d640949deb7eac1`.
Independent read-back confirmed checkpoint
`97480d78d3e7a69f22f4d17c6551f6b4d9d877d0`, the exact inventory,
ownership, modes, hashes, and preserved package/engine boundary. The promotion
created no identity, plan, authorization, disk, seed, socket, process, cleanup,
Gate 2, or Gate 3 action. Fresh preparation remains separate.

Fresh offline preparation then completed for run `g2r-40a86fe85ed7b2f8`
and cohort `g2c-c0805ed9b9d9aff1` using promotion
`vmp-2026080897480d78`. Qualification input SHA-256 is
`410adcc1056cb9fe1ac8d190e30ff4d414ceeddcf56d54725ff3cd2fd3ddba37`,
provisioning input SHA-256 is
`74b5d529a49da372f9baec334fd4ddc72183ebdb4cb976b986932150ded621ad`,
contract SHA-256 is
`010406e2cbdf7903229c7cc344fcf1cdfb99fdc2f43cd52b37a36afc1ace6a6a`,
and inert plan SHA-256 is
`afa3ac656db78672aea4b03a4fd55561ffb7b050224714598a09dcd6e3ab9881`.
The identity expires at `2026-08-09T18:46:09Z`. The plan remains
`live_authorized=false`, `execute=false`, and `authorization_required=true`,
binds the 240-second verification operation, and keeps QMP, agent, and console
socket paths at 93, 95, and 92 bytes. All four live roots remain absent; live
Gate 2 remains a separate exact authorization.

That executor-v6 authorization ran once and is permanently spent. Cloud-init
successfully created the locked agent account, formatted and mounted the data
disk, installed all deferred files with the correct ownership, and started the
service. The agent then exited immediately with `permission denied` opening
`/dev/virtio-ports/org.dockpipe.agent.1`; signed bootstrap never began. The
complete instance was preserved, recorded QEMU PID `3947652` is absent, and no
retry, signal, cleanup, or Gate 3 action occurred.

Version 1.1.4 grants only the `dockpipe-agent` group read/write access to that
exact virtio port before service start: `/usr/bin/chgrp --dereference` keeps the
device root-owned while `/usr/bin/chmod 0660` limits access to root and the
agent group. No shell, wildcard, broader device rule, capability, or root-run
agent is introduced. Executor-v7 owns the corrected policy; executor-v6 remains
cleanup-only for the preserved run.

Offline validation passed the full VM Go suite, `go vet`, focused race tests
for provisioning and executor packages, the VM package test harness, both
workflow validators, isolated workflow and resolver compilation, and the
cloud-init 26.1 schema check for the rendered user-data shape. The isolated
compiled packages remain under `/tmp`; no generated package artifact is part
of this source correction.

The correction was then rebuilt from exact checkpoint
`4eb50c762005ae6f10f51cd57daf9790196cd4ac` under private review root
`/tmp/dockpipe-vm-source-review.4eb50c76.ZsNklPyL`. Two independent
Linux/amd64 lanes used separate caches and temporary directories with Go
1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and
`-buildvcs=false`. Controller outputs are byte-identical at `5447254` bytes
with SHA-256
`9f2e2827cffe6924645a90e7381b804111c5f4ec1c46eaab2c270c85a4b1e0d9`;
guest-agent outputs are byte-identical at `3870222` bytes with SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`.
The Windows/amd64 compatibility guest remains `3966976` bytes with SHA-256
`86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`.
Embedded metadata matches every requested platform and flag, and all
non-standard dependencies resolve under `packages/vm/tools/**`. These remain
unpromoted offline review artifacts; no cleanup, live Gate 2, or Gate 3 action
occurred.

The separately approved executor-v6 cleanup wrapper SHA-256
`26644340417e14c6cb0c3d2c3c80e96bc373964679a94925f33f68a10da5a715`
then executed exactly once. It created a 600-second cleanup authorization with
SHA-256
`8a5fbdbf19f744121b3ec7ea42b611fd1a5b0cd24972904299ae2370ac14f460`
bound to execution
`0dcc9d5aeeb8a7159749ac2a565b0eacd4cc58091904a593455f509b7d08a5b1`
and only its executor-ordered 12 resources. The pinned controller returned
`completed=["cleanup"]`, `cleanup_run=true`, and `preserved=false`; cleanup
result SHA-256 is
`55114a525775c99bcc33eab203a598bcd8aba79878e72817a97aced772d46231`.
Independent read-back confirmed every ordered resource and recorded QEMU PID
`3947652` absent. The immutable executor-v6 promotion controller and
executor-v7 source-review controller retain their reviewed hashes. No retry,
promotion, preparation, live Gate 2, or Gate 3 action occurred.

The next separately approved offline gate published immutable executor-v7
promotion `vmp-202608084eb50c76` from exact checkpoint
`4eb50c762005ae6f10f51cd57daf9790196cd4ac`. Its closed inventory contains
only controller SHA-256
`9f2e2827cffe6924645a90e7381b804111c5f4ec1c46eaab2c270c85a4b1e0d9`
at `5447254` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes, each owner-only executable mode `0500`. Canonical promotion
evidence SHA-256 is
`6fec8b7a8154f6286fac655fe668184cd9fb23f558bab7aed578dc2a502b5101`.
Independent read-back confirmed the two-file promotion inventory and one-file
evidence inventory. Windows compatibility remained unpromoted, the
package/engine boundary remained preserved, and no preparation, authorization,
disk, seed, socket, process, cleanup, Gate 2, or Gate 3 action occurred.

The next separately approved offline preparation created fresh run
`g2r-f3037fbd6df82729` and cohort `g2c-eda2763a444462ef` for promotion
`vmp-202608084eb50c76`. The owner-only identity bundle expires at Unix
`1786303837`, exactly 24 hours after creation. Qualification input SHA-256 is
`8da854d047acee50f8aee4cd9065dc0f5f54be4e2bfbfe1cd3b62f9e98913595`;
provisioning input SHA-256 is
`461cfbc52ce64ebefbdd5b810c2c8eba825bb315cbfe056273621f28ab248764`;
inert plan file SHA-256 is
`18314569a259d85b09a68bc415f9efcc777284c792dd79fee201b55ce9a99187`.
The plan binds contract
`821997f35e04e528edf5d3e8e9a67a2effe82f24cad0f76517a52abaeb9532b3`,
plan digest
`0d1fd14723fc8f504a9b9347343a3d1eb01f56aef9b151ff23b46f379c0b06e6`,
bootstrap nonce
`fbcd69ca7bc51a50a45ef13a88bba72f9e9dd4ad6514a222cc5dbeedbf0ad5e0`,
executor-v7, and the 240-second verification window. It remains
`live_authorized=false`, `execute=false`, and authorization-required. All four
live roots remain absent; no live authorization, VM execution, cleanup, or
Gate 3 action occurred.

The executor-v7 live wrapper SHA-256
`6f5052c05ac61256673360322cf9c3746d3c5091cc25f76bd8be134090977ba1`
then ran once with authorization SHA-256
`4a4e71e3d40bc8e4a86592cd77f5152962cfc633d63db32d40dedac8af4fe9e6`.
Guest bootstrap and all three signed verification capabilities succeeded:
identity matched the exact machine, disk, and boot ID; health reported
`healthy=true`; and launch-hash-pinned reported `matched=true` for the promoted
controller and guest hashes. Bootstrap evidence SHA-256 is
`77d9adb16aa125dfd1eeb8645537d066b5006ab006cf774bbb6399caae238f84`;
verification evidence SHA-256 is
`1c555ff80801d04fa3274e230bac1ca1e9d6c740cec5225de8a9b51c4af550c7`.

The run then failed closed at `controlled-shutdown` with `QMP response id
mismatch`. Complete roots and the executor-ordered 12-resource cleanup list
were preserved; recorded QEMU PID `4150024` is absent. Executor file SHA-256 is
`e2ca1d6f89ffd4f55e55aa51967b7b07d94ff3bdf270753ed4cc41b83d580137`,
and first-boot console SHA-256 is
`af5786f74d73c5665a7f753ba236106ed7effe77dd0a75c28821fe547984ea66`
at `87088` bytes. The authorization is permanently spent; no retry, signal,
cleanup, or Gate 3 action occurred.

Version 1.1.5 corrects the package-owned QMP client. QMP may interleave
asynchronous event frames before the response carrying the requested command
ID; the old client treated the first event's absent ID as a mismatched response.
The client now skips only structurally valid asynchronous events, accepts at
most 64 before the exact response, and still rejects malformed event/response
hybrids, wrong response IDs, oversized frames, decode failures, QMP errors, and
deadline or transport failures. It adds no command, reconnect, retry, signal,
fallback, or automatic cleanup. Executor-v8 owns fresh execution; executor-v7
is exact-cleanup-only for the preserved run. No `src/**` file changed, so the
package/engine boundary remains preserved.

Offline validation passed the full VM Go suite, `go vet`, focused controller
and executor race tests, the VM package harness, both workflow validators, and
isolated workflow and resolver compilation. Regression coverage injects an
asynchronous `SHUTDOWN` event before the exact powerdown response and proves a
subsequent wrong response ID still fails closed. Temporary compilation outputs
remain under `/tmp`; no generated package artifact or live action was created.

The correction was then rebuilt from exact checkpoint
`1047d7a44a98c71fd45529d2721a808d659cdfda` under private review root
`/tmp/dockpipe-vm-source-review.1047d7a4.0i4iOy5k`. Two independent
Linux/amd64 lanes used separate caches and temporary directories with Go
1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and
`-buildvcs=false`. Controller outputs are byte-identical at `5447686` bytes
with SHA-256
`ae624b6d3c140ccadac34ca1ca2eea509d1b22ece011fd22c811ba2c6bde011c`;
guest-agent outputs remain byte-identical at `3870222` bytes with SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`.
The Windows/amd64 compatibility guest remains `3966976` bytes with SHA-256
`86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`.
Embedded metadata matches every requested platform and flag, and every
non-standard dependency resolves under `packages/vm/tools/**`. These are
unpromoted offline review artifacts; no cleanup, Gate 2, or Gate 3 action
occurred.

The separately approved executor-v7 cleanup wrapper then executed exactly once.
Cleanup authorization SHA-256 is
`262595f83f033d7311a6db7343d0b1ceeb296cebd2fc8f65538655a3b03bc676`;
the pinned controller returned `completed=["cleanup"]`, `cleanup_run=true`,
and `preserved=false`. Cleanup-result SHA-256 is
`253cd08488148f3d1508c9892aa12f813b4cd491ac836bd2704929ffbdaa608c`.
Independent read-back confirmed all 12 executor-ordered resources and recorded
QEMU PID `4150024` absent. The executor-v7 promotion controller and executor-v8
source-review controller retain their exact hashes. No retry, promotion,
preparation, live Gate 2, or Gate 3 action occurred.

The next separately approved offline promotion published immutable executor-v8
promotion `vmp-202608081047d7a4` from checkpoint
`1047d7a44a98c71fd45529d2721a808d659cdfda`. Its closed inventory contains
only controller SHA-256
`ae624b6d3c140ccadac34ca1ca2eea509d1b22ece011fd22c811ba2c6bde011c`
at `5447686` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes, each owner-only mode `0500`. Canonical evidence SHA-256 is
`77567cf76eed3e2bb6b44e960bb0949b2665a5cc98d86cece66bca1707ecfb16`.
Independent read-back confirmed the closed two-file promotion and one-file
evidence inventories. No preparation, authorization, disk, seed, socket,
process, cleanup, Gate 2, or Gate 3 action occurred.

Fresh executor-v8 preparation then created run `g2r-a29152ab33508801` and
cohort `g2c-ebce6db36b4937a1` for promotion `vmp-202608081047d7a4`. The
owner-only identity expires at Unix `1786305140`, exactly 24 hours after
creation. Qualification SHA-256 is
`ac349e8729a1c7c9851e64ea696e91f8a15320e0334587673f7c041c6ad1a203`;
provisioning SHA-256 is
`80915d8b46f1c6bf99d36d2110d6ef7077380978e22f4ce2403c3e7382bc65d9`;
inert-plan SHA-256 is
`288d0b64916714f2f986ed771863c86b792021530540e27486aa741d61f26149`.
The plan binds contract
`d4fbf18728e92875d5c42427380fe6a62c235b939f6e725e1f311517b78d8d29`,
plan digest
`7d3826a3040d21e9b1be177e32fda990aa14aad6cec9d41f110e29b47d4d424c`,
bootstrap nonce
`93ef19893b20e4d154fb2f1e4cd19140b51c0fbb6b13b4db598dc6e8dc19ff4c`,
and the 240-second verification window. It remains non-authorized and
non-executing; all four live roots remain absent.

The executor-v8 live wrapper SHA-256
`67c4775f43bc61204a902d459ab50e60012e83a9456dfc78f589dbb7677438f3`
then executed once with authorization SHA-256
`7b3cb2a0f69dec174ad36b4573fe543f3b4fa0886b79096f5b9b81a28025bbe6`.
Execution SHA-256
`ab1c2e632f814a5e406e48a8caaafbce103d5a7953a564bf6c2b4009b8b82db7`
completed the private OS clone, private data disk, NoCloud seed, QEMU launch,
signed guest verification, and controlled shutdown. The typed result reports
`preserved=false` and `cleanup_run=false`.

Bootstrap evidence SHA-256 is
`ce19af3864474a0171b1aa20e1a5721aee6f9c57c216e99f86b87e6c57bdf26f`;
verification evidence SHA-256 is
`fc8f9ab92407f32abaca4ab381c3c152d843f08276491fa33099e6774b5ae096`.
Authenticated sequences 1 through 4 prove the kernel boot ID, exact machine
and disk identities, `healthy=true`, and matching promoted controller and guest
hashes. Shutdown evidence SHA-256
`f1af02b35fdb9a51946cf1228071ad6db7e6b8885ea273a6af6f984d410e482e`
records `system_powerdown`, `clean_exit=true`, and PID `67010`; independent
read-back confirms that PID and the transient QMP/agent sockets absent.
Executor file SHA-256 is
`cc5f38063a9bf06541b62fe1ab12e4d14dc684cafe3fac8da7b029085b8e5b24`,
and first-boot console SHA-256 is
`3738cb9fe16cff9ca3570604b2fcb9d8ccf40141551da42f51b966b6f485bb69`
at `87171` bytes. Gate 2 is qualified. No cleanup or Gate 3 action occurred;
Gate 3 is now unblocked and remains separately authorized.

Version 1.2.0 adds the package-owned, offline Gate 3 durability contract. It
does not run Gate 3. The reviewed cohort covers four application-visible
SQLite transitions with three independent trials per transition: twelve
checkpoint/recovery trials, twelve authenticated hard-power events, and
thirteen guest boots. Each checkpoint creates a durable pending ticket bound
to the exact run, cohort, trial, machine, disk, bootstrap identity, boot ID,
nonce, and harness hash. Recovery requires a different authenticated kernel
boot ID and independently verifies the expected old or new revision,
`PRAGMA quick_check`, the native `unix` VFS, SQLite 3.53.3 source identity,
metadata hashes, and zero retry, replay, repair, or fallback counters.

The test-only SQLite harness is built from the package-owned
`sqliteevidence` test binary and installed as one root-owned, hash-pinned
guest executable. The guest adapter exposes only the fixed checkpoint and
recovery roles; it accepts no arguments, shell, network operation, or generic
execution. The controller independently validates the signed nested evidence
and uses `pidfd_open` plus `pidfd_send_signal(SIGKILL)` only after rereading
the exact QEMU process identity. Any mismatch fails closed and preserves the
instance. A complete cohort ends with one typed QMP controlled shutdown; no
cleanup is implied.

Executor-v9 is the only fresh-execution schema for this contract;
executor-v8 remains exact-cleanup-only for the already-qualified run
`g2r-a29152ab33508801`. That successful Gate 2 instance and its twelve ordered
resources remain untouched. A later Gate 3 chain requires a separately
approved three-binary Linux promotion, fresh preparation and Gate 2
qualification, then a short-lived authorization bound to the exact inert Gate
3 plan and destructive-token hash. Promotion, preparation, Gate 2, Gate 3,
and cleanup remain distinct approvals. No live Gate 3 or cleanup action was
performed by this offline implementation.

Exact checkpoint `a72789801a83b53761b710388618d7aafc15648e` was then
rebuilt in two independent Linux/amd64 lanes under private review root
`/tmp/dockpipe-vm-source-review.a7278980.TuSIQFWC`. Each lane used its own
source extraction, cache, temporary directory, and output directory with Go
1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`,
`-buildvcs=false`, and an empty build ID. The controller outputs are
byte-identical at `5721936` bytes with SHA-256
`a8ff286cba55cf03eed2832f26069ea3812f239b6c767c77c1cd4c2cf045bd1a`;
guest-agent outputs are byte-identical at `4238382` bytes with SHA-256
`4a2533d297d698328d5875e5af7c1f57d59b0a16b55c84f4a71c6107b5fb38a2`;
SQLite harness outputs are byte-identical at `11895893` bytes with SHA-256
`08b979ab70922c596ea14847ff023357616ebc5c92daee50ecab84ffbcfa3cc5`.
Embedded metadata matches the requested platform and flags, controller and
guest versions report 1.2.0 and 1.1.0 respectively, and the harness contract
self-tests pass. These files remain unpromoted offline source-review evidence;
no preparation, authorization, VM, Gate 2, Gate 3, or cleanup action occurred.

The original documentation decision created no promotion evidence and granted no Gate 2
authority. Deterministic source review, offline promotion, Gate 2 preparation,
Gate 2 live authorization and execution, cleanup, and Gate 3 remain distinct
gates. The latest executor-v8 evidence above qualifies Gate 2 and unblocks,
but does not authorize, Gate 3.

The separately approved three-file promotion `vmp-20260808a7278980` then
published the exact controller, guest agent, and SQLite harness recorded above.
Its canonical evidence SHA-256 is
`68a83d74dc3b1fc6d2db953f8399d93654086ebbca2bfc72716c5b94647faf25`.
Fresh executor-v9 run `g2r-097a3b41599cbc76`, cohort
`g2c-c729b6ca16835493`, qualified Gate 2 through signed guest verification and
controlled shutdown. Its execution SHA-256 is
`1e77a6513ab7029213726b8b40d35e380b78d002cd5cfdce61566e177c069fae`.

The separately approved Gate 3 plan SHA-256
`c77abc2ca47c4c410d993a91de65007d680933cf15f08eba7808fcc3291937c3`
started once and failed closed on boot 1 before any checkpoint, recovery, or
pidfd power cut. The guest reached multi-user startup and systemd started
`dockpipe-agent.service`, but no bootstrap frame arrived. The service's exact
virtio-port group and mode had been established only by first-boot cloud-init;
the recreated device on the next boot did not retain that access. Both recorded
QEMU PIDs were absent, the complete instance was preserved, the authorization
was not retried, and exact cleanup result SHA-256
`6d33a98e6618108efd9af0169ed5aa3a6f28b8914f8ab9ab357630e9c9a24b68`
removed only the executor-ordered 12-resource list.

Version 1.2.1 installs one reviewed udev rule matching only virtio port
`org.dockpipe.agent.1`, assigning group `dockpipe-agent` and mode `0660` on
every device creation. The existing first-boot exact `chgrp`/`chmod` remains;
no shell, wildcard, root-run agent, network access, retry, fallback, or
automatic cleanup is introduced. This correction is offline source evidence,
not live Gate 2 or Gate 3 proof. A new build, promotion, preparation, Gate 2,
Gate 3, and final cleanup each remain separately authorized.

The offline `dockpipe.vm.gate3-checkpoint-observation.v1` contract makes a
future freshly sealed checkpoint failure distinguish four ordered milestones
without changing the 60-second action deadline. After authenticating and
strictly decoding a checkpoint request, the guest writes canonical,
non-secret `dockpipe-gate3-checkpoint-observation` records for
`request-received`, `pending-ticket-accepted`, and
`harness-evidence-emitted` to stderr. The reviewed service routes stderr to
both the journal and the already bounded, controller-owned boot console. The
pending record is written only after the owner-only ticket's file sync,
atomic rename, and parent-directory sync complete. The harness record includes
only the ticket and canonical-evidence SHA-256 values and is written only
after the pinned harness evidence is read and validated. It never emits the
ticket nonce, request payload, key material, or database content.

The controller creates a separate owner-only
`<trial>-checkpoint-response-delivered.json` immediately after receiving and
verifying the guest-signed result kind, capability, and exact request context,
before deeper payload acceptance. Any guest observation write or host evidence
durability failure stops the checkpoint path before hard power. The existing
`<trial>-checkpoint.json` remains the fully validated checkpoint result. Thus
the preserved console and evidence inventory can distinguish request receipt,
durable pending-ticket acceptance, harness evidence emission, and signed
response delivery. Absence of a later milestone still proves no absence of
guest-side state, and grants no retry, disk inspection, recovery, cleanup, or
live authority. This source-only contract does not advance the executor schema
or prepare a build, promotion, plan, authorization, or live gate.

`manifests/linux-live-authorization.template.json` is separately inert with
`approved=false`. A later reviewed gate must bind a fresh, short-lived copy to
the emitted contract and plan SHA-256 values; the offline controller still
leaves the reviewed plan itself `execute=false`. A live invocation selects the
typed executor explicitly. `manifests/linux-cleanup-authorization.template.json`
is a second inert authorization bound to the exact executor digest and ordered
resource list; cleanup is never implied by execution success or failure.

The production CLI modes are mutually exclusive:

```text
--prepare-identity-material <absolute-root> --run-id <id> --cohort-id <id>
--validate-manifest <file> --configuration-sha256
--validate-manifest <file> --plan-provisioning <file> [--live-authorization <file>]
--validate-manifest <file> --plan-provisioning <file> --live-authorization <file> --identity-material <absolute-root> --execute-qualification
--gate3-executor <executor.json> --gate3-provisioning <provisioning.json> --gate3-manifest <qualification.json>
--gate3-executor <executor.json> --gate3-provisioning <provisioning.json> --gate3-manifest <qualification.json> --gate3-plan <plan.json> --gate3-authorization <authorization.json> --gate3-token <token> --execute-gate3
--cleanup-executor <executor.json> --cleanup-authorization <file>
```

Run package tests with:

```bash
./src/bin/dockpipe package test --workdir . --only vm
```

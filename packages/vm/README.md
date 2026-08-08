# DockPipe VM package

The `vm` package owns guest-specific workflows, QEMU resolver models, and the
VMM-neutral control protocol. DockPipe core remains generic. Version 1.1.3
retains the sealed first-boot console observation path, corrects the pinned
Ubuntu NoCloud system-user contract and virtio-blk serial bound, and gives the
networkless qualification path a complete post-boot verification window
alongside the unchanged `windows-vm` surface.

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
hash-pinned-launch are operational; only signed `request` frames are accepted
after the bootstrap, and checkpoint and recovery recognize only the
reviewed signed payload shape and fail closed because the Gate 2 foundation
does not own a harness adapter. No other capability or arbitrary execution
surface exists.

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

The executor-v6 verification request requires the controller to read and verify
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
provisioning plan, its digest, executor-v6, and the exact QEMU argv. QEMU exposes
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

Fresh qualification execution now requires executor-v6 with that exact policy.
Preserved executor-v5, executor-v4, executor-v3, and executor-v2 files remain
loadable only for their separately authorized exact cleanup resource lists;
they cannot regain qualification execution, and independent compatibility
tests pin all four cleanup paths. The checked-in live authorization remains disabled and
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

The original documentation decision created no promotion evidence and granted no Gate 2
authority. Deterministic source review, offline promotion, Gate 2 preparation,
Gate 2 live authorization and execution, cleanup, and Gate 3 remain distinct
gates. Gate 2 remains unqualified and Gate 3 remains blocked.

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
--cleanup-executor <executor.json> --cleanup-authorization <file>
```

Run package tests with:

```bash
./src/bin/dockpipe package test --workdir . --only vm
```

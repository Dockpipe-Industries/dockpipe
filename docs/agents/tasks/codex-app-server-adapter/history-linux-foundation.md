#### Linux VM reboot/power-loss package foundation (2026-08-05)

The package-owned foundation needed before a controlled Linux VM trial is now implemented under
`packages/vm/**` as VM package version `0.8.0`. It adds the `linux-vm` workflow and a separate
`LinuxQemuVmResolverConfig` root alongside the unchanged Windows model, keeps `runtime: vm` and
resolver `qemu`, and does not add VM-product policy to `src/**`. The Ubuntu 24.04 LTS amd64 profile
pins the immutable `20260801` cloud-image URL and SHA-256. It requires local NoCloud, disables
qualification networking and SSH, performs no `apt` update or upgrade, requests no additional debs,
uses only the XDG cache/state/config/runtime layout, and has no checkout-generated-state fallback.

The offline qualification manifest fails closed unless it identifies a disposable KVM guest with
host CPU, two vCPUs, 4096 MiB, no swap, distinct host/guest identities, exactly one private OS clone
and one private 4 GiB sparse raw data disk, and no physical disk, passthrough, share, extra disk,
network, SSH, or arbitrary command surface. The whole-device ext4 tuple fixes UUID mounting at
`/var/lib/dockpipe-qualification`, disables lazy inode/journal initialization, and requires exactly
`rw,noatime,nodev,nosuid,noexec,data=ordered`. Package Go sources provide per-instance Ed25519 keys,
mutual pinning, bounded length-prefixed canonical JSON, signed identity and phase context, replay and
substitution rejection, recovery-only pending-ticket semantics, safe QMP parsing, exact process
authorization, inert pidfd/SIGKILL planning, exact QEMU/block argument planning, trial isolation, and
exact cleanup planning. The current Windows guest agent remains the active Windows path; there is no
cutover in this slice.

The offline Gate 2 prerequisite is now implemented as a strict typed provisioning contract for
exactly one disposable qualification instance. It requires the pinned XDG cache image path, byte
size, regular non-symlink owner-only file, and SHA-256; rejects checkout, `.dockpipe`, `.dorkpipe`,
relative, overlapping, and pre-existing generated roots; and reserves fresh run, cohort, machine,
disk, filesystem, nonce, and Ed25519 identities exclusively without replacement. Fresh keypairs are
generated before authorization, their public hashes are bound into the contract and plan, and
reservation accepts only those same keys. The controller
deterministically emits the closed set of inert operations for the private OS clone, private 4 GiB
sparse raw data disk, reviewed NoCloud rendering and seed, hash-pinned assets, stable format/mount,
QEMU launch, signed verification, controlled shutdown, failure preservation, and exact later
cleanup. Planning invokes no subprocess and always emits `execute=false`. A distinct short-lived
authorization can bind only to the exact contract and plan digest; it does not add an executor.
The separate package authorization template defaults to `approved=false` and therefore grants no
authority until a later reviewed gate supplies both exact digests and a fresh bounded lifetime.

The reviewed NoCloud and systemd assets are now exact renderer inputs rather than design-only
placeholders, and their six source hashes are compiled into the controller and checked during both
planning and rendering. Rendering pins the controller and guest binaries and both Ed25519 public identities,
disables network and SSH, performs no apt update or upgrade, installs no package, fixes the exact
data-disk serial, filesystem UUID, mount path, and options, and exposes no user-provided command
field. The systemd-referenced guest-agent service mode is implemented with the existing canonical,
signed, length-prefixed protocol, accepts only signed `request` frames, and exposes only the five
reviewed capabilities. Identity, health, and
binary-pin verification respond; checkpoint and recovery validate their reviewed signed payload
shape and then fail closed because this Gate 2 foundation does not own a harness adapter. Invalid framing,
signature, public-key or binary pins, freshness, replay, sequence, nonce, identity, capability, and
payload substitutions fail closed. Windows workflow behavior remains unchanged.

Validation for this foundation covers package Go tests with `CGO_ENABLED=0 -mod=readonly`, strict
protocol negatives, fake filesystem and socket behavior, manifest/isolation failures, XDG paths,
two-disk and QEMU tuples, cleanup identity, legacy Windows workflow validation, both workflow/model
compiles, Linux/amd64 controller and guest-agent builds, and Windows/amd64 shared guest-agent source
compatibility. Cross-compilation is compatibility evidence only. It is not a Linux VM run, native
Windows evidence, or macOS evidence.

No image was downloaded or modified in this slice. The existing Gate 1 cache artifact was verified
read-only at the pinned path, type, ownership, mode, byte size, and SHA-256. No VM, disk, filesystem,
NoCloud seed, guest service, process, SQLite
fixture, or evidence tree was created, cloned, modified, attached, started, stopped, rebooted,
provisioned, killed, destroyed, or cleaned. No QMP power command or real signal was issued. Therefore
Linux reboot/power-loss remains open, macOS remains intentionally last, TASK-013 remains open, and
Slice 2 has not started.

The remaining live work stays split into separate maintainer approvals:

1. create/provision one disposable VM and install the hash-pinned agent assets;
2. run a non-destructive identity, controller, and recovery dry run;
3. run one bounded Linux destructive cohort and preserve/read back its complete evidence; and
4. consider a separate Windows VM gate later. macOS remains the final platform gate.

#### Linux VM task-owned executor/toolchain contract (2026-08-06)

The missing offline contract between the inert Gate 2 provisioning plan and any future live runner
is now implemented under `packages/vm/**` as VM package version `0.9.0`. “Package-owned” here means
the VM-specific source, policy, templates, and tests remain in the VM package. It does not wire the
task into DockPipe package installation, release, registry, signing, global-store, or version
resolution. Those package-layer capabilities remain separate backlog work. The future QEMU bundle
is instead a separately prepared task-owned local artifact, like the pinned image and task-owned
controller/guest builds.

The provisioning and plan contracts are now v2 and bind an exact absolute bundle root plus its raw
manifest SHA-256. The new `dockpipe.vm.toolchain.v1` manifest accepts only QEMU `11.0.3` on
Linux/amd64 with KVM, exactly `qemu-img`, `qemu-system-x86_64`, and a non-empty complete
runtime-library/ROM/data closure. It pins the official source and signature URLs, release-manager
fingerprint, source-archive hash, build-recipe hash, exact relative paths, version output, file
hashes, and finalized owner-only read/execute modes. Validation rejects checkout, `.dockpipe`,
`.dorkpipe`, VM instance/evidence/config/runtime overlap, symlinks, extra or missing files, widened
modes, changed hashes or versions, substituted tools, and fallback lookup. The bundle root and all
directories are finalized mode `0500`, the manifest/runtime data are `0400`, and executables/loaders
are `0500`.

The exact OS clone command is the bundle-pinned `qemu-img create -f qcow2 -F qcow2 -b <pinned
source> <fresh private target>` with a 120-second bound and no alternate tool. The Go controller owns
exclusive mode-`0600` 4 GiB sparse raw creation and deterministic `dockpipe-go-iso9660-v1` NoCloud
construction, so `cloud-localds`, `xorriso`, and `genisoimage` are absent. QEMU startup is bounded to
120 seconds, controller-signed/guest-signed identity, health, and launch-pin verification to 60
seconds, and QMP `system_powerdown` to 120 seconds with no fallback signal.

The new `dockpipe.vm.executor.v1` contract is deterministically derived from only the exact
authorized contract, plan digest, immutable toolchain manifest, QEMU argv, and reviewed NoCloud
rendering. Its injected runner has typed methods only for private clone, sparse raw disk, NoCloud
seed, QEMU launch, signed verification, controlled shutdown, preservation, and cleanup. It has no
generic command, shell, environment, network, SSH, passthrough, share, physical-disk, or fallback
surface. Any failure stops once, performs no retry or cleanup, and requests preservation of the
complete instance/evidence/config/runtime roots. Cleanup is never automatic and requires a separate
fresh authorization bound to the contract, plan, executor digest, run/cohort, and exact ordered
resource list.

Only fake runners exist and tests launch no subprocess. There is no `os/exec` adapter or live CLI
execution flag. Gate 1 materialization completed on 2026-08-07 using the exact QEMU 11.0.3 source
archive SHA-256 `da5fcffc32762820568b828ed430a728864d34d50b6d2f30358597760cbb0523`,
detached-signature SHA-256 `719f32c491ee724629f7d5918a6ff04ddc115d92a597b504cc4f12191e4a5e77`,
signer `CEACC9E15534EBABB82D3FA03353C9CEF108B584`, and pinned builder manifest
`sha256:9108d3cbdacbaf442f8b8938a2e94a7cdf04c0b093953866726c5734cb478f2e`.
The builder configuration digest is
`sha256:ae716e47ccf0cde02ef2b290116ddc2a7c66ac0a912a6f1b74f28a5670a3dd21`,
its complete 36,551-entry inventory SHA-256 is
`ecb649e86e299e6dd0e569f15a2c4fa207e6dc03bcddf540460453b819a48cb5`, and the
reviewed recipe SHA-256 is
`669021bd42c5a47c7173821e68ec9e37143c7406e9093338318504e79b502a69`.

Two independent no-network builds produced byte-identical 125-entry output inventories with
SHA-256 `22f24ba020b98b0802d67956bd5d7699bcd9d12a99773e185165087b8b1aedec`.
The immutable `jamie:jamie` bundle is
`/home/jamie/.cache/dockpipe/vm/toolchains/qemu-11.0.3-linux-amd64.1`; its
`toolchain.json` SHA-256 is
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`.
`qemu-img` is `8f136e6f9550ca0c4d0bed73c7fb761537425c4bd0e4f95c0fd8ee93b6b2ed81`, and
`qemu-system-x86_64` is
`3544680aaeaf8087bbf3ef693ff185c2691831560c767672defccd784ec37140`.
The exact bundle-owned musl interpreter, literal `$ORIGIN/../lib` RPATH, `NODEFLIB`, recursive
declared library closure, owner-only modes, absence of symlinks/writable/group/world entries, and
absolute-path `env -i` version/help execution were verified. The exact checked-in manifest and build
evidence contain no replacement markers.

Gate 1 is complete. Gate 2 and Gate 3 have not started; no VM, VM disk, NoCloud seed, live root,
QEMU process, socket, or cleanup action was created. A separately reviewed slice may add a real
runner and Gate 2 execution prompt later.

Offline validation passed the VM package test (including the new executor and toolchain negatives),
workflow and resolver compiles, `CGO_ENABLED=0 go test -mod=readonly ./...`, `go vet
-mod=readonly ./...`, two byte-identical Linux/amd64 `-trimpath -buildvcs=false` builds, and the
Windows/amd64 guest-agent compatibility build. The new controller SHA-256 is
`ccefd4daaa2394748b08c5f3ec21efe5298aba848b4b819b1b491aa2287c6549`; the unchanged guest
agent retains SHA-256 `cb99865a1f628083a0c732341dddff1c0ecbb6ba5609a55fd78ed3a4bee3856f`.
The two Linux builds matched byte-for-byte. Cross-compilation remains compatibility evidence only.
No VM, disk, NoCloud seed, XDG VM root, process, socket, or live cleanup was created or operated.

#### Accepted Linux guest boot-identity bootstrap (2026-08-07)

The blocked boot-ID decision is closed with a guest-first signed identity frame. A predetermined
kernel boot ID and an unsigned identity exchange are both rejected. Qualification manifest v2 no
longer accepts a pre-launch `boot_id`; it fixes only the reviewed source
`/proc/sys/kernel/random/boot_id`. Provisioning/plan/live-authorization v3 renames the already fresh
32-byte value to `bootstrap_nonce` and binds it, the per-instance Ed25519 key pins, static
machine/disk/run/scenario/durability identities, and rendered NoCloud bytes into the existing sealed
contracts. Guest-agent config v2 receives that exact nonce and source.

On opening virtio-serial, the guest reads the kernel value and writes before it reads. Its first and
only bootstrap is a canonical length-prefixed `dockpipe.vm.v2` frame with kind `bootstrap`,
capability `identity/v1`, sequence 1, phase `bootstrap`, the sealed bootstrap nonce, the actual boot
UUID, every static authenticated context field, and a payload containing the boot-ID source plus the
controller-public, guest-public, controller-binary, and guest-binary SHA-256 pins. The frame is
signed by the pinned guest private key. There is no unsigned frame, controller-signed frame without
a boot ID, alternate framing path, or fallback identity authority.

The future controller must read first. Within the existing 60-second bound it verifies canonical
framing, time window, pinned guest signature, bootstrap nonce, sequence/phase, static context, boot
UUID, source, and all four pins. Before writing any controller request, it exclusively creates
mode-`0600` `bootstrap.json`, records the verified frame and learned boot ID, and fsyncs the file and
parent evidence directory. Existing evidence or any verification/write/fsync failure stops once,
preserves the complete roots, and permits no retry, reconnect, fallback, or cleanup. The first
controller-signed `identity/v1` request then uses the authenticated boot ID at sequence 2 with a new
nonce; later requests are contiguous and cannot reuse the bootstrap or another request nonce.
Guest-signed results echo the request context. This second signed exchange proves the guest's pinned
controller identity and preserves mutual Ed25519 pinning.

The offline implementation covers protocol framing and negative verification, guest-first service
ordering, manifest/config rendering, sealed executor-v2 fields, and tamper rejection. It adds no
production runner or generic command surface. No VM or Gate 2 operation is authorized by this
decision, and no Gate 2 execution prompt is emitted here.

Post-decision offline validation passed the complete VM package Go suite, `go vet -mod=readonly
./...`, the VM package test, both Linux and Windows workflow validations, VM workflow/resolver
compiles into an isolated temporary workdir, two byte-identical Linux/amd64
`-trimpath -buildvcs=false` builds, and a Windows/amd64 guest-agent compatibility build.
The current source builds are controller
`f0f6b17ab730dc69d3638a39cf8dfb082cc8d288f2257c3cbd97ba38cf5d509d`, Linux guest agent
`3f9354ff666a21a5b1fc05b2089ffe523fe4123a5d3ef04968c6af00ac66a328`, and Windows guest agent
`a0666c4e00b1725944ffbe75f8fa3e9a26f9971d6e3710a66ceff74f1a1f5957`. These temporary builds
supersede the earlier source-build hashes for this uncommitted protocol revision but do not alter the
immutable Gate 1 QEMU bundle or authorize publication or live use.

#### Linux VM production qualification runner (2026-08-07)

The package-owned production runner is implemented under `packages/vm/**` as VM package version
`1.0.0`; `src/**` remains unchanged. The accepted pre-authorization identity-material decision uses
one new mode-`0700` task staging root outside the checkout and all live XDG roots. Preparation
generates the 32-byte bootstrap nonce plus controller and guest Ed25519 keypairs in memory,
exclusively writes exactly five mode-`0600` files, fsyncs every file and directory, and emits only
non-secret identity metadata, the nonce, and public-key hashes. The later live invocation strictly reloads that inventory, verifies
keypair integrity and the provisioning-v3 pins, durably reserves the same keys under the final
configuration root, and consumes the staging copy only afterward. A failure before durable
reservation preserves the staging bundle; a later failure preserves the final configuration root.
The staging descriptor expires after exactly 24 hours; expiry performs no automatic deletion or
fallback and requires explicit removal plus fresh preparation.

The controller CLI now separates identity preparation, inert planning/authorization review, typed
qualification execution, and separately authorized cleanup. The production adapter exposes no
generic command method and gives both subprocesses an empty environment. It revalidates the pinned
source and absolute binaries, runs only the sealed `qemu-img` clone argv, creates the exclusive
mode-`0600` 4 GiB sparse disk in Go, builds the deterministic `dockpipe-go-iso9660-v1` seed in Go,
launches only the pinned QEMU argv, and records the exact owned process identity. It reads and verifies
the guest-signed bootstrap before exclusively creating and fsyncing `bootstrap.json`, then sends
controller-signed identity, health, and launch-pin requests beginning at sequence 2 with fresh
contiguous nonces. Every guest result must echo the complete request context. Shutdown negotiates QMP
and sends only `system_powerdown`; there is no fallback signal. Any failure stops once, fsyncs the
complete roots, and performs no retry or cleanup. Cleanup requires a new owner-only authorization
matching the exact contract, plan, executor digest, run/cohort, and ordered resource list, and refuses
an active recorded QEMU process.

Offline proof passed focused executor/protocol/guest/identity/QMP tests, `CGO_ENABLED=0 go test
-mod=readonly ./...`, `CGO_ENABLED=0 go vet -mod=readonly ./...`, the VM package test, Linux and
Windows workflow validation, and workflow/resolver compilation into an isolated `/tmp` workdir. Two
independent Linux/amd64 `-trimpath -buildvcs=false` builds with separate caches were byte-identical.
The controller SHA-256 is `a079350a68649c2350122fe81d4617d13aebb4c09dec960cc3279ce196002fa8`;
the Linux guest-agent SHA-256 is
`df1a55c45ddcb367e803129e712bb2c926c4c5c5f0a42c5be9e1c5a2632ace96`; and the Windows/amd64
compatibility build is `5858a6cc18d89f1a6bdd2a6bb75515c5c628d62cc65cd08e2892d94bcb65e1f9`.
No QEMU process, real VM disk, NoCloud seed, live XDG root, QMP command, signal, cleanup, Gate 2, or
Gate 3 action occurred. Gate 2 remains a separate explicit execution approval.

#### Linux VM Gate 2 launch-path correction (2026-08-08)

One exact Gate 2 invocation was authorized for run
`gate2-run-cbc0d22e-ae56-4f80-aaf0-92b6b02531e3`, cohort
`gate2-cohort-567199c4-312f-439e-8f83-f694b34d76e1`, contract SHA-256
`b201995ab497d3f131cd899418a46097c2b2f4f84b886d61297e567c6794f01a`, and plan SHA-256
`cbc9d8b7cd376187ca8eea69ab6bea3ef9aed3f175fd359f3bc6aaa2a4878418`. It ran once and failed
closed at `launch-qemu`: QEMU exited with status 1 before its exact sockets became ready. No retry,
reconnect, fallback signal, private-payload inspection, or automatic cleanup occurred. All four
owner-only live roots were preserved and the exact owned QEMU process count was zero.

Offline read-back proved the authorized QMP pathname was 174 bytes and the agent pathname was 176
bytes. Linux pathname Unix sockets have only 107 usable bytes in `sockaddr_un.sun_path` once the
terminating NUL is reserved, so the authorized launch plan could not create either socket. A
separate exact cleanup authorization bound to executor SHA-256
`6280d29d100076181f55ebd54fd1c0fba1deeab06b34487f09dc8acb4d0ccfc5` ran once and removed the
stored executor's ordered 11-resource list. Narrow read-back confirmed every authorized path absent
and no owned QEMU process active; the checkout and the separate `/tmp` review/build root were not
cleanup targets.

The package-owned correction now rejects QMP or agent pathname sockets longer than 107 bytes during
inert QEMU plan construction, before plan authorization, identity reservation, or any subprocess.
Exact boundary tests accept 107 bytes and reject 108 bytes; provisioning coverage proves long but
otherwise schema-valid run/cohort identities fail before authorization. Existing tests that used
long framework-generated temporary runtime paths now use short, unique, platform-absolute runtime
roots while their disk, toolchain, and artifact fixtures remain isolated. The corrected controller
rejects the preserved failed contract offline with the exact safe error class
`QMP Unix socket path is 174 bytes; Linux pathname sockets permit at most 107`.

Post-correction offline validation passed focused manifest/provisioning tests, the complete
`CGO_ENABLED=0 go test -mod=readonly ./...` suite, `CGO_ENABLED=0 go vet -mod=readonly ./...`, the
VM package test, and workflow/resolver compilation into an isolated `/tmp` workdir. Two independent
Linux/amd64 `-trimpath -buildvcs=false` builds were byte-identical. The current controller SHA-256
is `f49ac43a78b3589c1375ab2c67c664be42a78140b43fa1919cc1e48df1dc2984`; the current Linux guest
agent SHA-256 is `fa83a65b89d76303e808578ba7b872a33f6bd6c2d122c9ca3ba8174d531fd8f6`; and the Windows/amd64
guest compatibility build is `677a71cd966599bb8d01a8481b107fd03b1b2b06f5fe571636c139d9c2e611e8`.

Gate 2 is not qualified and was not retried. Any future attempt must start from fresh run/cohort,
machine, filesystem, disk, nonce, key, staging, build, input, contract, plan, authorization, live-root,
socket, disk, and evidence identities. Gate 3 remains blocked behind a successful separately
authorized Gate 2.

#### Linux VM Gate 2 verify-guest failure and first-boot observation wiring (2026-08-08)

After the launch-path correction, one fresh qualification invocation ran for run
`g2r-970fd15c42e793bb`, cohort `g2c-6013982ee1e49710`, contract SHA-256
`01bf24d6f792608ed9c124e737ac175efa4816ed7a8bbfc001931eba25c61d2a`, plan SHA-256
`7565df9f071d21751b87d9ee46c785bb0e6210a5161adf2718b9296bd99b247c`, executor SHA-256
`94c258b22714a9d3ab6a57a66753ee28e3d69fe40e55d84eb3fedc3f3eb672bc`, toolchain SHA-256
`11a27f32eb93e62aba8ebc500dfd877339a71821793cbf30845b53964c22320c`, and bootstrap nonce
`c50fcbac0ec1f1f6b79278a9f433807bf6f2336f0c2ed8fa23d6d0d56c2124c7`. It ran once and failed
closed at `verify-guest` with
`read unix @->/run/user/1000/dockpipe/vm/g2r-970fd15c42e793bb/g2c-6013982ee1e49710/g2r-970fd15c42e793bb.agent: i/o timeout`.
The exact live authorization is spent and permanently non-reusable.

QEMU created both exact Unix sockets, and the controller connected to the agent chardev. The
executor created one 60-second verification context and the Linux runner copied that deadline to the
Unix connection. `protocol.ReadFramed` timed out before a complete four-byte-length-prefixed signed
bootstrap. Bootstrap verification, `bootstrap.json`, `verification.json`, and every controller
request were never reached. `qemu.log` was empty; `shutdown.json` was absent; `qemu-img.log` recorded
successful private-clone creation; and the recorded QEMU PID was no longer active. The complete
owner-only instance, evidence, configuration, and runtime roots remain preserved with their clone,
raw disk, NoCloud ISO, seed tree, final identity material, sockets, and executor contract.

A separately authorized offline forensic artifact with SHA-256
`e90fec92a46fb6e3e21ddb923b8a960866bb72437507ef9da92b97b04104fe68` ran once through network and
mount namespaces using kernel NBD read-only and ext4 `ro,noload,nodev,nosuid,noexec` mounts. It
uniquely identified `/dev/nbd15p1` as root, found `/var/lib/cloud` absent, found no cloud-init
`status.json`, `result.json`, or `boot-finished`, found no matching persistent-journal agent entries,
and found no DockPipe-specific udev ownership/mode rule. It did not record the actual runtime
virtio-port ownership or mode. NBD and mounts were detached; the preserved disk metadata tuple was
unchanged. The owner-only report SHA-256 is
`504e8e68acc91ace97eba74a676c1e9675d5a5c1d13a216ed93a27a3ad0e7565`. Earlier v1-v3 forensic
authorizations are spent and non-reusable.

The evidence therefore leaves unprivileged virtio-port access plausible but unproven. The earliest
missing observable milestone is cloud-init/first-boot state, before any evidenced agent-service
attempt. It does not establish a service, device-permission, cloud-init, or boot cause.

The bounded offline production-wiring slice is now implemented under `packages/vm/**` as VM package
version `1.1.0`. `dockpipe.vm.first-boot-observation.v1` is bound into every fresh deterministic
provisioning plan and digest, the sealed executor-v3 digest, the exact QEMU launch argv, the typed
Linux runner, and the new cleanup resource enumeration. QEMU exposes only the existing
`isa-serial/ttyS0` byte stream as a one-shot Unix client with reconnect disabled. Before launch, the
controller creates the exact Unix listener and exclusively creates the cohort
`first-boot-console.log` as mode `0600`; QEMU is never pointed at an ordinary output file.

The controller-owned sink retains exactly the first 4 MiB, fails closed if another byte arrives,
preserves the prefix, and propagates capture, file-sync, parent-directory-sync, and close errors. The
runner deterministically closes and joins capture before guest verification returns and again guards
shutdown and failure preservation. Planning and executor validation bind the evidence/runtime roots,
paths, source, transport, client/listener roles, no-reconnect setting, mode, exclusive creation, cap,
overflow policy, fsync policy, and stop/join requirement. Offline negatives reject path, transport,
mode, cap, replacement, reconnect, lifecycle, operation, and argv substitution before a runner can
be reached. No NoCloud/user-data or guest asset changed, and no private seed/key payload read,
network, generic command, shell, retry, reconnect, fallback, signal, or automatic cleanup was added.

Compatibility is explicit: fresh qualification execution requires executor-v3 and the exact
observation policy, while a stored executor-v2 contract with no observation field retains its
original digest and remains loadable only for separately authorized exact cleanup. It cannot regain
qualification execution. The checked-in live authorization template remains `approved=false`, the
reviewed plan remains `execute=false`, and this slice minted no identity or authorization and created
no live root, disk, seed, QEMU process, socket, or evidence. It did not read or change the preserved
Gate 2 instance. Gate 2 is still unqualified. Renewed source builds/hashes and one fresh separately
authorized Gate 2 invocation are the next live gate; Gate 3 remains blocked.

Offline validation passed `git diff --check`, the complete `GOWORK=off CGO_ENABLED=0 go test
-mod=readonly ./...` and `go vet -mod=readonly ./...` suites, the VM package test, Linux and Windows
workflow validation, and workflow/resolver compilation into a fresh isolated `/tmp` workdir. Focused
observation lifecycle tests also passed under the race detector and across ten repeated runs. They
cover deterministic plan binding, exact QEMU transport, path/mode/cap/overflow substitution,
exclusive owner-only evidence, exact-cap and one-byte-overflow behavior, file and directory sync
and close error propagation including listener-setup failure, goroutine joining, file-descriptor
closure, pre-authorization rejection, and independently checked historical executor-v2 digest and
cleanup compatibility. The sandbox prohibits binding a real Unix socket, so the
listener-ownership unit uses an injected inert listener; no QEMU process or VM was used. No build
intended for live use was produced.

A separately authorized offline source-build/review gate then used the fresh private root
`/tmp/dockpipe-vm-source-review.ZStX82CE`. The complete current source diff and both target dependency
closures were reviewed; every non-standard build input resolves under `packages/vm/tools/**`, with
no `src/**` input. Two independent Linux/amd64 lanes used separate build caches, temporary
directories, and output directories with Go 1.25.0, `GOWORK=off`, `CGO_ENABLED=0`, `-trimpath`, and
`-buildvcs=false`. Both controller outputs were byte-identical at SHA-256
`b3e428bbadd11d1c9576676ad1f7d0769baddf77a256022eda0bbbc6720cf8cc`; both guest-agent outputs
were byte-identical at SHA-256
`7434d3980013e0a978dd73851b4893f1325f6d1c2a27222afcfc20024d46e583`. The separate Windows/amd64
guest-agent compatibility build has SHA-256
`5f8b3b83b373ca5d8e70a63283871bdc1842b5e8bdd14a69191d56d362b2a84e`; it is cross-compilation
evidence only.

Fresh Go build/temp-cache validation passed `git diff --check`, the complete VM Go test and vet suites, focused
observation race tests across ten repeated runs, the VM package test, Linux and Windows workflow
validation, and workflow/resolver compilation into separate isolated workdirs under that task root.
The build outputs remain only under the offline task root and were not promoted, copied into any live
or preserved root, or used to prepare identity, provisioning, plan, or authorization material. No
preserved Gate 2 root was accessed, no live artifact or socket was created, and no VM, cleanup,
Gate 2, or Gate 3 action ran. These hashes are reviewed deterministic source-build evidence only;
any fresh Gate 2 invocation still requires another explicit authorization.


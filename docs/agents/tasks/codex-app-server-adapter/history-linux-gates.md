#### Linux VM Gate 2 system-user correction (2026-08-08)

Live Gate 2 script SHA-256
`1fc9d1074cce63bd5fa7be09557cc6984e9b1255d35f354925f4049c3ccd2f29`
executed exactly once for run `g2r-ff1cc0a230d1f0c2` and cohort
`g2c-1e7fe20cf8ac84ad`; its authorization is permanently spent. The sealed
executor-v5 reached `verify-guest`, timed out after 240 seconds, and preserved
the complete instance. It created no signed bootstrap or verification
evidence, issued no retry or fallback signal, performed no cleanup, and did not
enter Gate 3. The recorded QEMU PID `3439760` is absent. The owner-only
first-boot console is `87817` bytes with SHA-256
`6dee73649a3f94276e7b387880cbac21adb0d82c603ccb47eefb90bf7790895a`.

The console showed both `users_groups` and `write_files_deferred` failing. An
unprivileged read-only sparse forensic copy of the preserved overlay exposed
the exact cloud-init traceback: `ValueError: Not creating user dockpipe-agent.
Key(s) ssh_redirect_user cannot be provided with system`. The NoCloud user
combined `system: true` with `ssh_redirect_user: true`; cloud-init therefore
did not create the account, and the three deferred agent-owned key/config files
could not resolve their owner. The forensic copy modified neither the
preserved overlay nor any live root.

VM package 1.1.3 removes only `ssh_redirect_user` from the locked system user.
It retains `system: true`, `/usr/sbin/nologin`, `lock_passwd: true`, disabled
SSH, `-nic none`, and the existing systemd sandbox. The reviewed asset SHA-256
and package tests pin that corrected shape and explicitly reject the invalid
key. Executor-v6 owns fresh execution. Preserved executor-v5 remains loadable
only for its separately authorized exact cleanup list, alongside historical
executor-v4/v3/v2 cleanup compatibility. No `src/**` file changed, so the
package/engine boundary remains preserved. The spent run will not be retried;
exact cleanup, deterministic source review, promotion, fresh preparation, a
new live Gate 2 authorization, and Gate 3 remain separate gates.

Offline validation passed `GOWORK=off CGO_ENABLED=0 go test -mod=readonly
./...`, `go vet -mod=readonly ./...`, focused provisioning/executor race tests,
the VM package test, Linux and Windows workflow validation, and isolated
workflow/resolver compilation under `/tmp`. Cloud-init 26.1 extracted from the
exact preserved Ubuntu overlay reported the corrected rendered user-data shape
as valid. These checks created no repository-generated state, promotion,
identity, authorization, disk, socket, process, cleanup, Gate 2, or Gate 3
action.

The corrected source was then rebuilt twice from exact checkpoint
`97480d78d3e7a69f22f4d17c6551f6b4d9d877d0` under private root
`/tmp/dockpipe-vm-source-review.97480d78.5adWdyrB`. Independent Linux/amd64
lanes used separate caches and temporary directories with Go 1.25.0,
`GOWORK=off`, `CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and
`-buildvcs=false`. Their controller outputs are byte-identical at `5447246`
bytes with SHA-256
`d43af4d07ce6c338494f0a36acfe9530029fce1545201411387067dd6b1ced43`;
their guest-agent outputs are byte-identical at `3870222` bytes with SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`.
The Windows/amd64 compatibility output is `3966976` bytes with SHA-256
`86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`
and is not a Linux promotion input. Embedded Go metadata matches the requested
platforms and flags, and all non-standard dependencies resolve under
`packages/vm/tools/**`. The source-review artifacts remain unpromoted and were
not used for cleanup, Gate 2, or Gate 3.

The first separately authorized cleanup wrapper was executed exactly once but
its fixed cleanup authorization had expired. It stopped before controller
invocation, removed none of the 12 resources, wrote no result, and was not
retried. A separately approved fresh wrapper then created one 600-second exact
cleanup authorization with SHA-256
`d903cc895e189eccb5facf81ce1f5fdac32adb41bd10fb1340e6740a95ba6dc1`
and invoked only the prior promoted controller's cleanup mode for execution
SHA-256
`594d4cd1c143a1084af3369684e9abaac7cfb5892f8828dd4fb85e6023058c57`.
The controller returned `completed=["cleanup"]`, `cleanup_run=true`, and
`preserved=false`; cleanup-result SHA-256 is
`ff971ac3fc994e72e40886c8f2eb6140e1b4ffe4919023e489d12fed6489ace4`.
Independent read-back confirmed all 12 executor-ordered resources and recorded
QEMU PID `3439760` absent. The immutable executor-v5 promotion controller and
executor-v6 source-review controller retain their reviewed hashes. No retry,
promotion, preparation, live Gate 2, or Gate 3 action occurred.

The next separately authorized offline promotion published immutable
`vmp-2026080897480d78` from exact checkpoint
`97480d78d3e7a69f22f4d17c6551f6b4d9d877d0`. Its closed inventory contains
only controller SHA-256
`d43af4d07ce6c338494f0a36acfe9530029fce1545201411387067dd6b1ced43`
at `5447246` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes. Both are mode `0500`, owner-only, regular single-link
Linux/amd64 executables. Canonical promotion evidence SHA-256 is
`c82161da827842b926bbed834fddbd14957577df5a69ae5d4d640949deb7eac1`.
Independent read-back confirmed the repository checkpoint, source comparisons,
closed inventory, ownership, modes, sizes, hashes, and package/engine boundary.
The promotion performed no identity preparation, live input, plan,
authorization, disk, seed, socket, QEMU process, cleanup, Gate 2, or Gate 3
action. Fresh Gate 2 preparation remains separately authorized.

The separately authorized offline preparation then created fresh task root
`/tmp/dockpipe-vm-gate2-prep-20260808-7fddef13`, run
`g2r-40a86fe85ed7b2f8`, and cohort `g2c-c0805ed9b9d9aff1` from promotion
`vmp-2026080897480d78`. Identity descriptor SHA-256 is
`3738490e1e43882e7f8585f3e77de9c485ebc6c9804c9b43f50ef0543687fbe6`;
qualification input SHA-256 is
`410adcc1056cb9fe1ac8d190e30ff4d414ceeddcf56d54725ff3cd2fd3ddba37`;
provisioning input SHA-256 is
`74b5d529a49da372f9baec334fd4ddc72183ebdb4cb976b986932150ded621ad`;
and inert plan file SHA-256 is
`73eb04cafa4b739ff9225bf95176d6d26987118713d1af6917447f9930b7d163`.
The plan binds contract SHA-256
`010406e2cbdf7903229c7cc344fcf1cdfb99fdc2f43cd52b37a36afc1ace6a6a`
and plan SHA-256
`afa3ac656db78672aea4b03a4fd55561ffb7b050224714598a09dcd6e3ab9881`.
The identity expires at `2026-08-09T18:46:09Z`. The persisted plan remains
`live_authorized=false`, `execute=false`, and `authorization_required=true`;
both the provisioning input and typed `verify-guest` operation bind 240
seconds. QMP, agent, and first-boot-console socket paths are 93, 95, and 92
bytes. All four live roots remain absent. No live authorization, identity
reservation, disk, seed, socket, process, cleanup, Gate 2, or Gate 3 action
occurred. Live Gate 2 remains a separate exact authorization.

#### Linux VM Gate 2 virtio-port access correction (2026-08-08)

Live wrapper SHA-256
`d1f5786f0661ca1a99b06a554c57df9ed06079242f6c07b195052f371f146221`
created authorization SHA-256
`e2bfaed94a54e24fd3825100475f2bc9f373438292a5bc1dd06944709dd78006`
and executed once for run `g2r-40a86fe85ed7b2f8` and cohort
`g2c-c0805ed9b9d9aff1`. The executor-v6 execution SHA-256 is
`0dcc9d5aeeb8a7159749ac2a565b0eacd4cc58091904a593455f509b7d08a5b1`;
executor file SHA-256 is
`dbe53105424f9cd5e973c4ccb4a846a569c9bc02f7778ca3af12acee6175e753`.
Guest verification timed out after 240 seconds, preserved all roots, and
created no signed bootstrap or verification evidence. Recorded QEMU PID
`3947652` is absent. First-boot console SHA-256 is
`04df68c2754ec0121c810fcf48c5821a761e8ee0722286acecf37974c112f641`
at `86993` bytes.

The console and an unprivileged read-only forensic conversion prove every prior
correction worked: cloud-init created `dockpipe-agent`, formatted and mounted
the exact data disk, completed all three deferred writes with UID 999/GID 988,
and started `dockpipe-agent.service` at about 178 seconds. Syslog then records
the exact failure: `dockpipe-guest-agent: open
/dev/virtio-ports/org.dockpipe.agent.1: permission denied`, followed by service
exit status 2. The preserved overlay was not modified.

VM package 1.1.4 adds two exact root-run cloud-init commands before service
start. `/usr/bin/chgrp --dereference dockpipe-agent` changes only the target
group of `/dev/virtio-ports/org.dockpipe.agent.1`, retaining root ownership;
`/usr/bin/chmod 0660` restricts the target to root and agent-group access. The
fix adds no shell, wildcard, udev-wide rule, capability, additional group,
root-run agent, retry, signal, or automatic cleanup. Executor-v7 owns fresh
execution. Executor-v6 remains loadable only for separately authorized exact
cleanup, alongside executor-v5/v4/v3/v2. No `src/**` file changed and the
package/engine boundary remains preserved.

Offline validation passed the full VM Go suite, `go vet`, focused provisioning
and executor race tests, the package harness, both workflow validators,
isolated workflow and resolver compilation, and cloud-init 26.1 schema
validation. Only temporary isolated compilation outputs were generated.

Exact checkpoint `4eb50c762005ae6f10f51cd57daf9790196cd4ac` was then
rebuilt in two independent Linux/amd64 lanes under private root
`/tmp/dockpipe-vm-source-review.4eb50c76.ZsNklPyL`. Each lane used separate
caches and temporary directories with Go 1.25.0, `GOWORK=off`,
`CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and `-buildvcs=false`. Controller
outputs are byte-identical at `5447254` bytes with SHA-256
`9f2e2827cffe6924645a90e7381b804111c5f4ec1c46eaab2c270c85a4b1e0d9`;
guest-agent outputs are byte-identical at `3870222` bytes with SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`.
The Windows/amd64 compatibility guest is `3966976` bytes with SHA-256
`86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`.
Embedded build metadata matches the requested platforms and flags, and every
non-standard dependency remains under `packages/vm/tools/**`. The review root
is unpromoted; no cleanup, live Gate 2, or Gate 3 action occurred.

The separately approved executor-v6 cleanup wrapper SHA-256
`26644340417e14c6cb0c3d2c3c80e96bc373964679a94925f33f68a10da5a715`
executed exactly once and created cleanup authorization SHA-256
`8a5fbdbf19f744121b3ec7ea42b611fd1a5b0cd24972904299ae2370ac14f460`
for only the sealed executor-ordered 12 resources. The pinned controller
returned `completed=["cleanup"]`, `cleanup_run=true`, and `preserved=false`;
cleanup-result SHA-256 is
`55114a525775c99bcc33eab203a598bcd8aba79878e72817a97aced772d46231`.
Independent read-back found all 12 resources and recorded QEMU PID `3947652`
absent. Both the prior promotion controller and executor-v7 source-review
controller remain hash-identical. No retry, promotion, preparation, live Gate
2, or Gate 3 action occurred.

The next separately approved offline promotion created immutable
`vmp-202608084eb50c76` from checkpoint
`4eb50c762005ae6f10f51cd57daf9790196cd4ac`. Its closed inventory contains
only controller SHA-256
`9f2e2827cffe6924645a90e7381b804111c5f4ec1c46eaab2c270c85a4b1e0d9`
at `5447254` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes, each mode `0500`. Canonical evidence SHA-256 is
`6fec8b7a8154f6286fac655fe668184cd9fb23f558bab7aed578dc2a502b5101`.
Independent read-back confirmed the two promoted files and sole evidence file;
all live-action flags are false. No preparation, authorization, cleanup, Gate
2, or Gate 3 action occurred.

The next separately approved preparation created fresh run
`g2r-f3037fbd6df82729` and cohort `g2c-eda2763a444462ef` for promotion
`vmp-202608084eb50c76`. The owner-only identity expires at Unix `1786303837`,
exactly 24 hours after creation. Qualification input SHA-256 is
`8da854d047acee50f8aee4cd9065dc0f5f54be4e2bfbfe1cd3b62f9e98913595`;
provisioning input SHA-256 is
`461cfbc52ce64ebefbdd5b810c2c8eba825bb315cbfe056273621f28ab248764`;
inert-plan SHA-256 is
`18314569a259d85b09a68bc415f9efcc777284c792dd79fee201b55ce9a99187`.
The plan binds contract
`821997f35e04e528edf5d3e8e9a67a2effe82f24cad0f76517a52abaeb9532b3`,
plan digest
`0d1fd14723fc8f504a9b9347343a3d1eb01f56aef9b151ff23b46f379c0b06e6`,
and bootstrap nonce
`fbcd69ca7bc51a50a45ef13a88bba72f9e9dd4ad6514a222cc5dbeedbf0ad5e0`.
It remains non-authorized and non-executing with executor-v7's 240-second
verification window. All four live roots remain absent; no live authorization,
VM execution, cleanup, or Gate 3 action occurred.

The executor-v7 live wrapper SHA-256
`6f5052c05ac61256673360322cf9c3746d3c5091cc25f76bd8be134090977ba1`
executed exactly once with authorization SHA-256
`4a4e71e3d40bc8e4a86592cd77f5152962cfc633d63db32d40dedac8af4fe9e6`.
The guest emitted authenticated bootstrap sequence 1 and signed identity,
health, and launch-hash-pinned results at sequences 2 through 4. Health was
true and both promoted binary hashes matched. Bootstrap evidence SHA-256 is
`77d9adb16aa125dfd1eeb8645537d066b5006ab006cf774bbb6399caae238f84`;
verification evidence SHA-256 is
`1c555ff80801d04fa3274e230bac1ca1e9d6c740cec5225de8a9b51c4af550c7`.

The final controlled-shutdown operation failed closed with `QMP response id
mismatch`. The complete instance and executor-ordered 12-resource cleanup list
remain preserved, while recorded QEMU PID `4150024` is absent. Executor
SHA-256 is
`e2ca1d6f89ffd4f55e55aa51967b7b07d94ff3bdf270753ed4cc41b83d580137`;
first-boot console SHA-256 is
`af5786f74d73c5665a7f753ba236106ed7effe77dd0a75c28821fe547984ea66`
at `87088` bytes. No retry, signal, cleanup, or Gate 3 action occurred.

VM package 1.1.5 corrects the package-owned QMP response loop. The old loop
unconditionally decoded the next frame as the command response even though QMP
may interleave asynchronous event frames. The new loop skips only structurally
valid events, caps them at 64, and retains exact ID matching plus every prior
frame-size, decode, QMP-error, transport, and deadline failure. It adds no
command, reconnect, retry, signal, fallback, or automatic cleanup. Executor-v8
owns fresh execution; executor-v7 remains separately authorized exact-cleanup
only. No `src/**` file changed, preserving the package/engine boundary.

Offline validation passed the full VM Go suite, `go vet`, focused controller
and executor race tests, the VM package harness, both workflow validators, and
isolated workflow and resolver compilation. Regression tests prove a valid
asynchronous event may precede the exact response while a wrong response ID
still fails closed. The compilation outputs remain temporary.

Exact checkpoint `1047d7a44a98c71fd45529d2721a808d659cdfda` was then
rebuilt in two independent Linux/amd64 lanes under private root
`/tmp/dockpipe-vm-source-review.1047d7a4.0i4iOy5k`. Each lane used separate
caches and temporary directories with Go 1.25.0, `GOWORK=off`,
`CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, and `-buildvcs=false`. Controller
outputs are byte-identical at `5447686` bytes with SHA-256
`ae624b6d3c140ccadac34ca1ca2eea509d1b22ece011fd22c811ba2c6bde011c`;
guest-agent outputs remain byte-identical at `3870222` bytes with SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`.
The Windows/amd64 compatibility guest remains `3966976` bytes with SHA-256
`86caf93a18159e8b40275f43b02e2930baa9eaffad76227285df8e0a08f3ea6c`.
Embedded build metadata and every non-standard dependency preserve the
package-local boundary. The review root is unpromoted; no cleanup or live gate
action occurred.

The separately approved executor-v7 cleanup wrapper SHA-256
`7dd37da324571170d1e587441356aa4d5f96cd98c6a307544d6fb057d926be69`
then executed exactly once. Cleanup authorization SHA-256 is
`262595f83f033d7311a6db7343d0b1ceeb296cebd2fc8f65538655a3b03bc676`;
the pinned controller returned `completed=["cleanup"]`, `cleanup_run=true`,
and `preserved=false`. Cleanup-result SHA-256 is
`253cd08488148f3d1508c9892aa12f813b4cd491ac836bd2704929ffbdaa608c`.
Independent read-back confirmed all 12 executor-ordered resources and QEMU PID
`4150024` absent. The executor-v7 promotion and executor-v8 source-review
controllers remain hash-identical. No retry, promotion, preparation, live Gate
2, or Gate 3 action occurred.

The next separately approved offline promotion created immutable
`vmp-202608081047d7a4` from checkpoint
`1047d7a44a98c71fd45529d2721a808d659cdfda`. Its closed inventory contains
only controller SHA-256
`ae624b6d3c140ccadac34ca1ca2eea509d1b22ece011fd22c811ba2c6bde011c`
at `5447686` bytes and guest-agent SHA-256
`3a2d7657e13b6ec30fc8dc268ad977bb248b6598749979edf63223d364cc59e7`
at `3870222` bytes, each mode `0500`. Evidence SHA-256 is
`77567cf76eed3e2bb6b44e960bb0949b2665a5cc98d86cece66bca1707ecfb16`.
Independent read-back confirmed both inventories and every live-action flag
false. No preparation, authorization, cleanup, Gate 2, or Gate 3 action
occurred.

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
and bootstrap nonce
`93ef19893b20e4d154fb2f1e4cd19140b51c0fbb6b13b4db598dc6e8dc19ff4c`.
It is non-authorized and non-executing with all four live roots absent.

The executor-v8 live wrapper SHA-256
`67c4775f43bc61204a902d459ab50e60012e83a9456dfc78f589dbb7677438f3`
then executed once with authorization SHA-256
`7b3cb2a0f69dec174ad36b4573fe543f3b4fa0886b79096f5b9b81a28025bbe6`.
Execution SHA-256
`ab1c2e632f814a5e406e48a8caaafbce103d5a7953a564bf6c2b4009b8b82db7`
completed clone, disk, seed, launch, signed verification, and controlled
shutdown with `preserved=false` and `cleanup_run=false`.

Bootstrap evidence SHA-256 is
`ce19af3864474a0171b1aa20e1a5721aee6f9c57c216e99f86b87e6c57bdf26f`;
verification evidence SHA-256 is
`fc8f9ab92407f32abaca4ab381c3c152d843f08276491fa33099e6774b5ae096`.
Authenticated sequences 1 through 4 prove exact identity, `healthy=true`, and
matching promoted binary hashes. Shutdown evidence SHA-256 is
`f1af02b35fdb9a51946cf1228071ad6db7e6b8885ea273a6af6f984d410e482e`
and records `system_powerdown`, `clean_exit=true`, and PID `67010`.
Independent read-back confirms that PID and transient QMP/agent sockets absent.
Executor SHA-256 is
`cc5f38063a9bf06541b62fe1ab12e4d14dc684cafe3fac8da7b029085b8e5b24`;
console SHA-256 is
`3738cb9fe16cff9ca3570604b2fcb9d8ccf40141551da42f51b966b6f485bb69`
at `87171` bytes. Gate 2 is qualified. No cleanup or Gate 3 action occurred;
Gate 3 is unblocked and remains a separate approval boundary.

#### Linux VM offline Gate 3 durability cohort (2026-08-08)

VM package 1.2.0 implements the package-owned Gate 3 contract without running
it. The fixed cohort covers four application-visible SQLite durability
transitions and three independent attempts per transition. It therefore
requires twelve authenticated checkpoint/recovery trials, twelve hard-power
events, and thirteen boots. Durable pending tickets bind the exact run,
cohort, trial, machine, disk, scenario, transition, nonce, harness hash, and
checkpoint boot ID; recovery requires a distinct authenticated kernel boot ID.

The guest surface is restricted to a root-owned, mode-`0755`, hash-pinned
SQLite test harness and two fixed roles. It permits no arguments, shell,
network access, or arbitrary command dispatch. Guest and controller both
validate canonical nested evidence for the exact expected old/new revision,
SQLite 3.53.3 source identity, native `unix` VFS, metadata hashes,
`quick_check=ok`, and zero retry, replay, repair, or fallback counts. The host
runner rereads the exact QEMU identity after `pidfd_open` and uses only
`pidfd_send_signal(SIGKILL)` for the twelve reviewed cuts. Any mismatch stops
and preserves; a complete cohort uses typed QMP for one final controlled
shutdown.

Executor-v9 is fresh-execution-only and requires a new three-binary promotion,
fresh preparation, and fresh Gate 2 qualification before a short-lived Gate 3
authorization can be created. Executor-v8 is cleanup-only for successful run
`g2r-a29152ab33508801`; that instance and its executor-ordered resources remain
untouched. Promotion, preparation, Gate 2, Gate 3, and cleanup remain separate
approval boundaries. This implementation created no live authorization,
started no VM, performed no hard-power event, ran no Gate 3 action, and cleaned
nothing.

Exact checkpoint `a72789801a83b53761b710388618d7aafc15648e` was subsequently
built twice from independent source extractions under
`/tmp/dockpipe-vm-source-review.a7278980.TuSIQFWC`. Both Linux/amd64 lanes used
separate caches, temporary roots, and outputs with Go 1.25.0, `GOWORK=off`,
`CGO_ENABLED=0`, `GOAMD64=v1`, `-trimpath`, `-buildvcs=false`, and an empty
build ID. All outputs are byte-identical: controller SHA-256
`a8ff286cba55cf03eed2832f26069ea3812f239b6c767c77c1cd4c2cf045bd1a`
at `5721936` bytes, guest-agent SHA-256
`4a2533d297d698328d5875e5af7c1f57d59b0a16b55c84f4a71c6107b5fb38a2`
at `4238382` bytes, and SQLite harness SHA-256
`08b979ab70922c596ea14847ff023357616ebc5c92daee50ecab84ffbcfa3cc5`
at `11895893` bytes. Embedded build metadata, binary versions, and the harness
contract self-tests passed. The review root is unpromoted; it supplied no live
input and authorized no preparation, VM, Gate 2, Gate 3, or cleanup action.

#### Linux VM Gate 2 cloud-final timeout and safe receipt correction (2026-08-09)

Recovered executor-v10 run `g2r-aaaf79e59edac6de`, cohort
`g2c-9546f32de134237b`, against immutable promotion
`vmp-2026080958440ef8` and source
`58440ef8466568b7cce1f1df0a137148a1bcb7e2` began once at
`2026-08-09T22:30:07Z`. Cloud-final began at `22:33:37Z`, the repaired journal
ended at `22:33:43Z`, and `stage=modules-final` persisted at `22:33:44Z`.
The controller exited at `22:34:16.630Z`, only 32.630 seconds after the last
durable guest milestone, after the fixed 240-second `verify-guest` deadline
expired while reading the first framed bootstrap. There was no guest failure,
final-module completion, agent start, bootstrap, or verification result. The
forensic result SHA-256 is
`2a6789cde2f530165821bb61afd5baf0b28b5891b98e6cb35477a45209a20f71`;
the successful replay derivative SHA-256 is
`aa6afb9688d214655aecfa18fa92318d0317e3b91c9c05072776aca5f70c4935`.
The run and all replay authority are consumed; every preserved root remains
untouched.

VM package 1.3.2 seals executor-v11 with a 300-second verification policy.
The last observed legitimate milestone was about 217 seconds after controller
start, so preserving the existing 60-second signed-verification allowance
requires at least 277 seconds; 300 seconds is the smallest closed whole-minute
policy. Clone, launch, shutdown, complete preservation, no-retry, no-fallback,
and separate-cleanup rules remain unchanged. Executor-v10 is retained only for
exact separately authorized cleanup, avoiding reinterpretation of the consumed
contract.

The v11 plan also binds owner-only `verification-failure.json`. On timeout the
controller exclusively creates and fsyncs schema
`dockpipe.vm.guest-verification-failure.v1` before preservation. Its stable
fields are limited to operation, timeout reason and policy, bootstrap-verified
state, and completed public capability names. It contains no path, run/cohort
or boot identity, nonce, frame, payload, key, timestamp, secret, configuration,
or private material. Deterministic tests pin the 300-second policy, exact safe
JSON, mode `0600`, exclusive creation, current v11 path binding, and v10
cleanup-only compatibility.

This was a package-owned offline correction only. No preserved evidence was
re-read, no VM/process/socket/QMP/agent interaction occurred, and no replay,
recovery, cleanup, promotion, preparation, Gate 2, Gate 3, network, install,
commit, or publish action was performed.

The authorized follow-on source review built the current package-conventional
Linux inventory in two independent lanes beneath
`/tmp/dockpipe-vm-source-review.v11.cANo3uZi`. Its exact build closure was base
HEAD `6eb03ff3cf6d9edde37308400d5ba1940895afdc` plus owned build-input diff
SHA-256 `8e33045be0218f0cb57fa34867aad61af71beed6ddcac7aab013efbca0c3db66`.
Both lanes used the locally reviewed Go 1.25.0 binary SHA-256
`b93cdfdbc72f1afc3f21498c80bf3d155a44a9b95e2d690c940511051574bc25`,
with network resolution disabled, `GOWORK=off`, `CGO_ENABLED=0`,
`GOAMD64=v1`, `-trimpath`, `-buildvcs=false`, and an empty build ID. The
controller pair is byte-identical at `5858767` bytes with SHA-256
`f40ede2b6ddaa1b31202a724d5f3325729fddd5b16baab481374e2d96dfd99a0`;
the guest-agent pair is byte-identical at `4245103` bytes with SHA-256
`04c7456f8d94d47deba8babfdce2853fb775fc83c7859b5de910e0227c256713`;
and the unchanged SQLite harness pair is byte-identical at `11895893` bytes
with SHA-256
`08b979ab70922c596ea14847ff023357616ebc5c92daee50ecab84ffbcfa3cc5`.
All are static Linux/amd64 ELF files with the expected Go metadata and package
paths; controller 1.3.2, guest-agent 1.1.0, both harness contract self-tests,
the package boundary, Go test/vet, VM harness, package test, both workflow
validators, and isolated workflow/resolver compiles passed. Canonical owner-only
evidence SHA-256 is
`537fabf0e04702115e73789dfcf268576b926416e712075ff838eb52599ef90e`.
No output was promoted or used for preparation, live execution, Gate 2, Gate 3,
retry, replay, recovery, or cleanup.

<!-- END TASK-013 VERBATIM CLOSED HISTORY BLOCK B -->

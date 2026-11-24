# Remote and Browser qualification

The [feature registry](registry.json) supplies the platform profiles, existing
installed assertions, and additional role checks. The runner expands its
remote and Browser inventory into combinations of registered endpoints. It
reports unsupported feature, platform, and role combinations separately. Lanes
involving Windows Server appear last and start blocked for manual VM setup.

```bash
python3 workflows/qualify/product/assets/scripts/qualify.py --remote-matrix
python3 workflows/qualify/product/assets/scripts/qualify.py --remote-matrix \
  --remote-observations /absolute/private/viewer/observations.json \
  --remote-observations /absolute/private/worker/observations.json
```

- **passed** requires every installed assertion for the exact combination,
  installed artifact digests for both endpoints where applicable, and files
  whose SHA-256 values match the receipt.
- **failed** records an observed installed failure and its evidence.
- **blocked** records a concrete prerequisite or operator action.
- **unrun** means the eligible role combination still needs a complete result.
- **unsupported** means this profile has no implementation for the role.

An observation document has schema
`dockpipe.product-integration.remote-observations/v1` and an `observations`
array. Copy the `feature` and `participants` from the generated lane. For a
pass, set `status` to `passed`, `layer` to `installed_e2e`, and
`passed_assertions` to the assertion IDs observed. Set `artifact_sha256` to
an object keyed by role. Supply `evidence` as an array of objects with a
relative `path` and `sha256` beside the private observation document. For a
failed lane, attach the installed artifact identities, evidence and a short
`reason`. A blocked lane needs its prerequisite in `reason`.
It can include hash-verified readiness evidence before both endpoints have
been installed or selected.
An unrun lane can retain a verified subset in `passed_assertions` with the
same artifact and evidence fields used for a pass.

Keep observations, device identities, and evidence private. Record the exact
operator, broker and worker identities, installed CLI and GUI build receipts,
the platform and display backend, and cleanup for each run. Identify physical
m6 and Win11 evidence explicitly. Pass the observation document to the ordinary
qualification command to include remote coverage in the final result. Repeat
`--remote-observations` for independent receipts. Each evidence path resolves
beside its own receipt. If two receipts cover the same feature and role
combination, select the applicable receipt before running. The report records
each admitted receipt's SHA-256 with its lane and links the full
`remote-matrix.json` by SHA-256. Every applicable role combination must pass
for the full-product result.

Use `--remote-work-plan` with the same observation arguments to group open
checks by the platform environments their installed roles share. Reuse a
verified package installation and a fresh fixture set for each batch. Separate
batches can run concurrently with separate VMs, containers, brokers, and
artifact directories. Schedule batches that share physical machines, native
operator authority, or hardware keys around those shared resources. Failed
checks remain visible for investigation before retry.

The work plan also lists shared blocker reasons and hands-on checks alongside
the matrix counts. A lane can require several prerequisites, so those counts
can overlap. The hardware-key ceremony, native authentication, attached
displays, mixed DPI, and manual Windows Server VM setup appear with the
platform where they apply.

Native Windows operator lanes require a local broker that can authenticate the
Windows user and issue its scoped operator proofs. The Windows adapter reads
console lock and sleep state for workers. Its authentication entry point returns
an unavailable error on both amd64 and arm64. A physical Win11 worker can still
publish its granted capabilities to a supported broker. Installed local Browser
checks use the Windows desktop without remote operator authentication. Windows
Server VM setup is a separate prerequisite.

The physical Windows 11 Enterprise x64 build 26100 returned this distinction
through an exact diagnostic workflow. After the Linux broker was unlocked,
the existing Windows worker accepted delivery. The installed 0.6.39 CLI
reported native operator authority unavailable and locked because Windows
lacks its OS-owned security storage. The same readback found no configured
Windows broker. The Win11 worker still had no live monitoring snapshot, and
the broker's desktop catalog was empty.

After the operator host rebooted, the broker retained its saved enrollments
but the public tunnel returned HTTP 530. The broker's failed activation
journal prevented automatic connector startup. Restoring the existing tunnel
brought back fresh snapshots from physical m6 without a worker restart.
The approved broker-managed recovery verified its pinned process, owned
readiness endpoint, and signed public round trip. Physical m6 continued to
report fresh snapshots after the temporary connector stopped.
The running worker PID matched its persistent macOS user LaunchAgent, and
the saved LaunchAgent definition identified the same job.
The connector later recorded a failed supervision status while its process
still reported four ready connections, the public endpoint still reached
the broker, and m6 continued sending fresh snapshots. Recovery across a
subsequent broker restart remains unverified. Four later fresh signed public
health challenges passed while the recorded supervision status was failed.
Four subsequent read-only runs of the full watchdog check verified the
approved executable digest, ownership of the readiness listener, and a fresh
signed public broker response. The check did not establish what triggered the
earlier failure or change the recorded supervision status.
The installed Pop!_OS viewer displayed live system and process data from
physical m6. Selecting its separately enrolled test guest cleared the values,
and selecting physical m6 restored them. The physical Mac's Docker inventory
remained unavailable. A limited exact Terminal check returned native macOS and
ARM64 identity and confirmed the terminal session closed. The live worker was
bound to the private `dev+de30f81bcfb8.dirty` build. The complete Monitoring and
Terminal assertions still need qualification.
A later physical m6 host Terminal probe acknowledged generated input and
resized the same PTY from 160×24 to 101×37 in its granted root. A fresh session
used a different reference and PID and verified the first PID was gone. Both
sessions closed with exact receipts while the worker PID and generation stayed
live. The operator invoked the byte-matched private DEB CLI directly.

A fresh typed m6 Containers snapshot later reported available inventory
without truncation or notices. The installed Linux desktop also rendered
the m6 inventory in its read-only machine picker. The m6 worker rejected
the proposed disposable fixture delivery under its existing no-delivery
policy, so this check created no container and did not exercise start or
stop.

The Pop!_OS 22.04 viewer subsequently used its installed alternate-root GUI
and CLI to open the same m6 Terminal grant in a dedicated X11 session. The GUI
rendered a generated marker and preserved the same broker reference and shell
PID across a panel switch and resize from 118×50 to 95×37. A fresh GUI terminal
used a distinct reference and PID, verified the original PID had exited, and
returned an exact closed broker receipt. The original status became unavailable
after closure, so its cleanup evidence is the fresh shell's PID check. This
covers the granted host shell, generated input and output, and exact closure.
Interrupted transport recovery and container Terminal remain open for this
pairing.

A separate installed CLI check confirmed the m6 Terminal grant boundary. The
broker refused a generated, ungranted root and its exact status read with
HTTP 409. The existing review grant opened a different session, acknowledged
the generated marker, and returned an exact closed receipt. The worker PID,
generation, and Terminal epoch matched before and after.

A separate physical m6 Files probe used the existing write grant for that
isolated repository. It created a new empty file, checked read bytes and
revision, wrote exact binary bytes at that revision, and verified that a stale
write was refused without modifying the new bytes. The installed worker returned
a generic native rejection for the conflict. The saved checkout includes the
specific stale-revision mapping, which still needs a newer installed m6 worker
test. Exact-revision deletion and follow-up absence verified fixture cleanup.
The operator invoked the qualification CLI, so this is partial Files evidence.

A later installed Git status read against the same m6 review grant returned an
unknown outcome. The worker log showed that its Features executor had stopped
on journal capacity before that request. Terminal remained live and still
advertised the grant. The saved checkout now drops acknowledged read results
and rejects writes before execution when the mutation journal is full. A
focused broker and worker test verified continued reads after both cases.
The matching private `0.6.44+qualification7` DEBs then upgraded normally on
Mint. The installed worker acknowledged 128 reads without creating a durable
journal and executed 128 writes to fill it. It rejected one additional write
before native execution, served reads before and after that rejection, and
kept the same process and journal bytes. The physical m6 worker is still
running the older executable. A private Darwin ARM64 candidate was cross-built
and its deployment requires an independent owner session because the service
replacement closes the worker's own Terminal.
The m6 preflight confirmed that Remote Login is disabled and the separate
test pairing has no available runtime path. The private candidate's embedded
ad-hoc signature matched all its code pages. Native Apple trust assessment
and execution remain open.

A fresh paired readback found the same physical m6 process and pinned
executable. The unchanged durable journal contained two records, and the
bounded worker log still showed a stuck Features executor after its
journal-capacity stop. The live Terminal worker refreshed the shared catalog
and continued to advertise the Files and Git grants. The supervised worker
replacement needs a separate owner session. The two installed Files and Git
failures remain open on this pairing.

The owner-session replacement procedure pins the private Darwin ARM64
candidate, preserves the registered PATH, and checks that rollback can
restore the old LaunchAgent registration. Six focused service tests passed,
including the simulated macOS launchctl failure path. Native activation and
rollback have yet to be exercised.

In a later m6 Monitoring readback, system metrics were live and Docker
inventory reported that its executable was unavailable. The installed
LaunchAgent's saved PATH did not resolve Docker, while the executable at
the standard Apple Silicon Homebrew path was present. Its worker service
had no explicit Docker executable override. A subsequent physical check
confirmed that the Homebrew executable reached a running Docker daemon and
the installed typed Containers snapshot still failed at executable lookup.
The candidate checks standard macOS Docker locations after the captured PATH.
The Darwin ARM64 CLI cross-built and passed native signature verification,
but launchd rejected its activation and the original worker was restored.
An owner-session recovery pinned the standard Homebrew Docker executable in
the existing worker's LaunchAgent. The original pairing and grants were
preserved. The paired operator CLI then listed an isolated generated container,
started it, read its exact log marker, and stopped it. Native cleanup verified
that the container and transfer files were removed. The Dockpipe desktop List
view also displayed the paired m6 inventory. Native activation of the new
CLI and GUI lifecycle actions remain open.
On the same paired physical worker, the installed operator CLI listed Files,
read Git status and received live System metrics. These are partial CLI
observations. The installed GUI Git mutation and Browser reconnect checks
remain open for this physical combination.

The installed Pop!_OS Files GUI subsequently created the exact generated file
and saved a marker that both the Files CLI and the native Mac read back byte for
byte. Two subsequent Files reads cancelled before the peer write step. A scoped
native Terminal edit set the peer revision. The stale GUI Save remained
uncertain through refresh, cancellation and cleanup verification. The peer
bytes survived and the owned file was removed after the exact broker journal
record said cancelled and its deadline passed. An authenticated status probe
returned HTTP 409 even though the journal retained that record. The saved
checkout now permits exact status reads of expired terminal write receipts
while the journal retains them. In the running offline Mint VM, ordinary
`dpkg` upgraded the matching CLI and desktop packages to
`0.6.44+qualification6`. The installed broker authenticated through native
Polkit with the default lock policy. With disposable enrolled identities, the
installed CLI retrieved a metadata-only cancelled write after expiry and
rejected the wrong generation and a replay. The installed worker also rejected
a stale revision with the expected refresh diagnostic, preserved the peer edit
and removed the generated file at its current revision. The CLI retrieved the
expired failure by its exact reference. The private packages reused the prior
verified desktop binaries. This proves the installed CLI, broker and worker
path on Mint.

The installed Mint GUI then reproduced a second failure. The refreshed broker
receipt confirmed cancellation, but the dialog showed a generic message and
kept the operation in its uncertain list. After a normal desktop DEB upgrade to
`0.6.44+qualification6.1`, the GUI performed an exact-revision Save and then
issued one interrupted Save against a newer peer edit. Refresh displayed the
confirmed cancellation, requested a file refresh before the next write, and
removed only the confirmed entry. Independent readback verified that the peer
bytes survived and the generated file was removed. A separate installed CLI
check used independent read-only and writable roots. The read-only write never
entered the broker journal. After the old worker exited and its catalog entry
expired, a replacement with the same enrolled identity published a new
generation. The broker cancelled the queued write on the old generation before
dispatch and rejected a fresh request aimed at that old generation. Reads
resumed on the new generation and exact cleanup passed. The same-guest Mint
Files row passed all six assertions. The GUI retry on the physical pairing
remains open.

The installed Mint operator and worker also passed all seven Remote Security
assertions. The broker denied an unauthenticated catalog query and remained
locked when the native Polkit prompt was cancelled. It permitted the granted
read after authentication and rejected the ungranted write. Explicit lock and
the native Cinnamon lock each revoked access. Returning to the desktop did
not restore operator access. Five authenticated queries did not renew the
default inactivity deadline, which expired after 300 seconds. A separate deep
suspend and wake run recorded the guest kernel's suspend delta, and the broker
reported sleep/resume revocation and denied the next unauthenticated query.
Both runs preserved the default policy and removed their exact fixture files.

The installed Mint 22.2 viewer opened Remote Desktop and its provider setup.
The guest initially had no native X11 viewer provider. The locked Linux
provider producer subsequently passed 42 X11 CTest targets and the Go channel
tests. The installed CLI accepted the checksum-pinned private package from a
guest-only HTTPS catalog. The installed GUI listed `dockpipe.desktop.native`
0.1.0 as Installed. Both checks used the offline guest's isolated user store.
They establish package installation and discovery. On physical m6, the native
sender LaunchAgent was initially stopped, with exit code 1.
Its last sender diagnostic recorded `directive/invalid-directive` and verified
cleanup. The diagnostic covers several bridge and directive failures and lacks
the detail to identify the cause. A background Launch Services status probe
using the exact signed sender app reported Screen Recording, Accessibility,
and event posting granted. The probe exited, its transient files were
removed, and the original descriptor digest matched. The separate power
companion was running, and macOS reported one online display. One supervised
start of the existing sender job then kept the same process running. Three
fresh broker reads returned the same provider generation with view and input
available. Capture, input and clipboard still require an installed paired
session. The offline Mint guest has no authenticated route to the live m6 broker.
The current package contract fixes one display at session admission. Display
switching and mixed DPI remain unverified.

The running Mint guest also reproduced the Qt Multimedia load failure on
`0.6.44+qualification5` and upgraded the desktop in place with ordinary `dpkg`
to `0.6.44+qualification5.1`. Package ownership and manifest digests verified
both Qt libraries. The same native provider resolved all its relocations and
loaded against the installed runtime with `RTLD_NOW|RTLD_LOCAL`. The CLI and
provider archive digests matched before and after. The restarted launcher
still listed the installed provider. This check exercised local package
installation and loading in the guest.

The Pop!_OS 22.04 alternate-root viewer reached the same m6 broker with the
existing OS-user authority. Its first private package failed to load the native
provider because `libQt6Multimedia.so.6` was missing. The amd64 Linux runtime
packager now includes the pinned Qt Multimedia closure. The upgraded private
DEB passed the 72-ELF runtime gate, loaded the same provider, and decoded live
H.264 over the encrypted relay in View mode. Provider discovery required opening
a disposable project even though the provider was installed in the user store.

The viewer requested a 1280×720 virtual display. m6 reported a single online
1920×1080 display before the session, a single 1280×720 display during it, and a
single 1920×1080 display after disconnect. The readback did not expose display
IDs. The returned frame showed existing desktop windows, so the test stopped
without sending input or using the clipboard. The viewer reported verified
remote cleanup, and m6 was still available in the broker catalog. This records
partial installed view and closure coverage. It leaves display identity,
switching, fullscreen, mixed DPI, input, and clipboard journeys open.

A subsequent signed read-only check found the ordinary physical m6 worker
still publishing fresh Monitoring data. Neither paired Mac appeared in the
Desktop catalog. The physical m6 sender LaunchAgent remained registered and
stopped with exit 1. Its last diagnostic reached the native directive loop
after the CLI bridge handshake, then reported `directive/invalid-directive`
with verified bridge cleanup and no capture callback. The current diagnostic
combines stream closure, invalid framing and rejected directives, so it cannot
identify the failure that closed the stream. The physical session reported one
online 1920×1080 display. Multi-display, mixed-DPI, input and clipboard checks
still need an isolated graphical session with the required topology.

Physical Win11 resumed after the operator started its existing worker in the
paired Windows account. The installed Windows 0.6.39 CLI accepted two
digest-checked deliveries. It returned the Windows amd64 build 26100 authority
receipt, initialized a disposable project, ran a native PowerShell workflow,
and returned a matching output digest with exit code zero. Startup readback
found one interactive worker from the installed CLI and zero matching Dockpipe
scheduled tasks or current-user Startup files. Win11 Monitoring and the
separate m6 test guest still lack live snapshots.
The physical Win11 worker later accepted the same read-only authority
diagnostic unattended. The broker returned no Win11 feature target, no
Networking target, and an unavailable Monitoring snapshot. This check
establishes continued workflow delivery on the foreground worker.
The installed Win11 CLI independently retained the generated project's
successful PowerShell run and returned two advancing samples of aggregate CPU,
memory and disk data. The query was scoped to that project and excluded host
process discovery. Paired Monitoring and GUI live process checks remain open.
The installed Win11 CLI also selected its installed terminal helper and
started a hidden native ConPTY shell. It acknowledged resize and exact input,
returned the generated output and verified that the original shell exited.
The visible Terminal widget and paired Terminal path remain open.
The installed Win11 workflow validator accepted a valid generated PowerShell
workflow and rejected malformed YAML, an unsupported working-directory scope
and an unsupported shell with actionable diagnostics. It left the files
unchanged and executed no fixture code.
The installed broker operator downloaded the exact Win11 workflow result and
verified the artifact checksum. The receive-only Win11 installation could
retrieve neither its own result nor a separate known fixture result. Both
requests failed before broker access and left no download. Authenticated
operator access across targets is a separate check.
An installed Win11 workflow later emitted two markers over a twelve-second
run. Its final downloaded log contained both markers once in order. All 15
live preview requests failed at operator signing. The running broker binary
contained the normal jobs route and lacked the live-log route. The exact
installed Win11 worker contained the final-result route and lacked the live-log
publish route. Both artifacts need compatible builds before rerunning native
worker publication and GUI live logs on this pair.
The later physical Win11 0.6.45 SCM worker and compatibility broker passed
the CLI live-log check. Authenticated reads returned both generated markers
while the job was running. The final downloaded log contained each marker
once in order. A separate live job accepted cancellation after publishing its
process ID. The broker reported cancelled and native readback confirmed that
process had exited. The Linux desktop's remote jobs panel showed live progress
for a second disposable Win11 job, cancelled it through the GUI, and showed
the matching cancelled broker state.
An exact read-only diagnostic verified all 11 required Windows Browser runtime
files in the installed launcher tree. Qt Core and WebEngine reported 6.10.3.
The normal launcher shares the signed-in user's Qt profile even when started
with its second-instance flag. Physical Win11 Browser interaction remains unrun
pending a disposable Windows login or VM.
For Win11 as a Browser worker, the installed Linux X11 desktop routed to a
temporary site on the service-owned Win11 worker's loopback interface. The
fixture recorded the submitted text and three requests for its second page,
including a reload after switching away and reselecting Win11. A native
readback confirmed that the temporary route, listener and launch task were
removed. The worker needed a new enable transition after the Windows desktop
was unlocked, before the test route could be advertised.
The Windows host still had the expected QEMU tools and the evaluation disk.
A subsequent read-only check hashed the complete disk on that host and matched
the original transfer receipt. None of the named qualification QEMU processes
was running during the process check.
The earlier WHPX run reached the guest lock screen and an SSH banner. No
authenticated guest channel or signed-in test desktop was established. A
writable overlay and an attached owning VM run are needed for further checks.
The installed QEMU 11.1.0 advertises WHPX, and the known firmware, packaging
tools and OpenSSH client are present. The supported interactive setup opens
the GTK console on the Windows host and keeps the VM attached to its owning
job. No remote connection to that console has been verified.
The saved `m6-vm-test` pairing also returned no live Files, Network or Desktop
catalog entry. The separate x64 evaluation disk on m6 passed a read-only QCOW2
metadata check. Its guest and graphical session remain unverified. The existing
Dockpipe Darwin QEMU backend uses x64 emulation. The host's separate ARM64 QEMU
reports HVF support, with the Windows ARM64 guest boot still pending.
The m6 hardware and system Virtualization framework are present. The bounded
qualification preflight found no installed macOS VM manager in the checked
locations and no macOS guest or restore image in the dedicated qualification
directory. The isolated Desktop sender needs a supported Darwin guest runtime,
a dedicated graphical login, and guest-side sender consent and pairing.

The pinned Darwin ARM64 Browser SDK passed checksum, publisher attestation
and inventory verification. The physical m6 reports active Apple Command
Line Tools and native CMake. The checked standard paths had no full Xcode
toolchain or Qt 6.10.3 development manifests. A matching native desktop
build remains blocked.

Apple's CDN responds for the restore image that matches the physical m6's
macOS build. Guest compatibility still needs verification through
Virtualization.framework on m6. The image body was not downloaded and the
isolated guest did not boot in this check.

The pinned ARM64 Tart archive matched its published checksum. The embedded
signature and virtualization entitlement verified locally. Its certificate
chain verified offline against Apple's published Developer ID certificates. The
dedicated m6 qualification volume had 141 GiB available, with Apple's
code-signing and Gatekeeper tools present. Native Gatekeeper verification
still requires the exact bundle on m6. The runner configures one
display. The mixed-DPI case still needs a runner that exposes two displays
to the guest. [Tart display configuration](https://github.com/openai/tart/blob/6430e84ab0bb4f3fede93abb9a91402800b20e06/Sources/tart/Platform/Darwin.swift#L82)

Win11 accepted another delivery through the recovered broker-managed tunnel.
The installed worker failed to invoke the package script at a 264-character
path with the longer job ID. The same digest completed with a shorter job ID
and a 228-character script path. The Windows staging fix kept an equivalent
test path to 242 characters. A native probe on the physical host invoked
PowerShell at that path, matched the marker, and rejected duplicate staging.
The installed paired worker still uses 0.6.39. Windows UI Automation verified
that the exact installed launcher and UI library were running in a visible
interactive window. Interactive GUI workflow and Browser checks remain unrun.

Windows Installer reported one registered Dockpipe 0.6.39.0 product, with
the CLI and Launcher features installed locally. The paired Windows user's
Start Menu link and PATH resolved to the installed artifacts. The retained
evaluation guest still needs an authenticated disposable login and a matching
desktop MSI for the fresh-install, upgrade, and interactive checks.
The read-only build preflight on the physical host found no matching Qt,
SpatialWeb Browser SDK, MSVC, WiX, or prepared desktop stage at the expected
paths. The x64 MSI can be produced on a dedicated Windows build host with
those inputs before opening the disposable interactive session.

A separate signed workflow reached the same installed Win11 worker and
verified that its executable contains the Monitoring publish route. The
operator still received no Monitoring snapshot, and the worker was absent
from the live feature and networking catalogs. The four checked standard
Docker executable paths and two expected services were absent. Live
Monitoring requires confirmation of the worker's opted-in scope. Containers
requires a supported provider.

A later readback of the successful Win11 CLI delivery verified all three base
installed workflow assertions. Resubmitting the exact snapshot under its
original ID returned the original result. Submitting a different snapshot
under that ID returned HTTP 409. The downloaded marker matched the original
result hash and an independent worker readback found its original write time
and no remaining processes from that run. The GUI streaming and in-flight
reconnect assertions require a compatible installed broker and worker pair.

A signed native host workflow on the physical Win11 `0.6.39` worker also
passed all three base CLI assertions. The broker returned the signed result
and the exact 13-byte fixture, including a NUL and high bytes. The worker
verified the artifact working directory, source working directory, step
environment, and dotenv output. This run exercised the installed CLI path.

The installed CLI observed a separate native PowerShell job running and sent
a signed cancellation for that job. It returned cancelled after 4.09 seconds,
before the planned 24-second finish. An independent worker readback confirmed
the exact process had exited, and the original cancelled receipt still
matched after the planned finish time. This covers CLI cancellation. The
combined GUI live-log and cancellation assertion remains open.

The installed physical Win11 CLI passed local Files create, read, edit,
rename and delete with exact bytes. It refused stale revisions, traversal and
a junction escape, and preserved the outside canary. It also passed local
Git review, diff and selected-file commit with an isolated, hash-verified
MinGit provider. The stale review was refused and unrelated staged and
working bytes were retained. Both fixtures were removed. These results cover
the local CLI surfaces. The remote Files and Git roles require separate work.

For `profile-restart`, record the new process identity after quitting the
original launcher. In the local temporary profile, verify that website data
survives a page reload and clears after a full restart. Window reopening
verifies `session-reopened`. The desktop tray can keep the original launcher
active when the window closes.

The installed X11 Flatpak viewer and the native Pop!_OS 22.04 X11 DEB candidate
both passed the Browser journey with a physical macOS ARM64 worker. Their
private receipts cover page navigation, form submission, route loss, and
reconnection with the original tab restored. The Pop!_OS DEBs were installed
and upgraded in a private package root on the physical host using its installed
dependency libraries. Both runs used bounded foreground workers under the
existing Network grant. Background route recovery still needs its own run.

The installed Pop!_OS CLI used the persistent physical m6 worker for two
concurrent relays to one generated loopback service. Swapping proxy credentials
returned HTTP 407 without reaching the worker. Both intended requests arrived,
the trust snapshot matched the exact paired identity and generation, and the
relays and fixture closed. In a separate check, the m6 loopback endpoint
refused connections. Both HTTP GET and CONNECT returned the remote-dial denial. A
local listener at the same address passed its direct control check and
received no relay request. Worker-only DNS beyond localhost remains open.

An isolated Pop!_OS 22.04 DEB viewer also reconnected to a new generation of an
installed Pop!_OS Network worker on the same host. The original tab and its
site marker returned, and the test app recorded a form submission after
reconnect. The viewer kept its local and remote profiles separate. The run
verified eight of nine assertions. The combined profile and TLS trust assertion
remains open because trust sharing was off.

A follow-up with an installed Pop!_OS viewer and a rebuilt worker verified
explicit CA sharing with a real HTTPS app. The local profile still rejected
that CA. Removing the shared CA retired the route and its trust snapshot.
Restoring the CA on the same paired worker brought back the original remote
tab and its site marker. The private `0.6.39+qualification7` DEB viewer and
worker subsequently passed all nine installed Browser assertions after an
upgrade from 0.6.38. The app recorded one HTTPS POST through reconnect. The
installed viewer rejected a hostname mismatch and the closed route made no
local fallback request. The package-owned CLI also passed a host workflow.

The installed Pop!_OS viewer passed all nine Remote Browser assertions with
the MSI-installed Network worker in a disposable Windows 11 x64 VM. The
guest upgraded to the private 0.6.40 MSI, ran a PowerShell workflow, and
launched the installed GUI. Locking the guest retired the Browser route.
After console unlock and worker restart, the original HTTPS tab and site
marker returned. The app recorded one POST. A separate host-local listener
confirmed the closed route made no fallback request.

The private 0.6.40 Windows x64 MSI passed eight installed local Browser
assertions. DevTools was absent and switching tabs discarded an unsubmitted
address draft. Rebuilding the native SpatialWeb SDK and Dockpipe MSI resolved
both failures in private 0.6.41. Windows Installer upgraded the same guest
from 0.6.40. The installed CLI and GUI ran separate PowerShell workflows, and
all ten local Browser assertions passed. The 0.6.40 failure receipt remains
available for that artifact. A fresh 0.6.41 overlay also passed all nine
Remote Browser worker assertions with the installed Pop!_OS viewer. The
worker reconnected on a new generation after Windows console unlock. The
original HTTPS tab and site marker returned after refreshing the machine list.

A Pop!_OS package viewer on an isolated X11 display reached the physical
Win11 x64 worker with a temporary native CLI candidate and an exact loopback
destination grant. The operator proxy reached the generated HTTP app, refused
the same endpoint addressed by an unapproved hostname, and reopened the
route with a fresh proxy. The installed desktop displayed the app, submitted
keyboard input and rendered the returned text, and followed the second-page
link after refreshing the picker for a new worker generation. The first
desktop attempt lost the route during navigation. The repeat used a longer
fixture lifetime and passed these assertions. The same viewer switched to
This computer, submitted keyboard input to a separate Linux loopback app,
and followed its second-page link. These checks cover generated HTTP apps.
The full installed MSI Browser assertions remain open for the physical
pairing. The current embedded remote viewer is supported on Linux X11.

The installed Alpine 3.24.4 Docker worker could not start Remote Browser
because the Linux OS adapter could not verify local lock or sleep state in
the headless container. The installed Pop!_OS viewer showed the worker as
unavailable. A container-local HTTPS request verified the app and fixture CA.
The result applies to the headless Docker worker configuration.

The same Pop!_OS DEB candidate also passed all ten local Browser assertions,
including address draft retention, opening and using DevTools, remembered
permission denial, and the extension lifecycle. Keep the 0.6.38 failure as
historical evidence for that release when selecting the candidate receipt.

The physical Windows 11 x64 worker accepted a digest-pinned workflow from the
Pop!_OS operator CLI. The installed Pop!_OS GUI cancelled a separate exact run,
and a worker-side check confirmed its PowerShell PID was gone. The paired
worker was absent from the installed GUI's machine selector and the live
feature catalog, so initial GUI delivery failed for that operator and worker
combination. The rebuilt picker delivered a real workflow to delivery-only
workers on Linux and a disposable Windows 11 Enterprise Evaluation x64 VM
running build 26300. The isolated Linux broker received the exact artifact from
the staged Windows worker binary. The physical Windows lane still needs its own
rerun with the rebuilt stack. Live log streaming and worker reconnection remain
unrun.

The initial physical m6 readiness inventory reported one online 1920×1080 display at
60 Hz. Its native desktop worker was registered but stopped with exit code 1,
and it was absent from the broker's live desktop catalog. The desktop power
companion was running. Provider readiness initially blocked both viewer lanes.
The later supervised start and installed Pop!_OS view established partial
coverage on that pair. The attached-display switching
and mixed-DPI assertions also need a suitable second display. Historical
package runs independently verified generated clipboard round trips and all
four virtual display presets with exact cleanup.

A later paired read found no live physical m6 Desktop entry. The registered
native sender was stopped after exit 1. Its last diagnostic reported
`directive/invalid-directive` and verified cleanup. The ordinary Terminal
worker still responded. The native sender's LaunchAgent keeps successful
exits alive, so this failed exit left the sender stopped. The diagnostic
does not identify the underlying failure. The dedicated qualification
directory contained no macOS guest or restore candidate, and the paired test
guest advertised no Desktop provider. Interactive qualification stays blocked.

The installed Pop!_OS System view rendered fresh m6 metrics and processes.
Switching to Win11 cleared the m6 values and reported unavailable monitoring.
Switching back restored the m6 view. Machine monitoring shares system counters,
process metadata and authorized Docker inventory. Workflow execution records
remain private to the worker and job results are qualified in the workflow lane.
The m6 sample had no available Docker inventory, so the full monitoring case
remains unrun. Win11 monitoring remains blocked on a live snapshot.

The installed Pop!_OS DEB initially hid the Containers monitoring selector when
the workspace was local. A rebuilt launcher in a private Pop!_OS installation
passed against an installed Ubuntu 24.04 worker and an isolated broker. System
and Containers showed only the selected worker's data. Switching to a paired
node without sharing cleared both views. The worker published the running test
container and its Docker networks. The broker denied a typed stop request and
the container stayed running. Revoking monitoring on the worker cleared both
views. Preserve the original failure for its launcher digest.

The Pop!_OS operator candidate and installed Ubuntu 24.04 worker passed Remote
Files against separately granted roots containing the same filename. The GUI
saved the writable file against its reviewed revision and displayed the conflict
when a newer worker edit made that revision stale. The broker denied a write to
the read-only root. After the worker restarted, the GUI reopened the newest
bytes, and an exact retry returned the original receipt without replaying its
write. The isolated run verified worker, file and VM cleanup.

The installed Alpine 3.24.4 Docker worker passed all six Remote Files
assertions after upgrading its APK from 0.6.39 to private 0.6.44. The installed
Pop!_OS viewer verified the separate grants, exact-revision save and stale
conflict. The restarted worker returned the original write receipt without
changing the newer bytes, which the viewer reopened. Docker verified removal
of the fixture container and bridge. The 0.6.39 worker returned a generic
stale-save error and retains a separate failure receipt.

The installed Flatpak viewer and private 0.6.44 RPM worker passed all six
Remote Files assertions on Fedora Workstation 44 under GNOME Wayland. Both
roles ran as separate paired identities in one offline VM. The first attempt
timed out during GUI reattachment. On retry, refreshing the machine list and
reselecting the writable grant reopened the newer file after the worker
restarted. The exact write retry returned the original receipt. The GUI and
worker exited, both fixture roots were removed, and the VM's backing hashes
matched after shutdown.

Fedora Workstation 44 also passed the six Remote Git assertions with the
installed Flatpak and native 0.6.44 RPM in a fresh overlay. The GUI opened
separate Git grants and committed only the selected file. The broker
returned the original commit receipt after the worker restarted, and the
GUI reopened the same commit in History. The read-only repository and
unrelated changes kept their original hashes. The first Flatpak offered
Stage on the read-only root and the broker denied the write. The upgraded
Flatpak disabled Stage, Discard, Commit, Pull and Push on that root.
The writable grant repeated the selected commit and receipt recovery.
The updated Flatpak bundled the same verified CLI binary as the RPM.
Both fixture repositories, the worker and the GUI were cleaned up. The
parent image hashes matched, and the VM remained running.

The same Fedora installation passed Remote Workflows. The installed Flatpak
selected a delivery-only worker, previewed the bundle, and downloaded the
completed run's verified artifact through the desktop file picker. A second
job resumed live output on the same assignment after a four-second relay
interruption. The GUI cancelled the job and showed the terminal state on
refresh while the broker was online. The guest verified that the original
process had exited. Both roles ran in one offline VM with fixture operator
authority. The worker, relay, broker and viewer exited, the parent image
hash matched, and the VM remained running.

A reverse cross-guest run used the Fedora 44 RPM broker and CLI, the installed
Fedora Flatpak with bundled CLI 0.6.38, and the Mint 22.2 DEB worker. The
Flatpak previewed and submitted a real workflow, downloaded its result, full
log and artifact, and separately cancelled a running workflow. The RPM CLI
download of the completed GUI job matched all three files byte-for-byte. In a
separate CLI run, the worker relay dropped for four seconds and refused nine
connections. Live output resumed under the original assignment and worker
PID without replay. The CLI then cancelled that job. Mint verified all four
workflow PIDs had exited, and both VMs stayed running after fixture media
were detached. This covers five of six assertions. Live worker output
remained unverified on the released Flatpak.

The upgraded private 0.6.44 Flatpak selected the installed Mint worker and
streamed live output while its workflow was running. After a four-second cut
of the isolated worker relay and nine refused connections, the same selected
job continued showing new steps. The GUI cancelled the job. The broker's
stored result confirmed one workflow start, 108 ordered steps, and no success
artifact. A second GUI submission completed all 18 steps on Mint and the
installed GUI downloaded its result, log and artifact through the Fedora
portal. The log and artifact hashes matched the broker's stored results. Mint
confirmed both workflow PIDs exited. All six installed workflow assertions
passed for this exact combination. Both VMs stayed running and the test media
were detached. The first launcher attempt had selected the local machine and
the fixture's Mint-only guard rejected it before executing the workload.

A private SDK candidate built the Spatial-Web session and trust changes on
the owning development commit. It passed seven Browser suites, a relocated
consumer compile and run, and real DOM execution in an installed Flatpak with
Chromium's default sandbox. The KDE SDK, Platform and WebEngine BaseApp
commits matched the existing pins. The local proof records the exact software
renderer used after the older renderer import failed. Dockpipe's private SDK
admission verified the same archive after relocation. The public SDK pin
remains unchanged.

A separate run used the installed Mint 22.2 DEB operator and broker and the
installed Fedora 44 RPM worker in two VMs. The point-to-point virtio network
had no default route or host listener. The Mint GUI previewed and delivered
the workflow, showed live Fedora output, and downloaded the result, log,
and artifact. The independent CLI download matched every byte. A second
job resumed live output under the same job ID after a four-second worker
relay cut. The GUI cancelled it and showed the terminal state. The full
worker log contained one start and each step once, and Fedora verified both
exact workflow processes had exited. The fixture services and media were
removed. Both VMs stayed running for further paired tests.

The installed Mint 22.2 `0.6.44+qualification8` CLI and GUI and the installed
Fedora 44 `0.6.44+qualification9` RPM worker passed all six Remote Files
assertions over the same isolated link. The CLI verified separate read and
write grants, rejected a write under the read-only grant, and read back its
generated file. Mint saved the writable file in the GUI and Fedora verified
the exact bytes. After a separate Fedora edit, the stale GUI write failed.
The signed stale CLI request returned the revision conflict and a fresh
read returned the newer bytes. Stopping the worker removed its catalog
entry. After restart, the installed CLI saw the same fingerprint with a
new generation and the GUI reopened the file with the newer contents.
Fedora checked the three owned files before removing them, and both VMs
remained running after the fixture services and media were removed.

The read-only editor in the tested GUI allowed a draft and offered Save.
The denial left the source bytes unchanged. The GUI kept the unsaved draft
after the stale write, while the CLI exposed the refresh-and-retry message.
The original create receipt could no longer be queried after the reconnect
attempt exceeded the operation retention window. In-window receipt replay
was not exercised in this run.

The private Mint desktop DEB upgrade to `0.6.44+qualification8.1` fixed
the read-only editor. In the installed GUI, typing and Ctrl+S left the
read-only file unchanged in both panes. The GUI switched to the writable
grant with both panes open and saved the new text. Fedora confirmed the
exact bytes of both files and removed the fixtures. A focused Qt test
also verified that the GUI dispatches no write under the read-only grant.

A fresh run with the installed Mint `0.6.44+qualification8.2` GUI
and Fedora `0.6.44+qualification10` worker repeated all six Remote Files
assertions and verified in-window replay. The read-only GUI kept the source
text unchanged after typing and Ctrl+S. The writable GUI saved its exact
bytes and retained the draft when a newer Fedora revision rejected its next
save. The CLI independently observed the stale-revision error. After the
original worker left the catalog and the same fingerprint returned in a
new generation, the installed CLI resubmitted the exact create request.
The broker returned the original receipt and result with 89.45 seconds left
before its deadline. Fedora confirmed that its newer file bytes, inode and
timestamps stayed unchanged. Mint opened both newer files in the GUI, Fedora
removed the three fixture files, and both VMs remained running.

The same two guests passed all six Remote Git assertions using the
installed Mint `0.6.44+qualification8.1` desktop and Fedora
`0.6.44+qualification9` worker. The GUI displayed the read-only diff
with Stage and Discard inactive. The CLI read that root and the broker
refused a separately signed Stage request. On the writable Git-only
grant, the GUI staged and committed one selected file. Fedora verified
the exact parent and file bytes, the read-only index and draft, and both
unrelated writable changes.

The first restart was shorter than the broker's ten-second catalog
freshness window. The repeat kept the Fedora worker stopped for
fourteen seconds, and the installed CLI observed an empty catalog before
the same fingerprint returned in a new generation. It retrieved the
original commit receipt, and the GUI reopened that commit and its
one-file diff in History. Fedora verified exactly one commit, stopped
both worker processes and removed both generated repositories. The
fixture media were detached and both VMs stayed running.

The installed Mint `0.6.44+qualification8.1` GUI and Fedora
`0.6.44+qualification9` RPM met four of five Remote Terminal assertions
across the two guests. The GUI opened Bash in the granted root and the same
process accepted another command after switching panels. A worker restart
retired the old shell and refused a signed open pinned to the old generation.
The GUI opened a fresh Bash in the same root and used `/bin/sh` in the
selected Docker container. Fedora independently verified both Bash PIDs
exited and the exact Docker exec stopped. The container stayed live until
fixture cleanup, and SELinux remained enforcing.

The GUI's Verify cleanup action still reported an unknown outcome for the
original terminal after the worker restart. With Fedora upgraded offline to
`0.6.44+qualification10`, the installed Mint CLI reconciled the exact
original receipt against the same worker identity in a fresh generation
without a reboot. The Mint `0.6.44+qualification8.1` GUI continued to
report an unknown outcome.

Mint upgraded offline to `0.6.44+qualification8.2` and passed all five
assertions against the Fedora `0.6.44+qualification10` RPM in a fresh
installed run. The old Bash stayed usable across panel switches. After the
Fedora worker restarted with the same identity and a new generation, the
CLI refused the old-generation request. The GUI verified cleanup for the
original terminal and displayed the confirmation after its catalog refresh.
The broker retained a matching typed receipt. The GUI opened a fresh Bash in
the same root and used `/bin/sh` in the chosen Docker container. Fedora
verified both Bash PIDs and the exact Docker exec stopped with SELinux
enforcing. The Mint authorization window expired during an intermediate
retry and native reauthentication restored access. Both VMs stayed running
after the test workers stopped and the fixture media were detached.

The Fedora Flatpak and RPM also passed Remote Terminal. The GUI opened Bash
in the granted root and returned live input. The same shell stayed active
when the operator switched panels. After the worker restarted, the old
open request was refused and a fresh shell used the same root. The Containers
inspector opened `/bin/sh` in the exact test container. The broker recorded all
three closures, and the guest verified the corresponding processes were gone.
The bundled Docker fixture initially failed under SELinux. Applying Fedora's
existing container-runtime labels to the fixture's private paths made the
fixture run with SELinux enforcing. The test removed the container and stopped
the private daemon. Fedora remained running.

The installed Fedora Flatpak and RPM also passed all six Remote Containers
assertions with SELinux enforcing. The GUI inspected, stopped and started
the exact Docker fixture. A real worker relay interruption closed the
attached shell while the container kept running. After reconnection, the
CLI restarted the same container and the GUI opened and closed a fresh shell.
The guest verified both shell processes had exited and removed the fixture
container. Fedora remained running.

The installed Mint `0.6.44+qualification8.2` GUI and Fedora
`0.6.44+qualification10` RPM also passed all six Remote Containers
assertions across the two guests. The installed Mint CLI verified inventory,
inspect and logs for one exact container. The GUI stopped and started it and
ran `/bin/sh` in it. A four-second relay cut closed the exact first Docker
exec while the container kept the same start time. The same worker
fingerprint returned in a new generation. The CLI restarted the same
container, and the GUI opened and closed a new shell in it. Fedora confirmed
both witnessed execs had stopped and removed the container and its private
daemon with SELinux enforcing. The normal Mint authorization window expired
during an intermediate shell. Native reauthentication restored access before
the GUI close was verified. Both guests stayed running.

On the same Fedora guest, the installed RPM passed all six Remote
Networking assertions in both CLI roles. Two authenticated relays reached
the worker's IPv6 loopback origin concurrently. Closing one preserved
the other, and a mismatched peer identity and credentials from the other
relay were refused. The worker's DNS and trust snapshot matched the
fixture. After disconnect, the route was withdrawn and requests never
fell back to the separate local origin. The test used fixture broker
authority. Its processes exited, the parent image hash matched, and
Fedora remained running.

The installed Fedora Flatpak and RPM also passed Remote Monitoring.
System showed live worker metrics and processes. Containers showed the
exact running fixture and its network in read-only mode. The broker
rejected Stop before queueing the action, and a newer sample showed the
same container still running. Switching to a pairing with sharing off
cleared the inventory. Revoking sharing on the worker cleared both the
System and Containers views. The test used fixture broker authority
with SELinux enforcing. The test container and daemon were removed
from service, the parent image hash matched, and Fedora remained running.

The installed Mint `0.6.44+qualification8` CLI and Fedora
`0.6.44+qualification10` RPM passed all six remote Networking assertions
across the isolated guest link. Two authenticated proxies concurrently
resolved Fedora's loopback alias and reached its IPv6 origin. The CLI rejected
the alternate worker fingerprint and each proxy's credentials on the other
listener. The received public trust anchor matched the exact worker and the
fixture bytes. Closing one proxy preserved the other. Fedora's native GNOME
lock stopped the exact worker and withdrew the second proxy while both
origins still ran. Mint's direct loopback control remained reachable, and its
request count verified that there was no local fallback. The fixture TCP
listeners closed and the media were detached. Both guests stayed running.

Reversing the roles also passed all six installed Networking assertions.
Fedora `0.6.44+qualification10` used native Polkit and two authenticated
proxies to reach the Mint `0.6.44+qualification8` worker's IPv6 origin.
The DNS alias resolved on Mint and Fedora verified the exact public trust
anchor and refused the alternate worker fingerprint. Closing one proxy
preserved the other. Mint used the installed CLI to revoke the fixture's
network grant. The exact worker exited and the last proxy closed while the
origins stayed live. Fedora's local endpoint still answered directly and saw
no fallback traffic. Exact TCP listeners closed, the media were detached,
and both VMs stayed running.

The installed Mint `0.6.44+qualification8.1` GUI and Fedora
`0.6.44+qualification9` RPM also passed all five Remote Monitoring
assertions across two guests. Fedora enabled system-only sharing first,
then added Docker inventory from an isolated daemon with SELinux enforcing.
Mint showed the exact Fedora marker process and running container. The
container's Start, Stop and Exec controls were disabled. The signed CLI
Stop returned an unknown-outcome diagnostic. The operation was absent
from the broker queue, and a newer snapshot and direct Fedora readback
confirmed the same container still running.

The separately paired Fedora worker with sharing off showed no metrics,
processes or inventory. When the active worker revoked sharing, the Mint
GUI cleared both views and the CLI received a newer disabled snapshot with
no payloads. Fedora then removed the exact container and stopped the private
daemon and both fixture workers. Both guests remained running.

The reverse direction passed all five Monitoring assertions. Fedora's
installed `0.6.44+qualification13` RPM and released Flatpak used the isolated
broker and link to the installed Mint `0.6.44+qualification8` worker. The
Fedora CLI verified the exact Mint process and the system-only phase. The
Flatpak rendered the private container with Start, Stop and Exec disabled.
The exact signed CLI Stop failed, its operation was absent from the broker
queue, and a newer sample still showed the same running container. Selecting
the silent Mint worker cleared the GUI inventory. Revoking the active worker's
grant also cleared its GUI inventory and removed both snapshot payloads in a
newer CLI sample. Mint removed the exact container and stopped the private
daemon and workers. Both guests stayed running.

Fedora's installed broker denied job listing at startup, after cancelled
authentication, during GNOME screen lock, after unlocking the desktop,
and after an explicit Dockpipe lock. Both the RPM CLI and installed Flatpak
opened the native polkit prompt. Successful authentication from the GUI
allowed the same job-list request through the broker. A 60-second absolute
policy expired on schedule despite repeated successful requests. The
idle-only run kept the one-hour absolute deadline and used a 60-second
idle limit. One desktop input reset GNOME's idle counter and kept access
open past 60 seconds from authentication. Subsequent job-list requests
did not reset the counter. The broker first denied the request at the
66-second idle sample while the desktop was still unlocked.
In a separate deep-suspend run with screen-lock revocation disabled, the
broker admitted a job-list request before suspend and denied it after wake
on the same kernel boot. QEMU resumed the guest but its display stayed
inactive. Restarting from a preserved overlay restored the GNOME desktop.
The original backing image hash matched and the guest remained running.
Same-session GUI recovery and grant scoping remain open.

Fedora Workstation 44 passed all nine Remote Browser assertions in the
installed Flatpak using Qt XCB through GNOME Xwayland, with the installed
RPM as the worker in the same isolated guest. The local context rejected
the fixture CA. The paired context opened the HTTPS app, followed its link,
used Back and Forward, and submitted one accepted form. Disconnecting the
worker retired the exact Browser host and proxy. The local control received
its first request only after selecting This computer. Reconnecting the
worker restored the original remote tab and site marker with fresh helper
processes. The fixture processes exited, the parent image hash matched, and
Fedora remained running.

In a separate two-guest run, the installed Mint 22.2 X11 viewer and broker
connected to the installed Fedora 44 RPM worker. All nine Remote Browser
assertions passed. Mint's local Browser rejected the test CA. The paired
context rendered Fedora's HTTPS app and followed links and Back and Forward.
Stopping the worker closed the exact Browser host and proxy. Neither app
received traffic while the route was down. The same selected worker restored
the second page and its site marker with new helpers, then recorded exactly
one form submission. Mint's local Browser still rejected the CA after an
explicit switch back. The test closed the helpers and listeners and detached
the read-only media. Both guests remained running.

The released Fedora Flatpak 0.6.38 also loaded a loopback notes application
from the installed GNOME launcher. The server recorded one form submission
and the expected Back and Forward navigation. Reload showed connection
refused while the application was stopped. After restarting the application,
the same Browser tab reloaded the saved note and the server recorded the
request. Two tabs rendered distinct routes and shared temporary site storage.
The download saved through the GNOME picker matched the served bytes. File >
Quit closed both Flatpak processes. The next launch used new process IDs and
the same site showed empty storage. All three registered installed local
Browser happy-path assertions passed on this released baseline.

The public-input Flatpak build verified the SpatialWeb SDK attestations and
the locked KDE SDK, Platform and Qt WebEngine BaseApp commits. Compilation of
the Browser host against those inputs failed because the SDK lacks
`BrowserSessionOptions` and `BrowserWidget::saveSession`. The later private
SDK passed its owner's installed DOM check. The Dockpipe package built with
that SDK passed 112 component tests and its Flatpak Platform smoke test. On
Fedora, the existing released 0.6.38 ref updated offline to the private 0.6.44
commit `228e901cd561e4732f5906142795bbc76a2465dec1482081f3c84e93f4b9435b`.
The installed CLI version and digest matched the candidate, and every saved
project fixture file matched its pre-upgrade hash. The upgraded GUI reopened
the previously selected workspace without selecting it again.

The same installed 0.6.44 Flatpak passed all three local Browser happy-path
assertions and all ten expanded Browser checks. The loopback notes app
recorded one form submission, Back and Forward navigation, and recovery after
the app stopped and returned. The download matched the served bytes. Relaunch
created a fresh temporary profile. The installed GUI preserved an unsent
address draft across a tab switch and opened, inspected and closed DevTools.
The renderer reported `NoNewPrivs=1`, Seccomp filter mode and two filters
under enforcing SELinux. A generated extension loaded, enabled, showed its
popup, disabled and uninstalled. The loopback notification request was denied
and stayed blocked after reload. Fixture servers stopped, the media detached,
the backing hashes matched and Fedora remained running.

The Fedora RPM installed both CLI roles for an isolated Remote Network test
with a fixture broker. All six assertions passed. Concurrent authenticated
proxies reached the worker's IPv6 loopback app through its `localhost6`
alias, refused each other's credentials, and closed independently. The
operator verified the exact peer's trust anchor and rejected an alternate
fingerprint. Native GNOME lock stopped the worker and cut its remaining
relay. The IPv4 control received only direct requests. Both endpoints
exited, the fixture media detached, and Fedora remained running.

The installed Fedora RPM registered its broker and background worker as
systemd user services. The broker required native authentication before
issuing a one-guest workflow grant. The delegated worker retained its
identity across a service restart and appeared with a new generation.
The old generation was rejected, the new one ran the workflow, and the
downloaded artifact matched. Releasing the grant removed the guest and
prevented future delivery. Both test units were removed and Fedora stayed
running. The run seeded prior host trust in the isolated VM. Physical-key
pairing and foreground reconnect remain open for these enrollment lanes.

A fresh installed Fedora RPM broker accepted requests from the installed
Mint DEB over the isolated guest link. The locked broker denied node listing.
After native authentication, the broker opened a pairing window and refused
the legacy file invitation. Mint refused to start pairing without explicit
local delivery permission. The Fedora operator denied one pending request and
closed the window on a second. Mint verified both signed denials and retried
with the same private identity and grant. Public fingerprints matched across
both guests, and no nodes were enrolled. This establishes the broker setup
and health assertion. Physical-key approval, enrollment, reconnect and
revocation remain pending for this pair.

Cancelling Fedora's native Polkit prompt initially kept access locked but
suggested installing the action. The RPM was upgraded to private
0.6.44+qualification11 with corrected handling of Polkit's exit status.
Cancelling the native prompt returned exit 1 and reported cancellation.
The broker stayed locked, and the CLI returned no node list. A fresh
native authentication succeeded, then explicit lock denied another
unattended node query. The test brokers exited, both fixture media were
detached, and the VMs stayed running.

On the same installed Fedora RPM, the 60-second absolute policy first denied
the node query at 61.58 seconds while GNOME reported 22.48 seconds idle. With
the one-hour absolute policy and a 60-second idle limit, desktop input reset
the idle counter and the broker accepted queries past 60 seconds from unlock.
The first denial came at 61.64 seconds of GNOME idle. The broker health check
still passed after both denials. A separate native screen lock denied queries
while the lock screen was present and after the OS session was unlocked.
The fixture restored the original policy.

The same RPM incorrectly retained authorization after a native s2idle
cycle with screen-lock revocation disabled to isolate sleep handling. The
kernel journal recorded the suspend and resume, and the kernel counted the
transition. A private 0.6.44+qualification12 RPM adds that kernel counter to
the Linux admission check. On the installed replacement, the same s2idle
path revoked authorization for sleep/resume. The unattended node query was
denied, and native authentication restored access in the same GNOME
session. The guest's original policy and sleep mode were restored.

The installed qualification12 RPM then passed the remaining broker checks.
Native authentication cancellation returned no node list and kept access
locked. The 60-second absolute policy denied access at 61.32 seconds
despite fresh GNOME input. The idle policy denied access at 60.91 seconds
of GNOME idle, 94.90 seconds after authentication. The broker remained
healthy after both denials. Native screen lock denied access during and
after the OS lock, and explicit Dockpipe lock denied the next query after
successful reauthentication. A workflow-only Mint worker returned the
exact artifact without Files roots, Docker permissions, or a Network target.
Revoking the worker removed its live catalog entry, stopped the worker, and
prevented reconnect with the unchanged private configuration. Together with
the native suspend and wake retest, this covers all seven Fedora broker
security assertions on qualification12. Both VMs remained running.

That Fedora RPM also delivered a real workflow to an isolated, previously
paired Mint DEB worker. The exact artifact came back, and the worker advertised
workflow delivery without Files roots, Docker access, or a Network target.
Revocation removed the worker from the live catalog. Mint's foreground worker
exited, and restarting it with the same unchanged private configuration failed.
Both guests remained running after the fixture media were detached.

Linux Mint 22.2 Cinnamon passed all six Remote Files assertions in a
disposable X11 VM. The normal user ran the same workflow before and after
upgrading the matched CLI and desktop DEBs from 0.6.39 to private
0.6.44+qualification1. The desktop package retains the pinned 0.6.39 GUI
payload and updates its exact CLI dependency. The initial CLI-only upgrade
selected the desktop package for removal and then failed in APT. The
matched pair used a fresh overlay. The installed GUI reopened the newer
file after restart and exact receipt recovery. The viewer and worker
exited, and both fixture roots were removed.

Remote Git on the same Mint release failed the installed GUI check with
the 0.6.44+qualification1 desktop DEB. Stage, Discard, Pull and Push
remained enabled on the read-only grant, and Stage showed an uncertain
outcome. The broker returned HTTP 409 for the read-only mutation, and the
checkout and index matched their original state. On the writable grant,
the GUI committed only the selected file and reopened the commit after
the worker restarted. The exact receipt survived the restart, unrelated
files matched their original hashes, and both fixture roots were removed.
The overlay parent hash matched and the Mint guest stayed running. The
desktop build with the pinned public Browser SDK stopped at the missing
`BrowserSessionOptions` and `BrowserWidget::saveSession` APIs.

The private native desktop DEB built against the verified private Browser
SDK passed all six Remote Git assertions on the same guest. Both packages
upgraded in place to 0.6.44+qualification2 with `dpkg`. APT had rejected a
local package pathname before making the upgrade. The normal user ran the
same installed CLI workflow before and after. The running launcher hash
matched the upgraded package after the previous tray process exited.
The GUI blocked read-only actions, staged and committed only the selected
file on the writable grant, and recovered the same commit after worker
restart. The fixture roots were removed, the viewer exited, the media was
ejected, and the guest stayed up. This pass applies to the private DEB and
the public desktop build remains blocked on the SDK pin.

The upgraded `0.6.44+qualification7` Mint installation repeated all six Git
assertions. The installed GUI displayed the read-only diff with Stage and
Discard unavailable. A direct request also failed before admission and left
its index and working state unchanged. In the writable root, the GUI staged
only the selected file and committed it. The tracked and untracked drafts
were left intact. The old worker exited, its catalog entry expired, and its
replacement reconnected under the same identity with a new generation. After
reselecting the writable root, the GUI reopened the exact commit and its patch.
The original commit receipt matched, both generated repositories were removed,
and the Mint VM stayed running.

The same private Mint DEB passed all six Remote Workflows assertions. The
installed GUI previewed and delivered a workflow to a delivery-only worker.
The result, log and artifact downloaded through the native file picker
matched the broker receipt. A second job resumed live logs on the same
assignment after a four-second relay outage. The GUI cancelled it and
showed the terminal state on refresh. The guest verified that the original
process had exited. Both roles ran in the offline Mint VM with fixture
operator authority. The worker, relay, broker and viewer exited, the media
was ejected, and Mint stayed running.

The installed `0.6.44+qualification7` Mint GUI delivered a completed workflow
to a separate delivery-only worker, showed live output, and downloaded the
result, full log and exact requested artifact. Its separate reconnect attempt
exposed a transport failure: closing the fixture relay returned EOF before
the worker's machine challenge received an HTTP response. The worker stopped.
The updated core treats that transport EOF as retryable under the existing
heartbeat deadline and still refuses an empty successful response.
Normal DEB upgrade to private `0.6.44+qualification8` then passed the same
four-second relay cut. The installed GUI showed new output on the original
assignment, and cancellation closed the exact child with no replay. The full
result download also passed on `qualification8` in a separate successful job.
The installed GUI wrote the result, full log and requested artifact to the
selected folder, and all three matched the exact broker result byte for byte.
This completes all six Mint Remote Workflow assertions on the same installed
CLI and GUI package pair.

The installed Mint DEB also passed all five Remote Terminal assertions.
The first Bash session kept its process and granted root across a panel
switch. After worker restart, the old open request could not create a new
session and the GUI opened a fresh Bash in the granted root. The Containers
inspector opened `/bin/sh` in the exact test container. Broker receipts and
guest process checks confirmed closure of both Bash sessions and the
container shell. The initial fixture timed out before the GUI closed the
first session. The retry increased the observation window. Both runs used
an isolated daemon. The retry removed its container and confirmed the
daemon stopped while Mint remained running.

The same private package exposed a separate local Terminal failure. The
DEB contained the terminal provider but lacked the link beside the CLI.
The 0.6.44+qualification3 upgrade restored the link supplied by the normal
DEB builder. The installed GUI then opened Bash in the selected workspace,
returned live input, and resized the PTY from 48 by 118 to 63 by 171.
After the tab closed and the viewer exited, the guest confirmed that the
shell and its test child were gone.

Linux Mint 22.2 Cinnamon passed all six Remote Containers assertions using
the installed 0.6.44+qualification3 desktop and CLI. The GUI inspected,
stopped, and started the test container. Interrupting the relay closed the
attached shell while the same container kept running. After reconnection,
the CLI restarted that container and the GUI opened and closed a fresh
shell. Both roles ran in one offline VM with fixture operator authority.
Guest checks confirmed both shell processes were gone. The test removed
the container, stopped its private daemon, and ejected the test ISO.

The installed `0.6.44+qualification8` Mint CLI and GUI repeated all six
Containers assertions with a private Docker daemon. The worker relay cut
closed the first Docker exec ID and left the original container running with
the same start time. After the same worker returned in a new generation, the
CLI restarted the exact container and the GUI opened a different exec ID.
Both shells returned distinct input markers. Direct Docker readback confirmed
closure of the second exec ID after the GUI closed it. The test removed the
container and stopped its private daemon. A separate abrupt worker-stop probe
left a nested foreground command alive until fixture cleanup. That probe
records the worker-crash boundary separately from live transport recovery.

After the host reboot, a newly installed Mint 22.2 Cinnamon KVM guest
booted from a writable overlay over a verified backing disk. The installed
private 0.6.44+qualification4 CLI ran an offline host workflow with matching
binary and output digests. The public Browser SDK still failed compilation.
The matching private desktop DEB used the recovered Browser SDK whose 23
files matched the previously verified SDK receipt.

The installed Mint CLI and desktop passed all five Remote Monitoring
assertions as the worker and operator in that offline VM. The GUI showed
system and process data and the exact Docker container. Switching to a
silent pairing cleared the process list and metrics. The broker rejected an
ungranted stop, and a newer sample showed the fixture still running. After
the worker revoked sharing, the installed GUI cleared both views. The
running viewer and installed worker hashes matched their packages, the test
removed its container and stopped its daemon, and the backing-image hash
matched while the Mint VM stayed running.

Mint Network initially refused to start because the native lock signal was
unavailable. The installed 0.6.44+qualification5 CLI recognizes the Cinnamon
screen-saver service when it is running. With Cinnamon's native locker in
`--hold` mode in the offline VM, the installed CLI passed all six Networking
assertions as both operator and worker. Two independent proxies reached the
worker's IPv6 loopback app and rejected each other's credentials. Trust matched
the exact worker, an alternate fingerprint failed, and closing one proxy left
the other usable. The IPv4 control saw only direct requests. Locking the
Cinnamon session closed the worker and prevented traffic through the remaining
proxy. The default idle-stopped locker case remains blocked on native lock
evidence.

The installed Mint 22.2 desktop and CLI from the private
`0.6.44+qualification5` package pair passed all nine Remote Browser
assertions in the same offline X11 VM. The local Browser context
rejected the fixture CA. The paired context loaded the HTTPS app, followed
its link, and used Back and Forward. The viewer closed its exact Browser
helper on disconnect and denied reload. After the same worker returned, a
new helper restored the original tab and the exact site-storage value set
before disconnect. The app recorded one form POST after reconnect. Both
fixture roles exited and the Mint VM stayed running.

openSUSE Leap 16.0 passed all six Remote Files assertions with the pinned
Flatpak viewer and private 0.6.44 RPM worker on Plasma X11. Zypper upgraded
the CLI from 0.6.39 in the offline guest. The normal-user workflow ran on
both versions and project hashes matched. GUI reattachment reopened the
newer worker bytes after exact receipt recovery. The viewer and worker
exited, both fixture roots were removed, and the backing disks, media,
and original firmware hashes matched after shutdown.

The pinned Rocky Linux 9.6 and Amazon Linux 2023.11.20260526 Docker workers
each passed all six Remote Files assertions with the installed Pop!_OS viewer
after a DNF upgrade from 0.6.39 to the private 0.6.44 RPM. A normal user ran
the same host workflow before and after upgrade, and the project hashes
matched. The GUI verified the separate grants and reopened the newer file
after worker restart and exact receipt recovery. Both fixture containers and
bridges were removed, and the pre-existing container IDs matched.

The pinned Red Hat UBI 9.6 Docker worker passed the same six Remote Files
assertions with the installed Pop!_OS viewer. DNF installed 0.6.39 and
upgraded to the private 0.6.44 RPM with dependencies from the UBI 9
repositories. The normal-user workflow ran on each version with matching
project hashes. The fixture container and bridge were removed, and the
pre-existing container inventory matched. This covers the UBI worker lane.

The combined UBI 9.6 suite ran the installed Pop!_OS viewer and 0.6.44 RPM
worker against four separate Files and Git grants. Remote Git staged and
committed only the selected fixture file. Unrelated staged, unstaged and
untracked content remained in place. One worker restart recovered both
exact write receipts within their original deadlines. The GUI reopened
the newer file and original commit after reselecting their grants.
Both repository roots and both file roots were removed, along with the
fixture container and bridge.

The first Git stage was denied because the worker had no recovery mode.
Independent readback matched the original index, HEAD and file hashes.
The fixture retry explicitly selected the existing host-reboot recovery
mode. The successful run recovered completed receipts within their
deadlines, then performed the GUI reattachment checks.

The same installed Files and Git sequence passed on Rocky Linux 9.6,
Amazon Linux 2023.11.20260526, and Alpine 3.24.4 Docker workers. Each ran the
same CLI workflow before and after upgrading to 0.6.44. Amazon Linux and
Alpine ran concurrently using separate brokers, paired identities, private
X11 viewers, and bridges. Amazon's installer needed `SETFCAP` to unpack its
RPM dependency, while the worker ran unprivileged. Rocky's Git stage passed
before the GUI accepted commit input. The suite continued from the verified
staged state in the same window. Both write receipts were recovered before
their deadlines, and each viewer reopened the correct file and Git history
after restart. All fixture containers and bridges were removed, and the
pre-existing container inventories matched.

The installed Pop!_OS X11 GUI and Alpine 3.24.4 Docker worker passed all six
Remote Workflow assertions on the fixed APK. The first installed candidate
delivered the job but tried to run musl as the workflow executable. Under
Alpine's glibc compatibility loader, the worker's kernel executable link
identified musl. For that case, the worker and service definition now resolve
the mapped program file and verify its device and inode.

After APK upgraded the worker to `0.6.44-r2`, the same enrolled fingerprint
published a new generation. The GUI streamed a real workflow and downloaded
the result, log, and artifact byte for byte. A separate job resumed streaming
on its original assignment and process after a four-second relay outage. GUI
cancellation cleared the live log and stopped its shell. The fixture's PID 1
left a zombie entry until container removal. The container and private network
were removed, and the original Docker inventories matched.

The installed Pop!_OS X11 GUI and UBI 9.6 worker passed the same six Remote
Workflow assertions on private CLI `0.6.44+qualification9`. The pinned UBI
container passed both a clean RPM install and an in-place upgrade. Its
unprivileged project workflow preserved the original configuration and wrote
the exact version history. The GUI then delivered a real workflow and
downloaded the result, log, and artifact byte for byte. A second job recovered
from a four-second relay cut on its original signed assignment and process.
The GUI cancelled it, the exact process was reaped by the container init, and
the live log cleared. The private container and bridge were removed and the
original Docker inventories matched. An initial run recorded a fixture-only
failure from the wrong OS release path, which was corrected before retesting.

Rocky Linux 9.6 passed the same six Remote Workflow assertions with the
installed Pop!_OS GUI and the same private RPM. The pinned image passed both
a clean DNF install and an in-place upgrade from 0.6.38, with the generated
project hashes retained. The GUI download matched the broker result, full log,
and requested artifact. The separate interrupted job resumed on the original
assignment and process, and GUI cancellation stopped and reaped it. The
private container and bridge were removed. The first fixture was replaced
after correcting the test script's read permissions for the unprivileged user.

Amazon Linux 2023.11.20260526 passed the six Remote Workflow assertions with
the installed Pop!_OS GUI in a separate Docker fixture. The pinned container
release passed clean RPM installation and an upgrade from 0.6.38.
The GUI downloaded the exact result, full log, and requested artifact. A
separate four-second relay cut preserved the original running assignment.
GUI cancellation closed the exact child without replay. Both fixtures ran
with independent brokers and bridges. The Amazon container, bridge, and
private display were removed, and the unrelated Docker inventory matched.

Kali Linux 2026.3 amd64 passed the combined Remote Files and Remote Git
suite with the installed Flatpak viewer on Xfce/X11 and the native DEB
worker in the same isolated VM. The normal-user workflow completed before
and after the upgrade from 0.6.39 to 0.6.44. The GUI exercised separate
grants, rejected a stale file save, and committed only the selected change.
After worker restart, the original receipts were recovered and the GUI
reopened the newer file and the original commit. The worker, viewer, and
four fixture roots were cleaned up between suites.

In the same running Kali VM, the installed Browser relayed a private HTTPS
test app, followed links, used Back and Forward, and submitted a form. The
local Browser rejected the private CA. After a worker restart, the remote
tab and site marker returned, and the app journal contained one POST.
A second run tracked the Browser host and proxy by PID and kernel start
time. Both exited when the worker disconnected, and both had fresh
identities after reconnect. The app recorded no requests to the local
control URL until the viewer explicitly selected This computer.
All nine installed Remote Browser assertions passed.

On Kali, the installed Flatpak also showed the paired worker's system and
process data. Switching to a pairing with no sharing cleared the view, and
revoking worker sharing cleared it again. The native worker published the
exact running container and its network inventory. The broker denied Stop,
and the next sample showed the same container still running. The installed
Flatpak lacked a usable remote monitoring selector in Containers, so the
lane failed for this launcher digest. The test container was removed and
the guest Docker daemon stopped.

The rebuilt Flatpak passed after upgrading the installation from a private
offline Flatpak repository in the same Kali VM. With direct Docker socket
access hidden from the viewer, the restored selector showed the paired
worker's exact container and two networks. The inspector disabled Start,
Stop, and Exec. Switching to the silent pairing cleared the inventory.
Worker-side revocation cleared Containers and System. The fixture container
was removed and the daemon stopped. The baseline failure remains recorded
for its original launcher digest.

The private Windows 0.6.41 MSI worker failed before it published its Remote
Files grants. The feature journal inherited the Unix-only lock even though
the Windows workflow worker already had a native private-state lock. A
focused fix gives the feature journal its own Windows lock and reads Terminal
recovery stores only where a Terminal backend is available. Native Windows
tests verified feature catalog publication, contention and reacquisition while
the workflow lock was held.
The private 0.6.43 MSI passed all six Remote Files assertions in the
disposable Windows 11 x64 VM with the installed Pop!_OS viewer. The GUI saved
the writable file at its reviewed revision and rejected a stale draft.
After restart the exact fixture write returned its original receipt, and the
GUI reopened the newer worker bytes. Cleanup verified both fixture roots and
worker processes were gone and the backing image hash matched.

The physical Win11 x64 worker ran the installed 0.6.41 CLI. The paired Linux
operator listed and read a generated Files root, saved a new revision,
rejected a stale save, and read back the exact new bytes. System monitoring
returned two fresh samples. The temporary Files grant rotated the worker's
Terminal session. A new session confirmed the original Files grants were
restored, monitoring was disabled by removing the temporary opt-in, and the
fixture was gone. These are partial CLI observations. The installed GUI
checks remain unrun on the physical pairing. At that point Git and Docker
were absent from the checked executable locations.

The physical Win11 x64 worker later installed Git for Windows 2.56.1 through
Dockpipe's host dependency preflight. The installer and executable were
verified, and a second run used the existing Git without running an installer.
Docker Desktop's first per-user installation attempt identified the disabled
Virtual Machine Platform feature. A private Windows CLI candidate verified
that declining approval left the feature disabled and did not run the Docker
installer. The approved dependency enabled the feature and reported that a
restart was needed. Windows restarted and the desktop sender returned. The
ordinary worker refused to resume because the broker marked that exact reboot
workflow as interrupted. The operator reran the same Docker dependency in a
native Win11 owner session. It observed the Windows feature already enabled,
installed Docker Desktop per-user, and returned the Docker client version.
Docker accepted first-run setup and then reported a WSL startup timeout.
The host inventory showed an active hypervisor and no registered WSL service.
A second declared dependency installed the signed Microsoft WSL 3.0.5 MSI
after detecting the missing service. The registered service and WSL version
responded after repair. Docker Desktop started in the interactive session, but
the Linux engine named pipe did not appear. Neither the retry from Dockpipe's
host workflow nor the direct application start produced a server version.
A later launch in the signed-in owner's desktop session kept Docker Desktop
running after Terminal disconnected. The Linux engine pipe reported Docker
Engine 29.9.3. The installed Win11 0.6.41 CLI listed and inspected a disposable
container, started it, returned its exact log marker, stopped it, and inspected
the exited state. Native Docker verified the running state and removed the
fixture. After the service migration and reboot, Docker Desktop needed a
launch in the signed-in owner's session. Docker Engine 29.9.3 then responded.
The installed 0.6.45 CLI passed listing, inspection, start, exact log
readback, stop, and exited-state readback in a workflow delivered to the
service-owned Win11 worker. Cleanup confirmed the fixture was absent. The
paired Linux desktop app then selected the Win11 workspace, listed a new
disposable container as running, and displayed the exact generated log
marker in the inspector. The temporary read-only grant disabled Start, Stop
and Exec. Native Win11 readback confirmed the grant was restored, the
container removed and the temporary fixture task absent.
With persistent owner-approved Containers read and control and Windows
host-reboot recovery enabled, the Linux desktop app started a freshly
created disposable Win11 container and then stopped it. The app showed the
state transitions and separate native inspection confirmed running and exited
for the same full container ID. Native cleanup confirmed the fixture was gone,
and the paired catalog still advertised Containers control.

The broker's new exact-job acknowledgement preserved the interrupted
restart's unknown result and let the original Windows worker resume after
its existing background task was started. Startup migrated the acknowledgement
to private sidecar state to restore compatibility with older operator CLIs.
The operator confirmed the retained status and acknowledgement after restart.
With the worker recovered, the installed Linux desktop opened and saved the
generated Win11 file, rendered live Win11 CPU, memory, disk and network
telemetry, and displayed and staged the generated Git change. The first GUI
commit returned an uncertain outcome. Native inspection found only the initial
commit. A fresh disposable repository then passed in the older Linux desktop
app against the updated broker. The native receipt verified the expected new
commit and the unrelated file still untracked. The temporary grants and
monitoring configuration were restored and the repeat fixture was removed.
After starting the existing Win11 preview sender registration, the older
desktop app also rendered the live Win11 stream. The viewer reported verified
remote cleanup on disconnect. This check sent no Windows mouse or keyboard
events.

The rebuilt Pop!_OS Git view and installed Ubuntu 24.04 worker passed the
selected-file commit journey. The read-only root showed status and diffs with
write controls disabled. The writable root accepted staging and committing one
file while its unrelated changes and the separate root's staged file stayed
intact. Refreshing after worker restart reopened the same commit and remaining
changes. Retrying the completed request returned its original receipt without
adding a commit. Interrupted mutation recovery requires separate evidence.

The private Windows 0.6.43 MSI worker displayed Git status and diffs but
rejected staging before any native mutation because process recovery was
unavailable. The private 0.6.44 MSI passed all six Remote Git assertions
with the rebuilt Pop!_OS Git viewer and owner-selected host-reboot recovery.
The GUI committed one selected file, retained unrelated edits and the
separate read-only index, and reopened the original commit after worker
restart. Retrying the exact commit returned the original receipt. The run
used pinned MinGit 2.56.1 and verified fixture and VM cleanup.
An actual Windows reboot exposed that the boot environment GUID could stay
unchanged. The corrected native code requires the kernel boot sequence to
advance. After reboot, a native broker and worker test reconciled a seeded
uncertain Git receipt using the real prior boot sequence. The outcome stayed
unknown and the repository lock was preserved. Live interrupted Git recovery
through the installed GUI remains unrun.

The installed Pop!_OS GUI passed Remote Terminal against an Ubuntu 24.04 worker
with a private Docker daemon. The host shell started in the owner-selected
root, accepted input, and kept its process when the viewer switched panels.
After the worker restarted, a fresh shell used the new generation and an old
open request was rejected. The container shell used the selected ID and shell.
Guest read-back verified that both host shells and the container shell exited.
The isolated GUI needs its own writable temporary directory for terminal request
files. The separate local-terminal check verified input, resize, and shell-tree
cleanup after restoring native provider discovery in the Linux DEB layout.

The rebuilt Kali Flatpak and installed native worker passed Remote
Terminal. The first Bash kept its PID and selected root when the viewer
switched panels. After the worker restarted, the old open request was
rejected and the new Bash used the same granted root. The GUI then opened
`/bin/sh` in the exact test container and accepted live input. The broker
acknowledged all three closures, and the guest verified that the processes
were gone. The fixture container was removed and its daemon stopped.

Pop!_OS and Ubuntu 24.04 also passed Remote Containers with a private Docker
daemon. The installed GUI stopped and started one selected container, and the
installed CLI restarted it. A real relay interruption ended the attached shell
while the container kept running. After reconnecting the same worker and broker,
the GUI opened and closed a fresh shell. The run repaired the worker's cleanup
wait and its ability to discard a verified closure after the lease expires.
Uncertain start/stop mutation reconciliation remains a separate case.

The installed Kali Flatpak and native CLI passed Remote Containers in the
same Kali VM. The GUI stopped and started the exact fixture container.
A worker-only relay interruption terminated the attached shell without
changing the container's init process, and the same worker generation
returned with a verified close receipt. The native CLI then restarted the
container, preserving its ID and changing its init PID and start time.
The GUI opened a fresh shell, accepted input, and closed it. The guest
verified both shell exits, removed the container, and stopped Docker.

The installed Kali CLI also passed the Remote Network lane in both roles
inside the isolated guest. Two authenticated proxies reached an IPv6
loopback test app concurrently through the worker's localhost resolution.
Each refused the other's credentials, and one stayed live after the other
closed. Trust returned the exact configured CA for the paired fingerprint
and refused an alternate fingerprint. The alternate and stale targets
could not open a relay. The IPv4 local control saw only the two direct
requests. Both proxies, the worker, the broker, and the listeners exited.

On Kali, the first rebuilt Flatpak omitted the delivery-only worker from
the workflow picker. Its bundled CLI returned an empty catalog while the
native CLI listed the live worker. A private Flatpak upgrade paired the
same launcher with the verified 0.6.44 CLI. The picker then delivered a
workflow and downloaded its exact artifact. A second job resumed live
logs on the same assignment after a relay interruption. GUI cancellation
produced a cancelled result, and the guest verified the original process
had exited. The fixture broker stopped before the GUI refreshed the final
row on the first run. A repeat kept the broker online until an explicit GUI
refresh showed the same job as cancelled and up to date with the broker.
The broker receipt and worker process check matched that row. The repeat also
verified the downloaded artifact and resumed logs on the same assignment.
Both Flatpak variants are recorded against their respective commits.

The installed Kali DEB broker originally accepted a read-only job-list request
while Xfce showed its lock screen. Xfce reported an active locker while logind
reported an unlocked session. The private replacement DEB honors the Xfce signal.
It refused requests during screen lock and after the desktop was unlocked until
Dockpipe was authenticated again. The installed native CLI and private Flatpak
both opened the host polkit prompt. The broker also rejected a cancelled
authentication and requests after explicit lock. A separate 60-second run
expired on schedule despite successful job-list polling.

The broker role has partial evidence for the replacement DEB. A separate
ACPI deep suspend run disabled screen-lock revocation to isolate sleep
revocation. The guest accepted a job-list request before sleep, then rejected
one after QEMU woke it. The guest exported the result over serial, restored
the original policy and stopped the broker. The virtual display stayed blank
after wake, and a restart from the same checked overlay restored the desktop.
The base image digests matched after the run. Xfce reported no trusted
idle-input signal. GUI display recovery without restart, operation scoping
and per-capability grants still require evidence.

## Remote Desktop settings parity

Use the installed role and OS/architecture combinations in the product registry
for release acceptance. Measure a native platform capability through the
shipped provider. Component tests, cross-compilation, and manually configured
preview senders are separate evidence.

| Capability | Current evidence | Release qualification remaining |
| --- | --- | --- |
| Windows pointer | Physical Win11 amd64 passed native cursor drawing and primary capture. The signed preview sender started without a console. In the Linux X11 viewer, two pointer positions were visible in the stream with the local pointer hidden. The local pointer appeared and disappeared when the setting was toggled. The sender and tests cross-compiled for ARM64. | Repeat with release-installed artifacts, validate hotspots, display switching and mixed DPI, and execute native and installed tests on ARM64. |
| Text clipboard | A Linux X11 preview viewer and physical Win11 amd64 exchanged synthetic text in both directions in Control. A later installed View session denied Unicode clipboard transfer in both directions. The native fixture restored the prior Windows clipboard and removed its task. | Complete the fresh Control watch and focus-loss assertions after resolving the retained Win11 reservation. Repeat with release-installed artifacts. |
| Files and drops | The signed Win11 amd64 preview and Linux X11 viewer copied a generated file in both directions. On Win11, an independent read verified bytes after both keyboard paste and a viewer drag into Explorer. The viewer fixture also verified Ctrl+V after transfer commit. | Repeat on the release packages and Windows ARM64. Qualify cancellation, long-running cache ownership, and reparse-point rejection in the installed Windows lane. The native reparse fixture skipped because this test account could not create its link. |
| Images | Physical Win11 x64 passed the image asset protocol and file-transfer regression tests. A private test-signed sender on the installed Windows service passed both directions after a clean Control disconnect and reconnect from the installed Linux X11 viewer. A separate View session denied both directions. A later Control session preserved exact RGBA values in both directions and denied clipboard transfer while the viewer was unfocused. The sender and component tests built for Windows ARM64. | Repeat the installed checks with release artifacts and on Windows ARM64. |
| Display source and viewer roles | The private test-signed Win11 x64 sender, helper and CLI advertised both attached displays to the Linux X11 preview. The viewer switched from the 125% external monitor to the 100% laptop panel on reconnect. Both streams entered 1440×950 fullscreen under Openbox, accepted a click and exact keyboard input in separate Windows fixtures, and returned to the tab. | Repeat with release artifacts. Verify drag alignment and approved administrator input on both displays, other sender/viewer roles and architectures, and virtual displays where supported. |

An intermediate Windows image preview returned the Linux image to the viewer
after the WinForms fixture read it. The test-signed fix tags images published by
the current Windows connection. Five clipboard reads after a verified paste
retained the original X11 selection owner and exact PNG bytes. A distinct
WinForms copy then arrived with the expected pixels.

A separate installed Win11 x64 View session streamed video while the Windows
fixture published a generated image. The Linux viewer's synthetic clipboard
owner and exact PNG bytes stayed unchanged after three paste gestures.
Win11 never received the Linux image, and its fixture restored the prior
clipboard and removed the temporary task. The session had image sharing
enabled in the viewer's saved preferences.

In Control, a separate X11 window held keyboard focus while a translucent
Linux image was selected. The Windows fixture stayed at its waiting state
until the viewer regained focus and pasted. The fixture read the exact
color and alpha bytes in the native DIBV5. For the reverse direction, the
first scheduled Windows PNG copy ran while the viewer was unfocused and
left the Linux clipboard unchanged. A fresh copy with the viewer focused
arrived with both expected RGBA corners.

The physical Win11 x64 fixture now has a 1920×1200 laptop panel at 100%
and a 3840×2160 external monitor at 125%. A probe using Dockpipe's native
capture implementation obtained a full-size frame from each display and
rejected a coordinate outside both displays. The candidate sender publishes
an inventory through the desktop catalog, and the viewer applies
the selected source on the next connection. Both input paths use the same
opaque source identifier, which changes when its physical bounds change.
On x64, the native probe hit physical centers (960, 600) and (3840, 1080)
and restored the prior cursor position. Candidate Windows components
cross-compiled for ARM64. The Linux launcher built with Qt 6.10.3 and its
Remote Desktop widget suite passed. The package's X11 viewer and IPC suites
passed 113 and 24 checks respectively.

The first mixed-display preview connection accepted input, then failed to
release the Control reservation because its cleanup exchange withdrew desktop
readiness while still advertising displays. The fix removes the inventory
from the cleanup exchange and the saved close report. The signed Win11
update passed the external-to-laptop reconnect, fullscreen on both screens,
and independently recorded the exact click and keyboard input on each
display. The final test disconnected through the viewer and closed its
Windows fixtures. This was a private preview with standard Control.

A quick Control reconnect in the text clipboard follow-up left a retained
Win11 reference with zero lease and an `authority-lost` reason. Its original
sender generation differed from the live catalog; the pairing fingerprint
and broker epoch still matched. New opens returned HTTP 409 with text sharing
enabled and disabled. An exact close request did not obtain native cleanup
proof. The broker change only releases requests still queued at authority
loss. Once dispatch may have occurred, it retains the reservation until
verified native cleanup. The fix is source-level and has not been deployed
to the live broker.

## Roles and exercise

| Area | Roles | Installed observations |
| --- | --- | --- |
| Enrollment and access controls | Operator, worker, broker, background worker where shipped | Enroll using the real hardware key, activate only the chosen grants, verify identity, reconnect, revoke, and check denial across lock, idle expiry, sleep and wake. Exercise the service lifecycle, including the per-user Windows task on candidates that ship it. |
| Workflow, Files, Git and Terminal | Operator and worker | Use a generated project. Run and cancel jobs, observe logs and reconnect. Read and mutate generated files and Git state through independent grants. Verify revision checks, terminal input and output, and exact cleanup. |
| Containers, monitoring and networking | Operator and worker | Use a selectable daemon and owned workloads where the feature requires them. Exercise lifecycle, exec and terminal, read-only host and container metrics, independent worker relay contexts and worker-side DNS. Prove rejected grants and scoped cleanup. |
| Browser | Local installed Browser or X11 Browser viewer and paired worker | Open a generated test app inside Dockpipe. Navigate between pages, operate real controls and verify server-side actions. Reopen the local Browser or reconnect the paired context and verify the expected session behavior. For the remote route, verify worker-only origin, profile isolation, and no local fallback. |
| Remote Desktop | X11 viewer and Darwin sender with the installed native provider | Enumerate every attached display and verify a unique frame on each. Switch displays, enter and leave fullscreen, and validate coordinates in a mixed-DPI topology. Deliver keyboard and mouse input and verify text, image and file transfer in both directions. Check independent view and control grants, virtual display presets where provided, shutdown and lease expiry. |

Use the actual installed Qt backend. The remote Browser viewer and current
remote-desktop viewer require X11 (`xcb`). Their Wayland, macOS and Windows
viewer paths are unsupported. A pinned Linux desktop profile can declare
`display_backend` as `wayland` or `x11`. The generator excludes the X11-only
viewer roles on a pinned Wayland profile and retains its local Browser role.
For a profile without a declared backend, viewer runs must establish X11.
The native desktop sender is Darwin. Windows
native broker authority fails closed, and Windows worker coverage remains a
separate role. Test systems need the relevant desktop, display topology, Docker
daemon, and trusted OS authentication as prerequisites. [Remote nodes](../../../docs/runtime/remote-nodes.md)
· [Remote Browser](../../../docs/agents/tasks/remote-browser-contexts/overview.md)
· [Native Remote Desktop](../../../docs/architecture/native-remote-desktop.md)

## Host-authorized guest enrollment

For a disposable VM owned by the harness, an authenticated operator can grant
its paired host permission to enroll guests for one run. The guest generates
its own identity on the writable overlay. The host enrolls that identity using
the guest's signed public claim. The current delegation grants workflow delivery.

The isolated Ubuntu 24.04 pilot passed candidate CLI installation, delegated
enrollment, workflow-only target discovery, a real workflow bound to that target,
grant release, target disappearance, worker exit and verification that the cached
base stayed unchanged. Track this as delegated enrollment evidence.

Kali 2026.3 passed the delegated workflow lifecycle using the installed CLI,
an isolated native broker and two fixture host identities in the same VM.
The broker refused to issue the grant while locked and accepted it after
native polkit authentication. The authorized host enrolled one signed guest.
A different host and a second guest were refused. Exact retries preserved
the grant and guest identity. The guest advertised workflow delivery without
file roots, Docker or network grants. The CLI delivered a workflow to the exact
worker fingerprint and generation and verified its downloaded artifact.
Releasing the grant removed the guest, stopped its worker, and denied a new
delivery. The test stopped its broker and ejected the read-only fixture media.

The hardware-key assertion in `remote.enrollment` requires its own physical
machine run. The guided Ubuntu driver accepts an optional run grant for this
flow. A combined Ubuntu 24.04.4 amd64 run installed the guest through the normal
host GUI, installed published 0.6.38 DEBs, and upgraded to a matched private
0.6.39 CLI and desktop pair. The disposable broker delivered a real workflow to
the enrolled guest. The installed guest GUI opened in the embedded display and
restored the selected workspace after a disk-only restart. A workflow started
from that GUI exited successfully after the restart. The grant release removed
the guest worker, and both VM processes stopped normally.

The disposable broker used test operator authority. The installed desktop
observations were collected alongside the automated guided receipt, which left
the desktop case pending. The physical hardware-key enrollment and installed
operator GUI delivery assertions remain open for this combination.
[Guided driver configuration](platforms/linux/README.md#optional-workflow-worker-enrollment)

## Windows workflow candidate retest

The physical Win11 worker later claimed and completed two signed diagnostics
after the paired user restarted the installed 0.6.39 CLI in PowerShell. The
startup receipt verified the new process, its interactive parent, and the
existing enrollment. The scoped Windows error-log check did not establish why
the previous process exited. The Container detach check remains blocked on a
compatible Docker provider. The installed CLI exposed `--detach`, but its
Docker check found neither an eligible executable nor a reachable engine pipe.

The installed physical Win11 worker also completed a signed failure-case
run. PowerShell exited with code 37 after emitting the fixture's corrective
diagnostic once. The broker returned failure, the downstream step was
skipped, and the result contained no artifacts. This covers the native
worker's failure path.

The private Pop!_OS candidate selected a native Windows 11 x64 VM worker in the
workflow picker and displayed PowerShell output while the job was running.
Interrupting and restoring the owned relay resumed output on the same assignment.
The GUI cancelled that job and a guest read-back confirmed the PowerShell
process had exited. A follow-up run installed 0.6.38, upgraded to a native
0.6.39 qualification MSI, and ran a local workflow and installed GUI checks on
both versions. The MSI combined the current CLI with the pinned 0.6.39 Windows
desktop payload. The private Pop!_OS GUI also ran and downloaded a real
workflow result from the MSI-installed worker. A separate job with the same
installed worker passed live logs, relay reconnect without replay, and GUI
cancellation. Both brokers used test operator authority. The physical Windows
MSI lane still requires a retest with the updated broker, GUI, and worker.

In parallel, the physical m6 worker acknowledged Ctrl+C through a disposable
Terminal session. The native shell returned SIGINT exit status 130 within one
second, accepted a fresh marker in the same session, and closed after the
third acknowledged input. This is partial Terminal coverage.

A separate physical m6 check held two concurrent sessions in the same
review grant. Each kept its own reference, shell PID, terminal dimensions,
input acknowledgements and output. Closing the first retired its shell
while the second retained its PID and dimensions and accepted fresh input.
Both returned closed receipts. The worker PID, generation and terminal epoch
stayed the same.

A later physical m6 check acknowledged the first input and then rejected a
request that skipped the next sequence. The rejected marker never ran. Exact
closure retired the first shell, and a fresh control session accepted input
and closed. The existing worker PID, generation and terminal epoch matched
their preflight values. A separate Files read under the existing review grant
returned `unknown` at submission and exact status lookup. That result leaves
the Files route for private candidate staging blocked.

## Background worker startup and recovery

The `remote.enrollment` background-worker role applies to every supported
Windows and Linux CLI profile. Its registry checks require native activation,
continued polling after the launching terminal closes, one worker after
repeated activation, crash recovery, startup at the next native user session,
and reconnect with the same pairing. Record each exact platform and installer
independently, including Windows x64, Windows ARM64 and WSL 2. The container
profiles are blocked for this role until a native service session is admitted.

The Fedora 44 VM upgraded its native RPM to `0.6.44+qualification13` and
registered a disposable worker with its normal systemd user manager. The
worker survived the service command exiting, and repeated startup preserved
its PID. Its PID and session also survived 24.01 seconds of HTTP 503 responses
from a loopback fixture, covering five transient retries. A deliberate kill of
the exact unit started a new PID and polling session with byte-identical worker
configuration. The test unit and worker configuration were removed. A separate
normal desktop sign-out and sign-in on the same boot started a new invocation
automatically, with the same unit and worker configuration. That test removed
its unit as well. Fresh enrollment and installed GUI activation need their own
witnesses.

The Mint 22.2 VM upgraded its native CLI DEB in place from
`0.6.44+qualification8` to `0.6.44+qualification14`. The systemd user worker
remained active after its launching terminal tab closed, with no controlling
terminal. Repeat activation kept one PID. A 24.01-second loopback outage
returned five HTTP 503 responses and preserved the polling session. Killing
the worker caused systemd to start a new PID and polling session with the same
saved state. The first desktop sign-out left its old session closing, so it
preserved the original worker. After draining the old sessions, normal
graphical sign-in on the same VM boot started a fresh systemd user manager
and worker invocation with byte-identical registration and configuration.
Both disposable registrations were removed, the backing image hash matched,
and the VM stayed running. This covers native supervision.
Fresh hardware-key enrollment and installed GUI activation need separate
evidence for this candidate.

An Alpine 3.24.4 amd64 VM booted a writable overlay of the signature-verified
cloud image. A fresh `qualification13` APK install exposed an OpenRC detection
error on the real `rc-service (OpenRC) 0.63.2` version banner. The installed
`qualification13.1` APK upgrade registered and activated the user service as
UID 1000 with an initialized OpenRC runlevel. A second start kept the same PID.
The worker retained its PID and polling session through 24.01 seconds of HTTP
503 responses from a loopback fixture. Killing the exact worker produced a new
PID and polling session with identical saved configuration. The owned service
and its runlevel link were removed, the base digest matched, and the VM stayed
running. The running guest later presented its native serial login prompt.
The fresh user-session and terminal-close checks are blocked on authenticated
access to that isolated guest.

The Windows Task Scheduler definition passed schema validation and the CLI
built for native x64 and ARM64. A later signed run on physical Win11 found
that Task Scheduler returns the logon trigger user as a qualified account name
and supplies the unified scheduling setting during registration. The corrected
Go backend resolved the same owner SID, matched native readback, registered and
started a temporary owner-bound task, kept one instance on repeated activation,
and verified exact task removal. This was an isolated backend executable.
The full `0.6.44+qualification13.2` amd64 CLI later activated a separate
owner-bound task on physical Win11. It polled an isolated loopback broker,
recovered from an HTTP 503 in the same worker session, and preserved the
running instance on repeated service startup. A separate signed check confirmed
the exact task and private state were absent after cleanup. The candidate was
delivered privately through the installed `0.6.39` worker. MSI installation,
actual logon triggering, and native ARM64 execution remain open.
Two later crash tests on physical Win11 stopped only the temporary CLI
worker. The demand-started task did not restart within 88 seconds or during
the second 135-second observation. On the second run, native readback kept
reporting Ready, zero running instances and no next run despite the configured
one-minute restart interval and three attempts. A separate native time trigger
also failed to restart after a forced exit. Both tested paths returned the
same native result, `FFFFFFFF`. The temporary registrations were removed.
A disposable two-phase registration test verified that suppressed registration
left the task idle through one minute. A guarded update armed periodic startup
and launched the owned action. After a forced exit, the next repetition started
a replacement in 60.307 seconds. The subsequent repetition preserved a single
instance. The Dockpipe candidate uses this verified registration ordering and
the full `0.6.44+qualification15` x64 CLI confirmed it on physical Win11.
Repeat activation kept the same PID and polling session. A forced exit
recovered in 58.501 seconds with a new PID and polling session and identical
worker configuration. Thirty-three successful polls across a later scheduled
tick confirmed one instance. Explicit replacement started one new worker with
the same configuration. The isolated task, worker process and private state
were removed. A retry was needed after the first fixture attempted cleanup
while the worker still held its state lock. The existing installed worker and
pairing remained in use. The q15 MSI upgrade, actual Windows sign-in and
native ARM64 startup remain open.

The Windows SCM supervisor uses the paired owner's ordinary console token and
puts the worker and optional sender in separate Windows job objects. It supports
either configuration with or without administrator-input enrollment. The
service and registration tests built for Windows x64 and ARM64. The private
Win11 x64 upgrade MSI built in WiX and verified with the Win11 test certificate.
The signed 0.6.45 qualification MSI installed on physical Win11 x64. Native
readback showed the automatic service with one worker and one sender in the
paired owner's console session and both legacy tasks disabled. The worker
went offline during the planned restart. After sign-in on the new boot, the
automatic service again had exactly one worker and one sender in the same
console session while both legacy tasks were still disabled. The paired
Terminal worked and the sender advertised the signed build with the existing
view and input capabilities. The paired Linux desktop app rendered the
Win11 desktop and sent a Control-mode button click and keyboard string to a
disposable Windows form. The form's native receipt confirmed both inputs.
Both legacy tasks were exported for rollback
and removed. A separate native task enumeration confirmed zero matching
registrations.

The physical m6 worker was still running as a registered LaunchAgent under
the signed-in user, with `RunAtLoad` and `KeepAlive` enabled and no controlling
terminal. The separate candidate crash-recovery check needs a private delivery
route for the native test binary. The existing agent and login were kept in use.

A disposable Gio session launched the Linux XDG entry, confirmed
single-owner startup, and verified ownership released when its session socket
disappeared. Installed Slackware desktop sign-in and broker delivery remain open.

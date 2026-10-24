# Qt Guest Tools and package-contributed VM management

Objective contract:

```yaml
objective_id: TASK-045
state: executing
execution_skill: dorkpipe-objective-execution
execution_authority: explicit_user_request
authorized_objective: Finish native Windows validation, then build a Qt Guest Tools application inside the VM with durable automatic provisioning and a host VM tab activated through participating package contracts.
done_when:
  - Native Windows extraction validation runs through Dockpipe and records actual results.
  - Windows boot repair survives restart and reconciles incomplete provisioning.
  - A deployed Qt Guest Tools app runs visibly inside Windows and reports real setup progress, success and actionable failures.
  - Participating installed packages activate the host VM tab through a documented compatible contract.
  - Provider actions and configuration remain package-owned, with focused contract and UI checks.
  - Full installed Windows and macOS launchers exercise the real VM page, Canvas and Tiling through Dockpipe.
  - Pointer integration reaches guest edges correctly and handles software and hardware guest cursors.
  - Resolution changes are available through the VM package UI and YAML workflow contract.
  - Remaining extraction boundaries and pinned dependencies are prepared for staging, then the authorized Dockpipe staging release is verified.
inherited_invariants:
  - Preserve the extraction checkout, unrelated authored work and original Windows base image.
  - Use Dockpipe for Windows validation and VM lifecycle operations.
  - Keep GPU passthrough disabled.
  - Share provisioning policy between the guest GUI and durable startup repair.
  - Preserve accounts, credentials and existing host identities; do not log secrets.
explicit_exclusions:
  - Production publication remains excluded. The user explicitly authorized dependency staging and a Dockpipe staging release on 2026-10-23; preserve unrelated work and use reviewed runtime-owned Git operations.
checkpoint_policy: automatic_within_objective
verification_policy: Finish Windows validation first, then focused implementation tests and visible guest/host UI proof.
handoff_policy: user_requested_only
```

Current checkpoint (2026-10-23): the final native catalog source
`441a0a695f4876791ab54bed59264b0c8dfb32ca` passed Linux amd64 and Mac arm64
display suites (ten tests each), Windows amd64 (nine tests), package compilation
and public candidate publication. Independent public readback verified all seven
assets, source/toolchain evidence and eight package identities on those targets.
The exact candidate is pinned in the core consumer; the launcher and host SDK stay
in core, and VM implementation stays in the optional package.

Full installed Windows/macOS MainWindow, VM page, Canvas and Tiling acceptance
still needs the final owning desktop build. The preserved Windows guest's recorded
QEMU/TPM processes are stopped; no lifecycle action was taken after observing that.
Continue only with its existing overlay, firmware variables, TPM and identity,
checked offline, then the planned single continuation boot. The 1920-by-1080 boot
preference and strict QWidget-to-guest edge/corner proof remain unqualified.
Catalog staging promotion is tracked in catalog PR #2; final Dockpipe staging
publication waits for installed acceptance. The older checkpoints below retain
their original scope and do not substitute for these remaining gates.

The user explicitly authorized this order after clarifying that the requested Qt
application runs inside the guest, complementing a host top-level VM tab rather
than replacing it. The existing launcher does not yet implement generic
package-contributed top-level pages; its dynamic dashboard and installed workspace
package lifecycles are candidate seams, not proof of a VM contract.

Historical checkpoint: native Windows validation. The SSH service and key-authenticated
connection work after correcting missing host keys and their SYSTEM ownership/ACL
requirements. The durable provisioning task is installed; final helper repair and
portable native test tools are being reconciled in an ephemeral guest overlay.
Source artifacts and installer/provisioning receipts remain under
`/tmp/dockpipe-extraction/windows-validation/`. No guest GUI or host VM-tab
implementation has started before this checkpoint passes.

Execution direction confirmed by the user: select PowerShell explicitly for
Windows shell scripts, retain Bash as an explicit compatibility choice, and use
direct C++, Go or Python processes for shared platform logic. Do not add another
Bash-to-native path conversion layer to the Guest Tools flow.

Native checkpoint progress: PipeLang Windows build and the unchanged portable
acceptance tests passed through pinned Dockpipe 0.6.25 under the logged-in user's
limited token with an explicit native PATH. A Linux ELF/mmap research helper now
consistently declares its Linux platform boundary. Catalog acceptance remains in
progress. Current-runtime private-state creation tests passed natively under the
elevated Windows token, with existing wrong-owner rejection preserved.

The user additionally authorized a general native-host-shell default with an
explicit shell override. The implementation preserves `.sh`/`.ps1` interpreter
families, selects Windows PowerShell for Windows inline commands and Bash on
Unix, and removes the unconditional Bash admission gate. Native Windows proof
without Git Bash in PATH is pending the current update/restart.

Both native Windows extraction acceptance workflows have passed. The catalog
Windows matrix now explicitly covers its supported compatibility packages; Linux
qualification UID/mode/fsync checks remain unchanged in the full Linux lane. The
Windows manifest fixture replaces only host paths before production validation.
The durable restart test has deliberately removed the guest-agent startup task,
disabled the SSH firewall rule and set SSH startup to manual. Windows is applying
updates during that restart; boot repair readback remains pending.

Verified checkpoint: both native extraction acceptance workflows passed through
Dockpipe. After an actual Windows restart, the SYSTEM boot task restored automatic
SSH startup, the firewall rule and the missing guest-agent task before login.
The native shell default and explicit PowerShell/Bash choices passed in Windows,
including PowerShell execution with Bash absent from PATH. The shared provisioning
helper now publishes bounded public progress and supports detached registration.

The Qt application was built inside Windows with the explicit Qt 6.10.3 kit,
installed in an immutable ProgramFiles version directory, and observed visibly in
the logged-in limited user session. It reports actual current-boot readiness.
Generic package-contributed host page tests pass; the QEMU provider and installed
artifact integration remain in progress.

The user additionally requested working Linux-host/Windows-guest clipboard so they
can enter their own Windows product key later. Activation is outside this work.
Clipboard contents must remain in private stdin/local IPC, never operation logs,
arguments or environment variables. Interactive guest clipboard support belongs
in the logged-in Qt application, separate from SYSTEM boot provisioning.

Additional user-authorized integration: expose the VM display as a generic native
surface in the dashboard canvas and tiling. Reuse native acquisition/release with
fresh process identity checks; closing a panel releases the display and does not
shut down the VM. Keep the ordinary external QEMU window and CLI lifecycle usable.
Windows installation media and guest disk images remain operator-supplied and are
never included in Dockpipe packages or Guest Tools artifacts.

Verified clipboard checkpoint: the final extracted package closure passed actual
Linux-host to Windows-guest and Windows-guest to Linux-host Unicode transfers.
Original host MIME data and guest text stayed in memory and were restored with
conflict checks. The final Windows app also passed CRLF/LF, invalid-request and
atomic clipboard compare-and-set tests. Package-owned runtime media staging
verified all 36 Guest Tools files and uses the canonical core provisioning helper;
no OS installation media or guest disk is present.

UX correction from the user: the VM page must display the selected VM directly.
The dashboard dock's Canvas/Tiling picker must independently offer live VM display
resources; attaching from the VM page is not the required navigation flow.
Native display process identity must come from the X server and public kernel
telemetry. The new package display path is explicitly Linux/X11-host only while
Windows public process telemetry is unavailable. Existing manual desktop capture
behavior is outside this change.

During long-running native display qualification, five-second provider snapshots
filled the recent execution cache and evicted the still-running owner record.
The generic observer now merges runtime-owned active associations back into its
bounded result after checking exact owner kernel identities. Regression tests
cover history eviction, reused PIDs and linked registry entries; actual host
readback recovered the unchanged VM owner and QEMU identities. Native rendering
qualification remains pending the focus protocol diagnostic.

Actual Windows pixels are now confirmed inline in the corrected screen-dominant
VM page after the user's click focused the native view. This is the real QEMU
client, with the Windows desktop and Store visible, rather than a capture stream.
The earlier black viewport must not be reported as a continuing rendering failure;
normal click-to-focus is retained, and final Canvas/Tiling handoff checks remain in progress.

The user selected a more portable direction after seeing native window embedding:
run QEMU without its own window and supply a package-owned Qt display frontend via
QEMU's local display API. Shared framebuffer/texture transport is preferred over
remote desktop protocols. Core should only provide a generic versioned native
widget factory reused by the page, Canvas and Tiling. QEMU/GIO/input/rendering
implementation stays in the VM package. Normal terminal GUI launches retain the
ordinary QEMU GUI. This replaces window reparenting as the intended durable design;
the working Linux/X11 window path remains a qualified interim implementation.

The installed QEMU 6.2 predates the D-Bus display backend, so an official pinned
QEMU 10.2.4 source build and a disposable firmware-only display proof are being
prepared separately. The existing logged-in Windows VM stays running. No
cross-platform display claim is made until the actual Linux, Windows and macOS
builds and frontend integrations are qualified. Actual prior native inline and
Canvas display proofs passed; Tiling has fixture proof but still lacks a live
Windows visual proof.

The first headless native Qt proof passed through a task-owned Dockpipe workflow:
real SeaBIOS output, two shared-memory scanouts (640x480 to 720x400), 307 dirty
updates, six input acknowledgements, and a joined transport worker. The disposable
VM had no disks or network and its owned children were cleaned up. The viewer
uses bounded reads from the shared framebuffer FD rather than mapping unsealed
memory, so it does not claim zero-copy rendering. Raw pixel fallback resizing is
not yet qualified and must fail closed. Installed-SDK widget factory and package
supervisor integration remain in progress; Windows and macOS host frontends are
not qualified by this Linux proof.

Installed-candidate checkpoint: the package-owned Go launch shim, private bus
supervisor, public process association, package-native snapshot, installed Qt
factory, frame/input delivery, handoff and return all passed together through a
Dockpipe firmware-only workflow. The daemonized QEMU, supervisor and bus exited
with exact identity checks, and private state/socket directories were removed.
The final source sync preserved unrelated work; the core library sweep and
focused Qt/SDK/privacy checks passed. No commits or publication occurred.

The subsequent host-adapter phase implements Windows Win32.Map, macOS Unix.Map,
native input maps, and platform process/file admission. Native Windows and macOS
Go identity/lifecycle tests and Qt tests pass; complete real-backend qualification
is tracked below. GPU/zero-copy rendering remains unimplemented. The native path
requires QEMU >=10.2.4; stock terminal GUI QEMU remains separate.

The existing encrypted Windows guest now boots through the headless local display
API on Linux. A clean shutdown preserved its complete overlay, original backing
image, firmware and TPM; the test uses a separate child overlay and copied boot
state with the original machine version, UUID and device identities. BitLocker
stayed enabled and the original base image was not modified. The user logged in
through the Qt frontend. Actual desktop and Guest Tools pixels passed the
installed factory's first/handoff/return tests with each view observed beyond
its startup deadline; closing the views leaves QEMU running. Clipboard is enabled
through the existing package bridge, without another content-transfer test.

Windows' graphics driver switches from shared scanout to raw local D-Bus frames.
Readiness now accepts a validated frame from either delivery mode. Raw damage
uses bounded batching and asynchronous authoritative console geometry; rectangles
cannot expand the canvas. The actual Windows factory reports raw-local delivery.
A raw-only firmware resize/input proof also passes. These are local QEMU API
frames, with no remote desktop transport, window reparenting or zero-copy claim.

The real reboot exposed clock correction after startup: an otherwise healthy
provisioning result had a future timestamp and the Qt application rejected it.
The canonical helper now registers both startup and a delayed system-time-change
trigger on its existing SYSTEM task. Native regression tests, actual registration
and refreshed current-time readiness pass. The public package status reader also
rejects invalid/future timestamps instead of reporting misleading readiness.

Generic core process collectors now support Windows FILETIME identities and
macOS start-time identities with bounded discovery, parent checks and cancellation.
Native tests passed on both hosts. Process arguments and environment are excluded.
The complete core library suite passed after the sandbox-only local socket denial
was rerun in the reviewed host lane. No package-specific collector code was added.

The paired Apple Silicon Mac executes explicit source-delivery workflows through
Dockpipe. Its exact Qt development kit is task-private; its installed Dockpipe app
is unchanged. Native Qt tests pass after canonicalizing the temporary-path fixture,
without relaxing admission. The Windows development VM now also has the matching
MSVC toolchain and exact Qt kit; matching plugin compilation and native platform
tests pass. Real macOS and Windows-host QEMU frame/input/factory proof remains in
progress, and does not follow from cross-builds or standalone unit tests.

The final headless frontend also renders the logged-in Windows desktop and ready
Guest Tools directly in the VM tab, from the independent Canvas widget picker,
and from the Tiling dock application picker. Closing the Tiling view leaves the
same QEMU process running; returning to the VM tab reacquires live pixels. The
proof used a minimal task-owned dashboard. An earlier capture lost its window
handle; host-visible process verification confirmed the launcher had not exited.
That probe failure is not evidence of an application crash.

Native Windows MSVC qualification now passes all six platform tests without
skips. Real Windows-host QEMU shared-memory delivery, resize, input, installed
plugin first/handoff/return views with actual nonuniform pixels, and exact owned
process cleanup passed. This qualifies the package plugin; a full installed
Windows desktop launcher route has not been exercised by that fixture.

Final artifact checks caught broker executable permissions stripped by package
normalization. The package is moving its broker into the existing generic
`tooling/bin` executable seam; no QEMU-specific engine mode rule is added. On the
Mac, the private broker patch additionally requires CMake to detect and export
`getpeereid`, preserving both kernel UID and PID credentials for authentication.
That checkpoint preceded the native Mac frame/input/factory proof recorded below.

Real Apple Silicon macOS qualification now passes with the task-private patched
D-Bus broker: kernel service PID and UID match the expected QEMU process, shared
frames resize to 720x400, input is acknowledged, and the transport joins cleanly.
All three installed-plugin views remain ready with visible nonuniform pixels,
and owned QEMU/broker processes and private runtime state are cleaned up. The
full installed macOS launcher route remains outside this minimal factory proof.
The distributable Mac dependency closure still requires verified GLib license
texts from its pinned official source; that packaging gate has not been waived.

The full installed Linux shim proof also exposed a broker compile-prefix config
dependency. The package supervisor is being corrected to create its own bounded
private session-bus configuration rather than depend on a system session.conf.

The final Linux resolver artifact now passes the complete public Dockpipe shim
proof using its own packaged broker and private configuration, with no broker
override or system session.conf dependency. The actual broker executable is
bound to the extracted artifact. Public snapshot, shared frames, input, three
factory views and close/cleanup all pass; every disposable process exits and
private state is removed. All 33 extracted display inventory hashes match. The
archive contains no operating-system media or disk image.

The complete native Mac producer artifact now passes independently of ambient
Homebrew dependency lookup: its packaged broker and module deliver shared frames,
input and three visible factory views, followed by exact child cleanup. All 32
inventory hashes match. The missing GLib license texts are supplied from the
verified pinned upstream 2.90.1 source, without a license waiver. Windows isolated
artifact dependency loading remains the final packaging gate; the earlier
native Windows rendering proof alone does not satisfy that gate.

Final checkpoint: the bounded objective is complete. Both native Windows
acceptance workflows, actual durable boot repair, the visible Qt Guest Tools
application, clipboard validation, real Windows headless display on Linux, and
package-contributed VM/Canvas/Tiling views pass. Native Windows and macOS display
adapters also pass their complete isolated package-artifact proofs, with real
frames, input, handoffs and cleanup. The generic Windows loader admits contained
dependencies after ABI validation using native path separators and scoped safe
loader flags; it does not mutate PATH or the current directory. Initial page
selection now prefers a display over historical entries and preserves user choice.
All six focused host regressions pass. Guarded source sync preserves unrelated
postimages; no commits or publication occurred.

The full installed Windows/macOS launcher routes and public Go-shim lifecycles on
those hosts remain unexercised; their passing scope is the native display adapter
and installed-plugin contract. The Windows VM workflow still declares Windows
and Debian hosts. These qualifications do not add a Mac guest-launch workflow,
GPU transport, zero-copy rendering, OS images or automatic Windows activation.
Final task-local evidence is indexed by
`/tmp/dockpipe-extraction/windows-validation/host-adapters-validation.json` and
`vm-tools-integration-checkpoint.json` beside it. The existing Windows VM remains
running, with Guest Tools visibly ready in the final Dockpipe VM tab.


### 2026-10-23 installed-launcher and staging continuation

The user extended the objective to the full installed Windows/macOS launcher
flows, corrected pointer integration and package-owned resolution controls, then
completion of the extraction/dependency staging work and a Dockpipe staging
release. This is an active continuation; earlier standalone native plugin proofs
remain useful baseline evidence, not proof of the full installed application.

Both hosts now have task-private complete Qt 6.10.3 build kits. Native application
qualification is being added behind an explicit build option, with a separate
native settings namespace before startup singletons. It must drive the real
catalog, VM page, Canvas/Tiling pickers, input and view cleanup.

Pointer inspection found an endpoint scaling error, uncaptured relative input,
and ignored QEMU hardware-cursor messages. The package owns the correction.
Resolution support uses the interactive Guest Tools application and modes offered
by the Windows display driver, exposed through a public typed package workflow.
No GPU passthrough, bundled operating-system media or production release is part
of this continuation.

Staging prerequisite inventory is in the task runtime receipt
`/tmp/dockpipe-extraction/full-launcher-validation/staging-prerequisites-readonly.json`.
Fresh immutable SDK/package artifacts and release input pins must be verified
before claiming the new native VM features are present in a staging release.


The subsequent Linux real-MainWindow diagnostic passes the VM page, actual
Canvas/Tiling pickers, transport-acknowledged Shift input, all five nonuniform
frame captures, view close/reopen and exact owned QEMU/broker/supervisor cleanup.
It used the development Linux application and installed package closure; it is
not an installed Windows/macOS application claim. Both native packaged apps and
fresh pointer modules build, but the macOS installed run exposed a hidden-page
inventory lifecycle failure, which is being repaired and requalified.

Guest Tools build 8 passes native Windows tests and is installed in the retained
test overlay. Its new interactive resolution API successfully reapplies the
current mode and rejects unsupported geometry. Actual driver enumeration reports
only 1280x800, so live alternate-resolution validation remains incomplete.
Package YAML now also exposes optional startup EDID width/height preferences;
a diskless firmware fixture verifies an exact 1920x1080 EDID and checksum through
Dockpipe, without claiming a Windows mode change. A standard HID tablet has been
attached to the running Windows test guest without a restart. Exact interactive
guest corner measurement remains in progress; the first harness attempt used a
stale pre-resize content rectangle and correctly received no input acknowledgement.


The subsequent native macOS full installed development-candidate proof passes
the real MainWindow VM page, Canvas/Tiling pickers, all five nonuniform frame
captures, native Shift input acknowledgement, single-view handoff and actual
close/reopen. It uses the installed package, public Go shim/native snapshot and
foreground managed firmware-only QEMU. Independent process identity confirms
QEMU stays alive after the GUI views close; final owned QEMU, supervisor and
private broker cleanup removes metadata and socket directories. No production
collector change was justified by the earlier daemonized fixture failure.
Evidence: `/tmp/dockpipe-extraction/macos-host-validation/foreground-live-proof-result-37/extracted`.
The protected `/Applications/DockPipe.app`, existing Windows guest and clipboard
remain unchanged. This proves a task-private installed macOS host flow, not a
macOS guest-launch workflow or a published desktop release. The full installed
Windows application route remains pending its separate native gate.

### Installed Windows launcher qualification complete (2026-10-23)

The task-private complete Windows product runtime (165 deployed files, clean
owning packager stderr) passes the real MainWindow VM page, Canvas picker,
Tiling Apps picker, return, close and reopen flow. All five original PNGs show
SeaBIOS firmware text; decoded opaque pixel data independently verifies the
reopened frame. A reduced image preview initially obscured that text, so no
renderer fix or acceptance threshold change was made. Input acknowledgement,
single-resource leases, disconnect and exact-owned-process cleanup pass.

The final provider was built through the owning native Python source hook with
8/8 native CTests. The public Go supervisor exposed and now fixes canonical
D-Bus value escaping (`%3a` versus `%3A`) while retaining strict endpoint/SID
checks. The Windows native Display SDK consumer and Python host/package source
hooks also pass; Bash is absent from the latter fixture.

Evidence: `/tmp/dockpipe-extraction/windows-full-launcher/full-installed-windows-checkpoint.json`
and `full-installed-proof4/evidence/original-image-review.json`. Provider SHA-256
`8dfedd83fc926619bb34c6105d6ac63d46b185b2546e4a199527ddd2a69bade3`.
Scope is an installed task-private product payload and diskless/networkless
firmware VM, not an MSI upgrade or global installation. The original logged-in
Windows VM, disk, TPM and clipboard remain preserved. macOS's installed native
flow remains qualified by proof 37; native Mac Python smoke passes proof 43.

Staging remains in progress. New public source/CLI/Display SDK artifacts and
package-store pins must be published and read back before a staging application
release is claimed. No production promotion is authorized or reported.

# Platform expansion matrix

The baselines below define the OS and architecture to test. The
[registry](registry.json) includes the same profiles in this order. A selected
run must record the exact image or installation media hash, running OS release
and architecture, and both Dockpipe artifact hashes. For rolling distributions,
keep the package sources on the selected snapshot and record their identities.
For Windows and WSL 2, also record the Windows build. Record the WSL and WSLg
versions and verify that the selected Ubuntu instance is running in WSL 2 mode.

The [remote and Browser role matrix](REMOTE-MATRIX.md) covers installed feature
checks across these and the existing platform profiles.

Kubernetes pod environments are unsupported across these profiles.

| Target | Release / snapshot and desktop | Dockpipe install and upgrade path | Status |
| --- | --- | --- | --- |
| Fedora Workstation | [44](https://docs.fedoraproject.org/en-US/fedora/f44/release-notes/), amd64, GNOME | DNF RPM for CLI, Flatpak for GUI | RPM install, upgrade and workflow passed. Installed Flatpak CLI and GUI workflows, portal History, and cold-launch portal editor save and run passed on private candidates. Released Flatpak 0.6.38 passed the installed local Browser happy path. An offline update of the same user Flatpak ref to private 0.6.44 verified the replacement CLI digest and retained project bytes. This build used a privately qualified Browser SDK |
| Linux Mint | [22.2 Zara](https://blog.linuxmint.com/?p=4881), amd64, Cinnamon | APT CLI and desktop DEBs | Released 0.6.38 installation, upgrade to private 0.6.39, CLI and GUI workflows, and preference retention passed in the VM. All ten installed local Browser assertions passed on the private candidate. The normal desktop build with the pinned public Browser SDK is blocked |
| Arch Linux | [2026.11.01](https://archlinux.org/releng/releases/2026.11.01/), amd64, KDE Plasma Wayland | Pacman CLI package, Flatpak for GUI | Pinned Arch Linux Archive VM passed CLI install, upgrade and workflows with 0.6.38 and 0.6.39. Installed Flatpak 0.6.38 and private `dev-portal-editor` passed GUI workflows, with workspace and preferences retained through the upgrade. All 10 installed local Browser assertions passed, including temporary profile cleanup across a verified process restart. Remote viewer checks require X11 |
| openSUSE Leap | [16.0](https://doc.opensuse.org/release-notes/x86_64/openSUSE/Leap/16.0/release-notes-leap-160_color_en.pdf), amd64, KDE Plasma X11 | Zypper CLI RPM, Flatpak for GUI | In the desktop VM, CLI install, upgrade and workflows passed with 0.6.38 and 0.6.39 RPMs. GUI launch and saved workflows passed with Flatpak 0.6.38 and the private `dev-portal-editor` commit. Workspace and preference retention passed. All ten installed local Browser assertions passed on the private Flatpak |
| openSUSE Tumbleweed | [20261117](https://cdn.opensuse.org/tumbleweed/iso/), amd64, KDE Plasma Wayland | Zypper CLI RPM, Flatpak for GUI | Pinned offline DVD VM passed CLI install, upgrade and workflows with 0.6.38 and 0.6.39. Flatpak 0.6.38 and private `dev-portal-editor` passed installed GUI workflows. The updated running commit, workspace and preference were verified after an explicit tray Quit. All ten installed local Browser assertions passed on the private Flatpak in KDE Plasma Wayland |
| Red Hat Enterprise Linux | [9.6](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/9/html/9.6_release_notes/index), amd64, minimal | DNF CLI RPM | VM blocked pending a verified 9.6 Binary DVD or entitled repository access. The anonymous official DVD request reached the Red Hat content gateway and returned HTTP 400. [Official download and SHA-256](https://developers.redhat.com/products/rhel/download) |
| Red Hat Universal Base Image | [9.6](https://catalog.redhat.com/en/software/containers/ubi9/618326f8c0d15aff4912fe0b), amd64, container | DNF CLI RPM with signed public UBI 9 BaseOS and AppStream | Pinned image passed clean 0.6.39 install, in-place upgrade from 0.6.38, and workflows. The public repositories supplied newer RHEL 9 dependencies. Full RHEL VM qualification is tracked separately |
| Rocky Linux | [9.6](https://docs.rockylinux.org/latest/releases/release_notes/9_6/), amd64, container | DNF CLI RPM with the signed 9.6 vault repositories | Install, upgrade and workflow passed in the pinned container |
| Amazon Linux | [2023.11.20260526](https://docs.aws.amazon.com/linux/al2023/release-notes/relnotes-2023.11.20260526.html), amd64, container | DNF CLI RPM with the pinned release repository | Install, upgrade and workflow passed in the pinned container |
| Alpine Linux | [3.24.4](https://www.alpinelinux.org/posts/alpine-3.21.10-3.22.8-3.23.8-3.24.4-released/), amd64, container and supplemental VM | APK CLI package | Install, upgrade and workflow passed in the pinned container. The offline VM independently passed APK installation, the in-place upgrade from 0.6.38 to 0.6.39, and real workflows as an unprivileged user. The VM shut down and the R2-reconstructed base SHA-256 matched after the run |
| Kali Linux | [2026.3](https://www.kali.org/blog/kali-linux-2026-3-release/), amd64, Xfce X11 | APT CLI package, Flatpak for GUI | Verified offline DVD VM passed CLI install, upgrade and workflows with 0.6.38 and 0.6.39. Flatpak 0.6.38 and private `dev-portal-editor` passed installed GUI workflows. The upgraded process, retained workspace and preference were verified after File > Quit. Flatpak dependencies came from the signed, pinned snapshot. All ten installed local Browser assertions passed on the private Flatpak |
| NixOS | [26.05.14630.f68407657ad2](https://releases.nixos.org/nixos/26.05/nixos-26.05.14630.f68407657ad2/), amd64, GNOME Wayland | User-local CLI with `programs.nix-ld.enable`, Flatpak for GUI | Default loader rejected the unmodified CLI. With `nix-ld` at the pinned nixpkgs revision, the published portable installer installed 0.6.33, upgraded to 0.6.38 from verified offline staging artifacts, and ran the same workflow on both versions with its project inputs retained. A separate payload run verified 0.6.38 and 0.6.39. The released and private `dev-portal-editor` Flatpaks ran the GUI workflow and retained the project and preference. All ten installed local Browser assertions passed on the private Flatpak in GNOME Wayland. The offline VM shut down and both backing image digests matched |
| Gentoo Linux | [20261108T170107Z](https://distfiles-cdn.gentoo.org/releases/amd64/autobuilds/20261108T170107Z/), amd64 desktop OpenRC stage3, Xfce X11 | User-local portable CLI, Flatpak for GUI | Installed the signed stage3 with Portage at [21d6eb4c64df](https://github.com/gentoo/gentoo/commit/21d6eb4c64df9a2377d746e8195f9b8b56ca1cce) and booted kernel 6.18.62. The released portable installer installed 0.6.33 and upgraded to 0.6.38 using verified staging artifacts on read-only offline media. Both versions ran the same workflow and the project hashes were retained. The documented release catalog returned HTTP 404 from the runner. A separate payload run exercised 0.6.38 and 0.6.39. Flatpak 0.6.38 and private `dev-portal-editor` launched from Xfce and ran the GUI workflow with the project and preference retained. All ten installed local Browser assertions passed on the private Flatpak. The overlays passed inspection and their parent digests matched |
| Slackware64 | [15.0](https://www.slackware.com/announce/15.0.php), amd64, Xfce X11 | User-local portable CLI, Flatpak for GUI | The published portable installer installed 0.6.33 and upgraded to 0.6.38 using verified staging artifacts on read-only offline media. Both installed versions ran the same workflow and the project hashes were retained. A separate payload run verified 0.6.38 and 0.6.39. Flatpak 0.6.38 and private `dev-portal-editor` launched from Xfce and ran the GUI workflow with the project and preference retained. Flatpak prerequisites came from a signed Slackware 15.0 third-party repository. With the supplied setuid Bubblewrap, Browser exited before loading the test app. All ten installed local Browser assertions passed after switching the disposable guest to [unprivileged Bubblewrap](https://github.com/flatpak/flatpak/wiki/User-namespace-requirements). The VM shut down cleanly and the base digest matched |
| Physical m6 | macOS 27.0.1 build 26A434, ARM64 | Existing native installation and private paired worker | The persistent worker LaunchAgent resumed automatically after broker recovery. The installed Pop!_OS operator GUI displayed live system and process data. A supervised start restored the separate Remote Desktop sender, and its installed paired session and Docker inventory remain open. See the [role matrix](REMOTE-MATRIX.md) for the exact limits |
| Physical Win11 | Windows 11 Enterprise 24H2 build 26100, amd64 | Existing 0.6.39 CLI and GUI from the native MSI | Delivered PowerShell, project initialization, workflow discovery, local Files and local Git passed on the installed CLI. The installed window was visible through UI Automation. Fresh MSI install, upgrade and interactive GUI journeys remain open on this host |
| Windows 11 Pro ARM64 | [25H2](https://learn.microsoft.com/en-us/windows/release-health/status-windows-11-25h2), ARM64 | Native ARM64 CLI and GUI installer | The developer CLI workflow builds a verified Windows ARM64 executable. Physical m6 reports hardware virtualization and has `qemu-system-aarch64`, with guest boot still unverified. The [public Microsoft download](https://www.microsoft.com/en-us/software-download/windows11arm64) currently serves 26H2. Published Qt 6.10.3 ARM64 base packages are available. The checked installer catalogs had no matching WebEngine binary. The [pinned WebEngine source checksum](https://download.qt.io/archive/qt/6.10/6.10.3/submodules/qtwebengine-everywhere-src-6.10.3.tar.xz.sha256) is published. Exact 25H2 media, native execution, the ARM64 SpatialWeb SDK and the matching GUI installer remain pending |
| WSL 2 | [Ubuntu 24.04 LTS](https://learn.microsoft.com/en-us/windows/wsl/install-manual), amd64, on Windows 11 Pro 25H2 amd64 with [WSLg](https://learn.microsoft.com/en-us/windows/wsl/tutorials/gui-apps) | APT CLI and desktop DEBs inside the distro | The SHA-256-verified Ubuntu 24.04.5 image ran as a dedicated WSL 2 distro on physical Win11. APT installed the private CLI DEB and upgraded it in place. The installed CLI ran a non-root workflow at both versions and preserved the project input. WSLg and the Dockpipe GUI remain open |
| Windows Server | [2025 Standard](https://www.microsoft.com/en-us/evalcenter/evaluate-windows-server-2025), amd64, Desktop Experience | Windows amd64 MSI with desktop feature enabled | Manual VM setup by the operator |

## Required observations

1. On a fresh target, install the selected prior Dockpipe version through the
   selected install path. Capture package ownership where applicable, installed
   executable identity and version, and CLI launch from a normal user session.
2. Initialize a new project with the installed CLI. Run a real host-step
   workflow that invokes the installed CLI and writes its version to a project
   file. Record the result and hash the project configuration and workflow.
3. Upgrade Dockpipe in place using the same install path. Verify installed
   ownership and the replacement version. Confirm the project hashes and prior
   result survive. Rerun the same workflow and verify it appends the new version
   exactly once.
4. On each desktop target, install and launch the actual GUI from its registered
   entry point. Open the generated project, run the workflow from the GUI and
   verify the result. Set a harmless preference before the upgrade and verify
   it after upgrading and relaunching. On WSL 2, launch the Linux GUI via WSLg.
   For Flatpak, use two explicitly selected bundles. Quit the running app from
   its tray menu if closing the window leaves it active. Confirm that the old
   instance exited and that the relaunched instance uses the upgraded commit.
   Record both app commits and the runtime separately from the native CLI package.
5. Admit the observations under `install.cli`, `install.upgrade` and, where
   applicable, `install.desktop` and `gui.workflow-studio`. The complete
   target report retains any uncovered cases. Record missing media, artifacts,
   drivers or product compatibility as pending or blocked.
6. On every supported Windows and Linux CLI profile, enable the paired
   background worker as the enrolled user. Verify that the native supervisor
   owns the worker, the worker keeps polling after the launching terminal
   closes, and a repeated activation leaves one instance. Verify recovery
   after an unexpected worker exit, automatic startup at the next login or
   native user-service session start, and relay reconnection with the same
   pairing. Record the actual supervisor and installed candidate. Container
   profiles need an admitted native service session for this role. The
   [background worker matrix](REMOTE-MATRIX.md#background-worker-startup-and-recovery)
   has the current evidence.

The [package upgrade driver](DRIVER.md#pinned-linux-package-upgrade-containers)
recorded passing install and upgrade receipts for the four container profiles
using the released 0.6.38 CLI repacked into the baseline RPM and APK and the
locally built 0.6.39 candidate packages. The package managers reported both
versions and the workflow recorded them in order. Each candidate also passed a
clean installation and workflow run. The runner verified the evidence hashes
and exact container cleanup.

The same driver then ran all four container profiles concurrently with plans
generated by `prepare-plan.py --package-profile`. Each pinned image passed a
clean private 0.6.44 installation and an in-place upgrade from 0.6.39.
Independent evidence recorded the original and upgraded workflow versions
with matching project hashes. All four fixture containers were removed,
and the pre-existing container and running-container inventories matched.

After the qualification host restart, a fresh Fedora 44 Workstation install
from the signed `44-1.7` media booted on a writable overlay. With SELinux
enforcing and only loopback networking, DNF installed the 0.6.38 RPM and
upgraded it to the private RPM carrying `0.6.44+qualification9`. The same
unprivileged workflow wrote both versions in order and verified the original
project hashes. The VM remains running for desktop qualification.

The same Fedora guest installed the checksum-pinned 0.6.38 Flatpak and its
KDE runtime from read-only offline media. The installed Flatpak CLI executed
the generated workflow. GNOME launched the registered Dockpipe entry, and
its GUI opened the project and ran the same workflow. Independent readback
found exactly two markers and matching input hashes. The Dashboard sidebar
preference was changed in the baseline for the next upgrade check. The
current private Flatpak upgrade remains unrun in this guest.

The Windows MSI packager now accepts an explicit ARM64 target and validates
native executable architecture before invoking WiX. Physical Win11 x64 built
private CLI-only MSIs for both architectures and rejected mismatched payloads.
Independent MSI inspection found `Arm64;1033` and `x64;1033` platform metadata.
Extraction matched both supplied CLI and worker binaries byte for byte.
Native ARM64 install, execution, upgrade and uninstall are still unrun.

The full ARM64 desktop installer still needs a pinned ARM64 SpatialWeb Browser
SDK and a matching Qt and CRT deployment. The Windows desktop release producer
currently stages amd64. Native ARM64 testing also needs verified 25H2 media
and a native guest. The [Microsoft Update Catalog](https://www.catalog.update.microsoft.com/ScopedViewInline.aspx?updateid=42ed69d5-18ef-4071-a136-4ffcf105e069)
lists ARM64 servicing update KB5132061 and prerequisite KB5043080 for the
25H2 servicing path to build 26200.9974. That path starts with an installed
ARM64 guest. See the [Browser SDK lock](../../dependencies/browser-sdk/assets/spatialweb-browser.lock.json)
and [MSI producer](../../package/windows-msi/assets/scripts/build.ps1).

The physical Win11 WSL host reported WSL 3.0.5.0 and WSLg 1.0.79 with
Virtual Machine Platform enabled. A separate Ubuntu 24.04.5 distro imported
from the verified Canonical image reported WSL version 2 and x86_64 while
Docker Desktop retained its own registration. The installed CLI ran the
generated project as a non-root user under both private DEB versions
`0.6.44+qualification4` and `0.6.44+qualification14`. The input bytes and
ordered result markers survived the APT upgrade. The named distro remains
available for further qualification. The initial WSLg window witness
did not pass, and automatic Dockpipe Remote Access lock prevented reading
the queued retry result. A full Dockpipe GUI launch and CLI uninstall are
still unverified in this lane.

An isolated Remote Desktop sender on m6 needs a compatible Apple restore
image and a dedicated macOS guest with its own graphical login and sender
consent. The current macOS producer supports ARM64 and can produce a DMG
containing the desktop installer. The existing private rebuild covers the
CLI only. See [Apple’s macOS guest setup](https://developer.apple.com/documentation/virtualization/installing-macos-on-a-virtual-machine)
and the [macOS desktop producer](../../build/desktop/macos.yml).
The m6 SDK's `VZMacGraphicsDeviceConfiguration` supports at most one
display per graphics device. A two-device configuration still needs
validation in an entitled runner and an in-guest check that it presents
two active, unmirrored displays at different scales.
Apple Clang on physical m6 type-checked a probe that builds the one- and
two-device configurations and calls the SDK validation API. Runtime
validation and guest boot remain open.
The official Qt 6.10.3 macOS catalog has universal ARM64 and x86_64
archives for WebEngine, WebChannel and Positioning. The native desktop
build still needs a full Xcode environment and the matching Qt SDK.
[Qt macOS build requirements](https://doc.qt.io/qt-6.10/macos.html)

The paired physical m6 reported a Homebrew CLI and an existing launcher
bundle with no managed desktop installer receipt or managed CLI link. The
Homebrew command and unmanaged bundle each trigger the direct desktop PKG
preinstall guard. Fresh install and managed desktop upgrade remain pending in
a disposable macOS guest. The read-only preflight preserved the physical
install.

The installed physical m6 launcher and WebEngine helper contain ARM64 code
and passed strict macOS signature verification of the ad hoc-signed bundle.
The launcher has a macOS 13 minimum. The helper's signed JIT entitlement and
all six required WebEngine resource paths were present. Gatekeeper acceptance
and rendering remain unverified.

The installed m6 desktop bundle's ARM64 CLI helper contains the exact pinned
PipeLang v0.1.0 dependency and checksum, with no replacement. This establishes
the dependency identity for the bundled executable. Typed execution,
materialization and invalid-input checks remain unrun on that helper.

The same installed helper exercised the Workflow Studio document parser
through the CLI. A read-only request returned the YAML byte-for-byte and
projected the typed input. An in-memory edit changed only the selected field.
Duplicate mapping keys produced an actionable error and no document output.
The scoped review directory and installed executable retained their identity.
Native GUI editing, saving and running remain unrun.

An authenticated operator queried one closed m6 Terminal session by its exact
reference. Two requests that changed either the worker generation or the broker
epoch returned HTTP 409 with no terminal data. The original reference still
reported the session closed afterward.

The installed operator also observed an owned m6 process in live Monitoring.
Fresh samples retained the same PID, parent PID and native start token.
After the process exited and the Terminal session closed, the next
untruncated snapshot omitted it while system counters remained available.

In a separate run, closing the exact Terminal session stopped a synthetic
process while it was still active. The independently queried session receipt
reported closed, and the next untruncated Monitoring snapshot omitted the
process 17 seconds before its timer would have expired.

The signed physical Win11 preflight reached the installed worker on
Enterprise 24H2 build 26100.9974. Windows reported a hypervisor, but
WSL and Virtual Machine Platform were disabled and the WSL services
were absent. The separate WSL 2 and WSLg lane still needs a licensed
Windows 11 Pro 25H2 guest and a graphical test session.

On physical Win11, the installed 0.6.39 executable passed all three PipeLang
SDK acceptance assertions. The embedded v0.1.0 module checksum matched the
checkout pin. The installed CLI produced identical outputs on two compiles,
evaluated the typed fixture, and materialized the expected fields. Malformed
syntax and a private method invocation were rejected.

The installed Win11 `0.6.39` worker also returned the signed result and exact
binary output for a native host workflow, with the working directories,
environment, and dotenv output verified. This run covered the CLI workflow.

The installed Win11 CLI passed all three async workflow failure assertions.
Two native PowerShell branches ran concurrently, and run history retained
both exit codes of 7. The normal continuation was skipped, the workflow
entered `finally`, and its two generated markers were removed. Both exact
branch PIDs exited, the outside control file retained its hash, and the
disposable project was removed.

The installed Win11 CLI also passed the three workflow restart-recovery
assertions. After the test terminated only its disposable CLI owner process,
both PowerShell branches exited. Fresh observers retained their original IDs
and reported the exited processes as unverified. Neither branch replayed or
ran the normal continuation, and the outside control file retained its hash.

The installed Win11 CLI also verified result and log binding for a successful
native async run. Both branch IDs advanced from running to succeeded, and both
branches and their two PowerShell child processes exited. Reversed completion
order preserved declaration-order output merging. Container detach remains
unrun.

The broker accepted the later Win11 detach-readiness diagnostic but the
worker did not claim it. The test cancelled the queued diagnostic before
native execution. A retry after workstation operator authentication was
confirmed also remained unclaimed through 26 signed status checks, and the
test cancelled only that diagnostic. Container detach readiness remains
unverified.

The same physical Win11 installed CLI passed all four project initialization
assertions in a signed disposable workflow. It initialized a selected path
with spaces and populated its existing empty workflows directory. Running
initialization twice produced the same byte-level project inventory, and
the isolated home inventory stayed unchanged.

The physical Win11 installed CLI passed the package-authoring checks in an
isolated store. It compiled and exported the resolver and workflows, ran the
packaged child twice with matching typed results, and transferred the allowed
clone byte-for-byte. The forbidden clone returned an actionable error and
left the archive hashes unchanged. The generated `.cmd` package test passed.
The separate `.ps1` hook required `pwsh`, which was absent from the test PATH.

The physical Win11 CLI passed two of three package lifecycle assertions.
Two generated archives were staged offline in an empty private user store.
The installed catalog identified both. Uninstall refused an archive outside
that store, removed the selected archive, and preserved the other archive
byte-for-byte. The surviving workflow ran from its package. HTTPS
`package install` admission remains unrun.

A separate signed Win11 run confirmed the installed `0.6.39` CLI's trust
behavior against a temporary loopback HTTPS catalog. The stock CLI refused
the generated CA both with its default trust and with `SSL_CERT_FILE` and
`SSL_CERT_DIR` set. A client using the explicit CA fetched the exact manifest,
and the Linux stock CLI installed the same archive with its hash verified.
Windows requested no archive and left its private store empty. A trusted
Windows fixture origin is still needed for the positive installation check.

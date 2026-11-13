# Remote feature qualification

Remote desktop has two independent roles: an operator views or controls another
environment, while a provider hosts that environment's capture and input.
Contained applications may use the operator role. They must not host incoming
desktop capture or control. Other independently granted remote features remain
available; a desktop restriction must not disable files, Git, terminal, container
execution, monitoring, or browser network relays.

## Current implementation boundaries

| Surface | Implemented candidate | Qualification boundary |
| --- | --- | --- |
| Generic desktop session and relay | Independent view/input grants, exact provider identity, bounded signaling and encrypted-channel relay | Provider implementation and OS permission readiness remain separate. |
| Native desktop viewer package | Linux amd64, Qt 6.10.3, X11 physical keyboard mapping | Native Linux-to-Darwin private-preview proof exists. Flatpak, contained GUI, other Linux desktop environments and distribution closure need their own execution evidence. |
| Flatpak viewer candidate | Separate KDE 6.10 / Qt 6.10.3 / GNU 15.2 build profile, viewer-only payload | SDK/Platform component suites, strict factory/runtime loading, live viewing and generated-fixture control to Darwin passed. Installed GUI and distribution qualification remain separate. No released Flatpak DisplaySDK pin is implied. |
| Native desktop sender package | Darwin arm64, macOS 13 or newer, stable application identity | Permission, capture/input, clipboard, display and keep-awake evidence are separate. A smaller captured display or synthetic 4K decoding does not prove live 4K capture. |
| Windows or Linux native capture | No backend in the current native package | A working VM display, cross-build or portable fixture does not implement a desktop sender. |
| Windows or Darwin native viewing | No viewer artifact in the current native package | Do not infer viewing support from the generic GUI or session API. |
| RDP | Requires an independently implemented resolver/provider | A generic desktop picker or the VM package does not establish an RDP implementation. |
| Remote browser presentation | Linux X11 helper embedding | Network relay and trust-sharing tests do not qualify Windows, Darwin or native Wayland embedding. |
| VM management and embedded display | Contributed by the installed VM resolver's surface and native display payload | Old artifacts without that surface cannot expose the VM tab. Package installation, install/manage UI and actual guest embedding require separate checks. |

The current Linux X11 VM candidate passed real disk-free, network-free firmware
rendering and lifecycle checks in the installed display harness and MainWindow.
Both guest installation forms were inspected and cancelled. This qualifies that
bounded display/UI path, not installation or execution of a Windows or Linux
guest OS. GUI automatic launches require a compatible embedded frontend and
QEMU DBus display; explicit external-display choices remain available.

The bounded Flatpak Platform client decoded three nonuniform 1920×1080 software
frames without saving pixels, sending input or using the clipboard. Its exact
session reached package/Core closure with ACK 1, zero lease, no remaining signals,
a cleared journal and independent native channel/child absence. This is a live
component client proof, not installed Flatpak application qualification.

A separate fresh control session delivered one physical shortcut to a generated
fixture and verified exactly one phase change. Clipboard sharing stayed disabled.
Its package/Core closure, cleared journal, native channel/child absence and exact
fixture-process/file cleanup passed independently. A competing session was
preserved during a preliminary reservation conflict; its blocked intent journal
remains separate from successful closure proof. These bounded checks do not
qualify contained clipboard transfers, every keyboard layout or the installed GUI.

The host launches desktop display resolvers through the generic
`dockpipe.remote.desktop-display/v1` envelope. The selected package validates its
own supported preferences and protocol. Provider-specific capture, encoding,
input, display creation and transport behavior belongs in that package. The
native package also accepts its legacy launch schema for existing hosts.

Incoming desktop worker admission rejects recognized Linux application/container
boundaries before OS-session initialization or readiness publication. Enabling
incoming desktop permissions is rejected there too; reading or disabling an
existing grant remains available. Detection uses Flatpak/Snap/container process
metadata and standard Flatpak, Docker, Podman and systemd container markers,
including unreadable markers. It is a capability restriction, not an isolation
boundary against an operator who controls environment variables or mounts.

## Evidence requirements

Record the exact source, provider artifact, CLI, host SDK, Qt/runtime, OS release,
architecture, desktop session type and target role with each qualification run.
Keep native hosts, VMs, Flatpak, containers and cross-compilation distinct.

Cover dashboard placement, independent browser contexts and trust, terminal and
container execution, files/Git/recovery, and desktop settings/clipboard where the
selected provider implements them. Test cancellation, revoked grants, failure
and cleanup as well as successful connection. A skipped check remains pending.

An installed guest's firmware, TPM, machine identity and disk must remain a
coherent set. Ambiguous historical identity directories are not permission to
choose a new identity, reset credentials or boot an arbitrary combination.

Platform ownership remains in [the supported-platform program](../agents/tasks/supported-platform-program/overview.md),
[native desktop](../agents/tasks/native-remote-desktop/overview.md),
[browser contexts](../agents/tasks/remote-browser-contexts/overview.md) and
[VM Guest Tools](../agents/tasks/vm-guest-tools/overview.md).

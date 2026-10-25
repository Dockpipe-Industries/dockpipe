# TASK-011 Local Windows CI VM Parity And Guest Tooling

## Current State

Local validation uses the existing installed Windows VM through Dockpipe. The
external VM package owns Guest Tools provisioning, media and native display
integration; core owns the generic runtime, launcher and host SDK. Users supply
their own Windows installation media and disk image.

The current native Windows source passed the default ALL build and complete
58-case CTest order: 53 passes, five existing platform skips, zero failures and
zero timeouts. Its normal user console remains separate from the elevated
observer used to verify existing SSH and Guest Tools service identities.

Pinned Go 1.26.10 Windows execution passed 121 tests and three existing platform
skips across the affected remote, infrastructure and package-build boundaries.
The generated guest-agent help command also passed without installing or changing
a service. Private-file test setup now uses the existing Windows ownership and
ACL helpers; production checks remain intact. Numeric31 MSI installation, ordinary
launcher UI and uninstall are the remaining local installed-flow gate.

Native macOS ARM64, Linux amd64/ARM64 and installed Linux container consumers
provide separate platform evidence. Containers cover package installation and
loader/workflow behavior; installed VMs cover desktop/session behavior. A paired
ARM64 Mac does not establish native Intel Mac execution.

## Still Open

- Complete the current Windows MSI installed-flow gate and normal staging pipeline.
- Expose repeatable local qualification entrypoints using the owning package and
  release scripts, with declared pinned toolchains and explicit prerequisites.
- Keep first-install Microsoft interaction separate from durable Guest Tools
  repair and subsequent native tests; never require credentials in chat.
- Decide which additional hosted Windows image/tool contracts should be mirrored
  locally, keeping architecture and installed GUI evidence explicit.
- Retain distinct managed-volume tests for git-aware seed and worktree sync.

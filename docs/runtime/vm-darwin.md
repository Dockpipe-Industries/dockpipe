# Darwin VM runtime

Dockpipe 0.6.28 adds the generic `qemu-darwin` VM backend. Automatic selection
uses this backend on macOS. It emulates an x86_64 guest with TCG and CPU `max`
on Intel and Apple Silicon hosts. HVF and host PCI passthrough are rejected.
This backend does not select a guest operating system or supply guest media.

The backend uses the shared VM disk, cohort identity, firmware variables, TPM,
bootstrap media and SSH implementation. It owns QEMU in the foreground, reports
its observed PID through the existing session reporter and waits for that child.
It does not daemonize the VM or add a separate lifecycle driver. Interactive
terminal launches default to Cocoa; an explicit display choice is preserved.

The optional VM toolchain requires Bash 4.3 or newer. Set
`DOCKPIPE_HOST_BASH_BIN` to the absolute executable path of an installed Bash.
Host `.sh` scripts and sourced pre-scripts admit this explicit selection before
using the existing interpreter fallback. An invalid explicit path fails; it does
not silently select another interpreter. Native host-shell defaults are unchanged.
The VM script checks its actual Bash version before sourcing the shared SDK.

Supply native `qemu-system-x86_64` and `qemu-img`, firmware when required, and
`swtpm` when TPM is enabled. Configured VM session reporting also requires GNU
`timeout` or `gtimeout`; the Darwin preflight rejects its absence before launch.
These requirements do not apply to the desktop host or unrelated workflows.
Dockpipe does not install the tools or change the user's global PATH.

Darwin's default temporary directory can exceed the Unix socket path limit.
The runtime allocates its private socket directory under canonical `/private/tmp`
and checks the complete longest endpoint against the 103-byte pathname bound.
The package display supervisor independently admits its own private runtime
parent and endpoint bounds. There is no TCP fallback or shared global bus.

Qualification must distinguish firmware/control-plane tests from an installed
guest operating system. A successful firmware boot on macOS does not establish
Windows installation, guest drivers, live resolution switching or clipboard
qualification. The new backend requires a new source-bound core candidate and
consumer tests against that published artifact.

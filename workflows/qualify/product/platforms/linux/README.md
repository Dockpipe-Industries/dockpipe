# Linux installed-product lane

Invoke `driver.py --request /absolute/request.json --output /absolute/new-output`.
The shared [driver contract](../../DRIVER.md) supplies run, plan, registry and target
bindings. This adapter accepts the Ubuntu 24.04 amd64 VM target and reports other
unobserved requested cases as pending. It is local and opt-in; it adds no CI job.

Configuration requires six exact input references:

- `pins_file` and `pins_sha256`: the selected host CLI/GUI/package-store receipt.
- `media_file` and `media_sha256`: the independently verified Ubuntu installer ISO.
- `baseline_receipt` and `baseline_receipt_sha256`: selected published CLI/desktop
  DEBs with their size, digest and source identities.

The host candidate and the product installed inside the guest can differ. Keep
both identities explicit. A published baseline must never qualify unbuilt source.
The target artifact digest must identify one of the admitted baseline DEBs.
This adapter's target is exclusively the Ubuntu guest product. Provisioning in
the separately pinned host GUI is prerequisite evidence and never credits the
guest with `vm.install-linux`. That case is explicitly blocked; a VM-only request
does not admit prerequisites or launch a GUI/guest. A future host lane must bind
the actual executing host OS/profile and GUI artifact independently. The earlier
guided Pop!_OS 22.04 host / Ubuntu 24.04 guest observations remain historical
evidence of those distinct roles, not a fresh guest-platform VM-feature pass.

`desktop.py` supplies a private authenticated X11 server, separate session and
accessibility buses, normal Dockpipe GUI and isolated settings, workspace and
temporary directory. Terminal request files use that temporary directory. It
never selects or kills another GUI. `ui.py` scopes accessibility discovery and
actions to that exact process. Real VM launch uses its VM tab and workflow form;
the SessionManager invokes the installed owning VM package. There is no direct
QEMU execution fallback.

`seed.py` creates a fresh private guest-only account/key and Ubuntu autoinstall
configuration. The actual Ubuntu installer validates that provided configuration;
this is not an alternative provisioning engine. `guest.py` reads back installed
OS and a fresh fixture marker through that new identity, and admits transferred
published DEBs by hash before installing them. Private identities, host keys and
passwords are generated outputs, never authored inputs or receipt literals.

Reserve one guest slot with the operator coordinating other platform lanes.
Immediately before launch, require at least 6 GiB available host memory and 32 GiB
free disk for the 4 GiB, four-vCPU guest. A new destination must be absent. Keep
its disk, firmware, TPM policy and machine identity coherent across disk-only
restart; never substitute historical VM state.

The default `operation: admission` returns `manual_pending` and a blocked lane.
With `operation: guided-install`, the driver creates a NEW guest disk and private
normal GUI, serves only this invocation's private Ubuntu configuration on loopback,
and writes phase instructions to `GUIDED-ACTIONS.md`. A UI agent or human follows
those instructions through the actual VM action/form and embedded Ubuntu installer.
Optional `timeout_seconds` is bounded to 60–7200 seconds (default 3600).
The host receipt must select `xvfb` and include its executable hash in `pins` for
guided execution; no system Xvfb location is assumed. Private DBus sockets use a
short, owner-private directory exposed as `OwnedDesktop.socket_directory` so deep
artifact directories do not exceed Unix socket path bounds. A contained launcher
grants only that exact directory, and owned cleanup removes it.

For eligible guest-product cases, the driver independently admits the owning workflow's exact QEMU process, ISO,
disk and machine identity; reads back the fresh OS marker; installs hash-verified
published DEBs; and executes the installed CLI fixture. An input marker must be
typed through the embedded guest terminal and is read back independently with the
installed desktop process. Captured display frames remain reviewable evidence.
The driver then requires normal shutdown, a new boot of the same disk/UUID with
no installer media, retained product identity, and final exact-process absence.
These lifecycle observations verify fixture identity and cleanup; they do not
become VM-feature assertions under the guest target. It never runs QEMU directly
or imports earlier passed receipts. Only assertions
observed in this invocation pass; unsupported cases stay in the denominator.

If a failure leaves a live or unidentified guest, the private GUI and disk remain
for reconciliation, with an actionable guide. Coordinate normal shutdown and
explicit slot release before another platform boots. Admission tests and source
checks do not qualify this complete guided adapter: it needs its own fresh run.

A completed invocation returns zero even when its receipt lists pending cases.
The shared runner uses the receipt to determine coverage. An incomplete guided
lifecycle returns nonzero and cannot contribute passing coverage to the runner.

### Optional workflow-worker enrollment

In `guided-install` configuration, `guest_enrollment` takes `grant_file`,
`grant_sha256` and `host_state`, plus `worker_deb`, `worker_deb_sha256`,
`desktop_deb` and `desktop_deb_sha256`. Select the private receipt from
`dockpipe remote guest-grant`, and the private state directory of its paired
host. Use a grant for one guest, dedicated to this invocation, with `run`
equal to the qualification `run_id`. The broker receipt must record `guests`
as `1` because teardown releases the entire grant. Use a broker and grant CLI
that report this field. Request enough time for the installation and enrolled
desktop phase. The endpoint in the paired host
state must be reachable from the installed guest.
For an isolated HTTP broker on `127.0.0.1`, the worker's owned SSH transport
forwards the same port between guest and host loopback. It rejects a failed
forward and closes the port when the worker transport ends.

Select host and guest CLI builds that support delegated enrollment. The upgrade
requires `dockpipe` and `dockpipe-desktop` DEBs for amd64 at the same newer
version. After recording the published baseline install and CLI workflow, the
driver upgrades both packages in one transaction through the owned SSH transport.
It checks the installed launcher and CLI against the selected DEBs and verifies
their digests again after the disk-only restart. In this mode the embedded GUI
observation applies to the upgraded desktop.

Pin `guest_enrollment.py` with the other driver dependencies. The driver uses
the same SSH transport to generate the guest key and return a signed claim.
The pinned host CLI submits that claim and the driver starts the installed
foreground worker.
The guest enrollment lasts until the first shutdown phase or expiry. If the
grant expires or the worker transport exits while waiting for the embedded GUI
check, the driver reports the failure immediately. The driver releases the
invocation's entire grant before the first shutdown and records the broker
receipt and SSH process exit. Its error path requests the same release if
enrollment may have reached the broker. A failed release leaves the exact
grant and guest for reconciliation.

The enrollment and release receipts record lifecycle observations. To qualify
remote workflow execution, submit and verify a workflow for the node named in
`GUIDED-ACTIONS.md` before entering the embedded-input marker.
[Host-authorized enrollment commands](../../../../../docs/runtime/remote-nodes.md#enroll-disposable-workflow-workers-through-their-host)

## Installed Browser on Pop!_OS

Use `browser.py` for `browser.local/happy/installed_e2e` on the local
`linux-amd64-deb-pop22.04` target. Select the desktop DEB as `artifact`, the
matching CLI DEB as `cli_artifact` with `cli_artifact_sha256`, and the private
X11 server as `xvfb` with `xvfb_sha256`. Pin `contracts.py`,
`debian_installation.py`, `desktop.py`, and `ui.py` in the plan.
Use the system Python with PyGObject, PyAT-SPI,
and Pillow.

The driver verifies the installed launcher, UI library, Browser SDK, and CLI
against the selected package members. It uses a new authenticated X11 session
and only synthetic loopback pages. It switches between two actual tabs,
independently reads the download bytes, tests temporary storage across reload
and normal Quit and restart, then verifies fixture shutdown.

## Installed Terminal on Pop!_OS

Use `terminal.py` for `terminal.local/happy/installed_e2e` with the same
Debian package, X11, and helper selections. The system Python also needs GTK 3
and GDK 3 introspection for the isolated clipboard read-back.

The driver selects Bash and types a command through the terminal document.
It copies the synthetic first line of rendered output using the private
display's clipboard. Kernel read-back binds Bash to the installed launcher and
the sleeping fixture child to Bash. Terminal Close must remove both original
process identities before the child's deadline, followed by normal launcher
Quit and private display cleanup.

## Installed desktop startup and host workflow on Pop!_OS

Use `workflow_launch.py` for the `install.desktop` and `workflow.host` happy
cases at `installed_e2e`. Use the same pinned Debian packages, X11 server and
helper dependencies as the Browser driver.

The driver opens a generated project through the folder picker and starts its
one-step host workflow through the normal Workflows dialog. The public execution
observer verifies the original step's exit and ties the live owner to the
installed CLI. The fixture checks the selected working directory and the exact
value written from a declared workflow variable. After normal Quit, a fresh
launcher must restore the selected workspace from the same private profile.

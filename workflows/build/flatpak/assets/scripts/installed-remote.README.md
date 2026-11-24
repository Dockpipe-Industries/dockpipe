# Installed Flatpak remote GUI qualification

`installed-remote.py` prepares a normal, immutable installed Flatpak GUI with a
private provider catalog and profile. It is an opt-in local qualification helper;
it does not qualify a release or start remote effects by default.

The configuration pins the installed bundle/full commit/GUI/UI/CLI, qualified
Flatpak resolver inventory and producer receipt, catalog compiler, fixture facade,
TLS fixture, Xvfb and bwrap. The actual installed `/app/bin/dockpipe` exports the
platform catalog and installs the resolver through an isolated TLS loopback
catalog. A native host catalog must not be relabeled as Flatpak-compatible.

The read-only launcher preflight covers the canonical account home with new
private backing in an outer mount namespace. Only explicitly selected paired
state/security directories are exposed read-only at their original canonical
paths. Host root and the selected installation remain read-only. Namespace-only
`/tmp` scratch and exact task output are writable. The genuine installed CLI must
confirm authority availability, immutable artifact identities and the unchanged
real default profile before a live GUI route is considered.

Offline preparation uses a fresh authenticated Xvfb display, session/accessibility
buses and profile. Agent-guided actions bind the current screenshot digest,
sequence, exact GUI process identity and visible owned window. All clipboard
settings must be explicitly OFF in the normal GUI's saved preferences. Preparation
and a normal close are supplementary evidence; they do not pass a remote viewer
case. The read-only native accessibility observer runs in a supervised disposable
child because a native library failure can terminate that process outside Python
exception handling. Only the exact task-owned runtime may be exposed to the app.

The current live route deliberately refuses execution. Before enabling it, a
reviewed concrete plan must bind a new provider-owned fixture, exact original
GUI-created Core reference, provider/module identities, a proven viewport/focus
or generated visual marker, and independent exact closure observations. Never
substitute the SDK probe or top-level window geometry for normal GUI evidence.
Missing widget geometry blocks control. A view-only connection may provide useful
supplementary evidence, but cannot pass required generated-input assertions.

No remote pixels are persisted by a live observer. Any future chrome observation
must capture only an independently established chrome rectangle at the X server,
rather than capture a full remote image and redact afterward. All PID absence
checks must execute in the same host process namespace as the launched app;
sandbox `/proc` absence does not establish host cleanup.

Run inert consumer tests with:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s workflows/build/flatpak/tests -p test_installed_remote.py
```

`paired-prepare` is a separate read-only discovery phase. It retains the proven
outer private-home isolation and selected paired authority mounts, while permitting
ordinary public catalog/status requests. The only local actions are the observed
Remote Desktop sidebar item, its Settings button, Escape from that dialog and
the main-window close control. Connect, arbitrary keyboard navigation and viewer
input are rejected. It provides no capture or control credit. The exact owned
Xvfb filesystem socket is mounted read-only through an admitted socket descriptor;
no host display directory or bus is exposed.

`installed-remote-expiry-run.py` adds an explicitly reviewed view-only fault
operation. Its private plan binds the installed bundle/deployment/runtime and
browser dependency, current product registry, provider archive/operator CLI,
source/build receipts, never-launched nonce fixture and actual normal-GUI View
selection template. The template contains only local pre-connection chrome; the
current selected-node, View and provider-ready text must match before Connect.
A read-only provider inspection also requires exactly the approved paired chooser
identity. The default invocation validates the plan without starting services.

The live operation verifies the actual installed desktop-control CLI and the
resolver library path advertised by its catalog, binds the new GUI journal
reference and observes the whole fresh 128-bit marker in memory. It pauses only
the admitted GUI through a PID file descriptor, then requires original-reference
Core closure and native idle while that GUI remains stopped. Watchdog, resume,
Core-close or cutoff assistance prevents an unassisted expiry result. Keyboard
input is categorically refused. This supplementary fault proof does not satisfy
the installed viewer's generated-input happy assertion or claim product-case
coverage.

The independent local cutoff is anchored to the native fixture's fresh age.
TERM at age 18 seconds, 2 seconds of TERM grace, 1 second for KILL/reap,
5 seconds for bounded in-flight relay delivery, a 15-second native authenticated
read timeout, an 8.5-second native teardown allowance and a 3-second clock/transit
margin total 52.5 seconds, before the fixture's 55-second hard close. Actual
unassisted closure must arrive before the local cutoff. Startup that cannot
leave 12 seconds for that observation fails without extending the deadline. The exact
channel authority/transport/stop source and Qt renewal code must match their
selected native and Flatpak build inventories. Physical host-power and unrelated
network-fault cases need separate qualification.

Each cleanup resource is attempted independently. Readable failure/unknown
receipts and the original journal stay available for reconciliation. Source,
artifact and profile bytes remain pinned throughout the operation, and independent
host/native readback remains required after execution.

# Native Worker Package ABI v1

The native worker contract is independent of the Qt Display and Workspace SDKs.
It runs an explicitly selected package module inside Dockpipe's dedicated macOS
app host mode, with the existing `com.dockpipe.launcher` app identity. It does not
carry media, input events, credentials or a second remote protocol.

This source slice is not a published SDK release. Existing published Display SDK
1.0 artifacts are unchanged. Packages must consume a separately versioned header
artifact; do not infer a sibling checkout or add it to old SDK manifests.

The C-compatible contract is
[`DockpipeNativeWorkerPackage.h`](../../src/dockpipe-app/src/host-sdk/contracts/DockpipeNativeWorkerPackage.h).
Export `dockpipe_native_worker_package_v1` with C linkage and default visibility.
The synchronous entrypoint runs once on the main thread. It receives trusted CLI
and state references, an opaque owner-selected JSON profile (at most16KiB), and a
thread-safe nonblocking stop callback. Borrowed strings remain valid only until
return. No allocator or C++ object crosses the boundary.

The package owns its native run loop, explicit local setup/status operations and
all backend algorithms. It must poll the stop callback during active work and
native waits, preserve cancellation and uncertainty, and join owned native work
and children before setting `cleanup_verified=1`. A stop request never grants or
renews authority. It must not install process-wide signal handlers.

The host initializes result to Failure/unknown. Success and Cancelled are distinct
outcomes; cleanup is independent. Missing/invalid/unknown cleanup never becomes
success. The host retains the module for process life and never calls `dlclose`.
Unknown cleanup ends this dedicated process without unloading code referenced by
late callbacks. Providers must retain any state referenced by those callbacks.
After a local stop, the host allows at most15s before an unknown forced exit.
The provider and core's existing session leases remain independently enforced.

## Explicit host invocation

```text
DockPipe.app/Contents/MacOS/DockPipe --native-worker-host --request /absolute/private/host.json
```

Dispatch occurs before QApplication, GUI single-instance handling and ordinary
GUI arguments. Other arguments cannot be mixed into this dedicated mode. Other
platforms fail explicitly; normal GUI startup is unchanged. No app, service or
remote session starts automatically merely because a package is installed.

On macOS, the executable path alone does not determine the privacy permission
subject. Direct spawning can inherit the calling application's responsibility,
even when the accessing process is the correctly signed Dockpipe host. Launch
the canonical app through Launch Services, or qualify a GUI login agent's
application association, and check effective native access in that exact launch
context. For an explicit background invocation through Launch Services:

```text
open -n -g -W -a /absolute/DockPipe.app --args --native-worker-host --request /absolute/private/host.json
```

The `open` process is a waiter, not the native worker. Its exit status does not
replace the package's exact completion and cleanup evidence. A successful module
load or an enabled Settings row does not establish effective native access.

The descriptor is an owned0600 canonical regular file, at most32KiB. It contains
exactly:

```json
{
  "schema": "dockpipe.native-worker-host/v1",
  "module": {"path": "/absolute/package/module.so", "sha256": "<64 lowercase hex>"},
  "cli": {"path": "/absolute/DockPipe.app/Contents/Helpers/dockpipe", "sha256": "<64 lowercase hex>"},
  "state_directory": "/absolute/private/paired-state",
  "profile": {},
  "trust": {"mode": "production", "team_identifier": "<10-character Team ID>"}
}
```

All paths must already be canonical. State must be owned0700. Images are regular,
root/current-user owned, not group/world writable, hash-pinned and at most128MiB;
the CLI must be executable and must be the host app's bundled CLI. The package
profile is opaque: only the package interprets its schema or local actions.

Production requires a valid Developer ID Application chain, the same selected
Team ID for app/module/CLI, and Hardened Runtime on the actual host. The app's
identifier remains `com.dockpipe.launcher`. Code signing requirements are checked,
never rewritten. Library-validation disabling is rejected; there is no fallback
to unsigned/foreign-team code or automatic re-signing on the client.

An explicit private review descriptor may replace trust with
`{"mode":"review-adhoc","host_sha256":"<64 lowercase hex>"}`. Host, module and
CLI hashes are pinned and signatures must be valid ad-hoc signatures. This mode
is not production signing and does not promise reliable Local Network consent
across host updates. No environment variable enables it implicitly.

The host loads a byte-identical private snapshot after hash/signature validation,
avoiding a package-update race at `dlopen`. This first slice requires a
self-contained native module using OS frameworks; it does not materialize a
caller-selected dependency search path. A provider must bind its artifact to
`dladdr` of its entrypoint/module, not to the host executable hash. Helper and
manifest references should come from its frozen absolute package profile.

## Scope and qualification

This first slice hosts only a package worker. Existing feature-worker services,
pairing, owner grants and desktop references are not migrated. The normal GUI
permission widget remains the local consent UI; native macOS user approval is
still required. Do not claim one-app permission consolidation from a component
build alone: qualify the actual canonical app, LaunchServices/login invocation,
permission status, provider update, revocation and exact cleanup separately.

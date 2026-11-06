# Native DSM gateway

This deployment adapter validates the DSM browser session and owns the shared
desktop's operator binding. It delegates worker/client transport, PTYs, machine
snapshots and typed Docker operations to the generic `nativehost` infrastructure.
DSM APIs and package paths stay here and in `workflows/internal/synology`.

The package user runs the gateway with no effective capabilities and
`NoNewPrivileges=true`. First explicit activation binds the installation to the
verified local administrator. A fixed, disabled one-shot DSM task starts the
native CLI worker as that account; the browser removes the starter after it
connects. Worker admission checks kernel Unix peer credentials and a single-use
nonce. Task `owner` is the execution account; DSM's `real_owner` is pinned routing
metadata, not execution authority.

The worker socket is in `etc/native-host`, outside the GUI's mounted filesystems.
Do not move it into the GUI-writable `var` directory: a replacement registration
listener could retain a worker independently of the gateway's authorization.
Only the bounded client socket/key/reference are in `var/native-client`.

The gateway's TLS key and persistent owner binding also stay outside the GUI
mounts. The browser receives neither the client key nor the internal streaming
credential. Nginx pins the package gateway's private CA. The gateway pins the
streamer's exact self-signed certificate, strips DSM credentials before forwarding
and removes upstream cookie/authentication challenges.

Post-install initialization migrates an existing package-owned public streamer
certificate from mode 0644 to 0600 before gateway startup. It changes the opened
inode's permissions without regenerating the certificate or changing its bytes;
symlinks, nonregular files, foreign ownership and writable modes are rejected.

A verified browser heartbeat renews a 30-second lease bound to a hash of its DSM
`id` cookie and a separate opaque HttpOnly cookie. API transitions and report
reads share a transaction mutex; the lease mutex is never held across generic
broker callbacks. Disable clears the generation so stale heartbeats cannot undo
it. Expiry disconnects workers, clients and upgraded streams. A single installation
has one operator and a shared desktop, not independent per-viewer GUI sessions.
Successful prepare and heartbeat reports include `lease_expires_at`, the actual
stored deadline, so the launcher can close its stream when renewal stops.

Resource limits are explicit: 16 browser leases, 64 HTTP connections including
hijacked streams, 1 KiB API bodies, 64 KiB API replies, fixed local targets and
bounded authentication/read/write deadlines. The generic transport owns its own
frame, worker-operation and PTY limits. Native shells retain normal user terminal
semantics; deliberately detached or privilege-changed jobs are not confined.

Local tests cover real broker/worker/client identity, terminal cleanup, TLS
forgery, ownership races, actual HTTP Upgrade relay and slot/lease cleanup.
Actual DSM installation, Task Scheduler detachment and GUI interaction remain
hardware acceptance steps for each candidate. Qualification apps are not runtime
dependencies of the real package.

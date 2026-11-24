# TASK-056 — Browser contexts on paired worker networks

Status: first Linux X11 to Darwin ARM64 private preview installed and live-qualified
2026-11-11, authorized 2026-11-10. Broader platform/onboarding work remains open.

The browser renders on the operator workstation. Its machine selector chooses
local networking or an authenticated proxy whose TCP destinations and DNS belong
to the paired worker. Worker localhost and LAN services are intentional scope.
Independent dashboard browsers can select different workers. A failed remote
route never falls back to local networking.

The generic engine owns an independently granted, bounded TCP relay. The GUI
owns browser process isolation, proxy configuration, context selection and helper
lifetime. Spatial-Web owns persistent browser tabs, history, cookies and profile
locking through its installed SDK; the GUI consumes an explicit SDK candidate.
No sibling browser implementation is copied into Dockpipe.

Qt's application proxy requires a separate local browser helper for each remote
context. The initial embedding slice targets Linux X11. Other operator platforms
must report unsupported until their embedding is implemented and qualified.
Persistent remote profiles are separated by widget context and paired identity;
changing the global workspace machine does not change a browser's route.
Each panel runs at most one remote helper and relay. Changing machines saves and
stops the previous helper; selecting it again restores its private profile. Local
browsing keeps the existing temporary widget and live local tab tear-out behavior.
Embedding verifies the child PID and actual X11 parent ownership before adoption.
The helper exports a stable QWindow and reparents lazy WebEngine QWidget surface
replacements beneath it. The parent establishes a native container before showing
it and owns its coordinates. The hidden helper bypasses window-manager framing;
only the validated parent acknowledgement makes it visible. Settled native tests
cover movement, resize, page/window hide-show and a JavaScript-rendered pixel,
as well as absence from the window manager's top-level client list. It applies
the shell's theme. Container admission waits for Qt's own first-show reparenting;
manually reparenting its foreign window before that event detached it from later
resize updates. The production-context regression starts hidden, then checks
rendered content across repeated resize, maximize and restore operations.

The installed browser SDK preserves an unfinished address draft per tab across
blur and asynchronous page updates. Enter navigates, Escape restores the current
URL, and Ctrl+L selects the address. Developer tools are available from the browser
More menu, F12 or Ctrl+Shift+I. Each inspector belongs to its tab and uses that
tab's profile and trusted page configuration; no remote debugging port is exposed.

The Biztraak development-site investigation exposed concurrent connection failures
before HTTP admission: local proof IPC dropped burst connections. A separate
deterministic fixture also found four-slot verification overload returning busy.
A broker-only candidate now reserves the existing eight IPC workers before accept
and gives validated proofs bounded waiting while retaining four active native
verifications. Race checks passed for concurrent browser contexts and renewal
pulses, plus cancellation, expiry, lock, generation and replay rejection. Both
affected Go packages passed. The approved broker-only candidate was installed;
16 simultaneous real OS-authorized catalog reads passed without retries. The
paired-worker/Biztraak check remains unqualified: the Cloudflare edge journal had
already recorded failed health before replacement, outside its previous recovery
authority. Prior connector-process liveness was not established. Broker replacement
stopped its owned connector, and ordinary startup correctly refused to replay the
failed recovery. Separately approved explicit connector recovery passed its image,
owned-readiness and signed public HTTPS checks. Its failed journal was preserved
byte-for-byte and both worker catalogs returned. Two independent paired proxies
then passed eight concurrent verified HTTPS document/script reads. The actual
browser loaded its scripts without tunnel/certificate errors and clicked Log in
once. Login remains unqualified: OIDC discovery on the worker's port 5001 closes
during TLS, including a direct native probe on that worker without Dockpipe's
proxy. Its listener belongs to Lima's SSH forwarding path; the ultimate API
destination was not established. No application, container, forward, Lima or
certificate setting was changed, and certificate verification stayed enabled.

Cleanup validation also reproduced a Dockpipe authority bug: caller-canceled OS
queries could replace global security evidence and invalidate unrelated access.
A narrowly qualified follow-up discards only caller-aborted context-error samples
without refreshing evidence or leases, while preserving native-owned timeouts,
genuine errors, affirmative lock/resume evidence, ordering and freshness limits.
Focused/race checks and independent review passed. The separately approved build
is installed. Live checks passed 16 simultaneous catalog reads and 12 withdrawn
clients without reauthentication, global lock or absolute-expiry renewal. Two
independent paired proxies passed eight concurrent verified HTTPS reads. The
existing desktop job was resumed without forcing a restart or changing its
files/permissions; its fresh catalog advertises view/input readiness. The
diagnostic terminal closed cleanly and no uncertain host receipts remained.

The generic relay's focused package and race suites passed, including authenticated
HTTP, CONNECT, WebSocket upgrade, remote destination opening, cancellation,
capacity, grants and stale/revoked identity handling. Linux and Darwin ARM64 CLI
candidates compile. SDK profile/session and installed-consumer checks passed.
The native X11 helper test now verifies a `localhost` request reaches the selected
authenticated proxy, responds to its HTTP 407 challenge, and exits on parent EOF.
It also routes an unresolvable test hostname through that proxy and verifies
parent-owned child-window embedding and relay/helper cleanup. The helper blocks
native extension installation so its separate Qt downloader cannot use system
networking outside the selected route. Default local installation is unchanged.
Dashboard store checks passed (25 cases); focused page checks preserve distinct
routes and context identities and retain the existing local tab tear-out behavior.
The reviewed broker and paired worker CLI are now installed with the independent
TCP grant enabled. Real paired checks passed HTTP, certificate-verified HTTPS
CONNECT, browser JavaScript and WebSocket echo. Distinct worker/workstation
loopback origins proved the remote browser never contacted the local duplicate.
Proxy credentials did not reach the origin. A restarted browser helper restored
its tab and cookie, while a simultaneous second profile remained isolated.
The real embedded parent closed its helper and relay after grant revocation,
without constructing a local browser. Explicit re-enable produced a fresh
generation and successful new proxy. Test origins and launch registrations were
closed and removed; worker feature and network readiness remained available.

The background macOS activation stopped the old worker but its replacement
`launchctl bootstrap` failed with exit 5. Direct execution of the same approved
service command from the user's Terminal succeeded. This is a service activation
recovery gap, tracked with TASK-054, not a qualified unattended update path.
No re-pairing, reboot or privacy-permission change was required.

The later TLS-sharing update used an independent, preverified recovery executor
until authenticated worker health was confirmed. A known failed bootstrap with
no registered job succeeded on its bounded second activation attempt. The guard
then committed and its temporary launch registrations were removed. The private
fallback restores original grant/service preimages and executes the preserved
worker under the already-loaded guard, avoiding bootstrap as the only recovery
path. This private handoff is not a generally qualified product self-updater.

## Additional remote TLS trust

Owner opt-in `share_tls_trust` now publishes bounded, authenticated public SSL
anchors bound to the exact paired identity, generation and material digest.
The worker refreshes its source every five seconds; changes retire old streams
and profile lifetimes. Ordinary catalogs carry compact summaries. Proxy readiness
supplies a verified snapshot before the helper creates its isolated browser
profile through the installed SDK's additional-trusted-certificates option.

Native macOS collection shares explicit user/admin additional SSL anchors only,
with positive native SSL evaluation. Labels and private keys are not exported.
Unrepresentable denial, hostname/application restrictions and allowed-error rules
refuse sharing. Normal browser public roots remain; this does not reproduce every
macOS trust policy. No system trust-store writes or certificate-error overrides
are used. Native parser fixtures and the actual paired Mac export passed.

The paired HTTPS development-site check now passes using only the automatically
delivered snapshot. A concurrent default-trust profile rejects the same site,
proving scope separation. The production helper accepted the identity/digest-bound
snapshot and grant loss closed all owned helper/relay processes without local
fallback. Sharing was restored afterwards. One cleanup attempt was interrupted
by local OS authority loss; its native receipt absence and actual grant state were
inspected before restoration, and the matching-CLI rerun passed.

Remaining acceptance:

- Qualify the end-user picker flow across multiple distinct paired workers;
  current live proof uses one paired worker and the local workstation.
- Add supported worker-grant onboarding beyond the current explicit CLI grant
  and reviewed private activation. Ordinary service startup still retains its
  existing feature/delivery/profile authority prerequisites; a network-only owner
  can use the independent stdio worker.
- Keep broader operator-platform support and sustained resource/performance
  qualification separate from the first Linux-to-Darwin slice.

The physical Darwin ARM64 background worker reported a previous Network stop
without its cause. The installed X11 Flatpak later served and reconnected to
the same paired target using bounded foreground workers under the existing
Network grant. The source now preserves grant and OS session stop causes
through cancellation and logs a sanitized reason. Focused race tests and the
full remote and remote CLI suites passed. The existing background service has
yet to be qualified using this build.

## Current native interaction and login follow-up

The installed Linux-to-m6 browser renders the HTTPS development site and runs
its scripts. A fresh isolated login check reaches the OIDC discovery request on
m6 localhost:5001, which closes with ERR_CONNECTION_CLOSED. The same
certificate-verifying TLS handshake fails directly on m6 with SSLEOFError,
before certificate evaluation. The upstream cause remains unestablished; no
application, forwarding, container or certificate-policy change is included in
this diagnosis. Page-load and concurrent static-read proof do not qualify login.

The Linux X11 host now bridges native and Qt widget activation for its owned
embedded browser. The parent container accepts keyboard focus; the child repairs
logical widget activation after Qt's focus-window notification. Explicit browser
clicks complete the native handoff to the mapped owned browser window; URL and
geometry updates do not request activation. The external SDK is unchanged.

The native isolated-display regression passes actual address typing, Enter,
Ctrl+L after navigation, parent-input retention during frequent URL updates and
resize, and browser reacquisition. Its ordinary parent activation is supplied by
the fixture because Xvfb has no window manager; helper focus is not forced by the
fixture. Proxy authentication, parent-EOF shutdown and owned-child cleanup also
pass. These Linux checks do not qualify other platform embedding backends.

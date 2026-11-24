# Authenticated remote TCP network relay

Dockpipe's network relay opens TCP destinations on an explicitly selected paired
worker. DNS resolution and `localhost` belong to that worker. The operator and
broker never dial the destination, and errors never fall back to local networking.
This is a generic networking capability, independent of terminal execution,
workflow delivery, desktop viewing and desktop input.

## Worker ownership

On an already paired worker, `dockpipe remote network-config --request <private-file>`
accepts a private JSON file:

```json
{"schema":"dockpipe.remote.network/v1","enabled":true}
```

This is an explicit grant to reach **all TCP destinations accessible to the worker
account**, including loopback services, private/LAN networks and public networks,
on ports 1–65535. It does not grant UDP, raw sockets, filesystem access or process
execution. Enable it only for an operator trusted with those reachable services.
There is no implicit grant inherited from pairing, features or desktop access.

To allow one service on the worker, set an exact loopback IP and port:

```json
{"schema":"dockpipe.remote.network/v1","enabled":true,"loopback_destination":"127.0.0.1:8080"}
```

Use that same IP and port in the Browser URL. Requests for other destinations
fail on the worker before it opens a socket, including redirects to a different
port or hostname. Use a worker version that recognizes this field.

Missing, invalid or disabled permissions fail closed. Setting `enabled` to false
ends the running worker's connections; restarting requires a fresh worker lifetime.

The normal `dockpipe remote worker` service supervises the independent local grant
every 500 ms. An observed disabled-to-enabled transition starts a fresh network
lifetime. Revocation ends its active sockets through the shared worker loop. After
a failed, locked or uncertain lifetime, the network remains unavailable until an
observed disable-to-enable transition, an explicit permission change or an ordinary
worker restart; unrelated worker capabilities continue. Confirmed public-trust
changes rotate the network lifetime as described below. No TCP open is retried
automatically.

The background worker logs a stop category for the grant, OS lock or sleep
state, worker lock, or broker failure. Broker HTTP failures include the status
code. The service log omits response bodies and raw transport errors.

A GUI without the normal service can own `dockpipe remote network-worker --stdio`.
Both entry points share the same implementation and singleton exclusion. One
worker lifetime per state root publishes an independent generation and services
bounded TCP streams. The standalone command prints one `{"type":"ready"}` record
after the first authenticated exchange; its private stdin EOF cancels that lifetime.
A changed permission, an unknown/locked local OS session, sleep/resume or failed
broker exchange cancels either entry point and its sockets. Its
private `network-worker.lock` excludes competing lifetimes. Existing worker identity
and broker identity signatures authenticate every catalog/directive exchange.

On Linux Cinnamon, the running Cinnamon ScreenSaver service can provide the
native lock state when logind cannot establish it alone. Dockpipe queries the
existing service and does not activate it. If Cinnamon stops its idle service
and logind supplies no lock capability, the worker stays unavailable.

On Windows x64 and ARM64, the Network worker needs its own active local console
session. The worker obtains lock state from Windows session information and
checks elapsed and working-state clocks for suspension. Locked, disconnected,
remote-only and session-zero processes cannot establish an unlocked console
session. This worker support does not enable Windows broker authentication.

## Operator ownership

`dockpipe remote network-catalog --auth never` returns live catalogs:

```json
[{"schema":"dockpipe.remote.network/v1","target":{"node":"worker","fingerprint":"<64 hex>","generation":"<64 hex>"},"enabled":true,"received_at":"<timestamp>"}]
```

Capture the exact catalog target after selecting a paired machine. A fresh worker
generation on that same selected node and fingerprint may create a new proxy lifetime
after checking its current enabled grant. Never graft a new generation into old
streams or replay a pending TCP open; fingerprint changes require explicit selection.
A private proxy intent selects exactly that target:

```json
{"schema":"dockpipe.remote.network/v1","target":{"node":"worker","fingerprint":"<64 hex>","generation":"<64 hex>"}}
```

`dockpipe remote network-proxy --stdio --request <private-file> --auth never`
checks current OS authority and exact catalog readiness, binds a new IPv4 loopback
port, then prints exactly one private stdout record:

```json
{"type":"ready","endpoint":"http://127.0.0.1:<port>","username":"dockpipe","password":"<64 random hex>","target":{"node":"worker","fingerprint":"<64 hex>","generation":"<64 hex>"}}
```

Keep stdin open for the proxy lifetime. Any input byte or EOF cancels it. Never
log this readiness record or persist its credential. Every HTTP request and CONNECT
requires that instance's random Basic proxy credential, including subsequent
requests on reused sockets. Credentials are stripped before origin forwarding.
The loopback address is not independently an authentication boundary against other
processes running as the same OS user.

The proxy supports ordinary HTTP, HTTP WebSocket upgrades, and opaque CONNECT
streams. HTTPS certificates and TLS remain the endpoint application's responsibility;
Dockpipe does not intercept TLS or install a certificate authority. An embedding
browser must apply the proxy before engine initialization, disable direct loopback
bypass, non-proxied WebRTC/UDP/QUIC and speculative local DNS, supply the credential
only to its exact proxy challenge, and close the browsing process when its relay
dies. Browser integration is outside the generic engine.

## Broker authority and resource contract

Only OS-authenticated `remote.network.session` operations can discover, connect or
renew streams. All contexts share the existing eight-worker local security IPC
service. It reserves a worker before accepting a socket, leaving bursts in the
OS-bounded Unix listen backlog instead of resetting accepted proof requests. Peer
UID checks, request limits, cancellation and existing I/O deadlines remain enforced.

HTTP proof verification retains four concurrent native verification queries across
all clients. After signature validation consumes the one-use nonce, up to 128
additional valid admissions may wait without holding the authority mutex. Waiting
is bounded by five seconds, the request context and the original proof deadline;
after waiting, authority is checked before querying the OS and again afterward.
Proof issuance separately retains at most 128 outstanding nonces: this is an
independent phase limit, not a combined 128-request cap. Saturation, cancellation,
expiry, lock or generation change fails clearly and cannot replay the consumed
proof. No network request retries or signs another proof implicitly.

Renewal binds the original OS admission session and generation.
HTTP traffic and heartbeat requests never count as OS activity. Lock, inactivity,
absolute expiry, node revocation, worker grant loss, worker generation change and
stale catalog cancel current streams. A proxy instance fails on any renewal or
catalog error; reconnect requires a fresh exact intent.

The broker issues one-use role-specific attachment tickets via signed JSON
responses. WSS attachment also requires the matching operator/worker bearer.
Tickets cannot select a different destination or machine. Failed attachment consumes
the ticket; a lost response never silently replays a TCP open. The relay is opaque
binary WebSocket transport with compression disabled, not a terminal command or
HTTP JSON payload tunnel. WSS protects each broker hop; the trusted broker can see
plain TCP traffic unless the endpoint protocol uses its own end-to-end encryption.

Capacity is 256 live streams per broker, 32 per node, and 32 handlers per local
proxy. Accepted local sockets (including unauthenticated header readers and idle
keepalive connections) are bounded to 64 per proxy. Each broker direction retains
at most one 64 KiB frame and enforces a 1 GiB byte budget and 1,048,576-frame
processing budget (enough for the byte budget at average 1 KiB payloads). Streams have a one-hour
absolute lifetime, 120-second per-direction idle timeout, 15-second pairing window,
10-second worker dial deadline and five-second broker write deadline. Polling
requires a worker catalog newer than five seconds. Broker watchdog checks occur
every 250 ms; worker permission/session checks occur every 250 ms with bounded OS
sampling. Streams close when either endpoint ends; TCP half-close is not preserved.
The local proxy renews OS admission every second, with no automatic retry or direct
fallback after an uncertain operation.

All intents and permission files are private regular files. Proxy intents are at
most 4 KiB. The implementation creates no persisted content, DNS, destination or
session journal; only the explicit worker grant and existing process lock persist.

## Additional public TLS anchors

A paired worker owner may opt in independently to sharing approved **additional
SSL trust anchors**:

```json
{"schema":"dockpipe.remote.network/v1","enabled":true,"share_tls_trust":true}
```

The default is false. Existing workers and catalogs without this option retain
normal browser trust behavior. Enabling this option does not write a trust store,
export a private key, disable certificate verification, or grant an exception for
a hostname mismatch. The embedding client imports the public DER certificates
only into the selected network context. The ordinary browser public-root baseline
remains enabled: this additive mechanism does **not** reproduce all native trust
policy or subtract native distrust from that baseline.

On any supported Network worker, the owner can select explicit public CA roots
by writing `network-trust.json` in the worker's private state directory beside
`network.json`. The file takes precedence over native export. The owner must
keep it private and replace it atomically when rotating the selection.

```json
{"schema":"dockpipe.remote.network-trust-material/v1","certificates":["<base64 public DER>"]}
```

Supply an explicit array of valid self-signed CA certificates in DER form. The
usual count and byte bounds apply. Private keys and native trust restrictions
are outside this format. Add only roots approved for SSL use on the selected
worker. An empty array explicitly adds no roots. The worker does not read the
file unless `share_tls_trust` is enabled. An invalid or inaccessible file makes
sharing unavailable. Changing the accepted snapshot retires the worker's
current network generation and its connections.

On macOS the signed, fixed installed `dockpipe-system-auth --tls-trust` helper
exports only explicit user/admin additional SSL anchors that pass native SSL
assessment. Unsupported applicable restrictions or distrust yield an unavailable
result, never a partially accepted set or flattened hostname/application policy.
Default system roots and their trust-asset policies are not projected. Other
platforms without an equivalent provider need the explicit file above when
sharing is requested. Native verification and certificate collection are
read-only.

Discovery contains only an optional compact summary:

```json
{"tls_trust":{"state":"ready","digest":"<64 hex>","count":1,"bytes":850}}
```

States are `ready`, `disabled` and `unavailable`; only ready carries a digest.
An absent summary is backward-compatible disabled sharing. Unavailable sharing
remains visible in discovery but blocks new connections and proxy lease renewal.
Snapshots are immutable within one exact worker generation. Neither the public
certificate array nor a private key is included in the ordinary catalog.

For a private request containing the captured target and digest,
`dockpipe remote network-trust --request <private-file> --auth never` returns the
fresh authenticated snapshot:

```json
{"schema":"dockpipe.remote.network-trust/v1","target":{"node":"worker","fingerprint":"<64 hex>","generation":"<64 hex>"},"digest":"<64 hex>","certificates":["<base64 public DER>"]}
```

The request has the same schema, target and digest fields without `certificates`.
The existing signed machine exchange publishes snapshots; operator retrieval uses
OS-authenticated authority and checks current enrollment, generation, freshness
and digest. `network-proxy` retrieves and validates this same snapshot before
emitting its private ready descriptor, as optional `tls_trust` metadata. A failed
retrieval, generation drift or unavailable sharing closes the reserved listener
without emitting readiness. Legacy and disabled sharing omit this metadata.

Each snapshot permits at most 512 unique X.509 DER certificates, 16 KiB per
certificate, 2 MiB aggregate DER and 3 MiB JSON. The broker retains at most 16 MiB
aggregate DER across nodes, pruning stale records before admitting replacements.
Certificate material is never persisted by the relay. The ordinary security-helper
stdout limit remains 64 KiB; only the dedicated public-trust command permits
3 MiB. The helper and trust collection carry five-second deadlines.

Certificates are ordered by ascending SHA256 DER fingerprint. The material digest
is SHA256 over `dockpipe.remote.network-trust/v1` followed by a newline, then each
certificate's four-byte unsigned big-endian DER length followed by its DER bytes.
The signed snapshot binds this material to the captured target. The worker
network generation also binds the independent permission and current trust digest.

The worker checks for trust changes every five seconds, with a bounded five-second
collection operation. Confirmed addition, removal or a change to/from unavailable
cancels old sockets and creates a fresh worker generation under the still-enabled
owner grant. Existing proxy leases fail for that old generation; an embedding
client must retire the old process/profile before using a fresh context. Explicit
permission changes also require a fresh lifetime. Transport failures still do not
replay TCP opens or silently resume an uncertain lifetime.

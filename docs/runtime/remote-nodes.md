# Remote nodes

`dockpipe remote` delivers one explicitly assigned workflow to a user-owned node. The node makes
outbound HTTPS requests and invokes the existing local CLI. No SSH server, inbound worker port,
inbound shell endpoint or automatic Git operation is involved. Source delivery is an explicit,
bounded snapshot; it does not synchronize an entire checkout.

The Dockpipe app exposes remote setup and pairing in **Machines**, delivery in **Workflows**,
and results in **Executions → Runs & logs → Remote jobs**. The current single-operator implementation has Linux loopback
and local UI test evidence. Darwin builds and launchd manifests alone do not establish
complete native Mac pairing, sleep/wake or end-to-end provider acceptance.
Remote broker initialization, workers, and provider setup currently require Linux or macOS;
Windows remote workers remain a follow-up. Cross-platform protocol, storage, and process tests
do not establish Windows remote-node support.

## Ownership

Core owns the provider-neutral protocol, pairing, job state, cancellation, local invocation,
results, and process supervision. Resolvers own browser authentication, tunnels, DNS, and provider
configuration. Workflows still define what runs; runtimes still define local isolation. DorkPipe
retains graph scheduling, placement, fan-out, and distributed orchestration ownership.

The Cloudflare resolver source lives in the [external remote package](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/remote). Additional providers implement the same edge
contract without vendor branches in core. The `dockpipe.remote/v1` transport is
the current node-delivery contract; it does not provide a hosted service by itself.

## Install the resolver

From the app Marketplace, install `dockpipe.cloudflare.remote-edge` on the broker
host. Its title is **Cloudflare Tunnel remote edge**. The worker only needs the CLI.
The resolver was previously published as `cloudflare`; existing installs of that name
are not renamed automatically. New setup commands use the qualified identity below.

In **Machines → Set up remote access**, choose **Install provider** (or **Add provider**)
to open the Marketplace filtered to remote-access resolver packages. Select a package
and choose **Install**. The Dockpipe app uses the catalog's pinned version and checksum,
returns to setup, refreshes installed providers and selects the package you installed.
Choose its setup action to continue; installing or selecting a package alone does not
change your remote connection. Other providers appear through their `remote.edge`
or `remote.broker` package capability, without a Cloudflare-only app path.

An existing connection keeps its remote address visible, including when it was created
before provider metadata was saved. **Review remote setup** from the pairing page
opens the provider selector and pre-fills the existing hostname. Select the provider
that originally created the connection, then complete setup to verify its saved state
and record its provider details. The Dockpipe app does not infer a provider from a hostname
or silently replace an existing connection.

## Build the resolver (package authors)

Build from the external package repository with a pinned absolute `DOCKPIPE_BIN`.
Select `PACKAGE_REPOSITORY` as that checkout. Install the CLI at a stable path before installing services:

```bash
"$DOCKPIPE_BIN" package build source --workdir "$PACKAGE_REPOSITORY" --only remote
```

The package hook uses the source-build SDK, stages a resolver in package runtime state, and compiles
a self-contained resolver tarball with the host-native helper. Only the broker needs this resolver.
Use normal package distribution for that compiled artifact. Compiling the source-only resolver over
it removes the bundled binary. Build separate artifacts for other OS/architecture combinations.

Setup reuses Dockpipe's existing host-dependency preflight. On macOS it can offer the declared
`brew install cloudflared` installer. On Debian/Ubuntu/Pop!_OS it offers the package-declared sudo
installer for Cloudflare's official signed APT repository, scoped to the machine architecture.
The repository refresh targets only Cloudflare, so unrelated repository errors do not block it.
If curl or CA certificates are missing, their bootstrap first uses the existing system repositories.
Flatpak Marketplace artifacts bundle `cloudflared` and never run a native package installer.
Noninteractive setup does not silently approve dependency installation.
During setup, installers and browser-authentication tools own the terminal: no setup spinner or
dependency heartbeat overwrites their prompts. A password prompt may hide typed characters.
`DOCKPIPE_CLOUDFLARED_BIN` selects an explicit executable after preflight.

## Cloudflare setup

With a domain already managed by your Cloudflare account:

```bash
dockpipe remote setup --resolver dockpipe.cloudflare.remote-edge --hostname benchmarks.example.com
```

This command authorizes its documented setup effects: browser login, one named tunnel, its DNS
route, private local state, and a user service. Passwords and MFA stay in Cloudflare's browser flow.
Setup reports its login, tunnel, DNS, and readiness stages and prints the next pairing command.
The Dockpipe app's setup button starts this same flow in a terminal for interactive prompts
and browser authentication; there is no command to type. When it finishes, choose
**Check connection** in the app to reload the saved address and provider details.
Cloudflared opens the browser itself; the resolver preserves desktop-session variables including
XAUTHORITY and XDG desktop settings. If no window opens, follow cloudflared's printed login URL
and keep the terminal open. A valid existing account certificate skips browser login explicitly.
Existing account credentials are reused without copying them to nodes. Conflicting DNS records
are not overwritten. Account-management credentials never appear in the runtime command.

The broker listens on `127.0.0.1:47831` and supervises the edge process. Setup checks the authenticated
public endpoint before success. Failure preserves resources and recovery state; an ambiguous tunnel
creation is never automatically repeated. Cloudflare terminates HTTPS and forwards through its
encrypted tunnel to the loopback HTTP origin. It is part of the trusted transport boundary; this
is not opaque end-to-end encryption through Cloudflare. The broker authenticates operators and nodes
itself, without depending on an Access browser challenge for background worker requests.

Services use systemd user units on Linux and LaunchAgents on macOS. They retain the installation
user's PATH. They are user-session services, not a promise of operation before login or while the
machine sleeps. Configure power/session policy separately for an always-on benchmark worker.

After installing a new Dockpipe version, open **Machines**. The Dockpipe app reports when a
registered service differs from the current CLI installation, including when the binary
has changed at the same path. **Update broker service…** or **Update worker service…**
asks before replacing the service definition and restarting it. This can interrupt active
remote work and replaces custom service settings; pairing credentials and provider state
are preserved. Package installation alone does not migrate an existing user service.

The equivalent explicit CLI operation is `dockpipe remote service --role broker --replace`
(or `--role worker`). Run it from the stable installation you want the service to use.
Without `--replace`, a differing definition is refused. Updates publish a complete, private
service file atomically and retain a pending marker until activation succeeds. If activation
fails or is interrupted, retry the same explicit update after inspecting the error.
`remote info` exposes each local role's `service.state`: `missing`, `update-required`,
`current`, or `unavailable`. This checks registration and completion of the activation
command; it does not prove process health, worker presence, or successful workflow execution.

## Pair and run

### Register pairing hardware keys

New pairings require a physical FIDO2 security key with PIN/user verification and
trusted manufacturer attestation. A synced passkey, software authenticator or
code-only approval cannot satisfy this requirement. This change gates pairing;
it adds no per-action prompts or remote approval service.

On the broker computer, stop the broker service using the operating system's
service controls, then register a key locally:

```bash
dockpipe remote security-key-register --attestation-roots /path/to/verified-manufacturer-roots.pem
```

Obtain the manufacturer's attestation CA certificates through its authenticated
published documentation or metadata and verify the selected roots independently.
Include its published intermediate CA certificates in the PEM bundle when a key
does not return the complete chain. Only explicitly supplied self-signed roots
become trust anchors; supplemental intermediates must chain back to those roots.
Do not use your operating system's general HTTPS CA bundle or an unverified
certificate supplied by a pairing requester. The accepted initial attestation
formats are `packed` and `fido-u2f` with a certificate chain to these explicit
anchors, user presence and user verification, and no backup eligibility/state.
Keys without usable manufacturer attestation are rejected. Certificate validity is
checked at enrollment; stored keys retain that enrollment-time provenance so later
attestation-certificate expiry cannot disable the broker. This is not automatic
manufacturer revocation monitoring. Existing trust anchors
cannot be replaced through this command or broker HTTP APIs.

Dockpipe uses a native FIDO2 adapter through the installed libfido2 library;
no browser, localhost listener or browser extension is used. Linux and macOS
amd64/arm64 adapters load libfido2 only from supported absolute system locations.
The library must already be installed; this command does not install software.
Native Windows key interaction is not implemented in this slice and fails closed.
The existing Windows broker verification and machine identity contracts remain.

From a terminal, the CLI lists connected keys, displays the exact operation and
verified machine details, and asks for the selected key's existing FIDO2 PIN with
echo disabled. Then touch the key. A wrong PIN ends the attempt; there is no
automatic retry, PIN setup, PIN change, reset or fallback to a software credential.
The PIN belongs to the key, not Dockpipe. This prompt never creates a new PIN.
Cancellation restores the terminal. In-flight device operations have a bounded
timeout; device cleanup waits for the native operation to finish.

In the GUI, **Machines → Set up remote access → Pairing security keys** provides
first-key and spare-key registration. Select independently verified manufacturer
PEM roots for the first key. Stop the broker before registering; Dockpipe does not
stop services implicitly. Pairing approval and registration use a normal Dockpipe
dialog with plain consent text, key selection, a masked PIN field and touch status.
Closing the dialog cancels the operation.

The GUI invokes the same CLI with `--interaction gui`. Bounded, request-bound JSON
prompts travel over the child process's private stdout channel and PIN responses
over its stdin pipe; the host consumes control frames without putting them in
logs. CLI defaults to `--interaction terminal` and works without the GUI running.
PINs are not passed in arguments, stored in settings, or written to logs. Owned
buffers are cleared where practical; ordinary Go/Qt heap and OS transport memory
are not a guarantee of locked or completely erased secret memory.

The same broker-side WebAuthn verifier checks native authenticator responses,
manufacturer chains, challenges, user presence and verification. The existing
`localhost` relying-party ID and client-data origin remain signature namespaces
for credential compatibility; they do not cause a web page or listener to open.
No remote administrator token can register keys through HTTP.

Register a spare before relying on a single key. Adding another key uses:

```bash
dockpipe remote security-key-register
```

It first verifies an existing key, then registers the additional key against the
same manufacturer roots. Up to eight keys are supported. Start the broker again
after registration. Losing every registered key requires deliberate local owner
recovery; there is no password fallback or remote reset endpoint. Key removal,
manufacturer trust rotation and automated revocation metadata are not yet exposed.

### Pair without transferring a file

Use the same updated CLI version on the broker and worker. On the broker, open a
15-minute pairing window:

```bash
dockpipe remote pairing-open
```

On the worker, choose a name and explicitly grant workflow delivery authority:

```bash
dockpipe remote pair --endpoint https://your-broker.example.com --node mac-mini --allow-delivery
```

The worker prints a verification code and waits. On the broker, list requests and
approve only after comparing the code and both machine fingerprints on your intended worker:

```bash
dockpipe remote pairings
dockpipe remote approve --code ABCD-1234-5678
```

The code identifies the request. Approval opens the native key prompt and
requires a registered physical key. **Machines → Verify key and approve** uses the
same CLI and broker check. Code-only HTTP requests are rejected, including local
requests with the administrator token. Approval currently requires the broker
computer's local owner state; hosted/remote approval is deferred. Before showing
consent, the CLI verifies the signed machine details, code and WebAuthn options
against the protected local broker identity and a fresh operator nonce.
Pairing challenges expire within two minutes
and bind the exact code, worker identity and credential, node and pairing expiry.
Denial, pairing-close, identity changes, restart and consumed challenges invalidate
approval. Counters and enrollment publish together; uncertain persistence stops
broker requests until restart and inspection.

Before requesting approval, the worker privately saves its generated token and
Ed25519 installation identity. The broker likewise persists an independent
installation identity. The worker verifies the signed pairing response and pins
the broker key; compare both displayed SHA256 fingerprints as well as the code.
The configured HTTPS origin protects first contact; this is explicit first-use
trust, not hardware attestation of the remote computer. Identities belong to the
saved broker/worker installation state, persist across restarts and hostname
changes, and must not be copied between machines. Private identity storage still
uses the existing OS-user service/state boundary; OS-keystore binding and
machine-wide settings are separate follow-up work.

Newly paired workers prove their private key for each request using a bounded,
one-use broker nonce whose signed issuance binds a fresh client nonce. Recorded
challenge/response pairs cannot authenticate a later exchange. Broker responses
are signed and verified against the pinned
key. A token alone cannot impersonate either side of a new pairing. Changed broker
identity fails closed and needs inspection. Older broker binaries strictly reject the migrated private state; preserve it and
review state compatibility before any downgrade. Existing paired workers retain their
legacy credential behavior until explicitly revoked and re-paired; they are not
silently upgraded or retrospectively hardware-verified.

Retries retain identity and authority. Re-pairing after revocation rotates the
worker credential and requires a fresh hardware-key approval; it cannot widen the
saved execution permission. Public requests are accepted only during the window,
with bounded bodies, 32 retained requests and a global 12-new-requests-per-minute
limit. Names remain untrusted until verified.

Use `remote deny --code <code>` to reject a request or `remote pairing-close` to
close the window and deny all pending requests. Cancelling the worker's wait
preserves its private identity for retry; it does not revoke a request already
approved by the broker. `remote nodes` reports enrollment, not live presence.

The Dockpipe app's **Connect this computer** flow starts the worker user service
automatically after approval and reports service startup separately from pairing.
If startup fails, pairing is preserved; use **Start worker service** to retry.
Existing paired computers can use that button without pairing again.
For CLI pairing, start `dockpipe remote service --role worker` on the worker, or
`dockpipe remote worker` for foreground execution. View remote job progress in **Executions → Runs & logs → Remote jobs** on the computer
that manages the broker. Pairing authorizes a worker to receive workflows; it does
not give that worker permission to list, submit, cancel or download broker jobs.
A receive-only worker normally has no `operator.json`. Its remote jobs view explains
this role instead of requesting broker-wide jobs. Enrollment or successful service
installation does not prove that a workflow has executed; submit a job from the
managing computer and inspect its result to verify delivery.

On macOS, bundled CLI services associate their launch agent with the enclosing
app's `CFBundleIdentifier`. The app declares why it needs local-network access,
and packaged helpers retain explicit signing identifiers. Standalone CLI installs
do not claim an unrelated app identity. Existing service definitions require an
explicit replacement to adopt the association; installing new bytes alone does
not restart a running service or transfer a privacy permission.

Allow the app in **System Settings → Privacy & Security → Local Network** when
connecting to LAN hosts. Ad-hoc development signatures can test the exact build,
but reliable identity across updates requires a consistent Apple-issued signing
identity. Duplicate permission entries do not establish which build is authorized.

### Legacy file invitation recovery

Creating invitations and consuming unused invitation files are disabled because
they would bypass hardware-key pairing. An already consumed legacy invitation can
only recover its exact previously enrolled token; it cannot mint a new identity.
Existing workers can continue running their service without pairing again. New
workers must use the hardware-key flow above.

The default local delivery timeout is 3600 seconds, up to 86400 with `--timeout`.
Changing saved profiles or the delivery timeout requires inspecting the private
worker configuration while the worker is stopped; pairing retries do not widen it.

**Delivery grants this broker permission to execute supplied workflow code as the Mac user.** It is
not a sandbox. Only pair with your own trusted broker. Existing profile-only workers remain closed
to delivery. Both broker and worker must use a CLI revision supporting delivery; old versions reject
the new fields. Provider account credentials stay on the broker.

Back on Linux, choose a local workflow and preview the snapshot:

```bash
dockpipe remote submit --node mac-mini --id benchmark-001 \
  --workflow-file workflows/benchmark/config.yml \
  --artifact results/benchmark.json --dry-run
```

Remove `--dry-run` to submit. The preview lists filenames, total bytes, output selections, and the
content digest; it never prints file contents or contacts the broker. Paths resolve from the current
project, or `--workdir <directory>`. Submission sends the selected workflow's directory, including
its assets. A workflow file directly at the project root sends only that file, avoiding accidental
whole-project upload. Use repeatable `--include <relative-file-or-directory>` for inputs outside
the workflow directory, such as source files or small test data.

For package dependencies, add repeatable `--dependency <unpacked-package-directory>`. Each directory
must have a `package.yml` with a unique name and kind `workflow`, `resolver`, or `assets`. Include the
complete `depends` closure; missing dependencies fail with the required name. These are selected
package payloads, not the entire installed store. Bundles and core packages, registry downloads,
compile hooks, host tool installation, and cross-compilation are not performed by delivery. Build
portable source packages or target-compatible payloads first; the Mac still needs its native tools.
For example:

```bash
dockpipe remote submit --node mac-mini --id benchmark-002 \
  --workflow-file workflows/benchmark/config.yml \
  --include fixtures/sample.txt --dependency vendor/metrics \
  --artifact results/benchmark.json
```

The worker verifies the SHA256 digest and package manifest closure, writes the files into a new
private per-job workdir, validates the workflow YAML, then invokes its local Dockpipe executable.
Delivered packages are selected through a generated local project configuration. Global packages
and existing installations are not modified. Workflow imports and external script references must
be included explicitly; arbitrary script dependencies cannot be inferred. Missing runtime inputs
or native tools produce a failed result, not an automatic download or source synchronization.

No sender environment, home directory, project configuration, Git metadata, credential files, or
local state is automatically copied. Hidden paths, common credential filenames, links, special
files, duplicate/case-colliding paths, and file/directory collisions reject the snapshot. Select a
clean source tree rather than renaming private files to evade these checks. Filename checks are not
a secret scanner: review scripts and YAML for embedded secrets. Credentials needed by a workflow
must be configured separately on the worker through its normal local resolver mechanisms.

```bash
dockpipe remote jobs
dockpipe remote result --id benchmark-001
dockpipe remote result --id benchmark-001 --out /private/new-results-directory
dockpipe remote cancel --id benchmark-001
dockpipe remote revoke --id mac-mini
```

For an installed workflow instead, retain the narrower profile mode. Prepare its source on the Mac
and supply a local profiles JSON:

```json
{
  "bench": {
    "workdir": "/Users/you/source/project",
    "workflow": "benchmark",
    "timeout_seconds": 3600,
    "artifacts": ["results/benchmark.json"]
  }
}
```

Pair with `--profiles /private/path/profiles.json`, then submit with `--profile bench` in place of
`--workflow-file`. A profile selects either a workflow name or an absolute `workflow_file`, never
both. Profiles and delivery can be approved together at pairing. Profile submissions cannot replace
local paths, environment, argv, or workflow definitions.

Pairing files must be mode 0600; state directories must be private. Default state is the existing
global Dockpipe data root plus `remote`; all commands accept `--state` for another private directory.

Download writes `result.json`, `workflow.log`, and configured files under `artifacts/`. Results
record exit status, timing, OS, architecture, and log truncation. The workflow owns source/compiler
identity, benchmark measurements, and fresh output generation. Artifact paths are relative to the
profile checkout or the delivered job workdir;
linked, escaping, or oversized files reject.

## Recovery and bounds

The broker allows 64 nodes, 128 recent jobs in its hot metadata window, 32 MiB of broker metadata, one unresolved
assignment per node, a 24-hour profile timeout, 1 MiB of logs, and 8 MiB of artifacts per job.
A delivery allows at most 1024 regular files and 8 MiB of
source bytes, with canonical relative paths up to 512 characters. Executable bits are preserved;
other source permissions/ownership are not transferred.
Result payloads are stored separately under the private `results/` state directory, with
content digests in the broker metadata; maximum-size valid results fit for every admitted job.
Source bundles are separately retained under private `bundles/` files, with digests in metadata.
When a new job needs capacity, the oldest terminal job (success, failure, or cancelled)
is durably moved to an immutable, digest-checked record under private `terminal-jobs/`.
Each archived metadata record is bounded to 16 KiB; result payloads remain separate.
Queued, running, and unknown jobs are never retired. If all 128 jobs are unresolved,
new submissions refuse until work reaches a terminal state.

`remote jobs` lists the recent window. Archived jobs and evidence remain available with
`remote result --id`; exact submission retries return the original outcome, different
contents under that ID refuse, and cancel cannot requeue an archived job. Per-ID archives
and payloads accumulate intentionally to preserve execution identities; automatic deletion
is not implemented. Keep `terminal-jobs/`, `results/`, and `bundles/` together with
`broker.json` for backup/recovery. Worker `deliveries/`
staging and `jobs/` journals are retained for inspection, including failed validation.
Large-corpus transfer is not implemented. Once the first job is retired, broker metadata
contains a downgrade guard that older strict decoders reject. Do not roll back by restoring
a stale pre-migration `broker.json`: it can forget later acknowledged job identities.
Preserve the complete state set and use a broker version that understands archived jobs.

The same submission ID and contents returns its existing job; different contents reject. Assignments
bind a worker session, preventing another process from executing the same job. The worker persists
intent before execution and results before delivery. An interrupted execution becomes `unknown`,
never an automatic retry. Recover its journal and inspect surviving processes before proceeding.
Unknown completion blocks the node. Do not bypass it with a new node identity before confirming
the previous workflow has stopped.

Cancellation is polled during execution. Explicit cancellation reports `cancelled`; a profile
timeout or lost heartbeat stops the process and reports `failure`. Offline polling and result delivery
retry transient connection errors, HTTP 408/429, and server errors at most five times with backoff.
Authentication, authorization, conflicting state, and malformed responses stop the worker immediately.
The result journal remains available for recovery; restarting delivery never reruns completed work. Offline workers cannot immediately acknowledge cancellation/revocation.
Native process groups are terminated. Workflows that detach services or create external resources
must keep their runtime-owned cleanup and are outside native benchmark qualification.

## Resolver contract

A normal resolver profile declares `DOCKPIPE_REMOTE_EDGE_SETUP=assets/scripts/setup.sh`. Core locates
it through existing package/store helpers. The relative script runs through Bash with `--state`,
`--output`, `--hostname`, and `--origin`. `DOCKPIPE_BIN` identifies the caller and
`DOCKPIPE_RESOLVER_PROFILE` identifies the selected profile. Additional provider defaults belong in
the resolver. Hostname may be empty for providers that assign endpoints; Cloudflare requires one.

The script publishes private JSON with `schema: dockpipe.remote/v1`, `endpoint`, absolute
`executable`, and `arguments`. Endpoints require HTTPS off numeric loopback. Credentials must be
private file references, never literal tokens in argv. Core supervises the declared process with a
small ordinary environment and verifies public readiness. Ngrok/AWS/Azure are future adapters, not
implemented providers. Cloud-machine provisioning remains runtime-owned work.

### Provider discovery and connection details

`dockpipe catalog list --format json` preserves the `resolvers` name array and adds
`resolver_details`: name, title, version, description, capability and `remote_setup`.
The details describe the profile selected by normal resolver lookup, including project
precedence. Profile values, scripts and credentials are excluded. `remote_setup` is
`local` for an edge setup hook or `hosted` for the hosted contract below. Conflicting
hooks and incompatible capabilities are not offered in the app's provider selector.
Legacy edge profiles without a capability remain supported.

`dockpipe remote info` returns a safe configuration summary: optional `broker` with
mode, endpoint and recorded resolver metadata; optional `worker` with endpoint and node.
It performs no network probe and does not claim the broker or worker is online.
Older local setups show no resolver metadata until setup records it. Core never guesses
from a hostname. A local metadata snapshot is bound to the edge configuration digest;
changing the edge invalidates the displayed provider instead of leaving a stale label.

The Dockpipe app shows the configured provider and address independently of the provider
selected for a new setup. Setup uses the current workspace's resolver catalog and passes
`--workdir` to the CLI. Provider selection itself performs no setup or account mutation.

### Hosted broker contract

A hosted provider is a resolver package with `capability: remote.broker` and
`DOCKPIPE_REMOTE_BROKER_SETUP=assets/scripts/login.sh`. It must not also declare an edge
setup hook. The package owns browser authentication, account selection, credential
renewal and any provider dependencies. The user does not supply a hostname. The current
app hands authentication to a terminal; a fully integrated Dockpipe Cloud account
sign-in experience is future work, not an available provider.

Core runs the relative, non-symlinked Bash script with `--state <private-provider-dir>`
and `--output <private-staging-file>`, plus the same `DOCKPIPE_BIN` and
`DOCKPIPE_RESOLVER_PROFILE` environment as edge setup. The script must write private
JSON (owner-only file permissions, at most 16 KiB) with `schema: dockpipe.remote/v1`,
`endpoint` (HTTPS origin), and `token` (32–8192 visible ASCII bytes, no whitespace). Do not put tokens
in arguments or terminal output. Provider packages are trusted executable code and must
protect authentication material themselves.

The endpoint must implement the existing broker protocol and authenticate the operator
credential. Core validates the result, checks authenticated `/v1/health`, then atomically
adopts the connection in private `connection.json`. Failed or cancelled login leaves
existing connection credentials unchanged. Operator commands use this hosted endpoint;
local administration continues to use loopback. Workers retain explicit consent and
outbound connections. Hosted setup never initializes or starts a local broker or tunnel.

Setup can reauthenticate the same provider and endpoint. Replacing a configured provider,
changing its endpoint or mixing hosted/local broker state is rejected; use a separate
`--state` directory. Automatic provider migration, account logout/revocation, background
token renewal and multi-connection selection in the app are not implemented.
A future Dockpipe-managed provider must implement these identity/service requirements;
this contract alone does not supply a hosted service or multi-tenant authorization.

## Verification

```bash
# From the Dockpipe runtime checkout:
go test -race ./src/lib/infrastructure/remote ./src/lib/application/internal/remotecmd
# From the external package checkout, with its pinned Dockpipe SDK:
python3 "$PACKAGE_REPOSITORY/packages/remote/tests/smoke.py" "$DOCKPIPE_BIN"
python3 "$PACKAGE_REPOSITORY/packages/remote/tests/delivery_smoke.py" "$DOCKPIPE_BIN"
```

The smokes use real CLI broker/worker processes and native workflows on loopback. The delivery
smoke removes the original sender paths before starting an empty worker, executes a workflow
with assets, an extra input and a package dependency, and retrieves logs/results. It also checks
preview, pairing retries/authority changes, and submission identity. Neither smoke contacts
Cloudflare, installs services, or runs a production workload. Resolver tests use fake provider commands and
exercise the desktop environment passed to the browser launcher.

References: [Cloudflare login/setup](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/create-local-tunnel/),
[account and tunnel permissions](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/tunnel-permissions/).


### Binding a preview to submission

`remote submit --expected-digest <bundle_hash>` requires the exact bundle digest
returned by `--dry-run`. A changed snapshot fails before contacting the broker.
The Dockpipe app uses this to keep the file review and actual delivery consistent.

Removing machine access retains its record. Pairing again with the same name requires a
fresh worker credential and a newly approved verification code. An explicit `remote pair`
rotates a saved revoked credential; the background worker never re-enrolls itself. Old
credentials remain unable to authenticate after replacement.

## Optional machine monitoring

Updated workers can opt into sharing system/process or Docker inventory snapshots
with their existing broker using `dockpipe remote monitoring --system --docker`.
Use the worker's existing `--state` directory; `--disable` revokes both categories.
This does not change pairing or workflow-delivery permission. Updated operators
can select that machine in **Executions → System** or **Containers**, or run
`dockpipe remote observe --node <name>`. Remote Docker is read-only inventory and
topology. See [Execution Explorer and telemetry](execution-observability.md#overall-resources-and-remote-machines)
for platform support, freshness, privacy and resource limits.

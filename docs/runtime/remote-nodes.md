# Remote nodes

`dockpipe remote` delivers one explicitly assigned workflow to a user-owned node. The node makes
outbound HTTPS requests and invokes the existing local CLI. No SSH server, inbound worker port,
remote shell, source synchronization, or automatic Git operation is involved.

This initial single-operator CLI has Linux loopback proof. Darwin builds and launchd manifests
do not establish native Mac, sleep/wake, or live Cloudflare readiness. There is no remote GUI yet.

## Ownership

Core owns the provider-neutral protocol, pairing, job state, cancellation, local invocation,
results, and process supervision. Resolvers own browser authentication, tunnels, DNS, and provider
configuration. Workflows still define what runs; runtimes still define local isolation. DorkPipe
retains graph scheduling, placement, fan-out, and distributed orchestration ownership.

The Cloudflare resolver ships in `packages/remote`. Additional providers implement the same edge
contract without vendor branches in core. TASK-015's `node-execution.v1` fixtures stay unchanged;
this `dockpipe.remote/v1` transport does not turn its fake broker into a hosted service.

## Build the resolver

Use a CLI built from this revision. Install it at a stable path before installing services:

```bash
go build -o /tmp/dockpipe-remote ./src/cmd
/tmp/dockpipe-remote package build source --workdir . --only remote
```

The package hook uses the source-build SDK, stages a resolver in package runtime state, and compiles
a self-contained resolver tarball with the host-native helper. Only the broker needs this resolver.
Use normal package distribution for that compiled artifact. Compiling the source-only resolver over
it removes the bundled binary. Build separate artifacts for other OS/architecture combinations.

Setup reuses DockPipe's existing host-dependency preflight. On macOS it can offer the declared
`brew install cloudflared` installer. Linux users install the official Cloudflare package first;
this resolver does not add package repositories. Noninteractive setup does not silently approve
dependency installation. `DOCKPIPE_CLOUDFLARED_BIN` selects an explicit executable after preflight.

## Cloudflare setup

With a domain already managed by your Cloudflare account:

```bash
dockpipe remote setup --resolver cloudflare --hostname benchmarks.example.com
```

This command authorizes its documented setup effects: browser login, one named tunnel, its DNS
route, private local state, and a user service. Passwords and MFA stay in Cloudflare's browser flow.
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

## Pair and run

On the broker, issue a 15-minute invitation:

```bash
mkdir -m 700 pairing
dockpipe remote invite --node mac-mini --out "$PWD/pairing/mac-mini.json"
```

Transfer the private file to the Mac. Do not commit it or paste it into chat. It enrolls one worker
credential; an exact retry recovers a lost response. It contains no Cloudflare or operator credential.

Prepare the desired source revision and a local profile on the Mac. These example paths must match
an actual workflow; the connector does not create a Nucleon benchmark workflow:

```json
{
  "nucleon-bench": {
    "workdir": "/Users/you/source/nucleon",
    "workflow_file": "/Users/you/source/nucleon/workflows/benchmark/config.yml",
    "timeout_seconds": 3600,
    "artifacts": ["results/benchmark.json"]
  }
}
```

Alternatively, `workflow` selects a named installed/source workflow. Set exactly one selector.
Profiles grant local workflow authority. Requests cannot supply argv, environment, paths, workflow
bodies, or new profiles. A workflow retains the local user's ordinary authority; this connector
does not add a host sandbox.

```bash
dockpipe remote pair --invite /private/path/mac-mini.json --profiles /private/path/profiles.json
dockpipe remote service --role worker
```

Use `dockpipe remote worker` for foreground diagnosis. Pairing files must be mode 0600; state
directories must be private. Default state is the existing global DockPipe data root plus `remote`;
all commands accept `--state` for another private directory.

Back on the broker:

```bash
dockpipe remote submit --node mac-mini --profile nucleon-bench --id nucleon-arm64-001
dockpipe remote jobs
dockpipe remote result --id nucleon-arm64-001
dockpipe remote result --id nucleon-arm64-001 --out /private/new-results-directory
dockpipe remote cancel --id nucleon-arm64-001
dockpipe remote revoke --id mac-mini
```

Download writes `result.json`, `workflow.log`, and configured files under `artifacts/`. Results
record exit status, timing, OS, architecture, and log truncation. The workflow owns source/compiler
identity, benchmark measurements, and fresh output generation. Artifact paths are checkout-relative;
linked, escaping, or oversized files reject.

## Recovery and bounds

The first implementation allows 64 nodes, 128 retained jobs, 32 MiB of broker metadata, one unresolved
assignment per node, a 24-hour profile timeout, 1 MiB of logs, and 8 MiB of artifacts per job.
Result payloads are stored separately under the private `results/` state directory, with
content digests in the broker metadata; maximum-size valid results fit for every admitted job.
Keep that directory together with `broker.json` for backup/recovery.
Automatic retention and large-corpus transfer are not implemented. Download evidence and plan a
fresh broker state when the configured capacity is exhausted.

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

## Verification

```bash
go test -race ./src/lib/infrastructure/remote ./src/lib/application/internal/remotecmd ./packages/remote/tools/...
python3 packages/remote/tests/smoke.py /absolute/path/to/new/dockpipe
```

The smoke uses real CLI broker/worker processes and a native workflow on loopback. It does not
contact Cloudflare, install services, or run Nucleon. Resolver tests use fake provider commands.

References: [Cloudflare login/setup](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/create-local-tunnel/),
[account and tunnel permissions](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/tunnel-permissions/).

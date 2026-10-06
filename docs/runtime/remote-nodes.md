# Remote nodes

`dockpipe remote` delivers one explicitly assigned workflow to a user-owned node. The node makes
outbound HTTPS requests and invokes the existing local CLI. No SSH server, inbound worker port,
inbound shell endpoint or automatic Git operation is involved. Source delivery is an explicit,
bounded snapshot; it does not synchronize an entire checkout.

This initial single-operator CLI has Linux loopback proof. Darwin builds and launchd manifests
do not establish native Mac, sleep/wake, or live Cloudflare readiness. There is no remote GUI yet.
Remote broker initialization, workers, and provider setup currently require Linux or macOS;
Windows remote workers remain a follow-up. Cross-platform protocol, storage, and process tests
do not establish Windows remote-node support.

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
Setup reports its login, tunnel, DNS, and readiness stages and prints the next pairing command.
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

## Pair and run

On the broker, issue a 15-minute invitation:

```bash
mkdir -m 700 pairing
dockpipe remote invite --node mac-mini --out "$PWD/pairing/mac-mini.json"
```

Transfer the private file to the Mac. Do not commit it or paste it into chat. It enrolls one worker
credential; an exact retry recovers a lost response. It contains no Cloudflare or operator credential.

On the Mac, explicitly authorize workflow delivery from this broker and start the worker:

```bash
dockpipe remote pair --invite /private/path/mac-mini.json --allow-delivery
dockpipe remote worker
```

Pairing prints the next command. Use `dockpipe remote service --role worker` instead of the foreground
worker when you want a persistent user service. `--timeout 3600` is the default local delivery limit;
it can be set during pairing, up to 86400 seconds. Pairing does not silently change saved authority:
an exact retry is allowed, but changing profiles or the delivery timeout requires inspecting and
editing the private worker configuration while the worker is stopped.

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
private per-job workdir, validates the workflow YAML, then invokes its local DockPipe executable.
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
global DockPipe data root plus `remote`; all commands accept `--state` for another private directory.

Download writes `result.json`, `workflow.log`, and configured files under `artifacts/`. Results
record exit status, timing, OS, architecture, and log truncation. The workflow owns source/compiler
identity, benchmark measurements, and fresh output generation. Artifact paths are relative to the
profile checkout or the delivered job workdir;
linked, escaping, or oversized files reject.

## Recovery and bounds

The first implementation allows 64 nodes, 128 retained jobs, 32 MiB of broker metadata, one unresolved
assignment per node, a 24-hour profile timeout, 1 MiB of logs, and 8 MiB of artifacts per job.
A delivery allows at most 1024 regular files and 8 MiB of
source bytes, with canonical relative paths up to 512 characters. Executable bits are preserved;
other source permissions/ownership are not transferred.
Result payloads are stored separately under the private `results/` state directory, with
content digests in the broker metadata; maximum-size valid results fit for every admitted job.
Source bundles are separately retained under private `bundles/` files, with digests in metadata.
Keep `results/` and `bundles/` together with `broker.json` for backup/recovery. Worker `deliveries/`
staging and `jobs/` journals are retained for inspection, including failed validation.
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
python3 packages/remote/tests/delivery_smoke.py /absolute/path/to/new/dockpipe
```

The smokes use real CLI broker/worker processes and native workflows on loopback. The delivery
smoke removes the original sender paths before starting an empty worker, executes a workflow
with assets, an extra input and a package dependency, and retrieves logs/results. It also checks
preview, pairing retries/authority changes, and submission identity. Neither smoke contacts
Cloudflare, installs services, or runs Nucleon. Resolver tests use fake provider commands and
exercise the desktop environment passed to the browser launcher.

References: [Cloudflare login/setup](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/create-local-tunnel/),
[account and tunnel permissions](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/tunnel-permissions/).

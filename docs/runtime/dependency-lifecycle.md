# Dependency lifecycle: inventory and dry-run plans

The initial lifecycle slice is CLI-first and local. Core owns the inventory
contract, validation, assessment aggregation and non-executable review plans.
Resolver packages own tool detection and advisory/version semantics. It reuses
resolver discovery, the existing workflow runner, private state helpers and
operation results. It adds no scheduler or package installer.

## Commands

```sh
dockpipe dependency inventory --resolver example.provider --machine workstation --workdir /path/to/project > inventory.json
dockpipe dependency assess --report inventory.json
dockpipe dependency plan --report inventory.json --dry-run --policy security-only --pin 1.2.3
```

Inventory accepts a 5–120 second deadline through `--timeout-seconds` (default
60). The resolver must be explicitly selected from existing installed stores or
declared authoring roots. There is no automatic provider selection or fallback.
`--machine` is an operator-supplied observation identity, defaulting to `local`;
it is not cryptographic host identity or a remote execution target.

Inventory runs the selected resolver workflow with ordinary Dockpipe workflow
permissions. Select only trusted, reviewed resolver code, including when using a
project authoring root. The inventory protocol and contained workflow path do not
create an OS read-only sandbox. This milestone's first-party inspector performs
read-only detection. `assess` and `plan` do not run provider code; supplied reports
are untrusted evidence and never execution grants.

All five policy names are accepted: `manual` (default), `notify`, `security-only`,
`compatible`, and `latest`. This milestone records intent; it does not implement
unattended mutation or compatible/latest release selection. Provider advisory fix
floors are guidance, not authenticated artifact candidates. Every emitted plan
has `dry_run: true`, `executable: false` and explicit unresolved execution gates.

`--pin` is an additional exact constraint, not a new lockfile format. A differing
advisory fix produces `pin-conflict`, including under `latest` and `security-only`.
Existing ecosystem/package locks are not yet resolved; that remains a blocker
under every policy. The producer CLI lock is never treated as a host-tool lock.

## Optional resolver protocol

A resolver may add these profile assignments without replacing its normal
workflow or its existing capability:

```text
DOCKPIPE_DEPENDENCY_LIFECYCLE_PROTOCOL=dockpipe.dependency.inventory/v1
DOCKPIPE_DEPENDENCY_LIFECYCLE_WORKFLOW=assets/workflows/dependency-inspect/config.yml
```

Core resolves the workflow inside the selected resolver, rejecting escapes,
including links to outside files. Resolver catalog metadata exposes only
`dependency_inventory: true` for this known protocol, never the private hook path.
A separate workflow manifest with no host-tool requirement lets inspection report
a missing dependency without trying to install the dependency being inspected.

Core invokes its own CLI's ordinary `--workflow-file` path with explicit
`DOCKPIPE_DEPENDENCY_OUTPUT` and `DOCKPIPE_DEPENDENCY_MACHINE` environment bindings.
The output is a unique private receipt in a Core-created invocation directory;
ambient output/artifact bindings cannot choose it. The private receipt directory
is removed on completion. Workflow stdout/stderr are bounded diagnostics, never
the authoritative machine response. The existing process-tree owner handles
cancellation; diagnostics stop at 64 KiB and JSON at 1 MiB. Receipts must be private
regular files, not links, devices or reparse points.

`dockpipe.dependency.inventory/v1` carries provider ID/version, target label,
observation time, at most 128 installations and 64 coverage gaps. Each installation
identifies its dependency, location, owner/method, installed version, active
version/state and provider advisory assessment. Unknown fields, duplicate IDs,
unsupported schemas, trailing JSON, substituted provider/version/target and
oversized fields fail validation.

Active state is `running`, `stopped` or `unknown`. Advisory assessment is `affected`,
`not_affected` or `unknown` and known applicability requires an HTTPS source.
`not_affected` refers only to the reported advisory, not all vulnerabilities or the
latest available version. Incomplete coverage, unknown active versions or
observations older than 24 hours cannot produce a clean aggregate. Known affected
observations remain visible even alongside uncertainty. This is version-based
triage, not proof of exploitation or universal TLS-path remediation.

Plans bind the normalized inventory digest, provider/version, target, policy, pin,
steps and blockers into a deterministic content ID. This ID is not a signature or
approval. Existing operation events record the command outcome; a durable update
history API remains open; supported changes use private provider journals.

## Remaining phases

Authenticated artifact discovery/signatures, full ecosystem lock resolution,
installation-owner/platform matrices, remote target mutation, scheduled checks,
advisory notifications and MCP lifecycle surfaces remain follow-up work in
TASK-049–052. The separate explicit local change and supervised activation lanes
below do not authorize unattended policies or rewrite package locks. Enrollment
and OS unlock alone never approve a dependency change or connector replacement.

## Explicit local changes and recovery

The separate change contract supports provider-owned local installation changes:

```sh
dockpipe dependency update --workdir /project --request change.json --dry-run
dockpipe dependency update --workdir /project --request change.json --approve APPLY_ID
dockpipe dependency recover --workdir /project --request change.json --dry-run
dockpipe dependency recover --workdir /project --request change.json --approve RECOVERY_ID
```

The dry-run invokes trusted provider code to validate the request and stage/probe
verified bytes without replacing the installed target. It can create private
transaction directories. Apply and recovery IDs are distinct SHA-256 identities
of the operation and canonical request. They are explicit local operator
confirmation, not authentication or remote authorization. Changing any input or
resolver version invalidates approval. Core checks the selected resolver's
version and uses its existing workflow runner, process cancellation and bounded
private result transport. There is no automatic retry after an ambiguous exit.

A request has this generic envelope; the selected resolver documents its parameters:

```json
{
  "schema": "dockpipe.dependency.change/v1",
  "resolver": "example.provider",
  "provider_version": "1.0.0",
  "machine": "local",
  "parameters": {"target": "/private/tool", "provider_specific_input": "value"}
}
```

Remote machine IDs are rejected. This interface does not acquire a remote grant,
execute a command through a broker or bypass OS authentication. Inventory review
plans remain non-executable and cannot be passed as change requests. Existing
package locks are not rewritten or overridden. This explicit manual lane is not
an implementation of unattended compatible/latest policies or advisory selection.

A supporting resolver declares:

```text
DOCKPIPE_DEPENDENCY_CHANGE_PROTOCOL=dockpipe.dependency.change/v1
DOCKPIPE_DEPENDENCY_CHANGE_WORKFLOW=assets/workflows/dependency-change/config.yml
```

The runner receives `DOCKPIPE_DEPENDENCY_REQUEST` (private Core-written request),
`DOCKPIPE_DEPENDENCY_OPERATION` (`plan`, `apply`, `recover-plan`, `recover`) and
`DOCKPIPE_DEPENDENCY_OUTPUT` (new private result). The provider emits a strict
`dockpipe.dependency.change/v1` result with `status` and bounded `message`.
Statuses are `ready`, `installed-activation-pending`,
`recovered-activation-pending`, `recovery-required` or `manual-required`.
Attention/manual outcomes return a failing CLI exit code while preserving the
structured result. Workflow failure, cancellation or invalid results never imply
that no mutation occurred: inspect the durable provider journal before recovery.
Operation events include the exact operation request identity and resolver;
provider journals retain installation-specific recovery evidence.

The initial Cloudflare provider supports only explicitly operator-owned standalone
Linux binaries in a private directory. It consumes an already downloaded artifact
and an operator-verified checksum, keeps a durable verified backup, rejects stale
target bytes, and uses atomic replacement with directory synchronization. The update/recover operations never
restart a tunnel, service or broker, change credentials/configuration, invoke a
package manager, or claim running tunnel health. Activation is a separate operation below. Manager, service, Docker,
Flatpak and non-Linux update/activation paths remain unqualified. See the owning
remote package's README for its exact input and trust requirements.

In the GUI, Machines → Dependency update and recovery opens a thin CLI client.
Choose a prepared request and review either update or recovery before enabling its
corresponding action. The dialog snapshots the selected bytes privately, retains
no independent policy or installation state, and displays activation-pending and
recovery outcomes. Closing waits for an active bounded operation; an unexpected
app/process exit still requires journal reconciliation. Native platform and live
tunnel activation qualification remain separate from offscreen UI tests.


## Supervised local edge activation

An **edge** is the resolver-declared process exposing the local broker's public
endpoint. It is not a vendor name. Core owns its process lifetime, integrity,
HTTP readiness probe, broker-identity round trip, maintenance admission and
bounded recovery. Resolvers own launch arguments, readiness URLs, release/advisory
rules and installation ownership. Core contains no cloudflared command construction,
version floor, tunnel credential parser or package-manager logic.

```sh
dockpipe dependency activate --request activation.json --dry-run
dockpipe dependency activate --request activation.json --approve ACTIVATE_ID
dockpipe dependency activation-status --request activation.json
```

The resolver's existing change workflow receives `activation-plan`, `activate` or
`activation-status`. Activation has its own exact request approval, distinct from
installation/recovery. The provider prepares a generic
`dockpipe.remote.edge-activation/v1` broker request and invokes the current trusted
CLI. `remote edge-activate --state /private/broker --request request.json
--dry-run|--approve ID` and `remote edge-status --state /private/broker --id ID`
are the underlying vendor-independent interfaces. They use the existing signed
OS-session operator protocol and automatic reauthentication; bearer-only and
worker credentials cannot activate connectors. Status refresh never replays an
activation. Every request binds process/configuration digests, exact fallback,
provider-selected loopback readiness URL, public endpoint, explicit maintenance
policy and timing. The readiness contract is HTTP 200 from a listener owned by
the supervised process; Core does not prescribe the provider's URL path.

The broker pins a private snapshot of its actual native baseline image before
updates. Legacy wrapper/script edges keep their execution behavior; the broker
cannot arm activation where the native image cannot be pinned unambiguously.
Linux activation verifies `/proc` image identity and attributes the loopback
listener to the actual candidate process. Other hosts retain normal edge execution
but have no activation qualification. Snapshotting an interpreter as though it
were the declared script is rejected.

Activation starts the candidate alongside the current connector. The baseline
stays running until candidate image and readiness checks pass. A durable cutover
intent precedes old-process shutdown. Only after shutdown does Core require a
stable signed public HTTPS round trip using a fresh nonce and the broker's existing
machine identity. TLS verification, redirects and environment proxy settings cannot
be weakened by a request. This proves broker reachability through the published
endpoint; arbitrary application routes and all possible TLS origins remain separate
operator-owned probes.

The accepted operation belongs to the running broker, not its HTTP/CLI observer.
A client disconnect does not cancel its bounded recovery. Two consecutive health
failures during the approved recovery window restore the exact verified fallback
once; failure or expiry stops automatic attempts and leaves loopback management
available. The fallback must be health checked too. A successful fallback reports
`activation-recovered` and a failing activation command, never successful remediation.
`active-healthy` is an observation with a check time, not a future availability
promise. The provider displays activation identity, check time and recovery deadline.

Queued, running or uncertain jobs block activation. New submissions stay paused
while replacement or its rollback window can disrupt worker connectivity. Timing
is bounded: startup 2–30 seconds, stable observation 2–20 seconds, total recovery
at most 900 seconds and sufficient for both startup/restore checks. The Cloudflare
provider defaults to 10/3/120 seconds. Maintenance approval explicitly accepts
connector replacement; zero downtime is not promised for a single connector.

Private journals record intent before process mutation. Replayed identities query
existing results instead of launching again. Normal startup retains the separately
pinned baseline even when a new preflight was rejected. Interrupted cutovers use
only verified fallback bytes and remaining recovery authority. Failed replacement
recovery is not retried on every broker startup. Linux parent-death containment
kills the direct connector if the broker crashes; ordinary shutdown also cleans its
process group. The supported cloudflared connector is a direct native process.

The GUI adds **Review activation**, **Activate reviewed connector**, and **Refresh
activation health** to the same request-snapshot dialog. These invoke the CLI;
there is no GUI-owned updater, watchdog or authorization store. Refresh is an
explicit user action and may reauthenticate normally. Core and provider fixture,
TLS and offscreen tests do not claim a real Cloudflare tunnel or non-Linux activation
has been qualified.

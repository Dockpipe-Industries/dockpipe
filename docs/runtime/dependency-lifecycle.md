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
history API and mutation journal are not implemented.

## Remaining phases

Artifact provenance/signature verification, full lock resolution, approved apply,
health validation, rollback/recovery, remote target execution, scheduled checks,
advisory refresh/notifications and GUI/MCP lifecycle surfaces remain follow-up
work in TASK-049–052. Remote enrollment or OS unlock alone will not authorize an
installation or tunnel restart. No current command updates a host dependency,
restarts a tunnel or rewrites a package lock.

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
target bytes, and uses atomic replacement with directory synchronization. It never
restarts a tunnel, service or broker, changes credentials/configuration, invokes a
package manager, or claims running tunnel health. Manager, service, Docker,
Flatpak and non-Linux update/activation paths remain unqualified. See the owning
remote package's README for its exact input and trust requirements.

In the GUI, Machines → Dependency update and recovery opens a thin CLI client.
Choose a prepared request and review either update or recovery before enabling its
corresponding action. The dialog snapshots the selected bytes privately, retains
no independent policy or installation state, and displays activation-pending and
recovery outcomes. Closing waits for an active bounded operation; an unexpected
app/process exit still requires journal reconciliation. Native platform and live
tunnel activation qualification remain separate from offscreen UI tests.

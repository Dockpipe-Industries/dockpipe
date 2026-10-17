# TASK-039 — Unified Capability and Interface Refinement

Backlog contract, recorded 2026-10-17; no runtime behavior is implemented by this record.
The [user proposal](unified-capability-proposal.md) refines TASK-039 and its related owners,
not a new initiative. This contract supersedes conflicting MCP-only registry, mandatory
storage-prefix and always-durable wording in earlier proposals; preserve those as history.

## Ownership and Reuse

- Generic core owns protocol-independent registration, dispatch, generic authorization,
  approvals and optional execution services. Native packages own domain implementations,
  schemas, policy interpretation and state. One implementation serves CLI, MCP, explicitly
  supported REST and existing desktop consumers; no parallel execution engine.
- Use existing packages, catalogs, buckets, version selection and contribution lifecycle.
  Categories aid discovery; they do not require a separate MCP registry, bucket or prefix.
  Installation grants no authority. Contributions bind to the selected package version;
  disabled/removed packages cease discovery and new invocation without affecting others.
- Existing manifest `capability` / `requires_capabilities` describe resolver/discovery
  metadata: see [Capabilities](../../../concepts/capabilities.md). They are not an existing
  callable registry or authorization grants. Inventory compatibility and migration before
  defining a new callable contribution schema; do not silently reinterpret those fields.
- MCP is an adapter to native capabilities; TASK-040 retains separate, untrusted imported
  server boundaries. REST is authenticated, versioned and explicitly exposed per capability,
  never an automatic public surface for everything installed. CLI remains headless with
  structured output and useful exit codes when MCP and REST are disabled. Packages need
  no custom UI to operate; optional views consume the same services.
- DorkPipe remains optional and owns AI, graph, knowledge and context. TASK-042 verifies
  external identities; no new identity platform. Reuse existing observability contracts.
- TASK-038 retains Terraform plan analysis, saved-plan binding, reconciliation and its
  15-minute permission envelopes, fresh evaluation per plan and no automatic renewal.
  Direct Cloudflare capabilities are a separate native domain package with independently
  scoped grants, not implicit Terraform permissions. Inventory existing Cloudflare remote,
  R2 and provider integrations before allocating migrations; this task does not implement
  a new provider package or move private deployment configuration into public source.

## Lightweight Execution Is a Required Gate

Ordinary local commands must not require registry traversal, gateway startup, remote lookup,
identity exchange, a persistent daemon, a database, approval service or durable-job writes.
The registry is a contribution/discovery and invocation facility, not a mandatory detour
for the existing spawn → run → act command path. Trusted native work may run in process
where supported; use isolation where trust or declared policy requires it.

Execution requirements belong to trusted capability contracts and effective administrator
policy. Privileged work opts into required authorization/approval; long-running or recoverable
work opts into durable jobs, leases, polling and optional webhooks as appropriate. Optional
means unnecessary machinery is absent from simple calls, not that a caller can disable a
required control. Aliases, CLI, REST, MCP and UI cannot bypass applicable authorization,
including when durable services or gateways are unavailable. Reject unsupported guarantees.

Before implementation, record a representative local-command baseline on fixed fixtures,
including cold/warm latency, allocations, process/service starts and persistent I/O. Agree
measurable overhead limits from that evidence; do not invent universal timing budgets.
Compare the proposed path against the baseline, and justify or remove material overhead.

## Design Deliverables and Smallest Proof

1. Inventory existing runtime, packages, manifest metadata, registration, transports,
   authorization, jobs, UI, observability and Cloudflare integrations; identify reusable
   owners, gaps, compatibility mappings and migrations before selecting new abstractions.
2. Specify contribution and invocation contracts, policy enforcement, version selection,
   opt-in execution requirements, interface exposure and error/cancellation semantics.
   Record exact schema/docs/editor migrations and ownership; avoid a broad rewrite.
3. Prove two separately installed packages register into one registry and one MCP endpoint.
   Invoke at least one through CLI and explicit REST using the same domain implementation.
   Verify consistent authorization, collisions, version binding and disable/removal behavior.
   Terraform and Cloudflare are target consumers; use bounded fixtures before domain buildout.
4. With MCP/REST disabled and durable/approval services stopped, ordinary local commands
   and simple native CLI contributions still succeed without starting services or writing
   job state. Include core use with no optional packages and offline execution. Capture the
   performance comparison above; a functional-only test does not satisfy this gate.
5. Under those same unavailable-service conditions, privileged calls fail closed when
   required policy cannot be enforced. Test alternate interfaces, aliases and attempts to
   request a lighter mode. Verify durable cancellation/recovery separately for jobs that
   declare it; do not impose durability on every synchronous call.

TASK-043 networking and package/version foundations precede bounded capability work;
DorkPipe knowledge follows. Chat, skills management and unrelated domain packages remain
future work. Finish the existing staging checkpoint before new feature implementation.

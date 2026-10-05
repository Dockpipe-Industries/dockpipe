## Proposed Architecture

### Decision Hypothesis

Do **not** name or model this as `runtime: freenet`. A DockPipe runtime answers where local work runs;
Freenet would coordinate which opted-in node receives an unchanged local task contract. It is also
not a resolver because it does not select a tool or profile that performs the work.

The provisional product shape is a **package-owned decentralized broker/coordination adapter for
`node-execution.v1`**, with a Freenet-specific contract, companion, and optional delegate. A future
name such as a `dorkpipe.freenet` package is descriptive only; naming and package placement require a
later decision. Promotion of any seam into generic DockPipe code requires evidence that it is useful
outside Freenet and cannot be composed from existing package interfaces.

```text
Freenet application / DockPipe client
  -> optional Freenet delegate: identity, signing, local consent
    -> Freenet job contract: mergeable signed coordination records
      -> local Freenet client API
        -> package-owned Freenet companion / node connector
          -> existing node-execution.v1 validation and authority boundary
            -> DockPipe local workflow execution
              -> host step | container runtime | VM runtime with QEMU resolver
```

The companion is not a generic remote shell. It subscribes only to declared contract instances,
accepts only versioned allow-listed DockPipe workflow references, validates the complete signed
chain, and submits one exact accepted request to the existing local execution boundary. It exposes
no public runner listener.

## Reuse And Boundary Map

| Existing DockPipe/DorkPipe surface | Reuse expectation |
| --- | --- |
| `node-execution.v1` machine identity | Bind one enrolled runner independently of Freenet peer address, connection, or delegate identity. |
| Immutable capability snapshot | Keep the current snapshot unchanged. A Freenet-specific signed advertisement should bind its fingerprint, issuer, and expiry; refreshing it creates a new advertisement and snapshot identity. |
| Task lease and operation ID | Reuse attempt, expiry, cancellation, and idempotency bindings, but do not assume Freenet state supplies linearizable lease arbitration. |
| Execution receipt | Bind the exact request, selected machine/capability snapshot, lease grant, local run, terminal result, artifacts, and cleanup. Exact replay returns the same receipt. |
| Operation results/events | Keep `dockpipe.operation_event.v1` unchanged inside the current `node-execution.v1` event envelope. A separate Freenet coordination record should bind that envelope's fingerprint, runner signature, and predecessor reference without changing either existing schema. |
| Artifact manifests | Preserve the current path/URL-free digest, media type, and size manifest unchanged. Put bounded retrieval, availability, encryption, and retention metadata in a separate Freenet-specific record; retrieval failure is failure, not an empty artifact. |
| Local approvals | Keep intent, placement, dispatch, execution, apply, checkpoint, publication, and any privileged host action as distinct decisions. A signature is evidence only for the exact authority it encodes. |
| Runtime-owned Git lifecycle | Prepare exact revisions and governed workspaces locally. Do not put raw Git commands, credentials, or mutable checkout authority in the Freenet adapter. |

Freenet-specific state, contracts, SDK bindings, delegate assets, and compatibility tests belong in
the eventual package. `src/lib/` and `src/cmd/` remain generic.

## Proposed Coordination State

The contract should be designed as bounded immutable record sets with deterministic merge and
materialization rules, not an append-only sequence whose correctness depends on arrival order.
Candidate record families are:

- job request and request revocation
- immutable runner capability snapshot and expiry
- claim proposal
- authoritative lease grant, renewal, expiry, and cancellation
- per-attempt event envelope and acknowledgement watermark
- terminal execution receipt and cleanup outcome
- artifact availability/retention observation
- bounded tombstone or closed-epoch record

Every record needs a schema version, record ID, job/operation ID, issuer identity, signing-key ID,
issued/expiry time where relevant, payload digest, signature, and replay identity. The contract's
validity predicate must revalidate signatures, declared bounds, immutable bindings, and allowed
state transitions that can be expressed safely under merge.

### Identity And Idempotency

- Define `operation_id` as a stable digest over the protocol version, origin identity, origin nonce,
  and finalized DockPipe request fingerprint.
- Keep Freenet contract identity, origin application identity, delegate identity, DockPipe machine
  identity, capability snapshot, lease, attempt, local run, and execution receipt distinct.
- Key local deduplication by stable operation ID and exact request fingerprint. Re-delivery of the
  same accepted operation resumes or returns the same receipt; a conflicting payload fails closed.
- Never infer machine identity from a Freenet peer address or current connection.

### Lease And Partition Rule

Freenet's convergence model does not by itself provide an exclusive distributed lock. A runner
publishing a claim is therefore not authorized to execute.

For the trusted private-swarm proof, one explicitly trusted job-authority key should publish a
signed lease grant selecting exactly one machine, capability snapshot, request, attempt, and expiry
from the visible claim set. The grant may be produced by the originating client or its delegate; it
is not a centralized hosted service. A runner must observe and validate that exact grant before
execution and must deduplicate locally by operation ID.

Partitions can still delay a cancellation or reveal competing valid-looking histories. The proof
must define a conservative stabilization/expiry rule, reject conflicting grants, and demonstrate
that uncertainty stops new execution. A later threshold or deterministic decentralized grant model
is research; it must not be claimed to provide exactly-once execution without a proof.

### Event Ordering Over Mergeable State

DockPipe's ordered per-run event semantics remain local and authoritative. Each outer event record
binds the operation, attempt, lease, sequence, previous-event digest, inner event digest, and runner
signature. Freenet may deliver these records in any order or more than once. Consumers reconstruct a
contiguous verified chain and expose gaps, forks, and stale attempts explicitly. Contract merge order
does not become execution order, and a status projection does not grant lifecycle authority.

## Delegate And Local Companion Boundary

A DockPipe Freenet delegate may be useful for:

- holding application signing and identity keys
- signing exact job requests, claims, grants, approvals, cancellations, events, and receipts within
  a declared role
- enforcing caller-specific signing policy from Freenet-attributed sender identities
- prompting for consent when an unfamiliar application, runner, permission, or workload is seen
- connecting Freenet application identity to DockPipe audit records without exporting private keys

It must not be assumed to launch a native process, open arbitrary local IPC, access the DockPipe
workspace, or act as the runner. Official documentation describes delegates as sandboxed WebAssembly
message handlers; no supported native-process bridge is established by this task.

The first proof should use a separately installed least-privilege companion that talks to the local
Freenet node through a supported authenticated client API and to DockPipe through a narrow local-only
authenticated interface. If an exact delegate-to-companion message path is required, obtain an
upstream-supported pattern or capability first. Do not bypass the delegate sandbox, reuse a browser
auth token as runner credentials, add ambient localhost trust, or weaken DockPipe approval policy.

A delegate approval and a DockPipe local execution approval may be related but remain separate
artifacts. Each must bind the exact request and scope it authorizes. Neither approval grants apply,
checkpoint, commit, push, deployment, publication, secret access, or a future job unless those
actions are independently requested and approved.

## Secrets, Source, Logs, And Artifacts

- Contract state contains no plaintext secret, credential, private key, access token, private source,
  raw environment, or resolved vault value.
- Secret references remain local and are resolved only by an already authorized runner whose policy
  explicitly allows the workflow. A reference meaningful on one runner must not silently select a
  secret on another.
- Private repositories require an explicitly trusted runner, an exact immutable revision, local
  credential policy, and a runtime-owned workspace. Public or community runners do not receive
  private source in early phases.
- Freenet should carry only bounded status, hashes, signatures, compact diagnostics, and artifact
  metadata. Large logs and artifacts belong in an external content-addressed store or an explicitly
  trusted origin/runner store.
- Artifact references bind digest, size, media type, encryption/access policy, retention class, and
  availability expectations. Retrieval always verifies bytes against the manifest.
- Confidential artifacts require end-to-end encryption for named recipients. Content hashes alone
  can leak equality and must not be treated as confidentiality.
- Retention and garbage collection require explicit ownership. Contract closure can stop new
  references but cannot prove every peer or external store deleted replicated data.

## Security And Abuse Risks

Create a formal threat model before implementation. It must cover at least:

- malicious, malformed, oversized, stale, replayed, or conflicting job records
- botnet recruitment, malware execution, spam, credential attacks, scanning, denial of service, and
  unauthorized cryptocurrency mining
- capability lying, expired capability snapshots, package/runtime version substitution, and policy
  downgrade
- duplicate execution, competing claims/grants, delayed cancellation, lease expiry, and runner loss
  during a partition
- malicious runners that forge status or results, withhold artifacts, leak inputs, or return validly
  signed but false evidence
- malicious origins that attempt sandbox escape, resource exhaustion, network exfiltration,
  persistence, privilege escalation, or lateral movement
- compromised Freenet Core, delegate, companion, DockPipe service, signing key, artifact store, or
  local approval surface
- contract-state flooding, metadata leakage, traffic analysis, stale-but-valid state, Sybil attacks,
  eclipse/routing concentration, and coordinated denial of service
- unsafe artifact parsing, decompression bombs, hash substitution, retention failure, and garbage
  collection races
- UI spoofing or consent fatigue that causes users to approve unfamiliar workloads

Required early mitigations include signed allow-listed workflow identities, exact package/runtime
versions, immutable request digests, short grants, local deduplication, strict input and state bounds,
resource quotas, network-denied-by-default profiles, sandboxed execution, process-tree teardown,
audit records, runner quarantine/revocation, safe cancellation, and explicit human approval for
unfamiliar or privileged work.

Passing validation, a valid signature, a remote attestation, or a reproducible build may improve
confidence but cannot prove arbitrary execution was honest. Phase 1 relies on trusted machines.
Trustless result verification and confidential computing are separate research problems.

## Network Policy Interaction

- A runner whose effective DockPipe policy denies network access cannot participate in live Freenet
  coordination during that execution unless policy explicitly separates the companion's control
  channel from the workload's network namespace.
- Freenet connectivity must not implicitly grant workload egress. The companion may remain connected
  while the spawned workload has no network.
- Fully offline execution can consume a previously materialized, signed, unexpired request/grant only
  if the policy defines how cancellation uncertainty is handled. Default to refusing new work when
  current authority cannot be verified.
- All network policy and effective capability checks remain local and fail closed.


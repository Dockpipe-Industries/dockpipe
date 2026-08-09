# TASK-023 Lightweight Native Host-Sandbox Runtime

## Goal

Deliver `runtime: host-sandbox` as DockPipe's lightweight, OS-enforced native execution option for
workloads that need stronger isolation than unrestricted host execution without the startup and
packaging weight of a full container or VM.

This is a strategic roadmap item. It restores the Linux-first runtime work that was designed under
TASK-007 but was not implemented when the `software.dev` consumer contract closed. TASK-023 owns
the runtime work independently; reopening TASK-007 is neither required nor intended.

## Authoritative Decisions

Start with these sources instead of recreating the research:

1. [Architecture decision](../../research/host-sandbox-runtime-design-decision-2026.md) — approved
   scope, Linux-first decision, MVP boundary, and promotion gates.
2. [Design-audit addendum](../../research/host-sandbox-runtime-audit-addendum-2026.md) — authoritative
   corrections for assurance, dogfooding, Windows, YAML compatibility, Git, and terminology.
3. [Contract and roadmap](../../research/host-sandbox-runtime-contract-and-roadmap-2026.md) — proposed
   authored policy, enforcement report, lifecycle, threat model, phases, and tests.
4. [Platform research](../../research/host-sandbox-runtime-2026.md) and
   [platform appendix](../../research/host-sandbox-runtime-platform-appendix-2026.md) — mechanism and
   compatibility evidence.

The architecture decision and audit addendum supersede conflicting supporting examples.

## Strategic Role

`host-sandbox` should become a first-class runtime profile alongside unrestricted host, container,
VM, and remote execution. It is especially important for fast local agent workers, build/test
steps, and governed developer tooling where container image preparation is disproportionate but
unrestricted host access is unacceptable.

It is a constrained same-kernel runtime, not a security or reproducibility equivalent to a VM.
Hostile native code, kernel/device risk, clean-machine guarantees, or unavailable host enforcement
must select a stronger runtime.

## Current State And Gap

- The cross-platform research, contract proposal, architecture decision, and independent design
  audit are complete.
- A Linux architecture prototype is approved; production implementation and parity claims are not.
- `runtime: host-sandbox` is not accepted by the current authored schema.
- No common enforcement IR/report, Linux driver, active probe suite, conformance harness, runtime
  lifecycle integration, or production implementation exists.
- The current open task index has no owner for this work, which allowed it to become orphaned when
  TASK-007 closed.

## Non-Negotiable Boundaries

- Preserve the architecture model: workflow is what runs; runtime is where and under which
  lifecycle; resolver selects tools/profiles; strategy wraps lifecycle.
- Keep unrestricted `kind: host` behavior visibly separate. Sandbox failure never falls back or
  retries on unrestricted host.
- Compile desired policy separately from observed enforcement. A required unavailable guarantee
  denies execution before the workload starts.
- Keep engine primitives generic. Platform launchers and drivers consume a narrow compiled contract;
  they do not parse workflow YAML, resolve packages, load plugins, or accept policy as shell text.
- Linux MVP is rootless and network-off. It actively probes required user/mount/PID/IPC/UTS/network
  namespaces, filesystem construction, child inheritance, descriptor/environment isolation, and
  descendant teardown.
- Claim cgroup-v2 CPU, memory, process-count, or kill guarantees only when delegation and canaries
  prove them on the current host.
- Use a private home/temp and deny ambient credentials, authority-bearing sockets, Docker/Podman,
  SSH agents, cloud credentials, display/session buses, and host loopback by default.
- Git checkpoint, sync, and publish remain typed runtime-owned session operations, not sandbox
  escape through raw worker Git.
- Preview or experimental assurance requires explicit acceptance. Production promotion requires
  adversarial conformance, crash/teardown proof, provenance, fuzzing where applicable, performance
  evidence, and independent security review.
- Windows remains a separate narrower preview. Required macOS guarantees fail closed until a
  supported public platform mechanism exists.

## Roadmap Slices

1. **Canonical integration and Phase 0 contract.** Fold the audited decisions into canonical
   architecture, security, runtime, and authored-surface docs. Define the versioned normalized
   guarantee/policy IR, enforcement report, driver assurance, and fail-before-execution decision
   contract. Keep current public security YAML backward compatible.
2. **Linux probes and fixture conformance.** Add deterministic probes and fixtures for namespaces,
   mount construction, network-off, credential/FD isolation, cgroup delegation, child inheritance,
   teardown, and unsupported/downgraded guarantees without running arbitrary workloads.
3. **Offline Linux reference prototype.** Implement the smallest explicitly opted-in preview driver,
   initially using Bubblewrap as the namespace/mount constructor while DockPipe owns policy,
   probes, lifecycle, reports, and teardown. Prove no fallback to host.
4. **Runtime integration.** Add `runtime: host-sandbox` through the Go domain model, JSON Schema,
   language support, workflow docs, compiled runtime manifest, CLI/runtime selection, and tests in
   one synchronized change.
5. **Dogfood and promotion evidence.** Run bounded local/offline workloads, measure cold/warm setup
   and teardown, exercise cancellation/crash/orphan recovery, and complete the independent security
   review before any production claim.
6. **Later platform work.** Prototype Windows AppContainer/Job Object behavior and the experimental
   Bound File System API as distinct drivers. Do not schedule unsupported macOS Seatbelt behavior as
   a production runtime.

Each slice needs its own bounded implementation and validation contract. Completion of one slice
does not authorize the next, production promotion, privileged installation, or broader platform
claims.

## Success Criteria

- `runtime: host-sandbox` is a first-class, documented profile and cannot be confused with or
  silently replaced by unrestricted host execution.
- One versioned policy/guarantee model compiles into immutable runtime plans and structured observed
  enforcement reports.
- Required guarantees fail closed before workload execution when mechanisms, assurance, canonical
  paths, or probes do not match.
- Linux preview proves explicit filesystem visibility/write scope, offline networking, ambient
  credential isolation, inherited child restrictions, bounded output/time, and complete descendant
  teardown under cancellation and launcher failure.
- Runtime-specific implementation stays behind generic engine/application boundaries; resolver,
  workflow, and package knowledge does not enter the launcher.
- No preview or experimental evidence is represented as production assurance.
- A measured native-launcher replacement is considered only after it matches or narrows the same
  contract and materially improves deployability, latency, or throughput.

## Dependencies And Adjacency

- TASK-009 sandbox toolchain determinism is adjacent input for executable discovery and provenance;
  it does not own runtime isolation.
- TASK-013 and `software.dev` may later consume the runtime, but provider/session orchestration and
  the software-development workflow do not own its security boundary.
- TASK-014 Dev Containers, the VM package, Docker runtimes, Kubernetes workers, and remote workers
  remain distinct runtime options. `host-sandbox` does not replace them.
- Git runtime sessions must use the existing runtime-owned lifecycle and authorization model.

## Explicitly Out Of Scope For The First Slice

- production runtime implementation or production assurance
- executing untrusted workloads
- network allowlists, DNS brokers, cloud-agent credentials, GPU/device access, or local-service
  brokers
- Windows parity, privileged Windows WFP services, or macOS production support
- setuid helpers, privileged persistent services, or automatic host-policy installation
- changing `kind: host` behavior or adding fallback from sandbox to host
- replacing containers, VMs, Dev Containers, or remote workers

## Next Bounded Slice

Create the canonical-doc integration and fixture-only Phase 0 contract for the normalized guarantee
IR, driver assurance, enforcement report, and fail-before-execution decision. Produce an exact list
of synchronized Go/schema/language-support surfaces for the later implementation, but do not add a
runtime driver or execute a sandboxed workload in that slice.

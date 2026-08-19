# TASK-022 Go-First PipeLang Backend Services And Typed Clients

## Goal

Define first-class backend web-service support across PipeLang and DockPipe so one typed,
transport-neutral service contract can produce and verify:

- a standalone Go backend service;
- typed Qt/C++ client bindings for native and supported WebAssembly builds;
- OpenAPI and JSON Schema artifacts;
- a deterministic in-memory service implementation;
- contract, serialization, authorization, and real HTTP integration tests; and
- DockPipe build, packaging, execution, deployment, and artifact-verification outputs.

Go is the required first production backend target. This aligns with DockPipe's implementation,
portable-runtime and cross-compilation direction, preference for standalone binaries, and low
operational overhead. ASP.NET Core, Qt HTTP Server, WASI, and other ecosystems may become future
resolver targets, but they must not shape PipeLang core or the transport-neutral service model.

This is a future backlog/design initiative only. It does not authorize PipeLang syntax, parser,
semantic model, Service IR, resolver, Go generator, Qt client generator, schema generator, workflow,
runtime, package, deployment, generated-state, or toolchain changes.

## Dependencies And Ownership

- [TASK-021](../pipelang-reactive-application-language/overview.md) is the language prerequisite. It owns stable
  semantic IDs, types, optionals/collections/unions, contracts, effects/authority, determinism,
  replay, semantic graphs, typed HIR/Core IR, executable entrypoints, target profiles, self-hosting,
  and compatibility.
- [TASK-020](../declarative-application-surfaces-and-target-builders/overview.md) owns application surfaces and
  consumption of generated Qt clients. It does not own service or transport semantics.
- TASK-022 owns service declarations, transport-neutral operation semantics, the independently
  versioned Service IR specialization over TASK-021's semantic/Core foundation, resolver capability
  negotiation, Go service generation, Qt client generation, schemas, service testing, packaging,
  and deployment artifacts.

The architecture remains workflow = what, runtime = where, resolver = which platform/tool, and
strategy = lifecycle wrapper. Service compilation is workflow/package intent. Go, HTTP, OpenAPI,
Qt, containers, hosts, VMs, and remote execution are resolver/runtime outputs or selections rather
than new engine primitives.

TASK-021's deterministic Go backend lowers generic executable Core IR and seeds compiler bootstrap.
TASK-022's Go service resolver is a separate specialization that consumes Service IR to add HTTP,
service lifecycle, schema, and packaging behavior. Neither owns or modifies the other's semantics.

## Current Reusable Foundation

Repository investigation must precede design or implementation. Relevant existing ownership includes:

| Foundation | Existing owner/direction |
| --- | --- |
| PipeLang v0.0.0.1 | `docs/concepts/pipelang.md`, `src/lib/pipelang/`, application PipeLang commands/tests |
| Architecture invariants | `docs/concepts/architecture-model.md` and focused agent architecture guidance |
| Capability/resolver packages | package `capability:` plus workflow/package `requires_capabilities` and existing resolver compilation |
| Workflow/runtime/resolver/strategy | existing authored YAML, compiled packages, selection, and validation paths |
| Package-owned implementation | `packages/**` YAML, resolver profiles, assets/scripts, tests, and repo-local binary rules |
| Generated artifacts | project/package artifact manifests, scoped artifact paths, hashes/provenance, and operation results |
| Secret handling | reference-only templates and resolver-owned secret injection; no plaintext generated source |
| Go HTTP evidence | existing package-local `net/http` servers/tests demonstrate conventions but are not a generic service framework |
| Remote and isolated execution | existing DockPipe runtime/resolver selection and task-owned artifacts rather than service-specific infrastructure |

Do not introduce parallel package, capability, artifact, secret, approval, remote-execution, or
runtime abstractions when current DockPipe contracts can carry the requirement. Current HTTP servers
are implementation evidence only; they do not define the Service IR or generator contract.

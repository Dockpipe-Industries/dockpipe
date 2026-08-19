## Required Go Backend Resolver

Go is the first production backend resolver. Generated Go should:

- prefer `net/http`, `encoding/json`, `context`, standard error handling, graceful shutdown, and
  standard testing packages;
- introduce a routing dependency only after a concrete standard-library limitation is documented;
- use a conventional readable Go module/project layout;
- be deterministic, `gofmt`-formatted, inspectable, and traceable to PipeLang semantic IDs;
- compile/test without importing DockPipe internals;
- produce a standalone binary through normal Go tooling;
- cross-compile through explicit normal Go target settings;
- remain dependency-light and suitable for direct host execution or minimal containers; and
- expose explicit hooks for identity, policy, approval, observability, clocks, randomness, secrets,
  and external capabilities.

The Go resolver owns route registration, binding, JSON, validation calls, domain-error mapping,
middleware/adapters, health, observability hooks, startup, graceful shutdown, and generated source
layout. It does not own canonical service semantics.

## Generated Qt/C++ Client

The first client resolver is Qt/C++ to connect backend services with TASK-020's native and
WebAssembly application direction. It should generate:

- typed request, response, optional, union/result, and domain-error models;
- deterministic serialization and validation matching the service fixtures;
- authentication/authorization credential hooks without embedded secrets;
- cancellation, timeouts, and explicitly supported progress/streaming behavior;
- source mappings to PipeLang semantic IDs;
- native Qt build compatibility; and
- Qt WebAssembly compatibility where browser/platform capabilities permit it.

PipeLang remains canonical. The client must not treat generated Go or OpenAPI as its source model.
TASK-020 owns presenting and invoking the client inside application surfaces.

## OpenAPI And JSON Schema

Generate OpenAPI and JSON Schema from Service IR/PipeLang semantics, including stable operation IDs,
routes/bindings, parameters, bodies, response schemas, constraints, declared errors, authorization,
deprecation, and compatibility metadata.

OpenAPI is an output, not the internal semantic model. Arbitrary OpenAPI import need not be lossless
and is outside the first vertical slice.

## Deterministic In-Memory Service

Provide an in-memory implementation/adaptor consuming the same Service IR so tests can invoke
operations without sockets, inject clocks/randomness/effects, exercise policy and approvals, return
declared errors, verify contracts/invariants, and replay exact inputs/results.

The in-memory and Go HTTP targets share semantic conformance fixtures. It must not become a second,
weaker hand-written service implementation or silently omit transport-independent behavior.

## Testing Strategy

Required coverage includes:

- parser/semantic changes and duplicate/invalid service/operation IDs;
- operation binding, transport projection, type compatibility, and source mapping;
- deterministic serialization across Go, Qt, schemas, in-memory, and HTTP;
- constraints, boundary/invalid values, contracts, and structured validation errors;
- declared domain-error mapping and unexpected infrastructure failures;
- identity, authorization, approval, secret-reference, and effect enforcement metadata;
- resolver capability negotiation and unsupported-required failures;
- transactions/idempotency/retry metadata where selected;
- OpenAPI and JSON Schema generation/golden compatibility;
- generated Go formatting, compilation, tests, startup, health, routing, and graceful shutdown;
- generated Qt/C++ client compilation and interoperability;
- native and supported Qt/WASM client builds;
- in-memory and real Go service semantic equivalence;
- exact replay and deterministic fixtures; and
- old PipeLang/service/artifact compatibility.

The real integration suite starts the generated Go binary, exercises it through the generated client
or deterministic protocol harness, captures bounded diagnostics, and shuts it down cleanly.

## DockPipe Workflow And Artifacts

A package-owned workflow should eventually:

1. validate PipeLang and build the typed semantic model;
2. emit and validate Service IR;
3. select and negotiate the Go backend resolver;
4. generate and `gofmt` Go source;
5. run Go tests and build the standalone binary;
6. generate OpenAPI and JSON Schema;
7. generate the Qt/C++ client and build native/eligible WASM profiles;
8. run in-memory contract tests;
9. start the real backend and run HTTP/client integration tests;
10. capture bounded logs, diagnostics, source maps, and test reports;
11. package all exact outputs; and
12. verify the final closed artifact manifest, hashes, provenance, dependencies, licences, and
    runtime requirements.

Expected outputs include generated Go and Qt source, standalone service binary, native/WASM client
artifacts where supported, OpenAPI, JSON Schemas, test reports, source maps, dependency/licence
manifests, logs, and deployment metadata. Generated state belongs in scoped artifacts, never
unintentionally in source or compiled package stores.

## Deployment Direction

The first Go service output should support explicit deployment as:

- a direct standalone binary;
- a minimal container;
- a DockPipe host target;
- a DockPipe VM target; and
- a DockPipe remote target.

Kubernetes, cloud-provider products, edge platforms, and WASI are future resolver/workflow concerns.
Provider and infrastructure details do not enter PipeLang service semantics. Build, package,
deployment, authorization, and live apply remain separately reviewable operations.

## Initial Vertical Slice

The smallest credible end-to-end proof contains:

1. one representative DockPipe-domain service;
2. shared typed request/response models;
3. one GET and one POST operation;
4. path, query, and JSON-body binding;
5. validation from PipeLang constraints;
6. at least two declared domain errors;
7. stable service/operation/type/error identities;
8. effect declarations and basic authorization-policy enforcement hooks;
9. Go standard-library HTTP backend;
10. deterministic JSON serialization;
11. health endpoint and graceful shutdown;
12. OpenAPI and JSON Schema generation;
13. generated Qt/C++ client;
14. deterministic in-memory adapter;
15. contract and real HTTP integration tests;
16. standalone binary packaging; and
17. complete DockPipe artifact verification.

Use a real DockPipe domain such as builds, workflows, packages, sessions, or catalogs rather than an
artificial greeting/todo service. Streaming, websockets, multiple backend ecosystems, Kubernetes,
arbitrary OpenAPI import, and universal deployment are outside this slice.


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

- [TASK-021](pipelang-reactive-application-language.md) is the language prerequisite. It owns stable
  semantic IDs, types, optionals/collections/unions, contracts, effects/authority, determinism,
  replay, semantic graphs, and compatibility.
- [TASK-020](declarative-application-surfaces-and-target-builders.md) owns application surfaces and
  consumption of generated Qt clients. It does not own service or transport semantics.
- TASK-022 owns service declarations, transport-neutral operation semantics, Service IR, resolver
  capability negotiation, Go service generation, Qt client generation, schemas, service testing,
  packaging, and deployment artifacts.

The architecture remains workflow = what, runtime = where, resolver = which platform/tool, and
strategy = lifecycle wrapper. Service compilation is workflow/package intent. Go, HTTP, OpenAPI,
Qt, containers, hosts, VMs, and remote execution are resolver/runtime outputs or selections rather
than new engine primitives.

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

## Architecture Boundary

PipeLang describes transport-neutral service intent:

- services and operations;
- stable service/operation/type/member/error identities;
- typed inputs, outputs, optionals, collections, and declared domain errors;
- constraints, preconditions, postconditions, and invariants;
- effects, authorization policy, and occurrence-specific approval requirements;
- compatibility, deprecation, versioning, idempotency, and transactional expectations; and
- explicit transport bindings separate from the canonical operation.

The Go backend resolver decides how those semantics become Go types, route registration, HTTP
handlers, JSON serialization, validation, middleware/adapters, error mapping, health, observability,
startup/shutdown, and a standalone executable.

PipeLang core and Service IR must not contain Go-specific concepts such as `http.Handler`,
`context.Context`, goroutines, channels, middleware functions, package names, struct tags, module
layouts, or Go zero-value behavior.

The required pipeline is:

```text
PipeLang source
    |
    v
syntax tree
    |
    v
typed semantic model from TASK-021
    |
    v
versioned transport-neutral Service IR
    |
    +----------------+----------------+----------------+----------------+
    |                |                |                |                |
    v                v                v                v                v
Go backend      Qt/C++ client   OpenAPI/schema   in-memory service  build/deploy
source/binary   source/artifact artifacts        and test fixtures  manifests
```

Parser or typechecker code must not directly emit Go, C++, HTTP, OpenAPI, or deployment strings.
Resolvers consume the stable Service IR and must pass semantic conformance tests.

## PipeLang Service Direction

The authoring surface must remain familiar C#-style PipeLang. The following is non-normative and
does not accept keywords, attributes, modifiers, route spelling, contract clauses, or async syntax:

```csharp
[Id("service.builds")]
[Service("builds")]
public service BuildService
{
    [Id("builds.get")]
    [HttpGet("/builds/{id}")]
    [Effects(Effect.DatabaseRead)]
    [Authorize(Policy.BuildRead)]
    public async GetBuildResult GetBuild(BuildId id)
        requires id != BuildId.Empty;

    [Id("builds.start")]
    [HttpPost("/builds")]
    [Effects(Effect.DatabaseWrite, Effect.ProcessSpawn)]
    [Authorize(Policy.BuildStart)]
    [RequiresApproval("start-production-build")]
    public async StartBuildResult StartBuild(StartBuildRequest request)
        requires request.Repository != null
        ensures result.Build.Status == BuildStatus.Queued;
}
```

The design must decide whether `service`/operation declarations are new declaration forms, restrained
attributes over existing interfaces/classes/actions/effects, or another C#-feeling composition. It
must reuse TASK-021 semantics rather than inventing service-only IDs, effects, contracts, results,
authorization, or testing models.

## Stable Service And Wire Identity

Keep these identities distinct and explicit:

```text
PipeLang semantic ID
source symbol name
serialized wire name
transport operation ID
HTTP method and route
```

A source rename or file move must not silently break wire compatibility. The Service IR must define:

- stable service, operation, model, member, and domain-error IDs;
- serialized field and enum names;
- HTTP operation IDs independent of Go function names;
- route/method compatibility and deprecation;
- aliases/migration for deliberately changed public identities; and
- source mappings from every generated output back to semantic IDs and spans.

IDs integrate with TASK-021 semantic graphs, impact analysis, test failures, compatibility reports,
and verifiable change manifests.

## Transport-Neutral Operations

The canonical operation is transport-neutral; HTTP is the first binding. The Service IR should allow
future local IPC, messaging, CLI, embedded invocation, or another resolver transport without
changing the operation's typed contract.

HTTP binding metadata may provide:

- method and route;
- path, query, and header parameter projection;
- request body and response representation;
- status and content-type mapping;
- file transfer; and
- streaming as an explicitly deferred capability.

HTTP assumptions must not leak into domain types or canonical errors. Unsupported bindings or
ambiguous parameter projection fail compilation.

## Types, Serialization, And Compatibility

Existing and TASK-021 PipeLang models are the source of truth. Define deterministic rules for:

- required, optional, and nullable values;
- records, enums, tagged unions/results, collections, and maps;
- stable serialized member/enum names;
- dates/times, decimals, money, binary/file values, and future stream handles;
- defaults and absence versus explicit zero/empty values;
- unknown/deprecated fields and forward/backward compatibility;
- canonical JSON encoding, media types, ordering where relevant, and numeric precision; and
- version evolution with compile-time compatibility diagnostics.

Go defaults, zero values, reflection, and struct tags must not redefine PipeLang semantics. The Go,
Qt, schema, in-memory, and HTTP implementations must pass the same serialization fixtures.

## Validation And Contracts

TASK-021 constraints, preconditions, postconditions, and invariants are canonical. One definition
should drive:

- compiler/type validation;
- generated Go request and response validation;
- generated Qt client validation;
- JSON Schema constraints and OpenAPI documentation;
- generated forms through TASK-020;
- resolver/service runtime guards;
- invalid and boundary-value tests; and
- structured validation errors.

Resolvers implement the normalized contract metadata and pass conformance tests. They do not invent
weaker target-specific validation or silently omit unsupported constraints.

## Effects, Identity, Authorization, Approval, And Secrets

Service operations reuse TASK-021's explicit effect system. Relevant effects include database read
and write, network, filesystem, process execution, environment/configuration, secret access, clocks,
randomness, messaging, external models, and remote execution.

Keep four concepts separate:

- authentication establishes identity;
- authorization evaluates policy;
- approval authorizes one sensitive occurrence; and
- effects declare capabilities exercised by the operation.

PipeLang declares transport-neutral authorization policy requirements rather than a specific
identity provider. Future adapters may implement anonymous/authenticated users, roles, claims,
scopes, tenant boundaries, machine identities, and service-to-service policy. The Go resolver emits
explicit identity/policy/approval enforcement interfaces and fails closed when a required capability
has no selected adapter.

Credentials never appear in PipeLang service declarations, generated Go/Qt source, schemas, or test
fixtures. Generated services use DockPipe's governed secret references and resolver-owned resolution.

The compiler validates declared effects against operation implementations/capability calls and
rejects undeclared widening. External model invocation remains an explicit typed resolver-backed
effect with constrained output, authority, validation, and replay posture.

## Structured Domain Errors

Define transport-neutral typed domain outcomes separately from unexpected infrastructure failures.
Illustrative non-normative direction:

```csharp
public result GetBuildResult =
    Build
    | BuildNotFound
    | BuildAccessDenied;
```

The IR must map declared outcomes deterministically into:

- Go result/error types;
- HTTP status and problem-detail responses;
- generated Qt/C++ client result types;
- OpenAPI responses and JSON Schemas;
- logs/traces and contract tests; and
- in-memory service results.

Arbitrary exceptions/errors must never be guessed into a public status code. Unexpected failures use
a distinct fail-closed infrastructure error contract without leaking private details.

## Transactions, Retry, And Idempotency

Explore typed semantic metadata for:

- read-only operations and transaction boundaries;
- expected isolation and optimistic concurrency;
- idempotency and duplicate-request protection;
- retry safety versus explicitly prohibited retries;
- request/idempotency keys and replay interaction; and
- compensating actions where a resolver can truthfully support them.

Resolvers advertise enforceable capabilities. Compilation fails when a service requires a guarantee
the selected resolver cannot provide; documentation or best-effort behavior is not sufficient.

## Resolver Capability Negotiation

Service target selection is explicit and deterministic. A versioned capability set should cover at
least:

```text
http
json
authorization
approval
health-checks
graceful-shutdown
openapi
json-schema
cross-compilation
standalone-binary
container-packaging
transactions
idempotency
streaming
websocket
```

Required, optional, unsupported, and deferred capabilities are machine-readable. Silent degradation
is prohibited. Capability negotiation uses existing DockPipe package/resolver capability ownership
rather than creating a service-specific plugin system.

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

## Implementation Sequence

When selected:

1. Audit the current repository and TASK-020/TASK-021 outputs; document reusable architecture.
2. Record the Go-first decision and transport-neutral boundary.
3. Define service/operation identity, type, serialization, compatibility, error, contract, effect,
   authorization, approval, transaction, retry, and idempotency semantics.
4. Define versioned Service IR with source mappings and capability requirements.
5. Define resolver capability negotiation and deterministic target/artifact manifests.
6. Add only the minimum accepted PipeLang syntax/semantic projection required by the vertical slice.
7. Implement the Go backend resolver and deterministic in-memory adapter.
8. Generate OpenAPI/JSON Schema and prove conformance from the same IR.
9. Implement the generated Qt/C++ client resolver.
10. Integrate the package-owned end-to-end DockPipe workflow and closed artifacts.
11. Add semantic, negative, compatibility, conformance, build, and real integration tests.
12. Update canonical PipeLang/service, package/resolver, workflow, CLI, and artifact documentation.

Do not attempt unrelated PipeLang expansion, additional backend frameworks, transports, cloud
providers, or deployment systems during the first vertical slice.

## Acceptance Criteria

- One `.pipe` service contract generates a working standalone Go backend and readable deterministic
  Go source.
- PipeLang and Service IR remain transport- and target-neutral with no Go/HTTP/Qt concepts in domain
  types.
- Stable semantic and wire identities prevent source renames from silently breaking compatibility.
- Request/response serialization, validation, contracts, and domain errors agree across Go, Qt,
  schemas, in-memory, and HTTP fixtures.
- Effects, authorization, approvals, secrets, and unsupported capabilities fail closed and are
  enforceable rather than documentation-only.
- OpenAPI and JSON Schema are generated outputs rather than canonical input models.
- The Qt/C++ client interoperates with the real Go service in native builds and supported WASM
  environments.
- The in-memory adapter and real HTTP service pass the same transport-neutral contract fixtures.
- The real service starts, reports health, handles GET/POST requests, emits bounded diagnostics, and
  shuts down gracefully under integration tests.
- DockPipe produces verified source, binary, schema, client, test, provenance, dependency/licence,
  runtime, and deployment artifacts.
- Existing PipeLang programs and DockPipe architecture remain compatible or use explicit migrations.
- Package/engine boundaries remain preserved with service-specific generation inside package-owned
  resolvers/assets/workflows unless a separately justified generic primitive is accepted.

The demonstrated product claim is:

> Define the application contract once. DockPipe resolves it into a standalone Go backend, typed Qt
> clients, native and browser interfaces, schemas, tests, and verified deployable artifacts.

## Next Bounded Design Slice

Produce a documentation-and-fixtures-only decision packet containing:

1. repository ownership/reuse audit and Go-first decision;
2. non-normative PipeLang service fixture using accepted TASK-021 semantics;
3. versioned transport-neutral Service IR fixture with source mappings;
4. identity/wire compatibility, serialization, validation, error, effect, authorization, approval,
   transaction/idempotency, and capability-negotiation decisions;
5. deterministic Go, Qt client, OpenAPI, JSON Schema, and in-memory output fixtures;
6. target manifest and package-owned workflow fixture;
7. positive/negative semantic and resolver-capability fixtures; and
8. the exact initial vertical-slice validation matrix.

That slice must not change production PipeLang syntax/implementation, public YAML/schema, resolver
packages, CLI behavior, target builders, deployment behavior, or generated stores.

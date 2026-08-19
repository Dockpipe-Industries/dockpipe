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

PipeLang core and the shared semantic/Core projection must not contain Go-specific concepts such as
`http.Handler`, `context.Context`, goroutines, channels, middleware functions, package names, struct
tags, module layouts, or Go zero-value behavior. Service IR may carry explicit transport binding
metadata, but
HTTP realization and all Go/Qt/runtime behavior remain resolver-owned and cannot redefine language
types, failures, effects, serialization, or memory semantics.

The required pipeline is:

```text
PipeLang source
    |
    v
lossless syntax trees
    |
    v
module resolution and bound symbols
    |
    v
typed HIR from TASK-021
    |
    v
target-neutral Core IR + versioned semantic projection
    |
    v
versioned transport-neutral Service IR specialization
    |
    +----------------+----------------+----------------+----------------+
    |                |                |                |                |
    v                v                v                v                v
Go backend      Qt/C++ client   OpenAPI/schema   in-memory service  build/deploy
source/binary   source/artifact artifacts        and test fixtures  manifests
```

Parser, binder, or typechecker code must not directly emit Go, C++, HTTP, OpenAPI, or deployment
strings. Service IR is built from the shared projection/Core identities rather than reparsing or
copying language semantics. Resolvers consume the stable Service IR and must pass semantic
conformance tests; a missing required capability fails rather than silently degrading.

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


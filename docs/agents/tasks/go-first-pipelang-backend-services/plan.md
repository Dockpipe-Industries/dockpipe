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

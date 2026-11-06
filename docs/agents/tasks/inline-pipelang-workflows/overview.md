# TASK-055 Inline PipeLang in workflow YAML

## Status, scope and ownership

**Deferred design proposal; no implementation authorization.** Requested on
2026-10-31 and refined against the current checkout. All syntax, profile names,
limits and new manifest fields below are proposed, not supported today. This task
does not start compiler integration, add dependencies, change runtime behavior or
expand the independent native migration/publication work.

Dockpipe owns YAML, workflow contracts, scheduling, execution policy, approvals,
secret resolution and run evidence. PipeLang owns source semantics, compilation,
exported computation and its value adapter through a versioned installed C++
compiler/SDK. Neither owner imports sibling implementation source. The verified
seams and gaps are in [architecture-findings.md](architecture-findings.md).

The smallest useful implementation provides pure typed expressions and small
helpers for computed inputs, conditions and step results. It does not provide
arbitrary scripting, effects, process execution, dynamic code loading or a second
workflow language. Existing workflows keep their current behavior.

## 1. Embedding and scope

Reserve one additive `pipelang` mapping at workflow and step scope. Workflow
`schema: 1` means embedding-contract version 1, not the YAML document version.
The proposed profile identifier is `pipelang.workflow-pure.v1`. Unknown versions,
profiles and keys fail closed; there is no inference from code-looking strings.

- `source` contains one workflow-owned module as a literal block (`|` or `|-`).
  `expr` contains either a quoted UTF-8 string or literal block. Reject folded
  scalars, non-string scalars, custom tags, aliases and merge keys in this namespace.
  Do not change those YAML rules elsewhere. YAML decodes once; no shell expansion,
  `${{ ... }}`, template interpolation or recursive substitution occurs in code.
- One stateless class exposes named pure helpers; immutable records/enums describe
  values. `Rules.Select(...)` below denotes a profile export binding, not proof that
  the existing compiler supports static/free-function workflow exports. The SDK
  must define its export shim and validate the whole reachable pure call graph.
- Helpers share only this module's declarations. No imports, filesystem resolution,
  mutable globals, constructors with effects, async/tasks, reflection, native FFI,
  user recursion or user-authored loops in the first profile. Bounded standard
  collection operations remain possible. Extending this subset is upstream work.
- Reserved immutable bindings: `host` (explicit non-secret platform/architecture
  facts), `workflow.parameters`, `workflow.inputs`, `step.inputs`, `raw` (declared
  output-source fields during output evaluation), and `steps["literal-id"]`.
  Workflow parameter values are declared typed literals or explicitly supplied
  typed caller inputs; no implicit environment/secret binding. CLI override UX is
  a later interface decision, not a new flag asserted here.
- Every step in an opted-in workflow needs a unique stable ID. Step IDs are looked
  up using string literals, never runtime-computed keys. Helpers cannot shadow
  context roots. Computed workflow inputs cannot reference steps or `step`/`raw`.
- Helpers are reusable within one workflow only. A packaged child owns its module
  and scope. YAML imports do not import language symbols. Initially reject embedded
  code arriving through YAML imports with an origin diagnostic; ordinary imported
  steps may be used if IDs remain unique. Cross-module reuse requires a later exact
  dependency/export contract, not ambient package discovery.
- Existing `inputs` bindings and `outputs: file.env` retain their meanings. Typed
  declarations live under `pipelang.inputs`/`pipelang.outputs`; do not overload the
  legacy output path with a mapping. A compute-only step uses `compute: true`, has
  no command/runtime/resolver/process policy, and publishes only typed values.

### Proposed platform-specific build workflow

This is a design example, not a runnable fixture. `Platform` is a proposed
SDK/context enum; the helper export binding and pure profile are prerequisites.
Scripts named below are ordinary workflow-owned command adapters, not language
operations. Choosing a compiler name does not install or grant access to a compiler.

```yaml
name: typed-native-build
docker_preflight: false
pipelang:
  schema: 1
  profile: pipelang.workflow-pure.v1
  scheduling: dag
  source: |-
    public Record BuildOptions {
        public string compiler;
        public string configuration;
        public int jobs;
    }

    public Class Rules {
        public pure int Jobs(int requested) {
            if (requested < 1)
                return 1;
            if (requested > 16)
                return 16;
            return requested;
        }

        public pure BuildOptions Select(Platform platform, int jobs) {
            if (platform == Platform.Windows)
                return BuildOptions("MSVC", "Release", jobs);
            return BuildOptions("Clang", "Release", jobs);
        }
    }
  parameters:
    release: {type: bool, value: false}
    requested_jobs: {type: int, value: 4}
  inputs:
    jobs:
      type: int
      expr: 'Rules.Jobs(workflow.parameters.requested_jobs)'
steps:
  - id: select
    pipelang:
      compute: true
      outputs:
        options:
          type: BuildOptions
          expr: 'Rules.Select(host.platform, workflow.inputs.jobs)'

  - id: build
    kind: host
    run: assets/scripts/build.sh
    outputs: build.env
    pipelang:
      inputs:
        options: {type: BuildOptions, expr: 'steps["select"].outputs.options'}
      env:
        BUILD_COMPILER: {expr: 'step.inputs.options.compiler'}
        BUILD_CONFIGURATION: {expr: 'step.inputs.options.configuration'}
        BUILD_JOBS: {expr: 'step.inputs.options.jobs', format: canonical}
      output_source:
        kind: dotenv
        fields:
          artifact: {key: ARTIFACT, type: string}
      outputs:
        artifact: {type: string, expr: 'raw.artifact'}

  - id: docs
    kind: host
    run: assets/scripts/check-docs.sh

  - id: package
    kind: host
    run: assets/scripts/package.sh
    outputs: package.env
    pipelang:
      when: {expr: 'workflow.parameters.release'}
      inputs:
        artifact: {type: string, expr: 'steps["build"].outputs.artifact'}
      env:
        BUILD_ARTIFACT: {expr: 'step.inputs.artifact'}
      output_source:
        kind: dotenv
        fields:
          artifact: {key: PACKAGE_ARTIFACT, type: string}
      outputs:
        artifact: {type: string, expr: 'raw.artifact'}

  - id: summary
    pipelang:
      compute: true
      outputs:
        package:
          type: string
          expr: 'steps["package"].outputs.artifact.ValueOr("Packaging skipped")'
```

`docs` has no dependency on build/package results and can start independently.
The conditional producer makes its artifact reference `Option<string>`; `ValueOr`
is a proposed SDK-owned optional-value operation. No consumer gets an implicit
empty string or null. `env` is an explicit, validated data-to-environment adapter:
strings copy literally; other permitted scalars require canonical formatting.
It never substitutes values into `cmd`, `run`, resolver names, package identities,
secret references or other execution-authority fields. Scripts remain responsible
for using environment values as data and quoting them safely.

## 2. Compilation lifecycle: explicit build, then evaluate

1. Load/domain/schema-validate YAML and its declared source/import closure. Preserve
   raw bytes and scalar origins before import/group flattening. Verify feature
   version and typed declarations; do not evaluate runtime values.
2. Extract the one source module and expression sites. Ask the installed PipeLang
   SDK to analyze them against generated context signatures and declared types.
   Dockpipe does not parse expressions or manufacture a second semantic checker.
3. During **`dockpipe build` or explicit package compilation**, compile one pure
   module plus exported entrypoints for each condition/input/output site. The SDK
   returns type identities, free-context references, call/export identities and
   source maps. Do not discover dependencies by regex or by executing sample code.
4. Dockpipe constructs the phase/dataflow graph, checks output availability and
   declared boundary types, and asks the SDK to link the final context bindings.
   A graph/type error prevents publishing any executable computation artifact.
5. Stage the module, source maps, value schemas and execution manifest using existing
   package/artifact ownership helpers. Publish an immutable, hash-verified set only
   after all checks succeed. No shared-cache entry is executable while incomplete.
6. Run verifies this set and evaluates entrypoints only when their phase inputs are
   ready. It never compiles PipeLang, invokes a C++ compiler, fetches a compiler,
   or repairs a stale computation artifact automatically.

The workflow/module is the compilation unit; each expression has its own stable
site ID and typed export. All helpers and the approved pure standard library are
statically closed into the artifact. There is no dynamic linking to host libraries
or another workflow's mutable state. Source expressions may bind unknown runtime
values through known types; compilation must not require concrete step outputs.

Existing automatic transitive compilation on ordinary run paths must not compile
this feature. A missing/stale inline artifact returns `build_required` before
workflow side effects, even when ordinary compile-deps defaults are enabled. A
build may populate several target variants explicitly; run never switches target
or silently rebuilds. Runtime WASM interpretation/instantiation is evaluation;
any optional AOT work belongs to build and needs an exact target/engine identity.

Structural CLI validation stays non-executing. Semantic validation reports the
verified build's diagnostics or `build_required`; an explicit build performs
compilation. Editor checks may ask the installed SDK for transient, bounded
analysis of unsaved text, without emitting execution artifacts or running code.

## 3. Typed value boundary

Adopt a schema-directed, versioned value envelope over bounded UTF-8 JSON between
Dockpipe and the SDK evaluator process. This extends the SDK's existing JSON and
manifest conventions; it does **not** repurpose a model-compilation request as an
evaluation request. A new evaluator envelope is justified because no existing
installed contract accepts a compiled workflow module and arbitrary typed inputs.
Its final protocol identifier/encoding must be agreed and published by PipeLang
in phase 0, rather than asserted as an existing API here.

| Value | Proposed v1 representation and rule |
| --- | --- |
| `bool`, `string` | JSON booleans and valid UTF-8 strings. Preserve bytes/code points without implicit Unicode normalization or locale transforms. Reject invalid encodings. |
| `int` | Signed 64-bit checked arithmetic. Wire integer is a canonical decimal **string**, avoiding JSON/JavaScript 53-bit loss; no leading zeros or `-0`. Explicit checked conversions only; overflow/divide-by-zero are evaluation failures. |
| Enums | Nominal type identity plus declared symbol name. No ordinal coercion; unknown symbols fail. |
| Records/structs | Immutable declared fields, exact schema; no unknown/duplicate/missing required fields or cyclic references. No exposed class identity, pointers, capabilities or object graphs. |
| Collections | Bounded `List<T>` with stable element order. General maps/sets are deferred; their later canonical ordering/comparison rules require a new negotiated capability. |
| Optional/nullable | `Option<T>` tagged `some`/`none` forms, distinct from a missing required field. Nullable syntax, if admitted by the SDK, normalizes explicitly to this same boundary. Bare JSON null is not a wildcard conversion. |
| Results | Tagged `Result<T,E>` values with declared payload schemas. They are ordinary typed values, not exceptions or automatic false/empty results. Consumers handle them explicitly; incompatible boolean/input use fails type checking. |
| Floating point | Defer workflow floats/decimal in v1; reject them explicitly. A later profile must define IEEE-754/NaN/signed-zero or decimal semantics and a canonical lossless encoding before exposure. Existing native float64 support is not enough by itself. |

The envelope names its protocol, schema digest and logical entrypoint. Canonical
form orders object keys by UTF-8 byte order, preserves list order, escapes JSON
consistently and excludes nonsemantic whitespace from identity. PipeLang owns the
canonical codec/type projection; Dockpipe uses that published contract and shared
golden vectors. Reject duplicate keys, unknown envelope fields, excess depth/size
and noncanonical scalar encodings before function invocation.

Nominal identities include module/declaration identity and the transitive schema
hash (including enum symbols, field names/types and optionality). Container and
primitive compatibility is exact structural equality; named records/enums require
matching nominal identity and schema. No implicit numeric widening, field dropping
or duck typing. A schema change invalidates the artifact; version upgrades require
explicit adapters or rebuilds, not silent migration. Context schemas are owned by
Dockpipe, language type validation/projection by PipeLang.

All crossing values are copied into invocation-owned storage. No Go/C++ object,
allocator, exception, pointer or internal runtime layout crosses the boundary.
The evaluator owns guest buffers and destroys/reset-isolates them after one
invocation; Dockpipe owns validated returned bytes. A guest offset/length transport
inside WASM, if required, stays private to the SDK, with bounds checks on every
copy. Cancellation, traps, malformed output and timeout are outer evaluation
outcomes (`Success`, `Cancelled`, `Failure`), distinct from a successful computation
whose value is `Result.Failure`. Error payloads are bounded and redacted.

For command output sources, reuse the artifact-scoped dotenv path/lexer with a
strict declared-field adapter: capture once before legacy merging/deletion, reject
duplicate selected keys, invalid booleans/integers, missing required fields and
oversized values. Do not type arbitrary environment variables or scrape stdout.
In DAG mode the captured file is a typed source only, not a shared environment
mutation. Ordered compatibility mode preserves its existing merge rules.

## 4. Deterministic dataflow and execution

The compiler supplies all external value references from every reachable helper
and expression, including both branches. Dockpipe owns graph assembly and validates
unique IDs, unknown symbols, forward references, type compatibility and cycles.
Use phase nodes: workflow computed inputs, step guard, step input/env preparation,
command/compute action, and output validation/publication. Add explicit
`pipelang.after: [step-id]` edges for ordering effects without consuming a value.
Reject all cycles in the conservative union graph, including mutually exclusive
branches and self-output references. Dynamic step IDs and dynamic dependency
lookup are not admitted.

Two explicit scheduling modes prevent accidental semantic migration:

- `ordered` (default when using the feature) retains existing blocking/async batch
  barriers and dotenv behavior. Reject forward references and same-batch sibling
  value dependencies that conflict with those barriers. Add pure evaluation only
  at the corresponding existing step boundaries.
- `dag` opts into new readiness scheduling. No implicit YAML ordering edges or
  global dotenv-output merges; dependence/effect ordering must be declared. Do not
  combine it with existing async groups or `is_blocking` flags in v1. This is an
  additive scheduler capability with its own implementation gate, not existing
  behavior inferred from package compile closure logic.

A guard becomes eligible when its own dependencies are available. False publishes
`Skipped` plus absent outputs immediately; it need not evaluate unavailable step
inputs. True enables input/env evaluation; the action launches only when all those
values are validated. Output expressions run only after a successful action and
strict raw-output admission. Compute-only actions have no `raw` source and publish
all declared outputs atomically after all expressions succeed. No expression may
read another output being published by that same step.

| Producer outcome | Consumer behavior |
| --- | --- |
| Succeeded, required output present | Publish typed value once; dependent phases become ready. |
| Succeeded, required output absent/malformed | Producer fails output validation; do not publish partial outputs. |
| Succeeded with declared optional absence | Publish `None`; consumers must explicitly handle it. |
| Skipped by guard | Terminal `Skipped`; every output reference is statically `Option<T>` (flatten optional once), fulfilled as `None`. |
| Failed/cancelled | Not absence. Stop scheduling new work, cancel/drain active work, mark unstarted dependents blocked/cancelled and preserve the failure outcome. No v1 recovery/defaulting of failed steps. |

This first fail-fast policy also cancels unrelated running steps after an actual
workflow failure; before failure, independent ready steps are never held behind
unrelated output dependencies. Waiting is only for actual data/ordering edges or
an explicit compatibility barrier. Among simultaneously ready nodes use authored
ordinal then site ID as a stable queue order. Completion timing is not observable
by guest code. Keep bounded parallelism and immutable per-invocation snapshots;
never make sibling writes or completion order visible through a shared environment.

Resolve host facts once per run from the Dockpipe process executing the workflow
(after remote delivery), not the UI/controller machine. These are host-step facts,
not inferred guest/container facts. Cross-target workflows need an explicit typed
target parameter; do not silently relabel `host` from a resolver name. Record the
executor and declared target identities. Every evaluation gets only its
transitively referenced declared values. No clock, random, current-directory,
locale or environment observation. Same artifact, context and canonical inputs
must produce the same canonical result. Wall-clock cancellation/resource failure
can still differ across machines; successful-value determinism does not promise
identical scheduling or timeout outcomes. No automatic retries or result memoization
in v1. Preserve existing `finally` handling after active work drains; inline fields
inside `finally` are excluded from the first profile, without changing ordinary
cleanup workflows.

## 5. Enforced purity, isolation and limits

Recommend a **small WASM pure-computation module in an out-of-process SDK evaluator**.
Prefer an interpreter in v1; select and qualify a compatible engine during phase 0.
No such backend/engine adapter is currently verified in the inspected SDK. This
recommendation is gated design, not permission to add a runtime dependency now.
A native in-process C ABI is rejected. Native subprocess execution is not an
acceptable fallback without a separately qualified OS sandbox on every target.
See the [execution-format assessment](architecture-findings.md#execution-format-assessment).

Enforcement has separate layers:

1. PipeLang's dedicated profile rejects effects, unsafe/native calls, imports,
   mutable/ambient state and disallowed recursion/loops across the complete call
   graph. A `pure` modifier alone is insufficient.
2. Dockpipe/evaluator admits only the expected module hash, value-schema/profile
   version and validated WASM feature set. v1 exposes no imports, WASI, hostcall,
   shared memory, threads, dynamic linking or runtime code generation. Reject
   unexpected imports/exports/start sections and unsupported binary features.
   Initialization and all exported functions consume the same invocation budget.
3. The pinned engine enforces fixed linear-memory, stack and fuel limits independently
   of source annotations. Approved SDK allocation/collection operations also bound
   allocations and element counts. Fresh instance state prevents cross-call state.
4. Dockpipe supervises the native helper under its execution policy, with bounded
   input/output, wall deadline, cancellation, concurrency and artifact access. The
   helper receives no inherited secret environment, ambient filesystem preopens,
   network grants or arbitrary native library paths. Compiler/build processes also
   need existing build-policy admission; source purity does not sandbox a compiler.

Proposed initial limits per evaluation: 1 MiB request and 1 MiB response, value
nesting depth 32, 10,000 collection elements, 32 MiB fixed guest linear memory,
64 call frames and 1,000,000 engine-versioned fuel units. User recursion and loops
are rejected; bounded library traversal consumes fuel, and collection/allocator
entrypoints cap cumulative allocation requests at 10,000 and total allocated bytes
at 32 MiB. Allocation churn cannot evade cumulative counters. Guest initialization, SDK guest decoding/encoding and helper evaluation consume
the same guest budget. Host decoding/encoding has independent size/depth limits
and is covered by the outer deadline; fuel cannot meter arbitrary native codec
work. An implementation must demonstrate these limits,
not merely emit them in metadata.

Propose a 1-second outer wall deadline, one guest thread and at most four concurrent
evaluators under an aggregate workflow budget; policy can lower limits. Host helper
memory/CPU enforcement and engine overhead must be measured/qualified per platform
before choosing their release caps. These guest limits are not claims about total
native process RSS, compiler memory or general workflow limits. Unsupported required
enforcement fails admission; no advisory control masquerades as a hard cap. Changes
to limits/metering require a policy version and rebuild; fuel is not portable across
unnegotiated engine versions. OS/container policy is defense in depth, not proof of
WASM safety; runtime/compiler vulnerabilities and malicious build artifacts remain
risks requiring trusted distribution, integrity checks and security maintenance.

## 6. Diagnostics and source navigation

Retain raw YAML bytes, file identity and `yaml.Node` scalar origins. Extraction
builds a byte-span map for literal-block indentation, chomping, CRLF normalization,
quoted escapes and Unicode. Map generated wrapper/context spans through SDK maps,
then through YAML extraction maps. Synthetic bytes map to their owning expression
or end-of-scalar, never an invented YAML line. Reject tabs used as YAML indentation;
valid tabs inside code retain their real byte positions. Include imported file
origins even when groups/steps are flattened.

Use one normalized diagnostic record for CLI, editor and UI: stable code, severity,
phase, message, primary range and related ranges. PipeLang owns language diagnostics;
Dockpipe owns YAML/dataflow/availability/admission diagnostics (proposed `DPPL_*`
codes). Preserve original `PL*` codes/messages and compose locations rather than
reimplementing language errors. Cross-step type errors point at the consumer
expression with the producer declaration as a related range. Missing SDK/profile
or artifact diagnostics name the required identity and corrective build/install
action without fetching anything.

Canonical locations are UTF-8 byte spans; render human 1-based line/columns and
convert to LSP's negotiated character encoding (UTF-16 by default) through one
shared adapter. Test astral characters, combining sequences, empty expressions,
escaped newlines, indentation indicators and diagnostics at EOF. Source navigation
uses the same maps and declaration identities. The legacy structured diagnostic
projection is a reuse candidate; native model diagnostics currently lack enough
range detail, so the SDK must publish the profile's canonical diagnostic/source-map
contract before editor integration.

## 7. Manifest, cache and execution evidence

Propose one computation execution manifest alongside existing package artifacts;
its final schema identifier is agreed in phase 0. It contains:

- Embedding/schema version; workflow logical/package identity; raw workflow digest
  and resolved import/package closure digests.
- Module/source and expression-site digests; source-map artifact identities; helper
  exports and entrypoint signatures.
- Exact compiler and SDK versions plus manifest/executable digests, language profile,
  pure-library identity, value-codec/schema and context-schema digests.
- WASM target/features and artifact hashes/sizes; exact evaluator/engine identity,
  metering/limit policy, declared capabilities (empty in v1) and target variants.
- Typed parameter/input/output/raw-source contracts, optionality, env adapters,
  scheduling mode and phase graph/data/ordering edges.

Mandatory manifest field groups (draft version 1, all proposed):

| Field | Shape |
| --- | --- |
| `schema_version`, `embedding_version` | Positive integers; unknown values fail admission. |
| `workflow` | Logical ID, package identity when present, raw source SHA256 and ordered closure identities. |
| `module`, `sites`, `source_maps` | Source/module digest; stable site/export IDs and signatures; artifact references. |
| `compiler`, `sdk`, `library`, `codec`, `context` | Exact versions/profile IDs plus executable/manifest/schema SHA256 identities as applicable. |
| `execution` | Format, target/features, evaluator/engine identity, metering policy and limits, capabilities. |
| `contracts`, `graph` | Typed parameter/input/raw/output/env contracts, scheduling mode, phase IDs and edge kinds. |
| `artifacts` | Relative owned path, media/format version, byte count and SHA256 for each payload; no absolute paths. |

The source manifest digest binds all groups; nonsemantic build timestamps and local
cache locations are separate provenance. Runtime values are not manifest fields.

Reuse existing stable-JSON/fingerprint, staged package publication, checksum,
artifact-scope and operation-ID infrastructure. Keep a distinct computation
manifest; do not overload image manifests, forge an official package-lock schema
or persist absolute checkout paths/credentials in portable artifacts. Hashes prove
byte identity, not trust in a publisher; enforce existing trusted package admission
and distinguish private local builds from distributable artifacts.

Build cache key is a canonical digest of exact raw YAML/import bytes, extracted
source/expression bytes, embedding
and profile versions, compiler/SDK/library/codec identities, context/boundary types,
entrypoint set, graph, capability/limit policy and WASM target features. Source-location maps bind the raw YAML digest as well: moving code must never
reuse stale diagnostics. v1 does not attempt AST-equivalence or comment-insensitive
cache reuse. Changing any dependency, compiler bytes, target feature, schema,
policy or referenced declaration invalidates its closure. No newest-installed,
mtime-only, partial-write or unchecked cache hit. Verify artifacts on retrieval
and again at execution admission; publish manifest+payload as one validated unit.

Portable WASM payloads may be reused across qualified OS/CPU targets with identical
feature/codec contracts. The native evaluator is selected for the actual target.
Any future AOT cache additionally keys OS, architecture, CPU-feature baseline,
engine/compiler ABI and code-generation flags; never execute another target's native
cache. Run instantiates/evaluates validated artifacts, with no guest compilation or
JIT fallback. Cache eviction follows existing artifact ownership/retention helpers.

Build provenance describes how logic was produced; execution evidence describes
which logic and inputs drove a particular decision. Record workflow/run/step/site,
manifest/artifact/context-schema digests, readiness dependency IDs, target/policy,
outcome, resource counters and guard decision where approved. Do not record raw
inputs, result payloads, environments or unsalted hashes of potentially sensitive
low-entropy values. No secret values enter v1 pure context; script-derived values
can still be sensitive. Default receipts use opaque snapshot IDs; optional replay
uses explicit access-controlled, encrypted snapshot retention, not public logs.
Keep timestamps outside deterministic decision identity.

Reuse run IDs and canonical operation events. Existing observation is best-effort;
new feature decision receipts must be durably committed before releasing dependent
side effects, or fail closed for that feature. Define that stronger publication
boundary explicitly without changing unrelated legacy telemetry behavior. Do not
claim independent replay when input snapshots were intentionally not retained.

## 8. Compatibility and failure behavior

Activation requires the explicit namespace/profile and a Dockpipe version whose
schema/runner advertises that embedding contract. No root YAML schema migration
is required for workflows that do not use it; supporting versions extend the
existing schema and editor mirror together. Older engines reject unsupported
fields. Existing shell strings, inputs, dotenv, async groups, imports and cleanup
semantics remain unchanged when the feature is absent.

Build resolves an explicitly selected versioned installed C++ compiler/SDK through
supported package/artifact resolution and verifies its manifest/hash/profile.
Never use sibling source or whichever executable is first/newest on PATH. Missing,
unpublished or incompatible SDK/profile returns actionable install/selection
failure. Do not fall back to the older Go API or legacy configuration `invoke`.
The exact selection mechanism must be reconciled with its owning supported contract
in phase 0; the proposed package-selection lock design is not an implementation.

Run needs a compatible pinned evaluator and verified built artifact, not the
compiler. Missing compiler alone therefore does not break execution of a complete
compatible package; missing evaluator or stale/incompatible artifacts do. Handshake
must validate embedding, value, context, artifact, target and metering capabilities;
unknown versions fail, and artifacts are not silently migrated. Tests deliberately
remove compilers/build tools at runtime to prove this separation.

## 9. Decisions and upstream prerequisites

Resolved design direction: explicit namespaced opt-in; build-only compilation;
workflow-local stateless helpers; schema-directed JSON value boundary; immutable
phase dataflow; separate ordered/DAG modes; first-profile no effects or floats;
WASM sandbox in a supervised evaluator; no compilation/native-code fallback at run.

Required decisions before implementation are narrow and blocking:

1. PipeLang publishes the pure export/check/compile contract, exact nominal schema
   projection, structured diagnostics/source maps and bounded value codec. Existing
   model C++ headers and legacy `invoke` do not satisfy this API.
2. Select/qualify the WASM profile, freestanding pure library, metered interpreter
   (or explicitly build-time AOT alternative), private guest transport and native
   evaluator packaging for supported targets. Confirm all proposed counters and
   cross-target deterministic golden vectors; no engine dependency is selected now.
3. Agree exact installed compiler/evaluator identity resolution with the package
   owner, and the execution-manifest schema/mandatory decision-receipt publication
   API with Dockpipe's artifact/audit owners.
4. Specify caller parameter UX, the release host/aggregate resource caps and the
   initial qualified OS/CPU rows. Do not infer them from current experimental native
   migration evidence. The scope excludes arbitrary reusable package imports,
   floating point, maps, effectful tasks and conditional recovery of failed steps.

PipeLang owns prerequisites 1–2 and its side of 3; Dockpipe owns orchestration,
readiness, YAML source mapping, policy/admission and evidence. Prerequisites must
be published/qualified contracts, not local source imports or a duplicate compiler.

## 10. Dependency-ordered phases and acceptance gates

All phases remain deferred until separately selected. Partial phases do not close
the task; acceptance requires P5 and both promised scheduling modes. Each gate preserves the
unmodified legacy workflow corpus and requires focused tests before broader proof.

| Phase | Owner and completion gate |
| --- | --- |
| P0: contract agreement | Both owners: resolve the four blocking decisions; freeze profile/codec/diagnostic/manifest vectors, threat model and exact dependency/target matrix. A written spec is not compiler/runtime availability. |
| P1: upstream pure artifact | PipeLang: installed C++ SDK analyzes/exports the small pure subset, emits closed sandbox artifacts and canonical values, rejects capabilities, and independently proves fuel/memory/stack/allocation/cancellation behavior on qualified targets. |
| P2: explicit build and ordered integration | Dockpipe: namespaced schema, source extraction/maps, installed SDK adapter, typed sites and atomic artifacts; no runtime compilation. Conditions/computed inputs/outputs work at existing ordered barriers with backward compatibility. |
| P3: optional DAG readiness | Dockpipe: phase graph, strict raw adapters, compute-only nodes, optional/skipped results, explicit ordering edges and bounded fail-fast scheduling. Independent work starts without unrelated waits; legacy async behavior remains intact outside DAG mode. |
| P4: tooling and evidence | Both: shared CLI/editor/UI diagnostics and navigation, cache invalidation/tamper tests, mandatory decision receipts and privacy controls; no separate language diagnostic implementation. |
| P5: native/installed-package qualification | Both: fresh builds, relocated packages and execution with build tools absent on Linux, Windows and macOS for each claimed CPU row. Approved supported-syntax docs/feature activation only after every required gate passes; no publication is authorized here. |

| Test group | Required observable result |
| --- | --- |
| Expressions/helpers | Constants, pure helper composition, branches, records and context bindings return exact expected typed bytes; recursion/loops/effects outside the subset fail before build publication. |
| Build options | The example selects MSVC on Windows and Clang otherwise, clamps jobs, handles release true/false and keeps docs eligible independently. Does not assert compilers are installed. |
| Computed values | Workflow inputs, step inputs, boolean guards, strict raw outputs and compute-only outputs evaluate at their defined phases; no runtime value is needed to compile its type. |
| Scope/dependencies | Unknown helper/context/export, duplicate IDs, dynamic step lookup, imported code and cross-workflow symbol leakage are refused with useful spans; legal DAG forward references work, incompatible ordered references fail. |
| Missing/skipped/failure | Required missing output fails atomically; optional absence and skipped producers yield typed None; failed/cancelled producers never become None/defaults. False guards do not evaluate unavailable inputs. |
| Cycles/order | Direct, helper-hidden, transitive, self-output and branch-hidden cycles fail build. Explicit effect edges are respected; independent DAG roots are not serialized by unrelated outputs. |
| Determinism | Repeat canonical results across varied completion orders, locales, hash seeds and qualified OS/CPU rows; no ambient time/random/env or shared mutable invocation state. |
| Value contract | Round-trip signed 64-bit boundaries, UTF-8, enum symbols, records, nested lists/options/results; reject overflow, invalid UTF-8, duplicate/unknown fields, schema/nominal mismatch, excessive depth and unsupported floats/maps. |
| Diagnostics | Assert exact raw YAML byte spans and CLI/LSP positions for quoted escapes, block indentation/chomping/CRLF/Unicode/EOF; related producer spans and identical error codes/messages in CLI/editor/UI. |
| Security | Source-level forbidden capabilities fail compile; hand-crafted modules with imports, WASI, unsupported features/exports or substituted payloads fail admission independently of source checks. No host-native fallback or secret inheritance. |
| Exhaustion/cancellation | Fuel, fixed memory, call depth, allocation churn, collection/codec sizes, outer timeout and aggregate concurrency limits produce classified failures; cancellation stops evaluator work and publishes no partial outputs. Test instance initialization too. |
| Cache/artifacts | Every identity dimension invalidates as specified; reject tampering, partial publication, stale source maps, unsafe paths and cross-target AOT caches; relocate and verify exact installed artifacts. |
| Audit/privacy | Exact artifact/site/policy identities and decision outcomes are recorded before dependent effects; receipt-write failure blocks those effects. Canary-sensitive values and their unsalted hashes never appear in ordinary logs. |
| Build/run split | Build succeeds through installed SDK. Run succeeds with compiler/C++ build tools absent and fails with `build_required` or evaluator incompatibility when appropriate; never downloads, compiles, JITs or silently repairs. |
| Compatibility | Unused-feature corpus preserves YAML/shell/input/output/parallel/finally semantics without PipeLang installed; schema and editor mirrors agree on additive activation and unknown-version errors. |
| Native matrix | Execute build, evaluation, failure/cancellation and relocated installed-package consumers natively on Linux/Windows/macOS and every claimed architecture. Cross-compilation, Wine, static schema checks and declarations alone are insufficient. |

## Completed Bounded Production Slices (2026-08-16)

The separately authorized steps 1-4 slices are implemented. Steps 1-2 froze the exact legacy inventory and
introduced source-set/file identities, strict UTF-8 decoding, durable spans, and structured ordered
diagnostics across the existing lexer/parser/compiler entrypoint and direct CLI, catalog,
materialize, package-compile, and editor consumers. Step 3 replaced string-only static type plumbing
with structured unresolved/resolved references and one declaration symbol table. Step 4 added the
syntax-independent explicit module/import/dependency-lock binder while leaving the frozen parser and
every production consumer on `v0.0.0.1`. These slices added no public module/import syntax, public
semantic IDs, new types/declarations/expressions, HIR/Core IR, effects, or backends.

### Step 1 compatibility checkpoint (2026-08-16)

The separately authorized production slice established the frozen `v0.0.0.1` baseline before
source/span plumbing. `tests/pipelangcompat/legacy_compat_test.go` proves that the fixture is the
exact 45-file authored inventory, every file retains its parsed declaration/member/default
projection, every public class retains its workflow YAML and bindings JSON/env artifacts, and the
existing expression-bodied method retains its pure evaluation result. Existing focused compiler,
application CLI, materialize, catalog, and editor fixtures remain the direct consumer coverage.
The checkpoint is intentionally repository-level so the generic `src/lib` compiler does not learn
checkout-only package or workflow paths.

### Step 2 source/span/diagnostic checkpoint (2026-08-16)

`src/lib/pipelang` now admits deterministic path-identified source sets, rejects invalid UTF-8,
retains half-open file-aware byte spans on every token and existing AST node, resolves Unicode and
UTF-16 editor positions, and returns ordered diagnostics with stable code/category/severity plus
primary and optional related spans. `ParseFiles` preserves parse-only consumers while
`AnalyzeFiles` is the shared parse/type contract for compilation and diagnostics. `dockpipe
pipelang check` renders that contract as text or schema-1 JSON without evaluation or artifact
emission, and the VS Code extension consumes the JSON contract for unsaved buffers through stdin.

### Step 3 structured type/symbol checkpoint (2026-08-16)

The completed step-3 slice replaced parser-created type strings with structured unresolved
primitive, named, and applied `List<T>` references, including nested applications and exact source
spans. One deterministic symbol-table contract now owns both legacy interfaces and classes under an
explicit frozen sibling-source-set owner; every declared symbol has one analysis-local identity and
declaration span. Focused checks prove named resolution, cross-kind duplicate detection, unknown-type
locations, and conformance primary/related type spans. Type checking and invocation use resolved type
graphs; compiler, evaluator, CLI, catalog, materialize, package-compile, and editor projections retain
the frozen spellings and artifacts. The exact 45-source compatibility golden and affected offline
consumer/editor checks pass. Analysis-local symbol IDs are deliberately non-semantic and are not
persisted into legacy artifacts.

### Step 4 module/import/dependency-lock checkpoint (2026-08-16)

The completed step-4 slice introduced `ModuleSetInput` and `AnalyzeModuleSet` as the explicit,
syntax-independent compiler seam for a non-legacy language contract, root module, complete source
bytes, structured module/symbol imports, and a complete dependency lock. Locked module records carry
direct dependency identities and a deterministic SHA-256 digest over normalized source identities
plus exact bytes; missing or changed inputs diagnose and compilation never fetches. One symbol table
now supports both the frozen legacy owner and deterministic module-aware owners, allowing equal local
names in different modules while retaining one analysis-local symbol identity, visibility, owner,
and declaration span per declaration. Explicit symbol imports resolve unqualified names; explicit
module imports authorize qualified structured type references; no wildcard, relative, ambient, or
implicit cross-module lookup exists.

Focused tests cover input-order-independent symbol/lock queries, cross-module conformance, module and
symbol import resolution, lock drift, duplicate module owners, undeclared dependencies, unknown and
private imports, ambiguous symbol imports, import cycles, durable primary/related spans, and rejection
of selecting the frozen legacy contract through the module lane. The exact 45-source compatibility
golden plus affected compiler, CLI, catalog, materialize, package-compile, and editor checks retain
their prior behavior and artifacts. This slice deliberately chooses no public post-legacy language
name/release, module/import keyword or casing, manifest/YAML/schema shape, CLI selector, or editor
grammar; those unresolved founder/product spellings remain unguessed.

### Completed step 5 semantic identity/projection checkpoint (2026-08-16)

The first syntax-independent implementation checkpoint established a distinct `SemanticID`,
analysis-local `SymbolID`, semantic lock digest, diagnostic propagation, and deterministic semantic
projection. It used explicit caller-supplied ID-to-span assignments and caller-supplied contract
identities. Founder review subsequently accepted the public contract identities and superseded that
assignment model with the derived-and-baselined identity contract above. The initial code and tests
are preserved as predecessor work but are not the final step-5 contract.

The completed checkpoint replaced missing-explicit-ID behavior with deterministic
package/namespace/name derivation, reserved structured callable identity over ordered resolved
parameter and return `TypeRef`s, represented centralized compatibility/migration input without
production syntax, fixed the public contract constants, and retained deterministic structured
diagnostics and projection bytes. Migration records retain every promised former name, canonicalize
caller ordering into the semantic lock, reject missing/conflicting targets and identity cycles, and
make former public top-level names resolve as deprecated aliases through the one symbol table.

The public `pipelang.semantic.v1` projection is canonical deterministic JSON with separate
`package_id` and semantic path fields, structured named/applied/callable identities, declaration
source ranges, source and semantic lock digests, fixed compiler/language contracts, and explicit
public/workspace views. The public view excludes private declarations and rejects absolute source
paths; the workspace view may retain local source details. Private declarations remain
analysis-local and identity-optional. No timestamp, host path, package version, dependency content
digest, or analysis-local `SymbolID` becomes semantic identity.

Focused proof passed with cached Go 1.25.13 and `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
GOWORK=off` plus writable `/tmp` caches:

- `go test ./src/lib/pipelang ./tests/pipelangcompat`;
- exact equality of the frozen 45-path inventory and current authored `.pipe` inventory, including
  unchanged source, language-projection, emitted-artifact, and evaluation goldens;
- affected PipeLang application/CLI/catalog/materialize/package-compile tests using only the
  already-cached temporary `golang.org/x/sys v0.46.0` substitute because the checkout-pinned v0.28
  source is absent from the read-only module cache; no checkout dependency file changed;
- `go vet` over PipeLang and the affected internal consumers; and
- the existing VS Code extension validation.

The broad `src/lib/application` suite remains non-authoritative for this slice because its unrelated
`TestRunWorkflowStepsModeCliWorkdirOverridesInheritedEnvMap` fixture names
`/path/to/your/project`; the focused affected application suite passes. The frozen legacy lane,
emitted artifacts, generated stores, dependency files, and protected ignored bytes did not change.

### Completed step 6 typed HIR/Core/Go checkpoint (2026-08-16)

The separately authorized step-6 implementation checkpoint adds three cohesive generic compiler
packages: target-independent typed HIR, normalized target-independent Core IR, and the first Go
backend. `LowerSemanticMethodToHIR` accepts only a successful `v0.1.0` semantic module analysis and
one public method semantic identity. The resulting HIR keeps the owning module and analysis-local
class symbol, stable owner and callable semantic identities, resolved types, parameter bindings,
and durable declaration/type/expression spans. No target spelling or Go representation enters HIR.

`LowerHIRToCore` removes source spans and analysis-local symbols while preserving the callable
identity, ordered typed signature, parameter positions, typed literals/references, and normalized
existing unary/binary operators. The Go backend imports Core IR only, validates the fixed language
and compiler contracts, orders functions deterministically, and emits formatted dependency-free Go
for the primitive capability slice. It rejects missing identities, duplicate target names, named or
applied types, division, and logical operators rather than silently choosing Go behavior before
step 7 fixes the relevant failure and evaluation-order semantics.

The first vertical fixture uses only existing expression-bodied method syntax:
`Ready(int count) => count > 0`. HIR, Core, and generated-Go goldens prove the representation
boundaries. The generated Go compiles and executes only under `/tmp`; its result matches the frozen
pure evaluator for the same source and input. A source-architecture test rejects any Go-backend
import of the PipeLang parser, AST/compiler root, HIR, `go/parser`, or `go/ast`, so a direct
parser/AST-to-Go path fails the focused suite. HIR and Core lowering failures remain ordered
PipeLang diagnostics with durable spans and semantic identities; unsupported Go capabilities use
stable backend errors.

Terminal proof passed with the cached Go 1.25.13 toolchain, the required offline environment, and
writable `/tmp` caches:

- focused HIR/Core/backend/diagnostic goldens plus exact
  `go test ./src/lib/pipelang/... ./tests/pipelangcompat`;
- compile and execution of generated Go only in a temporary `/tmp` module, with the same result as
  the existing pure evaluator;
- affected application PipeLang compile/check/invoke, catalog, materialize, workflow/package compile,
  and CLI package checks using only a temporary `/tmp` modfile pinned to the already-cached
  `golang.org/x/sys v0.46.0` source; checkout dependencies did not change; and
- `go vet` across PipeLang, compatibility, and the affected application/internal/CLI packages.

The exact 45-source inventory and its source, language-projection, artifact, and evaluation goldens
did not drift. The semantic projection tests pass unchanged. This checkpoint changes no production
syntax, CLI/YAML/schema/editor surface, generated store, runtime, or external state.


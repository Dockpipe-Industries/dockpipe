# TASK-020 Declarative Application Surfaces And Standalone Target Builders

## Goal

Define a durable path from DockPipe's existing typed workflow/catalog metadata to declarative
application surfaces and standalone build artifacts without adding framework-specific behavior to
the engine.

This task tracks three related but distinct layers:

1. **DockPipe Site Compiler**: the user-facing product capability for authoring and producing sites
   and application surfaces.
2. **PipeLang managed language**: the shared target-neutral semantic/Core foundation plus typed
   models, reactive state, computed values, actions, governed effects, validation, and binding
   expressions owned by
   [TASK-021](pipelang-reactive-application-language.md).
3. **Generic DockPipe primitive**: a versioned normalized application/view contract plus
   target-agnostic building and artifact-manifest contracts.

The accepted working product direction is that Qt becomes DockPipe's standard first-party
application framework without becoming an engine dependency or the generic contract. PipeLang and
YAML describe the application; a normalized Application IR specializes TASK-021's versioned
semantic/Core projection and separates authored semantics from targets; package-owned adapters
produce Qt native, Qt WebAssembly, semantic web, or future outputs.
Qt Quick/QML plus generated C++ is the preferred standard-backend direction to validate because it
is declarative, reactive, responsive, and shared across native and WebAssembly. The current Qt
Widgets proof remains valid consumer evidence but does not by itself select Widgets as the final
standard renderer.

This is a backlog/design record only. It does not authorize implementation, public YAML/schema or
CLI changes, launcher rewrites, generated-state refreshes, or target-toolchain installation.

## Proven Consumer Evidence

The read-only consumer proof in `/home/jamie/source/dockpipe-cloud` demonstrates the direction using
current DockPipe behavior and no main-repository changes:

| Evidence | What it proves |
| --- | --- |
| `workflows/site.compile/config.yml` | Normal PipeLang `types:` and workflow `view:` metadata are normalized through `dockpipe catalog list --format json`. |
| `site/qt/README.md` | One embedded normalized catalog drives the same Qt Widgets source as a native application and a browser-running Qt WebAssembly application, with no DockPipe invocation after compilation. |
| `scripts/toolchains/README.md` | The proof uses a repository-local Qt 6.8.3 host/WASM kit and Emscripten 3.1.56 rather than a mutable implicit toolchain. |
| `docs/proposals/standalone-target-compilation.md` | A target-neutral input/output and artifact-manifest direction is already described from the consumer side. |
| `docs/agents/tasks/unified-qt-surface.md` | Desktop rendering, interactive tab navigation, and a 390x844 mobile reflow were browser-tested. |

The same contract also produces a distinct semantic HTML/CSS export. That export and the Qt
WebAssembly canvas application are separate target results, not interchangeable presentation
implementations.

## Existing Ownership Boundary

DockPipe already owns the normalized launcher/tooling contract:

- `src/lib/application/catalog_cmd.go` exposes `dockpipe catalog list --format json` and explicitly
  requires launchers to consume that contract instead of scanning repository/package trees.
- `src/lib/domain/workflow.go` owns the current authored `types:` and `view:` fields.
- `src/app/tooling/dockpipe-launcher/src/WorkflowCatalog.cpp` invokes the CLI catalog command and
  parses its JSON projection.
- `src/app/tooling/dockpipe-launcher/src/WorkflowCatalog.h` defines only launcher-side projection
  types; it is not a second workflow model or parser.

The Qt launcher therefore sits above the DockPipe CLI/catalog contract. It must continue consuming
a DockPipe-normalized model and must not become an independent YAML parser or workflow engine.
Moving hardcoded launcher screens to that model should be incremental, not a launcher rewrite.

## Architecture Contract

Preserve the normative model in `docs/concepts/architecture-model.md`:

| Concern | Owner |
| --- | --- |
| Application contents, behavior, and target intent | Workflow/PipeLang/YAML: **what** is described or built. |
| Compilation or execution environment | Runtime: **where** work happens. |
| Framework/toolchain implementation | Resolver adapter: **which** framework/toolchain performs the work; it never defines the runtime. |
| Templates, compilers, source files, and framework tooling | Package-owned assets/scripts. |
| Optional before/after lifecycle | Strategy. |
| Build result, entrypoints, hashes, provenance, and runtime needs | Versioned target artifact manifest. |

Qt, websites, CMake, Emscripten, SCSS, HTML, QML, and C++ must not become engine special cases.
Framework adapters may ship as resolver packages when they consume the same normalized application
contract. Illustrative adapter identities include `static-html`, `qt-wasm`, `qt-native`, `astro`,
and `vite`; these names do not establish a shipped catalog.

Publishing is a separate operation. Cloudflare Pages, S3, GitHub Pages, or another destination
belongs to separately selected resolver/workflow behavior, not to ordinary target building.

## Compiler And Product Direction

The intended compiler layering is:

```text
.pipe source -> TASK-021 syntax/binding/typed HIR/Core IR
                              |
                              v
             versioned semantic/Core projection
                              +
          YAML application composition and bindings
                              +
           portable theme rules and ordinary assets
                              |
                              v
                  DockPipe Application IR
                              |
                    +---------+----------+
                    |                    |
                    v                    v
             Qt Quick/QML + C++      semantic HTML/CSS/JS
                    |                    |
                    v                    v
             Qt native / Qt WASM     static or interactive web
```

The useful portability boundary is the Application IR, not HTML, C++, LLVM, or WebAssembly. HTML
must not be treated as an intermediate representation for Qt, and Qt types must not leak into the
generic IR. Both Qt and web adapters consume the same validated semantic component tree, typed
bindings, actions, accessibility metadata, layout constraints, theme tokens, and target capability
requirements.

The analogy is intentionally bounded: PipeLang is to the generated Qt/C++ application roughly what
a typed declarative application language is to its framework implementation. It is not merely
alternate C++ syntax and must not grow pointers, manual memory, threads, or unrestricted host APIs
to make the analogy literal.

## Default Authoring Experience

Ordinary users author only:

- `.pipe` models, state, validation, computed values, actions, and governed effect declarations;
- YAML application structure, components, bindings, actions, navigation, and target intent;
- portable SCSS/theme rules from a documented supported subset; and
- assets such as images, fonts, Markdown, and localization resources.

Ordinary users do **not** author HTML, JavaScript, QML, C++, CMake, or a WebAssembly loader. Those are
generated target artifacts. Semantic HTML remains an important separate output for accessibility,
SEO, document structure, and lightweight delivery; Qt WebAssembly renders through its Qt surface
and does not replace the semantic-web target.

An illustrative source tree is:

```text
workflows/product.site/
|- config.yml
|- models/
|  `- ProductSite.pipe
|- styles/
|  |- _tokens.scss
|  `- site.scss
`- assets/
   |- logo.svg
   `- terminal.webp
```

Generated Qt and web trees belong in artifacts, not authored source:

```text
dist/
|- qt-native/    # generated QML/C++/CMake plus native executable
|- qt-wasm/      # generated QML/C++ build plus wasm/js/bootstrap HTML
`- web/          # generated semantic HTML/CSS and bounded JS when required
```

## Declarative Application Direction

Extend the existing typed `types:` plus `view:` direction incrementally so a future versioned model
can represent:

- pages, tabs, panels, sections, and layouts
- typed fields and data bindings
- buttons and workflow/package actions
- tables, logs, artifacts, run status, and approval surfaces
- visibility and enabled conditions
- responsive layout hints
- reusable component identities
- shared theme and design tokens

Do not accept field names or YAML shapes in this backlog item. Any authored-surface proposal must be
reviewed and then update authored schema, generated schema, language support, catalog JSON, CLI help,
canonical docs, and tests together.

## PipeLang Dependency

[TASK-021](pipelang-reactive-application-language.md) owns the detailed PipeLang language gap,
versioning, type-system evolution, reactive state, pure computed properties, local actions, governed
effects, safe expressions, parser/typechecker/compiler work, compatibility, diagnostics, and editor
support. It is the first implementation dependency for this task.

TASK-020 consumes only TASK-021's accepted versioned semantic projection and referenced Core units.
Application IR is a specialized, independently versioned projection over that foundation, not an
internal compiler representation or parallel language model. Target adapters must not define
missing language semantics, parse `.pipe` independently, or land permissive syntax on behalf of a
renderer. This task retains ownership of portable application components, YAML composition, layout,
accessibility, styling, Application IR integration, and target artifacts.

## Application IR Contract

The versioned IR should normalize at least:

- typed model and state identities;
- semantic pages, component trees, navigation, and reusable component identities;
- property bindings, pure expressions, events, typed actions, and governed effects;
- lists, conditions, selection, validation, loading, error, and empty states;
- layout constraints, responsive breakpoints, shared theme tokens, and supported style rules;
- accessibility roles, labels, relationships, keyboard intent, and focus order;
- localization/resource references and target capability requirements; and
- source locations sufficient for diagnostics without requiring a backend to parse PipeLang or YAML.

Backends consume only this IR and selected package-owned assets. They do not independently parse the
repository, authored YAML, or PipeLang. Qt, web, Go bridge, and other target behavior is selected and
implemented by resolver-owned adapters. Conformance is semantic rather than pixel-identical:
targets must preserve state, action, validation, accessibility, and layout intent while using
native target primitives; missing required capability fails instead of degrading.

### Presentation escape hatches

Use a deliberate authoring ladder:

1. PipeLang, YAML, portable SCSS, and ordinary assets for the target-independent path.
2. QML or Qt stylesheet assets for advanced Qt presentation.
3. HTML plus SCSS assets for advanced direct-DOM web presentation.
4. User-authored C++ files for custom Qt components, native integration, or performance-sensitive
   behavior.

YAML should reference files and stable component identities rather than embed large HTML, SCSS,
QML, or C++ bodies. Every target-specific escape hatch declares its target and portability loss.
HTML is therefore available when needed, but it is not part of the normal authoring path.

## Standard Qt Backend

Qt is the standard first-party application backend to validate, not a generic engine primitive. The
preferred direction is:

- generate Qt Quick/QML for declarative components, responsive layout, animation, and bindings;
- generate typed C++ models, action dispatch, capability bridges, and performance-sensitive logic;
- generate CMake and resource manifests as backend build details;
- compile the same generated application for native and Qt WebAssembly profiles; and
- retain explicit QML and C++ custom-component escape hatches.

The Qt adapter must never define workflow execution semantics, parse authored YAML independently, or
become the owner of PipeLang types. The existing Qt Widgets consumer remains a useful compatibility
and feasibility fixture. A future decision may retain a Widgets adapter, but the standard framework
selection should compare it with Qt Quick against responsiveness, accessibility, desktop behavior,
WASM behavior, generated-code complexity, binary/download size, and licensing constraints.

## Semantic Web Backend

The semantic-web adapter consumes the same Application IR and generates HTML, CSS, and only the
bounded JavaScript required by declared interactions. It must preserve semantic document structure,
accessibility relationships, routing intent, and progressive behavior. Generated HTML is an output,
not user-owned source and not input to the Qt backend.

The Qt WebAssembly and semantic-web outputs are intentionally distinct:

| Output | Primary value |
| --- | --- |
| Qt WebAssembly | the same Qt application and rendering/runtime behavior in a browser |
| Semantic web | DOM semantics, accessibility, SEO, lightweight delivery, and progressive enhancement |

## Generic Target-Building Contract

A future generic primitive should:

- accept a selected workflow/package plus its normalized, versioned Application IR
- select one explicit target adapter
- use the existing scope and artifact model
- produce an inspectable, deterministic target manifest
- report entrypoints, files, content hashes, toolchain provenance, and runtime dependencies
- distinguish self-contained outputs from results requiring an external runtime
- fail explicitly when a required toolchain is absent or incompatible
- keep toolchain installation/fetching separate from ordinary builds unless explicitly requested

Do not settle command spelling here. `dockpipe compile` already means DockPipe package compilation.
`dockpipe target build ...` is an illustrative possibility only, not accepted syntax.

## Related Ownership

- [TASK-021](pipelang-reactive-application-language.md) owns the PipeLang language foundation and is
  the first implementation dependency for this task.
- [TASK-022](go-first-pipelang-backend-services.md) owns transport-neutral backend services, the
  Go-first service resolver, schemas, and generated Qt/C++ clients. This task owns consuming those
  clients in application surfaces, not backend or transport semantics.
- [TASK-008](agentic-app-ui.md) owns ForgePipe's launcher-context control and inspection UX. It may
  consume the application contract but does not own the generic target builder.
- [TASK-004](compile-performance-qt-vscode.md) owns measured Qt launcher and VS Code extension build
  performance, not target semantics or standalone artifact contracts.
- [TASK-010](declarative-dependency-install-ux.md) owns governed host-tool dependency and install UX.
  Target adapters declare toolchain needs; ordinary builds must not silently install them.
- [TASK-017](portable-embedded-execution-target.md) owns a portable embedded instruction/runtime
  target. It is adjacent evidence for target manifests and adapters, not the owner of application
  rendering or Site Compiler behavior.
- `/home/jamie/source/dockpipe-cloud/docs/agents/tasks/unified-qt-surface.md` and its sibling proposal
  remain consumer evidence. They do not define main-repository contracts.

## Implementation Sequence

When prioritized:

1. Admit TASK-021's accepted language decision packet and versioned semantic/Core fixtures.
2. Freeze the current normalized `types:` plus `view:` catalog projection as a fixture baseline.
3. Define and version the target-neutral Application IR with source-mapped diagnostics, consuming
   the TASK-021 projection rather than reparsing PipeLang.
4. Define the portable component, binding, action, effect, layout, accessibility, and styling model.
5. Define a generic target artifact manifest.
6. Build fixture-only `static-html` and Qt projections from the same IR; compare Qt Quick with the
   existing Widgets proof before fixing the standard Qt implementation.
7. Implement TASK-021's minimum accepted PipeLang/compiler vertical slice before production target
   adapter implementation.
8. Refactor `static-html`, `qt-native`, and `qt-wasm` behind interchangeable package-owned target
   adapters with deterministic artifact manifests.
9. Move one bounded launcher screen from hardcoded C++ to the normalized model without rewriting the
   launcher.
10. Preserve explicit HTML/SCSS, QML, Qt stylesheet, and C++ escape hatches with declared
    portability loss.
11. Return to `dockpipe-cloud` and migrate its proof onto the released contract.

## Acceptance Criteria

- The record preserves the complete consumer proof without treating consumer paths as core truth.
- Site Compiler product language stays distinct from the generic core primitive.
- Qt is the standard first-party application framework while remaining package-owned and absent from
  generic engine control flow.
- PipeLang owns target-neutral types, state, computed values, actions, and governed effects rather
  than becoming alternate C++ syntax.
- Ordinary authoring requires no HTML, JavaScript, QML, C++, CMake, or WASM-loader maintenance.
- The Application IR is versioned, source-mapped, target-neutral, and the only semantic input to
  target adapters.
- Workflow/runtime/resolver/strategy invariants remain explicit.
- HTML, SCSS, QML, C++, templates, compilers, and framework tooling stay target-package assets.
- No unreviewed YAML shape or CLI syntax is accepted by implication.
- Launcher migration is incremental and continues through the normalized catalog contract.
- At least two adapters, or another convincing generic fixture, prove target independence.
- `static-html` and the selected Qt implementation preserve the same typed bindings, actions,
  validation, accessibility, responsive intent, and governed effects from one IR fixture.
- Portable SCSS is a documented fail-closed subset; arbitrary browser CSS is never silently mapped
  into Qt behavior.
- Target manifests are deterministic and include content hashes and toolchain provenance.
- Missing or incompatible toolchains fail explicitly without implicit installation.
- Licensing, accessibility, download size, browser support, signing, publishing, and desktop
  bundling remain separate product/release decisions.
- Package/engine boundaries remain intact; no consumer or framework name enters generic control flow.

## Next Bounded Design Slice

Write one fixture-only compiler-design proposal that freezes the current normalized `types:` plus
`view:` catalog projection as the baseline, consumes TASK-021's versioned typed-projection fixture,
and provides:

1. a versioned Application IR fixture with semantic components, bindings, actions, layout,
   accessibility, theme tokens, source locations, and target capabilities;
2. a portable-SCSS support table showing accepted, target-mapped, and rejected constructs;
3. `static-html`, `qt-quick-native`, and `qt-quick-wasm` fixture projections consuming the same IR,
   plus the retained Widgets proof as comparison evidence; and
4. deterministic target manifests declaring files, entrypoints, hashes, toolchain provenance,
   self-contained/runtime-dependency status, portability escapes, and failure cases.

That slice is documentation and fixtures only. It must not change public YAML, schema, catalog
output, CLI spelling, PipeLang parser/typechecker, launcher code, resolver packages, or generated
stores.

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

- [TASK-021](../pipelang-reactive-application-language/overview.md) owns the PipeLang language foundation and is
  the first implementation dependency for this task.
- [TASK-022](../go-first-pipelang-backend-services/overview.md) owns transport-neutral backend services, the
  Go-first service resolver, schemas, and generated Qt/C++ clients. This task owns consuming those
  clients in application surfaces, not backend or transport semantics.
- [TASK-008](../agentic-app-ui/overview.md) owns ForgePipe's launcher-context control and inspection UX. It may
  consume the application contract but does not own the generic target builder.
- [TASK-004](../compile-performance-qt-vscode/overview.md) owns measured Qt launcher and VS Code extension build
  performance, not target semantics or standalone artifact contracts.
- [TASK-010](../declarative-dependency-install-ux/overview.md) owns governed host-tool dependency and install UX.
  Target adapters declare toolchain needs; ordinary builds must not silently install them.
- [TASK-017](../portable-embedded-execution-target/overview.md) owns a portable embedded instruction/runtime
  target. It is adjacent evidence for target manifests and adapters, not the owner of application
  rendering or Site Compiler behavior.
- `/home/jamie/source/dockpipe-cloud/docs/agents/tasks/unified-qt-surface.md` and its sibling proposal
  remain consumer evidence. They do not define main-repository contracts.

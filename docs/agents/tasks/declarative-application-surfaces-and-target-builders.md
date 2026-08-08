# TASK-020 Declarative Application Surfaces And Standalone Target Builders

## Goal

Define a durable path from DockPipe's existing typed workflow/catalog metadata to declarative
application surfaces and standalone build artifacts without adding framework-specific behavior to
the engine.

This task tracks two related but distinct layers:

1. **DockPipe Site Compiler**: the user-facing product capability for authoring and producing sites
   and application surfaces.
2. **Generic DockPipe primitive**: a versioned normalized application/view contract plus
   target-agnostic building and artifact-manifest contracts.

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

### Presentation escape hatches

Use a deliberate authoring ladder:

1. YAML for ordinary application structure and behavior.
2. HTML plus SCSS assets for direct DOM-based web presentation.
3. QML or Qt stylesheet assets for advanced Qt presentation.
4. User-authored C++ files for custom Qt components, native integration, or performance-sensitive
   behavior.

YAML should reference files and stable component identities rather than embed large HTML, SCSS,
QML, or C++ bodies. SCSS cannot directly style widgets rendered inside a Qt WebAssembly canvas;
shared design tokens may instead generate SCSS/CSS values for DOM targets and QML/QSS values for Qt
targets.

## Generic Target-Building Contract

A future generic primitive should:

- accept a selected workflow/package plus its normalized application/catalog contract
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

1. Define and version the normalized declarative application/view contract.
2. Define a generic component and action-binding model.
3. Define a generic target artifact manifest.
4. Refactor at least two consumer renderers, initially `static-html` and `qt-wasm`, behind
   interchangeable target adapters.
5. Move one bounded launcher screen from hardcoded C++ to the YAML-backed normalized model.
6. Preserve QML and C++ custom-component escape hatches.
7. Update YAML schema, generated schema, language support, catalog JSON, CLI help, canonical docs,
   and tests together.
8. Return to `dockpipe-cloud` and migrate its proof onto the released core contract.

## Acceptance Criteria

- The record preserves the complete consumer proof without treating consumer paths as core truth.
- Site Compiler product language stays distinct from the generic core primitive.
- Workflow/runtime/resolver/strategy invariants remain explicit.
- HTML, SCSS, QML, C++, templates, compilers, and framework tooling stay target-package assets.
- No unreviewed YAML shape or CLI syntax is accepted by implication.
- Launcher migration is incremental and continues through the normalized catalog contract.
- At least two adapters, or another convincing generic fixture, prove target independence.
- Target manifests are deterministic and include content hashes and toolchain provenance.
- Missing or incompatible toolchains fail explicitly without implicit installation.
- Licensing, accessibility, download size, browser support, signing, publishing, and desktop
  bundling remain separate product/release decisions.
- Package/engine boundaries remain intact; no consumer or framework name enters generic control flow.

## Next Bounded Design Slice

Write one fixture-only design proposal that freezes the current normalized `types:` plus `view:`
catalog projection as the baseline, identifies the minimum missing generic component/action concepts,
and defines a versioned target artifact manifest. Include `static-html` and `qt-wasm` fixture
projections consuming the same normalized input and declaring deterministic files, entrypoints,
hashes, toolchain provenance, self-contained/runtime-dependency status, and failure cases.

That slice is documentation and fixtures only. It must not change public YAML, schema, catalog
output, CLI spelling, launcher code, resolver packages, or generated stores.

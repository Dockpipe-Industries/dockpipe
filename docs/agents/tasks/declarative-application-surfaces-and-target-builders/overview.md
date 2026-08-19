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
   [TASK-021](../pipelang-reactive-application-language/overview.md).
3. **Generic DockPipe primitive**: a versioned normalized application/view contract plus
   target-agnostic building and artifact-manifest contracts.

The accepted working product direction is that Qt becomes DockPipe's standard first-party
application framework without becoming an engine dependency or the generic contract. PipeLang and
YAML remain interoperable: the future PipeLang application surface owns typed application/view
semantics, while YAML owns workflow composition, target intent, and current compatibility inputs. A
normalized Application IR specializes TASK-021's versioned semantic/Core projection and separates
authored semantics from targets; package-owned adapters produce Qt native, Qt WebAssembly, semantic
web, or future outputs.
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
Replacing the launcher implementation with generated PipeLang/Application IR output must be
incremental and parity-gated rather than a flag-day rewrite.

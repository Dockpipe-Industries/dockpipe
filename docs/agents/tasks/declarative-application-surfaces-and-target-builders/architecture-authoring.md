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

- `.pipe` models, state, validation, computed values, actions, governed effect declarations, and,
  after an explicit future contract, target-neutral declarative views/components/navigation;
- YAML workflow composition, PipeLang entrypoint and binding references, runtime/resolver selection,
  target intent, and compatibility `view:` metadata;
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

Freeze the existing typed `types:` plus YAML `view:` direction as the compatibility baseline. Evolve
the future versioned Application IR and explicitly accepted PipeLang view surface incrementally so
they can represent:

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
canonical docs, and tests together. No current YAML `view:` consumer migrates implicitly, and future
PipeLang view syntax must be accepted through TASK-021 before this task consumes it.

## PipeLang Dependency

[TASK-021](../pipelang-reactive-application-language/overview.md) owns the detailed PipeLang language gap,
versioning, type-system evolution, reactive state, pure computed properties, local actions, governed
effects, safe expressions, parser/typechecker/compiler work, compatibility, diagnostics, and editor
support. It is the first implementation dependency for this task.

TASK-020 consumes only TASK-021's accepted versioned semantic projection and referenced Core units.
Application IR is a specialized, independently versioned projection over that foundation, not an
internal compiler representation or parallel language model. Target adapters must not define
missing language semantics, parse `.pipe` independently, or land permissive syntax on behalf of a
renderer. This task retains ownership of portable application components, YAML composition, layout,
accessibility, styling, Application IR integration, and target artifacts.

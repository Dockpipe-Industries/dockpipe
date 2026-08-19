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
9. Move the read-only Docker observability screen through the normalized model as the first launcher
   parity slice, then advance through the accepted launcher parity order above.
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
- Launcher migration is incremental, one-to-one parity-gated, and continues through normalized
  catalog, Application IR, capability, and operation-result contracts.
- The local native launcher remains usable without a browser, PWA, remote account, or network
  dependency.
- Generated UI code never embeds raw Docker, process, Git, filesystem, VM, package, or workflow
  execution; selected DockPipe capabilities retain those authorities.
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

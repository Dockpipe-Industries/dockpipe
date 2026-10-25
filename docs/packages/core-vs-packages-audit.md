# Core and package boundaries

Dockpipe owns the generic engine and CLI in `src/lib/` and `src/cmd/`, the GUI
shell and host SDK in `src/dockpipe-app/`, and its workflow/configuration editor
extension. Fixed runtime and SDK resources live in `src/lib/runtimeassets/` and
`src/lib/sdkassets/`. They materialize only when needed; there is no required
core archive or embedded repository workflow tree.

Tool-specific package implementations, examples and tests live in their
[owning repositories](core-tools.md). PipeLang owns the compiler, language SDK,
language extension and research. Dockpipe consumes its pinned external Go SDK
for typed models, compiler commands and automatic materialization, without a
sibling-source fallback.

The project store under `bin/.dockpipe/internal/packages/` contains generated
workflow/resolver archives. Its name does not imply ownership of package sources.
Package installation and generic compilation remain engine features. A host
release can contain no optional packages; selected package archives come from
explicit immutable manifests and digests.

[Package metadata](package-model.md) defines the install/compile contract.
[Runtime resource ownership](../concepts/runtime-resource-ownership.md) describes
initialization and fixed library helpers. Earlier migration inventories record
historical monorepo paths; see the
[historical path note](../architecture/forgepipe/HISTORICAL-PATHS.md).

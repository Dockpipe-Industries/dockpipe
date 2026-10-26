# Engine Boundary

Read before editing `src/lib/` or `src/cmd/`.

## Boundary

`src/lib/` and `src/cmd/` are the engine. They must not know this checkout's package/workflow/staging layout.

## Allowed In The Engine

- generic workflow resolution
- generic resolver/runtime/profile loading
- generic script path resolution
- package manifest and install wire shapes
- project-local package store paths through infrastructure helpers
- global paths through global helper functions
- fixed generic runtime/SDK resources through their library-owned materializers; no authored workflow embedding

## Forbidden In The Engine

- hardcoded repo-relative `packages/<name>/...`
- hardcoded repo-root `workflows/<name>/...`
- any `.staging/...` dependency or user-facing prescription
- maintainer-only workflow/resolver names in control flow, tests, or user-facing strings
- product-specific logic for Codex, Claude, DorkPipe, Pipeon, or a package workflow
- "the way to do X is workflow foo under packages/..." baked into engine behavior

## Package/Workflow Interaction

Package source belongs to its [owning repository](repo-map.md#core-paths), including separate DorkPipe and Pipeon repositories. PipeLang belongs to its [external SDK repository](https://github.com/Dockpipe-Industries/pipelang). Consume pinned published artifacts; do not introduce sibling source paths or copied implementations. Packages interact with the engine through:

- compile into `bin/.dockpipe/internal/packages/`
- package/store tarballs
- library-owned SDK interfaces and on-demand generic resources
- declarative YAML fields already supported by the runner
- public CLI behavior

Existing compatibility behavior is not automatic removal authority; follow the
[compatibility retirement rules](compatibility-retirement.md) before retiring it.

## If You Need More

If a workflow/package cannot express something, propose a general primitive. Do not add a special case.

## Checks

- `rg -n "packages/|\\.staging/|workflows/" src/lib src/cmd`
- Use the [validation workflow route](validation-commands.md#core); start with affected packages/tests and broaden when required by the change.
- update docs/schema/language support for authored surface changes

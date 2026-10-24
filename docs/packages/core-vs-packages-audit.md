# Core and package boundaries

Dockpipe owns `src/lib/`, `src/cmd/`, the desktop host and required `src/core/`
definitions. Its authored embed manifest includes `src/core/`, this repository's
own `workflows/`, `VERSION` and the entrypoint asset. It excludes every external
`packages/` source tree. `embed_test.go` independently verifies this boundary.

All package implementations and their examples live in
[dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages).
PipeLang source and language tests live in
[PipeLang](https://github.com/Dockpipe-Industries/pipelang); the runtime consumes
its pinned SDK for typed models and existing compiler commands. There is no
sibling checkout or copied source fallback.

The compiled project store under `bin/.dockpipe/internal/packages/` is generated
runtime state. Its name does not imply that this checkout owns package sources.
Package installation and generic compilation remain engine features. Required
core archives and optional package archives have distinct ownership and release
inputs; optional archives are composed from exact external pins.

[Package metadata](package-model.md) defines the install/compile contract.
[Ecosystem ownership](core-tools.md) links the source repositories and examples.
Earlier migration inventories describe historical monorepo paths; see the
[historical path note](../architecture/forgepipe/HISTORICAL-PATHS.md).

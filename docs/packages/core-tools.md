# Dockpipe and ecosystem source ownership

| Repository | Owns | Examples and guidance |
| --- | --- | --- |
| [Dockpipe](https://github.com/Dockpipe-Industries/dockpipe) | Generic engine, CLI, GUI shell, host SDK and Dockpipe Language Support extension | [Workflow authoring](../workflows/workflow-authoring.md), [package model](package-model.md) |
| [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages) | Catalog packages such as VM, IDE and secret integrations, with their tests and producer workflows | [Package examples](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages), [contributor guidance](https://github.com/Dockpipe-Industries/dockpipe-packages/blob/master/AGENTS.md) |
| [DorkPipe](https://github.com/Dockpipe-Industries/dorkpipe) | DorkPipe orchestration, MCP and companion editor integration | [DorkPipe package source](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages) |
| [Pipeon](https://github.com/Dockpipe-Industries/pipeon) | Pipeon package and host apps; source access required | [Pipeon package source](https://github.com/Dockpipe-Industries/pipeon/tree/master/packages/pipeon) |
| [PipeLang](https://github.com/Dockpipe-Industries/pipelang) | Compiler, application IR, language SDK, standalone CLI, PipeLang editor extension, tests and research | [Language documentation](https://github.com/Dockpipe-Industries/pipelang/tree/master/docs), [contributor guidance](https://github.com/Dockpipe-Industries/pipelang/blob/master/AGENTS.md) |

Dockpipe consumes the versioned external PipeLang SDK for typed models,
`dockpipe pipelang` and automatic `.pipe` materialization. This preserves the
existing public behavior without owning language implementation source. PipeLang
also builds and runs independently. Other language integrations use workflows,
packages and public SDK contracts without product-specific engine branches.

Build and test the runtime from this checkout. Build and test a package from its
own repository with a pinned Dockpipe binary and required SDKs. Workflow-specific
scripts belong with their workflow; helpers shared by several workflows belong
at their common owning boundary. Release composition uses immutable package
archives with exact manifests and digests, rather than sibling checkout source.

Existing pinned releases remain reproducible; moving current source does not
rewrite old release artifacts. Source migration alone does not publish new
packages or qualify a distribution channel. See [package usage](package-quickstart.md).

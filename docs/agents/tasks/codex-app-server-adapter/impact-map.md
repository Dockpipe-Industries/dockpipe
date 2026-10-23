## Likely impact map

Repository ownership note: package paths in this record describe external source. Current catalog packages live in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages); previously extracted [DorkPipe/MCP](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages) and [Pipeon](https://github.com/Dockpipe-Industries/pipeon/tree/master/packages/pipeon) remain in their own repositories. Historical paths and receipts do not recreate a local package tree.

- packages/dorkpipe/lib: provider-neutral contracts, adapter package, state and tests;
- packages/dorkpipe-mcp/mcpbridge: normalized host session/approval operations;
- packages/dorkpipe/lib/cmd/dorkpipe/provider_pool.go: adapter selection retaining exec;
- packages/dorkpipe/resolvers/dorkpipe/assets/provider-pools/catalog.yml: capability policy;
- Pipeon extension: normalized session/event UI;
- DorkPipe/Pipeon tests and docs.

Do not modify those production areas for this research task.

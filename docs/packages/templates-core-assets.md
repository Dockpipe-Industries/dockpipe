# Runtime and package assets

Generic runtime resources and SDK helpers belong to the library. Workflow and
resolver assets belong to their owning project or package. The former shared core
authoring tree and mandatory archive are retired.

See [runtime resource ownership](../concepts/runtime-resource-ownership.md) for the
current source, materialization and compatibility contract, and the
[package model](package-model.md) for installation and resolution.

Tool-specific assets live in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages).
Workflow-specific scripts and tests stay beside their `config.yml`; reusable
helpers stay at the lowest common owning boundary. Packages consume released SDKs
and explicit tool pins rather than sibling source checkouts.

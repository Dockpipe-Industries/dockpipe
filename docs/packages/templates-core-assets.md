# Bundled core assets

Required shared support files live under `src/core/assets/`: scripts, image recipes
and Compose examples. They support the existing runtime/resolver/strategy model;
they are not additional engine primitives. See [architecture](../concepts/architecture-model.md).

| Source | Ownership |
| --- | --- |
| `src/core/assets/scripts/` | Generic Dockpipe shared helpers |
| `src/core/assets/images/` | Generic bundled image recipes |
| `src/core/assets/compose/` | Optional generic Compose examples |
| [External package assets](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages) | Product/provider-specific support and tests |

Terraform helpers live in the [Terraform package](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/terraform/resolvers/terraform-core).
DorkPipe scripts live in [DorkPipe](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages/dorkpipe).
Do not copy either into a root script shadow or make shared assets depend on their
source checkouts. Package consumers use compiled/installed artifacts and public
SDK helpers.

Workflow-specific scripts belong alongside the workflow's `config.yml`. A helper
shared by several workflows may sit at their common owning boundary. Keep
credentials and operator configuration outside bundled source; reusable assets
must accept operator-supplied targets. See [package model](package-model.md).

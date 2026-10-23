# Package Authoring

Read when authoring external package workflows, resolvers, assets or manifests. Catalog package source lives in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/); do package work in that repository, not this runtime checkout. Previously extracted DorkPipe/MCP and Pipeon remain in their dedicated repositories; see [ownership](../../packages/core-tools.md).

## Package Boundary

Packages are self-contained YAML plus assets plus resolver/runtime wiring. They cannot inject engine primitives without a separate core change.

## Hard Rules

- Keep package logic inside the package tree.
- Do not make maintainer/dev flows depend silently on whatever happens to be on `PATH`.
- Prefer real repo-local build outputs first, then fall back to `PATH`.
- Consume shared SDK helpers from the pinned Dockpipe SDK instead of reaching into a sibling core checkout. Models require the separately pinned PipeLang compiler/SDK.
- In package workflow YAML, reference local package files by the path the author sees, such as
  `assets/scripts/run.sh` or `../assets/scripts/run.sh`. Use `scripts/<name>/...` only when
  intentionally calling a shared/cross-package script namespace.
- For workflow scripts, prefer direct `dockpipe scope ...` calls for source/artifact/package/resolver paths.
- For reusable shell libraries, prefer `eval "$(dockpipe sdk)"` and `dockpipe_sdk ...` actions.
- Keep resolver-owned auth/config defaults in resolver profiles and read them with `dockpipe scope resolver <name> <field>`.
- Give every new resolver without a `profile` an authoritative `package.yml` with `kind: resolver`; `config.yml` alone describes a workflow, even under `resolvers/`.
- Choose state ownership explicitly: `dockpipe scope --package <owner-id>` is durable and owner-only; use `PackageRuntimeDir` or shell SDK `path package-runtime <owner-id>` for disposable caches, build output, scratch, and run evidence.
- A maintained package with mixed legacy public state declares `package_state.compatibility_import: package-owned` plus exact `owner_ids`, then owns exhaustive cohort selection and migration. Dockpipe propagates the selected manifest to package/workflow script context; never replace that declaration with engine package-name knowledge or classify an unknown cohort by guesswork.
- Never link durable state back into `bin/.dockpipe`; compatibility import is validated copy-and-publish and leaves legacy bytes untouched.
- Keep package tests self-contained.

## Tool selection and checks

Select an absolute `DOCKPIPE_BIN` for the pinned compiler binary and set
`PACKAGE_REPOSITORY` to the owning checkout and `PACKAGE_SOURCE` to the selected
package's absolute source directory. Build product binaries inside that repository;
never infer a sibling Dockpipe checkout. Package workflows must declare required
PipeLang and native SDK artifacts and fail when they are unavailable.

```bash
"$DOCKPIPE_BIN" package test --workdir "$PACKAGE_REPOSITORY" --only <package>
"$DOCKPIPE_BIN" package compile workflows --workdir "$PACKAGE_REPOSITORY" --from "$PACKAGE_SOURCE" --force
"$DOCKPIPE_BIN" package compile resolvers --workdir "$PACKAGE_REPOSITORY" --from "$PACKAGE_SOURCE" --force
```

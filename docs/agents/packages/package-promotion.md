# Package Promotion

Read when promoting package source in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/). This Dockpipe runtime checkout has no package source or `.staging/packages` authoring tree.

## Goal

Promote only source-of-truth package content. Leave experiments, generated state, caches, and local secrets behind.

## Promote

- `package.yml`
- workflow `config.yml` and workflow-local scripts/docs
- resolver `config.yml`, `profile`, assets, README
- package tests
- curated prompts/skills/policies/examples that are meant to ship

## Do Not Promote

- generated `bin/.dockpipe/` or `.dorkpipe/` outputs
- local `.env` or resolved secret material
- caches, logs, temporary outputs
- shell scripts that encode one workflow shape when YAML should carry the contract
- stale staging-only references in docs

## Required Review

- Confirm the selected external package tree compiles with the pinned Dockpipe binary.
- Bind the exact source revision, dependency pins and resulting artifact digests; source migration alone is not distribution qualification.
- Search for old workflow/script names after deletion.
- Verify docs describe package boundaries, not checkout-only paths as a public contract.

## Checks

Use the absolute tool/source selections described in [package authoring](package-authoring.md).

- `rg --files "$PACKAGE_SOURCE"`
- `rg -n "<old-name>|<old-script>" docs src workflows .github -S`
- `"$DOCKPIPE_BIN" package test --workdir "$PACKAGE_REPOSITORY" --only <name>`
- `"$DOCKPIPE_BIN" package compile workflows --workdir "$PACKAGE_REPOSITORY" --from "$PACKAGE_SOURCE" --force`
- `"$DOCKPIPE_BIN" package compile resolvers --workdir "$PACKAGE_REPOSITORY" --from "$PACKAGE_SOURCE" --force`

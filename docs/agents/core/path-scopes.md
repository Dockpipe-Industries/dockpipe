# Path Scopes

Read when moving generated files out of source control, migrating package/workflow artifacts, or
changing `cwd`, `scopes`, `outputs`, or `dockpipe scope` behavior.

## Default Flow

| Need | Use |
| --- | --- |
| Step only writes generated files | `cwd: artifacts`; use plain relative paths in the script. |
| Step must inspect/edit the checkout and write generated files | `cwd: repo` plus `scopes: { source: repo, artifacts: artifacts }`; use `dockpipe scope artifacts ...` for generated files. |
| Step needs checkout path explicitly | `dockpipe scope source ...`. |
| Workflow-run artifacts | `dockpipe scope artifacts ...`; do not write to repo-root `tmp/`, `.dockpipe/`, or package state. |
| Artifacts from another workflow | `dockpipe scope workflow <name> ...`. |
| Package-owned long-lived state | `dockpipe scope --package <name> ...`. |
| Package-owned disposable cache/run/build state | `PackageRuntimeDir` or shell SDK `path package-runtime <owner-id> ...`. |
| Resolver-owned auth/config paths | Declare them in the resolver profile and read with `dockpipe scope resolver <name> <field>`. |

## Package Extensions

DorkPipe’s `scope:...` expansion in agent access/context lists belongs to its
[harness configuration](https://github.com/Dockpipe-Industries/dorkpipe/blob/master/docs/workflows/harness-configuration.md).
Do not implement that package-specific prompt expansion in the generic scope resolver.
Keep normal repository files as plain relative paths.

## Migration Rules

- Prefer `cwd: artifacts` for simple producer steps so relative writes naturally land in workflow artifacts.
- Use `cwd: repo` only when the process must run from the checkout.
- Do not introduce package/workflow-specific env vars for artifact roots when `dockpipe scope` can resolve the path.
- Do not hardcode `bin/.dockpipe/...`, `tmp/...`, package names, workflow names, or resolver auth directories in scripts.
- Keep generated workflow outputs under workflow artifact scope, not package scope.
- Keep credentials, continuity authority, settings, and cumulative records in durable package state.
- Keep caches, build products, scratch, run artifacts, and reproducible evidence in package runtime; `bin/.dockpipe/**` must tolerate complete removal.
- Keep resolver auth/config defaults in resolver profiles, not workflow scripts.

## Surface Sync

When changing scope semantics, update together:

- `src/lib/domain/workflow/workflow.go` and runner behavior; retain the `domain` forwarding API
- `src/lib/application/sdk_scope.go` and affected SDK helpers
- `src/lib/infrastructure/schema/workflow.schema.json`
- `docs/workflows/workflow-yaml.md`
- `docs/packages/editor-extensions.md`
- affected package workflows/resolver profiles/tests

## Checks

- `"$DOCKPIPE_BIN" workflow validate <config.yml>` using an explicitly selected installed or built CLI
- `"$DOCKPIPE_BIN" package test --workdir "$PACKAGE_REPOSITORY" --only <package>`
- `"$DOCKPIPE_BIN" package compile workflows --workdir "$PACKAGE_REPOSITORY" --from "$PACKAGE_SOURCE" --force`
- Route focused scope/runner tests through the [core validation workflow](validation-commands.md#core), including the owning subpackages.

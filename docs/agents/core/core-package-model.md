# Core And Package Model

Read when touching package resolution, compile/install flows, state paths, or binary lookup.

## Stores

| Store | Path | Meaning |
| --- | --- | --- |
| Project-local compiled store | `bin/.dockpipe/internal/packages/` | Workflows, resolvers, core slices built for this project. |
| Durable project/package state | `dockpipe scope --package <owner-id> ...` | Owner-only state that must survive ordinary clean. |
| Disposable package runtime | `PackageRuntimeDir` / shell SDK `path package-runtime` | Caches, build products, scratch, run evidence, and reproducible outputs under `bin/.dockpipe/`. |
| Global install | `GlobalDockpipeDataDir()` / `DOCKPIPE_GLOBAL_ROOT` | User/machine shared package/core installs. |
| Authoring trees | `src/core/`, `workflows/`, `packages/`, legacy `templates/` | Editable source trees. |

## Hard Rules

- In Go, derive project paths from `infrastructure.DockpipeDirRel`, `StateRoot`, `PackagesRoot`, and related helpers.
- In Go, derive global paths from `GlobalDockpipeDataDir` and global helper functions.
- Do not hand-write bare `.dockpipe/internal` paths.
- Use `dockpipe scope artifacts ...` for workflow-run artifacts, `dockpipe scope --package <owner-id> ...` only for durable package state, and `PackageRuntimeDir` / shell SDK `path package-runtime` for disposable package products.
- `DOCKPIPE_PACKAGES_ROOT` selects the compiled store only. It never relocates durable project/package state.
- Ordinary `dockpipe clean` needs no generated-root declaration: `--dry-run` previews and clean removes only the exact checkout `bin/.dockpipe/` tree. It never follows an external `DOCKPIPE_PACKAGES_ROOT`; rebuild owns that separate guarded reset.
- Runtime resolution uses compiled packages and configured roots, not hardcoded checkout paths.
- Published compiled artifacts are the official versioned reference for external consumers.

## `dockpipe init`

- Bare `dockpipe init` creates a minimal project scaffold in the current directory.
- It creates `workflows/`, `README.md`, `dockpipe.config.json`, and `.env.vault.template.example` when missing.
- If no workflows exist yet, it seeds `workflows/example/config.yml`.
- `dockpipe init <name>` creates `workflows/<name>/config.yml` as a minimal workflow.
- `dockpipe init <name> --from <template>` copies a bundled/filesystem workflow tree.
- It does not clone Git repos and does not copy `templates/core/`, `scripts/`, or `images/` by default.

## Canonical Docs

- `docs/packages/package-model.md`
- `docs/packages/core-vs-packages-audit.md`
- `docs/cli-reference.md`
- `docs/packages/templates-core-assets.md`

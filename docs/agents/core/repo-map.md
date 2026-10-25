# Repo Map

Read when orienting in the Dockpipe checkout.

## Core Paths

| Path | Purpose |
| --- | --- |
| `src/lib/` | Dockpipe engine library. Must stay generic. |
| `src/cmd/` | Dockpipe CLI entrypoint. Must stay generic. |
| `src/dockpipe-app/` | Dockpipe GUI shell and host SDK. Feature contexts separate models, use cases, adapters and views; see its `ARCHITECTURE.md`. |
| `src/lib/runtimeassets/` and `src/lib/sdkassets/` | Fixed generic library resources; no authored workflow tree. |
| `workflows/` | Repository build, test, packaging, publication and developer workflows with colocated assets. |
| [Catalog packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/) | Remaining catalog package sources, outside this checkout. |
| [DorkPipe](https://github.com/Dockpipe-Industries/dorkpipe) | Agent harness, orchestration, skills and MCP packages. |
| [Pipeon](https://github.com/Dockpipe-Industries/pipeon) | Pipeon binaries and host application package. |
| [External PipeLang](https://github.com/Dockpipe-Industries/pipelang) | Versioned compiler/language SDK dependency; no local language implementation tree. |
| `bin/.dockpipe/` | Disposable generated project-local runtime, compiled packages, caches, and artifacts. |
| `.dorkpipe/` | Generated DorkPipe handoffs/analysis where present. |
| `docs/` | Human docs and agent routing. |
| `docs/agents/task-index.yaml` | AI entrypoint for the indexed cross-cutting backlog; update it and the linked task files when work materially completes or advances one of its items. |

## Generated State

Use generated artifacts as read-only grounding unless the user asks to refresh them.

| Generated path | Notes |
| --- | --- |
| `bin/.dockpipe/internal/packages/` | Project-local compiled package store. |
| package scope (`dockpipe scope --package <owner-id> ...`) | Durable owner-only project/package state outside the checkout. |
| `bin/.dockpipe/packages-runtime/` | Collision-safe disposable package runtime state. |
| `bin/.dockpipe/runs/` | Host step run records. |
| `.dorkpipe/` | Optional DorkPipe analysis/handoff state. |

## Fast Orientation

- Architecture terms: `docs/agents/core/architecture.md`
- Engine boundary: `docs/agents/core/engine-boundary.md`
- Package/store model: `docs/agents/core/core-package-model.md`
- Validation commands: `docs/agents/core/validation-commands.md`

## Internal Workflow Locations

| Location | Use |
| --- | --- |
| `workflows/<name>/` | This repo's lean CI/dogfood workflows. |
| External package workflows | Run in the owning repository above using pinned Dockpipe tools, not sibling-source fallback. |

First-party workflow scripts belong beside the workflow `config.yml`, not in repo-root shadow script trees.

## Editor Mirrors

If a local editor-rule mirror of `AGENTS.md` exists, keep it in sync. Do not assume every checkout has one.

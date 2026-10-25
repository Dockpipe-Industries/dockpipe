# Runtime resources and package assets

Dockpipe's CLI is self-contained. It needs neither an authored core tree nor a
core archive to initialize a project or resolve an installed workflow.

| Owner | Contents | Loading |
| --- | --- | --- |
| Generic library | Typed runtime defaults, SDK helpers, container bootstrap resources and runtime-owned Git adapters | Built with the library; resources materialize only when the corresponding capability is requested. |
| Project workflows | `config.yml`, workflow-specific scripts, tests and assets | Explicit project roots and `compile.workflows`, or `--workflow-file`. |
| Resolver/workflow packages | Tool implementations, VM backends, provisioning and contributed UI | Declared package sources or project, user and system stores. |

## Initialization and SDKs

Bare `dockpipe init` generates a runnable host workflow from the typed library
model with Docker preflight disabled. Named blank initialization remains minimal.
Explicit local `--from` sources remain supported; initialization does not copy an
embedded workflow or shared authoring tree.

The portable SDK helpers belong to `src/lib/sdkassets`; the Go helper is
`dockpipe/src/lib/sdk/repotools`. `dockpipe sdk shell-env` prepares only the shell
helper it needs. `dockpipe sdk path --language shell|powershell|python` selects a
fixed library helper without requiring a source checkout.

Generic container resources and explicit legacy Git adapters belong to
`src/lib/runtimeassets`. They do not contain tool workflows. Existing explicit
`commit` and `worktree` strategies remain library-owned compatibility behavior;
operator workflow/project strategy files take precedence.

## Workflow and script resolution

Named workflows come from the selected project directory, declared compile roots,
or installed/configured package archives. The executable does not contain the
repository's CI, agent or example workflows, and it does not extract a workflow
snapshot into a cache. The launcher uses Dockpipe's catalog as its authority.

Workflow-specific scripts live beside `config.yml`. Reusable scripts live at their
lowest common owning boundary. Standard project `scripts/` overrides and package
asset resolution remain supported; checkout-only `src/scripts` and core-tree
fallbacks are retired.

## Build and release ownership

This repository's build, test, packaging and publication workflows live under
`workflows/`. Make and GitHub Actions launch those workflows. A separately pinned,
released bootstrap CLI builds the next CLI; that producer pin does not constrain
the new runtime through a project `dockpipe_version` setting.

The CLI archive is always produced. A package store is an explicit optional input
containing workflow/resolver packages. Schema-1 stores may omit `packages.core`
and may contain no optional packages. Historical core metadata can still be read
where needed to account for existing immutable artifacts; it is not a prerequisite
or a new producer output. Existing published release records and download assets
are unchanged.

`package compile core`, `package build core`, and the old core-tree installer are
retired. Remove `compile.core_from` from project configuration. Use library runtime
defaults, workflow/project strategy files and explicitly installed packages.

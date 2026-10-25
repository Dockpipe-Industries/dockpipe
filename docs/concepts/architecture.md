# Dockpipe engine (data flow)

**Terms:** **[architecture-model.md](architecture-model.md)**. This page is the current execution
shape of the Go CLI: how Dockpipe resolves workflows, prepares runtime state, runs host/container
work, and applies lifecycle actions around it.

## Primitive

Dockpipe still has one core action:

1. prepare runtime-owned state and optional host pre-work
2. run the requested step or command in the selected runtime
3. apply lifecycle follow-up such as outputs merge, checkpoint, cleanup, or post-action

For simple single-run flows that still read like run -> isolate -> act, the engine now also
owns managed workspace/session behavior, host setup, packaged workflow calls, and compiled package
resolution.

**Detach:** **`-d`** runs the container without attaching (stays up until the inner command exits).

## Components

| Piece | Role |
|-------|------|
| **`src/bin/dockpipe`** | Generated developer CLI output; build it through the repository workflow. Installed users run `dockpipe` from their installation. |
| **`src/cmd`**, **`src/lib/…`** | CLI parsing, workflow/package resolution, runtime orchestration, Docker/host execution, lifecycle actions. |
| **library resources** | Typed runtime defaults plus fixed generic resources in **`src/lib/runtimeassets/`** and **`src/lib/sdkassets/`**, materialized on demand. |
| **`src/lib/runtimeassets/containers/entrypoint.sh`** | Container entrypoint: run command, then optional **`DOCKPIPE_ACTION`**. |
| **compiled package store** | Project-local compiled workflow/resolver tarballs under **`bin/.dockpipe/internal/packages/`**. |
| **session state** | Runtime-owned managed workspaces, Docker volumes, leases, checkpoints, and audit metadata under **`bin/.dockpipe/sessions/`**. |

**Paths:** profile/script/package resolution is centralized in **`src/lib/infrastructure/paths.go`**
and related package/store helpers. Do not treat old `templates/core` repo paths as the in-repo
source of truth.

## Workflow resolution and execution shape

Current execution can enter through several surfaces:

- named workflow from **`workflows/<name>/config.yml`**
- nested workflow roots listed in **`dockpipe.config.json compile.workflows`**
- compiled workflow tarball from the package store
- arbitrary **`--workflow-file <path>`**

After resolution, Dockpipe may:

1. merge workflow defaults, step overrides, runtime/resolver selection, and strategy behavior
2. prepare managed workspace/session state when **`workspace.mode: managed`** is in play
3. run host setup or host steps
4. run container steps with `/work` mounted from the checkout or runtime-owned session storage
5. merge outputs, write manifests, checkpoint sessions, sync managed volume state, and run finally
   or after hooks

## Typical data flows

### Simple container run

```text
dockpipe --runtime dockerimage --isolate alpine:3.22 -- echo ok
  -> resolve workflow + runtime/resolver
  -> prepare isolate/image
  -> docker run with /work mounted
  -> optional DOCKPIPE_ACTION / host follow-up
```

### Managed session volume run

```text
dockpipe --workflow <name>
  -> resolve workflow.workspace
  -> create or resume runtime-owned session branch/workspace
  -> preflight/create/seed managed Docker volume when workspace.storage=volume
  -> run worker/container against /work in that volume
  -> sync back, checkpoint, cleanup, or publish via runtime lifecycle
```

### Multi-step workflow

```text
workflow config.yml
  -> resolve steps / finally / packaged workflow calls
  -> run host and container steps in order
  -> merge outputs and artifacts between steps
  -> run finally steps even on failure
```

## Extension points

1. **Runtime and SDK resources** — typed library defaults and fixed generic helpers.
   There is no shared core authoring tree or mandatory core package. See
   [resource ownership](runtime-resource-ownership.md).
2. **Project and package source** — declared workflow roots, resolver profiles,
   workflow-local scripts and assets, and installed workflow/resolver archives.
   Tool integrations belong to their independent package repositories.
3. **Images** — explicit registry images or Dockerfile-backed resolver/package
   assets, with compiled image-artifact manifests.
4. **Workflow YAML** — **`config.yml`**, **`steps:`**, **`workspace:`**, **`security:`**,
   and packaged-workflow calls. The generic schema also carries **`agent:`** metadata;
   DorkPipe owns its orchestration behavior.

## Runtime-owned lifecycle

The engine now owns more than process launch:

- managed Git session branches and workspaces
- session Docker volume seed/sync/cleanup
- worker lease and checkpoint lifecycle
- build/package/image preparation flows
- operation-result logging with timestamps, durations, and progress heartbeats for meaningful work

Those are engine/runtime responsibilities, not resolver-local shell conventions.

## Permissions

For normal Docker runs, Dockpipe still prefers host-user file ownership semantics where possible so
files in **`/work`** map cleanly back to the host checkout. Exact user mapping can vary by runtime
profile or image, but workflow authors should continue to think in terms of governed `/work`
execution rather than ad hoc container flags.

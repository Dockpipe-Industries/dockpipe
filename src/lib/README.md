# Dockpipe library ownership

Organize code by responsibility inside each DDD layer. A Go directory is a package
boundary: an owner keeps its implementation and direct tests together and depends
on lower-level owners rather than importing its parent package.

| Area | Responsibility |
| --- | --- |
| `application/` | CLI composition and workflow execution orchestration. |
| `application/internal/clioptions/` | Flag parsing and CLI option values. |
| `application/internal/*cmd/` | Independent command handlers, including project Git and workflow document editing. |
| `application/internal/packagecompile/` | Package compilation and closure orchestration. |
| `application/internal/packageimages/` | Planned/indexed image inventory and availability presentation. |
| `application/internal/imageartifact/` | Image manifest construction, serialization and archive identification. |
| `application/internal/runtimepolicy/` | Effective runtime and security policy assembly. |
| `application/internal/hostdependencies/` | Workflow/package host admission with explicit workdir, approval policy and invocation adapters. |
| `application/internal/workflowinputs/` | Typed field discovery, metadata, defaults and input binding, independent of catalog presentation. |
| `domain/workflow/` | Workflow/step values, YAML parsing and imports, validation and runtime-artifact rules. |
| `domain/workflowdocument/` | Inert YAML document editing and editor capacity rules. |
| `domain/packagecontract/` | Package manifest, catalog metadata and engine compatibility rules. |
| `domain/dependency/`, `domain/namespace/` | Shared dependency and namespace validation. |
| `domain/environment/` | Environment conversion, normalization and merge rules. |
| `domain/secretenv/`, `domain/remote/` | Secret binding/vault tokens and remote job contracts. |
| `infrastructure/` | Filesystem, container, process and runtime-session adapters and composition. |
| `infrastructure/workspacegit/` | Project Git requests, snapshots, review and lifecycle operations. |
| Other `infrastructure/` children | Focused adapters such as archive codecs, package catalog/build, process supervision and editor discovery. |
| `model/` | Authored configuration and artifact wire types. |
| `sdk/`, `sdkassets/`, `runtimeassets/` | Public host SDK and fixed generic library resources. |

`src/cmd` calls `application.Run`. Root Domain and Infrastructure entry points retain
source compatibility through aliases and explicit forwarding functions; new code
should use its owning package when that avoids unnecessary parent dependencies.
Private command dispatch calls independent command owners directly.

Workflow run/step/environment orchestration remains in Application. CLI dispatch
and workflow selection have separate boundaries; a run session owns host environment
state and success-only finalization. Prepared step state retains separate host and
container environments, and nested steps inherit captured execution adapters.
Catalog JSON and view controls remain presentation concerns. Infrastructure owns
the workflow/artifact/CI path layout shared by execution and SDK scope projection.

Compatibility test seams and some process-level configuration remain shared; this
does not establish concurrent `Run` support or change legacy cancellation behavior.
The root Infrastructure Docker and durable
session families also remain coupled. Legacy file-based project and manifest
loading APIs remain in Domain; this organization pass does not claim to resolve
those layer boundaries. The [library audit](../../docs/agents/tasks/recurring-codebase-hygiene/reliability-audit.md)
records their follow-up and the confirmed correctness repairs.

Generic resources live beside their owners, including
`runtimeassets/containers/entrypoint.sh`. Authored build/test workflows and helpers
stay under the repository's `workflows/` tree. See [source builds](../README.md)
for the workflow entry points.

### Multi-step workflows: async group, join point, merge

**End-user / authoritative spec:** **[../../docs/workflows/workflow-yaml.md](../../docs/workflows/workflow-yaml.md)** (keep in sync when behavior changes).

One-liner: **parallel steps share one merge; declaration order decides overwrites; the next blocking step is the join.**

| Idea | In YAML |
|------|---------|
| **Async group** | One YAML entry **`group: { mode: async, tasks: [...] }`**. All tasks start after the previous **blocking** step finishes. |
| **Join point** | The next step with **`is_blocking: true`** (or default). It runs only after **every** step in the async group has finished. |
| **Merged inputs** | The join step sees env from **before** the async group, plus **`outputs:`** from each async step merged in **list order** (same key → **later** list entry wins). |
| **Reset** | After a blocking step runs, the next async group starts fresh from that step’s outputs (same as sequential workflows). |

Field names use **snake_case** in YAML (`is_blocking`, not `isBlocking`). Optional **`id:`** on a step labels it in logs (e.g. `[merge]` lines); if omitted, logs use `step 1`, `step 2`, …

**Inputs (inside an async group)**  
Each member sees environment from the **last blocking barrier** only: snapshot at group start, then **only that step’s** `vars:` and `run` / `pre_script`. Siblings do **not** see each other’s env while running.

**Outputs (async group → join)**  
Nothing is merged until **all** async members finish. Then each member’s **`outputs:`** file is applied in **declaration order**. Duplicate keys → **last declarer wins** (same as sequential step-to-step merges). Overwrites are logged to stderr as:

`[dockpipe] [merge] variable "KEY" overwritten by <id or step N> (previously set by …)`

Use **distinct** `outputs:` *file paths* within one async group (duplicate paths are rejected).

**Restrictions**  
Host **commit-worktree** is not allowed inside an async group. **`kind: host`** members only contribute at merge time (their `outputs:` file, in order).

**Async `group` syntax** — this is the supported parallel authoring form. A `group` entry must be **only** the key `group` (no sibling keys). `tasks` use the same fields as a normal step (`cmd`, `id`, `outputs`, …). **`is_blocking` inside `tasks`** is ignored except **`is_blocking: true`**, which is rejected.

```yaml
steps:
  - id: setup
    cmd: echo ready
    is_blocking: true

  - group:
      mode: async
      tasks:
        - id: task_a
          cmd: sh -c 'echo BRANCH=a > out-a.env'
          outputs: out-a.env
        - id: task_b
          cmd: sh -c 'echo BRANCH=b > out-b.env'
          outputs: out-b.env

  - id: aggregate
    cmd: sh -c 'echo branch=$BRANCH'
    is_blocking: true   # join; BRANCH=b (last task in group wins on collision)
```

Plain-step **`is_blocking: false`** is no longer accepted. Use the explicit `group` entry above instead.

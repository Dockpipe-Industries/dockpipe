# Architecture

Read when changing concepts, docs, workflow semantics, packages, or engine behavior.

## Core Action

Dockpipe's engine has one action:

1. spawn an isolated environment
2. run a command inside it
3. optionally act on the result

Capabilities are separate dotted ids documented in `docs/concepts/capabilities.md`; they are not this three-step action.

## Core Concepts

| Concept | Meaning | Authoring location |
| --- | --- | --- |
| Workflow/template | What happens. YAML, steps, scripts. | Project/declared `workflows/` and workflow packages |
| Runtime | Where execution runs. | Typed library defaults or explicitly selected profile files |
| Resolver | Which tool/profile performs work. | Independent resolver packages and declared source roots |
| Strategy | Lifecycle wrapper around execution. | Workflow/project files and runtime-owned compatibility defaults |

## Hard Rules

- Do not confuse runtimes and resolvers.
- Packaged workflow invocation is a step form: `workflow:` plus `package:`.
- Not runtimes: Kubernetes, cloud APIs, Terraform, AI providers. Use runtimes/resolvers plus scripts.
- `runtime.type: agent` classifies behavior. It is not a separate Dockpipe product model.
- DorkPipe is a package/harness on top of Dockpipe primitives.
- Dockpipe is extended with workflows, runtimes, resolvers, and strategies; not plugins, core branching, or special-case flags.
- Runtime-owned Git sessions are runtime lifecycle behavior. Agents and resolvers request lifecycle
  transitions; they do not own raw Git commands.

## Ownership

Use [runtime resource ownership](../../concepts/runtime-resource-ownership.md).
Do not reintroduce a mandatory core package, authored workflow embedding or
checkout-only discovery. Keep tool backends and provisioning in their packages;
materialize generic library resources only for the requested capability.

## Mental Model

- Top-level `runtime` and `resolver` set workflow defaults.
- Step-level `runtime` and `resolver` override defaults.
- `isolate` is an advanced low-level image/template override.
- `kind: host` means host execution outside container isolation.
- `workflow` plus `package` enters a packaged child workflow.

## Canonical Docs

- `docs/concepts/architecture-model.md` (user concepts)
- `docs/concepts/architecture-contract.md` (normative engine invariants and source layout)
- `docs/concepts/capabilities.md`
- `docs/runtime/operation-results.md`
- `docs/runtime/git-runtime-sessions.md`
- `docs/workflows/workflow-yaml.md`

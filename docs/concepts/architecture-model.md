# How Dockpipe workflows fit together

A workflow describes work you want to repeat. You can author it in your project or
install it as a package. The same concepts apply to both.

## Core model

| Concept | Your choice |
| --- | --- |
| **Workflow** | What happens: commands, ordered steps, inputs and outputs. |
| **Runtime** | Where execution happens, such as a container from an image or Dockerfile. |
| **Resolver** | Which tool integration supplies tool-specific setup and execution behavior. |
| **Strategy** | Optional lifecycle behavior before or after the workflow. |
| **Assets** | Scripts, configuration and other files shipped with the workflow or its packages. |

Runtime and resolver are separate. Selecting an AI resolver does not create a new
kind of runtime; selecting a container runtime does not install every tool.

## Start with a workflow

A project workflow normally lives at `workflows/<name>/config.yml`. For example,
save this as `workflows/location/config.yml`:

```yaml
name: location
runtime: dockerimage
isolate: alpine:3.22

steps:
  - id: show-directory
    cmd: pwd
```

From that project directory:

```sh
dockpipe workflow validate workflows/location/config.yml
dockpipe --workflow location --
```

This uses an explicit example image and prints the container's working directory.
Docker must be available. For a first run without Docker, follow
[Your first workflow](../onboarding.md).

## Defaults and overrides

Set `runtime` and, when needed, `resolver` at the top of the workflow. They become
defaults for container steps; a step can override either selection.
Use `isolate` when you need an explicit image or image template.

Use `kind: host` for a command in the environment running Dockpipe. Host steps are
outside Docker's container policy. In Flatpak they run inside the app sandbox
unless a package explicitly bridges to the host. See [Isolation](isolation-layer.md).

A strategy can add lifecycle actions, including Git operations. Choose it
explicitly and review its effects before use; it is not required for a simple
command workflow.

## Workflows and packaged workflows (same spine)

Installing a workflow package supplies its YAML and assets as a reusable unit.
It does not require you to rebuild Dockpipe. Optional packages and required
resolvers are installed separately; see [Find and use packages](../packages/package-quickstart.md).

To call an installed packaged workflow from another workflow, use a step with
`workflow` and `package`. Replace these example names with the child workflow name
and its declared namespace:

```yaml
steps:
  - id: child
    workflow: child-name
    package: acme-team
```

The child owns its runtime, resolver and security settings. A packaged workflow
call is a step form, not a runtime named `package`.

## Capabilities and behavior types

Package metadata can declare a **capability**, such as a named tool or storage
need, that a resolver satisfies. A capability describes a requirement, not an
execution environment.

Profiles may also classify runtime behavior as `execution`, `ide` or `agent`
through `DOCKPIPE_RUNTIME_TYPE`. This classification does not select a container
engine, provision a service or grant an agent extra permissions.

## Where to go next

- [Authoring workflows](../workflows/workflow-authoring.md): runnable examples.
- [YAML reference](../workflows/workflow-yaml.md): all workflow fields.
- [Package model](../packages/package-model.md): authoring, installation and storage.
- [Security policy](../security/security-policy.md): effective permissions and limits.

The [maintainer architecture contract](architecture-contract.md) preserves the
engine invariants and source layout rules. It is for extending Dockpipe itself,
not a prerequisite for using workflows.

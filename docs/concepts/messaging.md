# Product description

Use these descriptions for repository summaries and short introductions. They
should match the capabilities described in the [README](../../README.md).

**Short description:**

> Run commands and workflows across local, container and remote environments.

**Product line:**

> Dockpipe runs anything, anywhere, in isolation.

**Longer description:**

> Dockpipe is a runtime, CLI and GUI shell for repeatable workflows. Workflows
> describe the work, runtimes choose where it runs, and optional resolver packages
> supply tool integrations. Run a local command, use a container with declared
> policy, or deliver a workflow to a configured remote node.

Host steps run with the permissions of the environment running Dockpipe; they are
not container-sandboxed. Remote execution has its own setup and qualification
limits. See [isolation](isolation-layer.md) and [remote nodes](../runtime/remote-nodes.md).

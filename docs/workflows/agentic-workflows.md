# AI workflows

DorkPipe is the optional package for AI orchestration. Its
[AI workflow guide](https://github.com/Dockpipe-Industries/dorkpipe/blob/master/docs/workflows/agentic-workflows.md)
covers prerequisites, provider setup, execution environments and review.
See its [harness configuration](https://github.com/Dockpipe-Industries/dorkpipe/blob/master/docs/workflows/harness-configuration.md)
for worker images, authentication and provider selection.

Dockpipe supplies the generic runtime. Installing the CLI or adding an `agent`
declaration does not install or start an AI harness. Install the required workflow
and resolver packages, then follow their setup instructions.

- [Install packages](../packages/package-quickstart.md)
- [Workflow and agent syntax](workflow-yaml.md#agentic-steps-agent)
- [Runtime security policy](../security/security-policy.md)

Package guidance is maintained with DorkPipe; this page preserves the entrypoint
for existing links and the Dockpipe documentation site.

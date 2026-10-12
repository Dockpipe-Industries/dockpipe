# AI workflows

Use an AI workflow when you want repeatable inputs, a selected tool/model,
inspectable outputs and a review step. An AI command still runs as a normal
Dockpipe workflow step. The workflow decides what happens; its runtime selects
where it runs, and its resolver selects the tool integration.

## Before you run one

- Install the workflow and the resolver packages it requires from
  [Marketplace or the CLI](../packages/package-quickstart.md).
- Set up the provider account or local model service required by that package.
  Installing a resolver does not authenticate you or download every model.
- Check which project files will be mounted or sent to the provider, which commands
  may run, and whether the workflow can change files.
- Review any cloud model costs and the workflow's budget/approval settings.

Run the installed workflow from the project it should work on, using the workflow
name and arguments in its package instructions. Do not assume a workflow named in
a repository example is included with a core-only installation.

## Choose the execution environment

| Choice | What to check |
| --- | --- |
| Container step | The image/resolver supplies the AI tool; project mounts, network policy and authentication are configured. |
| Host step | The tool is installed and authenticated in the environment running Dockpipe; Docker policy does not sandbox it. |
| Flatpak host integration | The package explicitly bridges to a host-installed tool; app installation does not supply host provider credentials. |

A resolver does not grant permission to read unrelated files or publish changes.
Use bounded project access and review the package's actions before running it.

## Agent declarations and orchestration

Some workflows use DorkPipe as an orchestration harness. Their `agent` declarations
can describe startup prompts, project guidance, readable/writable paths, model
settings, tasks, verification and approval. `model_policy` describes model attempts,
validation and escalation choices. The harness consumes those declarations and
produces execution artifacts.

Adding `agent:` to an arbitrary command does not itself start an AI service or
install the DorkPipe harness. Use a workflow/package that implements the declared
behavior. See the [YAML agent fields](workflow-yaml.md#agentic-steps-agent) for the
configuration reference.

Access declarations and task context are not a complete operating-system sandbox.
Check the runtime's actual mounts, permissions and network enforcement as described
in [Security policy](../security/security-policy.md). In particular, host commands
remain outside Docker container policy.

## Review the result

A useful workflow makes its result inspectable:

1. Read the run logs and generated outputs.
2. Check the proposed changes against the original task.
3. Run the project's relevant validation.
4. Approve any apply, commit or publication action through the workflow's explicit
   approval step, if it provides one.

Do not treat a model's success message as test evidence. Automatic verification and
approval behavior depend on the selected workflow; Dockpipe does not add a human
review gate to every arbitrary command.

## Current limits

DorkPipe can record model lane selection and escalation metrics. Outcome-weighted
learning and richer automatic task splitting remain development work.
`agent.access` declarations are not uniformly enforced as hard runtime permissions;
check the selected harness and runtime before relying on them for isolation.
Cloud authentication, available models, budget enforcement and end-to-end behavior
must be verified for the workflow/provider you use.

For internal examples, tooling proposals and orchestration design, see the separate
[contributor design notes](agentic-design-notes.md). They are not prerequisites for
using installed AI packages.

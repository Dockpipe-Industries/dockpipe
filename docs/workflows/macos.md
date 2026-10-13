# Workflows on macOS

Workflows declare supported **host** platforms in `platforms`. A Mac can run a
Linux container workflow through Docker Desktop; its container commands do not
need to become macOS commands. Host scripts and nested workflows must also
support macOS. A package supporting macOS does not imply that every workflow
inside it does.

## Prerequisites

- Install and start Docker Desktop for container workflows. Enable file sharing
  for the checkout or worker directory if Docker Desktop requests it.
- Install the commands listed in the selected workflow's dependencies. macOS
  install hints are included for the workflows enabled below.
- For scripts using `dockpipe sdk`, install Homebrew Bash with `brew install bash`.
  The SDK needs Bash 4.2 or newer; Apple's `/bin/bash` is insufficient. Put
  Homebrew's `bin` directory before `/bin` on the PATH used to launch Dockpipe.
  Confirm `bash --version` in that environment. A background worker must receive
  that same PATH when its service is installed or updated; changing a terminal's
  PATH does not change an already-running worker.
- Provider credentials, AI tools, Terraform configuration, and installed desktop
  editors remain prerequisites where the selected workflow uses them.

## Enabled workflows

| Group | Workflows |
| --- | --- |
| Core | `example` resolver, `init` starter, `run`, `run-apply`, `run-apply-validate`, `secretstore` |
| Container examples and checks | `codex-pav`, `codex-security`, `test`, `dockpipe-repo-quality` |
| Agent resolver workflows | `codex`, `claude`, `ollama` |
| Editors | `vscode`, `cursor-dev`, `devcontainer` |
| Secrets and infrastructure | `secretstore-onepassword`, `dockpipe.terraform.core`, `dockpipe.cloudflare.r2infra`, `dockpipe.cloudflare.r2upload`, `secretstore-r2-publish-test`, `dockpipe.github.ghcrpush`, `package-store-infra`, `package-store-staging-infra` |
| DorkPipe helpers | `skills.render`, `cas.01.app-server`, `codex.cli.update`, `orchestrator`, `compliance-handoff`, `user-insight-process`; the `dorkpipe` asset-only workflow is also available |

The `test` workflow uses bridge networking by default, including on Linux. It
does not require Docker Desktop's optional host networking. On Linux, if bridge
DNS or IPv6 prevents dependency downloads, explicitly pass
`--var DOCKPIPE_DOCKER_NETWORK=host`.

These declarations reflect a source portability audit and local validation.
They do not establish native Intel/Apple Silicon execution of every workflow,
provider acceptance, or successful interactive editor launch. Workflows still
need their ordinary approvals before publishing, changing infrastructure, or
updating host tools.

## Remaining restrictions

- `linux-vm` and `windows-vm` retain their existing host restrictions. Their QEMU
  firmware, acceleration, TPM, and passthrough setup needs a separate macOS port.
- `ci-emulate` depends on `windows-vm` on non-Windows hosts, so it remains restricted.
- Docs orchestration, `software.dev`, `example.brain`, `orchestrate.stack`,
  `backlog.remote`, and self-analysis workflows remain restricted pending a complete
  audit of their helper and child-script paths. Those paths include modern Bash
  features and GNU-specific file/time commands; adding a platform label alone
  would not establish support.
- `agent.cloud-lanes.doctor` still uses GNU `find -printf` and `timeout`.
- Pipeon stack workflows still use GNU hashing/file enumeration and modern Bash
  features. Their managed desktop stack needs macOS qualification.

## Updating an existing workflow

Existing checkout YAML and compiled workflow packages retain their old platform
lists. Updating the Dockpipe executable alone does not rewrite them. Recompile
or install the updated workflow package, then submit a **new** remote job. A
queued job contains the workflow snapshot that was submitted originally.

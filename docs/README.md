# Documentation

Package source examples below belong to [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages), [DorkPipe/MCP](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages) or [Pipeon](https://github.com/Dockpipe-Industries/pipeon/tree/master/packages/pipeon), as applicable. Run package-source commands in that owning repository with pinned Dockpipe tools.

Dockpipe has a small front door and deeper reference material behind it. Start
with the task you are doing, then drop into reference docs only when needed.

## Start Here

If you are new to Dockpipe, stay in this section first. The concept and
maintainer docs below are reference material, not the shortest learning path.

| Goal | Doc |
|------|-----|
| Install and run the first command | [install.md](install.md), then [onboarding.md](onboarding.md) |
| Write a workflow | [workflows/workflow-authoring.md](workflows/workflow-authoring.md) |
| Look up every YAML key | [workflows/workflow-yaml.md](workflows/workflow-yaml.md) |
| Find, install and use packages | [packages/package-quickstart.md](packages/package-quickstart.md) |
| Inspect CLI flags and subcommands | [cli-reference.md](cli-reference.md) |

## Core Concepts

| Topic | Doc |
|-------|-----|
| How workflow, runtime, resolver and strategy fit together | [concepts/architecture-model.md](concepts/architecture-model.md) |
| Isolation layer and profile files | [concepts/isolation-layer.md](concepts/isolation-layer.md) |
| Capability ids and resolver packages | [concepts/capabilities.md](concepts/capabilities.md) |
| Governed AI/documentation workflows | [workflows/agentic-workflows.md](workflows/agentic-workflows.md) |
| Optional typed authoring layer | [concepts/pipelang.md](https://github.com/Dockpipe-Industries/pipelang/blob/master/docs/concepts/pipelang.md) |

## Packages, Security, And Images

| Topic | Doc |
|-------|-----|
| Package/store model | [packages/package-model.md](packages/package-model.md) |
| Security policy | [security/security-policy.md](security/security-policy.md) |
| VM boot display geometry | [runtime/vm-display-geometry.md](runtime/vm-display-geometry.md) |
| Docker image artifacts | [runtime/image-artifacts.md](runtime/image-artifacts.md) |
| Combined design notes / history | [security/docker-security-images.md](security/docker-security-images.md) |
| Windows WSL containers and WSL3 research | [research/wsl-containers-and-wsl3-2026.md](research/wsl-containers-and-wsl3-2026.md) |

## Advanced / Maintainer

Most users do not need this section on day one. Use it when you are maintaining
Dockpipe itself, publishing packages, or debugging compiled/runtime behavior.

| Topic | Doc |
|-------|-----|
| Engine invariants and source layout | [concepts/architecture-contract.md](concepts/architecture-contract.md) |
| Coding style, resource bounds and failure handling | [coding-style.md](coding-style.md) |
| AI workflow design and contributor examples | [workflows/agentic-design-notes.md](workflows/agentic-design-notes.md) |
| Self-hosted package publication | [packages/package-publishing.md](packages/package-publishing.md) |
| PipeLang foundation and proposals | [concepts/pipelang-foundation.md](https://github.com/Dockpipe-Industries/pipelang/blob/master/docs/concepts/pipelang-foundation.md), [contracts](https://github.com/Dockpipe-Industries/pipelang/blob/master/docs/concepts/pipelang-foundation-contracts.md), [delivery](https://github.com/Dockpipe-Industries/pipelang/blob/master/docs/concepts/pipelang-foundation-delivery.md) |
| Engine data flow | [concepts/architecture.md](concepts/architecture.md) |
| Dockpipe vs first-party maintainer packages | [packages/core-tools.md](packages/core-tools.md) |
| Generated maintainer artifacts | [runtime/artifacts.md](runtime/artifacts.md) |
| Compatibility retirement ledger | [compatibility-retirement.md](compatibility-retirement.md) |
| Codex workspace-sandbox session limits and host requests | [runtime/codex-sandbox-sessions.md](runtime/codex-sandbox-sessions.md) |
| Agent routing and focused maintainer rules | [agents/index.yaml](agents/index.yaml), [agents/](agents/) |
| Core-vs-packages audit | [packages/core-vs-packages-audit.md](packages/core-vs-packages-audit.md) |
| PipeLang verification optimization plan | [runtime/pipelang-verification.md](https://github.com/Dockpipe-Industries/pipelang/blob/master/docs/runtime/pipelang-verification.md) |
| Manual QA | [manual-qa.md](manual-qa.md) |
| Messaging / about copy | [concepts/messaging.md](concepts/messaging.md) |

MCP (`mcpd`) is DorkPipe-owned control-plane tooling; see the
[external MCP package](https://github.com/Dockpipe-Industries/dorkpipe/tree/master/packages/dorkpipe-mcp).
Browse [catalog package examples](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages)
and [ecosystem ownership](packages/core-tools.md) for the independent source repositories.

## CLI Orchestrator Workflow

The package-owned CLI orchestrator is implemented for core development and non-Pipeon users. It
exercises the same DorkPipe provider-pool contract as Pipeon direct chat, so the CLI path remains the
reference implementation rather than a second routing system.

Implemented workflow shape:

- Package-owned DorkPipe workflow for a direct orchestrator prompt.
- Inputs include prompt text, workspace path/session identity when needed, provider override, model
  override, budget/approval mode, and optional workflow-escalation policy.
- Default provider resolves from DorkPipe provider-pool config. Inline CLI override can select
  `ollama`, `codex`, `claude`, or future providers without editing the workflow file.
- Direct execution uses a warm provider worker when available. If the pool is not ready, the CLI shows
  `warming`, `auth-required`, `queued`, or `provider-disabled` state instead of silently falling back
  to an expensive cold path.
- A direct CLI worker can request escalation into YAML-backed agentic workflows, but the escalation
  remains explicit and artifact-backed.

Example target UX:

```bash
dockpipe --package dorkpipe --workflow orchestrator -- --prompt "summarize this repo"
dockpipe --package dorkpipe --workflow orchestrator -- --provider claude --prompt "review the package boundary"
dockpipe --package dorkpipe --workflow orchestrator -- --provider ollama --model llama3.2 --prompt "quick local answer"
```

Implementation notes:

- Keep provider pool config and provider availability in DorkPipe package-owned catalogs/config.
- Keep Pipeon provider dropdown labels and CLI provider names backed by the same catalog.
- Use the CLI orchestrator workflow for local/core development smoke tests before validating the
  Pipeon UX.
- If the CLI workflow changes authored YAML semantics, update schema/docs/language-support together.

## Prebuilt Image Strategy

The biggest release-time win is prebuilding common Pipeon images instead of building them on the
developer machine during first launch.

Candidate images:

- `dockpipe-code-server:<version>` with Pipeon and DockPipe language-support VSIX files already
  installed.
- `dockpipe-dorkpipe-stack:<version>-linux-amd64` with Linux `dockpipe`, `dorkpipe`, and `mcpd`
  binaries already present.
- Optional GPU-aware stack variants only if the runtime contract really differs. Prefer one image
  with compose/runtime GPU toggles when possible.
- Optional base/runtime variants for common host configs only after measuring actual demand.

Release flow:

- Build images from exact package inputs and versioned tool binaries.
- Tag by DockPipe version and content digest.
- Publish image metadata with expected package/version/signature.
- Let `pipeon-dev-stack` prefer matching prebuilt images when available.
- Fall back to local source builds when offline, unpublished, or running dirty development inputs.
- Keep local build mode available for open-source/offline users and maintainers.

This likely needs its own small release/helper application or package command that can generate and
publish the image matrix as new versions come out. Treat it like Docker layer caching plus package
release automation, not as ad hoc launch-script logic.

## Measurement Plan

Capture coarse timing around:

- code-server image signature check
- Pipeon VSIX packaging
- code-server image build or reuse
- Linux tool build or reuse
- DorkPipe stack image build or reuse
- Docker compose up
- MCP readiness
- host MCP bridge readiness
- Ollama readiness
- model pull or cached model check
- code-server container readiness
- desktop shell open

The launch script should emit enough timing to explain slow starts without forcing users to inspect
Docker logs manually.

## Remaining Decisions

- Which image variants are worth publishing for the first release: CPU-only, NVIDIA GPU, or one
  runtime-configurable image?
- Should image selection be automatic by version/signature or explicit by `PIPEON_DEV_STACK_IMAGE_*`
  overrides?
- Where should image metadata live: package catalog, release manifest, or generated package state?
- How should offline installs seed images: tarball import, local registry, or documented `docker pull`
  cache warming?
- What is the acceptable first-launch target on Windows with Docker Desktop already running?
- Can Codex eventually use a persistent protocol/MCP mode for lower latency, or is `codex exec
  resume` the right direct lane until the CLI exposes a stable daemon interface?


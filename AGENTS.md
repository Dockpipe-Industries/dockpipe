# AGENTS.md - Dockpipe Root Router

Dockpipe is the governed, cross-platform runtime for commands, packages, environments, CI jobs, AI workflows, and deployable tooling.
The engine has one action: spawn -> run -> act.
DorkPipe is a Dockpipe package and harness, not a replacement for Dockpipe.
This file is a lightweight router. Load only the focused docs needed for the task.

Machine-readable routing: `docs/agents/index.yaml`.

This checkout owns the runtime, CLI, GUI shell, host SDK and workflow editor extension. Remaining catalog package source is in [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages/). The compiler and language SDK are in [PipeLang](https://github.com/Dockpipe-Industries/pipelang). Consume versioned external artifacts and the pinned PipeLang Go module; never copy those implementations back or depend on a sibling checkout. Existing typed-model, `dockpipe pipelang` and materialization behavior remains supported.

The Dockpipe GUI shell lives in `src/dockpipe-app/`; read its `ARCHITECTURE.md`
before changing UI ownership or host SDK boundaries. Call the product Dockpipe.
Legacy installed IDs and SDK symbols remain compatibility contracts.

## Global Hard Rules

When user input is genuinely required and the host supports structured prompts, use Yes/No for
binary decisions or 2-3 mutually exclusive choices with the recommended option first. Use prose
when the decision needs more than three meaningful choices, structured choices would misrepresent
it, or the host lacks that UI.

| Rule | Where to read |
| --- | --- |
| Apply readable code, contract-based resource bounds, cancellation and failure-path verification to new or materially changed code. | `docs/coding-style.md` |
| Keep `src/lib/` and `src/cmd/` generic. No repo-specific package/workflow/staging knowledge in engine code. | `docs/agents/core/engine-boundary.md` |
| Preserve the architecture model: workflow/template = what, runtime = where, resolver = tool/profile, strategy = lifecycle wrapper. | `docs/agents/core/architecture.md` |
| Use package/store helpers for project/global paths. Do not hand-write bare `.dockpipe/internal` paths. | `docs/agents/core/core-package-model.md` |
| Prefer the scope model for generated paths: `cwd: artifacts` for simple producers, `cwd: repo` plus `scopes` only when checkout cwd is required. | `docs/agents/core/path-scopes.md` |
| Treat Git lifecycle as runtime-owned session behavior. Agents request checkpoint/sync/publish; they do not run raw Git. | `docs/agents/runtime/git-runtime-sessions.md` |
| For Codex workspace-sandbox sessions, verify effective capabilities. Request a narrow reviewed host operation for unavailable Docker, remote Git, or network access; never bypass the sandbox. | `docs/agents/runtime/codex-sandbox-sessions.md` |
| Template/workflow work should stay in YAML/assets/scripts unless a general primitive is needed. | `docs/agents/workflows/yaml-workflows.md` |
| Package-specific behavior belongs inside package YAML/assets/scripts/docs/tests. | `docs/agents/packages/package-authoring.md` |
| Keep authored YAML/schema/editor docs in sync when changing workflow/config surfaces. | `docs/agents/workflows/yaml-workflows.md` |
| Secrets must be references only. Never commit plaintext secrets or generated resolved templates. | `docs/agents/runtime/safety-guardrails.md` |
| This is the public product repository. Personal identifiers and deployment configuration stay private; reusable packages and workflows require operator-supplied targets. | `docs/public-repository-boundary.md` |
| Treat `bin/.dockpipe/` and `.dorkpipe/` as generated/read-only grounding unless the user asks to refresh. | `docs/agents/runtime/artifacts-and-mcp.md` |
| AI workflows must beat one strong direct worker on quality, safety, cost, review effort, or rerun value. DorkPipe owns the lower-level proof through artifacts/metrics, not user boilerplate. | `docs/agents/workflows/ai-workflow-value-bar.md` |
| Main docs stay canonical for repo facts and public behavior. `docs/agents/` is the compressed routing/safety layer, not a shadow documentation tree. | `docs/agents/docs/docs-system.md` |
| Treat `docs/agents/task-index.yaml` as the AI entrypoint for the cross-cutting backlog and keep the linked task files current when a task materially completes or advances one of those items. | `docs/agents/task-index.yaml` |
| Keep implementation uncommitted until authorized. An explicit request to move a bounded task to staging authorizes its validated commit and staging promotion flow without repeated approval. | `docs/agents/docs/session-handoffs.md` |

## Task Routing

Default agent execution uses `gpt-6.1-sol` with Medium reasoning. Delegate bounded,
independent work when its expected value exceeds coordination and context cost.
Prefer one stronger direct worker when that is cheaper overall; reserve escalation
for difficult architecture, security, or repeated repair. Keep ownership and checks
explicit, and avoid overlapping edits or automatic fanout.

| Task | Read first | Recommended skills |
| --- | --- | --- |
| Engine or CLI behavior | `docs/agents/core/engine-boundary.md`, `docs/agents/core/architecture.md`, `docs/agents/core/validation-commands.md` | `dorkpipe-core-review` |
| Workflow YAML or authored surface | `docs/agents/workflows/yaml-workflows.md`, `docs/agents/runtime/safety-guardrails.md` | `dorkpipe-yaml-workflows` |
| Path/scope/artifact migration | `docs/agents/core/path-scopes.md`, `docs/agents/workflows/yaml-workflows.md`, `docs/agents/core/core-package-model.md` | `dorkpipe-yaml-workflows`, `dorkpipe-package-authoring` |
| Git runtime sessions or workspace lifecycle | `docs/agents/runtime/git-runtime-sessions.md`, `docs/agents/core/architecture.md`, `docs/agents/core/path-scopes.md` | `dorkpipe-core-review`, `dorkpipe-yaml-workflows` |
| Package authoring | `docs/agents/packages/package-authoring.md`, `docs/agents/core/core-package-model.md` | `dorkpipe-package-authoring` |
| Package promotion | `docs/agents/packages/package-promotion.md`, `docs/agents/core/validation-commands.md` | `dorkpipe-package-authoring`, `dorkpipe-core-review` |
| Agentic/DorkPipe workflows | [DorkPipe guidance](https://github.com/Dockpipe-Industries/dorkpipe/blob/master/docs/agents/index.yaml), `docs/agents/workflows/yaml-workflows.md` for generic fields | `dorkpipe-agentic-yaml`, `dorkpipe-yaml-workflows` |
| GUI or editor surface | `src/dockpipe-app/ARCHITECTURE.md`, `docs/agents/workflows/tooling-surfaces.md`, `docs/packages/editor-extensions.md` | `dorkpipe-core-review` |
| Docs or agent guidance | `docs/agents/docs/token-optimization.md`, `docs/agents/docs/skills.md` | `dorkpipe-token-optimization` |
| Docs system or sync rules | `docs/agents/docs/docs-system.md`, `docs/agents/docs/token-optimization.md` | `dorkpipe-token-optimization`, `dorkpipe-core-review` |
| Artifacts or MCP | `docs/agents/runtime/artifacts-and-mcp.md`, `docs/agents/runtime/safety-guardrails.md` | `dorkpipe-core-review` |

## Skill Routing

Use target-independent skill ids. Do not write target-specific skill routing keys.
Select an absolute installed or built `DOCKPIPE_BIN` and verify that the external
DorkPipe package is available. Its renderer supports Codex, Claude and generic targets:

```bash
"$DOCKPIPE_BIN" --package dorkpipe --workflow skills.render -- --list
"$DOCKPIPE_BIN" --package dorkpipe --workflow skills.render -- --target codex
```

Curated skills available from the separately installed DorkPipe package (verify host availability before use):

- `dorkpipe-agentic-yaml`
- `dorkpipe-core-review`
- `dorkpipe-objective-execution`
- `dorkpipe-package-authoring`
- `dorkpipe-task-handoff`
- `dorkpipe-token-optimization`
- `dorkpipe-yaml-workflows`

## Forbidden Artifact Summary

Do not add or commit:

- plaintext secrets, resolved vault templates, local `.env` material with private values
- cache/build/generated state unless explicitly intended: `bin/.dockpipe/`, `.dorkpipe/`, `.staging` outputs, local assistant output
- repo-root one-off script shadows such as `scripts/dockpipe/...`
- engine references to checkout-only `packages/`, `workflows/`, or `.staging/` paths/names
- target-specific skill routing in AGENTS or `docs/agents/index.yaml`

## Before Editing

1. Identify task type in `docs/agents/index.yaml`.
2. Load only the routed docs and relevant skill instructions.
3. Check whether the work is engine, workflow, package, resolver, strategy, docs, or generated artifact.
4. If editing `src/`, verify the change is a general primitive.
5. If changing authored YAML semantics, update schema, docs, and language support together.
6. If touching external packages, work in their owning repository, keep logic in package assets, and select a pinned Dockpipe binary/SDK rather than sibling source.
7. If moving generated paths, decide first whether the file is workflow artifact, package state, resolver scope, or source.
8. Check `git status --short` and do not revert unrelated user changes.

## Final Report Checklist

Report:

- what changed and why
- files or areas touched
- validations run and results
- any generated artifacts created
- risks, TODOs, or skipped checks
- whether package/engine boundaries were preserved

## Focused Docs

Use `docs/agents/index.yaml` for guidance and `docs/agents/task-index.yaml` for
open core work. Package-owned routing lives with its owner; closed records are
historical evidence, not pending instructions. See `docs/agents/TASKS.md`.

Dockpipe runs anything, anywhere, in isolation. Keep it simple. Keep it composable.

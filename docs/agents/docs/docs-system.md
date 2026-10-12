# Docs System

Read when changing `docs/`, `AGENTS.md`, `docs/agents/`, indexes, or generated guidance.

## Goal

Keep Dockpipe documentation self-documenting without creating two competing source trees.

## Layering

| Layer | Role | Source of truth |
| --- | --- | --- |
| Main docs (`docs/*.md`) | Canonical human/reference documentation. | Canonical |
| `AGENTS.md` | Short root router and hard-rule entrypoint for agents. | Canonical for agent startup/routing only |
| `docs/agents/index.yaml` | Machine-readable routing and validation map. | Canonical for agent task routing |
| `docs/agents/*.md` | Focused compressed guidance, maintenance rules, and task-specific constraints. | Derived from canonical docs; do not become shadow reference manuals |
| Skills | Reusable assistant behavior. | Canonical for assistant behavior, not repo facts |

## Hard Rules

- Do not maintain full normalized duplicates of main docs under `docs/agents/`.
- Keep repo facts, public behavior, and reference semantics in the main docs first.
- Use `docs/agents/` for routing, compression, mandatory guardrails, and sync rules.
- If a focused agent doc starts restating a large reference page, either link to the canonical doc
  or split out only the decision-critical subset.
- One topic per file. Split files when they start mixing routing, policy, reference, and backlog in
  one place.

## Sync Contract

When a change affects user-visible behavior, authored surfaces, or terminology:

1. Update the canonical main doc first.
2. Update `docs/agents/` only where routing, compressed guidance, or safety rules changed.
3. Update `docs/agents/index.yaml` if task routing, required reads, or validation changed.
4. Update `AGENTS.md` only if the root router or global hard rules changed.
5. Update related task files when the change materially advances or completes backlog work.

## Split Rules

Split a doc when any of these become true:

- one file mixes more than one primary topic
- agent-facing routing starts obscuring the human-readable explanation
- a focused agent file grows into a partial copy of a canonical reference doc
- the maintenance checklist for a topic becomes large enough to deserve its own file

Preferred pattern:

- main doc explains the system for humans
- `docs/agents/<topic>.md` gives the minimal routing/safety layer for agents
- `docs/agents/index.yaml` points to the right file set for the task

Each active indexed backlog task uses `docs/agents/tasks/<task>/index.yaml`. The global task index
points to that local index, which names the smallest read set for orientation, decisions,
implementation, or history. Keep ordinary tasks to one `overview.md`; split only independently
useful branches.

## Token Optimization

Token optimization should usually improve the main docs, not create a second normalized corpus.

Use compressed agent-only text only when at least one of these is true:

- agents need a strict router or checklist that humans do not
- the canonical reference is intentionally detailed and a small decision subset is enough for most tasks
- the content is operational policy rather than general product/reference explanation

## Maintenance Checks

- Can a human still find the canonical explanation in `docs/` without reading `docs/agents/`?
- Can an agent route from `AGENTS.md` plus `docs/agents/index.yaml` without loading half the repo?
- Did a behavior change update the main doc before the compressed agent layer?
- Are there any stale agent summaries that still mention old paths, old flags, or old architecture terms?

## Public-site audience

The DockPipe Cloud site selects source pages in its `scripts/site/stage-docs.py`.
Before changing site-facing docs, inspect that selection rather than assuming all
repository Markdown is published.

- Installation and quickstarts assume an installed application and the user's own
  project, without a Dockpipe checkout, Go compiler or contributor Make targets.
- State package/image/tool prerequisites before commands; use runnable project
  examples rather than Dockpipe's own CI or documentation workflows.
- Keep engine invariants, release operations and product proposals in linked
  contributor references outside the public learning path.
- Preserve substantive contracts when moving material; check relative links and
  site link rewriting after edits.
- Validate examples outside this checkout. Distinguish schema validation, local
  execution and provider/platform acceptance.
- Source edits do not refresh or publish the Cloud site. Report snapshot/build
  boundaries explicitly.

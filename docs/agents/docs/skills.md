# Skills

Read when routing tasks to DorkPipe skills or rendering assistant-specific skill formats.

## Principle

Skill ids are target-independent. Codex and Claude are render targets/adapters, not DockPipe concepts.

Use:

```yaml
skills:
  - dorkpipe-core-review
  - dorkpipe-objective-execution
  - dorkpipe-one-shot-gate
  - dorkpipe-task-handoff
  - dorkpipe-token-optimization
```

Do not use target-specific skill routing keys. Keep routing neutral and let the renderer adapt the
skill for Codex, Claude, or another target.

## Installed Codex Skills

- `dorkpipe-agentic-yaml`
- `dorkpipe-core-review`
- `dorkpipe-objective-execution`
- `dorkpipe-one-shot-gate`
- `dorkpipe-package-authoring`
- `dorkpipe-task-execution`
- `dorkpipe-task-handoff`
- `dorkpipe-token-optimization`
- `dorkpipe-yaml-workflows`

## Render Commands

```bash
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --list
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target codex
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target claude --output /path/to/claude-skills
```

Codex default output:

```text
~/.codex/skills/<skill-name>/SKILL.md
```

Claude requires `--output` until a safe documented global install path is confirmed.

## Source Of Truth

Curated DorkPipe skill sources live in:

```text
packages/dorkpipe/resolvers/dorkpipe/assets/skills/
```

New governed work uses three lifecycle roles:

- `dorkpipe-objective-execution` owns the bounded outcome across ordinary checkpoints and enforces
  the receiving task's first-checkpoint and output budget.
- `dorkpipe-one-shot-gate` consumes one separately approved gate and returns to the objective.
- `dorkpipe-task-handoff` transports unchanged lifecycle state between tasks while keeping source
  and receiver transport allowances distinct.

`dorkpipe-task-execution` remains a compatibility router for existing task records. Do not use it
as the controller for new objectives.

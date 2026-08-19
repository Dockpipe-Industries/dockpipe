# Agent Tasks

The indexed agent task backlog lives under `docs/agents/tasks/`.

Start at:

- `docs/agents/task-index.yaml`

Closed history lives under:

- `docs/agents/tasks/closed/`

Keep the index and linked task files current when work materially completes or advances one of
those items.
Do not keep closed items in the active YAML index.

Use one Markdown file for an ordinary task. When a task has independently loadable contract,
planning, current-boundary, and history branches, move it into a folder with a local `index.yaml`.
Point the global task index at that local index and route agents to the smallest sufficient file
set; do not load or concatenate the whole folder by default.

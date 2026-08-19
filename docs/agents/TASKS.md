# Agent Tasks

The indexed agent task backlog lives under `docs/agents/tasks/`.

Start at:

- `docs/agents/task-index.yaml`

Closed history lives under:

- `docs/agents/tasks/closed/`

Keep the index and linked task files current when work materially completes or advances one of
those items.
Do not keep closed items in the active YAML index.

Every active task owns a folder with a local `index.yaml`. Keep an ordinary task in one focused
`overview.md`; add more files only when contract, planning, current-boundary, or history branches
are independently loadable. Point the global task index at the local index and route agents to the
smallest sufficient file set; do not load or concatenate the whole folder by default.

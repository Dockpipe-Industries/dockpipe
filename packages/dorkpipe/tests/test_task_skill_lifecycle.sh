#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SKILLS="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/skills"
EXECUTION="$SKILLS/dorkpipe-task-execution/instructions.md"
HANDOFF="$SKILLS/dorkpipe-task-handoff/instructions.md"

require_text() {
	local file="$1"
	local text="$2"
	local scenario="$3"
	if ! grep -Fq -- "$text" "$file"; then
		echo "missing lifecycle contract for scenario: $scenario" >&2
		exit 1
	fi
}

require_text "$EXECUTION" 'Acceptance passes; one exact successor is warranted' "successful execution with authorized successor"
require_text "$EXECUTION" 'Acceptance passes; no successor exists' "successful execution without further work"
require_text "$EXECUTION" 'A genuine invariant, authority, ownership, or human decision blocks safe progress' "genuine blocking invariant"
require_text "$EXECUTION" 'Required verification fails after the authorized implementation' "failed verification"
require_text "$EXECUTION" 'Repeated audit yields only non-blocking findings' "non-blocking repeated audit"
require_text "$EXECUTION" 'Dirty tree contains unrelated user-owned changes' "unrelated dirty-tree changes"
require_text "$EXECUTION" 'Handoff lacks authority to implement' "insufficient handoff authority"
require_text "$EXECUTION" 'Refuse recursion; execution runs only in the fresh successor' "execution and handoff recursion prevention"
require_text "$EXECUTION" 'More than one top-level successor is proposed' "single top-level successor"
require_text "$HANDOFF" 'After successful creation, report the new task and stop work in the old task.' "old session stops after handoff"

require_text "$EXECUTION" 'execution_authority: approved_task_creation' "creation authorizes exact slice execution"
require_text "$EXECUTION" 'Before that exact response, do not invoke handoff or create the task.' "approval happens before handoff"
require_text "$HANDOFF" 'none — approving creation authorized this slice' "successor does not ask again"
require_text "$EXECUTION" 'completed | blocked | failed_verification' "terminal state vocabulary"
require_text "$EXECUTION" 'top_level_successor_limit: 1' "successor ceiling"
require_text "$HANDOFF" 'execution_skill: dorkpipe-task-execution' "fresh successor invokes execution"
require_text "$HANDOFF" 'Never infer approval from a broad backlog' "narrow chaining authority"

echo "task skill lifecycle scenarios OK"

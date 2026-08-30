#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SKILLS="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/skills"
OBJECTIVE="$SKILLS/dorkpipe-objective-execution/instructions.md"
HANDOFF="$SKILLS/dorkpipe-task-handoff/instructions.md"
TOKEN="$SKILLS/dorkpipe-token-optimization/instructions.md"

require_text() {
	local file="$1"
	local text="$2"
	local scenario="$3"
	if ! grep -Fq -- "$text" "$file"; then
		echo "missing lifecycle contract for scenario: $scenario" >&2
		exit 1
	fi
}

reject_text() {
	local file="$1"
	local text="$2"
	local scenario="$3"
	if grep -Fq -- "$text" "$file"; then
		echo "forbidden lifecycle contract for scenario: $scenario" >&2
		exit 1
	fi
}

require_text "$OBJECTIVE" 'checkpoint_policy: automatic_within_objective' "ordinary checkpoints stay in one objective"
require_text "$OBJECTIVE" 'handoff_policy: user_requested_only' "handoff requires the user"
require_text "$OBJECTIVE" 'continue without asking for approval or creating a fresh task' "objective advances without micro-approval"
require_text "$OBJECTIVE" 'Once the exact action is authorized, keep its' "authorized operations stay in the objective"
require_text "$OBJECTIVE" 'Do not invent approval seals, reuse policies, attempt budgets, or no-retry rules.' "no artificial authority limits"
require_text "$OBJECTIVE" 'No external effect' "zero-effect retry classification"
require_text "$OBJECTIVE" 'Idempotent or reconcilable effect' "idempotent retry classification"
require_text "$OBJECTIVE" 'Partial or unknown effect' "ambiguous effects require read-back"
require_text "$OBJECTIVE" 'A failed preflight that produced no external effect may be fixed and rerun.' "preflight is retryable"
require_text "$OBJECTIVE" 'tell the user briefly that the current' "context pressure is disclosed"
require_text "$OBJECTIVE" 'Keep working in' "context warning does not auto-handoff"
require_text "$OBJECTIVE" 'Use `dorkpipe-task-handoff` only when the user requests a fresh task.' "handoff is user controlled"
require_text "$OBJECTIVE" 'run broad terminal verification once after the last material change' "broad proof is not repeated"
require_text "$OBJECTIVE" 'task-owned temporary log' "noisy output is bounded"

require_text "$HANDOFF" 'when the user explicitly requests it' "handoff entry is explicit"
require_text "$HANDOFF" 'mode: continue_objective' "single continuation mode"
require_text "$HANDOFF" 'transport_authority: user_requested' "transport authority is explicit"
require_text "$HANDOFF" 'A later' "future handoff needs another request"
require_text "$HANDOFF" 'A failed preflight or proven zero-effect attempt remains' "retry evidence survives transport"
require_text "$HANDOFF" 'Never create more than one task from' "one task per user request"
require_text "$HANDOFF" 'Target 500-900 words' "ordinary handoff is compact"
require_text "$HANDOFF" 'execute the pending boundary before expanding' "receiver starts from carried boundary"

require_text "$TOKEN" 'Target 500-900 words' "ordinary continuation stays compact"
require_text "$TOKEN" 'temporary log and return only' "noisy output stays outside conversation context"

reject_text "$OBJECTIVE" 'waiting_for_gate' "objective has no gate state"
reject_text "$OBJECTIVE" 'single_entry' "objective cannot impose single-entry approval"
reject_text "$OBJECTIVE" 'Never retry' "objective cannot impose blanket no-retry"
reject_text "$OBJECTIVE" 'automatic_at_safe_boundary' "objective cannot auto-create tasks"
reject_text "$OBJECTIVE" 'enter_one_shot_gate' "objective cannot split gate execution"
reject_text "$HANDOFF" 'enter_one_shot_gate' "handoff has no gate-entry mode"
reject_text "$HANDOFF" 'resume_objective' "handoff has no gate-return mode"
reject_text "$HANDOFF" 'single_entry' "handoff cannot expire approval by policy"
reject_text "$HANDOFF" 'automatic_at_safe_boundary' "handoff cannot auto-create tasks"
reject_text "$HANDOFF" 'source_transport_limit' "handoff cannot manufacture transport budgets"
reject_text "$HANDOFF" 'receiver_context_handoff_limit_per_task' "handoff cannot manufacture receiver budgets"

echo "persistent objective and user-requested handoff contracts OK"

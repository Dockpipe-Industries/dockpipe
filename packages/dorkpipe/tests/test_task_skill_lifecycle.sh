#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SKILLS="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/skills"
OBJECTIVE="$SKILLS/dorkpipe-objective-execution/instructions.md"
GATE="$SKILLS/dorkpipe-one-shot-gate/instructions.md"
HANDOFF="$SKILLS/dorkpipe-task-handoff/instructions.md"
COMPAT="$SKILLS/dorkpipe-task-execution/instructions.md"
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

require_text "$OBJECTIVE" 'checkpoint_policy: automatic_within_objective' "ordinary checkpoints do not need approval"
require_text "$OBJECTIVE" 'continue without asking for approval or creating a fresh task' "objective advances automatically"
require_text "$OBJECTIVE" 'Ordinary checkpoint completion is never a handoff trigger.' "no checkpoint handoff loop"
require_text "$OBJECTIVE" 'The objective controller never invokes a gate.' "objective cannot consume gate authority"
require_text "$OBJECTIVE" 'state: waiting_for_gate' "objective pauses at a gate"
require_text "$OBJECTIVE" 'The return is not a new objective or a request to choose a' "gate returns to same objective"
require_text "$OBJECTIVE" 'Only optional improvements remain' "objective convergence"
require_text "$OBJECTIVE" 'branch or HEAD change, unexpected dirty path, protected-state' "material rebaseline triggers"
require_text "$OBJECTIVE" 'context_handoff_policy: automatic_at_safe_boundary | ask_before_transport' "context handoff policy is explicit"
require_text "$OBJECTIVE" 'Do not claim access to an exact remaining-token meter.' "no imaginary token counter"
require_text "$OBJECTIVE" '**Hard signals:**' "hard context-pressure signals"
require_text "$OBJECTIVE" '**Soft signals:**' "soft context-pressure signals"
require_text "$OBJECTIVE" 'when any hard signal' "hard signal triggers handoff"
require_text "$OBJECTIVE" 'or at least two soft signals are present' "two soft signals trigger handoff"
require_text "$OBJECTIVE" 'Do not hand off during a mutation, running command,' "context handoff waits for safe boundary"
require_text "$OBJECTIVE" 'Do not hand off when the objective is close enough to finish safely' "near-complete objective stays local"
require_text "$OBJECTIVE" 'context_handoff_limit_per_task: 1' "one context handoff per task"
require_text "$OBJECTIVE" 'The fresh task starts with this receiver budget:' "receiver starts bounded"
require_text "$OBJECTIVE" 'own the single `Pending boundary` as the first checkpoint' "receiver executes carried boundary first"
require_text "$OBJECTIVE" 'each fresh task one outgoing context-saving handoff of its own' "per-task handoff allowance resets"
require_text "$OBJECTIVE" 'When the threshold is met, handoff takes priority' "handoff prevents receiver re-expansion"
require_text "$OBJECTIVE" 'run broad terminal verification once after the last material change' "broad proof is not repeated"
require_text "$OBJECTIVE" 'task-owned temporary log' "noisy objective output is bounded"

require_text "$GATE" 'attempt_count: 0' "fresh gate has no attempts"
require_text "$GATE" 'Mark `authority_consumed: true` and increment `attempt_count` as the exact invocation begins' "authority consumption boundary"
require_text "$GATE" 'Never retry under the same or reconstructed authority.' "no gate retry"
require_text "$GATE" 'failed_verification_before_consumption' "pre-invocation failure stays unconsumed"
require_text "$GATE" 'consumed_failed' "post-invocation failure spends authority"
require_text "$GATE" '`return_mode: resume_objective` is mandatory' "gate must return to objective"
require_text "$GATE" 'needs no new micro-slice approval' "return transport is preauthorized"
require_text "$GATE" 'Never hand off directly to another gate.' "gate chain prevention"

require_text "$HANDOFF" '`continue_objective`' "objective continuation mode"
require_text "$HANDOFF" '`enter_one_shot_gate`' "gate entry mode"
require_text "$HANDOFF" '`resume_objective`' "objective resume mode"
require_text "$HANDOFF" 'Handoff is transport only' "handoff cannot manufacture authority"
require_text "$HANDOFF" 'Objective authority survives' "objective authority survives transport"
require_text "$HANDOFF" 'Spent gate authority returns as evidence' "spent gate authority is not reusable"
require_text "$HANDOFF" 'preauthorizes one `resume_objective` handoff' "single automatic gate return"
require_text "$HANDOFF" 'Never create more than one task from' "single transport limit"
require_text "$HANDOFF" 'transport_authority: <user_requested | objective_context_policy' "objective context policy transports authority"
require_text "$HANDOFF" 'context_handoff_policy: automatic_at_safe_boundary' "automatic context handoff is honored"
require_text "$HANDOFF" 'records a hard signal or at least two soft signals' "handoff rechecks context-pressure evidence"
require_text "$HANDOFF" 'receiver_context_handoff_limit_per_task: 1' "receiver transport budget is explicit"
require_text "$HANDOFF" 'source_transport_limit: 1' "source transport budget is explicit"
require_text "$HANDOFF" 'Target 500-900 words' "ordinary handoff is compact"
require_text "$HANDOFF" 'Never encode a consumed source transport as a consumed receiver transport.' "source and receiver budgets stay distinct"
require_text "$HANDOFF" 'execute the pending boundary before expanding' "handoff constrains receiver intake"

require_text "$COMPAT" 'invoke `dorkpipe-one-shot-gate`' "legacy gate routing"
require_text "$COMPAT" 'invoke `dorkpipe-objective-execution`' "legacy objective routing"
require_text "$COMPAT" 'Do not execute implementation, validation, or live action inside this router.' "compatibility is routing only"
require_text "$COMPAT" 'Do not recreate `top_level_successor_limit: 1`' "old micro-slice ceiling removed"
require_text "$COMPAT" 'return it directly to `dorkpipe-objective-execution`' "legacy loop cannot capture gate return"
require_text "$COMPAT" "fresh objective task's allowance" "compatibility routing preserves receiver transport"

require_text "$TOKEN" 'Target 500-900 words' "ordinary continuation stays compact"
require_text "$TOKEN" 'temporary log and return only' "noisy output stays outside conversation context"
require_text "$TOKEN" 'Never compress it into an' "per-task allowance cannot become a chain limit"

reject_text "$OBJECTIVE" 'Approve next slice:' "objective cannot require per-slice approval"
reject_text "$HANDOFF" 'Approve next slice:' "handoff cannot require per-slice approval"
reject_text "$GATE" 'top_level_successor_limit' "gate cannot create successor slices"
reject_text "$HANDOFF" 'Hard stops:\nno second handoff' "handoff cannot invent a chain limit"

echo "objective and gate skill lifecycle scenarios OK"

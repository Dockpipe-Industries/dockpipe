#### Proposed post-reconciliation product/storage decision — 2026-08-04

**Status: accepted as product/storage direction on 2026-08-04.** Acceptance establishes
product/storage direction only. Implementation, migration, schemas, controls, and cross-platform
atomicity evidence remain separately gated. This subsection records design only; it adds no
implementation or authority.

**Evidence and current ownership.** Provider-pool currently owns three canonical package-state
records: the immutable Pipeon-session-to-`codex_app_server` binding, the last completed App Server
session state, and the consecutive unresolved turn claim. Pipeon separately owns its retained adapter
and `recovery_required` / `outcomeUnknown: true` snapshot inside VS Code workspace chat state. The
existing recovery-only observation proves only that the exact retained provider thread satisfied the
verified-idle `RecoverBaseline` contract during that attempt. It does not prove what happened to the
unknown prompt. The rejected separate-write-ordering fixture demonstrates that neither current owner
can safely remove its guard evidence before the other.

**Proposed truthful meaning.** Use `reconciled_outcome_unknown` as a working name for one non-terminal,
non-authorizing state. It means only that the exact retained provider thread identified by the bound
pre-state was reconciled to the existing verified-idle recovery baseline. The prior prompt may have
completed, failed, been cancelled, produced content, caused side effects, or never been observed; this
state makes none of those claims. It is not completed, failed, cancelled, dismissed, replayed, retried,
or recovered content. It authorizes no prompt, replay, retry, fallback, normal Send, claim reuse, or
new child.

Continued chat could become eligible only after a separate explicit user decision equivalent to
"acknowledge the unknown prior outcome and continue with a fresh later turn." That decision must be
bound to the exact aggregate revision and reconciliation fingerprint, must never authorize replay of
the unknown turn, and must be consumed at most once when a strictly later fresh turn is claimed. Until
that later decision is durably accepted, both host dispatch and the Pipeon Send projection remain
guarded. Closing or abandoning the session needs no continuation authority.

**Selected owner and authority.** The proposed sole durable transaction owner is the provider-pool
coordinator, using one canonical per-session App Server lifecycle aggregate under provider-pool
package state. This follows the existing ownership of adapter pinning, App Server session state,
recovery evidence, and turn claims. The aggregate, not a companion marker, becomes the authority for
the binding, completed state, unknown turn, recovery guard, observation consumption, and future user
decision. Mere aggregate-file existence supplies no authority: the complete canonical schema,
identity, revision, state, and fingerprints must validate or the session remains blocked.

Pipeon's retained App Server snapshot becomes projection-only. After migration it is rendered from
the authoritative aggregate and may be cached in workspace state for display, but stale, missing,
cleared, malformed, or early/late projection writes can neither authorize nor prevent an otherwise
authoritative transition. Before prompt dispatch, the Pipeon host must query/validate the aggregate's
guard state; absence or failure is closed. Pipeon messages and other chat presentation data remain
Pipeon-owned and do not enter the lifecycle aggregate.

The future aggregate must carry these semantics, independent of production syntax:

- aggregate schema/version and monotonic revision;
- exact Pipeon session ID and exact `codex_app_server` adapter binding;
- exact provider-session ID and recovery-evidence reference;
- retained model and reasoning values;
- last known completed turn, the consecutive unknown pending turn, and a turn high-water mark that
  prevents that pending turn number from ever being reused;
- non-terminal unknown-outcome state, including `outcome unknown = true` and
  `reconciled to verified idle = true` without a terminal outcome;
- an exact recovery-observation fingerprint bound to the exact pre-state fingerprint and the one
  successful recovery attempt;
- unresolved-claim consumption state and permanent replay-forbidden state;
- guarded explicit-user-decision state: initially required, later capable of recording one exact
  accepted decision bound to the current revision/fingerprint, and finally consumed by at most one
  strictly later fresh-turn claim.

**Future compare and commit semantics.** One provider-pool operation must hold a verified
cross-process, OS-backed exclusive lock for the session across observation, comparison, and commit.
A process-local mutex is insufficient because Pipeon, CLI/MCP hosts, restarts, and concurrent
provider-pool processes do not share memory and can all observe durable state.

The expected pre-state fingerprint must be computed from an unambiguous length-prefixed encoding of
the exact bytes (and their named paths/roles) of the current adapter binding, completed session state,
unresolved claim, and canonical Pipeon retained-adapter/unknown-outcome guard, plus the session,
provider-session, recovery-evidence, model, reasoning, completed-turn, and pending-turn identities.
The recovery observation must be minted and used only inside that locked owner operation and bind the
same fingerprint. A display projection, status label, bare lock file, or caller-supplied digest without
the exact source evidence is insufficient.

Immediately before commit, the owner must reread and byte-compare all authoritative source evidence,
reject a stale or substituted observation, verify the aggregate revision or legacy-absence condition,
and reject any semantic or byte mismatch without mutation. The sole commit point is a verified
same-directory, same-volume atomic replacement of the complete canonical aggregate. The owner must
fully write and synchronize the temporary file before replacement, then synchronize the aggregate
and parent directory before reporting success. A platform without verified atomic replacement and
required synchronization must fail closed rather than emulate the transaction with ordered writes.

Failure before or during comparison leaves the old state authoritative. Failure after comparison but
before replacement leaves only an ignorable temporary file and the old state authoritative. The
atomic replacement makes the new aggregate authoritative in one step. Failure after replacement but
before durable acknowledgement is an unknown commit result: no retry or replay is allowed; restart
must reload and validate the aggregate to determine which full revision is visible. Temporary files,
locks, or acknowledgements never override the valid aggregate.

On restart, readers accept only one complete valid aggregate revision and derive the guard from it.
Two concurrent operations using the same observation/pre-state serialize on the cross-process lock;
after the first commit, the second sees a changed revision and consumed observation and is rejected.
The exact observation is likewise rejected after restart. A new reconciliation attempt requires a
new observation over the still-current exact pre-state; it cannot reuse an earlier observation or
the unknown pending turn.

**Migration and mutation.** Initial migration must occur under the same owner lock. With no aggregate
present, the owner must strictly read the three current provider-pool records and the exact canonical
Pipeon retained-adapter/guard slice, bind them into the expected pre-state fingerprint, perform the
recovery observation, reread the sources, and atomically create the first authoritative aggregate.
The future Pipeon integration must make that exact retained slice available to the transaction owner;
display-only state is not migration evidence. The first aggregate commit is the authority cutover.
All legacy readers and writers must then reject or defer to the aggregate. The old provider-pool files
and Pipeon snapshot may remain frozen for audit/compatibility until a later cleanup, but are ignored
for authorization and are not deleted as part of this transition.

The transition preserves the adapter, provider session, recovery evidence, model, reasoning, and last
known completed turn. It consumes the exact unresolved claim into the authoritative guarded recovery
record, records its pending turn as the no-replay high-water mark, replaces the separate Pipeon guard's
authority with `reconciled_outcome_unknown`, records the bound successful observation as consumed, and
sets explicit-user-decision state to required. It does not delete the claim merely because idle was
observed; classify the prompt outcome; clear or authorize Send by projection; mutate messages or
content; start a prompt, provider, exec, fallback, retry, supervisor, or child; or alter the retained
provider thread.

**Rejected alternatives.** Each of these remains unsafe:

- Removing the claim and then clearing Pipeon strands the guard after destroying candidate evidence
  if the second write fails.
- Clearing Pipeon and then removing the claim removes the host/Send guard while the unresolved claim
  survives if the second write fails.
- Mapping the snapshot to `completed`, `failed`, or `cancelled` invents a terminal prompt outcome.
- Keeping multiple authoritative files behind a process-local mutex does not serialize other
  processes or survive restart and therefore preserves the partial-commit problem.
- Adding a transaction or tombstone file whose mere existence becomes authority cannot prove its
  contents bind the exact old records, observation, or committed post-state. Only the complete valid
  aggregate and atomic replacement may carry authority.
- Treating successful `RecoverBaseline` as a prompt result confuses verified idle with terminal
  outcome or recovered content.
- Allowing Pipeon display state to override provider-pool lifecycle evidence lets stale, missing, or
  reordered UI persistence authorize continuation.
- Reusing an exact observation after commit, restart, or in another process can apply the same
  transition twice or revive stale evidence; the aggregate must persist consumption and reject it.

**Maintainer decision:** accepted on 2026-08-04 as product/storage direction only. This acceptance
authorizes only a separately bounded implementation-planning slice, not implementation. Classifier,
recovery-only operation, current records, claims, bindings, provider-pool responses, fallback,
adapter pinning, Pipeon guards and controls, rollback, retry, and no-replay behavior remain unchanged.
TASK-013 and CAS-14 remain open.


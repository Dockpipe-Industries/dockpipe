## Set the receiver contract

The user request authorizes preparation of one continuation prompt for the user to paste into a
fresh chat. It does not authorize automatic future handoffs. A later handoff requires another
user request.

Require the fresh task to execute the pending boundary before expanding. Completed proof is
admitted, not replayed; only drift, new failure evidence, or a direct dependency of the pending
checkpoint justifies reopening it.

## Create and stop

- When the user explicitly requests handoff or continuation, return the complete paste-ready
  continuation prompt in one fenced text block without a second confirmation.
- Tell the user to open a fresh Claude chat in the same project and saved checkout and paste the
  prompt there. State that no new chat was created.
- Do not call `spawn_task` or another task-creation tool for this handoff. Use manual copy-paste
  so the handoff does not create a worktree.
- After returning the prompt, stop work in the old chat.

Never invoke the fresh task's execution skill in the old task. Never claim that ephemeral browser
or agent-local state will survive task transport; record how the receiver can reclaim and verify
any state it still needs.

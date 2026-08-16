# DorkPipe Token Optimization

Use this skill when editing agent-facing guidance.

## Goal

Keep routing precise while preserving repository-specific safety rules.

## Pattern

- Put the short router in `AGENTS.md`.
- Put focused task guidance under `docs/agents/`.
- Put reusable assistant behavior in DorkPipe skills.
- Use `docs/agents/index.yaml` to map task type to docs and skill ids.

## Compression Rules

- Prefer tables, checklists, and path maps.
- Keep one topic per file.
- Link to canonical docs instead of copying them.
- Avoid generic AI-agent advice.
- Avoid target-specific skill routing keys; use `skills`.
- Keep forbidden artifacts centralized instead of repeated everywhere.

## Handoff And Tool Output

- Target 500-900 words for an ordinary continuation packet. Exceed this only for an exact sealed
  gate contract or protected-state inventory that cannot be represented safely by counts and digests.
- Carry one canonical statement per fact. Preserve authority, exclusions, dirty ownership, failed
  proof, and the next boundary; remove chronology and explanations of already-completed work.
- Prefer affected paths, counts, and digests over full inventories and per-file hashes.
- Admit durable completed proof in the receiver. Do not instruct it to rerun or reconstruct that
  proof without drift, new failure evidence, or a direct dependency.
- Keep successful commands quiet. For predictably noisy checks, retain full output in a task-owned
  temporary log and return only exit status plus the relevant failure excerpt.
- Treat a per-task handoff limit as resetting in the fresh task. Never compress it into an
  objective-wide "no second handoff" rule unless that chain limit is explicit.

## Completed slices

When editing handoff guidance, use [Session Handoffs](../../../../../../../docs/agents/docs/session-handoffs.md) as the canonical checklist. The global AGENTS.md rule makes its commit question and compact next-slice prompt mandatory for every completed slice; do not duplicate the checklist here.

## Review Pass

1. Identify which task types need the guidance.
2. Remove duplicated product philosophy.
3. Preserve hard rules exactly once.
4. Confirm skill ids are target-independent.
5. Check that an agent can decide what to read without loading every doc.

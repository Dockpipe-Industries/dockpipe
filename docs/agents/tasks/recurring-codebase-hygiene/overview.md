# TASK-035 Recurring codebase hygiene

This remains an open recurring practice. Individual completed audits and repairs
are closed; they are not standing instructions to repeat old work.

## Current state — 2026-10-25

- The [library and GUI audit](reliability-audit.md) records completed archive,
  async-output, admission, SDK and application/GUI cleanup work with its focused
  verification. Earlier claims that all seven October 13 findings remain
  unimplemented are superseded. The same record lists the remaining bounded
  correctness and architecture findings; recheck source before selecting one.
- [Current source organization](topology.md) routes to the library's owner map.
  Eleven completed structural/audit records have moved to
  [closed history](../closed/recurring-codebase-hygiene/README.md).
- The [human-docs pass](public-docs-audit.md) and
  [agent guidance pass](agent-docs-audit.md) are completed documentation slices.
  Cross-platform builds, provider acceptance and publication remain separate.
- Retained compatibility work follows the [canonical ledger](../../../compatibility-retirement.md).
  Do not treat historical state/import or release records as fresh cleanup authority.

## Remaining work

Choose one concrete issue from the current audit when requested. Preserve existing
behavior and package boundaries, verify the owning failure paths, then update that
finding's disposition. A recurring hygiene label does not authorize unrelated
refactoring or re-open completed work.

Broader platform acceptance belongs to TASK-025 and its platform tasks; live
remote pairing/delivery belongs to TASK-036. [TASK-044 extraction](../closed/ecosystem-extraction/index.yaml)
is completed through staging; optional Guest Tools/VM acceptance remains with
TASK-045. Local documentation checks do not close those wider qualification tasks.

## Historical evidence

The task-local index has separate routes for historical release, durable-state and
compatibility work. Load those records only for a direct dependency or regression;
versions, absolute-location labels and old pending instructions describe their
original checkpoints. The [previous summary](../closed/recurring-codebase-hygiene/overview-before-2026-10-25.md)
is retained for provenance.

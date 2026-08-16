# Compatibility Retirement

Read this before inventorying, changing, warning on, or retiring a compatibility surface.
The canonical source-backed inventory is
[`docs/compatibility-retirement.md`](../../compatibility-retirement.md).

## Hard Rules

- Inventory is evidence, not removal authority. A retirement changes one ledger ID under one
  separately approved slice.
- Keep active source, current public promises, current callers/fixtures, source history, and closed
  historical evidence distinct. A repository search cannot prove downstream absence.
- Record an introduced or minimum-supported version only when it is provable. Keep it `unproven`
  when tags, release records, or package support policy do not establish it.
- Configuration and CLI retirement must update implementation, focused positive/rejection tests,
  canonical docs/help, schema, and editor support wherever the ledger entry names those mirrors.
- Layout and state retirement is not cleanup authority. Inventory, migration, behavior removal,
  live inspection, and deletion are separately approved boundaries.
- Package-owned compatibility stays in that package. Generic engine code may provide the primitive
  but must not learn package-specific aliases, layouts, schemas, or state cohorts.
- Closed task records, promotion packets, hashes, and old handoffs remain immutable context; they
  never authorize a retry, migration, cleanup, or execution.

## Review Checklist

1. Select one exact ledger ID and copy its named replacement, callers, missing proof, and proof
   profile into the approved task.
2. Re-read the current source, docs/help, schema/editor, package, and fixture anchors; line numbers
   are navigation hints and may drift.
3. Prove a downstream support/version floor. If that proof is unavailable, retain the surface.
4. Run the entry-specific positive, precedence, and old-input rejection tests plus the routed
   config/CLI/package validations.
5. Update the ledger disposition and all named mirrors in the same change. Preserve unrelated
   compatibility IDs and historical evidence.

#### Remaining cross-platform evidence and acceptance gates

CAS-13 is Windows-only evidence. Before CAS-14 is complete, controlled host-resident evidence is
still required on current supported Linux and macOS hosts for initialization/version/schema gate,
advertised model/reasoning validation, workspace-write containment, native manual and automatic
review modes, explicit broader-access confirmation and non-inheritance, bounded user input,
interruption terminal, clean exit, transport loss, direct-child termination, persisted idle recovery,
path/root validation, and process-tree cleanup. Raw payloads and credentials remain excluded on
every platform.

The founder product decision above is accepted. CAS-14 implementation still requires a separately
bounded implementation action and can be called complete only after all of the following are true:

- the primary/default single-consumer adapter, session-pinned model/reasoning/approval/sandbox
  selection, stable/experimental capability gates, neutral input-response seam, rendering contract,
  fallback, rollback, retention, and recovery behavior above are
  implemented and fixture-tested without changing bounded workers or engine code;
- the focused Go/package/Pipeon tests and the controlled Windows, Linux, and macOS evidence pass
  with no raw-protocol or sandbox-policy regression;
- the final evidence review confirms that `codex_exec` remains the governed legacy/fallback adapter,
  the App Server stays host-resident, every selected policy is visible and validated, each turn
  retains its selected native sandbox, and no prompt was replayed; and
- the maintainer explicitly accepts the CAS-14 implementation evidence.

CAS-15 cannot begin merely because CAS-14 is implemented or the default route exists. It requires
CAS-14 to be complete, a reviewed first-consumer evidence packet with rollback exercised, and a new
explicit maintainer decision naming exactly one additional compatible consumer. CAS-16 fallback
surface, CAS-17 operations guidance, ForgePipe, and remaining provider-pool work stay deferred.


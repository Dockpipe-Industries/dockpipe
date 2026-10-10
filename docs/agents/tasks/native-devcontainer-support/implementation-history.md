## Implementation Update — Read-Only Discovery/Status Slice (2026-07-13)

Implemented the recommended first vertical slice in the package-owned
`packages/ide/resolvers/devcontainer` resolver only.

- The `devcontainer` package workflow provides deterministic, filesystem-only discovery for the
  standard, legacy, and direct root `.devcontainer/*.json` / `*.jsonc` definitions. It records a
  workspace-relative reference, safe display name when parsable, and content fingerprint. Multiple
  candidates always fail closed until the caller supplies `--definition-ref`; no selection is
  guessed.
- `status` requires that explicit reference and accepts captured `read-configuration`, Docker
  inspect, and optional managed-session JSON fixtures. It deliberately rejects absent live
  adapters: no code in this resolver invokes Docker, the Dev Container CLI, hooks, an editor, or a
  provider. The resulting `devcontainer.lifecycle.v1` NDJSON stream normalizes
  `unavailable`, `selection_required`, `available`, `not_created`, `created`, `running`,
  `stopped`, and `ambiguous` states plus `external`, `managed`, `orphan_candidate`, and
  `ambiguous` ownership.
- CLI execution is the package workflow; DorkPipe's existing tiered generic `dockpipe.run` MCP
  bridge invokes that same workflow and returns the same recorded event stream. There is no
  Pipeon Docker/Dev Container CLI path, no provider-pool integration, and no engine change.
- Fixture tests cover standard/legacy/alternate/malformed definitions, stable ordering,
  multi-definition refusal, adapter absence, `not_created` / external / managed / orphan / changed
  fingerprint / duplicate-container status, event sequencing, and identifier/workspace redaction.

## Implementation Update — Approved Managed `up` Contract Slice (2026-07-13)

Implemented the next lifecycle contract slice in `packages/ide/resolvers/devcontainer` only.

- `up` requires an explicit workspace-relative `--definition-ref` and first emits an
  `up_requested` event. Without an approval record bound to the request id, workspace identity,
  selected definition reference/fingerprint, and all pull/build/Compose/features/hooks risks, it
  emits `approval_required` and fails before an adapter result is read or a session record exists.
- The fixture adapter accepts only a successful installed/pinned Dev Container CLI result whose
  installed version equals its pin. A successful result must bind the container id, session id,
  selected workspace/reference/fingerprint, and
  `com.dockpipe.devcontainer.session`. The resolver persists that exact managed record only to an
  explicit workspace-relative output path, while the event stream exposes an opaque environment
  reference rather than the container id.
- The existing `devcontainer.lifecycle.v1` NDJSON and generic `dockpipe.run` MCP stdout path are
  unchanged. Pipeon remains an event consumer; provider pools remain separate. This contract adds
  neither a live CLI/Docker invocation nor any external-container execution, adoption, cleanup,
  attach, build, stop/down/remove, or Pipeon control.
- Fixture tests cover unapproved and incomplete approval failure, approved pinned-adapter result,
  session-record persistence, record-backed managed status, redaction, and the prior discovery and
  status cases. No test invokes Docker, a Dev Container CLI, hooks, or an editor.

Remaining recovery/cleanup risks: a crash after a real future CLI creates a container but before
the session record is persisted will be an orphan candidate; failed/cancelled real starts need a
later reconciliation/repair contract. The retention policy is decided: Pipeon close stops only an
exact managed container and retains its record for reuse. Remove/down and automatic recovery remain
unauthorized until their separate destructive/recovery contracts exist.

## Implementation Update — Pinned Live Read-Configuration Verification (2026-07-13)

The package-owned resolver now has an explicit `--live-read-configuration` status adapter. It
requires the installed `@devcontainers/cli` to equal package pin `0.87.0`, then invokes only
`read-configuration` for the selected definition and still consumes Docker status exclusively from
the existing captured fixture. Windows resolves the npm command shim to the package JavaScript entry
point under the current Node executable, without a shell. Missing, unpinned, timed-out, malformed,
or identity-mismatched output fails closed and retains no raw adapter output. Dev Container CLI
`0.87.0` performs its own label-filtered, read-only `docker ps` during this operation; that is the
sole Docker exception for the slice. No direct Docker call, `up`, hook, editor, provider, `exec`,
stop, remove, or Pipeon lifecycle path was added.

# Long-term C++ implementation direction

The user selected a gradual C++ implementation across the Dockpipe ecosystem:
Dockpipe runtime/desktop/SDK, separately owned packages and PipeLang. Track this
as future work; extraction establishes repository and dependency boundaries and
does not perform the rewrite.

Replace bounded responsibilities incrementally while preserving observable
commands, workflows, language semantics, package manifests, state/wire formats,
SDK/ABI, error/cancellation propagation and resource contracts. Prove equivalent
behavior through independent consumers and existing regression gates before
retiring an implementation. Keep the repositories independently buildable and
use versioned external SDK/artifact dependencies during the transition.

Prioritize from measured build/runtime/maintenance costs rather than assuming
C++ is automatically faster or smaller. Each proposed slice needs explicit
ownership, compatibility proof, rollback and supported-platform evidence.

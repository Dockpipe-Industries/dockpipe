# Unified execution observability

Objective: `dockpipe-unified-execution-observability`.
State: completed for the supported local implementation described in
[the canonical runtime guide](../../../runtime/execution-observability.md).
No commit, publication, live infrastructure mutation or additional handoff occurred.

## Delivered

- Existing host-run IDs, workflow invocation/step context and supervision feed a
  shared typed telemetry model, collector interface and bounded async recorder.
- Host and Docker launch/wait boundaries and remote worker subprocesses record
  lifecycle. The VM runtime registers its exact QEMU host process through a
  generic resource-association API, preserving its existing PID cleanup sidecar.
- `runs observe` exposes project/global history and separate System discovery.
  Kernel start identities prevent PID reuse from attaching another process's
  metrics. Docker client metrics and selected container metrics remain separate.
- Execution Explorer reuses ResourceCanvas, CanvasPanel and ResourceLinkItem.
  Managed/System views, filtering, progressive disclosure, pause, CPU history,
  selected-resource inspection and safe operation-log summaries are integrated.
- YAML configuration uses existing `vars` string values, with schema and document
  round-trip proof; no additional authored YAML surface or frontend was added.

## Verification

- 35 relevant Go packages passed across application, infrastructure, domain and
  CLI. Existing loopback fixtures required the reviewed host test lane.
- Focused race tests passed for identities, concurrent recording, rapid exits,
  missing metrics, cancellation, collector failures, resource associations,
  queue/retention bounds and private-field omission from log summaries.
- Launcher built; 15/15 CTests passed. The final selection-history reset passed
  the focused Explorer test after that suite. Dark/light fixtures were rendered
  and visually inspected; selected cards remain visible beside the inspector.
- A fresh CLI fixture ran two concurrent workflows, verified distinct workflow
  IDs and live Linux metrics, then retained both successful terminal states.
- Monitor cancellation returned a cancellation error and reaped its fixture
  Docker-stats child. No actual Docker resource was started or stopped.
- Existing VM state-split fixtures passed. A direct runtime-sidecar fixture proved
  missing telemetry helpers remain nonfatal and opt-out preserves PID tracking.
- Windows amd64 and macOS arm64 Go cross-builds passed. Native runtime/Qt
  qualification and live Docker/QEMU telemetry remain unclaimed.
- `git diff --check` passed. Thirty-nine unrelated inherited files retained their
  admission digests; eight inherited UI/docs files were extended deliberately.
  Staging remains empty.

Local receipts are `/tmp/dockpipe-observability-final-go.log`,
`/tmp/dockpipe-observability-core.log`, `/tmp/dockpipe-explorer-ctest.log`,
`/tmp/dockpipe-explorer-last-test.log`,
`/tmp/dockpipe-observability-live-proof.json`,
`/tmp/dockpipe-observability-cancel-proof.log`,
`/tmp/dockpipe-observability-vm-tests.log`, and
`/tmp/dockpipe-observability-vm-adapter.log`.
Generated outputs are the existing launcher build directory, the isolated
`/tmp/dockpipe-observability-cli` binary, temporary fixtures/caches and preview
PNGs. Existing compiled package stores were not refreshed.

## Remaining platform boundaries

Linux has native process identity, System discovery and host metrics. macOS and
Windows retain lifecycle metadata without native process discovery or per-process
metrics. The Windows whole-system collector extension is tracked below. The updated remote worker can explicitly opt into system/Docker snapshots;
remote job status and raw logs are available in Executions under Runs & logs. QEMU guest metrics and generic descendant ownership are
not inferred. The bounded history is a recent-activity cache, not an audit archive.
See the runtime guide for recommended follow-ups and exact support limits.

## Overall resources and machine selection extension

- System now includes CPU, memory, swap, root filesystem capacity, network rates,
  uptime/load and a bounded CPU chart. Linux kernel counters remain nullable;
  aggregate CPU uses all cores as 100%, separately from per-process CPU.
- Both System and Docker select local or paired remote machines through existing
  broker credentials. Remote System is machine discovery; project-managed
  history is still local. Switching targets clears old data and guards callbacks.
- Worker-local `remote monitoring --system --docker` opts into an outbound,
  bounded snapshot channel; `--disable` revokes it without changing pairing or
  delivery authority. The broker validates node ownership, operator reads, 1 MiB
  samples, stale data and revocation. Monitoring is joined on worker shutdown.
- Remote Docker exports safe inventory/topology only. Container mutation,
  interactive terminals and raw logs remain local; no remote Docker control
  protocol or live deployment was introduced.
- Activity is merged into Executions under Runs & logs; the standalone navigation
  entry is removed. Local logs/stop and remote cancel/download remain available.

Extension verification: all 37 application/infrastructure/domain/CLI Go packages
passed; focused monitoring race tests passed; launcher build and all 15 CTests
passed, followed by the two affected UI tests after the final callback guard.
The CLI returned real local Linux resource counters. macOS arm64 and Windows
amd64 cross-builds passed; native execution is still unqualified. Remote transport,
revocation, expiry, privacy and cancellation were tested with local broker/fake
Docker fixtures, not a deployed worker or live Docker daemon. The resource view
was rendered and visually checked in the dark theme. Receipts:
`/tmp/dockpipe-system-full-go.log`, `/tmp/dockpipe-system-race.log`,
`/tmp/dockpipe-system-ctest.log`, `/tmp/dockpipe-system-final-ui.log`,
`/tmp/dockpipe-system-live.json`, `/tmp/dockpipe-system-overview.png`.
Generated build products remain in the existing launcher build directory and
`/tmp`; the new preview CLI is `/tmp/dockpipe-system-cli`. No commit, push,
remote opt-in, service replacement or infrastructure mutation was performed.

## Windows whole-system collector follow-up

The Windows dashboard previously received the unsupported-platform resource
fallback. A native collector now supplies CPU counters, physical memory,
system-volume capacity, non-loopback network counters and uptime through the
existing CLI snapshot. Kernel boot identity protects rate baselines across
short-lived collectors. Enumeration, retries and counter arithmetic are bounded;
individual API failures retain nullable metrics and notices. CPU is withheld on
multiple processor groups, and commit limits are not mislabeled as swap.

The dashboard regression covers two-sample Windows-style CPU/network rates,
reboot reset, and absent Unix swap/load fields. The focused dashboard widget
CTest and execution-observer tests pass locally. Windows CLI cross-compilation
passes; native Windows collection and UI validation remain pending. The added
native collector smoke test runs with the existing Windows runtime Go CI suite.
Changes remain local with the dashboard fixes; no publication occurred.

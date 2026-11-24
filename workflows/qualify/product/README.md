# Local product integration qualification

Run this workflow explicitly on operator-selected test environments. It is heavy
and deliberately has no CI registration. Prefer disposable VMs and generated
fixtures; existing paired environments require an explicit target selection.
Keep guest disks, firmware, TPM and installed product identities together.

The registry inventories product features, platform profiles, scenarios and
named assertions. The target is at least 90% of applicable registered cases and
100% of critical cases, with no failures. That measures observed regression
coverage; it is not a promise to detect 90% of all possible bugs. Component,
installed end-to-end, guided UI and human checks have separate counts. Missing
drivers, platforms, permissions, devices and human observations remain unpassed.

The [platform expansion matrix](PLATFORM-MATRIX.md) lists the pinned targets,
their installation paths, and the required upgrade, workflow, and desktop
observations. Windows Server is the final lane and uses an operator-provided VM.
The [remote and Browser role matrix](REMOTE-MATRIX.md) expands the existing
features into installed operator, viewer, sender, worker, and service lanes.
The [private VM image cache](IMAGE-CACHE.md) records the image metadata, upload
path and read-only download and overlay verification commands.

This implementation is being qualified incrementally. The registry is broader
than the implemented drivers. An inventory or successful runner contract test
does not establish product acceptance. In particular, the Linux portable driver
currently implements installation, init, workflow discovery, host-workflow and
file capability happy paths. A separate package-authoring lane compiles real
resolver and workflow archives, runs a package-owned test, exports a store,
invokes a packaged child with typed inputs twice, and checks permitted and
forbidden source cloning. The
Windows and Linux VM adapters and installed Flatpak lane have their own
prerequisites and documented limits. Unimplemented cases produce warnings rather
than a green full-product result.

## Run

Select an absolute Dockpipe executable and a private plan conforming to
[DRIVER.md](DRIVER.md). Each target pins the installed product artifact. Each
driver pins its entrypoint and imported helper files; platform configuration
supplies explicit media, installation and transport references. Credentials must
be private file references, never literal plan values.

```bash
python3 workflows/qualify/product/assets/scripts/prepare-plan.py \
  --portable-artifact "$DOCKPIPE_BIN" --run-id local-portable-01 \
  --output /absolute/private/plan.json
"$DOCKPIPE_BIN" workflow validate workflows/qualify/product/config.yml
"$DOCKPIPE_BIN" --workflow-file "$PWD/workflows/qualify/product/config.yml" -- \
  --plan /absolute/private/plan.json --output /absolute/private/fresh-run
```

The direct runner accepts `--list` without starting a test. Direct exit status is
0 only for full-product qualification, 1 for a failure or invalid evidence, 2
for incomplete coverage, and 130 for interruption. The Dockpipe host step reports
a nonzero child result as workflow failure; consult `result.json` to distinguish
incomplete coverage from an observed product failure.

Outputs include the exact captured plan and registry, private JSONL events,
per-driver requests/process logs/hashed evidence, and `result.json`. Plans and
outputs belong outside source control. Preserve artifacts from failed or
interrupted effectful operations until the owning provider reconciles the exact
resource references. Killing a driver process does not prove that a VM or remote
resource has stopped.

For an actual container lane, add `--docker-image sha256:<cached-image-id>` when
preparing the Linux portable plan. It requires the selected local Docker socket
and an already cached image containing Bash and the fixture's basic tools; it
does not pull images. The generated workflow disables shared data volumes and
declares offline networking, a read-only root, non-root execution and resource
limits. Independent daemon inspection must confirm those settings and the exact
container's removal before that case passes. An unavailable daemon or image is a
missing prerequisite, never a passing substitute.

Add `--docker-detach` with `--docker-image` to qualify the lifecycle happy case
through Docker. The driver checks that Dockpipe returns while the exact container
is still running, reads its log before releasing the generated workload, and
verifies the result, Docker's exit and removal events, and the observed process
identities. Use a local Linux daemon with guest-visible process identities.
Host lifecycle failure and cancellation still run through the host driver.

An explicit private plan can select `drivers/docker_cancel.py` for
`workflow.containers/cancellation/component`. Use the same `artifact`, `image`,
and `endpoint` fields as the offline Docker driver. Pin the contract and process
helpers and both `drivers/docker.py` and `drivers/docker_detach.py` as dependencies.
Run it as a non-root user on an empty disposable daemon with the image already
cached. The driver sends SIGTERM to the attached Dockpipe process only after it
observes the exact container and its worker identities. The receipt requires
normal signal forwarding, one start, exit 143 in Docker, container removal,
and disappearance of the observed worker identities. If any of those
observations is missing, preserve the evidence for reconciliation.

An explicit private plan can select `drivers/docker_recovery.py` for
`workflow.containers/restart-recovery/component` under the same daemon and image
requirements. Pin `contracts.py`, `process.py`, and `drivers/docker.py`,
`drivers/docker_detach.py`, `drivers/docker_cancel.py`, and
`drivers/lifecycle.py` as dependencies. The driver binds a project workspace
and stops the exact attached workflow's Dockpipe owner with SIGKILL. A fresh
`dockpipe runs observe` process must report the original execution and live
container without replay. After the original workload finishes, Docker must
report exit zero and one lifecycle. A final observer must retain the same
execution as unverified, and the driver removes the exact stopped container.

An amd64 Ubuntu 24.04 VM with a dedicated Docker Engine 29.9.3 passed both
component cases and the installed offline and detach cases with the pinned CLI
candidate. The daemon started and ended with no containers. The VM shut down
normally, the cached base digest matched, and the shared host daemon's existing
container IDs and states matched before and after the run. The installed GUI
and remote container-worker cases require their own evidence.

## Resume and human checks

Add `--resume` with the same plan, registry, runner sources and output directory. Completed
receipts and evidence are revalidated without repeating their actions. A changed
pin, missing receipt or incomplete invocation is rejected and retained. The
runner never silently repeats an existing uncertain installation or remote
operation. Select a new explicit invocation only after owning-driver read-back
and reconciliation.

`drivers/manual.py` creates human packets for requested human-layer cases.
Without actual observations they remain `manual_pending`. An explicit private
record can bind individual results and hashed evidence to the exact
run/plan/registry/product. Evidence semantics still require human review; the
runner validates bindings and completeness, not the truth of a person's claim.
It does not convert a blanket approval into feature passes.

Prefer real device passthrough into a VM for FIDO2, GPU and display tests where
that validly exercises the behavior. Only a check requiring a physical host
behavior, such as host suspend/keep-awake, requires bare metal. Unavailable
hardware produces `WARNING`/`not_run` with the precise capability; do not omit it
from the coverage denominator or infer its behavior from a guest simulation.

The host lifecycle lane exercises actual async workers, project-scoped run
observations/logs, failure and signal cancellation, with exact process-identity
cleanup. Host background execution does not qualify the container-only detach
contract, so its installed happy case remains blocked on that assertion.
The separate observability lane uses the same owned fixture mechanics to check
three bounded resource refreshes, exact project/process/kernel identity,
independent CPU/memory/disk totals, and advancing capture/uptime/CPU counters.
It reads aggregate system metrics and scoped fixture processes, without a global
process scan or command/environment capture.

The default Linux portable plan also exercises a synchronous host-step failure.
The earlier step must produce its marker, the failing script must report its exit
status and corrective diagnostic, and the next step must leave no output.
The driver verifies the generated project inputs and adjacent canary by digest.
A separate synchronous cancellation case sends SIGTERM only to the Dockpipe CLI
after the public observer reports the original running host step. It verifies
the fixture's signal acknowledgment, the exact worker and child exit, the same
terminal execution on two fresh observations, and the absence of the next step.
The restart-recovery case kills the original CLI owner and observes the same
execution through parent-death cleanup. The fixture must acknowledge SIGTERM and
leave no owned processes or replay. The final record stays unverified because
the crashed owner could no longer record the exit.

The initialization failure lane runs the selected CLI in a generated read-only
project as a non-root Linux user. It requires a normal permission refusal naming
that project, unchanged project and adjacent canary bytes, no fallback project
in the private home, and unchanged executable identities. Crashes and deadlines
cannot satisfy the expected-failure assertion.

The policy refusal lane checks authored workflow and step security through both
file and named workflow selection with dependency compilation disabled. Missing
compiled policy metadata must produce the owning actionable refusal before any
Docker invocation or connection. A private rejecting executable and Unix socket
isolate the test from the real daemon; a separate positive control checks the
connection counter. Task-local cache and failure-event files are recorded, while
authored sources and the adjacent canary must remain unchanged. A denied Unix
socket prerequisite produces a warning and blocked result.

For the package install/removal lane, pass both `--catalog-certificate` and
`--catalog-key` to the plan preparer. Use generated fixture-only TLS files: the
certificate must identify `127.0.0.1`, and the private key must have owner-only
permissions. The plan pins both files; the driver trusts that certificate only
inside its private CLI environment and serves generated archives on loopback.
It checks catalog/install digests, checksum refusal before archive download,
optional package removal and execution of a different retained package. It
stops the exact fixture listener and leaves generated evidence outside the repo.
It does not qualify package update/rollback or physical distribution installers.

The separate opt-in `drivers/package_recovery.py` implements only the existing
`packages.lifecycle/restart-recovery/component` case. It uses the same pinned CLI
and generated TLS configuration fields in an explicitly authored private plan.
After baseline installs and one exact workflow execution, its listener truncates
one pinned reinstall download with normal EOF. Recovery is allowed only after a
normal actionable CLI refusal, one observed archive request, unchanged installed
and bystander bytes and no staging residue. One fresh CLI process then reinstalls
the same name/version/digest and executes it once. Marker counts and final store
bytes detect duplicate effects. This is interrupted reinstall recovery, not a
version update, rollback, promotion or automatic retry. It is not selected by the
default preparer; source controls alone do not qualify the actual scenario.

`drivers/file_failures.py` separately selects only
`files.workspace/failure/component` in an explicit private plan. Its pinned CLI
first reads a generated file and confirms exact bytes/revision. The driver then
changes that generated revision and submits stale-write, parent-path escape and
generated symlink-write requests through `files --request`. Every refusal must
be normal exit 1 with its owning actionable diagnostic; crashes, deadlines and
log failures cannot pass. Exact owned-file, bystander, link and adjacent-canary
snapshots must remain unchanged after each refusal. An uncertain result stops
remaining probes. This opt-in component case does not qualify UI editing,
cancellation, remote files or document-portal access.

Add `--sdk-go /absolute/pinned/go` for the PipeLang SDK lane. The preparer captures
the owning repository's module version/checksum; the driver independently reads
that binding from the selected executable and rejects SDK replacements. Actual
CLI checks cover deterministic typed compilation, materialized fields, public
method results, structured syntax errors and private-method refusal. A different
release needs its own intended SDK pins in a private plan. The inspector reads
build metadata; it does not compile or fetch a language implementation.

Add `--git-fixture-manifest /absolute/producer/manifest.json` to register the
generated project Git lane. First run the explicitly manual
[fixture producer](fixtures/workspace-git/README.md) with pinned Go/Git tools in
a new private project; it has no CI registration. Plan preparation pins its
manifest and verifies the referenced broker and Git executable digests. The
actual selected CLI then reviews and commits only generated paths, preserves
unrelated index and draft work, and rejects a stale reviewed snapshot without
changing HEAD or content. Cancellation stays visibly `blocked` until a real
owned blocking Git child and exact signal/cleanup fixture is qualified. Omitting
this option leaves the existing default lanes and their dependencies unchanged.

Package-owned GUI qualification can consume the separately exported
[desktop fixture artifact](fixtures/desktop/README.md). Its pinned archive
contains the existing Linux desktop and accessibility helpers, avoiding a second
launcher or an implicit dependency on this checkout. Exporting it starts no GUI
or guest and provides no product coverage. The owning package must supply its
actions, fresh observations and cleanup evidence.

## Known regression targets

- [#127](https://github.com/Dockpipe-Industries/dockpipe/issues/127) is confirmed
  ready for release. Verify the selected installed build initializes the chosen
  directory, handles an initially empty workflows folder, discovers workflows
  and preserves the user's home directory. A source test alone does not qualify
  a shipped Flatpak.
- [#109](https://github.com/Dockpipe-Industries/dockpipe/issues/109): exercise the
  installed Flatpak CLI and normal GUI launcher.
- [#137](https://github.com/Dockpipe-Industries/dockpipe/issues/137): fresh init
  must not require an implicit vault; qualify document portal and linked host
  paths without weakening durable-root ownership checks. The critical
  `files.document-portal` cases require real GUI selection, cancellation and
  retained access after restart. A direct host project mount or a synthetic
  filesystem test does not pass them. Missing `document_portal` capability
  remains an explicit unpassed warning; the live driver is still pending.
- [#136](https://github.com/Dockpipe-Industries/dockpipe/issues/136): distinguish
  sandbox observations from explicitly authorized host observations.

Bazzite has a separate Flatpak profile. A Flatpak run on an Ubuntu host does not
qualify Bazzite portals or SELinux behavior. Unsupported backend roles, including
currently absent remote desktop sender/viewer combinations, remain explicit
registry exclusions rather than passing placeholders.

The remote viewer has separate component and installed-GUI happy cases. The
provider-owned live workflow exercises an exact contained DisplaySDK client,
generated-only fixture input and independent original-reference cleanup. Its
three component assertions do not qualify the normal Dockpipe GUI. The generic
package-workflow bridge keeps provider/transport policy with its owning package;
live operator targets, paired references and artifact pins stay in private plans.

## Runner validation

```bash
python3 -m unittest discover -s workflows/qualify/product/tests -v
```

These tests challenge evidence admission, process bounds, missing prerequisites,
nonzero driver results and safe resume. Protocol fixtures used in runner tests
are never registered as product qualification drivers.

## Explicit View-only expiry lane

`remote.desktop.viewer.lease-expiry` is a separate critical installed-GUI failure
case. It does not pass the viewer happy case, generated input, clipboard or sender
availability. Select it only with `--remote-expiry-plan ABS` and
`--remote-expiry-plan-sha256 SHA` when preparing a product plan. The private expiry
plan must use the same fresh run ID and captured registry as the product request.
Without `--remote-expiry-execute-reviewed`, its receipt remains `not_run` and no
GUI or provider operation starts. Matching hashes do not replace review of the
specific target and effects. This lane is never enabled in CI.

The thin driver delegates normal installed-GUI orchestration to the owning
Flatpak workflow assets. The selected provider workflow archive must contain its
versioned `--reconcile` entrypoint. That entrypoint owns native admission,
read-only prerequisite checks and exact-reference postflight; it never closes a
desktop, restarts a service or unlocks authority. Missing or locked prerequisites
remain `blocked`. A failed, cancelled, late or unknown effect is not retried.

A passing receipt requires fresh evidence of the entire nonce in the actual
installed viewer, the original 100ms capture and observed-stop-within-six-seconds
limits, unassisted Core/native closure before the unchanged 18-second watchdog,
normal GUI exit and journal clearing, and independent exact process/fixture/
endpoint closure. Retained directories are unpassed cleanup, never recursively
deleted by the adapter. Provider PID/catalog churn is recorded separately and
cannot imply availability acceptance. Earlier supplementary receipts remain
historical evidence and cannot be imported into this product case.

# Local product qualification drivers

This is an explicitly invoked local workflow. Do not register it in CI. Drivers
test installed product behavior in selected environments; they do not implement
runtime, package, resolver or guest provisioning behavior a second time.

Each driver accepts `--request /absolute/request.json --output /absolute/output`.
The output directory is invocation-owned. It writes `result.json` using
`dockpipe.product-integration.lane/v1`. The shared runner supplies this request:

```json
{
  "schema": "dockpipe.product-integration.request/v1",
  "run_id": "operator-selected-run",
  "plan_sha256": "<64 lowercase hex characters>",
  "registry_sha256": "<64 lowercase hex characters>",
  "target": {
    "id": "linux-desktop",
    "platform": "linux-amd64-deb-ubuntu24.04",
    "kind": "vm",
    "artifact_sha256": "<selected installed product artifact digest>"
  },
  "configuration": {},
  "cases": [
    {
      "id": "workflow.discovery",
      "scenario": "happy",
      "layer": "installed_e2e",
      "required_assertions": [
        "authored-workflow-discovered",
        "installed-workflow-discovered",
        "declared-inputs-preserved"
      ]
    }
  ]
}
```

`configuration` is driver-owned, strictly validated input. It supplies explicit
artifact/media pins, isolated paths and transport selections. Credentials are
private file references, never literal plan values or logged command arguments.
For VM lifecycle checks, distinguish the GUI host and guest in configuration and
observed facts. A Linux host installing Windows is not native Windows execution.

Plan driver `entrypoint` and dependency `path` references default to this product
workflow directory. Use `repo://workflows/...` for an existing owning workflow's
entrypoint or helper elsewhere in this repository, retaining its source digest.
`product://...` is the explicit local form. Both selectors enforce canonical
contained paths without links; external filesystem entrypoints are rejected.
Keep provider policy with its owning workflow instead of copying it into this
runner solely to satisfy a path restriction.

The result repeats the schema bindings above, records observed target facts, and
contains only requested cases. Each case has `id`, `scenario`, `layer`, `status`,
`reason`, `assertions` and `evidence`. Status is one of `passed`, `failed`,
`blocked`, `not_run`, or `manual_pending`. An assertion contains `id`, `status`
(`passed` or `failed`) and a bounded `detail`. Evidence entries contain a relative
path inside the invocation output and its SHA-256 digest. A passed case requires
every requested assertion to pass and evidence of the actual observation.

Use the following layers accurately:

- `component`: executable contract/fault-injection tests on the selected platform.
- `installed_e2e`: normal installed CLI/GUI and real runtime/provider behavior.
- `guided_ui`: observable UI actions driven by an agent or operator.
- `human`: hardware or OS interaction requiring a person, with recorded evidence.

The runner keeps component and installed coverage separate. A fixture's window,
cross-build, startup marker or suite exit code cannot qualify another feature.
Report expected unsupported combinations explicitly in the registry; unavailable
prerequisites stay in the coverage denominator. Never manufacture a passing row.

Prefer VMs. Declare `bare_metal` only for a specific physical behavior that a VM
cannot validly reproduce. Missing hardware is a `WARNING` and `not_run`, with the
exact OS/device/check requirement in the result. Required human actions remain
`manual_pending` until their observations and evidence exist.

Drivers must use fresh fixture workspaces and preserve unrelated state. VM
install, start, embedding, stop and disk-only restart use the owning installed
package's public GUI/workflow contracts. Keep guest identity, firmware, TPM and
disk coherent. Serialize memory-intensive guests, record resources, and refuse
insufficient free space or ambiguous ownership. Preserve uncertain effects and
original references for read-back; never silently retry an effectful operation.

For remote cases, record the exact paired identity and provider artifact. Use
generated fixtures, bounded input and original-reference cleanup. Do not capture
unrelated desktop pixels or use live clipboard content as evidence.

## Pinned Linux package upgrade containers

`drivers/package_upgrade.py` handles the Alpine, Rocky Linux, Amazon Linux and
Red Hat UBI container profiles in the [platform matrix](PLATFORM-MATRIX.md). Select
`install.cli` and `install.upgrade` with scenario `happy` and layer
`installed_e2e`. Pin `contracts.py`, `process.py`, `drivers/docker.py` and
`fixtures/package-upgrade/cli.sh` as dependencies.

Prepare the pair of installed cases with one driver and the registered image pin:

```bash
python3 workflows/qualify/product/assets/scripts/prepare-plan.py \
  --package-profile rhel-ubi-9.6-amd64 \
  --package-baseline /absolute/private/baseline.rpm --baseline-version 0.6.39 \
  --package-candidate /absolute/private/candidate.rpm --expected-version 0.6.44 \
  --run-id ubi-installed-packages --output /absolute/private/ubi-plan.json
python3 workflows/qualify/product/assets/scripts/qualify.py \
  --plan /absolute/private/ubi-plan.json --output /absolute/private/ubi-result
```

Run plans for independent container profiles concurrently with distinct run
IDs and output directories. Each driver owns its own container and logs and
binds its input packages read-only. Each plan still reports its actual coverage,
including checks beyond the installed package suite.

Set `configuration.image` to the full cached Docker image ID and
`configuration.endpoint` to `unix:///var/run/docker.sock`. Supply the absolute
candidate and prior package files as `artifact` and `prior_artifact`, their
numeric versions as `version` and `prior_version`, and the older package hash
as `prior_artifact_sha256`. The candidate hash belongs in the target's
`artifact_sha256`. The package manager may retrieve dependencies from the
target's release repositories. The driver checks the running release, verifies
package ownership and checksums, then runs workflows as an unprivileged user
after a clean install and on both sides of the in-place upgrade. The receipt
requires cleanup of the exact disposable container.

For UBI, pull the amd64 manifest digest recorded in the registry. The fixture
uses only the signed public UBI 9 BaseOS and AppStream repositories. These
repositories track the major release, so keep the final installed RPM inventory
from the driver log with the 9.6 image digest. The RHEL VM has its own media
and qualification lane.

## Generated local Git workspace lane

`drivers/git_workspace.py` accepts the local Linux portable target with
`configuration.artifact`, `fixture_manifest` and `fixture_manifest_sha256`.
The manifest comes from the manual `fixtures/workspace-git/config.yml` producer
and binds its broker executable, exact Git executable and compiler/source
inventory. The driver validates broker/Git digests before effects and again
after execution; plan preparation also checks those references. Include the
driver, contracts/process helpers and the producer's build script, overlay test
and workflow file in the plan's dependency pins.

The broker reuses owning Go test helpers only to seed its private
`testing.T.TempDir` repository and read final Git state. After readiness, real
review/compare/selected-commit/stale-refusal actions use the selected product's
public `git --request` API. Normal broker exit must remove the repository and
recorded original kernel identities must be absent. Include cancellation to
exercise a real `git status` child waiting on a generated clean filter. The
driver signals only the exact Dockpipe process and requires a normal cancellation
diagnostic, closure of the observed process identities, one filter start, and
equal fresh public snapshots and worktree bytes around the interrupted review.
This lane is intended for local Linux portable execution in generated repositories.

## Package-owned qualification workflow

`drivers/package_workflow.py` forwards the request to an exact compiled workflow
archive through an explicitly pinned CLI. Configure `cli`, `cli_sha256`,
`workflow_archive`, `workflow_archive_sha256`, `workflow_member`,
`provider_configuration`, `workflow_arguments` and `timeout_seconds`. Both files
are copied into the private invocation and verified before and after execution.
The CLI pin is separate from the target product artifact; the owning provider
must establish the target identity for each claimed case.

The owning workflow accepts `--request ABS --output FRESH_ABS`, receives
`provider_configuration` as its request's `configuration`, and writes the same
lane receipt protocol. Keep provider policy, target admission, actual probes and
external-resource cleanup in that owning package. The bridge validates bindings
and evidence, prefixes nested evidence paths, and refuses to forward passes from
a failed or timed-out workflow. Reserve three of the 64 evidence slots per case
for bridge bindings. Missing cases remain unpassed.

Arguments are an argv list, without shell evaluation. A whole argument equal to
`@request-sha256` becomes the digest of the exact captured provider request. This
supports an owner's explicit execution guard; matching a digest does not create
authorization. Additional arguments cannot replace the bound request or output.
The default owning workflow should prepare/check without live effects, with live
execution selected explicitly in the operator's captured plan. This bridge does
not treat a standalone provider probe as normal installed GUI acceptance.

`drivers/remote_expiry.py` delegates only the distinct
`remote.desktop.viewer.lease-expiry` / `failure` / `installed_e2e` case. Its exact
configuration is `expiry_plan`, `expiry_plan_sha256`, `execute_reviewed`, `observer_context` and
`host_profile` (`ID` and `VERSION_ID` from the executing Linux host). The target
is the actual Flatpak viewer bundle; the paired sender is a prerequisite, not a
second platform credited by this receipt. Bazzite attribution requires an actual
Bazzite host. Imported historical output paths are not configuration options.

The driver and owning Flatpak assets are pinned as repository dependencies. The
expiry plan independently pins the GUI, installed CLI, module, provider archive,
native fixture, source closure and original effects. Semantic checks bind the
full original Core reference and fixture, capture-time window/PID geometry,
nonce, original clocks, unassisted watchdog, normal journal clearing and exact
independent cleanup. A top-level `passed` flag alone is insufficient. The owning
provider's read-only reconciliation protocol is versioned in its archive;
provider admission policy is not reimplemented in this driver or the engine.

The expiry configuration also pins `observer_context` (`pid_namespace`, `mount_namespace` and
`procfs_device`) from the explicitly reviewed executing host. Prepare this lane
in that host context. A different namespace/procfs blocks before effects. The GUI
runner records its own observation context before launch; independent process
absence checks must match that record and the current invocation. A hidden
sandbox cannot silently reinterpret another namespace's host PID inventory.
Broken links at runtime directories or display endpoints count as retained
entries (`lexists`), not successful absence.

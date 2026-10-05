## Research Update — First Design Slice (2026-07-13)

### Local Evidence And Precise Gap

| Existing flow | What it owns | Gap to a repository Dev Container |
| --- | --- | --- |
| `packages/ide/resolvers/vscode` | A disposable `dockpipe-base-dev` container mounted at `/work`, then a host VS Code Dev Containers URI. | Does not read `.devcontainer` or use its image, Compose service, features, mounts, lifecycle commands, or `remoteUser`. |
| `packages/ide/resolvers/cursor-dev` | The same DockPipe-authored base image/container and best-effort editor-attachment/idle heuristics. | It is not a native Cursor/Dev Container lifecycle and its attachment heuristics must not be reused as an ownership signal. |
| `packages/pipeon/resolvers/pipeon-dev-stack` | A Pipeon-scoped Compose control plane, code-server container, host MCP bridge, state directory, labels, and teardown. | It is Pipeon's product stack, not the workspace environment. It must neither replace nor be torn down with a discovered Dev Container. |
| DorkPipe provider pools | Bounded, DorkPipe-owned provider workers exposed through CLI/MCP and consumed by Pipeon. | A ready Dev Container is not a provider-pool worker or generic runtime target without a later explicit resolver contract. |

The exact missing capability is therefore read-only discovery and status of a *repository-owned
definition*, followed only by an explicit, governed request to use it. Today no package reads a
selected definition or reports the corresponding container identity/state. Existing Pipeon stack
labels and cleanup apply only to Pipeon resources; existing IDE containers are DockPipe-authored,
disposable compatibility sessions. Neither is evidence that a user's native Dev Container is
DockPipe-owned.

### Upstream CLI And Docker Evidence

Use the reference [`@devcontainers/cli`](https://github.com/devcontainers/cli) as the initial
adapter boundary, pinned and compatibility-tested by the future package. Its documented/sourced
operations are:

| Operation | Use in this design | Machine-readable fact |
| --- | --- | --- |
| `read-configuration` | Read a selected definition before any action. | Structured configuration result; it can resolve the selected file. |
| `up` | Future explicit prepare/start only. | Final JSON includes `outcome`, container id, remote user, and remote workspace folder; `--log-format json` makes progress parseable. |
| `run-user-commands` | Future deliberate lifecycle-hook operation, never an incidental status check. | Final JSON outcome/result; hooks may execute repository-defined commands. |
| `exec` | Future explicit command operation against an already selected/running environment. | Final JSON outcome; the CLI applies Dev Container user/environment settings. |
| `build` | Future prebuild/rebuild action. | Final JSON outcome/image name. |

The current upstream command surface does **not** list a supported `stop` or `down` command (they
remain unchecked in its status list). Do not promise either as a Dev Container CLI operation. A
future stop/remove implementation may use Docker only after exact managed-session proof; Docker
supports label filtering and JSON `container inspect`, which is appropriate for bounded status and
recovery, not ownership guessing. Sources: [CLI README](https://github.com/devcontainers/cli),
[current CLI options/source](https://github.com/devcontainers/cli/blob/main/src/spec-node/devContainersSpecCLI.ts),
[Docker label filters](https://docs.docker.com/reference/cli/docker/container/ls/), and
[Docker inspect JSON](https://docs.docker.com/reference/cli/docker/container/inspect/).

`--workspace-folder` defaults only to the standard `.devcontainer/devcontainer.json`, then
`.devcontainer.json`; `--config` is the supported explicit alternative path. `--id-label` both
sets labels and queries for an existing container. This supports a stable adapter contract across
Windows, macOS, and Linux where Docker plus the CLI are available, but the upstream install script
is documented only for Linux/macOS; Windows installation/version support must be an explicit
package prerequisite and fixture-tested before it is claimed as turnkey.

### Discovery, Selection, And Status

Discovery is a filesystem-only scan of the workspace root:

1. Include the standard `.devcontainer/devcontainer.json` and legacy `.devcontainer.json` when
   present.
2. Enumerate other JSON/JSONC definitions directly under `.devcontainer/` as candidates, but never
   treat a file as selected merely because it appears first.
3. For every candidate, record a workspace-relative definition reference, display name (if safely
   readable), and content fingerprint. Do not resolve `${localEnv:...}`, secrets, Compose state, or
   lifecycle commands during discovery.
4. Zero candidates is `unavailable`; one candidate becomes the proposed selection; two or more is
   `selection_required`. Non-interactive CLI/MCP calls fail closed with the candidate list until a
   workspace-relative `definition_ref` is supplied.

Status requires an explicit definition reference, then combines read-only selected-definition facts
with Docker inspection of containers whose labels match the workspace/definition identity. Return
`not_created`, `created`, `running`, `stopped`, `ambiguous`, or `unavailable`; do not infer an
editor attachment state. Docker labels are discovery hints, not sufficient ownership proof.

### Ownership, Approval, Cleanup, And Recovery

| Classification | Rule | Allowed automatic action |
| --- | --- | --- |
| `external` | Any discovered/user-started container, including one that happens to carry Dev Container labels but lacks an exact DockPipe session record and DockPipe session label. | Read-only status/log reference only; never stop, remove, rebuild, or adopt for cleanup. |
| `managed` | A future `up` result whose exact container id, selected definition fingerprint, workspace identity, and DockPipe session label were recorded together. | Read-only reconciliation only. |
| `orphan_candidate` | A Docker-labeled prior managed container whose local session record is missing or mismatched. | Report repair options; no automatic cleanup. |
| `ambiguous` | More than one matching container, a changed definition fingerprint, or any missing proof. | Fail closed and require a user selection/repair action. |

Future managed starts should pass a namespaced label such as `com.dockpipe.devcontainer.session`
through the CLI's `--id-label`, while preserving Dev Container labels. The local record must bind
the opaque container id, workspace identity, definition reference/fingerprint, and session id.
Never add a DockPipe label to an existing container solely to “adopt” it. “Use existing” initially
means read-only status and, only after a later explicit product decision, explicit `exec` without
cleanup authority.

Approval classes are: no approval for discovery/status; explicit intent plus approval for image
pull/build, feature installation, Compose create/start, and lifecycle hooks; separate approval for
rebuild; explicit reviewed intent for `exec`; and explicit approval for host editor launch. Stop is
an explicit managed-session action. Remove/down is destructive and requires a stronger confirmation
after exact managed-session proof. On cancellation or crash, retain the session record and report
reconcile/status; only an exact managed record may offer stop/remove repair. No action ever starts a
container during discovery or status.

### Recommended First Vertical Slice

Implement **read-only discovery plus selected-definition status only**. It proves the product seam
without pulling an image, starting Docker Compose, changing a lockfile, or launching an editor.

- Put Dev Container-specific scanning, `read-configuration` adaptation, Docker-label inspection,
  and normalization in a new `packages/ide/resolvers/devcontainer` resolver/package-owned assets.
  Do not add a `src/lib` or `src/cmd` special case.
- Expose its package workflow/command as the sole CLI execution path; the exact friendly command
  name can be chosen later, but it must accept `--workspace` and `--definition-ref` and fail closed
  on multi-definition discovery.
- Add one package-local CLI/MCP operation-result envelope, for example
  `devcontainer.lifecycle.v1`: `request_id`, `workspace_ref`, `definition_ref`,
  `definition_fingerprint`, `operation` (`discover` or `status` in this slice), normalized `state`,
  `ownership`, opaque `environment_ref` when known, safe `summary`, `log_ref`, and
  `next_actions`. Stream only ordered `discovered`, `selection_required`, `status`, `progress`,
  `approval_required`, `completed`, and `failed` events. No raw Docker/CLI command text, secret,
  resolved configuration, or editor-process heuristic crosses the boundary.
- Surface that same package operation through the DorkPipe host MCP bridge. Pipeon maps
  `unavailable` to no card, `selection_required` to a picker, `not_created` to “Dev Container
  available”, and `running`/ownership to status/repair UI. Start, logs, attach, rebuild, and stop
  controls remain disabled or absent until their matching CLI/MCP operations exist. The extension
  must not call Docker or the Dev Container CLI directly.
- Keep provider pools separate. A later resolver contract must prove how a managed Dev Container
  becomes an execution location before provider workers can use it.

Fixture-only validation: standard/legacy/alternate/multiple/malformed definitions; stable candidate
ordering and explicit-selection failures; captured `read-configuration` and Docker inspect/label
JSON for each normalized status/ownership outcome; changed-fingerprint, duplicate-container, and
lost-session recovery cases; CLI/MCP event sequence and redaction assertions; and Pipeon UI mapping
against recorded events. No test may invoke Docker, pull/build an image, run hooks, or require an
editor/account.

### Lifecycle Decisions

1. **Adapter distribution — decided (2026-07-13):** require an installed/pinned Dev Container CLI.
   The first lifecycle contract fixture-verifies the installed version against its pin; it does not
   yet execute the CLI live.
2. **Existing environments — decided (2026-07-13):** first release is managed-only. External or
   user-started containers remain status-only; no `exec`, adoption, labeling, cleanup, or mutation
   is permitted.
3. **Cleanup policy — decided (2026-07-13):** Pipeon close explicitly requests a stop only for an
   exactly proven managed container. The stopped container and its managed session record are
   retained for reuse; remove/down remains a separate destructive action requiring stronger
   confirmation. This is a Dev Container lifecycle request, never an incidental side effect of
   Pipeon stack teardown.
4. **First attachment:** VS Code, Cursor, Pipeon code-server, or status/exec-only; attach remains a
   host action and not proof of lifecycle ownership.
5. **Definition scope:** whether recursive/nonstandard definitions beyond the root and direct
   `.devcontainer/*.json` candidates are a supported product feature; the first slice should not
   guess.


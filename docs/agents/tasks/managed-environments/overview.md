# CLI and GUI Environments

The CLI is the entry point for creating an environment, generating editable
YAML and applying an existing definition. The GUI consumes the same scoped
definition and inventory. Personal definitions use Dockpipe's user data root.
Users select a project path explicitly for shareable YAML. Disks, caches,
credentials and pairing keys stay in their owning runtime or host state.
Project mounts and sync require an explicit choice. Files targets the selected
environment's filesystem and stays independent of the definition location.

The canvas and list must select the same environment and show the same status
and actions. The canvas should show physical host to VM to container, and
physical host directly to container. VM lifecycle operations belong to the
host that runs the VM. Reuse the owner package's VM workflows and runtime
receipts, and Dockpipe's Docker and remote capability contracts. Preserve an
offline entry only when the product has an admitted identity or lifecycle
record for it. Capability actions require the selected runtime, platform,
identity and grants. Supported actions include Terminal, Files, logs, workflows,
Browser, and Remote Desktop on supported graphical sessions.

The target model applies across feature pages. Bare metal and VM guests are
hosts with the full host feature set where their platform, runtime and grants
allow it. A container is its own work target for Terminal, Files, Git, Browser,
workflows and resource monitoring. Containers and Remote Desktop are host
features. Attachment to an owning host is a topology and lifecycle relationship.
Authorized clients can use a paired guest from anywhere its identity and grants
allow. A host's grants do not imply its guest's grants.

Remote host Terminal uses the worker account's shell and starts in its granted
home or selected workspace. The paired worker owns its consent and shell
configuration. The desktop setup can grant a home Terminal while pairing or
enable it for an already paired worker. Source-level broker and real Unix PTY
checks cover multiple granted roots. The Windows host path uses the bundled
native ConPTY provider. A physical Windows fixture verified provider input,
output, resize and cleanup. A signed Windows 11 test MSI installed the full
desktop app and its native Terminal helper. Mouse and keyboard input in the
installed app ran a real local shell command. A signed Windows 11 upgrade
also verified the installed background worker with no visible console.
The Linux desktop app opened its paired Home Terminal and received real
PowerShell output after the worker restarted.

Use the existing host-authorized guest enrollment contract. The paired host
must have explicit provisioning and delegation authority. The guest produces
its own identity and receives only the allowed grants. Reboot retains that
identity. Recreating the guest requires a new identity. Preserve existing
pairings and the ordinary physical pairing path. The current guest-enrollment
implementation grants workflow delivery within a scoped run. Extend tested
delegation where needed for guest tools rather than inferring authority from
topology or from a host's ordinary workflow grant.

An optional storage configuration reference selects an installed resolver
capability. The first storage source is R2 through the existing verified
content-addressed image cache. Verify or reuse the immutable base before
creating a local writable overlay on the VM host. Keep credentials in scoped
references. The public checkout may contain logical profile names. Resolve
the private endpoint, bucket, object or snapshot locator, and authorization
from 1Password through the existing resolver facilities on the executing host.
Keep resolved values and private references out of the public repository,
exported definitions, logs and qualification receipts. Snapshot uploads are
explicit and follow the image license and authorization for the selected
snapshot. Live cloud block storage is a
separate resolver capability. The qualification harness should use the same
environment and resolver contract after the product path is verified.

The initial CLI definition path generates, saves, reads, and applies personal
and explicit project YAML through normal workflow resolution. A local fixture
verified both paths and strict validation. The same `environment apply` entry
point ran the qualification image boot workflow against the pinned public
Ubuntu 24.04 base. The offline guest wrote to its fresh overlay, shut down,
and left the base SHA-256 unchanged. The storage resolver is a self-contained
Go executable in the owning resolver package. Offline tests verified signed
GETs, size and digest checks, cold restore, warm reuse, and concurrent cache
publication with synthetic objects. The rebuilt CLI invoked the packaged
executable and QEMU wrote to a separate overlay while the base digest stayed
unchanged. The resolver currently needs POSIX cache ownership and link
protections. Live 1Password-backed R2 and the qualification harness migration
remain open.

The shared definitions command emits saved personal and explicitly selected
project definitions for both GUI views. The Qt build and focused page and
navigation tests passed, including selection across views and opening the
selected YAML. Native topology, remote host provisioning and enrollment, and
installed GUI proof remain open.

Kubernetes pod environments are deferred. CLI and YAML requests for a pod kind
return an unsupported error, and Canvas and List omit those definitions.

First apply assigns a private VM instance ID to the saved definition. The
QEMU package uses that ID for a fresh durable cohort and retains a writable
overlay for a verified managed image. Tests covered two definitions with the
same base, a repeated apply, concurrent creation, a wrong backing path, and
QEMU's write lock. The VM provider carries the instance ID in its lifecycle
and inventory records. The ID correlates an observed VM with its definition.
Remote authority still requires its own delegated grant.

For local testing, the CLI and GUI now use the same apply path and runtime
status. VM process observations are pinned to the provider's recorded
workspace and run. Local Docker definitions support a direct YAML container
configuration or a workflow that explicitly labels the container with the
instance ID. The Add Container form saves the direct configuration and can
start it immediately. The CLI expands its container presets into the same YAML.
A Linux desktop preview created an Ubuntu container and observed it running.
Local CLI validation verified container reuse, restart, restricted settings,
loopback port binding, and a persistent Docker volume. Remote placement,
guest Docker, and installed platform coverage remain open.
The native Docker apply path now binds the environment name, instance ID,
settings and selected host to the invocation authority before looking up
Docker. Focused tests cover remote authority denial and binding changes when
the settings change. The paired-worker dispatch and its provisioning grant
remain open.

Direct VM definitions declare the OS, CPUs, RAM, new disk size, and optional
installer ISO. The GUI saves the same YAML that CLI presets generate. Apply
uses the installed VM resolver and a persistent disk in the local instance
state directory. A Linux QEMU fixture verified the selected CPU and RAM values
and exact new disk capacity. Full OS installation and platform parity remain
separate qualification requirements.

The original Environments backlog record was not recovered. The user-confirmed
requirements above are the working contract.

Author new qualification policy and assertions in the source-owned PipeLang
product harness. Use native adapters for platform effects and preserve the
existing product registry, assertion identities and receipt checks when moving
an active qualification driver. Keep new Environments checks out of the legacy
Python drivers.

Installed acceptance requires creating a VM through the CLI on an authorized
remote host, matching its identity in both GUI views, using an admitted guest
feature, and observing guest offline while the host remains reachable. Verify
Docker on a physical host and inside a VM where supported. Verify R2 base
selection and reuse, an unchanged base, and a writable local overlay through the
same product path. Keep the existing platform qualification scope and order.

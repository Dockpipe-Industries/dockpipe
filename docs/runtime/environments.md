# Environments

An environment definition identifies a VM or container. A VM can declare its
guest OS and hardware, and a Docker container can declare its image and settings.
A workflow-backed definition uses
normal project and package workflow resolution, with the workflow provider
owning its runtime settings, inputs and lifecycle.
Kubernetes pod environments are unsupported and excluded from Canvas and List.
An explicit pod kind in a CLI command or YAML definition returns an unsupported
error. The definitions inventory reports those files as invalid with the
`unsupported-kind` reason.

```bash
dockpipe environment generate --name dev-vm --kind vm --workflow linux-vm
dockpipe environment generate --name dev-vm --kind vm --workflow linux-vm --file ./dev-vm.yaml
dockpipe environment apply --file ./dev-vm.yaml -- <provider-arguments>
```

`generate` prints editable YAML by default. Use `--personal` to save to the
Dockpipe user scope or `--file` to choose a path, including a path in a project
you share. `create` saves the definition and starts the environment.
It saves in the user scope by default. Generation and creation refuse to
overwrite an existing definition. Use `show --name NAME` for a personal
definition or `show --file PATH` for a definition at an explicit location.

```yaml
schema: dockpipe.environment/v1
name: dev-vm
kind: vm
provision:
  workflow: linux-vm
```

Personal definitions use `environments/<name>.yaml` under Dockpipe's normal
user data root. `DOCKPIPE_GLOBAL_ROOT` selects that root for a custom layout.
An explicit definition file uses its directory for workflow resolution unless
you pass `--workdir`. A personal definition uses the current working directory
unless you pass `--workdir`.

`dockpipe environment definitions` returns the saved personal definitions as
JSON. Repeat `--file PATH` to include specific project definitions in the same
result. Each entry includes its scope and either a validated definition or a
file-specific error. These entries describe saved configuration.

Select a saved definition in Canvas or List and use **Apply** to run
the same action through the GUI. Personal definitions use the current project
while explicit files use their containing directory.

**Environments** is the entry point for paired hosts, supported VM inventory,
and saved definitions. In **Add Environment**, choose **Remote machine** to
start pairing, choose **Container** to configure a Docker container, choose
**Virtual machine** to configure a guest and its hardware, or import existing YAML.
Selecting an environment in Canvas or List
shows its details and available actions. **Open VM console** opens the provider's native display
for a selected running local VM. This includes the guest boot or installer
screen when the VM provider supplies a display. An enrolled guest can also
appear in **Remote Desktop** when its guest service and desktop grants permit it.

## Virtual machines

In **Add Virtual Machine**, select Ubuntu, Debian, Fedora, Other Linux, or
Windows 11. The OS selection fills in CPU, RAM, and disk defaults. You can edit
the hardware and choose an installer ISO you already have. **Save Definition**
saves the YAML without starting a guest. **Create VM** saves the YAML and opens
the installer through the local VM provider.

```bash
dockpipe environment presets --kind vm
dockpipe environment generate --name dev-vm --kind vm --preset ubuntu \
  --cpus 4 --memory-gib 8 --disk-gib 80 --installer-iso ./media/ubuntu.iso --file ./dev-vm.yaml
dockpipe environment apply --file ./dev-vm.yaml
```

```yaml
schema: dockpipe.environment/v1
name: dev-vm
kind: vm
vm:
  os: ubuntu
  cpus: 4
  memory_gib: 8
  disk_gib: 80
  installer_iso: ./media/ubuntu.iso
```

Relative ISO paths resolve from the definition file. The OS field selects
hardware defaults and guest settings. Supply matching installer media and any
required license. Dockpipe creates the persistent disk in host-owned state.
Disk size applies to the first creation. After installation, remove
`installer_iso` to start from the installed disk on subsequent applies. If more
than one VM resolver is installed, set `vm.resolver` to select one. VM launch
depends on the provider’s host support and prerequisites.

## Docker containers

In **Add Container**, choose a base OS, app image or custom Docker image. Pick
startup, networking and security on **General**. **Advanced** includes the
container user, a read-only root filesystem, a persistent volume, restart policy
and a TCP port published on this computer’s loopback address. **Create Container**
saves YAML and starts the container. **Save Definition** saves YAML for review or
later use. Both use the same definition as the CLI.

```bash
dockpipe environment presets
dockpipe environment generate --name dev-shell --kind container --preset ubuntu \
  --network none --security restricted --user non-root --file ./dev-shell.yaml
dockpipe environment apply --file ./dev-shell.yaml
```

```yaml
schema: dockpipe.environment/v1
name: dev-shell
kind: container
container:
  image: ubuntu:24.04
  startup: keep-running
  network: none
  security: restricted
  user: non-root
```

Presets expand into explicit YAML. Use `--image` for a custom reference and
`--startup keep-running` for shell images that provide `/bin/sleep`. With the
default `image` startup, Docker runs the image’s own command. Docker runs Linux
containers on the current computer. Standard security prevents privilege
escalation and Restricted also drops all Linux capabilities. Host folders
are outside the direct container settings.

Apply starts the matching container or creates it on first use. If its
creation settings change, save a new environment to create a separate
container. A persistent Docker volume belongs to that environment instance.
In **Terminal**, use the target selector to choose a running container.
**Open terminal** in the environment details opens that same Terminal page.
Paired computers also appear in the target selector. Terminal access requires
a worker-owner grant for a starting folder and a shell profile on the selected
computer. The worker owner can grant multiple workspace folders. To offer a
home default, grant the worker account's home folder using the root ID
`home`. Choosing a workspace opens the shell in that workspace. The shell
runs with the worker account's permissions. Windows x64 and ARM64 workers also
require the installed Dockpipe native Terminal helper beside the worker
executable.
For a paired Docker host, select the host as the workspace machine. The
Terminal selector lists its running containers on Linux and macOS when Docker
read and container terminal permissions are both granted.

## Runtime status and ownership

Use `environment status --file PATH` or `environment status --name NAME` to
read the observed state. The GUI requests the same status for its inventory.
A VM provider reports the exact local workspace and run after verifying the
VM process. Status uses that recorded scope without contacting the guest.

Dockpipe labels direct containers automatically. For a workflow-backed
container, provisioning receives `DOCKPIPE_ENVIRONMENT_INSTANCE_ID`. To
associate one container with the definition, use the Docker label
`com.dockpipe.environment.instance=$DOCKPIPE_ENVIRONMENT_INSTANCE_ID`.
An absent or ambiguous runtime association leaves the state unverified.

For workflow-backed definitions, the workflow controls mounts, sync and guest
files. VM disks, caches, local credentials and pairing keys stay in their owning
runtime or host state. Use the workflow provider's credential references for
shareable project inputs.

For a VM that starts from a managed image, pass `--storage-profile NAME` to
`generate` or `create`, or add the following to its YAML.

```yaml
storage:
  profile: team-linux
```

The name selects `environment-storage/<name>.json` in the executing host's
Dockpipe user data root. Store the profile in a directory accessible only to
its owner, with the file readable and writable only by its owner. The profile
uses schema `dockpipe.environment.storage-profile/v1` and names an installed
storage `resolver` and a `secrets` configuration. The latter names the secret
resolver and provides its `parameters` and `bindings`. For private R2
snapshots, use the 1Password secret resolver for the endpoint, bucket, pinned
manifest digest and scoped reader credentials. Keep those references in the
host profile. The R2 reader ships as one compiled resolver executable.

Dockpipe verifies the content-addressed base in its user cache before handing
it to the provisioning workflow. QEMU uses the verified base with a separate
local writable overlay. Applying the definition only reads from cloud storage.

When a saved VM definition is first applied, Dockpipe assigns a local instance
ID and creates private state on the executing host. Applying that same
definition again uses the same instance even if the workflow working directory
changes. A separate definition gets its own instance. For a verified managed
image, QEMU retains one writable overlay per instance and checks that an
existing overlay still points to the selected base before opening it.

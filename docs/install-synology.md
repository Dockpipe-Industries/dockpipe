# Dockpipe on Synology (experimental)

Releases include manually installable CLI SPKs for **DSM 7+**, pending NAS
qualification. Download the matching artifact and `SHA256SUMS.txt` from the same
release and verify the SPK's SHA-256 before installing:

| NAS CPU | Artifact | DSM architecture family |
| --- | --- | --- |
| Intel/AMD 64-bit | `dockpipe_VERSION_synology_x64.spk` | `x86_64` |
| ARM 64-bit | `dockpipe_VERSION_synology_arm64.spk` | `armv8` |

These packages do not support 32-bit ARM. Check your model's CPU architecture
and DSM version before installing. Select **Package Center → Manual Install**
and choose the SPK. DSM displays a third-party package warning. No SynoCommunity
package source is required.

Enable SSH in DSM for terminal testing, then run as your normal NAS user:

```sh
/var/packages/dockpipe/target/bin/dockpipe --version
/var/packages/dockpipe/target/bin/dockpipe --help
```

The executable is package-local. To use `dockpipe` in the current shell:

```sh
export PATH="/var/packages/dockpipe/target/bin:$PATH"
```

The SPK installs the self-contained CLI without Go, Python, Qt or catalog packages.
Workflow dependencies are separate: native inline commands and `.sh` host scripts
require Bash; container workflows require a reachable Docker engine, appropriate
socket permissions and a model that supports Container Manager. Git and provider
tools are needed only by workflows using them. Run test workflows in a new
writable folder and verify required tools first. Installing the SPK does not
grant access to NAS shared folders or the Docker socket.

## GUI in a browser on the NAS

Releases also contain `dockpipe-desktop_VERSION_synology_amd64.tar.gz` and
`dockpipe-desktop_VERSION_synology_arm64.tar.gz`. These are Container Manager
project bundles containing the full existing GUI and CLI DEBs, a Dockerfile,
Compose configuration and setup instructions. Verify against the same release's
`SHA256SUMS.txt`, extract the matching archive into a dedicated folder and follow
its `SETUP.md`. Use `amd64` for Intel/AMD x64 NAS models.

The GUI executes in an Ubuntu XFCE container and streams to your browser over
HTTPS. It requires a NAS model supporting Container Manager and enough memory for
the desktop, Qt WebEngine and 1 GiB of shared memory. The bundle requires private
authentication configuration, defaults to a loopback listener, and persists its
state in `config/`. Access it through an SSH tunnel or an authenticated reverse
proxy supporting WebSockets. The initial test does not mount NAS shares or the
Docker socket. Desktop workflows execute inside the container; the host SPK is
a separate installation. Additional data mounts and Docker access are explicit
operator choices after the basic GUI works.

Building the project downloads the pinned Webtop base and Ubuntu dependencies.
The bundle is reproducible; the resulting apt-based image is not bit reproducible.
Container startup, browser interaction and NAS acceptance are separate checks.

## Desktop GUI controlling a NAS worker

The current Dockpipe GUI is a native Qt desktop application. This SPK does not
include a DSM browser GUI. Existing Linux desktop DEBs target Debian/Ubuntu and
must not be installed on DSM.

To use the existing GUI with the NAS, install Dockpipe Desktop on a supported
computer and use the NAS CLI as a remote worker. Follow the
[remote pairing guide](runtime/remote-nodes.md#pair-and-run), approve the NAS on the
broker computer, then run the worker in the foreground on the NAS:

```sh
/var/packages/dockpipe/target/bin/dockpipe remote worker
```

Keep pairing and execution under the same NAS account so they use the same
identity and home directory. The connection ends when the worker process exits.
The SPK does not start a worker automatically. Do not use `remote service` on DSM:
the current Linux service installer expects a user systemd service. DSM service
integration requires separate qualification. Remote control does not forward the
native Qt application into DSM; the GUI remains on the operator's computer.

## NAS acceptance before community submission

Record the model, CPU, DSM version and exact release checksum. Test:

1. Manual installation, `--version` and `--help` under a normal SSH account.
2. A minimal host workflow in a writable test folder, with Bash available.
3. Pairing, foreground worker connection, a GUI-submitted job and its returned
   logs/result. Verify the job ran on the NAS, rather than locally.
4. A container workflow if Container Manager is supported and explicitly enabled.
5. Upgrade to a newer SPK, repeat the CLI/job checks, then uninstall. Verify
   removal of the package executable and preservation of unrelated files and
   the user's Dockpipe state. Stop foreground workers before upgrading/removing.
6. For the browser desktop bundle, verify authenticated HTTPS access, launch
   Dockpipe, open a project in the container, execute a host workflow and inspect
   its logs. Restart the container and verify settings/project persistence. Test
   a bundle upgrade with a copy of the persistent state before using real work.

Package Center manages package files; per-user state and working directories
belong to the account that ran Dockpipe and are not deleted by package scripts.
Local archive checks cannot establish these NAS results. SynoCommunity submission
should follow successful hardware testing.

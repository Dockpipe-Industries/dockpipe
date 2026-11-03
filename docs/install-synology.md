# Dockpipe on Synology (experimental)

Choose **Dockpipe CLI** for SSH/remote workers, or **Dockpipe** for the combined
browser GUI and CLI. Both are manually installable SPKs, pending NAS qualification.
Download the matching artifact and `SHA256SUMS.txt` from the same release and verify
the SPK's SHA-256 before installing:

| NAS CPU | CLI only (DSM 7+) | GUI + CLI (DSM 7.2.1+) |
| --- | --- | --- |
| Intel/AMD 64-bit | `dockpipe_VERSION_synology_x64.spk` | `dockpipe-desktop_VERSION_synology_x64.spk` |
| ARM 64-bit | `dockpipe_VERSION_synology_arm64.spk` | `dockpipe-desktop_VERSION_synology_arm64.spk` |

These packages do not support 32-bit ARM. Check your model's CPU architecture
and DSM version before installing. Select **Package Center → Manual Install**
and choose the SPK. DSM displays a third-party package warning. No SynoCommunity
package source is required.

## Combined GUI + CLI installation

Combined build **0.6.37-1** has a confirmed DSM startup bug: the package worker
forces an unprivileged container UID while that build's Webtop init requires root.
A packaging repair is undergoing NAS acceptance; use the corrected candidate
rather than repeatedly reinstalling that build. CLI-only installation is unaffected.

Install Container Manager 20.10.23-1437 or newer on a model supporting it, then
manually install the combined SPK. Choose your Dockpipe sign-in in the installation
wizard. Building the application downloads components and can take several minutes.
After installation, select **Open** in Package Center or the **Dockpipe** icon in
DSM. Package Center owns start/stop and the application starts automatically with
its desktop session; no Compose file or shell setup is required.

Use your separate Dockpipe sign-in to enter the browser GUI. This is not DSM SSO.
Released URL-launcher builds use a self-signed HTTPS certificate on port 38139;
confirm the intended NAS endpoint before trusting it, or configure a trusted reverse
proxy. The DSM-window candidate described below instead uses DSM's existing HTTPS
connection and needs no additional operator-configured reverse proxy.
Do not forward the desktop port directly to the Internet. A single installation
has one shared desktop and workspace. The DSM launcher is initially admin-only.

Settings, sign-in and workspace data persist across upgrades/reinstall. Reinstall
does not reset the password or erase the workspace. Corrected builds offer an
uninstall choice: keep saved data (default), or permanently delete all Dockpipe
settings and workspace data. Upgrades always retain data. DSM owns project and
container removal in either case; hardware cleanup acceptance remains pending. Native commands are at
`/var/packages/dockpipe-desktop/target/bin/dockpipe`; the GUI uses its own matching
CLI inside the managed application environment.

The installation/startup flow must still be qualified on actual DSM hardware.
Local container tests do not establish Synology package-framework acceptance.

## NAS host connection candidate

The next desktop candidate uses host networking to restore outbound access on
NAS setups where the Docker bridge cannot reach the Internet. Its desktop stream
listens only on `127.0.0.1:38139`; DSM's package proxy provides browser access.
Host networking shares networking, not the NAS filesystem or process namespace.

The candidate asks for an SSH host connection. Enable DSM SSH, enter the NAS
address, SSH port and normal account, then sign in through **Open host terminal**.
Verify the host key before accepting it. The native CLI path is prefilled with
`/var/packages/dockpipe-desktop/target/bin/dockpipe`. System/process and Docker
views select that NAS connection; **GUI container** remains an explicit local
choice. Docker inventory requires the account's existing Docker permissions.
Passwords are handled by SSH and are not saved in the connection profile.

The host terminal runs on DSM. Browser, files and ordinary local workflows still
run in the GUI container. No host root filesystem or Docker socket is mounted.
Full hardware qualification of this candidate remains pending.

## CLI-only installation

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

## Portable GUI bundle (advanced)

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

The current Dockpipe GUI is a native Qt desktop application. The CLI-only SPK
does not include it. The combined SPK provides its own managed application
environment; existing Linux desktop DEBs must not be installed directly on DSM.

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
7. For the combined SPK, verify wizard validation, image preparation, DSM icon/
   Open navigation, direct GUI launch, Package Center start/stop and startup after
   reboot. Verify unavailable/unauthenticated service status, then upgrade and
   reinstall with a copy of saved state. Confirm credentials/workspace are retained.

Package Center manages package files. CLI-only state belongs to the account that
ran Dockpipe; combined-edition private settings and GUI workspace use the package's
persistent home/var. Package scripts do not delete these retained files.
Local archive checks cannot establish these NAS results. SynoCommunity submission
should follow successful hardware testing.

## Prebuilt standard and offline editions under qualification

The next packaging change prepares the desktop image in CI. The standard SPK
will download an exact image digest; an explicitly named `_offline.spk` will carry
the same image for DSM's native preload worker. Neither builds the image on the
NAS. Install Container Manager first; it is not bundled in the offline SPK.

These are alternative installers for the same Dockpipe package and saved data.
Standard is the smaller default. Offline is larger and intended for installation
without registry access. Workflow execution can still require networking.
Native preload completion and upgrades between editions must be qualified on the
NAS before these candidates are advertised as tested releases. Image import has
succeeded on x64 DSM, but the large offline installation has not reliably completed.

The candidate GUI launcher is a real DSM application window containing the
streamed Qt desktop. Its package-scoped proxy uses your existing HTTPS DSM origin;
the internal desktop port is loopback-only. Dockpipe sign-in remains separate
from DSM sign-in. A working embedded desktop has been confirmed on an x64 NAS;
the remaining lifecycle and interaction checks still require hardware acceptance.

Open DSM over HTTPS using the NAS address and its configured DSM port, then launch
Dockpipe from the DSM menu. The package installs its internal route automatically.
Users accessing DSM directly do not need to create a reverse proxy or expose the
internal desktop port.

If an existing reverse proxy fronts DSM, it must also forward WebSocket upgrades.
For Synology Reverse Proxy, edit the matching DSM hostname rule under **Control
Panel → Login Portal → Advanced → Reverse Proxy**, then select **Custom Header →
Create → WebSocket** and save. Refresh DSM and reopen Dockpipe. This requirement
applies to that existing proxy hop; see [Synology's WebSocket troubleshooting
guide](https://kb.synology.com/en-nz/DSM/tutorial/I_cannot_connect_to_a_virtual_machine_via_Virtual_Machine_Manager_What_can_I_do).

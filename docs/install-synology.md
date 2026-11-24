# Dockpipe on Synology (experimental)

Choose **Dockpipe CLI** for SSH/remote workers, or **Dockpipe** for the combined
browser GUI and CLI. Both are manually installable SPKs, pending NAS qualification.
Download the matching artifact and `SHA256SUMS.txt` from the same release and verify
the SPK's SHA-256 before installing:

| NAS CPU | CLI only (DSM 7+) | GUI + CLI (DSM 7.2.2+) |
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
does not reset the password or erase the workspace. Recovery candidates detect
retained state from bounded DSM volume metadata even when the wizard has no
installed-package links or exported home/volume path. They show **Restore
Dockpipe** without requesting replacement credentials. Restore is bound to the
selected installation volume; uncertain state stops setup instead of resetting
sign-in. During repair DSM fixes the existing installation volume; the native
installer validates that destination and the retained settings without requiring
a volume selector in the repair wizard. Discovery does not read the saved password. Corrected builds offer an
uninstall choice: keep saved data (default), or permanently delete all Dockpipe
settings and workspace data. Upgrades always retain data. DSM owns project and
container removal in either case; hardware cleanup acceptance remains pending. Native commands are at
`/var/packages/dockpipe-desktop/target/bin/dockpipe`; the GUI uses its own matching
CLI inside the managed application environment.

The installation/startup flow must still be qualified on actual DSM hardware.
Local container tests do not establish Synology package-framework acceptance.

## Host folder mounts (installer qualification)

The combined package's **Host folders** install/update step can map up to 16
existing NAS folders under `/host`. Choose **Edit folder mappings**, then **Add
folder** for each mapping. Use **Browse…** to choose a local NAS directory with
DSM's native folder picker, or enter its absolute NAS path directly. Choose
**Read only** (default) or
**Read and write**, and set the name shown inside Dockpipe. The name follows the
NAS folder's last path component until you override it. For example,
`/volume1/projects` defaults to `/host/projects`; naming it `work` instead maps it
to `/host/work`. Use these paths in Dockpipe's project folder picker.

Each mount name is one path component of at most 128 UTF-8 bytes. Names must be
unique; when two source folders share a name, override one before continuing.
The complete mapping list is limited to 64 KiB. These limits are checked both
by the wizard and by the installer helper.

**Keep** preserves all saved mappings across updates and reinstall. **Edit**
replaces the saved list with the rows shown in the wizard; include every folder
you want to retain. **Remove all** drops the mappings without deleting host
files. To change them later, run the package's manual update wizard with a newer
SPK. Save desktop work before updating. The repeatable wizard uses DSM 7.2.2's
custom component contract, so combined packages now require DSM 7.2.2 or later.
There is no live Compose editor or runtime mount/redeploy action in Dockpipe.

The candidate helper is resolved within DSM's supplied temporary directory,
with direct-payload and outer-directory layouts checked explicitly. Missing or
ambiguous candidates fail without using an old installation. The helper prepares
its own candidate Compose file before DSM acquires the
Docker Project resource. DSM applies the mount during the normal package
installation/update lifecycle. The mapping list is stored outside the GUI's mounted
configuration and rendered again for the next package update. The existing image,
package identity, internal settings mounts and native host connection are retained.
This installer behavior still requires qualification on DSM hardware.

Each selected folder must already exist. On **Next**, the candidate wizard uses
the signed-in DSM administrator session to prepare the **dockpipe-desktop**
account's chosen access through DSM's ACL API. Blocking parents receive traversal
on that directory only; the selected folder receives its chosen access with
folder/file inheritance. There is no separate permission-review dialog or action.
The wizard preserves existing rules and explicit denials, checks for intervening
ACL changes before writing, and verifies the resulting rules and effective access
before advancing. It reports a refused or uncertain operation rather than
retrying blindly or replacing existing ACLs. Ownership is retained.

These operations are bounded and sequential; they are not an atomic transaction
across multiple folders. Cancelling or encountering an error may leave permissions
already applied to earlier folders. The installer itself performs no ACL writes
and checks kernel access again before preparing mounts. It rejects volume roots,
system storage paths and symbolic links. Existing file ACLs remain in force, so
children may retain more restrictive permissions. Fresh-install package-account
creation timing and the complete automatic permission/install flow still require
DSM qualification; the current test targets updates with an existing account.

Existing r10 single-folder settings migrate while retaining `/host/source` when
**Keep** is selected; they are not silently renamed on upgrade.

Native host terminals keep using the authenticated DSM user's filesystem paths,
for example `/volume1/projects/source`. `/host/<name>` is the desktop container's
path; it is not an alias installed into the host filesystem. Native desktop builds
continue using their ordinary local folder picker. Other container deployments
can bind folders through their deployment configuration; these wizard controls
belong to the DSM package.

## Native NAS host integration (qualification)

The native-host Docker Inspector queries container, network and volume details
through the authenticated worker. Container Logs show up to the last 200 lines,
capped at 64 KiB with a truncation notice. Details and Raw inspect use a bounded
read-only inspect response. Unsupported logging drivers report a log-specific
error while retaining inspect data. Pausing updates, changing selection or losing
the host session cancels pending queries and prevents stale output from rendering.
The equivalent CLI is `dockpipe host docker-query --connection /absolute/connection.json
--kind container --id <hex-id> --json`; network and volume queries use their
corresponding kinds. This does not enable container exec.

The native candidate adds **Enable NAS access** inside the real Dockpipe DSM
window. It uses the existing DSM administrator session to create and run a fixed,
disabled, one-shot Task Scheduler entry. The starter launches the package's
native worker as that actual user, then the app removes the starter task. No SSH
profile, password, key, root GUI, Docker socket mount or host root mount is needed.
The separate qualification apps are diagnostic predecessors, not dependencies.

Hardware qualification proved the starter executes with the signed-in account's
kernel UID/GID and groups, lists NAS containers and reaches outbound HTTPS. The
integrated production candidate still needs installation and interaction checks
on DSM. Local integration tests establish the broker, terminal transport,
monitoring and session shutdown; they do not establish final NAS acceptance.

The first explicit activation binds the installation's shared desktop to one
verified local administrator. Another DSM account cannot view or operate that
user's desktop. The native gateway validates DSM authentication and renews a
short browser lease while the window is open. **Disable NAS access** revokes the
worker and closes streams; closing all windows lets the lease expire. Reopening
may require Enable again. Uninstall with saved data retained also retains the
operator binding; full data removal resets it.

The native installer preserves existing streamer certificate/key bytes and
migrates the old public certificate's 0644 permissions to the gateway's 0600
contract. Successful session renewal returns its actual lease deadline to the
launcher. Initial native setup remains explicit; an already connected authorized
session reopens the desktop automatically.

Dockpipe selects the native NAS for terminal, system/process metrics and Docker
inventory and Start/Stop controls. **Application container** remains a separate,
explicit choice. Loss of the native connection clears host views and terminal
replay rather than silently opening a container shell. Commands run with the
selected user's ordinary permissions, including whatever Docker access DSM
already grants that account. A terminal is a normal user shell, not a sandbox;
explicitly detached or privilege-changed jobs can outlive a terminal session.

Host networking is retained because same-image NAS testing confirmed it restores
outbound HTTPS where bridge TCP egress failed. The streamer listens only on
`127.0.0.1:38139`; a package gateway on loopback `38143` validates the DSM session
and proxies the embedded desktop. The package installs these internal routes.
A separately configured reverse proxy is unnecessary when connecting directly to
DSM over HTTPS. An existing front proxy must still forward WebSocket upgrades.

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

The release workflow prepares the desktop image in CI. The standard SPK
downloads an exact image digest; an explicitly named `_offline.spk` carries
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
the internal desktop port is loopback-only. The native candidate gates the desktop with DSM sign-in; the internal streaming
credential is not shown to the browser. A working embedded desktop has been confirmed on an x64 NAS;
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

An offline image import can take several minutes. If Package Center reports a
system error while the package is still installing or repairing, check the
reverse proxy error log before retrying. An `upstream timed out` entry while
waiting for DSM's response headers identifies a proxy read timeout, even though
the native installation can continue. For the affected DSM rule, use **Advanced
Settings → Proxy read timeout (sec.)** to allow enough time, such as 600 seconds.
This is a supported per-rule setting; do not patch Package Center, edit generated
nginx files, or remove installation flags. Leave connection/send timeouts and TLS
settings unchanged unless separate evidence requires a change. See
[Synology's reverse proxy settings](https://kb.synology.com/en-my/DSM/help/DSM/AdminCenter/system_login_portal_advanced).

The prebuild producer requires both the matching CLI (`--binary`) and static
native gateway (`--native-host-binary`), plus the matching CLI and desktop DEBs.
The gateway is built from
`./workflows/package/synology-desktop-spk/assets/native-host` for the selected Linux
architecture. `--package-revision` selects a positive DSM build revision without
changing the application version. Building a candidate does not publish its image;
standard SPKs require the recorded digest to be published before general use.

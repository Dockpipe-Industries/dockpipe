# Install Dockpipe

Choose **Desktop** for the launcher and CLI together, or **CLI / Remote Worker**
for terminals, servers and CI. You do not need a Dockpipe source checkout or Go.
Optional workflows and resolvers are installed from **Packages → Marketplace** or
`dockpipe package install`; the base CLI is self-contained.

Use the [release catalog](https://packages.dockpipe.com/packages/runtime/latest.json)
to select the current release. The website selects the matching catalog for
its environment; this source guide defaults to production.

| Platform | Desktop | CLI / Remote Worker |
| --- | --- | --- |
| macOS Apple Silicon / Intel | DMG installer | Native tarball or portable installer |
| Windows x64 | MSI with launcher selected | ZIP, or MSI with only the CLI feature |
| Ubuntu 22.04 / 24.04 amd64 / arm64; Pop!_OS 22.04 | `dockpipe-desktop` DEB plus `dockpipe` | `dockpipe` DEB |
| Linux amd64 with Flatpak | Combined launcher and CLI bundle, KDE Platform 6.10 | Bundled CLI via `flatpak run`; native packages are separate |
| Other Linux distributions | See Flatpak requirements and qualification limits | RPM, APK, Arch or portable archive matching your CPU |

Native host inline commands use Windows PowerShell on Windows and Bash on
Linux/macOS. Host `.sh` scripts and environment-sourcing setup scripts require
Bash; Git for Windows supplies it on Windows. PowerShell 7 (`pwsh`) is required
when a workflow selects it, or runs `.ps1` host scripts on Linux/macOS. Container
workflows need a reachable Docker engine. Git, provider CLIs and authentication
are needed only by workflows that use them. The Flatpak includes shell and
container client tools but still needs a host engine for container workflows.

After installation, start with [Your first workflow](onboarding.md).

## Desktop installation

**macOS:** download `dockpipe-desktop_VERSION_darwin_arm64.dmg` for Apple Silicon or `dockpipe-desktop_VERSION_darwin_amd64.dmg` for Intel, verify it against that release's `SHA256SUMS.txt`, open it, and run **Install Dockpipe.pkg**. Apple's Installer installs **DockPipe.app** in Applications and **`dockpipe`** in `/usr/local/bin`. Open Dockpipe from Applications; new terminals can run `dockpipe --version`. The desktop app contains the matching CLI and Qt runtime. macOS 13 or newer is required.

Use one installation method. The DMG installer refuses an existing foreign CLI or app; users with an existing Homebrew installation should keep that installation method or remove it before switching. Re-running the DMG installer updates an installation owned by that installer. Current DMGs use ad-hoc signatures and are not yet Developer ID signed and notarized. They may be blocked by Gatekeeper; the installation instructions do not require changing macOS security settings.

<!-- dockpipe:channel production -->
The Homebrew tap currently publishes staging formulas only. For production,
use the DMG above or the native CLI installer below.
<!-- dockpipe:end-channel -->

<!-- dockpipe:channel staging -->
**Homebrew on macOS (staging):** refresh the tap, then install the launcher and CLI together:

```sh
brew update
brew install --cask dockpipe-industries/dockpipe/dockpipe-desktop-staging
```

The existing CLI formula is reused if already installed. To update both:

```sh
brew update
brew upgrade dockpipe-staging dockpipe-desktop-staging
```

Uninstalling the desktop cask removes the app and retains the CLI formula. User contexts and data are retained. The app uses its bundled matching CLI; terminal commands use the formula. See [desktop packaging](../workflows/build/desktop/README.md) for direct-installer removal and signing requirements.

<!-- dockpipe:end-channel -->

**Windows:** run `dockpipe_VERSION_windows_amd64.msi`. The default installs the CLI, full Qt launcher runtime and a Start menu shortcut. Deselect **Dockpipe Launcher** for a CLI-only install, or change the feature later through the installer's Modify option. The ZIP is always CLI-only.

**Ubuntu 22.04 / 24.04 and Pop!_OS 22.04:** after configuring the signed APT source below, run `sudo apt install dockpipe-desktop`; it installs the matching CLI dependency. The DEB supports Ubuntu 22.04's Qt libraries and is installation-tested on both Ubuntu releases. For downloaded DEBs, install both together:

```sh
sudo apt install ./dockpipe_VERSION_amd64.deb ./dockpipe-desktop_VERSION_amd64.deb
```

Use `arm64` filenames on ARM64. Launch **Dockpipe** from the application menu. Removing `dockpipe-desktop` leaves the CLI available. Desktop RPM, APK and Arch packages are not currently provided; those formats remain CLI-only.

### Flatpak desktop (Linux amd64)

Install the launcher and CLI together on **Linux x86_64 (amd64)**. Install
Flatpak from your distribution first. DockPipe uses the **KDE Platform 6.10**
runtime from Flathub; the DockPipe app itself is a separate download. ARM64
Flatpak builds are not available.

**1. Add the runtime** (once):

```sh
flatpak remote-add --user --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo
flatpak install --user flathub org.kde.Platform/x86_64/6.10
```

**2. Download, verify and install the current release.** Run this block in Bash
with `curl`, `jq` and `sha256sum` installed. It reads the current release catalog,
then uses that same release for the bundle and checksum. It stops if a download
or checksum check fails; there is no version number to edit.

```bash
(
    set -euo pipefail
    mkdir -p "$HOME/Downloads/dockpipe"
    cd "$HOME/Downloads/dockpipe"

    origin=https://packages.dockpipe.com
    manifest=$(curl -fsSL "$origin/packages/runtime/latest.json" | jq -er '.manifest')
    base="$origin/${manifest%/*}"
    curl -fsSL "$origin/$manifest" -o release-manifest.json
    bundle=$(jq -er '.downloads["linux-amd64-flatpak-org.kde.Platform-6.10"].desktop' release-manifest.json)
    curl -fSL "$base/$bundle" -o "$bundle"
    curl -fsSL "$base/SHA256SUMS.txt" -o SHA256SUMS.txt
    awk -v file="$bundle" '$2 == file { print }' SHA256SUMS.txt > "$bundle.sha256"
    sha256sum --check "$bundle.sha256"
    flatpak install --user --or-update "./$bundle"
)
```

**3. Open DockPipe** from your application menu, or run:

```sh
flatpak run com.dockpipe.Dockpipe
```

The launcher sets up a user-level `dockpipe` shortcut on its first start. Open a
new terminal and run `dockpipe --version`. Existing native installations are
preserved. You can always run the bundled CLI directly:

```sh
flatpak run --command=dockpipe com.dockpipe.Dockpipe --version
```

For commands that work on a project, pass its directory and filesystem access
explicitly when bypassing the terminal shortcut. This also works around the
mounted-directory shortcut bug in 0.6.13. Run from the intended project folder:

```sh
(
    project_dir=$(pwd -P) || exit 1
    flatpak run --command=dockpipe --cwd="$project_dir" \
        --filesystem="$project_dir" com.dockpipe.Dockpipe init
)
```

**Updates:** close DockPipe and finish active workflows, then repeat step 2.
`flatpak update` updates the KDE runtime, but does not download new DockPipe
bundles. DockPipe does not yet provide an updating Flatpak remote.

**Optional packages:** open **Settings → Package Remotes → Use production**,
save, then open **Packages → Marketplace**. The Flatpak selects packages built
for its architecture and runtime; native package stores are separate.

**Host tools:** container workflows need a running host Docker or compatible
Podman API socket. Editors, provider CLIs and authentication must be installed
on the host when a workflow uses them. The Flatpak includes Git, Docker CLI,
Compose and Buildx; it does not install a host engine. See
[host integration and platform limits](../workflows/build/flatpak/README.md)
for permissions, terminal setup details and troubleshooting.

## Signed APT repository

For Ubuntu/Pop!_OS installations, configure the signed APT source
once. Download the public key and inspect its fingerprint:

```sh
curl -fsSL https://packages.dockpipe.com/apt/dockpipe-archive-keyring.gpg -o /tmp/dockpipe-archive-keyring.gpg
gpg --show-keys --with-fingerprint /tmp/dockpipe-archive-keyring.gpg
```

Confirm fingerprint `71B23431AB2FD718629B2696A27C6DAE150CD947` before proceeding:

```sh
sudo install -m 0644 /tmp/dockpipe-archive-keyring.gpg /usr/share/keyrings/dockpipe.gpg
printf '%s\n' 'deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/dockpipe.gpg] https://packages.dockpipe.com/apt stable main' | sudo tee /etc/apt/sources.list.d/dockpipe.list >/dev/null
sudo apt update
sudo apt install dockpipe-desktop
```

Use `sudo apt install dockpipe` for CLI only. The desktop package installs its
matching CLI dependency. Updates use:

```sh
sudo apt update
sudo apt upgrade
```

Use only one APT release channel at a time. Remove a previous channel source
before configuring the source above. Removing `dockpipe-desktop` retains the CLI.

## Direct downloads and complete package stores

Read the [release catalog](https://packages.dockpipe.com/packages/runtime/latest.json)
and follow its `manifest` path relative to `https://packages.dockpipe.com/`.
The release manifest lists numeric `version`, per-platform `downloads`, and
`stores`. Download filenames are relative to the manifest's directory.

Choose `amd64` for x86_64 or `arm64` for aarch64/Apple Silicon. Verify downloaded
files against `SHA256SUMS.txt` from that same release before installing. On Linux
use `sha256sum`; on macOS use `shasum -a 256`; on Windows use `Get-FileHash -Algorithm SHA256`.

The native CLI installer can use the current release. These commands
require `curl` and `jq` and select one immutable manifest before downloading:

```sh
origin=https://packages.dockpipe.com
manifest=$(curl -fsSL "$origin/packages/runtime/latest.json" | jq -er '.manifest')
base="$origin/${manifest%/*}"
version=$(curl -fsSL "$origin/$manifest" | jq -er '.version')
curl -fsSL "$base/install.sh" -o /tmp/dockpipe-install.sh
DOCKPIPE_VERSION="$version" DOCKPIPE_DOWNLOAD_BASE="$base" DOCKPIPE_INSTALL_MODE=portable sh /tmp/dockpipe-install.sh
```

The installer verifies its CLI payload checksum. It installs the portable CLI in
`~/.local/bin` by default; add that directory to your shell's `PATH` if needed.
Use the numeric manifest `version` for `DOCKPIPE_VERSION`.
This script installs the self-contained CLI, not the desktop launcher.

### Native Linux packages

After downloading and verifying the matching packages, install the appropriate
format. Replace `VERSION` and use `arm64` filenames where supported:

| Format | Example |
| --- | --- |
| DEB CLI | `sudo apt install ./dockpipe_VERSION_amd64.deb` |
| DEB desktop and CLI | `sudo apt install ./dockpipe_VERSION_amd64.deb ./dockpipe-desktop_VERSION_amd64.deb` |
| RPM CLI | `sudo dnf install ./dockpipe_VERSION_linux_amd64.rpm` |
| Alpine CLI | `sudo apk add --allow-untrusted ./dockpipe_VERSION_linux_amd64.apk` |
| Arch CLI | `sudo pacman -U ./dockpipe_VERSION_linux_amd64.pkg.tar.zst` |

RPM, APK and Arch packages are CLI-only. For direct-file upgrades, download,
verify and install the newer files using the same method. Generation of a package
format does not qualify every downstream distribution or container engine.

### Complete stores (optional)

Most users should install individual packages from Marketplace. If you need a
complete native store, download `dockpipe-packages_VERSION_OS-ARCH.tar.gz`, verify
its checksum and extract it. Register the extracted store in your project's
`dockpipe.config.json`:

```json
{"schema":1,"packages":{"sources":[{"kind":"tarball_dir","path":"/absolute/path/to/extracted-store"}]}}
```

Use the store built for your platform; native stores cannot replace a Flatpak
runtime-specific store. Provider tools and credentials may still be needed.
See the [package model](packages/package-model.md#remote-catalogs-and-user-installation).

## Windows CLI and optional WSL

For a native CLI installation, download and verify
`dockpipe_VERSION_windows_amd64.zip`, extract it and add its folder to your user
`PATH`. Alternatively use the MSI and select the CLI feature. Open a new terminal
and check:

```powershell
dockpipe --version
```

Native Windows host inline commands use the installed Windows PowerShell.
Install Git for Windows when a workflow uses Bash scripts or environment-sourcing
setup scripts, and PowerShell 7 when it selects `shell: pwsh`. Container execution
uses Docker Desktop. WSL is optional; a native install does not require a Linux
Dockpipe binary. Follow [the WSL bridge guide](runtime/wsl-windows.md) only when you
want execution inside a WSL distribution. Dockpipe is not listed in the default
winget catalog yet.

## macOS CLI / Remote Worker

Use the [native CLI installer](#direct-downloads-and-complete-package-stores)
or download the matching macOS tarball from the selected release catalog.
The portable installer installs the self-contained CLI into `~/.local/bin`; add it to
your shell PATH and check `dockpipe --version`.

<!-- dockpipe:channel staging -->
For the staging Homebrew CLI:

```sh
brew install dockpipe-industries/dockpipe/dockpipe-staging
dockpipe --version
```

Update with `brew update` followed by `brew upgrade dockpipe-staging`.
<!-- dockpipe:end-channel -->

### Containers with Colima

For container workflows, you can use [Colima's Docker runtime](https://github.com/abiosoft/colima#docker)
with the Docker CLI. Docker Desktop is another option. Neither is needed for host-only workflows.
For a new Colima installation:

```sh
brew install colima docker docker-buildx
colima start --runtime docker
docker context ls
docker version
```

Follow the [Homebrew Buildx setup](https://formulae.brew.sh/formula/docker-buildx) to add the actual
`$(brew --prefix)/lib/docker/cli-plugins` directory to `cliPluginsExtraDirs` in your existing Docker
configuration (`~/.docker/config.json`, or the directory selected by `DOCKER_CONFIG`). Merge that
setting without replacing existing configuration. Verify `docker buildx version` before workflows
that build images. Workflows using Compose also need
[docker-compose and its plugin setup](https://formulae.brew.sh/formula/docker-compose).

Dockpipe uses the [Docker client's context selection](https://docs.docker.com/engine/manage-resources/contexts/).
Use `docker context ls` to find your profile's context, then `docker context use <name>` if you want
to change the default for both terminals and the launcher. For one terminal command, use
`DOCKER_CONTEXT=<name> dockpipe doctor`. Existing `DOCKER_HOST`, `DOCKER_CONTEXT`, and `DOCKER_CONFIG`
settings retain Docker's normal precedence; Dockpipe does not switch contexts or start a VM.

The launcher adds `/opt/homebrew/bin` and `/usr/local/bin` after inherited `PATH` entries so Finder
launches can discover Homebrew tools. Shell-only exports are not inherited by Finder; use Docker's
saved context for Finder launches, or start the app executable from a configured terminal. Custom
Homebrew locations still need an explicit `PATH`. If Docker is unreachable, start the selected Colima
profile and verify `docker version` before retrying `dockpipe doctor`.

The compatibility changes are implemented in source. Native Colima container, mount, networking,
and Compose qualification on macOS remains pending; the earlier desktop installer dry run does not
cover this integration.

## Browsing packages in the launcher

Open **Settings → Package Remotes → Use production**, save, then open
**Packages → Marketplace**. The package origin for this guide is
`https://packages.dockpipe.com`. Keep application downloads and package
catalogs on the same release channel.

Marketplace verifies checksums and installs selected packages into your user
store. Base installers include the self-contained CLI. See [Find and use packages](packages/package-quickstart.md)
for dependencies, terminal installation and removal.

## Self-contained runtime and optional workflows

Install the application to obtain the CLI and its generic runtime capabilities.
Project workflows live under `workflows/` or explicitly declared source roots.
Optional workflow and resolver packages live in the package store; install them
through the Marketplace or `dockpipe package install`.

No source checkout, shared core archive or extracted workflow tree is required.
See [runtime resource ownership](concepts/runtime-resource-ownership.md) and the
[package model](packages/package-model.md).

## Building or maintaining Dockpipe

Source builds and contributor setup are covered in [CONTRIBUTING.md](../CONTRIBUTING.md).
Installer construction and release publication belong to the
[release documentation](../release/README.md). They are not installation steps for
an end user.

# Install Dockpipe

Choose **Desktop** for the launcher and CLI together, or **CLI / Remote Worker**
for terminals, servers and CI. You do not need a Dockpipe source checkout or Go.
Optional workflows and resolvers are installed from **Packages → Marketplace** or
`dockpipe package install`; the base installer includes required core.

The verified downloads below use the **staging channel**. Check the
[staging catalog](https://packages.staging.dockpipe.com/packages/latest.json) for the
current candidate. Staging builds carry the qualification limits described below.

| Platform | Desktop | CLI / Remote Worker |
| --- | --- | --- |
| macOS Apple Silicon / Intel | Homebrew staging cask or DMG | Homebrew staging formula or native tarball |
| Windows x64 | MSI with launcher selected | ZIP, or MSI with only the CLI feature |
| Ubuntu 22.04 / 24.04 amd64 / arm64; Pop!_OS 22.04 | `dockpipe-desktop` DEB plus `dockpipe` | `dockpipe` DEB |
| Linux amd64 with Flatpak | Combined launcher and CLI bundle, KDE Platform 6.10 | Bundled CLI via `flatpak run`; native packages are separate |
| Other Linux distributions | See Flatpak requirements and qualification limits | RPM, APK, Arch or portable archive matching your CPU |

Native host steps need Bash; Git for Windows supplies it on Windows. Container
workflows need a reachable Docker engine. Git, provider CLIs and authentication
are needed only by workflows that use them. The Flatpak includes shell and
container client tools but still needs a host engine for container workflows.

After installation, start with [Your first workflow](onboarding.md).

## Desktop installation

**macOS:** download `dockpipe-desktop_VERSION_darwin_arm64.dmg` for Apple Silicon or `dockpipe-desktop_VERSION_darwin_amd64.dmg` for Intel, verify it against that release's `SHA256SUMS.txt`, open it, and run **Install Dockpipe.pkg**. Apple's Installer installs **DockPipe.app** in Applications and **`dockpipe`** in `/usr/local/bin`. Open Dockpipe from Applications; new terminals can run `dockpipe --version`. The desktop app contains the matching CLI, Qt runtime and required core package. macOS 13 or newer is required.

Use one installation method. The DMG installer refuses an existing foreign CLI or app; users with the Homebrew CLI should use the cask below. Re-running the DMG installer updates an installation owned by that installer. Current staging DMGs use ad-hoc signatures and are not yet Developer ID signed and notarized. They may be blocked by Gatekeeper; the installation instructions do not require changing macOS security settings.

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

Uninstalling the desktop cask removes the app and retains the CLI formula. User contexts and data are retained. The app uses its bundled matching CLI; terminal commands use the formula. See [desktop packaging](../release/packaging/desktop/README.md) for direct-installer removal and signing requirements.

**Windows:** run `dockpipe_VERSION_windows_amd64.msi`. The default installs the CLI, full Qt launcher runtime and a Start menu shortcut. Deselect **Dockpipe Launcher** for a CLI-only install, or change the feature later through the installer's Modify option. The ZIP is always CLI-only.

**Ubuntu 22.04 / 24.04 and Pop!_OS 22.04:** after configuring the signed APT source below, run `sudo apt install dockpipe-desktop`; it installs the matching CLI dependency. The DEB supports Ubuntu 22.04's Qt libraries and is installation-tested on both Ubuntu releases. For downloaded DEBs, install both together:

```sh
sudo apt install ./dockpipe_VERSION_amd64.deb ./dockpipe-desktop_VERSION_amd64.deb
```

Use `arm64` filenames on ARM64. Launch **Dockpipe** from the application menu. Removing `dockpipe-desktop` leaves the CLI available. Desktop RPM, APK and Arch packages are not currently provided; those formats remain CLI-only.

### Flatpak desktop (staging, Linux amd64)

The staging package host publishes `dockpipe-desktop_VERSION_linux_amd64.flatpak`.
It installs **`com.dockpipe.Dockpipe`**, branch **`staging`**, with launcher, CLI,
required core, Git, Docker CLI, Compose and Buildx. Optional Marketplace packages
are downloaded separately. There is no published ARM64 Flatpak catalog or Dockpipe
Flathub listing. Native DEB/RPM/APK/Arch packages and tarballs remain separate host
CLI installation choices.

Install Flatpak using your distribution's instructions first. The app requires the
**`org.kde.Platform//6.10` x86_64 runtime**; the SDK is only needed to build it.
Set up the runtime in your user installation:

```sh
flatpak remote-add --user --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo
flatpak install --user flathub org.kde.Platform/x86_64/6.10
```

Flathub supplies the KDE runtime here, not the Dockpipe app. The bundle also carries
that runtime-repository hint; it does not contain the runtime itself.

The following immutable candidate was verified on **2026-10-10**. For a newer build,
read the [staging pointer](https://packages.staging.dockpipe.com/packages/latest.json),
then its `manifest` URL relative to `https://packages.staging.dockpipe.com/`.
Use the `downloads["linux-amd64-flatpak-org.kde.Platform-6.10"].desktop` filename
relative to that manifest's directory. Do not construct a GitHub asset URL: the
verified bundle is on the package host and was absent from the matching GitHub
release assets.

Run in a download directory; continue to installation only if the checksum reports
`OK`:

```sh
candidate=0.6.3-staging.37959504785.1.4da6948415b9
base="https://packages.staging.dockpipe.com/packages/candidates/$candidate"
bundle=dockpipe-desktop_0.6.3_linux_amd64.flatpak
curl -fSL "$base/$bundle" -o "$bundle"
curl -fsSL "$base/SHA256SUMS.txt" -o SHA256SUMS.txt
awk -v file="$bundle" '$2 == file { print }' SHA256SUMS.txt > "$bundle.sha256"
sha256sum --check "$bundle.sha256"
```

```sh
flatpak install --user ./dockpipe-desktop_0.6.3_linux_amd64.flatpak
flatpak run com.dockpipe.Dockpipe
flatpak run --command=dockpipe com.dockpipe.Dockpipe --version
```

The last command invokes the bundled CLI; installation does not add a native
`dockpipe` command to your host `PATH`. Normal app XDG data is separate from native
Dockpipe data. Project-local packages and explicit data-root overrides still take
precedence, so check those when sharing a project between native and Flatpak runs.

**Updates:** this is a standalone bundle, without an updating Dockpipe Flatpak
remote. `flatpak update` can update the KDE runtime from Flathub, but does not fetch
new Dockpipe bundles from the staging JSON catalog. Download and verify the newer
candidate as above, close the launcher and finish active workflows, then run
`flatpak install --user --or-update ./dockpipe-desktop_VERSION_linux_amd64.flatpak`
with the new filename. See Flatpak's [bundle guidance](https://docs.flatpak.org/en/latest/single-file-bundles.html)
and [install options](https://docs.flatpak.org/en/latest/flatpak-command-reference.html#flatpak-install).

**Packages:** in the Flatpak launcher, choose **Settings → Package Remotes → Use staging**,
save, then open **Packages → Marketplace**. Selection uses the running app's
architecture and KDE runtime branch, currently `linux-amd64-flatpak-org.kde.Platform-6.10`.
The verified catalog has 62 entries: core plus 61 optional artifacts, not 62 bundled
apps. Native Dockpipe still selects native stores on the same machine. Missing or
mismatched Flatpak stores fail explicitly, including pinned store URLs; there is no
automatic native fallback. Marketplace packages are ordinary Dockpipe archives,
not separate Flatpak apps, and are independent of the app-bundle update procedure.

**Host integrations and limits:** container clients require a reachable host Docker
or compatible Podman API socket; the app does not install or start an engine.
Explicit Docker environment settings take priority over detected sockets. Editors,
Codex/Claude, 1Password, QEMU and service/GPU management use explicit host bridges
and require the host tools and authentication. The app requests home access,
network, display/graphics, engine sockets and host execution permission. Local amd64
tests and hosted CI do not qualify native Bazzite, full Podman/SELinux behavior,
authenticated providers, VM boot or universal workflow parity. See the
[host integration and qualification details](../release/packaging/desktop/flatpak/README.md).

## Signed APT repository

For Ubuntu/Pop!_OS staging installations, configure the permanent staging source
once. Download the public key and inspect its fingerprint:

```sh
curl -fsSL https://packages.staging.dockpipe.com/apt/dockpipe-archive-keyring.gpg -o /tmp/dockpipe-staging-keyring.gpg
gpg --show-keys --with-fingerprint /tmp/dockpipe-staging-keyring.gpg
```

Confirm fingerprint `7295FA3FC25A3998E146D0779EAF522778B9C909` before proceeding:

```sh
sudo install -m 0644 /tmp/dockpipe-staging-keyring.gpg /usr/share/keyrings/dockpipe-staging.gpg
printf '%s\n' 'deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/dockpipe-staging.gpg] https://packages.staging.dockpipe.com/apt staging main' | sudo tee /etc/apt/sources.list.d/dockpipe-staging.list >/dev/null
sudo apt update
sudo apt install dockpipe-desktop
```

Use `sudo apt install dockpipe` for CLI only. The desktop package installs its
matching CLI dependency. Updates use:

```sh
sudo apt update
sudo apt upgrade
```

Do not configure staging and production APT sources together. If you previously
used a candidate-specific staging source, replace that source with the permanent
one above to receive later candidates. Removing `dockpipe-desktop` retains the CLI.

## Direct downloads and complete package stores

Read the [staging pointer](https://packages.staging.dockpipe.com/packages/latest.json)
and follow its `manifest` path relative to `https://packages.staging.dockpipe.com/`.
The release manifest lists numeric `version`, per-platform `downloads`, and
`stores`. Download filenames are relative to the manifest's directory.

Choose `amd64` for x86_64 or `arm64` for aarch64/Apple Silicon. Verify downloaded
files against `SHA256SUMS.txt` from that same candidate before installing. On Linux
use `sha256sum`; on macOS use `shasum -a 256`; on Windows use `Get-FileHash -Algorithm SHA256`.

The native CLI installer can use an explicit candidate. This example is pinned to
the same verified 0.6.3 staging candidate as the Flatpak instructions:

```sh
candidate=0.6.3-staging.37959504785.1.4da6948415b9
base="https://packages.staging.dockpipe.com/packages/candidates/$candidate"
curl -fsSL "$base/install.sh" -o /tmp/dockpipe-install.sh
DOCKPIPE_VERSION=0.6.3 DOCKPIPE_DOWNLOAD_BASE="$base" DOCKPIPE_INSTALL_MODE=portable sh /tmp/dockpipe-install.sh
```

The installer verifies its CLI payload checksum. It installs the portable CLI in
`~/.local/bin` by default; add that directory to your shell's `PATH` if needed.
Use the candidate's numeric version for `DOCKPIPE_VERSION`, not its staging suffix.
This script installs CLI/core, not the desktop launcher.

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

Native Windows workflows use Git for Windows for Bash and Docker Desktop for
container execution. WSL is optional; a native install does not require a Linux
Dockpipe binary. Follow [the WSL bridge guide](runtime/wsl-windows.md) only when you
want execution inside a WSL distribution. Dockpipe is not listed in the default
winget catalog yet.

## macOS CLI / Remote Worker

```sh
brew install dockpipe-industries/dockpipe/dockpipe-staging
dockpipe --version
```

This formula installs CLI/core. The desktop cask above adds the launcher. Update
with `brew update` followed by `brew upgrade dockpipe-staging`. The stable formula
is not published yet; native tarballs are another option.

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

## Browsing staging packages in the launcher

Open **Settings → Package Remotes → Use staging**, save, then open
**Packages → Marketplace**. The staging origin is
`https://packages.staging.dockpipe.com`; new launcher settings otherwise default to
production at `https://packages.dockpipe.com`. Select one channel explicitly.

Marketplace verifies checksums and installs selected packages into your user
store. Base installers include core only. See [Find and use packages](packages/package-quickstart.md)
for dependencies, terminal installation and removal.

## Bundled templates (no extra install tree)

The CLI and installer supply the required runtime assets. You do not need to copy
`templates/`, clone Dockpipe, or compile its source to run an installed workflow.
Dockpipe may materialize embedded assets in its user cache automatically.

Your own source workflows live under `workflows/` in your project. Optional packages
live in the user store; the [package model](packages/package-model.md) explains
storage and precedence when you need advanced configuration.

## Building or maintaining Dockpipe

Source builds and contributor setup are covered in [CONTRIBUTING.md](../CONTRIBUTING.md).
Installer construction and release publication belong to the
[release documentation](../release/README.md). They are not installation steps for
an end user.

# Installing dockpipe

**New to dockpipe?** Run **`dockpipe -- pwd`** after install, then read **[onboarding.md](onboarding.md)**. If something fails, **`dockpipe doctor`** checks **bash**, **Docker**, and bundled assets.

Choose **Desktop** for the launcher and CLI together, or **CLI / Remote Worker** for terminals, servers and CI. Docker is needed only for workflows that use containers. Host shell workflows need bash; Git for Windows supplies bash on Windows.

| Platform | Desktop | CLI / Remote Worker |
| --- | --- | --- |
| macOS Apple Silicon / Intel | DMG installer, or Homebrew desktop cask | Homebrew formula or native tarball |
| Windows x64 | MSI with launcher selected by default | ZIP, or MSI with only the CLI feature |
| Ubuntu 24.04 amd64 / arm64 | `dockpipe-desktop` DEB plus `dockpipe` | `dockpipe` DEB; RPM, APK, Arch and portable CLI packages also available |

Desktop packages are available in the staging channel. The Homebrew CLI formula and desktop cask
passed [native installation checks on Apple Silicon and Intel](https://github.com/Dockpipe-Industries/homebrew-dockpipe/actions/runs/37419531103)
before publication. Existing CLI-only installations do not acquire a launcher automatically; install
the desktop cask below to add it. Apple Developer ID signing and notarization remain pending.

## Desktop installation

**macOS:** download `dockpipe-desktop_VERSION_darwin_arm64.dmg` for Apple Silicon or `dockpipe-desktop_VERSION_darwin_amd64.dmg` for Intel, verify it against that release's `SHA256SUMS.txt`, open it, and run **Install DockPipe.pkg**. Apple's Installer installs **DockPipe.app** in Applications and **`dockpipe`** in `/usr/local/bin`. Open DockPipe from Applications; new terminals can run `dockpipe --version`. The desktop app contains the matching CLI, Qt runtime and complete native package store. macOS 13 or newer is required.

Use one installation method. The DMG installer refuses an existing foreign CLI or app; users with the Homebrew CLI should use the cask below. Re-running the DMG installer updates an installation owned by that installer. Locally built and current CI staging DMGs use ad-hoc signatures unless the maintainer supplies Developer ID signing and notarization. They are not yet normal Gatekeeper-approved public downloads; no security-setting changes are part of installation.

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

**Windows:** run `dockpipe_VERSION_windows_amd64.msi`. The default installs the CLI, full Qt launcher runtime and a Start menu shortcut. Deselect **DockPipe Launcher** for a CLI-only install, or change the feature later through the installer's Modify option. The ZIP is always CLI-only.

**Ubuntu 24.04:** after configuring the signed APT source below, run `sudo apt install dockpipe-desktop`; it installs the matching CLI dependency. For downloaded DEBs, install both together:

```sh
sudo apt install ./dockpipe_VERSION_amd64.deb ./dockpipe-desktop_VERSION_amd64.deb
```

Use `arm64` filenames on ARM64. Launch **DockPipe** from the application menu. Removing `dockpipe-desktop` leaves the CLI available. Desktop RPM, APK and Arch packages are not currently provided; those formats remain CLI-only.

### Bundled templates (no extra install tree)

The binary **embeds** **`templates/`** (including **`templates/core/`**: template **assets**, runtimes, resolvers, strategies) and repository root **`assets/entrypoint.sh`**. On first use it unpacks to the **user cache** with this **materialized** layout:

```text
dockpipe/
  core/          # assets/, resolvers/, runtimes/, strategies/
  workflows/     # one dir per bundled workflow (same as templates/<name>/ in a checkout)
```

**`assets/`** (embedded **`entrypoint.sh`**) and **`version`** sit beside **`dockpipe/`** at the cache root. The CLI resolves **either** this layout **or** a normal git checkout (**`templates/`** + **`templates/core/`** + repository **`assets/`**). You do **not** need a clone of the dockpipe repo next to the binary for **`--workflow`** or default images.

- **`DOCKPIPE_REPO_ROOT`** — optional override to point at a **dockpipe source tree** (e.g. when editing templates).
- **`DOCKPIPE_BUNDLED_CACHE`** — optional parent directory for the `dockpipe/bundled-*` folder (tests, custom cache location).

User-created workflow files from **`dockpipe init`** live in your project, typically under **`workflows/`**. Legacy template-oriented paths still exist in some maintainer and compatibility flows, but they are not the normal starting point for new projects.

---

## Signed APT repository

Once the 0.6 release and public hostname are live, Debian/Ubuntu users can install and receive updates from the signed repository:

```bash
curl -fsSL https://packages.dockpipe.com/apt/dockpipe-archive-keyring.gpg -o /tmp/dockpipe-archive-keyring.gpg
sudo install -m 0644 /tmp/dockpipe-archive-keyring.gpg /usr/share/keyrings/dockpipe-archive-keyring.gpg
printf '%s\n' 'deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/dockpipe-archive-keyring.gpg] https://packages.dockpipe.com/apt stable main' | sudo tee /etc/apt/sources.list.d/dockpipe.list
sudo apt-get update
sudo apt-get install dockpipe
```

The repository public key is restricted to this source by `signed-by`. APT validates the signed indexes and package hashes. The service must be provisioned and the first signed release published before these commands work.

## Direct downloads and complete package stores

GitHub release assets are also mirrored at `https://packages.dockpipe.com/packages/releases/VERSION/`. Linux and macOS can use the checksum-verifying installer against that origin:

```bash
curl -fsSL https://packages.dockpipe.com/packages/releases/0.6.0/install.sh -o /tmp/dockpipe-install.sh
DOCKPIPE_VERSION=0.6.0 DOCKPIPE_DOWNLOAD_BASE=https://packages.dockpipe.com/packages/releases/0.6.0 sh /tmp/dockpipe-install.sh
```

The CLI installer installs the CLI and core. Additional workflows and resolvers are in the complete native package-store bundle `dockpipe-packages_VERSION_OS-ARCH.tar.gz` (`linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, or `windows-amd64`). Verify the bundle against `SHA256SUMS.txt`, extract it to a local directory, and reference that directory through `packages.sources` in your project's `dockpipe.config.json`:

```json
{"schema":1,"packages":{"sources":[{"kind":"tarball_dir","path":"/absolute/path/to/extracted-store"}]}}
```

An Apple Silicon Mac uses `darwin-arm64`; it does not need to clone DockPipe or install Go to use the compiled CLI and store. Provider CLIs such as `cloudflared` or `op` are still required by their resolvers. On Windows use an absolute Windows path in the JSON. Each platform's individual tarballs and `packages-store-manifest.json` are also served under `stores/OS-ARCH/` in that version directory. Do not mix native helpers from different platforms.

---

## Install the .deb (Linux)

1. Download the latest `.deb` for your CPU from [Releases](https://github.com/Dockpipe-Industries/dockpipe/releases):
   - **x86_64** → `dockpipe_*_amd64.deb`
   - **aarch64** (ARM64 Linux, e.g. many cloud VMs / Raspberry Pi OS 64-bit) → `dockpipe_*_arm64.deb`  
   The two packages are **not** interchangeable (each contains a native Go binary). The `.deb` installs **`/usr/bin/dockpipe`** only (bundled assets are inside the binary; no `/usr/lib/dockpipe` layout).
2. Install:

   ```bash
   sudo dpkg -i dockpipe_*_amd64.deb    # or *_arm64.deb on aarch64
   ```

3. If `dpkg` reports missing dependencies (e.g. Docker):

   ```bash
   sudo apt-get install -f
   ```

Using `dpkg -i` avoids apt sandbox warnings when the .deb is in your home directory; `apt install ./file.deb` there can show a permission notice (apt’s `_apt` user can’t read the file).

**Upgrades:** download the new .deb (same arch as before) and run `sudo dpkg -i dockpipe_*_amd64.deb` or `dockpipe_*_arm64.deb` as appropriate.

**Requirements:** **amd64** or **arm64** package matching your machine. **`bash`** on the host, and **git** for clone/worktree/commit-on-host workflows. Container workflows additionally need **Docker** (`docker.io` or `docker-ce`). Install Docker if needed:

```bash
sudo apt-get install docker.io
```

**Persistent data:** By default dockpipe mounts a named volume `dockpipe-data` at `/dockpipe-data` and sets `HOME` there so tool state (e.g. first-time login) persists. Use `--data-vol <name>`, `--data-dir /path`, or `--no-data` to change or disable. If a tool exits immediately with the default volume, try `--no-data` or `--reinit` to get a fresh volume.

**Workflow YAML:** Multi-step templates (`steps:`, async groups, `outputs:`) are documented in **[workflows/workflow-yaml.md](workflows/workflow-yaml.md)**.

---

## Alpine, Fedora/RHEL, Arch Linux (release packages)

Releases ship **`.apk`** (Alpine), **`.rpm`** (Fedora, RHEL-compatible), and **`.pkg.tar.zst`** (Arch), for **amd64** and **arm64**, alongside **`.deb`** and **`.tar.gz`**.

| Format | Example install |
|--------|-----------------|
| **Alpine** | `sudo apk add --allow-untrusted ./dockpipe_*_linux_amd64.apk` |
| **Fedora** | `sudo dnf install ./dockpipe_*_linux_amd64.rpm` |
| **Arch** | `sudo pacman -U ./dockpipe_*_linux_amd64.pkg.tar.zst` |

Packages declare **`bash`** and **`git`** as dependencies; **Docker** is still something you install the usual way for that distro (`docker` / `docker-cli` / `podman` + compose, etc.) — same as the `.deb` story.

---

## One-liner Linux and macOS install

From a network-connected shell (uses [GitHub Releases](https://github.com/Dockpipe-Industries/dockpipe/releases); detects distro from `/etc/os-release`, otherwise drops the **portable `.tar.gz`** into **`~/.local/bin`**):

```bash
curl -fsSL https://raw.githubusercontent.com/Dockpipe-Industries/dockpipe/master/release/packaging/linux/install.sh | sh
```

Pin a version: `DOCKPIPE_VERSION=0.6.0 curl -fsSL … | sh`  
Forks: `DOCKPIPE_REPO=you/dockpipe curl -fsSL … | sh`

Script: **[release/packaging/linux/install.sh](../release/packaging/linux/install.sh)**.

---

## Or run from source (Linux or macOS, no root)

The CLI is built with **Go** matching **`go.mod`** (currently **1.25**; see `toolchain` there) (`go build -o src/bin/dockpipe.bin ./src/cmd` or **`make`**). The `src/bin/dockpipe` script runs the binary if present, otherwise `go run`.

```bash
git clone https://github.com/Dockpipe-Industries/dockpipe.git
cd dockpipe
make   # or: go build -o src/bin/dockpipe.bin ./src/cmd
export PATH="$PATH:$(pwd)/bin"
dockpipe -- ls -la
```

**Windows `dockpipe.exe` from a Unix dev machine:** from the **repo root**, run **`make build-windows`** — output is **`src/bin/dockpipe.exe`** (gitignored). Copy that file to your PC; do not rely on a hardcoded path on someone else’s machine.

---

## Windows (Docker Desktop + native `dockpipe.exe`)

**Required:** **Docker Desktop**, **`bash.exe`** on `PATH`, and **`dockpipe.exe`** on `PATH`. Dockpipe always invokes **bash** on the host; **Git for Windows** is the usual way to get **`bash.exe`** (and **`git.exe`**) together. Docker Desktop does **not** ship bash. If you use **WSL**, put **Git’s `…\Git\bin`** **before** `C:\Windows\System32` on `PATH` so **`bash`** is **Git Bash**, not **WSL’s** `bash.exe` (dockpipe prefers Git Bash when installed; otherwise it uses WSL path rules).

**Host `git`:** additionally required for **clone / worktree / commit-on-host** (e.g. **`--repo`**, **`clone-worktree.sh`**). Git for Windows covers that for most users.

**Local / gitignored config in worktrees** (e.g. `.env`, `appsettings.Development.json`): see **[runtime/worktree-include.md](runtime/worktree-include.md)** — use **`.dockpipe-worktreeinclude`** or **`.worktreeinclude`** so dockpipe copies those paths into the worktree after it is created.

You do **not** need a WSL distro or Linux `dockpipe` unless you opt into **`DOCKPIPE_USE_WSL_BRIDGE=1`**. If **`bash`** is missing but **WSL** is installed, the CLI may offer to **re-run through WSL** (interactive). Optional advanced note: **[runtime/wsl-windows.md](runtime/wsl-windows.md)** only if you still use **git bundle** handoff between WSL and Windows clones — not part of the default native-Windows flow.

### Install `dockpipe.exe` on Windows

Add **`dockpipe.exe`** to `PATH` (**install script** or **zip**; **MSI** when published on a given release). Install **Git for Windows** (or another **`bash`** + **`git`** on `PATH`). Start Docker Desktop when running container workflows.

**Optional WSL bridge:** if you set **`DOCKPIPE_USE_WSL_BRIDGE=1`**, commands are forwarded into WSL. Then you also need **`dockpipe` installed inside that distro** and should run **`dockpipe windows setup`** once.

**Automated (recommended):** downloads from the latest release — prefers **MSI** when the release includes it, otherwise **zip** — verifies **`SHA256SUMS.txt`** when available, installs **per-user** (no admin):

```powershell
irm https://raw.githubusercontent.com/Dockpipe-Industries/dockpipe/master/release/packaging/windows/install.ps1 | iex
```

Pin a version: save [release/packaging/windows/install.ps1](https://github.com/Dockpipe-Industries/dockpipe/blob/master/release/packaging/windows/install.ps1) and run `.\install.ps1 -Version 0.6.0`.

**Manual:** from [Releases](https://github.com/Dockpipe-Industries/dockpipe/releases):

- **`dockpipe_<version>_windows_amd64.zip`** — unzip and add the folder to `PATH`.
- **`dockpipe_<version>_windows_amd64.msi`** — **when published** for that release: double-click, or `msiexec /i .\….msi /qn` (adds `%LOCALAPPDATA%\dockpipe` to your user **PATH**). Some releases ship **zip only** until MSI is enabled for that tag.

**winget:** not in the default Microsoft catalog until a manifest is accepted; see **[release/packaging/winget/README.md](../release/packaging/winget/README.md)** for maintainers and future `winget install`.

Open a **new** terminal after install so `PATH` is picked up.

### Daily use from Windows

With **`dockpipe.exe`** on `PATH` (install script, zip, or MSI when available). From **PowerShell or CMD**, `cd` to your repo and run the same CLI as on Linux, e.g.:

```powershell
cd C:\Users\you\src\myrepo
dockpipe -- echo ok
```

**Native mode (default):** `dockpipe` runs on Windows; **`docker`** comes from Docker Desktop; **`bash`** (and usually **`git`**) from Git for Windows. **`git`** is only needed for worktree/repo flows (see above). No WSL shell or Linux `dockpipe` required unless you use the bridge.

### Optional: WSL bridge (`DOCKPIPE_USE_WSL_BRIDGE=1`)

Set the environment variable **for the session** (or persist it in **Windows user environment variables** / your shell profile) so **`dockpipe.exe` forwards** into WSL: cwd is mapped with `wslpath`, then **`dockpipe`** runs inside the distro from **`dockpipe windows setup`** (or the first listed distro).

```powershell
# PowerShell — this session only
$env:DOCKPIPE_USE_WSL_BRIDGE = "1"
```

```bat
REM cmd.exe — this session only
set DOCKPIPE_USE_WSL_BRIDGE=1
```

- **`dockpipe windows …`** always runs **only on Windows** (setup / doctor).
- With the bridge, path-like flags are rewritten to WSL paths before the inner `dockpipe` sees them. Arguments after **`--`** are not rewritten.

**One-time WSL bootstrap** (only if you use the bridge):

```powershell
dockpipe windows setup
```

What setup does: picks a distro, saves it to `%APPDATA%\dockpipe\windows-config.env`, bootstraps `~/.dockpipe/windows-host.env` in WSL, optionally runs `--install-command`, verifies `dockpipe` in that distro.

**Automated path for testers** (installs **WSL + Alpine** by default — small footprint — then **Linux `dockpipe`** from the latest GitHub release into `~/.local/bin` inside WSL — may prompt for **Administrator** or require a **reboot**). If **`wsl --install -d Alpine`** is not listed on your PC, use **`--distro Ubuntu`**.

```powershell
dockpipe windows setup --bootstrap-wsl --distro Alpine --non-interactive --install-dockpipe
```

The Windows **`install.ps1`** script runs that after installing **`dockpipe.exe`** unless you pass **`-SkipWSLSetup`**.

```powershell
dockpipe windows setup --distro Ubuntu --install-command "<your install command>" --non-interactive
dockpipe windows doctor
```

**Manual QA:** **[manual-qa.md](manual-qa.md)**.

---

## macOS CLI / Remote Worker

The published staging formula installs the CLI and complete native package store:

```sh
brew install dockpipe-industries/dockpipe/dockpipe-staging
dockpipe --version
```

The command is `dockpipe`; this formula alone does not install the desktop launcher. Use the desktop cask or DMG described above for both. The stable `dockpipe` formula is not published yet. Native release tarballs and the checksum-verifying CLI installer are alternatives that do not require Go or a source checkout.

See [staging installation](../release/docs/staging.md#homebrew-on-a-test-mac), [tap maintenance](../release/packaging/homebrew/README.md), and [release automation](../release/docs/releasing.md).

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

DockPipe uses the [Docker client's context selection](https://docs.docker.com/engine/manage-resources/contexts/).
Use `docker context ls` to find your profile's context, then `docker context use <name>` if you want
to change the default for both terminals and the launcher. For one terminal command, use
`DOCKER_CONTEXT=<name> dockpipe doctor`. Existing `DOCKER_HOST`, `DOCKER_CONTEXT`, and `DOCKER_CONFIG`
settings retain Docker's normal precedence; DockPipe does not switch contexts or start a VM.

The launcher adds `/opt/homebrew/bin` and `/usr/local/bin` after inherited `PATH` entries so Finder
launches can discover Homebrew tools. Shell-only exports are not inherited by Finder; use Docker's
saved context for Finder launches, or start the app executable from a configured terminal. Custom
Homebrew locations still need an explicit `PATH`. If Docker is unreachable, start the selected Colima
profile and verify `docker version` before retrying `dockpipe doctor`.

The compatibility changes are implemented in source. Native Colima container, mount, networking,
and Compose qualification on macOS remains pending; the earlier desktop installer dry run does not
cover this integration.

---

## Building the .deb (for maintainers)

From the repo root:

```bash
./release/packaging/build-deb.sh [version] [amd64|arm64]   # default: 0.6.0 amd64
./release/packaging/build-deb-all.sh [version]             # both amd64 + arm64
# Output: release/packaging/build/dockpipe_<version>_{amd64,arm64}.deb
```

Attach that file to a GitHub Release. If we add a proper APT repo later, we’ll document it here.

### Browsing staging packages in the launcher

After installing the CLI and launcher together, open **Settings → Package Remotes → Use staging**,
save, then open **Packages → Marketplace**. The staging origin is
`https://packages.staging.dockpipe.com`; new launcher settings otherwise default to production at
`https://packages.dockpipe.com`. Select one remote explicitly; the launcher does not mix channels.

Marketplace installs individual packages into your user store after checksum verification.
The full Brew/DMG installation already includes a matching package store. See
[remote package installation](packages/package-model.md#remote-catalogs-and-user-installation)
for the CLI contract and dependency behavior.

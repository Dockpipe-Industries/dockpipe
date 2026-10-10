# Dockpipe Launcher

Cross-platform **Qt 6** system-tray app: save **contexts** (folder + resolver / strategy / runtime), **launch** or **stop** `dockpipe` subprocesses, **open logs** and folders. It does **not** run workflows inside the GUI; all execution stays in **Dockpipe** (and optionally **DorkPipe** later).

This tree lives under **`src/app/tooling/dockpipe-launcher/`** as first-party Dockpipe tooling. It drives the Dockpipe CLI but is not part of the engine (**`src/lib/`**, **`src/cmd/`**). The tray/window icon comes from **`resources/images/dockpipe-launcher.png`** and Linux desktop installs use the generated **`resources/icons/hicolor/`** size set.

## Requirements

- **CMake** 3.16+
- **Qt 6** (`Widgets` + **`Network`**) — install dev packages (e.g. `qt6-base-dev` on Debian/Ubuntu) or use the [Qt Online Installer](https://www.qt.io/download) and set **`CMAKE_PREFIX_PATH`** to the Qt 6 prefix.
- **OpenGL / EGL development libraries** — Qt 6 Gui pulls in **WrapOpenGL**. On Ubuntu/Pop!_OS, if CMake says `WrapOpenGL could not be found` or `Qt6Gui_FOUND` is FALSE, install **`libgl1-mesa-dev`** and **`libegl1-mesa-dev`** (see Build section below).
- **`dockpipe`** on **`PATH`** (or set **dockpipe binary** in each context’s settings).
- Host tools Dockpipe already needs: **`bash`**, **`docker`**, **`git`** — see [docs/install.md](../../../../docs/install.md).

On macOS, Finder launches append the standard Homebrew binary directories to the inherited `PATH`.
Docker uses the user's saved context or inherited endpoint settings, including a Colima Docker
profile. The launcher does not start or switch container providers. See
[Colima setup](../../../../docs/install.md#containers-with-colima).

## Package remotes

Open **Settings → Package Remotes** and choose **Use production** or **Use staging**.
New settings default to `https://packages.dockpipe.com`; an explicitly saved empty list
keeps remote browsing disabled. **Use staging** selects `https://packages.staging.dockpipe.com`.
Installing a staging launcher does not silently change an existing remote selection.
Custom remotes can be HTTPS manifest URLs; official origin-only URLs expand to
`/packages/latest.json`. Multiple saved remotes appear in the package manager's selector.

**Packages → Marketplace** reads the selected catalog for this machine's platform, supports
search, and installs the selected package into the user store. Requests run asynchronously;
errors stay visible and operations can be cancelled. Install pins the displayed store and
checksum. A matching installed version is labelled **Installed**, with duplicate installation disabled;
this does not assert identical content across staging candidates. **Cancel operation** is
shown only during active requests and stops both inventory and remote requests. The details explain replacement and dependencies.

**Installed** uses the CLI inventory, including user packages and packages supplied by the
system installer, Brew, or app bundle. Optional user-store archives offer **Uninstall**, which
retains package data and settings. Core and externally managed entries explain why they cannot
be removed here. Normal installers include only core. Closing Packages refreshes the launcher's app list.
The saved **Global Root Override** now applies to both package commands and workflow children;
an inherited `DOCKPIPE_GLOBAL_ROOT` remains authoritative. The launcher delegates all package
networking, integrity checks, and installation to `dockpipe package`; see the
[package contract](../../../../docs/packages/package-model.md#remote-catalogs-and-user-installation).

## Build

Development builds show `dev+<12-character-commit>` and append `.dirty` for uncommitted
changes. The identity is captured at build time, refreshed during incremental builds,
and shown by `--version`, Help → About, and the development window title. Git-free
source builds show `dev+unknown`. Release packaging supplies
`-DDOCKPIPE_RELEASE_VERSION=<version>` and retains that exact version. To clear an old
explicit version from an existing build directory, configure with
`-U DOCKPIPE_RELEASE_VERSION`. Numeric macOS bundle metadata remains separate from the
development display identity.

`CMakeLists.txt` lives under **`src/app/tooling/dockpipe-launcher/`**. Run CMake with that directory as the **source** (or `cd` there first).

**Fastest — from the repo root:** `cmake -S src/app/tooling/dockpipe-launcher -B src/app/tooling/dockpipe-launcher/build && cmake --build src/app/tooling/dockpipe-launcher/build` (writes **`src/app/tooling/dockpipe-launcher/build/`**).

**Option A — from the repo root (CMake by hand):**

```bash
cd ~/source/dockpipe
sudo apt install cmake build-essential qt6-base-dev   # Pop!_OS / Ubuntu: Qt 6 Widgets + dev tools
cmake -S src/app/tooling/dockpipe-launcher -B src/app/tooling/dockpipe-launcher/build
cmake --build src/app/tooling/dockpipe-launcher/build
./src/app/tooling/dockpipe-launcher/build/dockpipe-launcher
```

**Option B — from the launcher directory:**

```bash
cd ~/source/dockpipe/src/app/tooling/dockpipe-launcher
cmake -B build
cmake --build build
./build/dockpipe-launcher
```

To install a Linux desktop entry and the full icon-theme size set for app launchers / docks (for example Pop OS / GNOME):

```bash
make install-dockpipe-launcher-global
```

If you use the **Qt Online Installer** instead of distro packages, point CMake at that kit (replace with your real path):

```bash
cmake -S src/app/tooling/dockpipe-launcher -B src/app/tooling/dockpipe-launcher/build \
  -DCMAKE_PREFIX_PATH="$HOME/Qt/6.8.0/gcc_64"
```

Do **not** use the placeholder `/path/to/Qt/6.x/...` literally — it must be a directory that contains **`Qt6Config.cmake`** (or install `qt6-base-dev` and omit `CMAKE_PREFIX_PATH`).

**If configuration failed earlier** (stale cache): remove the build dir and re-run CMake after installing `libgl1-mesa-dev` / `libegl1-mesa-dev`:

```bash
rm -rf src/app/tooling/dockpipe-launcher/build
cmake -S src/app/tooling/dockpipe-launcher -B src/app/tooling/dockpipe-launcher/build
cmake --build src/app/tooling/dockpipe-launcher/build
```

## LGPL / Qt

Qt is available under **LGPL** and commercially. If you **ship binaries**, comply with Qt’s license terms (e.g. dynamic linking and relinking for LGPL) or use a **commercial Qt license**. This README is not legal advice.

## Extra `dockpipe` env

**Edit context…** includes **Extra dockpipe env** (one `KEY=value` per line). Each line is passed to dockpipe as **`--env`**, same as the CLI.

**`src/scripts/docker-package-cache-demo.sh`** demonstrates named Docker volumes for persistent APT caches inside containers. Add env lines your workflows read, or pass **`dockpipe --mount`** when you run from the CLI; Pipeon can supply extra env lines if your wrapper reads them.

## Prompt bridge

The launcher now understands the shell SDK prompt bridge from **`dockpipe_sdk prompt ...`**. Package scripts can emit a framework prompt once and get:

- terminal interaction in plain CLI runs
- native launcher dialogs when the same workflow is started from Dockpipe Launcher

The launcher sets **`DOCKPIPE_SDK_PROMPT_MODE=json`** for managed `dockpipe` subprocesses, watches for prompt events on process output, and writes the user’s response back to the running workflow over stdin.

That includes **file prompts**: when a package or runtime emits `dockpipe_sdk prompt file ...`, the launcher renders a native file or directory picker instead of forcing the user to paste a path manually.

## Workspace navigation

The sidebar provides **Apps**, **Workflows**, **Machines**, **Activity**, and **Docker**.
The workspace selector above every page opens a folder or switches to a recent project.
That folder is passed to Dockpipe as `--workdir`.

- **Apps** lists installed workflows marked `category: app`. Each card offers **Open app** and
  **Configure**; double-click and keyboard activation also launch the selected app.
  **View → Icon grid / Compact list** changes presentation. Empty workspaces link directly to
  the package manager. **Refresh** or **F5** reloads the catalog.
- **Workflows** shows the complete project catalog, a **Run on** machine selector, and run output.
  Right-click a workflow for additional actions.
- **Docker** is a single independent page with Containers, Networks, and Volumes tabs. It shows
  local engine objects and refreshes while selected. Container context menus provide
  Inspect, Start, Stop, and Refresh.

The existing `basic` / `advanced` settings values remain compatible with Apps / Workflows.

## Data locations

| OS      | Config / contexts                          |
|---------|---------------------------------------------|
| Linux   | `~/.config/dockpipe/` or `~/.config/dockpipe-launcher/` fallback |
| macOS   | `~/Library/Application Support/dockpipe/` |
| Windows | `%APPDATA%\\dockpipe\\` |

- **`contexts.json`** — saved contexts.
- **`launcher.json`** — UI mode (`basic` / `advanced`), Basic view (`icons` / `list`), last **project folder** for Basic mode.
- **`logs/`** — per-launch log files for `dockpipe` stdout/stderr.

## Manual QA (short)

1. **Tray:** Icon appears; Show / hide window; Quit exits the app.
2. **Add folder:** New context; **Launch** starts `dockpipe` (requires image/build for workflows like `vscode`).
3. **Parallel:** Two contexts, different workdirs — both **Launch**; both show **running**; **Stop** each.
4. **Git:** **Refresh worktrees** on a repo with multiple worktrees adds missing paths as contexts.
5. **Stop all for repo:** Stops every **running** context whose `git rev-parse --show-toplevel` matches (includes linked worktrees that live outside the main checkout directory).
6. **Linux:** Verify tray under **X11** and **Wayland** (may depend on desktop).

## UI

The window uses **Qt Fusion** plus stylesheets embedded in **`dockpipe-launcher.qrc`**: shared `resources/theme/pipeon.qss` plus **`pipeon-light.qss`** or **`pipeon-dark.qss`**. Light/dark is chosen from **`QStyleHints::colorScheme`** when available (Qt 6.5+), and on **Linux** also from **`gsettings`** (`org.gnome.desktop.interface` color-scheme / gtk-theme), **`~/.config/gtk-3.0/settings.ini`** (`gtk-application-prefer-dark-theme`), **KDE** `~/.config/kdeglobals` `ColorScheme`, **`GTK_THEME`**, then palette luminance as a last resort. When dark is selected, the app applies a **Fusion dark palette** so backgrounds match the stylesheet (Qt often defaults to a light palette on Linux). **Light** mode uses stronger text/badge contrast. The stylesheet is **re-applied when the system color scheme changes** (Qt 6.5+). The main header groups **primary** session actions (launch, relaunch, stop, add folder) separately from **secondary** utilities (edit, refresh worktrees, logs, folder, remove, stop all for repo). The context list uses row widgets with a status badge; **Edit context** opens a grouped dialog with combo boxes populated from the dockpipe repo when `DOCKPIPE_REPO_ROOT` or the context workdir resolves to a checkout.

## Scope

Per design: the launcher only **controls** sessions. It does **not** replace **DorkPipe** orchestration or embed Ollama/containers.

## Workflow workspace and remote machines

The sidebar exposes Apps, Workflows, Machines, Activity, and Docker. Apps continue to use
`category: app` metadata and the existing local execution path. Workflows show a
**Run on** selector: this computer, or machines reported as paired by the configured
broker. Paired is an enrollment state, not a live-presence claim.

Machines uses the public `dockpipe remote` commands and CLI-owned private state.
Its default view lists managed machines and explains invitation, paired, and removed-access
states. **Add another machine** guides the managing computer through opening pairing,
continuing on the other computer, and comparing the verification code. Requests refresh
every five seconds while that page is visible; approval requires selecting a request and
confirming the code. On the other computer, **Connect this computer** accepts the HTTPS
address and machine name with explicit workflow-execution consent. Technical output is
collapsed under **Show details**. No invitation file or SSH
is required. Start the worker user service after pairing. Cancelling stops the
local wait; the pending request can still be denied or closed on the broker and
otherwise expires. Closing the launcher does not revoke existing enrollment.

The connection summary shows the recorded provider name, resolver ID/version, hosting mode
and address. Older setups explicitly show that their resolver was not recorded. This is
configuration information, not a health check. **Set up remote access…** discovers installed
remote providers from the public catalog. Edge providers run a broker on this computer;
hosted providers use package-owned sign-in without hostname fields. Selecting a provider
does not switch the active connection. Dockpipe Cloud is not yet an implemented provider.

Setup opens a real terminal for the selected installed resolver,
dependency prompts and browser authentication. Launching a terminal is not setup
success; completion is reported there. Linux uses an available desktop terminal;
Flatpak asks the host terminal to run the same app's CLI through `flatpak run`.
macOS uses Terminal. Provider behavior remains in the resolver package.

Remote workflow launch previews the selected workflow tree, extra source paths,
explicit unpacked package dependencies and requested results through `remote submit
--dry-run`. Submission binds `--expected-digest` to that preview. Editing the
selection invalidates approval; changing source contents makes the CLI reject
submission. Runtime/resolver settings come from the delivered YAML. Local launch
overrides are not silently dropped or sent. Credentials must never be included.

Activity shows launcher-owned local sessions and broker jobs. Remote jobs refresh
while the Remote tab is visible after the first successful refresh. Cancellation
uses the broker's existing cancellation request, and result downloads contain the
workflow log and requested artifacts. A submitted job is queued, not completed.
Local history is limited to this launcher session; remote history is broker-owned.

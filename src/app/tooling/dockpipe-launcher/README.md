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

The Flatpak launcher automatically sets up the host terminal command on first
launch. It preserves an existing host CLI; otherwise it creates
`~/.local/bin/dockpipe` and adds it to Bash, Zsh, or Fish PATH configuration when
needed. No opt-in setting or separate command is required. Setup failures remain
visible without blocking launcher startup.

The launcher only hides on close when a system tray is available. It restores a
hidden window if the tray disappears and exits normally when closed without one.
See the [Flatpak guide](../../../../release/packaging/desktop/flatpak/README.md)
for the narrowly handled Wayland teardown diagnostic and diagnostic opt-in.

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

The sidebar provides **Apps**, **Workflows**, **Machines**, **Containers**, and **Executions**.
The workspace selector above every page opens a folder or switches to a recent project.
That folder is passed to Dockpipe as `--workdir`.

- **Apps** lists installed workflows marked `category: app`. Each card offers **Open app** and
  **Configure**; double-click and keyboard activation also launch the selected app.
  **View → Icon grid / Compact list** changes presentation. Empty workspaces link directly to
  the package manager. **Refresh** or **F5** reloads the catalog.
- **Workflows** shows the complete project catalog, a **Run on** machine selector, and run output.
  Right-click a workflow for additional actions.
- **Containers** shows containers, networks, and volumes on a resource canvas and
  refreshes while selected. Local container controls provide Inspect, Start, Stop,
  and Refresh; remote inventory is read-only.

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

The sidebar exposes Apps, Workflows, Machines, Containers, and Executions. Apps continue to use
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

**Executions → Runs & logs** shows launcher-owned local sessions and broker jobs. Remote jobs refresh
while Remote jobs is selected after the first successful refresh. Cancellation
uses the broker's existing cancellation request, and result downloads contain the
workflow log and requested artifacts. A submitted job is queued, not completed.
Local history is limited to this launcher session; remote history is broker-owned.
Use the location selector to switch between this computer and remote jobs. Empty
history has a Browse workflows shortcut. Select a run to enable its available
actions; successful refreshes keep transport output hidden, while failures show
diagnostic details.

## Workflow studio

Application dialogs share rounded surfaces, an in-app draggable heading, clear
primary actions, and slim scrollbars. Confirmation dialogs retain their existing
default and escape actions. Native file pickers keep platform/portal integration.
On Linux and Windows the main window has a single menu/header row with minimize,
maximize/restore, and close controls; drag empty header space to move it and
double-click to maximize. Window edges use the platform's interactive resize.
macOS retains its native traffic lights and global menu.

**Workflows** is the canvas workspace. It starts with a blank workflow each time the
launcher opens, with properties and output collapsed. The **Workflows ▾** picker
(Ctrl/Cmd+P) searches discovered workflows and previews their descriptions, inputs,
and run target. **Run** launches directly without replacing your canvas draft;
**Edit on canvas** opens the source. Recent runs sort first and the last run target
is remembered. An unavailable saved target must be changed before running. The
picker can also open a UTF-8 YAML file. **⋯ → New workflow** starts
another blank canvas. Navigating to another page keeps your current draft; changing
workspaces, opening another workflow, or quitting checks for unsaved changes.

The canvas
supports ordered host/container commands, script commands, packaged workflow calls,
and one level of parallel groups. Select a group before adding a command or workflow
to add a parallel task. Select the canvas background to add another ordered step.

Drag cards to arrange the lined grid, drag its background to pan, scroll to zoom,
and use the lower-left **Fit canvas** control to see the whole workflow. Connections display the existing
YAML execution order; **Move earlier/later** changes that order. Dragging cards never
changes execution. Layout is stored in local launcher preferences with a source
fingerprint, separate from the workflow file.

Select a card to edit its command, working directory, environment, outputs, and
applicable runtime/resolver or package settings. **Apply changes** updates the YAML
preview; unapplied inspector changes block selection changes and saving. Use secret
references rather than embedding credentials. Script selection produces an editable
command using `sh`, `python3`, `node`, or `pwsh -File`; the interpreter must exist on
the execution machine. Review relative script paths before sharing a workflow.

The **Toggle YAML source** icon opens the full YAML source editor. Canvas changes preserve unedited fields and
comments, though YAML formatting may be normalized. Anchors/aliases and single-flow
workflows remain available in YAML; they are not rewritten into an approximate graph.
The lower-left **Undo** control restores previous canvas edits; the YAML editor has normal text
undo. The visual editor supports files up to 1 MiB and 256 visible cards; larger
workflows can still be authored with an external editor and run through Dockpipe.

**Save** validates through the same CLI workflow validator, then atomically writes the
file. External changes cause a conflict instead of an overwrite. **Run** saves and
validates, then uses the launcher's existing session/configuration path. Local runs
keep the canvas open and reveal a floating output panel. **Output** or Ctrl/Cmd+`
toggles that panel; it includes stop, full logs, and input for the running workflow.
This is a workflow console, not a separate system shell. Its preview is bounded;
full logs remain available. **Run on** keeps the existing remote source-preview and
submission flow, with remote results in **Executions → Runs & logs → Remote jobs**.

The lower-left **Properties** control toggles the inspector; selecting a card opens
it. Ctrl/Cmd+S saves. The keyboard icon lists the shortcuts. **+ Add** groups step types
in a menu at the upper right. Workspace and workflow selection float at the upper
left; the editor has no full-width toolbar, tab strip, or footer. Properties and
output overlay the grid without resizing or shifting it. Drag either panel by its
header, resize from its edges or lower-right grip, and click to bring it forward.
Panel placement persists in local launcher preferences; shrinking the app keeps
panels within reach. Use **⋯ → Reset panel layout** to restore their default placement.
Canvas controls use scalable vector icons with tooltips, keyboard focus, and
selected/disabled states. The zoom percentage resets the view to 100% when clicked.
This uses the same YAML and execution
engine and does not automatically install host tools. Live status overlays, arbitrary
dependency wiring, and package-step selectors remain follow-up work. Native
macOS/Windows UI qualification is separate from local Linux/offscreen tests.

The matching CLI must support `workflow document`; older CLIs leave the draft intact
and display upgrade guidance. To exercise the real CLI/editor integration locally:

```bash
DOCKPIPE_STUDIO_TEST_CLI=/absolute/path/to/current/dockpipe \
  ctest --test-dir src/app/tooling/dockpipe-launcher/build -R 'workflow-(studio|workspace)' --output-on-failure
```


### Resource views and activity

**Containers** opens a resource canvas showing containers, networks, and named volumes.
Solid arrows show dependencies declared in Compose labels (scoped to the Compose
project); dashed links show network membership or volume mounts. Sharing a network
does not imply a dependency. Select a resource to open its movable, resizable
**Inspector**. Containers open directly to their last 200 log lines, including stdout
and stderr. Logs refresh every four seconds while Docker is active; **Pause updates**
freezes automatic refresh, and Refresh remains available. Scrolling up keeps your
place. **Details** shows readable metadata; **Raw inspect** retains complete Docker
inspect output, including bind mounts. The inspector contains no inventory tables.
**All resources…** opens the complete inventory separately when needed.

Related network and volume links select and center those cards; network and volume
inspectors link back to attached containers. **Start** and **Stop** run asynchronously.
**Exec** opens `/bin/sh` or `/bin/bash` in a real terminal for the selected running
container, using its ID. The shell must exist in that image. Stopped/paused containers
cannot exec. Linux uses a desktop terminal; Flatpak opens the host Docker CLI through
the host terminal. macOS uses Terminal; Windows requires Windows Terminal. Native
macOS/Windows terminal behavior still needs platform qualification. Search dims unmatched cards. Drag
cards to arrange them, scroll to zoom, and use Fit graph or Reset layout as needed.
Periodic refresh preserves current positions, selection, and zoom.
Floating **Containers**, **Networks**, **Volumes**, **Stopped**, and **Dependencies**
toggles control visible layers and remember your choices. Hiding an object hides its
links too; the full inventory remains under Inspector → All resources…. Fit graph fits only
visible cards. Running containers have green status indicators, unhealthy containers
red, and paused/restarting containers amber. Stopped or not-yet-started containers
use muted cards with square indicators and stay selectable for inspection/actions.
The **Motion** toggle adds a slow travelling highlight to all graph connections,
including network and storage links when no Compose dependencies are declared.
Links touching stopped, not-started, paused, restarting, or unknown-state containers
stay static. Running containers can animate even when unhealthy; health is still
shown explicitly on the card. Highlights fade between 85% and 35% zoom and disappear
below 35%, keeping overview lines steady. Dependency arrows stay green and attachment
links stay blue/dashed. This is a
decorative direction cue, not traffic or execution telemetry. The preference is
remembered; animation pauses on hidden or minimized canvases and when all links
are filtered out. A single 20 fps timer
updates visible links without making additional Docker requests.
Lifecycle labels use Docker's reported state; health text does not turn a stopped
or paused container into a running one.

Relationship discovery inspects up to 256 containers per snapshot. Rendering is
limited to 1,024 objects and 4,096 links; warnings explain incomplete discovery or
rendering, and the inventory remains available under Inspector → All resources…. Docker command
output is bounded to 32 MiB and each command has a 15-second deadline.

**Machines** uses the same canvas for local, broker, and paired-machine relationships.
Cards report pairing/access state; they do not claim a paired computer is online.
Pairing, remote setup, and access controls remain in Details & actions.

**Executions → Runs & logs** provides the global run list. The Workflows canvas
also has a **Runs & logs** button opening a movable panel: local runs are filtered to the current workspace,
while its Remote jobs view explicitly shows all broker jobs. Existing log, stop, cancel,
and result-download actions remain available. Apps keeps its card library.

### Execution Explorer

**Executions → Processes & resources** provides a Managed canvas and a separate
searchable System process view. **Runs & logs** contains local run logs and stop
controls, plus remote job cancellation and result downloads. Select an execution for lifecycle records, duration, optional CPU/memory
metrics and its existing log reference. The shared movable/resizable inspector
keeps the selected resource prominent. Monitoring can be paused; graph, table,
queue and metric history are bounded. Missing measurements remain unavailable.

See [execution observability](../../../../docs/runtime/execution-observability.md)
for the CLI contract, runtime associations, privacy model, configuration and honest
platform/remote support matrix. Native process metrics currently require Linux;
macOS and Windows lifecycle paths still require native qualification.

### System resources and remote monitoring

In **Executions → System**, resource cards show total CPU, memory, swap, root disk
capacity, network activity and uptime/load alongside the process list. The
**System** and **Containers** machine selectors reuse paired machines from **Machines**.
On a paired worker, `dockpipe remote monitoring --system --docker` enables sharing;
`--disable` revokes it. Both worker and broker must be updated. Linux supports
native system metrics; other platforms report unavailable measurements. Remote
Docker currently shares read-only inventory/topology, with local container
controls and raw logs remaining local. See the
[monitoring contract](../../../../docs/runtime/execution-observability.md).

### Navigation and workspace space

The **View** menu lists the same five pages as the left sidebar and tracks the
current selection. Use the sidebar icon at the top-left, **View → Show sidebar**,
or **Ctrl+B** to hide/show the sidebar without leaving the current page. The
visibility preference is saved with the existing launcher settings; the View
menu remains available while the sidebar is hidden.

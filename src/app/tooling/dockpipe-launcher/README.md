# Dockpipe Launcher

Cross-platform **Qt 6** system-tray app: save **contexts** (folder + resolver / strategy / runtime), **launch** or **stop** `dockpipe` subprocesses, **open logs** and folders. It does **not** run workflows inside the GUI; all execution stays in **Dockpipe** (and optionally **DorkPipe** later).

This tree lives under **`src/app/tooling/dockpipe-launcher/`** as first-party Dockpipe tooling. It drives the Dockpipe CLI but is not part of the engine (**`src/lib/`**, **`src/cmd/`**). The tray/window icon comes from **`resources/images/dockpipe-launcher.png`** and Linux desktop installs use the generated **`resources/icons/hicolor/`** size set.

The selected project's VS Code tasks and launch configurations are also discovered
through the CLI catalog. Tasks appear as workflows and launch configurations as
apps, with VS Code source labels and **Clone to workflows** actions. Launch apps
run without a debugger. See [VS Code compatibility](../../../../docs/workflows/vscode-compatibility.md)
for supported configurations, inputs and cloning.

## Requirements

- **CMake** 3.16+
- **Qt 6** (`Widgets`, `Network`, `Concurrent`, `OpenGLWidgets`, `WebEngineWidgets`) — install development packages and the matching WebEngine renderer (see below), or use the [Qt Online Installer](https://www.qt.io/download) and set **`CMAKE_PREFIX_PATH`** to the Qt 6 prefix.
- **OpenGL / EGL development libraries** — Qt 6 Gui pulls in **WrapOpenGL**. On Ubuntu/Pop!_OS, if CMake says `WrapOpenGL could not be found` or `Qt6Gui_FOUND` is FALSE, install **`libgl1-mesa-dev`** and **`libegl1-mesa-dev`** (see Build section below).
- **`dockpipe`** on **`PATH`** (or set **dockpipe binary** in each context’s settings).
- Host tools Dockpipe already needs: **`bash`**, **`docker`**, **`git`** — see [docs/install.md](../../../../docs/install.md).

On macOS, Finder launches append the standard Homebrew binary directories to the inherited `PATH`.
Docker uses the user's saved context or inherited endpoint settings, including a Colima Docker
profile. The launcher does not start or switch container providers. See
[Colima setup](../../../../docs/install.md#containers-with-colima).

## Browser

Open **Browser** in the sidebar or **View → Browser** for ordinary web browsing.
The desktop includes this page without installing or enabling Cockpit or any
spatial package. It provides tabs, address/search entry and compact navigation
controls. **Annotate** opens a page capture for drawing and notes. **Downloads**
shows progress and lets you pause, resume, cancel, reveal saved folders or clear
finished entries without deleting files. Browser settings select a download folder
and whether to ask where each file should be saved.

The **More** menu includes find in page, printing, zoom, viewport-width previews,
screenshots, session history, clear browsing data and browser settings. Viewport
previews change layout width, not the browser's device identity. User-initiated
new-window links open in a new tab.

Browser settings are saved and shared between the top-level Browser and spatial
**Live page** in the existing launcher configuration. Web links opened by DockPipe
can use the system browser or the DockPipe browser, with a separate choice for
localhost and loopback development sites. File and folder links keep their normal
desktop behavior. The address display can show the site alone or the full URL;
editing always reveals the full address. JavaScript and image loading can also
be configured.

For HTTP(S) sites, camera, microphone, location and notifications
have **Ask**, **Allow** or **Block** defaults and exact-origin overrides (scheme,
host and port). Ask prompts appear only for the selected, visible live page;
background spatial content cannot prompt. Remembering a prompt decision is
optional. Unsupported permissions are denied. These settings do not grant AI
access or provide AI controls.

**Extensions** installs Manifest V3 extensions on Qt WebEngine 6.10+ builds with
extension support enabled. Visit a Chrome Web Store listing and choose Dockpipe's
**Add extension** button, or open Extensions and paste its Web Store URL/ID.
**Install package** accepts a ZIP or CRX3 file. Packages are downloaded directly
from Google's extension servers over HTTPS; Dockpipe preserves CRX public-key
identity when importing them. Local package imports do not verify CRX signatures.
The disabled extension's requested API, website and content-script permissions
are shown before **Install and enable**. Cancellation removes the pending install.
Installed extension code and explicit enable/disable choices are saved separately
from browsing data and restored in new browser sessions. **Uninstall** removes
that saved installation. **Load unpacked** remains a temporary developer action.
Extension installation is limited to 32 MiB packages, 64 MiB expanded content and
4096 entries; archives with links, traversal or encrypted entries are rejected.
Compatibility depends on the extension APIs provided by Qt WebEngine; Chrome-only
services, Manifest V2 and password-manager compatibility are not guaranteed.
Dockpipe has no built-in password vault.

**Known compatibility limit:** the 1Password Chrome extension downloads and installs
with its original ID, but its background worker fails on the current Qt 6.10.3
runtime because `chrome.notifications` is unavailable; its popup remains blank.
Installation success is not functional 1Password qualification. Current 1Password
also requires Chrome 140+, while Qt 6.10.3 uses Chromium 134. Desktop-app integration
adds native-messaging and explicit browser-trust requirements, and 1Password does
not support desktop-app communication from Linux Flatpak browsers. Account login,
autofill and desktop unlocking have not been qualified. See
[1Password requirements](https://support.1password.com/system-requirements/),
[additional browsers](https://support.1password.com/additional-browsers/) and
[desktop connection](https://support.1password.com/connect-1password-browser-app/).
The optional `browser-extension-compatibility-test`, with
`DOCKPIPE_EXTENSION_COMPAT_ID` set to an explicit extension ID, reproduces download,
installation and popup startup in temporary storage without accessing a vault.

The named browser profile uses isolated temporary storage removed on normal
shutdown. Cookies, website data, unpacked developer extensions and history are session-only; saved
downloads and captures remain on disk. **Clear browsing data** closes tabs,
cancels downloads and resets the session, reloading saved extension choices. Saved browser settings and remembered
site policies remain separate from session cookies and history.

The Browser is created on first selection and retains its tabs when you return
to another desktop section. It uses the shared browser runtime, while spatial
extraction and world presentation remain in the optional package. The headless
Dockpipe CLI has no Qt or browser dependency.

Ordinary browser settings belong to DockPipe. Optional DorkPipe assistance for
page summaries, annotated requests and approved browser actions is
[deferred TASK-008 scope](../../../../docs/agents/tasks/agentic-app-ui/browser-integration.md).
It is not implemented by installing the Browser or enabling browser extensions;
browsing remains available without the DorkPipe package.

## Terminal

Open **Terminal** below Browser in the sidebar, or add a **Terminal** widget to a
dashboard. Both use the same session service, with at most eight open terminal
sessions across the desktop. The terminal display uses locally bundled xterm.js
6.0 through the desktop's existing Qt WebEngine dependency; it does not fetch a
terminal frontend from a CDN. Renderer delivery waits for acknowledgement of each
32 KiB chunk. Each session retains its latest 256 KiB for replay; older unread
output can be discarded if a view falls behind, with a visible notice. Each display
keeps 2,000 scrollback lines by default (configurable from 100 to 10,000). Hidden
views stop rendering.

Use the **Terminal settings** sliders button on the Terminal page, or right-click
inside a dashboard terminal and choose **Terminal settings**, for shared themes
(DockPipe, Amber, Green, Paper, Midnight), monospace font and size, cursor style,
cursor blinking, scrollback, opacity, and optional static scanlines/text glow.
Save applies to open terminal pages and widgets without restarting their shells.
Preferences use the existing launcher settings store. Transparency is **off by
default** (100% opacity), and theme changes never enable it. Lower opacity reveals
the selected **Transparency reveals** target: the DockPipe canvas and panels
underneath a floating terminal (default), the panel background, or the desktop
behind DockPipe. Transparency selects Qt software rendering for terminal and
browser views at startup so foreground panels cover them correctly. First enabling
transparency requires a restart; until then terminal views remain opaque with an
explanation. Set opacity to 100% and restart to return to accelerated rendering.
The software backend disables WebGL in Qt WebEngine. Websites that require WebGL,
including the DockPipe site's Qt WebAssembly interface, can appear blank or fail
to initialize even when their HTTP requests succeed. Use 100% terminal opacity
and restart for those sites.
An explicit QT_QUICK_BACKEND override is respected. The native Linux fixture checks
focused opacity and overlap pixels; the local browser switching and idle checks
passed with the fallback. This does not qualify video or 3D-heavy browsing.
Canvas mode removes the terminal panel's solid fill; tiled groups retain their
own background. Desktop mode clears only the terminal viewport and keeps text and
app controls opaque. Enabling Desktop requires restarting DockPipe to allocate a
composited window surface; no screen capture or wallpaper copy is used. It requires
a compositing desktop and has been qualified on Linux/X11. Wayland and macOS still
need native qualification; foreign embedded application surfaces may remain opaque.
Text glow has **strength** (0–100%) and **spread** (0–8 px) sliders. Effects are off
by default; there is no animated CRT simulation or background effect loop.

The Unix backend connects a real shell to a pseudoterminal, including terminal
input and resizing. Windows uses the operating system's ConPTY support, available
on Windows 10 version 1809 and later; no separate ConPTY download is needed.
Native creation, pipe I/O, resizing and teardown run outside the GUI thread.
Closing a Windows session terminates its contained process tree. Native macOS
and Windows qualification is pending. The native Linux fixture verifies real
keyboard input, rendering, selected-session delivery, layout retention, bounded
output and cleanup. Native macOS and Windows qualification remain outstanding.

Choose an installed shell beside **New terminal**. Windows discovery includes
Windows PowerShell, side-by-side PowerShell installations, Git Bash and Bash in
installed WSL distributions. WSL sessions receive the selected project directory;
discovery lists distributions without starting them or installing Linux. macOS
and Linux list the login shell, `/etc/shells` entries and common installed shells,
including Homebrew locations on macOS. Refresh the list after installing a shell.
New sessions preserve existing sessions. In dashboard terminals, **Shells and
sessions** in the context menu provides the same choices and switches between
sessions belonging to that panel. Removing the panel closes its owned sessions.

Flatpak terminals discover and run host shells through the explicit host bridge.
The host terminal retains job control, resizing and session cleanup; missing host
access is an error, without a fallback to a sandbox shell. Actual Flatpak PTY,
input, resize, interrupt and foreground/background cleanup tests pass on the KDE
6.10 runtime. This does not qualify Windows or macOS native behavior.

The dashboard **Send input** widget selects one terminal session and sends the
entered text followed by Enter. This executes input in that selected shell; it
does not broadcast to other terminals. Check the selected session before sending.
Terminal sessions are live desktop state rather than saved shell processes.
On the Terminal page, choose **New terminal** to start a shell. A dashboard terminal
starts its own shell when first shown, without an extra toolbar or close button.
Hidden widgets that have never been shown do not start shells. Switching dashboard
environments preserves the shell; removing its panel stops only that widget's
owned session. Right-click a stopped widget to start a replacement session.
The Terminal page's **Close** button stops its selected session for every view.
Application exit stops the desktop's shells. New sessions use the current project directory;
changing projects does not move a running shell.

Future package-contributed destinations, including DorkPipe, belong to the
[TASK-008 integration backlog](../../../../docs/agents/tasks/agentic-app-ui/browser-integration.md#future-send-input-targets).
There are no AI targets or AI controls in this terminal implementation.

## Custom dashboards

Everyone has a built-in **Dashboard** above Apps in the sidebar. Its name is fixed
and it cannot be removed. Turn off **Settings → Show Dashboard above Apps in the
sidebar** to hide it without losing its widgets or layout. It is loaded only when
opened.

Choose **+ Dashboard** in the sidebar or **View → New dashboard…** to create up to
12 additional dashboards, then use **+ Add widget**. Custom dashboards appear below
Terminal and above Cockpit.
The cog beside Full screen opens dashboard settings, including Rename and Remove.

Widgets are compact views of system resources, workflows, executions, session
logs, containers, files, Git status and machines. **Open full view** takes you to
the corresponding sidebar page. Workflow launches use the existing configuration
and execution flow. Read-only status widgets share their data sources and pause
polling while hidden. The Browser widget uses the ordinary browser component,
including its settings, permissions, downloads and annotations.

**Canvas** and **Tiling** are independent environments with separate panels and
arrangements. Canvas lets you move, resize and overlap DockPipe panels; it never
captures desktop applications. Tiling arranges its own panels into draggable splits
and tab groups. Drag a tab onto another panel: the edges split, and the center or
tab strip creates a tab group. Right-click a tab or panel edge for arrangement,
removal and full-view actions. Floating panels can overlap tiles.
For a nested desktop app, hover its tile to reveal the small **⋯** handle:
drag that handle to split or tab, or click it for tile actions. The app's own
title bar does not move its DockPipe tile. Dragging a selected tab to an edge of
its own group splits it out; Escape cancels a drag.
Native app tabs show navigation labels without duplicating the app's window
buttons; DockPipe widgets retain their own panel controls.

Dashboard panel controls use DockPipe's own circular coral close and teal
expand/restore artwork, with visible symbols and keyboard focus indicators. The
outer application and dialogs retain their regular window controls.

The dashboard **File manager** defaults to the selected project in Canvas and the
full filesystem, starting in Home, in Tiling. Each environment keeps its own open
folder. Changing the selected project updates project-scoped views; full-system
views keep their current location. The **Project · Change** button on Canvas uses
the normal project-change flow, including unsaved-editor handling. Dashboard
settings let you independently select **Selected project** or **Full system —
start in Home** for each environment. Existing dashboards inherit these defaults
when no explicit scope preference was saved.

File browsing is lazy and read-only. Full-system views remain subject to normal OS
permissions and open files in their default applications. Project-scoped views
keep files inside the selected root and open them in DockPipe's project editor.
**Open folder** opens the current directory in the system file manager.

Each panel has close, minimize-to-dock, and expand controls on the left. Minimized
panels keep their live sessions and return when selected in the dock.

Tiling is compact by default: full-width title/tab strips, no gaps or borders, and no extra widget
footers. Dashboard settings can increase spacing, show or hide canvas/floating
title bars, and change the shared Canvas/Tiling background. The default matches the other canvas
pages' grid; Plain, Dusk, Ocean, a local image and Desktop background are also available. Images are
limited to 20 MiB and 16 megapixels and cached for painting, without a background
refresh loop. Settings and both environments survive a restart; live widget
instances stay open while switching environments.

New dashboards offer a three-step tour covering the independent workspaces,
appearance and optional Tiling-only app takeover. **Finish** applies choices;
**Dismiss**, Escape or closing the tour leaves the dashboard unchanged. Completing
the tour or checking **Do not show this tour again** stops automatic tours on future
dashboard creation. Dismissing without that checkbox offers it on the next new
dashboard, not every app launch. **Replay dashboard tour** in dashboard settings
always remains available. Tour preferences share the existing `launcher.json` store.

**Desktop background** inherits a local static wallpaper image and refreshes when
either workspace is shown again. Lookup runs off the UI thread, without continuous polling;
unchanged images reuse the paint cache. GNOME/Pop/Cinnamon settings and static
Plasma configuration are supported on Linux; Windows uses its wallpaper API and
macOS uses NSWorkspace's main-screen wallpaper. Video/dynamic backgrounds,
unsupported formats and ambiguous per-monitor/activity settings report a reason
and retain the current dashboard background (the grid after a fresh start). Selecting
an image manually remains available. This never captures the desktop or changes
system wallpaper. Native Windows/macOS wallpaper behavior still requires platform
qualification.

The dock's app-grid button opens **Apps and workflows**. Canvas lists project app
workflows; Tiling also lists installed desktop applications. Opening an app uses
the platform launcher or DockPipe's existing workflow launch flow. Installed apps
reserve a Tiling panel before launch. On X11, apps with a declared window class
(including Brave) are attached automatically when one matching new window appears.
A single existing matching window is brought straight into Tiling without launching
another copy. Multiple
matches require explicit selection; unrelated windows are never selected by title.
Discovery stops after 15 seconds or when you close the panel. Switching to Canvas
keeps the pending launch; native attachment resumes when Tiling is shown. A
short desktop appearance is still possible because discovery follows window mapping.
Apps without reliable window identity and other platforms use the existing window
picker. Wayland/macOS retain system consent and view-only sharing. Local app
workflows still offer manual window selection; remote workflows do not create a
local window panel. Native application icons are used
when available, with a generic icon fallback. Running panel shortcuts activate
only panels in the current environment.

**Bring open desktop apps into Tiling when I enter it** is off by default. On
supported X11/Windows desktops it acquires eligible windows, within the dashboard
panel limits (16 across Canvas and Tiling). Managed
minimized windows are eligible too; releasing them restores their minimized state.
The initial arrangement splits the first four panels and places later panels in
tabs; those apps remain acquired and can be selected from the tab strip or dock.
Each acquisition revalidates its selected window without rescanning every other
application's process. These panels remain attached for the DockPipe session, including
while Canvas or another page is shown. Closing a panel in this managed mode
requests that its OS window close, respecting the app's save prompts and cancellation.
Closing DockPipe, including closing to the tray, returns apps to the desktop with
their original window controls and size. Reopening Tiling can acquire them again.
Quitting also releases apps before native host windows are destroyed; disabling automatic takeover releases
automatically acquired panels. Cancelling selection or closing an app removes its tile and lets
the remaining panels fill the space. Native window identities are never saved
across restarts.
Saved layouts discard old empty desktop-window placeholders. Wayland and
macOS require their system consent pickers and do not support automatic takeover.
None of these options changes the desktop's window manager.

The dock auto-hides and overlays the workspace by default. Move to the bottom edge
or press **Ctrl+Shift+D** to reveal it. Turn off **Automatically hide the dock**
in dashboard settings to keep it visible below the workspace.

Use **Full screen** or **F11** for the compact dashboard top bar, clock and dock.
The **DockPipe** button opens a navigation menu without resizing the workspace. Leaving fullscreen
restores the previous window state and sidebar preference. The dock has no
continuous animation; the clock updates once a minute only while visible.

Names, widget identities and layouts are saved atomically in `dashboards.json`
alongside the existing launcher configuration. Browser sessions, native window
identities and sharing grants are not saved. Reopening the app restores the
arrangement; select external windows again. Existing shared-mode dashboards migrate
ordinary widgets into their last selected environment, retaining widget identities.
External windows are selected afresh in Tiling. Corrupt or conflicting settings are reported
instead of overwritten. Limits are 12 dashboards, 16 widgets per dashboard across
both environments, and 64 widgets overall, including temporary automatic panels.

**Desktop window** is experimental. The window picker opens when adding a panel.
There is no inline Choose or Release toolbar; right-click an empty panel to choose
or reconnect a window. With automatic app management off, closing the panel returns
the app to the desktop. With management enabled, it requests a normal OS window close;
it never forcibly terminates the application. Linux X11 uses the same native window support as Cockpit,
now shared by the desktop UI library. External native surfaces can have
platform-specific stacking behavior when panels overlap. Windows uses a Win32
foreign-window backend and only accepts compatible windows from the same user
and session; elevated, unresponsive and incompatible-DPI windows are excluded.
The Windows backend requires Windows build and native testing before qualification.

On X11, clients that omit `_NET_WM_PID` (including RustDesk) use the X-Resource
1.2 extension to identify their local owning process. Build with `libxres-dev`
for this fallback; without it or a server-provided owner, those windows remain
excluded. An unmapped X11 window is eligible only when its valid ICCCM state
identifies it as minimized; withdrawn clients and hidden helper windows stay excluded.
Embedded X11 clients advertising the extended frame synchronization counter
receive host frame acknowledgements while visible. GTK clients such as RustDesk
otherwise stop repainting after leaving the desktop window manager. Acknowledgements
stop while hidden and on release; legacy resize counters are left untouched.

Single external-app tiles omit Dockpipe's duplicate title/control strip. Hover at
the tile's top centre for its actions menu: expand/restore, move, minimize, or
release back to the desktop without closing the app. Right-click the menu button,
or use Ctrl+Alt+M when the dashboard has keyboard focus. Groups with multiple tabs
retain their navigation strip. Native X11 tiles clip advertised client-side shadow
margins so app content fills the tile; the hidden dock uses a nonpainting bottom
hotspot and does not cover the app with an input strip.

Wayland uses the desktop's consent dialog to select one **window**, with a
view-only preview. It does not embed or control native Wayland apps. Leaving Tiling,
cancel or widget removal closes the sharing session; no reusable permission
token is saved. The optional build needs Qt DBus; the running desktop needs its
ScreenCast portal, PipeWire, `gst-launch-1.0`, and the GStreamer PipeWire,
video-conversion/scaling/rate, JPEG and file-descriptor sink plugins. Missing
dependencies or denied sharing are shown in the widget. Preview output is capped
at 1280 × 720 and 15 frames per second. Bazzite requires a native desktop test;
Linux X11 or mock-portal tests do not establish that qualification.

macOS 14 and later uses **ScreenCaptureKit** and Apple's system content-sharing
picker for a single-window, view-only preview. Grant the requested window through
that picker; this implementation does not request Accessibility access or inject
input. Use the original app for interaction. One macOS window share is supported
at a time. Frames are capped at 1280 × 720 and 15 fps. Older OS/SDK builds report
that sharing is unavailable. The macOS backend still needs a native build and
tests of grant, cancel, revocation, layout changes and release on a Mac.

Hidden panels preserve their sharing grant. After a short debounce, Wayland's
owned capture helper suspends and resumes when shown; macOS capture throttles to
1 fps without copying hidden frames. Native window monitoring also pauses while
hidden, including inactive tabs, Canvas, and other pages. Switching away keeps
apps attached and out of the desktop; returning shows the same Tiling panels.
Removing a panel, deleting its dashboard, disabling automatic acquisition for an
automatically acquired app, or quitting DockPipe releases it. Hidden apps do not
retain keyboard focus over Canvas. Removal stops capture asynchronously without
waiting on the UI thread. Wayland/macOS sharing remains view-only and does not
hide the original desktop window.

Performance regressions cover 1, 8 and 16 retained widgets, shared polling,
unchanged large log tails, process cancellation, and a real local browser page
with seven additional compact widgets. `dashboard-performance-test` checks
layout response; `dashboard-page-test` checks independent environments and bounded
switching with retained panels; `dashboard-browser-performance-test` reports native event-loop
response, parent-process idle CPU/RSS, retained renderer/DOM/input, and bounded
object/process growth. Their gates are p95 below 100 ms, maximum below 500 ms,
and less than 32 MiB of warmed parent RSS growth across 50 browser mode-switch
pairs. These are UI-response and retention checks, not GPU frame-rate or
cross-platform performance certification.

For Windows qualification, build `desktop-app-window-win-test` and run
`ctest -R desktop-app-window-win --output-on-failure` from the configured build
directory in an interactive desktop session. The test creates its own disposable
window and checks embedding, mode changes and restoration. For Linux X11, build
`desktop-window-widget-test` and `desktop-app-window-test`, then run
`desktop-window-widget-test --native` in the graphical session. Native fixtures
must never select an unrelated user's window. The Wayland mock-portal tests
exercise cancellation and session cleanup without opening a real sharing dialog.

Optional DorkPipe chat input and output widgets remain future package
contributions, with separate arrangeable panels. Dashboard use does not require
DorkPipe or enable AI access to browser pages or external windows.

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

Single-configuration builds default to optimized `Release` when no build type is
selected. Use `-DCMAKE_BUILD_TYPE=Debug` explicitly when debugging; unoptimized
builds are not representative of interactive rendering performance.

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
sudo apt install cmake build-essential qt6-base-dev libqt6opengl6-dev \
  qt6-webengine-dev qt6-webengine-dev-tools libqt6webenginecore6-bin
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

For a Linux SDK installed with **aqtinstall**, run the installer from a generated
artifact directory. Its `--outputdir` controls the SDK destination, but its
`aqtinstall.log` is written to the working directory. From the repository root,
using a Python environment with aqtinstall installed:

```bash
qt_install_artifacts="$(./src/bin/dockpipe scope artifacts desktop/qt-install)"
qt_sdk_root="$(pwd)/src/app/tooling/dockpipe-launcher/build/qt-sdk"
mkdir -p "$qt_install_artifacts"
(
  cd "$qt_install_artifacts" || exit 1
  python3 -m aqt install-qt linux desktop 6.10.3 linux_gcc_64 \
    --outputdir "$qt_sdk_root" --modules qtwebengine qtwebchannel qtpositioning
)
```

The log stays under the resolved `bin/.dockpipe/` artifact scope, which Git
already ignores. The subshell preserves the caller's working directory.

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

The sidebar provides **Apps**, **Workflows**, **Machines**, **Containers**, **Executions**, **Files**, and **Git**.
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

The sidebar exposes Apps, Workflows, Machines, Containers, Executions, Files, and Git. Apps continue to use
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
workflows can still run through Dockpipe. The launcher never opens a default external text editor.

**Files** inside the workflow editor browses only the folder containing that workflow's
YAML. Save a new workflow first to establish this folder. The active workflow YAML
shares its document with the canvas/YAML view and uses the same validation/save path.
Scripts and supporting text files can be created and edited here.

The separate sidebar **Files** page works across the selected project directory.
Both views use the same native editor: custom vector explorer icons, a file header,
on-demand find (Ctrl/Cmd+F, Enter/Shift+Enter to navigate), line numbers, current-line
emphasis, indentation, undo, zoom (Ctrl+plus/minus/0) and save (Ctrl/Cmd+S).
The status bar shows language, encoding, line endings and cursor position.
Rename, reload and delete live in the file actions menu.

Syntax highlighting uses the offline KSyntaxHighlighting engine with bundled
language definitions, including C, C++, C#, Java, Go, Rust, Python, JavaScript/Node.js,
Ruby, PipeLang, R, shell, Kotlin and PHP, plus YAML, JSON, TypeScript, Markdown and
other common formats. Multiline comments/strings use grammar state. Unknown file
types remain plain text. Highlighting is not a compiler, autocomplete or a language
server. Lines over 16,384 characters stay editable with highlighting skipped to
keep generated/minified lines responsive. The compressed grammar data is about
1.5 MiB and ships separately with its license notices; no web runtime or downloads
are needed. See `third_party/ksyntaxhighlighting/NOTICE.md` for provenance.
 New file, new folder, rename and delete
operate inside that view's root. File editing supports UTF-8 up to 1 MiB and
preserves UTF-8 BOM and CRLF line endings. Binary files, linked paths outside the
root and Git metadata are not editable. Unsaved changes prompt before switching
files, projects or closing. External disk changes block overwrites; keep a copy of
the draft before choosing Reload. Rename the active workflow from project Files,
then reopen it in Workflows.

The **Git** page shows a changed-file list with selection checkboxes and status
badges, one selected file's highlighted review, and a separate commit panel.
Reviewing a file is a read-only, snapshot-checked CLI action; arbitrary filenames
are passed literally. Untracked text is shown as source; tracked files use diff
highlighting. A staged-only difference is labelled when the working file matches
HEAD. Committing still uses saved working files.

The sidebar **Git** page belongs to the project repository, never an individual
workflow. Refresh reviews saved changes, select files, enter a commit message and
choose Commit selected. Unselected staged changes remain staged. Pull requires a
clean checkout and fast-forwards only; Push to origin pushes the current branch
without force. Git uses existing identity/credentials and must be available to the
CLI (including inside Flatpak). Save editor buffers before reviewing Git. A stale
review is rejected; refresh after failures or cancellation to reconcile state.
The Changes tab can compare combined, staged or unstaged changes, stage or unstage
the selected file, discard its unstaged changes, and commit the existing index.
The History tab searches commit subjects and bodies by keywords, or matches a SHA
or subject prefix, across all refs or a chosen branch. Results are paged in groups
of 50. Inspect a commit, select several, and add them to the cherry-pick queue.
Move, reverse or remove entries before confirming the exact order and target
branch; a queue accepts up to 128 distinct commits. A single commit can also be
reverted. Merge commits require a mainline choice and are rejected by this surface.

Branches / stashes provides branch creation, switching, safe deletion, merging,
fetching all remotes without pruning, and stash save/apply/pop/drop. Mutations use
the reviewed repository snapshot and reject stale state. Branch switching, merging,
cherry-picking and reverting require a clean working tree; there is no automatic
stash or forced overwrite.

Git's sequencer preserves an interrupted cherry-pick queue. The Conflicts tab shows
the current commit and remaining work, opens conflicted text in the built-in Files
editor, and guides saving and staging a manual resolution before continuing.
Explicit use-ours, use-theirs and deletion actions cover other resolutions. Skip
and abort require confirmation because they discard current conflict resolutions.
Stash apply/pop conflicts use the same editor and resolution controls, retain the
stash, and leave the resolved result staged for a normal commit.
Unsaved conflict drafts are guarded when leaving the project or quitting. Refresh
after a failed or cancelled operation before explicitly retrying; cancellation does
not undo Git changes.

Git actions use the generic runtime-owned `dockpipe git --request` adapter. They
have a two-minute deadline, cancellation, an 8 MiB per-command output budget, a
256 MiB streamed content-hashing budget and a 2048 changed-file limit. They do not grant managed agents arbitrary Git authority.
For a real CLI/UI integration test in a disposable repository, set
`DOCKPIPE_GIT_TEST_CLI` to the newly built CLI and run `ctest -R project-git` in the
launcher build directory. Without that variable the Git integration test is skipped.

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

The bottom status bar appears only for an active message or the undismissed
notice. Idle pages use the full window height; clearing or expiring a message
releases that space on every page.

### Shared file explorer

Files, workflow Files and the dashboard's File manager use the same custom
explorer, including vector file-type icons and the existing folder artwork.
Use **Tree** for expandable rows or **Folders** for an icon grid with centred
icons and wrapped filenames. The selected item stays selected when switching
views. Dashboard Canvas and Tiling retain their own view and directory while
switching layouts; this view choice lasts for the lifetime of the panel.

The dashboard's compact navigation bar offers Up, Home/project root and an
editable location. System scope also exposes filesystem roots/drives; Project
scope continues to reject paths and links outside its project. Double-click or
press Enter to open a folder or file in the grid. Files-page folder browsing
retains the current draft; opening a different file still asks about unsaved
changes. View changes do not read file contents or generate previews/thumbnails.

### Markdown preview

Open a `.md`, `.markdown`, `.mdown` or `.mkd` file in **Files**, then click the
**Preview Markdown** eye beside Save and Search. The resizable preview appears
beside the source and updates from unsaved edits. Click the eye again to hide it;
Save and draft protection continue to use the existing editor.

The first-party Qt renderer supports headings, emphasis, lists, tables and fenced
code blocks on Windows, macOS and Linux, including Flatpak, without Cockpit.
Plain Markdown uses Qt's text renderer. Preview uses the editor's 1 MiB UTF-8
limit. Raw HTML is disabled and embedded Markdown images are not loaded.
Clicking a relative file link reuses Files' root and unsaved-change checks;
HTTP(S) links open in the system browser only when clicked. Other URL schemes
are ignored.

Fenced `mermaid` blocks render as diagrams beside the source. The desktop bundles
Mermaid 12.2.0 and uses an isolated Qt WebEngine renderer offline; no CDN, Node.js,
external command or Cockpit installation is needed. Flowcharts and sequence
diagrams use the selected light/dark theme. Diagram links are inert. Invalid
diagrams show an error with their source, and editing or hiding the preview
cancels pending work.

Live preview renders up to 16 diagrams within a 20-second document deadline and
32 MiB decoded-image budget. Remaining diagrams stay visible as source. Each
diagram allows 50,000 characters, 500 edges and ten seconds to render; output is
limited to four million pixels. Standalone UI hosts without the desktop renderer
show an explicit unavailable message and retain the diagram source. Native
Windows/macOS checks are configured in CI; current local proof is Linux.

### Workspace graph

Open **Files → Graph** to explore the selected folder (or the project root). The
same control is available in a workflow's scoped Files view. Graph opens only on
request and loads the installed Cockpit package on the first click; visiting
Cockpit first is unnecessary. The control requires Cockpit mode in Settings and
a matching native package. Loading failures appear in Files, and leaving the
page or changing roots cancels a pending open. The currently published Cockpit
renderer and ForgePipe SDK support Linux amd64; Windows/macOS graph packaging
remains unavailable until matching native packages are released.

The graph shows the root and immediate children, then expands directories when
you double-click them or choose **Expand**. Double-click a regular file to open it
in the existing editor; unsaved drafts retain the usual save/discard protection.
**Editor** returns to the file browser. Leaving Files closes the graph.

Drag the background to pan, scroll to zoom, drag nodes to arrange them, and use
**Focus** or **Fit** to navigate. Right-click a folder for **Explore this folder**,
**Refresh branch**, or **Show in explorer**. **Root** returns to the workspace root;
**Options → Reset layout** restores the deterministic tree positions. Files keep
the existing editor's CRUD commands rather than adding separate graph mutations.

Switch **2D / 3D** in the graph header for a circuit-board city. Files become
chips whose height reflects file size, folders become teal blocks, and symbolic links/unavailable
entries use amber. Connections follow circuit traces, while labels stay upright.
At city-wide zoom, silhouettes replace tiny labels and pins; zoom in for details.
Folder depth sets the district height. **Middle-mouse drag**, **Shift + drag**, or **Rotate** orbits the
camera. Background drag pans, scrolling zooms, and **Reset camera** restores the
initial angle. **Focus** makes the selected card readable; **Fit** shows the loaded
set. Cards can occlude each other from some angles; orbit or return to 2D to
separate them. Switching modes preserves loaded branches and manual 2D positions.
The 3D view uses native orthographic projection with a fixed-size floor grid,
without a continuous frame loop. Camera movement
projects only the already-visible nodes and never reads the filesystem.

The selected chip's label floats above all geometry at a readable screen size,
including when zoomed out. Selecting a different chip restores normal depth order
for the previous label; chip bodies retain their spatial order.

3D automatically tries a hardware OpenGL viewport when opened. The **GPU** or
**Software** indicator describes the active renderer in its tooltip. Disable
**Options → Hardware acceleration (3D)** for raster rendering, or launch with
`DOCKPIPE_GRAPH_SOFTWARE=1` to default to software. Offscreen displays, failed
contexts and detected software OpenGL drivers use the raster fallback. Returning
to 2D or closing the graph releases its OpenGL viewport. These checks cannot recover
from an operating-system graphics-driver crash. Hardware acceleration affects
painting; file reads and graph projection retain their existing bounds.

In normal **3D**, clicking a node sends a small drone to hover above it. Opening a
file lands the drone before opening the built-in editor; double-clicking a folder
opens its hangar and moves into that directory. **Parent** returns one level and
**Root** returns to the workspace. The explicit **Expand/Collapse** controls still
manage branches in place. **Options → Animate drone navigation** disables the
short transitions. Selection settles after 360 ms; entry/landing takes 650 ms.
Changing selection, switching modes or hiding the graph cancels pending navigation.
There is no idle hover animation or rotor loop.

After enabling **Settings → Cockpit mode**, **Options → Drone flight · Cockpit
mode** enters an optional first-person city.
Files become size-based, stepped and beveled towers with facade lights and roof
machinery. Teal folder hangars have signs, runway lights and proximity shutters.
Approach the front opening at simulation altitude 30–210 and fly through to load
that folder's immediate children as a new world. **Backspace / Parent** returns
one level; **N / Next page** replaces the current district with the next page.
**R** retries a failed read or refreshes the current directory. Empty folders and
read errors are shown in the cockpit. Failed reads preserve the previous world.
Links are scenery only, never portals or editable targets.

Double-click a folder hangar to fly inside automatically. Autopilot rises above
the current directory, crosses to the doorway and descends before entering.
**Space** arrives immediately; **Escape** cancels travel. Losing focus or hiding
flight also cancels folder travel. A missing or unreadable folder leaves the
current directory visible with an error and a retry option.

Aim the crosshair at a file and **left-click / Space** to fire a targeting pulse.
The first structure under the crosshair blocks the shot; folders explain how to
enter, and regular files open in a translucent **FILE LINK** cockpit overlay.
This mounts the same editor pane used by Files, with syntax highlighting, find,
undo, **Ctrl+S**, workflow document sharing and the existing save/conflict checks.
It creates no second editor document or filesystem model. Flight pauses while
editing. **Escape / Return to flight** dismisses the HUD and preserves unsaved
text and undo history in the normal editor. Exiting flight or closing the graph
also restores that same pane. Opening another file still asks about dirty edits.
Binary/non-UTF-8 files, files above 1 MiB, linked files and paths outside the root
remain rejected by the existing editor. Firing never saves or executes a file.

**WASD** moves, **Q/E** descends/ascends, **right-drag** or **arrow keys** looks
around, and **Shift** boosts. Buildings and altitude limits constrain movement.
**Escape / Exit flight** cancels loading and returns to the preserved graph.
The 16 ms movement tick runs only while controls are held and stops on release, focus
loss, hiding or exiting. Doors respond to movement, with no idle animation loop.
Graphics use the same hardware/software choice as the graph. Geometry is
procedural, detailed only within 1,800 simulation units, and clipped to the
viewport before drawing to avoid Qt coordinate overflow.

Flight retains one page of at most 150 structures, replacing it on navigation or
pagination. It uses the existing cancellable directory helper, with no recursive
scan, prefetch, retained ancestor worlds, texture downloads or game engine. The
normal graph remains bounded separately to 500 nodes and is restored on exit.

File height uses a logarithmic scale, capped to keep large files navigable. Empty
files still have visible thickness. The label shows the reported file size and
the tooltip includes the exact byte count. Size comes from cached file metadata
for the current page, without reading contents or summing folder descendants.
Folder heights remain fixed. Use **Refresh branch** after files change to update
the size snapshot; camera motion and view toggles do not poll the filesystem.

Each expansion loads at most 150 entries. **Load more** appears when entries remain.
The default visible limit is 500 nodes, including the root. **Options → Node limits**
can lower either limit for the current view. At the limit, collapse branches or
explore a smaller folder. Collapsing removes descendants and their metadata;
there is no hidden-node cache. Paths have a separate 4 MiB metadata budget and
helper responses are capped at 2 MiB. Symbolic links, including broken links, are
shown but never expanded or opened through the graph. Git metadata is excluded.
**Show dotfiles** is on by default, matching Files. Like the current file browser,
the graph does not interpret `.gitignore` or run Git to filter its inventory.

Directory reads run in a transient instance of the native launcher, using its
read-only `--workspace-graph-page` mode before GUI initialization. There is at most
one helper across graph views, with one replaceable navigation request. Closing,
cancelling, or navigating away kills the old read; no GUI thread waits on filesystem
I/O. The helper streams one directory and retains only the next sorted page, never
a recursive tree. Deterministic name ordering requires rescanning that immediate
directory for each additional page; memory stays bounded even for huge directories.
Directory timestamps detect changes between pages and prompt a branch refresh.
The helper has a 20-second enumeration budget and a 25-second parent deadline.
An OS filesystem call may take time to respond to termination; another helper is
not started while it is still stopping.

The graph reuses the existing native canvas pan/zoom/grid surface and vector icons.
Scene updates add only new nodes/edges, keep existing positions, and use Qt's scene
index and viewport culling. There is no continuous layout, animation, indexing,
filesystem watcher, dependency parser or Git scan owned by this view. Imports,
symbol references and other relationship overlays remain future work.

`workspace-graph-test` covers lazy expansion, pagination, node limits, stale pages,
links/cycles, denied/deleted directories, deep/Unicode paths, cancellation, timeout,
rapid navigation, editor routing and repeated cleanup. It reports fixture latency,
layout time, 500-node camera projection/render time, idle CPU and Linux memory samples.
It also checks 2D position restoration and mouse orbit without new reads.
Set `DOCKPIPE_GRAPH_SCREENSHOT` to
capture the real widgets; `DOCKPIPE_TEST_THEME=light` selects the light fixture.
`DOCKPIPE_GRAPH_GPU_SMOKE=1` runs the explicit native-display GPU fixture, including
viewport readback, software switching and flight entry/exit. It is separate from
offscreen tests and requires a working hardware OpenGL desktop.

### Workflow files and experimental app billboards

The built-in editor, including its flight HUD, offers **Open as workflow** for
`.yml` and `.yaml` files. This opens Workflow Studio for review; its existing
**Run** action validates and saves the workflow before using the normal launcher
session and input controls. `workflows/<name>/config.yml` is the usual workflow
entry point; other workflow YAML paths also work. Opening or shooting a file does
not execute it. Shell and other source scripts remain editor-only in this view.

Enable **Settings → Cockpit mode → App control · Experimental** first.
Before entering drone flight, enable **Options → App billboards · Experimental
(next flight)**, then choose up to three open application windows. The option is
off by default and window selections are never saved. Selected windows appear as
live billboards near the entry plaza in each directory world. Select once for a camera-facing holographic preview; select again to
bring one into a front-facing native control panel inside Dockpipe. Mouse and
keyboard operate the real app there; flight is paused. **Return to flight**
restores the window to the desktop. Use that button rather than Escape, which
belongs to the embedded app. Turning the option off stops sharing immediately.

This first backend requires a native Linux X11 session with XComposite/XRender.
Wayland, Windows, macOS, and Flatpak show an unavailable explanation; their capture
and input integrations are not implemented. Some applications, their popups, or
window-manager behaviors may not embed correctly. Window-manager handoff is asynchronous, cancellable, and limited to two seconds.
A failed handoff restores the selected window. The active native app pane is
opaque and front-facing; only the preview is mapped onto the 3D billboard. Flight
keeps its existing GPU viewport while a native app is connected. The app HUD owns
an isolated native surface, so attaching an app does not promote the cockpit's
ancestors to native windows or rebuild the renderer on return.

Capture is local and limited to selected billboard windows or the three leading
open apps on the Apps planet or unambiguous assigned room apps after enabling
App control. The owner performs
two captures per second total, round-robin, scaled on the X server to at most
1280 × 720 pixels each (up to 10.6 MiB of retained preview pixels). It retains at most three preview images, reads no larger
than an 8K source surface, and stops capturing while a file HUD or app control
panel is active. Three successive capture failures stop that stream until the
next flight. Hiding or closing flight releases all streams and restores the
active window. There is no input injection API, recording, network streaming,
or background capture outside the flight owner. X save-set registration protects
the foreign window if the Dockpipe process exits unexpectedly.

CMake enables the optional backend when X11, XComposite, XRender and XCB development
libraries are available; `-DDOCKPIPE_APP_BILLBOARDS=OFF` builds the unavailable stub.
`desktop-app-window-test` skips in offscreen CI. On an X11 desktop it creates only
its own disposable app and checks bounded capture, native focus/input, detach,
closed-window handling and survival of an embedded host crash. The graph's native
smoke uses `DOCKPIPE_BILLBOARD_FIXTURE=/absolute/path/to/desktop-app-window-test`
and optional `DOCKPIPE_GRAPH_SCREENSHOT=/tmp/preview` to check actual GPU drawing,
shooting, panel resize, and hide cleanup. These tests do not select user apps.

`cockpit-test --native-handoff` runs a disposable GPU app through nested launcher
layout and first-person room handoffs. It checks wall previews before control,
unchanged GPU viewport ownership, visible return frames, fullscreen, hide/show,
ambiguous-window suppression and app-closure recovery. It skips offscreen; run it
on the X11 desktop. Set `DOCKPIPE_SPATIAL_SCREENSHOT=/tmp/handoff` for fixture-only
screenshots. `DOCKPIPE_SPATIAL_FIXTURE` may select the raster test executable instead.

Flight has a **Fullscreen · F11** control. Fullscreen hides surrounding workspace
chrome without moving the flight or native app to another window. F11 or Escape
while flying restores the previous window state and workspace panels. The app
control panel has its own **Fullscreen / Exit fullscreen** button, which stays
visible while keyboard input belongs to the app. Its content uses the real app
at the available panel resolution; HD billboard previews are only for navigation.

### Cockpit workspace (experimental)

The **Git** planet provides local Working Changes review with opposing code walls,
focused selectable diffs, staged/unstaged comparisons, review progress and index
actions. See the [Git Planet guide](../../../../packages/cockpit/docs/git-review.md)
for controls, bounds and the current PR-provider limitation.

**Settings → Cockpit mode → Enable Cockpit mode** unlocks this optional bonus
workspace. It is **off by default**, including for existing installations. While
off, Cockpit is hidden from the sidebar and View menu, the Files graph's flight
entry is hidden, and the cockpit is not constructed at startup. The ordinary
Files graph remains available in 2D and 3D.

App control, orbital motion, controller input, local voice commands, spaceship
visibility and hardware acceleration are nested under **Cockpit mode** in the
same app settings. Child controls are disabled while the master switch is off.
Settings take effect on **OK** and persist in the `cockpit` object in
`launcher.json`; **Cancel** leaves the active settings unchanged. Turning the mode
off leaves flight, releases app control/capture, and stops controller and voice
input without deleting workspace layouts, HUD positions or file drafts. Child
preferences are retained for the next time it is enabled. **Ship controls →
Cockpit settings…** opens the same settings form.

Once enabled, **Cockpit** is a standalone sidebar/View destination. Start in a ship overlooking
planets for Filesystem, Apps, Workflows, Containers, Settings, Machines and
Executions. Pilot with WASD, Q/E, right-drag/arrows and Shift. Hold the middle
mouse button and drag to look around the 3D cabin independently of steering;
release to face forward. Planets have bounded sphere and ring geometry. Flying
into the atmosphere or selecting a planet starts an approach and surface descent.
Only the destination world is loaded. The visible **Back** cockpit button,
Backspace, and return portals go back one level: a nested folder returns to its
parent, and a planet surface returns to orbit. Planet exits float in the sky;
folder exits stand in the arrival apron. Fly through a gate or click its opening.
The original file-explorer drone returns to its parent, or exits at its root.
F11 fills the screen. The original spatial file explorer remains available.

Click directly on a file or open-app building to raise its camera-facing hologram;
select it again (or click its hologram) to edit or control it. Space and controller
selection use the crosshair. Mouse selection uses the pointer, including when
looking around the cabin. Previews stay anchored above the selected building and
turn toward the camera; they do not follow the ship to unrelated locations.
The windshield shows selection hints even when the navigation panels are closed.

Filesystem uses the same built-in editor, scoped to a chosen root. Workflows,
containers, machines and execution panels reuse their existing Dockpipe controls;
workflow buildings open Workflow Studio without executing a script or bypassing
its Run action. Closing a panel returns the original screen to its sidebar slot.
Unsaved file drafts stay in the editor when navigating between planets.

The Apps planet contains open windows and installed desktop applications. Enable
**Settings → Cockpit mode → App control · Experimental** to show live wall previews for the first three open
apps before taking control. Open apps occupy the front rows, ahead of the installed
catalog. Only visible previews update; turning App control off releases them.
Click an open app building
for a live hologram, select it again for HUD control, or fly through its hangar
for fullscreen control. Installed Linux
applications launch through GIO using their desktop entries. Automatic connection
requires one exact window-class match; ambiguous or unmatched windows must be
selected from Apps after refreshing. App control is off by default; selecting a
planet while it is off does not capture windows.
The X11/Flatpak/Wayland limitations of app billboards still apply.

App control verifies that the native window is visible inside Dockpipe's window
hierarchy and synchronizes its native position and size to the HUD before
considering the connection established. Moving/resizing the panel and the existing
500 ms health check correct apps that restore their old desktop geometry.
A two-second handoff
deadline and the existing reduced-rate health check detect an app that refuses
embedding, closes, or returns to the desktop. Flight then resumes, the original
desktop window is restored when available, and the preview shows a persistent
failure explanation. The renderer preference survives leaving or hiding app
control. Some applications can preview successfully while rejecting native
embedding; a working preview does not establish control compatibility.

Containers, Machines, Executions, Workflow studio and Settings also use the
first-click hologram, second-click HUD flow. Their previews take one bounded
768 × 432 snapshot without showing the underlying page or activating its monitors.
Individual workflows preview a bounded YAML excerpt before opening the studio;
previewing never runs a workflow. Installed-app buildings show a launch card first.
Cockpit Settings uses the existing preferences form embedded in the HUD.

**Navigation** and **Ship controls** toggle their windshield panels so the
flight view can stay clear. **Arrange HUD** unlocks drag handles and resize grips. Lock the layout to resume
piloting. Positions are remembered locally and clamped to the windshield on
resize; **Reset HUD layout** recovers the default arrangement. App interaction
still uses a front-facing native surface rather than perspective-warped input.

Voice commands are experimental and off by default, including when an older
installation had voice enabled. Opt in under **Settings → Cockpit mode →
Experimental → Enable local voice commands** to use
**Hold to talk · Local**. It records only while pressed, for at most eight seconds.
Recognition runs in an isolated helper with a 15-second cancellation deadline;
no audio is written to disk or sent over the network. Native Linux currently uses
Python 3, `libpocketsphinx.so.3`, `libsphinxbase.so.3`, `libpulse-simple.so.0` and the
English model at `/usr/share/pocketsphinx/model/en-us` (commonly provided by
`libpocketsphinx3`, `pocketsphinx-en-us`, and `libpulse0`). The optional packages
are not automatically installed or downloaded. Missing dependencies show a
status message; other platforms and Flatpak do not yet have this voice backend.
Commands are: “open files/apps/workflows/containers/settings/machines/executions”,
“return to orbit”, “close panel”, “full screen”, “exit full screen”, “reset HUD”.
Voice navigates and opens controls; it cannot execute arbitrary commands.

**Settings → Cockpit mode → Enable controller** loads the optional SDL2 runtime and uses its normalized game
controller mappings: left stick movement, right stick look, triggers altitude,
A interact and B back. **Start / F1** opens the shared command menu: navigation,
item spawning, room editing, preview scrolling, flight modes and fullscreen.
D-pad / left stick or arrow keys choose; A / Enter selects; B / Esc goes back.
The selected item's description explains its action. HUD and furnishing hints
switch with the most recent keyboard/mouse or controller input. Controller input
cannot move or roll the craft while the menu is open; sticks must return to neutral
after closing. **View / P** pauses/resumes preview scrolling outside furnishing;
View finishes furnishing while that mode is active. In Explorer, LB boosts and RB
is push-to-talk. In Pilot, LB/RB roll, L3 boosts and R3 is push-to-talk. Y / F switches
modes; fullscreen remains available in the menu and through F11.
A deadzone prevents drift. Input is released on focus loss,
disconnection and leaving the cockpit. A physical controller is required to
qualify device behavior; keyboard/axis fixture coverage alone is not that proof.

Idle scenes do not continuously animate. Controller polling is limited to 30 Hz
while enabled, world geometry to 150 structures, app-window matching to ten
seconds, and installed-app inventory to 1,000 metadata entries / 8 MiB / 500 apps.
The Apps city shows up to 150 structures; its searchable launch panel includes the
remaining catalog entries. No hidden world or app-capture cache is retained.

Selected files and open apps project a camera-facing hologram above their roof.
The sprite stays upright while its anchor follows the building through the 3D
view. File previews use the same syntax-highlighting definitions as the editor,
read at most 8 KiB / 128 lines through a cancellable 2.5-second helper, and label
excerpts. Binary/non-UTF-8 files show an explanation. Links and Git metadata stay
excluded; previewing never executes a file. Select the file again to open the
normal editor. Overflowing text pauses for three seconds, scrolls at 10 pixels
per second, pauses at the bottom, then loops; short text stays still. One 768 × 432
file preview and a bounded 736 × 4096 text raster are retained. Animation crops
cached pixels instead of re-highlighting text. Switching worlds or hiding flight
clears both. App holograms reuse the existing opt-in, reduced-rate capture.

The flight deck has a structural canopy, angled physical instrument displays,
and a destination console. Its radar plots the current world; heading, pitch,
altitude and navigation state reflect the ship. Deck buttons engage autopilot
from orbit and switch destinations on the surface. Planet labels are clickable.
Middle-drag still looks around the physical cabin independently of steering.

Destinations revolve around a central sun on visible orbital paths. Outer orbits
move more slowly using the radius-to-the-three-halves period relationship, with
compressed distances and time for navigation rather than an astronomical model.
The sun determines each planet's illuminated side. Autopilot locks the moving
destination, turns smoothly, and arcs above the sun when the direct route would
cross it. **Space** skips the trip and **Escape** cancels it.

**Settings → Cockpit mode → Orbital motion** pauses or resumes the solar system and remembers
the preference. Orbit animation runs at roughly 30 updates per second while the
visible orbital view is active, and pauses during travel and editing. Hidden
views stop the timer. Surface flight and interiors retain their input-driven
rendering without an idle loop.

The scene uses perspective-projected 3D geometry with the existing optional
OpenGL-backed Qt painter. It is a bounded workspace world, not a streaming game
engine: planetary entry swaps to the selected city during a short atmospheric
transition. Editors and native app controls remain readable front-facing HUD
surfaces. No mic or controller hardware qualification is implied by fixture tests.

Flight performance: static building meshes are generated once per bounded world
page, with detailed and distant variants. Flight culls offscreen buildings and
back-facing solid surfaces before projection; fully visible polygons bypass
edge clipping. Convex building faces use Qt's convex polygon path, avoiding general
tessellation per face. Mouse-look coalesces high-rate input into one repaint request
per 16 ms while retaining the latest orientation. Only hangar shutters regenerate
during movement. Movement and travel use a 16 ms precise timer while active, with
sub-millisecond integration. Orbit has a black starfield and ten cached 256 × 256
procedural planet surfaces (2.5 MiB), ten 128 × 128 atmosphere maps (0.625 MiB),
33 illumination phases (2.0625 MiB), and one 512 × 512 sun sprite (1 MiB).
The fixed 6.1875 MiB image cache is prepared on first orbit entry and shared across
worlds. Sun-facing illumination selects and rotates a cached phase, with no bloom,
blur, downloads or per-frame image creation.

For a repeatable rendering check, run `cockpit-test --flight-performance` on the
native desktop, optionally with `DOCKPIPE_PERF_FULLSCREEN=1`. The fixture rotates
150-/500-building cities and a seven-planet orbit, reporting CPU draw median/p95
separately from input/repaint/presentation time, plus measured straight-flight
cadence, frame interval p95/max, gaps over 25 ms and idle repaint count. It uses synthetic buildings
and does not open user files or capture user applications.

### Persistent custom workspaces

In **Cockpit → Navigation → Custom workspaces**, add a planet and choose an ocean,
desert or forest theme. Select it to add a named building with a hangar, tower or
pavilion style and a position. **Save layout** persists changes; closing the edit
panel discards unsaved drafts, and **Restore saved** reloads the file. Removing a
planet or building asks for confirmation. The existing Applications planet and
all conventional pages remain available.

Use **Travel now**, the navigation workspace selector, or manual flight to reach a
workspace. **Fly to selection** approaches a planet or building; Space skips travel
and Escape cancels it. Click a building or enter its front entrance to go inside.
Rooms use WASD to walk, right-drag to look, Shift to move faster, E to interact,
and Escape or Back to return outside. The controller uses the existing left/right
sticks, boost, A and B mappings; altitude input is ignored while walking. Rooms
have a fixed eye height, wall collision and a marked exit behind the arrival point.
**Settings → Cockpit mode → Show spaceship** is optional; saved workspace interiors use first person.
Spatial Web can also use its flight and driving controls.
The existing local voice helper also accepts “open workspace” and “next workspace”.

In edit mode, select a building and **Add display**. Choose a running window or an
installed catalogue application, a wall, horizontal offset, centre height, width
and height. **Edit** changes placement or size, **Remove** removes the reference,
and **Save layout** validates the wall and exit clearance. Positions snap to
20-unit increments. The installed-app catalogue also has **Place in a custom
workspace** to preselect an application for placement.

Placing or restoring a layout never launches or controls applications. App control
defaults to disabled and remembers the saved preference. With
**Settings → Cockpit mode → App control · Experimental** enabled, entering a room
connects up to three assigned apps for wall previews when each has exactly one
matching open window. Selecting a wall then opens its native control HUD.
Multiple matching windows require an explicit choice before preview or control.
If an installed application is not running, launching it requires
confirmation. Missing applications remain editable references with an explanation.
Use **Return to flight** in the HUD to resume the room and wall preview. Leaving a
room, deleting a display or closing Dockpipe releases control and preserves the
application process. Input belongs only to the explicitly active HUD target. Native
embedding retains the existing X11 and application-compatibility limitations.

Layouts live in version 1 of `spatial-workspaces.json` in the same application
configuration directory as `launcher.json`. UUIDs identify planets, buildings and
displays. Application references store desktop entry IDs, window classes and
catalogue labels, never window titles, native handles, commands, captured pixels,
textures or application contents. Window classes are not unique process identities:
ambiguous windows must be selected again after restarting. Saves are atomic and
reject conflicting writes from another instance. Malformed, oversized and future
versions remain untouched; restore a supported file before saving.

Capacity is 24 custom planets, 64 buildings per planet, 24 displays per room,
256 buildings and 2,048 displays in total, and a 2 MiB configuration file. These
limits bound the launcher-sized universe and metadata load. Only the active world
has geometry; inactive rooms retain configuration only. One room uses at most
three distinct streams and the existing total rate of two captures per second.
Repeated references share the selected stream; out-of-view displays suspend
capture and an active native HUD gets the existing health/geometry check instead.
Hiding or exiting the scene stops input, capture and rendering and releases pixels.
Procedural geometry needs no asynchronous external assets; filesystem previews
retain their existing cancellable helper. This extends the Qt painter/OpenGL
renderer without introducing a game engine, instancing layer or package SDK.

`spatial-workspace` tests versioned persistence, corruption, conflict protection,
aggregate capacity and failed-save retry. `cockpit-test --spatial` exercises editing,
walking, navigation, permission gates, process restart, idle activity and bounded
scene performance. Set `DOCKPIPE_SPATIAL_FIXTURE` to the built
`desktop-app-window-test` executable for disposable native input/capture/lifecycle
checks. `DOCKPIPE_SPATIAL_SCREENSHOT` saves only fixture screenshots. Physical
controller/microphone behavior and compatibility with each real application still
need device/application-specific testing. GPU memory is reported only when the
host exposes a per-process allocation; unavailable measurements are not zero.


### Spatial web preview inside Cockpit mode

Enable **Settings → Cockpit mode** and enter **Cockpit → Web**. The start screen
has an **Explore GitHub repositories** button and a real prefilled address; press
**Go** to use that address, or replace it with any HTTP or HTTPS address.
Web occupies the main cockpit view; **Back to cockpit** restores flight.
Dockpipe loads the actual website and its JavaScript in Qt WebEngine behind the
opaque start screen or current world. Navigation automatically builds the next
walkable room of up to twelve destinations. Geometry and signs appear first;
image sprites arrive individually without resetting your position. **Build space**
refreshes the interpretation. WASD moves and right-drag looks. Approach a destination
while facing its entrance, or select its display with a click / E, to follow its
link in the same browser. Walking past a destination does not open it. The current world
stays visible during loading, then a short phase effect reveals the next world.
Browser back/forward navigate real history.
**Live page** returns to the original interactive page for forms, authentication,
provider controls and content the spatial projection cannot represent.

The projection separates content meaning from construction. A hub/collection/
destination graph feeds a bounded planner using reusable parts, transforms,
materials, interaction anchors and traversal connections. The layout selector can
rebuild the same content as branching paths, a loop, a promenade or a layered
flight network without reloading the page. Page evidence supplies independent
starting style parameters. Products use image sprites and prices on fixtures;
they are not reconstructed as detailed 3D models.

Vehicle listings with structured Vehicle/Car markup or model-specific automotive
links become physical cars. Walk up to a car, aim at its body and press **E** (or
controller **A**) to enter. **W/S** accelerates/reverses, **A/D** steers, **Space**
brakes, and right-drag looks independently. **E**, **Esc** or controller **B** exits
at a clear spot beside the car; the vehicle stays parked where you left it.
Its separate sign still opens the original listing. Showrooms start on foot;
selecting **Drive** enters a nearby car. Leaving Web pauses movement.

These are explicitly **approximate procedural models**, with pickup, SUV, coupe,
sedan and van proportions inferred from the listing. Model names and years come
from page content; a year does not imply exact factory geometry. This slice does
not import manufacturer GLB/glTF files or reconstruct meshes from photographs.
The ground layouts retain the vehicles; the elevated flight layout presents their
listing surfaces instead. There is no suspension, terrain following or crash simulation.

The cinema retains a specialized shared-screen layout. Selecting a video poster
plays the real browser video on its central screen without leaving the theatre.
**Play / pause** and **Stop film** control that stream. Playback depends on the
site's consent/login requirements and the installed browser's codec/DRM support.
**Live page** remains available for provider controls.

Construction validates resource limits, content bindings, interaction visibility
and connected unobstructed routes before publishing. A simpler validated layout
is used if a proposal fails. Geometry is cached once; no cloud model, generation
or asset downloads run during movement. The 420 ms phase effect preserves the
current world while loading. Empty or failed destinations preserve that world.
See [construction architecture and current limits](../../../../packages/cockpit/docs/spatial-world-construction.md).

The browser is created only on entering Web. **Live page** uses the same tabbed
browser component as the top-level Browser, including annotations, downloads,
extensions and page tools. Its isolated temporary profile imports no existing
browser cookies, and disabling Cockpit mode destroys the session. Switching tabs
preserves the current world and camera; **Build space** projects the selected
HTTP(S) page. Extension pages are not spatial content. Inactive tabs are muted
and suspended. Background loading is muted; the browser freezes once image sprites
finish arriving. **Live page**, navigation and rebuilding reactivate it. Only an
explicit **Live page** action reveals the website. Leaving the Web view also
cancels conversion and stops loads. Returning to an existing world keeps its
browser frozen. Spatial extraction
uses an isolated JavaScript world, walks at most 5,000 elements, returns at most
96 KiB of metadata and retains twelve shared 512 × 320 textures (7.5 MiB).
Each world has at most 96 primitive solids (up to 768 faces), twelve textured
surfaces and 512 vertices / 256 faces of additional polygon geometry.
Vehicle scenes add at most twelve cached models with 224 faces each. Only a moving
vehicle updates its transform; the rest stays cached. Collision tests the complete
rotated footprint against solids, room bounds and other vehicles, with movement
steps no longer than 15 units or 0.03 radians.
World geometry is cached once; frames project/cull/sort it only while the view
needs painting. Static worlds have no idle animation or recurring capture. Explicit video
playback samples one 512 × 288 stream at up to twelve captures per second.
Conversion has
a 15-second deadline; navigation, hiding and teardown invalidate pending callbacks.
Spatial projection accepts only HTTP(S) destinations without embedded credentials.
Live page shares the ordinary browser's downloads, user-initiated tabs and saved
site permission policies. Ask prompts are confined to its selected visible page;
background spatial content cannot request an interactive permission decision.
Unsupported permissions are denied. Browser TLS validation and Chromium sandboxing
retain their defaults.

Build the optional [Cockpit package](../../../../packages/cockpit/README.md) with
`DOCKPIPE_SPATIAL_WEB=ON` for the spatial view. The normal launcher includes Qt
WebEngine for its top-level Browser but does not include spatial extraction or
world rendering. Use an up-to-date Qt WebEngine distribution for release builds; this
local host's Qt 6.2 browser does not establish modern-site or production security
qualification. Cross-platform browser packaging and codec/DRM coverage remain
unqualified. The native fixture is `cockpit-test --spatial-web`; the pure model
checks run as `cockpit-test --spatial-web-model`. Vehicle geometry and movement
checks run as `cockpit-test --spatial-vehicles`; set `DOCKPIPE_WEB_TEST_VEHICLES=1`
for the native browser vehicle fixture, optionally with `DOCKPIPE_WEB_LIVE_URL`.

### Optional Cockpit package

Cockpit implementation, spatial browser resources, voice helper and feature tests now live
in `packages/cockpit`. Settings installs the resolver package on first enable using
the configured HTTPS catalog and the CLI's checksum-verified package installer.
The switch is not saved as enabled until loading succeeds; failed downloads leave
it off with a retry action. Cancel stops the in-flight package operation. The
normal launcher has no link dependency on the spatial renderer. Ordinary browsing
uses its built-in shared browser runtime and Qt WebEngine. Shared file
editing remains in the small `dockpipe-launcher-ui` library shipped with desktop.
See the package README for native bundle compatibility and development builds.

With **Bring open desktop apps into Tiling** enabled, a desktop panel's close
button sends the OS window a normal close request. Unsaved-work prompts and
cancelled requests keep the panel alive. With management off, closing the panel
only releases the window. Disabling management or quitting DockPipe releases
apps without requesting that they close.


### Dashboard canvas gadgets

First-party widgets on the dashboard **Canvas** use compact rounded frames and a
small drag handle/menu instead of repeated window buttons and footer actions.
Open the widget menu for refresh, full view, expand, minimize or removal. Existing
widget placement and resizing remain available. Tiling and top-level pages retain
their ordinary controls.

**Send input** becomes an inline target picker, growing text pill and round send
button. Ctrl+Enter still sends only to the explicitly selected terminal; changing
presentation neither opens a shell nor sends a draft. Status appears when there
is an outcome or input error. System resources use compact cards and thin meters,
retaining unavailable readings rather than inventing zero values.

The canvas **Browser** starts with just a slim tab strip and the live viewport.
Its eye button or Ctrl+L reveals normal address/navigation controls, and the same
button returns to the viewport. Drag the small grip to move the browser widget.
Drag a browser tab out of its strip to create a separate canvas browser widget;
the live page, JavaScript state, navigation history, profile, extension session
and downloads are retained. Escape cancels a tab gesture. Closing the source
widget leaves the detached tab alive, and the shared profile is released after
its last browser closes. Clear browsing data asks you to close detached siblings
first so it cannot clear a session still used by another widget. Dashboard widget
limits still apply; a rejected tear-out leaves the original tab in place.

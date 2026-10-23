# Flatpak desktop and Marketplace packages

The Linux amd64 launcher-plus-CLI bundle is published on the staging package host.
Use the [installation guide](../../../../docs/install.md#flatpak-desktop-linux-amd64)
for KDE runtime setup, verified download URLs, user installation, launch commands,
manual bundle updates and Marketplace setup. This is not a Flathub listing or an
updating Dockpipe Flatpak remote.

Dockpipe running inside Flatpak selects Flatpak Marketplace artifacts. Native
Dockpipe selects native artifacts, even on an atomic OS or a machine with Flatpak
installed. Selection follows the process boundary, without a distribution toggle.

The base app contains the launcher, CLI, required core, Git and container clients
(Docker, Compose and Buildx). It does **not** install the optional Marketplace
catalog. Optional packages remain ordinary Dockpipe archives, built for the
Flatpak runtime and downloaded individually.

## Runtime and host integrations

The app uses `org.kde.Platform//6.10`. Package dependencies run inside that runtime;
additional tools and libraries live in the selected package's `assets/tooling/bin`.
First-party dependency inputs are version/checksum pinned in the adjacent lock
files. Azure wheels match the runtime's Python ABI. Wrangler uses its npm integrity
lock, with installation scripts disabled.

Container clients connect to a host engine. Both the launcher/CLI and subprocess
clients select the same socket: explicit Docker environment settings take priority,
then `/run/docker.sock`, the user's Docker socket, or the user's Podman socket.
Projects and temporary bind sources use host-visible paths. No container engine is
installed or started by the Flatpak app. Podman's API socket is supported as a
connection target; its complete workflow/Compose parity still needs native tests.
See [Podman's socket documentation](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html).

Explicit bridges through `flatpak-spawn --host` support desktop editors, host
Codex/Claude CLIs, 1Password, QEMU and host service/GPU management. These integrations
use the host's installed tools and authentication. They are not silently replaced
by a different provider. The app requests the Flatpak host execution permission,
home/project access, display/graphics, network and engine socket access. This is a
development tool with approved host integrations, not an isolated viewer.

Embedded terminals use that explicit host bridge too. The shell picker discovers
installed host shells, and new sessions receive the chosen project directory and
real PTY descriptors. The host service establishes the controlling terminal;
Dockpipe's fixed supervisor cleans the shell's foreground and background jobs on
close. Host `ps`, `head`, `timeout`, `sleep` and GNU `env` with
`--default-signal` support are required for bounded cleanup and interrupt handling.
Unavailable host access or utilities produce an error instead of opening a
different shell inside the sandbox. No host dependency is installed automatically.

The bundle also permits talking to `org.kde.StatusNotifierWatcher` for the Qt tray
icon. It does not request unrestricted session-bus access. The launcher checks tray
availability before registration and before hiding on close; if the tray disappears
while the launcher is hidden, the window is restored. Without a tray, closing exits.

On normal launcher startup, terminal integration runs automatically in a background
task. It reads only the needed host environment fields through `flatpak-spawn`,
preserves an existing host `dockpipe` command, and otherwise creates
`~/.local/bin/dockpipe`. No Settings switch or extra setup action is required.
If the host PATH omits that directory, setup adds an idempotent labelled block to
Bash/Zsh startup configuration or a Fish `conf.d` file. Existing content is retained;
symlinks, conflicting files, and modified Dockpipe blocks are not replaced. Setup
errors stay visible in the launcher without preventing it from opening.

The wrapper uses Flatpak's normal app selection and forwards arguments and exit
status. It resolves the caller's physical working directory, passes it explicitly
with `--cwd`, and grants that directory filesystem access for this invocation.
Mounted project folders outside the home directory therefore remain the target of
commands such as `dockpipe init`. If the directory disappears or Flatpak cannot
enter it, the command fails instead of initializing the home directory.

Launcher startup upgrades the exact previous Dockpipe-generated shortcut even when
it is already on PATH. User-modified shortcuts and native commands are preserved;
updates use an atomic file replacement and retain executable permissions.
Version/installation smoke probes and native launcher runs do not perform terminal
setup. Remove the wrapper and labelled PATH configuration when uninstalling the app.

For older bundles affected by [issue #127](https://github.com/Dockpipe-Industries/dockpipe/issues/127),
explicitly select the project directory when invoking Flatpak directly. See the
[installation guide](../../../../docs/install.md#flatpak-desktop-linux-amd64) for
the command. The source fix must be included in a new bundle before launcher
startup can update an installed shortcut.

The launcher filters only the older Qt text-input-v3 warning for a null surface
on leave, matching [Qt's treatment of destroyed surfaces as normal teardown](https://github.com/qt/qtbase/blob/dev/src/plugins/platforms/wayland/qwaylandtextinputv3.cpp).
No Wayland input behavior is disabled or replaced. Set `DOCKPIPE_QT_DIAGNOSTICS=1`
inside the Flatpak to retain the original diagnostic. Other messages, non-null
surface mismatches, and critical errors continue through the existing Qt handler.

Remote user services invoke `flatpak run --command=dockpipe APP_ID`; they never
attempt to execute a sandbox-only `/app` path on the host. The service file remains
in the app's XDG config directory and is explicitly linked into the host user
manager. VM workflows require host QEMU. GPU setup uses the host's authorization
prompt and refuses native package-manager installation on an OSTree host.

Pipeon packages include their container helpers and prepared code-server context,
including the provider catalog, so an installed package does not need this source
checkout or a Go compiler. Nested container mount targets are created by the user
and code-server runs with a private umask, preserving durable-state checks across
relaunches. When
no native Pipeon desktop shell is installed, its UI opens through the browser
portal. Stop the workflow to close that browser-backed stack. Core-only installs
remain core-only; install the optional packages needed for the chosen workflow.

## Desktop build

Prerequisites: Flatpak, Python 3, `org.kde.Sdk//6.10`, `org.kde.Platform//6.10`,
`io.qt.qtwebengine.BaseApp//6.10` for the embedded browser, a
matching static CLI, a verified core store, and `DOCKPIPE_FLATPAK_BROWSER_SDK` pointing to
the separately verified public Spatial-Web Flatpak Browser SDK prefix.
`DOCKPIPE_FLATPAK_COCKPIT_ASSETS` optionally selects the freshly verified locked Cockpit
compatibility fixture prepared by `prepare-cockpit.py` for the independent package integration lane. Generic host extension fixtures run without it. The builder does not install its
prerequisites or install the resulting app on the build machine.
The build bundles checksum-pinned libXRes 1.2.3 for X11 process identification,
including RustDesk windows that omit their process ID property. Its license is
included in the app. Qt WebEngine comes from the matching BaseApp.
Build-time browser tests disable Chromium's inner sandbox because BaseApp cannot
spawn nested Flatpak instances during a build. Installed apps retain browser
sandboxing. BaseApp development headers and build metadata are removed after tests.

```bash
bash release/packaging/desktop/flatpak/build.sh VERSION /absolute/path/dockpipe \
  /absolute/path/core-store /absolute/path/new-output
```

Before compiling, the builder reads back the Flatpak Browser SDK against
`release/dependencies/spatialweb-flatpak-browser.lock.json`. It requires the exact
qualified SDK, Platform and BaseApp commits, GNU 15.2.0, Qt Core/Gui/Widgets/Network/
PrintSupport 6.10.3 and WebEngineCore/WebEngineWidgets 6.10.2. The native five-platform
SDK lock is separate and unchanged. The builder checks actual component CMake
packages and binary dependency resolution, recording `browser-abi.txt`.
Different components or refs require a separately qualified Spatial-Web profile;
Dockpipe does not embed its source. Only the
Browser shared library, installed notices and its consumer receipt enter the app.
The workspace-reader ships beside the launcher and is exercised by Platform smoke.
The [Flatpak SDK consumer contract](../../prepare-flatpak-browser-sdk.md) documents
preparation, provenance, immutable refs and the separate package identity.

The builder compiles the launcher in the SDK, runs the Qt regression tests, and checks the
launcher, CLI, core-only inventory, workflow execution and package target selection
in the smaller Platform. Outputs include an unsigned staging `.flatpak` bundle, OSTree
repository, app build directory and SDK/runtime commit receipts.

The required installed Cockpit check reuses `PackageRenderingTest.cpp` against the
shipped Host UI in the actual Platform sandbox. SDK-context CTest excludes its
`installed-platform` label because this check runs after app installation, under
its own X11 display. The hosted workflow then requires that check to pass with
zero compositor failures, movement, software GL completion and recovery. It
re-extracts the exact locked archive and records actual mapped Qt, Browser, Host
UI and Cockpit paths and hashes. Bundled native Qt does not establish isolation
inside the host process. This explicit runtime compatibility proof does not
rewrite the native Cockpit compiler/Qt contract or qualify physical GPU behavior.


```bash
flatpak install --user ./dockpipe-desktop_VERSION_linux_amd64.flatpak
flatpak run com.dockpipe.Dockpipe
flatpak run --command=dockpipe com.dockpipe.Dockpipe --version
```

The CLI is included in the same app. Its normal XDG data is separate from native
Dockpipe. Project-local packages and explicit data-root overrides still take
precedence. The bundle exports app branch `staging` and supplies Flathub as its KDE
runtime-repository hint. It does not configure an updating Dockpipe Flatpak remote.

## Optional package builds

Custom packages own `assets/flatpak/build.sh COMPILED_STORE` and
`assets/flatpak/test.sh COMPILED_STORE`. The first builds offline in the SDK; the
second runs in the Platform without network, host execution or engine sockets.
Inputs must be prepared and verified before either recipe runs.

```bash
bash release/packaging/desktop/flatpak/build-package.sh /absolute/path/output/app \
  /absolute/path/package-source /absolute/path/new-package-output
```

The bundled-library fixture proves a package can execute its own executable and
shared library, including dependency preflight and package-scoped `PATH` selection.

First-party packages additionally own `assets/flatpak/package.json` profiles naming
exact artifacts, extra bundled tools, host integrations and required release
payloads. Each pinned native archive must carry a profile declaring its own
identity. The catalog builder rejects missing, duplicate or unowned artifacts and
unknown dependencies. Before adaptation, it reads only each selected archive’s
top-level package manifest and verifies both package dependencies and required
resolvers against the pinned store and shipped core. Nested fixtures do not
become catalog packages. A historical store with absent umbrella packages or
resolver archives must be repaired in its owning source; no checkout-only alias
or dependency removal is substituted. Final CLI store validation still runs.
It rebuilds archives with their runtime-specific dependencies,
executes dependency/preflight checks in the Platform, then exports a complete store
with checksums and a qualification receipt. Generated Flatpak metadata preserves the
workflow namespace and declares `platforms: [flatpak]`; native metadata is untouched.

```bash
python3 release/packaging/desktop/flatpak/prepare-package-tools.py \
  /absolute/path/new-tools --cache /absolute/path/download-cache \
  --app /absolute/path/output/app
python3 release/packaging/desktop/flatpak/build-catalog.py \
  /absolute/path/output/app /absolute/path/composed-linux-amd64-native-store \
  /absolute/path/new-tools /absolute/path/new-catalog
```

The native input store is assembled from the new core, the checksum-pinned
baseline, and the independently published DorkPipe and Pipeon archives. The
catalog verifies every archive against that native store, then qualifies each
adapted package inside the Flatpak Platform. Its inventory comparison catches
missing profiles or artifacts instead of publishing a partial store. Optional
dependency locks currently target Linux amd64 only.

## Release integration and proof limits

The reusable `.github/workflows/flatpak.yml` builds the desktop and generic bundled-library fixture, qualifies installed host terminals, and checks real Docker/Compose bind mounts. Its success remains required for the desktop release. It uploads the desktop bundle and a verified core-only store under the `linux-amd64-flatpak-org.kde.Platform-6.10` target. CLI, launcher and host SDK source remain in the main Dockpipe repository.

The independent `.github/workflows/flatpak-catalog.yml` retains strict optional catalog, installed Cockpit rendering and Marketplace lifecycle checks. It uploads `flatpak-optional-catalog-qualification-amd64` rather than runtime release artifacts. A failure in that lane does not make an optional package a host dependency. Marketplace promotion must use its qualified package store and update its own catalog pointer separately.

Native and Flatpak stores cannot be substituted, including through pinned store URLs. Runtime branch upgrades require package rebuilds and qualification.

Use the [production release catalog](https://packages.dockpipe.com/packages/runtime/latest.json)
to discover the current immutable release manifest. Its
`downloads["linux-amd64-flatpak-org.kde.Platform-6.10"].desktop` entry names the
bundle relative to the manifest directory. The same release supplies
`SHA256SUMS.txt` and the matching Flatpak core store. Optional Marketplace packages come from the independent package catalog. Do not pin installation
instructions to a historical staging candidate or assume a GitHub release has a
Flatpak attachment. The [installation guide](../../../../docs/install.md#flatpak-desktop-linux-amd64)
resolves the current catalog and verifies the bundle before installation.

For pre-release qualification, select the
[staging catalog](https://packages.staging.dockpipe.com/packages/runtime/latest.json)
explicitly and keep its manifest, checksums and stores together. Published bundles
currently retain the internal OSTree branch name `staging`; that branch name does
not select the production or staging package host.

Dependency/preflight checks cover package availability, not authenticated cloud
operations, editor interaction, VM boot or all container lifecycle behavior. These
still depend on the user's host tools, credentials and engine. Native Bazzite,
Podman/SELinux behavior, ARM64 Flatpak builds and a signed/updating Flatpak remote
remain separate qualification/publication work.
Do not describe local or hosted checks as universal workflow parity.

Local Linux amd64 qualification exercised all 61 optional artifacts in the Platform,
the core-only launcher/CLI, a bundled shared library, real Docker/Compose bind mounts,
and a read-only systemctl call through the host bridge. Pipeon started its stack,
MCP endpoints and browser UI twice from the same isolated project/state, without a
source checkout or provider credentials. That test reused an existing code-server
image; it does not prove a clean image build, authenticated agents or model inference.

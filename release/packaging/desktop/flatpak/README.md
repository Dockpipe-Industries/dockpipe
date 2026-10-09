# Flatpak desktop and Marketplace packages

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

Prerequisites: Flatpak, Python 3, `org.kde.Sdk//6.10`, `org.kde.Platform//6.10`, a
matching static CLI and a verified core store. The builder does not install its
prerequisites or install the resulting app on the build machine.

```bash
bash release/packaging/desktop/flatpak/build.sh VERSION /absolute/path/dockpipe \
  /absolute/path/core-store /absolute/path/new-output
```

The builder compiles the launcher in the SDK, runs six Qt tests, and checks the
launcher, CLI, core-only inventory, workflow execution and package target selection
in the smaller Platform. Outputs include an unsigned test `.flatpak` bundle, OSTree
repository, app build directory and SDK/runtime commit receipts.

```bash
flatpak install --user ./dockpipe-desktop_VERSION_linux_amd64.flatpak
flatpak run com.dockpipe.Dockpipe
flatpak run --command=dockpipe com.dockpipe.Dockpipe --version
```

The CLI is included in the same app. Its normal XDG data is separate from native
Dockpipe. Project-local packages and explicit data-root overrides still take
precedence. Installing a test bundle does not configure an updating Flatpak remote.

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
payloads. The catalog builder rejects missing, duplicate or unowned artifacts and
unknown dependencies. It rebuilds archives with their runtime-specific dependencies,
executes dependency/preflight checks in the Platform, then exports a complete store
with checksums and a qualification receipt. Generated Flatpak metadata preserves the
workflow namespace and declares `platforms: [flatpak]`; native metadata is untouched.

```bash
python3 release/packaging/desktop/flatpak/prepare-package-tools.py \
  /absolute/path/new-tools --cache /absolute/path/download-cache \
  --app /absolute/path/output/app
python3 release/packaging/desktop/flatpak/build-catalog.py \
  /absolute/path/output/app /absolute/path/compiled-release-packages \
  /absolute/path/new-tools /absolute/path/new-catalog
```

Prepare release inputs with the normal source-build hooks first. Pipeon restores
its status/stop consumers after replacing its stack archive; release packaging also
restores top-level workflow consumers invalidated by resolver compilation. The
catalog's inventory comparison catches missing artifacts instead of publishing a
partial store. Optional dependency locks currently target Linux amd64 only.

## Release integration and proof limits

The reusable `.github/workflows/flatpak.yml` builds the desktop, bundled-library
fixture and first-party catalog, and checks real Docker/Compose bind mounts.
Standalone runs upload qualification artifacts. The release workflow calls the
same job and requires its success before assembly/publication. Its catalog entry is
`linux-amd64-flatpak-org.kde.Platform-6.10`, separate from all native stores; the
Flatpak download contains both launcher and CLI. Native and Flatpak stores cannot
be substituted, including through pinned store URLs. Runtime branch upgrades
require package rebuilds and qualification.

Dependency/preflight checks cover package availability, not authenticated cloud
operations, editor interaction, VM boot or all container lifecycle behavior. These
still depend on the user's host tools, credentials and engine. Native Bazzite,
Podman/SELinux behavior, arm64, hosted CI and public distribution must be qualified
separately. Do not describe these local checks as universal workflow parity.

Local Linux amd64 qualification exercised all 61 optional artifacts in the Platform,
the core-only launcher/CLI, a bundled shared library, real Docker/Compose bind mounts,
and a read-only systemctl call through the host bridge. Pipeon started its stack,
MCP endpoints and browser UI twice from the same isolated project/state, without a
source checkout or provider credentials. That test reused an existing code-server
image; it does not prove a clean image build, authenticated agents or model inference.

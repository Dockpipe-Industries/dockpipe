# Desktop distribution

Desktop means **launcher plus CLI**. Headless servers and CI continue to use the
CLI packages. See [the installation guide](../../../docs/install.md) for user
choices.

The desktop includes the top-level Browser and Qt WebEngine independently of the
optional Cockpit package. Native amd64 DEBs bundle Qt 6.10.3, its plugins and
WebEngine helper/resources/locales. `dpkg-shlibdeps` resolves the remaining
distribution libraries. ARM64 DEBs retain distribution Qt and its helper package;
macOS/Windows deployment tools bundle the matching helper, resources and locales.
Flatpak uses the WebEngine runtime supplied by its matching KDE Platform.

| Target | Desktop artifact | Contents |
| --- | --- | --- |
| macOS arm64 / amd64 | `dockpipe-desktop_VERSION_darwin_ARCH.dmg` | Apple Installer package: `/Applications/DockPipe.app` plus `/usr/local/bin/dockpipe` symlink |
| macOS Homebrew | `dockpipe-desktop_VERSION_darwin_ARCH.zip` | Same self-contained app; cask depends on the CLI formula |
| Ubuntu 22.04 / 24.04 amd64; Pop!_OS 22.04 | `dockpipe-desktop_VERSION_amd64.deb` | Launcher, private Qt 6.10.3 runtime, icons, menu entry and exact-version CLI dependency |
| Ubuntu 22.04 / 24.04 arm64 | `dockpipe-desktop_VERSION_arm64.deb` | Launcher with older distribution Qt; modern browser parity and native ForgePipe/Cockpit SDK are not qualified |
| Windows amd64 | `dockpipe_VERSION_windows_amd64.msi` | CLI, core package, deployed Qt launcher and Start menu shortcut; launcher selected by default |
| Linux amd64 Flatpak (staging) | `dockpipe-desktop_VERSION_linux_amd64.flatpak` | Launcher, CLI, core and container clients; requires KDE Platform 6.10; optional packages use a separate runtime-specific store |

The macOS app contains the CLI in `Contents/Helpers`, a wrapper in `Contents/MacOS`,
and only the required core package in `Contents/Resources/share/dockpipe`. The wrapper
sets the existing `DOCKPIPE_SYSTEM_ROOT` only when unset. User data retains its
normal location; no engine-specific macOS app knowledge is needed. Launcher
discovery preserves explicit `DOCKPIPE_BIN` and repository development binaries,
then checks its sibling CLI before falling back to PATH.

## Native builds and checks

First run `bash release/packaging/build-platform.sh VERSION` on the target host.
On Linux, run `bash release/packaging/desktop/build-linux.sh VERSION release/artifacts`.
It requires Docker and builds natively in Ubuntu 22.04. On amd64 the image installs
the pinned Qt 6.10.3 `gcc_64` SDK using `install-linux-qt.sh`, also used by native
SDK consumers through `DOCKPIPE_QT_ROOT`. ARM64 retains distribution Qt 6.2. The
official modern ARM64 Qt binaries require a newer glibc and do not establish an
Ubuntu 22.04 ARM64 SDK baseline. Do not infer native Cockpit or ForgePipe support
from a successful ARM64 launcher build.

The amd64 deployer checks every ELF against the Ubuntu 22.04 glibc ceiling,
collects the linked SDK library closure and runtime plugins, and uses relative
RPATHs and `qt.conf` under `/usr/lib/dockpipe`. It never copies SDK source, headers,
examples or CMake files into the DEB. The package includes Qt license texts,
upstream SPDX inventories and source references. Every deployed ELF participates
in `dpkg-shlibdeps`; bundled SONAMEs resolve to this package rather than distro Qt.
The installed-runtime check verifies hashes, dependency resolution, Qt isolation
and absence of debug sections.

Native Linux amd64 Release builds retain optimized private symbols before stripping
the public launcher and UI library. Set `DOCKPIPE_PRIVATE_DEBUG_DIR` to private storage
outside the artifact directory; the default is the sibling `ARTIFACT_DIRECTORY.private-debug`.
Keep this directory private and retain it alongside the matching release. It is
not an upload or publication input. Qt's vendor runtime arrives separately;
this process does not manufacture upstream private debugging symbols.

The all-inclusive amd64 release lane sets `DOCKPIPE_BUILD_COCKPIT=1` and supplies
the attested `DOCKPIPE_FORGEPIPE_SDK` plus the already-built CLI. The SDK is mounted
read-only, and Cockpit uses the same CMake build and Qt toolchain as the launcher.
Its package-owned producer qualifies native rendering and emits a separate
`cockpit-store`; the desktop DEB still contains the launcher and runtime only.
On macOS, run `bash release/packaging/desktop/build-unix.sh VERSION release/artifacts`
with CMake and Qt 6.8.3 including WebEngine, WebChannel and Positioning modules,
with deployment target macOS 13.
Windows uses `build-windows.ps1` with Visual
Studio 2022, the same Qt modules and `windeployqt`, then the existing WiX MSI builder.
The native deployment scripts reject missing WebEngine helper/resources/locales.
macOS signs the renderer with Qt's supplied entitlements and preserves them when
signing the enclosing app, following [Qt's WebEngine deployment guide](https://doc.qt.io/qt-6.8/qtwebengine-deploying.html).

`smoke.py` runs the installed launcher's `--check-installation` diagnostic outside
the checkout, with isolated user directories and no injected CLI override. It
verifies Qt loads, the selected CLI runs, CLI and launcher versions match, and the
actual window stays running. `dockpipe-launcher --version` reports the generated
release version without requiring a display server.
On disposable macOS CI runners, `smoke-unix.sh` mounts the DMG, uses Apple's real
Installer, runs a host workflow through the installed CLI, checks the desktop,
and exercises a second installation. It refuses that system-install test outside
GitHub Actions. `smoke-linux.sh VERSION [ARTIFACT_DIRECTORY]` installs the same CLI
and launcher DEBs with APT in clean Ubuntu 22.04 and 24.04 containers on each native
architecture. Neither container has the build toolchain or preinstalled Qt. It
checks runtime closure and licenses on amd64, dependency resolution, core-only inventory, launcher startup, a host
workflow, and desktop removal preserving the CLI. The host OS is not modified.
An additional amd64 probe loads a page and executes JavaScript through the installed
WebEngine helper. Docker build tests and this probe disable Chromium's inner
sandbox only inside disposable test containers, whose namespace restrictions
prevent that sandbox from starting. Installed desktop settings are unchanged;
these checks do not qualify the browser's inner sandbox on a normal desktop.
Windows smoke installs the real MSI, checks the app/CLI, modifies the launcher
feature and uninstalls. Homebrew separately tests formula/cask installation and
cask removal on both Mac architectures before updating the tap.

Published staging Linux desktop artifacts include DEBs and the
[Flatpak launcher-plus-CLI bundle](flatpak/README.md), with separate on-demand
runtime-compatible packages. The Flatpak bundle is available from the staging
package host; this does not establish Bazzite/Pipeon or full Podman/SELinux support.
See [Flatpak installation and manual updates](../../../docs/install.md#flatpak-desktop-linux-amd64).
Do not put the glibc Qt binary into
an Alpine package. Other Linux package formats and portable archives remain CLI
options. Native Mac/Windows installation proof cannot be inferred from a Linux
cross-build or script fixture test.

## macOS signing and notarization

The native builder supports these references to an already provisioned keychain:

- `DOCKPIPE_MAC_APP_IDENTITY`: Developer ID Application signing identity.
- `DOCKPIPE_MAC_INSTALLER_IDENTITY`: Developer ID Installer signing identity.
- `DOCKPIPE_MAC_NOTARY_PROFILE`: saved `notarytool` keychain profile.

When all are supplied, the builder signs the app and installer, notarizes and
staples the app and installer, then signs, notarizes and staples the DMG. Without
them it makes an ad-hoc signed app and unsigned installer for development/staging
qualification. Current hosted jobs do not provision Apple signing credentials;
those outputs are not Gatekeeper-approved public desktop releases. Signing
provisioning and native signed-release verification remain release requirements.
Do not document a quarantine bypass as normal installation.

Qt deployment follows [Qt's macOS deployment guide](https://doc.qt.io/qt-6/macos-deployment.html).
The [Homebrew cask](../homebrew/README.md) uses an app artifact and formula
dependency as documented in the [Cask Cookbook](https://docs.brew.sh/Cask-Cookbook).
Dynamic Qt license notices are included in the amd64 Linux, macOS and Windows payloads.

## Ownership, updates and removal

The direct macOS installer refuses foreign CLI links/files, an Apple Silicon
Homebrew command, or an existing app without its package receipt. Use the cask
when Homebrew already manages Dockpipe. Do not mix a cask-managed app and a
package-managed app. Re-running the direct installer updates its managed app
and link. It does not install a daemon or change user data.

For a direct-installer removal, first verify the receipt with
`pkgutil --pkg-info com.dockpipe.desktop` and verify that
`readlink /usr/local/bin/dockpipe` reports
`/Applications/DockPipe.app/Contents/MacOS/dockpipe-cli`. Remove that link and
`/Applications/DockPipe.app` with administrator permission, then run
`sudo pkgutil --forget com.dockpipe.desktop`. Do not remove a command managed by
another installer. For Homebrew use `brew uninstall --cask dockpipe-desktop-staging`;
the CLI formula remains unless separately removed. All methods retain user data.

## Activation order

1. Run the new release matrix as a dry run and inspect native desktop results.
2. Configure and qualify Apple signing before presenting DMGs as normal public
   desktop downloads; staging outputs must retain their qualification status.
3. Publish a candidate containing the desktop artifacts and complete checksums.
4. Copy reviewed tap source changes to the public tap and pass its native cask
   checks before publishing the generated cask.

No live tap or release update is performed by editing these source files.

## Native symbol boundaries

Before macOS signing, the producer explicitly strips only Dockpipe's launcher
and host UI dylib, verifies that required external symbols remain unchanged,
and rejects debug sections or private defined symbols. Apple strip may insert
its synthetic `radr://5614542` loader marker; the audit recognizes it only at
symbol index zero with the exact Mach-O type, fields and indirect-symbol-table
preconditions, and records it separately from private symbols. The app carries
`Contents/Resources/dockpipe-native-symbol-audit.json`; its hashes identify the
stripped binaries before signing changes their bytes. Third-party runtime
libraries retain their existing deployment and licensing rules.

The Windows producer rejects PDBs, object files, import libraries and other
private compiler intermediates anywhere in the desktop stage before the MSI
harvests it. Release builds do not enable compiler/linker debug output.

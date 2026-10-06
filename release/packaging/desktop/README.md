# Desktop distribution

Desktop means **launcher plus CLI**. Headless servers and CI continue to use the
CLI packages. See [the installation guide](../../../docs/install.md) for user
choices.

| Target | Desktop artifact | Contents |
| --- | --- | --- |
| macOS arm64 / amd64 | `dockpipe-desktop_VERSION_darwin_ARCH.dmg` | Apple Installer package: `/Applications/DockPipe.app` plus `/usr/local/bin/dockpipe` symlink |
| macOS Homebrew | `dockpipe-desktop_VERSION_darwin_ARCH.zip` | Same self-contained app; cask depends on the CLI formula |
| Ubuntu 24.04 arm64 / amd64 | `dockpipe-desktop_VERSION_ARCH.deb` | Launcher, icons, menu entry; exact-version CLI dependency and native Qt dependencies |
| Windows amd64 | `dockpipe_VERSION_windows_amd64.msi` | CLI, core package, deployed Qt launcher and Start menu shortcut; launcher selected by default |

The macOS app contains the CLI in `Contents/Helpers`, a wrapper in `Contents/MacOS`,
and the complete package store in `Contents/Resources/share/dockpipe`. The wrapper
sets the existing `DOCKPIPE_SYSTEM_ROOT` only when unset. User data retains its
normal location; no engine-specific macOS app knowledge is needed. Launcher
discovery preserves explicit `DOCKPIPE_BIN` and repository development binaries,
then checks its sibling CLI before falling back to PATH.

## Native builds and checks

First run `bash release/packaging/build-platform.sh VERSION` on the target host.
Then run `bash release/packaging/desktop/build-unix.sh VERSION release/artifacts`
on Linux or macOS. Both require CMake and Qt Widgets, Network and Concurrent;
Linux additionally requires OpenGL development files and Debian packaging tools.
The release matrix uses distribution Qt on Ubuntu 24.04 and Qt 6.8.3 on macOS,
with deployment target macOS 13. Windows uses `build-windows.ps1` with Visual
Studio 2022, Qt 6.8.3 and `windeployqt`, then the existing WiX MSI builder.

`smoke.py` runs the installed launcher's `--check-installation` diagnostic outside
the checkout, with isolated user directories and no injected CLI override. It
verifies Qt loads, the selected CLI runs, and the actual window stays running.
On disposable macOS CI runners, `smoke-unix.sh` mounts the DMG, uses Apple's real
Installer, runs a host workflow through the installed CLI, checks the desktop,
and exercises a second installation. It refuses that system-install test outside
GitHub Actions. Linux smoke extracts the DEBs without modifying the host.
Windows smoke installs the real MSI, checks the app/CLI, modifies the launcher
feature and uninstalls. Homebrew separately tests formula/cask installation and
cask removal on both Mac architectures before updating the tap.

Linux desktop support is currently DEB only. Do not put the glibc Qt binary into
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
Dynamic Qt license notices are included in the macOS/Windows payloads.

## Ownership, updates and removal

The direct macOS installer refuses foreign CLI links/files, an Apple Silicon
Homebrew command, or an existing app without its package receipt. Use the cask
when Homebrew already manages DockPipe. Do not mix a cask-managed app and a
package-managed app. Re-running the direct installer updates its managed app
and link. It does not install a daemon or change user data.

For a direct-installer removal, first verify the receipt with
`pkgutil --pkg-info com.dockpipe.desktop` and verify that
`readlink /usr/local/bin/dockpipe` reports
`/Applications/DockPipe.app/Contents/MacOS/dockpipe`. Remove that link and
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

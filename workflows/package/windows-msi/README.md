# Windows MSI (WiX 3)

Builds **`dockpipe_<version>_windows_amd64.msi`**: per-user install to `%LOCALAPPDATA%\dockpipe`, appends that directory to the **user** `PATH` (no admin for silent install), and includes **Dockpipe** by default in release builds, with the full Qt runtime and a per-user Start Menu shortcut. The GUI shell remains an optional MSI feature; the CLI is required.

## Release pipeline (optional)

**Push to `master`:** the GitHub **Release** workflow builds the MSI when the committed marker file **`workflows/package/windows-msi/assets/SHIP_MSI`** is present. This repository now keeps that marker checked in so normal releases publish **`dockpipe_<version>_windows_amd64.msi`** alongside the zip.

**Manual `workflow_dispatch`:** use input **`build_msi`** (default **true**).

## Local producers and packaging

Use the separate CLI, desktop and MSI workflows with an admitted Go toolchain, Qt 6.10.3, Browser SDK and WiX 3.14. The MSI packager accepts produced files; it does not compile the CLI or download a catalog.

```powershell
python workflows/shared/launch.py workflows/build/cli/config.yml -- --version 0.6.0 --target windows/amd64 --worker-host --output bin/dockpipe.exe
python workflows/shared/launch.py workflows/build/desktop/windows.yml -- -Version 0.6.0 -StageDir bin/desktop-stage
python workflows/shared/launch.py workflows/package/windows-msi/config.yml -- -Version 0.6.0 -SourceExe bin/dockpipe.exe -LauncherStageDir bin/desktop-stage -OutDir bin/msi-dist -WixRoot C:/tools/wix314
```

Omit `-LauncherStageDir` for an explicit CLI-only MSI. Both variants include
`dockpipe-worker.exe`, which starts the scheduled worker without a console window.
The packager reads it beside `-SourceExe` by default. Use `-WorkerHost` when it is
staged separately. A catalog store is optional and can be passed as `-PackageStoreDir`.

For a native ARM64 CLI-only MSI, build the CLI and worker host with
`--target windows/arm64 --worker-host`, then pass `-Architecture arm64` to the
same MSI workflow. WiX produces `dockpipe_<version>_windows_arm64.msi`.
The packager checks the PE architecture of the native entry points.
An equal-version MSI in the same install scope must be removed before that
ARM64 package can be installed. The ARM64 GUI still needs its matching pinned
Browser SDK and Qt runtime. The default release pipeline builds the amd64 MSI.

To bundle Windows administrator input, supply `-DesktopInputService`,
`-DesktopInputHelper`, and `-DesktopInputNotices` from the signed native desktop
provider build. This variant installs to Program Files on Windows 11 and
requires administrator permission. Windows verifies the signatures of the CLI,
background worker host, service and helper before packaging. To select this
variant in the release entrypoints, set `DOCKPIPE_WINDOWS_DESKTOP_INPUT_STAGE`
for both build and smoke. The directory must contain `dockpipe.exe`,
`dockpipe-worker.exe`, `dockpipe-desktop-input-service.exe`,
`dockpipe-desktop-input-helper.exe`, and `THIRD-PARTY-NOTICES.txt`.

The bundled service uses the existing protected desktop input configuration so an
upgrade can keep the machine's enrollment. It is registered for automatic
startup and the MSI can install before the machine is enrolled. The system-owned
`DockpipeDesktopInput\worker-registration.json` in ProgramData binds the
ordinary worker and optional sender to the paired Windows user and signed
installed binaries. Those processes start after that user signs in. The
administrator input helper uses its existing authorization. Fresh installation
and upgrade verification must cover protected registration, both processes
returning after sign-in, and retirement of the exact previous task registrations.

When a GUI shell payload is supplied, **Dockpipe** is a separate MSI feature. Silent installs can select `ADDLOCAL=MainFeature` for the CLI alone. Native Windows smoke tests exercise the installed GUI shell, feature removal/restoration and uninstall; PowerShell parameter checks on another host do not qualify installation.

## CI

The **`build-msi`** job in **`.github/workflows/release.yml`** runs on **`windows-latest`** only (WiX does not run on Linux). It downloads **`wix314-binaries.zip`**, expands it under **`RUNNER_TEMP`**, and passes that folder as **`-WixRoot`**. **`build.ps1`** also supports an installed WiX layout with **`bin\candle.exe`**. **`GITHUB_ENV`** is avoided for WiX paths.

That job now also runs **`smoke-test.ps1`** to verify silent per-user install, direct execution of the installed **`dockpipe.exe`**, and silent uninstall before publishing the artifact. Release builds require the GUI shell payload. The smoke test also verifies **`dockpipe-launcher.exe`**, **`Qt6Core.dll`**, **`platforms\qwindows.dll`**, and the Start Menu shortcut.

For the machine-wide bundle, use an elevated Windows 11 session to run the
smoke test with `-ExpectDesktopInput`. It verifies the Program Files payload,
machine PATH, common Start Menu shortcut, and service registration and removal.

CI builds the native GUI shell with Qt 6.10.3 and `windeployqt`. Native smoke
starts the installed Qt app, checks its sibling CLI and the background worker
executable, exercises GUI shell feature removal/restoration, and uninstalls.
Portable ZIPs include the CLI and background worker host. These native checks
must pass on Windows.

# Windows MSI (WiX 3)

Builds **`dockpipe_<version>_windows_amd64.msi`**: per-user install to `%LOCALAPPDATA%\dockpipe`, appends that directory to the **user** `PATH` (no admin for silent install), and includes **Dockpipe Launcher** by default in release builds, with the full Qt runtime and a per-user Start Menu shortcut. The launcher remains an optional MSI feature; the CLI is required.

## Release pipeline (optional)

**Push to `master`:** the GitHub **Release** workflow builds the MSI when the committed marker file **`workflows/package/windows-msi/assets/SHIP_MSI`** is present. This repository now keeps that marker checked in so normal releases publish **`dockpipe_<version>_windows_amd64.msi`** alongside the zip.

**Manual `workflow_dispatch`:** use input **`build_msi`** (default **true**).

## Local producers and packaging

Use the separate CLI, desktop and MSI workflows with an admitted Go toolchain, Qt 6.10.3, Browser SDK and WiX 3.14. The MSI packager accepts produced files; it does not compile the CLI or download a catalog.

```powershell
python workflows/shared/launch.py workflows/build/cli/config.yml -- --version 0.6.0 --target windows/amd64 --output bin/dockpipe.exe
python workflows/shared/launch.py workflows/build/desktop/windows.yml -- -Version 0.6.0 -StageDir bin/desktop-stage
python workflows/shared/launch.py workflows/package/windows-msi/config.yml -- -Version 0.6.0 -SourceExe bin/dockpipe.exe -LauncherStageDir bin/desktop-stage -OutDir bin/msi-dist -WixRoot C:/tools/wix314
```

Omit `-LauncherStageDir` for an explicit CLI-only MSI. A catalog store is optional and must be passed as `-PackageStoreDir`; the default installation contains the standalone CLI.

When a launcher payload is supplied, **Dockpipe Launcher** is a separate MSI feature. Silent installs can select `ADDLOCAL=MainFeature` for the CLI alone. Native Windows smoke tests exercise the installed launcher, feature removal/restoration and uninstall; PowerShell parameter checks on another host do not qualify installation.

## CI

The **`build-msi`** job in **`.github/workflows/release.yml`** runs on **`windows-latest`** only (WiX does not run on Linux). It downloads **`wix314-binaries.zip`**, expands it under **`RUNNER_TEMP`**, and passes that folder as **`-WixRoot`**. **`build.ps1`** also supports an installed WiX layout with **`bin\candle.exe`**. **`GITHUB_ENV`** is avoided for WiX paths.

That job now also runs **`smoke-test.ps1`** to verify silent per-user install, direct execution of the installed **`dockpipe.exe`**, and silent uninstall before publishing the artifact. Release builds require the launcher payload. The smoke test also verifies **`dockpipe-launcher.exe`**, **`Qt6Core.dll`**, **`platforms\qwindows.dll`**, and the Start Menu shortcut.

CI now builds the native launcher with Qt 6.10.3 and `windeployqt`; a missing payload fails instead of silently producing a CLI-only MSI. Native smoke also starts the installed Qt app, checks its sibling CLI, exercises launcher feature removal/restoration, and uninstalls. Portable ZIPs remain CLI-only. These native checks must pass on Windows; PowerShell parameter tests on Linux do not qualify installation.

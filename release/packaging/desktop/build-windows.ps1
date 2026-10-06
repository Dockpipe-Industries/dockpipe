param(
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$StageDir
)
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$build = Join-Path ([IO.Path]::GetTempPath()) ("dockpipe-desktop-" + [guid]::NewGuid().ToString("N"))
if (Test-Path -LiteralPath $StageDir) {
    throw "Desktop stage must be new: $StageDir"
}
$vswhere = Join-Path ${env:ProgramFiles(x86)} "Microsoft Visual Studio/Installer/vswhere.exe"
$visualStudio = & $vswhere -latest -products '*' -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
if (-not $visualStudio) {
    throw "Visual Studio C++ tools are required"
}
$developerShell = Join-Path $visualStudio "Common7/Tools/Launch-VsDevShell.ps1"
& $developerShell -Arch amd64 -HostArch amd64 -SkipAutomaticLocation
if (-not $env:VCToolsInstallDir -or -not $env:VCToolsRedistDir) {
    throw "Visual Studio developer environment is incomplete"
}

try {
    cmake -S (Join-Path $root "src/app/tooling/dockpipe-launcher") -B $build -G "NMake Makefiles" `
        -DCMAKE_BUILD_TYPE=Release "-DDOCKPIPE_RELEASE_VERSION=$Version"
    if ($LASTEXITCODE -ne 0) {
        throw "Desktop configure failed"
    }
    cmake --build $build
    if ($LASTEXITCODE -ne 0) {
        throw "Desktop build failed"
    }
    New-Item -ItemType Directory -Path $StageDir | Out-Null
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "licenses") -Destination $StageDir -Recurse
    Copy-Item -LiteralPath (Join-Path $root "LICENSE") -Destination (Join-Path $StageDir "licenses/DockPipe-LICENSE.txt")
    $launcher = Join-Path $StageDir "dockpipe-launcher.exe"
    Copy-Item -LiteralPath (Join-Path $build "dockpipe-launcher.exe") -Destination $launcher
    windeployqt --release --compiler-runtime --dir $StageDir $launcher
    if ($LASTEXITCODE -ne 0) {
        throw "Qt deployment failed"
    }
    # A per-user MSI cannot rely on running a machine-wide VC redistributable
    # installer. Ship Microsoft's redistributable CRT DLLs beside the application.
    $crtRoot = Join-Path $env:VCToolsRedistDir "x64"
    $crt = Get-ChildItem -LiteralPath $crtRoot -Directory -Filter "Microsoft.VC*.CRT" | Sort-Object Name | Select-Object -Last 1
    if (-not $crt) {
        throw "Visual C++ redistributable CRT directory is missing"
    }
    Get-ChildItem -LiteralPath $crt.FullName -Filter "*.dll" | Copy-Item -Destination $StageDir
    foreach ($required in @("Qt6Core.dll", "Qt6Gui.dll", "Qt6Widgets.dll", "platforms/qwindows.dll", "vcruntime140.dll", "msvcp140.dll")) {
        if (-not (Test-Path -LiteralPath (Join-Path $StageDir $required))) {
            throw "Incomplete desktop runtime: $required"
        }
    }
} finally {
    if (Test-Path -LiteralPath $build) {
        Remove-Item -LiteralPath $build -Recurse -Force
    }
}

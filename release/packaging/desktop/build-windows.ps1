param(
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$StageDir,
    [switch]$EnableQualification
)
$ErrorActionPreference = "Stop"

function Assert-PublicDesktopRuntime {
    param([Parameter(Mandatory = $true)][string]$RuntimeDirectory)
    # The MSI harvests the whole stage. Fail before harvesting private build state.
    $privateExtensions = @(".pdb", ".obj", ".ilk", ".exp", ".lib", ".pch", ".iobj", ".ipdb", ".idb")
    $privateFiles = @(Get-ChildItem -LiteralPath $RuntimeDirectory -Recurse -File | Where-Object {
        $privateExtensions -contains $_.Extension.ToLowerInvariant()
    })
    if ($privateFiles.Count -ne 0) {
        throw "Desktop runtime contains private build files: $($privateFiles.Name -join ', ')"
    }
}

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
    $qualification = if ($EnableQualification) { "ON" } else { "OFF" }
    cmake -S (Join-Path $root "src/app/tooling/dockpipe-launcher") -B $build -G "NMake Makefiles" `
        -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTING=OFF "-DDOCKPIPE_ENABLE_QUALIFICATION=$qualification" "-DDOCKPIPE_RELEASE_VERSION=$Version"
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
    Copy-Item -LiteralPath (Join-Path $build "dockpipe-syntax.rcc") -Destination $StageDir
    Copy-Item -LiteralPath (Join-Path $build "syntax-licenses") -Destination $StageDir -Recurse
    Copy-Item -LiteralPath (Join-Path $build "dockpipe-launcher-ui.dll") -Destination $StageDir
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
    Assert-PublicDesktopRuntime -RuntimeDirectory $StageDir
    foreach ($required in @(
        "Qt6Core.dll", "Qt6Gui.dll", "Qt6Widgets.dll", "Qt6OpenGL.dll",
        "Qt6WebEngineCore.dll", "Qt6WebEngineWidgets.dll", "QtWebEngineProcess.exe",
        "resources/icudtl.dat", "resources/qtwebengine_resources.pak", "translations/qtwebengine_locales/en-US.pak",
        "resources/qtwebengine_resources_100p.pak", "resources/qtwebengine_resources_200p.pak",
        "platforms/qwindows.dll", "vcruntime140.dll", "msvcp140.dll"
    )) {
        if (-not (Test-Path -LiteralPath (Join-Path $StageDir $required))) {
            throw "Incomplete desktop runtime: $required"
        }
    }
} finally {
    if (Test-Path -LiteralPath $build) {
        Remove-Item -LiteralPath $build -Recurse -Force
    }
}

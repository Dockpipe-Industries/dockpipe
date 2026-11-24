# Validates the Dockpipe MSI install/uninstall path on Windows.
param(
    [Parameter(Mandatory = $true)][string]$MsiPath,
    [switch]$ExpectLauncher,
    [switch]$ExpectDesktopInput
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $MsiPath -PathType Leaf)) {
    throw "MSI not found: $MsiPath"
}

$installRoot = $env:LOCALAPPDATA
$startMenuRoot = $env:APPDATA
$pathScope = "User"
if ($ExpectDesktopInput) {
    $installRoot = $env:ProgramW6432
    if (-not $installRoot) {
        $installRoot = $env:ProgramFiles
    }
    $startMenuRoot = $env:ProgramData
    $pathScope = "Machine"
}

$installDir = Join-Path $installRoot "dockpipe"
$exePath = Join-Path $installDir "dockpipe.exe"
$workerPath = Join-Path $installDir "dockpipe-worker.exe"
$packageDir = Join-Path $installDir "packages"
$launcherPath = Join-Path $installDir "dockpipe-launcher.exe"
$launcherPlatformPlugin = Join-Path $installDir "platforms/qwindows.dll"
$launcherCoreDll = Join-Path $installDir "Qt6Core.dll"
$terminalHostPath = Join-Path $installDir "dockpipe-terminal-host.exe"
$startMenuShortcut = Join-Path $startMenuRoot "Microsoft/Windows/Start Menu/Programs/Dockpipe/Dockpipe.lnk"
$logPath = Join-Path $env:TEMP "dockpipe-msi-smoke.log"
$launcherPaths = @($launcherPath, $launcherPlatformPlugin, $launcherCoreDll, $terminalHostPath, $startMenuShortcut)
$desktopInputPaths = @()
if ($ExpectDesktopInput) {
    $desktopInputDir = Join-Path $installRoot "Dockpipe Desktop Input"
    $desktopInputPaths = @(
        Join-Path $desktopInputDir "dockpipe.exe"
        Join-Path $desktopInputDir "dockpipe-desktop-input-service.exe"
        Join-Path $desktopInputDir "dockpipe-desktop-input-helper.exe"
        Join-Path $desktopInputDir "THIRD-PARTY-NOTICES.txt"
    )
}

function Get-InstalledPathValue {
    param([EnvironmentVariableTarget]$Scope)

    return [Environment]::GetEnvironmentVariable("Path", $Scope)
}

function Get-DesktopInputService {
    Get-CimInstance -ClassName Win32_Service -Filter "Name = 'DockpipeDesktopInput'"
}

function Get-DirectorySnapshot {
    param([string]$PathValue)

    if (-not (Test-Path -LiteralPath $PathValue)) {
        return @()
    }

    $root = [IO.Path]::GetFullPath($PathValue)
    $items = Get-ChildItem -LiteralPath $PathValue -Recurse -Force
    return @($items | ForEach-Object {
        [IO.Path]::GetRelativePath($root, $_.FullName).Replace('\', '/')
    } | Sort-Object -Unique)
}

function Test-PathEntryPresent {
    param([string]$PathValue, [string]$Entry)

    $normalizedEntry = $Entry.TrimEnd('\', '/')
    $parts = @($PathValue -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    foreach ($part in $parts) {
        if ($part.TrimEnd('\', '/') -ieq $normalizedEntry) {
            return $true
        }
    }
    return $false
}

function Invoke-SmokeMsi {
    param([string]$Action, [string[]]$Options = @())

    $arguments = @($Action, ('"{0}"' -f $MsiPath), "/qn", "/norestart") + $Options
    $process = Start-Process msiexec.exe -ArgumentList $arguments -Wait -PassThru
    return $process.ExitCode
}

function Assert-WorkerSubsystem {
    param([string]$Path)

    $worker = [IO.BinaryReader]::new([IO.File]::OpenRead($Path))
    try {
        $worker.BaseStream.Position = 0x3c
        $peHeader = $worker.ReadInt32()
        $worker.BaseStream.Position = $peHeader
        if ($worker.ReadUInt32() -ne 0x4550) {
            throw "Installed background worker is not a Windows executable."
        }

        $worker.BaseStream.Position = $peHeader + 24 + 68
        if ($worker.ReadUInt16() -ne 2) {
            throw "Installed background worker must use the Windows GUI subsystem."
        }
    } finally {
        $worker.Dispose()
    }
}

$trackedPaths = @($exePath, $workerPath) + $launcherPaths + $desktopInputPaths
foreach ($path in $trackedPaths) {
    if (Test-Path -LiteralPath $path) {
        throw "MSI smoke test requires a clean install of the selected scope. Existing path: $path"
    }
}
if ($ExpectDesktopInput -and (Get-DesktopInputService)) {
    throw "MSI smoke test requires DockpipeDesktopInput to be absent before install."
}

$baselinePath = Get-InstalledPathValue -Scope $pathScope
$baselineInstallDirInPath = Test-PathEntryPresent -PathValue $baselinePath -Entry $installDir
$baselinePackageDir = Test-Path -LiteralPath $packageDir -PathType Container
$baselinePackageSnapshot = @(Get-DirectorySnapshot -PathValue $packageDir)
$installSucceeded = $false
$smokeFailure = $null

try {
    Write-Host "Installing $MsiPath ($pathScope scope)"
    $install = Invoke-SmokeMsi -Action "/i" -Options @("/l*v", ('"{0}"' -f $logPath))
    if ($install -notin @(0, 3010)) {
        throw "MSI install failed with exit code $install"
    }
    $installSucceeded = $true

    $expectedPaths = @($exePath, $workerPath) + $desktopInputPaths
    if ($ExpectLauncher) {
        $expectedPaths += $launcherPaths
    }
    foreach ($path in $expectedPaths) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "Installed MSI payload not found at $path"
        }
    }
    Assert-WorkerSubsystem -Path $workerPath

    if ($ExpectDesktopInput) {
        $service = Get-DesktopInputService
        if (-not $service -or $service.StartMode -ne "Auto" -or $service.StartName -ne "LocalSystem") {
            throw "DockpipeDesktopInput must be registered as an automatic LocalSystem service."
        }
    }

    if ($ExpectLauncher) {
        python (Join-Path $PSScriptRoot "../../../../build/desktop/assets/scripts/smoke.py") $launcherPath $exePath --inventory-system-root $installDir
        if ($LASTEXITCODE -ne 0) {
            throw "Installed desktop runtime smoke failed"
        }

        $cliOnly = Invoke-SmokeMsi -Action "/i" -Options @("ADDLOCAL=MainFeature", "REMOVE=LauncherFeature")
        if ($cliOnly -notin @(0, 3010) -or (Test-Path -LiteralPath $launcherPath) -or -not (Test-Path -LiteralPath $exePath)) {
            throw "Switching MSI to CLI-only failed"
        }

        $restoreDesktop = Invoke-SmokeMsi -Action "/i" -Options @("ADDLOCAL=MainFeature,LauncherFeature")
        if ($restoreDesktop -notin @(0, 3010) -or -not (Test-Path -LiteralPath $launcherPath) -or
            -not (Test-Path -LiteralPath $terminalHostPath -PathType Leaf)) {
            throw "Restoring MSI desktop feature failed"
        }
    }

    $installedPath = Get-InstalledPathValue -Scope $pathScope
    if (-not (Test-PathEntryPresent -PathValue $installedPath -Entry $installDir)) {
        throw "$pathScope PATH does not contain $installDir after install"
    }

    Write-Host "Running installed executable"
    & $exePath '--help' | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Installed dockpipe.exe --help exited $LASTEXITCODE"
    }
} catch {
    $smokeFailure = $_
    throw
} finally {
    try {
        Write-Host "Uninstalling MSI"
        $uninstall = Invoke-SmokeMsi -Action "/x"
        $alreadyAbsent = -not $installSucceeded -and $uninstall -in @(1605, 1614)
        if ($uninstall -notin @(0, 3010) -and -not $alreadyAbsent) {
            throw "MSI uninstall failed with exit code $uninstall"
        }

        foreach ($path in $trackedPaths) {
            if (Test-Path -LiteralPath $path) {
                throw "Installed MSI payload still exists after uninstall: $path"
            }
        }
        if ($ExpectDesktopInput -and (Get-DesktopInputService)) {
            throw "DockpipeDesktopInput is still registered after MSI uninstall."
        }

        $currentPackageSnapshot = @(Get-DirectorySnapshot -PathValue $packageDir)
        if ((@($currentPackageSnapshot) -join "`n") -ne (@($baselinePackageSnapshot) -join "`n")) {
            throw "Installed dockpipe package contents did not return to baseline after uninstall: $packageDir"
        }
        if ((Test-Path -LiteralPath $packageDir -PathType Container) -ne $baselinePackageDir) {
            throw "Installed dockpipe package directory did not return to baseline after uninstall: $packageDir"
        }

        $pathAfter = Get-InstalledPathValue -Scope $pathScope
        if ((Test-PathEntryPresent -PathValue $pathAfter -Entry $installDir) -ne $baselineInstallDirInPath) {
            throw "$pathScope PATH entry for $installDir did not return to baseline after uninstall"
        }
    } catch {
        if (-not $smokeFailure) {
            throw
        }
        Write-Warning "MSI cleanup after a smoke test failure also failed. $($_.Exception.Message)"
    }
}

Write-Host "MSI smoke test passed"

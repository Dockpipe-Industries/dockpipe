#Requires -Version 5.1
<#
.SYNOPSIS
  Download and install dockpipe on Windows (MSI preferred, else zip).

.DESCRIPTION
  - Fetches the latest GitHub release (or a specific -Version).
  - Requires and verifies SHA256 using SHA256SUMS.txt from the same release.
  - Installs dockpipe.exe and, when the MSI includes it for that release, dockpipe-launcher.exe.
  - MSI: per-user WiX install to %LOCALAPPDATA%\dockpipe, PATH updated. Zip fallback: %LOCALAPPDATA%\Programs\dockpipe.

  After install, optionally configures WSL for DOCKPIPE_USE_WSL_BRIDGE=1 (minimal Alpine + latest Linux dockpipe from GitHub). Use -SkipWSLSetup to skip. May prompt for Administrator (WSL) or require a reboot.

.EXAMPLE
  iwr -useb https://raw.githubusercontent.com/Dockpipe-Industries/dockpipe/master/release/packaging/windows/install.ps1 | iex

.EXAMPLE
  .\install.ps1 -Version 0.6.0
#>
param(
    [string]$Version = "",
    [string]$Repo = "Dockpipe-Industries/dockpipe",
    [switch]$SkipWSLSetup
)

$ErrorActionPreference = "Stop"

function Invoke-DockpipeWslSetup {
    param([Parameter(Mandatory = $true)][string]$DockpipeExe)
    if ($SkipWSLSetup) {
        Write-Host "Skipping WSL setup (-SkipWSLSetup). For bridge users later: dockpipe windows setup --bootstrap-wsl --distro Alpine --non-interactive --install-dockpipe"
        return
    }
    if (-not (Test-Path -LiteralPath $DockpipeExe)) {
        Write-Warning "dockpipe.exe not found at $DockpipeExe — skipping WSL setup."
        return
    }
    Write-Host "Configuring WSL for optional DOCKPIPE_USE_WSL_BRIDGE (Alpine + dockpipe from GitHub). Administrator / reboot may be required..."
    & $DockpipeExe @('windows', 'setup', '--bootstrap-wsl', '--distro', 'Alpine', '--non-interactive', '--install-dockpipe')
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "WSL setup exited $LASTEXITCODE — install Docker Desktop + Git for Windows for native mode, or run manually: dockpipe windows doctor"
    }
}

function Install-VerifiedReleaseAsset {
    param(
        [Parameter(Mandatory = $true)]$Asset,
        [Parameter(Mandatory = $true)][string]$ExpectedHash,
        [Parameter(Mandatory = $true)][string]$Destination
    )
    if ($ExpectedHash -notmatch '^[a-fA-F0-9]{64}$') {
        throw "Missing or invalid release checksum for $($Asset.name)"
    }
    $directory = Split-Path -Parent $Destination
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
    # Stage on the destination volume so publishing is a rename, never a copy
    # over a live package. Only this invocation's temporary file is removed.
    $temporary = Join-Path $directory (".dockpipe-download-" + [Guid]::NewGuid().ToString("N"))
    try {
        Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $temporary -UseBasicParsing
        $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $temporary).Hash
        if ($actualHash -ne $ExpectedHash) {
            throw "SHA256 mismatch for $($Asset.name)."
        }
        if (Test-Path -LiteralPath $Destination) {
            [System.IO.File]::Replace($temporary, $Destination, [NullString]::Value)
        } else {
            [System.IO.File]::Move($temporary, $Destination)
        }
    } finally {
        if (Test-Path -LiteralPath $temporary) {
            Remove-Item -LiteralPath $temporary -Force
        }
    }
}

function Get-Release {
    param([string]$Ver)
    $base = "https://api.github.com/repos/$Repo/releases"
    if ($Ver) {
        $tag = if ($Ver.StartsWith("v")) { $Ver } else { "v$Ver" }
        return Invoke-RestMethod -Uri "$base/tags/$tag" -Headers @{ "User-Agent" = "dockpipe-install" }
    }
    return Invoke-RestMethod -Uri "$base/latest" -Headers @{ "User-Agent" = "dockpipe-install" }
}

function Get-Sha256Map {
    param($Release)
    $sumAsset = $Release.assets | Where-Object { $_.name -eq "SHA256SUMS.txt" } | Select-Object -First 1
    if (-not $sumAsset) { throw "Release has no SHA256SUMS.txt" }
    $tmp = [System.IO.Path]::GetTempFileName()
    Invoke-WebRequest -Uri $sumAsset.browser_download_url -OutFile $tmp -UseBasicParsing
    $map = @{}
    Get-Content $tmp | ForEach-Object {
        # GNU sha256sum: "hash  file" or "hash *file"
        if ($_ -match '^\s*([a-fA-F0-9]{64})\s+\*?\s*(.+)$') {
            $name = $matches[2].Trim()
            $map[$name] = $matches[1].ToLowerInvariant()
        }
    }
    Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    $map
}

$rel = Get-Release -Ver $Version
$verTag = $rel.tag_name.TrimStart("v")
$sums = Get-Sha256Map -Release $rel

$msi = $rel.assets | Where-Object { $_.name -match "dockpipe_.*_windows_amd64\.msi$" } | Select-Object -First 1
$zip = $rel.assets | Where-Object { $_.name -match "dockpipe_.*_windows_amd64\.zip$" } | Select-Object -First 1
$core = $rel.assets | Where-Object { $_.name -match "^dockpipe-core-.*\.tar\.gz$" } | Select-Object -First 1

foreach ($asset in @($msi, $zip, $core)) {
    if ($asset -and -not $sums.ContainsKey($asset.name)) {
        throw "Missing release checksum for $($asset.name)"
    }
}

if ($msi) {
    $dl = Join-Path $env:TEMP $msi.name
    Write-Host "Downloading $($msi.name) ..."
    Invoke-WebRequest -Uri $msi.browser_download_url -OutFile $dl -UseBasicParsing
    if ($sums.ContainsKey($msi.name)) {
        $h = (Get-FileHash -Algorithm SHA256 -LiteralPath $dl).Hash.ToLowerInvariant()
        if ($h -ne $sums[$msi.name]) {
            throw "SHA256 mismatch for $($msi.name). Expected $($sums[$msi.name]), got $h"
        }
    }
    Write-Host "Installing MSI (elevates if needed) ..."
    $p = Start-Process msiexec.exe -ArgumentList @("/i", "`"$dl`"", "/qn", "/norestart") -Wait -PassThru
    if ($p.ExitCode -ne 0 -and $p.ExitCode -ne 3010) {
        throw "msiexec failed with exit code $($p.ExitCode)"
    }
    $msiExe = Join-Path $env:LOCALAPPDATA "dockpipe\dockpipe.exe"
    Invoke-DockpipeWslSetup -DockpipeExe $msiExe
    Write-Host "Installed dockpipe $verTag. Open a new terminal for PATH changes, then: dockpipe --help"
    exit 0
}

if (-not $zip) {
    throw "No windows_amd64.msi or .zip found in release $($rel.tag_name)."
}

$zipPath = Join-Path $env:TEMP $zip.name
Write-Host "Downloading $($zip.name) (no MSI in this release) ..."
Invoke-WebRequest -Uri $zip.browser_download_url -OutFile $zipPath -UseBasicParsing
if ($sums.ContainsKey($zip.name)) {
    $h = (Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath).Hash.ToLowerInvariant()
    if ($h -ne $sums[$zip.name]) {
        throw "SHA256 mismatch for $($zip.name)."
    }
}

$dest = Join-Path $env:LOCALAPPDATA "Programs\dockpipe"
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Expand-Archive -LiteralPath $zipPath -DestinationPath $dest -Force
$exe = Join-Path $dest "dockpipe.exe"
if (-not (Test-Path $exe)) {
    throw "Expected dockpipe.exe under $dest"
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$dest*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
    $env:Path = "$env:Path;$dest"
}
if ($core) {
    $coreDir = Join-Path $env:LOCALAPPDATA "dockpipe\packages\core"
    New-Item -ItemType Directory -Force -Path $coreDir | Out-Null
    $corePath = Join-Path $coreDir $core.name
    Write-Host "Downloading $($core.name) ..."
    Install-VerifiedReleaseAsset -Asset $core -ExpectedHash $sums[$core.name] -Destination $corePath
}
Invoke-DockpipeWslSetup -DockpipeExe $exe
Write-Host "Installed dockpipe $verTag to $dest (user PATH updated). Open a new terminal, then: dockpipe --help"

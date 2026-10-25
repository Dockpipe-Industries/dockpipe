# Install a CLI produced by the separate Dockpipe build workflow. Desktop
# installation belongs to the MSI packaging workflow and its verified payload.
param(
    [Parameter(Mandatory = $true)][string]$SourceExe,
    [string]$InstallRoot = (Join-Path $env:LOCALAPPDATA "dockpipe"),
    [switch]$UpdatePath
)

$ErrorActionPreference = "Stop"
$source = Get-Item -LiteralPath $SourceExe
if ($source.PSIsContainer -or ($source.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw "SourceExe must be a regular built executable"
}
$stream = [IO.File]::OpenRead($source.FullName)
try {
    if ($stream.ReadByte() -ne 0x4D -or $stream.ReadByte() -ne 0x5A) {
        throw "SourceExe is not a Windows executable"
    }
}
finally {
    $stream.Dispose()
}

$before = (Get-FileHash -LiteralPath $source.FullName -Algorithm SHA256).Hash
New-Item -ItemType Directory -Force -Path $InstallRoot | Out-Null
$destination = Join-Path $InstallRoot "dockpipe.exe"
$temporary = Join-Path $InstallRoot (".dockpipe-install-" + [Guid]::NewGuid().ToString("N") + ".tmp")
try {
    Copy-Item -LiteralPath $source.FullName -Destination $temporary
    $copied = (Get-FileHash -LiteralPath $temporary -Algorithm SHA256).Hash
    $after = (Get-FileHash -LiteralPath $source.FullName -Algorithm SHA256).Hash
    if ($before -ne $copied -or $before -ne $after) {
        throw "Built CLI changed during installation"
    }
    Move-Item -LiteralPath $temporary -Destination $destination -Force
}
finally {
    if (Test-Path -LiteralPath $temporary) {
        Remove-Item -LiteralPath $temporary
    }
}

if ($UpdatePath) {
    $current = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @($current -split ";" | Where-Object { $_ })
    $selected = [IO.Path]::GetFullPath($InstallRoot).TrimEnd("\")
    $present = @($entries | Where-Object { $_.TrimEnd("\") -ieq $selected }).Count -gt 0
    if (-not $present) {
        [Environment]::SetEnvironmentVariable("Path", (($entries + $selected) -join ";"), "User")
    }
}
Write-Host "Installed built CLI at $destination"

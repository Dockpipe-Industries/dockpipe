# Build dockpipe MSI (WiX 3.x). Run on Windows.
# Usage: .\build.ps1 -Version 0.6.0 -SourceExe C:\path\dockpipe.exe -OutDir C:\out [-LauncherStageDir C:\path\launcher-stage]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$SourceExe,
    [Parameter(Mandatory = $true)][string]$OutDir,
    [ValidateSet("amd64", "arm64")][string]$Architecture = "amd64",
    [Parameter(Mandatory = $false)][string]$WorkerHost = "",
    [Parameter(Mandatory = $false)][string]$PackageStoreDir = "",
    [Parameter(Mandatory = $false)][string]$LauncherStageDir = "",
    [Parameter(Mandatory = $false)][string]$LauncherExe = "",
    [Parameter(Mandatory = $false)][string]$DesktopInputService = "",
    [Parameter(Mandatory = $false)][string]$DesktopInputHelper = "",
    [Parameter(Mandatory = $false)][string]$DesktopInputNotices = "",
    # Optional: WiX root (extract folder: root candle.exe from wix314-binaries.zip, or parent of bin\candle.exe). Prefer passing this in CI — GITHUB_ENV can mangle Windows paths.
    [Parameter(Mandatory = $false)][string]$WixRoot = ""
)

function Assert-MsiBinaryArchitecture {
    param([string]$Path, [string]$Architecture)

    $expectedMachine = if ($Architecture -eq "arm64") { 0xAA64 } else { 0x8664 }
    $file = [IO.File]::OpenRead($Path)
    try {
        $reader = [IO.BinaryReader]::new($file, [Text.Encoding]::ASCII, $true)
        try {
            if ($file.Length -lt 64 -or $reader.ReadUInt16() -ne 0x5A4D) {
                throw "Expected a Windows executable or DLL: $Path"
            }

            $file.Position = 0x3C
            $header = $reader.ReadUInt32()
            if ($header -lt 64 -or $header -gt ($file.Length - 6)) {
                throw "Invalid PE header offset: $Path"
            }

            $file.Position = $header
            if ($reader.ReadUInt32() -ne 0x00004550) {
                throw "Invalid PE signature: $Path"
            }
            if ($reader.ReadUInt16() -ne $expectedMachine) {
                throw "Binary does not match the $Architecture MSI architecture: $Path"
            }
        } finally {
            $reader.Dispose()
        }
    } finally {
        $file.Dispose()
    }
}

function Write-MsiLicenseRtf {
    param([string]$Source, [string]$Destination)

    $license = [IO.File]::ReadAllText($Source)
    if ([string]::IsNullOrWhiteSpace($license)) {
        throw "MSI license text is empty: $Source"
    }

    $escaped = $license.Replace('\', '\\').Replace('{', '\{').Replace('}', '\}')
    $escaped = [regex]::Replace($escaped, '[^\x09\x0A\x0D\x20-\x7E]', {
        param($match)

        $codeUnit = [int][char]$match.Value[0]
        if ($codeUnit -gt 32767) {
            $codeUnit -= 65536
        }
        "\u$codeUnit" + '?'
    })
    $escaped = $escaped.Replace("`t", '\tab ')
    $escaped = [regex]::Replace($escaped, '\r\n|\r|\n', "\par`r`n")

    $header = '{\rtf1\ansi\deff0{\fonttbl{\f0 Segoe UI;}}\f0\fs20 '
    [IO.File]::WriteAllText($Destination, $header + $escaped + '}', [Text.Encoding]::ASCII)
}

$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $SourceExe)) {
    throw "SourceExe not found: $SourceExe"
}
if ($PackageStoreDir -and -not (Test-Path -LiteralPath $PackageStoreDir)) {
    throw "PackageStoreDir not found: $PackageStoreDir"
}
if ($LauncherStageDir -and -not (Test-Path -LiteralPath $LauncherStageDir)) {
    throw "LauncherStageDir not found: $LauncherStageDir"
}
if ($LauncherExe -and -not (Test-Path -LiteralPath $LauncherExe)) {
    throw "LauncherExe not found: $LauncherExe"
}

# WiX requires Product/@Version as X.Y.Z.W
$fourPart = if ($Version -match '^\d+\.\d+\.\d+$') { "$Version.0" } elseif ($Version -match '^\d+\.\d+\.\d+\.\d+$') { $Version } else {
    throw "Version must be semver like 0.6.0 or 0.6.0.0, got: $Version"
}

$rawWix = if ($WixRoot) { $WixRoot } else { $env:WIX }
# CI may set WIX with forward slashes; normalize for Windows APIs.
if (-not $rawWix) {
    throw "WIX environment variable must point to WiX Toolset v3 root (candle.exe). Install from https://github.com/wixtoolset/wix3/releases"
}
$wixRoot = [IO.Path]::GetFullPath($rawWix.Replace('/', '\'))
# Official wix314-binaries.zip lays tools at the extract root (candle.exe, light.exe). Installed WiX often uses bin\candle.exe.
$binCandle = Join-Path $wixRoot "bin\candle.exe"
$rootCandle = Join-Path $wixRoot "candle.exe"
if (Test-Path -LiteralPath $binCandle) {
    $candle = $binCandle
    $light = Join-Path $wixRoot "bin\light.exe"
} elseif (Test-Path -LiteralPath $rootCandle) {
    $candle = $rootCandle
    $light = Join-Path $wixRoot "light.exe"
} else {
    throw "WiX root must contain bin\candle.exe (typical install) or candle.exe (wix314-binaries.zip layout). Got: $rawWix"
}
if (-not (Test-Path -LiteralPath $light)) {
    throw "light.exe not found next to candle (expected bin\light.exe or light.exe under WiX root). Root: $wixRoot"
}
$heat = Join-Path $wixRoot "heat.exe"
if (-not (Test-Path -LiteralPath $heat)) {
    throw "heat.exe not found under WiX root: $wixRoot"
}
$wxs = Join-Path $PSScriptRoot "../dockpipe.wxs"
$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "../../../../.."))
$Architecture = $Architecture.ToLowerInvariant()
$wixArchitecture = if ($Architecture -eq "arm64") { "arm64" } else { "x64" }
$objDir = Join-Path $OutDir "wixobj/$Architecture"
New-Item -ItemType Directory -Force -Path $objDir | Out-Null
$msiName = "dockpipe_${Version}_windows_${Architecture}.msi"
$msiPath = Join-Path $OutDir $msiName

$srcAbs = (Resolve-Path -LiteralPath $SourceExe).Path
if (-not $WorkerHost) {
    $WorkerHost = Join-Path (Split-Path -Parent $srcAbs) "dockpipe-worker.exe"
}
if (-not (Test-Path -LiteralPath $WorkerHost -PathType Leaf)) {
    throw "The Windows background worker executable is missing: $WorkerHost"
}
$workerHostAbs = (Resolve-Path -LiteralPath $WorkerHost).Path
Assert-MsiBinaryArchitecture -Path $srcAbs -Architecture $Architecture
Assert-MsiBinaryArchitecture -Path $workerHostAbs -Architecture $Architecture

$desktopInputEnabled = "0"
$installScope = "perUser"
$installRoot = "LocalAppDataFolder"
$registryRoot = "HKCU"
$systemPath = "no"
$upgradeCode = "B7E3F2A1-4C5D-6E7F-8091-A2B3C4D5E6F7"
$upgradeSchedule = "afterInstallValidate"
$cliComponentGuid = "6E07E906-C5C2-4D49-8FB2-6DAFB36D3AB5"
$workerComponentGuid = "45097E93-766D-46FD-B419-410C11087D57"
$packageComponentGuid = "A13DC67E-188B-534F-9FEC-4C2864E3B23D"
$launcherComponentGuid = "D25B9DAA-215E-4F19-B1AA-3E5A43A6766B"
$shortcutComponentGuid = "8C8E1798-E040-41F3-A26F-4B9CA5C0A293"
if ($DesktopInputService -or $DesktopInputHelper -or $DesktopInputNotices) {
    if (-not $DesktopInputService -or -not $DesktopInputHelper -or -not $DesktopInputNotices) {
        throw "The Windows desktop input bundle requires the service, helper and notices."
    }

    $DesktopInputService = (Resolve-Path -LiteralPath $DesktopInputService).Path
    $DesktopInputHelper = (Resolve-Path -LiteralPath $DesktopInputHelper).Path
    $DesktopInputNotices = (Resolve-Path -LiteralPath $DesktopInputNotices).Path
    foreach ($binary in @($DesktopInputService, $DesktopInputHelper)) {
        Assert-MsiBinaryArchitecture -Path $binary -Architecture $Architecture
    }
    foreach ($binary in @($srcAbs, $workerHostAbs, $DesktopInputService, $DesktopInputHelper)) {
        if ((Get-AuthenticodeSignature -LiteralPath $binary).Status -ne 'Valid') {
            throw "The desktop input bundle requires a valid Authenticode signature: $binary"
        }
    }

    $desktopInputEnabled = "1"
    $installScope = "perMachine"
    $installRoot = "ProgramFiles64Folder"
    $registryRoot = "HKLM"
    $systemPath = "yes"
    $upgradeCode = "848B2116-5B07-4D41-86DE-4DCFA445D5AD"
    $upgradeSchedule = "afterInstallInitialize"
    $cliComponentGuid = "FB9ABBA3-826C-4272-B49A-65EE019DC92F"
    $workerComponentGuid = "B71B8477-A973-4EC9-A270-141027DC1870"
    $packageComponentGuid = "2ACF0B1A-E98E-47BD-B489-3DB5465C08BC"
    $launcherComponentGuid = "AADEC247-EFE4-4CBC-B7ED-41F5A238D735"
    $shortcutComponentGuid = "A0DCCA8D-20ED-4A5F-A21F-861CE57C9267"
}
$packageStageAbs = ""
$launcherStageAbs = ""
$tempLauncherStageDir = ""

if ($PackageStoreDir) {
    $packageStage = Join-Path $objDir "optional-package-stage"
    & python3 (Join-Path $repository "workflows/release/metadata/assets/scripts/release-artifacts.py") stage-store $PackageStoreDir --destination $packageStage
    if ($LASTEXITCODE -ne 0) { throw "Optional package store validation failed" }
    $packages = Join-Path $packageStage "packages"
    if (Test-Path -LiteralPath $packages) {
        $packageStageAbs = (Resolve-Path -LiteralPath $packages).Path
    }
}

if ($LauncherStageDir) {
    $launcherStageAbs = (Resolve-Path -LiteralPath $LauncherStageDir).Path
} elseif ($LauncherExe) {
    # Back-compat path: stage the single launcher exe so callers still produce a valid payload tree.
    $tempLauncherStageDir = Join-Path $objDir "launcher-stage"
    New-Item -ItemType Directory -Force -Path $tempLauncherStageDir | Out-Null
    Copy-Item -LiteralPath $LauncherExe -Destination (Join-Path $tempLauncherStageDir "dockpipe-launcher.exe") -Force
    $launcherStageAbs = (Resolve-Path -LiteralPath $tempLauncherStageDir).Path
}

$launcherEnabled = if ($launcherStageAbs) { "1" } else { "0" }
$packagesEnabled = if ($packageStageAbs) { "1" } else { "0" }
$packageHarvestWxs = Join-Path $objDir "optional-package-payload.wxs"
$harvestWxs = Join-Path $objDir "launcher-payload.wxs"
$candleInputs = @($wxs)
$lightOptions = @()

if ($packageStageAbs) {
    & $heat dir $packageStageAbs `
        -nologo `
        -gg `
        -scom `
        -sreg `
        -sfrag `
        -srd `
        -dr PACKAGESDIR `
        -cg PackagePayloadComponents `
        -var var.DockpipePackageStoreDir `
        -out $packageHarvestWxs
    if ($LASTEXITCODE -ne 0) { throw "heat failed" }
    $candleInputs += $packageHarvestWxs
}

if ($launcherStageAbs) {
    $launcherExeFromStage = Join-Path $launcherStageAbs "dockpipe-launcher.exe"
    if (-not (Test-Path -LiteralPath $launcherExeFromStage)) {
        throw "LauncherStageDir must contain dockpipe-launcher.exe at its root. Got: $launcherStageAbs"
    }
    Assert-MsiBinaryArchitecture -Path $launcherExeFromStage -Architecture $Architecture
    if ($Architecture -eq "arm64") {
        Get-ChildItem -LiteralPath $launcherStageAbs -Recurse -File |
            Where-Object { $_.Extension -in ".exe", ".dll" } |
            ForEach-Object { Assert-MsiBinaryArchitecture -Path $_.FullName -Architecture $Architecture }
    }

    & $heat dir $launcherStageAbs `
        -nologo `
        -gg `
        -scom `
        -sreg `
        -sfrag `
        -srd `
        -dr INSTALLFOLDER `
        -cg LauncherPayloadComponents `
        -var var.DockpipeLauncherStageDir `
        -out $harvestWxs
    if ($LASTEXITCODE -ne 0) { throw "heat failed" }
    $candleInputs += $harvestWxs

    $licenseRtf = Join-Path (Resolve-Path -LiteralPath $objDir).Path "license.rtf"
    Write-MsiLicenseRtf -Source (Join-Path $repository "LICENSE") -Destination $licenseRtf
    $lightOptions += @("-ext", "WixUIExtension", "-dWixUILicenseRtf=$licenseRtf")
}

& $candle -nologo -arch $wixArchitecture `
    "-dProductVersion=$fourPart" `
    "-dDockpipeSource=$srcAbs" `
    "-dDockpipeWorkerHost=$workerHostAbs" `
    "-dDockpipeWorkerComponentGuid=$workerComponentGuid" `
    "-dDockpipeDesktopInputEnabled=$desktopInputEnabled" `
    "-dDockpipeDesktopInputService=$DesktopInputService" `
    "-dDockpipeDesktopInputHelper=$DesktopInputHelper" `
    "-dDockpipeDesktopInputNotices=$DesktopInputNotices" `
    "-dDockpipeInstallScope=$installScope" `
    "-dDockpipeInstallRoot=$installRoot" `
    "-dDockpipeRegistryRoot=$registryRoot" `
    "-dDockpipeSystemPath=$systemPath" `
    "-dDockpipeUpgradeCode=$upgradeCode" `
    "-dDockpipeUpgradeSchedule=$upgradeSchedule" `
    "-dDockpipeCliComponentGuid=$cliComponentGuid" `
    "-dDockpipePackageComponentGuid=$packageComponentGuid" `
    "-dDockpipeLauncherComponentGuid=$launcherComponentGuid" `
    "-dDockpipeShortcutComponentGuid=$shortcutComponentGuid" `
    "-dDockpipePackagesEnabled=$packagesEnabled" `
    "-dDockpipePackageStoreDir=$packageStageAbs" `
    "-dDockpipeLauncherEnabled=$launcherEnabled" `
    "-dDockpipeLauncherStageDir=$launcherStageAbs" `
    -out "$objDir\\" `
    $candleInputs
if ($LASTEXITCODE -ne 0) { throw "candle failed" }

$wixobjPaths = @($candleInputs | ForEach-Object {
    $objectName = [IO.Path]::GetFileNameWithoutExtension($_) + ".wixobj"
    $objectPath = Join-Path $objDir $objectName
    if (-not (Test-Path -LiteralPath $objectPath -PathType Leaf)) {
        throw "WiX did not compile the expected object: $objectPath"
    }

    $objectPath
})

# WiX 3: -arch is for candle only; light treats unknown args as .wixobj paths — "x64" became a bogus Source file (LGHT0103).
& $light -nologo `
    @lightOptions `
    -sw1076 `
    -sice:ICE38 `
    -sice:ICE64 `
    -out $msiPath `
    $wixobjPaths
if ($LASTEXITCODE -ne 0) { throw "light failed" }

Write-Host "Built: $msiPath"
if ($packageStageAbs) {
    Write-Host "Included explicitly selected package payload: $packageStageAbs"
}
if ($launcherStageAbs) {
    Write-Host "Included launcher payload: $launcherStageAbs"
}

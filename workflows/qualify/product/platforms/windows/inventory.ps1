param(
    [Parameter(Mandatory = $true)]
    [string]$Dockpipe,

    [Parameter(Mandatory = $true)]
    [string]$Output
)

$ErrorActionPreference = 'Stop'

if (-not [System.IO.Path]::IsPathRooted($Dockpipe)) {
    throw 'Dockpipe must be an absolute path to the selected Windows executable.'
}
if (-not [System.IO.Path]::IsPathRooted($Output)) {
    throw 'Output must be an absolute path to a new evidence directory.'
}
if (Test-Path -LiteralPath $Output) {
    throw 'Output already exists; preserve earlier evidence and select a new directory.'
}

$executable = Get-Item -LiteralPath $Dockpipe
if ($executable.PSIsContainer) {
    throw 'Dockpipe must identify a file.'
}

$operatingSystem = Get-CimInstance -ClassName Win32_OperatingSystem
$processors = @(Get-CimInstance -ClassName Win32_Processor)
$versionLines = @(& $executable.FullName --version 2>&1)
$versionExitCode = $LASTEXITCODE
if ($versionExitCode -ne 0) {
    throw "Selected Dockpipe executable returned exit code $versionExitCode for --version."
}

$inventory = [ordered]@{
    schema = 'dockpipe.product-integration.windows-inventory/v1'
    observed_at_utc = [DateTime]::UtcNow.ToString('o')
    platform = 'windows'
    os = [ordered]@{
        caption = $operatingSystem.Caption
        version = $operatingSystem.Version
        build = $operatingSystem.BuildNumber
        architecture = $operatingSystem.OSArchitecture
    }
    process_architecture = $env:PROCESSOR_ARCHITECTURE
    logical_processors = [int](($processors | Measure-Object -Property NumberOfLogicalProcessors -Sum).Sum)
    total_visible_memory_bytes = [long]$operatingSystem.TotalVisibleMemorySize * 1024
    dockpipe = [ordered]@{
        path = $executable.FullName
        sha256 = (Get-FileHash -LiteralPath $executable.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        version_exit_code = $versionExitCode
        version = ($versionLines | Out-String).Trim()
    }
    limits = @(
        'Identity and selected executable admission only.'
        'Does not prove installation, GUI embedding, remote features, or guest lifecycle.'
    )
}

$null = New-Item -ItemType Directory -Path $Output
$receipt = Join-Path $Output 'windows-inventory.json'
$json = $inventory | ConvertTo-Json -Depth 8
[System.IO.File]::WriteAllText($receipt, $json + "`n", (New-Object System.Text.UTF8Encoding($false)))
Write-Output $receipt

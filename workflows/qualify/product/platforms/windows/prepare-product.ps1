param(
    [Parameter(Mandatory = $true)]
    [string]$Payload,

    [Parameter(Mandatory = $true)]
    [string]$Request,

    [Parameter(Mandatory = $true)]
    [string]$Output,

    [Parameter(Mandatory = $true)]
    [string]$ExpectedMsiSha256,

    [Parameter(Mandatory = $true)]
    [string]$ExpectedCliSha256,

    [Parameter(Mandatory = $true)]
    [string]$ExpectedPythonSha256,

    [switch]$InstallInFreshGuest
)

$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT' -or -not $InstallInFreshGuest) {
    throw 'This operation requires Windows and explicit selection of a fresh disposable guest.'
}
foreach ($path in @($Payload, $Request, $Output)) {
    if (-not [System.IO.Path]::IsPathRooted($path)) {
        throw 'Payload, request, and output paths must be absolute.'
    }
}
foreach ($hash in @($ExpectedMsiSha256, $ExpectedCliSha256, $ExpectedPythonSha256)) {
    if ($hash -notmatch '^[a-f0-9]{64}$') {
        throw 'Expected artifact hashes must be explicit lowercase SHA-256 pins.'
    }
}
if (Test-Path -LiteralPath $Output) {
    throw 'Preserve prior evidence and select a new output directory.'
}

$selection = Get-Content -LiteralPath $Request -Raw | ConvertFrom-Json
if ($selection.schema -ne 'dockpipe.product-integration.request/v1' -or
    $selection.target.artifact_sha256 -ne $ExpectedMsiSha256) {
    throw 'Request must select this exact Windows MSI artifact.'
}

$msi = Join-Path $Payload 'dockpipe.msi'
$pythonArchive = Join-Path $Payload 'python-embed.zip'
if ((Get-FileHash -LiteralPath $msi -Algorithm SHA256).Hash.ToLowerInvariant() -ne $ExpectedMsiSha256) {
    throw 'Windows MSI artifact digest differs from the request.'
}
if ((Get-FileHash -LiteralPath $pythonArchive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $ExpectedPythonSha256) {
    throw 'Embedded Python artifact digest differs from the selected pin.'
}

$installDirectory = Join-Path $env:LOCALAPPDATA 'dockpipe'
$shortcut = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Dockpipe\Dockpipe.lnk'
if ((Test-Path -LiteralPath $installDirectory) -or (Test-Path -LiteralPath $shortcut)) {
    throw 'Existing Dockpipe installation or shortcut found; fresh guest installation required.'
}

$null = New-Item -ItemType Directory -Path $Output
$pythonDirectory = Join-Path $Output 'python'
Expand-Archive -LiteralPath $pythonArchive -DestinationPath $pythonDirectory
$python = Join-Path $pythonDirectory 'python.exe'
$msiLog = Join-Path $Output 'msi-install.log'
$installer = Start-Process msiexec.exe -ArgumentList @(
    '/i', "`"$msi`"", '/qn', '/norestart', '/l*v', "`"$msiLog`""
) -PassThru
if (-not $installer.WaitForExit(180000)) {
    throw 'MSI installation exceeded 180 seconds; preserve its uncertain state for inspection.'
}
if ($installer.ExitCode -notin @(0, 3010)) {
    throw "Windows Installer returned exit code $($installer.ExitCode)."
}

$cli = Join-Path $installDirectory 'dockpipe.exe'
$launcher = Join-Path $installDirectory 'dockpipe-launcher.exe'
foreach ($required in @($cli, $launcher, (Join-Path $installDirectory 'Qt6Core.dll'),
                        (Join-Path $installDirectory 'platforms\qwindows.dll'), $shortcut)) {
    if (-not (Test-Path -LiteralPath $required)) {
        throw "Expected default MSI installation payload is missing: $required"
    }
}
if ((Get-FileHash -LiteralPath $cli -Algorithm SHA256).Hash.ToLowerInvariant() -ne $ExpectedCliSha256) {
    throw 'Installed CLI digest differs from the pinned release executable.'
}

$receiptPath = Join-Path $Output 'msi-install.json'
$receipt = [ordered]@{
    schema = 'dockpipe.product-integration.windows-msi-install/v1'
    run_id = $selection.run_id
    plan_sha256 = $selection.plan_sha256
    registry_sha256 = $selection.registry_sha256
    target = $selection.target
    artifact = $msi
    msi_sha256 = $ExpectedMsiSha256
    installed_cli_sha256 = $ExpectedCliSha256
    embedded_python_sha256 = $ExpectedPythonSha256
    installer_exit_code = $installer.ExitCode
    installer_log_sha256 = (Get-FileHash -LiteralPath $msiLog -Algorithm SHA256).Hash.ToLowerInvariant()
    reboot_requested = ($installer.ExitCode -eq 3010)
    cli = $cli
    launcher = $launcher
    launcher_sha256 = (Get-FileHash -LiteralPath $launcher -Algorithm SHA256).Hash.ToLowerInvariant()
    native_result = (Join-Path $Output 'native-cli\result.json')
    gui_observed = $false
    limits = @('Default MSI payload presence checked; actual GUI and lifecycle acceptance require separate observations.')
}
[System.IO.File]::WriteAllText($receiptPath,
    ($receipt | ConvertTo-Json -Depth 20) + "`n", (New-Object System.Text.UTF8Encoding($false)))

$selection.configuration = [ordered]@{
    cli = $cli
    cli_sha256 = $ExpectedCliSha256
    artifact = $msi
    install_receipt = $receiptPath
    timeout_seconds = 60
}
$nativeRequest = Join-Path $Output 'native-request.json'
[System.IO.File]::WriteAllText($nativeRequest, ($selection | ConvertTo-Json -Depth 20) + "`n",
    (New-Object System.Text.UTF8Encoding($false)))
$driver = Join-Path $Payload 'driver.py'
& $python $driver --request $nativeRequest --output (Join-Path $Output 'native-cli')
if ($LASTEXITCODE -ne 0) {
    throw "Windows native qualification driver returned exit code $LASTEXITCODE."
}
Write-Output $receiptPath

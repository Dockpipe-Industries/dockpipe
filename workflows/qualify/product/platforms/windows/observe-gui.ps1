param(
    [Parameter(Mandatory = $true)]
    [string]$Launcher,

    [Parameter(Mandatory = $true)]
    [string]$ExpectedLauncherSha256,

    [Parameter(Mandatory = $true)]
    [string]$Cli,

    [Parameter(Mandatory = $true)]
    [string]$Output,

    [switch]$FreshGuestDesktop
)

$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT' -or -not $FreshGuestDesktop) {
    throw 'Native Windows observation requires explicit selection of the fresh guest desktop.'
}
foreach ($path in @($Launcher, $Cli, $Output)) {
    if (-not [System.IO.Path]::IsPathRooted($path)) {
        throw 'Launcher, CLI, and output selections must be absolute.'
    }
}
if ($ExpectedLauncherSha256 -notmatch '^[a-f0-9]{64}$' -or
    (Get-FileHash -LiteralPath $Launcher -Algorithm SHA256).Hash.ToLowerInvariant() -ne $ExpectedLauncherSha256) {
    throw 'Installed launcher digest differs from the selected MSI observation.'
}
if ((Test-Path -LiteralPath $Output) -or -not (Test-Path -LiteralPath $Cli)) {
    throw 'Select an existing installed CLI and a new private GUI evidence directory.'
}

$null = New-Item -ItemType Directory -Path $Output
foreach ($directory in @('config', 'local', 'workspace')) {
    $null = New-Item -ItemType Directory -Path (Join-Path $Output $directory)
}
$parameters = [ordered]@{ launcher = $Launcher; cli = $Cli; output = $Output }
$parametersPath = Join-Path $Output 'parameters.json'
[System.IO.File]::WriteAllText($parametersPath, ($parameters | ConvertTo-Json) + "`n",
    (New-Object System.Text.UTF8Encoding($false)))
$worker = Join-Path $Output 'observe-interactive.ps1'
$workerText = @'
param([Parameter(Mandatory = $true)][string]$Parameters)
$ErrorActionPreference = 'Stop'
$selection = Get-Content -LiteralPath $Parameters -Raw | ConvertFrom-Json
$gui = $null
try {
    $env:APPDATA = Join-Path $selection.output 'config'
    $env:LOCALAPPDATA = Join-Path $selection.output 'local'
    $env:DOCKPIPE_BIN = $selection.cli
    $env:QT_ACCESSIBILITY = '1'
    $env:QT_LINUX_ACCESSIBILITY_ALWAYS_ON = '1'
    $gui = Start-Process -FilePath $selection.launcher -ArgumentList '--allow-second-instance' `
        -WorkingDirectory (Join-Path $selection.output 'workspace') -PassThru
    $deadline = [DateTime]::UtcNow.AddSeconds(60)
    do {
        Start-Sleep -Milliseconds 200
        $gui.Refresh()
        if ($gui.HasExited) {
            throw 'Installed GUI exited before creating an interactive window.'
        }
    } while ($gui.MainWindowHandle -eq 0 -and [DateTime]::UtcNow -lt $deadline)
    if ($gui.MainWindowHandle -eq 0 -or $gui.SessionId -ne (Get-Process -Id $PID).SessionId) {
        throw 'Installed GUI did not produce a window in this interactive user session.'
    }
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    Add-Type -AssemblyName System.Drawing
    Add-Type -AssemblyName System.Windows.Forms
    $window = [System.Windows.Automation.AutomationElement]::FromHandle($gui.MainWindowHandle)
    if ($null -eq $window -or $window.Current.ProcessId -ne $gui.Id) {
        throw 'UI Automation did not resolve the exact installed GUI window.'
    }
    $screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    $bitmap = New-Object System.Drawing.Bitmap($screen.Width, $screen.Height)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    try {
        $graphics.CopyFromScreen($screen.Location, [System.Drawing.Point]::Empty, $screen.Size)
        $bitmap.Save((Join-Path $selection.output 'guest-desktop.png'), [System.Drawing.Imaging.ImageFormat]::Png)
    } finally {
        $graphics.Dispose()
        $bitmap.Dispose()
    }
    $receipt = [ordered]@{
        schema = 'dockpipe.product-integration.windows-gui-observation/v1'
        pid = $gui.Id
        session_id = $gui.SessionId
        start_time_utc = $gui.StartTime.ToUniversalTime().ToString('o')
        executable = $selection.launcher
        executable_sha256 = (Get-FileHash -LiteralPath $selection.launcher -Algorithm SHA256).Hash.ToLowerInvariant()
        window_title = $window.Current.Name
        window_handle = $gui.MainWindowHandle.ToInt64()
        screenshot = 'guest-desktop.png'
        screenshot_sha256 = (Get-FileHash -LiteralPath (Join-Path $selection.output 'guest-desktop.png') -Algorithm SHA256).Hash.ToLowerInvariant()
        worker_task_execution_limit_seconds = 120
        limits = @('Fresh fixture interactive desktop only.', 'Window and screenshot observation does not attest embedded input, feature behavior, or VM lifecycle.', 'The task execution limit may end its owned GUI process; further UI tests must observe process liveness.')
    }
    [System.IO.File]::WriteAllText((Join-Path $selection.output 'gui-observation.json'),
        ($receipt | ConvertTo-Json -Depth 8) + "`n", (New-Object System.Text.UTF8Encoding($false)))
} catch {
    [System.IO.File]::WriteAllText((Join-Path $selection.output 'gui-error.txt'), $_.Exception.Message)
    throw
}
'@
[System.IO.File]::WriteAllText($worker, $workerText, (New-Object System.Text.UTF8Encoding($false)))

# SSH may run in a non-interactive session. Use the already logged-in fixture
# user's Interactive token; no password, new logon, or SYSTEM desktop is used.
$taskName = 'DockpipeQualification-' + [Guid]::NewGuid().ToString('N')
$principal = New-ScheduledTaskPrincipal -UserId ([Security.Principal.WindowsIdentity]::GetCurrent().Name) `
    -LogonType Interactive -RunLevel Limited
$action = New-ScheduledTaskAction -Execute (Join-Path $env:WINDIR 'System32\WindowsPowerShell\v1.0\powershell.exe') `
    -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$worker`" -Parameters `"$parametersPath`""
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Minutes 2)
$registered = $false
$observed = $false
try {
    $null = Register-ScheduledTask -TaskName $taskName -Action $action -Principal $principal -Settings $settings
    $registered = $true
    Start-ScheduledTask -TaskName $taskName
    $deadline = [DateTime]::UtcNow.AddSeconds(90)
    do {
        if (Test-Path -LiteralPath (Join-Path $Output 'gui-error.txt')) {
            throw 'Interactive GUI worker failed; inspect its private error evidence.'
        }
        if (Test-Path -LiteralPath (Join-Path $Output 'gui-observation.json')) {
            $observed = $true
            Write-Output (Join-Path $Output 'gui-observation.json')
            return
        }
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $deadline)
    throw 'No GUI observation within 90 seconds; confirm the new guest has its expected interactive fixture login.'
} finally {
    if ($registered) {
        if (-not $observed) {
            Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
        }
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }
}

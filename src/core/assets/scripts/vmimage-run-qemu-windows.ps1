param(
  [Parameter(Mandatory = $true)]
  [ValidateSet('start', 'wait', 'stop', 'start-wait', 'report')]
  [string]$Action,

  [string]$QemuBin,
  [string]$PidFile,
  [string]$ArgsFile,
  [string]$StdOutFile,
  [string]$StdErrFile,
  [ValidateSet('prepared', 'started', 'exited')]
  [string]$ReporterPhase = 'prepared',
  [int]$ReporterPid = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Publish-VmSession {
  param([string]$Phase, [int]$ProcessId)

  $executable = $env:DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE
  if ([string]::IsNullOrWhiteSpace($executable)) {
    return
  }
  $names = @('PATH', 'HOME', 'USERPROFILE', 'LOCALAPPDATA', 'APPDATA',
    'SYSTEMROOT', 'WINDIR', 'TEMP', 'TMP', 'XDG_STATE_HOME', 'XDG_CACHE_HOME',
    'XDG_CONFIG_HOME', 'XDG_RUNTIME_DIR', 'DOCKPIPE_BIN',
    'DOCKPIPE_VM_EVENT_SCHEMA', 'DOCKPIPE_VM_EVENT_RUN_ID', 'DOCKPIPE_VM_EVENT_LAUNCH_ID',
    'DOCKPIPE_VM_EVENT_RESOURCE_ID', 'DOCKPIPE_VM_EVENT_WORKDIR',
    'DOCKPIPE_VM_EVENT_MACHINE_UUID', 'DOCKPIPE_VM_EVENT_SSH_HOST',
    'DOCKPIPE_VM_EVENT_SSH_PORT', 'DOCKPIPE_VM_EVENT_SSH_USER',
    'DOCKPIPE_VM_EVENT_SSH_KEY_REFERENCE', 'DOCKPIPE_VM_EVENT_AGENT_PORT',
    'DOCKPIPE_VM_EVENT_EXEC_MODE')
  $start = [Diagnostics.ProcessStartInfo]::new()
  $start.FileName = $executable
  $start.Arguments = 'report'
  $start.UseShellExecute = $false
  $start.CreateNoWindow = $true
  $start.EnvironmentVariables.Clear()
  foreach ($name in $names) {
    $value = [Environment]::GetEnvironmentVariable($name)
    if ($null -ne $value) {
      $start.EnvironmentVariables[$name] = $value
    }
  }
  $start.EnvironmentVariables['DOCKPIPE_VM_EVENT_PHASE'] = $Phase
  $start.EnvironmentVariables['DOCKPIPE_VM_EVENT_PID'] = [string]$ProcessId
  $process = [Diagnostics.Process]::new()
  $process.StartInfo = $start
  try {
    if (-not $process.Start()) {
      throw 'Session reporter did not start'
    }
    if (-not $process.WaitForExit(10000)) {
      $process.Kill()
      throw 'Session reporter exceeded its deadline'
    }
    if ($process.ExitCode -ne 0) {
      throw 'Session reporter failed'
    }
  } catch {
    Write-Warning 'VM session reporting failed; management status is unavailable.'
  } finally {
    $process.Dispose()
  }
}

function Read-QemuArgs {
  param([string]$Path)
  if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
    return @()
  }
  return @(Get-Content -LiteralPath $Path)
}

function Start-QemuProcess {
  param(
    [Parameter(Mandatory = $true)]
    [string]$FilePath,
    [string[]]$ArgumentList = @(),
    [string]$StdOutPath,
    [string]$StdErrPath,
    [switch]$CaptureOutput
  )

  $psi = [System.Diagnostics.ProcessStartInfo]::new()
  $psi.FileName = $FilePath
  $psi.UseShellExecute = $false

  foreach ($arg in @($ArgumentList)) {
    [void]$psi.ArgumentList.Add($arg)
  }

  if ($CaptureOutput -and -not [string]::IsNullOrWhiteSpace($StdOutPath)) {
    $psi.RedirectStandardOutput = $true
  }
  if ($CaptureOutput -and -not [string]::IsNullOrWhiteSpace($StdErrPath)) {
    $psi.RedirectStandardError = $true
  }

  $process = [System.Diagnostics.Process]::new()
  $process.StartInfo = $psi
  if (-not $process.Start()) {
    throw "Failed to start QEMU process."
  }

  return $process
}

function Wait-QemuProcess {
  param(
    [Parameter(Mandatory = $true)]
    [System.Diagnostics.Process]$Process
  )

  $cancelHandler = [ConsoleCancelEventHandler]{
    param($sender, $eventArgs)
    $eventArgs.Cancel = $true
    try {
      if (-not $Process.HasExited) {
        Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
      }
    } catch {
    }
  }

  [Console]::add_CancelKeyPress($cancelHandler)
  try {
    $Process.WaitForExit()
  } finally {
    [Console]::remove_CancelKeyPress($cancelHandler)
  }
}

function Get-QemuProcess {
  param([string]$Path)
  if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
    return $null
  }
  $pidText = (Get-Content -LiteralPath $Path -Raw).Trim()
  if ([string]::IsNullOrWhiteSpace($pidText)) {
    return $null
  }
  try {
    return Get-Process -Id ([int]$pidText) -ErrorAction Stop
  } catch {
    return $null
  }
}

function Stop-QemuProcessGracefully {
  param(
    [Parameter(Mandatory = $true)]
    [System.Diagnostics.Process]$Process,
    [int]$WaitMilliseconds = 15000
  )

  try {
    if ($Process.HasExited) {
      return
    }
  } catch {
    return
  }

  try {
    if ($Process.MainWindowHandle -ne 0) {
      $null = $Process.CloseMainWindow()
      if ($Process.WaitForExit($WaitMilliseconds)) {
        return
      }
    }
  } catch {
  }

  Stop-Process -Id $Process.Id -Force
}

switch ($Action) {
  'report' {
    Publish-VmSession -Phase $ReporterPhase -ProcessId $ReporterPid
    break
  }
  'start' {
    if ([string]::IsNullOrWhiteSpace($QemuBin)) { throw 'QemuBin is required for start' }
    if ([string]::IsNullOrWhiteSpace($PidFile)) { throw 'PidFile is required for start' }
    $args = Read-QemuArgs -Path $ArgsFile
    $process = Start-QemuProcess -FilePath $QemuBin -ArgumentList $args
    Set-Content -LiteralPath $PidFile -Value $process.Id -NoNewline
    Publish-VmSession -Phase 'started' -ProcessId $process.Id
    break
  }
  'start-wait' {
    if ([string]::IsNullOrWhiteSpace($QemuBin)) { throw 'QemuBin is required for start-wait' }
    if ([string]::IsNullOrWhiteSpace($PidFile)) { throw 'PidFile is required for start-wait' }
    $args = Read-QemuArgs -Path $ArgsFile
    $process = Start-QemuProcess -FilePath $QemuBin -ArgumentList $args -StdOutPath $StdOutFile -StdErrPath $StdErrFile -CaptureOutput
    Set-Content -LiteralPath $PidFile -Value $process.Id -NoNewline
    Publish-VmSession -Phase 'started' -ProcessId $process.Id
    Wait-QemuProcess -Process $process
    $exitCode = $process.ExitCode
    Publish-VmSession -Phase 'exited' -ProcessId $process.Id
    if (-not [string]::IsNullOrWhiteSpace($StdOutFile)) {
      [System.IO.File]::WriteAllText($StdOutFile, $process.StandardOutput.ReadToEnd())
    }
    if (-not [string]::IsNullOrWhiteSpace($StdErrFile)) {
      [System.IO.File]::WriteAllText($StdErrFile, $process.StandardError.ReadToEnd())
    }
    exit $exitCode
  }
  'wait' {
    $process = Get-QemuProcess -Path $PidFile
    if ($null -ne $process) {
      Wait-QemuProcess -Process $process
      Publish-VmSession -Phase 'exited' -ProcessId $process.Id
    }
    break
  }
  'stop' {
    $process = Get-QemuProcess -Path $PidFile
    if ($null -ne $process) {
      Stop-QemuProcessGracefully -Process $process
    }
    break
  }
}

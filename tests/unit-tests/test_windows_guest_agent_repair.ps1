# Execute only extracted repair functions with task/process/security commands mocked.
# No Windows service, scheduled task, live process or network endpoint is touched.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$source = Join-Path $PSScriptRoot '../../src/core/assets/scripts/provision-windows-ssh.ps1'
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$errors)
if ($errors.Count) {
  throw 'Provisioning source parse failed'
}
foreach ($name in @('Ensure-DockPipeAgent', 'Test-GuestAgentUnlimitedExecution', 'Test-DockPipeAgentTask', 'Test-DockPipeAgentConfiguration', 'Stop-SelectedDockPipeAgent')) {
  $definition = $ast.Find({
    param($node)
    $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
  }, $true)
  if ($null -eq $definition) {
    throw 'Required repair function missing'
  }
  Invoke-Expression $definition.Extent.Text
}

function Assert-True([bool]$Value, [string]$Message) {
  if (-not $Value) {
    throw $Message
  }
}

# Scalar names and numeric values captured from native Windows PowerShell 5.1.
# These local enum types model serialization; they are not native Task Scheduler calls.
Add-Type -TypeDefinition @'
namespace DockpipeObservedTaskShape {
  public enum LogonType { ServiceAccount = 5 }
  public enum RunLevel { Highest = 1 }
  public enum MultipleInstances { IgnoreNew = 2 }
}
'@

$originalProgramData = $env:ProgramData
$originalWindows = $env:WINDIR
$fixture = Join-Path ([IO.Path]::GetTempPath()) ('dockpipe-agent-repair-' + [guid]::NewGuid().ToString('N'))
[void][IO.Directory]::CreateDirectory($fixture)
$env:ProgramData = Join-Path $fixture 'data'
$env:WINDIR = Join-Path $fixture 'windows'
$agentRoot = Join-Path $env:ProgramData 'Dockpipe/GuestAgent'
$script:sourceAgent = Join-Path $fixture 'source.exe'
$target = Join-Path $agentRoot 'dockpipe-guest-agent.exe'
$config = Join-Path $agentRoot 'config.json'
$statePath = Join-Path $agentRoot 'state.json'
[void][IO.Directory]::CreateDirectory($agentRoot)

function Resolve-DockPipeAgentSourcePath {
  return $script:sourceAgent
}

function Assert-GuestToolsStatusPath {
  param([string]$Path, [bool]$Directory)
  if ($script:rejectPath) {
    throw 'Simulated unsafe path'
  }
}

function Get-GuestProvisioningSecurityFingerprint {
  param([string]$Path, [bool]$Directory)
  $script:fingerprintCalls++
  if ($script:securityChange -and $script:fingerprintCalls -eq 1) {
    return 'old-security'
  }
  return 'secure'
}

function Protect-GuestProvisioningDirectory {
  param([string]$Path)
  $script:events += 'protect-directory'
  if ($script:securityFailure) {
    throw 'Simulated ACL repair failure'
  }
}

function Protect-GuestProvisioningFile {
  param([string]$Path)
  $script:events += 'protect-file'
}

function Write-Step {
  param([string]$Message)
}

function Get-ScheduledTask {
  [CmdletBinding()]
  param([string]$TaskName, [string]$TaskPath)
  Assert-True ($TaskPath -ceq '\') 'Only the owned root scheduled task may be selected'
  return $script:task
}

function Get-DockPipeAgentProcesses {
  param([string]$Executable, [string]$Arguments, [string]$NativePath, [string]$ScriptPath)
  $script:events += 'inspect-processes'
  if ($script:running) {
    return [pscustomobject]@{
      Id = 100
      Image = $NativePath
      StartFileTime = 123
      DesiredInvocation = $script:desiredInvocation
    }
  }
}

function Stop-SelectedDockPipeAgent {
  param($Identity)
  $script:events += 'stop-exact-agent'
}

function Stop-ScheduledTask {
  throw 'Repair must never stop the potentially modified task action'
}

function Unregister-ScheduledTask {
  param([string]$TaskName, [switch]$Confirm)
  $script:events += 'unregister'
}

function New-ScheduledTaskAction {
  param([string]$Execute, [string]$Argument)
  return [pscustomobject]@{
    Execute = $Execute
    Arguments = $Argument
    WorkingDirectory = ''
  }
}

function New-ScheduledTaskTrigger {
  param([switch]$AtStartup)
  return [pscustomobject]@{
    Enabled = $true
    CimClass = [pscustomobject]@{
      CimClassName = 'MSFT_TaskBootTrigger'
    }
  }
}

function New-ScheduledTaskPrincipal {
  param([string]$UserId, [string]$LogonType, [string]$RunLevel)
  return [pscustomobject]@{
    UserId = $UserId
    LogonType = $LogonType
    RunLevel = $RunLevel
  }
}

function New-ScheduledTaskSettingsSet {
  param(
    [switch]$AllowStartIfOnBatteries,
    [switch]$DontStopIfGoingOnBatteries,
    [string]$MultipleInstances,
    [switch]$StartWhenAvailable,
    [TimeSpan]$ExecutionTimeLimit
  )
  Assert-True ($ExecutionTimeLimit -eq [TimeSpan]::Zero) 'Long-lived agent must explicitly request unlimited execution'
  return [pscustomobject]@{
    Enabled = $true
    DisallowStartIfOnBatteries = $false
    StopIfGoingOnBatteries = $false
    MultipleInstances = $MultipleInstances
    StartWhenAvailable = [bool]$StartWhenAvailable
    ExecutionTimeLimit = 'PT0S'
  }
}

function Register-ScheduledTask {
  param([string]$TaskName, $Action, $Trigger, $Principal, $Settings)
  $script:events += 'register'
  if ($script:registrationFailure -is [string] -and $script:registrationFailure -ceq 'cancelled') {
    throw [OperationCanceledException]::new('Simulated registration cancellation')
  }
  if ($script:registrationFailure) {
    throw 'Simulated registration failure'
  }
  $script:task = [pscustomobject]@{
    State = 'Ready'
    TaskPath = '\'
    Actions = @($Action)
    Triggers = @($Trigger)
    Principal = $Principal
    Settings = $Settings
  }
  if ($script:registrationDropsPolicy) {
    $script:task.Settings.ExecutionTimeLimit = 'PT72H'
  }
}

function Start-ScheduledTask {
  param([string]$TaskName)
  $script:events += 'start-task'
}

function Reset-Fixture {
  $script:events = @()
  $script:fingerprintCalls = 0
  $script:securityChange = $false
  $script:securityFailure = $false
  $script:registrationFailure = $false
  $script:rejectPath = $false
  $script:running = $true
  $script:desiredInvocation = $true
  $script:registrationDropsPolicy = $false
  [IO.File]::WriteAllText($script:sourceAgent, 'same-admitted-source')
  [IO.File]::WriteAllText($target, 'same-admitted-source')
  [ordered]@{
    port = 47831
    bind_address = '127.0.0.1'
    state_path = $statePath
    service_account = 'LocalSystem'
  } | ConvertTo-Json | Set-Content -LiteralPath $config
  $script:task = [pscustomobject]@{
    State = 'Running'
    TaskPath = '\'
    Actions = @([pscustomobject]@{
      Execute = $target
      Arguments = ('-service -ConfigPath "{0}"' -f $config)
      WorkingDirectory = ''
    })
    Principal = [pscustomobject]@{
      UserId = 'SYSTEM'
      LogonType = 'ServiceAccount'
      RunLevel = 'Highest'
    }
    Triggers = @([pscustomobject]@{
      Enabled = $true
      CimClass = [pscustomobject]@{
        CimClassName = 'MSFT_TaskBootTrigger'
      }
    })
    Settings = [pscustomobject]@{
      Enabled = $true
      ExecutionTimeLimit = 'PT0S'
      DisallowStartIfOnBatteries = $false
      StopIfGoingOnBatteries = $false
      MultipleInstances = 'IgnoreNew'
      StartWhenAvailable = $true
    }
  }
}

try {

  Reset-Fixture
  $before = (Get-Item -LiteralPath $config).LastWriteTimeUtc
  Ensure-DockPipeAgent -Port 47831
  Assert-True ($script:events -notcontains 'unregister' -and $script:events -notcontains 'stop-task' -and $script:events -notcontains 'start-task') 'Healthy repair must preserve task and process identity'
  Assert-True ((Get-Item -LiteralPath $config).LastWriteTimeUtc -eq $before) 'Healthy repair must not rewrite agent configuration'
  Assert-True ($script:events -contains 'protect-directory' -and $script:events -contains 'protect-file' -and $script:events -contains 'inspect-processes') 'Healthy fast path must still enforce security and inspect processes'

  Reset-Fixture
  $script:task.Principal.LogonType = [DockpipeObservedTaskShape.LogonType]::ServiceAccount
  $script:task.Principal.RunLevel = [DockpipeObservedTaskShape.RunLevel]::Highest
  $script:task.Settings.MultipleInstances = [DockpipeObservedTaskShape.MultipleInstances]::IgnoreNew
  $script:task.Settings.ExecutionTimeLimit = 'PT72H'
  Assert-True ([int]$script:task.Principal.LogonType -eq 5) 'Observed ServiceAccount value must remain 5'
  Assert-True ([int]$script:task.Principal.RunLevel -eq 1) 'Observed Highest value must remain 1'
  Assert-True ([int]$script:task.Settings.MultipleInstances -eq 2) 'Observed IgnoreNew value must remain 2'
  $arguments = $script:task.Actions[0].Arguments
  Assert-True (-not (Test-DockPipeAgentTask -Task $script:task -Executable $target -Arguments $arguments)) 'Actual existing PT72H task must require repair despite valid native enum shape'
  $script:task.Settings.ExecutionTimeLimit = 'PT0S'
  Assert-True (Test-DockPipeAgentTask -Task $script:task -Executable $target -Arguments $arguments) 'Observed enum shape with explicit unlimited duration must qualify'
  Ensure-DockPipeAgent -Port 47831
  Assert-True ($script:events -notcontains 'unregister' -and $script:events -notcontains 'start-task') 'Native-shaped healthy task must preserve its process'
  $script:task.Settings.ExecutionTimeLimit = 'PT15M'
  Assert-True (-not (Test-DockPipeAgentTask -Task $script:task -Executable $target -Arguments $arguments)) 'Provisioning finite duration must not qualify the long-lived agent'

  foreach ($change in @('binary', 'configuration', 'task', 'security', 'stopped', 'missing', 'obsolete', 'wrong-invocation')) {
    Reset-Fixture
    switch ($change) {
      'wrong-invocation' {
        $script:desiredInvocation = $false
      }
      'binary' {
        [IO.File]::WriteAllText($script:sourceAgent, 'new-admitted-source')
      }
      'obsolete' {
        [IO.File]::WriteAllText((Join-Path $agentRoot 'dockpipe-guest-agent.ps1'), 'old-script')
      }
      'configuration' {
        Set-Content -LiteralPath $config -Value '{"port":1}'
      }
      'task' {
        $script:task.Principal.UserId = 'unapproved-user'
      }
      'security' {
        $script:securityChange = $true
      }
      'stopped' {
        $script:running = $false
        $script:task.State = 'Ready'
      }
      'missing' {
        $script:running = $false
        $script:task = $null
      }
    }
    Ensure-DockPipeAgent -Port 47831
    Assert-True ($script:events -contains 'register' -and $script:events -contains 'start-task') ('Changed/unprovisioned state must recover: ' + $change)
    Assert-True ((Get-FileHash -LiteralPath $target).Hash -eq (Get-FileHash -LiteralPath $script:sourceAgent).Hash) 'Recovered binary must match admitted source'
    Assert-True (Test-DockPipeAgentConfiguration -Path $config -Port 47831 -StatePath $statePath) 'Recovered configuration must match exact contract'
  }

  Reset-Fixture
  Set-Content -LiteralPath $config -Value '{broken-json'
  Ensure-DockPipeAgent -Port 47831
  Assert-True ($script:events -contains 'register') 'Malformed owned configuration must be repaired, not accepted as healthy'

  foreach ($failure in @('path', 'security', 'registration', 'cancellation')) {
    Reset-Fixture
    switch ($failure) {
      'path' {
        $script:rejectPath = $true
      }
      'security' {
        $script:securityFailure = $true
      }
      'registration' {
        $script:registrationFailure = $true
        $script:task.Principal.UserId = 'wrong'
      }
      'cancellation' {
        $script:registrationFailure = 'cancelled'
        $script:task.Principal.UserId = 'wrong'
      }
    }
    $rejected = $false
    try {
      Ensure-DockPipeAgent -Port 47831
    } catch {
      $rejected = $true
    }
    Assert-True $rejected ('Expected repair failure must propagate: ' + $failure)
    Assert-True ($script:events -notcontains 'start-task') 'Failure must not continue to task start'
    if ($failure -in @('path', 'security')) {
      Assert-True ($script:events -notcontains 'stop-task' -and $script:events -notcontains 'unregister') 'Admission/security failures must not interrupt live agent'
    }
  }

  foreach ($change in @('action', 'trigger', 'runlevel', 'settings', 'disabled', 'limited-duration')) {
    Reset-Fixture
    switch ($change) {
      'disabled' {
        $script:task.Settings.Enabled = $false
      }
      'limited-duration' {
        $script:task.Settings.ExecutionTimeLimit = 'PT72H'
      }
      'action' {
        $script:task.Actions[0].Arguments = 'unapproved-arguments'
      }
      'trigger' {
        $script:task.Triggers[0].CimClass.CimClassName = 'MSFT_TaskTimeTrigger'
      }
      'runlevel' {
        $script:task.Principal.RunLevel = 'Limited'
      }
      'settings' {
        $script:task.Settings.MultipleInstances = 'Parallel'
      }
    }
    Ensure-DockPipeAgent -Port 47831
    Assert-True ($script:events -contains 'register') ('Unapproved task policy cannot take healthy fast path: ' + $change)
  }

  Reset-Fixture
  $script:task.Actions[0].Execute = 'C:\unrelated.exe'
  $script:registrationDropsPolicy = $true
  $rejected = $false
  try {
    Ensure-DockPipeAgent -Port 47831
  } catch {
    $rejected = $true
  }
  Assert-True ($rejected -and $script:events -notcontains 'start-task') 'Registration readback must reject a finite execution limit before start'
  Assert-True ($script:events -contains 'unregister' -and $script:events -contains 'stop-exact-agent') 'Modified task is unregistered, only selected agent is stopped'

  foreach ($duration in @('PT0S', 'P0D', [TimeSpan]::Zero)) {
    Assert-True (Test-GuestAgentUnlimitedExecution -Value $duration) 'Native zero-duration representations must be admitted'
  }

  foreach ($duration in @($null, '', 'PT72H', '00:00:00', 'invalid', [TimeSpan]::FromMinutes(1))) {
    Assert-True (-not (Test-GuestAgentUnlimitedExecution -Value $duration)) 'Absent, finite or malformed duration must not pass durable admission'
  }

  & {
    foreach ($name in @('Get-DockPipeAgentProcesses', 'Stop-SelectedDockPipeAgent')) {
      $definition = $ast.Find({
        param($node)
        $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
      }, $true)
      Invoke-Expression $definition.Extent.Text
    }
    $script:processOwner = 'S-1-5-18'

    $script:processImage = $target
    $script:processTime = [DateTime]::UtcNow
    $script:killed = $false
    $script:disposed = $false
    $script:waitResult = $true
    $script:candidates = @([pscustomobject]@{
      ProcessId = 123
      ExecutablePath = $target
      CommandLine = ('"{0}" -service -ConfigPath "{1}"' -f $target, $config)
    })
    function Get-CimInstance {
      param([string]$ClassName, [string]$Filter)
      return $script:candidates
    }
    function Invoke-CimMethod {
      param($InputObject, [string]$MethodName)
      return [pscustomobject]@{
        ReturnValue = 0
        Sid = $script:processOwner
      }
    }
    function Get-Process {
      param([int]$Id)
      $process = [pscustomobject]@{
        Id = $Id
        Path = $script:processImage
        StartTime = $script:processTime
        Handle = 42
      }
      $process | Add-Member ScriptMethod Dispose {
        $script:disposed = $true
      }
      $process | Add-Member ScriptMethod Kill {
        $script:killed = $true
      }
      $process | Add-Member ScriptMethod WaitForExit {
        param($Milliseconds)
        Assert-True ($Milliseconds -eq 5000) 'Exact cleanup wait must remain bounded'
        return $script:waitResult
      }
      return $process
    }
    $arguments = '-service -ConfigPath "{0}"' -f $config
    $selected = @(Get-DockPipeAgentProcesses -Executable $target -Arguments $arguments -NativePath $target -ScriptPath (Join-Path $agentRoot 'dockpipe-guest-agent.ps1'))
    Assert-True ($selected.Count -eq 1 -and $selected[0].Id -eq 123) 'Exact SYSTEM agent must be selected'
    Assert-True $selected[0].DesiredInvocation 'Exact desired invocation must be marked healthy'
    $script:candidates[0].CommandLine = ('"{0}" -service -ConfigPath "C:\wrong-config.json"' -f $target)
    $wrongInvocation = @(Get-DockPipeAgentProcesses -Executable $target -Arguments $arguments -NativePath $target -ScriptPath (Join-Path $agentRoot 'dockpipe-guest-agent.ps1'))
    Assert-True ($wrongInvocation.Count -eq 1 -and -not $wrongInvocation[0].DesiredInvocation) 'Same owned image with wrong arguments is cleanup-eligible but never healthy'

    $script:processOwner = 'S-1-5-32-545'
    $rejected = $false
    try {
      Get-DockPipeAgentProcesses -Executable $target -Arguments $arguments -NativePath $target -ScriptPath (Join-Path $agentRoot 'dockpipe-guest-agent.ps1') | Out-Null
    } catch {
      $rejected = $true
    }
    Assert-True $rejected 'Non-SYSTEM process must not be accepted or stopped'
    $script:processOwner = 'S-1-5-18'
    $scriptPath = Join-Path $agentRoot 'dockpipe-guest-agent.ps1'
    $powershell = Join-Path $env:WINDIR 'System32/WindowsPowerShell/v1.0/powershell.exe'
    $scriptArguments = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" -Service -ConfigPath "{1}"' -f $scriptPath, $config
    $script:processImage = $powershell
    $script:candidates = @([pscustomobject]@{
      ProcessId = 124
      ExecutablePath = $powershell
      CommandLine = ('"{0}" {1}' -f $powershell, $scriptArguments)
    })
    $selected = @(Get-DockPipeAgentProcesses -Executable $target -Arguments $arguments -NativePath $target -ScriptPath $scriptPath)
    Assert-True ($selected.Count -eq 1) 'Exact old PowerShell fallback must be selectable when moving to native mode'
    $script:candidates[0].CommandLine = 'powershell -Command "Write-Host ' + $scriptPath + '"'
    $selected = @(Get-DockPipeAgentProcesses -Executable $target -Arguments $arguments -NativePath $target -ScriptPath $scriptPath)
    Assert-True ($selected.Count -eq 0) 'An unrelated command mentioning the script must never be selected for termination'

    $script:processImage = $target
    $identity = [pscustomobject]@{
      Id = 123
      Image = $target
      StartFileTime = $script:processTime.ToFileTimeUtc() - 1
    }
    $script:killed = $false
    $script:disposed = $false
    $rejected = $false
    try {
      Stop-SelectedDockPipeAgent -Identity $identity
    } catch {
      $rejected = $true
    }
    Assert-True ($rejected -and -not $script:killed -and $script:disposed) 'PID reuse must reject termination and still dispose the process handle'
    $identity.StartFileTime = $script:processTime.ToFileTimeUtc()
    $script:waitResult = $false
    $rejected = $false
    try {
      Stop-SelectedDockPipeAgent -Identity $identity
    } catch {
      $rejected = $true
    }
    Assert-True ($rejected -and $script:killed -and $script:disposed) 'Exact cleanup timeout must propagate and dispose'
    $script:waitResult = $true
    $script:killed = $false
    Stop-SelectedDockPipeAgent -Identity $identity
    Assert-True ($script:killed -and $script:disposed) 'Exact matching process must be stopped and disposed'
  }
  Write-Output 'Agent repair, invocation, task-duration and identity scenarios passed with mocked native boundaries; no Windows mutation or network.'
} finally {
  $env:ProgramData = $originalProgramData
  $env:WINDIR = $originalWindows
  Remove-Item -LiteralPath $fixture -Recurse -Force
}

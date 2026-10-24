# Offline guest-service regression checks. Load function definitions only;
# never install capabilities, mutate Windows accounts, or contact a guest.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$source = Join-Path $PSScriptRoot '../../src/core/assets/scripts/provision-windows-ssh.ps1'
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) {
    throw "Guest provisioning script has parse errors: $parseErrors"
}
foreach ($name in @('Ensure-AuthorizedKey', 'Set-SshdOption', 'Start-ProvisionedSshService', 'Read-StartupRepairConfiguration', 'Protect-GuestProvisioningDirectory', 'Protect-GuestProvisioningFile', 'Install-GuestProvisioningStartupTask', 'Assert-GuestToolsStatusPath', 'Protect-GuestToolsStatusDirectory', 'Protect-GuestToolsStatusFile', 'Initialize-GuestToolsStatusStorage', 'Write-GuestToolsStatus', 'Test-GuestToolsAgentHealth', 'Wait-GuestToolsHealth')) {
    $definition = $ast.Find({ param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
    }, $true)
    if ($null -eq $definition) {
        throw "Guest provisioning function is missing: $name"
    }
    . ([scriptblock]::Create($definition.Extent.Text))
}

# Execute only the source script's parameter/default-resolution statements.
# Its administration and provisioning entrypoint remains excluded.
$firstFunction = $ast.EndBlock.Statements |
    Where-Object { $_ -is [Management.Automation.Language.FunctionDefinitionAst] } |
    Select-Object -First 1
$defaultStatements = @($ast.EndBlock.Statements |
    Where-Object { $_.Extent.StartOffset -lt $firstFunction.Extent.StartOffset })
$bootstrapAssignment = $ast.Find({ param($node)
    $node -is [Management.Automation.Language.AssignmentStatementAst] -and
    $node.Left.Extent.Text -eq '$bootstrapKeyPath'
}, $true)
$bootstrapBranch = $ast.Find({ param($node)
    $node -is [Management.Automation.Language.IfStatementAst] -and
    $node.Extent.Text -match '^if \(-not \$PSBoundParameters.ContainsKey\(''AuthorizedKey''\)'
}, $true)
$defaultCode = $ast.ParamBlock.Extent.Text + "`n" +
    '$PSScriptRoot = $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA' + "`n" +
    (($defaultStatements | ForEach-Object { $_.Extent.Text }) -join "`n") + "`n" +
    $bootstrapAssignment.Extent.Text + "`n" + $bootstrapBranch.Extent.Text + "`n" +
    '[pscustomobject]@{UserName=$UserName;AuthorizedKey=$AuthorizedKey;CreateUser=$CreateUser;PasswordPlain=$PasswordPlain}'
$resolveBootstrapDefaults = [scriptblock]::Create($defaultCode)

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) {
        throw $Message
    }
}

function New-TestSid {
    param([string]$Value)
    $sid = [pscustomobject]@{ Value = $Value }
    $sid | Add-Member -MemberType ScriptMethod -Name ToString -Value { $this.Value } -Force
    return $sid
}

function Write-Step {
    param([string]$Message, [string]$Phase)
}

function Get-LocalUser {
    [CmdletBinding()]
    param([string]$Name)
    Assert-True ($Name -eq 'fixture-user') 'Unexpected account lookup'
    return [pscustomobject]@{ Name = $Name; SID = $script:userSid }
}

function Get-LocalGroupMember {
    [CmdletBinding()]
    param($SID)
    Assert-True ([string]$SID -eq 'S-1-5-32-544') 'Administrator membership must use the well-known SID'
    if ($script:administrator) {
        return [pscustomobject]@{ Name = 'FIXTURE\fixture-user'; SID = $script:userSid }
    }
    return @()
}

function Get-ItemProperty {
    [CmdletBinding()]
    param([Alias('LiteralPath')][string]$Path, [string]$Name)
    Assert-True ($Path.EndsWith($script:userSid.Value)) 'Profile lookup must identify the exact account SID'
    if (-not $script:profilePresent) {
        return $null
    }
    return [pscustomobject]@{ ProfileImagePath = $script:profileRoot }
}

function icacls {
    $script:aclCalls += ,@($args)
    $global:LASTEXITCODE = $script:aclExit
}

function Get-Command {
    [CmdletBinding()]
    param([string]$Name)
    Assert-True ($Name -in @('sshd.exe', 'ssh-keygen.exe')) 'Only the SSH daemon and host-key generator may be selected for validation'
    return [pscustomobject]@{ Source = $Name; Path = $Name }
}

function ssh-keygen.exe {
    $script:serviceCalls += 'host-keys'
    Assert-True (($args -join '|') -eq '-A') 'Host-key generation must preserve existing host identities'
    $global:LASTEXITCODE = $script:keygenExit
}

function sshd.exe {
    $script:serviceCalls += 'validate'
    Assert-True (($args -join '|') -eq ('-t|-f|' + $script:serviceConfig)) 'Validate the selected SSH configuration'
    $global:LASTEXITCODE = $script:validationExit
}

function Get-Service {
    [CmdletBinding()]
    param([string]$Name)
    Assert-True ($Name -eq 'sshd') 'Unexpected service lookup'
    return [pscustomobject]@{ Status = $script:serviceStatus }
}

function Set-Service {
    [CmdletBinding()]
    param([string]$Name, [string]$StartupType)
    Assert-True ($Name -eq 'sshd' -and $StartupType -eq 'Automatic') 'SSH service must start automatically'
    $script:serviceCalls += 'automatic'
}

function Start-Service {
    [CmdletBinding()]
    param([string]$Name)
    Assert-True ($Name -eq 'sshd') 'Unexpected service start'
    $script:serviceCalls += 'start'
}

function Restart-Service {
    [CmdletBinding()]
    param([string]$Name, [switch]$Force)
    Assert-True ($Name -eq 'sshd') 'Unexpected service restart'
    $script:serviceCalls += 'restart'
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ('dockpipe-guest-services-' + [Guid]::NewGuid().ToString('N'))
$oldProgramData = $env:ProgramData
$oldSystemDrive = $env:SystemDrive
$oldComputerName = $env:COMPUTERNAME
$oldUsername = $env:USERNAME
$oldWindowsDirectory = $env:WINDIR
$oldBootstrapMedia = $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    $env:ProgramData = Join-Path $testRoot 'ProgramData'
    $env:SystemDrive = Join-Path $testRoot 'SystemDrive'
    $env:COMPUTERNAME = 'FIXTURE'
    $script:userSid = New-TestSid 'S-1-5-21-123-456-789-1001'
    $script:profilePresent = $true
    $script:aclExit = 0
    $script:profileRoot = Join-Path $testRoot 'Profiles/actual-profile'
    $key = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJDMtS1LxWpYYgH0HVeMHuZiQPiwyEY3sFKPBtYCPtLf fixture'
    $existingKey = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH3NdQ7IC6uY1Je1zTW6LDVCORq0xs1bO0TwOVYWZVCh previous'

    foreach ($script:administrator in @($false, $true)) {
        $script:aclCalls = @()
        $authKeys = if ($script:administrator) {
            Join-Path $env:ProgramData 'ssh/administrators_authorized_keys'
        } else {
            Join-Path $script:profileRoot '.ssh/authorized_keys'
        }
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $authKeys) | Out-Null
        Set-Content -LiteralPath $authKeys -Value $existingKey
        Ensure-AuthorizedKey -Name 'fixture-user' -Key $key
        Ensure-AuthorizedKey -Name 'fixture-user' -Key $key
        $keys = @(Get-Content -LiteralPath $authKeys)
        Assert-True ($keys.Count -eq 2 -and $keys -contains $existingKey -and $keys -contains $key) 'Key installation must preserve existing entries and be idempotent'
        $aclText = ($script:aclCalls | ForEach-Object { $_ -join ' ' }) -join "`n"
        Assert-True ($aclText.Contains('/inheritance:r')) 'Authorized keys must disable inherited access'
        Assert-True ($aclText.Contains('/grant:r')) 'Authorized keys must replace intended explicit grants'
        Assert-True ($aclText -match 'S-1-5-18|SYSTEM') 'SYSTEM must retain authorized-key access'
        if ($script:administrator) {
            Assert-True ($aclText.Contains('S-1-5-32-544')) 'Administrator key ACL must use the well-known group SID'
        }
    }

    $script:administrator = $false
    $script:profilePresent = $false
    $fallbackKeys = Join-Path $env:SystemDrive 'Users/fixture-user/.ssh/authorized_keys'
    Ensure-AuthorizedKey -Name 'fixture-user' -Key $key
    Assert-True (Test-Path -LiteralPath $fallbackKeys) 'Unregistered profiles must use the account profile fallback'
    $script:profilePresent = $true

    foreach ($invalidKey in @("$key`nsecond-key", ('x' * 16385))) {
        $script:aclCalls = @()
        $rejected = $false
        try {
            Ensure-AuthorizedKey -Name 'fixture-user' -Key $invalidKey
        } catch {
            $rejected = $true
        }
        Assert-True $rejected 'Reject malformed or oversized public-key input'
        Assert-True ($script:aclCalls.Count -eq 0) 'Invalid public keys must not mutate permissions'
    }
    $script:aclExit = 5
    $rejected = $false
    try {
        Ensure-AuthorizedKey -Name 'fixture-user' -Key $key
    } catch {
        $rejected = $true
    }
    Assert-True $rejected 'Permission tool failure must stop key provisioning'
    $script:aclExit = 0

    $configPath = Join-Path $testRoot 'sshd_config'
    Set-Content -LiteralPath $configPath -Value @(
        'PasswordAuthentication yes',
        'Match Group administrators',
        '    PasswordAuthentication no',
        '    AuthorizedKeysFile __PROGRAMDATA__/ssh/administrators_authorized_keys'
    )
    Set-SshdOption -ConfigPath $configPath -Name 'PasswordAuthentication' -Value 'no'
    $lines = @(Get-Content -LiteralPath $configPath)
    Assert-True ($lines[0] -eq 'PasswordAuthentication no') 'Replace the existing global setting'
    Assert-True ($lines[2] -eq '    PasswordAuthentication no') 'Preserve conditional settings'

    Set-Content -LiteralPath $configPath -Value @(
        '# Global settings',
        'Match Group administrators',
        '    PubkeyAuthentication no'
    )
    Set-SshdOption -ConfigPath $configPath -Name 'PubkeyAuthentication' -Value 'yes'
    $lines = @(Get-Content -LiteralPath $configPath)
    $globalIndex = [Array]::IndexOf($lines, 'PubkeyAuthentication yes')
    $matchIndex = [Array]::IndexOf($lines, 'Match Group administrators')
    Assert-True ($globalIndex -ge 0 -and $globalIndex -lt $matchIndex) 'Insert new global options before the first Match block'
    Assert-True ($lines[-1] -eq '    PubkeyAuthentication no') 'Preserve matching-block options when inserting globals'

    $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA = Join-Path $testRoot 'bootstrap-media'
    New-Item -ItemType Directory -Path $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA | Out-Null
    $env:USERNAME = 'logged-in-fixture'
    $bootstrapKey = Join-Path $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA 'dockpipe-guest-ssh.pub'

    $defaults = & $resolveBootstrapDefaults
    Assert-True ($defaults.UserName -eq 'dockpipe' -and $defaults.CreateUser) 'Absent bootstrap metadata must preserve established explicit provisioning defaults'
    Set-Content -LiteralPath $bootstrapKey -Value $key
    $defaults = & $resolveBootstrapDefaults
    Assert-True ($defaults.AuthorizedKey -eq $key) 'Attached bootstrap public key must be loaded and trimmed'
    Assert-True ($defaults.UserName -eq 'logged-in-fixture') 'Existing-image setup must select the logged-in account'
    Assert-True (-not $defaults.CreateUser -and $defaults.PasswordPlain -eq '') 'Bootstrap defaults must not create accounts or change passwords'

    $defaults = & $resolveBootstrapDefaults -UserName 'explicit-fixture'
    Assert-True ($defaults.UserName -eq 'explicit-fixture' -and -not $defaults.CreateUser) 'An explicit username must remain authoritative without implicit account creation'
    $defaults = & $resolveBootstrapDefaults -UserName 'explicit-new-user' -CreateUser $true
    Assert-True ($defaults.UserName -eq 'explicit-new-user' -and $defaults.CreateUser) 'Explicit account-creation intent must remain available'
    $defaults = & $resolveBootstrapDefaults -AuthorizedKey $existingKey -UserName 'explicit-user' -CreateUser $false
    Assert-True ($defaults.AuthorizedKey -eq $existingKey -and $defaults.UserName -eq 'explicit-user' -and -not $defaults.CreateUser) 'Explicit bootstrap parameters must override attached metadata'
    $defaults = & $resolveBootstrapDefaults -AuthorizedKey ''
    Assert-True ($defaults.AuthorizedKey -eq '') 'An explicitly empty key must not be silently replaced with attached metadata'

    [IO.File]::WriteAllText($bootstrapKey, ('x' * 16385))
    $rejected = $false
    try {
        & $resolveBootstrapDefaults | Out-Null
    } catch {
        $rejected = $true
    }
    Assert-True $rejected 'Oversized bootstrap public-key files must be rejected before provisioning'
    $defaults = & $resolveBootstrapDefaults -AuthorizedKey $existingKey -UserName 'explicit-user' -CreateUser $false
    Assert-True ($defaults.AuthorizedKey -eq $existingKey) 'Explicit keys must not read unused oversized bootstrap metadata'

    # Existing private host keys must receive service-safe ACLs without rotation.
    $actualFileProtection = (Get-Item Function:\Protect-GuestProvisioningFile).ScriptBlock
    $script:hostKeyProtectionFailure = $false
    $hostKeyContents = @{}
    foreach ($name in @('ssh_host_rsa_key', 'ssh_host_ecdsa_key', 'ssh_host_ed25519_key')) {
        $path = Join-Path (Join-Path $env:ProgramData 'ssh') $name
        $contents = 'preserved-host-identity-' + $name
        [IO.File]::WriteAllText($path, $contents)
        $hostKeyContents[$path] = $contents
    }
    function Protect-GuestProvisioningFile {
        param([string]$Path)
        $script:serviceCalls += 'protect-host-key'
        Assert-True ([IO.Path]::GetFileName($Path) -in @('ssh_host_rsa_key', 'ssh_host_ecdsa_key', 'ssh_host_ed25519_key')) 'Only private host-key files must receive host identity protection'
        if ($script:hostKeyProtectionFailure) {
            throw 'Simulated host-key ACL failure'
        }
    }
    $script:serviceConfig = $configPath
    $script:keygenExit = 0
    foreach ($script:serviceStatus in @('Running', 'Stopped')) {
        $script:serviceCalls = @()
        $script:validationExit = 0
        Start-ProvisionedSshService -ConfigPath $configPath
        $expectedAction = if ($script:serviceStatus -eq 'Running') { 'restart' } else { 'start' }
        Assert-True (($script:serviceCalls -join '|') -eq ('host-keys|protect-host-key|protect-host-key|protect-host-key|validate|automatic|' + $expectedAction)) 'Validate before mutating or starting the service'
    }
    $script:serviceCalls = @()
    $script:validationExit = 1
    $rejected = $false
    try {
        Start-ProvisionedSshService -ConfigPath $configPath
    } catch {
        $rejected = $true
    }
    Assert-True $rejected 'Reject invalid SSH configuration'
    Assert-True (($script:serviceCalls -join '|') -eq 'host-keys|protect-host-key|protect-host-key|protect-host-key|validate') 'Invalid configuration must not mutate service state'
    $script:serviceCalls = @()
    $script:keygenExit = 1
    $rejected = $false
    try {
        Start-ProvisionedSshService -ConfigPath $configPath
    } catch {
        $rejected = $true
    }
    Assert-True $rejected 'Host-key generation failure must stop service preparation'
    Assert-True (($script:serviceCalls -join '|') -eq 'host-keys') 'Host-key generation failure must prevent configuration validation and service mutation'
    $script:keygenExit = 0

    $script:serviceCalls = @()
    $script:hostKeyProtectionFailure = $true
    $rejected = $false
    try {
        Start-ProvisionedSshService -ConfigPath $configPath
    } catch {
        $rejected = $true
    }
    Assert-True $rejected 'Host-key permission failure must stop service preparation'
    Assert-True (($script:serviceCalls -join '|') -eq 'host-keys|protect-host-key') 'Host-key permission failure must prevent validation and service mutation'
    foreach ($path in $hostKeyContents.Keys) {
        Assert-True ([IO.File]::ReadAllText($path) -eq $hostKeyContents[$path]) 'Repair must preserve existing private host-key bytes'
    }
    Set-Item Function:\Protect-GuestProvisioningFile -Value $actualFileProtection

    # Validate persisted repair metadata without executing the provisioning entrypoint.
    $repairFixture = Join-Path $testRoot 'repair'
    New-Item -ItemType Directory -Path $repairFixture | Out-Null
    $repairConfig = Join-Path $repairFixture 'configuration.json'
    $validConfiguration = @{
        SchemaVersion = 1
        UserName = 'fixture-user'
        AuthorizedKey = $key
        DefaultShell = 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe'
        EnablePasswordAuth = $false
        EnablePublicKeyAuth = $true
        InstallDockPipeAgent = $true
        AgentPort = 47831
    }
    $validConfiguration | ConvertTo-Json | Set-Content -LiteralPath $repairConfig
    $loaded = Read-StartupRepairConfiguration -Path $repairConfig
    Assert-True ($loaded.UserName -eq 'fixture-user' -and $loaded.AgentPort -eq 47831) 'Valid repair metadata must retain its exact identity and port'
    $invalidConfigurations = @(
        @{ Field = 'SchemaVersion'; Value = '1' },
        @{ Field = 'SchemaVersion'; Value = 2 },
        @{ Field = 'UserName'; Value = '' },
        @{ Field = 'AuthorizedKey'; Value = "$key`nsecond-key" },
        @{ Field = 'AuthorizedKey'; Value = ('x' * 16385) },
        @{ Field = 'DefaultShell'; Value = 1 },
        @{ Field = 'EnablePasswordAuth'; Value = 'false' },
        @{ Field = 'EnablePublicKeyAuth'; Value = 1 },
        @{ Field = 'InstallDockPipeAgent'; Value = $null },
        @{ Field = 'AgentPort'; Value = '47831' },
        @{ Field = 'AgentPort'; Value = 0 },
        @{ Field = 'AgentPort'; Value = 65536 },
        @{ Field = 'PasswordPlain'; Value = 'must-not-persist' }
    )
    foreach ($scenario in $invalidConfigurations) {
        $invalid = $validConfiguration.Clone()
        $invalid[$scenario.Field] = $scenario.Value
        $invalid | ConvertTo-Json | Set-Content -LiteralPath $repairConfig
        $rejected = $false
        try {
            Read-StartupRepairConfiguration -Path $repairConfig | Out-Null
        } catch {
            $rejected = $true
        }
        Assert-True $rejected ("Invalid repair metadata field was accepted: " + $scenario.Field)
    }
    [IO.File]::WriteAllText($repairConfig, ('x' * 32769))
    $rejected = $false
    try {
        Read-StartupRepairConfiguration -Path $repairConfig | Out-Null
    } catch {
        $rejected = $true
    }
    Assert-True $rejected 'Oversized repair metadata must be rejected'
    $validConfiguration | ConvertTo-Json | Set-Content -LiteralPath $repairConfig

    # These reject before platform-specific ACL construction, including on Linux.
    $ordinaryFile = Join-Path $repairFixture 'not-a-directory'
    Set-Content -LiteralPath $ordinaryFile -Value 'preserved'
    $rejected = $false
    try {
        Protect-GuestProvisioningDirectory -Path $ordinaryFile
    } catch {
        $rejected = $true
    }
    Assert-True $rejected 'A file must not be accepted as protected repair storage'
    if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
        $redirect = Join-Path $repairFixture 'redirect'
        New-Item -ItemType SymbolicLink -Path $redirect -Target $testRoot | Out-Null
        $rejected = $false
        try {
            Protect-GuestProvisioningDirectory -Path $redirect
        } catch {
            $rejected = $true
        }
        Assert-True $rejected 'Linked repair directories must be rejected'
        $redirectedConfig = Join-Path $repairFixture 'redirected-configuration.json'
        New-Item -ItemType SymbolicLink -Path $redirectedConfig -Target $repairConfig | Out-Null
        $rejected = $false
        try {
            Read-StartupRepairConfiguration -Path $redirectedConfig | Out-Null
        } catch {
            $rejected = $true
        }
        Assert-True $rejected 'Linked repair configuration must be rejected'
    }

    $rejected = $false
    try { Protect-GuestProvisioningFile -Path $repairFixture } catch { $rejected = $true }
    Assert-True $rejected 'A directory must not be accepted as an ordinary persisted repair file'
    if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
        $rejected = $false
        try { Protect-GuestProvisioningFile -Path $redirectedConfig } catch { $rejected = $true }
        Assert-True $rejected 'Linked persisted repair files must be rejected'
    }

    # Task assembly uses fixture files and mocks every Windows scheduler/ACL call.
    $env:WINDIR = Join-Path $testRoot 'Windows'
    $script:taskEvents = @()
    $script:registration = $null
    $script:protectionFailure = $false
    $script:fileProtectionFailure = $false
    $script:protectedFiles = @()
    $script:taskSourceRoot = Join-Path $repairFixture 'source'
    New-Item -ItemType Directory -Path $script:taskSourceRoot | Out-Null
    $script:helperSource = Join-Path $script:taskSourceRoot 'provision-windows-ssh.ps1'
    $script:agentSource = Join-Path $script:taskSourceRoot 'dockpipe-guest-agent.exe'
    $installDefinition = $ast.Find({ param($node)
        $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Install-GuestProvisioningStartupTask'
    }, $true)
    Set-Content -LiteralPath $script:helperSource -Value $installDefinition.Extent.Text
    # A real fixture source file gives PSCommandPath the same semantics as the
    # installed script while still containing only the isolated test function.
    . $script:helperSource
    Set-Content -LiteralPath $script:agentSource -Value 'fixture-agent-source'
    function Protect-GuestProvisioningDirectory {
        param([string]$Path)
        $script:taskEvents += 'protect'
        if ($script:protectionFailure) { throw 'Simulated ACL protection failure' }
        New-Item -ItemType Directory -Force -Path $Path | Out-Null
    }
    function Protect-GuestProvisioningFile {
        param([string]$Path)
        $script:taskEvents += 'protect-file'
        if ($script:fileProtectionFailure) { throw 'Simulated file ACL protection failure' }
        Assert-True (Test-Path -LiteralPath $Path -PathType Leaf) 'Only an existing file can receive persisted repair protection'
        $script:protectedFiles += $Path
    }
    function Resolve-DockPipeAgentSourcePath { return $script:agentSource }
    function New-ScheduledTaskAction {
        param([string]$Execute, [string]$Argument)
        return [pscustomobject]@{ Execute = $Execute; Argument = $Argument }
    }
    function New-ScheduledTaskTrigger {
        param([switch]$AtStartup)
        Assert-True $AtStartup.IsPresent 'Repair must run at startup'
        return [pscustomobject]@{ AtStartup = $true }
    }
    function New-ScheduledTaskPrincipal {
        param([string]$UserId, [string]$LogonType, [string]$RunLevel)
        Assert-True ($UserId -eq 'SYSTEM' -and $LogonType -eq 'ServiceAccount' -and $RunLevel -eq 'Highest') 'Repair must run as SYSTEM without an interactive user login'
        return [pscustomobject]@{ UserId = $UserId }
    }
    function Get-CimClass {
        param([string]$Namespace, [string]$ClassName)
        Assert-True ($Namespace -eq 'Root/Microsoft/Windows/TaskScheduler' -and $ClassName -eq 'MSFT_TaskEventTrigger') 'Clock repair must use the native task event trigger'
        return [pscustomobject]@{ ClassName = $ClassName }
    }
    function New-CimInstance {
        param([switch]$ClientOnly, $CimClass, [hashtable]$Property)
        Assert-True $ClientOnly.IsPresent 'Clock trigger construction must not create an unmanaged task'
        Assert-True ($CimClass.ClassName -eq 'MSFT_TaskEventTrigger') 'Clock trigger must preserve its native class inheritance'
        Assert-True ($Property.Enabled -and $Property.Delay -eq 'PT30S') 'Clock repair must wait for time synchronization to settle'
        [xml]$subscription = $Property.Subscription
        Assert-True ($subscription.QueryList.Query.Select.InnerText -eq "*[System[Provider[@Name='Microsoft-Windows-Kernel-General'] and EventID=1]]") 'Clock repair must be scoped to system time changes'
        return [pscustomobject]$Property
    }
    function New-ScheduledTaskSettingsSet {
        param(
            [switch]$AllowStartIfOnBatteries, [switch]$DontStopIfGoingOnBatteries,
            [switch]$StartWhenAvailable, [string]$MultipleInstances,
            [TimeSpan]$ExecutionTimeLimit, [int]$RestartCount, [TimeSpan]$RestartInterval
        )
        Assert-True ($MultipleInstances -eq 'IgnoreNew') 'Repair invocations must not overlap'
        Assert-True ($ExecutionTimeLimit.TotalMinutes -eq 15) 'Repair attempts must be bounded to 15 minutes'
        Assert-True ($RestartCount -eq 3 -and $RestartInterval.TotalMinutes -eq 1) 'Repair retries must use the declared finite retry policy'
        return [pscustomobject]@{ MultipleInstances = $MultipleInstances }
    }
    function Register-ScheduledTask {
        param([string]$TaskName, $Action, $Trigger, $Principal, $Settings, [switch]$Force)
        $script:taskEvents += 'register'
        $script:registration = [pscustomobject]@{ TaskName = $TaskName; Action = $Action; Trigger = $Trigger; Force = $Force }
    }
    function Start-ScheduledTask { throw 'Repair installation must not start itself recursively' }
    function Invoke-TestRepairInstallation {
        param([hashtable]$Configuration)
        $PSCommandPath = $script:helperSource
        Install-GuestProvisioningStartupTask -Configuration $Configuration
    }
    Invoke-TestRepairInstallation -Configuration $validConfiguration
    $persistedRoot = Join-Path $env:ProgramData 'Dockpipe/GuestProvisioning'
    $persistedConfig = Join-Path $persistedRoot 'configuration.json'
    Assert-True ($null -ne $script:registration -and $script:registration.TaskName -eq 'DockPipeGuestProvisioning') 'Repair task must be registered under its stable identity'
    Assert-True ($script:registration.Trigger.Count -eq 2) 'Provisioning must recover both at startup and after clock correction'
    Assert-True ($script:taskEvents[-1] -eq 'register') 'Files and protection must be prepared before registration'
    Assert-True ($script:registration.Action.Argument.EndsWith('-StartupRepair')) 'Scheduled task must use the dedicated repair entrypoint'
    Assert-True ($script:registration.Action.Argument.Contains((Join-Path $persistedRoot 'provision-windows-ssh.ps1'))) 'Scheduled task must use the protected persisted helper'
    foreach ($name in @('provision-windows-ssh.ps1', 'dockpipe-guest-agent.exe', 'configuration.json')) {
        Assert-True (Test-Path -LiteralPath (Join-Path $persistedRoot $name)) ('Persisted repair asset is missing: ' + $name)
    }
    foreach ($name in @('provision-windows-ssh.ps1', 'dockpipe-guest-agent.exe', 'configuration.json')) {
        Assert-True ($script:protectedFiles -contains (Join-Path $persistedRoot $name)) ('Persisted repair file must receive an explicit protected ACL: ' + $name)
    }
    $script:protectedFiles = @()
    $persistedHelper = Join-Path $persistedRoot 'provision-windows-ssh.ps1'
    . $persistedHelper
    Invoke-TestRepairInstallation -Configuration $validConfiguration
    Assert-True ($script:protectedFiles -contains $persistedHelper) 'Same-source reinstall must protect the helper without deleting its own source'
    . $script:helperSource
    $published = [IO.File]::ReadAllText($persistedConfig)
    Assert-True (-not $published.Contains('PasswordPlain') -and -not $published.Contains('CreateUser')) 'Repair metadata must not persist passwords or account-creation authority'

    $script:registration = $null
    $invalid = $validConfiguration.Clone()
    $invalid.EnablePasswordAuth = 'false'
    $rejected = $false
    try { Invoke-TestRepairInstallation -Configuration $invalid } catch { $rejected = $true }
    Assert-True $rejected 'Invalid replacement repair configuration must be rejected'
    Assert-True ($null -eq $script:registration) 'Invalid configuration must not register a startup task'
    Assert-True ([IO.File]::ReadAllText($persistedConfig) -eq $published) 'Invalid updates must preserve the last valid repair configuration'

    # A failed media copy must preserve the installed helper and existing task.
    $previousHelper = [Convert]::ToBase64String([IO.File]::ReadAllBytes($persistedHelper))
    $script:failAssetCopy = $true
    $script:registration = $null
    function Copy-Item {
        [CmdletBinding()]
        param([string]$LiteralPath, [string]$Destination, [switch]$Force)
        if ($script:failAssetCopy -and [IO.Path]::GetFileName($LiteralPath) -eq 'provision-windows-ssh.ps1') {
            [IO.File]::WriteAllText($Destination, 'interrupted-partial-copy')
            throw 'Simulated media copy failure'
        }
        Microsoft.PowerShell.Management\Copy-Item -LiteralPath $LiteralPath -Destination $Destination -Force:$Force
    }
    $rejected = $false
    try { Invoke-TestRepairInstallation -Configuration $validConfiguration } catch { $rejected = $true }
    Assert-True $rejected 'Interrupted asset copies must fail the update'
    Assert-True ($null -eq $script:registration) 'Interrupted asset copies must not replace the startup task'
    Assert-True ([Convert]::ToBase64String([IO.File]::ReadAllBytes($persistedHelper)) -eq $previousHelper) 'Interrupted asset copies must preserve the installed helper bytes'
    Assert-True ([IO.File]::ReadAllText($persistedConfig) -eq $published) 'Interrupted asset copies must preserve the installed repair configuration'
    Assert-True (@(Get-ChildItem -LiteralPath $persistedRoot -Filter '*.tmp').Count -eq 0) 'Interrupted asset copies must remove only their temporary partial file'
    $script:failAssetCopy = $false

    $script:protectionFailure = $true
    $script:registration = $null
    $rejected = $false
    try { Invoke-TestRepairInstallation -Configuration $validConfiguration } catch { $rejected = $true }
    Assert-True ($rejected -and $null -eq $script:registration) 'Protection failure must stop task registration'
    $script:protectionFailure = $false
    $script:fileProtectionFailure = $true
    $script:registration = $null
    $rejected = $false
    try { Invoke-TestRepairInstallation -Configuration $validConfiguration } catch { $rejected = $true }
    Assert-True ($rejected -and $null -eq $script:registration) 'File ACL failure must stop task registration'
    $script:fileProtectionFailure = $false

    # Run the actual entrypoint's repair-selection branch, excluding all service
    # and account mutation that follows it. Prior success must never suppress repair.
    $lastFunction = $ast.EndBlock.Statements |
        Where-Object { $_ -is [Management.Automation.Language.FunctionDefinitionAst] } |
        Select-Object -Last 1
    $repairBranch = $ast.Find({ param($node)
        $node -is [Management.Automation.Language.IfStatementAst] -and
        $node.Extent.StartOffset -gt $lastFunction.Extent.EndOffset -and
        $node.Clauses[0].Item1.Extent.Text -eq '$StartupRepair'
    }, $true)
    Assert-True ($null -ne $repairBranch) 'Startup repair dispatch branch is missing'
    $repairCode = 'param([switch]$StartupRepair,[switch]$RegisterOnly)' + "`n" +
        '$PSScriptRoot=$env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA; $CreateUser=$true; $PasswordPlain="sentinel"; $GrantAdministrators=$true' + "`n" +
        $repairBranch.Extent.Text + "`n" +
        '[pscustomobject]@{UserName=$UserName;AuthorizedKey=$AuthorizedKey;CreateUser=$CreateUser;PasswordPlain=$PasswordPlain;GrantAdministrators=$GrantAdministrators}'
    $invokeRepairBranch = [scriptblock]::Create($repairCode)
    $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA = $repairFixture
    $script:recursiveInstallCalls = 0
    function Install-GuestProvisioningStartupTask {
        param([hashtable]$Configuration)
        $script:recursiveInstallCalls++
        throw 'Startup repair must not install its own task'
    }
    Set-Content -LiteralPath (Join-Path $repairFixture 'previous-success.ok') -Value 'success'
    $repaired = & $invokeRepairBranch -StartupRepair
    Assert-True ($repaired.UserName -eq 'fixture-user' -and -not $repaired.CreateUser -and $repaired.PasswordPlain -eq '' -and -not $repaired.GrantAdministrators) 'Startup repair must discard account-creation, password, and elevation changes'
    $changed = $validConfiguration.Clone()
    $changed.AuthorizedKey = $existingKey
    $changed | ConvertTo-Json | Set-Content -LiteralPath $repairConfig
    $repaired = & $invokeRepairBranch -StartupRepair
    Assert-True ($repaired.AuthorizedKey -eq $existingKey -and $script:recursiveInstallCalls -eq 0) 'Repair must read current configuration even after prior success, without task recursion'
    Remove-Item -LiteralPath $repairConfig
    $rejected = $false
    try { & $invokeRepairBranch -StartupRepair | Out-Null } catch { $rejected = $true }
    Assert-True ($rejected -and $script:recursiveInstallCalls -eq 0) 'Missing configuration must fail without falling back to account creation or recursive installation'
    # Public progress has its own ACL boundary; private assets remain protected
    # by the existing repair helpers tested above.
    $actualStatusDirectoryProtection = (Get-Item Function:\Protect-GuestToolsStatusDirectory).ScriptBlock
    $actualStatusFileProtection = (Get-Item Function:\Protect-GuestToolsStatusFile).ScriptBlock
    $statusFixture = Join-Path $testRoot 'status-fixture'
    New-Item -ItemType Directory -Path $statusFixture | Out-Null
    $env:ProgramData = $statusFixture
    $script:statusAclCalls = @()
    $script:statusAclFailure = $false
    function Protect-GuestToolsStatusDirectory {
        param([string]$Path, [bool]$InheritUserRead = $true)
        Assert-GuestToolsStatusPath -Path $Path -Directory $true
        $script:statusAclCalls += [pscustomobject]@{ Path = $Path; Directory = $true; InheritUserRead = $InheritUserRead }
        New-Item -ItemType Directory -Force -Path $Path | Out-Null
    }
    function Protect-GuestToolsStatusFile {
        param([string]$Path)
        Assert-GuestToolsStatusPath -Path $Path -Directory $false
        if ($script:statusAclFailure) { throw 'Simulated public-status ACL failure' }
        $script:statusAclCalls += [pscustomobject]@{ Path = $Path; Directory = $false }
    }
    $script:GuestToolsPhase = 'preparing'
    $script:GuestToolsSshReady = $false
    $script:GuestToolsAgentReady = $false
    $script:GuestToolsStartupRegistered = $false
    Initialize-GuestToolsStatusStorage
    Assert-True (-not $script:statusAclCalls[0].InheritUserRead -and $script:statusAclCalls[1].InheritUserRead) 'Public product-root read access must not inherit into private config/key/helper storage'
    $publicStatusRoot = Join-Path $env:ProgramData 'Dockpipe/GuestToolsStatus'
    $publicStatusPath = Join-Path $publicStatusRoot 'status.json'
    Write-GuestToolsStatus -State provisioning -Phase capability
    $publicStatus = Get-Content -Raw -LiteralPath $publicStatusPath | ConvertFrom-Json
    Assert-True ($publicStatus.schema -eq 1 -and @($publicStatus.PSObject.Properties).Count -eq 8) 'Public progress must use exactly the schema-1 fields'
    Assert-True ($publicStatus.state -eq 'provisioning' -and $publicStatus.phase -eq 'capability') 'Progress must report the actual provisioning phase'
    Assert-True ($publicStatus.ssh -is [bool] -and $publicStatus.agent -is [bool] -and $publicStatus.startup_repair_registered -is [bool]) 'Public health flags must be booleans'
    Assert-True ((Get-Item -LiteralPath $publicStatusPath).Length -le 65536) 'Public JSON status must remain bounded to 64 KiB'
    $null = [DateTime]::Parse($publicStatus.updated_at)
    $script:GuestToolsSshReady = $true
    $script:GuestToolsAgentReady = $true
    $script:GuestToolsStartupRegistered = $true
    Write-GuestToolsStatus -State ready -Phase ready
    $previousPublicStatus = [IO.File]::ReadAllText($publicStatusPath)
    $rejected = $false
    try { Write-GuestToolsStatus -State provisioning -Phase ('x' * 65) } catch { $rejected = $true }
    Assert-True $rejected 'Unknown or oversized status phases must fail before publication'
    Assert-True ([IO.File]::ReadAllText($publicStatusPath) -eq $previousPublicStatus) 'Rejected progress must preserve the last valid public status'
    $script:statusAclFailure = $true
    $rejected = $false
    try { Write-GuestToolsStatus -State failed -Phase health } catch { $rejected = $true }
    Assert-True $rejected 'Status ACL failure must prevent publication'
    Assert-True ([IO.File]::ReadAllText($publicStatusPath) -eq $previousPublicStatus) 'Status ACL failure must preserve the previous document'
    Assert-True (@(Get-ChildItem -LiteralPath $publicStatusRoot -Filter '*.tmp').Count -eq 0) 'Status failures must remove their temporary file'
    $script:statusAclFailure = $false
    $writeStepDefinition = $ast.Find({ param($node)
        $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Write-Step'
    }, $true)
    . ([scriptblock]::Create($writeStepDefinition.Extent.Text))
    Write-Step -Phase account -Message 'sentinel-private-account-and-key-details'
    Assert-True (-not ([IO.File]::ReadAllText($publicStatusPath)).Contains('sentinel-private')) 'Detailed console messages must never flow into the public status'
    Write-GuestToolsStatus -State failed -Phase account
    $publicStatus = Get-Content -Raw -LiteralPath $publicStatusPath | ConvertFrom-Json
    Assert-True ($publicStatus.state -eq 'failed' -and $publicStatus.message -like 'Guest service setup failed.*') 'Failures must publish a fixed safe message'
    $rejected = $false
    try { & $actualStatusDirectoryProtection -Path $publicStatusPath } catch { $rejected = $true }
    Assert-True $rejected 'Status directories must reject ordinary files before touching ACLs'
    $rejected = $false
    try { & $actualStatusFileProtection -Path $publicStatusRoot } catch { $rejected = $true }
    Assert-True $rejected 'Status files must reject directories before touching ACLs'
    if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
        $redirectedStatusRoot = Join-Path $statusFixture 'redirected-status'
        New-Item -ItemType SymbolicLink -Path $redirectedStatusRoot -Target $publicStatusRoot | Out-Null
        $rejected = $false
        try { Assert-GuestToolsStatusPath -Path (Join-Path $redirectedStatusRoot 'status.json') -Directory $false } catch { $rejected = $true }
        Assert-True $rejected 'Status must reject symlinks in any ancestor path'
    } else {
        # Native Windows also proves the actual ACL identities and access masks.
        & $actualStatusDirectoryProtection -Path $publicStatusRoot
        & $actualStatusFileProtection -Path $publicStatusPath
        foreach ($path in @($publicStatusRoot, $publicStatusPath)) {
            $acl = Get-Acl -LiteralPath $path
            Assert-True ($acl.AreAccessRulesProtected) 'Public status ACLs must disable inherited access'
            $rules = @($acl.GetAccessRules($true, $true, [Security.Principal.SecurityIdentifier]))
            $userRule = @($rules | Where-Object { $_.IdentityReference.Value -eq 'S-1-5-32-545' })
            Assert-True ($userRule.Count -eq 1) 'Users must receive one explicit read-only status ACE'
            $writeMask = [Security.AccessControl.FileSystemRights]::Write -bor [Security.AccessControl.FileSystemRights]::Delete -bor [Security.AccessControl.FileSystemRights]::ChangePermissions -bor [Security.AccessControl.FileSystemRights]::TakeOwnership
            Assert-True (($userRule[0].FileSystemRights -band $writeMask) -eq 0) 'Users must not modify public status or its ACL'
        }
    }

    # Health is based on live services/tasks and bounded loopback response, not
    # a success marker or merely the registration API returning successfully.
    $script:healthStatusWrites = @()
    function Write-GuestToolsStatus {
        param([string]$State, [string]$Phase)
        $script:healthStatusWrites += [pscustomobject]@{ State = $State; Phase = $Phase }
    }
    $script:healthAgentTaskState = 'Running'
    $script:healthAgentHealthy = $true
    $script:healthSshState = 'Running'
    $script:healthRepairRegistered = $true
    $script:healthRequests = 0
    $script:healthSleeps = 0
    function Get-Service {
        [CmdletBinding()]
        param([string]$Name)
        return [pscustomobject]@{ Status = $script:healthSshState }
    }
    function Get-ScheduledTask {
        [CmdletBinding()]
        param([string]$TaskName)
        if ($TaskName -eq 'DockPipeGuestProvisioning') {
            if ($script:healthRepairRegistered) { return [pscustomobject]@{ State = 'Running' } }
            return $null
        }
        Assert-True ($TaskName -eq 'DockPipeGuestAgent') 'Health checks must use the stable guest-agent task identity'
        return [pscustomobject]@{ State = $script:healthAgentTaskState }
    }
    function Test-GuestToolsAgentHealth {
        param([int]$Port)
        Assert-True ($Port -eq 47831) 'Health must probe the configured local-only agent port'
        $script:healthRequests++
        return $script:healthAgentHealthy
    }
    function Start-Sleep {
        param([int]$Seconds)
        Assert-True ($Seconds -eq 1) 'Health retries must use the declared bounded delay'
        $script:healthSleeps++
    }
    Wait-GuestToolsHealth -AgentRequested $true -Port 47831
    Assert-True ($script:GuestToolsSshReady -and $script:GuestToolsAgentReady -and $script:GuestToolsStartupRegistered) 'Ready requires SSH, loopback agent health, and registered boot repair'
    Assert-True ($script:healthRequests -eq 1 -and $script:healthSleeps -eq 0) 'Healthy services should finish without retrying'
    $script:healthAgentHealthy = $false
    $script:healthRequests = 0
    $script:healthSleeps = 0
    $rejected = $false
    try { Wait-GuestToolsHealth -AgentRequested $true -Port 47831 } catch { $rejected = $true }
    Assert-True ($rejected -and $script:healthRequests -eq 15 -and $script:healthSleeps -eq 14) 'Unhealthy agents must exhaust exactly the finite polling contract'
    Assert-True (-not $script:GuestToolsAgentReady) 'An agent task alone must not report healthy'
    $script:healthRequests = 0
    Wait-GuestToolsHealth -AgentRequested $false -Port 47831
    Assert-True ($script:healthRequests -eq 0 -and -not $script:GuestToolsAgentReady) 'Disabled agents must not be probed or reported as healthy'
    $script:healthRepairRegistered = $false
    $rejected = $false
    try { Wait-GuestToolsHealth -AgentRequested $false -Port 47831 } catch { $rejected = $true }
    Assert-True $rejected 'SSH running without durable startup repair must not report ready'
    $script:healthRepairRegistered = $true

    # Execute the actual guarded entrypoint with mocked operating-system
    # boundaries; RegisterOnly must queue SYSTEM work and return before setup.
    $entrypoint = $ast.EndBlock.Statements |
        Where-Object { $_ -is [Management.Automation.Language.TryStatementAst] } |
        Select-Object -Last 1
    $entryCode = $ast.ParamBlock.Extent.Text + "`n" +
        '$PSScriptRoot=$env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA' + "`n" +
        (($defaultStatements | ForEach-Object { $_.Extent.Text }) -join "`n") + "`n" +
        $entrypoint.Extent.Text
    $invokeEntry = [scriptblock]::Create($entryCode)
    $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA = Join-Path $testRoot 'entry-media'
    New-Item -ItemType Directory -Path $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA | Out-Null
    $script:entryEvents = @()
    $script:entryStatuses = @()
    $script:entryConfiguration = $null
    $script:entryCapabilityFailure = $false
    $script:entryAllowProvisioning = $false
    $script:entryRegistrationFailure = $false
    function Assert-Administrator { $script:entryEvents += 'admin' }
    function Initialize-GuestToolsStatusStorage { $script:GuestToolsStatusInitialized = $true }
    function Write-GuestToolsStatus {
        param([string]$State, [string]$Phase)
        $script:entryStatuses += [pscustomobject]@{ State = $State; Phase = $Phase; Ssh = $script:GuestToolsSshReady; Agent = $script:GuestToolsAgentReady; Registered = $script:GuestToolsStartupRegistered }
        $script:entryEvents += $State + ':' + $Phase
    }
    function Install-GuestProvisioningStartupTask {
        param([hashtable]$Configuration)
        $script:entryEvents += 'register'
        $script:entryConfiguration = $Configuration.Clone()
        if ($script:entryRegistrationFailure) { throw 'sentinel-private-registration-error' }
    }
    function Start-ScheduledTask {
        [CmdletBinding()]
        param([string]$TaskName)
        Assert-True ($TaskName -eq 'DockPipeGuestProvisioning') 'RegisterOnly must start the independent SYSTEM provisioning task'
        $script:entryEvents += 'start-task'
    }
    function Ensure-OpenSSHServer {
        $script:entryEvents += 'capability'
        if ($script:entryCapabilityFailure) { throw 'sentinel-private-capability-error' }
        if (-not $script:entryAllowProvisioning) { throw 'RegisterOnly must return before provisioning Windows services' }
    }
    $rejected = $false
    try { & $invokeEntry -RegisterOnly -StartupRepair } catch { $rejected = $true }
    Assert-True ($rejected -and $script:entryStatuses[-1].State -eq 'failed') 'Conflicting entrypoint modes must fail safely before task dispatch'
    $badBootstrapKey = Join-Path $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA 'dockpipe-guest-ssh.pub'
    [IO.File]::WriteAllText($badBootstrapKey, ('x' * 16385))
    $rejected = $false
    try { & $invokeEntry -RegisterOnly } catch { $rejected = $true }
    Assert-True ($rejected -and $script:entryStatuses[-1].State -eq 'failed' -and $script:entryStatuses[-1].Phase -eq 'preparing') 'Bootstrap validation failures must publish a safe failed state before registration'
    Remove-Item -LiteralPath $badBootstrapKey
    & $invokeEntry -RegisterOnly -UserName 'original-user' -CreateUser $false -EnablePasswordAuth $false
    Assert-True ($script:entryConfiguration.UserName -eq 'original-user' -and -not $script:entryConfiguration.EnablePasswordAuth) 'First registration must preserve explicit non-secret identity/authentication inputs'
    Assert-True ($script:entryEvents[-1] -eq 'start-task' -and $script:entryEvents -notcontains 'capability') 'RegisterOnly must queue the task and return without coupling provisioning to the installer process'
    Assert-True ($script:entryStatuses[-1].State -eq 'provisioning' -and $script:entryStatuses[-1].Phase -eq 'queued') 'RegisterOnly must publish queued progress before the worker starts'
    $existingRoot = Join-Path $env:ProgramData 'Dockpipe/GuestProvisioning'
    New-Item -ItemType Directory -Path $existingRoot | Out-Null
    $validConfiguration | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $existingRoot 'configuration.json')
    & $invokeEntry -RegisterOnly -UserName 'wrong-admin' -AuthorizedKey 'wrong-key'
    Assert-True ($script:entryConfiguration.UserName -eq $validConfiguration.UserName -and $script:entryConfiguration.AuthorizedKey -eq $validConfiguration.AuthorizedKey) 'RegisterOnly must reuse the existing durable identity/key when repairing a missing task'
    $script:entryRegistrationFailure = $true
    $rejected = $false
    try { & $invokeEntry -RegisterOnly } catch { $rejected = $true }
    Assert-True ($rejected -and $script:entryStatuses[-1].State -eq 'failed') 'Registration failures must publish failed and rethrow'
    Assert-True ($script:entryStatuses[-1].Phase -eq 'registering') 'Failure must retain the real phase rather than fake completion'
    $script:entryRegistrationFailure = $false
    $startupConfiguration = $validConfiguration.Clone()
    $startupConfiguration.DefaultShell = ''
    $startupConfiguration.AuthorizedKey = ''
    $startupConfiguration | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA 'configuration.json')
    $script:entryCapabilityFailure = $true
    $rejected = $false
    try { & $invokeEntry -StartupRepair } catch { $rejected = $true }
    Assert-True ($rejected -and $script:entryStatuses[-1].State -eq 'failed') 'Startup repair failures must publish failed and rethrow'
    Assert-True (@($script:entryStatuses | Where-Object State -eq 'ready').Count -eq 0) 'Neither queued nor failed runs may publish ready'
    Assert-True ($script:entryStatuses[-1].Phase -eq 'capability') 'Startup failure must retain the actual capability phase'
    $script:entryCapabilityFailure = $false
    $script:entryAllowProvisioning = $true
    $script:healthAgentHealthy = $true
    $script:entryEvents = @()
    function Ensure-LocalUser { throw 'Startup repair must not create or reset accounts' }
    function Ensure-SshConfigFile { return (Join-Path $statusFixture 'sshd_config') }
    function Set-SshdOption { }
    function Ensure-SshFirewallRule { $script:entryEvents += 'firewall' }
    function Ensure-AuthorizedKey { throw 'Empty startup public key must not be replaced' }
    function Ensure-DockPipeAgent { $script:entryEvents += 'agent' }
    function Start-ProvisionedSshService { $script:entryEvents += 'sshd' }
    function Set-Service {
        param([string]$Name, [string]$StartupType)
        Assert-True ($Name -eq 'ssh-agent' -and $StartupType -eq 'Manual') 'Only the existing ssh-agent startup contract may be configured after SSH starts'
    }
    & $invokeEntry -StartupRepair
    Assert-True ($script:entryEvents -notcontains 'register' -and $script:entryEvents -notcontains 'start-task') 'A normal boot repair must not recursively register or restart itself'
    Assert-True ($script:entryStatuses[-1].State -eq 'ready' -and $script:entryStatuses[-1].Ssh -and $script:entryStatuses[-1].Agent -and $script:entryStatuses[-1].Registered) 'Ready may publish only after the entrypoint verifies all requested live services'
    $script:healthAgentHealthy = $false
    $rejected = $false
    try { & $invokeEntry -StartupRepair } catch { $rejected = $true }
    Assert-True ($rejected -and $script:entryStatuses[-1].State -eq 'failed' -and $script:entryStatuses[-1].Phase -eq 'health') 'Failed live health must publish failed instead of leaving a successful status'

    Write-Host 'Windows guest-service, durable startup repair, progress, RegisterOnly, key, and service lifecycle tests passed' 

} finally {
    $env:ProgramData = $oldProgramData
    $env:SystemDrive = $oldSystemDrive
    $env:COMPUTERNAME = $oldComputerName
    $env:USERNAME = $oldUsername
    $env:WINDIR = $oldWindowsDirectory
    $env:DOCKPIPE_TEST_BOOTSTRAP_MEDIA = $oldBootstrapMedia
    Remove-Item -LiteralPath $testRoot -Recurse -Force
}

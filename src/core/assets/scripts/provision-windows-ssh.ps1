param(
  [string]$UserName = "dockpipe",
  [string]$PasswordPlain = "",
  [string]$AuthorizedKey = "",
  [bool]$CreateUser = $true,
  [bool]$GrantAdministrators = $false,
  [bool]$EnablePasswordAuth = $true,
  [bool]$EnablePublicKeyAuth = $true,
  [string]$DefaultShell = "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe",
  [bool]$InstallDockPipeAgent = $true,
  [int]$AgentPort = 47831,
  [switch]$StartupRepair,
  [switch]$RegisterOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:GuestToolsStatusInitialized = $false
$script:GuestToolsPhase = 'preparing'
$script:GuestToolsSshReady = $false
$script:GuestToolsAgentReady = $false
$script:GuestToolsStartupRegistered = $false

function Write-Step {
  param([string]$Message, [string]$Phase = '')
  Write-Host "[dockpipe windows-vm] $Message"
  if (-not [string]::IsNullOrWhiteSpace($Phase)) {
    $script:GuestToolsPhase = $Phase
  }
  if ($script:GuestToolsStatusInitialized) {
    # Console details can contain account names and local paths. Public progress
    # uses only fixed phase messages, never keys, passwords, or exception text.
    Write-GuestToolsStatus -State provisioning -Phase $script:GuestToolsPhase
  }
}

function Assert-GuestToolsStatusPath {
  param([string]$Path, [bool]$Directory)

  $current = [IO.Path]::GetFullPath($Path)
  $leaf = $true
  while (-not [string]::IsNullOrWhiteSpace($current)) {
    if (Test-Path -LiteralPath $current) {
      $item = Get-Item -LiteralPath $current -Force -ErrorAction Stop
      if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or
          ($leaf -and $item.PSIsContainer -ne $Directory) -or
          (-not $leaf -and -not $item.PSIsContainer)) {
        throw 'Guest tools status storage must use ordinary paths.'
      }
    }
    $parent = [IO.Directory]::GetParent($current)
    if ($null -eq $parent) { break }
    $current = $parent.FullName
    $leaf = $false
  }
}

function Protect-GuestToolsStatusDirectory {
  param([string]$Path, [bool]$InheritUserRead = $true)

  Assert-GuestToolsStatusPath -Path $Path -Directory $true
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path | Out-Null
  }
  $administrators = [Security.Principal.SecurityIdentifier]::new('S-1-5-32-544')
  $system = [Security.Principal.SecurityIdentifier]::new('S-1-5-18')
  $users = [Security.Principal.SecurityIdentifier]::new('S-1-5-32-545')
  $security = [Security.AccessControl.DirectorySecurity]::new()
  $security.SetOwner($administrators)
  $security.SetAccessRuleProtection($true, $false)
  foreach ($identity in @($administrators, $system)) {
    $security.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new(
      $identity, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow'))
  }
  $inheritance = if ($InheritUserRead) { 'ContainerInherit,ObjectInherit' } else { 'None' }
  $security.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new(
    $users, 'ReadAndExecute', $inheritance, 'None', 'Allow'))
  Set-Acl -LiteralPath $Path -AclObject $security
}

function Protect-GuestToolsStatusFile {
  param([string]$Path)

  Assert-GuestToolsStatusPath -Path $Path -Directory $false
  $file = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
  $administrators = [Security.Principal.SecurityIdentifier]::new('S-1-5-32-544')
  $system = [Security.Principal.SecurityIdentifier]::new('S-1-5-18')
  $users = [Security.Principal.SecurityIdentifier]::new('S-1-5-32-545')
  $security = [Security.AccessControl.FileSecurity]::new()
  $security.SetOwner($administrators)
  $security.SetAccessRuleProtection($true, $false)
  foreach ($identity in @($administrators, $system)) {
    $security.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($identity, 'FullControl', 'Allow'))
  }
  $security.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($users, 'Read', 'Allow'))
  Set-Acl -LiteralPath $Path -AclObject $security
  $file.IsReadOnly = $false
}

function Initialize-GuestToolsStatusStorage {
  $productRoot = Join-Path $env:ProgramData 'Dockpipe'
  $statusRoot = Join-Path $productRoot 'GuestToolsStatus'
  # Users may traverse/read the product directory, but this ACE does not inherit
  # into private provisioning/config/key storage. Those ACLs remain unchanged.
  Protect-GuestToolsStatusDirectory -Path $productRoot -InheritUserRead $false
  Protect-GuestToolsStatusDirectory -Path $statusRoot
  $script:GuestToolsStatusInitialized = $true
}

function Write-GuestToolsStatus {
  param(
    [ValidateSet('provisioning', 'ready', 'failed')][string]$State,
    [string]$Phase
  )

  $messages = @{
    preparing = 'Preparing Windows guest services.'
    registering = 'Saving automatic startup repair.'
    queued = 'Automatic repair has been queued.'
    capability = 'Checking or installing Windows OpenSSH Server.'
    account = 'Checking the requested local account.'
    ssh_configuration = 'Configuring Windows OpenSSH.'
    firewall = 'Repairing the SSH firewall rule.'
    authorized_key = 'Configuring SSH public-key access.'
    agent = 'Installing and starting the guest agent.'
    ssh_service = 'Starting the SSH service.'
    health = 'Checking SSH and guest-agent health.'
    ready = 'Windows guest services are ready.'
  }
  if ($Phase.Length -gt 64 -or -not $messages.ContainsKey($Phase)) {
    throw 'Unknown guest tools progress phase.'
  }
  $message = if ($State -eq 'failed') {
    'Guest service setup failed. Run repair again or inspect the administrator logs.'
  } else {
    $messages[$Phase]
  }
  $status = [ordered]@{
    schema = 1
    state = $State
    phase = $Phase
    message = $message
    updated_at = [DateTime]::UtcNow.ToString('o')
    ssh = [bool]$script:GuestToolsSshReady
    agent = [bool]$script:GuestToolsAgentReady
    startup_repair_registered = [bool]$script:GuestToolsStartupRegistered
  }
  $bytes = [Text.Encoding]::UTF8.GetBytes(($status | ConvertTo-Json -Compress))
  if ($message.Length -gt 512 -or $bytes.Length -gt 65536) {
    throw 'Guest tools progress exceeds its size contract.'
  }
  $root = Join-Path (Join-Path $env:ProgramData 'Dockpipe') 'GuestToolsStatus'
  Assert-GuestToolsStatusPath -Path $root -Directory $true
  $path = Join-Path $root 'status.json'
  Assert-GuestToolsStatusPath -Path $path -Directory $false
  $temporary = Join-Path $root ([guid]::NewGuid().ToString('N') + '.tmp')
  try {
    [IO.File]::WriteAllBytes($temporary, $bytes)
    Protect-GuestToolsStatusFile -Path $temporary
    if (Test-Path -LiteralPath $path) {
      Protect-GuestToolsStatusFile -Path $path
      # A GUI reader may briefly deny delete sharing while reading one bounded
      # status document. Retry only that transient Windows sharing violation.
      for ($attempt = 0; $attempt -lt 3; $attempt++) {
        try {
          [IO.File]::Replace($temporary, $path, [NullString]::Value)
          break
        } catch [IO.IOException] {
          $windowsError = $_.Exception.HResult -band 0xffff
          if ($attempt -eq 2 -or $windowsError -notin @(32, 33)) { throw }
          Start-Sleep -Milliseconds 100
        }
      }
    } else {
      [IO.File]::Move($temporary, $path)
    }
    Protect-GuestToolsStatusFile -Path $path
  } finally {
    if (Test-Path -LiteralPath $temporary) {
      Remove-Item -LiteralPath $temporary -Force
    }
  }
}

function Test-GuestToolsAgentHealth {
  param([int]$Port)

  # Buffering, headers, and elapsed request time are bounded independently. This
  # endpoint is local-only and must never redirect to another host or use a proxy.
  Add-Type -AssemblyName System.Net.Http
  $handler = [Net.Http.HttpClientHandler]::new()
  $handler.UseProxy = $false
  $handler.AllowAutoRedirect = $false
  $handler.MaxResponseHeadersLength = 16
  $client = [Net.Http.HttpClient]::new($handler)
  $client.Timeout = [TimeSpan]::FromSeconds(2)
  $client.MaxResponseContentBufferSize = 4096
  try {
    $body = $client.GetStringAsync("http://127.0.0.1:$Port/health").GetAwaiter().GetResult()
    $health = $body | ConvertFrom-Json
    return $health.name -eq 'dockpipe-guest-agent' -and $health.status -eq 'ready'
  } catch {
    return $false
  } finally {
    $client.Dispose()
  }
}

function Wait-GuestToolsHealth {
  param([bool]$AgentRequested, [int]$Port)

  # At most 15 local HTTP requests (2 seconds each) and 14 one-second delays.
  for ($attempt = 0; $attempt -lt 15; $attempt++) {
    $sshd = Get-Service -Name sshd -ErrorAction SilentlyContinue
    $script:GuestToolsSshReady = $null -ne $sshd -and $sshd.Status -eq 'Running'
    $repairTask = Get-ScheduledTask -TaskName 'DockPipeGuestProvisioning' -ErrorAction SilentlyContinue
    $script:GuestToolsStartupRegistered = $null -ne $repairTask
    $script:GuestToolsAgentReady = $false
    if ($AgentRequested) {
      $agentTask = Get-ScheduledTask -TaskName 'DockPipeGuestAgent' -ErrorAction SilentlyContinue
      if ($null -ne $agentTask -and $agentTask.State -eq 'Running') {
        $script:GuestToolsAgentReady = Test-GuestToolsAgentHealth -Port $Port
      }
    }
    Write-GuestToolsStatus -State provisioning -Phase health
    if ($script:GuestToolsSshReady -and $script:GuestToolsStartupRegistered -and
        (-not $AgentRequested -or $script:GuestToolsAgentReady)) {
      return
    }
    if ($attempt -lt 14) { Start-Sleep -Seconds 1 }
  }
  throw 'Windows guest services did not become healthy within the bounded startup wait.'
}

function Assert-Administrator {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = [Security.Principal.WindowsPrincipal]::new($identity)
  if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Run this script from an elevated PowerShell session."
  }
}

function Ensure-OpenSSHServer {
  $capability = Get-WindowsCapability -Online | Where-Object Name -like "OpenSSH.Server*"
  if ($null -eq $capability) {
    throw "OpenSSH.Server capability was not found on this Windows image."
  }
  if ($capability.State -ne "Installed") {
    Write-Step "Installing OpenSSH Server capability"
    Add-WindowsCapability -Online -Name $capability.Name | Out-Null
  } else {
    Write-Step "OpenSSH Server capability already installed"
  }
}

function Read-StartupRepairConfiguration {
  param([string]$Path)

  $file = Get-Item -LiteralPath $Path -ErrorAction Stop
  if ($file.Length -gt 32768 -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw "Invalid durable guest provisioning configuration."
  }
  $configuration = Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json
  $fields = @('SchemaVersion', 'UserName', 'AuthorizedKey', 'DefaultShell',
    'EnablePasswordAuth', 'EnablePublicKeyAuth', 'InstallDockPipeAgent', 'AgentPort')
  if (@($configuration.PSObject.Properties).Count -ne $fields.Count) {
    throw "Invalid guest provisioning configuration fields."
  }
  foreach ($property in $configuration.PSObject.Properties) {
    if ($property.Name -cnotin $fields) {
      throw "Unknown guest provisioning configuration field: $($property.Name)"
    }
  }
  if (($configuration.SchemaVersion -isnot [int] -and $configuration.SchemaVersion -isnot [long]) -or
      $configuration.SchemaVersion -ne 1) {
    throw "Unsupported guest provisioning configuration version."
  }
  foreach ($name in @('UserName', 'AuthorizedKey', 'DefaultShell')) {
    if ($configuration.$name -isnot [string]) {
      throw "Invalid guest provisioning configuration field: $name"
    }
  }
  if ([string]::IsNullOrWhiteSpace($configuration.UserName) -or
      $configuration.UserName.Length -gt 256 -or
      $configuration.AuthorizedKey.Length -gt 16384 -or
      $configuration.AuthorizedKey.Contains("`r") -or
      $configuration.AuthorizedKey.Contains("`n")) {
    throw "Invalid durable guest provisioning identity."
  }
  foreach ($name in @('EnablePasswordAuth', 'EnablePublicKeyAuth', 'InstallDockPipeAgent')) {
    if ($configuration.$name -isnot [bool]) {
      throw "Invalid guest provisioning configuration field: $name"
    }
  }
  if ($configuration.AgentPort -isnot [int] -and $configuration.AgentPort -isnot [long]) {
    throw "Invalid guest provisioning agent port."
  }
  if ($configuration.AgentPort -lt 1 -or $configuration.AgentPort -gt 65535) {
    throw "Invalid guest provisioning agent port."
  }
  return $configuration
}

function Protect-GuestProvisioningDirectory {
  param([string]$Path)

  if (Test-Path -LiteralPath $Path) {
    $directory = Get-Item -LiteralPath $Path -Force
    if (-not $directory.PSIsContainer -or ($directory.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
      throw "Guest provisioning storage must be an ordinary directory."
    }
  } else {
    New-Item -ItemType Directory -Path $Path | Out-Null
  }
  $administrators = [Security.Principal.SecurityIdentifier]::new('S-1-5-32-544')
  $system = [Security.Principal.SecurityIdentifier]::new('S-1-5-18')
  $security = [Security.AccessControl.DirectorySecurity]::new()
  $security.SetOwner($administrators)
  $security.SetAccessRuleProtection($true, $false)
  foreach ($identity in @($administrators, $system)) {
    $rule = [Security.AccessControl.FileSystemAccessRule]::new(
      $identity, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
    $security.AddAccessRule($rule)
  }
  Set-Acl -LiteralPath $Path -AclObject $security
}

function Install-GuestProvisioningStartupTask {
  param([hashtable]$Configuration)

  # Persist before capability installation, so an interrupted setup can recover
  # at the next boot without the original removable media or a user login.
  $productRoot = Join-Path $env:ProgramData 'Dockpipe'
  $repairRoot = Join-Path $productRoot 'GuestProvisioning'
  Protect-GuestProvisioningDirectory -Path $productRoot
  Protect-GuestProvisioningDirectory -Path $repairRoot
  $helperPath = Join-Path $repairRoot 'provision-windows-ssh.ps1'
  $configPath = Join-Path $repairRoot 'configuration.json'
  $sources = @($PSCommandPath)
  if ($Configuration.InstallDockPipeAgent) {
    $sources += Resolve-DockPipeAgentSourcePath
  }
  foreach ($source in $sources) {
    $destination = Join-Path $repairRoot ([IO.Path]::GetFileName($source))
    if ([IO.Path]::GetFullPath($source) -eq [IO.Path]::GetFullPath($destination)) {
      Protect-GuestProvisioningFile -Path $destination
      continue
    }
    $temporaryAsset = Join-Path $repairRoot ([guid]::NewGuid().ToString('N') + '.tmp')
    try {
      Copy-Item -LiteralPath $source -Destination $temporaryAsset
      Protect-GuestProvisioningFile -Path $temporaryAsset
      if (Test-Path -LiteralPath $destination) {
        Protect-GuestProvisioningFile -Path $destination
        [IO.File]::Replace($temporaryAsset, $destination, [NullString]::Value)
      } else {
        [IO.File]::Move($temporaryAsset, $destination)
      }
      Protect-GuestProvisioningFile -Path $destination
    } finally {
      if (Test-Path -LiteralPath $temporaryAsset) {
        Remove-Item -LiteralPath $temporaryAsset -Force
      }
    }
  }
  $temporaryConfig = Join-Path $repairRoot ([guid]::NewGuid().ToString('N') + '.json')
  try {
    $Configuration | ConvertTo-Json | Set-Content -LiteralPath $temporaryConfig -Encoding UTF8
    Read-StartupRepairConfiguration -Path $temporaryConfig | Out-Null
    if (Test-Path -LiteralPath $configPath) {
      if ((Get-Item -LiteralPath $configPath -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "Guest provisioning configuration must not be redirected."
      }
      Protect-GuestProvisioningFile -Path $configPath
      [IO.File]::Replace($temporaryConfig, $configPath, [NullString]::Value)
    } else {
      [IO.File]::Move($temporaryConfig, $configPath)
    }
    Protect-GuestProvisioningFile -Path $configPath
  } finally {
    if (Test-Path -LiteralPath $temporaryConfig) {
      Remove-Item -LiteralPath $temporaryConfig -Force
    }
  }

  $powershell = Join-Path $env:WINDIR 'System32\WindowsPowerShell\v1.0\powershell.exe'
  $action = New-ScheduledTaskAction -Execute $powershell -Argument (
    '-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "{0}" -StartupRepair' -f $helperPath)
  $trigger = New-ScheduledTaskTrigger -AtStartup
  # Time synchronization can move the clock after startup. Recheck services and
  # publish a fresh status instead of leaving the UI with a future timestamp.
  $clockTriggerClass = Get-CimClass -Namespace 'Root/Microsoft/Windows/TaskScheduler' `
    -ClassName 'MSFT_TaskEventTrigger'
  $clockTrigger = New-CimInstance -ClientOnly -CimClass $clockTriggerClass -Property @{
      Enabled = $true
      Delay = 'PT30S'
      Subscription = '<QueryList><Query Id="0" Path="System"><Select Path="System">*[System[Provider[@Name=''Microsoft-Windows-Kernel-General''] and EventID=1]]</Select></Query></QueryList>'
    }
  $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
  # Windows servicing can need several minutes. Each attempt gets 15 minutes;
  # Task Scheduler permits three additional attempts, then reports failure.
  $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -StartWhenAvailable -MultipleInstances IgnoreNew -ExecutionTimeLimit (New-TimeSpan -Minutes 15) `
    -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
  Register-ScheduledTask -TaskName 'DockPipeGuestProvisioning' -Action $action -Trigger @($trigger, $clockTrigger) `
    -Principal $principal -Settings $settings -Force | Out-Null
}

function Protect-GuestProvisioningFile {
  param([string]$Path)

  $file = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
  if ($file.PSIsContainer -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw "Guest provisioning assets must be ordinary files."
  }
  $administrators = [Security.Principal.SecurityIdentifier]::new('S-1-5-32-544')
  $system = [Security.Principal.SecurityIdentifier]::new('S-1-5-18')
  $security = [Security.AccessControl.FileSecurity]::new()
  $security.SetOwner($administrators)
  $security.SetAccessRuleProtection($true, $false)
  foreach ($identity in @($administrators, $system)) {
    $security.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($identity, 'FullControl', 'Allow'))
  }
  Set-Acl -LiteralPath $Path -AclObject $security
  # Copying from optical media preserves ReadOnly. A persisted asset must remain
  # replaceable by its trusted owner during a later installer/repair update.
  $file.IsReadOnly = $false
}

function Ensure-SshConfigFile {
  $sshRoot = Join-Path $env:ProgramData "ssh"
  $configPath = Join-Path $sshRoot "sshd_config"
  if (Test-Path -LiteralPath $configPath) {
    return $configPath
  }

  New-Item -ItemType Directory -Force -Path $sshRoot | Out-Null

  $defaultCandidates = @(
    (Join-Path $env:WINDIR "System32\OpenSSH\sshd_config_default"),
    (Join-Path $env:WINDIR "SysNative\OpenSSH\sshd_config_default"),
    (Join-Path $env:WINDIR "System32\OpenSSH\sshd_config")
  )

  foreach ($candidate in $defaultCandidates) {
    if (Test-Path -LiteralPath $candidate) {
      Write-Step "Bootstrapping sshd_config from $candidate"
      Copy-Item -LiteralPath $candidate -Destination $configPath -Force
      return $configPath
    }
  }

  Write-Step "Bootstrapping minimal sshd_config"
  @(
    "Port 22"
    "PasswordAuthentication yes"
    "PubkeyAuthentication yes"
    "Subsystem sftp sftp-server.exe"
  ) | Set-Content -LiteralPath $configPath
  return $configPath
}

function Ensure-LocalUser {
  param(
    [string]$Name,
    [string]$PlainPassword,
    [bool]$AddToAdministrators
  )

  $user = Get-LocalUser -Name $Name -ErrorAction SilentlyContinue
  if ($null -eq $user) {
    if ([string]::IsNullOrWhiteSpace($PlainPassword)) {
      throw "User '$Name' does not exist. Supply -PasswordPlain to create it."
    }
    Write-Step "Creating local user '$Name'"
    $secure = ConvertTo-SecureString -String $PlainPassword -AsPlainText -Force
    $user = New-LocalUser -Name $Name -Password $secure -PasswordNeverExpires -AccountNeverExpires
  } elseif (-not [string]::IsNullOrWhiteSpace($PlainPassword)) {
    Write-Step "Resetting password for local user '$Name'"
    $secure = ConvertTo-SecureString -String $PlainPassword -AsPlainText -Force
    $user | Set-LocalUser -Password $secure
  } else {
    Write-Step "Local user '$Name' already exists"
  }

  if ($AddToAdministrators) {
    $member = Get-LocalGroupMember -Group "Administrators" -ErrorAction SilentlyContinue | Where-Object Name -match "\\$([regex]::Escape($Name))$"
    if ($null -eq $member) {
      Write-Step "Adding '$Name' to Administrators"
      Add-LocalGroupMember -Group "Administrators" -Member $Name
    }
  }
}

function Ensure-AuthorizedKey {
  param(
    [string]$Name,
    [string]$Key
  )

  if ([string]::IsNullOrWhiteSpace($Key)) {
    return
  }
  if ($Key.Length -gt 16384 -or $Key.Contains("`r") -or $Key.Contains("`n")) {
    throw "AuthorizedKey must contain one public key of at most 16384 characters."
  }

  $user = Get-LocalUser -Name $Name -ErrorAction Stop
  $administrators = @(Get-LocalGroupMember -SID "S-1-5-32-544" -ErrorAction Stop)
  $isAdministrator = @($administrators | Where-Object { $_.SID.Value -eq $user.SID.Value }).Count -gt 0
  if ($isAdministrator) {
    $sshDir = Join-Path $env:ProgramData "ssh"
    $authKeys = Join-Path $sshDir "administrators_authorized_keys"
    $principal = "*S-1-5-32-544"
  } else {
    $profile = Get-ItemProperty -LiteralPath "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList\$($user.SID.Value)" -ErrorAction SilentlyContinue
    if ($null -ne $profile -and -not [string]::IsNullOrWhiteSpace($profile.ProfileImagePath)) {
      $profileRoot = [Environment]::ExpandEnvironmentVariables($profile.ProfileImagePath)
    } else {
      $profileRoot = Join-Path $env:SystemDrive "Users\$Name"
    }
    $sshDir = Join-Path $profileRoot ".ssh"
    $authKeys = Join-Path $sshDir "authorized_keys"
    $principal = "*$($user.SID.Value)"
  }
  New-Item -ItemType Directory -Force -Path $sshDir | Out-Null

  $keys = @()
  if (Test-Path -LiteralPath $authKeys) {
    $keys = @(Get-Content -LiteralPath $authKeys)
  }
  if ($keys -notcontains $Key) {
    Write-Step "Adding authorized key for '$Name'"
    Add-Content -LiteralPath $authKeys -Value $Key -Encoding ascii
  }

  if (-not $isAdministrator) {
    & icacls $sshDir /inheritance:r /grant:r "${principal}:(OI)(CI)F" "*S-1-5-18:(OI)(CI)F" | Out-Null
    if ($LASTEXITCODE -ne 0) {
      throw "Failed to restrict the SSH directory permissions."
    }
  }
  & icacls $authKeys /inheritance:r /grant:r "${principal}:F" "*S-1-5-18:F" | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "Failed to restrict the authorized-key file permissions."
  }
}

function Set-SshdOption {
  param(
    [string]$ConfigPath,
    [string]$Name,
    [string]$Value
  )

  $content = [Collections.Generic.List[string]]::new()
  $content.AddRange([string[]]@(Get-Content -LiteralPath $ConfigPath))
  $pattern = "^\s*#?\s*$([regex]::Escape($Name))\s+.*$"
  $replacement = "$Name $Value"
  $globalEnd = $content.Count
  for ($index = 0; $index -lt $content.Count; $index++) {
    if ($content[$index] -match '^\s*Match\s+') {
      $globalEnd = $index
      break
    }
    if ($content[$index] -match $pattern) {
      if ($content[$index] -ceq $replacement) {
        return $false
      }
      $content[$index] = $replacement
      Set-Content -LiteralPath $ConfigPath -Value $content -Encoding ascii
      return $true
    }
  }
  $content.Insert($globalEnd, $replacement)
  Set-Content -LiteralPath $ConfigPath -Value $content -Encoding ascii
  return $true
}

function Start-ProvisionedSshService {
  param([string]$ConfigPath, [bool]$ConfigurationChanged = $false)

  # The Windows service normally generates these on its first start. Validation
  # runs before service mutation, so create missing keys explicitly; -A keeps
  # existing host identities intact on later boots and repair runs.
  $keygen = Get-Command ssh-keygen.exe -ErrorAction Stop
  & $keygen.Source -A
  if ($LASTEXITCODE -ne 0) {
    throw "OpenSSH host-key generation failed; the service was not restarted."
  }
  foreach ($name in @('ssh_host_rsa_key', 'ssh_host_ecdsa_key', 'ssh_host_ed25519_key')) {
    $hostKey = Join-Path (Join-Path $env:ProgramData 'ssh') $name
    if (Test-Path -LiteralPath $hostKey) {
      # Keys created by an elevated interactive user can retain that user's
      # writable ACE. The SYSTEM daemon must trust only SYSTEM/Administrators.
      Protect-GuestProvisioningFile -Path $hostKey
    }
  }
  $sshd = Get-Command sshd.exe -ErrorAction Stop
  & $sshd.Source -t -f $ConfigPath
  if ($LASTEXITCODE -ne 0) {
    throw "OpenSSH configuration validation failed; the service was not restarted."
  }
  Set-Service -Name sshd -StartupType Automatic
  if ((Get-Service -Name sshd).Status -eq 'Running') {
    if ($ConfigurationChanged) {
      Restart-Service -Name sshd -ErrorAction Stop
    }
  } else {
    Start-Service -Name sshd -ErrorAction Stop
  }
}

function Ensure-SshFirewallRule {
  $rule = Get-NetFirewallRule -Name "OpenSSH-Server-In-TCP" -ErrorAction SilentlyContinue
  if ($null -eq $rule) {
    Write-Step "Creating inbound firewall rule for TCP 22"
    New-NetFirewallRule -Name "OpenSSH-Server-In-TCP" -DisplayName "OpenSSH Server (TCP 22)" -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22 | Out-Null
  } else {
    Write-Step "Repairing OpenSSH firewall rule"
    Set-NetFirewallRule -Name "OpenSSH-Server-In-TCP" -Enabled True -Direction Inbound -Action Allow | Out-Null
    Get-NetFirewallRule -Name "OpenSSH-Server-In-TCP" | Get-NetFirewallPortFilter |
      Set-NetFirewallPortFilter -Protocol TCP -LocalPort 22 | Out-Null
  }
}

function Resolve-DockPipeAgentSourcePath {
  $searchRoots = @()
  if ($PSCommandPath) {
    $searchRoots += (Split-Path -Parent $PSCommandPath)
  }
  $searchRoots += (Get-Location).Path
  $searchRoots = $searchRoots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique

  foreach ($root in $searchRoots) {
    $candidate = Join-Path $root "dockpipe-guest-agent.exe"
    if (Test-Path -LiteralPath $candidate) {
      return (Resolve-Path -LiteralPath $candidate).Path
    }
    $candidate = Join-Path $root "dockpipe-guest-agent.ps1"
    if (Test-Path -LiteralPath $candidate) {
      return (Resolve-Path -LiteralPath $candidate).Path
    }
  }

  throw "dockpipe-guest-agent.exe or dockpipe-guest-agent.ps1 was not found beside the provisioning script or in the current directory."
}

function Get-GuestProvisioningSecurityFingerprint {
  param([string]$Path, [bool]$Directory)

  if (-not (Test-Path -LiteralPath $Path)) {
    return $null
  }
  Assert-GuestToolsStatusPath -Path $Path -Directory $Directory
  $sections = [Security.AccessControl.AccessControlSections]'Owner,Group,Access'
  return (Get-Acl -LiteralPath $Path).GetSecurityDescriptorSddlForm($sections)
}

function Test-DockPipeAgentConfiguration {
  param([string]$Path, [int]$Port, [string]$StatePath)

  if (-not (Test-Path -LiteralPath $Path)) {
    return $false
  }
  Assert-GuestToolsStatusPath -Path $Path -Directory $false
  if ((Get-Item -LiteralPath $Path).Length -gt 32768) {
    throw 'Guest agent configuration exceeds its bounded contract.'
  }
  try {
    $configuration = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
  } catch [ArgumentException] {
    return $false
  }
  $fields = @('port', 'bind_address', 'state_path', 'service_account')
  if (@($configuration.PSObject.Properties).Count -ne $fields.Count) {
    return $false
  }
  foreach ($name in $fields) {
    if ($configuration.PSObject.Properties.Name -cnotcontains $name) {
      return $false
    }
  }
  return ($configuration.port -is [int] -or $configuration.port -is [long]) -and
    $configuration.port -eq $Port -and $configuration.bind_address -ceq '127.0.0.1' -and
    $configuration.state_path -ceq $StatePath -and $configuration.service_account -ceq 'LocalSystem'
}

function Test-GuestAgentUnlimitedExecution {
  param($Value)

  if ($Value -is [TimeSpan]) {
    return $Value -eq [TimeSpan]::Zero
  }
  if ($Value -isnot [string] -or [string]::IsNullOrWhiteSpace($Value) -or $Value.Length -gt 64) {
    return $false
  }
  try {
    return [Xml.XmlConvert]::ToTimeSpan($Value) -eq [TimeSpan]::Zero
  } catch [FormatException] {
    return $false
  }
}

function Test-DockPipeAgentTask {
  param($Task, [string]$Executable, [string]$Arguments)

  if ($null -eq $Task -or @($Task.Actions).Count -ne 1 -or @($Task.Triggers).Count -ne 1) {
    return $false
  }
  $action = $Task.Actions[0]
  $principal = $Task.Principal
  $settings = $Task.Settings
  $trigger = $Task.Triggers[0]
  return $Task.TaskPath -ceq '\' -and $action.Execute -ieq $Executable -and $action.Arguments -ceq $Arguments -and
    [string]::IsNullOrEmpty($action.WorkingDirectory) -and
    $principal.UserId -in @('SYSTEM', 'S-1-5-18') -and
    [string]$principal.LogonType -eq 'ServiceAccount' -and [string]$principal.RunLevel -eq 'Highest' -and
    $trigger.CimClass.CimClassName -eq 'MSFT_TaskBootTrigger' -and $trigger.Enabled -and
    -not $settings.DisallowStartIfOnBatteries -and -not $settings.StopIfGoingOnBatteries -and
    [string]$settings.MultipleInstances -in @('IgnoreNew', '2') -and $settings.StartWhenAvailable -and
    $settings.Enabled -and (Test-GuestAgentUnlimitedExecution -Value $settings.ExecutionTimeLimit)
}

function Get-DockPipeAgentProcesses {
  param([string]$Executable, [string]$Arguments, [string]$NativePath, [string]$ScriptPath)

  $scriptExecutable = Join-Path $env:WINDIR 'System32\WindowsPowerShell\v1.0\powershell.exe'
  $scriptArguments = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" -Service -ConfigPath "{1}"' -f $ScriptPath, (Join-Path (Split-Path -Parent $ScriptPath) 'config.json')
  $candidates = @(Get-CimInstance Win32_Process -Filter "Name = 'dockpipe-guest-agent.exe' OR Name = 'powershell.exe'")
  if ($candidates.Count -gt 4096) {
    throw 'Guest agent process discovery exceeds its bound.'
  }
  $selected = @()
  foreach ($candidate in $candidates) {
    $matchesNative = $candidate.ExecutablePath -ieq $NativePath
    $matchesScript = $candidate.ExecutablePath -ieq $scriptExecutable -and
      $candidate.CommandLine -cin @((('"{0}" {1}' -f $scriptExecutable, $scriptArguments)), ($scriptExecutable + ' ' + $scriptArguments))
    if (-not $matchesNative -and -not $matchesScript) {
      continue
    }
    if ($selected.Count -ge 16) {
      throw 'Guest agent process selection exceeds its bound.'
    }
    $owner = Invoke-CimMethod -InputObject $candidate -MethodName GetOwnerSid -ErrorAction Stop
    if ($owner.ReturnValue -ne 0 -or $owner.Sid -ne 'S-1-5-18') {
      throw 'A selected guest agent process is not owned by SYSTEM.'
    }
    $process = Get-Process -Id $candidate.ProcessId -ErrorAction Stop
    try {
      if ($process.Path -ine $candidate.ExecutablePath) {
        throw 'Guest agent image changed during observation.'
      }
      $selected += [pscustomobject]@{
        Id = $process.Id
        Image = $process.Path
        StartFileTime = $process.StartTime.ToUniversalTime().ToFileTimeUtc()
        DesiredInvocation = $candidate.ExecutablePath -ieq $Executable -and
          $candidate.CommandLine -cin @((('"{0}" {1}' -f $Executable, $Arguments)), ($Executable + ' ' + $Arguments))
      }
    } finally {
      $process.Dispose()
    }
  }
  return $selected
}

function Stop-SelectedDockPipeAgent {
  param($Identity)

  $process = Get-Process -Id $Identity.Id -ErrorAction SilentlyContinue
  if ($null -eq $process) {
    return
  }
  try {
    $handle = $process.Handle
    if ($process.Path -ine $Identity.Image -or $process.StartTime.ToUniversalTime().ToFileTimeUtc() -ne $Identity.StartFileTime) {
      throw 'Changed guest agent identity refused cleanup.'
    }
    $process.Kill()
    if (-not $process.WaitForExit(5000)) {
      throw 'Selected guest agent did not exit within its cleanup bound.'
    }
  } finally {
    $process.Dispose()
  }
}

function Ensure-DockPipeAgent {
  param([int]$Port)

  $agentRoot = Join-Path $env:ProgramData 'Dockpipe\GuestAgent'
  $agentExe = Join-Path $agentRoot 'dockpipe-guest-agent.exe'
  $agentScript = Join-Path $agentRoot 'dockpipe-guest-agent.ps1'
  $configPath = Join-Path $agentRoot 'config.json'
  $statePath = Join-Path $agentRoot 'state.json'
  $sourceAgent = Resolve-DockPipeAgentSourcePath
  Assert-GuestToolsStatusPath -Path $sourceAgent -Directory $false
  $sourceHash = (Get-FileHash -LiteralPath $sourceAgent -Algorithm SHA256).Hash
  $native = [IO.Path]::GetExtension($sourceAgent) -ieq '.exe'
  $target = if ($native) { $agentExe } else { $agentScript }
  $obsolete = if ($native) { $agentScript } else { $agentExe }
  $executable = if ($native) { $agentExe } else { Join-Path $env:WINDIR 'System32\WindowsPowerShell\v1.0\powershell.exe' }
  $arguments = if ($native) {
    '-service -ConfigPath "{0}"' -f $configPath
  } else {
    '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" -Service -ConfigPath "{1}"' -f $agentScript, $configPath
  }

  $securityBefore = Get-GuestProvisioningSecurityFingerprint -Path $agentRoot -Directory $true
  Protect-GuestProvisioningDirectory -Path $agentRoot
  $securityChanged = $securityBefore -cne (Get-GuestProvisioningSecurityFingerprint -Path $agentRoot -Directory $true)
  foreach ($path in @($target, $obsolete, $configPath)) {
    if (Test-Path -LiteralPath $path) {
      $securityBefore = Get-GuestProvisioningSecurityFingerprint -Path $path -Directory $false
      Protect-GuestProvisioningFile -Path $path
      $securityChanged = $securityChanged -or $securityBefore -cne (Get-GuestProvisioningSecurityFingerprint -Path $path -Directory $false)
    }
  }
  $binaryChanged = -not (Test-Path -LiteralPath $target)
  if (-not $binaryChanged) {
    $binaryChanged = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -cne $sourceHash
  }
  $contentChanged = $binaryChanged
  $obsoletePresent = Test-Path -LiteralPath $obsolete
  $binaryChanged = $contentChanged -or $obsoletePresent
  $configurationChanged = -not (Test-DockPipeAgentConfiguration -Path $configPath -Port $Port -StatePath $statePath)
  $taskName = 'DockPipeGuestAgent'
  $task = Get-ScheduledTask -TaskPath '\' -TaskName $taskName -ErrorAction SilentlyContinue
  $taskChanged = -not (Test-DockPipeAgentTask -Task $task -Executable $executable -Arguments $arguments)
  $processes = @(Get-DockPipeAgentProcesses -Executable $executable -Arguments $arguments -NativePath $agentExe -ScriptPath $agentScript)
  $running = $null -ne $task -and $task.State -eq 'Running' -and $processes.Count -eq 1 -and $processes[0].Image -ieq $executable -and $processes[0].DesiredInvocation
  if (-not $binaryChanged -and -not $configurationChanged -and -not $taskChanged -and -not $securityChanged -and $running) {
    Write-Step 'The validated guest agent is already running; preserving its process identity.'
    return
  }

  # No live process is interrupted until desired bytes, security and task policy have been checked.
  if ($null -ne $task) {
    # A modified task may point at an unrelated executable. Never stop its action.
    # Unregister only the owned task; terminate only the selected agent identities below.
    Unregister-ScheduledTask -TaskPath '\' -TaskName $taskName -Confirm:$false -ErrorAction Stop
  }
  foreach ($identity in $processes) {
    Stop-SelectedDockPipeAgent -Identity $identity
  }
  if ($contentChanged) {
    Copy-Item -LiteralPath $sourceAgent -Destination $target -Force -ErrorAction Stop
    Protect-GuestProvisioningFile -Path $target
    if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -cne $sourceHash) {
      throw 'Copied guest agent differs from the admitted source bytes.'
    }
  }
  if ($obsoletePresent) {
    Remove-Item -LiteralPath $obsolete -Force -ErrorAction Stop
  }
  if ($configurationChanged) {
    [ordered]@{
      port = $Port
      bind_address = '127.0.0.1'
      state_path = $statePath
      service_account = 'LocalSystem'
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $configPath -ErrorAction Stop
    Protect-GuestProvisioningFile -Path $configPath
  }
  $action = New-ScheduledTaskAction -Execute $executable -Argument $arguments
  $trigger = New-ScheduledTaskTrigger -AtStartup
  $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
  $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -MultipleInstances IgnoreNew -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero)
  # Set the schema duration explicitly as well, rather than accepting a module default.
  $settings.ExecutionTimeLimit = 'PT0S'
  Register-ScheduledTask -TaskPath '\' -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -ErrorAction Stop | Out-Null
  $registered = Get-ScheduledTask -TaskPath '\' -TaskName $taskName -ErrorAction Stop
  if (-not (Test-DockPipeAgentTask -Task $registered -Executable $executable -Arguments $arguments)) {
    throw 'Registered guest agent task did not retain the admitted durable policy.'
  }
  Start-ScheduledTask -TaskPath '\' -TaskName $taskName -ErrorAction Stop
}

try {
  Assert-Administrator
  Initialize-GuestToolsStatusStorage
  if ($StartupRepair -and $RegisterOnly) {
    throw 'StartupRepair and RegisterOnly cannot be combined.'
  }
  Write-Step -Phase preparing -Message 'Preparing Windows guest services'
  # Attached bootstrap media carries only the public half of the host identity.
  $bootstrapKeyPath = Join-Path $PSScriptRoot "dockpipe-guest-ssh.pub"
  if (-not $PSBoundParameters.ContainsKey('AuthorizedKey') -and (Test-Path -LiteralPath $bootstrapKeyPath)) {
    if ((Get-Item -LiteralPath $bootstrapKeyPath).Length -gt 16384) {
      throw "The bootstrap SSH public key exceeds 16384 bytes."
    }
    $AuthorizedKey = (Get-Content -Raw -LiteralPath $bootstrapKeyPath).Trim()
    if (-not $PSBoundParameters.ContainsKey('UserName')) {
      $UserName = $env:USERNAME
    }
    if (-not $PSBoundParameters.ContainsKey('CreateUser')) {
      $CreateUser = $false
    }
  }

  if ($StartupRepair) {
    $configuration = Read-StartupRepairConfiguration -Path (Join-Path $PSScriptRoot 'configuration.json')
    $UserName = $configuration.UserName
    $AuthorizedKey = $configuration.AuthorizedKey
    $DefaultShell = $configuration.DefaultShell
    $EnablePasswordAuth = $configuration.EnablePasswordAuth
    $EnablePublicKeyAuth = $configuration.EnablePublicKeyAuth
    $InstallDockPipeAgent = $configuration.InstallDockPipeAgent
    $AgentPort = $configuration.AgentPort
    $CreateUser = $false
    $PasswordPlain = ''
    $GrantAdministrators = $false
  } else {
    Write-Step -Phase registering -Message 'Registering durable guest provisioning'
    $initialConfiguration = @{
      SchemaVersion = 1
      UserName = $UserName
      AuthorizedKey = $AuthorizedKey
      DefaultShell = $DefaultShell
      EnablePasswordAuth = $EnablePasswordAuth
      EnablePublicKeyAuth = $EnablePublicKeyAuth
      InstallDockPipeAgent = $InstallDockPipeAgent
      AgentPort = $AgentPort
    }
    $existingConfigurationPath = Join-Path (Join-Path $env:ProgramData 'Dockpipe\GuestProvisioning') 'configuration.json'
    if ($RegisterOnly -and (Test-Path -LiteralPath $existingConfigurationPath)) {
      $existingConfiguration = Read-StartupRepairConfiguration -Path $existingConfigurationPath
      foreach ($name in @($initialConfiguration.Keys)) {
        $initialConfiguration[$name] = $existingConfiguration.$name
      }
    }
    Install-GuestProvisioningStartupTask -Configuration $initialConfiguration
    # Installation deliberately restricts the product root before persisting any
    # private data; restore only the non-inheriting public directory read ACE.
    Initialize-GuestToolsStatusStorage
    $script:GuestToolsStartupRegistered = $null -ne (
      Get-ScheduledTask -TaskName 'DockPipeGuestProvisioning' -ErrorAction SilentlyContinue)
    if (-not $script:GuestToolsStartupRegistered) {
      throw 'Automatic guest provisioning task registration could not be verified.'
    }
  }
  if ($RegisterOnly) {
    Write-Step -Phase queued -Message 'Starting detached automatic guest provisioning'
    Start-ScheduledTask -TaskName 'DockPipeGuestProvisioning' -ErrorAction Stop
    return
  }
  Write-Step -Phase capability -Message 'Checking Windows OpenSSH Server'
  Ensure-OpenSSHServer

  if ($CreateUser) {
    Write-Step -Phase account -Message 'Checking the requested local account'
    Ensure-LocalUser -Name $UserName -PlainPassword $PasswordPlain -AddToAdministrators $GrantAdministrators
  }

  Write-Step -Phase ssh_configuration -Message 'Configuring Windows OpenSSH'
  $sshConfigExisted = Test-Path -LiteralPath (Join-Path (Join-Path $env:ProgramData 'ssh') 'sshd_config')
  $sshdConfig = Ensure-SshConfigFile
  $passwordConfigurationChanged = Set-SshdOption -ConfigPath $sshdConfig -Name "PasswordAuthentication" -Value ($(if ($EnablePasswordAuth) { "yes" } else { "no" }))
  $publicKeyConfigurationChanged = Set-SshdOption -ConfigPath $sshdConfig -Name "PubkeyAuthentication" -Value ($(if ($EnablePublicKeyAuth) { "yes" } else { "no" }))
  $sshdConfigurationChanged = -not $sshConfigExisted -or $passwordConfigurationChanged -or $publicKeyConfigurationChanged

  if (-not [string]::IsNullOrWhiteSpace($DefaultShell)) {
    $existingShell = Get-ItemProperty -LiteralPath "HKLM:\SOFTWARE\OpenSSH" -Name DefaultShell -ErrorAction SilentlyContinue
    $existingShellValue = $null
    if ($null -ne $existingShell -and $null -ne $existingShell.PSObject.Properties['DefaultShell']) {
      $existingShellValue = $existingShell.DefaultShell
    }
    if ($existingShellValue -cne $DefaultShell) {
      Write-Step "Setting DefaultShell to $DefaultShell"
      New-Item -Path "HKLM:\SOFTWARE\OpenSSH" -Force | Out-Null
      New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" -Name "DefaultShell" -Value $DefaultShell -PropertyType String -Force | Out-Null
      $sshdConfigurationChanged = $true
    }
  }

  Write-Step -Phase firewall -Message 'Repairing SSH firewall access'
  Ensure-SshFirewallRule

  if ($EnablePublicKeyAuth -and -not [string]::IsNullOrWhiteSpace($AuthorizedKey)) {
    Write-Step -Phase authorized_key -Message 'Configuring SSH public-key access'
    Ensure-AuthorizedKey -Name $UserName -Key $AuthorizedKey
  }

  if ($InstallDockPipeAgent) {
    Write-Step -Phase agent -Message 'Configuring the Dockpipe guest agent'
    Ensure-DockPipeAgent -Port $AgentPort
  }

  Write-Step -Phase ssh_service -Message 'Configuring sshd service startup'
  Start-ProvisionedSshService -ConfigPath $sshdConfig -ConfigurationChanged $sshdConfigurationChanged

  Write-Step "Configuring ssh-agent service startup"
  Set-Service -Name ssh-agent -StartupType Manual

  $status = Get-Service -Name sshd
  Write-Step "sshd status: $($status.Status)"

  Write-Step -Phase health -Message 'Checking Windows guest services'
  Wait-GuestToolsHealth -AgentRequested $InstallDockPipeAgent -Port $AgentPort
  Write-Step "Provisioning complete. Test with: ssh $UserName@localhost"
  Write-GuestToolsStatus -State ready -Phase ready
} catch {
  if ($script:GuestToolsStatusInitialized) {
    try {
      Write-GuestToolsStatus -State failed -Phase $script:GuestToolsPhase
    } catch {
      Write-Warning 'Guest tools progress could not be recorded securely.'
    }
  }
  throw
}

# Exercise the release workflow's actual PowerShell calls against the MSI scripts'
# parameter contracts, without downloading WiX or installing an MSI.
$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../../..")).Path
$workflowLines = Get-Content -LiteralPath (Join-Path $repoRoot ".github/workflows/release.yml")

function Get-WorkflowScript {
    param([string]$StepName)

    $inStep = $false
    $hasReleaseVersion = $false
    foreach ($line in $workflowLines) {
        if ($line -eq "      - name: $StepName") {
            $inStep = $true
            continue
        }
        if (-not $inStep) { continue }
        if ($line -ceq '          DOCKPIPE_ADAPTER_0: ${{ needs.meta.outputs.version }}') {
            $hasReleaseVersion = $true
        }
        if ($line -match '^        run: python workflows/shared/launch.py (workflows/[^ ]+\.yml)$') {
            if (-not $hasReleaseVersion) {
                throw "Expected $StepName to receive the release version"
            }
            $workflow = Join-Path $repoRoot $Matches[1]
            foreach ($command in Get-Content -LiteralPath $workflow) {
                if ($command -match '^    cmd: pwsh -NoProfile -File (workflows/[^ ]+\.ps1)$') {
                    return Join-Path $repoRoot $Matches[1]
                }
            }
            throw "Expected an owning PowerShell workflow for $StepName"
        }
        if ($line.StartsWith('      - ')) { break }
    }
    throw "Workflow step not found: $StepName"
}

function New-ParameterRecorder {
    param([string]$Source, [string]$Destination)

    $tokens = $null
    $parseErrors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($Source, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count -ne 0 -or -not $ast.ParamBlock) {
        throw "Cannot read MSI parameter contract: $Source"
    }
    $body = '$PSBoundParameters | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath ($PSCommandPath + ".json")'
    Set-Content -LiteralPath $Destination -Value ($ast.ParamBlock.Extent.Text + "`n" + $body)
}

function Assert-Equal {
    param($Actual, $Expected, [string]$Label)

    if ($Actual -cne $Expected) { throw "${Label}: expected '$Expected', got '$Actual'" }
}

function Test-MsiBinaryArchitecture {
    param([string]$Fixture)

    $source = Join-Path $repoRoot "workflows/package/windows-msi/assets/scripts/build.ps1"
    $tokens = $null
    $parseErrors = $null
    $build = [System.Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count -ne 0) { throw "Cannot parse MSI packager" }

    $check = $build.Find({
        param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
            $node.Name -eq "Assert-MsiBinaryArchitecture"
    }, $true)
    if (-not $check) { throw "MSI PE architecture check is missing" }
    $assertArchitecture = [scriptblock]::Create($check.Body.Extent.Text.TrimStart('{').TrimEnd('}'))
    $binary = Join-Path $Fixture "architecture.exe"
    $bytes = [byte[]]::new(70)
    $bytes[0] = 0x4D
    $bytes[1] = 0x5A
    $bytes[0x3C] = 64
    $bytes[64] = 0x50
    $bytes[65] = 0x45

    foreach ($target in @(
        @{ Name = "amd64"; Machine = [uint16]0x8664; Other = "arm64" },
        @{ Name = "arm64"; Machine = [uint16]0xAA64; Other = "amd64" }
    )) {
        [BitConverter]::GetBytes($target.Machine).CopyTo($bytes, 68)
        [IO.File]::WriteAllBytes($binary, $bytes)
        & $assertArchitecture -Path $binary -Architecture $target.Name

        $rejected = $false
        try { & $assertArchitecture -Path $binary -Architecture $target.Other } catch {
            if (-not $_.Exception.Message.StartsWith("Binary does not match")) { throw }
            $rejected = $true
        }
        Assert-Equal $rejected $true "Reject a mixed-architecture payload"
    }

    $bytes[0x3C] = 255
    [IO.File]::WriteAllBytes($binary, $bytes)
    $rejected = $false
    try { & $assertArchitecture -Path $binary -Architecture "arm64" } catch {
        if (-not $_.Exception.Message.StartsWith("Invalid PE header offset")) { throw }
        $rejected = $true
    }
    Assert-Equal $rejected $true "Reject an out-of-range PE header"
}

function Test-MsiLicenseRendering {
    param([string]$Fixture)

    $source = Join-Path $repoRoot "workflows/package/windows-msi/assets/scripts/build.ps1"
    $tokens = $null
    $parseErrors = $null
    $build = [System.Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count -ne 0) { throw "Cannot parse MSI packager" }

    $renderer = $build.Find({
        param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
            $node.Name -eq "Write-MsiLicenseRtf"
    }, $true)
    if (-not $renderer) { throw "MSI license renderer is missing" }

    $rtfPath = Join-Path $Fixture "test license.rtf"
    $licensePath = Join-Path $repoRoot "LICENSE"
    $header = '{\rtf1\ansi\deff0{\fonttbl{\f0 Segoe UI;}}\f0\fs20 '
    $render = [scriptblock]::Create($renderer.Body.Extent.Text.TrimStart('{').TrimEnd('}'))
    & $render -Source $licensePath -Destination $rtfPath
    $rtf = [IO.File]::ReadAllText($rtfPath)
    Assert-Equal ($rtf.StartsWith($header) -and $rtf.EndsWith('}')) $true "License RTF envelope"
    $body = $rtf.Substring($header.Length, $rtf.Length - $header.Length - 1)
    $plain = $body.Replace("\par`r`n", "`n").Replace('\{', '{').Replace('\}', '}').Replace('\\', '\')
    $license = [IO.File]::ReadAllText($licensePath) -replace '\r\n|\r', "`n"
    Assert-Equal $plain $license "MSI license preserves the repository license text"

    $fixtureLicense = Join-Path $Fixture "raw license.txt"
    [IO.File]::WriteAllText($fixtureLicense, '{\}' + [char]0x00E9)
    & $render -Source $fixtureLicense -Destination $rtfPath
    Assert-Equal ([IO.File]::ReadAllText($rtfPath)) ($header + '\{\\\}\u233?}') "RTF special characters"
}

function Test-PackageSnapshotComparison {
    $tokens = $null
    $parseErrors = $null
    $source = Join-Path $repoRoot "workflows/package/windows-msi/assets/scripts/smoke-test.ps1"
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count -ne 0) { throw "Cannot parse MSI smoke test" }
    $comparisons = @($ast.FindAll({
        param($node)
        if ($node -isnot [System.Management.Automation.Language.IfStatementAst]) { return $false }
        $condition = $node.Clauses[0].Item1.Extent.Text
        return $condition.Contains('$currentPackageSnapshot') -and $condition.Contains('$baselinePackageSnapshot')
    }, $true))
    if ($comparisons.Count -ne 1) { throw "Expected one package snapshot comparison" }
    $comparison = [scriptblock]::Create($comparisons[0].Extent.Text)
    $cases = @(
        @{
            Name = "empty"
            Baseline = @()
            Current = @()
            Reject = $false
        },
        @{
            Name = "unchanged"
            Baseline = @("existing")
            Current = @("existing")
            Reject = $false
        },
        @{
            Name = "unchanged tree"
            Baseline = @("dir", "dir/file")
            Current = @("dir", "dir/file")
            Reject = $false
        },
        @{
            Name = "added"
            Baseline = @()
            Current = @("leftover")
            Reject = $true
        },
        @{
            Name = "removed"
            Baseline = @("existing")
            Current = @()
            Reject = $true
        },
        @{
            Name = "replaced"
            Baseline = @("existing")
            Current = @("different")
            Reject = $true
        }
    )

    foreach ($case in $cases) {
        $baselinePackageSnapshot = $case.Baseline
        $currentPackageSnapshot = $case.Current
        $packageDir = "fixture core directory"
        $rejected = $false
        try {
            & $comparison
        } catch {
            if (-not $_.Exception.Message.StartsWith("Installed dockpipe package contents did not return to baseline")) {
                throw
            }
            $rejected = $true
        }
        Assert-Equal $rejected $case.Reject ("Core snapshot " + $case.Name)
    }
}

function New-FixtureFile {
    param([string]$Path)

    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    Set-Content -LiteralPath $Path -Value "fixture"
}

function Assert-FailsWith {
    param([scriptblock]$Action, [string]$Message)

    try {
        & $Action
    } catch {
        if (-not $_.Exception.Message.Contains($Message)) {
            throw
        }
        return
    }
    throw "Expected an error containing '$Message'"
}

function Test-ReleaseEntrypoints {
    param([string]$Fixture)

    function Invoke-WebRequest {
        param([string]$Uri, [string]$OutFile)

        New-FixtureFile -Path $OutFile
    }

    function Expand-Archive {
        param([string]$Path, [string]$DestinationPath, [switch]$Force)

        New-FixtureFile -Path (Join-Path $DestinationPath "candle.exe")
    }

    $scripts = Join-Path $Fixture "scripts"
    New-Item -ItemType Directory -Force -Path $scripts | Out-Null
    Copy-Item -LiteralPath (Get-WorkflowScript "Install WiX and build MSI") -Destination $scripts
    Copy-Item -LiteralPath (Get-WorkflowScript "Smoke test MSI install/uninstall") -Destination $scripts
    $release = Join-Path $scripts "release.ps1"
    $smokeRelease = Join-Path $scripts "smoke-release.ps1"
    $buildRecorder = Join-Path $scripts "build.ps1"
    $smokeRecorder = Join-Path $scripts "smoke-test.ps1"
    $source = Join-Path $repoRoot "workflows/package/windows-msi/assets/scripts"
    New-ParameterRecorder -Source (Join-Path $source "build.ps1") -Destination $buildRecorder
    New-ParameterRecorder -Source (Join-Path $source "smoke-test.ps1") -Destination $smokeRecorder

    $unsignedCli = Join-Path $Fixture "src/bin/dockpipe.exe"
    $signedStage = Join-Path $Fixture "signed input stage"
    $signedCli = Join-Path $signedStage "dockpipe.exe"
    $launcherStage = Join-Path $Fixture "bin/desktop-stage"
    $outputDir = Join-Path $Fixture "bin/msi-dist"
    $env:DOCKPIPE_ADAPTER_0 = "0.6.0"
    $env:DOCKPIPE_WINDOWS_DESKTOP_INPUT_STAGE = $null
    $env:RUNNER_TEMP = Join-Path $Fixture "runner temp"
    $env:TEMP = $env:RUNNER_TEMP

    Push-Location $Fixture
    try {
        New-FixtureFile -Path $unsignedCli
        Assert-FailsWith -Action { & $release } -Message "Desktop MSI requires the deployed launcher runtime"
        foreach ($required in @("dockpipe-launcher.exe", "Qt6Core.dll", "platforms/qwindows.dll")) {
            New-FixtureFile -Path (Join-Path $launcherStage $required)
        }

        $signedPayload = @{
            WorkerHost = "dockpipe-worker.exe"
            DesktopInputService = "dockpipe-desktop-input-service.exe"
            DesktopInputHelper = "dockpipe-desktop-input-helper.exe"
            DesktopInputNotices = "THIRD-PARTY-NOTICES.txt"
        }
        foreach ($name in $signedPayload.Values) {
            New-FixtureFile -Path (Join-Path $signedStage $name)
        }

        $msiPath = Join-Path $outputDir "dockpipe_0.6.0_windows_amd64.msi"
        New-FixtureFile -Path $msiPath
        New-FixtureFile -Path (Join-Path $outputDir "dockpipe_0.5.0_windows_amd64.msi")
        foreach ($machine in @($false, $true)) {
            if ($machine) {
                $env:DOCKPIPE_WINDOWS_DESKTOP_INPUT_STAGE = $signedStage
                Assert-FailsWith -Action { & $release } -Message "Desktop input MSI requires the staged file: $signedCli"
                New-FixtureFile -Path $signedCli
            }

            & $release
            $build = Get-Content -Raw -LiteralPath ($buildRecorder + ".json") | ConvertFrom-Json
            Assert-Equal $build.Version "0.6.0" "Version"
            Assert-Equal $build.OutDir $outputDir "OutDir"
            Assert-Equal $build.WixRoot (Join-Path $env:RUNNER_TEMP "wix314-expand") "WixRoot"
            Assert-Equal $build.LauncherStageDir $launcherStage "LauncherStageDir"
            Assert-Equal ($build.PSObject.Properties.Name -contains "PackageStoreDir") $false "Standalone MSI has no package store input"
            if ($machine) {
                Assert-Equal $build.SourceExe $signedCli "Machine-wide SourceExe"
                foreach ($parameter in $signedPayload.Keys) {
                    Assert-Equal $build.$parameter (Join-Path $signedStage $signedPayload[$parameter]) $parameter
                }
            } else {
                Assert-Equal $build.SourceExe $unsignedCli "Per-user SourceExe"
                Assert-Equal ($build.PSObject.Properties.Name -contains "DesktopInputService") $false "Per-user MSI omits the service"
            }

            & $smokeRelease
            $smoke = Get-Content -Raw -LiteralPath ($smokeRecorder + ".json") | ConvertFrom-Json
            Assert-Equal $smoke.MsiPath $msiPath "Select the version being released"
            Assert-Equal $smoke.ExpectLauncher.IsPresent $true "ExpectLauncher"
            Assert-Equal $smoke.ExpectDesktopInput.IsPresent $machine "ExpectDesktopInput"
        }
    } finally {
        Pop-Location
    }
}

function Test-SmokeCleanupAndScope {
    param([string]$Fixture, [bool]$Machine, [switch]$InstallFails)

    $env:LOCALAPPDATA = Join-Path $Fixture "Local AppData"
    $env:APPDATA = Join-Path $Fixture "Roaming AppData"
    $env:ProgramW6432 = Join-Path $Fixture "Program Files"
    $env:ProgramData = Join-Path $Fixture "ProgramData"
    $env:TEMP = $Fixture

    $fixtureInstallRoot = $env:LOCALAPPDATA
    $fixtureStartMenuRoot = $env:APPDATA
    $fixtureScope = "User"
    if ($Machine) {
        $fixtureInstallRoot = $env:ProgramW6432
        $fixtureStartMenuRoot = $env:ProgramData
        $fixtureScope = "Machine"
    }

    $fixtureInstall = Join-Path $fixtureInstallRoot "dockpipe"
    $fixtureWorker = Join-Path $fixtureInstall "dockpipe-worker.exe"
    $fixturePayload = @(
        Join-Path $fixtureInstall "dockpipe.exe"
        Join-Path $fixtureInstall "dockpipe-launcher.exe"
        Join-Path $fixtureInstall "dockpipe-terminal-host.exe"
        Join-Path $fixtureInstall "Qt6Core.dll"
        Join-Path $fixtureInstall "platforms/qwindows.dll"
        Join-Path $fixtureStartMenuRoot "Microsoft/Windows/Start Menu/Programs/Dockpipe/Dockpipe.lnk"
        $fixtureWorker
    )
    if ($Machine) {
        $fixtureInput = Join-Path $fixtureInstallRoot "Dockpipe Desktop Input"
        $fixturePayload += @(
            Join-Path $fixtureInput "dockpipe.exe"
            Join-Path $fixtureInput "dockpipe-desktop-input-service.exe"
            Join-Path $fixtureInput "dockpipe-desktop-input-helper.exe"
            Join-Path $fixtureInput "THIRD-PARTY-NOTICES.txt"
        )
    }

    $fixtureState = @{ Actions = [Collections.Generic.List[string]]::new(); ServiceInstalled = $false; ServiceChecks = 0 }
    function Get-CimInstance {
        param([string]$ClassName, [string]$Filter)

        Assert-Equal $ClassName "Win32_Service" "Query service registration"
        Assert-Equal $Filter "Name = 'DockpipeDesktopInput'" "Query only the Dockpipe service"
        $fixtureState.ServiceChecks++
        if ($fixtureState.ServiceInstalled) {
            return @{ StartMode = "Auto"; StartName = "LocalSystem" }
        }
    }

    function Start-Process {
        param([string]$FilePath, [string[]]$ArgumentList, [switch]$Wait, [switch]$PassThru)

        Assert-Equal $FilePath "msiexec.exe" "Run Windows Installer"
        Assert-Equal $pathScope $fixtureScope "Check the expected scoped PATH"
        $action = $ArgumentList[0]
        $fixtureState.Actions.Add($action)

        if ($action -eq "/i") {
            if ($InstallFails) {
                return @{ ExitCode = 1603 }
            }

            foreach ($path in $fixturePayload) {
                New-FixtureFile -Path $path
            }
            $workerBytes = [byte[]]::new(158)
            $workerBytes[0x3c] = 64
            $workerBytes[64] = 0x50
            $workerBytes[65] = 0x45
            $workerBytes[156] = 2
            [IO.File]::WriteAllBytes($fixtureWorker, $workerBytes)
            $fixtureState.ServiceInstalled = $Machine
        } elseif ($action -eq "/x") {
            if ($InstallFails) {
                return @{ ExitCode = 1605 }
            }

            foreach ($path in $fixturePayload) {
                Remove-Item -LiteralPath $path
            }
            $fixtureState.ServiceInstalled = $false
        }
        return @{ ExitCode = 0 }
    }

    function python {
        throw "Fixture failure after validating the installed payload"
    }

    $msi = Join-Path $Fixture "test.msi"
    New-FixtureFile -Path $msi
    $smoke = Join-Path $repoRoot "workflows/package/windows-msi/assets/scripts/smoke-test.ps1"
    $expectedFailure = if ($InstallFails) {
        "MSI install failed with exit code 1603"
    } else {
        "Fixture failure after validating the installed payload"
    }
    Assert-FailsWith -Action {
        & $smoke -MsiPath $msi -ExpectLauncher -ExpectDesktopInput:$Machine
    } -Message $expectedFailure
    Assert-Equal ($fixtureState.Actions -join ",") "/i,/x" "Uninstall runs after the smoke assertion fails"
    $expectedServiceChecks = if ($Machine) { 3 } else { 0 }
    Assert-Equal $fixtureState.ServiceChecks $expectedServiceChecks "Check machine service before install, after install and after uninstall"
}

function Test-DesktopInputServiceAuthoring {
    $source = Join-Path $repoRoot "workflows/package/windows-msi/assets/dockpipe.wxs"
    [xml]$installer = Get-Content -LiteralPath $source -Raw
    $namespaces = [Xml.XmlNamespaceManager]::new($installer.NameTable)
    $namespaces.AddNamespace("wix", "http://schemas.microsoft.com/wix/2006/wi")
    $service = $installer.SelectSingleNode("//wix:ServiceInstall[@Name='DockpipeDesktopInput']", $namespaces)
    $control = $service.ParentNode.SelectSingleNode("wix:ServiceControl", $namespaces)
    Assert-Equal $service.Start "auto" "Start on boot"
    Assert-Equal $service.Account "LocalSystem" "Machine service identity"
    Assert-Equal $control.GetAttribute("Start") "" "Install can complete before enrollment"
    Assert-Equal $control.Stop "both" "Stop before servicing or uninstall"
    Assert-Equal $control.Remove "uninstall" "Remove the machine service on uninstall"
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("dockpipe-msi-binding-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $testRoot | Out-Null
$originalEnvironment = @{}
foreach ($variable in @("DOCKPIPE_ADAPTER_0", "DOCKPIPE_WINDOWS_DESKTOP_INPUT_STAGE", "RUNNER_TEMP", "TEMP",
    "LOCALAPPDATA", "APPDATA", "ProgramW6432", "ProgramData")) {
    $originalEnvironment[$variable] = [Environment]::GetEnvironmentVariable($variable)
}

try {
    Test-MsiBinaryArchitecture -Fixture $testRoot
    Test-MsiLicenseRendering -Fixture $testRoot
    Test-PackageSnapshotComparison
    Test-DesktopInputServiceAuthoring
    Test-ReleaseEntrypoints -Fixture (Join-Path $testRoot "checkout with spaces")
    Test-SmokeCleanupAndScope -Fixture (Join-Path $testRoot "per user smoke") -Machine $false
    Test-SmokeCleanupAndScope -Fixture (Join-Path $testRoot "machine smoke") -Machine $true
    Test-SmokeCleanupAndScope -Fixture (Join-Path $testRoot "failed install") -Machine $false -InstallFails
} finally {
    foreach ($variable in $originalEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($variable, $originalEnvironment[$variable])
    }
    Remove-Item -LiteralPath $testRoot -Recurse -Force
}
Write-Host "MSI argument binding, scope, cleanup, license, architecture and snapshot checks passed"

& (Join-Path $PSScriptRoot '../../../build/desktop/tests/native-symbols.Tests.ps1')

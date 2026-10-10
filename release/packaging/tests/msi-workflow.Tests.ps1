# Exercise the release workflow's actual PowerShell calls against the MSI scripts'
# parameter contracts, without downloading WiX or installing an MSI.
$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$workflowLines = Get-Content -LiteralPath (Join-Path $repoRoot ".github/workflows/release.yml")

function Get-WorkflowScript {
    param([string]$StepName)

    $inStep = $false
    $inScript = $false
    $scriptLines = @()
    foreach ($line in $workflowLines) {
        if ($line -eq "      - name: $StepName") {
            $inStep = $true
            continue
        }
        if (-not $inStep) { continue }
        if ($line -eq "        run: |") {
            $inScript = $true
            continue
        }
        if (-not $inScript) { continue }
        if ($line.StartsWith("          ")) {
            $scriptLines += $line.Substring(10)
        } elseif ([string]::IsNullOrWhiteSpace($line)) {
            $scriptLines += ""
        } else {
            break
        }
    }
    if ($scriptLines.Count -eq 0) { throw "Workflow script not found: $StepName" }
    return $scriptLines -join "`n"
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

function Test-CoreSnapshotComparison {
    $tokens = $null
    $parseErrors = $null
    $source = Join-Path $repoRoot "release/packaging/msi/smoke-test.ps1"
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count -ne 0) { throw "Cannot parse MSI smoke test" }
    $comparisons = @($ast.FindAll({
        param($node)
        if ($node -isnot [System.Management.Automation.Language.IfStatementAst]) { return $false }
        $condition = $node.Clauses[0].Item1.Extent.Text
        return $condition.Contains('$currentCoreSnapshot') -and $condition.Contains('$baselineCoreSnapshot')
    }, $true))
    if ($comparisons.Count -ne 1) { throw "Expected one core snapshot comparison" }
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
        $baselineCoreSnapshot = $case.Baseline
        $currentCoreSnapshot = $case.Current
        $corePackageDir = "fixture core directory"
        $rejected = $false
        try {
            & $comparison
        } catch {
            if (-not $_.Exception.Message.StartsWith("Installed dockpipe core package contents did not return to baseline")) {
                throw
            }
            $rejected = $true
        }
        Assert-Equal $rejected $case.Reject ("Core snapshot " + $case.Name)
    }
}

Test-CoreSnapshotComparison

$buildScript = Get-WorkflowScript "Install WiX and build MSI"
$argumentStart = $buildScript.IndexOf('$v = ')
if ($argumentStart -lt 0) { throw "MSI build invocation not found" }
$buildScript = $buildScript.Substring($argumentStart).Replace('${{ needs.meta.outputs.version }}', '0.6.0')
$smokeScript = Get-WorkflowScript "Smoke test MSI install/uninstall"
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("dockpipe-msi-binding-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $testRoot | Out-Null

try {
    foreach ($withLauncher in @($false, $true)) {
        $fixture = Join-Path $testRoot ("checkout with spaces " + $withLauncher)
        foreach ($relative in @("release/packaging/msi", "release/artifacts/stores/windows-amd64", "src/bin")) {
            New-Item -ItemType Directory -Force -Path (Join-Path $fixture $relative) | Out-Null
        }
        $buildRecorder = Join-Path $fixture "release/packaging/msi/build.ps1"
        $smokeRecorder = Join-Path $fixture "release/packaging/msi/smoke-test.ps1"
        New-ParameterRecorder -Source (Join-Path $repoRoot "release/packaging/msi/build.ps1") -Destination $buildRecorder
        New-ParameterRecorder -Source (Join-Path $repoRoot "release/packaging/msi/smoke-test.ps1") -Destination $smokeRecorder
        Set-Content -LiteralPath (Join-Path $fixture "release/artifacts/stores/windows-amd64/dockpipe-core-0.6.0.tar.gz") -Value "fixture"
        Set-Content -LiteralPath (Join-Path $fixture "src/bin/dockpipe.exe") -Value "fixture"
        $launcherStage = Join-Path $fixture "bin/desktop-stage"
        $launcher = Join-Path $launcherStage "dockpipe-launcher.exe"
        if ($withLauncher) {
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $launcher) | Out-Null
            Set-Content -LiteralPath $launcher -Value "fixture"
            Set-Content -LiteralPath (Join-Path $launcherStage "Qt6Core.dll") -Value "fixture"
            New-Item -ItemType Directory -Path (Join-Path $launcherStage "platforms") | Out-Null
            Set-Content -LiteralPath (Join-Path $launcherStage "platforms/qwindows.dll") -Value "fixture"
        }
        $wixRoot = Join-Path $testRoot "WiX tools"
        Push-Location $fixture
        try {
            if (-not $withLauncher) {
                $rejected = $false
                try { & ([scriptblock]::Create($buildScript)) } catch {
                    if (-not $_.Exception.Message.StartsWith("Desktop MSI requires")) { throw }
                    $rejected = $true
                }
                Assert-Equal $rejected $true "Missing desktop must fail"
                continue
            }
            & ([scriptblock]::Create($buildScript))
            $build = Get-Content -Raw -LiteralPath ($buildRecorder + ".json") | ConvertFrom-Json
            Assert-Equal $build.Version "0.6.0" "Version"
            Assert-Equal $build.SourceExe (Join-Path $fixture "src/bin/dockpipe.exe") "SourceExe"
            Assert-Equal $build.OutDir (Join-Path $fixture "bin/msi-dist") "OutDir"
            Assert-Equal $build.CoreStageDir (Join-Path $fixture "bin/msi-core") "CoreStageDir"
            Assert-Equal $build.WixRoot $wixRoot "WixRoot"
            Assert-Equal $build.LauncherStageDir $launcherStage "LauncherStageDir"

            $msiPath = Join-Path $fixture "bin/msi-dist/dockpipe_0.6.0_windows_amd64.msi"
            Set-Content -LiteralPath $msiPath -Value "fixture"
            & ([scriptblock]::Create($smokeScript))
            $smoke = Get-Content -Raw -LiteralPath ($smokeRecorder + ".json") | ConvertFrom-Json
            Assert-Equal $smoke.MsiPath $msiPath "MsiPath"
            Assert-Equal $smoke.ExpectLauncher.IsPresent $true "ExpectLauncher"
        } finally {
            Pop-Location
        }
    }
} finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force
}
Write-Host "MSI snapshot comparison and workflow parameter binding passed"

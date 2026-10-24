param(
    [string]$RuntimeScript = (Join-Path $PSScriptRoot '../../src/core/assets/scripts/vmimage-run-qemu-windows.ps1'),
    [string]$TestRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ($PSVersionTable.PSVersion.Major -lt 7) {
    throw 'The native QEMU runtime regression requires PowerShell 7.'
}

$runtime = (Get-Item -LiteralPath $RuntimeScript).FullName
if ([string]::IsNullOrWhiteSpace($TestRoot)) {
    $TestRoot = Join-Path ([IO.Path]::GetTempPath()) ('dockpipe-qemu-exit-' + [guid]::NewGuid().ToString('N'))
}
if (Test-Path -LiteralPath $TestRoot) {
    throw 'The regression needs a fresh output directory.'
}
[void][IO.Directory]::CreateDirectory($TestRoot)
$TestRoot = (Get-Item -LiteralPath $TestRoot).FullName
$powerShell = [Diagnostics.Process]::GetCurrentProcess().Path
$childScript = Join-Path $TestRoot 'owned-child.ps1'
$childSource = @'
param([int]$Status)
[Console]::Out.WriteLine('fixture stdout ' + $Status)
[Console]::Error.WriteLine('fixture stderr ' + $Status)
exit $Status
'@
[IO.File]::WriteAllText($childScript, $childSource, [Text.UTF8Encoding]::new($false))
$results = [Collections.Generic.List[object]]::new()

function Invoke-ExitFixture {
    param([string]$Name, [int]$Status, [bool]$UnavailableReporter)

    $directory = Join-Path $TestRoot $Name
    [void][IO.Directory]::CreateDirectory($directory)
    $argumentsPath = Join-Path $directory 'arguments.txt'
    $pidPath = Join-Path $directory 'child.pid'
    $stdoutPath = Join-Path $directory 'backend.stdout'
    $stderrPath = Join-Path $directory 'backend.stderr'
    [IO.File]::WriteAllLines($argumentsPath, @('-NoProfile', '-NonInteractive', '-File', $childScript, '-Status', [string]$Status))

    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $powerShell
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    foreach ($argument in @('-NoProfile', '-NonInteractive', '-File', $runtime,
        '-Action', 'start-wait', '-QemuBin', $powerShell, '-PidFile', $pidPath,
        '-ArgsFile', $argumentsPath, '-StdOutFile', $stdoutPath, '-StdErrFile', $stderrPath)) {
        [void]$start.ArgumentList.Add($argument)
    }
    [void]$start.Environment.Remove('DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE')
    if ($UnavailableReporter) {
        $start.Environment['DOCKPIPE_VM_SESSION_REPORTER_EXECUTABLE'] = Join-Path $directory 'missing-reporter.exe'
    }

    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    try {
        if (-not $process.Start()) {
            throw 'The owned runtime fixture did not start.'
        }
        $heldHandle = $process.Handle
        $identity = [ordered]@{
            pid = $process.Id
            start_filetime = $process.StartTime.ToFileTimeUtc().ToString()
            executable = $powerShell
        }
        # These streams contain only fixed fixture messages and the fixed reporter
        # warning; the child source above cannot generate expandable output.
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit(20000)) {
            $process.Kill()
            throw 'The exact owned runtime child exceeded its 20-second deadline.'
        }
        $outerStdout = $stdoutTask.GetAwaiter().GetResult()
        $outerStderr = $stderrTask.GetAwaiter().GetResult()
        [IO.File]::WriteAllText((Join-Path $directory 'runtime.stdout'), $outerStdout)
        [IO.File]::WriteAllText((Join-Path $directory 'runtime.stderr'), $outerStderr)
        $capturedStdout = [IO.File]::ReadAllText($stdoutPath)
        $capturedStderr = [IO.File]::ReadAllText($stderrPath)
        $stdoutPreserved = $capturedStdout -ceq ('fixture stdout ' + $Status + [Environment]::NewLine)
        $stderrPreserved = $capturedStderr -ceq ('fixture stderr ' + $Status + [Environment]::NewLine)
        $reporterWarning = $outerStdout.Contains('VM session reporting failed; management status is unavailable.')
        $childPid = [int]([IO.File]::ReadAllText($pidPath).Trim())
        $passed = $process.ExitCode -eq $Status -and $stdoutPreserved -and $stderrPreserved -and $childPid -gt 0
        if ($UnavailableReporter -and -not $reporterWarning) {
            $passed = $false
        }
        $results.Add([ordered]@{
            name = $Name
            expected_exit_code = $Status
            actual_exit_code = $process.ExitCode
            stdout_preserved = $stdoutPreserved
            stderr_preserved = $stderrPreserved
            unavailable_reporter = $UnavailableReporter
            reporter_warning = $reporterWarning
            runtime_identity = $identity
            backend_pid = $childPid
            runtime_reaped = $true
            passed = $passed
        })
    } finally {
        $process.Dispose()
    }
}

Invoke-ExitFixture -Name 'normal-zero' -Status 0 -UnavailableReporter $false
Invoke-ExitFixture -Name 'backend-failure' -Status 37 -UnavailableReporter $false
Invoke-ExitFixture -Name 'backend-failure-reporter-unavailable' -Status 37 -UnavailableReporter $true
$passed = @($results | Where-Object { -not $_.passed }).Count -eq 0
$receipt = [ordered]@{
    schema = 1
    success = $passed
    platform = [Environment]::OSVersion.Platform.ToString()
    powershell_version = $PSVersionTable.PSVersion.ToString()
    runtime_script = $runtime
    runtime_sha256 = (Get-FileHash -LiteralPath $runtime -Algorithm SHA256).Hash.ToLowerInvariant()
    cases = $results.ToArray()
    VM_started = $false
    cancellation_handler_modified = $false
}
[IO.File]::WriteAllText((Join-Path $TestRoot 'receipt.json'), ($receipt | ConvertTo-Json -Depth 7), [Text.UTF8Encoding]::new($false))
[Console]::Out.WriteLine('QEMU runtime exit-status cases passed: ' + $passed)
if (-not $passed) {
    exit 1
}

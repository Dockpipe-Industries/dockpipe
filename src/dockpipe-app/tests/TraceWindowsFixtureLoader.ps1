param(
    [Parameter(Mandatory = $true)]
    [string] $BuildDirectory
)

$ErrorActionPreference = 'Stop'
$debugger = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits/10/Debuggers/x64/cdb.exe'
if (-not (Test-Path -LiteralPath $debugger -PathType Leaf)) {
    $command = Get-Command cdb.exe -ErrorAction SilentlyContinue
    if (-not $command) {
        Write-Warning 'CDB is unavailable; no loader replay was performed.'
        return
    }
    $debugger = $command.Source
}
Write-Host "Loader debugger: $debugger version=$((Get-Item -LiteralPath $debugger).VersionInfo.FileVersion)"

$traceDirectory = Join-Path $BuildDirectory 'windows-loader-trace'
New-Item -ItemType Directory -Path $traceDirectory -ErrorAction Stop | Out-Null
$commands = Join-Path $traceDirectory 'commands.txt'
$inputFile = Join-Path $traceDirectory 'stdin.txt'
$outputFile = Join-Path $traceDirectory 'stdout.txt'
$errorFile = Join-Path $traceDirectory 'stderr.txt'
# Initial breakpoints precede DLL initializers. Set loader snaps in each
# debuggee's memory, including descendants, without changing registry flags.
@'
sxe -c ".printf \"LOADER_INITIAL_BREAK pid=%x\\n\", @$tpid; !gflag +sls; !gflag; g" ibp
sxe -c ".printf \"LOADER_CHILD_CREATE pid=%x\\n\", @$tpid; !gflag +sls; !gflag; g" cpr
.printf "LOADER_ROOT pid=%x\n", @$tpid
!gflag +sls
!gflag
g
'@ | Set-Content -LiteralPath $commands -Encoding ascii
Set-Content -LiteralPath $inputFile -Value '' -Encoding ascii
$launcher = Join-Path $BuildDirectory 'windows-test-launcher.exe'
$test = Join-Path $BuildDirectory 'desktop-app-window-win-test.exe'
$arguments = @('-o', '-hd', '-G', '-cf', "`"$commands`"", "`"$launcher`"", "`"$test`"")
$process = Start-Process -FilePath $debugger -ArgumentList $arguments `
    -WorkingDirectory $BuildDirectory -NoNewWindow -PassThru `
    -RedirectStandardInput $inputFile -RedirectStandardOutput $outputFile -RedirectStandardError $errorFile
$deadline = [System.Diagnostics.Stopwatch]::StartNew()
try {
    while (-not $process.WaitForExit(250)) {
        $outputBytes = (Get-Item -LiteralPath $outputFile).Length
        $errorBytes = (Get-Item -LiteralPath $errorFile).Length
        if ($deadline.Elapsed.TotalSeconds -ge 60 -or $outputBytes -gt 10MB -or $errorBytes -gt 1MB) {
            Write-Warning 'Loader replay reached its time or output limit; terminating its owned process tree.'
            $process.Kill($true)
            if (-not $process.WaitForExit(5000)) {
                throw 'Loader debugger cleanup exceeded five seconds.'
            }
            break
        }
    }
    Write-Host "Loader replay exit=$($process.ExitCode) elapsedSeconds=$($deadline.Elapsed.TotalSeconds)"
} finally {
    if (-not $process.HasExited) {
        $process.Kill($true)
        $process.WaitForExit(5000) | Out-Null
    }
    $process.Dispose()
    foreach ($log in @($outputFile, $errorFile)) {
        Write-Host "Loader diagnostic: $log"
        Select-String -LiteralPath $log -Pattern 'LOADER_|ShowLoaderSnaps|c0000142|failed|ERROR' |
            Select-Object -First 120 | ForEach-Object { Write-Host $_.Line }
        Get-Content -LiteralPath $log -Tail 160
    }
}

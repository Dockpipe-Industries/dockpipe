$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrEmpty($env:TRACE) -or -not [System.IO.Path]::IsPathRooted($env:TRACE)) {
    throw 'Select an absolute task-owned TRACE path'
}

function Add-ControlTrace {
    param([Parameter(Mandatory = $true)][string]$Line)

    $encoding = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::AppendAllText($env:TRACE, "$Line`n", $encoding)
}

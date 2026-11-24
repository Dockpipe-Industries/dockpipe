. (Join-Path $PSScriptRoot 'trace.ps1')

Add-ControlTrace 'test'
if ($env:FAIL_TEST -eq 'true') {
    exit 23
}

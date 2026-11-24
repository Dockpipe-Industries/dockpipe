. (Join-Path $PSScriptRoot 'trace.ps1')

if ([string]::IsNullOrEmpty($env:TARGET)) {
    throw 'The source controller must supply a target'
}
Add-ControlTrace "build:$($env:TARGET)"

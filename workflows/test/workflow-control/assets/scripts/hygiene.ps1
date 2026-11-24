. (Join-Path $PSScriptRoot 'trace.ps1')

Add-ControlTrace "hygiene:$($env:ARG_A):$($env:ARG_B):$($env:ARG_C)"

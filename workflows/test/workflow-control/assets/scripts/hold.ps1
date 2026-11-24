. (Join-Path $PSScriptRoot 'trace.ps1')

Add-ControlTrace 'ready'
while ($true) {
    Start-Sleep -Seconds 1
}

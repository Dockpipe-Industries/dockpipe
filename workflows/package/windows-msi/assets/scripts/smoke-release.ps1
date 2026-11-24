$ErrorActionPreference = "Stop"
$version = $env:DOCKPIPE_ADAPTER_0
if (-not $version) {
  throw "DOCKPIPE_ADAPTER_0 must specify the MSI release version"
}

$smokeArgs = @{
  MsiPath = Join-Path $PWD "bin/msi-dist/dockpipe_${version}_windows_amd64.msi"
  ExpectLauncher = $true
  ExpectDesktopInput = [bool]$env:DOCKPIPE_WINDOWS_DESKTOP_INPUT_STAGE
}
& (Join-Path $PSScriptRoot "smoke-test.ps1") @smokeArgs

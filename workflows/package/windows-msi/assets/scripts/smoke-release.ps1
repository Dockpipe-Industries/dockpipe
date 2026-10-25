$ErrorActionPreference = "Stop"
$msi = Get-ChildItem -LiteralPath (Join-Path $PWD "bin\msi-dist") -Filter "*.msi" | Select-Object -First 1
if (-not $msi) {
  throw "No MSI found under bin\\msi-dist"
}
$smokeArgs = @{ MsiPath = $msi.FullName; ExpectLauncher = $true }
& (Join-Path $PWD "workflows\package\windows-msi\assets\scripts\smoke-test.ps1") @smokeArgs

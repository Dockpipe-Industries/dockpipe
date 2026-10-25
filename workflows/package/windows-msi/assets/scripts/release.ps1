$ErrorActionPreference = "Stop"
$ProgressPreference = 'SilentlyContinue'
$zip = Join-Path $env:TEMP "wix314.zip"
Invoke-WebRequest -Uri "https://github.com/wixtoolset/wix3/releases/download/wix3141rtm/wix314-binaries.zip" -OutFile $zip
$dest = Join-Path $env:RUNNER_TEMP "wix314-expand"
if (Test-Path -LiteralPath $dest) { Remove-Item -LiteralPath $dest -Recurse -Force }
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Expand-Archive -Path $zip -DestinationPath $dest -Force
$destFull = (Resolve-Path -LiteralPath $dest).Path.TrimEnd('\')
# wix314-binaries.zip puts candle.exe + light.exe at the extract root (not bin\). build.ps1 supports both layouts.
$rootCandle = Join-Path $destFull "candle.exe"
$binCandle = Join-Path $destFull "bin\candle.exe"
if (-not ((Test-Path -LiteralPath $rootCandle) -or (Test-Path -LiteralPath $binCandle))) {
  throw "candle.exe not found at WiX zip root or under bin\ at $destFull"
}
$wixRoot = $destFull
$v = "$env:DOCKPIPE_ADAPTER_0"
$launcherStage = Join-Path $PWD "bin/desktop-stage"
foreach ($required in @("dockpipe-launcher.exe", "Qt6Core.dll", "platforms/qwindows.dll")) {
  if (-not (Test-Path -LiteralPath (Join-Path $launcherStage $required))) {
    throw "Desktop MSI requires the deployed launcher runtime: $required"
  }
}
New-Item -ItemType Directory -Force -Path bin\msi-dist | Out-Null
$buildArgs = @{
  Version = $v
  SourceExe = Join-Path $PWD "src\bin\dockpipe.exe"
  LauncherStageDir = $launcherStage
  OutDir = Join-Path $PWD "bin\msi-dist"
  WixRoot = $wixRoot
}
& (Join-Path $PWD "workflows\package\windows-msi\assets\scripts\build.ps1") @buildArgs

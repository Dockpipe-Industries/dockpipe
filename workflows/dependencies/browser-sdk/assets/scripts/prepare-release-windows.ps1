$ErrorActionPreference = "Stop"
python workflows/dependencies/browser-sdk/assets/scripts/prepare-browser-sdk.py --lock workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json `
  --platform windows-amd64 --output "$env:RUNNER_TEMP/browser-sdk"
if ($LASTEXITCODE -ne 0) { throw "Browser SDK preparation failed" }
"DOCKPIPE_BROWSER_SDK=$env:RUNNER_TEMP/browser-sdk" | Out-File -FilePath $env:GITHUB_ENV -Append -Encoding utf8

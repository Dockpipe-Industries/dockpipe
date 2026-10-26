$ErrorActionPreference = "Stop"
./workflows/build/desktop/assets/scripts/build-windows.ps1 -Version "$env:DOCKPIPE_ADAPTER_0" -StageDir (Join-Path $PWD "bin/desktop-stage") -EnableQualification:($env:QUALIFY_PACKAGE_DISPLAY -eq 'true')

      # One pwsh step: GITHUB_ENV mangles/truncates Windows paths for WIX — pass -WixRoot into build.ps1 instead.

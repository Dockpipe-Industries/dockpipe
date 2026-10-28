param([int]$Jobs = 2)
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $env:DOCKPIPE_SOURCE_ROOT

$ErrorActionPreference = 'Stop'
$vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/vswhere.exe'
$visualStudio = & $vswhere -latest -products '*' -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
if (-not $visualStudio) { throw 'Visual Studio C++ tools are required.' }
& (Join-Path $visualStudio 'Common7/Tools/Launch-VsDevShell.ps1') -Arch amd64 -HostArch amd64 -SkipAutomaticLocation
if (-not $env:VCToolsInstallDir) { throw 'The Visual Studio developer environment is incomplete.' }
$launcher = Join-Path $env:RUNNER_TEMP 'windows-test-launcher.exe'
$object = Join-Path $env:RUNNER_TEMP 'windows-test-launcher.obj'
cl.exe /nologo /std:c++17 /EHsc /W4 /DUNICODE /D_UNICODE `
  src/dockpipe-app/tests/WindowsTestLauncher.cpp `
  "/Fe:$launcher" "/Fo:$object" /link advapi32.lib user32.lib
if ($LASTEXITCODE -ne 0) { throw 'Windows test launcher compilation failed.' }
& $launcher --self-test
if ($LASTEXITCODE -ne 0) { throw 'Windows test launcher qualification failed.' }

$env:GOMAXPROCS = [string]$Jobs
& go test -p $Jobs ./src/lib/infrastructure -run '^TestWorkspaceGit' -count=1
if ($LASTEXITCODE -ne 0) { throw 'Native Git runtime qualification failed.' }
$gitCli = Join-Path $env:RUNNER_TEMP 'dockpipe-git-runtime.exe'
& go build -p $Jobs -trimpath -ldflags='-s -w' -o $gitCli ./src/cmd
if ($LASTEXITCODE -ne 0) { throw 'Native Git runtime compilation failed.' }
$env:DOCKPIPE_GIT_TEST_CLI = $gitCli

$ErrorActionPreference = 'Stop'
python -m venv "$env:RUNNER_TEMP/aqt-terminal"
if ($LASTEXITCODE -ne 0) { throw 'Could not create the Qt installer environment.' }
$python = "$env:RUNNER_TEMP/aqt-terminal/Scripts/python.exe"
& $python -m pip install aqtinstall==3.3.0
if ($LASTEXITCODE -ne 0) { throw 'Could not install the pinned Qt installer.' }
& $python -m aqt install-qt windows desktop 6.10.3 win64_msvc2022_64 --outputdir "$env:RUNNER_TEMP/Qt" --modules qtwebengine qtwebchannel qtpositioning
if ($LASTEXITCODE -ne 0) { throw 'Could not install release Qt.' }
$env:PATH = "$env:RUNNER_TEMP/Qt/6.10.3/msvc2022_64/bin;$env:PATH"
$env:CMAKE_PREFIX_PATH = "$env:RUNNER_TEMP/Qt/6.10.3/msvc2022_64"

$ErrorActionPreference = 'Stop'
& $env:DOCKPIPE_BIN --workflow-file workflows/dependencies/browser-sdk/config.yml --workdir $env:DOCKPIPE_SOURCE_ROOT -- `
  --lock workflows/dependencies/browser-sdk/assets/spatialweb-browser.lock.json `
  --platform windows-amd64 --output "$env:RUNNER_TEMP/browser-sdk"
if ($LASTEXITCODE -ne 0) { throw 'Browser SDK preparation failed.' }
$env:CMAKE_PREFIX_PATH = "$env:RUNNER_TEMP/browser-sdk;$env:CMAKE_PREFIX_PATH"
$env:PATH = "$env:RUNNER_TEMP/browser-sdk/bin;$env:PATH"

$ErrorActionPreference = 'Stop'
$build = "$env:RUNNER_TEMP/dockpipe-native-terminal"
$vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/vswhere.exe'
$visualStudio = & $vswhere -latest -products '*' -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
if (-not $visualStudio) { throw 'Visual Studio C++ tools are required.' }
$developerShell = Join-Path $visualStudio 'Common7/Tools/Launch-VsDevShell.ps1'
& $developerShell -Arch amd64 -HostArch amd64 -SkipAutomaticLocation
if (-not $env:VCToolsInstallDir) { throw 'The Visual Studio developer environment is incomplete.' }
cmake -S src/dockpipe-app -B $build -G 'NMake Makefiles' -DBUILD_TESTING=ON -DCMAKE_BUILD_TYPE=Release
if ($LASTEXITCODE -ne 0) { throw 'Native terminal configuration failed.' }
# Qualify the Core-only transports before compiling the full desktop.
cmake --build $build --config Release --parallel $Jobs --target terminal-session-win-test hardware-key-transport-test
if ($LASTEXITCODE -ne 0) { throw 'Native terminal transport compilation failed.' }
ctest --test-dir $build -C Release --verbose --no-tests=error --parallel $Jobs -R '^(terminal-session-win|hardware-key-transport)$'
$transportExit = $LASTEXITCODE
$transportLog = Join-Path $build 'native-terminal-session-results.txt'
if (Test-Path -LiteralPath $transportLog) { Get-Content -LiteralPath $transportLog }
if ($transportExit -ne 0) { throw 'Native terminal or security-key transport qualification failed.' }
# Isolate native embedding startup before compiling every desktop test.
cmake --build $build --config Release --parallel $Jobs --target desktop-app-window-win-test
if ($LASTEXITCODE -ne 0) { throw 'Native embedding fixture compilation failed.' }
# Match the installed application: standard-user fixtures use an exact
# app-local Browser DLL instead of depending on the SDK directory ACLs.
python -B workflows/dependencies/browser-sdk/assets/scripts/browser-runtime.py `
  --sdk "$env:RUNNER_TEMP/browser-sdk" --platform windows-amd64 --runtime $build
if ($LASTEXITCODE -ne 0) { throw 'Native test Browser runtime deployment failed.' }
# Report effective static imports without replacing the native test gate.
try {
    $dumpbin = (Get-Command dumpbin.exe -ErrorAction Stop).Source
    $cmake = (Get-Command cmake -ErrorAction Stop).Source
    $scanScript = (Resolve-Path src/dockpipe-app/tests/WindowsRuntimeDependencies.cmake).Path
    $scanArguments = @(
        "`"-DTEST_EXECUTABLE=$build/desktop-app-window-win-test.exe`"",
        "`"-DDUMPBIN=$dumpbin`"",
        '-P', "`"$scanScript`""
    )
    $scan = Start-Process -FilePath $cmake -ArgumentList $scanArguments -NoNewWindow -PassThru
    try {
        if (-not $scan.WaitForExit(60000)) {
            $scan.Kill($true)
            throw 'Windows import diagnostics exceeded the 60-second deadline.'
        }
        if ($scan.ExitCode -ne 0) { throw 'Windows import diagnostics failed.' }
    } finally {
        $scan.Dispose()
    }
    Push-Location -LiteralPath $build
    try {
        & (Join-Path $build 'windows-test-launcher.exe') --probe-runtime (Join-Path $build 'windows-runtime-dependencies.txt')
        if ($LASTEXITCODE -ne 0) { Write-Warning 'Restricted runtime access diagnostics failed; native test follows.' }
    } finally {
        Pop-Location
    }
} catch {
    # Diagnostics must never replace the required native embedding gate.
    Write-Warning "Windows import diagnostics failed: $_"
}
$previousPluginDiagnostics = $env:QT_DEBUG_PLUGINS
try {
  $env:QT_DEBUG_PLUGINS = '1'
  $embeddingReport = Join-Path $build ("native-embedding-" + [guid]::NewGuid().ToString("N") + ".xml")
  ctest --test-dir $build -C Release --verbose --no-tests=error --parallel $Jobs --output-junit $embeddingReport -R '^desktop-app-window-win$' 
  if ($LASTEXITCODE -ne 0) {
    try {
      & ./src/dockpipe-app/tests/TraceWindowsFixtureLoader.ps1 -BuildDirectory $build
    } catch {
      Write-Warning "Loader diagnostics failed: $_"
    }
    throw 'Native embedding qualification failed.'
  }
  python workflows/shared/ctest_result.py --report $embeddingReport --require desktop-app-window-win
  if ($LASTEXITCODE -ne 0) { throw 'Native embedding did not produce a genuine non-skipped pass.' }
} finally {
  $env:QT_DEBUG_PLUGINS = $previousPluginDiagnostics
}
cmake --build $build --config Release --parallel $Jobs
if ($LASTEXITCODE -ne 0) { throw 'Native terminal compilation failed.' }
# Exercise the same full desktop suite that gates the Windows release.
ctest --test-dir $build -C Release --verbose --no-tests=error --parallel $Jobs --timeout 120 --stop-on-failure
$qualificationExit = $LASTEXITCODE
foreach ($testLog in Get-ChildItem -LiteralPath $build -Filter 'native-*-results.txt') {
  Write-Host "Qt test results: $($testLog.Name)"
  Get-Content -LiteralPath $testLog.FullName
}
if ($qualificationExit -ne 0) { throw 'Native terminal qualification failed.' }

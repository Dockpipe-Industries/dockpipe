# Offline regression test; loads only the install helper, never runs the installer.
$ErrorActionPreference = 'Stop'
$installer = Join-Path $PSScriptRoot '../windows/install.ps1'
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($installer, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) { throw "Installer parse errors: $parseErrors" }
$function = $ast.Find({ param($node)
    $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Install-VerifiedReleaseAsset'
}, $true)
if ($null -eq $function) { throw 'Installer publication helper not found' }
. ([scriptblock]::Create($function.Extent.Text))

$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('dockpipe install test ' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    $source = Join-Path $testRoot 'source'
    $destination = Join-Path $testRoot 'core.tar.gz'
    [System.IO.File]::WriteAllText($source, 'verified-package')
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $source).Hash
    $asset = @{ name = 'core.tar.gz'; browser_download_url = 'https://example.invalid/core.tar.gz' }
    function Invoke-WebRequest {
        param($Uri, $OutFile, [switch]$UseBasicParsing)
        [System.IO.File]::WriteAllText($OutFile, $script:downloadContent)
        if ($script:interruptDownload) { throw 'Simulated interrupted download' }
    }
    foreach ($scenario in @('mismatch', 'interrupted', 'valid', 'first-install')) {
        [System.IO.File]::WriteAllText($destination, 'previous-package')
        if ($scenario -eq 'first-install') { Remove-Item -LiteralPath $destination }
        $script:downloadContent = if ($scenario -eq 'mismatch') { 'damaged-package' } else { 'verified-package' }
        $script:interruptDownload = $scenario -eq 'interrupted'
        $failed = $false
        try {
            Install-VerifiedReleaseAsset -Asset $asset -ExpectedHash $hash -Destination $destination
        } catch {
            $failed = $true
            if ($scenario -in @('valid', 'first-install')) { throw }
        }
        $expectedFailure = $scenario -in @('mismatch', 'interrupted')
        if ($failed -ne $expectedFailure) { throw "Unexpected outcome for $scenario" }
        $expected = if ($expectedFailure) { 'previous-package' } else { 'verified-package' }
        if ([System.IO.File]::ReadAllText($destination) -ne $expected) { throw "Installed package damaged by $scenario" }
        if (@(Get-ChildItem -LiteralPath $testRoot -Filter '.dockpipe-download-*' -Force).Count -ne 0) { throw "Temporary download leaked after $scenario" }
    }
    Write-Host 'Windows installer publication tests passed'
} finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force
}

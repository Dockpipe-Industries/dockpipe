$ErrorActionPreference = 'Stop'
$script = Join-Path $PSScriptRoot '../desktop/build-windows.ps1'
$tokens = $null
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($script, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw $errors }
$function = $ast.Find({ param($node)
    $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Assert-PublicDesktopRuntime'
}, $true)
. ([scriptblock]::Create($function.Extent.Text))
$directory = Join-Path ([System.IO.Path]::GetTempPath()) ('dockpipe-stage-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path (Join-Path $directory 'nested') | Out-Null
try {
    [System.IO.File]::WriteAllText((Join-Path $directory 'dockpipe-launcher.exe'), 'release executable')
    Assert-PublicDesktopRuntime -RuntimeDirectory $directory
    foreach ($extension in @('.PDB', '.obj', '.ilk', '.exp', '.lib', '.pch', '.iobj', '.ipdb', '.idb')) {
        $privateFile = Join-Path $directory "nested/private$extension"
        [System.IO.File]::WriteAllText($privateFile, 'private build file')
        $rejected = $false
        try { Assert-PublicDesktopRuntime -RuntimeDirectory $directory } catch { $rejected = $true }
        if (-not $rejected) { throw "Private extension accepted: $extension" }
        Remove-Item -LiteralPath $privateFile
    }
    Write-Host 'Public Windows native stage boundary passed'
} finally {
    Remove-Item -LiteralPath $directory -Recurse -Force
}

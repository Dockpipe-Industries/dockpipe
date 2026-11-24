$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrEmpty($env:JOURNEY_CORRELATION)) {
    throw 'An explicit invocation correlation is required'
}

$encoding = [System.Text.UTF8Encoding]::new($false)
[System.IO.File]::WriteAllText(
    [System.IO.Path]::Combine((Get-Location).ProviderPath, 'after.receipt'),
    "$($env:JOURNEY_CORRELATION)`n", $encoding)
[Console]::Out.WriteLine("after:$($env:JOURNEY_CORRELATION)")

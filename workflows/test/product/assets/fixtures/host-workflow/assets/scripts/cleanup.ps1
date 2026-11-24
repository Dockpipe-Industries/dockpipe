$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrEmpty($env:JOURNEY_CORRELATION)) {
    throw 'An explicit invocation correlation is required'
}

$artifactDirectory = (Get-Location).ProviderPath
[System.IO.File]::Delete([System.IO.Path]::Combine($artifactDirectory, 'resource.marker'))
$encoding = [System.Text.UTF8Encoding]::new($false)
[System.IO.File]::WriteAllText(
    [System.IO.Path]::Combine($artifactDirectory, 'cleanup.receipt'),
    "$($env:JOURNEY_CORRELATION)`n", $encoding)
[Console]::Out.WriteLine("cleanup:$($env:JOURNEY_CORRELATION)")

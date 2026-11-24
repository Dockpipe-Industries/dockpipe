$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrEmpty($env:JOURNEY_CORRELATION)) {
    throw 'An explicit invocation correlation is required'
}

$artifactDirectory = (Get-Location).ProviderPath
$encoding = [System.Text.UTF8Encoding]::new($false)
[Console]::Out.WriteLine("producer:$($env:JOURNEY_CORRELATION)")
[Console]::Error.WriteLine("producer-error:$($env:JOURNEY_CORRELATION)")
[System.IO.File]::WriteAllText(
    [System.IO.Path]::Combine($artifactDirectory, 'values.env'),
    "MERGED_CORRELATION=$($env:JOURNEY_CORRELATION)`n", $encoding)
[System.IO.File]::WriteAllText(
    [System.IO.Path]::Combine($artifactDirectory, 'resource.marker'),
    "$($env:JOURNEY_CORRELATION)`n", $encoding)

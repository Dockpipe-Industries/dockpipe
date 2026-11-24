$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrEmpty($env:MERGED_CORRELATION)) {
    throw 'The producer output must reach this step'
}
if ([string]::IsNullOrEmpty($env:JOURNEY_MODE)) {
    throw 'An explicit fixture mode is required'
}

[Console]::Out.WriteLine("consumer:$($env:MERGED_CORRELATION)")
switch -Exact ($env:JOURNEY_MODE) {
    'fail' {
        exit 23
    }
    'success' {
        exit 0
    }
    'hold' {
        Start-Sleep -Seconds 30
    }
    default {
        exit 24
    }
}

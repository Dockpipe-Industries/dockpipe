<!--
Copy this file to release/releasenotes/<VERSION>.md, using the repo-root VERSION
as the notes baseline. Replace X.Y.Z throughout. The release workflow substitutes
the generated numeric version into the published body; do not hardcode a staging
candidate or guess the next patch. Remove these instructions and unused sections.

Write for someone installing and using Dockpipe. Lead with observable changes,
then installation, upgrade actions and limitations. Verify claims against the
implementation, artifact inventory and relevant platform/provider evidence.
Keep source-build instructions, internal task IDs and CI chronology in contributor
docs. Mark preview/staging capabilities explicitly. Do not promise publication,
signing, provider acceptance or platform support from a successful build alone.
-->

# Dockpipe vX.Y.Z

<!-- One short paragraph describing what users gain from this release. -->

## What's new

<!-- Group related user outcomes. Explain prerequisites and link to setup guides.
For a fix, describe the symptom and the corrected behavior, not just an internal
component name. Do not advertise every optional package as bundled or enabled. -->

## Install

Choose assets for your operating system and CPU. Native host steps need Bash
(Git for Windows supplies it on Windows). Container workflows additionally need a
reachable container engine. Install provider tools only for workflows that use them.

### Linux

```sh
curl -fsSL https://github.com/Dockpipe-Industries/dockpipe/releases/download/vX.Y.Z/install.sh -o /tmp/dockpipe-install.sh
DOCKPIPE_VERSION=X.Y.Z sh /tmp/dockpipe-install.sh
export PATH="$HOME/.local/bin:$PATH"
```

<!-- Name the desktop asset and verified minimum OS versions when offered.
Distinguish CLI-only package formats from desktop packages. -->

### macOS

```sh
curl -fsSL https://github.com/Dockpipe-Industries/dockpipe/releases/download/vX.Y.Z/install.sh -o /tmp/dockpipe-install.sh
DOCKPIPE_VERSION=X.Y.Z sh /tmp/dockpipe-install.sh
export PATH="$HOME/.local/bin:$PATH"
```

<!-- State desktop availability, minimum OS, signing/notarization and conflicts
between installation methods. Include Homebrew only for a verified channel. -->

### Windows

```powershell
$installer = Join-Path $env:TEMP 'dockpipe-install.ps1'
Invoke-WebRequest https://github.com/Dockpipe-Industries/dockpipe/releases/download/vX.Y.Z/install.ps1 -OutFile $installer
& $installer -Version X.Y.Z -SkipWSLSetup
```

Open a new terminal after installation. WSL is optional; this command skips setup.

<!-- Describe the MSI/ZIP and launcher choices actually attached to this release,
and the current signing status. -->

The install scripts verify downloaded assets against the release checksums. For
manual downloads, check `SHA256SUMS.txt` from the same release before installation.
See the [installation guide](https://github.com/Dockpipe-Industries/dockpipe/blob/vX.Y.Z/docs/install.md)
for platform details and any separately labelled staging channels.

## Get started

```sh
dockpipe --version
```

Follow [your first workflow](https://github.com/Dockpipe-Industries/dockpipe/blob/vX.Y.Z/docs/onboarding.md).

<!-- Optionally include one tested installed-user example. Do not use repo-only
workflows such as internal CI tests or commands requiring a source checkout. -->

## Upgrade notes

<!-- State the previous version/configuration affected and concrete action needed.
Cover package dependencies, channel selection, state migration and changed defaults
where relevant. Distinguish retained compatibility from actual breaking changes. -->

## Known limitations

<!-- Include limits that affect adoption: previews, unsupported combinations,
provider prerequisites and unverified native behavior. Do not bury them in internal
qualification links or describe unfinished features as shipped. -->

[Report a problem](https://github.com/Dockpipe-Industries/dockpipe/issues) with your
Dockpipe version, platform and steps to reproduce it.

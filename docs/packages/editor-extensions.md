# Dockpipe editor extensions

Dockpipe Language Support and the DorkPipe companion extension are owned by the
public [DorkPipe repository](https://github.com/Dockpipe-Industries/dorkpipe) under `tooling/vscode-extensions/`. They are optional
editor integrations. Dockpipe core builds and runs independently of their source.

## Proposed consolidation (backlog; awaiting review)

The 2026-10-17 product direction is one DockPipe editor integration covering
DockPipe workflow/package authoring, PipeLang language support and optional
DockPipe-related AI chat through DorkPipe. DorkPipe retains extension source ownership;
DockPipe branding does not move AI logic into core or require DorkPipe for basic
language support. Chat uses the same package operations, project/version context,
permissions and session contracts as DockPipe's desktop chat.

Pipeon retirement and consolidation of the companion extension are proposals, not
completed migrations. Before implementation, audit extension identities, settings,
commands, saved sessions, existing installs, artifact pins and rollback. Decide whether
to extend the language extension or provide an explicit migration; no extension ID
rename, removal, marketplace publication or package retirement is authorized by this
backlog record. See [direction review](../agents/tasks/agentic-app-ui/direction-review.md).

## Current delivery

Published extension identities remain `dockpipe.dockpipe-language-support` and
`dorkpipe.pipeon`. The companion retains its existing name and settings for
compatibility; Pipeon product development is paused.

The reviewed staging release provides [Dockpipe Language Support 0.0.2](https://packages.staging.dockpipe.com/packages/standalone/dorkpipe/candidates/0.6.5-staging.cc31cf0aa848/dockpipe-language-support-0.0.2.vsix)
and the [companion extension 0.0.7](https://packages.staging.dockpipe.com/packages/standalone/dorkpipe/candidates/0.6.5-staging.cc31cf0aa848/pipeon-0.0.7.vsix).
Verify the download against that release's [SHA256SUMS](https://packages.staging.dockpipe.com/packages/standalone/dorkpipe/candidates/0.6.5-staging.cc31cf0aa848/SHA256SUMS),
then use your editor's **Install from VSIX** action. These downloads do not require
source-repository credentials. Neither extension is published to an
editor marketplace by this release.

Maintainers build and test both extensions from the DorkPipe repository root
with its pinned DockPipe compiler:

```sh
"$DOCKPIPE_BIN" --workflow-file workflows/editor.candidate/config.yml --workdir .
```

The output contains versioned VSIX files and `extensions.json` with sizes and
SHA-256 digests. The standalone release pipeline tests and freezes these files,
then publishes the reviewed artifacts to the matching package source. Source
repository visibility and package distribution are separate choices.

Pipeon packaging accepts `DOCKPIPE_LANGUAGE_VSIX`, `DOCKPIPE_LANGUAGE_VSIX_SHA256`,
`PIPEON_VSIX`, and `PIPEON_VSIX_SHA256`. Select each immutable artifact and its digest
from the approved release manifest. Both selections must also match
`packages/pipeon/assets/editor-dependencies.lock.json`, which ships with the
consumer package. The importer checks bytes, package identity and the exact
locked version before staging the selected artifact. Updating environment
variables cannot override this lock. An upgrade changes the committed lock and
runs the consumer's editor-pin tests and packaging checks. It does not compile extensions
or select an arbitrary previously built VSIX.

For the contained PipeLang integration campaign, extract the published
`dockpipe-language-test-sdk-<version>.tar.gz` and set `DORKPIPE_EDITOR_TEST_SOURCE`
to that absolute directory. `release/editor-source.lock.json` pins its editor
version, DorkPipe source commit, source inventory and manifest digest. The campaign
checks those pins and includes the admitted source in its input fingerprint.
It requires the actual Node editor tests to pass. Ordinary core tests do not
need this SDK.

When workflow/package authoring contracts change, update the engine parser/schema,
canonical docs and the language extension in its owning repository together. For
example, installable package manifests support `hidden: true`, with completion and
hover help in the language extension. See [Package model](package-model.md).

# Dockpipe editor extensions

Dockpipe owns [Dockpipe Language Support](../../tooling/vscode-extensions/dockpipe-language-support/README.md)
(`dockpipe.dockpipe-language-support`) for workflow/package YAML, project JSON,
typed-model integration and portable SDK snippets. PipeLang owns the separate
[`dockpipe.pipelang-language-support`](https://github.com/Dockpipe-Industries/pipelang/tree/dev/tooling/vscode-extensions/pipelang-language-support)
extension for `.pipe` grammar, snippets and standalone compiler diagnostics.
Dockpipe declares that extension as a dependency. DorkPipe retains its companion
extension and AI behavior in its own repository. The CLI runs independently of
editor extensions.

## Maintainer build and publication

Run the owning repository's `editor.build` workflow with its pinned Dockpipe CLI:

```sh
"$DOCKPIPE_BIN" --workflow-file workflows/editor.build/config.yml --workdir . -- --out /absolute/path/to/fresh-vsix-output
```

Node 22 or later is required. The workflow uses locked npm/vsce dependencies,
runs editor and artifact-contract tests, then emits a versioned VSIX and
`extension.json` binding its identity, source inventory, size and SHA-256.
The restored local candidates are Dockpipe 0.0.3 and PipeLang 0.0.1. Their
qualification does not claim native VS Code extension-host testing or Marketplace
publication. Selected working-source provenance is recorded in
[RESTORATION-SOURCES.json](../../tooling/vscode-extensions/RESTORATION-SOURCES.json).

Each repository has its own Marketplace GitHub Action and Dockpipe publication
workflow. Manual publication selects a successful candidate run and its exact
version and checksum, verifies that artifact against the source inventory, and
publishes the same VSIX using the `VSCE_PAT` secret reference. Publish PipeLang
first because Dockpipe depends on it. See the
[editor workflow contract](../../workflows/editor/README.md) for credential setup
and explicit installation/publication options. Ordinary builds do not upload or
install extensions. No external editor-source checkout or test SDK is required.

## Historical downloads

The earlier staging artifact references are:
[Dockpipe Language Support 0.0.2](https://packages.staging.dockpipe.com/packages/standalone/dorkpipe/candidates/0.6.5-staging.cc31cf0aa848/dockpipe-language-support-0.0.2.vsix)
and the [companion extension 0.0.7](https://packages.staging.dockpipe.com/packages/standalone/dorkpipe/candidates/0.6.5-staging.cc31cf0aa848/pipeon-0.0.7.vsix).
Verify those exact downloads against their
[SHA256SUMS](https://packages.staging.dockpipe.com/packages/standalone/dorkpipe/candidates/0.6.5-staging.cc31cf0aa848/SHA256SUMS)
before using **Install from VSIX**. They are historical delivered artifacts;
restoring source ownership does not replace their bytes or publish newer versions.
Pipeon artifact dependency upgrades remain owned by its package and locked manifest.

When workflow/package contracts change, update the runtime parser/schema,
canonical documentation and Dockpipe extension together. PipeLang language
contracts and compiler integration are maintained in PipeLang.

## Project configuration schema

`dockpipe.config.json` has a separate authoritative
[JSON schema](../../src/lib/infrastructure/schema/dockpipe.config.schema.json).
Associate that file through your editor's JSON schema settings, or set `$schema`
to its local path. In a Dockpipe source checkout:

```json
{
  "$schema": "./src/lib/infrastructure/schema/dockpipe.config.schema.json",
  "schema": 1,
  "dockpipe_version": "0.6.24"
}
```

For external repositories, use the schema from the installed/pinned Dockpipe
release or a published immutable schema URL for that release. The schema source
is authoritative; adding this file does not publish a new hosted schema endpoint
or install an editor extension. `dockpipe_version` requires an exact complete
SemVer identity, including build metadata. Consumer repositories should use
`min_dockpipe_version` for a complete SemVer lower bound instead. Both fields are
available in project completion and hover; the CLI checks their consistency and
enforces them before executing repository commands. A JSON schema cannot compare
two arbitrary semantic versions, so contradictory declarations are rejected by
the runtime. See [consumer compatibility migration](consumer-cli-compatibility.md).
Package minimum-version metadata remains independent.


Package authors may associate `assets/ui/surface.json` with
`src/lib/infrastructure/schema/package-surface.schema.json` from a pinned Dockpipe
source/schema distribution. The package manifest's optional `surface` field names
that descriptor. This is separate from workflow `view` metadata: surface actions
still reference ordinary workflow files or public workflow IDs, and typed public
workflow inputs continue through the existing GUI dialog. See
[package-contributed host pages](package-model.md#package-contributed-host-pages).

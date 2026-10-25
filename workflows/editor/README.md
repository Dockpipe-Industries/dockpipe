# Editor workflows

Run `editor.build` through the repository's pinned Dockpipe runner. It installs
checksum-locked npm dependencies in an isolated generated scope, runs the editor's
Node tests and packages the owning extension with locked vsce 3.7.1. Node 22+ is
required. Source checkout files and unrelated editors are never packaged together.

The output is the versioned VSIX plus `extension.json`, which binds identity,
version, byte count, SHA-256 and the qualified source inventory. VSIX ZIP timestamps
may vary between builds; no byte-reproducibility claim is made. Review these exact
produced bytes rather than rebuilding at the publication boundary.

The GitHub Action builds candidate artifacts on source changes. To publish, select
a successful candidate run and its exact version/SHA-256 in manual dispatch. The
manual job downloads that candidate, verifies it against the current editor source
and invokes `vsce publish --packagePath` on the same file. Configure the existing
publisher's Marketplace token as the `VSCE_PAT` secret in the `vscode-marketplace`
GitHub environment. Publication is not part of ordinary builds or pull requests.
Create/verify the publisher account separately before the first upload; this task
only implements the action and does not create an account or publish a version.
Publish the PipeLang extension before Dockpipe, which declares it as an extension
dependency. For local testing, install the reviewed PipeLang VSIX first.

Official contracts checked during implementation:
- https://code.visualstudio.com/api/working-with-extensions/publishing-extension
- https://code.visualstudio.com/api/working-with-extensions/continuous-integration
- https://github.com/microsoft/vscode-vsce/blob/main/src/publish.ts

Run publication or installation explicitly with `--artifact-dir`, `--version` and
`--sha256`. Wrong identity/version, changed source, missing qualification, or a
checksum mismatch fails before invoking Marketplace or the local editor.

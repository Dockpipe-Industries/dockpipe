# Dockpipe editor ownership restoration

Applied and qualified locally on 2026-10-25; uncommitted and unpublished.

- Dockpipe owns `tooling/vscode-extensions/dockpipe-language-support` version
  0.0.3, its current engine schema copies, host SDK completion and typed-model
  metadata adapter. PipeLang owns the separate `.pipe` grammar, snippets and
  standalone compiler diagnostics in its own editor extension version 0.0.1.
- Selected working DorkPipe source was admitted at
  `b1f93d50be9974b03d49285fec3ae158f7d8c2fb`; provenance manifests record exact
  per-file working-byte hashes and distinguish dirty postimages from HEAD.
  All twelve inherited DorkPipe changes and Pipeon source remain untouched.
- Both owners have Dockpipe `editor.build` and `editor.publish` workflows with
  colocated scripts/tests and thin Marketplace GitHub Actions. Runtime Make
  editor targets call Dockpipe workflows; the sibling source fallback is gone.
- Actual owner builds passed with checksum-locked public runners: runtime
  0.6.29-staging.cli.sha324a2f2fd49e and PipeLang's unchanged
  0.6.26-staging.cli.sha49987891f4c3 pin. Node authoring/language tests, bounded
  compiler request tests and artifact identity/checksum/source rejection tests
  passed. All five workflow configurations validated.
- Qualified local artifacts: Dockpipe 0.0.3, 54,534 bytes,
  SHA-256 `9d9670a015bc3d1bad980b0a8cabb7a40d8669e741ec87b0046500d6b73c3dca`;
  PipeLang 0.0.1, 37,334 bytes,
  SHA-256 `94a57f73ff0b407fdf151f182d92e5e3450b4e759721dbe416606a20d3e9aaaf`.
  Receipts bind the exact VSIX and source inventory; ZIP byte reproducibility
  is not claimed.
- Manual publication requires reviewed candidate run/version/SHA and the
  `VSCE_PAT` environment secret reference; it publishes the same verified VSIX.
  Publish PipeLang first because Dockpipe declares that extension dependency.

No Marketplace upload, editor installation or native extension-host qualification
has run. SDK helper-path and Go completion require the matching next runtime/SDK
release. Canonical usage and publication guidance: `docs/packages/editor-extensions.md`.

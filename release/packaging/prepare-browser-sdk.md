# Exact Browser SDK preparation

`prepare-browser-sdk.py` consumes the committed
`release/dependencies/spatialweb-browser.lock.json`. It downloads public immutable
release assets and verifies their publisher attestations; it never chooses a latest
release or resolves a branch. Qt remains separate and must match **6.10.3**.

```bash
python3 release/packaging/prepare-browser-sdk.py \
  --lock release/dependencies/spatialweb-browser.lock.json \
  --platform linux-amd64 --output /absolute/new/browser-sdk \
  --receipt /absolute/new/browser-sdk-receipt.json
```

The output prefix and optional receipt file must be fresh. `--local-assets DIR`
uses already downloaded assets while retaining checksum and public attestation
verification for all three files. `gh attestation verify` requires access to public
GitHub attestations. The helper does not use private repository credentials.

The lock's top-level fields are `schema: 1`,
`product: "SpatialWebBrowserSDK"`, `version: "0.1.0"`, `qt_version: "6.10.3"`,
`source_repository: "Dockpipe-Industries/Spatial-Web"`, the exact 40-character
`source_sha`, `release`, and `platforms`. `release` contains only the source
repository and tag `browser-sdk-0.1.0-staging.<source_sha>`.

`platforms` must contain exactly `linux-amd64`, `linux-arm64`, `darwin-amd64`,
`darwin-arm64`, and `windows-amd64`. Each entry contains:

- `artifact`, `metadata`, and `file_manifest`: exact `{name, size_bytes, sha256}`
  pins for `SpatialWebBrowserSDK-0.1.0-<platform>.tar.gz`, `.release.json`, and
  `.files.json` respectively.
- `compiler`: exact producer `{id, version}`. Linux uses GNU, macOS AppleClang,
  and Windows MSVC. The lock preserves the full producer compiler version.
- Windows alone also requires `msvc_runtime: "dynamic-release-/MD"`.

No example release digests are provided: populate this lock only from the
published candidate's verified assets and compiler metadata.

The helper validates metadata, inventory, every archive file, required public
header/CMake package, and rejects Qt/Projection/spatial runtime bundling. Archive
extraction rejects links, special files, duplicate and unsafe portable paths, and
excessive archive, decompressed, metadata, member, or installed sizes. Installation
uses a temporary sibling directory and publishes the fresh prefix only after all
checks pass. Publisher attestations must bind the exact source SHA, dev ref, and
`Spatial-Web/.github/workflows/browser-sdk.yml`.

The prefix retains the original platform metadata and inventory, unchanged
`consumer-lock.json`, and `consumer-receipt.json`. The same receipt is printed as
JSON and optionally written to `--receipt`. It contains `selected_platform`,
`installed_prefix`, the unchanged full `lock_sha256`, all three selected exact
`pins`, and source/Qt/compiler/release `identity` (plus Windows runtime). Downstream
packaging must compare these values against the selected committed lock and use
the returned prefix; the receipt is evidence, not a replacement for verification.

Network-free boundary tests (only attestation verification is mocked):

```bash
python3 -B -m unittest discover -s release/packaging/tests \
  -p test_prepare_browser_sdk.py -v
```

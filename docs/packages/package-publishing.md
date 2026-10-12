# Publish a self-hosted package store

This reference is for package publishers. To install and use existing packages,
see [Find and use packages](package-quickstart.md). Official Dockpipe release
operations are documented separately in [Releasing](../../release/docs/releasing.md).

## Export your project's packages

Author package metadata, dependencies and assets in your project before compiling.
For the hello workflow from [onboarding](../onboarding.md):

```sh
dockpipe package compile workflow workflows/hello --workdir .
dockpipe package build store --workdir .
```

The export writes package archives, checksum sidecars and
`packages-store-manifest.json` under `release/artifacts/` by default. Use `--out`
to choose an output directory. This exports local artifacts; it does not publish
them or install them for other users.

Inspect the manifest and test packages on the intended execution platform before
publishing. Native helper binaries and Flatpak runtime dependencies must match
that target. A checksum proves integrity against your catalog, not independent
publisher trust.

## Host the store

Serve the manifest, archives and checksum files from a trusted HTTPS origin.
Keep immutable versions available for consumers who pin them. Remote catalogs
restrict referenced downloads to the selected origin.

For an S3-compatible destination, `dockpipe release upload` wraps a configured
provider CLI; inspect its options in the [CLI reference](../cli-reference.md#dockpipe-release).
Credentials and upload permission are separate from building the archives.

Consumers can use your HTTPS store manifest with `dockpipe package catalog` and
`dockpipe package install`. See [Remote catalogs and user installation](package-model.md#remote-catalogs-and-user-installation)
for the manifest and verification contract. Dependencies are selected separately.

## Legacy core-tree mirrors

`dockpipe package build core` exports a core-tree tarball and `install-manifest.json`
for the older `dockpipe install core` mirror flow. This is separate from normal
application installation and Marketplace package installation. The archive's
entries use a `core/` prefix; use the export command rather than manually assembling
an archive with a different layout.

Maintainers building official core payloads from the Dockpipe checkout can use
`make package-templates-core`. That Make target and Dockpipe's own release workflows
are contributor tooling, not requirements for installing third-party packages.

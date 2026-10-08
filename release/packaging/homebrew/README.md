# Homebrew packaging

This directory contains the staging tap source and the older stable formula stub.

## Staging tap

`tap/` is the maintained source for the active public
[`Dockpipe-Industries/homebrew-dockpipe`](https://github.com/Dockpipe-Industries/homebrew-dockpipe)
repository, with default branch `main`. Its initial native validation and formula
publication passed in [run 37397290496](https://github.com/Dockpipe-Industries/homebrew-dockpipe/actions/runs/37397290496).
Copy reviewed authored updates, including `.github/workflows/update-staging.yml`,
into that repository. Do not copy caches, local prepared output, or the stable stub.
No new secret is required: the workflow
reads public R2 release metadata and uses the tap's own `GITHUB_TOKEN` for the generated
formula and cask updates. Grant `contents: write` only to the publication job.

The scheduled workflow checks every 15 minutes and supports manual dispatch. It
requires the exact source CI run/attempt to have succeeded before generating the
formula and cask. macOS Intel and Apple Silicon runners each install the formula and desktop cask and test their CLI/runtime
before publication. The publisher rechecks the latest pointer and uses the existing
blob SHAs to reject concurrent overwrites. Both versions are checked before writing; the CLI is published first, and each update is idempotent if the pair is interrupted. The desktop cask requires a completed candidate containing both macOS desktop ZIPs. Publish that candidate before deploying these sync changes; older CLI-only candidates cannot seed the desktop cask.

Review a formula locally without any remote write:

```sh
python3 release/packaging/homebrew/tap/scripts/sync_staging.py prepare \
  --output /tmp/dockpipe-homebrew-prepared
```

For local unit checks:

```sh
python3 -m unittest discover -s release/packaging/tests -p test_homebrew.py -v
```

The formula installs the native CLI plus only the required core archive in its
Homebrew keg. Its small launcher defaults `DOCKPIPE_SYSTEM_ROOT` to that package
directory, respecting an explicit user override. No engine changes or writes to
`/Library/Application Support` are needed. The stable `dockpipe` formula is not
published, so staging must not declare a conflict with that unavailable formula.
Add reciprocal conflicts when the stable formula is published: both install the
`dockpipe` command. Staging is not a separate product identity. Its candidate version is
upgradeable and the binary reports the generated numeric version.

The tap is macOS-only initially. APT and portable archives remain the Linux paths.
Native Homebrew installation must pass on hosted Macs before the formula is made
available. A locally generated Ruby file is not installation proof.

## Desktop cask

`dockpipe-desktop-staging` installs `DockPipe.app` from the candidate's native desktop ZIP and depends on `dockpipe-staging` for the terminal command. The app carries its matching runtime and required core package, so Finder launches do not depend on a login shell's PATH. Cask removal retains the CLI and user data. The direct DMG uses Apple's Installer instead and must not be mixed with the cask.

The desktop cask and updated CLI formula are published for candidate
`0.6.0-staging.37414177001.1.48a5701e3652` after
[run 37419531103](https://github.com/Dockpipe-Industries/homebrew-dockpipe/actions/runs/37419531103)
passed install, CLI/launcher checks, uninstall, and version ordering on both Mac architectures.
Published definitions were read back and matched the tested bytes. Desktop source and the remaining
Apple signing/notarization boundary are documented in [desktop/README.md](../desktop/README.md).

## Stable formula follow-up

## Files

- `dockpipe.rb` — Unpublished source-build stub; its checksum is still a placeholder.

## Maintainer flow (new release)

1. Compute source tarball SHA for the release tag:

   ```bash
   VERSION=0.6.0
   curl -L "https://github.com/Dockpipe-Industries/dockpipe/archive/refs/tags/v${VERSION}.tar.gz" -o "/tmp/dockpipe-${VERSION}.tar.gz"
   shasum -a 256 "/tmp/dockpipe-${VERSION}.tar.gz"
   ```

2. Update formula:
   - `url` -> `.../v<version>.tar.gz`
   - `sha256` -> computed hash

3. Commit formula update in tap repo (recommended: `homebrew-dockpipe`):
   - path: `Formula/dockpipe.rb`

4. Verify install:

   ```bash
   brew tap Dockpipe-Industries/dockpipe
   brew install dockpipe
   dockpipe --help
   ```

## Notes

- Formula builds from source using Go and installs the **`dockpipe`** binary only. Bundled templates, scripts, and images are **embedded** and materialize to the user cache on first run (same as `.deb` and Windows zip).
- Optional: set **`DOCKPIPE_REPO_ROOT`** to a git checkout of dockpipe when developing templates.

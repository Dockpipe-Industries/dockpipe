# Homebrew packaging

This directory contains the staging tap source and the older stable formula stub.

## Staging tap

`tap/` is the complete seed for the public `Dockpipe-Industries/homebrew-dockpipe`
repository, with default branch `main`. Copy its authored files, including
`.github/workflows/update-staging.yml`, into that repository. Do not copy caches,
local prepared output, or the stable stub. No new secret is required: the workflow
reads public R2 release metadata and uses the tap's own `GITHUB_TOKEN` for its one
formula commit. Grant `contents: write` only to the publication job.

The scheduled workflow checks every 15 minutes and supports manual dispatch. It
requires the exact source CI run/attempt to have succeeded before generating a
formula. macOS Intel and Apple Silicon runners each install and test the candidate
before publication. The publisher rechecks the latest pointer and uses the existing
formula blob SHA to reject concurrent overwrites. No new DockPipe release is needed
to seed the tap from an already completed candidate.

Review a formula locally without any remote write:

```sh
python3 release/packaging/homebrew/tap/scripts/sync_staging.py prepare \
  --output /tmp/dockpipe-homebrew-prepared
```

For local unit checks:

```sh
python3 -m unittest discover -s release/packaging/tests -p test_homebrew.py -v
```

The formula installs the native CLI plus all core/workflow/resolver archives in its
Homebrew keg. Its small launcher defaults `DOCKPIPE_SYSTEM_ROOT` to that package
directory, respecting an explicit user override. No engine changes or writes to
`/Library/Application Support` are needed. The formula conflicts with stable
`dockpipe`; staging is not a separate product identity. Its candidate version is
upgradeable even while the binary reports the unchanged numeric core version.

The tap is macOS-only initially. APT and portable archives remain the Linux paths.
Native Homebrew installation must pass on hosted Macs before the formula is made
available. A locally generated Ruby file is not installation proof.

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

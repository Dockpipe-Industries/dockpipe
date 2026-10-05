# Release packaging

| Script / file | Output |
|---------------|--------|
| **[package-templates-core.sh](package-templates-core.sh)** | **`templates-core-<VERSION>.tar.gz`** + **`.sha256`** + **`install-manifest.json`** → **`release/artifacts/`** (same layout as **`dockpipe package build core`**; **`make package-templates-core`**) |
| **[build-core-package.sh](build-core-package.sh)** | **`dockpipe-core-<VERSION>.tar.gz`** + **packages-store-manifest.json** → compiled core package used by MSI / Linux installers |
| **[build-deb.sh](build-deb.sh)** | Debian **`.deb`** (amd64, arm64) → `release/packaging/build/` |
| **[build-nfpm.sh](build-nfpm.sh)** | **Alpine `.apk`**, **RPM `.rpm`**, **Arch Linux `.pkg.tar.zst`** (amd64, arm64) → `release/artifacts/` in CI |
| **[nfpm.yaml.in](nfpm.yaml.in)** | Template for [nfpm](https://github.com/goreleaser/nfpm) (substituted by `build-nfpm.sh`) |
| **[linux/install.sh](linux/install.sh)** | Optional one-liner installer (detects distro) |
| **[windows/install.ps1](windows/install.ps1)** | Windows zip/MSI + optional WSL setup |
| **[wsl/README.md](wsl/README.md)** | WSL bridge notes |

Local test (requires Go):

```bash
./release/packaging/build-nfpm.sh "$(tr -d '\n' < VERSION)" /tmp/dockpipe-nfpm-test
ls /tmp/dockpipe-nfpm-test/
```


## Exact bundled inputs

The root `embed_assets.go` declares individual authored files, so ignored build
output beside an asset cannot enter an ordinary Go build. `embed.go` preserves
file reads, directory enumeration and open-directory traversal through the same
bundle paths; engine consumers remain generic. Nested Go modules and hidden or
underscore-prefixed entries retain the former Go directory-walk exclusion.

When adding/removing authored assets, run `python3 release/packaging/embedded-inputs.py`
and review the manifest diff. `--check` refuses stale membership. Discovery includes
tracked and nonignored untracked authored files, never ignored caches/binaries.
The manifest is checked-in source metadata, not a resolved/generated payload.
`go test .` independently checks every formerly visible authored asset byte and
validates the filesystem interface. Release preparation checks membership too.

`prepare-embedded-dorkpipe-assets.sh prepare` still produces the three intentional
Linux tools (`dockpipe`, `dorkpipe`, `mcpd`). It now creates one ignored, exact
`embed_release_generated.go` declaration for those paths only. The root filesystem
merges their directory entries without copying their payloads into a new heap map.
Cleanup validates that declaration before removing its own generated file through
the existing cleanup hook. It does not modify `embed_assets.go`. No ignored Tauri,
Cargo, npm or incidental neighboring binary becomes a release input.

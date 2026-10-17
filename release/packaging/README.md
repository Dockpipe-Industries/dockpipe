# Release packaging

| Script / file | Output |
|---------------|--------|
| **[package-templates-core.sh](package-templates-core.sh)** | **`templates-core-<VERSION>.tar.gz`** + **`.sha256`** + **`install-manifest.json`** → **`release/artifacts/`** (same layout as **`dockpipe package build core`**; **`make package-templates-core`**) |
| **[build-core-package.sh](build-core-package.sh)** | **`dockpipe-core-<VERSION>.tar.gz`** + **packages-store-manifest.json** → compiled core package used by MSI / Linux installers |
| **[desktop/README.md](desktop/README.md)** | Desktop DMG/ZIP, Qt launcher DEB and Windows MSI runtime; CLI-only packages remain available |
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

DorkPipe, DorkPipe MCP, and Pipeon are independently published packages and are
excluded from the main binary's embed declaration. Native releases compile only
core, then assemble a complete store from checksum-pinned public package bundles.
Flatpak adapts and qualifies the pinned Linux amd64 store inside its runtime.
The main build does not run the former embedded DorkPipe preparation hook.

The repository-quality workflow stays in the main repository as a thin caller.
Native packaging compiles that wrapper into a separate main-owned store and
replaces its older baseline archive by exact package identity. Its source lock
binds each supported platform to an immutable catalog, package kind and version,
archive checksum and size, and the checksum of DorkPipe's owned summary script.
The wrapper verifies those bytes, caches the archive in its disposable package
runtime, and executes only that script. Other installed versions cannot replace
its selection. The package-owned documentation orchestration workflows instead
ship with DorkPipe. These explicit first-party pins do not implement the proposed
general project/package lock or package-manager version-selection model.
The five frozen 0.6.16 platform baselines retain their published GitHub release
URLs. New standalone Linux amd64 candidates are fetched from the package host:
`https://packages.staging.dockpipe.com/packages/standalone/<owner>/candidates/<version>-staging.<producer-sha12>/<owner>-packages-<version>-linux-amd64.tar.gz`.
Stable releases use
`https://packages.dockpipe.com/packages/standalone/<owner>/releases/<version>/<owner>-packages-<version>-linux-amd64.tar.gz`.
The lock records exact archive size, archive SHA-256, and nested store manifest
SHA-256. Candidate downloads reject redirects; paths using credentials, queries,
mutable selectors, or another owner/version cannot satisfy the lock. Public
package delivery does not change the visibility of the owning source repositories.
The trusted `pinned-stores.lock.json` also lists package identities hidden from
catalog discovery. The six Pipeon resolver and workflow entries remain available
for explicit installation and dependency resolution. This applies to the new
Linux amd64 Pipeon archive and to the retained 0.6.3 archives on the other four
native platforms. Hiding changes composed catalog metadata only; the pinned
archive bytes, versions, and source manifest checksums stay unchanged.

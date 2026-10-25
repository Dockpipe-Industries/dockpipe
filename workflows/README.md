# Repository workflows

Dockpipe owns this repository's build, test, package and publication orchestration. Make and GitHub Actions provide thin entrypoints. Each workflow keeps its scripts, fixtures, pins and tests beside the operation that owns them.

Run an operation with the separately pinned released bootstrap:

```bash
python3 workflows/shared/launch.py workflows/build/cli/config.yml -- --output /tmp/dockpipe --version dev
python3 workflows/shared/launch.py workflows/ci/core/config.yml
python3 workflows/shared/launch.py workflows/ci/workflow-contracts/config.yml
```

`workflows/shared/bootstrap.lock.json` pins the producer tool independently of the runtime being built. `DOCKPIPE_BOOTSTRAP_ARCHIVE` can select an already downloaded archive with that exact checksum. The produced candidate never substitutes for its own bootstrap.

| Owner | Responsibility |
| --- | --- |
| `build/cli`, `build/cli-candidate` | Developer CLI and qualified immutable CLI candidates |
| `build/go-sdk`, `build/display-sdk`, `build/launcher-sdk` | Independent SDK producers and their source contracts |
| `build/desktop`, `build/desktop-development`, `build/flatpak` | Native desktop, developer builds and Flatpak qualification |
| `build/platform` | Explicit release identity, CLI build, archive assembly and native Linux packaging |
| `package/linux-cli`, `package/macos`, `package/windows-msi` | Installers consuming produced binaries and explicit optional stores |
| `dependencies/browser-sdk`, `qualify/cockpit` | Pinned external ABI inputs and separate optional package qualification |
| `catalog/compose` | Explicit pinned catalog composition; never a default CLI dependency |
| `ci` | Runtime, shell, native and workflow contract checks |
| `publish` | Artifact-specific validation and immutable publication |
| `release/metadata` | Version allocation, release catalog and public-content checks |
| `install`, `setup`, `demo` | Explicit local operations |
| `editor.build`, `editor.install`, `editor.publish` | Repository-owned Dockpipe VSCode extension lifecycle |

A standalone CLI contains no catalog packages. A caller may explicitly supply a validated optional store. Historical manifests may retain a legacy core archive for readback; builds do not compile or require one. Optional catalog qualification and publication are separate operations.

Shared helpers contain generic archive, bootstrap and launch behavior. Artifact-specific policy stays with its owning producer or publisher. Public release notes and articles remain in `release/`.

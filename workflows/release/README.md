# Release workflow ownership

Release orchestration lives under `workflows/`. Scripts, tests and pins are colocated with the operation that owns them; [the workflow index](../README.md) lists those boundaries.

The release adapter calls `build/platform/config.yml` for the standalone CLI and archives, `build/desktop` for native desktop producers, and the platform-specific `package` workflows for installers. A catalog store is an explicit optional input. Default CLI, MSI, DEB, RPM, APK, Arch and Flatpak products do not compile or install a core package.

Version allocation, release-notes headers and final artifact validation belong to `release/metadata`. SDK and CLI candidate publication retain their own immutable artifact validators. Shared GitHub release transaction behavior lives under `publish/shared`; APT, R2, Homebrew and release articles have separate owners.

The released bootstrap is pinned by `shared/bootstrap.lock.json`, independently of the produced CLI. Make and GitHub Actions launch these workflows and retain event, platform and secret-reference configuration. Defining a publisher does not upload an artifact; publication remains gated by its concrete release workflow.

Release notes are generated into `release/artifacts/RELEASE_NOTES.md`. The publisher
appends GitHub's merged-PR changelog using the selected previous tag; read-only dry
runs retain only the generated header. Reviewed announcement content remains in
`release/articles/announcement.md`, independently of the generated release version.

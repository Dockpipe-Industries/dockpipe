# Repository guidance and DockPipe execution

Extraction repositories use root `AGENTS.md`, `dockpipe.config.json` and
`workflows/<name>/config.yml`. Initialization uses `dockpipe init`, retaining
existing documentation, configuration and unrelated Git state. Implementation
starts on `js/dev`; GitHub qualification precedes promotion to `dev` and `staging`.
A staging push publishes to the staging package source; a master push publishes
to the production package source after qualification. Production promotion remains
a separate gate from the current staging delivery.

| Repository | Root workflows | Current boundary |
| --- | --- | --- |
| ForgePipe | `check`, `build`, `test`, `sdk.candidate` | Installed native engine SDK; extraction remains partial |
| DorkPipe | `check`, `build`, `test`, `test.integration`, `editor.candidate`, `sdk.candidate`, `package.candidate`, `package.compile`, `package.delivery` | Agentic package, MCP and both VS Code extensions |
| Pipeon | `check`, `test`, `package.candidate`, `package.compile`, `package.delivery` | Paused package; six resolver/workflow discovery entries hidden |
| Nucleon | `check`, `build`, `test`, `sdk.candidate` | Independent native compression SDK; codec/research safeguards retained |
| MesaMoto | `check` | Local scaffold; game extraction and standalone application pending |
| SpatialPipe | `check` | Local scaffold; offline library/application extraction pending |
| Spatial-Web | `check` | Local scaffold; public library/browser extraction pending |

MesaMoto, SpatialPipe and Spatial-Web have existing empty remote repositories and
local scaffolds without a Git checkout connection. A scaffold check does not
qualify an application or publish its source.

## Models and ownership

Default execution is `gpt-6.1-sol` with Medium reasoning. Delegate bounded independent
work with explicit ownership when it reduces total cost, including context transfer,
coordination, review and retries. A stronger direct worker can be cheaper overall;
escalate difficult seams rather than routinely using Astra High or broad fanout.
These are agent instructions, not an automatic runtime model-selection feature.

ForgePipe is the native game engine. DorkPipe owns unbuilt AI feature contributions
inside the existing DockPipe application, CLI and MCP; it is not a separate AI app
or package. Native libraries remain independent of the DockPipe executable at
runtime; DockPipe orchestrates development and delivery. Generated output uses
artifact scopes, with product-specific build and test behavior inside its repository.

## Compiler bootstrap and execution

Each root commits `release/toolchains.lock.json` with the CLI version, immutable
GitHub release archive and SHA-256. DorkPipe and Pipeon select DockPipe 0.6.17 from
`v0.6.17-staging.38994929768.1.53e295fff81d`; its Linux amd64 archive SHA-256 is
`88f1e52cfa38e3dfe2293fd33ae4290792f87dc6edbce414cc89bdad7ae8a978`.
The other five initialized roots retain their independently qualified 0.6.11 locks.

Bootstrap bounds HTTPS downloads, verifies the digest before extracting the single
executable and rejects environment overrides that differ from the committed lock.
CI invokes `--workflow-file workflows/<name>/config.yml` from the repository root.
Product scripts, tests and package compilation run through those workflows. Updates
change the lock deliberately and qualify the consumer before promotion. Action
references use verified commit SHAs with readable version comments.

The 0.6.17 compiler supports explicitly typed asset-only resolver packages,
checksum-verified installation of declared transitive dependencies from a selected
catalog, and hidden-package metadata. General project and package version locks
remain a separate implementation objective. Both resolver and
workflow identities must carry the intended metadata; hiding only one identity does
not hide the other. Hidden discovery does not revoke explicit installation or access.

## Go compatibility SDK

`release/packaging/build-go-sdk.py` exports the public runtime dependency closure
used by DorkPipe/MCP. It discovers Linux, Windows and macOS source variants and
records every file and archive digest. Private implementations and optional native
Cockpit source are excluded. Source discovery is not native platform qualification.

DorkPipe selects the committed compatibility SDK from runtime source
`c604b291d463906acf0a05ef8cfe2efc071bdbaa`, version
`0.1.1-source.c8e6e7102c16a873`. Its archive SHA-256 is
`235945df5ba24b435e6f203f31494fc96b11357033f72396d091a1c80190bfdf`
and manifest SHA-256 is
`2e2d2fbb53a2261ae5bda4ef122505b1bc875dab6190ab73c1c514326296a06e`.
These identities are committed in public DorkPipe `release/dependencies.lock.json`
and delivered by its [public SDK release](https://github.com/Dockpipe-Industries/dorkpipe/releases/tag/dockpipe-go-sdk-0.1.1-source.c8e6e7102c16a873).
The 0.1.1 manifest refresh retains the previously verified archive bytes.

The importer verifies the independently trusted manifest digest, archive, complete
member inventory and individual file digests. Traversal, links, duplicate or private
paths, missing files and content mismatches fail before publication. Bounds are
4 MiB manifest, 64 MiB compressed archive, 256 MiB source and 10,000 files, plus
expanded tar-header capacity. Existing outputs are never replaced.

Builds use temporary module files and a pinned local Go toolchain. CI fetches the
actual fixture workspace's module graph and selected archives before offline shell
execution. This closes historical module-metadata gaps without changing immutable
SDK module files. Four helper binaries and DorkPipe/MCP tests work without a sibling
DockPipe source checkout. The source compatibility SDK does not promise a stable
native ABI or protect Go implementation details.

## Qualified deliveries

- ForgePipe [0.1.0 staging](https://github.com/Dockpipe-Industries/forgepipe/releases/tag/v0.1.0-staging.bcafe39cda1b)
  passed native engine SDK qualification and downloaded checksum readback. Native
  redistribution still requires approved terms.
- Nucleon source `1d04f8d7e371303cade091381e73d6d62ec78878` passed native
  correctness, fourteen SDK lanes, eleven package consumers and four debug bindings.
  The [private candidate release](https://github.com/Dockpipe-Industries/nucleon/releases/tag/sdk-candidate-0.3.0-source.1d04f8d7e371-run.38995832666.1)
  durably retains the tested packages and receipts, linked to the
  [matching private debug release](https://github.com/Dockpipe-Industries/nucleon/releases/tag/sdk-debug-0.3.0-source.1d04f8d7e371-run.38995832666.1).
  Both passed downloaded checksums and provenance verification. Public registry
  and legal distribution approvals remain false; PipeLang availability is disabled.
- Public DorkPipe [0.6.5 staging](https://github.com/Dockpipe-Industries/dorkpipe/releases/tag/v0.6.5-staging.cc31cf0aa848)
  passed qualification and publication at exact source
  `cc31cf0aa8481479df4d54142003ca0b6546d677`: SDK build, all 21 shell tests,
  both editors and all 26 packages (eight resolvers and eighteen workflows).
  The package owns both documentation orchestration workflows. Six downloaded
  assets match source-bound provenance and checksums, including language extension
  0.0.2, companion extension 0.0.7 and the curated editor test SDK.
  The Linux amd64 package bundle SHA-256 is
  `e00a93057b3e1afedfd13d01c951f16c261ddbd4ea677584ff34b347d5140955`.
  All 59 objects in its [staging package feed](https://packages.staging.dockpipe.com/packages/standalone/dorkpipe/candidates/0.6.5-staging.cc31cf0aa848/packages/packages-store-manifest.json)
  passed independent size and checksum readback. Public reusable source and private
  platform ownership are separate; `dorkpipe-platform` retains private history and
  platform-specific integration code.
- Pipeon [0.6.6 staging](https://github.com/Dockpipe-Industries/pipeon/releases/tag/v0.6.6-staging.aa57b250fb8a)
  passed qualification and publication at source
  `aa57b250fb8a0444087ef14c721ea3a0067c33b4`. All three downloaded assets
  match source-bound provenance and checksums. The six hidden owned packages use
  the exact DorkPipe 0.6.5 resolver and editor inputs. Its package bundle SHA-256 is
  `f9310d5dc46081c220bb10e76348c144853bf3ce095da364a9d00ea7fff2fe5e`.
  All six identities remain hidden while the product is paused.
  Its [staging package feed](https://packages.staging.dockpipe.com/packages/standalone/pipeon/candidates/0.6.6-staging.aa57b250fb8a/packages/packages-store-manifest.json)
  includes the byte-identical DorkPipe 0.6.5 resolver dependency. All eighteen feed
  objects passed independent size and checksum readback. This is a maintenance
  dependency update; Pipeon feature work and retirement remain paused.

## Consumer pins and shared store

Controlled consumers select immutable versions, archive SHA-256, exact byte sizes
and manifest digests. Freeze the actual qualified archive before recording a lock;
recompilation can change headers and digests. Upgrade version and digest together,
then verify installation and use before promotion.

The standalone repositories publish their audited, frozen package deliveries to
the package host. Source repository visibility and package distribution are
independent decisions. DorkPipe now has public source and release evidence; Pipeon
retains private source and release evidence. The package host remains the composed
store distribution boundary. Private native ForgePipe and Nucleon SDKs
are not part of this package-host distribution. `release/packaging/pinned-stores.lock.json` selects the five
qualified native baseline stores and the new Linux amd64 standalone bundles.
Other native platforms retain their previous qualified package versions until their
own standalone builds pass. New Linux package proof does not qualify those targets.

`compose-package-store.py` replaces exactly the identities in separately trusted
ownership declarations. Candidate metadata cannot grant ownership or replace engine
core. Every retained and incoming archive must match its digest and declared size.
Links, missing files, corrupt bytes, immutable-name collisions, exhausted capacity
and existing outputs fail without publishing a partial store. Hidden flags and
unrelated baseline packages remain intact.

Standalone publishers do not update the full engine/APT `latest.json` pointer. Main
DockPipe staging publication qualifies and publishes the composed store separately.
The staging and production publisher environments are configured with staging-only
and master-only branch policies respectively; no production packages were published.
An exact frozen release source pin still needs to be committed and qualified for a
future master promotion. Environment configuration alone is not release qualification.
The frozen DorkPipe 0.6.5/Pipeon 0.6.6 inputs passed local Linux store composition:
69 optional packages plus core, with all seventy archive digests verified. All 69
KDE 6.10 Flatpak dependency/preflight profiles and final store export also pass
with the published 0.6.17 CLI. This qualifies these package bytes locally; it does
not establish live AI/provider behavior or main engine hosted release publication.
The guarded source-retirement checkpoint and main staging release remain pending.
Standalone publishers write only their immutable product paths; the main DockPipe
publisher controls shared catalog and APT pointers.

# TASK-044 — Standalone PipeLang and external packages

Status: **completed through staging**, closed 2026-10-26 by user direction.
The original extraction acceptance criteria are satisfied. Production promotion
and broader optional-feature qualification are separate work, not migration blockers.

## Result and ownership

- PipeLang compiler, language SDKs, tests, docs and producer workflows live in
  [PipeLang](https://github.com/Dockpipe-Industries/pipelang).
- Remaining package source, assets, tests and producers live in
  [dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages).
  Previously extracted products remain with their existing owners.
- [DorkPipe](https://github.com/Dockpipe-Industries/dorkpipe) owns the reusable
  harness/MCP package and its guides; private platform history was not exported.
- Dockpipe retains the generic runtime, CLI, desktop host and host SDK. The old
  package source roots and bundled core tree are retired. Builds consume the
  versioned PipeLang module and declared external artifacts, without sibling
  source fallbacks. No package-specific implementation was copied into the engine.
- Documentation ownership and links were audited; owner-held documentation was
  promoted with the source cleanup. Historical checkpoints are evidence only.

## Acceptance and terminal evidence

| Acceptance | Completed evidence |
| --- | --- |
| Independent language build and SDK consumption | Public PipeLang module `v0.1.0` fetched with fresh public-only caches; standalone compiler `v0.1.1-staging.1` passed all five native release lanes; all 12 published assets and attestations verified. |
| Independent package source and dependencies | Catalog source build, tooling, producer, contract and VM package tests passed. Published Go SDK inputs were reproduced and hash-verified; native immutable catalog candidate verified on its declared Linux amd64, Windows amd64 and macOS arm64 profiles. |
| Generic runtime and external package boundary | Final core/native/Flatpak qualification passed; core-only stores remain separate from the optional catalog. Installed Linux CLI workflow smoke passed outside the checkout. |
| Owning docs and package publication | Pipeon PRs [10](https://github.com/Dockpipe-Industries/pipeon/pull/10) / [11](https://github.com/Dockpipe-Industries/pipeon/pull/11), DorkPipe PRs [16](https://github.com/Dockpipe-Industries/dorkpipe/pull/16) / [17](https://github.com/Dockpipe-Industries/dorkpipe/pull/17), and catalog staging promotion completed. DorkPipe staging publication verified all 26 package objects against manifest sizes and hashes. |
| Runtime cleanup promoted through staging | Dockpipe PRs [222](https://github.com/Dockpipe-Industries/dockpipe/pull/222) / [223](https://github.com/Dockpipe-Industries/dockpipe/pull/223); [release run 40247668385](https://github.com/Dockpipe-Industries/dockpipe/actions/runs/40247668385), attempt 1, completed successfully. |
| Public staging delivery | Candidate `0.6.33-staging.40247668385.1.b107ea64782c`: 25 downloaded files / 1,669,815,680 bytes checked against the immutable checksum manifest; 22 GitHub attachment digests independently matched. Immutable and rolling APT signatures, indexes and eight pool payloads verified. |

Original source labels in these receipts identify the builds that actually ran.
Later identity-only history correction does not rebind existing binaries or their
attestations to rewritten commits. Keep frozen release identifiers and consumer
pins intact; this closeout changes no source, dependency or release identity.

Receipt locator roles (historical evidence, not portable executable paths):

- `<local-evidence>/dockpipe-staging-review-20261025/final-receipt.md` and `.json`:
  final runtime promotion, hosted qualification, public payload/APT readback and
  isolated installed CLI proof.
- `<local-evidence>/dockpipe-staging-preparation/packages-checkpoint/receipt.json`:
  selected catalog source/tests and preservation of the five excluded paths.
- `<local-evidence>/dockpipe-staging-preparation/owner-docs-promotion/final-receipt.md`:
  owner promotions, public package readback and private-history preservation.
- `<local-evidence>/dockpipe-extraction/windows-final-host-prep-18ed33b-1/final-installed-acceptance-receipt.json`:
  earlier installed Windows host/public-package integration, including view,
  input acknowledgement, close/reopen and owned child cleanup.

## Limits and separately owned work

- No production promotion is included in this completion.
- Final staging installer/native checks are not a claim that every ordinary UI
  and live VM path was repeated on that exact candidate. The later local Windows
  0.6.31 installed test covered an ordinary UI subset; VM setup/launch was unrun.
  Earlier installed-host proof and protocol tests retain their exact source scope.
  Additional Guest Tools/VM work belongs to [TASK-045](../../vm-guest-tools/index.yaml);
  broader host qualification belongs to [TASK-025](../../supported-platform-program/index.yaml).
- Native catalog support is limited to its three declared profiles. Runtime
  release support on other platforms does not imply optional native VM support.
- The final receipt did not reverify asynchronous Homebrew synchronization or run
  a fresh public Windows PowerShell installation. These are distribution/platform
  acceptance limits, not missing source extraction.
- Five excluded catalog paths (README/backlog and Linux VM model/workflow edits)
  and four private-platform README/editor paths remain untouched. Their inclusion,
  retirement or separate follow-up is a user decision; task closure grants no
  permission to publish or delete them.
- Remaining historical email cleanup was explicitly stopped. It is not migration
  follow-up. Keep future repository identities configured as selected by the user.
- The [gradual C++ direction](cpp-direction.md) remains future work, outside the
  extraction acceptance contract and without implementation authority here.

## History

[Historical checkpoints](history.md) preserve the original scope, implementation,
failed attempts and superseded pending states. They do not reopen completed work.

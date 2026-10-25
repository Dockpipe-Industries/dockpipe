# Releasing dockpipe

This repo now supports an automated GitHub Actions release pipeline.

**Optional dev.to:** **PUT** a main article (**`DEVTO_ARTICLE_ID`**) and/or **POST** a one-time post per release (**`DEVTO_ONE_TIME_POST`**) — see **[devto.md](devto.md)** (**`DEVTO_PUBLISH`**, **`DEVTO_API_KEY`** secret).

**Ship model:** Integrate on **`staging`**; when ready, **PR `staging` → `master`** — that merge runs **Release** (see **[branching.md](branching.md)**). Repo-root **`VERSION`** selects the release line/notes baseline; the pipeline generates the next unused patch from stable and staging tags; **`release/releasenotes/X.Y.Z.md`** must exist and be updated on the **ship** PR. **CI** runs on **`staging`** PRs too (tests only); the **release-notes gate** applies only to PRs **into `master`**.

**Release notes body:** Start from [TEMPLATE.md](../releasenotes/TEMPLATE.md) and update the notes matching repo-root `VERSION`. Lead with end-user outcomes, then Linux/macOS/Windows installation, upgrade actions and known limits. Include a working recommended install path per platform; link alternatives and contributor build instructions instead of duplicating them. Verify asset names, prerequisites and signing/platform claims, and label preview or staging-only features. The workflow replaces baseline version references with the generated numeric version and publishes the body as the GitHub Release description.

---

After a green staging push, [the staging channel](staging.md) publishes a separate
installable candidate. Package versions are independent of the CLI release version.

## Release workflow

Pipeline file: `.github/workflows/release.yml`

Trigger options:

1. **Merge (push) to `master`** — ships the next generated **`vX.Y.Z`** if **`release/releasenotes/${VERSION}.md`** exists on that commit.
2. **Manual dispatch** (Actions UI):
   - `version`: optional release baseline — defaults to **`VERSION`**; the selected patch may advance beyond it
   - `dry_run`: defaults to `true` → build, verify, and upload workflow artifacts without requesting deployment approval. `false` is accepted only on `master`; other refs fail before platform builds.
   - `build_msi`: optional — defaults to **`true`**. On **push** to `master`, MSI is built when the committed marker file **`workflows/package/windows-msi/assets/SHIP_MSI`** is present. This repo currently keeps that marker checked in, so normal releases include WiX/MSI unless you intentionally remove it.

---

## What the pipeline does

1. Dockpipe workflows build the standalone CLI on Linux amd64/arm64, macOS Intel/Apple Silicon and Windows amd64. Runtime defaults and SDK resources are library-owned; no source checkout or core archive is required by an installed CLI. Native smoke checks execute outside the checkout.
2. Owning workflows create CLI archives, Linux packages, macOS desktop installers, optional Windows MSI and the Flatpak launcher-plus-CLI bundle against pinned Qt and Browser SDK inputs. Optional stores are explicit inputs; a valid empty store does not create a core-package requirement.
3. Native desktop and Flatpak host qualification remain required. Windows MSI packaging requires an actual non-skipped embedding PASS. Optional Cockpit and Marketplace catalogs retain independent strict checks and do not become mandatory runtime payloads.
4. The unprotected assembly job prepares catalogs and checksums. Dry runs also verify signed APT metadata with a disposable key and retain workflow artifacts. They have read-only repository permissions, no production secret references and no deployment environment.
5. Production publication requires a non-dry-run on `master` and the protected `release` environment. Staging publication requires the current `staging` head, its successful caller checks and the separate `release-staging` environment. Owning publication workflows upload the qualified immutable artifact set, APT indexes and runtime catalog to the configured destinations. Existing releases, package payloads and by-hash files are retained; production announcements remain master-only.

Stable and staging publication share one concurrency group from version selection through publication. Dry runs use separate groups and do not reserve versions. Dry-run verification must not require a release-environment approval or administrator bypass. Production environment protections remain in place.

Package generation does not prove installation on every downstream distro/version. The hosted matrix covers the selected native runners; physical Mac launchd, sleep/wake, Docker, and remote-worker acceptance still need hardware testing. macOS notarization, Windows Authenticode and winget submission are separate follow-ups; staging Homebrew has its own [native qualification and publication lane](../../workflows/publish/homebrew/README.md).

The Flatpak includes the CLI inside the desktop app; it does not replace native host
CLI packages. Staging publication and matching hosted Flatpak qualification are
[verified for the 0.6.3 candidate](../../workflows/build/flatpak/README.md#release-integration-and-proof-limits).
The public package host serves the `.flatpak`; the matching GitHub release assets
did not include it. Resolve download links from the published catalog rather than
assuming GitHub attachments. There is no Dockpipe Flathub listing or automatic-update
remote. Bazzite, Podman/SELinux, ARM64 Flatpak and authenticated provider/VM behavior
remain separate qualification work. See [user installation and manual updates](../../docs/install.md#flatpak-desktop-linux-amd64).

## Runtime and optional package release boundaries

The CLI, launcher and host SDK stay in this repository. A CLI-only installation does not need the launcher; desktop distributions provide their matching standalone CLI. Qt and the pinned Browser library remain required desktop dependencies. PipeLang's pinned Go module remains a compile-time library dependency of the CLI; its standalone compiler release is independent.

Optional VM, Cockpit, DorkPipe and other packages release from their owning repositories. They consume published host/SDK artifacts and declare `min_dockpipe_version` on each installable slice that needs newer host features. Repository `dockpipe_version` and its lock select the exact producer executable; they are not a package's compatibility minimum. Native plugins additionally retain their platform, ABI and Qt checks.

Host publication updates `packages/runtime/latest.json`. Marketplace discovery continues to use `packages/latest.json`, which is owned by independent package catalog promotion. A host release must not replace that catalog pointer or download optional packages to assemble its own artifacts. Immutable host payload URLs under `packages/releases/` and `packages/candidates/` remain unchanged. The staging Homebrew synchronizer falls back to the legacy combined pointer only when the new runtime pointer returns HTTP 404; corruption and other HTTP failures remain errors.

A new generic capability can therefore ship as a host/SDK release before a package uses it. The package then builds and qualifies against that published contract, without requiring a synchronized host rebuild. Optional integration failures remain visible in their own strict qualification workflows.

## Production credentials and public origin

The public production package origin is **https://packages.dockpipe.com**.
Operators configure its backing storage and private infrastructure independently;
release jobs receive no Cloudflare management token or Terraform-state credential.

Provide the selected publication repository, `R2_ENDPOINT_URL`,
`DOCKPIPE_RELEASE_BUCKET` and optional `R2_PREFIX` explicitly. Scoped
`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` references authorize object uploads.
`APT_SIGNING_KEY` references the channel's existing signing key; the public
`APT_SIGNING_FINGERPRINT` binds its expected identity. Production and staging use
separate protected GitHub environments and credentials. See the
[fork setup boundary](branching.md#running-a-fork) and
[R2 publication workflow](../../workflows/publish/r2/config.yml).

Configure and rotate those secret values through the operator's private setup
process. No production key is generated by a release or dry-run job. Preserve an
independent recovery copy and verify the public fingerprint. Never publish APT
metadata signed with the disposable dry-run key.

## Qualification before publication

Run Actions → Release with `dry_run=true` on the intended commit first. Inspect all five platform CLI outputs, the desktop and Flatpak bundles, qualification receipts, any explicitly selected optional stores, the non-skipped MSI embedding result, signed APT test and checksums. Production publication additionally requires the scoped secrets above and a verified public R2 hostname. Do not treat a local Linux build or a Go cross-build as native Mac/Windows qualification.

GitHub and R2 are separate services, so publication is not an atomic transaction across both. If a release upload fails, inspect which objects and tags exist before recovery; do not dispatch a second release with changed bytes under an existing version. Existing Git tags and a published version catalog are rejected before publication. A fresh complete run selects the next unused patch; update the release-line notes for the shipped changes. Change `VERSION` intentionally when advancing the major/minor line or setting a higher patch floor. Keep published tags: they are the allocation history.

## PipeLang qualification boundary

`workflows/ci/runtime/config.yml` runs every Go package owned by this standalone
runtime repository through the canonical sanitized Go test workflow. PipeLang is
a pinned external compiler/SDK module; its compiler conformance campaigns and
language extension build/publication workflows belong to the
[PipeLang repository](https://github.com/Dockpipe-Industries/pipelang).
Runtime qualification does not claim that separate compiler campaign passed.

## Pipeon desktop updater boundary

Pipeon and DorkPipe own their editor, code-server and optional desktop distribution
lanes. Runtime publication does not rebuild or replace those packages or unrelated
installed editors. Their independently locked artifacts and qualification remain
with their owning repositories.

> `release/releasenotes/<version>.md` is required. The workflow fails fast if it is missing.

**Before merging to `master` (optional but recommended):** run **[manual QA](../../docs/manual-qa.md)** for the platforms you changed.

**winget:** after the release is live, optionally submit/update a manifest for the Microsoft community repo — see **[../../workflows/install/distribution/assets/winget/README.md](../../workflows/install/distribution/assets/winget/README.md)**.

---

## Homebrew release follow-up

After release artifacts are published, update Homebrew formula SHA/version:

- Formula source in repo: `workflows/publish/homebrew/assets/dockpipe.rb`
- Maintainer instructions: `workflows/publish/homebrew/README.md`

---

## dev.to announcement (optional)

If **`DEVTO_PUBLISH=true`** and **`DEVTO_API_KEY`** are set, the workflow can **PUT** the main article (**`DEVTO_ARTICLE_ID`**) and/or **POST** a new one-time article each release (**`DEVTO_ONE_TIME_POST=true`**). See **[devto.md](devto.md)**.

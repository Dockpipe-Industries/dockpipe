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
   - `build_msi`: optional — defaults to **`true`**. On **push** to `master`, MSI is built when the committed marker file **`release/packaging/msi/SHIP_MSI`** is present. This repo currently keeps that marker checked in, so normal releases include WiX/MSI unless you intentionally remove it.

---

## What the pipeline does

1. Builds on native Linux amd64/arm64, macOS Intel/Apple Silicon, and Windows amd64 runners. Each runner builds the CLI and every package's source hook, verifies its complete store manifest, and runs a host workflow smoke test outside the checkout.
2. Creates Linux DEB, RPM, Alpine APK, Arch packages, Linux/macOS tarballs, Windows ZIP, and optional MSI. It also creates `dockpipe-packages_VERSION_OS-ARCH.tar.gz` for each native target. These stores include native resolver helpers and must not be interchanged across platforms. The reusable `.github/workflows/flatpak.yml` separately builds and tests `dockpipe-desktop_VERSION_linux_amd64.flatpak` (launcher plus CLI/core) and its `linux-amd64-flatpak-org.kde.Platform-6.10` Marketplace store. Pipeon's optional desktop application has its own distribution lane.
3. Requires all five native platform stores and successful Flatpak qualification before assembly. It verifies package checksums and records the separate Flatpak store and combined launcher/CLI download in `release-manifest.json`; `SHA256SUMS.txt` includes the bundle. Linux runs runtime/package/shell regressions and real signed-APT tests; Windows runs runtime/package regressions and MSI installation/removal when enabled.
4. The unprotected `assemble` job prepares the catalog and checksums. For dry runs it also builds a signed APT repository for amd64 and arm64 with immutable by-hash indexes, using a throwaway key, and uploads workflow artifacts. It has read-only repository permissions, no production secret references, and no deployment environment.
5. Only a non-dry-run on `master` enters `publish`, which requires the protected `release` environment. It downloads the prepared artifacts, signs APT with the production key, and publishes release assets to GitHub and every individual package/store manifest to R2 at `packages/releases/VERSION/`. APT lives at `apt/`. Upload order is package payloads, APT pool/index files, signed metadata, then the version catalog and `packages/latest.json`. Old versions and old by-hash files are retained. The optional dev.to job uses the same master-only production condition.

Stable and staging publication share one concurrency group from version selection through publication. Dry runs use separate groups and do not reserve versions. Dry-run verification must not require a release-environment approval or administrator bypass. Production environment protections remain in place.

Package generation does not prove installation on every downstream distro/version. The hosted matrix covers the selected native runners; physical Mac launchd, sleep/wake, Docker, and remote-worker acceptance still need hardware testing. macOS notarization, Windows Authenticode and winget submission are separate follow-ups; staging Homebrew has its own [native qualification and publication lane](../packaging/homebrew/README.md).

The Flatpak includes the CLI inside the desktop app; it does not replace native host
CLI packages. Staging publication and matching hosted Flatpak qualification are
[verified for the 0.6.3 candidate](../packaging/desktop/flatpak/README.md#release-integration-and-proof-limits).
The public package host serves the `.flatpak`; the matching GitHub release assets
did not include it. Resolve download links from the published catalog rather than
assuming GitHub attachments. There is no Dockpipe Flathub listing or automatic-update
remote. Bazzite, Podman/SELinux, ARM64 Flatpak and authenticated provider/VM behavior
remain separate qualification work. See [user installation and manual updates](../../docs/install.md#flatpak-desktop-staging-linux-amd64).

## Production credentials and public origin

Public origin: **https://packages.dockpipe.com**. The R2 package bucket is **dockpipe**. Binding that hostname to the bucket, public access, and the private Terraform-state bucket are managed separately through `package-store-infra`; release jobs receive no Cloudflare management token or state-bucket credential.

The 1Password Environment **Dockpipe Packages - Production** (`rimccmsvbaehthwukfmrmskh6m`) should contain only:

| Variable | Scope |
| --- | --- |
| `AWS_ACCESS_KEY_ID` | R2 Object Read & Write credential restricted to the `dockpipe` bucket |
| `AWS_SECRET_ACCESS_KEY` | Matching restricted R2 secret |
| `APT_SIGNING_KEY` | ASCII-armored private APT signing key for unattended signing |

Keep the existing broader Cloudflare credentials in their original vault item. The project config names `packages-production` for the two upload variables and `packages-release-setup` for all three. Neither is the project default.

Run the [release setup workflow](../../workflows/package/package-release-setup/README.md) to validate the key and copy only those three secrets into GitHub's **release** environment. The helper also sets public variables `APT_SIGNING_FINGERPRINT`, `R2_ENDPOINT_URL`, `DOCKPIPE_RELEASE_BUCKET`, and `R2_PREFIX`. GitHub then publishes unattended using scoped copies; it does not require desktop login or access to the rest of the 1Password vault. Re-run setup when rotating credentials. Apply environment protection rules before publishing.

No production key is generated by a release or dry-run job. Create the production key once, save it in the new environment, preserve an independent recovery copy, and verify its public fingerprint. Do not publish an APT repository signed with the dry-run key.

## Qualification before publication

Run Actions → Release with `dry_run=true` on the intended commit first. Inspect all five native stores, the Flatpak bundle and runtime-specific store, Flatpak qualification receipts, native smoke results, the MSI check, signed APT test, and checksums. Production publication additionally requires the scoped secrets above and a verified public R2 hostname. Do not treat a local Linux build or a Go cross-build as native Mac/Windows qualification.

GitHub and R2 are separate services, so publication is not an atomic transaction across both. If a release upload fails, inspect which objects and tags exist before recovery; do not dispatch a second release with changed bytes under an existing version. Existing Git tags and a published version catalog are rejected before publication. A fresh complete run selects the next unused patch; update the release-line notes for the shipped changes. Change `VERSION` intentionally when advancing the major/minor line or setting a higher patch floor. Keep published tags: they are the allocation history.

## PipeLang qualification boundary

The 0.6 runtime release uses `release/packaging/test-runtime.sh`: all root-module Go packages except `src/lib/pipelang/...`, `src/lib/applicationir`, `tests/pipelangcompat`, and `tests/containedexec/...`. Host CI on Linux and Windows and the nested Docker test workflow use the same selector. This is runtime qualification only; the separate PipeLang campaign remains required for compiler qualification. Those compiler suites intentionally refuse generated compilation without their verified containment runner. A plain broad local test run failed those containment checks and timed out in PipeLang; it is not passing release evidence.

PipeLang's compiler campaigns remain under `tests/containedexec/` and must pass separately before the planned language release. This runtime gate does not claim compiler conformance or remove those tests. The public language rollout is deferred to 0.7; existing experimental CLI surfaces remain present.

## Pipeon desktop updater boundary

If/when this repo publishes `pipeon-desktop` updater artifacts, keep the updater scope narrow:

- updater artifacts may replace the **Tauri desktop shell** only
- updater artifacts must **not** bundle the Pipeon code-server image, Pipeon VSIX, stock VS Code, Cursor, or unrelated Dockpipe/DorkPipe binaries

Treat the Pipeon desktop shell and the Pipeon editor/runtime surface as **separate distribution lanes**:

- **desktop shell** = signed Tauri updater artifacts plus a `latest.json` feed
- **Pipeon surface** = explicit refresh/rebuild/restart flow for the code-server image, VSIX, and local first-party binaries

> `release/releasenotes/<version>.md` is required. The workflow fails fast if it is missing.

**Before merging to `master` (optional but recommended):** run **[manual QA](../../docs/manual-qa.md)** for the platforms you changed.

**winget:** after the release is live, optionally submit/update a manifest for the Microsoft community repo — see **[../packaging/winget/README.md](../packaging/winget/README.md)**.

---

## Homebrew release follow-up

After release artifacts are published, update Homebrew formula SHA/version:

- Formula source in repo: `release/packaging/homebrew/dockpipe.rb`
- Maintainer instructions: `release/packaging/homebrew/README.md`

---

## dev.to announcement (optional)

If **`DEVTO_PUBLISH=true`** and **`DEVTO_API_KEY`** are set, the workflow can **PUT** the main article (**`DEVTO_ARTICLE_ID`**) and/or **POST** a new one-time article each release (**`DEVTO_ONE_TIME_POST=true`**). See **[devto.md](devto.md)**.

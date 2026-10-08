# Staging release channel

A successful **push to `staging`** runs the Linux and Windows CI jobs, then calls
`release.yml` from the same commit. PRs and manual CI runs do not publish. All five
native builds, package stores, installer checks and release tooling checks must
succeed before publication. CodeQL remains a separate workflow; this dependency
chain gates on the `CI` test jobs, not the separate CodeQL result.

The staging publisher uses GitHub environment **`release-staging`**, R2 bucket
**`dockpipe-staging`**, and **`https://packages.staging.dockpipe.com`**. Configure the
environment to allow only the staging branch, without a manual reviewer if
publication should be automatic. Its credentials must be restricted to the staging
bucket. Production still requires master and environment `release`.

The CI call uses `secrets: inherit` to work around GitHub's environment-secret
resolution issue in reusable workflows
([actions/runner#4453](https://github.com/actions/runner/issues/4453)). This also
makes caller repository and organization secrets available to the called workflow.
Keep publication credentials in their respective GitHub environments: the staging
publisher binds `release-staging`, whose values take precedence for that job.

Staging uses the same native build and artifact assembly jobs as production:

| Output | Platforms |
| --- | --- |
| DockPipe CLI archives and complete package-store bundles | Linux amd64/arm64, macOS amd64/arm64, Windows amd64 |
| DEB, RPM, APK and Arch packages | Linux amd64/arm64 |
| MSI installer | Windows amd64; always enabled for staging |
| Signed APT repository | amd64/arm64, suite `staging`, separate signing key |
| Install scripts, release catalog, package manifests and checksums | Included with each candidate |

GitHub receives a prerelease with the downloadable release assets. R2 receives the
complete artifact tree, including individual platform package stores and APT.
Each new successful staging push creates a separately selectable candidate.

## Versions and immutable candidates

- Repo-root `VERSION` selects the release line and notes baseline. The pipeline generates
  the next unused numeric patch from all stable and staging release tags (for example,
  `0.6.1`, then `0.6.2`). CLI, core, launcher and native installers use that version.
- Each package's `package.yml` owns its independent version. Generated children
  inherit the nearest owner; explicit child versions take precedence. Existing
  equal version numbers do not imply that future bumps must be synchronized.
- A staging candidate is `X.Y.Z-staging.RUN_ID.ATTEMPT.SHORT_SHA`. GitHub publishes
  `v<CANDIDATE>` as a prerelease with `make_latest: false`.
- R2 stores each complete candidate at `packages/candidates/<CANDIDATE>/`.
  `release-manifest.json` records its full source SHA and per-platform store paths.
  Each store manifest retains the individual package versions and checksums.
- `packages/latest.json` points to the latest completed staging candidate.
  Publication uploads payloads and signed APT metadata before the catalog commit
  marker and pointer. Existing committed candidates cannot be overwritten.

Native package and MSI versions use the generated numeric `X.Y.Z`, so each published
build advances the package manager's version. Stable and staging publication share
one serialized allocator; dry runs preview the next version without reserving it.
An existing published tag, including a partial publication, consumes that patch.
New full runs select the next unused patch. Do not delete release tags to reuse numbers.

Staging publishes a rolling signed APT repository at `https://packages.staging.dockpipe.com/apt`
(suite `staging`), plus an immutable `<candidate-base>/apt` snapshot for explicit pins.
Old pool objects and by-hash indexes are retained. These installers use the normal
DockPipe product identity; configure one channel per machine. Source/default installs
continue to select production unless staging is explicitly configured.

## Infrastructure preparation

Run from this checkout with the existing 1Password desktop integration available:

```bash
./src/bin/dockpipe --workflow package-store-staging-infra --workdir . --tf plan --
```

The maintainer wrapper calls the existing Cloudflare R2 package's Terraform runner.
It pins the staging bucket and hostname after vault injection, copies only the
module's `.tf` files and optional lockfile to a private workflow artifact directory,
and uses state bucket `dockpipe-tfstate` with key
`state/package-store-staging/terraform.tfstate`. It clears inherited production
Terraform flags, imports, workspaces, tfvars and backend overrides. Zone-level
cache and WAF rules remain disabled to avoid separate state claiming the existing
zone entrypoints. The existing R2 custom-domain resource owns the HTTPS binding.

The workflow is plan-only and reports a saved `staging.tfplan` path. Review that
plan for exactly the staging bucket and custom domain before separately approving
apply. Plan files contain sensitive provider inputs; keep them in the private
artifact directory and never attach them to a PR. No new state bucket is needed.

## Publication credentials

Provision bucket-scoped R2 Object Read & Write credentials for `dockpipe-staging`
and a staging APT signing identity. Store them in a separate secret environment;
do not reuse the production upload token. `dockpipe.config.json` maps
`packages-staging-release-setup` to the staging 1Password environment with bindings
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `APT_SIGNING_KEY`.
`packages-staging` binds only the upload credential pair.

The existing setup workflow supports checks followed by separately approved writes:

```bash
./src/bin/dockpipe --workflow package-release-setup --workdir . \
  --secret-environment packages-staging-release-setup \
  --var RELEASE_SETUP_ENVIRONMENT=release-staging \
  --var DOCKPIPE_RELEASE_BUCKET=dockpipe-staging --
```

After review, the same command with `--var RELEASE_SETUP_APPLY=1` copies only those
three secrets and the public endpoint, bucket, prefix and signing fingerprint into
`release-staging`. It neither creates provider credentials nor starts a release.
The existing root `.env.template` remains the infrastructure credential source.

## Installing a selected candidate

Read the staging pointer, inspect its catalog and select its immutable identity.
For example, substitute the actual published candidate below:

```bash
candidate='0.6.1-staging.RUN_ID.ATTEMPT.SHORT_SHA'
base="https://packages.staging.dockpipe.com/packages/candidates/$candidate"
curl -fsSL "$base/install.sh" -o /tmp/dockpipe-staging-install.sh
DOCKPIPE_VERSION=0.6.1 DOCKPIPE_DOWNLOAD_BASE="$base" \
  DOCKPIPE_INSTALL_MODE=portable sh /tmp/dockpipe-staging-install.sh
```

Use the catalog's numeric `version` for `DOCKPIPE_VERSION`; do not pass the candidate
suffix as a native installer version. Windows supports the same environment
variables with the candidate's `install.ps1`. Full platform package bundles are
available beside the CLI, and individual stores are under `stores/<platform>/`.

### Homebrew on a test Mac

The public [DockPipe tap](https://github.com/Dockpipe-Industries/homebrew-dockpipe)
is active. Its first [native validation run](https://github.com/Dockpipe-Industries/homebrew-dockpipe/actions/runs/37397290496)
passed installation and formula tests on Apple Silicon and Intel before publishing:

```sh
brew install Dockpipe-Industries/dockpipe/dockpipe-staging
dockpipe --version
```

Subsequent candidates use `brew update` followed by `brew upgrade dockpipe-staging`.
The tap polls completed staging releases every 15 minutes and tests installation
on Apple Silicon and Intel before updating. GitHub schedules can run late; manual
dispatch is also available. No additional upload secret is required.

The Homebrew formula version includes the candidate provenance in addition to the
generated numeric version. It installs the
native CLI and required core package. The command is still `dockpipe` and uses normal user data unless
`DOCKPIPE_GLOBAL_ROOT` is set to a separate test directory.
The stable formula is not yet published, so a conflict declaration against it is invalid.

The desktop cask is published and passed
[native Homebrew validation on both Mac architectures](https://github.com/Dockpipe-Industries/homebrew-dockpipe/actions/runs/37419531103).
Run `brew update`, then use
`brew install --cask dockpipe-industries/dockpipe/dockpipe-desktop-staging` for the
launcher and CLI together. The DMG offers a direct Apple Installer alternative.
See [desktop installation](../../docs/install.md#desktop-installation) for platform
choices and the outstanding Apple signing/notarization boundary.

### APT staging installation and upgrades

After a release using the rolling repository is published, configure its permanent URL
once. Download the staging public key and check its fingerprint:

```bash
curl -fsSL https://packages.staging.dockpipe.com/apt/dockpipe-archive-keyring.gpg -o /tmp/dockpipe-staging-keyring.gpg
gpg --show-keys --with-fingerprint /tmp/dockpipe-staging-keyring.gpg
```

The staging fingerprint is `7295FA3FC25A3998E146D0779EAF522778B9C909`
(expires 2028-10-04). After confirming it:

```bash
sudo install -m 0644 /tmp/dockpipe-staging-keyring.gpg /usr/share/keyrings/dockpipe-staging.gpg
printf '%s\n' 'deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/dockpipe-staging.gpg] https://packages.staging.dockpipe.com/apt staging main' \
  | sudo tee /etc/apt/sources.list.d/dockpipe-staging.list >/dev/null
sudo apt update
sudo apt install dockpipe-desktop
```

The desktop package installs its exact-version CLI dependency. Headless machines can
install only `dockpipe`. Subsequent releases use normal `sudo apt update` and
`sudo apt upgrade`; no candidate URL edits or `--reinstall` are needed.
Existing candidate-pinned sources must be replaced with this permanent source once;
installers do not rewrite user APT configuration. Do not enable production and staging
sources together. The immutable candidate APT URL remains available for intentional pins.

A rerun of all jobs gets a new attempt identity. Rerunning only a failed publication
job may encounter an existing GitHub tag or R2 catalog; inspect the partial result
first. GitHub and R2 do not form one transaction. Concurrent staging runs are
serialized, and the publisher rejects commits that are no longer the staging head.

Activation requires infrastructure, scoped credentials and these source changes on
staging. The reusable workflow resolves from the caller's commit, so staging can
be qualified before merging the feature into master. Native hosted success and a
real installation from the public staging origin remain required release evidence.

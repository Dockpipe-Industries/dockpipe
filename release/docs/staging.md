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

- Repo-root `VERSION` names the planned CLI/core release.
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

Native package and MSI versions remain numeric `X.Y.Z`. Repeated staging candidates
for the same planned release are explicitly selected installs; package managers
will not recognize them as successive version upgrades. APT repositories are also
candidate-specific (`<candidate-base>/apt`, suite `staging`) so two candidates never
replace the same native package file. Use an isolated test installation; these
installers use the normal DockPipe product/package identity, not a side-by-side
staging product. Defaults continue to select stable releases.

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
candidate='0.6.0-staging.RUN_ID.ATTEMPT.SHORT_SHA'
base="https://packages.staging.dockpipe.com/packages/candidates/$candidate"
curl -fsSL "$base/install.sh" -o /tmp/dockpipe-staging-install.sh
DOCKPIPE_VERSION=0.6.0 DOCKPIPE_DOWNLOAD_BASE="$base" \
  DOCKPIPE_INSTALL_MODE=portable sh /tmp/dockpipe-staging-install.sh
```

Use the catalog's numeric `version` for `DOCKPIPE_VERSION`; do not pass the candidate
suffix as a native installer version. Windows supports the same environment
variables with the candidate's `install.ps1`. Full platform package bundles are
available beside the CLI, and individual stores are under `stores/<platform>/`.

### APT installation in a test VM

Use the same candidate `base` selected above. Download its public key and compare
the fingerprint with the staging signing identity before adding the repository:

```bash
curl -fsSL "$base/apt/dockpipe-archive-keyring.gpg" -o /tmp/dockpipe-staging-keyring.gpg
gpg --show-keys --with-fingerprint /tmp/dockpipe-staging-keyring.gpg
```

The initial staging key fingerprint is
`7295FA3FC25A3998E146D0779EAF522778B9C909` (expires 2028-10-04).
After confirming the fingerprint, run in the disposable test VM:

```bash
sudo install -m 0644 /tmp/dockpipe-staging-keyring.gpg /usr/share/keyrings/dockpipe-staging.gpg
printf 'deb [signed-by=/usr/share/keyrings/dockpipe-staging.gpg] %s/apt staging main\n' "$base" \
  | sudo tee /etc/apt/sources.list.d/dockpipe-staging.list >/dev/null
sudo apt-get update
sudo apt-get install dockpipe
dockpipe --version
```

To test another candidate with the same numeric version, update the source URL,
run `apt-get update`, then `apt-get install --reinstall dockpipe`. Do not configure
both production and staging repositories in this VM: their package name and numeric
version can be identical. Test the other native installers on their target OS;
portable-install success alone does not qualify APT or MSI installation.

A rerun of all jobs gets a new attempt identity. Rerunning only a failed publication
job may encounter an existing GitHub tag or R2 catalog; inspect the partial result
first. GitHub and R2 do not form one transaction. Concurrent staging runs are
serialized, and the publisher rejects commits that are no longer the staging head.

Activation requires infrastructure, scoped credentials and these source changes on
staging. The reusable workflow resolves from the caller's commit, so staging can
be qualified before merging the feature into master. Native hosted success and a
real installation from the public staging origin remain required release evidence.

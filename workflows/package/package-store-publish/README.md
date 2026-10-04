# package-store-publish

Repo-local maintainer workflow for the self-hosted package mirror.

What it does:

1. builds the repo-local `dockpipe` binary
2. runs `dockpipe build --no-images`
3. writes `release/artifacts/install-manifest.json` plus `templates-core-<version>.tar.gz`
4. writes `release/artifacts/packages-store-manifest.json` plus one tarball per compiled workflow and resolver
5. uploads each artifact file individually with `dockpipe release upload`

This is the correct mirror shape for `dockpipe install core` and store-backed workflow/resolver pulls. It does **not** tar the whole `release/artifacts/` directory into one object.

## Secrets

The workflow uses `vault: environment`. Select `packages-production`, configured in the repo's `dockpipe.config.json`, to read only `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` from **Dockpipe Packages - Production** (`rimccmsvbaehthwukfmrmskh6m`). The credential must have Object Read & Write access to the **dockpipe** package bucket only. Existing Cloudflare management and Terraform-state credentials stay in the original vault item.

The endpoint and bucket are non-secret workflow variables. No `op://` template or broad Cloudflare token is loaded. Install a 1Password CLI version supporting `op run --environment` and authenticate it first. Build the secrets resolver with `dockpipe build --no-images` after updating this checkout.

This local workflow builds a **single host's** store. The release pipeline builds separate stores for each OS/CPU pair because resolvers contain native helpers. Use the release pipeline for the public cross-platform catalog; do not replace it with a store from a different host.

## Run

Dry-run is the default:

```bash
./src/bin/dockpipe --workflow package-store-publish --secret-environment packages-production --
```

On a machine without the AWS CLI, DockPipe will prompt to install it from the workflow dependency definition before continuing.

Real upload:

```bash
./src/bin/dockpipe --workflow package-store-publish --secret-environment packages-production --var R2_PUBLISH_DRY_RUN=0 --
```

Build only:

```bash
./src/bin/dockpipe --workflow package-store-publish --secret-environment packages-production --var PACKAGE_RELEASE_SKIP_UPLOAD=1 --
```

If you still need bucket/domain provisioning, run `package-store-infra` separately first.

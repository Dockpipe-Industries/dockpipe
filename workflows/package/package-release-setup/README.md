# Package release setup

One-time bootstrap for the GitHub `release` environment, using **Dockpipe Packages - Production**. It copies exactly three secrets over stdin: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `APT_SIGNING_KEY`. The R2 credential must be restricted to Object Read & Write on the `dockpipe` package bucket. Do not use the existing credential that can access Terraform state.

The existing Cloudflare management token, state credentials, and their vault item remain unchanged. GitHub receives scoped copies, so CI requires neither interactive desktop approval nor a broad 1Password service account. Rotate these copies by rerunning setup after changing the production environment.

Prerequisites: authenticated `op` CLI with Environments support, `gh` with `secret` and `variable` commands and repository secret-management access, Python 3, Bash, and GnuPG. The GitHub environment named `release` must exist. Store an ASCII-armored private signing key as `APT_SIGNING_KEY`; the helper checks that it can sign unattended and derives its fingerprint. If the environment flattens the armor into one line, the helper restores line breaks before GnuPG validation and GitHub upload. It does not generate a production key or write to 1Password.

Check configuration (no GitHub writes):

```bash
./src/bin/dockpipe --workflow package-release-setup --secret-environment packages-release-setup --
```

Apply after inspecting the target and fingerprint:

```bash
./src/bin/dockpipe --workflow package-release-setup --secret-environment packages-release-setup --var RELEASE_SETUP_APPLY=1 --
```

GnuPG uses a temporary private directory, which is removed after validation. Provider errors and GitHub responses are withheld. No resolved env file or credentials are written into the checkout. A GitHub API failure can leave a partial configuration; rerunning the same setup completes it. This workflow does not publish artifacts or modify Cloudflare infrastructure.

For staging, select `RELEASE_SETUP_ENVIRONMENT=release-staging`, the `dockpipe-staging`
bucket, and a separate named secret environment containing staging-scoped keys.
See [staging releases](../../../release/docs/staging.md).

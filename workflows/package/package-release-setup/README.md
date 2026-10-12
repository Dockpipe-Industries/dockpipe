# Package release setup

Configure your GitHub `release` or `release-staging` environment with scoped package
publication credentials. The helper copies exactly three secrets over stdin:
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `APT_SIGNING_KEY`. Restrict the
storage credential to your publication bucket; do not supply Terraform-state or
account-management credentials.

No repository, cloud account, endpoint, or bucket is selected for you. Configure
these non-secret targets through an ignored `.env` file, your private vault template,
or explicit `--var` arguments:

```dotenv
RELEASE_SETUP_REPOSITORY=your-owner/your-repository
RELEASE_SETUP_ENVIRONMENT=release
R2_ENDPOINT_URL=https://your-account-id.r2.cloudflarestorage.com
DOCKPIPE_RELEASE_BUCKET=your-publication-bucket
R2_PREFIX=packages
```

The values above are placeholders. Keep actual identifiers in private configuration.
Configure a named secret environment in your own private project, or use your
provider's CLI to inject only the three required credentials and run with `--no-vault`.
The public project configuration contains example environment IDs only.

Prerequisites: authenticated provider CLI, `gh` with repository secret-management
access, Python 3, Bash, and GnuPG. Create the target GitHub environment first.
Store an ASCII-armored private signing key as `APT_SIGNING_KEY`; the helper checks
unattended signing and derives its fingerprint. It restores valid flattened armor
but does not generate a key or change your secret provider.

Check the explicitly configured targets without GitHub writes:

```bash
dockpipe --workflow package-release-setup --secret-environment packages-release-setup --
```

After inspecting the repository, environment, bucket, endpoint, and fingerprint,
repeat with `--var RELEASE_SETUP_APPLY=1` to copy the scoped secrets and public
settings. Missing target settings fail before GnuPG or GitHub operations.

GnuPG uses a temporary private directory removed after validation. Provider errors
and GitHub responses are withheld. A GitHub failure can leave partial configuration;
read it back before retrying. This workflow publishes no artifacts and changes no
Cloudflare infrastructure. For staging use `release-staging` and separate scoped
credentials, a staging publication bucket, and private state configuration.

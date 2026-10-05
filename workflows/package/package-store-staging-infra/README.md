# Staging package infrastructure

Run `dockpipe --workflow package-store-staging-infra --workdir . --tf plan --`
from the repository checkout. This maintainer workflow composes the existing
Cloudflare R2 package and Terraform pipeline with a staging-only bucket, custom
domain and remote state key. It requires the existing `op` vault template and
produces a private saved plan; it does not apply it.

See [the staging release guide](../../../release/docs/staging.md) for isolation,
credentials, candidate installation and activation.

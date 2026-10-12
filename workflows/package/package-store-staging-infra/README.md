# Staging package infrastructure

This workflow produces a saved Terraform plan; it never applies it. Supply your
own staging targets in private configuration or with explicit `--var` arguments:

```dotenv
STAGING_R2_BUCKET=your-staging-package-bucket
STAGING_PUBLIC_HOSTNAME=staging-packages.example.com
STAGING_TF_STATE_BUCKET=your-private-state-bucket
STAGING_TF_STATE_KEY=state/staging/terraform.tfstate
```

Also supply your Cloudflare account, zone, provider credentials and separate state
credentials through the ignored vault template. Replace every example value.
The staging state key must be distinct from production, and the publication bucket
must differ from the private state bucket. Missing targets fail before Terraform.

Run `dockpipe --workflow package-store-staging-infra --tf plan --` from the checkout.
The wrapper preserves the explicit staging targets, removes inherited production
Terraform settings, and uses a fresh module directory without production tfvars or
state. See [staging releases](../../../release/docs/staging.md).

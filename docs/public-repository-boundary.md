# Public repository configuration boundary

This repository and its published packages are public, reusable product source.
A setting being safe in a private operational repository does not make it suitable
here. Account identifiers and infrastructure locations can be private even when
they are not credentials.

## What belongs here

- Portable code, workflows, schemas, documentation, and synthetic test fixtures.
- Empty required settings and clearly labelled placeholders such as
  `your-production-environment-id`, `example/project`, and `packages.example.com`.
- Official product identity, license attribution, source links, and public download
  endpoints needed to install Dockpipe.
- Provider-wide constants, such as documented public managed-ruleset identifiers.

## What stays private

Personal vault/environment IDs, cloud account and zone IDs, private hostnames and
addresses, machine paths, deployment bucket names, credential references identifying
an operator's account, and operational receipts belong in private configuration or
private operational repositories. Do not copy them into examples, research records,
agent task notes, package assets, or release documentation.

Use ignored `.env` files or vault templates and explicitly supplied workflow variables
for local operations. The checked-in named secret environments contain placeholders;
configure actual references in your private project. `dockpipe.config.local.json` is
not a supported override. Do not invent a local override that the runtime never reads.
For a maintainer running this checkout, an authenticated provider CLI can supply
scoped credentials around a command using `--no-vault`; keep its environment ID in
private shell configuration. GitHub publication uses separately configured
`release` and `release-staging` environments.

Reusable deployment tools must require their target repository, account, endpoint,
and bucket before mutation. Missing settings must fail with a useful message, not
fall back to Dockpipe Industries infrastructure. Keep staging and production
credentials, buckets, and state keys separate. Checks-only and dry-run modes remain
the default where provided.

## Verification

Run `python3 release/packaging/check-public-content.py` before committing or packaging.
CI runs the same check. The release build also checks compiled package archives.
The checker reports locations and reasons without repeating matched values.
It detects common environment IDs, account-specific endpoints, personal home paths,
and organization-specific deployment defaults. It is a focused regression gate,
not proof that every possible secret or personal detail has been detected.

Review changed examples and generated package contents as well as source. Synthetic
fixtures may model provider formats but must never reuse a real operator's values.
Deleting a value from the current tree does not remove previous commits or immutable
release artifacts; history removal is a separate coordinated operation.

# Secrets package

Resolver-backed named secret environments for 1Password Environments, AWS Secrets Manager, Azure Key Vault, and Infisical. Existing 1Password template workflows remain supported.

See [Secret environments](../../docs/runtime/vault.md) for configuration, provider authentication, limits, and the generic resolver contract.

Build native helper binaries and compile the resolvers from this checkout:

```bash
./src/bin/dockpipe build --no-images
./src/bin/dockpipe package test --workdir . --only secrets
```

Release stores contain a helper built for the store's OS and CPU. Install the matching platform store; no Go compiler is needed by consumers. Provider CLIs (`op`, `aws`, `az`, or `infisical`) must be installed and authenticated on the host. Source-only resolver compilation needs the source-build hook to package the helper first.

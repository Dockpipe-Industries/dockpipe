# Windows qualification validation artifact

This manually invoked producer exports the public pure QEMU forwarding validator
without importing it, starting a display, or contacting a guest. It does not run
in CI and does not qualify a feature.

```sh
"$DOCKPIPE_BIN" --workflow-file /absolute/product/fixtures/windows-validation/config.yml -- --output /absolute/new-output
```

Output `dockpipe-product-windows-validation-0.1.0.tar.gz` has exactly two regular
members: `manifest.json` and `native_arguments.py`. The strict manifest declares
schema `dockpipe.product-windows-validation/v1`, version `0.1.0`, API
`qemu-forwarding/v1`, and the module's exact byte length and SHA-256. Tar/gzip
metadata is deterministic. A linked, oversized or changing source is refused
before creating output. The existing generic desktop fixture/default producer
is unchanged.

The VM package consumes an operator-selected absolute artifact path and SHA.
Its admission checkpoint verifies closure and digests without importing this
code. A later qualified host observer must admit actual QEMU identity and argv
before invoking `validate_forwarding`; the artifact itself proves no guest or
transport readiness. No sibling source checkout is a runtime dependency.

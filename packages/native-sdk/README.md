# Native SDK artifact verification

Shared publisher tooling for ForgePipe, Nucleon and future native SDKs. This first
Linux implementation audits exact ELF exports, retains separate debug symbols,
checks matching build IDs and verifies that stripping leaves every allocated
section unchanged. It does not obfuscate code or change compiler optimization.

```sh
python3 assets/scripts/native_sdk.py audit --binary libexample.so \
  --exports public-exports.txt --receipt audit.json
python3 assets/scripts/native_sdk.py split --binary libexample-with-debug.so \
  --exports public-exports.txt --runtime public/libexample.so \
  --debug private/libexample.debug --receipt private/split-receipt.json
```

Build with debug information first. A Release binary without debug information is
rejected by `split`: the only diagnostic copy must never be discarded. Receipts
and private symbols belong in private retained artifacts, outside public archive
roots. Final runtime/SDK archive allowlisting remains the publisher's responsibility.
Do not distribute a raw build/install directory as the protected SDK.

`verify` accepts detached OpenSSL raw-message signatures (tested with Ed25519) and a publisher public key
supplied from independently trusted configuration. Neither an archive's own key
nor a checksum is publisher authentication. Metadata is UTF-8 JSON (at most 1 MiB):
`schema: 1`, `artifact_class: runtime|sdk`, and `files`, each containing `path`,
`bytes`, `sha256`. All files must be listed exactly, with no links. `--max-bytes`
is the consuming product's declared aggregate package budget.

```sh
python3 assets/scripts/native_sdk.py verify --manifest release.json \
  --signature release.sig --public-key trusted-publisher.pem \
  --root unpacked --max-bytes 134217728
```

Verify an archive's digest/signature before safe extraction, then verify its
complete extracted file set before any execution. This tool supplies directory
verification, not an archive extractor or a configured release signing service.
Signing keys, approvals and retention are owned by the release environment.
No real signing key is created or installed by this package.

Windows PDB/export and macOS dSYM/code-signing verification are still pending.
No native performance, licensing approval or public publication is implied by a
successful artifact audit. SDK consumers must still test the distributed binary.

# Native SDK audit and release boundaries

Scope: ForgePipe, Nucleon, their PipeLang/DockPipe consumers and an optional ForgePipe
compression adapter. This supplements the extraction; it does not replace its acceptance.

## Observed baseline

| Area | Current evidence | Required change |
| --- | --- | --- |
| ForgePipe initial SDK | Qt C++ API, 141 dynamic definitions including Qt template internals; Release ELF retains `.symtab`; no private debug build | Treat compatibility SDK as internal; add minimal versioned C ABI, explicit exports, private symbols and packaged consumers |
| Nucleon 0.3.0 | ABI 1, 20 exported C entry points; opaque decoder/stream handles; hidden visibility and Linux export map; 29/29 fresh native tests pass | Preserve API and codec. Tighten export allowlist; separate debug artifacts; verify binary packages and identity |
| Nucleon release | Fourteen SDK lanes, eleven package consumers and four debug bindings pass; tested packages, receipts and matching debug symbols are durably published privately | Keep public registry publication gated on approved license/destination and authenticated delivery; SHA-256 alone is not publisher authentication |
| Nucleon approvals | `distribution_approved=false`, `names_verified=false`, license URL empty | Retain external publication block until approved terms and destinations exist |
| PipeLang | Package-owned Nucleon facades/manifests, installed CMake integration, optional executable-compression pilot | Pin compiler + manifest + SDK identities; keep private compiler out of external SDK bundles; do not claim a production-wide default |
| DockPipe catalog | SHA-256 checking and bounded safe installation; no publisher-signature check found in packagecatalog | Establish publisher authentication independently of artifact checksums before public native SDK distribution |
| Platform proof | Fresh Linux x86-64 native SDK tests; existing portable target definitions and pipeline instructions | Windows/macOS/ARM64/Flatpak require actual build/runtime evidence; do not infer them from Linux |

The first ForgePipe extraction predates the hardening request and is explicitly an internal
compatibility SDK. Public metadata must not label its broad Qt C++ headers as the final
protected native API. Nucleon's implementation, embedded models and performance research
stay in its private repository. No algorithm source belongs in DockPipe or ForgePipe.

## Threat model

- Accidental source/debug/credential disclosure: construct artifacts from explicit allowlists,
  reject unexpected members and private implementation files; inspect the final archive.
- Excess symbols and implementation details: hidden internals and an exact public export set;
  retain full private diagnostics tied to the runtime build ID.
- Modified packages/metadata: verify content digests against authenticated publisher metadata
  using independently trusted keys/identity; fail before extraction or execution.
- Unauthorized redistribution: clear artifact-specific license terms and release authorization.
  Technical integrity alone cannot grant rights or prevent copying an authorized native binary.
- Algorithm reverse engineering: client code is inspectable. No invasive anti-debugging or DRM;
  optional selective obfuscation stays disabled unless measured benefits justify its costs.

## Artifact and license segregation

Runtime: executable/shared libraries and required assets/notices only. SDK: approved public
headers, import libraries/wrappers, examples/docs and runtime compatibility metadata. Internal:
private symbols, source maps, implementation headers, profiling/test instrumentation and detailed
build evidence. Private symbols have separate storage/access and retention at least as long as
supported released binaries. Never discard the sole diagnostic copy while stripping releases.

Preserve Apache-2.0 provenance for public DockPipe-derived source, CC0 asset notices, Nucleon's
independent commercial identity, and each third-party license. Private repository visibility is
not a license change. Public engine redistribution intent includes Cockpit/Spatial Browser and
community modifications/worlds, subject to final terms. An internal-use artifact is not thereby
approved for public redistribution. No new license grant has been authored as legal fact.

## Verification and rollout

First freeze baseline build/source identities, functional tests, exports and representative
workloads. Measure runtime code changes in paired runs on the same host, including codec ratio,
encode/decode/stream latency and memory; engine frame/physics/terrain/load/startup metrics.
Stripping debug information should preserve executable sections byte-for-byte; prove that and
still smoke-test the packaged binary. LTO/obfuscation/security flags are not automatically
accepted: preserve performance evidence and require approval for measured regressions.

Release candidates must fail if they lack the export audit, debug matching, integrity, consumer,
license or required platform evidence. Sign only in the existing approved release environment.
Do not replace a runtime loaded by an application. Install immutable versions and select an
explicit compatible lock; rollback selects the previous verified lock and preserves user data.

Actual public download and signature/registry verification are pending and require publication
under approved terms. Local archives and workflow files must not be reported as published proof.

## Implemented tooling checkpoint

The package-owned `native-sdk` tooling now audits an exact ELF export list and
creates separate runtime/private-debug artifacts while requiring matching build IDs
and unchanged allocated section addresses, sizes and bytes. Detached publisher
signatures are verified against an independently supplied public key before checking
the complete, bounded artifact file inventory. Tests cover a real compiled shared
library, runtime invocation after stripping, symbol mismatch, missing diagnostics,
signed metadata, byte budgets, unlisted files, symlinks and tampering.

These are Linux tooling fixtures, not hardened product release qualification.
ForgePipe's compatibility SDK still exposes Qt/C++ interfaces; its stable protected
ABI, real product debug retention, authenticated feed integration, macOS/Windows
lanes and paired workload performance evidence remain open. Nucleon's private
release candidate now has fourteen SDK lanes, eleven package consumers and a
verified durable archive for its four debug files. All eleven tested packages and
receipts are also retained in a private candidate release linked to that archive.
Its public registry release remains unpublished; distribution and name approval
flags remain false, and no license grant was changed.

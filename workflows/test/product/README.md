# Local host product journey

This Linux/deb-only workflow builds a PipeLang consumer with an explicitly selected
installed SDK and its immutable `PipeLang.Testing` source library. Dockpipe owns
preparation, build containment, fixture execution and proof preservation. The
source harness owns stable case registration, expectations, phase sequencing,
deadline choices, graceful interrupt requests and cleanup outcomes.

The first case runs the fixture with `fail`, verifies output merging, fail-fast
behavior, product finally cleanup and complete child reaping, then runs `success`
with a fresh correlation against the same owned project. The second case runs
`hold` in a separate owned project, observes actual consumer readiness, starts a
short monotonic source deadline, requests a graceful CLI interrupt, and verifies
terminal failure, skipped downstream work and product cleanup. Native force
disposal fails the case; native code never removes the fixture resource marker.

Supply these operator-selected environment values when invoking
`workflows/test/product/config.yml` through Dockpipe:

| Variable | Required selection |
| --- | --- |
| `PRODUCT_DOCKPIPE_BIN` | Absolute installed or built Dockpipe executable under test |
| `PIPELANG_SDK_PREFIX` | Absolute installed, versioned and qualified PipeLang SDK |
| `PIPELANG_SDK_MANIFEST_SHA256` | Expected `share/pipelang-model/sdk-manifest.json` SHA256 |
| `PIPELANG_TESTING_LIBRARY_SHA256` | Expected installed testing `library.json` SHA256 |
| `PIPELANG_CONTAINED_BIN` | Exact qualified `pipelang-contained` executable |
| `PIPELANG_CONTAINMENT_PROBE` | Matching qualified containment probe |
| `PIPELANG_CONTAINMENT_ADMISSION` | Prior qualified containment admission receipt |
| `PIPELANG_CONTAINMENT_ADMISSION_SHA256` | Expected admission receipt SHA256 |
| `PRODUCT_SANITIZERS` | `OFF` for normal or `ON` for a separate complete sanitizer proof |
| `PRODUCT_CMAKE`, `PRODUCT_CXX` | Optional absolute installed tool selections |
| `PIPELANG_CXX_RESOURCE_PROFILE` | Optional installed SDK compiler resource profile; defaults to `system` |

The compiler must support awaited source finally, independent authored body and
cleanup tokens, source-interface async dispatch and sound empty interface target
sets. A compiler refusal must be resolved and qualified in PipeLang before this
consumer is adapted. There is no alternate driver or dummy implementation.

Each workflow invocation allocates a fresh artifact-scoped proof with exactly two
owned projects and private XDG state/cache roots. Run normal and sanitizer proofs
as separate workflow invocations. The selected containment runner retains its
unchanged physical caps: aggregate job 2 GiB memory, 1536 MiB high, no swap and 384
PIDs; each unit 1 GiB memory, 700 MiB high, 800 MiB proactive stop, no swap and 128
PIDs; an 8 GiB proof estate; sequential compilation and execution.

`report.json`, identity hashes, generated source/build products and containment
receipts remain in the fresh proof directory returned through `product.env`.
The workflow finally action writes a preservation receipt. It deletes no product
evidence or resources. An outer containment stop is an infrastructure failure,
not evidence of successful source cleanup or product finally behavior.

This family establishes no VM/container/remote/GUI or Windows/macOS acceptance.
It does not change SDK defaults, packages, release selection or publication state.

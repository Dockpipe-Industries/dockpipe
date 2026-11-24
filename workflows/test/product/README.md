# Local host product journey

This Linux/deb-only workflow builds a PipeLang consumer with an explicitly selected
installed SDK, its immutable `PipeLang.Testing` source library, and a materialized
`Dockpipe.SDK` bundle from the explicitly selected Dockpipe executable. Dockpipe owns
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

## Windows preparation

The source cases receive outside-proof file and directory probes through typed
inputs. Linux preparation retains `/proc/version` and `/proc`; platform ports
must supply valid absolute paths for their operating system. The source case IDs,
assertions and phase budgets are unchanged.

The fixture's `assets/fixtures/host-workflow/windows.yml` selects native PowerShell
scripts with the same output keys, exit codes and LF receipt bytes as the Linux
fixture. Local PowerShell component checks do not qualify Windows execution.
The complete journey remains Linux-only until Windows process ownership,
file confinement, graceful interruption and native containment are qualified.

The harness's source `ProcessPort` builds typed SDK requests and retains journey
selection and variable policy. The SDK supplies general CLI invocation, bounded
capture, scope observation and child custody. Confined file observations remain
independent. The native product-specific process implementation has been removed.

A deterministic failed-start control also runs the actual source cases against
an injected transport that reports launch failure after retaining a newer
operation. It requires primary failure plus successful cleanup of that newer
handle, without forced disposal. The control creates no OS processes; its
observations remain separate from the real journey's five process receipts.

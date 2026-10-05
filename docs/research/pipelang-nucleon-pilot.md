# Nucleon SDK / PipeLang pilot — 2026-09-15

Status: local SDK integration and bounded pilot complete. Changes are uncommitted; no public release or new full-language acceptance.

Nucleon ran through its private C++ shared library on real PipeLang-generated executables. The consumer contains only the optional SDK client, representation adapter and tests. No proprietary implementation, models or SDK binary were added to DockPipe.

## Result

The same 200 executables occupied **1,216,832,232 bytes** with identity records. Nucleon’s complete representation occupied **616,780,429 bytes**, including the SDK, records, recipes and support metadata: **49.31% smaller**. All 200 current-source logical cases passed in every final arm.

| Path | Inclusive representation bytes | Warm complete-job wall | Warm CPU seconds | Observed peak MiB |
| --- | ---: | ---: | ---: | ---: |
| Ordinary executables | 1,216,832,232 | 187.046 s | 227.450 | 481.91 |
| Nucleon whole-file SDK | 616,780,429 | 195.129 s | 236.066 | 389.10 |
| Existing structural/Zstd with shared references | 191,935,246 | 218.060 s | 266.596 | 303.43 |

Nucleon’s measured warm job was **4.32% slower than ordinary** and **10.52% faster than the existing compressed path**. This is one serial timing pair per final condition, not a precise general speed guarantee. The earlier ordinary control was 189.890 s. Memory peaks are observed cgroup values, not a proven memory improvement.

The existing path stores much less data and prepares faster. Its structural transforms and cross-artifact references exploit this related executable set. Nucleon compresses each complete executable independently. This compares usable workflow alternatives, not raw codecs under identical preprocessing or dictionary access.

## Preparation and reconstruction costs

| Path | Cold complete-job wall | Preparation events, total | Warm reconstruction events, total |
| --- | ---: | ---: | ---: |
| Nucleon | 848.245 s | 200 / 566.229 s | 200 / 5.781 s |
| Structural/Zstd | 329.955 s | 200 / 96.370 s | 200 / 27.206 s |

Nucleon warm reconstruction averaged **28.91 ms per executable**. This event includes payload reading/hash verification, SDK parsing/allocation/decode/CRC/copies and output SHA/sealing. It excludes server startup, descriptor transfer and the Go receiver’s independent verification, which are included in whole-job timing. It is not the historical decode-kernel GB/s measurement. Whole-job timing also includes fixture/oracle execution, containment and receipt work; it is not bare application-start latency.

Both warm compressed runs performed zero preparations. Nucleon cold preparation exceeded the 25-second batch deadline in eight groups of five; all affected cases passed individual retries. Its cold total includes those failures/retries, and preparation/reconstruction event totals include failed attempts. Structural/Zstd cold and all final warm arms had no failed attempts. Use the default batch size of one for initial preparation; use bounded larger batches only once warm.

## Storage accounting

- Raw executables: 1,216,792,632 bytes; identity records: 39,600 bytes.
- Nucleon framed payloads: 616,547,454 bytes; SDK binary including frozen models: 124,096 bytes.
- Structural/Zstd payloads: 185,969,210 bytes; copied decoder/preparation support binaries: 5,596,055 bytes.
- Inclusive sizes include all required recipe/reference closure, records, configuration and one representation-store copy of support binaries. Neither final path used unsupported-file fallback. All referenced artifacts belong to the measured 200-key set.
- System C/C++/Python/Go runtime installations, the separately installed SDK copy, compiler/build caches, test receipts and filesystem block overhead are excluded from representation sizes. This is not a total development-directory footprint.
- **Actual freed bytes: zero.** Originals and prior research/backups remain. Savings describe replacement representation size, not disk space reclaimed.

## Coverage and method

- Host: Intel(R) Core(TM) i9-10900K CPU @ 3.70GHz; 20 allowed logical CPUs; Linux-7.1.1-76070101-generic-x86_64-with-glibc2.35. SDK: c++ (Ubuntu 11.4.0-1ubuntu1~22.04.3) 11.4.0, cmake version 3.22.1, Release `-O3 -DNDEBUG`, C++17. No SDK CPU pinning.

- Current dirty DockPipe checkout based on `1c839e3ce92591c845fc647970d453bcad6dffea`; preexisting language/harness edits were preserved. The Nucleon retained source/models were unchanged from private baseline `332f8835f4cdcdada632d4effbbe2e116411c81b`.
- Complete `TestV910NestedTerminalInitializersLayouts` family: 200 logical shape cases, one of 884 discovered top-level tests. Every final arm accepted exactly the same 200 artifact keys with fresh current Value/Trace checks and fresh native child processes. No result receipts were reused.
- Go 1.25.13, Linux x86-64, workers=1, shape batch size=5, native bundles enabled, parallel shapes disabled, no scheduling profiles. Compiler/executable caches were warm for the five final comparison arms; compressed stores were empty for their cold arms.
- Run order after the socket fix: Nucleon cold, ordinary warm, Nucleon warm, structural/Zstd cold, structural/Zstd warm. Comparisons ran serially. Detailed job commands, source/toolchain hashes and environment identities are in the receipts.
- Existing containment retained: 1 GiB/128-task child units, no swap, 800 MiB proactive stop; 2 GiB aggregate job, no swap, 384 tasks, 1800 MiB proactive stop; 512 MiB coordinator. Native test deadline 25 seconds, outer unit deadline 30 seconds. No ceilings were raised.
- All final arms reported unchanged source/toolchain, complete reconciliation and removed process trees. This is workload evidence, not a full 884-test language proof, a held-out corpus benchmark or an industry-wide win.

## SDK and boundary validation

- Private experimental C ABI 1, Linux x86-64/AVX2, 64 MiB SDK input cap. The native adapter admits nonempty artifacts up to 16 MiB and uses the ordinary fallback for oversized inputs.
- Release CTest: 3/3 passed. ASan/UBSan CTest: 3/3 passed. Empty/block boundaries, roundtrips, CRC/truncation/capacity rejection, overlap and concurrent initialization covered. Broader fuzzing and portability remain open.
- 13 adapter/planner checks passed, including corrupt identity/payload handling, sealed replay after deleting the test original, SDK identity binding, fallback and a real child-process descriptor transfer from a long temporary directory.
- Frozen wire parity: SDK encoding of Canterbury `fields.c` matched the retained wire exactly; SDK decoded that wire and the retained 8,474,240-byte X-ray input exactly. Original bytes were only the external test oracle.
- An additional real PipeLang shape (199) passed using a records-only cache with no original executable and no recompilation. The Go receiver’s valid/unsealed/wrong-byte subcases passed independently.
- Import audit preserved 15,377 historical research files and 142 retained source/model files. SDK binary SHA-256: `701a0d2e3969b0883e1717cb9b8a644dfd16e683780ba2e402e6276f327d72bb`.

## Preserved failures and fixes

- Initial empty Go cache: bootstrap exceeded the existing 30-second deadline, before codec work. Retry used the populated compiler cache and unchanged limits.
- Ordinary artifact population used batches of 25, timed out in all eight groups and then passed all 200 individual retries. Observed setup cost: 830.910 s. This is investigation/setup evidence, not the final warm baseline or a recommended cold batching configuration.
- First Nucleon attempt failed before codec invocation because nested TMPDIR paths exceeded Unix socket `sun_path`. The socket now remains in that private directory but is addressed through `/proc/<parent-pid>/fd/<directory-fd>/socket`. A long-path transport regression test passed before all five final arms.
- SDK handles unload before their sealed descriptors close, preventing a recycled `/proc/self/fd` loader path from reusing an old library handle.

## Local use and next work

Install the SDK in the private Nucleon checkout with CMake, then use `--native-representation --native-codec nucleon --nucleon-sdk /absolute/path/to/libnucleon.so` with the existing contained suite. The example in `tests/containedexec/README.md` defaults to single-case cold preparation. Ordinary execution remains the default; no hosted registry is required.

This is a working optional native-artifact test integration. Production distribution, a stable SDK contract, broad malformed-input fuzzing, portability, full-suite acceptance and public benchmark claims remain separate work. The next useful engineering measurements are encoder cost and complete SDK reconstruction overhead; preserve ratio and independent integrity/oracle checks while tuning.

## Evidence

Durable local archive: [nucleon-pipelang-20260915](/home/jamie/.codex/visualizations/2026/09/15/01a0a309-d1b8-7021-bd8e-96fc2c309059/nucleon-pipelang-20260915/README.md). The archive contains detailed measurements, footprint inventories, source/SDK identities, all successful and failed receipts, test logs and reproduction commands. `MANIFEST.json` records archived file hashes. Original absolute paths are retained; this is a forensic copy, not a relocated resumable campaign. Native executables, compressed payloads, build caches and temporary build directories remain outside this compact archive.

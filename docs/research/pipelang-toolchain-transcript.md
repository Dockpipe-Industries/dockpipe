# Sealed toolchain transcript experiment

The bounded experiment is complete and rejected: two warm comparisons were slower
and two were faster. The preset retention rule required a complete-path improvement
in every comparison. Correctness and resource acceptance passed, but the performance
result did not meet that rule. All prototype source changes were restored; the exact
experimental files and receipts remain available for review. No default behavior changed.

The [objective record](../agents/tasks/pipelang-reactive-application-language/toolchain-transcript.md)
owns scope and authority. This follows [verification overhead profiling](pipelang-verification-overhead-profile.md)
on saved `js/pipelang` at `8add7dc2cb7c755cb9d73c60c92452a75eca3a09`.

## Mechanism and preserved proof

The opt-in Linux prototype packed the exact legacy Go toolchain digest input into
one kernel-sealed memfd per suite. Each worker independently hashed all bytes using
positional reads. This removed repeated file enumeration and opens without trusting
a parent digest or skipping content hashes. The transcript contained 172,505,477 bytes
from 9,366 files. Each candidate suite independently compared the Python producer
against the direct Go walk before selected execution.

Source watches guarded the original toolchain through closing acceptance. Workers
checked runtime identity, owner credentials, nonce, seals, bounds and content digest.
Focused Go checks covered malformed, truncated, extra-byte and unsealed inputs,
concurrent reads, transport authentication and settings mismatch. Six final Python
tests covered framing, mutation and restoration, new/missing files, short writes,
symlinks, lost watches, owner lifetime and fresh restart. Suite regressions and the
real-toolchain smoke also passed. Explicit candidate failures were fail-closed.

## Matched sample result

Both lanes used the same profiling instrumentation, 48 frozen cases, two workers,
and independently admitted scheduling profiles with 19 pairs and 10 singletons.
After one population pass and two learning passes, execution order was off/on,
on/off, off/on, on/off. Times include suite preparation, admission and closing work.
The retention rule was frozen before the first comparison; results were not tuned.

| Round | Direct control (s) | Sealed transcript (s) | Candidate change |
| --- | ---: | ---: | ---: |
| 1 | 47.283 | 47.541 | +0.55% slower |
| 2 | 47.007 | 47.296 | +0.61% slower |
| 3 | 47.328 | 46.348 | -2.07% faster |
| 4 | 48.722 | 46.256 | -5.06% faster |

The means were 47.585 versus 46.860 seconds, an observed 1.52% reduction. The mixed
pairs do not establish the required consistent benefit, statistical significance,
a full-campaign gain or a whole-language speedup.

Worker toolchain content hashing averaged 9.891 summed seconds plus 1.219 seconds
of enumeration/metadata in the control, versus 8.885 summed seconds hashing the
transcript. The candidate additionally averaged 1.304 seconds preparing its shared
input and 0.818 seconds in the mandatory direct-admission unit. These overlapping
and nested measurements must not be added to elapsed suite time. Each warm candidate
still performed 24 complete transcript hashes: 4,140,131,448 logical bytes including
framing, versus 4,127,633,808 toolchain content bytes in the control. Physical I/O
counters were unavailable. Longer-lifetime amortization remains unproven.

## Independent acceptance and resources

Independent reconciliation accepted all 11 fresh passes: 48 cases, 294 ordered
audits and 294 fresh native children per pass, with exact source/fixture, artifact,
Value/Trace and native-oracle agreement with the prior accepted sample. There was
no resumed execution reuse and no warm cache miss. The checker accepted 415 contained
units, including the final live Python tests; preparation checks are additional.
It performed 7,906 artifact reads covering 5,431,778,331 logical bytes. All candidate
owners closed and stopped with no reported errors or refused requests.

The 96-GiB storage ceiling and 8-GiB reserve, 2-GiB aggregate/1536-MiB high,
512-MiB coordinator, 1-GiB unit/700-MiB high, zero swap, and existing compiler/child
limits were unchanged. The shared 172.5-MB transcript was charged within those
limits. Measurement aggregate peak was 1,418,809,344 bytes; accepted aggregate and
unit soft events, hard/OOM events and swap use were zero. Preparation, measurement
and independent acceptance jobs completed with their process trees removed.
Measurement took 652.470 seconds; preparation took 45.062 seconds and independent
acceptance 38.759 seconds. These are separate proof jobs, not total investigation time.

The post-run admitted estate held 38,169,245,978 logical and 40,185,835,520 allocated
file bytes, with sampled allocated peak 40,262,258,688 bytes and 255,833,305,088 bytes
available. This covers declared experiment, prior-profile, cache/toolchain/source
roots, not unrelated historical campaigns. After source restoration, this experiment
alone retained 616,353,241 logical and 640,999,424 allocated bytes across 7,880 files,
before final reporting metadata. Hardlinks are deduplicated; shared extents and exact
temporary peaks are not measured. Existing cache growth is included in the wider
estate, not attributed as a standalone new saving. Existing retained bytes freed: zero.

## Disposition and evidence

Three tracked prototype files were restored byte-for-byte to HEAD and four owned
prototype additions were removed from the checkout after archiving their exact bytes.
Production Go code was unchanged throughout. Engine/package boundaries were preserved;
no implementation remains in source. Only task/report documentation remains uncommitted,
alongside the prior completed profiling documents. No full campaign, promotion, install,
commit, push, publication, retained-artifact cleanup or successor selection occurred.

Evidence root:
`/home/jamie/.codex/visualizations/2026/09/13/01a09c2e-b445-7322-8a02-1d2328313bec/toolchain-transcript/`.
Key records: `frozen-design.json`, `retention-rule.json`, `runs.json`,
`independent-acceptance.json`, `experiment-job.json`, `accept-job.json`,
`post-run-storage.json`, `task-retained-storage.json`, `rejected-source/manifest.json`,
`restoration.json` and `final-acceptance.json`. The archived source explains the
historical candidate receipts; the current checkout contains the restored baseline.

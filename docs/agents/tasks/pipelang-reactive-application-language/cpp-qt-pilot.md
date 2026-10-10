# C++ / assembly / Qt pilot

Objective: `pipelang-cpp-qt-pilot-20260913`.
State: completed. Execution skill: `dorkpipe-objective-execution`.
Execution authority: approved_objective_creation. User said "I approve that" to
C++ output, compiler-generated assembly and one working Qt integration.

Own a Core-only Go-implemented C++ emitter for the bounded native pilot, independent
Go/evaluator/native tests, compiler-generated assembly equivalence, and one small Qt
Widgets host calling generated logic with properties, signals, value/error conversion,
visible output and object cleanup. Qt-specific support belongs in the pilot adapter,
never Core. Keep Go compiler/backend and all accepted v0.113.0 behavior.

Done when: frozen eight-program/160-vector corpus and 32 negative inputs pass current
frontend -> HIR -> Core -> evaluator, independent expected values/traces, Go, C++ and
assembled native execution; malformed/unsupported inputs refuse. Source/toolchain/
fixture invalidation, wrong-result/trace controls and interruption/recovery pass.
The Qt host compiles through moc, exercises real events and proves the displayed
result and cleanup. Measure matched application and verification costs with all
runtime/Qt dependencies, compile/link, RAM, retained source/object/debug/support.
Report performance and Qt integration independently: user-approved integration value
can justify keeping a correct pilot without claiming the proposed performance gates.
No full-language backend promotion follows from subset proof.

Contract: C++17, Linux x86-64 SysV, baseline CPU, -O2 -g, no LTO/PGO/stripping;
assembly via the same compiler -S then assembler/linker. Frozen numeric/control and
allocating text/record/list cases follow the canonical native-backend pilot proposal.
Only existing installed toolchains/Qt; no install authority. Original compiler/child
ceilings, 2-GiB aggregate/1536-MiB high, 512-MiB coordinator, 1-GiB unit/700-MiB high,
zero swap, 96-GiB estate/8-GiB reserve remain. Pilot candidate/control outputs target
2-GiB retained/3-GiB incremental peak including Qt adapters; installed dependencies
are included in the broader estate and separately in deployed closure. Stop before
allocation if bounded admission cannot fit; do not raise limits.

Checkpoint policy: automatic_within_objective. Handoff: user_requested_only.
Context pressure: warn_and_continue. Terminal conditions: completed, blocked,
failed_verification or cancelled. No worktree, delegation, raw Git lifecycle,
commit/push/publication, cache/evidence deletion, full campaign, direct machine-code
emitter, new CLI surface, language expansion or full app port.

Admission: clean saved `js/pipelang` at 0bf0b0b49da04823c633fdba37558787d6da39eb.
Protected stashes match prior evidence. Previous experiment documentation is committed
and preserved. Evidence root:
`<local-evidence>/2026/09/13/01a09c2e-b445-7322-8a02-1d2328313bec/cpp-qt-pilot/`.
Qt 6.2.4 CMake packages and moc are installed despite absent pkg-config entries;
contained compilation and actual GUI execution subsequently passed.


Preparation checkpoint: all 160 independent vectors pass current frontend/HIR/Core
and evaluator checks, with 16 malformed/unsupported Core refusals. Numeric/control
programs use v0.113; allocating cases retain their actual v0.12/v0.17/v0.20/v0.22
contracts. Core versions are never relabeled. Integer division in the original
proposal is unsupported by the accepted source contract and is a negative case;
checked integer add/subtract/multiply/negate are the positive arithmetic subset.
The old proposal's zero-argument record/list source exposed a pre-existing
`typecheck.go` panic at `resolvedParameters[1:]`; its original failed evidence is
preserved in `prepare-5/export.output`. No frontend repair or language expansion
has been made as part of this backend experiment.

Preparation 1–2 failed storage admission before compiler work; 3 corrected test
import aliases; 4 corrected the test helper's default language contract; 5–6
established actual supported fixtures and explicit missing-identity refusal.
Preparation 7 passed the frozen export and all Core refusals with source watches.
The first native admission attempt was stopped after unrelated system-library
hashing caused coordinator hard-limit pressure. Compiler dependency discovery and
existing coordinator reclaim policy now bound identity admission without skipping
required bytes. The next preparation was stopped before native execution to add
sealed-descriptor execution; its successful compile artifacts remain retained.
These preparations are investigation cost, not accepted timing comparisons.


Completion: retain the experimental integration backend. The
[canonical result report](../../../research/pipelang-cpp-qt-pilot.md) owns coverage,
measurements, observed Qt dependency accounting and limits. No full-language or
default-backend promotion and no performance adoption claim follows.

Accepted evidence: preparation 7; all 49 native stages; deliberate exit 19 after
42 recovery stages followed by 42 validated reuses and seven fresh completions;
independent raw-output/artifact acceptance for both campaigns; all 16 source-only
compiler probes within 5 s/128 MiB; six integrity/comparator controls; 160,000
additional larger-batch values; 26 existing receipt regression tests; real xcb Qt
button/property/queued-signal/error/ownership proof and a dependency-observation
rerun. All completed process trees were removed. The initial synthetic-regression
invocation's two metadata-induced refusals are preserved and explained in the
report. No failed preparation is promoted into performance evidence.

The original broader proposal's repeated clean/private-cache and incremental build
study was not run: this objective is retained for demonstrated Qt integration
value. Go remains the accepted backend. Existing source, compiler and resource
ceilings are unchanged. The Qt dependency observation adds 91 existing runtime
files beyond compiler/linker discovery to the final accounting snapshot; portable
Qt packaging remains unproven. Frontend zero-argument record/list panic is a
separate recorded finding, not silently repaired here.

Final checks: Go formatting, Python syntax, YAML and local links, diff whitespace,
owned postimages, unchanged HEAD/protected stashes and empty staging. Generated
code, assembly, binaries, images and raw evidence stay in the external evidence
root. Package/engine boundaries are preserved. Changes are local and uncommitted;
there is no active successor objective, commit, push, install or publication.

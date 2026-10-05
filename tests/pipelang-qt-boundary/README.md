# Qt boundary reference checks

This bounded resolver-side reference adapter implements the
[canonical integration contract](../../docs/concepts/pipelang-qt-boundary.md).
It stays in the experimental harness until a production target package is approved.
It does not change the original pilot or introduce Qt into PipeLang Core.

Configure CMake with `PIPELANG_PILOT_INPUT` set to an admitted directory containing
`generated.hpp` and `qt_binding.hpp` for the pilot N2 semantic-identity binding.
Build `pipelang-qt-boundary` through moc and execute it freshly. Qt Core's main
application event queue suffices; this proof needs no desktop, QApplication or
rendering backend. The prior pilot retains the actual xcb/Widgets proof.

Run configuration/build/execution through `tests/containedexec/job.py` and `run.py`,
including watched source/toolchain/artifact identities, complete estate accounting,
sealed executable admission and independent receipt/cleanup checks. Keep the
2-GiB aggregate/1536-MiB high, 512-MiB coordinator, 1-GiB unit/700-MiB high,
zero swap, 96-GiB estate and 8-GiB reserve. Compiler-only source probes retain
128 MiB/5 seconds. Do not launch compilers or native tests outside containment.

The test's raw direct worker call is a deliberate release-build refusal control.
Production completion delivery uses the context-bound queued connection demonstrated
next to it. The sender and worker are joined before teardown.

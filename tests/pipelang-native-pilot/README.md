# Experimental C++ / assembly / Qt pilot

This harness owns the eight-program native integration experiment described in
[the result report](../../docs/research/pipelang-cpp-qt-pilot.md). It is Linux-only
and is not part of default backend selection or a full-language acceptance suite.

`src/lib/pipelang/cppbackend` consumes validated Core only. `cpp_pilot_test.go`
exports fresh source, Core, independently checked vectors, C++/Go application
adapters and separate trace variants. Qt stays in `qt/`; its QObject bridge calls
one generated checked-arithmetic helper, exercises the actual GUI event loop and
writes a screenshot plus an ownership/event receipt. It requires an existing Qt 6
Widgets development installation and an available xcb desktop.

## Running the native proof

Use the repository's existing `tests/containedexec/job.py` aggregate envelope and
`run.py` units. Do not launch `worker.py` directly. Keep the accepted resource
limits, pinned offline Go toolchain and complete storage accounting roots. The
roots must contain all task generations, private/shared caches, compiler sources,
resolved compiler/linker inputs and runtime dependencies; keep failed attempts.

Inside a contained unit, compile `./src/lib/pipelang` with `go test -c`, then run
that test binary with `PIPELANG_CPP_PILOT_OUTPUT` set to a fresh output directory
and `-test.run '^TestCPPPilot(Export|CoreRefusals)$'`. The directory is the frozen
`--prepared` corpus. Export validates all 160 vectors independently before output.

Launch the coordinator through `job.py`, supplying `driver.py` with these options:

- `--prepared`: absolute frozen corpus directory.
- `--output`: fresh campaign directory, or the same directory for resume.
- `--cache`: absolute shared Go cache included in accounting.
- `--go`: absolute pinned Go 1.25.13 executable.
- `--roots`: JSON array of complete canonical accounting roots.
- `--repetitions 10`: balanced-order fresh application and verification runs.
- `--interrupt-after 42`: optional deliberate interruption; exits 19 after receipt publication.
- `--mode resume`: admit completed work after the prior job tree is gone.

Each invocation rebuilds the exporter, repeats frontend/HIR/Core/evaluator/oracle
proof and compares exact fresh output against the frozen corpus. Source, harness,
fixtures, toolchain content, settings and include-search directories are guarded.
Reuse requires matching identities and checksummed artifacts. Every native launch
executes a hashed, write-sealed private memfd and closes its descriptors.

Generated `program.s` and `program.o` retain the compiler-generated assembly lane.
C++ flags are `-std=c++17 -O2 -g -march=x86-64 -mtune=generic`. No stripping or LTO
is enabled. The reported short-run wall time includes executable admission and
adapter I/O; it is not pure generated-code execution time. Go's reflection adapter
and the typed C++ adapter also prevent attributing their whole timing difference
to the backend.

## Qt input and artifacts

Configure `qt/CMakeLists.txt` with `PIPELANG_PILOT_INPUT` pointing at a directory
containing the N2 `generated.hpp` and a `qt_binding.hpp` that exposes
`pilot_calculate(int64_t, int64_t)` by calling the generated `Forward` binding.
Use the emitter's returned semantic-identity binding, not source-name guessing.
CMake AUTOMOC builds `pipelang-qt-pilot`. Execute it in a contained unit on the
actual desktop, passing a fresh artifact directory; retain `qt-result.json` and
`qt-pilot.png`. No library installation, public ABI or portable Qt deployment
manifest follows from this experiment.

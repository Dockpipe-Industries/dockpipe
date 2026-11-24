# Native workflow-control fixture

Build this fixture explicitly through `dockpipe workflow build-control` using a
selected installed SDK and its manifest SHA256. Run with an absolute task-owned
`TRACE` path. The trace proves conditional execution, two typed loop arguments,
valid identifiers that used to collide with generated locals, and core `finally`
cleanup. The controller is a compiled PipeLang program, not a shell expression.

The default trace is `test`, `build:alpha`, `build:beta`,
`hygiene:literal:argument:true`, `cleanup`. `--var RUN_TESTS=false` skips `test`.
`--var FAIL_TEST=true` records `test` then `cleanup` and fails. For cancellation,
set `--var HOLD=true`, wait for `ready`, then interrupt the CLI; it must fail,
skip subsequent steps, and record `cleanup`.

A normal build and an instrumented `--sanitizers` build use the same fixture.
LeakSanitizer must execute in a host lane that supports it; a tracing sandbox's
LeakSanitizer fatal error is not a passing sanitizer result. Windows VM and
physical Windows/macOS qualification are recorded separately below.

## Windows qualification fixture

`windows.yml` declares the same PipeLang program and typed calls with native
PowerShell scripts. Select an installed Windows SDK and its manifest SHA256,
an absolute CMake path, and an absolute compiler path in the compiler's prepared
developer environment. Build `windows.yml` explicitly with `workflow build-control`,
then run that same file with an absolute, fresh `TRACE` path for each case above.
The controller must be built on the target; the Linux controller is not portable
to Windows.

The builder selects Release explicitly and admits the executable from a fixed
output directory, including with multi-config generators. The complete sanitizer
profile requires a GNU-compatible compiler supporting both ASan and UBSan;
MSVC is rejected for that profile.

The admitted Windows VM passes original PowerShell 5 default/skip/failure traces,
interruption after `ready` with independent cleanup and no later steps, and
fresh-project prebuilt-package relocation without build tools in runtime PATH.
`assets/harness` owns these five cases in PipeLang.Testing.Async with the default
source evaluator and unchanged 8000ms body/5000ms independent cleanup deadlines.
Native adapters supply the real SDK transport, confined trace reads and clock.

The native host-owner regressions also pass genuine EOF, cancellation after root
exit, active cancellation, and redirected-child survival after launcher exits 0
and 17. The survival checks retain and validate the child's process handle before
allowing the launcher to exit. Normal early-return GUI/helper semantics remain
supported. Physical Windows 11 and macOS remain unverified; this VM result does
not establish broad GUI/product or IDE qualification.

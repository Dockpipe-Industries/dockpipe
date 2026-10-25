# Native desktop qualification

Run this workflow on the matching macOS or Windows host through the checksum-locked
repository runner. `--jobs 2` bounds Go and CMake build parallelism. `--check` runs
the host-dispatch, deadline and actual CTest result-contract tests without native desktop builds.

The owning assets preserve the ordered native gates: runtime Git tests and CLI,
matching Qt and attested Browser SDK, macOS symbol/DMG fixtures and deployed app
audit, or Windows standard-user launcher, terminal transport, native embedding,
and complete desktop suite. Every CTest gate rejects an empty selection. Windows
loader/import diagnostics remain advisory; the embedding test remains mandatory.
Fresh JUnit reports must prove an exact non-skipped embedding pass before staging;
CTest success alone cannot satisfy that gate.
macOS offscreen dashboard tests retain software rendering settings.

GitHub Actions provide host setup, secret references and the pinned Dockpipe
launch adapter. Actual native execution remains required on both macOS runners
and Windows; dispatch tests or Linux inspection do not replace those results.

# TASK-004 Qt App And VS Code Extension Compile Performance

## Current State

- The Pipeon build already fingerprints its inputs and skips language-support VSIX packaging,
  Pipeon npm build/smoke work, and Pipeon VSIX packaging when the corresponding outputs are current.
- The Dockpipe GUI uses a persistent CMake build directory and normal incremental compiler outputs.
- The task does not yet have measured hot-path budgets or evidence identifying which remaining
  invalidations make common edit/test cycles unnecessarily expensive.

## Still Open

- Measure Dockpipe GUI configure/compile/link and Dockpipe Language Support VSIX
  paths separately, then record reproducible no-op and one-file-change baselines.
  PipeLang owns its language extension. Pipeon product performance work remains
  paused in its owning repository; it is not an active core build target.
- Identify the remaining unnecessary invalidations and add only the focused cache or narrower build
  surface supported by that evidence.
- Add validation proving relevant source changes rebuild required outputs while repeated no-op builds
  skip them.

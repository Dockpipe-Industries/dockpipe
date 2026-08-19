# TASK-033 macOS DockPipe Platform Support And Qualification

## Goal

Make DockPipe work and remain qualified on selected macOS releases and Apple Silicon, with Intel
support retained only if explicitly selected and proven.

## Scope

- Select exact macOS release/architecture baselines and lifecycle policy.
- Implement and validate installation/upgrade, codesigning/quarantine expectations, shell and PATH,
  APFS permissions and durability, process trees, CLI, packages/resolvers/workflows, runtimes,
  applications, Docker/VM integrations, and diagnostics.
- Keep TASK-013 ownership of App Server storage/session evidence and TASK-023 ownership of any future
  host-sandbox decision; consume their evidence without inferring platform-wide support.
- Distinguish native arm64, Rosetta/x86_64, virtualized macOS, and cross-compiled artifacts.

## Acceptance Criteria

- Every advertised macOS release/architecture combination passes reproducible native evidence for
  all claimed surfaces, including APFS, permissions, process teardown, and application integration.
- Codesigning, notarization, quarantine, developer-tool, and container/VM prerequisites are explicit.
- Intel support is independently retained, deprecated, or excluded; TASK-025 records the decision.

## First Bounded Slice

Inventory current Darwin builds, installation paths, Apple Silicon/Intel assumptions, APFS and
process gaps, available CI/native hardware, and TASK-013 evidence. Do not run paid macOS CI or VMs,
install software, modify source, publish/notarize artifacts, or claim support.

# Validation Commands

Read before final reporting or when choosing checks.

## Core

```bash
python3 workflows/shared/launch.py workflows/ci/core/config.yml
python3 workflows/shared/launch.py workflows/ci/repository/config.yml
python3 workflows/shared/launch.py workflows/build/cli/config.yml -- --output /absolute/path/to/dockpipe
```

The launcher verifies the separately released bootstrap CLI against
`workflows/shared/bootstrap.lock.json`. Set `DOCKPIPE_BOOTSTRAP_ARCHIVE` to an
already downloaded pinned archive for offline execution. Make targets are thin
aliases for these workflows.

Use writable task-owned cache/state directories when an isolated check supports them.
If a required host capability is unavailable, follow the
[sandbox guide](../runtime/codex-sandbox-sessions.md) for a narrow reviewed operation;
a cache permission failure alone does not require running the whole suite on the host.

## Workflow

```bash
./src/bin/dockpipe workflow validate <config.yml>
./src/bin/dockpipe --workflow <name> --
```

## Package

Run these checks in the external package checkout with absolute `DOCKPIPE_BIN`,
`PACKAGE_REPOSITORY` and `PACKAGE_SOURCE` selections; see [package authoring](../packages/package-authoring.md).

```bash
"$DOCKPIPE_BIN" package test --workdir "$PACKAGE_REPOSITORY" --only <package>
"$DOCKPIPE_BIN" package compile workflows --workdir "$PACKAGE_REPOSITORY" --from "$PACKAGE_SOURCE" --force
"$DOCKPIPE_BIN" package compile resolvers --workdir "$PACKAGE_REPOSITORY" --from "$PACKAGE_SOURCE" --force
```

## DorkPipe Skills

Install the external DorkPipe package before invoking its skill renderer. It is not bundled source.

```bash
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --list
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target codex --dry-run
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target claude --output /tmp/dorkpipe-claude-skills --dry-run
```

## Search

```bash
rg -n "<old-name>|<old-script>" docs src workflows .github -S
git status --short
git diff --stat
```

## Report

Always say which checks ran, which failed, and which were skipped.

## Windows CI Notes

- When debugging Windows CI or `ci-emulate` failures around managed workspace Docker volumes,
  do not assume the runner checkout has a standalone `.git/` directory at the workspace root.
- In `src/lib/infrastructure/docker.go`, that checkout-shape difference can legitimately switch
  managed workspace bootstrap between:
  - git-aware seed path: `session.volume.seed`
  - tar sync path: `session.volume.sync_in`
- Prefer topology-agnostic assertions unless the test is explicitly validating one bootstrap mode.

## Dogfooding

This repo runs the same released binary and declarative workflows users get. Dogfooding is a quality
bar, not a special CLI primitive. After the developer CLI build workflow, run dogfood workflows from the repo root with:

```bash
./src/bin/dockpipe --workflow <name> --
```

Omit `--workdir .` when the current directory is already the project root.

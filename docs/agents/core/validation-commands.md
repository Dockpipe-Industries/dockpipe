# Validation Commands

Read before choosing checks or reporting proof. Match checks to the changed behavior;
docs-only work needs link/path and diff checks, not a build or full test suite.

## Core

Use the repository workflow to preserve its bootstrap and test-environment setup.
Arguments after `--` are forwarded to `go test`; begin with affected packages and
a relevant test expression when appropriate:

```bash
python3 workflows/shared/launch.py workflows/ci/core/config.yml -- ./src/lib/application/... -run '^TestName$' -count=1
```

Replace the package/test selection with real affected coverage. For a required full
suite, repository checks or CLI build, use the corresponding workflow:

```bash
python3 workflows/shared/launch.py workflows/ci/core/config.yml
python3 workflows/shared/launch.py workflows/ci/repository/config.yml
python3 workflows/shared/launch.py workflows/build/cli/config.yml -- --output /absolute/path/to/dockpipe
```

The bootstrap script verifies the separately released bootstrap CLI against
`workflows/shared/bootstrap.lock.json`. Set `DOCKPIPE_BOOTSTRAP_ARCHIVE` to an
already downloaded pinned archive for offline execution. Make targets are thin
aliases for these workflows.

Use writable task-owned cache/state directories when an isolated check supports them.
If a required host capability is unavailable, follow the
[sandbox guide](../runtime/codex-sandbox-sessions.md) for a narrow reviewed operation;
a cache permission failure alone does not require running the whole suite on the host.

## Workflow

Select an installed CLI or an existing build explicitly as absolute `DOCKPIPE_BIN`.
The build workflow requires `--output`; do not assume `src/bin/dockpipe` exists.
Validate authored examples in a task-owned temporary project. Execute only examples
whose effects and prerequisites are within the current authorization.

```bash
"$DOCKPIPE_BIN" workflow validate <config.yml>
"$DOCKPIPE_BIN" --workflow <name> --
```

Schema validation does not prove dependency availability or runtime/platform behavior.

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
"$DOCKPIPE_BIN" --package dorkpipe --workflow skills.render -- --list
"$DOCKPIPE_BIN" --package dorkpipe --workflow skills.render -- --target codex --dry-run
"$DOCKPIPE_BIN" --package dorkpipe --workflow skills.render -- --target claude --output /tmp/dorkpipe-claude-skills --dry-run
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

## Desktop

Use the [desktop contributor instructions](../../../src/dockpipe-app/README.md) for
SDK inputs and the build workflow. Run already-configured native tests through:

```bash
python3 workflows/shared/launch.py workflows/ci/desktop-native/config.yml -- --build /absolute/build/directory --regex '<affected-test-expression>'
```

Local/offscreen tests do not establish Windows, macOS or live embedding acceptance.
Do not broaden to a cross-platform build matrix unless the task requires it.

## Dogfooding

Run an existing repository workflow through the bootstrap script, or use the
explicitly selected built CLI from the repository root. Dogfooding uses the same
runtime contract as consumer workflows; it does not permit checkout-only engine
behavior. Building or running these workflows does not authorize publication.

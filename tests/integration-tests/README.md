# Integration tests

Tests that run dockpipe with Docker: templates, actions, data volume, mounts, env, detach, and the full feature set using the retained generic base-dev image. Require Docker to be installed and runnable.

**Run from repo root:**

```bash
bash tests/integration-tests/run.sh
```

Or run a single test:

```bash
bash tests/integration-tests/test_mount_and_env.sh
bash tests/integration-tests/test_commit_action_git_repo.sh
bash tests/integration-tests/test_action_resolution.sh
# ... etc
```

| Test | What it does |
|------|----------------|
| `test_commit_action_git_repo.sh` | Temp git repo, commit-worktree action, command adds a file; asserts action commits with given message. |
| `test_action_resolution.sh` | Run from a different cwd with `--action scripts/commit-worktree.sh`; asserts relative path resolves to bundled script and commit works. |
| `test_action_init.sh` | `dockpipe action init my-action.sh` creates boilerplate; `init my-commit.sh --from commit-worktree` (and `--from print-summary`) clones bundled actions; asserts files exist and content matches. |
| `test_mount_and_env.sh` | `--mount` binds a file into the container; `--env` passes a var; asserts content and value in container. |
| `test_detach.sh` | `-d` runs container in background; stdout is container ID (hex); optional cleanup. |
| `test_default_template.sh` | No `--template`: default (base-dev) image runs `echo hello`; asserts output. |
| `test_print_summary_action.sh` | Temp git repo, print-summary action, command creates uncommitted file; asserts stderr contains summary and "Uncommitted changes". |
| `test_export_patch_action.sh` | Temp git repo, export-patch action, command creates file; asserts `dockpipe.patch` exists and contains the diff. |
| `test_repo_worktree_local_remote.sh` | Creates a local bare Git remote (`file://`), runs `--repo/--branch` with `--data-dir`, and asserts host worktree creation/reuse without credentials. |

These are separate from `tests/unit-tests/` (unit and smoke tests). The main suite is `bash tests/run_tests.sh`; use `bash tests/integration-tests/run.sh` when you want to validate the generic runtime flow with Docker and the base-dev image.

Package-owned Cursor session guards and agent image/tooling tests live in
[dockpipe-packages](https://github.com/Dockpipe-Industries/dockpipe-packages/tree/master/packages).
Run their owning package hooks with a pinned installed Dockpipe CLI. Agent image
integration checks require Docker but make no provider API calls. Generic core
fixtures never skip just because optional package source is absent.

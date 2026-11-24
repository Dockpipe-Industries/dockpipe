# Generated project Git fixture

This producer is opt-in and has no CI registration. Its Go overlay adds a broker
test to the owning `workspacegit` test package without modifying `src/`. The
existing test helpers initialize and seed one `testing.T.TempDir` repository.
After readiness, the broker only observes final Git objects/index state and
waits for a nonce-bound stop request. The pinned installed CLI performs the
actual selected-file commit and stale-review refusal through `git --request`.

Invoke `config.yml` through an explicitly pinned Dockpipe CLI with an absolute
workflow path and a new private `--workdir`. Forward `--go`, `--go-sha256`,
`--git`, `--git-sha256` and fresh absolute `--output` arguments. The script runs
Go from its owning source directory, with private HOME, GOCACHE, TMPDIR and
GOTMPDIR, two build jobs, offline module resolution and before/after input
inventories. It does not create a repository while building the broker.

The `drivers/git_workspace.py` lane selects the CLI artifact and exact producer
manifest. Every repository is private and generated; system/global Git config,
templates and interactive authentication are excluded. The fixture deliberately
keeps unrelated staged and working versions plus tracked and untracked drafts.
Public API responses and an independent broker oracle verify only the selected
file entered HEAD. A changed draft invalidates a previously reviewed snapshot;
the normal refusal must preserve HEAD and every current file. Normal broker exit
must remove the test repository, and all observed kernel identities must close.

For cancellation, an equal-size edit in the generated repository makes Git
read a tracked file through a process-scoped clean filter. The driver verifies
the exact Git and filter process identities before sending SIGTERM to Dockpipe.
It requires Dockpipe's cancellation result, disappearance of the observed
processes, one filter start, and the same public HEAD, index and worktree
snapshot before and after cancellation. The lane qualifies local Linux portable
execution with a generated repository and a pinned Git executable.

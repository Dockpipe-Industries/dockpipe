# Git Planet: local Working Changes

Cockpit's Git planet follows the active local project. **Working Changes** is the
primary entrance: it works with no remote, account, push, pull request, or initial
commit. Choose **Review all changes**, walk with WASD or the controller's left
stick, and look with right-drag or the right stick. Select a wall with the mouse
or controller A to inspect its code. Page Up / Page Down and the searchable file
list jump directly between files. The end of each section continues the route.

Each file has opposing original and modified code walls. Nearby code loads on
demand. Four chambers are present at once, so a large change set does not create
thousands of meshes or textures. Folder groups influence chamber placement;
review, stale, and conflict states control completion strips. These indicators
are review state, not a risk score.

## Correct versions and practical review

The comparison selector explicitly distinguishes:

| Comparison | Original | Modified |
| --- | --- | --- |
| Unstaged | Index | Saved working tree |
| Staged | HEAD | Index |
| Combined | HEAD | Saved working tree |

A file with both staged and unstaged changes requires both comparisons to be
marked reviewed. Marking only the combined result cannot hide the index version.
New files have an absent original side. Deletions and rename metadata remain
visible; conflicts and binary changes are labeled explicitly. A metadata-only
change is not presented as a fabricated text hunk.

Selecting a wall opens native, aligned, selectable source panes in the same main
screen, with syntax highlighting, actual line numbers, synchronized scrolling,
and hunk navigation. Context can expand from 3 to 20 or 1000 lines. **Return to
space** preserves the chamber position and look. **Open editor** uses Dockpipe's
existing project editor and its save/conflict checks. Editor saves do not stage
files. Refresh after editing to load the new comparison.

Review marks bind to content hashes, index entries, modes and HEAD, not timestamps
alone. Refresh flags marks stale when those identities change. The most recent
file/hunk and local notes persist on this machine. Notes are not remote review
comments. The decision view lists reviewed, remaining, stale, conflicted and
staged files.

**Stage file** and **Unstage file** use the shared project Git service. Discard
requires confirmation and restores an ordinary tracked working file from the
index; it preserves staged changes. It does not delete untracked files or discard
renames. **Commit staged changes** requires a message and confirmation and commits
exactly the current index. Select that set by staging/unstaging first. It never
silently adds unstaged content. A changed snapshot blocks writes until refreshed.
Cancellation or failure asks for refresh rather than an automatic retry.

## Architecture and bounds

`GitReviewModel` describes files and aligned hunks independently of presentation.
`GitReviewProgress` owns user-local revision-bound review state. `GitReviewWorld`
translates it into the existing semantic graph, spatial plan and world compiler.
`GitReviewWidget` coordinates navigation and capabilities. The shared renderer has
no Git command or provider logic. All world code stays in the optional package.

The host's `ProjectGitClient` is also used by conventional Project Git. It calls
`dockpipe git --request`; the generic runtime owns snapshot reads, comparisons and
index operations. Repository-controlled strings are plain text. Diff commands
disable external diff/text conversion and use literal path arguments.

The local snapshot contract is 2048 changed paths, 8 MiB per Git command output,
and 256 MiB of streamed working-content hashing per snapshot. Exceeding a contract
fails explicitly. Focused code allows a 1 MiB working file, 2 MiB patch, 20,000
rows and 16,384 characters per line; larger content gets an explicit limitation
instead of a silently incomplete comparison. Code textures preview at most 128
rows, with 240 characters per row, and point to the full focused hunk. Long lines
wrap within the preview. Overflowing text pauses for three seconds, scrolls at
10 pixels per second, pauses at the bottom, then loops. Headers and footers stay
fixed; text that fits stays still. The current file's two previews cache at most
two 512×4096 text rasters (16 MiB total); taller content is labelled as an excerpt.
Animation only crops those cached pixels at up to 10 updates per second, without
re-running syntax highlighting. A section retains at most nine 512×320
code/navigation textures. The scene stops movement/rendering when hidden; Git
commands run asynchronously with cancellation and the existing two-minute limit.
Local progress retains at most sixteen repository sessions and 2048 file records
per session; notes allow 4096 characters per record.

## Current limits and next steps

This is the working-tree vertical slice. Branch/upstream information and recent
commit parent relationships are available as a read-only hub catalog, capped at
128 branches and 64 commits. Branch/commit comparison routes, branch mutation,
remote pull-request discovery, CI, remote comments, approvals and merges are not
implemented. The current project Git service has no PR provider; the PR district
states that explicitly. The common model reserves source kinds so a future
provider can feed the same aligned-hunk and spatial construction components.

Hunk staging, an in-world merge-conflict editor, semantic symbol search and
provider-specific review operations remain future work. Conflict resolution and
unsupported/large content use conventional tools. The focused viewer is a diff,
not an entire-file IDE. Unsaved external editor buffers are not read.

## Validation

Build the matching CLI with `make build-core`, then build the package against the
matching launcher SDK. `ctest -R '^git-review$'` runs an isolated real local Git
repository with fifteen changed files. It exercises opposing-wall activation,
aligned panes, staged/unstaged separation, every-file jumps, staging, review
progress, local notes, stale invalidation, deterministic bounded sections and
hidden-scene quiescence. It never mutates the user's checkout.

The Go `TestWorkspaceGit*` fixtures cover exact-index commits, untracked/unborn
repositories, deleted and renamed paths, binary versions, conflicts, limits,
cancellation, stale snapshots and preservation of unselected changes. Existing
Project Git, source editor, flight and spatial-web tests cover shared regressions.

GPU viewport initialization waits until the review scene is visible. Time spent
on the Git hub must not trigger a false graphics failure and software fallback.
Walking across a section boundary retains held keyboard/controller input; loading
a comparison updates selection without rebuilding the full file sidebar.
`DOCKPIPE_GIT_STRESS=1 cockpit-test --git-review` profiles fullscreen traversal
with 2048 changed files in a disposable repository, using `DOCKPIPE_BIN` for the
matching local CLI. It reports draw times separately from frame scheduling.

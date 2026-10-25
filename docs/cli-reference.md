# CLI reference

Use `dockpipe -- <command>` for a one-off command or
`dockpipe --workflow <name> --` for a workflow you created or installed.

**Run → isolate → act.** Overrides use the same names as workflow YAML. Precedence: **CLI** > config > environment.

For most users, the main knobs are:

- **`--workflow`** — which workflow to run
- **`--runtime`** — where it runs
- **`--resolver`** — which tool/profile it uses

Treat **`--isolate`** as the low-level image/template override, not the main way to think about workflow selection.

**Prerequisites:** native host steps need Bash; container workflows need a reachable
engine. Git is needed for Git operations. Flatpak supplies its shell/container
clients and uses explicit host integrations where required. See [installation](install.md).

**Workflow YAML (`config.yml`):** single-command layout (`run` / `isolate` / `act`) or multi-step **`steps:`** (ordering, **`outputs:`**, explicit **`group.mode: async`** for parallel work). In step mode, top-level `run` / `act` are not used. Full reference: **[workflows/workflow-yaml.md](workflows/workflow-yaml.md)**.

**Main subcommands:** `dockpipe init`, `dockpipe build|clean|rebuild`, `dockpipe package`, `dockpipe workflow validate|list|test`, `dockpipe test`, `dockpipe doctor`, `dockpipe windows setup|doctor`, and optional maintainer/publishing commands such as `dockpipe release upload`, and `dockpipe pipelang ...`.

If you just want the common path:

- `dockpipe --workflow <name> -- ...` runs a workflow
- `dockpipe init` creates a starter project/workflow layout
- `dockpipe workflow validate <config.yml>` checks authored YAML
- `dockpipe test` runs package-owned tests plus workflow-local tests
- `dockpipe doctor` checks host prerequisites and installed resources

Use an installed CLI from your own project folder. For Flatpak, replace `dockpipe`
with `flatpak run --command=dockpipe com.dockpipe.Dockpipe`.

The detailed sections include optional package-author and self-hosted publishing
commands. They are not steps required to install or use Dockpipe.

## Remote workers

`dockpipe remote` provides provider-neutral setup, pairing, outbound workers, explicit workflow
submission (including source/assets/package delivery with `--workflow-file`), cancellation, and result download.
Workers explicitly opt in with `remote pair --allow-delivery`; existing profiles remain supported. See [Remote nodes](runtime/remote-nodes.md) for the
Cloudflare resolver, local profiles, credentials, recovery, and current qualification limits.

## Project Git

The desktop **Git** page operates on the repository containing the selected project
folder. Its CLI adapter is `dockpipe git --request <file.json>`. A read request is
`{"workdir":"/path/to/project","action":"status"}`. The response includes the root,
branch, HEAD, changed paths, review text and a `snapshot` fingerprint.

Human-requested write actions supply that fingerprint as `snapshot`: `commit` also
requires `paths` (exact selected paths) and `message`; `pull` requires a clean tree
and uses fast-forward only; `push` sends the current branch to the same branch on
origin without force. Commit includes the selected files' saved contents and keeps
unselected staged changes. A changed review is rejected until refreshed. A failed
commit can leave selected files staged; refresh before retrying. Git identity and
credentials use the existing local Git configuration; interactive terminal prompts
are disabled. Review output is limited to 8 MiB and 2048 changed files; an operation
has a two-minute deadline. JSON goes to stdout and operation diagnostics to stderr.

`tag-publish` is a separately confirmed remote publication. Supply `name` (a
short tag name, without `refs/tags/`), `commit` (a full lowercase object ID matching
the repository format), the fresh `snapshot`, and `confirm: true`. For example:

```json
{
  "workdir": "/path/to/project",
  "action": "tag-publish",
  "snapshot": "<fresh status fingerprint>",
  "name": "reviewed-example",
  "commit": "<exact full reviewed commit object ID>",
  "confirm": true
}
```

The project must be clean, have no active Git operation, and contain the selected
commit in the reviewed HEAD's ancestry. The runtime resolves exactly one effective
`origin` push destination and uses that same destination for observation and one
non-force `<commit>:refs/tags/<name>` push. It does not create a local tag, move a
branch, change upstream configuration, fetch, force, delete or follow other tags.
Before observing or pushing, the resolved literal destination must remain the same
under Git's fetch URL resolution and must match no further `insteadOf` or
`pushInsteadOf` prefix. This conservative check rejects rewrite ambiguity,
including explicit `origin.pushurl` rules that a later literal push would handle
differently. It is repeated immediately before pushing without changing stored
configuration, credential helpers or hooks. Ordinary origin shorthand rewrites
are supported when their resolved destination needs no further rewrite.
A literal destination that is also a configured remote name is rejected.

An existing remote ref is accepted only when its actual object ID exactly equals
the selected commit; an annotated tag object is not equivalent to its peeled
commit. A competing ref is never overwritten.

Success includes `tag_publication` with the requested and observed commit, exact
ref, `origin`, a SHA256 of its effective push destination, and `already_published`
and `recovered` flags. The destination URL is not returned. `recovered` means a
push reported failure but one readback proved that the remote accepted the exact
commit. Failed or ambiguous readback is an error; there is no blind push retry.
Cancellation propagates and may leave an already accepted remote tag in place.
Refresh and reconcile that exact ref before retrying. The existing two-minute
CLI deadline and bounded Git output apply.

This adapter serves explicit human actions in an existing project. Autonomous
agents continue to use the managed [session lifecycle](runtime/git-runtime-sessions.md).

## `dockpipe init`

Local project setup only: **no `git clone`**, no treating **`init`** as a remote bootstrap. Everything runs in the **current working directory**.

| Command | Purpose |
|---------|---------|
| `dockpipe init` | Create the **minimal project scaffold**: **`workflows/`**, **`README.md`**, and **`dockpipe.config.json`** when missing. If no Dockpipe workflows exist yet, also generate **`workflows/example/config.yml`** with one native host-shell greeting step and Docker preflight disabled. These built-in scaffolds work offline without extracting bundled core assets. Secrets are opt-in: no vault template or provider is configured. It does **not** copy **`templates/core/`**, **`scripts/`**, or **`images/`** into the project. |
| `dockpipe init <name>` | Create **`workflows/<name>/config.yml`** as a **minimal empty workflow** (name + description only). **`--from init`** instead generates the runnable host starter. If **`workflows/`** already exists but has no **`*/config.yml`** trees, the CLI prints a **warning** (often a conflict with GitHub Actions). |
| `dockpipe init <name> --workflows-dir <path>` | Put the new workflow under **`<path>/<name>/`** instead of **`workflows/`** (repo-relative or absolute). Same as env **`DOCKPIPE_WORKFLOWS_DIR`** for **`dockpipe run`**. |
| `dockpipe init <name> --from <source>` | Generate the built-in **`blank`** or **`init`** scaffold, or copy another **declared project workflow** or a **local directory** into **`workflows/<name>/`** (or **`--workflows-dir`**). Explicit filesystem paths are copied directly. Not a Git URL. |
| `dockpipe init <name> --resolver <n> --runtime <n> --strategy <n>` | Optional; written into the new **`config.yml`**. New workflows use plain **`resolver:`** and **`runtime:`**. Example: **`dockpipe init my-pipeline --from run-apply --resolver codex --runtime dockerimage`**. |
| `dockpipe init --gitignore` | **Opt-in** only: append a marked block to **`.gitignore`** at the **git repository root** (`bin/.dockpipe/`, Go caches, `tmp/`). Idempotent if the block is already present. Requires a **git working tree** (run from inside a repo). |

Named workflows normally live at `workflows/<name>/config.yml` in your project.
Installed packages and configured source roots are also searched. Override the
primary source folder with `--workflows-dir` or `DOCKPIPE_WORKFLOWS_DIR`; use
`--workflow-file` when you want an exact file. Legacy `templates/<name>/config.yml`
remains a compatibility fallback.

**`dockpipe init`** creates the root **`README.md`** when missing and ensures **`workflows/`** exists.

The built-in `init` starter is generated from the library workflow model. It does not copy the older Docker/Bash example or its PipeLang model files. To reuse an authored example, pass its explicit local directory. The legacy `dockpipe template init [--from <source>] <directory>` command still creates a standalone workflow directory, but no longer copies shared `templates/core/`; use installed runtime/resolver packages when that workflow needs them.

## Core-tree installer retirement

The CLI is self-contained. `dockpipe install core` is retired and reports a
migration error without fetching or materializing a tree. Use the
[application installer](install.md) or `dockpipe package install` for optional
workflow/resolver packages. Existing published download assets are unchanged.

## `dockpipe clone`

| Command | Purpose |
|---------|---------|
| `dockpipe clone <name> [--workdir <path>] [--to <dest>] [--force]` | Copy **`bin/.dockpipe/internal/packages/workflows/<name>/`** to **`--to`** (default **`<workdir>/workflows/<name>`**) only if **`package.yml`** has **`allow_clone: true`**. Refused when **`allow_clone`** is false or omitted (typical for commercial **binary-only** drops). |

See **[packages/package-model.md](packages/package-model.md)** (**Clone, education, and commercial packages**).

## `dockpipe build` / `clean` / `rebuild`

Project build and cleanup commands (see **[packages/package-model.md](packages/package-model.md)**).

| Command | Purpose |
|---------|---------|
| `dockpipe build [options]` | Rebuild the compiled package/workflow store under **`bin/.dockpipe/internal/packages/`** for the current project. In practice this is the main “refresh compiled state” command. It also materializes configured PipeLang roots and prebuilds Dockerfile-backed image artifacts unless you pass **`--no-images`**. If **`--workdir`** is omitted, the project directory is the folder containing **`dockpipe.config.json`** (walking up from the current directory), or the current directory if that file is absent. Options: **`--workdir`**, **`--for-workflow <name>`**, **`--images`**, **`--no-images`**. |
| `dockpipe clean [--workdir <path>] [--dry-run]` | Preview or remove the complete checkout-local disposable tree at **`<workdir>/bin/.dockpipe`**. Dry-run reports the exact resolved target, logical bytes, file count, and action without mutation. A missing tree is a reported no-op. Ordinary clean never follows **`DOCKPIPE_PACKAGES_ROOT`** outside the checkout. |
| `dockpipe rebuild [options]` | Separately reset the resolved compiled store, including **`DOCKPIPE_PACKAGES_ROOT`**, then run **`build`**. The reset is reported and rejects filesystem roots, home/workdir authority, durable state, links/reparse points, filesystem substitutions, and unsafe trees. |

Clean accepts only its exact checkout-local target. It rejects parent traversal, filesystem-root
workdirs, workdir/ancestor or durable-state targets, links/reparse points, special files, and
filesystem substitutions. Durable package state resolved by `dockpipe scope --package` remains
outside the disposable tree and survives ordinary clean. Rebuild retains its separate guarded reset
of the resolved compiled store for `DOCKPIPE_PACKAGES_ROOT` compatibility.

## `dockpipe package`

Discover and install remote packages, inspect local inventory, or compile your own
workflows. Remote installation uses the per-user store; project compilation uses
`bin/.dockpipe/internal/packages/`. See [Find and use packages](packages/package-quickstart.md)
for the normal install/run path and [Package model](packages/package-model.md) for
advanced authoring and storage.

| Command | Purpose |
|---------|---------|
| `dockpipe package list [--workdir <path>]` | Print project-store package metadata in text form; use `--format json` for inventory across project, configured, user and system stores, with warnings. |
| `dockpipe package list --format json [--workdir <path>]` | Inventory project, configured, user, and system packages, plus warnings and the user install root. |
| `dockpipe package catalog --remote <HTTPS manifest URL>` | Return the current execution platform's remote packages and resolved store manifest as JSON. Inside Flatpak this includes its runtime ID/branch; native processes retain OS/architecture selection. Accepts a latest pointer, release catalog, or explicit platform store. |
| `dockpipe package install --remote <URL> --kind core\|workflow\|resolver --name <name> [--sha256 <digest>]` | Verify and install one package into the user store. A digest pins the selected catalog entry. Dependencies are installed separately. |
| `dockpipe package uninstall --path <absolute archive path>` | Remove one optional user-store archive. Core, external stores, directories, and symlinks are protected; package data and settings are retained. |
| `dockpipe package images [--workdir <path>]` | Merge planned image artifacts from compiled workflow tarballs with materialized/cached receipts under **`bin/.dockpipe/internal/images/by-fingerprint/`**; print **fingerprint**, **status**, **state**, **source**, **image_ref**, **workflow**, **package**, **step_id**, **image_key** (tab-separated). Status can show **`ready`**, **`missing`**, **`stale`**, **`planned`**, **`referenced`**, or **`docker-error`**. |
| `dockpipe package manifest` | Print an example **`package.yml`** (schema: **name**, **version**, **title**, **description**, **author**, **website**, **license**, optional **kind**). |
| `dockpipe package test [--workdir <path>] [--only <package>]` | Run package-owned tests for packages in the current source checkout that declare **`test.script`** in **`package.yml`**. |
| `dockpipe package build store [--workdir <path>] [--out <dir>] [--only <slice>] [--version <ver>]` | Gzip tar per compiled package under **`bin/.dockpipe/internal/packages/`** (after **`dockpipe build`**), archive paths **`workflows/<name>/`**, **`resolvers/<name>/`**. **`packages-store-manifest.json`** lists artifacts. Default **`--out`**: **`release/artifacts/`**. |
| `dockpipe package compile workflow <source-dir> [--workdir <path>] [--name <n>] [--force]` | **`workflow validate`** on **`source-dir/config.yml`**, then copy the tree to **`bin/.dockpipe/internal/packages/workflows/<name>/`** (writes **`package.yml`** if missing, with **`allow_clone: true`** and **`distribution: source`** for local round-trips). |

**Environment:** **`DOCKPIPE_PACKAGES_ROOT`** — override the compiled package root (default **`<workdir>/bin/.dockpipe/internal/packages`**) for compile/build/rebuild. It does not relocate durable package state and ordinary clean does not use an external override.

## `dockpipe workflow test` and `dockpipe test`

Workflow-local tests are discovered from on-disk workflow directories that
contain one of:

- `tests/run.sh`
- `tests/run.ps1`
- `tests/run.cmd`
- `tests/run.bat`

Commands:

| Command | Purpose |
|---------|---------|
| `dockpipe workflow test [--workdir <path>] [--only <workflow>]` | Run workflow-local tests for workflows discovered from the current project/workdir. |
| `dockpipe test [--workdir <path>]` | Umbrella test command: run `dockpipe package test` plus `dockpipe workflow test`. |
| `dockpipe test --only <name>` | Limit the umbrella run to one matching package/workflow name when applicable. |

## `dockpipe pipelang`

Typed authoring helper backed by the versioned external [PipeLang SDK](https://github.com/Dockpipe-Industries/pipelang). Dockpipe retains these commands and automatic `.pipe` materialization for existing models. The standalone `pipelang` CLI is also available from that repository. PipeLang compiles to inspectable artifacts; workflow execution still uses normal Dockpipe YAML.

When a workflow also declares `view:`, that YAML remains a launcher/tooling presentation layer over the typed model rather than a separate execution contract. See **[concepts/pipelang.md](https://github.com/Dockpipe-Industries/pipelang/blob/master/docs/concepts/pipelang.md)** and **[workflows/workflow-yaml.md](workflows/workflow-yaml.md)**.

| Command | Purpose |
|---------|---------|
| `dockpipe pipelang check --in <file.pipe> [--format text\|json] [--stdin]` | Strictly decode, parse, and type-check the source set without evaluating code or emitting artifacts. JSON is the stable structured diagnostic projection; `--stdin` replaces the selected file bytes for unsaved editor buffers. |
| `dockpipe pipelang compile --in <file.pipe> [--entry <Class>] [--out <dir>]` | Parse + type-check PipeLang and emit artifacts: `<Class>.workflow.yml`, `<Class>.bindings.json`, `<Class>.bindings.env` (default out: `bin/.dockpipe/pipelang/`). |
| `dockpipe pipelang invoke --in <file.pipe> [--class <Class>] --method <name> [--arg <value>]... [--format text|json|env]` | Invoke a PipeLang method through CLI only. No resolver/runtime execution path is used. |
| `dockpipe pipelang materialize [--workdir <path>] [--from <root>]... [--force]` | Recursively compile `.pipe` files under roots into sidecar `.pipelang/` artifacts. Default roots are resolved from `dockpipe.config.json` `compile.workflows`. |

Method support (v0.0.0.1): expression-bodied methods are parsed, type-checked, included in bindings metadata, and invocable only via `dockpipe pipelang invoke`.

## `dockpipe release`

Upload a single file to an **S3-compatible** bucket (e.g. **Cloudflare R2**) using the **`aws`** CLI. Requires **`AWS_ACCESS_KEY_ID`** and **`AWS_SECRET_ACCESS_KEY`** (or compatible credentials). Not required for **official** Dockpipe distribution; use for **self-hosted** package mirrors.

| Command | Purpose |
|---------|---------|
| `dockpipe release upload <local-file> [--bucket <name>] [--key <object-key>] [--endpoint-url <url>] [--region <name>] [--content-type <ct>] [--dry-run]` | **`aws s3 cp`** to **`s3://bucket/key`**. **Bucket:** **`--bucket`** or **`DOCKPIPE_RELEASE_BUCKET`** or **`R2_BUCKET`**. **Endpoint:** **`--endpoint-url`** or **`R2_ENDPOINT_URL`** / **`AWS_ENDPOINT_URL_S3`**, or derive **`https://<id>.r2.cloudflarestorage.com`** from **`CLOUDFLARE_ACCOUNT_ID`** / **`R2_ACCOUNT_ID`**. **Key** defaults to the file’s basename. |

## Workflow variables (`--workflow`)

When you use `--workflow <name>`, the resolved **`config.yml`** (see **[workflows/workflow-yaml.md](workflows/workflow-yaml.md)** for lookup order) may include a top-level **`vars:`** block: indented `KEY: value` lines. Those names are **exported** before run scripts, **only if** they are not already set in your shell (so your environment and CI secrets win).

Additional sources (each only sets **unset** variables, except `--var`):

1. `vars:` defaults in `config.yml`
2. `.env` beside the selected workflow `config.yml`
3. `.env` at the explicitly selected project/root context
4. Each `--env-file <path>` (in order)
5. Path in **`DOCKPIPE_ENV_FILE`** if set
6. **`--var KEY=VAL`** (always exported; overrides yml and `.env`)

Use **`--var`** for one-off overrides; use **`.env`** files for local secrets and team defaults.

## Flags

All options must appear **before** a standalone **`--`**. The command and its arguments follow **`--`**.

**`--workdir`:** Optional. If omitted, the project directory is the **current working directory** (`PWD`). **Typical local use:** **`cd`** to the repo root and run **`dockpipe`** with **no** **`--workdir`**. Use **`--workdir <path>`** when the process is not started from that directory (CI, **`make`**, wrappers) or when you want an explicit path without **`cd`**.

| Flag | Aliases | Purpose |
|------|---------|---------|
| `--workflow <name>` | | Select a named workflow from project/declared roots or installed/configured packages. With `steps:`, a final `--` is optional. Mutually exclusive with `--workflow-file`. |
| `--workflow-file <path>` | | Load explicit workflow YAML; relative run/act assets resolve beside it. Resolver selection uses declared/installed package roots. Mutually exclusive with `--workflow`. |
| `--package <name>` | | With **`--workflow <name>`**, select an unpacked packaged workflow whose nearest **`package.yml`** has matching **`name:`**. This is a generic package selector, not an AI-provider concept. Mutually exclusive with **`--workflow-file`**. |
| `--workflows-dir <path>` | | Repo-relative or absolute directory for **`--workflow <name>`** resolution (default **`workflows/`**). Same as **`DOCKPIPE_WORKFLOWS_DIR`**. Also **`dockpipe init <name> --workflows-dir …`**. |
| `--run <path>` | `--pre-script` | **Run:** script(s) on the host before the container. Repeatable. |
| `--isolate <name>` | `--template`, `--image` | Low-level image/template override. Builds if the value is a template. Prefer **`--runtime`** + **`--resolver`** for the normal path, and use **`--isolate`** when you need to pin the exact image/template. |
| `--act <path>` | `--action` | **Act:** script after the container command (e.g. commit-worktree). |
| `--runtime <name>` | | Select typed library runtime defaults or an explicit project/package profile. Resolver assignments overlay runtime defaults. |
| `--resolver <name>` | | Select a tool/platform adapter from declared source roots or installed packages. |
| `--strategy <name>` | | Select workflow/project lifecycle files, an explicit file, or the typed `commit`/`worktree` compatibility strategies. Operator files take precedence. |
| `--repo <url>` | | Clone URL for worktree flows. If omitted and the workflow uses **`clone-worktree.sh`**, dockpipe sets the URL from **`git remote get-url origin`** in the current dir (or **`--workdir`**). When that URL matches your **`origin`**, the worktree can be created from **your local checkout** (not only a fresh remote default). |
| `--branch <name>` | | Explicit work branch for **`--repo`** (optional). |
| `--work-branch <name>` | | When **`--repo`** is set and **`--branch`** is **omitted**, use this as the **full** branch name. If both are omitted, dockpipe generates **`prefix/<adj>-<noun>-<adj>-<noun>`** (random hyphenated slug; **`prefix`** from resolver / template — see **`domain.RandomWorkBranchSlug`** in **[branchslug.go](../src/lib/domain/branchslug.go)**). If **`--branch`** is set, it takes precedence over **`--work-branch`**. |
| `--workdir <path>` | | Host path mounted at `/work`. Default: **current directory** (see note above — omit when you **`cd`** to the project first). |
| `--work-path <path>` | | Subdirectory inside the repo used as the container working directory (`/work/<path>`). |
| `--bundle-out <path>` | | After commit-on-host, write a git bundle here (default: **current branch only**). Set **`DOCKPIPE_BUNDLE_ALL=1`** for `git bundle create … --all`. |
| `--env KEY=VAL` | | Pass env into the container. Repeatable. When `/work` is a git worktree, dockpipe also sets **`DOCKPIPE_WORKTREE_BRANCH`**, **`DOCKPIPE_WORKTREE_HEAD`**, or **`DOCKPIPE_WORKTREE_DETACHED=1`** from the **host** `git` view. |
| `--mount <host:container>` | | Extra volume mount. Repeatable. |
| `--data-dir <path>` | | Bind-mount host path for persistent tool data (`HOME` in container). |
| `--data-vol <name>` | `--data-volume` | Named Docker volume for persistent data (default: `dockpipe-data` when neither **`--data-dir`** nor **`--no-data`**). |
| `--no-data` | | Do not mount a data volume / data dir. |
| `--reinit` | | With a data volume: remove that volume before run (fresh HOME). Confirms on TTY; use **`-f`** / **`--force`** to skip confirmation. |
| `-f` / `--force` | | With **`--reinit`**, skip confirmation (warning may still print). |
| `--build <path>` | | Parsed (relative to repo root or POSIX absolute). **Not currently consumed** by the Go runner — image build dirs come from **`--isolate`** / template resolution. Kept for CLI compatibility; see **`CliOpts.BuildPath`** in **[flags.go](../src/lib/application/flags.go)**. |
| `--env-file <path>` | | Merge `KEY=VAL` lines into workflow env (unset keys only unless overridden). Repeatable. |
| `--var KEY=VAL` | | Set workflow var; **overrides** yaml / `.env`. Repeatable. |
| `-d` / `--detach` | | Run the container in the background. |
| `-h` / `--help` | | Print usage. |
| `--version`, `-v`, `-V` | | Print version (handled in **`main`** before **`Run`**). |

## Windows subcommands (`dockpipe windows …`)

**Optional.** For **`DOCKPIPE_USE_WSL_BRIDGE=1`**: install **`dockpipe`** in WSL and run **`dockpipe windows setup`** / **`doctor`**. Native **`dockpipe.exe`** users can ignore this. See **Optional: WSL bridge** in **[install.md](install.md)**.

| Command | Purpose |
|---------|---------|
| `dockpipe windows setup` | Interactive: select WSL distro, bootstrap env, optional **`--install-command`** in the distro. |
| `dockpipe windows setup --distro <name> --non-interactive` | Scripted setup (**`--distro`** required). |
| `dockpipe windows setup --bootstrap-wsl` | If WSL is missing or has no distros, run **`wsl --install`** (default distro **Alpine** when unset; may UAC / reboot), then continue. |
| `dockpipe windows setup --install-dockpipe` | Download latest **`dockpipe_*_linux_*.tar.gz`** from GitHub into **`~/.local/bin`** in WSL (for the bridge). |
| `dockpipe windows setup --install-command "<cmd>"` | Run a command inside the selected distro during setup (overrides **`--install-dockpipe`**). |
| `dockpipe windows doctor` | Print **`wsl --version`**, list distros and configured default, or hints if WSL is missing. |

### Windows host: native vs WSL bridge

**Default:** **`dockpipe.exe`** runs on **Windows**. **Docker Desktop** supplies **`docker`**; **`bash`** and **`git`** must be on **`PATH`** separately (e.g. **Git for Windows**) — Docker Desktop does not ship them.

**Container user:** **Linux/macOS** pass **`-u`** with the host **uid:gid** (overrides **`USER`** in the image). If you run **`dockpipe` as root** (e.g. **`sudo`**), that maps to **`-u 0:0`**, which breaks CLIs that refuse flags like **`--dangerously-skip-permissions`** as root. For **claude / codex / agent-dev** images, dockpipe defaults to **`-u node`** when the host uid is **0** (unless **`DOCKPIPE_FORCE_ROOT_CONTAINER=1`**). Override with **`DOCKPIPE_CONTAINER_USER`**.

**Windows:** host uid is unavailable; dockpipe **does not** pass **`-u`** unless **`DOCKPIPE_WINDOWS_CONTAINER_USER`** is set (image **`USER`** applies — e.g. **`node`** in **claude/codex** images). Defaulting **`-u node`** from the CLI caused bind-mount stalls for some Docker Desktop setups, so use an **explicit** value when needed: **`DOCKPIPE_WINDOWS_CONTAINER_USER=node`** for Claude Code with **`--dangerously-skip-permissions`**, or **`0`** for root.

**Claude Code `--dangerously-skip-permissions`:** The CLI blocks that flag when it thinks you’re root/sudo. In Dockpipe that behavior is handled by the **resolver image wrappers**, not the shared root entrypoint: the **claude** and **codex** images set **`IS_SANDBOX=1`** by default (Claude Code’s supported disposable-sandbox mode — see **[anthropics/claude-code#9184](https://github.com/anthropics/claude-code/issues/9184)**) and re-exec **root → `node`** via **`runuser`**/**`setpriv`** when the container still starts as uid 0. Opt out: **`DOCKPIPE_NO_SANDBOX_ENV=1`** and **`DOCKPIPE_SKIP_DROP_TO_NODE=1`**. **`DOCKPIPE_DEBUG=1`** still prints **`id`** from the shared entrypoint. Rebuild the resolver image after wrapper changes. **`DOCKPIPE_WINDOWS_CONTAINER_USER`** remains available if you want an explicit **`-u`** from the host.

**Design: loose defaults in isolation.** Dockpipe targets **disposable containers**; resolver-owned images may apply automation-friendly defaults like **`IS_SANDBOX=1`** while the CLI still passes host **uid:gid** on Unix. People who want stricter behavior can opt out with the env vars above. **Dockpipe does not append** **`--dangerously-skip-permissions`** to your command — add it after **`--`** if you want that mode. **Security** is mostly **what you mount** (only **`/work`** etc.), **secrets** in env/volumes, and **network** — not “CLI tightness” alone.

**Windows + host `git` / Docker mounts:** Bash scripts may export **`DOCKPIPE_WORKDIR`** as **`/c/Users/...`** (Git Bash). Native **`git.exe`** and **`docker -v`** need normal Windows paths; dockpipe converts MSYS-style paths before **`git -C`**, **`docker run` bind mounts**, **`--data-dir`**, and **`--mount`**. If **`/work`** looks empty in the container, rebuild with a current dockpipe and confirm the printed **Mount /work ← …** path exists on the host.

**Code edits vs host user:** On **Linux/macOS**, the container usually runs as **your host uid:gid**, so files written under **`/work`** are owned by **you** on the host (unless you override **`-u`**). **Claude**’s permission model (prompts vs **`--dangerously-skip-permissions`**) is **application-level**; it does not replace **POSIX** ownership — both apply: the process uid writes files, and Claude may still ask or gate tools unless you pass flags it documents.

**WSL bridge (opt-in):** set **`DOCKPIPE_USE_WSL_BRIDGE=1`** so every command **except** `dockpipe windows …` runs inside WSL via **`wsl.exe -d <distro>`**. The current Windows working directory is mapped with **`wslpath`**.

Environment: **`DOCKPIPE_WINDOWS_BRIDGE=1`** is set for the **inner** process when the bridge is used. **Windows-style paths** in path-like flags (`--workdir`, `--work-path`, `--data-dir`, `--mount`, `--build`, `--env-file`, `--run` / `--act`, `--isolate` / `--template` / `--image` when the value is a host path, `--env` / `--var` when the value is a path, etc.) and in **`init` / `action init` / `pre init` / `template init`** destination args are translated to WSL paths: **`wslpath -u`** first, then a **drive-letter** fallback (`C:\…` → `/mnt/c/…`). **UNC** paths (`\\server\share\…`) use a fallback that avoids breaking **`filepath.Clean`**. Everything after a standalone **`--`** is passed through unchanged.

## Examples

**Minimal (workflow does the rest):**

```bash
dockpipe --workflow my-ai --resolver claude --repo https://github.com/you/repo.git -- claude -p "Fix the bug"
```

**Override branch, add env:**

```bash
dockpipe --workflow my-ai --resolver claude --repo https://github.com/you/repo.git --branch my-feature \
  --env "GIT_PAT=$GIT_PAT" -- claude -p "Implement the plan"
```

**No workflow — override run/isolate/act:**

```bash
dockpipe --resolver claude --repo https://github.com/you/repo.git \
  --run ./my-run.sh --act ./my-act.sh -- claude -p "Task"
```

## Secret environments

Use `--secret-environment NAME` with a named entry in `dockpipe.config.json` → `secrets.environments`. `vault: environment` requires the selected/default environment; a missing selection or failed load stops execution. `--no-vault` disables both environment resolution and legacy template injection. See [Secret environments](runtime/vault.md) for 1Password, AWS Secrets Manager, Azure Key Vault, and Infisical configuration.

## Portable SDK paths

Use `dockpipe sdk shell-env` to initialize the shell SDK.
`dockpipe sdk path --language shell|powershell|python [--workdir <path>]` returns
the requested library-owned helper path; no source checkout is required.
Go consumers use `dockpipe/src/lib/sdk/repotools` from the published SDK.

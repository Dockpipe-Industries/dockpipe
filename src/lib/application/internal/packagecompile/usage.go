package packagecompile

const packageCompileUsageText = `dockpipe package compile

Validate and materialize packages into bin/.dockpipe/internal/packages/ (see docs/packages/package-model.md).

Usage:
  dockpipe package compile core [options]
  dockpipe package compile resolvers [options]
  dockpipe package compile bundles [options]   (alias: same as compile workflows)
  dockpipe package compile workflows [options]
  dockpipe package compile all [options]
  dockpipe package compile for-workflow <name> [options]   (core + transitive resolvers/workflows only)
  dockpipe package compile workflow [options] [--from] <source-dir>

Order for a full local store: core (spine only) → resolvers → workflows (dockpipe-workflow-* tarballs only).
Use "compile all" to run the default sequence.

`

const packageCompileWorkflowUsageText = `dockpipe package compile workflow <source-dir>

Runs workflow YAML validation (same rules as dockpipe workflow validate), runs optional
compile_hooks from config.yml against the staged package copy (shell, cwd = staged copy; source and
staging paths exported via DOCKPIPE_COMPILE_* env vars), then writes the workflow tarball under
<workdir>/bin/.dockpipe/internal/packages/workflows/.

Options:
  --workdir <path>   Project directory (default: current directory)
  --from <path>      Source workflow directory (same as positional <source-dir>)
  --name <n>         Package folder name (default: workflow name from config.yml, else basename of source)
  --force            Replace existing package directory

`

const packageCompileResolversUsageText = `dockpipe package compile resolvers

Merges each child directory from each --from source into
bin/.dockpipe/internal/packages/resolvers/<name>/ (later --from wins on name clash).

Defaults: compile.workflows roots, plus
src/core/resolvers and templates/core/resolvers when those directories exist.
Dirs with profile/ under each root become resolver tarballs.

Pack roots:
  - <from>/resolvers/... for a flat vendor tree
  - Any nested directory named resolvers/ under a declared compile root
Each immediate child with profile/ becomes one store tarball.

Optional resolver.yaml next to each profile may set namespace: <label> (same rules as workflow namespace).

Options:
  --workdir <path>      Project directory (default: current directory)
  --from <path>         Repeatable; each root's subdirectories are resolver profiles

`

const packageCompileBundlesUsageText = `dockpipe package compile bundles

Same as "dockpipe package compile workflows". Legacy command name only; source roots still come
from compile.workflows or repeatable --from options.

Options:
  --workdir <path>      Project directory (default: current directory)
  --from <path>         Repeatable workflow roots
  --force               Replace existing tarballs

`

const packageCompileWorkflowsUsageText = `dockpipe package compile workflows

Compiles every immediate subdirectory that contains config.yml under each --from root.

Defaults: dockpipe.config.json compile.workflows, else workflows/ when present.

Options:
  --workdir <path>       Project directory (default: current directory)
  --from <path>          Repeatable; roots to scan for named workflow folders
  --force                Replace existing packages/workflows/<name>
  --prune-stale          Remove workflow tarballs not produced by this compile pass

`

const packageCompileAllUsageText = `dockpipe package compile all

Runs: compile core → compile resolver packages (one tarball per profile) → compile workflow packages.
Uses dockpipe.config.json compile.workflows for source lists when present (see packages/package-model.md).

Note: dockpipe build runs this command with --force so existing compiled trees are replaced.

Options:
  --workdir <path>   Project directory (default: directory with dockpipe.config.json, walking up from cwd; else cwd)
  --force            Replace existing packages/core and tarball outputs under packages/workflows/
  --with-bundles, --skip-bundles   Ignored (compatibility no-ops)

`

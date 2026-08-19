## Rank 10 Core Source-Hygiene Audit — 2026-08-15

Completed a read-only `src/core` reachability and architecture-drift audit against `js/dev` at
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`. Before this documentation checkpoint the checkout had
116 tracked dirty paths, 42 untracked paths, no staged files, and status SHA-256
`c53dfeb9f49650fcb26bf8c2e82374851e570629ee49d8d65c5222837c488751`. Existing dirty source,
TASK-035 ranks 1-9, both completed compatibility retirements, and unrelated package/editor/VM bytes
remain authoritative.

The audit classified evidence without deleting or changing product behavior:

- `src/core` is not broadly dead. The engine resolves runtimes, resolvers, strategies, scripts, and
  bundled workflows by name, and `embed.go` carries the authored tree. `base-dev`, `dev`, VM,
  `dotenv`, the shared SDKs, and bundled workflows have maintained source or documentation callers;
- one ignored 8,408-byte Python cache exists at
  `src/core/assets/scripts/lib/__pycache__/repo_tools.cpython-310.pyc`. Core compilation calls
  `copyDirExcludingTopLevel`, and both that path and source-checkout template merging reach the
  generic unfiltered `copyDir` filesystem walk. The ignored file can therefore become an artifact
  passenger even though it is not maintained source;
- ShellCheck identified the unused `line` declaration in `vmimage_sync_host_to_guest`. Other
  reported dynamic export, SSH, trap, nameref, and pattern warnings were not classified as dead;
- `docs/concepts/architecture-model.md` says the core root contains only category directories, but
  `src/core/package.yml` intentionally drives the guest-agent source build and the Python package
  markers support the documented SDK import path. The layout guard checks category directories but
  does not express those intentional loose-file exceptions;
- `src/core/assets/images/{example,minimal}`, the agnostic Compose demos, example scripts,
  `helloworld.ps1`, and the example resolver are plausible untethering candidates, not proven dead
  code. They remain documented, test-retained, manually buildable, or dynamically copyable. The
  existing `docs/packages/core-vs-packages-audit.md` roadmap already requires a separate product
  decision before moving demo images, Compose, bundled workflows, or lean resolvers into packages.

Read-only/fabricated validation found no unused Go code under `src/cmd` or `src/lib` with
`staticcheck -checks=U1000`. The full infrastructure package tests and focused core compile/template
merge application tests passed offline with isolated `/tmp` caches. A broader application package
run stopped producing output after the infrastructure result and was interrupted without claiming
success; the focused affected tests passed afterward. No repository file changed during the audit,
and its start/final dirty-status hash matched exactly.

This checkpoint authorizes no cleanup. Rank 10 implementation is one separately gated generic
source-hygiene slice only: exact generated-Python-cache exclusion with compiled/scaffolded fixtures,
the unused local removal, and documentation/layout-guard reconciliation. It explicitly excludes
demo/image/Compose/workflow/resolver removal or relocation, compatibility behavior, CLI aliases,
package/state/clean behavior, generated-state cleanup, live resources, staging, commit, push, and
worktrees.

## Rank 10 Core Source-Hygiene Implementation — 2026-08-15

Implemented only the approved rank-10 source-hygiene slice in the saved dirty checkout. Before
mutation the checkout matched the delegated anchors exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 116 tracked
dirty paths, 42 untracked paths, and full status SHA-256
`c53dfeb9f49650fcb26bf8c2e82374851e570629ee49d8d65c5222837c488751`. This
record, the compatibility ledger, and the existing ignored cache matched every handed-off digest,
mode, and size.

The generic `copyDir` contract remains unchanged for workflows, resolvers, clones, and other source
trees. A core-only wrapper now filters exactly directories named `__pycache__` and files ending in
`.pyc` or `.pyo`, without consulting Git or ignore files. Core compilation applies that filter below
its existing top-level resolver/bundle/workflow exclusions, and bundled-core merging applies it only
while copying core source categories or a materialized core source tree. The compiler still carries
loose root source files, and both paths still carry ordinary Python, hidden/ignored non-cache, and
names that merely contain a cache-like suffix.

Focused fixtures fabricate nested `__pycache__` passengers and loose `.pyc`/`.pyo` bytecode. The
compiled-core test proves their omission from the tarball while retaining a root marker, a hidden
core asset, `.py`, and `.pyc.txt` files. The source-checkout merge test proves the cache directory and
loose bytecode do not enter the scaffold while the same ordinary assets and runtime category do.
`vmimage_sync_host_to_guest` lost only its unused `line` declaration. The canonical architecture
document now names `package.yml` as the core source-build manifest and `__init__.py` as the Python
package marker alongside the five category directories; the layout guard enforces those exact file
and directory sets and rejects special root entries.

Focused offline verification used cached Go 1.25.11 with `GOTOOLCHAIN=local`, `GOPROXY=off`,
`GOSUMDB=off`, the existing read-only module cache, and isolated `/tmp` build/temp roots:

- the exact compiled-core, source-build, installed-core merge, and source-checkout merge application
  tests passed, including every fabricated retained/excluded path assertion;
- the complete `src/lib/infrastructure` package passed, including the reconciled core-layout guard;
- Linux shell syntax and warning-level ShellCheck passed after excluding only the pre-recorded
  unrelated dynamic-export, pattern, trap, and array warnings; the unused-local warning class stayed
  enabled;
- repository-layout and canonical template-path guards, Go formatting, and `git diff --check`
  passed;
- Windows/amd64 compile-only builds for the application and infrastructure test packages passed
  with `CGO_ENABLED=0`, emitting only isolated `/tmp` test executables. An earlier `go test -run
  '^$'` attempt compiled those Windows binaries and then predictably failed to execute them on Linux;
  it was replaced by the correct non-executing `go test -c` proof. Two initial setup-only commands
  selected host Go 1.22 under `GOTOOLCHAIN=local` and stopped at the Go 1.25 workspace requirement;
  neither setup attempt changed repository state.

Final dirty ownership is 121 tracked paths and 42 untracked paths with status SHA-256
`ce5ce274a35c3a04f1b0bda4a15e051952816fccf87aceb183c630acae470494`; no files
are staged. The compatibility ledger remains SHA-256
`14517144ee400971d6e35899e91129cdd2e339d76adf1d726ed88f9a4bffcdf0`, the
protected Cursor/VS Code resolver aggregate remains
`788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`, and
`src/lib/application/package_compile.go` remains SHA-256
`8552aab1424d1a065c740826abb5a66c257bba5c1293d29e808b5b287606cbc6`. The real
ignored cache remains present and byte-identical at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`. Reversing
only the owned vmimage local-line edit reconstructs its exact pre-slice SHA-256
`44aff5d4a23fcdebff00e442b07b2baa0b2cd8571756154d44097f089442e1a8`.

No real cache, runtime, resolver, strategy, workflow, image, Compose file, example, PowerShell file,
SDK, CLI alias, compatibility entry, package/state/clean/rebuild behavior, generated checkout state,
live service, Docker/VM/network resource, staging, commit, push, worktree, or successor was touched.
Generated validation artifacts exist only under `/tmp`. The engine/package boundary remains intact:
generic application code owns a narrowly scoped core-source hygiene rule without any package or
product-name special case.

Terminal disposition: `completed`. Demo/core untethering remains a separate product decision, not a
pending rank-10 action. No successor was created.


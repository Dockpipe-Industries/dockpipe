### Rank 2 Durable Package-State VM Identity Split — 2026-08-14

Implemented only the approved TASK-035 VM durable guest-identity split in the saved dirty checkout.
Before mutation the checkout matched the delegated anchors exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 47
tracked dirty paths, 25 untracked paths, full status SHA-256
`abedb18004e87e4349d49170057d1c10716340ad573b439b9b7d1002d43241e0`, and this
894-line task record at SHA-256
`5d7bb603cd0d566bf05f9ce0308823ec9edfdc93b49da04b9c4bafcf130175ba`.

The VM package now requests two independent roots through one hidden, generic application bridge:
a collision-safe durable project/package cohort keyed by the resolved guest disk identity and a
collision-safe disposable package-runtime run directory keyed by the run identity. Machine UUID,
network MAC, disk serial, writable firmware variables, TPM state, generated Windows administrator
password, and Windows SSH identity use the durable root. QEMU overlays, sync archives, PID files,
TPM sockets/logs, QEMU logs/argument files, installer/bootstrap media, unattended-install products,
and other run temporaries continue through the disposable runtime root. Generated credentials and
identity files are owner-only, reject missing/empty/linked or internally multiline authority, and
survive a new run without regeneration; an interrupted missing SSH public key is reconstructed from
the durable private key and then checked for an exact key identity match.

The minimum general helper builds on cohort 1's durable project/package and package-runtime
primitives. The VM package supplies its owner, cohort, instance, run, exact legacy file/tree
mappings, and TPM socket/log exclusions; no VM or DorkPipe product knowledge entered the generic
importer. Selected legacy regular files and directories are inventoried in sorted order with sizes,
SHA-256 hashes, and source filesystem identities, copied without source mutation into an owner-only
sibling temporary, re-inventoried, synchronized, and atomically published with canonical
provenance. Durable authority wins over later legacy divergence. Ready temporaries resume without
copy replay; incomplete temporaries are removed only when their bytes are a proven source subset.
Malformed provenance, overlapping/traversing mappings, links/reparse points, special files,
filesystem crossings, same-byte object replacement, source/destination substitution, permission
drift, ambiguous legacy TPM authorities, and malformed helper output fail closed. Legacy discovery
remains non-mutating and real state was not exercised.

Bounded offline validation used isolated durable, legacy, runtime, cache, home, and cross-compile
roots only:

- focused importer and hidden-command tests passed under `-race`, including selected-copy and
  no-mutation, durable-wins/divergence, link/special-file/object/destination substitution,
  permission drift, concurrent preparation, incomplete/ready/lost-ack interruption, malformed
  provenance, and disposable-run separation cases;
- full `src/lib/infrastructure`, focused `src/lib/application`, `src/cmd`, and `go vet` checks
  passed with `GOPROXY=off`; Windows/amd64 and macOS/amd64 compile-only checks passed and none of
  their binaries was executed;
- the authoritative `dockpipe package test --workdir . --only vm` harness passed all VM Go tests
  and the package-owned state-split contract, proving restart stability without identity/password/
  SSH regeneration, per-run runtime separation, collision separation between disks, exact legacy
  mapping, ambiguous TPM rejection, interrupted public-key reconstruction, empty-authority and
  malformed-helper rejection, owner modes, and static durable/disposable caller classification;
- `gofmt`, shell syntax, focused ShellCheck, generic-boundary/reference scans, protected-prefix
  hashes, and `git diff --check` passed. Full-script ShellCheck reports only the inherited warning
  set and removes three inherited `SC2155` findings; it introduces no new finding. A broad
  application package run did not produce a terminal result and was stopped after more than two
  minutes; the focused affected application tests and race run are authoritative for this slice.

The package harness refreshed only ignored package-test products and `/tmp` caches/artifacts. No VM
or native/live qualification, real state migration/deletion/cleanup, disposable caller conversion,
IDE/code-server state, public package-state/get/scope/environment/SDK/editor/workflow/documentation
surface, clean/rebuild, package-root cutover, generated-state/prune behavior, external dependency,
staging, commit, push, worktree, or successor action occurred. Cohorts 1-3, their tests, generated
state history, and unrelated dirty bytes remain authoritative. The package/engine boundary is
preserved by a generic hidden durable-cohort primitive and package-owned VM mappings and behavior.

Terminal disposition: `completed`. Disposable caller conversion, unresolved IDE/code-server state,
public cutover, cleanup, and every later cohort remain separately gated. No successor was created.

### Rank 2 Disposable Package-Runtime Callers — 2026-08-14

Implemented only the approved maintained non-IDE disposable caller conversion in the saved dirty
checkout. Before mutation the delegated anchors matched exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 49
tracked dirty paths, 31 untracked paths, full status SHA-256
`bc33008d857b3a0be3c4b355a36bc0c6555b2b8cc1e80429ff22e77ff252a4e0`, and this
961-line task record at SHA-256
`852911d441f52f11a1e14fb217f31393842ea0a043e35b2cee90047d434e2cab`.

One hidden generic application bridge now exposes cohort 1's collision-safe `PackageRuntimeDir`
with validated optional suffixes. Its optional private-root preparation rejects linked/reparsed or
filesystem-substituted runtime components, applies owner-only protection to only the
`packages-runtime` parent and selected package root, and leaves the checkout state-root mode
unchanged. Package-bound CI paths now use the runtime helper, including the exact manifest owner in
package hooks so legacy-sanitizer collisions cannot share CI bytes. `DOCKPIPE_PACKAGE_STATE_DIR`,
`PackageStateDir`, public `get`/`scope`, the Go/editor public SDKs, and workflow/schema surfaces did
not change.

The DorkPipe package-owned facade now sends only its disposable families to package runtime:

- edit/reasoning request artifacts, `run.json`, nodes, CI raw/analysis, self-analysis, handoff and
  paste prompts, and deterministic `analysis/by-category` exports;
- provider leases and scratch, while provider resume bindings, adapter pins, App Server recovery
  state, insights/history, and metrics/training remain on their separately prepared durable
  authorities;
- package-script build caches, local dev-stack products, self-analysis products, handoff products,
  and node aggregation through the same hidden runtime bridge, with no fallback to public package
  state.

Pipeon built VSIX output, regenerated context, chat/status context lookup, and dev-stack ports,
PIDs, stack context, runtime environment, GPU/status/image stamps, API keys, and TLS material now
select collision-safe `pipeon` or `pipeon-dev-stack` runtime ownership. Existing private keys are
revalidated as regular non-links and repaired to owner-only mode; linked key/certificate/runtime
paths fail closed, newly written runtime environment files are owner-only atomic replacements, and
the runtime root is owner-only. The Pipeon extension and DorkPipe edit context reader resolve the
new context path and do not fall back to the legacy package-state location.

The exact unresolved hard stop remains `pipeon_stack_code_server_home`: it still resolves only the
legacy public `pipeon-dev-stack/code-server-home` path because that tree may contain user settings
or extensions. The similarly unresolved Cursor and VS Code package homes, resolver markers/caches,
and all IDE state were not converted. The two legacy-looking VSIX paths in the temporary
code-server Docker build context are image-context filenames rather than checkout state owners;
they were classified but not treated as package-state authority.

Focused offline validation used isolated durable, legacy, runtime, cache, home, and compile roots:

- affected infrastructure/application and DorkPipe statepaths, CI analysis, engine, handoff,
  insight, worker, and exact CLI context-reader tests passed under `-race`; full non-race
  infrastructure and focused application tests passed, and `go vet` passed across all affected Go
  packages;
- Windows/amd64 and macOS/amd64 compile-only checks passed for affected core and DorkPipe packages,
  with outputs under `/tmp` and no produced binary executed;
- shell syntax, focused DorkPipe/Pipeon runtime classification, DorkPipe SDK/insight/self-analysis,
  Pipeon package, secret-mode/link-rejection, TypeScript typecheck, and Pipeon package-runtime tests
  passed;
- the authoritative Pipeon package harness passed. The DorkPipe package harness passed all affected
  CI/runtime/insight/training/self-analysis/repository/build/orchestration/auth/GPU/CAS/lifecycle
  checks and retained only its two inherited rank-1 failures: the missing
  `workflows/software-dev/task-pack.yml` fixture and canonical backlog `--next` unexpectedly
  selecting a task;
- the complete infrastructure race suite exposed an unrelated inherited race in
  `operation_result_test.go`'s heartbeat counter, while the exact affected race cohort passed. The
  broad DorkPipe CLI package likewise retained only the inherited Windows-style workdir-candidate
  assertion already recorded by cohorts 2-3.

Tests changed only package-test products and isolated `/tmp` roots. An ignored Pipeon extension
build product used for typechecking was restored from the pre-slice tracked source before final
proof. Final protected-byte aggregate SHA-256 values were
`72a2dc05e04d8a18a9b01a8d92a0ac289773d44cc7238804def8eed525d07d93` for the
provider/learning migration implementation and tests,
`58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc` for the VM
state-split implementation and test, and
`a4d0e0ef6afa7b4ab4a8a303b7b231e71316394aee602a09f6ea1f1079e49d4e` for the
unresolved Cursor/VS Code resolver trees. The untouched public Go/CLI package-state compatibility
files hashed to `537a3b9e1eee2f3a758e1e712abab5fd8d7bdd0db67d696afc7bc9481524f466`.
The ignored Pipeon extension build product was restored at SHA-256
`4683442e439a7c9ec921e42c2b3ed276c0327181987bcd8ec2b6b3ddd96425bf`.

No real durable or legacy package state was imported, migrated, rewritten, deleted, or cleaned.
Cohorts 1-3 and VM durable identity behavior remain separated; no VM, IDE/code-server home, public
state cutover, canonical-doc/workflow/schema change, clean/rebuild, package-root cutover,
generated-state/prune action, external dependency/resource, staging, commit, push, worktree, or
successor action occurred. The engine/package boundary remains preserved: only generic runtime
path/private-root plumbing entered core, while DorkPipe and Pipeon own their caller classification.

Terminal disposition: `completed`. IDE/code-server state resolution, public package-state cutover,
cleanup, and every later cohort remain separately gated. No successor was created.

### Rank 2 IDE and Code-Server Ownership Split — 2026-08-14

Implemented only the approved maintained Cursor, VS Code, and Pipeon code-server ownership split
in the saved dirty checkout. Before mutation the delegated anchors matched exactly: branch
`js/dev`, HEAD `6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1
ahead, `stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 67
tracked dirty paths, 33 untracked paths, full status SHA-256
`a5e40fd818f809aa768d0a82abcfd208c1db3e3902e8e957a79ea9cbe331ee47`, and this
1,051-line task record at SHA-256
`fc0eef844007225841c61c99ce4a81563ae0346855c1b48a33ed055b64055822`.
The pre-slice Cursor/VS Code resolver-tree aggregate matched
`a4d0e0ef6afa7b4ab4a8a303b7b231e71316394aee602a09f6ea1f1079e49d4e`.

Each maintained IDE tree now has explicit package-owned ownership rather than one relabeled mixed
home:

- Cursor and VS Code general container-home bytes use separate collision-safe owner-only durable
  `ide-user-home-v1` cohorts. Their downloaded server products, binaries, logs, sockets, session
  activity, active-container markers, XDG cache, Dotnet/NuGet, Go module/build/workspace caches, and
  npm cache use the resolver's collision-safe `PackageRuntimeDir` owner.
- Cursor `.cursor-server` and VS Code `.vscode-server` are runtime trees with nested durable mounts
  only for installed remote extensions and `data/User` and `data/Machine`. XDG configuration and
  data use the independent `ide-user-data-v1` durable cohort; XDG cache remains runtime. Resolver
  guidance and session markers also remain runtime. The former VS Code repo-local home/cache move
  path was removed, so source `.vscode-server`, `.cache`, `.copilot`, `.dotnet`, `.gocache`, and
  `.gitconfig` bytes are never migration authority or mutation targets.
- Pipeon's general code-server home uses `code-server-user-home-v1`, excluding tool caches and all
  `.local/share/code-server` products. The latter is runtime except for nested durable `User` and
  `Machine` data, while user-installed extensions use the independent
  `code-server-user-data-v1` extension directory. Package-built Pipeon and DockPipe language VSIXes
  are copied into the image-owned built-in extension tree, so rehydratable package products cannot
  become durable user state. Code-server settings are seeded into the durable User directory by an
  owner-only atomic replacement that rejects linked or non-regular targets.

Every compatibility import is package-declared and uses the existing generic durable-cohort
importer. Selected legacy trees are copied without source mutation, atomically published, and
retain the importer’s durable-wins, restart, interruption, lost-acknowledgement, object-identity,
link/reparse, special-file, permission, and filesystem-boundary protections. Mixed homes are split
with explicit ignores rather than copied into one authority. One minimum hidden generic helper
creates a validated private subdirectory beneath an already private durable/runtime root; it
rejects traversal, duplicate command arguments, links/reparse points, and filesystem substitution.
No IDE, code-server, or Pipeon product knowledge entered `src/lib` or `src/cmd`.

Focused offline validation used only fabricated durable, legacy, runtime, cache, home, workdir,
global, and temporary roots:

- focused infrastructure/application tests and their race-enabled equivalents passed for the
  private-directory bridge and durable importer; `src/cmd`, affected `go vet`, `gofmt`, and
  `git diff --check` passed;
- Windows/amd64 and macOS/amd64 compile-only checks passed for infrastructure, application, and the
  CLI, with outputs under `/tmp` and no cross-compiled binary executed;
- the authoritative IDE package harness passed its existing devcontainer lifecycle fixture and the
  new ownership fixture. The fixture proves exact durable/runtime classification, collision-safe
  owners, owner-only modes, durable-wins restart behavior, linked-legacy failure, legacy byte
  preservation, workdir isolation, and no source mutation. Its local Node child required the
  narrowly reviewed host execution because the workspace sandbox rejected that child process;
- the authoritative Pipeon package harness passed its repository, SDK, package-runtime, host-MCP,
  and extended code-server split contracts. Shell syntax, focused ShellCheck excluding only the
  inherited warning classes, maintained-reference/classification scans, generic engine-boundary
  scans, and protected generated-extension hash proof passed.

Tests created only isolated `/tmp` roots and the existing ignored package-test products. No real
durable or legacy package state was inspected, imported, migrated, rewritten, deleted, or cleaned;
no Cursor, VS Code, code-server, Docker, VM, or external dependency/resource ran. Exact inherited
protected aggregates remained
`72a2dc05e04d8a18a9b01a8d92a0ac289773d44cc7238804def8eed525d07d93` for provider/learning,
`58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc` for VM identity, and
`537a3b9e1eee2f3a758e1e712abab5fd8d7bdd0db67d696afc7bc9481524f466` for public Go/CLI
package-state compatibility. The ignored Pipeon extension product remained at SHA-256
`4683442e439a7c9ec921e42c2b3ed276c0327181987bcd8ec2b6b3ddd96425bf`.

Public `PackageStateDir`, `dockpipe get`/`scope`, `DOCKPIPE_PACKAGE_STATE_DIR`, public Go/shell/editor
SDKs, canonical docs, workflows, schemas, clean/rebuild, `DOCKPIPE_PACKAGES_ROOT`, generated-state
and prune behavior, staging, commit, push, worktree, and successor creation remain unchanged and
separately gated. The engine/package boundary is preserved: core owns only generic validated
private-directory plumbing; the IDE and Pipeon packages own all state classification, mappings,
mounts, and fixtures.

Terminal disposition: `completed`. Public package-state cutover, canonical guidance, cleanup, and
every later step remain separately gated. No successor was created.

### Rank 2 Public Durable Package-State Cutover — 2026-08-14

Implemented only ordered step 5 in the saved dirty checkout. Before mutation the delegated anchors
matched exactly: branch `js/dev`, HEAD `6752dce7c0540d68cb95e1f718ba0998ea0eae35`,
upstream relation 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 77 tracked dirty paths,
34 untracked paths, full status SHA-256
`e36e869d108ef3e2b163736ee5c8cc2cbf5466a251d24dcc43c90238ad6f2d64`, and this
1,133-line task record at SHA-256
`6ba2a7e311a708589ef7705f984fcb7b839a8cefb0e84c04e6142dd546497756`.
The protected Cursor/VS Code aggregate matched
`788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`.

Public package state now resolves to cohort 1's collision-safe, owner-only durable project/package
directory:

- `PackageStateDir`, `dockpipe get package_state_dir`, `dockpipe scope --package`,
  `DOCKPIPE_PACKAGE_STATE_DIR`, package/workflow injection, and shell/Go/editor SDK guidance
  converge on the exact trimmed, case-preserving durable owner. Public suffixes are boundary
  validated; empty/default identity, traversal, links/reparse points, and unsafe injected roots fail
  closed. Workdir refresh discards inherited package-state authority.
- A validated `package.yml` may declare
  `package_state.compatibility_import: package-owned` plus exact `owner_ids`. DockPipe propagates
  the selected manifest through generic package/workflow script context. The engine contains no
  maintained package names or cohort mappings: declared mixed owners leave the legacy public tree
  untouched for their package-owned exact cohort importers, while undeclared third-party owners
  conservatively import the complete validated legacy scope.
- Whole-tree compatibility import uses owner-only sibling temporaries, sorted
  path/type/size/SHA-256 and source-object evidence, a second source inventory, synchronized files
  and directories, atomic publication, and durable-wins divergence reporting. Restart recovers only
  byte-proven incomplete/ready temporaries. Legacy bytes are never rewritten, deleted, or linked to
  durable state, and no rename/copy/project heuristic merges identities.
- DorkPipe, IDE, and Pipeon manifests own their exact maintained public compatibility IDs. The
  unchanged Cursor/VS Code resolver scripts continue passing the public durable token into the
  generic cohort importer; their fabricated ownership fixture proves that only declared durable
  settings/configuration/extensions import while server products, caches, logs, and markers remain
  disposable.

Ordinary `dockpipe clean` remains limited to the checkout compiled store and no longer follows an
external `DOCKPIPE_PACKAGES_ROOT`. `dockpipe rebuild` no longer calls clean: its separately
reported compiled-store reset preserves the override compatibility, validates the exact target,
rejects filesystem roots, user home, workdir/ancestors, durable roots, links/reparse points, special
files, and filesystem substitutions, then builds. No real clean, rebuild, or package-store reset was
run.

Canonical package-model, path-scope, package-authoring, CLI, workflow, artifact, shell SDK, Go SDK,
editor completion, and affected first-party package guidance now agree on durable package state
versus disposable package runtime.

Focused offline validation used only fabricated durable, legacy, runtime, home, cache, workdir,
package-store, and temporary roots:

- current focused infrastructure and application tests passed under the race detector, covering
  whole/selected import, durable-wins, interruption recovery, collision/rename isolation,
  permissions, override validation, traversal, manifest propagation, CLI/get/scope/environment,
  and isolated clean/rebuild reset behavior;
- full domain and Go SDK packages, focused package-hook/application tests, DorkPipe statepaths race,
  Go vet, Go formatting, shell syntax, focused ShellCheck, editor JavaScript syntax, and maintained
  reference/classification checks passed;
- the fabricated shell SDK, IDE ownership, DorkPipe runtime, and Pipeon runtime package contracts
  passed. Windows/amd64 and macOS/amd64 compile-only checks passed for infrastructure and
  application; produced binaries stayed under `/tmp` and were not executed;
- the protected Cursor/VS Code resolver aggregate remained
  `788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`; the VM identity
  implementation/test aggregate remained
  `58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.
  Provider/learning migration paths were outside this slice's owned diff and remained untouched.

No real durable or legacy package state was inspected, imported, migrated, rewritten, deleted, or
cleaned. No clean/rebuild, Cursor, VS Code, code-server, Docker, VM, external dependency/resource,
generated-state/prune removal, clean widening, staging, commit, push, worktree, or successor action
occurred. Generated-state history and all inherited unrelated bytes remain authoritative. The
engine/package boundary is preserved: core owns only generic identity, declaration, import, boundary,
and compiled-store reset primitives; packages own maintained identities and exact cohort mappings.

Terminal disposition: `completed`. Ordered step 6 generated-state/prune removal and ordinary clean
expansion remain separately gated. No successor was created.



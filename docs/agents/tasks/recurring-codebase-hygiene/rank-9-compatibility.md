## Rank 9 Compatibility Retirement Ledger — 2026-08-14

Implemented only the compatibility retirement inventory in the saved dirty checkout. Before
mutation the delegated anchors matched exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 101
tracked dirty paths, 40 untracked paths, full status SHA-256
`492b7aa103bf2d1fda7bfcf05149b3a3852bfea0b999b3bc54137363c1a62b7e`, and this
1,281-line task record at SHA-256
`183f9036bcaa59af899a3cf876c4725d0b22bc72b2a1c3dd6994949d693aa615`.

The maintained contract now has one bounded retirement source of truth:

- `docs/compatibility-retirement.md` records 43 separately addressable surfaces across engine
  configuration and authored YAML, schema/editor mirrors, CLI commands and flags, legacy layouts,
  durable/recovery readers, Git session modes, core environment contracts, and first-party package
  aliases and schemas;
- each row records the exact surface, owner/boundary and active source anchors, current public
  promise, callers/fixtures, first source commit or unproven age, missing removal evidence,
  disposition, and the exact separately gated proof needed before that one surface can retire;
- six proof profiles (`CONFIG`, `CLI`, `LAYOUT`, `STATE`, `PACKAGE`, and `RECOVERY`) keep public
  behavior, state inventory, migration, cleanup, recovery, and package ownership separate;
- active-supported synonyms and architecture fields, forward-compatibility behavior, rejected old
  keys, third-party generated schemas, interoperability inputs, current layouts, and research-only
  future designs are explicitly classified so broad searches cannot silently turn them into debt or
  removal authority;
- `docs/agents/core/compatibility-retirement.md`, `docs/agents/index.yaml`, and `docs/README.md`
  provide the compressed task route and canonical maintainer entrypoint without duplicating the
  ledger.

Focused offline validation used only read operations and isolated Go caches under `/tmp`:

- all 43 IDs are unique and complete, every row has all four fields plus a separately gated proof,
  and 191 parsed exact file/line anchors resolve within 113 current files;
- canonical/compressed local links, repository JSON, and the agent router YAML passed parsing;
- focused domain, application, and infrastructure tests passed for compile-root aliases, vault
  precedence, step aliases/scopes, CLI flags/aliases, workflow layout/runtime normalization,
  legacy run-policy reads, and output-root propagation;
- focused DorkPipe planner/worker, DorkPipe MCP tier, and VM executor compatibility tests passed.
  The first Go commands used `GOSUMDB=off`, which prevented verification of the already selected
  cached Go toolchain; the exact tests passed offline after retaining local checksum verification
  with `GOPROXY=off`. The VM package required `GOWORK=off` from its nested module root before its
  focused tests compiled and passed;
- `git diff --check`, Markdown whitespace/structure, maintained reference scans, protected hashes,
  and final dirty ownership passed. The final status has only the two clean tracked router/reference
  edits plus the two new ledger documents as rank-9 additions; the task record remains its inherited
  untracked path. Final status is 103 tracked dirty and 42 untracked paths with SHA-256
  `ed7063b6ba62292bbc8109623e8d425a8a6d64fd599c132904322a0879cee772`;
- the protected Cursor/VS Code resolver-tree aggregate remained
  `788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`, and the VM identity
  implementation/test aggregate remained
  `58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.

No alias, layout, flag, schema, state, package, workflow, or runtime behavior was deprecated,
warned on, migrated, removed, or changed. No generated state, clean/rebuild, real state inspection,
live service, Docker, VM, network, external resource, staging, commit, push, worktree, or successor
action occurred. Generated artifacts are limited to isolated `/tmp` Go caches. Package/engine
boundaries remain unchanged: the ledger names existing owners and requires package-specific
retirement to remain package-owned.

Terminal disposition: `completed`. Every compatibility retirement remains separately gated. No
successor was created.

## CR-001 `compile.resolvers` Retirement — 2026-08-14

Retired only compatibility ledger entry `CR-001` in the saved dirty checkout. The product owner
established `v0.6.0` as DockPipe's first supported release and confirmed that unreleased development
use of `dockpipe.config.json` `compile.resolvers` does not require compatibility. Local release
policy ships tagged versions from `master`; all available tags stop at `v0.5.8`, the old key first
appeared later in commit `297af0cd`, and no maintained or generated project config uses it. This
completed the previously blocked downstream support/version-floor predicate without inferring a
general downstream absence policy.

Before mutation the checkout still matched the delegated anchors: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 103 tracked
dirty paths, 42 untracked paths, full status SHA-256
`ed7063b6ba62292bbc8109623e8d425a8a6d64fd599c132904322a0879cee772`, this
1,344-line task record at SHA-256
`764e7bdf7a644f7fe22c25f79a3cbbd75587c3da013d2a9cf3ed35727c09de7a`, and the
compatibility ledger at SHA-256
`0b0adf4b9b1aea0506974631b8fa17b2ccbaf7ce1c9b468cede9d9b0fb930b45`.

The bounded retirement removes the `DockpipeCompileConfig.Resolvers` field and resolver-root merge,
rejects that exact JSON key with an error naming `compile.workflows`, and leaves all other unknown
project keys forward-compatible. Canonical `compile.workflows` roots retain configured ordering,
deduplication, flat core-resolver discovery, and the separate `compile.bundles` merge. Package-model
documentation, resolver CLI help, and the VS Code project-config key mirror now expose only the
canonical key. No other compatibility ledger entry changed behavior.

Focused offline verification used isolated `/tmp` Go caches and fabricated project roots. Domain,
command, infrastructure, and focused resolver-compile application tests passed; a native `/tmp` CLI
accepted canonical `compile.workflows`, produced the expected resolver tarball, and explicitly
rejected `compile.resolvers`. The complete application package did not reach a terminal result after
two minutes and was stopped; its CR-001-focused tests had already passed. Editor JavaScript syntax,
Go formatting, 5,920-file JSON caller scanning, 43-row ledger structure, local Markdown links,
focused reference scans, and `git diff --check` passed. Final dirty ownership is 108 tracked paths
and 42 untracked paths with status SHA-256
`d099a470bad79a2f850a766676782540f3ea061b2e7141660568e07dc234a5f3`. The protected
Cursor/VS Code resolver-tree aggregate remains
`788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`; the VM identity
implementation/test aggregate remains
`58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.

No generated checkout state, clean/rebuild, package-state, layout migration, live service, Docker,
VM, network, external resource, staging, commit, push, worktree, or successor action occurred. The
engine/package boundary remains intact: generic project-config parsing owns the exact rejection and
packages continue to consume the canonical generic compile-root contract.

Terminal disposition: `completed`. CR-002 through CR-043 remain separately gated. No successor was
created.

## CR-002 `compile.bundles` Retirement — 2026-08-15

Retired only compatibility ledger entry `CR-002` in the saved dirty checkout. The product owner
established `v0.6.0` as DockPipe's first supported release and confirmed that unreleased development
use of `dockpipe.config.json` `compile.bundles` does not require compatibility. The old key first
appeared after tag `v0.5.8` in commit `297af0cd`, and no authored or generated JSON project config
uses it. The separately governed CLI command `dockpipe package compile bundles` and the
`--with-bundles` / `--skip-bundles` compatibility no-ops remain supported and unchanged.

Before mutation the checkout matched the delegated anchors exactly: branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 108 tracked
dirty paths, 42 untracked paths, full status SHA-256
`d099a470bad79a2f850a766676782540f3ea061b2e7141660568e07dc234a5f3`, this task
record at SHA-256 `5edcd2efe0906ec43d8f394c1aa7832496761241620cb48b28c7c6a538271907`,
and the compatibility ledger at SHA-256
`833e17491f353ad64ef87f3e9bf0108000b80712190ed38ca017058dd20638bf`.

The bounded retirement removes the `DockpipeCompileConfig.Bundles` field, workflow-root merge,
dedicated bundle-root cache/API, and dedicated script/Dockerfile consumption. Generic project-config
parsing now rejects that exact key with an error naming `compile.workflows`; all other unknown
project keys remain forward-compatible. `compile.workflows` is the sole configured root authority
for workflow compilation, resolver discovery, PipeLang materialization, logical script lookup, and
Dockerfile lookup. The former old-key path fixture now uses the canonical key.

Maintained coverage proves canonical config loading, exact old-key rejection, rejection when both
canonical and old keys are present rather than precedence or merging, unrelated unknown-key
acceptance, ordered existing/missing root resolution, workflow/resolver/script/Dockerfile consumers,
and a complete fabricated `compile all` that emits core, resolver, and workflow tarballs. Canonical
package-model, package/compile/PipeLang help, and VS Code project-config completion/hover now expose
only `compile.workflows` and `compile.core_from`. The separate CLI alias and no-op help remain
present. Ledger entry `CR-002` is `retired_before_v0.6.0`; `CR-023` and `CR-024` remain unchanged.

Focused offline verification used the reviewed cached Go 1.25.0 toolchain, `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, the existing read-only module cache, and isolated `/tmp` build/temp
caches:

- full `src/lib/domain`, focused compile-root/script/Dockerfile infrastructure, focused config and
  fabricated compile-all application tests, and `src/cmd` passed; focused domain/infrastructure
  `go vet` also passed;
- an isolated `/tmp` native CLI accepted canonical `compile.workflows` plus unrelated future keys,
  emitted the expected workflow tarball, displayed the retained `package compile bundles` alias and
  both retained no-op flags, and rejected a both-key config with the exact
  `compile.bundles is not supported; use compile.workflows` error before creating `bin/`;
- editor JavaScript syntax, Go formatting, 5,992-file local JSON caller scanning, 43-row ledger
  structure, focused source/docs/help/editor reference scans, and `git diff --check` passed;
- two setup-only test attempts stopped before compilation: the first selected the host Go 1.22
  binary under `GOTOOLCHAIN=local`, and the second used an empty isolated module cache. Neither
  changed repository state; validation then passed with the reviewed cached Go 1.25.0 binary and
  existing offline module cache.

Final dirty ownership is 116 tracked paths and 42 untracked paths with status SHA-256
`c53dfeb9f49650fcb26bf8c2e82374851e570629ee49d8d65c5222837c488751`. The
compatibility ledger is SHA-256
`14517144ee400971d6e35899e91129cdd2e339d76adf1d726ed88f9a4bffcdf0`. The
protected Cursor/VS Code resolver-tree aggregate remains
`788ceeaf6866b8b41a9303f08417068bea07177caa375e9bf890edf9e280ff20`; the VM identity
implementation/test aggregate remains
`58a2c4607fadb95af64f2a675aab05275223209d82095838984515cdb2a999fc`.

No generated checkout state, clean/rebuild, package-state, layout migration, live service, Docker,
VM, network, external resource, staging, commit, push, worktree, successor, or other compatibility
retirement occurred. Generated artifacts are limited to isolated `/tmp` caches, fixture trees,
tarballs, and the native validation CLI. The engine/package boundary remains intact: generic config
parsing and compile-root resolution own the retirement, while packages continue to consume the
canonical generic contract.

Terminal disposition: `completed`. CR-003 through CR-043 remain separately gated, including the
independent CLI alias/no-op entries. No successor was created.


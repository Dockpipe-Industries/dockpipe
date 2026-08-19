## Application SDK Scope-Projection Boundary Audit — 2026-08-15

Completed only the approved read-only rank-2 boundary audit in the saved dirty checkout. Before
this documentation edit, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 57 default-collapsed untracked paths, and status SHA-256
`948183f18ab980683467508feebecf1ab8204a38806a6c28e328ea895cf955f2` matched the delegated
anchors exactly. This task record was
`f17cee2fae4aa46d9d2d9a397e2dfe8deb0dbb479f0f97004a4adaf08702688c`; the aggregate digest
of the sorted per-file SHA-256 inventory for every file below `src/` was
`ce7de671ec813e61702c5139c09b076c9b206db82143bbb2763efbb585e75388`. The protected
ignored Python cache remained a regular file at mode `0664`, size 8,408, and SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

### Exact declaration and seam inventory

`src/lib/application/sdk_cmd.go` is 721 physical lines with SHA-256
`9a2fd29919c62b894f29bf315b0451bf6b7b38be6504d41c4c69f4317807f6ff`. The destination
`src/lib/application/sdk_scope.go` is absent. Fourteen scope-owned declarations occupy five exact
source-order blocks totaling 353 lines; concatenating the five blocks in that order has SHA-256
`69c82b20579d5dbb6a78d500eb13bd8bcb38ea25e316dc63bdda35e83ee867f4`.

| Exact block, including each trailing declaration separator | Lines | SHA-256 |
| --- | ---: | --- |
| `scopeUsageText` | 57-93 (37) | `357f80a2ec0a7e18e0ef13a6746e6e6dea9d290753a7c4f06ca0446f13366425` |
| `cmdScope`, both scope types, and `printScopeObject` | 254-344 (91) | `31067b344b06ba6f3b1b1006c656f1527da2fa666afe349a88baec1ff04016a5` |
| `parseScopeArgs` | 360-396 (37) | `ac87e0b0ea9327f27df6f8dc61dd9453f50592e5b700704e8c31d604000c5601` |
| workflow/package/resolver projections through `resolveOutputRoot` | 437-601 (165) | `289348c6082e7d25088d4a89ad0dc6da1cc7e2813bc100481996571e29fff7da` |
| `sanitizeNamedScope` | 632-654 (23) | `48f7349d8b2ab862d73ce7fc0dc98772eb7e354838ac078b2255c912824be3ce` |

The declaration-level inventory is exact and includes the blank separator following each
declaration:

| Declaration | Lines | SHA-256 |
| --- | ---: | --- |
| `scopeUsageText` | 57-93 | `357f80a2ec0a7e18e0ef13a6746e6e6dea9d290753a7c4f06ca0446f13366425` |
| `cmdScope` | 254-313 | `baa3534481e232a76f9bd841c4fbc62fee95a1f9ad0ce1684199da61895734e3` |
| `scopeArgs` | 314-320 | `7be34000c4ec628121fb83672a0e0515bb28b96c84ce7e5b101954ced0c2fa69` |
| `scopeObject` | 321-335 | `407bc853f0bc9e2d7ce58c403a0c89b8fafcd998f03a11987d8116b633381adf` |
| `printScopeObject` | 336-344 | `d2c2149493ef54c351fac2aae2ffc63f9afda685d3c2bdab7f3a4e95d035c9c4` |
| `parseScopeArgs` | 360-396 | `ac87e0b0ea9327f27df6f8dc61dd9453f50592e5b700704e8c31d604000c5601` |
| `workflowScopeObject` | 437-482 | `1b834a99d678479e58ed9fe14d53844e8d6933a9eae1d2a9aa016ff85a1e131a` |
| `packageScopeObject` | 483-506 | `c674d47b9b6b5a034c4e0fb83832c3b875599056b986f2c86f9092a296478e24` |
| `resolveScopeRoot` | 507-522 | `d70fbe366d1ea724c99e852c5070f2b5e6095df9f56f606fd241353c56da69f4` |
| `resolverScopeValue` | 523-552 | `d5994d7ea7ac7d236c505ea1bf4b5405140211a302fef9e0c59250fd1f222bf1` |
| `workflowScopePath` | 553-565 | `9cfbeceb25844c67c680e215fa616c48d7ce15047c9b90c43d946ddd9d129c6f` |
| `resolveResolverHostPath` | 566-590 | `4a2e6ebee0f4f22092936be9f1fbaccbbed9cbebf07a17f3525734454299276d` |
| `resolveOutputRoot` | 591-601 | `134e78f168c9eec6034ec37de40989f3b00acd87b16499bc1d6d5fb0b9700e5e` |
| `sanitizeNamedScope` | 632-654 | `48f7349d8b2ab862d73ce7fc0dc98772eb7e354838ac078b2255c912824be3ce` |

The seam is non-contiguous for source-backed reasons, not because scope ownership is ambiguous:

- lines 94-253 are the 160-line `cmdSDK`/`cmdGet` block, SHA-256
  `fdc08ae774d7959367f3ec8e411645d8ea63bc2fdd2e64cecd78f85a6230bfea`;
- lines 345-359 are the 15-line shell-environment command, SHA-256
  `0ec0cd016162acf3332f8867d07ee2026eed781187c28382f06b7871269edda8`;
- lines 397-436 are the 40-line shared SDK workdir parser/normalizer block, SHA-256
  `e539104880c1c3cab31268423e20134d00bd266f84a18c97130b32fdef6394f7`;
- lines 602-631 are the 30-line event-log/event-index path block shared by `cmdGet` and the
  workflow scope object, SHA-256
  `456b474c381f50577219d2106693e7074117412488176c36089b1385bf298fb9`;
- lines 655-721 are the 67-line binary selection, child-process fallback, field normalization,
  shell-SDK path, and quoting tail, SHA-256
  `10248002e8f3248f079ffc4ba7ce9774ade155b24246198e835cea90f083ae7a`.

The exact shared dependencies that must stay out of the move are `resolveSDKWorkdir`, used by both
scope parsing and `parseSDKWorkdirFlag`; `resolveEventLogPath` and `resolveEventIndexPath`, used by
both `cmdGet` and `workflowScopeObject`; `resolveDockpipeBinForSDK`, used by `cmdGet`, both scope
objects, and the child-process fallback; and `normalizeGetField`, used by `cmdGet` and scope
dispatch/projection. `workflowArtifactRoot` and `sanitizeWorkflowStateScope` remain in
`state_env.go`, where workflow env and CI-artifact callers also use them. The broadly shared
`firstNonEmpty` remains in `workflow_env.go`. Shell-only `cmdSDKShellEnv`, `resolveShellSDKPath`,
and `shellQuote`, plus `parseSDKWorkdirFlag`, child-binary selection, and repo-local binary lookup,
remain in `sdk_cmd.go`.

The future child needs `encoding/json`, `fmt`, `os`, `path/filepath`, `strings`, and
`infrastructure`. Only the existing `encoding/json` import is exclusive to the 353-line move; its
line has SHA-256 `926ebc76318f711b7e6fce77071eb1a689adb7f244f8cbde070b2852db74b02e`.
The parent retains every other import, including `os/exec`, so no dependency direction or package
boundary changes.

### Production callers and direct proof

`Run` in `run.go` is the sole external production caller of `cmdScope`; the complete SDK/get/scope
dispatch excerpt at lines 157-165 has SHA-256
`9f7596971b622778119ab5d404c7fbfaf27b5ace38f8ef23b481acb08b5221ee`. Every other moved
declaration is called only inside the candidate graph. The graph in turn calls the shared helpers
listed above plus generic infrastructure state, package-state, resolver-profile, and safe state-path
helpers. Repository package/workflow scripts consume the public `scope` CLI forms for artifacts,
workflow artifacts, package state, and resolver auth, but none is a Go declaration caller and none
needs to move or change.

All seven colocated SDK tests are in the unchanged 371-line `sdk_cmd_test.go`, SHA-256
`0315da7df102b84ed12f38925ffadd4e0b0e01acddf2bd5e692b302e71be3887`. The three direct
scope tests are `TestCmdScopeWorkflowAndPackageObjects`, `TestCmdScopeResolverAuthFields`, and
`TestResolveResolverHostPathUsesUserHomeWhenHomeEnvMissing`. The four shared-seam tests are
`TestCmdGetStateFields`, `TestCmdGetStateDirNormalizesGitBashWindowsWorkdir`,
`TestResolveDockpipeBinForSDKPrefersRepoLocalExe`, and
`TestResolveDockpipeBinForChildProcessPrefersCurrentExecutable`. No test or caller move is needed.

### Exact separately gated successor

Exactly one successor is selected: **TASK-035 application SDK scope-projection responsibility
split**.

- **Result:** create `src/lib/application/sdk_scope.go`; move byte-for-byte, in original order, only
  the five exact blocks and fourteen declarations inventoried above. Remove only those source
  blocks and the now-unused `encoding/json` import from `sdk_cmd.go`. Keep all shared helpers,
  dispatch, SDK/get/shell behavior, callers, and tests in place. The destination is an in-package
  responsibility sibling, not a new package or abstraction.
- **Behavior and boundary:** preserve every name, signature, declaration body, JSON tag and byte,
  CLI/help/error/log/environment/path byte, workdir and Git-Bash normalization, source/artifact/
  workflow/package/resolver projection, resolver-auth lookup/default, durable package-state import
  and divergence behavior, safe suffix join, package/runtime/state separation, child-binary
  selection, cross-platform behavior, and generic engine/package boundary.
- **Exclusions:** no shared-helper, caller, test, SDK/get/shell, state-env, workflow-env, child-binary,
  package-state, resolver-profile, infrastructure, Domain, schema, configuration, manifest,
  compatibility, cleanup/migration, generated, runtime, or durable-state edit; no new package or
  abstraction; no behavior/API/CLI/help/error/log/environment/path change; no live, network,
  Docker, VM, staging, commit, push, worktree, or additional successor.
- **Required proof:** retain every per-block and per-declaration hash plus the 353-line aggregate;
  reconstruct the original `sdk_cmd.go` byte-for-byte at SHA-256
  `9a2fd29919c62b894f29bf315b0451bf6b7b38be6504d41c4c69f4317807f6ff` by restoring the
  exclusive import and reinserting the five blocks at their exact anchors; prove each moved
  declaration has one definition and `Run` remains the sole external caller; run all seven named
  SDK tests together; compile the Linux and Windows/amd64 application test packages without
  executing the Windows binary; and verify Go formatting, import ownership, offline
  `go list -mod=readonly`, `git diff --check`, task/status/full-`src` reconstruction, every
  unrelated source byte, and protected-cache preservation.

Audit validation used cached Go 1.25.0 with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`,
`GOWORK=off`, and isolated `/tmp` build/temp roots. The first candidate module cache was correctly
rejected because pinned `golang.org/x/sys` v0.28.0 was absent. The existing complete matching cache
at `/tmp/dockpipe-vm-source-lane-a.fL2D8H1F/tmp/validation-mod` then passed
`go list -mod=readonly` for application and both internal leaves, Domain, Infrastructure and both
child packages without network fallback or an import cycle. Structural declaration, import,
caller, direct-test, CLI-consumer, destination-absence, exact-hash, dirty-state, and protected-cache
checks passed. No test suite was run because this audit changed no source behavior; the exact
successor owns focused and cross-platform compile proof. Generated audit files exist only under
`/tmp/dockpipe-task035-sdk-audit-01a007b3`.

A no-index comparison against the preserved task preimage shows one insertion-only hunk with no
removed or replaced line. Final status and full-`src` digests remain the exact anchored values
above; repository `git diff --check` and the TASK-only no-index whitespace check both pass.

This audit changes no Go source, test, caller, helper, schema, configuration, manifest, API, CLI,
package, workflow, generated, ignored, runtime, durable, or external byte. The generic
engine/package boundary remains intact.

Terminal disposition: `completed`. The selected successor was not implemented, handed off, or
created. It requires the exact separate user response
`Approve next slice: TASK-035 application SDK scope-projection responsibility split` before any
fresh-task execution.

### SDK scope-projection responsibility split completion

Completed only the approved TASK-035 application SDK scope-projection responsibility split in the
saved dirty checkout. Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, upstream relation 0 behind/1 ahead,
`stash@{0}` object `26ea507907550d2449dc6f9c81b9942bd52d8629`, no staged files, 133 tracked
dirty paths, 57 default-collapsed untracked paths, and status SHA-256
`948183f18ab980683467508feebecf1ab8204a38806a6c28e328ea895cf955f2` matched the delegated
anchors exactly. This task record was
`59c73057892fe0099dfc5d236c491d03e7125f31a8f020f6780d8ff327d307bf`; the complete sorted
per-file `src` inventory, `sdk_cmd.go`, absent destination, exact candidate blocks/declarations,
and protected ignored Python cache also matched every handed-off digest, type, mode, and size.

Created `src/lib/application/sdk_scope.go` and moved byte-for-byte, in original order, only the
five audited scope blocks and fourteen declarations. `sdk_cmd.go` lost only those blocks and its
exclusive `encoding/json` import. Every shared workdir, event-path, binary, normalization,
workflow-artifact, workflow-state-scope, and shell helper remains in its audited owner; dispatch,
callers, tests, SDK/get/shell behavior, infrastructure and Domain code, package-state and resolver
semantics, APIs, JSON/CLI/help/error/log/environment/path bytes, and cross-platform behavior remain
unchanged. The destination is an in-package responsibility sibling and introduces no package,
abstraction, or dependency-direction change.

The five blocks reconstructed with each original declaration separator retain their exact
37/91/37/165/23-line SHA-256 values
`357f80a2ec0a7e18e0ef13a6746e6e6dea9d290753a7c4f06ca0446f13366425`,
`31067b344b06ba6f3b1b1006c656f1527da2fa666afe349a88baec1ff04016a5`,
`ac87e0b0ea9327f27df6f8dc61dd9453f50592e5b700704e8c31d604000c5601`,
`289348c6082e7d25088d4a89ad0dc6da1cc7e2813bc100481996571e29fff7da`, and
`48f7349d8b2ab862d73ce7fc0dc98772eb7e354838ac078b2255c912824be3ce`.
Their 353-line source-order aggregate remains
`69c82b20579d5dbb6a78d500eb13bd8bcb38ea25e316dc63bdda35e83ee867f4`, and all fourteen
declarations retain the exact individual hashes in the audit table above. Reconstructing the
original parent by restoring `encoding/json` and reinserting those blocks at the five audited
anchors reproduces the original 721-line `sdk_cmd.go` SHA-256
`9a2fd29919c62b894f29bf315b0451bf6b7b38be6504d41c4c69f4317807f6ff`.
Each moved declaration has exactly one definition, and `Run` remains the sole external production
caller of `cmdScope`.

Focused offline validation used a cached Go 1.25.0 binary and the existing complete matching module
cache with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`, and isolated `/tmp`
build/temp roots:

- `go list -mod=readonly` passed for application and both internal leaves, Domain, Infrastructure,
  and both infrastructure child packages without network fallback;
- all seven exact `sdk_cmd_test.go` tests were selected together: five passed, while the
  Windows-only Git-Bash normalization case and the environment-gated `os.UserHomeDir` fallback
  reported their existing skips;
- Linux/amd64 and Windows/amd64 application test packages compiled through non-executing
  `go test -c` commands; the Windows binary was not run;
- `gofmt`, import/declaration/helper/caller ownership, all block/declaration hashes, parent and
  full-`src` reconstruction, dirty ownership, `git diff --check`, and protected-cache checks passed.

After the move, excluding only the new destination reconstructs the original 133-tracked/57-
untracked status at SHA-256
`948183f18ab980683467508feebecf1ab8204a38806a6c28e328ea895cf955f2`. Excluding that destination
and substituting the reconstructed parent reproduces the original sorted per-file `src` inventory
SHA-256 `ce7de671ec813e61702c5139c09b076c9b206db82143bbb2763efbb585e75388`, proving every
unrelated source byte unchanged. The protected cache remains a regular mode-`0664`, 8,408-byte file
at SHA-256 `25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.
Generated validation artifacts exist only under `/tmp/dockpipe-task035-sdk-split-01a007c7`.

No shared helper, caller, test, SDK/get/shell, state/workflow environment, infrastructure, Domain,
package-state/resolver-profile, behavior, schema, configuration, manifest, compatibility,
cleanup/migration, generated/runtime/durable, live service, network, Docker/VM resource, staging,
commit, push, worktree, handoff, or successor was touched. The generic engine/package boundary
remains intact.

Terminal disposition: `completed`. No successor was selected, created, or implemented; any later
TASK-035 slice remains separately gated.


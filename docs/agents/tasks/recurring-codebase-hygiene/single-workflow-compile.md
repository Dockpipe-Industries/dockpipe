## Single-Workflow Package Compilation Responsibility Split — 2026-08-15

Lifecycle contract terminal state: `completed`. Fresh-task creation supplied
`execution_authority: approved_task_creation` for only the selected single-workflow compilation
split. One-shot readiness remained `required: false`, `status: not_applicable`. No excluded live,
generated, durable, cleanup, compatibility, migration, staging, Git, network, Docker, VM, handoff,
or successor action was taken.

Before mutation, branch `js/dev`, HEAD
`6752dce7c0540d68cb95e1f718ba0998ea0eae35`, origin `js/dev`
`115fb785f0c4dc789ffce8c93c79fe147ea58229`, 0 behind/1 ahead, `stash@{0}` object
`26ea507907550d2449dc6f9c81b9942bd52d8629`, zero staged files, 133 tracked dirty paths, 58
default-collapsed untracked paths, and status SHA-256
`5e8f318e6f1b291e022bf78f2fe2ed0eb616b71fb19940a36d27f26a88752176` matched the delegated
anchors. This TASK was SHA-256
`48d579b28c488423615cadc75690800a0ee48c3e78d2307f48891078bf8ef1fc`,
`package_compile.go` was 869 lines at SHA-256
`03373e9034f34e6f0772270edf16e38d6bc9268c6983b9e08a1bacd16022d085`, the destination was
absent, the exact lines 82–328 block was
`ff02ffb88b02b3c693260e5cd7a25525241c37fa21ae6ff4441a1dbd82e58ef3`, and the exclusive YAML
import line was `9a31bac62ac062ba0fa459b0e216501cece27853cb1db62ebaebf40761ddf1d8`.
The complete sorted per-file `src` inventory remained
`801fd0f559acef3647f733d753fc0b6d03997ce2ddcf95a8f43e627b3596a2d3`; the protected ignored
cache remained a regular mode-`0664`, 8,408-byte file at SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`.

Created `src/lib/application/package_compile_workflow.go` as an in-package sibling. It contains
only the package/import header plus the three declarations `cmdPackageCompileWorkflow`,
`compileWorkflowOne`, and `runWorkflowCompileHooks` in original order. Its lines 17–263 reproduce
the exact 247-line block SHA-256
`ff02ffb88b02b3c693260e5cd7a25525241c37fa21ae6ff4441a1dbd82e58ef3`; its YAML import line
retains SHA-256 `9a31bac62ac062ba0fa459b0e216501cece27853cb1db62ebaebf40761ddf1d8`.
The formatted destination is 263 lines at SHA-256
`54f8debc2ae81af52556f05b8899c1fba361db099fd65518bc89504612f49ab9`.
The parent is 619 lines at SHA-256
`58a6c99b4534982877a3221085dccba041915df20c6c891c1ca88d5f55c680ff`; formatting removed only
the redundant separators made adjacent by the exact block/import removal. Reinserting the exact
block, separator, and exclusive import reconstructs the original parent byte-for-byte at
`03373e9034f34e6f0772270edf16e38d6bc9268c6983b9e08a1bacd16022d085`.

Each moved declaration has exactly one production definition. The dispatcher and batch callers
remain in `package_compile.go`; dependency-closure and PipeLang materialization callers remain in
`package_compile_closure.go` and `pipelang_materialize.go`. Those four caller lines retain combined
SHA-256 `9abccf7ed9c84f2b64c0a9758ecfca3c1a82cc7e0b509a8fb3f446ada8fddad8`.
The sorted inventory of every unrelated `src` file remains
`639c25d374dda1e8c71dcf68de6f8f38805ecdf1684097c968dd1e164a476790`; substituting the
original parent hash and omitting the new sibling reconstructs the complete pre-split `src`
inventory at `801fd0f559acef3647f733d753fc0b6d03997ce2ddcf95a8f43e627b3596a2d3`.

Cached Go 1.25.0 validation used `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`,
and slice-owned `/tmp` module/build/temp roots. The 13 named direct tests in the required-proof
list passed together (`ok dockpipe/src/lib/application`, 96.387s). Two preceding setup probes
stopped before compilation because the inherited module cache was read-only/incomplete; the
passing invocation used the already cached dependency archives copied into the isolated module
cache and made no network request. Linux/amd64 and Windows/amd64 application test packages both
compiled with `go test -c`; the Windows binary was not executed. Offline `go list -mod=readonly`
resolved application and both internal leaves, Domain, Infrastructure and both child packages,
and PipeLang. Go 1.25 `gofmt -d` emitted no diff, import ownership is exact, and repository
`git diff --check` passes.

The compile-only binaries exist only under
`/tmp/dockpipe-task035-workflow-split-01a007fa/`: Linux SHA-256
`6b3f04992b75c1ad7ce2d34963920519961921fa8e902b6121a8f21b9db15bf4` and Windows/amd64
SHA-256 `80d27be8c83eb4939e130bfb430adc780b71911c8b714756d5030b08032910e2`.
No generated checkout artifact was created. Final checkout status is zero staged, 133 tracked
dirty paths and 59 default-collapsed untracked paths at SHA-256
`3c8fe5a18ed7ea866a615fa32524a687ec0f20a2caa25570942059f637d12a05`; removing only the new
sibling status entry reconstructs the inherited status digest
`5e8f318e6f1b291e022bf78f2fe2ed0eb616b71fb19940a36d27f26a88752176`.
The protected cache retains its anchored type, mode, size, and hash. Removing only this terminal
section reconstructs the pre-implementation TASK at SHA-256
`48d579b28c488423615cadc75690800a0ee48c3e78d2307f48891078bf8ef1fc`.

This was a byte-preserving in-package responsibility move: no behavior, API, CLI, help, error,
log, operation-result, environment, path, parsing, freshness/rebuild, legacy-store, cache,
staging, hook, PipeLang, runtime/security/image artifact, manifest, namespace, tarball,
cross-platform, helper, caller, test, package, or authored-surface contract changed. The generic
engine/package boundary remains intact. No successor was selected, created, or implemented.


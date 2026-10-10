## Application/Domain/Infrastructure Package Topology — 2026-08-16

State: `completed`. Objective `TASK-035-package-topology` replaces flat shared namespaces with
cohesive Go packages while leaving root Application, Domain, and Infrastructure as public
facades/composition surfaces. The completed Domain vocabulary and Model/Domain boundary proof is
admitted unchanged. Automatic reversible checkpoints are authorized; behavior, schema, authored
surface, compatibility retirement, feature work, generated/durable/live state, migration, network,
Docker, VM, credentials, staging, commit, push, publication, worktree, external actions, and
unrelated cleanup remain excluded.

Admission matched the clean saved-checkout anchors: branch `js/dev`, HEAD
`4e71cdeeb2c43aeb43296e23afcb8dc94fe7af6e`, origin `js/dev`
`3fe03013e695b7b13b62221bd7ccbc6ad4334e00`, 0 behind/2 ahead, both inherited stash objects,
zero staged/unstaged/untracked paths, empty status SHA-256
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, and record SHA-256
`5434d637ef58900b245761cef4c09de2821d73c863cc6671e8b88375cf821349`. The root production
inventory was and remains 65 Application, 20 Domain, and 58 Infrastructure Go files. The protected
Python cache and the two inherited empty mode-`0664` checkout cache locks retain their admitted
type, mode, size, and hashes.

### Checkpoint 1 — session command owner

Selected and completed `application/internal/sessioncmd` as the sole owner of session list,
inspect, switch, checkpoint, publication, worker-lease command parsing, presentation, and the exact
usage surface. Its only public API is `Run(args []string) error`; the six-line root
`session_cmd.go` compatibility dispatcher preserves the parent caller. The child imports the
narrow `shellquote` and `textvalue` leaves plus Domain and Infrastructure, and does not import
parent Application. Runtime-owned Git checkpoint/publication/lease behavior remains in
Infrastructure and is only invoked by the command owner.

The parent `firstNonEmpty` compatibility helper now delegates to
`textvalue.FirstNonBlank`, whose direct test proves that selection preserves the original untrimmed
bytes. The shared POSIX quoting algorithm moved from the SDK command into the narrowly named
`application/internal/shellquote` owner; both SDK and session commands depend downward on its
tested `POSIX(string) string` API. No generic helper or command bucket was created.

Reversing only package/API qualification changes reconstructs the original 641-line
`session_cmd.go` exactly at SHA-256
`779685f4c086a9b933c64a2eb0e4bd610b74b5150ec76f1d7acc5f1ca1806487`. Removing the one added
dispatch/worker-validation test and child-local stdout capture, then reversing the same package/API
qualification, reconstructs the original 258-line test exactly at SHA-256
`1bfe6f1ef41676c405a33b1324d706626ba3a66c9c20a4c3fdbcf736c1810860`.
Current owner hashes are `acd4ced308ed36c1c0bb2dde34ea109e15c03109d1d5d4d4553681d4ebfb205f`
for `session.go` and `4efd3e92ac50ca9576d8f36cfa2cb187fbf91f8f263241d0571e79a460f608da`
for its colocated tests.

Cached-offline Go 1.25.0 proof passed for sessioncmd, shellquote, and textvalue; all three moved
session integration tests; the new exact usage/unknown/worker-validation test; and affected SDK,
scope, and first-non-blank parent callers. Offline list and vet passed for Application, the three
affected leaves, Domain, Infrastructure, and Model owners through the admitted `/tmp`-only
compatibility module/cache. Linux and Windows/amd64 compile-only binaries passed for both
sessioncmd and Application; Windows binaries were not executed. Their SHA-256 values are
`a94b7617cef75d512ea0cc9b49b956988aa1c7acc78a16b0ff12ea549ae94c5e` /
`3ba4c81c0d913ca3291cc5abfe7b2b90cf67d849c3b81eb9f5944dca29d5df45` for sessioncmd and
`f86b0ca93a46ef33d61b9aaef4dba2c7a3cdcd20db5bd00a901858e2d9b070c8` /
`c3b19459775feb752d1ec88144713966943d2f6daeeecea17c657610a18c39d4` for Application.
`gofmt`, `git diff --check`, declaration uniqueness, dependency direction, and protected-state
checks pass. Verification artifacts exist only under `/tmp/dockpipe-task035-package-topology/`.

Topology selection after this checkpoint is pending a fresh caller/dependency/test audit. The
session owner is selected/completed; the previously completed Model wire owners remain selected and
complete; the existing Application internal leaves remain admitted downward owners. No additional
Application, Domain, or Infrastructure root cluster is selected merely from size.

### Checkpoint 2 — operation-result command owner

The live caller/test audit selected and completed `application/internal/resultcmd`. It solely owns
canonical operation-result parsing, validation, stderr rendering, event-log mirroring, environment
override/restore, and the exact help text behind `Run(args []string) error`. The root
`result_cmd.go` is a six-line dispatcher retained for the main Application command switch and its
existing dispatch test. The child imports only Infrastructure and does not import parent
Application. Existing parent integration tests plus a colocated validation test pass.

Reversing package/API qualification reconstructs the original implementation exactly at SHA-256
`d0e3d71b1ec014625f23d61ece2c7a3b448f2f2301c9f12f37ddf42f3e9589db`. Current implementation
and colocated-test hashes are `db53af6db65129546a12a17d7c9d265caaad5445e53c25ce23946e96bde967ac`
and `e405879ff9dc8460bf771ac48d0dda451251c4eb2b575598c8df3bdb49f81491`.
Cached-offline Go 1.25.0 focused tests, list, vet, and Linux/Windows compile-only proof pass for the
owner and parent Application. Binary hashes are
`56e80d279a2960ad0c70db02ad5c0eda6e77408dbe729e0ae163eade6be917e3` /
`a6f1018a92ce2f3ea54511f6424583f39f92bef6bf593cf6cefa91d99868851f` for resultcmd and
`27372a7e7374d12ae08c25d67f1b3cbaa28895dae4fcd905f1074dde80594453` /
`73e92ed25602bfd80bf8d1b49122496367fc8934e31263dc84b8290c1a062bdb` for Application. The first
vet setup stopped before analysis because its new `/tmp` `GOTMPDIR` did not yet exist; creating
that directory and rerunning the identical offline command passed. `gofmt`, diff, reconstruction,
declaration, dependency-direction, and protected-state checks pass.

The next selected low-risk vertical slice is the hidden internal-state command: its only production
caller is the parent dispatcher, its five direct tests cover durable-cohort, disposable runtime,
private-directory, collision, traversal, and link behavior, and its implementation depends only on
Infrastructure plus the standard library. Clone, runs, doctor, Windows, Terraform, package-image,
and test/build command surfaces remain under live API/caller audit and are not selected yet.

### Checkpoint 3 — hidden internal-state command owner

Selected and completed `application/internal/internalstatecmd` behind the sole public child API
`Run(args []string) error`; root Application retains only its six-line hidden-dispatch adapter. The
owner imports Infrastructure and the standard library, never parent Application. All five direct
tests moved beside the implementation with a test-local stdout capture and pass, covering exact
durable-cohort result fields, collision-safe disposable runtime paths, non-mutation, private modes,
malformed mappings, traversal, duplicate flags, and symlink rejection.

Reversing package/API qualification reconstructs the original implementation exactly at SHA-256
`7d7da5fa60bfbdda9ff548bd52707d04690cd2cc581ea7b78d2c470a4464d0cc`; removing only the
test-local capture and reversing the same qualification reconstructs the original tests exactly at
`c8a2aeebce5c1dc6ee73bb1218ea8d07bc9c86f64f981c6ac449842f5dfa9380`. Current implementation
and colocated-test hashes are `b0313afd8d672972e9ce290c9cccd1dab83a6f815b28291f095e093287f7d4c5`
and `f61b7b4b66d00aa9d12b7199f5a9782d3d227c34249e979a8a104c300fcf9401`.
Focused tests, offline list/vet, and Linux/Windows compile-only proof pass for the owner and parent;
binary hashes are `76355fdb7fcb04e55bd9a5eb3a50b27468fd4f6971d63b1c30c0a74cfe9157aa` /
`92594ffff471e9983e983e54d497686d10aa42c112a3cf22b86bb952931caf5f` and
`eafbd8565cf88771f3d220957358ba3c17183e643bbf24f3f1d50c771ebe4f33` /
`0eb1d01266b98bb78d0aa77620879c6aa923c3cfbacf4973b54f4a407a4ac277`.
No durable or disposable checkout state was changed by implementation or proof.

The live Application command audit now selects three remaining independent vertical owners in
order: doctor, clone, then runs. Each has one parent dispatch caller, no production dependency on a
parent Application type/helper, direct behavior tests, and a narrow `Run` boundary. Result and
internal-state are complete. Terraform remains unselected pending the explicit ownership decision
for shared `CliOpts` translation; package-image remains unselected pending ownership of the shared
Docker image-existence seam; package/workflow test and package-build remain coupled through parent
target/config/build adapters; catalog input projection remains coupled to parent catalog wire
records; project build/clean/rebuild remains one guarded destructive-safety transaction; run/steps,
dependencies, and strategy remain parent orchestration state. Windows command/bridge remains under
cross-platform public-facade audit rather than being selected by size.

The Domain map remains as admitted by the completed Model boundary: root Domain is the public
facade plus cohesive validation/value/workflow graph behavior; moving workflow graph/inject would
create a Domain-value cycle, and filesystem loaders require a separately designed Infrastructure
API. The Infrastructure audit currently finds its large durable-state, Git-session, Docker,
workflow-store, and path/layout families internally cohesive but mutually dependent on root helper
surfaces; no child is selected until an exact facade/API plan proves it will not import parent
Infrastructure. Existing `fetchinstall` and `packagebuild` remain valid downward children.

Next selected checkpoint: `application/internal/doctorcmd`.

### Checkpoint 4 — doctor command owner

Selected and completed `application/internal/doctorcmd` with `Run(args []string) error`; root
Application retains only the dispatcher facade. The child owns all doctor prerequisite checks,
operation-result records, exact help/output/error bytes, and its test seams. The former shared
`opLookPathFn` test seam was not a production ownership requirement: doctor now has the narrowly
owned `doctorOpLookPathFn`, while workflow secret injection retains its independent seam. The child
imports compileconfig, Domain, and Infrastructure downward and never parent Application.

Both direct doctor tests moved beside the owner and pass. Reverse transformation exactly rebuilds
the original implementation at SHA-256
`e7008a06bd40969e1142fe1fe96b1e63b9685af5e92f99bcb3ff1531ae925f1d` and its tests at
`2fde29cbe6a64fa5496107514be82df4b31c01f62f0528fc0b5dee85f9144b02`. Current implementation
and test hashes are `18c06256479a443e3d96819d73d10ad7631997e881dcab5acd5b2ea6622a9185`
and `5107d3d5b94bea071caaafd2c0d7b3e4bbedd25cf26cd953792d2b1e6f37c126`.
Offline list/vet and Linux/Windows compile-only proof pass for doctorcmd and Application. Binary
hashes are `b9f1a781f95ca04ed6fd508294134197e8cfe8cee88640c46a88916ac1a3eec6` /
`4bf8b1a57ee48052da27f6e9c7fe43cc99cde9a0b3075e92c477869fed9e38b6` and
`89a1ef50fc40f0c83240066513f75e80c002ea0c408bde45d630bb1bf25e0ab3` /
`cc4df12288437bb4d4a4eb1bf2ac4cf4286144183d6936e907b6c5f77cb47ddc`.
`gofmt`, diff, declaration, reconstruction, and dependency-direction checks pass.

Next selected checkpoint: `application/internal/clonecmd`.

### Checkpoint 5 — clone command owner

Selected and completed `application/internal/clonecmd` with the narrow `Run(args []string) error`
API and a root dispatch facade. The child owns compiled-workflow selection, manifest clone policy,
destination safety, extraction, copying, operation-result logging, and exact help/error bytes. Its
only former parent helper dependency was `copyDir`; the owner now imports the existing downward
`treecopy.Copy` primitive that already backs that exact parent facade. It otherwise imports Domain,
Infrastructure, and packagebuild, never parent Application.

The existing two parent integration tests still prove compile-to-clone success and facade denial.
A colocated owner test independently constructs the denied compiled tarball and proves direct API
policy enforcement. Reversing package/API qualification and the exact treecopy facade substitution
reconstructs the original implementation at SHA-256
`9ce00aea58029ac81bbdc8abd30a182f8c4240435631bfbf034ab1bbd6874710`. Current implementation
and test hashes are `1306fbcfd1225ecccab9b28d20198c2ba35fa7a634b6efe986b7908806233057`
and `592fc775666e52d39377e41b9463f7aff97183c58b23607f1758499b32919c21`.
Focused owner/caller tests, offline list/vet, and Linux/Windows compile-only proof pass; binary
hashes are `d90780c4b8ec418c224ec72063a3f0c806d034a71b048ec1a09d4668542e5394` /
`37009e9fb5474adb096008325e0cd764e14911d9d8a1e02e9ba86fedef50649e` and
`41e02e1f86eebe92ea8f4eef6f915f695bedc5217644482fd2ade5df67ae90b1` /
`a0bbddbb2002dfd4450c97d27dd45523f48fce058c06fd4a5d3cc887a5dd4bc9`.

Next selected checkpoint: `application/internal/runscmd`.

### Checkpoint 6 — runs inspection command owner

Selected and completed `application/internal/runscmd` with `Run(args []string) error`; the parent
file is only a dispatcher. The child owns host-run/policy/event selection, filtering, indexing,
text/JSON rendering, exact errors, and the exact runs usage text moved from the root usage
composition file. Its former `firstNonEmpty` dependency now uses the byte-preserving
`textvalue.FirstNonBlank` leaf. The child otherwise imports only Infrastructure and never parent
Application.

All existing `TestCmdRuns*` parent integration tests and two colocated owner tests pass. Removing
the moved usage block and reversing package/API/helper qualification reconstructs the original
command exactly at SHA-256
`e90376f0ccd14ea8cd52219f06e75495a42295d9441ee078c4ea6c2533bd0f7d`; the moved usage block
itself remains exact at SHA-256
`33374a7df7ebc8a0ea67300732fe0b5f72a48e5bfc2ae567a79ca995ac907123`. Current implementation
and test hashes are `541707dbcfcba8d2306e86f6d32c2fdf56f40e18ce27d8e16aec4e3889ba7a98`
and `546fc37419b58ab8db2d7dedb4382bcd1adafa9df1bb98bfb419c375371140ad`.
Offline list/vet and Linux/Windows compile-only proof pass for runscmd and Application. Binary
hashes are `0132b9e3daea5c4ccbab7ae25de2f259d41d6a8553da28305b0ebbe6a25664fe` /
`55599424e3022bf40c2225b3acd0ecda12740108c2d0c74c6717492a0aad5f17` and
`e8a04506caedb50a1a60b0325a1a9248fa0cfed8d96ffc99a71a87c9d8614147` /
`19abddb9b78ed83d662ed7593b411198ab880e8f80ae2cbcc28976f86ea3dbc7`.

The Infrastructure audit selects one low-risk independent owner:
`infrastructure/envfile` for dotenv file/byte parsing. It depends only on the standard library,
has a direct parsing test, and can retain root `ParseEnvFile`/`ParseEnvBytes` compatibility
wrappers without any reverse import. Operation result/event is not selected: it depends on root
terminal/spinner/time seams (`StartLineSpinner`, `fdInt`, `isTerminalDockerFn`, and
`timeNowDockerFn`), so extraction requires a named lifecycle-dependency API rather than a move.
Removal safety similarly depends on the durable-state device identity seam. Next selected
checkpoint: `infrastructure/envfile`.

### Checkpoint 7 — environment-file parser owner

Selected and completed `infrastructure/envfile`. The standard-library-only child owns file, byte,
and reader parsing; root Infrastructure retains source-compatible `ParseEnvFile` and
`ParseEnvBytes` wrappers. The original file implementation, byte implementation, and test each
reconstruct exactly at SHA-256
`0ee59a9e3082837ce8ceec40c72673e7287b6f6d10e4513ae13906fca27095bf`,
`7f1cee82831cbf410dbca4b64249ccfea2541d7d9494824de04119b013b99748`, and
`e69466fb327eed7a483be3043ae62450aa4513ff5c2a9f5f1c0337df3c87ec55`. A new colocated byte
parser test covers the previously untested public wrapper path. Current child hashes are
`10dc22d2f1c32d5dbe7b544fa75351f4192936c2904c988dfa672bace23c43e3`,
`1ec2238a4d22e9f1c703ef61f26f7d7e35032aa1cd710d86bc2307d6ba2abe89`, and
`17bba6c88de74d6eee07bad7c03a1d50998b1a52c05ef460d5c274f82a80e06f`.

Focused child and Application caller tests, offline list/vet, and Linux/Windows compile-only proof
pass for envfile, root Infrastructure, and Application. Binary hashes are
`c76dd2315c18bc9b1c473f3a842f2e9d688930c3fcf7c891e7b4dab831cdb13d` /
`bb891f51cff446dc5dc6b98982a45dab42653e68b64f8f8ede1a4fa52f24a464`,
`2a8ac7ab95f554c5aa3a4bf91801ebd316e6f91d59493e469ad8bc76bb421aa8` /
`53e095cceb94d358f5552eeb0e0138b4218843ba0d3acec0a90913a1c71d06d6`, and
`363388de02cffca28250d253e1f7a3a41ed873308f9e844fa16c95d1767dd510` /
`f44a712f05d418e4725276ef35b5de316461aa890ee7c59c581b2478a003029e`.
The Infrastructure root production inventory is now 57 files.

The final simple-owner pass selects `infrastructure/sourcemtime`: its three source freshness/path
selection functions are standard-library-only, directly tested, and consumed through a stable
three-function root API by packagecompile. Resolver/strategy file loading remains a deliberately
small cohesive root surface because splitting its shared assignment grammar would either make the
strategy owner depend on resolver semantics or introduce the prohibited generic parser bucket.
Next selected checkpoint: `infrastructure/sourcemtime`.

### Checkpoint 8 — source freshness owner

Selected and completed `infrastructure/sourcemtime`. The standard-library-only child owns maximum
source-tree modification time, source-versus-reference freshness, and newest-path selection; root
Infrastructure preserves the three exported wrappers consumed by packagecompile. Implementation
and tests reconstruct exactly at SHA-256
`bd19bdf53cff3dc53a6d317f0fa81a054bfc6fdbc8531ddd868b565ec04804cf` and
`e5dfacfd7522cd74e42204bd8428f569f194b3d347bfeb53d3072841deecb28b`.
Current child hashes are `bb8704d726b0fb9175f94833d32c360c5dc047388eab421bd601c7c44689db40`
and `deabbe6c4e23be79f2af61d46395b38f2c7d07dd29601b9c65ddf18e71b07c9d`.
Focused owner proof, offline list/vet, and Linux/Windows compile-only proof pass for sourcemtime,
root Infrastructure, and packagecompile. Binary hashes are
`d984d39aeaea253cc9b6fa842bb87e061c7129e090227e946bf719862cb1fc89` /
`c1028c8acfb6b4046826b37f3452a5f683701dc8d70b814ed63f30ed7524d2f8`,
`b0416eba78cd2eeab2733cf9899febcd36d71553eb77ec78f71ff1d27e1ae446` /
`9166f19370e35a7b53bba375a109c9be4a595318dcc04edaf34d05458d03dfae`, and
`7813451d1d76d217820fa9f73162fe6cf568aa903c2161f29119c4618bfc51b2` /
`368d7f7c0ce2703c8d0473a05a76a8b0a5cb8d5a60c9ee215b081a17689d7b5d`.

### Checkpoint 9 — operation-record owner and fetchinstall direction repair

Selected and completed `infrastructure/operationrecord` as the sole owner of operation result
types/options, stable stderr records, progress heartbeats, JSONL events and indexes, configured event
mirroring, and line-spinner lifecycle. Root Infrastructure retains source-compatible aliases,
constants, and wrappers for every exported operation API. The child owns independent
`time.Now`/`term.IsTerminal` seams plus its own safe file-descriptor conversion with the same
production defaults; it imports only the standard library and `golang.org/x/term`, never parent
Infrastructure or Application.

The two direct operation test files moved beside the owner. Reverse package/seam qualification
reconstructs the original operation-result implementation and test exactly at SHA-256
`c6c24c67bfdb2c0d9733c3be68dc6833d4bbec23bc968e206c296c2960a34270` and
`c2f9de22858446696a383a0ed7e8eccc7598655b24277890ae1a0448dcbcc0fc`; the event implementation
and test reconstruct at `cd077033cf8d9b8e44bd8a862f983a363e66a2532ab4e518540245a7d89ca693`
and `1e57ead7b7cf1280de7256b21175598ff9226742356a5ff67b81cee91fcf510d`; removing only the local
terminal seam and reversing qualification reconstructs the spinner exactly at
`0bff86ace66ae27e8ab10e17389e12500769d499c8c21e019dbf0919f8b3ad87`. Current owner hashes are
`7c30c9920a037bb322be4e683c1517b22f91ef0412d550f5b70f02255f1061ea`,
`c609b78d9c66ee5ee38f0a8a89ea8e3fdecf3db0a700271917b3196de70ede95`, and
`af3dd95715c440294307ddbb626ed7f128c40dc3e483a7088ec0b7cd71357573` for result, event, and
spinner, with colocated-test hashes
`6e569569bdf687f3ec6003d463d5d613d2586970b6ca22c5a999c0d6f2662130` and
`ec2273725630c5636305e706265227c09357117377dd52852ee6b6b99edc86eb`. Root facade hashes are
`c9d4faaf79319a24e6ff8a7ed9e24ffafab976e0c47bfeb6040afe6a2338bacd`,
`08a4ef90e960aa43cc3019f73edbce57553264d7e5420f95451567ce60f80fbc`, and
`3ce914ae8ec03d8d23d7107bae61920a0c088fef8ee8c52beb2482d2acd28cec`.

`fetchinstall` now imports the sibling operationrecord owner directly for the unchanged
resolve/download/extract calls. Reversing only that import and qualification reconstructs its
original source exactly at SHA-256
`eed6773b138ae693f2fb8550d1c941c0a8389acc6504b003753fa996f6f61179`; the current source is
`aa8512dd5ad807b07ef8205aa5c40f156d6082ab187f7e101297376b626a56cb`. The install CLI proof now
also asserts the exact nested JSONL unit/status order and retained checksum/result IDs while its
existing stderr assertions retain the resolve/download/extract, status, mode, checksum, result,
and destination bytes; the updated test is
`e6bee745889efd3573f3605553ddfa0555ea4125bfd50a719b0abe3519cfeae2`.

Cached-offline Go 1.25.0 proof passed for all operationrecord tests, all fetchinstall tests including
the end-to-end listener path, the install CLI dry-run and nested operation-result/event test, and
seven representative Docker operation-facade callers. The loopback tests first encountered or
skipped the expected Codex listener restriction, then passed under the narrow localhost-only host
test allowance. Offline list and vet passed for operationrecord, fetchinstall, root Infrastructure,
and Application using the admitted `/tmp` compatibility modfile and a copied `/tmp` module cache.
The first two setup attempts stopped before compilation because the host Go 1.22 launcher was
selected and the checkout-external module cache was read-only; neither was a source failure.

Linux and Windows/amd64 compile-only proof passed for operationrecord, fetchinstall, root
Infrastructure, and Application; Windows binaries were not executed. Binary hashes are
`d1b71646f60e1c1684e75ca23ae00846bf1b7d8c8e44942104c66122aa3bbdf3` /
`1bab4dab56df151dab63cdaf43ae7c8491d836055e0dc24383926cc0b1baa830`,
`96e1546dcfe45b283433f18a55cd2a003b1d7f4696ce92d7614e2569b1e2c0a4` /
`cd4765bfc820c82382bd91e479fbcb4e572244c9d87e4c020ce23ee3546a8ac3`,
`db4a39fe94f24d01438836d0bd7a32d2f933a561b263c888cdd1815cba51c648` /
`f164fa86232158ee0480869cf88beb1cbde04067b05da126f9dc4d0e3e499adc`, and
`b66a64d775ac38ff7e7656d03a71554c077e4acb4cb4b57dc213f8e9c0a3e19f` /
`ccf8e75d9c9dc4349fbf9b3acaa93808b4fee747cb71b69712173e2672fa4611`.
`gofmt`, `git diff --check`, implementation uniqueness, and dependency-direction checks pass. Root
production counts remain 65 Application, 20 Domain, and 57 Infrastructure files. All new proof
artifacts are under `/tmp/dockpipe-task035-package-topology/`; no generated or durable checkout
state changed, and the protected cache and both inherited empty locks remain exact.

### Current topology map and terminal proof boundary

Selected/completed Application children are the admitted compileconfig, imageartifact,
operationids, packagecompile, packagescript, packageversion, pipelangmaterialize, runtimepolicy,
textvalue, treecopy, and wslbridge owners plus this objective's sessioncmd, shellquote, resultcmd,
internalstatecmd, doctorcmd, clonecmd, and runscmd owners. Their root command files are thin
dispatch/facade surfaces. Run/run-steps/strategy/workflow-environment files remain cohesive parent
orchestration state; catalog input projection is blocked on parent catalog record ownership;
Terraform is blocked on `CliOpts` translation ownership; Windows command/bridge is blocked on its
public facades plus the host-bash `windowsGoosFn` seam; package images is blocked on the shared
Docker image-existence seam; package/workflow test and build commands remain coupled to parent
target/config/build adapters; project build/clean/rebuild is one guarded destructive-safety
transaction. SDK, flags, usage, and subcommand files are root composition/public facades.

Domain remains the admitted public facade plus cohesive workflow graph, validation, normalization,
value, import/merge, resolver, and strategy behavior. Model/dependency, model/package,
model/project, and model/runtimeartifact remain the selected wire owners. Workflow graph/inject is
blocked on a Domain-value cycle; filesystem loaders require a separately owned Infrastructure API.

Selected/completed Infrastructure children are packagebuild, envfile, sourcemtime, and
operationrecord. `fetchinstall` is a cohesive downward child and now imports only its sibling
operationrecord owner; no Infrastructure child imports parent Infrastructure or Application.
Durable-state, Git-session, Docker, workflow-store, and path/layout families remain cohesive public
transactions/composition; removal safety is blocked on durable device identity; resolver/strategy
assignment loading stays a small cohesive root surface rather than introducing a generic parser
bucket. No additional Infrastructure owner is selected.

The selected/unselected map is converged and every selected owner has a narrow API, colocated proof,
and acyclic direction. No further implementation seam is selected or preauthorized by this map.

Terminal verification is complete. Cached-offline Go 1.25.0 `go list -mod=readonly` and `go vet`
passed across all 31 `src/lib/...` packages. The same full package set compiled as test packages on
Linux/amd64 and Windows/amd64; the broad runs selected no test bodies and did not execute Windows
binaries, so unrelated stateful tests could not mutate excluded generated/durable checkout state.
Focused behavior proof remains the operationrecord owner suite, the full fetchinstall suite with its
end-to-end localhost path, the install CLI nested stderr/JSONL test, and representative root-facade
Docker callers recorded above. `gofmt -d` is empty for every checkpoint-9 source/test file,
`git diff --check` passes, and terminal dependency-direction searches are empty.

Terminal anchors remain branch `js/dev`, HEAD
`4e71cdeeb2c43aeb43296e23afcb8dc94fe7af6e`, origin `js/dev`
`3fe03013e695b7b13b62221bd7ccbc6ad4334e00`, 0 behind/2 ahead, both inherited stash objects, and
zero staged paths. Before this terminal record, the 37-path default-collapsed status was SHA-256
`b8bcf471bec25939e93dd5a29913aa3572f939091d4761a966cad0dc0255b8a4` and the task record was
`3e7559794e98ed0c7528ab7d14df5a6877d65f16e546b86d6f0095e65553a2b3`. The protected ignored
Python cache remains regular mode-`0664`, 8,408 bytes at SHA-256
`25632b7dad1855a5692207c9d4fc22b2e57203b2f9219182679d622bf3b63e10`; the two inherited empty
mode-`0664` checkout locks remain SHA-256 `e3b0c442...`. Verification artifacts exist only under
the admitted `/tmp/dockpipe-task035-package-topology/` tree. No gate or receiver transport,
worktree, cleanup, generated/durable-state mutation, compatibility change, commit, push,
publication, or external action was used. Terminal disposition: `completed`.

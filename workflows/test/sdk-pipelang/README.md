# PipeLang SDK consumer qualification

This Linux workflow materializes `Dockpipe.SDK` from an explicitly selected
Dockpipe CLI and builds a source consumer against an explicitly pinned installed
PipeLang SDK. It records exact tool, bundle and consumer identities. Native
compilation and execution run sequentially inside the qualified aggregate and
workload containment units.

Export `PRODUCT_DOCKPIPE_BIN`, `PIPELANG_SDK_PREFIX`,
`PIPELANG_SDK_MANIFEST_SHA256`, `PIPELANG_CONTAINED_BIN`,
`PIPELANG_CONTAINMENT_PROBE`, `PIPELANG_CONTAINMENT_ADMISSION`, and
`PIPELANG_CONTAINMENT_ADMISSION_SHA256` with absolute tool/receipt paths and their
verified identities. Select the installed compiler profile explicitly when
required with `PIPELANG_CXX_RESOURCE_PROFILE`.

The embedded host control program defaults to one `PRODUCT_SANITIZERS=OFF`
profile. An inherited `PRODUCT_SANITIZERS=ON` or explicit `--var` override still
selects the sanitizer profile. Set `--var SDK_ALL_PROFILES=true` to run prepare
and prove sequentially for both `OFF` and `ON` in one workflow invocation. Do not
also lock `PRODUCT_SANITIZERS` with `--var` when requesting both: a step argument
cannot override a differing explicit caller binding.

The sanitizer run instruments the generated producer, SDK transport, bridge and
consumer, retaining full debug information and enabling ASan, UBSan and leak
detection. Neither profile changes installed SDK defaults. Existing scripts,
scopes and dotenv outputs retain their behavior; each prepare call creates a
fresh proof directory.

Build the controller explicitly using the selected SDK before running:

```sh
"$DOCKPIPE_BIN" workflow build-control workflows/test/sdk-pipelang/config.yml \
  --sdk "$PIPELANG_SDK_PREFIX" \
  --sdk-sha256 "$PIPELANG_SDK_MANIFEST_SHA256"

# Default: one profile, with the existing environment/--var precedence.
"$DOCKPIPE_BIN" --workflow-file workflows/test/sdk-pipelang/config.yml --no-vault

# Optional: OFF followed by ON through the same typed step declarations.
"$DOCKPIPE_BIN" --workflow-file workflows/test/sdk-pipelang/config.yml --no-vault \
  --var SDK_ALL_PROFILES=true
```

The orchestration conversion has native Linux normal/sanitizer proof; current
context-owner changes require native journey requalification. Build
and run use trusted native host code; the controller is not a sandbox. A missing,
stale or tampered controller artifact returns `build_required`, with no compiler
invoked during run. See [host control](../../../docs/workflows/workflow-yaml.md#inline-pipelang-host-control)
for typed inputs, synchronous calls and execution limits.

PipeLang owns workflow/package scope and lifecycle expectations, operation-handle
scope observation, foreign-ID rejection,
cancellation retaining custody, readiness before graceful interruption, single
active-operation admission, explicit forced cleanup and scope truncation checks.
The native witness independently inspects receipts and exact argv boundaries,
checks complete tree closure, and exercises malformed, duplicate, truncated and
Unicode scope decoding. Fresh proof directories preserve all unit receipts,
identity hashes and a `qualification.result` marker on success.

This qualifies the dedicated Linux headless transport. Native Windows/macOS,
GUI/shared child reaping, installed package workflow execution, and cancellation
during an already-running scope service remain separate qualification work.

The Windows `assets/fault-harness` is a separate source-owned OS-effects suite.
Its five fixed cases pass against the production SDK broker on the admitted
native host: exact argument bytes, actual BREAK receipt and custody, root exit
with live writers, 2 MiB overflow drainage, and separate-group exclusion. This
component proof does not establish broad product acceptance. Earlier real-CLI
Windows PowerShell BREAK/debugger failures are retained; the repaired host owner
now passes the original shared consumer's nine operations and codec controls.

The harness also builds the explicit test-only `sdk_lost_offer_helper` artifact.
Supply its absolute path as the optional fifth operand to `sdk_fault_probe.exe`,
after the native process fixture, fresh owned root, workflow selection and
production console helper. PipeLang then adds the lost-offer case and supplies
the aggregate watchdog budget. The helper records a fixed create-new
`lost-offer-child.bin` witness only after actual suspended child creation and
drops the private reply without resuming it. Source asserts the failure,
unknown-root receipt, retained ID and explicit forced custody closure, with an
independent cleanup deadline. No runtime fault switch is added to the production
SDK. This case and the additive partial-adoption and late-offer cases now pass
on the admitted native host. All eight source cases retain the default source
budget and independent cleanup. The late case requires a factual attempt after
the exact deadline; raw native receipts preserve the three unknown roots and
original startup failures after explicit forced custody closure. The caller
wrapper records identical console, standard-handle, code-page, membership and
handler-marker facts before/after each phase and after actual transport closure.
All eight cases and fifty observations pass together. Maximum inputs and broader
product qualification remain separate.

The unchanged shared real-CLI consumer's nine operations and codec controls pass
with the original `assets/fixtures/held/windows.yml` PowerShell 5 profile after
native host-context repair. An earlier additive `windows-pwsh.yml` profile also
passed with a task-private portable PowerShell 7 selection. No system installation
or default changes are required. Earlier debugger/closure failures remain
separate negative evidence. The final owner preserves early-return redirected
GUI/helper children while retaining cancellation through actual output EOF.

With a nonempty process-local `DOCKPIPE_QUALIFICATION_SENTINEL`, the source harness
adds `sdk.windows.command-capacity` and `sdk.windows.exact-child-inputs` before
selecting optional fault helpers. Rebuild the process fixture with its generic
argument-statistics and child-input observations. These two cases and the
original eight pass together at the default source budget: eleven raw receipts,
62 identical caller observations and independent cleanup for every case. Source
asserts the actual 32,766-character UTF-16 command boundary, one-character excess
refusal before custody, the independent UTF-8 argument bound, exact canonical
cwd/environment and exclusion of the actual ambient sentinel. Without that
explicit sentinel, the existing eight-case profile remains available.

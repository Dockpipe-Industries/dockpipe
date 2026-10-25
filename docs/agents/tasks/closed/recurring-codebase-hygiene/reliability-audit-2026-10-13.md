# Superseded reliability audit — 2026-10-13

Archived 2026-10-25. Findings and selected repairs below describe the original
snapshot; they are not current work instructions. The
[current audit](../../recurring-codebase-hygiene/reliability-audit.md) owns repaired
and still-open disposition. Preserve historical evidence without recreating retired
core installation paths.

The retired `fetchinstall` core-replacement findings below no longer identify a
live implementation. The broad unbounded-archive description is also superseded
by the existing shared 2 GiB decoded-byte limit; member/allocation capacity still
needs its own review. Silent incomplete extraction is repaired above. Older Git,
host shutdown and workflow-import concerns remain open as described above; the
publication-script item is outside this `src/lib` audit.

Baseline: `f2efb71e3bded6c7c0ca22637b149314918a875d` on `js/dev`.
Scope: execution, package/archive handling, authentication, deployment and untrusted
workflow parsing. This is a focused risk review, not a complete security audit.
The [coding standard](../../../../coding-style.md) is canonical policy. Findings below
are open repair candidates; this audit authorizes no implementation or publication.
The separately requested launcher pairing correction does not resolve these items.

Severity: **P1** is a reproducible destructive/resource-exhaustion path with a
credible input trigger; **P2** is a significant conditional hang, malformed-success
or recovery gap. Ranking reflects impact and reachability, not file size or an
assumption that all loops or recursion are defects.

## Prioritized repairs

### 1. P1 — Failed core replacement removes the previous install

Evidence: `src/lib/infrastructure/fetchinstall/fetchinstall.go` (historical, retired source):148–160
removes `templates/core` before extraction and post-install verification. The
application calls this installer from
[install.go](../../../../../src/lib/application/install.go):148–156.
A digest can authenticate malformed bytes; it does not establish archive validity.

A bounded fake-HTTP fixture supplied a checksum-matching invalid gzip replacement.
The operation returned an error, and the previous installation's sentinel was gone.
No network or real installation was touched. A failed upgrade can therefore remove
a working legacy core tree. Catalog package installation uses a different path.

Repair first: extract and validate into a sibling temporary tree, then publish
with a recoverable replacement protocol appropriate to supported platforms. Keep
the previous tree on download, extraction, validation and publication failure.
Test malformed gzip, invalid entries, failed writes/rename and interrupted recovery;
assert old bytes survive and a successful replacement is complete.

### 2. P1 — Legacy/local archive consumers lack aggregate resource budgets

Evidence: `src/lib/infrastructure/fetchinstall/fetchinstall.go` (historical, retired source):378–396
reads the complete HTTP body; :400–465 extracts without expanded-byte/member bounds;
:518–522 allocates from the advertised member size and reads the extracted file
again. Its default HTTP client **does** have a five-minute timeout (:108–110).
This finding concerns resource growth, not an absent HTTP timeout.

[packagebuild/tarball_read.go](../../../../../src/lib/infrastructure/packagebuild/tarball_read.go):16–49
uses `io.ReadAll` for a selected member; :53–91 accumulates member paths.
[packagebuild/extract.go](../../../../../src/lib/infrastructure/packagebuild/extract.go):16–77
copies expanded members without a total budget. These are reached by local/tarball
workflow loading and extraction in
[workflow_tarball.go](../../../../../src/lib/infrastructure/workflow_tarball.go):20–26,159,190.
Large or highly compressed supplied archives can exhaust memory/disk before a
useful error. No exhaustion experiment was run.

Repair second: establish shared archive contracts for compressed/expanded bytes,
member count, per-member allocation and cancellation. Apply them to every entry
path, including lookup and listing; stop before expensive work and remove partial
outputs. Reuse existing policy where appropriate rather than copying validators.
Test boundary and over-boundary fixtures, dishonest headers, compression expansion,
truncated trailers and cancellation using small injectable test budgets.

### 3. P2 — Invalid archive members can produce successful incomplete extraction

Evidence: [packagebuild/extract.go](../../../../../src/lib/infrastructure/packagebuild/extract.go):41–42,74–75
silently skips invalid paths and unsupported entry types. Its cache helper
(:108–112) treats a successful return as completed extraction.
A one-member fixture containing `../rejected.txt` returned success and an empty
destination. The fixture did **not** write outside the destination; the demonstrated
problem is malformed-success, not a proven traversal exploit.

Repair with the archive work: reject disallowed entries explicitly and publish no
completion marker. Distinguish intentionally supported exclusions from malformed
input. Test unsafe paths, unsupported links/types and failure after valid members;
assert callers never consume/cache an incomplete package as valid.

### 4. P2 — Host-script stop has no escalation or pipe-drain deadline

Evidence: [prescript.go](../../../../../src/lib/infrastructure/prescript.go):70–129
starts the script, forwards signals and waits for output-copy goroutines.
[prescript_wait_unix.go](../../../../../src/lib/infrastructure/prescript_wait_unix.go):20–39
forwards signals only to the immediate process, ignores signal errors and waits
without escalation. A script that ignores termination, or a descendant holding an
output pipe, can prevent requested shutdown from completing.

Repair next: define graceful stop, bounded process-tree termination and bounded
pipe drain under the existing process owner. Preserve legitimate long-running
workflows; do not impose an arbitrary total runtime. Test an ignored signal and
a descendant retaining a pipe with a bounded test harness. Static evidence only;
no hanging process was deliberately launched during this audit.

### 5. P2 — Workflow import depth does not bound total parsing work

Evidence: [config.go](../../../../../src/lib/infrastructure/config.go):22–27
reads whole files. [workflow_imports.go](../../../../../src/lib/domain/workflow_imports.go):11,64–66
already enforces depth 32, and :113–116 rejects cycles. However, :117–138 re-reads
and expands repeated imports without a cumulative read/byte/expanded-step budget.
A fixture with nine unique documents at depth eight made **510 reads** and produced
**256 steps**. This is bounded proof of amplification below the existing depth
limit, not a claim of unbounded recursion or a production OOM reproduction.

Repair: define aggregate parse/expansion budgets and bounded file reads, retaining
the existing depth/cycle protections. Memoize immutable parse work only if import
semantics permit it; memoization alone does not bound duplicated output steps.
Test repeated DAG imports, wide graphs, cycles, exact depth and cumulative limits.
Keep this loader concern separate from planned PipeLang runtime fuel semantics.

### 6. P2 — Git lifecycle subprocesses lack operation cancellation and output bounds

Evidence: [git_runtime_session.go](../../../../../src/lib/infrastructure/git_runtime_session.go):1126–1145
uses `exec.Command` with `Output`/`CombinedOutput`. Sync merge (:533) and publish
push (:614) use these helpers. A stuck hook, credential helper or transport can
block the lifecycle, and retained output grows with child output. Existing Git
policy checks do not supply a command duration/output contract.

Repair: carry caller cancellation and operation-specific deadlines through the
runtime-owned Git client, bound captured diagnostics and process-tree cleanup,
and preserve actionable error classification. Reconcile remote state after an
ambiguous push before retrying. Test hung helpers, excessive output, cancellation
and response-loss recovery using a fake Git executable. No real remote write is
needed to prove these cases; this audit performed none.

### 7. P2 — Publication subprocesses lack an overall local duration/output contract

Evidence: [publish-r2.py](../../../../../workflows/publish/r2/assets/scripts/publish-r2.py):24,28–29,36–40
runs AWS commands without a subprocess timeout; metadata reads use unbounded
captured output. Provider socket/retry settings or an outer CI job timeout do not
establish bounded completion and cleanup for this individual operation. A hung
credential helper or a large response can stall or consume publisher resources.

Repair: set a documented publication/operation budget, bounded metadata and
redacted diagnostics, child cleanup and an explicit uncertain-write outcome.
Preserve immutable publication and forward-version checks. Test a hung helper,
oversized metadata and a write whose response is lost; inspect remote state before
any retry. Broader partial-publication recovery should be reviewed in that slice,
not silently treated as solved by adding a timeout.

## Existing protections and limits of the review

- [packagecatalog/install.go](../../../../../src/lib/infrastructure/packagecatalog/install.go):21–23,71–99,103–168
  already caps download/expanded bytes and members, checks cancellation, validates
  archive contents/digest and publishes a temporary download by rename. Do not
  describe all package paths as unbounded or replace these checks with weaker ones.
- [secretenv/resolve.go](../../../../../src/lib/infrastructure/secretenv/resolve.go):32–38,104–119,140–155
  bounds secret output, uses two-minute resolution/five-minute authentication
  contexts and a one-second process wait delay. This reviewed auth seam is a
  positive example, not evidence that every provider hook has been audited.
- [remote/client.go](../../../../../src/lib/infrastructure/remote/client.go):32–36,62
  bounds HTTP duration, headers and body. [remote/retry.go](../../../../../src/lib/infrastructure/remote/retry.go)
  bounds attempts and uses cancellable backoff. Diagnostic truncation explicitly
  marked by the remote executor is distinct from silent successful result truncation.
- PipeLang's foundation allows recursion with declared logical depth/step budgets.
  That is a design contract, not proof those budgets exist on every execution path.
- Fixture results above came from a task-local Go harness with temporary files and
  a fake HTTP transport; source review supports the remaining findings. No native
  Mac process failure, resource exhaustion, live authentication or provider fault
  injection was performed. Full test success would not invalidate these gaps.

Implement one bounded repair at a time, starting with preservation of the old core
install, then shared archive enforcement. Re-admit the current revision before
turning this dated audit into implementation work.

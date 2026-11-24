# Windows SDK process stimulus

This native executable supplies bounded OS behavior for source-owned SDK tests.
It contains no product cases, assertion selection, pass accounting or policy.
Build it on the admitted Windows host using the selected materialized SDK:

```powershell
cmake -S <fixture-source> -B <fixture-build> -G Ninja `
  -DDOCKPIPE_SOURCE_SDK_DESCRIPTOR=<absolute-library.json> `
  -DDOCKPIPE_SOURCE_SDK_SHA256=<descriptor-sha256>
cmake --build <fixture-build> --target sdk_windows_process_fixture
```

Select the built native executable as the SDK client's executable. Use any
nonempty workflow-file selection and supply these operands in the request's
`arguments` list; the fixture reads them only after the SDK's `--` separator:

| Arguments | Actual OS behavior |
| --- | --- |
| `echo`, followed by arbitrary arguments | Emit each UTF-8 operand as `byte-count:operand` plus LF, emit `SDK_STDERR` plus LF on stderr, then exit 23. Empty operands, Unicode, quotes and trailing backslashes remain observable. |
| `hold`, milliseconds | Register a real console break handler, emit `SDK_READY` plus LF, then wait for break or deadline. Only an actual `CTRL_BREAK_EVENT` emits `SDK_BREAK_RECEIVED` plus LF and exits 130; timeout exits 124. |
| `ignore`, milliseconds | Emit readiness, report any actual break with `SDK_BREAK_RECEIVED`, and remain live until the deadline. |
| `root-exit`, milliseconds | Start a suspended descendant with inherited job custody, resume it, and exit zero. The descendant retains output writers and remains live until its deadline. |
| `group-child`, milliseconds | Start a descendant in another console group under the same inherited job. The root responds to actual console break; the child remains live and reports any incorrectly delivered break with `SDK_CHILD_BREAK_RECEIVED`. |
| `flood`, bytes | Write up to 16 MiB of stdout through the real pipe, with a 10-second cooperative loop deadline. Pipe backpressure can block a write, so the owning SDK job and source cleanup budget remain required. |

Wait operands are 0..10000ms. Descendants emit `SDK_CHILD_READY` plus LF and
ignore break termination while reporting any delivered break. Child liveness alone
does not establish signal exclusion; source tests must inspect the child receipt.
All fixture-created descendants start suspended, inherit the SDK's
non-breakaway job, and are resumed only after job membership is observed.

Source tests must independently check unknown/foreign IDs, concurrent refusal,
out-of-root workdirs, cancellation retention, exact argument bytes, real break
delivery, root-exit/descendant-live distinction, capture overflow without invented
EOF, and explicit force disposal. The fixture's exit code is never an assertion
or acceptance result. Native nested-job assignment failure and incompatible
directory-handle/reparse controls need separately admitted OS setup; this fixture
does not fake those failures or bypass job admission.

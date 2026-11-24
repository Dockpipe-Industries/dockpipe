# Local CLI lifecycle lane

This opt-in driver runs fresh host workflows through the selected, hash-pinned
installed Linux CLI. It uses private HOME/XDG directories and project roots,
two concurrent tasks, generated markers, task outputs and a blocking join.
Public `runs observe` records must match the task PID and kernel start token;
public `runs events` and the joined result bind logs and output to the run.

The cancellation probe sends SIGTERM only to the CLI. Both tasks must acknowledge
the forwarded cancellation, their children must disappear, and a second public
observation must show no replay. Supervisor process-group cleanup is a safety
fallback and prevents a passing case. Deadlines, oversized logs, unexpected
exits and unknown remaining owned processes cannot count as cancellation.

The restart-recovery probe kills only the original workflow CLI after both
workers are observed running. Fresh public observers must retain the original
execution and process identities and report the closed workers as unverified
when their owner could not record an exit. Both workers must acknowledge the
parent-death signal, leave no live descendants, and start only once without
running the join or producing a successful workflow result.

The failure, cancellation, and restart-recovery component cases can pass. The happy installed case
remains blocked because `--detach` is a container contract: launching a host CLI
under this supervisor is not product detach. Its supported log/result/closure
assertions are still evaluated; their failure makes the case fail.

Invoke using the shared `--request ABS_JSON --output ABS_DIR` contract. Register
`drivers/lifecycle.py`, dependencies `assets/scripts/contracts.py` and
`assets/scripts/process.py`, feature `workflow.lifecycle`, scenarios `happy`,
`failure`, `cancellation`, `restart-recovery`, and layers `installed_e2e`, `component`. Configuration
requires `artifact` (absolute pinned CLI path). Optional `explicit_workdir: true`
sets both the public workdir flag and environment binding. The default omits
both and exercises ordinary admitted-project observation scope. Only a local
`linux-portable` target is admitted. No Docker, VM, remote target or shared state
is used; this lane does not register itself or change the registry denominator.

Run focused checks with
`python3 -m unittest discover -s workflows/qualify/product/tests -p test_lifecycle.py`.

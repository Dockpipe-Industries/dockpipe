# Native DSM task qualification companion

This separate experimental package qualifies a fixed one-shot probe launched by
DSM Task Scheduler as the authenticated normal user. The probe reports its actual
identity and bounded Docker/network observations. It is not a shell or a general
command runner. This package declares `run-as: package`, no capabilities, no
setuid executable and a user service with `NoNewPrivileges=true`.

The ordinary mode-0755 binary is installed at
`/var/packages/dockpipe-host-task-qualification/target/bin/task-host`. The service
uses `serve`; post-install uses its `endpoint-init` role. A session-bound DSM task
may invoke only the app's fixed `probe --socket <package-var>/run/task.sock --nonce
<64-hex>` command. The task is created and run only after the app's explicit setup action.

Package var and run directories have mode 0755 so the actual DSM user can traverse
the callback path. The daemon creates `run/task.sock` with mode 0666 intentionally:
Unix permission alone is not its authorization boundary. The backend must verify
the kernel's `SO_PEERCRED` UID and the matching single-use challenge, reject root
and mismatched users, bound frames and connection lifetimes, and admit only its
fixed probe report. It must never accept a claimed UID or an arbitrary command.

The browser report route is exactly
`/webman/3rdparty/dockpipe-host-task-qualification/host-auth.cgi`. nginx forwards
GET/POST to the TLS endpoint at 127.0.0.1:38142 after verifying the per-install
certificate. The private key is mode 0600 under mode-0700 `endpoint-private`;
the public CA is mode 0644 under mode-0755 `endpoint-public`. Keys are generated
locally and are absent from the SPK. The health check uses `--cacert` and accepts
only HTTP 204 plus an existing non-symlink callback socket. No insecure TLS option
or unauthenticated plaintext credential forwarding is used.

The package UID is a trusted broker identity. Other processes running as that same
UID are not a secret-isolation boundary. Never run untrusted workloads under it or
mount this package's var directory into the shared GUI. The scheduled probe runs
as the normal DSM user and receives a one-time challenge, not DSM session cookies.

The app owns creation, execution and explicit removal of its matching disabled
one-shot task while the DSM session is available. Package stop/uninstall manages
only the native service and package route; it does not promise deletion of DSM
tasks. Confirm app removal before uninstalling if a task was created. No cookie,
token, challenge or scheduler credential is stored in packaging metadata.

The producer and local packaging tests verify archive identity, permissions,
privilege declarations, certificate pinning, reproducibility and lifecycle
behavior. They do not establish live Task Scheduler permissions, actual user/Docker
behavior, task deletion or NAS acceptance. Those remain separate native checks.

The callback result labels its `peer_uid`, `peer_gid` and `peer_pid` separately
from the probe's report. `groups_match_account` compares diagnostic supplementary
groups with the local account database; it does not authenticate groups or grant
additional permissions. Callback frames are limited to 64 KiB and eight concurrent
connections with three-second reads. HTTP reports reserve four additional KiB for
the fixed envelope and are checked before callback acceptance. The Docker query
selects only ID, name and state, at most 128 records and 64 KiB aggregate output;
it does not read container environments, configuration, secrets or log contents.
The native probe has a 15-second overall deadline; Docker and direct GitHub HTTPS
checks each have five-second deadlines. No persistent worker or terminal is started.

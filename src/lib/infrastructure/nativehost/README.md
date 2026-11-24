# Native host transport

This Linux transport connects a reverse worker running as an ordinary OS user
to a broker owned by a separate service account. The broker does not switch
users or execute commands itself. The embedding application owns authentication,
grant lifetime, nonce generation, and worker launch.

`Options.Authorize` must return the current nonroot UID and an opaque generation
that changes whenever the authorization changes. Returning an error revokes the
connection. The callback must be safe for concurrent calls and return promptly.
The broker checks it on requests and every second. `ExpectWorker` binds a
single-use 64-hex nonce to the expected kernel UID/GID, current generation, and
an expiry within five minutes. `Revoke` closes the worker and clients but leaves
listeners available for a later grant. `Close` also closes the listeners.

Socket directories must belong to the broker account and must not be writable
by another account. Framework-managed ancestor symlinks are resolved. Both
socket leaves use mode 0666: worker admission requires the nonce and kernel
UID/GID; client admission requires the key and kernel `ClientUID`. A connection
file uses schema `dockpipe.native_host_connection.v1` and contains only absolute
`socket` and `key_file` references. The key file contains 64 hex characters,
must be private, and identifies the expected broker UID through file ownership.
It must not be shared with untrusted processes. Processes sharing a UID are
within the same trust boundary.

The wire uses bounded length-prefixed JSON: snapshots retain the existing
machine snapshot DTO and its 1 MiB limit; terminal byte frames are at most
64 KiB. There are at most 32 accepted connections, 24 pending operations,
8 active PTYs, and 8 worker snapshot jobs. Snapshot deadlines are eight seconds;
handshakes and individual transport writes have three-second limits. Worker
startup retries for at most 15 seconds. A detached starter waits at most
18 seconds for acknowledgement through a private pipe, clears inherited
environment/stdio, and executes its existing ordinary `/proc/self/exe` image.

Workers require matching nonroot real/effective/saved UID and zero effective,
permitted, and inheritable capabilities. They use the account's actual home and
login shell, without importing the launching request's environment. PTYs are
new OS sessions. Disconnect closes PTYs, signals processes remaining in their
sessions and shell process groups, and reaps the shell. This is ordinary-user
cleanup: it cannot guarantee termination of descendants that deliberately
change privilege or escape the session (including through user-authorized
`sudo`). It is not a sandbox. The worker does not force `NoNewPrivileges`, so
normal user-authorized OS operations retain their usual semantics.

The PTY leader remains waitable with `waitid(WNOWAIT)` until cleanup completes,
so its session ID cannot be reused by a different session during cleanup. The
process scan reads `/proc` in batches with a 32,768-entry/one-second limit;
scan exhaustion or denied signals produce terminal failure instead of a success
claim. The leader is reaped only after that cleanup attempt.

Non-Linux platforms fail closed with `ErrUnavailable`.

The optional Docker operation permits only `start` or `stop` of a 12–64 digit
hexadecimal container ID. It talks HTTP directly to `/var/run/docker.sock`,
ignores endpoint environment/configuration, validates the daemon API version,
and accepts only fixed operation paths. It uses an eight-second total deadline
and a five-second container stop grace period. Daemon bodies are bounded and
discarded; results contain only the typed outcome and a fixed reason. A failed
or canceled request may already have reached the daemon, so clients must refresh
inventory and must not automatically retry. The authority remains exactly the
worker account's existing Docker socket permissions.

Worker snapshots also pin Docker inventory to this system daemon. They reuse the
existing inventory projection with an explicit Unix host flag, TLS disabled,
and Docker endpoint/context/config environment removed. Ordinary local
`dockpipe machine snapshot` retains its existing user-context behavior.

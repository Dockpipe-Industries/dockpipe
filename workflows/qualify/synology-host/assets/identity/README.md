# Native identity qualification companion

This separate experimental package proves one fixed identity-report operation. It
does not upgrade the working host qualification package, open a shell, integrate
the shared GUI, run Docker commands, or install SSH credentials.

`conf/privilege` requests `cap_setuid,cap_setgid` on the package-owned mode-0700
daemon only. No executable is owned by root or has a setuid bit. These capabilities
can technically select UID 0; the helper rejects root, system, non-login, nobody,
package and unsupported accounts. Initial qualification requires an explicit
local `administrators` membership. Account lookup uses only bounded `/etc/passwd`
and `/etc/group`; LDAP, domain identities and NSS-only groups are unsupported.

The DSM user unit must permit the daemon's initial file-capability acquisition.
Before listening on fixed loopback `127.0.0.1:38141`, it requires exact effective
and permitted mask `0xc0`, zero inheritable/ambient capabilities and non-root IDs.
It then enables no-new-privileges on every Go runtime thread. Missing, excessive,
or unavailable capabilities fail closed before the HTTP listener starts.

Version 0.1.0-1 is superseded and must not be installed: its mutable package-owned
worker path was an executable substitution privilege escalation boundary.
Version 0.1.0-2 is also superseded: its plain loopback upstream did not authenticate
the listener before forwarding DSM credentials. Its sealed-image local proof
remains valid; it did not establish a secure credential transport.

Version 0.1.0-3 generates a private P-256 key and self-signed endpoint certificate
during post-install using the ordinary, capability-free `identity-endpoint-init`
role. The key is mode 0600 inside package-owned mode-0700 `var/endpoint-private`.
Only the public certificate is mode 0644 inside mode-0755 `var/endpoint-public`;
var itself is mode 0755 for nginx traversal. Neither directory is mounted into
the GUI. No private key or certificate is bundled in the artifact.

The certificate has SANs localhost and 127.0.0.1 and a ten-year qualification
lifetime. Existing valid material is preserved across upgrades. Missing, partial,
linked, expired, mismatched, foreign-owned or incorrectly permissioned material
fails closed. The native listener uses TLS 1.2 or later. nginx verifies its
certificate against the package's public certificate before sending cookies or
SynoToken; the local health check also uses that pin and never `-k`.

The dedicated companion package UID is part of the trusted broker boundary.
Unix ownership does not isolate secrets from another process with that same UID:
the TLS key is owned by it, and the vendor authentication executable may expose
its request environment to same-UID readers after an ordinary exec. Never run
untrusted workloads under the companion UID. The GUI has a different UID and no
companion-var mount; production integration must retain that broker separation.

The daemon opens `/proc/self/exe`, which anchors its executing inode even if the
package path is replaced. It copies at most 64 MiB into one memfd and applies
`F_SEAL_WRITE|F_SEAL_SHRINK|F_SEAL_GROW|F_SEAL_SEAL` before listening. There is no
filesystem worker/probe executable and no path or mutable-manifest fallback.

Each authenticated report executes that same sealed image through inherited FD3
(`/proc/self/fd/3`) as a separate ordinary package-UID worker. The daemon passes
only its existing two permitted capabilities through an ambient-capability exec
transition. The worker requires all four seals and compares the image's SHA-256
with its own `/proc/self/exe`, then immediately clears inheritable/ambient sets
and verifies exact cap2 plus NNP. This requires sealed memfd execution and working
Linux ambient capabilities; unsupported kernels fail closed before serving.
The worker independently validates the current DSM cookie and SynoToken through
the shared fixed Synology authentication helper. Credentials travel only through
private host process pipes and are never written to reports, logs or saved files.

The worker resolves the authenticated local account and executes only the same
verified sealed image as the ordinary `identity-probe` role with its full UID/GID/groups. A random challenge,
parent PID and actual SIGKILL parent-death setting bind the child response to that
invocation. Real/effective/saved/filesystem IDs, complete groups, zero effective,
permitted/inheritable/ambient capabilities and NNP must match. The child bounding
set may remain nonzero; NNP prevents gaining file capabilities or normal sudo
privileges. This deliberately does not qualify production terminal semantics.

Cancellation kills the same-package-UID worker. The kernel then kills the fixed
different-UID child using its parent-death signal. Go's pinned Linux fork/exec
sets that signal after credential changes and checks the parent PID before exec;
the spawning OS thread remains locked until the child completes. The fixed child
creates no descendants. This is not a general PTY/job-control cleanup design.

The overall report deadline is 8 seconds, DSM authentication is bounded to 3
seconds, aggregate helper output is 4096 bytes and complete reports are limited
to 16 KiB. CGI metadata is limited to 32 KiB; its escaped private JSON transport
has a separate 128 KiB limit. One identity operation may run at once, with at most
16 local HTTP connections and 32 KiB request headers. No-cookie and rejected
authentication requests expose no process identity. The local `/health` route
returns only HTTP 204 after startup admission and is not published by nginx.

Build with `assets/scripts/build_identity.py --architecture amd64 --output PATH`.
Local tests prove bounds, rejection, protocols and parent-death cleanup. They do
not establish Synology's tool-worker acceptance, actual cap2 delegation, sealed
execution/ambient capability propagation or different-UID cancellation. Those require a separately
authorized NAS installation and native report/readback. Installation is not run
by the producer.

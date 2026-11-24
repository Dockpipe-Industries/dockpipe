# Native DSM host qualification

This separate diagnostic SPK qualifies the DSM boundary needed to replace the
SSH candidate. It does not upgrade Dockpipe, start a container, provide a shell,
read Docker inventory, change account membership or request extra capabilities.

Build through the repository workflow:

```sh
python3 workflows/shared/launch.py workflows/qualify/synology-host/config.yml -- \
  --architecture amd64 --output /absolute/private/output
```

Use `arm64` for the other producer architecture. A cross-built artifact is not
native hardware acceptance. The receipt records its binary and package hashes.

Manually install **Dockpipe host qualification** in DSM, then open its application
icon while signed into DSM over HTTPS. Copy only the JSON report. It contains the
authenticated username, effective process identity/capabilities and native
service status. It excludes session cookies, authentication tokens, environment
values and authentication-helper diagnostics. No extra login is requested.

Also test the report without a valid DSM session: it must fail with no identity
report. An expired DSM session must likewise fail. Do not share cookies, tokens
or request headers.

The native user service uses DSM's `systemd-user-unit` worker and runs as the
package account. It exposes private Unix status and, in revision 2, one
DSM-authenticated read-only report on `127.0.0.1:38140`. A package-owned exact
nginx route keeps the original `host-auth.cgi` URL while bypassing generic CGI
dispatch. The proxy overwrites routing metadata, forwards the DSM cookie only
to the native service, and disables access logging for this endpoint. The
native service independently validates the DSM session using the fixed
`authenticate.cgi`; it never trusts a claimed username or UID header.
Package stop/removal must stop it and release its socket/unit. The original standalone CGI mode intentionally refuses private socket access
from a different identity. Neither route makes the private socket public.
The authenticated browser username and native service UID are distinct proof;
this report does not claim user identity switching or terminal authorization.

DSM authentication returns a username; it does not change the process UID.
Actual user-scoped host terminals require a separately proven identity boundary.
The shared streamed desktop must also have one verified operator owner or
separate per-user instances. A request-supplied username, container environment
or shared package UID is not authenticated DSM user identity.

Official contracts:

- [Application Authentication](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html)
- [Systemd User Unit](https://help.synology.com/developer-guide/resource_acquisition/systemd_user_unit.html)
- [DSM 7 breaking changes](https://help.synology.com/developer-guide/breaking_changes.html)

This is qualification work. It is not a completed replacement host terminal or
a supported production integration.

Revision 1 hardware feedback confirmed the native service runs on the NAS with
an unprivileged package UID, no effective capabilities and `no_new_privs` set.
Its UI symlink, file/directory modes and generic CGI dispatch rule were present,
but the browser CGI request returned DSM's 404. Revision 2 introduced the scoped
proxy route above, which was subsequently tested on hardware in revision 3.

Revision 3 hardware testing confirmed successful upgrade and native service
startup. DSM releases the user-service registration after stopping it and before
running `preuninst`; that hook must not attempt another stop. Older revision 1
installations require a backed-up repair of their installed hook before upgrading,
because DSM executes the old hook before unpacking the replacement.

A same-origin browser request with the existing DSM cookie and `X-Syno-Token`
returned an authenticated username, the NAS hostname and an available native
service. A bare iframe request did not supply the token and failed authentication.
Revision 4 sends the token from the DSM-loaded application script in a request
header and displays the bounded report as text. It does not put the token in the
report URL, logs or persistent state. Automatic display remains a separate live
hardware check; successful authentication does not establish Unix identity
delegation or a host terminal.

## In-app Task Scheduler qualification

The capability-based identity-switching companion was rejected by DSM's privilege
admission before installation. It is not an installable native-host solution.
The separate **Dockpipe host task qualification** candidate instead declares only
`run-as: package` and offers an explicit **Enable NAS access** action inside DSM.

Build this candidate with the standalone producer:

```sh
python3 workflows/shared/run.py python3 workflows/qualify/synology-host/assets/scripts/build_task.py \
  --architecture amd64 --output /absolute/private/output
```

Install the SPK alongside the existing desktop and original qualification package.
Open its icon from the HTTPS DSM desktop. Opening the window reads status only.
Enable NAS access uses the existing DSM session to create a disabled one-shot
Task Scheduler entry for the verified local administrator, reads its owner and
exact script back, and runs its fixed native probe. It never selects root or
accepts a caller-supplied command. No SSH configuration or separate login is used.

The result separates the callback's kernel-observed UID/GID/PID from the probe's
reported host identity, groups, capabilities, local Docker inventory and direct
HTTPS check. Supplementary-group differences are visible diagnostics, not proof
of equivalent rights. This qualifies native execution; it is not yet a terminal.
Docker queries explicitly target the local Unix daemon. Network checks target
GitHub over verified HTTPS without an environment proxy.

Use **Remove test task** before uninstalling. A timed-out create or delete is an
uncertain operation: reconcile the exact task instead of blindly repeating it.
The package cannot promise to remove scheduler entries during uninstall without
a live authorized DSM session. Challenges/results expire after 15 minutes and
are invalidated by package restart; disabled tasks may outlive that state.

Local tests cover authentication, nonce ownership/replay, concurrent setup,
callback framing/shutdown, TLS pins, UI API reconciliation and archive privileges.
The internal Task Scheduler API, disabled-task manual run, actual NAS identity,
Docker access and cleanup still require hardware acceptance.

Revision 1 of the task companion installed on DSM and its pinned route reached
the authenticated native service. Two hardware failures were identified before
any task execution: DSM can render the parent window before a constructor-added
`afterrender` listener, and the local account database can legitimately alias
unrelated system accounts (`anonymous` and `ftp`). Revision 2 starts its initial
status read after either synchronous or deferred rendering, and rejects UID
aliases only when they affect the selected operator. These fixes do not relax
root, package-account or administrator eligibility checks.

Revision 2 hardware testing confirmed that the native report recognizes the
eligible DSM account after the selected-UID alias fix. The browser still executed
the revision-1 UI source even though fetching the same script URL returned the
revision-2 file; normal and hard reloads did not clear that mismatch. The cache
layer has not been identified. Revision 3 gives the UI script a distinct versioned
filename and uses DSM's native `SYNO.ux.Button` controls: generic Ext buttons were
present in the DOM but rendered with zero-size text under DSM's stylesheet.
Task creation/execution and deletion remain untested on the NAS.

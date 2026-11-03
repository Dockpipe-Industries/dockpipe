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

Also test the same CGI URL in a logged-out/private browser: it must return an
unauthenticated response, with no identity report. An expired DSM session must
likewise fail. Do not share cookies, tokens or request headers.

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
but the browser CGI request returned DSM's 404. Revision 2 uses the scoped proxy
route above; DSM authentication through that route remains a hardware test.

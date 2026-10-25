# Git Runtime Auth

Read when changing provider detection, helper-container auth mounts, SSH/HTTPS support, or host-side
Git/SSH discovery for runtime-owned sessions.

## Boundary

- Keep helper containers Git-generic.
- Keep provider detection and auth-path discovery on the host side.
- Mount auth material into runtime helper containers only, never AI worker containers.
- Treat auth adapters as runtime infrastructure, not workflow/package logic.

## Current Support And Design

Read [Git runtime sessions](../../runtime/git-runtime-sessions.md), especially the
provider/auth design section, before treating a proposed adapter as implemented.
Current volume seeding clones from the mounted local repository or uses the tar-sync
fallback. Host-side publication uses the repository's existing Git configuration.
The design’s GitHub/Azure SSH adapter examples do not establish a live provider
support matrix or permission to copy host credentials into a container.

## Host Discovery

Inspect only the effective remote, SSH command and credential-helper settings needed
for the operation. Use read-only Git/SSH inspection; it does not authorize lifecycle
mutations or an authentication attempt. URLs and configuration may contain private
identifiers or credentials: do not dump `git config --list`, resolved URLs or SSH
configuration into chat, public logs or committed evidence. Preserve only sanitized
metadata required to bind the approved operation.

## Helper Rules

- Worker editing surface stays at `/work`.
- AI workers edit files only. They do not clone, branch, checkpoint, or publish.
- Runtime helper tools may clone, fetch, checkout, checkpoint, inspect, and publish.
- Keep session metadata and audit logs under `bin/.dockpipe/sessions/...` even when the active Git
  checkout lives inside a volume workspace.

## Validation

- `git diff --check`
- Use the [core validation workflow](../core/validation-commands.md#core) for focused auth/discovery tests; provider acceptance needs separate authorized evidence.
- verify provider-specific docs/schema updates when authored workflow surfaces change

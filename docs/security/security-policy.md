# Security Policy

Dockpipe security policy is part of the runtime/isolation layer. Workflow YAML
declares high-level intent; compile resolves that intent into an effective
runtime/security manifest; run consumes the compiled truth.

Set this policy in your project's workflow YAML. It applies to container steps;
it does not sandbox host steps or grant permission to run an untrusted package.
The public YAML does not expose raw Docker flags.

## Secure Default

The default posture is intentionally conservative:

- offline network unless a workflow/profile asks for more
- read-only root filesystem where compatible
- workspace-scoped writes
- temporary paths such as `/tmp`
- non-root execution where compatible
- `no-new-privileges`
- dropped Linux capabilities
- PID/resource limits when configured

Actual enforcement is recorded in the compiled manifest so users can see what
Dockpipe applied.

## Public YAML

```yaml
security:
  profile: secure-default
  network:
    mode: offline
  filesystem:
    root: readonly
    writes: workspace-only
  process:
    user: non-root
    pid_limit: 256
```

Supported public fields:

| Field | Purpose |
|-------|---------|
| `profile` | Core-owned preset such as `secure-default`, `internet-client`, `build-online`, or `sidecar-client`. |
| `network.mode` | `offline`, `restricted`, `allowlist`, or `internet`. |
| `network.allow`, `network.block` | Domain/pattern intent for allow/block policy. |
| `filesystem.root` | `readonly` or `writable`. |
| `filesystem.writes` | `workspace-only` or `declared`. |
| `filesystem.writable_paths`, `filesystem.temp_paths` | Explicit writable/temp paths. |
| `process.user` | `auto`, `non-root`, or `root`. |
| `process.pid_limit` | PID limit. |
| `process.resources.cpu`, `process.resources.memory` | Resource hints/limits. |

## Precedence

Compile merges policy in this order:

1. engine defaults
2. runtime baseline
3. selected security profile
4. workflow-level overrides
5. step-level overrides

Step-level security applies only to that container step. It is not meaningful on
`kind: host` steps, and packaged child workflows should keep their own policy
inside the child workflow.

## Network Modes

| Mode | Meaning |
|------|---------|
| `offline` | No outbound internet. Preserve Docker-internal behavior needed for container startup and internal DNS. |
| `allowlist` | Only declared destinations are allowed when enforcement is available. |
| `restricted` | Baseline-deny intent with curated allowances. |
| `internet` | Normal outbound access, still with filesystem/process hardening. |

Dockpipe records whether enforcement is `native`, `proxy`, or `advisory`.
Domain allow/block rules are not something Docker enforces cleanly by itself, so
inspect the effective manifest and logs before relying on an allowlist.
In the current implementation, restricted/allowlist rules can be advisory; an
`allow` entry alone is not proof that other destinations are blocked.

## Host Steps

`kind: host` steps run outside Docker container isolation. They may still be part
of a workflow, but Docker network/filesystem/process policy does not sandbox the
host process.

Use host steps only when the work genuinely needs host access.

## Effective Manifests

Compile emits effective manifests with the compiled workflow/package material.
They are the runtime truth for:

- selected security profile
- network mode and enforcement type
- filesystem/process restrictions
- policy fingerprint
- rule ids and summaries for explainability

Run records store the policy summary and image decision so later tools can answer
what was enforced and why something was blocked or rebuilt.

## "Why Was This Blocked?"

1. Read the failing command and the run's policy summary.
2. Check `network.mode` and whether enforcement is native, proxy-backed or advisory.
3. Check whether the step runs in a container or on the host.
4. Inspect the effective runtime/security manifest in the compiled workflow package
   when you need the exact resolved settings.

For example, current advisory-policy logs can include:

```text
policy enforcement: network restricted is advisory in this build; full egress filtering is not active yet
policy coverage: domain allow/block rules are compiled for inspection but are not enforced natively by Docker
```

These messages describe a limit, not a successful outbound block. If a command
needs network access, declare that requirement deliberately rather than disabling
all policy to get past an error.

## Advanced

For internal design history and image-artifact interaction notes, see
[docker-security-images.md](docker-security-images.md). For Docker image caching
and rebuild behavior, see [../runtime/image-artifacts.md](../runtime/image-artifacts.md).

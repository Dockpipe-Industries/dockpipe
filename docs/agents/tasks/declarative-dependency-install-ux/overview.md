# TASK-010 Declarative Dependency Install UX

## Current State

- Workflow YAML and package manifests can declare host tool dependencies with `dependencies.host`.
- Workflow YAML and package manifests can declare supported host platforms with top-level
  `platforms` (`windows`, `macos`, `linux`, `deb`).
- Core preflights required host dependencies before workflow execution and can run the current
  platform's author-provided installer after explicit approval.
- `workflows/ci/ci-emulate` declares `docker` and `act`, so missing `act` fails before any CI
  emulator script runs.
- Workflow/package host dependency preflight now emits canonical `dependency.host.preflight` units for
  found, missing, optional-missing, and malformed dependency declarations before install approval
  or workflow execution.

- Package installation now prepares and verifies transitive `depends` and `requires_resolvers` archives from the same platform catalog before publishing. Engine minimums are enforced at install/load boundaries, with explicit update guidance. Missing/cyclic/ambiguous dependencies fail without replacing installed packages during preparation; host executable installation still uses the separate approval flow.

## Exact package selection objective

The user also needs explicit version selection for projects, package-owned
dependency scopes, and a package manager that shows installed, selected, active,
and required-by state. The [canonical proposal](../../../packages/version-pinning.md)
records those proposed semantics separately from the implemented producer-tool pin. It is ready for a
bounded implementation slice; the lock format, workflow call syntax, history
index, and UI actions are not published behavior yet. Ordinary installs can
assert a resolved catalog checksum, but they do not persist project/package
version selections or bind execution to those exact versions.

Package-owned workflows must move with their package. Root workflows should
remain as callers that select the package exactly. DorkPipe already owns its package sources and docs orchestration. A root
repository-quality caller must use the available explicit package-call contract;
future exact-version selection remains separate design work. Do not remove those published
catalog identities or source workflows before their replacement capabilities
and exact-call path are qualified.

## Still Open

- Define trusted install command policy by OS/package manager (`winget`, `brew`, apt/dnf/pacman,
  custom enterprise scripts) without hardcoding product-specific dependencies.
- Add marketplace validation for dependency installer commands/scripts before packages are published
  or promoted.
- Expand platform certification beyond the currently tested Windows and Debian paths.
- Add package install-time dependency checks so package installation can surface missing host tools
  before first run.
- Migrate the whole repo over time: every first-party workflow, package, resolver script, CI helper,
  and maintainer command should declare external host tools in `dependencies.host` instead of hiding
  them in script bodies or README-only setup notes.
- Continue expanding dependency operation-result coverage as install-time package dependency checks and
  trusted installer policy are added.
- Decide whether optional dependencies should appear in catalog/doctor output even when they do not
  block execution.

## Contract Sketch

```yaml
platforms: [windows, deb]
dependencies:
  host:
    - id: act
      command: act
      description: Runs GitHub Actions workflows locally.
      required: true
      install:
        windows: winget install nektos.act
        deb: sudo apt-get update && sudo apt-get install -y act
```

## Rules

- `depends` remains package-to-package graph metadata.
- Top-level `platforms` declares the host platforms the workflow/package supports.
- `dependencies.host` is for external host tools and author-provided OS package-manager install
  commands.
- Core owns validation and preflight; packages own dependency declarations.
- If a workflow/package declares a supported platform, all required dependencies and host scripts
  must work on that platform.
- New first-party workflows/packages should add dependency declarations when they introduce a host
  executable requirement.
- Auto-install must always cross an approval boundary unless a future explicit policy file permits it.

## Security and reliability lifecycle linkage (2026-10-26)

**P1 prerequisite scope** in the [dependency lifecycle and authentication hardening
initiative](../dependency-lifecycle/overview.md). TASK-010 continues to own trusted
installers and exact package selection/lock semantics. TASK-049 adds generic
lifecycle orchestration; TASK-050 owns Cloudflare package integration, TASK-051
advisory assessment and TASK-052 qualification. These are linked workstreams,
not replacements for this task or claims that proposed locks are implemented.

Before automatic updates, define shared host dependency versus package-lock
conflicts and explicit vulnerable-pin resolution. Lifecycle policies cannot
rewrite the producer CLI lock or a package-owned selection silently. Include
negative tests for source/digest drift, install versus selection confusion,
manager trust failure and conflicting shared-service versions. Preserve the
existing approval requirement for host installation.

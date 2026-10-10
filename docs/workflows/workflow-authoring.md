# Workflow Authoring Quickstart

Use a workflow when a command becomes repeatable. A workflow is a
`workflows/<name>/config.yml` file plus any scripts/assets beside it.

Use your own project folder; none of the examples require Dockpipe's source
checkout. Start with [Your first workflow](../onboarding.md) if you have not run a
workflow yet. For the complete key reference, see [workflow-yaml.md](workflow-yaml.md).

## Mental Model

| Concept | Meaning |
|---------|---------|
| `workflow` | What happens. |
| `step` | One action in the workflow. |
| `runtime` | Where the action runs. |
| `resolver` | Which tool/profile the action uses. |
| `kind: host` | Run this step on the host instead of in a container. |

Top-level `runtime` and `resolver` set workflow defaults. Step-level `runtime`
and `resolver` override those defaults for one step. Use `isolate` only when you
need to pin a specific image/template.

## Simple Container Workflow

Create `workflows/location/config.yml` with the following contents. Docker must
be running; the first run may download the example image.

```yaml
name: location
runtime: dockerimage
isolate: alpine:3.22

steps:
  - id: location
    cmd: pwd
```

Run it:

```bash
dockpipe workflow validate workflows/location/config.yml
dockpipe --workflow location --
```

The command prints `/work`, your mounted project directory. Replace the command
and image with ones appropriate to your project. An image must contain the tools
the command uses; Dockpipe does not add Node or other language toolchains to an
arbitrary image.

## Host Step

Use `kind: host` only when the step genuinely needs host access, such as a local
CLI, GUI launcher, or sidecar lifecycle script. The following example assumes your
project has `scripts/prepare.sh`, `package.json`, and installed Node dependencies.
Save it as `workflows/build-with-setup/config.yml`.

```yaml
name: build-with-setup

steps:
  - id: prepare
    kind: host
    cmd: ./scripts/prepare.sh

  - id: test
    runtime: dockerimage
    isolate: node:22
    cmd: npm test
```

Host steps are outside Docker container security policy. Keep container policy on
container steps.

## Resolver-Backed Container Step

Install the `codex` resolver and satisfy its authentication requirements before
using it. This example assumes your project supplies `scripts/review.sh` and a
Node test setup; choose an image version that matches your project.
Set defaults once, then override only where a step differs:

```yaml
name: review
runtime: dockerimage

steps:
  - id: lint
    isolate: node:22
    cmd: npm run lint

  - id: security-review
    resolver: codex
    cmd: ./scripts/review.sh
```

## Packaged Child Workflow

Use explicit `workflow` + `package` for nested packaged workflows. Install the
child first and replace the example names with its workflow name and namespace:

```yaml
steps:
  - workflow: child-name
    package: acme-team
```

Do not model packaged workflow calls as runtimes. The child workflow keeps its
own runtime/resolver/security settings.

## Security Profile Example

This example expects dependencies to be present in your project or image before
execution; offline policy prevents downloading them during the step.

```yaml
name: offline-test
runtime: dockerimage
isolate: node:22

security:
  profile: secure-default
  network:
    mode: offline

steps:
  - cmd: npm test
```

Security policy is container-only. Dockpipe compiles it into an effective runtime
manifest that run consumes. For the security model, see
[../security/security-policy.md](../security/security-policy.md).

## Step Outputs

Steps pass values forward with dotenv-style output files. This example uses
host steps so both commands run with your local shell tools; it does not require
Docker. Save it as `workflows/version/config.yml`:

```yaml
name: version
docker_preflight: false

steps:
  - id: compute
    kind: host
    cwd: artifacts
    cmd: sh -c 'echo VERSION=1.2.3 > version.env'
    outputs: version.env

  - id: use
    kind: host
    cmd: echo "$VERSION"
```

## Next

- Full YAML reference: [workflow-yaml.md](workflow-yaml.md)
- Package/reuse workflows: [../packages/package-quickstart.md](../packages/package-quickstart.md)
- Terms and boundaries: [../concepts/architecture-model.md](../concepts/architecture-model.md)

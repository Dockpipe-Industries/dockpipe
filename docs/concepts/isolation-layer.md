# Choose where commands run

Dockpipe workflows can run commands in containers or in the environment running
Dockpipe. Select the boundary that matches the command's needs.

## Container or host step?

| Choice | Use it for | Requirements |
| --- | --- | --- |
| Container from an image (`runtime: dockerimage`) | Commands that need a repeatable tool environment | A reachable Docker engine and an image containing the tools |
| Dockerfile-backed execution (`runtime: dockerfile`) | A workflow/resolver that builds its own tool image | Docker, the declared build inputs, and any permitted build network access |
| Host step (`kind: host`) | Local tools or explicit host integrations | The required tools in the environment running Dockpipe |

Installing a package does not start a container engine. A native CLI uses your
configured Docker context/environment. The Flatpak bundles container clients and
connects to an available host socket; see [installation](../install.md).

## Select an image

For a one-off command:

```sh
dockpipe --runtime dockerimage --isolate alpine:3.22 -- pwd
```

The project is mounted at `/work`. The command runs in the container, which is
removed when it finishes. Files written to the project mount remain in your
project. The image may need to be downloaded on first use.

The equivalent workflow selection is:

```yaml
name: location
runtime: dockerimage
isolate: alpine:3.22

steps:
  - id: location
    cmd: pwd
```

Choose an image that contains your command's tools. A minimal Alpine image does
not include Node, Go or your provider CLI. For repeatable builds, use an image
version or digest approved for your project.

## Use a tool integration

A resolver adds a tool's setup and execution behavior. Install its package and
follow its requirements before selecting it with `resolver:` or `--resolver`.
A resolver may use a Dockerfile-backed image, delegate to a packaged workflow or
perform an explicit host integration; read its package documentation.

Runtime and resolver remain separate choices: the runtime owns the execution
boundary, while the resolver supplies tool-specific behavior. Neither the
presence of a local executable nor installing Flatpak changes which package
platform a native Dockpipe process selects.

## Host steps and Flatpak

```yaml
name: hello
docker_preflight: false

steps:
  - id: hello
    kind: host
    cmd: printf 'Hello from this environment!\n'
```

On a native installation, this runs with your user permissions on the host.
Inside Flatpak, it runs in the app environment. Packages that open an editor,
call a host provider CLI or manage a host service use explicit host integrations
and need those host tools and permissions.

Docker's network, filesystem and process policy does not sandbox `kind: host`
steps. Do not use a host step to run code you intended to isolate in Docker.

## Network, files and reusable images

Use workflow `security` settings to declare container policy. Check the effective
policy and logs: some network rules may be advisory rather than enforced.
Provider calls and dependency installation may need explicitly permitted network
access. See [Security policy](../security/security-policy.md).

Dockpipe can reuse local images and build receipts. Use `dockpipe package images`
from your project to inspect image state; see
[Image artifacts](../runtime/image-artifacts.md) for missing/stale image behavior.

You can author workflows and select installed profiles without editing Dockpipe's
source or its core files. For package authoring, use the
[package model](../packages/package-model.md); engine layout belongs to the
[maintainer architecture contract](architecture-contract.md).

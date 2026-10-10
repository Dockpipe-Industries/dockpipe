# Dockpipe

Run commands and reusable workflows in your project, with explicit choices for
container execution, host tools and follow-up actions. Use the desktop launcher or
the CLI; workflows use the same YAML and package model in both.

## Install

Start with the [installation guide](docs/install.md). Published staging options
include:

| Platform | Desktop | CLI |
| --- | --- | --- |
| Linux | DEB launcher or Linux amd64 Flatpak bundle | DEB, RPM, APK, Arch or portable archive |
| macOS | Homebrew staging cask or DMG | Homebrew staging formula or tarball |
| Windows x64 | MSI with launcher | MSI CLI feature or ZIP |

The [Flatpak instructions](docs/install.md#flatpak-desktop-staging-linux-amd64)
cover KDE Platform 6.10, verified downloads and manual bundle updates. Bazzite and
full Podman/SELinux qualification remain open. macOS staging DMGs are not yet
Developer ID signed and notarized.

Installers include required core. Optional workflows and resolvers are selected
from **Packages → Marketplace** or the CLI. You do not need to clone this repository
or compile Dockpipe to use them.

## Run your first workflow

Follow [Your first workflow](docs/onboarding.md) to create and run a small workflow
in your own project. It starts with a host step that needs no Docker engine.

For a container command, start your engine and run from your project folder:

```sh
dockpipe --runtime dockerimage --isolate alpine:3.22 -- pwd
```

This downloads the example image if needed, mounts your project at `/work`, prints
the working directory and removes the container. Files written into the mounted
project remain on your machine. Choose an image that contains your command's tools.

For Flatpak, replace `dockpipe` with
`flatpak run --command=dockpipe com.dockpipe.Dockpipe`.

## Make work repeatable

Save a workflow as `workflows/location/config.yml` in your project:

```yaml
name: location
runtime: dockerimage
isolate: alpine:3.22

steps:
  - id: location
    cmd: pwd
```

Validate and run it:

```sh
dockpipe workflow validate workflows/location/config.yml
dockpipe --workflow location --
```

[Workflow authoring](docs/workflows/workflow-authoring.md) covers host steps,
container tools, outputs and packaged child workflows.

## Use packages

Open **Packages → Marketplace** in the launcher. For staging, first choose
**Settings → Package Remotes → Use staging** and save.

[Find and use packages](docs/packages/package-quickstart.md) explains installation,
dependencies, terminal commands and removal. A package can require host tools,
authentication or a container engine; installing it does not supply every external
service or approve every action it may perform.

## Learn more

- [Documentation](docs/README.md)
- [CLI reference](docs/cli-reference.md)
- [Workflow YAML](docs/workflows/workflow-yaml.md)
- [Security policy](docs/security/security-policy.md)
- [Package model](docs/packages/package-model.md)

## Contribute

To build or change Dockpipe itself, read [CONTRIBUTING.md](CONTRIBUTING.md) and the
repository's [agent guidance](AGENTS.md). Release construction and publication are
covered in [release documentation](release/README.md).

Dockpipe is licensed under the [Apache License 2.0](LICENSE).

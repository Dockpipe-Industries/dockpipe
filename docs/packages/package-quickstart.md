# Find and use packages

Packages add reusable workflows and tool integrations (resolvers) to Dockpipe.
The installer includes required core; optional packages are installed separately.
You do not need to clone or build Dockpipe to use them.

## Use the Marketplace

1. Open **Settings → Package Remotes** and choose your channel. For staging builds,
   select **Use staging**, then save.
2. Open **Packages → Marketplace** and choose a workflow or resolver.
3. Review its description, platform and dependencies before installing it.
4. Install any required packages separately, then follow the workflow's usage instructions.

Installation checks the archive against the catalog checksum and places it in your
user store. Installing a package does not install every external tool it may use,
start an engine, or sign in to a provider.

Flatpak automatically selects packages built for its architecture and KDE runtime.
A native CLI on the same machine selects native packages. Incompatible stores are
rejected rather than substituted. See [installation](../install.md) for channel and
platform setup.

## Use the terminal

List packages available for the running CLI's platform:

```sh
dockpipe package catalog --remote https://packages.staging.dockpipe.com/packages/latest.json
```

The result is JSON. Choose the exact `name` and `kind` from that catalog, then
substitute them below (`workflow` and `resolver` are the optional package kinds):

```sh
dockpipe package install --remote https://packages.staging.dockpipe.com/packages/latest.json --kind workflow --name WORKFLOW_NAME
dockpipe package list --format json
```

For Flatpak, replace `dockpipe` with
`flatpak run --command=dockpipe com.dockpipe.Dockpipe`.
For reproducible installation, use an immutable release/store manifest URL from
the selected catalog and add `--sha256 DIGEST` with that entry's checksum.
Dependencies are installed separately; inspect the package's requirements before
running it.

## Run an installed workflow

From the project you want it to operate on:

```sh
dockpipe --workflow WORKFLOW_NAME --
```

Use the workflow name documented by the package; resolver packages provide tools
and are not always workflows themselves. Project source workflows can take
precedence over an installed workflow of the same name. Use
`dockpipe package list --format json` to inspect the active package inventory.

Provider authentication and host tools remain your responsibility. Read a
package's prompts before allowing actions such as installing software or changing
services. Package installation alone does not approve those later actions.

## Update or remove a package

To obtain a newer package, inspect the chosen channel's catalog again and install
the selected entry. Other versions can remain installed; inspect the inventory and
remove an obsolete optional user archive explicitly when appropriate. Core/CLI application updates and optional package selection
are separate operations; use your application's installation method for CLI updates.

To remove an optional user-installed archive, take its absolute path from the
package inventory and run:

```sh
dockpipe package uninstall --path /absolute/path/to/package.tar.gz
```

Uninstall preserves package data and settings. Core, external stores, directories
and symlinks are protected from this command.

## Package your own workflow

You can run source workflows without compiling them. When you want to distribute
one, start with [workflow authoring](../workflows/workflow-authoring.md). For a
workflow saved at `workflows/hello/config.yml`:

```sh
dockpipe package compile workflow workflows/hello --workdir .
dockpipe package list --format json
```

Compilation validates the workflow and writes its archive to the project package
store under `bin/.dockpipe/internal/packages/workflows/`. It does not publish the
archive or run Docker builds. Include the scripts/assets the workflow needs and
declare dependencies in its package metadata.

For package metadata, source builds, tests, durable data and store layout, use the
[package model](package-model.md). For container image reuse and diagnostics, use
[image artifacts](../runtime/image-artifacts.md).

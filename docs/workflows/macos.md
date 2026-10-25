# Workflows on macOS

A workflow's `platforms` field describes the **host** platforms it supports. A Mac
can run Linux container commands through Docker Desktop, but host scripts,
native helpers and nested workflows must also support macOS. A package's macOS
support does not imply that every workflow inside it supports macOS.

## Before running a workflow

1. Install Dockpipe using the [macOS installation guide](../install.md#macos-cli--remote-worker).
2. For container steps, start Docker Desktop and allow access to the selected
   project directory when required. Host-only workflows do not require Docker.
3. Install the external tools declared by the chosen workflow. Installing a
   package does not authenticate a provider or start a service automatically.
4. Check the package's macOS instructions, including its Intel/Apple Silicon
   requirements. Native helper binaries must match the execution architecture.

For Bash scripts that need features unavailable in Apple's `/bin/bash`, select a
compatible Bash explicitly with `DOCKPIPE_HOST_BASH_BIN`, using its absolute path.
Follow the package's minimum version requirement. Check the environment used by
Dockpipe: changing a terminal's PATH does not change an already-running GUI or
background worker.

## Check the workflow, not an old package list

Use the workflow's current `platforms` declaration and the package's own
qualification record. Package source and platform instructions live in the
[owning repositories](../packages/core-tools.md). Source declarations and a passing
schema check do not establish native execution, provider authentication or an
interactive desktop launch.

The optional VM package documents its
[macOS QEMU backend](https://github.com/Dockpipe-Industries/dockpipe-packages/blob/master/packages/vm/docs/vm-darwin.md).
Its emulation, guest setup and hardware limits are separate from Dockpipe's generic
workflow execution. Do not infer guest acceleration or passthrough support from a
macOS label alone.

## Updating an existing workflow

Changing the Dockpipe executable does not rewrite checkout YAML or compiled
package platform declarations. Install or compile the revised package, then
submit a **new** remote job. An already queued job retains the workflow snapshot
that was originally submitted.

See [workflow authoring](workflow-authoring.md) for examples and the
[YAML reference](workflow-yaml.md) for the `platforms` contract.

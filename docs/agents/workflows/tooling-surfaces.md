# GUI and editor surfaces

Read for changes to the Dockpipe GUI shell, host SDK, workflow editor or language
extension. The source boundaries are in [the GUI architecture](../../../src/dockpipe-app/ARCHITECTURE.md)
and [editor ownership](../../packages/editor-extensions.md).

## Ownership

| Surface | Owner |
| --- | --- |
| GUI shell, generic package contributions and workflow editing | Dockpipe, `src/dockpipe-app/` |
| Workflow/package YAML and project JSON editor support | Dockpipe, `tooling/vscode-extensions/dockpipe-language-support/` |
| PipeLang grammar, compiler diagnostics and language SDK | PipeLang |
| AI domain logic, model/provider configuration and AI contribution proposals | DorkPipe |
| Existing Pipeon application | Pipeon; product development is paused |

Call the GUI product Dockpipe. Preserve legacy installed IDs, host SDK symbols and
package ABI names where compatibility requires them.

## Contracts

- Keep domain/application operations independent of Qt views. UI adapters present
  state, collect input and invoke application operations.
- Workflow YAML and declared package metadata remain the durable execution
  contracts. UI preferences, drafts and caches must not become a second workflow
  engine or an independent credential/approval store.
- Preserve cancellation, failure reporting, input validation and package removal
  behavior across UI and headless entrypoints.
- Update parser/schema, editor mirrors and human documentation together when a
  public authored contract changes.
- DorkPipe's proposed AI contributions use the existing app and generic extension
  contracts. There is no separate Dockpipe AI application to implement here.
  Follow the owning backlog before implementing deferred features.

## Checks

Use the focused checks in [validation commands](../core/validation-commands.md)
and the affected source architecture guide. Editor changes use the owning
`editor.build` workflow; GUI changes use the native desktop build/test workflows.
Distinguish Linux/offscreen checks from native interactive acceptance. A docs-only
change needs link/routing checks, not a desktop build or extension installation.

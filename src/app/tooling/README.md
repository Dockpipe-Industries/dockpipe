# `src/app/tooling/`

First-party Dockpipe tooling surfaces that are **not** part of the engine live here.

Rules:

- **Not engine code:** keep engine behavior in **`src/lib/`** and **`src/cmd/`**
- **First-party tooling only:** desktop shells, editor extensions, and similar tooling that belongs to Dockpipe itself
- **No package ownership confusion:** package/runtime products stay under **`packages/`**; shared Dockpipe tooling lives here

Current contents:

- **`dockpipe-launcher/`** — Qt desktop shell for launching Dockpipe-backed sessions
- **Editor extensions** — maintained in DorkPipe; see [ownership and build contracts](../../../docs/packages/editor-extensions.md)


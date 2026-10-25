# Dockpipe Language Support

Dockpipe workflow YAML, package manifests, project JSON configuration and public SDK
helper completion. PipeLang owns `.pipe` syntax and compiler diagnostics in the
separate `dockpipe.pipelang-language-support` dependency. The lightweight typed-model
reader here supplies Dockpipe YAML field metadata; it is not a language compiler.

Build and test through the repository's `workflows/editor.build/config.yml` with an
absolute pinned `DOCKPIPE_BIN` (or `DOCKPIPE_BOOTSTRAP_BIN`). Node 22 or newer is required.
The workflow packages this extension only and records the exact VSIX identity, version
and SHA-256. It never packages the DorkPipe/Pipeon companion.

The schema files ship from this runtime's authoritative sources. `compile.core_from`
is retired. SDK helper-path snippets require the next runtime release that includes
`dockpipe sdk path --language shell|powershell|python`; older releases retain shell
bootstrap through `eval "$(dockpipe sdk)"`. The Go snippets require the matching next SDK artifact containing
`dockpipe/src/lib/sdk/repotools`; no current published artifact is claimed here.

See the repository editor workflow documentation for candidate review and manual
Marketplace publication. Building a VSIX does not publish it.

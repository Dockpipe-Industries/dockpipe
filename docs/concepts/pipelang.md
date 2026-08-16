# PipeLang (v0.0.0.1)

PipeLang is an optional typed authoring layer for DockPipe.

It does not replace YAML workflows. YAML remains first-class.

This page defines the frozen `v0.0.0.1` contract. The accepted future direction is a
general-purpose, deterministic, target-neutral managed language that can eventually compile its own
compiler. That direction is C#-familiar, not C#-compatible, and does not add pointers, manual memory,
unsafe APIs, hidden host access, or target-specific syntax. No future direction described here
changes existing source or artifact behavior without an explicit language-version migration.

Use PipeLang for:
- typed workflow/package configuration models
- defaults and documentation summaries close to the model
- launcher/editor metadata derived from those types

In `v0.0.0.1`, do not use PipeLang as a replacement for workflow execution logic. DockPipe still
runs normal workflow YAML.

## Scope in v0.0.0.1

Supported:
- Primitive types: `string`, `int`, `bool`, `float`
- `Interface` declarations (structural contracts)
- `Class` declarations with optional defaults
- Object/interface-typed fields inside other classes
- Generic list shapes such as `List<string>` and `List<IImageResource>`
- Split declarations across multiple `.pipe` files in the same module tree
- `Class : Interface` conformance checks
- Expression-bodied methods (`=>`) with static type-checking
- Deterministic artifact generation
- CLI-only method invocation

Not supported in this version:
- Side effects in methods
- Runtime/resolver execution through methods
- Hidden execution during compile
- General-purpose scripting/runtime behavior

These are version boundaries, not permanent limits on the language roadmap.

Reserved/plumbed but not fully implemented yet:
- `IComparable` for custom object comparison. The parser/type checker understands the contract boundary, but custom object comparisons should still fail clearly until the runtime semantics land.

## Authoring model

PipeLang is the **data model** layer.

Typical responsibilities:
- define the root workflow config type
- define nested objects such as `General`, `Storage`, or `Network`
- define list-valued fields such as `List<string>`
- define defaults on the implementing class
- keep XML summaries and field docs close to the type

Typical non-responsibilities:
- page layout
- section ordering
- launcher tab/group structure
- select-vs-create UX

Those presentation concerns live in workflow YAML `view:` metadata, not in the PipeLang type system.

## Workflow binding

Workflow YAML binds to PipeLang through top-level `types:`.

Example:

```yaml
types:
  - ../../resolvers/qemu/models/QemuVmResolverConfig.pipe
```

This means:
- the referenced `.pipe` file is the **entrypoint**
- DockPipe reads the **module tree** rooted beside that file
- sibling `.pipe` files in that module may contribute additional interfaces/classes
- the selected entry type becomes the **root model** for tooling/catalog/launcher use

The workflow still executes through normal YAML/env resolution. PipeLang only shapes the authored model and the generated metadata.

## Environment mapping

Leaf fields bind to environment variables through:
- explicit annotations such as `[EnvName = "DOCKPIPE_VM_DISK"]`, or
- inferred names when the model/catalog layer can derive them

Example:

```pipelang
public Interface IWindowsVmStorage
{
    [EnvName = "DOCKPIPE_VM_DISK"]
    public string Disk;
}
```

This keeps the strong typed model separate from the runtime’s existing env contract.

## Workflow `view:` relationship

PipeLang works together with workflow YAML `view:`.

- PipeLang defines **what the data is**
- workflow YAML `view:` defines **how a launcher may present it**

Example shape:

```yaml
types:
  - ../../resolvers/qemu/models/QemuVmResolverConfig.pipe

view:
  entry:
    type: choice
    field: General.BootSource
    options:
      - value: image
        pages: [image]
      - value: installer-iso
        pages: [install]
  pages:
    - id: image
      title: Existing Image
      sections:
        - id: media
          title: Existing Image
          fields:
            - Storage.Disk
    - id: install
      title: Install Media
      sections:
        - id: media
          title: Images And Media
          fields:
            - Storage.Cdrom
```

The field paths in `view:` reference the PipeLang root model recursively. Entry routing still binds back to the same model field; it is not a separate runtime state machine.

## Commands

Check a source set without evaluation or artifact emission:

```bash
dockpipe pipelang check --in workflows/mywf/model.pipe --format json
```

Compile artifacts:

```bash
dockpipe pipelang compile --in workflows/mywf/model.pipe --entry DefaultDeployConfig --out bin/.dockpipe/pipelang
```

Invoke method via CLI:

```bash
dockpipe pipelang invoke --in workflows/mywf/model.pipe --class DefaultDeployConfig --method FullImage --format text
```

Materialize all `.pipe` files under configured compile roots:

```bash
dockpipe pipelang materialize --workdir .
```

## Source and diagnostic contract

All compiler entrypoints admit source through one deterministic source-set contract. File identity
is the normalized source path, input must be strict UTF-8, and token/AST spans are half-open UTF-8
byte ranges tied to that identity. Resolved diagnostics also expose one-based line, Unicode scalar
column, and UTF-16 column values so CLI and editor rendering share the same locations.

Diagnostics have stable `code`, `category`, and `severity` fields, one primary span, optional
related spans, and deterministic file/span/code ordering. `pipelang check --format json` exposes
schema 1 of that compiler result. It is offline and inert: it does not evaluate methods, emit
workflow/bindings artifacts, refresh generated stores, or enter any runtime/resolver path. The
legacy `compile`, `invoke`, catalog, and materialize paths use the same parser/source identities;
their `v0.0.0.1` syntax and artifacts remain frozen.

## Structured module-binding foundation

The compiler library also has a syntax-independent module-binding query for the accepted future
contract. `AnalyzeModuleSet` receives an explicit non-legacy language-contract identity, one root
module, complete module source bytes, structured module/symbol imports with durable spans, and a
complete dependency lock. Each locked module records its direct dependencies and a deterministic
SHA-256 digest over normalized source identities plus exact source bytes. Missing bytes, digest
drift, undeclared dependencies, duplicate module owners, unknown/private/ambiguous imports, and
import cycles fail through the same ordered structured-diagnostic contract. Compilation remains
offline and never fetches or repairs dependencies.

This is a binder/input foundation, not a released source surface. No post-`v0.0.0.1` public name,
version, module keyword, import spelling, manifest format, CLI selector, YAML field, or editor grammar
is selected by it. The existing parser and every CLI/catalog/materialize/package/editor consumer
remain on the frozen sibling-source-set lane until a separately reviewed migration chooses those
public spellings explicitly.

The next compiler-only foundation can additionally receive explicit semantic-ID assignments by
declaration span. `AnalyzeSemanticModuleSet` validates lowercase dotted ASCII IDs, requires them for
public modules/types/members, leaves private implementation declarations ID-optional, verifies a
separate semantic-assignment digest in the dependency lock, and carries matching IDs through
structured diagnostics. `BuildSemanticProjection` emits deterministic module, import, type, member,
resolved-type, lock-digest, source-range, and diagnostic records without exposing analysis-local
`SymbolID` values. Its language, compiler, and projection identities are explicit inputs; no first
production values or source spellings have been selected yet.

## Artifacts

Compile emits:
- `<Class>.workflow.yml`
- `<Class>.bindings.json`
- `<Class>.bindings.env`

Those artifacts are inspectable authoring outputs. They are not a second execution engine.

`bindings.env` is intended for direct script consumption:

```bash
source bin/.dockpipe/pipelang/DefaultDeployConfig.bindings.env
echo "$PIPELANG_IMAGE"
```

## Architecture boundary

PipeLang `v0.0.0.1` is an authoring/compiler feature.

DockPipe execution still uses compiled YAML and existing workflow execution paths.

`dockpipe run` does not parse PipeLang directly.

Think of the layering as:

- PipeLang: types, defaults, docs
- workflow YAML: execution plus optional authored `view:` metadata
- launcher/tools: render the model/view contract
- runner: consume the existing workflow/env contract

## Accepted future compiler boundary

Future executable PipeLang uses one compiler contract:

```text
source -> syntax -> module binding -> typed HIR -> Core IR
                                             |          |
                                             |          `-> executable backends (Go first)
                                             `-> versioned semantic projection
                                                          |- Application IR
                                                          `- Service IR
```

Syntax trees, bound trees, typed HIR, and Core IR remain distinct compiler representations. The
public semantic projection is independently versioned. Application IR and Service IR specialize
the same semantic/Core foundation; neither reparses `.pipe` files nor defines language behavior.

Pure compilation remains offline and deterministic. Executable entrypoints declare typed effects;
ordinary external work still crosses the governed DockPipe workflow/package -> runtime -> resolver
-> optional strategy boundary. Qt, Go, HTTP, browser, operating-system, and embedded behavior stays
in target resolvers and cannot redefine or silently weaken PipeLang semantics.

Go is the first deterministic native backend and bootstrap seed. Eventual self-hosting uses a
pinned Go stage 0, a PipeLang stage 1, and a PipeLang stage 2; normalized semantic/Core artifacts,
diagnostics, behavior, and target outputs must reproduce between stages 1 and 2. Full, constrained,
and MCU target profiles select validated backend capabilities without changing source syntax.

The complete accepted decisions, compatibility inventory, and bounded implementation order live in
[TASK-021](../agents/tasks/pipelang-reactive-application-language.md).

# TASK-021 PipeLang Deterministic Self-Hosting Managed Language Foundation

## Decision Status

The `vNext` foundation decision packet in this record is **accepted** as of 2026-08-16. Acceptance
fixes semantics, compiler boundaries, bootstrap stages, compatibility, and implementation order;
examples and fixtures remain non-normative and accept no production syntax. Separately authorized
bounded objectives have completed implementation-order steps 1 through 6 and step-7 slices 7a
through 7aa. The current explicit language contract is `v0.26.0`; the frozen `v0.0.0.1` lane remains
unchanged. This record does not by itself authorize another step-7 or later language slice.

## Goal

Evolve PipeLang from its current optional typed configuration/model layer into a general-purpose,
target-neutral managed language for deterministic programs, compiler implementation, AI-assisted
change analysis, automated verification, declarative applications, and transport-neutral services.
PipeLang must eventually compile its own compiler without becoming C#-compatible, target-shaped, or
an unsafe/manual-memory systems language.

PipeLang should own types, validation, reactive state, pure computed values, typed state actions,
stable semantic identities, explicit effects and authority, contracts/invariants, deterministic
replay metadata, governed effect declarations, test-generation metadata, and safe binding
expressions. It compiles those semantics through one generic executable pipeline and publishes
versioned semantic projections consumed by tooling. The Application IR in
[TASK-020](../declarative-application-surfaces-and-target-builders/overview.md) and Service IR in
[TASK-022](../go-first-pipelang-backend-services/overview.md) are specialized projections over that shared
semantic/Core foundation, not independent language models.

This is the durable design and bounded-progress record. It does not authorize additional parser,
lexer, AST, typechecker, evaluator, compiler, CLI, schema, catalog, editor-extension,
generated-artifact, or runtime changes beyond an explicitly granted implementation objective.

## Priority And Dependency

This is the first implementation dependency for TASK-020. Define and prove the language semantics
before implementing Qt/web application adapters or accepting a public application YAML shape.

TASK-021 owns the language and compiler contract. TASK-020 owns semantic application components,
layout/styling, Application IR integration, target adapters, and artifact manifests. Qt, HTML, CSS,
QML, C++, CMake, and WebAssembly remain absent from PipeLang semantics.

[TASK-022](../go-first-pipelang-backend-services/overview.md) consumes the stable IDs, type, contract,
effect/authority, determinism, replay, and semantic-graph foundations defined here to describe
transport-neutral services. TASK-022 owns Service IR, Go backend and Qt client resolvers, schemas,
service tests, packaging, and deployment artifacts; backend concerns do not widen PipeLang core by
implication.

The initial delivery priority is:

1. source files, UTF-8 decoding, source spans, and structured diagnostics;
2. structured `TypeRef`, symbol ownership, modules/imports, and stable semantic identities;
3. one binding/type-checking path into typed HIR and backend-neutral Core IR;
4. explicit effects/authority plus contracts, replay, and first-class tests; and
5. the deterministic Go seed/backend needed to prove executable output and later self-hosting.

Broader reactive-state and application-language features build on those foundations rather than
landing as one uncontrolled rewrite.

## Language Personality

PipeLang remains C#-familiar without claiming C# source, binary, library, runtime, reflection, or
tooling compatibility:

- familiar braces, declarations, attributes, expressions, generics, and type spelling;
- strong static typing and ordinary code readable by C# developers;
- minimal punctuation and ceremony with clear source-located compiler diagnostics;
- deterministic semantics and explicit authority;
- no YAML-like indentation/nesting inside `.pipe` files;
- no Lisp-like, academic, or unnecessarily exotic surface syntax;
- no magical AI syntax embedded through normal programs; and
- one language contract across CLI, editor, compiler, semantic graph, tests, and backends.

Advanced semantics should make ordinary code safer and easier to inspect without making ordinary
code noisy. Attributes are appropriate for stable metadata and restrained architectural intent;
core control flow and contracts should use readable language constructs when attributes would hide
semantics.

The goal is not to bolt a model onto the language. Normal PipeLang remains fully useful without AI.
The language instead exposes enough stable, typed, deterministic structure for humans, agents,
compilers, IDEs, and verification tools to reason about the same program.

## Current Baseline

Canonical current behavior is documented in `docs/concepts/pipelang.md` as PipeLang `v0.0.0.1`.

| Current capability | Current boundary |
| --- | --- |
| Primitive types | `string`, `int`, `bool`, and `float` |
| Declarations | `Interface` and `Class`, visibility, annotations, fields, defaults, and structural conformance |
| Composite shapes | object/interface fields and `List<T>` type shapes |
| Modules | declarations merged from sibling `.pipe` files under one detected module tree |
| Methods | expression-bodied, statically typed, side-effect-free methods with CLI-only invocation |
| Expressions | literals, identifiers, unary operators, and bounded binary operators |
| Compilation | deterministic workflow YAML, bindings JSON, and bindings env artifacts |
| Integration | workflow `types:`, catalog/tooling metadata, materialization, CLI compile/invoke, and VS Code/Cursor language support |

The current contract explicitly rejects side effects, runtime/resolver execution through methods,
hidden compile-time execution, and general-purpose scripting. YAML remains first-class and DockPipe
workflow execution does not parse PipeLang directly.

## Existing Ownership Map

| Area | Current owner |
| --- | --- |
| Canonical language behavior | `docs/concepts/pipelang.md` |
| AST and type representation | `src/lib/pipelang/ast.go` |
| Lexing and parsing | `src/lib/pipelang/lexer.go`, `parser.go` |
| Static semantics | `src/lib/pipelang/typecheck.go` |
| Pure method evaluation | `src/lib/pipelang/eval.go` |
| Deterministic compilation | `src/lib/pipelang/compile.go` |
| CLI compile/invoke/materialize | `src/lib/application/pipelang_cmd.go`, `pipelang_materialize.go` |
| Workflow/catalog integration | `src/lib/domain/workflow.go` plus existing catalog/application projections |
| Editor language support | `src/app/tooling/vscode-extensions/dockpipe-language-support/` |
| Tests and compatibility fixtures | `src/lib/pipelang/*_test.go`, application PipeLang tests, and compiler golden tests |

Any public language change must update all affected owners together. Generated syntax support may
not lead or silently define the language.

## Hard Language Boundary

PipeLang is a general-purpose managed language and compiler foundation, not a second DockPipe
execution engine and not an unsafe/manual-memory systems language.

It must not expose unrestricted:

- filesystem, network, process, shell, environment, or secret access;
- pointers, manual allocation, unsafe memory, threads, locks, or target runtime handles;
- Qt, QML, browser DOM, JavaScript, C++, CMake, WebAssembly, or resolver-specific APIs;
- hidden work during parsing, type-checking, compilation, catalog projection, or editor analysis; or
- direct runtime/resolver execution that bypasses workflow/package and policy ownership.

Pure language evaluation is deterministic and offline. External work is represented by a typed
effect declaration and remains governed by the existing DockPipe model:

```text
PipeLang effect declaration
          |
          v
typed capability request
          |
          v
workflow/package -> runtime -> resolver -> optional strategy
```

Compiling, cataloging, rendering, hovering, or completing a PipeLang file never invokes the effect.

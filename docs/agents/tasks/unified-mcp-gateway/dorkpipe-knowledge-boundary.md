# DorkPipe Knowledge Features — Ownership and Generic Extension Dependencies

Recorded 2026-10-17. The user clarified that Unified Knowledge/Graph/Context Management is a **new
feature**, not an existing initiative to migrate. Its single authoritative backlog is **DORK-001**
in the current `Dockpipe-Industries/dorkpipe` repository:

- Entry point: `docs/agents/task-index.yaml`.
- Task: `docs/agents/tasks/unified-knowledge-context/index.yaml`.
- Requirements: `docs/agents/tasks/unified-knowledge-context/overview.md`.
- Source/reuse inventory: `docs/agents/tasks/unified-knowledge-context/integration-inventory.md`.

These repository-relative references are cross-repository coordination records, not sibling-source
build dependencies or a claim that the uncommitted documents are already published. No prior
knowledge item was deleted or marked moved; this document is a dependency pointer, not another
knowledge initiative.

## Ownership

DockPipe retains generic runtime, package install/update/enable/disable/remove, capability/tool
registration, authentication/authorization, CLI/UI extension contracts, and existing generic
configuration/persistence interfaces. Core must remain usable without DorkPipe installed.

DorkPipe owns local sources, Markdown/document ingestion, persistent indexes, RAG/full-text/hybrid
retrieval, knowledge graphs, DAG ingestion/traversal, context routing/ranking, domain data and its
CLI/UI/MCP contributions. Preserve the extracted repository, independent package versioning,
provenance and pinned artifact/SDK interfaces. Do not move knowledge implementation into core.

Cloud synchronization, shared teams and hosted knowledge/retrieval remain future work. TASK-038's
Terraform-specific ownership and scope are unchanged. TASK-008 now records DorkPipe-owned AI authoring and inspection features inside the
existing DockPipe app, not a separate DockPipe AI package/application. Knowledge and
AI workflow features share DorkPipe ownership and generic CLI/UI/MCP contributions.
TASK-037 extraction and spatial/Cockpit history remain unchanged.

## Delivery Priority

The user's latest 2026-10-17 priority is shared core networking and package/version
foundations, then unified MCP, then DORK-001 graphs, knowledge and context. Optional
Models navigation, model management, chat and richer AI authoring remain deferred. Prove the smallest generic contribution seams with
useful knowledge operations first; do not require a complete AI UI platform.
DorkPipe may later contribute a top-level Models area with user-selected supported
local/remote/hosted execution adapters through DockPipe contracts. Neither a model
runtime nor a local container stack is a prerequisite for basic knowledge features.

## AGENTS Bootstrap and Context Routing

Target backlog: repository AGENTS.md becomes a minimal versioned bootstrap with
identity, essential rules, configured MCP reference and safe offline behavior.
TASK-039 routes authenticated knowledge/context requests to DorkPipe; DORK-001
owns retrieval, decision/task state, provenance, freshness and supersession.
See [the proposed contract](https://github.com/Dockpipe-Industries/dorkpipe/blob/master/docs/workflows/agentic-design-notes.md#proposed-agent-bootstrap-and-knowledge-routing).
Keep source permissions, policy authority and execution approvals separate from
retrieved content. Do not remove the current Markdown guidance or assume host MCP
support before migration and failure-path qualification. This is not shipped behavior.

## Generic Work Retained in TASK-039

1. Inspect whether existing package metadata and command dispatch can register trusted package-owned
   top-level commands. If missing, define the smallest generic extension. DorkPipe contributes
   knowledge/graph operations; core must not hardcode individual DorkPipe commands.
2. Coordinate with [TASK-020](../declarative-application-surfaces-and-target-builders/overview.md)
   on normalized catalog/application contracts for package-contributed navigation/panels. Reuse
   current UI components and core APIs, without a new framework or arbitrary unrestricted UI loader.
   Do not make wholesale application generation a prerequisite for a small contribution seam.
3. Use the unified registry to expose package-scoped knowledge tools with stable namespaces,
   collision checks, authorization-aware discovery and invocation, explicit read/write policy,
   and no mandatory separate DorkPipe MCP listener/process.
4. Make CLI, UI and MCP consume the same package-domain implementation and authoritative
   configuration; preserve headless use and [CLI-first parity](cli-first-management.md).
5. Coordinate enable/disable/update/remove across commands, views, tools, indexing work and watchers.
   Retain user sources/indexes/graphs on uninstall according to established retention rules; use
   migration fixtures, not live user data. Version and compatibility checks remain package-owned
   through generic lifecycle contracts.

Existing package/catalog commands and desktop CLI-result consumers provide reuse candidates. Dynamic
top-level command and domain-view registration have not been qualified by this backlog inspection.
DorkPipe's existing separate MCP/compose/PostgreSQL stack is source evidence, not a requirement for
the new local feature. Identify a compatible migration seam; do not retire working behavior now.

## Cross-Repository Acceptance

- DockPipe without DorkPipe retains core CLI/runtime/package management and unified MCP capability.
- Installing/enabling a versioned DorkPipe package reveals its CLI, Knowledge/Graphs/Context views
  and authorized MCP tools through generic interfaces; no second application or ordinary-use
  container/daemon/database service is required without a documented technical constraint.
- CLI/UI/MCP share the same package data and operations. Disable removes contributions cleanly;
  re-enable restores retained data, and uninstall does not silently delete it.
- DorkPipe qualifies Markdown indexing/search, cyclic knowledge graphs versus acyclic DAGs,
  retrieval/routing, resource bounds and offline behavior in its repository.
- DockPipe qualifies generic extension registration/lifecycle and UI/CLI integration without
  importing DorkPipe source or duplicating domain logic.

## Authorized Repository Split — 2026-10-17

The user now authorizes renaming the current private `Dockpipe-Industries/dorkpipe`
to `dorkpipe-platform` and creating a new public `dorkpipe` repository for package,
MCP, editor and AI/knowledge feature code. This supersedes the earlier deferral.
The platform repository owns website/platform and private integration-specific code;
public reusable sources require operator-supplied targets and credential references.

Preserve private history and frozen releases in the renamed repository. Move an audited
source snapshot with licenses/provenance intact to the public repository; do not expose
private history or silently rewrite old artifacts. Inventory SDK/bootstrap dependencies,
publisher visibility checks, environments, references and consumer pins before retargeting.
Public release workflows must qualify the new source independently. Old package-host
candidates remain immutable; repository visibility is separate from package distribution.

The user's next checkpoint is the completed cross-repository direction/reuse audit and
related qualified changes on staging before new feature implementation. Implementation
status must be recorded from actual remote readback, not inferred from this authorization.

# Exact package selection (proposal)

**Status:** Proposed for a release after 0.6.17. The project and package locks,
side-by-side execution, scoped workflow calls, version picker, and selected-version
UI described below are not available in 0.6.17. Their file format and workflow
syntax are still under design; do not author files or calls from this page yet.

Today, `dockpipe package install` can use a resolved HTTPS catalog URL and a
`--sha256` assertion against the selected archive row. It verifies package
archives and prepares declared package dependencies from the selected
platform catalog. Other versions can remain on disk, but an install does not
persist a project selection or make ordinary execution use that version.
Installed-package inventory shows what is available, not which version a
workflow will use. Runtime resolution still follows the existing source and
store precedence. Refreshing a catalog or installing an archive is therefore
not a project-level upgrade operation.

## Proposed selection model

A project would lock only its **direct** package dependencies for each target
platform. A verified package would carry its **own** lock for its direct
dependencies. When execution enters that package, its lock becomes the active
dependency scope; the project cannot silently replace the package's choices.
The selection would identify the package kind, name, exact version and platform,
the immutable catalog URL and digest, and the archive digest and size. The
engine would verify those bytes before admitting an object. Project and package
locks would be separate authorities, never one flattened global winner.

Multiple exact versions could coexist as verified, digest-addressed objects.
Installation would make an object available but would not select or activate it.
Selecting, upgrading, or downgrading a project's direct dependency would be an
explicit operation that leaves other projects and package-owned locks intact.
Execution would not download a missing selected version or substitute a newer
one. A missing or changed selection would fail with a clear install or pin-drift
error.

Different package contexts could use different dependency versions when their
execution is isolated. If they must share a runtime, resolver, profile, or
service binding, incompatible exact artifacts would fail before that shared
resource starts or changes. Equal version labels with different digests would
still conflict. No automatic compatibility assumption or last-writer-wins
override is proposed.

Package-owned workflows would travel with their package. Root workflows that
use them would call an exact project-selected package dependency, rather than
copy the package implementation or rely on a namespace-only lookup. The two
documentation orchestration workflows are planned to move with DorkPipe; the
root repository-quality workflow should remain as a thin pinned caller. Existing
published workflow IDs and archives remain available while new reviewed
versions establish that contract. The call format is not yet a supported
workflow field.

## Proposed package manager behavior

The CLI would expose exact package selection and history with records the app
can consume. The app would distinguish **installed** objects, the version
**selected** by the current project or package scope, and the version **active**
in a running shared resource. It would show source, digest, and which direct
dependency requires each object. Install, Select, Upgrade, and Downgrade would
be deliberate actions; refresh, discovery, and launch would never upgrade a
selection automatically. A known hidden package could be explicitly installed
by exact identity and version while remaining absent from ordinary discovery.

The central package host would publish bounded version history from immutable
catalog snapshots. Standalone package publishers would supply verified
artifacts to that host; client installation would not require access to private
source repositories. Catalog integrity, same-origin archive retrieval, platform
and engine compatibility, cancellation, and dependency limits would remain
admission checks.

This is the [TASK-010](../agents/tasks/declarative-dependency-install-ux/overview.md)
implementation objective. Current install and runtime behavior is documented in
the [package model](package-model.md); the proposed lock filename, schema and
call syntax will become reference documentation only after implementation and
qualification.

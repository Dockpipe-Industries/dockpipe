# Package model and storage

Dockpipe distinguishes **what you author** (source trees) from **what you ship** (tarballs and the local compile store). This doc describes that split and how **`dockpipe package compile`** / **`dockpipe build`** validate packages.

If you only need the common workflow, start with
**[package-quickstart.md](package-quickstart.md)**. This file is the detailed
package/store reference for users configuring shared stores and authors packaging
their own workflows. Paths under a project refer to your project, not a required
clone of Dockpipe.

## Canonical layout (mental model)

Dockpipe separates project build outputs, user installs, system installs and published artifacts. Source authoring and build validation are described alongside those roots below.

1. **Project-local build output** lives under **`<workdir>/bin/.dockpipe/`**. The default project package root is **`<workdir>/bin/.dockpipe/internal/packages/`**, or **`DOCKPIPE_PACKAGES_ROOT`** when explicitly overridden. **`dockpipe package compile`** materializes **tarballs** here: **`core/`**, **`resolvers/`**, **`workflows/`** (**`dockpipe-workflow-*`** only). This is a **working** store for the project, not the global install root.

2. **Global installs** live under **`DOCKPIPE_GLOBAL_ROOT`** when set, otherwise the OS user data directory (for example **`~/.local/share/dockpipe`** on Linux). Global packages use **`<global-root>/packages/`** and global core uses **`<global-root>/templates/core/`**. There is no **`bin/`** segment in the global root.

3. **System installs** may also ship package tarballs under a shared root (override with **`DOCKPIPE_SYSTEM_ROOT`**). Linux packages install compiled tarballs under **`/usr/share/dockpipe/packages/`**; runtime resolution checks project-local first, then per-user global roots, then system-shared roots.

4. **Published / remote packages** are versioned tarballs on a static origin or registry-like store. Each installable unit is a tarball (for example **`dockpipe-workflow-<name>-<ver>.tar.gz`**, **`dockpipe-resolver-…`**, **`dockpipe-core-…`**). The engine can load package tarballs directly when appropriate; it does not need every remote package unpacked into an authoring tree.

5. **Source-level** resolution applies when you point at **authoring** trees: repo **`workflows/`**, **`templates/`**, **`src/core/…`**, etc. — normal edit/run **without** packaging.

6. **Build (`dockpipe build` / `compile all`)** is considered successful only if:
   - **`compile`** order is **core → resolvers → workflows**,
   - each **workflow** and **resolver** package resolves a **valid `namespace`** (from **`package.yml`**, **`config.yml` / `resolver.yaml`**, or repo-root **`dockpipe.config.json`** **`packages.namespace`** as a default),
   - every **`depends`** entry in **`package.yml`** names a package **already present** in the compiled store (names from **`package.yml`** under **`core/`**, **`resolvers/`**, **`workflows/`**).

**Runtime** may still resolve workflows from **project-local tarballs** under **`bin/.dockpipe/internal/packages/workflows/`**, explicit local package sources from **`packages.sources`** in **`dockpipe.config.json`**, **global packages** under **`<global-root>/packages/workflows/`**, or **`packages.tarball_dir`** / **`release/artifacts`** when no on-disk workflow config wins; optional **`packages.namespace`** filters tarball choice when set.

## State roots

| Scope | Root | What belongs there |
|------|------|--------------------|
| **Project-local disposable** | **`<workdir>/bin/.dockpipe/`** | Compile outputs, project package store, run records, caches, workflow artifacts, and package runtime products. The entire tree is disposable by convention. |
| **Project/package durable** | OS state root keyed by stable project ID and exact package owner ID | Owner-only settings, credentials, continuity/recovery authority, and cumulative records that must survive ordinary clean. Resolve with `dockpipe scope --package <owner-id>`. |
| **Global** | **`DOCKPIPE_GLOBAL_ROOT`** or the OS user data directory | User-wide installed core, resolver/workflow packages, global image artifact metadata, global download cache. |
| **Published remote** | Static origin / package registry / OCI registry | Versioned package tarballs and OCI image refs. |

The OS-selected durable storage base may use a filesystem alias, such as
`/home` pointing to `/var/home`. Dockpipe resolves that base before creating its
state directory. Links inside the Dockpipe-owned state directory remain rejected.

Use the public `dockpipe scope` commands or package SDK helpers when scripts need
state paths; do not hardcode another user's cache or state location.

## Published packages and source projects

Normal installers provide CLI/core. Install optional packages from a published
catalog with `dockpipe package install` or the launcher Marketplace. No Dockpipe
source checkout or release workflow is required.

The launcher shows compressed package sizes in **Marketplace** and **Installed**.
Marketplace sizes come from the catalog's optional `size_bytes` field; store builds
measure each finished archive, and `dockpipe package catalog` preserves that value.
`dockpipe package list --format json` reports the actual local archive size using
the same field. Older catalogs and unpacked package directories omit it and display
**Not available**. These sizes exclude extracted runtime files, caches and separate
dependencies; they are not total disk-usage estimates.

For a reproducible setup, use an immutable manifest URL and the selected package's
checksum. When authoring your own packages, keep YAML, scripts and assets together
and declare dependencies so another user can run them without your source tree.

### Independent host and package releases

The CLI/core and desktop host release independently of optional packages. The launcher remains part of the Dockpipe repository, and CLI-only users do not need it. Host releases publish their download pointer at `packages/runtime/latest.json`; Marketplace discovery uses the separately promoted `packages/latest.json` catalog. Releasing the host does not replace the package catalog or require VM/Cockpit packages to be installed.

Packages use published SDKs and declare `min_dockpipe_version` for required engine features. Their build repository can pin one exact producer with `dockpipe_version` and a checksum lock. Runtime compatibility, producer reproducibility and native plugin ABI checks are separate contracts. See [release boundaries](../../release/docs/releasing.md#runtime-and-optional-package-release-boundaries).

### Hidden packages

Set `hidden: true` in an installable workflow, resolver, or core `package.yml`
to omit it from normal catalog discovery and the launcher's Marketplace.
The default is `false`. Each installable slice has its own flag; an umbrella
`kind: package` manifest does not apply it to child slices.

```yaml
schema: 1
kind: workflow
name: experimental-tool
hidden: true
```

Use `dockpipe package catalog --remote <HTTPS manifest URL> --include-hidden`
to inspect every entry. Explicit installation by kind/name and dependency
resolution still include hidden entries. Existing installations remain listed
and usable. Recompile and publish the updated catalog for this setting to reach
remote users; older clients may still list the entries.

This is a discovery preference, not access control or a licensing restriction.
A hidden artifact on a public origin remains downloadable.

## Authoring vs execution (two modes, both supported)

| Mode | What you run | Friction | Notes |
|------|----------------|----------|--------|
| **Source** | Workflow YAML from **`workflows/`**, legacy **`templates/<name>/`**, etc. | **Low** for day-to-day editing — no compile step required. | **`scripts/…`** resolves per **`paths.go`** (project **`scripts/`** first, then bundled **resolvers** / **bundles** / **`assets/scripts/`**). Users can keep scripts wherever those rules allow. |
| **Compiled / packaged** | **`packages/workflows/`** (tarballs), **`packages/resolvers/`**, **`packages/core/`**, from **`compile all`** under **`bin/.dockpipe/internal/packages/`**. | **One** compile (or CI) before run. | **Cleaner** tree: optional **`package.yml`** per slice; resolver search prefers **`packages/resolvers/`** when present. |
| **Global installed** | **`<global-root>/packages/workflows/`**, **`<global-root>/packages/resolvers/`**, **`<global-root>/templates/core/`**. | **One install/update** per user or machine. | Shared extensions available to many projects without copying them into each repo. |

**Authors are not forced to pick one path:** keep editing and running from source for low friction; use **compile → package → release** when you want a **self-contained** published artifact.

### Authoring: workflow YAML vs resolver / runtime / strategy slices

A **workflow** is primarily **`config.yml`** (plus assets next to it). In that file you **declare** what to run and **which profiles** to merge — you do **not** embed full resolver or strategy trees inside the workflow directory (unless you choose colocated assets for scripts/images).

| Concern | Where definitions usually live | What the workflow YAML does |
|--------|--------------------------------|-----------------------------|
| **Runtime** (substrate) | **Only in core:** **`templates/core/runtimes/<name>/`** (or bundled **`src/core/runtimes/`**) — substrates are an **engine** concept. | **`runtime`** references an existing profile **name**. Top-level **`runtime`** sets the workflow default; a step may override it. Workflows **do not** define or override substrate types; adding a new runtime is a **core** change. |
| **Resolver** (tool / env profile) | **`templates/core/resolvers/<name>/`** or maintainer trees listed under **`compile.workflows`** (e.g. nested **`…/resolvers/codex/`** with **`profile`**) | **`resolver`** names the profile. Top-level **`resolver`** sets the workflow default; a step may override it. Package metadata may still declare **`capability`**, but normal workflow authoring should lead with **`resolver`**. |
| **Security policy** | **Core-owned presets** plus engine defaults, compiled into the effective runtime manifest. | Workflow YAML may select `security.profile` and apply bounded `network`, `filesystem`, and `process` overrides. It does **not** expose raw Docker flags or define a second runtime system. |
| **Strategy** (lifecycle wrapper) | **`templates/core/strategies/<name>/`** | **`strategy`**, **`strategies:`** select host before/after scripts. |
| **Package-owned workflows** | Same as workflows: **`config.yml`** under e.g. **`dockpipe/dorkpipe/`** or nested under **`dockpipe/<group>/resolvers/<name>/`** (resolver-shaped trees include **`profile`** + workflow assets) | **`scripts/…`** resolves via compiled **`dockpipe-workflow-*`** tarballs first, then source trees. |

So **one repo** can ship **workflows**, **resolvers**, and **runtimes** / **strategies** / the rest of the spine via **`compile core`** ( **`templates/core`** or **`src/core`** ): list **`compile.workflows`** as the entry point. Resolver packaging uses the same roots as workflows plus flat **`src/core/resolvers`** and **`templates/core/resolvers`** when those exist. **Runtimes** still live under **core**; workflows only select them. **Compile** emits **`dockpipe-workflow-*`**, **`dockpipe-resolver-*`**, and **`core`** tarballs while **`package.yml`** records **`depends`**, **`namespace`**, and metadata. Step-by-step keys: **[../workflows/workflow-yaml.md](../workflows/workflow-yaml.md)**.

### Local compile (`dockpipe build` / `dockpipe package compile` / `dockpipe compile`)

Materialize a **project-local** store under **`bin/.dockpipe/internal/packages/`** without moving authoring trees.

**Repo-root `dockpipe.config.json` (optional)** stores repository settings.

Set the top-level `dockpipe_version` string to pin a repository to one exact
Dockpipe executable version, for example `"dockpipe_version": "0.6.24"`.
This belongs in repository configuration rather than `package.yml`, whose
`min_dockpipe_version` remains a package compatibility lower bound. The pin accepts
complete SemVer, including prerelease/build identifiers, without a `v` prefix.
Ranges, `latest`, partial versions, empty strings and development tags are invalid.
The running executable must match exactly, including build metadata; a project
`VERSION` file cannot satisfy the check. An unversioned development executable
also fails an explicit pin. Omitting the field preserves normal behavior.

The CLI checks the current/`--workdir` project's nearest configuration before
workflow resolution, dependency compilation, or execution commands dispatch.
Help, version, diagnostics and installation/bootstrap commands remain available
to repair a mismatch. `--help` passed after the command separator is workflow
input and does not bypass the check. The pin selects no source checkout and
compiles no external repository; install the matching release separately.

The authoritative editor schema is
[`dockpipe.config.schema.json`](../../src/lib/infrastructure/schema/dockpipe.config.schema.json).
See [editor schema association](editor-extensions.md#project-configuration-schema).

The `compile` object supports `workflows` and `core_from`; `secrets` and
`packages` are top-level objects:

- **`workflows`** — array of repo-relative (or absolute) **directories** to scan for named workflow folders (each with **`config.yml`**). Same roots drive **`dockpipe package compile resolvers`** (nested **`…/resolvers/<name>/`**) plus flat **`src/core/resolvers`** and **`templates/core/resolvers`** when present.
- **`core_from`** — optional path override for **`compile core`** (same as **`--from`**).
- **`secrets`** (optional) — not secrets themselves; pointers for humans and tooling:
  - **`op_inject_template`** — repo-relative or absolute path to a **mapping file** for **`op inject`** (e.g. **`.env.op.template`** with **`op://`** lines). **`dockpipe doctor`** reports whether that file exists when **`dockpipe.config.json`** is present in the current directory.
  - **`environment`**, **`environments`** — default selection and named resolver-backed environments, with explicit variable bindings. See [Secret environments](../runtime/vault.md).
  - **`notes`** — free-text reminder (e.g. vault naming, policy).
- **`packages.sources`** (optional) — explicit local package sources used for resolution without publishing first:
  - **`{ "kind": "store", "path": "C:\\Source\\dockpipe\\bin\\.dockpipe\\internal\\packages" }`** for a compiled package store root
  - **`{ "kind": "tarball_dir", "path": "release/artifacts" }`** for a directory of **`dockpipe-*.tar.gz`** artifacts
  - these supplement local/global/system package roots and do **not** replace **`packages.tarball_dir`**, namespace filtering, or self-hosted S3/R2 package mirrors

If **`dockpipe.config.json`** is **missing**, compile uses built-in defaults for each omitted key. **`dockpipe init`** seeds a starter JSON. **Maintainer package trees** (for example **`packages/`** in this repo) are **not** implied: add them explicitly under **`compile.workflows`** when you want them compiled.

Compile steps:

1. **`compile core`** — copies **`src/core`** (default when **`src/core/runtimes`** exists) or **`templates/core`**, or **`compile.core_from`** / **`--from`**, into **`packages/core/`** and writes **`package.yml`** (`kind: core`). **Omits** top-level **`resolvers/`**, **`bundles/`**, and **`workflows/`** from that copy so those slices stay separate packages.
2. **`compile resolvers`** — repeatable **`--from`**; default roots are **`compile.workflows`**, **`src/core/resolvers`**, and **`templates/core/resolvers`** when those directories exist. **Pack roots** under each **`--from`** directory include: **`resolvers/`** (flat); **`packages/<group>/resolvers/`**; **`dockpipe/<group>/resolvers/`** (e.g. dockpipe repo: **`agent`**, **`ide`**, **`secrets`**, **`cloud/storage`**, …). Every source leaf under each pack root that contains **`profile`** or **`config.yml`** becomes **one** tarball (**`dockpipe-resolver-<name>-…`**) — the **store** still lists **separate** installable resolvers. Asset-only resolver packages may omit **`profile`**; grouping directories without either marker are traversed. **`src/core/resolvers`** stays a flat list (no nested **`resolvers/resolvers/`**). **Strategies** and **runtimes** are not packed from the same vendor folder as resolvers; keep lifecycle slices in **`compile core`** until a follow-up convention exists.
3. **`compile workflows`** — every **`config.yml`** under each **`--from`** root (recursive walk). With no config, the default is **`workflows/`** when present. List maintainer roots (for example **`packages/`**) in **`compile.workflows`** when you want them compiled. **`dockpipe package compile bundles`** is an alias for **`compile workflows`**. Optional **`compile_hooks:`** in workflow/resolver `config.yml` is a list of **shell** strings run against the staged package copy **after** validation and **before** the tarball is written; Dockpipe exports **`DOCKPIPE_COMPILE_WORKDIR`**, **`DOCKPIPE_COMPILE_SOURCE_DIR`**, **`DOCKPIPE_COMPILE_STAGING_DIR`**, and **`DOCKPIPE_COMPILE_KIND`** so hooks can read source-only inputs while writing packaged outputs into the staged tree (e.g. **`go build`**, codegen, asset bundling).
4. **`compile all`** — runs **core → resolvers → workflows** and emits compiled runtime/security/image manifests, but does not run Docker builds.
5. **`dockpipe build`** — runs **compile all --force**, then prebuilds Dockerfile-backed image artifacts by default and records materialized receipts under **`bin/.dockpipe/internal/images/by-fingerprint/`**. **`dockpipe run`** can reuse those receipts before deciding whether a Docker build is needed. Use **`dockpipe build --no-images`** when you want manifest/package materialization only. **`dockpipe clean --dry-run`** previews the exact checkout-local **`bin/.dockpipe/`** tree and logical bytes; ordinary **`dockpipe clean`** removes that complete disposable tree. **`dockpipe rebuild`** separately resets the resolved compiled store, including `DOCKPIPE_PACKAGES_ROOT`, through guarded target validation before building; ordinary clean never follows that override outside the checkout.

The runner checks compiled package-store roots before authoring **`CoreDir`** so you can opt in to the compiled store per workdir. Declare **namespaces**, **`depends`**, and metadata in the authored **`package.yml`** before compiling; do not rely on edits to generated package output.

## Names, descriptions and discovery

Author catalog metadata in `package.yml`. Workflow-backed packages can also declare
these fields in `config.yml`:

```yaml
schema: 1
kind: workflow
name: editor-workspace
title: Browser development workspace
description: Open a containerized editor in your browser. Requires Docker.
catalog_type: app
categories: [development]
```

`name` and `kind` remain the package's install identity. Use `title` for a readable
display name and `description` to explain what the package provides, its purpose
and important prerequisites. For third-party tool integrations, describe the
Dockpipe integration rather than implying that Dockpipe distributes the vendor's
product itself.

`catalog_type` controls discovery: `app`, `workflow`, `integration`, or `core`.
For example, an app can still install with `--kind workflow`; a resolver normally
appears as an integration. `categories` adds purpose filters such as `development`,
`ai`, `secrets`, or `virtual-machines`. It accepts at most 16 unique lowercase
slugs, each starting with a letter and containing only letters, digits and hyphens,
up to 64 characters. These fields do not grant capabilities or change execution.

During workflow/resolver compilation, nonempty manifest strings override matching
workflow metadata. An explicit manifest `categories: []` clears workflow categories.
Missing titles fall back to the package name; missing types use workflow metadata,
legacy `category: app`, then the package kind (`resolver` becomes `integration`).
An explicit workflow `catalog_type` takes precedence over legacy `category: app`.
Core packages take metadata directly from their manifest.

Store manifests, the remote catalog and installed-package inventory carry these
fields. The launcher and public catalog display titles and filter discovery types;
search also matches stable IDs and purpose categories. Older published catalogs
remain readable with name/kind fallbacks.

To update published metadata, edit the authored source, compile and publish a new
package version, then rebuild the consuming catalog/site. Existing installed
archives retain their metadata until upgraded. Do not edit generated stores or
replace an immutable release solely to refresh its description.

## Remote catalogs and user installation

The [exact package selection proposal](version-pinning.md) distinguishes the
current checksum-asserted install from future project/package version locks and
runtime selection. Its lock and workflow-call formats are not implemented in
0.6.17.

`dockpipe package catalog --remote <HTTPS manifest URL>` returns JSON for the current
execution platform (OS/architecture and, inside Flatpak, runtime identity). It accepts a latest pointer, release catalog, or explicit platform
store manifest. Latest pointers use origin-relative `manifest` paths; release catalogs
use release-relative `stores.<platform>.manifest` paths. Store manifests retain the
existing schema-1 `packages.core`, `packages.workflows`, and `packages.resolvers` layout.
The response includes the resolved store `manifest`, `platform`, and flattened `packages`.
An explicit store URL must declare a compatible platform; native and Flatpak stores cannot be substituted.

`dockpipe package install --remote <URL> --kind workflow --name <name>` installs the selected
archive under the user package root in the appropriate `core`, `workflows`, or `resolvers`
subdirectory. `DOCKPIPE_GLOBAL_ROOT` selects another user root. The command prints JSON with
its installed `path`. Use the catalog response's resolved manifest URL and `--sha256 <digest>`
to pin a selection; a changed checksum is rejected before downloading the archive.

### Alternate archive encodings

Gzip remains the default and the store builder continues to emit its existing
`.tar.gz` archives. A publisher may additionally attach `variants` to a schema-1
store entry. Older clients ignore this optional field and keep installing gzip.
Each variant describes the same complete, uncompressed tar stream:

```json
{
  "codec": "zstd",
  "tarball": "dockpipe-workflow-example-1.2.3.tar.zst",
  "sha256": "<SHA-256 of the encoded archive>",
  "size_bytes": 1234,
  "decoded_sha256": "<SHA-256 of the complete tar stream>",
  "decoded_size_bytes": 5678
}
```

Entries with variants also declare `decoded_sha256` and `decoded_size_bytes` on
the canonical gzip entry. These values must agree across every encoding; installation
checks the selected archive's actual decoded bytes against them.

Use `dockpipe package install --remote <URL> --kind workflow --name example
--codec zstd` to select an alternative explicitly. `--sha256` then binds the
selected encoding's digest. The selected package must publish that encoding;
dependencies prefer it when available and otherwise use their canonical archive.
A corrupt alternative or unavailable decoder fails installation without silently
retrying another encoding. Marketplace continues to use the canonical gzip
selection. These options do not change package version resolution.

The store retains the exact downloaded bytes, with a separate filename and
SHA-keyed extraction cache per encoding. Existing versions and gzip artifacts
coexist. Readers, listing and optional-package removal support these archives;
same-version discovery prefers gzip, then Zstd, then other codec IDs in lexical
order. Runtime package reads remain local.

Gzip and Zstd decoding are built into the runtime. Additional decoders are
explicit operator configuration: set `DOCKPIPE_ARCHIVE_DECODERS` to an absolute
JSON file path. A catalog can supply codec IDs and integrity metadata, never
decoder executables, arguments, environment variables or download instructions.
For example, a locally trusted provider can be registered with:

```json
{
  "schema": 1,
  "providers": {
    "example": {
      "executable": "/absolute/path/to/decoder",
      "sha256": "<executable SHA-256>",
      "args": ["decode"],
      "files": [
        {
          "path": "/absolute/path/to/libdecoder.so",
          "sha256": "<library SHA-256>",
          "name": "lib/libdecoder.so"
        }
      ]
    }
  }
}
```

Such an archive uses `.tar.example`. The runtime verifies and stages the pinned
executable and dependencies in a private temporary directory, supplies encoded
bytes on stdin and accepts decoded tar bytes on stdout. It runs a supervised
process without a shell, with a bounded diagnostic stream and a ten-minute
deadline. Decoder failure, cancellation and output overflow prevent publication
and clean up temporary files. This is permission to execute the operator-selected
decoder; the downloaded package cannot register a provider. Private SDKs can stay
inside their owning package's distribution and setup workflows.

Catalog downloads remain limited to 512 MiB per archive and decoded tar streams
to 2 GiB. The built-in Zstd reader accepts windows up to 64 MiB. Provider
configuration is limited to 1 MiB and staged decoder files to
64 MiB in total. A provider may impose a smaller supported input or output size;
publish gzip or Zstd for packages outside that provider's supported range. Decoder
memory use and temporary-file I/O are part of the consumer cost and must be
included in real installation measurements.

### Installation verification

Installation requires HTTPS, restricts catalog references and redirects to the selected
origin, bounds manifest/download/expanded sizes, and verifies SHA-256 and archive metadata.
Catalog paths cannot traverse directories. Archives may contain only files and directories
inside their declared package; links, duplicate members, conflicting metadata, and unsafe
paths are rejected. An omitted package.yml version uses the store's version, matching the
existing build contract. The completed archive is renamed into the user store only after
verification. Failed verification preserves an existing package; publication atomicity follows
the host filesystem's rename guarantees. Installing the same filename replaces that user copy.
Other versions remain installed, and existing runtime resolution precedence still applies.

These checks establish integrity against the chosen HTTPS catalog; they are not independent
publisher signatures. Configure origins you trust to supply executable packages. Installation
does not execute package code, modify the system/Brew store, or fetch anything
during ordinary workflow resolution. Explicit package install prepares declared
`depends` and `requires_resolvers` from the selected platform catalog before
publishing them; host executable installation uses a separate approval flow.
Normal installers supply only core; optional packages are selected explicitly
from the remote catalog.
The complete store is a separate opt-in download.

`dockpipe package list --format json` returns `packages`, `warnings`, and `install_root`,
covering project, configured, user, and system package stores. Entries include name, version,
kind, description, source, path, and `removable`. Only optional user-store archives are removable. This is an inventory of available packages; a displayed
entry does not promise it wins every workflow's resolution precedence. The existing text
listing remains scoped to the project store.

`dockpipe package uninstall --path <absolute archive path>` removes the selected optional
workflow or resolver archive from the user store. Use a `removable: true` entry from the
JSON inventory. Core, system/installer packages, project/configured stores outside that
root, compiled directories, and symbolic links are rejected. Uninstall retains package
state, settings, caches, other versions, and dependencies; it never recursively deletes
a package tree. The response contains the removed `path`.

The launcher offers **Uninstall** on removable Installed entries and explains why managed
entries are protected. Marketplace marks a matching name/kind/version **Installed** and
disables duplicate installation; matching versions do not prove identical staging content.
**Cancel operation** is visible only while a request is active and stops both catalog and
inventory requests, with a bounded forced stop if the process does not exit.

## Network boundary: install, not every `run`

**HTTPS / CDN / registry traffic** should be confined to **explicit install (and publish)** commands — e.g. **`dockpipe install core`**, **`dockpipe package catalog/install`**, **`dockpipe release upload`**. After artifacts are on disk, **`dockpipe run`** against local workflows or installed packages should **not** need network unless the **workflow itself** does (e.g. `docker pull`, API calls).

Package metadata may declare an OCI image reference as a **hint/reference**:

```yaml
image:
  source: registry
  ref: ghcr.io/acme/tool@sha256:...
  pull_policy: if-missing
```

The package manifest does **not** become runtime truth. Compile folds the reference into the effective runtime/image manifests, and run consumes those compiled manifests. If the image is already local and valid, run stays local. If it is missing, run may pull only when the compiled pull policy and compiled network policy allow it.

## Lifecycle: compile → package → release

1. **`compile`** — Validate workflow YAML and **materialize** a **self-contained** tree: copy the workflow and (as the implementation grows) **pull in domain-specific assets** referenced from source so the compiled directory is the **single** execution root for that package.
2. **`package`** — Archive **that compiled tree** (plus **`package.yml`**, checksums, optional lock metadata). **`dockpipe package build store`** turns the compiled store into **gzip tarballs** (one per core / workflow / resolver) and **`packages-store-manifest.json`**. **`dockpipe run --workflow`** can **stream** a workflow from **`dockpipe-workflow-<name>-*.tar.gz`** when on-disk paths do not win. An explicit **`config.yml` namespace** remains authoritative; when it is absent, a **`packages.namespace`** filter can match the namespace in the same archive's validated **`package.yml`** (including the repository default recorded during compilation) (see **`packages.tarball_dir`** / **`packages.namespace`** in **`dockpipe.config.json`**). **`dockpipe package build core`** builds the **`templates/core`** artifact for **`dockpipe install core`**.
3. **`release`** — Upload tarball + manifest to your **static origin**; consumers **`install`** to pull it.

**CI** can chain the same steps locally: **compile → package → release**.

## Workflow install and resolver dependencies

**`dockpipe package install`** installs the selected package and its transitive **`depends`** and **`requires_resolvers`** dependencies from the same platform catalog. It verifies every archive and compatibility requirement in a temporary store before publishing dependencies first and the selected package last. Missing dependencies, ambiguous names, cycles, checksum failures and incompatible engine versions fail preparation without changing installed packages. A publication I/O failure or cancellation can leave already-published dependencies in place; the error reports that partial outcome. Closure limits are 128 packages, 64 dependency levels and 2 GiB of downloaded archives (one bounded archive may be downloaded before an inaccurate size is detected). **Domain-specific** scripts and assets belong **inside** the workflow package / compile output — not as a separate CDN hop for every file at run time. **Resolver** packages remain **shared adapters** (tool profiles, resolver-owned assets).

Installed dependency scripts are resolved from global and system package stores
before bundled fallback assets. Tarball-backed workflows use their extracted
workflow directory at runtime. Recompilation retains an extraction while it is
still the compiler's source; changed archives use distinct content-addressed caches.

Package dependencies should remain package-shaped:

- **`depends`** names other package ids. A name matching multiple package kinds is rejected; use **`requires_resolvers`** to identify a required resolver explicitly.
- **`requires_resolvers`** names resolver profile ids that must be available project-locally, globally, or from the install closure. Workflows that use logical resolver script ids such as **`scripts/dorkpipe/...`** should declare the owning resolver here. A declared **`depends`** or workflow **`inject`** edge to another workflow that declares the resolver also satisfies this check through the workflow dependency closure. Merely having an unrelated resolver installed does not. Direct declarations remain the clearest choice for new workflows and allow source compilation before workflow dependencies are available.
- **`requires_capabilities`** names dotted capability ids for catalog/search and dependency checks.
- **`platforms`** declares which host platforms the package supports, and **`dependencies.host`** names required external executables with OS-specific install commands. Core preflights these before workflow execution. Tools bundled under the selected package’s `assets/tooling/bin` take precedence over host executables during preflight and in host-script `PATH`. Flatpak packages declare `platforms: [flatpak]` and supply runtime-compatible dependencies; native distribution installers are disabled in Flatpak.
- **`image`** may point at a normal OCI reference, ideally digest-pinned, but Docker layers remain in Docker/OCI registries rather than Dockpipe package tarballs.

Security metadata in **`package.yml`** should stay compatibility-only, for example **`compatible_security_profiles`** or **`requires_network: true`** if added later. Effective network, filesystem, process, and Docker enforcement settings belong only in compiled runtime manifests.

## Distribution split (core and optional packages)

| What | Where it usually lives | Notes |
|------|-------------------------|--------|
| **Runtimes** | **Installed core** | Stable, light profiles — **not** the main “store” surface. |
| **Strategies** | **Installed core** | Lifecycle wiring shipped with core. |
| **Compiled core** | **Application installer / core package** | Required runtime assets; ordinary users receive them with the application. |
| **Resolvers** | **Bundled** and/or **store packages** | **Plugin adapters** — shared across workflows; extended catalogs ship as packages with rich **`package.yml`**. |
| **Workflows** | **Authoring tree**, **`bin/.dockpipe/internal/packages/workflows/`**, or **store** | **Primary rich-metadata** packages for authoring and discovery (`kind: workflow`). |

**CLI + core** provide the base application. **Workflows** and **resolver** packages are the optional features you browse, version and install.

## 1. Packages (installed, store-backed)

**Packages** are **self-contained** artifacts you fetch from an object store (e.g. **Cloudflare R2** behind HTTPS) or another registry. They are **building blocks** for YAML workflows: full workflows, slices of **`templates/core`** (resolvers, runtimes, strategies, assets), or extra workflow tarballs from package installs.

- **Default project compile layout:** **`<workdir>/bin/.dockpipe/internal/packages/`** — kept under **`internal/`** so user-created and installable packages stay separate from other **`bin/.dockpipe/`** state (runs, handoffs, CI).
  Override with **`DOCKPIPE_PACKAGES_ROOT`** (absolute path, or relative to workdir), e.g. **`vendor/dockpipe-packages`** if you want packages **versioned in git** without fighting a blanket **`.dockpipe/`** ignore.

Suggested subdirectories (mirror authoring concepts; not all are required):

| Path | Role |
|------|------|
| **`bin/.dockpipe/internal/packages/workflows/<name>/`** | Workflow-shaped trees (`config.yml`, steps, …). |
| **`bin/.dockpipe/internal/packages/core/`** | Compiled **spine** only: **`runtimes/`**, **`strategies/`**, **`assets/`**, etc. — not resolver/bundle/workflow packages. |
| **`bin/.dockpipe/internal/packages/resolvers/<name>/`** | One resolver package per profile (same shape as **`templates/core/resolvers/<name>/`**). |
| **`bin/.dockpipe/internal/packages/workflows/`** | Workflow packages (**`dockpipe-workflow-*`**). |
| **`bin/.dockpipe/internal/packages/assets/`** | Optional top-level packs (e.g. large binaries) that are not folded under **`core/`**. |

**Metadata:** each installable unit should include **`package.yml`** next to its payload (see **`dockpipe package manifest`**). Core fields: **`name`**, **`version`**, **`title`**, **`description`**, **`author`**, **`website`**, **`license`**, **`kind`** (`workflow` \| `resolver` \| `core` \| `assets` \| `bundle`). Optional **`namespace`** — same rules as workflow **`config.yml`** **`namespace:`** (lowercase label; reserved words like **`dockpipe`**, **`core`**, **`system`** are rejected).

**Versioning:** `package.yml version` is the independent release identity of that package;
it does not need to match the CLI's repo-root `VERSION`. Bump a package when its
published contents change. Generated workflow/resolver manifests inherit the nearest
explicitly versioned owning `package.yml`, searching from the source directory up to
`--workdir`. An explicit child version wins. External sources consult only their own
manifest; loose sources without an owner fall back to repo-root `VERSION` (or `0.0.0`).
An inherited version change invalidates compiled child output even when the child's
files have not changed. Core remains tied to the CLI version. Store assembly preserves
package versions; its `--version` is only a fallback, not a bulk version override.
Versions must be semver-shaped so tarball names and CDN paths stay stable.

**Rich metadata (authoring & store discovery)** — optional but recommended for **workflow** and **resolver** packages:

| Field | Purpose |
|-------|---------|
| **`provider`** | Optional platform or vendor id for filtering and catalog facets (e.g. **`cloudflare`**, **`aws`**, **`github`**) — short stable label, not a URL |
| **`capability`** | For **`kind: resolver`** — dotted id this package provides (e.g. **`cli.codex`**, **`blob.storage`**) — see **[../concepts/capabilities.md](../concepts/capabilities.md)** |
| **`requires_capabilities`** | For **`kind: workflow`** — dotted capability ids this workflow expects (complements **`requires_resolvers`**) |
| **`tags`**, **`keywords`** | Faceted search / UI filters |
| **`min_dockpipe_version`** | Optional complete semantic-version minimum, e.g. `0.6.14`; checked before package installation and workflow/resolver loading |
| **`repository`** | Source repo URL |
| **`provides`** | Resolver capability names (e.g. tool ids) for **`kind: resolver`** |
| **`requires_resolvers`** | Hint compatible resolver profiles for **`kind: workflow`** |
| **`depends`** | Other package **names** this package expects |
| **`dependencies`** | Supported host platforms plus external host tools needed by this package's workflows/scripts, separate from package graph **`depends`** |
| **`namespace`** | Validated author/org identity used for discovery and packaged workflow selection |
| **`package_state`** | Optional maintained mixed-state contract: `compatibility_import: package-owned` plus exact `owner_ids`. Undeclared public owners use conservative whole-tree compatibility import. |
| **`allow_clone`** | If **`true`**, **`dockpipe clone`** may export the compiled tree to **`workflows/`**; if false or omitted, clone is refused. |
| **`distribution`** | Optional hint: **`source`** or **`binary`** (documentation for store pages). |
| **`image`** | Optional normal OCI image reference for a workflow package. Compile records it into the effective runtime/image manifests; `run` may reuse or pull it according to compiled pull policy. |

Use `dockpipe package manifest` to inspect example package metadata.

**Compression:** store objects are typically **`.tar.gz`** (or **`.tar.zst`** later) to keep bandwidth and R2 storage small; the CLI unpacks into the layout above. **Binary-only** packs are possible for asset-only packages if you add a small unpack step later.

**CLI today:** **`dockpipe install core`** pulls **`templates/core`** into **`templates/core`** (bootstrap path). **`dockpipe package compile workflow`** validates YAML and copies a workflow tree into **`bin/.dockpipe/internal/packages/workflows/<name>/`** (see **`docs/cli-reference.md`**). Future **`dockpipe install package …`** will unpack registry artifacts and extend resolution order (project **`workflows/`** + **installed packages** + embedded bundle).

### Clone, education, and commercial packages

- **`package.yml`** may set **`allow_clone: true`** so **`dockpipe clone <name>`** can copy a compiled workflow package from **`bin/.dockpipe/internal/packages/workflows/<name>/`** into **`workflows/<name>/`** (or **`--to`**). This supports **education** and **forking** when the author opts in.
- **`allow_clone: false`** or omitting **`allow_clone`** means **`dockpipe clone`** refuses — appropriate for **commercial** or **binary-only** releases where the publisher does not grant a recoverable source tree.
- Optional **`distribution: binary`** documents that the published artifact is not meant for source recovery; publishers who need **strong** protection should ship **only** non-recoverable binaries (obfuscation, native modules, etc.) — Dockpipe cannot cryptographically enforce that; **`allow_clone`** is the explicit license bit for **`dockpipe clone`**.
- **`dockpipe package compile workflow`** writes **`allow_clone: true`** and **`distribution: source`** into a generated **`package.yml`** so local compiles stay cloneable unless you edit the manifest before release.

## 2. Uncompressed working tree (authoring / clone)

**Not** “packages” in this sense: the repo you edit every day.

- **`workflows/`** (default project root), **`src/core/workflows/<name>/`** (bundled examples in the dockpipe repo), legacy **`templates/<name>/`** — YAML and assets you **build or clone**, commit, and run with **`--workflow`** as today.
- No **`package.yml`** required; this is normal development.

## 3. Disposable runtime and durable package state

**`bin/.dockpipe/`** is the **project-local disposable** tree for generated state: host run records (**`bin/.dockpipe/runs/`**), workflow artifact roots, collision-safe package runtime directories, handoffs, optional demo stubs, caches, and **installed package material** under **`bin/.dockpipe/internal/`**. Step `outputs:` files are resolved under the selected workflow output/artifact scope. Every producer in this tree must tolerate its complete removal.

Durable package state is separate. `dockpipe scope --package <owner-id>` and `dockpipe get package_state_dir` resolve an owner-only OS state directory keyed by stable project identity plus the exact trimmed, case-preserving owner ID. `DOCKPIPE_PACKAGES_ROOT`, `DOCKPIPE_GLOBAL_ROOT`, checkout basename, remote URL, and the legacy sanitized scope do not relocate or identify it. Public suffixes are validated as data and cannot traverse outside that root.

On first public access, an undeclared third-party scope conservatively copies its complete validated legacy `bin/.dockpipe/packages/<sanitized-scope>` tree into durable state with byte inventory, source identity, owner-only permissions, interruption recovery, and atomic publication. Legacy bytes remain untouched; durable state wins after publication and divergence is reported without automatic merge. Maintained mixed packages instead declare exact package-owned migration aliases:

```yaml
package_state:
  compatibility_import: package-owned
  owner_ids: [my-package, my-resolver]
```

That declaration means package code exhaustively selects durable cohorts and leaves caches, build output, scratch, and unresolved cohorts in package runtime. Dockpipe propagates the selected package manifest through package/workflow script context so the generic importer can match exact owner IDs without engine knowledge of maintained packages. It is not a retention label and does not authorize deleting or rewriting legacy state. Unknown scopes have no declaration and therefore import whole. There is no durable-to-legacy symlink or cross-project/name-folding heuristic.

Package scripts should use `PackageRuntimeDir` or shell SDK `dockpipe_sdk path package-runtime <owner-id> ...` for disposable products. `DOCKPIPE_PACKAGE_STATE_DIR` is accepted only when it equals the resolved durable directory or names an independently existing owner-only override outside the checkout; workdir refresh clears inherited state authority.

`dockpipe clean --dry-run` needs no project declaration. It deterministically reports the exact
resolved checkout **`bin/.dockpipe/`** target, logical bytes, file count, and whether removal would
occur. Ordinary clean removes that whole tree after rejecting links/reparse points, special files,
filesystem substitutions, and unsafe boundaries. A missing tree is a reported no-op. Durable state
and external `DOCKPIPE_PACKAGES_ROOT` stores remain outside ordinary clean authority.

**`bin/.dockpipe/internal/packages/`** is the default store for **fetched or compiled** package trees (workflows, core slices, assets) — the same conceptual layout whether content arrived as a **`.tar.gz`** or from **`dockpipe package compile workflow`**. **Uncompressed** authoring under **`workflows/`** remains normal; **compile** validates and **materializes** into **`bin/.dockpipe/internal/...`** when you opt into the packaged path. Resolution order for **`--workflow`** is implemented in **`workflow_dirs.go`** (**`workflows/`** → **packages** → legacy **`templates/`** paths, etc.).

Image artifact metadata is **not** a package payload by default. See
**[../runtime/image-artifacts.md](../runtime/image-artifacts.md)** for the user-facing lifecycle. Use:

- **`bin/.dockpipe/internal/images/`** for project-local image artifact indexes.
- **`bin/.dockpipe/internal/cache/images/`** for project-local cached image artifact records.
- **`<global-root>/images/`** for global image artifact indexes.

Compiled workflow tarballs may contain **`.dockpipe/image-artifact.json`** and per-step image manifests as inspectable compiled truth, but the local/global image index is separate from **`packages/`** so packages, runtime manifests, image artifacts, and run records do not blur together.

## 4. Global installs

### Execution environment and Marketplace artifacts

Native processes select `OS-ARCH` Marketplace stores. Inside Flatpak, Dockpipe reads
`/.flatpak-info` and selects `linux-ARCH-flatpak-RUNTIME_ID-RUNTIME_BRANCH`.
Installing Flatpak or using an atomic distribution does not switch a native CLI.

Store builds record this key as `platform` in `packages-store-manifest.json`.
Release catalogs index stores by the same key. Direct/pinned store URLs must also
match; unlabelled legacy stores remain native-only. Missing Flatpak artifacts fail
explicitly instead of falling back to native packages.

Optional packages remain Dockpipe archives with dependencies supplied by the
runtime or explicitly bundled and tested by authors. See the
[Flatpak build contract](../../release/packaging/desktop/flatpak/README.md).
The base installer continues to contain required core only.

### User stores

Global installs are for user-wide Dockpipe extensions. They do not live under **`bin/.dockpipe`** because there may be no project checkout involved.

The user package store contains `packages/core/`, `packages/workflows/` and
`packages/resolvers/`. Legacy core-tree installs use `templates/core/` instead.
Use `dockpipe package list --format json` to see the user install root, installed
archives and warnings for your environment. Do not infer a user-store location
from the project compile directory.

For the current CLI remote-install contract, see
[Remote catalogs and user installation](#remote-catalogs-and-user-installation).

## Resolution order (source and installed packages)

Project source workflows and configured source roots can override installed
workflows of the same name. Package lookup checks the project store, configured
package stores, the user store and system stores; explicit tarball sources and
legacy/bundled fallbacks are also supported. Use `--workflow-file` to select an
exact source file when name lookup is ambiguous.

This is current install/run behavior, not a future global-install proposal.
The selected process platform also controls remote catalog compatibility, including
Flatpak runtime identity as described above.

See also **[../concepts/architecture-model.md](../concepts/architecture-model.md)** and **[../cli-reference.md](../cli-reference.md)** (`dockpipe package`, `dockpipe install`).

### Engine compatibility diagnostics

Set `min_dockpipe_version` in `package.yml` when a package uses newer engine features.
The minimum is retained in exported store catalogs and checked again against verified
archive metadata. A version mismatch reports the package, required version, running
executable version and an instruction to update Dockpipe and its desktop app. The
comparison uses semantic-version ordering, including prereleases; build metadata does
not affect ordering. Unversioned `dev` builds use normal schema validation instead.
Older archives without a declared minimum remain supported. Unsupported workflow
fields retain their schema diagnostic and add an update-or-check-field-names hint.
Already-installed older CLIs must be updated to gain these checks.


## Package-contributed host pages

An installed resolver may declare `surface: assets/ui/surface.json` in its
`package.yml`. The optional descriptor contributes a generic resource page to
launchers. Discovery is data-only; it does not execute the snapshot or actions.
Resolvers with the same surface ID share one page with a provider selector.
The page title comes from package metadata; the host has no provider-name switch.

```json
{
  "schema": 1,
  "id": "example.resources",
  "title": "Example Resources",
  "snapshot": "assets/ui/snapshot/config.yml",
  "actions": [
    {"id": "create", "title": "Create", "scope": "surface", "workflow_id": "example-create"},
    {"id": "inspect", "title": "Inspect", "scope": "resource", "workflow": "assets/ui/inspect/config.yml"}
  ]
}
```

The snapshot is a package-relative workflow file. Each action declares exactly one
contained `workflow` file or a `workflow_id` available in the public workflow
catalog. Public workflows use their existing typed launch dialog and session
lifecycle. Missing or unsupported public workflows are not offered as actions.
Descriptor and workflow references must resolve to regular files within their
owning resolver, including after symlink resolution. IDs use 1–64 lowercase ASCII
letters, digits, dots, underscores or hyphens, starting with a letter or digit.
Titles contain 1–160 characters. A descriptor is at most 64 KiB and has at most
32 uniquely named actions. Schema: `src/lib/infrastructure/schema/package-surface.schema.json`.

`dockpipe catalog list --format json` projects the descriptor under
`resolver_details[].surface`, resolving contained file references to absolute
paths. Profile values and credentials are not projected. A surface declares no
new execution semantics: its files remain ordinary Dockpipe workflows and use
installed tools and SDK contracts.

### Surface workflow events

Snapshot and contained action workflows emit UTF-8 JSON events on complete lines,
prefixed by `::dockpipe-surface::`. Other output remains ordinary operation output.
The viewer passes `DOCKPIPE_SURFACE_ID`, `DOCKPIPE_SURFACE_ACTION_ID` and
`DOCKPIPE_SURFACE_RESOURCE_ID` as per-invocation context. The resource ID is opaque
and package-owned; the action must validate it against current provider state.
Do not place credential values in IDs, event details or progress messages.

```text
::dockpipe-surface::{"schema":1,"kind":"snapshot","resources":[{"id":"instance-one","title":"Example instance","state":"running","details":{"Health":"Ready"},"actions":["inspect"]}]}
::dockpipe-surface::{"schema":1,"kind":"progress","resource_id":"instance-one","phase":"inspect","message":"Checking health","percent":50}
```

A snapshot replaces the resource collection atomically after complete validation.
The interactive v1 contract permits 256 resources, 32 string details and 32 action
IDs per resource. Resource IDs are unique nonempty strings of at most 256 characters;
titles and states are at most 160 characters; detail values and progress messages
are at most 4096 characters. Detail names are at most 160 characters. Resource IDs
cannot contain NUL or line breaks. Progress has a phase (at most 160 characters), a
message, an optional resource ID and an optional percentage from 0 to 100.

The viewer bounds each event/output line to 1 MiB and each contained operation to
2 MiB of output. It retains the last valid snapshot on malformed/failed output,
marks it stale and disables resource actions until a successful refresh. Snapshot
refreshes have a 30-second deadline, run at most once at a time, and poll every five
seconds only while the page is visible and selected. Contained management actions
have a two-minute deadline and can be cancelled. Public workflow sessions retain
their normal lifecycle, including long-running workflows; their configured inputs
and normal Stop controls remain available. Packages own their connection records,
health checks and operation results. The host page does not infer readiness from
provider-specific files or claim successful mutation from a progress message.

A snapshot resource may optionally advertise a local native display:

```json
"display": {
  "kind": "native-window",
  "process_id": 42,
  "process_start_token": "boot-id:kernel-start-ticks"
}
```

The package emits this field only after verifying the resource's live process identity.
The display contract is defined in
`src/lib/infrastructure/schema/package-native-display.schema.json`: a positive integer
PID up to 2147483647 and a nonempty token up to 256 characters without control characters.
Linux uses the public execution telemetry token (`boot-id:start-ticks`). Package
display attachment currently requires a Linux X11 host. Windows hosts have no public
kernel telemetry collector, so package display attachment explicitly reports unavailable;
existing manually selected Windows desktop embedding remains supported. No display
handle, address, credential, or executable is accepted.

The desktop shows the selected, fresh resource's display directly in its package page.
The dashboard dock's app picker and the Add widget picker also list live package displays;
the operator opens one directly in Canvas or Tiling. Opening either picker requests a
bounded one-shot package snapshot, including pages that are currently hidden. Hidden
pages do not poll in the background.

The host requires exactly one eligible local window matching the XRes server-owned PID
and kernel identity and revalidates immediately before embedding. Builds or X servers
without XRes 1.2 process identities cannot show package displays. Missing, ambiguous,
unsupported, or reused identities fail without attaching another window. One visible
view owns a display at a time: switching pages or layouts releases the native window
before the next view acquires it. Package and CLI workflows remain usable without the desktop.

Attached displays belong only to the desktop session. Their resource identifiers,
process identities, native handles and panel arrangement are excluded from saved
dashboards. Closing or releasing their panel restores the external window; it does
not close the application or stop the resource. Resource shutdown remains an explicit
package workflow action. Automatic desktop acquisition remains limited to Tiling;
package displays can be explicitly attached in either layout.

For launcher-origin display routing, a resource may include an optional `launch_id`
containing 1–128 ASCII letters, digits, dots, underscores or hyphens. When a local
launcher workflow matches exactly one installed surface's surface-scoped
`workflow_id` action, the launcher injects a fresh per-invocation
`DOCKPIPE_SURFACE_LAUNCH_ID` into a transient context and opens that surface. The
package propagates this opaque value through its own lifecycle record and emits it
in the corresponding snapshot resource. It is never a credential or a native handle.
The host selects and displays only the matching resource, with a 120-second deadline;
failure, cancellation, provider removal or workspace change cancels the pending route.
No heuristic based on newly appearing resources substitutes for this correlation.

Terminal launches do not receive a launcher correlation token and retain their normal
standalone display. Explicitly opening a package page can select its initial resource;
that selection opportunity is consumed by the first snapshot, even if it is empty.
Subsequent background snapshots never select a newly appearing terminal resource.
The operator can explicitly select a resource or open it through the dashboard picker.
The package page presents a screen-dominant native view with compact provider/resource
selectors and View/Actions menus; package metadata supplies all operation labels.

### Package-owned native display widgets

A surface may optionally declare a native widget factory:

```json
"native_display": {
  "interface": 1,
  "libraries": { "linux": "assets/ui/libexample-display.so" }
}
```

Every library is a regular contained file in the installed package. The catalog
projects canonical absolute library paths and `package_root`; authored descriptors
must not supply `package_root`. Only the immutable descriptor selects code. A
resource may then advertise:

```json
"display": {
  "kind": "package-native",
  "options": { "resource_id": "example-123", "state_ref": "public-reference" }
}
```

Options are opaque public resource/connection references, never resolved keys,
passwords, tokens or other secret contents. They cannot override the library.
The host enforces a compact UTF-8 JSON limit of 16 KiB, depth 8, 128 JSON values
including the root, nonempty keys of at most 64 characters, and strings of at most
4096 characters without NUL. Schema describes the value shapes; the host also
enforces aggregate byte, node and depth limits before calling package code.
Options and native panels remain session-only and are not saved into layouts.

The installed host SDK exports `LauncherDisplayPackage.h`, interface
`com.dockpipe.launcher.DisplayPackage/1.0`. A Qt plugin implements
`initialize(packageRoot)` and `createDisplay(options, parent)`. Its
`LauncherDisplaySurface` QWidget implements `setActive(bool)` and idempotent
`closeDisplay()`. Inactive views release pressed keys/buttons and suppress input;
closing cancels and joins workers, disconnects transport, and leaves the managed
resource running. Status/failure callbacks run on the GUI thread and cease before
close returns. The package owns rendering, transport, endpoint validation, input
mapping and any process launch behavior.

Plugin metadata must contain `display_abi: 1`, the exact host `qt_version`,
`build_abi` from `QSysInfo::buildAbi()`, and the matching CMake `compiler_id` and
`compiler_version`. The SDK's CMake configuration also enforces the host compiler
and Qt kit. The loader rejects missing or mismatched metadata before instantiation;
Qt additionally validates its binary plugin compatibility. A package declaring
only Linux support is honestly unavailable on other hosts. Native code is trusted
installed package code, not a sandbox.
On Windows, the host admits adjacent dependency DLLs with a native library scope
after validating the plugin metadata. It does not change PATH or the current
directory. The installed SDK exports the same scope helper for native consumers.

The same generic wrapper serves a package page and dashboard Canvas/Tiling. One
visible view holds a lease per package root and resource ID. An explicit new view
hands off from the previous view; background snapshots do not reacquire a handed-off
resource. Hiding or closing a panel disconnects its widget. Plugin code remains
mapped for process lifetime while widgets and their workers are destroyed safely.
Legacy `native-window` resources remain supported on their qualified platform.
When a package page first selects a resource, it prefers a resource with a display
over historical entries without one. Refreshes preserve an existing user selection.

A task SDK candidate can be installed with CMake's `LauncherSDK` component, then
`tests/package-display-consumer` can build its disposable plugin solely through
`find_package(DockPipeLauncherSDK 1.0.0 EXACT)`. This is a local candidate boundary
proof; it does not change or impersonate a published SDK release identity.

A public surface workflow action can declare a platform-scoped transient launch
environment without a host shell setup script:

```json
"launch_environment": {
  "linux": {
    "REAL_TOOL": { "inherit": "TOOL_BIN" },
    "TOOL_BIN": { "asset": "assets/tooling/bin/native-tool" }
  }
}
```

This field is allowed only on `scope: "surface"` actions with `workflow_id`.
Platforms are `linux`, `windows`, and `macos`; omitted platforms receive no bindings.
Each platform has 1 to 16 bindings with ASCII environment identifiers of at most
64 characters. A binding contains exactly one `asset` or `inherit` reference;
there are no literal values or secret contents in package metadata. Assets must
be regular contained executable files; catalog projection resolves them and the
launcher rechecks containment before invocation.

The launcher snapshots the original process environment, explicit environment-file
fallbacks, and invocation overrides before resolving any binding. Explicit context
assignments take precedence, including an explicitly empty value. Missing inherited
sources leave the target alone. Inherited values are limited to 4096 UTF-8 bytes;
resolved bindings total at most 16 KiB. The explicit environment-file fallback is
bounded to 1 MiB and follows the CLI's simple dotenv syntax.

Resolved bindings go into the CLI child's process environment for that invocation.
Conflicting and inherited-source context assignments are removed from invocation
arguments after capture. Their values never enter command previews, logs, saved
contexts or package snapshots. The stored context and parent process environment
are unchanged. Only an unambiguous installed public surface action matching the
launched workflow applies this field; ordinary terminal execution is unaffected.

For packages needing only the native display interface, the explicit leaf SDK
at `src/app/tooling/dockpipe-launcher/display-sdk` installs `DockPipeDisplaySDK`
and `DockPipe::DisplayHeaders`. It exports the same core-owned
`LauncherDisplayPackage.h` and requires only the exact Qt Widgets kit, platform
and compiler used to create that SDK candidate. It links no host implementation
and does not configure or relax the full WebEngine-dependent Launcher SDK.

Both SDKs also install `LauncherNativeLibraryScope.h`. After contained-path and
exact IID/ABI/Qt/compiler admission, Windows hosts use this scoped reference to
load the module with `LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR` and
`LOAD_LIBRARY_SEARCH_DEFAULT_DIRS`. Package-adjacent dependencies can resolve
without adding package directories to PATH, changing the working directory or
mutating process-wide DLL search directories. Other platforms use the existing
Qt loader. A view closes its surface and joins package workers, releases the Qt
loader, then releases the additional native reference. The helper's hash is
recorded separately in the leaf SDK candidate metadata; the display ABI remains
unchanged.

A package must explicitly select this smaller SDK in its build configuration;
there is no fallback from a failed full Launcher SDK lookup. The leaf's
`display-sdk-candidate.json` records the public interface hash, native compiler,
platform and Qt version as a development candidate. It is not a published SDK
release identity. Its installed external consumer and compatibility rejection
checks live under `display-sdk/tests`.

A full installed desktop qualification build can explicitly enable
`DOCKPIPE_ENABLE_QUALIFICATION=ON` (default OFF). The owning Windows desktop
packager exposes this as `-EnableQualification`; ordinary runtime packaging keeps
it disabled. `--qualify-package-display CONFIG.json` exercises the real MainWindow,
installed catalog and package snapshot, provider page, Canvas widget picker,
Tiling Apps picker, handoff and panel close/reopen. It never injects a fake catalog
or factory and never owns the external resource lifecycle.

The bounded configuration requires `schema: 1`, existing absolute `workspace`,
fresh absolute `output`, `surface_id`, `resource_id`, stable unique `profile_id`,
boolean `allow_input`, and integer `timeout_ms` from 30000 through 180000.
The profile ID uses 1–64 ASCII letters, digits, underscores or hyphens. Surface
and resource IDs use 1–128 ASCII letters, digits, dots, underscores, colons or
hyphens. A unique organization/application namespace is selected before startup
preferences or MainWindow construction. Invoke the same configuration with
`--qualification-profile-path` to print its exact profile path and exit before
preferences initialize.

The qualifier requires authoritative readiness, frame revision and input
acknowledgement properties from the package surface. It captures both native
frame pixels and the complete application window after each view has remained
ready for at least 6.5 seconds, checks one active resource lease, and records
bounded lifecycle results. Explicitly enabled input sends only a native Shift
press/release through the real widget and requires protocol acknowledgements;
it does not type text or access the clipboard. Use an owned disposable resource
for that proof. Resource creation, process identity validation, final liveness
and cleanup remain the surrounding Dockpipe workflow's responsibility.


### Package script interpreters

Package build and test hooks ending in `.py` run directly with Python 3, without
Bash or PowerShell wrapping. Dockpipe selects `python3` from PATH, then `python`
on Windows. Set `DOCKPIPE_HOST_PYTHON_BIN` to an absolute executable path to select
an installed interpreter explicitly. A missing, empty or invalid explicit
selection fails without falling back. Package authors must declare Python 3 as a
source-build prerequisite when using these hooks. This selects an interpreter;
it does not install Python or change the native host shell choice.

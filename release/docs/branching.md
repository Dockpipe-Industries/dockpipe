# Branching, CI, and releases

## Model: **`staging` → `master` = ship**

1. **Contributors** (or you) work on **feature branches** or **forks** → open **PR → `staging`**.
2. **Merge to `staging`** when the change is accepted. After green CI, an installable [staging candidate](staging.md) publishes automatically. You can continue testing before production — you can push follow-ups, edit **`release/releasenotes/`**, bump **`VERSION`**, or tweak the contributor’s work on **`staging`** (via PR or maintainer commits, per your rules).
3. When you’re ready to **cut a release**, open **PR `staging` → `master`** (or merge with the same protections). That PR must **update `release/releasenotes/X.Y.Z.md`** for the selected release line; patch numbers are generated (CI enforces this only for PRs **targeting `master`**).
4. **Merge to `master`** → **Release** workflow runs: artifacts + **GitHub Release** **`vX.Y.Z`**.

**`master`** is **the released line**. **`staging`** holds “next release” integration until you ship.

The current maintainer promotion path is **`js/pipelang` → `js/dev` → `dev` → `staging` → `master`**. Intermediate branches are for integration and qualification; production release publication runs only from `master`.

There is **no separate “push a tag to release”** step for normal flow — the tag is created as part of the GitHub Release. (You can still use **Release → Run workflow** manually for dry-runs.)

### Vetting: PRs only (no direct pushes to `staging` / `master`)

**Use pull requests** instead of pushing straight to protected branches. Typical setup:

- **Outside contributors:** **fork** → **PR → `staging`**.
- **Maintainers:** **feature branch** → **PR → `staging`** (same as contributors), or whatever your org allows on **`staging`**.
- **Release:** **PR `staging` → `master`** when **`VERSION`** + release notes are final — merge triggers **Release**.

Turn off **“allow administrators to bypass”** if you want **your own** changes to use the same PR path.

### Default branch = latest release (no drift)

**`master`** should match **what you last shipped** (and the generated release tag on that branch). **`staging`** may be **ahead** until the next ship PR.

**First-time GitHub Actions:** Manual dispatch requires the workflow on the default branch. The staging push path calls the reusable release workflow from the same commit and can qualify candidates before a production merge.

### Recommended flow

1. **On a feature branch:** **Actions → CI → Run workflow** → pick your branch (full tests; **no** VERSION gate).
2. **PR → `staging`:** CI runs **without** the release-notes / VERSION bump requirement.
3. **On `staging`:** You adjust code, **release notes**, and **`VERSION`** when you’re ready to ship.
4. **PR `staging` → `master`:** CI runs **with** the release-notes gate → merge **ships**.
5. **Release dry run:** **Actions → Release → Run workflow** → branch + **dry_run: true**.

### Dependabot

**`.github/dependabot.yml`** runs **weekly** version updates for **`gomod`**
(repo root), **`github-actions`** (grouped), and **`cargo`** (Pipeon desktop,
grouped). Version-update PRs target **`dev`** and follow **`dev` → `staging` →
`master`**. CI and CodeQL cover both PRs and pushes to `dev`; pushes to `dev`
do not publish a staging candidate or a production release.

Dependabot reads its configuration from the repository's default branch, so
configuration changes become active after promotion to `master`. Fork owners
using this routing need a `dev` branch.

**Security updates always target the default branch.** GitHub does not apply
`target-branch` to them. Retarget those PRs to `dev` and update their branches
with the current `dev` before qualification; do not bypass required checks.
See [GitHub's target-branch rules](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference#target-branch).

Pipeon desktop changes also run **Pipeon desktop qualification**, which builds
the locked Rust dependency graph on Linux, macOS, and Windows. The Flatpak
qualification alone does not cover this optional Tauri desktop shell.

---

## `VERSION` + `release/releasenotes/X.Y.Z.md`

- **[`VERSION`](../../VERSION)** — single line, semver **`X.Y.Z`** (no `v` prefix). This selects the major/minor release line and minimum patch. Publication generates the next unused numeric patch across stable and staging tags.
- **`release/releasenotes/${VERSION}.md`** — required body for the GitHub release.

**Every PR into `master` must** (enforced by CI):

1. Keep a valid numeric **`VERSION`** baseline; a manual patch bump is not required.
2. **Modify** **`release/releasenotes/<baseline-version>.md`** in that same PR.

**PRs into `staging`** do **not** run that gate — integrate freely, then finalize notes + version on **`staging`** before the ship PR.

**Docs-only or chore ship PRs** receive a generated patch and still need a short release note (e.g. “Docs: …”).

---

## One PR → one CI workflow run (three jobs)

**`.github/workflows/ci.yml`** is a single workflow named **CI**. Each trigger creates **one** run in the Actions list. Inside it, three **jobs** run in parallel:

- **`test`** (Ubuntu) — **`govulncheck`**, **`gosec`**, **`go test`**, **`make`**, **`.deb`**, shell + integration tests, and (on PRs to **`master`**) the release-notes gate.
- **`test-windows`** (Windows) — **`go test ./...`** and **`test_clone_worktree_include.sh`** (bash + git; host pre-script–like coverage). Does **not** run Docker integration tests or full **`tests/run_tests.sh`** (those stay on Linux).
- **`codeql`** — **CodeQL** (Go, **`security-extended`** via **`.github/codeql/codeql-config.yml`**), uploads to **Security → Code scanning** when allowed.

On the **weekly schedule**, only **`codeql`** runs (**`test`** and **`test-windows`** are skipped).

---

## What runs when

| Event | Workflow | What it does |
|--------|-----------|----------------|
| **PR** → **`staging`** | **`ci.yml`** | Jobs **`test`** + **`test-windows`** + **`codeql`** — **no** release-notes gate on **`test`** |
| **PR** → **`master`** | **`ci.yml`** | Same + **release notes** on **`test`** |
| **Push** **`staging`** / **`master`** | **`ci.yml`** | **`test`** + **`test-windows`** (no VERSION gate on push); staging then builds and publishes a candidate. CodeQL runs separately. |
| **workflow_dispatch** | **`ci.yml`** | **`test`** + **`test-windows`** + **`codeql`** (no VERSION gate) |
| **Schedule** (weekly) | **`ci.yml`** | **`codeql`** only |
| **Push** **`master`** (merge) | **`release.yml`** | Full build + **GitHub Release** with generated `vX.Y.Z`; optional **dev.to** ([devto.md](devto.md)) |
| **workflow_dispatch** on Release | **`release.yml`** | Defaults to **dry_run=true**, without deployment approval; **dry_run=false** requires **`master`** |

> **Release** still runs only on **`push` to `master`**, not on pushes to **`staging`**.

---

## Dry-run a release (no GitHub Release)

**Actions → Release → Run workflow** → optional **version**, set **dry_run** to **true** → download the **workflow artifact** (MSI, debs, checksums, etc.).

---

## Local checks before a **ship** PR (`staging` → `master`)

```bash
go test ./…
# Ship PR must match VERSION file:
test -f "release/releasenotes/$(tr -d ' \t\r\n' < VERSION).md"
```

**Makefile** `deb` / `deb-all` use **`VERSION`** automatically when the file exists.

## Running a fork

CI and manual release dry runs work without an organization allowlist. Automatic
publication is off until the fork owner sets repository variable
`DOCKPIPE_RELEASE_ENABLED=true`. First configure separate `release` and
`release-staging` GitHub environments with your own scoped credentials, signing
identity, endpoint, bucket, and prefix. Preserve the `staging` → `master` release
flow and apply branch protections appropriate for your repository.

The [release setup workflow](../../workflows/package/package-release-setup/README.md)
requires `RELEASE_SETUP_REPOSITORY=your-owner/your-repository`; it never infers a
credential destination from the upstream repository. Run checks before enabling
apply. For CLI installation from a fork, use `DOCKPIPE_REPO=your-owner/your-repository`
with the shell installer or `-Repo your-owner/your-repository` with PowerShell.
Official Dockpipe download defaults remain available for ordinary users.

Do not enable `DEVTO_PUBLISH` in a fork unless you deliberately configure your own
article destination and credentials. Follow the [public configuration boundary](../../docs/public-repository-boundary.md)
for operator settings and examples.

# Public documentation audience audit — 2026-10-10

The user requested an audit and correction of user-facing documentation, especially
pages selected for the website. Work stays in the saved Dockpipe checkout and
preserves the earlier Flatpak documentation changes. No commit, push, release,
Cloud source edit or site refresh is included.

## Selection and findings

The inspected DockPipe Cloud `scripts/site/stage-docs.py` selects these 12 source
pages. Its selection is the authority for this pass; it does not publish every
Markdown file in the Dockpipe repository.

| Selected page | Finding and correction |
| --- | --- |
| `docs/install.md` | Mixed user downloads, hypothetical production setup, source builds and release tooling. Lead with available staging installation, platform choices and verified APT setup; retain Flatpak instructions and explicit qualification limits. |
| `docs/onboarding.md` | Sent new users into Dockpipe's CI and docs workflows. Replace with a project-local hello workflow, then an explicit container example and package discovery. |
| `docs/workflows/workflow-authoring.md` | Examples assumed undeclared images, scripts and installed resolvers. Supply an explicit basic image, state project/tool prerequisites, and use runnable host output steps. |
| `docs/workflows/workflow-yaml.md` | Front-loaded internal lookup paths and ended with repo-specific workflow commands. Use project/package lookup guidance and link to complete user examples; neutralize a private-project workspace example. |
| `docs/workflows/agentic-workflows.md` | Mixed current usage, repository dogfood and product proposals. Explain installed workflow prerequisites, harness behavior and enforcement limits; preserve internal material in `agentic-design-notes.md`. |
| `docs/cli-reference.md` | Mixed user installation with mirror construction and source-tree assumptions. Clarify prerequisites and package stores; move publishing detail to `packages/package-publishing.md`. |
| `docs/concepts/architecture-model.md` | Normative engine/source-layout document served as the user explanation. Replace the learning page with practical composition; preserve the original contract in `architecture-contract.md` and route maintainer guidance there. |
| `docs/concepts/isolation-layer.md` | Instructed users to modify engine source and described extension proposals. Explain container/host selection, installed resolvers and Flatpak boundaries instead. |
| `docs/packages/package-quickstart.md` | Led with contributor builds and called package installation future work. Lead with Marketplace/CLI discovery, install, dependencies, update/removal and optional authoring. |
| `docs/packages/package-model.md` | Mixed current remote installs with future global-store proposals and internal path-helper rules. Clarify user/system/project storage and current lookup; retain package-author reference details. |
| `docs/security/security-policy.md` | Diagnostic example implied an outbound request was blocked despite advisory coverage. Use actual implementation messages and explain their limit. |
| `docs/runtime/image-artifacts.md` | Ended with implementation direction rather than user limits. Add user troubleshooting context and describe present scope. |

README and the documentation index now lead to the same installed-user path.
The docs-system guidance records the audience and validation rules for later edits.
Advanced package authoring remains user documentation; maintaining Dockpipe's own
engine, CI or release process is linked separately.

## Verification and limits

- Existing CLI validated 12 complete YAML examples from the rewritten guides.
- In a fresh project outside the checkout, `init hello`, hello execution, step
  outputs, single-workflow compilation and JSON inventory passed with isolated
  user cache/data/state directories. No Dockpipe source-root override was used.
- The actual Cloud staging script generated a 12-page working-tree bundle under
  `/tmp`; selected-page link rewriting completed successfully.
- Relative links/anchors and shell-block syntax were checked across selected pages
  and newly linked contributor references. The repository path guard and whitespace
  checks were run after edits.
- The staging APT key was fetched and its fingerprint matched the documented value.
- Container examples were schema-validated, not executed. No native Windows/macOS
  installation, provider authentication, Flatpak update or full reference-command
  acceptance suite was run in this documentation pass.

This is source documentation proof. The live site remains on its existing snapshot.
New contributor-reference links need the matching source revision when these changes
are committed and the site is refreshed; a working-tree preview is not publication.


## 0.6.x release notes and review template — 2026-10-10

Rewrote [the release-line notes](../../../../release/releasenotes/0.6.0.md) around
launcher use, optional Marketplace packages, workflows, named secret environments,
remote delivery, AI workflows and durable package data. The repo-root `VERSION`
remains 0.6.0; the existing release workflow substitutes the generated patch.
Removed checkout-only test workflows and architecture/CI implementation highlights.

Claims were checked against the launcher navigation/package UI, CLI flags and
installer scripts, release artifact builders/workflow, secret-provider contract,
remote-node documentation and TASK-036 qualification record. Remote delivery is
labelled preview; native Mac acceptance, desktop signing, staging-only channels,
Flatpak platform limits and experimental PipeLang scope are explicit. Corrected
the remote guide's stale claim that no launcher controls exist.

Added the GitHub [default PR template](../../../../.github/pull_request_template.md)
for the repository's MR workflow. It asks for behavior, verification and user impact,
with a release-only checklist matching the existing master-targeted CI gate.
The reusable release-note template and releasing/contributing instructions now
follow the same end-user focus and generated-version policy.

Validation: 40 source/local links, 18 shell blocks, two PowerShell blocks and
whitespace checks passed. The release body's exact version-substitution expression
was exercised with sample version 0.6.17. The documented hello workflow was created,
validated and run in a fresh project with isolated user directories. Evidence:
`/tmp/dockpipe-release-docs-5lqnplk5/validation.json` and its adjacent logs/rendered body.
Install commands were syntax/source checked, not executed; no new native install,
provider acceptance, signing or publication proof is claimed. No release version,
workflow, implementation or Git publication changed.

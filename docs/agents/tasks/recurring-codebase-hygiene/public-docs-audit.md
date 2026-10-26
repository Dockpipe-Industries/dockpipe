# Human documentation audit — 2026-10-25

Status: completed documentation slice; all changes remain unstaged/uncommitted.

Corrected installation and onboarding, native shell assumptions, Flatpak setup,
retired core-package guidance, source layout, runtime/workflow contracts and GUI
terminology. Package-specific DorkPipe, secrets-provider and VM guidance moved to
its owners, with entrypoint pointers retained. Historical VM JSON evidence moved
byte-for-byte. Existing PipeLang owner pointers were already correct.

Verification: 461 file/owner links and 26 local anchors resolved across 108 files;
YAML/JSON and shell/PowerShell syntax checks passed. Isolated Linux onboarding and
output-forwarding examples passed. Both Cloud channels produced their 12-page
documentation bundle; no site publication occurred. External URLs were checked
against saved owner files, not remote publication. Full native builds, installs,
VMs, providers and telemetry were not qualified by this pass.

The [October 10 audit](../closed/recurring-codebase-hygiene/public-docs-audit-2026-10-10.md)
is historical. Current public-page selection lives in Dockpipe Cloud's
`src/frontend/console/content/public-documents.json`; the workflow-owned
`stage-docs.py` renders that catalog.

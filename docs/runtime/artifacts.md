# Maintainer Artifacts

Generated files used as **context** for humans and AI. Refresh the Dockpipe CI scans before relying on them; DorkPipe self-analysis is maintained in its own repository.

## Governance and security questions

When asked *“compliance issues?”* or *“security posture?”*:

1. Read **`AGENTS.md`**.
2. Inspect current raw govulncheck and gosec receipts from `dockpipe scope artifacts ci-raw --workdir .`. If DorkPipe supplied a separately qualified analysis, identify its source and version. Do **not** invent scan output.
3. If artifacts are missing or stale vs `HEAD`, say so — do **not** claim “clean” without evidence.
4. This is **not** a certified compliance verdict (SOC2, ISO, etc.).

The `compliance-handoff` workflow is owned and qualified by the separately published DorkPipe package.

## CI scan bundle

Dockpipe CI keeps the original govulncheck and gosec JSON receipts under its `ci-raw` workflow artifact directory. It runs the tools and enforces their exit status. DorkPipe owns any separately published normalization or analysis of those receipts. **`make ci`** reproduces the GitHub Actions test job locally through **`act`**.

| Path | Role |
|------|------|
| `dockpipe scope artifacts ci-raw --workdir .` | Original tool JSON directory |

## User insight queue

Structured human guidance is owned by the DorkPipe insight CLI and its durable learning-authority helper; do not reconstruct its path with a generic public package-scope suffix. Schemas live under **`src/schemas/dockpipe-user-insight-*.schema.json`**.

**Canonical implementation:** **`dorkpipe insight ...`** in the maintainer **`dorkpipe`** package; the workflow **`user-insight-process`** is a host-side entrypoint that uses the same CLI surface. See **`resolvers/user-insight-process/README.md`** there.

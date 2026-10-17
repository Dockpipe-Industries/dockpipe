# Repo-local workflows

This directory is the **canonical** place for **this repository’s** Dockpipe workflows: CI pipelines, Codex demos, R2 publish, self-analysis stacks, sandbox experiments, and host-only helpers.

See [macOS workflow support](../docs/workflows/macos.md) for prerequisites,
Docker Desktop networking, and workflows that still have platform restrictions.

Workflows are grouped by purpose while retaining their leaf workflow names for `--workflow`:

- `agent/` — Codex demos and DorkPipe agentic docs-orchestration dogfood.
- `ci/` — local CI mirrors and quality summaries.
- `package/` — package/store/infrastructure composers.

| Workflow | Role |
|----------|------|
| **`agent/codex-pav`** | Optional Codex plan/apply/validate resolver demo (`OPENAI_API_KEY`; CI gated by `DOCKPIPE_CI_CODEX`). |
| **`agent/codex-security`** | Optional Codex security-review resolver demo (`OPENAI_API_KEY`; CI gated by `DOCKPIPE_CI_CODEX`). |
| **`ci/test`** | Multi-step Docker chain: go test → vet → govulncheck → gosec → security brief (mirrors the spirit of `.github/workflows/ci.yml`’s Dockpipe workflow step). |
| **`ci/ci-emulate`** | Local CI emulator. Runs the Linux **`.github/workflows/ci.yml`** **`test`** job through **`act`** and validates the Windows CI subset either on the native Windows host or through **`windows-vm`** on Linux. |
| **`package/package-store-infra`** | Thin composer: shared **`vars`** + nested packaged workflow **`dockpipe.cloudflare.r2infra`** (`workflow:` + `package:`); optional **`--tf`**. Store tarballs: run **`dockpipe package build store`** separately when needed. |
| **`package/package-store-publish`** | Maintainer mirror workflow: `vault: op` → build release artifacts → upload each file to R2/S3 with `dockpipe release upload`. Dry-run by default. |

**Suggested local “full stack”:** run **`make ci`** or **`./src/bin/dockpipe --workflow ci-emulate --workdir . --`** to execute the Linux job locally through **`act`** and the Windows subset through the native host or **`windows-vm`**, depending on platform. On Linux, assume the configured Windows VM image is already provisioned for SSH. CI uploads raw govulncheck and gosec scan receipts. For Cloudflare R2 infra with repo **`vars`**: **`./src/bin/dockpipe --workflow package-store-infra --workdir . --`**. For package mirror artifacts + R2 upload: **`./src/bin/dockpipe --workflow package-store-publish --workdir . --`**.

## Current Status

In progress. Launch now skips cached Ollama model pulls and hides non-interactive Windows PowerShell
startup calls. Prebuilt image generation is documented here as the larger release-engineering item.

The first vertical slice for shared DorkPipe provider pools landed and was subsequently extended by
the stream-worker and workflow-leasing passes recorded below:

- DorkPipe now owns a package-level provider-pool catalog/config for `ollama`, `claude`, and `codex`.
- DorkPipe exposes shared provider-pool catalog/status/chat operations through its CLI and host MCP
  bridge so Pipeon and non-Pipeon surfaces use the same contract.
- A package-owned DorkPipe `orchestrator` workflow now routes direct prompts through the provider pool
  contract and supports inline provider/model overrides such as `--provider claude` and
  `--model llama3.2`.
- Direct CLI orchestration now returns explicit `warming`, `auth-required`, `queued`, `disabled`, or
  `failed` states instead of silently cold-starting a provider lane.
- Pipeon provider selection and direct chat now consume the shared DorkPipe provider-pool catalog and
  MCP chat tool rather than owning a separate routing model.
- Pipeon dev-stack startup now explicitly calls the shared DorkPipe provider-pool warm lifecycle and
  writes a provider-pool status snapshot into stack state, so warm-up is deliberate and observable
  instead of being triggered implicitly by a read-only status/catalog call.

Current lane status on a normal core-dev machine is intentionally honest:

- Codex direct chat preserves session bindings and can use the host `codex exec` resume lane.
- Ollama reports `warming` until the local service is reachable instead of triggering an implicit cold
  path.
- Claude guarded warm-worker health is now materially better:
  - explicit `op inject` is no longer forced just because project config selects 1Password; workflows
    without referenced secret-template keys now skip vault injection cleanly
  - direct prompts wait briefly for a warming provider instead of immediately failing back to “retry”
  - the guarded worker now reuses the same pool identity across direct CLI, workflow host steps, and
    future app surfaces because relative and absolute workdirs canonicalize to the same repo-root key
  - the guarded worker bootstrap now avoids copying heavyweight host Claude state such as
    `file-history`, while still copying the smaller auth/session files needed for a viable warm lane
  - the keepalive no longer depends on `runuser`, which was missing from the current `dockpipe-claude`
    image and caused worker exits
- Claude remains the active tuning area rather than a correctness blocker:
  - shared-pool direct prompt latency is currently about 31 seconds on the measured Windows core-dev
    machine once the worker is warm
  - the full `dockpipe --package dorkpipe --workflow orchestrator` path is currently about 41 seconds
    against that warm worker
  - the remaining gap appears to be inside the guarded Claude container lane itself, not provider-pool
    identity drift, hidden cold starts, or workflow dispatch overhead

Latest Claude provider-pool tuning pass:

- The warm guarded worker was already reusing the expected container
  `dorkpipe-provider-pool-claude-4ca0fbabc6` and image `dockpipe-claude:latest`; direct Docker
  inspection showed the worker was up before prompt measurements.
- Cheap container probes were not the bottleneck on the Windows core-dev machine:
  - `docker exec ... true`: about 0.6 seconds
  - `docker exec ... node -e`: about 0.6 seconds
  - `docker exec ... claude --version`: about 1.0 seconds
- The prompt path was still using `docker exec -i` even though Claude is invoked with `-p` and does
  not need stdin. Removing `-i` avoids keeping an interactive stdin pipe open for every pooled
  prompt. Raw warm-worker probes went from about 37.6 seconds with `-i` to about 21.6-25.2 seconds
  without `-i` for the same "latency probe" prompt.
- Provider-pool prompt JSON now emits coarse timing metadata so CLI, workflow, and Pipeon callers can
  see `auth_check_ms`, `image_check_ms`, `image_recheck_ms`, `container_running_check_ms`,
  `worker_start_ms` when a worker is started, `status_ms`, `readiness_wait_ms`, `queue_wait_ms`,
  `provider_prompt_ms`, `claude_command_ms`, and `total_ms`.
- Representative post-change measurements:
  - direct `dorkpipe provider-pool prompt --workdir . --provider claude --json`: 23.7 seconds total
    in the best warmed sample, with `claude_command_ms=22037`, `status_ms=1180`, and
    `queue_wait_ms=3`
  - direct final sample after adding readiness timing: 44.0 seconds total, with
    `claude_command_ms=42272`, `status_ms=1245`, and `queue_wait_ms=2`
  - CLI orchestrator workflow warmed sample:
    `dockpipe --package dorkpipe --workflow orchestrator -- --provider claude --json`: 27.0 seconds
    end-to-end, with `claude_command_ms=23235`, `provider_prompt_ms=23362`, `status_ms=505`, and
    `queue_wait_ms=1`
- Current conclusion: the avoidable Docker stdin overhead is reduced, and the remaining variance is
  overwhelmingly inside `claude --dangerously-skip-permissions --model sonnet -p ...` within the
  guarded container. Pool identity, queueing, readiness, image checks, and workflow dispatch are no
  longer the dominant warm-path cost. Pipeon should benefit automatically because it calls the same
  shared DorkPipe provider-pool contract and receives the same timing metadata through the MCP bridge.

Follow-up stream-worker experiment:

- `claude --bare` is not viable for the current guarded lane because it rejects the copied
  subscription/OAuth-style host auth and reports `Not logged in`; the help text says bare mode uses
  API-key/helper auth only.
- `claude --safe-mode` preserved current auth but did not materially improve prompt latency; a simple
  latency probe took about 31 seconds.
- Claude's machine stream mode is viable for a real warm worker:
  `claude --dangerously-skip-permissions --model sonnet -p --input-format stream-json
  --output-format stream-json --include-partial-messages --replay-user-messages --verbose` accepted
  multiple JSONL user messages on one process.
- In the two-turn proof, the first turn paid initialization and returned `first-turn` with
  `time_to_request_ms=22293`, `ttft_ms=28550`, and `duration_ms=28571`; the second turn in the same
  Claude process returned `second-turn` with `time_to_request_ms=44`, `ttft_ms=2657`, and
  `duration_ms=2731`.
- Experiment conclusion at the time: keep the existing DorkPipe provider-pool/MCP contract as the
  public surface, but replace the sleeping guarded Claude container plus per-prompt
  `docker exec claude -p` with a session-affine in-container Claude stream process managed by
  DorkPipe. MCP can keep the provider session addressable and route prompts, but the latency win
  comes from keeping the Claude stream process alive behind that contract.

Stream-worker implementation pass:

- The generic provider-pool public path remains unchanged: CLI workflows, MCP, and Pipeon still call
  `dorkpipe provider-pool prompt --json` through `dorkpipe.provider_pool_chat`. No Claude-specific
  public MCP tool or DockPipe core logic was added.
- Claude direct prompts now default to a session/model-affine stream worker inside the guarded
  container. The worker is addressed by generic provider-pool fields (`provider`, `session_id`,
  `worker_id`, `worker_mode`, `prompt_turn_id`) and launches:
  `claude --dangerously-skip-permissions --model <model> -p --input-format stream-json
  --output-format stream-json --include-partial-messages --replay-user-messages --verbose`.
- The stream worker is managed by DorkPipe inside the existing warm container using a
  container-local Unix socket. Each prompt is sent as one JSONL user turn, and DorkPipe reads stream
  events until Claude emits `type=result`.
- The previous one-shot `docker exec claude -p` path remains as the explicit fallback. Set
  `DORKPIPE_PROVIDER_POOL_CLAUDE_STREAM_WORKER=single_prompt` to force it, or
  `DORKPIPE_PROVIDER_POOL_CLAUDE_SINGLE_PROMPT_FALLBACK=1` to fall back after a stream-worker error.
- Prompt JSON now includes the requested stream timing fields where available:
  `queue_wait_ms`, `status_ms`, `worker_start_ms` when a worker container starts,
  `stream_start_ms`, `stream_ready_ms`, `time_to_request_ms`, `time_to_first_token_ms`,
  `provider_turn_ms`, and `total_ms`. It also includes `provider_session_id`,
  `provider_request_id`, `prompt_turn_id`, `prompt_count`, `stream_reused`, and
  `stream_restart_reason`.
- Direct validation on the Windows core-dev machine:
  - first streamed direct prompt after daemon restart returned `stream-smoke` with
    `stream_reused=false`, `stream_start_ms=3355`, `stream_ready_ms=484`,
    `time_to_request_ms=1`, `time_to_first_token_ms=32043`, `provider_turn_ms=32283`, and
    `total_ms=36856`
  - second direct prompt on the same provider-pool session returned `stream-smoke-2` with
    `stream_reused=true`, `time_to_request_ms=1`, `time_to_first_token_ms=3266`,
    `provider_turn_ms=3302`, and `total_ms=4784`
  - `dockpipe --package dorkpipe --workflow orchestrator -- --provider claude --json` reused the
    same stream worker and returned `orchestrator-stream-smoke` with `stream_reused=true`,
    `provider_turn_ms=2885`, `provider_prompt_ms=3516`, and provider-pool `total_ms=4082`
- Pipeon should pick up the fast path automatically: the extension call sites still read
  `dorkpipe.provider_pool_catalog` and send direct chat through `dorkpipe.provider_pool_chat`, which
  invokes the same `provider-pool prompt --json` implementation.

### Provider-pool lifecycle hardening pass on 2026-07-09

- Codex provider-pool scratch files now stay under project package state at
  `bin/.dockpipe/packages/dorkpipe/provider-pools/scratch` instead of system temp. This keeps generated
  provider-pool handoff material in DockPipe-owned project state and avoids `.tmp` drift.
- Workdir hash fallback now accounts for Windows-style path normalization and candidate hashes. This
  preserves cleanup when a worker was started through one host path spelling and stopped through
  another, such as Git Bash/MSYS slash conversion or case differences.
- Pipeon code-server on Windows now seeds container Git config with `core.autocrlf=true`,
  `core.filemode=false`, and `core.safecrlf=false`. The finding was that the `/work` bind mount could
  otherwise make Git report broad file modifications because Linux container defaults did not match the
  Windows checkout's CRLF/filemode behavior.
- Claude provider-pool stop now removes all matching managed workers for the workdir, including
  name/hash candidates and Docker label/hash fallback. Stop reporting now lists all removed worker ids
  and reports the count instead of assuming one container name.
- Pipeon `launch.sh` autodown and `stop.sh` both call `dorkpipe provider-pool stop --workdir <repo>`
  before stack/container teardown, so provider-pool workers are tied to Pipeon lifecycle unless a future
  explicit detached mode is added.
- Manual lifecycle validation on Windows/Docker Desktop 29.6.1:
  - `dorkpipe provider-pool warm --workdir . --provider claude --json` started
    `dorkpipe-provider-pool-claude-4ca0fbabc6`; reported timings included `worker_start_ms=1473`,
    `container_running_check_ms=258`, `image_check_ms=203`, and `image_recheck_ms=200`.
  - Docker inspection showed one `com.dockpipe.provider-pool=true` container running.
  - Git Bash `stop.sh` completed in about 18 seconds and reported stack removal for `/c/Source/dockpipe`.
  - A final Docker label query returned no `com.dockpipe.provider-pool=true` containers.
- Focused validation from this pass:
  - `go test ./packages/dorkpipe/lib/statepaths ./packages/dorkpipe/lib/cmd/dorkpipe` passed with a
    repo-local Go cache under `bin/.dockpipe` after the default user Go cache was denied by the sandbox.
    Package test times reported by Go were `statepaths=0.104s` and `cmd/dorkpipe=0.324s` on the passing
    run.
  - Git Bash syntax checks passed for touched Pipeon scripts:
    `launch.sh`, `desktop.sh`, and `stop.sh`.

Boundary check:

- DockPipe core under `src/lib` and `src/cmd` was not changed.
- Provider-pool lifecycle logic remains in the DorkPipe package, and Pipeon behavior remains package-local
  to the `pipeon-dev-stack` scripts.
- Direct Pipeon chat isolation from workflow/subagent sessions is preserved: Pipeon still calls the shared
  provider-pool/MCP direct-chat path, while workflow provider-pool leasing remains explicit through
  workflow model policy.

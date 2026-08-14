package mcpbridge

import "encoding/json"

// mcpToolMeta is one row for tools/list (filtered by ToolAllowed(ctx, name)).
type mcpToolMeta struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

func mcpToolCatalog() []mcpToolMeta {
	return []mcpToolMeta{
		{
			Name:        "dockpipe.version",
			Description: "Run dockpipe --version. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "capabilities.workflows",
			Description: "List workflow names for the current project or bundled cache. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "repo.list_files",
			Description: "List repo-relative files, optionally filtered by a path substring. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`),
		},
		{
			Name:        "repo.read_file",
			Description: "Read a UTF-8 text file under repo root. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"max_chars":{"type":"integer","minimum":1,"maximum":20000}},"required":["path"],"additionalProperties":false}`),
		},
		{
			Name:        "repo.search_text",
			Description: "Search UTF-8 text files under repo root and return matching lines. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100}},"required":["query"],"additionalProperties":false}`),
		},
		{
			Name:        "dockpipe.validate_workflow",
			Description: "Validate workflow YAML (dockpipe workflow validate). Tier: validate+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"optional path to config.yml, repo-relative (e.g. workflows/ci/test/config.yml); omit only for a flat single-workflow project"}},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.validate_spec",
			Description: "Validate a DorkPipe DAG spec (dorkpipe validate -f). Tier: validate+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"spec_path":{"type":"string"}},"required":["spec_path"],"additionalProperties":false}`),
		},
		{
			Name:        "dockpipe.run",
			Description: "Run dockpipe with --workflow, optional --package, --workdir, argv after --. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workflow":{"type":"string"},"package":{"type":"string","description":"optional package name for package-owned workflows"},"workdir":{"type":"string"},"argv":{"type":"array","items":{"type":"string"}},"result_mode":{"type":"string","enum":["summary","stdout"],"description":"summary (default) wraps CLI stdout/stderr; stdout returns the package event stream unchanged"}},"required":["workflow"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.run_spec",
			Description: "Run dorkpipe run -f <spec>. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"spec_path":{"type":"string"},"workdir":{"type":"string"}},"required":["spec_path"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.request",
			Description: "Run dorkpipe request --execute through the MCP control plane. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"message":{"type":"string"},"mode":{"type":"string"},"session_id":{"type":"string"},"provider_preset":{"type":"string"},"model_provider":{"type":"string"},"model":{"type":"string"},"active_file":{"type":"string"},"open_files":{"type":"array","items":{"type":"string"}},"selection_text":{"type":"string"},"attachment_files":{"type":"array","items":{"type":"string"}}},"required":["message"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_catalog",
			Description: "Read the shared DorkPipe provider-pool catalog plus current provider states. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"}},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_status",
			Description: "Read current DorkPipe provider-pool status, optionally filtered to one provider. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"provider":{"type":"string","enum":["ollama","codex","claude"]}},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_chat",
			Description: "Route a direct prompt through the shared DorkPipe provider-pool contract. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"message":{"type":"string"},"provider":{"type":"string","enum":["ollama","codex","claude"]},"model":{"type":"string"},"session_id":{"type":"string"},"session_adapter":{"type":"string","enum":["codex_exec","codex_app_server"]},"active_file":{"type":"string"},"open_files":{"type":"array","items":{"type":"string"}},"selection_text":{"type":"string"}},"required":["message"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_approval_request",
			Description: "Read the exact neutral approval request currently pending for this MCP server's one active provider-pool chat. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_approval_decide",
			Description: "Deliver one exact approve or deny decision to this MCP server's pending provider-pool approval request. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"correlation":{"type":"object","properties":{"process_incarnation_id":{"type":"string","minLength":1},"connection_id":{"type":"string","minLength":1},"session_id":{"type":"string","minLength":1},"interaction_id":{"type":"string","minLength":1},"activity_id":{"type":"string","minLength":1},"request_id":{"type":"string","minLength":1},"decision_id":{"type":"string","minLength":1}},"required":["process_incarnation_id","connection_id","session_id","interaction_id","activity_id","request_id","decision_id"],"additionalProperties":false},"decision":{"type":"string","enum":["approve","deny"]}},"required":["correlation","decision"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_user_input_request",
			Description: "Read a defensive copy of the exact neutral user-input prompt currently pending for this MCP server's one active provider-pool chat. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_user_input_respond",
			Description: "Deliver one complete exact neutral response to this MCP server's pending provider-pool user-input prompt. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"correlation":{"type":"object","properties":{"process_incarnation_id":{"type":"string","minLength":1},"connection_id":{"type":"string","minLength":1},"session_id":{"type":"string","minLength":1},"interaction_id":{"type":"string","minLength":1},"activity_id":{"type":"string","minLength":1},"request_id":{"type":"string","minLength":1},"decision_id":{"type":"string","minLength":1}},"required":["process_incarnation_id","connection_id","session_id","interaction_id","activity_id","request_id","decision_id"],"additionalProperties":false},"prompt_ref":{"type":"string","minLength":1,"maxLength":128,"pattern":"^[A-Za-z0-9_.:-]+$"},"selected_option_refs":{"type":"array","maxItems":16,"uniqueItems":true,"items":{"type":"string","minLength":1,"maxLength":128,"pattern":"^[A-Za-z0-9_.:-]+$"}},"text":{"type":"string","maxLength":4096}},"required":["correlation","prompt_ref"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_cancellation_request",
			Description: "Read a defensive copy of the exact neutral cancellation scope currently pending for this MCP server's one active provider-pool chat. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_cancellation_deliver",
			Description: "Deliver one exact neutral cancellation intent to this MCP server's pending provider-pool cancellation scope. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"session":{"type":"object","properties":{"provider":{"type":"string","enum":["codex"]},"session_id":{"type":"string","minLength":1}},"required":["provider","session_id"],"additionalProperties":false},"correlation":{"type":"object","properties":{"process_incarnation_id":{"type":"string","minLength":1},"connection_id":{"type":"string","minLength":1},"session_id":{"type":"string","minLength":1},"interaction_id":{"type":"string","minLength":1},"activity_id":{"type":"string","maxLength":0},"request_id":{"type":"string","maxLength":0},"decision_id":{"type":"string","maxLength":0}},"required":["process_incarnation_id","connection_id","session_id","interaction_id","activity_id","request_id","decision_id"],"additionalProperties":false},"reason":{"type":"string","enum":["user_requested","safety_stop","deadline_exceeded"]}},"required":["session","correlation","reason"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.host_codex_chat",
			Description: "Host bridge for direct Codex chat. Runs codex exec with workspace sandboxing and the host Codex model config by default. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"message":{"type":"string"},"model":{"type":"string"},"session_id":{"type":"string"},"active_file":{"type":"string"},"open_files":{"type":"array","items":{"type":"string"}},"selection_text":{"type":"string"}},"required":["message"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.host_claude_chat",
			Description: "Host bridge for guarded Claude chat. Routes through DockPipe's Claude workflow boundary instead of raw host Claude. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"message":{"type":"string"},"model":{"type":"string"},"session_id":{"type":"string"},"active_file":{"type":"string"},"open_files":{"type":"array","items":{"type":"string"}},"selection_text":{"type":"string"}},"required":["message"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.host_claude_auth",
			Description: "Backward-compatible alias for dorkpipe.provider_auth_repair with provider=claude. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"}},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_auth_status",
			Description: "Check host provider auth state without launching a worker. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"provider":{"type":"string","enum":["codex","claude"]},"workdir":{"type":"string"}},"required":["provider"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_auth_repair",
			Description: "Launch the provider's host authentication flow directly, then recheck provider status. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"provider":{"type":"string","enum":["claude"]},"workdir":{"type":"string"}},"required":["provider"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.apply_edit",
			Description: "Run dorkpipe apply-edit for a prepared artifact directory. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"artifact_dir":{"type":"string"}},"required":["artifact_dir"],"additionalProperties":false}`),
		},
	}
}

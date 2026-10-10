package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"dorkpipe.orchestrator/providersession"
)

// Server holds MCP server state (version string for initialize).
type Server struct {
	Version                                 string
	providerPoolChatRunner                  dorkpipeRunner
	providerPoolChatStreamRunner            providerPoolChatStreamingRunner
	providerPoolChatInteractiveStreamRunner providerPoolChatInteractiveStreamingRunner
	activeChat                              activeProviderPoolChatController
}

type dorkpipeRunner func(context.Context, []string) (stdout, stderr string, exitCode int, err error)

type activeProviderPoolChat struct {
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	approvals     *transientApprovalController
	inputs        *transientUserInputController
	cancellations *transientCancellationController
}

type activeProviderPoolChatContextKey struct{}

type activeProviderPoolChatController struct {
	mu     sync.Mutex
	active *activeProviderPoolChat
}

type stdioReadResult struct {
	raw []byte
	err error
}

type stdioAsyncResponse struct {
	response     *rpcResponse
	replyAsBatch bool
}

type serializedResponseWriter struct {
	mu     sync.Mutex
	out    io.Writer
	closed bool
}

func (w *serializedResponseWriter) write(body []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("mcpbridge: stdio response transport is closed")
	}
	return WriteMessage(w.out, body)
}

func (w *serializedResponseWriter) close() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
}

// NewServer builds a server; version falls back to DOCKPIPE_MCP_SERVER_VERSION or "0.0.0-dev".
func NewServer(version string) *Server {
	v := strings.TrimSpace(version)
	if v == "" {
		v = strings.TrimSpace(os.Getenv("DOCKPIPE_MCP_SERVER_VERSION"))
	}
	if v == "" {
		v = "0.0.0-dev"
	}
	return &Server{Version: v}
}

// ServeStdio runs the MCP JSON-RPC loop over Content-Length–framed messages.
func (s *Server) ServeStdio(in io.Reader, out io.Writer, log io.Writer) error {
	serveCtx, cancelServe := context.WithCancel(context.Background())
	responses := &serializedResponseWriter{out: out}
	br := bufio.NewReader(in)
	reads := make(chan stdioReadResult, 1)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			raw, err := ReadMessage(br)
			select {
			case reads <- stdioReadResult{raw: raw, err: err}:
			case <-serveCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	asyncResponses := make(chan stdioAsyncResponse, 1)
	shutdown := func(closeInput bool) {
		responses.close()
		cancelServe()
		s.cancelAndWaitForActiveProviderPoolChat()
		if closeInput {
			if closer, ok := in.(io.Closer); ok {
				_ = closer.Close()
				<-readerDone
				return
			}
			select {
			case <-readerDone:
			default:
			}
			return
		}
		<-readerDone
	}

	for {
		select {
		case read := <-reads:
			if read.err != nil {
				shutdown(false)
				if read.err == io.EOF {
					return nil
				}
				return read.err
			}
			reqBody, replyAsBatch := unwrapSingleRequestBatch(read.raw)
			if id, ok := providerPoolChatRequestID(serveCtx, reqBody); ok {
				active, started := s.beginActiveProviderPoolChat(serveCtx)
				if !started {
					resp := errResponse(id, -32000, "dorkpipe.provider_pool_chat is already active for this MCP server")
					if err := writeStdioResponse(responses, resp, replyAsBatch, log); err != nil {
						shutdown(true)
						return err
					}
					continue
				}
				go func(raw []byte, batch bool, call *activeProviderPoolChat) {
					resp := s.handleMessage(call.ctx, raw, log)
					s.finishActiveProviderPoolChat(call)
					if resp == nil {
						return
					}
					select {
					case asyncResponses <- stdioAsyncResponse{response: resp, replyAsBatch: batch}:
					case <-serveCtx.Done():
					}
				}(reqBody, replyAsBatch, active)
				continue
			}
			resp := s.handleMessage(serveCtx, reqBody, log)
			if resp == nil {
				continue
			}
			if err := writeStdioResponse(responses, resp, replyAsBatch, log); err != nil {
				shutdown(true)
				return err
			}
		case async := <-asyncResponses:
			if err := writeStdioResponse(responses, async.response, async.replyAsBatch, log); err != nil {
				shutdown(true)
				return err
			}
		}
	}
}

func unwrapSingleRequestBatch(raw []byte) ([]byte, bool) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 || trim[0] != '[' {
		return raw, false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(trim, &arr); err != nil || len(arr) != 1 {
		return raw, false
	}
	return []byte(arr[0]), true
}

func providerPoolChatRequestID(ctx context.Context, raw []byte) (*json.RawMessage, bool) {
	var req rpcRequest
	if json.Unmarshal(raw, &req) != nil || req.JSONRPC != "2.0" || req.ID == nil || req.Method != "tools/call" {
		return nil, false
	}
	var params struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(req.Params, &params) != nil || params.Name != "dorkpipe.provider_pool_chat" || !ToolAllowed(ctx, params.Name) {
		return nil, false
	}
	return req.ID, true
}

func writeStdioResponse(out *serializedResponseWriter, resp *rpcResponse, replyAsBatch bool, log io.Writer) error {
	body, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(log, "mcpbridge: marshal response: %v\n", err)
		return nil
	}
	if replyAsBatch {
		body, err = json.Marshal([]json.RawMessage{json.RawMessage(body)})
		if err != nil {
			fmt.Fprintf(log, "mcpbridge: marshal batch response: %v\n", err)
			return nil
		}
	}
	return out.write(body)
}

func (s *Server) beginActiveProviderPoolChat(parent context.Context) (*activeProviderPoolChat, bool) {
	s.activeChat.mu.Lock()
	defer s.activeChat.mu.Unlock()
	if s.activeChat.active != nil {
		return nil, false
	}
	baseCtx, cancel := context.WithCancel(parent)
	active := &activeProviderPoolChat{
		cancel:        cancel,
		done:          make(chan struct{}),
		approvals:     newTransientApprovalController(),
		inputs:        newTransientUserInputController(),
		cancellations: newTransientCancellationController(),
	}
	active.ctx = context.WithValue(baseCtx, activeProviderPoolChatContextKey{}, active)
	s.activeChat.active = active
	return active, true
}

func (s *Server) finishActiveProviderPoolChat(active *activeProviderPoolChat) {
	s.activeChat.mu.Lock()
	if s.activeChat.active == active {
		s.activeChat.active = nil
	}
	active.approvals.close(errors.New("provider-pool chat is no longer active"))
	active.inputs.close(errors.New("provider-pool chat is no longer active"))
	active.cancellations.close(errors.New("provider-pool chat is no longer active"))
	close(active.done)
	s.activeChat.mu.Unlock()
}

func (s *Server) cancelAndWaitForActiveProviderPoolChat() {
	s.activeChat.mu.Lock()
	active := s.activeChat.active
	if active != nil {
		active.cancel()
		active.approvals.close(errors.New("provider-pool chat was cancelled"))
		active.inputs.close(errors.New("provider-pool chat was cancelled"))
		active.cancellations.close(errors.New("provider-pool chat was cancelled"))
	}
	s.activeChat.mu.Unlock()
	if active != nil {
		<-active.done
	}
}

func activeProviderPoolChatFromContext(ctx context.Context) (*activeProviderPoolChat, bool) {
	active, ok := ctx.Value(activeProviderPoolChatContextKey{}).(*activeProviderPoolChat)
	return active, ok && active != nil
}

func (s *Server) activeApprovalRequest() (providersession.ApprovalRequest, error) {
	s.activeChat.mu.Lock()
	active := s.activeChat.active
	s.activeChat.mu.Unlock()
	if active == nil {
		return providersession.ApprovalRequest{}, errors.New("no provider-pool chat is active")
	}
	return active.approvals.pendingRequest()
}

func (s *Server) submitActiveApprovalDecision(ctx context.Context, decision providersession.ApprovalDecision) error {
	s.activeChat.mu.Lock()
	active := s.activeChat.active
	s.activeChat.mu.Unlock()
	if active == nil {
		return errors.New("no provider-pool chat is active")
	}
	return active.approvals.submit(ctx, decision)
}

func (s *Server) activeUserInputPrompt() (providersession.UserInputPrompt, error) {
	s.activeChat.mu.Lock()
	active := s.activeChat.active
	s.activeChat.mu.Unlock()
	if active == nil {
		return providersession.UserInputPrompt{}, errors.New("no provider-pool chat is active")
	}
	return active.inputs.pendingPrompt()
}

func (s *Server) submitActiveUserInputResponse(ctx context.Context, response providersession.UserInputResponse) error {
	s.activeChat.mu.Lock()
	active := s.activeChat.active
	s.activeChat.mu.Unlock()
	if active == nil {
		return errors.New("no provider-pool chat is active")
	}
	err := active.inputs.submit(ctx, response)
	if err != nil && ctx.Err() != nil {
		active.cancel()
		active.approvals.close(ctx.Err())
		active.inputs.close(ctx.Err())
		active.cancellations.close(ctx.Err())
		<-active.done
	}
	return err
}

func (s *Server) activeCancellationScope() (providerPoolCancellationScope, error) {
	s.activeChat.mu.Lock()
	active := s.activeChat.active
	s.activeChat.mu.Unlock()
	if active == nil {
		return providerPoolCancellationScope{}, errors.New("no provider-pool chat is active")
	}
	return active.cancellations.pendingScope()
}

func (s *Server) submitActiveCancellationIntent(ctx context.Context, intent providersession.CancellationIntent) error {
	s.activeChat.mu.Lock()
	active := s.activeChat.active
	s.activeChat.mu.Unlock()
	if active == nil {
		return errors.New("no provider-pool chat is active")
	}
	err := active.cancellations.submit(ctx, intent)
	if err != nil && ctx.Err() != nil {
		active.cancel()
		active.approvals.close(ctx.Err())
		active.inputs.close(ctx.Err())
		active.cancellations.close(ctx.Err())
		<-active.done
	}
	return err
}

func (s *Server) handleMessage(ctx context.Context, raw []byte, log io.Writer) *rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return errResponse(nil, -32700, "parse error")
	}
	if req.JSONRPC != "2.0" {
		return errResponse(req.ID, -32600, "invalid request")
	}
	if req.ID == nil {
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil
		}
		return nil
	}
	switch req.Method {
	case "initialize":
		return s.handleInitialize(&req)
	case "tools/list":
		return s.handleToolsList(ctx, &req)
	case "tools/call":
		return s.handleToolsCall(ctx, &req, log)
	case "ping":
		return okResponse(req.ID, json.RawMessage(`{}`))
	default:
		return errResponse(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(req *rpcRequest) *rpcResponse {
	type result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools struct{} `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	type initParams struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	var p initParams
	_ = json.Unmarshal(req.Params, &p)
	ver := strings.TrimSpace(p.ProtocolVersion)
	if ver == "" {
		ver = "2024-11-05"
	}
	// Cursor and other hosts send newer MCP protocol lines; echo the client's version so
	// the handshake completes (we only implement tools over stdio; wire format matches).
	var r result
	r.ProtocolVersion = ver
	r.Capabilities.Tools = struct{}{}
	r.ServerInfo.Name = "dorkpipe"
	r.ServerInfo.Version = s.Version
	b, err := json.Marshal(r)
	if err != nil {
		return errResponse(req.ID, -32603, "internal error")
	}
	return okResponse(req.ID, b)
}

func (s *Server) handleToolsList(ctx context.Context, req *rpcRequest) *rpcResponse {
	type tool struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"`
	}
	var tools []tool
	for _, m := range mcpToolCatalog() {
		if !ToolAllowed(ctx, m.Name) {
			continue
		}
		tools = append(tools, tool{Name: m.Name, Description: m.Description, InputSchema: m.InputSchema})
	}
	out := struct {
		Tools []tool `json:"tools"`
	}{Tools: tools}
	b, err := json.Marshal(out)
	if err != nil {
		return errResponse(req.ID, -32603, "internal error")
	}
	return okResponse(req.ID, b)
}

func (s *Server) handleToolsCall(ctx context.Context, req *rpcRequest, log io.Writer) *rpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errResponse(req.ID, -32602, "invalid params")
	}
	raw, isErr, err := s.dispatchTool(ctx, params.Name, params.Arguments)
	if err != nil {
		fmt.Fprintf(log, "mcpbridge tool %s: %v\n", params.Name, err)
		return errResponse(req.ID, -32000, err.Error())
	}
	tr := toolResultJSON(string(raw), isErr)
	return okResponse(req.ID, json.RawMessage(tr))
}

func (s *Server) dispatchTool(ctx context.Context, name string, args json.RawMessage) ([]byte, bool, error) {
	if !ToolAllowed(ctx, name) {
		return nil, true, fmt.Errorf("mcp: tool %q not allowed (effective tier %s; see DOCKPIPE_MCP_TIER or HTTP key tiers file)", name, effectiveTierLabel(ctx))
	}
	switch name {
	case "dockpipe.version":
		out, stderr, code, err := runDockpipe(ctx, []string{"--version"})
		if err != nil {
			return nil, true, err
		}
		text := strings.TrimSpace(out + stderr)
		if code != 0 {
			return []byte(text), true, nil
		}
		return []byte(text), false, nil

	case "capabilities.workflows":
		names, err := listWorkflowNames()
		if err != nil {
			return nil, true, err
		}
		b, err := json.MarshalIndent(names, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return b, false, nil

	case "repo.list_files":
		var in struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		_ = json.Unmarshal(args, &in)
		files, err := repoListFiles(in.Query, in.Limit)
		if err != nil {
			return nil, true, err
		}
		b, err := json.MarshalIndent(files, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return b, false, nil

	case "repo.read_file":
		var in struct {
			Path     string `json:"path"`
			MaxChars int    `json:"max_chars"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		text, err := repoReadFile(in.Path, in.MaxChars)
		if err != nil {
			return nil, true, err
		}
		return []byte(text), false, nil

	case "repo.search_text":
		var in struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		matches, err := repoSearchText(in.Query, in.Limit)
		if err != nil {
			return nil, true, err
		}
		b, err := json.MarshalIndent(matches, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return b, false, nil

	case "dockpipe.validate_workflow":
		var in struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &in)
		path := strings.TrimSpace(in.Path)
		var cmdArgs []string
		if path == "" {
			cmdArgs = []string{"workflow", "validate"}
		} else {
			absPath, err := ResolvePathUnderRepoRoot(path)
			if err != nil {
				return nil, true, err
			}
			cmdArgs = []string{"workflow", "validate", absPath}
		}
		_, stderr, code, err := runDockpipe(ctx, cmdArgs)
		msg := strings.TrimSpace(stderr)
		if err != nil {
			return nil, true, err
		}
		if code != 0 {
			return []byte(msg), true, nil
		}
		return []byte(msg), false, nil

	case "dorkpipe.validate_spec":
		var in struct {
			SpecPath string `json:"spec_path"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		sp := strings.TrimSpace(in.SpecPath)
		if sp == "" {
			return nil, true, fmt.Errorf("spec_path required")
		}
		absSpec, err := ResolvePathUnderRepoRoot(sp)
		if err != nil {
			return nil, true, err
		}
		_, stderr, code, err := runDorkpipe(ctx, []string{"validate", "-f", absSpec})
		msg := strings.TrimSpace(stderr)
		if err != nil {
			return nil, true, err
		}
		if code != 0 {
			return []byte(msg), true, nil
		}
		return []byte(msg), false, nil

	case "dockpipe.run":
		var in dockpipeRunInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		cmdArgs, err := in.commandArgs()
		if err != nil {
			return nil, true, err
		}
		stdout, stderr, code, err := runDockpipe(ctx, cmdArgs)
		if err != nil {
			return nil, true, err
		}
		return in.formatResult(stdout, stderr, code)

	case "dorkpipe.run_spec":
		var in struct {
			SpecPath string `json:"spec_path"`
			Workdir  string `json:"workdir"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		sp := strings.TrimSpace(in.SpecPath)
		if sp == "" {
			return nil, true, fmt.Errorf("spec_path required")
		}
		absSpec, err := ResolvePathUnderRepoRoot(sp)
		if err != nil {
			return nil, true, err
		}
		dargs := []string{"run", "-f", absSpec}
		if restrictExecWorkdirToRepo() {
			awd, err := resolveExecWorkdir(in.Workdir)
			if err != nil {
				return nil, true, err
			}
			dargs = append(dargs, "--workdir", awd)
		} else if wd := strings.TrimSpace(in.Workdir); wd != "" {
			awd, err := absWorkdir(wd)
			if err != nil {
				return nil, true, err
			}
			dargs = append(dargs, "--workdir", awd)
		}
		stdout, stderr, code, err := runDorkpipe(ctx, dargs)
		if err != nil {
			return nil, true, err
		}
		summary := fmt.Sprintf("exit_code=%d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		return []byte(summary), code != 0, nil

	case "dorkpipe.request":
		var in struct {
			Workdir         string   `json:"workdir"`
			Message         string   `json:"message"`
			Mode            string   `json:"mode"`
			SessionID       string   `json:"session_id"`
			ProviderPreset  string   `json:"provider_preset"`
			ModelProvider   string   `json:"model_provider"`
			Model           string   `json:"model"`
			ActiveFile      string   `json:"active_file"`
			OpenFiles       []string `json:"open_files"`
			SelectionText   string   `json:"selection_text"`
			AttachmentFiles []string `json:"attachment_files"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		message := strings.TrimSpace(in.Message)
		if message == "" {
			return nil, true, fmt.Errorf("message required")
		}
		wd, err := resolveExecWorkdir(in.Workdir)
		if err != nil {
			return nil, true, err
		}
		activeFile, err := normalizeRepoHintPath(in.ActiveFile)
		if err != nil {
			return nil, true, err
		}
		openFiles, err := normalizeRepoHintPaths(in.OpenFiles)
		if err != nil {
			return nil, true, err
		}
		attachmentFiles, err := normalizeAttachmentPaths(in.AttachmentFiles)
		if err != nil {
			return nil, true, err
		}
		mode := strings.TrimSpace(in.Mode)
		if mode == "" {
			mode = "ask"
		}
		dargs := []string{"request", "--execute", "--workdir", wd, "--mode", mode, "--message", message}
		if providerPreset := strings.TrimSpace(in.ProviderPreset); providerPreset != "" {
			dargs = append(dargs, "--provider-preset", providerPreset)
		}
		if modelProvider := strings.TrimSpace(in.ModelProvider); modelProvider != "" {
			dargs = append(dargs, "--model-provider", modelProvider)
		}
		if model := strings.TrimSpace(in.Model); model != "" {
			dargs = append(dargs, "--model", model)
		}
		if activeFile != "" {
			dargs = append(dargs, "--active-file", activeFile)
		}
		for _, openFile := range openFiles {
			dargs = append(dargs, "--open-file", openFile)
		}
		if selection := strings.TrimSpace(in.SelectionText); selection != "" {
			dargs = append(dargs, "--selection-text", selection)
		}
		for _, attachment := range attachmentFiles {
			dargs = append(dargs, "--attachment-file", attachment)
		}
		summary, err := runDorkpipeEventStream(ctx, dargs)
		if err != nil {
			return nil, true, err
		}
		out, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return nil, true, err
		}
		isError := summary.ExitCode != 0
		if summary.FinalEvent != nil {
			if strings.TrimSpace(in.SessionID) != "" {
				if metadata, ok := summary.FinalEvent["metadata"].(map[string]any); ok {
					metadata["pipeon_session_id"] = strings.TrimSpace(in.SessionID)
					metadata["session_context"] = "pipeon_recent_history"
				}
			}
			if eventType, _ := summary.FinalEvent["type"].(string); strings.TrimSpace(eventType) == "error" {
				isError = true
			}
		}
		return out, isError, nil

	case "dorkpipe.provider_pool_catalog":
		var in struct {
			Workdir string `json:"workdir"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		wd, err := resolveExecWorkdir(in.Workdir)
		if err != nil {
			return nil, true, err
		}
		stdout, stderr, code, err := runDorkpipe(ctx, []string{"provider-pool", "catalog", "--workdir", wd, "--json"})
		if err != nil {
			return nil, true, err
		}
		if code != 0 {
			return []byte(stdout + stderr), true, nil
		}
		return []byte(stdout), false, nil

	case "dorkpipe.provider_pool_status":
		var in struct {
			Workdir  string `json:"workdir"`
			Provider string `json:"provider"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		wd, err := resolveExecWorkdir(in.Workdir)
		if err != nil {
			return nil, true, err
		}
		dargs := []string{"provider-pool", "status", "--workdir", wd, "--json"}
		if strings.TrimSpace(in.Provider) != "" {
			dargs = append(dargs, "--provider", strings.TrimSpace(in.Provider))
		}
		stdout, stderr, code, err := runDorkpipe(ctx, dargs)
		if err != nil {
			return nil, true, err
		}
		if code != 0 {
			return []byte(stdout + stderr), true, nil
		}
		return []byte(stdout), false, nil

	case "dorkpipe.provider_pool_approval_request":
		var in struct{}
		if err := decodeClosedJSON(args, &in); err != nil {
			return nil, true, fmt.Errorf("invalid approval-request arguments: %w", err)
		}
		request, err := s.activeApprovalRequest()
		if err != nil {
			return nil, true, err
		}
		out, err := json.Marshal(request)
		if err != nil {
			return nil, true, err
		}
		return out, false, nil

	case "dorkpipe.provider_pool_approval_decide":
		var decision providersession.ApprovalDecision
		if err := decodeClosedJSON(args, &decision); err != nil {
			return nil, true, fmt.Errorf("invalid approval-decision arguments: %w", err)
		}
		if err := s.submitActiveApprovalDecision(ctx, decision); err != nil {
			return nil, true, err
		}
		return []byte(`{"delivered":true}`), false, nil

	case "dorkpipe.provider_pool_user_input_request":
		var in struct{}
		if err := decodeClosedJSON(args, &in); err != nil {
			return nil, true, fmt.Errorf("invalid user-input-request arguments: %w", err)
		}
		prompt, err := s.activeUserInputPrompt()
		if err != nil {
			return nil, true, err
		}
		out, err := json.Marshal(prompt)
		if err != nil {
			return nil, true, err
		}
		return out, false, nil

	case "dorkpipe.provider_pool_user_input_respond":
		var response providersession.UserInputResponse
		if !utf8.Valid(args) {
			return nil, true, errors.New("invalid user-input-response arguments: valid UTF-8 is required")
		}
		if err := decodeClosedJSON(args, &response); err != nil {
			return nil, true, fmt.Errorf("invalid user-input-response arguments: %w", err)
		}
		if err := s.submitActiveUserInputResponse(ctx, response); err != nil {
			return nil, true, err
		}
		return []byte(`{"delivered":true}`), false, nil

	case "dorkpipe.provider_pool_cancellation_request":
		var in map[string]json.RawMessage
		if err := decodeClosedJSON(args, &in); err != nil {
			return nil, true, fmt.Errorf("invalid cancellation-request arguments: %w", err)
		}
		if in == nil || len(in) != 0 {
			return nil, true, errors.New("invalid cancellation-request arguments: an empty object is required")
		}
		scope, err := s.activeCancellationScope()
		if err != nil {
			return nil, true, err
		}
		out, err := json.Marshal(scope)
		if err != nil {
			return nil, true, err
		}
		return out, false, nil

	case "dorkpipe.provider_pool_cancellation_deliver":
		var intent providersession.CancellationIntent
		if err := decodeClosedJSON(args, &intent); err != nil {
			return nil, true, fmt.Errorf("invalid cancellation-deliver arguments: %w", err)
		}
		if err := s.submitActiveCancellationIntent(ctx, intent); err != nil {
			return nil, true, err
		}
		return []byte(`{"delivered":true}`), false, nil

	case "dorkpipe.provider_pool_chat":
		var in providerPoolChatInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		wd, err := resolveExecWorkdir(in.Workdir)
		if err != nil {
			return nil, true, err
		}
		dargs := providerPoolChatArgs(wd, in)
		active, activeOK := activeProviderPoolChatFromContext(ctx)
		if !activeOK {
			return nil, true, errors.New("provider-pool chat is not owned by an active MCP request")
		}
		var stdout, stderr string
		var code int
		if s.providerPoolChatInteractiveStreamRunner != nil {
			stdout, stderr, code, err = s.providerPoolChatInteractiveStreamRunner(ctx, dargs, active.approvals, active.inputs, active.cancellations)
		} else if s.providerPoolChatStreamRunner != nil {
			stdout, stderr, code, err = s.providerPoolChatStreamRunner(ctx, dargs, active.approvals)
		} else if s.providerPoolChatRunner != nil {
			stdout, stderr, code, err = s.providerPoolChatRunner(ctx, dargs)
		} else if providerPoolChatUsesApprovalTransport(in) {
			stdout, stderr, code, err = runDorkpipeWithInteractiveTransport(ctx, dargs, active.approvals, active.inputs, active.cancellations)
		} else {
			stdout, stderr, code, err = runDorkpipe(ctx, dargs)
		}
		if err != nil {
			return nil, true, err
		}
		if code != 0 && strings.TrimSpace(stdout) == "" {
			return []byte(stdout + stderr), true, nil
		}
		return []byte(stdout), false, nil

	case "dorkpipe.host_codex_chat":
		var in struct {
			Workdir       string   `json:"workdir"`
			Message       string   `json:"message"`
			Model         string   `json:"model"`
			SessionID     string   `json:"session_id"`
			ActiveFile    string   `json:"active_file"`
			OpenFiles     []string `json:"open_files"`
			SelectionText string   `json:"selection_text"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		activeFile, err := normalizeRepoHintPath(in.ActiveFile)
		if err != nil {
			return nil, true, err
		}
		openFiles, err := normalizeRepoHintPaths(in.OpenFiles)
		if err != nil {
			return nil, true, err
		}
		summary, err := runHostCodexChat(ctx, in.Workdir, in.Message, in.Model, in.SessionID, activeFile, in.SelectionText, openFiles)
		if err != nil {
			return nil, true, err
		}
		out, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return out, summary.ExitCode != 0, nil

	case "dorkpipe.host_claude_chat":
		var in struct {
			Workdir       string   `json:"workdir"`
			Message       string   `json:"message"`
			Model         string   `json:"model"`
			SessionID     string   `json:"session_id"`
			ActiveFile    string   `json:"active_file"`
			OpenFiles     []string `json:"open_files"`
			SelectionText string   `json:"selection_text"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		activeFile, err := normalizeRepoHintPath(in.ActiveFile)
		if err != nil {
			return nil, true, err
		}
		openFiles, err := normalizeRepoHintPaths(in.OpenFiles)
		if err != nil {
			return nil, true, err
		}
		summary, err := runHostClaudeChat(ctx, in.Workdir, in.Message, in.Model, in.SessionID, activeFile, in.SelectionText, openFiles)
		if err != nil {
			return nil, true, err
		}
		out, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return out, summary.ExitCode != 0, nil

	case "dorkpipe.host_claude_auth":
		var in struct {
			Workdir string `json:"workdir"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		summary, err := runHostClaudeAuth(ctx, in.Workdir)
		if err != nil {
			return nil, true, err
		}
		out, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return out, false, nil

	case "dorkpipe.provider_auth_status":
		var in struct {
			Provider string `json:"provider"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		provider := strings.ToLower(strings.TrimSpace(in.Provider))
		if provider != "codex" && provider != "claude" {
			return nil, true, fmt.Errorf("unsupported provider %q", in.Provider)
		}
		status := providerAuthStatusFor(provider)
		out, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return out, !status.Authenticated, nil

	case "dorkpipe.provider_auth_repair":
		var in struct {
			Provider string `json:"provider"`
			Workdir  string `json:"workdir"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		provider := strings.ToLower(strings.TrimSpace(in.Provider))
		if provider != "claude" {
			return nil, true, fmt.Errorf("auth repair currently supports claude only")
		}
		summary, err := runHostClaudeAuth(ctx, in.Workdir)
		if err != nil {
			return nil, true, err
		}
		out, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return out, false, nil

	case "dorkpipe.apply_edit":
		var in struct {
			Workdir     string `json:"workdir"`
			ArtifactDir string `json:"artifact_dir"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		artifactDir := strings.TrimSpace(in.ArtifactDir)
		if artifactDir == "" {
			return nil, true, fmt.Errorf("artifact_dir required")
		}
		wd, err := resolveExecWorkdir(in.Workdir)
		if err != nil {
			return nil, true, err
		}
		if !filepath.IsAbs(artifactDir) {
			artifactDir = filepath.Join(wd, artifactDir)
		}
		artifactDir, err = resolveRepoBoundAbsolutePath(artifactDir)
		if err != nil {
			return nil, true, err
		}
		summary, err := runDorkpipeEventStream(ctx, []string{"apply-edit", "--workdir", wd, "--artifact-dir", artifactDir})
		if err != nil {
			return nil, true, err
		}
		out, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return nil, true, err
		}
		isError := summary.ExitCode != 0
		if summary.FinalEvent != nil {
			if eventType, _ := summary.FinalEvent["type"].(string); strings.TrimSpace(eventType) == "error" {
				isError = true
			}
		}
		return out, isError, nil

	default:
		return nil, true, fmt.Errorf("unknown tool %q", name)
	}
}

type providerPoolChatInput struct {
	Workdir        string   `json:"workdir"`
	Message        string   `json:"message"`
	Provider       string   `json:"provider"`
	Model          string   `json:"model"`
	SessionID      string   `json:"session_id"`
	SessionAdapter string   `json:"session_adapter"`
	ActiveFile     string   `json:"active_file"`
	OpenFiles      []string `json:"open_files"`
	SelectionText  string   `json:"selection_text"`
}

func providerPoolChatArgs(workdir string, in providerPoolChatInput) []string {
	dargs := []string{"provider-pool", "prompt", "--workdir", workdir, "--json", "--prompt", in.Message}
	for _, value := range []struct {
		flag  string
		value string
	}{
		{"--provider", in.Provider},
		{"--model", in.Model},
		{"--session-id", in.SessionID},
		{"--session-adapter", in.SessionAdapter},
		{"--active-file", in.ActiveFile},
	} {
		if trimmed := strings.TrimSpace(value.value); trimmed != "" {
			dargs = append(dargs, value.flag, trimmed)
		}
	}
	for _, item := range in.OpenFiles {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			dargs = append(dargs, "--open-file", trimmed)
		}
	}
	if selection := strings.TrimSpace(in.SelectionText); selection != "" {
		dargs = append(dargs, "--selection-text", selection)
	}
	return dargs
}

func providerPoolChatUsesApprovalTransport(in providerPoolChatInput) bool {
	return strings.TrimSpace(in.SessionAdapter) == "codex_app_server"
}

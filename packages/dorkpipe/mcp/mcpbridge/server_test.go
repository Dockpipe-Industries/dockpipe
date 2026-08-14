package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestServerInitialize(t *testing.T) {
	t.Parallel()
	s := &Server{Version: "1.2.3"}
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	resp := s.handleMessage(context.Background(), raw, io.Discard)
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp=%+v", resp)
	}
	var out struct {
		ServerInfo struct {
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if out.ServerInfo.Version != "1.2.3" {
		t.Fatalf("version %q", out.ServerInfo.Version)
	}
}

func TestProviderPoolChatArgsPassesExplicitSessionAdapter(t *testing.T) {
	got := providerPoolChatArgs("C:\\repo", providerPoolChatInput{
		Message:        "hello",
		Provider:       " codex ",
		Model:          " config ",
		SessionID:      " pipeon-session-1 ",
		SessionAdapter: " codex_exec ",
		ActiveFile:     " main.go ",
		OpenFiles:      []string{" one.go ", "", "two.go"},
		SelectionText:  " selected ",
	})
	want := []string{"provider-pool", "prompt", "--workdir", "C:\\repo", "--json", "--prompt", "hello", "--provider", "codex", "--model", "config", "--session-id", "pipeon-session-1", "--session-adapter", "codex_exec", "--active-file", "main.go", "--open-file", "one.go", "--open-file", "two.go", "--selection-text", "selected"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestProviderPoolChatSchemaPinsKnownAdapterValues(t *testing.T) {
	var schema struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
		AdditionalProperties bool `json:"additionalProperties"`
	}
	found := false
	for _, tool := range mcpToolCatalog() {
		if tool.Name != "dorkpipe.provider_pool_chat" {
			continue
		}
		found = true
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatal("provider-pool chat tool missing")
	}
	if !reflect.DeepEqual(schema.Properties["session_adapter"].Enum, []string{"codex_exec", "codex_app_server"}) || schema.AdditionalProperties {
		t.Fatalf("session adapter schema = %+v", schema)
	}
}

func TestProviderPoolChatApprovalTransportRequiresExplicitAppServerAdapter(t *testing.T) {
	for _, tc := range []struct {
		adapter string
		want    bool
	}{
		{"", false},
		{"codex_exec", false},
		{" codex_app_server ", true},
		{"unknown", false},
	} {
		if got := providerPoolChatUsesApprovalTransport(providerPoolChatInput{SessionAdapter: tc.adapter}); got != tc.want {
			t.Fatalf("adapter %q approval transport=%t, want %t", tc.adapter, got, tc.want)
		}
	}
}

func TestServerInitialize_echoesClientProtocolVersion(t *testing.T) {
	t.Parallel()
	s := &Server{Version: "1"}
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	resp := s.handleMessage(context.Background(), raw, io.Discard)
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp=%+v", resp)
	}
	var out struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if out.ProtocolVersion != "2025-11-25" {
		t.Fatalf("protocolVersion %q", out.ProtocolVersion)
	}
}

func TestServeStdio_repliesWithBatchWhenRequestWasSingleElementBatch(t *testing.T) {
	t.Parallel()
	s := &Server{Version: "9.9.9"}
	payload := `[{"jsonrpc":"2.0","id":99,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"c","version":"1"}}}]`
	var inBuf bytes.Buffer
	if err := WriteMessage(&inBuf, []byte(payload)); err != nil {
		t.Fatal(err)
	}
	var outBuf bytes.Buffer
	if err := s.ServeStdio(&inBuf, &outBuf, io.Discard); err != nil {
		t.Fatal(err)
	}
	br := bytes.NewReader(outBuf.Bytes())
	got, err := ReadMessage(bufio.NewReader(br))
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if got[0] != '[' {
		t.Fatalf("expected JSON array response for batch request, got: %s", got)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(got, &arr); err != nil || len(arr) != 1 {
		t.Fatalf("expected one-element batch, got %v err=%v", arr, err)
	}
	var inner struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(arr[0], &inner); err != nil {
		t.Fatal(err)
	}
	if inner.Result.ProtocolVersion != "2025-11-25" {
		t.Fatalf("protocolVersion %q", inner.Result.ProtocolVersion)
	}
}

func TestServerNotificationNoResponse(t *testing.T) {
	t.Parallel()
	s := &Server{Version: "1"}
	raw := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`)
	resp := s.handleMessage(context.Background(), raw, io.Discard)
	if resp != nil {
		t.Fatalf("expected nil for notification, got %+v", resp)
	}
}

func TestServeStdioProviderPoolChatAllowsPingToCompleteFirstWithExactFraming(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	started := make(chan struct{})
	release := make(chan struct{})
	server.providerPoolChatRunner = func(ctx context.Context, _ []string) (string, string, int, error) {
		close(started)
		select {
		case <-release:
			return `{"state":"ready"}`, "", 0, nil
		case <-ctx.Done():
			return "", "", -1, ctx.Err()
		}
	}

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, outWriter, io.Discard) }()

	writeStdioFixture(t, inWriter, providerPoolChatFixture(`"chat-request"`))
	waitFixtureSignal(t, started, "provider-pool chat start")
	writeStdioFixture(t, inWriter, `{"jsonrpc":"2.0","id":42,"method":"ping"}`)

	reader := bufio.NewReader(outReader)
	ping := readStdioFixture(t, reader)
	assertResponseID(t, ping, "42")
	close(release)
	chat := readStdioFixture(t, reader)
	assertResponseID(t, chat, `"chat-request"`)

	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	_ = outWriter.Close()
	_ = outReader.Close()
}

func TestServeStdioRejectsSecondConcurrentChatBeforeRunnerAndReleasesSlot(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	server.providerPoolChatRunner = func(ctx context.Context, _ []string) (string, string, int, error) {
		call := calls.Add(1)
		if call == 1 {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return "", "", -1, ctx.Err()
			}
		}
		return fmt.Sprintf(`{"call":%d}`, call), "", 0, nil
	}

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, outWriter, io.Discard) }()
	reader := bufio.NewReader(outReader)

	writeStdioFixture(t, inWriter, providerPoolChatFixture("1"))
	waitFixtureSignal(t, started, "first provider-pool chat start")
	writeStdioFixture(t, inWriter, providerPoolChatFixture("2"))
	busy := readStdioFixture(t, reader)
	assertResponseID(t, busy, "2")
	var busyResponse rpcResponse
	if err := json.Unmarshal(busy, &busyResponse); err != nil {
		t.Fatal(err)
	}
	if busyResponse.Error == nil || !strings.Contains(busyResponse.Error.Message, "already active") || calls.Load() != 1 {
		t.Fatalf("busy response=%s runner calls=%d", busy, calls.Load())
	}

	close(release)
	first := readStdioFixture(t, reader)
	assertResponseID(t, first, "1")
	writeStdioFixture(t, inWriter, providerPoolChatFixture("3"))
	third := readStdioFixture(t, reader)
	assertResponseID(t, third, "3")
	if calls.Load() != 2 {
		t.Fatalf("runner calls=%d, want 2", calls.Load())
	}

	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	_ = outWriter.Close()
	_ = outReader.Close()
}

func TestServeStdioEOFCancelsAndJoinsActiveChatWithoutLateResponse(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server.providerPoolChatRunner = func(ctx context.Context, _ []string) (string, string, int, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return "", "", -1, ctx.Err()
	}

	inReader, inWriter := io.Pipe()
	var output bytes.Buffer
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, &output, io.Discard) }()
	writeStdioFixture(t, inWriter, providerPoolChatFixture("7"))
	waitFixtureSignal(t, started, "provider-pool chat start")
	if err := inWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitServeResult(t, serveDone); err != nil {
		t.Fatal(err)
	}
	waitFixtureSignal(t, cancelled, "provider-pool chat cancellation")
	if output.Len() != 0 {
		t.Fatalf("late response written after EOF: %q", output.String())
	}
}

func TestServeStdioWriteFailureCancelsAndJoinsActiveChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server.providerPoolChatRunner = func(ctx context.Context, _ []string) (string, string, int, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return "", "", -1, ctx.Err()
	}

	inReader, inWriter := io.Pipe()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.ServeStdio(inReader, errorWriter{}, io.Discard) }()
	writeStdioFixture(t, inWriter, providerPoolChatFixture("8"))
	waitFixtureSignal(t, started, "provider-pool chat start")
	writeStdioFixture(t, inWriter, `{"jsonrpc":"2.0","id":9,"method":"ping"}`)
	if err := waitServeResult(t, serveDone); err == nil || !strings.Contains(err.Error(), "fixture transport failure") {
		t.Fatalf("serve error=%v", err)
	}
	waitFixtureSignal(t, cancelled, "provider-pool chat cancellation")
	_ = inWriter.Close()
}

func TestServeStdioProviderPoolChatKeepsTierEnforcementAndSerialPingBehavior(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_TIER", "validate")
	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "")
	t.Setenv("DOCKPIPE_MCP_ALLOWED_TOOLS", "")
	server := NewServer("test")
	var calls atomic.Int32
	server.providerPoolChatRunner = func(context.Context, []string) (string, string, int, error) {
		calls.Add(1)
		return "unexpected", "", 0, nil
	}

	var input bytes.Buffer
	writeStdioFixture(t, &input, providerPoolChatFixture("10"))
	writeStdioFixture(t, &input, `{"jsonrpc":"2.0","id":11,"method":"ping"}`)
	var output bytes.Buffer
	if err := server.ServeStdio(&input, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(bytes.NewReader(output.Bytes()))
	denied := readStdioFixture(t, reader)
	assertResponseID(t, denied, "10")
	var deniedResponse rpcResponse
	if err := json.Unmarshal(denied, &deniedResponse); err != nil {
		t.Fatal(err)
	}
	if deniedResponse.Error == nil || !strings.Contains(deniedResponse.Error.Message, "not allowed") || calls.Load() != 0 {
		t.Fatalf("denied response=%s runner calls=%d", denied, calls.Load())
	}
	ping := readStdioFixture(t, reader)
	assertResponseID(t, ping, "11")
}

func TestMCPBridgeExposesOnlyBoundedApprovalOperationsAtExecTier(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range mcpToolCatalog() {
		name := strings.ToLower(tool.Name)
		if strings.Contains(name, "approval") || strings.Contains(name, "decision") || strings.Contains(name, "decide") {
			seen[tool.Name] = true
			if tier, ok := minTierForTool(tool.Name); !ok || tier != TierExec {
				t.Fatalf("approval tool %q tier=%v present=%v", tool.Name, tier, ok)
			}
		}
	}
	want := map[string]bool{
		"dorkpipe.provider_pool_approval_request": true,
		"dorkpipe.provider_pool_approval_decide":  true,
	}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("approval tools=%v, want %v", seen, want)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("fixture transport failure")
}

func enableMCPExecTier(t *testing.T) {
	t.Helper()
	t.Setenv("DOCKPIPE_MCP_TIER", "exec")
	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "")
	t.Setenv("DOCKPIPE_MCP_ALLOWED_TOOLS", "")
}

func providerPoolChatFixture(id string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":{"name":"dorkpipe.provider_pool_chat","arguments":{"message":"fixture"}}}`, id)
}

func writeStdioFixture(t *testing.T, writer io.Writer, payload string) {
	t.Helper()
	if err := WriteMessage(writer, []byte(payload)); err != nil {
		t.Fatal(err)
	}
}

func readStdioFixture(t *testing.T, reader *bufio.Reader) []byte {
	t.Helper()
	type result struct {
		body []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		body, err := ReadMessage(reader)
		done <- result{body: body, err: err}
	}()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		return got.body
	case <-time.After(3 * time.Second):
		t.Fatal("timed out reading MCP response")
		return nil
	}
}

func assertResponseID(t *testing.T, body []byte, want string) {
	t.Helper()
	var response struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("invalid response frame %q: %v", body, err)
	}
	if string(response.ID) != want {
		t.Fatalf("response id=%s, want %s; body=%s", response.ID, want, body)
	}
}

func waitFixtureSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func waitServeResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for MCP server shutdown")
		return nil
	}
}

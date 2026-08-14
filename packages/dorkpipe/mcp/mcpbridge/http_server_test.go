package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

func TestAcceptsJSONContentType(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		ct   string
		want bool
	}{
		{"", true},
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"APPLICATION/JSON", true},
		{"application/ld+json", true},
		{"text/json", true},
		{"application/octet-stream", false},
		{"text/plain", false},
	} {
		if got := acceptsJSONContentType(tc.ct); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.ct, got, tc.want)
		}
	}
}

func TestIsLoopbackBind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8443", true},
		{"[::1]:8443", true},
		{"localhost:9", true},
		{":8443", false},
		{"0.0.0.0:8443", false},
		{"10.0.0.1:8443", false},
	} {
		if got := isLoopbackBind(tc.addr); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.addr, got, tc.want)
		}
	}
}

func TestServeHTTPRequiresAPIKeyAndTLSOrLoopback(t *testing.T) {
	t.Parallel()
	s := NewServer("test")
	ctx := context.Background()

	err := s.ServeHTTP(ctx, HTTPConfig{ListenAddr: "127.0.0.1:0", APIKey: ""})
	if err == nil || !strings.Contains(err.Error(), "MCP_HTTP") {
		t.Fatalf("expected HTTP auth config error, got %v", err)
	}

	err = s.ServeHTTP(ctx, HTTPConfig{ListenAddr: "127.0.0.1:0", APIKey: "k"})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") && !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("expected TLS / insecure hint, got %v", err)
	}

	err = s.ServeHTTP(ctx, HTTPConfig{ListenAddr: ":8443", APIKey: "k", AllowInsecurePlainHTTP: true})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback error for :port, got %v", err)
	}
}

func TestAPIKeyGateUnauthorized(t *testing.T) {
	t.Parallel()
	s := NewServer("1")
	h := s.jsonRPCHandler(io.Discard)
	srv := newIPv4TestServer(t, apiKeyGate("secret", h))
	defer srv.Close()

	resp, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestAPIKeyGateOK(t *testing.T) {
	t.Parallel()
	s := NewServer("1")
	h := s.jsonRPCHandler(io.Discard)
	srv := newIPv4TestServer(t, apiKeyGate("secret", h))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPKeyTierGateReadonlyVsExec(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_TIER", "")
	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "")
	s := NewServer("1")
	entries := []keyTierEntry{
		{key: "ro", tier: TierReadonly},
		{key: "ex", tier: TierExec},
	}
	h := httpKeyTierGate(entries, s.jsonRPCHandler(io.Discard))
	srv := newIPv4TestServer(t, h)
	defer srv.Close()

	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	reqRO, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	reqRO.Header.Set("Content-Type", "application/json")
	reqRO.Header.Set("Authorization", "Bearer ro")
	respRO, err := http.DefaultClient.Do(reqRO)
	if err != nil {
		t.Fatal(err)
	}
	defer respRO.Body.Close()
	var wrapRO struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(respRO.Body).Decode(&wrapRO); err != nil {
		t.Fatal(err)
	}
	wantRO := expectedToolNamesForTier(TierReadonly)
	if got := toolNamesFromHTTPList(wrapRO.Result.Tools); !equalStringSlices(got, wantRO) {
		t.Fatalf("readonly key tools mismatch: got %v want %v", got, wantRO)
	}

	reqEX, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	reqEX.Header.Set("Content-Type", "application/json")
	reqEX.Header.Set("Authorization", "Bearer ex")
	respEX, err := http.DefaultClient.Do(reqEX)
	if err != nil {
		t.Fatal(err)
	}
	defer respEX.Body.Close()
	var wrapEX struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(respEX.Body).Decode(&wrapEX); err != nil {
		t.Fatal(err)
	}
	wantEX := expectedToolNamesForTier(TierExec)
	if got := toolNamesFromHTTPList(wrapEX.Result.Tools); !equalStringSlices(got, wantEX) {
		t.Fatalf("exec key tools mismatch: got %v want %v", got, wantEX)
	}
}

func TestHTTPProviderPoolApprovalReadAndDecisionShareTheExactActiveChat(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	request := approvalFixtureRequest("http-request")
	providerResolved := make(chan struct{})
	var runnerCalls atomic.Int32
	server.providerPoolChatStreamRunner = func(ctx context.Context, _ []string, approvals *transientApprovalController) (string, string, int, error) {
		runnerCalls.Add(1)
		delivery, err := approvals.awaitDecision(ctx, request)
		if err != nil {
			return "", "", -1, err
		}
		delivery.complete(nil)
		select {
		case <-providerResolved:
			return `{"state":"ready"}`, "", 0, nil
		case <-ctx.Done():
			return "", "", -1, ctx.Err()
		}
	}
	srv := newIPv4TestServer(t, server.jsonRPCHandler(io.Discard))
	defer srv.Close()

	type callResult struct {
		response *rpcResponse
		err      error
	}
	chatDone := make(chan callResult, 1)
	go func() {
		response, err := callHTTPRPC(srv.URL, providerPoolChatFixture("30"))
		chatDone <- callResult{response: response, err: err}
	}()
	waitForApprovalRequest(t, server, request)

	readResponse, err := callHTTPRPC(srv.URL, approvalReadFixture("31"))
	if err != nil {
		t.Fatal(err)
	}
	if got := approvalRequestFromRPCResponse(t, readResponse); !reflect.DeepEqual(got, request) {
		t.Fatalf("HTTP approval request=%+v, want %+v", got, request)
	}

	second, err := callHTTPRPC(srv.URL, providerPoolChatFixture("32"))
	if err != nil {
		t.Fatal(err)
	}
	if second.Error == nil || !strings.Contains(second.Error.Message, "already active") || runnerCalls.Load() != 1 {
		t.Fatalf("second chat=%+v runner calls=%d", second, runnerCalls.Load())
	}

	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	decideResponse, err := callHTTPRPC(srv.URL, approvalDecisionFixture("33", decision))
	if err != nil || decideResponse.Error != nil {
		t.Fatalf("decision response=%+v err=%v", decideResponse, err)
	}
	ping, err := callHTTPRPC(srv.URL, `{"jsonrpc":"2.0","id":34,"method":"ping"}`)
	if err != nil || ping.Error != nil {
		t.Fatalf("ping response=%+v err=%v", ping, err)
	}
	select {
	case result := <-chatDone:
		t.Fatalf("chat completed before provider resolution: response=%+v err=%v", result.response, result.err)
	default:
	}
	close(providerResolved)
	select {
	case result := <-chatDone:
		if result.err != nil || result.response == nil || result.response.Error != nil {
			t.Fatalf("chat response=%+v err=%v", result.response, result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for HTTP chat completion")
	}
}

func TestHTTPOrdinaryProviderPoolChatRetainsBufferedRunner(t *testing.T) {
	enableMCPExecTier(t)
	server := NewServer("test")
	server.providerPoolChatRunner = func(context.Context, []string) (string, string, int, error) {
		return `{"state":"ready"}`, "", 0, nil
	}
	srv := newIPv4TestServer(t, server.jsonRPCHandler(io.Discard))
	defer srv.Close()
	response, err := callHTTPRPC(srv.URL, `{"jsonrpc":"2.0","id":35,"method":"tools/call","params":{"name":"dorkpipe.provider_pool_chat","arguments":{"message":"fixture","session_adapter":"codex_exec"}}}`)
	if err != nil || response == nil || response.Error != nil {
		t.Fatalf("ordinary HTTP chat response=%+v err=%v", response, err)
	}
}

func callHTTPRPC(url, payload string) (*rpcResponse, error) {
	response, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var rpc rpcResponse
	if err := json.NewDecoder(response.Body).Decode(&rpc); err != nil {
		return nil, err
	}
	return &rpc, nil
}

func approvalRequestFromRPCResponse(t *testing.T, response *rpcResponse) providersession.ApprovalRequest {
	t.Helper()
	if response == nil || response.Error != nil {
		t.Fatalf("approval response=%+v", response)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("approval result=%s", response.Result)
	}
	var request providersession.ApprovalRequest
	if err := json.Unmarshal([]byte(result.Content[0].Text), &request); err != nil {
		t.Fatal(err)
	}
	return request
}

func expectedToolNamesForTier(tier MCPTier) []string {
	ctx := WithMCPTier(context.Background(), tier)
	var out []string
	for _, m := range mcpToolCatalog() {
		if ToolAllowed(ctx, m.Name) {
			out = append(out, m.Name)
		}
	}
	return out
}

func toolNamesFromHTTPList(tools []struct {
	Name string `json:"name"`
}) []string {
	out := make([]string, 0, len(tools))
	for _, tool := range tools {
		out = append(out, tool.Name)
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newIPv4TestServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(h)
	srv.Listener = ln
	srv.Start()
	return srv
}

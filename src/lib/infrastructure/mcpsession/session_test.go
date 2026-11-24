package mcpsession

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fixtureInput struct {
	Value string `json:"value,omitempty"`
}

func TestMCPSessionFixture(t *testing.T) {
	if os.Getenv("DOCKPIPE_MCP_FIXTURE") != "1" {
		return
	}
	options := &mcp.ServerOptions{PageSize: 2}
	if os.Getenv("DOCKPIPE_MCP_LEGACY") == "1" {
		options.SupportedProtocolVersions = []string{"2025-11-25"}
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "neutral-fixture", Version: "1"}, options)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, request)
			if listed, ok := result.(*mcp.ListToolsResult); ok {
				listed.TTLMs = 60000
			}
			return result, err
		}
	})
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "Return exact value"}, func(ctx context.Context, request *mcp.CallToolRequest, args fixtureInput) (*mcp.CallToolResult, any, error) {
		if os.Getenv("DOCKPIPE_TEST_SECRET") != "" {
			return nil, nil, fmt.Errorf("inherited secret")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: args.Value}}}, nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "block"}, func(ctx context.Context, request *mcp.CallToolRequest, args fixtureInput) (*mcp.CallToolResult, any, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	})
	mcp.AddTool(server, &mcp.Tool{Name: "crash"}, func(ctx context.Context, request *mcp.CallToolRequest, args fixtureInput) (*mcp.CallToolResult, any, error) {
		os.Exit(9)
		return nil, nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "change"}, func(ctx context.Context, request *mcp.CallToolRequest, args fixtureInput) (*mcp.CallToolResult, any, error) {
		mcp.AddTool(server, &mcp.Tool{Name: "new-tool"}, func(context.Context, *mcp.CallToolRequest, fixtureInput) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{}, nil, nil
		})
		return &mcp.CallToolResult{}, nil, nil
	})
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func fixtureCommand(t *testing.T) CommandFactory {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	return func(ctx context.Context) (*exec.Cmd, error) {
		command := exec.CommandContext(ctx, executable, "-test.run=^TestMCPSessionFixture$")
		command.Dir = directory
		command.Env = []string{"DOCKPIPE_MCP_FIXTURE=1"}
		// Windows needs the OS directory; this is an explicit non-secret runtime
		// prerequisite, not inheritance of the invoking environment.
		if root := os.Getenv("SystemRoot"); root != "" {
			command.Env = append(command.Env, "SystemRoot="+root)
		}
		return command, nil
	}
}

func startFixture(t *testing.T) (*Session, Discovery) {
	t.Helper()
	session, err := Start(context.Background(), fixtureCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Stop(); err != nil {
			t.Error(err)
		}
	})
	if session.client.InitializeResult().ProtocolVersion != "2026-07-28" {
		t.Fatal("current protocol was not negotiated")
	}
	discovery, err := session.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Tools) != 4 || discovery.Tools[0].Name != "block" || discovery.Tools[3].Name != "echo" || len(discovery.SHA256) != 64 {
		t.Fatalf("unexpected discovery: %+v", discovery)
	}
	return session, discovery
}

func TestMCPSessionSemanticCallsAndApproval(t *testing.T) {
	t.Setenv("DOCKPIPE_TEST_SECRET", "must-not-reach-worker")
	session, discovery := startFixture(t)
	result, err := session.Call(context.Background(), discovery.SHA256, "echo", json.RawMessage(`{"value":"exact semantic result"}`))
	if err != nil || result.IsError || len(result.Content) != 1 || result.Content[0].(*mcp.TextContent).Text != "exact semantic result" {
		t.Fatalf("result %+v error %v", result, err)
	}
	for _, test := range []struct{ digest, name, args string }{
		{"", "echo", `{}`}, {strings.Repeat("0", 64), "echo", `{}`}, {discovery.SHA256, "missing", `{}`}, {discovery.SHA256, "echo", `null`}, {discovery.SHA256, "echo", `[]`},
	} {
		if _, err := session.Call(context.Background(), test.digest, test.name, json.RawMessage(test.args)); err == nil {
			t.Fatalf("accepted %+v", test)
		}
	}
	if _, err := session.Call(context.Background(), discovery.SHA256, "change", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Call(context.Background(), discovery.SHA256, "echo", json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "fresh review") {
		t.Fatalf("stale review accepted: %v", err)
	}
}

func TestMCPSessionCancellationCrashAndExplicitRestart(t *testing.T) {
	session, discovery := startFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := session.Call(ctx, discovery.SHA256, "block", json.RawMessage(`{}`)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := session.Call(context.Background(), discovery.SHA256, "echo", json.RawMessage(`{"value":"after cancel"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Call(context.Background(), discovery.SHA256, "crash", json.RawMessage(`{}`)); err == nil {
		t.Fatal("crash returned success")
	}
	if _, err := session.Discover(context.Background()); err == nil {
		t.Fatal("crash implicitly restarted")
	}
	// Restart is an explicit fresh Start through the caller's command factory;
	// no prior invocation or approval is replayed by the owner.
	replacement, fresh := startFixture(t)
	if _, err := replacement.Call(context.Background(), fresh.SHA256, "echo", json.RawMessage(`{"value":"new process"}`)); err != nil {
		t.Fatal(err)
	}
	if err := replacement.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := replacement.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := replacement.Discover(context.Background()); err == nil {
		t.Fatal("stopped session usable")
	}
}

func TestMCPSessionRejectsUnpreparedCommand(t *testing.T) {
	for _, factory := range []CommandFactory{nil, func(context.Context) (*exec.Cmd, error) { return nil, nil }, func(ctx context.Context) (*exec.Cmd, error) { return exec.CommandContext(ctx, "unqualified"), nil }} {
		if _, err := Start(context.Background(), factory); err == nil {
			t.Fatal("accepted unprepared command")
		}
	}
	factory := fixtureCommand(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Start(ctx, factory); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancel: %v", err)
	}
	session, _ := startFixture(t)
	session.operation <- struct{}{}
	if _, err := session.Discover(context.Background()); err == nil {
		t.Fatal("unbounded concurrency")
	}
	<-session.operation
}

func TestMCPSessionRetainedCacheBound(t *testing.T) {
	middleware := boundedToolCache()
	oversized := false
	handler := middleware(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		description := "small"
		if oversized {
			description = strings.Repeat("x", maxDiscovery)
		}
		return &mcp.ListToolsResult{Tools: []*mcp.Tool{{Name: "echo", Description: description}}}, nil
	})
	for index := 0; index < maxTools; index++ {
		result, err := handler(context.Background(), "tools/list", &mcp.ClientRequest[*mcp.ListToolsParams]{Params: &mcp.ListToolsParams{Cursor: fmt.Sprint(index)}})
		if err != nil || result.(*mcp.ListToolsResult).TTLMs != 0 {
			t.Fatalf("cache slot %d: %v", index, err)
		}
	}
	if _, err := handler(context.Background(), "tools/list", &mcp.ClientRequest[*mcp.ListToolsParams]{Params: &mcp.ListToolsParams{Cursor: "new"}}); err == nil {
		t.Fatal("unbounded distinct cursor cache")
	}
	oversized = true
	if _, err := handler(context.Background(), "tools/list", &mcp.ClientRequest[*mcp.ListToolsParams]{Params: &mcp.ListToolsParams{Cursor: "0"}}); err == nil {
		t.Fatal("unbounded metadata bytes")
	}
}

func TestMCPSessionLegacyNegotiationAndStartupFailure(t *testing.T) {
	prepare := fixtureCommand(t)
	session, err := Start(context.Background(), func(ctx context.Context) (*exec.Cmd, error) {
		command, err := prepare(ctx)
		command.Env = append(command.Env, "DOCKPIPE_MCP_LEGACY=1")
		return command, err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Stop()
	if session.client.InitializeResult().ProtocolVersion != "2025-11-25" {
		t.Fatal("legacy protocol was not negotiated")
	}
	discovery, err := session.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.Call(context.Background(), discovery.SHA256, "echo", json.RawMessage(`{"value":"legacy exact result"}`))
	if err != nil || result.IsError || result.Content[0].(*mcp.TextContent).Text != "legacy exact result" {
		t.Fatalf("legacy call: %+v %v", result, err)
	}
	_, err = Start(context.Background(), func(ctx context.Context) (*exec.Cmd, error) {
		command, err := prepare(ctx)
		command.Path = command.Path + "-does-not-exist"
		return command, err
	})
	if err == nil {
		t.Fatal("failed startup accepted")
	}
}

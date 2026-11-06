package mcpimport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	mcpmodel "dockpipe/src/lib/model/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func selected(t *testing.T) Selection {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Linux container import host-path mapping is not implemented on Windows")
	}
	root := t.TempDir()
	runtime := domain.CompiledRuntimeManifest{Schema: 2, Kind: domain.RuntimeManifestKind, RuntimeProfile: "dockerimage", PolicyProfile: "secure-default", Security: domain.CompiledSecurityPolicy{
		Preset: "secure-default", Network: domain.CompiledNetworkPolicy{Mode: "offline", Enforcement: "native"},
		FS:      domain.CompiledFilesystemPolicy{Root: "readonly", Writes: "workspace-only", TempPaths: []string{"/tmp"}},
		Process: domain.CompiledProcessPolicy{User: "non-root", NoNewPrivileges: true, DropCaps: []string{"ALL"}, PIDLimit: 64},
	}, Image: domain.CompiledImageSelection{Source: "registry", Ref: "example.invalid/runtime@sha256:" + strings.Repeat("1", 64), ExpectedDigest: "sha256:" + strings.Repeat("1", 64), PullPolicy: "never"}}
	runtime.PolicyFingerprint, _ = domain.FingerprintJSON(runtime.Security)
	return Selection{Name: "neutral", Workspace: root, ArtifactSHA256: strings.Repeat("2", 64), Runtime: runtime, Server: mcpmodel.Server{Transport: "stdio", Local: &mcpmodel.Local{Executable: "/bundle/bin/neutral", WorkingDirectory: root, Runtime: "dockerimage", Platform: "linux/amd64"}}}
}

func trusted(_ context.Context, request Request) (Decision, error) {
	digest, err := domain.FingerprintJSON(request)
	return Decision{RequestSHA256: digest, Principal: "test-human", ExpiresAt: time.Now().Add(time.Minute)}, err
}

func TestImportPlanFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Selection)
	}{
		{"host", func(s *Selection) { s.Server.Local.Runtime = "host" }},
		{"different runtime", func(s *Selection) { s.Runtime.RuntimeProfile = "vm" }},
		{"mutable image", func(s *Selection) { s.Runtime.Image.Ref = "example.invalid/runtime:latest" }},
		{"pull", func(s *Selection) { s.Runtime.Image.PullPolicy = "always" }},
		{"build", func(s *Selection) { s.Runtime.Image.AutoBuild = "always" }},
		{"stale policy", func(s *Selection) { s.Runtime.PolicyFingerprint = strings.Repeat("0", 64) }},
		{"network", func(s *Selection) { s.Runtime.Security.Network.Mode = "internet" }},
		{"root", func(s *Selection) { s.Runtime.Security.Process.User = "root" }},
		{"secret", func(s *Selection) { s.Server.Local.SecretEnvironment = "secret" }},
		{"workspace escape", func(s *Selection) { s.Server.Local.WorkingDirectory = t.TempDir() }},
		{"artifact", func(s *Selection) { s.ArtifactSHA256 = "latest" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			selection := selected(t)
			test.change(&selection)
			if _, err := Compile(selection); err == nil {
				t.Fatal("unsafe plan accepted")
			}
		})
	}
	plan, err := Compile(selected(t))
	if err != nil {
		t.Fatal(err)
	}
	called := false
	executor := func(context.Context, Selection, infrastructure.RunOpts) (*Lease, error) {
		called = true
		return nil, nil
	}
	for _, authorizer := range []Authorizer{nil, func(context.Context, Request) (Decision, error) { return Decision{}, nil }, func(ctx context.Context, r Request) (Decision, error) {
		d, e := trusted(ctx, r)
		d.ExpiresAt = time.Now().Add(-time.Second)
		return d, e
	}, func(ctx context.Context, r Request) (Decision, error) {
		d, e := trusted(ctx, r)
		d.RequestSHA256 = strings.Repeat("0", 64)
		return d, e
	}} {
		if _, err := plan.Start(context.Background(), authorizer, executor); err == nil {
			t.Fatal("invalid authority accepted")
		}
	}
	if called {
		t.Fatal("executor called before authorization")
	}
}

func TestImportAuthorizationBindsEveryInput(t *testing.T) {
	selection := selected(t)
	plan, err := Compile(selection)
	if err != nil {
		t.Fatal(err)
	}
	var saved Request
	capture := func(ctx context.Context, r Request) (Decision, error) { saved = r; return trusted(ctx, r) }
	if err := plan.authorize(context.Background(), capture, "call", "echo", strings.Repeat("3", 64), json.RawMessage(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}
	replay := func(ctx context.Context, r Request) (Decision, error) { return trusted(ctx, saved) }
	for _, args := range []struct{ op, tool, metadata, body string }{{"call", "other", strings.Repeat("3", 64), `{"x":1}`}, {"call", "echo", strings.Repeat("4", 64), `{"x":1}`}, {"call", "echo", strings.Repeat("3", 64), `{"x":2}`}, {"discover", "", "", `{}`}} {
		if err := plan.authorize(context.Background(), replay, args.op, args.tool, args.metadata, json.RawMessage(args.body)); err == nil {
			t.Fatal("authorization replay accepted")
		}
	}
	selection.ArtifactSHA256 = strings.Repeat("5", 64)
	changed, err := Compile(selection)
	if err != nil {
		t.Fatal(err)
	}
	if err := changed.authorize(context.Background(), replay, "call", "echo", strings.Repeat("3", 64), json.RawMessage(`{"x":1}`)); err == nil {
		t.Fatal("cross-artifact authorization accepted")
	}
}

func TestImportProtocolFixture(t *testing.T) {
	if os.Getenv("DOCKPIPE_IMPORT_FIXTURE") != "1" {
		return
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "neutral", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo"}, func(ctx context.Context, r *mcp.CallToolRequest, args struct {
		Value string `json:"value"`
	}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: args.Value}}}, nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "crash"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		os.Exit(8)
		return nil, nil, nil
	})
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// This test executor is a neutral host fixture only. It verifies ownership and
// authorization wiring; it provides no evidence of Docker containment/cleanup.
func fixtureExecutor(t *testing.T, closes *atomic.Int32, fail bool) Executor {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, s Selection, o infrastructure.RunOpts) (*Lease, error) {
		if o.NetworkMode != "none" || !o.ReadOnlyRootFS || len(o.CapDrop) != 1 || o.CapDrop[0] != "ALL" || o.PIDLimit != 64 || o.Image != s.Runtime.Image.Ref || o.WorkdirHost != s.Workspace {
			t.Fatal("compiled policy not applied")
		}
		return &Lease{Prepare: func(ctx context.Context) (*exec.Cmd, error) {
			if fail {
				return nil, errors.New("fixture startup failed")
			}
			command := exec.CommandContext(ctx, binary, "-test.run=^TestImportProtocolFixture$")
			command.Dir = s.Workspace
			command.Env = []string{"DOCKPIPE_IMPORT_FIXTURE=1"}
			if root := os.Getenv("SystemRoot"); root != "" {
				command.Env = append(command.Env, "SystemRoot="+root)
			}
			return command, nil
		}, Close: func(context.Context) error { closes.Add(1); return nil }}, nil
	}
}

func TestImportLeaseCleanupAndExplicitCalls(t *testing.T) {
	plan, err := Compile(selected(t))
	if err != nil {
		t.Fatal(err)
	}
	var closes atomic.Int32
	if _, err := plan.Start(context.Background(), trusted, fixtureExecutor(t, &closes, true)); err == nil || closes.Load() != 1 {
		t.Fatalf("startup cleanup: %v %d", err, closes.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := plan.Start(ctx, trusted, fixtureExecutor(t, &closes, false))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Stop()
	discovery, err := session.Discover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.Call(ctx, discovery.SHA256, "echo", json.RawMessage(`{"value":"authorized exact result"}`))
	if err != nil || result.IsError || result.Content[0].(*mcp.TextContent).Text != "authorized exact result" {
		t.Fatalf("call: %+v %v", result, err)
	}
	cancel()
	deadline := time.After(3 * time.Second)
	for closes.Load() != 2 {
		select {
		case <-deadline:
			t.Fatal("cancel did not release owned lease")
		case <-time.After(time.Millisecond):
		}
	}
	if err := session.Stop(); err != nil {
		t.Fatal(err)
	}
	if closes.Load() != 2 {
		t.Fatal("cleanup ran twice")
	}
}

func TestImportRechecksAuthorityAfterAcquisition(t *testing.T) {
	plan, err := Compile(selected(t))
	if err != nil {
		t.Fatal(err)
	}
	checks := 0
	authorizer := func(ctx context.Context, request Request) (Decision, error) {
		checks++
		decision, err := trusted(ctx, request)
		if checks == 2 {
			decision.ExpiresAt = time.Now().Add(-time.Second)
		}
		return decision, err
	}
	var closes atomic.Int32
	executor := fixtureExecutor(t, &closes, false)
	if _, err = plan.Start(context.Background(), authorizer, executor); err == nil || closes.Load() != 1 || checks != 2 {
		t.Fatalf("expired post-acquisition authority: %v closes=%d checks=%d", err, closes.Load(), checks)
	}
	failure := errors.New("runtime cleanup failed")
	executor = func(context.Context, Selection, infrastructure.RunOpts) (*Lease, error) {
		return &Lease{Close: func(context.Context) error { return failure }}, errors.New("partial acquisition")
	}
	if _, err = plan.Start(context.Background(), trusted, executor); !errors.Is(err, failure) {
		t.Fatalf("cleanup error lost: %v", err)
	}
}

func TestImportLeaseCleanupOnChildExit(t *testing.T) {
	plan, err := Compile(selected(t))
	if err != nil {
		t.Fatal(err)
	}
	var closes atomic.Int32
	session, err := plan.Start(context.Background(), trusted, fixtureExecutor(t, &closes, false))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Stop()
	discovery, err := session.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = session.Call(context.Background(), discovery.SHA256, "crash", json.RawMessage(`{}`)); err == nil {
		t.Fatal("crash returned success")
	}
	deadline := time.After(3 * time.Second)
	for closes.Load() != 1 {
		select {
		case <-deadline:
			t.Fatal("child exit did not release owned lease")
		case <-time.After(time.Millisecond):
		}
	}
}

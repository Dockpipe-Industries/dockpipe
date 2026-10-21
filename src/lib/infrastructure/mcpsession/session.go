// Package mcpsession owns a supervised stdio protocol connection. It does not
// resolve runtime profiles, authorize imports, install artifacts, or expose a
// gateway. An application must complete those checks before supplying a command.
package mcpsession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"dockpipe/src/lib/infrastructure/process"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// Match the SDK frame budget. Discovery also has an aggregate budget so a
	// paginated server cannot grow the client's retained state without bound.
	maxFrame        = 16 << 20
	maxDiscovery    = 8 << 20
	maxTools        = 1024
	startupTimeout  = 30 * time.Second
	requestTimeout  = 30 * time.Second
	shutdownTimeout = 3 * time.Second
)

// CommandFactory must prepare an already authorized runtime command using the
// supplied context. It must set an explicit environment (empty is permitted),
// absolute executable/cwd, and leave all stdio streams unset. No implicit host
// approval or environment inheritance is supplied by this package.
type CommandFactory func(context.Context) (*exec.Cmd, error)

// Session permits one operation at a time; concurrent submissions fail instead
// of accumulating an unbounded queue. Stop may interrupt an in-flight operation.
type Session struct {
	client    *mcp.ClientSession
	cancel    context.CancelFunc
	done      chan error
	exited    chan struct{}
	operation chan struct{}
	once      sync.Once
	stopped   chan struct{}
	stopErr   error
}

type Discovery struct {
	Tools  []*mcp.Tool `json:"tools"`
	SHA256 string      `json:"sha256"`
}

// Start owns the process until Stop or parent cancellation. It negotiates the
// current protocol with legacy fallback through the official SDK. Initialization
// cannot grant sampling, elicitation, filesystem roots, or upstream credentials.
func Start(ctx context.Context, prepare CommandFactory) (*Session, error) {
	if prepare == nil {
		return nil, fmt.Errorf("MCP requires an authorized command factory")
	}
	lifetime, cancel := context.WithCancel(ctx)
	command, err := prepare(lifetime)
	if err != nil {
		cancel()
		return nil, err
	}
	if command == nil || command.Cancel == nil || !filepath.IsAbs(command.Path) || !filepath.IsAbs(command.Dir) || command.Env == nil || command.Stdin != nil || command.Stdout != nil || command.Stderr != nil {
		cancel()
		return nil, fmt.Errorf("MCP requires a context-bound command, absolute executable/cwd, explicit environment and unassigned stdio")
	}
	if err := lifetime.Err(); err != nil {
		cancel()
		return nil, err
	}
	input, inputWriter, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, err
	}
	outputReader, output, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, errors.Join(err, input.Close(), inputWriter.Close())
	}
	command.Stdin, command.Stdout, command.Stderr = input, output, io.Discard
	command.WaitDelay = shutdownTimeout
	s := &Session{cancel: cancel, done: make(chan error, 1), exited: make(chan struct{}), operation: make(chan struct{}, 1), stopped: make(chan struct{})}
	type startResult struct {
		running  bool
		closeErr error
	}
	started := make(chan startResult, 1)
	go func() {
		runErr := process.RunObserved(command, func(cmd *exec.Cmd) func(error) {
			// After Start the child owns inherited descriptors. Close parent duplicates
			// even on startup failure, so EOF and cancellation cannot retain pipe owners.
			closeErr := errors.Join(input.Close(), output.Close())
			started <- startResult{cmd.Process != nil, closeErr}
			return func(error) {}
		})
		s.done <- runErr
		close(s.exited)
	}()
	select {
	case result := <-started:
		if !result.running || result.closeErr != nil {
			cancel()
			closeErr := errors.Join(inputWriter.Close(), outputReader.Close())
			return nil, errors.Join(<-s.done, result.closeErr, closeErr)
		}
	case <-lifetime.Done():
		cancel()
		return nil, errors.Join(lifetime.Err(), inputWriter.Close(), outputReader.Close())
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "dockpipe", Version: "1"}, &mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}})
	client.AddSendingMiddleware(boundedToolCache())
	initializing, finish := context.WithTimeout(lifetime, startupTimeout)
	s.client, err = client.Connect(initializing, &mcp.IOTransport{Reader: outputReader, Writer: inputWriter, MaxLineLength: maxFrame}, nil)
	finish()
	if err != nil {
		cancel()
		closeErr := errors.Join(inputWriter.Close(), outputReader.Close())
		select {
		case <-s.done:
		case <-time.After(shutdownTimeout):
		}
		return nil, fmt.Errorf("MCP initialization failed: %w", errors.Join(err, closeErr))
	}
	return s, nil
}

func (s *Session) acquire() error {
	select {
	case <-s.stopped:
		return fmt.Errorf("MCP session stopped")
	default:
	}
	select {
	case s.operation <- struct{}{}:
		return nil
	default:
		return fmt.Errorf("MCP session already has an operation in flight")
	}
}

// Discover returns complete, sorted tool metadata and a digest suitable for an
// application approval record. Metadata is untrusted data, never instructions.
func (s *Session) Discover(ctx context.Context) (Discovery, error) {
	if err := s.acquire(); err != nil {
		return Discovery{}, err
	}
	defer func() { <-s.operation }()
	bounded, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	return s.discover(bounded)
}

func (s *Session) discover(ctx context.Context) (Discovery, error) {
	result := Discovery{Tools: []*mcp.Tool{}}
	names := map[string]bool{}
	cursors := map[string]bool{}
	cursor := ""
	bytes := 0
	for page := 0; page < maxTools; page++ {
		response, err := s.client.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return Discovery{}, err
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			return Discovery{}, err
		}
		bytes += len(encoded)
		if bytes > maxDiscovery || len(result.Tools)+len(response.Tools) > maxTools {
			return Discovery{}, fmt.Errorf("MCP tool discovery exceeds 8 MiB or 1024 tools")
		}
		for _, tool := range response.Tools {
			if tool == nil || tool.Name == "" || names[tool.Name] {
				return Discovery{}, fmt.Errorf("MCP tool discovery contains invalid or duplicate names")
			}
			names[tool.Name] = true
			result.Tools = append(result.Tools, tool)
		}
		cursor = response.NextCursor
		if cursor == "" {
			sort.Slice(result.Tools, func(i, j int) bool { return result.Tools[i].Name < result.Tools[j].Name })
			encoded, err = json.Marshal(result.Tools)
			if err != nil {
				return Discovery{}, err
			}
			digest := sha256.Sum256(encoded)
			result.SHA256 = hex.EncodeToString(digest[:])
			return result, nil
		}
		if cursors[cursor] {
			return Discovery{}, fmt.Errorf("MCP tool discovery repeated a cursor")
		}
		cursors[cursor] = true
	}
	return Discovery{}, fmt.Errorf("MCP tool discovery exceeds 1024 pages")
}

// Call rechecks the complete tool contract against the caller's reviewed digest.
// No annotations confer authority, and a failed/uncertain call is never replayed.
// A timeout cancels the request; it does not prove the upstream operation stopped.
func (s *Session) Call(ctx context.Context, approvedDigest, name string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	return s.CallChecked(ctx, approvedDigest, name, arguments, nil)
}

// CallChecked invokes a trusted authorization check after fresh discovery and
// immediately before sending the operation. A nil check retains protocol-only
// behavior; application import owners must supply their authority boundary.
func (s *Session) CallChecked(ctx context.Context, approvedDigest, name string, arguments json.RawMessage, check func(context.Context) error) (*mcp.CallToolResult, error) {
	if len(approvedDigest) != 64 || name == "" || len(arguments) > maxDiscovery || !json.Valid(arguments) {
		return nil, fmt.Errorf("MCP call requires a reviewed discovery digest, tool name and bounded JSON arguments")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &object); err != nil || object == nil {
		return nil, fmt.Errorf("MCP tool arguments must be an object")
	}
	if err := s.acquire(); err != nil {
		return nil, err
	}
	defer func() { <-s.operation }()
	bounded, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	current, err := s.discover(bounded)
	if err != nil {
		return nil, err
	}
	if current.SHA256 != approvedDigest {
		return nil, fmt.Errorf("MCP tool contract changed; fresh review required")
	}
	found := false
	for _, tool := range current.Tools {
		if tool.Name == name {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("MCP tool is absent from reviewed discovery")
	}
	if check != nil {
		if err := check(bounded); err != nil {
			return nil, err
		}
	}
	return s.client.CallTool(bounded, &mcp.CallToolParams{Name: name, Arguments: arguments})
}

// Stop closes protocol pipes, then forces whole-tree cancellation if graceful
// exit exceeds the shutdown budget. It is idempotent and safe during a request.
func (s *Session) Stop() error {
	s.once.Do(func() {
		defer close(s.stopped)
		closed := make(chan error, 1)
		go func() { closed <- s.client.Close() }()
		select {
		case err := <-closed:
			if err != nil && !errors.Is(err, os.ErrClosed) {
				s.stopErr = err
			}
		case <-time.After(shutdownTimeout):
			s.cancel()
		}
		select {
		case <-s.done:
		case <-time.After(shutdownTimeout):
			s.cancel()
			select {
			case <-s.done:
			case <-time.After(shutdownTimeout):
				s.stopErr = fmt.Errorf("MCP process did not stop within shutdown budget")
			}
		}
		s.cancel()
	})
	return s.stopErr
}

// The SDK currently retains one list result per cursor, including zero-TTL
// results. Bound that lifetime cache as well as individual discovery operations,
// and override server TTL hints: schema approval requires fresh metadata.
func boundedToolCache() mcp.Middleware {
	var mu sync.Mutex
	sizes := map[string]int{}
	total := 0
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, request)
			if err != nil || method != "tools/list" {
				return result, err
			}
			listed, ok := result.(*mcp.ListToolsResult)
			params, paramsOK := request.GetParams().(*mcp.ListToolsParams)
			if !ok || !paramsOK {
				return nil, fmt.Errorf("invalid MCP list result")
			}
			listed.TTLMs = 0
			encoded, err := json.Marshal(listed)
			if err != nil {
				return nil, err
			}
			mu.Lock()
			defer mu.Unlock()
			old, exists := sizes[params.Cursor]
			updated := total - old + len(encoded)
			if updated > maxDiscovery || (!exists && len(sizes) >= maxTools) {
				return nil, fmt.Errorf("MCP retained tool metadata exceeds session budget; stop and review before reconnecting")
			}
			sizes[params.Cursor] = len(encoded)
			total = updated
			return result, nil
		}
	}
}

// Exited closes when the supervised process exits, including startup failure.
func (s *Session) Exited() <-chan struct{} { return s.exited }

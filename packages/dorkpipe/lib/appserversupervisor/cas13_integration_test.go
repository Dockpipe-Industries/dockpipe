//go:build cas13

package appserversupervisor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

// cas13Launcher uses the existing authenticated direct App Server command.
func cas13Launcher() HostLauncher {
	executable := os.Getenv("DOCKPIPE_CAS13_EXECUTABLE")
	if executable == "" {
		executable = "codex"
	}
	return HostLauncher{Executable: executable, Args: []string{"app-server", "--stdio"}}
}

// TestCAS13ControlledCodex is deliberately excluded from ordinary tests. It
// accepts only the CAS-11 direct child and writes no evidence: callers may
// retain only their redacted outcome classification outside this process.
func TestCAS13ControlledCodex(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) {
		t.Fatal("CAS-13 requires an absolute declared workspace")
	}
	if filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 workspace must be clean")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-13 clean shutdown failed: %v", err)
		}
		if s.ShutdownRecord().Forced {
			t.Error("CAS-13 clean shutdown required a forced kill")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-13 thread lifecycle failed: %v", err)
	}
	if _, err := s.startTurn(ctx, thread, policy, cas13Prompt(t)); err != nil {
		t.Fatalf("CAS-13 turn lifecycle failed: %s", cas13FailureClass(s))
	}
	if terminal := waitCAS13Terminal(ctx, s); terminal != "turn_completed" {
		t.Fatalf("CAS-13 normalized terminal = %s", terminal)
	}
}

func TestCAS13FailedTurnReconcilesThread(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 requires a clean absolute declared workspace")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-reconcile"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 reconciliation initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-13 reconciliation shutdown failed: %v", err)
		}
		if s.ShutdownRecord().Forced {
			t.Error("CAS-13 reconciliation shutdown required a forced kill")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-13 reconciliation thread lifecycle failed: %v", err)
	}
	if _, err := s.startTurn(ctx, thread, policy, cas13Prompt(t)); err != nil {
		t.Fatalf("CAS-13 reconciliation turn lifecycle failed: %s", cas13FailureClass(s))
	}
	if terminal := waitCAS13Terminal(ctx, s); terminal != "turn_failed_other" {
		t.Fatalf("CAS-13 reconciliation terminal = %s", terminal)
	}
	if _, err := s.ReadThread(ctx, thread, policy); err != nil {
		t.Fatalf("CAS-13 failed-turn thread read = %s", cas13FailureClass(s))
	}
}

// TestCAS13ControlledCancellation uses the same constrained no-tool turn as
// the completion probe, then requests the existing neutral user cancellation.
// It is successful only on the exact correlated interrupted terminal state.
func TestCAS13ControlledCancellation(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 requires a clean absolute declared workspace")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-cancel"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 cancellation initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-13 cancellation shutdown failed: %v", err)
		}
		if s.ShutdownRecord().Forced {
			t.Error("CAS-13 cancellation shutdown required a forced kill")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-13 cancellation thread lifecycle failed: %v", err)
	}
	turn, err := s.startTurn(ctx, thread, policy, cas13Prompt(t))
	if err != nil {
		t.Fatalf("CAS-13 cancellation turn lifecycle failed: %s", cas13FailureClass(s))
	}
	if outcome := waitCAS13ActiveItem(ctx, s); outcome != "item_started" {
		t.Fatalf("CAS-13 cancellation active item = %s", outcome)
	}
	if err := s.Cancel(ctx, cancellationIntent(turn)); err != nil {
		t.Fatalf("CAS-13 cancellation delivery failed: %s", cas13FailureClass(s))
	}
	if terminal := waitCAS13Cancellation(ctx, s); terminal != "cancelled" {
		t.Fatalf("CAS-13 cancellation terminal = %s", terminal)
	}
}

func TestCAS13ControlledApprovalDeny(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 requires a clean absolute declared workspace")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-approval"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 approval initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-13 approval shutdown failed: %v", err)
		}
		if s.ShutdownRecord().Forced {
			t.Error("CAS-13 approval shutdown required a forced kill")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-13 approval thread lifecycle failed: %v", err)
	}
	if _, err := s.startTurn(ctx, thread, policy, cas13Prompt(t)); err != nil {
		t.Fatalf("CAS-13 approval turn lifecycle failed: %s", cas13FailureClass(s))
	}
	for {
		select {
		case event := <-s.Events():
			if event.Kind == providersession.EventApprovalRequested && event.Approval != nil {
				if err := s.Decide(ctx, providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny}); err != nil {
					t.Fatalf("CAS-13 denial delivery failed: %s", cas13FailureClass(s))
				}
				if outcome := waitCAS13ApprovalResolution(ctx, s); outcome != string(DisconnectApprovalDenied) {
					t.Fatalf("CAS-13 denial resolution = %s", outcome)
				}
				return
			}
			if event.Kind == providersession.EventProgress && event.Summary == "turn_failed" {
				t.Fatal("CAS-13 approval turn failed before an approval request")
			}
			if event.State == providersession.StateDisconnected {
				s.mu.RLock()
				last := s.lastNotification
				s.mu.RUnlock()
				if last != "" && last != "other" {
					t.Fatalf("CAS-13 approval request = %s_%s", event.Summary, last)
				}
				t.Fatalf("CAS-13 approval request = %s", event.Summary)
			}
		case <-ctx.Done():
			t.Fatal("CAS-13 approval request did not arrive")
		}
	}
}

// TestCAS14ControlledUserInput is deliberately excluded from ordinary tests.
// It observes one exact response only in memory, retains only whether the
// intended answer was delivered, and never logs a provider payload.
func TestCAS14ControlledUserInput(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS14") != "run" {
		t.Skip("set DOCKPIPE_CAS14=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS14_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-14 requires a clean absolute declared workspace")
	}
	const intendedLabel = "Continue safely"
	observer := &cas14ResponseObserver{}
	launcher := &cas14ObservingLauncher{host: cas13Launcher(), observer: observer}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas14", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas14-user-input"}, launcher, deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-14 initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-14 clean shutdown failed: %v", err)
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-14 thread lifecycle failed: %v", err)
	}
	input := []any{map[string]any{"type": "text", "text": "Use the request_user_input tool exactly once now. Ask one single-choice question with exactly these two choices in this exact order: Stop safely; Continue safely. Do not call any other tool, do not modify files, and after the answer finish without repeating the question or answer."}}
	if _, err := s.startTurn(ctx, thread, policy, input); err != nil {
		t.Fatalf("CAS-14 turn lifecycle failed: %s", cas13FailureClass(s))
	}

	requestEvent := waitCAS14UserInputRequest(t, ctx, s)
	request := *requestEvent.UserInput
	prompt, err := s.UserInputPrompt(ctx, request)
	if err != nil {
		t.Fatal("CAS-14 normalized prompt was unavailable")
	}
	if err := prompt.ValidateFor(request); err != nil || prompt.Kind != providersession.UserInputPromptSelectOne || len(prompt.Options) != 2 || len(prompt.Summary) == 0 || len(prompt.Summary) > 512 {
		t.Fatal("CAS-14 normalized prompt contract changed")
	}
	if prompt.Correlation != request.Correlation || prompt.PromptRef != request.PromptRef || !cas14CompleteCorrelation(prompt.Correlation) {
		t.Fatal("CAS-14 normalized prompt correlation was incomplete")
	}
	selectedIndex := -1
	selectedRef := ""
	for index, option := range prompt.Options {
		if len(option.Label) == 0 || len(option.Label) > 128 || !strings.HasPrefix(option.OptionRef, "option-") || option.OptionRef == option.Label {
			t.Fatal("CAS-14 normalized option contract changed")
		}
		if option.Label == intendedLabel {
			selectedIndex, selectedRef = index, option.OptionRef
		}
	}
	if prompt.Options[0].OptionRef == prompt.Options[1].OptionRef || selectedIndex <= 0 || selectedRef == "" {
		t.Fatal("CAS-14 intended non-first option was unavailable")
	}
	observer.expect(intendedLabel)
	response := providersession.UserInputResponse{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef, SelectedOptionRefs: []string{selectedRef}}
	if err := s.RespondUserInput(ctx, response); err != nil {
		t.Fatal("CAS-14 user-input response was not delivered")
	}
	if !observer.exactAnswerObserved() {
		t.Fatal("CAS-14 private provider response did not contain the exact intended answer")
	}
	resolved := waitCAS14UserInputResolution(t, ctx, s)
	if resolved.Correlation != prompt.Correlation {
		t.Fatal("CAS-14 user-input resolution correlation changed")
	}
	if err := s.RespondUserInput(ctx, response); !errors.Is(err, ErrUserInputResponseUnavailable) {
		t.Fatal("CAS-14 replayed response was not rejected")
	}
	rejected := waitCAS14ReplayRejection(t, ctx, s)
	cas14AssertNoRetainedInput(t, s, []providersession.Event{requestEvent, resolved, rejected}, prompt)
}

// TestCAS14ControlledUserInputRequestDiagnostic performs one authenticated,
// no-write turn with the installed App Server's experimental API and
// default-mode user-input feature enabled. It sends no user-input response and
// reports only allow-listed structural classifications; provider content and
// frames are discarded in memory.
func TestCAS14ControlledUserInputRequestDiagnostic(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS14_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS14_DIAGNOSTIC=run for the reviewed request diagnostic")
	}
	workspace := os.Getenv("DOCKPIPE_CAS14_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-14 requires a clean absolute declared workspace")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	launcher := cas14RequestDiagnosticLauncher{host: cas13Launcher()}
	child, err := launcher.Start(ctx)
	if err != nil {
		t.Fatal("CAS-14 direct child could not start")
	}
	defer func() { _ = child.Stdin().Close(); _ = child.Kill(); _ = child.Wait() }()
	encoder := json.NewEncoder(child.Stdin())
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas14-diagnostic", "version": "0.1.0"}, "capabilities": map[string]any{"experimentalApi": true, "optOutNotificationMethods": []string{remoteControlStatusNotification}}}}); err != nil {
		t.Fatal("CAS-14 initialization request could not be sent")
	}
	if _, ok := cas13DiagnosticResponse(t, scanner, 1); !ok {
		t.Fatal("CAS-14 experimental initialization was not accepted")
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		t.Fatal("CAS-14 initialized notification could not be sent")
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	threadResult := cas13DiagnosticRequest(t, encoder, scanner, 2, "thread/start", policy.params())
	threadID, reason := projectThread(threadResult)
	clear(threadResult)
	if reason != "" {
		t.Fatal("CAS-14 diagnostic thread response was invalid")
	}
	turnParams := policy.params()
	turnParams["threadId"] = threadID
	turnParams["input"] = []any{map[string]any{"type": "text", "text": "Use the request_user_input tool exactly once now. Ask one single-choice question with exactly these two choices in this exact order: Stop safely; Continue safely. Do not call any other tool, do not modify files, and after the answer finish without repeating the question or answer."}}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "turn/start", "params": turnParams}); err != nil {
		t.Fatal("CAS-14 diagnostic turn request could not be sent")
	}

	terminalClass := "not_reached"
	requestMethodClass := "none"
	schemaShape := "none"
	autoResolutionClass := "not_observed"
	isOtherClass := "not_observed"
	isSecretClass := "not_observed"
	invoked := false
	frames := make(chan []byte)
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer close(frames)
		for scanner.Scan() {
			frame := append([]byte(nil), scanner.Bytes()...)
			select {
			case frames <- frame:
			case <-done:
				clear(frame)
				return
			}
		}
	}()
	for count := 0; count < 256; count++ {
		select {
		case frame, ok := <-frames:
			if !ok {
				t.Fatal("CAS-14 diagnostic transport closed before a request or terminal")
			}
			var envelope struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
				Error  json.RawMessage `json:"error"`
			}
			if json.Unmarshal(frame, &envelope) != nil {
				clear(frame)
				continue
			}
			if len(envelope.ID) != 0 && envelope.Method != "" {
				requestMethodClass = cas13RequestMethodClass(envelope.Method)
				invoked = requestMethodClass == "user_input"
				if invoked {
					classification := cas14UserInputRequestSchemaShape(envelope.Params)
					schemaShape = classification.Shape
					autoResolutionClass = classification.AutoResolution
					isOtherClass = classification.IsOther
					isSecretClass = classification.IsSecret
				}
				clear(frame)
				t.Logf("CAS-14 diagnostic: terminal_class=%s user_input_tool_advertised=true user_input_tool_invoked=%t request_method_class=%s schema_shape=%s auto_resolution=%s is_other=%s is_secret=%s", terminalClass, invoked, requestMethodClass, schemaShape, autoResolutionClass, isOtherClass, isSecretClass)
				return
			}
			if envelope.Method == "turn/completed" {
				terminalClass = cas14TerminalClass(envelope.Params)
				clear(frame)
				t.Logf("CAS-14 diagnostic: terminal_class=%s user_input_tool_advertised=true user_input_tool_invoked=false request_method_class=%s schema_shape=%s auto_resolution=%s is_other=%s is_secret=%s", terminalClass, requestMethodClass, schemaShape, autoResolutionClass, isOtherClass, isSecretClass)
				return
			}
			if len(envelope.ID) != 0 && len(envelope.Error) != 0 {
				terminalClass = "turn_request_rejected"
				clear(frame)
				t.Logf("CAS-14 diagnostic: terminal_class=%s user_input_tool_advertised=true user_input_tool_invoked=false request_method_class=%s schema_shape=%s auto_resolution=%s is_other=%s is_secret=%s", terminalClass, requestMethodClass, schemaShape, autoResolutionClass, isOtherClass, isSecretClass)
				return
			}
			clear(frame)
		case <-ctx.Done():
			t.Fatal("CAS-14 diagnostic deadline expired")
		}
	}
	t.Fatal("CAS-14 diagnostic exceeded the bounded frame count")
}

// TestCAS14ControlledUserInputPromptDiagnostic proves only that one live
// feature-enabled request reaches the existing supervisor parser and bounded
// prompt lookup. It sends no user-input response and retains no provider data.
func TestCAS14ControlledUserInputPromptDiagnostic(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS14_PROMPT_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS14_PROMPT_DIAGNOSTIC=run for the reviewed prompt diagnostic")
	}
	workspace := os.Getenv("DOCKPIPE_CAS14_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-14 requires a clean absolute declared workspace")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas14-prompt-diagnostic", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	observer := &cas14PromptDiagnosticObserver{}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas14-prompt-diagnostic"}, cas14ExperimentalInitializationLauncher{host: cas13Launcher(), observer: observer}, deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-14 prompt diagnostic initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-14 prompt diagnostic shutdown failed")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatal("CAS-14 prompt diagnostic thread lifecycle failed")
	}
	input := []any{map[string]any{"type": "text", "text": "Use the request_user_input tool exactly once now. Ask one single-choice question with exactly these two choices in this exact order: Stop safely; Continue safely. Do not call any other tool, do not modify files, and after the answer finish without repeating the question or answer."}}
	if _, err := s.startTurn(ctx, thread, policy, input); err != nil {
		t.Fatalf("CAS-14 prompt diagnostic turn lifecycle failed: %s", cas13FailureClass(s))
	}
	requestEvent, terminalClass, disconnectClass := waitCAS14PromptDiagnostic(t, ctx, s)
	if terminalClass != "" {
		classification := observer.classification()
		t.Logf("CAS-14 prompt diagnostic: terminal_class=%s disconnect_class=%s pre_prompt_class=%s user_input_tool_advertised=true user_input_tool_invoked=%t request_method_class=%s schema_shape=%s request_compatibility=%s parser_class=not_reached prompt_lookup=unavailable response_sent=false", terminalClass, disconnectClass, classification.PrePromptClass, classification.UserInputInvoked, classification.RequestMethodClass, classification.SchemaShape, classification.RequestCompatibility)
		return
	}
	request := *requestEvent.UserInput
	prompt, err := s.UserInputPrompt(ctx, request)
	if err != nil {
		t.Fatal("CAS-14 live normalized prompt lookup was unavailable")
	}
	if err := prompt.ValidateFor(request); err != nil || prompt.Kind != providersession.UserInputPromptSelectOne || len(prompt.Options) != 2 || len(prompt.Summary) == 0 || len(prompt.Summary) > 512 {
		t.Fatal("CAS-14 live normalized prompt contract changed")
	}
	if prompt.Correlation != request.Correlation || prompt.PromptRef != request.PromptRef || !cas14CompleteCorrelation(prompt.Correlation) {
		t.Fatal("CAS-14 live normalized prompt correlation was incomplete")
	}
	for _, option := range prompt.Options {
		if len(option.Label) == 0 || len(option.Label) > 128 || !strings.HasPrefix(option.OptionRef, "option-") || option.OptionRef == option.Label {
			t.Fatal("CAS-14 live normalized option contract changed")
		}
	}
	if prompt.Options[0].OptionRef == prompt.Options[1].OptionRef {
		t.Fatal("CAS-14 live normalized option references were not unique")
	}
	cas14AssertNoRetainedInput(t, s, []providersession.Event{requestEvent}, prompt)
	t.Log("CAS-14 prompt diagnostic: terminal_class=not_reached request_class=user_input parser_class=accepted prompt_lookup=available prompt_kind=select_one option_count=two correlation_class=complete response_sent=false")
}

type cas14RequestDiagnosticLauncher struct {
	host HostLauncher
}

func (l cas14RequestDiagnosticLauncher) validateLaunch() error {
	return l.host.validateLaunch()
}

func (l cas14RequestDiagnosticLauncher) Start(ctx context.Context) (Child, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := l.validateLaunch(); err != nil {
		return nil, err
	}
	cmd := exec.Command(l.host.Executable, "app-server", "--enable", "default_mode_request_user_input", "--stdio")
	cmd.WaitDelay = hostPipeWaitDelay
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	return commandChild{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

type cas14ExperimentalInitializationLauncher struct {
	host     HostLauncher
	observer *cas14PromptDiagnosticObserver
}

func (l cas14ExperimentalInitializationLauncher) validateLaunch() error {
	if l.observer == nil {
		return errors.New("CAS-14 diagnostic observer is required")
	}
	return l.host.validateLaunch()
}

func (l cas14ExperimentalInitializationLauncher) Start(ctx context.Context) (Child, error) {
	child, err := (cas14RequestDiagnosticLauncher{host: l.host}).Start(ctx)
	if err != nil {
		return nil, err
	}
	return &cas14ExperimentalInitializationChild{
		Child:  child,
		stdin:  &cas14ExperimentalInitializationWriter{WriteCloser: child.Stdin()},
		stdout: &cas14PromptDiagnosticReader{ReadCloser: child.Stdout(), observer: l.observer},
	}, nil
}

type cas14ExperimentalInitializationChild struct {
	Child
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (c *cas14ExperimentalInitializationChild) Stdin() io.WriteCloser { return c.stdin }
func (c *cas14ExperimentalInitializationChild) Stdout() io.ReadCloser { return c.stdout }

type cas14ExperimentalInitializationWriter struct {
	io.WriteCloser
	mu          sync.Mutex
	buffer      []byte
	initialized bool
}

func (w *cas14ExperimentalInitializationWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buffer)+len(value) > 1<<20 {
		clear(w.buffer)
		w.buffer = nil
		return 0, errors.New("CAS-14 diagnostic initialize frame exceeded its bound")
	}
	w.buffer = append(w.buffer, value...)
	for {
		end := bytes.IndexByte(w.buffer, '\n')
		if end < 0 {
			break
		}
		frame := append([]byte(nil), w.buffer[:end]...)
		w.buffer = append(w.buffer[:0], w.buffer[end+1:]...)
		payload := frame
		if !w.initialized {
			var err error
			payload, err = cas14ExperimentalInitializeFrame(frame)
			clear(frame)
			if err != nil {
				clear(payload)
				return 0, err
			}
			w.initialized = true
		}
		wire := make([]byte, len(payload)+1)
		copy(wire, payload)
		wire[len(wire)-1] = '\n'
		clear(payload)
		remaining := wire
		for len(remaining) > 0 {
			written, err := w.WriteCloser.Write(remaining)
			if err != nil || written <= 0 {
				clear(wire)
				if err != nil {
					return 0, err
				}
				return 0, io.ErrShortWrite
			}
			remaining = remaining[written:]
		}
		clear(wire)
	}
	return len(value), nil
}

func (w *cas14ExperimentalInitializationWriter) Close() error {
	w.mu.Lock()
	clear(w.buffer)
	w.buffer = nil
	w.mu.Unlock()
	return w.WriteCloser.Close()
}

func cas14ExperimentalInitializeFrame(frame []byte) ([]byte, error) {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(frame, &envelope) != nil || envelope == nil {
		return nil, errors.New("CAS-14 diagnostic initialize envelope changed")
	}
	var method string
	if json.Unmarshal(envelope["method"], &method) != nil || method != "initialize" {
		return nil, errors.New("CAS-14 diagnostic first request was not initialize")
	}
	var params map[string]json.RawMessage
	if json.Unmarshal(envelope["params"], &params) != nil || params == nil {
		return nil, errors.New("CAS-14 diagnostic initialize params changed")
	}
	var capabilities map[string]json.RawMessage
	if json.Unmarshal(params["capabilities"], &capabilities) != nil || capabilities == nil {
		return nil, errors.New("CAS-14 diagnostic initialize capabilities changed")
	}
	if _, exists := capabilities["experimentalApi"]; exists {
		return nil, errors.New("CAS-14 production initialize unexpectedly advertised the experimental API")
	}
	capabilities["experimentalApi"] = json.RawMessage("true")
	encodedCapabilities, err := json.Marshal(capabilities)
	if err != nil {
		return nil, errors.New("CAS-14 diagnostic capabilities could not be encoded")
	}
	params["capabilities"] = encodedCapabilities
	encodedParams, err := json.Marshal(params)
	clear(encodedCapabilities)
	if err != nil {
		return nil, errors.New("CAS-14 diagnostic initialize params could not be encoded")
	}
	envelope["params"] = encodedParams
	encodedEnvelope, err := json.Marshal(envelope)
	clear(encodedParams)
	if err != nil {
		return nil, errors.New("CAS-14 diagnostic initialize envelope could not be encoded")
	}
	return encodedEnvelope, nil
}

func TestCAS14ExperimentalInitializeFrame(t *testing.T) {
	frame := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"diagnostic","version":"0.1.0"},"capabilities":{"optOutNotificationMethods":["safe"]}}}`)
	rewritten, err := cas14ExperimentalInitializeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(rewritten)
	var envelope struct {
		Method string `json:"method"`
		Params struct {
			Capabilities struct {
				ExperimentalAPI           bool     `json:"experimentalApi"`
				OptOutNotificationMethods []string `json:"optOutNotificationMethods"`
			} `json:"capabilities"`
		} `json:"params"`
	}
	if json.Unmarshal(rewritten, &envelope) != nil || envelope.Method != "initialize" || !envelope.Params.Capabilities.ExperimentalAPI || len(envelope.Params.Capabilities.OptOutNotificationMethods) != 1 {
		t.Fatal("CAS-14 experimental initialize shim changed its bounded shape")
	}
}

type cas14PromptDiagnosticClassification struct {
	PrePromptClass       string `json:"pre_prompt_class"`
	UserInputInvoked     bool   `json:"user_input_invoked"`
	RequestMethodClass   string `json:"request_method_class"`
	SchemaShape          string `json:"schema_shape"`
	RequestCompatibility string `json:"request_compatibility"`
}

type cas14PromptDiagnosticObserver struct {
	mu                   sync.Mutex
	eventClass           string
	userInputInvoked     bool
	requestMethodClass   string
	schemaShape          string
	requestCompatibility string
	invalid              bool
}

func (o *cas14PromptDiagnosticObserver) inspect(frame []byte) {
	var envelope struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if json.Unmarshal(frame, &envelope) != nil {
		o.invalidate()
		return
	}
	if envelope.Method == "" {
		if len(envelope.ID) == 0 {
			o.invalidate()
		}
		return
	}
	if len(envelope.ID) == 0 {
		o.observeNotification(envelope.Method, envelope.Params)
		return
	}
	if envelope.Method == "item/tool/requestUserInput" {
		shape := cas14UserInputRequestSchemaShape(envelope.Params).Shape
		compatibility := cas14PromptRequestCompatibility(envelope.Params)
		o.mu.Lock()
		if !o.invalid {
			o.userInputInvoked = true
			o.requestMethodClass = "user_input"
			o.schemaShape = shape
			o.requestCompatibility = compatibility
		}
		o.mu.Unlock()
		return
	}
	o.mu.Lock()
	if !o.invalid && !o.userInputInvoked {
		o.requestMethodClass = cas13RequestMethodClass(envelope.Method)
	}
	o.mu.Unlock()
}

func (o *cas14PromptDiagnosticObserver) observeNotification(method string, params json.RawMessage) {
	classification := ""
	if notificationFields(method) == nil {
		classification = "event_method_incompatible"
	} else if len(params) == 0 || !json.Valid(params) || !notificationShapeAllowed(method, params) {
		classification = "event_shape_incompatible"
	}
	if classification == "" {
		return
	}
	o.mu.Lock()
	if !o.invalid && o.eventClass == "" && !o.userInputInvoked {
		o.eventClass = classification
	}
	o.mu.Unlock()
}

func (o *cas14PromptDiagnosticObserver) invalidate() {
	o.mu.Lock()
	o.invalid = true
	o.eventClass = "frame_shape_incompatible"
	o.userInputInvoked = false
	o.requestMethodClass = "none_observed"
	o.schemaShape = "none_observed"
	o.requestCompatibility = "not_observed"
	o.mu.Unlock()
}

func (o *cas14PromptDiagnosticObserver) classification() cas14PromptDiagnosticClassification {
	o.mu.Lock()
	defer o.mu.Unlock()
	classification := cas14PromptDiagnosticClassification{
		PrePromptClass:       "unclassified",
		UserInputInvoked:     o.userInputInvoked,
		RequestMethodClass:   o.requestMethodClass,
		SchemaShape:          o.schemaShape,
		RequestCompatibility: o.requestCompatibility,
	}
	if classification.RequestMethodClass == "" {
		classification.RequestMethodClass = "none_observed"
	}
	if classification.SchemaShape == "" {
		classification.SchemaShape = "none_observed"
	}
	if classification.RequestCompatibility == "" {
		classification.RequestCompatibility = "not_observed"
	}
	switch {
	case o.invalid:
		classification.PrePromptClass = "frame_shape_incompatible"
	case o.requestCompatibility != "":
		classification.PrePromptClass = o.requestCompatibility
	case o.eventClass != "":
		classification.PrePromptClass = o.eventClass
	}
	return classification
}

func cas14PromptRequestCompatibility(raw json.RawMessage) string {
	var params map[string]json.RawMessage
	if json.Unmarshal(raw, &params) != nil || params == nil {
		return "request_shape_incompatible"
	}
	if value, present := params["autoResolutionMs"]; present && cas14ExperimentalFieldCompatibility("autoResolutionMs", value) != "safely_ignorable_default" {
		return "default_only_request_incompatible"
	}
	var questions []json.RawMessage
	if json.Unmarshal(params["questions"], &questions) != nil || len(questions) != 1 {
		return "request_shape_incompatible"
	}
	var question map[string]json.RawMessage
	if json.Unmarshal(questions[0], &question) != nil || question == nil {
		return "request_shape_incompatible"
	}
	for _, field := range []string{"isOther", "isSecret"} {
		if value, present := question[field]; present && cas14ExperimentalFieldCompatibility(field, value) != "safely_ignorable_default" {
			return "default_only_request_incompatible"
		}
	}
	var parsed serverRequestParams
	if !serverRequestShapeAllowed("item/tool/requestUserInput", raw) || json.Unmarshal(raw, &parsed) != nil {
		return "request_shape_incompatible"
	}
	if _, ok := parseProviderUserInputQuestion(parsed.Questions); !ok {
		return "request_shape_incompatible"
	}
	return "default_only_request_compatible"
}

type cas14PromptDiagnosticReader struct {
	io.ReadCloser
	observer *cas14PromptDiagnosticObserver
	mu       sync.Mutex
	buffer   []byte
}

func (r *cas14PromptDiagnosticReader) Read(value []byte) (int, error) {
	written, err := r.ReadCloser.Read(value)
	if written > 0 {
		r.mu.Lock()
		if len(r.buffer)+written > 1<<20 {
			clear(r.buffer)
			r.buffer = nil
			r.observer.invalidate()
		} else {
			r.buffer = append(r.buffer, value[:written]...)
			r.inspectFramesLocked(false)
		}
		r.mu.Unlock()
	}
	if errors.Is(err, io.EOF) {
		r.mu.Lock()
		r.inspectFramesLocked(true)
		r.mu.Unlock()
	}
	return written, err
}

func (r *cas14PromptDiagnosticReader) inspectFramesLocked(atEOF bool) {
	for {
		end := bytes.IndexByte(r.buffer, '\n')
		if end < 0 {
			if !atEOF || len(r.buffer) == 0 {
				return
			}
			end = len(r.buffer)
		}
		frame := append([]byte(nil), r.buffer[:end]...)
		if end == len(r.buffer) {
			clear(r.buffer)
			r.buffer = nil
		} else {
			r.buffer = append(r.buffer[:0], r.buffer[end+1:]...)
		}
		r.observer.inspect(frame)
		clear(frame)
	}
}

func (r *cas14PromptDiagnosticReader) Close() error {
	r.mu.Lock()
	clear(r.buffer)
	r.buffer = nil
	r.mu.Unlock()
	return r.ReadCloser.Close()
}

func TestCAS14PromptDiagnosticClassifierIsBoundedAndContentFree(t *testing.T) {
	compatibleRequest := `{"jsonrpc":"2.0","id":41,"method":"item/tool/requestUserInput","params":{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","autoResolutionMs":null,"questions":[{"id":"private-question","header":"private header","question":"private question","isOther":false,"isSecret":false,"options":[{"label":"private one","description":"private first"},{"label":"private two","description":"private second"}]}]}}`
	fixtures := []struct {
		name      string
		frames    string
		want      cas14PromptDiagnosticClassification
		forbidden string
	}{
		{
			name:   "default_only_request_compatible",
			frames: `{"jsonrpc":"2.0","id":1,"result":{}}` + "\n" + compatibleRequest + "\n",
			want:   cas14PromptDiagnosticClassification{PrePromptClass: "default_only_request_compatible", UserInputInvoked: true, RequestMethodClass: "user_input", SchemaShape: "single_select", RequestCompatibility: "default_only_request_compatible"},
		},
		{
			name:   "default_only_request_incompatible",
			frames: strings.Replace(compatibleRequest, `"autoResolutionMs":null`, `"autoResolutionMs":1000`, 1) + "\n",
			want:   cas14PromptDiagnosticClassification{PrePromptClass: "default_only_request_incompatible", UserInputInvoked: true, RequestMethodClass: "user_input", SchemaShape: "single_select", RequestCompatibility: "default_only_request_incompatible"},
		},
		{
			name:   "request_shape_incompatible",
			frames: strings.Replace(compatibleRequest, `"isSecret":false`, `"isSecret":false,"futureField":false`, 1) + "\n",
			want:   cas14PromptDiagnosticClassification{PrePromptClass: "request_shape_incompatible", UserInputInvoked: true, RequestMethodClass: "user_input", SchemaShape: "single_select", RequestCompatibility: "request_shape_incompatible"},
		},
		{
			name:   "event_method_incompatible",
			frames: `{"jsonrpc":"2.0","method":"item/tool/callStarted","params":{}}` + "\n",
			want:   cas14PromptDiagnosticClassification{PrePromptClass: "event_method_incompatible", RequestMethodClass: "none_observed", SchemaShape: "none_observed", RequestCompatibility: "not_observed"},
		},
		{
			name:   "event_shape_incompatible",
			frames: `{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress","futureField":false}}}` + "\n",
			want:   cas14PromptDiagnosticClassification{PrePromptClass: "event_shape_incompatible", RequestMethodClass: "none_observed", SchemaShape: "none_observed", RequestCompatibility: "not_observed"},
		},
		{
			name:      "frame_shape_incompatible",
			frames:    `{"jsonrpc":"2.0","method":"private`,
			want:      cas14PromptDiagnosticClassification{PrePromptClass: "frame_shape_incompatible", RequestMethodClass: "none_observed", SchemaShape: "none_observed", RequestCompatibility: "not_observed"},
			forbidden: "private",
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			observer := &cas14PromptDiagnosticObserver{}
			reader := &cas14PromptDiagnosticReader{ReadCloser: io.NopCloser(strings.NewReader(fixture.frames)), observer: observer}
			buffer := make([]byte, 7)
			for {
				_, err := reader.Read(buffer)
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if len(reader.buffer) != 0 {
				t.Fatal("CAS-14 diagnostic reader retained a provider frame")
			}
			classification := observer.classification()
			if classification != fixture.want {
				t.Fatalf("CAS-14 diagnostic classification = %+v, want %+v", classification, fixture.want)
			}
			encoded, _ := json.Marshal(classification)
			for _, private := range []string{"private-question", "private header", "private question", "private one", "private two", fixture.forbidden} {
				if private != "" && bytes.Contains(encoded, []byte(private)) {
					t.Fatal("CAS-14 diagnostic classification retained provider content")
				}
			}
		})
	}
}

type cas14UserInputSchemaClassification struct {
	Shape          string
	AutoResolution string
	IsOther        string
	IsSecret       string
}

func cas14UserInputRequestSchemaShape(raw json.RawMessage) cas14UserInputSchemaClassification {
	classification := cas14UserInputSchemaClassification{Shape: "changed", AutoResolution: "unknown", IsOther: "unknown", IsSecret: "unknown"}
	var paramFields map[string]json.RawMessage
	if json.Unmarshal(raw, &paramFields) != nil || paramFields == nil {
		return classification
	}
	classification.AutoResolution = cas14FieldPresence(paramFields, "autoResolutionMs")
	var params struct {
		ThreadID  string            `json:"threadId"`
		TurnID    string            `json:"turnId"`
		ItemID    string            `json:"itemId"`
		Questions []json.RawMessage `json:"questions"`
	}
	if json.Unmarshal(raw, &params) != nil || params.ThreadID == "" || params.TurnID == "" || params.ItemID == "" || len(params.Questions) != 1 {
		return classification
	}
	var questionFields map[string]json.RawMessage
	if json.Unmarshal(params.Questions[0], &questionFields) != nil || questionFields == nil {
		return classification
	}
	classification.IsOther = cas14FieldPresence(questionFields, "isOther")
	classification.IsSecret = cas14FieldPresence(questionFields, "isSecret")
	var question struct {
		ID       string `json:"id"`
		Header   string `json:"header"`
		Question string `json:"question"`
		Options  []struct {
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"options"`
	}
	if json.Unmarshal(params.Questions[0], &question) != nil || question.ID == "" || question.Header == "" || question.Question == "" || len(question.Options) != 2 {
		return classification
	}
	for _, option := range question.Options {
		if option.Label == "" || option.Description == "" {
			return classification
		}
	}
	classification.Shape = "single_select"
	return classification
}

func cas14FieldPresence(fields map[string]json.RawMessage, name string) string {
	if _, present := fields[name]; present {
		return "present"
	}
	return "absent"
}

func TestCAS14UserInputRequestSchemaShape(t *testing.T) {
	classification := cas14UserInputRequestSchemaShape(json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","autoResolutionMs":1000,"questions":[{"id":"question-1","header":"header","question":"question","isOther":false,"isSecret":false,"options":[{"label":"one","description":"first"},{"label":"two","description":"second"}]}]}`))
	if classification.Shape != "single_select" || classification.AutoResolution != "present" || classification.IsOther != "present" || classification.IsSecret != "present" {
		t.Fatalf("CAS-14 user-input structural classification = %+v", classification)
	}
}

func cas14ExperimentalFieldCompatibility(field string, raw json.RawMessage) string {
	switch field {
	case "autoResolutionMs":
		if string(raw) == "null" {
			return "safely_ignorable_default"
		}
		var value int64
		if json.Unmarshal(raw, &value) == nil && value >= 0 {
			return "unsupported"
		}
	case "isOther", "isSecret":
		var value bool
		if json.Unmarshal(raw, &value) == nil {
			if !value {
				return "safely_ignorable_default"
			}
			return "unsupported"
		}
	}
	return "invalid"
}

func TestCAS14ExperimentalFieldCompatibilityDecision(t *testing.T) {
	fixtures := []struct {
		field string
		raw   string
		want  string
	}{
		{field: "autoResolutionMs", raw: "null", want: "safely_ignorable_default"},
		{field: "autoResolutionMs", raw: "0", want: "unsupported"},
		{field: "autoResolutionMs", raw: "1000", want: "unsupported"},
		{field: "isOther", raw: "false", want: "safely_ignorable_default"},
		{field: "isOther", raw: "true", want: "unsupported"},
		{field: "isSecret", raw: "false", want: "safely_ignorable_default"},
		{field: "isSecret", raw: "true", want: "unsupported"},
		{field: "unknown", raw: "false", want: "invalid"},
	}
	for _, fixture := range fixtures {
		if got := cas14ExperimentalFieldCompatibility(fixture.field, json.RawMessage(fixture.raw)); got != fixture.want {
			t.Fatalf("CAS-14 %s compatibility = %s, want %s", fixture.field, got, fixture.want)
		}
	}
}

func cas14TerminalClass(raw json.RawMessage) string {
	var params eventParams
	if json.Unmarshal(raw, &params) != nil {
		return "changed"
	}
	switch turnEventStatus(params.Turn.Status) {
	case "completed":
		return "completed"
	case "failed":
		return "failed"
	case "interrupted":
		return "interrupted"
	default:
		return "changed"
	}
}

type cas14ObservingLauncher struct {
	host     HostLauncher
	observer *cas14ResponseObserver
}

func (l *cas14ObservingLauncher) validateLaunch() error { return l.host.validateLaunch() }

func (l *cas14ObservingLauncher) Start(ctx context.Context) (Child, error) {
	child, err := l.host.Start(ctx)
	if err != nil {
		return nil, err
	}
	return &cas14ObservingChild{Child: child, stdin: &cas14ObservingWriter{WriteCloser: child.Stdin(), observer: l.observer}}, nil
}

type cas14ObservingChild struct {
	Child
	stdin io.WriteCloser
}

func (c *cas14ObservingChild) Stdin() io.WriteCloser { return c.stdin }

type cas14ObservingWriter struct {
	io.WriteCloser
	observer *cas14ResponseObserver
	mu       sync.Mutex
	buffer   []byte
}

func (w *cas14ObservingWriter) Write(value []byte) (int, error) {
	written, err := w.WriteCloser.Write(value)
	if written == 0 {
		return written, err
	}
	w.mu.Lock()
	w.buffer = append(w.buffer, value[:written]...)
	for {
		end := bytes.IndexByte(w.buffer, '\n')
		if end < 0 {
			break
		}
		frame := append([]byte(nil), w.buffer[:end]...)
		w.buffer = append(w.buffer[:0], w.buffer[end+1:]...)
		w.observer.inspect(frame)
		clear(frame)
	}
	if len(w.buffer) > 1<<20 {
		clear(w.buffer)
		w.buffer = nil
		w.observer.invalidate()
	}
	w.mu.Unlock()
	return written, err
}

func (w *cas14ObservingWriter) Close() error {
	w.mu.Lock()
	clear(w.buffer)
	w.buffer = nil
	w.mu.Unlock()
	return w.WriteCloser.Close()
}

type cas14ResponseObserver struct {
	mu       sync.Mutex
	expected string
	observed bool
	invalid  bool
}

func (o *cas14ResponseObserver) expect(answer string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.expected != "" || o.observed || answer == "" {
		o.invalid = true
		return
	}
	o.expected = answer
}

func (o *cas14ResponseObserver) inspect(frame []byte) {
	var envelope struct {
		Result struct {
			Answers map[string]struct {
				Answers []string `json:"answers"`
			} `json:"answers"`
		} `json:"result"`
	}
	if json.Unmarshal(frame, &envelope) != nil || envelope.Result.Answers == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	exact := len(envelope.Result.Answers) == 1
	for _, answer := range envelope.Result.Answers {
		exact = exact && len(answer.Answers) == 1 && answer.Answers[0] == o.expected
	}
	o.observed = exact && o.expected != ""
	o.invalid = o.invalid || !o.observed
	o.expected = ""
}

func (o *cas14ResponseObserver) invalidate() {
	o.mu.Lock()
	o.invalid = true
	o.expected = ""
	o.mu.Unlock()
}

func (o *cas14ResponseObserver) exactAnswerObserved() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.observed && !o.invalid && o.expected == ""
}

func waitCAS14UserInputRequest(t *testing.T, ctx context.Context, s *Supervisor) providersession.Event {
	t.Helper()
	for {
		select {
		case event := <-s.Events():
			if event.Kind == providersession.EventUserInputRequested && event.State == providersession.StateWaitingForUserInput && event.UserInput != nil {
				return event
			}
			if event.State == providersession.StateDisconnected || event.Summary == "turn_failed" || event.Summary == "turn_completed" {
				t.Fatal("CAS-14 supported user-input request was not produced")
			}
		case <-ctx.Done():
			t.Fatal("CAS-14 user-input request deadline expired")
		}
	}
}

func waitCAS14PromptDiagnostic(t *testing.T, ctx context.Context, s *Supervisor) (providersession.Event, string, string) {
	t.Helper()
	for {
		select {
		case event := <-s.Events():
			if event.Kind == providersession.EventUserInputRequested && event.State == providersession.StateWaitingForUserInput && event.UserInput != nil {
				return event, "", "none"
			}
			switch {
			case event.Summary == "turn_completed":
				return providersession.Event{}, "completed", "none"
			case event.Summary == "turn_failed":
				return providersession.Event{}, "failed", "none"
			case event.State == providersession.StateDisconnected:
				s.mu.RLock()
				last := s.lastNotification
				s.mu.RUnlock()
				return providersession.Event{}, "disconnected", cas14PromptDisconnectClass(last)
			}
		case <-ctx.Done():
			t.Fatal("CAS-14 prompt diagnostic deadline expired")
		}
	}
}

func cas14PromptDisconnectClass(last string) string {
	switch last {
	case "approval_inactive":
		return "inactive"
	case "approval_turn_mismatch":
		return "turn_mismatch"
	case "approval_item_mismatch":
		return "item_mismatch"
	case "approval_not_running":
		return "not_running"
	default:
		return "other"
	}
}

func waitCAS14UserInputResolution(t *testing.T, ctx context.Context, s *Supervisor) providersession.Event {
	t.Helper()
	for {
		select {
		case event := <-s.Events():
			if event.State == providersession.StateRunning && event.Summary == "user_input_resolved" {
				return event
			}
			if event.State == providersession.StateDisconnected {
				t.Fatal("CAS-14 user-input request did not resolve")
			}
		case <-ctx.Done():
			t.Fatal("CAS-14 user-input resolution deadline expired")
		}
	}
}

func waitCAS14ReplayRejection(t *testing.T, ctx context.Context, s *Supervisor) providersession.Event {
	t.Helper()
	for {
		select {
		case event := <-s.Events():
			if event.State == providersession.StateDisconnected && event.Summary == string(DisconnectDecisionRejected) {
				return event
			}
		case <-ctx.Done():
			t.Fatal("CAS-14 replay rejection deadline expired")
		}
	}
}

func cas14CompleteCorrelation(correlation providersession.Correlation) bool {
	return correlation.ProcessIncarnationID != "" && correlation.ConnectionID != "" && correlation.SessionID != "" && correlation.InteractionID != "" && correlation.ActivityID != "" && correlation.RequestID != "" && correlation.DecisionID != ""
}

func cas14AssertNoRetainedInput(t *testing.T, s *Supervisor, events []providersession.Event, prompt providersession.UserInputPrompt) {
	t.Helper()
	private := []string{prompt.Summary}
	for _, option := range prompt.Options {
		private = append(private, option.Label, option.OptionRef)
	}
	for _, event := range events {
		encoded, _ := json.Marshal(event)
		for _, value := range private {
			if value != "" && bytes.Contains(encoded, []byte(value)) {
				t.Fatal("CAS-14 input data was retained in an event")
			}
		}
	}
	s.audit.mu.Lock()
	encoded, _ := json.Marshal(s.audit.document)
	s.audit.mu.Unlock()
	for _, value := range private {
		if value != "" && bytes.Contains(encoded, []byte(value)) {
			t.Fatal("CAS-14 input data was retained in audit state")
		}
	}
}

func TestCAS13ControlledTransportLoss(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-loss"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 transport-loss initialization failed: %s", cas13FailureClass(s))
	}
	s.mu.RLock()
	stdout := s.stdout
	s.mu.RUnlock()
	if stdout == nil || stdout.Close() != nil {
		t.Fatal("CAS-13 transport-loss close could not be delivered")
	}
	for {
		select {
		case event := <-s.Events():
			if event.State != providersession.StateDisconnected {
				continue
			}
			if event.Summary != string(DisconnectTransportClosed) {
				t.Fatalf("CAS-13 transport-loss class = %s", event.Summary)
			}
			return
		case <-ctx.Done():
			s.mu.RLock()
			state, waitDone := s.state, s.waitDone
			s.mu.RUnlock()
			waited := false
			if waitDone != nil {
				select {
				case <-waitDone:
					waited = true
				default:
				}
			}
			t.Fatalf("CAS-13 transport-loss had no bounded disconnect (state=%s, wait=%t)", state, waited)
		}
	}
}

// TestCAS13ControlledChildDeath kills only the direct child this test started
// and requires the process-exit path—not a synthetic stream close—to produce
// the fail-closed normalized disconnect.
func TestCAS13ControlledChildDeath(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-child-death"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 child-death initialization failed: %s", cas13FailureClass(s))
	}
	s.mu.RLock()
	child := s.child
	s.mu.RUnlock()
	if child == nil || child.Kill() != nil {
		t.Fatal("CAS-13 direct child could not be terminated")
	}
	for {
		select {
		case event := <-s.Events():
			if event.State != providersession.StateDisconnected {
				continue
			}
			if event.Summary != string(DisconnectChildExit) {
				t.Fatalf("CAS-13 child-death class = %s", event.Summary)
			}
			return
		case <-ctx.Done():
			t.Fatal("CAS-13 child death did not produce a bounded disconnect")
		}
	}
}

func waitCAS13Terminal(ctx context.Context, s *Supervisor) string {
	for {
		select {
		case event := <-s.Events():
			if event.State == providersession.StateDisconnected {
				s.mu.RLock()
				last := s.lastNotification
				s.mu.RUnlock()
				if last != "" && last != "other" {
					return event.Summary + "_" + last
				}
				return event.Summary
			}
			if event.Kind == providersession.EventProgress && event.Summary == "turn_failed" {
				s.mu.RLock()
				terminal := s.lastNotification
				s.mu.RUnlock()
				return terminal
			}
			if event.Kind == providersession.EventProgress && event.Summary == "turn_completed" {
				return event.Summary
			}
		case <-ctx.Done():
			s.mu.RLock()
			last := s.lastNotification
			s.mu.RUnlock()
			if last != "" && last != "other" {
				return "request_deadline_" + last
			}
			return "request_deadline"
		}
	}
}

func cas13Prompt(t *testing.T) []any {
	t.Helper()
	prompt := os.Getenv("DOCKPIPE_CAS13_PROMPT")
	if len(prompt) == 0 || len(prompt) > 256 || strings.TrimSpace(prompt) != prompt || strings.ContainsAny(prompt, "\r\n") {
		t.Fatal("CAS-13 requires one bounded single-line ephemeral prompt")
	}
	return []any{map[string]any{"type": "text", "text": prompt}}
}

func waitCAS13Cancellation(ctx context.Context, s *Supervisor) string {
	for {
		select {
		case event := <-s.Events():
			if event.State == providersession.StateCancelled && event.Summary == "cancelled" {
				return "cancelled"
			}
			if event.State == providersession.StateDisconnected {
				return event.Summary
			}
		case <-ctx.Done():
			return "request_deadline"
		}
	}
}

func waitCAS13ActiveItem(ctx context.Context, s *Supervisor) string {
	for {
		select {
		case event := <-s.Events():
			if event.Kind == providersession.EventProgress && event.Summary == "item_started" && event.Correlation.ActivityID != "" {
				s.mu.RLock()
				kind := s.lastNotification
				s.mu.RUnlock()
				if kind == "item_agent" || kind == "item_reasoning" {
					return "item_started"
				}
			}
			if event.State == providersession.StateDisconnected {
				return event.Summary
			}
		case <-ctx.Done():
			return "request_deadline"
		}
	}
}

func waitCAS13ApprovalResolution(ctx context.Context, s *Supervisor) string {
	for {
		select {
		case event := <-s.Events():
			if event.Summary == "approval_resolved" && event.State == providersession.StateRunning {
				return "approval_resolved"
			}
			if event.State == providersession.StateDisconnected {
				return event.Summary
			}
		case <-ctx.Done():
			return "request_deadline"
		}
	}
}

func cas13FailureClass(s *Supervisor) string {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-s.Events():
			if event.State == providersession.StateDisconnected && validAuditDisconnectReason(event.Summary) {
				s.mu.RLock()
				last := s.lastNotification
				s.mu.RUnlock()
				if last != "" && last != "other" {
					return event.Summary + "_" + last
				}
				return event.Summary
			}
		case <-timer.C:
			s.mu.RLock()
			last := s.lastNotification
			s.mu.RUnlock()
			if last != "" {
				return "initialization_rejected_" + last
			}
			return "initialization_rejected"
		}
	}
}

// TestCAS13InitializationEnvelopeShape is a redacted diagnostic for the
// existing direct-child gate. It reports only the first JSON-RPC envelope's
// structural class and discards its content immediately.
func TestCAS13InitializationEnvelopeShape(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS13_DIAGNOSTIC=run for the reviewed shape diagnostic")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child, err := cas13Launcher().Start(ctx)
	if err != nil {
		t.Fatal("CAS-13 direct child could not start")
	}
	defer func() {
		_ = child.Stdin().Close()
		_ = child.Kill()
		_ = child.Wait()
	}()
	if err := json.NewEncoder(child.Stdin()).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas13", "version": "0.1.0"}, "capabilities": map[string]any{}}}); err != nil {
		t.Fatal("CAS-13 initialization request could not be sent")
	}
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if !scanner.Scan() {
		t.Fatal("CAS-13 initialization produced no envelope")
	}
	t.Logf("CAS-13 initialization envelope shape: %s", cas13EnvelopeShape(scanner.Bytes()))
	t.Logf("CAS-13 initialization result shape: %s", cas13InitializationResultShape(scanner.Bytes()))
}

func TestCAS13PostInitializationNotificationClass(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS13_DIAGNOSTIC=run for the reviewed shape diagnostic")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child, err := cas13Launcher().Start(ctx)
	if err != nil {
		t.Fatal("CAS-13 direct child could not start")
	}
	defer func() {
		_ = child.Stdin().Close()
		_ = child.Kill()
		_ = child.Wait()
	}()
	encoder := json.NewEncoder(child.Stdin())
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas13", "version": "0.1.0"}, "capabilities": map[string]any{}}}); err != nil {
		t.Fatal("CAS-13 initialization request could not be sent")
	}
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if !scanner.Scan() {
		t.Fatal("CAS-13 initialization produced no envelope")
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		t.Fatal("CAS-13 initialized notification could not be sent")
	}
	next := make(chan []byte, 1)
	go func() {
		if scanner.Scan() {
			next <- append([]byte(nil), scanner.Bytes()...)
		}
	}()
	select {
	case frame := <-next:
		t.Logf("CAS-13 post-initialization notification class: %s", cas13NotificationClass(frame))
	case <-time.After(5 * time.Second):
		t.Log("CAS-13 post-initialization notification class: none")
	}
}

func TestCAS13ThreadStartedShape(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS13_DIAGNOSTIC=run for the reviewed shape diagnostic")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) {
		t.Fatal("CAS-13 requires an absolute declared workspace")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child, err := cas13Launcher().Start(ctx)
	if err != nil {
		t.Fatal("CAS-13 direct child could not start")
	}
	defer func() { _ = child.Stdin().Close(); _ = child.Kill(); _ = child.Wait() }()
	encoder := json.NewEncoder(child.Stdin())
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas13", "version": "0.1.0"}, "capabilities": map[string]any{"optOutNotificationMethods": []string{remoteControlStatusNotification}}}}); err != nil {
		t.Fatal("CAS-13 initialization request could not be sent")
	}
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if !scanner.Scan() {
		t.Fatal("CAS-13 initialization produced no envelope")
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		t.Fatal("CAS-13 initialized notification could not be sent")
	}
	params := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}.params()
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "thread/start", "params": params}); err != nil {
		t.Fatal("CAS-13 thread request could not be sent")
	}
	for deadline := time.NewTimer(10 * time.Second); ; {
		select {
		case <-deadline.C:
			t.Fatal("CAS-13 thread start notification was not observed")
		default:
		}
		if !scanner.Scan() {
			t.Fatal("CAS-13 thread start transport closed")
		}
		if shape, found := cas13ThreadStartedShape(scanner.Bytes()); found {
			t.Logf("CAS-13 thread started shape: %s", shape)
			return
		}
	}
}

// TestCAS13ThreadStatusShape retains no provider frame. It classifies only
// the field names and JSON value kinds of the first status notification that
// follows a constrained untrusted workspace-change request.
func TestCAS13ThreadStatusShape(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS13_DIAGNOSTIC=run for the reviewed shape diagnostic")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 requires a clean absolute declared workspace")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child, err := cas13Launcher().Start(ctx)
	if err != nil {
		t.Fatal("CAS-13 direct child could not start")
	}
	defer func() { _ = child.Stdin().Close(); _ = child.Kill(); _ = child.Wait() }()
	encoder := json.NewEncoder(child.Stdin())
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas13", "version": "0.1.0"}, "capabilities": map[string]any{"optOutNotificationMethods": []string{remoteControlStatusNotification}}}}); err != nil {
		t.Fatal("CAS-13 initialization request could not be sent")
	}
	if _, ok := cas13DiagnosticResponse(t, scanner, 1); !ok {
		t.Fatal("CAS-13 initialization response was not observed")
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		t.Fatal("CAS-13 initialized notification could not be sent")
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	threadResult := cas13DiagnosticRequest(t, encoder, scanner, 2, "thread/start", policy.params())
	threadID, reason := projectThread(threadResult)
	if reason != "" {
		t.Fatal("CAS-13 diagnostic thread response was invalid")
	}
	turnParams := policy.params()
	turnParams["threadId"] = threadID
	turnParams["input"] = cas13Prompt(t)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "turn/start", "params": turnParams}); err != nil {
		t.Fatal("CAS-13 diagnostic turn request could not be sent")
	}
	statuses := 0
	for scanner.Scan() {
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &envelope) != nil {
			continue
		}
		if envelope.Method == "thread/status/changed" {
			statuses++
			t.Logf("CAS-13 thread status %d shape: %s", statuses, cas13ObjectShape(envelope.Params))
			if statuses > 8 {
				t.Fatal("CAS-13 thread status diagnostic exceeded bounded sequence")
			}
			continue
		}
		if len(envelope.ID) != 0 && envelope.Method != "" {
			t.Logf("CAS-13 approval request shape: id=%s,method=%s,params=%s", cas13ValueShape(envelope.ID), cas13RequestMethodClass(envelope.Method), cas13ObjectShape(envelope.Params))
			return
		}
	}
	t.Fatal("CAS-13 approval request was not observed")
}

func cas13DiagnosticRequest(t *testing.T, encoder *json.Encoder, scanner *bufio.Scanner, id uint64, method string, params map[string]any) json.RawMessage {
	t.Helper()
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		t.Fatalf("CAS-13 %s request could not be sent", method)
	}
	result, ok := cas13DiagnosticResponse(t, scanner, id)
	if !ok {
		t.Fatalf("CAS-13 %s response was not observed", method)
	}
	return result
}

func cas13DiagnosticResponse(t *testing.T, scanner *bufio.Scanner, id uint64) (json.RawMessage, bool) {
	t.Helper()
	for scanner.Scan() {
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &envelope) != nil || len(envelope.ID) == 0 {
			continue
		}
		var received uint64
		if json.Unmarshal(envelope.ID, &received) != nil || received != id || len(envelope.Error) != 0 || len(envelope.Result) == 0 {
			continue
		}
		return append(json.RawMessage(nil), envelope.Result...), true
	}
	return nil, false
}

func cas13ObjectShape(raw json.RawMessage) string {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return "not_object"
	}
	parts := make([]string, 0, len(fields))
	for name, value := range fields {
		parts = append(parts, name+":"+cas13ValueShape(value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func cas13ValueShape(raw json.RawMessage) string {
	var stringValue string
	if json.Unmarshal(raw, &stringValue) == nil {
		return "string(len=" + strconv.Itoa(len(stringValue)) + ",id=" + strconv.FormatBool(validID(stringValue)) + ")"
	}
	var unsigned uint64
	if json.Unmarshal(raw, &unsigned) == nil {
		return "uint(" + strconv.FormatBool(unsigned > 0) + ")"
	}
	var signed int64
	if json.Unmarshal(raw, &signed) == nil {
		return "int"
	}
	var decimal float64
	if json.Unmarshal(raw, &decimal) == nil {
		return "decimal"
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil && object != nil {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return "object(" + strings.Join(keys, "+") + ")"
	}
	if string(raw) == "null" {
		return "null"
	}
	return "other"
}

func cas13RequestMethodClass(method string) string {
	switch method {
	case "item/commandExecution/requestApproval":
		return "command_approval"
	case "item/fileChange/requestApproval":
		return "file_approval"
	case "item/permissions/requestApproval":
		return "permission_approval"
	case "item/tool/requestUserInput":
		return "user_input"
	default:
		return "other"
	}
}

func cas13EnvelopeShape(frame []byte) string {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(frame, &envelope) != nil || envelope == nil {
		return "not_json_object"
	}
	jsonRPC := "absent"
	if raw, found := envelope["jsonrpc"]; found {
		var version string
		if json.Unmarshal(raw, &version) == nil && version == "2.0" {
			jsonRPC = "2.0"
		} else {
			jsonRPC = "other"
		}
	}
	id := "absent"
	if raw, found := envelope["id"]; found {
		var number uint64
		var text string
		switch {
		case json.Unmarshal(raw, &number) == nil && number > 0:
			id = "number"
		case json.Unmarshal(raw, &text) == nil && text != "":
			id = "string"
		default:
			id = "other"
		}
	}
	method := "absent"
	if raw, found := envelope["method"]; found {
		var value string
		if json.Unmarshal(raw, &value) == nil && value != "" {
			method = "present"
		} else {
			method = "other"
		}
	}
	result, failure, params := "absent", "absent", "absent"
	if len(envelope["result"]) != 0 {
		result = "present"
	}
	if len(envelope["error"]) != 0 {
		failure = "present"
	}
	if len(envelope["params"]) != 0 {
		params = "present"
	}
	return "jsonrpc=" + jsonRPC + ",id=" + id + ",method=" + method + ",result=" + result + ",error=" + failure + ",params=" + params
}

func cas13NotificationClass(frame []byte) string {
	var envelope struct {
		Method string `json:"method"`
	}
	if json.Unmarshal(frame, &envelope) != nil || envelope.Method == "" {
		return "not_notification"
	}
	switch envelope.Method {
	case "account/updated":
		return "account_update"
	case "remoteControl/status/changed":
		return "remote_control_status"
	case "configWarning", "config/updated":
		return "configuration"
	case "thread/started", "thread/status/changed", "turn/started", "item/started":
		return "lifecycle"
	case "error", "warning":
		return "diagnostic"
	default:
		return "unrecognized"
	}
}

func cas13InitializationResultShape(frame []byte) string {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(frame, &envelope) != nil {
		return "unavailable"
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(envelope.Result, &result) != nil || result == nil {
		return "not_object"
	}
	fields := []string{"protocolVersion", "serverInfo", "capabilities", "configWarnings", "userAgent", "codexHome", "platformFamily", "platformOs"}
	shape := ""
	for _, field := range fields {
		if len(result[field]) == 0 {
			shape += "0"
		} else {
			shape += "1"
		}
	}
	return shape
}

func cas13ThreadStartedShape(frame []byte) (string, bool) {
	var envelope struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if json.Unmarshal(frame, &envelope) != nil || envelope.Method != "thread/started" {
		return "", false
	}
	var params struct {
		Thread map[string]json.RawMessage `json:"thread"`
	}
	if json.Unmarshal(envelope.Params, &params) != nil || params.Thread == nil {
		return "not_object", true
	}
	known := []string{"id", "name", "preview", "createdAt", "updatedAt", "status", "cwd", "model", "modelProvider", "sandbox", "approvalPolicy"}
	shape, extras := "", 0
	for _, field := range known {
		if len(params.Thread[field]) == 0 {
			shape += "0"
		} else {
			shape += "1"
		}
	}
	for field := range params.Thread {
		found := false
		for _, knownField := range known {
			if field == knownField {
				found = true
				break
			}
		}
		if !found {
			extras++
		}
	}
	return shape + ":extra=" + string(rune('0'+extras)), true
}

func verifyCAS13Catalog(ctx context.Context, s *Supervisor) error {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return errors.New("CAS-13 catalog route is unavailable")
	}
	result, err := s.lifecycleRequest(ctx, client, "model/list", map[string]any{})
	if err != nil || containsModelReroute(result) {
		return errors.New("CAS-13 pinned model policy could not be verified")
	}
	var catalog struct {
		Data []struct {
			ID                        string `json:"id"`
			SupportedReasoningEfforts []struct {
				ReasoningEffort string `json:"reasoningEffort"`
			} `json:"supportedReasoningEfforts"`
		} `json:"data"`
	}
	if json.Unmarshal(result, &catalog) != nil {
		return errors.New("CAS-13 model catalog is malformed")
	}
	for _, model := range catalog.Data {
		if model.ID != PinnedModel {
			continue
		}
		for _, effort := range model.SupportedReasoningEfforts {
			if effort.ReasoningEffort == PinnedReasoningEffort {
				return nil
			}
		}
	}
	return errors.New("CAS-13 pinned model and reasoning effort are unavailable")
}

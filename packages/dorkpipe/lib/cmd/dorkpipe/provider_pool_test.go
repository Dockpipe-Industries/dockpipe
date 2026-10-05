package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dorkpipe.orchestrator/appserversupervisor"
	"dorkpipe.orchestrator/providersession"
)

func TestProviderPoolPromptArgsUsesCanonicalWorkflowArgs(t *testing.T) {
	t.Setenv("DOCKPIPE_ARGS_JSON", `["--provider","ollama","--prompt","hello"]`)
	got := providerPoolPromptArgs(nil)
	want := []string{"--provider", "ollama", "--prompt", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv=%v want %v", got, want)
	}
}

func TestProviderPoolPromptArgsAppendsWorkflowArgsAfterScriptFlags(t *testing.T) {
	t.Setenv("DOCKPIPE_ARGS_JSON", `["--provider","ollama","--prompt","hello"]`)
	got := providerPoolPromptArgs([]string{"--workdir", "C:\\repo"})
	want := []string{"--workdir", "C:\\repo", "--provider", "ollama", "--prompt", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv=%v want %v", got, want)
	}
}

func TestProviderPoolPrivateApprovalStdioCarriesExactValidatedNeutralDecision(t *testing.T) {
	request := providerPoolApprovalFixture("stdio", providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	encoded, err := json.Marshal(providerPoolPrivateApprovalDecisionFrame{Type: "approval_decision", Decision: decision})
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.NewBufferString(providerPoolPrivateApprovalFramePrefix + string(encoded) + "\n")
	var output bytes.Buffer
	source := newProviderPoolPrivateApprovalStdio(input, &output).decisionSource
	got, found, err := source(context.Background(), request)
	if err != nil || !found || !reflect.DeepEqual(got, decision) {
		t.Fatalf("decision=%+v found=%t err=%v", got, found, err)
	}
	line := strings.TrimSpace(output.String())
	if !strings.HasPrefix(line, providerPoolPrivateApprovalFramePrefix) {
		t.Fatalf("request frame=%q", line)
	}
	var frame providerPoolPrivateApprovalRequestFrame
	decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(line, providerPoolPrivateApprovalFramePrefix)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&frame); err != nil || frame.Type != "approval_request" || !reflect.DeepEqual(frame.Request, request) {
		t.Fatalf("request frame=%+v err=%v", frame, err)
	}
}

func TestProviderPoolPrivateApprovalStdioRejectsSubstitutedAndExtendedDecisions(t *testing.T) {
	request := providerPoolApprovalFixture("strict", providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	substituted := decision
	substituted.Correlation.RequestID = "other-request"
	for _, tc := range []struct {
		name  string
		frame string
	}{
		{name: "substituted", frame: providerPoolPrivateDecisionFixture(t, substituted, "")},
		{name: "extended", frame: providerPoolPrivateDecisionFixture(t, decision, `,"provider_payload":"forbidden"`)},
		{name: "wrong_type", frame: strings.Replace(providerPoolPrivateDecisionFixture(t, decision, ""), `"approval_decision"`, `"approval_request"`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			source := newProviderPoolPrivateApprovalStdio(bytes.NewBufferString(tc.frame), &output).decisionSource
			if _, found, err := source(context.Background(), request); err == nil || found {
				t.Fatalf("found=%t err=%v", found, err)
			}
		})
	}
}

func TestProviderPoolPrivateApprovalStdioWaitsWithoutAutomaticDecision(t *testing.T) {
	request := providerPoolApprovalFixture("blocked", providersession.ApprovalReasonWorkspaceChange, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	transport := newProviderPoolPrivateApprovalStdio(inputReader, outputWriter)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := transport.decisionSource(ctx, request)
		done <- err
	}()
	line, err := bufio.NewReader(outputReader).ReadString('\n')
	if err != nil || !strings.HasPrefix(line, providerPoolPrivateApprovalFramePrefix) {
		t.Fatalf("request line=%q err=%v", line, err)
	}
	select {
	case err := <-done:
		t.Fatalf("source returned without a decision: %v", err)
	default:
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	_ = inputWriter.Close()
	_ = outputReader.Close()
	_ = outputWriter.Close()
}

func providerPoolPrivateDecisionFixture(t *testing.T, decision providersession.ApprovalDecision, extra string) string {
	t.Helper()
	encoded, err := json.Marshal(providerPoolPrivateApprovalDecisionFrame{Type: "approval_decision", Decision: decision})
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if extra != "" {
		text = strings.TrimSuffix(text, "}") + extra + "}"
	}
	return providerPoolPrivateApprovalFramePrefix + text + "\n"
}

func TestProviderPoolPrivateInteractiveStdioCarriesExactUserInputPromptsAndResponses(t *testing.T) {
	request := providerPoolUserInputFixture("private-transport")
	for _, tc := range []struct {
		name     string
		prompt   providersession.UserInputPrompt
		response providersession.UserInputResponse
	}{
		{
			name:     "select_one",
			prompt:   providersession.UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Kind: providersession.UserInputPromptSelectOne, Summary: "Choose one safe option", Options: []providersession.UserInputOption{{OptionRef: "one", Label: "One"}, {OptionRef: "two", Label: "Two"}}, MaxSelections: 1},
			response: providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{"two"}},
		},
		{
			name:     "select_many",
			prompt:   providersession.UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Kind: providersession.UserInputPromptSelectMany, Summary: "Choose safe options", Options: []providersession.UserInputOption{{OptionRef: "one", Label: "One"}, {OptionRef: "two", Label: "Two"}, {OptionRef: "three", Label: "Three"}}, MaxSelections: 2},
			response: providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{"one", "three"}},
		},
		{
			name:     "bounded_text",
			prompt:   providersession.UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Kind: providersession.UserInputPromptText, Summary: "Provide bounded text", MaxTextBytes: 32},
			response: providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, Text: "safe answer"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(providerPoolPrivateUserInputResponseFrame{Type: "user_input_response", Response: tc.response})
			if err != nil {
				t.Fatal(err)
			}
			input := bytes.NewBufferString(providerPoolPrivateUserInputFramePrefix + string(encoded) + "\n")
			var output bytes.Buffer
			got, found, err := newProviderPoolPrivateInteractiveStdio(input, &output).responseSource(context.Background(), tc.prompt)
			if err != nil || !found || !reflect.DeepEqual(got, tc.response) {
				t.Fatalf("response=%+v found=%t err=%v", got, found, err)
			}
			line := strings.TrimSpace(output.String())
			if !strings.HasPrefix(line, providerPoolPrivateUserInputFramePrefix) {
				t.Fatalf("prompt frame=%q", line)
			}
			var frame providerPoolPrivateUserInputPromptFrame
			if err := decodeProviderPoolPrivateFrame([]byte(strings.TrimPrefix(line, providerPoolPrivateUserInputFramePrefix)), &frame); err != nil || frame.Type != "user_input_prompt" || !reflect.DeepEqual(frame.Prompt, tc.prompt) {
				t.Fatalf("prompt frame=%+v err=%v", frame, err)
			}
		})
	}
}

func TestProviderPoolPrivateInteractiveStdioPreservesExactApprovalTransport(t *testing.T) {
	request := providerPoolApprovalFixture("combined-approval", providersession.ApprovalReasonWorkspaceChange, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}
	encoded, err := json.Marshal(providerPoolPrivateApprovalDecisionFrame{Type: "approval_decision", Decision: decision})
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.NewBufferString(providerPoolPrivateApprovalFramePrefix + string(encoded) + "\n")
	var output bytes.Buffer
	got, found, err := newProviderPoolPrivateInteractiveStdio(input, &output).decisionSource(context.Background(), request)
	if err != nil || !found || !reflect.DeepEqual(got, decision) {
		t.Fatalf("decision=%+v found=%t err=%v", got, found, err)
	}
	line := strings.TrimSpace(output.String())
	if !strings.HasPrefix(line, providerPoolPrivateApprovalFramePrefix) || strings.Contains(line, providerPoolPrivateUserInputFramePrefix) {
		t.Fatalf("approval request frame=%q", line)
	}
	var frame providerPoolPrivateApprovalRequestFrame
	if err := decodeProviderPoolPrivateFrame([]byte(strings.TrimPrefix(line, providerPoolPrivateApprovalFramePrefix)), &frame); err != nil || frame.Type != "approval_request" || !reflect.DeepEqual(frame.Request, request) {
		t.Fatalf("approval request frame=%+v err=%v", frame, err)
	}
}

func TestProviderPoolPrivateInteractiveStdioDemultiplexesCancellationAndApproval(t *testing.T) {
	scope := providerPoolCancellationScope{
		Session: providersession.SessionRef{Provider: "codex", SessionID: "session-private-concurrent"},
		Correlation: providersession.Correlation{
			ProcessIncarnationID: "process-private-concurrent",
			ConnectionID:         "connection-private-concurrent",
			SessionID:            "session-private-concurrent",
			InteractionID:        "turn-private-concurrent",
		},
	}
	intent := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonSafetyStop}
	request := providerPoolApprovalFixture("private-concurrent", providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	decision := providersession.ApprovalDecision{Correlation: request.Correlation, Decision: providersession.DecisionApprove}

	fromBridge, toChild := io.Pipe()
	frames := &providerPoolCapturedFrameWriter{frames: make(chan []byte, 2)}
	transport := newProviderPoolPrivateInteractiveStdio(fromBridge, frames)
	defer transport.close(errors.New("fixture complete"))

	type cancellationResult struct {
		intent providersession.CancellationIntent
		found  bool
		err    error
	}
	cancellationDone := make(chan cancellationResult, 1)
	go func() {
		got, found, err := transport.cancellationSource(context.Background(), scope)
		cancellationDone <- cancellationResult{intent: got, found: found, err: err}
	}()
	if line := string(<-frames.frames); !strings.HasPrefix(line, providerPoolPrivateCancellationFramePrefix) {
		t.Fatalf("first frame=%q", line)
	}

	type approvalResult struct {
		decision providersession.ApprovalDecision
		found    bool
		err      error
	}
	approvalDone := make(chan approvalResult, 1)
	go func() {
		got, found, err := transport.decisionSource(context.Background(), request)
		approvalDone <- approvalResult{decision: got, found: found, err: err}
	}()
	if line := string(<-frames.frames); !strings.HasPrefix(line, providerPoolPrivateApprovalFramePrefix) {
		t.Fatalf("second frame=%q", line)
	}

	approvalJSON, _ := json.Marshal(providerPoolPrivateApprovalDecisionFrame{Type: "approval_decision", Decision: decision})
	cancellationJSON, _ := json.Marshal(providerPoolPrivateCancellationIntentFrame{Type: "cancellation_intent", Intent: intent})
	if _, err := fmt.Fprintf(toChild, "%s%s\n%s%s\n", providerPoolPrivateApprovalFramePrefix, approvalJSON, providerPoolPrivateCancellationFramePrefix, cancellationJSON); err != nil {
		t.Fatal(err)
	}
	if got := <-approvalDone; got.err != nil || !got.found || !reflect.DeepEqual(got.decision, decision) {
		t.Fatalf("approval=%+v", got)
	}
	if got := <-cancellationDone; got.err != nil || !got.found || !reflect.DeepEqual(got.intent, intent) {
		t.Fatalf("cancellation=%+v", got)
	}
	_ = toChild.Close()
}

func TestProviderPoolPrivateInteractiveStdioRejectsSubstitutedCancellationIntent(t *testing.T) {
	scope := providerPoolCancellationScope{
		Session:     providersession.SessionRef{Provider: "codex", SessionID: "session-private-stale"},
		Correlation: providersession.Correlation{ProcessIncarnationID: "process-private-stale", ConnectionID: "connection-private-stale", SessionID: "session-private-stale", InteractionID: "turn-private-stale"},
	}
	intent := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonUserRequested}
	intent.Correlation.ConnectionID = "other-connection"
	encoded, _ := json.Marshal(providerPoolPrivateCancellationIntentFrame{Type: "cancellation_intent", Intent: intent})
	transport := newProviderPoolPrivateInteractiveStdio(bytes.NewBufferString(providerPoolPrivateCancellationFramePrefix+string(encoded)+"\n"), io.Discard)
	_, found, err := transport.cancellationSource(context.Background(), scope)
	if err == nil || found {
		t.Fatalf("found=%t err=%v", found, err)
	}
}

type providerPoolCapturedFrameWriter struct {
	frames chan []byte
}

func (w *providerPoolCapturedFrameWriter) Write(raw []byte) (int, error) {
	w.frames <- append([]byte(nil), raw...)
	return len(raw), nil
}

func TestProviderPoolPrivateInteractiveStdioRejectsInvalidUserInputFramesBeforeDelivery(t *testing.T) {
	request := providerPoolUserInputFixture("private-invalid")
	prompt := providersession.UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Kind: providersession.UserInputPromptText, Summary: "Provide bounded text", MaxTextBytes: 8}
	valid := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, Text: "safe"}
	substituted := valid
	substituted.Correlation.ProcessIncarnationID = "other-process"
	control := valid
	control.Text = "bad\ntext"
	oversized := valid
	oversized.Text = "123456789"

	frame := func(response providersession.UserInputResponse, extra string) string {
		encoded, err := json.Marshal(providerPoolPrivateUserInputResponseFrame{Type: "user_input_response", Response: response})
		if err != nil {
			t.Fatal(err)
		}
		text := string(encoded)
		if extra != "" {
			text = strings.TrimSuffix(text, "}") + extra + "}"
		}
		return providerPoolPrivateUserInputFramePrefix + text + "\n"
	}
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{name: "substituted", raw: frame(substituted, "")},
		{name: "control", raw: frame(control, "")},
		{name: "oversized", raw: frame(oversized, "")},
		{name: "extended", raw: frame(valid, `,"provider_payload":"forbidden"`)},
		{name: "wrong_class", raw: strings.Replace(frame(valid, ""), providerPoolPrivateUserInputFramePrefix, providerPoolPrivateApprovalFramePrefix, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			_, found, err := newProviderPoolPrivateInteractiveStdio(bytes.NewBufferString(tc.raw), &output).responseSource(context.Background(), prompt)
			if err == nil || found {
				t.Fatalf("found=%t err=%v", found, err)
			}
		})
	}
}

func TestProviderPoolShapeJSONTags(t *testing.T) {
	raw, err := json.Marshal(providerPoolProviderShape{
		MinReady:        1,
		MaxActive:       2,
		IdleTTLSeconds:  900,
		Role:            "direct",
		SessionAffinity: true,
		WarmMode:        "guarded_container",
		RequiresAuth:    true,
		WarmSource:      "docker-claude-resolver",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if got == "" || !containsAll(got,
		`"min_ready":1`,
		`"max_active":2`,
		`"idle_ttl_seconds":900`,
		`"session_affinity":true`,
		`"warm_mode":"guarded_container"`,
	) {
		t.Fatalf("unexpected json: %s", got)
	}
}

func TestProviderPoolDockpipeBinPrefersRepoLocalBinary(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "src", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(binDir, "dockpipe.exe")
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := providerPoolDockpipeBin(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q want %q", got, path)
	}
}

func TestProviderPoolCodexCLIPathPrefersExplicitEnv(t *testing.T) {
	root := t.TempDir()
	codexPath := filepath.Join(root, "codex.exe")
	if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_CLI_PATH", codexPath)
	got, err := providerPoolCodexCLIPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != codexPath {
		t.Fatalf("got %q want %q", got, codexPath)
	}
}

func TestProviderPoolCodexAppServerDispatchResumesOnlyVerifiedIdleAndNeverReplaysUnresolvedTurn(t *testing.T) {
	root := providerPoolDurableTestWorkdir(t)
	codexPath := filepath.Join(root, "codex.exe")
	if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_CLI_PATH", codexPath)
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-app-server-1", providerPoolCodexAppServerAdapter); err != nil {
		t.Fatal(err)
	}
	original := runProviderPoolCodexAppServerPromptFunc
	originalCapture := runProviderPoolCommandCaptureFunc
	t.Cleanup(func() {
		runProviderPoolCodexAppServerPromptFunc = original
		runProviderPoolCommandCaptureFunc = originalCapture
	})
	calls := 0
	execCalls := 0
	failTurn := false
	runProviderPoolCommandCaptureFunc = func(_ context.Context, workdir, executable string, args ...string) (string, string, int, error) {
		execCalls++
		if workdir != root || executable != codexPath || len(args) == 0 || args[len(args)-1] != "fixture" {
			t.Fatalf("setup fallback exec = %q %v in %q", executable, args, workdir)
		}
		return "setup-fallback", "", 0, nil
	}
	runProviderPoolCodexAppServerPromptFunc = func(_ context.Context, opts providerPoolPromptOptions, model, executable string, prior *providerPoolAppServerSessionState, turn uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
		calls++
		if opts.SessionAdapter != providerPoolCodexAppServerAdapter || model != "gpt-5.5" || executable != codexPath {
			t.Fatalf("dispatch = %+v, %q, %q", opts, model, executable)
		}
		if opts.cancellationIntentSource != nil {
			t.Fatal("normal App Server dispatch installed a cancellation source")
		}
		if turn == 1 && prior != nil {
			t.Fatalf("first turn has prior state: %+v", prior)
		}
		if turn > 1 && (prior == nil || prior.CompletedTurn != turn-1 || prior.ProviderSessionID != "thread-1") {
			t.Fatalf("resume turn %d prior = %+v", turn, prior)
		}
		if failTurn {
			return &providerPoolAppServerRunResult{Response: &providerPoolPromptResponse{Provider: "codex", State: "failed", Text: "unknown", ExitCode: 1}}, providerPoolAppServerDispatchedOrUnknown, nil
		}
		return &providerPoolAppServerRunResult{
			Response:          &providerPoolPromptResponse{Provider: "codex", State: "ready", Text: fmt.Sprintf("fixture-%d", turn), ExitCode: 0},
			ProviderSessionID: "thread-1",
			RecoveryEvidence:  providerPoolCodexAppServerRecoveryEvidence(opts.SessionID),
			VerifiedIdle:      true,
		}, providerPoolAppServerDispatchedOrUnknown, nil
	}
	opts := providerPoolPromptOptions{Workdir: root, Provider: "codex", Prompt: "fixture", SessionID: "pipeon-app-server-1", SessionAdapter: providerPoolCodexAppServerAdapter}
	setupFallback, err := runProviderPoolCodexPrompt(context.Background(), opts, "config")
	if err != nil || setupFallback.Text != "setup-fallback" || execCalls != 1 {
		t.Fatalf("config alias fallback = %+v, err=%v, exec=%d", setupFallback, err, execCalls)
	}
	if calls != 0 {
		t.Fatalf("config alias reached App Server runner: %d", calls)
	}
	result, err := runProviderPoolCodexPrompt(context.Background(), opts, "gpt-5.5")
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "fixture-1" || calls != 1 || execCalls != 1 {
		t.Fatalf("first dispatch = %+v, calls=%d, exec=%d", result, calls, execCalls)
	}
	result, err = runProviderPoolCodexPrompt(context.Background(), opts, "gpt-5.5")
	if err != nil || result.Text != "fixture-2" || calls != 2 {
		t.Fatalf("resumed dispatch = %+v, err=%v, calls=%d", result, err, calls)
	}
	state, found, err := loadProviderPoolCodexAppServerSession(root, opts.SessionID)
	if err != nil || !found || state.CompletedTurn != 2 || state.Model != "gpt-5.5" || state.ReasoningEffort != appserversupervisor.PinnedReasoningEffort {
		t.Fatalf("persisted idle state = %+v, found=%t, err=%v", state, found, err)
	}
	if _, err := runProviderPoolCodexPrompt(context.Background(), opts, "gpt-5.4"); err == nil || !strings.Contains(err.Error(), "pinned to model") {
		t.Fatalf("model drift error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("model drift reached App Server runner: %d", calls)
	}
	failTurn = true
	failed, err := runProviderPoolCodexPrompt(context.Background(), opts, "gpt-5.5")
	if err != nil || failed.State != "failed" || calls != 3 {
		t.Fatalf("failed third dispatch = %+v, err=%v, calls=%d", failed, err, calls)
	}
	if _, err := runProviderPoolCodexPrompt(context.Background(), opts, "gpt-5.5"); err == nil || !strings.Contains(err.Error(), "unresolved prior turn") {
		t.Fatalf("replay block error = %v", err)
	}
	if calls != 3 {
		t.Fatalf("blocked replay reached App Server runner: %d", calls)
	}
	if execCalls != 1 {
		t.Fatalf("unresolved App Server turn replayed through exec: %d", execCalls)
	}
}

type fakeProviderPoolAppServerController struct {
	events               chan providersession.Event
	eventCalls           int
	state                providersession.State
	stateCalls           int
	decisions            []providersession.ApprovalDecision
	decideErr            error
	userInputPrompt      providersession.UserInputPrompt
	userInputPromptCalls int
	userInputPromptErr   error
	userInputResponses   []providersession.UserInputResponse
	respondUserInputErr  error
	cancellationIntents  []providersession.CancellationIntent
	cancelErr            error
	onCancel             func(providersession.CancellationIntent)
	completedText        string
	recovery             string
}

type fakeProviderPoolAppServerSupervisor struct {
	fakeProviderPoolAppServerController
	failStage        string
	startCalls       int
	selectionCalls   int
	threadCalls      int
	recoveryCalls    int
	recoveryRequests []providersession.RecoveryRequest
	recoveryPolicies []appserversupervisor.LifecyclePolicy
	turnStartCalls   int
	shutdownCalls    int
	shutdownErr      error
	providerSession  string
}

func (f *fakeProviderPoolAppServerSupervisor) Start(context.Context) error {
	f.startCalls++
	if f.failStage == "initialization" {
		return errors.New("fixture initialization failure")
	}
	return nil
}

func (f *fakeProviderPoolAppServerSupervisor) Shutdown(context.Context) error {
	f.shutdownCalls++
	return f.shutdownErr
}

func (f *fakeProviderPoolAppServerSupervisor) SelectBaselinePolicy(context.Context, string, string) (providersession.EffectivePolicySnapshot, error) {
	f.selectionCalls++
	if f.failStage == "selection" {
		return providersession.EffectivePolicySnapshot{}, errors.New("fixture selection failure")
	}
	return providersession.EffectivePolicySnapshot{}, nil
}

func (f *fakeProviderPoolAppServerSupervisor) StartThread(context.Context, appserversupervisor.LifecyclePolicy) (appserversupervisor.LifecycleReference, error) {
	f.threadCalls++
	if f.failStage == "thread" {
		return appserversupervisor.LifecycleReference{}, errors.New("fixture thread failure")
	}
	return appserversupervisor.LifecycleReference{Session: providersession.SessionRef{Provider: "codex", SessionID: f.providerSession}}, nil
}

func (f *fakeProviderPoolAppServerSupervisor) RecoverBaseline(_ context.Context, request providersession.RecoveryRequest, policy appserversupervisor.LifecyclePolicy) (appserversupervisor.LifecycleReference, providersession.EffectivePolicySnapshot, error) {
	f.recoveryCalls++
	f.recoveryRequests = append(f.recoveryRequests, request)
	f.recoveryPolicies = append(f.recoveryPolicies, policy)
	if f.failStage == "recovery" {
		return appserversupervisor.LifecycleReference{}, providersession.EffectivePolicySnapshot{}, errors.New("fixture recovery failure")
	}
	return appserversupervisor.LifecycleReference{Session: providersession.SessionRef{Provider: "codex", SessionID: f.providerSession}}, providersession.EffectivePolicySnapshot{}, nil
}

func (f *fakeProviderPoolAppServerSupervisor) StartPromptTurn(context.Context, appserversupervisor.LifecycleReference, appserversupervisor.LifecyclePolicy, string) (appserversupervisor.LifecycleReference, error) {
	f.turnStartCalls++
	if f.failStage == "turn_start" {
		return appserversupervisor.LifecycleReference{}, errors.New("fixture turn start failure")
	}
	return appserversupervisor.LifecycleReference{Session: providersession.SessionRef{Provider: "codex", SessionID: f.providerSession}}, nil
}

func (f *fakeProviderPoolAppServerController) Events() <-chan providersession.Event {
	f.eventCalls++
	return f.events
}

func (f *fakeProviderPoolAppServerController) State() providersession.State {
	f.stateCalls++
	return f.state
}

func (f *fakeProviderPoolAppServerController) Decide(_ context.Context, decision providersession.ApprovalDecision) error {
	f.decisions = append(f.decisions, decision)
	return f.decideErr
}

func (f *fakeProviderPoolAppServerController) UserInputPrompt(_ context.Context, _ providersession.UserInputRequest) (providersession.UserInputPrompt, error) {
	f.userInputPromptCalls++
	return cloneProviderPoolUserInputPrompt(f.userInputPrompt), f.userInputPromptErr
}

func (f *fakeProviderPoolAppServerController) RespondUserInput(_ context.Context, response providersession.UserInputResponse) error {
	f.userInputResponses = append(f.userInputResponses, cloneProviderPoolUserInputResponse(response))
	return f.respondUserInputErr
}

func (f *fakeProviderPoolAppServerController) Cancel(_ context.Context, intent providersession.CancellationIntent) error {
	f.cancellationIntents = append(f.cancellationIntents, intent)
	if f.onCancel != nil {
		f.onCancel(intent)
	}
	return f.cancelErr
}

func (f *fakeProviderPoolAppServerController) CompletedTurnText() (string, bool) {
	return f.completedText, f.completedText != ""
}

func (f *fakeProviderPoolAppServerController) RecoveryEvidence() string {
	return f.recovery
}

func TestProviderPoolAppServerFallbackEligibilityFollowsTurnStartBoundary(t *testing.T) {
	originalFactory := newProviderPoolAppServerSupervisorFunc
	originalCapture := runProviderPoolCommandCaptureFunc
	t.Cleanup(func() {
		newProviderPoolAppServerSupervisorFunc = originalFactory
		runProviderPoolCommandCaptureFunc = originalCapture
	})
	captureCalls := 0
	runProviderPoolCommandCaptureFunc = func(context.Context, string, string, ...string) (string, string, int, error) {
		captureCalls++
		return "", "", 1, errors.New("unexpected codex exec fixture call")
	}

	if providerPoolAppServerDispatchedOrUnknown.allowsFallback() {
		t.Fatal("dispatched-or-unknown classification authorized fallback")
	}
	if !providerPoolAppServerEligibleBeforeTurnStart.allowsFallback() {
		t.Fatal("proven pre-turn/start classification did not retain eligibility")
	}

	root := providerPoolDurableTestWorkdir(t)
	opts := providerPoolPromptOptions{Workdir: root, Prompt: "fixture", SessionID: "readiness-session", SessionAdapter: providerPoolCodexAppServerAdapter}
	if _, eligibility, err := runProviderPoolCodexAppServerPrompt(context.Background(), providerPoolPromptOptions{Workdir: "\x00", SessionID: opts.SessionID, Prompt: opts.Prompt}, "gpt-5.5", "codex", nil, 1); err == nil || !eligibility.allowsFallback() {
		t.Fatalf("policy construction classification = %v, err=%v", eligibility, err)
	}

	newProviderPoolAppServerSupervisorFunc = func(providerPoolAppServerSupervisorSpec) (providerPoolAppServerSupervisor, error) {
		return nil, errors.New("fixture supervisor construction failure")
	}
	if _, eligibility, err := runProviderPoolCodexAppServerPrompt(context.Background(), opts, "gpt-5.5", "codex", nil, 1); err == nil || !eligibility.allowsFallback() {
		t.Fatalf("supervisor construction classification = %v, err=%v", eligibility, err)
	}

	for _, tc := range []struct {
		name  string
		stage string
		prior *providerPoolAppServerSessionState
	}{
		{name: "initialization", stage: "initialization"},
		{name: "model and policy selection", stage: "selection"},
		{name: "thread creation", stage: "thread"},
		{name: "verified idle recovery", stage: "recovery", prior: &providerPoolAppServerSessionState{ProviderSessionID: "thread-fixture", RecoveryEvidence: providerPoolCodexAppServerRecoveryEvidence(opts.SessionID)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &fakeProviderPoolAppServerSupervisor{
				fakeProviderPoolAppServerController: fakeProviderPoolAppServerController{events: make(chan providersession.Event)},
				failStage:                           tc.stage,
				providerSession:                     "thread-fixture",
			}
			newProviderPoolAppServerSupervisorFunc = func(providerPoolAppServerSupervisorSpec) (providerPoolAppServerSupervisor, error) {
				return fixture, nil
			}
			_, eligibility, err := runProviderPoolCodexAppServerPrompt(context.Background(), opts, "gpt-5.5", "codex", tc.prior, 1)
			if err == nil || !eligibility.allowsFallback() {
				t.Fatalf("classification = %v, err=%v", eligibility, err)
			}
			if fixture.turnStartCalls != 0 {
				t.Fatalf("pre-turn/start failure reached StartPromptTurn: %d", fixture.turnStartCalls)
			}
		})
	}

	for _, tc := range []struct {
		name        string
		stage       string
		closeEvents bool
	}{
		{name: "turn start error", stage: "turn_start"},
		{name: "consumer error", closeEvents: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := make(chan providersession.Event)
			if tc.closeEvents {
				close(events)
			}
			fixture := &fakeProviderPoolAppServerSupervisor{
				fakeProviderPoolAppServerController: fakeProviderPoolAppServerController{events: events},
				failStage:                           tc.stage,
				providerSession:                     "thread-fixture",
			}
			newProviderPoolAppServerSupervisorFunc = func(providerPoolAppServerSupervisorSpec) (providerPoolAppServerSupervisor, error) {
				return fixture, nil
			}
			run, eligibility, err := runProviderPoolCodexAppServerPrompt(context.Background(), opts, "gpt-5.5", "codex", nil, 1)
			if eligibility != providerPoolAppServerDispatchedOrUnknown || eligibility.allowsFallback() {
				t.Fatalf("classification = %v, run=%+v, err=%v", eligibility, run, err)
			}
			if tc.closeEvents {
				if err != nil || run == nil || run.Response == nil || run.Response.State != "failed" {
					t.Fatalf("consumer failure = %+v, err=%v", run, err)
				}
			} else if err == nil {
				t.Fatal("StartPromptTurn fixture error was not returned")
			}
			if fixture.turnStartCalls != 1 {
				t.Fatalf("StartPromptTurn calls = %d", fixture.turnStartCalls)
			}
		})
	}

	if captureCalls != 0 {
		t.Fatalf("readiness classification launched codex exec %d times", captureCalls)
	}
}

func TestProviderPoolCodexAdapterRoutesStayExactAcrossOneShotFallback(t *testing.T) {
	originalAppServer := runProviderPoolCodexAppServerPromptFunc
	originalCapture := runProviderPoolCommandCaptureFunc
	t.Cleanup(func() {
		runProviderPoolCodexAppServerPromptFunc = originalAppServer
		runProviderPoolCommandCaptureFunc = originalCapture
	})
	root := providerPoolDurableTestWorkdir(t)
	codexPath := filepath.Join(root, "codex.exe")
	if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_CLI_PATH", codexPath)

	appServerCalls := 0
	execCalls := 0
	runProviderPoolCodexAppServerPromptFunc = func(_ context.Context, _ providerPoolPromptOptions, _ string, _ string, _ *providerPoolAppServerSessionState, _ uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
		appServerCalls++
		return &providerPoolAppServerRunResult{Response: &providerPoolPromptResponse{Provider: "codex", State: "failed", ExitCode: 1}}, providerPoolAppServerDispatchedOrUnknown, nil
	}
	runProviderPoolCommandCaptureFunc = func(_ context.Context, workdir, executable string, args ...string) (string, string, int, error) {
		execCalls++
		if workdir != root || executable != codexPath || len(args) == 0 || args[0] != "exec" {
			t.Fatalf("governed exec route = %q %v in %q", executable, args, workdir)
		}
		return "exec fixture", "", 0, nil
	}

	appServerOpts := providerPoolPromptOptions{Workdir: root, Prompt: "app server fixture", SessionID: "exact-app-server", SessionAdapter: providerPoolCodexAppServerAdapter}
	if _, err := runProviderPoolCodexAppServerFallback(context.Background(), providerPoolPromptOptions{Workdir: root, SessionAdapter: providerPoolCodexExecAdapter}, "gpt-5.5", codexPath, providerPoolAppServerEligibleBeforeTurnStart, "", 0); err == nil || execCalls != 0 {
		t.Fatalf("non-App-Server request entered fallback: err=%v exec=%d", err, execCalls)
	}
	if _, err := runProviderPoolCodexAppServerFallback(context.Background(), appServerOpts, "gpt-5.5", codexPath, providerPoolAppServerDispatchedOrUnknown, "", 0); err == nil || execCalls != 0 {
		t.Fatalf("unsafe classification entered fallback: err=%v exec=%d", err, execCalls)
	}
	if result, eligibility, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), appServerOpts, "config"); err != nil || result.Text != "exec fixture" || !eligibility.allowsFallback() || appServerCalls != 0 || execCalls != 1 {
		t.Fatalf("pre-dispatch setup = eligibility=%v, err=%v, app-server=%d, exec=%d", eligibility, err, appServerCalls, execCalls)
	}
	appServerResult, eligibility, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), appServerOpts, "gpt-5.5")
	if err != nil || appServerResult.State != "failed" || eligibility != providerPoolAppServerDispatchedOrUnknown || appServerCalls != 1 || execCalls != 1 {
		t.Fatalf("App Server route = %+v, eligibility=%v, err=%v, app-server=%d, exec=%d", appServerResult, eligibility, err, appServerCalls, execCalls)
	}

	execOpts := providerPoolPromptOptions{Workdir: root, Prompt: "exec fixture", SessionID: "exact-exec", SessionAdapter: providerPoolCodexExecAdapter}
	execResult, eligibility, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), execOpts, "gpt-5.5")
	if err != nil || execResult.Text != "exec fixture" || eligibility != providerPoolAppServerDispatchedOrUnknown || appServerCalls != 1 || execCalls != 2 {
		t.Fatalf("exec route = %+v, eligibility=%v, err=%v, app-server=%d, exec=%d", execResult, eligibility, err, appServerCalls, execCalls)
	}

	for _, classification := range []providerPoolAppServerFallbackEligibility{providerPoolAppServerEligibleBeforeTurnStart, providerPoolAppServerDispatchedOrUnknown} {
		caseRoot := providerPoolDurableTestWorkdir(t)
		caseCodexPath := filepath.Join(caseRoot, "codex.exe")
		if err := os.WriteFile(caseCodexPath, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CODEX_CLI_PATH", caseCodexPath)
		runProviderPoolCommandCaptureFunc = func(_ context.Context, workdir, executable string, args ...string) (string, string, int, error) {
			execCalls++
			if workdir != caseRoot || executable != caseCodexPath || len(args) == 0 || args[0] != "exec" {
				t.Fatalf("classification exec route = %q %v in %q", executable, args, workdir)
			}
			return "exec fixture", "", 0, nil
		}
		runProviderPoolCodexAppServerPromptFunc = func(context.Context, providerPoolPromptOptions, string, string, *providerPoolAppServerSessionState, uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
			appServerCalls++
			return nil, classification, errors.New("fixture App Server failure")
		}
		beforeExec := execCalls
		response, got, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), providerPoolPromptOptions{Workdir: caseRoot, Prompt: "no replay", SessionID: fmt.Sprintf("classification-%d", classification), SessionAdapter: providerPoolCodexAppServerAdapter}, "gpt-5.5")
		if classification.allowsFallback() {
			if err != nil || response == nil || response.Text != "exec fixture" || got != classification || execCalls != beforeExec+1 {
				t.Fatalf("eligible classification did not fall back exactly once: got=%v response=%+v err=%v exec-before=%d exec-after=%d", got, response, err, beforeExec, execCalls)
			}
		} else if err == nil || got != classification || execCalls != beforeExec {
			t.Fatalf("unsafe classification triggered fallback: got=%v err=%v exec-before=%d exec-after=%d", got, err, beforeExec, execCalls)
		}
	}
}

func TestProviderPoolAppServerEligibleFallbackRemovesExactClaimBeforeOneExec(t *testing.T) {
	originalAppServer := runProviderPoolCodexAppServerPromptFunc
	originalCapture := runProviderPoolCommandCaptureFunc
	originalRemove := removeProviderPoolAppServerTurnClaimFunc
	t.Cleanup(func() {
		runProviderPoolCodexAppServerPromptFunc = originalAppServer
		runProviderPoolCommandCaptureFunc = originalCapture
		removeProviderPoolAppServerTurnClaimFunc = originalRemove
	})

	for _, tc := range []struct {
		name          string
		completedTurn uint64
	}{
		{name: "first turn"},
		{name: "recovered turn", completedTurn: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := providerPoolDurableTestWorkdir(t)
			codexPath := filepath.Join(root, "codex.exe")
			if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CODEX_CLI_PATH", codexPath)
			sessionID := "eligible-" + strings.ReplaceAll(tc.name, " ", "-")
			if _, err := resolveProviderPoolCodexSessionAdapter(root, sessionID, providerPoolCodexAppServerAdapter); err != nil {
				t.Fatal(err)
			}
			bindingPath, err := providerPoolSessionAdapterBindingPath(root, sessionID)
			if err != nil {
				t.Fatal(err)
			}
			bindingBefore, err := os.ReadFile(bindingPath)
			if err != nil {
				t.Fatal(err)
			}

			sessionPath, err := providerPoolCodexAppServerSessionPath(root, sessionID)
			if err != nil {
				t.Fatal(err)
			}
			var sessionBefore []byte
			if tc.completedTurn > 0 {
				state := providerPoolAppServerSessionState{
					Schema:            1,
					SessionID:         sessionID,
					CompletedTurn:     tc.completedTurn,
					ProviderSessionID: "recovered-provider-session",
					RecoveryEvidence:  providerPoolCodexAppServerRecoveryEvidence(sessionID),
					Model:             "gpt-5.5",
					ReasoningEffort:   appserversupervisor.PinnedReasoningEffort,
				}
				sessionBefore, err = json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				sessionBefore = append(sessionBefore, '\n')
				if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(sessionPath, sessionBefore, 0o600); err != nil {
					t.Fatal(err)
				}
			}

			appServerCalls := 0
			execCalls := 0
			expectedTurn := tc.completedTurn + 1
			runProviderPoolCodexAppServerPromptFunc = func(_ context.Context, opts providerPoolPromptOptions, model, executable string, prior *providerPoolAppServerSessionState, turn uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
				appServerCalls++
				if opts.SessionAdapter != providerPoolCodexAppServerAdapter || model != "gpt-5.5" || executable != codexPath || turn != expectedTurn {
					t.Fatalf("App Server fallback attempt = opts=%+v model=%q executable=%q turn=%d", opts, model, executable, turn)
				}
				if tc.completedTurn == 0 && prior != nil {
					t.Fatalf("first turn unexpectedly recovered %+v", prior)
				}
				if tc.completedTurn > 0 && (prior == nil || prior.CompletedTurn != tc.completedTurn || prior.ProviderSessionID != "recovered-provider-session") {
					t.Fatalf("recovered prior state = %+v", prior)
				}
				return nil, providerPoolAppServerEligibleBeforeTurnStart, errors.New("fixture eligible failure")
			}
			runProviderPoolCommandCaptureFunc = func(_ context.Context, workdir, executable string, args ...string) (string, string, int, error) {
				execCalls++
				if workdir != root || executable != codexPath || len(args) == 0 || args[0] != "exec" || args[len(args)-1] != "original prompt" {
					t.Fatalf("fallback exec = %q %v in %q", executable, args, workdir)
				}
				if _, err := os.Stat(sessionPath + ".lock"); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("exec started before exact claim rollback: %v", err)
				}
				return "one-shot exec response", "", 0, nil
			}

			response, eligibility, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), providerPoolPromptOptions{
				Workdir:        root,
				Provider:       "codex",
				Prompt:         "original prompt",
				SessionID:      sessionID,
				SessionAdapter: providerPoolCodexAppServerAdapter,
			}, "gpt-5.5")
			if err != nil || !eligibility.allowsFallback() || response == nil || response.Text != "one-shot exec response" || appServerCalls != 1 || execCalls != 1 {
				t.Fatalf("one-shot fallback = response=%+v eligibility=%v err=%v app-server=%d exec=%d", response, eligibility, err, appServerCalls, execCalls)
			}
			bindingAfter, err := os.ReadFile(bindingPath)
			if err != nil || !bytes.Equal(bindingAfter, bindingBefore) {
				t.Fatalf("adapter binding changed: before=%q after=%q err=%v", bindingBefore, bindingAfter, err)
			}
			if tc.completedTurn == 0 {
				if _, err := os.Stat(sessionPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("first-turn fallback created App Server session state: %v", err)
				}
			} else {
				sessionAfter, err := os.ReadFile(sessionPath)
				if err != nil || !bytes.Equal(sessionAfter, sessionBefore) {
					t.Fatalf("verified-idle state changed: before=%q after=%q err=%v", sessionBefore, sessionAfter, err)
				}
			}
		})
	}
}

func TestProviderPoolAppServerFallbackRejectsNonExactOrUnremovableClaim(t *testing.T) {
	originalAppServer := runProviderPoolCodexAppServerPromptFunc
	originalCapture := runProviderPoolCommandCaptureFunc
	originalRemove := removeProviderPoolAppServerTurnClaimFunc
	t.Cleanup(func() {
		runProviderPoolCodexAppServerPromptFunc = originalAppServer
		runProviderPoolCommandCaptureFunc = originalCapture
		removeProviderPoolAppServerTurnClaimFunc = originalRemove
	})

	cases := []struct {
		name   string
		mutate func(string) error
	}{
		{name: "missing", mutate: func(path string) error { return os.Remove(path) }},
		{name: "malformed", mutate: func(path string) error { return os.WriteFile(path, []byte("{"), 0o600) }},
		{name: "extended", mutate: func(path string) error {
			return os.WriteFile(path, []byte("{\"schema\":1,\"session_id\":\"claim-extended\",\"pending_turn\":1,\"extra\":true}\n"), 0o600)
		}},
		{name: "substituted", mutate: func(path string) error {
			return os.WriteFile(path, []byte("{\"schema\":1,\"session_id\":\"other-session\",\"pending_turn\":1}\n"), 0o600)
		}},
		{name: "stale", mutate: func(path string) error {
			return os.WriteFile(path, []byte("{\"schema\":1,\"session_id\":\"claim-stale\",\"pending_turn\":2}\n"), 0o600)
		}},
		{name: "mismatched", mutate: func(path string) error {
			return os.WriteFile(path, []byte("{\"schema\":2,\"session_id\":\"claim-mismatched\",\"pending_turn\":1}\n"), 0o600)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := providerPoolDurableTestWorkdir(t)
			codexPath := filepath.Join(root, "codex.exe")
			if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CODEX_CLI_PATH", codexPath)
			sessionID := "claim-" + tc.name
			execCalls := 0
			runProviderPoolCommandCaptureFunc = func(context.Context, string, string, ...string) (string, string, int, error) {
				execCalls++
				return "unexpected", "", 0, nil
			}
			runProviderPoolCodexAppServerPromptFunc = func(_ context.Context, _ providerPoolPromptOptions, _ string, _ string, _ *providerPoolAppServerSessionState, _ uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
				path, err := providerPoolCodexAppServerSessionPath(root, sessionID)
				if err != nil {
					t.Fatal(err)
				}
				if err := tc.mutate(path + ".lock"); err != nil {
					t.Fatal(err)
				}
				return nil, providerPoolAppServerEligibleBeforeTurnStart, errors.New("fixture eligible failure")
			}
			_, eligibility, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), providerPoolPromptOptions{Workdir: root, Prompt: "must not replay", SessionID: sessionID, SessionAdapter: providerPoolCodexAppServerAdapter}, "gpt-5.5")
			if err == nil || !eligibility.allowsFallback() || execCalls != 0 {
				t.Fatalf("%s claim did not fail closed: eligibility=%v err=%v exec=%d", tc.name, eligibility, err, execCalls)
			}
		})
	}

	t.Run("removal failure", func(t *testing.T) {
		root := providerPoolDurableTestWorkdir(t)
		codexPath := filepath.Join(root, "codex.exe")
		if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CODEX_CLI_PATH", codexPath)
		execCalls := 0
		runProviderPoolCommandCaptureFunc = func(context.Context, string, string, ...string) (string, string, int, error) {
			execCalls++
			return "unexpected", "", 0, nil
		}
		runProviderPoolCodexAppServerPromptFunc = func(context.Context, providerPoolPromptOptions, string, string, *providerPoolAppServerSessionState, uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
			return nil, providerPoolAppServerEligibleBeforeTurnStart, errors.New("fixture eligible failure")
		}
		removeCalls := 0
		removeProviderPoolAppServerTurnClaimFunc = func(string) error {
			removeCalls++
			return errors.New("fixture removal failure")
		}
		_, eligibility, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), providerPoolPromptOptions{Workdir: root, Prompt: "must not replay", SessionID: "claim-removal-failure", SessionAdapter: providerPoolCodexAppServerAdapter}, "gpt-5.5")
		if err == nil || !strings.Contains(err.Error(), "claim rollback failed") || !eligibility.allowsFallback() || removeCalls != 1 || execCalls != 0 {
			t.Fatalf("removal failure = eligibility=%v err=%v remove=%d exec=%d", eligibility, err, removeCalls, execCalls)
		}
		removeProviderPoolAppServerTurnClaimFunc = originalRemove
	})
}

func TestProviderPoolAppServerUnsafeAndExecAmbiguousOutcomesNeverReplay(t *testing.T) {
	originalAppServer := runProviderPoolCodexAppServerPromptFunc
	originalCapture := runProviderPoolCommandCaptureFunc
	t.Cleanup(func() {
		runProviderPoolCodexAppServerPromptFunc = originalAppServer
		runProviderPoolCommandCaptureFunc = originalCapture
	})

	for _, tc := range []struct {
		name string
		run  *providerPoolAppServerRunResult
		err  error
	}{
		{name: "turn start error", err: errors.New("fixture turn start ambiguity")},
		{name: "consumer failure", run: &providerPoolAppServerRunResult{Response: &providerPoolPromptResponse{Provider: "codex", State: "failed", ExitCode: 1}}},
		{name: "consumer error", err: errors.New("fixture consumer error")},
		{name: "consumer missing result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := providerPoolDurableTestWorkdir(t)
			codexPath := filepath.Join(root, "codex.exe")
			if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CODEX_CLI_PATH", codexPath)
			appServerCalls := 0
			execCalls := 0
			runProviderPoolCodexAppServerPromptFunc = func(context.Context, providerPoolPromptOptions, string, string, *providerPoolAppServerSessionState, uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
				appServerCalls++
				return tc.run, providerPoolAppServerDispatchedOrUnknown, tc.err
			}
			runProviderPoolCommandCaptureFunc = func(context.Context, string, string, ...string) (string, string, int, error) {
				execCalls++
				return "unexpected", "", 0, nil
			}
			sessionID := "unsafe-" + strings.ReplaceAll(tc.name, " ", "-")
			response, eligibility, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), providerPoolPromptOptions{Workdir: root, Prompt: "ambiguous prompt", SessionID: sessionID, SessionAdapter: providerPoolCodexAppServerAdapter}, "gpt-5.5")
			if eligibility != providerPoolAppServerDispatchedOrUnknown || appServerCalls != 1 || execCalls != 0 {
				t.Fatalf("unsafe outcome replayed: response=%+v eligibility=%v err=%v app-server=%d exec=%d", response, eligibility, err, appServerCalls, execCalls)
			}
			path, pathErr := providerPoolCodexAppServerSessionPath(root, sessionID)
			if pathErr != nil {
				t.Fatal(pathErr)
			}
			if _, statErr := os.Stat(path + ".lock"); statErr != nil {
				t.Fatalf("unsafe outcome did not retain unresolved-turn claim: %v", statErr)
			}
		})
	}

	t.Run("ambiguous exec result", func(t *testing.T) {
		root := providerPoolDurableTestWorkdir(t)
		codexPath := filepath.Join(root, "codex.exe")
		if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CODEX_CLI_PATH", codexPath)
		appServerCalls := 0
		execCalls := 0
		runProviderPoolCodexAppServerPromptFunc = func(context.Context, providerPoolPromptOptions, string, string, *providerPoolAppServerSessionState, uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
			appServerCalls++
			return nil, providerPoolAppServerEligibleBeforeTurnStart, errors.New("fixture eligible failure")
		}
		runProviderPoolCommandCaptureFunc = func(context.Context, string, string, ...string) (string, string, int, error) {
			execCalls++
			return "", "", -1, errors.New("fixture ambiguous exec result")
		}
		_, eligibility, err := runProviderPoolCodexPromptWithFallbackEligibility(context.Background(), providerPoolPromptOptions{Workdir: root, Prompt: "one shot only", SessionID: "ambiguous-exec", SessionAdapter: providerPoolCodexAppServerAdapter}, "gpt-5.5")
		if err == nil || !eligibility.allowsFallback() || appServerCalls != 1 || execCalls != 1 {
			t.Fatalf("ambiguous exec result retried: eligibility=%v err=%v app-server=%d exec=%d", eligibility, err, appServerCalls, execCalls)
		}
	})
}

func TestProviderPoolAppServerApprovalSourceRunsOnceAfterExactRequestAndResumesOnlyAfterResolution(t *testing.T) {
	request := providerPoolApprovalFixture("1", providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	controller := providerPoolApprovalController(request, 4)
	controller.completedText = "fixture approved"
	controller.recovery = "recovery-1"
	controller.events <- providerPoolApprovalEvent(1, request)
	controller.events <- providerPoolProgressEvent(2, request, providersession.EventStateChanged, providersession.StateRunning, "approval_resolved")
	controller.events <- providerPoolProgressEvent(3, request, providersession.EventProgress, providersession.StateCompleted, "turn_completed")
	controller.events <- providerPoolProgressEvent(4, request, providersession.EventProgress, providersession.StateReady, "thread_idle")

	calls := 0
	source := func(_ context.Context, got providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
		calls++
		if !reflect.DeepEqual(got, request) || got.Validate() != nil {
			t.Fatalf("decision source request = %+v, want %+v", got, request)
		}
		return providersession.ApprovalDecision{Correlation: got.Correlation, Decision: providersession.DecisionApprove}, true, nil
	}
	result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, source, nil, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(controller.decisions) != 1 || controller.decisions[0].Correlation != request.Correlation {
		t.Fatalf("source calls=%d decisions=%+v", calls, controller.decisions)
	}
	if result == nil || result.Response == nil || result.Response.State != "ready" || result.Response.Text != "fixture approved" || !result.VerifiedIdle || result.RecoveryEvidence != "recovery-1" {
		t.Fatalf("approved result = %+v", result)
	}
}

func TestProviderPoolAppServerApprovalDenialTerminatesWithoutContinuationOrIdle(t *testing.T) {
	request := providerPoolApprovalFixture("1", providersession.ApprovalReasonWorkspaceChange, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	controller := providerPoolApprovalController(request, 3)
	controller.events <- providerPoolApprovalEvent(1, request)
	controller.events <- providerPoolProgressEvent(2, request, providersession.EventProgress, providersession.StateCompleted, "turn_completed")
	controller.events <- providerPoolProgressEvent(3, request, providersession.EventProgress, providersession.StateReady, "thread_idle")
	source := func(_ context.Context, got providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
		return providersession.ApprovalDecision{Correlation: got.Correlation, Decision: providersession.DecisionDeny}, true, nil
	}
	result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, source, nil, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Response == nil || result.Response.State != "failed" || result.VerifiedIdle || result.Response.Metadata["terminal_summary"] != "approval_denied" {
		t.Fatalf("denied result = %+v", result)
	}
	if len(controller.decisions) != 1 || controller.decisions[0].Decision != providersession.DecisionDeny || len(controller.events) != 2 {
		t.Fatalf("denial continued: decisions=%+v pending_events=%d", controller.decisions, len(controller.events))
	}
}

func TestProviderPoolAppServerMissingDecisionPreservesInteractiveFailure(t *testing.T) {
	request := providerPoolApprovalFixture("1", providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	for _, tc := range []struct {
		name   string
		source providerPoolApprovalDecisionSource
	}{
		{name: "production_default"},
		{name: "source_has_no_decision", source: func(context.Context, providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
			return providersession.ApprovalDecision{}, false, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := providerPoolApprovalController(request, 1)
			controller.events <- providerPoolApprovalEvent(1, request)
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, tc.source, nil, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Response.State != "failed" || result.Response.Metadata["terminal_summary"] != "interactive_control_required" || result.VerifiedIdle || len(controller.decisions) != 0 {
				t.Fatalf("missing-decision result = %+v decisions=%+v", result, controller.decisions)
			}
		})
	}
	if (providerPoolPromptOptions{}).approvalDecisionSource != nil {
		t.Fatal("production prompt options unexpectedly have an automatic approval source")
	}
	if (providerPoolPromptOptions{}).cancellationIntentSource != nil {
		t.Fatal("zero-value production prompt options unexpectedly have a cancellation source")
	}
}

func TestProviderPoolAppServerDecisionSourceIsNotCalledWithoutAnExactApprovalRequest(t *testing.T) {
	request := providerPoolApprovalFixture("1", providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	for _, tc := range []struct {
		name  string
		event providersession.Event
	}{
		{name: "malformed_approval", event: func() providersession.Event {
			event := providerPoolApprovalEvent(1, request)
			event.Approval = nil
			return event
		}()},
		{name: "separate_user_input_path", event: providersession.Event{
			ContractVersion: providersession.ContractVersion,
			Sequence:        1,
			OccurredAt:      time.Now().UTC(),
			Session:         providersession.SessionRef{Provider: "codex", SessionID: request.Correlation.SessionID},
			Kind:            providersession.EventUserInputRequested,
			State:           providersession.StateWaitingForUserInput,
			Correlation:     request.Correlation,
			Summary:         "user_input_requested",
			UserInput:       &providersession.UserInputRequest{Correlation: request.Correlation, PromptRef: request.Correlation.RequestID},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := providerPoolApprovalController(request, 1)
			controller.events <- tc.event
			calls := 0
			source := func(context.Context, providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
				calls++
				return providersession.ApprovalDecision{}, false, nil
			}
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, source, nil, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 0 || result == nil || result.Response == nil || result.Response.State != "failed" || result.VerifiedIdle {
				t.Fatalf("non-approval result = %+v calls=%d", result, calls)
			}
		})
	}
}

func TestProviderPoolAppServerRejectsReplayedSubstitutedAndCrossRequestDecisions(t *testing.T) {
	request := providerPoolApprovalFixture("1", providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	for _, tc := range []struct {
		name   string
		mutate func(*providersession.ApprovalDecision)
		state  providersession.State
	}{
		{name: "malformed", mutate: func(decision *providersession.ApprovalDecision) { decision.Decision = "unknown" }, state: providersession.StateWaitingForApproval},
		{name: "substituted_connection", mutate: func(decision *providersession.ApprovalDecision) {
			decision.Correlation.ConnectionID = "connection-other"
		}, state: providersession.StateWaitingForApproval},
		{name: "cross_session", mutate: func(decision *providersession.ApprovalDecision) { decision.Correlation.SessionID = "session-other" }, state: providersession.StateWaitingForApproval},
		{name: "cross_turn", mutate: func(decision *providersession.ApprovalDecision) { decision.Correlation.InteractionID = "turn-other" }, state: providersession.StateWaitingForApproval},
		{name: "cross_item", mutate: func(decision *providersession.ApprovalDecision) { decision.Correlation.ActivityID = "item-other" }, state: providersession.StateWaitingForApproval},
		{name: "cross_request", mutate: func(decision *providersession.ApprovalDecision) { decision.Correlation.RequestID = "request-other" }, state: providersession.StateWaitingForApproval},
		{name: "cross_decision", mutate: func(decision *providersession.ApprovalDecision) { decision.Correlation.DecisionID = "decision-other" }, state: providersession.StateWaitingForApproval},
		{name: "stale_process", mutate: func(decision *providersession.ApprovalDecision) {
			decision.Correlation.ProcessIncarnationID = "process-old"
		}, state: providersession.StateWaitingForApproval},
		{name: "post_disconnect", mutate: func(*providersession.ApprovalDecision) {}, state: providersession.StateDisconnected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := providerPoolApprovalController(request, 1)
			controller.state = tc.state
			controller.events <- providerPoolApprovalEvent(1, request)
			source := func(_ context.Context, got providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
				decision := providersession.ApprovalDecision{Correlation: got.Correlation, Decision: providersession.DecisionApprove}
				tc.mutate(&decision)
				return decision, true, nil
			}
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, source, nil, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Response.Metadata["terminal_summary"] != "approval_decision_rejected" || result.VerifiedIdle || len(controller.decisions) != 0 {
				t.Fatalf("rejected result = %+v decisions=%+v", result, controller.decisions)
			}
		})
	}

	controller := providerPoolApprovalController(request, 2)
	controller.events <- providerPoolApprovalEvent(1, request)
	controller.events <- providerPoolApprovalEvent(2, request)
	calls := 0
	source := func(_ context.Context, got providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
		calls++
		return providersession.ApprovalDecision{Correlation: got.Correlation, Decision: providersession.DecisionApprove}, true, nil
	}
	result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, source, nil, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Response.Metadata["terminal_summary"] != "approval_request_replayed" || calls != 1 || len(controller.decisions) != 1 || result.VerifiedIdle {
		t.Fatalf("replay result = %+v calls=%d decisions=%+v", result, calls, controller.decisions)
	}
}

func TestProviderPoolAppServerConsumerCannotWidenPinnedDecisionSet(t *testing.T) {
	request := providerPoolApprovalFixture("1", providersession.ApprovalReasonPermission, []string{providersession.DecisionDeny})
	controller := providerPoolApprovalController(request, 1)
	controller.events <- providerPoolApprovalEvent(1, request)
	source := func(_ context.Context, got providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
		got.AllowedDecisions = append(got.AllowedDecisions, providersession.DecisionApprove)
		return providersession.ApprovalDecision{Correlation: got.Correlation, Decision: providersession.DecisionApprove}, true, nil
	}
	result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, source, nil, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Response.Metadata["terminal_summary"] != "approval_decision_rejected" || result.VerifiedIdle || len(controller.decisions) != 0 || !reflect.DeepEqual(request.AllowedDecisions, []string{providersession.DecisionDeny}) {
		t.Fatalf("widened result = %+v decisions=%+v request=%+v", result, controller.decisions, request)
	}
}

func TestProviderPoolAppServerUserInputSourceHandlesExactNeutralPromptsAndWaitsForResolution(t *testing.T) {
	fixtures := []struct {
		name     string
		prompt   providersession.UserInputPrompt
		response providersession.UserInputResponse
	}{
		{
			name:     "select_one",
			prompt:   providersession.UserInputPrompt{Kind: providersession.UserInputPromptSelectOne, Summary: "Choose one safe option", Options: []providersession.UserInputOption{{OptionRef: "option-a", Label: "Option A"}, {OptionRef: "option-b", Label: "Option B"}}, MaxSelections: 1},
			response: providersession.UserInputResponse{SelectedOptionRefs: []string{"option-b"}},
		},
		{
			name:     "select_many",
			prompt:   providersession.UserInputPrompt{Kind: providersession.UserInputPromptSelectMany, Summary: "Choose bounded safe options", Options: []providersession.UserInputOption{{OptionRef: "option-a", Label: "Option A"}, {OptionRef: "option-b", Label: "Option B"}, {OptionRef: "option-c", Label: "Option C"}}, MaxSelections: 2},
			response: providersession.UserInputResponse{SelectedOptionRefs: []string{"option-a", "option-c"}},
		},
		{
			name:     "text",
			prompt:   providersession.UserInputPrompt{Kind: providersession.UserInputPromptText, Summary: "Provide a short safe answer", MaxTextBytes: 32},
			response: providersession.UserInputResponse{Text: "bounded answer"},
		},
	}
	for index, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			request := providerPoolUserInputFixture(fmt.Sprintf("exact-%d", index))
			fixture.prompt.Correlation, fixture.prompt.PromptRef = request.Correlation, request.PromptRef
			fixture.response.Correlation, fixture.response.PromptRef = request.Correlation, request.PromptRef
			controller := providerPoolUserInputController(fixture.prompt, 4)
			controller.completedText = "fixture completed"
			controller.recovery = "recovery-exact"
			controller.events <- providerPoolUserInputEvent(1, request)
			controller.events <- providerPoolUserInputProgressEvent(2, request, providersession.EventStateChanged, providersession.StateRunning, "user_input_resolved")
			controller.events <- providerPoolUserInputProgressEvent(3, request, providersession.EventProgress, providersession.StateCompleted, "turn_completed")
			controller.events <- providerPoolUserInputProgressEvent(4, request, providersession.EventProgress, providersession.StateReady, "thread_idle")

			calls := 0
			source := func(_ context.Context, got providersession.UserInputPrompt) (providersession.UserInputResponse, bool, error) {
				calls++
				if !reflect.DeepEqual(got, fixture.prompt) || got.ValidateFor(request) != nil {
					t.Fatalf("source prompt = %+v, want %+v", got, fixture.prompt)
				}
				got.Correlation.SessionID = "mutated-session"
				got.PromptRef = "mutated-prompt"
				got.Kind = providersession.UserInputPromptText
				got.MaxSelections = 99
				got.MaxTextBytes = 99999
				got.Options = append(got.Options, providersession.UserInputOption{OptionRef: "option-injected", Label: "Injected"})
				return fixture.response, true, nil
			}
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || len(controller.userInputResponses) != 1 || !reflect.DeepEqual(controller.userInputResponses[0], fixture.response) {
				t.Fatalf("source calls=%d delivered=%+v", calls, controller.userInputResponses)
			}
			if result == nil || result.Response == nil || result.Response.State != "ready" || result.Response.Text != "fixture completed" || !result.VerifiedIdle || result.RecoveryEvidence != "recovery-exact" {
				t.Fatalf("resolved result = %+v", result)
			}
			if !reflect.DeepEqual(controller.userInputPrompt, fixture.prompt) {
				t.Fatalf("source mutation changed retained prompt: got %+v want %+v", controller.userInputPrompt, fixture.prompt)
			}
		})
	}
}

func TestProviderPoolAppServerUserInputMissingSourceAndMissingResponsePreserveInteractiveFailure(t *testing.T) {
	request := providerPoolUserInputFixture("missing")
	prompt := providerPoolSelectOnePrompt(request)
	for _, tc := range []struct {
		name   string
		source providerPoolUserInputResponseSource
	}{
		{name: "production_default"},
		{name: "source_has_no_response", source: func(context.Context, providersession.UserInputPrompt) (providersession.UserInputResponse, bool, error) {
			return providersession.UserInputResponse{}, false, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := providerPoolUserInputController(prompt, 1)
			controller.events <- providerPoolUserInputEvent(1, request)
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, nil, tc.source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Response.Metadata["terminal_summary"] != "interactive_control_required" || result.VerifiedIdle || len(controller.userInputResponses) != 0 {
				t.Fatalf("missing response result = %+v delivered=%+v", result, controller.userInputResponses)
			}
		})
	}
	if (providerPoolPromptOptions{}).userInputResponseSource != nil {
		t.Fatal("production prompt options unexpectedly have a user-input response source")
	}
	if (providerPoolPromptOptions{}).cancellationIntentSource != nil {
		t.Fatal("user-input-only production options unexpectedly have a cancellation source")
	}
}

func TestProviderPoolAppServerUserInputErrorsFailClosedWithBoundedClasses(t *testing.T) {
	request := providerPoolUserInputFixture("errors")
	validPrompt := providerPoolSelectOnePrompt(request)
	validResponse := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{"option-a"}}
	fixtures := []struct {
		name        string
		configure   func(*fakeProviderPoolAppServerController)
		source      providerPoolUserInputResponseSource
		wantSummary string
	}{
		{name: "prompt_lookup_error", configure: func(c *fakeProviderPoolAppServerController) {
			c.userInputPromptErr = errors.New("private lookup failure")
		}, source: fixedProviderPoolUserInputSource(validResponse), wantSummary: "user_input_prompt_unavailable"},
		{name: "invalid_prompt", configure: func(c *fakeProviderPoolAppServerController) { c.userInputPrompt.PromptRef = "substituted-prompt" }, source: fixedProviderPoolUserInputSource(validResponse), wantSummary: "user_input_prompt_rejected"},
		{name: "source_error", source: func(context.Context, providersession.UserInputPrompt) (providersession.UserInputResponse, bool, error) {
			return providersession.UserInputResponse{}, false, errors.New("private source failure")
		}, wantSummary: "user_input_response_unavailable"},
		{name: "stale_controller", configure: func(c *fakeProviderPoolAppServerController) { c.state = providersession.StateRunning }, source: fixedProviderPoolUserInputSource(validResponse), wantSummary: "user_input_response_rejected"},
		{name: "delivery_error", configure: func(c *fakeProviderPoolAppServerController) {
			c.respondUserInputErr = errors.New("private delivery failure")
		}, source: fixedProviderPoolUserInputSource(validResponse), wantSummary: "user_input_response_delivery_failed"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			controller := providerPoolUserInputController(validPrompt, 1)
			if fixture.configure != nil {
				fixture.configure(controller)
			}
			controller.events <- providerPoolUserInputEvent(1, request)
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, nil, fixture.source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Response.Metadata["terminal_summary"] != fixture.wantSummary || result.VerifiedIdle {
				t.Fatalf("error result = %+v", result)
			}
		})
	}
}

func TestProviderPoolAppServerRejectsInvalidUserInputResponsesBeforeDelivery(t *testing.T) {
	request := providerPoolUserInputFixture("invalid-response")
	selectPrompt := providerPoolSelectOnePrompt(request)
	textPrompt := providersession.UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Kind: providersession.UserInputPromptText, Summary: "Provide bounded text", MaxTextBytes: 8}
	fixtures := []struct {
		name     string
		prompt   providersession.UserInputPrompt
		response providersession.UserInputResponse
	}{
		{name: "unknown_option", prompt: selectPrompt, response: providersession.UserInputResponse{SelectedOptionRefs: []string{"option-unknown"}}},
		{name: "duplicated_option", prompt: providersession.UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Kind: providersession.UserInputPromptSelectMany, Summary: "Choose up to two", Options: selectPrompt.Options, MaxSelections: 2}, response: providersession.UserInputResponse{SelectedOptionRefs: []string{"option-a", "option-a"}}},
		{name: "over_limit", prompt: selectPrompt, response: providersession.UserInputResponse{SelectedOptionRefs: []string{"option-a", "option-b"}}},
		{name: "empty_text", prompt: textPrompt, response: providersession.UserInputResponse{Text: "   "}},
		{name: "oversized_text", prompt: textPrompt, response: providersession.UserInputResponse{Text: "123456789"}},
		{name: "control_text", prompt: textPrompt, response: providersession.UserInputResponse{Text: "bad\x00text"}},
		{name: "text_for_choice", prompt: selectPrompt, response: providersession.UserInputResponse{Text: "wrong kind"}},
		{name: "choice_for_text", prompt: textPrompt, response: providersession.UserInputResponse{SelectedOptionRefs: []string{"option-a"}}},
		{name: "substituted_prompt", prompt: selectPrompt, response: providersession.UserInputResponse{PromptRef: "prompt-other", SelectedOptionRefs: []string{"option-a"}}},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			response := fixture.response
			response.Correlation = request.Correlation
			if response.PromptRef == "" {
				response.PromptRef = request.PromptRef
			}
			controller := providerPoolUserInputController(fixture.prompt, 1)
			controller.events <- providerPoolUserInputEvent(1, request)
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, nil, fixedProviderPoolUserInputSource(response), "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Response.Metadata["terminal_summary"] != "user_input_response_rejected" || result.VerifiedIdle || len(controller.userInputResponses) != 0 {
				t.Fatalf("invalid response result = %+v delivered=%+v", result, controller.userInputResponses)
			}
		})
	}
}

func TestProviderPoolAppServerRejectsSubstitutedUserInputCorrelationAndReplay(t *testing.T) {
	request := providerPoolUserInputFixture("correlation")
	prompt := providerPoolSelectOnePrompt(request)
	base := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{"option-a"}}
	fixtures := []struct {
		name   string
		mutate func(*providersession.UserInputResponse)
		state  providersession.State
	}{
		{name: "stale_process", mutate: func(r *providersession.UserInputResponse) { r.Correlation.ProcessIncarnationID = "process-old" }, state: providersession.StateWaitingForUserInput},
		{name: "substituted_connection", mutate: func(r *providersession.UserInputResponse) { r.Correlation.ConnectionID = "connection-other" }, state: providersession.StateWaitingForUserInput},
		{name: "cross_session", mutate: func(r *providersession.UserInputResponse) { r.Correlation.SessionID = "session-other" }, state: providersession.StateWaitingForUserInput},
		{name: "cross_turn", mutate: func(r *providersession.UserInputResponse) { r.Correlation.InteractionID = "turn-other" }, state: providersession.StateWaitingForUserInput},
		{name: "cross_item", mutate: func(r *providersession.UserInputResponse) { r.Correlation.ActivityID = "item-other" }, state: providersession.StateWaitingForUserInput},
		{name: "cross_request", mutate: func(r *providersession.UserInputResponse) { r.Correlation.RequestID = "request-other" }, state: providersession.StateWaitingForUserInput},
		{name: "cross_decision", mutate: func(r *providersession.UserInputResponse) { r.Correlation.DecisionID = "decision-other" }, state: providersession.StateWaitingForUserInput},
		{name: "post_disconnect", mutate: func(*providersession.UserInputResponse) {}, state: providersession.StateDisconnected},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			controller := providerPoolUserInputController(prompt, 1)
			controller.state = fixture.state
			controller.events <- providerPoolUserInputEvent(1, request)
			response := base
			fixture.mutate(&response)
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, nil, fixedProviderPoolUserInputSource(response), "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Response.Metadata["terminal_summary"] != "user_input_response_rejected" || len(controller.userInputResponses) != 0 || result.VerifiedIdle {
				t.Fatalf("correlation result = %+v delivered=%+v", result, controller.userInputResponses)
			}
		})
	}

	controller := providerPoolUserInputController(prompt, 2)
	controller.events <- providerPoolUserInputEvent(1, request)
	controller.events <- providerPoolUserInputEvent(2, request)
	calls := 0
	source := func(context.Context, providersession.UserInputPrompt) (providersession.UserInputResponse, bool, error) {
		calls++
		return base, true, nil
	}
	result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Response.Metadata["terminal_summary"] != "user_input_request_replayed" || calls != 1 || len(controller.userInputResponses) != 1 || result.VerifiedIdle {
		t.Fatalf("replay result = %+v calls=%d delivered=%+v", result, calls, controller.userInputResponses)
	}
}

func TestProviderPoolAppServerRejectsMalformedUserInputBeforeSource(t *testing.T) {
	request := providerPoolUserInputFixture("malformed")
	prompt := providerPoolSelectOnePrompt(request)
	fixtures := []struct {
		name            string
		event           providersession.Event
		providerSession string
	}{
		{name: "missing_request", event: func() providersession.Event { e := providerPoolUserInputEvent(1, request); e.UserInput = nil; return e }(), providerSession: request.Correlation.SessionID},
		{name: "wrong_state", event: func() providersession.Event {
			e := providerPoolUserInputEvent(1, request)
			e.State = providersession.StateRunning
			return e
		}(), providerSession: request.Correlation.SessionID},
		{name: "substituted_event_correlation", event: func() providersession.Event {
			e := providerPoolUserInputEvent(1, request)
			e.Correlation.RequestID = "request-other"
			return e
		}(), providerSession: request.Correlation.SessionID},
		{name: "substituted_event_session", event: func() providersession.Event {
			e := providerPoolUserInputEvent(1, request)
			e.Session.SessionID = "session-other"
			return e
		}(), providerSession: request.Correlation.SessionID},
		{name: "cross_live_session", event: providerPoolUserInputEvent(1, request), providerSession: "session-other"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			controller := providerPoolUserInputController(prompt, 1)
			controller.events <- fixture.event
			calls := 0
			source := func(context.Context, providersession.UserInputPrompt) (providersession.UserInputResponse, bool, error) {
				calls++
				return providersession.UserInputResponse{}, false, nil
			}
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, fixture.providerSession, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Response.Metadata["terminal_summary"] != "user_input_request_rejected" || calls != 0 || len(controller.userInputResponses) != 0 || result.VerifiedIdle {
				t.Fatalf("malformed result = %+v calls=%d delivered=%+v", result, calls, controller.userInputResponses)
			}
		})
	}
}

func TestProviderPoolAppServerUserInputRequiresExactResolutionAndDoesNotLeakContent(t *testing.T) {
	request := providerPoolUserInputFixture("resolution")
	prompt := providerPoolSelectOnePrompt(request)
	response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{"option-a"}}
	for _, tc := range []struct {
		name       string
		resolution providersession.Event
	}{
		{name: "missing", resolution: providerPoolUserInputProgressEvent(2, request, providersession.EventProgress, providersession.StateCompleted, "turn_completed")},
		{name: "mismatched", resolution: func() providersession.Event {
			e := providerPoolUserInputProgressEvent(2, request, providersession.EventStateChanged, providersession.StateRunning, "user_input_resolved")
			e.Correlation.RequestID = "request-other"
			return e
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := providerPoolUserInputController(prompt, 3)
			controller.events <- providerPoolUserInputEvent(1, request)
			controller.events <- tc.resolution
			controller.events <- providerPoolUserInputProgressEvent(3, request, providersession.EventProgress, providersession.StateReady, "thread_idle")
			result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), controller, nil, fixedProviderPoolUserInputSource(response), "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if result.Response.Metadata["terminal_summary"] != "user_input_resolution_missing" || result.VerifiedIdle || len(controller.userInputResponses) != 1 {
				t.Fatalf("resolution result = %+v delivered=%+v", result, controller.userInputResponses)
			}
			encoded, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			for _, private := range []string{prompt.Summary, prompt.Options[0].Label, prompt.Options[0].OptionRef, response.SelectedOptionRefs[0], response.Text} {
				if private != "" && strings.Contains(string(encoded), private) {
					t.Fatalf("transient prompt or response leaked into result: %s", encoded)
				}
			}
		})
	}
}

func TestProviderPoolAppServerInteractiveSourcesRemainSeparate(t *testing.T) {
	approval := providerPoolApprovalFixture("separate", providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny})
	approvalController := providerPoolApprovalController(approval, 1)
	approvalController.events <- providerPoolApprovalEvent(1, approval)
	userCalls := 0
	userSource := func(context.Context, providersession.UserInputPrompt) (providersession.UserInputResponse, bool, error) {
		userCalls++
		return providersession.UserInputResponse{}, false, nil
	}
	result, err := consumeProviderPoolCodexAppServerTurn(context.Background(), approvalController, nil, userSource, "gpt-5.5", providersession.EffectivePolicySnapshot{}, approval.Correlation.SessionID, 1, false)
	if err != nil || result.Response.Metadata["terminal_summary"] != "interactive_control_required" || userCalls != 0 {
		t.Fatalf("approval used user-input source: result=%+v err=%v calls=%d", result, err, userCalls)
	}

	request := providerPoolUserInputFixture("separate")
	inputController := providerPoolUserInputController(providerPoolSelectOnePrompt(request), 1)
	inputController.events <- providerPoolUserInputEvent(1, request)
	approvalCalls := 0
	approvalSource := func(context.Context, providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
		approvalCalls++
		return providersession.ApprovalDecision{}, false, nil
	}
	result, err = consumeProviderPoolCodexAppServerTurn(context.Background(), inputController, approvalSource, nil, "gpt-5.5", providersession.EffectivePolicySnapshot{}, request.Correlation.SessionID, 1, false)
	if err != nil || result.Response.Metadata["terminal_summary"] != "interactive_control_required" || approvalCalls != 0 {
		t.Fatalf("user input used approval source: result=%+v err=%v calls=%d", result, err, approvalCalls)
	}
}

func TestProviderPoolAppServerCancellationSourceStartsOnceAfterExactTurnAndJoins(t *testing.T) {
	scope := providerPoolCancellationFixture("scope")
	controller := providerPoolCancellationController(2)
	started := make(chan providerPoolCancellationScope, 1)
	joined := make(chan struct{})
	source := func(ctx context.Context, got providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
		started <- got
		<-ctx.Done()
		close(joined)
		return providersession.CancellationIntent{}, false, ctx.Err()
	}
	done := make(chan *providerPoolAppServerRunResult, 1)
	go func() {
		result, _ := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
		done <- result
	}()
	controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
	got := <-started
	if got != scope {
		t.Fatalf("source scope = %+v, want %+v", got, scope)
	}
	close(controller.events)
	result := <-done
	if result.Response.Metadata["terminal_summary"] != "transport_closed" || result.VerifiedIdle {
		t.Fatalf("closure result = %+v", result)
	}
	select {
	case <-joined:
	default:
		t.Fatal("event-channel closure did not cancel and join the source")
	}
}

func TestProviderPoolAppServerCancellationContextCancellationJoinsSource(t *testing.T) {
	scope := providerPoolCancellationFixture("context")
	controller := providerPoolCancellationController(1)
	started := make(chan struct{})
	joined := make(chan struct{})
	source := func(ctx context.Context, _ providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
		close(started)
		<-ctx.Done()
		close(joined)
		return providersession.CancellationIntent{}, false, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *providerPoolAppServerRunResult, 1)
	go func() {
		result, _ := consumeProviderPoolCodexAppServerTurnWithCancellation(ctx, controller, nil, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
		done <- result
	}()
	controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
	<-started
	cancel()
	result := <-done
	if result.Response.Metadata["terminal_summary"] != "cancellation_source_cancelled" || result.VerifiedIdle {
		t.Fatalf("context result = %+v", result)
	}
	select {
	case <-joined:
	default:
		t.Fatal("parent cancellation did not join the source")
	}
}

func TestProviderPoolAppServerCancellationScopeIsNeutralAndDefensivelyCopied(t *testing.T) {
	typeOfScope := reflect.TypeOf(providerPoolCancellationScope{})
	if typeOfScope.NumField() != 2 || typeOfScope.Field(0).Name != "Session" || typeOfScope.Field(1).Name != "Correlation" {
		t.Fatalf("cancellation scope fields = %v", typeOfScope)
	}
	scope := providerPoolCancellationFixture("copy")
	controller := providerPoolCancellationController(3)
	controller.onCancel = func(intent providersession.CancellationIntent) {
		controller.events <- providerPoolCancellationRequestEvent(2, intent)
		controller.events <- providerPoolCancellationTerminalEvent(3, intent)
	}
	source := func(_ context.Context, got providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
		intent := providersession.CancellationIntent{Session: got.Session, Correlation: got.Correlation, Reason: providersession.CancellationReasonUserRequested}
		got.Session.SessionID = "mutated-session"
		got.Correlation.InteractionID = "mutated-turn"
		return intent, true, nil
	}
	controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
	result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
	if err != nil || result.Response.State != "cancelled" || len(controller.cancellationIntents) != 1 || controller.cancellationIntents[0].Session != scope.Session || controller.cancellationIntents[0].Correlation != scope.Correlation {
		t.Fatalf("defensive copy result=%+v err=%v intents=%+v", result, err, controller.cancellationIntents)
	}
}

func TestProviderPoolAppServerCancellationFoundFalseAllowsOriginalCompletion(t *testing.T) {
	scope := providerPoolCancellationFixture("not-found")
	controller := providerPoolCancellationController(3)
	controller.completedText = "original completion"
	controller.recovery = "recovery-not-found"
	controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
	controller.events <- providerPoolCancellationProgressEvent(2, scope, providersession.EventProgress, providersession.StateCompleted, "turn_completed")
	controller.events <- providerPoolCancellationProgressEvent(3, scope, providersession.EventProgress, providersession.StateReady, "thread_idle")
	calls := 0
	source := func(context.Context, providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
		calls++
		return providersession.CancellationIntent{}, false, nil
	}
	result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
	if err != nil || calls != 1 || result.Response.State != "ready" || result.Response.Text != "original completion" || !result.VerifiedIdle || len(controller.cancellationIntents) != 0 {
		t.Fatalf("found=false result=%+v err=%v calls=%d intents=%+v", result, err, calls, controller.cancellationIntents)
	}
}

func TestProviderPoolAppServerCancellationAcceptsClosedReasonsAndExactTerminalSequence(t *testing.T) {
	for _, reason := range []string{providersession.CancellationReasonUserRequested, providersession.CancellationReasonSafetyStop, providersession.CancellationReasonDeadline} {
		t.Run(reason, func(t *testing.T) {
			scope := providerPoolCancellationFixture(reason)
			controller := providerPoolCancellationController(4)
			controller.onCancel = func(intent providersession.CancellationIntent) {
				controller.events <- providerPoolCancellationRequestEvent(2, intent)
				controller.events <- providerPoolCancellationProgressEvent(3, scope, providersession.EventProgress, "", "background_process_risk_possible")
				controller.events <- providerPoolCancellationTerminalEvent(4, intent)
			}
			controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
			result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, fixedProviderPoolCancellationSource(scope, reason), "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
			if err != nil || result.Response.State != "cancelled" || result.Response.Metadata["terminal_summary"] != "cancelled" || result.Response.Metadata["outcome_unknown"] != false || len(result.Response.Metadata) != 2 || result.VerifiedIdle || result.RecoveryEvidence != "" || len(controller.cancellationIntents) != 1 {
				t.Fatalf("cancelled result=%+v err=%v intents=%+v", result, err, controller.cancellationIntents)
			}
		})
	}
}

func TestProviderPoolAppServerCancellationDeliveryDoesNotCompleteChat(t *testing.T) {
	scope := providerPoolCancellationFixture("delivery-only")
	controller := providerPoolCancellationController(3)
	delivered := make(chan providersession.CancellationIntent, 1)
	controller.onCancel = func(intent providersession.CancellationIntent) {
		controller.events <- providerPoolCancellationRequestEvent(2, intent)
		delivered <- intent
	}
	controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
	done := make(chan *providerPoolAppServerRunResult, 1)
	go func() {
		result, _ := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, fixedProviderPoolCancellationSource(scope, providersession.CancellationReasonUserRequested), "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
		done <- result
	}()
	intent := <-delivered
	select {
	case result := <-done:
		t.Fatalf("delivery completed chat early: %+v", result)
	default:
	}
	controller.events <- providerPoolCancellationTerminalEvent(3, intent)
	result := <-done
	if result.Response.State != "cancelled" || result.VerifiedIdle {
		t.Fatalf("terminal result = %+v", result)
	}
}

func TestProviderPoolAppServerCancellationRejectsInvalidOrSubstitutedIntentsBeforeDelivery(t *testing.T) {
	scope := providerPoolCancellationFixture("intent-reject")
	base := providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonUserRequested}
	fixtures := []struct {
		name   string
		mutate func(*providersession.CancellationIntent)
	}{
		{name: "malformed", mutate: func(i *providersession.CancellationIntent) { i.Session = providersession.SessionRef{} }},
		{name: "unknown_reason", mutate: func(i *providersession.CancellationIntent) { i.Reason = "display-label" }},
		{name: "cross_process", mutate: func(i *providersession.CancellationIntent) { i.Correlation.ProcessIncarnationID = "process-other" }},
		{name: "cross_connection", mutate: func(i *providersession.CancellationIntent) { i.Correlation.ConnectionID = "connection-other" }},
		{name: "cross_session", mutate: func(i *providersession.CancellationIntent) {
			i.Session.SessionID, i.Correlation.SessionID = "session-other", "session-other"
		}},
		{name: "substituted_session_ref", mutate: func(i *providersession.CancellationIntent) { i.Session.SessionID = "session-other" }},
		{name: "cross_turn", mutate: func(i *providersession.CancellationIntent) { i.Correlation.InteractionID = "turn-other" }},
		{name: "expanded_activity", mutate: func(i *providersession.CancellationIntent) { i.Correlation.ActivityID = "item-other" }},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			controller := providerPoolCancellationController(1)
			controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
			intent := base
			fixture.mutate(&intent)
			source := func(context.Context, providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
				return intent, true, nil
			}
			result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
			if err != nil || result.Response.Metadata["terminal_summary"] != "cancellation_intent_rejected" || len(result.Response.Metadata) != 2 || result.VerifiedIdle || len(controller.cancellationIntents) != 0 {
				t.Fatalf("rejection result=%+v err=%v intents=%+v", result, err, controller.cancellationIntents)
			}
			encoded, marshalErr := json.Marshal(result)
			if marshalErr != nil || strings.Contains(string(encoded), "intent-reject") || strings.Contains(string(encoded), "process-other") || strings.Contains(string(encoded), "session-other") || strings.Contains(string(encoded), "turn-other") {
				t.Fatalf("private cancellation content leaked: %s err=%v", encoded, marshalErr)
			}
		})
	}
}

func TestProviderPoolAppServerCancellationRejectsMalformedScopesBeforeSource(t *testing.T) {
	base := providerPoolCancellationFixture("scope-reject")
	fixtures := []struct {
		name   string
		mutate func(*providersession.Event)
	}{
		{name: "missing_process", mutate: func(e *providersession.Event) { e.Correlation.ProcessIncarnationID = "" }},
		{name: "malformed_connection", mutate: func(e *providersession.Event) { e.Correlation.ConnectionID = " connection-other" }},
		{name: "cross_session", mutate: func(e *providersession.Event) { e.Session.SessionID = "session-other" }},
		{name: "cross_turn_shape", mutate: func(e *providersession.Event) { e.Correlation.ActivityID = "item-other" }},
		{name: "wrong_provider", mutate: func(e *providersession.Event) { e.Session.Provider = "claude" }},
		{name: "substituted_state", mutate: func(e *providersession.Event) { e.State = providersession.StateRunning }},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			controller := providerPoolCancellationController(1)
			event := providerPoolCancellationTurnStartedEvent(1, base)
			fixture.mutate(&event)
			controller.events <- event
			calls := 0
			source := func(context.Context, providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
				calls++
				return providersession.CancellationIntent{}, false, nil
			}
			result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, base.Session.SessionID, 1, false)
			if err != nil || result.Response.Metadata["terminal_summary"] != "cancellation_scope_rejected" || calls != 0 || len(controller.cancellationIntents) != 0 {
				t.Fatalf("scope result=%+v err=%v calls=%d intents=%+v", result, err, calls, controller.cancellationIntents)
			}
		})
	}
}

func TestProviderPoolAppServerCancellationSourceAndControllerFailuresAreBounded(t *testing.T) {
	scope := providerPoolCancellationFixture("failures")
	for _, fixture := range []struct {
		name        string
		configure   func(*fakeProviderPoolAppServerController)
		source      providerPoolCancellationIntentSource
		wantSummary string
	}{
		{name: "source_error", source: func(context.Context, providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
			return providersession.CancellationIntent{}, false, errors.New("private source failure")
		}, wantSummary: "cancellation_intent_unavailable"},
		{name: "cancel_error", configure: func(c *fakeProviderPoolAppServerController) { c.cancelErr = errors.New("private cancel failure") }, source: fixedProviderPoolCancellationSource(scope, providersession.CancellationReasonDeadline), wantSummary: "cancellation_delivery_failed"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			controller := providerPoolCancellationController(1)
			if fixture.configure != nil {
				fixture.configure(controller)
			}
			controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
			result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, fixture.source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
			wantCalls := 0
			if fixture.name == "cancel_error" {
				wantCalls = 1
			}
			if err != nil || result.Response.Metadata["terminal_summary"] != fixture.wantSummary || len(controller.cancellationIntents) != wantCalls || result.VerifiedIdle {
				t.Fatalf("failure result=%+v err=%v intents=%+v", result, err, controller.cancellationIntents)
			}
		})
	}
	t.Run("state_drift_before_cancel", func(t *testing.T) {
		controller := providerPoolCancellationController(1)
		controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
		source := func(context.Context, providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
			controller.state = providersession.StateWaitingForApproval
			return providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonSafetyStop}, true, nil
		}
		result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
		if err != nil || result.Response.Metadata["terminal_summary"] != "cancellation_intent_rejected" || len(controller.cancellationIntents) != 0 || result.VerifiedIdle {
			t.Fatalf("state drift result=%+v err=%v intents=%+v", result, err, controller.cancellationIntents)
		}
	})
}

func TestProviderPoolAppServerCancellationRejectsRequestAndTerminalProtocolDrift(t *testing.T) {
	scope := providerPoolCancellationFixture("protocol")
	fixtures := []struct {
		name        string
		emit        func(*fakeProviderPoolAppServerController, providersession.CancellationIntent)
		wantSummary string
	}{
		{name: "malformed_request", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			e := providerPoolCancellationRequestEvent(2, i)
			e.Cancellation = nil
			c.events <- e
		}, wantSummary: "cancellation_request_rejected"},
		{name: "mismatched_request", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			e := providerPoolCancellationRequestEvent(2, i)
			e.Correlation.ConnectionID = "connection-other"
			c.events <- e
		}, wantSummary: "cancellation_request_rejected"},
		{name: "duplicate_request", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			c.events <- providerPoolCancellationRequestEvent(2, i)
			c.events <- providerPoolCancellationRequestEvent(3, i)
		}, wantSummary: "cancellation_request_replayed"},
		{name: "terminal_before_request", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			c.events <- providerPoolCancellationTerminalEvent(2, i)
		}, wantSummary: "cancellation_terminal_rejected"},
		{name: "mismatched_terminal", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			c.events <- providerPoolCancellationRequestEvent(2, i)
			e := providerPoolCancellationTerminalEvent(3, i)
			e.Correlation.InteractionID = "turn-other"
			c.events <- e
		}, wantSummary: "cancellation_terminal_rejected"},
		{name: "non_cancelled_terminal", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			c.events <- providerPoolCancellationRequestEvent(2, i)
			c.events <- providerPoolCancellationProgressEvent(3, scope, providersession.EventStateChanged, providersession.StateFailed, "turn_failed")
		}, wantSummary: "cancellation_terminal_missing"},
		{name: "completion_pending", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			c.events <- providerPoolCancellationRequestEvent(2, i)
			c.events <- providerPoolCancellationProgressEvent(3, scope, providersession.EventProgress, providersession.StateCompleted, "turn_completed")
		}, wantSummary: "cancellation_terminal_missing"},
		{name: "idle_pending", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			c.events <- providerPoolCancellationRequestEvent(2, i)
			c.events <- providerPoolCancellationProgressEvent(3, scope, providersession.EventProgress, providersession.StateReady, "thread_idle")
		}, wantSummary: "cancellation_terminal_missing"},
		{name: "missing_terminal", emit: func(c *fakeProviderPoolAppServerController, i providersession.CancellationIntent) {
			c.events <- providerPoolCancellationRequestEvent(2, i)
			close(c.events)
		}, wantSummary: "cancellation_terminal_missing"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			controller := providerPoolCancellationController(4)
			controller.onCancel = func(intent providersession.CancellationIntent) { fixture.emit(controller, intent) }
			controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
			result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, fixedProviderPoolCancellationSource(scope, providersession.CancellationReasonUserRequested), "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
			if err != nil || result.Response.Metadata["terminal_summary"] != fixture.wantSummary || result.VerifiedIdle || len(controller.cancellationIntents) != 1 {
				t.Fatalf("protocol result=%+v err=%v intents=%+v", result, err, controller.cancellationIntents)
			}
		})
	}
}

func TestProviderPoolAppServerCancellationRequestBeforeIntentAndScopeReplayFailClosed(t *testing.T) {
	scope := providerPoolCancellationFixture("replay")
	for _, fixture := range []struct {
		name        string
		events      []providersession.Event
		wantSummary string
	}{
		{name: "request_before_intent", events: []providersession.Event{providerPoolCancellationTurnStartedEvent(1, scope), providerPoolCancellationRequestEvent(2, providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: providersession.CancellationReasonUserRequested})}, wantSummary: "cancellation_request_unexpected"},
		{name: "replayed_scope", events: []providersession.Event{providerPoolCancellationTurnStartedEvent(1, scope), providerPoolCancellationTurnStartedEvent(2, scope)}, wantSummary: "cancellation_scope_rejected"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			controller := providerPoolCancellationController(len(fixture.events))
			for _, event := range fixture.events {
				controller.events <- event
			}
			source := func(ctx context.Context, _ providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
				<-ctx.Done()
				return providersession.CancellationIntent{}, false, ctx.Err()
			}
			result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, nil, nil, source, "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
			if err != nil || result.Response.Metadata["terminal_summary"] != fixture.wantSummary || len(controller.cancellationIntents) != 0 {
				t.Fatalf("replay result=%+v err=%v intents=%+v", result, err, controller.cancellationIntents)
			}
		})
	}
}

func TestProviderPoolAppServerCancellationRemainsIndependentFromApprovalAndUserInput(t *testing.T) {
	scope := providerPoolCancellationFixture("independent")
	controller := providerPoolCancellationController(3)
	controller.onCancel = func(intent providersession.CancellationIntent) {
		controller.events <- providerPoolCancellationRequestEvent(2, intent)
		controller.events <- providerPoolCancellationTerminalEvent(3, intent)
	}
	approvalCalls, inputCalls := 0, 0
	approvalSource := func(context.Context, providersession.ApprovalRequest) (providersession.ApprovalDecision, bool, error) {
		approvalCalls++
		return providersession.ApprovalDecision{}, false, nil
	}
	inputSource := func(context.Context, providersession.UserInputPrompt) (providersession.UserInputResponse, bool, error) {
		inputCalls++
		return providersession.UserInputResponse{}, false, nil
	}
	controller.events <- providerPoolCancellationTurnStartedEvent(1, scope)
	result, err := consumeProviderPoolCodexAppServerTurnWithCancellation(context.Background(), controller, approvalSource, inputSource, fixedProviderPoolCancellationSource(scope, providersession.CancellationReasonUserRequested), "gpt-5.5", providersession.EffectivePolicySnapshot{}, scope.Session.SessionID, 1, false)
	if err != nil || result.Response.State != "cancelled" || approvalCalls != 0 || inputCalls != 0 || len(controller.decisions) != 0 || len(controller.userInputResponses) != 0 {
		t.Fatalf("independence result=%+v err=%v approvals=%d inputs=%d", result, err, approvalCalls, inputCalls)
	}
}

func providerPoolCancellationFixture(suffix string) providerPoolCancellationScope {
	return providerPoolCancellationScope{
		Session:     providersession.SessionRef{Provider: "codex", SessionID: "session-" + suffix},
		Correlation: providersession.Correlation{ProcessIncarnationID: "process-" + suffix, ConnectionID: "connection-" + suffix, SessionID: "session-" + suffix, InteractionID: "turn-" + suffix},
	}
}

func providerPoolCancellationController(capacity int) *fakeProviderPoolAppServerController {
	return &fakeProviderPoolAppServerController{events: make(chan providersession.Event, capacity), state: providersession.StateRunning}
}

func providerPoolCancellationTurnStartedEvent(sequence uint64, scope providerPoolCancellationScope) providersession.Event {
	return providerPoolCancellationProgressEvent(sequence, scope, providersession.EventProgress, "", "turn_started")
}

func providerPoolCancellationRequestEvent(sequence uint64, intent providersession.CancellationIntent) providersession.Event {
	retained := intent
	return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: sequence, OccurredAt: time.Now().UTC(), Session: intent.Session, Kind: providersession.EventCancellationRequested, Correlation: intent.Correlation, Summary: "cancellation_requested", Cancellation: &retained}
}

func providerPoolCancellationTerminalEvent(sequence uint64, intent providersession.CancellationIntent) providersession.Event {
	return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: sequence, OccurredAt: time.Now().UTC(), Session: intent.Session, Kind: providersession.EventStateChanged, State: providersession.StateCancelled, Correlation: intent.Correlation, Summary: "cancelled"}
}

func providerPoolCancellationProgressEvent(sequence uint64, scope providerPoolCancellationScope, kind providersession.EventKind, state providersession.State, summary string) providersession.Event {
	return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: sequence, OccurredAt: time.Now().UTC(), Session: scope.Session, Kind: kind, State: state, Correlation: scope.Correlation, Summary: summary}
}

func fixedProviderPoolCancellationSource(scope providerPoolCancellationScope, reason string) providerPoolCancellationIntentSource {
	return func(context.Context, providerPoolCancellationScope) (providersession.CancellationIntent, bool, error) {
		return providersession.CancellationIntent{Session: scope.Session, Correlation: scope.Correlation, Reason: reason}, true, nil
	}
}

func providerPoolUserInputFixture(suffix string) providersession.UserInputRequest {
	correlation := providerPoolApprovalFixture(suffix, providersession.ApprovalReasonCommandExecution, []string{providersession.DecisionApprove, providersession.DecisionDeny}).Correlation
	return providersession.UserInputRequest{Correlation: correlation, PromptRef: "prompt-" + suffix}
}

func providerPoolSelectOnePrompt(request providersession.UserInputRequest) providersession.UserInputPrompt {
	return providersession.UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Kind: providersession.UserInputPromptSelectOne, Summary: "Choose one bounded option", Options: []providersession.UserInputOption{{OptionRef: "option-a", Label: "Option A"}, {OptionRef: "option-b", Label: "Option B"}}, MaxSelections: 1}
}

func providerPoolUserInputController(prompt providersession.UserInputPrompt, capacity int) *fakeProviderPoolAppServerController {
	return &fakeProviderPoolAppServerController{events: make(chan providersession.Event, capacity), state: providersession.StateWaitingForUserInput, userInputPrompt: cloneProviderPoolUserInputPrompt(prompt)}
}

func providerPoolUserInputEvent(sequence uint64, request providersession.UserInputRequest) providersession.Event {
	cloned := request
	return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: sequence, OccurredAt: time.Now().UTC(), Session: providersession.SessionRef{Provider: "codex", SessionID: request.Correlation.SessionID}, Kind: providersession.EventUserInputRequested, State: providersession.StateWaitingForUserInput, Correlation: request.Correlation, Summary: "user_input_requested", UserInput: &cloned}
}

func providerPoolUserInputProgressEvent(sequence uint64, request providersession.UserInputRequest, kind providersession.EventKind, state providersession.State, summary string) providersession.Event {
	return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: sequence, OccurredAt: time.Now().UTC(), Session: providersession.SessionRef{Provider: "codex", SessionID: request.Correlation.SessionID}, Kind: kind, State: state, Correlation: request.Correlation, Summary: summary}
}

func fixedProviderPoolUserInputSource(response providersession.UserInputResponse) providerPoolUserInputResponseSource {
	return func(context.Context, providersession.UserInputPrompt) (providersession.UserInputResponse, bool, error) {
		return cloneProviderPoolUserInputResponse(response), true, nil
	}
}

func providerPoolApprovalFixture(suffix, reason string, allowed []string) providersession.ApprovalRequest {
	return providersession.ApprovalRequest{
		Correlation: providersession.Correlation{
			ProcessIncarnationID: "process-" + suffix,
			ConnectionID:         "connection-" + suffix,
			SessionID:            "session-" + suffix,
			InteractionID:        "turn-" + suffix,
			ActivityID:           "item-" + suffix,
			RequestID:            "request-" + suffix,
			DecisionID:           "decision-" + suffix,
		},
		Reason:           reason,
		AllowedDecisions: append([]string(nil), allowed...),
	}
}

func providerPoolApprovalController(request providersession.ApprovalRequest, capacity int) *fakeProviderPoolAppServerController {
	return &fakeProviderPoolAppServerController{events: make(chan providersession.Event, capacity), state: providersession.StateWaitingForApproval}
}

func providerPoolApprovalEvent(sequence uint64, request providersession.ApprovalRequest) providersession.Event {
	cloned := cloneProviderPoolApprovalRequest(request)
	return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: sequence, OccurredAt: time.Now().UTC(), Session: providersession.SessionRef{Provider: "codex", SessionID: request.Correlation.SessionID}, Kind: providersession.EventApprovalRequested, State: providersession.StateWaitingForApproval, Correlation: request.Correlation, Summary: "approval_requested", Approval: &cloned}
}

func providerPoolProgressEvent(sequence uint64, request providersession.ApprovalRequest, kind providersession.EventKind, state providersession.State, summary string) providersession.Event {
	return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: sequence, OccurredAt: time.Now().UTC(), Session: providersession.SessionRef{Provider: "codex", SessionID: request.Correlation.SessionID}, Kind: kind, State: state, Correlation: request.Correlation, Summary: summary}
}

func TestProviderPoolCodexCLIPathPrefersBundledBeforePath(t *testing.T) {
	root := t.TempDir()
	bundled := filepath.Join(root, ".vscode", "extensions", "openai.chatgpt-test", "bin", "windows-x86_64", "codex.exe")
	if err := os.MkdirAll(filepath.Dir(bundled), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundled, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_CLI_PATH", "")
	t.Setenv("CLAUDE_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", root)
	got, err := providerPoolCodexCLIPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != bundled {
		t.Fatalf("got %q want bundled %q", got, bundled)
	}
}

func TestProviderPoolPromptWarmWaitTimeout(t *testing.T) {
	t.Setenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS", "")
	if got := providerPoolPromptWarmWaitTimeout(); got != 10*time.Second {
		t.Fatalf("default timeout = %s, want 10s", got)
	}

	t.Setenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS", "0")
	if got := providerPoolPromptWarmWaitTimeout(); got != 0 {
		t.Fatalf("zero timeout = %s, want 0", got)
	}

	t.Setenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS", "3")
	if got := providerPoolPromptWarmWaitTimeout(); got != 3*time.Second {
		t.Fatalf("seconds timeout = %s, want 3s", got)
	}

	t.Setenv("DORKPIPE_PROVIDER_POOL_PROMPT_WAIT_SECONDS", "1500ms")
	if got := providerPoolPromptWarmWaitTimeout(); got != 1500*time.Millisecond {
		t.Fatalf("duration timeout = %s, want 1500ms", got)
	}
}

func TestProviderPoolLeaseHonorsEffectiveMaxActive(t *testing.T) {
	root := t.TempDir()
	provider := providerPoolProvider{
		ID:          "codex",
		DisplayName: "Codex",
		Pool: providerPoolProviderShape{
			MaxActive: 2,
		},
	}
	release1, queued, err := acquireProviderPoolLease(context.Background(), root, provider, "session-1", "reviewer", "run-1", "node-1", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if queued {
		t.Fatal("first lease queued unexpectedly")
	}
	defer release1()
	release2, queued, err := acquireProviderPoolLease(context.Background(), root, provider, "session-2", "reviewer", "run-1", "node-2", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if queued {
		t.Fatal("second lease queued unexpectedly")
	}
	defer release2()
	release3, queued, err := acquireProviderPoolLease(context.Background(), root, provider, "session-3", "reviewer", "run-1", "node-3", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if release3 != nil {
		defer release3()
	}
	if !queued {
		t.Fatal("third lease should queue at max_active=2")
	}
	release2()
	release4, queued, err := acquireProviderPoolLease(context.Background(), root, provider, "session-4", "reviewer", "run-1", "node-4", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if queued {
		t.Fatal("lease after release queued unexpectedly")
	}
	release4()
}

func TestProviderPoolLeasePolicyCanNarrowMaxActive(t *testing.T) {
	root := t.TempDir()
	provider := providerPoolProvider{
		ID:          "claude",
		DisplayName: "Claude",
		Pool: providerPoolProviderShape{
			MaxActive: 3,
		},
	}
	release, queued, err := acquireProviderPoolLease(context.Background(), root, provider, "session-1", "reviewer", "run-1", "node-1", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if queued {
		t.Fatal("first narrowed lease queued unexpectedly")
	}
	defer release()
	_, queued, err = acquireProviderPoolLease(context.Background(), root, provider, "session-2", "reviewer", "run-1", "node-2", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !queued {
		t.Fatal("second narrowed lease should queue at requested max_active=1")
	}
}

func TestProviderPoolWorkdirHashCandidatesIncludeWindowsStyleNormalizations(t *testing.T) {
	workdir := `C:\Source\DockPipe`
	candidates := providerPoolWorkdirCanonicalCandidates(workdir)
	if !containsAll(strings.Join(candidates, "\x00"),
		`c:\source\dockpipe`,
		`c:/source/dockpipe`,
	) {
		t.Fatalf("windows-style candidates missing normalized variants: %#v", candidates)
	}

	lowerHash := providerPoolWorkdirHash(`C:\Source\DockPipe`)
	upperHash := providerPoolWorkdirHash(`C:\SOURCE\DOCKPIPE`)
	if lowerHash != upperHash {
		t.Fatalf("windows-style hash should be case-insensitive: %q != %q", lowerHash, upperHash)
	}

	hashes := providerPoolWorkdirHashCandidates(workdir)
	if len(hashes) < 2 {
		t.Fatalf("expected multiple hash candidates for windows-style path, got %#v", hashes)
	}
}

func TestStopProviderPoolsReportsMultipleClaudeWorkers(t *testing.T) {
	root := t.TempDir()
	catalogPath := filepath.Join(root, "catalog.yml")
	catalog := `schema: 1
providers:
  - id: claude
    display_name: Claude
    pool:
      warm_mode: guarded_container
      warm_source: docker-claude-resolver
`
	if err := os.WriteFile(catalogPath, []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DORKPIPE_PROVIDER_POOL_CATALOG", catalogPath)

	oldStop := stopProviderPoolClaudeWorkersFunc
	stopProviderPoolClaudeWorkersFunc = func(context.Context, string) ([]string, error) {
		return []string{"worker-a", "worker-b"}, nil
	}
	t.Cleanup(func() { stopProviderPoolClaudeWorkersFunc = oldStop })

	resp, err := stopProviderPools(context.Background(), root, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Providers) != 1 {
		t.Fatalf("provider count = %d, want 1", len(resp.Providers))
	}
	status := resp.Providers[0]
	if status.State != "stopped" {
		t.Fatalf("state = %q, want stopped", status.State)
	}
	if !reflect.DeepEqual(status.StoppedWorkers, []string{"worker-a", "worker-b"}) {
		t.Fatalf("stopped workers = %#v", status.StoppedWorkers)
	}
	if !strings.Contains(status.Status, "Removed 2 worker(s)") {
		t.Fatalf("status did not report multiple workers: %q", status.Status)
	}
}
func TestProviderPoolCodexOutputFailedDetectsZeroExitErrors(t *testing.T) {
	if !providerPoolCodexOutputFailed("[2026-07-09T15:50:15] ERROR: unexpected status 400 Bad Request") {
		t.Fatal("expected Codex ERROR line to be treated as failed")
	}
	if providerPoolCodexOutputFailed("provider-pool-direct-smoke") {
		t.Fatal("normal output should not be treated as failed")
	}
}

func TestProviderPoolClaudeWarmBootstrapScriptUsesAllowlistAndPortableKeepalive(t *testing.T) {
	script := providerPoolClaudeWarmBootstrapScript()
	for _, name := range []string{
		".credentials.json",
		".last-cleanup",
		"history.jsonl",
		"ide",
		"mcp-needs-auth-cache.json",
		"plans",
		"plugins",
		"skills",
		"projects",
		"session-env",
		"sessions",
		"settings.json",
		"shell-snapshots",
		"policy-limits.json",
		"remote-settings.json",
	} {
		if !strings.Contains(script, name) {
			t.Fatalf("bootstrap script missing allowlisted path %q", name)
		}
	}
	if strings.Contains(script, "runuser") {
		t.Fatalf("bootstrap script should not depend on runuser: %s", script)
	}
	if !strings.Contains(script, "while :; do sleep 3600; done") {
		t.Fatalf("bootstrap script missing keepalive loop: %s", script)
	}
}

func TestProviderPoolClaudePromptDockerArgsDoNotKeepStdinOpen(t *testing.T) {
	args := providerPoolClaudePromptDockerArgs("worker", "sonnet", "hello")
	if len(args) == 0 {
		t.Fatal("expected docker args")
	}
	if args[0] != "exec" {
		t.Fatalf("first arg = %q, want exec", args[0])
	}
	for _, arg := range args {
		if arg == "-i" {
			t.Fatalf("claude prompt args should not keep stdin open: %v", args)
		}
	}
	if !reflect.DeepEqual(args[:8], []string{"exec", "-u", "node", "-e", "HOME=/home/node", "-w", "/work", "worker"}) {
		t.Fatalf("unexpected docker exec prefix: %v", args)
	}
	if !containsAll(strings.Join(args, "\x00"), "claude", "--dangerously-skip-permissions", "--model", "sonnet", "-p", "hello") {
		t.Fatalf("unexpected claude args: %v", args)
	}
}

func TestProviderPoolClaudeStreamWorkerModeCanBeDisabledExplicitly(t *testing.T) {
	t.Setenv("DORKPIPE_PROVIDER_POOL_CLAUDE_STREAM_WORKER", "")
	if !providerPoolClaudeStreamWorkerEnabled() {
		t.Fatal("stream worker should be enabled by default")
	}
	if got := providerPoolClaudeWorkerMode(); got != "stream_worker" {
		t.Fatalf("mode = %q, want stream_worker", got)
	}

	t.Setenv("DORKPIPE_PROVIDER_POOL_CLAUDE_STREAM_WORKER", "single_prompt")
	if providerPoolClaudeStreamWorkerEnabled() {
		t.Fatal("stream worker should be disabled by explicit single_prompt mode")
	}
	if got := providerPoolClaudeWorkerMode(); got != "single_prompt" {
		t.Fatalf("mode = %q, want single_prompt", got)
	}
}

func TestProviderPoolClaudeStreamDaemonArgsUseGenericWorkerBoundary(t *testing.T) {
	args := providerPoolClaudeStreamDaemonDockerArgs("worker", "/tmp/dorkpipe-provider-pool/claude.sock", "sonnet")
	if len(args) == 0 {
		t.Fatal("expected docker args")
	}
	if !reflect.DeepEqual(args[:8], []string{"exec", "-d", "-u", "node", "-e", "HOME=/home/node", "-w", "/work"}) {
		t.Fatalf("unexpected daemon docker prefix: %v", args)
	}
	joined := strings.Join(args, "\x00")
	if !containsAll(joined,
		"worker",
		"node",
		"--input-format', 'stream-json'",
		"--output-format', 'stream-json'",
		"--include-partial-messages",
		"--replay-user-messages",
		"--verbose",
		"/tmp/dorkpipe-provider-pool/claude.sock",
		"sonnet",
	) {
		t.Fatalf("unexpected daemon args: %v", args)
	}
}

func TestProviderPoolClaudeStreamClientArgsUseUnixSocket(t *testing.T) {
	args := providerPoolClaudeStreamClientDockerArgs("worker", "/tmp/dorkpipe-provider-pool/claude.sock", "hello", "turn-1")
	if !reflect.DeepEqual(args[:7], []string{"exec", "-u", "node", "-e", "HOME=/home/node", "-w", "/work"}) {
		t.Fatalf("unexpected client docker prefix: %v", args)
	}
	joined := strings.Join(args, "\x00")
	if !containsAll(joined, "worker", "node", "createConnection", "/tmp/dorkpipe-provider-pool/claude.sock", "hello", "turn-1") {
		t.Fatalf("unexpected client args: %v", args)
	}
}

func TestMergePromptTimingsPreservesProviderTiming(t *testing.T) {
	dst := map[string]int64{"status_ms": 7}
	mergePromptTimings(dst, map[string]any{
		"claude_command_ms": float64(25),
	})
	if dst["status_ms"] != 7 {
		t.Fatalf("status timing changed: %v", dst)
	}
	if dst["claude_command_ms"] != 25 {
		t.Fatalf("claude command timing missing: %v", dst)
	}
}

func TestProviderPoolCodexSessionAdapterPinsFirstExplicitChoice(t *testing.T) {
	root := providerPoolDurableTestWorkdir(t)
	adapter, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-1", "")
	if err != nil || adapter != providerPoolCodexExecAdapter {
		t.Fatalf("unselected adapter = %q, err=%v", adapter, err)
	}
	path, err := providerPoolSessionAdapterBindingPath(root, "pipeon-session-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing adapter choice created state: %v", err)
	}

	adapter, err = resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-1", providerPoolCodexExecAdapter)
	if err != nil || adapter != providerPoolCodexExecAdapter {
		t.Fatalf("first explicit adapter = %q, err=%v", adapter, err)
	}
	adapter, err = resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-1", providerPoolCodexExecAdapter)
	if err != nil || adapter != providerPoolCodexExecAdapter {
		t.Fatalf("repeated adapter = %q, err=%v", adapter, err)
	}
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-1", ""); err == nil {
		t.Fatal("pinned session accepted an omitted adapter choice")
	}
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-1", providerPoolCodexAppServerAdapter); err == nil {
		t.Fatal("pinned session accepted adapter drift")
	}
}

func TestProviderPoolCodexSessionAdapterPinsUnavailableAppServerWithoutSubstitution(t *testing.T) {
	root := providerPoolDurableTestWorkdir(t)
	adapter, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-2", providerPoolCodexAppServerAdapter)
	if err != nil || adapter != providerPoolCodexAppServerAdapter {
		t.Fatalf("App Server adapter = %q, err=%v", adapter, err)
	}
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-2", providerPoolCodexExecAdapter); err == nil {
		t.Fatal("App Server selection silently substituted exec")
	}
}

func TestProviderPoolCodexSessionAdapterRejectsInvalidOrMalformedEvidence(t *testing.T) {
	root := providerPoolDurableTestWorkdir(t)
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "", providerPoolCodexExecAdapter); err == nil {
		t.Fatal("explicit adapter without a session id was accepted")
	}
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-3", "unknown"); err == nil {
		t.Fatal("unknown adapter was accepted")
	}
	path, err := providerPoolSessionAdapterBindingPath(root, "pipeon-session-4")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":1,"session_id":"pipeon-session-4","adapter":"codex_exec","extra":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-4", providerPoolCodexExecAdapter); err == nil {
		t.Fatal("extended adapter binding was accepted")
	}
}

func TestProviderPoolRecoveryAuthoritySurvivesMigrationRestartAndRetainsNoReplay(t *testing.T) {
	root := providerPoolDurableTestWorkdir(t)
	sessionID := "migrated-app-server-session"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(sessionID)))
	legacy := filepath.Join(root, infrastructure.DockpipeDirRel, "packages", "dorkpipe", "provider-pools")
	binding := providerPoolSessionAdapterBinding{Schema: 1, SessionID: sessionID, Adapter: providerPoolCodexAppServerAdapter}
	state := providerPoolAppServerSessionState{Schema: 1, SessionID: sessionID, CompletedTurn: 7, ProviderSessionID: "thread-migrated", RecoveryEvidence: providerPoolCodexAppServerRecoveryEvidence(sessionID), Model: "gpt-5.5", ReasoningEffort: appserversupervisor.PinnedReasoningEffort}
	claim := providerPoolAppServerTurnClaim{Schema: 1, SessionID: sessionID, PendingTurn: 8}
	fixtures := map[string][]byte{
		"sessions.json": []byte("{\"pipeon-resume\":\"provider-resume\"}\n"),
		filepath.Join("session-adapters", digest+".json"):            mustProviderPoolRecoveryCandidateJSON(t, binding),
		filepath.Join("app-server", "sessions", digest+".json"):      mustProviderPoolRecoveryCandidateJSON(t, state),
		filepath.Join("app-server", "sessions", digest+".json.lock"): mustProviderPoolRecoveryCandidateJSON(t, claim),
		filepath.Join("app-server", "snapshots", "restart.json"):     []byte("{\"snapshot\":true}\n"),
		filepath.Join("app-server", "audit", "restart.audit.json"):   []byte("{\"audit\":true}\n"),
		filepath.Join("app-server", "aggregates", "restart.json"):    []byte("{\"aggregate\":true}\n"),
	}
	for rel, raw := range fixtures {
		path := filepath.Join(legacy, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bindings, err := loadProviderPoolCodexBindings(root)
	if err != nil || bindings["pipeon-resume"] != "provider-resume" {
		t.Fatalf("migrated resume binding = %v, %v", bindings, err)
	}
	if adapter, err := resolveProviderPoolCodexSessionAdapter(root, sessionID, providerPoolCodexAppServerAdapter); err != nil || adapter != providerPoolCodexAppServerAdapter {
		t.Fatalf("migrated immutable adapter = %q, %v", adapter, err)
	}
	if _, err := resolveProviderPoolCodexSessionAdapter(root, sessionID, providerPoolCodexExecAdapter); err == nil {
		t.Fatal("migrated adapter pin allowed substitution")
	}
	if _, _, _, err := beginProviderPoolCodexAppServerTurn(root, sessionID, state.Model, state.ReasoningEffort); err == nil || !strings.Contains(err.Error(), "unresolved prior turn") {
		t.Fatalf("migrated unresolved claim did not retain no-replay authority: %v", err)
	}

	legacyAfter := legacy + "-detached"
	if err := os.Rename(legacy, legacyAfter); err != nil {
		t.Fatal(err)
	}
	bindings, err = loadProviderPoolCodexBindings(root)
	if err != nil || bindings["pipeon-resume"] != "provider-resume" {
		t.Fatalf("restart lost durable resume binding = %v, %v", bindings, err)
	}
	if _, _, _, err := beginProviderPoolCodexAppServerTurn(root, sessionID, state.Model, state.ReasoningEffort); err == nil || !strings.Contains(err.Error(), "unresolved prior turn") {
		t.Fatalf("restart lost durable no-replay claim: %v", err)
	}
	sessionPath, err := providerPoolCodexAppServerSessionPath(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	durableProviderRoot := filepath.Dir(filepath.Dir(filepath.Dir(sessionPath)))
	for _, rel := range []string{
		filepath.Join("app-server", "snapshots", "restart.json"),
		filepath.Join("app-server", "audit", "restart.audit.json"),
		filepath.Join("app-server", "aggregates", "restart.json"),
	} {
		if _, err := os.Stat(filepath.Join(durableProviderRoot, rel)); err != nil {
			t.Fatalf("restart lost %s: %v", rel, err)
		}
	}
	if err := os.WriteFile(filepath.Join(durableProviderRoot, "sessions.json"), []byte("{\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProviderPoolCodexBindings(root); err == nil {
		t.Fatal("malformed durable resume authority was treated as an empty mapping")
	}
}

func TestProviderPoolAppServerRecoveryCandidateRequiresExactReadOnlyEvidence(t *testing.T) {
	originalAppServer := runProviderPoolCodexAppServerPromptFunc
	originalCapture := runProviderPoolCommandCaptureFunc
	originalRemove := removeProviderPoolAppServerTurnClaimFunc
	originalSupervisor := newProviderPoolAppServerSupervisorFunc
	t.Cleanup(func() {
		runProviderPoolCodexAppServerPromptFunc = originalAppServer
		runProviderPoolCommandCaptureFunc = originalCapture
		removeProviderPoolAppServerTurnClaimFunc = originalRemove
		newProviderPoolAppServerSupervisorFunc = originalSupervisor
	})
	runProviderPoolCodexAppServerPromptFunc = func(context.Context, providerPoolPromptOptions, string, string, *providerPoolAppServerSessionState, uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
		t.Fatal("recovery-candidate classification started an App Server prompt")
		return nil, providerPoolAppServerDispatchedOrUnknown, errors.New("unexpected prompt")
	}
	runProviderPoolCommandCaptureFunc = func(context.Context, string, string, ...string) (string, string, int, error) {
		t.Fatal("recovery-candidate classification started a provider or exec command")
		return "", "", -1, errors.New("unexpected command")
	}
	removeProviderPoolAppServerTurnClaimFunc = func(string) error {
		t.Fatal("recovery-candidate classification removed a claim")
		return errors.New("unexpected claim removal")
	}
	newProviderPoolAppServerSupervisorFunc = func(providerPoolAppServerSupervisorSpec) (providerPoolAppServerSupervisor, error) {
		t.Fatal("recovery-candidate classification created a supervisor or child")
		return nil, errors.New("unexpected supervisor")
	}

	fixture := writeProviderPoolRecoveryCandidateFixture(t, "candidate-exact", 7)
	if err := os.WriteFile(filepath.Join(fixture.root, "display-and-response-noise.json"), []byte(`{"appServerStatus":{"state":"completed"},"terminal_summary":"turn_completed","messages":["ready"],"provider_available":true,"catalog_order":["codex"],"authenticated":true,"response_text":"recovered"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DORKPIPE_PROVIDER_POOL_CODEX_ADAPTER", providerPoolCodexExecAdapter)
	before := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root)
	for attempt := 0; attempt < 2; attempt++ {
		if got := classifyProviderPoolCodexAppServerRecoveryCandidate(fixture.root, fixture.sessionID, providerPoolCodexAppServerAdapter); got != providerPoolAppServerRecoveryCandidate {
			t.Fatalf("classification attempt %d = %v", attempt+1, got)
		}
	}
	after := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("classification changed package state: before=%q after=%q", before, after)
	}
}

func TestProviderPoolAppServerRecoveryCandidateFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		session string
		adapter string
	}{
		{name: "empty session", session: "", adapter: providerPoolCodexAppServerAdapter},
		{name: "trimmed session substitution", session: " candidate-exact ", adapter: providerPoolCodexAppServerAdapter},
		{name: "empty adapter", session: "candidate-exact", adapter: ""},
		{name: "exec adapter", session: "candidate-exact", adapter: providerPoolCodexExecAdapter},
		{name: "extended adapter", session: "candidate-exact", adapter: providerPoolCodexAppServerAdapter + "-extended"},
		{name: "unknown adapter", session: "candidate-exact", adapter: "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := writeProviderPoolRecoveryCandidateFixture(t, "candidate-exact", 7)
			if got := classifyProviderPoolCodexAppServerRecoveryCandidate(fixture.root, tc.session, tc.adapter); got != providerPoolAppServerRecoveryNotCandidate {
				t.Fatalf("classification = %v", got)
			}
		})
	}

	cases := []struct {
		name   string
		mutate func(*testing.T, providerPoolRecoveryCandidateFixture)
	}{
		{name: "binding missing", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustRemoveRecoveryCandidateFixture(t, f.bindingPath)
		}},
		{name: "binding malformed", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.bindingPath, []byte("{"))
		}},
		{name: "binding extended", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.bindingPath, []byte(fmt.Sprintf(`{"schema":1,"session_id":%q,"adapter":"codex_app_server","extra":true}`+"\n", f.sessionID)))
		}},
		{name: "binding substituted", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.bindingPath, mustProviderPoolRecoveryCandidateJSON(t, providerPoolSessionAdapterBinding{Schema: 1, SessionID: f.sessionID, Adapter: providerPoolCodexExecAdapter}))
		}},
		{name: "binding cross session", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.bindingPath, mustProviderPoolRecoveryCandidateJSON(t, providerPoolSessionAdapterBinding{Schema: 1, SessionID: "other-session", Adapter: providerPoolCodexAppServerAdapter}))
		}},
		{name: "binding stale schema", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.bindingPath, mustProviderPoolRecoveryCandidateJSON(t, providerPoolSessionAdapterBinding{Schema: 2, SessionID: f.sessionID, Adapter: providerPoolCodexAppServerAdapter}))
		}},
		{name: "binding reordered", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.bindingPath, []byte(fmt.Sprintf(`{"adapter":"codex_app_server","session_id":%q,"schema":1}`+"\n", f.sessionID)))
		}},
		{name: "binding oversized", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.bindingPath, []byte(strings.Repeat("x", providerPoolRecoveryCandidateMaxEvidenceBytes+1)))
		}},
		{name: "state missing", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustRemoveRecoveryCandidateFixture(t, f.statePath)
		}},
		{name: "state malformed", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.statePath, []byte("{"))
		}},
		{name: "state extended", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.statePath, append(bytes.TrimSuffix(f.stateRaw, []byte("}\n")), []byte(",\"extra\":true}\n")...))
		}},
		{name: "state cross session", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			state := f.state
			state.SessionID = "other-session"
			mustWriteRecoveryCandidateFixture(t, f.statePath, mustProviderPoolRecoveryCandidateJSON(t, state))
		}},
		{name: "state zero completed turn", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			state := f.state
			state.CompletedTurn = 0
			mustWriteRecoveryCandidateFixture(t, f.statePath, mustProviderPoolRecoveryCandidateJSON(t, state))
		}},
		{name: "state turn overflow", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			state := f.state
			state.CompletedTurn = ^uint64(0)
			mustWriteRecoveryCandidateFixture(t, f.statePath, mustProviderPoolRecoveryCandidateJSON(t, state))
			claim := f.claim
			claim.PendingTurn = 0
			mustWriteRecoveryCandidateFixture(t, f.claimPath, mustProviderPoolRecoveryCandidateJSON(t, claim))
		}},
		{name: "state invalid provider session", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			state := f.state
			state.ProviderSessionID = ""
			mustWriteRecoveryCandidateFixture(t, f.statePath, mustProviderPoolRecoveryCandidateJSON(t, state))
		}},
		{name: "state substituted recovery", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			state := f.state
			state.RecoveryEvidence = "other-recovery"
			mustWriteRecoveryCandidateFixture(t, f.statePath, mustProviderPoolRecoveryCandidateJSON(t, state))
		}},
		{name: "state substituted model", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			state := f.state
			state.Model = " gpt-5.5"
			mustWriteRecoveryCandidateFixture(t, f.statePath, mustProviderPoolRecoveryCandidateJSON(t, state))
		}},
		{name: "state substituted reasoning", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			state := f.state
			state.ReasoningEffort = "high "
			mustWriteRecoveryCandidateFixture(t, f.statePath, mustProviderPoolRecoveryCandidateJSON(t, state))
		}},
		{name: "state oversized", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.statePath, []byte(strings.Repeat("x", providerPoolRecoveryCandidateMaxEvidenceBytes+1)))
		}},
		{name: "claim missing", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustRemoveRecoveryCandidateFixture(t, f.claimPath)
		}},
		{name: "claim malformed", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.claimPath, []byte("{"))
		}},
		{name: "claim extended", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.claimPath, []byte(fmt.Sprintf(`{"schema":1,"session_id":%q,"pending_turn":8,"extra":true}`+"\n", f.sessionID)))
		}},
		{name: "claim duplicated", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.claimPath, []byte(fmt.Sprintf(`{"schema":1,"schema":1,"session_id":%q,"pending_turn":8}`+"\n", f.sessionID)))
		}},
		{name: "claim reordered", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.claimPath, []byte(fmt.Sprintf(`{"pending_turn":8,"session_id":%q,"schema":1}`+"\n", f.sessionID)))
		}},
		{name: "claim substituted", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			claim := f.claim
			claim.SessionID = "other-session"
			mustWriteRecoveryCandidateFixture(t, f.claimPath, mustProviderPoolRecoveryCandidateJSON(t, claim))
		}},
		{name: "claim stale schema", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			claim := f.claim
			claim.Schema = 2
			mustWriteRecoveryCandidateFixture(t, f.claimPath, mustProviderPoolRecoveryCandidateJSON(t, claim))
		}},
		{name: "claim zero turn", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			claim := f.claim
			claim.PendingTurn = 0
			mustWriteRecoveryCandidateFixture(t, f.claimPath, mustProviderPoolRecoveryCandidateJSON(t, claim))
		}},
		{name: "claim prior turn", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			claim := f.claim
			claim.PendingTurn = f.state.CompletedTurn
			mustWriteRecoveryCandidateFixture(t, f.claimPath, mustProviderPoolRecoveryCandidateJSON(t, claim))
		}},
		{name: "claim cross turn", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			claim := f.claim
			claim.PendingTurn = f.state.CompletedTurn + 2
			mustWriteRecoveryCandidateFixture(t, f.claimPath, mustProviderPoolRecoveryCandidateJSON(t, claim))
		}},
		{name: "claim partial", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.claimPath, []byte(`{"schema":1`))
		}},
		{name: "claim trailing substitution", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.claimPath, append(append([]byte(nil), f.claimRaw...), []byte(`{"schema":1}`)...))
		}},
		{name: "claim oversized", mutate: func(t *testing.T, f providerPoolRecoveryCandidateFixture) {
			mustWriteRecoveryCandidateFixture(t, f.claimPath, []byte(strings.Repeat("x", providerPoolRecoveryCandidateMaxEvidenceBytes+1)))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := writeProviderPoolRecoveryCandidateFixture(t, "candidate-"+strings.ReplaceAll(tc.name, " ", "-"), 7)
			tc.mutate(t, fixture)
			before := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root)
			if got := classifyProviderPoolCodexAppServerRecoveryCandidate(fixture.root, fixture.sessionID, providerPoolCodexAppServerAdapter); got != providerPoolAppServerRecoveryNotCandidate {
				t.Fatalf("classification = %v", got)
			}
			if after := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root); !reflect.DeepEqual(after, before) {
				t.Fatalf("failed-closed classification changed package state: before=%q after=%q", before, after)
			}
		})
	}

	t.Run("first turn claim without prior state", func(t *testing.T) {
		fixture := writeProviderPoolRecoveryCandidateFixture(t, "candidate-first-turn", 7)
		mustRemoveRecoveryCandidateFixture(t, fixture.statePath)
		claim := fixture.claim
		claim.PendingTurn = 1
		mustWriteRecoveryCandidateFixture(t, fixture.claimPath, mustProviderPoolRecoveryCandidateJSON(t, claim))
		if got := classifyProviderPoolCodexAppServerRecoveryCandidate(fixture.root, fixture.sessionID, providerPoolCodexAppServerAdapter); got != providerPoolAppServerRecoveryNotCandidate {
			t.Fatalf("classification = %v", got)
		}
	})

	t.Run("display and configured state cannot authorize", func(t *testing.T) {
		root := providerPoolDurableTestWorkdir(t)
		t.Setenv("DORKPIPE_PROVIDER_POOL_CODEX_ADAPTER", providerPoolCodexAppServerAdapter)
		mustWriteRecoveryCandidateFixture(t, filepath.Join(root, "display-state.json"), []byte(`{"adapter":"codex_app_server","appServerStatus":{"state":"recovery_required","outcomeUnknown":true},"messages":["continue"],"terminal_summary":"recovered_idle","provider_available":true,"catalog_order":["codex"],"authenticated":true,"response_text":"ready"}`))
		if got := classifyProviderPoolCodexAppServerRecoveryCandidate(root, "display-only", providerPoolCodexAppServerAdapter); got != providerPoolAppServerRecoveryNotCandidate {
			t.Fatalf("classification = %v", got)
		}
	})
}

func TestProviderPoolAppServerRecoveryAttemptIsOneShotAndNonAuthorizing(t *testing.T) {
	originalFactory := newProviderPoolAppServerSupervisorFunc
	originalPrompt := runProviderPoolCodexAppServerPromptFunc
	originalCapture := runProviderPoolCommandCaptureFunc
	originalRemove := removeProviderPoolAppServerTurnClaimFunc
	t.Cleanup(func() {
		newProviderPoolAppServerSupervisorFunc = originalFactory
		runProviderPoolCodexAppServerPromptFunc = originalPrompt
		runProviderPoolCommandCaptureFunc = originalCapture
		removeProviderPoolAppServerTurnClaimFunc = originalRemove
	})
	runProviderPoolCodexAppServerPromptFunc = func(context.Context, providerPoolPromptOptions, string, string, *providerPoolAppServerSessionState, uint64) (*providerPoolAppServerRunResult, providerPoolAppServerFallbackEligibility, error) {
		t.Fatal("recovery-only attempt dispatched a prompt")
		return nil, providerPoolAppServerDispatchedOrUnknown, errors.New("unexpected prompt")
	}
	runProviderPoolCommandCaptureFunc = func(context.Context, string, string, ...string) (string, string, int, error) {
		t.Fatal("recovery-only attempt started exec or fallback")
		return "", "", -1, errors.New("unexpected command")
	}
	removeProviderPoolAppServerTurnClaimFunc = func(string) error {
		t.Fatal("recovery-only attempt removed the unresolved claim")
		return errors.New("unexpected claim removal")
	}

	fixture := writeProviderPoolRecoveryCandidateFixture(t, "recovery-attempt-exact", 7)
	codexPath := filepath.Join(t.TempDir(), "codex-fixture")
	mustWriteRecoveryCandidateFixture(t, codexPath, []byte("fixture"))
	t.Setenv("CODEX_CLI_PATH", codexPath)
	t.Setenv("DORKPIPE_PROVIDER_POOL_CODEX_ADAPTER", providerPoolCodexExecAdapter)
	mustWriteRecoveryCandidateFixture(t, filepath.Join(fixture.root, "display-and-response-noise.json"), []byte(`{"appServerStatus":{"state":"completed"},"terminal_summary":"turn_completed","messages":["ready"],"provider_available":true,"catalog_order":["codex"],"authenticated":true,"response_text":"recovered"}`))
	before := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root)

	fake := &fakeProviderPoolAppServerSupervisor{
		fakeProviderPoolAppServerController: fakeProviderPoolAppServerController{events: make(chan providersession.Event)},
		providerSession:                     fixture.state.ProviderSessionID,
	}
	constructCalls := 0
	var constructed providerPoolAppServerSupervisorSpec
	newProviderPoolAppServerSupervisorFunc = func(spec providerPoolAppServerSupervisorSpec) (providerPoolAppServerSupervisor, error) {
		constructCalls++
		constructed = spec
		return fake, nil
	}

	observation, err := observeProviderPoolCodexAppServerRecoveryAttempt(context.Background(), fixture.root, fixture.sessionID, providerPoolCodexAppServerAdapter)
	if err != nil || observation != providerPoolAppServerRecoveryAttemptObserved {
		t.Fatalf("recovery-only observation = %v, err=%v", observation, err)
	}
	if constructCalls != 1 || fake.recoveryCalls != 1 || fake.shutdownCalls != 1 {
		t.Fatalf("calls: construct=%d RecoverBaseline=%d shutdown=%d", constructCalls, fake.recoveryCalls, fake.shutdownCalls)
	}
	if fake.startCalls != 0 || fake.selectionCalls != 0 || fake.threadCalls != 0 || fake.turnStartCalls != 0 {
		t.Fatalf("unexpected lifecycle calls: start=%d selection=%d thread=%d prompt=%d", fake.startCalls, fake.selectionCalls, fake.threadCalls, fake.turnStartCalls)
	}
	if fake.eventCalls != 0 || fake.stateCalls != 0 || len(fake.decisions) != 0 || fake.userInputPromptCalls != 0 || len(fake.userInputResponses) != 0 || len(fake.cancellationIntents) != 0 {
		t.Fatalf("unexpected controller calls: events=%d state=%d approvals=%d user-input-prompts=%d user-input-responses=%d cancellations=%d", fake.eventCalls, fake.stateCalls, len(fake.decisions), fake.userInputPromptCalls, len(fake.userInputResponses), len(fake.cancellationIntents))
	}
	wantSession := providersession.SessionRef{Provider: "codex", SessionID: fixture.state.ProviderSessionID}
	wantRequest := providersession.RecoveryRequest{Session: wantSession, RecoveryEvidence: fixture.state.RecoveryEvidence}
	wantPolicy, policyErr := appserversupervisor.BaselineLifecyclePolicy(fixture.root, fixture.state.Model, fixture.state.ReasoningEffort)
	if policyErr != nil {
		t.Fatal(policyErr)
	}
	if constructed.Session != wantSession || constructed.Identity.RecoveryEvidence != fixture.state.RecoveryEvidence {
		t.Fatalf("constructed spec lost retained evidence: session=%+v identity=%+v", constructed.Session, constructed.Identity)
	}
	if len(fake.recoveryRequests) != 1 || fake.recoveryRequests[0] != wantRequest || len(fake.recoveryPolicies) != 1 || !reflect.DeepEqual(fake.recoveryPolicies[0], wantPolicy) {
		t.Fatalf("RecoverBaseline inputs changed: requests=%+v policies=%+v", fake.recoveryRequests, fake.recoveryPolicies)
	}
	if after := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root); !reflect.DeepEqual(after, before) {
		t.Fatalf("recovery-only observation changed retained evidence: before=%q after=%q", before, after)
	}
}

func TestProviderPoolAppServerRecoveryAttemptFailuresRemainClosed(t *testing.T) {
	originalFactory := newProviderPoolAppServerSupervisorFunc
	t.Cleanup(func() { newProviderPoolAppServerSupervisorFunc = originalFactory })
	codexPath := filepath.Join(t.TempDir(), "codex-fixture")
	mustWriteRecoveryCandidateFixture(t, codexPath, []byte("fixture"))
	t.Setenv("CODEX_CLI_PATH", codexPath)
	t.Setenv("DORKPIPE_PROVIDER_POOL_CODEX_ADAPTER", providerPoolCodexExecAdapter)

	t.Run("non-candidate constructs nothing", func(t *testing.T) {
		constructCalls := 0
		newProviderPoolAppServerSupervisorFunc = func(providerPoolAppServerSupervisorSpec) (providerPoolAppServerSupervisor, error) {
			constructCalls++
			return nil, errors.New("unexpected constructor")
		}
		root := providerPoolDurableTestWorkdir(t)
		mustWriteRecoveryCandidateFixture(t, filepath.Join(root, "display-only.json"), []byte(`{"appServerStatus":{"state":"recovery_required","outcomeUnknown":true},"messages":["continue"],"provider_available":true,"authenticated":true}`))
		if observation, err := observeProviderPoolCodexAppServerRecoveryAttempt(context.Background(), root, "display-only", providerPoolCodexAppServerAdapter); err == nil || observation != providerPoolAppServerRecoveryAttemptNotObserved {
			t.Fatalf("non-candidate observation = %v, err=%v", observation, err)
		}
		if constructCalls != 0 {
			t.Fatalf("non-candidate constructor calls = %d", constructCalls)
		}
	})

	for _, tc := range []struct {
		name           string
		constructorErr error
		recoveryErr    bool
		shutdownErr    bool
		wantRecover    int
		wantShutdown   int
	}{
		{name: "constructor failure", constructorErr: errors.New("fixture constructor failure")},
		{name: "RecoverBaseline failure", recoveryErr: true, wantRecover: 1, wantShutdown: 1},
		{name: "shutdown failure", shutdownErr: true, wantRecover: 1, wantShutdown: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := writeProviderPoolRecoveryCandidateFixture(t, "recovery-attempt-"+strings.ReplaceAll(tc.name, " ", "-"), 7)
			before := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root)
			fake := &fakeProviderPoolAppServerSupervisor{
				fakeProviderPoolAppServerController: fakeProviderPoolAppServerController{events: make(chan providersession.Event)},
				providerSession:                     fixture.state.ProviderSessionID,
			}
			if tc.recoveryErr {
				fake.failStage = "recovery"
			}
			if tc.shutdownErr {
				fake.shutdownErr = errors.New("fixture shutdown failure")
			}
			constructCalls := 0
			newProviderPoolAppServerSupervisorFunc = func(providerPoolAppServerSupervisorSpec) (providerPoolAppServerSupervisor, error) {
				constructCalls++
				if tc.constructorErr != nil {
					return nil, tc.constructorErr
				}
				return fake, nil
			}
			observation, err := observeProviderPoolCodexAppServerRecoveryAttempt(context.Background(), fixture.root, fixture.sessionID, providerPoolCodexAppServerAdapter)
			if err == nil || observation != providerPoolAppServerRecoveryAttemptNotObserved {
				t.Fatalf("failure observation = %v, err=%v", observation, err)
			}
			if constructCalls != 1 || fake.recoveryCalls != tc.wantRecover || fake.shutdownCalls != tc.wantShutdown || fake.turnStartCalls != 0 {
				t.Fatalf("calls: construct=%d RecoverBaseline=%d shutdown=%d prompt=%d", constructCalls, fake.recoveryCalls, fake.shutdownCalls, fake.turnStartCalls)
			}
			if after := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root); !reflect.DeepEqual(after, before) {
				t.Fatalf("failed attempt changed retained evidence: before=%q after=%q", before, after)
			}
		})
	}
}

func TestProviderPoolAppServerRecoveryTransitionHasNoSafeSeparateWriteOrdering(t *testing.T) {
	const pipeonRecoveryRequired = `{"codexSessionAdapter":"codex_app_server","codexAppServerPostTurnSnapshot":{"adapter":"codex_app_server","state":"recovery_required","outcomeUnknown":true}}` + "\n"
	const pipeonSnapshotCleared = `{"codexSessionAdapter":"codex_app_server"}` + "\n"

	for _, tc := range []struct {
		name          string
		applyFirst    func(t *testing.T, fixture providerPoolRecoveryCandidateFixture, pipeonPath string)
		wantCandidate providerPoolAppServerRecoveryCandidateClassification
		wantClaim     bool
		wantGuard     bool
		changedPath   func(providerPoolRecoveryCandidateFixture, string) string
	}{
		{
			name: "claim first strands the retained Pipeon guard",
			applyFirst: func(t *testing.T, fixture providerPoolRecoveryCandidateFixture, _ string) {
				mustRemoveRecoveryCandidateFixture(t, fixture.claimPath)
			},
			wantCandidate: providerPoolAppServerRecoveryNotCandidate,
			wantClaim:     false,
			wantGuard:     true,
			changedPath:   func(fixture providerPoolRecoveryCandidateFixture, _ string) string { return fixture.claimPath },
		},
		{
			name: "Pipeon first removes the guard while the unresolved claim survives",
			applyFirst: func(t *testing.T, _ providerPoolRecoveryCandidateFixture, pipeonPath string) {
				mustWriteRecoveryCandidateFixture(t, pipeonPath, []byte(pipeonSnapshotCleared))
			},
			wantCandidate: providerPoolAppServerRecoveryCandidate,
			wantClaim:     true,
			wantGuard:     false,
			changedPath:   func(_ providerPoolRecoveryCandidateFixture, pipeonPath string) string { return pipeonPath },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := writeProviderPoolRecoveryCandidateFixture(t, "recovery-transition-"+strings.ReplaceAll(tc.name, " ", "-"), 7)
			pipeonPath := filepath.Join(fixture.root, "pipeon-workspace-chat-state.json")
			mustWriteRecoveryCandidateFixture(t, pipeonPath, []byte(pipeonRecoveryRequired))
			before := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root)

			if candidate, claim, guard := inspectProviderPoolRecoveryTransitionOrderingFixture(t, fixture, pipeonPath); candidate != providerPoolAppServerRecoveryCandidate || !claim || !guard {
				t.Fatalf("invalid initial fixture: candidate=%v claim=%t guard=%t", candidate, claim, guard)
			}

			// Model an injected crash, synchronization failure, or commit failure after
			// the first owner has durably changed its record and before the second can.
			tc.applyFirst(t, fixture, pipeonPath)
			after := snapshotProviderPoolRecoveryCandidateTree(t, fixture.root)
			candidate, claim, guard := inspectProviderPoolRecoveryTransitionOrderingFixture(t, fixture, pipeonPath)
			if candidate != tc.wantCandidate || claim != tc.wantClaim || guard != tc.wantGuard {
				t.Fatalf("reloaded partial state: candidate=%v claim=%t guard=%t", candidate, claim, guard)
			}
			if !bytes.Equal(after[mustRelativeRecoveryTransitionPath(t, fixture.root, fixture.bindingPath)], before[mustRelativeRecoveryTransitionPath(t, fixture.root, fixture.bindingPath)]) {
				t.Fatal("separate write changed the authoritative adapter binding")
			}
			if !bytes.Equal(after[mustRelativeRecoveryTransitionPath(t, fixture.root, fixture.statePath)], before[mustRelativeRecoveryTransitionPath(t, fixture.root, fixture.statePath)]) {
				t.Fatal("separate write changed the completed App Server state")
			}

			changed := 0
			wantChanged := mustRelativeRecoveryTransitionPath(t, fixture.root, tc.changedPath(fixture, pipeonPath))
			for path, oldRaw := range before {
				newRaw, found := after[path]
				if !found || !bytes.Equal(newRaw, oldRaw) {
					changed++
					if path != wantChanged {
						t.Fatalf("unexpected partial mutation at %q; wanted only %q", path, wantChanged)
					}
				}
			}
			for path := range after {
				if _, found := before[path]; !found {
					changed++
					if path != wantChanged {
						t.Fatalf("unexpected new partial mutation at %q; wanted only %q", path, wantChanged)
					}
				}
			}
			if changed != 1 {
				t.Fatalf("changed paths after injected failure = %d, want 1", changed)
			}
		})
	}
}

func inspectProviderPoolRecoveryTransitionOrderingFixture(t *testing.T, fixture providerPoolRecoveryCandidateFixture, pipeonPath string) (providerPoolAppServerRecoveryCandidateClassification, bool, bool) {
	t.Helper()
	candidate := classifyProviderPoolCodexAppServerRecoveryCandidate(fixture.root, fixture.sessionID, providerPoolCodexAppServerAdapter)
	claimRaw, claimErr := os.ReadFile(fixture.claimPath)
	claimPresent := claimErr == nil && bytes.Equal(claimRaw, fixture.claimRaw)
	if claimErr != nil && !errors.Is(claimErr, os.ErrNotExist) {
		t.Fatal(claimErr)
	}
	pipeonRaw, err := os.ReadFile(pipeonPath)
	if err != nil {
		t.Fatal(err)
	}
	var pipeon struct {
		CodexSessionAdapter            string `json:"codexSessionAdapter"`
		CodexAppServerPostTurnSnapshot *struct {
			Adapter        string `json:"adapter"`
			State          string `json:"state"`
			OutcomeUnknown bool   `json:"outcomeUnknown"`
		} `json:"codexAppServerPostTurnSnapshot"`
	}
	if err := json.Unmarshal(pipeonRaw, &pipeon); err != nil {
		t.Fatal(err)
	}
	guardPresent := pipeon.CodexSessionAdapter == providerPoolCodexAppServerAdapter &&
		pipeon.CodexAppServerPostTurnSnapshot != nil &&
		pipeon.CodexAppServerPostTurnSnapshot.Adapter == providerPoolCodexAppServerAdapter &&
		pipeon.CodexAppServerPostTurnSnapshot.State == "recovery_required" &&
		pipeon.CodexAppServerPostTurnSnapshot.OutcomeUnknown
	return candidate, claimPresent, guardPresent
}

func mustRelativeRecoveryTransitionPath(t *testing.T, root, path string) string {
	t.Helper()
	for prefix, base := range map[string]string{
		"checkout": root,
		"durable":  mustProviderPoolDurablePackageRoot(t, root),
	} {
		relative, err := filepath.Rel(base, path)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return filepath.Join(prefix, relative)
		}
	}
	t.Fatalf("recovery transition path %q is outside checkout and durable roots", path)
	return ""
}

type providerPoolRecoveryCandidateFixture struct {
	root        string
	sessionID   string
	bindingPath string
	statePath   string
	claimPath   string
	stateRaw    []byte
	state       providerPoolAppServerSessionState
	claimRaw    []byte
	claim       providerPoolAppServerTurnClaim
}

func writeProviderPoolRecoveryCandidateFixture(t *testing.T, sessionID string, completedTurn uint64) providerPoolRecoveryCandidateFixture {
	t.Helper()
	root := providerPoolDurableTestWorkdir(t)
	bindingPath, err := providerPoolSessionAdapterBindingPath(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	statePath, err := providerPoolCodexAppServerSessionPath(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(bindingPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		t.Fatal(err)
	}
	binding := providerPoolSessionAdapterBinding{Schema: 1, SessionID: sessionID, Adapter: providerPoolCodexAppServerAdapter}
	state := providerPoolAppServerSessionState{Schema: 1, SessionID: sessionID, CompletedTurn: completedTurn, ProviderSessionID: "thread-fixture", RecoveryEvidence: providerPoolCodexAppServerRecoveryEvidence(sessionID), Model: "gpt-5.5", ReasoningEffort: appserversupervisor.PinnedReasoningEffort}
	claim := providerPoolAppServerTurnClaim{Schema: 1, SessionID: sessionID, PendingTurn: completedTurn + 1}
	stateRaw := mustProviderPoolRecoveryCandidateJSON(t, state)
	claimRaw := mustProviderPoolRecoveryCandidateJSON(t, claim)
	mustWriteRecoveryCandidateFixture(t, bindingPath, mustProviderPoolRecoveryCandidateJSON(t, binding))
	mustWriteRecoveryCandidateFixture(t, statePath, stateRaw)
	mustWriteRecoveryCandidateFixture(t, statePath+".lock", claimRaw)
	return providerPoolRecoveryCandidateFixture{root: root, sessionID: sessionID, bindingPath: bindingPath, statePath: statePath, claimPath: statePath + ".lock", stateRaw: stateRaw, state: state, claimRaw: claimRaw, claim: claim}
}

func providerPoolDurableTestWorkdir(t *testing.T) string {
	t.Helper()
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("LOCALAPPDATA", stateHome)
	t.Setenv("HOME", stateHome)
	return t.TempDir()
}

func mustProviderPoolRecoveryCandidateJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func mustWriteRecoveryCandidateFixture(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustRemoveRecoveryCandidateFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func snapshotProviderPoolRecoveryCandidateTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	snapshot := map[string][]byte{}
	for prefix, base := range map[string]string{
		"checkout": root,
		"durable":  mustProviderPoolDurablePackageRoot(t, root),
	} {
		if err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snapshot[filepath.Join(prefix, relative)] = raw
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return snapshot
}

func mustProviderPoolDurablePackageRoot(t *testing.T, workdir string) string {
	t.Helper()
	root, err := infrastructure.ProjectPackageStateDir(workdir, "dorkpipe")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

package main

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
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestProviderPoolCodexAppServerDispatchResumesOnlyVerifiedIdleAndNeverFallsBack(t *testing.T) {
	root := t.TempDir()
	codexPath := filepath.Join(root, "codex.exe")
	if err := os.WriteFile(codexPath, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_CLI_PATH", codexPath)
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-app-server-1", providerPoolCodexAppServerAdapter); err != nil {
		t.Fatal(err)
	}
	original := runProviderPoolCodexAppServerPromptFunc
	t.Cleanup(func() { runProviderPoolCodexAppServerPromptFunc = original })
	calls := 0
	failTurn := false
	runProviderPoolCodexAppServerPromptFunc = func(_ context.Context, opts providerPoolPromptOptions, model, executable string, prior *providerPoolAppServerSessionState, turn uint64) (*providerPoolAppServerRunResult, error) {
		calls++
		if opts.SessionAdapter != providerPoolCodexAppServerAdapter || model != "gpt-5.5" || executable != codexPath {
			t.Fatalf("dispatch = %+v, %q, %q", opts, model, executable)
		}
		if turn == 1 && prior != nil {
			t.Fatalf("first turn has prior state: %+v", prior)
		}
		if turn > 1 && (prior == nil || prior.CompletedTurn != turn-1 || prior.ProviderSessionID != "thread-1") {
			t.Fatalf("resume turn %d prior = %+v", turn, prior)
		}
		if failTurn {
			return &providerPoolAppServerRunResult{Response: &providerPoolPromptResponse{Provider: "codex", State: "failed", Text: "unknown", ExitCode: 1}}, nil
		}
		return &providerPoolAppServerRunResult{
			Response:          &providerPoolPromptResponse{Provider: "codex", State: "ready", Text: fmt.Sprintf("fixture-%d", turn), ExitCode: 0},
			ProviderSessionID: "thread-1",
			RecoveryEvidence:  providerPoolCodexAppServerRecoveryEvidence(opts.SessionID),
			VerifiedIdle:      true,
		}, nil
	}
	opts := providerPoolPromptOptions{Workdir: root, Provider: "codex", Prompt: "fixture", SessionID: "pipeon-app-server-1", SessionAdapter: providerPoolCodexAppServerAdapter}
	if _, err := runProviderPoolCodexPrompt(context.Background(), opts, "config"); err == nil || !strings.Contains(err.Error(), "requires an explicit model") {
		t.Fatalf("config alias error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("config alias reached App Server runner: %d", calls)
	}
	result, err := runProviderPoolCodexPrompt(context.Background(), opts, "gpt-5.5")
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "fixture-1" || calls != 1 {
		t.Fatalf("first dispatch = %+v, calls=%d", result, calls)
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
}

type fakeProviderPoolAppServerController struct {
	events              chan providersession.Event
	state               providersession.State
	decisions           []providersession.ApprovalDecision
	decideErr           error
	userInputPrompt     providersession.UserInputPrompt
	userInputPromptErr  error
	userInputResponses  []providersession.UserInputResponse
	respondUserInputErr error
	completedText       string
	recovery            string
}

func (f *fakeProviderPoolAppServerController) Events() <-chan providersession.Event {
	return f.events
}

func (f *fakeProviderPoolAppServerController) State() providersession.State {
	return f.state
}

func (f *fakeProviderPoolAppServerController) Decide(_ context.Context, decision providersession.ApprovalDecision) error {
	f.decisions = append(f.decisions, decision)
	return f.decideErr
}

func (f *fakeProviderPoolAppServerController) UserInputPrompt(_ context.Context, _ providersession.UserInputRequest) (providersession.UserInputPrompt, error) {
	return cloneProviderPoolUserInputPrompt(f.userInputPrompt), f.userInputPromptErr
}

func (f *fakeProviderPoolAppServerController) RespondUserInput(_ context.Context, response providersession.UserInputResponse) error {
	f.userInputResponses = append(f.userInputResponses, cloneProviderPoolUserInputResponse(response))
	return f.respondUserInputErr
}

func (f *fakeProviderPoolAppServerController) CompletedTurnText() (string, bool) {
	return f.completedText, f.completedText != ""
}

func (f *fakeProviderPoolAppServerController) RecoveryEvidence() string {
	return f.recovery
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
	root := t.TempDir()
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
	root := t.TempDir()
	adapter, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-2", providerPoolCodexAppServerAdapter)
	if err != nil || adapter != providerPoolCodexAppServerAdapter {
		t.Fatalf("App Server adapter = %q, err=%v", adapter, err)
	}
	if _, err := resolveProviderPoolCodexSessionAdapter(root, "pipeon-session-2", providerPoolCodexExecAdapter); err == nil {
		t.Fatal("App Server selection silently substituted exec")
	}
}

func TestProviderPoolCodexSessionAdapterRejectsInvalidOrMalformedEvidence(t *testing.T) {
	root := t.TempDir()
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

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

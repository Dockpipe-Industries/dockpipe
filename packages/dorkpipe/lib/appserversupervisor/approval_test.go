package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

func startApprovalTurn(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, LifecyclePolicy, LifecycleReference) {
	t.Helper()
	s, child, scanner, policy, turn := startEventTurn(t)
	sendNotification(t, child, "item/started", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"commandExecution","status":"inProgress"}}`)
	if event := nextEvent(t, s); event.Summary != "item_started" {
		t.Fatalf("item event = %+v", event)
	}
	return s, child, scanner, policy, turn
}

func sendServerRequest(t *testing.T, child *fakeChild, id uint64, method, params string) {
	t.Helper()
	// App Server request identifiers are numeric; avoid retaining a provider
	// identifier in any projected test value.
	frame := `{"jsonrpc":"2.0","id":` + jsonNumber(id) + `,"method":` + quoteJSON(method) + `,"params":` + params + "}\n"
	if _, err := child.stdoutW.Write([]byte(frame)); err != nil {
		t.Fatal(err)
	}
}

func jsonNumber(value uint64) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func approvalRequest(t *testing.T, s *Supervisor, child *fakeChild, id uint64, method, params string) providersession.Event {
	t.Helper()
	sendServerRequest(t, child, id, method, params)
	event := nextEvent(t, s)
	if event.Kind != providersession.EventApprovalRequested || event.State != providersession.StateWaitingForApproval || event.Approval == nil {
		t.Fatalf("approval event = %+v", event)
	}
	return event
}

func TestApprovalRelayProjectsSafeCorrelatedRequestsAndOneTimeDecision(t *testing.T) {
	for _, fixture := range []struct {
		name        string
		method      string
		params      string
		actionClass string
		decision    string
		wantResult  map[string]any
	}{
		{"command", "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","command":"private command","cwd":"private path","reason":"private reason"}`, "command_execution", providersession.DecisionApprove, map[string]any{"decision": "accept"}},
		{"file", "item/fileChange/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","patch":"private patch","reason":"private reason","startedAtMs":1710000000000}`, "workspace_change", providersession.DecisionDeny, map[string]any{"decision": "decline"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, scanner, _, _ := startApprovalTurn(t)
			event := approvalRequest(t, s, child, 41, fixture.method, fixture.params)
			if event.Approval.ActionClass != fixture.actionClass || len(event.Approval.Scope) != 1 || event.Approval.Scope[0] != "turn" || event.Approval.Correlation.ProcessIncarnationID == "" || event.Approval.Correlation.ConnectionID == "" || event.Approval.Correlation.SessionID != "thread-1" || event.Approval.Correlation.InteractionID != "turn-1" || event.Approval.Correlation.ActivityID != "item-1" || event.Approval.Correlation.RequestID == "" || event.Approval.Correlation.DecisionID == "" {
				t.Fatalf("unsafe approval projection = %+v", event)
			}
			data, _ := json.Marshal(event)
			if strings.Contains(string(data), "private") || strings.Contains(string(data), "command") && fixture.name != "command" {
				t.Fatalf("raw approval content leaked: %s", data)
			}
			done := make(chan error, 1)
			go func() {
				done <- s.Decide(context.Background(), providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: fixture.decision})
			}()
			if !scanner.Scan() {
				t.Fatal("expected private decision response")
			}
			var response struct {
				ID     uint64         `json:"id"`
				Result map[string]any `json:"result"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &response); err != nil || response.ID != 41 || !sameResult(response.Result, fixture.wantResult) {
				t.Fatalf("decision response = %s, err=%v", scanner.Text(), err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			sendNotification(t, child, "serverRequest/resolved", `{"threadId":"thread-1","requestId":41}`)
			if resolved := nextEvent(t, s); resolved.State != providersession.StateRunning || resolved.Summary != "approval_resolved" || resolved.Correlation != event.Approval.Correlation {
				t.Fatalf("resolution event = %+v", resolved)
			}
		})
	}
}

func TestApprovalRelayPermissionIsDeclaredAndDenyOnly(t *testing.T) {
	s, child, scanner, policy, _ := startApprovalTurn(t)
	params := `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","permissions":{"fileSystem":{"write":[` + quoteJSON(policy.Workspace) + `]}}}`
	event := approvalRequest(t, s, child, 42, "item/permissions/requestApproval", params)
	if event.Approval.ActionClass != "declared_permission" || len(event.Approval.Scope) != 1 || event.Approval.Scope[0] != "declared_writable_roots" {
		t.Fatalf("permission projection = %+v", event)
	}
	approve := providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionApprove}
	if err := s.Decide(context.Background(), approve); err == nil {
		t.Fatal("permission approval without a neutral granted subset must fail")
	}
	if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectDecisionRejected) {
		t.Fatalf("permission decision event = %+v", disconnected)
	}
	_ = scanner
}

func TestApprovalRelayIgnoresCurrentUnchangedThreadStatusWhilePending(t *testing.T) {
	s, child, scanner, _, _ := startApprovalTurn(t)
	event := approvalRequest(t, s, child, 48, "item/fileChange/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
	sendNotification(t, child, "thread/status/changed", `{"threadId":"thread-1","status":"idle"}`)
	time.Sleep(20 * time.Millisecond)
	if s.State() != providersession.StateWaitingForApproval {
		t.Fatalf("pending thread status changed session state: %s", s.State())
	}
	done := make(chan error, 1)
	go func() {
		done <- s.Decide(context.Background(), providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny})
	}()
	if !scanner.Scan() {
		t.Fatal("expected decision after unchanged thread status")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestApprovalRelayAcceptsZeroServerRequestID(t *testing.T) {
	s, child, scanner, _, _ := startApprovalTurn(t)
	event := approvalRequest(t, s, child, 0, "item/fileChange/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","startedAtMs":1710000000000}`)
	done := make(chan error, 1)
	go func() {
		done <- s.Decide(context.Background(), providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny})
	}()
	if !scanner.Scan() {
		t.Fatal("expected zero-ID decision response")
	}
	var response struct {
		ID     uint64         `json:"id"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil || response.ID != 0 || !sameResult(response.Result, map[string]any{"decision": "decline"}) {
		t.Fatalf("zero-ID decision response = %s, err=%v", scanner.Text(), err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	sendNotification(t, child, "serverRequest/resolved", `{"threadId":"thread-1","requestId":0}`)
	if resolved := nextEvent(t, s); resolved.State != providersession.StateRunning || resolved.Summary != "approval_resolved" {
		t.Fatalf("zero-ID resolution event = %+v", resolved)
	}
}

func TestUserInputRelayIsOpaqueAndRejectsApprovalDecision(t *testing.T) {
	s, child, _, _, _ := startApprovalTurn(t)
	sendServerRequest(t, child, 43, "item/tool/requestUserInput", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"private header","question":"private prompt","options":[{"label":"Private A","description":"private a"},{"label":"Private B","description":"private b"}]}]}`)
	event := nextEvent(t, s)
	if event.Kind != providersession.EventUserInputRequested || event.State != providersession.StateWaitingForUserInput || event.UserInput == nil || event.UserInput.PromptRef == "" || event.UserInput.Correlation.ActivityID != "item-1" {
		t.Fatalf("user input event = %+v", event)
	}
	data, _ := json.Marshal(event)
	if strings.Contains(string(data), "private") || strings.Contains(string(data), "question-1") {
		t.Fatalf("user-input content leaked: %s", data)
	}
	if err := s.Decide(context.Background(), providersession.ApprovalDecision{Correlation: event.UserInput.Correlation, Decision: providersession.DecisionDeny}); err == nil {
		t.Fatal("user-input answer must not be encoded as an approval decision")
	}
	if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectDecisionRejected) {
		t.Fatalf("unsupported input decision = %+v", disconnected)
	}
}

func TestCAS14UserInputAcceptsOnlyExperimentalDefaults(t *testing.T) {
	s, child, _, _, _ := startApprovalTurn(t)
	sendServerRequest(t, child, 53, "item/tool/requestUserInput", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","autoResolutionMs":null,"questions":[{"id":"question-1","header":"private header","question":"private prompt","isOther":false,"isSecret":false,"options":[{"label":"Private A","description":"private a"},{"label":"Private B","description":"private b"}]}]}`)
	event := nextEvent(t, s)
	if event.Kind != providersession.EventUserInputRequested || event.State != providersession.StateWaitingForUserInput || event.UserInput == nil {
		t.Fatalf("default-only experimental request = %+v", event)
	}
	prompt, err := s.UserInputPrompt(context.Background(), *event.UserInput)
	if err != nil || prompt.Kind != providersession.UserInputPromptSelectOne || len(prompt.Options) != 2 {
		t.Fatalf("default-only experimental prompt = %+v, err=%v", prompt, err)
	}
}

func TestCAS14UserInputRejectsActiveMalformedAndSubstitutedExperimentalFields(t *testing.T) {
	fixtures := []struct {
		name   string
		params string
	}{
		{name: "auto_resolution_zero", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","autoResolutionMs":0,"questions":[{"id":"question-1","header":"header","question":"prompt","options":null}]}`},
		{name: "auto_resolution_positive", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","autoResolutionMs":1000,"questions":[{"id":"question-1","header":"header","question":"prompt","options":null}]}`},
		{name: "auto_resolution_substituted", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","autoResolutionMs":"null","questions":[{"id":"question-1","header":"header","question":"prompt","options":null}]}`},
		{name: "other_active", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"prompt","isOther":true,"options":null}]}`},
		{name: "other_null", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"prompt","isOther":null,"options":null}]}`},
		{name: "other_substituted", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"prompt","isOther":"false","options":null}]}`},
		{name: "secret_active", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"prompt","isSecret":true,"options":null}]}`},
		{name: "secret_null", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"prompt","isSecret":null,"options":null}]}`},
		{name: "secret_substituted", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"prompt","isSecret":0,"options":null}]}`},
		{name: "unknown_question_field", params: `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"prompt","futureField":false,"options":null}]}`},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, _, _, _ := startApprovalTurn(t)
			sendServerRequest(t, child, 54, "item/tool/requestUserInput", fixture.params)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedEvent) {
				t.Fatalf("experimental field rejection = %+v", event)
			}
		})
	}
}

func startUserInputRequest(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, providersession.UserInputRequest, providersession.UserInputPrompt) {
	t.Helper()
	s, child, scanner, _, _ := startApprovalTurn(t)
	sendServerRequest(t, child, 49, "item/tool/requestUserInput", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"private header","question":"private prompt","options":[{"label":"Private A","description":"private a"},{"label":"Private B","description":"private b"}]}]}`)
	event := nextEvent(t, s)
	if event.Kind != providersession.EventUserInputRequested || event.UserInput == nil {
		t.Fatalf("user-input request event = %+v", event)
	}
	request := *event.UserInput
	prompt, err := s.UserInputPrompt(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return s, child, scanner, request, prompt
}

func promptOptionRef(t *testing.T, prompt providersession.UserInputPrompt, label string) string {
	t.Helper()
	for _, option := range prompt.Options {
		if option.Label == label {
			return option.OptionRef
		}
	}
	t.Fatalf("prompt option %q not found in %+v", label, prompt)
	return ""
}

func TestCAS14ProviderUserInputPromptIsNormalizedExactDefensiveAndLookupOnly(t *testing.T) {
	s, _, _, request, first := startUserInputRequest(t)
	if err := first.ValidateFor(request); err != nil || first.Summary != "private prompt" || len(first.Options) != 2 || first.Options[0].Label != "Private A" || first.Options[1].Label != "Private B" {
		t.Fatalf("normalized prompt lookup = %+v, err=%v", first, err)
	}
	for _, option := range first.Options {
		if !strings.HasPrefix(option.OptionRef, "option-") || strings.Contains(option.OptionRef, option.Label) || option.OptionRef == request.PromptRef {
			t.Fatalf("option reference is not opaque: %+v", option)
		}
	}
	first.Options[0].Label = "returned mutation"
	second, err := s.UserInputPrompt(context.Background(), request)
	if err != nil || second.Options[0].Label != "Private A" {
		t.Fatalf("prompt lookup was not defensively copied: %+v, err=%v", second, err)
	}

	s.mu.RLock()
	pending := s.lifecycle.pending
	state := s.state
	inFlight := pending != nil && pending.decisionInFlight
	s.mu.RUnlock()
	if state != providersession.StateWaitingForUserInput || pending == nil || inFlight {
		t.Fatalf("lookup changed input or lifecycle state: state=%s pending=%v in_flight=%t", state, pending != nil, inFlight)
	}
}

func TestCAS14ProviderUserInputOptionReferencesDoNotDependOnOrder(t *testing.T) {
	_, _, _, request, _ := startUserInputRequest(t)
	first, ok := parseProviderUserInputQuestion([]json.RawMessage{json.RawMessage(`{"id":"question-1","header":" header ","question":" choose   one ","options":[{"label":"Private A","description":"private a"},{"label":"Private B","description":"private b"}]}`)})
	if !ok {
		t.Fatal("first provider question was rejected")
	}
	second, ok := parseProviderUserInputQuestion([]json.RawMessage{json.RawMessage(`{"id":"question-1","header":"header","question":"choose one","options":[{"label":"Private B","description":"private b"},{"label":"Private A","description":"private a"}]}`)})
	if !ok {
		t.Fatal("reordered provider question was rejected")
	}
	firstPrompt, _, ok := projectProviderUserInput(first, request)
	if !ok {
		t.Fatal("first prompt projection was rejected")
	}
	secondPrompt, _, ok := projectProviderUserInput(second, request)
	if !ok {
		t.Fatal("reordered prompt projection was rejected")
	}
	for _, label := range []string{"Private A", "Private B"} {
		if promptOptionRef(t, firstPrompt, label) != promptOptionRef(t, secondPrompt, label) {
			t.Fatalf("option reference for %q depended on provider ordering", label)
		}
	}
	if firstPrompt.Summary != "choose one" || secondPrompt.Summary != "choose one" {
		t.Fatalf("question text was not normalized: first=%q second=%q", firstPrompt.Summary, secondPrompt.Summary)
	}
}

func TestCAS14ProviderUserInputNormalizationRejectsMalformedOversizedAndAmbiguousPrompts(t *testing.T) {
	requests := map[string]string{
		"empty_question":      `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"   ","options":null}]}`,
		"control_character":   `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"unsafe\u0001question","options":null}]}`,
		"duplicate_labels":    `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"choose","options":[{"label":"Option A","description":"first"},{"label":"Option A","description":"second"}]}]}`,
		"ambiguous_labels":    `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"choose","options":[{"label":"Option  A","description":"first"},{"label":"Option A","description":"second"}]}]}`,
		"missing_description": `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"choose","options":[{"label":"Option A"}]}]}`,
	}
	requests["oversized_question"] = `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":` + quoteJSON(strings.Repeat("x", 513)) + `,"options":null}]}`
	requests["oversized_label"] = `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"header","question":"choose","options":[{"label":` + quoteJSON(strings.Repeat("x", 129)) + `,"description":"private"}]}]}`
	for name, params := range requests {
		t.Run(name, func(t *testing.T) {
			s, child, _, _, _ := startApprovalTurn(t)
			sendServerRequest(t, child, 49, "item/tool/requestUserInput", params)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedEvent) {
				t.Fatalf("normalization rejection = %+v", event)
			}
		})
	}
}

func TestCAS14UserInputPromptLookupMismatchAndExpiryFailClosed(t *testing.T) {
	for _, name := range []string{"stale", "cross_session", "substituted_reference"} {
		t.Run(name, func(t *testing.T) {
			s, _, _, request, _ := startUserInputRequest(t)
			lookup := request
			switch name {
			case "stale":
				lookup.Correlation.DecisionID = "decision-stale"
			case "cross_session":
				lookup.Correlation.SessionID = "thread-other"
			case "substituted_reference":
				lookup.PromptRef = "request-substituted"
			}
			if _, err := s.UserInputPrompt(context.Background(), lookup); !errors.Is(err, ErrUserInputPromptRejected) {
				t.Fatalf("mismatched lookup error = %v", err)
			}
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectCorrelationMismatch) {
				t.Fatalf("mismatched lookup rejection = %+v", event)
			}
		})
	}

	t.Run("expired", func(t *testing.T) {
		s, _, _, request, _ := startUserInputRequest(t)
		s.expirePending(request.Correlation)
		if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectRequestDeadline) {
			t.Fatalf("expiry rejection = %+v", event)
		}
		if _, err := s.UserInputPrompt(context.Background(), request); !errors.Is(err, ErrUserInputPromptUnavailable) {
			t.Fatalf("expired lookup error = %v", err)
		}
		s.mu.RLock()
		pending := s.lifecycle.pending
		s.mu.RUnlock()
		if pending != nil {
			t.Fatal("expired normalized prompt remained in private state")
		}
	})
}

func TestCAS14UserInputResponseDeliversExactChoiceOnceAndRetainsNoAnswer(t *testing.T) {
	s, child, scanner, request, prompt := startUserInputRequest(t)
	selectedRef := promptOptionRef(t, prompt, "Private B")
	response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{selectedRef}}
	done := make(chan error, 1)
	go func() { done <- s.RespondUserInput(context.Background(), response) }()
	if !scanner.Scan() {
		t.Fatal("expected private user-input response")
	}
	var wire struct {
		ID     uint64         `json:"id"`
		Result map[string]any `json:"result"`
	}
	want := map[string]any{"answers": map[string]any{"question-1": map[string]any{"answers": []string{"Private B"}}}}
	if err := json.Unmarshal(scanner.Bytes(), &wire); err != nil || wire.ID != 49 || !sameResult(wire.Result, want) {
		t.Fatalf("user-input response = %s, err=%v", scanner.Text(), err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	s.mu.RLock()
	pending := s.lifecycle.pending
	retainedPrompt := pending != nil && pending.prompt != nil
	retainedMapping := pending != nil && pending.input != nil
	s.mu.RUnlock()
	if pending == nil || retainedPrompt || retainedMapping || !pending.decisionInFlight {
		t.Fatalf("transient response state retained: pending=%v prompt=%t mapping=%t", pending != nil, retainedPrompt, retainedMapping)
	}
	s.audit.mu.Lock()
	auditData, _ := json.Marshal(s.audit.document)
	s.audit.mu.Unlock()
	for _, private := range []string{"Private B", selectedRef, "question-1"} {
		if strings.Contains(string(auditData), private) {
			t.Fatalf("private response data leaked into audit: %s", auditData)
		}
	}

	sendNotification(t, child, "serverRequest/resolved", `{"threadId":"thread-1","requestId":49}`)
	resolved := nextEvent(t, s)
	if resolved.State != providersession.StateRunning || resolved.Summary != "user_input_resolved" || resolved.Correlation != request.Correlation {
		t.Fatalf("user-input resolution = %+v", resolved)
	}
	encoded, _ := json.Marshal(resolved)
	if strings.Contains(string(encoded), "Private B") || strings.Contains(string(encoded), selectedRef) || strings.Contains(string(encoded), "question-1") {
		t.Fatalf("private response data leaked into event: %s", encoded)
	}
}

func TestCAS14UserInputTextResponseIsTransientAndExact(t *testing.T) {
	s, child, scanner, _, _ := startApprovalTurn(t)
	sendServerRequest(t, child, 50, "item/tool/requestUserInput", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"private-text-question","header":"private header","question":"private prompt","options":null}]}`)
	event := nextEvent(t, s)
	if event.UserInput == nil {
		t.Fatalf("user-input request = %+v", event)
	}
	request := *event.UserInput
	prompt, err := s.UserInputPrompt(context.Background(), request)
	if err != nil || prompt.Kind != providersession.UserInputPromptText || prompt.Summary != "private prompt" || prompt.MaxTextBytes != 4096 {
		t.Fatalf("text prompt = %+v, err=%v", prompt, err)
	}
	response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, Text: "private answer"}
	done := make(chan error, 1)
	go func() { done <- s.RespondUserInput(context.Background(), response) }()
	if !scanner.Scan() {
		t.Fatal("expected private text response")
	}
	var wire struct {
		Result map[string]any `json:"result"`
	}
	want := map[string]any{"answers": map[string]any{"private-text-question": map[string]any{"answers": []string{"private answer"}}}}
	if err := json.Unmarshal(scanner.Bytes(), &wire); err != nil || !sameResult(wire.Result, want) {
		t.Fatalf("text response = %s, err=%v", scanner.Text(), err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	s.mu.RLock()
	retained := s.lifecycle.pending != nil && (s.lifecycle.pending.prompt != nil || s.lifecycle.pending.input != nil)
	s.mu.RUnlock()
	if retained {
		t.Fatal("text answer or provider mapping remained in pending state")
	}
}

func TestCAS14UserInputResponseRejectsMismatchesMalformedAndUnknownOptions(t *testing.T) {
	tests := map[string]struct {
		mutate func(*providersession.UserInputResponse)
		reason DisconnectReason
	}{
		"stale": {
			mutate: func(response *providersession.UserInputResponse) { response.Correlation.DecisionID = "decision-stale" },
			reason: DisconnectCorrelationMismatch,
		},
		"cross_session": {
			mutate: func(response *providersession.UserInputResponse) { response.Correlation.SessionID = "thread-other" },
			reason: DisconnectCorrelationMismatch,
		},
		"mismatched_prompt": {
			mutate: func(response *providersession.UserInputResponse) { response.PromptRef = "request-other" },
			reason: DisconnectCorrelationMismatch,
		},
		"unknown_option": {
			mutate: func(response *providersession.UserInputResponse) {
				response.SelectedOptionRefs = []string{"option-unknown"}
			},
			reason: DisconnectDecisionRejected,
		},
		"malformed": {
			mutate: func(response *providersession.UserInputResponse) { response.SelectedOptionRefs = nil },
			reason: DisconnectDecisionRejected,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s, _, _, request, prompt := startUserInputRequest(t)
			response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{promptOptionRef(t, prompt, "Private A")}}
			test.mutate(&response)
			if err := s.RespondUserInput(context.Background(), response); !errors.Is(err, ErrUserInputResponseRejected) {
				t.Fatalf("response rejection = %v", err)
			}
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(test.reason) {
				t.Fatalf("response rejection event = %+v", event)
			}
		})
	}
}

func TestCAS14UserInputResponseRejectsOversizedDuplicateExpiredDisconnectedAndReplay(t *testing.T) {
	t.Run("oversized_text", func(t *testing.T) {
		s, child, _, _, _ := startApprovalTurn(t)
		sendServerRequest(t, child, 51, "item/tool/requestUserInput", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"text-question","header":"private header","question":"private prompt","options":null}]}`)
		request := *nextEvent(t, s).UserInput
		response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, Text: strings.Repeat("x", 4097)}
		if err := s.RespondUserInput(context.Background(), response); !errors.Is(err, ErrUserInputResponseRejected) {
			t.Fatalf("oversized response = %v", err)
		}
		if event := nextEvent(t, s); event.Summary != string(DisconnectDecisionRejected) {
			t.Fatalf("oversized response event = %+v", event)
		}
	})

	t.Run("duplicate_in_flight", func(t *testing.T) {
		s, _, scanner, request, prompt := startUserInputRequest(t)
		response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{promptOptionRef(t, prompt, "Private A")}}
		done := make(chan error, 1)
		go func() { done <- s.RespondUserInput(context.Background(), response) }()
		if !scanner.Scan() {
			t.Fatal("expected first response")
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if err := s.RespondUserInput(context.Background(), response); !errors.Is(err, ErrUserInputResponseUnavailable) {
			t.Fatalf("duplicate response = %v", err)
		}
		if event := nextEvent(t, s); event.Summary != string(DisconnectDecisionRejected) {
			t.Fatalf("duplicate response event = %+v", event)
		}
	})

	t.Run("expired", func(t *testing.T) {
		s, _, _, request, prompt := startUserInputRequest(t)
		s.expirePending(request.Correlation)
		if event := nextEvent(t, s); event.Summary != string(DisconnectRequestDeadline) {
			t.Fatalf("expiry event = %+v", event)
		}
		response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{promptOptionRef(t, prompt, "Private A")}}
		if err := s.RespondUserInput(context.Background(), response); !errors.Is(err, ErrUserInputResponseUnavailable) {
			t.Fatalf("expired response = %v", err)
		}
	})

	t.Run("post_disconnect", func(t *testing.T) {
		s, _, _, request, prompt := startUserInputRequest(t)
		s.fail(DisconnectTransportClosed)
		_ = nextEvent(t, s)
		response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{promptOptionRef(t, prompt, "Private A")}}
		if err := s.RespondUserInput(context.Background(), response); !errors.Is(err, ErrUserInputResponseUnavailable) {
			t.Fatalf("post-disconnect response = %v", err)
		}
	})

	t.Run("replayed_after_resolution", func(t *testing.T) {
		s, child, scanner, request, prompt := startUserInputRequest(t)
		response := providersession.UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{promptOptionRef(t, prompt, "Private A")}}
		done := make(chan error, 1)
		go func() { done <- s.RespondUserInput(context.Background(), response) }()
		if !scanner.Scan() {
			t.Fatal("expected first response")
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		sendNotification(t, child, "serverRequest/resolved", `{"threadId":"thread-1","requestId":49}`)
		_ = nextEvent(t, s)
		if err := s.RespondUserInput(context.Background(), response); !errors.Is(err, ErrUserInputResponseUnavailable) {
			t.Fatalf("replayed response = %v", err)
		}
		if event := nextEvent(t, s); event.Summary != string(DisconnectDecisionRejected) {
			t.Fatalf("replayed response event = %+v", event)
		}
	})
}

func TestCAS14UserInputRejectsMultiQuestionBatchWithoutPartialAnswer(t *testing.T) {
	s, child, _, _, _ := startApprovalTurn(t)
	sendServerRequest(t, child, 52, "item/tool/requestUserInput", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"first","question":"first prompt","options":null},{"id":"question-2","header":"second","question":"second prompt","options":null}]}`)
	if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedEvent) {
		t.Fatalf("multi-question rejection = %+v", event)
	}
}

func TestApprovalRelayRejectsDuplicateStaleReorderedAndCrossCorrelatedMessages(t *testing.T) {
	tests := []struct {
		name   string
		apply  func(*testing.T, *Supervisor, *fakeChild, *bufio.Scanner, providersession.Event)
		reason DisconnectReason
	}{
		{"duplicate", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
		}, DisconnectEventOrdering},
		{"malformed", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `[]`)
		}, DisconnectMalformedEnvelope},
		{"uncorrelated", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1"}`)
		}, DisconnectMalformedEnvelope},
		{"cross_thread", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-2","turnId":"turn-1","itemId":"item-1"}`)
		}, DisconnectCorrelationMismatch},
		{"cross_turn", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-2","itemId":"item-1"}`)
		}, DisconnectCorrelationMismatch},
		{"cross_item", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-2"}`)
		}, DisconnectCorrelationMismatch},
		{"reordered_resolution", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendNotification(t, child, "serverRequest/resolved", `{"threadId":"thread-1","requestId":44}`)
		}, DisconnectEventOrdering},
		{"unsupported_network", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","networkApprovalContext":{}}`)
		}, DisconnectUnsupportedEvent},
		{"stale_decision", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, event providersession.Event) {
			decision := providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny}
			decision.Correlation.DecisionID = "stale"
			_ = s.Decide(context.Background(), decision)
		}, DisconnectCorrelationMismatch},
	}
	for _, fixture := range tests {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, scanner, _, _ := startApprovalTurn(t)
			event := approvalRequest(t, s, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
			fixture.apply(t, s, child, scanner, event)
			if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(fixture.reason) {
				t.Fatalf("disconnect event = %+v", disconnected)
			}
		})
	}
}

func TestApprovalRelayRejectsDuplicateDecisionIdentity(t *testing.T) {
	s, child, scanner, _, _ := startApprovalTurn(t)
	event := approvalRequest(t, s, child, 47, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
	decision := providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny}
	done := make(chan error, 1)
	go func() { done <- s.Decide(context.Background(), decision) }()
	if !scanner.Scan() {
		t.Fatal("expected first decision response")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := s.Decide(context.Background(), decision); err == nil {
		t.Fatal("duplicate decision identity must be rejected")
	}
	if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectDecisionRejected) {
		t.Fatalf("duplicate decision event = %+v", disconnected)
	}
}

func TestApprovalRelayFailsClosedOnExpiryTransportChildExitProviderErrorAndReroute(t *testing.T) {
	t.Run("expiry", func(t *testing.T) {
		s, child, scanner, _, _ := startApprovalTurn(t)
		s.deadlines.Request = 20 * time.Millisecond
		_ = approvalRequest(t, s, child, 45, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
		if !scanner.Scan() {
			t.Fatal("expiry must send a private default-deny response")
		}
		var response struct {
			Result map[string]any `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil || !sameResult(response.Result, map[string]any{"decision": "decline"}) {
			t.Fatalf("expiry response = %s, err=%v", scanner.Text(), err)
		}
		if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectRequestDeadline) {
			t.Fatalf("expiry event = %+v", disconnected)
		}
	})
	for _, fixture := range []struct {
		name   string
		apply  func(*testing.T, *fakeChild)
		reason DisconnectReason
	}{
		{"transport", func(_ *testing.T, child *fakeChild) { _ = child.stdoutW.Close() }, DisconnectTransportClosed},
		{"child_exit", func(_ *testing.T, child *fakeChild) { child.exit(errors.New("died")) }, DisconnectChildExit},
		{"provider_error", func(t *testing.T, child *fakeChild) { sendServerRequestError(t, child, 46) }, DisconnectProviderError},
		{"reroute", func(t *testing.T, child *fakeChild) {
			sendNotification(t, child, "model/rerouted", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
		}, DisconnectModelRerouted},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, _, _, _ := startApprovalTurn(t)
			_ = approvalRequest(t, s, child, 46, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
			fixture.apply(t, child)
			if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(fixture.reason) {
				t.Fatalf("disconnect event = %+v", disconnected)
			}
		})
	}
}

func sendServerRequestError(t *testing.T, child *fakeChild, id uint64) {
	t.Helper()
	if _, err := child.stdoutW.Write([]byte(`{"jsonrpc":"2.0","id":` + jsonNumber(id) + `,"error":{"message":"private provider error"}}` + "\n")); err != nil {
		t.Fatal(err)
	}
}

func sameResult(got, want map[string]any) bool {
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	return string(gotJSON) == string(wantJSON)
}

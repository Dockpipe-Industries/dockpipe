package providersession

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func decisionCorrelation() Correlation {
	return Correlation{ProcessIncarnationID: "process", ConnectionID: "connection", SessionID: "session", InteractionID: "interaction", ActivityID: "activity", RequestID: "request", DecisionID: "decision"}
}

func modelCatalog() ModelReasoningCatalog {
	return ModelReasoningCatalog{
		CatalogRef: "catalog-1",
		Options: []ModelReasoningOption{
			{ModelRef: "model-a", ReasoningRef: "high"},
			{ModelRef: "model-b", ReasoningRef: "medium"},
		},
	}
}

func effectivePolicy() EffectivePolicySnapshot {
	return EffectivePolicySnapshot{
		Selection:             ModelReasoningSelection{CatalogRef: "catalog-1", ModelRef: "model-a", ReasoningRef: "high"},
		EffectiveModelRef:     "model-a",
		EffectiveReasoningRef: "high",
		Approval:              PolicySelection{SelectedRef: "native-review", EffectiveRef: "native-review"},
		Sandbox:               PolicySelection{SelectedRef: "full-access", EffectiveRef: "full-access", AuthorityExpanding: true, SessionConfirmed: true},
		Capabilities: []CapabilityRecord{
			{CapabilityRef: "stable-safe", Supported: true, UserEnabled: true},
			{CapabilityRef: "experimental-one", Supported: true, UserEnabled: true, Experimental: true, SessionConfirmed: true},
		},
	}
}

func TestFailClosedTransitions(t *testing.T) {
	if CanTransition(StateDisconnected, StateReady, false) {
		t.Fatal("unverified recovery must remain disconnected")
	}
	if !CanTransition(StateDisconnected, StateReady, true) {
		t.Fatal("verified recovery must permit ready")
	}
	if !CanTransition(StateRunning, StateWaitingForApproval, false) || !CanTransition(StateWaitingForApproval, StateRunning, false) {
		t.Fatal("approval wait transitions must be explicit")
	}
	if CanTransition(StateCompleted, StateRunning, true) {
		t.Fatal("terminal sessions must not restart")
	}
}

func TestSequenceRejectsDuplicateStaleAndGappedEvents(t *testing.T) {
	if err := ValidateNextSequence(7, 8); err != nil {
		t.Fatalf("next sequence: %v", err)
	}
	for _, next := range []uint64{0, 7, 6, 9} {
		if err := ValidateNextSequence(7, next); err == nil {
			t.Fatalf("sequence %d must be rejected", next)
		}
	}
}

func TestApprovalRequiresCompleteOneTimeCorrelation(t *testing.T) {
	event := Event{ContractVersion: ContractVersion, Sequence: 1, OccurredAt: time.Now(), Session: SessionRef{Provider: "example", SessionID: "session"}, Kind: EventApprovalRequested, Approval: &ApprovalRequest{Correlation: decisionCorrelation(), Reason: ApprovalReasonWorkspaceChange, AllowedDecisions: []string{DecisionApprove, DecisionDeny}}}
	if err := event.Validate(); err != nil {
		t.Fatalf("valid approval event: %v", err)
	}
	event.Approval.Correlation.DecisionID = ""
	if err := event.Validate(); err == nil {
		t.Fatal("missing decision identity must be rejected")
	}
}

func TestApprovalDecisionIsNeutralAndBounded(t *testing.T) {
	for _, decision := range []string{DecisionApprove, DecisionDeny} {
		if err := (ApprovalDecision{Correlation: decisionCorrelation(), Decision: decision}).Validate(); err != nil {
			t.Fatalf("valid decision %q: %v", decision, err)
		}
	}
	if err := (ApprovalDecision{Correlation: decisionCorrelation(), Decision: "acceptForSession"}).Validate(); err == nil {
		t.Fatal("provider-specific session grant must be rejected")
	}
	request := ApprovalRequest{Correlation: decisionCorrelation(), Reason: ApprovalReasonPermission, AllowedDecisions: []string{DecisionDeny}}
	if err := (ApprovalDecision{Correlation: decisionCorrelation(), Decision: DecisionDeny}).ValidateFor(request); err != nil {
		t.Fatalf("exact deny decision: %v", err)
	}
	if err := (ApprovalDecision{Correlation: decisionCorrelation(), Decision: DecisionApprove}).ValidateFor(request); err == nil {
		t.Fatal("decision outside the exact request set must be rejected")
	}
	substituted := ApprovalDecision{Correlation: decisionCorrelation(), Decision: DecisionDeny}
	substituted.Correlation.ConnectionID = "other-connection"
	if err := substituted.ValidateFor(request); err == nil {
		t.Fatal("substituted decision correlation must be rejected")
	}
	for _, malformed := range [][]string{nil, {DecisionDeny, DecisionDeny}, {DecisionApprove}, {DecisionDeny, DecisionApprove}} {
		candidate := request
		candidate.AllowedDecisions = malformed
		if err := candidate.Validate(); err == nil {
			t.Fatalf("malformed decision set accepted: %#v", malformed)
		}
	}
}

func TestCatalogAndEffectivePolicyRequireExactSelections(t *testing.T) {
	catalog := modelCatalog()
	if err := catalog.Validate(); err != nil {
		t.Fatalf("valid catalog: %v", err)
	}
	policy := effectivePolicy()
	if err := policy.Validate(catalog); err != nil {
		t.Fatalf("valid effective policy: %v", err)
	}

	policy.EffectiveModelRef = "model-b"
	if err := policy.Validate(catalog); err == nil {
		t.Fatal("silent model substitution must be rejected")
	}
	policy = effectivePolicy()
	policy.Approval.EffectiveRef = "automatic-review"
	if err := policy.Validate(catalog); err == nil {
		t.Fatal("silent approval-policy substitution must be rejected")
	}
	policy = effectivePolicy()
	policy.Selection.ReasoningRef = "medium"
	if err := policy.Validate(catalog); err == nil {
		t.Fatal("unavailable model and reasoning combination must be rejected")
	}

	catalog.Options = append(catalog.Options, catalog.Options[0])
	if err := catalog.Validate(); err == nil {
		t.Fatal("duplicate catalog combination must be rejected")
	}
}

func TestAuthorityAndExperimentalCapabilitiesFailClosed(t *testing.T) {
	catalog := modelCatalog()
	policy := effectivePolicy()
	policy.Sandbox.SessionConfirmed = false
	if err := policy.Validate(catalog); err == nil {
		t.Fatal("authority-expanding sandbox without session confirmation must be rejected")
	}

	policy = effectivePolicy()
	policy.Capabilities[0] = CapabilityRecord{CapabilityRef: "unknown-authority", AuthorityExpanding: true, UserEnabled: true, SessionConfirmed: true}
	if err := policy.Validate(catalog); err == nil {
		t.Fatal("unsupported authority-expanding capability must remain disabled")
	}

	policy = effectivePolicy()
	policy.Capabilities[1].SessionConfirmed = false
	if err := policy.Validate(catalog); err == nil {
		t.Fatal("experimental capability without individual confirmation must be rejected")
	}

	policy = effectivePolicy()
	policy.Approval = PolicySelection{SelectedRef: "automatic-review", EffectiveRef: "automatic-review", AuthorityExpanding: true, SessionConfirmed: true}
	policy.Sandbox = PolicySelection{SelectedRef: "workspace-write", EffectiveRef: "workspace-write"}
	if err := policy.Validate(catalog); err != nil {
		t.Fatalf("independent approval and sandbox choices: %v", err)
	}
}

func TestEventKindsRequireTheirSafeReferences(t *testing.T) {
	session := SessionRef{Provider: "example", SessionID: "session"}
	input := Event{ContractVersion: ContractVersion, Sequence: 1, OccurredAt: time.Now(), Session: session, Kind: EventUserInputRequested, UserInput: &UserInputRequest{Correlation: decisionCorrelation(), PromptRef: "prompt-1"}}
	if err := input.Validate(); err != nil {
		t.Fatalf("valid user-input event: %v", err)
	}
	cancellation := Event{ContractVersion: ContractVersion, Sequence: 2, OccurredAt: time.Now(), Session: session, Kind: EventCancellationRequested, Cancellation: &CancellationIntent{Session: session, Correlation: decisionCorrelation(), Reason: "user_requested"}}
	if err := cancellation.Validate(); err != nil {
		t.Fatalf("valid cancellation event: %v", err)
	}
}

func TestUserInputPromptAndResponseAreBoundedAndExactlyCorrelated(t *testing.T) {
	request := UserInputRequest{Correlation: decisionCorrelation(), PromptRef: "prompt-1"}
	prompt := UserInputPrompt{
		Correlation:   request.Correlation,
		PromptRef:     request.PromptRef,
		Kind:          UserInputPromptSelectOne,
		Summary:       "Choose one safe option",
		Options:       []UserInputOption{{OptionRef: "option-a", Label: "Option A"}, {OptionRef: "option-b", Label: "Option B"}},
		MaxSelections: 1,
	}
	if err := prompt.ValidateFor(request); err != nil {
		t.Fatalf("valid prompt: %v", err)
	}
	response := UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{"option-a"}}
	if err := response.ValidateFor(prompt); err != nil {
		t.Fatalf("valid response: %v", err)
	}

	response.Correlation.DecisionID = "stale-decision"
	if err := response.ValidateFor(prompt); err == nil {
		t.Fatal("stale response correlation must be rejected")
	}
	response = UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, SelectedOptionRefs: []string{"unknown-option"}}
	if err := response.ValidateFor(prompt); err == nil {
		t.Fatal("unknown option must be rejected")
	}
	prompt.Summary = "unsafe\nsummary"
	if err := prompt.ValidateFor(request); err == nil {
		t.Fatal("control characters in renderable prompt text must be rejected")
	}
}

func TestTextInputIsTransientAndBounded(t *testing.T) {
	request := UserInputRequest{Correlation: decisionCorrelation(), PromptRef: "prompt-text"}
	prompt := UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Kind: UserInputPromptText, Summary: "Provide a short answer", MaxTextBytes: 12}
	if err := prompt.ValidateFor(request); err != nil {
		t.Fatalf("valid text prompt: %v", err)
	}
	if err := (UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, Text: "short answer"}).ValidateFor(prompt); err != nil {
		t.Fatalf("valid text response: %v", err)
	}
	if err := (UserInputResponse{Correlation: request.Correlation, PromptRef: request.PromptRef, Text: "answer exceeds bound"}).ValidateFor(prompt); err == nil {
		t.Fatal("oversized text response must be rejected")
	}
}

func TestCancellationReasonsAreNeutralAndBounded(t *testing.T) {
	intent := CancellationIntent{Session: SessionRef{Provider: "example", SessionID: "session"}, Correlation: decisionCorrelation(), Reason: CancellationReasonUserRequested}
	for _, reason := range []string{CancellationReasonUserRequested, CancellationReasonSafetyStop, CancellationReasonDeadline} {
		intent.Reason = reason
		if err := intent.Validate(); err != nil {
			t.Fatalf("valid cancellation reason %q: %v", reason, err)
		}
	}
	intent.Reason = "provider_specific_reason"
	if err := intent.Validate(); err == nil {
		t.Fatal("unbounded cancellation reason must be rejected")
	}
}

func TestRecoveryRequestRequiresExactBoundedEvidence(t *testing.T) {
	request := RecoveryRequest{Session: SessionRef{Provider: "example", SessionID: "session"}, RecoveryEvidence: "recovery-safe_1"}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid recovery request: %v", err)
	}
	for _, evidence := range []string{"", "unsafe/evidence", strings.Repeat("x", 129)} {
		request.RecoveryEvidence = evidence
		if err := request.Validate(); err == nil {
			t.Fatalf("unsafe recovery evidence %q was accepted", evidence)
		}
	}
}

func TestContractSourceDoesNotLeakProviderProtocolTypes(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("contract source location unavailable")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(source), "contract.go"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"codex", "jsonrpc", "rawmessage", "threadid", "turnid", "itemid", "credential", "token", "requestapproval", "requestuserinput", "serverrequest"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("generic contract leaks forbidden provider detail %q", forbidden)
		}
	}
}

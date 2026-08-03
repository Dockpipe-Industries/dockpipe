package appserversupervisor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"dorkpipe.orchestrator/providersession"
)

// CAS-07 keeps App Server request shapes private. It projects only a small
// neutral subset and never persists command text, patches, prompts, messages,
// provider request IDs, permission payloads, answers, or private mappings.
var (
	ErrDecisionUnavailable          = errors.New("app server approval decision is unavailable")
	ErrDecisionRejected             = errors.New("app server approval decision was rejected")
	ErrUserInputPromptUnavailable   = errors.New("app server user-input prompt is unavailable")
	ErrUserInputPromptRejected      = errors.New("app server user-input prompt was rejected")
	ErrUserInputResponseUnavailable = errors.New("app server user-input response is unavailable")
	ErrUserInputResponseRejected    = errors.New("app server user-input response was rejected")
)

type pendingKind uint8

const (
	pendingCommand pendingKind = iota + 1
	pendingFileChange
	pendingPermission
	pendingUserInput
)

type pendingRequest struct {
	providerID       uint64
	kind             pendingKind
	correlation      providersession.Correlation
	prompt           *providersession.UserInputPrompt
	input            *pendingUserInputState
	decisionInFlight bool
	timer            *time.Timer
}

type pendingUserInputState struct {
	questionID     string
	answerByOption map[string]string
}

type providerUserInputQuestion struct {
	id      string
	summary string
	options []providerUserInputOption
}

type providerUserInputOption struct {
	answer      string
	label       string
	description string
}

type serverRequestParams struct {
	ThreadID                     string            `json:"threadId"`
	TurnID                       string            `json:"turnId"`
	ItemID                       string            `json:"itemId"`
	GrantRoot                    string            `json:"grantRoot"`
	AdditionalPermissions        json.RawMessage   `json:"additionalPermissions"`
	NetworkApprovalContext       json.RawMessage   `json:"networkApprovalContext"`
	ProposedExecpolicyAmendment  json.RawMessage   `json:"proposedExecpolicyAmendment"`
	ProposedNetworkPolicyChanges json.RawMessage   `json:"proposedNetworkPolicyAmendments"`
	Permissions                  json.RawMessage   `json:"permissions"`
	Questions                    []json.RawMessage `json:"questions"`
}

func (s *Supervisor) handleServerRequest(providerID uint64, method string, raw json.RawMessage) DisconnectReason {
	if len(raw) == 0 || !json.Valid(raw) || containsModelReroute(raw) {
		if containsModelReroute(raw) {
			return DisconnectModelRerouted
		}
		return DisconnectMalformedEnvelope
	}
	var params serverRequestParams
	if json.Unmarshal(raw, &params) != nil {
		return DisconnectMalformedEnvelope
	}
	if !serverRequestShapeAllowed(method, raw) {
		return DisconnectUnsupportedEvent
	}
	kind, actionClass, scope, reason := classifyServerRequest(method, params)
	if reason != "" {
		return reason
	}
	var inputQuestion *providerUserInputQuestion
	if kind == pendingUserInput {
		parsed, ok := parseProviderUserInputQuestion(params.Questions)
		if !ok {
			return DisconnectUnsupportedEvent
		}
		inputQuestion = &parsed
	}

	s.mu.Lock()
	if !s.started || !s.initialized || !s.lifecycle.active {
		s.lastNotification = "approval_inactive"
		s.mu.Unlock()
		return DisconnectEventOrdering
	}
	if !validID(params.ThreadID) || !validID(params.TurnID) || !validID(params.ItemID) {
		s.mu.Unlock()
		return DisconnectMalformedEnvelope
	}
	if params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID || params.ItemID != s.lifecycle.itemID {
		if params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID {
			s.lastNotification = "approval_turn_mismatch"
		} else {
			s.lastNotification = "approval_item_mismatch"
		}
		s.mu.Unlock()
		return DisconnectCorrelationMismatch
	}
	if s.state != providersession.StateRunning || s.lifecycle.pending != nil {
		s.lastNotification = "approval_not_running"
		s.mu.Unlock()
		return DisconnectEventOrdering
	}
	if kind == pendingPermission && !s.permissionScopeDeclared(params.Permissions) {
		s.mu.Unlock()
		return DisconnectUnsupportedEvent
	}
	s.lifecycle.requestCounter++
	requestRef := "request-" + strconv.FormatUint(s.lifecycle.requestCounter, 10)
	decisionRef := "decision-" + strconv.FormatUint(s.lifecycle.requestCounter, 10)
	correlation := s.eventCorrelation(s.lifecycle.turnID, s.lifecycle.itemID)
	correlation.RequestID, correlation.DecisionID = requestRef, decisionRef
	pending := &pendingRequest{providerID: providerID, kind: kind, correlation: correlation}
	if kind == pendingUserInput {
		request := providersession.UserInputRequest{Correlation: correlation, PromptRef: requestRef}
		prompt, input, ok := projectProviderUserInput(*inputQuestion, request)
		if !ok {
			s.mu.Unlock()
			return DisconnectUnsupportedEvent
		}
		pending.prompt = &prompt
		pending.input = &input
	}
	s.lifecycle.pending = pending
	pending.timer = time.AfterFunc(s.deadlines.Request, func() { s.expirePending(correlation) })
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Correlation: correlation}
	if kind == pendingUserInput {
		s.state = providersession.StateWaitingForUserInput
		event.Kind, event.State, event.Summary = providersession.EventUserInputRequested, s.state, "user_input_requested"
		event.UserInput = &providersession.UserInputRequest{Correlation: correlation, PromptRef: requestRef}
	} else {
		s.state = providersession.StateWaitingForApproval
		event.Kind, event.State, event.Summary = providersession.EventApprovalRequested, s.state, "approval_requested"
		event.Approval = &providersession.ApprovalRequest{Correlation: correlation, ActionClass: actionClass, Summary: actionClass + "_approval", Scope: scope}
	}
	s.mu.Unlock()
	if !s.publish(event, "approval", "accepted", "waiting") {
		return DisconnectAuditFailure
	}
	return ""
}

func serverRequestShapeAllowed(method string, raw json.RawMessage) bool {
	var allowed []string
	switch method {
	case "item/commandExecution/requestApproval":
		allowed = []string{"threadId", "turnId", "itemId", "command", "cwd", "reason"}
	case "item/fileChange/requestApproval":
		allowed = []string{"threadId", "turnId", "itemId", "patch", "reason", "grantRoot", "startedAtMs"}
	case "item/permissions/requestApproval":
		allowed = []string{"threadId", "turnId", "itemId", "permissions", "additionalPermissions", "networkApprovalContext", "proposedExecpolicyAmendment", "proposedNetworkPolicyAmendments"}
	case "item/tool/requestUserInput":
		allowed = []string{"threadId", "turnId", "itemId", "questions", "autoResolutionMs"}
	default:
		return false
	}
	fields, ok := objectFields(raw, allowed...)
	if !ok {
		return false
	}
	if startedAt, found := fields["startedAtMs"]; found && !validRequestTimestamp(startedAt) {
		return false
	}
	if autoResolution, found := fields["autoResolutionMs"]; found && !explicitJSONNull(autoResolution) {
		return false
	}
	if questions, found := fields["questions"]; found {
		var values []json.RawMessage
		if json.Unmarshal(questions, &values) != nil {
			return false
		}
		for _, value := range values {
			questionFields, allowed := objectFields(value, "id", "header", "question", "options", "isOther", "isSecret")
			if !allowed || !absentOrExplicitFalse(questionFields, "isOther") || !absentOrExplicitFalse(questionFields, "isSecret") {
				return false
			}
		}
	}
	return true
}

// validRequestTimestamp validates the current App Server's optional event
// timestamp without storing or projecting it. It has no approval semantics.
func validRequestTimestamp(raw json.RawMessage) bool {
	var value uint64
	return json.Unmarshal(raw, &value) == nil
}

func explicitJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func absentOrExplicitFalse(fields map[string]json.RawMessage, name string) bool {
	raw, found := fields[name]
	if !found {
		return true
	}
	var value *bool
	return json.Unmarshal(raw, &value) == nil && value != nil && !*value
}

func classifyServerRequest(method string, params serverRequestParams) (pendingKind, string, []string, DisconnectReason) {
	switch method {
	case "item/commandExecution/requestApproval":
		if unsafeApprovalExtension(params) {
			return 0, "", nil, DisconnectUnsupportedEvent
		}
		return pendingCommand, "command_execution", []string{"turn"}, ""
	case "item/fileChange/requestApproval":
		if strings.TrimSpace(params.GrantRoot) != "" {
			return 0, "", nil, DisconnectUnsupportedEvent
		}
		return pendingFileChange, "workspace_change", []string{"turn"}, ""
	case "item/permissions/requestApproval":
		if len(params.Permissions) == 0 || unsafeApprovalExtension(params) {
			return 0, "", nil, DisconnectUnsupportedEvent
		}
		return pendingPermission, "declared_permission", []string{"declared_writable_roots"}, ""
	case "item/tool/requestUserInput":
		return pendingUserInput, "", nil, ""
	default:
		return 0, "", nil, DisconnectUnsupportedEvent
	}
}

func unsafeApprovalExtension(params serverRequestParams) bool {
	return presentJSON(params.AdditionalPermissions) || presentJSON(params.NetworkApprovalContext) || presentJSON(params.ProposedExecpolicyAmendment) || presentJSON(params.ProposedNetworkPolicyChanges)
}

func presentJSON(raw json.RawMessage) bool {
	return len(raw) != 0 && string(raw) != "null"
}

func parseProviderUserInputQuestion(questions []json.RawMessage) (providerUserInputQuestion, bool) {
	// The neutral contract represents one prompt and one response. A provider
	// batch cannot be partially answered or correlated safely by this slice.
	if len(questions) != 1 {
		return providerUserInputQuestion{}, false
	}
	fields, ok := objectFields(questions[0], "id", "header", "question", "options", "isOther", "isSecret")
	if !ok || !absentOrExplicitFalse(fields, "isOther") || !absentOrExplicitFalse(fields, "isSecret") {
		return providerUserInputQuestion{}, false
	}
	for _, required := range []string{"id", "header", "question"} {
		if _, found := fields[required]; !found {
			return providerUserInputQuestion{}, false
		}
	}
	var question struct {
		ID       string            `json:"id"`
		Header   string            `json:"header"`
		Question string            `json:"question"`
		Options  []json.RawMessage `json:"options"`
	}
	if json.Unmarshal(questions[0], &question) != nil || !validID(question.ID) || len(question.Options) > 16 {
		return providerUserInputQuestion{}, false
	}
	if _, ok := normalizeUserInputDisplayText(question.Header, 128); !ok {
		return providerUserInputQuestion{}, false
	}
	summary, ok := normalizeUserInputDisplayText(question.Question, 512)
	if !ok {
		return providerUserInputQuestion{}, false
	}
	parsed := providerUserInputQuestion{id: question.ID, summary: summary, options: make([]providerUserInputOption, 0, len(question.Options))}
	answers := make(map[string]struct{}, len(question.Options))
	displayLabels := make(map[string]struct{}, len(question.Options))
	for _, raw := range question.Options {
		optionFields, allowed := objectFields(raw, "label", "description")
		if !allowed {
			return providerUserInputQuestion{}, false
		}
		if _, found := optionFields["label"]; !found {
			return providerUserInputQuestion{}, false
		}
		if _, found := optionFields["description"]; !found {
			return providerUserInputQuestion{}, false
		}
		var option struct {
			Label       string `json:"label"`
			Description string `json:"description"`
		}
		if json.Unmarshal(raw, &option) != nil || len(option.Label) > 4096 || len(option.Description) > 4096 || !utf8.ValidString(option.Label) || !utf8.ValidString(option.Description) || strings.ContainsRune(option.Label, '\x00') || strings.ContainsRune(option.Description, '\x00') {
			return providerUserInputQuestion{}, false
		}
		if _, duplicate := answers[option.Label]; duplicate {
			return providerUserInputQuestion{}, false
		}
		label, ok := normalizeUserInputDisplayText(option.Label, 128)
		if !ok {
			return providerUserInputQuestion{}, false
		}
		if _, duplicate := displayLabels[label]; duplicate {
			return providerUserInputQuestion{}, false
		}
		answers[option.Label] = struct{}{}
		displayLabels[label] = struct{}{}
		parsed.options = append(parsed.options, providerUserInputOption{answer: option.Label, label: label, description: option.Description})
	}
	return parsed, true
}

func normalizeUserInputDisplayText(value string, maximum int) (string, bool) {
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return "", false
	}
	normalized := strings.Join(strings.Fields(value), " ")
	if normalized == "" || len(normalized) > maximum {
		return "", false
	}
	for _, character := range normalized {
		if unicode.IsControl(character) {
			return "", false
		}
	}
	return normalized, true
}

func projectProviderUserInput(question providerUserInputQuestion, request providersession.UserInputRequest) (providersession.UserInputPrompt, pendingUserInputState, bool) {
	prompt := providersession.UserInputPrompt{Correlation: request.Correlation, PromptRef: request.PromptRef, Summary: question.summary}
	input := pendingUserInputState{questionID: question.id}
	if len(question.options) == 0 {
		prompt.Kind = providersession.UserInputPromptText
		prompt.MaxTextBytes = 4096
	} else {
		prompt.Kind = providersession.UserInputPromptSelectOne
		prompt.MaxSelections = 1
		prompt.Options = make([]providersession.UserInputOption, 0, len(question.options))
		input.answerByOption = make(map[string]string, len(question.options))
		for _, option := range question.options {
			optionRef := providerUserInputOptionReference(request.Correlation, question.id, option)
			if _, duplicate := input.answerByOption[optionRef]; duplicate {
				return providersession.UserInputPrompt{}, pendingUserInputState{}, false
			}
			prompt.Options = append(prompt.Options, providersession.UserInputOption{OptionRef: optionRef, Label: option.label})
			input.answerByOption[optionRef] = option.answer
		}
	}
	if err := prompt.ValidateFor(request); err != nil {
		return providersession.UserInputPrompt{}, pendingUserInputState{}, false
	}
	return prompt, input, true
}

func providerUserInputOptionReference(correlation providersession.Correlation, questionID string, option providerUserInputOption) string {
	hash := sha256.New()
	for _, value := range []string{
		correlation.ProcessIncarnationID,
		correlation.ConnectionID,
		correlation.SessionID,
		correlation.InteractionID,
		correlation.ActivityID,
		correlation.RequestID,
		correlation.DecisionID,
		questionID,
		option.answer,
		option.description,
	} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return fmt.Sprintf("option-%x", hash.Sum(nil))
}

// UserInputPrompt returns a defensive copy of the one normalized prompt bound
// to the exact current request. Lookup is read-only and does not consume,
// answer, persist, replay, or dispatch the prompt.
func (s *Supervisor) UserInputPrompt(ctx context.Context, request providersession.UserInputRequest) (providersession.UserInputPrompt, error) {
	if err := ctx.Err(); err != nil {
		return providersession.UserInputPrompt{}, err
	}
	if err := request.Validate(); err != nil {
		return providersession.UserInputPrompt{}, s.rejectUserInputPrompt(DisconnectUnsupportedEvent, ErrUserInputPromptRejected)
	}
	s.mu.RLock()
	pending := s.lifecycle.pending
	ready := s.started && s.initialized && s.state == providersession.StateWaitingForUserInput && pending != nil && pending.kind == pendingUserInput
	if !ready {
		s.mu.RUnlock()
		return providersession.UserInputPrompt{}, s.rejectUserInputPrompt(DisconnectDecisionRejected, ErrUserInputPromptUnavailable)
	}
	current := providersession.UserInputRequest{Correlation: pending.correlation, PromptRef: pending.correlation.RequestID}
	if request != current {
		s.mu.RUnlock()
		return providersession.UserInputPrompt{}, s.rejectUserInputPrompt(DisconnectCorrelationMismatch, ErrUserInputPromptRejected)
	}
	if pending.prompt == nil {
		s.mu.RUnlock()
		return providersession.UserInputPrompt{}, s.rejectUserInputPrompt(DisconnectUnsupportedEvent, ErrUserInputPromptUnavailable)
	}
	prompt := cloneUserInputPrompt(*pending.prompt)
	s.mu.RUnlock()
	if err := prompt.ValidateFor(request); err != nil {
		return providersession.UserInputPrompt{}, s.rejectUserInputPrompt(DisconnectUnsupportedEvent, ErrUserInputPromptRejected)
	}
	return prompt, nil
}

func (s *Supervisor) rejectUserInputPrompt(reason DisconnectReason, err error) error {
	s.fail(reason)
	return err
}

func cloneUserInputPrompt(prompt providersession.UserInputPrompt) providersession.UserInputPrompt {
	prompt.Options = append([]providersession.UserInputOption(nil), prompt.Options...)
	return prompt
}

// RespondUserInput delivers one exact validated response to the current private
// App Server request. Answer content and provider mapping are used only to build
// this write and are cleared before control returns to the caller.
func (s *Supervisor) RespondUserInput(parent context.Context, response providersession.UserInputResponse) error {
	started := time.Now()
	s.mu.Lock()
	pending := s.lifecycle.pending
	ready := s.started && s.initialized && s.state == providersession.StateWaitingForUserInput && pending != nil && pending.kind == pendingUserInput && pending.prompt != nil && pending.input != nil
	if !ready {
		s.mu.Unlock()
		return s.rejectUserInputResponse(DisconnectDecisionRejected, ErrUserInputResponseUnavailable)
	}
	if response.Correlation != pending.correlation || response.PromptRef != pending.prompt.PromptRef {
		s.mu.Unlock()
		return s.rejectUserInputResponse(DisconnectCorrelationMismatch, ErrUserInputResponseRejected)
	}
	if pending.decisionInFlight {
		s.mu.Unlock()
		return s.rejectUserInputResponse(DisconnectDecisionRejected, ErrUserInputResponseRejected)
	}
	if err := response.ValidateFor(*pending.prompt); err != nil {
		s.mu.Unlock()
		return s.rejectUserInputResponse(DisconnectDecisionRejected, ErrUserInputResponseRejected)
	}
	result, ok := userInputResult(response, *pending.input)
	if !ok {
		s.mu.Unlock()
		return s.rejectUserInputResponse(DisconnectDecisionRejected, ErrUserInputResponseRejected)
	}
	client, providerID, correlation := s.client, pending.providerID, pending.correlation
	pending.decisionInFlight = true
	// Clear all prompt content and private mapping before the provider write.
	pending.prompt = nil
	pending.input = nil
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, s.deadlines.Request)
	defer cancel()
	if client == nil || client.respond(ctx, providerID, result) != nil {
		s.fail(DisconnectTransportClosed)
		return ErrUserInputResponseUnavailable
	}
	if !s.auditOperation("user_input", "delivered", "waiting", "user_input_delivered", correlation, started) {
		return ErrUserInputResponseUnavailable
	}
	return nil
}

func userInputResult(response providersession.UserInputResponse, input pendingUserInputState) (map[string]any, bool) {
	answers := make([]string, 0, len(response.SelectedOptionRefs)+1)
	if response.Text != "" {
		answers = append(answers, response.Text)
	} else {
		for _, optionRef := range response.SelectedOptionRefs {
			answer, found := input.answerByOption[optionRef]
			if !found {
				return nil, false
			}
			answers = append(answers, answer)
		}
	}
	return map[string]any{"answers": map[string]any{input.questionID: map[string]any{"answers": answers}}}, true
}

func (s *Supervisor) rejectUserInputResponse(reason DisconnectReason, err error) error {
	s.fail(reason)
	return err
}

func (s *Supervisor) permissionScopeDeclared(raw json.RawMessage) bool {
	fields, ok := objectFields(raw, "fileSystem")
	if !ok {
		return false
	}
	if !nestedObjectFields(fields["fileSystem"], "write") {
		return false
	}
	var permissions struct {
		FileSystem map[string]json.RawMessage `json:"fileSystem"`
	}
	if json.Unmarshal(raw, &permissions) != nil || len(permissions.FileSystem) != 1 {
		return false
	}
	write, found := permissions.FileSystem["write"]
	if !found {
		return false
	}
	var roots []string
	if json.Unmarshal(write, &roots) != nil || len(roots) == 0 {
		return false
	}
	for _, root := range roots {
		if !s.lifecycle.declaredRoots[root] {
			return false
		}
	}
	return true
}

// Decide maps the existing neutral one-turn approve/deny decision to the
// private App Server response. It cannot grant session access, amend policy,
// answer a user-input request, or approve a permission profile.
func (s *Supervisor) Decide(parent context.Context, decision providersession.ApprovalDecision) error {
	started := time.Now()
	if err := decision.Validate(); err != nil {
		return s.rejectDecision(DisconnectDecisionRejected)
	}
	s.mu.Lock()
	if !s.started || !s.initialized || s.state == providersession.StateDisconnected || s.lifecycle.pending == nil {
		s.mu.Unlock()
		return s.rejectDecision(DisconnectDecisionRejected)
	}
	pending := s.lifecycle.pending
	if decision.Correlation != pending.correlation {
		s.mu.Unlock()
		return s.rejectDecision(DisconnectCorrelationMismatch)
	}
	if pending.decisionInFlight || pending.kind == pendingUserInput || pending.kind == pendingPermission && decision.Decision != providersession.DecisionDeny {
		s.mu.Unlock()
		return s.rejectDecision(DisconnectDecisionRejected)
	}
	client := s.client
	pending.decisionInFlight = true
	providerID, result := pending.providerID, decisionResult(pending.kind, decision.Decision)
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, s.deadlines.Request)
	defer cancel()
	if client == nil || client.respond(ctx, providerID, result) != nil {
		s.fail(DisconnectTransportClosed)
		return ErrDecisionUnavailable
	}
	if !s.auditOperation("approval", "delivered", "waiting", "approval_delivered", decision.Correlation, started) {
		return ErrDecisionUnavailable
	}
	return nil
}

func decisionResult(kind pendingKind, decision string) map[string]any {
	if kind == pendingPermission {
		return map[string]any{"permissions": map[string]any{}}
	}
	if decision == providersession.DecisionApprove {
		return map[string]any{"decision": "accept"}
	}
	return map[string]any{"decision": "decline"}
}

func (s *Supervisor) rejectDecision(reason DisconnectReason) error {
	s.fail(reason)
	return ErrDecisionRejected
}

func (s *Supervisor) expirePending(correlation providersession.Correlation) {
	s.mu.Lock()
	if s.state == providersession.StateDisconnected || s.lifecycle.pending == nil || s.lifecycle.pending.correlation != correlation {
		s.mu.Unlock()
		return
	}
	pending, client := s.lifecycle.pending, s.client
	s.lifecycle.pending = nil
	s.mu.Unlock()
	operation := "approval"
	if pending.kind == pendingUserInput {
		operation = "user_input"
	}
	if !s.auditOperation(operation, "expired", "waiting", "request_expired", correlation, time.Time{}) {
		return
	}
	if pending.kind != pendingUserInput && client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), s.deadlines.Request)
		_ = client.respond(ctx, pending.providerID, decisionResult(pending.kind, providersession.DecisionDeny))
		cancel()
	}
	s.fail(DisconnectRequestDeadline)
}

func (s *Supervisor) handleServerRequestResolved(params eventParams) DisconnectReason {
	providerID, ok := providerRequestID(params.RequestID)
	if !ok || !validID(params.ThreadID) {
		return DisconnectMalformedEnvelope
	}
	s.mu.Lock()
	if !s.started || !s.initialized || !s.lifecycle.active || s.lifecycle.pending == nil || params.ThreadID != s.lifecycle.threadID {
		s.mu.Unlock()
		return DisconnectEventOrdering
	}
	pending := s.lifecycle.pending
	if pending.providerID != providerID {
		s.mu.Unlock()
		return DisconnectCorrelationMismatch
	}
	wantState := providersession.StateWaitingForApproval
	if pending.kind == pendingUserInput {
		wantState = providersession.StateWaitingForUserInput
	}
	if !pending.decisionInFlight || s.state != wantState {
		s.mu.Unlock()
		return DisconnectEventOrdering
	}
	if pending.timer != nil {
		pending.timer.Stop()
	}
	s.lifecycle.pending = nil
	s.state = providersession.StateRunning
	s.sequence++
	operation, summary := "approval", "approval_resolved"
	if pending.kind == pendingUserInput {
		operation, summary = "user_input", "user_input_resolved"
	}
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventStateChanged, State: s.state, Correlation: pending.correlation, Summary: summary}
	s.mu.Unlock()
	if !s.publish(event, operation, "resolved", "running") {
		return DisconnectAuditFailure
	}
	return ""
}

func providerRequestID(raw json.RawMessage) (uint64, bool) {
	var id uint64
	if len(raw) == 0 || json.Unmarshal(raw, &id) != nil {
		return 0, false
	}
	return id, true
}

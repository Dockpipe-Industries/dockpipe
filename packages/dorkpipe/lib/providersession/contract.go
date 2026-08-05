// Package providersession defines the provider-neutral contract for top-level sessions.
// Implementations belong to future adapter slices; this package owns no transport or process behavior.
package providersession

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const ContractVersion = "dorkpipe.provider_session.v1"

const (
	maxOpaqueReferenceBytes   = 128
	maxCatalogOptions         = 128
	maxCapabilities           = 64
	maxPromptSummaryBytes     = 512
	maxPromptOptions          = 16
	maxPromptOptionLabelBytes = 128
	maxUserInputTextBytes     = 4096
)

func validateOpaqueReference(value, name string) error {
	if len(value) == 0 || len(value) > maxOpaqueReferenceBytes {
		return errors.New(name + " must be a bounded opaque reference")
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' || character == ':') {
			return errors.New(name + " must be a safe opaque reference")
		}
	}
	return nil
}

func validateDisplayText(value, name string, limit int) error {
	if strings.TrimSpace(value) == "" || len(value) > limit || !utf8.ValidString(value) {
		return errors.New(name + " must be bounded display text")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New(name + " must not contain control characters")
		}
	}
	return nil
}

// ModelReasoningCatalog contains only validated, currently available stable
// combinations. References are opaque: consumers may select and display them,
// but cannot derive provider behavior from their contents.
type ModelReasoningCatalog struct {
	CatalogRef string                 `json:"catalog_ref"`
	Options    []ModelReasoningOption `json:"options"`
}

type ModelReasoningOption struct {
	ModelRef     string `json:"model_ref"`
	ReasoningRef string `json:"reasoning_ref"`
}

func (c ModelReasoningCatalog) Validate() error {
	if err := validateOpaqueReference(c.CatalogRef, "catalog reference"); err != nil {
		return err
	}
	if len(c.Options) == 0 || len(c.Options) > maxCatalogOptions {
		return errors.New("bounded model and reasoning options are required")
	}
	seen := make(map[string]struct{}, len(c.Options))
	for _, option := range c.Options {
		if err := validateOpaqueReference(option.ModelRef, "model reference"); err != nil {
			return err
		}
		if err := validateOpaqueReference(option.ReasoningRef, "reasoning reference"); err != nil {
			return err
		}
		key := option.ModelRef + "\x00" + option.ReasoningRef
		if _, exists := seen[key]; exists {
			return errors.New("model and reasoning options must be unique")
		}
		seen[key] = struct{}{}
	}
	return nil
}

type ModelReasoningSelection struct {
	CatalogRef   string `json:"catalog_ref"`
	ModelRef     string `json:"model_ref"`
	ReasoningRef string `json:"reasoning_ref"`
}

func (s ModelReasoningSelection) Validate(catalog ModelReasoningCatalog) error {
	if err := catalog.Validate(); err != nil {
		return err
	}
	if s.CatalogRef != catalog.CatalogRef {
		return errors.New("model selection must use the exact catalog reference")
	}
	for _, option := range catalog.Options {
		if s.ModelRef == option.ModelRef && s.ReasoningRef == option.ReasoningRef {
			return nil
		}
	}
	return errors.New("model and reasoning selection is not available")
}

// PolicySelection records one independently selected policy dimension. Any
// authority-expanding choice requires confirmation for this session; callers
// must not copy that confirmation into a new session.
type PolicySelection struct {
	SelectedRef        string `json:"selected_ref"`
	EffectiveRef       string `json:"effective_ref"`
	AuthorityExpanding bool   `json:"authority_expanding"`
	SessionConfirmed   bool   `json:"session_confirmed"`
}

func (p PolicySelection) validate(name string) error {
	if err := validateOpaqueReference(p.SelectedRef, name+" selected reference"); err != nil {
		return err
	}
	if err := validateOpaqueReference(p.EffectiveRef, name+" effective reference"); err != nil {
		return err
	}
	if p.SelectedRef != p.EffectiveRef {
		return errors.New(name + " policy must not be silently substituted")
	}
	if p.AuthorityExpanding != p.SessionConfirmed {
		return errors.New(name + " authority confirmation must be explicit and per-session")
	}
	return nil
}

// CapabilityRecord is a safe projection, not a provider capability union.
// Unsupported capabilities cannot be enabled. Experimental and
// authority-expanding capabilities require their own per-session confirmation.
type CapabilityRecord struct {
	CapabilityRef      string `json:"capability_ref"`
	Supported          bool   `json:"supported"`
	UserEnabled        bool   `json:"user_enabled"`
	AuthorityExpanding bool   `json:"authority_expanding"`
	Experimental       bool   `json:"experimental"`
	SessionConfirmed   bool   `json:"session_confirmed"`
}

func (c CapabilityRecord) Validate() error {
	if err := validateOpaqueReference(c.CapabilityRef, "capability reference"); err != nil {
		return err
	}
	if c.UserEnabled && !c.Supported {
		return errors.New("unsupported capability must remain disabled")
	}
	requiresConfirmation := c.UserEnabled && (c.AuthorityExpanding || c.Experimental)
	if c.SessionConfirmed != requiresConfirmation {
		return errors.New("capability confirmation must be explicit, individual, and per-session")
	}
	return nil
}

// EffectivePolicySnapshot is safe for a consumer to render. It deliberately
// excludes adapter identity and provider protocol values. Selected and effective
// references must match so policy changes fail visibly instead of substituting.
type EffectivePolicySnapshot struct {
	Selection             ModelReasoningSelection `json:"selection"`
	EffectiveModelRef     string                  `json:"effective_model_ref"`
	EffectiveReasoningRef string                  `json:"effective_reasoning_ref"`
	Approval              PolicySelection         `json:"approval"`
	Sandbox               PolicySelection         `json:"sandbox"`
	Capabilities          []CapabilityRecord      `json:"capabilities,omitempty"`
}

func (p EffectivePolicySnapshot) Validate(catalog ModelReasoningCatalog) error {
	if err := p.Selection.Validate(catalog); err != nil {
		return err
	}
	if p.EffectiveModelRef != p.Selection.ModelRef || p.EffectiveReasoningRef != p.Selection.ReasoningRef {
		return errors.New("effective model and reasoning must exactly match the selection")
	}
	if err := p.Approval.validate("approval"); err != nil {
		return err
	}
	if err := p.Sandbox.validate("sandbox"); err != nil {
		return err
	}
	if len(p.Capabilities) > maxCapabilities {
		return errors.New("capability projection exceeds its bound")
	}
	seen := make(map[string]struct{}, len(p.Capabilities))
	for _, capability := range p.Capabilities {
		if err := capability.Validate(); err != nil {
			return err
		}
		if _, exists := seen[capability.CapabilityRef]; exists {
			return errors.New("capability references must be unique")
		}
		seen[capability.CapabilityRef] = struct{}{}
	}
	return nil
}

type State string

const (
	StateReady               State = "ready"
	StateRunning             State = "running"
	StateWaitingForApproval  State = "waiting_for_approval"
	StateWaitingForUserInput State = "waiting_for_user_input"
	StateCompleted           State = "completed"
	StateCancelled           State = "cancelled"
	StateFailed              State = "failed"
	StateDisconnected        State = "disconnected"
)

func (s State) IsTerminal() bool {
	return s == StateCompleted || s == StateCancelled || s == StateFailed
}

func (s State) IsKnown() bool {
	switch s {
	case StateReady, StateRunning, StateWaitingForApproval, StateWaitingForUserInput, StateCompleted, StateCancelled, StateFailed, StateDisconnected:
		return true
	default:
		return false
	}
}

// CanTransition is intentionally fail-closed. A disconnected session may return to ready
// only when a future adapter has completed its verified recovery check.
func CanTransition(from, to State, recoveryVerified bool) bool {
	if !from.IsKnown() || !to.IsKnown() || from == to || from.IsTerminal() {
		return false
	}
	switch from {
	case StateReady:
		return to == StateRunning || to == StateFailed || to == StateDisconnected
	case StateRunning:
		return to == StateWaitingForApproval || to == StateWaitingForUserInput || to.IsTerminal() || to == StateDisconnected
	case StateWaitingForApproval, StateWaitingForUserInput:
		return to == StateRunning || to == StateCancelled || to == StateFailed || to == StateDisconnected
	case StateDisconnected:
		return to == StateReady && recoveryVerified
	default:
		return false
	}
}

// ValidateNextSequence rejects duplicate, stale, and gapped events before an
// adapter applies them. Persistence and reconciliation remain future work.
func ValidateNextSequence(previous, next uint64) error {
	if next == 0 || next != previous+1 {
		return errors.New("event sequence must advance by one")
	}
	return nil
}

type SessionRef struct {
	Provider  string `json:"provider"`
	SessionID string `json:"session_id"`
}

func (r SessionRef) Validate() error {
	if strings.TrimSpace(r.Provider) == "" || strings.TrimSpace(r.SessionID) == "" {
		return errors.New("provider and session identity are required")
	}
	return nil
}

// Correlation is opaque to callers. A provider adapter maps its own identifiers into
// these neutral scopes and must require every field for a one-time human decision.
type Correlation struct {
	ProcessIncarnationID string `json:"process_incarnation_id"`
	ConnectionID         string `json:"connection_id"`
	SessionID            string `json:"session_id"`
	InteractionID        string `json:"interaction_id"`
	ActivityID           string `json:"activity_id"`
	RequestID            string `json:"request_id"`
	DecisionID           string `json:"decision_id"`
}

func (c Correlation) ValidateForDecision() error {
	for _, value := range []string{c.ProcessIncarnationID, c.ConnectionID, c.SessionID, c.InteractionID, c.ActivityID, c.RequestID, c.DecisionID} {
		if strings.TrimSpace(value) == "" {
			return errors.New("complete correlation is required for a decision")
		}
	}
	return nil
}

type ApprovalRequest struct {
	Correlation      Correlation `json:"correlation"`
	Reason           string      `json:"reason"`
	AllowedDecisions []string    `json:"allowed_decisions"`
}

const (
	ApprovalReasonCommandExecution = "command_execution"
	ApprovalReasonWorkspaceChange  = "workspace_change"
	ApprovalReasonPermission       = "declared_permission"
)

func (r ApprovalRequest) Validate() error {
	if err := r.Correlation.ValidateForDecision(); err != nil {
		return err
	}
	want := []string{DecisionApprove, DecisionDeny}
	if r.Reason == ApprovalReasonPermission {
		want = []string{DecisionDeny}
	} else if r.Reason != ApprovalReasonCommandExecution && r.Reason != ApprovalReasonWorkspaceChange {
		return errors.New("known approval reason is required")
	}
	if len(r.AllowedDecisions) != len(want) {
		return errors.New("exact allowed approval decisions are required")
	}
	for index := range want {
		if r.AllowedDecisions[index] != want[index] {
			return errors.New("exact allowed approval decisions are required")
		}
	}
	return nil
}

type UserInputRequest struct {
	Correlation Correlation `json:"correlation"`
	PromptRef   string      `json:"prompt_ref"`
}

func (r UserInputRequest) Validate() error {
	if err := r.Correlation.ValidateForDecision(); err != nil {
		return err
	}
	return validateOpaqueReference(r.PromptRef, "user-input prompt reference")
}

type UserInputPromptKind string

const (
	UserInputPromptSelectOne  UserInputPromptKind = "select_one"
	UserInputPromptSelectMany UserInputPromptKind = "select_many"
	UserInputPromptText       UserInputPromptKind = "text"
)

type UserInputOption struct {
	OptionRef string `json:"option_ref"`
	Label     string `json:"label"`
}

// UserInputPrompt is a bounded, renderable DockPipe record. Summary and option
// labels must be normalized by the adapter; raw provider question/option objects
// and provider request identifiers must never populate this type.
type UserInputPrompt struct {
	Correlation   Correlation         `json:"correlation"`
	PromptRef     string              `json:"prompt_ref"`
	Kind          UserInputPromptKind `json:"kind"`
	Summary       string              `json:"summary"`
	Options       []UserInputOption   `json:"options,omitempty"`
	MaxSelections int                 `json:"max_selections,omitempty"`
	MaxTextBytes  int                 `json:"max_text_bytes,omitempty"`
}

func (p UserInputPrompt) ValidateFor(request UserInputRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if p.Correlation != request.Correlation || p.PromptRef != request.PromptRef {
		return errors.New("user-input prompt must match the exact current correlation")
	}
	if err := validateDisplayText(p.Summary, "user-input summary", maxPromptSummaryBytes); err != nil {
		return err
	}
	if len(p.Options) > maxPromptOptions {
		return errors.New("user-input options exceed their bound")
	}
	seen := make(map[string]struct{}, len(p.Options))
	for _, option := range p.Options {
		if err := validateOpaqueReference(option.OptionRef, "user-input option reference"); err != nil {
			return err
		}
		if err := validateDisplayText(option.Label, "user-input option label", maxPromptOptionLabelBytes); err != nil {
			return err
		}
		if _, exists := seen[option.OptionRef]; exists {
			return errors.New("user-input option references must be unique")
		}
		seen[option.OptionRef] = struct{}{}
	}
	switch p.Kind {
	case UserInputPromptSelectOne:
		if len(p.Options) == 0 || p.MaxSelections != 1 || p.MaxTextBytes != 0 {
			return errors.New("single-choice prompt requires options and one selection")
		}
	case UserInputPromptSelectMany:
		if len(p.Options) == 0 || p.MaxSelections < 1 || p.MaxSelections > len(p.Options) || p.MaxTextBytes != 0 {
			return errors.New("multiple-choice prompt requires bounded options and selections")
		}
	case UserInputPromptText:
		if len(p.Options) != 0 || p.MaxSelections != 0 || p.MaxTextBytes < 1 || p.MaxTextBytes > maxUserInputTextBytes {
			return errors.New("text prompt requires one bounded text answer")
		}
	default:
		return errors.New("known user-input prompt kind is required")
	}
	return nil
}

// UserInputResponse is transient operation input. Adapters must consume it at
// most once for the exact correlation and must not retain answer text in events,
// snapshots, diagnostics, or audit records.
type UserInputResponse struct {
	Correlation        Correlation `json:"correlation"`
	PromptRef          string      `json:"prompt_ref"`
	SelectedOptionRefs []string    `json:"selected_option_refs,omitempty"`
	Text               string      `json:"text,omitempty"`
}

func (r UserInputResponse) ValidateFor(prompt UserInputPrompt) error {
	request := UserInputRequest{Correlation: prompt.Correlation, PromptRef: prompt.PromptRef}
	if err := prompt.ValidateFor(request); err != nil {
		return err
	}
	if r.Correlation != prompt.Correlation || r.PromptRef != prompt.PromptRef {
		return errors.New("user-input response must match the exact current correlation")
	}
	switch prompt.Kind {
	case UserInputPromptText:
		if len(r.SelectedOptionRefs) != 0 || strings.TrimSpace(r.Text) == "" || len(r.Text) > prompt.MaxTextBytes || !utf8.ValidString(r.Text) || strings.ContainsRune(r.Text, '\x00') {
			return errors.New("bounded text response is required")
		}
		return nil
	case UserInputPromptSelectOne, UserInputPromptSelectMany:
		if r.Text != "" {
			return errors.New("choice response must not contain text")
		}
		if len(r.SelectedOptionRefs) == 0 || len(r.SelectedOptionRefs) > prompt.MaxSelections {
			return errors.New("bounded option selections are required")
		}
		allowed := make(map[string]struct{}, len(prompt.Options))
		for _, option := range prompt.Options {
			allowed[option.OptionRef] = struct{}{}
		}
		seen := make(map[string]struct{}, len(r.SelectedOptionRefs))
		for _, optionRef := range r.SelectedOptionRefs {
			if _, exists := allowed[optionRef]; !exists {
				return errors.New("selected option is not available")
			}
			if _, exists := seen[optionRef]; exists {
				return errors.New("selected options must be unique")
			}
			seen[optionRef] = struct{}{}
		}
		return nil
	default:
		return errors.New("known user-input prompt kind is required")
	}
}

type CancellationIntent struct {
	Session     SessionRef  `json:"session"`
	Correlation Correlation `json:"correlation"`
	Reason      string      `json:"reason"`
}

const (
	CancellationReasonUserRequested = "user_requested"
	CancellationReasonSafetyStop    = "safety_stop"
	CancellationReasonDeadline      = "deadline_exceeded"
)

func (i CancellationIntent) Validate() error {
	if err := i.Session.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.Correlation.SessionID) == "" || strings.TrimSpace(i.Correlation.InteractionID) == "" {
		return errors.New("session and interaction are required for cancellation")
	}
	switch i.Reason {
	case CancellationReasonUserRequested, CancellationReasonSafetyStop, CancellationReasonDeadline:
		return nil
	default:
		return errors.New("known cancellation reason is required")
	}
}

type EventKind string

const (
	EventStateChanged          EventKind = "state_changed"
	EventProgress              EventKind = "progress"
	EventApprovalRequested     EventKind = "approval_requested"
	EventUserInputRequested    EventKind = "user_input_requested"
	EventCancellationRequested EventKind = "cancellation_requested"
	EventRecoveryRequired      EventKind = "recovery_required"
)

type Event struct {
	ContractVersion string              `json:"contract_version"`
	Sequence        uint64              `json:"sequence"`
	OccurredAt      time.Time           `json:"occurred_at"`
	Session         SessionRef          `json:"session"`
	Kind            EventKind           `json:"kind"`
	State           State               `json:"state,omitempty"`
	Correlation     Correlation         `json:"correlation,omitempty"`
	Summary         string              `json:"summary,omitempty"`
	Approval        *ApprovalRequest    `json:"approval,omitempty"`
	UserInput       *UserInputRequest   `json:"user_input,omitempty"`
	Cancellation    *CancellationIntent `json:"cancellation,omitempty"`
}

func (e Event) Validate() error {
	if e.ContractVersion != ContractVersion || e.Sequence == 0 || e.OccurredAt.IsZero() {
		return errors.New("event contract version, sequence, and timestamp are required")
	}
	if err := e.Session.Validate(); err != nil {
		return err
	}
	switch e.Kind {
	case EventStateChanged:
		if !e.State.IsKnown() {
			return errors.New("known state is required")
		}
	case EventApprovalRequested:
		if e.Approval == nil {
			return errors.New("approval request is required")
		}
		return e.Approval.Validate()
	case EventUserInputRequested:
		if e.UserInput == nil {
			return errors.New("user-input request is required")
		}
		return e.UserInput.Validate()
	case EventCancellationRequested:
		if e.Cancellation == nil {
			return errors.New("cancellation intent is required")
		}
		return e.Cancellation.Validate()
	case EventProgress, EventRecoveryRequired:
		return nil
	default:
		return errors.New("known event kind is required")
	}
	return nil
}

type StartRequest struct {
	WorkspaceRef string `json:"workspace_ref"`
	PolicyRef    string `json:"policy_ref"`
	InputRef     string `json:"input_ref"`
}

type InteractionRequest struct {
	Session     SessionRef  `json:"session"`
	InputRef    string      `json:"input_ref"`
	Correlation Correlation `json:"correlation"`
}

type ApprovalDecision struct {
	Correlation Correlation `json:"correlation"`
	Decision    string      `json:"decision"`
}

const (
	DecisionApprove = "approve"
	DecisionDeny    = "deny"
)

// Validate keeps decision values provider-neutral and deliberately bounded.
// Adapters map these one-turn human choices to their private protocol values;
// session grants and policy changes need separate contract surfaces. User-input
// answers use the separately correlated transient response operation.
func (d ApprovalDecision) Validate() error {
	if err := d.Correlation.ValidateForDecision(); err != nil {
		return err
	}
	if d.Decision != DecisionApprove && d.Decision != DecisionDeny {
		return errors.New("known approval decision is required")
	}
	return nil
}

// ValidateFor binds one explicit local decision to the exact request and its
// closed decision set. Policy, capability, connection, or consumer state is
// never evidence for a decision.
func (d ApprovalDecision) ValidateFor(request ApprovalRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if d.Correlation != request.Correlation {
		return errors.New("approval decision correlation does not match the request")
	}
	for _, allowed := range request.AllowedDecisions {
		if d.Decision == allowed {
			return nil
		}
	}
	return errors.New("approval decision is not allowed for the request")
}

type RecoveryRequest struct {
	Session          SessionRef `json:"session"`
	RecoveryEvidence string     `json:"recovery_evidence"`
}

// Validate keeps recovery evidence opaque, bounded, and bound to one neutral
// session. Adapters own the evidence format and must reject any mismatch.
func (r RecoveryRequest) Validate() error {
	if err := r.Session.Validate(); err != nil {
		return err
	}
	if len(r.RecoveryEvidence) == 0 || len(r.RecoveryEvidence) > 128 {
		return errors.New("bounded recovery evidence is required")
	}
	for _, character := range r.RecoveryEvidence {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.') {
			return errors.New("recovery evidence must be an opaque safe reference")
		}
	}
	return nil
}

// Adapter is a contract only. Implementations own process supervision, transport,
// lifecycle execution, one-time decision enforcement, and persistence. Catalog,
// policy, and prompt operations return only validated provider-neutral records.
type Adapter interface {
	Catalog(context.Context) (ModelReasoningCatalog, error)
	Start(context.Context, StartRequest) (SessionRef, error)
	EffectivePolicy(context.Context, SessionRef) (EffectivePolicySnapshot, error)
	Send(context.Context, InteractionRequest) (Correlation, error)
	Decide(context.Context, ApprovalDecision) error
	UserInputPrompt(context.Context, UserInputRequest) (UserInputPrompt, error)
	RespondUserInput(context.Context, UserInputResponse) error
	Cancel(context.Context, CancellationIntent) error
	Recover(context.Context, RecoveryRequest) (SessionRef, error)
}

package appserversupervisor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"dorkpipe.orchestrator/providersession"
)

const (
	humanReviewPolicyRef              = "human-review"
	nativeAutoReviewPolicyRef         = "native-auto-review"
	workspaceWritePolicyRef           = "workspace-write"
	providerApprovalPolicyUntrusted   = "untrusted"
	providerApprovalsReviewerUser     = "user"
	providerApprovalsReviewerAuto     = "auto_review"
	providerSandboxWorkspaceWrite     = "workspace-write"
	providerSandboxTypeWorkspaceWrite = "workspaceWrite"
	maxNativePolicyOptions            = 16
	maxCapabilityOptions              = 64
)

var (
	ErrModelPolicyUnavailable = errors.New("app server model policy is unavailable")
	ErrModelPolicyRejected    = errors.New("app server model policy was rejected")
)

// NativeApprovalPolicyOption is a fixture-backed, provider-private
// advertisement reduced to one opaque stable approval/reviewer reference.
// An exact provider mapping, when present, remains private to this package.
// AuthorityExpanding is descriptive policy evidence, never authorization.
type NativeApprovalPolicyOption struct {
	PolicyRef          string
	Stable             bool
	Available          bool
	AuthorityExpanding bool
	providerPolicy     string
	providerReviewer   string
}

// NativeSandboxPolicyOption is a fixture-backed, provider-private
// advertisement reduced to one opaque stable sandbox reference. An exact
// provider mapping, when present, remains private to this package. A projected
// sandbox may never carry thread shell-command or policy-bypass authority.
type NativeSandboxPolicyOption struct {
	PolicyRef                string
	Stable                   bool
	Available                bool
	AuthorityExpanding       bool
	ThreadShellCommand       bool
	BypassesPolicyValidation bool
	providerSandbox          string
	providerSandboxType      string
}

// NativePolicyAdvertisement keeps approval/reviewer and sandbox choices in
// separate dimensions. Opaque references are projection evidence only; the
// lifecycle resolver may use only a separately retained exact private mapping.
type NativePolicyAdvertisement struct {
	Approval []NativeApprovalPolicyOption
	Sandbox  []NativeSandboxPolicyOption
}

// NativePolicyCatalog is the bounded safe projection pinned to one supervisor.
// CatalogRef changes when either independently advertised dimension changes.
type NativePolicyCatalog struct {
	CatalogRef string
	Approval   []NativeApprovalPolicyOption
	Sandbox    []NativeSandboxPolicyOption
}

// NativePolicySelection names one exact option in each policy dimension.
// Confirmation is explicit and separate so one choice cannot confirm another.
type NativePolicySelection struct {
	CatalogRef               string
	ApprovalRef              string
	ApprovalSessionConfirmed bool
	SandboxRef               string
	SandboxSessionConfirmed  bool
}

// CapabilityOption is fixture-backed policy evidence for one opaque stable
// capability. Available and Supported are independent: availability never
// implies that DockPipe policy supports or enables the capability.
type CapabilityOption struct {
	CapabilityRef      string
	Stable             bool
	Available          bool
	Supported          bool
	AuthorityExpanding bool
	Experimental       bool
}

// CapabilityAdvertisement is a complete fixture-backed capability view. It is
// projection evidence only and is never dispatched to a lifecycle request.
type CapabilityAdvertisement struct {
	Capabilities []CapabilityOption
}

// CapabilityCatalog is the bounded safe projection pinned to one supervisor.
type CapabilityCatalog struct {
	CatalogRef   string
	Capabilities []CapabilityOption
}

// CapabilityChoice enables one exact advertised capability reference. Each
// choice carries only its own per-session confirmation evidence.
type CapabilityChoice struct {
	CapabilityRef    string
	SessionConfirmed bool
}

// CapabilitySelection names the exact pinned catalog and the individually
// enabled subset. An empty Enabled list is the baseline.
type CapabilitySelection struct {
	CatalogRef string
	Enabled    []CapabilityChoice
}

// Catalog projects the initialized child's currently advertised stable model
// and reasoning combinations. The safe projection is pinned to this supervisor
// so a later selection cannot be checked against a substituted catalog.
func (s *Supervisor) Catalog(ctx context.Context) (providersession.ModelReasoningCatalog, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.mu.RLock()
	if s.modelCatalog != nil {
		catalog := cloneModelReasoningCatalog(*s.modelCatalog)
		s.mu.RUnlock()
		return catalog, nil
	}
	client := s.client
	ready := s.started && s.initialized && s.state == providersession.StateReady && client != nil && s.lifecycle.threadID == "" && !s.lifecycle.startPending
	s.mu.RUnlock()
	if !ready {
		return providersession.ModelReasoningCatalog{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}

	result, err := s.lifecycleRequest(ctx, client, "model/list", map[string]any{})
	if err != nil {
		s.fail(client.failureReason())
		return providersession.ModelReasoningCatalog{}, ErrModelPolicyUnavailable
	}
	catalog, reason := projectModelReasoningCatalog(result)
	if reason != "" {
		return providersession.ModelReasoningCatalog{}, s.rejectModelPolicy(reason)
	}

	s.mu.Lock()
	if s.state == providersession.StateDisconnected || s.lifecycle.threadID != "" || s.lifecycle.startPending {
		s.mu.Unlock()
		return providersession.ModelReasoningCatalog{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	stored := cloneModelReasoningCatalog(catalog)
	s.modelCatalog = &stored
	s.mu.Unlock()
	return cloneModelReasoningCatalog(catalog), nil
}

// SelectModelReasoning validates and pins one exact combination from Catalog.
// The returned snapshot keeps the existing human-review/workspace-write policy;
// native policy and capability selection require separate seams.
func (s *Supervisor) SelectModelReasoning(selection providersession.ModelReasoningSelection) (providersession.EffectivePolicySnapshot, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.mu.RLock()
	ready := s.started && s.initialized && s.state == providersession.StateReady && s.client != nil && s.lifecycle.threadID == "" && !s.lifecycle.startPending
	if !ready || s.modelCatalog == nil {
		s.mu.RUnlock()
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	catalog := cloneModelReasoningCatalog(*s.modelCatalog)
	if s.effectivePolicy != nil {
		policy := cloneEffectivePolicy(*s.effectivePolicy)
		s.mu.RUnlock()
		if policy.Selection != selection {
			return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
		}
		return policy, nil
	}
	s.mu.RUnlock()

	if err := selection.Validate(catalog); err != nil {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}
	policy := providersession.EffectivePolicySnapshot{
		Selection:             selection,
		EffectiveModelRef:     selection.ModelRef,
		EffectiveReasoningRef: selection.ReasoningRef,
		Approval: providersession.PolicySelection{
			SelectedRef:  humanReviewPolicyRef,
			EffectiveRef: humanReviewPolicyRef,
		},
		Sandbox: providersession.PolicySelection{
			SelectedRef:  workspaceWritePolicyRef,
			EffectiveRef: workspaceWritePolicyRef,
		},
	}
	if err := policy.Validate(catalog); err != nil {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}

	s.mu.Lock()
	if s.state == providersession.StateDisconnected || s.lifecycle.threadID != "" || s.lifecycle.startPending {
		s.mu.Unlock()
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	stored := cloneEffectivePolicy(policy)
	s.effectivePolicy = &stored
	s.mu.Unlock()
	return cloneEffectivePolicy(policy), nil
}

// ProjectNativePolicies validates and pins one complete fixture-backed stable
// policy advertisement against the existing model snapshot. It performs no
// provider request and grants no lifecycle authority.
func (s *Supervisor) ProjectNativePolicies(advertisement NativePolicyAdvertisement) (NativePolicyCatalog, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	catalog, reason := projectNativePolicyCatalog(advertisement)
	if reason != "" {
		return NativePolicyCatalog{}, s.rejectModelPolicy(reason)
	}

	s.mu.Lock()
	ready := s.started && s.initialized && s.state == providersession.StateReady && s.client != nil && s.lifecycle.threadID == "" && !s.lifecycle.startPending && s.modelCatalog != nil && s.effectivePolicy != nil
	if !ready {
		s.mu.Unlock()
		return NativePolicyCatalog{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	if s.nativePolicyCatalog != nil {
		pinned := cloneNativePolicyCatalog(*s.nativePolicyCatalog)
		s.mu.Unlock()
		if !sameNativePolicyCatalog(pinned, catalog) {
			return NativePolicyCatalog{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
		}
		return pinned, nil
	}
	stored := cloneNativePolicyCatalog(catalog)
	s.nativePolicyCatalog = &stored
	s.mu.Unlock()
	return cloneNativePolicyCatalog(catalog), nil
}

// SelectNativePolicies independently validates one exact approval/reviewer and
// one exact sandbox reference. Authority-expanding choices require their own
// per-session confirmations. Lifecycle use still requires every other pinned
// dimension plus an exact provider-private mapping.
func (s *Supervisor) SelectNativePolicies(selection NativePolicySelection) (providersession.EffectivePolicySnapshot, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.mu.RLock()
	ready := s.started && s.initialized && s.state == providersession.StateReady && s.client != nil && s.lifecycle.threadID == "" && !s.lifecycle.startPending && s.modelCatalog != nil && s.nativePolicyCatalog != nil && s.effectivePolicy != nil
	if !ready {
		s.mu.RUnlock()
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	modelCatalog := cloneModelReasoningCatalog(*s.modelCatalog)
	policyCatalog := cloneNativePolicyCatalog(*s.nativePolicyCatalog)
	policy := cloneEffectivePolicy(*s.effectivePolicy)
	alreadySelected := s.nativePoliciesSelected
	s.mu.RUnlock()

	if selection.CatalogRef != policyCatalog.CatalogRef {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}
	approval, approvalOK := advertisedApprovalPolicy(policyCatalog, selection.ApprovalRef)
	sandbox, sandboxOK := advertisedSandboxPolicy(policyCatalog, selection.SandboxRef)
	if !approvalOK || !sandboxOK || approval.AuthorityExpanding != selection.ApprovalSessionConfirmed || sandbox.AuthorityExpanding != selection.SandboxSessionConfirmed {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}

	selectedApproval := providersession.PolicySelection{SelectedRef: approval.PolicyRef, EffectiveRef: approval.PolicyRef, AuthorityExpanding: approval.AuthorityExpanding, SessionConfirmed: selection.ApprovalSessionConfirmed}
	selectedSandbox := providersession.PolicySelection{SelectedRef: sandbox.PolicyRef, EffectiveRef: sandbox.PolicyRef, AuthorityExpanding: sandbox.AuthorityExpanding, SessionConfirmed: selection.SandboxSessionConfirmed}
	if alreadySelected && (policy.Approval != selectedApproval || policy.Sandbox != selectedSandbox) {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}
	policy.Approval, policy.Sandbox = selectedApproval, selectedSandbox
	if err := policy.Validate(modelCatalog); err != nil {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}

	s.mu.Lock()
	if s.state == providersession.StateDisconnected || s.lifecycle.threadID != "" || s.lifecycle.startPending {
		s.mu.Unlock()
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	stored := cloneEffectivePolicy(policy)
	s.effectivePolicy = &stored
	s.nativePoliciesSelected = true
	s.mu.Unlock()
	return cloneEffectivePolicy(policy), nil
}

// ProjectCapabilities validates and pins one complete fixture-backed stable
// capability advertisement. It does not infer support from availability or
// enable any capability.
func (s *Supervisor) ProjectCapabilities(advertisement CapabilityAdvertisement) (CapabilityCatalog, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	catalog, reason := projectCapabilityCatalog(advertisement)
	if reason != "" {
		return CapabilityCatalog{}, s.rejectModelPolicy(reason)
	}

	s.mu.Lock()
	ready := s.started && s.initialized && s.state == providersession.StateReady && s.client != nil && s.lifecycle.threadID == "" && !s.lifecycle.startPending && s.modelCatalog != nil && s.nativePolicyCatalog != nil && s.effectivePolicy != nil && s.nativePoliciesSelected
	if !ready {
		s.mu.Unlock()
		return CapabilityCatalog{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	if s.capabilityCatalog != nil {
		pinned := cloneCapabilityCatalog(*s.capabilityCatalog)
		s.mu.Unlock()
		if !sameCapabilityCatalog(pinned, catalog) {
			return CapabilityCatalog{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
		}
		return pinned, nil
	}
	stored := cloneCapabilityCatalog(catalog)
	s.capabilityCatalog = &stored
	s.mu.Unlock()
	return cloneCapabilityCatalog(catalog), nil
}

// SelectCapabilities validates one exact enabled subset. Zero enabled
// capabilities is the baseline. Every enabled authority-expanding or
// experimental capability requires its own per-session confirmation. The
// resulting effective-policy snapshot may authorize only the zero-enabled
// lifecycle baseline until exact package-owned provider mappings exist.
func (s *Supervisor) SelectCapabilities(selection CapabilitySelection) (providersession.EffectivePolicySnapshot, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.mu.RLock()
	ready := s.started && s.initialized && s.state == providersession.StateReady && s.client != nil && s.lifecycle.threadID == "" && !s.lifecycle.startPending && s.modelCatalog != nil && s.capabilityCatalog != nil && s.effectivePolicy != nil && s.nativePoliciesSelected
	if !ready {
		s.mu.RUnlock()
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	modelCatalog := cloneModelReasoningCatalog(*s.modelCatalog)
	capabilityCatalog := cloneCapabilityCatalog(*s.capabilityCatalog)
	policy := cloneEffectivePolicy(*s.effectivePolicy)
	alreadySelected := s.capabilitiesSelected
	s.mu.RUnlock()

	if selection.CatalogRef != capabilityCatalog.CatalogRef || len(selection.Enabled) > len(capabilityCatalog.Capabilities) {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}
	enabled := make(map[string]CapabilityChoice, len(selection.Enabled))
	for _, choice := range selection.Enabled {
		if !validID(choice.CapabilityRef) {
			return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
		}
		if _, duplicate := enabled[choice.CapabilityRef]; duplicate {
			return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
		}
		option, advertised := advertisedCapability(capabilityCatalog, choice.CapabilityRef)
		requiresConfirmation := option.AuthorityExpanding || option.Experimental
		if !advertised || !option.Supported || choice.SessionConfirmed != requiresConfirmation {
			return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
		}
		enabled[choice.CapabilityRef] = choice
	}

	records := make([]providersession.CapabilityRecord, 0, len(capabilityCatalog.Capabilities))
	for _, option := range capabilityCatalog.Capabilities {
		choice, isEnabled := enabled[option.CapabilityRef]
		records = append(records, providersession.CapabilityRecord{
			CapabilityRef:      option.CapabilityRef,
			Supported:          option.Supported,
			UserEnabled:        isEnabled,
			AuthorityExpanding: option.AuthorityExpanding,
			Experimental:       option.Experimental,
			SessionConfirmed:   isEnabled && choice.SessionConfirmed,
		})
	}
	if alreadySelected && !sameCapabilityRecords(policy.Capabilities, records) {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}
	policy.Capabilities = records
	if err := policy.Validate(modelCatalog); err != nil {
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectPolicyMismatch)
	}

	s.mu.Lock()
	if s.state == providersession.StateDisconnected || s.lifecycle.threadID != "" || s.lifecycle.startPending {
		s.mu.Unlock()
		return providersession.EffectivePolicySnapshot{}, s.rejectModelPolicy(DisconnectLifecycleRejected)
	}
	stored := cloneEffectivePolicy(policy)
	s.effectivePolicy = &stored
	s.capabilitiesSelected = true
	s.mu.Unlock()
	return cloneEffectivePolicy(policy), nil
}

func (s *Supervisor) rejectModelPolicy(reason DisconnectReason) error {
	s.fail(reason)
	return ErrModelPolicyRejected
}

type modelCatalogResponse struct {
	Data       []json.RawMessage `json:"data"`
	NextCursor json.RawMessage   `json:"nextCursor"`
}

type modelCatalogEntry struct {
	ID                        string            `json:"id"`
	SupportedReasoningEfforts []json.RawMessage `json:"supportedReasoningEfforts"`
}

type modelCatalogEffort struct {
	ReasoningEffort string `json:"reasoningEffort"`
}

func projectModelReasoningCatalog(result json.RawMessage) (providersession.ModelReasoningCatalog, DisconnectReason) {
	if containsModelReroute(result) {
		return providersession.ModelReasoningCatalog{}, DisconnectModelRerouted
	}
	fields, ok := objectFields(result, "data", "nextCursor")
	if !ok || len(fields["data"]) == 0 {
		return providersession.ModelReasoningCatalog{}, DisconnectUnsupportedCapability
	}
	var response modelCatalogResponse
	if json.Unmarshal(result, &response) != nil || len(response.Data) == 0 || catalogHasNextPage(response.NextCursor) {
		return providersession.ModelReasoningCatalog{}, DisconnectUnsupportedCapability
	}

	options := make([]providersession.ModelReasoningOption, 0, len(response.Data))
	for _, rawModel := range response.Data {
		if !uniqueJSONObjectKeys(rawModel) {
			return providersession.ModelReasoningCatalog{}, DisconnectUnsupportedCapability
		}
		var model modelCatalogEntry
		if json.Unmarshal(rawModel, &model) != nil || len(model.SupportedReasoningEfforts) == 0 {
			return providersession.ModelReasoningCatalog{}, DisconnectUnsupportedCapability
		}
		for _, rawEffort := range model.SupportedReasoningEfforts {
			if !uniqueJSONObjectKeys(rawEffort) {
				return providersession.ModelReasoningCatalog{}, DisconnectUnsupportedCapability
			}
			var effort modelCatalogEffort
			if json.Unmarshal(rawEffort, &effort) != nil {
				return providersession.ModelReasoningCatalog{}, DisconnectUnsupportedCapability
			}
			options = append(options, providersession.ModelReasoningOption{ModelRef: model.ID, ReasoningRef: effort.ReasoningEffort})
		}
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].ModelRef == options[j].ModelRef {
			return options[i].ReasoningRef < options[j].ReasoningRef
		}
		return options[i].ModelRef < options[j].ModelRef
	})
	catalog := providersession.ModelReasoningCatalog{CatalogRef: modelCatalogReference(options), Options: options}
	if err := catalog.Validate(); err != nil {
		return providersession.ModelReasoningCatalog{}, DisconnectUnsupportedCapability
	}
	return catalog, ""
}

func catalogHasNextPage(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var cursor string
	return json.Unmarshal(raw, &cursor) != nil || cursor != ""
}

func modelCatalogReference(options []providersession.ModelReasoningOption) string {
	hash := sha256.New()
	for _, option := range options {
		_, _ = hash.Write([]byte(option.ModelRef))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(option.ReasoningRef))
		_, _ = hash.Write([]byte{0})
	}
	return fmt.Sprintf("catalog-%x", hash.Sum(nil))
}

func projectNativePolicyCatalog(advertisement NativePolicyAdvertisement) (NativePolicyCatalog, DisconnectReason) {
	if len(advertisement.Approval) == 0 || len(advertisement.Approval) > maxNativePolicyOptions || len(advertisement.Sandbox) == 0 || len(advertisement.Sandbox) > maxNativePolicyOptions {
		return NativePolicyCatalog{}, DisconnectUnsupportedCapability
	}
	approval := append([]NativeApprovalPolicyOption(nil), advertisement.Approval...)
	sandbox := append([]NativeSandboxPolicyOption(nil), advertisement.Sandbox...)
	sort.Slice(approval, func(i, j int) bool { return approval[i].PolicyRef < approval[j].PolicyRef })
	sort.Slice(sandbox, func(i, j int) bool { return sandbox[i].PolicyRef < sandbox[j].PolicyRef })

	baselineApproval, mappedNonBaselineApproval, baselineSandbox := false, false, false
	seen := make(map[string]struct{}, len(approval)+len(sandbox))
	for _, option := range approval {
		if !validID(option.PolicyRef) || !option.Stable || !option.Available || !validOptionalApprovalMapping(option) {
			return NativePolicyCatalog{}, DisconnectUnsupportedCapability
		}
		key := "approval\x00" + option.PolicyRef
		if _, exists := seen[key]; exists {
			return NativePolicyCatalog{}, DisconnectUnsupportedCapability
		}
		seen[key] = struct{}{}
		if option.providerPolicy != "" {
			mappingKey := "approval-mapping\x00" + option.providerPolicy + "\x00" + option.providerReviewer
			if _, exists := seen[mappingKey]; exists {
				return NativePolicyCatalog{}, DisconnectUnsupportedCapability
			}
			seen[mappingKey] = struct{}{}
		}
		if option.PolicyRef == humanReviewPolicyRef {
			if option.AuthorityExpanding {
				return NativePolicyCatalog{}, DisconnectPolicyMismatch
			}
			baselineApproval = true
		}
		if option.PolicyRef == nativeAutoReviewPolicyRef {
			if !option.AuthorityExpanding {
				return NativePolicyCatalog{}, DisconnectPolicyMismatch
			}
			mappedNonBaselineApproval = true
		}
	}
	for _, option := range sandbox {
		if !validID(option.PolicyRef) || !option.Stable || !option.Available || option.ThreadShellCommand || option.BypassesPolicyValidation || !validOptionalSandboxMapping(option) {
			return NativePolicyCatalog{}, DisconnectUnsupportedCapability
		}
		key := "sandbox\x00" + option.PolicyRef
		if _, exists := seen[key]; exists {
			return NativePolicyCatalog{}, DisconnectUnsupportedCapability
		}
		seen[key] = struct{}{}
		if option.providerSandbox != "" {
			mappingKey := "sandbox-mapping\x00" + option.providerSandbox + "\x00" + option.providerSandboxType
			if _, exists := seen[mappingKey]; exists {
				return NativePolicyCatalog{}, DisconnectUnsupportedCapability
			}
			seen[mappingKey] = struct{}{}
		}
		if option.PolicyRef == workspaceWritePolicyRef {
			if option.AuthorityExpanding {
				return NativePolicyCatalog{}, DisconnectPolicyMismatch
			}
			baselineSandbox = true
		}
	}
	if !baselineApproval || !mappedNonBaselineApproval || !baselineSandbox {
		return NativePolicyCatalog{}, DisconnectPolicyMismatch
	}
	catalog := NativePolicyCatalog{Approval: approval, Sandbox: sandbox}
	catalog.CatalogRef = nativePolicyCatalogReference(catalog)
	return catalog, ""
}

func nativePolicyCatalogReference(catalog NativePolicyCatalog) string {
	hash := sha256.New()
	for _, option := range catalog.Approval {
		_, _ = fmt.Fprintf(hash, "approval\x00%s\x00%t\x00%s\x00%s\x00", option.PolicyRef, option.AuthorityExpanding, option.providerPolicy, option.providerReviewer)
	}
	for _, option := range catalog.Sandbox {
		_, _ = fmt.Fprintf(hash, "sandbox\x00%s\x00%t\x00%s\x00%s\x00", option.PolicyRef, option.AuthorityExpanding, option.providerSandbox, option.providerSandboxType)
	}
	return fmt.Sprintf("policy-catalog-%x", hash.Sum(nil))
}

func validOptionalApprovalMapping(option NativeApprovalPolicyOption) bool {
	switch option.PolicyRef {
	case humanReviewPolicyRef:
		return option.providerPolicy == providerApprovalPolicyUntrusted && option.providerReviewer == providerApprovalsReviewerUser
	case nativeAutoReviewPolicyRef:
		return option.providerPolicy == providerApprovalPolicyUntrusted && option.providerReviewer == providerApprovalsReviewerAuto
	default:
		return option.providerPolicy == "" && option.providerReviewer == ""
	}
}

func validOptionalSandboxMapping(option NativeSandboxPolicyOption) bool {
	if option.providerSandbox == "" && option.providerSandboxType == "" {
		return true
	}
	return validID(option.providerSandbox) && validID(option.providerSandboxType)
}

func projectCapabilityCatalog(advertisement CapabilityAdvertisement) (CapabilityCatalog, DisconnectReason) {
	if len(advertisement.Capabilities) == 0 || len(advertisement.Capabilities) > maxCapabilityOptions {
		return CapabilityCatalog{}, DisconnectUnsupportedCapability
	}
	capabilities := append([]CapabilityOption(nil), advertisement.Capabilities...)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i].CapabilityRef < capabilities[j].CapabilityRef })
	seen := make(map[string]struct{}, len(capabilities))
	for _, option := range capabilities {
		if !validID(option.CapabilityRef) || !option.Stable || !option.Available {
			return CapabilityCatalog{}, DisconnectUnsupportedCapability
		}
		if _, duplicate := seen[option.CapabilityRef]; duplicate {
			return CapabilityCatalog{}, DisconnectUnsupportedCapability
		}
		seen[option.CapabilityRef] = struct{}{}
	}
	catalog := CapabilityCatalog{Capabilities: capabilities}
	catalog.CatalogRef = capabilityCatalogReference(catalog)
	return catalog, ""
}

func capabilityCatalogReference(catalog CapabilityCatalog) string {
	hash := sha256.New()
	for _, option := range catalog.Capabilities {
		_, _ = fmt.Fprintf(hash, "capability\x00%s\x00%t\x00%t\x00%t\x00", option.CapabilityRef, option.Supported, option.AuthorityExpanding, option.Experimental)
	}
	return fmt.Sprintf("capability-catalog-%x", hash.Sum(nil))
}

func advertisedCapability(catalog CapabilityCatalog, ref string) (CapabilityOption, bool) {
	for _, option := range catalog.Capabilities {
		if option.CapabilityRef == ref {
			return option, true
		}
	}
	return CapabilityOption{}, false
}

func advertisedApprovalPolicy(catalog NativePolicyCatalog, ref string) (NativeApprovalPolicyOption, bool) {
	for _, option := range catalog.Approval {
		if option.PolicyRef == ref {
			return option, true
		}
	}
	return NativeApprovalPolicyOption{}, false
}

func advertisedSandboxPolicy(catalog NativePolicyCatalog, ref string) (NativeSandboxPolicyOption, bool) {
	for _, option := range catalog.Sandbox {
		if option.PolicyRef == ref {
			return option, true
		}
	}
	return NativeSandboxPolicyOption{}, false
}

func sameNativePolicyCatalog(first, second NativePolicyCatalog) bool {
	if first.CatalogRef != second.CatalogRef || len(first.Approval) != len(second.Approval) || len(first.Sandbox) != len(second.Sandbox) {
		return false
	}
	for i := range first.Approval {
		if first.Approval[i] != second.Approval[i] {
			return false
		}
	}
	for i := range first.Sandbox {
		if first.Sandbox[i] != second.Sandbox[i] {
			return false
		}
	}
	return true
}

func sameCapabilityCatalog(first, second CapabilityCatalog) bool {
	if first.CatalogRef != second.CatalogRef || len(first.Capabilities) != len(second.Capabilities) {
		return false
	}
	for i := range first.Capabilities {
		if first.Capabilities[i] != second.Capabilities[i] {
			return false
		}
	}
	return true
}

func sameCapabilityRecords(first, second []providersession.CapabilityRecord) bool {
	if len(first) != len(second) {
		return false
	}
	for i := range first {
		if first[i] != second[i] {
			return false
		}
	}
	return true
}

func cloneModelReasoningCatalog(catalog providersession.ModelReasoningCatalog) providersession.ModelReasoningCatalog {
	catalog.Options = append([]providersession.ModelReasoningOption(nil), catalog.Options...)
	return catalog
}

func cloneEffectivePolicy(policy providersession.EffectivePolicySnapshot) providersession.EffectivePolicySnapshot {
	policy.Capabilities = append([]providersession.CapabilityRecord(nil), policy.Capabilities...)
	return policy
}

func cloneNativePolicyCatalog(catalog NativePolicyCatalog) NativePolicyCatalog {
	catalog.Approval = append([]NativeApprovalPolicyOption(nil), catalog.Approval...)
	catalog.Sandbox = append([]NativeSandboxPolicyOption(nil), catalog.Sandbox...)
	return catalog
}

func cloneCapabilityCatalog(catalog CapabilityCatalog) CapabilityCatalog {
	catalog.Capabilities = append([]CapabilityOption(nil), catalog.Capabilities...)
	return catalog
}

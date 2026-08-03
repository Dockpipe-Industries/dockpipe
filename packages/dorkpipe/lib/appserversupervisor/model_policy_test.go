package appserversupervisor

import (
	"context"
	"errors"
	"testing"

	"dorkpipe.orchestrator/providersession"
)

const modelCatalogFixture = `{"data":[{"id":"gpt-5.6-terra","supportedReasoningEfforts":[{"reasoningEffort":"high"}]},{"id":"model-stable-b","supportedReasoningEfforts":[{"reasoningEffort":"medium"},{"reasoningEffort":"high"}]}],"nextCursor":null}`

func nativePolicyFixture() NativePolicyAdvertisement {
	return NativePolicyAdvertisement{
		Approval: []NativeApprovalPolicyOption{
			{PolicyRef: humanReviewPolicyRef, Stable: true, Available: true, providerPolicy: providerApprovalPolicyUntrusted, providerReviewer: providerApprovalsReviewerUser},
			{PolicyRef: nativeAutoReviewPolicyRef, Stable: true, Available: true, AuthorityExpanding: true, providerPolicy: providerApprovalPolicyUntrusted, providerReviewer: providerApprovalsReviewerAuto},
		},
		Sandbox: []NativeSandboxPolicyOption{
			{PolicyRef: workspaceWritePolicyRef, Stable: true, Available: true, providerSandbox: providerSandboxWorkspaceWrite, providerSandboxType: providerSandboxTypeWorkspaceWrite},
			{PolicyRef: "broader-native-sandbox", Stable: true, Available: true, AuthorityExpanding: true},
		},
	}
}

func discoverModelCatalog(t *testing.T, result string) (*Supervisor, providersession.ModelReasoningCatalog) {
	t.Helper()
	s, child, scanner, _ := initializedUnselectedLifecycle(t)
	done := make(chan struct {
		catalog providersession.ModelReasoningCatalog
		err     error
	}, 1)
	go func() {
		catalog, err := s.Catalog(context.Background())
		done <- struct {
			catalog providersession.ModelReasoningCatalog
			err     error
		}{catalog, err}
	}()
	_ = lifecycleRequest(t, scanner, "model/list", 2)
	_, _ = child.stdoutW.Write([]byte(response(2, result)))
	discovered := <-done
	if discovered.err != nil {
		t.Fatal(discovered.err)
	}
	return s, discovered.catalog
}

func hasModelReasoning(catalog providersession.ModelReasoningCatalog, model, reasoning string) bool {
	for _, option := range catalog.Options {
		if option.ModelRef == model && option.ReasoningRef == reasoning {
			return true
		}
	}
	return false
}

func supervisorWithSelectedModel(t *testing.T) (*Supervisor, providersession.ModelReasoningCatalog) {
	t.Helper()
	s, catalog := discoverModelCatalog(t, modelCatalogFixture)
	selection := providersession.ModelReasoningSelection{CatalogRef: catalog.CatalogRef, ModelRef: PinnedModel, ReasoningRef: PinnedReasoningEffort}
	if _, err := s.SelectModelReasoning(selection); err != nil {
		t.Fatal(err)
	}
	return s, catalog
}

func supervisorWithNativePolicies(t *testing.T) (*Supervisor, providersession.ModelReasoningCatalog) {
	t.Helper()
	s, modelCatalog := supervisorWithSelectedModel(t)
	catalog, err := s.ProjectNativePolicies(nativePolicyFixture())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SelectNativePolicies(NativePolicySelection{CatalogRef: catalog.CatalogRef, ApprovalRef: humanReviewPolicyRef, SandboxRef: workspaceWritePolicyRef}); err != nil {
		t.Fatal(err)
	}
	return s, modelCatalog
}

func capabilityFixture() CapabilityAdvertisement {
	return CapabilityAdvertisement{Capabilities: []CapabilityOption{
		{CapabilityRef: "stable-safe", Stable: true, Available: true, Supported: true},
		{CapabilityRef: "authority-one", Stable: true, Available: true, Supported: true, AuthorityExpanding: true},
		{CapabilityRef: "experimental-one", Stable: true, Available: true, Supported: true, Experimental: true},
		{CapabilityRef: "known-unsupported", Stable: true, Available: true},
	}}
}

func capabilityRecord(policy providersession.EffectivePolicySnapshot, ref string) (providersession.CapabilityRecord, bool) {
	for _, record := range policy.Capabilities {
		if record.CapabilityRef == ref {
			return record, true
		}
	}
	return providersession.CapabilityRecord{}, false
}

func TestCAS14CatalogProjectsStableOptionsAndExactEffectivePolicy(t *testing.T) {
	s, catalog := discoverModelCatalog(t, modelCatalogFixture)
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	if !hasModelReasoning(catalog, PinnedModel, PinnedReasoningEffort) {
		t.Fatal("CAS-13 baseline is missing from the projected catalog")
	}
	if !hasModelReasoning(catalog, "model-stable-b", "medium") {
		t.Fatal("catalog projection retained only the CAS-13 baseline")
	}

	selection := providersession.ModelReasoningSelection{CatalogRef: catalog.CatalogRef, ModelRef: "model-stable-b", ReasoningRef: "medium"}
	policy, err := s.SelectModelReasoning(selection)
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.Validate(catalog); err != nil {
		t.Fatal(err)
	}
	if policy.Selection != selection || policy.EffectiveModelRef != selection.ModelRef || policy.EffectiveReasoningRef != selection.ReasoningRef {
		t.Fatalf("effective model policy was substituted: %+v", policy)
	}
	if policy.Approval.SelectedRef != humanReviewPolicyRef || policy.Approval.EffectiveRef != humanReviewPolicyRef || policy.Approval.AuthorityExpanding || policy.Approval.SessionConfirmed {
		t.Fatalf("approval policy expanded or changed: %+v", policy.Approval)
	}
	if policy.Sandbox.SelectedRef != workspaceWritePolicyRef || policy.Sandbox.EffectiveRef != workspaceWritePolicyRef || policy.Sandbox.AuthorityExpanding || policy.Sandbox.SessionConfirmed || len(policy.Capabilities) != 0 {
		t.Fatalf("sandbox or capability policy expanded: %+v", policy)
	}

	again, err := s.Catalog(context.Background())
	if err != nil || again.CatalogRef != catalog.CatalogRef {
		t.Fatalf("pinned catalog lookup = %+v, %v", again, err)
	}
	again.Options[0].ModelRef = "caller-mutation"
	pinned, err := s.Catalog(context.Background())
	if err != nil || hasModelReasoning(pinned, "caller-mutation", pinned.Options[0].ReasoningRef) {
		t.Fatal("caller mutation changed the pinned catalog")
	}
}

func TestCAS14CatalogReferenceIsStableAcrossAdvertisedOrdering(t *testing.T) {
	first, reason := projectModelReasoningCatalog([]byte(modelCatalogFixture))
	if reason != "" {
		t.Fatal(reason)
	}
	second, reason := projectModelReasoningCatalog([]byte(`{"data":[{"id":"model-stable-b","supportedReasoningEfforts":[{"reasoningEffort":"high"},{"reasoningEffort":"medium"}]},{"id":"gpt-5.6-terra","supportedReasoningEfforts":[{"reasoningEffort":"high"}]}]}`))
	if reason != "" {
		t.Fatal(reason)
	}
	if first.CatalogRef != second.CatalogRef {
		t.Fatalf("catalog references drifted with ordering: %s != %s", first.CatalogRef, second.CatalogRef)
	}
}

func TestCAS14CatalogFailsClosedOnEmptyDuplicateMalformedPagedAndReroutedResults(t *testing.T) {
	fixtures := map[string]struct {
		result string
		reason DisconnectReason
	}{
		"empty":     {`{"data":[]}`, DisconnectUnsupportedCapability},
		"duplicate": {`{"data":[{"id":"model-a","supportedReasoningEfforts":[{"reasoningEffort":"high"}]},{"id":"model-a","supportedReasoningEfforts":[{"reasoningEffort":"high"}]}]}`, DisconnectUnsupportedCapability},
		"malformed": {`{"data":[{"id":"unsafe model","supportedReasoningEfforts":[{"reasoningEffort":"high"}]}]}`, DisconnectUnsupportedCapability},
		"paged":     {`{"data":[{"id":"model-a","supportedReasoningEfforts":[{"reasoningEffort":"high"}]}],"nextCursor":"more"}`, DisconnectUnsupportedCapability},
		"rerouted":  {`{"data":[{"id":"model-a","supportedReasoningEfforts":[{"reasoningEffort":"high"}]}],"modelRerouted":true}`, DisconnectModelRerouted},
	}
	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			s, child, scanner, _ := initializedUnselectedLifecycle(t)
			done := make(chan error, 1)
			go func() { _, err := s.Catalog(context.Background()); done <- err }()
			_ = lifecycleRequest(t, scanner, "model/list", 2)
			_, _ = child.stdoutW.Write([]byte(response(2, fixture.result)))
			if err := <-done; !errors.Is(err, ErrModelPolicyRejected) {
				t.Fatalf("catalog rejection error = %v", err)
			}
			_ = nextEvent(t, s)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(fixture.reason) {
				t.Fatalf("catalog rejection = %+v", event)
			}
		})
	}
}

func TestCAS14SelectionFailsClosedOnUnavailableRemovedMismatchedAndChangedValues(t *testing.T) {
	previous, reason := projectModelReasoningCatalog([]byte(modelCatalogFixture))
	if reason != "" {
		t.Fatal(reason)
	}
	tests := map[string]struct {
		fixture   string
		selection func(providersession.ModelReasoningCatalog) providersession.ModelReasoningSelection
	}{
		"empty": {modelCatalogFixture, func(providersession.ModelReasoningCatalog) providersession.ModelReasoningSelection {
			return providersession.ModelReasoningSelection{}
		}},
		"unavailable": {modelCatalogFixture, func(c providersession.ModelReasoningCatalog) providersession.ModelReasoningSelection {
			return providersession.ModelReasoningSelection{CatalogRef: c.CatalogRef, ModelRef: "missing-model", ReasoningRef: "high"}
		}},
		"removed": {`{"data":[{"id":"gpt-5.6-terra","supportedReasoningEfforts":[{"reasoningEffort":"high"}]}]}`, func(providersession.ModelReasoningCatalog) providersession.ModelReasoningSelection {
			return providersession.ModelReasoningSelection{CatalogRef: previous.CatalogRef, ModelRef: "model-stable-b", ReasoningRef: "medium"}
		}},
		"mismatched_catalog": {modelCatalogFixture, func(c providersession.ModelReasoningCatalog) providersession.ModelReasoningSelection {
			return providersession.ModelReasoningSelection{CatalogRef: "catalog-stale", ModelRef: PinnedModel, ReasoningRef: PinnedReasoningEffort}
		}},
		"mismatched_pair": {modelCatalogFixture, func(c providersession.ModelReasoningCatalog) providersession.ModelReasoningSelection {
			return providersession.ModelReasoningSelection{CatalogRef: c.CatalogRef, ModelRef: PinnedModel, ReasoningRef: "medium"}
		}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s, catalog := discoverModelCatalog(t, test.fixture)
			if _, err := s.SelectModelReasoning(test.selection(catalog)); !errors.Is(err, ErrModelPolicyRejected) {
				t.Fatalf("selection rejection error = %v", err)
			}
			_ = nextEvent(t, s)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectPolicyMismatch) {
				t.Fatalf("selection rejection = %+v", event)
			}
		})
	}
}

func TestCAS14SelectionPinsOneExactCombination(t *testing.T) {
	s, catalog := discoverModelCatalog(t, modelCatalogFixture)
	first := providersession.ModelReasoningSelection{CatalogRef: catalog.CatalogRef, ModelRef: PinnedModel, ReasoningRef: PinnedReasoningEffort}
	if _, err := s.SelectModelReasoning(first); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SelectModelReasoning(first); err != nil {
		t.Fatalf("exact selection lookup is not idempotent: %v", err)
	}
	changed := providersession.ModelReasoningSelection{CatalogRef: catalog.CatalogRef, ModelRef: "model-stable-b", ReasoningRef: "high"}
	if _, err := s.SelectModelReasoning(changed); !errors.Is(err, ErrModelPolicyRejected) {
		t.Fatalf("selection drift error = %v", err)
	}
	_ = nextEvent(t, s)
	if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectPolicyMismatch) {
		t.Fatalf("selection drift = %+v", event)
	}
}

func TestCAS14CatalogRequiresInitializedIdleSupervisor(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if _, err := s.Catalog(context.Background()); !errors.Is(err, ErrModelPolicyRejected) {
		t.Fatalf("pre-initialization catalog error = %v", err)
	}
	if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectLifecycleRejected) {
		t.Fatalf("pre-initialization catalog rejection = %+v", event)
	}
}

func TestCAS14NativeApprovalAndSandboxPoliciesAreIndependentlySelected(t *testing.T) {
	t.Run("approval_expands_without_sandbox_expansion", func(t *testing.T) {
		s, modelCatalog := supervisorWithSelectedModel(t)
		policyCatalog, err := s.ProjectNativePolicies(nativePolicyFixture())
		if err != nil {
			t.Fatal(err)
		}
		policy, err := s.SelectNativePolicies(NativePolicySelection{
			CatalogRef:               policyCatalog.CatalogRef,
			ApprovalRef:              nativeAutoReviewPolicyRef,
			ApprovalSessionConfirmed: true,
			SandboxRef:               workspaceWritePolicyRef,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := policy.Validate(modelCatalog); err != nil {
			t.Fatal(err)
		}
		if !policy.Approval.AuthorityExpanding || !policy.Approval.SessionConfirmed || policy.Sandbox.AuthorityExpanding || policy.Sandbox.SessionConfirmed || policy.Sandbox.SelectedRef != workspaceWritePolicyRef {
			t.Fatalf("approval selection coupled sandbox authority: %+v", policy)
		}
	})

	t.Run("sandbox_expands_without_approval_expansion", func(t *testing.T) {
		s, modelCatalog := supervisorWithSelectedModel(t)
		policyCatalog, err := s.ProjectNativePolicies(nativePolicyFixture())
		if err != nil {
			t.Fatal(err)
		}
		policy, err := s.SelectNativePolicies(NativePolicySelection{
			CatalogRef:              policyCatalog.CatalogRef,
			ApprovalRef:             humanReviewPolicyRef,
			SandboxRef:              "broader-native-sandbox",
			SandboxSessionConfirmed: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := policy.Validate(modelCatalog); err != nil {
			t.Fatal(err)
		}
		if policy.Approval.AuthorityExpanding || policy.Approval.SessionConfirmed || policy.Approval.SelectedRef != humanReviewPolicyRef || !policy.Sandbox.AuthorityExpanding || !policy.Sandbox.SessionConfirmed {
			t.Fatalf("sandbox selection coupled approval authority: %+v", policy)
		}
	})
}

func TestCAS14NativePolicyCatalogIsExactStableAndPinned(t *testing.T) {
	s, _ := supervisorWithSelectedModel(t)
	fixture := nativePolicyFixture()
	catalog, err := s.ProjectNativePolicies(fixture)
	if err != nil {
		t.Fatal(err)
	}
	reordered := nativePolicyFixture()
	reordered.Approval[0], reordered.Approval[1] = reordered.Approval[1], reordered.Approval[0]
	reordered.Sandbox[0], reordered.Sandbox[1] = reordered.Sandbox[1], reordered.Sandbox[0]
	again, err := s.ProjectNativePolicies(reordered)
	if err != nil || again.CatalogRef != catalog.CatalogRef {
		t.Fatalf("order-independent policy catalog = %+v, %v", again, err)
	}
	again.Approval[0].PolicyRef = "caller-mutation"
	pinned, err := s.ProjectNativePolicies(fixture)
	if err != nil || pinned.Approval[0].PolicyRef == "caller-mutation" {
		t.Fatal("caller mutation changed the pinned policy catalog")
	}

	changed := nativePolicyFixture()
	changed.Sandbox[1].AuthorityExpanding = false
	if _, err := s.ProjectNativePolicies(changed); !errors.Is(err, ErrModelPolicyRejected) {
		t.Fatalf("changed policy advertisement error = %v", err)
	}
	_ = nextEvent(t, s)
	if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectPolicyMismatch) {
		t.Fatalf("changed policy advertisement = %+v", event)
	}
}

func TestCAS14NativePolicyAdvertisementFailsClosed(t *testing.T) {
	tests := map[string]func(*NativePolicyAdvertisement){
		"empty_approval":       func(f *NativePolicyAdvertisement) { f.Approval = nil },
		"duplicate":            func(f *NativePolicyAdvertisement) { f.Approval = append(f.Approval, f.Approval[0]) },
		"unavailable":          func(f *NativePolicyAdvertisement) { f.Sandbox[1].Available = false },
		"unsupported_unstable": func(f *NativePolicyAdvertisement) { f.Approval[1].Stable = false },
		"removed_baseline":     func(f *NativePolicyAdvertisement) { f.Sandbox = f.Sandbox[1:] },
		"baseline_changed":     func(f *NativePolicyAdvertisement) { f.Approval[0].AuthorityExpanding = true },
		"partial_approval_map": func(f *NativePolicyAdvertisement) { f.Approval[0].providerReviewer = "" },
		"missing_nonbaseline_map": func(f *NativePolicyAdvertisement) {
			f.Approval[1].providerPolicy, f.Approval[1].providerReviewer = "", ""
		},
		"partial_nonbaseline_map": func(f *NativePolicyAdvertisement) { f.Approval[1].providerReviewer = "" },
		"unproven_nonbaseline_map": func(f *NativePolicyAdvertisement) {
			f.Approval[1].providerReviewer = "guardian_subagent"
		},
		"partial_sandbox_map": func(f *NativePolicyAdvertisement) { f.Sandbox[0].providerSandboxType = "" },
		"ambiguous_approval_map": func(f *NativePolicyAdvertisement) {
			f.Approval[1].providerPolicy, f.Approval[1].providerReviewer = providerApprovalPolicyUntrusted, providerApprovalsReviewerUser
		},
		"ambiguous_sandbox_map": func(f *NativePolicyAdvertisement) {
			f.Sandbox[1].providerSandbox, f.Sandbox[1].providerSandboxType = "workspace-write", "workspaceWrite"
		},
		"thread_shell":  func(f *NativePolicyAdvertisement) { f.Sandbox[1].ThreadShellCommand = true },
		"policy_bypass": func(f *NativePolicyAdvertisement) { f.Sandbox[1].BypassesPolicyValidation = true },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			s, _ := supervisorWithSelectedModel(t)
			fixture := nativePolicyFixture()
			mutate(&fixture)
			if _, err := s.ProjectNativePolicies(fixture); !errors.Is(err, ErrModelPolicyRejected) {
				t.Fatalf("policy advertisement error = %v", err)
			}
			_ = nextEvent(t, s)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected {
				t.Fatalf("policy advertisement rejection = %+v", event)
			}
		})
	}
}

func TestCAS14NativePolicySelectionFailsClosedWithoutSubstitution(t *testing.T) {
	tests := map[string]NativePolicySelection{
		"empty":                    {},
		"mismatched_catalog":       {CatalogRef: "policy-catalog-stale", ApprovalRef: humanReviewPolicyRef, SandboxRef: workspaceWritePolicyRef},
		"unavailable_approval":     {ApprovalRef: "removed-reviewer", SandboxRef: workspaceWritePolicyRef},
		"removed_sandbox":          {ApprovalRef: humanReviewPolicyRef, SandboxRef: "removed-sandbox"},
		"unconfirmed_approval":     {ApprovalRef: nativeAutoReviewPolicyRef, SandboxRef: workspaceWritePolicyRef},
		"unconfirmed_sandbox":      {ApprovalRef: humanReviewPolicyRef, SandboxRef: "broader-native-sandbox"},
		"cross_confirmed_approval": {ApprovalRef: humanReviewPolicyRef, ApprovalSessionConfirmed: true, SandboxRef: "broader-native-sandbox", SandboxSessionConfirmed: true},
	}
	for name, selection := range tests {
		t.Run(name, func(t *testing.T) {
			s, _ := supervisorWithSelectedModel(t)
			catalog, err := s.ProjectNativePolicies(nativePolicyFixture())
			if err != nil {
				t.Fatal(err)
			}
			if selection.CatalogRef == "" && name != "empty" {
				selection.CatalogRef = catalog.CatalogRef
			}
			if _, err := s.SelectNativePolicies(selection); !errors.Is(err, ErrModelPolicyRejected) {
				t.Fatalf("policy selection error = %v", err)
			}
			_ = nextEvent(t, s)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectPolicyMismatch) {
				t.Fatalf("policy selection rejection = %+v", event)
			}
		})
	}
}

func TestCAS14NativePolicySelectionPinsExactRefs(t *testing.T) {
	s, _ := supervisorWithSelectedModel(t)
	catalog, err := s.ProjectNativePolicies(nativePolicyFixture())
	if err != nil {
		t.Fatal(err)
	}
	selection := NativePolicySelection{CatalogRef: catalog.CatalogRef, ApprovalRef: nativeAutoReviewPolicyRef, ApprovalSessionConfirmed: true, SandboxRef: "broader-native-sandbox", SandboxSessionConfirmed: true}
	first, err := s.SelectNativePolicies(selection)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SelectNativePolicies(selection)
	if err != nil || first.Approval != second.Approval || first.Sandbox != second.Sandbox {
		t.Fatalf("exact native policy lookup is not idempotent: %+v, %v", second, err)
	}
	changed := selection
	changed.SandboxRef = workspaceWritePolicyRef
	changed.SandboxSessionConfirmed = false
	if _, err := s.SelectNativePolicies(changed); !errors.Is(err, ErrModelPolicyRejected) {
		t.Fatalf("silent policy substitution error = %v", err)
	}

}

func TestCAS14CapabilityCatalogKeepsZeroEnabledBaseline(t *testing.T) {
	s, modelCatalog := supervisorWithNativePolicies(t)
	catalog, err := s.ProjectCapabilities(capabilityFixture())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := s.SelectCapabilities(CapabilitySelection{CatalogRef: catalog.CatalogRef})
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.Validate(modelCatalog); err != nil {
		t.Fatal(err)
	}
	if len(policy.Capabilities) != len(catalog.Capabilities) {
		t.Fatalf("effective capability projection = %+v", policy.Capabilities)
	}
	for _, record := range policy.Capabilities {
		if record.UserEnabled || record.SessionConfirmed {
			t.Fatalf("capability was inferred enabled from catalog or policy context: %+v", record)
		}
	}
	unsupported, found := capabilityRecord(policy, "known-unsupported")
	if !found || unsupported.Supported {
		t.Fatalf("availability was inferred as support: %+v", unsupported)
	}
	if policy.Approval.SelectedRef != humanReviewPolicyRef || policy.Sandbox.SelectedRef != workspaceWritePolicyRef || policy.EffectiveModelRef != PinnedModel || policy.EffectiveReasoningRef != PinnedReasoningEffort {
		t.Fatalf("capability projection changed another policy dimension: %+v", policy)
	}
}

func TestCAS14CapabilitySelectionRequiresIndividualConfirmation(t *testing.T) {
	s, modelCatalog := supervisorWithNativePolicies(t)
	catalog, err := s.ProjectCapabilities(capabilityFixture())
	if err != nil {
		t.Fatal(err)
	}
	selection := CapabilitySelection{CatalogRef: catalog.CatalogRef, Enabled: []CapabilityChoice{
		{CapabilityRef: "stable-safe"},
		{CapabilityRef: "authority-one", SessionConfirmed: true},
		{CapabilityRef: "experimental-one", SessionConfirmed: true},
	}}
	policy, err := s.SelectCapabilities(selection)
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.Validate(modelCatalog); err != nil {
		t.Fatal(err)
	}
	for _, choice := range selection.Enabled {
		record, found := capabilityRecord(policy, choice.CapabilityRef)
		if !found || !record.UserEnabled || record.SessionConfirmed != choice.SessionConfirmed {
			t.Fatalf("capability selection was not exact: %+v", record)
		}
	}
	unsupported, _ := capabilityRecord(policy, "known-unsupported")
	if unsupported.UserEnabled || unsupported.SessionConfirmed {
		t.Fatalf("another capability enabled unsupported authority: %+v", unsupported)
	}
}

func TestCAS14CapabilityCatalogIsOrderIndependentDefensiveAndPinned(t *testing.T) {
	s, _ := supervisorWithNativePolicies(t)
	fixture := capabilityFixture()
	catalog, err := s.ProjectCapabilities(fixture)
	if err != nil {
		t.Fatal(err)
	}
	reordered := capabilityFixture()
	reordered.Capabilities[0], reordered.Capabilities[3] = reordered.Capabilities[3], reordered.Capabilities[0]
	again, err := s.ProjectCapabilities(reordered)
	if err != nil || again.CatalogRef != catalog.CatalogRef {
		t.Fatalf("order-independent capability catalog = %+v, %v", again, err)
	}
	again.Capabilities[0].CapabilityRef = "caller-mutation"
	pinned, err := s.ProjectCapabilities(fixture)
	if err != nil || pinned.Capabilities[0].CapabilityRef == "caller-mutation" {
		t.Fatal("caller mutation changed the pinned capability catalog")
	}

	changed := capabilityFixture()
	changed.Capabilities[0].Experimental = true
	if _, err := s.ProjectCapabilities(changed); !errors.Is(err, ErrModelPolicyRejected) {
		t.Fatalf("changed capability advertisement error = %v", err)
	}
	_ = nextEvent(t, s)
	if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectPolicyMismatch) {
		t.Fatalf("changed capability advertisement = %+v", event)
	}
}

func TestCAS14CapabilityAdvertisementFailsClosed(t *testing.T) {
	tests := map[string]func(*CapabilityAdvertisement){
		"empty":       func(f *CapabilityAdvertisement) { f.Capabilities = nil },
		"duplicate":   func(f *CapabilityAdvertisement) { f.Capabilities = append(f.Capabilities, f.Capabilities[0]) },
		"unavailable": func(f *CapabilityAdvertisement) { f.Capabilities[0].Available = false },
		"unstable":    func(f *CapabilityAdvertisement) { f.Capabilities[0].Stable = false },
		"malformed":   func(f *CapabilityAdvertisement) { f.Capabilities[0].CapabilityRef = "unsafe capability" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			s, _ := supervisorWithNativePolicies(t)
			fixture := capabilityFixture()
			mutate(&fixture)
			if _, err := s.ProjectCapabilities(fixture); !errors.Is(err, ErrModelPolicyRejected) {
				t.Fatalf("capability advertisement error = %v", err)
			}
			_ = nextEvent(t, s)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedCapability) {
				t.Fatalf("capability advertisement rejection = %+v", event)
			}
		})
	}
}

func TestCAS14CapabilitySelectionFailsClosedWithoutInferenceOrSubstitution(t *testing.T) {
	tests := map[string]func(CapabilityCatalog) CapabilitySelection{
		"empty_catalog": func(CapabilityCatalog) CapabilitySelection { return CapabilitySelection{} },
		"mismatched_catalog": func(CapabilityCatalog) CapabilitySelection {
			return CapabilitySelection{CatalogRef: "capability-catalog-stale"}
		},
		"duplicate": func(c CapabilityCatalog) CapabilitySelection {
			return CapabilitySelection{CatalogRef: c.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "stable-safe"}, {CapabilityRef: "stable-safe"}}}
		},
		"removed": func(c CapabilityCatalog) CapabilitySelection {
			return CapabilitySelection{CatalogRef: c.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "removed-capability"}}}
		},
		"unsupported": func(c CapabilityCatalog) CapabilitySelection {
			return CapabilitySelection{CatalogRef: c.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "known-unsupported"}}}
		},
		"unconfirmed_authority": func(c CapabilityCatalog) CapabilitySelection {
			return CapabilitySelection{CatalogRef: c.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "authority-one"}}}
		},
		"unconfirmed_experimental": func(c CapabilityCatalog) CapabilitySelection {
			return CapabilitySelection{CatalogRef: c.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "experimental-one"}}}
		},
		"another_capability_cannot_confirm": func(c CapabilityCatalog) CapabilitySelection {
			return CapabilitySelection{CatalogRef: c.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "authority-one"}, {CapabilityRef: "experimental-one", SessionConfirmed: true}}}
		},
		"safe_capability_cannot_consume_confirmation": func(c CapabilityCatalog) CapabilitySelection {
			return CapabilitySelection{CatalogRef: c.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "stable-safe", SessionConfirmed: true}}}
		},
	}
	for name, selection := range tests {
		t.Run(name, func(t *testing.T) {
			s, _ := supervisorWithNativePolicies(t)
			catalog, err := s.ProjectCapabilities(capabilityFixture())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.SelectCapabilities(selection(catalog)); !errors.Is(err, ErrModelPolicyRejected) {
				t.Fatalf("capability selection error = %v", err)
			}
			_ = nextEvent(t, s)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectPolicyMismatch) {
				t.Fatalf("capability selection rejection = %+v", event)
			}
		})
	}
}

func TestCAS14CapabilitySelectionPinsExactRefs(t *testing.T) {
	s, _ := supervisorWithNativePolicies(t)
	catalog, err := s.ProjectCapabilities(capabilityFixture())
	if err != nil {
		t.Fatal(err)
	}
	selection := CapabilitySelection{CatalogRef: catalog.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "authority-one", SessionConfirmed: true}}}
	first, err := s.SelectCapabilities(selection)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SelectCapabilities(selection)
	if err != nil || !sameCapabilityRecords(first.Capabilities, second.Capabilities) {
		t.Fatalf("exact capability selection is not idempotent: %+v, %v", second, err)
	}
	changed := CapabilitySelection{CatalogRef: catalog.CatalogRef, Enabled: []CapabilityChoice{{CapabilityRef: "experimental-one", SessionConfirmed: true}}}
	if _, err := s.SelectCapabilities(changed); !errors.Is(err, ErrModelPolicyRejected) {
		t.Fatalf("silent capability substitution error = %v", err)
	}

}

package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"dorkpipe.orchestrator/providersession"
)

func preInitializationIntent(t *testing.T, planner *preInitializationCapabilityPlanner, advertisement CapabilityAdvertisement) preInitializationCapabilityIntent {
	t.Helper()
	catalog, reason := projectCapabilityCatalog(advertisement)
	if reason != "" {
		t.Fatal(reason)
	}
	return preInitializationCapabilityIntent{
		Target:                planner.target,
		Advertisement:         advertisement,
		RequestedCapabilities: []string{requestAttestationCapabilityRef},
		Confirmation: preInitializationCapabilityConfirmation{
			ConfirmationRef: "confirmation-1",
			Target:          planner.target,
			CatalogRef:      catalog.CatalogRef,
			CapabilityRef:   requestAttestationCapabilityRef,
		},
	}
}

func freshPreInitializationPlanner(t *testing.T) (*Supervisor, *fakeChild, *preInitializationCapabilityPlanner) {
	t.Helper()
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	planner, err := newPreInitializationCapabilityPlanner(s)
	if err != nil {
		t.Fatal(err)
	}
	return s, child, planner
}

func TestCAS14PreInitializationPlanIsExactImmutableAndOneShot(t *testing.T) {
	s, _, planner := freshPreInitializationPlanner(t)
	intent := preInitializationIntent(t, planner, capabilityFixture())
	plan, err := planner.plan(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.validateFor(s); err != nil {
		t.Fatal(err)
	}
	if plan.session != planner.target.Session || plan.supervisorRef != planner.target.SupervisorRef || plan.plannerRef != planner.target.PlannerRef || plan.catalogRef != intent.Confirmation.CatalogRef || plan.capabilityRef != requestAttestationCapabilityRef || plan.providerCapability != providerCapabilityAttestation || !plan.providerEnabled || plan.confirmationRef != "confirmation-1" {
		t.Fatalf("pre-initialization plan was not exact: %+v", plan)
	}

	intent.Advertisement.Capabilities[0].CapabilityRef = "caller-mutation"
	intent.RequestedCapabilities[0] = "caller-mutation"
	intent.Confirmation.CatalogRef = "caller-mutation"
	if err := plan.validateFor(s); err != nil {
		t.Fatalf("caller mutation changed immutable plan state: %v", err)
	}

	if _, err := planner.plan(preInitializationIntent(t, planner, capabilityFixture())); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("reused confirmation error = %v", err)
	}
	replacementPlanner, err := newPreInitializationCapabilityPlanner(s)
	if err != nil {
		t.Fatal(err)
	}
	if replacementPlanner.target.SupervisorRef != planner.target.SupervisorRef || replacementPlanner.target.PlannerRef == planner.target.PlannerRef {
		t.Fatal("replacement planner did not retain supervisor identity with a fresh one-shot target")
	}
	if _, err := replacementPlanner.plan(preInitializationIntent(t, planner, capabilityFixture())); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("confirmation replay through a replacement planner error = %v", err)
	}
	mutated := plan
	mutated.catalogRef = "capability-catalog-mutated"
	if err := mutated.validateFor(s); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("mutated plan error = %v", err)
	}
}

func TestCAS14PreInitializationIntentFailsClosed(t *testing.T) {
	tests := map[string]func(*preInitializationCapabilityIntent){
		"missing_target":  func(i *preInitializationCapabilityIntent) { i.Target = preInitializationCapabilityTarget{} },
		"missing_request": func(i *preInitializationCapabilityIntent) { i.RequestedCapabilities = nil },
		"empty_advertisement": func(i *preInitializationCapabilityIntent) {
			i.Advertisement = CapabilityAdvertisement{}
		},
		"more_than_one_request": func(i *preInitializationCapabilityIntent) {
			i.RequestedCapabilities = append(i.RequestedCapabilities, "stable-safe")
		},
		"wrong_request":          func(i *preInitializationCapabilityIntent) { i.RequestedCapabilities[0] = "stable-safe" },
		"missing_confirmation":   func(i *preInitializationCapabilityIntent) { i.Confirmation = preInitializationCapabilityConfirmation{} },
		"malformed_confirmation": func(i *preInitializationCapabilityIntent) { i.Confirmation.ConfirmationRef = "unsafe confirmation" },
		"wrong_confirmation_target": func(i *preInitializationCapabilityIntent) {
			i.Confirmation.Target.SupervisorRef = "preinit-target-stale"
		},
		"wrong_confirmation_session":    func(i *preInitializationCapabilityIntent) { i.Confirmation.Target.Session.SessionID = "other-session" },
		"wrong_confirmation_catalog":    func(i *preInitializationCapabilityIntent) { i.Confirmation.CatalogRef = "capability-catalog-stale" },
		"wrong_confirmation_capability": func(i *preInitializationCapabilityIntent) { i.Confirmation.CapabilityRef = "stable-safe" },
		"catalog_removed": func(i *preInitializationCapabilityIntent) {
			i.Advertisement.Capabilities = i.Advertisement.Capabilities[:len(i.Advertisement.Capabilities)-1]
		},
		"catalog_unavailable":   func(i *preInitializationCapabilityIntent) { i.Advertisement.Capabilities[4].Available = false },
		"catalog_unsupported":   func(i *preInitializationCapabilityIntent) { i.Advertisement.Capabilities[4].Supported = false },
		"catalog_experimental":  func(i *preInitializationCapabilityIntent) { i.Advertisement.Capabilities[4].Experimental = true },
		"catalog_non_authority": func(i *preInitializationCapabilityIntent) { i.Advertisement.Capabilities[4].AuthorityExpanding = false },
		"mapping_drift": func(i *preInitializationCapabilityIntent) {
			i.Advertisement.Capabilities[4].providerCapability = "requestAttestationChanged"
		},
		"mapping_disabled": func(i *preInitializationCapabilityIntent) { i.Advertisement.Capabilities[4].providerEnabled = false },
		"second_mapping": func(i *preInitializationCapabilityIntent) {
			i.Advertisement.Capabilities = append(i.Advertisement.Capabilities, CapabilityOption{CapabilityRef: "second-mapped", Stable: true, Available: true, Supported: true, AuthorityExpanding: true, providerCapability: providerCapabilityAttestation, providerEnabled: true})
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, planner := freshPreInitializationPlanner(t)
			intent := preInitializationIntent(t, planner, capabilityFixture())
			mutate(&intent)
			if _, err := planner.plan(intent); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
				t.Fatalf("intent rejection error = %v", err)
			}
		})
	}
}

func TestCAS14PreInitializationCatalogIdentityIgnoresOrderButRejectsContentDrift(t *testing.T) {
	_, _, planner := freshPreInitializationPlanner(t)
	intent := preInitializationIntent(t, planner, capabilityFixture())
	reordered := capabilityFixture()
	reordered.Capabilities[0], reordered.Capabilities[3] = reordered.Capabilities[3], reordered.Capabilities[0]
	intent.Advertisement = reordered
	if _, err := planner.plan(intent); err != nil {
		t.Fatalf("catalog ordering changed identity: %v", err)
	}

	_, _, planner = freshPreInitializationPlanner(t)
	intent = preInitializationIntent(t, planner, capabilityFixture())
	intent.Advertisement.Capabilities[0].Supported = false
	if _, err := planner.plan(intent); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("catalog content drift error = %v", err)
	}
}

func TestCAS14PreInitializationConfirmationRejectsCrossSupervisorSessionAndRecoveryReuse(t *testing.T) {
	_, _, firstPlanner := freshPreInitializationPlanner(t)
	intent := preInitializationIntent(t, firstPlanner, capabilityFixture())

	second, _, secondPlanner := freshPreInitializationPlanner(t)
	if _, err := secondPlanner.plan(intent); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("cross-supervisor confirmation error = %v", err)
	}
	if secondPlanner.target.Session != firstPlanner.target.Session || secondPlanner.target.SupervisorRef == firstPlanner.target.SupervisorRef {
		t.Fatal("fresh supervisor target did not retain the session while changing incarnation identity")
	}

	otherChild := newFakeChild()
	other, err := New(providersession.SessionRef{Provider: "test", SessionID: "other-session"}, fakeLauncher{start: func(context.Context) (Child, error) { return otherChild, nil }}, testDeadlines(), testInitialization())
	if err != nil {
		t.Fatal(err)
	}
	otherPlanner, err := newPreInitializationCapabilityPlanner(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherPlanner.plan(intent); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("cross-session confirmation error = %v", err)
	}

	// Recovery constructs another fresh Supervisor. Its unique target rejects
	// the prior confirmation before child startup; no plan is inherited.
	recoveredPlanner, err := newPreInitializationCapabilityPlanner(second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recoveredPlanner.plan(intent); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("recovery confirmation reuse error = %v", err)
	}
}

func TestCAS14PreInitializationPlanDoesNotChangeWireOrAuthorizeAttestation(t *testing.T) {
	s, child, planner := freshPreInitializationPlanner(t)
	plan, err := planner.plan(preInitializationIntent(t, planner, capabilityFixture()))
	if err != nil || plan.validateFor(s) != nil {
		t.Fatalf("plan = %+v, err=%v", plan, err)
	}

	started := make(chan error, 1)
	go func() { started <- s.Start(context.Background()) }()
	scanner := bufio.NewScanner(child.stdinR)
	request := lifecycleRequest(t, scanner, "initialize", 1)
	params := requestParams(t, request)
	capabilities, ok := params["capabilities"].(map[string]any)
	if !ok || len(capabilities) != 1 {
		t.Fatalf("initialize capabilities changed: %#v", params["capabilities"])
	}
	if _, exists := capabilities[providerCapabilityAttestation]; exists {
		t.Fatal("requestAttestation escaped into initialize")
	}
	for _, forbidden := range []string{"experimentalApi", "mcpServerOpenaiFormElicitation", "mcp"} {
		if _, exists := capabilities[forbidden]; exists {
			t.Fatalf("forbidden initialize capability %q was emitted", forbidden)
		}
	}
	if _, exists := capabilities["optOutNotificationMethods"]; !exists {
		t.Fatal("existing notification opt-outs were removed")
	}
	_, _ = child.stdoutW.Write([]byte(response(1, `{"userAgent":"codex/0.144.1","codexHome":"C:/codex","platformFamily":"windows","platformOs":"windows"}`)))
	if !scanner.Scan() {
		t.Fatal("expected initialized notification")
	}
	var initialized map[string]json.RawMessage
	if json.Unmarshal(scanner.Bytes(), &initialized) != nil {
		t.Fatal("initialized notification is malformed")
	}
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	if err := plan.validateFor(s); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("started supervisor retained pre-initialization authority: %v", err)
	}
	if _, err := newPreInitializationCapabilityPlanner(s); !errors.Is(err, errPreInitializationCapabilityPlanRejected) {
		t.Fatalf("started supervisor accepted a new pre-initialization planner: %v", err)
	}

	_, _ = child.stdoutW.Write([]byte(`{"jsonrpc":"2.0","id":99,"method":"attestation/generate","params":{}}` + "\n"))
	if event := nextEvent(t, s); event.State != providersession.StateReady {
		t.Fatalf("ready event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedEvent) {
		t.Fatalf("attestation request was not rejected: %+v", event)
	}
}

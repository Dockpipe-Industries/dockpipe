package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

func testLifecyclePolicy(t *testing.T) LifecyclePolicy {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "workspace")
	return LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: providerSandboxWorkspaceWrite, ApprovalPolicy: providerApprovalPolicyUntrusted, Reviewer: providerApprovalsReviewerUser, Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
}

func initializedUnselectedLifecycle(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, LifecyclePolicy) {
	t.Helper()
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	return s, child, bufio.NewScanner(child.stdinR), testLifecyclePolicy(t)
}

func initializedSelectedLifecycle(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, LifecyclePolicy) {
	t.Helper()
	s, child, scanner, _ := initializedUnselectedLifecycle(t)
	policy := selectLifecyclePolicyForTest(t, s, "model-stable-b", "medium", NativePolicySelection{}, CapabilitySelection{})
	return s, child, scanner, policy
}

func initializedLifecycle(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, LifecyclePolicy) {
	t.Helper()
	return initializedSelectedLifecycle(t)
}

func initializedAutoReviewLifecycle(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, LifecyclePolicy) {
	t.Helper()
	s, child, scanner, _ := initializedUnselectedLifecycle(t)
	policy := selectLifecyclePolicyForTest(t, s, "model-stable-b", "medium", NativePolicySelection{
		ApprovalRef:              nativeAutoReviewPolicyRef,
		ApprovalSessionConfirmed: true,
		SandboxRef:               workspaceWritePolicyRef,
	}, CapabilitySelection{})
	return s, child, scanner, policy
}

func selectLifecyclePolicyForTest(t *testing.T, s *Supervisor, model, reasoning string, nativeSelection NativePolicySelection, capabilitySelection CapabilitySelection) LifecyclePolicy {
	t.Helper()
	modelCatalog, reason := projectModelReasoningCatalog([]byte(modelCatalogFixture))
	if reason != "" {
		t.Fatal(reason)
	}
	s.mu.Lock()
	storedModelCatalog := cloneModelReasoningCatalog(modelCatalog)
	s.modelCatalog = &storedModelCatalog
	s.mu.Unlock()
	if _, err := s.SelectModelReasoning(providersession.ModelReasoningSelection{CatalogRef: modelCatalog.CatalogRef, ModelRef: model, ReasoningRef: reasoning}); err != nil {
		t.Fatal(err)
	}
	nativeCatalog, err := s.ProjectNativePolicies(nativePolicyFixture())
	if err != nil {
		t.Fatal(err)
	}
	if nativeSelection.ApprovalRef == "" {
		nativeSelection.ApprovalRef = humanReviewPolicyRef
	}
	if nativeSelection.SandboxRef == "" {
		nativeSelection.SandboxRef = workspaceWritePolicyRef
	}
	nativeSelection.CatalogRef = nativeCatalog.CatalogRef
	if _, err := s.SelectNativePolicies(nativeSelection); err != nil {
		t.Fatal(err)
	}
	capabilityCatalog, err := s.ProjectCapabilities(capabilityFixture())
	if err != nil {
		t.Fatal(err)
	}
	capabilitySelection.CatalogRef = capabilityCatalog.CatalogRef
	if _, err := s.SelectCapabilities(capabilitySelection); err != nil {
		t.Fatal(err)
	}
	policy := testLifecyclePolicy(t)
	policy.Model, policy.ReasoningEffort = model, reasoning
	if nativeSelection.ApprovalRef == nativeAutoReviewPolicyRef {
		policy.Reviewer = providerApprovalsReviewerAuto
		policy.AutoReview = true
	}
	return policy
}

func protocolRequestID(client *protocolClient) uint64 {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.nextID
}

func lifecycleRequest(t *testing.T, scanner *bufio.Scanner, wantMethod string, wantID uint64) map[string]any {
	t.Helper()
	if !scanner.Scan() {
		t.Fatalf("expected %s request", wantMethod)
	}
	var request map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
		t.Fatal(err)
	}
	if request["method"] != wantMethod || request["id"] != float64(wantID) {
		t.Fatalf("request = %#v, want %s/%d", request, wantMethod, wantID)
	}
	return request
}

func requestParams(t *testing.T, request map[string]any) map[string]any {
	t.Helper()
	params, ok := request["params"].(map[string]any)
	if !ok {
		t.Fatalf("request has no parameters: %#v", request)
	}
	return params
}

func assertSelectedPolicy(t *testing.T, request map[string]any, policy LifecyclePolicy) {
	t.Helper()
	params := requestParams(t, request)
	if params["cwd"] != policy.Workspace || params["sandbox"] != providerSandboxWorkspaceWrite || params["approvalPolicy"] != policy.ApprovalPolicy || params["approvalsReviewer"] != policy.Reviewer || params["model"] != policy.Model || params["effort"] != policy.ReasoningEffort || params["modelProvider"] != "openai" {
		t.Fatalf("request does not retain the exact selected policy: %#v", request)
	}
	sandbox, ok := params["sandboxPolicy"].(map[string]any)
	if !ok || sandbox["type"] != providerSandboxTypeWorkspaceWrite || sandbox["networkAccess"] != false {
		t.Fatalf("sandbox policy = %#v", request["sandboxPolicy"])
	}
	roots, ok := sandbox["writableRoots"].([]any)
	if !ok || len(roots) != 1 || roots[0] != policy.Workspace {
		t.Fatalf("writable roots = %#v", sandbox["writableRoots"])
	}
}

func startThreadForTest(t *testing.T, s *Supervisor, child *fakeChild, scanner *bufio.Scanner, policy LifecyclePolicy) LifecycleReference {
	t.Helper()
	s.mu.RLock()
	selected := s.modelCatalog != nil && s.nativePolicyCatalog != nil && s.capabilityCatalog != nil && s.effectivePolicy != nil && s.nativePoliciesSelected && s.capabilitiesSelected
	s.mu.RUnlock()
	if !selected {
		_ = selectLifecyclePolicyForTest(t, s, policy.Model, policy.ReasoningEffort, NativePolicySelection{}, CapabilitySelection{})
	}
	done := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartThread(context.Background(), policy)
		done <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	request := lifecycleRequest(t, scanner, "thread/start", 2)
	assertSelectedPolicy(t, request, policy)
	_, _ = child.stdoutW.Write([]byte(response(2, `{"thread":{"id":"thread-1"}}`)))
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	return result.ref
}

func TestLifecycleInitializedThreadReadResumeTurnAndSteer(t *testing.T) {
	s, child, scanner, policy := initializedSelectedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	for _, operation := range []struct {
		method string
		call   func() (LifecycleReference, error)
	}{
		{"thread/read", func() (LifecycleReference, error) { return s.ReadThread(context.Background(), thread, policy) }},
		{"thread/resume", func() (LifecycleReference, error) { return s.ResumeThread(context.Background(), thread, policy) }},
	} {
		done := make(chan error, 1)
		go func() { _, err := operation.call(); done <- err }()
		request := lifecycleRequest(t, scanner, operation.method, map[string]uint64{"thread/read": 3, "thread/resume": 4}[operation.method])
		assertSelectedPolicy(t, request, policy)
		params := requestParams(t, request)
		if params["threadId"] != "thread-1" || params["includeTurns"] != false {
			t.Fatalf("thread lifecycle params = %#v", request)
		}
		_, _ = child.stdoutW.Write([]byte(response(map[string]uint64{"thread/read": 3, "thread/resume": 4}[operation.method], `{"thread":{"id":"thread-1"}}`)))
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	turnDone := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1")
		turnDone <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	turnRequest := lifecycleRequest(t, scanner, "turn/start", 5)
	assertSelectedPolicy(t, turnRequest, policy)
	if requestParams(t, turnRequest)["threadId"] != "thread-1" {
		t.Fatalf("turn request = %#v", turnRequest)
	}
	_, _ = child.stdoutW.Write([]byte(response(5, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	turn := <-turnDone
	if turn.err != nil || turn.ref.Session.SessionID != "thread-1" || turn.ref.Correlation.InteractionID != "turn-1" {
		t.Fatalf("turn projection = %+v, err=%v", turn.ref, turn.err)
	}
	steerDone := make(chan error, 1)
	go func() { steerDone <- s.SteerTurn(context.Background(), turn.ref, policy, "turn-input-2") }()
	steerRequest := lifecycleRequest(t, scanner, "turn/steer", 6)
	assertSelectedPolicy(t, steerRequest, policy)
	if requestParams(t, steerRequest)["threadId"] != "thread-1" || requestParams(t, steerRequest)["turnId"] != "turn-1" {
		t.Fatalf("steer request = %#v", steerRequest)
	}
	_, _ = child.stdoutW.Write([]byte(response(6, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	if err := <-steerDone; err != nil {
		t.Fatal(err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatalf("initial event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "running" || event.Summary != "turn_started" {
		t.Fatalf("turn event = %+v", event)
	}
}

func TestCAS14LifecycleDispatchesExactNativeAutoReviewWithIndependentWorkspaceWriteSandbox(t *testing.T) {
	s, child, scanner, policy := initializedAutoReviewLifecycle(t)
	if policy.ApprovalPolicy != providerApprovalPolicyUntrusted || policy.Reviewer != providerApprovalsReviewerAuto || !policy.AutoReview {
		t.Fatalf("native automatic-review caller policy = %+v", policy)
	}
	thread := startThreadForTest(t, s, child, scanner, policy)

	for _, operation := range []struct {
		method string
		id     uint64
		call   func() (LifecycleReference, error)
	}{
		{"thread/read", 3, func() (LifecycleReference, error) { return s.ReadThread(context.Background(), thread, policy) }},
		{"thread/resume", 4, func() (LifecycleReference, error) { return s.ResumeThread(context.Background(), thread, policy) }},
	} {
		done := make(chan error, 1)
		go func() { _, err := operation.call(); done <- err }()
		request := lifecycleRequest(t, scanner, operation.method, operation.id)
		assertSelectedPolicy(t, request, policy)
		_, _ = child.stdoutW.Write([]byte(response(operation.id, `{"thread":{"id":"thread-1"}}`)))
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}

	turnDone := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1")
		turnDone <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	turnRequest := lifecycleRequest(t, scanner, "turn/start", 5)
	assertSelectedPolicy(t, turnRequest, policy)
	_, _ = child.stdoutW.Write([]byte(response(5, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	turn := <-turnDone
	if turn.err != nil {
		t.Fatal(turn.err)
	}

	steerDone := make(chan error, 1)
	go func() { steerDone <- s.SteerTurn(context.Background(), turn.ref, policy, "turn-input-2") }()
	steerRequest := lifecycleRequest(t, scanner, "turn/steer", 6)
	assertSelectedPolicy(t, steerRequest, policy)
	_, _ = child.stdoutW.Write([]byte(response(6, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	if err := <-steerDone; err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleRejectsStaleAndDuplicateTurnReferences(t *testing.T) {
	s, child, scanner, policy := initializedSelectedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	stale := thread
	stale.Correlation.ConnectionID = "stale"
	if _, err := s.ReadThread(context.Background(), stale, policy); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("stale read error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatalf("ready event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" || event.Summary != string(DisconnectLifecycleRejected) {
		t.Fatalf("stale event = %+v", event)
	}

	s, child, scanner, policy = initializedSelectedLifecycle(t)
	thread = startThreadForTest(t, s, child, scanner, policy)
	stale = thread
	stale.Correlation.ProcessIncarnationID = "stale"
	if _, err := s.ResumeThread(context.Background(), stale, policy); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("stale resume error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatalf("ready event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" || event.Summary != string(DisconnectLifecycleRejected) {
		t.Fatalf("stale resume event = %+v", event)
	}

	s, child, scanner, policy = initializedSelectedLifecycle(t)
	thread = startThreadForTest(t, s, child, scanner, policy)
	first := make(chan error, 1)
	go func() { _, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1"); first <- err }()
	_ = lifecycleRequest(t, scanner, "turn/start", 3)
	second := make(chan error, 1)
	go func() { _, err := s.StartTurn(context.Background(), thread, policy, "turn-input-2"); second <- err }()
	_, _ = child.stdoutW.Write([]byte(response(3, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("duplicate turn error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatalf("ready event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "running" {
		t.Fatalf("running event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" {
		t.Fatalf("duplicate event = %+v", event)
	}
}

func TestLifecycleSteerRejectsNonSteerableAndBadResponse(t *testing.T) {
	s, child, scanner, policy := initializedSelectedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	done := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1")
		done <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	_ = lifecycleRequest(t, scanner, "turn/start", 3)
	_, _ = child.stdoutW.Write([]byte(response(3, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	turn := <-done
	if turn.err != nil {
		t.Fatal(turn.err)
	}
	s.lifecycle.steerable = false
	if err := s.SteerTurn(context.Background(), turn.ref, policy, "turn-input-2"); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("non-steerable error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.State != "running" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" {
		t.Fatal(event)
	}
}

func TestLifecycleProtocolAndDeadlineFailuresFailClosed(t *testing.T) {
	for name, fixture := range map[string]struct {
		frame  string
		reason DisconnectReason
	}{
		"malformed":      {"not-json\n", DisconnectMalformedEnvelope},
		"provider_error": {`{"jsonrpc":"2.0","id":2,"error":{"message":"private"}}` + "\n", DisconnectProviderError},
		"id_mismatch":    {response(3, `{"thread":{"id":"thread-1"}}`), DisconnectCorrelationMismatch},
		"invalid_shape":  {response(2, `{"thread":{"id":"bad id"}}`), DisconnectUnsupportedLifecycle},
		"reroute":        {response(2, `{"thread":{"id":"thread-1"},"modelRerouted":true}`), DisconnectModelRerouted},
	} {
		t.Run(name, func(t *testing.T) {
			s, child, scanner, policy := initializedSelectedLifecycle(t)
			done := make(chan error, 1)
			go func() { _, err := s.StartThread(context.Background(), policy); done <- err }()
			_ = lifecycleRequest(t, scanner, "thread/start", 2)
			_, _ = child.stdoutW.Write([]byte(fixture.frame))
			if err := <-done; err == nil {
				t.Fatal("expected lifecycle failure")
			}
			_ = nextEvent(t, s)
			event := nextEvent(t, s)
			if event.State != "disconnected" || event.Summary != string(fixture.reason) {
				t.Fatalf("event = %+v", event)
			}
		})
	}
	t.Run("deadline", func(t *testing.T) {
		s, _, scanner, policy := initializedSelectedLifecycle(t)
		s.deadlines.Request = 20 * time.Millisecond
		done := make(chan error, 1)
		go func() { _, err := s.StartThread(context.Background(), policy); done <- err }()
		_ = lifecycleRequest(t, scanner, "thread/start", 2)
		if err := <-done; err == nil {
			t.Fatal("expected deadline")
		}
		_ = nextEvent(t, s)
		event := nextEvent(t, s)
		if event.Summary != string(DisconnectRequestDeadline) {
			t.Fatalf("event = %+v", event)
		}
	})
}

func TestTurnLifecycleStateIsRejected(t *testing.T) {
	s, child, scanner, policy := initializedSelectedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	done := make(chan error, 1)
	go func() { _, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1"); done <- err }()
	_ = lifecycleRequest(t, scanner, "turn/start", 3)
	_, _ = child.stdoutW.Write([]byte(response(3, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"completed"}}`)))
	if err := <-done; !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("turn lifecycle-state error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.Summary != string(DisconnectUnsupportedLifecycle) {
		t.Fatal(event)
	}
}

func TestLifecyclePolicyAndTransportGate(t *testing.T) {
	policy := testLifecyclePolicy(t)
	for name, mutate := range map[string]func(*LifecyclePolicy){
		"full_access": func(p *LifecyclePolicy) { p.FullAccess = true }, "shell": func(p *LifecyclePolicy) { p.AllowShell = true },
		"auto_review_without_mapping": func(p *LifecyclePolicy) { p.AutoReview = true }, "network": func(p *LifecyclePolicy) { p.NetworkEnabled = true },
		"no_roots": func(p *LifecyclePolicy) { p.WritableRoots = nil }, "model": func(p *LifecyclePolicy) { p.Model = "unsafe model" },
		"effort": func(p *LifecyclePolicy) { p.ReasoningEffort = "" }, "fallback": func(p *LifecyclePolicy) { p.FallbackModel = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := policy
			mutate(&candidate)
			if candidate.validate() == nil {
				t.Fatal("unsafe policy was accepted")
			}
		})
	}
	s, child, scanner, policy := initializedSelectedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	changed := policy
	changed.Workspace = filepath.Join(t.TempDir(), "other-workspace")
	changed.WritableRoots = []string{changed.Workspace}
	if _, err := s.ReadThread(context.Background(), thread, changed); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("changed policy error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.Summary != string(DisconnectPolicyMismatch) {
		t.Fatal(event)
	}

	s, child, _, policy = initializedSelectedLifecycle(t)
	child.exit(errors.New("lost"))
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" {
		t.Fatal(event)
	}
	if _, err := s.StartThread(context.Background(), policy); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("post-loss lifecycle error = %v", err)
	}
}

func TestCAS14LifecycleRequiresCompletePinnedSelectionsBeforeRequest(t *testing.T) {
	s, _, _, policy := initializedUnselectedLifecycle(t)
	client := s.client
	before := protocolRequestID(client)
	if _, err := s.StartThread(context.Background(), policy); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("missing selection error = %v", err)
	}
	if after := protocolRequestID(client); after != before {
		t.Fatalf("lifecycle request was sent before selection: request id %d -> %d", before, after)
	}
	if event := nextEvent(t, s); event.State != providersession.StateReady {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectLifecycleRejected) {
		t.Fatalf("missing selection event = %+v", event)
	}
}

func TestCAS14LifecycleBlocksDimensionsWithoutExactProviderMapping(t *testing.T) {
	tests := map[string]struct {
		native       NativePolicySelection
		capabilities CapabilitySelection
	}{
		"sandbox": {
			native: NativePolicySelection{ApprovalRef: humanReviewPolicyRef, SandboxRef: "broader-native-sandbox", SandboxSessionConfirmed: true},
		},
		"enabled_capability": {
			capabilities: CapabilitySelection{Enabled: []CapabilityChoice{{CapabilityRef: "stable-safe"}}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			s, _, _, _ := initializedUnselectedLifecycle(t)
			policy := selectLifecyclePolicyForTest(t, s, "model-stable-b", "medium", test.native, test.capabilities)
			client := s.client
			before := protocolRequestID(client)
			if _, err := s.StartThread(context.Background(), policy); !errors.Is(err, ErrLifecycleRejected) {
				t.Fatalf("missing mapping error = %v", err)
			}
			if after := protocolRequestID(client); after != before {
				t.Fatalf("lifecycle request was sent without an exact mapping: request id %d -> %d", before, after)
			}
			if event := nextEvent(t, s); event.State != providersession.StateReady {
				t.Fatal(event)
			}
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedCapability) {
				t.Fatalf("missing mapping event = %+v", event)
			}
		})
	}
}

func TestCAS14LifecycleOperationsRejectCallerPolicyDriftBeforeRequest(t *testing.T) {
	operations := map[string]func(*Supervisor, LifecycleReference, LifecyclePolicy) error{
		"read": func(s *Supervisor, thread LifecycleReference, policy LifecyclePolicy) error {
			_, err := s.ReadThread(context.Background(), thread, policy)
			return err
		},
		"resume": func(s *Supervisor, thread LifecycleReference, policy LifecyclePolicy) error {
			_, err := s.ResumeThread(context.Background(), thread, policy)
			return err
		},
		"start_turn": func(s *Supervisor, thread LifecycleReference, policy LifecyclePolicy) error {
			_, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1")
			return err
		},
		"steer_turn": func(s *Supervisor, thread LifecycleReference, policy LifecyclePolicy) error {
			return s.SteerTurn(context.Background(), thread, policy, "turn-input-1")
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			s, child, scanner, policy := initializedSelectedLifecycle(t)
			thread := startThreadForTest(t, s, child, scanner, policy)
			client := s.client
			before := protocolRequestID(client)
			drifted := policy
			drifted.Model = PinnedModel
			if err := operation(s, thread, drifted); !errors.Is(err, ErrLifecycleRejected) {
				t.Fatalf("caller policy drift error = %v", err)
			}
			if after := protocolRequestID(client); after != before {
				t.Fatalf("drifted lifecycle request was sent: request id %d -> %d", before, after)
			}
			if event := nextEvent(t, s); event.State != providersession.StateReady {
				t.Fatal(event)
			}
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectPolicyMismatch) {
				t.Fatalf("caller policy drift event = %+v", event)
			}
		})
	}
}

func TestCAS14LifecycleRejectsCatalogAndEffectiveSnapshotDriftBeforeRequest(t *testing.T) {
	tests := map[string]func(*Supervisor){
		"catalog_drift": func(s *Supervisor) {
			s.modelCatalog.Options[0].ReasoningRef = "drifted"
		},
		"selected_effective_mismatch": func(s *Supervisor) {
			s.effectivePolicy.EffectiveModelRef = PinnedModel
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			s, child, scanner, policy := initializedSelectedLifecycle(t)
			thread := startThreadForTest(t, s, child, scanner, policy)
			client := s.client
			before := protocolRequestID(client)
			s.mu.Lock()
			mutate(s)
			s.mu.Unlock()
			if _, err := s.ReadThread(context.Background(), thread, policy); !errors.Is(err, ErrLifecycleRejected) {
				t.Fatalf("stored policy drift error = %v", err)
			}
			if after := protocolRequestID(client); after != before {
				t.Fatalf("stored policy drift request was sent: request id %d -> %d", before, after)
			}
			if event := nextEvent(t, s); event.State != providersession.StateReady {
				t.Fatal(event)
			}
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectPolicyMismatch) {
				t.Fatalf("stored policy drift event = %+v", event)
			}
		})
	}
}

func TestCAS14NativeAutoReviewThreadRejectsEveryPolicyDriftBeforeRequest(t *testing.T) {
	operations := map[string]func(*Supervisor, LifecycleReference, LifecyclePolicy) error{
		"read": func(s *Supervisor, thread LifecycleReference, policy LifecyclePolicy) error {
			_, err := s.ReadThread(context.Background(), thread, policy)
			return err
		},
		"resume": func(s *Supervisor, thread LifecycleReference, policy LifecyclePolicy) error {
			_, err := s.ResumeThread(context.Background(), thread, policy)
			return err
		},
		"start_turn": func(s *Supervisor, thread LifecycleReference, policy LifecyclePolicy) error {
			_, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1")
			return err
		},
		"steer_turn": func(s *Supervisor, thread LifecycleReference, policy LifecyclePolicy) error {
			return s.SteerTurn(context.Background(), thread, policy, "turn-input-1")
		},
	}
	drifts := map[string]struct {
		mutateStored func(*Supervisor)
		mutateCaller func(*LifecyclePolicy)
	}{
		"mapping_missing": {mutateStored: func(s *Supervisor) {
			s.nativePolicyCatalog.Approval[1].providerPolicy, s.nativePolicyCatalog.Approval[1].providerReviewer = "", ""
		}},
		"mapping_partial": {mutateStored: func(s *Supervisor) {
			s.nativePolicyCatalog.Approval[1].providerReviewer = ""
		}},
		"mapping_ambiguous": {mutateStored: func(s *Supervisor) {
			s.nativePolicyCatalog.Approval[1].providerReviewer = providerApprovalsReviewerUser
		}},
		"mapping_changed": {mutateStored: func(s *Supervisor) {
			s.nativePolicyCatalog.Approval[1].providerReviewer = "guardian_subagent"
		}},
		"selection_changed": {mutateStored: func(s *Supervisor) {
			s.effectivePolicy.Approval.SelectedRef = humanReviewPolicyRef
		}},
		"catalog_changed": {mutateStored: func(s *Supervisor) {
			s.nativePolicyCatalog.CatalogRef = "policy-catalog-stale"
		}},
		"effective_changed": {mutateStored: func(s *Supervisor) {
			s.effectivePolicy.Approval.EffectiveRef = humanReviewPolicyRef
		}},
		"confirmation_missing": {mutateStored: func(s *Supervisor) {
			s.effectivePolicy.Approval.SessionConfirmed = false
		}},
		"option_removed": {mutateStored: func(s *Supervisor) {
			s.nativePolicyCatalog.Approval = s.nativePolicyCatalog.Approval[:1]
		}},
		"option_unavailable": {mutateStored: func(s *Supervisor) {
			s.nativePolicyCatalog.Approval[1].Available = false
		}},
		"caller_policy_changed": {mutateCaller: func(policy *LifecyclePolicy) {
			policy.Reviewer = providerApprovalsReviewerUser
			policy.AutoReview = false
		}},
	}

	for operationName, operation := range operations {
		for driftName, drift := range drifts {
			t.Run(operationName+"/"+driftName, func(t *testing.T) {
				s, child, scanner, policy := initializedAutoReviewLifecycle(t)
				thread := startThreadForTest(t, s, child, scanner, policy)
				client := s.client
				before := protocolRequestID(client)
				if drift.mutateStored != nil {
					s.mu.Lock()
					drift.mutateStored(s)
					s.mu.Unlock()
				}
				if drift.mutateCaller != nil {
					drift.mutateCaller(&policy)
				}
				if err := operation(s, thread, policy); !errors.Is(err, ErrLifecycleRejected) {
					t.Fatalf("policy drift error = %v", err)
				}
				if after := protocolRequestID(client); after != before {
					t.Fatalf("drifted lifecycle request was sent: request id %d -> %d", before, after)
				}
				if event := nextEvent(t, s); event.State != providersession.StateReady {
					t.Fatal(event)
				}
				if event := nextEvent(t, s); event.State != providersession.StateDisconnected {
					t.Fatalf("policy drift event = %+v", event)
				}
			})
		}
	}
}

package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type nodeConnectorPlacementExecutionHandoffTestFixture struct {
	base       *nodeConnectorPlacementDispatchSubmissionTestFixture
	submission NodeConnectorPlacementDispatchSubmission
	expected   NodeConnectorPlacementExecutionHandoffExpected
	fixture    NodeConnectorPlacementExecutionHandoffDecisionFixture
}

func TestNodeConnectorPlacementExecutionHandoffApprovedEmitsOneUnconsumedRequestWithoutInvocation(t *testing.T) {
	value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
	brokerBefore := nodeExecutionStateArtifacts(t, value.base.brokerRoot)
	operationBefore := cloneNodeExecutionState(value.base.broker.state).Operations[value.submission.ExecutionRequest.OperationID]
	executorCalls := 0
	value.base.broker.executor = func(NodeExecutionRequest, NodeExecutionTaskLease) { executorCalls++ }
	connectorCalls := 0
	connector, err := NewNodeValidationConnector(value.submission.ExecutionRequest.Workflow, value.submission.ExecutionRequest.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
		connectorCalls++
		return NodeValidationEvidence{}, errors.New("must not be invoked")
	})
	if err != nil || connector == nil {
		t.Fatal(err)
	}

	handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
	decision, request := mustDecideNodeConnectorPlacementExecutionHandoff(t, handoffs, value.fixture)
	if decision.Decision != "approved" || request == nil || request.AuthorizationConsumed || !request.OneTimeHandoff || !request.InProcessConnectorSessionOnly || request.HandoffScope != nodeConnectorPlacementExecutionHandoffScope {
		t.Fatal("approved independent decision did not emit exactly one unconsumed connector-session handoff request")
	}
	wantAuthority := NodeConnectorPlacementExecutionHandoffAuthority{FixtureConnectorHandoff: true}
	if decision.Authority != (NodeConnectorPlacementExecutionHandoffAuthority{}) || request.Authority != wantAuthority || request.ConnectorInvoked || request.ExecutorInvoked || request.ExecutionStarted || request.EventsPublished || request.ReceiptPublished || request.CancellationRequested || request.NetworkInvoked || request.ProviderInvoked || request.RetryRequested || request.RepairRequested || request.ServiceInvoked || request.ValidationExecuted || request.MutationApplied || request.GitInvoked || request.ApplyInvoked || request.CheckpointInvoked || request.CommitInvoked || request.PushInvoked || request.PublicationInvoked || request.CompletionClaimed || request.LifecycleAdvanced || request.NextTaskAdvanced {
		t.Fatal("execution handoff decision or request gained authority beyond one future fixture connector handoff")
	}
	if connectorCalls != 0 || executorCalls != 0 || !nodeExecutionStringSlicesEqual(brokerBefore, nodeExecutionStateArtifacts(t, value.base.brokerRoot)) || !reflect.DeepEqual(operationBefore, value.base.broker.state.Operations[value.submission.ExecutionRequest.OperationID]) {
		t.Fatal("decision/request publication invoked the connector or executor or changed broker generation/operation evidence")
	}
	if request.SubmissionFingerprint != value.submission.SubmissionFingerprint || request.BrokerStateFingerprint != value.submission.BrokerStateFingerprint || !reflect.DeepEqual(request.TaskLease, value.submission.TaskLease) || !reflect.DeepEqual(request.ExecutionRequest, value.submission.ExecutionRequest) || !reflect.DeepEqual(request.SelectedNode, value.submission.SelectedNode) {
		t.Fatal("handoff request lost its exact submission, broker, lease, execution-request, or selected-node binding")
	}
	raw := mustReadNodeConnectorPlacementExecutionHandoffFile(t, value.base.root, nodeConnectorPlacementExecutionHandoffRequestName)
	var decoded NodeConnectorPlacementExecutionHandoffRequest
	if len(raw) > nodeConnectorPlacementExecutionHandoffMaxArtifactBytes || decodeNodeConnectorPlacementExecutionHandoffArtifact(raw, &decoded) != nil || !reflect.DeepEqual(decoded, *request) {
		t.Fatal("durable handoff request is not the exact bounded canonical returned artifact")
	}
}

func TestNodeConnectorPlacementExecutionHandoffRejectedEmitsNoRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "rejected")
	brokerBefore := nodeExecutionStateArtifacts(t, value.base.brokerRoot)
	decision, request := mustDecideNodeConnectorPlacementExecutionHandoff(t, mustOpenNodeConnectorPlacementExecutionHandoffs(t, value), value.fixture)
	if decision.Decision != "rejected" || request != nil || decision.ExecutionHandoffRequestID != "" {
		t.Fatal("rejected independent decision emitted or referenced a handoff request")
	}
	if _, err := os.Lstat(filepath.Join(value.base.root, nodeConnectorPlacementExecutionHandoffRequestName)); !os.IsNotExist(err) {
		t.Fatal("rejected decision published a request artifact")
	}
	if !nodeExecutionStringSlicesEqual(brokerBefore, nodeExecutionStateArtifacts(t, value.base.brokerRoot)) {
		t.Fatal("rejected decision changed broker evidence")
	}
}

func TestNodeConnectorPlacementExecutionHandoffRevalidatesCompleteUpstreamBrokerOperationAndLease(t *testing.T) {
	t.Run("upstream-placement-chain", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
		handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
		path := filepath.Join(value.base.root, nodeConnectorPlacementDispatchRequestName)
		raw := mustReadNodeConnectorPlacementDispatchFile(t, value.base.root, nodeConnectorPlacementDispatchRequestName)
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"authorization_consumed": false`), []byte(`"authorization_consumed": true`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		assertNodeConnectorPlacementExecutionHandoffRejected(t, value, handoffs, value.fixture)
	})

	t.Run("submission", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
		handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
		path := filepath.Join(value.base.root, nodeConnectorPlacementDispatchSubmissionName)
		raw := mustReadNodeConnectorPlacementDispatchSubmissionFile(t, value.base.root)
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"executor_invoked": false`), []byte(`"executor_invoked": true`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		assertNodeConnectorPlacementExecutionHandoffRejected(t, value, handoffs, value.fixture)
	})

	for _, test := range []struct {
		name   string
		mutate func(*nodeExecutionOperationState)
	}{
		{name: "request", mutate: func(operation *nodeExecutionOperationState) {
			operation.Request.TaskID = "task-substituted-handoff-001"
		}},
		{name: "lease", mutate: func(operation *nodeExecutionOperationState) {
			operation.Lease.LeaseID = "lease-substituted-handoff-001"
		}},
		{name: "event", mutate: func(operation *nodeExecutionOperationState) {
			operation.Events = append(operation.Events, NodeExecutionEventEnvelope{})
		}},
	} {
		t.Run("broker-"+test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
			handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
			state := cloneNodeExecutionState(value.base.broker.state)
			operation := state.Operations[value.submission.ExecutionRequest.OperationID]
			test.mutate(&operation)
			state.Operations[value.submission.ExecutionRequest.OperationID] = operation
			state.StateFingerprint = ""
			_ = finalizeNodeExecutionState(&state)
			mustWriteCanonicalNodeConnectorPlacementDispatchSubmission(t, filepath.Join(value.base.brokerRoot, nodeExecutionStateFileName(state.Generation)), state)
			value.base.broker.state = state
			assertNodeConnectorPlacementExecutionHandoffRejected(t, value, handoffs, value.fixture)
		})
	}
}

func TestNodeConnectorPlacementExecutionHandoffChangedBindingsFailClosed(t *testing.T) {
	value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
	handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)

	mutations := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionHandoffDecisionFixture)
	}{
		{"decision", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) { value.Decision = "" }},
		{"submission", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.SubmissionID = "submission-substituted-001"
		}},
		{"submission-replay", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.SubmissionReplayIdentity = "replay-substituted-001"
		}},
		{"submission-fingerprint", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.SubmissionFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"selected-node", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.SelectedNode.NodeID = "node-substituted-001"
		}},
		{"selected-machine", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.SelectedNode.MachineID = "machine-substituted-001"
		}},
		{"selected-capability", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.SelectedNode.CapabilitySnapshotID = nodeConnectorInventoryFingerprint("0")
		}},
		{"execution-request", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.ExecutionRequest.TaskID = "task-substituted-001"
		}},
		{"request-fingerprint", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.ExecutionRequestFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"broker-state", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.BrokerStateFingerprint = nodeConnectorInventoryFingerprint("0")
		}},
		{"lease", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.TaskLease.LeaseID = "lease-substituted-001"
		}},
		{"candidate-set", func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.CandidateNodeIDs = value.CandidateNodeIDs[1:]
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionHandoffDecisionFixture(value.fixture)
			test.mutate(&changed)
			assertNodeConnectorPlacementExecutionHandoffRejected(t, value, handoffs, changed)
		})
	}
}

func TestNodeConnectorPlacementExecutionHandoffEvidenceCannotImplyApproval(t *testing.T) {
	value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
	handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)

	for _, field := range []string{"connection_present", "healthy", "available", "load", "risk", "cost", "ordering", "ranking", "provider_evidence", "broker_acceptance", "lease_exists"} {
		t.Run(field, func(t *testing.T) {
			valid := mustMarshalNodeConnectorPlacementExecutionHandoff(t, value.fixture)
			raw := append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":true}`)...)
			assertNodeConnectorPlacementExecutionHandoffRawRejected(t, value, handoffs, raw)
		})
	}
	changed := cloneNodeConnectorPlacementExecutionHandoffDecisionFixture(value.fixture)
	changed.Decision = ""
	assertNodeConnectorPlacementExecutionHandoffRejected(t, value, handoffs, changed)
}

func TestNodeConnectorPlacementExecutionHandoffReplayCollisionAndRestartAreStrict(t *testing.T) {
	value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
	handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
	firstDecision, firstRequest := mustDecideNodeConnectorPlacementExecutionHandoff(t, handoffs, value.fixture)
	brokerBefore := nodeExecutionStateArtifacts(t, value.base.brokerRoot)
	secondDecision, secondRequest := mustDecideNodeConnectorPlacementExecutionHandoff(t, handoffs, value.fixture)
	if !reflect.DeepEqual(firstDecision, secondDecision) || !reflect.DeepEqual(firstRequest, secondRequest) || !nodeExecutionStringSlicesEqual(brokerBefore, nodeExecutionStateArtifacts(t, value.base.brokerRoot)) {
		t.Fatal("exact replay changed handoff artifacts or broker evidence")
	}
	restarted := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
	thirdDecision, thirdRequest := mustDecideNodeConnectorPlacementExecutionHandoff(t, restarted, value.fixture)
	if !reflect.DeepEqual(firstDecision, thirdDecision) || !reflect.DeepEqual(firstRequest, thirdRequest) {
		t.Fatal("restart did not recover the exact decision and request")
	}

	for _, mutate := range []func(*NodeConnectorPlacementExecutionHandoffDecisionFixture){
		func(changed *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			changed.Reason = "changed explicit reason"
		},
		func(changed *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			changed.ReplayIdentity = "replay-handoff-changed-001"
		},
		func(changed *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			changed.ExecutionHandoffRequestID = "handoff-request-changed-001"
		},
	} {
		changed := cloneNodeConnectorPlacementExecutionHandoffDecisionFixture(value.fixture)
		mutate(&changed)
		assertNodeConnectorPlacementExecutionHandoffRejected(t, value, restarted, changed)
	}

	for _, collision := range []string{value.fixture.DecisionID, value.submission.SubmissionID, value.submission.ReplayIdentity} {
		caseValue := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
		changed := cloneNodeConnectorPlacementExecutionHandoffDecisionFixture(caseValue.fixture)
		changed.ReplayIdentity = collision
		assertNodeConnectorPlacementExecutionHandoffRejected(t, caseValue, mustOpenNodeConnectorPlacementExecutionHandoffs(t, caseValue), changed)
	}
}

func TestNodeConnectorPlacementExecutionHandoffRejectsMalformedUnknownTrailingOversizedAndNoncanonicalJSON(t *testing.T) {
	value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
	handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
	valid := mustMarshalNodeConnectorPlacementExecutionHandoff(t, value.fixture)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, valid, "", "  "); err != nil {
		t.Fatal(err)
	}
	inputs := [][]byte{[]byte("{not-json"), pretty.Bytes(), append(append([]byte{}, valid...), []byte(" trailing")...), make([]byte, nodeConnectorPlacementExecutionHandoffMaxDecisionBytes+1)}
	for _, field := range []string{"unknown", "connector", "executor", "execution", "event", "receipt", "network", "provider", "approval_inferred"} {
		inputs = append(inputs, append(append([]byte{}, valid[:len(valid)-1]...), []byte(`,"`+field+`":true}`)...))
	}
	for index, raw := range inputs {
		assertNodeConnectorPlacementExecutionHandoffRawRejected(t, value, handoffs, raw)
		if _, err := os.Lstat(filepath.Join(value.base.root, nodeConnectorPlacementExecutionHandoffDecisionName)); !os.IsNotExist(err) {
			t.Fatalf("invalid input %d published partial evidence", index)
		}
	}
}

func TestNodeConnectorPlacementExecutionHandoffRejectsInvalidOrUnboundedIssuancePolicy(t *testing.T) {
	value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
	handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)

	mutations := []func(*NodeConnectorPlacementExecutionHandoffDecisionFixture){
		func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) { value.Reason = "" },
		func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) { value.Reason = " leading" },
		func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.Reason = strings.Repeat("x", nodeConnectorPlacementExecutionHandoffMaxReasonBytes+1)
		},
		func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) { value.IssuedAt = "not-a-time" },
		func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.IssuedAt = "2026-07-28T21:00:59Z"
		},
		func(value *NodeConnectorPlacementExecutionHandoffDecisionFixture) {
			value.IssuedAt = value.TaskLease.ExpiresAt
		},
	}
	for index, mutate := range mutations {
		changed := cloneNodeConnectorPlacementExecutionHandoffDecisionFixture(value.fixture)
		mutate(&changed)
		assertNodeConnectorPlacementExecutionHandoffRejected(t, value, handoffs, changed)
		if _, err := os.Lstat(filepath.Join(value.base.root, nodeConnectorPlacementExecutionHandoffDecisionName)); !os.IsNotExist(err) {
			t.Fatalf("invalid issuance case %d published a decision", index)
		}
	}
}

func TestNodeConnectorPlacementExecutionHandoffAtomicFailuresRecoverWithoutPartialOutput(t *testing.T) {
	t.Run("decision", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
		handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
		original := nodeConnectorPlacementExecutionHandoffWriteDecisionAtomic
		nodeConnectorPlacementExecutionHandoffWriteDecisionAtomic = func(string, any) error { return errors.New("injected decision write failure") }
		t.Cleanup(func() { nodeConnectorPlacementExecutionHandoffWriteDecisionAtomic = original })
		assertNodeConnectorPlacementExecutionHandoffRejected(t, value, handoffs, value.fixture)
		for _, name := range []string{nodeConnectorPlacementExecutionHandoffDecisionName, nodeConnectorPlacementExecutionHandoffRequestName} {
			if _, err := os.Lstat(filepath.Join(value.base.root, name)); !os.IsNotExist(err) {
				t.Fatal("atomic decision failure left partial output")
			}
		}
		nodeConnectorPlacementExecutionHandoffWriteDecisionAtomic = original
		mustDecideNodeConnectorPlacementExecutionHandoff(t, handoffs, value.fixture)
	})

	t.Run("request-recovery", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
		handoffs := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
		brokerBefore := nodeExecutionStateArtifacts(t, value.base.brokerRoot)
		original := nodeConnectorPlacementExecutionHandoffWriteRequestAtomic
		nodeConnectorPlacementExecutionHandoffWriteRequestAtomic = func(string, any) error { return errors.New("injected request write failure") }
		t.Cleanup(func() { nodeConnectorPlacementExecutionHandoffWriteRequestAtomic = original })
		assertNodeConnectorPlacementExecutionHandoffRejected(t, value, handoffs, value.fixture)
		if _, err := os.Stat(filepath.Join(value.base.root, nodeConnectorPlacementExecutionHandoffDecisionName)); err != nil {
			t.Fatal("request publication failure did not preserve the durable decision")
		}
		if _, err := os.Lstat(filepath.Join(value.base.root, nodeConnectorPlacementExecutionHandoffRequestName)); !os.IsNotExist(err) {
			t.Fatal("atomic request failure left a partial request")
		}
		nodeConnectorPlacementExecutionHandoffWriteRequestAtomic = original
		decision, request := mustDecideNodeConnectorPlacementExecutionHandoff(t, handoffs, value.fixture)
		restarted := mustOpenNodeConnectorPlacementExecutionHandoffs(t, value)
		restartedDecision, restartedRequest := mustDecideNodeConnectorPlacementExecutionHandoff(t, restarted, value.fixture)
		if !reflect.DeepEqual(decision, restartedDecision) || !reflect.DeepEqual(request, restartedRequest) || !nodeExecutionStringSlicesEqual(brokerBefore, nodeExecutionStateArtifacts(t, value.base.brokerRoot)) {
			t.Fatal("retry/restart did not recover identical artifacts without broker mutation")
		}
	})
}

func TestNodeConnectorPlacementExecutionHandoffRestartRejectsTamperAndOrphanedEvidence(t *testing.T) {
	t.Run("tampered-request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
		mustDecideNodeConnectorPlacementExecutionHandoff(t, mustOpenNodeConnectorPlacementExecutionHandoffs(t, value), value.fixture)
		path := filepath.Join(value.base.root, nodeConnectorPlacementExecutionHandoffRequestName)
		raw := mustReadNodeConnectorPlacementExecutionHandoffFile(t, value.base.root, nodeConnectorPlacementExecutionHandoffRequestName)
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"connector_invoked": false`), []byte(`"connector_invoked": true`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionHandoffs(value.base.root, value.expected, value.base.broker); err == nil {
			t.Fatal("tampered durable handoff request was accepted or repaired")
		}
	})

	t.Run("orphaned-request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionHandoffTestFixture(t, "approved")
		mustDecideNodeConnectorPlacementExecutionHandoff(t, mustOpenNodeConnectorPlacementExecutionHandoffs(t, value), value.fixture)
		if err := os.Remove(filepath.Join(value.base.root, nodeConnectorPlacementExecutionHandoffDecisionName)); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionHandoffs(value.base.root, value.expected, value.base.broker); err == nil {
			t.Fatal("orphaned durable handoff request was accepted or repaired")
		}
	})
}

func TestNodeConnectorPlacementExecutionHandoffExistingSchemasRemainUnchanged(t *testing.T) {
	got := map[string]string{
		"machine": NodeExecutionMachineIdentitySchema, "capability": NodeExecutionCapabilitySnapshotSchema, "execution_request": NodeExecutionRequestSchema, "lease": NodeExecutionLeaseSchema,
		"connector_session": NodeConnectorSessionNegotiationSchema, "placement": NodeConnectorPlacementDecisionSchema, "placement_request": NodeConnectorPlacementRequestSchema,
		"dispatch": NodeConnectorPlacementDispatchDecisionSchema, "dispatch_request": NodeConnectorPlacementDispatchRequestSchema, "submission": NodeConnectorPlacementDispatchSubmissionSchema,
		"handoff_fixture": NodeConnectorPlacementExecutionHandoffDecisionFixtureSchema, "handoff_decision": NodeConnectorPlacementExecutionHandoffDecisionSchema, "handoff_request": NodeConnectorPlacementExecutionHandoffRequestSchema,
	}
	want := map[string]string{
		"machine": "dorkpipe.node-execution.machine-identity/v1", "capability": "dorkpipe.node-execution.capability-snapshot/v1", "execution_request": "dorkpipe.node-execution.execution-request/v1", "lease": "dorkpipe.node-execution.task-lease/v1",
		"connector_session": "dorkpipe.node-connector.session-negotiation/v1", "placement": "dorkpipe.node-placement-decision/v1", "placement_request": "dorkpipe.node-placement-request/v1",
		"dispatch": "dorkpipe.node-placement-dispatch-decision/v1", "dispatch_request": "dorkpipe.node-placement-dispatch-request/v1", "submission": "dorkpipe.node-placement-dispatch-submission/v1",
		"handoff_fixture": "dorkpipe.node-placement-execution-handoff-decision-fixture/v1", "handoff_decision": "dorkpipe.node-placement-execution-handoff-decision/v1", "handoff_request": "dorkpipe.node-placement-execution-handoff-request/v1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("an existing or new TASK-015 schema changed unexpectedly: %#v", got)
	}
}

func newNodeConnectorPlacementExecutionHandoffTestFixture(t *testing.T, decision string) *nodeConnectorPlacementExecutionHandoffTestFixture {
	t.Helper()
	base := newNodeConnectorPlacementDispatchSubmissionTestFixture(t, "approved", true)
	submission := mustSubmitNodeConnectorPlacementDispatch(t, mustOpenNodeConnectorPlacementDispatchSubmissions(t, base), base.connection, base.fixture)
	expected := NodeConnectorPlacementExecutionHandoffExpected{Submission: base.expected, SubmissionFingerprint: submission.SubmissionFingerprint}
	fixture := NodeConnectorPlacementExecutionHandoffDecisionFixture{
		Schema: NodeConnectorPlacementExecutionHandoffDecisionFixtureSchema, DecisionID: "placement-execution-handoff-decision-001", ReplayIdentity: "replay-placement-execution-handoff-001", Decision: decision,
		Reason: "explicit fixture connector handoff approval", IssuedAt: nodeExecutionTime(time.Date(2026, 7, 28, 21, 2, 0, 0, time.UTC)),
		SubmissionID: submission.SubmissionID, SubmissionReplayIdentity: submission.ReplayIdentity, SubmissionFingerprint: submission.SubmissionFingerprint, SubmissionProvenance: submission.Provenance,
		InventorySnapshotID: submission.InventorySnapshotID, InventorySnapshotFingerprint: submission.InventorySnapshotFingerprint, PlacementInputID: submission.PlacementInputID, PlacementInputFingerprint: submission.PlacementInputSnapshotFingerprint,
		PlacementDecisionID: submission.PlacementDecisionID, PlacementDecisionFingerprint: submission.PlacementDecisionFingerprint, PlacementRequestID: submission.PlacementRequestID, PlacementRequestFingerprint: submission.PlacementRequestFingerprint,
		DispatchDecisionID: submission.PlacementDispatchDecisionID, DispatchDecisionFingerprint: submission.PlacementDispatchDecisionFingerprint, DispatchRequestID: submission.PlacementDispatchRequestID, DispatchRequestFingerprint: submission.PlacementDispatchRequestFingerprint,
		WorkloadID: submission.WorkloadID, CandidateNodeIDs: append([]string{}, submission.CandidateNodeIDs...), SelectedNode: submission.SelectedNode,
		ExecutionTaskID: submission.ExecutionTaskID, ExecutionRequest: cloneNodeExecutionRequest(submission.ExecutionRequest), ExecutionRequestFingerprint: submission.ExecutionRequestFingerprint,
		BrokerStateFingerprint: submission.BrokerStateFingerprint, TaskLease: submission.TaskLease, Provenance: nodeConnectorPlacementExecutionHandoffDecisionProvenance,
	}
	if decision == "approved" {
		fixture.ExecutionHandoffRequestID = "placement-execution-handoff-request-001"
	}
	return &nodeConnectorPlacementExecutionHandoffTestFixture{base: base, submission: submission, expected: expected, fixture: fixture}
}

func mustOpenNodeConnectorPlacementExecutionHandoffs(t *testing.T, value *nodeConnectorPlacementExecutionHandoffTestFixture) *NodeConnectorPlacementExecutionHandoffs {
	t.Helper()
	handoffs, err := OpenNodeConnectorPlacementExecutionHandoffs(value.base.root, value.expected, value.base.broker)
	if err != nil {
		t.Fatal(err)
	}
	return handoffs
}

func mustDecideNodeConnectorPlacementExecutionHandoff(t *testing.T, handoffs *NodeConnectorPlacementExecutionHandoffs, fixture NodeConnectorPlacementExecutionHandoffDecisionFixture) (NodeConnectorPlacementExecutionHandoffDecision, *NodeConnectorPlacementExecutionHandoffRequest) {
	t.Helper()
	decision, request, err := handoffs.Decide(mustMarshalNodeConnectorPlacementExecutionHandoff(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func assertNodeConnectorPlacementExecutionHandoffRejected(t *testing.T, value *nodeConnectorPlacementExecutionHandoffTestFixture, handoffs *NodeConnectorPlacementExecutionHandoffs, fixture NodeConnectorPlacementExecutionHandoffDecisionFixture) {
	t.Helper()
	assertNodeConnectorPlacementExecutionHandoffRawRejected(t, value, handoffs, mustMarshalNodeConnectorPlacementExecutionHandoff(t, fixture))
}

func assertNodeConnectorPlacementExecutionHandoffRawRejected(t *testing.T, value *nodeConnectorPlacementExecutionHandoffTestFixture, handoffs *NodeConnectorPlacementExecutionHandoffs, raw []byte) {
	t.Helper()
	before := nodeExecutionStateArtifacts(t, value.base.brokerRoot)
	if _, _, err := handoffs.Decide(raw); err == nil {
		t.Fatal("changed, inferred, malformed, or conflicting handoff input was accepted")
	}
	if !nodeExecutionStringSlicesEqual(before, nodeExecutionStateArtifacts(t, value.base.brokerRoot)) {
		t.Fatal("rejected handoff input changed broker generation evidence")
	}
}

func mustMarshalNodeConnectorPlacementExecutionHandoff(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustReadNodeConnectorPlacementExecutionHandoffFile(t *testing.T, root, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorPlacementExecutionHandoffDecisionFixture(value NodeConnectorPlacementExecutionHandoffDecisionFixture) NodeConnectorPlacementExecutionHandoffDecisionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionHandoffDecisionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type nodeConnectorPlacementExecutionGraphReconciliationTestFixture struct {
	reconciliation *nodeConnectorPlacementExecutionReconciliationTestFixture
	decision       NodeConnectorPlacementExecutionReconciliationDecision
	request        NodeConnectorPlacementExecutionReconciliationRequest
	expected       NodeConnectorPlacementExecutionGraphReconciliationExpected
}

func TestNodeConnectorPlacementExecutionGraphReconciliationPassedTaskConsumesOnlyExactRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
	root := value.reconciliation.deliveryValue.handoff.base.root
	requestBefore := mustReadNodeConnectorPlacementExecutionGraphReconciliationFile(t, root, nodeConnectorPlacementExecutionReconciliationRequestName)
	brokerBefore := nodeConnectorStateBytes(t, value.reconciliation.deliveryValue.handoff.base.brokerRoot)
	validationBefore := *value.reconciliation.deliveryValue.validationCalls
	artifact := mustReconcileNodeConnectorPlacementExecutionGraph(t, mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, value))
	delivery, receipt := value.reconciliation.delivery, value.reconciliation.delivery.Receipt
	leaseFingerprint, _ := nodeExecutionFingerprintValue(delivery.TaskLease)
	eventsFingerprint, _ := nodeExecutionFingerprintValue(delivery.Events)
	if artifact.TaskOutcome != "passed" || artifact.TerminalResult != "succeeded" || artifact.CleanupStatus != "not_required" || artifact.CleanupEvidenceDigest != "" {
		t.Fatalf("valid successful receipt was not interpreted as a passed task: %#v", artifact)
	}
	if artifact.GraphRunID != delivery.ExecutionRequest.GraphRunID || artifact.RunID != delivery.ExecutionRequest.RunID || artifact.TaskID != delivery.ExecutionRequest.TaskID || artifact.OperationID != delivery.ExecutionRequest.OperationID || artifact.Attempt != delivery.TaskLease.Attempt || artifact.ExecutionRequestFingerprint != delivery.ExecutionRequestFingerprint || artifact.LeaseID != delivery.TaskLease.LeaseID || artifact.LeaseFingerprint != leaseFingerprint || artifact.EventStreamFingerprint != eventsFingerprint || artifact.ReceiptID != receipt.ReceiptID || artifact.ReceiptFingerprint != receipt.ReceiptFingerprint || artifact.ArtifactManifestFingerprint != receipt.Artifacts.ManifestFingerprint || artifact.DeliveryFingerprint != delivery.DeliveryFingerprint || artifact.ReconciliationDecisionFingerprint != value.decision.DecisionFingerprint || artifact.ReconciliationRequestFingerprint != value.request.RequestFingerprint {
		t.Fatal("graph reconciliation omitted or substituted an immutable identity or fingerprint binding")
	}
	if !artifact.CompleteChainRevalidated || !artifact.AuthorizationConsumed || !artifact.TerminalOutcomeInterpreted || !artifact.ReceiptAuthoritative || !artifact.GraphReconciliationPerformed || artifact.EventsAuthoritative || artifact.ProviderEvidenceAuthoritative || artifact.GraphCompletionClaimed || artifact.GraphFailurePropagated || artifact.NextTaskScheduled || artifact.ExecutionOrLifecycleSideEffects || artifact.Authority != (NodeConnectorPlacementExecutionGraphReconciliationAuthority{}) {
		t.Fatal("graph reconciliation widened authority or omitted its exact consumption and interpretation record")
	}
	if artifact.ConnectorInvoked || artifact.PreparedValidationInvoked || artifact.BrokerExecutorInvoked || artifact.BrokerOperationCreated || artifact.LeaseCreated || artifact.AttemptCreated || artifact.ConnectionCreated || artifact.SessionCreated || artifact.EnrollmentCreated || artifact.CredentialCreated || artifact.EventCreated || artifact.ReceiptCreated || artifact.DeliveryCreated {
		t.Fatal("graph reconciliation claimed a forbidden execution or lifecycle side effect")
	}
	if !bytes.Equal(requestBefore, mustReadNodeConnectorPlacementExecutionGraphReconciliationFile(t, root, nodeConnectorPlacementExecutionReconciliationRequestName)) || validationBefore != *value.reconciliation.deliveryValue.validationCalls || !nodeConnectorStateBytesEqual(brokerBefore, nodeConnectorStateBytes(t, value.reconciliation.deliveryValue.handoff.base.brokerRoot)) || artifact.ReconciliationRequest.AuthorizationConsumed {
		t.Fatal("durable consumption changed the immutable request, reinvoked validation, or changed broker history")
	}
	if delivery.BrokerExecutorInvocations != 0 || artifact.BrokerExecutorInvoked {
		t.Fatal("graph reconciliation invoked the fake-broker executor")
	}
}

func TestNodeConnectorPlacementExecutionGraphReconciliationFailedReceiptsRemainFailed(t *testing.T) {
	for _, result := range []string{"failed", "degraded"} {
		t.Run(result, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, result)
			artifact := mustReconcileNodeConnectorPlacementExecutionGraph(t, mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, value))
			if artifact.TaskOutcome != "failed" || artifact.TerminalResult != result || artifact.CleanupStatus != "not_required" {
				t.Fatalf("valid %s receipt did not produce a failed task outcome: %#v", result, artifact)
			}
			if artifact.EventsAuthoritative || !artifact.ReceiptAuthoritative {
				t.Fatal("events displaced the execution receipt as the terminal authority")
			}
		})
	}

	t.Run("cancelled", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
		receipt := value.reconciliation.delivery.Receipt
		receipt.Result = "cancelled"
		receipt.CancellationID = value.reconciliation.delivery.TaskLease.CancellationID
		receipt.CancellationAcknowledged = true
		receipt.Cleanup = NodeExecutionCleanupOutcome{Status: "succeeded", EvidenceDigest: nodeExecutionTestDigest("cancelled-cleanup")}
		finalized, err := FinalizeNodeExecutionReceipt(receipt)
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := nodeConnectorPlacementExecutionTaskOutcome(finalized)
		if err != nil || outcome != "failed" {
			t.Fatalf("valid cancelled terminal receipt did not map to failed: outcome=%q err=%v", outcome, err)
		}
		if finalized.Cleanup.Status != "succeeded" || finalized.Cleanup.EvidenceDigest == "" {
			t.Fatal("cancelled receipt did not preserve exact cleanup evidence")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphReconciliationReceiptNotEventsOrClaimsIsAuthoritative(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "failed")
	if len(value.reconciliation.delivery.Events) < 2 || !bytes.Contains(value.reconciliation.delivery.Events[0].Event, []byte(`"status":"done"`)) || !bytes.Contains(value.reconciliation.delivery.Events[len(value.reconciliation.delivery.Events)-1].Event, []byte(`"status":"fail"`)) {
		t.Fatal("test precondition requires nonterminal successful-looking evidence followed by a valid failed terminal event")
	}
	artifact := mustReconcileNodeConnectorPlacementExecutionGraph(t, mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, value))
	if artifact.TaskOutcome != "failed" || artifact.TerminalResult != "failed" || artifact.EventsAuthoritative || artifact.ProviderEvidenceAuthoritative {
		t.Fatal("event or provider-shaped evidence overrode the validated failed receipt")
	}
}

func TestNodeConnectorPlacementExecutionGraphReconciliationReplayRestartAndConcurrencyConverge(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
	reconciliations := mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, value)
	first := mustReconcileNodeConnectorPlacementExecutionGraph(t, reconciliations)
	root := value.reconciliation.deliveryValue.handoff.base.root
	raw := mustReadNodeConnectorPlacementExecutionGraphReconciliationFile(t, root, nodeConnectorPlacementExecutionGraphReconciliationName)
	second := mustReconcileNodeConnectorPlacementExecutionGraph(t, reconciliations)
	restarted := mustReconcileNodeConnectorPlacementExecutionGraph(t, mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, value))
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, restarted) || !bytes.Equal(raw, mustReadNodeConnectorPlacementExecutionGraphReconciliationFile(t, root, nodeConnectorPlacementExecutionGraphReconciliationName)) {
		t.Fatal("exact replay or restart rewrote or changed the canonical graph reconciliation")
	}

	const callers = 16
	results := make(chan NodeConnectorPlacementExecutionGraphReconciliation, callers)
	errs := make(chan error, callers)
	var wait sync.WaitGroup
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := reconciliations.Reconcile()
			if err != nil {
				errs <- err
				return
			}
			results <- result
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for result := range results {
		if !reflect.DeepEqual(first, result) {
			t.Fatal("concurrent exact calls did not converge on the canonical artifact")
		}
	}
}

func TestNodeConnectorPlacementExecutionGraphReconciliationRequiresApprovedUnconsumedAuthorization(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
		expected := NodeConnectorPlacementExecutionGraphReconciliationExpected{Reconciliation: value.expected, ReconciliationDecisionFingerprint: nodeConnectorInventoryFingerprint("missing-decision"), ReconciliationRequestFingerprint: nodeConnectorInventoryFingerprint("missing-request")}
		if _, err := OpenNodeConnectorPlacementExecutionGraphReconciliations(value.deliveryValue.handoff.base.root, expected, value.deliveryValue.handoff.base.broker); err == nil {
			t.Fatal("missing reconciliation authorization was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphReconciliationAbsent(t, value.deliveryValue.handoff.base.root)
	})

	t.Run("rejected", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "rejected")
		decision, request := mustDecideNodeConnectorPlacementExecutionReconciliation(t, mustOpenNodeConnectorPlacementExecutionReconciliations(t, value), value.fixture)
		if request != nil {
			t.Fatal("test precondition requires no request from rejected decision")
		}
		expected := NodeConnectorPlacementExecutionGraphReconciliationExpected{Reconciliation: value.expected, ReconciliationDecisionFingerprint: decision.DecisionFingerprint, ReconciliationRequestFingerprint: nodeConnectorInventoryFingerprint("rejected-request")}
		if _, err := OpenNodeConnectorPlacementExecutionGraphReconciliations(value.deliveryValue.handoff.base.root, expected, value.deliveryValue.handoff.base.broker); err == nil {
			t.Fatal("rejected reconciliation decision was treated as authority")
		}
		assertNodeConnectorPlacementExecutionGraphReconciliationAbsent(t, value.deliveryValue.handoff.base.root)
	})

	value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
	root := value.reconciliation.deliveryValue.handoff.base.root
	requestPath := filepath.Join(root, nodeConnectorPlacementExecutionReconciliationRequestName)
	requestRaw := mustReadNodeConnectorPlacementExecutionGraphReconciliationFile(t, root, nodeConnectorPlacementExecutionReconciliationRequestName)
	for _, mutation := range []struct {
		name string
		from string
		to   string
	}{
		{"already-consumed", `"authorization_consumed": false`, `"authorization_consumed": true`},
		{"authority-escalation", `"graph_reconciliation": false`, `"graph_reconciliation": true`},
		{"stale-request", `"request_id": "placement-execution-reconciliation-request-001"`, `"request_id": "placement-execution-reconciliation-request-stale-001"`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if err := os.WriteFile(requestPath, requestRaw, 0o644); err != nil {
				t.Fatal(err)
			}
			mutateNodeConnectorPlacementExecutionGraphReconciliationFile(t, filepath.Join(root, nodeConnectorPlacementExecutionReconciliationRequestName), []byte(mutation.from), []byte(mutation.to))
			if _, err := OpenNodeConnectorPlacementExecutionGraphReconciliations(root, value.expected, value.reconciliation.deliveryValue.handoff.base.broker); err == nil {
				t.Fatal("consumed, escalated, or stale reconciliation authorization was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphReconciliationAbsent(t, root)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphReconciliationRejectsTamperAndSubstitutionAtEveryBoundary(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
	root := value.reconciliation.deliveryValue.handoff.base.root

	tests := []struct {
		name string
		path func(*nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string
		from func(*nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string
		to   string
	}{
		{"dispatch-submission", func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return filepath.Join(v.reconciliation.deliveryValue.handoff.base.root, nodeConnectorPlacementDispatchSubmissionName)
		}, func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return v.reconciliation.delivery.SubmissionID
		}, "submission-placement-substituted-001"},
		{"handoff-decision", func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return filepath.Join(v.reconciliation.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionHandoffDecisionName)
		}, func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return v.reconciliation.delivery.HandoffDecisionID
		}, "placement-execution-handoff-decision-substituted-001"},
		{"handoff-request", func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return filepath.Join(v.reconciliation.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionHandoffRequestName)
		}, func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return v.reconciliation.delivery.HandoffRequestID
		}, "placement-execution-handoff-request-substituted-001"},
		{"terminal-delivery", func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return filepath.Join(v.reconciliation.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionDeliveryName)
		}, func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return v.reconciliation.delivery.DeliveryID
		}, "placement-execution-delivery-substituted-001"},
		{"reconciliation-decision", func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return filepath.Join(v.reconciliation.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationDecisionName)
		}, func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return v.decision.DecisionID
		}, "placement-execution-reconciliation-decision-substituted-001"},
		{"reconciliation-request", func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return filepath.Join(v.reconciliation.deliveryValue.handoff.base.root, nodeConnectorPlacementExecutionReconciliationRequestName)
		}, func(v *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) string {
			return v.request.RequestID
		}, "placement-execution-reconciliation-request-substituted-001"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := test.path(value)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.WriteFile(path, raw, 0o644); err != nil {
					t.Error(err)
				}
			}()
			mutateNodeConnectorPlacementExecutionGraphReconciliationFile(t, path, []byte(test.from(value)), []byte(test.to))
			if _, err := OpenNodeConnectorPlacementExecutionGraphReconciliations(root, value.expected, value.reconciliation.deliveryValue.handoff.base.broker); err == nil {
				t.Fatal("tampered or substituted immutable chain boundary was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphReconciliationAbsent(t, root)
		})
	}

	t.Run("broker-operation", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
		states := nodeExecutionStateArtifacts(t, value.reconciliation.deliveryValue.handoff.base.brokerRoot)
		path := filepath.Join(value.reconciliation.deliveryValue.handoff.base.brokerRoot, states[len(states)-1])
		mutateNodeConnectorPlacementExecutionGraphReconciliationFile(t, path, []byte(`"result": "succeeded"`), []byte(`"result": "failed"`))
		if _, err := OpenNodeConnectorPlacementExecutionGraphReconciliations(value.reconciliation.deliveryValue.handoff.base.root, value.expected, value.reconciliation.deliveryValue.handoff.base.broker); err == nil {
			t.Fatal("tampered terminal broker operation was accepted")
		}
	})

	t.Run("expected-identity", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
		changed := value.expected
		changed.ReconciliationRequestFingerprint = nodeConnectorInventoryFingerprint("substituted-request")
		if _, err := OpenNodeConnectorPlacementExecutionGraphReconciliations(value.reconciliation.deliveryValue.handoff.base.root, changed, value.reconciliation.deliveryValue.handoff.base.broker); err == nil {
			t.Fatal("substituted expected request fingerprint was accepted")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphReconciliationRejectsMalformedUnknownTrailingOversizedAndConflictingArtifacts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
	mustReconcileNodeConnectorPlacementExecutionGraph(t, mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, value))
	root := value.reconciliation.deliveryValue.handoff.base.root
	path := filepath.Join(root, nodeConnectorPlacementExecutionGraphReconciliationName)
	raw := mustReadNodeConnectorPlacementExecutionGraphReconciliationFile(t, root, nodeConnectorPlacementExecutionGraphReconciliationName)

	mutations := []struct {
		name string
		raw  func([]byte) []byte
	}{
		{"malformed", func([]byte) []byte { return []byte("{not-json") }},
		{"unknown-field", func(raw []byte) []byte {
			return append(bytes.TrimSuffix(raw, []byte("}\n")), []byte(",\n  \"unknown\": true\n}\n")...)
		}},
		{"trailing-content", func(raw []byte) []byte { return append(append([]byte{}, raw...), []byte("trailing")...) }},
		{"oversized", func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphReconciliationMaxArtifactBytes+1)
		}},
		{"conflicting", func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"task_outcome": "passed"`), []byte(`"task_outcome": "failed"`), 1)
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if err := os.WriteFile(path, mutation.raw(raw), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenNodeConnectorPlacementExecutionGraphReconciliations(root, value.expected, value.reconciliation.deliveryValue.handoff.base.broker); err == nil {
				t.Fatal("malformed, unknown, trailing, oversized, or conflicting durable artifact was accepted or repaired")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphReconciliationAtomicWriteFailureLeavesNoPartialArtifact(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
	original := nodeConnectorPlacementExecutionGraphReconciliationWriteAtomic
	nodeConnectorPlacementExecutionGraphReconciliationWriteAtomic = func(string, any) error { return errors.New("injected atomic write failure") }
	t.Cleanup(func() { nodeConnectorPlacementExecutionGraphReconciliationWriteAtomic = original })
	if _, err := mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, value).Reconcile(); err == nil {
		t.Fatal("atomic write failure was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphReconciliationAbsent(t, value.reconciliation.deliveryValue.handoff.base.root)
}

func newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t *testing.T, result string) *nodeConnectorPlacementExecutionGraphReconciliationTestFixture {
	t.Helper()
	var reconciliation *nodeConnectorPlacementExecutionReconciliationTestFixture
	if result == "succeeded" {
		reconciliation = newNodeConnectorPlacementExecutionReconciliationTestFixture(t, "approved")
	} else {
		deliveryValue := newNodeConnectorPlacementExecutionDeliveryTestFixture(t)
		evidenceFixture := &nodeExecutionTestFixture{request: deliveryValue.request.ExecutionRequest, now: time.Date(2026, 7, 28, 21, 2, 0, 0, time.UTC)}
		evidence := nodeConnectorTestEvidence(t, evidenceFixture)
		evidence.TerminalResult = result
		evidence.Events[0].Event = nodeConnectorOperationEvent(t, evidenceFixture.now.Add(time.Minute), "done")
		evidence.Events[len(evidence.Events)-1].Event = nodeConnectorOperationEvent(t, evidenceFixture.now.Add(2*time.Minute), "fail")
		connector, err := NewNodeValidationConnector(deliveryValue.request.ExecutionRequest.Workflow, deliveryValue.request.ExecutionRequest.SourceRevision, func(NodeValidationInvocation) (NodeValidationEvidence, error) {
			*deliveryValue.validationCalls++
			return evidence, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := prepareNodeValidationDelivery(deliveryValue.request.ExecutionRequest, deliveryValue.request.TaskLease, nil, evidence)
		if err != nil {
			t.Fatal(err)
		}
		deliveryValue.connector = connector
		deliveryValue.fixture.ExpectedEvents = cloneNodeConnectorPlacementExecutionDeliveryEvents(prepared.events)
		deliveryValue.fixture.ExpectedReceipt = prepared.receipt
		deliveryValue.fixture.ExpectedReceiptFingerprint = prepared.receipt.ReceiptFingerprint
		delivery := mustDeliverNodeConnectorPlacementExecution(t, mustOpenNodeConnectorPlacementExecutionDeliveries(t, deliveryValue), deliveryValue.fixture, deliveryValue.connector)
		deliveryValue.handoff.base.broker.Disconnect(deliveryValue.negotiation.ConnectionID)
		reconciliationExpected := NodeConnectorPlacementExecutionReconciliationExpected{Delivery: deliveryValue.expected, DeliveryFingerprint: delivery.DeliveryFingerprint}
		decisionFixture := NodeConnectorPlacementExecutionReconciliationDecisionFixture{
			Schema: NodeConnectorPlacementExecutionReconciliationDecisionFixtureSchema, DecisionID: "placement-execution-reconciliation-decision-001", ReplayIdentity: "replay-placement-execution-reconciliation-001", Decision: "approved",
			Delivery: cloneNodeConnectorPlacementExecutionDelivery(delivery), ReconciliationRequestID: "placement-execution-reconciliation-request-001", Provenance: nodeConnectorPlacementExecutionReconciliationDecisionProvenance,
		}
		reconciliation = &nodeConnectorPlacementExecutionReconciliationTestFixture{deliveryValue: deliveryValue, delivery: delivery, expected: reconciliationExpected, fixture: decisionFixture}
	}
	decision, requestPointer := mustDecideNodeConnectorPlacementExecutionReconciliation(t, mustOpenNodeConnectorPlacementExecutionReconciliations(t, reconciliation), reconciliation.fixture)
	request := *requestPointer
	expected := NodeConnectorPlacementExecutionGraphReconciliationExpected{Reconciliation: reconciliation.expected, ReconciliationDecisionFingerprint: decision.DecisionFingerprint, ReconciliationRequestFingerprint: request.RequestFingerprint}
	return &nodeConnectorPlacementExecutionGraphReconciliationTestFixture{reconciliation: reconciliation, decision: decision, request: request, expected: expected}
}

func mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t *testing.T, value *nodeConnectorPlacementExecutionGraphReconciliationTestFixture) *NodeConnectorPlacementExecutionGraphReconciliations {
	t.Helper()
	reconciliations, err := OpenNodeConnectorPlacementExecutionGraphReconciliations(value.reconciliation.deliveryValue.handoff.base.root, value.expected, value.reconciliation.deliveryValue.handoff.base.broker)
	if err != nil {
		t.Fatal(err)
	}
	return reconciliations
}

func mustReconcileNodeConnectorPlacementExecutionGraph(t *testing.T, reconciliations *NodeConnectorPlacementExecutionGraphReconciliations) NodeConnectorPlacementExecutionGraphReconciliation {
	t.Helper()
	artifact, err := reconciliations.Reconcile()
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

func mutateNodeConnectorPlacementExecutionGraphReconciliationFile(t *testing.T, path string, from, to []byte) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(raw, from, to, 1)
	if bytes.Equal(raw, changed) {
		t.Fatalf("test mutation source was not found in %s", path)
	}
	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadNodeConnectorPlacementExecutionGraphReconciliationFile(t *testing.T, root, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertNodeConnectorPlacementExecutionGraphReconciliationAbsent(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementExecutionGraphReconciliationName)); !os.IsNotExist(err) {
		t.Fatal("rejected graph reconciliation published a partial artifact")
	}
}

func TestNodeConnectorPlacementExecutionGraphReconciliationSchemaAndForbiddenSurfaceRemainBounded(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, "succeeded")
	artifact := mustReconcileNodeConnectorPlacementExecutionGraph(t, mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, value))
	if artifact.Schema != "dorkpipe.node-placement-execution-graph-reconciliation/v1" || value.request.Authority != (NodeConnectorPlacementExecutionReconciliationAuthority{LocalGraphReconciliationRequest: true}) || artifact.Authority != (NodeConnectorPlacementExecutionGraphReconciliationAuthority{}) {
		t.Fatal("graph reconciliation schema or authority boundary changed")
	}
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"graph_completion_claimed":true`, `"graph_failure_propagated":true`, `"next_task_scheduled":true`, `"publication":true`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("forbidden graph or publication authority appeared: %s", forbidden)
		}
	}
}

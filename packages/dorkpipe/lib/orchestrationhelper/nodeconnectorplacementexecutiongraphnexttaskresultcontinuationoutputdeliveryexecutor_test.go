package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
)

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture struct {
	root                string
	expected            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected
	decision            NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision
	request             NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest
	output              NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord
	outputReceipt       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
	consumer            *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake
	acknowledgementPath string
	receiptPath         string
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestTemplate struct {
	once    sync.Once
	fixture nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture
	files   map[string][]byte
}

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestTemplates = map[string]*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestTemplate{
	"succeeded\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute:           {},
	"succeeded\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute: {},
	"failed\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute:        {},
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake struct {
	mu                  sync.Mutex
	consumerID          string
	contractFingerprint string
	operationKey        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey
	request             NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest
	output              NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord
	acknowledgement     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
	accepted            bool
	deliveryCount       int
	lookupError         error
	deliveryError       error
	mutate              func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement)
}

func (consumer *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake) ConsumerID() string {
	return consumer.consumerID
}

func (consumer *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake) ConsumerContractFingerprint() string {
	return consumer.contractFingerprint
}

func (consumer *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake) LookupAcknowledgement(operationKey NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, bool, error) {
	consumer.mu.Lock()
	defer consumer.mu.Unlock()
	if consumer.lookupError != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, false, consumer.lookupError
	}
	if operationKey != consumer.operationKey {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, false, errors.New("unexpected operation key")
	}
	if !consumer.accepted {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, false, nil
	}
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(consumer.acknowledgement), true, nil
}

func (consumer *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake) Deliver(operationKey NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey, request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest, output NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, error) {
	consumer.mu.Lock()
	defer consumer.mu.Unlock()
	consumer.deliveryCount++
	if consumer.deliveryError != nil {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, consumer.deliveryError
	}
	if operationKey != consumer.operationKey || !nodeExecutionEqual(request, consumer.request) || !nodeExecutionEqual(output, consumer.output) {
		return NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, errors.New("consumer received a conflicting request or output")
	}
	acknowledgement := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(consumer.acknowledgement)
	if consumer.mutate != nil {
		consumer.mutate(&acknowledgement)
	}
	consumer.acknowledgement = acknowledgement
	consumer.accepted = true
	return cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(acknowledgement), nil
}

func (consumer *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake) deliveries() int {
	consumer.mu.Lock()
	defer consumer.mu.Unlock()
	return consumer.deliveryCount
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExactRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, result, route, postState, outputType, deliveryType, outcome string
		authority                                                         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority
	}{
		{"continuation handoff", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "continued", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffDelivery, "passed", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{ContinuationHandoffDeliveryAttempt: true}},
		{"successful terminal result", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery, "passed", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{SuccessfulTerminalGraphResultDeliveryAttempt: true}},
		{"failed terminal result", "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery, "failed", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{FailedTerminalGraphResultDeliveryAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, test.result, test.route)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			acknowledgement, receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value)
			if value.request.Authority != test.authority || acknowledgement.Binding.Route != test.route || acknowledgement.Binding.PostState != test.postState || acknowledgement.Binding.OutputType != test.outputType || acknowledgement.Binding.DeliveryType != test.deliveryType || acknowledgement.Binding.TaskOutcome != test.outcome || receipt.Binding != acknowledgement.Binding {
				t.Fatal("delivery executor did not preserve the exact mutually exclusive route, output, delivery, state, outcome, or authority binding")
			}
			if !acknowledgement.Accepted || acknowledgement.AcceptedLocalConsumerDeliveryCount != 1 || !acknowledgement.FixtureOwned || receipt.LogicalDeliveryAttemptCount != 1 || receipt.ConsumerInvocationCount != 1 || receipt.AcceptedAcknowledgementCount != 1 || receipt.AcknowledgementArtifactWriteCount != 1 || receipt.ExecutorReceiptWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.CompleteImmutablePredecessorChainRevalidated || !receipt.NoDuplicateDelivery || receipt.ConsumerReinvoked || !receipt.FixtureOwned || value.consumer.deliveries() != 1 {
				t.Fatal("delivery executor did not record exactly one accepted consumer delivery and durable consumption receipt")
			}
			if acknowledgement.OperationKey.RequestID != value.request.RequestID || acknowledgement.OperationKey.ReplayIdentity != value.request.DecisionReplayIdentity || acknowledgement.Binding.DeliveryPolicyDecisionID != value.decision.DecisionID || acknowledgement.Binding.DeliveryPolicyRequestID != value.request.RequestID || acknowledgement.Binding.DeliveryAuthenticationID != value.request.AuthenticationID || acknowledgement.Binding.OutputRecordID != value.output.OutputRecordID || acknowledgement.Binding.OutputExecutorReceiptID != value.outputReceipt.ExecutorReceiptID || acknowledgement.Binding.ConsumerID != value.consumer.consumerID || acknowledgement.Binding.ConsumerContractFingerprint != value.consumer.contractFingerprint || acknowledgement.Binding.GraphRunID == "" || acknowledgement.Binding.TerminalTaskID == "" || acknowledgement.Binding.SelectedTaskID == "" || acknowledgement.Binding.CandidatesFingerprint == "" || acknowledgement.Binding.AcceptedResultID == "" || acknowledgement.Binding.ReconciliationReceiptID == "" {
				t.Fatal("acknowledgement omitted an exact request, authentication, output, consumer, graph, task, candidate, result, or reconciliation binding")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorNoAuthority(t, acknowledgement, receipt)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorOnlyOutputsChanged(t, value, before)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRejectsChangedCompletePredecessorChain(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	raw, _ := json.Marshal(value.expected)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected)
	}{
		{"delivery decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"delivery request", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"output record", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.OutputRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"output receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.OutputExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"output policy decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"output policy request", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"transition receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"transition record", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.TransitionRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"continuation policy", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"reconciliation receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('0')
		}},
		{"accepted result", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a')
		}},
		{"observation authentication", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.Reconciliation.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('b')
		}},
		{"launch receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.Reconciliation.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('c')
		}},
		{"launch authorization", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.Reconciliation.Executor.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('d')
		}},
		{"scheduling", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.Reconciliation.Executor.Policy.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('e')
		}},
		{"dependency transition", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('f')
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var changed NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected
			_ = json.Unmarshal(raw, &changed)
			test.mutate(&changed)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(value.root, changed, value.consumer); err == nil {
				t.Fatal("changed immutable predecessor chain was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifactsAbsent(t, value)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRejectsRequestOutputConsumerAndAuthorityConflicts(t *testing.T) {
	t.Parallel()
	base := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	decisionPath := filepath.Join(base.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionName)
	requestPath := filepath.Join(base.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName)
	outputPath := filepath.Join(base.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName)
	outputReceiptPath := filepath.Join(base.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName)
	decisionRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, decisionPath)
	requestRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, requestPath)
	outputRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, outputPath)
	outputReceiptRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, outputReceiptPath)
	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture)
	}{
		{"missing decision", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			_ = os.Remove(filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionName))
		}},
		{"missing request", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			_ = os.Remove(filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName))
		}},
		{"missing output", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			_ = os.Remove(filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName))
		}},
		{"missing output receipt", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			_ = os.Remove(filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName))
		}},
		{"consumed request", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.request.AuthorizationConsumed = true
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRequest(t, v)
		}},
		{"replayed request", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.request.DeliveryPerformed = true
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRequest(t, v)
		}},
		{"inferred acknowledgement", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.request.AcknowledgementReceived = true
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRequest(t, v)
		}},
		{"authority escalation", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.request.Authority.Publication = true
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRequest(t, v)
		}},
		{"continuation as terminal delivery", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.request.DeliveryType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRequest(t, v)
		}},
		{"continuation as terminal output", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.request.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRequest(t, v)
		}},
		{"changed route", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.request.Route = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRequest(t, v)
		}},
		{"wrong output action count", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.outputReceipt.OutputActionCount = 2
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorOutputReceipt(t, v)
		}},
		{"wrong output write count", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.outputReceipt.OutputRecordWriteCount = 2
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorOutputReceipt(t, v)
		}},
		{"unconsumed output authorization", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.outputReceipt.AuthorizationConsumed = false
			rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorOutputReceipt(t, v)
		}},
		{"wrong output version", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.output.Version = 2
			v.output.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(v.output)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName), v.output)
		}},
		{"non fixture output", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
			v.output.FixtureOwned = false
			v.output.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(v.output)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName), v.output)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer mustRestoreNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorPredecessorArtifacts(t, decisionPath, requestPath, outputPath, outputReceiptPath, decisionRaw, requestRaw, outputRaw, outputReceiptRaw)
			changed := *base
			changed.request = cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(base.request)
			changed.output = cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(base.output)
			changed.outputReceipt = base.outputReceipt
			test.mutate(&changed)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(changed.root, changed.expected, changed.consumer); err == nil {
				t.Fatal("missing, inferred, consumed, replayed, route-incompatible, output-invalid, or authority-escalated evidence was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifactsAbsent(t, &changed)
		})
	}

	for _, test := range []struct {
		name     string
		consumer NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumer
	}{
		{"missing consumer", nil},
		{"wrong consumer", &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake{consumerID: "downstream-consumer-wrong-001", contractFingerprint: base.consumer.contractFingerprint}},
		{"wrong contract", &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake{consumerID: base.consumer.consumerID, contractFingerprint: testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(base.root, base.expected, test.consumer); err == nil {
				t.Fatal("missing, wrong, or ambiguous consumer contract was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorConsumerFailurePublishesNothing(t *testing.T) {
	t.Parallel()
	for _, lookup := range []bool{false, true} {
		value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
		if lookup {
			value.consumer.lookupError = errors.New("lookup failed")
		} else {
			value.consumer.deliveryError = errors.New("rejected")
		}
		executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value)
		if _, _, err := executor.Execute(); err == nil {
			t.Fatal("consumer lookup/rejection unexpectedly succeeded")
		}
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifactsAbsent(t, value)
	}

	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	value.consumer.mutate = func(acknowledgement *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement) {
		acknowledgement.Binding.ConsumerContractFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		acknowledgement.AcknowledgementFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementFingerprint(*acknowledgement)
	}
	if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value).Execute(); err == nil {
		t.Fatal("consumer acknowledgement with a conflicting contract was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifactsAbsent(t, value)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReplayRestartConcurrencyAndRecovery(t *testing.T) {
	t.Parallel()
	base := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	t.Run("replay and restart", func(t *testing.T) {
		value := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, base)
		executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value)
		acknowledgement1, receipt1, err := executor.Execute()
		if err != nil {
			t.Fatal(err)
		}
		ackBytes1 := mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t, value.acknowledgementPath)
		receiptBytes1 := mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t, value.receiptPath)
		acknowledgement2, receipt2, err := executor.Execute()
		if err != nil {
			t.Fatal(err)
		}
		acknowledgement3, receipt3, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value).Execute()
		if err != nil || !reflect.DeepEqual(acknowledgement1, acknowledgement2) || !reflect.DeepEqual(acknowledgement1, acknowledgement3) || !reflect.DeepEqual(receipt1, receipt2) || !reflect.DeepEqual(receipt1, receipt3) || !bytes.Equal(ackBytes1, mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t, value.acknowledgementPath)) || !bytes.Equal(receiptBytes1, mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t, value.receiptPath)) || value.consumer.deliveries() != 1 {
			t.Fatal("exact replay/restart changed evidence or reinvoked the consumer")
		}
	})

	t.Run("identical concurrency", func(t *testing.T) {
		value := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, base)
		const workers = 8
		results := make(chan NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt, workers)
		errs := make(chan error, workers)
		var group sync.WaitGroup
		for i := 0; i < workers; i++ {
			group.Add(1)
			go func() {
				defer group.Done()
				executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(value.root, value.expected, value.consumer)
				if err != nil {
					errs <- err
					return
				}
				_, receipt, err := executor.Execute()
				if err != nil {
					errs <- err
					return
				}
				results <- receipt
			}()
		}
		group.Wait()
		close(results)
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		var first *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
		for receipt := range results {
			if first == nil {
				copy := receipt
				first = &copy
			} else if !reflect.DeepEqual(*first, receipt) {
				t.Fatal("identical concurrency did not converge")
			}
		}
		if value.consumer.deliveries() != 1 {
			t.Fatalf("consumer invoked %d times under identical concurrency", value.consumer.deliveries())
		}
	})

	t.Run("consumer accepted before local acknowledgement", func(t *testing.T) {
		value := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, base)
		executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value)
		executor.writeAcknowledgementAtomic = func(string, any) error { return errors.New("injected acknowledgement write failure") }
		if _, _, err := executor.Execute(); err == nil || value.consumer.deliveries() != 1 {
			t.Fatal("consumer acceptance failure point was not reached exactly once")
		}
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifactsAbsent(t, value)
		acknowledgement, _ := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value)
		if value.consumer.deliveries() != 1 || !reflect.DeepEqual(acknowledgement, value.consumer.acknowledgement) {
			t.Fatal("durable consumer lookup did not recover byte-equivalent acknowledgement evidence without reinvocation")
		}
	})

	t.Run("acknowledgement before receipt", func(t *testing.T) {
		value := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, base)
		executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value)
		executor.writeReceiptAtomic = func(string, any) error { return errors.New("injected receipt write failure") }
		if _, _, err := executor.Execute(); err == nil || value.consumer.deliveries() != 1 {
			t.Fatal("acknowledgement-before-receipt failure point was not reached")
		}
		if _, err := os.Lstat(value.acknowledgementPath); err != nil {
			t.Fatal("accepted acknowledgement was not durable")
		}
		if _, err := os.Lstat(value.receiptPath); !os.IsNotExist(err) {
			t.Fatal("failed receipt publication left a receipt")
		}
		mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value)
		if value.consumer.deliveries() != 1 {
			t.Fatal("acknowledgement-before-receipt recovery reinvoked the consumer")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRejectsOrphansTamperingAndUnsafeArtifacts(t *testing.T) {
	t.Parallel()
	base := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs(base.root, base.expected, base.consumer)
	if err != nil {
		t.Fatal(err)
	}
	inputs.acknowledgement = deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(inputs)
	inputs.acknowledgementExists = true
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt(inputs)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, base.receiptPath, receipt)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(base.root, base.expected, base.consumer); err == nil {
		t.Fatal("receipt without its exact acknowledgement was accepted")
	}

	success := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, success)
	acknowledgementRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, success.acknowledgementPath)
	receiptRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, success.receiptPath)
	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"malformed", []byte("{")},
		{"unknown", []byte("{\"unknown\":true}\n")},
		{"trailing", []byte("{}\n{}\n")},
		{"noncanonical", []byte("{}")},
		{"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorMaxBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer mustRestoreNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifacts(t, success, acknowledgementRaw, receiptRaw)
			if err := os.WriteFile(success.acknowledgementPath, test.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(success.root, success.expected, success.consumer); err == nil {
				t.Fatal("malformed, noncanonical, unknown-field, trailing, empty, or oversized acknowledgement was accepted")
			}
		})
	}

	t.Run("tampered acknowledgement", func(t *testing.T) {
		defer mustRestoreNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifacts(t, success, acknowledgementRaw, receiptRaw)
		var acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
		mustDecodeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t, success.acknowledgementPath, &acknowledgement)
		acknowledgement.Binding.GraphRunID = "graph-run-tampered-001"
		acknowledgement.AcknowledgementFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementFingerprint(acknowledgement)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, success.acknowledgementPath, acknowledgement)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(success.root, success.expected, success.consumer); err == nil {
			t.Fatal("tampered acknowledgement was accepted")
		}
	})

	t.Run("tampered receipt", func(t *testing.T) {
		defer mustRestoreNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifacts(t, success, acknowledgementRaw, receiptRaw)
		var changedReceipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
		mustDecodeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t, success.receiptPath, &changedReceipt)
		changedReceipt.ConsumerInvocationCount = 2
		changedReceipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptFingerprint(changedReceipt)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, success.receiptPath, changedReceipt)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(success.root, success.expected, success.consumer); err == nil {
			t.Fatal("tampered receipt was accepted")
		}
	})

	t.Run("symlinked acknowledgement", func(t *testing.T) {
		defer mustRestoreNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifacts(t, success, acknowledgementRaw, receiptRaw)
		target := success.acknowledgementPath + ".target"
		if err := os.Rename(success.acknowledgementPath, target); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, success.acknowledgementPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(success.root, success.expected, success.consumer); err == nil {
			t.Fatal("symlinked acknowledgement was accepted")
		}
	})

	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorCanonicalArtifact(success.root, filepath.Join(success.root, "..", "unsafe-delivery-acknowledgement.json"), &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement{}, true); err == nil {
		t.Fatal("unsafe acknowledgement path was accepted")
	}
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t *testing.T, terminalResult, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture {
	t.Helper()
	template, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestTemplates[terminalResult+"\x00"+route]
	if !ok {
		t.Fatalf("unsupported output-delivery executor test route %q/%q", terminalResult, route)
	}
	template.once.Do(func() {
		value := buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, terminalResult, route)
		template.fixture = *value
		template.fixture.consumer = nil
		template.files = mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	})
	root := t.TempDir()
	for relative, raw := range template.files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	value := template.fixture
	value.root = root
	value.request = cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(value.request)
	value.output = cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(value.output)
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{expected: value.expected, output: value.output, outputReceipt: value.outputReceipt, decision: value.decision, request: value.request}
	value.consumer = &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake{consumerID: value.request.ConsumerID, contractFingerprint: value.request.ConsumerContractFingerprint, operationKey: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(value.request), request: value.request, output: value.output, acknowledgement: deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(inputs)}
	value.acknowledgementPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName)
	value.receiptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)
	return &value
}

func buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t *testing.T, terminalResult, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, terminalResult, "approved", route)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, policy)
	if request == nil {
		t.Fatal("approved delivery policy did not produce a request")
	}
	expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorExpected{Policy: policy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint, PolicyRequestFingerprint: request.RequestFingerprint}
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{expected: expected, output: policy.output, outputReceipt: policy.receipt, decision: decision, request: *request}
	acknowledgement := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(inputs)
	consumer := &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake{consumerID: request.ConsumerID, contractFingerprint: request.ConsumerContractFingerprint, operationKey: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(*request), request: *request, output: policy.output, acknowledgement: acknowledgement}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture{root: policy.root, expected: expected, decision: decision, request: *request, output: policy.output, outputReceipt: policy.receipt, consumer: consumer, acknowledgementPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName), receiptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)}
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture {
	t.Helper()
	root := t.TempDir()
	for relative, raw := range mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	request := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(value.request)
	output := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(value.output)
	inputs := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorInputs{expected: value.expected, output: output, outputReceipt: value.outputReceipt, decision: value.decision, request: request}
	consumer := &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryConsumerFake{consumerID: request.ConsumerID, contractFingerprint: request.ConsumerContractFingerprint, operationKey: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryOperationKey(request), request: request, output: output, acknowledgement: deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement(inputs)}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture{root: root, expected: value.expected, decision: value.decision, request: request, output: output, outputReceipt: value.outputReceipt, consumer: consumer, acknowledgementPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName), receiptPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)}
}

func mustRestoreNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorPredecessorArtifacts(t *testing.T, decisionPath, requestPath, outputPath, outputReceiptPath string, decisionRaw, requestRaw, outputRaw, outputReceiptRaw []byte) {
	t.Helper()
	for _, artifact := range []struct {
		path string
		raw  []byte
	}{
		{decisionPath, decisionRaw},
		{requestPath, requestRaw},
		{outputPath, outputRaw},
		{outputReceiptPath, outputReceiptRaw},
	} {
		if err := os.WriteFile(artifact.path, artifact.raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func mustRestoreNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifacts(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture, acknowledgementRaw, receiptRaw []byte) {
	t.Helper()
	for _, artifact := range []struct {
		path string
		raw  []byte
	}{
		{value.acknowledgementPath, acknowledgementRaw},
		{value.receiptPath, receiptRaw},
	} {
		if err := os.Remove(artifact.path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.WriteFile(artifact.path, artifact.raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(value.root, value.expected, value.consumer)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt) {
	t.Helper()
	acknowledgement, receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return acknowledgement, receipt
}

func rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorRequest(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
	t.Helper()
	value.request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequestFingerprint(value.request)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName), value.request)
}

func rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorOutputReceipt(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
	t.Helper()
	value.outputReceipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptFingerprint(value.outputReceipt)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName), value.outputReceipt)
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifactsAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture) {
	t.Helper()
	for _, path := range []string{value.acknowledgementPath, value.receiptPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed delivery executor unexpectedly published %s", path)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture, before map[string][]byte) {
	t.Helper()
	after := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	changed := make([]string, 0, 2)
	for path, raw := range after {
		old, existed := before[path]
		if !existed || !bytes.Equal(old, raw) {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, exists := after[path]; !exists {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("delivery executor changed a predecessor or adjacent artifact: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorNoAuthority(t *testing.T, acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt) {
	t.Helper()
	if acknowledgement.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority{}) || receipt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAuthority{}) {
		t.Fatal("acknowledgement or receipt granted adjacent authority")
	}
	raw, err := json.Marshal(struct {
		Acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
		Receipt         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
	}{acknowledgement, receipt})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"lifecycle_advancement":true`, `"graph_mutation":true`, `"dependency_release":true`, `"failure_propagation":true`, `"candidate_discovery":true`, `"candidate_selection":true`, `"scheduling":true`, `"execution":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"callback":true`, `"publication":true`, `"provider":true`, `"connector":true`, `"broker":true`, `"forgepipe":true`, `"process":true`, `"network":true`, `"remote_execution":true`, `"validation":true`, `"checkout_mutation":true`, `"git":true`, `"checkpoint":true`, `"commit":true`, `"push":true`, `"external_action":true`, `"consumer_reinvoked":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("delivery executor performed or granted forbidden activity: %s", forbidden)
		}
	}
}

func mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustDecodeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t *testing.T, path string, target any) {
	t.Helper()
	if err := json.Unmarshal(mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorArtifact(t, path), target); err != nil {
		t.Fatal(err)
	}
}

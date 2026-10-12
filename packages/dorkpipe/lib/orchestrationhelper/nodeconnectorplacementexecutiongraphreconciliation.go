package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const (
	NodeConnectorPlacementExecutionGraphReconciliationSchema = "dorkpipe.node-placement-execution-graph-reconciliation/v1"

	nodeConnectorPlacementExecutionGraphReconciliationProvenance       = "fixture_only_local_task_graph_reconciliation"
	nodeConnectorPlacementExecutionGraphReconciliationName             = "node-placement-execution-graph-reconciliation.json"
	nodeConnectorPlacementExecutionGraphReconciliationMaxArtifactBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphReconciliationWriteAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphReconciliationLocks       sync.Map
)

type NodeConnectorPlacementExecutionGraphReconciliationExpected struct {
	Reconciliation                    NodeConnectorPlacementExecutionReconciliationExpected `json:"reconciliation"`
	ReconciliationDecisionFingerprint string                                                `json:"reconciliation_decision_fingerprint"`
	ReconciliationRequestFingerprint  string                                                `json:"reconciliation_request_fingerprint"`
}

// NodeConnectorPlacementExecutionGraphReconciliationAuthority is intentionally
// all-negative. Consuming the exact request authorizes only this local durable
// task-level interpretation; the resulting evidence grants no adjacent action.
type NodeConnectorPlacementExecutionGraphReconciliationAuthority struct {
	GraphCompletion bool `json:"graph_completion"`
	GraphFailure    bool `json:"graph_failure"`
	NextTask        bool `json:"next_task"`
	Execution       bool `json:"execution"`
	Connector       bool `json:"connector"`
	Validation      bool `json:"validation"`
	Executor        bool `json:"executor"`
	Broker          bool `json:"broker"`
	Dispatch        bool `json:"dispatch"`
	Lease           bool `json:"lease"`
	Cancellation    bool `json:"cancellation"`
	Retry           bool `json:"retry"`
	Repair          bool `json:"repair"`
	Quarantine      bool `json:"quarantine"`
	Service         bool `json:"service"`
	Network         bool `json:"network"`
	Provider        bool `json:"provider"`
	Mutation        bool `json:"mutation"`
	Git             bool `json:"git"`
	Apply           bool `json:"apply"`
	Checkpoint      bool `json:"checkpoint"`
	Commit          bool `json:"commit"`
	Push            bool `json:"push"`
	Publication     bool `json:"publication"`
	Lifecycle       bool `json:"lifecycle"`
}

type NodeConnectorPlacementExecutionGraphReconciliation struct {
	Schema                            string                                                      `json:"schema"`
	ReconciliationRequest             NodeConnectorPlacementExecutionReconciliationRequest        `json:"reconciliation_request"`
	ReconciliationRequestID           string                                                      `json:"reconciliation_request_id"`
	ReconciliationRequestFingerprint  string                                                      `json:"reconciliation_request_fingerprint"`
	ReconciliationDecisionID          string                                                      `json:"reconciliation_decision_id"`
	ReconciliationDecisionFingerprint string                                                      `json:"reconciliation_decision_fingerprint"`
	DeliveryID                        string                                                      `json:"delivery_id"`
	DeliveryFingerprint               string                                                      `json:"delivery_fingerprint"`
	GraphRunID                        string                                                      `json:"graph_run_id"`
	RunID                             string                                                      `json:"run_id"`
	TaskID                            string                                                      `json:"task_id"`
	OperationID                       string                                                      `json:"operation_id"`
	Attempt                           int                                                         `json:"attempt"`
	ExecutionRequestFingerprint       string                                                      `json:"execution_request_fingerprint"`
	LeaseID                           string                                                      `json:"lease_id"`
	LeaseFingerprint                  string                                                      `json:"lease_fingerprint"`
	EventStreamFingerprint            string                                                      `json:"event_stream_fingerprint"`
	ReceiptID                         string                                                      `json:"receipt_id"`
	ReceiptFingerprint                string                                                      `json:"receipt_fingerprint"`
	ArtifactManifestFingerprint       string                                                      `json:"artifact_manifest_fingerprint"`
	TerminalResult                    string                                                      `json:"terminal_result"`
	CleanupStatus                     string                                                      `json:"cleanup_status"`
	CleanupEvidenceDigest             string                                                      `json:"cleanup_evidence_digest,omitempty"`
	TaskOutcome                       string                                                      `json:"task_outcome"`
	CompleteChainRevalidated          bool                                                        `json:"complete_chain_revalidated"`
	AuthorizationConsumed             bool                                                        `json:"authorization_consumed"`
	TerminalOutcomeInterpreted        bool                                                        `json:"terminal_outcome_interpreted"`
	ReceiptAuthoritative              bool                                                        `json:"receipt_authoritative"`
	EventsAuthoritative               bool                                                        `json:"events_authoritative"`
	ProviderEvidenceAuthoritative     bool                                                        `json:"provider_evidence_authoritative"`
	GraphReconciliationPerformed      bool                                                        `json:"graph_reconciliation_performed"`
	GraphCompletionClaimed            bool                                                        `json:"graph_completion_claimed"`
	GraphFailurePropagated            bool                                                        `json:"graph_failure_propagated"`
	NextTaskScheduled                 bool                                                        `json:"next_task_scheduled"`
	ExecutionOrLifecycleSideEffects   bool                                                        `json:"execution_or_lifecycle_side_effects"`
	ConnectorInvoked                  bool                                                        `json:"connector_invoked"`
	PreparedValidationInvoked         bool                                                        `json:"prepared_validation_invoked"`
	BrokerExecutorInvoked             bool                                                        `json:"broker_executor_invoked"`
	BrokerOperationCreated            bool                                                        `json:"broker_operation_created"`
	LeaseCreated                      bool                                                        `json:"lease_created"`
	AttemptCreated                    bool                                                        `json:"attempt_created"`
	ConnectionCreated                 bool                                                        `json:"connection_created"`
	SessionCreated                    bool                                                        `json:"session_created"`
	EnrollmentCreated                 bool                                                        `json:"enrollment_created"`
	CredentialCreated                 bool                                                        `json:"credential_created"`
	EventCreated                      bool                                                        `json:"event_created"`
	ReceiptCreated                    bool                                                        `json:"receipt_created"`
	DeliveryCreated                   bool                                                        `json:"delivery_created"`
	Provenance                        string                                                      `json:"provenance"`
	FixtureOwned                      bool                                                        `json:"fixture_owned"`
	Authority                         NodeConnectorPlacementExecutionGraphReconciliationAuthority `json:"authority"`
	ArtifactFingerprint               string                                                      `json:"artifact_fingerprint"`
}

type nodeConnectorPlacementExecutionGraphReconciliationInputs struct {
	reconciliation NodeConnectorPlacementExecutionReconciliationExpected
	chain          nodeConnectorPlacementExecutionReconciliationInputs
	decision       NodeConnectorPlacementExecutionReconciliationDecision
	request        NodeConnectorPlacementExecutionReconciliationRequest
}

type NodeConnectorPlacementExecutionGraphReconciliations struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphReconciliationExpected
	broker   *NodeExecutionFakeBroker
	inputs   nodeConnectorPlacementExecutionGraphReconciliationInputs
	artifact *NodeConnectorPlacementExecutionGraphReconciliation
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphReconciliations(root string, expected NodeConnectorPlacementExecutionGraphReconciliationExpected, broker *NodeExecutionFakeBroker) (*NodeConnectorPlacementExecutionGraphReconciliations, error) {
	normalized, err := normalizeNodeConnectorPlacementExecutionGraphReconciliationExpected(expected)
	if err != nil {
		return nil, err
	}
	inputs, err := loadNodeConnectorPlacementExecutionGraphReconciliationInputs(root, normalized, broker)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphReconciliations{root: root, expected: normalized, broker: broker, inputs: inputs}
	artifact, exists, err := loadNodeConnectorPlacementExecutionGraphReconciliation(root, inputs)
	if err != nil {
		return nil, err
	}
	if exists {
		value.artifact = &artifact
	}
	return value, nil
}

func (reconciliations *NodeConnectorPlacementExecutionGraphReconciliations) Reconcile() (NodeConnectorPlacementExecutionGraphReconciliation, error) {
	reconciliations.mu.Lock()
	defer reconciliations.mu.Unlock()
	path := filepath.Join(reconciliations.root, nodeConnectorPlacementExecutionGraphReconciliationName)
	pathLock, _ := nodeConnectorPlacementExecutionGraphReconciliationLocks.LoadOrStore(path, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	inputs, err := loadNodeConnectorPlacementExecutionGraphReconciliationInputs(reconciliations.root, reconciliations.expected, reconciliations.broker)
	if err != nil || !nodeExecutionEqual(inputs, reconciliations.inputs) {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, errors.New("graph reconciliation could not directly revalidate the complete immutable authorized chain")
	}
	derived, err := deriveNodeConnectorPlacementExecutionGraphReconciliation(inputs)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, err
	}
	existing, exists, err := loadNodeConnectorPlacementExecutionGraphReconciliation(reconciliations.root, inputs)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, err
	}
	if exists {
		if !nodeExecutionEqual(existing, derived) {
			return NodeConnectorPlacementExecutionGraphReconciliation{}, errors.New("conflicting graph reconciliation replay is rejected")
		}
		reconciliations.artifact = &existing
		return cloneNodeConnectorPlacementExecutionGraphReconciliation(existing), nil
	}
	if _, err := os.Lstat(path); err == nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, errors.New("conflicting graph reconciliation artifact already exists")
	} else if !os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, errors.New("graph reconciliation artifact path cannot be inspected")
	}
	if err := nodeConnectorPlacementExecutionGraphReconciliationWriteAtomic(path, derived); err != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, errors.New("graph reconciliation artifact could not be published")
	}
	persisted, exists, err := loadNodeConnectorPlacementExecutionGraphReconciliation(reconciliations.root, inputs)
	if err != nil || !exists || !nodeExecutionEqual(persisted, derived) {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, errors.New("graph reconciliation artifact publication could not be verified")
	}
	reconciliations.artifact = &persisted
	return cloneNodeConnectorPlacementExecutionGraphReconciliation(persisted), nil
}

func (reconciliations *NodeConnectorPlacementExecutionGraphReconciliations) Artifact() *NodeConnectorPlacementExecutionGraphReconciliation {
	reconciliations.mu.Lock()
	defer reconciliations.mu.Unlock()
	if reconciliations.artifact == nil {
		return nil
	}
	cloned := cloneNodeConnectorPlacementExecutionGraphReconciliation(*reconciliations.artifact)
	return &cloned
}

func normalizeNodeConnectorPlacementExecutionGraphReconciliationExpected(value NodeConnectorPlacementExecutionGraphReconciliationExpected) (NodeConnectorPlacementExecutionGraphReconciliationExpected, error) {
	reconciliation, err := normalizeNodeConnectorPlacementExecutionReconciliationExpected(value.Reconciliation)
	if err != nil || !nodeExecutionFingerprint.MatchString(value.ReconciliationDecisionFingerprint) || !nodeExecutionFingerprint.MatchString(value.ReconciliationRequestFingerprint) {
		return NodeConnectorPlacementExecutionGraphReconciliationExpected{}, errors.New("graph reconciliation expected binding is invalid")
	}
	value.Reconciliation = reconciliation
	return value, nil
}

func loadNodeConnectorPlacementExecutionGraphReconciliationInputs(root string, expected NodeConnectorPlacementExecutionGraphReconciliationExpected, broker *NodeExecutionFakeBroker) (nodeConnectorPlacementExecutionGraphReconciliationInputs, error) {
	chain, err := loadNodeConnectorPlacementExecutionReconciliationInputs(root, expected.Reconciliation, broker)
	if err != nil {
		return nodeConnectorPlacementExecutionGraphReconciliationInputs{}, errors.New("graph reconciliation could not revalidate dispatch, handoff, broker, delivery, events, and receipt evidence")
	}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionReconciliationDecision(root, chain)
	if err != nil || !decisionExists || decision.Decision != "approved" || decision.DecisionFingerprint != expected.ReconciliationDecisionFingerprint {
		return nodeConnectorPlacementExecutionGraphReconciliationInputs{}, errors.New("graph reconciliation requires the exact approved reconciliation decision")
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionReconciliationRequest(root, chain, decision, decisionExists)
	wantAuthority := NodeConnectorPlacementExecutionReconciliationAuthority{LocalGraphReconciliationRequest: true}
	if err != nil || !requestExists || request.RequestFingerprint != expected.ReconciliationRequestFingerprint || request.Authority != wantAuthority || !request.OneTimeRequest || request.AuthorizationConsumed || !request.TerminalOutcomeOpaque || request.TerminalOutcomeInterpreted || request.GraphReconciliationPerformed {
		return nodeConnectorPlacementExecutionGraphReconciliationInputs{}, errors.New("graph reconciliation requires the exact approved unconsumed one-time local-only request")
	}
	return nodeConnectorPlacementExecutionGraphReconciliationInputs{reconciliation: expected.Reconciliation, chain: chain, decision: decision, request: request}, nil
}

func deriveNodeConnectorPlacementExecutionGraphReconciliation(inputs nodeConnectorPlacementExecutionGraphReconciliationInputs) (NodeConnectorPlacementExecutionGraphReconciliation, error) {
	delivery, receipt := inputs.chain.delivery, inputs.chain.delivery.Receipt
	outcome, err := nodeConnectorPlacementExecutionTaskOutcome(receipt)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, err
	}
	leaseFingerprint, err := nodeExecutionFingerprintValue(inputs.chain.request.TaskLease)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, err
	}
	eventsFingerprint, err := nodeExecutionFingerprintValue(delivery.Events)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, err
	}
	artifact := NodeConnectorPlacementExecutionGraphReconciliation{
		Schema:                  NodeConnectorPlacementExecutionGraphReconciliationSchema,
		ReconciliationRequest:   cloneNodeConnectorPlacementExecutionReconciliationRequest(inputs.request),
		ReconciliationRequestID: inputs.request.RequestID, ReconciliationRequestFingerprint: inputs.request.RequestFingerprint,
		ReconciliationDecisionID: inputs.decision.DecisionID, ReconciliationDecisionFingerprint: inputs.decision.DecisionFingerprint,
		DeliveryID: delivery.DeliveryID, DeliveryFingerprint: delivery.DeliveryFingerprint,
		GraphRunID: delivery.ExecutionRequest.GraphRunID, RunID: delivery.ExecutionRequest.RunID, TaskID: delivery.ExecutionRequest.TaskID,
		OperationID: delivery.ExecutionRequest.OperationID, Attempt: delivery.TaskLease.Attempt,
		ExecutionRequestFingerprint: delivery.ExecutionRequestFingerprint, LeaseID: delivery.TaskLease.LeaseID, LeaseFingerprint: leaseFingerprint,
		EventStreamFingerprint: eventsFingerprint, ReceiptID: receipt.ReceiptID, ReceiptFingerprint: receipt.ReceiptFingerprint,
		ArtifactManifestFingerprint: receipt.Artifacts.ManifestFingerprint, TerminalResult: receipt.Result,
		CleanupStatus: receipt.Cleanup.Status, CleanupEvidenceDigest: receipt.Cleanup.EvidenceDigest, TaskOutcome: outcome,
		CompleteChainRevalidated: true, AuthorizationConsumed: true, TerminalOutcomeInterpreted: true, ReceiptAuthoritative: true,
		GraphReconciliationPerformed: true, Provenance: nodeConnectorPlacementExecutionGraphReconciliationProvenance, FixtureOwned: true,
	}
	fingerprint, err := nodeConnectorPlacementExecutionGraphReconciliationFingerprint(artifact)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, err
	}
	artifact.ArtifactFingerprint = fingerprint
	if err := validateNodeConnectorPlacementExecutionGraphReconciliation(artifact, inputs); err != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, err
	}
	return artifact, nil
}

func nodeConnectorPlacementExecutionTaskOutcome(receipt NodeExecutionReceipt) (string, error) {
	if err := validateNodeExecutionReceiptShape(receipt); err != nil {
		return "", errors.New("graph reconciliation requires a valid immutable execution receipt")
	}
	if receipt.Result == "succeeded" && receipt.Cleanup.Status == "not_required" {
		return "passed", nil
	}
	if receipt.Result == "failed" || receipt.Result == "degraded" || receipt.Result == "cancelled" {
		return "failed", nil
	}
	return "", errors.New("graph reconciliation cannot interpret the terminal receipt")
}

func validateNodeConnectorPlacementExecutionGraphReconciliation(value NodeConnectorPlacementExecutionGraphReconciliation, inputs nodeConnectorPlacementExecutionGraphReconciliationInputs) error {
	derivedRequest := inputs.request
	if value.Schema != NodeConnectorPlacementExecutionGraphReconciliationSchema || value.Provenance != nodeConnectorPlacementExecutionGraphReconciliationProvenance || !value.FixtureOwned || !value.CompleteChainRevalidated || !value.AuthorizationConsumed || !value.TerminalOutcomeInterpreted || !value.ReceiptAuthoritative || value.EventsAuthoritative || value.ProviderEvidenceAuthoritative || !value.GraphReconciliationPerformed || value.GraphCompletionClaimed || value.GraphFailurePropagated || value.NextTaskScheduled || value.ExecutionOrLifecycleSideEffects || value.Authority != (NodeConnectorPlacementExecutionGraphReconciliationAuthority{}) || !nodeExecutionEqual(value.ReconciliationRequest, derivedRequest) {
		return errors.New("graph reconciliation contract, authority, consumption, or side-effect boundary is invalid")
	}
	delivery, receipt := inputs.chain.delivery, inputs.chain.delivery.Receipt
	leaseFingerprint, leaseErr := nodeExecutionFingerprintValue(inputs.chain.request.TaskLease)
	eventsFingerprint, eventsErr := nodeExecutionFingerprintValue(delivery.Events)
	outcome, outcomeErr := nodeConnectorPlacementExecutionTaskOutcome(receipt)
	if leaseErr != nil || eventsErr != nil || outcomeErr != nil || value.ReconciliationRequestID != derivedRequest.RequestID || value.ReconciliationRequestFingerprint != derivedRequest.RequestFingerprint || value.ReconciliationDecisionID != inputs.decision.DecisionID || value.ReconciliationDecisionFingerprint != inputs.decision.DecisionFingerprint || value.DeliveryID != delivery.DeliveryID || value.DeliveryFingerprint != delivery.DeliveryFingerprint || value.GraphRunID != delivery.ExecutionRequest.GraphRunID || value.RunID != delivery.ExecutionRequest.RunID || value.TaskID != delivery.ExecutionRequest.TaskID || value.OperationID != delivery.ExecutionRequest.OperationID || value.Attempt != delivery.TaskLease.Attempt || value.ExecutionRequestFingerprint != delivery.ExecutionRequestFingerprint || value.LeaseID != delivery.TaskLease.LeaseID || value.LeaseFingerprint != leaseFingerprint || value.EventStreamFingerprint != eventsFingerprint || value.ReceiptID != receipt.ReceiptID || value.ReceiptFingerprint != receipt.ReceiptFingerprint || value.ArtifactManifestFingerprint != receipt.Artifacts.ManifestFingerprint || value.TerminalResult != receipt.Result || value.CleanupStatus != receipt.Cleanup.Status || value.CleanupEvidenceDigest != receipt.Cleanup.EvidenceDigest || value.TaskOutcome != outcome {
		return errors.New("graph reconciliation immutable identity, fingerprint, receipt, or task-outcome binding is invalid")
	}
	if value.ConnectorInvoked || value.PreparedValidationInvoked || value.BrokerExecutorInvoked || value.BrokerOperationCreated || value.LeaseCreated || value.AttemptCreated || value.ConnectionCreated || value.SessionCreated || value.EnrollmentCreated || value.CredentialCreated || value.EventCreated || value.ReceiptCreated || value.DeliveryCreated {
		return errors.New("graph reconciliation cannot claim an execution or lifecycle side effect")
	}
	fingerprint, err := nodeConnectorPlacementExecutionGraphReconciliationFingerprint(value)
	if err != nil || fingerprint != value.ArtifactFingerprint {
		return errors.New("graph reconciliation artifact fingerprint is invalid")
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > nodeConnectorPlacementExecutionGraphReconciliationMaxArtifactBytes {
		return errors.New("graph reconciliation artifact exceeds its encoded bound")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphReconciliation(root string, inputs nodeConnectorPlacementExecutionGraphReconciliationInputs) (NodeConnectorPlacementExecutionGraphReconciliation, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionGraphReconciliationName))
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphReconciliationMaxArtifactBytes {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, false, errors.New("durable graph reconciliation artifact cannot be read within its bound")
	}
	var artifact NodeConnectorPlacementExecutionGraphReconciliation
	if decodeNodeExecutionStrict(raw, &artifact) != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, false, errors.New("durable graph reconciliation artifact is malformed, trailing, or contains unknown fields")
	}
	canonical, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || validateNodeConnectorPlacementExecutionGraphReconciliation(artifact, inputs) != nil {
		return NodeConnectorPlacementExecutionGraphReconciliation{}, false, errors.New("durable graph reconciliation artifact is noncanonical, tampered, conflicting, or orphaned")
	}
	return artifact, true, nil
}

func nodeConnectorPlacementExecutionGraphReconciliationFingerprint(value NodeConnectorPlacementExecutionGraphReconciliation) (string, error) {
	value.ArtifactFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}

func cloneNodeConnectorPlacementExecutionGraphReconciliation(value NodeConnectorPlacementExecutionGraphReconciliation) NodeConnectorPlacementExecutionGraphReconciliation {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphReconciliation
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

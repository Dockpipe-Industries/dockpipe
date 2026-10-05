package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
)

const (
	NodeConnectorPlacementExecutionGraphFinalizationDecisionFixtureSchema = "dorkpipe.node-placement-execution-graph-finalization-decision-fixture/v1"
	NodeConnectorPlacementExecutionGraphFinalizationDecisionSchema        = "dorkpipe.node-placement-execution-graph-finalization-decision/v1"
	NodeConnectorPlacementExecutionGraphFinalizationRequestSchema         = "dorkpipe.node-placement-execution-graph-finalization-request/v1"

	nodeConnectorPlacementExecutionGraphFinalizationDecisionName     = "node-placement-execution-graph-finalization-decision.json"
	nodeConnectorPlacementExecutionGraphFinalizationRequestName      = "node-placement-execution-graph-finalization-request.json"
	nodeConnectorPlacementExecutionGraphFinalizationDecisionMaxBytes = 4 << 20
	nodeConnectorPlacementExecutionGraphFinalizationArtifactMaxBytes = 8 << 20
)

var (
	nodeConnectorPlacementExecutionGraphFinalizationID                  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	nodeConnectorPlacementExecutionGraphFinalizationWriteDecisionAtomic = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphFinalizationWriteRequestAtomic  = writeJSONFileAtomic
	nodeConnectorPlacementExecutionGraphFinalizationLocks               sync.Map
)

// NodeConnectorPlacementExecutionGraphFinalizationAuthority deliberately
// grants only a later local consumer permission to examine this explicit
// decision. It never grants graph lifecycle or execution authority.
type NodeConnectorPlacementExecutionGraphFinalizationAuthority struct {
	LocalGraphFinalization bool `json:"local_graph_finalization"`
	GraphCompletion        bool `json:"graph_completion"`
	GraphFailure           bool `json:"graph_failure"`
	DependencyRelease      bool `json:"dependency_release"`
	NextTask               bool `json:"next_task"`
	Retry                  bool `json:"retry"`
	Repair                 bool `json:"repair"`
	Cancellation           bool `json:"cancellation"`
	Execution              bool `json:"execution"`
	Broker                 bool `json:"broker"`
	Provider               bool `json:"provider"`
	Validation             bool `json:"validation"`
	Mutation               bool `json:"mutation"`
	Git                    bool `json:"git"`
	Publication            bool `json:"publication"`
	Lifecycle              bool `json:"lifecycle"`
}

type NodeConnectorPlacementExecutionGraphFinalizationExpected struct {
	GraphRunID string                                               `json:"graph_run_id"`
	Outcomes   []NodeConnectorPlacementExecutionGraphReconciliation `json:"outcomes"`
}

// NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture is the
// only approval source. Outcomes are prior durable canonical task outcomes;
// provider, event, connection, machine, capability, and lease evidence is not
// an input to this boundary.
type NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture struct {
	Schema         string                                               `json:"schema"`
	DecisionID     string                                               `json:"decision_id"`
	ReplayIdentity string                                               `json:"replay_identity"`
	Decision       string                                               `json:"decision"`
	Finalization   string                                               `json:"finalization,omitempty"`
	GraphRunID     string                                               `json:"graph_run_id"`
	Outcomes       []NodeConnectorPlacementExecutionGraphReconciliation `json:"outcomes"`
	RequestID      string                                               `json:"request_id,omitempty"`
	Provenance     string                                               `json:"provenance"`
}

type NodeConnectorPlacementExecutionGraphFinalizationDecision struct {
	Schema              string                                                    `json:"schema"`
	DecisionID          string                                                    `json:"decision_id"`
	ReplayIdentity      string                                                    `json:"replay_identity"`
	Decision            string                                                    `json:"decision"`
	Finalization        string                                                    `json:"finalization,omitempty"`
	GraphRunID          string                                                    `json:"graph_run_id"`
	Outcomes            []NodeConnectorPlacementExecutionGraphReconciliation      `json:"outcomes"`
	OutcomesFingerprint string                                                    `json:"outcomes_fingerprint"`
	ApprovalInferred    bool                                                      `json:"approval_inferred"`
	FixtureOwned        bool                                                      `json:"fixture_owned"`
	Authority           NodeConnectorPlacementExecutionGraphFinalizationAuthority `json:"authority"`
	DecisionFingerprint string                                                    `json:"decision_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphFinalizationRequest struct {
	Schema                string                                                    `json:"schema"`
	RequestID             string                                                    `json:"request_id"`
	DecisionID            string                                                    `json:"decision_id"`
	DecisionFingerprint   string                                                    `json:"decision_fingerprint"`
	Finalization          string                                                    `json:"finalization"`
	GraphRunID            string                                                    `json:"graph_run_id"`
	OutcomesFingerprint   string                                                    `json:"outcomes_fingerprint"`
	OneTimeRequest        bool                                                      `json:"one_time_request"`
	AuthorizationConsumed bool                                                      `json:"authorization_consumed"`
	FixtureOwned          bool                                                      `json:"fixture_owned"`
	Authority             NodeConnectorPlacementExecutionGraphFinalizationAuthority `json:"authority"`
	RequestFingerprint    string                                                    `json:"request_fingerprint"`
}

type NodeConnectorPlacementExecutionGraphFinalizations struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphFinalizationExpected
	decision *NodeConnectorPlacementExecutionGraphFinalizationDecision
	request  *NodeConnectorPlacementExecutionGraphFinalizationRequest
	mu       sync.Mutex
}

func OpenNodeConnectorPlacementExecutionGraphFinalizations(root string, expected NodeConnectorPlacementExecutionGraphFinalizationExpected) (*NodeConnectorPlacementExecutionGraphFinalizations, error) {
	normalized, err := normalizeNodeConnectorPlacementExecutionGraphFinalizationExpected(expected)
	if err != nil {
		return nil, err
	}
	value := &NodeConnectorPlacementExecutionGraphFinalizations{root: root, expected: normalized}
	decision, decisionExists, err := loadNodeConnectorPlacementExecutionGraphFinalizationDecision(root, normalized)
	if err != nil {
		return nil, err
	}
	request, requestExists, err := loadNodeConnectorPlacementExecutionGraphFinalizationRequest(root, normalized, decision, decisionExists)
	if err != nil || (requestExists && !decisionExists) {
		return nil, errors.New("graph finalization request is orphaned or invalid")
	}
	if decisionExists {
		value.decision = &decision
	}
	if requestExists {
		value.request = &request
	}
	return value, nil
}

func (finalizations *NodeConnectorPlacementExecutionGraphFinalizations) Decide(raw []byte) (NodeConnectorPlacementExecutionGraphFinalizationDecision, *NodeConnectorPlacementExecutionGraphFinalizationRequest, error) {
	finalizations.mu.Lock()
	defer finalizations.mu.Unlock()
	if len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphFinalizationDecisionMaxBytes {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("graph finalization fixture exceeds its encoded bound")
	}
	var fixture NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture
	if decodeNodeExecutionCanonical(raw, &fixture) != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("graph finalization fixture is not strict canonical JSON")
	}
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphFinalization(finalizations.expected, fixture)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, err
	}
	pathLock, _ := nodeConnectorPlacementExecutionGraphFinalizationLocks.LoadOrStore(finalizations.root, &sync.Mutex{})
	lock := pathLock.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if finalizations.decision != nil {
		if !nodeExecutionEqual(*finalizations.decision, decision) {
			return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("changed or conflicting graph finalization decision replay is rejected")
		}
	} else {
		if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(filepath.Join(finalizations.root, nodeConnectorPlacementExecutionGraphFinalizationDecisionName), "graph finalization decision"); err != nil {
			return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, err
		}
		if err := nodeConnectorPlacementExecutionGraphFinalizationWriteDecisionAtomic(filepath.Join(finalizations.root, nodeConnectorPlacementExecutionGraphFinalizationDecisionName), decision); err != nil {
			return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("graph finalization decision could not be published")
		}
		finalizations.decision = &decision
	}
	if request == nil {
		if finalizations.request != nil {
			return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("rejected graph finalization conflicts with a durable request")
		}
		return cloneNodeConnectorPlacementExecutionGraphFinalizationDecision(*finalizations.decision), nil, nil
	}
	if finalizations.request != nil {
		if !nodeExecutionEqual(*finalizations.request, *request) {
			return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("changed or conflicting graph finalization request replay is rejected")
		}
		cloned := cloneNodeConnectorPlacementExecutionGraphFinalizationRequest(*finalizations.request)
		return cloneNodeConnectorPlacementExecutionGraphFinalizationDecision(*finalizations.decision), &cloned, nil
	}
	if err := requireNodeConnectorPlacementExecutionReconciliationArtifactAbsent(filepath.Join(finalizations.root, nodeConnectorPlacementExecutionGraphFinalizationRequestName), "graph finalization request"); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, err
	}
	if err := nodeConnectorPlacementExecutionGraphFinalizationWriteRequestAtomic(filepath.Join(finalizations.root, nodeConnectorPlacementExecutionGraphFinalizationRequestName), *request); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("graph finalization request could not be published")
	}
	finalizations.request = request
	cloned := cloneNodeConnectorPlacementExecutionGraphFinalizationRequest(*request)
	return cloneNodeConnectorPlacementExecutionGraphFinalizationDecision(*finalizations.decision), &cloned, nil
}

func normalizeNodeConnectorPlacementExecutionGraphFinalizationExpected(value NodeConnectorPlacementExecutionGraphFinalizationExpected) (NodeConnectorPlacementExecutionGraphFinalizationExpected, error) {
	if !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(value.GraphRunID) || len(value.Outcomes) == 0 {
		return NodeConnectorPlacementExecutionGraphFinalizationExpected{}, errors.New("graph finalization requires nonempty exact graph outcomes")
	}
	value.Outcomes = cloneNodeConnectorPlacementExecutionGraphFinalizationOutcomes(value.Outcomes)
	if err := validateNodeConnectorPlacementExecutionGraphFinalizationOutcomes(value.GraphRunID, value.Outcomes); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationExpected{}, err
	}
	return value, nil
}

func deriveNodeConnectorPlacementExecutionGraphFinalization(expected NodeConnectorPlacementExecutionGraphFinalizationExpected, fixture NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture) (NodeConnectorPlacementExecutionGraphFinalizationDecision, *NodeConnectorPlacementExecutionGraphFinalizationRequest, error) {
	if fixture.Schema != NodeConnectorPlacementExecutionGraphFinalizationDecisionFixtureSchema || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.DecisionID) || !nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.ReplayIdentity) || fixture.DecisionID == fixture.ReplayIdentity || fixture.GraphRunID != expected.GraphRunID || fixture.Provenance != "fixture_only_local_graph_finalization_decision" || !nodeExecutionEqual(fixture.Outcomes, expected.Outcomes) {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("graph finalization fixture identity or immutable outcome binding is invalid")
	}
	outcomesFingerprint, err := nodeExecutionFingerprintValue(expected.Outcomes)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, err
	}
	if fixture.Decision != "approved" && fixture.Decision != "rejected" {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("graph finalization decision is invalid")
	}
	if fixture.Decision == "rejected" && (fixture.Finalization != "" || fixture.RequestID != "") {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("rejected graph finalization cannot name a result or request")
	}
	if fixture.Decision == "approved" && (!nodeConnectorPlacementExecutionGraphFinalizationID.MatchString(fixture.RequestID) || (fixture.Finalization != "succeeded" && fixture.Finalization != "failed") || fixture.Finalization != nodeConnectorPlacementExecutionGraphFinalizationOutcome(expected.Outcomes)) {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, errors.New("approved graph finalization must explicitly match durable terminal outcomes")
	}
	decision := NodeConnectorPlacementExecutionGraphFinalizationDecision{Schema: NodeConnectorPlacementExecutionGraphFinalizationDecisionSchema, DecisionID: fixture.DecisionID, ReplayIdentity: fixture.ReplayIdentity, Decision: fixture.Decision, Finalization: fixture.Finalization, GraphRunID: expected.GraphRunID, Outcomes: cloneNodeConnectorPlacementExecutionGraphFinalizationOutcomes(expected.Outcomes), OutcomesFingerprint: outcomesFingerprint, FixtureOwned: true}
	fingerprint, err := nodeConnectorPlacementExecutionGraphFinalizationDecisionFingerprint(decision)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, err
	}
	decision.DecisionFingerprint = fingerprint
	if fixture.Decision == "rejected" {
		return decision, nil, validateNodeConnectorPlacementExecutionGraphFinalizationDecision(decision, expected)
	}
	request := &NodeConnectorPlacementExecutionGraphFinalizationRequest{Schema: NodeConnectorPlacementExecutionGraphFinalizationRequestSchema, RequestID: fixture.RequestID, DecisionID: decision.DecisionID, DecisionFingerprint: decision.DecisionFingerprint, Finalization: fixture.Finalization, GraphRunID: expected.GraphRunID, OutcomesFingerprint: outcomesFingerprint, OneTimeRequest: true, FixtureOwned: true, Authority: NodeConnectorPlacementExecutionGraphFinalizationAuthority{LocalGraphFinalization: true}}
	requestFingerprint, err := nodeConnectorPlacementExecutionGraphFinalizationRequestFingerprint(*request)
	if err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, err
	}
	request.RequestFingerprint = requestFingerprint
	if err := validateNodeConnectorPlacementExecutionGraphFinalizationDecision(decision, expected); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, err
	}
	if err := validateNodeConnectorPlacementExecutionGraphFinalizationRequest(*request, expected, decision); err != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, nil, err
	}
	return decision, request, nil
}

func validateNodeConnectorPlacementExecutionGraphFinalizationOutcomes(graphRunID string, outcomes []NodeConnectorPlacementExecutionGraphReconciliation) error {
	if len(outcomes) == 0 {
		return errors.New("graph finalization needs outcomes")
	}
	keys := make([]string, 0, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.Schema != NodeConnectorPlacementExecutionGraphReconciliationSchema || !outcome.FixtureOwned || !outcome.CompleteChainRevalidated || !outcome.AuthorizationConsumed || !outcome.TerminalOutcomeInterpreted || !outcome.ReceiptAuthoritative || outcome.EventsAuthoritative || outcome.ProviderEvidenceAuthoritative || !outcome.GraphReconciliationPerformed || outcome.GraphCompletionClaimed || outcome.GraphFailurePropagated || outcome.NextTaskScheduled || outcome.ExecutionOrLifecycleSideEffects || outcome.Authority != (NodeConnectorPlacementExecutionGraphReconciliationAuthority{}) || outcome.GraphRunID != graphRunID || (outcome.TaskOutcome != "passed" && outcome.TaskOutcome != "failed") || !nodeExecutionFingerprint.MatchString(outcome.ArtifactFingerprint) {
			return errors.New("graph finalization outcome is not a canonical terminal task outcome")
		}
		fingerprint, err := nodeConnectorPlacementExecutionGraphReconciliationFingerprint(outcome)
		if err != nil || fingerprint != outcome.ArtifactFingerprint {
			return errors.New("graph finalization outcome fingerprint is invalid")
		}
		keys = append(keys, outcome.TaskID+"/"+outcome.OperationID+"/"+outcome.ReceiptFingerprint)
	}
	if !sort.StringsAreSorted(keys) {
		return errors.New("graph finalization outcomes must be ordinally sorted")
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] == keys[i-1] {
			return errors.New("graph finalization outcomes contain a duplicate identity")
		}
	}
	return nil
}

func nodeConnectorPlacementExecutionGraphFinalizationOutcome(outcomes []NodeConnectorPlacementExecutionGraphReconciliation) string {
	for _, outcome := range outcomes {
		if outcome.TaskOutcome == "failed" {
			return "failed"
		}
	}
	return "succeeded"
}

func validateNodeConnectorPlacementExecutionGraphFinalizationDecision(value NodeConnectorPlacementExecutionGraphFinalizationDecision, expected NodeConnectorPlacementExecutionGraphFinalizationExpected) error {
	fingerprint, err := nodeConnectorPlacementExecutionGraphFinalizationDecisionFingerprint(value)
	outcomesFingerprint, outcomeErr := nodeExecutionFingerprintValue(expected.Outcomes)
	if err != nil || outcomeErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphFinalizationDecisionSchema || (value.Decision != "approved" && value.Decision != "rejected") || !value.FixtureOwned || value.ApprovalInferred || value.Authority != (NodeConnectorPlacementExecutionGraphFinalizationAuthority{}) || value.GraphRunID != expected.GraphRunID || !nodeExecutionEqual(value.Outcomes, expected.Outcomes) || value.OutcomesFingerprint != outcomesFingerprint || fingerprint != value.DecisionFingerprint || validateNodeConnectorPlacementExecutionGraphFinalizationOutcomes(value.GraphRunID, value.Outcomes) != nil {
		return errors.New("graph finalization decision is invalid or escalates authority")
	}
	if (value.Decision == "approved" && value.Finalization != nodeConnectorPlacementExecutionGraphFinalizationOutcome(expected.Outcomes)) || (value.Decision == "rejected" && value.Finalization != "") {
		return errors.New("graph finalization result is invalid")
	}
	return nil
}

func validateNodeConnectorPlacementExecutionGraphFinalizationRequest(value NodeConnectorPlacementExecutionGraphFinalizationRequest, expected NodeConnectorPlacementExecutionGraphFinalizationExpected, decision NodeConnectorPlacementExecutionGraphFinalizationDecision) error {
	fingerprint, err := nodeConnectorPlacementExecutionGraphFinalizationRequestFingerprint(value)
	outcomesFingerprint, outcomeErr := nodeExecutionFingerprintValue(expected.Outcomes)
	if err != nil || outcomeErr != nil || value.Schema != NodeConnectorPlacementExecutionGraphFinalizationRequestSchema || !value.FixtureOwned || value.DecisionID != decision.DecisionID || value.DecisionFingerprint != decision.DecisionFingerprint || value.GraphRunID != expected.GraphRunID || value.OutcomesFingerprint != outcomesFingerprint || !value.OneTimeRequest || value.AuthorizationConsumed || value.Authority != (NodeConnectorPlacementExecutionGraphFinalizationAuthority{LocalGraphFinalization: true}) || value.Finalization != nodeConnectorPlacementExecutionGraphFinalizationOutcome(expected.Outcomes) || fingerprint != value.RequestFingerprint {
		return errors.New("graph finalization request is invalid or escalates authority")
	}
	return nil
}

func loadNodeConnectorPlacementExecutionGraphFinalizationDecision(root string, expected NodeConnectorPlacementExecutionGraphFinalizationExpected) (NodeConnectorPlacementExecutionGraphFinalizationDecision, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionGraphFinalizationDecisionName))
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphFinalizationArtifactMaxBytes {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, false, errors.New("graph finalization decision cannot be read")
	}
	var value NodeConnectorPlacementExecutionGraphFinalizationDecision
	if decodeNodeExecutionStrict(raw, &value) != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, false, errors.New("graph finalization decision is malformed")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || validateNodeConnectorPlacementExecutionGraphFinalizationDecision(value, expected) != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationDecision{}, false, errors.New("graph finalization decision is noncanonical, tampered, or conflicting")
	}
	return value, true, nil
}

func loadNodeConnectorPlacementExecutionGraphFinalizationRequest(root string, expected NodeConnectorPlacementExecutionGraphFinalizationExpected, decision NodeConnectorPlacementExecutionGraphFinalizationDecision, decisionExists bool) (NodeConnectorPlacementExecutionGraphFinalizationRequest, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, nodeConnectorPlacementExecutionGraphFinalizationRequestName))
	if os.IsNotExist(err) {
		return NodeConnectorPlacementExecutionGraphFinalizationRequest{}, false, nil
	}
	if err != nil || !decisionExists || decision.Decision != "approved" || len(raw) == 0 || len(raw) > nodeConnectorPlacementExecutionGraphFinalizationArtifactMaxBytes {
		return NodeConnectorPlacementExecutionGraphFinalizationRequest{}, false, errors.New("graph finalization request cannot be read")
	}
	var value NodeConnectorPlacementExecutionGraphFinalizationRequest
	if decodeNodeExecutionStrict(raw, &value) != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationRequest{}, false, errors.New("graph finalization request is malformed")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || validateNodeConnectorPlacementExecutionGraphFinalizationRequest(value, expected, decision) != nil {
		return NodeConnectorPlacementExecutionGraphFinalizationRequest{}, false, errors.New("graph finalization request is noncanonical, tampered, or conflicting")
	}
	return value, true, nil
}

func nodeConnectorPlacementExecutionGraphFinalizationDecisionFingerprint(value NodeConnectorPlacementExecutionGraphFinalizationDecision) (string, error) {
	value.DecisionFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}
func nodeConnectorPlacementExecutionGraphFinalizationRequestFingerprint(value NodeConnectorPlacementExecutionGraphFinalizationRequest) (string, error) {
	value.RequestFingerprint = ""
	return nodeExecutionFingerprintValue(value)
}
func cloneNodeConnectorPlacementExecutionGraphFinalizationOutcomes(value []NodeConnectorPlacementExecutionGraphReconciliation) []NodeConnectorPlacementExecutionGraphReconciliation {
	raw, _ := json.Marshal(value)
	var cloned []NodeConnectorPlacementExecutionGraphReconciliation
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
func cloneNodeConnectorPlacementExecutionGraphFinalizationDecision(value NodeConnectorPlacementExecutionGraphFinalizationDecision) NodeConnectorPlacementExecutionGraphFinalizationDecision {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphFinalizationDecision
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}
func cloneNodeConnectorPlacementExecutionGraphFinalizationRequest(value NodeConnectorPlacementExecutionGraphFinalizationRequest) NodeConnectorPlacementExecutionGraphFinalizationRequest {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphFinalizationRequest
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

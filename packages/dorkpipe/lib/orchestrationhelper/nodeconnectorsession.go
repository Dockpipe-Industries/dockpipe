package orchestrationhelper

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	NodeConnectorEnrollmentSchema         = "dorkpipe.node-connector.enrollment/v1"
	NodeConnectorCredentialEvidenceSchema = "dorkpipe.node-connector.credential-evidence/v1"
	NodeConnectorSessionHelloSchema       = "dorkpipe.node-connector.session-hello/v1"
	NodeConnectorSessionNegotiationSchema = "dorkpipe.node-connector.session-negotiation/v1"
	NodeConnectorSessionEvidenceSchema    = "dorkpipe.node-connector.session-evidence/v1"
	nodeConnectorSessionStateSchema       = "dorkpipe.node-connector.fake-session-state/v1"
	nodeConnectorSessionMaxTransitions    = 256
)

var (
	nodeConnectorCredentialIDPattern = regexp.MustCompile(`^cred-[a-z0-9][a-z0-9._:-]{6,122}$`)
	nodeConnectorSessionStateName    = regexp.MustCompile(`^connector-session-state-([0-9]{12})\.json$`)
	nodeConnectorSessionWriteAtomic  = writeJSONFileAtomic
)

// NodeConnectorSessionAuthority is deliberately all-negative. Session,
// presence, health, and capability evidence cannot authorize broker work.
type NodeConnectorSessionAuthority struct {
	LeaseGranted           bool `json:"lease_granted"`
	ExecutionAuthorized    bool `json:"execution_authorized"`
	CompletionAuthorized   bool `json:"completion_authorized"`
	CancellationAuthorized bool `json:"cancellation_authorized"`
	RetryAuthorized        bool `json:"retry_authorized"`
	MutationAuthorized     bool `json:"mutation_authorized"`
	PublicationAuthorized  bool `json:"publication_authorized"`
}

type NodeConnectorEnrollment struct {
	Schema                string `json:"schema"`
	EnrollmentID          string `json:"enrollment_id"`
	MachineID             string `json:"machine_id"`
	InitialCredentialID   string `json:"initial_credential_id"`
	EnrolledAt            string `json:"enrolled_at"`
	EnrollmentFingerprint string `json:"enrollment_fingerprint"`
}

// NodeConnectorCredentialEvidence stores only bounded opaque identities. It
// never carries credential material.
type NodeConnectorCredentialEvidence struct {
	Schema               string `json:"schema"`
	Sequence             int64  `json:"sequence"`
	EvidenceID           string `json:"evidence_id"`
	ReplayIdentity       string `json:"replay_identity"`
	EnrollmentID         string `json:"enrollment_id"`
	MachineID            string `json:"machine_id"`
	Action               string `json:"action"`
	PreviousCredentialID string `json:"previous_credential_id,omitempty"`
	CredentialID         string `json:"credential_id"`
	ObservedAt           string `json:"observed_at"`
	EvidenceFingerprint  string `json:"evidence_fingerprint"`
}

type NodeConnectorSessionHello struct {
	Schema               string `json:"schema"`
	Sequence             int64  `json:"sequence"`
	NegotiationID        string `json:"negotiation_id"`
	ReplayIdentity       string `json:"replay_identity"`
	EnrollmentID         string `json:"enrollment_id"`
	MachineID            string `json:"machine_id"`
	CredentialID         string `json:"credential_id"`
	ConnectionID         string `json:"connection_id"`
	PreviousSessionID    string `json:"previous_session_id,omitempty"`
	CapabilitySnapshotID string `json:"capability_snapshot_id"`
	ObservedAt           string `json:"observed_at"`
	HelloFingerprint     string `json:"hello_fingerprint"`
}

type NodeConnectorSessionNegotiation struct {
	Schema                 string                        `json:"schema"`
	Sequence               int64                         `json:"sequence"`
	NegotiationID          string                        `json:"negotiation_id"`
	SessionID              string                        `json:"session_id"`
	ConnectionID           string                        `json:"connection_id"`
	EnrollmentID           string                        `json:"enrollment_id"`
	MachineID              string                        `json:"machine_id"`
	CredentialID           string                        `json:"credential_id"`
	CapabilitySnapshotID   string                        `json:"capability_snapshot_id"`
	PreviousSessionID      string                        `json:"previous_session_id,omitempty"`
	RestartNegotiated      bool                          `json:"restart_negotiated"`
	NegotiatedAt           string                        `json:"negotiated_at"`
	HelloFingerprint       string                        `json:"hello_fingerprint"`
	Authority              NodeConnectorSessionAuthority `json:"authority"`
	NegotiationFingerprint string                        `json:"negotiation_fingerprint"`
}

// NodeConnectorSessionEvidence is transport-neutral evidence for presence,
// health, or an explicitly registered immutable capability refresh.
type NodeConnectorSessionEvidence struct {
	Schema                       string                        `json:"schema"`
	Sequence                     int64                         `json:"sequence"`
	EvidenceID                   string                        `json:"evidence_id"`
	ReplayIdentity               string                        `json:"replay_identity"`
	Kind                         string                        `json:"kind"`
	Status                       string                        `json:"status"`
	SessionID                    string                        `json:"session_id"`
	ConnectionID                 string                        `json:"connection_id"`
	EnrollmentID                 string                        `json:"enrollment_id"`
	MachineID                    string                        `json:"machine_id"`
	CredentialID                 string                        `json:"credential_id"`
	PreviousCapabilitySnapshotID string                        `json:"previous_capability_snapshot_id,omitempty"`
	CapabilitySnapshotID         string                        `json:"capability_snapshot_id"`
	ObservedAt                   string                        `json:"observed_at"`
	Authority                    NodeConnectorSessionAuthority `json:"authority"`
	EvidenceFingerprint          string                        `json:"evidence_fingerprint"`
}

// NodeConnectorSessionTransport is the only transport seam. Tests inject a
// deterministic in-process implementation; this package opens no connection.
type NodeConnectorSessionTransport func(NodeConnectorSessionHello) (NodeConnectorSessionNegotiation, error)

type nodeConnectorNegotiationRecord struct {
	Hello       NodeConnectorSessionHello       `json:"hello"`
	Negotiation NodeConnectorSessionNegotiation `json:"negotiation"`
}

type nodeConnectorSessionTransition struct {
	Kind        string                           `json:"kind"`
	Credential  *NodeConnectorCredentialEvidence `json:"credential,omitempty"`
	Negotiation *nodeConnectorNegotiationRecord  `json:"negotiation,omitempty"`
	Evidence    *NodeConnectorSessionEvidence    `json:"evidence,omitempty"`
}

type nodeConnectorSessionState struct {
	Schema                   string                           `json:"schema"`
	Generation               int64                            `json:"generation"`
	PreviousStateFingerprint string                           `json:"previous_state_fingerprint,omitempty"`
	Enrollment               NodeConnectorEnrollment          `json:"enrollment"`
	Transitions              []nodeConnectorSessionTransition `json:"transitions"`
	StateFingerprint         string                           `json:"state_fingerprint"`
}

type nodeConnectorRuntimeSession struct {
	SessionID          string
	CredentialID       string
	CapabilitySnapshot string
	ConnectionID       string
	Present            bool
	Health             string
}

type nodeConnectorDerivedState struct {
	currentCredential string
	revoked           map[string]bool
	sessions          map[string]nodeConnectorRuntimeSession
	negotiations      map[string]nodeConnectorNegotiationRecord
	negotiationReplay map[string]string
	evidence          map[string]any
	evidenceReplay    map[string]string
	lastAt            time.Time
}

type NodeConnectorSessionFake struct {
	root      string
	broker    *NodeExecutionFakeBroker
	transport NodeConnectorSessionTransport
	state     nodeConnectorSessionState
	mu        sync.Mutex
}

func FinalizeNodeConnectorEnrollment(value NodeConnectorEnrollment) (NodeConnectorEnrollment, error) {
	value.Schema = NodeConnectorEnrollmentSchema
	value.EnrollmentFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil {
		return NodeConnectorEnrollment{}, err
	}
	value.EnrollmentFingerprint = fingerprint
	if err := validateNodeConnectorEnrollment(value); err != nil {
		return NodeConnectorEnrollment{}, err
	}
	return value, nil
}

func FinalizeNodeConnectorCredentialEvidence(value NodeConnectorCredentialEvidence) (NodeConnectorCredentialEvidence, error) {
	value.Schema = NodeConnectorCredentialEvidenceSchema
	value.EvidenceFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil {
		return NodeConnectorCredentialEvidence{}, err
	}
	value.EvidenceFingerprint = fingerprint
	if err := validateNodeConnectorCredentialEvidence(value); err != nil {
		return NodeConnectorCredentialEvidence{}, err
	}
	return value, nil
}

func FinalizeNodeConnectorSessionHello(value NodeConnectorSessionHello) (NodeConnectorSessionHello, error) {
	value.Schema = NodeConnectorSessionHelloSchema
	value.HelloFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil {
		return NodeConnectorSessionHello{}, err
	}
	value.HelloFingerprint = fingerprint
	if err := validateNodeConnectorSessionHello(value); err != nil {
		return NodeConnectorSessionHello{}, err
	}
	return value, nil
}

func FinalizeNodeConnectorSessionNegotiation(value NodeConnectorSessionNegotiation) (NodeConnectorSessionNegotiation, error) {
	value.Schema = NodeConnectorSessionNegotiationSchema
	value.NegotiationFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	value.NegotiationFingerprint = fingerprint
	if err := validateNodeConnectorSessionNegotiation(value); err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	return value, nil
}

func FinalizeNodeConnectorSessionEvidence(value NodeConnectorSessionEvidence) (NodeConnectorSessionEvidence, error) {
	value.Schema = NodeConnectorSessionEvidenceSchema
	value.EvidenceFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil {
		return NodeConnectorSessionEvidence{}, err
	}
	value.EvidenceFingerprint = fingerprint
	if err := validateNodeConnectorSessionEvidence(value); err != nil {
		return NodeConnectorSessionEvidence{}, err
	}
	return value, nil
}

func NewNodeConnectorSessionFake(root string, broker *NodeExecutionFakeBroker, enrollment NodeConnectorEnrollment, transport NodeConnectorSessionTransport) (*NodeConnectorSessionFake, error) {
	if broker == nil {
		return nil, errors.New("node connector session requires the unchanged node execution broker")
	}
	if transport == nil {
		return nil, errors.New("node connector session requires an injected deterministic transport")
	}
	if err := validateNodeConnectorEnrollment(enrollment); err != nil {
		return nil, err
	}
	if enrollment.MachineID != broker.state.Machine.MachineID {
		return nil, errors.New("connector enrollment is bound to a different broker machine")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	states, err := loadNodeConnectorSessionStates(root, broker)
	if err != nil {
		return nil, err
	}
	fake := &NodeConnectorSessionFake{root: root, broker: broker, transport: transport}
	if len(states) == 0 {
		state := nodeConnectorSessionState{Schema: nodeConnectorSessionStateSchema, Generation: 1, Enrollment: enrollment, Transitions: []nodeConnectorSessionTransition{}}
		if err := finalizeNodeConnectorSessionState(&state, broker); err != nil {
			return nil, err
		}
		if err := nodeConnectorSessionWriteAtomic(filepath.Join(root, nodeConnectorSessionStateFileName(1)), state); err != nil {
			return nil, err
		}
		fake.state = state
		return fake, nil
	}
	state := states[len(states)-1]
	if !nodeExecutionEqual(state.Enrollment, enrollment) {
		return nil, errors.New("connector enrollment conflicts with durable session state")
	}
	fake.state = state
	return fake, nil
}

func (fake *NodeConnectorSessionFake) RecordCredential(raw []byte) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var value NodeConnectorCredentialEvidence
	if err := decodeNodeExecutionCanonical(raw, &value); err != nil {
		return fmt.Errorf("credential evidence is invalid: %w", err)
	}
	if err := validateNodeConnectorCredentialEvidence(value); err != nil {
		return err
	}
	derived, err := deriveNodeConnectorSessionState(fake.state, fake.broker)
	if err != nil {
		return err
	}
	if existing, ok := derived.evidence[value.EvidenceID]; ok {
		if accepted, same := existing.(NodeConnectorCredentialEvidence); same && nodeExecutionEqual(accepted, value) {
			return nil
		}
		return errors.New("changed duplicate credential evidence is rejected")
	}
	if _, ok := derived.evidenceReplay[value.ReplayIdentity]; ok {
		return errors.New("replayed credential evidence identity is rejected")
	}
	return fake.appendTransition(nodeConnectorSessionTransition{Kind: "credential", Credential: &value})
}

func (fake *NodeConnectorSessionFake) Negotiate(raw []byte) (NodeConnectorSessionNegotiation, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var hello NodeConnectorSessionHello
	if err := decodeNodeExecutionCanonical(raw, &hello); err != nil {
		return NodeConnectorSessionNegotiation{}, fmt.Errorf("session hello is invalid: %w", err)
	}
	if err := validateNodeConnectorSessionHello(hello); err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	derived, err := deriveNodeConnectorSessionState(fake.state, fake.broker)
	if err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	if existing, ok := derived.negotiations[hello.NegotiationID]; ok {
		if !nodeExecutionEqual(existing.Hello, hello) {
			return NodeConnectorSessionNegotiation{}, errors.New("changed duplicate session negotiation is rejected")
		}
		if existing.Hello.CredentialID != derived.currentCredential || derived.revoked[existing.Hello.CredentialID] {
			return NodeConnectorSessionNegotiation{}, errors.New("accepted negotiation replay uses a stale or revoked connector credential")
		}
		return existing.Negotiation, nil
	}
	if _, ok := derived.negotiationReplay[hello.ReplayIdentity]; ok {
		return NodeConnectorSessionNegotiation{}, errors.New("replayed session negotiation identity is rejected")
	}
	if hello.Sequence != int64(len(fake.state.Transitions)+1) {
		return NodeConnectorSessionNegotiation{}, errors.New("session negotiation sequence is stale or contains a gap")
	}
	if hello.EnrollmentID != fake.state.Enrollment.EnrollmentID || hello.MachineID != fake.state.Enrollment.MachineID {
		return NodeConnectorSessionNegotiation{}, errors.New("stale or differently bound enrollment is rejected")
	}
	if hello.CredentialID != derived.currentCredential || derived.revoked[hello.CredentialID] {
		return NodeConnectorSessionNegotiation{}, errors.New("stale or revoked connector credential is rejected")
	}
	if _, ok := fake.broker.capability(hello.CapabilitySnapshotID); !ok {
		return NodeConnectorSessionNegotiation{}, errors.New("session capability snapshot is not registered for the enrolled machine")
	}
	if hello.PreviousSessionID == "" {
		if len(derived.sessions) != 0 {
			return NodeConnectorSessionNegotiation{}, errors.New("a new session cannot replace an existing session identity")
		}
	} else {
		session, ok := derived.sessions[hello.PreviousSessionID]
		if !ok {
			return NodeConnectorSessionNegotiation{}, errors.New("restart references an unknown session identity")
		}
		if session.Present {
			return NodeConnectorSessionNegotiation{}, errors.New("restart negotiation requires explicit disconnect evidence")
		}
		if hello.CapabilitySnapshotID != session.CapabilitySnapshot {
			return NodeConnectorSessionNegotiation{}, errors.New("capability substitution requires explicit refresh evidence")
		}
	}
	negotiation, err := fake.transport(hello)
	if err != nil {
		return NodeConnectorSessionNegotiation{}, fmt.Errorf("injected session transport failed: %w", err)
	}
	record := nodeConnectorNegotiationRecord{Hello: hello, Negotiation: negotiation}
	if err := validateNodeConnectorNegotiationBinding(record); err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	if hello.PreviousSessionID == "" {
		if negotiation.RestartNegotiated {
			return NodeConnectorSessionNegotiation{}, errors.New("initial session was returned as a restart")
		}
		if negotiation.SessionID == hello.MachineID || negotiation.SessionID == hello.ConnectionID || negotiation.SessionID == hello.EnrollmentID || negotiation.SessionID == hello.CredentialID {
			return NodeConnectorSessionNegotiation{}, errors.New("session identity cannot substitute for another protocol identity")
		}
	} else if !negotiation.RestartNegotiated || negotiation.SessionID != hello.PreviousSessionID {
		return NodeConnectorSessionNegotiation{}, errors.New("restart transport returned a conflicting session identity")
	}
	if err := fake.appendTransition(nodeConnectorSessionTransition{Kind: "negotiation", Negotiation: &record}); err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	return negotiation, nil
}

func (fake *NodeConnectorSessionFake) RecordEvidence(raw []byte) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var value NodeConnectorSessionEvidence
	if err := decodeNodeExecutionCanonical(raw, &value); err != nil {
		return fmt.Errorf("session evidence is invalid: %w", err)
	}
	if err := validateNodeConnectorSessionEvidence(value); err != nil {
		return err
	}
	derived, err := deriveNodeConnectorSessionState(fake.state, fake.broker)
	if err != nil {
		return err
	}
	if existing, ok := derived.evidence[value.EvidenceID]; ok {
		if accepted, same := existing.(NodeConnectorSessionEvidence); same && nodeExecutionEqual(accepted, value) {
			return nil
		}
		return errors.New("changed duplicate session evidence is rejected")
	}
	if _, ok := derived.evidenceReplay[value.ReplayIdentity]; ok {
		return errors.New("replayed session evidence identity is rejected")
	}
	return fake.appendTransition(nodeConnectorSessionTransition{Kind: "evidence", Evidence: &value})
}

func (fake *NodeConnectorSessionFake) appendTransition(transition nodeConnectorSessionTransition) error {
	next := cloneNodeConnectorSessionState(fake.state)
	next.Transitions = append(next.Transitions, transition)
	return fake.persist(next)
}

func (fake *NodeConnectorSessionFake) persist(next nodeConnectorSessionState) error {
	next.Generation = fake.state.Generation + 1
	next.PreviousStateFingerprint = fake.state.StateFingerprint
	next.StateFingerprint = ""
	if err := finalizeNodeConnectorSessionState(&next, fake.broker); err != nil {
		return err
	}
	path := filepath.Join(fake.root, nodeConnectorSessionStateFileName(next.Generation))
	if _, err := os.Lstat(path); err == nil {
		return errors.New("next connector session state artifact already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := nodeConnectorSessionWriteAtomic(path, next); err != nil {
		return err
	}
	fake.state = next
	return nil
}

func deriveNodeConnectorSessionState(state nodeConnectorSessionState, broker *NodeExecutionFakeBroker) (nodeConnectorDerivedState, error) {
	derived := nodeConnectorDerivedState{
		currentCredential: state.Enrollment.InitialCredentialID, revoked: map[string]bool{}, sessions: map[string]nodeConnectorRuntimeSession{},
		negotiations: map[string]nodeConnectorNegotiationRecord{}, negotiationReplay: map[string]string{}, evidence: map[string]any{}, evidenceReplay: map[string]string{},
	}
	derived.lastAt, _ = parseNodeExecutionTime(state.Enrollment.EnrolledAt)
	for index, transition := range state.Transitions {
		sequence := int64(index + 1)
		var at time.Time
		switch transition.Kind {
		case "credential":
			if transition.Credential == nil || transition.Negotiation != nil || transition.Evidence != nil {
				return derived, errors.New("credential transition shape is invalid")
			}
			value := *transition.Credential
			if err := validateNodeConnectorCredentialEvidence(value); err != nil {
				return derived, err
			}
			if value.Sequence != sequence || value.EnrollmentID != state.Enrollment.EnrollmentID || value.MachineID != state.Enrollment.MachineID {
				return derived, errors.New("credential evidence sequence or enrollment binding is invalid")
			}
			if _, exists := derived.evidence[value.EvidenceID]; exists {
				return derived, errors.New("credential evidence identity is duplicated")
			}
			if _, exists := derived.evidenceReplay[value.ReplayIdentity]; exists {
				return derived, errors.New("credential replay identity is duplicated")
			}
			if value.Action == "rotate" {
				if value.PreviousCredentialID != derived.currentCredential || value.CredentialID == derived.currentCredential || derived.revoked[value.CredentialID] {
					return derived, errors.New("credential rotation does not replace the current non-revoked identity")
				}
				derived.revoked[derived.currentCredential] = true
				derived.currentCredential = value.CredentialID
			} else {
				if value.PreviousCredentialID != "" || value.CredentialID != derived.currentCredential {
					return derived, errors.New("credential revocation does not bind the current identity")
				}
				derived.revoked[value.CredentialID] = true
				derived.currentCredential = ""
			}
			derived.evidence[value.EvidenceID] = value
			derived.evidenceReplay[value.ReplayIdentity] = value.EvidenceID
			at, _ = parseNodeExecutionTime(value.ObservedAt)
		case "negotiation":
			if transition.Negotiation == nil || transition.Credential != nil || transition.Evidence != nil {
				return derived, errors.New("session negotiation transition shape is invalid")
			}
			record := *transition.Negotiation
			if err := validateNodeConnectorNegotiationBinding(record); err != nil {
				return derived, err
			}
			hello, response := record.Hello, record.Negotiation
			if hello.Sequence != sequence || response.Sequence != sequence || hello.EnrollmentID != state.Enrollment.EnrollmentID || hello.MachineID != state.Enrollment.MachineID {
				return derived, errors.New("session negotiation sequence or enrollment binding is invalid")
			}
			if _, exists := derived.negotiations[hello.NegotiationID]; exists {
				return derived, errors.New("session negotiation identity is duplicated")
			}
			if _, exists := derived.negotiationReplay[hello.ReplayIdentity]; exists {
				return derived, errors.New("session negotiation replay identity is duplicated")
			}
			if hello.CredentialID != derived.currentCredential || derived.revoked[hello.CredentialID] {
				return derived, errors.New("durable session negotiation uses a stale or revoked credential")
			}
			capability, ok := broker.capability(hello.CapabilitySnapshotID)
			if !ok || capability.MachineID != state.Enrollment.MachineID {
				return derived, errors.New("durable session negotiation substitutes an unknown capability")
			}
			if hello.PreviousSessionID == "" {
				if len(derived.sessions) != 0 || response.RestartNegotiated {
					return derived, errors.New("durable initial session negotiation conflicts with session history")
				}
			} else {
				previous, ok := derived.sessions[hello.PreviousSessionID]
				if !ok || previous.Present || previous.CapabilitySnapshot != hello.CapabilitySnapshotID || !response.RestartNegotiated || response.SessionID != hello.PreviousSessionID {
					return derived, errors.New("durable restart negotiation conflicts with prior disconnect or capability evidence")
				}
			}
			derived.sessions[response.SessionID] = nodeConnectorRuntimeSession{SessionID: response.SessionID, CredentialID: hello.CredentialID, CapabilitySnapshot: hello.CapabilitySnapshotID, ConnectionID: hello.ConnectionID}
			derived.negotiations[hello.NegotiationID] = record
			derived.negotiationReplay[hello.ReplayIdentity] = hello.NegotiationID
			at, _ = parseNodeExecutionTime(response.NegotiatedAt)
		case "evidence":
			if transition.Evidence == nil || transition.Credential != nil || transition.Negotiation != nil {
				return derived, errors.New("session evidence transition shape is invalid")
			}
			value := *transition.Evidence
			if err := validateNodeConnectorSessionEvidence(value); err != nil {
				return derived, err
			}
			if value.Sequence != sequence || value.EnrollmentID != state.Enrollment.EnrollmentID || value.MachineID != state.Enrollment.MachineID {
				return derived, errors.New("session evidence sequence or enrollment binding is invalid")
			}
			if _, exists := derived.evidence[value.EvidenceID]; exists {
				return derived, errors.New("session evidence identity is duplicated")
			}
			if _, exists := derived.evidenceReplay[value.ReplayIdentity]; exists {
				return derived, errors.New("session evidence replay identity is duplicated")
			}
			session, ok := derived.sessions[value.SessionID]
			if !ok || session.ConnectionID != value.ConnectionID || session.CredentialID != value.CredentialID || value.CredentialID != derived.currentCredential || derived.revoked[value.CredentialID] {
				return derived, errors.New("session evidence conflicts with the active session or credential")
			}
			if value.Kind == "capability_refresh" {
				if !session.Present || value.PreviousCapabilitySnapshotID != session.CapabilitySnapshot {
					return derived, errors.New("capability refresh requires present session and exact prior snapshot")
				}
				capability, registered := broker.capability(value.CapabilitySnapshotID)
				if !registered || capability.MachineID != value.MachineID {
					return derived, errors.New("capability refresh substitutes an unregistered snapshot")
				}
				session.CapabilitySnapshot = value.CapabilitySnapshotID
			} else {
				if value.PreviousCapabilitySnapshotID != "" || value.CapabilitySnapshotID != session.CapabilitySnapshot {
					return derived, errors.New("presence or health evidence substitutes the session capability")
				}
				if value.Kind == "presence" {
					session.Present = value.Status == "connected"
					if !session.Present {
						session.Health = ""
					}
				} else {
					if !session.Present {
						return derived, errors.New("health evidence requires connected presence")
					}
					session.Health = value.Status
				}
			}
			derived.sessions[value.SessionID] = session
			derived.evidence[value.EvidenceID] = value
			derived.evidenceReplay[value.ReplayIdentity] = value.EvidenceID
			at, _ = parseNodeExecutionTime(value.ObservedAt)
		default:
			return derived, errors.New("connector session transition kind is invalid")
		}
		if !at.After(derived.lastAt) {
			return derived, errors.New("connector session transition time is stale or reordered")
		}
		derived.lastAt = at
	}
	return derived, nil
}

func validateNodeConnectorEnrollment(value NodeConnectorEnrollment) error {
	if value.Schema != NodeConnectorEnrollmentSchema {
		return errors.New("connector enrollment schema is invalid")
	}
	if err := validateNodeExecutionTypedID("enrollment", value.EnrollmentID); err != nil {
		return err
	}
	if err := validateNodeExecutionTypedID("machine", value.MachineID); err != nil {
		return err
	}
	if err := validateNodeConnectorCredentialID(value.InitialCredentialID); err != nil {
		return err
	}
	if _, err := parseNodeExecutionTime(value.EnrolledAt); err != nil {
		return err
	}
	expected := value
	expected.EnrollmentFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if value.EnrollmentFingerprint != fingerprint {
		return errors.New("connector enrollment fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeConnectorCredentialEvidence(value NodeConnectorCredentialEvidence) error {
	if value.Schema != NodeConnectorCredentialEvidenceSchema || value.Sequence < 1 {
		return errors.New("credential evidence schema or sequence is invalid")
	}
	if err := validateNodeExecutionTypedID("evidence", value.EvidenceID); err != nil {
		return err
	}
	if err := validateNodeExecutionTypedID("replay", value.ReplayIdentity); err != nil {
		return err
	}
	if err := validateNodeExecutionTypedID("enrollment", value.EnrollmentID); err != nil {
		return err
	}
	if err := validateNodeExecutionTypedID("machine", value.MachineID); err != nil {
		return err
	}
	if err := validateNodeConnectorCredentialID(value.CredentialID); err != nil {
		return err
	}
	if value.Action == "rotate" {
		if err := validateNodeConnectorCredentialID(value.PreviousCredentialID); err != nil {
			return err
		}
	} else if value.Action != "revoke" || value.PreviousCredentialID != "" {
		return errors.New("credential evidence action or prior identity is invalid")
	}
	if _, err := parseNodeExecutionTime(value.ObservedAt); err != nil {
		return err
	}
	expected := value
	expected.EvidenceFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if value.EvidenceFingerprint != fingerprint {
		return errors.New("credential evidence fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeConnectorSessionHello(value NodeConnectorSessionHello) error {
	if value.Schema != NodeConnectorSessionHelloSchema || value.Sequence < 1 {
		return errors.New("session hello schema or sequence is invalid")
	}
	for kind, id := range map[string]string{"negotiation": value.NegotiationID, "replay": value.ReplayIdentity, "enrollment": value.EnrollmentID, "machine": value.MachineID, "connection": value.ConnectionID} {
		if err := validateNodeExecutionTypedID(kind, id); err != nil {
			return err
		}
	}
	if value.PreviousSessionID != "" {
		if err := validateNodeExecutionTypedID("session", value.PreviousSessionID); err != nil {
			return err
		}
	}
	if err := validateNodeConnectorCredentialID(value.CredentialID); err != nil {
		return err
	}
	if !nodeExecutionFingerprint.MatchString(value.CapabilitySnapshotID) {
		return errors.New("session hello capability snapshot identity is invalid")
	}
	if _, err := parseNodeExecutionTime(value.ObservedAt); err != nil {
		return err
	}
	expected := value
	expected.HelloFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if value.HelloFingerprint != fingerprint {
		return errors.New("session hello fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeConnectorSessionNegotiation(value NodeConnectorSessionNegotiation) error {
	if value.Schema != NodeConnectorSessionNegotiationSchema || value.Sequence < 1 {
		return errors.New("session negotiation schema or sequence is invalid")
	}
	for kind, id := range map[string]string{"negotiation": value.NegotiationID, "session": value.SessionID, "connection": value.ConnectionID, "enrollment": value.EnrollmentID, "machine": value.MachineID} {
		if err := validateNodeExecutionTypedID(kind, id); err != nil {
			return err
		}
	}
	if value.PreviousSessionID != "" {
		if err := validateNodeExecutionTypedID("session", value.PreviousSessionID); err != nil {
			return err
		}
	}
	if err := validateNodeConnectorCredentialID(value.CredentialID); err != nil {
		return err
	}
	if !nodeExecutionFingerprint.MatchString(value.CapabilitySnapshotID) || !nodeExecutionFingerprint.MatchString(value.HelloFingerprint) {
		return errors.New("session negotiation fingerprint binding is invalid")
	}
	if _, err := parseNodeExecutionTime(value.NegotiatedAt); err != nil {
		return err
	}
	if !nodeExecutionEqual(value.Authority, NodeConnectorSessionAuthority{}) {
		return errors.New("session negotiation cannot grant lifecycle authority")
	}
	expected := value
	expected.NegotiationFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if value.NegotiationFingerprint != fingerprint {
		return errors.New("session negotiation fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeConnectorNegotiationBinding(record nodeConnectorNegotiationRecord) error {
	if err := validateNodeConnectorSessionHello(record.Hello); err != nil {
		return err
	}
	if err := validateNodeConnectorSessionNegotiation(record.Negotiation); err != nil {
		return err
	}
	hello, response := record.Hello, record.Negotiation
	if response.Sequence != hello.Sequence || response.NegotiationID != hello.NegotiationID || response.ConnectionID != hello.ConnectionID || response.EnrollmentID != hello.EnrollmentID || response.MachineID != hello.MachineID || response.CredentialID != hello.CredentialID || response.CapabilitySnapshotID != hello.CapabilitySnapshotID || response.PreviousSessionID != hello.PreviousSessionID || response.NegotiatedAt != hello.ObservedAt || response.HelloFingerprint != hello.HelloFingerprint {
		return errors.New("session negotiation does not bind the exact hello")
	}
	return nil
}

func validateNodeConnectorSessionEvidence(value NodeConnectorSessionEvidence) error {
	if value.Schema != NodeConnectorSessionEvidenceSchema || value.Sequence < 1 {
		return errors.New("session evidence schema or sequence is invalid")
	}
	for kind, id := range map[string]string{"evidence": value.EvidenceID, "replay": value.ReplayIdentity, "session": value.SessionID, "connection": value.ConnectionID, "enrollment": value.EnrollmentID, "machine": value.MachineID} {
		if err := validateNodeExecutionTypedID(kind, id); err != nil {
			return err
		}
	}
	if err := validateNodeConnectorCredentialID(value.CredentialID); err != nil {
		return err
	}
	if !nodeExecutionFingerprint.MatchString(value.CapabilitySnapshotID) {
		return errors.New("session evidence capability snapshot identity is invalid")
	}
	switch value.Kind {
	case "presence":
		if value.Status != "connected" && value.Status != "disconnected" {
			return errors.New("presence evidence status is invalid")
		}
	case "health":
		if value.Status != "healthy" && value.Status != "degraded" && value.Status != "unhealthy" {
			return errors.New("health evidence status is invalid")
		}
	case "capability_refresh":
		if value.Status != "refreshed" || !nodeExecutionFingerprint.MatchString(value.PreviousCapabilitySnapshotID) || value.PreviousCapabilitySnapshotID == value.CapabilitySnapshotID {
			return errors.New("capability refresh evidence is invalid")
		}
	default:
		return errors.New("session evidence kind is invalid")
	}
	if _, err := parseNodeExecutionTime(value.ObservedAt); err != nil {
		return err
	}
	if !nodeExecutionEqual(value.Authority, NodeConnectorSessionAuthority{}) {
		return errors.New("presence, health, and capability evidence cannot grant lifecycle authority")
	}
	expected := value
	expected.EvidenceFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if value.EvidenceFingerprint != fingerprint {
		return errors.New("session evidence fingerprint does not match immutable content")
	}
	return nil
}

func validateNodeConnectorCredentialID(value string) error {
	if !nodeConnectorCredentialIDPattern.MatchString(value) || containsNodeExecutionSecret(value) || strings.Contains(value, "://") {
		return errors.New("opaque connector credential identity is invalid")
	}
	return nil
}

func finalizeNodeConnectorSessionState(state *nodeConnectorSessionState, broker *NodeExecutionFakeBroker) error {
	state.StateFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(*state)
	if err != nil {
		return err
	}
	state.StateFingerprint = fingerprint
	return validateNodeConnectorSessionState(*state, broker)
}

func validateNodeConnectorSessionState(state nodeConnectorSessionState, broker *NodeExecutionFakeBroker) error {
	if state.Schema != nodeConnectorSessionStateSchema || state.Generation < 1 || len(state.Transitions) > nodeConnectorSessionMaxTransitions {
		return errors.New("connector session state schema, generation, or transition count is invalid")
	}
	if state.Generation == 1 && state.PreviousStateFingerprint != "" {
		return errors.New("initial connector session state cannot have a previous fingerprint")
	}
	if state.Generation > 1 && !nodeExecutionFingerprint.MatchString(state.PreviousStateFingerprint) {
		return errors.New("connector session previous fingerprint is invalid")
	}
	if err := validateNodeConnectorEnrollment(state.Enrollment); err != nil {
		return err
	}
	if state.Enrollment.MachineID != broker.state.Machine.MachineID {
		return errors.New("durable connector session is bound to another broker machine")
	}
	if _, err := deriveNodeConnectorSessionState(state, broker); err != nil {
		return err
	}
	expected := state
	expected.StateFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if state.StateFingerprint != fingerprint {
		return errors.New("connector session state fingerprint does not match durable content")
	}
	return nil
}

func loadNodeConnectorSessionStates(root string, broker *NodeExecutionFakeBroker) ([]nodeConnectorSessionState, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if regexp.MustCompile(`^connector-session-state-`).MatchString(entry.Name()) {
			if !nodeConnectorSessionStateName.MatchString(entry.Name()) {
				return nil, fmt.Errorf("malformed connector session state artifact name %q", entry.Name())
			}
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	states := make([]nodeConnectorSessionState, 0, len(names))
	previous := ""
	for index, name := range names {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		var state nodeConnectorSessionState
		if err := decodeNodeExecutionStrict(raw, &state); err != nil {
			return nil, fmt.Errorf("connector session state %s is malformed: %w", name, err)
		}
		if state.Generation != int64(index+1) || name != nodeConnectorSessionStateFileName(state.Generation) || state.PreviousStateFingerprint != previous {
			return nil, fmt.Errorf("connector session state chain is broken at %s", name)
		}
		if err := validateNodeConnectorSessionState(state, broker); err != nil {
			return nil, fmt.Errorf("connector session state %s failed revalidation: %w", name, err)
		}
		previous = state.StateFingerprint
		states = append(states, state)
	}
	return states, nil
}

func nodeConnectorSessionStateFileName(generation int64) string {
	return fmt.Sprintf("connector-session-state-%012d.json", generation)
}

func cloneNodeConnectorSessionState(state nodeConnectorSessionState) nodeConnectorSessionState {
	raw, _ := jsonMarshalNodeConnector(state)
	var cloned nodeConnectorSessionState
	_ = decodeNodeExecutionStrict(raw, &cloned)
	return cloned
}

func jsonMarshalNodeConnector(value any) ([]byte, error) {
	return json.Marshal(value)
}

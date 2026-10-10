package orchestrationhelper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

const (
	NodeConnectorWireFrameSchema = "dorkpipe.node-connector.authenticated-frame/v1"
	NodeConnectorWireMaxBytes    = 64 * 1024

	NodeConnectorWireConnectorToBroker = "connector_to_broker"
	NodeConnectorWireBrokerToConnector = "broker_to_connector"

	NodeConnectorWireSessionHello       = "connector.session_hello"
	NodeConnectorWireSessionNegotiation = "broker.session_negotiation"
	NodeConnectorWireExecutionRequest   = "broker.execution_request"
	NodeConnectorWireTaskLease          = "broker.task_lease"
	NodeConnectorWireExecutionReceipt   = "connector.execution_receipt"

	nodeConnectorWireStateSchema    = "dorkpipe.node-connector.authenticated-frame-state/v1"
	nodeConnectorWireMaxPayload     = 48 * 1024
	nodeConnectorWireMaxFreshness   = 5 * time.Minute
	nodeConnectorWireMaxAcceptances = 512
)

var (
	nodeConnectorWireStateName   = regexp.MustCompile(`^connector-wire-state-([0-9]{12})\.json$`)
	nodeConnectorWireProof       = regexp.MustCompile(`^proof-[A-Za-z0-9._:-]{8,250}$`)
	nodeConnectorWireWriteAtomic = writeJSONFileAtomic
)

// NodeConnectorWireAuthentication is an injected deterministic collaborator.
// It receives canonical public binding bytes and returns/verifies an opaque
// proof. This fixture profile does not select or claim production cryptography.
type NodeConnectorWireAuthentication struct {
	Authenticate func([]byte) (string, error)
	Verify       func([]byte, string) error
}

// NodeConnectorWireCredentialCheck resolves only opaque references. Credential
// material and provider configuration are outside this package contract.
type NodeConnectorWireCredentialCheck func(direction, peerID, credentialReference string, at time.Time) error

type NodeConnectorWireFrameInput struct {
	Direction           string
	FrameID             string
	ReplayIdentity      string
	CredentialReference string
	MessageKind         string
	Payload             []byte
	IssuedAt            time.Time
	ExpiresAt           time.Time
}

// NodeConnectorWireFrame is the complete canonical JSON layout. Its payload is
// an unchanged canonical connector-session or node-execution.v1 message.
type NodeConnectorWireFrame struct {
	Schema              string          `json:"schema"`
	FrameID             string          `json:"frame_id"`
	ReplayIdentity      string          `json:"replay_identity"`
	Direction           string          `json:"direction"`
	SenderRole          string          `json:"sender_role"`
	ReceiverRole        string          `json:"receiver_role"`
	SenderPeerID        string          `json:"sender_peer_id"`
	ReceiverPeerID      string          `json:"receiver_peer_id"`
	CredentialReference string          `json:"credential_reference"`
	MessageKind         string          `json:"message_kind"`
	MessageSchema       string          `json:"message_schema"`
	PayloadFingerprint  string          `json:"payload_fingerprint"`
	IssuedAt            string          `json:"issued_at"`
	ExpiresAt           string          `json:"expires_at"`
	Payload             json.RawMessage `json:"payload"`
	AuthenticationProof string          `json:"authentication_proof"`
	FrameFingerprint    string          `json:"frame_fingerprint"`
}

type nodeConnectorWireAcceptance struct {
	FrameID          string `json:"frame_id"`
	ReplayIdentity   string `json:"replay_identity"`
	FrameFingerprint string `json:"frame_fingerprint"`
}

type nodeConnectorWireState struct {
	Schema                   string                        `json:"schema"`
	Generation               int64                         `json:"generation"`
	PreviousStateFingerprint string                        `json:"previous_state_fingerprint,omitempty"`
	Accepted                 []nodeConnectorWireAcceptance `json:"accepted"`
	StateFingerprint         string                        `json:"state_fingerprint"`
}

type nodeConnectorWirePrepared struct {
	frame   NodeConnectorWireFrame
	payload []byte
}

type NodeConnectorWireProfile struct {
	root                    string
	session                 *NodeConnectorSessionFake
	connectorPeerID         string
	brokerPeerID            string
	brokerCredential        string
	connectorAuthentication NodeConnectorWireAuthentication
	brokerAuthentication    NodeConnectorWireAuthentication
	credentialCheck         NodeConnectorWireCredentialCheck
	state                   nodeConnectorWireState
	mu                      sync.Mutex
}

type nodeConnectorWireMessageProfile struct {
	schema    string
	direction string
}

var nodeConnectorWireMessages = map[string]nodeConnectorWireMessageProfile{
	"connector.enrollment":              {NodeConnectorEnrollmentSchema, NodeConnectorWireConnectorToBroker},
	"connector.credential_evidence":     {NodeConnectorCredentialEvidenceSchema, NodeConnectorWireConnectorToBroker},
	NodeConnectorWireSessionHello:       {NodeConnectorSessionHelloSchema, NodeConnectorWireConnectorToBroker},
	NodeConnectorWireSessionNegotiation: {NodeConnectorSessionNegotiationSchema, NodeConnectorWireBrokerToConnector},
	"connector.session_evidence":        {NodeConnectorSessionEvidenceSchema, NodeConnectorWireConnectorToBroker},
	"connector.machine_identity":        {NodeExecutionMachineIdentitySchema, NodeConnectorWireConnectorToBroker},
	"connector.capability_snapshot":     {NodeExecutionCapabilitySnapshotSchema, NodeConnectorWireConnectorToBroker},
	NodeConnectorWireExecutionRequest:   {NodeExecutionRequestSchema, NodeConnectorWireBrokerToConnector},
	NodeConnectorWireTaskLease:          {NodeExecutionLeaseSchema, NodeConnectorWireBrokerToConnector},
	"connector.execution_event":         {NodeExecutionEventSchema, NodeConnectorWireConnectorToBroker},
	"broker.cancellation":               {NodeExecutionCancellationSchema, NodeConnectorWireBrokerToConnector},
	"connector.cancellation_ack":        {NodeExecutionCancellationAckSchema, NodeConnectorWireConnectorToBroker},
	"connector.artifact_manifest":       {NodeExecutionArtifactManifestSchema, NodeConnectorWireConnectorToBroker},
	NodeConnectorWireExecutionReceipt:   {NodeExecutionReceiptSchema, NodeConnectorWireConnectorToBroker},
}

func NewNodeConnectorWireProfile(root string, session *NodeConnectorSessionFake, connectorPeerID, brokerPeerID, brokerCredential string, connectorAuthentication, brokerAuthentication NodeConnectorWireAuthentication, credentialCheck NodeConnectorWireCredentialCheck) (*NodeConnectorWireProfile, error) {
	if session == nil {
		return nil, errors.New("authenticated framing requires the existing connector session")
	}
	for kind, value := range map[string]string{"connector peer": connectorPeerID, "broker peer": brokerPeerID} {
		if err := validateNodeExecutionTypedID("peer", value); err != nil {
			return nil, fmt.Errorf("%s identity is invalid: %w", kind, err)
		}
	}
	if connectorPeerID == brokerPeerID || connectorPeerID == session.state.Enrollment.MachineID || brokerPeerID == session.state.Enrollment.MachineID {
		return nil, errors.New("peer authentication and machine identities must remain distinct")
	}
	if err := validateNodeConnectorCredentialID(brokerCredential); err != nil {
		return nil, fmt.Errorf("broker credential reference is invalid: %w", err)
	}
	if connectorAuthentication.Authenticate == nil || connectorAuthentication.Verify == nil || brokerAuthentication.Authenticate == nil || brokerAuthentication.Verify == nil || credentialCheck == nil {
		return nil, errors.New("authenticated framing requires independent injected authentication and credential collaborators")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	states, err := loadNodeConnectorWireStates(root)
	if err != nil {
		return nil, err
	}
	profile := &NodeConnectorWireProfile{
		root: root, session: session, connectorPeerID: connectorPeerID, brokerPeerID: brokerPeerID,
		brokerCredential: brokerCredential, connectorAuthentication: connectorAuthentication,
		brokerAuthentication: brokerAuthentication, credentialCheck: credentialCheck,
	}
	if len(states) == 0 {
		state := nodeConnectorWireState{Schema: nodeConnectorWireStateSchema, Generation: 1, Accepted: []nodeConnectorWireAcceptance{}}
		if err := finalizeNodeConnectorWireState(&state); err != nil {
			return nil, err
		}
		if err := nodeConnectorWireWriteAtomic(filepath.Join(root, nodeConnectorWireStateFileName(1)), state); err != nil {
			return nil, err
		}
		profile.state = state
		return profile, nil
	}
	profile.state = states[len(states)-1]
	return profile, nil
}

func (profile *NodeConnectorWireProfile) EncodeFrame(input NodeConnectorWireFrameInput) ([]byte, error) {
	message, ok := nodeConnectorWireMessages[input.MessageKind]
	if !ok || message.direction != input.Direction {
		return nil, errors.New("wire message kind or direction is unsupported")
	}
	if len(input.Payload) == 0 || len(input.Payload) > nodeConnectorWireMaxPayload {
		return nil, errors.New("wire payload is empty or oversized")
	}
	if err := validateNodeConnectorWirePayload(input.MessageKind, message.schema, input.Payload); err != nil {
		return nil, err
	}
	senderRole, receiverRole, senderPeer, receiverPeer := profile.directionBindings(input.Direction)
	frame := NodeConnectorWireFrame{
		Schema: NodeConnectorWireFrameSchema, FrameID: input.FrameID, ReplayIdentity: input.ReplayIdentity,
		Direction: input.Direction, SenderRole: senderRole, ReceiverRole: receiverRole,
		SenderPeerID: senderPeer, ReceiverPeerID: receiverPeer, CredentialReference: input.CredentialReference,
		MessageKind: input.MessageKind, MessageSchema: message.schema, PayloadFingerprint: nodeConnectorWirePayloadFingerprint(input.Payload),
		IssuedAt: nodeExecutionTime(input.IssuedAt), ExpiresAt: nodeExecutionTime(input.ExpiresAt), Payload: append(json.RawMessage{}, input.Payload...),
	}
	if err := profile.validateFrameShape(frame); err != nil {
		return nil, err
	}
	binding, err := nodeConnectorWireAuthenticationBytes(frame)
	if err != nil {
		return nil, err
	}
	authentication := profile.authentication(input.Direction)
	proof, err := authentication.Authenticate(binding)
	if err != nil {
		return nil, fmt.Errorf("injected wire authentication failed: %w", err)
	}
	frame.AuthenticationProof = proof
	if !nodeConnectorWireProof.MatchString(proof) {
		return nil, errors.New("wire authentication proof is malformed")
	}
	frame.FrameFingerprint, err = nodeConnectorWireFrameFingerprint(frame)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return nil, err
	}
	if len(raw) > NodeConnectorWireMaxBytes {
		return nil, errors.New("canonical wire frame exceeds the explicit size bound")
	}
	return raw, nil
}

// NegotiateSession verifies one connector-to-broker hello frame before the
// unchanged session fake is invoked. Replay evidence is published only after
// the session negotiation succeeds.
func (profile *NodeConnectorWireProfile) NegotiateSession(raw []byte, at time.Time) (NodeConnectorSessionNegotiation, error) {
	profile.mu.Lock()
	defer profile.mu.Unlock()
	prepared, err := profile.prepare(raw, NodeConnectorWireSessionHello, at)
	if err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	var hello NodeConnectorSessionHello
	if err := decodeNodeExecutionCanonical(prepared.payload, &hello); err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	if prepared.frame.CredentialReference != hello.CredentialID {
		return NodeConnectorSessionNegotiation{}, errors.New("hello frame credential reference does not bind the session credential")
	}
	negotiation, err := profile.session.Negotiate(prepared.payload)
	if err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	if err := profile.accept([]nodeConnectorWirePrepared{prepared}); err != nil {
		return NodeConnectorSessionNegotiation{}, err
	}
	return negotiation, nil
}

// AcceptSessionNegotiation authenticates the broker direction independently
// and proves the payload is the exact already accepted session response.
func (profile *NodeConnectorWireProfile) AcceptSessionNegotiation(raw []byte, expected NodeConnectorSessionNegotiation, at time.Time) error {
	profile.mu.Lock()
	defer profile.mu.Unlock()
	prepared, err := profile.prepare(raw, NodeConnectorWireSessionNegotiation, at)
	if err != nil {
		return err
	}
	var negotiation NodeConnectorSessionNegotiation
	if err := decodeNodeExecutionCanonical(prepared.payload, &negotiation); err != nil {
		return err
	}
	if !nodeExecutionEqual(negotiation, expected) {
		return errors.New("broker negotiation frame does not preserve the exact accepted payload")
	}
	derived, err := deriveNodeConnectorSessionState(profile.session.state, profile.session.broker)
	if err != nil {
		return err
	}
	record, ok := derived.negotiations[negotiation.NegotiationID]
	if !ok || !nodeExecutionEqual(record.Negotiation, negotiation) {
		return errors.New("broker negotiation frame is not bound to accepted session evidence")
	}
	return profile.accept([]nodeConnectorWirePrepared{prepared})
}

// DispatchAcceptedValidation carries two independently authenticated broker
// frames through the existing session-to-dispatch seam. Frames never create a
// request or lease; both payloads must already exist byte-exactly in the broker.
func (profile *NodeConnectorWireProfile) DispatchAcceptedValidation(connector *NodeValidationConnector, negotiation NodeConnectorSessionNegotiation, requestFrame, leaseFrame []byte, at time.Time) (NodeExecutionReceipt, error) {
	profile.mu.Lock()
	defer profile.mu.Unlock()
	if err := profile.revalidateDurableState(); err != nil {
		return NodeExecutionReceipt{}, err
	}
	requestPrepared, err := profile.prepare(requestFrame, NodeConnectorWireExecutionRequest, at)
	if err != nil {
		return NodeExecutionReceipt{}, err
	}
	leasePrepared, err := profile.prepare(leaseFrame, NodeConnectorWireTaskLease, at)
	if err != nil {
		return NodeExecutionReceipt{}, err
	}
	if requestPrepared.frame.FrameID == leasePrepared.frame.FrameID || requestPrepared.frame.ReplayIdentity == leasePrepared.frame.ReplayIdentity {
		return NodeExecutionReceipt{}, errors.New("request and lease frames require distinct frame and replay identities")
	}
	var request NodeExecutionRequest
	var lease NodeExecutionTaskLease
	if err := decodeNodeExecutionCanonical(requestPrepared.payload, &request); err != nil {
		return NodeExecutionReceipt{}, err
	}
	if err := decodeNodeExecutionCanonical(leasePrepared.payload, &lease); err != nil {
		return NodeExecutionReceipt{}, err
	}
	identities := []string{
		requestPrepared.frame.FrameID, requestPrepared.frame.ReplayIdentity, leasePrepared.frame.FrameID, leasePrepared.frame.ReplayIdentity,
		profile.connectorPeerID, profile.brokerPeerID, negotiation.MachineID, negotiation.CapabilitySnapshotID,
		request.OperationID, lease.LeaseID, negotiation.ConnectionID, negotiation.EnrollmentID,
		negotiation.CredentialID, profile.brokerCredential, negotiation.SessionID,
	}
	if err := validateNodeConnectorWireDistinctIdentities(identities); err != nil {
		return NodeExecutionReceipt{}, err
	}
	receipt, err := profile.session.DispatchAcceptedValidation(connector, negotiation, request, lease, at)
	if err != nil {
		return NodeExecutionReceipt{}, err
	}
	if err := profile.accept([]nodeConnectorWirePrepared{requestPrepared, leasePrepared}); err != nil {
		return NodeExecutionReceipt{}, err
	}
	return receipt, nil
}

// AcceptExecutionReceipt authenticates receipt evidence only. It revalidates
// the existing durable receipt and cannot create completion authority.
func (profile *NodeConnectorWireProfile) AcceptExecutionReceipt(raw []byte, negotiation NodeConnectorSessionNegotiation, expected NodeExecutionReceipt, at time.Time) error {
	profile.mu.Lock()
	defer profile.mu.Unlock()
	if err := profile.revalidateDurableState(); err != nil {
		return err
	}
	prepared, err := profile.prepare(raw, NodeConnectorWireExecutionReceipt, at)
	if err != nil {
		return err
	}
	var receipt NodeExecutionReceipt
	if err := decodeNodeExecutionCanonical(prepared.payload, &receipt); err != nil {
		return err
	}
	if prepared.frame.CredentialReference != negotiation.CredentialID || !nodeExecutionEqual(receipt, expected) {
		return errors.New("receipt frame does not bind the current connector credential or exact receipt")
	}
	derived, err := deriveNodeConnectorSessionState(profile.session.state, profile.session.broker)
	if err != nil {
		return err
	}
	session, ok := derived.sessions[negotiation.SessionID]
	operation, operationOK := profile.session.broker.state.Operations[receipt.OperationID]
	if !ok || !session.Present || session.Health != "healthy" || session.CredentialID != prepared.frame.CredentialReference || derived.revoked[prepared.frame.CredentialReference] || !operationOK || operation.Receipt == nil || !nodeExecutionEqual(*operation.Receipt, receipt) {
		return errors.New("receipt frame is not bound to current session and durable broker evidence")
	}
	identities := []string{prepared.frame.FrameID, prepared.frame.ReplayIdentity, profile.connectorPeerID, profile.brokerPeerID, receipt.MachineID, receipt.CapabilitySnapshotID, receipt.OperationID, receipt.LeaseID, receipt.ReceiptID, negotiation.ConnectionID, negotiation.EnrollmentID, negotiation.CredentialID, profile.brokerCredential, negotiation.SessionID}
	if err := validateNodeConnectorWireDistinctIdentities(identities); err != nil {
		return err
	}
	return profile.accept([]nodeConnectorWirePrepared{prepared})
}

func (profile *NodeConnectorWireProfile) prepare(raw []byte, expectedKind string, at time.Time) (nodeConnectorWirePrepared, error) {
	if len(raw) == 0 || len(raw) > NodeConnectorWireMaxBytes {
		return nodeConnectorWirePrepared{}, errors.New("wire frame is empty or exceeds the explicit size bound")
	}
	var frame NodeConnectorWireFrame
	if err := decodeNodeExecutionCanonical(raw, &frame); err != nil {
		return nodeConnectorWirePrepared{}, fmt.Errorf("wire frame is malformed or noncanonical: %w", err)
	}
	if frame.MessageKind != expectedKind {
		return nodeConnectorWirePrepared{}, errors.New("wire frame message kind is not accepted at this seam")
	}
	if err := profile.validateFrameShape(frame); err != nil {
		return nodeConnectorWirePrepared{}, err
	}
	message := nodeConnectorWireMessages[frame.MessageKind]
	if frame.Direction != message.direction {
		return nodeConnectorWirePrepared{}, errors.New("wire frame direction conflicts with its message kind")
	}
	expectedSenderRole, expectedReceiverRole, expectedSenderPeer, expectedReceiverPeer := profile.directionBindings(frame.Direction)
	if frame.SenderRole != expectedSenderRole || frame.ReceiverRole != expectedReceiverRole || frame.SenderPeerID != expectedSenderPeer || frame.ReceiverPeerID != expectedReceiverPeer {
		return nodeConnectorWirePrepared{}, errors.New("wire frame direction, roles, or peer identities are substituted")
	}
	if frame.Direction == NodeConnectorWireBrokerToConnector && frame.CredentialReference != profile.brokerCredential {
		return nodeConnectorWirePrepared{}, errors.New("broker wire frame credential reference is substituted")
	}
	issuedAt, _ := parseNodeExecutionTime(frame.IssuedAt)
	expiresAt, _ := parseNodeExecutionTime(frame.ExpiresAt)
	at = at.UTC()
	if at.Before(issuedAt) || !at.Before(expiresAt) {
		return nodeConnectorWirePrepared{}, errors.New("wire frame freshness window is not current")
	}
	binding, err := nodeConnectorWireAuthenticationBytes(frame)
	if err != nil {
		return nodeConnectorWirePrepared{}, err
	}
	if err := profile.authentication(frame.Direction).Verify(binding, frame.AuthenticationProof); err != nil {
		return nodeConnectorWirePrepared{}, fmt.Errorf("wire peer authentication failed: %w", err)
	}
	if err := profile.credentialCheck(frame.Direction, frame.SenderPeerID, frame.CredentialReference, at); err != nil {
		return nodeConnectorWirePrepared{}, fmt.Errorf("wire credential reference is not current: %w", err)
	}
	for _, accepted := range profile.state.Accepted {
		if accepted.FrameID == frame.FrameID {
			return nodeConnectorWirePrepared{}, errors.New("wire frame identity is replayed")
		}
		if accepted.ReplayIdentity == frame.ReplayIdentity {
			return nodeConnectorWirePrepared{}, errors.New("wire replay identity is replayed")
		}
	}
	return nodeConnectorWirePrepared{frame: frame, payload: append([]byte{}, frame.Payload...)}, nil
}

func (profile *NodeConnectorWireProfile) validateFrameShape(frame NodeConnectorWireFrame) error {
	if frame.Schema != NodeConnectorWireFrameSchema {
		return errors.New("wire frame schema is unsupported")
	}
	for _, value := range []struct{ kind, id string }{
		{"frame", frame.FrameID}, {"replay", frame.ReplayIdentity}, {"peer", frame.SenderPeerID}, {"peer", frame.ReceiverPeerID},
	} {
		if err := validateNodeExecutionTypedID(value.kind, value.id); err != nil {
			return err
		}
	}
	if err := validateNodeConnectorCredentialID(frame.CredentialReference); err != nil {
		return err
	}
	if err := validateNodeConnectorWireDistinctIdentities([]string{frame.FrameID, frame.ReplayIdentity, frame.SenderPeerID, frame.ReceiverPeerID, frame.CredentialReference}); err != nil {
		return err
	}
	message, ok := nodeConnectorWireMessages[frame.MessageKind]
	if !ok || frame.MessageSchema != message.schema || frame.Direction != message.direction {
		return errors.New("wire message kind, schema, or direction is unsupported")
	}
	if len(frame.Payload) == 0 || len(frame.Payload) > nodeConnectorWireMaxPayload || frame.PayloadFingerprint != nodeConnectorWirePayloadFingerprint(frame.Payload) {
		return errors.New("wire payload bytes, size, or fingerprint are invalid")
	}
	if err := validateNodeConnectorWirePayload(frame.MessageKind, frame.MessageSchema, frame.Payload); err != nil {
		return err
	}
	issuedAt, err := parseNodeExecutionTime(frame.IssuedAt)
	if err != nil {
		return err
	}
	expiresAt, err := parseNodeExecutionTime(frame.ExpiresAt)
	if err != nil || !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > nodeConnectorWireMaxFreshness {
		return errors.New("wire frame freshness window is invalid")
	}
	if frame.AuthenticationProof != "" && !nodeConnectorWireProof.MatchString(frame.AuthenticationProof) {
		return errors.New("wire authentication proof is malformed")
	}
	if frame.FrameFingerprint != "" {
		expected, err := nodeConnectorWireFrameFingerprint(frame)
		if err != nil || frame.FrameFingerprint != expected {
			return errors.New("wire frame fingerprint does not bind canonical content")
		}
	}
	return nil
}

func (profile *NodeConnectorWireProfile) directionBindings(direction string) (string, string, string, string) {
	if direction == NodeConnectorWireConnectorToBroker {
		return "connector", "broker", profile.connectorPeerID, profile.brokerPeerID
	}
	return "broker", "connector", profile.brokerPeerID, profile.connectorPeerID
}

func (profile *NodeConnectorWireProfile) authentication(direction string) NodeConnectorWireAuthentication {
	if direction == NodeConnectorWireConnectorToBroker {
		return profile.connectorAuthentication
	}
	return profile.brokerAuthentication
}

func (profile *NodeConnectorWireProfile) accept(prepared []nodeConnectorWirePrepared) error {
	if len(profile.state.Accepted)+len(prepared) > nodeConnectorWireMaxAcceptances {
		return errors.New("wire replay evidence bound is exhausted")
	}
	next := profile.state
	next.Accepted = append([]nodeConnectorWireAcceptance{}, profile.state.Accepted...)
	seenFrame, seenReplay := map[string]bool{}, map[string]bool{}
	for _, value := range prepared {
		if seenFrame[value.frame.FrameID] || seenReplay[value.frame.ReplayIdentity] {
			return errors.New("wire acceptance batch contains duplicate identities")
		}
		seenFrame[value.frame.FrameID], seenReplay[value.frame.ReplayIdentity] = true, true
		next.Accepted = append(next.Accepted, nodeConnectorWireAcceptance{FrameID: value.frame.FrameID, ReplayIdentity: value.frame.ReplayIdentity, FrameFingerprint: value.frame.FrameFingerprint})
	}
	next.Generation++
	next.PreviousStateFingerprint = profile.state.StateFingerprint
	if err := finalizeNodeConnectorWireState(&next); err != nil {
		return err
	}
	path := filepath.Join(profile.root, nodeConnectorWireStateFileName(next.Generation))
	if _, err := os.Lstat(path); err == nil {
		return errors.New("next wire replay artifact already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := nodeConnectorWireWriteAtomic(path, next); err != nil {
		return err
	}
	profile.state = next
	return nil
}

func (profile *NodeConnectorWireProfile) revalidateDurableState() error {
	states, err := loadNodeConnectorWireStates(profile.root)
	if err != nil {
		return fmt.Errorf("wire replay state failed revalidation: %w", err)
	}
	if len(states) == 0 || states[len(states)-1].StateFingerprint != profile.state.StateFingerprint {
		return errors.New("wire replay state is missing or stale")
	}
	return nil
}

func validateNodeConnectorWirePayload(kind, schema string, raw []byte) error {
	fail := func(err error) error {
		if err == nil {
			return nil
		}
		return fmt.Errorf("wire %s payload is invalid: %w", kind, err)
	}
	switch schema {
	case NodeConnectorEnrollmentSchema:
		var value NodeConnectorEnrollment
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeConnectorEnrollment(value) }))
	case NodeConnectorCredentialEvidenceSchema:
		var value NodeConnectorCredentialEvidence
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeConnectorCredentialEvidence(value) }))
	case NodeConnectorSessionHelloSchema:
		var value NodeConnectorSessionHello
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeConnectorSessionHello(value) }))
	case NodeConnectorSessionNegotiationSchema:
		var value NodeConnectorSessionNegotiation
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeConnectorSessionNegotiation(value) }))
	case NodeConnectorSessionEvidenceSchema:
		var value NodeConnectorSessionEvidence
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeConnectorSessionEvidence(value) }))
	case NodeExecutionMachineIdentitySchema:
		var value NodeExecutionMachineIdentity
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionMachine(value) }))
	case NodeExecutionCapabilitySnapshotSchema:
		var value NodeExecutionCapabilitySnapshot
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionCapability(value) }))
	case NodeExecutionRequestSchema:
		var value NodeExecutionRequest
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionRequest(value) }))
	case NodeExecutionLeaseSchema:
		var value NodeExecutionTaskLease
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionLease(value) }))
	case NodeExecutionEventSchema:
		var value NodeExecutionEventEnvelope
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionEvent(value) }))
	case NodeExecutionCancellationSchema:
		var value NodeExecutionCancellation
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionCancellation(value) }))
	case NodeExecutionCancellationAckSchema:
		var value NodeExecutionCancellationAck
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionCancellationAck(value) }))
	case NodeExecutionArtifactManifestSchema:
		var value NodeExecutionArtifactManifest
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionManifest(value) }))
	case NodeExecutionReceiptSchema:
		var value NodeExecutionReceipt
		return fail(decodeAndValidateNodeConnectorWire(raw, &value, func() error { return validateNodeExecutionReceiptShape(value) }))
	default:
		return errors.New("wire message schema is not allowlisted")
	}
}

func decodeAndValidateNodeConnectorWire(raw []byte, target any, validate func() error) error {
	if err := decodeNodeExecutionCanonical(raw, target); err != nil {
		return err
	}
	return validate()
}

func nodeConnectorWireAuthenticationBytes(frame NodeConnectorWireFrame) ([]byte, error) {
	frame.AuthenticationProof = ""
	frame.FrameFingerprint = ""
	return json.Marshal(frame)
}

func nodeConnectorWireFrameFingerprint(frame NodeConnectorWireFrame) (string, error) {
	frame.FrameFingerprint = ""
	return nodeExecutionFingerprintValue(frame)
}

func nodeConnectorWirePayloadFingerprint(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validateNodeConnectorWireDistinctIdentities(values []string) error {
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" {
			return errors.New("wire protocol identity cannot be empty")
		}
		if seen[value] {
			return errors.New("wire, peer, credential, session, request, lease, and receipt identities must remain distinct")
		}
		seen[value] = true
	}
	return nil
}

func finalizeNodeConnectorWireState(state *nodeConnectorWireState) error {
	state.StateFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(*state)
	if err != nil {
		return err
	}
	state.StateFingerprint = fingerprint
	return validateNodeConnectorWireState(*state)
}

func validateNodeConnectorWireState(state nodeConnectorWireState) error {
	if state.Schema != nodeConnectorWireStateSchema || state.Generation < 1 || len(state.Accepted) > nodeConnectorWireMaxAcceptances {
		return errors.New("wire replay state schema, generation, or acceptance count is invalid")
	}
	if state.Generation == 1 && state.PreviousStateFingerprint != "" || state.Generation > 1 && !nodeExecutionFingerprint.MatchString(state.PreviousStateFingerprint) {
		return errors.New("wire replay state previous fingerprint is invalid")
	}
	frames, replays := map[string]bool{}, map[string]bool{}
	for _, accepted := range state.Accepted {
		if err := validateNodeExecutionTypedID("frame", accepted.FrameID); err != nil {
			return err
		}
		if err := validateNodeExecutionTypedID("replay", accepted.ReplayIdentity); err != nil {
			return err
		}
		if frames[accepted.FrameID] || replays[accepted.ReplayIdentity] || !nodeExecutionFingerprint.MatchString(accepted.FrameFingerprint) {
			return errors.New("wire replay state contains duplicate or malformed evidence")
		}
		frames[accepted.FrameID], replays[accepted.ReplayIdentity] = true, true
	}
	expected := state
	expected.StateFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if state.StateFingerprint != fingerprint {
		return errors.New("wire replay state fingerprint does not match durable content")
	}
	return nil
}

func loadNodeConnectorWireStates(root string) ([]nodeConnectorWireState, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if bytes.HasPrefix([]byte(entry.Name()), []byte("connector-wire-state-")) {
			if !nodeConnectorWireStateName.MatchString(entry.Name()) {
				return nil, fmt.Errorf("malformed wire replay state artifact name %q", entry.Name())
			}
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	states := make([]nodeConnectorWireState, 0, len(names))
	previous := ""
	for index, name := range names {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		var state nodeConnectorWireState
		if err := decodeNodeExecutionStrict(raw, &state); err != nil {
			return nil, fmt.Errorf("wire replay state %s is malformed: %w", name, err)
		}
		if state.Generation != int64(index+1) || name != nodeConnectorWireStateFileName(state.Generation) || state.PreviousStateFingerprint != previous {
			return nil, fmt.Errorf("wire replay state chain is broken at %s", name)
		}
		if err := validateNodeConnectorWireState(state); err != nil {
			return nil, fmt.Errorf("wire replay state %s failed revalidation: %w", name, err)
		}
		previous = state.StateFingerprint
		states = append(states, state)
	}
	return states, nil
}

func nodeConnectorWireStateFileName(generation int64) string {
	return fmt.Sprintf("connector-wire-state-%012d.json", generation)
}

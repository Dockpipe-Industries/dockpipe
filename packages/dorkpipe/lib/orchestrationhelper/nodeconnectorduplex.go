package orchestrationhelper

import (
	"bytes"
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
	NodeConnectorDuplexConfigSchema = "dorkpipe.node-connector.duplex-config/v1"

	nodeConnectorDuplexStateSchema  = "dorkpipe.node-connector.duplex-state/v1"
	nodeConnectorDuplexMaxItems     = 512
	nodeConnectorDuplexQueued       = "queued"
	nodeConnectorDuplexInFlight     = "in_flight"
	nodeConnectorDuplexAcknowledged = "acknowledged"
)

var (
	nodeConnectorDuplexStateName   = regexp.MustCompile(`^connector-duplex-state-([0-9]{12})\.json$`)
	nodeConnectorDuplexWriteAtomic = writeJSONFileAtomic
)

type NodeConnectorDuplexLimits struct {
	MaxQueuedFrames   int `json:"max_queued_frames"`
	MaxQueuedBytes    int `json:"max_queued_bytes"`
	MaxInFlightFrames int `json:"max_in_flight_frames"`
	MaxInFlightBytes  int `json:"max_in_flight_bytes"`
	MaxFrameBytes     int `json:"max_frame_bytes"`
}

// NodeConnectorDuplexConfig binds one deterministic exchange identity and its
// explicit flow-control limits. It contains no lease, execution, lifecycle, or
// credential authority.
type NodeConnectorDuplexConfig struct {
	Schema            string                    `json:"schema"`
	ExchangeID        string                    `json:"exchange_id"`
	ConnectorPeerID   string                    `json:"connector_peer_id"`
	BrokerPeerID      string                    `json:"broker_peer_id"`
	Limits            NodeConnectorDuplexLimits `json:"limits"`
	ConfigFingerprint string                    `json:"config_fingerprint"`
}

type NodeConnectorDuplexCursor struct {
	ExchangeID           string `json:"exchange_id"`
	ConfigFingerprint    string `json:"config_fingerprint"`
	Direction            string `json:"direction"`
	AcceptedSequence     int64  `json:"accepted_sequence"`
	DeliveredSequence    int64  `json:"delivered_sequence"`
	AcknowledgedSequence int64  `json:"acknowledged_sequence"`
}

type NodeConnectorDuplexDirectionSnapshot struct {
	AcceptedSequence     int64 `json:"accepted_sequence"`
	DeliveredSequence    int64 `json:"delivered_sequence"`
	AcknowledgedSequence int64 `json:"acknowledged_sequence"`
	QueuedFrames         int   `json:"queued_frames"`
	QueuedBytes          int   `json:"queued_bytes"`
	InFlightFrames       int   `json:"in_flight_frames"`
	InFlightBytes        int   `json:"in_flight_bytes"`
}

type NodeConnectorDuplexSnapshot struct {
	ExchangeID        string                               `json:"exchange_id"`
	ConfigFingerprint string                               `json:"config_fingerprint"`
	ConnectorToBroker NodeConnectorDuplexDirectionSnapshot `json:"connector_to_broker"`
	BrokerToConnector NodeConnectorDuplexDirectionSnapshot `json:"broker_to_connector"`
}

type nodeConnectorDuplexItem struct {
	Sequence         int64  `json:"sequence"`
	FrameID          string `json:"frame_id"`
	ReplayIdentity   string `json:"replay_identity"`
	FrameFingerprint string `json:"frame_fingerprint"`
	FrameBytes       []byte `json:"frame_bytes"`
	State            string `json:"state"`
}

type nodeConnectorDuplexDirectionState struct {
	AcceptedSequence     int64                     `json:"accepted_sequence"`
	DeliveredSequence    int64                     `json:"delivered_sequence"`
	AcknowledgedSequence int64                     `json:"acknowledged_sequence"`
	Items                []nodeConnectorDuplexItem `json:"items"`
}

type nodeConnectorDuplexDirections struct {
	ConnectorToBroker nodeConnectorDuplexDirectionState `json:"connector_to_broker"`
	BrokerToConnector nodeConnectorDuplexDirectionState `json:"broker_to_connector"`
}

type nodeConnectorDuplexState struct {
	Schema                   string                        `json:"schema"`
	Generation               int64                         `json:"generation"`
	PreviousStateFingerprint string                        `json:"previous_state_fingerprint,omitempty"`
	Config                   NodeConnectorDuplexConfig     `json:"config"`
	Directions               nodeConnectorDuplexDirections `json:"directions"`
	StateFingerprint         string                        `json:"state_fingerprint"`
}

// NodeConnectorDuplexReceiver is an injected in-process acceptance boundary.
// It receives byte-exact authenticated frames in one direction and may only
// acknowledge success after the existing wire/session contract accepts them.
type NodeConnectorDuplexReceiver func(frames [][]byte) error

// NodeConnectorDuplex is a deterministic proof seam. It opens no transport and
// cannot create requests, leases, receipts, sessions, credentials, or work.
type NodeConnectorDuplex struct {
	root  string
	wire  *NodeConnectorWireProfile
	state nodeConnectorDuplexState
	mu    sync.Mutex
}

func FinalizeNodeConnectorDuplexConfig(config NodeConnectorDuplexConfig) (NodeConnectorDuplexConfig, error) {
	config.Schema = NodeConnectorDuplexConfigSchema
	config.ConfigFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(config)
	if err != nil {
		return NodeConnectorDuplexConfig{}, err
	}
	config.ConfigFingerprint = fingerprint
	if err := validateNodeConnectorDuplexConfig(config); err != nil {
		return NodeConnectorDuplexConfig{}, err
	}
	return config, nil
}

func NewNodeConnectorDuplex(root string, wire *NodeConnectorWireProfile, config NodeConnectorDuplexConfig) (*NodeConnectorDuplex, error) {
	if wire == nil {
		return nil, errors.New("duplex exchange requires the existing authenticated wire profile")
	}
	if err := validateNodeConnectorDuplexConfig(config); err != nil {
		return nil, err
	}
	if config.ConnectorPeerID != wire.connectorPeerID || config.BrokerPeerID != wire.brokerPeerID {
		return nil, errors.New("duplex configuration conflicts with authenticated wire peers")
	}
	if config.ExchangeID == config.ConnectorPeerID || config.ExchangeID == config.BrokerPeerID || config.ExchangeID == wire.session.state.Enrollment.MachineID {
		return nil, errors.New("exchange, peer, and machine identities must remain distinct")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	states, err := loadNodeConnectorDuplexStates(root)
	if err != nil {
		return nil, err
	}
	exchange := &NodeConnectorDuplex{root: root, wire: wire}
	if len(states) == 0 {
		state := nodeConnectorDuplexState{
			Schema:     nodeConnectorDuplexStateSchema,
			Generation: 1,
			Config:     config,
			Directions: nodeConnectorDuplexDirections{
				ConnectorToBroker: nodeConnectorDuplexDirectionState{Items: []nodeConnectorDuplexItem{}},
				BrokerToConnector: nodeConnectorDuplexDirectionState{Items: []nodeConnectorDuplexItem{}},
			},
		}
		if err := finalizeNodeConnectorDuplexState(&state); err != nil {
			return nil, err
		}
		if err := nodeConnectorDuplexWriteAtomic(filepath.Join(root, nodeConnectorDuplexStateFileName(1)), state); err != nil {
			return nil, err
		}
		exchange.state = state
		return exchange, nil
	}
	last := states[len(states)-1]
	if !nodeExecutionEqual(last.Config, config) {
		return nil, errors.New("durable duplex configuration identity is incompatible")
	}
	exchange.state = last
	return exchange, nil
}

// AcceptFrame verifies and durably queues one authenticated frame at the exact
// next sequence in its direction. Rejection publishes no cursor or counters.
func (exchange *NodeConnectorDuplex) AcceptFrame(direction string, sequence int64, raw []byte, at time.Time) error {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if err := exchange.revalidateDurableState(); err != nil {
		return err
	}
	directionState, err := exchange.direction(direction)
	if err != nil {
		return err
	}
	if sequence != directionState.AcceptedSequence+1 {
		return errors.New("duplex frame sequence is gapped, regressed, or duplicated")
	}
	if len(raw) == 0 || len(raw) > exchange.state.Config.Limits.MaxFrameBytes {
		return errors.New("duplex frame is empty or exceeds the individual frame limit")
	}
	var frame NodeConnectorWireFrame
	if err := decodeNodeExecutionCanonical(raw, &frame); err != nil {
		return fmt.Errorf("duplex frame is malformed or noncanonical: %w", err)
	}
	if frame.Direction != direction {
		return errors.New("duplex direction substitution is rejected")
	}
	if exchange.hasWireIdentity(frame.FrameID, frame.ReplayIdentity) {
		return errors.New("duplex authenticated wire identity is replayed")
	}
	exchange.wire.mu.Lock()
	prepared, err := exchange.wire.prepare(raw, frame.MessageKind, at)
	exchange.wire.mu.Unlock()
	if err != nil {
		return err
	}
	if !bytes.Equal(prepared.payload, frame.Payload) || prepared.frame.FrameFingerprint != frame.FrameFingerprint {
		return errors.New("duplex frame preparation changed authenticated content")
	}
	queuedFrames, queuedBytes, _, _ := nodeConnectorDuplexUsage(*directionState)
	limits := exchange.state.Config.Limits
	if queuedFrames+1 > limits.MaxQueuedFrames || queuedBytes+len(raw) > limits.MaxQueuedBytes {
		return errors.New("duplex queued frame or byte limit is exhausted")
	}
	next := cloneNodeConnectorDuplexState(exchange.state)
	nextDirection, _ := nodeConnectorDuplexDirection(&next, direction)
	nextDirection.AcceptedSequence = sequence
	nextDirection.Items = append(nextDirection.Items, nodeConnectorDuplexItem{
		Sequence: sequence, FrameID: frame.FrameID, ReplayIdentity: frame.ReplayIdentity,
		FrameFingerprint: frame.FrameFingerprint, FrameBytes: append([]byte{}, raw...), State: nodeConnectorDuplexQueued,
	})
	return exchange.persist(next)
}

// Deliver passes the next contiguous queued frames byte-exactly to one
// existing acceptance boundary. Only a successful callback moves them into
// the durable in-flight window; acknowledgement remains separate.
func (exchange *NodeConnectorDuplex) Deliver(direction string, count int, receiver NodeConnectorDuplexReceiver) error {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if receiver == nil || count < 1 {
		return errors.New("duplex delivery requires a bounded injected receiver")
	}
	if err := exchange.revalidateDurableState(); err != nil {
		return err
	}
	directionState, err := exchange.direction(direction)
	if err != nil {
		return err
	}
	_, _, inFlightFrames, inFlightBytes := nodeConnectorDuplexUsage(*directionState)
	frames := make([][]byte, 0, count)
	sequences := make([]int64, 0, count)
	batchBytes := 0
	nextSequence := directionState.DeliveredSequence + 1
	for _, item := range directionState.Items {
		if item.Sequence < nextSequence {
			continue
		}
		if item.Sequence != nextSequence || item.State != nodeConnectorDuplexQueued {
			return errors.New("duplex delivery frontier conflicts with queued ordering")
		}
		frames = append(frames, append([]byte{}, item.FrameBytes...))
		sequences = append(sequences, item.Sequence)
		batchBytes += len(item.FrameBytes)
		nextSequence++
		if len(frames) == count {
			break
		}
	}
	if len(frames) != count {
		return errors.New("duplex delivery requests frames beyond the accepted queue frontier")
	}
	limits := exchange.state.Config.Limits
	if inFlightFrames+count > limits.MaxInFlightFrames || inFlightBytes+batchBytes > limits.MaxInFlightBytes {
		return errors.New("duplex in-flight frame or byte limit is exhausted")
	}
	if err := receiver(frames); err != nil {
		return fmt.Errorf("duplex downstream acceptance failed: %w", err)
	}
	next := cloneNodeConnectorDuplexState(exchange.state)
	nextDirection, _ := nodeConnectorDuplexDirection(&next, direction)
	for index := range nextDirection.Items {
		for _, sequence := range sequences {
			if nextDirection.Items[index].Sequence == sequence {
				nextDirection.Items[index].State = nodeConnectorDuplexInFlight
			}
		}
	}
	nextDirection.DeliveredSequence = sequences[len(sequences)-1]
	return exchange.persist(next)
}

// Acknowledge advances exactly one ordered item after downstream acceptance.
// Credit and acknowledgement cannot create or refresh any other contract.
func (exchange *NodeConnectorDuplex) Acknowledge(direction string, sequence int64) error {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if err := exchange.revalidateDurableState(); err != nil {
		return err
	}
	directionState, err := exchange.direction(direction)
	if err != nil {
		return err
	}
	if sequence != directionState.AcknowledgedSequence+1 || sequence > directionState.DeliveredSequence {
		return errors.New("duplex acknowledgement is reordered or beyond the delivered frontier")
	}
	next := cloneNodeConnectorDuplexState(exchange.state)
	nextDirection, _ := nodeConnectorDuplexDirection(&next, direction)
	found := false
	for index := range nextDirection.Items {
		if nextDirection.Items[index].Sequence == sequence {
			if nextDirection.Items[index].State != nodeConnectorDuplexInFlight {
				return errors.New("duplex acknowledgement does not bind an in-flight frame")
			}
			nextDirection.Items[index].State = nodeConnectorDuplexAcknowledged
			found = true
			break
		}
	}
	if !found {
		return errors.New("duplex acknowledgement does not bind an accepted frame")
	}
	nextDirection.AcknowledgedSequence = sequence
	return exchange.persist(next)
}

func (exchange *NodeConnectorDuplex) Cursor(direction string) (NodeConnectorDuplexCursor, error) {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if err := exchange.revalidateDurableState(); err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	state, err := exchange.direction(direction)
	if err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	return exchange.cursor(direction, *state), nil
}

// Resume accepts only the exact durable cursor. Stale, ahead, substituted, or
// cross-direction cursors fail closed rather than replaying accepted frames.
func (exchange *NodeConnectorDuplex) Resume(cursor NodeConnectorDuplexCursor) (NodeConnectorDuplexSnapshot, error) {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if err := exchange.revalidateDurableState(); err != nil {
		return NodeConnectorDuplexSnapshot{}, err
	}
	state, err := exchange.direction(cursor.Direction)
	if err != nil {
		return NodeConnectorDuplexSnapshot{}, err
	}
	if !nodeExecutionEqual(cursor, exchange.cursor(cursor.Direction, *state)) {
		return NodeConnectorDuplexSnapshot{}, errors.New("duplex resume cursor conflicts with durable exchange state")
	}
	return nodeConnectorDuplexSnapshot(exchange.state), nil
}

func (exchange *NodeConnectorDuplex) Snapshot() (NodeConnectorDuplexSnapshot, error) {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if err := exchange.revalidateDurableState(); err != nil {
		return NodeConnectorDuplexSnapshot{}, err
	}
	return nodeConnectorDuplexSnapshot(exchange.state), nil
}

func (exchange *NodeConnectorDuplex) cursor(direction string, state nodeConnectorDuplexDirectionState) NodeConnectorDuplexCursor {
	return NodeConnectorDuplexCursor{
		ExchangeID: exchange.state.Config.ExchangeID, ConfigFingerprint: exchange.state.Config.ConfigFingerprint, Direction: direction,
		AcceptedSequence: state.AcceptedSequence, DeliveredSequence: state.DeliveredSequence, AcknowledgedSequence: state.AcknowledgedSequence,
	}
}

func (exchange *NodeConnectorDuplex) direction(direction string) (*nodeConnectorDuplexDirectionState, error) {
	return nodeConnectorDuplexDirection(&exchange.state, direction)
}

func nodeConnectorDuplexDirection(state *nodeConnectorDuplexState, direction string) (*nodeConnectorDuplexDirectionState, error) {
	switch direction {
	case NodeConnectorWireConnectorToBroker:
		return &state.Directions.ConnectorToBroker, nil
	case NodeConnectorWireBrokerToConnector:
		return &state.Directions.BrokerToConnector, nil
	default:
		return nil, errors.New("duplex direction is unsupported")
	}
}

func (exchange *NodeConnectorDuplex) hasWireIdentity(frameID, replayIdentity string) bool {
	for _, direction := range []*nodeConnectorDuplexDirectionState{&exchange.state.Directions.ConnectorToBroker, &exchange.state.Directions.BrokerToConnector} {
		for _, item := range direction.Items {
			if item.FrameID == frameID || item.ReplayIdentity == replayIdentity {
				return true
			}
		}
	}
	return false
}

func (exchange *NodeConnectorDuplex) persist(next nodeConnectorDuplexState) error {
	next.Generation = exchange.state.Generation + 1
	next.PreviousStateFingerprint = exchange.state.StateFingerprint
	next.StateFingerprint = ""
	if err := finalizeNodeConnectorDuplexState(&next); err != nil {
		return err
	}
	path := filepath.Join(exchange.root, nodeConnectorDuplexStateFileName(next.Generation))
	if _, err := os.Lstat(path); err == nil {
		return errors.New("next duplex state artifact already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := nodeConnectorDuplexWriteAtomic(path, next); err != nil {
		return err
	}
	exchange.state = next
	return nil
}

func (exchange *NodeConnectorDuplex) revalidateDurableState() error {
	states, err := loadNodeConnectorDuplexStates(exchange.root)
	if err != nil {
		return fmt.Errorf("duplex durable state failed revalidation: %w", err)
	}
	if len(states) == 0 || states[len(states)-1].StateFingerprint != exchange.state.StateFingerprint || !nodeExecutionEqual(states[len(states)-1].Config, exchange.state.Config) {
		return errors.New("duplex durable state is missing, stale, or identity-conflicted")
	}
	return nil
}

func validateNodeConnectorDuplexConfig(config NodeConnectorDuplexConfig) error {
	if config.Schema != NodeConnectorDuplexConfigSchema {
		return errors.New("duplex configuration schema is unsupported")
	}
	for _, value := range []struct{ kind, id string }{{"exchange", config.ExchangeID}, {"peer", config.ConnectorPeerID}, {"peer", config.BrokerPeerID}} {
		if err := validateNodeExecutionTypedID(value.kind, value.id); err != nil {
			return err
		}
	}
	if config.ExchangeID == config.ConnectorPeerID || config.ExchangeID == config.BrokerPeerID || config.ConnectorPeerID == config.BrokerPeerID {
		return errors.New("duplex exchange and peer identities must remain distinct")
	}
	limits := config.Limits
	if limits.MaxQueuedFrames < 1 || limits.MaxQueuedFrames > nodeConnectorDuplexMaxItems || limits.MaxInFlightFrames < 1 || limits.MaxInFlightFrames > nodeConnectorDuplexMaxItems ||
		limits.MaxFrameBytes < 1 || limits.MaxFrameBytes > NodeConnectorWireMaxBytes || limits.MaxQueuedBytes < limits.MaxFrameBytes || limits.MaxInFlightBytes < limits.MaxFrameBytes ||
		limits.MaxQueuedBytes > nodeConnectorDuplexMaxItems*NodeConnectorWireMaxBytes || limits.MaxInFlightBytes > nodeConnectorDuplexMaxItems*NodeConnectorWireMaxBytes {
		return errors.New("duplex frame and byte limits are invalid or unbounded")
	}
	expected := config
	expected.ConfigFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if config.ConfigFingerprint != fingerprint {
		return errors.New("duplex configuration fingerprint does not bind immutable limits")
	}
	return nil
}

func finalizeNodeConnectorDuplexState(state *nodeConnectorDuplexState) error {
	state.StateFingerprint = ""
	fingerprint, err := nodeExecutionFingerprintValue(*state)
	if err != nil {
		return err
	}
	state.StateFingerprint = fingerprint
	return validateNodeConnectorDuplexState(*state)
}

func validateNodeConnectorDuplexState(state nodeConnectorDuplexState) error {
	if state.Schema != nodeConnectorDuplexStateSchema || state.Generation < 1 {
		return errors.New("duplex state schema or generation is invalid")
	}
	if err := validateNodeConnectorDuplexConfig(state.Config); err != nil {
		return err
	}
	if state.Generation == 1 && state.PreviousStateFingerprint != "" || state.Generation > 1 && !nodeExecutionFingerprint.MatchString(state.PreviousStateFingerprint) {
		return errors.New("duplex previous state fingerprint is invalid")
	}
	frameIDs, replayIDs := map[string]bool{}, map[string]bool{}
	for _, value := range []struct {
		direction string
		state     nodeConnectorDuplexDirectionState
	}{{NodeConnectorWireConnectorToBroker, state.Directions.ConnectorToBroker}, {NodeConnectorWireBrokerToConnector, state.Directions.BrokerToConnector}} {
		if err := validateNodeConnectorDuplexDirectionState(value.direction, value.state, state.Config.Limits, frameIDs, replayIDs); err != nil {
			return err
		}
	}
	expected := state
	expected.StateFingerprint = ""
	fingerprint, _ := nodeExecutionFingerprintValue(expected)
	if state.StateFingerprint != fingerprint {
		return errors.New("duplex state fingerprint does not bind durable content")
	}
	return nil
}

func validateNodeConnectorDuplexDirectionState(direction string, state nodeConnectorDuplexDirectionState, limits NodeConnectorDuplexLimits, frameIDs, replayIDs map[string]bool) error {
	if state.AcknowledgedSequence < 0 || state.DeliveredSequence < state.AcknowledgedSequence || state.AcceptedSequence < state.DeliveredSequence || state.AcceptedSequence > nodeConnectorDuplexMaxItems || int64(len(state.Items)) != state.AcceptedSequence {
		return errors.New("duplex sequence frontiers or tracked item count are invalid")
	}
	for index, item := range state.Items {
		sequence := int64(index + 1)
		if item.Sequence != sequence || len(item.FrameBytes) == 0 || len(item.FrameBytes) > limits.MaxFrameBytes {
			return errors.New("duplex durable item ordering or size is invalid")
		}
		var frame NodeConnectorWireFrame
		if err := decodeNodeExecutionCanonical(item.FrameBytes, &frame); err != nil {
			return fmt.Errorf("duplex durable frame is malformed or noncanonical: %w", err)
		}
		if frame.Direction != direction || frame.FrameID != item.FrameID || frame.ReplayIdentity != item.ReplayIdentity || frame.FrameFingerprint != item.FrameFingerprint {
			return errors.New("duplex durable frame direction or immutable identity is substituted")
		}
		if frameIDs[item.FrameID] || replayIDs[item.ReplayIdentity] {
			return errors.New("duplex durable frame or replay identity is duplicated")
		}
		frameIDs[item.FrameID], replayIDs[item.ReplayIdentity] = true, true
		expectedState := nodeConnectorDuplexQueued
		if sequence <= state.AcknowledgedSequence {
			expectedState = nodeConnectorDuplexAcknowledged
		} else if sequence <= state.DeliveredSequence {
			expectedState = nodeConnectorDuplexInFlight
		}
		if item.State != expectedState {
			return errors.New("duplex durable item state conflicts with sequence frontiers")
		}
	}
	queuedFrames, queuedBytes, inFlightFrames, inFlightBytes := nodeConnectorDuplexUsage(state)
	if queuedFrames > limits.MaxQueuedFrames || queuedBytes > limits.MaxQueuedBytes || inFlightFrames > limits.MaxInFlightFrames || inFlightBytes > limits.MaxInFlightBytes {
		return errors.New("duplex durable state exceeds configured flow-control limits")
	}
	return nil
}

func nodeConnectorDuplexUsage(state nodeConnectorDuplexDirectionState) (queuedFrames, queuedBytes, inFlightFrames, inFlightBytes int) {
	for _, item := range state.Items {
		switch item.State {
		case nodeConnectorDuplexQueued:
			queuedFrames++
			queuedBytes += len(item.FrameBytes)
		case nodeConnectorDuplexInFlight:
			inFlightFrames++
			inFlightBytes += len(item.FrameBytes)
		}
	}
	return
}

func nodeConnectorDuplexSnapshot(state nodeConnectorDuplexState) NodeConnectorDuplexSnapshot {
	convert := func(direction nodeConnectorDuplexDirectionState) NodeConnectorDuplexDirectionSnapshot {
		queuedFrames, queuedBytes, inFlightFrames, inFlightBytes := nodeConnectorDuplexUsage(direction)
		return NodeConnectorDuplexDirectionSnapshot{
			AcceptedSequence: direction.AcceptedSequence, DeliveredSequence: direction.DeliveredSequence, AcknowledgedSequence: direction.AcknowledgedSequence,
			QueuedFrames: queuedFrames, QueuedBytes: queuedBytes, InFlightFrames: inFlightFrames, InFlightBytes: inFlightBytes,
		}
	}
	return NodeConnectorDuplexSnapshot{
		ExchangeID: state.Config.ExchangeID, ConfigFingerprint: state.Config.ConfigFingerprint,
		ConnectorToBroker: convert(state.Directions.ConnectorToBroker), BrokerToConnector: convert(state.Directions.BrokerToConnector),
	}
}

func loadNodeConnectorDuplexStates(root string) ([]nodeConnectorDuplexState, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if bytes.HasPrefix([]byte(entry.Name()), []byte("connector-duplex-state-")) {
			if !nodeConnectorDuplexStateName.MatchString(entry.Name()) {
				return nil, fmt.Errorf("malformed duplex state artifact name %q", entry.Name())
			}
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	states := make([]nodeConnectorDuplexState, 0, len(names))
	previous := ""
	var config NodeConnectorDuplexConfig
	for index, name := range names {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		var state nodeConnectorDuplexState
		if err := decodeNodeExecutionStrict(raw, &state); err != nil {
			return nil, fmt.Errorf("duplex state %s is malformed: %w", name, err)
		}
		if state.Generation != int64(index+1) || name != nodeConnectorDuplexStateFileName(state.Generation) || state.PreviousStateFingerprint != previous {
			return nil, fmt.Errorf("duplex state chain is broken at %s", name)
		}
		if err := validateNodeConnectorDuplexState(state); err != nil {
			return nil, fmt.Errorf("duplex state %s failed revalidation: %w", name, err)
		}
		if index == 0 {
			config = state.Config
		} else if !nodeExecutionEqual(config, state.Config) {
			return nil, fmt.Errorf("duplex state %s changes immutable configuration", name)
		}
		previous = state.StateFingerprint
		states = append(states, state)
	}
	return states, nil
}

func nodeConnectorDuplexStateFileName(generation int64) string {
	return fmt.Sprintf("connector-duplex-state-%012d.json", generation)
}

func cloneNodeConnectorDuplexState(state nodeConnectorDuplexState) nodeConnectorDuplexState {
	raw, _ := json.Marshal(state)
	var cloned nodeConnectorDuplexState
	_ = decodeNodeExecutionStrict(raw, &cloned)
	return cloned
}

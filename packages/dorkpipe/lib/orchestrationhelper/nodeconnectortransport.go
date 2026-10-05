package orchestrationhelper

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

const (
	NodeConnectorTransportRecordSchema = "dorkpipe.node-connector.transport-record/v1"

	nodeConnectorTransportResume = "resume"
	nodeConnectorTransportFrame  = "frame"
	nodeConnectorTransportAck    = "acknowledgement"

	nodeConnectorTransportMinRecordBytes = NodeConnectorWireMaxBytes + 1024
	nodeConnectorTransportMaxRecordBytes = 2 * NodeConnectorWireMaxBytes
	nodeConnectorTransportMaxTimeout     = time.Minute
)

// NodeConnectorTransportLimits bounds connection establishment and every
// individual record operation. The limits carry no execution authority.
type NodeConnectorTransportLimits struct {
	MaxRecordBytes int
	ConnectTimeout time.Duration
	IOTimeout      time.Duration
}

// NodeConnectorTransportBrokerListener is the only listener-owning transport
// role. It binds one explicit numeric loopback IP at an ephemeral port.
type NodeConnectorTransportBrokerListener struct {
	listener net.Listener
	limits   NodeConnectorTransportLimits
}

// NodeConnectorTransportConnector owns only outbound dialing behavior.
type NodeConnectorTransportConnector struct {
	limits NodeConnectorTransportLimits
}

type nodeConnectorTransportRecord struct {
	Schema            string                     `json:"schema"`
	Kind              string                     `json:"kind"`
	ExchangeID        string                     `json:"exchange_id"`
	ConfigFingerprint string                     `json:"config_fingerprint"`
	Direction         string                     `json:"direction"`
	Sequence          int64                      `json:"sequence"`
	Cursor            *NodeConnectorDuplexCursor `json:"cursor,omitempty"`
	Frame             json.RawMessage            `json:"frame,omitempty"`
}

func NewNodeConnectorTransportBrokerListener(endpoint string, limits NodeConnectorTransportLimits) (*NodeConnectorTransportBrokerListener, error) {
	if err := validateNodeConnectorTransportLimits(limits); err != nil {
		return nil, err
	}
	network, address, err := validateNodeConnectorTransportEndpoint(endpoint, true)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, fmt.Errorf("loopback broker listener failed: %w", err)
	}
	bound, ok := listener.Addr().(*net.TCPAddr)
	if !ok || bound.IP == nil || !bound.IP.IsLoopback() || bound.IP.IsUnspecified() || bound.Port < 1 {
		_ = listener.Close()
		return nil, errors.New("broker listener did not bind an explicit ephemeral loopback endpoint")
	}
	return &NodeConnectorTransportBrokerListener{listener: listener, limits: limits}, nil
}

func NewNodeConnectorTransportConnector(limits NodeConnectorTransportLimits) (*NodeConnectorTransportConnector, error) {
	if err := validateNodeConnectorTransportLimits(limits); err != nil {
		return nil, err
	}
	return &NodeConnectorTransportConnector{limits: limits}, nil
}

func (broker *NodeConnectorTransportBrokerListener) Endpoint() string {
	return broker.listener.Addr().String()
}

func (broker *NodeConnectorTransportBrokerListener) Accept() (net.Conn, error) {
	if tcp, ok := broker.listener.(*net.TCPListener); ok {
		if err := tcp.SetDeadline(time.Now().Add(broker.limits.ConnectTimeout)); err != nil {
			return nil, err
		}
	}
	connection, err := broker.listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("loopback broker accept failed: %w", err)
	}
	remote, ok := connection.RemoteAddr().(*net.TCPAddr)
	if !ok || remote.IP == nil || !remote.IP.IsLoopback() || remote.IP.IsUnspecified() {
		_ = connection.Close()
		return nil, errors.New("broker rejected a non-loopback connector peer")
	}
	return connection, nil
}

func (broker *NodeConnectorTransportBrokerListener) Close() error {
	return broker.listener.Close()
}

func (connector *NodeConnectorTransportConnector) Dial(ctx context.Context, endpoint string) (net.Conn, error) {
	network, address, err := validateNodeConnectorTransportEndpoint(endpoint, false)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: connector.limits.ConnectTimeout}
	connection, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("outbound loopback connector dial failed: %w", err)
	}
	remote, ok := connection.RemoteAddr().(*net.TCPAddr)
	if !ok || remote.IP == nil || !remote.IP.IsLoopback() || remote.IP.IsUnspecified() {
		_ = connection.Close()
		return nil, errors.New("connector dial resolved outside explicit loopback")
	}
	return connection, nil
}

func nodeConnectorTransportWriteResume(connection net.Conn, cursor NodeConnectorDuplexCursor, limits NodeConnectorTransportLimits) error {
	record := nodeConnectorTransportRecord{
		Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportResume,
		ExchangeID: cursor.ExchangeID, ConfigFingerprint: cursor.ConfigFingerprint,
		Direction: cursor.Direction, Sequence: cursor.AcknowledgedSequence, Cursor: &cursor,
	}
	return writeNodeConnectorTransportRecord(connection, record, limits)
}

func nodeConnectorTransportAcceptResume(connection net.Conn, exchange *NodeConnectorDuplex, direction string, limits NodeConnectorTransportLimits) (NodeConnectorDuplexCursor, error) {
	record, err := readNodeConnectorTransportRecord(connection, limits)
	if err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	if record.Kind != nodeConnectorTransportResume || record.Cursor == nil || len(record.Frame) != 0 || record.Direction != direction || record.Sequence != record.Cursor.AcknowledgedSequence {
		return NodeConnectorDuplexCursor{}, errors.New("transport resume record shape or direction is invalid")
	}
	if _, err := exchange.Resume(*record.Cursor); err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	return *record.Cursor, nil
}

func nodeConnectorTransportWriteFrame(connection net.Conn, config NodeConnectorDuplexConfig, direction string, sequence int64, frame []byte, limits NodeConnectorTransportLimits) error {
	record := nodeConnectorTransportRecord{
		Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportFrame,
		ExchangeID: config.ExchangeID, ConfigFingerprint: config.ConfigFingerprint,
		Direction: direction, Sequence: sequence, Frame: append(json.RawMessage{}, frame...),
	}
	return writeNodeConnectorTransportRecord(connection, record, limits)
}

// nodeConnectorTransportAcceptFrame advances transport acknowledgement only
// after the existing duplex and downstream wire/session boundary accept the
// byte-exact frame. Rejection sends no acknowledgement record.
func nodeConnectorTransportAcceptFrame(connection net.Conn, exchange *NodeConnectorDuplex, at time.Time, receiver NodeConnectorDuplexReceiver, limits NodeConnectorTransportLimits) (NodeConnectorDuplexCursor, error) {
	return nodeConnectorTransportAcceptFrames(connection, exchange, 1, at, receiver, limits)
}

func nodeConnectorTransportAcceptFrames(connection net.Conn, exchange *NodeConnectorDuplex, count int, at time.Time, receiver NodeConnectorDuplexReceiver, limits NodeConnectorTransportLimits) (NodeConnectorDuplexCursor, error) {
	if count < 1 || count > exchange.state.Config.Limits.MaxQueuedFrames {
		return NodeConnectorDuplexCursor{}, errors.New("transport frame batch count is invalid or unbounded")
	}
	records := make([]nodeConnectorTransportRecord, 0, count)
	for index := 0; index < count; index++ {
		record, err := readNodeConnectorTransportRecord(connection, limits)
		if err != nil {
			return NodeConnectorDuplexCursor{}, err
		}
		if record.Kind != nodeConnectorTransportFrame || record.Cursor != nil || len(record.Frame) == 0 {
			return NodeConnectorDuplexCursor{}, errors.New("transport frame record shape is invalid")
		}
		if record.ExchangeID != exchange.state.Config.ExchangeID || record.ConfigFingerprint != exchange.state.Config.ConfigFingerprint {
			return NodeConnectorDuplexCursor{}, errors.New("transport frame substitutes exchange identity or configuration")
		}
		if index != 0 && record.Direction != records[0].Direction {
			return NodeConnectorDuplexCursor{}, errors.New("transport frame batch substitutes direction")
		}
		records = append(records, record)
	}
	next, baseFingerprint, err := exchange.prepareTransportFrames(records, at)
	if err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	direction := records[0].Direction
	frames := make([][]byte, 0, len(records))
	for _, record := range records {
		frames = append(frames, append([]byte{}, record.Frame...))
	}
	if receiver == nil {
		return NodeConnectorDuplexCursor{}, errors.New("transport frame delivery requires the existing downstream acceptance boundary")
	}
	if err := receiver(frames); err != nil {
		return NodeConnectorDuplexCursor{}, fmt.Errorf("transport downstream acceptance failed: %w", err)
	}
	if err := exchange.commitTransportFrames(next, baseFingerprint); err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	if err := exchange.Deliver(direction, count, func([][]byte) error { return nil }); err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	if err := exchange.acknowledgeTransportFrames(direction, records); err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	cursor, err := exchange.Cursor(direction)
	if err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	acknowledgement := nodeConnectorTransportRecord{
		Schema: NodeConnectorTransportRecordSchema, Kind: nodeConnectorTransportAck,
		ExchangeID: cursor.ExchangeID, ConfigFingerprint: cursor.ConfigFingerprint,
		Direction: cursor.Direction, Sequence: records[len(records)-1].Sequence, Cursor: &cursor,
	}
	if err := writeNodeConnectorTransportRecord(connection, acknowledgement, limits); err != nil {
		return NodeConnectorDuplexCursor{}, err
	}
	return cursor, nil
}

func (exchange *NodeConnectorDuplex) prepareTransportFrames(records []nodeConnectorTransportRecord, at time.Time) (nodeConnectorDuplexState, string, error) {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if err := exchange.revalidateDurableState(); err != nil {
		return nodeConnectorDuplexState{}, "", err
	}
	direction := records[0].Direction
	directionState, err := exchange.direction(direction)
	if err != nil {
		return nodeConnectorDuplexState{}, "", err
	}
	queuedFrames, queuedBytes, _, _ := nodeConnectorDuplexUsage(*directionState)
	seenFrames, seenReplays := map[string]bool{}, map[string]bool{}
	totalBytes := 0
	for index, record := range records {
		if record.Sequence != directionState.AcceptedSequence+int64(index)+1 {
			return nodeConnectorDuplexState{}, "", errors.New("transport frame sequence is gapped, regressed, or duplicated")
		}
		if len(record.Frame) > exchange.state.Config.Limits.MaxFrameBytes {
			return nodeConnectorDuplexState{}, "", errors.New("transport frame exceeds the duplex frame limit")
		}
		var frame NodeConnectorWireFrame
		if err := decodeNodeExecutionCanonical(record.Frame, &frame); err != nil {
			return nodeConnectorDuplexState{}, "", fmt.Errorf("transport authenticated frame is malformed or noncanonical: %w", err)
		}
		if frame.Direction != direction || exchange.hasWireIdentity(frame.FrameID, frame.ReplayIdentity) || seenFrames[frame.FrameID] || seenReplays[frame.ReplayIdentity] {
			return nodeConnectorDuplexState{}, "", errors.New("transport authenticated frame direction or replay identity is invalid")
		}
		exchange.wire.mu.Lock()
		prepared, prepareErr := exchange.wire.prepare(record.Frame, frame.MessageKind, at)
		exchange.wire.mu.Unlock()
		if prepareErr != nil {
			return nodeConnectorDuplexState{}, "", prepareErr
		}
		if prepared.frame.FrameFingerprint != frame.FrameFingerprint {
			return nodeConnectorDuplexState{}, "", errors.New("transport changed authenticated frame content")
		}
		seenFrames[frame.FrameID], seenReplays[frame.ReplayIdentity] = true, true
		totalBytes += len(record.Frame)
	}
	limits := exchange.state.Config.Limits
	if queuedFrames+len(records) > limits.MaxQueuedFrames || queuedBytes+totalBytes > limits.MaxQueuedBytes {
		return nodeConnectorDuplexState{}, "", errors.New("transport frame batch exceeds duplex queued flow control")
	}
	next := cloneNodeConnectorDuplexState(exchange.state)
	nextDirection, _ := nodeConnectorDuplexDirection(&next, direction)
	for _, record := range records {
		var frame NodeConnectorWireFrame
		_ = decodeNodeExecutionCanonical(record.Frame, &frame)
		nextDirection.AcceptedSequence = record.Sequence
		nextDirection.Items = append(nextDirection.Items, nodeConnectorDuplexItem{
			Sequence: record.Sequence, FrameID: frame.FrameID, ReplayIdentity: frame.ReplayIdentity,
			FrameFingerprint: frame.FrameFingerprint, FrameBytes: append([]byte{}, record.Frame...), State: nodeConnectorDuplexQueued,
		})
	}
	return next, exchange.state.StateFingerprint, nil
}

func (exchange *NodeConnectorDuplex) commitTransportFrames(next nodeConnectorDuplexState, baseFingerprint string) error {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if err := exchange.revalidateDurableState(); err != nil {
		return err
	}
	if exchange.state.StateFingerprint != baseFingerprint {
		return errors.New("duplex state changed during transport downstream acceptance")
	}
	return exchange.persist(next)
}

func (exchange *NodeConnectorDuplex) acknowledgeTransportFrames(direction string, records []nodeConnectorTransportRecord) error {
	exchange.mu.Lock()
	defer exchange.mu.Unlock()
	if err := exchange.revalidateDurableState(); err != nil {
		return err
	}
	directionState, err := exchange.direction(direction)
	if err != nil {
		return err
	}
	if records[0].Sequence != directionState.AcknowledgedSequence+1 || records[len(records)-1].Sequence > directionState.DeliveredSequence {
		return errors.New("transport acknowledgement batch is reordered or beyond delivery")
	}
	next := cloneNodeConnectorDuplexState(exchange.state)
	nextDirection, _ := nodeConnectorDuplexDirection(&next, direction)
	for _, record := range records {
		item := &nextDirection.Items[record.Sequence-1]
		if item.Sequence != record.Sequence || item.State != nodeConnectorDuplexInFlight {
			return errors.New("transport acknowledgement does not bind in-flight frames")
		}
		item.State = nodeConnectorDuplexAcknowledged
		nextDirection.AcknowledgedSequence = record.Sequence
	}
	return exchange.persist(next)
}

func nodeConnectorTransportReadAcknowledgement(connection net.Conn, expected NodeConnectorDuplexCursor, sequence int64, limits NodeConnectorTransportLimits) error {
	record, err := readNodeConnectorTransportRecord(connection, limits)
	if err != nil {
		return err
	}
	if record.Kind != nodeConnectorTransportAck || record.Cursor == nil || len(record.Frame) != 0 || record.Sequence != sequence || !nodeExecutionEqual(*record.Cursor, expected) {
		return errors.New("transport acknowledgement is stale, ahead, substituted, or reordered")
	}
	return nil
}

func writeNodeConnectorTransportRecord(connection net.Conn, record nodeConnectorTransportRecord, limits NodeConnectorTransportLimits) error {
	if connection == nil {
		return errors.New("transport record requires a connection")
	}
	if err := validateNodeConnectorTransportRecord(record); err != nil {
		return err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(raw) == 0 || len(raw) > limits.MaxRecordBytes {
		return errors.New("transport record exceeds the configured size bound")
	}
	if err := connection.SetWriteDeadline(time.Now().Add(limits.IOTimeout)); err != nil {
		return err
	}
	prefix := make([]byte, 4)
	binary.BigEndian.PutUint32(prefix, uint32(len(raw)))
	if err := writeNodeConnectorTransportBytes(connection, prefix); err != nil {
		return fmt.Errorf("transport record prefix write failed: %w", err)
	}
	if err := writeNodeConnectorTransportBytes(connection, raw); err != nil {
		return fmt.Errorf("transport record body write failed: %w", err)
	}
	return nil
}

func readNodeConnectorTransportRecord(connection net.Conn, limits NodeConnectorTransportLimits) (nodeConnectorTransportRecord, error) {
	if connection == nil {
		return nodeConnectorTransportRecord{}, errors.New("transport record requires a connection")
	}
	if err := connection.SetReadDeadline(time.Now().Add(limits.IOTimeout)); err != nil {
		return nodeConnectorTransportRecord{}, err
	}
	prefix := make([]byte, 4)
	if _, err := io.ReadFull(connection, prefix); err != nil {
		return nodeConnectorTransportRecord{}, fmt.Errorf("transport record prefix is empty, partial, or timed out: %w", err)
	}
	length := binary.BigEndian.Uint32(prefix)
	if length == 0 || uint64(length) > uint64(limits.MaxRecordBytes) {
		return nodeConnectorTransportRecord{}, errors.New("transport record length is empty or exceeds the configured bound")
	}
	raw := make([]byte, int(length))
	if _, err := io.ReadFull(connection, raw); err != nil {
		return nodeConnectorTransportRecord{}, fmt.Errorf("transport record body is truncated or timed out: %w", err)
	}
	var record nodeConnectorTransportRecord
	if err := decodeNodeExecutionCanonical(raw, &record); err != nil {
		return nodeConnectorTransportRecord{}, fmt.Errorf("transport record is malformed, noncanonical, or has trailing data: %w", err)
	}
	if err := validateNodeConnectorTransportRecord(record); err != nil {
		return nodeConnectorTransportRecord{}, err
	}
	return record, nil
}

func writeNodeConnectorTransportBytes(connection net.Conn, raw []byte) error {
	for len(raw) != 0 {
		written, err := connection.Write(raw)
		if err != nil {
			return err
		}
		if written < 1 {
			return io.ErrUnexpectedEOF
		}
		raw = raw[written:]
	}
	return nil
}

func validateNodeConnectorTransportRecord(record nodeConnectorTransportRecord) error {
	if record.Schema != NodeConnectorTransportRecordSchema {
		return errors.New("transport record schema is unsupported")
	}
	if err := validateNodeExecutionTypedID("exchange", record.ExchangeID); err != nil {
		return err
	}
	if !nodeExecutionFingerprint.MatchString(record.ConfigFingerprint) {
		return errors.New("transport record configuration fingerprint is invalid")
	}
	if record.Direction != NodeConnectorWireConnectorToBroker && record.Direction != NodeConnectorWireBrokerToConnector {
		return errors.New("transport record direction is unsupported")
	}
	if record.Sequence < 0 || record.Sequence > nodeConnectorDuplexMaxItems {
		return errors.New("transport record sequence is invalid or unbounded")
	}
	switch record.Kind {
	case nodeConnectorTransportResume, nodeConnectorTransportAck:
		if record.Cursor == nil || len(record.Frame) != 0 || record.Cursor.ExchangeID != record.ExchangeID || record.Cursor.ConfigFingerprint != record.ConfigFingerprint || record.Cursor.Direction != record.Direction {
			return errors.New("transport cursor record does not bind its exchange, configuration, and direction")
		}
	case nodeConnectorTransportFrame:
		if record.Cursor != nil || record.Sequence < 1 || len(record.Frame) == 0 || len(record.Frame) > NodeConnectorWireMaxBytes {
			return errors.New("transport frame record is empty, oversized, or carries cursor authority")
		}
	default:
		return errors.New("transport record kind is unsupported")
	}
	return nil
}

func validateNodeConnectorTransportLimits(limits NodeConnectorTransportLimits) error {
	if limits.MaxRecordBytes < nodeConnectorTransportMinRecordBytes || limits.MaxRecordBytes > nodeConnectorTransportMaxRecordBytes ||
		limits.ConnectTimeout <= 0 || limits.ConnectTimeout > nodeConnectorTransportMaxTimeout || limits.IOTimeout <= 0 || limits.IOTimeout > nodeConnectorTransportMaxTimeout {
		return errors.New("transport record and timeout limits are invalid or unbounded")
	}
	return nil
}

func validateNodeConnectorTransportEndpoint(endpoint string, listener bool) (string, string, error) {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" || portText == "" {
		return "", "", errors.New("transport endpoint must be an explicit numeric loopback IP and port")
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || !ip.IsLoopback() {
		return "", "", errors.New("transport endpoint rejects hostnames, wildcard, unspecified, and non-loopback addresses")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 || listener && port != 0 || !listener && port == 0 {
		return "", "", errors.New("broker must use an ephemeral port and connector must use the resulting explicit port")
	}
	network := "tcp6"
	if ip.To4() != nil {
		network = "tcp4"
	}
	return network, net.JoinHostPort(ip.String(), strconv.Itoa(port)), nil
}

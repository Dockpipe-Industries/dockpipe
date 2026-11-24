package remote

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"

	"github.com/coder/websocket"
)

const desktopRelayTicketHeader = "X-Dockpipe-Desktop-Ticket"
const desktopRelayPairTimeout = 10 * time.Second
const desktopRelayIdleTimeout = 30 * time.Second
const desktopRelayWriteTimeout = 5 * time.Second

type desktopRelayTicket struct {
	request contract.DesktopRelayRequest
	hash    string
	expires time.Time
}

type desktopRelay struct {
	context       context.Context
	cancel        context.CancelFunc
	ready         chan struct{}
	tickets       [2]*desktopRelayTicket
	sockets       [2]*websocket.Conn
	reserved      [2]bool
	retired       bool
	watching      bool
	paired        bool
	started       time.Time
	administrator bool
}

func desktopRelayRole(role string) int {
	if role == "viewer" {
		return 0
	}
	return 1
}

// desktopRelayTicket executes under the existing broker request mutex after
// operator OS admission or the worker's single-use signed machine challenge.
func (broker *Broker) desktopRelayTicket(raw []byte, role, node string, admission operatorAdmission) (any, error) {
	var request contract.DesktopRelayRequest
	if decodeFeature(raw, &request) != nil || request.Validate() != nil || request.Role != role {
		return nil, errors.New("invalid desktop relay ticket request")
	}
	state := broker.desktopState()
	record := state.records[request.Reference.ID]
	if record == nil || request.Reference.Epoch != state.epoch || record.open.Reference() != request.Reference || broker.desktopLease(record) <= 0 ||
		role == "viewer" && !desktopOwner(record, admission) || role == "worker" && request.Reference.Target.Node != node {
		return nil, errors.New("desktop relay ticket lacks current exact session authority")
	}
	index := desktopRelayRole(role)
	local, peer := record.viewer.fingerprint, record.worker.fingerprint
	if index == 1 {
		local, peer = peer, local
	}
	if local == "" || peer == "" || request.LocalFingerprint != local || request.PeerFingerprint != peer {
		return nil, errors.New("desktop relay endpoint identities were not published by both authenticated peers")
	}
	if record.relay == nil {
		ctx, cancel := context.WithDeadline(context.Background(), record.open.Deadline)
		record.relay = &desktopRelay{context: ctx, cancel: cancel, ready: make(chan struct{})}
	}
	expires := minDesktopTime(record.open.Deadline, time.Now().Add(contract.DesktopRelayTicketLifetime))
	return issueDesktopRelayTicket(record, record.relay, request, expires)
}

func issueDesktopRelayTicket(record *desktopRecord, relay *desktopRelay, request contract.DesktopRelayRequest,
	expires time.Time) (contract.DesktopRelayTicket, error) {
	index := desktopRelayRole(request.Role)
	if relay.retired || relay.reserved[index] || !expires.After(time.Now()) {
		return contract.DesktopRelayTicket{}, errors.New("desktop relay endpoint is already attached or unavailable")
	}
	secret, err := Secret()
	if err != nil {
		return contract.DesktopRelayTicket{}, err
	}

	// A reissued ticket invalidates any unused ticket for the same endpoint.
	relay.tickets[index] = &desktopRelayTicket{request: request, hash: Hash(secret), expires: expires}
	return contract.DesktopRelayTicket{Request: request, Secret: secret, ExpiresAt: expires,
		Mode: record.open.Mode, Provider: record.open.Provider, LeaseMillis: time.Until(expires).Milliseconds()}, nil
}

func (record *desktopRecord) relayFor(administrator bool) *desktopRelay {
	if administrator {
		return record.administratorRelay
	}
	return record.relay
}

func (broker *Broker) desktopRelayCurrent(record *desktopRecord, relay *desktopRelay) bool {
	return !broker.poisoned && broker.desktops != nil && broker.desktops.records[record.open.ID] == record &&
		record.relayFor(relay.administrator) == relay && broker.desktopLease(record) > 0 &&
		relay.context.Err() == nil && !relay.retired &&
		(!relay.administrator || record.administrator != nil && record.administrator.until.After(time.Now()) &&
			broker.desktopAdministratorEligible(record, record.open.Reference()))
}

// GET is admitted only on this fixed upgrade route. All ordinary JSON routes
// retain POST, existing request timeouts, and their existing authentication.
func (broker *Broker) serveDesktopRelay(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet || request.URL.RawQuery != "" || request.ContentLength > 0 ||
		len(request.Header.Values("Authorization")) != 1 || len(request.Header.Values(desktopRelayTicketHeader)) != 1 || request.Header.Get("Origin") != "" {
		http.Error(w, "invalid native desktop relay attachment", http.StatusBadRequest)
		return
	}
	secret := request.Header.Get(desktopRelayTicketHeader)
	if !validMachineNonce(secret) {
		http.Error(w, "desktop relay authorization required", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	broker.mutex.Lock()
	administrator := request.URL.Path == contract.DesktopAdministratorRelayPath
	var selected *desktopRecord
	var index int
	if !broker.poisoned && broker.desktops != nil {
		for _, record := range broker.desktops.records {
			relay := record.relayFor(administrator)
			if relay == nil || relay.retired {
				continue
			}
			for role, ticket := range relay.tickets {
				if ticket != nil && matches(secret, ticket.hash) {
					selected, index = record, role
				}
			}
		}
	}
	if selected == nil {
		broker.mutex.Unlock()
		http.Error(w, "desktop relay ticket is unavailable", http.StatusUnauthorized)
		return
	}
	relay := selected.relayFor(administrator)
	ticket := relay.tickets[index]
	// Every attempt consumes the one-use ticket, including a bad bearer or
	// failed upgrade. A timeout/lost response never permits implicit reattach.
	relay.tickets[index] = nil
	authorized := index == 0 && matches(token, broker.state.AdminHash)
	if index == 1 {
		enrollment := broker.state.Nodes[selected.open.Target.Node]
		tokenHash := enrollment.TokenHash
		if administrator {
			tokenHash = broker.desktops.administratorKeys[selected.open.Target.Node].tokenHash
		}
		authorized = !enrollment.Revoked && matches(token, tokenHash)
	}
	if !authorized || relay.reserved[index] || !time.Now().Before(ticket.expires) || ticket.request.Reference != selected.open.Reference() ||
		ticket.request.Reference.Epoch != broker.desktops.epoch || !broker.desktopRelayCurrent(selected, relay) {
		broker.mutex.Unlock()
		http.Error(w, "desktop relay authority expired", http.StatusForbidden)
		return
	}
	relay.reserved[index] = true
	broker.mutex.Unlock()

	// Give this authenticated upgrade a bounded handshake deadline. After
	// upgrade, frame contexts/watchdog own only this connection's lifetime.
	controller := http.NewResponseController(w)
	if controller.SetReadDeadline(time.Now().Add(desktopRelayWriteTimeout)) != nil || controller.SetWriteDeadline(time.Now().Add(desktopRelayWriteTimeout)) != nil {
		broker.stopDesktopRelay(selected, relay)
		http.Error(w, "desktop relay deadline ownership is unavailable", http.StatusInternalServerError)
		return
	}
	// The WebSocket library owns RFC6455 framing/handshake validation. No
	// compression or browser-origin access is enabled on this native channel.
	socket, err := websocket.Accept(w, request, &websocket.AcceptOptions{Subprotocols: []string{contract.DesktopRelayProtocol}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		broker.stopDesktopRelay(selected, relay)
		return
	}
	if controller.SetReadDeadline(time.Time{}) != nil || controller.SetWriteDeadline(time.Time{}) != nil {
		_ = socket.CloseNow()
		broker.stopDesktopRelay(selected, relay)
		return
	}
	socket.SetReadLimit(contract.MaxDesktopRelayFrame)
	defer socket.CloseNow()
	if socket.Subprotocol() != contract.DesktopRelayProtocol {
		broker.stopDesktopRelay(selected, relay)
		return
	}
	broker.mutex.Lock()
	current := broker.desktopRelayCurrent(selected, relay)
	if current {
		relay.sockets[index] = socket
		if !relay.watching {
			relay.watching = true
			relay.started = time.Now()
			go broker.watchDesktopRelay(selected, relay)
		}
		if relay.sockets[0] != nil && relay.sockets[1] != nil && !relay.paired {
			relay.paired = true
			close(relay.ready)
		}
	}
	broker.mutex.Unlock()
	if !current {
		broker.stopDesktopRelay(selected, relay)
		return
	}
	defer broker.stopDesktopRelay(selected, relay)
	select {
	case <-relay.context.Done():
		return
	case <-relay.ready:
	}
	broker.mutex.Lock()
	peer := relay.sockets[1-index]
	broker.mutex.Unlock()
	var total, frames uint64
	for {
		readContext := relay.context
		var stop context.CancelFunc
		if !relay.administrator {
			readContext, stop = context.WithTimeout(relay.context, desktopRelayIdleTimeout)
		}
		kind, payload, err := socket.Read(readContext)
		if stop != nil {
			stop()
		}
		if err != nil || kind != websocket.MessageBinary || len(payload) == 0 || len(payload) > contract.MaxDesktopRelayFrame {
			return
		}
		total += uint64(len(payload))
		frames++
		if total > contract.MaxDesktopRelayBytes || frames > contract.MaxDesktopRelayFrames {
			return
		}
		writeContext, stop := context.WithTimeout(relay.context, desktopRelayWriteTimeout)
		err = peer.Write(writeContext, websocket.MessageBinary, payload)
		stop()
		if err != nil {
			return
		}
	}
}

func (broker *Broker) watchDesktopRelay(record *desktopRecord, relay *desktopRelay) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-relay.context.Done():
			broker.stopDesktopRelay(record, relay)
			return
		case <-ticker.C:
			broker.mutex.Lock()
			valid := broker.desktopRelayCurrent(record, relay) &&
				(relay.paired || time.Since(relay.started) < desktopRelayPairTimeout)
			broker.mutex.Unlock()
			if !valid {
				broker.stopDesktopRelay(record, relay)
				return
			}
		}
	}
}

func (broker *Broker) stopDesktopRelay(record *desktopRecord, relay *desktopRelay) {
	broker.mutex.Lock()
	if relay.retired {
		broker.mutex.Unlock()
		return
	}
	relay.retired = true
	relay.tickets = [2]*desktopRelayTicket{}
	relay.cancel()
	sockets := relay.sockets
	if !relay.administrator && record.state != "closed" {
		record.close, record.state, record.reason = true, "unknown", "transport-lost"
	}
	broker.mutex.Unlock()
	for _, socket := range sockets {
		if socket != nil {
			_ = socket.CloseNow()
		}
	}
}

// CloseDesktopRelays terminates hijacked sockets on broker shutdown; net/http
// cannot own their lifetime after a WebSocket upgrade.
func (broker *Broker) CloseDesktopRelays() {
	broker.mutex.Lock()
	type relayOwner struct {
		record *desktopRecord
		relay  *desktopRelay
	}
	var relays []relayOwner
	if broker.desktops != nil {
		for _, record := range broker.desktops.records {
			for _, relay := range []*desktopRelay{record.relay, record.administratorRelay} {
				if relay != nil {
					relays = append(relays, relayOwner{record, relay})
				}
			}
		}
	}
	broker.mutex.Unlock()
	for _, owner := range relays {
		broker.stopDesktopRelay(owner.record, owner.relay)
	}
}

// DesktopRelay is opaque authenticated transport. It does not provide endpoint
// confidentiality: callers must run pinned E2E encryption before sending content.
type DesktopRelay struct {
	socket *websocket.Conn
	once   sync.Once
}

func (relay *DesktopRelay) Read(ctx context.Context) ([]byte, error) {
	kind, payload, err := relay.socket.Read(ctx)
	if err != nil {
		return nil, err
	}
	if kind != websocket.MessageBinary || len(payload) == 0 || len(payload) > contract.MaxDesktopRelayFrame {
		return nil, errors.New("invalid desktop binary relay message")
	}
	return payload, nil
}

func (relay *DesktopRelay) Write(ctx context.Context, payload []byte) error {
	if len(payload) == 0 || len(payload) > contract.MaxDesktopRelayFrame {
		return errors.New("desktop relay message exceeds its frame contract")
	}
	return relay.socket.Write(ctx, websocket.MessageBinary, payload)
}

func (relay *DesktopRelay) Close() error {
	var err error
	relay.once.Do(func() { err = relay.socket.CloseNow() })
	return err
}

func (client *Client) DesktopRelayTicket(ctx context.Context, request contract.DesktopRelayRequest) (contract.DesktopRelayTicket, error) {
	var ticket contract.DesktopRelayTicket
	if err := request.Validate(); err != nil {
		return ticket, err
	}
	started := time.Now()
	if err := client.Call(ctx, contract.DesktopRelayTicketPath, request, &ticket); err != nil {
		return ticket, err
	}
	if ticket.Request != request || ticket.Provider.Validate() != nil || ticket.Mode != "view" && ticket.Mode != "control" || !validMachineNonce(ticket.Secret) || ticket.ExpiresAt.IsZero() || ticket.LeaseMillis <= 0 || ticket.LeaseMillis > contract.DesktopRelayTicketLifetime.Milliseconds() {
		return ticket, errors.New("desktop relay ticket changed its authenticated endpoint binding")
	}
	ticket.LeaseMillis -= time.Since(started).Milliseconds()
	if ticket.LeaseMillis <= 0 {
		return ticket, errors.New("desktop relay ticket expired in transit")
	}
	return ticket, nil
}

func (client *Client) AttachDesktopRelay(ctx context.Context, ticket contract.DesktopRelayTicket) (*DesktopRelay, error) {
	return client.attachDesktopRelay(ctx, contract.DesktopRelayPath, ticket)
}

func (client *Client) attachDesktopRelay(ctx context.Context, path string, ticket contract.DesktopRelayTicket) (*DesktopRelay, error) {
	if ticket.Request.Validate() != nil || !validMachineNonce(ticket.Secret) || ticket.LeaseMillis <= 0 || ticket.LeaseMillis > contract.DesktopRelayTicketLifetime.Milliseconds() {
		return nil, errors.New("invalid or expired desktop relay ticket")
	}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+client.Token)
	headers.Set(desktopRelayTicketHeader, ticket.Secret)
	// Only this dedicated handshake client has no whole-stream HTTP deadline.
	// Dial/frames and broker authority retain independent bounded contexts.
	httpClient := *client.HTTP
	httpClient.Timeout = 0
	handshake, stop := context.WithTimeout(ctx, time.Duration(ticket.LeaseMillis)*time.Millisecond)
	defer stop()
	socket, _, err := websocket.Dial(handshake, client.Endpoint+path, &websocket.DialOptions{
		HTTPClient: &httpClient, HTTPHeader: headers, Subprotocols: []string{contract.DesktopRelayProtocol}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return nil, errors.New("authenticated desktop relay connection failed")
	}
	if socket.Subprotocol() != contract.DesktopRelayProtocol {
		_ = socket.CloseNow()
		return nil, errors.New("desktop relay protocol negotiation changed")
	}
	socket.SetReadLimit(contract.MaxDesktopRelayFrame)
	return &DesktopRelay{socket: socket}, nil
}

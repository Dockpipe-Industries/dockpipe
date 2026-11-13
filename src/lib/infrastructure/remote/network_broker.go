package remote

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
	"github.com/coder/websocket"
)

const networkTicketHeader = "X-Dockpipe-Network-Ticket"

type networkBroker struct {
	catalogs   map[string]contract.NetworkCatalog
	streams    map[string]*networkStream
	trusts     map[string]networkTrustRecord
	trustBytes int
}

type networkStream struct {
	request    contract.NetworkConnect
	admission  operatorAdmission
	tickets    [2]string
	expires    time.Time
	context    context.Context
	cancel     context.CancelFunc
	ready      chan struct{}
	sockets    [2]*websocket.Conn
	reserved   [2]bool
	dispatched bool
}

func (broker *Broker) networkState() *networkBroker {
	if broker.networks == nil {
		broker.networks = &networkBroker{catalogs: map[string]contract.NetworkCatalog{}, streams: map[string]*networkStream{}, trusts: map[string]networkTrustRecord{}}
	}
	return broker.networks
}

func (broker *Broker) networkCurrent(catalog contract.NetworkCatalog) bool {
	enrollment, exists := broker.state.Nodes[catalog.Target.Node]
	return exists && !enrollment.Revoked && enrollment.MachineKey != "" && remoteidentity.Fingerprint(enrollment.MachineKey) == catalog.Target.Fingerprint && catalog.Enabled && time.Since(catalog.ReceivedAt) <= contract.NetworkFreshness
}

func (broker *Broker) networkValid(stream *networkStream) bool {
	catalog := broker.networkState().catalogs[stream.request.Target.Node]
	return !broker.poisoned && stream.context.Err() == nil && catalog.Target == stream.request.Target && broker.networkCurrent(catalog) && broker.networkTrustUsable(catalog) && broker.security != nil && broker.security.CheckAdmission(stream.admission) == nil
}

func (broker *Broker) networkOperator(path string, raw []byte, admission operatorAdmission) (any, error) {
	state := broker.networkState()
	if path == "/v1/network-trust" {
		return broker.readNetworkTrust(raw)
	}
	if path == "/v1/network-pulse" {
		var target contract.FeatureTarget
		if decodeFeature(raw, &target) != nil || target.Validate() != nil {
			return nil, errors.New("invalid network lease target")
		}
		catalog := state.catalogs[target.Node]
		if catalog.Target != target || !broker.networkCurrent(catalog) || !broker.networkTrustUsable(catalog) {
			return nil, errors.New("network target became unavailable")
		}
		for _, stream := range state.streams {
			if stream.request.Target == target && stream.admission.session == admission.session && stream.admission.generation == admission.generation && stream.context.Err() == nil && broker.security.CheckAdmission(stream.admission) == nil {
				stream.admission = admission
			}
		}
		return map[string]bool{"active": true}, nil
	}
	if path == "/v1/network-catalog" {
		var request struct {
			Schema string `json:"schema"`
		}
		if decodeFeature(raw, &request) != nil || request.Schema != contract.NetworkVersion {
			return nil, errors.New("invalid network discovery")
		}
		catalogs := []contract.NetworkCatalog{}
		for _, catalog := range state.catalogs {
			if broker.networkCurrent(catalog) {
				catalogs = append(catalogs, catalog)
			}
		}
		sort.Slice(catalogs, func(i, j int) bool { return catalogs[i].Target.Node < catalogs[j].Target.Node })
		return catalogs, nil
	}
	var request contract.NetworkConnect
	if path != "/v1/network-connect" || decodeFeature(raw, &request) != nil || request.Validate() != nil {
		return nil, errors.New("invalid network connection")
	}
	catalog := state.catalogs[request.Target.Node]
	if catalog.Target != request.Target || !broker.networkCurrent(catalog) || !broker.networkTrustUsable(catalog) {
		return nil, errors.New("exact network target is unavailable")
	}
	count := 0
	for id, stream := range state.streams {
		if stream.context.Err() != nil {
			delete(state.streams, id)
			continue
		}
		if stream.request.Target.Node == request.Target.Node {
			count++
		}
	}
	if len(state.streams) >= contract.MaxNetworkStreams || count >= contract.MaxNetworkNodeStreams {
		return nil, errors.New("network TCP stream capacity reached")
	}
	id, err := Secret()
	if err != nil {
		return nil, err
	}
	viewer, err := Secret()
	if err != nil {
		return nil, err
	}
	worker, err := Secret()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(contract.NetworkLifetime)

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	stream := &networkStream{request: request, admission: admission, tickets: [2]string{Hash(viewer), worker}, expires: time.Now().Add(15 * time.Second), context: ctx, cancel: cancel, ready: make(chan struct{})}
	state.streams[id] = stream
	go broker.watchNetwork(stream)
	return contract.NetworkTicket{ID: id, Secret: viewer, Request: request}, nil
}

// Worker polling requires the existing persistent machine signature on every
// exchange. Tickets are issued once and bind both the exact destination and node.
func (broker *Broker) networkWorker(node string, raw []byte) (any, error) {
	var catalog contract.NetworkCatalog
	if decodeFeature(raw, &catalog) != nil || catalog.Validate() != nil || catalog.Target.Node != node {
		return nil, errors.New("invalid network worker catalog")
	}
	enrollment := broker.state.Nodes[node]
	if enrollment.MachineKey == "" || remoteidentity.Fingerprint(enrollment.MachineKey) != catalog.Target.Fingerprint {
		return nil, errors.New("network requires paired machine identity")
	}
	catalog.ReceivedAt = time.Now()
	state := broker.networkState()
	previousCatalog, published := state.catalogs[node]
	if published && previousCatalog.Target == catalog.Target && networkCatalogTrustSummary(previousCatalog) != networkCatalogTrustSummary(catalog) {
		return nil, errors.New("network trust sharing cannot change within a worker generation")
	}
	if catalog.TLSTrust != nil && catalog.TLSTrust.State == "ready" && !broker.networkTrustUsable(catalog) {
		return nil, errors.New("network catalog has no exact authenticated TLS trust publication")
	}
	if catalog.TLSTrust == nil || catalog.TLSTrust.State != "ready" {
		if previous, exists := state.trusts[node]; exists {
			state.trustBytes -= previous.summary.Bytes
			delete(state.trusts, node)
		}
	}
	state.catalogs[node] = catalog
	broker.pruneNetworkTrust()
	tickets := []contract.NetworkTicket{}
	for id, stream := range state.streams {
		if stream.context.Err() != nil {
			delete(state.streams, id)
			continue
		}
		if stream.request.Target == catalog.Target && !stream.dispatched && broker.networkValid(stream) {
			tickets = append(tickets, contract.NetworkTicket{ID: id, Secret: stream.tickets[1], Request: stream.request})
			stream.tickets[1] = Hash(stream.tickets[1])
			stream.dispatched = true
		}
	}
	return tickets, nil
}

func (broker *Broker) watchNetwork(stream *networkStream) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	defer broker.stopNetwork(stream)
	for {
		select {
		case <-stream.context.Done():
			return
		case <-ticker.C:
			broker.mutex.Lock()
			valid := broker.networkValid(stream) && (stream.sockets[0] != nil && stream.sockets[1] != nil || time.Now().Before(stream.expires))
			broker.mutex.Unlock()
			if !valid {
				return
			}
		}
	}
}

func (broker *Broker) stopNetwork(stream *networkStream) {
	broker.mutex.Lock()
	stream.cancel()
	sockets := stream.sockets
	broker.mutex.Unlock()
	for _, socket := range sockets {
		if socket != nil {
			_ = socket.CloseNow()
		}
	}
}

func (broker *Broker) CloseNetworkRelays() {
	broker.mutex.Lock()
	streams := []*networkStream{}
	if broker.networks != nil {
		for _, stream := range broker.networks.streams {
			streams = append(streams, stream)
		}
	}
	broker.mutex.Unlock()
	for _, stream := range streams {
		broker.stopNetwork(stream)
	}
}

func (broker *Broker) serveNetworkRelay(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet || request.URL.RawQuery != "" || request.ContentLength > 0 || request.Header.Get("Origin") != "" || len(request.Header.Values("Authorization")) != 1 || len(request.Header.Values(networkTicketHeader)) != 1 {
		http.Error(w, "invalid network relay attachment", 400)
		return
	}
	secret := request.Header.Get(networkTicketHeader)
	if !validMachineNonce(secret) {
		http.Error(w, "network authorization required", 401)
		return
	}
	token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	broker.mutex.Lock()
	var selected *networkStream
	index := 0
	for _, stream := range broker.networkState().streams {
		for role, ticket := range stream.tickets {
			if (role == 0 || stream.dispatched) && matches(secret, ticket) {
				selected, index = stream, role
			}
		}
	}
	if selected == nil {
		broker.mutex.Unlock()
		http.Error(w, "network ticket unavailable", 401)
		return
	}
	selected.tickets[index] = ""
	enrollment := broker.state.Nodes[selected.request.Target.Node]
	authorized := index == 0 && matches(token, broker.state.AdminHash) || index == 1 && !enrollment.Revoked && matches(token, enrollment.TokenHash)
	if !authorized || selected.reserved[index] || !time.Now().Before(selected.expires) || !broker.networkValid(selected) {
		broker.mutex.Unlock()
		http.Error(w, "network authority expired", 403)
		return
	}
	selected.reserved[index] = true
	broker.mutex.Unlock()
	defer broker.stopNetwork(selected)
	controller := http.NewResponseController(w)
	if controller.SetReadDeadline(time.Now().Add(5*time.Second)) != nil || controller.SetWriteDeadline(time.Now().Add(5*time.Second)) != nil {
		return
	}
	socket, err := websocket.Accept(w, request, &websocket.AcceptOptions{Subprotocols: []string{contract.NetworkProtocol}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	defer socket.CloseNow()
	if controller.SetReadDeadline(time.Time{}) != nil || controller.SetWriteDeadline(time.Time{}) != nil || socket.Subprotocol() != contract.NetworkProtocol {
		return
	}
	socket.SetReadLimit(contract.MaxNetworkFrame)
	broker.mutex.Lock()
	if !broker.networkValid(selected) {
		broker.mutex.Unlock()
		return
	}
	selected.sockets[index] = socket
	if selected.sockets[0] != nil && selected.sockets[1] != nil {
		close(selected.ready)
	}
	broker.mutex.Unlock()
	select {
	case <-selected.context.Done():
		return
	case <-selected.ready:
	}
	broker.mutex.Lock()
	peer := selected.sockets[1-index]
	broker.mutex.Unlock()
	total := uint64(0)
	frames := uint64(0)
	for {
		ctx, cancel := context.WithTimeout(selected.context, contract.NetworkIdle)
		kind, payload, readErr := socket.Read(ctx)
		cancel()
		if readErr != nil || kind != websocket.MessageBinary || len(payload) == 0 {
			return
		}
		total += uint64(len(payload))
		frames++
		if total > contract.MaxNetworkBytes || frames > contract.MaxNetworkFrames {
			return
		}
		ctx, cancel = context.WithTimeout(selected.context, 5*time.Second)
		err = peer.Write(ctx, websocket.MessageBinary, payload)
		cancel()
		if err != nil {
			return
		}
	}
}

package nativehost

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"sync"
	"time"
)

type Broker struct {
	options   Options
	mu        sync.Mutex
	expected  *workerExpectation
	worker    *wire
	status    Status
	grant     Grant
	clients   map[*net.UnixConn]struct{}
	pending   map[uint64]chan frame
	nextID    uint64
	terminals int
	listeners []*net.UnixListener
	closed    bool
	slots     chan struct{}
}

func NewBroker(options Options) (*Broker, error) {
	if !validSocketPath(options.WorkerSocket) || !validSocketPath(options.ClientSocket) || options.WorkerSocket == options.ClientSocket || !validToken(options.ClientKey) || options.Authorize == nil {
		return nil, errors.New("invalid native host broker options")
	}
	return &Broker{options: options, status: unavailableStatus(), clients: make(map[*net.UnixConn]struct{}), pending: make(map[uint64]chan frame), slots: make(chan struct{}, 32)}, nil
}

func (b *Broker) ExpectWorker(uid, gid uint32, nonce string, expires time.Time) error {
	grant, err := b.options.Authorize()
	if err != nil || uid == 0 || grant.UID != uid || grant.Generation == "" || len(grant.Generation) > 256 || !validToken(nonce) || !expires.After(time.Now()) || expires.After(time.Now().Add(5*time.Minute)) {
		return ErrUnavailable
	}
	b.Revoke()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrUnavailable
	}
	b.expected = &workerExpectation{uid: uid, gid: gid, nonce: nonce, expires: expires, grant: grant}
	return nil
}

func (b *Broker) Status() Status {
	grant, err := b.options.Authorize()
	b.mu.Lock()
	defer b.mu.Unlock()
	if err != nil || b.worker == nil || grant != b.grant {
		return unavailableStatus()
	}
	status := b.status
	status.Groups = append([]uint32(nil), status.Groups...)
	return status
}

func (b *Broker) Revoke() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.revokeLocked()
	return nil
}

func (b *Broker) revokeLocked() {
	b.expected = nil
	if b.worker != nil {
		b.worker.Close()
		b.worker = nil
	}
	for client := range b.clients {
		client.Close()
	}
	for id, responses := range b.pending {
		close(responses)
		delete(b.pending, id)
	}
	b.status = unavailableStatus()
}

func (b *Broker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.revokeLocked()
	for _, listener := range b.listeners {
		listener.Close()
	}
	return nil
}

func (b *Broker) Serve(ctx context.Context) error {
	worker, err := listenSocket(b.options.WorkerSocket)
	if err != nil {
		return err
	}
	client, err := listenSocket(b.options.ClientSocket)
	if err != nil {
		worker.Close()
		return err
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		worker.Close()
		client.Close()
		return ErrUnavailable
	}
	b.listeners = []*net.UnixListener{worker, client}
	b.mu.Unlock()
	defer b.Close()
	errors := make(chan error, 2)
	go func() { errors <- b.accept(ctx, worker, true) }()
	go func() { errors <- b.accept(ctx, client, false) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errors:
			return err
		case <-tick.C:
			grant, err := b.options.Authorize()
			b.mu.Lock()
			if b.worker != nil && (err != nil || grant != b.grant) {
				b.revokeLocked()
			}
			if b.expected != nil && (!b.expected.expires.After(time.Now()) || err != nil || grant != b.expected.grant) {
				b.expected = nil
			}
			b.mu.Unlock()
		}
	}
}

func (b *Broker) accept(ctx context.Context, listener *net.UnixListener, worker bool) error {
	for {
		connection, err := listener.AcceptUnix()
		if err != nil {
			return err
		}
		select {
		case b.slots <- struct{}{}:
			go func() {
				defer func() { <-b.slots }()
				defer connection.Close()
				if worker {
					b.registerWorker(connection)
				} else {
					b.serveClient(ctx, connection)
				}
			}()
		default:
			connection.Close()
		}
	}
}

func (b *Broker) registerWorker(connection *net.UnixConn) {
	credential, err := peer(connection)
	if err != nil || credential.Uid == 0 {
		return
	}
	connection.SetDeadline(time.Now().Add(3 * time.Second))
	transport := &wire{ReadWriteCloser: connection}
	hello, err := transport.receive()
	if err != nil || hello.Kind != "worker" || hello.Status == nil {
		return
	}
	grant, err := b.options.Authorize()
	b.mu.Lock()
	expected := b.expected
	if err != nil || expected == nil || b.closed || !expected.expires.After(time.Now()) || expected.grant != grant || credential.Uid != expected.uid || credential.Gid != expected.gid || subtle.ConstantTimeCompare([]byte(hello.Nonce), []byte(expected.nonce)) != 1 || !validWorkerStatus(*hello.Status, credential.Uid, credential.Gid) {
		b.mu.Unlock()
		return
	}
	b.expected = nil
	b.worker = transport
	b.grant = grant
	b.status = *hello.Status
	b.status.Schema = StatusSchema
	b.status.Available = true
	b.status.Connected = true
	b.status.Generation = grant.Generation
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		if b.worker == transport {
			b.revokeLocked()
		}
		b.mu.Unlock()
	}()
	if transport.send(frame{Kind: "accepted"}) != nil {
		return
	}
	connection.SetDeadline(time.Time{})
	for {
		value, err := transport.receive()
		if err != nil {
			return
		}
		b.mu.Lock()
		responses := b.pending[value.ID]
		if responses != nil {
			select {
			case responses <- value:
			default:
				b.mu.Unlock()
				return
			}
		}
		b.mu.Unlock()
	}
}

func validWorkerStatus(status Status, uid, gid uint32) bool {
	return status.UID == uid && status.GID == gid && len(status.Node) > 0 && len(status.Node) <= 255 && len(status.Username) > 0 && len(status.Username) <= 256 && len(status.Home) <= 4096 && len(status.Groups) <= 1024
}

func (b *Broker) serveClient(ctx context.Context, connection *net.UnixConn) {
	credential, err := peer(connection)
	if err != nil || credential.Uid != b.options.ClientUID {
		return
	}
	connection.SetDeadline(time.Now().Add(3 * time.Second))
	transport := &wire{ReadWriteCloser: connection}
	request, err := transport.receive()
	if err != nil || len(request.Snapshot) != 0 || request.Status != nil || subtle.ConstantTimeCompare([]byte(request.Key), []byte(b.options.ClientKey)) != 1 {
		return
	}
	request.Key = ""
	status := b.Status()
	if !status.Available {
		transport.send(frame{Kind: "error", Error: "native_host_unavailable", Status: &status})
		return
	}
	b.mu.Lock()
	if b.worker == nil || b.grant.Generation != status.Generation {
		b.mu.Unlock()
		return
	}
	b.clients[connection] = struct{}{}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.clients, connection)
		b.mu.Unlock()
	}()
	if request.Kind == "status" {
		transport.send(frame{Kind: "status", Status: &status})
		return
	}
	if request.Kind != "snapshot" && request.Kind != "terminal" && request.Kind != "docker" && request.Kind != "docker-query" {
		return
	}
	if request.Kind == "docker" && !validDockerAction(request.Action, request.Container) {
		return
	}
	if request.Kind == "docker-query" && ValidateDockerQuery(request.ResourceKind, request.ResourceID) != nil {
		return
	}
	b.mu.Lock()
	if len(b.pending) >= 24 || (request.Kind == "terminal" && b.terminals >= 8) || b.worker == nil {
		b.mu.Unlock()
		return
	}
	b.nextID++
	id := b.nextID
	queueSize := 1
	if request.Kind == "terminal" {
		queueSize = 16
	}
	responses := make(chan frame, queueSize)
	b.pending[id] = responses
	worker := b.worker
	if request.Kind == "terminal" {
		b.terminals++
	}
	b.mu.Unlock()
	defer func() {
		worker.send(frame{Kind: "close", ID: id})
		b.mu.Lock()
		delete(b.pending, id)
		if request.Kind == "terminal" {
			b.terminals--
		}
		b.mu.Unlock()
	}()
	request.ID = id
	if worker.send(request) != nil {
		return
	}
	if request.Kind == "snapshot" || request.Kind == "docker" || request.Kind == "docker-query" {
		connection.SetDeadline(time.Now().Add(8 * time.Second))
		timer := time.NewTimer(8 * time.Second)
		defer timer.Stop()
		disconnected := make(chan struct{})
		go func() {
			// A snapshot connection carries exactly one request. EOF or an
			// extra request cancels that request's worker operation.
			transport.receive()
			close(disconnected)
		}()
		select {
		case response, ok := <-responses:
			if ok {
				transport.send(response)
			}
		case <-timer.C:
		case <-disconnected:
		case <-ctx.Done():
		}
		return
	}
	connection.SetDeadline(time.Time{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer connection.Close()
		for {
			value, err := transport.receive()
			if err != nil || (value.Kind != "input" && value.Kind != "resize" && value.Kind != "close") {
				return
			}
			grant, err := b.options.Authorize()
			if err != nil || grant.UID != status.UID || grant.Generation != status.Generation {
				return
			}
			value.ID = id
			if worker.send(value) != nil || value.Kind == "close" {
				return
			}
		}
	}()
	for {
		select {
		case response, ok := <-responses:
			if !ok || transport.send(response) != nil || response.Kind == "exit" || response.Kind == "error" {
				return
			}
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

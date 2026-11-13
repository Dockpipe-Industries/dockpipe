package remote

import (
	"bytes"
	"context"
	"crypto/rand"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"github.com/coder/websocket"
)

type desktopRelayFixture struct {
	broker       *Broker
	viewer       *Client
	worker       *Client
	catalog      contract.DesktopCatalog
	open         contract.DesktopOpen
	viewerTicket contract.DesktopRelayTicket
	workerTicket contract.DesktopRelayTicket
}

func newDesktopRelayFixture(t *testing.T) desktopRelayFixture {
	t.Helper()
	broker, viewer, worker, _, _, _, catalog := desktopFixture(t)
	request := desktopIntent(t, catalog)
	ctx := context.Background()
	if _, err := viewer.OpenDesktop(ctx, request); err != nil {
		t.Fatal(err)
	}
	viewerPin, workerPin := Hash("fixture-viewer-ephemeral"), Hash("fixture-worker-ephemeral")
	if _, err := viewer.SignalDesktop(ctx, contract.DesktopSignal{Reference: request.Reference(), Sequence: 1, Type: "binding", Fingerprint: viewerPin}); err != nil {
		t.Fatal(err)
	}
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: catalog.Epoch, Catalog: catalog,
		Reports: []contract.DesktopReport{{Reference: request.Reference(), State: "opening", Ack: 1, Signals: []contract.DesktopSignal{{Reference: request.Reference(), Sequence: 1, Type: "binding", Fingerprint: workerPin}}}}}
	if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
		t.Fatal(err)
	}
	viewerTicket, err := viewer.DesktopRelayTicket(ctx, contract.DesktopRelayRequest{Reference: request.Reference(), Role: "viewer", LocalFingerprint: viewerPin, PeerFingerprint: workerPin})
	if err != nil {
		t.Fatal(err)
	}
	workerTicket, err := worker.DesktopRelayTicket(ctx, contract.DesktopRelayRequest{Reference: request.Reference(), Role: "worker", LocalFingerprint: workerPin, PeerFingerprint: viewerPin})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(broker.CloseDesktopRelays)
	return desktopRelayFixture{broker: broker, viewer: viewer, worker: worker, catalog: catalog, open: request, viewerTicket: viewerTicket, workerTicket: workerTicket}
}

func (fixture desktopRelayFixture) attach(t *testing.T) (*DesktopRelay, *DesktopRelay) {
	t.Helper()
	viewer, err := fixture.viewer.AttachDesktopRelay(context.Background(), fixture.viewerTicket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = viewer.Close() })
	worker, err := fixture.worker.AttachDesktopRelay(context.Background(), fixture.workerTicket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Close() })
	return viewer, worker
}

func TestDesktopRelayOpaqueDuplexAndOneUseBinding(t *testing.T) {
	fixture := newDesktopRelayFixture(t)
	viewer, worker := fixture.attach(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	payload := make([]byte, contract.MaxDesktopRelayFrame)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	if err := viewer.Write(ctx, payload); err != nil {
		t.Fatal(err)
	}
	received, err := worker.Read(ctx)
	if err != nil || !bytes.Equal(received, payload) {
		t.Fatal("opaque viewer bytes changed", err)
	}
	if err := worker.Write(ctx, payload[:37]); err != nil {
		t.Fatal(err)
	}
	received, err = viewer.Read(ctx)
	if err != nil || !bytes.Equal(received, payload[:37]) {
		t.Fatal("opaque worker bytes changed", err)
	}
	if _, err := fixture.viewer.AttachDesktopRelay(ctx, fixture.viewerTicket); err == nil {
		t.Fatal("one-use viewer ticket replayed")
	}
	if _, err := fixture.viewer.DesktopRelayTicket(ctx, fixture.viewerTicket.Request); err == nil {
		t.Fatal("second simultaneous viewer channel admitted")
	}
	if err := viewer.Write(ctx, make([]byte, contract.MaxDesktopRelayFrame+1)); err == nil {
		t.Fatal("client accepted unbounded frame")
	}
}

func TestDesktopRelayRejectsUnboundForeignAndConsumedAuthority(t *testing.T) {
	for _, scenario := range []string{"peer", "epoch", "role", "bearer", "lock", "revoked", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := newDesktopRelayFixture(t)
			request := fixture.viewerTicket.Request
			switch scenario {
			case "peer":
				request.PeerFingerprint = Hash("foreign-peer")
			case "epoch":
				request.Reference.Epoch = Hash("foreign-epoch")
			case "role":
				request.Role = "worker"
			}
			if scenario == "peer" || scenario == "epoch" || scenario == "role" {
				if _, err := fixture.viewer.DesktopRelayTicket(context.Background(), request); err == nil {
					t.Fatal("foreign endpoint/session gained an attach ticket")
				}
				return
			}
			client := *fixture.viewer
			if scenario == "bearer" {
				client.Token = Hash("wrong-bearer")
			}
			if scenario == "lock" {
				fixture.broker.security.Lock()
			}
			fixture.broker.mutex.Lock()
			if scenario == "revoked" {
				enrollment := fixture.broker.state.Nodes[request.Reference.Target.Node]
				enrollment.Revoked = true
				fixture.broker.state.Nodes[request.Reference.Target.Node] = enrollment
			}
			if scenario == "expired" {
				fixture.broker.desktops.records[fixture.open.ID].relay.tickets[0].expires = time.Now().Add(-time.Second)
			}
			fixture.broker.mutex.Unlock()
			if _, err := client.AttachDesktopRelay(context.Background(), fixture.viewerTicket); err == nil {
				t.Fatal("stale or unauthenticated relay attach succeeded")
			}
			if _, err := fixture.viewer.AttachDesktopRelay(context.Background(), fixture.viewerTicket); err == nil {
				t.Fatal("failed attach reused a consumed ticket")
			}
		})
	}
}

func TestDesktopRelayDisconnectLockAndFrameFailureRetainUnknown(t *testing.T) {
	for _, scenario := range []string{"disconnect", "lock", "revoked", "lease", "policy", "oversize", "text"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := newDesktopRelayFixture(t)
			viewer, worker := fixture.attach(t)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			switch scenario {
			case "disconnect":
				_ = viewer.Close()
			case "lock":
				fixture.broker.security.Lock()
			case "revoked":
				fixture.broker.mutex.Lock()
				enrollment := fixture.broker.state.Nodes[fixture.open.Target.Node]
				enrollment.Revoked = true
				fixture.broker.state.Nodes[fixture.open.Target.Node] = enrollment
				fixture.broker.mutex.Unlock()
			case "lease":
				fixture.broker.mutex.Lock()
				fixture.broker.desktops.records[fixture.open.ID].admission.deadline = time.Now().Add(-time.Second)
				fixture.broker.mutex.Unlock()
			case "policy":
				fixture.broker.mutex.Lock()
				catalog := fixture.broker.desktops.catalogs[fixture.open.Target.Node]
				catalog.Target.Generation = Hash("changed-owner-policy")
				fixture.broker.desktops.catalogs[fixture.open.Target.Node] = catalog
				fixture.broker.mutex.Unlock()
			case "oversize":
				_ = viewer.socket.Write(ctx, websocket.MessageBinary, make([]byte, contract.MaxDesktopRelayFrame+1))
			case "text":
				_ = viewer.socket.Write(ctx, websocket.MessageText, []byte("not-binary"))
			}
			if _, err := worker.Read(ctx); err == nil {
				t.Fatal("failed relay left opposite endpoint live")
			}
			fixture.broker.mutex.Lock()
			record := fixture.broker.desktops.records[fixture.open.ID]
			unknown := record.close && record.state == "unknown" && record.relay.retired
			fixture.broker.mutex.Unlock()
			if !unknown {
				t.Fatal("relay transport failure claimed native cleanup or released quota")
			}
		})
	}
}

func TestDesktopRelayBackpressureLockCancelsBlockedTransfer(t *testing.T) {
	fixture := newDesktopRelayFixture(t)
	viewer, _ := fixture.attach(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		payload := make([]byte, contract.MaxDesktopRelayFrame)
		for index := 0; index < 256; index++ {
			if err := viewer.Write(ctx, payload); err != nil {
				finished <- err
				return
			}
		}
		finished <- nil
	}()
	time.Sleep(100 * time.Millisecond)
	fixture.broker.security.Lock()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("unread peer accepted the entire bounded burst without backpressure")
		}
	case <-time.After(time.Second):
		t.Fatal("authority lock did not cancel backpressured relay")
	}
}

func TestDesktopRelaySurvivesOrdinaryHTTPTimeout(t *testing.T) {
	if os.Getenv("DOCKPIPE_TEST_DESKTOP_RELAY_LONG") != "1" {
		t.Skip("explicit 35-second authenticated connection qualification")
	}
	fixture := newDesktopRelayFixture(t)
	server := httptest.NewUnstartedServer(fixture.broker)
	server.Config.ReadTimeout, server.Config.WriteTimeout = 30*time.Second, 30*time.Second
	server.Start()
	defer server.Close()
	fixture.viewer.HTTP.CloseIdleConnections()
	fixture.worker.HTTP.CloseIdleConnections()
	fixture.viewer.Endpoint, fixture.worker.Endpoint = server.URL, server.URL
	viewer, worker := fixture.attach(t)
	defer viewer.Close()
	defer worker.Close()
	defer fixture.broker.CloseDesktopRelays()
	ctx, cancel := context.WithTimeout(context.Background(), 42*time.Second)
	defer cancel()
	pulses := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				pulses <- nil
				return
			case <-ticker.C:
				_, err := fixture.viewer.PulseDesktop(ctx, contract.DesktopRead{Reference: fixture.open.Reference(), After: 1})
				if err == nil {
					_, err = fixture.worker.ExchangeDesktop(ctx, contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: fixture.catalog.Epoch, Catalog: fixture.catalog})
				}
				if err != nil {
					if ctx.Err() != nil {
						pulses <- nil
					} else {
						pulses <- err
					}
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		if err := <-pulses; err != nil {
			t.Error(err)
		}
	}()
	started := time.Now()
	for sample := 0; sample < 8; sample++ {
		if sample != 0 && !pause(ctx, 5*time.Second) {
			t.Fatal(ctx.Err())
		}
		payload := []byte{byte(sample), 0x15, 0xa9, 0x01}
		if err := viewer.Write(ctx, payload); err != nil {
			t.Fatal(err)
		}
		received, err := worker.Read(ctx)
		if err != nil || !bytes.Equal(received, payload) {
			t.Fatal("relay failed across HTTP30s boundary", err)
		}
		if err := worker.Write(ctx, payload); err != nil {
			t.Fatal(err)
		}
		if _, err := viewer.Read(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if time.Since(started) < 35*time.Second {
		t.Fatal("long connection fixture did not cross HTTP deadline")
	}
}

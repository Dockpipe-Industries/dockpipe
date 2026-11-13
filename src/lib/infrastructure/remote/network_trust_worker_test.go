package remote

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
)

type networkTrustPlatformFixture struct {
	*securityFixturePlatform
	trustMutex sync.Mutex
	anchors    [][]byte
	trustError error
	calls      int
}

func (platform *networkTrustPlatformFixture) NetworkTLSAnchors(ctx context.Context) ([][]byte, error) {
	platform.trustMutex.Lock()
	defer platform.trustMutex.Unlock()
	platform.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make([][]byte, 0, len(platform.anchors))
	for _, anchor := range platform.anchors {
		result = append(result, append([]byte(nil), anchor...))
	}
	return result, platform.trustError
}

func TestNetworkTrustWorkerExplicitSharingAndUnavailableMaterial(t *testing.T) {
	_, _, _, catalog := networkFixture(t)
	platform := &networkTrustPlatformFixture{securityFixturePlatform: &securityFixturePlatform{}, anchors: networkTrustCertificates(t, 1, 0)}
	disabled := collectNetworkTrust(context.Background(), platform, catalog.Target, false)
	if disabled.summary.State != "disabled" || disabled.snapshot != nil || platform.calls != 0 {
		t.Fatal("public TLS material collected without explicit owner opt-in")
	}
	ready := collectNetworkTrust(context.Background(), platform, catalog.Target, true)
	if ready.summary.State != "ready" || ready.snapshot == nil {
		t.Fatal("explicit public trust unavailable")
	}
	platform.trustError = errors.New("native policy is not representable")
	unavailable := collectNetworkTrust(context.Background(), platform, catalog.Target, true)
	if unavailable.summary.State != "unavailable" || unavailable.snapshot != nil {
		t.Fatal("partial anchors accepted after native trust export failure")
	}
	unsupported := collectNetworkTrust(context.Background(), platform.securityFixturePlatform, catalog.Target, true)
	if unsupported.summary.State != "unavailable" || unsupported.snapshot != nil {
		t.Fatal("unsupported platform silently flattened local trust")
	}
}

func TestNetworkTrustWorkerRemovalRetiresSocketsAndRefreshesGeneration(t *testing.T) {
	broker, operator, worker, config, root, _, _ := featureFixture(t)
	t.Cleanup(broker.CloseNetworkRelays)
	platform := &networkTrustPlatformFixture{securityFixturePlatform: &securityFixturePlatform{snapshot: osauth.Snapshot{LockKnown: true, SleepKnown: true}}, anchors: networkTrustCertificates(t, 1, 0)}
	permission := contract.NetworkPermissions{Schema: contract.NetworkVersion, Enabled: true, ShareTLSTrust: true}
	if err := WritePrivate(filepath.Join(root, "network.json"), permission); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	samplerDone := make(chan struct{})
	go func() {
		defer close(samplerDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				broker.security.Sample(ctx)
			}
		}
	}()
	ready := make(chan struct{}, 2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runNetworkService(ctx, root, func(ctx context.Context) error {
			return runNetworkWorkerWithPlatform(ctx, root, config, worker, func() error { ready <- struct{}{}; return nil }, platform)
		})
	}()
	defer func() {
		cancel()
		<-samplerDone
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("trust-aware network service did not join")
		}
	}()
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("trust-aware service did not become ready")
	}
	catalogs, err := operator.NetworkCatalog(ctx)
	if err != nil || len(catalogs) != 1 || catalogs[0].TLSTrust == nil || catalogs[0].TLSTrust.Count != 1 {
		t.Fatal(catalogs, err)
	}
	original := catalogs[0]
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	echoDone := make(chan struct{})
	go func() {
		defer close(echoDone)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = io.Copy(connection, connection)
	}()
	connection, err := operator.networkDial(ctx, original.Target, listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err = connection.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	if _, err = io.ReadFull(connection, make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	platform.trustMutex.Lock()
	platform.anchors = nil
	platform.trustMutex.Unlock()
	select {
	case <-ready:
	case <-time.After(8 * time.Second):
		t.Fatal("removed public anchor did not create a fresh network generation")
	}
	refreshed, err := operator.NetworkCatalog(ctx)
	if err != nil || len(refreshed) != 1 {
		t.Fatal(refreshed, err)
	}
	current := refreshed[0]
	if current.Target == original.Target || current.TLSTrust == nil || current.TLSTrust.State != "ready" || current.TLSTrust.Count != 0 || current.TLSTrust.Digest == original.TLSTrust.Digest {
		t.Fatal("removed trust retained its original profile generation")
	}
	_, err = operator.NetworkTrust(ctx, contract.NetworkTrustRequest{Schema: contract.NetworkTrustVersion, Target: original.Target, Digest: original.TLSTrust.Digest})
	if err == nil {
		t.Fatal("old trust generation remained retrievable after removal")
	}
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, err = connection.Read(make([]byte, 1)); err == nil {
		t.Fatal("removed public trust retained an active original TCP stream")
	}
	select {
	case <-echoDone:
	case <-time.After(time.Second):
		t.Fatal("removed trust retained the worker destination socket")
	}
}

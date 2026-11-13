package remote

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
)

func TestNetworkServiceExplicitEnableAndNoUncertainRestart(t *testing.T) {
	root := privateTestDirectory(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	starts := make(chan struct{}, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runNetworkService(ctx, root, func(context.Context) error {
			starts <- struct{}{}
			return errors.New("uncertain fixture transport failure")
		})
	}()
	assertQuiet := func() {
		t.Helper()
		select {
		case <-starts:
			t.Fatal("network started without a new explicit enabled grant")
		case <-time.After(600 * time.Millisecond):
		}
	}
	assertQuiet()
	permission := contract.NetworkPermissions{Schema: contract.NetworkVersion, Enabled: false}
	path := filepath.Join(root, "network.json")
	if err := WritePrivate(path, permission); err != nil {
		t.Fatal(err)
	}
	assertQuiet()
	permission.Enabled = true
	if err := WritePrivate(path, permission); err != nil {
		t.Fatal(err)
	}
	select {
	case <-starts:
	case <-time.After(time.Second):
		t.Fatal("running normal worker did not notice explicit network enable")
	}
	assertQuiet()
	permission.Enabled = false
	if err := WritePrivate(path, permission); err != nil {
		t.Fatal(err)
	}
	assertQuiet()
	permission.Enabled = true
	if err := WritePrivate(path, permission); err != nil {
		t.Fatal(err)
	}
	select {
	case <-starts:
	case <-time.After(time.Second):
		t.Fatal("new enable transition failed to start a fresh network lifetime")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("network supervisor did not join on shutdown")
	}
}

func TestNetworkWorkerDisabledGrantSingletonAndCancellation(t *testing.T) {
	for _, reason := range []string{"cancel", "grant", "lock"} {
		t.Run(reason, func(t *testing.T) {
			broker, operator, worker, config, root, _, _ := featureFixture(t)
			t.Cleanup(broker.CloseNetworkRelays)
			permission := contract.NetworkPermissions{Schema: contract.NetworkVersion, Enabled: false}
			path := filepath.Join(root, "network.json")
			if err := WritePrivate(path, permission); err != nil {
				t.Fatal(err)
			}
			platform := &securityFixturePlatform{snapshot: osauth.Snapshot{LockKnown: true, SleepKnown: true}}
			if runNetworkWorkerWithPlatform(context.Background(), root, config, worker, nil, platform) == nil {
				t.Fatal("disabled network owner grant started a worker")
			}
			permission.Enabled = true
			if err := WritePrivate(path, permission); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- runNetworkWorkerWithPlatform(ctx, root, config, worker, func() error { close(ready); return nil }, platform)
			}()
			select {
			case <-ready:
			case err := <-done:
				t.Fatal("worker stopped before readiness", err)
			case <-time.After(time.Second):
				t.Fatal("worker readiness timed out")
			}
			if runNetworkWorkerWithPlatform(ctx, root, config, worker, nil, platform) == nil {
				t.Fatal("second network worker lifetime bypassed singleton ownership")
			}
			catalogs, err := operator.NetworkCatalog(ctx)
			if err != nil || len(catalogs) != 1 {
				t.Fatal(catalogs, err)
			}
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
			connection, err := operator.networkDial(ctx, catalogs[0].Target, listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			if _, err = connection.Write([]byte("worker echo")); err != nil {
				t.Fatal(err)
			}
			reply := make([]byte, 11)
			if _, err = io.ReadFull(connection, reply); err != nil || string(reply) != "worker echo" {
				t.Fatal(string(reply), err)
			}
			switch reason {
			case "cancel":
				cancel()
			case "grant":
				permission.Enabled = false
				if err = WritePrivate(path, permission); err != nil {
					t.Fatal(err)
				}
			case "lock":
				platform.mutex.Lock()
				platform.snapshot.Locked = true
				platform.mutex.Unlock()
			}
			select {
			case err = <-done:
				if err == nil {
					t.Fatal("cancelled network worker reported success")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("worker did not cancel and join its TCP streams")
			}
			_ = connection.SetReadDeadline(time.Now().Add(time.Second))
			if _, err = connection.Read(make([]byte, 1)); err == nil {
				t.Fatal("ended worker retained its TCP stream")
			}
			select {
			case <-echoDone:
			case <-time.After(time.Second):
				t.Fatal("destination TCP socket did not close")
			}
		})
	}
}

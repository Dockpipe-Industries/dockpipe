package remotecmd

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func serve(ctx context.Context, root string) error {
	if err := requireLocalBroker(root); err != nil {
		return err
	}
	if err := remoteio.PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "broker.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	var config OperatorConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &config); err != nil {
		return err
	}
	if err := validateListen(config.Listen); err != nil {
		return err
	}
	broker, err := remoteio.NewBroker(root)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", config.Listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	platform, err := osauth.New()
	if err != nil {
		return err
	}
	closeSecurity, err := broker.StartOperatorSecurity(ctx, platform)
	if err != nil {
		return err
	}
	defer closeSecurity()
	server := &http.Server{Handler: broker, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	defer server.Close()
	failures := make(chan error, 2)
	go func() { failures <- server.Serve(listener) }()

	var edgeFailures <-chan error
	var edge contract.EdgeConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "edge.json"), &edge); err == nil {
		if err := validateEdge(edge); err != nil {
			return err
		}
		closeEdge, failures, err := broker.StartEdgeSupervisor(ctx, root, edge)
		if err != nil {
			return err
		}
		defer closeEdge()
		edgeFailures = failures
	} else if !os.IsNotExist(err) {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-edgeFailures:
		return err
	case err := <-failures:
		return err
	}
}

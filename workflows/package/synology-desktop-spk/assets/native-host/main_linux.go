//go:build linux

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dockpipe/src/lib/infrastructure/nativehost"
	"dockpipe/workflows/internal/synology/dsmauth"
	"dockpipe/workflows/internal/synology/endpointtls"
	"dockpipe/workflows/internal/synology/hostidentity"
	"dockpipe/workflows/internal/synology/localaccount"
	"dockpipe/workflows/internal/synology/localhttp"
)

const listenAddress = "127.0.0.1:38143"

func ordinaryPackageIdentity() error {
	metadata, err := hostidentity.Read(nativeSchema)
	if err != nil || metadata.Identity.UID <= 0 || metadata.Identity.EUID != metadata.Identity.UID {
		return errors.New("ordinary_package_identity_required")
	}
	for _, name := range []string{"CapEff", "CapPrm", "CapInh", "CapAmb"} {
		if value, exists := metadata.Identity.Capabilities[name]; exists && value != "0000000000000000" {
			return errors.New("capabilities_not_allowed")
		}
	}
	return nil
}

func serve(ctx context.Context) error {
	configuration, err := endpointtls.Load(endpointRoot)
	if err != nil {
		return errors.New("native_tls_unavailable")
	}
	key, err := readPrivate(clientRoot + "/client.key")
	if err != nil {
		return errors.New("client_key_unavailable")
	}
	leases, err := newLeaseManager(ownerStore{path: endpointRoot + "/owner.json"})
	if err != nil {
		return err
	}
	broker, err := nativehost.NewBroker(nativehost.Options{
		WorkerSocket: workerSocket, ClientSocket: clientSocket, ClientKey: string(key),
		ClientUID: uint32(os.Getuid()), Authorize: leases.authorize,
	})
	if err != nil {
		return errors.New("native_broker_unavailable")
	}
	defer broker.Close()
	username, err := readPrivate(packageRoot + "/home/desktop-settings/username")
	if err != nil {
		return errors.New("stream_credentials_unavailable")
	}
	password, err := readPrivate(packageRoot + "/home/desktop-settings/password")
	if err != nil {
		return errors.New("stream_credentials_unavailable")
	}
	certificate, err := endpointtls.File(packageRoot+"/var/ssl/cert.pem", 0600, os.Geteuid())
	if err != nil {
		return errors.New("stream_certificate_unavailable")
	}
	upstreamTLS, err := pinnedStreamerTLS(certificate)
	if err != nil {
		return err
	}
	transport := &http.Transport{
		Proxy: nil, TLSClientConfig: upstreamTLS, TLSHandshakeTimeout: 3 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second, IdleConnTimeout: 30 * time.Second,
		MaxIdleConns: 16, MaxIdleConnsPerHost: 16, MaxConnsPerHost: 64,
	}
	defer transport.CloseIdleConnections()
	tracker := &streamTracker{connections: make(map[*trackedConnection]string)}
	defer tracker.closeInvalid(func(string) bool { return false })
	handler := &nativeServer{
		leases: leases, broker: broker, authenticate: dsmauth.Authenticate, resolve: localaccount.Resolve,
		desktop: desktopProxy(leases, tracker, transport, string(username), string(password)),
	}
	listener, err := net.Listen("tcp4", listenAddress)
	if err != nil {
		return errors.New("native_https_listener_unavailable")
	}
	defer listener.Close()
	bounded := &localhttp.Listener{Listener: listener, Slots: make(chan struct{}, 64)}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second, IdleTimeout: time.Second, MaxHeaderBytes: 32 << 10}
	server.SetKeepAlivesEnabled(false)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	completed := make(chan error, 2)
	go func() { completed <- broker.Serve(ctx) }()
	go func() { completed <- server.Serve(tls.NewListener(bounded, configuration)) }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var serviceError error
	running := true
	for running {
		select {
		case <-ctx.Done():
			running = false
		case <-completed:
			serviceError = errors.New("native_service_stopped")
			running = false
		case <-ticker.C:
			tracker.closeInvalid(leases.gateAlive)
		}
	}
	cancel()
	leases.revoke()
	_ = broker.Close()
	tracker.closeInvalid(func(string) bool { return false })
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), time.Second)
	defer cancelShutdown()
	if server.Shutdown(shutdown) != nil {
		_ = server.Close()
	}
	return serviceError
}

func main() {
	err := ordinaryPackageIdentity()
	if err == nil {
		if len(os.Args) != 2 {
			err = errors.New("expected_role")
		} else {
			switch os.Args[1] {
			case "folder-prepare", "folder-save":
				err = hostFolderRole(os.Args[1])
			case "endpoint-init":
				err = initializeStorage()
			case "serve":
				// Keep native process memory out of same-UID diagnostic ptrace
				// access. DSM credentials exist only during fixed helper calls.
				_, _, errno := syscall.Syscall(syscall.SYS_PRCTL, 4, 0, 0)
				if errno != 0 {
					err = errors.New("process_protection_unavailable")
					break
				}
				ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
				err = serve(ctx)
				stop()
			default:
				err = errors.New("unknown_role")
			}
		}
	}
	if err != nil {
		if len(os.Args) == 2 && (os.Args[1] == "folder-prepare" || os.Args[1] == "folder-save") {
			fmt.Fprintln(os.Stderr, "Dockpipe host folder setup:", err)
		} else {
			fmt.Fprintln(os.Stderr, "Dockpipe native host service is unavailable.")
		}
		os.Exit(1)
	}
}

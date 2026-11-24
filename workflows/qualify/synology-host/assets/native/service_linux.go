//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"dockpipe/workflows/internal/synology/localhttp"
)

// Diagnostic responses are bounded at 16 KiB. A group list exceeding that
// contract fails explicitly rather than reporting an incomplete identity.
const maximumResponseBytes = 16 << 10
const maximumConnections = 16
const maximumAuthenticationHeaderBytes = 32 << 10

func decodeStatus(response *http.Response) (status, error) {
	if response.StatusCode != http.StatusOK {
		return status{}, errors.New("service unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maximumResponseBytes+1))
	if err != nil || len(data) > maximumResponseBytes {
		return status{}, errors.New("service response exceeds budget")
	}
	var metadata status
	if json.Unmarshal(data, &metadata) != nil || metadata.Schema != schema || !validHostname(metadata.Hostname) || len(metadata.Identity.Groups) > maximumGroups || metadata.Identity.UID <= 0 || metadata.Identity.EUID != os.Geteuid() {
		return status{}, errors.New("invalid service response")
	}
	return metadata, nil
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid()
}

func admitSocket(path string, existing bool) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 107 {
		return errors.New("socket must be an absolute clean Linux Unix socket path")
	}
	parent := filepath.Dir(path)
	// DSM owns FHS aliases above the runtime directory, including package var.
	// Resolve those aliases, while requiring the runtime leaf itself to be real.
	resolvedAnchor, err := filepath.EvalSymlinks(filepath.Dir(parent))
	if err != nil {
		return errors.New("cannot resolve socket directory anchor")
	}
	resolvedParent := filepath.Join(resolvedAnchor, filepath.Base(parent))
	parentInfo, err := os.Lstat(resolvedParent)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 || !ownedByCurrentUser(parentInfo) || parentInfo.Mode().Perm() != 0700 {
		return errors.New("socket parent must be a real directory owned by the service identity with mode 0700")
	}
	info, err := os.Lstat(filepath.Join(resolvedParent, filepath.Base(path)))
	if !existing && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if !existing || err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0600 || !ownedByCurrentUser(info) {
		return errors.New("socket must be a private owned socket, or absent before service start")
	}
	return nil
}

type privateListener struct {
	*net.UnixListener
	slots chan struct{}
}

type loopbackListener struct {
	net.Listener
	slots chan struct{}
}

func peerUID(connection *net.UnixConn) (uint32, error) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return 0, err
	}
	var credential *syscall.Ucred
	var credentialError error
	err = raw.Control(func(descriptor uintptr) {
		credential, credentialError = syscall.GetsockoptUcred(int(descriptor), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return 0, err
	}
	if credentialError != nil {
		return 0, credentialError
	}
	return credential.Uid, nil
}

func (listener *privateListener) Accept() (net.Conn, error) {
	for {
		connection, err := listener.AcceptUnix()
		if err != nil {
			return nil, err
		}
		uid, err := peerUID(connection)
		if err != nil || int(uid) != os.Geteuid() {
			_ = connection.Close()
			continue
		}
		if bounded := reserveConnection(connection, listener.slots); bounded != nil {
			return bounded, nil
		}
	}
}

func (listener *loopbackListener) Accept() (net.Conn, error) {
	return localhttp.AcceptLoopback(listener.Listener, listener.slots)
}
func reserveConnection(connection net.Conn, slots chan struct{}) net.Conn {
	return localhttp.ReserveConnection(connection, slots)
}

func serviceHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		secureHeaders(writer)
		if request.URL.Path != "/v1/status" || request.URL.RawQuery != "" {
			writeFailure(writer, http.StatusNotFound, "not_found")
			return
		}
		if !readOnlyRequest(writer, request) {
			return
		}
		metadata, err := currentStatus()
		if err != nil {
			writeFailure(writer, http.StatusServiceUnavailable, "unavailable")
			return
		}
		writeBoundedJSON(writer, metadata)
	})
}

func serve(ctx context.Context, socket string) error {
	return serveWithAuthentication(ctx, socket, "")
}

func qualificationServer(handler http.Handler) *http.Server {
	return localhttp.Server(handler, 4096, requestTimeout)
}

func serveWithAuthentication(ctx context.Context, socket, authListen string) error {
	ctx, cancelService := context.WithCancel(ctx)
	defer cancelService()
	if os.Getuid() == 0 || os.Geteuid() == 0 {
		return errors.New("qualification service requires an unprivileged identity")
	}
	if err := admitSocket(socket, false); err != nil {
		return err
	}
	if authListen != "" && authListen != authenticationListenAddress {
		return errors.New("authentication listener must be exactly 127.0.0.1:38140")
	}
	// Service is a standalone process; no other goroutine creates files here.
	syscall.Umask(0077)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return errors.New("cannot bind qualification socket")
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0600); err != nil {
		return errors.New("cannot secure qualification socket")
	}
	slots := make(chan struct{}, maximumConnections)
	listeners := []net.Listener{&privateListener{UnixListener: listener, slots: slots}}
	servers := []*http.Server{qualificationServer(serviceHandler())}
	if authListen != "" {
		authListener, err := net.Listen("tcp4", authListen)
		if err != nil {
			return errors.New("cannot bind authentication report listener")
		}
		defer authListener.Close()
		listeners = append(listeners, &loopbackListener{Listener: authListener, slots: slots})
		// The report already executes inside this service. Avoid a second Unix
		// connection, which would need another slot while the caller holds one.
		reportServer := qualificationServer(authenticationGateway(authenticate, inProcessServiceStatus))
		// DSM cookies may occupy 16 KiB; leave the remaining bounded header
		// budget for routing metadata and other browser request headers.
		reportServer.MaxHeaderBytes = maximumAuthenticationHeaderBytes
		servers = append(servers, reportServer)
	}
	return runQualificationServers(ctx, servers, listeners)
}

func runQualificationServers(ctx context.Context, servers []*http.Server, listeners []net.Listener) error {
	completed := make(chan error, len(servers))
	for index, server := range servers {
		go func(server *http.Server, listener net.Listener) {
			completed <- server.Serve(listener)
		}(server, listeners[index])
	}
	var result error
	finished := 0
	select {
	case <-ctx.Done():
	case <-completed:
		finished++
		result = errors.New("qualification HTTP service stopped unexpectedly")
	}
	// Both listeners drain concurrently within the same one-second deadline.
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var shutdown sync.WaitGroup
	for _, server := range servers {
		shutdown.Add(1)
		go func(server *http.Server) {
			defer shutdown.Done()
			if server.Shutdown(shutdownContext) != nil {
				_ = server.Close()
			}
		}(server)
	}
	shutdown.Wait()
	for finished < len(servers) {
		err := <-completed
		finished++
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			result = errors.New("qualification HTTP service failed")
		}
	}
	return result
}

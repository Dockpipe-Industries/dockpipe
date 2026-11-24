//go:build linux

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"dockpipe/workflows/internal/synology/dsmauth"
	"dockpipe/workflows/internal/synology/dsmproxy"
	"dockpipe/workflows/internal/synology/endpointtls"
	"dockpipe/workflows/internal/synology/localaccount"
	"dockpipe/workflows/internal/synology/localhttp"
)

const listenAddress = "127.0.0.1:38142"
const reportScript = "/webman/3rdparty/dockpipe-host-task-qualification/host-auth.cgi"

type authenticateRequest func(context.Context, []string) (string, error)
type resolveUser func(string) (localaccount.Account, error)

func writeJSON(writer http.ResponseWriter, code int, value any) {
	data, err := json.Marshal(value)
	if err != nil || len(data) > maximumResponseBytes {
		code = http.StatusServiceUnavailable
		data = []byte(`{"error":"response_exceeds_budget"}`)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(code)
	_, _ = writer.Write(data)
}

func failure(writer http.ResponseWriter, code int, reason string) {
	writeJSON(writer, code, map[string]string{"error": reason})
}

func gateway(current *state, authenticate authenticateRequest, resolve resolveUser) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/health" && request.Method == http.MethodGet && request.URL.RawQuery == "" {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		if request.URL.Path != "/report" || request.URL.RawQuery != "" {
			failure(writer, http.StatusNotFound, "not_found")
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodPost {
			failure(writer, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		// No unauthenticated listener receives or stores a DSM session. nginx
		// verifies our per-install TLS pin before it forwards these headers.
		environment, err := dsmproxy.Environment(request, reportScript)
		if err != nil || request.Header.Get("Cookie") == "" || len(request.Header.Values("X-Syno-Token")) != 1 || request.Header.Get("X-Syno-Token") == "" {
			failure(writer, http.StatusForbidden, "invalid_authentication_metadata")
			return
		}
		if request.Method == http.MethodPost && request.Header.Get("Origin") != "https://"+request.Host {
			failure(writer, http.StatusForbidden, "origin_required")
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), dsmauth.Timeout)
		defer cancel()
		username, err := authenticate(ctx, environment)
		if err != nil {
			code := http.StatusServiceUnavailable
			if errors.Is(err, dsmauth.ErrUnauthenticated) {
				code = http.StatusUnauthorized
			}
			failure(writer, code, "authentication_unavailable")
			return
		}
		account, err := resolve(username)
		if err != nil {
			failure(writer, http.StatusForbidden, "local_administrator_required")
			return
		}
		prepare, clear := false, false
		expectedNonce := ""
		if request.Method == http.MethodPost {
			if request.Header.Get("Content-Type") != "application/json" {
				failure(writer, http.StatusUnsupportedMediaType, "json_required")
				return
			}
			data, err := io.ReadAll(io.LimitReader(request.Body, 1025))
			var operation struct {
				Action string `json:"action"`
				Nonce  string `json:"nonce,omitempty"`
			}
			decoder := json.NewDecoder(strings.NewReader(string(data)))
			decoder.DisallowUnknownFields()
			if err != nil || len(data) > 1024 || decoder.Decode(&operation) != nil || decoder.Decode(new(any)) != io.EOF {
				failure(writer, http.StatusBadRequest, "invalid_operation")
				return
			}
			expectedNonce = operation.Nonce
			switch operation.Action {
			case "prepare":
				prepare = true
			case "clear":
				clear = true
			default:
				failure(writer, http.StatusBadRequest, "invalid_operation")
				return
			}
		}
		report, err := current.view(account, prepare, clear, expectedNonce)
		if err != nil {
			failure(writer, http.StatusConflict, "qualification_unavailable_or_owned")
			return
		}
		writeJSON(writer, http.StatusOK, report)
	})
}

func serve(ctx context.Context) error {
	configuration, err := endpointtls.Load(packageRoot + "/var")
	if err != nil {
		return errors.New("endpoint_tls_unavailable")
	}
	current := newState()
	callbackListener, err := callbackListener(callbackSocket)
	if err != nil {
		return err
	}
	defer callbackListener.Close()
	listener, err := net.Listen("tcp4", listenAddress)
	if err != nil {
		return errors.New("https_listener_unavailable")
	}
	defer listener.Close()
	callbackContext, stopCallbacks := context.WithCancel(ctx)
	defer stopCallbacks()
	callbacksFinished := make(chan struct{})
	go func() {
		defer close(callbacksFinished)
		acceptCallbacks(callbackContext, callbackListener, current)
	}()
	server := localhttp.Server(gateway(current, dsmauth.Authenticate, localaccount.Resolve), 32<<10, 5*time.Second)
	bounded := &localhttp.Listener{Listener: listener, Slots: make(chan struct{}, 16)}
	completed := make(chan error, 1)
	go func() {
		completed <- server.Serve(tls.NewListener(bounded, configuration))
	}()
	var serviceError error
	select {
	case <-ctx.Done():
	case <-callbacksFinished:
		serviceError = errors.New("callback_listener_stopped")
	case <-completed:
		serviceError = errors.New("https_service_stopped")
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if server.Shutdown(shutdownContext) != nil {
		_ = server.Close()
	}
	stopCallbacks()
	_ = callbackListener.Close()
	<-callbacksFinished
	return serviceError
}

// Package var is a DSM FHS alias. The runtime leaf must be a real, owned
// directory; an existing socket is never removed blindly on startup.
func validRuntimeDirectory(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0755 || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	identifier := os.Geteuid()
	if identifier < 0 || uint64(identifier) > math.MaxUint32 {
		return false
	}
	return ownedBy(info, uint32(identifier))
}

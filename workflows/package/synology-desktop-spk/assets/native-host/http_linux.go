//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"dockpipe/src/lib/infrastructure/nativehost"
	"dockpipe/workflows/internal/synology/dsmauth"
	"dockpipe/workflows/internal/synology/dsmproxy"
	"dockpipe/workflows/internal/synology/localaccount"
)

const nativeSchema = "dockpipe.synology_native_host.v1"
const nativeEndpoint = "/webman/3rdparty/dockpipe-desktop/native-host.cgi"
const desktopPrefix = "/webman/3rdparty/dockpipe-desktop/desktop/"
const maximumHTTPResponseBytes = 64 << 10

type workerBroker interface {
	ExpectWorker(uint32, uint32, string, time.Time) error
	Status() nativehost.Status
	Revoke() error
}

type nativeServer struct {
	// Serialize lifecycle transitions across lease state and broker admission.
	activationMu sync.Mutex
	leases       *leaseManager
	broker       workerBroker
	authenticate func(context.Context, []string) (string, error)
	resolve      func(string) (localaccount.Account, error)
	desktop      http.Handler
}

type publicReport struct {
	Schema            string             `json:"schema"`
	AuthenticatedUser string             `json:"authenticated_user"`
	ExpectedUID       uint32             `json:"expected_uid"`
	Created           bool               `json:"created"`
	LeaseExpiresAt    *time.Time         `json:"lease_expires_at,omitempty"`
	Challenge         *activation        `json:"challenge"`
	Result            *nativehost.Status `json:"result"`
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil || len(data) > maximumHTTPResponseBytes {
		status = http.StatusServiceUnavailable
		data = []byte(`{"error":"response_exceeds_budget"}`)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	_, _ = writer.Write(data)
}

func writeFailure(writer http.ResponseWriter, status int, reason string) {
	writeJSON(writer, status, map[string]string{"error": reason})
}

func (server *nativeServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/health" && request.Method == http.MethodGet && request.URL.RawQuery == "" {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if strings.HasPrefix(request.URL.Path, desktopPrefix) {
		server.desktop.ServeHTTP(writer, request)
		return
	}
	if request.URL.Path != nativeEndpoint || request.URL.RawQuery != "" {
		writeFailure(writer, http.StatusNotFound, "not_found")
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		writeFailure(writer, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	controller := http.NewResponseController(writer)
	if controller.SetReadDeadline(time.Now().Add(5*time.Second)) != nil || controller.SetWriteDeadline(time.Now().Add(8*time.Second)) != nil {
		writeFailure(writer, http.StatusServiceUnavailable, "request_deadline_unavailable")
		return
	}
	environment, err := dsmproxy.Environment(request, nativeEndpoint)
	if err != nil || request.Header.Get("Cookie") == "" || len(request.Header.Values("X-Syno-Token")) != 1 || request.Header.Get("X-Syno-Token") == "" {
		writeFailure(writer, http.StatusForbidden, "invalid_authentication_metadata")
		return
	}
	if request.Method == http.MethodPost && request.Header.Get("Origin") != "https://"+request.Host {
		writeFailure(writer, http.StatusForbidden, "origin_required")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), dsmauth.Timeout)
	defer cancel()
	username, err := server.authenticate(ctx, environment)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, dsmauth.ErrUnauthenticated) {
			status = http.StatusUnauthorized
		}
		writeFailure(writer, status, "authentication_unavailable")
		return
	}
	account, err := server.resolve(username)
	if err != nil {
		writeFailure(writer, http.StatusForbidden, "local_administrator_required")
		return
	}
	server.activationMu.Lock()
	defer server.activationMu.Unlock()
	if ctx.Err() != nil {
		writeFailure(writer, http.StatusRequestTimeout, "authentication_expired")
		return
	}
	if server.leases.checkOwner(account) != nil {
		writeFailure(writer, http.StatusForbidden, "different_operator_configured")
		return
	}
	created := false
	var leaseExpiresAt *time.Time
	if request.Method == http.MethodPost {
		if request.Header.Get("Content-Type") != "application/json" {
			writeFailure(writer, http.StatusUnsupportedMediaType, "json_required")
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
			writeFailure(writer, http.StatusBadRequest, "invalid_operation")
			return
		}
		if ctx.Err() != nil {
			writeFailure(writer, http.StatusRequestTimeout, "authentication_expired")
			return
		}
		switch operation.Action {
		case "prepare":
			if _, err := sessionHash(request); err != nil {
				writeFailure(writer, http.StatusForbidden, "dsm_session_cookie_required")
				return
			}
			challenge, allocated, err := server.leases.prepare(account)
			if err != nil {
				writeFailure(writer, http.StatusConflict, "native_access_unavailable")
				return
			}
			created = allocated
			// The reverse-worker broker admits only a live authenticated grant.
			// Establish this browser lease before installing its expectation.
			cookie, err := server.leases.renew(request, account)
			if err != nil {
				writeFailure(writer, http.StatusConflict, "native_access_unavailable")
				return
			}
			if allocated {
				if server.broker.ExpectWorker(account.UID, account.GID, challenge.Nonce, challenge.ExpiresAt) != nil {
					server.leases.revoke()
					writeFailure(writer, http.StatusServiceUnavailable, "native_worker_unavailable")
					return
				}
			}
			http.SetCookie(writer, cookie)
			leaseExpiresAt = &cookie.Expires
		case "heartbeat":
			cookie, err := server.leases.renew(request, account)
			if err != nil {
				writeFailure(writer, http.StatusConflict, "native_access_unavailable")
				return
			}
			http.SetCookie(writer, cookie)
			leaseExpiresAt = &cookie.Expires
		case "finish":
			if server.leases.finish(account, operation.Nonce) != nil {
				writeFailure(writer, http.StatusConflict, "activation_changed")
				return
			}
		case "disconnect":
			server.leases.revoke()
			if server.broker.Revoke() != nil {
				writeFailure(writer, http.StatusServiceUnavailable, "native_worker_unavailable")
				return
			}
			http.SetCookie(writer, &http.Cookie{Name: gateCookieName, Path: gateCookiePath, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		default:
			writeFailure(writer, http.StatusBadRequest, "invalid_operation")
			return
		}
	}
	challenge, _ := server.leases.publicState()
	status := server.broker.Status()
	var result *nativehost.Status
	if status.Connected && status.UID == account.UID {
		result = &status
	}
	writeJSON(writer, http.StatusOK, publicReport{Schema: nativeSchema, AuthenticatedUser: username, ExpectedUID: account.UID, Created: created, LeaseExpiresAt: leaseExpiresAt, Challenge: challenge, Result: result})
}

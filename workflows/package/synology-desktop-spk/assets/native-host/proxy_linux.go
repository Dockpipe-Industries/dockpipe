//go:build linux

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"
	"sync"
	"time"

	"dockpipe/workflows/internal/synology/dsmproxy"
)

type trackedConnection struct {
	net.Conn
	once    sync.Once
	release func()
}

func (connection *trackedConnection) Close() error {
	err := connection.Conn.Close()
	connection.once.Do(connection.release)
	return err
}

type streamTracker struct {
	mutex       sync.Mutex
	connections map[*trackedConnection]string
}

type trackedWriter struct {
	http.ResponseWriter
	tracker *streamTracker
	lease   string
}

func (writer *trackedWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := writer.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("stream_hijacking_unavailable")
	}
	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}
	tracked := &trackedConnection{Conn: connection}
	tracked.release = func() {
		writer.tracker.mutex.Lock()
		defer writer.tracker.mutex.Unlock()
		delete(writer.tracker.connections, tracked)
	}
	writer.tracker.mutex.Lock()
	writer.tracker.connections[tracked] = writer.lease
	writer.tracker.mutex.Unlock()
	return tracked, buffer, nil
}

func (writer *trackedWriter) Flush() {
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (tracker *streamTracker) closeInvalid(alive func(string) bool) {
	tracker.mutex.Lock()
	stale := make([]*trackedConnection, 0)
	for connection, token := range tracker.connections {
		if !alive(token) {
			stale = append(stale, connection)
		}
	}
	tracker.mutex.Unlock()
	for _, connection := range stale {
		_ = connection.Close()
	}
}

func pinnedStreamerTLS(certificate []byte) (*tls.Config, error) {
	// Existing installations use a self-signed CN-only certificate. Authenticate
	// that exact package-owned DER certificate instead of accepting arbitrary
	// loopback certificates or relying on deprecated CN hostname matching.
	block, _ := pem.Decode(certificate)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("stream_certificate_invalid")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.New("stream_certificate_invalid")
	}
	pin := sha256.Sum256(leaf.Raw)
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // #nosec G402 -- VerifyConnection below authenticates the exact package certificate and its validity interval.
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 || sha256.Sum256(state.PeerCertificates[0].Raw) != pin {
				return errors.New("stream_certificate_pin_mismatch")
			}
			now := time.Now()
			if now.Before(state.PeerCertificates[0].NotBefore) || now.After(state.PeerCertificates[0].NotAfter) {
				return errors.New("stream_certificate_expired")
			}
			return nil
		},
	}, nil
}

func desktopProxy(manager *leaseManager, tracker *streamTracker, transport http.RoundTripper, username, password string) http.Handler {
	proxy := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			request.URL.Scheme = "https"
			request.URL.Host = "127.0.0.1:38139"
			// No DSM cookie, CSRF token, browser Basic credential or internal
			// routing metadata crosses into the GUI container.
			for key := range request.Header {
				lower := strings.ToLower(key)
				if lower == "cookie" || lower == "authorization" || lower == "x-syno-token" || lower == "referer" || strings.HasPrefix(lower, "x-dockpipe-") || strings.HasPrefix(lower, "x-forwarded-") {
					request.Header.Del(key)
				}
			}
			request.Header["X-Forwarded-For"] = nil
			request.SetBasicAuth(username, password)
		},
		Transport:     transport,
		FlushInterval: -1,
		ModifyResponse: func(response *http.Response) error {
			response.Header.Del("Set-Cookie")
			response.Header.Del("WWW-Authenticate")
			response.Header.Set("Cache-Control", "no-store")
			return nil
		},
		ErrorHandler: func(writer http.ResponseWriter, _ *http.Request, _ error) {
			writeFailure(writer, http.StatusBadGateway, "desktop_unavailable")
		},
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, err := dsmproxy.Environment(request, nativeEndpoint); err != nil || !strings.HasPrefix(request.URL.Path, desktopPrefix) || path.Clean(request.URL.Path) != strings.TrimSuffix(request.URL.Path, "/") {
			writeFailure(writer, http.StatusForbidden, "invalid_proxy_metadata")
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead && request.Header.Get("Origin") != "https://"+request.Host {
			writeFailure(writer, http.StatusForbidden, "origin_required")
			return
		}
		if request.URL.Query().Has("SynoToken") || request.URL.Query().Has("_sid") {
			writeFailure(writer, http.StatusBadRequest, "unexpected_session_parameter")
			return
		}
		token, allowed := manager.gate(request)
		if !allowed {
			writeFailure(writer, http.StatusUnauthorized, "native_access_required")
			return
		}
		// Bound ordinary HTTP response streaming by request cancellation when
		// its browser lease expires, just as tracked upgraded WS tunnels close.
		ctx, cancel := context.WithCancel(request.Context())
		defer cancel()
		finished := make(chan struct{})
		defer close(finished)
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-finished:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !manager.gateAlive(token) {
						cancel()
						return
					}
				}
			}
		}()
		proxy.ServeHTTP(&trackedWriter{ResponseWriter: writer, tracker: tracker, lease: token}, request.WithContext(ctx))
	})
}

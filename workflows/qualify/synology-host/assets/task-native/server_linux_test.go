//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dockpipe/workflows/internal/synology/dsmauth"
	"dockpipe/workflows/internal/synology/localaccount"
)

func testRequest(method, body string) *http.Request {
	request := httptest.NewRequest(method, "https://nas.example/report", strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:42000"
	request.Header.Set("Cookie", "id=test-only-fixture")
	request.Header.Set("X-Syno-Token", "fixture-token")
	request.Header.Set("X-Dockpipe-Remote-Addr", "192.0.2.1")
	request.Header.Set("X-Dockpipe-Server-Addr", "192.0.2.2")
	request.Header.Set("X-Dockpipe-Server-Port", "443")
	request.Header.Set("X-Dockpipe-Scheme", "https")
	request.Header.Set("Origin", "https://nas.example")
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestGatewayRejectsAuthenticationAndMutationFailures(t *testing.T) {
	for _, scenario := range []string{"missing-cookie", "missing-token", "foreign-origin", "missing-origin", "wrong-peer", "expired-session", "helper-fault", "non-admin", "oversized", "unknown-field", "trailing-json", "unknown-action", "wrong-content-type"} {
		t.Run(scenario, func(t *testing.T) {
			current := newState()
			request := testRequest(http.MethodPost, `{"action":"prepare"}`)
			authenticationError := error(nil)
			accountError := error(nil)
			switch scenario {
			case "missing-cookie":
				request.Header.Del("Cookie")
			case "missing-token":
				request.Header.Del("X-Syno-Token")
			case "foreign-origin":
				request.Header.Set("Origin", "https://attacker.example")
			case "missing-origin":
				request.Header.Del("Origin")
			case "wrong-peer":
				request.RemoteAddr = "192.0.2.1:2000"
			case "expired-session":
				authenticationError = dsmauth.ErrUnauthenticated
			case "helper-fault":
				authenticationError = errors.New("secret-helper-stderr")
			case "non-admin":
				accountError = errors.New("ineligible-account")
			case "oversized":
				request = testRequest(http.MethodPost, strings.Repeat(" ", 1025))
			case "unknown-field":
				request = testRequest(http.MethodPost, `{"action":"prepare","command":"arbitrary"}`)
			case "trailing-json":
				request = testRequest(http.MethodPost, `{"action":"prepare"} {}`)
			case "unknown-action":
				request = testRequest(http.MethodPost, `{"action":"execute"}`)
			case "wrong-content-type":
				request.Header.Set("Content-Type", "text/plain")
			}
			handler := gateway(current, func(context.Context, []string) (string, error) { return "operator", authenticationError }, func(string) (localaccount.Account, error) { return testAccount(), accountError })
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code < 400 || strings.Contains(recorder.Body.String(), "secret") || current.current != nil {
				t.Fatal("failed request allocated authority or exposed helper output")
			}
		})
	}
}

func TestGatewayReadAndPrepareUseVerifiedAccount(t *testing.T) {
	current := newState()
	handler := gateway(current, func(ctx context.Context, environment []string) (string, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("auth helper has no deadline")
		}
		if !strings.Contains(strings.Join(environment, "\n"), "HTTP_X_SYNO_TOKEN=fixture-token") {
			t.Fatal("session token missing")
		}
		return "operator", nil
	}, func(username string) (localaccount.Account, error) {
		if username != "operator" {
			t.Fatal("unverified username")
		}
		return testAccount(), nil
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, testRequest(http.MethodGet, ""))
	var read response
	if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &read) != nil || read.Challenge != nil {
		t.Fatal("GET mutated challenge")
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, testRequest(http.MethodPost, `{"action":"prepare"}`))
	var prepared response
	if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &prepared) != nil || !prepared.Created || prepared.AuthenticatedUser != "operator" {
		t.Fatal("prepare failed")
	}
}

func TestFailureDiagnosticsNeverExposeUntrustedErrors(t *testing.T) {
	if safeFailureReason(errors.New("private arbitrary stderr")) != "qualification_unavailable" {
		t.Fatal("untrusted error escaped diagnostic allowlist")
	}
	if safeFailureReason(errors.New("probe_callback_unavailable")) != "probe_callback_unavailable" {
		t.Fatal("fixed native task failure reason was hidden")
	}
}

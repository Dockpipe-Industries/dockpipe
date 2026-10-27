//go:build linux || darwin

package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/hardwarekey"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/internal/testsupport/securitykey"
)

const testCeremonyToken = "test-local-ceremony-capability"

func ceremonyRequest(handler http.Handler, method, path string, body []byte, mutate func(*http.Request)) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, hardwarekey.Origin+path, bytes.NewReader(body))
	request.Header.Set("Origin", hardwarekey.Origin)
	request.Header.Set("X-Dockpipe-Ceremony", testCeremonyToken)
	if mutate != nil {
		mutate(request)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestHardwareCeremonyRejectsUntrustedRequestsBeforeEffects(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		mutate func(*http.Request)
		status int
	}{
		{
			name: "missing token", method: http.MethodPost, path: "/options", status: http.StatusForbidden,
			mutate: func(request *http.Request) {
				request.Header.Del("X-Dockpipe-Ceremony")
			},
		},
		{
			name: "wrong token", method: http.MethodPost, path: "/options", status: http.StatusForbidden,
			mutate: func(request *http.Request) {
				request.Header.Set("X-Dockpipe-Ceremony", "other")
			},
		},
		{
			name: "missing origin", method: http.MethodPost, path: "/options", status: http.StatusForbidden,
			mutate: func(request *http.Request) {
				request.Header.Del("Origin")
			},
		},
		{
			name: "wrong origin", method: http.MethodPost, path: "/options", status: http.StatusForbidden,
			mutate: func(request *http.Request) {
				request.Header.Set("Origin", "https://attacker.example")
			},
		},
		{
			name: "wrong host", method: http.MethodPost, path: "/options", status: http.StatusForbidden,
			mutate: func(request *http.Request) {
				request.Host = "attacker.example:47832"
			},
		},
		{
			name: "numeric host", method: http.MethodPost, path: "/options", status: http.StatusForbidden,
			mutate: func(request *http.Request) {
				request.Host = "127.0.0.1:47832"
			},
		},
		{
			name: "wrong port", method: http.MethodPost, path: "/options", status: http.StatusForbidden,
			mutate: func(request *http.Request) {
				request.Host = "localhost:47833"
			},
		},
		{
			name: "query", method: http.MethodPost, path: "/options?unexpected=1", status: http.StatusForbidden,
		},
		{
			name: "wrong method", method: http.MethodGet, path: "/options", status: http.StatusForbidden,
		},
		{
			name: "unknown route", method: http.MethodPost, path: "/unknown", status: http.StatusNotFound,
		},
		{
			name: "finish wrong token", method: http.MethodPost, path: "/finish", status: http.StatusForbidden,
			mutate: func(request *http.Request) {
				request.Header.Del("X-Dockpipe-Ceremony")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			effects := 0
			done := make(chan error, 1)
			handler := hardwareCeremonyHandler(testCeremonyToken, func() (hardwareCeremonyOptions, error) {
				effects++
				return hardwareCeremonyOptions{}, nil
			}, func([]byte) (bool, error) {
				effects++
				return false, nil
			}, done)
			response := ceremonyRequest(handler, test.method, test.path, nil, test.mutate)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if effects != 0 || len(done) != 0 {
				t.Fatal("rejected request reached ceremony callbacks")
			}
		})
	}
}

func TestHardwareCeremonyRegistersSignedCredential(t *testing.T) {
	key := securitykey.New(t)
	registry, err := hardwarekey.NewRegistry(key.RootsPEM)
	if err != nil {
		t.Fatal(err)
	}
	options, session, err := registry.BeginRegistration()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	handler := hardwareCeremonyHandler(testCeremonyToken, func() (hardwareCeremonyOptions, error) {
		return hardwareCeremonyOptions{Kind: "create", Options: options}, nil
	}, func(response []byte) (bool, error) {
		return false, registry.FinishRegistration(*session, response)
	}, done)
	response := ceremonyRequest(handler, http.MethodPost, "/options", nil, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("options rejected: %d", response.Code)
	}
	var browserOptions struct {
		Kind    string          `json:"kind"`
		Options json.RawMessage `json:"options"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &browserOptions); err != nil {
		t.Fatal(err)
	}
	if browserOptions.Kind != "create" {
		t.Fatal("registration ceremony did not request credential creation")
	}
	response = ceremonyRequest(handler, http.MethodPost, "/finish", key.Registration(browserOptions.Options), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("signed registration rejected: %d, %s", response.Code, response.Body.String())
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("successful ceremony did not complete")
	}
	if len(registry.Keys) != 1 || registry.Validate() != nil {
		t.Fatal("signed registration did not establish a trusted credential")
	}
}

func TestHardwareCeremonyCompletesOnlyOnceUnderConcurrentFinish(t *testing.T) {
	finishCalls := 0
	done := make(chan error, 1)
	handler := hardwareCeremonyHandler(testCeremonyToken, func() (hardwareCeremonyOptions, error) {
		return hardwareCeremonyOptions{}, nil
	}, func([]byte) (bool, error) {
		finishCalls++
		return false, nil
	}, done)
	const requests = 8
	statuses := make(chan int, requests)
	var workers sync.WaitGroup
	for range requests {
		workers.Add(1)
		go func() {
			defer workers.Done()
			statuses <- ceremonyRequest(handler, http.MethodPost, "/finish", []byte("{}"), nil).Code
		}()
	}
	workers.Wait()
	close(statuses)
	successes := 0
	for status := range statuses {
		if status == http.StatusOK {
			successes++
		} else if status != http.StatusConflict {
			t.Fatalf("unexpected concurrent finish status: %d", status)
		}
	}
	if successes != 1 || finishCalls != 1 || len(done) != 1 {
		t.Fatal("concurrent finishes produced more than one completion effect")
	}
	response := ceremonyRequest(handler, http.MethodPost, "/options", nil, nil)
	if response.Code != http.StatusConflict {
		t.Fatal("completed ceremony issued fresh options")
	}
}

func TestHardwareCeremonyBoundsAndVerificationFailure(t *testing.T) {
	t.Run("oversized response", func(t *testing.T) {
		finishCalls := 0
		done := make(chan error, 1)
		handler := hardwareCeremonyHandler(testCeremonyToken, func() (hardwareCeremonyOptions, error) {
			return hardwareCeremonyOptions{}, nil
		}, func([]byte) (bool, error) {
			finishCalls++
			return false, nil
		}, done)
		response := ceremonyRequest(handler, http.MethodPost, "/finish", make([]byte, hardwarekey.MaxResponseBytes+1), nil)
		if response.Code != http.StatusRequestEntityTooLarge || finishCalls != 0 || len(done) != 0 {
			t.Fatal("oversized response reached verification or completed ceremony")
		}
	})
	t.Run("malformed response", func(t *testing.T) {
		verificationError := errors.New("invalid registration payload")
		done := make(chan error, 1)
		handler := hardwareCeremonyHandler(testCeremonyToken, func() (hardwareCeremonyOptions, error) {
			return hardwareCeremonyOptions{}, nil
		}, func(response []byte) (bool, error) {
			if json.Valid(response) {
				t.Fatal("expected malformed fixture payload")
			}
			return false, verificationError
		}, done)
		response := ceremonyRequest(handler, http.MethodPost, "/finish", []byte("{"), nil)
		if response.Code != http.StatusConflict {
			t.Fatal("malformed response did not reject ceremony")
		}
		select {
		case err := <-done:
			if !errors.Is(err, verificationError) {
				t.Fatalf("verification error lost: %v", err)
			}
		default:
			t.Fatal("verification failure did not complete ceremony")
		}
		response = ceremonyRequest(handler, http.MethodPost, "/finish", []byte("{}"), nil)
		if response.Code != http.StatusConflict {
			t.Fatal("failed verification could be retried in the same ceremony")
		}
	})
}

func TestHardwareCeremonyPageSecurityHeaders(t *testing.T) {
	handler := hardwareCeremonyHandler(testCeremonyToken, nil, nil, make(chan error, 1))
	response := ceremonyRequest(handler, http.MethodGet, "/", nil, nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "<script>") {
		t.Fatal("local ceremony page did not render")
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("ceremony page lacks private-response headers")
	}
	policy := response.Header().Get("Content-Security-Policy")
	for _, expected := range []string{"default-src 'none'", "script-src 'sha256-", "connect-src 'self'", "frame-ancestors 'none'", "base-uri 'none'", "form-action 'none'"} {
		if !strings.Contains(policy, expected) {
			t.Fatalf("ceremony CSP omits %q", expected)
		}
	}
	response = ceremonyRequest(handler, http.MethodGet, "/?token=leak", nil, nil)
	if response.Code != http.StatusForbidden {
		t.Fatal("ceremony page accepted query parameters")
	}
}

func TestPairingKeyRegistrationRequiresStoppedLocalBroker(t *testing.T) {
	root := filepath.Join(t.TempDir(), "broker")
	if _, err := initialize(root, "127.0.0.1:47831"); err != nil {
		t.Fatal(err)
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "broker.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	err = registerPairingKey(context.Background(), root, "")
	if err == nil || !strings.Contains(err.Error(), "stop the broker") {
		t.Fatalf("registration bypassed live broker lock: %v", err)
	}
	var state remoteio.BrokerState
	if err := remoteio.ReadPrivate(filepath.Join(root, "broker.json"), &state); err != nil {
		t.Fatal(err)
	}
	if state.PairingKeys != nil {
		t.Fatal("rejected registration changed hardware trust")
	}
}

func TestHardwareCeremonyLauncherKeepsCapabilityOutOfArguments(t *testing.T) {
	address := hardwarekey.Origin + "/#" + strings.Repeat("a", 64)
	launcher, cleanup, err := prepareHardwareCeremonyLauncher(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	parsed, err := url.Parse(launcher)
	if err != nil || parsed.Scheme != "file" || parsed.Host != "" || parsed.Fragment != "" || parsed.RawQuery != "" {
		t.Fatalf("launcher must be a plain local file URI: %q", launcher)
	}
	path := filepath.FromSlash(parsed.Path)
	for _, entry := range []struct {
		path      string
		directory bool
	}{
		{path: filepath.Dir(path), directory: true},
		{path: path, directory: false},
	} {
		if err := infrastructure.ValidatePrivatePath(entry.path, entry.directory); err != nil {
			t.Fatalf("launcher is not private: %v", err)
		}
	}
	page, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	scriptStart := strings.Index(string(page), "<script>") + len("<script>")
	scriptEnd := strings.Index(string(page), "</script>")
	if scriptStart < len("<script>") || scriptEnd < scriptStart {
		t.Fatal("launcher does not contain its redirect script")
	}
	script := string(page[scriptStart:scriptEnd])
	encodedAddress := strings.TrimSuffix(strings.TrimPrefix(script, "location.replace("), ");")
	var redirectedAddress string
	if err := json.Unmarshal([]byte(encodedAddress), &redirectedAddress); err != nil || redirectedAddress != address {
		t.Fatal("private launcher does not redirect to the exact ceremony capability")
	}
	if !strings.Contains(string(page), `name="referrer" content="no-referrer"`) {
		t.Fatal("private launcher does not suppress referrers")
	}
	command := hardwareCeremonyBrowserCommand(launcher)
	if command == nil {
		t.Fatal("private launcher was rejected")
	}
	for _, argument := range command.Args {
		if strings.Contains(argument, address) || strings.Contains(argument, strings.Repeat("a", 64)) {
			t.Fatal("browser launch arguments expose the ceremony capability")
		}
	}
	if command.Args[len(command.Args)-1] != launcher {
		t.Fatal("browser opener does not receive the private file URI")
	}
	if hardwareCeremonyBrowserCommand(address) != nil {
		t.Fatal("browser opener permits a raw ceremony capability in its arguments")
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("ceremony cleanup retained the private launcher: %v", err)
	}
}

func TestHardwareCeremonyLauncherFailureDoesNotRetainFiles(t *testing.T) {
	root := t.TempDir()
	notDirectory := filepath.Join(root, "not-directory")
	if err := os.WriteFile(notDirectory, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", notDirectory)
	launcher, cleanup, err := prepareHardwareCeremonyLauncher(hardwarekey.Origin + "/#private-capability")
	if err == nil || launcher != "" || cleanup != nil {
		t.Fatal("unavailable temporary directory produced a usable launcher")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "not-directory" {
		t.Fatalf("failed preparation retained temporary state: %v", err)
	}
}

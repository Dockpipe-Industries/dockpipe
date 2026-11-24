//go:build linux

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"dockpipe/workflows/internal/synology/localaccount"
)

type activationBodyBarrier struct {
	reader  io.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (body *activationBodyBarrier) Read(buffer []byte) (int, error) {
	body.once.Do(func() {
		close(body.entered)
		<-body.release
	})
	return body.reader.Read(buffer)
}

func (*activationBodyBarrier) Close() error { return nil }

type activationHTTPResult struct {
	code int
	body string
}

func activationRequestResult(server *nativeServer, request *http.Request) activationHTTPResult {
	recorder := &deadlineRecorder{httptest.NewRecorder()}
	server.ServeHTTP(recorder, request)
	return activationHTTPResult{code: recorder.Code, body: recorder.Body.String()}
}

func awaitActivationSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(description)
	}
}

func awaitActivationResult(t *testing.T, result <-chan activationHTTPResult) activationHTTPResult {
	t.Helper()
	select {
	case response := <-result:
		return response
	case <-time.After(2 * time.Second):
		t.Fatal("native request did not finish after activation was released")
		return activationHTTPResult{}
	}
}

func TestFirstOwnerBindingSerializesForeignReadAndDisconnect(t *testing.T) {
	for _, operation := range []string{"read", "disconnect"} {
		t.Run(operation, func(t *testing.T) {
			server, broker := fixtureServer(t)
			foreignResolved := make(chan struct{})
			server.authenticate = func(_ context.Context, environment []string) (string, error) {
				for _, value := range environment {
					if value == "HTTP_X_SYNO_TOKEN=foreign-fixture-token" {
						return "other", nil
					}
				}
				return "operator", nil
			}
			server.resolve = func(username string) (localaccount.Account, error) {
				if username == "other" {
					close(foreignResolved)
					return localaccount.Account{Username: "other", UID: 1027, GID: 100}, nil
				}
				return fixtureAccount(), nil
			}
			body := &activationBodyBarrier{
				reader:  strings.NewReader(`{"action":"prepare"}`),
				entered: make(chan struct{}), release: make(chan struct{}),
			}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(body.release) }) }
			t.Cleanup(release)
			ownerRequest := fixtureRequest(http.MethodPost, `{"action":"prepare"}`)
			ownerRequest.Body = body
			ownerDone := make(chan activationHTTPResult, 1)
			go func() { ownerDone <- activationRequestResult(server, ownerRequest) }()
			awaitActivationSignal(t, body.entered, "owner request never reached its pre-binding body barrier")

			manager := server.leases
			manager.mutex.Lock()
			unbound := manager.owner == nil
			manager.mutex.Unlock()
			if !unbound {
				t.Fatal("fixture must pause before the first owner is bound")
			}
			foreignRequest := fixtureRequest(http.MethodGet, "")
			if operation == "disconnect" {
				foreignRequest = fixtureRequest(http.MethodPost, `{"action":"disconnect"}`)
			}
			foreignRequest.Header.Set("X-Syno-Token", "foreign-fixture-token")
			foreignDone := make(chan activationHTTPResult, 1)
			go func() { foreignDone <- activationRequestResult(server, foreignRequest) }()
			awaitActivationSignal(t, foreignResolved, "foreign request did not finish verified account resolution")

			// Both requests have resolved their accounts while ownership is still
			// unset. The foreign request must wait for the complete first binding.
			select {
			case <-foreignDone:
				release()
				_ = awaitActivationResult(t, ownerDone)
				t.Fatal("foreign request escaped the in-progress first-owner transaction")
			case <-time.After(50 * time.Millisecond):
			}
			release()
			ownerResult := awaitActivationResult(t, ownerDone)
			foreignResult := awaitActivationResult(t, foreignDone)
			if ownerResult.code != http.StatusOK || foreignResult.code != http.StatusForbidden {
				t.Fatalf("owner/foreign responses: %d/%d", ownerResult.code, foreignResult.code)
			}
			challenge, owner := manager.publicState()
			if owner == nil || owner.Username != "operator" || owner.UID != 1026 || challenge == nil {
				t.Fatal("foreign request changed the committed owner or activation")
			}
			if strings.Contains(foreignResult.body, challenge.Nonce) || broker.expectations != 1 || broker.revocations != 0 {
				t.Fatal("foreign request disclosed the challenge or revoked the owner's worker")
			}
			if grant, err := manager.authorize(); err != nil || grant.UID != 1026 {
				t.Fatal("foreign disconnect invalidated the owner's live grant")
			}
		})
	}
}

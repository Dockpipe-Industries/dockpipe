//go:build linux

package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/workflows/internal/synology/localaccount"
)

func fixtureAccount() localaccount.Account {
	return localaccount.Account{Username: "operator", UID: 1026, GID: 100, Groups: []uint32{100, 101}}
}

func fixtureLeases(t *testing.T) *leaseManager {
	t.Helper()
	manager, err := newLeaseManager(ownerStore{path: filepath.Join(t.TempDir(), "owner.json")})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func fixtureRequest(method, body string) *http.Request {
	request, _ := http.NewRequest(method, "https://nas.example"+nativeEndpoint, strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:40000"
	request.Header.Set("Cookie", "id=fixture-session")
	request.Header.Set("X-Syno-Token", "fixture-token")
	request.Header.Set("X-Dockpipe-Remote-Addr", "192.0.2.1")
	request.Header.Set("X-Dockpipe-Server-Addr", "192.0.2.2")
	request.Header.Set("X-Dockpipe-Server-Port", "443")
	request.Header.Set("X-Dockpipe-Scheme", "https")
	request.Header.Set("Origin", "https://nas.example")
	request.Header.Set("Content-Type", "application/json")
	return request
}

func attachCookie(request *http.Request, cookie *http.Cookie) {
	request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
}

func TestBrowserLeaseIsBoundToSessionAndExpires(t *testing.T) {
	manager := fixtureLeases(t)
	account := fixtureAccount()
	activation, created, err := manager.prepare(account)
	if err != nil || !created {
		t.Fatal("prepare failed")
	}
	if !strings.HasPrefix(activation.Script, packageRoot+"/target/bin/dockpipe host worker --detach --socket "+workerSocket+" --nonce ") {
		t.Fatal("starter command changed")
	}
	request := fixtureRequest(http.MethodPost, "")
	cookie, err := manager.renew(request, account)
	if err != nil {
		t.Fatal(err)
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != gateCookiePath {
		t.Fatal("browser cookie lacks scope/protections")
	}
	attachCookie(request, cookie)
	token, allowed := manager.gate(request)
	if !allowed || token != cookie.Value {
		t.Fatal("valid browser lease rejected")
	}
	wrongSession := fixtureRequest(http.MethodGet, "")
	wrongSession.Header.Set("Cookie", "id=another-session")
	attachCookie(wrongSession, cookie)
	if _, allowed := manager.gate(wrongSession); allowed {
		t.Fatal("lease cookie replayed under another DSM session")
	}
	duplicateSession := fixtureRequest(http.MethodGet, "")
	duplicateSession.AddCookie(&http.Cookie{Name: "id", Value: "second-session"})
	attachCookie(duplicateSession, cookie)
	if _, allowed := manager.gate(duplicateSession); allowed {
		t.Fatal("ambiguous DSM cookies accepted")
	}
	now := manager.now()
	manager.now = func() time.Time { return now.Add(leaseLifetime + time.Second) }
	if _, allowed := manager.gate(request); allowed {
		t.Fatal("expired browser stream permitted")
	}
	if _, err := manager.authorize(); err == nil {
		t.Fatal("expired browser left native client authorized")
	}
}

func TestFinishPreservesGrantButDisableCannotBeUndoneByHeartbeat(t *testing.T) {
	manager := fixtureLeases(t)
	account := fixtureAccount()
	activation, _, _ := manager.prepare(account)
	cookie, err := manager.renew(fixtureRequest(http.MethodPost, ""), account)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.finish(account, strings.Repeat("0", 64)); err == nil {
		t.Fatal("stale setup finished new generation")
	}
	if err := manager.finish(account, activation.Nonce); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.authorize(); err != nil {
		t.Fatal("starter cleanup revoked active native access")
	}
	challenge, _ := manager.publicState()
	if challenge != nil {
		t.Fatal("finished starter retained activation")
	}
	manager.revoke()
	request := fixtureRequest(http.MethodPost, "")
	attachCookie(request, cookie)
	if _, err := manager.renew(request, account); err == nil {
		t.Fatal("old automatic heartbeat reversed explicit Disable")
	}
	if _, err := manager.authorize(); err == nil {
		t.Fatal("disabled native access remained live")
	}
	next, created, err := manager.prepare(account)
	if err != nil || !created || next.Nonce == activation.Nonce {
		t.Fatal("fresh explicit enable did not create new generation")
	}
}

func TestBindingRejectsOtherPrincipalAndUntrustedFiles(t *testing.T) {
	manager := fixtureLeases(t)
	account := fixtureAccount()
	if _, _, err := manager.prepare(account); err != nil {
		t.Fatal(err)
	}
	other := account
	other.Username = "other-operator"
	other.UID++
	if manager.checkOwner(other) == nil {
		t.Fatal("another principal read configured owner's state")
	}
	if _, _, err := manager.prepare(other); err == nil {
		t.Fatal("another principal inherited stored owner")
	}
	original, err := os.ReadFile(manager.store.path)
	if err != nil {
		t.Fatal(err)
	}
	if os.Chmod(manager.store.path, 0644) != nil {
		t.Fatal("fixture permissions")
	}
	if _, err := newLeaseManager(manager.store); err == nil {
		t.Fatal("loosely permissioned owner file accepted")
	}
	if err := os.WriteFile(manager.store.path, original, 0600); err != nil {
		t.Fatal(err)
	}
}

//go:build linux

package main

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"sync"
	"time"

	"dockpipe/src/lib/infrastructure/nativehost"
	"dockpipe/workflows/internal/synology/localaccount"
)

const leaseLifetime = 30 * time.Second
const activationLifetime = 5 * time.Minute
const maximumBrowserLeases = 16
const gateCookieName = "dockpipe_native_access"
const gateCookiePath = "/webman/3rdparty/dockpipe-desktop/"

type activation struct {
	Nonce     string    `json:"nonce"`
	Script    string    `json:"script"`
	TaskName  string    `json:"task_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

type browserLease struct {
	session [32]byte
	expires time.Time
}

type leaseManager struct {
	mutex      sync.Mutex
	now        func() time.Time
	store      ownerStore
	owner      *ownerBinding
	generation string
	challenge  *activation
	browsers   map[string]browserLease
}

func newLeaseManager(store ownerStore) (*leaseManager, error) {
	owner, err := store.read()
	if err != nil {
		return nil, err
	}
	return &leaseManager{now: time.Now, store: store, owner: owner, browsers: make(map[string]browserLease)}, nil
}

func sessionHash(request *http.Request) ([32]byte, error) {
	// Synology's documented browser-session cookie is named id. Never retain
	// its plaintext value or forward it to the GUI streamer.
	cookies := request.CookiesNamed("id")
	if len(cookies) != 1 || cookies[0].Value == "" || len(cookies[0].Value) > 4096 {
		return [32]byte{}, errors.New("dsm_session_cookie_required")
	}
	return sha256.Sum256([]byte(cookies[0].Value)), nil
}

func (manager *leaseManager) expireLocked() {
	now := manager.now()
	for key, lease := range manager.browsers {
		if !now.Before(lease.expires) {
			delete(manager.browsers, key)
		}
	}
	if manager.challenge != nil && !now.Before(manager.challenge.ExpiresAt) {
		manager.challenge = nil
	}
}

func (manager *leaseManager) authorize() (nativehost.Grant, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.expireLocked()
	if manager.owner == nil || manager.generation == "" || len(manager.browsers) == 0 {
		return nativehost.Grant{}, errors.New("native_access_unavailable")
	}
	return nativehost.Grant{UID: manager.owner.UID, Generation: manager.generation}, nil
}

func (manager *leaseManager) checkOwner(account localaccount.Account) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if !sameOwner(manager.owner, account) {
		return errors.New("different_operator_configured")
	}
	return nil
}

func (manager *leaseManager) renew(request *http.Request, account localaccount.Account) (*http.Cookie, error) {
	digest, err := sessionHash(request)
	if err != nil {
		return nil, err
	}
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.expireLocked()
	if manager.owner == nil || !sameOwner(manager.owner, account) || manager.generation == "" {
		return nil, errors.New("native_access_not_prepared")
	}
	token := ""
	existing := request.CookiesNamed(gateCookieName)
	if len(existing) == 1 {
		lease, found := manager.browsers[existing[0].Value]
		if found && lease.session == digest {
			token = existing[0].Value
		}
	}
	if token == "" {
		if len(manager.browsers) >= maximumBrowserLeases {
			return nil, errors.New("browser_lease_limit")
		}
		token, err = randomToken()
		if err != nil {
			return nil, err
		}
	}
	expires := manager.now().Add(leaseLifetime)
	manager.browsers[token] = browserLease{session: digest, expires: expires}
	return &http.Cookie{Name: gateCookieName, Value: token, Path: gateCookiePath, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: 60}, nil
}

func (manager *leaseManager) prepare(account localaccount.Account) (*activation, bool, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.expireLocked()
	if !sameOwner(manager.owner, account) {
		return nil, false, errors.New("different_operator_configured")
	}
	if manager.challenge != nil {
		return manager.challenge, false, nil
	}
	if err := manager.store.bind(account); err != nil {
		return nil, false, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, false, err
	}
	manager.owner = &ownerBinding{account.Username, account.UID, account.GID}
	manager.generation = token
	manager.browsers = make(map[string]browserLease)
	manager.challenge = &activation{
		Nonce:     token,
		Script:    packageRoot + "/target/bin/dockpipe host worker --detach --socket " + workerSocket + " --nonce " + token,
		TaskName:  "Dockpipe host access " + token[:16],
		ExpiresAt: manager.now().Add(activationLifetime),
	}
	return manager.challenge, true, nil
}

func (manager *leaseManager) finish(account localaccount.Account, nonce string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.expireLocked()
	if !sameOwner(manager.owner, account) || manager.challenge == nil || manager.challenge.Nonce != nonce {
		return errors.New("activation_changed")
	}
	manager.challenge = nil
	return nil
}

func (manager *leaseManager) revoke() {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.browsers = make(map[string]browserLease)
	manager.challenge = nil
	manager.generation = ""
}

func (manager *leaseManager) gate(request *http.Request) (string, bool) {
	digest, err := sessionHash(request)
	if err != nil {
		return "", false
	}
	cookies := request.CookiesNamed(gateCookieName)
	if len(cookies) != 1 {
		return "", false
	}
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.expireLocked()
	lease, found := manager.browsers[cookies[0].Value]
	return cookies[0].Value, found && lease.session == digest
}

func (manager *leaseManager) gateAlive(token string) bool {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.expireLocked()
	_, found := manager.browsers[token]
	return found
}

func (manager *leaseManager) publicState() (*activation, *ownerBinding) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.expireLocked()
	return manager.challenge, manager.owner
}

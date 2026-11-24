//go:build linux

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"dockpipe/workflows/internal/synology/hostidentity"
	"dockpipe/workflows/internal/synology/localaccount"
)

const schema = "dockpipe.synology_host_task_qualification.v1"
const packageRoot = "/var/packages/dockpipe-host-task-qualification"
const callbackSocket = packageRoot + "/var/run/task.sock"
const challengeLifetime = 15 * time.Minute

// A single in-memory challenge binds this one-shot test to one verified operator.
// It is neither a persistent host grant nor a production terminal session.
type challenge struct {
	Nonce     string    `json:"nonce"`
	Script    string    `json:"script"`
	TaskName  string    `json:"task_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

type commandCheck struct {
	Available  bool            `json:"available"`
	Outcome    string          `json:"outcome"`
	Containers []containerInfo `json:"containers,omitempty"`
}

type containerInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type probeReport struct {
	Host   hostidentity.Status `json:"host"`
	Docker commandCheck        `json:"docker"`
	HTTPS  commandCheck        `json:"https"`
}

type verifiedResult struct {
	GroupsMatchAccount bool        `json:"groups_match_account"`
	PeerUID            uint32      `json:"peer_uid"`
	PeerGID            uint32      `json:"peer_gid"`
	PeerPID            int32       `json:"peer_pid"`
	ReceivedAt         time.Time   `json:"received_at"`
	Report             probeReport `json:"report"`
}

type response struct {
	Schema            string          `json:"schema"`
	AuthenticatedUser string          `json:"authenticated_user"`
	Challenge         *challenge      `json:"challenge"`
	Result            *verifiedResult `json:"result"`
	Created           bool            `json:"created"`
}

type state struct {
	mutex   sync.Mutex
	current *challenge
	account localaccount.Account
	result  *verifiedResult
	now     func() time.Time
}

func newState() *state {
	return &state{now: time.Now}
}

func (current *state) expireLocked() {
	if current.current != nil && !current.now().Before(current.current.ExpiresAt) {
		current.current = nil
		current.account = localaccount.Account{}
		current.result = nil
	}
}

func (current *state) view(account localaccount.Account, prepare, clear bool, expectedNonce string) (response, error) {
	current.mutex.Lock()
	defer current.mutex.Unlock()
	current.expireLocked()
	if current.current != nil && (current.account.UID != account.UID || current.account.Username != account.Username) {
		return response{}, errors.New("another_operator_active")
	}
	if clear && current.current != nil && expectedNonce != current.current.Nonce {
		return response{}, errors.New("challenge_changed")
	}
	if clear {
		current.current = nil
		current.account = localaccount.Account{}
		current.result = nil
	}
	created := false
	if prepare && current.current == nil {
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			return response{}, errors.New("challenge_unavailable")
		}
		nonce := hex.EncodeToString(random[:])
		current.current = &challenge{
			Nonce:     nonce,
			Script:    fmt.Sprintf("%s/target/bin/task-host probe --socket %s --nonce %s", packageRoot, callbackSocket, nonce),
			TaskName:  "Dockpipe host test " + nonce[:16],
			ExpiresAt: current.now().Add(challengeLifetime),
		}
		current.account = account
		created = true
	}
	return response{Schema: schema, AuthenticatedUser: account.Username, Challenge: current.current, Result: current.result, Created: created}, nil
}

func validNonce(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}

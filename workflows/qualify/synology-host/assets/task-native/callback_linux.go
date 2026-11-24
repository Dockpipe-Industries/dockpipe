//go:build linux

package main

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

func ownedBy(info os.FileInfo, uid uint32) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uid
}

func callbackListener(path string) (*net.UnixListener, error) {
	if !validRuntimeDirectory(filepath.Dir(path)) {
		return nil, errors.New("runtime_directory_unavailable")
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("callback_path_not_absent")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, errors.New("callback_listener_unavailable")
	}
	// Other identities can connect but cannot replace the package-owned socket.
	// Possession of a nonce is insufficient: kernel peer UID/GID are verified.
	if os.Chmod(path, 0666) != nil {
		_ = listener.Close()
		return nil, errors.New("callback_permissions_unavailable")
	}
	return listener, nil
}

func peerCredentials(connection *net.UnixConn) (*syscall.Ucred, error) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return nil, err
	}
	var credentials *syscall.Ucred
	var credentialError error
	err = raw.Control(func(descriptor uintptr) {
		credentials, credentialError = syscall.GetsockoptUcred(int(descriptor), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return nil, err
	}
	return credentials, credentialError
}

func equalGroups(actual []int, expected []uint32, primary int) bool {
	unique := map[int]bool{primary: true}
	for _, group := range actual {
		unique[group] = true
	}
	if len(unique) != len(expected) {
		return false
	}
	normalized := make([]int, 0, len(unique))
	for group := range unique {
		normalized = append(normalized, group)
	}
	sort.Ints(normalized)
	for index, group := range expected {
		if normalized[index] != int(group) {
			return false
		}
	}
	return true
}

func (current *state) accept(credentials *syscall.Ucred, message callback) bool {
	current.mutex.Lock()
	defer current.mutex.Unlock()
	current.expireLocked()
	if current.current == nil || current.result != nil || credentials == nil || credentials.Pid <= 0 || credentials.Uid == 0 || credentials.Uid != current.account.UID || credentials.Gid != current.account.GID {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(message.Nonce), []byte(current.current.Nonce)) != 1 {
		return false
	}
	identity := message.Report.Host.Identity
	if message.Report.Host.Schema != schema || !validReportedHost(message.Report) || identity.UID != int(credentials.Uid) || identity.EUID != identity.UID || identity.SavedUID != identity.UID || identity.FSUID != identity.UID || identity.GID != int(credentials.Gid) || identity.EGID != identity.GID || identity.SavedGID != identity.GID || identity.FSGID != identity.GID {
		return false
	}
	// Capabilities and supplementary groups are probe diagnostics, not authority
	// to perform operations. Only SO_PEERCRED establishes callback ownership.
	for _, name := range []string{"CapEff", "CapPrm", "CapInh", "CapAmb"} {
		value, found := identity.Capabilities[name]
		if !found && name == "CapAmb" {
			continue
		}
		if !found || value != "0000000000000000" {
			return false
		}
	}
	candidate := &verifiedResult{GroupsMatchAccount: equalGroups(identity.Groups, current.account.Groups, identity.GID), PeerUID: credentials.Uid, PeerGID: credentials.Gid, PeerPID: credentials.Pid, ReceivedAt: current.now(), Report: message.Report}
	// Re-encoding untrusted strings can expand JSON escapes. Never consume a
	// callback whose complete public response would become unreadable.
	envelope := response{Schema: schema, AuthenticatedUser: current.account.Username, Challenge: current.current, Result: candidate}
	encoded, err := json.Marshal(envelope)
	if err != nil || len(encoded) > maximumResponseBytes {
		return false
	}
	current.result = candidate
	return true
}

func acceptCallbacks(ctx context.Context, listener *net.UnixListener, current *state) {
	slots := make(chan struct{}, 8)
	var workers sync.WaitGroup
	defer workers.Wait()
	for {
		connection, err := listener.AcceptUnix()
		if err != nil {
			return
		}
		select {
		case slots <- struct{}{}:
		default:
			_ = connection.Close()
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { <-slots }()
			defer connection.Close()
			stopCancellation := context.AfterFunc(ctx, func() { _ = connection.Close() })
			defer stopCancellation()
			if connection.SetDeadline(time.Now().Add(3*time.Second)) != nil {
				return
			}
			credentials, err := peerCredentials(connection)
			if err != nil || !current.eligiblePeer(credentials) {
				return
			}
			reader := bufio.NewReader(io.LimitReader(connection, maximumReportBytes+1))
			data, err := reader.ReadBytes('\n')
			var message callback
			accepted := err == nil && len(data) <= maximumReportBytes && json.Unmarshal(data, &message) == nil && current.accept(credentials, message)
			_ = json.NewEncoder(connection).Encode(map[string]bool{"accepted": accepted})
		}()
	}
}

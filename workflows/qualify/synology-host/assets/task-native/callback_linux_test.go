//go:build linux

package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"dockpipe/workflows/internal/synology/hostidentity"
	"dockpipe/workflows/internal/synology/localaccount"
)

func callbackFixture(t *testing.T) (string, *state, func()) {
	t.Helper()
	directory, err := os.MkdirTemp("/tmp", "dptask-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	if os.Chmod(directory, 0755) != nil {
		t.Fatal("runtime permissions")
	}
	socket := filepath.Join(directory, "task.sock")
	listener, err := callbackListener(socket)
	if err != nil {
		t.Fatal(err)
	}
	current := newState()
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() { defer close(finished); acceptCallbacks(ctx, listener, current) }()
	stop := func() {
		cancel()
		_ = listener.Close()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Fatal("callback workers outlived service shutdown")
		}
	}
	return socket, current, stop
}

func realAccount(t *testing.T) localaccount.Account {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("requires unprivileged test identity")
	}
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatal(err)
	}
	unique := map[int]bool{os.Getgid(): true}
	for _, group := range groups {
		unique[group] = true
	}
	identifiers := make([]uint32, 0, len(unique))
	for group := range unique {
		identifiers = append(identifiers, uint32(group))
	}
	sort.Slice(identifiers, func(a, b int) bool { return identifiers[a] < identifiers[b] })
	return localaccount.Account{Username: "fixture-operator", UID: uint32(os.Getuid()), GID: uint32(os.Getgid()), Groups: identifiers}
}

func sendCallback(t *testing.T, socket string, data []byte) bool {
	t.Helper()
	connection, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if connection.SetDeadline(time.Now().Add(time.Second)) != nil {
		t.Fatal("deadline")
	}
	if _, err = connection.Write(data); err != nil {
		t.Fatal(err)
	}
	var acknowledgement struct {
		Accepted bool `json:"accepted"`
	}
	if json.NewDecoder(connection).Decode(&acknowledgement) != nil {
		return false
	}
	return acknowledgement.Accepted
}

func TestActualUnixPeerCredentialsAndOversizedCallback(t *testing.T) {
	socket, current, stop := callbackFixture(t)
	defer stop()
	account := realAccount(t)
	challenge, _ := current.view(account, true, false, "")
	oversized := []byte(strings.Repeat(" ", maximumReportBytes) + "\n")
	if sendCallback(t, socket, oversized) {
		t.Fatal("oversized callback accepted")
	}
	host, err := hostidentity.Read(schema)
	if err != nil {
		t.Fatal(err)
	}
	message := callback{Nonce: challenge.Challenge.Nonce, Report: probeReport{Host: host, Docker: commandCheck{Outcome: "docker_start_failed"}, HTTPS: commandCheck{Outcome: "https_unavailable"}}}
	data, _ := json.Marshal(message)
	if !sendCallback(t, socket, append(data, '\n')) {
		t.Fatal("valid native peer callback rejected")
	}
	report, _ := current.view(account, false, false, "")
	if report.Result == nil || report.Result.PeerUID != uint32(os.Getuid()) || report.Result.PeerGID != uint32(os.Getgid()) || report.Result.PeerPID != int32(os.Getpid()) {
		t.Fatal("kernel peer identity not retained")
	}
	if sendCallback(t, socket, append(data, '\n')) {
		t.Fatal("replayed callback accepted")
	}
}

func TestCallbackShutdownCancelsPartialBodyAndRejectsUnsafePath(t *testing.T) {
	socket, current, stop := callbackFixture(t)
	_, _ = current.view(realAccount(t), true, false, "")
	connection, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_, _ = connection.Write([]byte(`{"nonce":`))
	stop()
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatal("socket remained after listener stop")
	}
	directory := filepath.Dir(socket)
	if os.Symlink("/tmp/should-not-be-followed", socket) != nil {
		t.Fatal("fixture symlink")
	}
	if _, err := callbackListener(socket); err == nil {
		t.Fatal("existing linked callback path replaced")
	}
	_ = os.Remove(socket)
	if os.Chmod(directory, 0777) != nil {
		t.Fatal("fixture permissions")
	}
	if _, err := callbackListener(socket); err == nil {
		t.Fatal("writable runtime directory accepted")
	}
}

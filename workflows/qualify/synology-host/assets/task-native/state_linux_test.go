//go:build linux

package main

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"dockpipe/workflows/internal/synology/hostidentity"
	"dockpipe/workflows/internal/synology/localaccount"
)

func testAccount() localaccount.Account {
	return localaccount.Account{Username: "operator", UID: 1026, GID: 100, Groups: []uint32{100, 101}}
}

func testCallback(nonce string) callback {
	disabled := false
	return callback{Nonce: nonce, Report: probeReport{
		Host: hostidentity.Status{Schema: schema, Hostname: "test-nas", Identity: hostidentity.Identity{
			UID: 1026, EUID: 1026, SavedUID: 1026, FSUID: 1026, GID: 100, EGID: 100, SavedGID: 100, FSGID: 100, Groups: []int{101},
			Capabilities: map[string]string{"CapEff": "0000000000000000", "CapPrm": "0000000000000000", "CapInh": "0000000000000000", "CapAmb": "0000000000000000"}, NoNewPrivs: &disabled,
		}},
		Docker: commandCheck{Available: true, Outcome: "ok"},
		HTTPS:  commandCheck{Available: true, Outcome: "https_200"},
	}}
}

func TestPrepareIsSingleWriterReadOnlyAndGenerationBound(t *testing.T) {
	current := newState()
	account := testAccount()
	read, err := current.view(account, false, false, "")
	if err != nil || read.Challenge != nil || read.Created {
		t.Fatal("GET reserved a challenge")
	}
	var writers atomic.Int32
	var wait sync.WaitGroup
	for index := 0; index < 16; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			report, err := current.view(account, true, false, "")
			if err != nil {
				t.Error(err)
				return
			}
			if report.Created {
				writers.Add(1)
			}
		}()
	}
	wait.Wait()
	report, _ := current.view(account, false, false, "")
	if writers.Load() != 1 || !validNonce(report.Challenge.Nonce) {
		t.Fatal("prepare admitted duplicate create owners")
	}
	expires := report.Challenge.ExpiresAt
	repeated, _ := current.view(account, true, false, "")
	if repeated.Created || repeated.Challenge.Nonce != report.Challenge.Nonce || !repeated.Challenge.ExpiresAt.Equal(expires) {
		t.Fatal("idempotent prepare rotated challenge or expiry")
	}
	if _, err := current.view(account, false, true, strings.Repeat("0", 64)); err == nil {
		t.Fatal("stale window cleared another generation")
	}
	other := account
	other.UID++
	if _, err := current.view(other, false, false, ""); err == nil {
		t.Fatal("another operator read active report")
	}
	if _, err := current.view(account, false, true, report.Challenge.Nonce); err != nil {
		t.Fatal(err)
	}
	next, _ := current.view(account, true, false, "")
	if next.Challenge.Nonce == report.Challenge.Nonce {
		t.Fatal("clear reused challenge")
	}
}

func TestCallbackRejectsWrongPeerNonceAndReportWithoutConsuming(t *testing.T) {
	cases := []string{"root", "wrong-uid", "wrong-gid", "wrong-pid", "nonce", "claimed-uid", "capabilities", "unknown-outcome", "expiry"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			current := newState()
			report, _ := current.view(testAccount(), true, false, "")
			message := testCallback(report.Challenge.Nonce)
			credentials := &syscall.Ucred{Pid: 100, Uid: 1026, Gid: 100}
			switch name {
			case "root":
				credentials.Uid = 0
			case "wrong-uid":
				credentials.Uid++
			case "wrong-gid":
				credentials.Gid++
			case "wrong-pid":
				credentials.Pid = 0
			case "nonce":
				message.Nonce = strings.Repeat("0", 64)
			case "claimed-uid":
				message.Report.Host.Identity.EUID = 0
			case "capabilities":
				message.Report.Host.Identity.Capabilities["CapEff"] = "00000000000000c0"
			case "unknown-outcome":
				message.Report.HTTPS.Outcome = "private-untrusted-output"
			case "expiry":
				current.now = func() time.Time { return report.Challenge.ExpiresAt }
			}
			if current.accept(credentials, message) {
				t.Fatal("invalid callback accepted")
			}
			if name != "expiry" && !current.accept(&syscall.Ucred{Pid: 100, Uid: 1026, Gid: 100}, testCallback(report.Challenge.Nonce)) {
				t.Fatal("rejected callback consumed valid challenge")
			}
		})
	}
}

func TestConcurrentCallbackAcceptsExactlyOnce(t *testing.T) {
	current := newState()
	report, _ := current.view(testAccount(), true, false, "")
	var accepted atomic.Int32
	var wait sync.WaitGroup
	for index := 0; index < 16; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if current.accept(&syscall.Ucred{Pid: 100, Uid: 1026, Gid: 100}, testCallback(report.Challenge.Nonce)) {
				accepted.Add(1)
			}
		}()
	}
	wait.Wait()
	if accepted.Load() != 1 {
		t.Fatal("callback result was replayable")
	}
	repeated, _ := current.view(testAccount(), true, false, "")
	if repeated.Result == nil || repeated.Created || repeated.Challenge.Nonce != report.Challenge.Nonce {
		t.Fatal("completed prepare erased proof")
	}
}

func TestDockerInventoryBudgetAndPinnedCommand(t *testing.T) {
	record := `{"ID":"` + strings.Repeat("a", 64) + `","Names":"test","State":"running"}` + "\n"
	inventory, err := parseContainers(strings.Repeat(record, maximumContainers))
	if err != nil || len(inventory) != maximumContainers {
		t.Fatal("inventory boundary rejected")
	}
	if _, err := parseContainers(strings.Repeat(record, maximumContainers+1)); err == nil {
		t.Fatal("inventory overflow accepted")
	}
	if _, err := parseContainers(`{"ID":"not-a-container","Names":"test"}`); err == nil {
		t.Fatal("bad ID accepted")
	}
	// Callback/report metadata never contains scheduler credentials.
	data, _ := json.Marshal(testCallback(strings.Repeat("b", 64)))
	if strings.Contains(string(data), "Cookie") {
		t.Fatal("report includes authentication data")
	}
}

func TestGroupDifferencesAreReportedAsDiagnostics(t *testing.T) {
	current := newState()
	report, _ := current.view(testAccount(), true, false, "")
	message := testCallback(report.Challenge.Nonce)
	message.Report.Host.Identity.Groups = []int{999}
	if !current.accept(&syscall.Ucred{Pid: 100, Uid: 1026, Gid: 100}, message) {
		t.Fatal("group initialization difference hid the host report")
	}
	result, _ := current.view(testAccount(), false, false, "")
	if result.Result.GroupsMatchAccount {
		t.Fatal("different task groups were reported as equivalent account rights")
	}
}

func TestAcceptedBoundaryReportRemainsReadable(t *testing.T) {
	current := newState()
	report, _ := current.view(testAccount(), true, false, "")
	message := testCallback(report.Challenge.Nonce)
	message.Report.Host.Identity.Groups = make([]int, 6000)
	for index := range message.Report.Host.Identity.Groups {
		message.Report.Host.Identity.Groups[index] = 12345678
	}
	frame, _ := json.Marshal(message)
	if len(frame) > maximumReportBytes || len(frame) < 50<<10 {
		t.Fatal("fixture missed the upper callback budget")
	}
	credentials := &syscall.Ucred{Pid: 100, Uid: 1026, Gid: 100}
	if !current.accept(credentials, message) {
		t.Fatal("bounded report was rejected")
	}
	result, _ := current.view(testAccount(), false, false, "")
	encoded, _ := json.Marshal(result)
	if len(encoded) > maximumResponseBytes {
		t.Fatal("accepted callback cannot be read through report endpoint")
	}

	oversized := newState()
	next, _ := oversized.view(testAccount(), true, false, "")
	expanded := testCallback(next.Challenge.Nonce)
	expanded.Report.Docker.Containers = make([]containerInfo, maximumContainers)
	for index := range expanded.Report.Docker.Containers {
		expanded.Report.Docker.Containers[index] = containerInfo{ID: strings.Repeat("a", 64), Name: strings.Repeat("<", 255), State: "running"}
	}
	if oversized.accept(credentials, expanded) || oversized.result != nil {
		t.Fatal("JSON escape expansion stored an unreadable result")
	}
}

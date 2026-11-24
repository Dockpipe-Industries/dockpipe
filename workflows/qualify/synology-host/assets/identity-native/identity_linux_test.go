//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"dockpipe/workflows/internal/synology/boundedcmd"
	"dockpipe/workflows/internal/synology/hostidentity"
)

func capabilityIdentity() hostidentity.Identity {
	return hostidentity.Identity{
		UID: 268603, EUID: 268603, SavedUID: 268603, FSUID: 268603,
		Capabilities: map[string]string{"CapEff": capabilityPair, "CapPrm": capabilityPair, "CapInh": noCapabilities, "CapAmb": noCapabilities},
	}
}

func TestCapabilityAdmissionIsExactAndNonRoot(t *testing.T) {
	if !admitCapabilities(capabilityIdentity(), false) {
		t.Fatal("exact package capability pair rejected")
	}
	for _, field := range []string{"CapEff", "CapPrm", "CapInh", "CapAmb"} {
		metadata := capabilityIdentity()
		metadata.Capabilities[field] = "00000000000000e0"
		if admitCapabilities(metadata, false) {
			t.Fatalf("extra or incorrect %s accepted", field)
		}
	}
	metadata := capabilityIdentity()
	metadata.EUID = 0
	if admitCapabilities(metadata, false) {
		t.Fatal("root effective identity accepted")
	}
	if _, err := prepareCapabilities(false); err == nil {
		t.Fatal("ordinary local test process unexpectedly admitted as capability daemon")
	}
}

const localPasswd = "root:x:0:0:root:/root:/bin/sh\nalice:x:1026:100:Alice:/home/alice:/bin/sh\nservice:x:268603:268603:Service:/nonexistent:/sbin/nologin\n"
const localGroups = "users:x:100:\nadministrators:x:101:alice\nproject:x:65537:alice\n"

func TestLocalAccountResolutionAndDenial(t *testing.T) {
	account, err := resolveLocalAccount("alice", strings.NewReader(localPasswd), strings.NewReader(localGroups), 268603)
	if err != nil || account.UID != 1026 || account.GID != 100 || len(account.Groups) != 3 || account.Groups[2] != 65537 {
		t.Fatalf("local account not resolved completely: %+v %v", account, err)
	}
	for _, username := range []string{"root", "nobody", "service", "domain-user", "sc-service", "dockpipe-host-qualification"} {
		if _, err := resolveLocalAccount(username, strings.NewReader(localPasswd), strings.NewReader(localGroups), 268603); err == nil {
			t.Fatalf("ineligible account %s accepted", username)
		}
	}
	if _, err := resolveLocalAccount("alice", strings.NewReader(localPasswd), strings.NewReader("users:x:100:alice\n"), 268603); err == nil {
		t.Fatal("account without explicit local administrator membership accepted")
	}
	if _, err := resolveLocalAccount("alice", strings.NewReader(localPasswd), strings.NewReader(localGroups), 1026); err == nil {
		t.Fatal("package identity accepted")
	}
	if _, err := resolveLocalAccount("alice", strings.NewReader(strings.Repeat("x", maximumAccountBytes+1)), strings.NewReader(localGroups), 268603); err == nil {
		t.Fatal("account database budget ignored")
	}
}

func TestChildEvidenceIncludesSavedIDsGroupsAndSupervision(t *testing.T) {
	enabled := true
	identity := hostidentity.Identity{
		UID: 1026, EUID: 1026, SavedUID: 1026, FSUID: 1026,
		GID: 100, EGID: 100, SavedGID: 100, FSGID: 100, Groups: []int{101, 100}, NoNewPrivs: &enabled,
		Capabilities: map[string]string{"CapEff": noCapabilities, "CapPrm": noCapabilities, "CapInh": noCapabilities, "CapAmb": noCapabilities},
	}
	report := childReport{Status: hostidentity.Status{Schema: schema, Hostname: "nas", Identity: identity}, Nonce: "challenge", ParentPID: os.Getpid(), ParentDeathSignal: int32(syscall.SIGKILL)}
	account := localAccount{UID: 1026, GID: 100, Groups: []uint32{100, 101}}
	if !verifyChild(report, account, "challenge", "nas") {
		t.Fatal("complete matching child evidence rejected")
	}
	report.Status.Identity.SavedUID = 268603
	if verifyChild(report, account, "challenge", "nas") {
		t.Fatal("retained package saved UID accepted")
	}
	report.Status.Identity.SavedUID = 1026
	report.ParentDeathSignal = 0
	if verifyChild(report, account, "challenge", "nas") {
		t.Fatal("missing parent-death cleanup accepted")
	}
}

func identityRequest() *http.Request {
	request := httptest.NewRequest("GET", "http://nas/report", nil)
	request.RemoteAddr = "127.0.0.1:40000"
	request.Header.Set("Cookie", "id=secret-cookie")
	request.Header.Set("X-Syno-Token", "secret-token")
	request.Header.Set("X-Dockpipe-Remote-Addr", "192.0.2.1")
	request.Header.Set("X-Dockpipe-Server-Addr", "192.0.2.2")
	request.Header.Set("X-Dockpipe-Server-Port", "5001")
	request.Header.Set("X-Dockpipe-Scheme", "https")
	return request
}

func TestIdentityHTTPBoundaryNeverTrustsClientIdentity(t *testing.T) {
	request := identityRequest()
	request.Header.Set("X-DSM-User", "root")
	request.Header.Set("X-DSM-UID", "0")
	handler := identityHandler(func(ctx context.Context, environment []string) (workerResponse, error) {
		if strings.Contains(strings.Join(environment, "\n"), "root") {
			t.Fatal("claimed browser identity reached worker")
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > overallTimeout {
			t.Fatal("overall identity operation has no bounded deadline")
		}
		return workerResponse{Error: "unauthenticated"}, nil
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 401 || strings.Contains(response.Body.String(), "secret") || strings.Contains(response.Body.String(), "identity") {
		t.Fatal("failed authentication exposed identity or credentials")
	}
	request = identityRequest()
	request.Header.Del("Cookie")
	identityHandler(func(context.Context, []string) (workerResponse, error) {
		t.Fatal("no-cookie request invoked capability worker")
		return workerResponse{}, nil
	}).ServeHTTP(httptest.NewRecorder(), request)
}

func TestIdentityHTTPPropagatesCancellation(t *testing.T) {
	request := identityRequest()
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	started := make(chan struct{})
	done := make(chan struct{})
	handler := identityHandler(func(ctx context.Context, _ []string) (workerResponse, error) {
		close(started)
		<-ctx.Done()
		return workerResponse{}, ctx.Err()
	})
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled identity request kept running")
	}
}

func TestPrivateProtocolRejectsIdentityAndCommandInputs(t *testing.T) {
	for _, data := range []string{`{"username":"root"}`, `{"uid":0}`, `{"command":"/bin/sh"}`, `{"environment":[]} {}`} {
		var request workerRequest
		if decodeRequest(strings.NewReader(data), 1024, &request) == nil {
			t.Fatal("unsupported private protocol fields accepted")
		}
	}
}

func TestSupervisorFixture(t *testing.T) {
	switch os.Getenv("IDENTITY_SUPERVISION_TEST") {
	case "child":
		time.Sleep(time.Minute)
		os.Exit(0)
	case "worker":
		runtime.LockOSThread()
		child := exec.Command(os.Args[0], "-test.run=^TestSupervisorFixture$")
		child.Env = append(os.Environ(), "IDENTITY_SUPERVISION_TEST=child")
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
		if child.Start() != nil {
			os.Exit(2)
		}
		if os.WriteFile(os.Getenv("IDENTITY_SUPERVISION_PID_FILE"), []byte(strconv.Itoa(child.Process.Pid)), 0600) != nil {
			os.Exit(3)
		}
		_ = child.Wait()
		os.Exit(0)
	}
}

func TestCancelledSupervisorKillsChildInSeparateProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child-pid")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSupervisorFixture$")
	command.Env = append(os.Environ(), "IDENTITY_SUPERVISION_TEST=worker", "IDENTITY_SUPERVISION_PID_FILE="+pidFile)
	completed := make(chan *boundedcmd.Failure, 1)
	go func() {
		_, failure := boundedcmd.Run(command, 1024)
		completed <- failure
	}()
	deadline := time.Now().Add(time.Second)
	var childPID int
	for {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, err = strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("supervisor did not start its child")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if failure := <-completed; failure == nil || failure.Reason != "cancelled" {
		t.Fatal("supervisor cancellation was not reported")
	}
	deadline = time.Now().Add(time.Second)
	for {
		data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(childPID), "status"))
		if errors.Is(err, os.ErrNotExist) || strings.Contains(string(data), "State:\tZ") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child survived its supervisor despite its separate process group")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestResponseBudgetIsCompleteOrFailure(t *testing.T) {
	writer := httptest.NewRecorder()
	if writeResponse(writer, strings.Repeat("x", maximumResponseBytes)) == nil || writer.Body.Len() != 0 {
		t.Fatal("oversized identity report was partially published")
	}
	var value workerRequest
	if decodeRequest(strings.NewReader(strings.Repeat("x", 1025)), 1024, &value) == nil {
		t.Fatal("private request budget ignored")
	}
	if json.Unmarshal([]byte(`{"environment":[]}`), &value) != nil {
		t.Fatal("fixture protocol invalid")
	}
}

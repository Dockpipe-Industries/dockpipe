package controller

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestQMPFakeSocketNegotiatesAndSendsOnlyTypedPowerdown(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	done := make(chan error, 1)
	go func() {
		_, _ = serverConn.Write([]byte(`{"QMP":{"version":{"qemu":{"major":11}},"capabilities":[]}}` + "\n"))
		reader := bufio.NewReader(serverConn)
		for _, want := range []struct{ command, id string }{{"qmp_capabilities", "capabilities-1"}, {"system_powerdown", "powerdown-1"}} {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				done <- err
				return
			}
			var request map[string]string
			if err = json.Unmarshal(line, &request); err != nil {
				done <- err
				return
			}
			if request["execute"] != want.command || request["id"] != want.id {
				done <- fmt.Errorf("unexpected QMP request: %v", request)
				return
			}
			if want.command == "system_powerdown" {
				_, _ = serverConn.Write([]byte(`{"event":"SHUTDOWN","data":{"guest":true},"timestamp":{"seconds":1,"microseconds":2}}` + "\n"))
			}
			_, _ = serverConn.Write([]byte(`{"return":{},"id":"` + want.id + `"}` + "\n"))
		}
		done <- nil
	}()
	client := QMPClient{Conn: clientConn}
	if err := client.Negotiate("capabilities-1"); err != nil {
		t.Fatal(err)
	}
	if err := client.SystemPowerdown("powerdown-1"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestQMPRejectsWrongResponseIDAfterAsyncEvent(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	go func() {
		reader := bufio.NewReader(serverConn)
		_, _ = reader.ReadBytes('\n')
		_, _ = serverConn.Write([]byte(`{"event":"STOP"}` + "\n"))
		_, _ = serverConn.Write([]byte(`{"return":{},"id":"other"}` + "\n"))
	}()
	_, err := (QMPClient{Conn: clientConn}).Query("query-status", "status-1")
	if err == nil || err.Error() != "QMP response id mismatch" {
		t.Fatalf("wrong response ID must remain fail-closed: %v", err)
	}
}

func TestQMPRejectsMalformedAsyncEvent(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	go func() {
		reader := bufio.NewReader(serverConn)
		_, _ = reader.ReadBytes('\n')
		_, _ = serverConn.Write([]byte(`{"event":"STOP","id":"status-1"}` + "\n"))
	}()
	_, err := (QMPClient{Conn: clientConn}).Query("query-status", "status-1")
	if err == nil || err.Error() != "invalid QMP asynchronous event" {
		t.Fatalf("event/response hybrid must fail closed: %v", err)
	}
}

func TestQMPBoundsAsyncEventSkipping(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	go func() {
		reader := bufio.NewReader(serverConn)
		_, _ = reader.ReadBytes('\n')
		for range maxQMPAsyncEvents + 1 {
			_, _ = serverConn.Write([]byte(`{"event":"STOP"}` + "\n"))
		}
	}()
	_, err := (QMPClient{Conn: clientConn}).Query("query-status", "status-1")
	if err == nil || err.Error() != "QMP asynchronous event limit exceeded" {
		t.Fatalf("asynchronous event stream must remain bounded: %v", err)
	}
}

func TestQMPFakeSocketAllowsQueriesAndRejectsPower(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	go func() {
		reader := bufio.NewReader(serverConn)
		line, _ := reader.ReadBytes('\n')
		var request map[string]string
		_ = json.Unmarshal(line, &request)
		_, _ = serverConn.Write([]byte(`{"return":{"status":"running"},"id":"status-1"}` + "\n"))
	}()
	response, err := (QMPClient{Conn: clientConn}).Query("query-status", "status-1")
	if err != nil || len(response.Return) == 0 {
		t.Fatalf("safe fake QMP query: %+v %v", response, err)
	}
	for _, command := range []string{"quit", "stop", "system_powerdown", "system_reset", "human-monitor-command"} {
		if _, err := (QMPClient{Conn: clientConn}).Query(command, "unsafe"); err == nil {
			t.Fatalf("expected %s rejection", command)
		}
	}
}

func TestHardPowerCanOnlyBePlannedForExactOwnedProcess(t *testing.T) {
	token := "single-use-authentication-token"
	sum := sha256.Sum256([]byte(token))
	process := ProcessIdentity{PID: 4242, UID: 1000, StartTicks: 987654, ExecutableSHA: strings.Repeat("a", 64), CommandSHA: strings.Repeat("b", 64), InstanceRoot: t.TempDir()}
	auth := DestructiveAuthorization{Qualification: true, Disposable: true, MachineUUID: "machine", DiskSerial: "disk", RunID: "run-001", CheckpointAuthenticated: true, ExpectedProcess: process, ObservedProcess: process, ExpectedTokenSHA256: hex.EncodeToString(sum[:]), PresentedToken: token}
	plan, err := PlanHardPower(auth)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Execute || plan.Mechanism != "pidfd_send_signal" || plan.Signal != "SIGKILL" {
		t.Fatalf("hard-power foundation must be inert: %+v", plan)
	}
	auth.ObservedProcess.StartTicks++
	if _, err := PlanHardPower(auth); err == nil {
		t.Fatal("expected PID reuse/process substitution rejection")
	}
	auth.ObservedProcess = process
	auth.PresentedToken = "wrong"
	if _, err := PlanHardPower(auth); err == nil {
		t.Fatal("expected token rejection")
	}
}

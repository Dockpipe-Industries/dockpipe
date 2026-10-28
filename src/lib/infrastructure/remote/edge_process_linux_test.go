//go:build linux

package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func TestEdgeChildFixture(t *testing.T) {
	for _, argument := range os.Args {
		if strings.HasPrefix(argument, "edge-listen=") {
			address := strings.TrimPrefix(argument, "edge-listen=")
			listener, err := net.Listen("tcp4", address)
			if err != nil {
				os.Exit(2)
			}
			server := http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })}
			_ = server.Serve(listener)
			os.Exit(0)
		}
	}
}

func TestEdgeReadinessBelongsToActualSupervisedProcess(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	child, err := startEdgeChild(ctx, contract.EdgeConfig{Executable: executable, Arguments: []string{"-test.run=^TestEdgeChildFixture$", "--", "edge-listen=" + address}})
	if err != nil {
		t.Fatal(err)
	}
	defer child.stop()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if checkEdgeReadiness(ctx, child, "http://"+address+"/fixture-health") == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := checkEdgeReadiness(ctx, child, "http://"+address+"/fixture-health"); err != nil {
		t.Fatal(err)
	}
	if digest, err := child.digest(); err != nil || len(digest) != 64 {
		t.Fatal(digest, err)
	}
	unrelated, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer unrelated.Close()
	if child.ownsReadiness("http://" + unrelated.Addr().String() + "/fixture-health") {
		t.Fatal("unrelated listener was attributed to candidate")
	}
	child.stop()
	if child.alive() {
		t.Fatal("supervised process survived stop")
	}
}

func TestEdgeSnapshotsRejectLinksAndChangedBytes(t *testing.T) {
	root := privateTestDirectory(t)
	path := filepath.Join(root, "binary")
	if err := os.WriteFile(path, []byte("approved"), 0700); err != nil {
		t.Fatal(err)
	}
	expected, err := edgePrivateDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "snapshots", "image")
	if err := snapshotEdgeBinary(path, destination, expected); err != nil {
		t.Fatal(err)
	}
	if err := verifyEdgeSnapshot(destination, expected); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := snapshotEdgeBinary(link, destination, expected); err == nil {
		t.Fatal("linked source accepted")
	}
	if err := os.WriteFile(path, []byte("changed"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := snapshotEdgeBinary(path, destination, expected); err == nil {
		t.Fatal("changed source accepted")
	}
	if err := verifyEdgeSnapshot(destination, expected); err != nil {
		t.Fatal("rejected snapshot corrupted verified bytes", err)
	}
}

func TestEdgeParentFixture(t *testing.T) {
	var addresses []string
	var record string
	for _, argument := range os.Args {
		if strings.HasPrefix(argument, "edge-parent-listen=") {
			addresses = append(addresses, strings.TrimPrefix(argument, "edge-parent-listen="))
		}
		if strings.HasPrefix(argument, "edge-parent-record=") {
			record = strings.TrimPrefix(argument, "edge-parent-record=")
		}
	}
	if record == "" {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(2)
	}
	var pids []int
	for _, address := range addresses {
		child, err := startEdgeChild(context.Background(), contract.EdgeConfig{Executable: executable, Arguments: []string{"-test.run=^TestEdgeChildFixture$", "--", "edge-listen=" + address}})
		if err != nil {
			os.Exit(3)
		}
		pids = append(pids, child.(*edgeChild).command.Process.Pid)
	}
	data, _ := json.Marshal(pids)
	if err := os.WriteFile(record, data, 0600); err != nil {
		os.Exit(4)
	}
	select {}
}

func TestEdgeBrokerParentKillContainsBothConnectorChildren(t *testing.T) {
	var addresses []string
	for index := 0; index < 2; index++ {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addresses = append(addresses, listener.Addr().String())
		listener.Close()
	}
	record := filepath.Join(t.TempDir(), "children.json")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(executable, "-test.run=^TestEdgeParentFixture$", "--", "edge-parent-listen="+addresses[0], "edge-parent-listen="+addresses[1], "edge-parent-record="+record)
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = parent.Process.Kill(); _ = parent.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	var pids []int
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(record)
		if json.Unmarshal(data, &pids) == nil && len(pids) == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(pids) != 2 {
		t.Fatal("fixture parent did not start both children")
	}
	defer func() {
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}()
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = parent.Wait()
	for _, pid := range pids {
		deadline = time.Now().Add(3 * time.Second)
		exited := false
		for time.Now().Before(deadline) {
			data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
			parts := strings.SplitN(string(data), ") ", 2)
			if os.IsNotExist(err) || (len(parts) == 2 && strings.HasPrefix(parts[1], "Z")) {
				exited = true
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if !exited {
			t.Fatal("connector survived abrupt broker-parent death", pid)
		}
	}
}

func TestEdgeLegacyScriptStillRunsWithoutUnsafeInterpreterPinning(t *testing.T) {
	root := privateTestDirectory(t)
	script := filepath.Join(root, "edge-script")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec /bin/sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := contract.EdgeConfig{Schema: contract.Version, Endpoint: "https://edge.example.test", Executable: script, Arguments: []string{"serve"}}
	supervisor := newEdgeSupervisor(ctx, root, base, func(context.Context, string) error { return nil })
	if err := supervisor.boot(); err != nil {
		t.Fatal(err)
	}
	defer supervisor.current.stop()
	if !supervisor.current.alive() || supervisor.baseline.Digest != "" {
		t.Fatal("legacy wrapper was stopped or interpreter falsely pinned")
	}
	if _, err := os.Stat(filepath.Join(root, "edge-selected.json")); !os.IsNotExist(err) {
		t.Fatal("interpreter deployment was persisted")
	}
}

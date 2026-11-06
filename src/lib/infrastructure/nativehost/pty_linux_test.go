package nativehost

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPTYRetainsExitedLeaderUntilSessionCleanup(t *testing.T) {
	identity := requireTestWorkerIdentity(t)
	first, err := startPTY(identity, frame{})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := first.master.Write([]byte("exit 0\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.exited:
	case <-time.After(2 * time.Second):
		t.Fatal("shell did not exit")
	}
	path := fmt.Sprintf("/proc/%d/stat", first.command.Process.Pid)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("session leader was reaped before cleanup: %v", err)
	}
	end := strings.LastIndex(string(data), ")")
	if end < 0 || !strings.HasPrefix(strings.TrimSpace(string(data[end+1:])), "Z ") {
		t.Fatal("exited leader was not retained as a waitable zombie")
	}
	second, err := startPTY(identity, frame{})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if exit := first.Wait(); exit != 0 {
		t.Fatalf("unexpected exit: %d", exit)
	}
	if err := syscall.Kill(second.command.Process.Pid, 0); err != nil {
		t.Fatalf("unrelated session was affected by exited session cleanup: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("leader was not reaped after cleanup")
	}
	second.Close()
	second.Wait()
}

//go:build windows && (amd64 || arm64)

package remote

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

func TestWindowsRemoteHostTerminalConPTY(t *testing.T) {
	if !terminalSupported() {
		t.Skip("installed native Windows terminal provider is unavailable")
	}
	if containerTerminalSupported() {
		t.Fatal("the Windows host provider must not advertise Docker exec")
	}

	root := privateTestDirectory(t)
	release, err := lockTerminalWorker(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	if duplicate, err := lockTerminalWorker(root); err == nil {
		duplicate()
		t.Fatal("a second terminal worker acquired the same state")
	}

	systemRoot := os.Getenv("SystemRoot")
	profile := terminalsession.Profile{
		ID: "owner-terminal", Executable: filepath.Join(systemRoot, "System32", "cmd.exe"),
		Args: []string{"/D", "/Q"}, Dir: root,
		Env: []string{"SystemRoot=" + systemRoot, "PATH=" + filepath.Join(systemRoot, "System32"), "TEMP=" + root, "TMP=" + root},
	}
	authority := terminalsession.Authority{
		Target: invocation.Target{Kind: invocation.Remote, ID: "windows-fixture",
			Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Identity:  invocation.Identity{Issuer: "fixture-broker", Subject: "fixture-operator"},
		Authorize: func(context.Context, invocation.Binding) error { return nil },
	}
	reference := contract.TerminalReference{Schema: contract.TerminalVersion, ID: strings.Repeat("1", 64),
		Target: contract.FeatureTarget{Node: authority.Target.ID, Fingerprint: authority.Target.Fingerprint, Generation: authority.Target.Generation},
		Epoch:  strings.Repeat("c", 64)}
	journal, err := prepareHostTerminalJournal(root, reference, "home")
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.phase("start-requested"); err != nil {
		t.Fatal(err)
	}

	manager, err := newHostTerminalManager(profile, authority)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := manager.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), terminalsession.MaxLease)
	defer cancel()
	session, err := manager.Open(ctx, terminalsession.Size{Columns: 80, Rows: 24}, terminalsession.MaxLease)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Resize(ctx, terminalsession.Size{Columns: 100, Rows: 32}); err != nil {
		t.Fatal(err)
	}
	if err := session.Input(ctx, 1, []byte("set dockpipe_proof=ready\r\necho dockpipe-conpty-%dockpipe_proof%\r\n")); err != nil {
		t.Fatal(err)
	}

	var cursor uint64
	var output strings.Builder
	for {
		chunk, err := session.Read(ctx, cursor, 4096)
		if err != nil {
			t.Fatal(err)
		}
		output.Write(chunk.Data)
		cursor = chunk.Cursor
		if strings.Contains(output.String(), "dockpipe-conpty-ready") {
			break
		}
		select {
		case <-session.OutputReady():
		case <-session.Done():
			t.Fatal("ConPTY exited before returning the command output")
		case <-ctx.Done():
			t.Fatal("ConPTY did not return the command output before the lease expired")
		}
	}
	if err := session.Close(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := session.Wait(ctx)
	if err != nil || result.CleanupError != nil {
		t.Fatalf("ConPTY cleanup was not verified: %v, %v", err, result.CleanupError)
	}
	if err := journal.finish(true); err != nil {
		t.Fatal(err)
	}
}

package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestContainerTerminalReceiptReadWaitsForPrivatePublication(t *testing.T) {
	if !terminalSupported() {
		t.Skip("remote worker platform unavailable")
	}
	root := privateTestDirectory(t)
	reference := containerTerminalTestReference()
	receipt := containerTerminalReceipt{Schema: containerTerminalReceiptSchema, Reference: reference, Resource: containerTerminalTestReceipt(), Uncertain: true}
	if err := saveContainerTerminalReceipt(root, receipt); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	// Hold a real staging file in the same state as WritePrivateJSON before
	// publication. The reader must wait, rather than classify it as damage.
	staged, err := os.CreateTemp(filepath.Join(root, "container-terminals"), ".dockpipe-state-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	defer os.Remove(staged.Name())
	if _, err := staged.WriteString("{"); err != nil {
		t.Fatal(err)
	}
	reading := make(chan struct{})
	finished := make(chan error, 1)
	written := make(chan error, 1)
	go func() {
		written <- saveContainerTerminalReceipt(root, receipt)
	}()
	go func() {
		close(reading)
		receipts, err := containerTerminalReceipts(root)
		if err == nil && (len(receipts) != 1 || receipts[0] != receipt) {
			err = os.ErrInvalid
		}
		finished <- err
	}()
	<-reading
	select {
	case err := <-finished:
		t.Fatalf("reader crossed publication ownership: %v", err)
	case err := <-written:
		t.Fatalf("writer crossed publication ownership: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := staged.Truncate(0); err != nil {
		t.Fatal(err)
	}
	if _, err := staged.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(staged).Encode(receipt); err != nil {
		t.Fatal(err)
	}
	if err := staged.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := staged.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staged.Name(), containerTerminalReceiptPath(root, reference.ID)); err != nil {
		t.Fatal(err)
	}
	unlock()
	unlock = nil
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("reader did not resume after publication")
	}
	select {
	case err := <-written:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("writer did not resume after publication")
	}
}

func TestContainerTerminalReceiptStoreRejectsUnexpectedPublicationArtifacts(t *testing.T) {
	if !terminalSupported() {
		t.Skip("remote worker platform unavailable")
	}
	for _, name := range []string{"unexpected", "unexpected.json", ".dockpipe-state-123456789.tmp"} {
		t.Run(name, func(t *testing.T) {
			root := privateTestDirectory(t)
			receipt := containerTerminalReceipt{Schema: containerTerminalReceiptSchema, Reference: containerTerminalTestReference(), Resource: containerTerminalTestReceipt()}
			if err := saveContainerTerminalReceipt(root, receipt); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "container-terminals", name), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := containerTerminalReceipts(root); err == nil {
				t.Fatal("unexpected store entry was ignored")
			}
		})
	}
}

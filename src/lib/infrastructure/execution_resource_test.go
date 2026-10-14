//go:build linux

package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionResourceAssociationRequiresLiveRunIdentity(t *testing.T) {
	directory := t.TempDir()
	runID, file, _, err := BeginHostRun(directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteHostRunRecord(file, runID, os.Getpid(), directory, "fixture.sh"); err != nil {
		t.Fatal(err)
	}
	if err := RegisterExecutionResource(context.Background(), directory, runID, "qemu", "fixture-vm", os.Getpid()); err != nil {
		t.Fatal(err)
	}
	resource := readExecutionResource(directory, runID)
	if resource == nil || resource.ID != "fixture-vm" || resource.Process.StartToken == "" {
		t.Fatal(resource)
	}
	if err := RegisterExecutionResource(context.Background(), directory, runID, "qemu", "replacement", os.Getpid()); err == nil {
		t.Fatal("association overwritten")
	}
	if err := RegisterExecutionResource(context.Background(), directory, "00000000", "qemu", "unowned", os.Getpid()); err == nil {
		t.Fatal("unregistered root accepted")
	}
	RemoveHostRunArtifacts(file)
	if _, err := os.Stat(filepath.Join(HostRunsDir(directory), runID+".resource")); !os.IsNotExist(err) {
		t.Fatal("sidecar not cleaned")
	}
}

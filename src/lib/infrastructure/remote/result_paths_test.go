package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestResultPathRejectsNonComponentIDs(t *testing.T) {
	root := privateTestDirectory(t)
	broker := &Broker{path: filepath.Join(root, "broker.json")}
	for _, id := range []string{"", ".", "..", "../outside", "a/b", `a\b`, "/tmp/outside", `C:\outside`, "bad\x00id"} {
		if path, err := broker.resultPath(id); err == nil {
			t.Errorf("accepted ID %q as %q", id, path)
		}
		job := contract.Job{
			Submission: contract.Submission{ID: id, Node: "mini", Profile: "bench"},
			Status:     "running", Session: strings.Repeat("b", 64),
			ResultHash: strings.Repeat("c", 64),
		}
		if _, err := broker.withResult(job); err == nil {
			t.Errorf("read result for invalid ID %q", id)
		}
		if err := broker.storeResult(&job, &contract.Result{Status: "success"}); err == nil {
			t.Errorf("stored result for invalid ID %q", id)
		}
	}
	path, err := broker.resultPath("job-1_ok")
	if err != nil || path != filepath.Join(root, "results", "job-1_ok.json") {
		t.Fatalf("valid result path = %q, %v", path, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected IDs changed storage: %v, %v", entries, err)
	}
}

func TestReadPrivateRejectsLinkedAndOversizedFiles(t *testing.T) {
	root := privateTestDirectory(t)
	path := filepath.Join(root, "state.json")
	if err := WritePrivate(path, map[string]string{"value": "inside"}); err != nil {
		t.Fatal(err)
	}
	var value map[string]string
	if err := ReadPrivate(path, &value); err != nil || value["value"] != "inside" {
		t.Fatalf("private file read: %v, %v", value, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((64 << 20) + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()
	if err := ReadPrivate(path, &value); err == nil {
		t.Fatal("accepted oversized private state")
	}
	linked := filepath.Join(root, "linked.json")
	if err := os.Symlink(path, linked); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := ReadPrivate(linked, &value); err == nil {
		t.Fatal("accepted linked private state")
	}
}

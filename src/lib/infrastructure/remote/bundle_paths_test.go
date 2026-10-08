package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestBundleStorageRejectsNonComponentIDs(t *testing.T) {
	root := privateTestDirectory(t)
	broker := &Broker{path: filepath.Join(root, "broker.json")}
	bundle := contract.Bundle{
		WorkflowFile: "config.yml",
		Files:        []contract.BundleFile{{Path: "config.yml", Data: []byte("name: sent\nsteps: []\n")}},
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", ".", "..", "../outside", "a/b", `a\b`, "/tmp/outside", `C:\outside`, "bad\x00id"} {
		if path, err := broker.bundlePath(id); err == nil {
			t.Errorf("accepted ID %q as %q", id, path)
		}
		request := contract.SubmitRequest{
			Submission: contract.Submission{ID: id, Node: "mini", BundleHash: digest},
			Bundle:     &bundle,
		}
		// Call storage directly: safety must not depend on prior queue validation.
		if err := broker.storeBundle(request); err == nil {
			t.Errorf("stored bundle for invalid ID %q", id)
		}
		job := contract.Job{
			Submission: request.Submission,
			Status:     "running", Session: strings.Repeat("a", 64),
		}
		if _, err := broker.loadBundle(job); err == nil {
			t.Errorf("loaded bundle for invalid ID %q", id)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected IDs changed storage: %v, %v", entries, err)
	}
	path, err := broker.bundlePath("job-1_ok")
	if err != nil || path != filepath.Join(root, "bundles", "job-1_ok.json") {
		t.Fatalf("valid bundle path = %q, %v", path, err)
	}
}

func TestBundleStorageRejectsLinkedTargets(t *testing.T) {
	for _, directoryLink := range []bool{false, true} {
		name := "file"
		if directoryLink {
			name = "directory"
		}
		t.Run(name, func(t *testing.T) {
			root := privateTestDirectory(t)
			outside := privateTestDirectory(t)
			broker := &Broker{path: filepath.Join(root, "broker.json")}
			bundle := contract.Bundle{
				WorkflowFile: "config.yml",
				Files:        []contract.BundleFile{{Path: "config.yml", Data: []byte("steps: []\n")}},
			}
			digest, err := bundle.Digest()
			if err != nil {
				t.Fatal(err)
			}
			outsideFile := filepath.Join(outside, "sent.json")
			if err := WritePrivate(outsideFile, bundle); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(outsideFile)
			if err != nil {
				t.Fatal(err)
			}
			bundles := filepath.Join(root, "bundles")
			link, target := bundles, outside
			if !directoryLink {
				if err := PrivateDirectory(bundles); err != nil {
					t.Fatal(err)
				}
				link, target = filepath.Join(bundles, "sent.json"), outsideFile
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			request := contract.SubmitRequest{
				Submission: contract.Submission{ID: "sent", Node: "mini", BundleHash: digest},
				Bundle:     &bundle,
			}
			if err := broker.storeBundle(request); err == nil {
				t.Fatal("accepted linked bundle storage")
			}
			job := contract.Job{Submission: request.Submission, Status: "running", Session: strings.Repeat("a", 64)}
			if _, err := broker.loadBundle(job); err == nil {
				t.Fatal("read linked bundle storage")
			}
			after, err := os.ReadFile(outsideFile)
			if err != nil || string(after) != string(before) {
				t.Fatalf("outside file changed: %v", err)
			}
		})
	}
}

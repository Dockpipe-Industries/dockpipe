package remote

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func sourceFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestBundleSnapshotAndStage(t *testing.T) {
	source := t.TempDir()
	sourceFile(t, source, "work/config.yml", "name: example\ndocker_preflight: false\nsteps:\n  - id: run\n    kind: host\n    run: assets/run.sh\n")
	sourceFile(t, source, "work/assets/run.sh", "#!/bin/sh\necho delivered\n")
	sourceFile(t, source, ".env", "MUST_NOT_COPY=secret")
	bundle, err := BuildBundle(source, "work/config.yml", nil, nil, []string{"result.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Files) != 2 {
		t.Fatal("copied unselected source")
	}
	digest, _ := bundle.Digest()
	submission := contract.Submission{ID: "job", Node: "node", BundleHash: digest}
	root := t.TempDir()
	profile, err := StageBundle(context.Background(), root, submission, *bundle, 30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(profile.Workdir, "work/assets/run.sh")); err != nil {
		t.Fatal(err)
	}
	if _, err := StageBundle(context.Background(), root, submission, *bundle, 30); err == nil {
		t.Fatal("reused staging directory")
	}
	bundle.Files[0].Data = []byte("tampered")
	if _, err := StageBundle(context.Background(), t.TempDir(), submission, *bundle, 30); err == nil {
		t.Fatal("accepted corrupted delivery")
	}
	sourceFile(t, source, "work/.env", "secret")
	if _, err := BuildBundle(source, "work/config.yml", nil, nil, nil); err == nil {
		t.Fatal("copied private file from workflow tree")
	}
}

func TestBundleRejectsSymlinksAndMissingPackageClosure(t *testing.T) {
	source := t.TempDir()
	sourceFile(t, source, "work/config.yml", "name: work\nsteps: []\n")
	sourceFile(t, source, "dependency/package.yml", "name: helper\nkind: workflow\nversion: 1.0.0\ndepends: [missing]\n")
	if _, err := BuildBundle(source, "work/config.yml", nil, []string{"dependency"}, nil); err == nil || !strings.Contains(err.Error(), "requires missing") {
		t.Fatalf("missing package closure: %v", err)
	}
	if err := os.Symlink(filepath.Join(source, "dependency/package.yml"), filepath.Join(source, "work/link")); err != nil {
		t.Skip(err)
	}
	if _, err := BuildBundle(source, "work/config.yml", nil, nil, nil); err == nil {
		t.Fatal("followed source symlink")
	}
}

func TestWorkerMustExplicitlyApproveDelivery(t *testing.T) {
	_, err := deliveryProfile(context.Background(), nil, t.TempDir(), contract.Job{}, nil)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("delivery did not reject before contacting broker")
	}
	if err := ValidateWorkerAuthority(contract.WorkerConfig{}); err == nil {
		t.Fatal("empty worker authority accepted")
	}
	if err := ValidateWorkerAuthority(contract.WorkerConfig{Delivery: &contract.DeliveryPermission{TimeoutSeconds: 30}}); err != nil {
		t.Fatal(err)
	}
}

func TestReceiverRejectsMissingOrForgedPackageManifests(t *testing.T) {
	for _, files := range [][]contract.BundleFile{
		{{Path: "store/assets/data/input.txt"}},
		{{Path: "store/workflows/helper/package.yml", Data: []byte("name: different\nkind: workflow\n")}},
		{{Path: "store/workflows/helper/package.yml", Data: []byte("name: helper\nkind: workflow\ndepends: [absent]\n")}},
		{
			{Path: "store/workflows/helper/package.yml", Data: []byte("name: helper\nkind: workflow\n")},
			{Path: "store/assets/helper/input.txt"},
		},
	} {
		if err := validateBundlePackages(contract.Bundle{Files: files}); err == nil {
			t.Fatal("receiver accepted malformed package closure")
		}
	}
}

func TestSenderRejectsOversizedDependencyManifest(t *testing.T) {
	source := t.TempDir()
	sourceFile(t, source, "work/config.yml", "name: work\nsteps: []\n")
	sourceFile(t, source, "dependency/package.yml", strings.Repeat("x", contract.MaxBundleBytes+1))
	if _, err := BuildBundle(source, "work/config.yml", nil, []string{"dependency"}, nil); err == nil || !strings.Contains(err.Error(), "bounded regular file") {
		t.Fatalf("oversized manifest reached YAML parsing: %v", err)
	}
}

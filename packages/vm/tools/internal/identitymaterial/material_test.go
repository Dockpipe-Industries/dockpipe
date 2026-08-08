package identitymaterial

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
)

func TestPreparedIdentityMaterialIsDurableExactAndConsumedAfterReservation(t *testing.T) {
	base := t.TempDir()
	if err := os.Chmod(base, 0o700); err != nil {
		t.Fatal(err)
	}
	contract := provisioning.Contract{Schema: provisioning.Schema, Purpose: manifest.QualificationPurpose, Disposable: true, InstanceCount: 1, RunID: "run-001", CohortID: "cohort-001", MachineUUID: "11111111-1111-4111-8111-111111111111", DiskSerial: "dockpipe-qual-data-001", FilesystemUUID: "44444444-4444-4444-8444-444444444444", Roots: provisioning.Roots{Instances: filepath.Join(base, "instances"), Evidence: filepath.Join(base, "evidence"), Config: filepath.Join(base, "config"), Runtime: filepath.Join(base, "runtime")}}
	stageParent := filepath.Join(base, "stage-parent")
	if err := os.Mkdir(stageParent, 0o700); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(stageParent, "identity")
	descriptor, err := Prepare(stage, filepath.Join(base, "checkout"), contract.RunID, contract.CohortID, contract.Roots, time.Unix(1_800_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	contract.BootstrapNonce = descriptor.BootstrapNonce
	contract.Artifacts.ControllerPublicKeySHA256 = descriptor.ControllerPublicKeySHA256
	contract.Artifacts.GuestPublicKeySHA256 = descriptor.GuestPublicKeySHA256
	loaded, keys, err := Load(stage, filepath.Join(base, "checkout"), contract, time.Unix(1_800_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(stage, filepath.Join(base, "checkout"), contract, time.Unix(1_800_000_000, 0).Add(MaxLifetime)); err == nil {
		t.Fatal("expected expired identity-material rejection")
	}
	destinationParent := filepath.Join(base, "destination")
	if err := os.Mkdir(destinationParent, 0o700); err != nil {
		t.Fatal(err)
	}
	reserved, err := provisioning.ReserveIdentity(filepath.Join(destinationParent, "identity"), contract, keys)
	if err != nil {
		t.Fatal(err)
	}
	if err := Consume(stage, loaded, reserved); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		t.Fatalf("staging bundle survived consumption: %v", err)
	}
	if _, err := os.Stat(reserved.ControllerPrivateKey); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityMaterialRejectsOverlapTamperingAndSubstitution(t *testing.T) {
	base := t.TempDir()
	_ = os.Chmod(base, 0o700)
	roots := provisioning.Roots{Instances: filepath.Join(base, "instances"), Evidence: filepath.Join(base, "evidence"), Config: filepath.Join(base, "config"), Runtime: filepath.Join(base, "runtime")}
	if _, err := Prepare(filepath.Join(roots.Config, "stage"), "", "run-001", "cohort-001", roots, time.Now()); err == nil {
		t.Fatal("expected live-root overlap rejection")
	}
	parent := filepath.Join(base, "private")
	_ = os.Mkdir(parent, 0o700)
	stage := filepath.Join(parent, "stage")
	d, err := Prepare(stage, "", "run-001", "cohort-001", roots, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := provisioning.Contract{RunID: "run-001", CohortID: "cohort-001", BootstrapNonce: d.BootstrapNonce, Artifacts: provisioning.Artifacts{ControllerPublicKeySHA256: d.ControllerPublicKeySHA256, GuestPublicKeySHA256: d.GuestPublicKeySHA256}}
	if err := os.WriteFile(filepath.Join(stage, "unexpected"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(stage, "", c, time.Now()); err == nil {
		t.Fatal("expected inventory substitution rejection")
	}
}

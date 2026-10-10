package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
)

func TestRetainedGate3InputsPreserveExactBytesAndLoadOnlyFromDeterministicPaths(t *testing.T) {
	execution, contract, contractJSON, qualification, qualificationJSON := retainedGate3Fixture(t)
	paths, err := StoreRetainedGate3Inputs(execution, contract, contractJSON, qualification, qualificationJSON)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(execution.ProvisioningRoots.Config, "instances", execution.RunID, execution.CohortID, "gate3-inputs")
	if paths.Root != wantRoot || paths.Provisioning != filepath.Join(wantRoot, "provisioning-contract.json") || paths.Qualification != filepath.Join(wantRoot, "qualification-manifest.json") {
		t.Fatalf("retained paths changed: %+v", paths)
	}
	for path, want := range map[string][]byte{paths.Provisioning: contractJSON, paths.Qualification: qualificationJSON} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			t.Fatalf("retained input mode or type changed: path=%s info=%v err=%v", path, info, err)
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
			t.Fatalf("retained input owner changed: path=%s uid=%d", path, stat.Uid)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("retained input bytes changed: %s", path)
		}
	}
	loadedContract, loadedQualification, err := LoadRetainedGate3Inputs(execution, paths.Provisioning, paths.Qualification)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loadedContract, contract) || !reflect.DeepEqual(loadedQualification, qualification) {
		t.Fatal("loaded retained Gate 3 inputs changed")
	}
	if _, err := BuildGate3Plan(execution, loadedContract, loadedQualification); err != nil {
		t.Fatalf("retained inputs did not produce the v1 inert plan: %v", err)
	}
	if _, err := StoreRetainedGate3Inputs(execution, contract, contractJSON, qualification, qualificationJSON); err == nil {
		t.Fatal("retained input root was not created exclusively")
	}
	if got, err := os.ReadFile(paths.Provisioning); err != nil || !reflect.DeepEqual(got, contractJSON) {
		t.Fatalf("exclusive-create failure changed retained bytes: %v", err)
	}
}

func TestRetainedGate3InputFailurePreservesPartialRoot(t *testing.T) {
	execution, contract, contractJSON, qualification, qualificationJSON := retainedGate3Fixture(t)
	writes := 0
	injected := func(path string, data []byte, mode os.FileMode) error {
		writes++
		if writes == 2 {
			return fmt.Errorf("injected qualification retention failure")
		}
		return durableRetainedInput(path, data, mode)
	}
	paths, err := storeRetainedGate3Inputs(execution, contract, contractJSON, qualification, qualificationJSON, injected)
	if err == nil || !strings.Contains(err.Error(), "injected qualification retention failure") {
		t.Fatalf("expected injected retained-input failure, got %v", err)
	}
	if info, statErr := os.Lstat(paths.Root); statErr != nil || !info.IsDir() {
		t.Fatalf("partial retained root was removed: info=%v err=%v", info, statErr)
	}
	if got, readErr := os.ReadFile(paths.Provisioning); readErr != nil || !reflect.DeepEqual(got, contractJSON) {
		t.Fatalf("first durable retained input was removed or changed: %v", readErr)
	}
	if _, statErr := os.Lstat(paths.Qualification); !os.IsNotExist(statErr) {
		t.Fatalf("failed second retained input unexpectedly exists: %v", statErr)
	}
}

func TestGate3V1RejectsRetainedInputPathMetadataJSONAndIdentityDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, execution Contract, contract provisioning.Contract, qualification manifest.Manifest, paths RetainedGate3InputPaths)
		load   func(execution Contract, paths RetainedGate3InputPaths) error
	}{
		{
			name: "alternate path",
			load: func(execution Contract, paths RetainedGate3InputPaths) error {
				_, _, err := LoadRetainedGate3Inputs(execution, paths.Provisioning+".alternate", paths.Qualification)
				return err
			},
		},
		{
			name: "absent input",
			mutate: func(t *testing.T, _ Contract, _ provisioning.Contract, _ manifest.Manifest, paths RetainedGate3InputPaths) {
				if err := os.Remove(paths.Provisioning); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlink input",
			mutate: func(t *testing.T, _ Contract, _ provisioning.Contract, _ manifest.Manifest, paths RetainedGate3InputPaths) {
				data, err := os.ReadFile(paths.Provisioning)
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(t.TempDir(), "target.json")
				if err := os.WriteFile(target, data, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(paths.Provisioning); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, paths.Provisioning); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "mode drift",
			mutate: func(t *testing.T, _ Contract, _ provisioning.Contract, _ manifest.Manifest, paths RetainedGate3InputPaths) {
				if err := os.Chmod(paths.Qualification, 0o640); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "malformed JSON",
			mutate: func(t *testing.T, _ Contract, _ provisioning.Contract, _ manifest.Manifest, paths RetainedGate3InputPaths) {
				if err := os.WriteFile(paths.Provisioning, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "trailing JSON",
			mutate: func(t *testing.T, _ Contract, _ provisioning.Contract, _ manifest.Manifest, paths RetainedGate3InputPaths) {
				data, err := os.ReadFile(paths.Qualification)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(paths.Qualification, append(data, []byte("{}")...), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unknown JSON field",
			mutate: func(t *testing.T, _ Contract, contract provisioning.Contract, _ manifest.Manifest, paths RetainedGate3InputPaths) {
				data, err := json.Marshal(contract)
				if err != nil {
					t.Fatal(err)
				}
				data = append(data[:len(data)-1], []byte(`,"unknown":true}`)...)
				if err := os.WriteFile(paths.Provisioning, data, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "contract mismatch",
			mutate: func(t *testing.T, _ Contract, contract provisioning.Contract, _ manifest.Manifest, paths RetainedGate3InputPaths) {
				contract.CohortID = "cohort-substituted"
				data, err := json.Marshal(contract)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(paths.Provisioning, data, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "identity mismatch",
			mutate: func(t *testing.T, _ Contract, _ provisioning.Contract, qualification manifest.Manifest, paths RetainedGate3InputPaths) {
				qualification.MachineUUID = "44444444-4444-4444-8444-444444444444"
				data, err := json.Marshal(qualification)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(paths.Qualification, data, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execution, contract, contractJSON, qualification, qualificationJSON := retainedGate3Fixture(t)
			paths, err := StoreRetainedGate3Inputs(execution, contract, contractJSON, qualification, qualificationJSON)
			if err != nil {
				t.Fatal(err)
			}
			if test.mutate != nil {
				test.mutate(t, execution, contract, qualification, paths)
			}
			if test.load != nil {
				err = test.load(execution, paths)
			} else {
				_, _, err = LoadRetainedGate3Inputs(execution, paths.Provisioning, paths.Qualification)
			}
			if err == nil {
				t.Fatalf("Gate 3 v1 accepted %s", test.name)
			}
		})
	}
}

func TestRetainedGate3InputOwnerGuardRejectsDifferentUID(t *testing.T) {
	execution, contract, contractJSON, qualification, qualificationJSON := retainedGate3Fixture(t)
	paths, err := StoreRetainedGate3Inputs(execution, contract, contractJSON, qualification, qualificationJSON)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readRetainedInput(paths.Provisioning, os.Geteuid()+1); err == nil {
		t.Fatal("retained input accepted owner drift")
	}
}

func retainedGate3Fixture(t *testing.T) (Contract, provisioning.Contract, []byte, manifest.Manifest, []byte) {
	t.Helper()
	execution := executorFixture(t)
	qualification, err := manifest.Load(filepath.Join("..", "..", "..", "manifests", "linux-qualification.json"))
	if err != nil {
		t.Fatal(err)
	}
	qualification.RunID = execution.RunID
	contract := provisioning.Contract{
		Schema: provisioning.Schema, Purpose: manifest.QualificationPurpose, Disposable: true, InstanceCount: 1,
		RunID: execution.RunID, CohortID: execution.CohortID, MachineUUID: qualification.MachineUUID, DiskSerial: qualification.DataDisk.Serial, FilesystemUUID: qualification.Filesystem.UUID,
		Roots: *execution.ProvisioningRoots, Artifacts: provisioning.Artifacts{HarnessBinarySHA256: qualification.Protocol.HarnessSHA256},
	}
	contractSHA256, err := contract.Digest()
	if err != nil {
		t.Fatal(err)
	}
	execution.ContractSHA256 = contractSHA256
	execution.ExecutionSHA256, err = execution.Digest()
	if err != nil {
		t.Fatal(err)
	}
	configRoot := filepath.Join(execution.ProvisioningRoots.Config, "instances", execution.RunID, execution.CohortID)
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	contractJSON, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	contractJSON = append(contractJSON, '\n')
	qualificationJSON, err := json.MarshalIndent(qualification, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	qualificationJSON = append(qualificationJSON, '\n')
	return execution, contract, contractJSON, qualification, qualificationJSON
}

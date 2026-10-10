package executor

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/protocol"
	"dockpipe.vm/tools/internal/provisioning"
)

func TestGate3ReconstitutionAuthenticatesDurableProofAndRemainsInert(t *testing.T) {
	execution, executorPath, verificationPath := reconstitutionFixture(t)
	reconstitution, err := ReconstituteGate3(executorPath)
	if err != nil {
		t.Fatal(err)
	}
	if reconstitution.ExecutionSHA256 != execution.ExecutionSHA256 || reconstitution.Gate2.SignedSequences[3] != 4 || !reconstitution.Gate2.Health || !reconstitution.Gate2.LaunchHashesMatched || !reconstitution.Gate2.CleanExit || reconstitution.PrivateKeysRead || reconstitution.LiveAuthorized || reconstitution.Execute || reconstitution.CleanupAuthorized {
		t.Fatalf("unexpected reconstitution: %+v", reconstitution)
	}
	reconstitutionPath := filepath.Join(t.TempDir(), "gate3-reconstitution.json")
	reconstitutionJSON, _ := json.Marshal(reconstitution)
	if err := os.WriteFile(reconstitutionPath, reconstitutionJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	loadedExecution, executorFileSHA256, err := LoadWithSHA256(executorPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadGate3Reconstitution(reconstitutionPath, loadedExecution, executorFileSHA256)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildGate3PlanFromReconstitution(loadedExecution, loaded, executorFileSHA256)
	if err != nil || plan.Schema != Gate3ReconstitutedPlanSchema || plan.ReconstitutionSHA256 != loaded.ReconstitutionSHA256 {
		t.Fatalf("reconstituted planning failed: plan=%+v err=%v", plan, err)
	}
	if _, err := ExecuteGate3(context.Background(), plan, loadedExecution, Gate3Authorization{}, time.Now(), &fakeGate3Runner{}); err == nil || !strings.Contains(err.Error(), "inert") {
		t.Fatalf("reconstituted plan conveyed execution authority: %v", err)
	}
	if err := (Gate3Authorization{}).Validate(plan, loadedExecution, time.Now()); err == nil {
		t.Fatal("reconstituted plan conveyed authorization authority")
	}
	if err := StoreGate3Result(plan, Gate3Result{}); err == nil {
		t.Fatal("reconstituted plan accepted an execution result")
	}
	tampered, err := os.ReadFile(verificationPath)
	if err != nil {
		t.Fatal(err)
	}
	tampered = []byte(strings.Replace(string(tampered), `"healthy":true`, `"healthy":false`, 1))
	if err := os.WriteFile(verificationPath, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReconstituteGate3(executorPath); err == nil {
		t.Fatal("reconstitution accepted tampered signed evidence")
	}
}

func reconstitutionFixture(t *testing.T) (Contract, string, string) {
	t.Helper()
	execution := executorFixture(t)
	controllerPublic, controllerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	guestPublic, guestPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	machineUUID := "11111111-1111-4111-8111-111111111111"
	filesystemUUID := "22222222-2222-4222-8222-222222222222"
	bootID := "33333333-3333-4333-8333-333333333333"
	diskSerial := "dockpipe-data-000001"
	scenario := "sqlite-rollback-recovery"
	durability := "host-crash-power-loss"
	nonce := strings.Repeat("3", 64)
	controllerBinarySHA256 := strings.Repeat("4", 64)
	guestBinarySHA256 := strings.Repeat("5", 64)
	harnessSHA256 := strings.Repeat("6", 64)
	config := provisioning.AgentConfig{
		Schema: "dockpipe.vm.guest-agent-config.v3", ControllerPublicKeyPath: "/etc/dockpipe-agent/controller.pub", ControllerPublicKeySHA256: testSHA(controllerPublic),
		GuestPrivateKeyPath: "/etc/dockpipe-agent/guest.key", GuestPublicKeySHA256: testSHA(guestPublic), ControllerBinarySHA256: controllerBinarySHA256, GuestAgentBinarySHA256: guestBinarySHA256,
		HarnessBinaryPath: "/usr/libexec/dockpipe-sqlite-vm-harness", HarnessBinarySHA256: harnessSHA256, QualificationRoot: manifest.QualificationMount,
		MachineUUID: machineUUID, DiskSerial: diskSerial, RunID: execution.RunID, Scenario: scenario, DurabilityBoundary: durability, BootstrapNonce: nonce, BootIDSource: manifest.KernelBootIDSource,
	}
	configJSON, _ := json.Marshal(config)
	userData := []byte("#cloud-config\nwrite_files:\n  - path: /etc/dockpipe-agent/config.json\n    owner: dockpipe-agent:dockpipe-agent\n    permissions: \"0400\"\n    encoding: b64\n    defer: true\n    content: " + base64.StdEncoding.EncodeToString(configJSON) + "\n")
	for index := range execution.NoCloud.Files {
		if execution.NoCloud.Files[index].Name == "user-data" {
			execution.NoCloud.Files[index].Content = userData
			execution.NoCloud.Files[index].SHA256 = testSHA(userData)
		}
	}
	execution.Guest.Bootstrap.BootstrapNonce = nonce
	execution.ExecutionSHA256, err = execution.Digest()
	if err != nil {
		t.Fatal(err)
	}
	identityContract := provisioning.Contract{
		Schema: provisioning.Schema, Purpose: manifest.QualificationPurpose, Disposable: true, InstanceCount: 1,
		RunID: execution.RunID, CohortID: execution.CohortID, MachineUUID: machineUUID, DiskSerial: diskSerial, FilesystemUUID: filesystemUUID, BootstrapNonce: nonce,
		Artifacts: provisioning.Artifacts{ControllerPublicKeySHA256: testSHA(controllerPublic), GuestPublicKeySHA256: testSHA(guestPublic)},
	}
	identityRoot := filepath.Join(execution.ProvisioningRoots.Config, "instances", execution.RunID, execution.CohortID, "identity")
	if err := os.MkdirAll(filepath.Dir(identityRoot), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := provisioning.ReserveIdentity(identityRoot, identityContract, provisioning.KeyMaterial{ControllerPublic: controllerPublic, ControllerPrivate: controllerPrivate, GuestPublic: guestPublic, GuestPrivate: guestPrivate}); err != nil {
		t.Fatal(err)
	}
	evidenceRoot := filepath.Dir(execution.Guest.Evidence)
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	contextBase := protocol.Context{MachineUUID: machineUUID, DiskSerial: diskSerial, BootID: bootID, Sequence: 1, RunID: execution.RunID, Nonce: nonce, Scenario: scenario, DurabilityBoundary: durability, Phase: protocol.BootstrapPhase}
	bootstrapPayload := protocol.IdentityBootstrapPayload{BootIDSource: manifest.KernelBootIDSource, ControllerPublicKeySHA256: testSHA(controllerPublic), GuestPublicKeySHA256: testSHA(guestPublic), ControllerBinarySHA256: controllerBinarySHA256, GuestAgentBinarySHA256: guestBinarySHA256}
	bootstrapFrame, err := protocol.Sign(protocol.BootstrapKind, "identity/v1", contextBase, bootstrapPayload, now, now.Add(time.Minute), guestPrivate)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapEvidence, _ := json.Marshal(struct {
		Schema string          `json:"schema"`
		BootID string          `json:"boot_id"`
		Frame  json.RawMessage `json:"frame"`
	}{"dockpipe.vm.bootstrap-evidence.v1", bootID, bootstrapFrame})
	if err := os.WriteFile(execution.Guest.Bootstrap.ExclusiveEvidencePath, bootstrapEvidence, 0o600); err != nil {
		t.Fatal(err)
	}
	results := make([]json.RawMessage, 0, 3)
	payloads := []any{
		map[string]string{"machine_uuid": machineUUID, "disk_serial": diskSerial, "boot_id": bootID},
		map[string]bool{"healthy": true},
		map[string]any{"controller_binary_sha256": controllerBinarySHA256, "guest_agent_binary_sha256": guestBinarySHA256, "matched": true},
	}
	for index, capability := range execution.Guest.Capabilities {
		frameContext := contextBase
		frameContext.Sequence = uint64(index + 2)
		frameContext.Nonce = strings.Repeat(string(rune('a'+index)), 64)
		frameContext.Phase = "verification"
		frame, err := protocol.Sign(protocol.ResultKind, capability, frameContext, payloads[index], now.Add(time.Duration(index+1)*time.Second), now.Add(time.Minute), guestPrivate)
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, frame)
	}
	verificationEvidence, _ := json.Marshal(struct {
		Schema  string            `json:"schema"`
		BootID  string            `json:"boot_id"`
		Results []json.RawMessage `json:"results"`
	}{"dockpipe.vm.verification-evidence.v1", bootID, results})
	if err := os.WriteFile(execution.Guest.Evidence, verificationEvidence, 0o600); err != nil {
		t.Fatal(err)
	}
	shutdownEvidence, _ := json.Marshal(map[string]any{"schema": "dockpipe.vm.shutdown-evidence.v1", "command": ControlledPowerdown, "pid": 12345, "clean_exit": true})
	if err := os.WriteFile(execution.Shutdown.Evidence, shutdownEvidence, 0o600); err != nil {
		t.Fatal(err)
	}
	executorPath := filepath.Join(t.TempDir(), "executor.json")
	if err := Store(executorPath, execution); err != nil {
		t.Fatal(err)
	}
	return execution, executorPath, execution.Guest.Evidence
}

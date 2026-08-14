package executor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"dockpipe.vm/tools/internal/protocol"
	"dockpipe.vm/tools/internal/provisioning"
)

const Gate3ReconstitutionSchema = "dockpipe.vm.gate3-reconstitution.v1"

type Gate2EvidenceProof struct {
	BootstrapPath       string   `json:"bootstrap_path"`
	BootstrapSHA256     string   `json:"bootstrap_sha256"`
	VerificationPath    string   `json:"verification_path"`
	VerificationSHA256  string   `json:"verification_sha256"`
	ShutdownPath        string   `json:"shutdown_path"`
	ShutdownSHA256      string   `json:"shutdown_sha256"`
	BootID              string   `json:"boot_id"`
	SignedSequences     []uint64 `json:"signed_sequences"`
	Health              bool     `json:"health"`
	LaunchHashesMatched bool     `json:"launch_hashes_matched"`
	ShutdownCommand     string   `json:"shutdown_command"`
	ShutdownPID         int      `json:"shutdown_pid"`
	CleanExit           bool     `json:"clean_exit"`
}

// Gate3Reconstitution is a read-only, proof-bound replacement for the lost
// provisioning and qualification preimages needed by inert Gate 3 planning.
// It is deliberately insufficient for authorization or execution.
type Gate3Reconstitution struct {
	Schema                 string             `json:"schema"`
	ReconstitutionSHA256   string             `json:"reconstitution_sha256"`
	ExecutorPath           string             `json:"executor_path"`
	ExecutorFileSHA256     string             `json:"executor_file_sha256"`
	ExecutionSHA256        string             `json:"execution_sha256"`
	ContractSHA256         string             `json:"contract_sha256"`
	ProvisioningSHA256     string             `json:"provisioning_sha256"`
	ToolchainSHA256        string             `json:"toolchain_sha256"`
	RunID                  string             `json:"run_id"`
	CohortID               string             `json:"cohort_id"`
	IdentityRoot           string             `json:"identity_root"`
	MachineUUID            string             `json:"machine_uuid"`
	DiskSerial             string             `json:"disk_serial"`
	FilesystemUUID         string             `json:"filesystem_uuid"`
	Scenario               string             `json:"scenario"`
	DurabilityBoundary     string             `json:"durability_boundary"`
	BootIDSource           string             `json:"boot_id_source"`
	BootstrapNonceSHA256   string             `json:"bootstrap_nonce_sha256"`
	ControllerPublicSHA256 string             `json:"controller_public_sha256"`
	GuestPublicSHA256      string             `json:"guest_public_sha256"`
	ControllerBinarySHA256 string             `json:"controller_binary_sha256"`
	GuestAgentBinarySHA256 string             `json:"guest_agent_binary_sha256"`
	HarnessSHA256          string             `json:"harness_sha256"`
	Gate2                  Gate2EvidenceProof `json:"gate2"`
	HistoricalEvidenceOnly bool               `json:"historical_evidence_only"`
	PlanningOnly           bool               `json:"planning_only"`
	PrivateKeysRead        bool               `json:"private_keys_read"`
	LiveAuthorized         bool               `json:"live_authorized"`
	Execute                bool               `json:"execute"`
	CleanupAuthorized      bool               `json:"cleanup_authorized"`
}

func ReconstituteGate3(executorPath string) (Gate3Reconstitution, error) {
	var out Gate3Reconstitution
	execution, executorFileSHA256, err := LoadWithSHA256(executorPath)
	if err != nil {
		return out, err
	}
	if execution.Schema != Schema || execution.ProvisioningRoots == nil {
		return out, fmt.Errorf("Gate 3 reconstitution requires the current qualified executor")
	}
	var userData []byte
	for _, file := range execution.NoCloud.Files {
		if file.Name == "user-data" {
			if userData != nil {
				return out, fmt.Errorf("executor contains duplicate NoCloud user-data")
			}
			userData = file.Content
		}
	}
	if userData == nil {
		return out, fmt.Errorf("executor does not contain sealed NoCloud user-data")
	}
	config, err := provisioning.RecoverAgentConfig(userData)
	if err != nil {
		return out, err
	}
	identityRoot := filepath.Join(execution.ProvisioningRoots.Config, "instances", execution.RunID, execution.CohortID, "identity")
	identity, err := provisioning.LoadReservedPublicIdentity(identityRoot)
	if err != nil {
		return out, err
	}
	record := identity.Record
	if record.RunID != execution.RunID || record.CohortID != execution.CohortID || record.MachineUUID != config.MachineUUID || record.DiskSerial != config.DiskSerial || record.BootstrapNonce != config.BootstrapNonce || config.RunID != execution.RunID || config.BootstrapNonce != execution.Guest.Bootstrap.BootstrapNonce || config.BootIDSource != execution.Guest.Bootstrap.BootIDSource {
		return out, fmt.Errorf("durable identity, executor, and rendered guest configuration do not match")
	}
	controllerPublicSHA256 := reconstitutionHash(identity.ControllerPublic)
	guestPublicSHA256 := reconstitutionHash(identity.GuestPublic)
	if controllerPublicSHA256 != config.ControllerPublicKeySHA256 || guestPublicSHA256 != config.GuestPublicKeySHA256 {
		return out, fmt.Errorf("durable public identities do not match the rendered pins")
	}
	bootstrapJSON, bootstrapSHA256, err := readEvidence(execution.Guest.Bootstrap.ExclusiveEvidencePath)
	if err != nil {
		return out, err
	}
	verificationJSON, verificationSHA256, err := readEvidence(execution.Guest.Evidence)
	if err != nil {
		return out, err
	}
	shutdownJSON, shutdownSHA256, err := readEvidence(execution.Shutdown.Evidence)
	if err != nil {
		return out, err
	}
	var bootstrapEvidence struct {
		Schema string          `json:"schema"`
		BootID string          `json:"boot_id"`
		Frame  json.RawMessage `json:"frame"`
	}
	if err := decodeExactJSON(bootstrapJSON, &bootstrapEvidence); err != nil || bootstrapEvidence.Schema != "dockpipe.vm.bootstrap-evidence.v1" {
		return out, fmt.Errorf("durable bootstrap evidence is invalid: %v", err)
	}
	bootstrap, err := protocol.VerifyRecorded(bootstrapEvidence.Frame, identity.GuestPublic)
	if err != nil {
		return out, fmt.Errorf("authenticate recorded bootstrap: %w", err)
	}
	wantContext := protocol.Context{MachineUUID: config.MachineUUID, DiskSerial: config.DiskSerial, RunID: config.RunID, Scenario: config.Scenario, DurabilityBoundary: config.DurabilityBoundary}
	if bootstrap.Kind != protocol.BootstrapKind || bootstrap.Capability != "identity/v1" || bootstrap.Context.MachineUUID != wantContext.MachineUUID || bootstrap.Context.DiskSerial != wantContext.DiskSerial || bootstrap.Context.RunID != wantContext.RunID || bootstrap.Context.Scenario != wantContext.Scenario || bootstrap.Context.DurabilityBoundary != wantContext.DurabilityBoundary || bootstrap.Context.Nonce != config.BootstrapNonce || bootstrap.Context.Sequence != protocol.BootstrapSequence || bootstrap.Context.Phase != protocol.BootstrapPhase || bootstrap.Context.BootID != bootstrapEvidence.BootID {
		return out, fmt.Errorf("recorded bootstrap identity does not match the durable inputs")
	}
	wantPayload := protocol.IdentityBootstrapPayload{BootIDSource: config.BootIDSource, ControllerPublicKeySHA256: config.ControllerPublicKeySHA256, GuestPublicKeySHA256: config.GuestPublicKeySHA256, ControllerBinarySHA256: config.ControllerBinarySHA256, GuestAgentBinarySHA256: config.GuestAgentBinarySHA256}
	var bootstrapPayload protocol.IdentityBootstrapPayload
	if err := decodeRecordedPayload(bootstrap.Payload, &bootstrapPayload); err != nil || bootstrapPayload != wantPayload {
		return out, fmt.Errorf("recorded bootstrap payload does not match the durable pins: %v", err)
	}
	var verificationEvidence struct {
		Schema  string            `json:"schema"`
		BootID  string            `json:"boot_id"`
		Results []json.RawMessage `json:"results"`
	}
	if err := decodeExactJSON(verificationJSON, &verificationEvidence); err != nil || verificationEvidence.Schema != "dockpipe.vm.verification-evidence.v1" || verificationEvidence.BootID != bootstrap.Context.BootID || len(verificationEvidence.Results) != len(execution.Guest.Capabilities) {
		return out, fmt.Errorf("durable verification evidence is invalid: %v", err)
	}
	sequences := []uint64{bootstrap.Context.Sequence}
	nonces := map[string]bool{bootstrap.Context.Nonce: true}
	for index, raw := range verificationEvidence.Results {
		frame, err := protocol.VerifyRecorded(raw, identity.GuestPublic)
		if err != nil {
			return out, fmt.Errorf("authenticate recorded verification result %d: %w", index+1, err)
		}
		sequence := execution.Guest.FirstRequestSequence + uint64(index)
		if frame.Kind != protocol.ResultKind || frame.Capability != execution.Guest.Capabilities[index] || frame.Context.MachineUUID != wantContext.MachineUUID || frame.Context.DiskSerial != wantContext.DiskSerial || frame.Context.BootID != bootstrap.Context.BootID || frame.Context.RunID != wantContext.RunID || frame.Context.Scenario != wantContext.Scenario || frame.Context.DurabilityBoundary != wantContext.DurabilityBoundary || frame.Context.Sequence != sequence || frame.Context.Phase != "verification" || nonces[frame.Context.Nonce] {
			return out, fmt.Errorf("recorded verification context %d changed", index+1)
		}
		if err := validateRecordedResult(frame, config.ControllerBinarySHA256, config.GuestAgentBinarySHA256); err != nil {
			return out, err
		}
		nonces[frame.Context.Nonce] = true
		sequences = append(sequences, sequence)
	}
	var shutdown struct {
		Schema    string `json:"schema"`
		Command   string `json:"command"`
		PID       int    `json:"pid"`
		CleanExit bool   `json:"clean_exit"`
	}
	if err := decodeExactJSON(shutdownJSON, &shutdown); err != nil || shutdown.Schema != "dockpipe.vm.shutdown-evidence.v1" || shutdown.Command != execution.Shutdown.Command || shutdown.Command != ControlledPowerdown || shutdown.PID < 1 || !shutdown.CleanExit {
		return out, fmt.Errorf("durable shutdown evidence is invalid: %v", err)
	}
	out = Gate3Reconstitution{
		Schema: Gate3ReconstitutionSchema, ExecutorPath: executorPath, ExecutorFileSHA256: executorFileSHA256,
		ExecutionSHA256: execution.ExecutionSHA256, ContractSHA256: execution.ContractSHA256, ProvisioningSHA256: execution.PlanSHA256, ToolchainSHA256: execution.ToolchainSHA256,
		RunID: execution.RunID, CohortID: execution.CohortID, IdentityRoot: identityRoot, MachineUUID: record.MachineUUID, DiskSerial: record.DiskSerial, FilesystemUUID: record.FilesystemUUID,
		Scenario: config.Scenario, DurabilityBoundary: config.DurabilityBoundary, BootIDSource: config.BootIDSource, BootstrapNonceSHA256: reconstitutionHash([]byte(config.BootstrapNonce)),
		ControllerPublicSHA256: controllerPublicSHA256, GuestPublicSHA256: guestPublicSHA256, ControllerBinarySHA256: config.ControllerBinarySHA256, GuestAgentBinarySHA256: config.GuestAgentBinarySHA256, HarnessSHA256: config.HarnessBinarySHA256,
		Gate2:                  Gate2EvidenceProof{BootstrapPath: execution.Guest.Bootstrap.ExclusiveEvidencePath, BootstrapSHA256: bootstrapSHA256, VerificationPath: execution.Guest.Evidence, VerificationSHA256: verificationSHA256, ShutdownPath: execution.Shutdown.Evidence, ShutdownSHA256: shutdownSHA256, BootID: bootstrap.Context.BootID, SignedSequences: sequences, Health: true, LaunchHashesMatched: true, ShutdownCommand: shutdown.Command, ShutdownPID: shutdown.PID, CleanExit: shutdown.CleanExit},
		HistoricalEvidenceOnly: true, PlanningOnly: true,
	}
	out.ReconstitutionSHA256, err = out.Digest()
	if err != nil {
		return Gate3Reconstitution{}, err
	}
	return out, out.Validate(execution, executorFileSHA256)
}

func (r Gate3Reconstitution) Digest() (string, error) {
	copy := r
	copy.ReconstitutionSHA256 = ""
	copy.PrivateKeysRead = false
	copy.LiveAuthorized = false
	copy.Execute = false
	copy.CleanupAuthorized = false
	b, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	return reconstitutionHash(b), nil
}

func (r Gate3Reconstitution) Validate(execution Contract, executorFileSHA256 string) error {
	digest, err := r.Digest()
	if err != nil || r.Schema != Gate3ReconstitutionSchema || r.ReconstitutionSHA256 != digest || r.ExecutorFileSHA256 != executorFileSHA256 || !isSHA256(r.ExecutorFileSHA256) || r.ExecutionSHA256 != execution.ExecutionSHA256 || r.ContractSHA256 != execution.ContractSHA256 || r.ProvisioningSHA256 != execution.PlanSHA256 || r.ToolchainSHA256 != execution.ToolchainSHA256 || r.RunID != execution.RunID || r.CohortID != execution.CohortID {
		return fmt.Errorf("Gate 3 reconstitution identity or digest is invalid")
	}
	if !filepath.IsAbs(r.ExecutorPath) || filepath.Clean(r.ExecutorPath) != r.ExecutorPath || !filepath.IsAbs(r.IdentityRoot) || execution.ProvisioningRoots == nil || r.IdentityRoot != filepath.Join(execution.ProvisioningRoots.Config, "instances", execution.RunID, execution.CohortID, "identity") {
		return fmt.Errorf("Gate 3 reconstitution durable paths changed")
	}
	for _, sum := range []string{r.BootstrapNonceSHA256, r.ControllerPublicSHA256, r.GuestPublicSHA256, r.ControllerBinarySHA256, r.GuestAgentBinarySHA256, r.HarnessSHA256, r.Gate2.BootstrapSHA256, r.Gate2.VerificationSHA256, r.Gate2.ShutdownSHA256} {
		if !isSHA256(sum) {
			return fmt.Errorf("Gate 3 reconstitution hash pin is invalid")
		}
	}
	if r.Gate2.BootstrapPath != execution.Guest.Bootstrap.ExclusiveEvidencePath || r.Gate2.VerificationPath != execution.Guest.Evidence || r.Gate2.ShutdownPath != execution.Shutdown.Evidence || !slices.Equal(r.Gate2.SignedSequences, []uint64{1, 2, 3, 4}) || !r.Gate2.Health || !r.Gate2.LaunchHashesMatched || r.Gate2.ShutdownCommand != ControlledPowerdown || r.Gate2.ShutdownPID < 1 || !r.Gate2.CleanExit {
		return fmt.Errorf("Gate 3 reconstitution qualification proof is incomplete")
	}
	if !r.HistoricalEvidenceOnly || !r.PlanningOnly || r.PrivateKeysRead || r.LiveAuthorized || r.Execute || r.CleanupAuthorized {
		return fmt.Errorf("Gate 3 reconstitution cannot convey live or cleanup authority")
	}
	if r.MachineUUID == "" || r.DiskSerial == "" || r.FilesystemUUID == "" || r.Scenario == "" || r.DurabilityBoundary == "" || r.BootIDSource == "" || r.Gate2.BootID == "" {
		return fmt.Errorf("Gate 3 reconstitution identity proof is incomplete")
	}
	return nil
}

func LoadGate3Reconstitution(path string, execution Contract, executorFileSHA256 string) (Gate3Reconstitution, error) {
	var reconstitution Gate3Reconstitution
	if err := decodeOwnerOnly(path, &reconstitution); err != nil {
		return reconstitution, err
	}
	return reconstitution, reconstitution.Validate(execution, executorFileSHA256)
}

func readEvidence(path string) ([]byte, string, error) {
	data, err := readOwnerOnly(path)
	if err != nil {
		return nil, "", fmt.Errorf("read durable qualification evidence %s: %w", filepath.Base(path), err)
	}
	return data, reconstitutionHash(data), nil
}

func reconstitutionHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func decodeRecordedPayload(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return fmt.Errorf("recorded payload contains trailing JSON")
	}
	return nil
}

func validateRecordedResult(frame protocol.SignedFrame, controllerSHA256, guestSHA256 string) error {
	switch frame.Capability {
	case "identity/v1":
		var payload struct {
			MachineUUID string `json:"machine_uuid"`
			DiskSerial  string `json:"disk_serial"`
			BootID      string `json:"boot_id"`
		}
		if err := decodeRecordedPayload(frame.Payload, &payload); err != nil || payload.MachineUUID != frame.Context.MachineUUID || payload.DiskSerial != frame.Context.DiskSerial || payload.BootID != frame.Context.BootID {
			return fmt.Errorf("recorded identity result mismatch: %v", err)
		}
	case "health/v1":
		var payload struct {
			Healthy bool `json:"healthy"`
		}
		if err := decodeRecordedPayload(frame.Payload, &payload); err != nil || !payload.Healthy {
			return fmt.Errorf("recorded health result mismatch: %v", err)
		}
	case "launch-hash-pinned/v1":
		var payload struct {
			ControllerBinarySHA256 string `json:"controller_binary_sha256"`
			GuestAgentBinarySHA256 string `json:"guest_agent_binary_sha256"`
			Matched                bool   `json:"matched"`
		}
		if err := decodeRecordedPayload(frame.Payload, &payload); err != nil || !payload.Matched || payload.ControllerBinarySHA256 != controllerSHA256 || payload.GuestAgentBinarySHA256 != guestSHA256 {
			return fmt.Errorf("recorded launch-pin result mismatch: %v", err)
		}
	default:
		return fmt.Errorf("unexpected recorded verification capability")
	}
	return nil
}

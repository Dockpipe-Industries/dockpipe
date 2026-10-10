package guest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/protocol"
)

const ConfigSchema = "dockpipe.vm.guest-agent-config.v3"

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Config struct {
	Schema                    string `json:"schema"`
	ControllerPublicKeyPath   string `json:"controller_public_key_path"`
	ControllerPublicKeySHA256 string `json:"controller_public_key_sha256"`
	GuestPrivateKeyPath       string `json:"guest_private_key_path"`
	GuestPublicKeySHA256      string `json:"guest_public_key_sha256"`
	ControllerBinarySHA256    string `json:"controller_binary_sha256"`
	GuestAgentBinarySHA256    string `json:"guest_agent_binary_sha256"`
	HarnessBinaryPath         string `json:"harness_binary_path"`
	HarnessBinarySHA256       string `json:"harness_binary_sha256"`
	QualificationRoot         string `json:"qualification_root"`
	MachineUUID               string `json:"machine_uuid"`
	DiskSerial                string `json:"disk_serial"`
	RunID                     string `json:"run_id"`
	Scenario                  string `json:"scenario"`
	DurabilityBoundary        string `json:"durability_boundary"`
	BootstrapNonce            string `json:"bootstrap_nonce"`
	BootIDSource              string `json:"boot_id_source"`
}

type Service struct {
	ControllerPublic ed25519.PublicKey
	GuestPrivate     ed25519.PrivateKey
	Expected         protocol.Context
	AgentSHA256      string
	ControllerSHA256 string
	BootstrapNonce   string
	BootIDSource     string
	BootstrapPayload protocol.IdentityBootstrapPayload
	Harness          HarnessAdapter
	Observability    io.Writer
	Replay           *protocol.ReplayGuard
	Now              func() time.Time
}

type HarnessRequest struct {
	MachineUUID       string
	DiskSerial        string
	BootID            string
	RunID             string
	CohortID          string
	Scenario          string
	Boundary          string
	TrialID           string
	Attempt           int
	TicketNonce       string
	CheckpointBootID  string
	HarnessSHA256     string
	observeCheckpoint func(stage, ticketSHA256, evidenceSHA256 string) error
}

type HarnessAdapter interface {
	Checkpoint(HarnessRequest) (any, error)
	Recovery(HarnessRequest) (any, error)
}

func LoadService(configPath, executablePath, bootIDPath string) (*Service, error) {
	configJSON, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config Config
	dec := json.NewDecoder(bytes.NewReader(configJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode guest-agent config: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("guest-agent config contains trailing JSON")
	}
	if config.Schema != ConfigSchema || !shaPattern.MatchString(config.ControllerPublicKeySHA256) || !shaPattern.MatchString(config.GuestPublicKeySHA256) || !shaPattern.MatchString(config.ControllerBinarySHA256) || !shaPattern.MatchString(config.GuestAgentBinarySHA256) || !shaPattern.MatchString(config.HarnessBinarySHA256) || !shaPattern.MatchString(config.BootstrapNonce) {
		return nil, fmt.Errorf("guest-agent config schema or hash pins are invalid")
	}
	if config.BootIDSource != bootIDPath || config.BootIDSource != manifest.KernelBootIDSource {
		return nil, fmt.Errorf("guest-agent boot identity source is not the reviewed kernel path")
	}
	if !filepath.IsAbs(config.ControllerPublicKeyPath) || !filepath.IsAbs(config.GuestPrivateKeyPath) || !filepath.IsAbs(config.HarnessBinaryPath) || !filepath.IsAbs(config.QualificationRoot) || !filepath.IsAbs(executablePath) || !filepath.IsAbs(bootIDPath) {
		return nil, fmt.Errorf("guest-agent key, binary, and boot identity paths must be absolute")
	}
	controllerPublic, err := os.ReadFile(config.ControllerPublicKeyPath)
	if err != nil {
		return nil, err
	}
	guestPrivate, err := os.ReadFile(config.GuestPrivateKeyPath)
	if err != nil {
		return nil, err
	}
	if len(controllerPublic) != ed25519.PublicKeySize || len(guestPrivate) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("guest-agent Ed25519 key material is invalid")
	}
	if hash(controllerPublic) != config.ControllerPublicKeySHA256 || hash(guestPrivate[32:]) != config.GuestPublicKeySHA256 {
		return nil, fmt.Errorf("guest-agent mutual public-key pin mismatch")
	}
	executable, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, err
	}
	if hash(executable) != config.GuestAgentBinarySHA256 {
		return nil, fmt.Errorf("guest-agent binary hash pin mismatch")
	}
	bootID, err := os.ReadFile(bootIDPath)
	if err != nil {
		return nil, err
	}
	expected := protocol.Context{MachineUUID: config.MachineUUID, DiskSerial: config.DiskSerial, BootID: strings.TrimSpace(string(bootID)), RunID: config.RunID, Scenario: config.Scenario, DurabilityBoundary: config.DurabilityBoundary}
	bootstrapPayload := protocol.IdentityBootstrapPayload{
		BootIDSource: config.BootIDSource, ControllerPublicKeySHA256: config.ControllerPublicKeySHA256,
		GuestPublicKeySHA256: config.GuestPublicKeySHA256, ControllerBinarySHA256: config.ControllerBinarySHA256,
		GuestAgentBinarySHA256: config.GuestAgentBinarySHA256,
	}
	harness, err := NewLinuxHarnessAdapter(config.HarnessBinaryPath, config.HarnessBinarySHA256, config.QualificationRoot)
	if err != nil {
		return nil, err
	}
	service := &Service{ControllerPublic: controllerPublic, GuestPrivate: guestPrivate, Expected: expected, AgentSHA256: config.GuestAgentBinarySHA256, ControllerSHA256: config.ControllerBinarySHA256, BootstrapNonce: config.BootstrapNonce, BootIDSource: config.BootIDSource, BootstrapPayload: bootstrapPayload, Harness: harness, Observability: os.Stderr, Now: time.Now}
	service.Replay = protocol.NewReplayGuardAfterBootstrap(expected, config.BootstrapNonce)
	return service, nil
}

func (s *Service) Serve(rw io.ReadWriter) error {
	if rw == nil {
		return fmt.Errorf("virtio-serial stream is required")
	}
	bootstrap, err := s.IdentityBootstrap()
	if err != nil {
		return err
	}
	if err := protocol.WriteFramed(rw, bootstrap); err != nil {
		return err
	}
	for {
		request, err := protocol.ReadFramed(rw)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
		response, err := s.Handle(request)
		if err != nil {
			return err
		}
		if err := protocol.WriteFramed(rw, response); err != nil {
			return err
		}
	}
}

func (s *Service) IdentityBootstrap() ([]byte, error) {
	if err := s.requirePinned(); err != nil {
		return nil, err
	}
	ctx := s.Expected
	ctx.Sequence = protocol.BootstrapSequence
	ctx.Nonce = s.BootstrapNonce
	ctx.Phase = protocol.BootstrapPhase
	now := s.Now()
	return protocol.Sign(protocol.BootstrapKind, "identity/v1", ctx, s.BootstrapPayload, now, now.Add(time.Minute), s.GuestPrivate)
}

func (s *Service) Handle(request []byte) ([]byte, error) {
	if err := s.requirePinned(); err != nil {
		return nil, err
	}
	now := s.Now()
	frame, err := protocol.Verify(request, s.ControllerPublic, now)
	if err != nil {
		return nil, err
	}
	if frame.Kind != protocol.RequestKind {
		return nil, fmt.Errorf("guest-agent accepts only signed request frames")
	}
	if err := s.Replay.Accept(frame); err != nil {
		return nil, err
	}
	payload, err := s.handleCapability(frame)
	if err != nil {
		return nil, err
	}
	return protocol.Sign(protocol.ResultKind, frame.Capability, frame.Context, payload, now, now.Add(time.Minute), s.GuestPrivate)
}

func (s *Service) requirePinned() error {
	if len(s.ControllerPublic) != ed25519.PublicKeySize || len(s.GuestPrivate) != ed25519.PrivateKeySize || s.Replay == nil || s.Now == nil || !shaPattern.MatchString(s.AgentSHA256) || !shaPattern.MatchString(s.ControllerSHA256) || !shaPattern.MatchString(s.BootstrapNonce) || s.BootIDSource != manifest.KernelBootIDSource || s.BootstrapPayload.BootIDSource != s.BootIDSource || hash(s.ControllerPublic) != s.BootstrapPayload.ControllerPublicKeySHA256 || hash(s.GuestPrivate[32:]) != s.BootstrapPayload.GuestPublicKeySHA256 || s.ControllerSHA256 != s.BootstrapPayload.ControllerBinarySHA256 || s.AgentSHA256 != s.BootstrapPayload.GuestAgentBinarySHA256 {
		return fmt.Errorf("guest-agent service is not fully pinned")
	}
	return nil
}

func (s *Service) handleCapability(frame protocol.SignedFrame) (any, error) {
	switch frame.Capability {
	case "identity/v1":
		if err := requireEmptyObject(frame.Payload); err != nil {
			return nil, err
		}
		return map[string]any{"machine_uuid": frame.Context.MachineUUID, "disk_serial": frame.Context.DiskSerial, "boot_id": frame.Context.BootID}, nil
	case "health/v1":
		if err := requireEmptyObject(frame.Payload); err != nil {
			return nil, err
		}
		return map[string]any{"healthy": true}, nil
	case "launch-hash-pinned/v1":
		var payload struct {
			ControllerBinarySHA256 string `json:"controller_binary_sha256"`
			GuestAgentBinarySHA256 string `json:"guest_agent_binary_sha256"`
		}
		if err := decodePayload(frame.Payload, &payload); err != nil || payload.ControllerBinarySHA256 != s.ControllerSHA256 || payload.GuestAgentBinarySHA256 != s.AgentSHA256 {
			return nil, fmt.Errorf("launch binary hash pin rejected")
		}
		return map[string]any{"controller_binary_sha256": s.ControllerSHA256, "guest_agent_binary_sha256": s.AgentSHA256, "matched": true}, nil
	case "checkpoint/v1":
		var payload struct {
			CohortID      string `json:"cohort_id"`
			TrialID       string `json:"trial_id"`
			Attempt       int    `json:"attempt"`
			Boundary      string `json:"boundary"`
			TicketNonce   string `json:"ticket_nonce"`
			HarnessSHA256 string `json:"harness_sha256"`
		}
		if err := decodePayload(frame.Payload, &payload); err != nil || !shaPattern.MatchString(payload.TicketNonce) || !shaPattern.MatchString(payload.HarnessSHA256) {
			return nil, fmt.Errorf("checkpoint payload rejected")
		}
		if s.Harness == nil {
			return nil, fmt.Errorf("checkpoint harness ownership is not authorized in the Gate 2 foundation")
		}
		request := HarnessRequest{MachineUUID: frame.Context.MachineUUID, DiskSerial: frame.Context.DiskSerial, BootID: frame.Context.BootID, RunID: frame.Context.RunID, CohortID: payload.CohortID, Scenario: frame.Context.Scenario, Boundary: payload.Boundary, TrialID: payload.TrialID, Attempt: payload.Attempt, TicketNonce: payload.TicketNonce, HarnessSHA256: payload.HarnessSHA256}
		request.observeCheckpoint = func(stage, ticketSHA256, evidenceSHA256 string) error {
			return s.writeCheckpointObservation(request, stage, ticketSHA256, evidenceSHA256)
		}
		if err := request.observeCheckpoint(checkpointStageRequestReceived, "", ""); err != nil {
			return nil, err
		}
		return s.Harness.Checkpoint(request)
	case "recovery/v1":
		var payload struct {
			CohortID         string `json:"cohort_id"`
			TrialID          string `json:"trial_id"`
			Attempt          int    `json:"attempt"`
			Boundary         string `json:"boundary"`
			TicketNonce      string `json:"ticket_nonce"`
			CheckpointBootID string `json:"checkpoint_boot_id"`
			HarnessSHA256    string `json:"harness_sha256"`
		}
		if err := decodePayload(frame.Payload, &payload); err != nil || !shaPattern.MatchString(payload.TicketNonce) || !shaPattern.MatchString(payload.HarnessSHA256) {
			return nil, fmt.Errorf("recovery payload rejected")
		}
		if s.Harness == nil {
			return nil, fmt.Errorf("recovery harness ownership is not authorized in the Gate 2 foundation")
		}
		return s.Harness.Recovery(HarnessRequest{MachineUUID: frame.Context.MachineUUID, DiskSerial: frame.Context.DiskSerial, BootID: frame.Context.BootID, RunID: frame.Context.RunID, CohortID: payload.CohortID, Scenario: frame.Context.Scenario, Boundary: payload.Boundary, TrialID: payload.TrialID, Attempt: payload.Attempt, TicketNonce: payload.TicketNonce, CheckpointBootID: payload.CheckpointBootID, HarnessSHA256: payload.HarnessSHA256})
	default:
		return nil, fmt.Errorf("capability is not reviewed")
	}
}

func requireEmptyObject(data []byte) error {
	var payload struct{}
	return decodePayload(data, &payload)
}

func decodePayload(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("payload contains trailing JSON")
	}
	return nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

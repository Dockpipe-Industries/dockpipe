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

	"dockpipe.vm/tools/internal/protocol"
)

const ConfigSchema = "dockpipe.vm.guest-agent-config.v1"

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Config struct {
	Schema                    string `json:"schema"`
	ControllerPublicKeyPath   string `json:"controller_public_key_path"`
	ControllerPublicKeySHA256 string `json:"controller_public_key_sha256"`
	GuestPrivateKeyPath       string `json:"guest_private_key_path"`
	GuestPublicKeySHA256      string `json:"guest_public_key_sha256"`
	ControllerBinarySHA256    string `json:"controller_binary_sha256"`
	GuestAgentBinarySHA256    string `json:"guest_agent_binary_sha256"`
	MachineUUID               string `json:"machine_uuid"`
	DiskSerial                string `json:"disk_serial"`
	RunID                     string `json:"run_id"`
	Scenario                  string `json:"scenario"`
	DurabilityBoundary        string `json:"durability_boundary"`
}

type Service struct {
	ControllerPublic ed25519.PublicKey
	GuestPrivate     ed25519.PrivateKey
	Expected         protocol.Context
	AgentSHA256      string
	ControllerSHA256 string
	Replay           *protocol.ReplayGuard
	Now              func() time.Time
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
	if config.Schema != ConfigSchema || !shaPattern.MatchString(config.ControllerPublicKeySHA256) || !shaPattern.MatchString(config.GuestPublicKeySHA256) || !shaPattern.MatchString(config.ControllerBinarySHA256) || !shaPattern.MatchString(config.GuestAgentBinarySHA256) {
		return nil, fmt.Errorf("guest-agent config schema or hash pins are invalid")
	}
	if !filepath.IsAbs(config.ControllerPublicKeyPath) || !filepath.IsAbs(config.GuestPrivateKeyPath) || !filepath.IsAbs(executablePath) || !filepath.IsAbs(bootIDPath) {
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
	service := &Service{ControllerPublic: controllerPublic, GuestPrivate: guestPrivate, Expected: expected, AgentSHA256: config.GuestAgentBinarySHA256, ControllerSHA256: config.ControllerBinarySHA256, Now: time.Now}
	service.Replay = protocol.NewReplayGuard(expected)
	return service, nil
}

func (s *Service) Serve(rw io.ReadWriter) error {
	if rw == nil {
		return fmt.Errorf("virtio-serial stream is required")
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

func (s *Service) Handle(request []byte) ([]byte, error) {
	if len(s.ControllerPublic) != ed25519.PublicKeySize || len(s.GuestPrivate) != ed25519.PrivateKeySize || s.Replay == nil || s.Now == nil || !shaPattern.MatchString(s.AgentSHA256) || !shaPattern.MatchString(s.ControllerSHA256) {
		return nil, fmt.Errorf("guest-agent service is not fully pinned")
	}
	now := s.Now()
	frame, err := protocol.Verify(request, s.ControllerPublic, now)
	if err != nil {
		return nil, err
	}
	if frame.Kind != "request" {
		return nil, fmt.Errorf("guest-agent accepts only signed request frames")
	}
	if err := s.Replay.Accept(frame); err != nil {
		return nil, err
	}
	payload, err := s.handleCapability(frame)
	if err != nil {
		return nil, err
	}
	return protocol.Sign("result", frame.Capability, frame.Context, payload, now, now.Add(time.Minute), s.GuestPrivate)
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
			CheckpointSHA256 string `json:"checkpoint_sha256"`
		}
		if err := decodePayload(frame.Payload, &payload); err != nil || !shaPattern.MatchString(payload.CheckpointSHA256) {
			return nil, fmt.Errorf("checkpoint payload rejected")
		}
		return nil, fmt.Errorf("checkpoint harness ownership is not authorized in the Gate 2 foundation")
	case "recovery/v1":
		var payload struct {
			TicketSHA256 string `json:"ticket_sha256"`
		}
		if err := decodePayload(frame.Payload, &payload); err != nil || !shaPattern.MatchString(payload.TicketSHA256) {
			return nil, fmt.Errorf("recovery payload rejected")
		}
		return nil, fmt.Errorf("recovery harness ownership is not authorized in the Gate 2 foundation")
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

package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const EdgeActivationSchema = "dockpipe.remote.edge-activation/v1"

type EdgeActivationRequest struct {
	Inputs               map[string]string `json:"inputs"`
	Schema               string            `json:"schema"`
	ExpectedConfigDigest string            `json:"expected_config_digest"`
	Candidate            EdgeConfig        `json:"candidate"`
	CandidateSHA256      string            `json:"candidate_sha256"`
	Recovery             EdgeConfig        `json:"recovery"`
	RecoverySHA256       string            `json:"recovery_sha256"`
	ReadinessURL         string            `json:"readiness_url"`
	StartupSeconds       int               `json:"startup_seconds"`
	StableSeconds        int               `json:"stable_seconds"`
	RecoverySeconds      int               `json:"recovery_seconds"`
	Maintenance          string            `json:"maintenance"`
}

func ValidEdgeDigest(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == 32 && hex.EncodeToString(data) == value
}

func EdgeConfigDigest(config EdgeConfig) string {
	data, _ := json.Marshal(config)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func (request EdgeActivationRequest) ID() string {
	data, _ := json.Marshal(request)
	var canonical map[string]any
	_ = json.Unmarshal(data, &canonical)
	data, _ = json.Marshal(canonical)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func (request EdgeActivationRequest) Validate() error {
	if request.Schema != EdgeActivationSchema || !ValidEdgeDigest(request.ExpectedConfigDigest) ||
		!ValidEdgeDigest(request.CandidateSHA256) || !ValidEdgeDigest(request.RecoverySHA256) ||
		request.Maintenance != "allow-connector-replacement" {
		return errors.New("activation requires exact digests and explicit connector maintenance approval")
	}
	for _, config := range []EdgeConfig{request.Candidate, request.Recovery} {
		if config.Schema != Version || !filepath.IsAbs(config.Executable) || len(config.Executable) > 4096 ||
			len(config.Arguments) < 1 || len(config.Arguments) > 64 || Endpoint(config.Endpoint) != nil || !strings.HasPrefix(config.Endpoint, "https://") {
			return errors.New("invalid activation process configuration")
		}
		for _, argument := range config.Arguments {
			if len(argument) > 4096 || strings.ContainsAny(argument, "\x00\r\n") {
				return errors.New("invalid activation argument")
			}
		}
	}
	if request.Candidate.Endpoint != request.Recovery.Endpoint {
		return errors.New("recovery must preserve the published endpoint")
	}
	address, err := url.Parse(request.ReadinessURL)
	if err != nil || address.Scheme != "http" || address.Hostname() != "127.0.0.1" || address.Port() == "" ||
		address.User != nil || address.RawQuery != "" || address.Fragment != "" || (address.Path == "" || !strings.HasPrefix(address.Path, "/") || len(request.ReadinessURL) > 4096) {
		return errors.New("readiness must be a provider-declared HTTP endpoint on an explicit IPv4 loopback port")
	}
	port, portError := strconv.Atoi(address.Port())
	if portError != nil || port < 1 || port > 65535 {
		return errors.New("invalid readiness port")
	}
	if len(request.Inputs) > 4 {
		return errors.New("too many activation input digests")
	}
	for path, digest := range request.Inputs {
		if !filepath.IsAbs(path) || !ValidEdgeDigest(digest) {
			return errors.New("invalid activation input constraint")
		}
	}
	if _, err := net.ResolveTCPAddr("tcp4", address.Host); err != nil {
		return errors.New("invalid readiness address")
	}
	if request.StartupSeconds < 2 || request.StartupSeconds > 30 || request.StableSeconds < 2 || request.StableSeconds > 20 ||
		request.RecoverySeconds < 2*request.StartupSeconds+request.StableSeconds+10 || request.RecoverySeconds > 900 {
		return errors.New("activation/recovery timing exceeds bounded policy")
	}
	return nil
}

type EdgeActivationStatus struct {
	Schema        string    `json:"schema"`
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	Message       string    `json:"message"`
	ActiveSHA256  string    `json:"active_sha256,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
	RecoveryUntil time.Time `json:"recovery_until"`
}

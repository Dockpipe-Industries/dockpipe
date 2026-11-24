package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type desktopAdministratorAdmission struct {
	consent string
	serial  uint64
	until   time.Time
	viewer  string
	service string
}

const administratorServiceConfigName = "desktop-administrator-services.json"

type desktopAdministratorServiceConfig struct {
	Schema   string                         `json:"schema"`
	Services []desktopAdministratorKeyEntry `json:"services"`
}

type desktopAdministratorKeyEntry struct {
	Node           string `json:"node"`
	Fingerprint    string `json:"fingerprint"`
	PublicKey      string `json:"public_key"`
	RelayTokenHash string `json:"relay_token_hash,omitempty"`
}

// loadDesktopAdministratorServices keeps elevated input disabled unless
// the broker owner provisioned independently approved service identities.
func (broker *Broker) loadDesktopAdministratorServices(root string) error {
	var config desktopAdministratorServiceConfig
	err := readPrivateBounded(filepath.Join(root, administratorServiceConfigName), &config, 32<<10)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if config.Schema != "dockpipe.remote.desktop-admin-services/v1" || len(config.Services) > contract.MaxDesktopSessions {
		return errors.New("invalid desktop administrator services configuration")
	}
	entries := make(map[string]desktopAdministratorService, len(config.Services))
	usedTokens := map[string]bool{broker.state.AdminHash: true}
	for _, enrollment := range broker.state.Nodes {
		usedTokens[enrollment.TokenHash] = true
	}
	for _, entry := range config.Services {
		fingerprint := remoteidentity.Fingerprint(entry.PublicKey)
		if !contract.ValidID(entry.Node) || !validMachineNonce(entry.Fingerprint) ||
			fingerprint == "" || fingerprint == entry.Fingerprint ||
			entry.RelayTokenHash != "" && (!validMachineNonce(entry.RelayTokenHash) || usedTokens[entry.RelayTokenHash]) {
			return errors.New("invalid desktop administrator service identity")
		}
		if _, duplicate := entries[entry.Node]; duplicate {
			return errors.New("duplicate desktop administrator service target")
		}
		usedTokens[entry.RelayTokenHash] = true
		entries[entry.Node] = desktopAdministratorService{fingerprint: entry.Fingerprint, key: entry.PublicKey, tokenHash: entry.RelayTokenHash}
	}
	broker.desktopState().administratorKeys = entries
	return nil
}

// SetDesktopAdministratorService configures an independently approved service
// key for an exact paired target. Service keys must come from trusted broker
// configuration, never from a worker advertisement or an operator request.
func (broker *Broker) SetDesktopAdministratorService(target contract.FeatureTarget, publicKey string) error {
	fingerprint := remoteidentity.Fingerprint(publicKey)
	if target.Validate() != nil || fingerprint == "" || fingerprint == target.Fingerprint {
		return errors.New("administrator input requires a distinct approved service identity")
	}

	broker.mutex.Lock()
	defer broker.mutex.Unlock()

	enrollment, ok := broker.state.Nodes[target.Node]
	if !ok || enrollment.Revoked || remoteidentity.Fingerprint(enrollment.MachineKey) != target.Fingerprint {
		return errors.New("administrator input service target is not paired")
	}
	state := broker.desktopState()
	if state.administratorKeys == nil {
		state.administratorKeys = map[string]desktopAdministratorService{}
	}
	if existing, ok := state.administratorKeys[target.Node]; ok && existing.fingerprint == target.Fingerprint && existing.key == publicKey {
		return nil
	}
	state.administratorKeys[target.Node] = desktopAdministratorService{fingerprint: target.Fingerprint, key: publicKey}
	for _, record := range state.records {
		if record.open.Target.Node == target.Node && record.administrator != nil {
			record.administrator.until = time.Time{}
		}
	}
	return nil
}

type desktopAdministratorService struct {
	fingerprint string
	key         string
	tokenHash   string
}

func (broker *Broker) grantDesktopAdministratorInput(raw []byte, admission operatorAdmission) (any, error) {
	state := broker.desktopState()
	now := time.Now()
	var envelope contract.DesktopAdministratorEnvelope
	var consent contract.DesktopAdministratorConsent
	if len(raw) > 4096 || Decode(raw, &envelope) != nil || Decode(envelope.Payload, &consent) != nil ||
		consent.Schema != contract.DesktopAdministratorConsentVersion || consent.Reference.Validate() != nil ||
		!validMachineNonce(consent.Operator) || !validMachineNonce(consent.Challenge) || !validMachineNonce(consent.PeerFingerprint) ||
		!validMachineNonce(consent.ServiceFingerprint) || consent.ServiceFingerprint == consent.PeerFingerprint ||
		len(consent.WindowsUserSID) > 184 || !strings.HasPrefix(consent.WindowsUserSID, "S-1-") ||
		consent.WindowsSession == 0 || consent.WindowsSession == math.MaxUint32 ||
		!time.UnixMilli(consent.Deadline).After(now) {
		return nil, errors.New("invalid administrator input consent")
	}

	approved, ok := state.administratorKeys[consent.Reference.Target.Node]
	if !ok || approved.fingerprint != consent.Reference.Target.Fingerprint ||
		!remoteidentity.Verify(approved.key, administratorSignedMessage(contract.DesktopAdministratorConsentVersion, envelope.Payload), envelope.Signature) {
		return nil, errors.New("administrator input consent must come from the approved service")
	}

	record := state.records[consent.Reference.ID]
	operator := Hash(fmt.Sprintf("%s:%d", admission.session, admission.generation))
	if consent.Operator != operator || record == nil || record.open.Reference() != consent.Reference || record.open.Mode != "control" ||
		record.state != "running" || record.viewer.fingerprint != consent.PeerFingerprint ||
		!desktopOwner(record, admission) || time.UnixMilli(consent.Deadline).After(record.open.Deadline) ||
		broker.desktopLease(record) == 0 || broker.security.CheckAdmission(admission) != nil {
		return nil, errors.New("administrator input requires the current operator and exact running desktop")
	}

	if approved.tokenHash != "" {
		approval := record.administratorApproval
		if approval == nil || approval.Consent == nil || approval.Denied ||
			Hash(string(approval.Consent.Payload)) != Hash(string(envelope.Payload)) {
			return nil, errors.New("administrator input requires a resolved local approval")
		}
	}

	consentDigest := Hash(string(envelope.Payload))
	previous := record.administrator
	if previous != nil && (previous.consent != consentDigest || !previous.until.After(now)) {
		return nil, errors.New("administrator input consent was changed, revoked or expired")
	}

	expires := now.Add(contract.DesktopLease)
	for _, bound := range []time.Time{time.UnixMilli(consent.Deadline), admission.deadline, record.admission.deadline} {
		if bound.Before(expires) {
			expires = bound
		}
	}
	expires = time.UnixMilli(expires.UnixMilli())
	if !expires.After(now) {
		return nil, errors.New("administrator input authority expired")
	}

	serial := state.administratorSerial + 1
	if previous != nil {
		serial = previous.serial
	} else if serial == 0 {
		return nil, errors.New("administrator input grant capacity exhausted")
	}
	grant := contract.DesktopAdministratorGrant{
		Schema: contract.DesktopAdministratorGrantVersion, Consent: envelope.Payload, Serial: serial,
		Operator:   operator,
		Generation: admission.generation, LeaseExpires: expires.UnixMilli(),
	}
	payload, err := json.Marshal(grant)
	if err != nil {
		return nil, err
	}
	signature, err := broker.state.Identity.Sign(administratorSignedMessage(contract.DesktopAdministratorGrantVersion, payload))
	if err != nil {
		return nil, err
	}
	if previous == nil {
		state.administratorSerial = serial
	}
	record.administrator = &desktopAdministratorAdmission{
		consent: consentDigest, serial: serial, until: expires,
		viewer: consent.PeerFingerprint, service: consent.ServiceFingerprint,
	}
	return contract.DesktopAdministratorEnvelope{Payload: payload, Signature: signature}, nil
}

func administratorSignedMessage(schema string, payload []byte) []byte {
	return append([]byte(schema+"\n"), payload...)
}

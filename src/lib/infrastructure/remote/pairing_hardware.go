package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
	"github.com/go-webauthn/webauthn/webauthn"
)

type hardwarePairingChallenge struct {
	Session webauthn.SessionData
	Pairing PairingSession
}

func pairingIdentityMessage(request contract.PairingRequest) []byte {
	return []byte("dockpipe.pairing-machine.v1\n" + request.Node + "\n" + request.Token + "\n" + request.MachineKey + "\n" + request.Nonce)
}

// SignPairingRequest proves ownership of the persistent worker identity before
// asking a human to approve it. The worker saves its private key before sending.
func SignPairingRequest(request *contract.PairingRequest, identity remoteidentity.Identity) error {
	nonce, err := Secret()
	if err != nil {
		return err
	}
	request.Nonce = nonce
	request.MachineKey = identity.PublicKey
	signature, err := identity.Sign(pairingIdentityMessage(*request))
	request.MachineProof = signature
	return err
}

func pairingStatusMessage(status contract.PairingStatus, workerKey, tokenHash string) []byte {
	status.BrokerProof = ""
	data, _ := json.Marshal(status)
	return append([]byte("dockpipe.pairing-broker.v1\n"+workerKey+"\n"+tokenHash+"\n"), data...)
}

func (b *Broker) signedPairingStatus(status contract.PairingStatus, workerKey, tokenHash, requestNonce string) (any, error) {
	status.RequestNonce = requestNonce
	status.BrokerKey = b.state.Identity.PublicKey
	signature, err := b.state.Identity.Sign(pairingStatusMessage(status, workerKey, tokenHash))
	status.BrokerProof = signature
	return status, err
}

func VerifyPairingStatus(status contract.PairingStatus, workerKey, token, requestNonce, pinnedBroker string) error {
	if len(requestNonce) != 64 || status.RequestNonce != requestNonce {
		return errors.New("broker pairing response is stale")
	}
	if pinnedBroker != "" && status.BrokerKey != pinnedBroker {
		return errors.New("broker machine identity changed; inspect before pairing")
	}
	if !remoteidentity.Verify(status.BrokerKey, pairingStatusMessage(status, workerKey, Hash(token)), status.BrokerProof) {
		return errors.New("broker machine identity proof is invalid")
	}
	return nil
}

func (b *Broker) pendingPairing(code string) (PairingSession, error) {
	session, exists := b.state.Pairings[code]
	if !exists || session.Status != "pending" || !time.Now().Before(session.ExpiresAt) ||
		!time.Now().Before(b.state.PairingUntil) || remoteidentity.Fingerprint(session.MachineKey) == "" {
		return session, errors.New("pairing is not pending with a verified machine identity or has expired")
	}
	if node, exists := b.state.Nodes[session.Node]; exists && !node.Revoked && node.TokenHash != "" {
		return session, errors.New("node name already enrolled")
	}
	if _, exists := b.state.Nodes[session.Node]; !exists && len(b.state.Nodes) >= 64 {
		return session, errors.New("node capacity reached")
	}
	return session, nil
}

func (b *Broker) beginHardwarePairing(raw []byte) (any, error) {
	if b.state.PairingKeys == nil {
		return nil, errors.New("register a hardware security key locally before approving pairing")
	}
	var request contract.PairingChallengeRequest
	if err := Decode(raw, &request); err != nil || !validMachineNonce(request.Nonce) {
		return nil, errors.New("pairing approval requires a fresh operator nonce")
	}
	code := strings.ToUpper(strings.TrimSpace(request.Code))
	pairing, err := b.pendingPairing(code)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for id, challenge := range b.pairingChallenges {
		if !now.Before(challenge.Session.Expires) || challenge.Pairing.Code == code {
			delete(b.pairingChallenges, id)
		}
	}
	if len(b.pairingChallenges) >= 32 {
		return nil, errors.New("hardware pairing challenge capacity reached")
	}
	options, session, err := b.state.PairingKeys.BeginAssertion()
	if err != nil {
		return nil, err
	}
	if pairing.ExpiresAt.Before(session.Expires) {
		session.Expires = pairing.ExpiresAt
	}
	id, err := Secret()
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	if b.pairingChallenges == nil {
		b.pairingChallenges = map[string]hardwarePairingChallenge{}
	}
	b.pairingChallenges[id] = hardwarePairingChallenge{Session: *session, Pairing: pairing}
	challenge := contract.PairingChallenge{ID: id, Code: code, Node: pairing.Node,
		RequestNonce: request.Nonce, BrokerKey: b.state.Identity.PublicKey,
		WorkerFingerprint: remoteidentity.Fingerprint(pairing.MachineKey),
		BrokerFingerprint: remoteidentity.Fingerprint(b.state.Identity.PublicKey),
		ExpiresAt:         session.Expires, Options: encoded}
	message, err := pairingChallengeMessage(challenge)
	if err != nil {
		return nil, err
	}
	challenge.Signature, err = b.state.Identity.Sign(message)
	if err != nil {
		return nil, err
	}
	return challenge, nil
}

func (b *Broker) finishHardwarePairing(raw []byte) (any, error) {
	var request contract.PairingApproval
	if err := Decode(raw, &request); err != nil {
		return nil, err
	}
	challenge, exists := b.pairingChallenges[request.ChallengeID]
	// Every finish attempt consumes its nonce, including invalid signatures.
	delete(b.pairingChallenges, request.ChallengeID)
	if !exists || b.state.PairingKeys == nil || request.Code != challenge.Pairing.Code || !time.Now().Before(challenge.Session.Expires) {
		return nil, errors.New("fresh hardware-key verification is required for this pairing")
	}
	pairing, err := b.pendingPairing(request.Code)
	if err != nil {
		return nil, err
	}
	previous, _ := json.Marshal(challenge.Pairing)
	current, _ := json.Marshal(pairing)
	if !bytes.Equal(previous, current) {
		return nil, errors.New("pairing changed after hardware-key challenge")
	}
	keys, err := b.state.PairingKeys.FinishAssertion(challenge.Session, request.Response)
	if err != nil {
		return nil, err
	}
	b.state.PairingKeys = keys
	b.state.Nodes[pairing.Node] = Enrollment{TokenHash: pairing.TokenHash, MachineKey: pairing.MachineKey}
	pairing.Status = "approved"
	b.state.Pairings[pairing.Code] = pairing
	if err := b.save(); err != nil {
		return nil, err
	}
	return b.signedPairingStatus(pairing.PairingStatus, pairing.MachineKey, pairing.TokenHash, "")
}

func pairingChallengeMessage(challenge contract.PairingChallenge) ([]byte, error) {
	challenge.Signature = ""
	data, err := json.Marshal(challenge)
	if err != nil {
		return nil, err
	}
	return append([]byte("dockpipe.pairing-consent.v1\n"), data...), nil
}

// VerifyPairingChallenge authenticates both the browser's consent details and its
// WebAuthn options against a broker key read from protected local owner state.
func VerifyPairingChallenge(challenge contract.PairingChallenge, brokerKey, code, requestNonce string) error {
	if !validMachineNonce(requestNonce) || challenge.RequestNonce != requestNonce || challenge.Code != code ||
		challenge.BrokerKey != brokerKey || remoteidentity.Fingerprint(brokerKey) == "" ||
		challenge.BrokerFingerprint != remoteidentity.Fingerprint(brokerKey) || !time.Now().Before(challenge.ExpiresAt) {
		return errors.New("pairing consent identity, request or expiry does not match")
	}
	message, err := pairingChallengeMessage(challenge)
	if err != nil || !remoteidentity.Verify(brokerKey, message, challenge.Signature) {
		return errors.New("pairing consent signature verification failed")
	}
	return nil
}

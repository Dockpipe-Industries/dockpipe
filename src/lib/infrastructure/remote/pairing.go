package remote

import (
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type PairingSession struct {
	contract.PairingStatus
	TokenHash string `json:"token_hash"`
}

// Public requests are accepted only during an operator-opened window. All limits
// are global: forwarded client IP headers are intentionally not trusted.
func (b *Broker) requestPairing(raw []byte) (any, error) {
	var request contract.PairingRequest
	if err := Decode(raw, &request); err != nil || !contract.ValidID(request.Node) {
		return nil, errors.New("invalid pairing request")
	}
	decoded, err := hex.DecodeString(request.Token)
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("invalid pairing credential")
	}
	now := time.Now().UTC()
	hash := Hash(request.Token)
	for name, node := range b.state.Nodes {
		if name != request.Node && matches(request.Token, node.TokenHash) {
			return nil, errors.New("credential is already bound to another node")
		}
	}
	if node, exists := b.state.Nodes[request.Node]; exists && matches(request.Token, node.TokenHash) {
		if node.Revoked {
			return contract.PairingStatus{Node: request.Node, Status: "revoked"}, nil
		}
		return contract.PairingStatus{Node: request.Node, Status: "approved"}, nil
	}
	for code, session := range b.state.Pairings {
		if session.TokenHash == hash && session.Node != request.Node {
			return nil, errors.New("pairing credential is already bound to another request")
		}
		if session.TokenHash != hash || session.Node != request.Node {
			continue
		}
		if session.Status == "approved" {
			node := b.state.Nodes[session.Node]
			if node.Revoked || node.TokenHash != hash {
				return nil, errors.New("pairing revoked")
			}
			return session.PairingStatus, nil
		}
		if now.Before(session.ExpiresAt) {
			return session.PairingStatus, nil
		}
		delete(b.state.Pairings, code)
		break
	}
	if !now.Before(b.state.PairingUntil) {
		return nil, errors.New("pairing is closed; open pairing on the broker first")
	}
	if now.Sub(b.pairingRateStart) >= time.Minute {
		b.pairingRateStart = now
		b.pairingRateCount = 0
	}
	if b.pairingRateCount >= 12 {
		return nil, errors.New("pairing request limit reached; retry later")
	}
	b.pairingRateCount++
	for code, session := range b.state.Pairings {
		if !now.Before(session.ExpiresAt) {
			delete(b.state.Pairings, code)
		}
	}
	if len(b.state.Pairings) >= 32 {
		return nil, errors.New("pairing capacity reached")
	}
	if node, exists := b.state.Nodes[request.Node]; exists && !node.Revoked && node.TokenHash != "" {
		return nil, errors.New("node name already enrolled")
	}
	secret, err := Secret()
	if err != nil {
		return nil, err
	}
	code := strings.ToUpper(secret[:4] + "-" + secret[4:8] + "-" + secret[8:12])
	if _, exists := b.state.Pairings[code]; exists {
		return nil, errors.New("pairing code collision; retry")
	}
	status := contract.PairingStatus{Code: code, Node: request.Node, Status: "pending", ExpiresAt: b.state.PairingUntil}
	b.state.Pairings[code] = PairingSession{PairingStatus: status, TokenHash: hash}
	return status, b.save()
}

func (b *Broker) pairingOperator(path string, raw []byte) (any, error) {
	now := time.Now().UTC()
	switch path {
	case "/v1/pairing-open":
		b.state.PairingUntil = now.Add(15 * time.Minute)
		return map[string]time.Time{"expires_at": b.state.PairingUntil}, b.save()
	case "/v1/pairing-close":
		b.state.PairingUntil = time.Time{}
		for code, session := range b.state.Pairings {
			if session.Status == "pending" {
				session.Status = "denied"
				b.state.Pairings[code] = session
			}
		}
		return map[string]string{"status": "closed"}, b.save()
	case "/v1/pairings":
		statuses := []contract.PairingStatus{}
		for _, session := range b.state.Pairings {
			if session.Status == "pending" && now.Before(session.ExpiresAt) {
				statuses = append(statuses, session.PairingStatus)
			}
		}
		sort.Slice(statuses, func(i, j int) bool { return statuses[i].Code < statuses[j].Code })
		return statuses, nil
	case "/v1/approve", "/v1/deny":
		var request struct {
			Code string `json:"code"`
		}
		if err := Decode(raw, &request); err != nil {
			return nil, err
		}
		code := strings.ToUpper(strings.TrimSpace(request.Code))
		session, exists := b.state.Pairings[code]
		if !exists || !now.Before(session.ExpiresAt) || session.Status != "pending" {
			return nil, errors.New("pairing is not pending or has expired")
		}
		session.Status = "denied"
		if path == "/v1/approve" {
			node, exists := b.state.Nodes[session.Node]
			if exists && !node.Revoked && node.TokenHash != "" {
				return nil, errors.New("node name already enrolled")
			}
			if !exists && len(b.state.Nodes) >= 64 {
				return nil, errors.New("node capacity reached")
			}
			// Replace an unused file invitation only upon explicit approval. This
			// also invalidates that invitation so it cannot enroll another token.
			b.state.Nodes[session.Node] = Enrollment{TokenHash: session.TokenHash}
			session.Status = "approved"
		}
		b.state.Pairings[code] = session
		return session.PairingStatus, b.save()
	}
	return nil, errors.New("unknown pairing operation")
}

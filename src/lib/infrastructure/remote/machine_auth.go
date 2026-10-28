package remote

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"dockpipe/src/lib/infrastructure/remoteidentity"
)

const machineNonceHeader = "X-Dockpipe-Machine-Nonce"
const machineSignatureHeader = "X-Dockpipe-Machine-Signature"

type machineChallenge struct {
	Node      string
	ExpiresAt time.Time
}

type machineChallengeRequest struct {
	Nonce string `json:"nonce"`
}

type machineChallengeResponse struct {
	RequestNonce string `json:"request_nonce"`
	Nonce        string `json:"nonce"`
	Signature    string `json:"signature"`
}

func machineMessage(kind, nonce, path string, status int, body []byte) []byte {
	digest := sha256.Sum256(body)
	return []byte("dockpipe.machine." + kind + ".v1\n" + nonce + "\n" + path + "\n" + strconv.Itoa(status) + "\n" + hex.EncodeToString(digest[:]))
}

func (b *Broker) serveMachine(w http.ResponseWriter, r *http.Request, node string, enrollment Enrollment, body []byte) {
	if r.URL.Path == "/v1/machine-challenge" {
		var request machineChallengeRequest
		if err := Decode(body, &request); err != nil || !validMachineNonce(request.Nonce) {
			http.Error(w, "fresh client nonce required", http.StatusBadRequest)
			return
		}
		now := time.Now()
		for nonce, challenge := range b.machineChallenges {
			if !now.Before(challenge.ExpiresAt) {
				delete(b.machineChallenges, nonce)
			}
		}
		nodeNonces := 0
		oldestNonce := ""
		var oldestExpiry time.Time
		for nonce, challenge := range b.machineChallenges {
			if challenge.Node == node {
				nodeNonces++
				if oldestNonce == "" || challenge.ExpiresAt.Before(oldestExpiry) {
					oldestNonce, oldestExpiry = nonce, challenge.ExpiresAt
				}
			}
		}
		if nodeNonces >= 4 {
			delete(b.machineChallenges, oldestNonce)
		}
		if len(b.machineChallenges) >= 64*4 {
			http.Error(w, "machine challenge capacity reached", http.StatusTooManyRequests)
			return
		}
		nonce, err := Secret()
		if err != nil {
			http.Error(w, "machine challenge unavailable", http.StatusServiceUnavailable)
			return
		}
		if b.machineChallenges == nil {
			b.machineChallenges = map[string]machineChallenge{}
		}
		b.machineChallenges[nonce] = machineChallenge{Node: node, ExpiresAt: now.Add(20 * time.Second)}
		signature, err := b.state.Identity.Sign(machineMessage("challenge", nonce, "", 0, []byte(request.Nonce)))
		if err != nil {
			http.Error(w, "machine challenge unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(machineChallengeResponse{RequestNonce: request.Nonce, Nonce: nonce, Signature: signature})
		return
	}
	nonce := r.Header.Get(machineNonceHeader)
	challenge, exists := b.machineChallenges[nonce]
	delete(b.machineChallenges, nonce)
	if !exists || challenge.Node != node || !time.Now().Before(challenge.ExpiresAt) ||
		!remoteidentity.Verify(enrollment.MachineKey, machineMessage("request", nonce, r.URL.Path, 0, body), r.Header.Get(machineSignatureHeader)) {
		http.Error(w, "machine identity proof required", http.StatusUnauthorized)
		return
	}
	value, err := b.worker(node, r.URL.Path, body)
	status := http.StatusOK
	var response []byte
	if err != nil {
		status = http.StatusConflict
		response = []byte("remote operation rejected\n")
	} else {
		response, err = json.Marshal(value)
		if err != nil {
			status = http.StatusInternalServerError
			response = []byte("remote response unavailable\n")
		}
	}
	signature, err := b.state.Identity.Sign(machineMessage("response", nonce, r.URL.Path, status, response))
	if err != nil {
		http.Error(w, "machine identity unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set(machineSignatureHeader, signature)
	w.WriteHeader(status)
	_, _ = w.Write(response)
}

func (c *Client) machineNonce(ctxRequest *http.Request) (string, error) {
	requestNonce, err := Secret()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(machineChallengeRequest{Nonce: requestNonce})
	if err != nil {
		return "", err
	}
	challengeRequest, err := http.NewRequestWithContext(ctxRequest.Context(), http.MethodPost, c.Endpoint+"/v1/machine-challenge", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	challengeRequest.Header.Set("Authorization", "Bearer "+c.Token)
	challengeRequest.Header.Set("Content-Type", "application/json")
	response, err := c.HTTP.Do(challengeRequest)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var challenge machineChallengeResponse
	if response.StatusCode != http.StatusOK || json.NewDecoder(http.MaxBytesReader(nil, response.Body, 2048)).Decode(&challenge) != nil || !validMachineNonce(challenge.Nonce) || challenge.RequestNonce != requestNonce ||
		!remoteidentity.Verify(c.BrokerKey, machineMessage("challenge", challenge.Nonce, "", 0, []byte(requestNonce)), challenge.Signature) {
		return "", errors.New("broker machine identity verification failed")
	}
	return challenge.Nonce, nil
}

func validMachineNonce(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

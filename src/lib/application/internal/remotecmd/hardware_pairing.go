package remotecmd

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/hardwarekey"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
	"github.com/go-webauthn/webauthn/webauthn"
)

func registerPairingKey(ctx context.Context, root, rootsPath string) error {
	if err := requireLocalBroker(root); err != nil {
		return err
	}
	// Enrollment is a local owner operation with the broker stopped. A stolen
	// HTTP administrator token cannot alter hardware trust or enroll another key.
	unlock, err := remoteio.Lock(filepath.Join(root, "broker.lock"))
	if err != nil {
		return fmt.Errorf("stop the broker before registering a pairing key: %w", err)
	}
	defer unlock()
	if _, err := remoteio.NewBroker(root); err != nil {
		return err
	}
	var state remoteio.BrokerState
	if err := remoteio.ReadPrivate(filepath.Join(root, "broker.json"), &state); err != nil {
		return err
	}
	var operator OperatorConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &operator); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(remoteio.Hash(operator.Token)), []byte(state.AdminHash)) != 1 {
		return errors.New("local broker owner credential does not match")
	}
	registry := state.PairingKeys
	authorized := registry == nil
	if registry == nil {
		raw, err := readAttestationRoots(rootsPath)
		if err != nil {
			return err
		}
		registry, err = hardwarekey.NewRegistry(string(raw))
		if err != nil {
			return err
		}
	} else if rootsPath != "" {
		return errors.New("existing manufacturer trust cannot be replaced while adding a key")
	}
	var session *webauthn.SessionData
	var options any
	begin := func() (hardwareCeremonyOptions, error) {
		kind := "get"
		title := "Verify an existing pairing key"
		if options == nil {
			if authorized {
				options, session, err = registry.BeginRegistration()
			} else {
				options, session, err = registry.BeginAssertion()
			}
			if err != nil {
				return hardwareCeremonyOptions{}, err
			}
		}
		if authorized {
			kind = "create"
			title = "Register a physical pairing key"
		}
		return hardwareCeremonyOptions{Title: title, Details: "This key will authorize new machine pairings to this Dockpipe broker.", Kind: kind, Options: options}, nil
	}
	finish := func(response []byte) (bool, error) {
		if session == nil {
			return false, errors.New("hardware-key ceremony has not started")
		}
		current := *session
		session = nil
		options = nil
		if !authorized {
			verified, err := registry.FinishAssertion(current, response)
			if err != nil {
				return false, err
			}
			registry = verified
			authorized = true
			return true, nil
		}
		if err := registry.FinishRegistration(current, response); err != nil {
			return false, err
		}
		state.PairingKeys = registry
		if err := remoteio.WritePrivate(filepath.Join(root, "broker.json"), state); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := runHardwareCeremony(ctx, begin, finish); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Pairing hardware key registered. Start the broker to use it. Register a spare key before relying on this key alone.")
	return nil
}

func readAttestationRoots(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("first registration requires --attestation-roots with verified manufacturer CA certificates")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > hardwarekey.MaxResponseBytes {
		return nil, errors.New("attestation roots must be a bounded regular PEM file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("attestation roots changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, hardwarekey.MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > hardwarekey.MaxResponseBytes {
		return nil, errors.New("attestation roots exceed their bound")
	}
	return raw, nil
}

func approveWithHardwareKey(ctx context.Context, root, code string) error {
	client, brokerKey, err := localPairingApprovalClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	code = strings.ToUpper(strings.TrimSpace(code))
	var challenge *contract.PairingChallenge
	begin := func() (hardwareCeremonyOptions, error) {
		if challenge == nil {
			prepared, err := preparePairingApproval(ctx, client, brokerKey, code)
			if err != nil {
				return hardwareCeremonyOptions{}, err
			}
			challenge = &prepared
		}
		return hardwareCeremonyOptions{Title: "Approve machine pairing", Kind: "get", Options: challenge.Options,
			Details: fmt.Sprintf("Machine: %s\nCode: %s\nWorker fingerprint: %s\nBroker fingerprint: %s\nCompare these with the requesting machine before verifying.", challenge.Node, challenge.Code, challenge.WorkerFingerprint, challenge.BrokerFingerprint)}, nil
	}
	finish := func(response []byte) (bool, error) {
		if challenge == nil {
			return false, errors.New("pairing challenge has not started")
		}
		request := contract.PairingApproval{Code: code, ChallengeID: challenge.ID, Response: json.RawMessage(response)}
		challenge = nil
		var status contract.PairingStatus
		if err := client.Call(ctx, "/v1/approve", request, &status); err != nil {
			return false, err
		}
		return false, json.NewEncoder(os.Stdout).Encode(status)
	}
	return runHardwareCeremony(ctx, begin, finish)
}

func localPairingApprovalClient(root string) (*remoteio.Client, string, error) {
	connection, err := readConnection(root)
	if err != nil {
		return nil, "", err
	}
	if connection != nil && connection.Mode == "hosted" {
		return nil, "", errors.New("hardware-key pairing approval requires the broker's local owner; hosted approval is not supported yet")
	}
	var state remoteio.BrokerState
	if err := remoteio.ReadPrivate(filepath.Join(root, "broker.json"), &state); err != nil {
		return nil, "", err
	}
	if state.Identity == nil || remoteidentity.Fingerprint(state.Identity.PublicKey) == "" {
		return nil, "", errors.New("start the updated local broker before approving pairing")
	}
	client, err := operatorClient(root)
	if err != nil {
		return nil, "", err
	}
	if subtle.ConstantTimeCompare([]byte(remoteio.Hash(client.Token)), []byte(state.AdminHash)) != 1 {
		return nil, "", errors.New("local broker owner credential does not match")
	}
	return client, state.Identity.PublicKey, nil
}

func preparePairingApproval(ctx context.Context, client *remoteio.Client, brokerKey, code string) (contract.PairingChallenge, error) {
	var challenge contract.PairingChallenge
	nonce, err := remoteio.Secret()
	if err != nil {
		return challenge, err
	}
	request := contract.PairingChallengeRequest{Code: code, Nonce: nonce}
	if err := client.Call(ctx, "/v1/approve-begin", request, &challenge); err != nil {
		return challenge, err
	}
	if err := remoteio.VerifyPairingChallenge(challenge, brokerKey, code, nonce); err != nil {
		return contract.PairingChallenge{}, err
	}
	return challenge, nil
}

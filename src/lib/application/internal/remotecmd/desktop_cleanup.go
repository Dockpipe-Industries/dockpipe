package remotecmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

const (
	desktopExitProofBasis       = "provider-process-exited"
	desktopInputResetProofBasis = "native-input-reset-and-provider-exited"
)

type desktopCleanupRequest struct {
	Reference  contract.DesktopReference `json:"reference"`
	Provider   contract.DesktopProvider  `json:"provider"`
	Ack        *uint64                   `json:"ack"`
	ProofBasis string                    `json:"proof_basis,omitempty"`
}

type desktopCleanupDirective struct {
	Type        string                    `json:"type"`
	Reference   contract.DesktopReference `json:"reference"`
	Provider    contract.DesktopProvider  `json:"provider"`
	Mode        string                    `json:"mode"`
	Close       bool                      `json:"close"`
	LeaseMillis int64                     `json:"lease_millis"`
	ProofBasis  string                    `json:"proof_basis"`
	Challenge   string                    `json:"challenge,omitempty"`
}

type desktopExitProof struct {
	Type      string                    `json:"type"`
	Reference contract.DesktopReference `json:"reference"`
	Provider  contract.DesktopProvider  `json:"provider"`
}

type desktopInputResetProof struct {
	Type           string                    `json:"type"`
	Reference      contract.DesktopReference `json:"reference"`
	Provider       contract.DesktopProvider  `json:"provider"`
	Challenge      string                    `json:"challenge"`
	ResetVerified  bool                      `json:"reset_verified"`
	ProviderExited bool                      `json:"provider_exited"`
}

func (request desktopCleanupRequest) cleanupMode() (string, error) {
	switch request.ProofBasis {
	case "", desktopExitProofBasis:
		return "view", nil
	case desktopInputResetProofBasis:
		return "control", nil
	default:
		return "", errors.New("desktop cleanup requires a supported explicit native proof basis")
	}
}

func decodeDesktopCleanupRequest(data []byte) (desktopCleanupRequest, error) {
	var request desktopCleanupRequest
	if len(data) > 4096 || decodeFeatureInput(data, &request) != nil || request.Reference.Validate() != nil ||
		request.Provider.Validate() != nil || request.Ack == nil || *request.Ack > contract.MaxDesktopSignals {
		return request, errors.New("desktop cleanup requires exact original metadata and its authenticated status acknowledgment")
	}
	if _, err := request.cleanupMode(); err != nil {
		return request, err
	}
	return request, nil
}

// The trusted provider owns process-exit evidence. Core never infers cleanup
// from a PID, a vanished window, or a caller choosing to abandon uncertainty.
// Process exit alone is VIEW-only. CONTROL requires a separately selected native
// input-reset and provider-exit proof, bound to a fresh challenge after exact
// authenticated Close0. Neither path asserts a native capture-stop acknowledgment.
func desktopCleanupBridge(parent context.Context, root, path string) error {
	if path == "" || path == "-" {
		return errors.New("desktop-cleanup requires --request <private-file>; stdin carries only native cleanup proof")
	}
	if err := infrastructure.ValidatePrivatePath(path, false); err != nil {
		return err
	}
	data, err := readFeatureInput(parent, path)
	if err != nil {
		return err
	}
	request, err := decodeDesktopCleanupRequest(data)
	if err != nil {
		return err
	}
	unlock, err := remoteio.LockDesktopWorker(root)
	if err != nil {
		return err
	}
	defer unlock()
	permissions, err := remoteio.LoadDesktopPermissions(root)
	if err != nil {
		return err
	}
	var config contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &config); err != nil {
		return err
	}
	if config.Node != request.Reference.Target.Node || remoteidentity.Fingerprint(config.MachineKey) != request.Reference.Target.Fingerprint ||
		permissions.Provider != request.Provider {
		return errors.New("desktop cleanup differs from the current paired worker or frozen owner provider")
	}
	client, err := remoteio.NewWorkerClient(config)
	if err != nil || client.Identity == nil {
		return errors.New("desktop cleanup requires paired worker machine keys")
	}
	defer client.HTTP.CloseIdleConnections()
	return streamDesktopCleanup(parent, root, request, permissions, client, os.Stdin, os.Stdout)
}

func streamDesktopCleanup(parent context.Context, root string, request desktopCleanupRequest, permissions remoteio.DesktopPermissions,
	client *remoteio.Client, input io.ReadCloser, output io.WriteCloser) error {
	mode, err := request.cleanupMode()
	if err != nil {
		return err
	}
	basis := request.ProofBasis
	if basis == "" {
		basis = desktopExitProofBasis
	}
	ctx, stop := context.WithTimeout(parent, featureCommandTimeout)
	defer stop()
	defer input.Close()
	defer output.Close()
	interrupt := context.AfterFunc(ctx, func() { _ = input.Close(); _ = output.Close() })
	defer interrupt()

	// Preserve only the old reference's generation for cleanup. No new provider
	// lifetime, media permission, signaling or lease can be opened on this path.
	catalog := contract.DesktopCatalog{Schema: contract.DesktopVersion, Target: request.Reference.Target, Provider: request.Provider}
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: request.Reference.Epoch, Catalog: catalog}
	response, err := client.ExchangeDesktop(ctx, exchange)
	if err != nil {
		return err
	}
	if len(response.Directives) != 1 || len(response.Forget) != 0 {
		return errors.New("desktop cleanup did not obtain the exact original closing directive; outcome remains unknown")
	}
	directive := response.Directives[0]
	if directive.Open.Reference() != request.Reference || directive.Open.Provider != request.Provider || directive.Open.Mode != mode ||
		!directive.Close || directive.LeaseMillis != 0 || len(directive.Signals) != 0 {
		return errors.New("desktop cleanup requires an exact revoked session matching its selected native proof mode")
	}
	challenge := ""
	if mode == "control" {
		if err := remoteio.CheckDesktopPolicy(root, permissions); err != nil {
			return err
		}
		var nonce [32]byte
		if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
			return errors.New("desktop input-reset cleanup could not generate its native proof challenge")
		}
		challenge = hex.EncodeToString(nonce[:])
	}
	encoder := json.NewEncoder(output)
	if err := encoder.Encode(desktopCleanupDirective{Type: "cleanup", Reference: request.Reference, Provider: request.Provider,
		Mode: mode, Close: true, ProofBasis: basis, Challenge: challenge}); err != nil {
		return err
	}
	proofDeadline := time.AfterFunc(20*time.Second, func() { _ = input.Close() })
	defer proofDeadline.Stop()
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 4097)
	if !scanner.Scan() {
		return errors.New("native provider process-exit proof was not received; desktop cleanup remains unknown")
	}
	if len(scanner.Bytes()) > 4096 {
		return errors.New("native desktop cleanup proof exceeds its metadata bound")
	}
	if mode == "control" {
		var proof desktopInputResetProof
		if decodeFeatureInput(scanner.Bytes(), &proof) != nil || proof.Type != "input-reset-and-provider-exited" ||
			proof.Reference != request.Reference || proof.Provider != request.Provider || proof.Challenge != challenge ||
			!proof.ResetVerified || !proof.ProviderExited {
			return errors.New("native desktop input-reset proof does not match this challenged original control session")
		}
	} else {
		var proof desktopExitProof
		if decodeFeatureInput(scanner.Bytes(), &proof) != nil || proof.Type != "process-exited" ||
			proof.Reference != request.Reference || proof.Provider != request.Provider {
			return errors.New("native desktop process-exit proof changed the original view-only session")
		}
	}
	proofDeadline.Stop()
	if err := remoteio.CheckDesktopPolicy(root, permissions); err != nil {
		return err
	}
	// Directive.Ack acknowledges the opposite direction. Use only the captured
	// authenticated status ACK; the original broker validates its exact range.
	exchange.Reports = []contract.DesktopReport{{Reference: request.Reference, State: "closed", Ack: *request.Ack}}
	response, err = client.ExchangeDesktop(ctx, exchange)
	if err != nil {
		return err
	}
	if len(response.Directives) != 0 || len(response.Forget) != 1 || response.Forget[0] != request.Reference {
		return errors.New("desktop cleanup did not receive exact broker closure proof; outcome remains unknown")
	}
	return encoder.Encode(struct {
		Type       string                    `json:"type"`
		Reference  contract.DesktopReference `json:"reference"`
		ProofBasis string                    `json:"proof_basis"`
	}{Type: "forget", Reference: request.Reference, ProofBasis: basis})
}

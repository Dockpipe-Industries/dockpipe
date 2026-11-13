package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestDesktopCleanupRequiresOriginalViewDirectiveAndNativeProof(t *testing.T) {
	for _, scenario := range []string{"correct", "control", "foreign", "provider", "epoch", "live-lease", "signals", "eof", "proof-foreign", "proof-provider", "policy-change", "forged-ack", "lost-forget"} {
		t.Run(scenario, func(t *testing.T) {
			intent := desktopTestIntent()
			intent.Mode = "view"
			if scenario == "control" {
				intent.Mode = "control"
			}
			intent.ID, _ = intent.Digest()
			ack := uint64(1)
			if scenario == "forged-ack" {
				ack = 63
			}
			request := desktopCleanupRequest{Reference: intent.Reference(), Provider: intent.Provider, Ack: &ack}
			root := privateConnectionDirectory(t)
			permissions := remoteio.DesktopPermissions{Schema: contract.DesktopVersion, Provider: intent.Provider, View: true, Input: true}
			if err := remoteio.SaveDesktopPermissions(root, permissions); err != nil {
				t.Fatal(err)
			}
			var calls, acceptedClosed atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var exchange contract.DesktopExchange
				if err := json.NewDecoder(r.Body).Decode(&exchange); err != nil {
					t.Error(err)
					return
				}
				if exchange.Catalog.Target != intent.Target || exchange.Catalog.Provider != intent.Provider || exchange.Catalog.View || exchange.Catalog.Input {
					t.Error("cleanup changed original generation or renewed desktop capabilities")
				}
				response := contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: intent.Epoch}
				if calls.Add(1) == 1 {
					if len(exchange.Reports) != 0 {
						t.Error("cleanup asserted native proof before obtaining the original directive")
					}
					open := intent
					if scenario == "foreign" {
						open.Target.Generation = strings.Repeat("f", 64)
					}
					if scenario == "provider" {
						open.Provider.Artifact = strings.Repeat("f", 64)
					}
					if scenario == "epoch" {
						response.Epoch = strings.Repeat("f", 64)
					}
					open.ID, _ = open.Digest()
					directive := contract.DesktopDirective{Open: open, Close: true, Operator: strings.Repeat("a", 64)}
					if scenario == "live-lease" {
						directive.LeaseMillis = 1
					}
					if scenario == "signals" {
						directive.Signals = []contract.DesktopSignal{{Reference: intent.Reference(), Sequence: 1, Type: "binding", Fingerprint: strings.Repeat("a", 64)}}
					}
					response.Directives = []contract.DesktopDirective{directive}
					if scenario == "policy-change" {
						changed := permissions
						changed.Input = false
						if err := remoteio.SaveDesktopPermissions(root, changed); err != nil {
							t.Error(err)
						}
					}
				} else {
					if len(exchange.Reports) != 1 || exchange.Reports[0].Reference != intent.Reference() || exchange.Reports[0].State != "closed" || len(exchange.Reports[0].Signals) != 0 {
						t.Error("native proof became another operation or forwarded content")
					}
					if exchange.Reports[0].Ack != 1 {
						http.Error(w, "invalid original acknowledgment", http.StatusBadRequest)
						return
					}
					acceptedClosed.Add(1)
					if scenario != "lost-forget" {
						response.Forget = []contract.DesktopReference{intent.Reference()}
					}
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			client, err := remoteio.NewClient(server.URL, strings.Repeat("a", 64))
			if err != nil {
				t.Fatal(err)
			}
			defer client.HTTP.CloseIdleConnections()
			proof := desktopExitProof{Type: "process-exited", Reference: intent.Reference(), Provider: intent.Provider}
			if scenario == "proof-foreign" {
				proof.Reference.Target.Generation = strings.Repeat("f", 64)
			}
			if scenario == "proof-provider" {
				proof.Provider.Artifact = strings.Repeat("f", 64)
			}
			var input bytes.Buffer
			if scenario != "eof" {
				_ = json.NewEncoder(&input).Encode(proof)
			}
			var output desktopBuffer
			err = streamDesktopCleanup(context.Background(), root, request, permissions, client, io.NopCloser(&input), &output)
			if scenario == "correct" {
				if err != nil || acceptedClosed.Load() != 1 || !strings.Contains(output.String(), `"type":"forget"`) {
					t.Fatal("exact native view-only closure was lost", err)
				}
			} else if err == nil || strings.Contains(output.String(), `"type":"forget"`) {
				t.Fatal("uncertain/foreign/control cleanup was reported as closed")
			}
			if scenario != "correct" && scenario != "lost-forget" && acceptedClosed.Load() != 0 {
				t.Fatal("non-view or unproven operation released native uncertainty")
			}
			if strings.Contains(output.String(), intent.Nonce) || strings.Contains(output.String(), `"signals"`) {
				t.Fatal("cleanup transport exposed the original intent or signaling")
			}
		})
	}
}

func TestDesktopCleanupMetadataRejectsMissingAckAndContent(t *testing.T) {
	intent := desktopTestIntent()
	ack := uint64(1)
	request := desktopCleanupRequest{Reference: intent.Reference(), Provider: intent.Provider, Ack: &ack}
	valid, _ := json.Marshal(request)
	if _, err := decodeDesktopCleanupRequest(valid); err != nil {
		t.Fatal(err)
	}
	for _, extra := range []string{`,"ack":2`, `,"payload":"private"`} {
		invalid := append(append([]byte(nil), valid[:len(valid)-1]...), []byte(extra+"}")...)
		if _, err := decodeDesktopCleanupRequest(invalid); err == nil {
			t.Fatal("cleanup admitted duplicate authority or content")
		}
	}
	request.Ack = nil
	missing, _ := json.Marshal(request)
	if _, err := decodeDesktopCleanupRequest(missing); err == nil {
		t.Fatal("cleanup invented an original status acknowledgment")
	}
	ack = contract.MaxDesktopSignals + 1
	request.Ack = &ack
	oversized, _ := json.Marshal(request)
	if _, err := decodeDesktopCleanupRequest(oversized); err == nil {
		t.Fatal("cleanup admitted acknowledgment outside the signaling contract")
	}
	if err := Run([]string{"desktop-cleanup"}, nil); err == nil {
		t.Fatal("cleanup bridge started without private stdio")
	}
}

func TestDesktopCleanupPairedIdentityAndOwnerMatchBeforeNetwork(t *testing.T) {
	for _, scenario := range []string{"node", "fingerprint", "provider", "legacy"} {
		t.Run(scenario, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				http.Error(w, "must not reach network", http.StatusServiceUnavailable)
			}))
			defer server.Close()
			root := privateConnectionDirectory(t)
			identity, err := remoteidentity.New()
			if err != nil {
				t.Fatal(err)
			}
			broker, err := remoteidentity.New()
			if err != nil {
				t.Fatal(err)
			}
			config := contract.WorkerConfig{Schema: contract.Version, Node: "worker", Endpoint: server.URL, Token: strings.Repeat("a", 64),
				MachineKey: identity.PublicKey, MachinePrivateKey: identity.PrivateKey, BrokerKey: broker.PublicKey}
			intent := desktopTestIntent()
			intent.Target.Fingerprint = remoteidentity.Fingerprint(identity.PublicKey)
			intent.ID, _ = intent.Digest()
			ack := uint64(1)
			request := desktopCleanupRequest{Reference: intent.Reference(), Provider: intent.Provider, Ack: &ack}
			permissions := remoteio.DesktopPermissions{Schema: contract.DesktopVersion, Provider: intent.Provider, View: true}
			switch scenario {
			case "node":
				config.Node = "other"
			case "fingerprint":
				request.Reference.Target.Fingerprint = strings.Repeat("f", 64)
			case "provider":
				permissions.Provider.Artifact = strings.Repeat("f", 64)
			case "legacy":
				config.MachineKey, config.BrokerKey, config.MachinePrivateKey = "", "", nil
			}
			if err := remoteio.WritePrivate(filepath.Join(root, "worker.json"), config); err != nil {
				t.Fatal(err)
			}
			if err := remoteio.SaveDesktopPermissions(root, permissions); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "cleanup.json")
			if err := remoteio.WritePrivate(path, request); err != nil {
				t.Fatal(err)
			}
			if err := desktopCleanupBridge(context.Background(), root, path); err == nil || calls.Load() != 0 {
				t.Fatal("foreign/legacy cleanup reached machine publication or native proof")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Cancelling after the cleanup directive models a deadline/disconnect while
// native process evidence is still outstanding, without a wall-clock sleep.
type cancelDesktopCleanupOutput struct {
	desktopBuffer
	cancel context.CancelFunc
}

func (output *cancelDesktopCleanupOutput) Write(data []byte) (int, error) {
	written, err := output.desktopBuffer.Write(data)
	output.cancel()
	return written, err
}

func TestDesktopCleanupCancelledProofWaitCannotPublishClosed(t *testing.T) {
	intent := desktopTestIntent()
	intent.Mode = "view"
	intent.ID, _ = intent.Digest()
	ack := uint64(1)
	request := desktopCleanupRequest{Reference: intent.Reference(), Provider: intent.Provider, Ack: &ack}
	root := privateConnectionDirectory(t)
	permissions := remoteio.DesktopPermissions{Schema: contract.DesktopVersion, Provider: intent.Provider, View: true}
	if err := remoteio.SaveDesktopPermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: intent.Epoch,
			Directives: []contract.DesktopDirective{{Open: intent, Close: true, Operator: strings.Repeat("a", 64)}}})
	}))
	defer server.Close()
	client, err := remoteio.NewClient(server.URL, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input, producer := io.Pipe()
	defer producer.Close()
	output := cancelDesktopCleanupOutput{cancel: cancel}
	err = streamDesktopCleanup(ctx, root, request, permissions, client, input, &output)
	if err == nil || calls.Load() != 1 || strings.Contains(output.String(), `"type":"forget"`) {
		t.Fatal("cancellation manufactured a native process-exit proof")
	}
}

func TestDesktopCleanupPublicRunAdmitsPrivateRequestBeforeBridge(t *testing.T) {
	root := privateConnectionDirectory(t)
	path := filepath.Join(root, "cleanup.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	// Exercise all global CLI admission and routing, not just the stream helper.
	// Deliberately invalid metadata must reach the handler's strict decoder,
	// before state locking, machine publication, native proof or any network.
	err := Run([]string{"desktop-cleanup", "--stdio", "--request", path, "--state", root, "--auth", "never"}, nil)
	if err == nil || !strings.Contains(err.Error(), "desktop cleanup requires exact original metadata") {
		t.Fatalf("public cleanup request never reached its private-file decoder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "desktop.lock")); !os.IsNotExist(err) {
		t.Fatal("invalid cleanup metadata reached worker state or native admission")
	}
	err = Run([]string{"desktop-cleanup", "--stdio", "--request", "-", "--state", root, "--auth", "never"}, nil)
	if err == nil || !strings.Contains(err.Error(), "desktop-cleanup requires --request <private-file>") {
		t.Fatalf("global CLI admission bypassed the cleanup private-transport contract: %v", err)
	}
}

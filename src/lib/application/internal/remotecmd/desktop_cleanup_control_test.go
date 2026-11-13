package remotecmd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type challengedDesktopCleanupOutput struct {
	desktopBuffer
	cleanup func(desktopCleanupDirective)
}

func (output *challengedDesktopCleanupOutput) Write(data []byte) (int, error) {
	count, err := output.desktopBuffer.Write(data)
	var frame desktopCleanupDirective
	if json.Unmarshal(data, &frame) == nil && frame.Type == "cleanup" {
		output.cleanup(frame)
	}
	return count, err
}

func TestDesktopControlCleanupRequiresFreshResetAndExitProof(t *testing.T) {
	intent := desktopTestIntent()
	var previousChallenge string
	for _, scenario := range []string{
		"correct", "fresh", "revoked-input", "view-directive", "foreign-reference", "foreign-provider", "foreign-epoch",
		"no-close", "live-lease", "signals", "policy-before-challenge", "eof", "cancelled", "plain-process-exit", "replayed-proof",
		"empty-challenge", "wrong-challenge",
		"wrong-proof-reference", "wrong-proof-provider", "reset-false", "exit-false", "missing-reset", "extra-content",
		"duplicate-challenge", "oversized-proof", "policy-after-proof", "forged-ack", "lost-forget", "foreign-forget",
	} {
		t.Run(scenario, func(t *testing.T) {
			root := privateConnectionDirectory(t)
			permissions := remoteio.DesktopPermissions{Schema: contract.DesktopVersion, Provider: intent.Provider, View: true, Input: true}
			if scenario == "revoked-input" {
				permissions.Input = false
			}
			if err := remoteio.SaveDesktopPermissions(root, permissions); err != nil {
				t.Fatal(err)
			}
			ack := uint64(1)
			if scenario == "forged-ack" {
				ack = 63
			}
			request := desktopCleanupRequest{Reference: intent.Reference(), Provider: intent.Provider, Ack: &ack,
				ProofBasis: desktopInputResetProofBasis}
			var calls, acceptedClosed atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var exchange contract.DesktopExchange
				if err := json.NewDecoder(r.Body).Decode(&exchange); err != nil {
					t.Error(err)
					return
				}
				if exchange.Catalog.Target != intent.Target || exchange.Catalog.Provider != intent.Provider ||
					exchange.Catalog.View || exchange.Catalog.Input {
					t.Error("control cleanup renewed capabilities or changed original routing")
				}
				response := contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: intent.Epoch}
				if calls.Add(1) == 1 {
					if len(exchange.Reports) != 0 {
						t.Error("closed report preceded native input reset and process-exit proof")
					}
					open := intent
					switch scenario {
					case "view-directive":
						open.Mode = "view"
					case "foreign-reference":
						open.Target.Generation = strings.Repeat("f", 64)
					case "foreign-provider":
						open.Provider.Artifact = strings.Repeat("f", 64)
					case "foreign-epoch":
						response.Epoch = strings.Repeat("f", 64)
					}
					open.ID, _ = open.Digest()
					directive := contract.DesktopDirective{Open: open, Close: true, Operator: strings.Repeat("a", 64)}
					if scenario == "no-close" {
						directive.Close = false
					}
					if scenario == "live-lease" {
						directive.LeaseMillis = 1
					}
					if scenario == "signals" {
						directive.Signals = []contract.DesktopSignal{{Reference: intent.Reference(), Sequence: 1, Type: "binding", Fingerprint: strings.Repeat("a", 64)}}
					}
					response.Directives = []contract.DesktopDirective{directive}
					if scenario == "policy-before-challenge" {
						changed := permissions
						changed.View, changed.Input = false, false
						if err := remoteio.SaveDesktopPermissions(root, changed); err != nil {
							t.Error(err)
						}
					}
				} else {
					if len(exchange.Reports) != 1 || exchange.Reports[0].Reference != intent.Reference() ||
						exchange.Reports[0].State != "closed" || len(exchange.Reports[0].Signals) != 0 {
						t.Error("reset proof changed the original session or forwarded content")
					}
					if exchange.Reports[0].Ack != 1 {
						http.Error(w, "wrong original acknowledgment", http.StatusBadRequest)
						return
					}
					acceptedClosed.Add(1)
					if scenario != "lost-forget" {
						forgotten := intent.Reference()
						if scenario == "foreign-forget" {
							forgotten.Target.Generation = strings.Repeat("f", 64)
						}
						response.Forget = []contract.DesktopReference{forgotten}
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
			input, producer := io.Pipe()
			defer producer.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var produced chan error
			output := challengedDesktopCleanupOutput{cleanup: func(frame desktopCleanupDirective) {
				if frame.Reference != intent.Reference() || frame.Provider != intent.Provider || frame.Mode != "control" ||
					!frame.Close || frame.LeaseMillis != 0 || frame.ProofBasis != desktopInputResetProofBasis {
					t.Error("native reset challenge changed original revoked control metadata")
				}
				decoded, err := hex.DecodeString(frame.Challenge)
				if err != nil || len(decoded) != 32 || frame.Challenge != strings.ToLower(frame.Challenge) || frame.Challenge == previousChallenge {
					t.Error("control recovery did not issue a fresh canonical challenge")
				}
				if scenario == "correct" {
					previousChallenge = frame.Challenge
				}
				proof := desktopInputResetProof{Type: "input-reset-and-provider-exited", Reference: intent.Reference(),
					Provider: intent.Provider, Challenge: frame.Challenge, ResetVerified: true, ProviderExited: true}
				switch scenario {
				case "replayed-proof":
					proof.Challenge = previousChallenge
				case "empty-challenge":
					proof.Challenge = ""
				case "wrong-challenge":
					proof.Challenge = strings.Repeat("f", 64)
				case "wrong-proof-reference":
					proof.Reference.Target.Generation = strings.Repeat("f", 64)
				case "wrong-proof-provider":
					proof.Provider.Artifact = strings.Repeat("f", 64)
				case "reset-false":
					proof.ResetVerified = false
				case "exit-false":
					proof.ProviderExited = false
				}
				data, _ := json.Marshal(proof)
				if scenario == "policy-after-proof" {
					// Complete the native evidence, then change the owner policy
					// before Core receives it. The post-proof check must reject it.
					changed := permissions
					changed.View, changed.Input = false, false
					if err := remoteio.SaveDesktopPermissions(root, changed); err != nil {
						t.Error(err)
					}
				}
				if scenario == "cancelled" {
					cancel()
				}
				switch scenario {
				case "plain-process-exit":
					data, _ = json.Marshal(desktopExitProof{Type: "process-exited", Reference: intent.Reference(), Provider: intent.Provider})
				case "missing-reset":
					var value map[string]any
					_ = json.Unmarshal(data, &value)
					delete(value, "reset_verified")
					data, _ = json.Marshal(value)
				case "extra-content":
					data = append(data[:len(data)-1], []byte(`,"signals":[]}`)...)
				case "duplicate-challenge":
					data = append(data[:len(data)-1], []byte(`,"challenge":"`+frame.Challenge+`"}`)...)
				case "oversized-proof":
					data = append(data[:len(data)-1], []byte(`,"padding":"`+strings.Repeat("x", 4096)+`"}`)...)
				}
				produced = make(chan error, 1)
				go func() {
					var err error
					if scenario != "eof" {
						_, err = producer.Write(append(data, '\n'))
					}
					_ = producer.Close()
					produced <- err
				}()
			}}
			err = streamDesktopCleanup(ctx, root, request, permissions, client, input, &output)
			if produced != nil {
				<-produced
			}
			success := scenario == "correct" || scenario == "fresh" || scenario == "revoked-input"
			if success {
				if err != nil || acceptedClosed.Load() != 1 || !strings.Contains(output.String(), `"type":"forget"`) {
					t.Fatal("exact challenged reset proof did not receive exact Forget", err)
				}
			} else if err == nil || strings.Contains(output.String(), `"type":"forget"`) {
				t.Fatal("invalid or unacknowledged control recovery became known closed")
			}
			if !success && scenario != "lost-forget" && scenario != "foreign-forget" && acceptedClosed.Load() != 0 {
				t.Fatal("control uncertainty was released without exact fresh reset and exit proof")
			}
			if strings.Contains(output.String(), intent.Nonce) || strings.Contains(output.String(), `"signals"`) {
				t.Fatal("control cleanup exposed old intent or signaling content")
			}
		})
	}
}

func TestDesktopCleanupProofBasisIsExplicitAndBounded(t *testing.T) {
	intent := desktopTestIntent()
	ack := uint64(1)
	request := desktopCleanupRequest{Reference: intent.Reference(), Provider: intent.Provider, Ack: &ack}
	for _, basis := range []string{"", desktopExitProofBasis, desktopInputResetProofBasis} {
		request.ProofBasis = basis
		data, _ := json.Marshal(request)
		decoded, err := decodeDesktopCleanupRequest(data)
		if err != nil || decoded.ProofBasis != basis {
			t.Fatal("supported explicit proof basis lost", err)
		}
	}
	request.ProofBasis = "abandon-or-guess"
	data, _ := json.Marshal(request)
	if _, err := decodeDesktopCleanupRequest(data); err == nil {
		t.Fatal("unknown proof basis admitted")
	}
}

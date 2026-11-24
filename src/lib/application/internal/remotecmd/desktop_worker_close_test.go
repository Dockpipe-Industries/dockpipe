package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestDesktopWorkerCleanupAcknowledgesOnlyExactClosedProof(t *testing.T) {
	request := desktopTestIntent()
	for _, scenario := range []string{"correct", "missing", "foreign", "unknown", "new-directive"} {
		t.Run(scenario, func(t *testing.T) {
			report := contract.DesktopReport{Reference: request.Reference(), State: "closed", Ack: 1}
			response := contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: request.Epoch,
				Forget: []contract.DesktopReference{request.Reference()}}
			switch scenario {
			case "missing":
				response.Forget = nil
			case "foreign":
				response.Forget[0].ID = strings.Repeat("f", 64)
			case "unknown":
				report.State = "unknown"
			case "new-directive":
				response.Directives = []contract.DesktopDirective{{Open: request, Close: true}}
			}
			var output bytes.Buffer
			err := emitDesktopCleanupForget(json.NewEncoder(&output), response, report)
			if scenario == "correct" {
				if err != nil || !strings.Contains(output.String(), `"type":"forget"`) {
					t.Fatal("exact native closure lost", err)
				}
			} else if err == nil || output.Len() != 0 {
				t.Fatal("uncertain or foreign cleanup fabricated an acknowledgment")
			}
		})
	}
}

func TestDesktopWorkerFenceDrainOnlyForwardsExactCleanupProof(t *testing.T) {
	for _, scenario := range []string{"closed", "lost-ack", "unknown", "running", "foreign", "signals", "eof"} {
		t.Run(scenario, func(t *testing.T) {
			request := desktopTestIntent()
			catalog := contract.DesktopCatalog{
				Schema: contract.DesktopVersion, Target: request.Target, Provider: request.Provider,
				View: true, Input: true, AdministratorInput: true,
				Displays: []contract.DesktopDisplay{{Source: "external-display", Width: 3840, Height: 2160}},
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var exchange contract.DesktopExchange
				if err := json.NewDecoder(r.Body).Decode(&exchange); err != nil {
					t.Error(err)
					return
				}
				if exchange.Catalog.View || exchange.Catalog.Input || exchange.Catalog.AdministratorInput || len(exchange.Catalog.Displays) != 0 {
					t.Error("cleanup drain renewed desktop grants or published displays")
				}
				count := calls.Add(1)
				response := contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: request.Epoch}
				if count == 1 {
					response.Directives = []contract.DesktopDirective{{Open: request, Operator: strings.Repeat("a", 64), Close: true}}
				} else {
					if len(exchange.Reports) != 1 || exchange.Reports[0].Reference != request.Reference() || len(exchange.Reports[0].Signals) != 0 {
						t.Error("cleanup drain forwarded changed original reference/signals")
					}
					if scenario == "closed" {
						response.Forget = []contract.DesktopReference{request.Reference()}
					}
					if scenario == "lost-ack" {
						http.Error(w, "test transport failure", http.StatusServiceUnavailable)
						return
					}
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			client, err := remoteio.NewClient(server.URL, strings.Repeat("f", 64))
			if err != nil {
				t.Fatal(err)
			}
			defer client.HTTP.CloseIdleConnections()
			report := contract.DesktopReport{Reference: request.Reference(), State: "closed"}
			switch scenario {
			case "unknown":
				report.State = "unknown"
			case "running":
				report.State = "running"
			case "foreign":
				report.Reference.Target.Generation = strings.Repeat("f", 64)
			case "signals":
				report.Signals = []contract.DesktopSignal{{}}
			}
			frames := make(chan desktopFrame, 1)
			failures := make(chan error, 1)
			if scenario == "eof" {
				failures <- io.EOF
			} else {
				frames <- desktopFrame{Type: "report", Report: &report}
				close(frames)
			}
			var output desktopBuffer
			journal := desktopCleanupTestJournal(t)
			drainDesktopWorker(context.Background(), client, journal, contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: request.Epoch, Catalog: catalog}, nil,
				json.NewEncoder(&output), frames, failures, io.NopCloser(strings.NewReader("")), &output)
			expected := int32(1)
			if scenario == "closed" || scenario == "unknown" || scenario == "lost-ack" {
				expected = 2
			}
			if calls.Load() != expected {
				t.Fatal("cleanup-only drain accepted forbidden effects or lost native proof")
			}
			if strings.Contains(output.String(), `"type":"forget"`) != (scenario == "closed") {
				t.Fatal("cleanup proof was confused with uncertainty")
			}
			if !strings.Contains(output.String(), `"lease_millis":0`) || !strings.Contains(output.String(), `"close":true`) {
				t.Fatal("native shutdown directive retained desktop authority")
			}
			_, err = os.Stat(string(journal))
			if scenario == "lost-ack" && err != nil || scenario != "lost-ack" && !os.IsNotExist(err) {
				t.Fatalf("journal must retain only a verified close without an acknowledgment: %v", err)
			}
		})
	}
}

func TestDesktopWorkerFencePreservesAlreadyQueuedCleanupProof(t *testing.T) {
	request := desktopTestIntent()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var exchange contract.DesktopExchange
		if err := json.NewDecoder(r.Body).Decode(&exchange); err != nil {
			t.Error(err)
			return
		}
		if exchange.Catalog.View || exchange.Catalog.Input || len(exchange.Reports) != 1 || exchange.Reports[0].Reference != request.Reference() || exchange.Reports[0].State != "closed" {
			t.Error("local fence discarded a native closure proof already queued before failure")
		}
		_ = json.NewEncoder(w).Encode(contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: request.Epoch, Forget: []contract.DesktopReference{request.Reference()}})
	}))
	defer server.Close()
	client, err := remoteio.NewClient(server.URL, strings.Repeat("f", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	catalog := contract.DesktopCatalog{Schema: contract.DesktopVersion, Target: request.Target, Provider: request.Provider, View: true, Input: true}
	active := contract.DesktopDirective{Open: request, Operator: strings.Repeat("a", 64), LeaseMillis: 1}
	var output desktopBuffer
	drainDesktopWorker(context.Background(), client, desktopCleanupTestJournal(t), contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: request.Epoch, Catalog: catalog,
		Reports: []contract.DesktopReport{{Reference: request.Reference(), State: "closed"}}}, &active,
		json.NewEncoder(&output), nil, nil, io.NopCloser(strings.NewReader("")), &output)
	if calls.Load() != 1 || !strings.Contains(output.String(), `"type":"forget"`) {
		t.Fatal("known closure became stranded unknown quota during lock race")
	}
}

func TestDesktopWorkerFenceDrainsQueuedOrdinaryReportsBeforeClosedProof(t *testing.T) {
	for _, state := range []string{"opening", "running"} {
		t.Run(state, func(t *testing.T) {
			request := desktopTestIntent()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var exchange contract.DesktopExchange
				if err := json.NewDecoder(r.Body).Decode(&exchange); err != nil {
					t.Error(err)
					return
				}
				if exchange.Catalog.View || exchange.Catalog.Input {
					t.Error("cleanup drain renewed native authority")
				}
				response := contract.DesktopExchangeResult{Schema: contract.DesktopVersion, Epoch: request.Epoch}
				if calls.Add(1) == 1 {
					if len(exchange.Reports) != 0 {
						t.Error("initial fence publication forwarded an ordinary report")
					}
					response.Directives = []contract.DesktopDirective{{Open: request, Operator: strings.Repeat("a", 64), Close: true}}
				} else {
					if len(exchange.Reports) != 1 {
						t.Error("native cleanup report was lost behind an ordinary report")
						return
					}
					report := exchange.Reports[0]
					if report.Reference != request.Reference() || report.State != "closed" || report.Ack != 1 || len(report.Signals) != 0 {
						t.Error("cleanup drain changed the original proof or forwarded queued binding signals")
					}
					response.Forget = []contract.DesktopReference{request.Reference()}
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			client, err := remoteio.NewClient(server.URL, strings.Repeat("f", 64))
			if err != nil {
				t.Fatal(err)
			}
			defer client.HTTP.CloseIdleConnections()
			catalog := contract.DesktopCatalog{Schema: contract.DesktopVersion, Target: request.Target, Provider: request.Provider, View: true, Input: true}
			ordinary := contract.DesktopReport{Reference: request.Reference(), State: state, Ack: 1,
				Signals: []contract.DesktopSignal{{Reference: request.Reference(), Sequence: 1, Type: "binding", Fingerprint: strings.Repeat("b", 64)}}}
			closed := contract.DesktopReport{Reference: request.Reference(), State: "closed", Ack: 1}
			frames := make(chan desktopFrame, 2)
			frames <- desktopFrame{Type: "report", Report: &ordinary}
			frames <- desktopFrame{Type: "report", Report: &closed}
			close(frames)
			var output desktopBuffer
			drainDesktopWorker(context.Background(), client, desktopCleanupTestJournal(t), contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: request.Epoch, Catalog: catalog}, nil,
				json.NewEncoder(&output), frames, nil, io.NopCloser(strings.NewReader("")), &output)
			if calls.Load() != 2 {
				t.Fatalf("queued ordinary report discarded the later native cleanup proof: %d exchanges", calls.Load())
			}
			decoder := json.NewDecoder(strings.NewReader(output.String()))
			var closing, forgotten desktopEvent
			if decoder.Decode(&closing) != nil || closing.Type != "directive" || closing.Directive == nil || !closing.Directive.Close || closing.Directive.LeaseMillis != 0 || len(closing.Directive.Signals) != 0 {
				t.Fatal("fenced directive retained media or signaling authority")
			}
			if decoder.Decode(&forgotten) != nil || forgotten.Type != "forget" || forgotten.Reference == nil || *forgotten.Reference != request.Reference() {
				t.Fatal("cleanup drain did not retain the exact authenticated Forget")
			}
		})
	}
}

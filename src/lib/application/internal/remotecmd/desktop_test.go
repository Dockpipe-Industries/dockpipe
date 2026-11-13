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
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func desktopTestIntent() contract.DesktopOpen {
	request := contract.DesktopOpen{Schema: contract.DesktopVersion, Target: contract.FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Epoch: strings.Repeat("c", 64), Provider: contract.DesktopProvider{ID: "fixture.native", Version: "1", Artifact: strings.Repeat("e", 64)}, Mode: "control", Nonce: strings.Repeat("d", 64), Deadline: time.Now().UTC().Add(time.Minute)}
	request.ID, _ = request.Digest()
	return request
}

type desktopBuffer struct{ bytes.Buffer }

func (buffer *desktopBuffer) Close() error { return nil }

func TestDesktopPrepareIsLocalAndContentFree(t *testing.T) {
	request := desktopTestIntent()
	request.ID = ""
	data, _ := json.Marshal(request)
	decoded, err := decodeDesktopOpen(data, time.Now())
	if err != nil || decoded.ID == "" {
		t.Fatal("desktop local canonical preparation", err)
	}
	path := filepath.Join(t.TempDir(), "intent.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := prepareDesktop(context.Background(), path, &output); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output.Bytes(), []byte(request.Nonce)) || bytes.Contains(output.Bytes(), []byte(`"nonce"`)) || bytes.Contains(output.Bytes(), []byte(`"parameters"`)) {
		t.Fatal("desktop prepared receipt retained transport secrets")
	}
	var prepared desktopPrepared
	if err := json.Unmarshal(output.Bytes(), &prepared); err != nil || prepared.Reference != decoded.Reference() {
		t.Fatal("prepared desktop identity changed", err)
	}
}

func TestDesktopViewerRequiresExactPreparedACKBeforeOpening(t *testing.T) {
	for _, scenario := range []string{"eof", "foreign", "signal-before-ack", "correct"} {
		t.Run(scenario, func(t *testing.T) {
			request := desktopTestIntent()
			var opens atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/desktop-open" {
					opens.Add(1)
				}
				_ = json.NewEncoder(w).Encode(contract.DesktopView{Reference: request.Reference(), State: "closed"})
			}))
			defer server.Close()
			client, err := remoteio.NewClient(server.URL, strings.Repeat("f", 64))
			if err != nil {
				t.Fatal(err)
			}
			defer client.HTTP.CloseIdleConnections()
			reference := request.Reference()
			if scenario == "foreign" {
				reference.Target.Generation = strings.Repeat("f", 64)
			}
			var input bytes.Buffer
			if scenario != "eof" {
				frame := desktopFrame{Type: "prepared-ack", Reference: &reference}
				if scenario == "signal-before-ack" {
					frame = desktopFrame{Type: "signal", Signal: &contract.DesktopSignal{Reference: reference, Sequence: 1, Type: "offer", SDP: "v=0\r\n"}}
				}
				_ = json.NewEncoder(&input).Encode(frame)
			}
			var output desktopBuffer
			err = streamDesktopViewer(context.Background(), client, request, io.NopCloser(&input), &output)
			if scenario == "correct" {
				if err != nil || opens.Load() != 1 {
					t.Fatal("correct exact prepared intent was not opened once", err)
				}
			} else if err == nil || opens.Load() != 0 {
				t.Fatal("unacknowledged desktop preparation reached broker effects", err)
			}
		})
	}
}

func TestDesktopPrivateFramesRejectForeignCapabilitiesAndContent(t *testing.T) {
	for _, raw := range []string{
		`{"type":"frame","data":"private"}`,
		`{"type":"signal","signal":{},"input":true}`,
		`{"type":"close","clipboard":"private"}`,
		`{"type":"ready","provider":{"id":"fixture","version":"1","artifact":"short"}}`,
		`{"type":"close","type":"ready"}`,
	} {
		if _, err := decodeDesktopFrame([]byte(raw)); err == nil {
			t.Fatal("private desktop bridge admitted unsupported content")
		}
	}
	if err := Run([]string{"desktop-worker"}, nil); err == nil {
		t.Fatal("desktop worker bridge ran without explicit private stdio")
	}
	if err := Run([]string{"features", "--stdio"}, nil); err == nil {
		t.Fatal("desktop transport changed existing feature command options")
	}
}

func TestDesktopWorkerReadyMustMatchOwnerBeforeNetwork(t *testing.T) {
	for _, scenario := range []string{"eof", "artifact", "grant", "locked", "lock-unknown", "sleep-unknown", "correct"} {
		t.Run(scenario, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				http.Error(w, "fixture stops before broker admission", http.StatusServiceUnavailable)
			}))
			defer server.Close()
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			workerIdentity, err := remoteidentity.New()
			if err != nil {
				t.Fatal(err)
			}
			brokerIdentity, err := remoteidentity.New()
			if err != nil {
				t.Fatal(err)
			}
			config := contract.WorkerConfig{Schema: contract.Version, Node: "worker", Endpoint: server.URL, Token: strings.Repeat("a", 64),
				MachineKey: workerIdentity.PublicKey, MachinePrivateKey: workerIdentity.PrivateKey, BrokerKey: brokerIdentity.PublicKey}
			if err := remoteio.WritePrivate(filepath.Join(root, "worker.json"), config); err != nil {
				t.Fatal(err)
			}
			permissions := remoteio.DesktopPermissions{Schema: contract.DesktopVersion, Provider: desktopTestIntent().Provider, View: true, Input: false}
			if err := remoteio.SaveDesktopPermissions(root, permissions); err != nil {
				t.Fatal(err)
			}
			provider := permissions.Provider
			if scenario == "artifact" {
				provider.Artifact = strings.Repeat("f", 64)
			}
			var input bytes.Buffer
			if scenario != "eof" {
				_ = json.NewEncoder(&input).Encode(desktopFrame{Type: "ready", Provider: &provider, View: true, Input: scenario == "grant"})
			}
			snapshot := osauth.Snapshot{LockKnown: true, SleepKnown: true}
			if scenario == "locked" {
				snapshot.Locked = true
			}
			if scenario == "lock-unknown" {
				snapshot.LockKnown = false
			}
			if scenario == "sleep-unknown" {
				snapshot.SleepKnown = false
			}
			var output desktopBuffer
			if err := streamDesktopWorkerWithSession(context.Background(), root, io.NopCloser(&input), &output, remoteio.NewDesktopSessionFence(desktopSessionFixture{snapshot: snapshot})); err == nil {
				t.Fatal("fixture unexpectedly established a native desktop worker")
			}
			if scenario == "correct" {
				if calls.Load() != 1 {
					t.Fatal("valid owner readiness did not reach one fresh machine challenge")
				}
			} else if calls.Load() != 0 {
				t.Fatal("missing/foreign/escalated provider readiness reached network publication")
			}
		})
	}
}

type desktopUnlockedFixture struct{}

func (desktopUnlockedFixture) Snapshot(context.Context) (osauth.Snapshot, error) {
	return osauth.Snapshot{LockKnown: true, SleepKnown: true}, nil
}

type desktopSessionFixture struct{ snapshot osauth.Snapshot }

func (fixture desktopSessionFixture) Snapshot(context.Context) (osauth.Snapshot, error) {
	return fixture.snapshot, nil
}

func TestDesktopViewerBindingThenCloseEmitsExactNativeClosedProof(t *testing.T) {
	request := desktopTestIntent()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		view := contract.DesktopView{Reference: request.Reference(), State: "running", LeaseMillis: 5000, Ack: 1}
		switch r.URL.Path {
		case "/v1/desktop-open":
			view.State, view.Ack = "opening", 0
		case "/v1/desktop-pulse":
			var read contract.DesktopRead
			_ = json.NewDecoder(r.Body).Decode(&read)
			if read.After == 0 {
				view.Signals = []contract.DesktopSignal{{Reference: request.Reference(), Sequence: 1, Type: "binding", Fingerprint: strings.Repeat("f", 64)}}
			}
		case "/v1/desktop-close":
			view.State, view.LeaseMillis = "closed", 0
		}
		_ = json.NewEncoder(w).Encode(view)
	}))
	defer server.Close()
	client, err := remoteio.NewClient(server.URL, strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	input, producer := io.Pipe()
	consumer, output := io.Pipe()
	defer producer.Close()
	defer consumer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- streamDesktopViewer(ctx, client, request, input, output) }()
	decoder := json.NewDecoder(consumer)
	var prepared desktopPrepared
	if err := decoder.Decode(&prepared); err != nil || prepared.Reference != request.Reference() {
		t.Fatal("canonical preparation missing", err)
	}
	reference := request.Reference()
	encoder := json.NewEncoder(producer)
	if err := encoder.Encode(desktopFrame{Type: "prepared-ack", Reference: &reference}); err != nil {
		t.Fatal(err)
	}
	var event desktopEvent
	if err := decoder.Decode(&event); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(desktopFrame{Type: "signal", Signal: &contract.DesktopSignal{Reference: reference, Sequence: 1, Type: "binding", Fingerprint: strings.Repeat("a", 64)}}); err != nil {
		t.Fatal(err)
	}
	for {
		event = desktopEvent{}
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.View != nil && len(event.View.Signals) == 1 {
			break
		}
	}
	if err := encoder.Encode(desktopFrame{Type: "close"}); err != nil {
		t.Fatal(err)
	}
	for {
		event = desktopEvent{}
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.View != nil && event.View.State == "closed" {
			break
		}
	}
	if event.View.Reference != reference || event.View.LeaseMillis != 0 || len(event.View.Signals) != 0 {
		t.Fatal("closure lost exact proof after incoming binding cursor advanced")
	}
	if err := <-finished; err != nil {
		t.Fatal("known native closure became unknown", err)
	}
}

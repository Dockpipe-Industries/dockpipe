//go:build linux || darwin

package remote

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func terminalFixture(t *testing.T) (*Broker, *Client, *Client, string, FeaturePermissions, contract.FeatureCatalog) {
	t.Helper()
	broker, admin, worker, config, root, permissions, _ := featureFixture(t)
	worker.HTTP.Transport = &featureDisconnectTransport{RoundTripper: worker.HTTP.Transport}
	permissions.Roots[0].Terminal = true
	permissions.TerminalProfile = &TerminalProfile{Executable: "/bin/sh", Args: []string{"-i"}}
	if err := SaveFeaturePermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	instance, _ := Secret()
	go func() { done <- runTerminalWorker(ctx, root, config, worker, instance) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
		case <-time.After(4 * time.Second):
			t.Error("terminal worker did not stop")
		}
	})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		catalogs, err := admin.FeatureCatalog(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, catalog := range catalogs {
			if catalog.TerminalEpoch != "" && terminalRoot(catalog, "project") {
				return broker, admin, worker, root, permissions, catalog
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("terminal catalog did not become available")
	return nil, nil, nil, "", FeaturePermissions{}, contract.FeatureCatalog{}
}

func terminalIntent(t *testing.T, catalog contract.FeatureCatalog) contract.TerminalOpen {
	t.Helper()
	nonce, _ := Secret()
	request := contract.TerminalOpen{Schema: contract.TerminalVersion, Target: catalog.Target, Epoch: catalog.TerminalEpoch,
		Root: "project", Nonce: nonce, Deadline: time.Now().UTC().Add(time.Minute), Columns: 80, Rows: 24}
	request.ID, _ = request.Digest()
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	return request
}

func awaitTerminal(t *testing.T, client *Client, reference contract.TerminalReference, condition func(contract.TerminalView) bool) contract.TerminalView {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := client.PulseTerminal(context.Background(), reference); err != nil {
			t.Fatal(err)
		}
		view, err := client.ReadTerminal(context.Background(), reference, 0)
		if err != nil {
			t.Fatal(err)
		}
		if condition(view) {
			return view
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("terminal condition did not complete")
	return contract.TerminalView{}
}

func TestTerminalRealPTYTypedInputResizeAndNoJournal(t *testing.T) {
	broker, admin, _, root, _, catalog := terminalFixture(t)
	request := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal("exact open retry failed", err)
	}
	reference := request.Reference()
	input := contract.TerminalInput{Reference: reference, Sequence: 1, Data: []byte("stty -echo; test -t 0 && printf '__%s__\\n' REAL_REMOTE_PTY\n")}
	if _, err := admin.InputTerminal(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	view := awaitTerminal(t, admin, reference, func(view contract.TerminalView) bool {
		return view.AckSequence == 1 && bytes.Contains(view.Data, []byte("__REAL_REMOTE_PTY__\r\n"))
	})
	before := view.End
	if _, err := admin.InputTerminal(context.Background(), input); err != nil {
		t.Fatal("exact input retry failed", err)
	}
	if _, err := admin.ResizeTerminal(context.Background(), contract.TerminalResize{Reference: reference, Columns: 113, Rows: 37}); err != nil {
		t.Fatal(err)
	}
	input.Sequence, input.Data = 2, []byte("stty size; printf '__%s__\\n' SECOND_REMOTE_INPUT\n")
	if _, err := admin.InputTerminal(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	view = awaitTerminal(t, admin, reference, func(view contract.TerminalView) bool {
		return view.AckSequence == 2 && bytes.Contains(view.Data, []byte("37 113\r\n"))
	})
	if bytes.Count(view.Data, []byte("__REAL_REMOTE_PTY__\r\n")) != 1 || view.End <= before {
		t.Fatal("input retry replayed bytes or cursor did not advance")
	}
	if _, err := admin.CloseTerminal(context.Background(), reference); err != nil {
		t.Fatal(err)
	}
	awaitTerminal(t, admin, reference, func(view contract.TerminalView) bool { return view.State == "closed" })
	if _, err := admin.ReleaseTerminal(context.Background(), reference); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{root, filepath.Dir(broker.path)} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte("REAL_REMOTE_PTY")) || bytes.Contains(data, []byte("SECOND_REMOTE_INPUT")) {
				t.Fatal("terminal bytes reached durable state")
			}
			if bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString(input.Data))) {
				t.Fatal("encoded terminal input reached durable state")
			}
		}
	}
}

func TestTerminalWrongTargetOwnerEpochAndGrantFailClosed(t *testing.T) {
	broker, admin, worker, _, _, catalog := terminalFixture(t)
	request := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	wrong := request.Reference()
	wrong.Target.Generation = strings.Repeat("b", 64)
	if _, err := admin.CloseTerminal(context.Background(), wrong); err == nil {
		t.Fatal("wrong generation cancelled terminal")
	}
	if err := admin.Call(context.Background(), "/v1/terminal-read", contract.TerminalInput{Reference: request.Reference(), Sequence: 1, Data: []byte("forbidden")}, nil); err == nil {
		t.Fatal("read route accepted input")
	}
	legacy, err := NewClient(worker.Endpoint, worker.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.HTTP.CloseIdleConnections()
	if err := legacy.Call(context.Background(), "/v1/terminal-worker", contract.TerminalExchange{Schema: contract.TerminalVersion, Epoch: catalog.TerminalEpoch, Catalog: catalog}, nil); err == nil {
		t.Fatal("legacy bearer accepted terminal exchange")
	}
	changed := request
	changed.Columns++
	if _, err := admin.OpenTerminal(context.Background(), changed); err == nil {
		t.Fatal("changed open accepted same ID")
	}
	broker.security.Lock()
	if err := broker.security.Unlock(context.Background(), os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ReadTerminal(context.Background(), request.Reference(), 0); err == nil {
		t.Fatal("new OS session read old terminal")
	}
	if _, err := admin.CloseTerminal(context.Background(), request.Reference()); err == nil {
		t.Fatal("new OS session cancelled old terminal")
	}

	restarted, err := NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	if restarted.terminalState().epoch == request.Epoch || len(restarted.terminalState().records) != 0 {
		t.Fatal("broker restored terminal epoch or sessions")
	}
	if _, err := restarted.terminalOperator("/v1/terminal-open", marshal(t, request), operatorAdmission{}); err == nil {
		t.Fatal("old epoch reopened shell")
	}
}

func TestTerminalAuthorityLossAndRestartCloseNativeSessions(t *testing.T) {
	for _, scenario := range []string{"lock", "sleep", "disconnect", "grants", "broker-restart"} {
		t.Run(scenario, func(t *testing.T) {
			broker, admin, worker, root, permissions, catalog := terminalFixture(t)
			transport := worker.HTTP.Transport.(*featureDisconnectTransport)
			request := terminalIntent(t, catalog)
			if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(permissions.Roots[0].Path, "shell-stopped")
			// The worker shell reports its PID into an owner fixture file. No
			// secrets or process arguments are inspected by production cleanup.
			input := []byte("printf '%s' $$ > shell-stopped; printf '__%s__\\n' READY\n")
			if _, err := admin.InputTerminal(context.Background(), contract.TerminalInput{Reference: request.Reference(), Sequence: 1, Data: input}); err != nil {
				t.Fatal(err)
			}
			awaitTerminal(t, admin, request.Reference(), func(view contract.TerminalView) bool { return bytes.Contains(view.Data, []byte("__READY__\r\n")) })
			pidData, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			process, err := terminalTestProcess(pidData)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "lock":
				broker.security.Lock()
			case "sleep":
				platform := broker.security.platform.(*securityFixturePlatform)
				platform.mutex.Lock()
				platform.snapshot.SleepGeneration++
				platform.mutex.Unlock()
				broker.security.Sample(context.Background())
			case "disconnect":
				transport.disconnected.Store(true)
			case "grants":
				permissions.Roots[0].Terminal = false
				permissions.TerminalProfile = nil
				if err := SaveFeaturePermissions(root, permissions); err != nil {
					t.Fatal(err)
				}
			case "broker-restart":
				broker.mutex.Lock()
				broker.terminals = nil // simulate loss of all ephemeral broker state
				broker.mutex.Unlock()
			}
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) && terminalTestAlive(process) {
				time.Sleep(20 * time.Millisecond)
			}
			if terminalTestAlive(process) {
				t.Fatal("native PTY survived authority loss")
			}
		})
	}
}

func TestTerminalReadDoesNotRenewExecuteLease(t *testing.T) {
	broker, admin, _, _, _, catalog := terminalFixture(t)
	request := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	awaitTerminal(t, admin, request.Reference(), func(view contract.TerminalView) bool { return view.State == "running" })
	broker.mutex.Lock()
	expired := time.Now().Add(-time.Second)
	broker.terminals.records[request.ID].admission.deadline = expired
	broker.mutex.Unlock()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		view, err := admin.ReadTerminal(context.Background(), request.Reference(), 0)
		if err != nil {
			t.Fatal(err)
		}
		broker.mutex.Lock()
		unchanged := broker.terminals.records[request.ID].admission.deadline.Equal(expired)
		broker.mutex.Unlock()
		if !unchanged {
			t.Fatal("read scope renewed execute authority")
		}
		if view.State == "closed" {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("read-only activity kept the terminal alive")
}

func TestTerminalWorkerGenerationCannotRestoreOldOpen(t *testing.T) {
	broker, admin, worker, _, _, _, catalog := featureFixture(t)
	catalog.Roots[0].Terminal = true
	broker.mutex.Lock()
	epoch := broker.terminalState().epoch
	broker.mutex.Unlock()
	exchange := contract.TerminalExchange{Schema: contract.TerminalVersion, Epoch: epoch, Catalog: catalog}
	var response contract.TerminalExchangeResult
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, &response); err != nil {
		t.Fatal(err)
	}
	catalog.TerminalEpoch = epoch
	request := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Directives) != 1 || response.Directives[0].Close {
		t.Fatal("initial open not dispatched")
	}
	exchange.Catalog.Target.Generation = strings.Repeat("f", 64)
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Directives) != 0 {
		t.Fatal("worker restart received old open intent")
	}
	view, err := admin.OpenTerminal(context.Background(), request)
	if err != nil || view.State != "unknown" {
		t.Fatal("restart replay did not retain unknown outcome", err)
	}
}

func TestTerminalGrantIsIndependentOfUnknownFeatureMutation(t *testing.T) {
	broker, admin, _, _, _, catalog := terminalFixture(t)
	feature := fileFeatureRequest(t, catalog, "create")
	broker.mutex.Lock()
	broker.features.state.Operations[feature.ID] = contract.FeatureOperation{Request: feature, Status: "unknown", Result: &contract.FeatureResult{Status: "unknown"}}
	broker.mutex.Unlock()
	request := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	awaitTerminal(t, admin, request.Reference(), func(view contract.TerminalView) bool { return view.State == "running" })
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	if broker.features.state.Operations[feature.ID].Status != "unknown" {
		t.Fatal("terminal silently resolved unknown mutation")
	}
}

type terminalSuspendFixture struct{ generation uint64 }

func (clock *terminalSuspendFixture) SuspendGeneration() (uint64, error) {
	return clock.generation, nil
}

func TestTerminalWorkerSuspendCancelsWithoutClearingReceipts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock := &terminalSuspendFixture{generation: 2}
	record := &terminalWorkerRecord{cancel: cancel}
	service := &terminalWorkerService{suspendClock: clock, sleepGeneration: 1, records: map[string]*terminalWorkerRecord{"captured": record}}
	if err := service.checkSuspend(); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() == nil || service.sleepGeneration != 2 || service.records["captured"] != record {
		t.Fatal("worker suspend retained authority or discarded receipt")
	}
}

func TestTerminalExpiredLeaseDiscardsInputAndCannotRevive(t *testing.T) {
	broker, admin, worker, _, _, _, catalog := featureFixture(t)
	catalog.Roots[0].Terminal = true
	broker.mutex.Lock()
	epoch := broker.terminalState().epoch
	broker.mutex.Unlock()
	exchange := contract.TerminalExchange{Schema: contract.TerminalVersion, Epoch: epoch, Catalog: catalog}
	var response contract.TerminalExchangeResult
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, &response); err != nil {
		t.Fatal(err)
	}
	catalog.TerminalEpoch = epoch
	request := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, &response); err != nil {
		t.Fatal(err)
	}
	input := contract.TerminalInput{Reference: request.Reference(), Sequence: 1, Data: []byte("do-not-retain-pending-input")}
	if _, err := admin.InputTerminal(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	broker.terminals.records[request.ID].admission.deadline = time.Now().Add(-time.Second)
	broker.mutex.Unlock()
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Directives) != 1 || !response.Directives[0].Close || response.Directives[0].Input != nil {
		t.Fatal("expired authority delivered bytes")
	}
	if _, err := admin.PulseTerminal(context.Background(), request.Reference()); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/terminal-worker", exchange, &response); err != nil {
		t.Fatal(err)
	}
	if !response.Directives[0].Close {
		t.Fatal("fresh poll resurrected expired session")
	}
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	record := broker.terminals.records[request.ID]
	if record.input != nil || record.pendingDigest != contract.TerminalInputDigest(input.Data) {
		t.Fatal("revocation retained raw input instead of acknowledgement metadata")
	}
}

func TestTerminalLockReclaimsOnlyVerifiedClosedSessions(t *testing.T) {
	broker, admin, _, _, _, catalog := terminalFixture(t)
	request := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	awaitTerminal(t, admin, request.Reference(), func(view contract.TerminalView) bool { return view.State == "running" })
	broker.security.Lock()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		broker.mutex.Lock()
		record := broker.terminals.records[request.ID]
		reclaimed := record.state == "closed" && record.released && record.output == nil
		broker.mutex.Unlock()
		if reclaimed {
			if err := broker.security.Unlock(context.Background(), os.Getpid()); err != nil {
				t.Fatal(err)
			}
			if _, err := admin.ReadTerminal(context.Background(), request.Reference(), 0); err == nil {
				t.Fatal("new OS session read reclaimed private output")
			}
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("verified closed terminal quota survived owner lock")
}

func TestTerminalWorkerUsesRemainingLeaseNotBrokerWallClock(t *testing.T) {
	root := privateTestDirectory(t)
	permissions := FeaturePermissions{Schema: contract.FeatureVersion,
		Roots:           []FeatureRootPermission{{FeatureRoot: contract.FeatureRoot{ID: "project", Name: "Project", Terminal: true}, Path: t.TempDir()}},
		TerminalProfile: &TerminalProfile{Executable: "/bin/sh", Args: []string{"-i"}}}
	if err := SaveFeaturePermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	identity := remoteidentityMustNew(t)
	service := &terminalWorkerService{root: root, instance: strings.Repeat("e", 64), epoch: strings.Repeat("f", 64),
		config: contract.WorkerConfig{Node: "clock-fixture", MachineKey: identity.PublicKey}, records: map[string]*terminalWorkerRecord{}}
	if err := service.configure(permissions, permissionFingerprint(permissions)); err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	catalog := service.catalog
	catalog.TerminalEpoch = service.epoch
	request := terminalIntent(t, catalog)
	// Simulate a broker clock behind the worker. Its authenticated relative
	// lease is current even though its absolute timestamp looks old locally.
	request.Deadline = time.Now().UTC().Add(-time.Minute)
	request.ID, _ = request.Digest()
	directive := contract.TerminalDirective{Open: request, Operator: strings.Repeat("a", 64), LeaseMillis: 1000, Columns: 80, Rows: 24}
	if err := service.apply(context.Background(), directive, time.Now()); err != nil {
		t.Fatal(err)
	}
	if service.records[request.ID].session == nil {
		t.Fatal("worker substituted its wall clock for the broker lease")
	}
}

func TestTerminalUncertainNativeInputClosesWithoutAcknowledgement(t *testing.T) {
	_, admin, _, _, _, catalog := terminalFixture(t)
	request := terminalIntent(t, catalog)
	if _, err := admin.OpenTerminal(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	setup := contract.TerminalInput{Reference: request.Reference(), Sequence: 1, Data: []byte("stty raw -echo; printf '__%s__' READY; sleep 30\n")}
	if _, err := admin.InputTerminal(context.Background(), setup); err != nil {
		t.Fatal(err)
	}
	awaitTerminal(t, admin, request.Reference(), func(view contract.TerminalView) bool {
		return view.AckSequence == 1 && bytes.Contains(view.Data, []byte("__READY__"))
	})
	blocked := contract.TerminalInput{Reference: request.Reference(), Sequence: 2, Data: bytes.Repeat([]byte("x"), contract.MaxTerminalInput)}
	if _, err := admin.InputTerminal(context.Background(), blocked); err != nil {
		t.Fatal(err)
	}
	view := awaitTerminal(t, admin, request.Reference(), func(view contract.TerminalView) bool { return view.State == "closed" })
	if view.AckSequence != 1 || view.Reason != "input-uncertain" {
		t.Fatal("uncertain bytes were acknowledged or reason was hidden")
	}
	view, err := admin.InputTerminal(context.Background(), blocked)
	if err != nil || view.State != "closed" || view.AckSequence != 1 {
		t.Fatal("closed input was retried", err)
	}
}

func TestTerminalQuotaAndOutputBounds(t *testing.T) {
	broker, admin, _, _, _, catalog := terminalFixture(t)
	for index := 0; index < contract.MaxTerminalSessions; index++ {
		if _, err := admin.OpenTerminal(context.Background(), terminalIntent(t, catalog)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := admin.OpenTerminal(context.Background(), terminalIntent(t, catalog)); err == nil {
		t.Fatal("ninth terminal admitted")
	}
	broker.mutex.Lock()
	if len(broker.terminalState().records) != 8 {
		t.Error("terminal quota changed")
	}
	broker.mutex.Unlock()
	var ring terminalBuffer
	chunk := bytes.Repeat([]byte("x"), contract.MaxTerminalChunk)
	for index := 0; index < 12; index++ {
		if err := ring.append(uint64(index*len(chunk)), chunk, false); err != nil {
			t.Fatal(err)
		}
	}
	view, err := ring.read(0)
	if err != nil || !view.Truncated || view.Oldest != 4*contract.MaxTerminalChunk || len(view.Data) != contract.MaxTerminalChunk {
		t.Fatal("terminal ring exceeded bound or lost truncation")
	}
	encoded, _ := json.Marshal(view)
	if len(encoded) > contract.MaxTerminalBody {
		t.Fatal("terminal encoded output exceeded transport bound")
	}
}

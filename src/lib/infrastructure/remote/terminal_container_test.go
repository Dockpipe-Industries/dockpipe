package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

func containerTerminalTestReference() contract.TerminalReference {
	return contract.TerminalReference{Schema: contract.TerminalVersion, ID: strings.Repeat("a", 64), Epoch: strings.Repeat("b", 64),
		Target: contract.FeatureTarget{Node: "worker", Fingerprint: strings.Repeat("c", 64), Generation: strings.Repeat("d", 64)}}
}

func containerTerminalTestReceipt() infrastructure.DockerExecReceipt {
	return infrastructure.DockerExecReceipt{ContainerLifecycleReceipt: infrastructure.ContainerLifecycleReceipt{
		Container: strings.Repeat("e", 64), Provider: strings.Repeat("f", 64), Daemon: strings.Repeat("a", 64)}, ExecID: strings.Repeat("b", 64), Phase: "running"}
}

func TestContainerTerminalGrantIsIndependentAndCatalogExcludesActiveSession(t *testing.T) {
	reference := containerTerminalTestReference()
	request := contract.TerminalOpen{Container: strings.Repeat("e", 64), Shell: "/bin/sh"}
	catalog := contract.FeatureCatalog{Schema: contract.FeatureVersion, Target: reference.Target, DockerRead: true, DockerControl: true,
		Roots: []contract.FeatureRoot{{ID: "project", Name: "Project", Terminal: true}}}
	if terminalSelectionAllowed(catalog, request) {
		t.Fatal("control or generic terminal grant silently authorized exec")
	}
	catalog.DockerExec = true
	if !terminalSelectionAllowed(catalog, request) {
		t.Fatal("explicit container exec grant rejected")
	}
	root := privateTestDirectory(t)
	journal := &containerTerminalJournal{root: root, value: containerTerminalReceipt{Schema: containerTerminalReceiptSchema, Reference: reference}}
	if err := journal.observe(containerTerminalTestReceipt()); err != nil {
		t.Fatal(err)
	}
	published, err := ContainerTerminalRecoveryCatalog(root, catalog)
	if err != nil || len(published.ContainerTerminals) != 0 {
		t.Fatalf("healthy session advertised as uncertain: %+v (%v)", published.ContainerTerminals, err)
	}
	if err := journal.markUncertain(); err != nil {
		t.Fatal(err)
	}
	published, err = ContainerTerminalRecoveryCatalog(root, catalog)
	if err != nil || len(published.ContainerTerminals) != 1 || published.ContainerTerminals[0].Reference != reference {
		t.Fatalf("unknown original reference lost: %+v (%v)", published.ContainerTerminals, err)
	}
	journal.mutex.Lock()
	journal.value.Uncertain = false
	err = journal.saveLocked()
	journal.mutex.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	catalog.Target.Generation = strings.Repeat("9", 64)
	published, err = ContainerTerminalRecoveryCatalog(root, catalog)
	if err != nil || len(published.ContainerTerminals) != 1 {
		t.Fatal("restart orphan reference was hidden")
	}
}

func TestContainerTerminalRecoveryBindsReviewedResourceBeforeEffects(t *testing.T) {
	reference := containerTerminalTestReference()
	resource := containerTerminalTestReceipt()
	resource.Phase, resource.ExecID, resource.Daemon = "planned", "", ""
	root := privateTestDirectory(t)
	journal := &containerTerminalJournal{root: root, value: containerTerminalReceipt{Schema: containerTerminalReceiptSchema, Reference: reference}}
	if err := journal.observe(resource); err != nil {
		t.Fatal(err)
	}
	permissions := FeaturePermissions{Schema: contract.FeatureVersion, DockerRead: true, DockerExec: true}
	payload := terminalRecoveryPayload{Reference: reference, ExpectedContainer: strings.Repeat("8", 64)}
	request, err := NewFeatureRequest(reference.Target, "", "containers.terminal-recover", payload, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTerminalStateRoot(context.Background(), root)
	identity := invocation.Identity{Issuer: "authenticated", Subject: "operator"}
	authorize := func(context.Context, invocation.Binding) error { return nil }
	if _, err := executeTerminalRecoveryFeature(ctx, permissions, request, identity, authorize); err == nil || !strings.Contains(err.Error(), "reviewed") {
		t.Fatalf("reviewed resource drift reached effects: %v", err)
	}
	payload.ExpectedContainer = resource.Container
	request, err = NewFeatureRequest(reference.Target, "", "containers.terminal-recover", payload, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	denied := errors.New("denied")
	if _, err := executeTerminalRecoveryFeature(ctx, permissions, request, identity, func(context.Context, invocation.Binding) error { return denied }); !errors.Is(err, denied) {
		t.Fatalf("recovery bypassed native admission: %v", err)
	}
	value, err := executeTerminalRecoveryFeature(ctx, permissions, request, identity, authorize)
	if err != nil || value == nil {
		t.Fatalf("planned launch without process effects did not reconcile: %v", err)
	}
	payload.Retire = true
	request, err = NewFeatureRequest(reference.Target, "", "containers.terminal-recover", payload, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	permissions.DockerControl = true
	if _, err := executeTerminalRecoveryFeature(ctx, permissions, request, identity, authorize); err == nil {
		t.Fatal("DockerControl silently granted container retirement")
	}
}

func TestContainerTerminalRecoveryClearsOnlyExactBrokerReservation(t *testing.T) {
	reference := containerTerminalTestReference()
	container := strings.Repeat("e", 64)
	record := &terminalRecord{open: contract.TerminalOpen{Schema: reference.Schema, ID: reference.ID, Target: reference.Target, Epoch: reference.Epoch, Container: container},
		state: "unknown", output: &terminalBuffer{}, input: &contract.TerminalInput{Data: []byte("private input")}, ackSequence: 3}
	broker := &Broker{terminals: &terminalBroker{records: map[string]*terminalRecord{reference.ID: record}}}
	payload := terminalRecoveryPayload{Reference: reference, ExpectedContainer: container}
	request, err := NewFeatureRequest(reference.Target, "", "containers.terminal-recover", payload, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	resultPayload, _ := json.Marshal(map[string]any{"schema": containerTerminalReceiptSchema, "reference": reference, "container": container, "reconciled": true})
	result := contract.FeatureResult{Status: "success", Payload: resultPayload}
	if err := broker.completeContainerTerminalRecovery(request, result); err != nil {
		t.Fatal(err)
	}
	if !record.released || record.state != "closed" || record.input != nil || record.ackSequence != 3 {
		t.Fatal("verified recovery lost original closure/input outcome distinction")
	}
	request.Payload = nil
	if err := (&Broker{}).completeContainerTerminalRecovery(request, result); err != nil {
		t.Fatal("metadata-only restart completion needed replayable input")
	}
}

type completedContainerTerminalBackend struct{}

func (*completedContainerTerminalBackend) Read([]byte) (int, error)          { return 0, io.EOF }
func (*completedContainerTerminalBackend) Write(data []byte) (int, error)    { return len(data), nil }
func (*completedContainerTerminalBackend) SetWriteDeadline(time.Time) error  { return nil }
func (*completedContainerTerminalBackend) Resize(terminalsession.Size) error { return nil }
func (*completedContainerTerminalBackend) Wait() (int, error)                { return 0, nil }
func (*completedContainerTerminalBackend) Stop() error                       { return nil }
func (*completedContainerTerminalBackend) Close() error                      { return nil }

func testCompletedContainerTerminalManager(t *testing.T, target contract.FeatureTarget, started *bool) *terminalsession.Manager {
	t.Helper()
	manager, err := terminalsession.NewOwnedProvider(terminalsession.Profile{ID: "fixture", Executable: filepath.Join(t.TempDir(), "provider"), Dir: t.TempDir(), Env: []string{"PATH=/usr/bin:/bin"}},
		terminalsession.Authority{Target: terminalInvocationTarget(target), Identity: invocation.Identity{Issuer: "worker", Subject: "fixture"}, Authorize: func(context.Context, invocation.Binding) error { return nil }},
		func(context.Context, terminalsession.Profile, terminalsession.Size) (terminalsession.Backend, error) {
			*started = true
			return &completedContainerTerminalBackend{}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Shutdown(ctx)
	})
	return manager
}

func TestContainerTerminalOrphansConsumeHostTerminalQuotaAfterRestart(t *testing.T) {
	if !terminalSupported() {
		t.Skip("native terminal platform unavailable")
	}
	root := privateTestDirectory(t)
	reference := containerTerminalTestReference()
	permissions := FeaturePermissions{Schema: contract.FeatureVersion, ProcessRecovery: "host-reboot", Roots: []FeatureRootPermission{{
		FeatureRoot: contract.FeatureRoot{ID: "project", Name: "Project", Terminal: true}, Path: root}}, TerminalProfile: &TerminalProfile{Executable: filepath.Join(root, "shell")}}
	if err := SaveFeaturePermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	started := false
	manager := testCompletedContainerTerminalManager(t, reference.Target, &started)
	service := &terminalWorkerService{root: root, policy: permissionFingerprint(permissions), processRecovery: "host-reboot", catalog: permissions.catalog(reference.Target),
		epoch: reference.Epoch, manager: manager, records: map[string]*terminalWorkerRecord{}}
	var orphaned []contract.TerminalReference
	for index := 0; index < contract.MaxTerminalSessions; index++ {
		orphan := reference
		orphan.ID = fmt.Sprintf("%064x", index+1)
		orphan.Target.Generation = strings.Repeat("9", 64)
		journal := &containerTerminalJournal{root: root, value: containerTerminalReceipt{Schema: containerTerminalReceiptSchema, Reference: orphan, Uncertain: true}}
		if err := journal.observe(containerTerminalTestReceipt()); err != nil {
			t.Fatal(err)
		}
		orphaned = append(orphaned, orphan)
	}
	request := contract.TerminalOpen{Schema: contract.TerminalVersion, Target: reference.Target, Epoch: reference.Epoch, Root: "project", Nonce: strings.Repeat("7", 64),
		Deadline: time.Now().Add(time.Minute), Columns: 80, Rows: 24}
	request.ID, _ = request.Digest()
	directive := contract.TerminalDirective{Open: request, Operator: strings.Repeat("6", 64), LeaseMillis: 5000, Columns: 80, Rows: 24}
	if err := service.apply(context.Background(), directive, time.Now()); err != nil {
		t.Fatal(err)
	}
	if started || service.records[request.ID].session != nil {
		t.Fatal("host terminal bypassed eight retained container exec reservations")
	}
	for _, orphan := range orphaned {
		if err := removeContainerTerminalReceipt(root, orphan); err != nil {
			t.Fatal(err)
		}
	}
	request.Nonce = strings.Repeat("5", 64)
	request.ID, _ = request.Digest()
	directive.Open = request
	if err := service.apply(context.Background(), directive, time.Now()); err != nil {
		t.Fatal(err)
	}
	if !started || service.records[request.ID].session == nil {
		t.Fatal("host launch stayed blocked after orphan reservations were removed")
	}
}

func TestContainerTerminalKnownHostRebootShutdownRecordsVerifiedClosure(t *testing.T) {
	root := privateTestDirectory(t)
	reference := containerTerminalTestReference()
	resource := containerTerminalTestReceipt()
	resource.Completed, resource.Phase = true, "closed"
	journal := &containerTerminalJournal{root: root, value: containerTerminalReceipt{Schema: containerTerminalReceiptSchema, Reference: reference}}
	if err := journal.observe(resource); err != nil {
		t.Fatal(err)
	}
	started := false
	manager := testCompletedContainerTerminalManager(t, reference.Target, &started)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := manager.OpenAs(ctx, terminalsession.Size{Columns: 80, Rows: 24}, 5*time.Second,
		invocation.Identity{Issuer: "operator", Subject: "fixture"}, func(context.Context, invocation.Binding) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	wait, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if result, err := session.Wait(wait); err != nil || result.CleanupError != nil {
		t.Fatalf("fixture closure failed: %+v (%v)", result, err)
	}
	_, scope := process.WithScopeMode(ctx, reference.ID, "host-reboot", journal.observeScope)
	service := &terminalWorkerService{processRecovery: "host-reboot", records: map[string]*terminalWorkerRecord{reference.ID: {
		open: contract.TerminalOpen{ID: reference.ID}, session: session, manager: manager, cancel: cancel, scope: scope, journal: journal}}}
	if err := service.cancelSessions(); err != nil {
		t.Fatal(err)
	}
	journal.mutex.Lock()
	verified := journal.value.ClientStopped
	journal.mutex.Unlock()
	if !verified {
		t.Fatal("known relay/exec closure was relabelled unknown under host-reboot policy")
	}
	if _, err := os.Stat(containerTerminalReceiptPath(root, reference.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("verified normal shutdown retained an orphan receipt: %v", err)
	}
}

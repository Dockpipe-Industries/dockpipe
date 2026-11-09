package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/process"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

func hostTerminalTestReceipt(t *testing.T) hostTerminalReceipt {
	t.Helper()
	boot, err := process.HostBootIdentity()
	if err != nil {
		t.Skip("native host boot identity unavailable")
	}
	return hostTerminalReceipt{Schema: contract.HostTerminalReceiptVersion, Reference: containerTerminalTestReference(), Root: "project", Phase: "running",
		Scope: process.ScopeReference{Mode: "host-reboot", Boot: boot}, Uncertain: true}
}

func hostTerminalTestService(t *testing.T, root string, provider terminalsession.ProviderFactory) (*terminalWorkerService, contract.TerminalDirective) {
	t.Helper()
	reference := containerTerminalTestReference()
	permissions := FeaturePermissions{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{{
		FeatureRoot: contract.FeatureRoot{ID: "project", Name: "Project", Terminal: true}, Path: root}},
		TerminalProfile: &TerminalProfile{Executable: filepath.Join(root, "secret-command-marker")}}
	if err := SaveFeaturePermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	manager, err := terminalsession.NewOwnedProvider(terminalsession.Profile{ID: "fixture", Executable: permissions.TerminalProfile.Executable, Dir: root, Env: []string{"SECRET=private-env-marker"}},
		terminalsession.Authority{Target: terminalInvocationTarget(reference.Target), Identity: invocation.Identity{Issuer: "worker", Subject: "fixture"}, Authorize: func(context.Context, invocation.Binding) error { return nil }}, provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Shutdown(ctx)
	})
	service := &terminalWorkerService{root: root, policy: permissionFingerprint(permissions), catalog: permissions.catalog(reference.Target), epoch: reference.Epoch,
		manager: manager, records: map[string]*terminalWorkerRecord{}}
	request := contract.TerminalOpen{Schema: contract.TerminalVersion, Target: reference.Target, Epoch: reference.Epoch, Root: "project", Nonce: strings.Repeat("7", 64),
		Deadline: time.Now().Add(time.Minute), Columns: 80, Rows: 24}
	request.ID, _ = request.Digest()
	return service, contract.TerminalDirective{Open: request, Operator: strings.Repeat("6", 64), LeaseMillis: 5000, Columns: 80, Rows: 24}
}

func TestHostTerminalBootReceiptPrecedesNativeLaunchAndPersistenceFailurePreventsIt(t *testing.T) {
	if !terminalSupported() {
		t.Skip("native remote host terminal unavailable")
	}
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			root := privateTestDirectory(t)
			started := false
			var reference contract.TerminalReference
			service, directive := hostTerminalTestService(t, root, func(context.Context, terminalsession.Profile, terminalsession.Size) (terminalsession.Backend, error) {
				started = true
				receipt, err := loadHostTerminalReceipt(root, reference)
				if err != nil || receipt.Phase != "start-requested" || receipt.Scope.Mode != "host-reboot" || receipt.Scope.Validate() != nil {
					t.Fatal("native PTY started without its exact durable boot receipt", err)
				}
				return &completedContainerTerminalBackend{}, nil
			})
			reference = directive.Open.Reference()
			if failure {
				if err := os.WriteFile(filepath.Join(root, "host-terminals"), []byte("storage-failure"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := service.apply(context.Background(), directive, time.Now()); err != nil {
				t.Fatal(err)
			}
			if started == failure {
				t.Fatal("receipt persistence did not gate native launch")
			}
			if !failure {
				data, err := os.ReadFile(hostTerminalReceiptPath(root, reference.ID))
				if err != nil {
					t.Fatal(err)
				}
				for _, private := range []string{"secret-command-marker", "private-env-marker", root, directive.Open.Nonce} {
					if strings.Contains(string(data), private) {
						t.Fatal("host receipt retained profile/input content", private)
					}
				}
			}
		})
	}
}

func TestHostTerminalRecoveryRequiresCapturedBootExactReferenceAndIndependentRootGrant(t *testing.T) {
	root := privateTestDirectory(t)
	original := hostTerminalTestReceipt(t)
	if err := saveHostTerminalReceipt(root, original); err != nil {
		t.Fatal(err)
	}
	current := original.Reference.Target
	current.Generation = strings.Repeat("9", 64)
	permissions := FeaturePermissions{Schema: contract.FeatureVersion, Roots: []FeatureRootPermission{{FeatureRoot: contract.FeatureRoot{ID: original.Root, Name: "Project", Terminal: true}, Path: root}}}
	request, err := NewFeatureRequest(current, original.Root, "terminal.host-recover", hostTerminalRecoveryPayload{Reference: original.Reference}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTerminalStateRoot(context.Background(), root)
	identity := invocation.Identity{Issuer: "paired-broker", Subject: "operator"}
	authorize := func(context.Context, invocation.Binding) error { return nil }
	if _, err := executeHostTerminalRecovery(ctx, permissions, request, identity, authorize); !errors.Is(err, process.ErrRebootRequired) {
		t.Fatal("same-boot host session was treated as cleaned", err)
	}
	denied := errors.New("denied before receipt access")
	if _, err := executeHostTerminalRecovery(WithTerminalStateRoot(context.Background(), filepath.Join(root, "missing")), permissions, request, identity,
		func(context.Context, invocation.Binding) error { return denied }); !errors.Is(err, denied) {
		t.Fatal("host recovery accessed storage before native authorization", err)
	}
	prior := original
	prior.Scope.Boot = "00000000-0000-0000-0000-000000000000"
	if err := saveHostTerminalReceipt(root, prior); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"root", "identity", "reference", "grant", "legacy"} {
		t.Run(scenario, func(t *testing.T) {
			selected := request
			grants := permissions
			switch scenario {
			case "root":
				selected.Root = "other"
				grants.Roots = []FeatureRootPermission{{FeatureRoot: contract.FeatureRoot{ID: "other", Name: "Other", Terminal: true}, Path: root}}
			case "identity":
				selected.Target.Fingerprint = strings.Repeat("8", 64)
			case "reference":
				wrong := prior.Reference
				wrong.Epoch = strings.Repeat("8", 64)
				selected.Payload, _ = json.Marshal(hostTerminalRecoveryPayload{Reference: wrong})
			case "grant":
				grants.Roots = []FeatureRootPermission{{FeatureRoot: contract.FeatureRoot{ID: original.Root, Name: "Project", FilesRead: true, FilesWrite: true}, Path: root}}
				grants.DockerRead, grants.DockerExec, grants.DockerRecovery = true, true, true
			case "legacy":
				ctx = WithTerminalStateRoot(context.Background(), privateTestDirectory(t))
			}
			if _, err := executeHostTerminalRecovery(ctx, grants, selected, identity, authorize); err == nil {
				t.Fatal("host recovery accepted missing or changed ownership evidence")
			}
		})
	}
	ctx = WithTerminalStateRoot(context.Background(), root)
	value, err := executeHostTerminalRecovery(ctx, permissions, request, identity, authorize)
	proof, ok := value.(contract.HostTerminalRecovery)
	if err != nil || !ok || proof.Validate() != nil || proof.Reference != original.Reference || proof.Root != original.Root {
		t.Fatal("prior-boot host proof lost its original reference/root", err)
	}
	if _, err := os.Stat(hostTerminalReceiptPath(root, original.Reference.ID)); err != nil {
		t.Fatal("host receipt was discarded before broker acknowledgment", err)
	}
}

func TestHostTerminalRecoveryReleasesOnlyExactBrokerQuotaWithoutInputAcknowledgment(t *testing.T) {
	original := hostTerminalTestReceipt(t)
	current := original.Reference.Target
	current.Generation = strings.Repeat("9", 64)
	request, err := NewFeatureRequest(current, original.Root, "terminal.host-recover", hostTerminalRecoveryPayload{Reference: original.Reference}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	input := []byte("unacknowledged-input")
	record := &terminalRecord{open: contract.TerminalOpen{Schema: original.Reference.Schema, ID: original.Reference.ID, Epoch: original.Reference.Epoch, Target: original.Reference.Target, Root: original.Root},
		state: "unknown", input: &contract.TerminalInput{Data: input}, output: &terminalBuffer{}, ackSequence: 4, pendingSequence: 5}
	broker := &Broker{terminals: &terminalBroker{records: map[string]*terminalRecord{original.Reference.ID: record}}}
	if err := broker.revokeHostTerminalRecovery(request); err != nil || !record.close || record.released || record.input != nil || record.ackSequence != 4 {
		t.Fatal("recovery did not revoke only the original transport before proof", err)
	}
	proof := contract.HostTerminalRecovery{Schema: contract.HostTerminalReceiptVersion, Reference: original.Reference, Root: original.Root, Reconciled: true}
	wrong := proof
	wrong.Reference.Epoch = strings.Repeat("8", 64)
	if err := broker.completeHostTerminalRecovery(request, wrong); err == nil || record.released {
		t.Fatal("mismatched host proof released a quota reservation")
	}
	if err := broker.completeHostTerminalRecovery(request, proof); err != nil || !record.released || record.state != "closed" || record.ackSequence != 4 || record.pendingSequence != 5 {
		t.Fatal("verified host recovery changed input success or another transport", err)
	}
}

func TestHostAndContainerOrphansShareOneRetainedSessionBudget(t *testing.T) {
	root := privateTestDirectory(t)
	base := hostTerminalTestReceipt(t)
	for index := 0; index < contract.MaxTerminalSessions/2; index++ {
		host := base
		host.Reference.ID = fmt.Sprintf("%064x", index+1)
		if err := saveHostTerminalReceipt(root, host); err != nil {
			t.Fatal(err)
		}
		container := containerTerminalReceipt{Schema: containerTerminalReceiptSchema, Reference: base.Reference, Resource: containerTerminalTestReceipt(), Uncertain: true}
		container.Reference.ID = fmt.Sprintf("%064x", index+33)
		if err := saveContainerTerminalReceipt(root, container); err != nil {
			t.Fatal(err)
		}
	}
	ninth := base
	ninth.Reference.ID = fmt.Sprintf("%064x", 100)
	if err := saveHostTerminalReceipt(root, ninth); err == nil {
		t.Fatal("host receipts admitted an independent ninth unknown session")
	}
	started := false
	service, directive := hostTerminalTestService(t, root, func(context.Context, terminalsession.Profile, terminalsession.Size) (terminalsession.Backend, error) {
		started = true
		return &completedContainerTerminalBackend{}, nil
	})
	if err := service.apply(context.Background(), directive, time.Now()); err != nil {
		t.Fatal(err)
	}
	if started {
		t.Fatal("durable host/container orphans did not block a ninth host PTY")
	}
}

func TestHostTerminalRecoveryMetadataSurvivesFeatureWorkerRestart(t *testing.T) {
	root := privateTestDirectory(t)
	original := hostTerminalTestReceipt(t)
	current := original.Reference.Target
	current.Generation = strings.Repeat("9", 64)
	request, err := NewFeatureRequest(current, original.Root, "terminal.host-recover", hostTerminalRecoveryPayload{Reference: original.Reference}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	proof := contract.HostTerminalRecovery{Schema: contract.HostTerminalReceiptVersion, Reference: original.Reference, Root: original.Root, Reconciled: true}
	payload, _ := json.Marshal(proof)
	result := contract.FeatureResult{Status: "success", Payload: payload}
	journal := featureWorkerJournal{Schema: contract.FeatureVersion, Records: map[string]featureWorkerRecord{request.ID: {Request: request, Result: &result, HostRecovery: &proof}}}
	if err := writeFeatureJournal(root, journal); err != nil {
		t.Fatal(err)
	}
	recovered, err := readFeatureJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	completion := featureCompletionFor(recovered.Records[request.ID])
	if !completion.ReceiptOnly || len(completion.Result.Payload) != 0 || completion.HostRecovery == nil || *completion.HostRecovery != proof {
		t.Fatal("restart lost the content-free original host cleanup proof")
	}
	operation := contract.FeatureOperation{Request: request, Status: "unknown"}
	operation.Request.Payload, operation.Request.Nonce = nil, ""
	broker := &Broker{}
	if err := broker.acceptHostTerminalCompletion(&operation, completion); err != nil || operation.HostRecovery == nil {
		t.Fatal("metadata-only completion required original replayable content", err)
	}
}

func TestHostTerminalReceiptPublicationIsSerializedAndUnexpectedFilesFailClosed(t *testing.T) {
	root := privateTestDirectory(t)
	receipt := hostTerminalTestReceipt(t)
	if err := saveHostTerminalReceipt(root, receipt); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockContainerTerminalStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	staged, err := os.CreateTemp(filepath.Join(root, "host-terminals"), ".dockpipe-state-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	defer os.Remove(staged.Name())
	finished := make(chan error, 1)
	go func() {
		_, err := hostTerminalReceipts(root)
		finished <- err
	}()
	select {
	case err := <-finished:
		t.Fatalf("host catalog crossed private publication ownership: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if err := os.Remove(staged.Name()); err != nil {
		t.Fatal(err)
	}
	unlock()
	unlock = nil
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("host catalog did not resume after receipt publication")
	}
	if err := os.WriteFile(filepath.Join(root, "host-terminals", "unexpected.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := hostTerminalReceipts(root); err == nil {
		t.Fatal("corrupt/unknown host receipt was silently skipped")
	}
}

func TestTerminalHeartbeatPreservesHostOrphansOnlyForExactWorkerTarget(t *testing.T) {
	broker, _, worker, _, _, _, catalog := featureFixture(t)
	broker.mutex.Lock()
	epoch := broker.terminalState().epoch
	broker.mutex.Unlock()
	reference := contract.TerminalReference{Schema: contract.TerminalVersion, ID: strings.Repeat("1", 64), Epoch: epoch, Target: catalog.Target}
	catalog.HostTerminals = []contract.HostTerminalReference{{Reference: reference, Root: "project", Phase: "running"}}
	if err := worker.Call(context.Background(), "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	heartbeat := catalog
	heartbeat.HostTerminals = nil
	if err := worker.Call(context.Background(), "/v1/terminal-worker", contract.TerminalExchange{Schema: contract.TerminalVersion, Epoch: epoch, Catalog: heartbeat}, nil); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	retained := broker.features.catalogs[catalog.Target.Node]
	broker.mutex.Unlock()
	if len(retained.HostTerminals) != 1 || retained.HostTerminals[0].Reference != reference {
		t.Fatal("terminal heartbeat erased the feature worker's host orphan discovery")
	}
	heartbeat.Target.Generation = strings.Repeat("9", 64)
	if err := worker.Call(context.Background(), "/v1/terminal-worker", contract.TerminalExchange{Schema: contract.TerminalVersion, Epoch: epoch, Catalog: heartbeat}, nil); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	retained = broker.features.catalogs[catalog.Target.Node]
	broker.mutex.Unlock()
	if len(retained.HostTerminals) != 0 {
		t.Fatal("a different worker generation inherited an old orphan publication")
	}
}

func TestHostTerminalRecoveryAcrossBrokerEpochRetainsProofUntilAcknowledged(t *testing.T) {
	broker, admin, worker, config, root, permissions, originalCatalog := featureFixture(t)
	permissions.Roots[0].Terminal = true
	permissions.TerminalProfile = &TerminalProfile{Executable: "/bin/sh"}
	if err := SaveFeaturePermissions(root, permissions); err != nil {
		t.Fatal(err)
	}
	original := hostTerminalTestReceipt(t)
	original.Reference.Target = originalCatalog.Target
	original.Reference.Epoch = strings.Repeat("8", 64)
	original.Scope.Boot = "00000000-0000-0000-0000-000000000000"
	if err := saveHostTerminalReceipt(root, original); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	done := make(chan error, 1)
	go func() { done <- RunFeatureWorker(ctx, root, config, worker) }()
	defer func() { cancel(); <-done }()
	var selected contract.FeatureCatalog
	for ctx.Err() == nil {
		catalogs, err := admin.FeatureCatalog(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(catalogs) == 1 && len(catalogs[0].HostTerminals) == 1 {
			selected = catalogs[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if selected.Target == original.Reference.Target || len(selected.HostTerminals) != 1 || selected.HostTerminals[0].Reference != original.Reference || selected.TerminalEpoch == original.Reference.Epoch {
		t.Fatal("new worker/broker epoch lost the exact old host recovery reference")
	}
	request, err := NewFeatureRequest(selected.Target, original.Root, "terminal.host-recover", hostTerminalRecoveryPayload{Reference: original.Reference}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.SubmitFeature(ctx, request); err != nil {
		t.Fatal(err)
	}
	var operation contract.FeatureOperation
	for ctx.Err() == nil {
		operation, err = admin.FeatureStatus(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if operation.Status == "success" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if operation.Status != "success" || operation.HostRecovery == nil || operation.HostRecovery.Reference != original.Reference || operation.HostRecovery.Root != original.Root {
		t.Fatalf("old-epoch proof was not validated and retained: %+v", operation)
	}
	for ctx.Err() == nil {
		if _, err := os.Stat(hostTerminalReceiptPath(root, original.Reference.ID)); os.IsNotExist(err) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(hostTerminalReceiptPath(root, original.Reference.ID)); !os.IsNotExist(err) {
		t.Fatal("acknowledged host cleanup retained an orphan quota record", err)
	}
	restarted, err := NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	retained := restarted.features.state.Operations[request.ID]
	if retained.HostRecovery == nil || *retained.HostRecovery != *operation.HostRecovery || !retained.ReceiptOnly || len(retained.Request.Payload) != 0 {
		t.Fatal("broker restart lost the metadata-only original host proof")
	}
}

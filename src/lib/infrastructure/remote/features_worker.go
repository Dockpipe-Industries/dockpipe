package remote

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type featureWorkerRecord struct {
	Request          contract.FeatureRequest `json:"request"`
	Result           *contract.FeatureResult `json:"result,omitempty"`
	ReceiptOnly      bool
	ResultDigest     string
	Reconciled       bool
	Supervised       bool
	Scope            *process.ScopeReference
	ExecutionStopped bool
	Docker           *infrastructure.ContainerLifecycleReceipt
	HostRecovery     *contract.HostTerminalRecovery
}

type featureWorkerJournal struct {
	Schema  string                         `json:"schema"`
	Records map[string]featureWorkerRecord `json:"records"`
}

func readFeatureJournal(root string) (featureWorkerJournal, error) {
	journal := featureWorkerJournal{Schema: contract.FeatureVersion, Records: map[string]featureWorkerRecord{}}
	ledger := featureLedger{Schema: contract.FeatureVersion, Records: map[string]featureReceipt{}}
	err := readPrivateBounded(filepath.Join(root, "feature-worker.json"), &ledger, contract.MaxFeatureStorage)
	if err != nil && !os.IsNotExist(err) {
		return journal, err
	}
	if ledger.Schema != contract.FeatureVersion || ledger.Records == nil || len(ledger.Records) > contract.MaxJobs {
		return journal, errors.New("invalid worker feature journal")
	}
	for id, receipt := range ledger.Records {
		if id != receipt.ID || receipt.validate() != nil {
			return journal, errors.New("invalid worker feature intent")
		}
		result := recoveredFeatureResult(receipt.Status)
		journal.Records[id] = featureWorkerRecord{Request: receipt.requestMetadata(), Result: &result, ReceiptOnly: true, ResultDigest: receipt.ResultDigest, Reconciled: receipt.Reconciled,
			Supervised: receipt.Supervised, Scope: receipt.Scope, ExecutionStopped: receipt.ExecutionStopped, Docker: receipt.Docker, HostRecovery: receipt.HostRecovery}
	}
	return journal, nil
}

func writeFeatureJournal(root string, journal featureWorkerJournal) error {
	ledger := featureLedger{Schema: contract.FeatureVersion, Records: map[string]featureReceipt{}}
	for id, record := range journal.Records {
		if contract.FeatureAccess(record.Request.Action) == "read" {
			continue
		}
		status := "running"
		if record.Result != nil {
			status = record.Result.Status
		}
		digest := record.ResultDigest
		if !record.ReceiptOnly {
			digest = featureResultDigest(record.Result)
		}
		receipt := receiptFor(record.Request, status, record.Reconciled, digest)
		receipt.RecoveryRequested = record.Reconciled
		receipt.Reconciled = record.Reconciled
		receipt.Supervised = record.Supervised
		receipt.Scope = record.Scope
		receipt.ExecutionStopped = record.ExecutionStopped
		receipt.Docker = record.Docker
		receipt.HostRecovery = record.HostRecovery
		ledger.Records[id] = receipt
	}
	if len(ledger.Records) == 0 {
		if _, err := os.Lstat(filepath.Join(root, "feature-worker.json")); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
	}
	data, err := json.Marshal(ledger)
	if err != nil {
		return err
	}
	if len(data) > contract.MaxFeatureStorage {
		return errors.New("worker feature journal budget exhausted")
	}
	return WritePrivate(filepath.Join(root, "feature-worker.json"), ledger)
}

func permissionFingerprint(permissions FeaturePermissions) string {
	encoded, _ := json.Marshal(permissions)
	return Hash(string(encoded))
}

// RunFeatureWorker is owned by RunWorker, independent of workflow execution and
// monitoring. It sends only outbound authenticated HTTPS. One active operation
// serializes worker roots; a permission change creates a new target generation.
func RunFeatureWorker(ctx context.Context, root string, config contract.WorkerConfig, client *Client) error {
	instance, err := Secret()
	if err != nil {
		return err
	}
	return runFeatureWorkerWithInstance(ctx, root, config, client, instance)
}

func runFeatureWorkerWithInstance(ctx context.Context, root string, config contract.WorkerConfig, client *Client, instance string) error {
	if !config.Features || client == nil || client.Identity == nil || client.BrokerKey == "" || client.Identity.PublicKey != config.MachineKey {
		return errors.New("feature worker requires explicit opt-in and paired machine keys")
	}
	if err := PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := lockFeatureWorker(root)
	if err != nil {
		return err
	}
	defer unlock()
	journal, err := readFeatureJournal(root)
	if err != nil {
		return err
	}
	// Persist recovered unknown states before contacting the broker. Restart is
	// never permission to execute an intent again.
	if err := writeFeatureJournal(root, journal); err != nil {
		return err
	}
	for id, record := range journal.Records {
		if record.Request.Target.Node != config.Node || record.Request.Target.Fingerprint != remoteidentity.Fingerprint(config.MachineKey) {
			return errors.New("worker feature journal belongs to a different machine")
		}
		completion := featureCompletionFor(record)
		var acknowledgement contract.FeatureResultAck
		if err := client.callWithRetry(ctx, "/v1/features-result", completion, &acknowledgement); err != nil {
			return err
		}
		if acknowledgement.Reference != completion.Reference {
			return errors.New("feature receipt acknowledgement identity changed")
		}
		if record.HostRecovery != nil && record.Result.Status == "success" {
			if err := removeHostTerminalReceipt(root, record.HostRecovery.Reference); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if acknowledgement.Expired && (record.Result.Status != "unknown" || record.Reconciled) {
			delete(journal.Records, id)
		}
	}
	if err := writeFeatureJournal(root, journal); err != nil {
		return err
	}
	retainedUntil := map[string]time.Time{}
	for id := range journal.Records {
		retainedUntil[id] = time.Now().Add(contract.FeatureLifetime)
	}
	for ctx.Err() == nil {
		permissions, err := loadFeaturePermissions(root)
		if err != nil {
			if !pause(ctx, time.Second) {
				break
			}
			continue
		}
		policy := permissionFingerprint(permissions)
		target := contract.FeatureTarget{Node: config.Node, Fingerprint: remoteidentity.Fingerprint(config.MachineKey), Generation: Hash(instance + policy)}
		catalog := permissions.catalog(target)
		catalog.ProcessRecovery = permissions.ProcessRecovery
		if catalog.ProcessRecovery == "" {
			catalog.ProcessRecovery = process.RecoveryMode()
		}
		if containerTerminalSupported() {
			catalog, err = ContainerTerminalRecoveryCatalog(root, catalog)
			if err != nil {
				return err
			}
		}
		catalog, err = HostTerminalRecoveryCatalog(root, catalog)
		if err != nil {
			return err
		}
		if err := catalog.Validate(); err != nil {
			return err
		}
		var assignment *contract.FeatureAssignment
		started := time.Now()
		if err := client.Call(ctx, "/v1/features-next", catalog, &assignment); err != nil {
			if !pause(ctx, time.Second) {
				break
			}
			continue
		}
		if assignment == nil {
			// Half-second idle polling keeps interactive file operations responsive
			// without holding a broker request or mutex open between assignments.
			if !pause(ctx, 500*time.Millisecond) {
				break
			}
			continue
		}
		if assignment.Reconcile {
			if err := validateFileReconciliation(*assignment, target); err != nil {
				return err
			}
			record, exists := journal.Records[assignment.Request.ID]
			failure := ""
			if !exists && assignment.Request.Target != target {
				failure = "original-receipt-unavailable"
			}
			if exists && (record.Result == nil || record.Request.Reference() != assignment.Request.Reference() ||
				record.Request.Root != assignment.Request.Root || record.Request.Action != assignment.Request.Action ||
				!record.Request.Deadline.Equal(assignment.Request.Deadline)) {
				failure = "original-receipt-unavailable"
			}
			if failure == "" {
				failure = recoverWorkerFeature(ctx, client, root, target, permissions, *assignment, record, exists)
			}
			if failure != "" {
				if err := client.callWithRetry(ctx, "/v1/features-recovery-failed", featureRecoveryFailure{Reference: assignment.Request.Reference(), Reason: failure}, nil); err != nil {
					return err
				}
				continue
			}
			proof := contract.FeatureReconciliation{Reference: assignment.Request.Reference(), Worker: target, Resource: assignment.RecoveryResource}
			var acknowledgement contract.FeatureResultAck
			if err := client.callWithRetry(ctx, "/v1/features-reconciled", proof, &acknowledgement); err != nil {
				return err
			}
			if acknowledgement.Reference != proof.Reference {
				return errors.New("feature reconciliation acknowledgement identity changed")
			}
			if exists {
				record.Request = assignment.Request
				result := recoveredFeatureResult("unknown")
				record.Result = &result
				record.ReceiptOnly = true
				record.Reconciled = true
				journal.Records[proof.Reference.ID] = record
				if err := writeFeatureJournal(root, journal); err != nil {
					return err
				}
			}
			continue
		}
		if assignment.Request.Validate() != nil || assignment.Request.Target != target || !validMachineNonce(assignment.Operator) || !featureCatalogAllows(catalog, assignment.Request) {
			return errors.New("broker returned a feature outside this worker's identity or grants")
		}
		deadline, err := featureLeaseDeadline(started, assignment.LeaseMillis)
		if err != nil {
			return err
		}
		record, exists := journal.Records[assignment.Request.ID]
		readOnly := contract.FeatureAccess(assignment.Request.Action) == "read"
		if !exists {
			for id, previous := range journal.Records {
				if previous.Result != nil && (previous.Result.Status != "unknown" || previous.Reconciled) && time.Now().After(previous.Request.Deadline) && time.Now().After(retainedUntil[id]) {
					delete(journal.Records, id)
					delete(retainedUntil, id)
				}
			}
			if !readOnly && len(journal.Records) >= contract.MaxJobs {
				rejection := featureWorkerRecord{
					Request: assignment.Request,
					Result: &contract.FeatureResult{
						Status: "failure",
						Error:  "Worker mutation journal is full; no native operation started.",
					},
				}
				if err := acknowledgeFeatureResult(ctx, client, rejection); err != nil {
					return err
				}
				continue
			}
			record = featureWorkerRecord{Request: assignment.Request}
			record.Supervised = !readOnly &&
				(strings.HasPrefix(record.Request.Action, "git.") || strings.HasPrefix(record.Request.Action, "containers."))
			journal.Records[record.Request.ID] = record
			retainedUntil[record.Request.ID] = time.Now().Add(contract.FeatureLifetime)
			if !readOnly {
				if err := writeFeatureJournal(root, journal); err != nil {
					return err
				}
			}
			executionContext := WithTerminalStateRoot(ctx, root)
			executionContext = context.WithValue(executionContext, hostRecoveryObserverKey{}, func(proof contract.HostTerminalRecovery) error {
				record.HostRecovery = &proof
				payload, err := json.Marshal(proof)
				if err != nil {
					return err
				}
				completed := record
				completed.Result = &contract.FeatureResult{Status: "success", Payload: payload}
				// The boot proof and completed metadata are one atomic receipt.
				// Restart can report it without retaining or replaying a payload.
				journal.Records[record.Request.ID] = completed
				return writeFeatureJournal(root, journal)
			})
			var scope *process.Scope
			if record.Supervised {
				executionContext, scope = process.WithScopeMode(executionContext, record.Request.ID, catalog.ProcessRecovery, func(reference process.ScopeReference) error {
					record.Scope = &reference
					journal.Records[record.Request.ID] = record
					return writeFeatureJournal(root, journal)
				})
				executionContext = infrastructure.WithContainerLifecycle(executionContext, func(receipt infrastructure.ContainerLifecycleReceipt) error {
					record.Docker = &receipt
					journal.Records[record.Request.ID] = record
					return writeFeatureJournal(root, journal)
				})
			}
			result := executeLeasedFeature(executionContext, client, root, permissions, policy, catalog, *assignment, deadline)
			if scope != nil {
				cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				err := scope.Stop(cleanup)
				cancel()
				record.ExecutionStopped = err == nil
				if err != nil && !(errors.Is(err, process.ErrRebootRequired) && (result.Status == "success" || result.Status == "failure")) {
					result = contract.FeatureResult{Status: "unknown", Error: "Native operation cleanup is unconfirmed; reconcile the captured process scope before new work."}
				}
			}
			if record.Docker != nil && !record.Docker.Completed && result.Status != "success" {
				result = contract.FeatureResult{Status: "unknown", Error: "Docker daemon completion is uncertain; native client exit does not cancel daemon work. Inspect or explicitly retire the captured container."}
			}
			record.Result = &result
			journal.Records[record.Request.ID] = record
			if !readOnly {
				if err := writeFeatureJournal(root, journal); err != nil {
					return err
				}
			}
		}
		if record.Result == nil {
			return errors.New("unfinished feature intent cannot be replayed")
		}
		if err := acknowledgeFeatureResult(ctx, client, record); err != nil {
			return err
		}
		if readOnly {
			// The broker already holds this read result. Keep the mutation
			// journal budget available for work that needs durable receipts.
			delete(journal.Records, record.Request.ID)
			delete(retainedUntil, record.Request.ID)
			continue
		}
		if record.HostRecovery != nil && record.Result.Status == "success" {
			if err := removeHostTerminalReceipt(root, record.HostRecovery.Reference); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		// The broker acknowledged delivery. Keep only the mutation identity/status
		// needed to prevent replay; plaintext output is not a worker cache.
		digest := featureResultDigest(record.Result)
		if record.ReceiptOnly {
			digest = record.ResultDigest
		}
		receipt := receiptFor(record.Request, record.Result.Status, false, digest)
		metadata := recoveredFeatureResult(receipt.Status)
		record.Request = receipt.requestMetadata()
		record.Result = &metadata
		record.ReceiptOnly = true
		record.ResultDigest = digest
		journal.Records[record.Request.ID] = record
	}
	return ctx.Err()
}

func acknowledgeFeatureResult(ctx context.Context, client *Client, record featureWorkerRecord) error {
	completion := featureCompletionFor(record)
	var acknowledgement contract.FeatureResultAck
	if err := client.callWithRetry(ctx, "/v1/features-result", completion, &acknowledgement); err != nil {
		return err
	}
	if acknowledgement.Reference != completion.Reference {
		return errors.New("feature result acknowledgement identity changed")
	}
	return nil
}

func featureCompletionFor(record featureWorkerRecord) contract.FeatureCompletion {
	digest := record.ResultDigest
	if !record.ReceiptOnly {
		digest = featureResultDigest(record.Result)
	}
	resource := ""
	if record.Docker != nil {
		resource = record.Docker.Container
	}
	var hostRecovery *contract.HostTerminalRecovery
	if record.Result.Status == "success" {
		hostRecovery = record.HostRecovery
	}
	return contract.FeatureCompletion{Reference: record.Request.Reference(), Result: *record.Result, RecoveryResource: resource, HostRecovery: hostRecovery,
		ReceiptOnly: record.ReceiptOnly, ResultDigest: digest, Deadline: record.Request.Deadline, Reconciled: record.Reconciled}
}

func featureLeaseDeadline(started time.Time, milliseconds int64) (time.Time, error) {
	if milliseconds <= 0 || milliseconds > contract.FeatureLease.Milliseconds() {
		return time.Time{}, errors.New("invalid feature lease duration")
	}
	deadline := started.Add(time.Duration(milliseconds) * time.Millisecond)
	if !time.Now().Before(deadline) {
		return time.Time{}, errors.New("feature lease expired in transit")
	}
	return deadline, nil
}

func executeLeasedFeature(parent context.Context, client *Client, root string, permissions FeaturePermissions, policy string, catalog contract.FeatureCatalog, assignment contract.FeatureAssignment, deadline time.Time) contract.FeatureResult {
	return executeLeasedFeatureUsing(parent, client, root, permissions, policy, catalog, assignment, deadline, executeFeature)
}

type featureExecutor func(context.Context, FeaturePermissions, contract.FeatureRequest, invocation.Identity, invocation.Authorize) contract.FeatureResult

func executeLeasedFeatureUsing(parent context.Context, client *Client, root string, permissions FeaturePermissions, policy string, catalog contract.FeatureCatalog, assignment contract.FeatureAssignment, deadline time.Time, execute featureExecutor) contract.FeatureResult {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var leaseMutex sync.Mutex
	leaseDeadline := deadline
	timer := time.AfterFunc(time.Until(deadline), cancel)
	defer timer.Stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for pause(ctx, time.Second) {
			current, err := loadFeaturePermissions(root)
			if err != nil || permissionFingerprint(current) != policy {
				cancel()
				return
			}
			leaseMutex.Lock()
			until := leaseDeadline
			leaseMutex.Unlock()
			pulseContext, stop := context.WithDeadline(ctx, until)
			// Refresh the worker-owned catalog only, never operator authorization.
			if err := client.Call(pulseContext, "/v1/features-next", catalog, nil); err != nil {
				stop()
				cancel()
				return
			}
			started := time.Now()
			var pulse contract.FeaturePulse
			err = client.Call(pulseContext, "/v1/features-pulse", assignment.Request.Reference(), &pulse)
			stop()
			if err != nil || pulse.Cancel {
				cancel()
				return
			}
			renewed, err := featureLeaseDeadline(started, pulse.LeaseMillis)
			if err != nil {
				cancel()
				return
			}
			leaseMutex.Lock()
			leaseDeadline = renewed
			timer.Reset(time.Until(renewed))
			leaseMutex.Unlock()
		}
	}()
	identity := invocation.Identity{Issuer: "dockpipe.remote.broker:" + remoteidentity.Fingerprint(client.BrokerKey), Subject: assignment.Operator}
	authorize := func(ctx context.Context, binding invocation.Binding) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if binding.Target.Kind != invocation.Remote || binding.Target.ID != assignment.Request.Target.Node || binding.Target.Fingerprint != assignment.Request.Target.Fingerprint || binding.Target.Generation != assignment.Request.Target.Generation || binding.Identity != identity {
			return errors.New("native invocation escaped its authenticated feature assignment")
		}
		parts := strings.SplitN(assignment.Request.Action, ".", 2)
		if len(parts) != 2 || binding.Contract.Capability != "dockpipe."+parts[0] || binding.Contract.Action != parts[1] || binding.Contract.Version != "1" {
			return errors.New("native invocation changed the authorized feature action")
		}
		current, err := loadFeaturePermissions(root)
		if err != nil || permissionFingerprint(current) != policy || !featureCatalogAllows(current.catalog(catalog.Target), assignment.Request) {
			return errors.New("worker-local feature authority changed")
		}
		leaseMutex.Lock()
		valid := time.Now().Before(leaseDeadline)
		leaseMutex.Unlock()
		if !valid {
			return errors.New("operator feature lease expired")
		}
		if assignment.Request.Root != "" {
			for _, permission := range current.Roots {
				if permission.ID != assignment.Request.Root {
					continue
				}
				if err := infrastructure.ValidateUnlinkedPath(permission.Path); err != nil {
					return err
				}
				if strings.HasPrefix(assignment.Request.Action, "git.") {
					// Git must not discover a repository above the granted root.
					metadata, err := os.Lstat(filepath.Join(permission.Path, ".git"))
					if err != nil || metadata.Mode()&os.ModeSymlink != 0 {
						return errors.New("Git grants must select an exact trusted checkout root")
					}
				}
			}
		}
		return nil
	}
	result := execute(ctx, permissions, assignment.Request, identity, authorize)
	cancel()
	<-done
	return result
}

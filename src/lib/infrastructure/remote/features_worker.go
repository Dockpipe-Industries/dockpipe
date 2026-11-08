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
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type featureWorkerRecord struct {
	Request      contract.FeatureRequest `json:"request"`
	Result       *contract.FeatureResult `json:"result,omitempty"`
	ReceiptOnly  bool
	ResultDigest string
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
		journal.Records[id] = featureWorkerRecord{Request: receipt.requestMetadata(), Result: &result, ReceiptOnly: true, ResultDigest: receipt.ResultDigest}
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
		ledger.Records[id] = receiptFor(record.Request, status, false, digest)
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
	unlock, err := Lock(filepath.Join(root, "features.lock"))
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
		if acknowledgement.Expired && record.Result.Status != "unknown" {
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
		if assignment.Request.Validate() != nil || assignment.Request.Target != target || !validMachineNonce(assignment.Operator) || !featureCatalogAllows(catalog, assignment.Request) {
			return errors.New("broker returned a feature outside this worker's identity or grants")
		}
		deadline, err := featureLeaseDeadline(started, assignment.LeaseMillis)
		if err != nil {
			return err
		}
		record, exists := journal.Records[assignment.Request.ID]
		if !exists {
			for id, previous := range journal.Records {
				if previous.Result != nil && previous.Result.Status != "unknown" && time.Now().After(previous.Request.Deadline) && time.Now().After(retainedUntil[id]) {
					delete(journal.Records, id)
					delete(retainedUntil, id)
				}
			}
			if len(journal.Records) >= contract.MaxJobs {
				return errors.New("worker feature journal capacity reached")
			}
			record = featureWorkerRecord{Request: assignment.Request}
			journal.Records[record.Request.ID] = record
			retainedUntil[record.Request.ID] = time.Now().Add(contract.FeatureLifetime)
			if contract.FeatureAccess(record.Request.Action) != "read" {
				if err := writeFeatureJournal(root, journal); err != nil {
					return err
				}
			}
			result := executeLeasedFeature(ctx, client, root, permissions, policy, catalog, *assignment, deadline)
			record.Result = &result
			journal.Records[record.Request.ID] = record
			if contract.FeatureAccess(record.Request.Action) != "read" {
				if err := writeFeatureJournal(root, journal); err != nil {
					return err
				}
			}
		}
		if record.Result == nil {
			return errors.New("unfinished feature intent cannot be replayed")
		}
		completion := featureCompletionFor(record)
		var acknowledgement contract.FeatureResultAck
		if err := client.callWithRetry(ctx, "/v1/features-result", completion, &acknowledgement); err != nil {
			return err
		}
		if acknowledgement.Reference != completion.Reference {
			return errors.New("feature result acknowledgement identity changed")
		}
		// The broker acknowledged delivery. Keep only the mutation identity/status
		// needed to prevent replay; plaintext output is not a worker cache.
		digest := featureResultDigest(record.Result)
		if record.ReceiptOnly {
			digest = record.ResultDigest
		}
		receipt := receiptFor(record.Request, record.Result.Status, false, digest)
		metadata := recoveredFeatureResult(receipt.Status)
		journal.Records[record.Request.ID] = featureWorkerRecord{Request: receipt.requestMetadata(), Result: &metadata, ReceiptOnly: true, ResultDigest: digest}
	}
	return ctx.Err()
}

func featureCompletionFor(record featureWorkerRecord) contract.FeatureCompletion {
	digest := record.ResultDigest
	if !record.ReceiptOnly {
		digest = featureResultDigest(record.Result)
	}
	return contract.FeatureCompletion{Reference: record.Request.Reference(), Result: *record.Result,
		ReceiptOnly: record.ReceiptOnly, ResultDigest: digest, Deadline: record.Request.Deadline}
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

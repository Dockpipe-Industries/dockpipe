package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

type featureBrokerState struct {
	Schema     string                               `json:"schema"`
	Operations map[string]contract.FeatureOperation `json:"operations"`
}

type featureBroker struct {
	state      featureBrokerState
	catalogs   map[string]contract.FeatureCatalog
	admissions map[string]operatorAdmission
	recoveries map[string]operatorAdmission
}

func (broker *Broker) loadFeatures() error {
	features := &featureBroker{
		state:    featureBrokerState{Schema: contract.FeatureVersion, Operations: map[string]contract.FeatureOperation{}},
		catalogs: map[string]contract.FeatureCatalog{}, admissions: map[string]operatorAdmission{}, recoveries: map[string]operatorAdmission{},
	}
	path := filepath.Join(filepath.Dir(broker.path), "feature-operations.json")
	ledger, err := readFeatureLedger(path)
	if err != nil {
		return err
	}
	for id, receipt := range ledger.Records {
		if id != receipt.ID || receipt.validate() != nil {
			return errors.New("invalid stored feature receipt")
		}
		features.state.Operations[id] = receipt.metadataOperation()
	}
	broker.features = features
	return nil
}

// Only the status route can read a reclaimed, expired mutation from the last
// durable snapshot. The caller never gets the payload or an execution lease.
func (broker *Broker) retainedFeatureStatus(reference contract.FeatureReference) (contract.FeatureOperation, bool) {
	ledger, err := readFeatureLedger(filepath.Join(filepath.Dir(broker.path), "feature-operations.json"))
	if err != nil {
		return contract.FeatureOperation{}, false
	}
	receipt, exists := ledger.Records[reference.ID]
	if !exists || receipt.ID != reference.ID || receipt.Target != reference.Target ||
		receipt.validate() != nil || time.Now().Before(receipt.Deadline) {
		return contract.FeatureOperation{}, false
	}
	switch receipt.Status {
	case "success", "failure", "cancelled":
		return receipt.metadataOperation(), true
	default:
		return contract.FeatureOperation{}, false
	}
}

func (broker *Broker) storeFeature(operation contract.FeatureOperation) error {
	encodedOperation, err := json.Marshal(operation)
	if err != nil || len(encodedOperation) > contract.MaxBody {
		return errors.New("feature operation exceeds the transport response budget")
	}
	previous, existed := broker.features.state.Operations[operation.Request.ID]
	broker.features.state.Operations[operation.Request.ID] = operation
	data, err := json.Marshal(broker.features.state)
	if err == nil && len(data) > contract.MaxFeatureStorage {
		err = errors.New("remote feature storage budget exhausted")
	}
	if err != nil {
		if existed {
			broker.features.state.Operations[operation.Request.ID] = previous
		} else {
			delete(broker.features.state.Operations, operation.Request.ID)
		}
		return err
	}
	if contract.FeatureAccess(operation.Request.Action) == "read" {
		return nil
	}
	ledger := featureLedger{Schema: contract.FeatureVersion, Records: map[string]featureReceipt{}}
	for id, retained := range broker.features.state.Operations {
		if contract.FeatureAccess(retained.Request.Action) == "read" {
			continue
		}
		digest := retained.ResultDigest
		if !retained.ReceiptOnly {
			digest = featureResultDigest(retained.Result)
		}
		receipt := receiptFor(retained.Request, retained.Status, retained.CancelRequested, digest)
		receipt.RecoveryRequested = retained.RecoveryRequested
		receipt.Reconciled = retained.Reconciled
		receipt.RecoveryChoice = retained.RecoveryChoice
		receipt.RecoveryResource = retained.RecoveryResource
		receipt.RecoveryError = retained.RecoveryError
		receipt.HostRecovery = retained.HostRecovery
		ledger.Records[id] = receipt
	}
	if err := WritePrivate(filepath.Join(filepath.Dir(broker.path), "feature-operations.json"), ledger); err != nil {
		broker.poisoned = true
		return err
	}
	return nil
}

func (broker *Broker) reclaimFeatures(now time.Time) {
	for id, operation := range broker.features.state.Operations {
		if now.Before(operation.Request.Deadline) || operation.Status == "running" || operation.Status == "unknown" || operation.Status == "queued" {
			continue
		}
		delete(broker.features.state.Operations, id)
		delete(broker.features.admissions, id)
	}
	// The next successful durable publication commits this reclamation. An old
	// on-disk receipt is harmless: its digest-bound deadline forbids replay.
}

func featureOperatorPath(path string) bool {
	return strings.HasPrefix(path, "/v1/features-")
}

func (broker *Broker) featureOperator(path string, raw []byte, admission operatorAdmission) (any, error) {
	broker.reclaimFeatures(time.Now())
	if path == "/v1/features-catalog" {
		var request struct {
			Schema              string `json:"schema"`
			IncludeWorkflowOnly bool   `json:"include_workflow_only,omitempty"`
		}
		if decodeFeature(raw, &request) != nil || request.Schema != contract.FeatureVersion {
			return nil, errors.New("invalid feature catalog request")
		}
		catalogs := []contract.FeatureCatalog{}
		for _, catalog := range broker.features.catalogs {
			if catalog.WorkflowOnly && !request.IncludeWorkflowOnly {
				continue
			}
			if broker.featureCatalogCurrent(catalog) {
				if !catalog.WorkflowOnly {
					catalog = broker.publishTerminalCatalog(catalog)
				}
				catalogs = append(catalogs, catalog)
			}
		}
		sort.Slice(catalogs, func(i, j int) bool { return catalogs[i].Target.Node < catalogs[j].Target.Node })
		return catalogs, nil
	}
	if path == "/v1/features-read-submit" || path == "/v1/features-write-submit" {
		var request contract.FeatureRequest
		if err := decodeFeature(raw, &request); err != nil || request.Validate() != nil || validateFeaturePayload(request) != nil {
			return nil, errors.New("invalid typed feature request")
		}
		if (contract.FeatureAccess(request.Action) == "read") != (path == "/v1/features-read-submit") {
			return nil, errors.New("feature access does not match authenticated route")
		}
		if previous, exists := broker.features.state.Operations[request.ID]; exists {
			return previous, nil
		}
		now := time.Now().UTC()
		if !now.Before(request.Deadline) || request.Deadline.Sub(now) > contract.FeatureLifetime {
			return nil, errors.New("feature deadline must be within two minutes")
		}
		catalog, exists := broker.features.catalogs[request.Target.Node]
		if !exists || catalog.Target != request.Target || !broker.featureCatalogCurrent(catalog) || !featureCatalogAllows(catalog, request) {
			return nil, errors.New("selected machine, generation or feature grant is unavailable")
		}
		broker.reclaimFeatures(now)
		if len(broker.features.state.Operations) >= contract.MaxJobs {
			return nil, errors.New("remote feature operation capacity reached")
		}
		operation := contract.FeatureOperation{Request: request, Status: "queued"}
		if request.Action == "terminal.host-recover" {
			if err := broker.revokeHostTerminalRecovery(request); err != nil {
				return nil, err
			}
		}
		if strings.HasPrefix(request.Action, "containers.") && request.Action != "containers.terminal-recover" && contract.FeatureAccess(request.Action) == "write" {
			var input featureContainerRequest
			if decodeFeature(request.Payload, &input) != nil {
				return nil, errors.New("invalid container recovery identity")
			}
			operation.RecoveryResource = input.Container
		}
		if err := broker.storeFeature(operation); err != nil {
			return nil, err
		}
		broker.features.admissions[request.ID] = admission
		return operation, nil
	}
	var reference contract.FeatureReference
	choice := "verify"
	reviewedResource := ""
	if path == "/v1/features-reconcile" {
		var recovery contract.FeatureRecoveryRequest
		if decodeFeature(raw, &recovery) != nil || recovery.FeatureReference.Validate() != nil ||
			recovery.Choice != "" && recovery.Choice != "verify" && recovery.Choice != "retire-container" {
			return nil, errors.New("invalid feature recovery request")
		}
		reference = recovery.FeatureReference
		if recovery.Choice != "" {
			choice = recovery.Choice
		}
		reviewedResource = recovery.Resource
	} else if err := decodeFeature(raw, &reference); err != nil || reference.Validate() != nil {
		return nil, errors.New("invalid feature operation reference")
	}
	operation, exists := broker.features.state.Operations[reference.ID]
	if !exists && path == "/v1/features-write-status" {
		if retained, ok := broker.retainedFeatureStatus(reference); ok {
			return retained, nil
		}
	}
	if !exists || operation.Request.Target != reference.Target {
		return nil, errors.New("feature operation does not belong to this machine lifetime")
	}
	access := contract.FeatureAccess(operation.Request.Action)
	switch path {
	case "/v1/features-read-status", "/v1/features-write-status":
		if (access == "read") != (path == "/v1/features-read-status") {
			return nil, errors.New("feature status route cannot renew this operation")
		}
		if previous, exists := broker.features.admissions[reference.ID]; exists &&
			previous.session == admission.session && previous.generation == admission.generation {
			broker.features.admissions[reference.ID] = admission
		}
		if previous, exists := broker.features.recoveries[reference.ID]; exists && previous.session == admission.session && previous.generation == admission.generation {
			broker.features.recoveries[reference.ID] = admission
		}
		return operation, nil
	case "/v1/features-cancel":
		operation.CancelRequested = true
		if operation.Status == "queued" {
			operation.Status = "cancelled"
			operation.Result = &contract.FeatureResult{Status: "cancelled"}
		}
		return operation, broker.storeFeature(operation)
	case "/v1/features-reconcile":
		if access == "read" || choice == "retire-container" && !strings.HasPrefix(operation.Request.Action, "containers.") {
			return nil, errors.New("recovery strategy does not match the captured mutation")
		}
		if choice == "retire-container" {
			catalog, exists := broker.features.catalogs[reference.Target.Node]
			if !exists || !broker.featureCatalogCurrent(catalog) || !catalog.DockerRecovery || catalog.Target.Fingerprint != reference.Target.Fingerprint ||
				!validMachineNonce(reviewedResource) || operation.RecoveryResource != reviewedResource {
				return nil, errors.New("container retirement requires its exact resource and an independent live worker recovery grant")
			}
		}
		if operation.Status == "queued" {
			operation.CancelRequested = true
			operation.Status = "cancelled"
			operation.Result = &contract.FeatureResult{Status: "cancelled"}
			delete(broker.features.admissions, reference.ID)
			if err := broker.storeFeature(operation); err != nil {
				return nil, err
			}
			return featureMetadataOperation(operation), nil
		}
		if operation.Status != "running" && operation.Status != "unknown" {
			return featureMetadataOperation(operation), nil
		}
		// Revocation is permanent for this captured intent. Status polling cannot
		// recreate the removed admission, and no new assignment can execute it.
		operation.CancelRequested = true
		operation.RecoveryRequested = true
		operation.RecoveryChoice = choice
		operation.RecoveryError = ""
		delete(broker.features.admissions, reference.ID)
		broker.features.recoveries[reference.ID] = admission
		if err := broker.storeFeature(operation); err != nil {
			return nil, err
		}
		return featureMetadataOperation(operation), nil
	default:
		return nil, errors.New("unknown operator feature operation")
	}
}

func (broker *Broker) featureCatalogCurrent(catalog contract.FeatureCatalog) bool {
	enrollment, exists := broker.state.Nodes[catalog.Target.Node]
	now := time.Now()
	return exists && broker.enrollmentActive(enrollment, now) && enrollment.TokenHash != "" && enrollment.MachineKey != "" &&
		remoteidentity.Fingerprint(enrollment.MachineKey) == catalog.Target.Fingerprint && now.Sub(catalog.ReceivedAt) <= contract.FeatureCatalogFreshness
}

func featureCatalogAllows(catalog contract.FeatureCatalog, request contract.FeatureRequest) bool {
	if catalog.WorkflowOnly {
		return false
	}

	access := contract.FeatureAccess(request.Action)
	if request.Action == "terminal.host-recover" {
		return terminalRoot(catalog, request.Root)
	}
	if strings.HasPrefix(request.Action, "containers.") {
		if request.Action == "containers.terminal-recover" {
			var payload terminalRecoveryPayload
			if decodeFeature(request.Payload, &payload) != nil {
				return false
			}
			return catalog.DockerRead && (payload.Retire && catalog.DockerRecovery || !payload.Retire && catalog.DockerExec)
		}
		return access == "read" && catalog.DockerRead || access == "write" && catalog.DockerControl
	}
	for _, root := range catalog.Roots {
		if root.ID != request.Root {
			continue
		}
		if strings.HasPrefix(request.Action, "files.") {
			return access == "read" && root.FilesRead || access == "write" && root.FilesWrite
		}
		return access == "read" && root.GitRead || access == "write" && root.GitWrite || access == "publish" && root.GitPublish
	}
	return false
}

func (broker *Broker) featureLease(operation contract.FeatureOperation) contract.FeaturePulse {
	admission, exists := broker.features.admissions[operation.Request.ID]
	if !exists || broker.security == nil || broker.security.CheckAdmission(admission) != nil || operation.CancelRequested || !time.Now().Before(operation.Request.Deadline) {
		return contract.FeaturePulse{Cancel: true}
	}
	catalog, exists := broker.features.catalogs[operation.Request.Target.Node]
	if !exists || !broker.featureCatalogCurrent(catalog) || catalog.Target != operation.Request.Target || !featureCatalogAllows(catalog, operation.Request) {
		return contract.FeaturePulse{Cancel: true}
	}
	until := time.Now().Add(contract.FeatureLease)
	if admission.deadline.Before(until) {
		until = admission.deadline
	}
	if operation.Request.Deadline.Before(until) {
		until = operation.Request.Deadline
	}
	return contract.FeaturePulse{LeaseMillis: time.Until(until).Milliseconds()}
}

func (broker *Broker) featureWorker(node, path string, raw []byte) (any, error) {
	enrollment := broker.state.Nodes[node]
	if enrollment.MachineKey == "" || enrollment.Revoked {
		return nil, errors.New("remote features require paired machine identities")
	}
	if path == "/v1/features-next" {
		broker.reclaimFeatures(time.Now())
		var catalog contract.FeatureCatalog
		if decodeFeature(raw, &catalog) != nil || catalog.Validate() != nil || catalog.Target.Node != node || catalog.Target.Fingerprint != remoteidentity.Fingerprint(enrollment.MachineKey) {
			return nil, errors.New("invalid authenticated feature catalog")
		}
		if enrollment.GuestGrant != "" && !catalog.WorkflowOnly {
			return nil, errors.New("disposable guests can only advertise workflow delivery")
		}

		catalog.ReceivedAt = time.Now().UTC()
		broker.features.catalogs[node] = catalog
		if catalog.WorkflowOnly {
			return nil, nil
		}
		// Recoveries do not execute work. An unrelated unknown receipt must not
		// prevent the worker from acknowledging the specifically revoked intent.
		for _, operation := range broker.features.state.Operations {
			if operation.Request.Target.Node == node && operation.RecoveryRequested && !operation.Reconciled &&
				(operation.Status == "running" || operation.Status == "unknown") {
				lease := broker.featureRecoveryLease(operation)
				if operation.RecoveryChoice == "retire-container" && lease.Cancel {
					return nil, nil
				}
				assignment := contract.FeatureAssignment{Request: featureMetadataOperation(operation).Request, Reconcile: true, RecoveryChoice: operation.RecoveryChoice, RecoveryResource: operation.RecoveryResource}
				if operation.RecoveryChoice == "retire-container" {
					assignment.LeaseMillis = lease.LeaseMillis
					admission := broker.features.recoveries[operation.Request.ID]
					assignment.Operator = Hash(fmt.Sprintf("%s:%d", admission.session, admission.generation))
				}
				return assignment, nil
			}
		}
		// One operation per worker serializes all root writes and container control.
		for _, operation := range broker.features.state.Operations {
			if operation.Request.Target.Node == node && (operation.Status == "running" || operation.Status == "unknown" && !operation.Reconciled) {
				lease := broker.featureLease(operation)
				if contract.FeatureAccess(operation.Request.Action) == "read" && (lease.Cancel || operation.Request.Target != catalog.Target) {
					operation.Status = "cancelled"
					operation.Result = &contract.FeatureResult{Status: "cancelled"}
					if err := broker.storeFeature(operation); err != nil {
						return nil, err
					}
					continue
				}
				if operation.Status == "running" && operation.Request.Target == catalog.Target && !lease.Cancel {
					return broker.featureAssignment(operation.Request, lease), nil
				}
				return nil, nil
			}
		}
		ids := make([]string, 0, len(broker.features.state.Operations))
		for id := range broker.features.state.Operations {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			operation := broker.features.state.Operations[id]
			if operation.Status != "queued" || operation.Request.Target.Node != node {
				continue
			}
			lease := broker.featureLease(operation)
			if lease.Cancel || operation.Request.Target != catalog.Target {
				operation.Status = "cancelled"
				operation.Result = &contract.FeatureResult{Status: "cancelled", Error: "Feature authority or worker generation changed before dispatch."}
				if err := broker.storeFeature(operation); err != nil {
					return nil, err
				}
				continue
			}
			operation.Status = "running"
			if err := broker.storeFeature(operation); err != nil {
				return nil, err
			}
			return broker.featureAssignment(operation.Request, lease), nil
		}
		return nil, nil
	}
	if path == "/v1/features-reconciled" {
		return broker.featureReconciled(node, raw)
	}
	if path == "/v1/features-recovery-pulse" {
		var reference contract.FeatureReference
		if decodeFeature(raw, &reference) != nil || reference.Validate() != nil || reference.Target.Node != node {
			return nil, errors.New("invalid recovery lease reference")
		}
		operation, exists := broker.features.state.Operations[reference.ID]
		if !exists || operation.Request.Target != reference.Target {
			return nil, errors.New("recovery operation identity changed")
		}
		return broker.featureRecoveryLease(operation), nil
	}
	if path == "/v1/features-recovery-failed" {
		return broker.featureRecoveryFailed(node, raw)
	}
	var reference contract.FeatureReference
	var completion contract.FeatureCompletion
	if path == "/v1/features-result" {
		// Results include native maps such as Docker labels. Their byte bound is
		// authoritative; request-schema object-field limits do not apply to data.
		if Decode(raw, &completion) != nil || completion.Result.Validate() != nil || completion.Deadline.IsZero() {
			return nil, errors.New("invalid feature completion")
		}
		if completion.ReceiptOnly {
			if len(completion.Result.Payload) != 0 || completion.ResultDigest != "" && !validMachineNonce(completion.ResultDigest) {
				return nil, errors.New("invalid recovered feature receipt")
			}
		} else if completion.ResultDigest != featureResultDigest(&completion.Result) {
			return nil, errors.New("feature result digest mismatch")
		}
		if completion.Reconciled && (!completion.ReceiptOnly || completion.Result.Status != "unknown") {
			return nil, errors.New("invalid reconciliation receipt")
		}
		reference = completion.Reference
	} else if decodeFeature(raw, &reference) != nil {
		return nil, errors.New("invalid feature reference")
	}
	operation, exists := broker.features.state.Operations[reference.ID]
	if !exists && path == "/v1/features-result" && completion.ReceiptOnly && (completion.Result.Status != "unknown" || completion.Reconciled) &&
		reference.Validate() == nil && reference.Target.Node == node && reference.Target.Fingerprint == remoteidentity.Fingerprint(enrollment.MachineKey) && !time.Now().Before(completion.Deadline) {
		// The paired worker may discard an expired metadata-only receipt. This
		// neither creates an operation nor authorizes execution of missing work.
		return contract.FeatureResultAck{Reference: reference, Expired: true}, nil
	}
	if reference.Validate() != nil || !exists || reference.Target.Node != node || reference.Target != operation.Request.Target || reference.Target.Fingerprint != remoteidentity.Fingerprint(enrollment.MachineKey) {
		return nil, errors.New("feature operation identity changed")
	}
	if path == "/v1/features-pulse" {
		if operation.Status != "running" {
			return contract.FeaturePulse{Cancel: true}, nil
		}
		// Pulses do not renew the operator admission or catalog grant. The worker
		// republishes its same catalog while running through a separate request.
		return broker.featureLease(operation), nil
	}
	if path != "/v1/features-result" {
		return nil, errors.New("unknown worker feature operation")
	}
	if !completion.Deadline.Equal(operation.Request.Deadline) {
		return nil, errors.New("feature completion deadline changed")
	}
	if operation.Request.Action == "containers.terminal-recover" && completion.Result.Status == "success" && !completion.ReceiptOnly {
		if err := broker.completeContainerTerminalRecovery(operation.Request, completion.Result); err != nil {
			return nil, err
		}
	}
	if completion.RecoveryResource != "" {
		if !validMachineNonce(completion.RecoveryResource) || operation.RecoveryResource != "" && operation.RecoveryResource != completion.RecoveryResource {
			return nil, errors.New("completion changed the captured recovery resource")
		}
		if operation.RecoveryResource == "" && operation.Request.Action != "containers.terminal-recover" {
			return nil, errors.New("completion introduced an unrelated recovery resource")
		}
		operation.RecoveryResource = completion.RecoveryResource
	}
	if completion.Reconciled && !operation.Reconciled {
		return nil, errors.New("reconciliation requires the worker cleanup protocol")
	}
	if operation.Reconciled {
		if completion.HostRecovery != nil {
			return nil, errors.New("reconciled feature outcome cannot acquire a different terminal recovery result")
		}
		// A late pre-reconciliation completion cannot erase the acknowledgment
		// or relabel the unknown outcome as success.
		return contract.FeatureResultAck{Reference: reference, Expired: !time.Now().Before(operation.Request.Deadline)}, nil
	}
	if operation.Status != "running" && operation.Status != "unknown" {
		digest := operation.ResultDigest
		if !operation.ReceiptOnly {
			digest = featureResultDigest(operation.Result)
		}
		if digest != completion.ResultDigest {
			return nil, errors.New("feature result changed after publication")
		}
		if err := broker.acceptHostTerminalCompletion(&operation, completion); err != nil {
			return nil, err
		}
		return contract.FeatureResultAck{Reference: reference, Expired: !time.Now().Before(operation.Request.Deadline)}, nil
	}
	if err := broker.acceptHostTerminalCompletion(&operation, completion); err != nil {
		return nil, err
	}
	operation.Result = &completion.Result
	if completion.Result.Status == "unknown" && operation.RecoveryError != "" {
		result := completion.Result
		result.Error = recoveryFailureMessage(operation.RecoveryError)
		operation.Result = &result
	}
	operation.Status = completion.Result.Status
	operation.ReceiptOnly = operation.ReceiptOnly || completion.ReceiptOnly
	operation.ResultDigest = completion.ResultDigest
	if completion.ReceiptOnly {
		operation.Request = receiptFor(operation.Request, operation.Status, operation.CancelRequested, operation.ResultDigest).requestMetadata()
	}
	if err := broker.storeFeature(operation); err != nil {
		return nil, err
	}
	return contract.FeatureResultAck{Reference: reference, Expired: !time.Now().Before(operation.Request.Deadline)}, nil
}

func (broker *Broker) featureAssignment(request contract.FeatureRequest, lease contract.FeaturePulse) contract.FeatureAssignment {
	admission := broker.features.admissions[request.ID]
	return contract.FeatureAssignment{Request: request, LeaseMillis: lease.LeaseMillis,
		Operator: Hash(fmt.Sprintf("%s:%d", admission.session, admission.generation))}
}

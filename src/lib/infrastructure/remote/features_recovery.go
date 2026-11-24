package remote

import (
	"errors"
	"strings"
	"time"

	"context"
	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func featureMetadataOperation(operation contract.FeatureOperation) contract.FeatureOperation {
	if !operation.ReceiptOnly {
		operation.ResultDigest = featureResultDigest(operation.Result)
	}
	operation.Request.Payload = nil
	operation.Request.Nonce = ""
	operation.ReceiptOnly = true
	if operation.Result != nil {
		result := recoveredFeatureResult(operation.Status)
		operation.Result = &result
	}
	return operation
}

func (broker *Broker) featureReconciled(node string, raw []byte) (any, error) {
	var proof contract.FeatureReconciliation
	if decodeFeature(raw, &proof) != nil || proof.Reference.Validate() != nil || proof.Worker.Validate() != nil ||
		proof.Reference.Target.Node != node || proof.Worker.Node != node || proof.Worker.Fingerprint != proof.Reference.Target.Fingerprint {
		return nil, errors.New("invalid worker reconciliation identity")
	}
	catalog, exists := broker.features.catalogs[node]
	if !exists || !broker.featureCatalogCurrent(catalog) || catalog.Target != proof.Worker {
		return nil, errors.New("reconciliation requires the current authenticated worker")
	}
	operation, exists := broker.features.state.Operations[proof.Reference.ID]
	if !exists || operation.Request.Target != proof.Reference.Target || !operation.RecoveryRequested ||
		!operation.CancelRequested || contract.FeatureAccess(operation.Request.Action) == "read" {
		return nil, errors.New("reconciliation does not match a revoked mutation")
	}
	if strings.HasPrefix(operation.Request.Action, "containers.") && proof.Resource != operation.RecoveryResource {
		return nil, errors.New("reconciliation changed the captured container identity")
	}
	if operation.RecoveryChoice == "retire-container" && broker.featureRecoveryLease(operation).Cancel {
		return nil, errors.New("container retirement authority expired before cleanup confirmation")
	}
	if _, exists := broker.features.admissions[proof.Reference.ID]; exists {
		return nil, errors.New("operation still has live invocation authority")
	}
	if operation.Status != "running" && operation.Status != "unknown" {
		return nil, errors.New("operation already has a terminal outcome")
	}
	operation = featureMetadataOperation(operation)
	operation.Status = "unknown"
	operation.Result = &contract.FeatureResult{Status: "unknown", Error: "Worker confirmed execution cleanup and invocation revocation. Refresh remote state before new work; the mutation outcome remains unknown."}
	operation.Reconciled = true
	if err := broker.storeFeature(operation); err != nil {
		return nil, err
	}
	return contract.FeatureResultAck{Reference: proof.Reference, Expired: !time.Now().Before(operation.Request.Deadline)}, nil
}

// Called only from the serial worker loop while features.lock is held. File
// operations run synchronously in that process and create no child processes.
// A restarted exclusive lock holder therefore also proves their old execution
// is gone. Git hooks and container/provider work do not have this property and
// cannot use this acknowledgment.
func validateFileReconciliation(assignment contract.FeatureAssignment, target contract.FeatureTarget) error {
	request := assignment.Request
	if !assignment.Reconcile || request.Reference().Validate() != nil || target.Validate() != nil ||
		request.Target.Node != target.Node || request.Target.Fingerprint != target.Fingerprint ||
		contract.FeatureAccess(request.Action) == "" || contract.FeatureAccess(request.Action) == "read" ||
		request.Deadline.IsZero() || len(request.Payload) != 0 || request.Nonce != "" {
		return errors.New("invalid metadata-only reconciliation assignment")
	}
	rootRequired := contract.FeatureUsesRoot(request.Action)
	if rootRequired && !contract.ValidID(request.Root) || !rootRequired && request.Root != "" {
		return errors.New("recovery root does not match its captured action")
	}
	if assignment.RecoveryChoice == "retire-container" {
		if !strings.HasPrefix(request.Action, "containers.") || assignment.LeaseMillis <= 0 || assignment.LeaseMillis > contract.FeatureLease.Milliseconds() || !validMachineNonce(assignment.Operator) {
			return errors.New("container retirement requires a bounded recovery lease")
		}
	} else if assignment.RecoveryChoice != "" && assignment.RecoveryChoice != "verify" || assignment.LeaseMillis != 0 || assignment.Operator != "" {
		return errors.New("invalid recovery strategy or authority")
	}
	return nil
}

func (broker *Broker) featureRecoveryLease(operation contract.FeatureOperation) contract.FeaturePulse {
	admission, exists := broker.features.recoveries[operation.Request.ID]
	catalog, available := broker.features.catalogs[operation.Request.Target.Node]
	if !exists || broker.security == nil || broker.security.CheckAdmission(admission) != nil || !operation.RecoveryRequested ||
		!available || !broker.featureCatalogCurrent(catalog) || catalog.Target.Fingerprint != operation.Request.Target.Fingerprint ||
		operation.RecoveryChoice == "retire-container" && !catalog.DockerRecovery || operation.RecoveryChoice != "retire-container" && !catalog.DockerRead {
		return contract.FeaturePulse{Cancel: true}
	}
	until := time.Now().Add(contract.FeatureLease)
	if admission.deadline.Before(until) {
		until = admission.deadline
	}
	return contract.FeaturePulse{LeaseMillis: time.Until(until).Milliseconds()}
}

type featureRecoveryFailure struct {
	Reference contract.FeatureReference `json:"reference"`
	Reason    string                    `json:"reason"`
}

func recoveryFailureMessage(reason string) string {
	switch reason {
	case "legacy-process-scope-unavailable":
		return "The original operation has no durable process scope; its descendant cleanup cannot be verified."
	case "container-retirement-required":
		return "The Docker daemon completion is uncertain. Review the exact container and explicitly choose retirement to fence delayed work."
	case "retirement-grant-unavailable":
		return "Container retirement requires the independent worker Docker recovery grant and current operator authority."
	case "process-cleanup-unconfirmed":
		return "The original native process scope could not be confirmed empty; the mutation remains blocked."
	case "worker-reboot-required":
		return "This worker uses boot-fenced recovery. Reboot the worker host before reconciling this uncertain operation; its captured kernel boot identity has not changed."
	case "provider-identity-unavailable":
		return "The original Docker provider/resource identity or its authoritative removal could not be confirmed."
	case "original-receipt-unavailable":
		return "The original matching worker receipt is unavailable; cleanup cannot be acknowledged."
	default:
		return ""
	}
}

func (broker *Broker) featureRecoveryFailed(node string, raw []byte) (any, error) {
	var failure featureRecoveryFailure
	if decodeFeature(raw, &failure) != nil || failure.Reference.Validate() != nil || failure.Reference.Target.Node != node || recoveryFailureMessage(failure.Reason) == "" {
		return nil, errors.New("invalid recovery failure receipt")
	}
	operation, exists := broker.features.state.Operations[failure.Reference.ID]
	if !exists || operation.Request.Target != failure.Reference.Target || !operation.RecoveryRequested || operation.Reconciled {
		return nil, errors.New("recovery failure changed its captured operation")
	}
	operation = featureMetadataOperation(operation)
	operation.Status = "unknown"
	operation.Result = &contract.FeatureResult{Status: "unknown", Error: recoveryFailureMessage(failure.Reason)}
	operation.RecoveryError = failure.Reason
	operation.RecoveryRequested = false
	delete(broker.features.recoveries, failure.Reference.ID)
	return contract.FeatureResultAck{Reference: failure.Reference}, broker.storeFeature(operation)
}

func recoverWorkerFeature(ctx context.Context, client *Client, root string, target contract.FeatureTarget, permissions FeaturePermissions, assignment contract.FeatureAssignment, record featureWorkerRecord, exists bool) string {
	request := assignment.Request
	if strings.HasPrefix(request.Action, "files.") || request.Action == "terminal.host-recover" {
		return ""
	}
	if !exists {
		return "original-receipt-unavailable"
	}
	if !record.Supervised {
		return "legacy-process-scope-unavailable"
	}
	if record.Docker != nil && record.Docker.Container != assignment.RecoveryResource {
		return "provider-identity-unavailable"
	}
	if record.Scope != nil {
		cleanup, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := process.StopScope(cleanup, *record.Scope)
		cancel()
		if err != nil {
			if errors.Is(err, process.ErrRebootRequired) {
				return "worker-reboot-required"
			}
			return "process-cleanup-unconfirmed"
		}
	}
	if !strings.HasPrefix(request.Action, "containers.") || record.Docker == nil {
		// No provider receipt means the durable observer never admitted a daemon mutation.
		return ""
	}
	retire := assignment.RecoveryChoice == "retire-container"
	if !record.Docker.Completed && !retire {
		return "container-retirement-required"
	}
	if retire {
		if !permissions.DockerRecovery {
			return "retirement-grant-unavailable"
		}
		var lease contract.FeaturePulse
		started := time.Now()
		if client.Call(ctx, "/v1/features-recovery-pulse", request.Reference(), &lease) != nil || lease.Cancel {
			return "retirement-grant-unavailable"
		}
		deadline, err := featureLeaseDeadline(started, lease.LeaseMillis)
		if err != nil {
			return "retirement-grant-unavailable"
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	nativeTarget := invocation.Target{Kind: invocation.Remote, ID: target.Node, Fingerprint: target.Fingerprint, Generation: target.Generation}
	identity := invocation.Identity{Issuer: "dockpipe.remote.broker:" + remoteidentity.Fingerprint(client.BrokerKey), Subject: "recovery:" + request.ID}
	authorize := func(ctx context.Context, binding invocation.Binding) error {
		current, err := loadFeaturePermissions(root)
		if err != nil || permissionFingerprint(current) != permissionFingerprint(permissions) || binding.Target != nativeTarget || binding.Identity != identity ||
			retire && !current.DockerRecovery || !current.DockerRead {
			return errors.New("worker recovery authority changed")
		}
		var pulse contract.FeaturePulse
		if client.Call(ctx, "/v1/features-recovery-pulse", request.Reference(), &pulse) != nil || pulse.Cancel {
			return errors.New("operator recovery authority expired")
		}
		return ctx.Err()
	}
	if infrastructure.RecoverContainerLifecycleWithAuthority(ctx, *record.Docker, retire, nativeTarget, identity, authorize) != nil {
		return "provider-identity-unavailable"
	}
	return ""
}

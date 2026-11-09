package remote

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
)

// Only mutations produce durable receipts. Neither request payloads (including
// file paths/content) nor result payloads are retained outside live memory.
type featureReceipt struct {
	ID                string                                    `json:"id"`
	Target            contract.FeatureTarget                    `json:"target"`
	Root              string                                    `json:"root,omitempty"`
	Action            string                                    `json:"action"`
	Deadline          time.Time                                 `json:"deadline"`
	Status            string                                    `json:"status"`
	CancelRequested   bool                                      `json:"cancel_requested,omitempty"`
	ResultDigest      string                                    `json:"result_digest,omitempty"`
	RecoveryRequested bool                                      `json:"recovery_requested,omitempty"`
	Reconciled        bool                                      `json:"reconciled,omitempty"`
	RecoveryChoice    string                                    `json:"recovery_choice,omitempty"`
	RecoveryResource  string                                    `json:"recovery_resource,omitempty"`
	RecoveryError     string                                    `json:"recovery_error,omitempty"`
	Supervised        bool                                      `json:"supervised,omitempty"`
	Scope             *process.ScopeReference                   `json:"scope,omitempty"`
	ExecutionStopped  bool                                      `json:"execution_stopped,omitempty"`
	Docker            *infrastructure.ContainerLifecycleReceipt `json:"docker,omitempty"`
	HostRecovery      *contract.HostTerminalRecovery            `json:"host_recovery,omitempty"`
}

type featureLedger struct {
	Schema  string                    `json:"schema"`
	Records map[string]featureReceipt `json:"records"`
}

func (receipt featureReceipt) validate() error {
	if receipt.HostRecovery != nil && (receipt.Action != "terminal.host-recover" || receipt.HostRecovery.Validate() != nil || receipt.HostRecovery.Root != receipt.Root ||
		receipt.HostRecovery.Reference.Target.Node != receipt.Target.Node || receipt.HostRecovery.Reference.Target.Fingerprint != receipt.Target.Fingerprint) {
		return errors.New("invalid host terminal recovery metadata")
	}
	if receipt.Scope != nil && (!receipt.Supervised || receipt.Scope.Validate() != nil) || receipt.ExecutionStopped && !receipt.Supervised ||
		receipt.RecoveryError != "" && recoveryFailureMessage(receipt.RecoveryError) == "" {
		return errors.New("invalid native lifecycle receipt")
	}
	if receipt.Scope != nil && receipt.Scope.Mode != "host-reboot" && filepath.Base(receipt.Scope.Path) != "dockpipe-feature-"+receipt.ID {
		return errors.New("native scope does not belong to the captured operation")
	}
	if receipt.Docker != nil && (!receipt.Supervised || !validMachineNonce(receipt.Docker.Container) || !validMachineNonce(receipt.Docker.Provider) || !validMachineNonce(receipt.Docker.Daemon)) {
		return errors.New("invalid Docker lifecycle receipt")
	}
	if receipt.Reconciled && (receipt.Status != "unknown" || !receipt.RecoveryRequested || !receipt.CancelRequested) {
		return errors.New("invalid reconciled feature receipt")
	}
	if receipt.RecoveryChoice != "" && receipt.RecoveryChoice != "verify" && receipt.RecoveryChoice != "retire-container" ||
		receipt.RecoveryResource != "" && !validMachineNonce(receipt.RecoveryResource) {
		return errors.New("invalid feature recovery metadata")
	}
	if !validMachineNonce(receipt.ID) || receipt.Target.Validate() != nil || receipt.Deadline.IsZero() ||
		contract.FeatureAccess(receipt.Action) == "" || contract.FeatureAccess(receipt.Action) == "read" ||
		receipt.Root != "" && !contract.ValidID(receipt.Root) || receipt.ResultDigest != "" && !validMachineNonce(receipt.ResultDigest) {
		return errors.New("invalid private feature receipt")
	}
	if contract.FeatureUsesRoot(receipt.Action) {
		if !contract.ValidID(receipt.Root) {
			return errors.New("invalid private feature root receipt")
		}
	} else if receipt.Root != "" {
		return errors.New("container receipt cannot select a file root")
	}
	switch receipt.Status {
	case "queued", "running", "unknown", "success", "failure", "cancelled":
		return nil
	default:
		return errors.New("invalid private feature receipt status")
	}
}

func (receipt featureReceipt) requestMetadata() contract.FeatureRequest {
	return contract.FeatureRequest{Schema: contract.FeatureVersion, ID: receipt.ID, Target: receipt.Target,
		Root: receipt.Root, Action: receipt.Action, Deadline: receipt.Deadline}
}

func featureResultDigest(result *contract.FeatureResult) string {
	if result == nil {
		return ""
	}
	encoded, _ := json.Marshal(result)
	return Hash(string(encoded))
}

func receiptFor(request contract.FeatureRequest, status string, cancel bool, digest string) featureReceipt {
	return featureReceipt{ID: request.ID, Target: request.Target, Root: request.Root, Action: request.Action,
		Deadline: request.Deadline, Status: status, CancelRequested: cancel, ResultDigest: digest}
}

func recoveredFeatureResult(status string) contract.FeatureResult {
	if status == "queued" || status == "running" {
		status = "unknown"
	}
	return contract.FeatureResult{Status: status, Error: "Only a durable mutation receipt remains; refresh state. Input and result content were not retained."}
}

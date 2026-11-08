package remote

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

// Only mutations produce durable receipts. Neither request payloads (including
// file paths/content) nor result payloads are retained outside live memory.
type featureReceipt struct {
	ID              string                 `json:"id"`
	Target          contract.FeatureTarget `json:"target"`
	Root            string                 `json:"root,omitempty"`
	Action          string                 `json:"action"`
	Deadline        time.Time              `json:"deadline"`
	Status          string                 `json:"status"`
	CancelRequested bool                   `json:"cancel_requested,omitempty"`
	ResultDigest    string                 `json:"result_digest,omitempty"`
}

type featureLedger struct {
	Schema  string                    `json:"schema"`
	Records map[string]featureReceipt `json:"records"`
}

func (receipt featureReceipt) validate() error {
	if !validMachineNonce(receipt.ID) || receipt.Target.Validate() != nil || receipt.Deadline.IsZero() ||
		contract.FeatureAccess(receipt.Action) == "" || contract.FeatureAccess(receipt.Action) == "read" ||
		receipt.Root != "" && !contract.ValidID(receipt.Root) || receipt.ResultDigest != "" && !validMachineNonce(receipt.ResultDigest) {
		return errors.New("invalid private feature receipt")
	}
	if strings.HasPrefix(receipt.Action, "files.") || strings.HasPrefix(receipt.Action, "git.") {
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

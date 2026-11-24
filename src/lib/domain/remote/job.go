package remote

import (
	"errors"
	"time"
)

type Job struct {
	Submission
	Status                      string    `json:"status"`
	Session                     string    `json:"session,omitempty"`
	CancelRequested             bool      `json:"cancel_requested"`
	CreatedAt                   time.Time `json:"created_at"`
	Result                      *Result   `json:"result,omitempty"`
	ResultHash                  string    `json:"result_hash,omitempty"`
	InterruptionAcknowledgement string    `json:"-"`
}

func (j Job) IsInterruptedAssignment() bool {
	return j.Status == "unknown" && len(j.Session) == 64 && ValidID(j.Session)
}

func (j Job) HasInterruptedResult() bool {
	return j.IsInterruptedAssignment() && featureDigest(j.ResultHash)
}

func (j Job) HasAcknowledgedInterruption() bool {
	return j.IsInterruptedAssignment() && (j.ResultHash == "" || featureDigest(j.ResultHash)) && featureDigest(j.InterruptionAcknowledgement)
}

func (j Job) Resolved() bool {
	return j.Status == "success" || j.Status == "failure" || j.Status == "cancelled" || j.HasAcknowledgedInterruption()
}

// Recover preserves assignment identity across a broker restart. Claimed work
// may have executed, so it cannot be returned to the queue automatically.
func (j Job) Recover() Job {
	if j.Status == "running" {
		j.Status = "unknown"
	}
	return j
}

func (j Job) Cancel() (job Job, changed bool) {
	if j.ResultHash != "" || j.HasAcknowledgedInterruption() {
		return j, false
	}
	j.CancelRequested = true
	if j.Status == "queued" {
		j.Status = "cancelled"
	}
	return j, true
}

// Complete evaluates a delivery using the adapter's content digest. It returns
// metadata only; the adapter must durably store the payload before that metadata.
// Exact retries succeed without changing the original outcome or assignment.
func (j Job) Complete(result *Result, digest string) (job Job, changed bool, err error) {
	if result == nil {
		return Job{}, false, errors.New("result required")
	}
	if j.ResultHash != "" {
		if digest != j.ResultHash {
			return Job{}, false, errors.New("conflicting result")
		}
		return j, false, nil
	}
	if j.HasAcknowledgedInterruption() {
		return Job{}, false, errors.New("the operator closed this assignment without a worker result")
	}
	if j.Status != "running" && j.Status != "unknown" {
		return Job{}, false, errors.New("job is not assigned")
	}
	if err := result.Validate(); err != nil {
		return Job{}, false, err
	}
	j.Status = result.Status
	j.ResultHash = digest
	j.Result = nil
	return j, true, nil
}

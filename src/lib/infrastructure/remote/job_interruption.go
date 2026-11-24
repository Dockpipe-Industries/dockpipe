package remote

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func (b *Broker) jobInterruptionToken(job contract.Job) (string, error) {
	purpose := "dockpipe.job-interruption/v1"
	if job.ResultHash == "" && job.IsInterruptedAssignment() {
		purpose = "dockpipe.inactive-guest-without-result/v1"
	} else if !job.HasInterruptedResult() {
		return "", errors.New("job does not have a reviewable interrupted assignment")
	}

	// Only immutable assignment and result fields participate. Acknowledging the
	// interruption keeps this token stable, including across broker restarts.
	binding := struct {
		Broker     string              `json:"broker"`
		Submission contract.Submission `json:"submission"`
		Session    string              `json:"session"`
		CreatedAt  time.Time           `json:"created_at"`
		ResultHash string              `json:"result_hash"`
	}{b.state.Identity.PublicKey, job.Submission, job.Session, job.CreatedAt.UTC(), job.ResultHash}
	data, err := json.Marshal(binding)
	if err != nil {
		return "", err
	}
	return Hash(purpose + "\x00" + string(data)), nil
}

func (b *Broker) verifyInterruptionEvidence(job contract.Job) error {
	if job.ResultHash == "" {
		// A crash may have published the immutable result before its digest
		// reached broker.json. Never acknowledge its absence in that case.
		path, err := b.resultPath(job.ID)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			if err != nil {
				return err
			}
			return errors.New("stored worker result requires reconciliation before review")
		}
		return nil
	}

	retained, err := b.withResult(job)
	if err != nil {
		return err
	}
	if retained.Result == nil || retained.Result.Status != "unknown" || retained.Result.Validate() != nil {
		return errors.New("interrupted result evidence is unavailable or invalid")
	}
	return nil
}

func (b *Broker) jobInterruption(raw []byte, acknowledge bool) (contract.JobInterruptionReview, error) {
	var request contract.JobInterruptionRequest
	var review contract.JobInterruptionReview
	if err := Decode(raw, &request); err != nil {
		return review, errors.New("invalid job interruption request")
	}
	if err := request.Validate(acknowledge); err != nil {
		return review, err
	}

	job, _, err := b.lookupJob(request.ID)
	if err != nil {
		return review, err
	}
	if job.Node != request.Node {
		return review, errors.New("job interruption does not match the reviewed node and job")
	}
	wasRunning := job.Status == "running"
	if job.ResultHash == "" && !job.HasAcknowledgedInterruption() {
		enrollment, exists := b.state.Nodes[job.Node]
		if !exists || enrollment.GuestGrant == "" || b.enrollmentActive(enrollment, time.Now()) {
			return review, errors.New("assignment requires a retained worker result while its identity can deliver")
		}
		// Inspect is read-only. Acknowledge rechecks the enrollment and token
		// under the broker lock before committing the interrupted status.
		job = job.Recover()
	}
	token, err := b.jobInterruptionToken(job)
	if err != nil {
		return review, err
	}
	if acknowledge && request.ReviewToken != token {
		return review, errors.New("job interruption changed since review")
	}

	if err := b.verifyInterruptionEvidence(job); err != nil {
		return review, err
	}
	if acknowledge {
		if job.HasAcknowledgedInterruption() {
			if err := b.verifyJobInterruptionAcknowledgement(job.ID, token); err != nil {
				b.poisoned = true
				return review, err
			}
		} else {
			if wasRunning {
				// Publish the unknown job before the sidecar. A crash between
				// these writes leaves the original assignment held for review.
				b.state.Jobs[job.ID] = job
				if err := b.save(); err != nil {
					return review, err
				}
				delete(b.liveLogs, job.ID)
			}
			if err := b.storeJobInterruptionAcknowledgement(job.ID, token); err != nil {
				b.poisoned = true
				return review, err
			}
			job.InterruptionAcknowledgement = token
			b.state.Jobs[job.ID] = job
		}
	}

	review = contract.JobInterruptionReview{
		ID: job.ID, Node: job.Node, Status: job.Status, ResultHash: job.ResultHash,
		ReviewToken: token, Acknowledged: job.HasAcknowledgedInterruption(),
	}
	if job.ResultHash == "" {
		review.Evidence = contract.InactiveGuestWithoutResult
	}
	return review, nil
}

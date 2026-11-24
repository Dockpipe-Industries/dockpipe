package remote

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
)

const maxJobInterruptionAcknowledgementBytes = 1024

type jobInterruptionAcknowledgement struct {
	Schema      int    `json:"schema"`
	ID          string `json:"id"`
	ReviewToken string `json:"review_token"`
}

// Transitional brokers wrote the acknowledgement inline. Only startup accepts
// that old encoding. Ordinary saves and job responses leave the field private.
func readBrokerStateForStartup(path string) (BrokerState, error) {
	var stored struct {
		BrokerState
		Jobs map[string]struct {
			contract.Job
			Acknowledgement json.RawMessage `json:"interruption_acknowledgement"`
		} `json:"jobs"`
	}
	if err := ReadPrivate(path, &stored); err != nil {
		return BrokerState{}, err
	}
	if len(stored.Jobs) > contract.MaxJobs {
		return BrokerState{}, errors.New("invalid broker job count")
	}
	if stored.Jobs != nil {
		stored.BrokerState.Jobs = make(map[string]contract.Job, len(stored.Jobs))
		for id, entry := range stored.Jobs {
			job := entry.Job
			if len(entry.Acknowledgement) != 0 {
				if err := json.Unmarshal(entry.Acknowledgement, &job.InterruptionAcknowledgement); err != nil || !validMachineNonce(job.InterruptionAcknowledgement) {
					return BrokerState{}, errors.New("invalid legacy job interruption acknowledgement")
				}
			}
			stored.BrokerState.Jobs[id] = job
		}
	}
	return stored.BrokerState, nil
}

func (b *Broker) jobInterruptionAcknowledgementPath(id string) (string, error) {
	if !contract.ValidID(id) || !filepath.IsLocal(id) {
		return "", errors.New("invalid interruption acknowledgement job ID")
	}
	// Use the existing broker directory so the exclusive write flushes the
	// sidecar name without depending on a newly created parent directory.
	return filepath.Join(filepath.Dir(b.path), "job-interruption-acknowledgement-"+id+".json"), nil
}

func (b *Broker) readJobInterruptionAcknowledgement(id string) (jobInterruptionAcknowledgement, error) {
	var record jobInterruptionAcknowledgement
	path, err := b.jobInterruptionAcknowledgementPath(id)
	if err != nil {
		return record, err
	}
	if err := readPrivateBounded(path, &record, maxJobInterruptionAcknowledgementBytes); err != nil {
		return record, err
	}
	if record.Schema != 1 || record.ID != id || !validMachineNonce(record.ReviewToken) {
		return record, errors.New("invalid job interruption acknowledgement record")
	}
	return record, nil
}

func (b *Broker) storeJobInterruptionAcknowledgement(id, token string) error {
	path, err := b.jobInterruptionAcknowledgementPath(id)
	if err != nil {
		return err
	}
	record := jobInterruptionAcknowledgement{Schema: 1, ID: id, ReviewToken: token}
	if err := infrastructure.WritePrivateJSONExclusive(path, record); err != nil && !os.IsExist(err) {
		return err
	}
	return b.verifyJobInterruptionAcknowledgement(id, token)
}

func (b *Broker) verifyJobInterruptionAcknowledgement(id, token string) error {
	record, err := b.readJobInterruptionAcknowledgement(id)
	if err != nil {
		return err
	}
	if record.ReviewToken != token {
		return errors.New("stored job interruption acknowledgement does not match the reviewed job")
	}
	return nil
}

func (b *Broker) restoreJobInterruptionAcknowledgement(job *contract.Job) error {
	inline := job.InterruptionAcknowledgement
	record, err := b.readJobInterruptionAcknowledgement(job.ID)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		return err
	}
	if missing && inline == "" {
		return nil
	}

	token, err := b.jobInterruptionToken(*job)
	if err != nil || (inline != "" && (!job.HasInterruptedResult() || inline != token)) || (!missing && record.ReviewToken != token) {
		return errors.New("invalid stored job interruption acknowledgement")
	}
	if err := b.verifyInterruptionEvidence(*job); err != nil {
		return err
	}
	if inline != "" {
		// Publish the sidecar before NewBroker saves metadata without the old field.
		// The exclusive retry also flushes an exact overlap from an interrupted
		// publication before migration drops the inline copy.
		if err := b.storeJobInterruptionAcknowledgement(job.ID, token); err != nil {
			return err
		}
	}
	job.InterruptionAcknowledgement = token
	return nil
}

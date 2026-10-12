package remote

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	contract "dockpipe/src/lib/domain/remote"
)

func resultHash(result *contract.Result) (string, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return Hash(string(data)), nil
}

// Result payloads are immutable, separately bounded files. The broker commits
// their digest only after durable publication, keeping metadata independent of
// log and artifact volume. Job IDs are retained to prevent duplicate execution.
func (b *Broker) storeResult(job *contract.Job, result *contract.Result) error {
	if err := job.Validate(); err != nil {
		return err
	}
	if err := result.Validate(); err != nil {
		return err
	}
	path, err := b.resultPath(job.ID)
	if err != nil {
		return err
	}
	digest, err := resultHash(result)
	if err != nil {
		return err
	}
	var existing contract.Result
	if err := ReadPrivate(path, &existing); err == nil {
		existingDigest, err := resultHash(&existing)
		if err != nil {
			return err
		}
		if existingDigest != digest {
			return errors.New("stored result conflicts with delivery")
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if err := WritePrivate(path, result); err != nil {
		return err
	}
	job.ResultHash = digest
	job.Result = nil
	return nil
}

func (b *Broker) withResult(job contract.Job) (contract.Job, error) {
	if job.ResultHash == "" {
		return job, nil
	}
	if err := job.Validate(); err != nil {
		return contract.Job{}, err
	}
	var result contract.Result
	path, err := b.resultPath(job.ID)
	if err != nil {
		return contract.Job{}, err
	}
	if err := ReadPrivate(path, &result); err != nil {
		return contract.Job{}, err
	}
	digest, err := resultHash(&result)
	if err != nil {
		return contract.Job{}, err
	}
	if digest != job.ResultHash {
		return contract.Job{}, errors.New("stored result digest mismatch")
	}
	job.Result = &result
	return job, nil
}

// resultPath enforces the filesystem contract at the storage boundary as well as
// the domain ID validation: a job ID must be exactly one local path component.
func (b *Broker) resultPath(id string) (string, error) {
	if !filepath.IsLocal(id) || strings.ContainsAny(id, `/\`) || !contract.ValidID(id) {
		return "", errors.New("result job ID must be a single local path component")
	}
	return filepath.Join(filepath.Dir(b.path), "results", id+".json"), nil
}

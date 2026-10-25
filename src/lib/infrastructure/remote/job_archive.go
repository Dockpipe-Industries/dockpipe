package remote

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
)

const maxArchivedJobBytes = 16 << 10

type archivedJob struct {
	Schema int          `json:"schema"`
	Job    contract.Job `json:"job"`
	SHA256 string       `json:"sha256"`
}

func terminalJob(job contract.Job) bool {
	return job.Status == "success" || job.Status == "failure" || job.Status == "cancelled"
}

func jobMetadataHash(job contract.Job) (string, error) {
	if err := job.Validate(); err != nil {
		return "", err
	}
	if !terminalJob(job) || job.Result != nil {
		return "", errors.New("job archive requires terminal metadata without a result payload")
	}
	if job.Session != "" && (len(job.Session) != 64 || !contract.ValidID(job.Session)) {
		return "", errors.New("invalid archived worker session")
	}
	if job.ResultHash != "" {
		decoded, err := hex.DecodeString(job.ResultHash)
		if err != nil || len(decoded) != 32 || strings.ToLower(job.ResultHash) != job.ResultHash {
			return "", errors.New("invalid archived result digest")
		}
	}
	data, err := json.Marshal(job)
	if err != nil {
		return "", err
	}
	return Hash(string(data)), nil
}

func (b *Broker) archivedJobPath(id string) (string, error) {
	if !filepath.IsLocal(id) || strings.ContainsAny(id, `/\`) || !contract.ValidID(id) {
		return "", errors.New("archived job ID must be one local path component")
	}
	directory := filepath.Join(filepath.Dir(b.path), "terminal-jobs")
	if _, err := os.Lstat(directory); err == nil {
		if err := infrastructure.ValidatePrivatePath(directory, true); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return filepath.Join(directory, id+".json"), nil
}

func (b *Broker) readArchivedJob(id string) (contract.Job, error) {
	path, err := b.archivedJobPath(id)
	if err != nil {
		return contract.Job{}, err
	}
	var record archivedJob
	if err := readPrivateBounded(path, &record, maxArchivedJobBytes); err != nil {
		return contract.Job{}, err
	}
	digest, err := jobMetadataHash(record.Job)
	if err != nil {
		return contract.Job{}, err
	}
	if record.Schema != 1 || record.Job.ID != id || digest != record.SHA256 {
		return contract.Job{}, errors.New("archived job identity or digest mismatch")
	}
	return record.Job, nil
}

// Lookup consults archived identities even when the hot record still exists.
// A crash after archive publication but before broker publication can leave both;
// only identical overlap is safe, and neither copy permits another execution.
func (b *Broker) lookupJob(id string) (contract.Job, bool, error) {
	archived, err := b.readArchivedJob(id)
	hot, exists := b.state.Jobs[id]
	if err == nil {
		if exists {
			digest, hashErr := jobMetadataHash(hot)
			archivedDigest, archivedErr := jobMetadataHash(archived)
			if hashErr != nil || archivedErr != nil || digest != archivedDigest {
				return contract.Job{}, false, errors.New("hot job conflicts with immutable archive")
			}
		}
		return archived, true, nil
	}
	if !os.IsNotExist(err) {
		return contract.Job{}, false, err
	}
	if exists {
		return hot, false, nil
	}
	return contract.Job{}, false, os.ErrNotExist
}

// Immutable publication uses an exclusive hard link of a completely flushed
// private temporary file. A partial write is never visible as an archive record;
// an existing destination can only be accepted after full identity validation.
func (b *Broker) storeArchivedJob(job contract.Job) error {
	digest, err := jobMetadataHash(job)
	if err != nil {
		return err
	}
	path, err := b.archivedJobPath(job.ID)
	if err != nil {
		return err
	}
	if err := PrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := json.Marshal(archivedJob{Schema: 1, Job: job, SHA256: digest})
	if err != nil {
		return err
	}
	if len(data) > maxArchivedJobBytes {
		return errors.New("archived job metadata exceeds its 16 KiB bound")
	}
	if err := infrastructure.WritePrivateJSONExclusive(path, archivedJob{Schema: 1, Job: job, SHA256: digest}); err != nil && !os.IsExist(err) {
		return fmt.Errorf("publish terminal job archive: %w", err)
	}
	stored, err := b.readArchivedJob(job.ID)
	if err != nil {
		return err
	}
	storedDigest, err := jobMetadataHash(stored)
	if err != nil || storedDigest != digest {
		return errors.New("immutable job archive conflicts with terminal job")
	}
	return nil
}

// Retire at most one oldest terminal job for one new admission. No directory
// scan or active-job eviction is needed, and the live map is untouched until
// its replacement and the downgrade guard can be published together.
func (b *Broker) jobsForAdmission() (map[string]contract.Job, bool, error) {
	jobs := make(map[string]contract.Job, len(b.state.Jobs)+1)
	for id, job := range b.state.Jobs {
		jobs[id] = job
	}
	if len(jobs) < contract.MaxJobs {
		return jobs, false, nil
	}
	var oldest *contract.Job
	for _, job := range jobs {
		if terminalJob(job) && (oldest == nil || job.CreatedAt.Before(oldest.CreatedAt) ||
			(job.CreatedAt.Equal(oldest.CreatedAt) && job.ID < oldest.ID)) {
			selected := job
			oldest = &selected
		}
	}
	if oldest == nil {
		return nil, false, errors.New("all 128 recent jobs are unresolved; no terminal job can be archived")
	}
	if err := b.storeArchivedJob(*oldest); err != nil {
		b.poisoned = true
		return nil, false, err
	}
	delete(jobs, oldest.ID)
	return jobs, true, nil
}

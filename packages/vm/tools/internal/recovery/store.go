package recovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var safeRunID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,127}$`)

type FileStore struct{ Root string }

func (s FileStore) path(runID string) (string, error) {
	if !filepath.IsAbs(s.Root) || !safeRunID.MatchString(runID) {
		return "", fmt.Errorf("file store requires an absolute root and safe run id")
	}
	return filepath.Join(filepath.Clean(s.Root), runID+".json"), nil
}

func (s FileStore) Load(runID string) (Ticket, bool, error) {
	var ticket Ticket
	path, err := s.path(runID)
	if err != nil {
		return ticket, false, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ticket, false, nil
	}
	if err != nil {
		return ticket, false, err
	}
	if err := json.Unmarshal(b, &ticket); err != nil {
		return ticket, false, err
	}
	return ticket, true, nil
}

func (s FileStore) Save(ticket Ticket) error {
	path, err := s.path(ticket.Identity.TrialID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(ticket)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	f, err := os.OpenFile(tmp, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s FileStore) Delete(runID string) error {
	path, err := s.path(runID)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

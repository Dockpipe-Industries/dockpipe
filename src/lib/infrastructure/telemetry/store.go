package telemetry

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Store keeps one bounded recent-history snapshot. The lock is nonblocking:
// observation must never serialize or prevent actual execution.
type Store struct{ Directory string }

func validateStorePath(path string) error {
	current, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return errors.New("invalid execution history path")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func (store Store) open() (*os.Root, error) {
	if err := validateStorePath(store.Directory); err != nil {
		return nil, err
	}
	info, err := os.Lstat(store.Directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid execution history directory")
	}
	return os.OpenRoot(store.Directory)
}

func readRecords(root *os.Root) ([]Execution, error) {
	info, err := root.Lstat("snapshot.json")
	if os.IsNotExist(err) {
		return []Execution{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxSnapshotBytes {
		return nil, errors.New("invalid execution history file")
	}
	file, err := root.Open("snapshot.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxSnapshotBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxSnapshotBytes {
		return nil, errors.New("execution history size exceeded")
	}
	var records []Execution
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	if len(records) > MaxExecutions {
		return nil, errors.New("execution history capacity exceeded")
	}
	return records, nil
}

func (store Store) Read() ([]Execution, error) {
	root, err := store.open()
	if os.IsNotExist(err) {
		return []Execution{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readRecords(root)
}

func (store Store) Record(record Execution) error {
	if record.ID == "" {
		return errors.New("execution ID required")
	}
	if err := validateStorePath(store.Directory); err != nil {
		return err
	}
	if err := os.MkdirAll(store.Directory, 0700); err != nil {
		return err
	}
	root, err := store.open()
	if err != nil {
		return err
	}
	defer root.Close()
	if info, err := root.Lstat("lock"); err == nil && !info.Mode().IsRegular() {
		return errors.New("invalid execution history lock")
	}
	lock, err := root.OpenFile("lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := tryLock(lock); err != nil {
		return ErrBusy
	}
	records, err := readRecords(root)
	if err != nil {
		return err
	}
	found := false
	for index := range records {
		if records[index].ID == record.ID {
			records[index] = record
			found = true
			break
		}
	}
	if !found {
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].StartedAt.After(records[j].StartedAt) })
	if len(records) > MaxExecutions {
		records = records[:MaxExecutions]
	}
	data, err := json.Marshal(records)
	if err != nil {
		return err
	}
	if len(data) > MaxSnapshotBytes {
		return errors.New("execution history size exceeded")
	}
	// The fixed staging file is protected by the same cross-process lock.
	file, err := root.OpenFile("snapshot.tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		if err := root.Remove("snapshot.tmp"); err != nil {
			return err
		}
		file, err = root.OpenFile("snapshot.tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	}
	if err != nil {
		return err
	}
	defer root.Remove("snapshot.tmp")
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename("snapshot.tmp", "snapshot.json")
}

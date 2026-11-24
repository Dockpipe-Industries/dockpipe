package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

const (
	environmentInstanceSchema   = "dockpipe.environment.instance/v1"
	environmentInstanceKey      = "DOCKPIPE_ENVIRONMENT_INSTANCE_ID"
	environmentStateWorkdirKey  = "DOCKPIPE_ENVIRONMENT_STATE_WORKDIR"
	maxEnvironmentInstanceBytes = 32 << 10
)

var environmentInstanceID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type environmentInstanceRecord struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
}

func environmentInstanceForDefinition(definitionPath string) (environmentInstanceRecord, string, error) {
	recordPath, err := environmentInstanceRecordPath(definitionPath)
	if err != nil {
		return environmentInstanceRecord{}, "", err
	}
	record, err := loadOrCreateEnvironmentInstance(recordPath)
	if err != nil {
		return environmentInstanceRecord{}, "", err
	}

	stateWorkdir, err := infrastructure.PreparePrivateStateSubdirectory(filepath.Dir(recordPath), "runtime/"+record.ID)
	if err != nil {
		return environmentInstanceRecord{}, "", err
	}

	return record, stateWorkdir, nil
}

func environmentInstanceRecordPath(definitionPath string) (string, error) {
	absolute, err := filepath.Abs(definitionPath)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		canonical = strings.ToLower(canonical)
	}
	identity := sha256.Sum256([]byte(canonical))

	root, err := infrastructure.GlobalDockpipeDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, "environment-instances", hex.EncodeToString(identity[:])+".json"), nil
}

func loadOrCreateEnvironmentInstance(recordPath string) (environmentInstanceRecord, error) {
	record, err := readEnvironmentInstance(recordPath)
	if !errors.Is(err, os.ErrNotExist) {
		return record, err
	}

	var id [16]byte
	rand.Read(id[:])
	record = environmentInstanceRecord{
		Schema: environmentInstanceSchema,
		ID:     hex.EncodeToString(id[:]),
	}
	if err := infrastructure.WritePrivateJSONExclusive(recordPath, record); err != nil {
		if errors.Is(err, os.ErrExist) {
			return readEnvironmentInstance(recordPath)
		}
		return environmentInstanceRecord{}, err
	}

	return record, nil
}

func readEnvironmentInstance(path string) (environmentInstanceRecord, error) {
	var record environmentInstanceRecord
	if err := infrastructure.ValidatePrivatePath(path, false); err != nil {
		return record, err
	}
	file, err := os.Open(path)
	if err != nil {
		return record, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxEnvironmentInstanceBytes+1))
	if err != nil || len(data) > maxEnvironmentInstanceBytes {
		return record, errors.New("cannot read the bounded environment instance record")
	}
	if err := decodeStorageJSON(data, &record); err != nil || record.Schema != environmentInstanceSchema ||
		!environmentInstanceID.MatchString(record.ID) {
		return environmentInstanceRecord{}, errors.New("invalid environment instance record")
	}

	return record, nil
}

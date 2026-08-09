package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"

	"dockpipe.vm/tools/internal/manifest"
	"dockpipe.vm/tools/internal/provisioning"
)

const (
	retainedGate3Directory        = "gate3-inputs"
	retainedProvisioningFilename  = "provisioning-contract.json"
	retainedQualificationFilename = "qualification-manifest.json"
	retainedGate3DirectoryMode    = os.FileMode(0o700)
	retainedGate3InputMode        = os.FileMode(0o600)
)

type RetainedGate3InputPaths struct {
	Root          string
	Provisioning  string
	Qualification string
}

type retainedInputWriter func(string, []byte, os.FileMode) error

func Gate3RetainedInputPaths(execution Contract) (RetainedGate3InputPaths, error) {
	var paths RetainedGate3InputPaths
	if err := execution.Validate(); err != nil || execution.Schema != Schema || execution.ProvisioningRoots == nil {
		return paths, fmt.Errorf("retained Gate 3 inputs require the current executor-v10 contract")
	}
	configRoot := filepath.Join(execution.ProvisioningRoots.Config, "instances", execution.RunID, execution.CohortID)
	if !filepath.IsAbs(configRoot) || filepath.Clean(configRoot) != configRoot {
		return paths, fmt.Errorf("retained Gate 3 input root is invalid")
	}
	paths.Root = filepath.Join(configRoot, retainedGate3Directory)
	paths.Provisioning = filepath.Join(paths.Root, retainedProvisioningFilename)
	paths.Qualification = filepath.Join(paths.Root, retainedQualificationFilename)
	return paths, nil
}

// StoreRetainedGate3Inputs durably preserves the exact validated Gate 2 input
// bytes. It deliberately performs no rollback: any failure leaves every root
// and file already created available for inspection.
func StoreRetainedGate3Inputs(execution Contract, contract provisioning.Contract, contractJSON []byte, qualification manifest.Manifest, qualificationJSON []byte) (RetainedGate3InputPaths, error) {
	return storeRetainedGate3Inputs(execution, contract, contractJSON, qualification, qualificationJSON, durableRetainedInput)
}

func storeRetainedGate3Inputs(execution Contract, contract provisioning.Contract, contractJSON []byte, qualification manifest.Manifest, qualificationJSON []byte, write retainedInputWriter) (RetainedGate3InputPaths, error) {
	paths, err := Gate3RetainedInputPaths(execution)
	if err != nil {
		return paths, err
	}
	if write == nil {
		return paths, fmt.Errorf("retained Gate 3 input writer is required")
	}
	if err := validateRetainedInputBytes(execution, contract, contractJSON, qualification, qualificationJSON); err != nil {
		return paths, err
	}
	parent := filepath.Dir(paths.Root)
	if err := validateRetainedDirectory(parent, os.Geteuid()); err != nil {
		return paths, fmt.Errorf("retained Gate 3 config root: %w", err)
	}
	if err := os.Mkdir(paths.Root, retainedGate3DirectoryMode); err != nil {
		return paths, fmt.Errorf("exclusively create retained Gate 3 input root: %w", err)
	}
	if err := validateRetainedDirectory(paths.Root, os.Geteuid()); err != nil {
		return paths, err
	}
	if err := syncRetainedDirectory(parent); err != nil {
		return paths, fmt.Errorf("sync retained Gate 3 input parent: %w", err)
	}
	if err := write(paths.Provisioning, contractJSON, retainedGate3InputMode); err != nil {
		return paths, fmt.Errorf("retain provisioning contract: %w", err)
	}
	if err := write(paths.Qualification, qualificationJSON, retainedGate3InputMode); err != nil {
		return paths, fmt.Errorf("retain qualification manifest: %w", err)
	}
	return paths, nil
}

func LoadRetainedGate3Inputs(execution Contract, provisioningPath, qualificationPath string) (provisioning.Contract, manifest.Manifest, error) {
	return loadRetainedGate3Inputs(execution, provisioningPath, qualificationPath, os.Geteuid())
}

func loadRetainedGate3Inputs(execution Contract, provisioningPath, qualificationPath string, euid int) (provisioning.Contract, manifest.Manifest, error) {
	var contract provisioning.Contract
	var qualification manifest.Manifest
	paths, err := Gate3RetainedInputPaths(execution)
	if err != nil {
		return contract, qualification, err
	}
	if provisioningPath != paths.Provisioning || qualificationPath != paths.Qualification {
		return contract, qualification, fmt.Errorf("Gate 3 v1 requires the deterministic retained provisioning and qualification paths")
	}
	if err := validateRetainedDirectory(paths.Root, euid); err != nil {
		return contract, qualification, fmt.Errorf("retained Gate 3 input root: %w", err)
	}
	contractJSON, err := readRetainedInput(paths.Provisioning, euid)
	if err != nil {
		return contract, qualification, fmt.Errorf("retained provisioning contract: %w", err)
	}
	qualificationJSON, err := readRetainedInput(paths.Qualification, euid)
	if err != nil {
		return contract, qualification, fmt.Errorf("retained qualification manifest: %w", err)
	}
	if err := decodeRetainedExact(contractJSON, &contract); err != nil {
		return contract, qualification, fmt.Errorf("decode retained provisioning contract: %w", err)
	}
	if err := decodeRetainedExact(qualificationJSON, &qualification); err != nil {
		return contract, qualification, fmt.Errorf("decode retained qualification manifest: %w", err)
	}
	if err := qualification.Validate(); err != nil {
		return contract, qualification, err
	}
	if _, err := BuildGate3Plan(execution, contract, qualification); err != nil {
		return contract, qualification, err
	}
	return contract, qualification, nil
}

func validateRetainedInputBytes(execution Contract, contract provisioning.Contract, contractJSON []byte, qualification manifest.Manifest, qualificationJSON []byte) error {
	var decodedContract provisioning.Contract
	if err := decodeRetainedExact(contractJSON, &decodedContract); err != nil {
		return fmt.Errorf("validated provisioning bytes changed: %w", err)
	}
	var decodedQualification manifest.Manifest
	if err := decodeRetainedExact(qualificationJSON, &decodedQualification); err != nil {
		return fmt.Errorf("validated qualification bytes changed: %w", err)
	}
	if !reflect.DeepEqual(decodedContract, contract) || !reflect.DeepEqual(decodedQualification, qualification) {
		return fmt.Errorf("retained Gate 3 bytes do not match the validated inputs")
	}
	if err := decodedQualification.Validate(); err != nil {
		return err
	}
	if _, err := BuildGate3Plan(execution, decodedContract, decodedQualification); err != nil {
		return err
	}
	return nil
}

func decodeRetainedExact(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("contains trailing JSON")
	}
	return nil
}

func readRetainedInput(path string, euid int) ([]byte, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() || pathInfo.Mode().Perm() != retainedGate3InputMode {
		return nil, fmt.Errorf("must be a regular non-symlink mode 0600 file")
	}
	pathStat, ok := pathInfo.Sys().(*syscall.Stat_t)
	if !ok || int(pathStat.Uid) != euid {
		return nil, fmt.Errorf("must be owned by the current user")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	openedStat, ok := openedInfo.Sys().(*syscall.Stat_t)
	if !ok || !os.SameFile(pathInfo, openedInfo) || !openedInfo.Mode().IsRegular() || openedInfo.Mode().Perm() != retainedGate3InputMode || int(openedStat.Uid) != euid {
		return nil, fmt.Errorf("type, mode, or ownership changed while opening")
	}
	return io.ReadAll(file)
}

func validateRetainedDirectory(path string, euid int) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != retainedGate3DirectoryMode {
		return fmt.Errorf("must be a private regular directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != euid {
		return fmt.Errorf("must be owned by the current user")
	}
	return nil
}

func durableRetainedInput(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if written, writeErr := file.Write(data); writeErr != nil {
		err = writeErr
	} else if written != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if _, err := readRetainedInput(path, os.Geteuid()); err != nil {
		return err
	}
	return syncRetainedDirectory(filepath.Dir(path))
}

func syncRetainedDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

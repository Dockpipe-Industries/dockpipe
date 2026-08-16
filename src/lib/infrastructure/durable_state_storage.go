package infrastructure

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func readDurableProjectIndex(root, path string) (durableProjectIndex, error) {
	index := durableProjectIndex{Schema: durableStateSchema, Projects: []durableProjectRecord{}}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return index, nil
	} else if err != nil {
		return index, err
	}
	if err := readPrivateJSON(root, path, &index); err != nil {
		return index, fmt.Errorf("read durable project index: %w", err)
	}
	if index.Schema != durableStateSchema {
		return index, fmt.Errorf("durable project index schema %d is unsupported", index.Schema)
	}
	seenIDs := map[string]struct{}{}
	for _, record := range index.Projects {
		if err := validateDurableProjectRecord(record); err != nil {
			return index, err
		}
		if _, exists := seenIDs[record.ProjectID]; exists {
			return index, fmt.Errorf("durable project index contains duplicate project ID %q", record.ProjectID)
		}
		seenIDs[record.ProjectID] = struct{}{}
	}
	return index, nil
}

func validateExistingProjectMetadata(root, path string, record durableProjectRecord) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	var metadata durableProjectMetadata
	if err := readPrivateJSON(root, path, &metadata); err != nil {
		return fmt.Errorf("read durable project metadata: %w", err)
	}
	if metadata.Schema != durableStateSchema || metadata.ProjectID != record.ProjectID || metadata.FilesystemIdentity != record.FilesystemIdentity {
		return errors.New("durable project metadata does not match its stable identity")
	}
	return nil
}

func validateDurableProjectRecord(record durableProjectRecord) error {
	if len(record.ProjectID) != 32 {
		return errors.New("durable project index contains a malformed project ID")
	}
	if _, err := hex.DecodeString(record.ProjectID); err != nil || strings.ToLower(record.ProjectID) != record.ProjectID {
		return errors.New("durable project index contains a malformed project ID")
	}
	if !filepath.IsAbs(record.CanonicalPath) || strings.TrimSpace(record.FilesystemIdentity) == "" {
		return errors.New("durable project index contains incomplete identity metadata")
	}
	for _, alias := range record.PathAliases {
		if !filepath.IsAbs(alias) {
			return errors.New("durable project index contains a non-absolute path alias")
		}
	}
	return nil
}

func addDurablePathAlias(aliases *[]string, path string) bool {
	for _, alias := range *aliases {
		if sameDurablePath(alias, path) {
			return false
		}
	}
	*aliases = append(*aliases, path)
	sort.Slice(*aliases, func(i, j int) bool { return (*aliases)[i] < (*aliases)[j] })
	return true
}

func removeDurablePathAlias(aliases []string, path string) []string {
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if !sameDurablePath(alias, path) {
			out = append(out, alias)
		}
	}
	return out
}

func durableJSONDiffers(root, path string, value any) (bool, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return false, err
	}
	raw = append(raw, '\n')
	existing, err := readPrivateFile(root, path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !bytes.Equal(existing, raw), nil
}

func readPrivateJSON(root, path string, target any) error {
	raw, err := readPrivateFile(root, path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("metadata contains multiple JSON values")
		}
		return err
	}
	return nil
}

func readPrivateFile(root, path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if durableFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
		return nil, errors.New("durable metadata is linked, reparsed, or not a regular file")
	}
	if err := validateExistingStateBoundary(root, path); err != nil {
		return nil, err
	}
	if err := makePrivatePath(path, false); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("durable metadata changed while being opened")
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	final, err := file.Stat()
	if err != nil || !os.SameFile(opened, final) || int64(len(raw)) != final.Size() {
		return nil, errors.New("durable metadata changed while being read")
	}
	return raw, nil
}

func writePrivateJSONAtomic(root, path string, value any) error {
	if !sameOrWithinDurablePath(root, path) || filepath.Clean(root) == filepath.Clean(path) {
		return errors.New("durable metadata target escapes its root")
	}
	if err := validateExistingStateBoundary(root, filepath.Dir(path)); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if durableFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
			return errors.New("durable metadata target is linked, reparsed, or not a regular file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".dockpipe-state-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := true
	defer func() {
		_ = temporary.Close()
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := makePrivatePath(temporaryPath, false); err != nil {
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replacePrivateFile(temporaryPath, path); err != nil {
		return err
	}
	cleanup = false
	if err := makePrivatePath(path, false); err != nil {
		return err
	}
	return syncPrivateDirectory(filepath.Dir(path))
}

func openPrivateLockFile(root, path string) (*os.File, error) {
	if err := validateExistingStateBoundary(root, filepath.Dir(path)); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
			if os.IsExist(createErr) {
				continue
			}
			if createErr != nil {
				return nil, createErr
			}
			if err := makePrivatePath(path, false); err != nil {
				file.Close()
				return nil, err
			}
			return file, nil
		}
		if err != nil {
			return nil, err
		}
		if durableFileInfoIsLinkOrReparse(info) || !info.Mode().IsRegular() {
			return nil, errors.New("lock path is linked, reparsed, or not a regular file")
		}
		if err := makePrivatePath(path, false); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(path, os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		opened, err := file.Stat()
		if err != nil || !os.SameFile(info, opened) {
			file.Close()
			return nil, errors.New("lock path changed while being opened")
		}
		return file, nil
	}
	return nil, errors.New("lock path changed during creation")
}

func ensurePrivateRoot(path string) error {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(abs)
	rest := strings.TrimPrefix(abs, volume)
	rest = strings.TrimLeft(rest, `/\`)
	current := volume + string(filepath.Separator)
	if volume == "" {
		current = string(filepath.Separator)
	}
	parts := strings.FieldsFunc(rest, func(r rune) bool { return r == '/' || r == '\\' })
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		created := false
		if os.IsNotExist(statErr) {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil && !os.IsExist(mkdirErr) {
				return mkdirErr
			}
			info, statErr = os.Lstat(current)
			created = statErr == nil
		}
		if statErr != nil {
			return statErr
		}
		if durableFileInfoIsLinkOrReparse(info) || !info.IsDir() {
			return fmt.Errorf("path component %q is linked, reparsed, or not a directory", current)
		}
		if created || i == len(parts)-1 {
			if err := makePrivatePath(current, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensurePrivateSubdirectory(root, path string) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if !sameOrWithinDurablePath(root, path) {
		return errors.New("private directory escapes its durable root")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if durableFileInfoIsLinkOrReparse(rootInfo) || !rootInfo.IsDir() {
		return errors.New("durable root is linked, reparsed, or not a directory")
	}
	if err := makePrivatePath(root, true); err != nil {
		return err
	}
	rootDevice, err := durableDeviceIdentity(root)
	if err != nil {
		return err
	}
	rel, _ := filepath.Rel(root, path)
	current := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil && !os.IsExist(mkdirErr) {
				return mkdirErr
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil {
			return statErr
		}
		if durableFileInfoIsLinkOrReparse(info) || !info.IsDir() {
			return fmt.Errorf("path component %q is linked, reparsed, or not a directory", current)
		}
		device, err := durableDeviceIdentity(current)
		if err != nil || device != rootDevice {
			return fmt.Errorf("path component %q crosses a filesystem boundary", current)
		}
		if err := makePrivatePath(current, true); err != nil {
			return err
		}
	}
	return nil
}

func validateExistingStateBoundary(root, candidate string) error {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	if !sameOrWithinDurablePath(root, candidate) {
		return errors.New("state path escapes its selected root")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if durableFileInfoIsLinkOrReparse(rootInfo) || !rootInfo.IsDir() {
		return errors.New("state root is linked, reparsed, or not a directory")
	}
	rootDevice, err := durableDeviceIdentity(root)
	if err != nil {
		return err
	}
	rel, _ := filepath.Rel(root, candidate)
	current := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if durableFileInfoIsLinkOrReparse(info) {
			return fmt.Errorf("state path component %q is linked or reparsed", current)
		}
		device, err := durableDeviceIdentity(current)
		if err != nil || device != rootDevice {
			return fmt.Errorf("state path component %q crosses a filesystem boundary", current)
		}
	}
	return nil
}

func sameOrWithinDurablePath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func durablePathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

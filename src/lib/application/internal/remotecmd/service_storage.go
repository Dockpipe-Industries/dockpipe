package remotecmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// User-service definitions contain one command and its environment. Larger files
// are treated as custom configuration requiring manual review.
const maxServiceDefinitionBytes = 64 << 10

func readServiceDefinition(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxServiceDefinitionBytes {
		return nil, errors.New("service definition must be a regular file no larger than 64 KiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("service definition changed during inspection")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxServiceDefinitionBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxServiceDefinitionBytes {
		return nil, errors.New("service definition exceeds 64 KiB")
	}
	return data, nil
}

func readServiceForReplacement(definition serviceDefinition, replace bool) ([]byte, bool, error) {
	previous, err := readServiceDefinition(definition.path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if string(previous) != definition.content && !replace {
		return nil, true, fmt.Errorf("service registration differs from this Dockpipe installation: %s; review it, then use remote service --role %s --replace to update and restart it", definition.path, definition.role)
	}
	return previous, true, nil
}

func publishServiceDefinition(ctx context.Context, definition serviceDefinition, replace bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	previous, exists, err := readServiceForReplacement(definition, replace)
	if err != nil {
		return err
	}
	if exists && string(previous) == definition.content {
		return nil
	}
	if len(definition.content) > maxServiceDefinitionBytes {
		return errors.New("generated service definition exceeds 64 KiB")
	}
	parent := filepath.Dir(definition.path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(parent, ".dockpipe-service-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	defer file.Close()
	if _, err := file.WriteString(definition.content); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Refuse to overwrite a concurrent editor or a newly introduced symlink.
	current, err := readServiceDefinition(definition.path)
	if exists {
		if err != nil || string(current) != string(previous) {
			return errors.New("service definition changed before publication; inspect and retry")
		}
	} else if !os.IsNotExist(err) {
		return errors.New("service definition appeared before publication; inspect and retry")
	}
	if err := os.Rename(temporary, definition.path); err != nil {
		return err
	}
	directory, err := os.Open(parent)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

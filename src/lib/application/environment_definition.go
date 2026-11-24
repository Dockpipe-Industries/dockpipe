package application

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/model/managedenvironment"

	"gopkg.in/yaml.v3"
)

const maxEnvironmentDefinitionBytes = 64 << 10

func personalEnvironmentPath(name string, createDirectory bool) (string, error) {
	if !managedenvironment.ValidName(name) {
		return "", fmt.Errorf("invalid environment name %q", name)
	}
	directory, err := personalEnvironmentDirectory()
	if err != nil {
		return "", err
	}
	if createDirectory {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return "", err
		}
	}
	return filepath.Join(directory, name+".yaml"), nil
}

func personalEnvironmentDirectory() (string, error) {
	root, err := infrastructure.GlobalDockpipeDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Join(root, "environments"))
}

func readEnvironment(path string) (managedenvironment.Definition, error) {
	info, err := os.Stat(path)
	if err != nil {
		return managedenvironment.Definition{}, err
	}
	if !info.Mode().IsRegular() {
		return managedenvironment.Definition{}, errors.New("environment definition must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return managedenvironment.Definition{}, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxEnvironmentDefinitionBytes+1))
	if err != nil {
		return managedenvironment.Definition{}, err
	}
	if len(data) > maxEnvironmentDefinitionBytes {
		return managedenvironment.Definition{}, errors.New("environment definition exceeds 64 KiB")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var definition managedenvironment.Definition
	if err := decoder.Decode(&definition); err != nil {
		return managedenvironment.Definition{}, fmt.Errorf("read environment definition: %w", err)
	}
	if err := decoder.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return managedenvironment.Definition{}, errors.New("environment definition must contain exactly one YAML document")
	}
	return definition, definition.Validate()
}

func writeEnvironment(w io.Writer, definition managedenvironment.Definition) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return errors.Join(encoder.Encode(definition), encoder.Close())
}

func saveNewEnvironment(path string, definition managedenvironment.Definition) error {
	var encoded bytes.Buffer
	if err := writeEnvironment(&encoded, definition); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	identity, statErr := file.Stat()
	if statErr != nil {
		return errors.Join(statErr, file.Close())
	}
	_, writeErr := encoded.WriteTo(file)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		if current, statErr := os.Lstat(path); statErr == nil && os.SameFile(identity, current) {
			return errors.Join(err, os.Remove(path))
		}
		return err
	}
	return nil
}

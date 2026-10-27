package dependencylifecycle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	"dockpipe/src/lib/domain/dependency"
)

// ReadInventory rejects links, devices, trailing data and unknown wire fields.
// Caller-supplied reports are evidence, not authenticated approval or history.
func ReadInventory(path string) (dependency.Inventory, error) {
	var inventory dependency.Inventory
	before, err := os.Lstat(path)
	if err != nil {
		return inventory, err
	}
	if !before.Mode().IsRegular() || before.Size() > dependency.MaxInventoryBytes {
		return inventory, errors.New("dependency receipt must be a bounded regular file")
	}
	file, err := openReceipt(path)
	if err != nil {
		return inventory, err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return inventory, errors.New("dependency receipt changed before opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, dependency.MaxInventoryBytes+1))
	if err != nil {
		return inventory, err
	}
	if len(data) > dependency.MaxInventoryBytes {
		return inventory, errors.New("dependency receipt exceeds its bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&inventory); err != nil {
		return inventory, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return inventory, errors.New("dependency receipt contains trailing data")
	}
	return inventory, nil
}

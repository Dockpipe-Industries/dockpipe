package orchestrationhelper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dockpipe/src/lib/infrastructure/filepublication"
)

func writeJSONFileAtomic(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("atomic JSON target cannot be a symlink: %s", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	temporaryPath, err := filepublication.Stage(filepath.Dir(path), ".promotion-candidate-*.json", append(raw, '\n'))
	if err != nil {
		return err
	}
	defer os.Remove(temporaryPath)
	return os.Rename(temporaryPath, path)
}

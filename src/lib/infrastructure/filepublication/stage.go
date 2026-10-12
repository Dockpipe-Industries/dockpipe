// Package filepublication shares file staging mechanics. Callers own path
// validation, permissions beyond the temporary file's 0600 mode, replacement,
// directory syncing, and recovery after a publication with uncertain outcome.
package filepublication

import "os"

// Stage writes, syncs, and closes a unique temporary file in directory. On
// failure it removes the temporary file. On success the caller owns that path
// and must remove or publish it. Stage never modifies the eventual destination.
// It is not suitable for files requiring an ACL before the first content write.
func Stage(directory, pattern string, content []byte) (string, error) {
	file, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	path := file.Name()
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	complete = true
	return path, nil
}

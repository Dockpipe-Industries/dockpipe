package remote

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"

	"dockpipe/src/lib/infrastructure"
)

func Secret() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func Decode(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

// PrivateDirectory rejects symlink components and refuses an existing directory
// readable by other users. Do not silently chmod user-owned shared directories.
func PrivateDirectory(path string) error {
	return infrastructure.PreparePrivateDirectory(path)
}

func ReadPrivate(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return errors.New("remote state must be a bounded private regular file")
	}
	if err := infrastructure.ValidatePrivatePath(path, false); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Decode(data, target)
}

// WritePrivate publishes a complete file only after flushing its contents.
func WritePrivate(path string, value any) error {
	return infrastructure.WritePrivateJSON(path, value)
}

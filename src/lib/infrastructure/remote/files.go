package remote

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

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
	return readPrivateBounded(path, target, 64<<20)
}

func readPrivateBounded(path string, target any, maximum int64) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maximum {
		return errors.New("remote state must be a bounded private regular file")
	}
	if err := infrastructure.ValidatePrivatePath(path, false); err != nil {
		return err
	}
	file, err := os.OpenInRoot(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() > maximum || !os.SameFile(info, opened) {
		return errors.New("remote state changed while being opened")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maximum {
		return errors.New("remote state exceeds size limit")
	}
	return Decode(data, target)
}

// WritePrivate publishes a complete file only after flushing its contents.
func WritePrivate(path string, value any) error {
	return infrastructure.WritePrivateJSON(path, value)
}

package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
)

const environmentStorageSchema = "dockpipe.environment.storage/v1"
const maxEnvironmentImageBytes = 64 << 30

var environmentImageDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

type environmentStorageRequest struct {
	Schema string            `json:"schema"`
	Cache  string            `json:"cache"`
	Values map[string]string `json:"values"`
}

type restoredEnvironmentImage struct {
	Schema string `json:"schema"`
	Path   string `json:"base"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type environmentStorageOutput struct{ bytes.Buffer }

func (output *environmentStorageOutput) Write(data []byte) (int, error) {
	if len(data) > maxEnvironmentDefinitionBytes-output.Len() {
		return 0, errors.New("environment storage response exceeds limit")
	}
	return output.Buffer.Write(data)
}

func restoreEnvironmentImage(ctx context.Context, hook, directory, cache string, values map[string]string) (restoredEnvironmentImage, error) {
	var image restoredEnvironmentImage
	input, err := json.Marshal(environmentStorageRequest{Schema: environmentStorageSchema, Cache: cache, Values: values})
	defer clear(input)
	if err != nil || len(input) > 1<<20 {
		return image, errors.New("invalid environment storage request")
	}
	deadline, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	command := exec.CommandContext(deadline, hook)
	if strings.EqualFold(filepath.Ext(hook), ".sh") {
		command = exec.CommandContext(deadline, "bash", hook)
	}
	command.Dir = directory
	command.Env = environmentStorageHookEnviron()
	command.Stdin = bytes.NewReader(input)
	var output environmentStorageOutput
	defer func() { clear(output.Bytes()) }()
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	if err := process.Run(command); err != nil {
		if deadline.Err() != nil {
			return image, deadline.Err()
		}
		return image, errors.New("environment image restore failed (provider output withheld)")
	}
	if err := decodeStorageJSON(output.Bytes(), &image); err != nil || image.Schema != environmentStorageSchema {
		return image, errors.New("invalid environment image receipt (provider output withheld)")
	}
	return image, verifyRestoredImage(deadline, cache, image)
}

func environmentStorageHookEnviron() []string {
	environ := make([]string, 0, 12)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "PATH", "SYSTEMROOT", "WINDIR", "TMP", "TEMP", "TMPDIR", "LANG", "LC_ALL":
			environ = append(environ, entry)
		}
	}
	return environ
}

func verifyRestoredImage(ctx context.Context, cache string, image restoredEnvironmentImage) error {
	if !environmentImageDigest.MatchString(image.SHA256) || image.Bytes <= 0 || image.Bytes > maxEnvironmentImageBytes {
		return errors.New("invalid environment image identity")
	}
	expectedPath := filepath.Join(cache, image.SHA256+".qcow2")
	if !filepath.IsAbs(image.Path) || filepath.Clean(image.Path) != expectedPath {
		return errors.New("environment image must be in its content-addressed cache slot")
	}
	if err := infrastructure.ValidateUnlinkedPath(expectedPath); err != nil {
		return errors.New("environment image cache path is unavailable or linked")
	}
	info, err := os.Lstat(expectedPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o222 != 0 || info.Size() != image.Bytes {
		return errors.New("environment base must be a regular read-only image of the expected size")
	}
	file, err := os.Open(expectedPath)
	if err != nil {
		return errors.New("cannot open the restored environment image")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return errors.New("environment image changed before verification")
	}
	hash := sha256.New()
	reader := &environmentImageReader{context: ctx, reader: file}
	size, err := io.Copy(hash, io.LimitReader(reader, image.Bytes+1))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || size != image.Bytes || hex.EncodeToString(hash.Sum(nil)) != image.SHA256 {
		return errors.New("environment image content differs from its receipt")
	}
	return nil
}

type environmentImageReader struct {
	context context.Context
	reader  io.Reader
}

func (reader *environmentImageReader) Read(buffer []byte) (int, error) {
	if err := reader.context.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

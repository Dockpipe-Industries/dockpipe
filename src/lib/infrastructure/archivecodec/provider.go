package archivecodec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxProviderBytes int64 = 64 << 20
const maxConfigBytes int64 = 1 << 20

type providerFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Name   string `json:"name"`
}

type provider struct {
	Executable string         `json:"executable"`
	SHA256     string         `json:"sha256"`
	Args       []string       `json:"args"`
	Files      []providerFile `json:"files"`
}

func configuredProvider(ctx context.Context, codec string) (provider, error) {
	path := os.Getenv("DOCKPIPE_ARCHIVE_DECODERS")
	if !filepath.IsAbs(path) {
		return provider{}, fmt.Errorf("archive codec %q requires an absolute DOCKPIPE_ARCHIVE_DECODERS config path", codec)
	}
	file, err := openRegular(path)
	if err != nil {
		return provider{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, reader: file}, maxConfigBytes+1))
	if err != nil {
		return provider{}, err
	}
	if int64(len(data)) > maxConfigBytes {
		return provider{}, errors.New("archive decoder config exceeds 1 MiB")
	}
	var config struct {
		Schema    int                 `json:"schema"`
		Providers map[string]provider `json:"providers"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return provider{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return provider{}, errors.New("archive decoder config contains trailing data")
	}
	if config.Schema != 1 {
		return provider{}, errors.New("unsupported archive decoder config schema")
	}
	for name := range config.Providers {
		if _, err := Extension(name); err != nil || name == "gzip" || name == "zstd" {
			return provider{}, fmt.Errorf("invalid external archive decoder ID %q", name)
		}
	}
	selected, ok := config.Providers[codec]
	if !ok {
		return provider{}, fmt.Errorf("archive codec %q has no configured provider", codec)
	}
	return selected, nil
}

func copyPinned(ctx context.Context, source, expected, destination string, budget *int64, executable bool) (resultErr error) {
	if !filepath.IsAbs(source) {
		return errors.New("archive decoder files must use absolute local paths")
	}
	want, err := hex.DecodeString(expected)
	if err != nil || len(want) != sha256.Size {
		return errors.New("archive decoder file requires a SHA-256 pin")
	}
	input, err := openRegular(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > *budget {
		return errors.New("archive decoder files exceed the 64 MiB aggregate limit")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, output.Close()) }()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(output, hash), io.LimitReader(contextReader{ctx: ctx, reader: input}, *budget+1))
	if err != nil {
		return err
	}
	if count > *budget {
		return errors.New("archive decoder files exceed the 64 MiB aggregate limit")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expected) {
		return errors.New("archive decoder file checksum mismatch")
	}
	*budget -= count
	mode := os.FileMode(0o600)
	if executable {
		mode = 0o700
	}
	return output.Chmod(mode)
}

func validStagedName(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
			len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
		for _, char := range part {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._-", char)) {
				return false
			}
		}
	}
	return true
}

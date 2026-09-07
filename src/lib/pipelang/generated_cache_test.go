package pipelang

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const generatedCacheVersion = "pipelang-native-validation-v2"

var generatedToolchain = struct {
	sync.Once
	digest string
	err    error
}{}

func generatedToolchainDigest() (string, error) {
	generatedToolchain.Do(func() {
		hash := sha256.New()
		fmt.Fprintf(hash, "%s\x00%s\x00%s\x00%s\x00", runtime.GOROOT(), runtime.Version(), runtime.GOOS, runtime.GOARCH)
		// Include tool binaries and library source, not just a version label. A
		// toolchain repair at the same path must invalidate executable reuse.
		for _, part := range []string{"bin", "pkg/tool", "src", "go.env", "VERSION"} {
			err := filepath.WalkDir(filepath.Join(runtime.GOROOT(), part), func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, "_test.go") {
					return nil
				}
				if !entry.Type().IsRegular() {
					return fmt.Errorf("nonregular toolchain input %s", path)
				}
				relative, err := filepath.Rel(runtime.GOROOT(), path)
				if err != nil {
					return err
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				fmt.Fprintf(hash, "%d:%s:%d:%d\x00", len(relative), relative, info.Size(), info.Mode())
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(hash, file)
				closeErr := file.Close()
				if err != nil {
					return err
				}
				return closeErr
			})
			if err != nil {
				generatedToolchain.err = err
				return
			}
		}
		generatedToolchain.digest = hex.EncodeToString(hash.Sum(nil))
	})
	return generatedToolchain.digest, generatedToolchain.err
}

type generatedCacheRecord struct{ Version, Key, BinarySHA256 string }

func generatedFileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func generatedArtifactKey(dir string) (string, error) {
	toolchain, err := generatedToolchainDigest()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00go test -c -p=1\x00", generatedCacheVersion, toolchain)
	// Only Go build inputs affect the executable. Oracle fixture bytes are never
	// cached: each invocation writes and reads its current fixtures independently.
	for _, name := range []string{"GOFLAGS", "GOEXPERIMENT", "GOOS", "GOARCH", "GOAMD64", "GO386", "GOARM", "GOARM64", "GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64", "GOWASM", "CGO_ENABLED", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS", "CC", "CXX", "GODEBUG", "GOFIPS140", "GO_EXTLINK_ENABLED", "GO_LDSO", "GOTOOLCHAIN"} {
		fmt.Fprintf(h, "%s=%q\x00", name, os.Getenv(name))
	}
	var files []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(path, ".go") || d.Name() == "go.mod") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%d:%s:%d:", len(relative), relative, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func generatedArtifactExpectedDigest(root, key string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, key, "record.json"))
	if err != nil {
		return "", err
	}
	var record generatedCacheRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return "", err
	}
	if record.Version != generatedCacheVersion || record.Key != key {
		return "", fmt.Errorf("compiled artifact identity changed before sealing")
	}
	return record.BinarySHA256, nil
}

func readGeneratedArtifact(root, key string) (string, bool) {
	dir := filepath.Join(root, key)
	data, err := os.ReadFile(filepath.Join(dir, "record.json"))
	if err != nil {
		return "", false
	}
	var record generatedCacheRecord
	if json.Unmarshal(data, &record) != nil || record.Version != generatedCacheVersion || record.Key != key {
		return "", false
	}
	binary := filepath.Join(dir, "program.test")
	info, err := os.Lstat(binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", false
	}
	digest, err := generatedFileDigest(binary)
	return binary, err == nil && digest == record.BinarySHA256
}

func publishGeneratedArtifact(root, key, binary string) (string, error) {
	stage, err := os.MkdirTemp(root, ".publish-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	input, err := os.Open(binary)
	if err != nil {
		return "", err
	}
	defer input.Close()
	output, err := os.OpenFile(filepath.Join(stage, "program.test"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0500)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(output, input)
	closeErr := output.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	digest, err := generatedFileDigest(filepath.Join(stage, "program.test"))
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(generatedCacheRecord{generatedCacheVersion, key, digest})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(stage, "record.json"), data, 0600); err != nil {
		return "", err
	}
	destination := filepath.Join(root, key)
	if err := os.Rename(stage, destination); err != nil {
		// A concurrent publisher may have completed the same immutable artifact.
		if ready, ok := readGeneratedArtifact(root, key); ok {
			return ready, nil
		}
		return "", fmt.Errorf("publish generated artifact: %w", err)
	}
	return filepath.Join(destination, "program.test"), nil
}

func generatedCacheRoot() (string, error) {
	root := os.Getenv("PIPELANG_COMPILED_CACHE")
	if root == "" || os.Getenv("GOFLAGS") != "" {
		return "", nil
	}
	if os.Getenv("GOENV") != "off" {
		return "", fmt.Errorf("compiled cache requires GOENV=off so all build settings are explicit")
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("compiled cache path must be absolute")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return "", fmt.Errorf("compiled cache must be a private directory")
	}
	return root, nil
}

func quarantineGeneratedArtifact(root, key string) error {
	entry := filepath.Join(root, key)
	if _, err := os.Lstat(entry); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	quarantine, err := os.MkdirTemp(root, ".invalid-")
	if err != nil {
		return err
	}
	if err := os.Rename(entry, filepath.Join(quarantine, "entry")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

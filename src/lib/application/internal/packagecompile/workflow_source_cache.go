package packagecompile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const workflowSourceBindingMaximumBytes int64 = 4 << 10
const workflowSourceArchiveMaximumBytes int64 = 2 << 30

type workflowSourceBinding struct {
	Schema               int    `json:"schema"`
	SourceRootSHA256     string `json:"source_root_sha256"`
	AuthoredConfigSHA256 string `json:"authored_config_sha256"`
	ArchiveSHA256        string `json:"archive_sha256"`
}

func workflowSourceBindingPath(archive string) string {
	return archive + ".source.json"
}

func workflowSourceRootSHA256(source string) (string, error) {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("canonical workflow source: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("canonical workflow source: %w", err)
	}
	information, err := os.Stat(canonical)
	if err != nil {
		return "", fmt.Errorf("inspect workflow source: %w", err)
	}
	if !information.IsDir() {
		return "", fmt.Errorf("workflow source is not a directory")
	}
	return workflowSourceDigest([]byte(filepath.Clean(canonical))), nil
}

func workflowSourceDigest(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func validWorkflowSourceDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

// openWorkflowSourceCacheFile rejects links and nonregular cache entries. A
// missing or replaced entry is a cache miss; genuine filesystem errors propagate.
func openWorkflowSourceCacheFile(path string) (*os.File, error) {
	information, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !information.Mode().IsRegular() {
		return nil, nil
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(information, opened) {
		_ = file.Close()
		return nil, nil
	}
	return file, nil
}

func workflowSourceBindingMatches(archive, source string, authoredConfig []byte) (bool, error) {
	metadata, err := openWorkflowSourceCacheFile(workflowSourceBindingPath(archive))
	if err != nil {
		return false, fmt.Errorf("read workflow source binding: %w", err)
	}
	if metadata == nil {
		return false, nil
	}
	defer metadata.Close()
	contents, err := io.ReadAll(io.LimitReader(metadata, workflowSourceBindingMaximumBytes+1))
	if err != nil {
		return false, fmt.Errorf("read workflow source binding: %w", err)
	}
	if int64(len(contents)) > workflowSourceBindingMaximumBytes {
		return false, nil
	}
	var binding workflowSourceBinding
	if err := json.Unmarshal(contents, &binding); err != nil {
		return false, nil
	}
	if binding.Schema != 1 || !validWorkflowSourceDigest(binding.SourceRootSHA256) ||
		!validWorkflowSourceDigest(binding.AuthoredConfigSHA256) || !validWorkflowSourceDigest(binding.ArchiveSHA256) {
		return false, nil
	}
	sourceDigest, err := workflowSourceRootSHA256(source)
	if err != nil {
		return false, err
	}
	if binding.SourceRootSHA256 != sourceDigest || binding.AuthoredConfigSHA256 != workflowSourceDigest(authoredConfig) {
		return false, nil
	}
	file, err := openWorkflowSourceCacheFile(archive)
	if err != nil {
		return false, fmt.Errorf("read bound workflow archive: %w", err)
	}
	if file == nil {
		return false, nil
	}
	defer file.Close()
	information, err := file.Stat()
	if err != nil {
		return false, fmt.Errorf("inspect bound workflow archive: %w", err)
	}
	if information.Size() > workflowSourceArchiveMaximumBytes {
		return false, fmt.Errorf("bound workflow archive exceeds the 2 GiB byte limit")
	}
	digest := sha256.New()
	bytesRead, err := io.Copy(digest, io.LimitReader(file, workflowSourceArchiveMaximumBytes+1))
	if err != nil {
		return false, fmt.Errorf("hash bound workflow archive: %w", err)
	}
	if bytesRead > workflowSourceArchiveMaximumBytes {
		return false, fmt.Errorf("bound workflow archive exceeds the 2 GiB byte limit")
	}
	return hex.EncodeToString(digest.Sum(nil)) == binding.ArchiveSHA256, nil
}

// writeWorkflowSourceBinding records only digests beside the local archive. The
// archive writer supplies its computed digest, avoiding another archive read.
func writeWorkflowSourceBinding(archive, source string, authoredConfig []byte, archiveSHA string) error {
	if !validWorkflowSourceDigest(archiveSHA) {
		return fmt.Errorf("workflow archive SHA-256 must contain 64 lowercase hexadecimal characters")
	}
	sourceDigest, err := workflowSourceRootSHA256(source)
	if err != nil {
		return err
	}
	binding := workflowSourceBinding{Schema: 1, SourceRootSHA256: sourceDigest,
		AuthoredConfigSHA256: workflowSourceDigest(authoredConfig), ArchiveSHA256: archiveSHA}
	contents, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("encode workflow source binding: %w", err)
	}
	contents = append(contents, '\n')
	path := workflowSourceBindingPath(archive)
	temporary, err := os.CreateTemp(filepath.Dir(path), ".dockpipe-workflow-source-*")
	if err != nil {
		return fmt.Errorf("create workflow source binding: %w", err)
	}
	defer os.Remove(temporary.Name())
	defer temporary.Close()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("protect workflow source binding: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		return fmt.Errorf("write workflow source binding: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush workflow source binding: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close workflow source binding: %w", err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("publish workflow source binding: %w", err)
	}
	return nil
}

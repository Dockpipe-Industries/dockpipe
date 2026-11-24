package packagecompile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func workflowSourceCacheFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "private-authored-source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "workflow.tar.gz")
	if err := os.WriteFile(archive, []byte("compiled archive A"), 0o644); err != nil {
		t.Fatal(err)
	}
	return archive, source, []byte("name: source-cache-fixture\n")
}

func writeWorkflowSourceCacheFixture(t *testing.T, archive, source string, config []byte) {
	t.Helper()
	contents, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	if err := writeWorkflowSourceBinding(archive, source, config, hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowSourceBindingRequiresSameCanonicalSourceAndConfig(t *testing.T) {
	archive, source, config := workflowSourceCacheFixture(t)
	writeWorkflowSourceCacheFixture(t, archive, source, config)
	matches, err := workflowSourceBindingMatches(archive, source, config)
	if err != nil || !matches {
		t.Fatalf("same source did not retain its cache: matches=%v error=%v", matches, err)
	}
	otherSource := t.TempDir()
	matches, err = workflowSourceBindingMatches(archive, otherSource, config)
	if err != nil || matches {
		t.Fatalf("another source reused the archive: matches=%v error=%v", matches, err)
	}
	matches, err = workflowSourceBindingMatches(archive, source, []byte("name: changed\n"))
	if err != nil || matches {
		t.Fatalf("changed authored config reused the archive: matches=%v error=%v", matches, err)
	}

	t.Run("canonical-alias", func(t *testing.T) {
		alias := filepath.Join(t.TempDir(), "source-alias")
		if err := os.Symlink(source, alias); err != nil {
			t.Skipf("source alias unavailable: %v", err)
		}
		matches, err := workflowSourceBindingMatches(archive, alias, config)
		if err != nil || !matches {
			t.Fatalf("canonical source alias lost its binding: matches=%v error=%v", matches, err)
		}
	})
}

func TestWorkflowSourceBindingRejectsReplacedArchive(t *testing.T) {
	archive, source, config := workflowSourceCacheFixture(t)
	writeWorkflowSourceCacheFixture(t, archive, source, config)
	if err := os.WriteFile(archive, []byte("compiled archive B"), 0o644); err != nil {
		t.Fatal(err)
	}
	matches, err := workflowSourceBindingMatches(archive, source, config)
	if err != nil || matches {
		t.Fatalf("replaced archive retained its old binding: matches=%v error=%v", matches, err)
	}
}

func TestWorkflowSourceBindingMissingOrMalformedMetadataIsCacheMiss(t *testing.T) {
	cases := map[string][]byte{
		"absent":          nil,
		"corrupt":         []byte("not JSON"),
		"unknown-schema":  []byte(`{"schema":2}`),
		"missing-digests": []byte(`{"schema":1}`),
		"oversized":       []byte(strings.Repeat(" ", int(workflowSourceBindingMaximumBytes)+1)),
	}
	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			archive, source, config := workflowSourceCacheFixture(t)
			if contents != nil {
				if err := os.WriteFile(workflowSourceBindingPath(archive), contents, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			matches, err := workflowSourceBindingMatches(archive, source, config)
			if err != nil || matches {
				t.Fatalf("invalid metadata was not a cache miss: matches=%v error=%v", matches, err)
			}
		})
	}
}

func TestWorkflowSourceBindingRejectsLinkedOrNonregularMetadata(t *testing.T) {
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			archive, source, config := workflowSourceCacheFixture(t)
			path := workflowSourceBindingPath(archive)
			if kind == "directory" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				writeWorkflowSourceCacheFixture(t, archive, source, config)
				target := path + ".target"
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("metadata link unavailable: %v", err)
				}
			}
			matches, err := workflowSourceBindingMatches(archive, source, config)
			if err != nil || matches {
				t.Fatalf("nonregular metadata was admitted: matches=%v error=%v", matches, err)
			}
		})
	}
}

func TestWorkflowSourceBindingPublicationIsPrivateAndContainsOnlyDigests(t *testing.T) {
	archive, source, config := workflowSourceCacheFixture(t)
	before, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	writeWorkflowSourceCacheFixture(t, archive, source, config)
	// Replacement exercises the same atomic publication path as a source switch.
	writeWorkflowSourceCacheFixture(t, archive, source, config)
	path := workflowSourceBindingPath(archive)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), source) || strings.Contains(string(contents), filepath.Base(source)) ||
		strings.Contains(string(contents), string(config)) || int64(len(contents)) > workflowSourceBindingMaximumBytes {
		t.Fatalf("private source data escaped into cache metadata: %s", contents)
	}
	var fields map[string]any
	if err := json.Unmarshal(contents, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 4 || fields["schema"] != float64(1) {
		t.Fatalf("unexpected source binding contract: %s", contents)
	}
	information, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && information.Mode().Perm() != 0o600 {
		t.Fatalf("source metadata mode=%o, want 0600", information.Mode().Perm())
	}
	after, err := os.ReadFile(archive)
	if err != nil || string(after) != string(before) {
		t.Fatalf("sidecar publication changed public archive bytes: %q, %v", after, err)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".dockpipe-workflow-source-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("publication left temporary records: %v, %v", leftovers, err)
	}
}

func TestWorkflowSourceBindingFilesystemFailureIsActionable(t *testing.T) {
	// A NUL in the path causes a filesystem error on both POSIX and Windows.
	archive := filepath.Join(t.TempDir(), "invalid\x00archive")
	matches, err := workflowSourceBindingMatches(archive, t.TempDir(), nil)

	var pathError *os.PathError
	if matches || !errors.As(err, &pathError) || pathError.Path != workflowSourceBindingPath(archive) {
		t.Fatalf("filesystem failure became a cache miss: matches=%v error=%v", matches, err)
	}
}

func TestWorkflowSourceBindingBoundsArchiveHash(t *testing.T) {
	archive, source, config := workflowSourceCacheFixture(t)
	writeWorkflowSourceCacheFixture(t, archive, source, config)
	if err := os.Truncate(archive, workflowSourceArchiveMaximumBytes+1); err != nil {
		t.Fatal(err)
	}
	matches, err := workflowSourceBindingMatches(archive, source, config)
	if err == nil || matches || !strings.Contains(err.Error(), "2 GiB") {
		t.Fatalf("oversized archive bypassed its hash bound: matches=%v error=%v", matches, err)
	}
}

package pipelang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Preparation is cache-specific and content-verified in every fresh process.
// A damaged/missing archive reconstructs under the existing child deadline.
var generatedPreparations sync.Map

type generatedPreparationRecord struct {
	Key      string
	Archives map[string]string
}

func prepareGeneratedCache(t *testing.T, command *exec.Cmd, cache string) {
	t.Helper()
	started := time.Now()
	// This key includes exact toolchain bytes and all build-affecting settings,
	// using an empty source directory so it is independent of generated bundles.
	identityDir := t.TempDir()
	key, err := generatedArtifactKey(identityDir)
	if err != nil {
		t.Fatal(err)
	}
	key = "preparation-" + key
	processKey := cache + "/" + key
	if _, ok := generatedPreparations.Load(processKey); ok {
		return
	}
	unlock, err := lockGeneratedArtifact(cache, key)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	path := filepath.Join(cache, key+".json")
	valid := func() bool { return validGeneratedPreparation(path, key, cache) }
	hit := valid()
	if !hit {
		prepare := exec.Command(command.Path, "build", "-p=1", "testing", "encoding/json", "reflect", "strings")
		prepare.Dir, prepare.Env = command.Dir, command.Env
		if output, _, err := measureGeneratedBuild(prepare); err != nil {
			t.Fatalf("prepare native build cache: %v\n%s", err, output)
		}
		listing := exec.Command(command.Path, "list", "-deps", "-export", "-f", "{{.Export}}", "testing", "encoding/json", "reflect", "strings")
		listing.Dir, listing.Env = command.Dir, command.Env
		output, _, err := measureGeneratedBuild(listing)
		if err != nil {
			t.Fatalf("inventory native preparation: %v\n%s", err, output)
		}
		record := generatedPreparationRecord{Key: key, Archives: map[string]string{}}
		for _, archive := range strings.Fields(string(output)) {
			value, err := generatedFileDigest(archive)
			if err != nil {
				t.Fatal(err)
			}
			record.Archives[archive] = value
		}
		if len(record.Archives) == 0 {
			t.Fatal("empty preparation inventory")
		}
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.CreateTemp(cache, ".preparation-")
		if err != nil {
			t.Fatal(err)
		}
		temporary := file.Name()
		if _, err = file.Write(data); err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if err := os.Rename(temporary, path); err != nil {
			t.Fatal(err)
		}
		directory, err := os.Open(cache)
		if err != nil {
			t.Fatal(err)
		}
		err = directory.Sync()
		directory.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !valid() {
			t.Fatal("preparation changed during publication")
		}
	}
	generatedPreparations.Store(processKey, true)
	if os.Getenv("PIPELANG_PERFORMANCE_PROFILE") == "1" {
		t.Logf("generated_preparation cache_hit=%t elapsed_ns=%d key=%s", hit, time.Since(started).Nanoseconds(), key)
	}
}

func validGeneratedPreparation(path, key, cache string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var record generatedPreparationRecord
	if json.Unmarshal(data, &record) != nil || record.Key != key || len(record.Archives) == 0 {
		return false
	}
	for archive, expected := range record.Archives {
		if !strings.HasPrefix(archive, cache+string(os.PathSeparator)) {
			return false
		}
		info, err := os.Lstat(archive)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
		got, err := generatedFileDigest(archive)
		if err != nil || got != expected {
			return false
		}
	}
	return true
}

func TestGeneratedPreparationRejectsCorruptManifest(t *testing.T) {
	root := t.TempDir()
	archive, manifest := filepath.Join(root, "archive"), filepath.Join(root, "manifest")
	if err := os.WriteFile(archive, []byte("verified archive"), 0600); err != nil {
		t.Fatal(err)
	}
	expected, err := generatedFileDigest(archive)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(generatedPreparationRecord{Key: "toolchain-settings", Archives: map[string]string{archive: expected}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, data, 0600); err != nil {
		t.Fatal(err)
	}
	if !validGeneratedPreparation(manifest, "toolchain-settings", root) {
		t.Fatal("valid archive rejected")
	}
	if validGeneratedPreparation(manifest, "changed-settings", root) {
		t.Fatal("changed settings admitted")
	}
	if err := os.WriteFile(archive, []byte("corrupt archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if validGeneratedPreparation(manifest, "toolchain-settings", root) {
		t.Fatal("corrupt archive admitted")
	}
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	if validGeneratedPreparation(manifest, "toolchain-settings", root) {
		t.Fatal("missing archive admitted")
	}
	if err := os.WriteFile(manifest, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if validGeneratedPreparation(manifest, "toolchain-settings", root) {
		t.Fatal("partial publication admitted")
	}
}

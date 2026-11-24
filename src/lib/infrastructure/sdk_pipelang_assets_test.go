package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/sdkassets"
)

func TestPipeLangSDKMaterializesCompleteBundle(t *testing.T) {
	workdir := t.TempDir()
	descriptor, err := SDKHelperPath(workdir, "pipelang")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(descriptor) || filepath.Base(descriptor) != "library.json" {
		t.Fatalf("SDK descriptor = %q", descriptor)
	}
	bundle := filepath.Dir(descriptor)
	err = fs.WalkDir(sdkassets.PipeLang, "pipelang", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		expected, err := sdkassets.PipeLang.ReadFile(path)
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(path, "pipelang/")
		actual, err := os.ReadFile(filepath.Join(bundle, filepath.FromSlash(relative)))
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, expected) {
			t.Errorf("materialized bundle differs at %q", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := PipeLangSDKPath(workdir)
	if err != nil || repeated != descriptor {
		t.Fatalf("repeated descriptor = %q, %v; want %q", repeated, err, descriptor)
	}
}

func TestPipeLangSDKDescriptorPinsCompleteSourceInventory(t *testing.T) {
	content, err := sdkassets.PipeLang.ReadFile("pipelang/library.json")
	if err != nil {
		t.Fatal(err)
	}
	var descriptor struct {
		Protocol   string `json:"protocol"`
		Name       string `json:"name"`
		Version    string `json:"version"`
		SourceRoot string `json:"source_root"`
		Sources    []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"sources"`
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&descriptor); err != nil {
		t.Fatal(err)
	}
	if descriptor.Protocol != "pipelang.source-library.v1" || descriptor.Name != "Dockpipe.SDK" || descriptor.Version == "" || descriptor.SourceRoot != "src" {
		t.Fatalf("unexpected SDK source-library identity: %+v", descriptor)
	}
	sources := make(map[string]bool, len(descriptor.Sources))
	previous := ""
	for _, source := range descriptor.Sources {
		if !fs.ValidPath(source.Path) || !strings.HasSuffix(source.Path, ".pipe") || source.Path <= previous {
			t.Fatalf("invalid or unsorted source path %q after %q", source.Path, previous)
		}
		folded := strings.ToLower(source.Path)
		if sources[folded] {
			t.Fatalf("duplicate source path %q", source.Path)
		}
		sources[folded] = true
		previous = source.Path
		content, err := sdkassets.PipeLang.ReadFile("pipelang/src/" + source.Path)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(content)
		if source.SHA256 != hex.EncodeToString(digest[:]) {
			t.Errorf("source pin differs for %q", source.Path)
		}
	}
	if len(sources) == 0 {
		t.Fatal("SDK descriptor has no source files")
	}
	indexed := 0
	err = fs.WalkDir(sdkassets.PipeLang, "pipelang/src", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".pipe") {
			indexed++
			relative := strings.TrimPrefix(path, "pipelang/src/")
			if !sources[strings.ToLower(relative)] {
				t.Errorf("SDK source %q is missing from the descriptor", relative)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if indexed != len(sources) {
		t.Fatalf("indexed %d source files; descriptor pins %d", indexed, len(sources))
	}
}

func TestPipeLangSDKRejectsModifiedBundle(t *testing.T) {
	for _, relative := range []string{
		"library.json",
		"src/Client.pipe",
		"native/Client_posix.cpp",
		"cmake/DockpipePipeLang.cmake",
		"src/unexpected.pipe",
	} {
		t.Run(relative, func(t *testing.T) {
			workdir := t.TempDir()
			descriptor, err := PipeLangSDKPath(workdir)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(filepath.Dir(descriptor), filepath.FromSlash(relative))
			if err := os.WriteFile(path, []byte("modified"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := PipeLangSDKPath(workdir); err == nil {
				t.Fatal("modified SDK cache accepted")
			}
		})
	}
}

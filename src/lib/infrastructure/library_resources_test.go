package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLibraryResourcesConcurrentPublicationAndCorruption(t *testing.T) {
	root := t.TempDir()
	files := map[string][]byte{"Dockerfile": []byte("FROM scratch\n"), "assets/helper.sh": []byte("echo hi\n")}
	const callers = 8
	var wait sync.WaitGroup
	paths := make(chan string, callers)
	errors := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			path, err := materializeLibraryResources(root, "test", files)
			paths <- path
			errors <- err
		}()
	}
	wait.Wait()
	close(paths)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var published string
	for path := range paths {
		if published != "" && path != published {
			t.Fatal("concurrent callers published different resource roots")
		}
		published = path
	}
	if err := os.WriteFile(filepath.Join(published, "Dockerfile"), []byte("FROM altered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := materializeLibraryResources(root, "test", files); err == nil {
		t.Fatal("corrupted cached resource accepted")
	}
}

func TestLibraryResourcesRejectTraversalAndUnexpectedFiles(t *testing.T) {
	root := t.TempDir()
	if _, err := materializeLibraryResources(root, "test", map[string][]byte{"../escape": []byte("x")}); err == nil {
		t.Fatal("resource traversal accepted")
	}
	files := map[string][]byte{"helper": []byte("owned")}
	directory, err := materializeLibraryResources(root, "test", files)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "unexpected"), []byte("extra"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := materializeLibraryResources(root, "test", files); err == nil {
		t.Fatal("unexpected cached resource accepted")
	}
}

func TestLibraryResourcesRejectUnexpectedDirectoryTrees(t *testing.T) {
	for _, shape := range []string{"empty", "wide", "deep"} {
		t.Run(shape, func(t *testing.T) {
			workdir := t.TempDir()
			files := map[string][]byte{"helper": []byte("owned")}
			directory, err := materializeLibraryResources(workdir, "test", files)
			if err != nil {
				t.Fatal(err)
			}
			unexpected := filepath.Join(directory, "unexpected")
			if err := os.Mkdir(unexpected, 0o700); err != nil {
				t.Fatal(err)
			}
			switch shape {
			case "wide":
				for index := 0; index < 128; index++ {
					if err := os.Mkdir(filepath.Join(unexpected, fmt.Sprint(index)), 0o700); err != nil {
						t.Fatal(err)
					}
				}
			case "deep":
				// Exercise depth without exceeding the host's maximum path length.
				if err := os.MkdirAll(filepath.Join(unexpected, strings.Repeat("n/", 128)), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := materializeLibraryResources(workdir, "test", files); err == nil {
				t.Fatal("unexpected cached directory tree accepted")
			}
		})
	}
}

func TestLibraryResourcesVerifyExactNestedDirectoryShape(t *testing.T) {
	workdir := t.TempDir()
	files := map[string][]byte{
		"Dockerfile":              []byte("FROM scratch\n"),
		"assets/helpers/start.sh": []byte("echo start\n"),
		"assets/helpers/stop.sh":  []byte("echo stop\n"),
		"assets/config.json":      []byte("{}\n"),
	}
	directory, err := materializeLibraryResources(workdir, "test", files)
	if err != nil {
		t.Fatal(err)
	}
	if repeated, err := materializeLibraryResources(workdir, "test", files); err != nil || repeated != directory {
		t.Fatalf("nested resources did not verify: %q, %v", repeated, err)
	}
	if err := os.Mkdir(filepath.Join(directory, "assets", "helpers", "unexpected"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := materializeLibraryResources(workdir, "test", files); err == nil {
		t.Fatal("extra nested directory accepted")
	}
}

func TestLibraryResourcesRejectFileDirectoryConflict(t *testing.T) {
	files := map[string][]byte{"helper": []byte("file"), "helper/child": []byte("child")}
	if _, err := materializeLibraryResources(t.TempDir(), "test", files); err == nil {
		t.Fatal("conflicting file and directory resource accepted")
	}
}

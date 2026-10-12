package pipelang

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dockpipe/tests/containedexec"
)

func TestGeneratedArtifactReuseExecutesCurrentOracle(t *testing.T) {
	if mode := os.Getenv("PIPELANG_ARTIFACT_PROBE"); mode != "" {
		batch := strings.HasPrefix(mode, "batch-")
		mode = strings.TrimPrefix(mode, "batch-")
		source := "package generated\nfunc Value() string { return \"one\" }\n"
		if mode == "source" {
			source = strings.ReplaceAll(source, "one", "two")
		}
		checks := []byte(`package generated
import ("os"; "testing")
func TestCurrentOracle(t *testing.T) {
 data, err := os.ReadFile("oracle.txt")
 if err != nil { t.Fatal(err) }
 if string(data) != Value() { t.Fatalf("current oracle mismatch: %q", data) }
}
`)
		if mode == "compile-only" || mode == "compile-invalid" {
			checks = []byte("package generated\n")
			if mode == "compile-invalid" {
				source = "package generated; func Value() string { return undefinedValue }"
			}
		}
		fixture := []byte("one")
		if mode == "oracle" {
			fixture = []byte("changed")
		}
		if batch {
			checks = bytes.Replace(checks, []byte(`"os"; "testing"`), []byte(`"os"; "testing"; "strings"`), 1)
			checks = bytes.Replace(checks, []byte(`data, err :=`), []byte(`path, err := os.Readlink("/proc/self/exe")
 if err != nil || !strings.Contains(path, "memfd:pipelang-native-bundle") { t.Fatalf("batch did not execute sealed bytes: %s %v", path, err) }
 data, err :=`), 1)
			for i := 0; i < 4; i++ {
				compileAndRunGeneratedGoFilesWithFixtures(t, []byte(source), checks, map[string][]byte{"oracle.txt": fixture})
			}
		} else {
			compileAndRunGeneratedGoFilesWithFixtures(t, []byte(source), checks, map[string][]byte{"oracle.txt": fixture})
		}
		return
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func(mode string, wantFailure, wantHit bool) {
		t.Helper()
		cmd := exec.Command(executable, "-test.run", "^TestGeneratedArtifactReuseExecutesCurrentOracle$", "-test.v", "-test.count=1", "-test.timeout=25s")
		cmd.Env = append(os.Environ(), "PIPELANG_GENERATED_BATCH=1", "PIPELANG_COMPILED_CACHE="+root, "PIPELANG_ARTIFACT_PROBE="+mode, "GOFLAGS=", "GOENV=off")
		output, _, err := containedexec.Measure(cmd)
		if (err != nil) != wantFailure {
			t.Fatalf("probe %s: %v\n%s", mode, err, output)
		}
		token := "cache_hit=false"
		if wantHit {
			token = "cache_hit=true"
		}
		if mode != "compile-invalid" && !bytes.Contains(output, []byte(token)) {
			t.Fatalf("probe %s missing %s:\n%s", mode, token, output)
		}
		failure := "current oracle mismatch"
		if mode == "compile-invalid" {
			failure = "undefined: undefinedValue"
		}
		if wantFailure && !bytes.Contains(output, []byte(failure)) {
			t.Fatalf("wrong failure: %s", output)
		}
	}

	first, err := lockGeneratedArtifact(root, "serialization-probe")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first)
	type lockResult struct {
		release func()
		err     error
	}
	acquired := make(chan lockResult, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		release, err := lockGeneratedArtifact(root, "serialization-probe")
		acquired <- lockResult{release, err}
	}()
	<-started
	select {
	case second := <-acquired:
		if second.release != nil {
			second.release()
		}
		t.Fatalf("second publisher entered before first released: %v", second.err)
	case <-time.After(50 * time.Millisecond):
	}
	first()
	select {
	case second := <-acquired:
		if second.err != nil {
			t.Fatal(second.err)
		}
		second.release()
	case <-time.After(5 * time.Second):
		t.Fatal("publisher lock did not release")
	}
	run("compile-only", false, false)
	run("compile-only", false, true)
	run("compile-invalid", true, false)
	run("initial", false, false)
	run("repeat", false, true)
	run("oracle", true, true)
	run("source", true, false)
	if runtime.GOOS == "linux" {
		run("batch-initial", false, false)
		run("batch-repeat", false, true)
		run("batch-oracle", true, true)
		run("batch-source", true, false)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		binary := filepath.Join(root, entry.Name(), "program.test")
		// Replace the path atomically: /proc sampling may briefly retain an
		// exited executable's inode, making in-place writes return ETXTBSY.
		if err := os.WriteFile(binary+".corrupt", []byte("corrupted"), 0500); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(binary+".corrupt", binary); err != nil {
			t.Fatal(err)
		}
	}
	run("repaired", false, false)
	if runtime.GOOS == "linux" {
		run("batch-repaired", false, false)
	}
}

func TestGeneratedBatchPreservesSpecialHarnesses(t *testing.T) {
	source := []byte("package generated\nfunc Value() int { return 1 }")
	t.Setenv("PIPELANG_GENERATED_BATCH", "1")
	t.Setenv("GOFLAGS", "-cover")
	if queueGeneratedBatch(t, source, []byte("package generated; import \"testing\"; func TestX(t *testing.T) {}"), nil) {
		t.Fatal("explicit Go flags must retain the original harness")
	}

	for _, checks := range []string{
		"package generated; import \"testing\"; func init() {}; func TestX(t *testing.T) {}",
		"package generated; import \"testing\"; func TestMain(m *testing.M) {}; func TestX(t *testing.T) {}",
		"package generated; import \"testing\"; func TestX(t *testing.T) { _ = t.Name() }",
		"package generated; import (\"testing\"; \"reflect\"); func TestX(t *testing.T) { _ = reflect.TypeOf(1).PkgPath() }",
		"package generated_test; import \"testing\"; func TestX(t *testing.T) {}",
		"package generated; import (\"testing\"; \"runtime\"); func TestX(t *testing.T) { _ = runtime.GOOS }",
	} {
		if _, ok := generatedBatchTests(source, []byte(checks)); ok {
			t.Fatalf("special harness accepted: %s", checks)
		}
	}
}

func TestGeneratedArtifactKeysTrackBuildInputs(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "generated.go")
	write := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	key := func() string {
		t.Helper()
		k, err := generatedArtifactKey(dir)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	write(source, "package generated; const Value = 1")
	first := key()
	write(filepath.Join(dir, "oracle.json"), "current oracle")
	if key() != first {
		t.Fatal("runtime fixture unexpectedly changed executable key")
	}
	write(source, "package generated; const Value = 2")
	second := key()
	if second == first {
		t.Fatal("source change reused key")
	}
	write(filepath.Join(dir, "generated_test.go"), "package generated; // changed check")
	third := key()
	if third == second {
		t.Fatal("oracle code change reused key")
	}
	t.Setenv("GOAMD64", "artifact-key-probe")
	if key() == third {
		t.Fatal("build setting change reused key")
	}
}

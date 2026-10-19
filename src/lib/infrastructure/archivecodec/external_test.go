package archivecodec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The copied test executable is a native decoder fixture, never a shell command.
func TestDecoderProcess(t *testing.T) {
	for index, argument := range os.Args {
		if argument != "--" || index+1 >= len(os.Args) {
			continue
		}
		switch os.Args[index+1] {
		case "copy":
			_, err := io.Copy(os.Stdout, os.Stdin)
			if err != nil {
				os.Exit(2)
			}
		case "fail":
			fmt.Fprint(os.Stdout, "partial untrusted output")
			fmt.Fprint(os.Stderr, strings.Repeat("diagnostic", 10000))
			os.Exit(3)
		case "wait":
			if err := os.WriteFile("started", []byte("ready"), 0o600); err != nil {
				os.Exit(4)
			}
			time.Sleep(time.Minute)
		case "environment":
			if os.Getenv("LD_PRELOAD") != "" || os.Getenv("ZSTD_NBTHREADS") != "" || os.Getenv("PRIVATE_TEST_SECRET") != "" {
				os.Exit(5)
			}
			data, err := os.ReadFile("lib/support.txt")
			if err != nil || string(data) != "support" {
				os.Exit(6)
			}
			fmt.Fprint(os.Stdout, "clean")
		default:
			os.Exit(7)
		}
		os.Exit(0)
	}
}

func testProvider(t *testing.T, mode string) provider {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return provider{Executable: executable, SHA256: hex.EncodeToString(sum[:]), Args: []string{"-test.run=^TestDecoderProcess$", "--", mode}}
}

func configure(t *testing.T, selected provider) string {
	t.Helper()
	directory := t.TempDir()
	path := filepath.Join(directory, "decoders.json")
	data, err := json.Marshal(map[string]any{"schema": 1, "providers": map[string]provider{"fixture": selected}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_ARCHIVE_DECODERS", path)
	return path
}

func isolatedDecoderTemps(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	t.Setenv("TMPDIR", directory)
	t.Setenv("TMP", directory)
	t.Setenv("TEMP", directory)
	return directory
}

func requireClean(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("decoder temp cleanup: entries=%v err=%v", entries, err)
	}
}

func TestExternalCompletionAndCleanup(t *testing.T) {
	configure(t, testProvider(t, "copy"))
	directory := isolatedDecoderTemps(t)
	data := []byte("complete decoded tar bytes")
	reader, err := Open(context.Background(), bytes.NewReader(data), "fixture", int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatalf("decode %q: %v", actual, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	requireClean(t, directory)
}

func TestExternalRefusesInvalidProvidersAndOutput(t *testing.T) {
	for _, variant := range []string{"hash", "relative", "overflow", "exit", "traversal", "collision", "symlink"} {
		t.Run(variant, func(t *testing.T) {
			selected := testProvider(t, "copy")
			maximum := int64(100)
			switch variant {
			case "hash":
				selected.SHA256 = strings.Repeat("0", 64)
			case "relative":
				selected.Executable = "decoder"
			case "overflow":
				maximum = 2
			case "exit":
				selected = testProvider(t, "fail")
			case "traversal":
				selected.Files = []providerFile{{Name: "../outside", Path: selected.Executable, SHA256: selected.SHA256}}
			case "collision":
				executableName := "decoder"
				if runtime.GOOS == "windows" {
					executableName += ".exe"
				}
				selected.Files = []providerFile{{Name: executableName, Path: selected.Executable, SHA256: selected.SHA256}}
			case "symlink":
				link := filepath.Join(t.TempDir(), "linked-decoder")
				if err := os.Symlink(selected.Executable, link); err != nil {
					t.Skipf("host cannot create symlinks: %v", err)
				}
				selected.Executable = link
			}
			configure(t, selected)
			directory := isolatedDecoderTemps(t)
			reader, err := Open(context.Background(), strings.NewReader("payload"), "fixture", maximum)
			if reader != nil {
				t.Cleanup(func() {
					if err := reader.Close(); err != nil {
						t.Errorf("close unexpectedly accepted decoder: %v", err)
					}
				})
			}
			if err == nil || reader != nil {
				t.Fatalf("accepted %s: reader=%v err=%v", variant, reader, err)
			}
			if variant == "overflow" && !errors.Is(err, ErrDecodedLimit) {
				t.Fatalf("wrong overflow error: %v", err)
			}
			if variant == "exit" && (!strings.Contains(err.Error(), "stderr truncated") || len(err.Error()) > maxDiagnosticBytes+200) {
				t.Fatalf("unbounded/missing diagnostics: %d", len(err.Error()))
			}
			requireClean(t, directory)
		})
	}
}

func TestExternalMissingProvider(t *testing.T) {
	t.Setenv("DOCKPIPE_ARCHIVE_DECODERS", "")
	if reader, err := Open(context.Background(), strings.NewReader("x"), "fixture", 10); err == nil || reader != nil {
		t.Fatal("accepted unconfigured provider")
	}
	configure(t, testProvider(t, "copy"))
	if reader, err := Open(context.Background(), strings.NewReader("x"), "other", 10); err == nil || reader != nil {
		t.Fatal("accepted unknown provider")
	}
}

func TestExternalPinnedDependenciesAndEnvironment(t *testing.T) {
	selected := testProvider(t, "environment")
	path := filepath.Join(t.TempDir(), "support.txt")
	if err := os.WriteFile(path, []byte("support"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("support"))
	selected.Files = []providerFile{{Path: path, SHA256: hex.EncodeToString(sum[:]), Name: "lib/support.txt"}}
	configure(t, selected)
	directory := isolatedDecoderTemps(t)
	t.Setenv("LD_PRELOAD", "must-not-be-inherited")
	t.Setenv("ZSTD_NBTHREADS", "99")
	t.Setenv("PRIVATE_TEST_SECRET", "must-not-be-inherited")
	reader, err := Open(context.Background(), strings.NewReader(""), "fixture", 5)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(reader)
	if err != nil || string(actual) != "clean" {
		t.Fatalf("environment: %q %v", actual, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	requireClean(t, directory)
}

func TestExternalCancellationKillsStartedProcess(t *testing.T) {
	configure(t, testProvider(t, "wait"))
	directory := isolatedDecoderTemps(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		reader, err := Open(ctx, strings.NewReader(""), "fixture", 10)
		if reader != nil {
			reader.Close()
		}
		finished <- err
	}()
	deadline := time.After(10 * time.Second)
	for {
		matches, _ := filepath.Glob(filepath.Join(directory, "dockpipe-archive-decoder-*", "started"))
		if len(matches) != 0 {
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("decoder stopped before cancellation: %v", err)
		case <-deadline:
			t.Fatal("decoder never started")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("decoder survived cancellation")
	}
	requireClean(t, directory)
}

func TestExternalAggregatePinnedFileLimit(t *testing.T) {
	selected := testProvider(t, "copy")
	path := filepath.Join(t.TempDir(), "large-support")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxProviderBytes); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()
	selected.Files = []providerFile{{Path: path, SHA256: strings.Repeat("0", 64), Name: "support"}}
	configure(t, selected)
	directory := isolatedDecoderTemps(t)
	if reader, err := Open(context.Background(), strings.NewReader(""), "fixture", 1); err == nil || reader != nil || !strings.Contains(err.Error(), "aggregate") {
		t.Fatalf("aggregate limit not enforced: %v", err)
	}
	requireClean(t, directory)
}

func TestExternalRejectsConfigAndStagedNameAmbiguities(t *testing.T) {
	for _, name := range []string{"NUL", "lib/CON.txt", "COM1.dll", "x\\y", "lib/../x", "x.", "/x"} {
		if validStagedName(name) {
			t.Fatalf("unsafe staged name %q", name)
		}
	}
	for _, data := range []string{
		`{"schema":2,"providers":{}}`,
		`{"schema":1,"providers":{"gzip":{}}}`,
		`{"schema":1,"providers":{"gz":{}}}`,
		`{"schema":1,"providers":{}} {}`,
		`{"schema":1,"providers":{},"unexpected":true}`,
		strings.Repeat(" ", int(maxConfigBytes)+1),
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("DOCKPIPE_ARCHIVE_DECODERS", path)
		if reader, err := Open(context.Background(), strings.NewReader(""), "fixture", 1); err == nil || reader != nil {
			t.Fatal("accepted invalid config")
		}
	}
}

func TestExternalReaderFollowsCallerCancellation(t *testing.T) {
	configure(t, testProvider(t, "copy"))
	directory := isolatedDecoderTemps(t)
	ctx, cancel := context.WithCancel(context.Background())
	reader, err := Open(ctx, strings.NewReader("payload"), "fixture", 7)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := reader.Read(make([]byte, 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("completed file reader lost caller cancellation: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	requireClean(t, directory)
}

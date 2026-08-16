package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dorkpipe.orchestrator/statepaths"
)

func TestAppendMetricsUsesDurableCumulativeAuthority(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(stateRoot, "state"))
	t.Setenv("LOCALAPPDATA", filepath.Join(stateRoot, "local-app-data"))
	t.Setenv("HOME", filepath.Join(stateRoot, "home"))
	workdir := t.TempDir()

	if err := appendMetricsJSONL(workdir, "run-1", 0.7, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := appendMetricsJSONL(workdir, "run-2", 0.9, true, true, nil); err != nil {
		t.Fatal(err)
	}
	metricsPath, err := statepaths.MetricsPath(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(filepath.Clean(metricsPath), filepath.Clean(workdir)+string(filepath.Separator)) {
		t.Fatalf("metrics remained inside checkout: %q", metricsPath)
	}
	raw, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"name":"run-1"`) || !strings.Contains(lines[1], `"name":"run-2"`) {
		t.Fatalf("cumulative metrics were replaced or replayed: %q", raw)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(metricsPath)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("metrics mode = %04o, want 0600", info.Mode().Perm())
		}
	}
}

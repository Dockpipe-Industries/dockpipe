package repotools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoLocalDockpipeBinUsesHostExecutableName(t *testing.T) {
	for _, test := range []struct {
		name            string
		operatingSystem string
		files           []string
		want            string
	}{
		{name: "Windows prefers executable", operatingSystem: "windows", files: []string{"dockpipe", "dockpipe.exe"}, want: "dockpipe.exe"},
		{name: "Windows executable only", operatingSystem: "windows", files: []string{"dockpipe.exe"}, want: "dockpipe.exe"},
		{name: "Windows keeps extensionless fallback", operatingSystem: "windows", files: []string{"dockpipe"}, want: "dockpipe"},
		{name: "Unix keeps native executable", operatingSystem: "linux", files: []string{"dockpipe", "dockpipe.exe"}, want: "dockpipe"},
		{name: "Unix ignores Windows executable", operatingSystem: "linux", files: []string{"dockpipe.exe"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "src", "bin")
			if err := os.MkdirAll(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, name := range test.files {
				if err := os.WriteFile(filepath.Join(bin, name), []byte("fixture"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			want := ""
			if test.want != "" {
				want = filepath.Join(bin, test.want)
			}
			if got := resolveRepoLocalDockpipeBin(root, test.operatingSystem); got != want {
				t.Fatalf("repo executable = %q; want %q", got, want)
			}
		})
	}
}

func TestResolveDockpipeBinPreservesExplicitOverride(t *testing.T) {
	t.Setenv("DOCKPIPE_BIN", filepath.Join(t.TempDir(), "selected-dockpipe"))
	got, err := ResolveDockpipeBin(t.TempDir())
	if err != nil || got != os.Getenv("DOCKPIPE_BIN") {
		t.Fatalf("configured executable = %q, %v", got, err)
	}
}

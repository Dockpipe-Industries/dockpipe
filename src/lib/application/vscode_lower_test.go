package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"gopkg.in/yaml.v3"
)

func TestVSCodeWorkflowPlatformMatchesAuthoredContract(t *testing.T) {
	for _, test := range []struct {
		goos string
		want string
	}{
		{goos: "darwin", want: "macos"},
		{goos: "linux", want: "linux"},
		{goos: "windows", want: "windows"},
	} {
		t.Run(test.goos, func(t *testing.T) {
			platform := vscodeWorkflowPlatform(test.goos)
			if platform != test.want {
				t.Fatalf("authored platform = %q, want %q", platform, test.want)
			}
			workflow := &domain.Workflow{
				Name:      "imported-platform",
				Platforms: []string{platform},
				Steps:     []domain.Step{{Kind: "host", Cmd: "echo imported"}},
			}
			if err := domain.ValidateLoadedWorkflow(workflow); err != nil {
				t.Fatalf("imported platform rejected by workflow model: %v", err)
			}
			content, err := yaml.Marshal(workflow)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "config.yml")
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := infrastructure.ValidateResolvedWorkflowYAML(path); err != nil {
				t.Fatalf("imported platform rejected by public YAML schema: %v", err)
			}
		})
	}
}

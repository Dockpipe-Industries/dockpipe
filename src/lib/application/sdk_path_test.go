package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDKPathMaterializesSelectedPortableHelper(t *testing.T) {
	root := t.TempDir()
	for language, filename := range map[string]string{"shell": "dockpipe-sdk.sh", "powershell": "repo-tools.ps1", "python": "repo_tools.py"} {
		t.Run(language, func(t *testing.T) {
			path, err := captureStdout(t, func() error { return cmdSDK([]string{"path", "--language", language, "--workdir", root}) })
			if err != nil || !filepath.IsAbs(path) || filepath.Base(path) != filename {
				t.Fatalf("sdk path = %q, %v", path, err)
			}
			if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
				t.Fatalf("SDK resource is unavailable: %v", err)
			}
			if strings.Contains(path, "src/core") {
				t.Fatal("SDK path depends on retired source tree")
			}
		})
	}
	if err := cmdSDK([]string{"path", "--language", "../unknown", "--workdir", root}); err == nil {
		t.Fatal("unknown resource selection accepted")
	}
	if err := cmdSDK([]string{"path", "--language"}); err == nil {
		t.Fatal("missing language accepted")
	}
}

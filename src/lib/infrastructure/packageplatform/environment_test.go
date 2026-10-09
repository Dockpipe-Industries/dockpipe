package packageplatform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessPlatform(t *testing.T) {
	for _, test := range []struct {
		name, goos, arch, id, metadata, want string
		missing, wantError                   bool
	}{
		{name: "native Linux", goos: "linux", arch: "amd64", missing: true, want: "linux-amd64"},
		{name: "native ignores installed Flatpak", goos: "darwin", arch: "arm64", id: "unused", want: "darwin-arm64"},
		{name: "Flatpak amd64", goos: "linux", arch: "amd64", metadata: "[Application]\nruntime=org.kde.Platform/x86_64/6.10\n", want: "linux-amd64-flatpak-org.kde.Platform-6.10"},
		{name: "Flatpak full ref", goos: "linux", arch: "amd64", metadata: "[Application]\nruntime=runtime/org.kde.Platform/x86_64/6.10\n", want: "linux-amd64-flatpak-org.kde.Platform-6.10"},
		{name: "Flatpak arm64", goos: "linux", arch: "arm64", metadata: "[Application]\nruntime=org.kde.Platform/aarch64/6.10\n", want: "linux-arm64-flatpak-org.kde.Platform-6.10"},
		{name: "missing marker", goos: "linux", arch: "amd64", id: "com.dockpipe.Dockpipe", missing: true, wantError: true},
		{name: "empty marker", goos: "linux", arch: "amd64", wantError: true},
		{name: "wrong section", goos: "linux", arch: "amd64", metadata: "[Instance]\nruntime=org.kde.Platform/x86_64/6.10\n", wantError: true},
		{name: "wrong arch", goos: "linux", arch: "arm64", metadata: "[Application]\nruntime=org.kde.Platform/x86_64/6.10\n", wantError: true},
		{name: "unsafe runtime", goos: "linux", arch: "amd64", metadata: "[Application]\nruntime=../x86_64/6.10\n", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "flatpak-info")
			if !test.missing {
				if err := os.WriteFile(path, []byte(test.metadata), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := processPlatform(test.goos, test.arch, test.id, path)
			if (err != nil) != test.wantError || got != test.want {
				t.Fatalf("platform = %q, %v; want %q, error=%v", got, err, test.want, test.wantError)
			}
		})
	}
}

package processenvironment

import (
	"os"
	"testing"
)

func TestContainmentBoundaries(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		goos   string
		env    string
		marker string
		err    error
		want   bool
	}{
		{name: "native Linux", goos: "linux"},
		{name: "Flatpak environment", goos: "linux", env: "FLATPAK_ID", want: true},
		{name: "Snap environment", goos: "linux", env: "SNAP", want: true},
		{name: "systemd container environment", goos: "linux", env: "container", want: true},
		{name: "Flatpak marker", goos: "linux", marker: "/.flatpak-info", want: true},
		{name: "Docker marker", goos: "linux", marker: "/.dockerenv", want: true},
		{name: "Podman marker", goos: "linux", marker: "/run/.containerenv", want: true},
		{name: "systemd container marker", goos: "linux", marker: "/run/systemd/container", want: true},
		{name: "unreadable marker", goos: "linux", marker: "/.flatpak-info", err: os.ErrPermission, want: true},
		{name: "native Darwin", goos: "darwin", env: "container"},
		{name: "native Windows", goos: "windows", marker: "/.dockerenv"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			getenv := func(name string) string {
				if name == scenario.env {
					return "present"
				}
				return ""
			}
			stat := func(path string) (os.FileInfo, error) {
				if path == scenario.marker {
					return nil, scenario.err
				}
				return nil, os.ErrNotExist
			}
			if got := isContained(scenario.goos, getenv, stat); got != scenario.want {
				t.Fatalf("contained = %v, want %v", got, scenario.want)
			}
		})
	}
}

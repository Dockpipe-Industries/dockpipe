//go:build linux

package sqliteevidence

import (
	"strings"
	"testing"
)

func TestLinuxDistributionFromOSReleaseAcceptsReviewedIdentities(t *testing.T) {
	for _, test := range []struct {
		name       string
		osRelease  string
		prettyName string
	}{
		{
			name: "Ubuntu 24.04 guest",
			osRelease: `NAME="Ubuntu"
VERSION="24.04.4 LTS (Noble Numbat)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 24.04.4 LTS"
VERSION_ID="24.04"
`,
			prettyName: "Ubuntu 24.04.4 LTS",
		},
		{
			name: "Pop!_OS host",
			osRelease: `NAME="Pop!_OS"
VERSION="22.04 LTS"
ID=pop
ID_LIKE="ubuntu debian"
PRETTY_NAME="Pop!_OS 22.04 LTS"
VERSION_ID="22.04"
`,
			prettyName: "Pop!_OS 22.04 LTS",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			prettyName, err := linuxDistributionFromOSRelease([]byte(test.osRelease))
			if err != nil {
				t.Fatalf("reviewed distribution rejected: %v", err)
			}
			if prettyName != test.prettyName {
				t.Fatalf("pretty name = %q, want %q", prettyName, test.prettyName)
			}
		})
	}
}

func TestLinuxDistributionFromOSReleaseRejectsIncompleteOrUnsupportedIdentities(t *testing.T) {
	for _, test := range []struct {
		name      string
		osRelease string
	}{
		{name: "missing ID", osRelease: "VERSION_ID=24.04\nPRETTY_NAME=Ubuntu 24.04.4 LTS\n"},
		{name: "missing VERSION_ID", osRelease: "ID=ubuntu\nPRETTY_NAME=Ubuntu 24.04.4 LTS\n"},
		{name: "missing PRETTY_NAME", osRelease: "ID=ubuntu\nVERSION_ID=24.04\n"},
		{name: "unsupported distribution", osRelease: "ID=debian\nVERSION_ID=12\nPRETTY_NAME=Debian GNU/Linux 12\n"},
		{name: "unsupported Ubuntu release", osRelease: "ID=ubuntu\nVERSION_ID=22.04\nPRETTY_NAME=Ubuntu 22.04 LTS\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := linuxDistributionFromOSRelease([]byte(test.osRelease))
			if err == nil || !strings.Contains(err.Error(), "unsupported or incomplete distribution identity") {
				t.Fatalf("error = %v, want unsupported or incomplete distribution identity", err)
			}
		})
	}
}

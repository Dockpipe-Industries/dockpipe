//go:build linux

package osauth

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
)

func TestSecurityDirectoryIgnoresProjectAndEnvironmentOverrides(t *testing.T) {
	before, beforeError := securityDirectory()
	if beforeError != nil {
		// Container runtimes may use a host UID absent from /etc/passwd. A
		// usable OS account must succeed; an unavailable account must stay
		// unavailable even when a project supplies its own HOME or state root.
		account, lookupError := user.LookupId(strconv.Itoa(os.Getuid()))
		if lookupError == nil && filepath.IsAbs(account.HomeDir) && account.HomeDir != "/" {
			t.Fatal(beforeError)
		}
	}
	for _, name := range []string{"HOME", "XDG_STATE_HOME", "XDG_DATA_HOME", "DOCKPIPE_GLOBAL_ROOT", "DOCKPIPE_REMOTE_STATE"} {
		t.Setenv(name, "/tmp/untrusted-project-security-root")
	}
	after, afterError := securityDirectory()
	if beforeError != nil {
		if before != "" || after != "" || afterError == nil || afterError.Error() != beforeError.Error() {
			t.Fatalf("environment overrides bypassed unavailable OS account: %q %v", after, afterError)
		}
		return
	}
	if afterError != nil || after != before || !filepath.IsAbs(after) {
		t.Fatalf("security owner followed project environment: %q %v", after, afterError)
	}
}

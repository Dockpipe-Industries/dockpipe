//go:build linux

package osauth

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// Account database lookup deliberately ignores HOME, XDG_*, and Dockpipe root
// overrides. The caller still owns private directory creation and validation.
func securityDirectory() (string, error) {
	account, err := user.LookupId(strconv.Itoa(os.Getuid()))
	if err != nil || !filepath.IsAbs(account.HomeDir) || account.HomeDir == "/" {
		return "", errors.New("remote security requires an absolute home directory from the OS account database")
	}
	return filepath.Join(account.HomeDir, ".local", "state", "dockpipe", "security"), nil
}

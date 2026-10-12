//go:build !windows

package cloudflare

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func validateCredentialAccess(path string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return errors.New("credential for Cloudflare is not owned by the current user")
	}
	// Cloudflared writes tunnel credentials read-only. We only read them, so
	// preserve that protection instead of requiring mutable Dockpipe-state permissions.
	if info.Mode() != 0o400 && info.Mode() != 0o600 {
		return fmt.Errorf("credential for Cloudflare %q must have owner-only read permissions (0400 or 0600), got %04o", path, info.Mode())
	}
	return nil
}

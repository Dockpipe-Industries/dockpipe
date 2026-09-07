//go:build linux

package pipelang

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// A bundle can serve many fresh children without rehashing its full contents for
// each child. Hash the exact snapshot, then let the kernel prohibit all changes.
// Execution uses the inherited sealed descriptor, never the mutable cache path.
func sealGeneratedExecutable(path, expected string) (*os.File, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return nil, fmt.Errorf("bundle executable exceeds bounds")
	}
	fd, err := unix.MemfdCreate("pipelang-native-bundle", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, err
	}
	sealed := os.NewFile(uintptr(fd), "pipelang-native-bundle")
	ok := false
	defer func() {
		if !ok {
			sealed.Close()
		}
	}()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(sealed, hash), io.LimitReader(input, (64<<20)+1))
	if err != nil {
		return nil, err
	}
	if n != info.Size() || n > 64<<20 || hex.EncodeToString(hash.Sum(nil)) != expected {
		return nil, fmt.Errorf("bundle executable changed before sealing")
	}
	if err := sealed.Chmod(0500); err != nil {
		return nil, err
	}
	const required = unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_SEAL
	if _, err := unix.FcntlInt(sealed.Fd(), unix.F_ADD_SEALS, required); err != nil {
		return nil, err
	}
	flags, err := unix.FcntlInt(sealed.Fd(), unix.F_GET_SEALS, 0)
	if err != nil {
		return nil, err
	}
	if flags&required != required {
		return nil, fmt.Errorf("bundle executable seals unavailable")
	}
	if _, err := sealed.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	ok = true
	return sealed, nil
}

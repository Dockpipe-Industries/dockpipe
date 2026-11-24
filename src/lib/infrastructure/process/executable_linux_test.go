package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestResolveExecutableUsesMappedProgramForMuslLoader(t *testing.T) {
	want, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	got, err := resolveExecutable("/lib/ld-musl-x86_64.so.1")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("mapped executable = %q, want %q", got, want)
	}
}

func TestMappedExecutableVerifiesFileIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "program with spaces")
	if err := os.WriteFile(path, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	var info unix.Stat_t
	if err := unix.Stat(path, &info); err != nil {
		t.Fatal(err)
	}

	line := func(inode uint64) *strings.Reader {
		return strings.NewReader(fmt.Sprintf("1000-2000 r-xp 00000000 %x:%x %d   %s\n",
			unix.Major(uint64(info.Dev)), unix.Minor(uint64(info.Dev)), inode, path))
	}
	got, err := mappedExecutable(line(info.Ino), 0x1000)
	if err != nil || got != path {
		t.Fatalf("mapped executable = %q, %v; want %q", got, err, path)
	}
	if got, err := mappedExecutable(line(info.Ino+1), 0x1000); err == nil {
		t.Fatalf("accepted a replaced executable path %q", got)
	}
}

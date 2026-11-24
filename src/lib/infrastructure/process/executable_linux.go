package process

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

var executableMapping = regexp.MustCompile(`^([0-9a-f]+)-([0-9a-f]+) [r-][w-]x[ps] [0-9a-f]+ ([0-9a-f]+):([0-9a-f]+) ([0-9]+) +(/.*)$`)

func resolveExecutable(executable string) (string, error) {
	name := filepath.Base(executable)
	if !strings.HasPrefix(name, "ld-musl-") || !strings.HasSuffix(name, ".so.1") {
		return executable, nil
	}

	// Alpine's glibc compatibility loader execs musl. The kernel's exe link
	// then identifies musl, while the Go code is mapped from the actual binary.
	// Resolve that mapping without consulting the mutable invocation argv or PATH.
	maps, err := os.Open("/proc/self/maps")
	if err != nil {
		return "", fmt.Errorf("read running executable mappings: %w", err)
	}
	defer maps.Close()

	var callers [1]uintptr
	if runtime.Callers(0, callers[:]) != 1 {
		return "", errors.New("cannot locate running executable code")
	}

	return mappedExecutable(maps, uint64(callers[0]-1))
}

func mappedExecutable(maps io.Reader, address uint64) (string, error) {
	scanner := bufio.NewScanner(maps)
	for scanner.Scan() {
		mapping := executableMapping.FindStringSubmatch(scanner.Text())
		if mapping == nil {
			continue
		}

		start, startErr := strconv.ParseUint(mapping[1], 16, 64)
		end, endErr := strconv.ParseUint(mapping[2], 16, 64)
		if startErr != nil || endErr != nil || address < start || address >= end {
			continue
		}

		major, majorErr := strconv.ParseUint(mapping[3], 16, 32)
		minor, minorErr := strconv.ParseUint(mapping[4], 16, 32)
		inode, inodeErr := strconv.ParseUint(mapping[5], 10, 64)
		if majorErr != nil || minorErr != nil || inodeErr != nil || inode == 0 {
			return "", errors.New("invalid running executable mapping identity")
		}

		path := mapping[6]
		var info unix.Stat_t
		if err := unix.Stat(path, &info); err != nil {
			return "", fmt.Errorf("stat mapped executable: %w", err)
		}
		if info.Mode&unix.S_IFMT != unix.S_IFREG || info.Mode&0o111 == 0 ||
			info.Ino != inode || unix.Major(uint64(info.Dev)) != uint32(major) ||
			unix.Minor(uint64(info.Dev)) != uint32(minor) {
			return "", errors.New("mapped executable file no longer matches the running image")
		}

		return path, nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan running executable mappings: %w", err)
	}

	return "", errors.New("running executable does not have a file-backed code mapping")
}

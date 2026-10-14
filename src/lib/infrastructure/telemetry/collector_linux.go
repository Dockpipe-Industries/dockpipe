//go:build linux

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// NativeCollector reads process metadata only. It never reads cmdline or environ.
type NativeCollector struct{ Root string }

func (collector NativeCollector) root() string {
	if collector.Root != "" {
		return collector.Root
	}
	return "/proc"
}

func readSmall(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	if len(data) > 8192 {
		return nil, errors.New("process metadata exceeds 8 KiB")
	}
	return data, err
}

func (collector NativeCollector) Inspect(ctx context.Context, pid int) (Process, error) {
	if err := ctx.Err(); err != nil {
		return Process{}, err
	}
	if pid <= 0 {
		return Process{}, errors.New("invalid process ID")
	}
	data, err := readSmall(filepath.Join(collector.root(), strconv.Itoa(pid), "stat"))
	if err != nil {
		return Process{}, err
	}
	process, err := parseStat(string(data), uint64(os.Getpagesize()))
	if err != nil {
		return Process{}, err
	}
	if process.PID != pid {
		return Process{}, errors.New("process identity mismatch")
	}
	boot, err := readSmall(filepath.Join(collector.root(), "sys/kernel/random/boot_id"))
	if err != nil {
		return Process{}, err
	}
	process.StartToken = strings.TrimSpace(string(boot)) + ":" + process.StartToken
	process.Machine = "local"
	return process, nil
}

func parseStat(data string, pageSize uint64) (Process, error) {
	opening, closing := strings.IndexByte(data, '('), strings.LastIndexByte(data, ')')
	if opening < 1 || closing <= opening {
		return Process{}, errors.New("invalid process stat")
	}
	fields := strings.Fields(data[closing+1:])
	if len(fields) < 22 {
		return Process{}, errors.New("incomplete process stat")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(data[:opening]))
	if err != nil {
		return Process{}, err
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return Process{}, err
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return Process{}, err
	}
	resident, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil || resident < 0 || uint64(resident) > ^uint64(0)/pageSize {
		return Process{}, errors.New("invalid resident memory")
	}
	memory := uint64(resident) * pageSize
	// Linux /proc stat uses USER_HZ (100 ticks/second), independent of CONFIG_HZ.
	user, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return Process{}, err
	}
	system, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return Process{}, err
	}
	seconds := float64(user)/100 + float64(system)/100
	return Process{ProcessIdentity: ProcessIdentity{PID: pid, StartToken: strconv.FormatUint(start, 10)},
		ParentPID: parent, Name: data[opening+1 : closing], State: fields[0],
		MemoryBytes: &memory, CPUSeconds: &seconds}, nil
}

func (collector NativeCollector) Discover(ctx context.Context) ([]Process, bool, error) {
	directory, err := os.Open(collector.root())
	if err != nil {
		return nil, false, err
	}
	defer directory.Close()
	processes := make([]Process, 0)
	denied := false
	inspected := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		entries, readErr := directory.ReadDir(128)
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil || !entry.IsDir() {
				continue
			}
			inspected++
			if inspected > MaxProcesses*2 || len(processes) == MaxProcesses {
				return processes, true, nil
			}
			process, err := collector.Inspect(ctx, pid)
			if err == nil {
				processes = append(processes, process)
			} else if os.IsPermission(err) {
				denied = true
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return processes, false, readErr
		}
	}
	if denied {
		return processes, false, fmt.Errorf("some process metadata is inaccessible")
	}
	return processes, false, nil
}

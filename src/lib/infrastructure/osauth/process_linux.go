//go:build linux && (amd64 || arm64)

package osauth

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type processIdentity struct {
	pid       int
	uid       int
	startTime uint64
}

func readProcessIdentity(pid int) (processIdentity, error) {
	if pid <= 0 {
		return processIdentity{}, errors.New("invalid local authentication process")
	}
	root := "/proc/" + strconv.Itoa(pid)
	status, err := boundedFile(root+"/status", 16<<10)
	if err != nil {
		return processIdentity{}, errors.New("local authentication process is unavailable")
	}
	uid := os.Getuid()
	validUID := false
	for _, line := range strings.Split(string(status), "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		values := strings.Fields(strings.TrimPrefix(line, "Uid:"))
		if len(values) != 4 {
			return processIdentity{}, errors.New("invalid local authentication process ownership")
		}
		for _, value := range values {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed != uid {
				return processIdentity{}, errors.New("authentication process must belong to the current OS account")
			}
		}
		validUID = true
	}
	if !validUID {
		return processIdentity{}, errors.New("local authentication process ownership is unavailable")
	}
	stat, err := boundedFile(root+"/stat", 16<<10)
	if err != nil {
		return processIdentity{}, errors.New("local authentication process is unavailable")
	}
	startTime, err := parseProcessStartTime(stat)
	if err != nil {
		return processIdentity{}, err
	}
	return processIdentity{pid: pid, uid: uid, startTime: startTime}, nil
}

func parseProcessStartTime(stat []byte) (uint64, error) {
	// The command name can contain spaces and parentheses. Fields after its final
	// ')' begin at field 3; starttime is field 22 in proc_pid_stat(5).
	end := bytes.LastIndexByte(stat, ')')
	if end < 0 {
		return 0, errors.New("invalid local process identity")
	}
	fields := strings.Fields(string(stat[end+1:]))
	if len(fields) < 20 {
		return 0, errors.New("incomplete local process identity")
	}
	value, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid local process start time")
	}
	return value, nil
}

func (identity processIdentity) subject() string {
	return fmt.Sprintf("%d,%d,%d", identity.pid, identity.startTime, identity.uid)
}

func boundedFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return nil, errors.New("OS security response exceeds its bound")
	}
	return data, nil
}

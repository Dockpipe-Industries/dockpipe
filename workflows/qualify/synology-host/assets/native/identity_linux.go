//go:build linux

package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

const schema = "dockpipe.synology_host_qualification.v1"

// Linux permits 65536 supplementary groups. The response contract retains all of
// them; silently truncating groups would misrepresent actual host permissions.
const maximumGroups = 65536
const maximumStatusBytes = 2 << 20

type identity struct {
	UID          int               `json:"uid"`
	EUID         int               `json:"euid"`
	GID          int               `json:"gid"`
	EGID         int               `json:"egid"`
	Groups       []int             `json:"groups"`
	Capabilities map[string]string `json:"capabilities"`
	NoNewPrivs   *bool             `json:"no_new_privs"`
}

type status struct {
	Schema   string   `json:"schema"`
	Hostname string   `json:"hostname"`
	Identity identity `json:"identity"`
}

func currentStatus() (status, error) {
	hostname, err := os.Hostname()
	if err != nil || !validHostname(hostname) {
		return status{}, errors.New("cannot obtain valid host identity")
	}
	groups, err := os.Getgroups()
	if err != nil || len(groups) > maximumGroups {
		return status{}, errors.New("cannot obtain complete process groups")
	}
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return status{}, errors.New("cannot read process capability metadata")
	}
	defer file.Close()
	capabilities, err := parseCapabilities(io.LimitReader(file, maximumStatusBytes+1))
	if err != nil {
		return status{}, err
	}
	noNewPrivs, err := currentNoNewPrivileges()
	if err != nil {
		return status{}, err
	}
	return status{
		Schema: schema, Hostname: hostname,
		Identity: identity{
			UID: os.Getuid(), EUID: os.Geteuid(), GID: os.Getgid(), EGID: os.Getegid(),
			Groups: groups, Capabilities: capabilities, NoNewPrivs: noNewPrivs,
		},
	}, nil
}

func validHostname(hostname string) bool {
	// Bound the diagnostic string to a DNS name's maximum textual length.
	if hostname == "" || len(hostname) > 255 || !utf8.ValidString(hostname) {
		return false
	}
	for _, character := range hostname {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

// PR_GET_NO_NEW_PRIVS has been available since Linux 3.5, before the corresponding
// /proc status field appeared in 4.10. This getter never changes privileges.
func currentNoNewPrivileges() (*bool, error) {
	const getNoNewPrivileges = 39
	value, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, getNoNewPrivileges, 0, 0, 0, 0, 0)
	return noNewPrivilegesResult(value, errno)
}

func noNewPrivilegesResult(value uintptr, errno syscall.Errno) (*bool, error) {
	if errno == syscall.EINVAL || errno == syscall.ENOSYS {
		// JSON null states that this kernel cannot report the attribute.
		return nil, nil
	}
	if errno != 0 || value > 1 {
		return nil, errors.New("cannot obtain process no-new-privileges metadata")
	}
	enabled := value == 1
	return &enabled, nil
}

func parseCapabilities(reader io.Reader) (map[string]string, error) {
	capabilities := make(map[string]string)
	foundNoNewPrivs := false
	scanner := bufio.NewScanner(reader)
	// A maximum-size Linux Groups line fits this bound.
	scanner.Buffer(make([]byte, 4096), maximumStatusBytes)
	bytesRead := 0
	for scanner.Scan() {
		bytesRead += len(scanner.Bytes()) + 1
		if bytesRead > maximumStatusBytes {
			return nil, errors.New("process status exceeds qualification budget")
		}
		key, value, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb":
			if _, duplicate := capabilities[key]; duplicate || len(value) != 16 {
				return nil, errors.New("invalid capability metadata")
			}
			if _, err := strconv.ParseUint(value, 16, 64); err != nil {
				return nil, errors.New("invalid capability metadata")
			}
			capabilities[key] = value
		case "NoNewPrivs":
			if foundNoNewPrivs || (value != "0" && value != "1") {
				return nil, errors.New("invalid privilege metadata")
			}
			foundNoNewPrivs = true
		}
	}
	if scanner.Err() != nil {
		return nil, errors.New("cannot read complete process capability metadata")
	}
	for _, required := range []string{"CapInh", "CapPrm", "CapEff", "CapBnd"} {
		if _, found := capabilities[required]; !found {
			return nil, errors.New("incomplete process capability metadata")
		}
	}
	return capabilities, nil
}

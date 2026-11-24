//go:build linux

package hostidentity

import (
	"bufio"
	"errors"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// Linux permits 65536 supplementary groups. The response contract retains all of
// them; silently truncating groups would misrepresent actual host permissions.
const MaximumGroups = 65536
const MaximumStatusBytes = 2 << 20

type Identity struct {
	UID          int               `json:"uid"`
	EUID         int               `json:"euid"`
	SavedUID     int               `json:"saved_uid"`
	FSUID        int               `json:"fs_uid"`
	GID          int               `json:"gid"`
	EGID         int               `json:"egid"`
	SavedGID     int               `json:"saved_gid"`
	FSGID        int               `json:"fs_gid"`
	Groups       []int             `json:"groups"`
	Capabilities map[string]string `json:"capabilities"`
	NoNewPrivs   *bool             `json:"no_new_privs"`
}

type Status struct {
	Schema   string   `json:"schema"`
	Hostname string   `json:"hostname"`
	Identity Identity `json:"identity"`
}

func Read(schema string) (Status, error) {
	hostname, err := os.Hostname()
	if err != nil || !ValidHostname(hostname) {
		return Status{}, errors.New("cannot obtain valid host identity")
	}
	groups, err := os.Getgroups()
	if err != nil || len(groups) > MaximumGroups {
		return Status{}, errors.New("cannot obtain complete process groups")
	}
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return Status{}, errors.New("cannot read process capability metadata")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaximumStatusBytes+1))
	if err != nil || len(data) > MaximumStatusBytes {
		return Status{}, errors.New("cannot read bounded process status")
	}
	capabilities, err := ParseCapabilities(strings.NewReader(string(data)))
	if err != nil {
		return Status{}, err
	}
	userIDs, err := statusIDs(string(data), "Uid")
	if err != nil {
		return Status{}, err
	}
	groupIDs, err := statusIDs(string(data), "Gid")
	if err != nil {
		return Status{}, err
	}
	noNewPrivs, err := CurrentNoNewPrivileges()
	if err != nil {
		return Status{}, err
	}
	return Status{
		Schema: schema, Hostname: hostname,
		Identity: Identity{
			UID: userIDs[0], EUID: userIDs[1], SavedUID: userIDs[2], FSUID: userIDs[3],
			GID: groupIDs[0], EGID: groupIDs[1], SavedGID: groupIDs[2], FSGID: groupIDs[3],
			Groups: groups, Capabilities: capabilities, NoNewPrivs: noNewPrivs,
		},
	}, nil
}

func statusIDs(data, name string) ([4]int, error) {
	var result [4]int
	for _, line := range strings.Split(data, "\n") {
		key, value, _ := strings.Cut(line, ":")
		if key != name {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) != len(result) {
			break
		}
		for index, field := range fields {
			identifier, err := strconv.ParseUint(field, 10, 32)
			if err != nil || identifier > uint64(math.MaxInt) {
				return result, errors.New("invalid process credential metadata")
			}
			result[index] = int(identifier)
		}
		return result, nil
	}
	return result, errors.New("missing process credential metadata")
}

func ValidHostname(hostname string) bool {
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
func CurrentNoNewPrivileges() (*bool, error) {
	const getNoNewPrivileges = 39
	value, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, getNoNewPrivileges, 0, 0, 0, 0, 0)
	return NoNewPrivilegesResult(value, errno)
}

func NoNewPrivilegesResult(value uintptr, errno syscall.Errno) (*bool, error) {
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

func ParseCapabilities(reader io.Reader) (map[string]string, error) {
	capabilities := make(map[string]string)
	foundNoNewPrivs := false
	scanner := bufio.NewScanner(reader)
	// A maximum-size Linux Groups line fits this bound.
	scanner.Buffer(make([]byte, 4096), MaximumStatusBytes)
	bytesRead := 0
	for scanner.Scan() {
		bytesRead += len(scanner.Bytes()) + 1
		if bytesRead > MaximumStatusBytes {
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

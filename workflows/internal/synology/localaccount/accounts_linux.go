//go:build linux

// Package localaccount resolves eligible local DSM administrators.
package localaccount

import (
	"errors"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

const MaximumAccountBytes = 2 << 20

type Account struct {
	Username string
	UID      uint32
	GID      uint32
	Groups   []uint32
}

func Resolve(username string) (Account, error) {
	passwd, err := os.Open("/etc/passwd")
	if err != nil {
		return Account{}, errors.New("local account database unavailable")
	}
	defer passwd.Close()
	groups, err := os.Open("/etc/group")
	if err != nil {
		return Account{}, errors.New("local group database unavailable")
	}
	defer groups.Close()
	identifier := os.Getuid()
	if identifier < 0 || uint64(identifier) > math.MaxUint32 {
		return Account{}, errors.New("package account identifier is out of range")
	}
	return ResolveFiles(username, passwd, groups, uint32(identifier))
}

func accountDatabase(reader io.Reader) ([]string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaximumAccountBytes+1))
	if err != nil || len(data) > MaximumAccountBytes {
		return nil, errors.New("local account database exceeds qualification budget")
	}
	return strings.Split(string(data), "\n"), nil
}

func ResolveFiles(username string, passwd, groupFile io.Reader, packageUID uint32) (Account, error) {
	if username == "root" || username == "nobody" || strings.HasPrefix(username, "sc-") || strings.HasPrefix(username, "dockpipe-") {
		return Account{}, errors.New("system and package accounts are ineligible")
	}
	lines, err := accountDatabase(passwd)
	if err != nil {
		return Account{}, err
	}
	var account Account
	found := false
	owners := make(map[uint32]string)
	aliased := make(map[uint32]bool)
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 7 {
			continue
		}
		uid, uidError := strconv.ParseUint(fields[2], 10, 32)
		gid, gidError := strconv.ParseUint(fields[3], 10, 32)
		if uidError != nil || gidError != nil {
			return Account{}, errors.New("invalid local account identifiers")
		}
		if previous, duplicate := owners[uint32(uid)]; duplicate && previous != fields[0] {
			aliased[uint32(uid)] = true
		}
		owners[uint32(uid)] = fields[0]
		if fields[0] != username {
			continue
		}
		if found || uid < 1024 || uid == 65534 || uid == 4294967294 || uint32(uid) == packageUID || gid == 0 {
			return Account{}, errors.New("local account is ineligible")
		}
		switch fields[6] {
		case "/bin/sh", "/bin/bash", "/bin/ash", "/usr/bin/bash":
		default:
			return Account{}, errors.New("non-login and unsupported local accounts are ineligible")
		}
		account = Account{Username: username, UID: uint32(uid), GID: uint32(gid)}
		found = true
	}
	if !found {
		return Account{}, errors.New("only an unambiguous local DSM account is supported")
	}
	// DSM legitimately aliases unrelated system accounts (for example ftp and
	// anonymous). Only ambiguity in the selected operator's UID is relevant.
	if aliased[account.UID] {
		return Account{}, errors.New("aliased local account identifiers are unsupported")
	}
	groupLines, err := accountDatabase(groupFile)
	if err != nil {
		return Account{}, err
	}
	identifiers := map[uint32]bool{account.GID: true}
	administrator := false
	for _, line := range groupLines {
		fields := strings.Split(line, ":")
		if len(fields) != 4 {
			continue
		}
		identifier, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			return Account{}, errors.New("invalid local group identifiers")
		}
		for _, member := range strings.Split(fields[3], ",") {
			if member == username {
				if identifier == 0 {
					return Account{}, errors.New("root group membership is ineligible")
				}
				identifiers[uint32(identifier)] = true
				if fields[0] == "administrators" {
					administrator = true
				}
			}
		}
	}
	if !administrator {
		return Account{}, errors.New("qualification requires explicit local administrators membership")
	}
	if len(identifiers) > 65536 {
		return Account{}, errors.New("local groups exceed the Linux qualification contract")
	}
	for identifier := range identifiers {
		account.Groups = append(account.Groups, identifier)
	}
	sort.Slice(account.Groups, func(left, right int) bool { return account.Groups[left] < account.Groups[right] })
	return account, nil
}

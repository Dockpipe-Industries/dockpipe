//go:build linux

package main

import (
	"dockpipe/workflows/internal/synology/localaccount"
	"io"
)

const maximumAccountBytes = localaccount.MaximumAccountBytes

type localAccount = localaccount.Account

func resolveAccount(username string) (localAccount, error) {
	return localaccount.Resolve(username)
}

func resolveLocalAccount(username string, passwd, groupFile io.Reader, packageUID uint32) (localAccount, error) {
	return localaccount.ResolveFiles(username, passwd, groupFile, packageUID)
}

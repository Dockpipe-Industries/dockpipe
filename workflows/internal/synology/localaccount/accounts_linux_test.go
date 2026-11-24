//go:build linux

package localaccount

import (
	"strings"
	"testing"
)

func TestUnrelatedDSMSystemAliasesDoNotRejectLocalAdministrator(t *testing.T) {
	// DSM can assign anonymous and ftp the same system UID. The selected
	// operator remains unambiguous, with normal users/administrators groups.
	passwd := "root:x:0:0:root:/root:/bin/sh\n" +
		"anonymous:x:21:21:anonymous:/:/sbin/nologin\n" +
		"ftp:x:21:21:ftp:/:/sbin/nologin\n" +
		"operator:x:1026:100:operator:/var/services/homes/operator:/bin/sh\n"
	groups := "users:x:100:operator\nadministrators:x:101:admin,operator\ndocker:x:65536:operator\n"
	account, err := ResolveFiles("operator", strings.NewReader(passwd), strings.NewReader(groups), 268604)
	if err != nil || account.UID != 1026 || account.GID != 100 || len(account.Groups) != 3 || account.Groups[2] != 65536 {
		t.Fatal("unrelated system alias blocked an eligible DSM account")
	}
	for _, alias := range []string{
		"second-operator:x:1026:100:alias:/:/bin/sh\n",
		"operator:x:1026:100:duplicate:/:/bin/sh\n",
	} {
		if _, err := ResolveFiles("operator", strings.NewReader(passwd+alias), strings.NewReader(groups), 268604); err == nil {
			t.Fatal("selected operator identity ambiguity was accepted")
		}
	}
}

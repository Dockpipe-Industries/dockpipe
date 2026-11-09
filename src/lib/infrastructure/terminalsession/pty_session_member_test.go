//go:build linux || darwin

package terminalsession

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestEnumeratedSessionMemberDisappearsAfterSessionLookup(t *testing.T) {
	for _, gone := range []error{os.ErrNotExist, unix.ESRCH, fmt.Errorf("kernel lookup: %w", unix.ESRCH)} {
		t.Run(gone.Error(), func(t *testing.T) {
			lookedUp := false
			sessionFor := func(pid int) (int, error) {
				if pid != 123 {
					t.Fatal("enumerated PID changed")
				}
				lookedUp = true
				return 456, nil
			}
			inspect := func(pid int) (processIdentity, error) {
				if !lookedUp || pid != 123 {
					t.Fatal("identity inspection did not follow matching session lookup")
				}
				return processIdentity{}, gone
			}
			_, member, err := inspectEnumeratedSessionMember(123, 456, sessionFor, inspect)
			if member || err != nil {
				t.Fatalf("proven disappearance made cleanup uncertain: member=%v error=%v", member, err)
			}
		})
	}
}

func TestEnumeratedSessionMemberRetainsInspectionUncertainty(t *testing.T) {
	for _, uncertain := range []error{os.ErrPermission, unix.EPERM, errors.New("kernel inspection failed")} {
		for _, stage := range []string{"session", "identity"} {
			t.Run(stage+"/"+uncertain.Error(), func(t *testing.T) {
				sessionFor := func(int) (int, error) {
					if stage == "session" {
						return 0, uncertain
					}
					return 456, nil
				}
				inspect := func(int) (processIdentity, error) {
					if stage == "session" {
						t.Fatal("inspection continued after an uncertain session lookup")
					}
					return processIdentity{}, uncertain
				}
				_, member, err := inspectEnumeratedSessionMember(123, 456, sessionFor, inspect)
				if member || !errors.Is(err, ErrCleanupIncomplete) || !errors.Is(err, uncertain) {
					t.Fatalf("inspection uncertainty was hidden: member=%v error=%v", member, err)
				}
				boundary := "inspect enumerated"
				if stage == "session" {
					boundary = "lookup session"
				}
				if !strings.Contains(err.Error(), boundary) || !strings.Contains(err.Error(), "pid=123") || !strings.Contains(err.Error(), "session=456") {
					t.Fatalf("inspection failure lost safe boundary provenance: %v", err)
				}
			})
		}
	}
}

func TestEnumeratedSessionMemberRetainsIdentityAndRechecksSession(t *testing.T) {
	identity := processIdentity{pid: 123, session: 456, group: 789, birth: "birth", zombie: true}
	sessionFor := func(int) (int, error) { return 456, nil }
	inspect := func(int) (processIdentity, error) { return identity, nil }
	observed, member, err := inspectEnumeratedSessionMember(123, 456, sessionFor, inspect)
	if err != nil || !member || observed != identity {
		t.Fatalf("native birth/group identity was lost: %+v member=%v error=%v", observed, member, err)
	}
	identity.session = 999
	_, member, err = inspectEnumeratedSessionMember(123, 456, sessionFor, inspect)
	if err != nil || member {
		t.Fatal("process that changed sessions was retained")
	}
}

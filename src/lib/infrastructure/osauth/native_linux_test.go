//go:build linux && (amd64 || arm64)

package osauth

import (
	"math"
	"testing"
)

func TestNativeSessionRejectsOutOfRangeIdentityBeforeOSAccess(t *testing.T) {
	for _, pid := range []int{-1, 0, math.MaxInt32 + 1} {
		if _, err := nativeProcessSession(pid, 0); err == nil {
			t.Fatalf("invalid process ID %d reached native session lookup", pid)
		}
	}
	for _, uid := range []int{-1, math.MaxUint32, math.MaxUint32 + 1} {
		if _, err := nativeProcessSession(1, uid); err == nil {
			t.Fatalf("invalid account ID %d reached native process lookup", uid)
		}
		if _, err := nativeDisplaySession(uid); err == nil {
			t.Fatalf("invalid account ID %d reached native desktop lookup", uid)
		}
		// A nil library proves rejection precedes any native function call.
		if err := verifyNativeSession(nil, "session", uid); err == nil {
			t.Fatalf("invalid account ID %d reached native session verification", uid)
		}
	}
	for _, uid := range []int{0, math.MaxUint32 - 1} {
		nativeUID, err := checkedNativeUID(uid)
		if err != nil || uint64(nativeUID) != uint64(uid) {
			t.Fatalf("valid account ID %d was changed or rejected: %d, %v", uid, nativeUID, err)
		}
	}
}

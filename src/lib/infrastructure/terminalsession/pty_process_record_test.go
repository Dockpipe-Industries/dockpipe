//go:build linux || darwin

package terminalsession

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestFixedProcessRecordDistinguishesDisappearanceFromKernelFailure(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		length uintptr
		err    error
		want   error
	}{
		{name: "complete", length: 32},
		{name: "exited", length: 0, want: os.ErrNotExist},
		{name: "short", length: 31, want: ErrCleanupIncomplete},
		{name: "oversized", length: 33, want: ErrCleanupIncomplete},
		{name: "kernel EIO", length: 32, err: unix.EIO, want: unix.EIO},
		{name: "kernel EIO with empty output", length: 0, err: unix.EIO, want: unix.EIO},
		{name: "kernel permission with empty output", length: 0, err: unix.EPERM, want: unix.EPERM},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			record := make([]byte, 32)
			called := false
			err := readFixedProcessRecord(record, func(buffer []byte) (uintptr, error) {
				called = true
				if len(buffer) != len(record) || &buffer[0] != &record[0] {
					t.Fatal("kernel query did not receive the caller's fixed record")
				}
				buffer[0] = 42
				return fixture.length, fixture.err
			})
			if !called || !errors.Is(err, fixture.want) {
				t.Fatalf("query result changed: called=%v error=%v expected=%v", called, err, fixture.want)
			}
			if fixture.want == nil && record[0] != 42 {
				t.Fatal("complete kernel identity was not retained")
			}
			if fixture.err != nil && errors.Is(err, os.ErrNotExist) {
				t.Fatal("kernel failure was mistaken for proven disappearance")
			}
		})
	}
}

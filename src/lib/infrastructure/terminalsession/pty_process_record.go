//go:build linux || darwin

package terminalsession

import (
	"fmt"
	"os"
)

// A fixed native process record has exactly one valid nonempty size. A
// successful empty query proves disappearance; an errno never does, even when
// the kernel also changes its output length to zero.
func readFixedProcessRecord(record []byte, query func([]byte) (uintptr, error)) error {
	length, err := query(record)
	if err != nil {
		return err
	}
	if length == 0 {
		return os.ErrNotExist
	}
	if length != uintptr(len(record)) {
		return fmt.Errorf("%w: native process record length=%d expected=%d", ErrCleanupIncomplete, length, len(record))
	}
	return nil
}

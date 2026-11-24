//go:build linux || darwin

package terminalsession

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// A process-table entry can disappear before either subsequent kernel lookup.
// Only proven disappearance can be omitted from a complete session snapshot.
func inspectEnumeratedSessionMember(pid, session int, sessionFor func(int) (int, error), inspect func(int) (processIdentity, error)) (processIdentity, bool, error) {
	candidateSession, err := sessionFor(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ESRCH) {
			return processIdentity{}, false, nil
		}
		return processIdentity{}, false, fmt.Errorf("%w: lookup session pid=%d expected_session=%d: %w", ErrCleanupIncomplete, pid, session, err)
	}
	if candidateSession != session {
		return processIdentity{}, false, nil
	}
	identity, err := inspect(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ESRCH) {
			return processIdentity{}, false, nil
		}
		return processIdentity{}, false, fmt.Errorf("%w: inspect enumerated pid=%d session=%d: %w", ErrCleanupIncomplete, pid, session, err)
	}
	return identity, identity.session == session, nil
}

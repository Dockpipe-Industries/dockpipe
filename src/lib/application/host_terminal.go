package application

import (
	"math"
	"os"
)

// stdinFDInt converts os.Stdin's file descriptor to int without truncation (G115).
func stdinFDInt() (int, bool) {
	fd := os.Stdin.Fd()
	if fd > uintptr(math.MaxInt) {
		return 0, false
	}
	return int(fd), true
}

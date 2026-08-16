package infrastructure

import (
	"os"

	"dockpipe/src/lib/infrastructure/operationrecord"
)

// StartLineSpinner draws an indeterminate status line on w until stop is called.
// It is a no-op when w is not a terminal (e.g. CI, pipes).
func StartLineSpinner(w *os.File, message string) (stop func()) {
	return operationrecord.StartLineSpinner(w, message)
}

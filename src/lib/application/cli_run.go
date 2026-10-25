package application

import (
	"errors"
	"os"
)

// Run is the CLI entry (after stripping os.Args[0]).
func Run(argv []string, baseEnviron []string) error {
	if baseEnviron == nil {
		baseEnviron = os.Environ()
	}
	if len(argv) == 0 {
		printUsage()
		return nil
	}
	if err := checkCLIProjectVersion(argv); err != nil {
		return err
	}
	if handled, err := dispatchCommand(argv); handled {
		return err
	}
	err := executeWorkflowRun(argv, baseEnviron, snapshotRunExecutionServices())
	var exit *runProcessExit
	if errors.As(err, &exit) {
		osExitAppFn(exit.code)
		return nil
	}
	return err
}

// Only execution branches that historically terminated the CLI use this outcome.
// Other returned errors retain the CLI's ordinary error handling.
type runProcessExit struct {
	code int
}

func (exit *runProcessExit) Error() string {
	return (&workflowExitCodeError{Code: exit.code}).Error()
}

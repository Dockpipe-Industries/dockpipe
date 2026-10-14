package process

import "os/exec"

// StartedObserver receives the actual process after Start, and returns its
// completion callback. No observer may replace process-tree supervision.
type StartedObserver func(*exec.Cmd) func(error)

func Run(command *exec.Cmd) error {
	return RunObserved(command, nil)
}

func RunObserved(command *exec.Cmd, observer StartedObserver) error {
	var finish func(error)
	started := func() {
		if observer != nil {
			finish = observer(command)
		}
	}
	err := run(command, started)
	if observer != nil && finish == nil {
		finish = observer(command)
	}
	if finish != nil {
		finish(err)
	}
	return err
}

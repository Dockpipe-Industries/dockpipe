//go:build !windows

package infrastructure

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func waitCommandWithSignalForward(cmd *exec.Cmd) error {
	sigCh := make(chan os.Signal, 8)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigCh)
	return waitCommandWithSignals(cmd, sigCh)
}

// A Linux host observation failure transfers its existing signal subscription
// here after joining group forwarding, so queued cancellation is not discarded.
func waitCommandWithSignals(cmd *exec.Cmd, sigCh <-chan os.Signal) error {
	done := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		for {
			select {
			case sig, open := <-sigCh:
				if !open {
					return
				}
				select {
				case <-done:
					return
				default:
				}
				if cmd.Process != nil {
					_ = cmd.Process.Signal(sig)
				}
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	<-joined
	return err
}

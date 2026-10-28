package remotecmd

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"golang.org/x/term"
)

// pkttyagent speaks to the OS directly. Dockpipe never reads or forwards the OS
// password. Graphical callers use the existing desktop authentication agent.
func startTerminalSecurityAgent(ctx context.Context) (func(), error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return func() {}, nil
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	command := exec.CommandContext(ctx, "/usr/bin/pkttyagent", "--process", strconv.Itoa(os.Getpid()), "--notify-fd", "3", "--fallback")
	command.ExtraFiles = []*os.File{writer}
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stderr, os.Stderr
	command.WaitDelay = time.Second
	if err := command.Start(); err != nil {
		// No child inherited this pipe; cleanup cannot recover the failed start.
		_ = writer.Close()
		return nil, errors.New("terminal OS authentication requires the installed PolicyKit terminal agent")
	}
	// The child owns its inherited descriptor; discard this local duplicate.
	_ = writer.Close()
	closeAgent := func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}
	if err := reader.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		closeAgent()
		return nil, err
	}
	var notification [1]byte
	// pkttyagent closes the notification descriptor when registration finishes.
	if count, err := reader.Read(notification[:]); count != 0 || (err != nil && !errors.Is(err, io.EOF)) {
		closeAgent()
		return nil, errors.New("terminal OS authentication agent did not become ready")
	}
	return closeAgent, nil
}

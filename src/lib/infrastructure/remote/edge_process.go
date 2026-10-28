package remote

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type managedEdge interface {
	alive() bool
	stop()
	digest() (string, error)
	ownsReadiness(string) bool
	snapshot(string, string) error
}

type edgeChild struct {
	command *exec.Cmd
	cancel  context.CancelFunc
	done    chan struct{}
}

func startEdgeChild(ctx context.Context, config contract.EdgeConfig) (managedEdge, error) {
	childContext, cancel := context.WithCancel(ctx)
	command := exec.CommandContext(childContext, config.Executable, config.Arguments...)
	for _, name := range []string{"PATH", "HOME", "USER", "TMPDIR", "LANG"} {
		if value, exists := os.LookupEnv(name); exists {
			command.Env = append(command.Env, name+"="+value)
		}
	}
	command.Stdout, command.Stderr = os.Stderr, os.Stderr
	command.WaitDelay = time.Second
	ContainProcess(command)
	bindEdgeParent(command)
	if err := command.Start(); err != nil {
		cancel()
		return nil, err
	}
	child := &edgeChild{command: command, cancel: cancel, done: make(chan struct{})}
	go func() { _ = command.Wait(); close(child.done) }()
	return child, nil
}

func (child *edgeChild) alive() bool {
	select {
	case <-child.done:
		return false
	default:
		return true
	}
}

func (child *edgeChild) stop() {
	if child.alive() {
		signalEdgeTermination(child.command)
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-child.done:
			timer.Stop()
		case <-timer.C:
			child.cancel()
		}
	}
	child.cancel()
	CleanupProcess(child.command)
	select {
	case <-child.done:
	case <-time.After(2 * time.Second):
	}
}

func (child *edgeChild) digest() (string, error) {
	if !child.alive() {
		return "", errors.New("edge process is not running")
	}
	return edgeImageDigest(child.command.Process.Pid)
}

func (child *edgeChild) ownsReadiness(address string) bool {
	return child.alive() && edgeOwnsSocket(child.command.Process.Pid, address)
}

func (child *edgeChild) snapshot(destination, expected string) error {
	if !child.alive() {
		return errors.New("baseline process exited before snapshot")
	}
	return snapshotEdgeProcessImage(child.command.Process.Pid, destination, expected)
}

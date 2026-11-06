package nativehost

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// DetachWorker starts this executing CLI image as an ordinary user, without
// inheriting request credentials or standard streams. Startup succeeds only
// after the reverse broker acknowledges the child worker.
func DetachWorker(ctx context.Context, options WorkerOptions) error {
	if !validSocketPath(options.Socket) || !validToken(options.Nonce) {
		return errors.New("invalid native host worker options")
	}
	if _, err := workerIdentity(); err != nil {
		return err
	}
	readiness, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer readiness.Close()
	defer writer.Close()
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	command := exec.Command("/proc/self/exe", "host", "worker", "--socket", options.Socket, "--nonce", options.Nonce, "--ready-fd", "3")
	command.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8"}
	command.Stdin = null
	command.Stdout = null
	command.Stderr = null
	command.ExtraFiles = []*os.File{writer}
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	writer.Close()
	startup, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		var ack [1]byte
		_, err := io.ReadFull(readiness, ack[:])
		if err == nil && ack[0] != 1 {
			err = ErrUnavailable
		}
		result <- err
	}()
	select {
	case err := <-result:
		if err == nil {
			return command.Process.Release()
		}
	case <-startup.Done():
	}
	command.Process.Kill()
	command.Wait()
	return ErrUnavailable
}

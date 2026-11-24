package localterminal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"dockpipe/src/lib/infrastructure/terminalsession"
)

// InstalledProvider resolves only an explicit operator override or the installed
// core's sibling. Neither the project cwd nor PATH participates. Resolution and
// executable preconditions run inside admitted Execute, never during Prepare.
func InstalledProvider(ctx context.Context, profile terminalsession.Profile, size terminalsession.Size) (terminalsession.Backend, error) {
	path, err := installedProviderPath()
	if err != nil {
		return nil, err
	}
	return startProvider(ctx, path, profile, size)
}

// InstalledProviderAvailable reports whether the owner-installed companion is
// present. Opening and verifying a PTY still requires the admitted provider.
func InstalledProviderAvailable() bool {
	_, err := installedProviderPath()
	return err == nil
}

func installedProviderPath() (string, error) {
	path := os.Getenv("DOCKPIPE_TERMINAL_PROVIDER")
	if path == "" {
		self, err := os.Executable()
		if err != nil {
			return "", errors.New("terminal provider unavailable")
		}
		name := "dockpipe-terminal-host"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		path = filepath.Join(filepath.Dir(self), name)
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("terminal provider must be an absolute installed executable")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return "", errors.New("terminal provider unavailable")
	}
	return path, nil
}

type providerBackend struct {
	mutex           sync.Mutex
	command         *exec.Cmd
	stdin           io.WriteCloser
	stdout          io.ReadCloser
	output          *io.PipeReader
	outputWriter    *io.PipeWriter
	requests        chan Command
	ready           chan struct{}
	done            chan struct{}
	processDone     chan struct{}
	deadlineChanged chan struct{}
	nextID          uint64
	pending         map[uint64]chan Event
	deadline        time.Time
	result          Event
	failed          bool
	closeOnce       sync.Once
}

func startProvider(ctx context.Context, path string, profile terminalsession.Profile, size terminalsession.Size) (terminalsession.Backend, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Empty argument/environment arrays are explicit snapshots, not JSON null or
	// permission for the native provider to inherit or rediscover defaults.
	if profile.Args == nil {
		profile.Args = []string{}
	}
	if profile.Env == nil {
		profile.Env = []string{}
	}
	command := exec.Command(path, "--stdio")
	configureProviderProcess(command)
	// The provider never inherits the selected shell environment. Its only shell
	// configuration is the owned start frame, and stderr cannot leak diagnostics.
	command.Stderr = io.Discard
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, errors.New("terminal provider unavailable")
	}
	stdout, childOutput, err := os.Pipe()
	if err != nil {
		_ = stdin.Close()
		return nil, errors.New("terminal provider unavailable")
	}
	command.Stdout = childOutput
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = childOutput.Close()
		return nil, errors.New("terminal provider unavailable")
	}
	_ = childOutput.Close()
	reader, writer := io.Pipe()
	backend := &providerBackend{command: command, stdin: stdin, stdout: stdout, output: reader, outputWriter: writer,
		requests: make(chan Command, 4), ready: make(chan struct{}), done: make(chan struct{}), processDone: make(chan struct{}),
		deadlineChanged: make(chan struct{}, 1), nextID: 1, pending: make(map[uint64]chan Event)}
	go backend.writeFrames()
	go backend.readFrames()
	go func() { _ = command.Wait(); close(backend.processDone) }()
	backend.requests <- Command{Schema: Schema, Type: "start", ID: 1, Profile: &profile, Columns: size.Columns, Rows: size.Rows}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-backend.ready:
		return backend, nil
	case <-ctx.Done():
	case <-timer.C:
	case <-backend.done:
	}
	cleanup := backend.Stop()
	_ = backend.Close()
	return nil, errors.Join(errors.New("terminal provider start failed"), cleanup)
}

func (backend *providerBackend) writeFrames() {
	encoder := json.NewEncoder(backend.stdin)
	for {
		select {
		case <-backend.done:
			return
		case request := <-backend.requests:
			if request.Type == "input" {
				backend.mutex.Lock()
				expired := !time.Now().Before(backend.deadline)
				backend.mutex.Unlock()
				if expired {
					clear(request.Data)
					continue
				}
			}
			err := encoder.Encode(request)
			clear(request.Data)
			if err != nil {
				_ = backend.stdout.Close()
				return
			}
		}
	}
}

func (backend *providerBackend) readFrames() {
	defer close(backend.done)
	defer backend.outputWriter.Close()
	scanner := bufio.NewScanner(backend.stdout)
	scanner.Buffer(make([]byte, 4096), MaxCommandLine)
	ready := false
	for scanner.Scan() {
		var event Event
		if decode(scanner.Bytes(), &event) != nil || !validProviderEvent(event, ready) {
			return
		}
		switch event.Type {
		case "ready":
			ready = true
			close(backend.ready)
		case "ack":
			backend.mutex.Lock()
			pending := backend.pending[event.ID]
			backend.mutex.Unlock()
			if pending == nil {
				return
			}
			select {
			case pending <- event:
			default:
				return
			}
		case "output":
			if _, err := backend.outputWriter.Write(event.Data); err != nil {
				return
			}
		case "closed":
			backend.mutex.Lock()
			backend.result = event
			backend.mutex.Unlock()
			return
		case "error":
			backend.mutex.Lock()
			backend.failed = true
			backend.mutex.Unlock()
		}
	}
}

func validProviderEvent(event Event, ready bool) bool {
	if event.Schema != Schema || event.ID > MaxID || event.SessionID != "" || event.Cursor != 0 || event.Oldest != 0 || event.Truncated {
		return false
	}
	switch event.Type {
	case "ready":
		return !ready && event.ID == 1 && event.Bytes == 0 && len(event.Data) == 0 && event.Cleanup == "" && event.Code == "" && event.ExitCode == 0
	case "ack":
		return ready && event.ID > 1 && event.Bytes >= 0 && event.Bytes <= terminalsession.MaxInput && len(event.Data) == 0 && event.Cleanup == "" && event.Code == "" && event.ExitCode == 0
	case "output":
		return ready && event.ID == 0 && len(event.Data) > 0 && len(event.Data) <= MaxOutputChunk && event.Bytes == 0 && event.Cleanup == "" && event.Code == "" && event.ExitCode == 0
	case "closed":
		return event.ID == 0 && (event.Cleanup == "complete" || event.Cleanup == "unknown") && len(event.Data) == 0 && event.Bytes == 0 && event.Code == ""
	case "error":
		if len(event.Data) != 0 || event.Bytes != 0 || event.Cleanup != "" || event.ExitCode != 0 {
			return false
		}
		switch event.Code {
		case "invalid_frame", "invalid_profile", "start_failed", "input_failed", "output_overflow", "cleanup_unknown", "resize_failed", "provider_failed":
			return true
		}
	}
	return false
}

func (backend *providerBackend) Read(data []byte) (int, error) { return backend.output.Read(data) }

func (backend *providerBackend) SetWriteDeadline(deadline time.Time) error {
	backend.mutex.Lock()
	backend.deadline = deadline
	backend.mutex.Unlock()
	select {
	case backend.deadlineChanged <- struct{}{}:
	default:
	}
	return nil
}

func (backend *providerBackend) request(command Command, useInputDeadline bool) (Event, error) {
	backend.mutex.Lock()
	if backend.nextID == MaxID || len(backend.pending) >= 3 {
		backend.mutex.Unlock()
		return Event{}, ErrProtocol
	}
	backend.nextID++
	command.ID, command.Schema = backend.nextID, Schema
	pending := make(chan Event, 1)
	backend.pending[command.ID] = pending
	select {
	case backend.requests <- command:
	case <-backend.done:
		delete(backend.pending, command.ID)
		backend.mutex.Unlock()
		clear(command.Data)
		return Event{}, terminalsession.ErrClosed
	default:
		delete(backend.pending, command.ID)
		backend.mutex.Unlock()
		clear(command.Data)
		return Event{}, terminalsession.ErrInputBusy
	}
	backend.mutex.Unlock()
	defer func() { backend.mutex.Lock(); delete(backend.pending, command.ID); backend.mutex.Unlock() }()
	until := time.Now().Add(2 * time.Second)
	for {
		if useInputDeadline {
			backend.mutex.Lock()
			until = backend.deadline
			backend.mutex.Unlock()
		}
		timer := time.NewTimer(time.Until(until))
		select {
		case event := <-pending:
			timer.Stop()
			return event, nil
		case <-backend.done:
			timer.Stop()
			return Event{}, terminalsession.ErrClosed
		case <-timer.C:
			return Event{}, terminalsession.ErrInputUncertain
		case <-backend.deadlineChanged:
			timer.Stop()
		}
	}
}

func (backend *providerBackend) Write(data []byte) (int, error) {
	event, err := backend.request(Command{Type: "input", Data: append([]byte(nil), data...)}, true)
	if err != nil || event.Bytes != len(data) {
		return 0, terminalsession.ErrInputUncertain
	}
	return len(data), nil
}

func (backend *providerBackend) Resize(size terminalsession.Size) error {
	event, err := backend.request(Command{Type: "resize", Columns: size.Columns, Rows: size.Rows}, false)
	if err != nil {
		return err
	}
	if event.Bytes != 0 {
		return ErrProtocol
	}
	return nil
}

func (backend *providerBackend) Wait() (int, error) {
	<-backend.done
	backend.mutex.Lock()
	defer backend.mutex.Unlock()
	if backend.result.Type != "closed" || backend.failed {
		return backend.result.ExitCode, ErrProtocol
	}
	return backend.result.ExitCode, nil
}

func (backend *providerBackend) Stop() error {
	backend.mutex.Lock()
	backend.nextID++
	command := Command{Schema: Schema, Type: "close", ID: backend.nextID}
	select {
	case backend.requests <- command:
	default:
	}
	backend.mutex.Unlock()
	// The native provider's bounded session/job cleanup may take three seconds.
	// Leave one additional second to deliver its explicit completion receipt.
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	select {
	case <-backend.done:
	case <-timer.C:
		return terminalsession.ErrCleanupIncomplete
	}
	backend.mutex.Lock()
	defer backend.mutex.Unlock()
	if backend.result.Cleanup != "complete" {
		return terminalsession.ErrCleanupIncomplete
	}
	return nil
}

func (backend *providerBackend) Close() error {
	backend.closeOnce.Do(func() {
		_ = backend.stdin.Close()
		_ = backend.stdout.Close()
		_ = backend.output.Close()
		_ = backend.command.Process.Kill()
	})
	return nil
}

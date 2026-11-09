package infrastructure

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

// DockerExecReceipt contains only daemon/resource routing and launch phase.
// Closed means the owned ExecID entrypoint stopped and its relay closed. It does
// not mean arbitrary shell descendants or persistent container effects vanished.
type DockerExecReceipt struct {
	ContainerLifecycleReceipt
	ExecID string `json:"exec_id,omitempty"`
	Phase  string `json:"phase"`
}

func ValidateDockerExecReceipt(receipt DockerExecReceipt) error {
	exact := func(value string) bool { return isFullContainerID(value) }
	if !exact(receipt.Container) || !exact(receipt.Provider) || receipt.Daemon != "" && !exact(receipt.Daemon) || receipt.ExecID != "" && !exact(receipt.ExecID) {
		return errors.New("invalid Docker exec receipt identity")
	}
	switch receipt.Phase {
	case "planned":
		if receipt.Completed || receipt.ExecID != "" {
			return errors.New("invalid planned Docker exec receipt")
		}
	case "create-requested":
		if !exact(receipt.Daemon) || receipt.Completed {
			return errors.New("invalid pending Docker exec receipt")
		}
	case "start-requested", "running", "closed":
		if !exact(receipt.Daemon) || !exact(receipt.ExecID) || receipt.Completed != (receipt.Phase == "closed") {
			return errors.New("invalid started Docker exec receipt")
		}
	default:
		return errors.New("unsupported Docker exec receipt phase")
	}
	return nil
}

type DockerTerminal struct {
	Manager *terminalsession.Manager
	Session *terminalsession.Session
}

// OpenContainerTerminal owns fixed Docker execution through the authenticated
// terminal transport. The observer must durably capture each phase before its
// corresponding external effect. No profile or terminal bytes enter receipts.
func OpenContainerTerminal(ctx context.Context, request ContainerExecRequest, environment []string,
	target invocation.Target, identity invocation.Identity, authorize invocation.Authorize,
	size terminalsession.Size, lease time.Duration, observe func(DockerExecReceipt) error) (DockerTerminal, error) {
	id := invocation.ID{Capability: "dockpipe.containers", Action: "exec", Version: "1"}
	definition := invocation.Definition{ID: id, Target: target, RequireAuthorization: true}
	native, err := invocation.New(definition, func(_ context.Context, owned ContainerExecRequest) (invocation.Prepared[DockerTerminal], error) {
		if err := ValidateContainerExecRequest(owned); err != nil {
			return invocation.Prepared[DockerTerminal]{}, err
		}
		if target.Kind != invocation.Remote || observe == nil || lease <= 0 || lease > terminalsession.MaxLease {
			return invocation.Prepared[DockerTerminal]{}, invocation.ErrInvalidCall
		}
		frozen := append([]string(nil), environment...)
		if err := ValidateNativeExecutionEnvironment(frozen); err != nil {
			return invocation.Prepared[DockerTerminal]{}, err
		}
		return invocation.Prepared[DockerTerminal]{Digest: NativeExecutionDigest([]string{owned.Container, owned.Shell}, frozen),
			Execute: func(ctx context.Context) (DockerTerminal, error) {
				name, err := resolveDockerCommandFromEnvironment(frozen)
				if err != nil {
					return DockerTerminal{}, err
				}
				client := &dockerEngineClient{name: name, environment: frozen}
				receipt := DockerExecReceipt{ContainerLifecycleReceipt: ContainerLifecycleReceipt{Container: owned.Container,
					Provider: ContainerProviderBinding(name, frozen)}, Phase: "planned"}
				if err := observe(receipt); err != nil {
					return DockerTerminal{}, err
				}
				connection, err := client.connect(ctx)
				if err != nil {
					return DockerTerminal{}, err
				}
				if err := connection.Close(); err != nil {
					return DockerTerminal{}, err
				}
				receipt.Daemon = client.daemon
				profile := terminalsession.Profile{ID: "container-exec", Executable: name, Dir: filepath.Dir(name),
					Args: []string{owned.Container, owned.Shell}, Env: frozen}
				authority := terminalsession.Authority{Target: target, Identity: identity, Authorize: authorize}
				manager, err := terminalsession.NewOwnedProvider(profile, authority,
					func(ctx context.Context, _ terminalsession.Profile, size terminalsession.Size) (terminalsession.Backend, error) {
						return startDockerTerminal(ctx, client, owned, receipt, size, observe)
					})
				if err != nil {
					return DockerTerminal{}, err
				}
				session, err := manager.OpenAs(ctx, size, lease, identity, authorize)
				if err != nil {
					return DockerTerminal{}, err
				}
				return DockerTerminal{Manager: manager, Session: session}, nil
			}}, nil
	}, authorize)
	if err != nil {
		return DockerTerminal{}, err
	}
	return native.Invoke(ctx, invocation.Call{Contract: id, Target: target, Identity: identity}, request)
}

type dockerTerminal struct {
	ctx             context.Context
	client          *dockerEngineClient
	connection      *dockerEngineChannel
	reader          *bufio.Reader
	receipt         DockerExecReceipt
	observe         func(DockerExecReceipt) error
	readDone        chan struct{}
	readOnce        sync.Once
	stopOnce        sync.Once
	stopError       error
	relayCancel     context.CancelFunc
	writeMutex      sync.Mutex
	writeStateMutex sync.Mutex
	stopping        bool
}

func startDockerTerminal(ctx context.Context, client *dockerEngineClient, request ContainerExecRequest,
	receipt DockerExecReceipt, size terminalsession.Size, observe func(DockerExecReceipt) error) (*dockerTerminal, error) {
	receipt.Phase = "create-requested"
	if err := observe(receipt); err != nil {
		return nil, err
	}
	var created struct {
		ID string `json:"Id"`
	}
	selection := struct {
		AttachStdin  bool     `json:"AttachStdin"`
		AttachStdout bool     `json:"AttachStdout"`
		AttachStderr bool     `json:"AttachStderr"`
		Tty          bool     `json:"Tty"`
		Cmd          []string `json:"Cmd"`
	}{true, true, true, true, []string{request.Shell}}
	if err := client.query(ctx, "POST", "/containers/"+request.Container+"/exec", selection, &created); err != nil {
		return nil, err
	}
	if !isFullContainerID(created.ID) {
		return nil, errors.New("Docker returned an invalid exec identity")
	}
	receipt.ExecID, receipt.Phase = created.ID, "start-requested"
	if err := observe(receipt); err != nil {
		return nil, err
	}
	// Startup still follows the admission/session context. Once attached,
	// the backend retains the relay long enough to deliver bounded teardown
	// after that context is cancelled. Stop/Close own the detached lifetime.
	relayContext, relayCancel := context.WithCancel(context.WithoutCancel(ctx))
	interruptStartup := context.AfterFunc(ctx, relayCancel)
	transferred := false
	defer func() {
		interruptStartup()
		if !transferred {
			relayCancel()
		}
	}()
	connection, err := client.connect(relayContext)
	if err != nil {
		return nil, err
	}
	body := []byte(`{"Detach":false,"Tty":true}`)
	response, err := connection.roundTrip("POST", "/exec/"+receipt.ExecID+"/start", body, true)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	if response.StatusCode != 101 {
		_ = response.Body.Close()
		_ = connection.Close()
		return nil, errors.New("Docker exec attachment was not acknowledged")
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		_ = connection.Close()
		return nil, err
	}
	terminal := &dockerTerminal{ctx: ctx, client: client, connection: connection, reader: connection.reader,
		receipt: receipt, observe: observe, readDone: make(chan struct{}), relayCancel: relayCancel}
	startup, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for {
		running, _, err := terminal.inspect(startup)
		if err != nil {
			_ = terminal.Stop()
			return nil, err
		}
		if running {
			terminal.receipt.Phase = "running"
			if err := observe(terminal.receipt); err != nil {
				_ = terminal.Stop()
				return nil, err
			}
			break
		}
		select {
		case <-startup.Done():
			_ = terminal.Stop()
			return nil, errors.New("Docker exec launch completion could not be verified")
		case <-time.After(25 * time.Millisecond):
		}
	}
	if err := terminal.Resize(size); err != nil {
		_ = terminal.Stop()
		return nil, err
	}
	if !interruptStartup() || ctx.Err() != nil {
		_ = terminal.Stop()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("Docker exec startup was cancelled")
	}
	transferred = true
	return terminal, nil
}

func (terminal *dockerTerminal) Read(data []byte) (int, error) {
	count, err := terminal.reader.Read(data)
	if err != nil {
		terminal.readOnce.Do(func() { close(terminal.readDone) })
	}
	return count, err
}

func (terminal *dockerTerminal) Write(data []byte) (int, error) {
	terminal.writeMutex.Lock()
	defer terminal.writeMutex.Unlock()
	terminal.writeStateMutex.Lock()
	stopping := terminal.stopping
	terminal.writeStateMutex.Unlock()
	if stopping {
		return 0, terminalsession.ErrClosed
	}
	return terminal.connection.Write(data)
}

func (terminal *dockerTerminal) SetWriteDeadline(deadline time.Time) error {
	terminal.writeStateMutex.Lock()
	defer terminal.writeStateMutex.Unlock()
	if terminal.stopping {
		return terminalsession.ErrClosed
	}
	return terminal.connection.SetWriteDeadline(deadline)
}

func (terminal *dockerTerminal) Resize(size terminalsession.Size) error {
	ctx, cancel := context.WithTimeout(terminal.ctx, 2*time.Second)
	defer cancel()
	path := "/exec/" + terminal.receipt.ExecID + "/resize?h=" + strconv.Itoa(size.Rows) + "&w=" + strconv.Itoa(size.Columns)
	return terminal.client.query(ctx, "POST", path, nil, nil)
}

func (terminal *dockerTerminal) inspect(ctx context.Context) (bool, int, error) {
	var state struct {
		ID        string `json:"ID"`
		Container string `json:"ContainerID"`
		Running   bool   `json:"Running"`
		ExitCode  int    `json:"ExitCode"`
	}
	err := terminal.client.query(ctx, "GET", "/exec/"+terminal.receipt.ExecID+"/json", nil, &state)
	if err != nil || state.ID != terminal.receipt.ExecID || state.Container != terminal.receipt.Container {
		return true, -1, errors.New("Docker exec closure identity could not be verified")
	}
	return state.Running, state.ExitCode, nil
}

func (terminal *dockerTerminal) Wait() (int, error) {
	select {
	case <-terminal.ctx.Done():
		return -1, terminal.ctx.Err()
	case <-terminal.readDone:
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(terminal.ctx), 2*time.Second)
	defer cancel()
	for {
		running, code, err := terminal.inspect(ctx)
		if err != nil {
			return -1, err
		}
		if !running {
			return code, nil
		}
		select {
		case <-ctx.Done():
			return -1, terminalsession.ErrCleanupIncomplete
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// closeEOF is a fixed terminal EOF control byte, not shell source. A shell
// that changes terminal mode, ignores EOF or has a foreground reader may keep
// running; only authoritative ExecInspect can turn closure into success.
func (terminal *dockerTerminal) closeEOF(deadline time.Time) bool {
	terminal.writeStateMutex.Lock()
	terminal.stopping = true
	// Interrupt any already admitted write before taking exclusive ownership.
	// Later deadline callbacks cannot replace the cleanup write deadline.
	_ = terminal.connection.SetWriteDeadline(time.Now())
	terminal.writeStateMutex.Unlock()
	terminal.writeMutex.Lock()
	defer terminal.writeMutex.Unlock()
	select {
	case <-terminal.readDone:
		return false
	default:
	}
	if terminal.receipt.Phase != "running" {
		return false
	}
	if err := terminal.connection.SetWriteDeadline(deadline); err != nil {
		return false
	}
	count, err := terminal.connection.Write([]byte{4})
	return count == 1 && err == nil
}

func (terminal *dockerTerminal) waitForClosure(ctx context.Context) bool {
	for {
		running, _, err := terminal.inspect(ctx)
		if err != nil {
			return false
		}
		if !running && terminal.receipt.Phase == "running" {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (terminal *dockerTerminal) Stop() error {
	terminal.stopOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(terminal.ctx), 2*time.Second)
		defer cancel()
		closed := false
		if terminal.closeEOF(time.Now().Add(250 * time.Millisecond)) {
			grace, stop := context.WithTimeout(ctx, 750*time.Millisecond)
			closed = terminal.waitForClosure(grace)
			stop()
		}
		terminal.stopError = terminal.Close()
		if !closed {
			closed = terminal.waitForClosure(ctx)
		}
		if !closed || terminal.stopError != nil {
			terminal.stopError = errors.Join(terminal.stopError, terminalsession.ErrCleanupIncomplete)
			return
		}
		terminal.receipt.Completed, terminal.receipt.Phase = true, "closed"
		terminal.stopError = terminal.observe(terminal.receipt)
	})
	return terminal.stopError
}

func (terminal *dockerTerminal) Close() error {
	if terminal.relayCancel != nil {
		terminal.relayCancel()
	}
	return terminal.connection.Close()
}

// InspectContainerExecClosure is readback for an existing private receipt. It
// never replays create/start or assumes a missing exec ID proves termination.
func InspectContainerExecClosure(ctx context.Context, receipt DockerExecReceipt) error {
	if err := ValidateDockerExecReceipt(receipt); err != nil {
		return err
	}
	if receipt.Completed {
		return nil
	}
	if !isFullContainerID(receipt.Container) || !isFullContainerID(receipt.ExecID) {
		return terminalsession.ErrCleanupIncomplete
	}
	return inspectContainerExecReceipt(ctx, receipt)
}

func inspectContainerExecReceipt(ctx context.Context, receipt DockerExecReceipt) error {
	environment := os.Environ()
	name, err := resolveDockerCommandFromEnvironment(environment)
	if err != nil || ContainerProviderBinding(name, environment) != receipt.Provider {
		return errors.New("the original Docker exec provider changed")
	}
	terminal := &dockerTerminal{client: &dockerEngineClient{name: name, environment: environment, daemon: receipt.Daemon}, receipt: receipt}
	running, _, err := terminal.inspect(ctx)
	if err != nil || running {
		return terminalsession.ErrCleanupIncomplete
	}
	return nil
}

var _ terminalsession.Backend = (*dockerTerminal)(nil)

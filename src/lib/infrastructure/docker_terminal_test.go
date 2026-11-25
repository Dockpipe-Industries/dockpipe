package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

const dockerEngineFixtureSource = `import sys,os,json,fcntl,urllib.parse
state_path=os.environ['DOCKER_ENGINE_FIXTURE_STATE']

def state(change=None):
    with open(state_path + '.lock', 'a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        try:
            with open(state_path) as source:
                data = json.load(source)
        except FileNotFoundError:
            data = {}
        if change:
            change(data)
            temporary = state_path + '.pending'
            with open(temporary, 'w') as output:
                json.dump(data, output)
            os.replace(temporary, state_path)
        return data

def send(value,status='200 OK'):
    data=json.dumps(value).encode()
    sys.stdout.buffer.write(('HTTP/1.1 '+status+'\r\nContent-Length: '+str(len(data))+'\r\n\r\n').encode()+data)
    sys.stdout.buffer.flush()

while True:
    line=sys.stdin.buffer.readline()
    if not line: break
    method,path,_=line.decode().strip().split(' ')
    headers={}
    while True:
        line=sys.stdin.buffer.readline()
        if line==b'\r\n': break
        key,value=line.decode().split(':',1)
        headers[key.lower()]=value.strip()
    body=sys.stdin.buffer.read(int(headers.get('content-length','0')))
    state(lambda d:d.setdefault('paths',[]).append(path))
    route=path.split('?',1)[0]
    if route=='/version':
        send({'ApiVersion':'1.56','MinAPIVersion':'1.44'})
    elif route.endswith('/info'):
        daemon='fixture-daemon'
        if os.environ.get('DOCKER_ENGINE_FIXTURE_MODE')=='changed-daemon' and state().get('created'):
            daemon='different-daemon'
        send({'ID':daemon})
    elif '/containers/' in route and route.endswith('/exec'):
        request=json.loads(body)
        assert request['Cmd'] in [['/bin/sh'],['/bin/bash']]
        assert request['AttachStdin'] and request['AttachStdout'] and request['AttachStderr'] and request['Tty']
        state(lambda d:d.update(created=True,running=False))
        send({'Id':'b'*64})
    elif route.endswith('/start'):
        state(lambda d:d.update(running=True,attach_pid=os.getpid()))
        if os.environ.get('DOCKER_ENGINE_FIXTURE_MODE')=='lost-start':
            break
        sys.stdout.buffer.write(b'HTTP/1.1 101 UPGRADED\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\nfixture-ready\r\n')
        sys.stdout.buffer.flush()
        line=b''
        while True:
            byte=sys.stdin.buffer.read(1)
            if not byte: break
            if byte==b'\x04':
                state(lambda d:d.update(eof_received=True))
                if os.environ.get('DOCKER_ENGINE_FIXTURE_MODE')=='cooperative-eof':
                    state(lambda d:d.update(running=False))
                    sys.stdout.buffer.write(b'fixture-eof-exited\r\n'); sys.stdout.buffer.flush()
                    break
                continue
            line+=byte
            if byte!=b'\n': continue
            if line.strip()==b'exit':
                state(lambda d:d.update(running=False))
                sys.stdout.buffer.write(b'fixture-exited\r\n'); sys.stdout.buffer.flush()
                break
            sys.stdout.buffer.write(b'echo:'+line.strip()+b'\r\n'); sys.stdout.buffer.flush()
            line=b''
        break
    elif route.endswith('/resize'):
        dimensions=urllib.parse.parse_qs(urllib.parse.urlsplit(path).query)
        state(lambda d:d.update(dimensions=dimensions))
        send({})
    elif route.endswith('/json'):
        current=state()
        if current.get('running') and os.environ.get('DOCKER_ENGINE_FIXTURE_MODE') not in ['stubborn','lost-start','cooperative-eof','ignored-eof']:
            try: os.kill(current['attach_pid'],0)
            except ProcessLookupError:
                state(lambda d:d.update(running=False))
                current=state()
        send({'ID':'b'*64,'ContainerID':'a'*64,'Running':current.get('running',False),'ExitCode':0})
    else:
        send({},'404 Not Found')
`

func nativeDockerEngineFixture(t *testing.T, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX native Docker Engine transport fixture")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source := filepath.Join(root, "engine.py")
	if err := os.WriteFile(source, []byte(dockerEngineFixtureSource), 0600); err != nil {
		t.Fatal(err)
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	provider := filepath.Join(root, "docker")
	script := "#!/bin/sh\n[ \"$#\" = 2 ] && [ \"$1:$2\" = system:dial-stdio ] || exit 8\nexec " + quote(python) + " " + quote(source) + "\n"
	if err := os.WriteFile(provider, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", provider)
	t.Setenv("DOCKER_ENGINE_FIXTURE_STATE", filepath.Join(root, "state.json"))
	t.Setenv("DOCKER_ENGINE_FIXTURE_MODE", mode)
	return root
}

func remoteDockerAuthority() (invocation.Target, invocation.Identity, invocation.Authorize) {
	return invocation.Target{Kind: invocation.Remote, ID: "worker", Fingerprint: strings.Repeat("c", 64), Generation: strings.Repeat("d", 64)},
		invocation.Identity{Issuer: "authenticated", Subject: "operator"}, func(context.Context, invocation.Binding) error { return nil }
}

func waitDockerTerminalOutput(t *testing.T, session *terminalsession.Session, expected string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		output, err := session.Read(context.Background(), 0, terminalsession.MaxOutput)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(output.Data), expected) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("native Docker terminal fixture did not produce expected output")
}

func TestDockerRemoteTerminalFixedAPIInputResizeAndOwnedClosure(t *testing.T) {
	root := nativeDockerEngineFixture(t, "normal")
	target, identity, authorize := remoteDockerAuthority()
	var mutex sync.Mutex
	var receipt DockerExecReceipt
	observe := func(value DockerExecReceipt) error { mutex.Lock(); defer mutex.Unlock(); receipt = value; return nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	terminal, err := OpenContainerTerminal(ctx, ContainerExecRequest{Container: strings.Repeat("a", 64), Shell: "/bin/sh"}, os.Environ(), target, identity, authorize,
		terminalsession.Size{Columns: 80, Rows: 24}, 5*time.Second, observe)
	if err != nil {
		t.Fatal(err)
	}
	waitDockerTerminalOutput(t, terminal.Session, "fixture-ready")
	if err := terminal.Session.Input(ctx, 1, []byte("private fixture input\n")); err != nil {
		t.Fatal(err)
	}
	waitDockerTerminalOutput(t, terminal.Session, "echo:private fixture input")
	if err := terminal.Session.Resize(ctx, terminalsession.Size{Columns: 113, Rows: 37}); err != nil {
		t.Fatal(err)
	}
	if err := terminal.Session.Input(ctx, 2, []byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	wait, stop := context.WithTimeout(context.Background(), 4*time.Second)
	defer stop()
	result, err := terminal.Session.Wait(wait)
	if err != nil || result.CleanupError != nil {
		t.Fatalf("owned exec closure failed: %+v (%v)", result, err)
	}
	mutex.Lock()
	captured := receipt
	mutex.Unlock()
	if !captured.Completed || captured.ExecID != strings.Repeat("b", 64) || captured.Phase != "closed" {
		t.Fatalf("invalid owned closure: %+v", captured)
	}
	data, err := os.ReadFile(filepath.Join(root, "state.json"))
	if err != nil || !strings.Contains(string(data), "1.56") || !strings.Contains(string(data), "113") {
		t.Fatalf("API negotiation/resize missing: %s (%v)", data, err)
	}
	if strings.Contains(string(data), "private fixture input") {
		t.Fatal("input entered fixture metadata")
	}
	if err := terminal.Manager.Release(context.Background(), terminal.Session.ID()); err != nil {
		t.Fatal(err)
	}
}

func TestDockerRemoteTerminalUnknownClosureRetainsSession(t *testing.T) {
	root := nativeDockerEngineFixture(t, "ignored-eof")
	target, identity, authorize := remoteDockerAuthority()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	terminal, err := OpenContainerTerminal(ctx, ContainerExecRequest{Container: strings.Repeat("a", 64), Shell: "/bin/sh"}, os.Environ(), target, identity, authorize,
		terminalsession.Size{Columns: 80, Rows: 24}, 5*time.Second, func(DockerExecReceipt) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	waitDockerTerminalOutput(t, terminal.Session, "fixture-ready")
	cancel()
	wait, stop := context.WithTimeout(context.Background(), 4*time.Second)
	defer stop()
	result, err := terminal.Session.Wait(wait)
	if err != nil || !errors.Is(result.CleanupError, terminalsession.ErrCleanupIncomplete) {
		t.Fatalf("daemon entrypoint became false cleanup: %+v (%v)", result, err)
	}
	if err := terminal.Manager.Release(context.Background(), terminal.Session.ID()); err == nil {
		t.Fatal("unknown daemon exec released its quota")
	}
	state, err := os.ReadFile(filepath.Join(root, "state.json"))
	if err != nil || !strings.Contains(string(state), `"eof_received": true`) {
		t.Fatalf("cooperative EOF was not attempted before retaining uncertainty: %s (%v)", state, err)
	}
}

func TestDockerRemoteTerminalRejectsAuthorityAndChangedDaemon(t *testing.T) {
	root := nativeDockerEngineFixture(t, "changed-daemon")
	target, identity, _ := remoteDockerAuthority()
	denied := errors.New("denied")
	request := ContainerExecRequest{Container: strings.Repeat("a", 64), Shell: "/bin/sh"}
	_, err := OpenContainerTerminal(context.Background(), request, os.Environ(), target, identity, func(context.Context, invocation.Binding) error { return denied },
		terminalsession.Size{Columns: 80, Rows: 24}, time.Second, func(DockerExecReceipt) error { t.Fatal("denied exec wrote receipt"); return nil })
	if !errors.Is(err, denied) {
		t.Fatalf("authority failure lost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "state.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("denied exec reached daemon transport")
	}
	_, _, authorize := remoteDockerAuthority()
	_, err = OpenContainerTerminal(context.Background(), request, os.Environ(), target, identity, authorize,
		terminalsession.Size{Columns: 80, Rows: 24}, 5*time.Second, func(DockerExecReceipt) error { return nil })
	if err == nil {
		t.Fatal("changed daemon accepted exec start")
	}
	data, _ := os.ReadFile(filepath.Join(root, "state.json"))
	if strings.Contains(string(data), "/start") {
		t.Fatal("changed daemon received exec start")
	}
}

func TestDockerEngineNegotiationAndHeaderBound(t *testing.T) {
	for _, versions := range [][2]string{{"1.56", "1.44"}, {"1.41", "1.24"}, {"1.99", "1.44"}} {
		if _, err := dockerEngineVersion(versions[0], versions[1]); err != nil {
			t.Fatal(err)
		}
	}
	for _, versions := range [][2]string{{"1.40", "1.24"}, {"1.99", "1.57"}, {"x", "1.41"}} {
		if _, err := dockerEngineVersion(versions[0], versions[1]); err == nil {
			t.Fatal("unsupported API version accepted")
		}
	}
	reader := &dockerEngineHeaderReader{source: strings.NewReader(strings.Repeat("x", 32*1024+1))}
	if _, err := io.ReadAll(reader); err == nil {
		t.Fatal("oversized Docker API headers accepted")
	}
	var captured DockerExecReceipt
	if err := json.Unmarshal([]byte(`{"container":"a","phase":"planned"}`), &captured); err != nil {
		t.Fatal(err)
	}
}

func TestDockerRemoteTerminalCloseDeliversEOFBeforeRelayDisconnect(t *testing.T) {
	for _, shell := range []string{"/bin/sh", "/bin/bash"} {
		t.Run(shell, func(t *testing.T) {
			root := nativeDockerEngineFixture(t, "cooperative-eof")
			target, identity, authorize := remoteDockerAuthority()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			terminal, err := OpenContainerTerminal(ctx, ContainerExecRequest{Container: strings.Repeat("a", 64), Shell: shell}, os.Environ(), target, identity, authorize,
				terminalsession.Size{Columns: 80, Rows: 24}, 5*time.Second, func(DockerExecReceipt) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			waitDockerTerminalOutput(t, terminal.Session, "fixture-ready")
			// Session close cancels this context before backend.Stop. The
			// attached relay must survive long enough to carry cooperative EOF.
			cancel()
			wait, stop := context.WithTimeout(context.Background(), 4*time.Second)
			defer stop()
			result, err := terminal.Session.Wait(wait)
			if err != nil || result.CleanupError != nil {
				t.Fatalf("default shell close was not verified: %+v (%v)", result, err)
			}
			state, err := os.ReadFile(filepath.Join(root, "state.json"))
			if err != nil || !strings.Contains(string(state), `"eof_received": true`) || !strings.Contains(string(state), `"running": false`) {
				t.Fatalf("relay disconnected before cooperative shell shutdown: %s (%v)", state, err)
			}
			if err := terminal.Manager.Release(context.Background(), terminal.Session.ID()); err != nil {
				t.Fatalf("verified cooperative closure retained its quota: %v", err)
			}
		})
	}
}

type blockedDockerInputConnection struct {
	net.Conn
	entered chan struct{}
	once    sync.Once
}

func (connection *blockedDockerInputConnection) Write(data []byte) (int, error) {
	if len(data) > 1 {
		connection.once.Do(func() { close(connection.entered) })
	}
	return connection.Conn.Write(data)
}

func TestDockerTerminalEOFInterruptsPendingInputWithoutInterleaving(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	connection := &blockedDockerInputConnection{Conn: client, entered: make(chan struct{})}
	terminal := &dockerTerminal{connection: &dockerEngineChannel{Conn: connection},
		receipt: DockerExecReceipt{Phase: "running"}, readDone: make(chan struct{})}
	if err := terminal.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	written := make(chan struct{})
	var writeError error
	go func() {
		_, writeError = terminal.Write([]byte("pending private input"))
		close(written)
	}()
	<-connection.entered
	received := make(chan byte, 1)
	go func() {
		<-written
		_ = server.SetReadDeadline(time.Now().Add(time.Second))
		var data [1]byte
		if _, err := io.ReadFull(server, data[:]); err == nil {
			received <- data[0]
		}
	}()
	if !terminal.closeEOF(time.Now().Add(time.Second)) {
		t.Fatal("cooperative EOF did not replace the interrupted input write")
	}
	<-written
	if writeError == nil {
		t.Fatal("pending unacknowledged input completed during teardown")
	}
	select {
	case value := <-received:
		if value != 4 {
			t.Fatalf("cleanup EOF interleaved with user input: %d", value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup EOF was not delivered")
	}
	if _, err := terminal.Write([]byte("later")); !errors.Is(err, terminalsession.ErrClosed) {
		t.Fatal("stopped session accepted new user input")
	}
	if err := terminal.SetWriteDeadline(time.Now().Add(time.Second)); !errors.Is(err, terminalsession.ErrClosed) {
		t.Fatal("an input deadline callback could overwrite the cleanup deadline")
	}
}

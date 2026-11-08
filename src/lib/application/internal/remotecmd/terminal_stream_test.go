//go:build linux || darwin

package remotecmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/terminalsession"
)

func TestTerminalStreamQuietPTYBecomesReadyBeforeInput(t *testing.T) {
	// Cryptographic pairing is covered in the remote package. This transport
	// fixture verifies the CLI's state framing against a real quiet native PTY.
	target, identity := nativeinvocation.Local()
	manager, err := terminalsession.New(terminalsession.Profile{ID: "quiet", Executable: "/bin/cat", Dir: t.TempDir()}, terminalsession.Authority{Target: target, Identity: identity})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown(context.Background())
	now := time.Now().UTC()
	intent := contract.TerminalOpen{Schema: contract.TerminalVersion, Target: contract.FeatureTarget{Node: "fixture", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Epoch: strings.Repeat("c", 64), Root: "owner", Nonce: strings.Repeat("d", 64), Deadline: now.Add(time.Minute), Columns: 80, Rows: 24}
	intent.ID, _ = intent.Digest()
	var mutex sync.Mutex
	var session *terminalsession.Session
	var acknowledged uint64
	closed := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		view := contract.TerminalView{Reference: intent.Reference(), State: "running", AckSequence: acknowledged}
		var operationError error
		switch request.URL.Path {
		case "/v1/terminal-open":
			session, operationError = manager.Open(context.Background(), terminalsession.Size{Columns: 80, Rows: 24}, 0)
			view.State = "queued"
		case "/v1/terminal-read", "/v1/terminal-pulse":
			var read contract.TerminalRead
			_ = json.NewDecoder(request.Body).Decode(&read)
			if !closed {
				output, err := session.Read(context.Background(), read.Cursor, contract.MaxTerminalChunk)
				operationError = err
				view.Data, view.Cursor, view.End, view.Oldest, view.Truncated = output.Data, output.Cursor, output.Cursor, output.Oldest, output.Truncated
			}
		case "/v1/terminal-input":
			var input contract.TerminalInput
			_ = json.NewDecoder(request.Body).Decode(&input)
			operationError = session.Input(context.Background(), input.Sequence, input.Data)
			if operationError == nil {
				acknowledged, view.AckSequence = input.Sequence, input.Sequence
			}
		case "/v1/terminal-close":
			operationError = session.Close(context.Background())
			_, _ = session.Wait(context.Background())
			closed = true
		case "/v1/terminal-release":
			operationError = manager.Release(context.Background(), session.ID())
		}
		if closed {
			view.State = "closed"
		}
		if operationError != nil {
			writer.WriteHeader(http.StatusConflict)
			return
		}
		_ = json.NewEncoder(writer).Encode(view)
	}))
	defer server.Close()
	client, err := remoteio.NewClient(server.URL, strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	defer inputWriter.Close()
	defer outputReader.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- streamTerminal(ctx, client, intent, inputReader, outputWriter) }()
	decoder := json.NewDecoder(outputReader)
	ready, echoed := false, false
	for !echoed {
		var event terminalEvent
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.Type == "output" && event.View != nil && event.View.State == "running" && !ready {
			if len(event.View.Data) != 0 {
				t.Fatal("quiet PTY unexpectedly produced startup output")
			}
			ready = true
			if err := json.NewEncoder(inputWriter).Encode(terminalCommand{Type: "input", Sequence: 1, Data: []byte("quiet-terminal-ready\n")}); err != nil {
				t.Fatal(err)
			}
		}
		if event.View != nil && strings.Contains(string(event.View.Data), "quiet-terminal-ready") {
			echoed = true
		}
	}
	if !ready {
		t.Fatal("input was required before running state")
	}
	_ = inputWriter.Close()
	// Drain remaining metadata so a full-duplex stdout consumer cannot block
	// the CLI's final acknowledgement/cleanup.
	_, _ = io.Copy(io.Discard, outputReader)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

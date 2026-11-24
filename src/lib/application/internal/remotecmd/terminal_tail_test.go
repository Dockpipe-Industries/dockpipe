package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestTerminalStreamDrainsTruncatedClosedTailWithoutDuplicateBytes(t *testing.T) {
	request := contract.TerminalOpen{Schema: contract.TerminalVersion, Target: contract.FeatureTarget{Node: "tail", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Epoch: strings.Repeat("c", 64), Root: "owner", Nonce: strings.Repeat("d", 64), Deadline: time.Now().UTC().Add(time.Minute), Columns: 80, Rows: 24}
	request.ID, _ = request.Digest()
	const end = uint64(600000)
	const oldest = end - contract.MaxTerminalOutput
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, httpRequest *http.Request) {
		view := contract.TerminalView{Reference: request.Reference(), State: "closed", End: end, Oldest: oldest}
		switch httpRequest.URL.Path {
		case "/v1/terminal-open":
			// State-only replies may describe a ring ahead of the controller's
			// cursor. They must not pretend to deliver its truncation/content.
			view.State, view.Truncated = "running", true
		case "/v1/terminal-pulse":
			var read contract.TerminalRead
			_ = json.NewDecoder(httpRequest.Body).Decode(&read)
			start := read.Cursor
			if start < oldest {
				start, view.Truncated = oldest, true
			}
			count := min(uint64(contract.MaxTerminalChunk), end-start)
			view.Data = bytes.Repeat([]byte("x"), int(count))
			view.Cursor = start + count
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- streamTerminal(ctx, client, request, inputReader, outputWriter) }()
	decoder := json.NewDecoder(outputReader)
	var cursor uint64
	var received int
	var truncated, closed bool
	for !closed {
		var event terminalEvent
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.View == nil {
			continue
		}
		view := event.View
		if view.Cursor < view.Oldest || view.Cursor > view.End {
			t.Fatal("metadata violated output cursor bounds")
		}
		if event.Type == "closed" {
			if len(view.Data) != 0 || view.Cursor != cursor || cursor != end {
				t.Fatal("closed event duplicated bytes or preceded final drain")
			}
			closed = true
			continue
		}
		if event.Type != "output" {
			continue
		}
		if view.Truncated {
			if cursor >= oldest {
				t.Fatal("unexpected repeated truncation")
			}
			cursor, truncated = oldest, true
		}
		if view.Cursor != cursor+uint64(len(view.Data)) {
			t.Fatal("output cursor advanced without matching bytes")
		}
		cursor = view.Cursor
		received += len(view.Data)
	}
	if !truncated || received != contract.MaxTerminalOutput {
		t.Fatal("closed tail was dropped or duplicated")
	}
	_, _ = io.Copy(io.Discard, outputReader)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

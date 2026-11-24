package localterminal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"dockpipe/src/lib/infrastructure/terminalsession"
)

type commandResult struct {
	command Command
	bytes   int
	err     error
}

const outputBatchInterval = 8 * time.Millisecond

// Stream owns exactly one local session. The desktop owns its eight-session
// retained/closing quota; this CLI does not create a machine-wide daemon/quota.
// The first private stdin frame holds the full owned launch snapshot. No frame
// is written to persistent storage or diagnostic logs.
func Stream(parent context.Context, input io.ReadCloser, output io.WriteCloser, authority terminalsession.Authority, provider terminalsession.ProviderFactory) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer input.Close()
	frames := make(chan []byte, 1)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer close(frames)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 4096), MaxStartLine)
		for scanner.Scan() {
			frame := append([]byte(nil), scanner.Bytes()...)
			select {
			case frames <- frame:
			case <-ctx.Done():
				clear(frame)
				return
			}
		}
	}()
	// Interrupt an unresponsive GUI pipe on cancellation; native cleanup does not
	// depend on the frontend continuing to consume events.
	stopOutput := context.AfterFunc(ctx, func() { _ = output.Close(); _ = input.Close() })
	defer stopOutput()
	encoder := json.NewEncoder(output)
	emit := func(event Event) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		event.Schema = Schema
		written := make(chan error, 1)
		go func() { written <- encoder.Encode(event) }()
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case err := <-written:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			cancel()
			return errors.New("local terminal consumer stopped reading")
		}
	}
	failStart := func(id uint64, cleanup string) error {
		_ = emit(Event{Type: "error", ID: id, Code: "start_failed"})
		_ = emit(Event{Type: "closed", Cleanup: cleanup})
		return errors.New("local terminal start failed")
	}
	var first []byte
	select {
	case <-ctx.Done():
		return ctx.Err()
	case first = <-frames:
	}
	start, err := DecodeCommand(first, true)
	clear(first)
	if err != nil {
		return failStart(1, "complete")
	}
	manager, err := terminalsession.NewWithProvider(*start.Profile, authority, provider)
	if err != nil {
		return failStart(1, "complete")
	}
	defer func() {
		cancel()
		cleanup, done := context.WithTimeout(context.Background(), 6*time.Second)
		defer done()
		_ = manager.Shutdown(cleanup)
	}()
	session, err := manager.Open(ctx, terminalsession.Size{Columns: start.Columns, Rows: start.Rows}, 0)
	if err != nil {
		cleanup := "complete"
		if errors.Is(err, terminalsession.ErrCleanupIncomplete) {
			cleanup = "unknown"
		}
		return failStart(1, cleanup)
	}
	if err := emit(Event{Type: "ready", ID: 1, SessionID: session.ID()}); err != nil {
		return err
	}
	var cursor, sequence uint64
	lastID := uint64(1)
	inputBusy, closing := false, false
	results := make(chan commandResult, 1)
	drain := func() (closed bool, more bool, err error) {
		view, err := session.Read(context.Background(), cursor, MaxOutputChunk)
		if err != nil {
			return false, false, err
		}
		cursor = view.Cursor
		if len(view.Data) != 0 || view.Truncated {
			if err := emit(Event{Type: "output", Data: view.Data, Cursor: cursor, Oldest: view.Oldest, Truncated: view.Truncated}); err != nil {
				return false, false, err
			}
		}
		more = len(view.Data) == MaxOutputChunk
		return view.Closed && !more, more, nil
	}
	closeSession := func() { closing = true; _ = session.Close(context.Background()) }
	outputTimer := time.NewTimer(outputBatchInterval)
	outputTimer.Stop()
	defer outputTimer.Stop()
	var nextOutput <-chan time.Time
	scheduleOutput := func() {
		if nextOutput == nil {
			outputTimer.Reset(outputBatchInterval)
			nextOutput = outputTimer.C
		}
	}
	sessionDone := session.Done()
	for {
		// Batch producer wakeups so a continuous output stream leaves time for
		// the frontend's input and paint events. An idle stream never polls.
		var outputReady <-chan struct{}
		if nextOutput == nil {
			outputReady = session.OutputReady()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-frames:
			if !ok {
				frames = nil
				closeSession()
				continue
			}
			command, err := DecodeCommand(frame, false)
			clear(frame)
			if err != nil || command.ID <= lastID || closing {
				_ = emit(Event{Type: "error", ID: command.ID, Code: "invalid_frame"})
				closeSession()
				continue
			}
			lastID = command.ID
			switch command.Type {
			case "input":
				if inputBusy {
					clear(command.Data)
					_ = emit(Event{Type: "error", ID: command.ID, Code: "input_failed"})
					closeSession()
					continue
				}
				inputBusy = true
				sequence++
				go func(command Command, sequence uint64) {
					err := session.Input(ctx, sequence, command.Data)
					count := len(command.Data)
					clear(command.Data)
					command.Data = nil
					results <- commandResult{command: command, bytes: count, err: err}
				}(command, sequence)
			case "resize":
				if err := session.Resize(ctx, terminalsession.Size{Columns: command.Columns, Rows: command.Rows}); err != nil {
					_ = emit(Event{Type: "error", ID: command.ID, Code: "resize_failed"})
					closeSession()
				} else if err := emit(Event{Type: "ack", ID: command.ID}); err != nil {
					return err
				}
			case "close":
				closeSession()
			}
		case result := <-results:
			inputBusy = false
			if result.err != nil {
				_ = emit(Event{Type: "error", ID: result.command.ID, Code: "input_failed"})
				closeSession()
			} else if err := emit(Event{Type: "ack", ID: result.command.ID, Bytes: result.bytes}); err != nil {
				return err
			}
		case <-outputReady:
			scheduleOutput()
		case <-sessionDone:
			sessionDone = nil
			scheduleOutput()
		case <-nextOutput:
			nextOutput = nil
			closed, more, err := drain()
			if err != nil {
				return err
			}
			if closed {
				result, err := session.Wait(context.Background())
				if err != nil {
					return err
				}
				cleanup := "complete"
				if result.CleanupError != nil {
					cleanup = "unknown"
				}
				if result.Cause != nil && !errors.Is(result.Cause, terminalsession.ErrClosed) && !errors.Is(result.Cause, context.Canceled) {
					if err := emit(Event{Type: "error", Code: "provider_failed"}); err != nil {
						return err
					}
				}
				return emit(Event{Type: "closed", Cursor: cursor, ExitCode: result.ExitCode, Cleanup: cleanup})
			}
			if more {
				scheduleOutput()
			}
		}
	}
}

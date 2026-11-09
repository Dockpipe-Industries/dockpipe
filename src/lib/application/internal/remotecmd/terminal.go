package remotecmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

// Terminal transport is private NDJSON on stdin/stdout. It never echoes input,
// places bytes in argv, logs stdout, or uses the workflow execution journal.
type terminalCommand struct {
	Type      string                      `json:"type"`
	Reference *contract.TerminalReference `json:"reference,omitempty"`
	Sequence  uint64                      `json:"sequence,omitempty"`
	Data      []byte                      `json:"data,omitempty"`
	Columns   int                         `json:"columns,omitempty"`
	Rows      int                         `json:"rows,omitempty"`
}

type terminalEvent struct {
	Type      string                      `json:"type"`
	Reference *contract.TerminalReference `json:"reference,omitempty"`
	View      *contract.TerminalView      `json:"view,omitempty"`
	Sequence  uint64                      `json:"sequence,omitempty"`
	Message   string                      `json:"message,omitempty"`
}

func decodeTerminalIntent(data []byte, now time.Time) (contract.TerminalOpen, error) {
	var request contract.TerminalOpen
	if err := decodeFeatureInput(data, &request); err != nil {
		return request, err
	}
	if request.ID == "" {
		var err error
		request.ID, err = request.Digest()
		if err != nil {
			return request, err
		}
	}
	if request.Validate() != nil || !now.Before(request.Deadline) || request.Deadline.Sub(now) > contract.FeatureLifetime {
		return request, errors.New("invalid terminal intent, epoch, nonce or deadline")
	}
	return request, nil
}

func decodeTerminalCommand(data []byte) (terminalCommand, error) {
	var command terminalCommand
	if err := decodeFeatureInput(data, &command); err != nil {
		return command, err
	}
	if command.Type != "prepared-ack" && command.Reference != nil {
		return command, errors.New("terminal data command cannot select another reference")
	}
	switch command.Type {
	case "prepared-ack":
		if command.Reference == nil || command.Reference.Validate() != nil || command.Sequence != 0 || len(command.Data) != 0 || command.Columns != 0 || command.Rows != 0 {
			return command, errors.New("invalid terminal preparation acknowledgment")
		}
	case "input":
		if command.Sequence == 0 || len(command.Data) == 0 || len(command.Data) > contract.MaxTerminalInput || command.Columns != 0 || command.Rows != 0 {
			return command, errors.New("invalid terminal input command")
		}
	case "resize":
		if !contract.ValidTerminalSize(command.Columns, command.Rows) || command.Sequence != 0 || len(command.Data) != 0 {
			return command, errors.New("invalid terminal resize command")
		}
	case "close":
		if command.Sequence != 0 || len(command.Data) != 0 || command.Columns != 0 || command.Rows != 0 {
			return command, errors.New("invalid terminal close command")
		}
	default:
		return command, errors.New("unsupported terminal command")
	}
	return command, nil
}

func remoteTerminal(ctx context.Context, root, path string, prepareHost bool) error {
	if path == "" || path == "-" {
		return errors.New("remote terminal requires --request <intent-file>; stdin is reserved for private NDJSON")
	}
	data, err := readFeatureInput(ctx, path)
	if err != nil {
		return err
	}
	request, err := decodeTerminalIntent(data, time.Now())
	if err != nil {
		return err
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	return streamTerminalPrepared(ctx, client, request, os.Stdin, os.Stdout, prepareHost)
}

func inspectTerminal(ctx context.Context, root, path string, closeSession bool) error {
	if path == "" || path == "-" {
		return errors.New("terminal receipt actions require --request <reference-file>")
	}
	data, err := readFeatureInput(ctx, path)
	if err != nil {
		return err
	}
	var reference contract.TerminalReference
	if err := decodeFeatureInput(data, &reference); err != nil {
		return err
	}
	if err := reference.Validate(); err != nil {
		return err
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var view contract.TerminalView
	if closeSession {
		view, err = closeAndReleaseTerminal(ctx, client, reference)
	} else {
		view, err = client.ReadTerminal(ctx, reference, 0)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(view)
}

func streamTerminal(parent context.Context, client *remoteio.Client, request contract.TerminalOpen, input io.ReadCloser, output io.WriteCloser) error {
	return streamTerminalPrepared(parent, client, request, input, output, false)
}

func streamTerminalPrepared(parent context.Context, client *remoteio.Client, request contract.TerminalOpen, input io.ReadCloser, output io.WriteCloser, prepareHost bool) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer input.Close()
	defer output.Close()
	interruptOutput := context.AfterFunc(ctx, func() { _ = output.Close() })
	defer interruptOutput()
	reference := request.Reference()
	encoder := json.NewEncoder(output)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 96*1024)
	// Containers always require the preparation handshake. Host consumers can
	// opt in while older host transports retain their one-way announcement.
	if request.Container != "" || prepareHost {
		if err := encoder.Encode(terminalEvent{Type: "prepared", Reference: &reference}); err != nil {
			return err
		}
		preparation, stop := context.WithDeadline(ctx, request.Deadline)
		interrupt := context.AfterFunc(preparation, func() { _ = input.Close() })
		scanned := scanner.Scan()
		interrupt()
		stop()
		if !scanned {
			return errors.New("terminal preparation was not acknowledged")
		}
		acknowledgment, err := decodeTerminalCommand(scanner.Bytes())
		if err != nil || acknowledgment.Type != "prepared-ack" || acknowledgment.Reference == nil || *acknowledgment.Reference != reference {
			return errors.New("terminal preparation reference changed")
		}
	}
	// Capture the original identity before an ambiguous submission response.
	if err := encoder.Encode(terminalEvent{Type: "session", Reference: &reference}); err != nil {
		return err
	}
	defer closeTerminalTransport(parent, client, reference)
	openContext, stopOpen := context.WithTimeout(ctx, 10*time.Second)
	view, err := client.OpenTerminal(openContext, request)
	stopOpen()
	if err != nil {
		_ = encoder.Encode(terminalEvent{Type: "error", Reference: &reference, Message: "Terminal open outcome is unknown; retain this reference. No second shell was launched."})
		return err
	}
	// Background polls never launch an authentication prompt or count as OS-user
	// activity. Lock/sleep/idle invalidation closes the existing shell.
	ctx = context.WithValue(ctx, remoteAuthenticationKey{}, false)
	commands := make(chan terminalCommand)
	inputError := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			command, err := decodeTerminalCommand(scanner.Bytes())
			if err != nil || command.Type == "prepared-ack" {
				if err == nil {
					err = errors.New("duplicate terminal preparation acknowledgment")
				}
				inputError <- err
				return
			}
			select {
			case commands <- command:
			case <-ctx.Done():
				clear(command.Data)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			inputError <- errors.New("terminal input framing failed")
			return
		}
		inputError <- io.EOF
	}()
	poll := time.NewTimer(200 * time.Millisecond)
	defer poll.Stop()
	var interactiveUntil time.Time
	resetPoll := func() {
		interval := 200 * time.Millisecond
		if time.Now().Before(interactiveUntil) {
			interval = 20 * time.Millisecond
		}
		if !poll.Stop() {
			select {
			case <-poll.C:
			default:
			}
		}
		poll.Reset(interval)
	}
	var cursor, acknowledged, pending uint64
	lastState := ""
	pollImmediately := false
	viewHasOutput := false
	for {
		if view.AckSequence > acknowledged && view.AckSequence != pending {
			return errors.New("terminal input acknowledgement changed")
		}
		if viewHasOutput && (len(view.Data) > 0 || view.Truncated) {
			if err := encoder.Encode(terminalEvent{Type: "output", View: &view}); err != nil {
				return err
			}
			cursor = view.Cursor
			lastState = view.State
			view.Data = nil
			view.Truncated = false
		}
		viewHasOutput = false
		if view.State != lastState {
			stateView := view
			stateView.Data = nil
			stateView.Cursor = cursor
			stateView.Oldest = min(stateView.Oldest, cursor)
			stateView.Truncated = false
			if err := encoder.Encode(terminalEvent{Type: "output", View: &stateView}); err != nil {
				return err
			}
			lastState = view.State
		}
		if view.AckSequence > acknowledged {
			if view.AckSequence != pending {
				return errors.New("terminal input acknowledgement changed")
			}
			acknowledged, pending = view.AckSequence, 0
			if err := encoder.Encode(terminalEvent{Type: "ack", Sequence: acknowledged}); err != nil {
				return err
			}
		}
		if (view.State == "closed" || view.State == "unknown") && cursor >= view.End {
			closedView := view
			closedView.Data = nil
			closedView.Cursor = cursor
			closedView.Oldest = min(closedView.Oldest, cursor)
			closedView.Truncated = false
			return encoder.Encode(terminalEvent{Type: "closed", View: &closedView})
		}
		if pollImmediately {
			pollImmediately = false
			callContext, stop := context.WithTimeout(ctx, 2*time.Second)
			view, err = client.PollTerminal(callContext, reference, cursor)
			viewHasOutput = true
			stop()
			if err != nil {
				return err
			}
			resetPoll()
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-inputError:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case command := <-commands:
			callContext, stop := context.WithTimeout(ctx, 2*time.Second)
			switch command.Type {
			case "close":
				stop()
				return nil
			case "resize":
				view, err = client.ResizeTerminal(callContext, contract.TerminalResize{Reference: reference, Columns: command.Columns, Rows: command.Rows})
			case "input":
				if pending != 0 || command.Sequence != acknowledged+1 {
					clear(command.Data)
					stop()
					return errors.New("wait for terminal acknowledgement before the next input")
				}
				pending = command.Sequence
				view, err = client.InputTerminal(callContext, contract.TerminalInput{Reference: reference, Sequence: command.Sequence, Data: command.Data})
				clear(command.Data)
				interactiveUntil = time.Now().Add(250 * time.Millisecond)
				pollImmediately = true
			}
			stop()
			if err != nil {
				return err
			} // no implicit input replay on ambiguous errors
		case <-poll.C:
			callContext, stop := context.WithTimeout(ctx, 2*time.Second)
			view, err = client.PollTerminal(callContext, reference, cursor)
			viewHasOutput = true
			stop()
			if err != nil {
				return err
			}
			resetPoll()
		}
	}
}

func closeTerminalTransport(parent context.Context, client *remoteio.Client, reference contract.TerminalReference) {
	cleanup := context.WithValue(context.WithoutCancel(parent), remoteAuthenticationKey{}, false)
	ctx, cancel := context.WithTimeout(cleanup, 2*time.Second)
	defer cancel()
	_, _ = closeAndReleaseTerminal(ctx, client, reference)
}

func closeAndReleaseTerminal(ctx context.Context, client *remoteio.Client, reference contract.TerminalReference) (contract.TerminalView, error) {
	view, err := client.CloseTerminal(ctx, reference)
	for err == nil && view.State != "closed" && view.State != "unknown" {
		select {
		case <-ctx.Done():
			return view, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
		view, err = client.ReadTerminal(ctx, reference, 0)
	}
	if err == nil && view.State == "closed" {
		_, err = client.ReleaseTerminal(ctx, reference)
	}
	view.Data = nil
	return view, err
}

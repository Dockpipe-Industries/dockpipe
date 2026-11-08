package remote

import (
	"context"
	"errors"

	contract "dockpipe/src/lib/domain/remote"
)

func (client *Client) OpenTerminal(ctx context.Context, request contract.TerminalOpen) (contract.TerminalView, error) {
	if err := request.Validate(); err != nil {
		return contract.TerminalView{}, err
	}
	return client.terminalCall(ctx, "/v1/terminal-open", request.Reference(), request)
}

func (client *Client) ReadTerminal(ctx context.Context, reference contract.TerminalReference, cursor uint64) (contract.TerminalView, error) {
	return client.terminalCall(ctx, "/v1/terminal-read", reference, contract.TerminalRead{Reference: reference, Cursor: cursor})
}

func (client *Client) PulseTerminal(ctx context.Context, reference contract.TerminalReference) (contract.TerminalView, error) {
	return client.PollTerminal(ctx, reference, 0)
}

// PollTerminal combines execute-scoped renewal and output retrieval into one
// signed round trip. ReadTerminal remains available without renewal authority.
func (client *Client) PollTerminal(ctx context.Context, reference contract.TerminalReference, cursor uint64) (contract.TerminalView, error) {
	return client.terminalCall(ctx, "/v1/terminal-pulse", reference, contract.TerminalRead{Reference: reference, Cursor: cursor})
}

func (client *Client) InputTerminal(ctx context.Context, input contract.TerminalInput) (contract.TerminalView, error) {
	if err := input.Validate(); err != nil {
		return contract.TerminalView{}, err
	}
	return client.terminalCall(ctx, "/v1/terminal-input", input.Reference, input)
}

func (client *Client) ResizeTerminal(ctx context.Context, request contract.TerminalResize) (contract.TerminalView, error) {
	if !contract.ValidTerminalSize(request.Columns, request.Rows) {
		return contract.TerminalView{}, errors.New("invalid terminal size")
	}
	return client.terminalCall(ctx, "/v1/terminal-resize", request.Reference, request)
}

func (client *Client) CloseTerminal(ctx context.Context, reference contract.TerminalReference) (contract.TerminalView, error) {
	return client.terminalCall(ctx, "/v1/terminal-close", reference, reference)
}

func (client *Client) ReleaseTerminal(ctx context.Context, reference contract.TerminalReference) (contract.TerminalView, error) {
	return client.terminalCall(ctx, "/v1/terminal-release", reference, reference)
}

func (client *Client) terminalCall(ctx context.Context, path string, reference contract.TerminalReference, input any) (contract.TerminalView, error) {
	var view contract.TerminalView
	if err := reference.Validate(); err != nil {
		return view, err
	}
	err := client.Call(ctx, path, input, &view)
	if err == nil && (view.Reference != reference || len(view.Data) > contract.MaxTerminalChunk || view.Cursor < view.Oldest && len(view.Data) > 0) {
		err = errors.New("terminal response identity or bounds changed")
	}
	if err == nil && (view.Cursor > view.End || view.Oldest > view.End || uint64(len(view.Data)) > view.Cursor) {
		err = errors.New("terminal response cursor is invalid")
	}
	if err == nil {
		switch view.State {
		case "queued", "opening", "running", "closed", "unknown":
		default:
			err = errors.New("terminal response state is invalid")
		}
	}
	return view, err
}

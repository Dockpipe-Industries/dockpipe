package nativehost

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

type Client struct{ Connection Connection }

func NewClient(path string) (*Client, error) {
	connection, err := ReadConnection(path)
	if err != nil {
		return nil, err
	}
	return &Client{Connection: connection}, nil
}

func (c *Client) connect(ctx context.Context, request frame) (*wire, error) {
	key, info, err := readKey(c.Connection.KeyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", c.Connection.Socket)
	if err != nil {
		return nil, ErrUnavailable
	}
	credential, err := peer(connection.(*net.UnixConn))
	owner, ok := info.Sys().(*syscall.Stat_t)
	if err != nil || !ok || credential.Uid != owner.Uid {
		connection.Close()
		return nil, ErrUnavailable
	}
	if deadline, ok := ctx.Deadline(); ok {
		connection.SetDeadline(deadline)
	}
	transport := &wire{ReadWriteCloser: connection}
	transport.writeLimit, _ = ctx.Deadline()
	stop := context.AfterFunc(ctx, func() { connection.Close() })
	defer stop()
	request.Key = key
	if transport.send(request) != nil {
		transport.Close()
		return nil, ErrUnavailable
	}
	return transport, nil
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	transport, err := c.connect(ctx, frame{Kind: "status"})
	if err != nil {
		return unavailableStatus(), err
	}
	defer transport.Close()
	stop := context.AfterFunc(ctx, func() { transport.Close() })
	defer stop()
	response, err := transport.receive()
	if err != nil || response.Kind != "status" || response.Status == nil || !response.Status.Available || !response.Status.Connected || !validWorkerStatus(*response.Status, response.Status.UID, response.Status.GID) || response.Status.UID == 0 || response.Status.Schema != StatusSchema {
		return unavailableStatus(), ErrUnavailable
	}
	return *response.Status, nil
}

func (c *Client) Snapshot(ctx context.Context, system, docker bool) (telemetry.MachineSnapshot, error) {
	var snapshot telemetry.MachineSnapshot
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	transport, err := c.connect(ctx, frame{Kind: "snapshot", System: system, Docker: docker})
	if err != nil {
		return snapshot, err
	}
	defer transport.Close()
	stop := context.AfterFunc(ctx, func() { transport.Close() })
	defer stop()
	response, err := transport.receive()
	if err != nil || response.Kind != "snapshot" || len(response.Snapshot) > telemetry.MaxMachineSnapshotBytes || json.Unmarshal(response.Snapshot, &snapshot) != nil || snapshot.Schema != telemetry.MachineSchema {
		return telemetry.MachineSnapshot{}, ErrUnavailable
	}
	return snapshot, nil
}

type Terminal struct {
	transport *wire
	closeOnce sync.Once
	done      chan struct{}
}

func (c *Client) OpenTerminal(ctx context.Context, options TerminalOptions) (*Terminal, error) {
	startup, cancel := boundedContext(ctx)
	defer cancel()
	transport, err := c.connect(startup, frame{Kind: "terminal", CWD: options.CWD, Rows: options.Rows, Columns: options.Columns})
	if err != nil {
		return nil, err
	}
	connection := transport.ReadWriteCloser.(*net.UnixConn)
	stop := context.AfterFunc(startup, func() { connection.Close() })
	defer stop()
	response, err := transport.receive()
	if err != nil || response.Kind != "opened" {
		transport.Close()
		return nil, ErrUnavailable
	}
	connection.SetDeadline(time.Time{})
	transport.writeLimit = time.Time{}
	terminal := &Terminal{transport: transport, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			terminal.Close()
		case <-terminal.done:
		}
	}()
	return terminal, nil
}

func (t *Terminal) Write(data []byte) (int, error) {
	written := 0
	for len(data) > 0 {
		size := min(len(data), maxTerminalData)
		if err := t.transport.send(frame{Kind: "input", Data: data[:size]}); err != nil {
			return written, err
		}
		written += size
		data = data[size:]
	}
	return written, nil
}

func (t *Terminal) Resize(rows, columns uint16) error {
	return t.transport.send(frame{Kind: "resize", Rows: rows, Columns: columns})
}

func (t *Terminal) CopyOutput(output io.Writer) (int, error) {
	for {
		response, err := t.transport.receive()
		if err != nil {
			return 1, ErrUnavailable
		}
		switch response.Kind {
		case "output":
			if err := writeAll(output, response.Data); err != nil {
				return 1, err
			}
		case "exit":
			return response.Exit, nil
		default:
			return 1, ErrUnavailable
		}
	}
}

func (t *Terminal) Close() error {
	t.closeOnce.Do(func() {
		close(t.done)
		t.transport.Close()
	})
	return nil
}

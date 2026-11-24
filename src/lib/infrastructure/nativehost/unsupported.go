//go:build !linux

package nativehost

import (
	"context"
	"io"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

type Broker struct{}
type Client struct{}
type Terminal struct{}

func NewBroker(Options) (*Broker, error) {
	return nil, ErrUnavailable
}

func (*Broker) Serve(context.Context) error {
	return ErrUnavailable
}

func (*Broker) ExpectWorker(uint32, uint32, string, time.Time) error {
	return ErrUnavailable
}

func (*Broker) Status() Status {
	return unavailableStatus()
}

func (*Broker) Revoke() error {
	return nil
}

func (*Broker) Close() error {
	return nil
}

func NewClient(string) (*Client, error) {
	return nil, ErrUnavailable
}

func (*Client) Status(context.Context) (Status, error) {
	return unavailableStatus(), ErrUnavailable
}

func (*Client) Snapshot(context.Context, bool, bool) (telemetry.MachineSnapshot, error) {
	return telemetry.MachineSnapshot{}, ErrUnavailable
}

func (*Client) OpenTerminal(context.Context, TerminalOptions) (*Terminal, error) {
	return nil, ErrUnavailable
}

func (*Client) Docker(_ context.Context, action, id string) (DockerResult, error) {
	return failedDocker(action, id, "native_host_unavailable"), ErrUnavailable
}

func (*Terminal) Write([]byte) (int, error) {
	return 0, ErrUnavailable
}

func (*Terminal) Resize(uint16, uint16) error {
	return ErrUnavailable
}

func (*Terminal) CopyOutput(io.Writer) (int, error) {
	return 1, ErrUnavailable
}

func (*Terminal) Close() error {
	return nil
}

func RunWorker(context.Context, WorkerOptions) error {
	return ErrUnavailable
}

func DetachWorker(context.Context, WorkerOptions) error {
	return ErrUnavailable
}

func (*Client) DockerQuery(_ context.Context, kind, id string) (DockerQueryResult, error) {
	return failedDockerQuery(kind, id, "native_host_unavailable"), ErrUnavailable
}

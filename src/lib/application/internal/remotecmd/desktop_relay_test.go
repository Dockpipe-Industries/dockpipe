package remotecmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type desktopRelayPipeFixture struct {
	incoming chan []byte
	outgoing chan []byte
	closed   chan struct{}
	once     sync.Once
}

func newDesktopRelayPipeFixture() *desktopRelayPipeFixture {
	return &desktopRelayPipeFixture{incoming: make(chan []byte, 1), outgoing: make(chan []byte, 1), closed: make(chan struct{})}
}

func (relay *desktopRelayPipeFixture) Read(ctx context.Context) ([]byte, error) {
	select {
	case payload := <-relay.incoming:
		return payload, nil
	case <-relay.closed:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (relay *desktopRelayPipeFixture) Write(ctx context.Context, payload []byte) error {
	select {
	case relay.outgoing <- append([]byte(nil), payload...):
		return nil
	case <-relay.closed:
		return io.EOF
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (relay *desktopRelayPipeFixture) Close() error {
	relay.once.Do(func() { close(relay.closed) })
	return nil
}

func TestDesktopRelayPrivateFramingAndCancellation(t *testing.T) {
	relay := newDesktopRelayPipeFixture()
	providerInput, bridgeInput := io.Pipe()
	bridgeOutput, providerOutput := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer providerInput.Close()
	defer providerOutput.Close()
	finished := make(chan error, 1)
	go func() { finished <- streamDesktopRelay(ctx, relay, providerInput, providerOutput) }()
	payload := []byte{0x00, 0xff, 0x15, 0x00, 0xa7}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := bridgeInput.Write(append(header[:], payload...)); err != nil {
		t.Fatal(err)
	}
	select {
	case received := <-relay.outgoing:
		if !bytes.Equal(received, payload) {
			t.Fatal("private framing changed opaque bytes")
		}
	case <-time.After(time.Second):
		t.Fatal("framed input was not delivered")
	}
	relay.incoming <- payload
	if _, err := io.ReadFull(bridgeOutput, header[:]); err != nil || binary.BigEndian.Uint32(header[:]) != uint32(len(payload)) {
		t.Fatal("binary output emitted a preamble or wrong length", err)
	}
	received := make([]byte, len(payload))
	if _, err := io.ReadFull(bridgeOutput, received); err != nil || !bytes.Equal(received, payload) {
		t.Fatal("private output changed opaque bytes", err)
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("cancel did not close/join private relay pumps")
	}
	select {
	case <-relay.closed:
	default:
		t.Fatal("private cancellation left underlying relay open")
	}
}

func TestDesktopRelayRejectsOversizeOrTruncatedPrivateFrames(t *testing.T) {
	for _, length := range []uint32{0, contract.MaxDesktopRelayFrame + 1, 12} {
		relay := newDesktopRelayPipeFixture()
		var input bytes.Buffer
		_ = binary.Write(&input, binary.BigEndian, length)
		var output desktopBuffer
		if err := streamDesktopRelay(context.Background(), relay, io.NopCloser(&input), &output); err == nil {
			t.Fatal("invalid binary private frame succeeded")
		}
		select {
		case <-relay.outgoing:
			t.Fatal("invalid private frame reached network transport")
		default:
		}
		if output.Len() != 0 {
			t.Fatal("binary bridge emitted JSON/diagnostics")
		}
	}
}

func TestDesktopRelayOwnerPolicyChangeStopsPrivateTransport(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	frozen := remoteio.DesktopPermissions{Schema: contract.DesktopVersion, Provider: desktopTestIntent().Provider, View: true, Input: true}
	if err := remoteio.SaveDesktopPermissions(root, frozen); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := desktopRelayPolicyContext(context.Background(), root, frozen)
	defer cancel()
	input, writer := io.Pipe()
	defer writer.Close()
	relay := newDesktopRelayPipeFixture()
	var output desktopBuffer
	finished := make(chan error, 1)
	go func() { finished <- streamDesktopRelay(ctx, relay, input, &output) }()
	changed := frozen
	changed.Input = false
	if err := remoteio.SaveDesktopPermissions(root, changed); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("worker policy revocation became successful transport closure")
		}
	case <-time.After(time.Second):
		t.Fatal("changed owner grant did not interrupt private relay")
	}
	select {
	case <-relay.closed:
	default:
		t.Fatal("worker policy change left the underlying relay open")
	}
}

package remotecmd

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

// The relay bridge is a dedicated opaque private pipe. It emits no JSON,
// preamble, content diagnostics or implicit payload retries. E2E encryption and
// exact peer/policy verification belong to the external native endpoint.
func desktopRelayBridge(parent context.Context, root, path string) error {
	if path == "" || path == "-" {
		return errors.New("desktop-relay requires --request <private-file>; stdin is opaque framed transport")
	}
	data, err := readFeatureInput(parent, path)
	if err != nil {
		return err
	}
	var request contract.DesktopRelayRequest
	if decodeFeatureInput(data, &request) != nil || request.Validate() != nil {
		return errors.New("invalid original desktop relay request")
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var client *remoteio.Client
	var permissions remoteio.DesktopPermissions
	var sessionFence *remoteio.DesktopSessionFence
	if request.Role == "worker" {
		permissions, err = remoteio.LoadDesktopPermissions(root)
		if err != nil {
			return err
		}
		platform, platformErr := osauth.New()
		if platformErr != nil {
			return platformErr
		}
		sessionFence = remoteio.NewDesktopSessionFence(platform)
		if err := sessionFence.Check(ctx); err != nil {
			return err
		}
		var config contract.WorkerConfig
		if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &config); err != nil {
			return err
		}
		if config.Node != request.Reference.Target.Node {
			return errors.New("desktop relay worker changed its original paired node")
		}
		client, err = remoteio.NewWorkerClient(config)
		if err == nil && client.Identity == nil {
			err = errors.New("desktop relay requires paired worker machine keys")
		}
	} else {
		client, err = operatorClient(root)
	}
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	ticket, err := client.DesktopRelayTicket(ctx, request)
	if err != nil {
		return err
	}
	if request.Role == "worker" {
		if permissions.Provider != ticket.Provider || !permissions.View || ticket.Mode == "control" && !permissions.Input || remoteio.CheckDesktopPolicy(root, permissions) != nil {
			return errors.New("desktop relay exceeds current independent worker owner permissions")
		}
	}
	relay, err := client.AttachDesktopRelay(ctx, ticket)
	if err != nil {
		return err
	}
	defer relay.Close()
	if request.Role == "worker" {
		ctx, cancel = desktopRelayPolicyContext(ctx, root, permissions, sessionFence.Check)
		defer cancel()
	}
	return streamDesktopRelay(ctx, relay, os.Stdin, os.Stdout)
}

type desktopBinaryRelay interface {
	Read(context.Context) ([]byte, error)
	Write(context.Context, []byte) error
	Close() error
}

func streamDesktopRelay(parent context.Context, relay desktopBinaryRelay, input io.ReadCloser, output io.WriteCloser) error {
	ctx, cancel := context.WithCancel(parent)
	var pumps sync.WaitGroup
	interrupt := context.AfterFunc(ctx, func() { _ = input.Close(); _ = output.Close(); _ = relay.Close() })
	defer func() {
		cancel()
		_ = input.Close()
		_ = output.Close()
		_ = relay.Close()
		interrupt()
		pumps.Wait()
	}()
	finished := make(chan error, 2)
	pumps.Add(1)
	go func() {
		defer pumps.Done()
		var header [4]byte
		for {
			deadline := time.AfterFunc(30*time.Second, cancel)
			if _, err := io.ReadFull(input, header[:]); err != nil {
				deadline.Stop()
				finished <- errors.New("desktop relay private input closed")
				return
			}
			length := binary.BigEndian.Uint32(header[:])
			if length == 0 || length > contract.MaxDesktopRelayFrame {
				deadline.Stop()
				finished <- errors.New("desktop relay private frame exceeds its bound")
				return
			}
			payload := make([]byte, length)
			if _, err := io.ReadFull(input, payload); err != nil {
				deadline.Stop()
				finished <- errors.New("desktop relay private frame was interrupted")
				return
			}
			deadline.Stop()
			writeContext, stop := context.WithTimeout(ctx, 5*time.Second)
			err := relay.Write(writeContext, payload)
			stop()
			if err != nil {
				finished <- errors.New("desktop relay connection closed while writing")
				return
			}
		}
	}()
	pumps.Add(1)
	go func() {
		defer pumps.Done()
		var header [4]byte
		for {
			readContext, stop := context.WithTimeout(ctx, 30*time.Second)
			payload, err := relay.Read(readContext)
			stop()
			if err != nil {
				finished <- errors.New("desktop relay connection closed while reading")
				return
			}
			payloadLength := len(payload)
			if payloadLength < 1 || payloadLength > contract.MaxDesktopRelayFrame {
				finished <- errors.New("desktop relay private frame exceeds its bound")
				return
			}

			binary.BigEndian.PutUint32(header[:], uint32(payloadLength))
			deadline := time.AfterFunc(5*time.Second, cancel)
			err = writeDesktopRelayBytes(output, header[:])
			if err == nil {
				err = writeDesktopRelayBytes(output, payload)
			}
			deadline.Stop()
			if err != nil {
				finished <- errors.New("desktop relay private output closed")
				return
			}
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-finished:
		return err
	}
}

func writeDesktopRelayBytes(output io.Writer, data []byte) error {
	for len(data) != 0 {
		written, err := output.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 || written > len(data) {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func desktopRelayPolicyContext(parent context.Context, root string, frozen remoteio.DesktopPermissions, sessionChecks ...func(context.Context) error) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		checkSession := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkSession = !checkSession
				if checkSession {
					for _, check := range sessionChecks {
						if check(ctx) != nil {
							cancel()
							return
						}
					}
				}
				if remoteio.CheckDesktopPolicy(root, frozen) != nil {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}

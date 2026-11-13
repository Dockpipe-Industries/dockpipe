package remotecmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/osauth"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

// Desktop bridges carry private control metadata only. The external native
// provider owns capture, media, input, artifact verification and key release.
type desktopFrame struct {
	Type      string                     `json:"type"`
	Reference *contract.DesktopReference `json:"reference,omitempty"`
	Provider  *contract.DesktopProvider  `json:"provider,omitempty"`
	View      bool                       `json:"view,omitempty"`
	Input     bool                       `json:"input,omitempty"`
	Signal    *contract.DesktopSignal    `json:"signal,omitempty"`
	Report    *contract.DesktopReport    `json:"report,omitempty"`
}

type desktopEvent struct {
	Type      string                     `json:"type"`
	Catalog   *contract.DesktopCatalog   `json:"catalog,omitempty"`
	View      *contract.DesktopView      `json:"view,omitempty"`
	Directive *contract.DesktopDirective `json:"directive,omitempty"`
	Reference *contract.DesktopReference `json:"reference,omitempty"`
}

func desktopScanner(input io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(input)
	// JSON escaping can expand a 64 KiB description sixfold. Aggregate signaling
	// is bounded independently by the broker's 512 KiB session contract.
	scanner.Buffer(make([]byte, 4096), 1<<20)
	return scanner
}

func decodeDesktopFrame(data []byte) (desktopFrame, error) {
	var frame desktopFrame
	if err := decodeFeatureInput(data, &frame); err != nil {
		return frame, err
	}
	switch frame.Type {
	case "ready":
		if frame.Provider == nil || frame.Provider.Validate() != nil || frame.Reference != nil || frame.Signal != nil || frame.Report != nil || frame.Input && !frame.View {
			return frame, errors.New("invalid desktop provider readiness")
		}
	case "prepared-ack":
		if frame.Reference == nil || frame.Reference.Validate() != nil || frame.Provider != nil || frame.Signal != nil || frame.Report != nil || frame.View || frame.Input {
			return frame, errors.New("invalid desktop prepared acknowledgment")
		}
	case "signal":
		if frame.Signal == nil || frame.Signal.Validate() != nil || frame.Reference != nil || frame.Provider != nil || frame.Report != nil || frame.View || frame.Input {
			return frame, errors.New("invalid desktop provider signaling")
		}
	case "report":
		if frame.Report == nil || frame.Reference != nil || frame.Provider != nil || frame.Signal != nil || frame.View || frame.Input {
			return frame, errors.New("invalid desktop provider report")
		}
	case "close":
		if frame.Reference != nil || frame.Provider != nil || frame.Signal != nil || frame.Report != nil || frame.View || frame.Input {
			return frame, errors.New("invalid desktop close command")
		}
	default:
		return frame, errors.New("unsupported desktop control frame")
	}
	return frame, nil
}

func readDesktopFrames(ctx context.Context, scanner *bufio.Scanner) (<-chan desktopFrame, <-chan error) {
	frames := make(chan desktopFrame)
	failures := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			frame, err := decodeDesktopFrame(scanner.Bytes())
			if err != nil {
				failures <- err
				return
			}
			select {
			case frames <- frame:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			failures <- err
		} else {
			failures <- io.EOF
		}
	}()
	return frames, failures
}

func desktopViewer(ctx context.Context, root, inputPath string) error {
	if inputPath == "" || inputPath == "-" {
		return errors.New("desktop --stdio requires --request <intent-file>; stdin is private control transport")
	}
	data, err := readFeatureInput(ctx, inputPath)
	if err != nil {
		return err
	}
	request, err := decodeDesktopOpen(data, time.Now())
	if err != nil {
		return err
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	return streamDesktopViewer(ctx, client, request, os.Stdin, os.Stdout)
}

func streamDesktopViewer(parent context.Context, client *remoteio.Client, request contract.DesktopOpen, input io.ReadCloser, output io.WriteCloser) error {
	ctx, cancel := context.WithDeadline(parent, request.Deadline)
	defer cancel()
	defer input.Close()
	defer output.Close()
	interruptInput := context.AfterFunc(ctx, func() { _ = input.Close() })
	defer interruptInput()
	interruptOutput := context.AfterFunc(ctx, func() { _ = output.Close() })
	defer interruptOutput()
	encoder := json.NewEncoder(output)
	if err := encoder.Encode(preparedDesktop(request)); err != nil {
		return err
	}
	scanner := desktopScanner(input)
	preparation := time.AfterFunc(featureCommandTimeout, func() { _ = input.Close() })
	defer preparation.Stop()
	if !scanner.Scan() {
		return errors.New("desktop preparation was not acknowledged")
	}
	preparation.Stop()
	frame, err := decodeDesktopFrame(scanner.Bytes())
	if err != nil || frame.Type != "prepared-ack" || *frame.Reference != request.Reference() {
		return errors.New("desktop preparation reference changed")
	}
	view, err := client.OpenDesktop(ctx, request)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer stop()
		cleanup = context.WithValue(cleanup, remoteAuthenticationKey{}, false)
		_, _ = client.CloseDesktop(cleanup, request.Reference())
	}()
	frames, failures := readDesktopFrames(ctx, scanner)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var after uint64
	for {
		for _, signal := range view.Signals {
			if signal.Sequence != after+1 {
				return errors.New("desktop incoming signaling sequence changed")
			}
			after = signal.Sequence
		}
		if err := encoder.Encode(desktopEvent{Type: "view", View: &view}); err != nil {
			return err
		}
		if view.State == "closed" {
			return nil
		}
		if view.State == "unknown" || view.LeaseMillis <= 0 && view.State != "closing" {
			return errors.New("desktop native cleanup or authority is unconfirmed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			return err
		case frame := <-frames:
			switch frame.Type {
			case "signal":
				if frame.Signal.Reference != request.Reference() {
					return errors.New("desktop signal changed its prepared session")
				}
				view, err = client.SignalDesktop(ctx, *frame.Signal)
			case "close":
				view, err = client.CloseDesktop(ctx, request.Reference())
			default:
				return errors.New("unexpected viewer control frame")
			}
		case <-ticker.C:
			view, err = client.PulseDesktop(ctx, contract.DesktopRead{Reference: request.Reference(), After: after})
		}
		if err != nil {
			return err
		}
	}
}

func desktopWorkerBridge(parent context.Context, root string) error {
	return streamDesktopWorker(parent, root, os.Stdin, os.Stdout)
}

func streamDesktopWorker(parent context.Context, root string, input io.ReadCloser, output io.WriteCloser) error {
	if err := requireDesktopHosting(); err != nil {
		return err
	}
	platform, err := osauth.New()
	if err != nil {
		return err
	}
	return streamDesktopWorkerWithSession(parent, root, input, output, remoteio.NewDesktopSessionFence(platform))
}

func streamDesktopWorkerWithSession(parent context.Context, root string, input io.ReadCloser, output io.WriteCloser, fence *remoteio.DesktopSessionFence) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer input.Close()
	defer output.Close()
	interrupt := context.AfterFunc(ctx, func() { _ = input.Close(); _ = output.Close() })
	defer interrupt()
	unlock, err := remoteio.Lock(filepath.Join(root, "desktop.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	permissions, err := remoteio.LoadDesktopPermissions(root)
	if err != nil {
		return err
	}
	var config contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &config); err != nil {
		return err
	}
	client, err := remoteio.NewWorkerClient(config)
	if err != nil || client.Identity == nil {
		return errors.New("desktop bridge requires paired worker machine keys")
	}
	defer client.HTTP.CloseIdleConnections()
	scanner := desktopScanner(input)
	preparation := time.AfterFunc(featureCommandTimeout, func() { _ = input.Close() })
	defer preparation.Stop()
	if !scanner.Scan() {
		return errors.New("native desktop provider did not publish readiness")
	}
	preparation.Stop()
	ready, err := decodeDesktopFrame(scanner.Bytes())
	if err != nil || ready.Type != "ready" || *ready.Provider != permissions.Provider {
		return errors.New("native desktop provider differs from the frozen owner identity")
	}
	if err := fence.Check(ctx); err != nil {
		return err
	}
	instance, err := remoteio.Secret()
	if err != nil {
		return err
	}
	catalog, err := remoteio.NewDesktopCatalog(config, permissions, instance, ready.View, ready.Input)
	if err != nil {
		return err
	}
	return exchangeDesktopWorker(ctx, root, permissions, client, catalog, scanner, input, output, fence)
}

// Identity and owner readiness are admitted by streamDesktopWorkerWithSession.
// Keep the exchange lifecycle independently testable without native OS effects.
func exchangeDesktopWorker(ctx context.Context, root string, permissions remoteio.DesktopPermissions,
	client desktopWorkerExchangeClient, catalog contract.DesktopCatalog, scanner *bufio.Scanner,
	input io.ReadCloser, output io.WriteCloser, fence *remoteio.DesktopSessionFence) error {
	encoder := json.NewEncoder(output)
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Catalog: catalog}
	response, err := client.ExchangeDesktop(ctx, exchange)
	if err != nil {
		return err
	}
	exchange.Epoch = response.Epoch
	catalog.Epoch = response.Epoch
	if err := encoder.Encode(desktopEvent{Type: "catalog", Catalog: &catalog}); err != nil {
		return err
	}
	frames, failures := readDesktopFrames(ctx, scanner)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var active *contract.DesktopDirective

	for {
		if err := fence.Check(ctx); err != nil {
			drainDesktopWorker(ctx, client, exchange, active, encoder, frames, failures, input, output)
			return err
		}
		if err := remoteio.CheckDesktopPolicy(root, permissions); err != nil {
			drainDesktopWorker(ctx, client, exchange, active, encoder, frames, failures, input, output)
			return err
		}
		started := time.Now()
		response, err = client.ExchangeDesktop(ctx, exchange)
		if err != nil {
			return err
		}
		if err := fence.Check(ctx); err != nil {
			if len(response.Directives) == 1 {
				active = &response.Directives[0]
			}
			drainDesktopWorker(ctx, client, exchange, active, encoder, frames, failures, input, output)
			return err
		}
		for _, reference := range response.Forget {
			if err := encoder.Encode(desktopEvent{Type: "forget", Reference: &reference}); err != nil {
				return err
			}
			exchange.Reports = nil
			active = nil
		}
		for _, directive := range response.Directives {
			// Count challenge and network transit against the remaining lease.
			// The native endpoint starts its own monotonic timer on receipt.
			directive.LeaseMillis = max(0, directive.LeaseMillis-time.Since(started).Milliseconds())
			if directive.LeaseMillis == 0 {
				directive.Close = true
			}
			active = &directive
			if directive.Close {
				// The native pipe can contain old ordinary reports. Once revoked,
				// consume them without another lease exchange per queued frame.
				return drainDesktopWorker(ctx, client, exchange, active, encoder, frames, failures, input, output)
			}
			if err := encoder.Encode(desktopEvent{Type: "directive", Directive: &directive}); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			return err
		case frame := <-frames:
			if frame.Type != "report" || active == nil || frame.Report.Reference != active.Open.Reference() {
				return errors.New("native desktop report changed the active session")
			}
			exchange.Reports = []contract.DesktopReport{*frame.Report}
		case <-ticker.C:
		}
	}
}

package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type desktopWorkerExchangeClient interface {
	ExchangeDesktop(context.Context, contract.DesktopExchange) (contract.DesktopExchangeResult, error)
}

// A local fence revokes publication before asking native composition to stop.
// This bounded drain accepts only original-session cleanup proof, never another
// opening, signaling message or lease renewal. Capture/key cleanup can take 5s,
// followed by bounded TLS child termination/reaping; the whole drain is 10s.
func drainDesktopWorker(parent context.Context, client desktopWorkerExchangeClient, exchange contract.DesktopExchange, fallback *contract.DesktopDirective,
	encoder *json.Encoder, frames <-chan desktopFrame, failures <-chan error, input io.ReadCloser, output io.WriteCloser) error {
	ctx, stop := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
	defer stop()
	interrupt := context.AfterFunc(ctx, func() { _ = input.Close(); _ = output.Close() })
	defer interrupt()
	exchange.Catalog.View, exchange.Catalog.Input = false, false
	queuedCleanup := false
	if len(exchange.Reports) == 1 && fallback != nil {
		report := exchange.Reports[0]
		queuedCleanup = report.Reference == fallback.Open.Reference() && len(report.Signals) == 0 && (report.State == "closed" || report.State == "unknown")
	}
	if !queuedCleanup {
		exchange.Reports = nil
	}
	publication, cancel := context.WithTimeout(ctx, 2*time.Second)
	response, err := client.ExchangeDesktop(publication, exchange)
	cancel()
	if err == nil && queuedCleanup {
		return emitDesktopCleanupForget(encoder, response, exchange.Reports[0])
	}
	var closing *contract.DesktopDirective
	if err == nil && len(response.Directives) == 1 {
		closing = &response.Directives[0]
	} else if fallback != nil {
		copy := *fallback
		closing = &copy
	}
	if closing == nil {
		return errors.New("desktop cleanup did not obtain the original closing directive")
	}
	closing.Close, closing.LeaseMillis, closing.Signals = true, 0, nil
	if err := encoder.Encode(desktopEvent{Type: "directive", Directive: closing}); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			if err == nil {
				return errors.New("desktop cleanup input ended without native proof")
			}
			return err
		case frame, open := <-frames:
			if !open || frame.Type != "report" || frame.Report == nil || frame.Report.Reference != closing.Open.Reference() {
				return errors.New("desktop cleanup report changed the original session")
			}
			// Ordinary reports may already be queued when the fence sends Close.
			// Discard them without publishing signals or renewing authority, then
			// keep observing the original native endpoint's bounded cleanup proof.
			if frame.Report.State == "opening" || frame.Report.State == "running" {
				continue
			}
			if len(frame.Report.Signals) != 0 || frame.Report.State != "closed" && frame.Report.State != "unknown" {
				return errors.New("desktop cleanup requires a content-free native terminal report")
			}
			exchange.Reports = []contract.DesktopReport{*frame.Report}
			response, err = client.ExchangeDesktop(ctx, exchange)
			if err != nil {
				return err
			}
			return emitDesktopCleanupForget(encoder, response, *frame.Report)
		}
	}
}

func emitDesktopCleanupForget(encoder *json.Encoder, response contract.DesktopExchangeResult, report contract.DesktopReport) error {
	if report.State != "closed" || len(response.Forget) != 1 || response.Forget[0] != report.Reference || len(response.Directives) != 0 {
		return errors.New("desktop native cleanup did not receive the exact closed acknowledgment")
	}
	return encoder.Encode(desktopEvent{Type: "forget", Reference: &response.Forget[0]})
}

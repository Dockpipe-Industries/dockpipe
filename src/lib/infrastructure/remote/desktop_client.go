package remote

import (
	"context"
	"errors"
	"net/http"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func (client *Client) DesktopCatalog(ctx context.Context) ([]contract.DesktopCatalog, error) {
	var catalogs []contract.DesktopCatalog
	err := client.Call(ctx, "/v1/desktop-catalog", map[string]string{"schema": contract.DesktopDiscoveryVersion}, &catalogs)
	if desktopDiscoveryUnsupported(err) {
		err = client.Call(ctx, "/v1/desktop-catalog", map[string]string{"schema": contract.DesktopVersion}, &catalogs)
	}
	if err != nil {
		return nil, err
	}
	if len(catalogs) > contract.MaxDesktopSessions {
		return nil, errors.New("desktop discovery exceeds enrolled node capacity")
	}
	for _, catalog := range catalogs {
		if catalog.Validate() != nil || !validMachineNonce(catalog.Epoch) {
			return nil, errors.New("desktop discovery changed its identity or epoch")
		}
	}
	return catalogs, nil
}

func desktopDiscoveryUnsupported(err error) bool {
	// Older brokers reject the discovery schema under their usual signed
	// operation status. Transport failures and unsigned errors stay terminal.
	var rejection *HTTPError
	return errors.As(err, &rejection) && rejection.authenticated && rejection.StatusCode == http.StatusConflict
}

func (client *Client) OpenDesktop(ctx context.Context, request contract.DesktopOpen) (contract.DesktopView, error) {
	if err := request.Validate(); err != nil {
		return contract.DesktopView{}, err
	}
	return client.desktopCall(ctx, "/v1/desktop-open", request.Reference(), request)
}

func (client *Client) DesktopStatus(ctx context.Context, read contract.DesktopRead) (contract.DesktopView, error) {
	view, err := client.desktopCall(ctx, "/v1/desktop-status", read.Reference, read)
	if err == nil && read.MetadataOnly && (len(view.Signals) != 0 || view.LeaseMillis != 0) {
		err = errors.New("desktop metadata status acquired signaling or media authority")
	}
	return view, err
}

func (client *Client) PulseDesktop(ctx context.Context, read contract.DesktopRead) (contract.DesktopView, error) {
	return client.desktopCall(ctx, "/v1/desktop-pulse", read.Reference, read)
}

func (client *Client) CloseDesktop(ctx context.Context, reference contract.DesktopReference) (contract.DesktopView, error) {
	return client.desktopCall(ctx, "/v1/desktop-close", reference, reference)
}

func (client *Client) SignalDesktop(ctx context.Context, signal contract.DesktopSignal) (contract.DesktopView, error) {
	if err := signal.Validate(); err != nil {
		return contract.DesktopView{}, err
	}
	return client.desktopCall(ctx, "/v1/desktop-signal", signal.Reference, signal)
}

func (client *Client) desktopCall(ctx context.Context, path string, reference contract.DesktopReference, input any) (contract.DesktopView, error) {
	var view contract.DesktopView
	if reference.Validate() != nil {
		return view, errors.New("invalid original desktop reference")
	}
	started := time.Now()
	if err := client.Call(ctx, path, input, &view); err != nil {
		return view, err
	}
	if view.Reference != reference || view.LeaseMillis < 0 || view.LeaseMillis > contract.DesktopLease.Milliseconds() || view.Ack > contract.MaxDesktopSignals || len(view.Signals) > contract.MaxDesktopSignals {
		return view, errors.New("desktop response changed identity or bounds")
	}
	switch view.State {
	case "queued", "opening", "running":
	case "closing", "closed", "unknown":
		if view.LeaseMillis != 0 {
			return view, errors.New("desktop closing or uncertain response retained media/input lease")
		}
	default:
		return view, errors.New("invalid desktop lifecycle response")
	}
	for _, signal := range view.Signals {
		if signal.Validate() != nil || signal.Reference != reference || signal.Type == "offer" || signal.Type == "parameters" {
			return view, errors.New("desktop response contains foreign signaling")
		}
	}
	view.LeaseMillis = max(0, view.LeaseMillis-time.Since(started).Milliseconds())
	return view, nil
}

// ExchangeDesktop negotiates discovery only on the read-only epoch probe.
// Publication and session reports run once. Native callers must close
// media/input when this channel or its remaining lease fails.
func (client *Client) ExchangeDesktop(ctx context.Context, exchange contract.DesktopExchange) (contract.DesktopExchangeResult, error) {
	var response contract.DesktopExchangeResult
	if exchange.Catalog.Validate() != nil || exchange.Schema != contract.DesktopVersion {
		return response, errors.New("invalid desktop worker capability publication")
	}

	wire := exchange
	wire.Catalog.AdministratorInput = false
	probe := exchange.Epoch == "" && len(exchange.Reports) == 0
	negotiate := probe && exchange.Catalog.View && client.Identity != nil
	discoveryEpoch := client.desktopDiscoveryEpoch.Load()
	switch {
	case negotiate:
		wire.Catalog.Displays = nil
		wire.Schema = contract.DesktopDiscoveryVersion
	case discoveryEpoch != nil && exchange.Epoch == *discoveryEpoch:
		wire.Schema = contract.DesktopDiscoveryVersion
	default:
		wire.Catalog.Displays = nil
	}

	err := client.Call(ctx, "/v1/desktop-worker", wire, &response)
	if negotiate && desktopDiscoveryUnsupported(err) {
		wire.Schema = contract.DesktopVersion
		err = client.Call(ctx, "/v1/desktop-worker", wire, &response)
	}
	if err != nil {
		return response, err
	}
	if response.Schema != contract.DesktopVersion || !validMachineNonce(response.Epoch) || len(response.Directives) > 1 || len(response.Forget) > 1 ||
		exchange.Epoch != "" && response.Epoch != exchange.Epoch {
		return response, errors.New("desktop broker lifetime or response bounds changed")
	}
	if probe && (len(response.Directives) != 0 || len(response.Forget) != 0) {
		return response, errors.New("desktop broker returned session effects during discovery")
	}
	for _, directive := range response.Directives {
		if directive.Open.Epoch != response.Epoch {
			return response, errors.New("desktop directive changed broker lifetime")
		}
		if err := ValidateDesktopDirective(exchange.Catalog, directive); err != nil {
			return response, err
		}
	}
	for _, reference := range response.Forget {
		if reference.Validate() != nil || reference.Epoch != response.Epoch || reference.Target != exchange.Catalog.Target {
			return response, errors.New("desktop forget changed its original identity")
		}
	}
	if negotiate && wire.Schema == contract.DesktopDiscoveryVersion {
		epoch := response.Epoch
		client.desktopDiscoveryEpoch.Store(&epoch)
	}
	return response, nil
}

func ValidateDesktopDirective(catalog contract.DesktopCatalog, directive contract.DesktopDirective) error {
	if directive.Open.Validate() != nil || directive.Open.Target != catalog.Target || directive.Open.Provider != catalog.Provider ||
		!validMachineNonce(directive.Operator) || directive.LeaseMillis < 0 || directive.LeaseMillis > contract.DesktopLease.Milliseconds() ||
		directive.Ack > contract.MaxDesktopSignals || len(directive.Signals) > contract.MaxDesktopSignals ||
		!directive.Close && (directive.LeaseMillis <= 0 || !catalog.View || directive.Open.Mode == "control" && !catalog.Input) {
		return errors.New("desktop native directive changed frozen authority or identity")
	}
	for _, signal := range directive.Signals {
		if signal.Validate() != nil || signal.Reference != directive.Open.Reference() || signal.Type == "answer" {
			return errors.New("desktop native directive contains foreign signaling")
		}
	}
	return nil
}

// GrantDesktopAdministratorInput requests a distinct broker lease using the
// signed, service-owned consent for the exact viewer and desktop session.
func (client *Client) GrantDesktopAdministratorInput(ctx context.Context, consent contract.DesktopAdministratorEnvelope) (contract.DesktopAdministratorEnvelope, error) {
	var grant contract.DesktopAdministratorEnvelope
	err := client.Call(ctx, contract.DesktopAdministratorGrantPath, consent, &grant)
	return grant, err
}

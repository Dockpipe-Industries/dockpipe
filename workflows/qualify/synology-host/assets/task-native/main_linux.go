//go:build linux

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"dockpipe/workflows/internal/synology/endpointtls"
	"dockpipe/workflows/internal/synology/hostidentity"
)

func ordinaryIdentity() error {
	metadata, err := hostidentity.Read(schema)
	if err != nil || metadata.Identity.UID <= 0 || metadata.Identity.UID != metadata.Identity.EUID {
		return errors.New("ordinary_identity_required")
	}
	for _, key := range []string{"CapEff", "CapPrm", "CapInh", "CapAmb"} {
		if value, found := metadata.Identity.Capabilities[key]; found && value != "0000000000000000" {
			return errors.New("capabilities_not_allowed")
		}
	}
	return nil
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("expected_role")
	}
	if err := ordinaryIdentity(); err != nil {
		return err
	}
	switch arguments[0] {
	case "endpoint-init":
		if len(arguments) != 1 {
			return errors.New("unexpected_arguments")
		}
		return endpointtls.Initialize(packageRoot + "/var")
	case "serve":
		if len(arguments) != 1 {
			return errors.New("unexpected_arguments")
		}
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer cancel()
		return serve(ctx)
	case "probe":
		options := flag.NewFlagSet("probe", flag.ContinueOnError)
		socket := options.String("socket", "", "fixed qualification socket")
		nonce := options.String("nonce", "", "one-shot challenge")
		if options.Parse(arguments[1:]) != nil || options.NArg() != 0 || *socket != callbackSocket || !validNonce(*nonce) {
			return errors.New("invalid_probe_arguments")
		}
		return runProbe(*socket, *nonce)
	default:
		return errors.New("unknown_role")
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		// Keep scheduler and package logs free of command output or credentials.
		fmt.Fprintln(os.Stderr, "Dockpipe host task qualification failed:", safeFailureReason(err))
		os.Exit(1)
	}
}

// Emit only fixed owning-boundary reasons. Neither vendor stderr nor arbitrary
// returned strings become package or Task Scheduler output.
func safeFailureReason(err error) string {
	switch err.Error() {
	case "ordinary_identity_required", "capabilities_not_allowed", "host_identity_unavailable",
		"probe_report_exceeds_budget", "probe_callback_unavailable", "probe_callback_deadline_unavailable",
		"probe_callback_failed", "probe_callback_rejected", "invalid_probe_arguments",
		"endpoint_tls_unavailable", "runtime_directory_unavailable", "callback_path_not_absent",
		"callback_listener_unavailable", "callback_permissions_unavailable", "https_listener_unavailable",
		"callback_listener_stopped", "https_service_stopped":
		return err.Error()
	default:
		return "qualification_unavailable"
	}
}

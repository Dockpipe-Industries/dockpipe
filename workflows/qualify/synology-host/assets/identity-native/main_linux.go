//go:build linux

// This companion qualifies one fixed identity report, never a terminal command.
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dockpipe/workflows/internal/synology/dsmproxy"
	"dockpipe/workflows/internal/synology/localhttp"
)

const listenAddress = "127.0.0.1:38141"
const reportScriptName = "/webman/3rdparty/dockpipe-host-identity-qualification/host-auth.cgi"
const overallTimeout = 8 * time.Second

type startupFailure struct {
	stage string
}

func (failure *startupFailure) Error() string {
	return "identity qualification startup failed"
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "Identity qualification accepts no command arguments.")
		os.Exit(1)
	}
	var err error
	switch filepath.Base(os.Args[0]) {
	case "identity-probe":
		err = runProbe()
	case "identity-worker":
		err = runWorker()
	case "identity-cap-helper":
		err = runDaemon()
	case "identity-endpoint-init":
		err = initializeFixedEndpoint()
		if err != nil {
			err = &startupFailure{stage: "endpoint_tls_init"}
		}
	default:
		err = errors.New("unsupported identity qualification executable role")
	}
	if err != nil {
		// Never print raw operation errors, process output or request metadata.
		var failure *startupFailure
		if errors.As(err, &failure) {
			fmt.Fprintf(os.Stderr, "Identity qualification startup failed: stage=%s.\n", failure.stage)
		} else {
			fmt.Fprintln(os.Stderr, "Identity qualification failed its fixed admission or operation contract.")
		}
		os.Exit(1)
	}
}

func runDaemon() error {
	if _, err := prepareCapabilities(false); err != nil {
		return &startupFailure{stage: "capability_admission"}
	}
	image, err := sealRunningExecutable()
	if err != nil {
		return &startupFailure{stage: "sealed_executable"}
	}
	defer image.Close()
	endpoint, err := loadEndpoint(endpointVar)
	if err != nil {
		return &startupFailure{stage: "endpoint_tls_load"}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := serve(ctx, image, endpoint); err != nil {
		return &startupFailure{stage: "endpoint_tls_service"}
	}
	return nil
}

func identityHandler(invoke func(context.Context, []string) (workerResponse, error)) http.Handler {
	operation := make(chan struct{}, 1)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'self'")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		fail := func(code int, reason string) {
			writer.WriteHeader(code)
			_ = json.NewEncoder(writer).Encode(map[string]string{"error": reason})
		}
		if request.Method != http.MethodGet || request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
			fail(405, "read_only_request_required")
			return
		}
		if request.URL.Path == "/health" && request.URL.RawQuery == "" {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		if request.URL.Path != "/report" {
			fail(404, "not_found")
			return
		}
		environment, err := dsmproxy.Environment(request, reportScriptName)
		if err != nil {
			fail(403, "invalid_proxy_metadata")
			return
		}
		if strings.TrimSpace(request.Header.Get("Cookie")) == "" {
			fail(401, "unauthenticated")
			return
		}
		select {
		case operation <- struct{}{}:
			defer func() { <-operation }()
		default:
			fail(503, "qualification_busy")
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), overallTimeout)
		defer cancel()
		response, err := invoke(ctx, environment)
		if err != nil {
			fail(503, "identity_worker_unavailable")
			return
		}
		if response.Error != "" {
			switch response.Error {
			case "unauthenticated":
				fail(401, "unauthenticated")
			case "authentication_unavailable", "local_account_ineligible", "delegation_failed":
				fail(503, response.Error)
			default:
				fail(503, "identity_qualification_unavailable")
			}
			return
		}
		if response.Report == nil || response.Report.Schema != schema || !response.Report.CapabilityAdmission {
			fail(503, "invalid_qualification_evidence")
			return
		}
		if err := writeResponse(writer, response.Report); err != nil {
			// Serialization limits are checked before any response bytes publish.
			fail(503, "response_exceeds_budget")
		}
	})
}

func serve(ctx context.Context, image *sealedImage, endpoint *tls.Config) error {
	listener, err := net.Listen("tcp4", listenAddress)
	if err != nil {
		return errors.New("identity report listener unavailable")
	}
	defer listener.Close()
	handler := identityHandler(func(ctx context.Context, environment []string) (workerResponse, error) {
		return invokeWorker(ctx, environment, image)
	})
	server := localhttp.Server(handler, 32<<10, overallTimeout+time.Second)
	finished := make(chan error, 1)
	go func() {
		bounded := &localhttp.Listener{Listener: listener, Slots: make(chan struct{}, 16)}
		finished <- server.Serve(tls.NewListener(bounded, endpoint))
	}()
	select {
	case err := <-finished:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if server.Shutdown(shutdown) != nil {
			_ = server.Close()
		}
		<-finished
		return nil
	}
}

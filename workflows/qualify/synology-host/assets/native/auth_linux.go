//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os/exec"

	"dockpipe/workflows/internal/synology/dsmauth"
)

const serviceSocket = "/var/packages/dockpipe-host-qualification/var/run/probe.sock"
const requestTimeout = dsmauth.Timeout
const maximumHelperOutput = dsmauth.MaximumHelperOutput

var errUnauthenticated = dsmauth.ErrUnauthenticated
var errAuthenticationFault = dsmauth.ErrFault

type authenticationFailure = dsmauth.Failure
type unavailableResponse struct {
	Error    string `json:"error"`
	Stage    string `json:"stage"`
	Reason   string `json:"reason"`
	ExitCode *int   `json:"helper_exit_code,omitempty"`
}

func authenticationEnvironment(environment []string) ([]string, error) {
	return dsmauth.Environment(environment)
}
func authenticate(ctx context.Context, environment []string) (string, error) {
	return dsmauth.Authenticate(ctx, environment)
}
func executeAuthenticationHelper(command *exec.Cmd) (string, error) {
	return dsmauth.Run(command)
}
func authenticatedUsername(output string) (string, error) {
	return dsmauth.Username(output)
}

type serviceResult struct {
	Available bool    `json:"available"`
	Status    *status `json:"status,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}

type authenticationResponse struct {
	Schema            string        `json:"schema"`
	Hostname          string        `json:"hostname"`
	AuthenticatedUser string        `json:"authenticated_user"`
	Identity          identity      `json:"identity"`
	Service           serviceResult `json:"service"`
}

type authenticator func(context.Context, []string) (string, error)
type serviceQuery func(context.Context) serviceResult

func secureHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'self'")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
}

func writeBoundedJSON(writer http.ResponseWriter, value any) {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded)+1 > maximumResponseBytes {
		writeFailure(writer, http.StatusServiceUnavailable, "response_exceeds_budget")
		return
	}
	_, _ = writer.Write(append(encoded, '\n'))
}

func writeFailure(writer http.ResponseWriter, code int, reason string) {
	writer.WriteHeader(code)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": reason})
}

func writeUnavailable(writer http.ResponseWriter, stage string, err error) {
	response := unavailableResponse{Error: "unavailable", Stage: stage, Reason: "metadata_unavailable"}
	if stage == "dsm_authentication" {
		response.Reason = "helper_process_failed"
		var failure *authenticationFailure
		if errors.As(err, &failure) {
			switch failure.Reason {
			case "helper_start_failed", "helper_exit_nonzero", "helper_timeout", "helper_cancelled", "helper_output_limit", "helper_process_failed":
				response.Reason = failure.Reason
			}
			if failure.Reason == "helper_exit_nonzero" && failure.ExitCode != nil && *failure.ExitCode > 0 && *failure.ExitCode <= 255 {
				response.ExitCode = failure.ExitCode
			}
		} else if errors.Is(err, context.DeadlineExceeded) {
			response.Reason = "helper_timeout"
		} else if errors.Is(err, context.Canceled) {
			response.Reason = "helper_cancelled"
		}
	}
	writer.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(writer).Encode(response)
}

func readOnlyRequest(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", "GET")
		writeFailure(writer, http.StatusMethodNotAllowed, "method_not_allowed")
		return false
	}
	if request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
		writeFailure(writer, http.StatusBadRequest, "body_not_allowed")
		return false
	}
	return true
}

func authenticationHandler(environment []string, auth authenticator, query serviceQuery) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		secureHeaders(writer)
		if !readOnlyRequest(writer, request) {
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), requestTimeout)
		defer cancel()
		username, err := auth(ctx, environment)
		if err != nil {
			if errors.Is(err, errUnauthenticated) {
				writeFailure(writer, http.StatusUnauthorized, "unauthenticated")
			} else {
				writeUnavailable(writer, "dsm_authentication", err)
			}
			return
		}
		// Keep the output invariant even when a fixture authenticator is supplied.
		username, err = authenticatedUsername(username)
		if err != nil {
			writeFailure(writer, http.StatusUnauthorized, "unauthenticated")
			return
		}
		metadata, err := currentStatus()
		if err != nil {
			writeUnavailable(writer, "process_status", err)
			return
		}
		writeBoundedJSON(writer, authenticationResponse{
			Schema: schema, Hostname: metadata.Hostname, AuthenticatedUser: username, Identity: metadata.Identity,
			Service: query(ctx),
		})
	})
}

func queryService(ctx context.Context) serviceResult {
	return querySocket(ctx, serviceSocket)
}

func inProcessServiceStatus(ctx context.Context) serviceResult {
	if ctx.Err() != nil {
		return serviceResult{Reason: "unavailable"}
	}
	metadata, err := currentStatus()
	if err != nil {
		return serviceResult{Reason: "unavailable"}
	}
	return serviceResult{Available: true, Status: &metadata}
}

func querySocket(ctx context.Context, socket string) serviceResult {
	if err := admitSocket(socket, true); err != nil {
		return serviceResult{Reason: "unavailable"}
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: 4096,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: requestTimeout}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/v1/status", nil)
	response, err := client.Do(request)
	if err != nil {
		return serviceResult{Reason: "unavailable"}
	}
	defer response.Body.Close()
	metadata, err := decodeStatus(response)
	if err != nil {
		return serviceResult{Reason: "unavailable"}
	}
	return serviceResult{Available: true, Status: &metadata}
}

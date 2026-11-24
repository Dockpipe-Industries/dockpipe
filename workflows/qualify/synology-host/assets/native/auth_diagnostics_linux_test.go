//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestFailureDiagnosticHelperProcess(t *testing.T) {
	switch os.Getenv("DOCKPIPE_QUALIFICATION_DIAGNOSTIC_FIXTURE") {
	case "exit-five":
		_, _ = io.WriteString(os.Stdout, "helper-stdout-secret")
		_, _ = io.WriteString(os.Stderr, "helper-stderr-secret")
		os.Exit(5)
	case "overflow":
		_, _ = io.WriteString(os.Stderr, strings.Repeat("x", maximumHelperOutput+1))
		os.Exit(0)
	case "signal":
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		os.Exit(2)
	case "wait":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func TestHelperFailureDiagnosticsAreCredentialSafe(t *testing.T) {
	for _, test := range []struct {
		mode       string
		wantReason string
		wantExit   int
	}{
		{mode: "missing", wantReason: "helper_start_failed"},
		{mode: "exit-five", wantReason: "helper_exit_nonzero", wantExit: 5},
		{mode: "overflow", wantReason: "helper_output_limit"},
		{mode: "signal", wantReason: "helper_process_failed"},
		{mode: "wait", wantReason: "helper_cancelled"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFailureDiagnosticHelperProcess$")
			if test.mode == "missing" {
				command = exec.CommandContext(ctx, "/nonexistent/helper-path-secret")
			}
			command.Env = append(os.Environ(), "DOCKPIPE_QUALIFICATION_DIAGNOSTIC_FIXTURE="+test.mode)
			output, err := executeAuthenticationHelper(command)
			if output != "" || !errors.Is(err, errAuthenticationFault) {
				t.Fatal("helper failure leaked output or lost its failure contract")
			}
			response := httptest.NewRecorder()
			writeUnavailable(response, "dsm_authentication", err)
			var diagnostic unavailableResponse
			if json.Unmarshal(response.Body.Bytes(), &diagnostic) != nil || response.Code != http.StatusServiceUnavailable || diagnostic.Error != "unavailable" || diagnostic.Stage != "dsm_authentication" || diagnostic.Reason != test.wantReason {
				t.Fatalf("unexpected safe diagnostic: %s", response.Body.String())
			}
			if test.wantExit == 0 && diagnostic.ExitCode != nil {
				t.Fatal("exit code reported for a non-exit failure")
			}
			if test.wantExit != 0 && (diagnostic.ExitCode == nil || *diagnostic.ExitCode != test.wantExit) {
				t.Fatal("normal helper exit code was not reported")
			}
			if strings.Contains(response.Body.String(), "secret") {
				t.Fatal("helper data reached failure JSON")
			}
		})
	}
}

func TestFailureStageDiagnosticsRejectRawErrors(t *testing.T) {
	for _, test := range []struct {
		stage      string
		err        error
		wantReason string
	}{
		{stage: "dsm_authentication", err: context.DeadlineExceeded, wantReason: "helper_timeout"},
		{stage: "dsm_authentication", err: context.Canceled, wantReason: "helper_cancelled"},
		{stage: "dsm_authentication", err: errors.New("raw-error-secret"), wantReason: "helper_process_failed"},
		{stage: "dsm_authentication", err: &authenticationFailure{Reason: "forged-reason-secret"}, wantReason: "helper_process_failed"},
		{stage: "process_status", err: errors.New("status-error-secret"), wantReason: "metadata_unavailable"},
	} {
		response := httptest.NewRecorder()
		writeUnavailable(response, test.stage, test.err)
		var diagnostic unavailableResponse
		if json.Unmarshal(response.Body.Bytes(), &diagnostic) != nil || diagnostic.Reason != test.wantReason || diagnostic.ExitCode != nil || strings.Contains(response.Body.String(), "secret") {
			t.Fatalf("unsafe failure diagnostic: %s", response.Body.String())
		}
	}
}

func TestAuthenticationHandlerIncludesOnlySafeFailureDiagnostics(t *testing.T) {
	exitCode := 5
	handler := authenticationHandler([]string{"HTTP_COOKIE=id=environment-secret"}, func(context.Context, []string) (string, error) {
		return "stdout-secret", &authenticationFailure{Reason: "helper_exit_nonzero", ExitCode: &exitCode}
	}, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	var diagnostic unavailableResponse
	if response.Code != 503 || json.Unmarshal(response.Body.Bytes(), &diagnostic) != nil || diagnostic.ExitCode == nil || *diagnostic.ExitCode != 5 || diagnostic.Stage != "dsm_authentication" {
		t.Fatal("authentication handler lost fixed failure diagnostics")
	}
	if strings.Contains(response.Body.String(), "secret") || strings.Contains(response.Body.String(), "identity") {
		t.Fatal("failed authentication exposed credentials or identity")
	}
}

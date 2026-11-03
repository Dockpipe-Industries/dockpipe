//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

const authenticationHelper = "/usr/syno/synoman/webman/modules/authenticate.cgi"
const serviceSocket = "/var/packages/dockpipe-host-qualification/var/run/probe.sock"
const requestTimeout = 3 * time.Second
const maximumHelperOutput = 4096

var errUnauthenticated = errors.New("unauthenticated")
var errAuthenticationFault = errors.New("authentication unavailable")

// The qualification endpoint allows 16 KiB cookies plus 16 KiB of CGI routing
// metadata. Nothing from the general service environment reaches the helper.
func authenticationEnvironment(environment []string) ([]string, error) {
	allowed := map[string]bool{
		"HTTP_COOKIE": true, "REMOTE_ADDR": true, "SERVER_ADDR": true,
		"SERVER_NAME": true, "SERVER_PORT": true, "SERVER_PROTOCOL": true,
		"HTTP_HOST": true, "GATEWAY_INTERFACE": true,
		"REQUEST_METHOD": true, "REQUEST_URI": true, "SCRIPT_NAME": true,
		"QUERY_STRING": true, "HTTPS": true, "HTTP_USER_AGENT": true,
		"HTTP_X_SYNO_TOKEN": true,
	}
	result := []string{"PATH=/usr/bin:/bin", "LANG=C"}
	seen := make(map[string]bool)
	total := 0
	for _, entry := range environment {
		key, value, found := strings.Cut(entry, "=")
		if !found || !allowed[key] {
			continue
		}
		limit := 2048
		if key == "HTTP_COOKIE" {
			limit = 16 << 10
		}
		total += len(entry)
		if seen[key] || len(value) > limit || total > 32<<10 || strings.ContainsAny(value, "\r\n\x00") {
			return nil, errors.New("invalid CGI metadata")
		}
		seen[key] = true
		result = append(result, entry)
	}
	return result, nil
}

// stdout and stderr share one byte budget. Stderr is never retained or returned.
type helperOutput struct {
	mutex     sync.Mutex
	remaining int
	stdout    strings.Builder
	exhausted bool
}

type helperWriter struct {
	output *helperOutput
	retain bool
}

func (writer helperWriter) Write(data []byte) (int, error) {
	writer.output.mutex.Lock()
	defer writer.output.mutex.Unlock()
	if len(data) > writer.output.remaining {
		writer.output.exhausted = true
		return 0, errors.New("helper output limit")
	}
	writer.output.remaining -= len(data)
	if writer.retain {
		writer.output.stdout.Write(data)
	}
	return len(data), nil
}

func authenticate(ctx context.Context, environment []string) (string, error) {
	command := exec.CommandContext(ctx, authenticationHelper)
	command.Env = environment
	output, err := executeAuthenticationHelper(command)
	if err != nil {
		return "", err
	}
	return authenticatedUsername(output)
}

// The command parameter is a unit-test seam. Production constructs it above
// using the fixed DSM helper; neither CGI inputs nor CLI flags select a helper.
func executeAuthenticationHelper(command *exec.Cmd) (string, error) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}
	command.WaitDelay = 100 * time.Millisecond
	defer func() {
		if command.Process != nil {
			// Also release descendants when the helper exits before its pipes do.
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
	}()
	output := &helperOutput{remaining: maximumHelperOutput}
	command.Stdout = helperWriter{output: output, retain: true}
	command.Stderr = helperWriter{output: output}
	if command.Run() != nil || output.exhausted {
		return "", errAuthenticationFault
	}
	return output.stdout.String(), nil
}

func authenticatedUsername(output string) (string, error) {
	username := strings.TrimSuffix(strings.TrimSuffix(output, "\n"), "\r")
	if username == "" || len(username) > 256 || !utf8.ValidString(username) {
		return "", errUnauthenticated
	}
	for _, character := range username {
		if unicode.IsControl(character) || unicode.IsSpace(character) || strings.ContainsRune("/:\x00", character) {
			return "", errUnauthenticated
		}
	}
	return username, nil
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
				writeFailure(writer, http.StatusServiceUnavailable, "unavailable")
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
			writeFailure(writer, http.StatusServiceUnavailable, "unavailable")
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

//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAuthenticationDoesNotTrustRequestClaims(t *testing.T) {
	t.Setenv("HOSTNAME", "forged-host-environment")
	environment, err := authenticationEnvironment([]string{
		"HTTP_COOKIE=id=secret", "REMOTE_ADDR=192.0.2.1", "SERVER_ADDR=192.0.2.2",
		"HTTP_X_DSM_USER=root", "USER=root", "LD_PRELOAD=/untrusted.so",
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := authenticationHandler(environment, func(ctx context.Context, actual []string) (string, error) {
		if _, found := ctx.Deadline(); !found {
			t.Fatal("authentication has no deadline")
		}
		joined := strings.Join(actual, "\n")
		if strings.Contains(joined, "root") || strings.Contains(joined, "LD_PRELOAD") {
			t.Fatal("untrusted environment forwarded")
		}
		return "real-user", nil
	}, func(context.Context) serviceResult { return serviceResult{Reason: "unavailable"} })
	request := httptest.NewRequest("GET", "/host-auth.cgi?username=root", nil)
	request.Header.Set("X-DSM-User", "root")
	request.Host = "forged-host-header"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var result authenticationResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.AuthenticatedUser != "real-user" || strings.Contains(response.Body.String(), "secret") {
		t.Fatal("request identity or credentials escaped authentication boundary")
	}
	hostname, err := os.Hostname()
	if err != nil || result.Hostname != hostname {
		t.Fatal("CGI hostname did not come from the operating system")
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing private response headers")
	}
}

func TestHostIdentityComesFromOperatingSystem(t *testing.T) {
	t.Setenv("HOSTNAME", "forged-host-environment")
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := currentStatus()
	if err != nil || metadata.Hostname != hostname {
		t.Fatalf("native host identity does not match operating system: %v", err)
	}
	for _, invalid := range []string{"", strings.Repeat("x", 256), "host\nforged", "host name", string([]byte{0xff})} {
		if validHostname(invalid) {
			t.Fatal("invalid hostname accepted")
		}
	}
}

func TestAuthenticationFailures(t *testing.T) {
	for _, test := range []struct {
		name     string
		username string
		err      error
		want     int
	}{
		{name: "empty", want: 401},
		{name: "forged multiline", username: "alice\nroot", want: 401},
		{name: "helper denied", err: errUnauthenticated, want: 401},
		{name: "helper failure", err: errors.New("cookie=secret"), want: 503},
		{name: "cancelled", err: context.Canceled, want: 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := authenticationHandler(nil, func(context.Context, []string) (string, error) {
				return test.username, test.err
			}, func(context.Context) serviceResult {
				t.Fatal("queried native service after failed authentication")
				return serviceResult{}
			})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
			if response.Code != test.want || strings.Contains(response.Body.String(), "secret") {
				t.Fatalf("unsafe failure response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestAuthenticationEnvironmentBudgets(t *testing.T) {
	for _, environment := range [][]string{
		{"HTTP_COOKIE=" + strings.Repeat("x", (16<<10)+1)},
		{"REMOTE_ADDR=192.0.2.1\r\nX-User: root"},
		{"HTTP_COOKIE=one", "HTTP_COOKIE=two"},
		{"REQUEST_URI=" + strings.Repeat("x", 2049)},
	} {
		if _, err := authenticationEnvironment(environment); err == nil {
			t.Fatal("accepted metadata outside the request contract")
		}
	}
	if _, err := authenticationEnvironment([]string{"HTTP_COOKIE=" + strings.Repeat("x", 16<<10)}); err != nil {
		t.Fatalf("rejected exact cookie budget: %v", err)
	}
}

func TestAuthenticationHelperProcess(t *testing.T) {
	switch os.Getenv("DOCKPIPE_QUALIFICATION_TEST_HELPER") {
	case "success":
		_, _ = io.WriteString(os.Stdout, "alice\n")
		os.Exit(0)
	case "overflow":
		_, _ = io.WriteString(os.Stderr, strings.Repeat("secret", maximumHelperOutput))
		os.Exit(0)
	case "wait":
		time.Sleep(time.Minute)
		os.Exit(0)
	case "descendant":
		child := exec.Command(os.Args[0], "-test.run=^TestAuthenticationHelperProcess$")
		child.Env = append(os.Environ(), "DOCKPIPE_QUALIFICATION_TEST_HELPER=wait")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		if os.WriteFile(os.Getenv("DOCKPIPE_QUALIFICATION_TEST_PID"), []byte(strconv.Itoa(child.Process.Pid)), 0600) != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
}

func TestHelperDescendantDoesNotOutlivePipeFault(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "descendant-pid")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAuthenticationHelperProcess$")
	command.Env = append(os.Environ(), "DOCKPIPE_QUALIFICATION_TEST_HELPER=descendant", "DOCKPIPE_QUALIFICATION_TEST_PID="+pidFile)
	if output, err := executeAuthenticationHelper(command); !errors.Is(err, errAuthenticationFault) || output != "" {
		t.Fatal("inherited pipe fault succeeded or leaked output")
	}
	encodedPID, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(encodedPID))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		processStatus, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
		// A dead child may remain as a zombie until the host's init reaps it.
		if errors.Is(err, os.ErrNotExist) || strings.Contains(string(processStatus), "State:\tZ") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper descendant survived process-group cleanup")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHelperExecutionLimitsAndCancellation(t *testing.T) {
	for _, mode := range []string{"success", "overflow", "wait"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAuthenticationHelperProcess$")
			// Race-instrumented helper subprocesses otherwise sleep for one
			// second after exit, obscuring the helper's cancellation budget.
			command.Env = append(os.Environ(), "DOCKPIPE_QUALIFICATION_TEST_HELPER="+mode, "GORACE=atexit_sleep_ms=0")
			started := time.Now()
			output, err := executeAuthenticationHelper(command)
			if mode == "success" {
				if err != nil || output != "alice\n" {
					t.Fatalf("helper success failed: %q %v", output, err)
				}
			} else if !errors.Is(err, errAuthenticationFault) || output != "" {
				t.Fatal("helper failure leaked output or succeeded")
			}
			if time.Since(started) > time.Second {
				t.Fatal("helper exceeded cancellation cleanup bound")
			}
		})
	}
}

func TestServiceRoutesAreReadOnly(t *testing.T) {
	for _, test := range []struct {
		method string
		path   string
		body   string
		want   int
	}{
		{method: "GET", path: "/v1/status", want: 200},
		{method: "POST", path: "/v1/status", want: 405},
		{method: "GET", path: "/v1/status", body: "command=id", want: 400},
		{method: "GET", path: "/v1/status?username=root", want: 404},
		{method: "GET", path: "/v1/terminal", want: 404},
	} {
		response := httptest.NewRecorder()
		var body io.Reader
		if test.body != "" {
			body = strings.NewReader(test.body)
		}
		serviceHandler().ServeHTTP(response, httptest.NewRequest(test.method, test.path, body))
		if response.Code != test.want {
			t.Fatalf("%s %s: got %d, want %d", test.method, test.path, response.Code, test.want)
		}
	}
}

func TestCGIRejectsBodyAndWrites(t *testing.T) {
	handler := authenticationHandler(nil, func(context.Context, []string) (string, error) {
		t.Fatal("authentication invoked for forbidden request")
		return "", nil
	}, nil)
	for _, request := range []*http.Request{
		httptest.NewRequest("POST", "/", nil),
		httptest.NewRequest("GET", "/", strings.NewReader("body")),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 400 && response.Code != 405 {
			t.Fatalf("unsafe method/body accepted: %d", response.Code)
		}
	}
}

func TestSocketAdmission(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "probe.sock")
	if err := admitSocket(path, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if admitSocket(path, false) == nil {
		t.Fatal("accepted nonprivate parent")
	}
	_ = os.Chmod(directory, 0700)
	if err := os.Symlink("missing", path); err != nil {
		t.Fatal(err)
	}
	if admitSocket(path, false) == nil || admitSocket(path, true) == nil {
		t.Fatal("accepted socket symlink")
	}
}

func TestSocketFrameworkAliasAndPrivateLeaf(t *testing.T) {
	root := t.TempDir()
	anchor := filepath.Join(root, "actual-var")
	if err := os.Mkdir(anchor, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(anchor, "run"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "var")
	if err := os.Symlink(anchor, alias); err != nil {
		t.Fatal(err)
	}
	if err := admitSocket(filepath.Join(alias, "run", "probe.sock"), false); err != nil {
		t.Fatalf("framework alias rejected: %v", err)
	}
	if err := os.Symlink(filepath.Join(anchor, "run"), filepath.Join(anchor, "linked-run")); err != nil {
		t.Fatal(err)
	}
	if admitSocket(filepath.Join(alias, "linked-run", "probe.sock"), false) == nil {
		t.Fatal("runtime leaf symlink accepted")
	}
}

func TestNativeServiceLifecycle(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("service intentionally refuses root")
	}
	directory := t.TempDir()
	_ = os.Chmod(directory, 0700)
	path := filepath.Join(directory, "probe.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan error, 1)
	go func() { completed <- serve(ctx, path) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		select {
		case err := <-completed:
			t.Fatalf("service exited before readiness: %v", err)
		default:
		}
		if admitSocket(path, true) == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("service did not acquire private socket")
		}
		time.Sleep(5 * time.Millisecond)
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	response, err := client.Get("http://localhost/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := decodeStatus(response)
	response.Body.Close()
	if err != nil || metadata.Identity.EUID != os.Geteuid() {
		t.Fatalf("invalid native status: %v", err)
	}
	if !querySocket(ctx, path).Available {
		t.Fatal("bounded status client could not query owned socket")
	}
	if err := runCommand([]string{"status", "--socket", path}); err != nil {
		t.Fatalf("status command failed: %v", err)
	}
	cancel()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("service shutdown exceeded deadline")
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("service left its socket behind")
	}
}

func TestCapabilityMetadataRejectsIncompleteAndOverflow(t *testing.T) {
	if _, err := parseCapabilities(strings.NewReader("CapEff:\t0000000000000000\n")); err == nil {
		t.Fatal("accepted incomplete metadata")
	}
	if _, err := parseCapabilities(strings.NewReader(strings.Repeat("x", maximumStatusBytes+1))); err == nil {
		t.Fatal("accepted excessive metadata")
	}
}

func TestOlderKernelCapabilityMetadata(t *testing.T) {
	oldKernelStatus := "CapInh:\t0000000000000000\nCapPrm:\t0000000000000000\nCapEff:\t0000000000000000\nCapBnd:\t0000003fffffffff\n"
	capabilities, err := parseCapabilities(strings.NewReader(oldKernelStatus))
	if err != nil {
		t.Fatalf("valid older kernel status rejected: %v", err)
	}
	if _, present := capabilities["CapAmb"]; present {
		t.Fatal("missing ambient capability metadata was synthesized")
	}
	for _, additional := range []string{
		"NoNewPrivs:\tinvalid\n",
		"CapAmb:\tnot-a-capability\n",
		"CapEff:\t0000000000000000\n",
	} {
		if _, err := parseCapabilities(strings.NewReader(oldKernelStatus + additional)); err == nil {
			t.Fatal("malformed present metadata accepted")
		}
	}
	if _, err := parseCapabilities(strings.NewReader(oldKernelStatus + "CapAmb:\t0000000000000000\nNoNewPrivs:\t1\n")); err != nil {
		t.Fatalf("valid newer kernel status rejected: %v", err)
	}
}

func TestNoNewPrivilegesGetterOutcomes(t *testing.T) {
	for _, value := range []uintptr{0, 1} {
		enabled, err := noNewPrivilegesResult(value, 0)
		if err != nil || enabled == nil || *enabled != (value == 1) {
			t.Fatalf("getter result %d misreported", value)
		}
	}
	for _, errno := range []syscall.Errno{syscall.EINVAL, syscall.ENOSYS} {
		enabled, err := noNewPrivilegesResult(0, errno)
		if err != nil || enabled != nil {
			t.Fatal("unsupported getter did not produce explicit unknown")
		}
	}
	if _, err := noNewPrivilegesResult(0, syscall.EPERM); err == nil {
		t.Fatal("getter permission failure was hidden")
	}
	if _, err := noNewPrivilegesResult(2, 0); err == nil {
		t.Fatal("invalid getter value accepted")
	}
	encoded, err := json.Marshal(identity{})
	if err != nil || !strings.Contains(string(encoded), `"no_new_privs":null`) {
		t.Fatal("unknown no-new-privileges flag was serialized as false")
	}
}

func TestNativeServiceResponseBudget(t *testing.T) {
	response := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maximumResponseBytes+1)))}
	if _, err := decodeStatus(response); err == nil {
		t.Fatal("accepted oversized native service response")
	}
	writer := httptest.NewRecorder()
	writeBoundedJSON(writer, strings.Repeat("x", maximumResponseBytes))
	if writer.Code != 503 || strings.Contains(writer.Body.String(), "xxxx") {
		t.Fatal("oversized response was partially published")
	}
}

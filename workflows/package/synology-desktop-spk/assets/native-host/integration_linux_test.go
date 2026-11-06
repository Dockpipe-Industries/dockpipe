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
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure/nativehost"
	"dockpipe/src/lib/infrastructure/telemetry"
	"dockpipe/workflows/internal/synology/localaccount"
)

func TestNativeServerRealBrokerWorkerLifecycle(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the native worker deliberately rejects root")
	}
	accountRecord, err := user.LookupId(strconv.Itoa(os.Getuid()))
	if err != nil {
		var unknown user.UnknownUserIdError
		if !errors.As(err, &unknown) {
			t.Fatal(err)
		}
		workerErr := nativehost.RunWorker(context.Background(), nativehost.WorkerOptions{
			Socket: "/tmp/native-host-unadmitted-service-worker.sock",
			Nonce:  strings.Repeat("a", 64),
		})
		if workerErr == nil || workerErr.Error() != "native host worker account unavailable" {
			t.Fatalf("nameless UID worker admission = %v", workerErr)
		}
		t.Skip("real worker lifecycle requires an ordinary UID with an actual account; nameless UID rejection verified")
	}
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatal(err)
	}
	account := localaccount.Account{
		Username: accountRecord.Username,
		UID:      uint32(os.Getuid()),
		GID:      uint32(os.Getgid()),
	}
	for _, group := range groups {
		account.Groups = append(account.Groups, uint32(group))
	}
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}

	// Keep Unix paths short and all authority files under private temporary
	// directories. No package installation paths or DSM state are touched.
	directory, err := os.MkdirTemp("", "native-e2e-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })
	serverRoot := filepath.Join(directory, "server")
	clientRoot := filepath.Join(directory, "client")
	for _, path := range []string{serverRoot, clientRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	leases, err := newLeaseManager(ownerStore{path: filepath.Join(serverRoot, "owner.json")})
	if err != nil {
		t.Fatal(err)
	}
	key, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	workerPath := filepath.Join(serverRoot, "worker.sock")
	clientPath := filepath.Join(clientRoot, "client.sock")
	broker, err := nativehost.NewBroker(nativehost.Options{
		WorkerSocket: workerPath,
		ClientSocket: clientPath,
		ClientKey:    key,
		ClientUID:    uint32(os.Getuid()),
		Authorize:    leases.authorize,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	brokerDone := make(chan error, 1)
	go func() {
		defer close(brokerDone)
		brokerDone <- broker.Serve(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		broker.Close()
		select {
		case <-brokerDone:
		case <-time.After(2 * time.Second):
			t.Error("broker did not stop")
		}
	})
	for deadline := time.Now().Add(2 * time.Second); ; {
		if _, err := os.Stat(clientPath); err == nil {
			break
		}
		select {
		case <-brokerDone:
			t.Fatal("broker could not open its local sockets")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("broker socket startup timed out")
		}
		time.Sleep(time.Millisecond)
	}
	keyPath := filepath.Join(clientRoot, "client.key")
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		t.Fatal(err)
	}
	connectionPath := filepath.Join(clientRoot, "connection.json")
	connection, err := json.Marshal(nativehost.Connection{
		Schema:  nativehost.ConnectionSchema,
		Socket:  clientPath,
		KeyFile: keyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(connectionPath, connection, 0600); err != nil {
		t.Fatal(err)
	}
	client, err := nativehost.NewClient(connectionPath)
	if err != nil {
		t.Fatal(err)
	}

	// Only the DSM authentication/account-resolution boundary is substituted.
	// HTTP admission, persisted lease state, broker and worker remain production.
	handler := &nativeServer{
		leases: leases,
		broker: broker,
		authenticate: func(context.Context, []string) (string, error) {
			return account.Username, nil
		},
		resolve: func(username string) (localaccount.Account, error) {
			if username != account.Username {
				return localaccount.Account{}, errors.New("unexpected account")
			}
			return account, nil
		},
		desktop: http.NotFoundHandler(),
	}
	endpoint := httptest.NewTLSServer(handler)
	t.Cleanup(endpoint.Close)
	httpClient := endpoint.Client()
	httpClient.Timeout = 8 * time.Second
	var accessCookie *http.Cookie
	callAPI := func(method, body string) publicReport {
		t.Helper()
		request, err := http.NewRequestWithContext(ctx, method, endpoint.URL+nativeEndpoint, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Host = "native-e2e.example"
		request.Header.Set("Cookie", "id=synthetic-integration-session")
		request.Header.Set("X-Syno-Token", "synthetic-integration-token")
		request.Header.Set("X-Dockpipe-Remote-Addr", "192.0.2.1")
		request.Header.Set("X-Dockpipe-Server-Addr", "192.0.2.2")
		request.Header.Set("X-Dockpipe-Server-Port", "443")
		request.Header.Set("X-Dockpipe-Scheme", "https")
		request.Header.Set("Origin", "https://"+request.Host)
		request.Header.Set("Content-Type", "application/json")
		if accessCookie != nil {
			request.AddCookie(accessCookie)
		}
		response, err := httpClient.Do(request)
		if err != nil {
			t.Fatal("native API request failed")
		}
		defer response.Body.Close()
		for _, cookie := range response.Cookies() {
			if cookie.Name == gateCookieName {
				accessCookie = cookie
			}
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, maximumHTTPResponseBytes+1))
		if err != nil || len(data) > maximumHTTPResponseBytes || response.StatusCode != http.StatusOK {
			t.Fatalf("native API request failed with HTTP %d", response.StatusCode)
		}
		var report publicReport
		if json.Unmarshal(data, &report) != nil || report.Schema != nativeSchema || report.AuthenticatedUser != account.Username || report.ExpectedUID != account.UID {
			t.Fatal("native API returned an invalid report")
		}
		return report
	}
	initial := callAPI(http.MethodGet, "")
	if initial.Challenge != nil || initial.Result != nil {
		t.Fatal("initial status unexpectedly granted native access")
	}
	if status, err := client.Status(ctx); err == nil || status.Available {
		t.Fatal("native client was authorized before prepare")
	}
	prepared := callAPI(http.MethodPost, `{"action":"prepare"}`)
	if !prepared.Created || prepared.Challenge == nil || len(prepared.Challenge.Nonce) != 64 || prepared.Result != nil {
		t.Fatal("prepare did not commit a worker expectation")
	}
	if accessCookie == nil || !accessCookie.Secure || !accessCookie.HttpOnly {
		t.Fatal("prepare did not establish the protected browser lease cookie")
	}
	if _, err := leases.authorize(); err != nil {
		t.Fatal("prepare did not establish a live browser grant before admission")
	}

	workerContext, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan error, 1)
	workerReady := make(chan struct{})
	go func() {
		workerDone <- nativehost.RunWorker(workerContext, nativehost.WorkerOptions{
			Socket: workerPath,
			Nonce:  prepared.Challenge.Nonce,
			Ready:  func() error { close(workerReady); return nil },
		})
	}()
	workerStopped := false
	t.Cleanup(func() {
		stopWorker()
		broker.Revoke()
		if workerStopped {
			return
		}
		select {
		case <-workerDone:
		case <-time.After(3 * time.Second):
			t.Error("worker did not stop")
		}
	})
	select {
	case <-workerReady:
	case <-workerDone:
		workerStopped = true
		t.Fatal("actual user worker was rejected")
	case <-ctx.Done():
		t.Fatal("actual user worker startup timed out")
	}
	assertConnected := func() nativehost.Status {
		t.Helper()
		status, err := client.Status(ctx)
		if err != nil || !status.Available || !status.Connected || status.UID != account.UID || status.GID != account.GID || status.Username != account.Username || status.Node != hostname || status.Generation == "" || !slices.Equal(status.Groups, account.Groups) {
			t.Fatal("native client identity differs from the actual worker account")
		}
		return status
	}
	status := assertConnected()
	snapshot, err := client.Snapshot(ctx, true, false)
	if err != nil || snapshot.Schema != telemetry.MachineSchema || snapshot.Node != hostname || snapshot.State != "live" || snapshot.Execution == nil || snapshot.Docker != nil {
		t.Fatal("actual user system snapshot was unavailable or represented another machine")
	}
	finishBody, err := json.Marshal(map[string]string{"action": "finish", "nonce": prepared.Challenge.Nonce})
	if err != nil {
		t.Fatal(err)
	}
	finished := callAPI(http.MethodPost, string(finishBody))
	if finished.Challenge != nil || finished.Result == nil || !finished.Result.Available || finished.Result.Generation != status.Generation {
		t.Fatal("finishing the starter revoked the live worker")
	}
	if assertConnected().Generation != status.Generation {
		t.Fatal("finishing the starter changed the active authorization")
	}
	disconnected := callAPI(http.MethodPost, `{"action":"disconnect"}`)
	if disconnected.Challenge != nil || disconnected.Result != nil {
		t.Fatal("disconnect retained the worker or challenge")
	}
	if status, err := client.Status(ctx); err == nil || status.Available || status.Connected {
		t.Fatal("native client remained authorized after disconnect")
	}
	select {
	case <-workerDone:
		workerStopped = true
	case <-time.After(3 * time.Second):
		t.Fatal("disconnect did not terminate the reverse worker")
	}
}

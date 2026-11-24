package nativehost

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func logFrame(stream byte, text string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(text)))
	return append(header, []byte(text)...)
}

func TestDockerQueryUsesReadOnlyBoundedEndpoints(t *testing.T) {
	for _, kind := range []string{"container", "network", "volume"} {
		t.Run(kind, func(t *testing.T) {
			id := "abcdef123456"
			if kind == "volume" {
				id = "project_data"
			}
			var paths []string
			client := &http.Client{Transport: dockerTransport(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet || request.Body != nil {
					t.Fatal("query attempted mutation")
				}
				paths = append(paths, request.URL.RequestURI())
				body := `{"Id":"abcdef1234567890","Config":{"Tty":false}}`
				switch {
				case request.URL.Path == "/version":
					body = `{"ApiVersion":"1.41"}`
				case strings.HasSuffix(request.URL.Path, "/logs"):
					if request.URL.Query().Get("tail") != "200" || request.URL.Query().Get("stdout") != "true" || request.URL.Query().Get("stderr") != "true" {
						t.Fatal("unbounded log query")
					}
					body = string(append(logFrame(1, "out\n"), logFrame(2, "err\n")...))
				case kind == "volume":
					body = `{"Name":"project_data","Mountpoint":"/data"}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			result := dockerQuery(context.Background(), client, kind, id)
			if result.Outcome != "Success" || !validDockerQueryResult(result, kind, id) {
				t.Fatalf("query failed: %+v", result)
			}
			if kind == "container" && (result.Logs != "out\nerr\n" || paths[1] != "/v1.41/containers/abcdef123456/json?size=false") {
				t.Fatalf("invalid log decoding or size scan: %v %+v", paths, result)
			}
			if kind != "container" && len(paths) != 2 {
				t.Fatalf("unexpected extra query: %v", paths)
			}
		})
	}
}

func TestDockerQueryRejectsResourceEscapeAndIdentityMismatch(t *testing.T) {
	for _, pair := range [][2]string{{"container", "../x"}, {"network", "abc"}, {"exec", "abcdef123456"}, {"volume", "../data"}, {"volume", "-option"}, {"volume", strings.Repeat("a", 256)}} {
		if ValidateDockerQuery(pair[0], pair[1]) == nil {
			t.Fatalf("accepted %v", pair)
		}
	}
	for _, body := range []string{`{"Id":"deadbeef0000"}`, `null`, `[]`, strings.Repeat("x", maxDockerInspect+1)} {
		client := &http.Client{Transport: dockerTransport(func(request *http.Request) (*http.Response, error) {
			content := body
			if request.URL.Path == "/version" {
				content = `{"ApiVersion":"1.41"}`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(content))}, nil
		})}
		if result := dockerQuery(context.Background(), client, "container", "abcdef123456"); result.Outcome != "Failure" {
			t.Fatalf("accepted bad response: %+v", result)
		}
	}
}

func TestDockerLogsAreBoundedAndValidateMultiplexFrames(t *testing.T) {
	for _, tty := range []bool{true, false} {
		raw := []byte(strings.Repeat("x", maxDockerLogs+1))
		if !tty {
			raw = logFrame(1, string(raw))
		}
		output, truncated, err := decodeDockerLogs(bytes.NewReader(raw), tty)
		if err != nil || !truncated || len(output) != maxDockerLogs {
			t.Fatalf("bounds failed: %v %v %d", err, truncated, len(output))
		}
	}
	for _, raw := range [][]byte{{1, 0}, logFrame(9, "bad"), []byte{1, 1, 0, 0, 0, 0, 0, 0}, logFrame(1, "lost")[:10]} {
		if _, _, err := decodeDockerLogs(bytes.NewReader(raw), false); err == nil {
			t.Fatal("accepted malformed frame")
		}
	}
}

func TestDockerQueryPreservesInspectionWhenLogsUnsupported(t *testing.T) {
	client := &http.Client{Transport: dockerTransport(func(request *http.Request) (*http.Response, error) {
		body, status := `{"Id":"abcdef123456"}`, 200
		if request.URL.Path == "/version" {
			body = `{"ApiVersion":"1.41"}`
		}
		if strings.HasSuffix(request.URL.Path, "/logs") {
			status = 501
			body = "driver does not support logs"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	result := dockerQuery(context.Background(), client, "container", "abcdef123456")
	if result.Outcome != "Success" || result.LogsError != "docker_logs_unavailable" || len(result.Inspect) == 0 {
		t.Fatalf("lost inspect: %+v", result)
	}
	result.ID = "deadbeef0000"
	if validDockerQueryResult(result, "container", "abcdef123456") {
		t.Fatal("accepted crossed response")
	}
}

func TestDockerQueryCancellationAndEncodedLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Transport: dockerTransport(func(request *http.Request) (*http.Response, error) {
		cancel()
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	if result := dockerQuery(ctx, client, "container", "abcdef123456"); result.Outcome != "Cancelled" {
		t.Fatalf("lost cancellation: %+v", result)
	}
	result := DockerQueryResult{Schema: DockerQuerySchema, Outcome: "Success", Kind: "container", ID: "abcdef123456", Inspect: json.RawMessage(`{"Id":"abcdef123456"}`), Logs: strings.Repeat("x", maxDockerLogs+1)}
	if validDockerQueryResult(result, "container", "abcdef123456") {
		t.Fatal("accepted oversized response")
	}
}

func TestDockerQueryBrokerRoundTripAndCancellation(t *testing.T) {
	broker, client, _ := startTestBroker(t)
	cancelledContext, cancelEarly := context.WithCancel(context.Background())
	cancelEarly()
	if result, err := client.DockerQuery(cancelledContext, "container", "abcdef123456"); err == nil || result.Outcome != "Cancelled" {
		t.Fatalf("early cancellation was lost: %+v %v", result, err)
	}
	nonce := strings.Repeat("f", 64)
	if err := broker.ExpectWorker(uint32(os.Getuid()), uint32(os.Getgid()), nonce, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	identity, err := workerIdentity()
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.Dial("unix", broker.options.WorkerSocket)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(3 * time.Second))
	worker := &wire{ReadWriteCloser: connection}
	if err := worker.send(frame{Kind: "worker", Nonce: nonce, Status: &identity}); err != nil {
		t.Fatal(err)
	}
	if accepted, err := worker.receive(); err != nil || accepted.Kind != "accepted" {
		t.Fatalf("admission: %+v %v", accepted, err)
	}
	done := make(chan error, 1)
	go func() {
		result, err := client.DockerQuery(context.Background(), "container", "abcdef123456")
		if err == nil && (result.Logs != "actual log\n" || result.Kind != "container") {
			err = errors.New("response mismatch")
		}
		done <- err
	}()
	request, err := worker.receive()
	if err != nil || request.Kind != "docker-query" || request.ResourceKind != "container" || request.ResourceID != "abcdef123456" {
		t.Fatalf("query: %+v %v", request, err)
	}
	result := DockerQueryResult{Schema: DockerQuerySchema, Outcome: "Success", Kind: "container", ID: "abcdef123456", Inspect: json.RawMessage(`{"Id":"abcdef123456"}`), Logs: "actual log\n"}
	if err := worker.send(frame{Kind: "docker-query", ID: request.ID, DockerQuery: &result}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if closed, err := worker.receive(); err != nil || closed.Kind != "close" {
		t.Fatalf("cleanup: %+v %v", closed, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, err := client.DockerQuery(ctx, "volume", "project_data"); done <- err }()
	request, err = worker.receive()
	if err != nil || request.Kind != "docker-query" {
		t.Fatalf("query: %+v %v", request, err)
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled query succeeded")
	}
	if closed, err := worker.receive(); err != nil || closed.Kind != "close" {
		t.Fatalf("cancel not relayed: %+v %v", closed, err)
	}
}

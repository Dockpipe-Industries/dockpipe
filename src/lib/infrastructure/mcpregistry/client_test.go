package mcpregistry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const recordJSON = `{"server":{"name":"org.example/server","version":"1.2.3","title":"Example","description":"Untrusted","packages":[{"registryType":"npm","identifier":"@example/server","version":"1.2.3","transport":{"type":"stdio"},"runtimeArguments":[{"value":"--untrusted"}]}],"remotes":[{"type":"streamable-http","url":"https://example.test/mcp"}],"futureExtension":{"preserved":true}},"_meta":{"io.modelcontextprotocol.registry/official":{"status":"active","isLatest":true}}}`

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func clientReturning(body string) *Client {
	client := NewClient()
	client.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return response(body), nil })
	return client
}

func TestSearchPaginationAndNormalization(t *testing.T) {
	client := NewClient()
	calls := 0
	client.http.Transport = roundTrip(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != "GET" || request.URL.Host != "registry.modelcontextprotocol.io" || request.URL.Path != "/v0.1/servers" {
			t.Fatal("unexpected endpoint", request.URL)
		}
		q := request.URL.Query()
		if q.Get("search") != "a+b & c" || q.Get("cursor") != "opaque:next&cursor" || q.Get("limit") != "2" || q.Get("version") != "latest" {
			t.Fatal("query not encoded", q)
		}
		if request.Header.Get("Authorization") != "" {
			t.Fatal("unexpected auth")
		}
		return response(`{"servers":[` + recordJSON + `],"metadata":{"nextCursor":"next-page","count":1}}`), nil
	})
	page, err := client.Search(context.Background(), Query{Search: "a+b & c", Cursor: "opaque:next&cursor", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || page.NextCursor != "next-page" || len(page.Servers) != 1 {
		t.Fatal("incorrect pagination", page)
	}
	server := page.Servers[0]
	if server.Name != "org.example/server" || server.Version != "1.2.3" || server.Status != "active" || !server.IsLatest {
		t.Fatal("incorrect normalization", server)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(server.Document, &document); err != nil {
		t.Fatal(err)
	}
	if len(document["futureExtension"]) == 0 || len(document["packages"]) == 0 || len(document["remotes"]) == 0 {
		t.Fatal("metadata silently dropped")
	}
}
func TestVersionPaths(t *testing.T) {
	client := NewClient()
	client.http.Transport = roundTrip(func(request *http.Request) (*http.Response, error) {
		if !strings.Contains(request.URL.EscapedPath(), "org.example%2Fserver") {
			t.Fatal("server name not escaped", request.URL)
		}
		if strings.HasSuffix(request.URL.Path, "/versions") {
			return response(`{"servers":[` + recordJSON + `],"metadata":{}}`), nil
		}
		return response(recordJSON), nil
	})
	if _, err := client.Version(context.Background(), "org.example/server", "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Versions(context.Background(), "org.example/server"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Version(context.Background(), "org.example/server", "2.0.0"); err == nil {
		t.Fatal("mismatched version accepted")
	}
}
func TestFailureResponses(t *testing.T) {
	for _, body := range []string{"not JSON", "null", "{}", `{"servers":null}`, `{"servers":[{}]}`, `{"servers":[` + recordJSON + `,` + recordJSON + `]}`, strings.Repeat(" ", maxResponseBytes+1)} {
		if _, err := clientReturning(body).Search(context.Background(), Query{}); err == nil {
			t.Fatal("invalid response accepted")
		}
	}
	client := NewClient()
	calls := 0
	client.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		r := response("secret error body")
		r.StatusCode = 429
		return r, nil
	})
	if _, err := client.Search(context.Background(), Query{}); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("bad status/error handling", err)
	}
	if calls != 1 {
		t.Fatal("unexpected retry")
	}
}
func TestRequestValidationBeforeNetwork(t *testing.T) {
	client := NewClient()
	client.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		t.Fatal("network called for invalid input")
		return nil, nil
	})
	for _, query := range []Query{{Limit: 101}, {Limit: -1}, {Search: strings.Repeat("a", 513)}, {Cursor: strings.Repeat("a", 4097)}, {Search: "bad\nquery"}} {
		if _, err := client.Search(context.Background(), query); err == nil {
			t.Fatal("invalid query accepted")
		}
	}
	for _, name := range []string{"", "../x", "no-namespace", "org.example/a/b", "org.example/%2fescape", "org.example/server?x"} {
		if _, err := client.Version(context.Background(), name, "1.0.0"); err == nil {
			t.Fatal("invalid name accepted", name)
		}
	}
}
func TestCancellationAndRedirects(t *testing.T) {
	client := NewClient()
	client.http.Transport = roundTrip(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := client.Search(ctx, Query{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("cancellation lost", err)
	}
	calls := 0
	client.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		r := response("")
		r.StatusCode = 302
		r.Header.Set("Location", "https://other.example.test")
		return r, nil
	})
	if _, err := client.Search(context.Background(), Query{}); err == nil {
		t.Fatal("redirect followed")
	}
	if calls != 1 {
		t.Fatal("redirect network request made")
	}
}

type closeBody struct {
	io.Reader
	closed bool
}

func (b *closeBody) Close() error { b.closed = true; return nil }
func TestResponseClosedOnDecodeError(t *testing.T) {
	body := &closeBody{Reader: strings.NewReader("malformed")}
	client := NewClient()
	client.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) { r := response(""); r.Body = body; return r, nil })
	_, _ = client.Search(context.Background(), Query{})
	if !body.closed {
		t.Fatal("response leaked")
	}
}

func TestVersionHistoryIdentity(t *testing.T) {
	client := clientReturning(`{"servers":[` + recordJSON + `],"metadata":{}}`)
	if _, err := client.Versions(context.Background(), "org.other/server"); err == nil {
		t.Fatal("version history returned another server")
	}
}

package remote

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

type machineTestTransport func(*http.Request) (*http.Response, error)

func (transport machineTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type recordedMachineResponse struct {
	status int
	header http.Header
	body   string
}

func (response recordedMachineResponse) response(request *http.Request) *http.Response {
	return &http.Response{StatusCode: response.status, Header: response.header.Clone(),
		Body: io.NopCloser(strings.NewReader(response.body)), Request: request}
}

func TestMachineClientRejectsReplayedChallengeAndResponse(t *testing.T) {
	root := privateTestDirectory(t)
	identity := remoteidentityMustNew(t)
	token, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	state := BrokerState{Schema: contract.Version, AdminHash: Hash(strings.Repeat("a", 64)),
		Nodes: map[string]Enrollment{"worker": {TokenHash: Hash(token), MachineKey: identity.PublicKey}},
		Jobs:  map[string]contract.Job{}}
	if err := WritePrivate(filepath.Join(root, "broker.json"), state); err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient("http://127.0.0.1:47831", token)
	if err != nil {
		t.Fatal(err)
	}
	client.Identity = &identity
	client.BrokerKey = broker.state.Identity.PublicKey
	recorded := map[string]recordedMachineResponse{}
	replay := false
	exchanges := 0
	client.HTTP.Transport = machineTestTransport(func(request *http.Request) (*http.Response, error) {
		exchanges++
		if replay {
			return recorded[request.URL.Path].response(request), nil
		}
		if request.Body == nil {
			request.Body = http.NoBody
		}
		recorder := httptest.NewRecorder()
		broker.ServeHTTP(recorder, request)
		snapshot := recordedMachineResponse{status: recorder.Code, header: recorder.Header().Clone(), body: recorder.Body.String()}
		recorded[request.URL.Path] = snapshot
		return snapshot.response(request), nil
	})
	input := map[string]string{"session": strings.Repeat("b", 64)}
	var output json.RawMessage
	if err := client.Call(context.Background(), "/v1/next", input, &output); err != nil {
		t.Fatal(err)
	}
	if string(output) != "null" || exchanges != 2 {
		t.Fatal("first signed exchange did not complete")
	}
	replay = true
	if err := client.Call(context.Background(), "/v1/next", input, &output); err == nil {
		t.Fatal("recorded broker challenge and response authenticated as a fresh exchange")
	}
}

package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func privateConnectionDirectory(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "private")
	if err := remoteio.PrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeConnectionFixture(t *testing.T, root, name string, value any) {
	t.Helper()
	if err := remoteio.WritePrivate(filepath.Join(root, name), value); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteInfoLegacyAndBoundResolver(t *testing.T) {
	root := privateConnectionDirectory(t)
	info, err := describeRemote(root)
	if err != nil || info.Broker != nil || info.Worker != nil {
		t.Fatalf("empty state: %+v, %v", info, err)
	}
	operator := OperatorConfig{Schema: contract.Version, Endpoint: "https://edge.example.com", Listen: "127.0.0.1:47831", Token: "operator-secret-never-public"}
	writeConnectionFixture(t, root, "operator.json", operator)
	info, err = describeRemote(root)
	if err != nil || info.Broker == nil || info.Broker.Resolver != nil {
		t.Fatalf("legacy provider must remain unknown: %+v, %v", info, err)
	}
	edge := contract.EdgeConfig{Schema: contract.Version, Endpoint: operator.Endpoint, Executable: "/private/provider", Arguments: []string{"edge-secret-never-public"}}
	writeConnectionFixture(t, root, "edge.json", edge)
	metadata := infrastructure.ResolverMetadata{Name: "example.edge", Version: "1.2.3", RemoteSetup: "local"}
	writeConnectionFixture(t, root, "connection.json", connectionConfig{Schema: contract.Version, Mode: "local", Endpoint: operator.Endpoint, Resolver: metadata, EdgeDigest: edgeDigest(edge)})
	writeConnectionFixture(t, root, "worker.json", contract.WorkerConfig{Schema: contract.Version, Endpoint: "https://other.example.com", Node: "worker", Token: "worker-secret-never-public"})
	info, err = describeRemote(root)
	if err != nil || info.Broker.Resolver == nil || info.Broker.Resolver.Name != metadata.Name || info.Worker.Node != "worker" {
		t.Fatalf("projection: %+v, %v", info, err)
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{operator.Token, "edge-secret-never-public", "worker-secret-never-public", "/private/provider"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("private material was included in remote info")
		}
	}
	edge.Arguments = []string{"changed-provider"}
	writeConnectionFixture(t, root, "edge.json", edge)
	info, err = describeRemote(root)
	if err != nil || info.Broker.Resolver != nil {
		t.Fatal("stale provider metadata was presented as current")
	}
	client, err := operatorClient(root)
	if err != nil || client.Endpoint != "http://127.0.0.1:47831" {
		t.Fatalf("local administration must stay on loopback: %v", err)
	}
}

func TestHostedConnectionAdoptionAndRouting(t *testing.T) {
	root := privateConnectionDirectory(t)
	metadata := infrastructure.ResolverMetadata{Name: "example.hosted", Title: "Example Hosted", RemoteSetup: "hosted", Capability: "remote.broker"}
	result := hostedSetupResult{Schema: contract.Version, Endpoint: "https://broker.example.com", Token: strings.Repeat("private", 8)}
	output := filepath.Join(root, "result.json")
	writeConnectionFixture(t, root, "result.json", result)
	checked := false
	check := func(_ context.Context, endpoint, token string) error {
		checked = endpoint == result.Endpoint && token == result.Token
		return nil
	}
	if err := adoptHostedConnection(context.Background(), root, output, metadata, check); err != nil || !checked {
		t.Fatalf("adopt: %v, checked=%v", err, checked)
	}
	client, err := operatorClient(root)
	if err != nil || client.Endpoint != result.Endpoint || client.Token != result.Token {
		t.Fatalf("hosted operator route: %v", err)
	}
	info, err := describeRemote(root)
	if err != nil || info.Broker.Mode != "hosted" || info.Broker.Resolver.Name != metadata.Name {
		t.Fatalf("hosted projection: %+v, %v", info, err)
	}
	data, err := json.Marshal(info)
	if err != nil || strings.Contains(string(data), result.Token) || strings.Contains(string(data), "token") {
		t.Fatal("hosted credential leaked into projection")
	}
	writeConnectionFixture(t, root, "operator.json", OperatorConfig{Schema: contract.Version})
	if _, err := operatorClient(root); err == nil {
		t.Fatal("mixed local and hosted state must fail closed")
	}
	if err := os.Remove(filepath.Join(root, "operator.json")); err != nil {
		t.Fatal(err)
	}
	if err := requireLocalBroker(root); err == nil {
		t.Fatal("hosted connection allowed a local broker")
	}
	if err := validateSetupMode(root, "local", "example.edge"); err == nil {
		t.Fatal("provider switch did not preserve existing connection")
	}
	result.Token = strings.Repeat("renewed", 8)
	writeConnectionFixture(t, root, "result.json", result)
	checkFailure := func(context.Context, string, string) error { return errors.New("offline") }
	if err := adoptHostedConnection(context.Background(), root, output, metadata, checkFailure); err == nil {
		t.Fatal("unreachable broker was adopted")
	}
	saved, err := readConnection(root)
	if err != nil || saved.Token != client.Token {
		t.Fatal("failed sign-in replaced existing credentials")
	}
	result.Endpoint = "https://different.example.com"
	writeConnectionFixture(t, root, "result.json", result)
	if err := adoptHostedConnection(context.Background(), root, output, metadata, check); err == nil {
		t.Fatal("different endpoint silently replaced existing machine authority")
	}
}

func TestHostedConnectionRejectsUnsafeOutput(t *testing.T) {
	for _, test := range []struct {
		name   string
		result hostedSetupResult
		public bool
	}{
		{"http", hostedSetupResult{contract.Version, "http://127.0.0.1:47831", strings.Repeat("s", 64)}, false},
		{"short-token", hostedSetupResult{contract.Version, "https://example.com", "short"}, false},
		{"header-injection", hostedSetupResult{contract.Version, "https://example.com", strings.Repeat("s", 64) + "\r\n"}, false},
		{"schema", hostedSetupResult{"other", "https://example.com", strings.Repeat("s", 64)}, false},
		{"public", hostedSetupResult{contract.Version, "https://example.com", strings.Repeat("s", 64)}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := privateConnectionDirectory(t)
			output := filepath.Join(root, "result.json")
			if test.public {
				// An ordinary file lacks the private mode on Unix and protected ACL on Windows.
				data, err := json.Marshal(test.result)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(output, data, 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(output, 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				writeConnectionFixture(t, root, "result.json", test.result)
			}
			check := func(context.Context, string, string) error {
				t.Fatal("unsafe output reached network validation")
				return nil
			}
			if err := adoptHostedConnection(context.Background(), root, output, infrastructure.ResolverMetadata{Name: "example.hosted"}, check); err == nil {
				t.Fatal("unsafe output accepted")
			}
			if _, err := os.Lstat(filepath.Join(root, "connection.json")); !os.IsNotExist(err) {
				t.Fatal("failed setup published configuration")
			}
		})
	}
}

func TestHostedSetupPreservesLocalBroker(t *testing.T) {
	root := privateConnectionDirectory(t)
	operator := OperatorConfig{Schema: contract.Version, Endpoint: "https://edge.example.com", Listen: "127.0.0.1:47831", Token: "existing-local-credential"}
	writeConnectionFixture(t, root, "operator.json", operator)
	if err := validateSetupMode(root, "hosted", "example.hosted"); err == nil {
		t.Fatal("hosted setup accepted existing local state")
	}
	var saved OperatorConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &saved); err != nil || saved != operator {
		t.Fatalf("existing local connection changed: %v", err)
	}
}

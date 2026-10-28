package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func activationContractFixture() EdgeActivationRequest {
	edge := EdgeConfig{Schema: Version, Endpoint: "https://edge.example.test", Executable: filepath.Join(os.TempDir(), "edge-tool"), Arguments: []string{"serve"}}
	return EdgeActivationRequest{Schema: EdgeActivationSchema, ExpectedConfigDigest: EdgeConfigDigest(edge), Candidate: edge, Recovery: edge,
		CandidateSHA256: strings.Repeat("a", 64), RecoverySHA256: strings.Repeat("b", 64), ReadinessURL: "http://127.0.0.1:45000/ready", StartupSeconds: 2, StableSeconds: 2, RecoverySeconds: 30, Maintenance: "allow-connector-replacement"}
}

func TestEdgeActivationContractBindsPolicyAndRejectsUnsafeProbes(t *testing.T) {
	request := activationContractFixture()
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/healthz", "/provider/status"} {
		generic := request
		generic.ReadinessURL = "http://127.0.0.1:45000" + path
		if err := generic.Validate(); err != nil {
			t.Fatal("provider readiness path rejected", path, err)
		}
	}
	id := request.ID()
	request.RecoverySeconds++
	if request.ID() == id {
		t.Fatal("recovery authority changed without changing approval")
	}
	for _, address := range []string{"http://example.test:45000/ready", "http://127.0.0.1:0/ready", "https://127.0.0.1:45000/ready", "http://user@127.0.0.1:45000/ready", "http://127.0.0.1:45000/ready?secret=value"} {
		request := activationContractFixture()
		request.ReadinessURL = address
		if request.Validate() == nil {
			t.Fatal("unsafe probe accepted", address)
		}
	}
	request = activationContractFixture()
	request.Maintenance = ""
	if request.Validate() == nil {
		t.Fatal("restart without maintenance policy accepted")
	}
	request = activationContractFixture()
	request.RecoverySeconds = 901
	if request.Validate() == nil {
		t.Fatal("unbounded recovery accepted")
	}
	for _, path := range []string{"/v1/edge-plan", "/v1/edge-activate", "/v1/edge-status"} {
		if OperatorScope(path) == "" {
			t.Fatal("missing OS authorization scope", path)
		}
	}
}

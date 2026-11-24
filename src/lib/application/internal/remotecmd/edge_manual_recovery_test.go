package remotecmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func TestEdgeManualRecoveryCLIRequiresExactFreshApprovalBeforeOperatorAccess(t *testing.T) {
	root := privateConnectionDirectory(t)
	config := contract.EdgeConfig{Schema: contract.Version, Endpoint: "https://edge.example.test", Executable: filepath.Join(root, "approved-image"), Arguments: []string{"serve"}}
	request := contract.EdgeActivationRequest{Schema: contract.EdgeActivationSchema, ExpectedConfigDigest: contract.EdgeConfigDigest(config),
		Candidate: config, Recovery: config, CandidateSHA256: strings.Repeat("a", 64), RecoverySHA256: strings.Repeat("a", 64),
		ReadinessURL: "http://127.0.0.1:45000/ready", StartupSeconds: 2, StableSeconds: 2, RecoverySeconds: 30,
		Maintenance: "allow-connector-replacement", ManualRecovery: &contract.EdgeManualRecovery{FailedActivationID: strings.Repeat("b", 64), Nonce: strings.Repeat("c", 64), ExpiresAt: time.Now().Add(time.Minute).UTC()}}
	path := filepath.Join(root, "request.json")
	if err := remoteio.WritePrivate(path, request); err != nil {
		t.Fatal(err)
	}
	oldApproval := request.ID()
	request.ManualRecovery.Nonce = strings.Repeat("d", 64)
	if err := remoteio.WritePrivate(path, request); err != nil {
		t.Fatal(err)
	}
	for _, approval := range []string{"", oldApproval} {
		err := activateEdge(context.Background(), root, path, approval, false)
		if err == nil || !strings.Contains(err.Error(), "approval does not match exact request") {
			t.Fatal("stale or missing approval reached operator authority", err)
		}
	}
}

package remote

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFeatureContractBindsDeadlineActionPayloadAndMachine(t *testing.T) {
	request := FeatureRequest{Schema: FeatureVersion, Target: FeatureTarget{Node: "node", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)},
		Root: "project", Action: "files.read", Payload: json.RawMessage(`{"path":"file"}`), Nonce: strings.Repeat("c", 64), Deadline: time.Now().Add(time.Minute).UTC()}
	request.ID, _ = request.Digest()
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*FeatureRequest){
		func(request *FeatureRequest) { request.Target.Node = "other" },
		func(request *FeatureRequest) { request.Target.Generation = strings.Repeat("d", 64) },
		func(request *FeatureRequest) { request.Action = "files.delete" },
		func(request *FeatureRequest) { request.Root = "other" },
		func(request *FeatureRequest) { request.Deadline = request.Deadline.Add(time.Second) },
		func(request *FeatureRequest) { request.Payload = json.RawMessage(`{}`) },
	} {
		changed := request
		mutate(&changed)
		if err := changed.Validate(); err == nil {
			t.Fatal("changed feature identity accepted")
		}
	}
}

func TestFeatureRoutesSeparateReadWriteAndUnknown(t *testing.T) {
	for _, path := range []string{"/v1/features-read-submit", "/v1/features-read-status", "/v1/features-catalog"} {
		if OperatorScope(path) != "remote.read" {
			t.Fatal("read feature route has wrong scope")
		}
	}
	for _, path := range []string{"/v1/features-write-submit", "/v1/features-write-status", "/v1/features-cancel", "/v1/features-reconcile"} {
		if OperatorScope(path) != "remote.execute" {
			t.Fatal("write feature route lacks execution scope")
		}
	}
	if OperatorScope("/v1/features-next") != "" || FeatureAccess("terminal.open") != "" || FeatureAccess("containers.exec") != "" {
		t.Fatal("unsupported generic execution exposed")
	}
	if FeatureAccess("git.push") != "publish" || FeatureAccess("git.fetch") != "write" {
		t.Fatal("Git access classification changed")
	}
}
